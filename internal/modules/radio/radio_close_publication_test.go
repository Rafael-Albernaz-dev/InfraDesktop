package radio

import (
	"context"
	"infradesktop/internal/modules/access"
	"sync"
	"testing"
	"time"
)

type publicationCloseGate struct {
	entered, release       chan struct{}
	enterOnce, releaseOnce sync.Once
}

func (g *publicationCloseGate) unblock() { g.releaseOnce.Do(func() { close(g.release) }) }

type publicationRemote struct {
	*radioLiveRemote
	gate *publicationCloseGate
}
type publicationSession struct {
	RadioReadSession
	gate *publicationCloseGate
	ap   bool
}

func (r *publicationRemote) OpenRadioSession(ctx context.Context, d Device, u, p string, f RadioFamily, link *RadioLink,
	check func() Code) (RadioReadSession, Code) {
	session, code := r.radioLiveRemote.OpenRadioSession(ctx, d, u, p, f, link, check)
	if code != OK {
		return nil, code
	}
	return &publicationSession{RadioReadSession: session, gate: r.gate, ap: d.Role == RoleAP}, OK
}
func (s *publicationSession) Tick() (*RadioReading, Code) {
	if s.ap {
		return nil, CollectorDefect
	}
	return s.RadioReadSession.Tick()
}
func (s *publicationSession) Close() {
	if s.ap {
		s.gate.enterOnce.Do(func() { close(s.gate.entered); <-s.gate.release })
	}
	s.RadioReadSession.Close()
}
func TestRadioTerminalStateWaitsForSessionCleanup(t *testing.T) {
	base := &radioLiveRemote{}
	service, command, input, _ := radioHarness(t, base)
	gate := &publicationCloseGate{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(gate.unblock)
	service.Remote = &publicationRemote{radioLiveRemote: base, gate: gate}
	started, code := service.HandleRadio(context.Background(), command, input)
	if code != OK {
		t.Fatal(code)
	}
	select {
	case <-gate.entered:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not enter session Close")
	}
	status := command
	status.Op = "radio.status"
	view, code := service.HandleRadio(context.Background(), status, RadioInput{JobID: started.Job.ID})
	if code != OK || view.Job.State != "RUNNING" {
		t.Fatalf("terminal state published before cleanup: %s %+v", code, view.Job)
	}
	next := input
	next.Operation = radioLiveUUID()
	if _, code = service.HandleRadio(context.Background(), command, next); code != access.Busy {
		t.Fatal("new work overlapped closing session", code)
	}
	gate.unblock()
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		view, code = service.HandleRadio(context.Background(), status, RadioInput{JobID: started.Job.ID})
		if code != OK {
			t.Fatal(code)
		}
		if view.Job.State != "RUNNING" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if view.Job.State == "RUNNING" || view.Job.Code != CollectorDefect {
		t.Fatal("cleanup changed failure precedence", view.Job)
	}
	if _, code = service.HandleRadio(context.Background(), command, next); code != OK {
		t.Fatal("terminal job still owns admission", code)
	}
}
