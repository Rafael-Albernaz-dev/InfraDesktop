package equipment

import "testing"

func assertRadioCounterDeltas(t *testing.T, metrics []RadioMetric) {
	t.Helper()
	found := map[string]bool{}
	for _, m := range metrics {
		found[m.Name] = true
		switch m.Name {
		case "tx_throughput":
			if m.Quality != "observed" || m.Value != 800 || m.Direction != "CPE → AP" {
				t.Fatal(m)
			}
		case "rx_throughput":
			if m.Quality != "reset" {
				t.Fatal("decrease became rate", m)
			}
		case "tx_packets_delta":
			if m.Value != 3 || m.Quality != "observed" {
				t.Fatal(m)
			}
		case "tx_retries_delta":
			if m.Quality != "unavailable" {
				t.Fatal("missing became zero", m)
			}
		}
	}
	for _, name := range []string{"tx_throughput", "rx_throughput", "tx_packets_delta", "tx_retries_delta"} {
		if !found[name] {
			t.Fatal("required counter metric omitted", name)
		}
	}
}
