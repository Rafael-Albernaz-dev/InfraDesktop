package routerssh_test

import (
	"context"
	"fmt"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"testing"
)

func TestPPPoEAdapterCodecAndFailures(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			login := "ação\\path\tCR\rLF\nfailure: not enough permissions"
			valid := "H\t1\t1\t" + map[string]string{"v6": "6.49.19", "v7": "7.26"}[family] + "\t2d1h1s\nR\t*1\t123\t" + routerssh.PPPoEEscape(login) + "\t198.51.100.1\t02:00:00:00:00:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
			for _, tc := range []struct {
				name, body string
				code       equipment.Code
				status     string
			}{
				{"valid", valid, equipment.OK, "complete"},
				{"permission", "failure: not enough permissions\n", "REMOTE_PERMISSION_DENIED", ""},
				{"missing-footer", strings.Split(valid, "Z\t")[0], equipment.OK, "partial"},
				{"churn", strings.Replace(valid, "\tcomplete", "\tchurn", 1), equipment.OK, "partial"},
				{"id-reuse", strings.Replace(strings.Replace(valid, "H\t1\t1", "H\t1\t2", 1), "Z\t1\tcomplete", strings.Split(valid, "\n")[1]+"\nZ\t2\tcomplete", 1), equipment.OK, "partial"},
				{"limit", "H\t1\t2049\t7.26\t2d1h1s\nZ\t0\trows\n", equipment.OK, "limited"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					peer, e := sshfixture.Start(sshfixture.Config{Family: family, PPPoE: func() string { return tc.body }})
					if e != nil {
						t.Fatal(e)
					}
					defer peer.Close()
					o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
					if c != tc.code {
						t.Fatalf("code %s want %s", c, tc.code)
					}
					if o != nil && o.Quality.Status != tc.status {
						t.Fatalf("quality %+v", o.Quality)
					}
					if tc.name == "valid" && (len(o.Rows) != 1 || o.Rows[0].Login != login) {
						t.Fatal("non-injective codec", o)
					}
					if o != nil && tc.status != "complete" && len(o.Rows) != 0 {
						t.Fatal("incomplete rows leaked")
					}
					if peer.Auth.Load() != 1 || peer.Commands.Load() != 1 {
						t.Fatal("unexpected network effects")
					}
				})
			}
		})
	}
}
func TestPPPoEParserBoundaries(t *testing.T) {
	for _, n := range []int{0, 2047, 2048, 2049} {
		var b strings.Builder
		fmt.Fprintf(&b, "H\t1\t%d\t7.26\t2d1h1s\n", n)
		if n <= 2048 {
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, "R\t*%d\t%d\tu\t198.51.100.1\t\t1h1s\n", i, i)
			}
			fmt.Fprintf(&b, "Z\t%d\tcomplete\t2d1h1s\n", n)
		} else {
			b.WriteString("Z\t0\trows\n")
		}
		o, c := routerssh.ParsePPPoE(b.String())
		if c != equipment.OK {
			t.Fatal(c)
		}
		if n <= 2048 && o.Quality.Status != "complete" {
			t.Fatal(n, o.Quality)
		}
		if n > 2048 && (o.Quality.Status != "limited" || o.Quality.Returned != 0) {
			t.Fatal(o.Quality)
		}
	}
	for _, s := range []string{"99999999999999999999999w", "bad", "-1s", "9223372036854775807s"} {
		if routerssh.ParsePPDuration(s).Known {
			t.Fatal("unsafe duration", s)
		}
	}
}

func TestPPPoEReviewMissingFinalUptimeThroughSSH(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{PPPoE: func() string { return "H\t1\t0\t7.26\t1s\nZ\t0\tcomplete\n" }})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
	if c != equipment.OK || o.Quality.Status != "partial" {
		t.Fatal("missing final reboot evidence accepted", c, o.Quality)
	}
}

func TestPPPoERebootGrowingFinalUptimeAndClockFormats(t *testing.T) {
	for _, body := range []string{"H\t1\t0\t7.26\t1s\nZ\t0\tcomplete\t10s\n", "H\t1\t0\t7.26\t0s\nZ\t0\tcomplete\t0s\n"} {
		peer, e := sshfixture.Start(sshfixture.Config{PPPoE: func() string { return body }})
		if e != nil {
			t.Fatal(e)
		}
		o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
		peer.Close()
		if c != equipment.OK || o.Quality.Status != "partial" {
			t.Fatal("reboot cannot be excluded", o, c)
		}
	}
	for _, pair := range [][2]string{{"01:02:03", "1h2m3s"}, {"1w2d01:02:03", "1w2d1h2m3s"}} {
		a, b := routerssh.ParsePPDuration(pair[0]), routerssh.ParsePPDuration(pair[1])
		if !a.Known || a != b {
			t.Fatal(pair, a, b)
		}
	}
}

func TestPPPoERejectsUnframedControlsPreservesEscapedData(t *testing.T) {
	prefix := "H\t1\t1\t7.26\t2d1h1s\nR\t*1\t1\t"
	suffix := "\t198.51.100.1\tcaller\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
	for _, control := range []byte{0, 1, 8, 11, 12, 27, 127} {
		o, c := routerssh.ParsePPPoE(prefix + "login" + string(control) + suffix)
		if c != equipment.OK || o.Quality.Status != "partial" || o.Quality.Reason != "codec" || len(o.Rows) != 0 {
			t.Fatalf("raw control %d was published: %s %#v", control, c, o.Quality)
		}
	}
	o, c := routerssh.ParsePPPoE(prefix + `login\tline\nreturn\rslash\\ação` + suffix)
	if c != equipment.OK || o.Quality.Status != "complete" || o.Rows[0].Login != "login\tline\nreturn\rslash\\ação" {
		t.Fatal("legitimate escaped data rejected", c, o.Quality)
	}
}
