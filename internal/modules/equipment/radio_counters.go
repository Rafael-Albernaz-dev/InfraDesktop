package equipment

import (
	"time"
)

type RadioTickReceipt struct {
	Failure *RadioFailure `json:"failure,omitempty"`
	At      string        `json:"observed_at"`
	CPU     []int         `json:"cpu"`
	AP      float64       `json:"ap_command_seconds"`
	CPE     float64       `json:"cpe_command_seconds"`
	ICMP    []float64     `json:"icmp_rtt_ms"`
	Reason  string        `json:"reason"`
}

// Counters are decimal strings so a uint64 is never rounded by JSON/JS.
// Only consecutive ticks in this observation form a delta. Missing/reset
// counters produce explicit qualities, never a negative rate or invented zero.
func RadioCounterMetrics(previous, next *RadioReading) {
	if next == nil {
		return
	}
	seconds := radioCounterSeconds(previous, next)
	if next.Perspective == "CPE" {
		var before map[string]string
		if previous != nil {
			before = previous.Counters
		}
		appendRadioCounterMetrics(before, next.Counters, &next.Metrics, "CPE → AP", "AP → CPE", seconds)
	}
	for i := range next.Clients {
		client := &next.Clients[i]
		before := previousRadioClientCounters(previous, *client)
		appendRadioCounterMetrics(before, client.Counters, &client.Metrics, "AP → CPE", "CPE → AP", seconds)
	}
}

func radioCounterSeconds(previous, next *RadioReading) float64 {
	if previous == nil {
		return 0
	}
	before, beforeError := time.Parse(time.RFC3339Nano, previous.At)
	after, afterError := time.Parse(time.RFC3339Nano, next.At)
	if beforeError != nil || afterError != nil {
		return 0
	}
	return after.Sub(before).Seconds()
}
