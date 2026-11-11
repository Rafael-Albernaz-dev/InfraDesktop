package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"strings"
	"testing"
)

func TestStandaloneWOMObservedLibraryAndRadioIdentifyFamily(t *testing.T) {
	peer := observedWOMPeer(t)
	defer peer.Close()
	device := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	reading, code := (routerssh.Adapter{}).ReadRadio(context.Background(), device, "reader", sshfixture.Password,
		equipment.RadioStandalone, func() equipment.Code { return equipment.OK })
	if code != equipment.OK || reading == nil {
		t.Fatalf("code=%s reading=%+v", code, reading)
	}
	if reading.Family != equipment.RadioIntelbrasWOM || reading.Manufacturer != "Intelbras" {
		t.Fatalf("identity=%+v", reading)
	}
	if reading.Model != "" || strings.Contains(reading.Version, "8.7c") {
		t.Fatalf("active model/firmware invented: %+v", reading)
	}
	if reading.Mode != "managed" || reading.WirelessMAC != "02:00:00:00:10:21" {
		t.Fatalf("mode/identity=%+v", reading)
	}
	if len(reading.Metrics) == 0 || peer.Commands.Load() != 3 {
		t.Fatalf("metrics=%v commands=%d", reading.Metrics, peer.Commands.Load())
	}
}

func observedWOMPeer(t *testing.T) *sshfixture.Server {
	t.Helper()
	identity, err := os.ReadFile("radioparse/testdata/radio_wom_library_identity.txt")
	if err != nil {
		t.Fatal(err)
	}
	radio, err := os.ReadFile("radioparse/testdata/radio_wom_managed_real.txt")
	if err != nil {
		t.Fatal(err)
	}
	peer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-dropbear", Command: func(command string) (string, bool) {
		if strings.Contains(command, "mca-status") {
			return "L07-DRIVER-ABSENT\n", true
		}
		if strings.Contains(command, "/lib/libfirmware.so") {
			if !strings.Contains(command, "grep -owF") {
				t.Error("identification must preserve whole-word source boundary")
			}
			return string(identity), true
		}
		if command == radioparse.RadioWOMCommand {
			return string(radio), true
		}
		t.Errorf("unexpected command %s", command)
		return "", false
	}})
	if err != nil {
		t.Fatal(err)
	}
	return peer
}
