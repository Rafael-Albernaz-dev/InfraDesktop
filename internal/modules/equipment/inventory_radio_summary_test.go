package equipment

import "testing"

func TestRadioSummaryRetainsObservedValuesWithoutIdentities(t *testing.T) {
	r := RadioReading{At: "2026-11-05T10:00:00Z", Version: "6.49", Model: "synthetic", DeviceUptime: "1d",
		CPU: []int{3, 7}, Perspective: "AP", Clients: []RadioClient{{MAC: "02:00:00:00:00:01"}},
		Metrics: []RadioMetric{{Name: "memory", Value: 35, Quality: "observed"},
			{Name: "voltage", Value: 24, Quality: "observed"}, {Name: "temperature", Value: 39, Quality: "observed"}}}
	s := ProjectRadioSummary(r)
	if s.At != r.At || s.Version != r.Version || s.Model != r.Model || s.Uptime != r.DeviceUptime ||
		s.CPU == nil || *s.CPU != 7 || s.Clients == nil || *s.Clients != 1 {
		t.Fatal("radio summary lost observed values", s)
	}
	assertRadioSensors(t, s)
	empty := ProjectRadioSummary(RadioReading{Perspective: "CPE"})
	if empty.CPU != nil || empty.Clients != nil || empty.Memory != nil || empty.Voltage != nil || empty.Temperature != nil {
		t.Fatal("unknown radio values must stay absent", empty)
	}
}

func assertRadioSensors(t *testing.T, s *RadioSummary) {
	t.Helper()
	if s.Memory == nil || *s.Memory != 35 || s.Voltage == nil || *s.Voltage != 24 ||
		s.Temperature == nil || *s.Temperature != 39 {
		t.Fatal("radio summary lost observed sensors", s)
	}
}
