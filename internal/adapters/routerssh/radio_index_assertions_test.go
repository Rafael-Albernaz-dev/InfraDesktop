package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func assertRadioIndexRejectsMalformedIdentity(t *testing.T, wire string) {
	t.Helper()
	for _, bad := range []string{strings.Replace(wire,
		"last-ip=0.0.0.0",
		"last-ip=invalid",
		1),
		strings.Replace(wire,
			"bssid=AA:BB:CC:DD:EE:00",
			"bssid=invalid",
			1),
		strings.Replace(wire,
			"L07-INDEX-DONE",
			"",
			1)} {
		if _, c := ParseRadioIndex(bad); c != equipment.Partial {
			t.Fatal("malformed index identity accepted", c)
		}
	}
}
func assertRadioIndexCommandProjection(t *testing.T) {
	t.Helper()
	if !strings.Contains(RadioIndexCommand,
		"registration-table print stats as-value") ||
		strings.Contains(RadioIndexCommand,
			"monitor") ||
		strings.Contains(RadioIndexCommand,
			"/system resource") {
		t.Fatal("index drifted from a single registration-table projection")
	}
}
