package routerssh

import (
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"os"
	"strings"
	"testing"
)

func TestRadioPartialReasons(t *testing.T) {
	b, e := os.ReadFile("testdata/l07_ap_v6_real.txt")
	if e != nil {
		t.Fatal(e)
	}
	raw := string(b)
	cases := []struct{ name, old, new, reason string }{
		{"start", "L07-CPU-BEFORE", "missing", "markers"},
		{"end", "L07-DONE", "missing", "markers"},
		{"cpu", "cpu-load: 6%", "cpu-load: invalid", "cpu"},
		{"identity", "02:00:00:00:10:06", "invalid", "interface-identity"},
		{"monitor-count", "L07-INTERFACE=Virtual-p03", "wrong=Virtual-p03", "monitor-count"},
		{"status", "running-ap", "stopped", "monitor-status"},
		{"registered", "registered-clients: 5", "registered-clients: invalid", "client-count"},
		{"client-mac", "mac-address=02:00:00:00:10:03", "mac-address=invalid", "client-identity"},
		{"trailing", "L07-DONE", "L07-DONE\nextra", "markers"},
		{"interface-empty", " 0  R name=", "bad name=", "interface-count"},
		{"interface-name", `name="Virtual-p03"`, `wrong="Virtual-p03"`, "interface-identity"},
		{"interface-duplicate", `name="wlan1"`, `name="Virtual-p03"`, "interface-identity"},
		{"interface-metric", "frequency=5215", "frequency=invalid", "interface-metric"},
		{"monitor-identity", "L07-INTERFACE=wlan1", "L07-INTERFACE=unknown", "monitor-identity"},
		{"power", "6Mbps:27(27/30)", "6Mbps:99(27/30)", "monitor-metric"},
		{"noise", "noise-floor: -107dBm", "noise-floor: -999dBm", "monitor-metric"},
		{"client-parse", " 0 interface=Virtual-p03", " 0 interface=Virtual-p03 interface=duplicate", "client-parse"},
		{"client-metric", "signal-strength=-66dBm", "signal-strength=-999dBm", "client-metric"},
		{"master", "master-interface=wlan1", "master-interface=unknown", "master-interface"},
		{"registration-count", "registered-clients: 5", "registered-clients: 0", "registration-count"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(raw, tc.old, tc.new, 1)
			if bad == raw {
				t.Fatalf("mutation absent: %s", tc.old)
			}
			r, c := ParseRadioAP(bad)
			if c != equipment.Partial {
				t.Fatalf("code=%s", c)
			}
			b, _ := json.Marshal(r)
			var got struct {
				Failure struct {
					Reason string `json:"reason"`
					Bytes  int    `json:"bytes"`
				} `json:"failure"`
			}
			_ = json.Unmarshal(b, &got)
			if got.Failure.Reason != tc.reason || got.Failure.Bytes != len(strings.ReplaceAll(bad, "\r", "")) {
				t.Fatalf("missing structural reason: %s want=%s", b, tc.reason)
			}
		})
	}
}
