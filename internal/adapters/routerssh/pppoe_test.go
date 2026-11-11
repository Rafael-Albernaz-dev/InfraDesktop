package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"testing"
	"time"
)

func TestPPPoEAdapterCodecAndFailures(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			login := "ação\\path\tCR\rLF\nfailure: not enough permissions"
			valid := "H\t1\t1\t" + map[string]string{"v6": "6.49.19", "v7": "7.26"}[family] + "\t2d1h1s\nR\t*1\t123\t" +
				pppoeparse.PPPoEEscape(login) + "\t198.51.100.1\t02:00:00:00:00:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
			for _, tc := range []ppCodecCase{
				{"valid", valid, equipment.OK, "complete"},
				{"permission", "failure: not enough permissions\n", "REMOTE_PERMISSION_DENIED", ""},
				{"missing-footer", strings.Split(valid, "Z\t")[0], equipment.OK, "partial"},
				{"churn", strings.Replace(valid, "\tcomplete", "\tchurn", 1), equipment.OK, "partial"},
				{"id-reuse", strings.Replace(strings.Replace(valid, "H\t1\t1", "H\t1\t2", 1), "Z\t1\tcomplete", strings.Split(valid,
					"\n")[1]+"\nZ\t2\tcomplete", 1), equipment.OK, "partial"},
				{"limit", "H\t1\t2049\t7.26\t2d1h1s\nZ\t0\trows\n", equipment.OK, "limited"},
			} {
				t.Run(tc.name, func(t *testing.T) { assertPPCodecCase(t, family, login, tc) })
			}
		})
	}
}

func TestPPPoEAH04ParsedUnknownDurationCannotSupportContinuity(t *testing.T) {
	parse := func(id, rowUptime, routerUptime string, at time.Time, sample int64) *equipment.PPObservation {
		body := "H\t1\t1\t7.26\t" + routerUptime + "\nR\t*1\tsession\treader\t198.51.100.1\tcaller\t" + rowUptime +
			"\nZ\t1\tcomplete\t" + routerUptime + "\n"
		o, c := pppoeparse.ParsePPPoE(body)
		if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != 1 {
			t.Fatal("codec fixture not complete", o, c)
		}
		o.ID, o.DeviceID = id, "same-device"
		p := &o.Provenance
		p.Installation, p.Company, p.Connection, p.Epoch = "installation", "company", "connection", "same-worker"
		p.Started, p.Ended = equipment.PPTime(at), equipment.PPTime(at)
		p.Sample = equipment.PPInterval{Min: sample, Max: sample, Known: true}
		o.Rows[0].ID, o.Rows[0].Sample = id+"-row", p.Sample
		return o
	}
	at := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ uptime, category string }{{"2h", "unchanged"}, {"1h1h", "ambiguous"}} {
		a := parse("base", tc.uptime, "2d1h1s", at, int64(time.Second))
		b := parse("target", "2h1m", "2d1h1m1s", at.Add(time.Minute), int64(61*time.Second))
		deltas, c := equipment.ComparePP(a, b)
		if c != equipment.OK || len(deltas) != 1 || deltas[0].Category != tc.category {
			t.Fatalf("parser -> continuity for %q: %+v (%s), expected %s", tc.uptime, deltas, c, tc.category)
		}
	}
}

func TestPPPoEReviewMissingFinalUptimeThroughSSH(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{PPPoE: func() string { return "H\t1\t0\t7.26\t1s\nZ\t0\tcomplete\n" }})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1",
		Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password,
		func() equipment.Code { return equipment.OK })
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
		o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1",
			Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password,
			func() equipment.Code { return equipment.OK })
		peer.Close()
		if c != equipment.OK || o.Quality.Status != "partial" {
			t.Fatal("reboot cannot be excluded", o, c)
		}
	}
	for _, pair := range [][2]string{{"01:02:03", "1h2m3s"}, {"1w2d01:02:03", "1w2d1h2m3s"}} {
		a, b := pppoeparse.ParsePPDuration(pair[0]), pppoeparse.ParsePPDuration(pair[1])
		if !a.Known || a != b {
			t.Fatal(pair, a, b)
		}
	}
}

type ppCodecCase struct {
	name, body string
	code       equipment.Code
	status     string
}

func assertPPCodecCase(t *testing.T, family, login string, tc ppCodecCase) {

	peer, e := sshfixture.Start(sshfixture.Config{Family: family, PPPoE: func() string { return tc.body }})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1",
		Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password,
		func() equipment.Code { return equipment.OK })
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
	expectedCommands := int32(1)
	if tc.status == "complete" {
		expectedCommands = 2
	}
	if peer.Auth.Load() != 1 || peer.Commands.Load() != expectedCommands {
		t.Fatal("unexpected network effects")
	}

}
