package radio

import "testing"

func TestRadioCounterRateBaselineResetAndUnavailable(t *testing.T) {
	at := "2026-10-30T00:00:00Z"
	nextAt := "2026-10-30T00:00:02Z"
	base := &RadioReading{Perspective: "CPE",
		At: at,
		Counters: map[string]string{"tx_bytes": "9007199254740993",
			"rx_bytes":   "100",
			"tx_packets": "20"}}
	first := *base
	RadioCounterMetrics(nil, &first)
	if first.Metrics[0].Quality != "baseline" {
		t.Fatal("first rate fabricated", first.Metrics)
	}
	next := &RadioReading{Perspective: "CPE",
		At: nextAt,
		Counters: map[string]string{"tx_bytes": "9007199254741193",
			"rx_bytes":   "50",
			"tx_packets": "23"}}
	RadioCounterMetrics(base, next)
	assertRadioCounterDeltas(t, next.Metrics)
	base.Counters["uptime_seconds"] = "100"
	next.Counters["uptime_seconds"] = "1"
	next.Metrics = nil
	RadioCounterMetrics(base, next)
	for _, m := range next.Metrics {
		if m.Name == "tx_throughput" && m.Quality != "reset" {
			t.Fatal("restarted uptime created burst from growing counters", m)
		}
	}
}
