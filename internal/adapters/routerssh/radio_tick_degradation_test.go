package routerssh_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"infradesktop/internal/adapters/routerssh"
	e "infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
)

func tickIdentity() (*e.RadioReading, *e.RadioLink) {
	return &e.RadioReading{Family: e.RadioRouterOSWirelessV6},
		&e.RadioLink{MAC: "AA:BB:CC:DD:EE:FF", Interface: `wl"an\1`, BSSID: "02:00:00:00:10:05"}
}

func TestRadioTickDegradesOnlyInvalidMetric(t *testing.T) {
	for _, tc := range []struct{ name, field, reason string }{
		{"empty rate", `tx-rate=""`, "tick-metric:tx-rate"},
		{"malformed ccq", `tx-ccq="broken"`, "tick-metric:tx-ccq"},
		{"absent ccq", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial, link := tickIdentity()
			wire := strings.Replace(projectedRadioTickWire, "L07-TICK-DONE", "  "+tc.field+"\nL07-TICK-DONE", 1)
			r, code := routerssh.ParseRadioAPTick(wire, initial, link)
			if code != e.OK || r == nil || len(r.CPU) != 1 || r.CPU[0] != 40 || len(r.Clients) != 1 {
				t.Fatalf("valid CPU/peer lost: %s %+v", code, r)
			}
			assertTickMetrics(t, r, link, tc.reason)
		})
	}
}

func TestRadioTickRejectsMissingCPUAndOpenFrame(t *testing.T) {
	for _, tc := range []struct{ wire, reason string }{
		{strings.Replace(projectedRadioTickWire, "cpu=40", "cpu=", 1), "tick-cpu"},
		{strings.Replace(projectedRadioTickWire, "L07-TICK-DONE", "", 1), "tick-frame"},
	} {
		initial, link := tickIdentity()
		r, c := routerssh.ParseRadioAPTick(tc.wire, initial, link)
		if c != e.Partial || r == nil || len(r.CPU) != 0 || r.Failure == nil || r.Failure.Reason != tc.reason {
			t.Fatalf("%s: %s %+v", tc.reason, c, r)
		}
	}
	initial, link := tickIdentity()
	badPeer := strings.Replace(projectedRadioTickWire, link.MAC, "02:00:00:00:00:99", 1)
	if _, c := routerssh.ParseRadioAPTick(badPeer, initial, link); c != e.RadioCodeLinkUnconfirmed {
		t.Fatal(c)
	}
}

func TestRadioTickDataErrorKeepsConnectionTransportCloses(t *testing.T) {
	var calls atomic.Int32
	peer, err := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) {
		if calls.Add(1) == 1 {
			return "L07-TICK\ncpu=40\n", true
		}
		return strings.ReplaceAll(projectedRadioTickWire, `wl\"an\\1`, "wlan1"), true
	}, DisconnectCommand: func(string) bool { return calls.Load() >= 2 }})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	_, link := tickIdentity()
	link.Interface = "wlan1"
	s, c := (routerssh.Adapter{}).OpenRadioSession(context.Background(), e.Device{Role: e.RoleAP, Address: "127.0.0.1",
		Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password,
		e.RadioRouterOSWirelessV6, link, func() e.Code { return e.OK })
	if c != e.OK {
		t.Fatal(c)
	}
	defer s.Close()
	if _, c = s.Tick(); c != e.Partial {
		t.Fatal("expected frame error", c)
	}
	if _, c = s.Tick(); c != e.OK {
		t.Fatal("data error closed usable connection", c)
	}
	if peer.Handshakes.Load() != 1 {
		t.Fatal("unexpected handshake")
	}
	if _, c = s.Tick(); c == e.OK {
		t.Fatal("transport failure accepted")
	}
	assertTransportTickClosed(t, peer)
}

func assertTickMetrics(t *testing.T, r *e.RadioReading, link *e.RadioLink, reason string) {
	t.Helper()

	if r.Clients[0].MAC != link.MAC || r.Clients[0].BSSID != link.BSSID {
		t.Fatal("identity lost")
	}
	for _, m := range r.Clients[0].Metrics {
		if m.Name == "rate_tx" || m.Name == "ccq_tx" {
			t.Fatal("invalid metric became a value", m)
		}
	}
	if reason != "" && (r.Failure == nil || r.Failure.Reason != reason) {
		t.Fatalf("missing diagnostic: %+v", r.Failure)
	}
	if reason == "" && r.Failure != nil {
		t.Fatal("absent optional metric is not malformed")
	}
}

func assertTransportTickClosed(t *testing.T, peer *sshfixture.Server) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for peer.ActiveConnections() != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if peer.ActiveConnections() != 0 {
		t.Fatal("transport failure retained SSH connection before explicit cleanup")
	}
}
