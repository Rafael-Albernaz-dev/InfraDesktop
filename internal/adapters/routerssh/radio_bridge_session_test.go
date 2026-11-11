package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"sync/atomic"
	"testing"
)

func TestStandaloneRealLHGBridgeFallback(t *testing.T) {
	for _, role := range []equipment.Role{equipment.RoleCPE, equipment.RolePTP} {
		t.Run(string(role), func(t *testing.T) { assertRealLHGBridgeFallback(t, role) })
	}
}
func assertRealLHGBridgeFallback(t *testing.T, role equipment.Role) {
	t.Helper()
	station := recoveryGolden(t, "radio_routeros_lhg_identity_bridge.txt")
	rich := recoveryGolden(t, "radio_routeros_lhg_bridge_nv2.txt")
	commands := []string{radioparse.RadioIdentificationCommand, radioparse.RadioStationCommand, radioparse.RadioAPCommand}
	replies := []string{standaloneIdentity(station), station, rich}
	var calls atomic.Int32
	peer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(command string) (string, bool) {
		i := int(calls.Add(1) - 1)
		if i >= len(commands) || command != commands[i] {
			t.Errorf("unexpected command %d: %s", i, command)
			return "", false
		}
		return replies[i], true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	d := equipment.Device{Role: role, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r, code := (routerssh.Adapter{}).ReadRadio(context.Background(), d, "reader", sshfixture.Password,
		equipment.RadioStandalone, func() equipment.Code { return equipment.OK })
	if code != equipment.OK || r == nil {
		t.Fatalf("code=%s reading=%+v", code, r)
	}
	assertLHGBridgeIdentity(t, r)
	if len(r.CPU) != 3 {
		t.Fatal("CPU receipt missing", r.CPU)
	}
	for _, cpu := range r.CPU {
		if cpu != 14 {
			t.Fatal("CPU receipt changed", r.CPU)
		}
	}
	if peer.Auth.Load() != 1 || peer.Commands.Load() != 3 {
		t.Fatal("unexpected retries", peer.Auth.Load(), peer.Commands.Load())
	}
}

func assertLHGBridgeIdentity(t *testing.T, r *equipment.RadioReading) {
	t.Helper()
	if r.Family != equipment.RadioRouterOSWirelessV6 || r.Model != "RouterBOARD LHG 5HPnD-XL" || r.Version != "6.49.13 (long-term)" {
		t.Fatalf("identity=%+v", r)
	}
	if r.Mode != "bridge" || r.WirelessProtocol != "nv2" || len(r.Clients) != 0 {
		t.Fatalf("mode/peers=%+v", r)
	}
}
