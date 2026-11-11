package radioparse

import (
	"infradesktop/internal/modules/equipment"
	"os"
	"testing"
)

func TestRouterOSBridgeNV2EmptyRegistrationPreservesUnavailableClients(t *testing.T) {
	raw, err := os.ReadFile("testdata/radio_routeros_lhg_bridge_nv2.txt")
	if err != nil {
		t.Fatal(err)
	}
	reading, code := ParseRadioAP(string(raw))
	if code != equipment.OK || reading == nil {
		t.Fatalf("code=%s reading=%+v", code, reading)
	}
	if reading.Family != equipment.RadioRouterOSWirelessV6 || reading.Version != "6.49.13 (long-term)" {
		t.Fatalf("identity=%+v", reading)
	}
	if len(reading.Clients) != 0 || len(reading.Interfaces) != 1 {
		t.Fatalf("interfaces/clients=%+v", reading)
	}
	if len(reading.CPU) != 3 {
		t.Fatalf("CPU=%v", reading.CPU)
	}
	for _, cpu := range reading.CPU {
		if cpu != 14 {
			t.Fatalf("CPU=%v", reading.CPU)
		}
	}
	if reading.Interfaces[0].BSSID != "02:00:00:00:10:10" {
		t.Fatal("synthetic identity not preserved")
	}
}
