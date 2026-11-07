package pulse

import (
	"context"
	"testing"
	"time"
)

type signalledReadInventory struct {
	fakeReadInventory
	rounds chan struct{}
}

func (i *signalledReadInventory) ReadTargets(ctx context.Context) (string, []ReadTarget, bool) {
	profile, targets, ok := i.fakeReadInventory.ReadTargets(ctx)
	i.rounds <- struct{}{}
	return profile, targets, ok
}
func TestReadTicksRefreshInventoryWhileHandshakeIsStalled(t *testing.T) {
	target := alvo
	for ReadSlot(target.ID) != 0 {
		target.ID += "a"
	}
	inv := &signalledReadInventory{rounds: make(chan struct{}, 2)}
	inv.set("p", target)
	d := cancelReadDialer{make(chan struct{}), make(chan struct{})}
	c := NewReadCollector(inv, d, always, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time, ReadSlots)
	done := make(chan struct{})
	go func() { defer close(done); c.runReadTicks(ctx, ticks) }()
	<-inv.rounds
	<-d.started
	for second := 1; second <= ReadSlots; second++ {
		ticks <- r0.Add(time.Duration(second) * time.Second)
	}
	select {
	case <-inv.rounds:
	case <-time.After(time.Second):
		t.Fatal("stalled handshake stopped inventory rounds")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runner did not close")
	}
	if _, ok := c.Snapshot("p", r0); ok {
		t.Fatal("runner left collector open")
	}
}
