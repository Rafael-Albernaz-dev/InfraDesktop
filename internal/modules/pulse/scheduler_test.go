package pulse

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type roundInventory struct {
	base   *fakeInventory
	rounds chan struct{}
}

func (i roundInventory) Targets(ctx context.Context) (string, []Target, bool) {
	profile, targets, ok := i.base.Targets(ctx)
	i.rounds <- struct{}{}
	return profile, targets, ok
}

type waitingProber struct{ entered chan struct{} }

func (p waitingProber) Probe(ctx context.Context, _ string) Outcome {
	p.entered <- struct{}{}
	<-ctx.Done()
	return NoReply
}
func TestSlowProbeDoesNotDelayNextInventoryRoundOrOutliveCancellation(t *testing.T) {
	target := a
	for n := 0; Slot(target.ID) != 0; n++ {
		target.ID = fmt.Sprint(n)
	}
	inv := roundInventory{&fakeInventory{profile: "p1", targets: []Target{target}, ok: true}, make(chan struct{}, 2)}
	prober := waitingProber{make(chan struct{}, 1)}
	collector := NewCollector(inv, prober, 16)
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { collector.runTicks(ctx, ticks); close(done) }()
	select {
	case <-prober.entered:
	case <-time.After(time.Second):
		t.Fatal("probe not started")
	}
	<-inv.rounds
	inv.base.set("p2", nil, true)
	for i := 1; i <= Slots; i++ {
		select {
		case ticks <- at(i):
		case <-time.After(time.Second):
			t.Fatal("slow probe blocked cadence")
		}
	}
	select {
	case <-inv.rounds:
	case <-time.After(time.Second):
		t.Fatal("inventory refresh skipped")
	}
	// Synchronize after refresh through the next tick before observing the profile.
	ticks <- at(Slots + 1)
	if _, ok := collector.Snapshot("p1", at(11)); ok {
		t.Fatal("old company survived new round")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe outlived collector")
	}
}
func TestSimultaneousSlicesShareOneProbeLimit(t *testing.T) {
	inv := &fakeInventory{profile: "p", targets: manyTargets(200), ok: true}
	prober := &fakeProber{outcome: Reply, hold: time.Millisecond}
	collector := NewCollector(inv, prober, 4)
	collector.refresh(context.Background())
	var wg sync.WaitGroup
	for slot := 1; slot <= 3; slot++ {
		wg.Add(1)
		go func(slot int) { defer wg.Done(); collector.Step(context.Background(), slot, at(slot)) }(slot)
	}
	wg.Wait()
	if got := prober.maxRunning.Load(); got > 4 || got == 0 {
		t.Fatal("global limit violated", got)
	}
}
