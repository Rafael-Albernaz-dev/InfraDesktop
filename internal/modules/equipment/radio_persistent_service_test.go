package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func radioHarness(t *testing.T, remote *radioLiveRemote) (*Service, Command, RadioInput, radioLiveStore) {
	t.Helper()
	ap, cpe, conc := radioLiveUUID(), radioLiveUUID(), radioLiveUUID()
	store := radioLiveStore{conc: conc,
		observation: radioLiveUUID(),
		row:         radioLiveUUID(),
		link: RadioLink{AP: ap,
			CPE:       cpe,
			MAC:       "AA:BB:CC:DD:EE:FF",
			Interface: "wlan1",
			BSSID:     "AA:BB:CC:DD:EE:00"},
		devices: map[string]Device{},
		saves:   &atomic.Int32{}}
	for i, item := range []struct {
		id   string
		role Role
	}{{ap, RoleAP}, {cpe, RoleCPE}, {conc, RoleConcentrator}} {
		store.devices[item.id] = Device{ID: item.id,
			Role:        item.role,
			Revision:    1,
			Active:      true,
			Address:     "192.0.2." + string(rune('1'+i)),
			Port:        22,
			Fingerprint: "synthetic",
			Transport:   TransportSSH}
	}
	s := New(radioLiveAuth{}, store, radioLiveVault{}, remote, radioLiveIDs{}, time.Now)
	t.Cleanup(s.Close)
	q := Command{Op: "radio.start",
		Scope: access.Command{Profile: "synthetic",
			ExpectedProfile:    "synthetic",
			Activation:         "active",
			ExpectedActivation: "active"}}
	key := radioKey(q, &store.link, []Device{store.devices[ap], store.devices[cpe]}, []uint64{1, 1})
	s.radioCosts = map[string]RadioCost{key: {Enabled: true, Interval: .002, PingLimit: 2}}
	in := RadioInput{DeviceID: conc,
		Revision:      1,
		ObservationID: store.observation,
		RowID:         store.row,
		Operation:     radioLiveUUID(),
		Mode:          RadioLive}
	return s, q, in, store
}

func TestRadioPersistentWorkerSequencesAndReceipt(t *testing.T) {
	for _, tc := range radioSequenceCases() {
		t.Run(tc.name, func(t *testing.T) { assertPersistentRadioSequence(t, tc) })
	}
}

type radioAddressMismatchStore struct{ radioLiveStore }

func (s radioAddressMismatchStore) PPSession(context.Context, string, PPInput) (PPView, Code) {
	return PPView{Rows: []PPRow{{ID: s.row, Login: "synthetic", Address: "192.0.2.99", State: "ONLINE"}}}, OK
}
func TestRadioPersistentCPEAddressCheckBeforeOpen(t *testing.T) {
	r := &radioLiveRemote{}
	s, q, in, store := radioHarness(t, r)
	s.store = radioAddressMismatchStore{store}
	j := &runningJob{command: q, radioInput: in}
	if _,
		c := s.openRadioSide(context.Background(),
		j,
		store.devices[store.link.CPE],
		1,
		RadioAirOSV6,
		&store.link); c != RadioCodeCPEUnreachable {
		t.Fatal("address mismatch admitted", c)
	}
	if r.calls.Load() != 0 || r.opens.Load() != 0 {
		t.Fatal("connection/query sent to mismatched CPE", r.opens.Load())
	}
}

func TestRadioOneObservationAcrossDistinctEndpoints(t *testing.T) {
	for _, modes := range [][2]RadioMode{{RadioMeasure, RadioMeasure}, {RadioLive, RadioMeasure}, {RadioMeasure, RadioLive}} {
		t.Run(string(modes[0])+"_"+string(modes[1]), func(t *testing.T) {
			s, q, in, _ := radioHarness(t, &radioLiveRemote{})
			in.Mode = modes[1]
			s.jobs[radioLiveUUID()] = &runningJob{command: q,
				device: Device{Address: "192.0.2.200",
					Port: 22},
				radioInput: RadioInput{Mode: modes[0]},
				cancel:     func() {}}
			if _, c := s.HandleRadio(context.Background(), q, in); c != access.Busy {
				t.Fatal("second distinct link admitted", modes, c)
			}
		})
	}
}

type live2RevocableAuth struct{ revoked atomic.Bool }

func (a *live2RevocableAuth) WithAuthorization(_ context.Context, _ uint32, _ access.Command, f func(uint64) Code) Code {
	if a.revoked.Load() {
		return access.Unauthorized
	}
	return f(1)
}
func TestRadioPersistentRevocationClosesIdleSessions(t *testing.T) {
	r := &radioLiveRemote{holdAfter: 1}
	s, q, in, _ := radioHarness(t, r)
	auth := &live2RevocableAuth{}
	s.auth = auth
	for key, cost := range s.radioCosts {
		cost.Interval = 5
		s.radioCosts[key] = cost
	}
	v, c := s.HandleRadio(context.Background(), q, in)
	if c != OK {
		t.Fatal(c)
	}
	deadline := time.Now().Add(time.Second)
	for r.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if r.calls.Load() < 2 {
		t.Fatal("no completed tick")
	}
	auth.revoked.Store(true)
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("revoked window retained sessions")
	}
	s.mu.Lock()
	job := s.jobs[v.Job.ID]
	got := job.view.Code
	s.mu.Unlock()
	if got != access.Unauthorized || r.closes.Load() != 2 {
		t.Fatal("lost revocation reason/resources", got, r.closes.Load())
	}
}

// The first tick after a reopened SSH connection is still an observed tick:
// its CPU must enter the series/streak, never be discarded as hidden validation.
type reopenedCPURemote struct {
	*radioLiveRemote
	apReads atomic.Int32
}
type reopenedCPUSession struct {
	RadioReadSession
	owner *reopenedCPURemote
	ap    bool
}

func (r *reopenedCPURemote) OpenRadioSession(ctx context.Context,
	d Device,
	u,
	p string,
	f RadioFamily,
	l *RadioLink,
	check func() Code) (RadioReadSession,
	Code) {
	session, c := r.radioLiveRemote.OpenRadioSession(ctx, d, u, p, f, l, check)
	return reopenedCPUSession{session, r, d.Role == RoleAP}, c
}
func (s reopenedCPUSession) Tick() (*RadioReading, Code) {
	if s.ap && s.owner.apReads.Add(1) == 1 {
		return &RadioReading{CommandSeconds: .1}, RadioCodeSSHConnectionLost
	}
	return s.RadioReadSession.Tick()
}
func TestRadioReopenedCPUIsAnObservedTick(t *testing.T) {
	remote := &reopenedCPURemote{radioLiveRemote: &radioLiveRemote{cpuTicks: [][]int{{100}, {100}}}}
	s, q, in, _ := radioHarness(t, remote.radioLiveRemote)
	s.remote = remote
	v, c := s.HandleRadio(context.Background(), q, in)
	if c != OK {
		t.Fatal(c)
	}
	s.wg.Wait()
	s.mu.Lock()
	j := s.jobs[v.Job.ID]
	got := j.view
	cost := j.radio.Cost
	s.mu.Unlock()
	if got.Code != RadioCodeLiveAborted ||
		remote.apReads.Load() != 3 ||
		cost == nil ||
		len(cost.Series) != 3 ||
		!strings.Contains(cost.Reason,
			"CPU") ||
		len(cost.Series[1].CPU) != 1 ||
		cost.Series[1].CPU[0] != 100 {
		t.Fatalf("reopen CPU was hidden/skipped: code=%s APreads=%d cost=%+v", got.Code, remote.apReads.Load(), cost)
	}
}
