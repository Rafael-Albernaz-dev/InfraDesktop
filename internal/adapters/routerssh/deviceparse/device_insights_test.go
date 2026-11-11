package deviceparse

import (
	"infradesktop/internal/modules/equipment"
	"os"
	"strings"
	"testing"
)

func TestRealFastTrackAndPorts(t *testing.T) {
	for _, family := range []string{"6", "7"} {
		f, e := os.ReadFile("testdata/t3_fasttrack_v" + family + "_real.txt")
		if e != nil {
			t.Fatal(e)
		}
		n, c := ParseDeviceFastTrack(string(f))
		if c != equipment.OK || n == nil || *n != 1 {
			t.Fatalf("real FastTrack %s count/code %v %s", family, n, c)
		}
		f, e = os.ReadFile("testdata/t3_ports_v" + family + "_real.txt")
		if e != nil {
			t.Fatal(e)
		}
		rows, c := ParseDevicePorts(string(f))
		want := 10
		if family == "7" {
			want = 11
		}
		if c != equipment.OK || len(rows) != want {
			t.Fatalf("real ports %s count/code %d %s", family, len(rows), c)
		}
		assertRealPortNegotiation(t, rows)
	}
}
func TestPortHalfFullAndCollectorFailure(t *testing.T) {
	for _, duplex := range []string{"false", "true"} {
		text := "T3E\t1\t6.49.18\nE\tX\tlink-ok\t100Mbps\t" + duplex + "\t33\nD\t00:00:01\t6\t6\nZ\t1\tcomplete\n"
		rows, c := ParseDevicePorts(text)
		if c != equipment.OK || len(rows) != 1 || rows[0].Port.FullDuplex == nil || *rows[0].Port.FullDuplex != (duplex == "true") {
			t.Fatal("half/full confused", rows, c)
		}
		for _, bad := range []string{strings.Replace(text, "Z\t1", "Z\t2", 1), strings.Replace(text, "100Mbps", "NaN", 1),
			strings.Replace(text, "\t33\n", "\t-1\n", 1)} {
			if _, c := ParseDevicePorts(bad); c != equipment.Partial {
				t.Fatal("bad port accepted", c)
			}
		}
	}
	if _, c := ParseDevicePorts("syntax error (line 1 column 4)\n"); c != equipment.CollectorDefect {
		t.Fatal("syntax masked", c)
	}
	if _, c := ParseDeviceFastTrack("syntax error (line 1 column 4)\n"); c != equipment.CollectorDefect {
		t.Fatal("syntax masked", c)
	}
}

func assertRealPortNegotiation(t *testing.T, rows []equipment.PPReadRow) {
	var down, fast bool
	for _, r := range rows {
		if r.Port == nil || r.Port.Running == nil {
			t.Fatal("presence unknown", r)
		}
		if !*r.Port.Running {
			down = true
			if r.Port.SpeedBPS != nil || r.Port.FullDuplex != nil {
				t.Fatal("DOWN fabricated negotiation", r)
			}
		}
		if r.Port.SpeedBPS != nil && *r.Port.SpeedBPS == 100000000 {
			fast = true
		}
	}
	if !down || !fast {
		t.Fatal("real DOWN and100M cases lost")
	}
}
