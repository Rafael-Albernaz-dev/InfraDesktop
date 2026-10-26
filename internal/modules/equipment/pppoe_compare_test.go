package equipment

import (
	"testing"
	"time"
)

func TestPPPoEContinuityIndependentSequences(t *testing.T) {
	sec := int64(time.Second)
	obs := func(id string, at, router, uptime int64) *PPObservation {
		return &PPObservation{ID: id, DeviceID: "device", Scope: PPScope, Quality: PPQuality{Status: "complete"}, Provenance: PPProvenance{Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1", Installation: "i", Company: "c", Connection: "same", Family: "v7", Epoch: "epoch", Started: PPTime(time.Unix(at, 0)), Ended: PPTime(time.Unix(at, 1)), Sample: PPInterval{Min: at * sec, Max: at*sec + 1, Known: true}, RouterUptime: PPInterval{Min: router * sec, Max: (router+1)*sec - 1, Known: true}}, Rows: []PPRow{{ID: id + "-row", RemoteID: "*1", SessionID: "17", Login: "same-login", Address: "198.51.100.1", Caller: "old", Sample: PPInterval{Min: at * sec, Max: at*sec + 1, Known: true}, Uptime: PPInterval{Min: uptime * sec, Max: (uptime+1)*sec - 1, Known: true}}}}
	}
	for _, tc := range []struct {
		name string
		edit func(*PPObservation)
		want string
	}{
		{"supported-changes", func(b *PPObservation) { b.Rows[0].Address = "198.51.100.2"; b.Rows[0].Caller = "new" }, "changed"},
		{"restart-epoch", func(b *PPObservation) { b.Provenance.Epoch = "restart" }, "ambiguous"},
		{"reused-uptime", func(b *PPObservation) { b.Rows[0].Uptime = PPInterval{Min: sec, Max: 2*sec - 1, Known: true} }, "ambiguous"},
		{"unknown-precision", func(b *PPObservation) { b.Rows[0].Uptime.Known = false }, "ambiguous"},
		{"long-gap-growing-uptime", func(b *PPObservation) {
			b.Provenance.Sample = PPInterval{Min: 100000 * sec, Max: 100000*sec + 1, Known: true}
			b.Rows[0].Sample = b.Provenance.Sample
		}, "ambiguous"},
		{"possible-router-reboot", func(b *PPObservation) { b.Provenance.RouterUptime = PPInterval{Min: sec, Max: 2*sec - 1, Known: true} }, "ambiguous"},
		{"clock-suspend-inconsistent", func(b *PPObservation) {
			b.Provenance.RouterUptime = PPInterval{Min: 200000 * sec, Max: 200001*sec - 1, Known: true}
		}, "ambiguous"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := obs("a", 100, 10000, 1000), obs("b", 110, 10010, 1010)
			tc.edit(b)
			ds, c := ComparePP(a, b)
			if c != OK || len(ds) != 1 || ds[0].Category != tc.want {
				t.Fatalf("got %+v %s", ds, c)
			}
			if tc.want == "changed" && (!ds[0].CallerChanged || !ds[0].AddressChanged) {
				t.Fatal("legitimate field changes lost")
			}
		})
	}
	a, b := obs("a", 100, 10000, 1000), obs("b", 110, 10010, 1010)
	b.Rows = append(b.Rows, b.Rows[0])
	b.Rows[1].ID = "duplicate"
	ds, c := ComparePP(a, b)
	if c != OK || len(ds) != 3 {
		t.Fatal(ds, c)
	}
	for _, d := range ds {
		if d.Category != "ambiguous" {
			t.Fatal("duplicate merged", ds)
		}
	}
	b.Provenance.Connection = "other"
	if _, c := ComparePP(a, b); c != ObservationIncompatible {
		t.Fatal("incompatible accepted")
	}
}

func TestPPPoEC03WallAndMonotonicWindows(t *testing.T) {
	sec := int64(time.Second)
	observation := func(id string, at, router, session int64) *PPObservation {
		sample := PPInterval{Min: at * sec, Max: at*sec + sec, Known: true}
		return &PPObservation{ID: id, DeviceID: "d", Scope: PPScope, Quality: PPQuality{Status: "complete"}, Provenance: PPProvenance{Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1", Epoch: "same-epoch", Started: PPTime(time.Unix(at, 0)), Ended: PPTime(time.Unix(at+1, 0)), Sample: sample, RouterUptime: PPInterval{Min: router * sec, Max: (router+1)*sec - 1, Known: true}}, Rows: []PPRow{{ID: id + "-row", RemoteID: "*1", SessionID: "session", Login: "same", Address: "198.51.100.1", Caller: "old", Sample: sample, Uptime: PPInterval{Min: session * sec, Max: (session+1)*sec - 1, Known: true}}}}
	}
	for _, scenario := range []string{"coherent-positive", "two-day-wall-gap", "missing-ended", "reversed-window"} {
		t.Run(scenario, func(t *testing.T) {
			a, b := observation("a", 100, 10000, 1000), observation("b", 110, 10010, 1010)
			b.Rows[0].Address = "198.51.100.2"
			b.Rows[0].Caller = "new"
			want := "ambiguous"
			switch scenario {
			case "coherent-positive":
				want = "changed"
			case "two-day-wall-gap":
				b.Provenance.Started = PPTime(time.Unix(110+2*86400, 0))
				b.Provenance.Ended = PPTime(time.Unix(111+2*86400, 0))
			case "missing-ended":
				b.Provenance.Ended = ""
			case "reversed-window":
				b.Provenance.Ended = PPTime(time.Unix(109, 0))
			}
			ds, c := ComparePP(a, b)
			if c != OK || len(ds) != 1 || ds[0].Category != want {
				t.Fatalf("%s got %+v code=%s want=%s", scenario, ds, c, want)
			}
			if want == "changed" && (!ds[0].AddressChanged || !ds[0].CallerChanged) {
				t.Fatal("legitimate field changes lost")
			}
			// Local identity within one immutable observation does not require continuity.
			ds, c = ComparePP(b, b)
			if c != OK || len(ds) != 1 || ds[0].Category != "unchanged" || ds[0].BaseRow != ds[0].TargetRow {
				t.Fatal("same observation identity lost", ds, c)
			}
		})
	}
}
