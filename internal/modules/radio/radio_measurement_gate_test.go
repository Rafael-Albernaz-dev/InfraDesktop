package radio

import (
	"math"
	"testing"
)

func TestRadioSpikesAndCommandInterval(t *testing.T) {
	for _, tc := range []struct {
		cpu     []int
		enabled bool
	}{{[]int{91, 50}, true}, {[]int{91, 92}, false}, {[]int{90, 90}, true}, {[]int{100, 100}, false}} {
		s := liveSamples(t)
		s[0].Load.CPU = tc.cpu
		got := AssessRadioMeasurements(s)
		if got.Enabled != tc.enabled {
			t.Errorf("CPU%v enabled=%v reason=%s", tc.cpu, got.Enabled, got.Reason)
		}
		if got.P95 != .4 || math.Abs(got.Interval-1.2) > 1e-9 {
			t.Errorf("interval includes handshakes: p95=%v interval=%v", got.P95, got.Interval)
		}
	}
}
