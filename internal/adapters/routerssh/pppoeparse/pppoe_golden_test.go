package pppoeparse

import (
	"bytes"
	"infradesktop/internal/modules/equipment"
	"os"
	"strings"
	"testing"
)

func TestRealSanitizedGoldens(t *testing.T) {
	sessions, e := os.ReadFile("testdata/device_sessions_v7_real_sanitized.txt")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Count(sessions, []byte("\r\n")) != 55 {
		t.Fatal("real CRLF shape changed")
	}
	o, c := ParsePPPoE(string(sessions))
	if c != equipment.OK || o.Quality.Status != "complete" || o.Quality.Eligible != 52 || len(o.Rows) != 52 {
		t.Fatal("real count prelude/sessions rejected", c, o.Quality)
	}
	if o.Rows[0].Login != "lab-session-001" || o.Rows[51].Address != "198.51.100.52" {
		t.Fatal("synthetic identities changed")
	}
	assertSanitizedPoolGolden(t)
}

func assertSanitizedPoolGolden(t *testing.T) {
	pools, e := os.ReadFile("testdata/device_pools_v7_real_sanitized.txt")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Count(pools, []byte("\r\n")) != 58 {
		t.Fatal("real pool CRLF shape changed")
	}
	h, c := ParsePPHealth(string(pools))
	if c != equipment.OK || len(h.Interfaces) != 48 || len(h.Pools) != 4 || !h.PoolsAvailable || !h.UplinksAvailable {
		t.Fatal("real pools/interfaces rejected", c)
	}
	for i, line := range strings.Split(string(pools), "\r\n") {
		if strings.HasPrefix(line, "P\t") {
			lines := strings.Split(string(pools), "\r\n")
			lines[i-1] = "999999"
			if _, c := ParsePPHealth(strings.Join(lines, "\r\n")); c != equipment.Partial {
				t.Fatal("unbound real count echo accepted")
			}
			break
		}
	}
}
