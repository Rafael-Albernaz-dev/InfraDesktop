package radio

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/access"
	"sync/atomic"
	"testing"
	"time"
)

var radioLiveSequence atomic.Uint64

func radioLiveUUID() string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", radioLiveSequence.Add(1))
}

type radioLiveAuth struct{}

func (radioLiveAuth) WithAuthorization(_ context.Context, _ uint32, _ access.Command, f func(uint64) Code) Code {
	return f(1)
}

type radioLiveVault struct{ Vault }

func (radioLiveVault) Ready(string, string, []byte) bool { return true }
func (radioLiveVault) Open(string, string, string, uint64, []byte, []byte) (string, Code) {
	return "synthetic", OK
}

type radioLiveIDs struct{}

func (radioLiveIDs) ID() (string, Code) { return radioLiveUUID(), OK }

type radioLiveStore struct {
	Store
	PPStore
	RadioStore
	devices                map[string]Device
	conc, observation, row string
	link                   RadioLink
	saves                  *atomic.Int32
}

func (s radioLiveStore) ReadPPTarget(_ context.Context, _ string, id string, _ bool) (PPTarget, Code) {
	return PPTarget{Device: s.devices[id], Credential: SealedCredential{Metadata: Credential{Revision: 1}}}, OK
}
func (s radioLiveStore) PPCurrent(context.Context, string, string) (PPView, Code) {
	d := s.devices[s.conc]
	return PPView{Current: &PPObservation{ID: s.observation,
			Provenance: PPProvenance{Revision: 1,
				CredentialRevision: 1,
				Connection:         PPConnection(d)}}},
		OK
}
func (s radioLiveStore) PPSession(context.Context, string, PPInput) (PPView, Code) {
	return PPView{Rows: []PPRow{{ID: s.row, Login: "synthetic", Address: s.devices[s.link.CPE].Address, State: "ONLINE"}}}, OK
}
func (s radioLiveStore) RadioCurrent(context.Context, string, string) (*RadioReading, *RadioProfile, Code) {
	return nil, &RadioProfile{Confirmed: true, Family: RadioRouterOSWirelessV6}, OK
}
func (s radioLiveStore) SaveRadio(context.Context, string, Device, uint64, *RadioReading) Code {
	if s.saves != nil {
		s.saves.Add(1)
	}
	return OK
}
func (s radioLiveStore) RadioLinks(context.Context, string, string, string) ([]RadioLink, Code) {
	return []RadioLink{s.link}, OK
}

type radioLiveRemote struct {
	Remote
	slowSide  string
	delay     time.Duration
	calls     atomic.Int32
	cpuTicks  [][]int
	pingTicks [][]float64
	apTicks   atomic.Int32
	closes    atomic.Int32
	opens     atomic.Int32
	holdAfter int
}

type radioFakeSession struct {
	remote *radioLiveRemote
	ctx    context.Context
	d      Device
	check  func() Code
}

func (r *radioLiveRemote) OpenRadioSession(ctx context.Context,
	d Device,
	_,
	_ string,
	_ RadioFamily,
	_ *RadioLink,
	check func() Code) (RadioReadSession,
	Code) {
	r.opens.Add(1)
	return &radioFakeSession{r, ctx, d, check}, OK
}
func (r *radioFakeSession) Initial() (*RadioReading, Code) {
	fresh := &radioLiveRemote{slowSide: r.remote.slowSide}
	return fresh.ReadRadio(r.ctx, r.d, "", "", "", r.check)
}
func (r *radioFakeSession) Tick() (*RadioReading, Code) {
	reading, c := r.remote.ReadRadio(r.ctx, r.d, "", "", "", r.check)
	if reading != nil {
		reading.Scope = "link-tick"
	}
	return reading, c
}
func (r *radioFakeSession) Probe() (RadioLoad, Code) {
	values := []float64{1}
	i := int(r.remote.apTicks.Load()) - 1
	if i >= 0 && i < len(r.remote.pingTicks) {
		values = r.remote.pingTicks[i]
	}
	return RadioLoad{Started: time.Now().Add(-time.Millisecond),
			Ended:        time.Now(),
			PingSent:     len(values),
			PingReceived: len(values),
			PingRTT:      values},
		OK
}
func (r *radioFakeSession) Close() { r.remote.closes.Add(1) }

func (r *radioLiveRemote) ReadRadio(ctx context.Context,
	d Device,
	_ string,
	_ string,
	_ RadioFamily,
	check func() Code) (*RadioReading,
	Code) {
	if c := check(); c != OK {
		return nil, c
	}
	index := -1
	if d.Role == RoleAP {
		index = int(r.apTicks.Add(1)) - 1
		if r.holdAfter > 0 && index >= r.holdAfter {
			<-ctx.Done()
			return nil, Cancelled
		}
	}
	if r.delay > 0 {
		timer := time.NewTimer(r.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, Cancelled
		case <-timer.C:
		}
	}
	r.calls.Add(1)
	ping := 1.0
	seconds := .4
	if string(d.Role) == r.slowSide {
		seconds = 3.2
	}
	cpu := []int{6, 7, 6}
	if index >= 0 && index < len(r.cpuTicks) {
		cpu = r.cpuTicks[index]
	}
	if r.slowSide == "cpu" {
		cpu = []int{91}
	}
	if r.slowSide == "cpu90" {
		cpu = []int{26, 90, 57}
	}
	return &RadioReading{CommandSeconds: seconds,
			PingRTT:    &ping,
			CPU:        cpu,
			Compatible: true,
			Clients: []RadioClient{{MAC: "AA:BB:CC:DD:EE:FF",
				Interface: "wlan1",
				BSSID:     "AA:BB:CC:DD:EE:00"}},
			APBSSID:     "AA:BB:CC:DD:EE:00",
			WirelessMAC: "AA:BB:CC:DD:EE:FF"},
		OK
}
func TestRadioLiveWorkerUsesBothCommandsAndTwoConsecutiveViolations(t *testing.T) {
	for _, side := range []string{"healthy", string(RoleAP), string(RoleCPE), "cpu", "cpu90"} {
		t.Run(side, func(t *testing.T) { assertCommandTimingLive(t, side) })
	}
}
