package equipment

import (
	"testing"
	"time"
)

func TestRadioAPOnlyStillNeedsCompleteAPCPUAndPing(t *testing.T) {
	now := time.Now()
	samples := make([]RadioMeasurement, 10)
	for i := range samples {
		samples[i] = RadioMeasurement{APOnly: true,
			Seconds: .5,
			AP: RadioTiming{CommandSeconds: .4,
				PathSeconds: .5},
			Complete: true,
			Load: RadioLoad{CPU: []int{50},
				Started:      now,
				Ended:        now.Add(time.Second),
				PingSent:     1,
				PingReceived: 1,
				PingRTT:      []float64{1}}}
	}
	if c := AssessRadioMeasurements(samples); !c.Enabled || !c.APOnly {
		t.Fatal("valid AP-only cost rejected", c.Reason)
	}
	samples[9].Load.CPU = nil
	if c := AssessRadioMeasurements(samples); c.Enabled {
		t.Fatal("AP-only bypassed CPU completeness")
	}
	samples[9].Load.CPU = []int{50}
	samples[9].Load.PingReceived = 0
	samples[9].Load.PingRTT = nil
	if c := AssessRadioMeasurements(samples); c.Enabled {
		t.Fatal("AP-only bypassed ping completeness")
	}
}
