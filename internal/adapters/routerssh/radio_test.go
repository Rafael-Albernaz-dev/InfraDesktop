package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"os"
	"strings"
	"testing"
)

func TestL07RadioRealGoldens(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	apText := read("l07_ap_v6_real.txt")
	ap, c := ParseRadioAP(apText)
	if c != equipment.OK || ap == nil || ap.Version != "6.49.10 (long-term)" || len(ap.Interfaces) != 2 || len(ap.Clients) != 6 || len(ap.CPU) != 3 {
		t.Fatalf("AP: %+v %s", ap, c)
	}
	if ap.Interfaces[0].Name != "Virtual-p03" || ap.Interfaces[0].Master != "wlan1" || ap.Interfaces[0].BSSID != "02:00:00:00:10:06" || ap.Interfaces[0].Clients == nil || *ap.Interfaces[0].Clients != 1 {
		t.Fatal("real virtual AP lost")
	}
	if !strings.Contains(ap.Interfaces[1].Power, "HT40-7:23(23/26)") || len(ap.Interfaces[1].PowerMetrics) == 0 {
		t.Fatal("continued power table lost", ap.Interfaces[1].Power)
	}
	if ap.Clients[4].MAC != "02:00:00:00:10:03" || ap.Clients[4].BSSID != "02:00:00:00:10:05" || ap.Clients[4].Uptime != "1d22h27m58s" {
		t.Fatal("WOM/AP association lost")
	}
	metrics := map[string]float64{}
	for _, m := range ap.Clients[4].Metrics {
		metrics[m.Name] = m.Value
	}
	if metrics["signal"] != -70 || metrics["chain0"] != -72 || metrics["chain1"] != -71 || metrics["snr"] != 37 || metrics["rate_tx"] != 60 || metrics["ccq_tx"] != 100 {
		t.Fatal(metrics)
	}
	if _, invented := metrics["ccq_rx"]; invented {
		t.Fatal("missing AP rx CCQ invented")
	}
	for _, bad := range []string{strings.Replace(apText, "L07-DONE", "", 1), strings.Replace(apText, "registered-clients: 5", "registered-clients: 0", 1), strings.Replace(apText, "02:00:00:00:10:06", "invalid", 1), apText + "\ntrailing"} {
		if _, c := ParseRadioAP(bad); c == equipment.OK {
			t.Fatal("incomplete/inconsistent AP accepted")
		}
	}
	u, c := ParseRadioAirOS(read("l07_airos_xm_real.txt"))
	if c != equipment.OK || u == nil || u.APBSSID != "02:00:00:00:10:05" || u.WirelessMAC != "02:00:00:00:10:01" || u.WirelessUptime != "12679s" {
		t.Fatalf("airOS %+v %s", u, c)
	}
	metrics = map[string]float64{}
	for _, m := range u.Metrics {
		metrics[m.Name] = m.Value
	}
	if metrics["signal"] != -62 || metrics["noise"] != -88 || metrics["ccq"] != 98.8 || metrics["rate_tx"] != 45 || metrics["rate_rx"] != 60 {
		t.Fatal(metrics)
	}
	w, c := ParseRadioWOM(read("l07_wom_iw_real.txt"))
	if c != equipment.OK || w == nil || w.APBSSID != "02:00:00:00:10:05" || w.WirelessMAC != "02:00:00:00:10:03" || w.WirelessUptime != "" {
		t.Fatalf("WOM %+v %s", w, c)
	}
	metrics = map[string]float64{}
	for _, m := range w.Metrics {
		metrics[m.Name] = m.Value
	}
	if metrics["signal"] != -55 || metrics["chain0"] != -58 || metrics["ccq"] != 100 || metrics["rate_tx"] != 60 || metrics["rate_rx"] != 60 {
		t.Fatal(metrics)
	}
	if _, invented := metrics["noise"]; invented {
		t.Fatal("invalid /proc noise -256 became a measurement")
	}
	for _, test := range []struct {
		file  string
		parse func(string) (*equipment.RadioReading, equipment.Code)
	}{{"l07_airos_xm_real.txt", ParseRadioAirOS}, {"l07_wom_iw_real.txt", ParseRadioWOM}} {
		raw := read(test.file)
		end := strings.LastIndex(strings.TrimSpace(raw), "\n")
		if end < 0 {
			t.Fatal("golden footer missing")
		}
		if _, c := test.parse(strings.TrimSpace(raw)[:end]); c == equipment.OK {
			t.Fatal("truncated real family accepted", test.file)
		}
	}
}

// Captured again in the installed pilot: RouterOS wraps strength-at-rates
// after HT40-, leaving a deeply indented numeric continuation (2 470ms).
func TestL07APWrappedRateContinuation(t *testing.T) {
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
