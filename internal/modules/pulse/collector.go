package pulse

import (
	"context"
	"sync"
	"time"
)

const Slots = 10

func Slot(id string) int {
	// FNV-1a keeps scheduling stable across daemon restarts without runtime state.
	var hash uint32 = 2166136261
	for i := 0; i < len(id); i++ {
		hash = (hash ^ uint32(id[i])) * 16777619
	}
	return int(hash % Slots)
}

type Collector struct {
	mu        sync.Mutex
	inventory Inventory
	prober    Prober
	slots     chan struct{}
	profile   string
	targets   []Target
	tracker   *Tracker
}

func NewCollector(inv Inventory, prober Prober, limit int) *Collector {
	return &Collector{inventory: inv, prober: prober, slots: make(chan struct{}, max(1, min(16, limit))), tracker: NewTracker()}
}
func (c *Collector) refresh(ctx context.Context) {
	profile, targets, ok := c.inventory.Targets(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !ok {
		profile, targets = "", nil
	}
	if profile != c.profile {
		c.tracker = NewTracker()
	}
	c.profile, c.targets = profile, targets
	c.tracker.Sync(targets)
}
func (c *Collector) Step(ctx context.Context, second int, now time.Time) {
	if ctx.Err() != nil {
		return
	}
	if second == 0 {
		c.refresh(ctx)
	}
	c.probeSlot(ctx, second, func() time.Time { return now })
}
func (c *Collector) probeSlot(ctx context.Context, second int, now func() time.Time) {
	c.mu.Lock()
	targets := append([]Target(nil), c.targets...)
	tracker := c.tracker
	c.mu.Unlock()
	var wg sync.WaitGroup
	for _, target := range targets {
		if Slot(target.ID) != second {
			continue
		}
		select {
		case c.slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		if ctx.Err() != nil {
			<-c.slots
			break
		}
		wg.Add(1)
		go func(target Target) {
			defer wg.Done()
			defer func() { <-c.slots }()
			outcome := c.prober.Probe(ctx, target.Address)
			if ctx.Err() == nil {
				tracker.Record(target, outcome, now())
			}
		}(target)
	}
	wg.Wait()
}
func (c *Collector) Snapshot(profile string, now time.Time) (Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if profile == "" || profile != c.profile {
		return Snapshot{}, false
	}
	return c.tracker.Snapshot(now), true
}
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	c.runTicks(ctx, ticker.C)
}
func (c *Collector) runTicks(ctx context.Context, ticks <-chan time.Time) {
	var workers sync.WaitGroup
	defer workers.Wait()
	done := make(chan int, Slots)
	busy := [Slots]bool{}
	launch := func(second int) {
		if second == 0 {
			c.refresh(ctx)
		}
		if busy[second] || ctx.Err() != nil {
			return
		}
		busy[second] = true
		workers.Add(1)
		go func() {
			defer workers.Done()
			c.probeSlot(ctx, second, time.Now)
			done <- second
		}()
	}
	second := 0
	launch(second)
	for {
		select {
		case <-ctx.Done():
			return
		case slot := <-done:
			busy[slot] = false
		case <-ticks:
			second = (second + 1) % Slots
			// At most one pending slice per slot: saturation never creates an unbounded queue.
			launch(second)
		}
	}
}
