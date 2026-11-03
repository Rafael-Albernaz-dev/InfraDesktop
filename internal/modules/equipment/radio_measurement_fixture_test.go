package equipment

import (
	"testing"
	"time"
)

func liveSamples(t *testing.T) []RadioMeasurement {
	t.Helper()
	at := time.Now()
	samples := make([]RadioMeasurement, 10)
	for i := range samples {
		samples[i].AP = RadioTiming{CommandSeconds: .4, PathSeconds: 2}
		samples[i].CPE = RadioTiming{CommandSeconds: .4, PathSeconds: 3}
		samples[i].Seconds = 5
		samples[i].Started = at
		samples[i].Ended = at.Add(5 * time.Second)
		samples[i].Complete = true
		samples[i].Load = RadioLoad{Started: at.Add(-time.Second),
			Ended: at.Add(6 * time.Second),
			CPU: []int{6,
				7,
				6},
			PingSent:     7,
			PingReceived: 7,
			PingRTT: []float64{1,
				1,
				1,
				1,
				1,
				1,
				1}}
	}
	return samples
}
