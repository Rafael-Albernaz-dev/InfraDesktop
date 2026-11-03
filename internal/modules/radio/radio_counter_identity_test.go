package radio

import "testing"

func TestRadioCounterIdentityAndInvalidTimePreserveBaseline(t *testing.T) {
	for _, tc := range []struct {
		name, quality string
		value         float64
	}{
		{"same", "observed", 400}, {"other BSSID", "baseline", 0},
		{"invalid time", "baseline", 0}, {"last duplicate", "observed", 720},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := RadioClient{MAC: "02:00:00:00:10:01", Interface: "wlan1", BSSID: "02:00:00:00:10:02",
				Counters: map[string]string{"tx_bytes": "100"}}
			before := &RadioReading{At: "2026-10-30T00:00:00Z", Clients: []RadioClient{peer}}
			after := &RadioReading{At: "2026-10-30T00:00:02Z", Clients: []RadioClient{peer}}
			after.Clients[0].Counters = map[string]string{"tx_bytes": "200"}
			if tc.name == "other BSSID" {
				after.Clients[0].BSSID = "02:00:00:00:10:03"
			}
			if tc.name == "invalid time" {
				after.At = "invalid"
				after.Clients[0].Counters["tx_bytes"] = "50"
			}
			if tc.name == "last duplicate" {
				duplicate := peer
				duplicate.Counters = map[string]string{"tx_bytes": "20"}
				before.Clients = append(before.Clients, duplicate)
			}
			RadioCounterMetrics(before, after)
			found := false
			for _, metric := range after.Clients[0].Metrics {
				if metric.Name != "tx_throughput" {
					continue
				}
				found = true
				if metric.Quality != tc.quality || metric.Value != tc.value {
					t.Fatalf("metric=%+v expected=%+v", metric, tc)
				}
			}
			if !found {
				t.Fatal("throughput metric missing")
			}
		})
	}
}
