package radio

import (
	"context"
	"sync/atomic"
	"testing"
)

type dataTickRemote struct {
	*radioLiveRemote
	code   Code
	reason string
	ticks  atomic.Int32
}
type dataTickSession struct {
	RadioReadSession
	owner *dataTickRemote
	ap    bool
}

func (r *dataTickRemote) OpenRadioSession(ctx context.Context, d Device, u, p string, f RadioFamily, l *RadioLink,
	check func() Code) (RadioReadSession, Code) {
	session, c := r.radioLiveRemote.OpenRadioSession(ctx, d, u, p, f, l, check)
	return dataTickSession{session, r, d.Role == RoleAP}, c
}
func (s dataTickSession) Tick() (*RadioReading, Code) {
	if s.ap && s.owner.ticks.Add(1) == 1 {
		return &RadioReading{CommandSeconds: .1, Failure: &RadioFailure{Reason: s.owner.reason}}, s.owner.code
	}
	return s.RadioReadSession.Tick()
}
func TestRadioWorkerReusesDataErrorSessionAndReopensTransport(t *testing.T) {
	for _, tc := range []struct {
		reason string
		code   Code
		opens  int32
	}{
		{"tick-frame", Partial, 2}, {"tick-cpu", Partial, 2}, {"exit", RadioCodeSSHConnectionLost, 3},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			remote := &dataTickRemote{radioLiveRemote: &radioLiveRemote{cpuTicks: [][]int{{100}, {100}}}, code: tc.code, reason: tc.reason}
			service, q, in, _ := radioHarness(t, remote.radioLiveRemote)
			service.Remote = remote
			view, c := service.HandleRadio(context.Background(), q, in)
			if c != OK {
				t.Fatal(c)
			}
			service.WG.Wait()
			service.Mu.Lock()
			result := service.Jobs[view.Job.ID].View
			service.Mu.Unlock()
			if result.Code != RadioCodeLiveAborted {
				t.Fatal("CPU gate changed", result.Code)
			}
			if remote.opens.Load() != tc.opens || remote.closes.Load() != tc.opens {
				t.Fatalf("%s: opens=%d closes=%d want=%d", tc.reason, remote.opens.Load(), remote.closes.Load(), tc.opens)
			}
		})
	}
}
