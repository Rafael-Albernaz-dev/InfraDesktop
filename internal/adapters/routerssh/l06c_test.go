package routerssh_test

import (
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func TestL06CCompletedMembershipChange(t *testing.T) {
	body := "H\t1\t2\t7.14.3\t2d\nR\t*1\tsid\treader\t198.51.100.1\tcaller\t1h\nZ\t1\tcomplete\t2d\n"
	o, c := routerssh.ParsePPPoE(body)
	if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != 1 {
		t.Fatalf("disappeared item invalidates completed reading: %+v %s", o, c)
	}
	for _, bad := range []string{strings.Split(body, "Z\t")[0], strings.Replace(body, "complete\t2d", "complete\t1s", 1)} {
		o, c = routerssh.ParsePPPoE(bad)
		if c != equipment.OK || o.Quality.Status == "complete" || len(o.Rows) != 0 {
			t.Fatal("truncated/reboot reading can mark offline", o, c)
		}
	}
}

func TestL06CRadiusUnknownRTTAndService(t *testing.T) {
	for _, service := range []string{"ppp", "user-man", "capsman", "future-service"} {
		body := "D\t1\tradius\t7.14.3\t2d\t2\nA\tyes\tyes\nR\t*1\t198.51.100.2\t" + service + "\tno\t1812\t1813\tudp\t0\t0\t0\t0\t0\t0\t0\t?\nZ\t2\tcomplete\t2d\n"
		o, c := routerssh.ParsePPDiagnostic(equipment.PDRadius, body)
		if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != 2 || o.Rows[1].Counters.LastRTT.Known {
			t.Fatalf("unknown RTT/service aborts batch %s: %+v %s", service, o, c)
		}
		if service == "ppp" && o.Rows[1].Services != "ppp" {
			t.Fatal("documented service lost")
		}
		if service == "future-service" && o.Rows[1].Services != "desconhecido" {
			t.Fatal("unrecognized service exposed as known")
		}
	}
}
