package equipment

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
}

func (s radioLiveStore) ReadPPTarget(_ context.Context, _ string, id string, _ bool) (PPTarget, Code) {
	return PPTarget{Device: s.devices[id], Credential: SealedCredential{Metadata: Credential{Revision: 1}}}, OK
}
func (s radioLiveStore) PPCurrent(context.Context, string, string) (PPView, Code) {
	d := s.devices[s.conc]
	return PPView{Current: &PPObservation{ID: s.observation, Provenance: PPProvenance{Revision: 1, CredentialRevision: 1, Connection: PPConnection(d)}}}, OK
}
func (s radioLiveStore) PPSession(context.Context, string, PPInput) (PPView, Code) {
	return PPView{Rows: []PPRow{{ID: s.row, Login: "synthetic", Address: s.devices[s.link.CPE].Address, State: "ONLINE"}}}, OK
}
func (s radioLiveStore) RadioCurrent(context.Context, string, string) (*RadioReading, *RadioProfile, Code) {
	return nil, &RadioProfile{Confirmed: true, Family: "routeros-wireless-v6"}, OK
}
func (s radioLiveStore) SaveRadio(context.Context, string, Device, uint64, *RadioReading) Code {
	return OK
}
func (s radioLiveStore) RadioLinks(context.Context, string, string, string) ([]RadioLink, Code) {
	return []RadioLink{s.link}, OK
}

type radioLiveRemote struct {
	Remote
	slowSide string
	delay    time.Duration
	calls    atomic.Int32
}

func (r *radioLiveRemote) ReadRadio(ctx context.Context, d Device, _ string, _ string, _ string, check func() Code) (*RadioReading, Code) {
	if c := check(); c != OK {
		return nil, c
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
	if d.Role == r.slowSide {
		seconds = 3.2
	}
	cpu := []int{6, 7, 6}
	if r.slowSide == "cpu" {
		cpu = []int{6, 91, 6}
	}
	if r.slowSide == "cpu90" {
		cpu = []int{26, 90, 57}
	}
	return &RadioReading{CommandSeconds: seconds, PingRTT: &ping, CPU: cpu, Compatible: true, Clients: []RadioClient{{MAC: "AA:BB:CC:DD:EE:FF", Interface: "wlan1", BSSID: "AA:BB:CC:DD:EE:00"}}, APBSSID: "AA:BB:CC:DD:EE:00", WirelessMAC: "AA:BB:CC:DD:EE:FF"}, OK
}
func TestL07LiveWorkerUsesBothCommandsAndTwoConsecutiveViolations(t *testing.T) {
	for _, side := range []string{"healthy", "ap", "cpe", "cpu", "cpu90"} {
		t.Run(side, func(t *testing.T) {
			ap, cpe, conc := radioLiveUUID(), radioLiveUUID(), radioLiveUUID()
			store := radioLiveStore{conc: conc, observation: radioLiveUUID(), row: radioLiveUUID(), link: RadioLink{AP: ap, CPE: cpe, MAC: "AA:BB:CC:DD:EE:FF", Interface: "wlan1", BSSID: "AA:BB:CC:DD:EE:00"}, devices: map[string]Device{}}
			for i, item := range []struct{ id, role string }{{ap, "ap"}, {cpe, "cpe"}, {conc, "concentrator"}} {
				store.devices[item.id] = Device{ID: item.id, Role: item.role, Revision: 1, Active: true, Address: "192.0.2." + string(rune('1'+i)), Port: 22, Fingerprint: "synthetic", Transport: "ssh"}
			}
			remote := &radioLiveRemote{slowSide: side}
			if side == "healthy" || side == "cpu90" {
				remote.delay = 2500 * time.Millisecond
			}
			s := New(radioLiveAuth{}, store, radioLiveVault{}, remote, radioLiveIDs{}, time.Now)
			t.Cleanup(s.Close)
			q := Command{Op: "radio.start", Scope: access.Command{Profile: "synthetic", ExpectedProfile: "synthetic", Activation: "active", ExpectedActivation: "active"}}
			key := radioKey(q, &store.link, []Device{store.devices[ap], store.devices[cpe]}, []uint64{1, 1})
			s.radioCosts = map[string]RadioCost{key: {Enabled: true, Interval: .001, CPUBaseline: 6, PingLimit: 2}}
			in := RadioInput{DeviceID: conc, Revision: 1, ObservationID: store.observation, RowID: store.row, Operation: radioLiveUUID(), Mode: "live"}
			view, c := s.HandleRadio(context.Background(), q, in)
			if c != OK {
				t.Fatal(c)
			}
			job := view.Job.ID
			// A second admission cannot launch another link while this one owns it.
			second := in
			second.Operation = radioLiveUUID()
			if _, c = s.HandleRadio(context.Background(), q, second); c != access.Busy {
				t.Fatal("second live admitted", c)
			}
			deadline := time.Now().Add(14 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(5 * time.Millisecond)
				status := q
				status.Op = "radio.status"
				view, c = s.HandleRadio(context.Background(), status, RadioInput{JobID: job})
				if c != OK {
					t.Fatal(c)
				}
				if (side == "healthy" || side == "cpu90") && remote.calls.Load() >= 4 {
					if view.Job.State != "RUNNING" {
						t.Fatal("5s path aborted despite0.4s commands", view.Job)
					}
					stop := q
					stop.Op = "radio.cancel"
					if _, c = s.HandleRadio(context.Background(), stop, RadioInput{JobID: job}); c != OK {
						t.Fatal(c)
					}
					s.wg.Wait()
					if remote.calls.Load() > 5 {
						t.Fatal("read continued after stop")
					}
					return
				}
				if view.Job.State != "RUNNING" {
					if side == "healthy" || side == "cpu90" || view.Job.Code != "RADIO_LIVE_ABORTED" || remote.calls.Load() != 4 {
						t.Fatal("expected two complete violating ticks", side, view.Job, remote.calls.Load())
					}
					s.wg.Wait()
					return
				}
			}
			s.mu.Lock()
			s.jobs[job].cancel()
			s.mu.Unlock()
			s.wg.Wait()
			t.Fatal("worker did not reach expected state")
		})
	}
}
