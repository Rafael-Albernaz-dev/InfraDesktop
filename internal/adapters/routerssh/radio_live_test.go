package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"testing"
	"time"
)

// Replaces the old extra-SSH observer coverage test: CPU is now read by the
// tick; the only continuous observer is local ICMP on the existing AP session.
func TestRadioPersistentICMPReportsActualWindow(t *testing.T) {
	raw, e := os.ReadFile("radioparse/testdata/l07_ap_v6_real.txt")
	if e != nil {
		t.Fatal(e)
	}
	peer, e := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) { return string(raw), true }})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := equipment.Device{Role: equipment.RoleAP, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	session, c := (routerssh.Adapter{}).OpenRadioSession(ctx, d, "reader", sshfixture.Password,
		equipment.RadioRouterOSWirelessV6, &equipment.RadioLink{MAC: "AA:BB:CC:DD:EE:FF"},
		func() equipment.Code { return equipment.OK })
	if c != equipment.OK {
		t.Fatal(c)
	}
	defer session.Close()
	started := time.Now()
	load, c := session.Probe()
	assertRadioProbeWindow(t, load, c, started)
	second, c := session.Probe()
	if c != equipment.OK || !second.Started.After(load.Started) || second.PingSent != 1 {
		t.Fatal("stale/empty probe", c, second)
	}
	if peer.Auth.Load() != 1 || peer.Commands.Load() != 0 {
		t.Fatal("ICMP created SSH observer", peer.Auth.Load(), peer.Commands.Load())
	}
	assertRadioProbeCloses(t, session, cancel)
}
