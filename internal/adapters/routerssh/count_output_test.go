package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

// RouterOS 6.49.10 was observed printing the value of an embedded count-only
// before the explicit H header, even with as-value. No subscriber data is used.
func TestRouterOSCountOutput(t *testing.T) {
	sessions := "H\t1\t0\t6.49.10 (long-term)\t4w5d23h1m51s\r\nZ\t0\tcomplete\t4w5d23h1m51s\r\n"
	t.Run("sessions", func(t *testing.T) {
		for _, prefix := range []string{"", "0\r\n"} {
			o, c := ParsePPPoE(prefix + sessions)
			if c != equipment.OK || o.Quality.Status != "complete" || o.Count != 0 || o.Quality.WireBytes != len(prefix+sessions) {
				t.Fatalf("complete empty reading rejected: code=%s quality=%+v", c, o.Quality)
			}
		}
		for _, prefix := range []string{"1\n", "0\n0\n", "failure: rejected\n", "1000001\n"} {
			o, _ := ParsePPPoE(prefix + sessions)
			if o.Quality.Status == "complete" || len(o.Rows) != 0 {
				t.Fatal("inconsistent/unexpected preamble accepted")
			}
		}
	})
	t.Run("catalog-and-radius", func(t *testing.T) {
		for _, tc := range []struct{ scope, prefix, body string }{
			{equipment.PDCatalog, "0\n0\n0\n", "D\t1\tcatalog\t6.49.10\t2d1h1s\t0\nZ\t0\tcomplete\t2d1h1s\n"},
			{equipment.PDRadius, "0\n", "D\t1\tradius\t6.49.10\t2d1h1s\t1\nA\tfalse\ttrue\nZ\t1\tcomplete\t2d1h1s\n"},
		} {
			for _, prefix := range []string{"", tc.prefix} {
				o, c := ParsePPDiagnostic(tc.scope, prefix+tc.body)
				if c != equipment.OK || o.Quality.Status != "complete" || o.Quality.WireBytes != len(prefix+tc.body) {
					t.Fatalf("%s valid count framing rejected: code=%s quality=%+v", tc.scope, c, o.Quality)
				}
			}
			for _, prefix := range []string{strings.Replace(tc.prefix, "0", "1", 1), "0\n" + tc.prefix} {
				o, _ := ParsePPDiagnostic(tc.scope, prefix+tc.body)
				if o.Quality.Status == "complete" || len(o.Rows) != 0 {
					t.Fatal("inconsistent diagnostic count framing accepted", tc.scope)
				}
			}
		}
	})
	t.Run("ping-context-and-result", func(t *testing.T) {
		context := "C\tv6\t0\t0\t0\n"
		if !pdContextSafe("v6", "0\n0\n0\n"+context) || !pdContextSafe("v6", context) {
			t.Fatal("verified default main context rejected")
		}
		if pdContextSafe("v6", "1\n0\n0\n"+context) || pdContextSafe("v6", "0\n0\n0\n0\n"+context) {
			t.Fatal("unsafe/inconsistent routing context accepted")
		}
		body := "SEQ HOST SIZE TTL TIME STATUS\n0 198.51.100.1 timeout\n1 198.51.100.1 timeout\n2 198.51.100.1 timeout\nsent=3 received=0 packet-loss=100%\n"
		for _, prefix := range []string{"", "0\n0\n0\n1\n"} {
			o, c := ParsePDPing(prefix+body, 3, "198.51.100.1")
			if c != equipment.OK || o.Sent != 3 || o.Received != 0 || o.Min != nil {
				t.Fatal("finite no-reply result rejected or RTT invented", c)
			}
		}
		if _, c := ParsePDPing("0\n0\n0\n2\n"+body, 3, "198.51.100.1"); c == equipment.OK {
			t.Fatal("ambiguous target count accepted")
		}
	})
	t.Run("interface-summary", func(t *testing.T) {
		for _, text := range []string{"eligible=0\ndynamic=1\ndone\n", "eligible=0\n1\ndynamic=1\ndone\n"} {
			o := &equipment.Observation{}
			if c := parseInterfaces(o, text); c != equipment.OK || o.InterfacePart.Dynamic != 1 {
				t.Fatal("known dynamic count rejected", c)
			}
		}
		if c := parseInterfaces(&equipment.Observation{}, "eligible=0\n2\ndynamic=1\ndone\n"); c == equipment.OK {
			t.Fatal("inconsistent dynamic count accepted")
		}
	})
}
