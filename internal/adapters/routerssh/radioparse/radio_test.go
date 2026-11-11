package radioparse

import (
	"infradesktop/internal/modules/equipment"
	"os"
	"strings"
	"testing"
)

func TestRadioRealGoldens(t *testing.T) {
	assertRadioAPGolden(t)
	assertRadioAirOSGolden(t)
	assertRadioWOMGolden(t)
	assertRadioCPEFooterRequired(t)
}

// Captured again in the installed pilot: RouterOS wraps strength-at-rates
// after HT40-, leaving a deeply indented numeric continuation (2 470ms).
func TestRadioAPWrappedRateContinuation(t *testing.T) {
	b, err := os.ReadFile("testdata/l07_ap_v6_wrap_real.txt")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "                  2 470ms,") {
		t.Fatal("causal numeric continuation absent")
	}
	reading, code := ParseRadioAP(text)
	if code != equipment.OK || reading == nil || len(reading.Clients) != 6 || len(reading.Interfaces) != 2 {
		t.Fatalf("complete wrapped AP rejected: %s", code)
	}
	found := false
	for _, client := range reading.Clients {
		if client.MAC == "02:00:00:00:10:03" {
			found = client.Interface == "wlan1" && client.BSSID == "02:00:00:00:10:05"
		}
	}
	if !found {
		t.Fatal("legitimate AP/CPE association lost")
	}
	for _, malformed := range []string{
		strings.Replace(text, " 5 interface=wlan1", " 5 missing-interface=wlan1", 1),
		strings.Replace(text, "mac-address=02:00:00:00:10:03", "mac-address=invalid", 1),
		strings.Replace(text, "L07-DONE", "", 1),
	} {
		if _, code := ParseRadioAP(malformed); code == equipment.OK {
			t.Fatal("malformed real record/truncation accepted")
		}
	}
}
