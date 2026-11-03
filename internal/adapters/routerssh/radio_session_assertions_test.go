package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func assertPersistentRadioFamily(t *testing.T, api equipment.RadioSessionRemote, ok bool, family equipment.RadioFamily) {
	t.Helper()
	peer, role := persistentRadioFamilyPeer(t, family)
	defer peer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var revoked atomic.Bool
	check := func() equipment.Code {
		if revoked.Load() {
			return equipment.Revision
		}
		return equipment.OK
	}
	d := equipment.Device{Role: role, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	requirePersistentRadioAPI(t, ok, ctx, d, family, check, peer)
	link := &equipment.RadioLink{MAC: "AA:BB:CC:DD:EE:FF", Interface: "wlan1", BSSID: "02:00:00:00:10:05"}
	s, c := api.OpenRadioSession(ctx, d, "reader", sshfixture.Password, family, link, check)
	if c != equipment.OK {
		t.Fatal(c)
	}
	defer s.Close()
	if _, c = s.Initial(); c != equipment.OK {
		t.Fatal("initial", c)
	}
	for i := 0; i < 10; i++ {
		if _, c = s.Tick(); c != equipment.OK {
			t.Fatal("tick", i, c)
		}
	}
	if peer.Auth.Load() != 1 || peer.Commands.Load() != 11 {
		t.Fatalf("handshakes=%d exec=%d; want1/11 (rich+10 ticks, A)", peer.Auth.Load(), peer.Commands.Load())
	}
	revoked.Store(true)
	if _, c = s.Tick(); c != equipment.Revision {
		t.Fatal("revocation lost", c)
	}
	if peer.Commands.Load() != 11 {
		t.Fatal("command after revocation")
	}
	assertPersistentRadioClose(t, s, peer)
}
func persistentRadioFamilyPeer(t *testing.T, family equipment.RadioFamily) (*sshfixture.Server, equipment.Role) {
	t.Helper()
	file, role := "l07_ap_v6_real.txt", equipment.RoleAP
	if family == equipment.RadioAirOSV6 {
		file, role = "l07_airos_xm_real.txt", equipment.RoleCPE
	}
	if family == equipment.RadioIntelbrasWOM {
		file, role = "l07_wom_iw_real.txt", equipment.RoleCPE
	}
	rich, e := os.ReadFile("testdata/" + file)
	if e != nil {
		t.Fatal(e)
	}
	peer, e := sshfixture.Start(sshfixture.Config{Command: func(command string) (string, bool) {
		if strings.Contains(command, "L07-TICK") {
			return ("L07-TICK\ncpu=40\nbssid=02:00:00:00:10:05\nL07-CLIENTS\n 0 interface=wlan1 mac" +
					"-address=AA:BB:CC:DD:EE:FF uptime=1m signal-strength=-60 bytes=100,200 packets" +
					"=10,20\nL07-TICK-DONE\n"),
				true
		}
		return string(rich), true
	}})
	if e != nil {
		t.Fatal(e)
	}
	return peer, role
}
func requirePersistentRadioAPI(t *testing.T, ok bool, ctx context.Context, d equipment.Device,
	family equipment.RadioFamily, check func() equipment.Code, peer *sshfixture.Server) {
	t.Helper()
	if !ok {
		for i := 0; i < 10; i++ {
			if _, c := (routerssh.Adapter{}).ReadRadio(ctx, d, "reader", sshfixture.Password, family, check); c != equipment.OK {
				t.Fatal(c)
			}
		}
		t.Fatalf("legacy ten ticks: handshakes=%d exec=%d; want one handshake per destination", peer.Auth.Load(), peer.Commands.Load())
	}
}
func assertPersistentRadioClose(t *testing.T, s equipment.RadioReadSession, peer *sshfixture.Server) {
	t.Helper()
	s.Close()
	s.Close()
	until := time.Now().Add(time.Second)
	for peer.ActiveConnections() != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if peer.ActiveConnections() != 0 {
		t.Fatal("connection leaked after revoke/close")
	}
}
