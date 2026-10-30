package equipment

import (
	"strings"
	"testing"
)

func TestL07CPU90GateAndTickBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cpu     []int
		allowed bool
	}{
		{"observed I1", []int{26, 57, 30}, true}, {"observed U1", []int{36, 59, 30}, true}, {"exact90", []int{6, 90, 7}, true},
		{"above90", []int{6, 91, 7}, false}, {"baseline above90", []int{91, 80, 70}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			samples := liveSamples(t)
			samples[0].Load.CPU = tc.cpu
			cost := AssessRadioMeasurements(samples)
			if cost.Enabled != tc.allowed {
				t.Errorf("CPU%v allowed=%v got enabled=%v reason=%s", tc.cpu, tc.allowed, cost.Enabled, cost.Reason)
			}
			if !tc.allowed && !strings.Contains(cost.Reason, "limite 90%") {
				t.Error("missing measured CPU limit", cost.Reason)
			}
			ping := 1.0
			reading := &RadioReading{CommandSeconds: .4, Seconds: 5, CPU: tc.cpu, PingRTT: &ping}
			if violation := RadioLiveLimit(reading, RadioCost{CPUBaseline: 6, PingLimit: 2}); violation == tc.allowed {
				t.Fatal("live CPU boundary disagrees", tc.cpu, violation)
			}
		})
	}
}
