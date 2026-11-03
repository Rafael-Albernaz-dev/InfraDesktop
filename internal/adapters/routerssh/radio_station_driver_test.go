package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"testing"
)

func TestKnownRouterOSStationUsesStationParser(t *testing.T) {
	raw, err := os.ReadFile("testdata/l07_avulso_station_v6.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []equipment.Role{equipment.RoleCPE, equipment.RolePTP} {
		t.Run(string(role), func(t *testing.T) {
			peer, err := sshfixture.Start(sshfixture.Config{Command: func(command string) (string, bool) {
				if command != routerssh.RadioStationCommand {
					t.Errorf("unexpected command %s", command)
					return "", false
				}
				return string(raw), true
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			device := equipment.Device{Role: role, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
			reading, code := (routerssh.Adapter{}).ReadRadio(context.Background(), device, "reader", sshfixture.Password,
				equipment.RadioRouterOSWirelessV6, func() equipment.Code { return equipment.OK })
			if code != equipment.OK || reading == nil {
				t.Fatalf("code=%s reading=%+v", code, reading)
			}
			if reading.Family != equipment.RadioRouterOSWirelessV6 || reading.Model != "RBLHG-5HPnD" || reading.Mode != "station-bridge" {
				t.Fatalf("identity=%+v", reading)
			}
			if reading.WirelessMAC != "02:00:00:00:20:01" || reading.APBSSID != "02:00:00:00:20:02" {
				t.Fatalf("link=%+v", reading)
			}
			if peer.Auth.Load() != 1 || peer.Commands.Load() != 1 {
				t.Fatalf("auth=%d commands=%d", peer.Auth.Load(), peer.Commands.Load())
			}
		})
	}
}
