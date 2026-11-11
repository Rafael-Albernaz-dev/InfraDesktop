package pppoeparse_test

import (
	"fmt"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func TestPPPoEAH04DurationGrammar(t *testing.T) {
	for _, bad := range []string{"1h1h", "1s1h", "1d1w", "1m2m", "+1w01:02:03", "1w+2d01:02:03", "+01:02:03", "01:+02:03",
		"01:02:+03", "01:02:03.+1", "01:02:03.-0", "1w1w01:02:03", "1d1w01:02:03", "01:02:03.1234567890", "9223372036854775807w"} {
		t.Run(bad, func(t *testing.T) {
			if got := pppoeparse.ParsePPDuration(bad); got.Known {
				t.Fatalf("malformed uptime accepted: %q => %+v", bad, got)
			}
		})
	}
	for _, good := range []string{"0s", "1w2d3h4m5s", "2h", "01:02:03", "1w2d01:02:03.1", "00:00:00.000000001", "00:00:00.123456789"} {
		t.Run(good, func(t *testing.T) {
			if got := pppoeparse.ParsePPDuration(good); !got.Known || got.Min < 0 || got.Max < got.Min {
				t.Fatalf("valid uptime rejected: %q => %+v", good, got)
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
		o, c := pppoeparse.ParsePPPoE(b.String())
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
		if pppoeparse.ParsePPDuration(s).Known {
			t.Fatal("unsafe duration", s)
		}
	}
}

func TestPPPoERejectsUnframedControlsPreservesEscapedData(t *testing.T) {
	prefix := "H\t1\t1\t7.26\t2d1h1s\nR\t*1\t1\t"
	suffix := "\t198.51.100.1\tcaller\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
	for _, control := range []byte{0, 1, 8, 11, 12, 27, 127} {
		o, c := pppoeparse.ParsePPPoE(prefix + "login" + string(control) + suffix)
		if c != equipment.OK || o.Quality.Status != "partial" || o.Quality.Reason != "codec" || len(o.Rows) != 0 {
			t.Fatalf("raw control %d was published: %s %#v", control, c, o.Quality)
		}
	}
	o, c := pppoeparse.ParsePPPoE(prefix + `login\tline\nreturn\rslash\\ação` + suffix)
	if c != equipment.OK || o.Quality.Status != "complete" || o.Rows[0].Login != "login\tline\nreturn\rslash\\ação" {
		t.Fatal("legitimate escaped data rejected", c, o.Quality)
	}
}
