package equipment

import (
	"strings"
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
		samples[i].Load = RadioLoad{Started: at.Add(-time.Second), Ended: at.Add(6 * time.Second), CPU: []int{6, 7, 6}, PingSent: 7, PingReceived: 7, PingRTT: []float64{1, 1, 1, 1, 1, 1, 1}}
	}
	return samples
}
func TestL07LiveCommandGateAndReasons(t *testing.T) {
	for _, tc := range []struct{ name, reason string }{
		{"healthy", ""}, {"AP slow", "Comando do AP"}, {"CPE slow", "Comando da CPE"},
		{"read failed", "Amostra de leitura falhou"}, {"CPU", "CPU do AP chegou"}, {"ping", "Ping degradou"}, {"loss", "Ping perdido"}, {"coverage", "Carga incompleta"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := liveSamples(t)
			switch tc.name {
			case "AP slow":
				s[0].AP = RadioTiming{CommandSeconds: 3.2, PathSeconds: 4}
			case "CPE slow":
				s[0].CPE = RadioTiming{CommandSeconds: 3.2, PathSeconds: 4}
			case "read failed":
				s[0].Complete = false
			case "CPU":
				s[0].Load.CPU = []int{6, 91, 6}
			case "ping":
				s[0].Load.PingRTT = []float64{1, 3, 3, 1, 1, 1, 1}
			case "loss":
				s[0].Load.PingReceived = 6
				s[0].Load.PingRTT = s[0].Load.PingRTT[:6]
			case "coverage":
				s[0].Load.Ended = s[0].Started
			}
			c := AssessRadioMeasurements(s)
			if tc.reason == "" {
				if !c.Enabled || c.P95 != 5 || c.Interval != 15 {
					t.Fatalf("healthy 0.4s commands / 5s path rejected: %+v", c)
				}
			} else if c.Enabled || !strings.Contains(c.Reason, tc.reason) {
				t.Fatalf("specific first reason %q lost: %+v", tc.reason, c)
			}
		})
	}
}
func TestL07LiveLimitsUseCommandTiming(t *testing.T) {
	rtt := 1.0
	r := &RadioReading{Seconds: 5, CPU: []int{6, 7, 6}, PingRTT: &rtt}
	r.CommandSeconds = .4
	c := RadioCost{CPUBaseline: 6, PingLimit: 2}
	if RadioLiveLimit(r, c) {
		t.Fatal("full path incorrectly aborts healthy live")
	}
	r.CommandSeconds = 3.2
	if !RadioLiveLimit(r, c) {
		t.Fatal("slow command accepted")
	}
}

func TestL07LiveCPUAbsoluteLimitKeepsSpecificFailure(t *testing.T) {
	s := liveSamples(t)
	for i := range s {
		s[i].Load.CPU = []int{10, 10, 10}
	}
	s[0].Load.CPU = []int{0, 91, 0}
	c := AssessRadioMeasurements(s)
	if c.Enabled || !strings.Contains(c.Reason, "CPU do AP chegou a 91%") {
		t.Fatal("absolute limit was not enforced", c.Reason)
	}
	s[1].Load.CPU = nil
	c = AssessRadioMeasurements(s)
	if c.Enabled || !strings.Contains(c.Reason, "CPU do AP chegou a 91%") {
		t.Fatal("another missing CPU hid observed91", c.Reason)
	}
	s[1].Load.CPU = []int{10, 10, 10}
	// An incomplete observer must not erase the measured, unsafe CPU increase.
	s[0].Load.Ended = time.Time{}
	c = AssessRadioMeasurements(s)
	if c.Enabled || !strings.Contains(c.Reason, "CPU do AP chegou a 91%") {
		t.Fatal(c.Reason)
	}
	s = liveSamples(t)
	s[0].Load.PingReceived = 6
	s[0].Load.PingRTT = s[0].Load.PingRTT[:6]
	s[0].Load.Ended = time.Time{}
	c = AssessRadioMeasurements(s)
	if c.Enabled || !strings.Contains(c.Reason, "Ping perdido") {
		t.Fatal(c.Reason)
	}
}
