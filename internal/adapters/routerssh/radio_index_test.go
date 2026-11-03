package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestRadioIndexOptionalIPAndStrictIdentity(t *testing.T) {
	wire := ("L07-INDEX\n 0 mac-address=AA:BB:CC:DD:EE:01 interface=wlan1 bssid=AA:BB:CC:DD:" +
		"EE:00 last-ip=0.0.0.0 signal-strength=-60\nL07-INDEX-DONE\n")
	entries, c := ParseRadioIndex(wire)
	if c != equipment.OK || len(entries) != 1 || entries[0].LastIP != "" || entries[0].MAC != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("absent IP erased MAC evidence: %+v %s", entries, c)
	}
	assertRadioIndexRejectsMalformedIdentity(t, wire)
	assertRadioIndexCommandProjection(t)
}
