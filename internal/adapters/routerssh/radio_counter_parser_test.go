package routerssh_test

import (
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func TestRadioTickCannotReuseCachedBSSID(t *testing.T) {
	initial := &equipment.RadioReading{Family: equipment.RadioRouterOSWirelessV6, Perspective: "AP"}
	link := &equipment.RadioLink{MAC: "AA:BB:CC:DD:EE:FF", Interface: "wlan1", BSSID: "02:00:00:00:10:05"}
	old := "L07-TICK\ncpu=40\nL07-CLIENTS\n 0 interface=wlan1 mac-address=AA:BB:CC:DD:EE:FF uptime=1m signal-strength=-60\nL07-TICK-DONE\n"
	if _, c := routerssh.ParseRadioAPTick(old, initial, link); c == equipment.OK {
		t.Fatal("cached BSSID accepted without current interface identity")
	}
	current := strings.Replace(old, "cpu=40\n", "cpu=40\nbssid="+link.BSSID+"\n", 1)
	if r, c := routerssh.ParseRadioAPTick(current, initial, link); c != equipment.OK || r.Clients[0].BSSID != link.BSSID {
		t.Fatal("current identity rejected", c)
	}
	changed := strings.Replace(current, link.BSSID, "64:D1:54:1D:88:30", 1)
	if _, c := routerssh.ParseRadioAPTick(changed, initial, link); c == equipment.OK {
		t.Fatal("changed BSSID accepted")
	}
}

func TestRadioProjectedTickQuotedFieldsAndArrayCounters(t *testing.T) {
	// Synthetic projected wire grammar, not a claimed RouterOS capture.
	text := projectedRadioTickWire
	link := &equipment.RadioLink{MAC: "AA:BB:CC:DD:EE:FF", Interface: `wl"an\1`, BSSID: "02:00:00:00:10:05"}
	r, c := routerssh.ParseRadioAPTick(text, &equipment.RadioReading{Family: equipment.RadioRouterOSWirelessV6, Perspective: "AP"}, link)
	if c != equipment.OK {
		t.Fatal(c)
	}
	client := r.Clients[0]
	if client.Interface != link.Interface ||
		client.Counters["tx_bytes"] != "18446744073709551614" ||
		client.Counters["rx_bytes"] != "200" ||
		client.Counters["uptime_seconds"] != "83" {
		t.Fatal("projected values changed", client)
	}
	for _, m := range client.Metrics {
		if strings.HasPrefix(m.Name, "ccq") {
			t.Fatal("absent CCQ fabricated", m)
		}
	}
	if _, c := routerssh.ParseRadioAPTick(strings.TrimSuffix(text, "L07-TICK-DONE\n"), &equipment.RadioReading{}, link); c == equipment.OK {
		t.Fatal("open frame accepted")
	}
}
