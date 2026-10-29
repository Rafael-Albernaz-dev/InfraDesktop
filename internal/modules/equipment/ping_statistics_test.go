package equipment

import (
	"math"
	"testing"
)

func TestL06T3JitterMedianAndLostProbe(t *testing.T) {
	a, b, c := int64(1000000), int64(3000000), int64(2000000)
	p := &PDPingResult{Sent: 3, Probes: []PDPingProbe{{Sequence: 0, RTT: &a}, {Sequence: 1, RTT: &b}, {Sequence: 2, RTT: &c}}}
	if got := PingJitter(p); got == nil || *got != 1500000 {
		t.Fatal("mean absolute consecutive differences", got)
	}
	p.Probes[1].RTT = nil
	if PingJitter(p) != nil {
		t.Fatal("lost probe connected nonconsecutive RTTs")
	}
	p.Probes[1].RTT = &a
	p.Probes[2].RTT = &a
	if got := PingJitter(p); got == nil || *got != 0 {
		t.Fatal("legitimate zero jitter lost")
	}
	h := []int64{10, 2, 9, 3, 8, 4, 7, 5, 6, 1}
	if got := PingMedianTen(h); got == nil || *got != 5 {
		t.Fatal("median10", got)
	}
	if PingMedianTen(h[:9]) != nil {
		t.Fatal("fewer than10 claimed baseline")
	}
	if h[0] != 10 {
		t.Fatal("median mutated source")
	}
	max := int64(math.MaxInt64)
	if got := PingMedianTen([]int64{max, max, max, max, max, max, max, max, max, max}); got == nil || *got != max {
		t.Fatal("median overflow")
	}
}
