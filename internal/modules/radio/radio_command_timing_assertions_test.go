package radio

import (
	"context"
	"infradesktop/internal/modules/access"
	"strings"
	"testing"
	"time"
)

func commandTimingStore() radioLiveStore {
	ap, cpe, conc := radioLiveUUID(), radioLiveUUID(), radioLiveUUID()
	store := radioLiveStore{conc: conc,
		observation: radioLiveUUID(),
		row:         radioLiveUUID(),
		link: RadioLink{AP: ap,
			CPE:       cpe,
			MAC:       "AA:BB:CC:DD:EE:FF",
			Interface: "wlan1",
			BSSID:     "AA:BB:CC:DD:EE:00"},
		devices: map[string]Device{}}
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
	return store
}
func commandTimingHarness(t *testing.T, side string) (*Service, Command, RadioInput, *radioLiveRemote) {
	t.Helper()
	store := commandTimingStore()
	remote := &radioLiveRemote{slowSide: side}
	if side == "healthy" || side == "cpu90" {
		remote.delay = 2500 * time.Millisecond
	}
	s := NewWithInventory(radioLiveAuth{}, store, radioLiveVault{}, remote, radioLiveIDs{}, time.Now)
	t.Cleanup(s.Close)
	q := Command{Op: "radio.start",
		Scope: access.Command{Profile: "synthetic",
			ExpectedProfile:    "synthetic",
			Activation:         "active",
			ExpectedActivation: "active"}}
	key := radioKey(q, &store.link, []Device{store.devices[store.link.AP], store.devices[store.link.CPE]}, []uint64{1, 1})
	s.radioCosts = map[string]RadioCost{key: {Enabled: true, Interval: .001, CPUBaseline: 6, PingLimit: 2}}
	in := RadioInput{DeviceID: store.conc,
		Revision:      1,
		ObservationID: store.observation,
		RowID:         store.row,
		Operation:     radioLiveUUID(),
		Mode:          RadioLive}
	return s, q, in, remote
}
func assertCommandTimingLive(t *testing.T, side string) {
	t.Helper()
	s, q, in, remote := commandTimingHarness(t, side)
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
	if observeCommandTimingLive(t, s, q, job, side, remote) {
		return
	}
	s.Mu.Lock()
	s.Jobs[job].Cancel()
	s.Mu.Unlock()
	s.WG.Wait()
	t.Fatal("worker did not reach expected state")
}
func observeCommandTimingLive(t *testing.T, s *Service, q Command, job, side string, remote *radioLiveRemote) bool {
	t.Helper()
	deadline := time.Now().Add(14 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		status := q
		status.Op = "radio.status"
		view, c := s.HandleRadio(context.Background(), status, RadioInput{JobID: job})
		if c != OK {
			t.Fatal(c)
		}
		if (side == "healthy" || side == "cpu90") && remote.calls.Load() >= 4 {
			assertHealthyCommandTimingStop(t, s, q, job, view, remote)
			return true
		}
		if view.Job.State != "RUNNING" {
			assertCommandTimingAbort(t, s, side, view, remote)
			return true
		}
	}
	return false
}
func assertHealthyCommandTimingStop(t *testing.T, s *Service, q Command, job string, view RadioView, remote *radioLiveRemote) {
	t.Helper()
	if view.Job.State != "RUNNING" {
		t.Fatal("5s path aborted despite0.4s commands", view.Job)
	}
	stop := q
	stop.Op = "radio.cancel"
	if _, c := s.HandleRadio(context.Background(), stop, RadioInput{JobID: job}); c != OK {
		t.Fatal(c)
	}
	s.WG.Wait()
	if remote.calls.Load() > 5 {
		t.Fatal("read continued after stop")
	}
}
func assertCommandTimingAbort(t *testing.T, s *Service, side string, view RadioView, remote *radioLiveRemote) {
	t.Helper()
	if side == string(RoleAP) || side == string(RoleCPE) {
		if view.Job.Code != RadioCodeLiveAborted ||
			remote.calls.Load() != 4 ||
			view.Cost == nil ||
			len(view.Cost.Series) != 2 ||
			!strings.Contains(view.Cost.Reason,
				"limite 3 s") {
			t.Fatal("two observed slow ticks must abort, without hidden validation", side, view.Job, remote.calls.Load(), view.Cost)
		}
		s.WG.Wait()
		return
	}

	if side == "healthy" || side == "cpu90" || view.Job.Code != RadioCodeLiveAborted || remote.calls.Load() != 4 {
		t.Fatal("expected two complete violating ticks", side, view.Job, remote.calls.Load())
	}
	s.WG.Wait()
	return
}
