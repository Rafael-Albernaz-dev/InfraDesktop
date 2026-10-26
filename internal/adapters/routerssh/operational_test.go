package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
)

func TestPermissionWordsAreData(t *testing.T) {
	for _, name := range []string{"permission denied", "not enough permissions", "failure: not enough permissions", "core permission denied switch"} {
		for _, family := range []string{"v6", "v7"} {
			t.Run(family+"/"+name, func(t *testing.T) {
				peer, err := sshfixture.Start(sshfixture.Config{Family: family, Identity: name, InterfaceName: name})
				if err != nil {
					t.Fatal(err)
				}
				defer peer.Close()
				d := equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
				o, code := (routerssh.Adapter{}).Query(context.Background(), d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
				if code != equipment.OK || o == nil {
					t.Fatalf("valid name rejected: %s", code)
				}
				if o.Identity != name || len(o.Interfaces) != 1 || o.Interfaces[0].Name != name || o.Quality != "complete" {
					t.Fatalf("name not preserved: %#v", o)
				}
			})
		}
	}
}

func TestOperationalVolumeKeepsSummary(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{Mode: "interfaces"})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	o, c := (routerssh.Adapter{}).Query(context.Background(), d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
	if c != equipment.OK || o == nil || o.Identity != "Fixture router" {
		t.Fatalf("interface volume discarded summary: %s %#v", c, o)
	}
}

func TestOperationalInterfacePartsAndDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		mode, status string
		code         equipment.Code
		dynamic      int
	}{
		{"dynamic", "complete", equipment.OK, 10000}, {"interfaces", "limited", equipment.OK, 4096},
		{"interfaces-malformed", "failed", equipment.OK, 0}, {"interfaces-permission", "failed", equipment.OK, 0}, {"interfaces-large", "failed", equipment.OK, 0},
		{"disconnect-auth", "", "SSH_CONNECTION_LOST", 0},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			peer, e := sshfixture.Start(sshfixture.Config{Mode: tc.mode})
			if e != nil {
				t.Fatal(e)
			}
			defer peer.Close()
			d := equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
			o, c := (routerssh.Adapter{}).Query(context.Background(), d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
			if c != tc.code {
				t.Fatalf("got %s want %s", c, tc.code)
			}
			if tc.code == equipment.OK {
				if o == nil || o.CPU != 12 || o.InterfacePart == nil || o.InterfacePart.Status != tc.status || o.InterfacePart.Dynamic != tc.dynamic {
					t.Fatalf("dishonest part %#v", o)
				}
				if tc.status == "failed" && (len(o.Interfaces) != 0 || o.Quality != "partial" || o.InterfacePart.At != "") {
					t.Fatal("failed part pretended valid")
				}
				if tc.status == "limited" && (len(o.Interfaces) != 128 || o.InterfacePart.Eligible != 129 || o.InterfacePart.Total != 4225 || o.Quality != "partial") {
					t.Fatal("limited result not explicit")
				}
			}
		})
	}
}
