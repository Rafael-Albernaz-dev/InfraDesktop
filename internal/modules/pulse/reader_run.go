package pulse

import (
	"context"
	"sync"
	"time"
)

func (c *ReadCollector) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	c.runReadTicks(ctx, ticker.C)
}
func (c *ReadCollector) runReadTicks(ctx context.Context, ticks <-chan time.Time) {
	var workers sync.WaitGroup
	defer workers.Wait()
	defer c.Close()
	done := make(chan int, ReadSlots)
	busy := [ReadSlots]bool{}
	launch := func(second int, at time.Time) {
		if second == 0 {
			c.refreshRead(ctx)
		}
		if busy[second] || ctx.Err() != nil {
			return
		}
		busy[second] = true
		workers.Add(1)
		go func() {
			defer workers.Done()
			c.readSlot(ctx, second, at)
			done <- second
		}()
	}
	second := 0
	launch(second, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-done:
			busy[s] = false
		case at := <-ticks:
			second = (second + 1) % ReadSlots
			launch(second, at)
		}
	}
}
