package pulse

import (
	"context"
	"testing"
	"time"
)

type cancelReadDialer struct{ started, cancelled chan struct{} }

func (d cancelReadDialer) Dial(ctx context.Context, _ ReadTarget) (ReadSession, ReadFault) {
	close(d.started)
	<-ctx.Done()
	close(d.cancelled)
	return nil, ReadLost
}
func TestInvalidatedTargetCancelsInFlightHandshake(t *testing.T) {
	for _, event := range []string{"company", "removed", "changed", "close"} {
		t.Run(event, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			target := alvo
			for ReadSlot(target.ID) == 0 {
				target.ID += "a"
			}
			inv := &fakeReadInventory{}
			inv.set("p", target)
			d := cancelReadDialer{make(chan struct{}), make(chan struct{})}
			c := NewReadCollector(inv, d, always, 4)
			defer c.Close()
			c.Step(ctx, 0, r0)
			done := make(chan struct{})
			go func() { defer close(done); c.Step(ctx, ReadSlot(target.ID), r0) }()
			<-d.started
			switch event {
			case "company":
				inv.set("q")
			case "removed":
				inv.set("p")
			case "changed":
				target.Stamp = "v2"
				inv.set("p", target)
			case "close":
				c.Close()
			}
			if event != "close" {
				c.Step(ctx, 0, r0.Add(time.Minute))
			}
			select {
			case <-d.cancelled:
			case <-time.After(time.Second):
				t.Fatal("invalidated handshake kept running")
			}
			<-done
		})
	}
}

type lateRefusalDialer struct {
	started, release chan struct{}
	calls            int
}

func (d *lateRefusalDialer) Dial(context.Context, ReadTarget) (ReadSession, ReadFault) {
	d.calls++
	if d.calls == 1 {
		close(d.started)
		<-d.release
	}
	return nil, ReadRefused
}
func TestLateRefusalSurvivesCompanySwitchAndReturn(t *testing.T) {
	target := alvo
	for ReadSlot(target.ID) == 0 {
		target.ID += "a"
	}
	inv := &fakeReadInventory{}
	inv.set("p", target)
	d := &lateRefusalDialer{started: make(chan struct{}), release: make(chan struct{})}
	c := NewReadCollector(inv, d, always, 4)
	defer c.Close()
	c.Step(context.Background(), 0, r0)
	done := make(chan struct{})
	go func() { defer close(done); c.Step(context.Background(), ReadSlot(target.ID), r0) }()
	<-d.started
	inv.set("q")
	c.Step(context.Background(), 0, r0.Add(time.Minute))
	close(d.release)
	<-done
	inv.set("p", target)
	ciclo(c, 2)
	if d.calls != 1 {
		t.Fatalf("refused password retried %d times", d.calls)
	}
}
