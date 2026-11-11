package routerssh_test

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/access"
	e "infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type recoveryAuth struct{ revoked atomic.Bool }

func (a *recoveryAuth) WithAuthorization(_ context.Context, _ uint32, _ access.Command, f func(uint64) e.Code) e.Code {
	if a.revoked.Load() {
		return access.Unauthorized
	}
	return f(1)
}

type recoveryVault struct{ e.Vault }

func (recoveryVault) Ready(string, string, []byte) bool { return true }
func (recoveryVault) Open(string, string, string, uint64, []byte, []byte) (string, e.Code) {
	return sshfixture.Password, e.OK
}

type recoveryIDs struct{}

func (recoveryIDs) ID() (string, e.Code) { return uuid.NewString(), e.OK }

type recoveryStore struct {
	e.Store
	e.PPStore
	e.RadioStore
	devices        map[string]e.Device
	conc, obs, row string
	link           e.RadioLink
}

func (s recoveryStore) ReadPPTarget(_ context.Context, _ string, id string, _ bool) (e.PPTarget, e.Code) {
	return e.PPTarget{Device: s.devices[id], Credential: e.SealedCredential{Metadata: e.Credential{Revision: 1, User: "reader"}}}, e.OK
}
func (s recoveryStore) PPCurrent(context.Context, string, string) (e.PPView, e.Code) {
	d := s.devices[s.conc]
	return e.PPView{Current: &e.PPObservation{ID: s.obs,
			Provenance: e.PPProvenance{Revision: 1,
				CredentialRevision: 1,
				Connection:         e.PPConnection(d)}}},
		e.OK
}
func (s recoveryStore) PPSession(context.Context, string, e.PPInput) (e.PPView, e.Code) {
	return e.PPView{Rows: []e.PPRow{{ID: s.row, Login: "synthetic", Address: "127.0.0.1", State: "ONLINE"}}}, e.OK
}
func (s recoveryStore) RadioCurrent(_ context.Context, _ string, id string) (*e.RadioReading, *e.RadioProfile, e.Code) {
	family := e.RadioRouterOSWirelessV6
	if id == s.link.CPE {
		family = e.RadioAirOSV6
	}
	return nil, &e.RadioProfile{Confirmed: true, Family: family}, e.OK
}
func (s recoveryStore) RadioLinks(context.Context, string, string, string) ([]e.RadioLink, e.Code) {
	return []e.RadioLink{s.link}, e.OK
}
func (s recoveryStore) SaveRadio(context.Context, string, e.Device, uint64, *e.RadioReading) e.Code {
	return e.OK
}
func recoveryHarness(t *testing.T, ap, cpe *sshfixture.Server) (*e.Service, e.Command, e.RadioInput, *recoveryAuth) {
	t.Helper()
	aid, cid, conc := uuid.NewString(), uuid.NewString(), uuid.NewString()
	store := recoveryStore{devices: map[string]e.Device{},
		conc: conc,
		obs:  uuid.NewString(),
		row:  uuid.NewString(),
		link: e.RadioLink{AP: aid,
			CPE:       cid,
			MAC:       "02:00:00:00:10:01",
			Interface: "wlan1",
			BSSID:     "02:00:00:00:10:05"}}
	for _, v := range []struct {
		id   string
		role e.Role
		p    *sshfixture.Server
	}{{aid, e.RoleAP, ap}, {cid, e.RoleCPE, cpe}, {conc, e.RoleConcentrator, ap}} {
		store.devices[v.id] = e.Device{ID: v.id,
			Role:        v.role,
			Revision:    1,
			Active:      true,
			Address:     "127.0.0.1",
			Port:        v.p.Port(),
			Fingerprint: v.p.Fingerprint(),
			Transport:   e.TransportSSH}
	}
	auth := &recoveryAuth{}
	s := e.New(auth, store, recoveryVault{}, &recoverySSHRemote{}, recoveryIDs{}, time.Now)
	t.Cleanup(s.Close)
	q := e.Command{Op: "radio.start",
		Scope: access.Command{Profile: "synthetic",
			ExpectedProfile:    "synthetic",
			Activation:         "active",
			ExpectedActivation: "active"}}
	return s,
		q,
		e.RadioInput{DeviceID: conc,
			Revision:      1,
			ObservationID: store.obs,
			RowID:         store.row,
			Operation:     uuid.NewString(),
			Mode:          e.RadioMeasure},
		auth
}
func recoveryStatus(t *testing.T, s *e.Service, q e.Command, id string, done func(e.RadioView) bool) e.RadioView {
	t.Helper()
	q.Op = "radio.status"
	deadline := time.Now().Add(18 * time.Second)
	for time.Now().Before(deadline) {
		v, c := s.HandleRadio(context.Background(), q, e.RadioInput{JobID: id})
		if c != e.OK {
			t.Fatal(c)
		}
		if done(v) {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("causal state was not observed")
	return e.RadioView{}
}
func recoveryStart(t *testing.T, s *e.Service, q e.Command, in e.RadioInput) string {
	t.Helper()
	v, c := s.HandleRadio(context.Background(), q, in)
	if c != e.OK {
		t.Fatal(c)
	}
	return v.Job.ID
}
func recoveryCancel(t *testing.T, s *e.Service, q e.Command, id string) {
	t.Helper()
	q.Op = "radio.cancel"
	if _, c := s.HandleRadio(context.Background(), q, e.RadioInput{JobID: id}); c != e.OK {
		t.Fatal(c)
	}
	s.Close()
}
func recoveryGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("radioparse/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const recoveryTick = ("L07-TICK\ncpu=40\nbssid=02:00:00:00:10:05\nL07-CLIENTS\n 0 interface=wlan1 mac" +
	"-address=02:00:00:00:10:01 uptime=1m signal-strength=-60 bytes=100,200 packets" +
	"=10,20\nL07-TICK-DONE\n")

func TestRadioTickRecoveryPeer(t *testing.T) {
	for _, scenario := range []string{"isolated", "consecutive", "changedPin", "collectorDefect"} {
		t.Run(scenario, func(t *testing.T) { runRecoveryScenario(t, scenario) })
	}
}
func TestRadioRealXWGolden(t *testing.T) {
	r, c := radioparse.ParseRadioAirOS(recoveryGolden(t, "l07_airos_xw_6_3_6.txt"))
	if c != e.OK ||
		r.Family != e.RadioAirOSV6 ||
		r.Model != "NanoStation loco M5" ||
		r.WirelessUptime != "191640s" ||
		r.DeviceUptime != "191732s" ||
		r.Counters["tx_bytes"] != "1183429511" {
		t.Fatalf("real sanitized XW identity/counters: %+v %s", r, c)
	}
	want := map[string]float64{"signal": -67, "snr": 30, "ccq": 99.1, "rate_tx": 45, "rate_rx": 90}
	for _, m := range r.Metrics {
		if value, ok := want[m.Name]; ok {
			if m.Value != value {
				t.Fatalf("%s=%v want=%v", m.Name, m.Value, value)
			}
			delete(want, m.Name)
		}
	}
	if len(want) != 0 {
		t.Fatal("real XW metrics absent", want)
	}
}
