package radioparse

import "infradesktop/internal/modules/equipment"

var radioAPClientMetricFields = []struct{ k, n, u, d string }{
	{"signal-strength", "signal", "dBm", "CPE → AP"},
	{"signal-strength-ch0", "chain0", "dBm", "CPE → AP"},
	{"signal-strength-ch1", "chain1", "dBm", "CPE → AP"},
	{"signal-to-noise", "snr", "dB", "CPE → AP"},
	{"tx-signal-strength", "remote_signal", "dBm", "AP → CPE"},
	{"tx-ccq", "ccq_tx", "%", "AP → CPE"}, {"rx-ccq", "ccq_rx", "%", "CPE → AP"},
	{"tx-rate", "rate_tx", "Mbps", "AP → CPE"}, {"rx-rate", "rate_rx", "Mbps", "CPE → AP"},
}

// Missing optional fields are absent; malformed fields cannot discard CPU or peer identity.
func addRadioAPTickMetrics(metrics *[]equipment.RadioMetric, fields map[string]string, size int) *equipment.RadioFailure {
	var first *equipment.RadioFailure
	for _, field := range radioAPClientMetricFields {
		raw, exists := fields[field.k]
		if !exists {
			continue
		}
		metric, valid := radioMetric(raw, field.n, field.u, field.d, 1)
		if valid {
			*metrics = append(*metrics, metric)
		} else if first == nil {
			first = &equipment.RadioFailure{Reason: "tick-metric:" + field.k, Bytes: size}
		}
	}
	return first
}
