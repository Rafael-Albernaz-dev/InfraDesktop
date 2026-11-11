package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUnknownCPEAutoDoesNotInferWOMFromGenericIW(t *testing.T) {
	payload, err := os.ReadFile("radioparse/testdata/l07_wom_iw_real.txt")
	if err != nil {
		t.Fatal(err)
	}
	var iwCalls atomic.Int64
	peer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-generic-linux", Command: func(command string) (string, bool) {
		// The replay has generic iw syntax only; it is not vendor identity evidence.
		if strings.Contains(command, "else iw dev") {
			iwCalls.Add(1)
			return string(payload), true
		}
		if strings.Contains(command, "mca-status") {
			return "L07-DRIVER-ABSENT\n", true
		}
		if strings.Contains(command, "/lib/libfirmware.so") {
			return "L07-LIBFIRMWARE-EXISTS\nNAO\n", true
		}
		if strings.Contains(command, "iw dev") {
			iwCalls.Add(1)
			return string(payload), true
		}
		t.Errorf("unexpected command %s", command)
		return "", false
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	device := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	reading, code := (routerssh.Adapter{}).ReadRadio(context.Background(), device, "reader", sshfixture.Password,
		equipment.RadioAuto, func() equipment.Code { return equipment.OK })
	if code != equipment.RadioCodeUnsupported || reading == nil || reading.Family != equipment.RadioUnsupported {
		t.Fatalf("generic Linux inferred as vendor: code=%s reading=%+v", code, reading)
	}
	if iwCalls.Load() != 0 || peer.Auth.Load() != 1 {
		t.Fatalf("iw=%d authentication=%d", iwCalls.Load(), peer.Auth.Load())
	}
}

func autoIdentityReplay(t *testing.T, family equipment.RadioFamily, identity, radio string) func(string) (string, bool) {
	t.Helper()
	return func(command string) (string, bool) {
		if strings.Contains(command, "mca-status") {
			if family == equipment.RadioAirOSV6 {
				return radio, true
			}
			return "L07-DRIVER-ABSENT\n", true
		}
		if command == radioparse.RadioWOMIdentificationCommand {
			return identity, true
		}
		if command == radioparse.RadioWOMCommand && family == equipment.RadioIntelbrasWOM {
			return radio, true
		}
		t.Errorf("unexpected command %s", command)
		return "", false
	}
}

func TestRadioAutoUsesPositiveFamilyIdentity(t *testing.T) {
	for _, test := range []struct {
		name, radio, manufacturer string
		family                    equipment.RadioFamily
		commands                  int32
	}{
		{"airOS XM", "l07_airos_xm_real.txt", "Ubiquiti", equipment.RadioAirOSV6, 1},
		{"airOS XW", "l07_airos_xw_6_3_6.txt", "Ubiquiti", equipment.RadioAirOSV6, 1},
		{"WOM", "radio_wom_managed_real.txt", "Intelbras", equipment.RadioIntelbrasWOM, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity, err := os.ReadFile("radioparse/testdata/radio_wom_library_identity.txt")
			if err != nil {
				t.Fatal(err)
			}
			radio, err := os.ReadFile("radioparse/testdata/" + test.radio)
			if err != nil {
				t.Fatal(err)
			}
			peer, err := sshfixture.Start(sshfixture.Config{Command: autoIdentityReplay(t, test.family, string(identity), string(radio))})
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			device := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
			reading, code := (routerssh.Adapter{}).ReadRadio(context.Background(), device, "reader", sshfixture.Password,
				equipment.RadioAuto, func() equipment.Code { return equipment.OK })
			if code != equipment.OK || reading == nil {
				t.Fatalf("code=%s reading=%+v", code, reading)
			}
			if reading.Family != test.family || reading.Manufacturer != test.manufacturer {
				t.Fatalf("identity=%+v", reading)
			}
			if peer.Auth.Load() != 1 || peer.Commands.Load() != test.commands {
				t.Fatalf("authentication=%d commands=%d", peer.Auth.Load(), peer.Commands.Load())
			}
		})
	}
}
