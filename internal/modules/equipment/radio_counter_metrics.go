package equipment

import "strconv"

func previousRadioClientCounters(previous *RadioReading, client RadioClient) map[string]string {
	var before map[string]string
	if previous == nil {
		return before
	}
	for _, old := range previous.Clients {
		if old.MAC == client.MAC && old.Interface == client.Interface && old.BSSID == client.BSSID {
			before = old.Counters
		}
	}
	return before
}

func appendRadioCounterMetrics(before, after map[string]string, metrics *[]RadioMetric, tx, rx string, seconds float64) {
	restarted := radioCountersRestarted(before, after)
	for _, key := range []string{"tx_bytes", "rx_bytes", "tx_packets", "rx_packets", "tx_frames", "rx_frames",
		"tx_hw_frames", "rx_hw_frames", "tx_retries", "tx_failed", "rx_errors", "tx_errors"} {
		*metrics = append(*metrics, radioCounterMetric(before, after, key, tx, rx, seconds, restarted))
	}
}

func radioCountersRestarted(before, after map[string]string) bool {
	old, oldError := strconv.ParseUint(before["uptime_seconds"], 10, 64)
	next, nextError := strconv.ParseUint(after["uptime_seconds"], 10, 64)
	return oldError == nil && nextError == nil && next < old
}

func radioCounterMetric(before, after map[string]string, key, tx, rx string, seconds float64, restarted bool) RadioMetric {
	metric := radioCounterDescriptor(key, tx, rx)
	value, err := strconv.ParseUint(after[key], 10, 64)
	if err != nil {
		return metric
	}
	metric.Quality = "baseline"
	old, err := strconv.ParseUint(before[key], 10, 64)
	if err != nil || !(seconds > 0) {
		return metric
	}
	if value < old || restarted {
		metric.Quality = "reset"
		return metric
	}
	metric.Value = float64(value - old)
	if metric.Unit == "bps" {
		metric.Value = metric.Value * 8 / seconds
	}
	metric.Quality = "observed"
	return metric
}

func radioCounterDescriptor(key, tx, rx string) RadioMetric {
	metric := RadioMetric{Name: key + "_delta", Unit: "packets", Direction: tx, Quality: "unavailable"}
	switch key {
	case "tx_frames", "rx_frames", "tx_hw_frames", "rx_hw_frames":
		metric.Unit = "frames"
	case "tx_bytes", "rx_bytes":
		metric.Unit = "bps"
		metric.Name = key[:2] + "_throughput"
	}
	if key[:2] == "rx" {
		metric.Direction = rx
	}
	return metric
}
