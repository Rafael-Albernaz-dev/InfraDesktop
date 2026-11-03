package equipment

import (
	"math"
	"testing"
	"time"
)

func TestRadioAllMeasurementSamplesAndCoverageRequired(t *testing.T) {
	at := time.Now()
	samples := make([]RadioMeasurement, 10)
	for i := range samples {
		samples[i] = RadioMeasurement{AP: RadioTiming{CommandSeconds: .4,
			PathSeconds: 1},
			CPE: RadioTiming{CommandSeconds: .4,
				PathSeconds: 1},
			Seconds:  1,
			Started:  at,
			Ended:    at.Add(time.Second),
			Complete: true,
			Load: RadioLoad{CPU: []int{6,
				7,
				6},
				PingSent:     6,
				PingReceived: 6,
				PingRTT: []float64{1,
					1,
					1,
					1,
					1,
					1},
				Started: at.Add(-time.Second),
				Ended:   at.Add(2 * time.Second)}}
	}
	if !AssessRadioMeasurements(samples).Enabled {
		t.Fatal("complete safe measurement rejected")
	}
	samples[0].Complete = false
	if AssessRadioMeasurements(samples).Enabled {
		t.Fatal("earlier failed side hidden by healthy tenth sample")
	}
	samples[0].Complete = true
	samples[3].Load.Ended = samples[3].Load.Started.Add(-time.Nanosecond)
	if AssessRadioMeasurements(samples).Enabled {
		t.Fatal("invalid ICMP observation timestamps accepted")
	}
	samples[3].Load.Ended = at.Add(2 * time.Second)
	samples[8].Load.PingReceived = 5
	if AssessRadioMeasurements(samples).Enabled {
		t.Fatal("ping loss hidden")
	}
	samples[8].Load.PingReceived = 6
	samples[2].Load.CPU = nil
	if AssessRadioMeasurements(samples).Enabled {
		t.Fatal("missing CPU accepted")
	}
	samples[2].Load.CPU = []int{6, 7, 6}
	samples[4].Load.PingRTT[0] = math.NaN()
	if AssessRadioMeasurements(samples).Enabled {
		t.Fatal("invalid baseline accepted")
	}
	samples[4].Load.PingRTT[0] = 1
	samples[5].Load.PingRTT = []float64{1, 3, 3, 1, 1, 1}
	if AssessRadioMeasurements(samples).Enabled {
		t.Fatal("two pings over twice baseline accepted")
	}
}
