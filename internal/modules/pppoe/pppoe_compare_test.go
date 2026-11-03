package pppoe

import (
	"testing"
	"time"
)

func makeCompareObs(id string, at, router, uptime int64) *PPObservation {
	sec := int64(time.Second)
	sample := PPInterval{Min: at * sec, Max: at*sec + 1, Known: true}
	return &PPObservation{
		ID:       id,
		DeviceID: "device",
		Scope:    PPScope,
		Quality:  PPQuality{Status: "complete"},
		Provenance: PPProvenance{
			Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1",
			Installation: "i", Company: "c", Connection: "same", Family: "v7", Epoch: "epoch",
			Started:      PPTime(time.Unix(at, 0)),
			Ended:        PPTime(time.Unix(at, 1)),
			Sample:       sample,
			RouterUptime: PPInterval{Min: router * sec, Max: (router+1)*sec - 1, Known: true},
		},
		Rows: []PPRow{{
			ID: id + "-row", RemoteID: "*1", SessionID: "17", Login: "same-login",
			Address: "198.51.100.1", Caller: "old", Sample: sample,
			Uptime: PPInterval{Min: uptime * sec, Max: (uptime+1)*sec - 1, Known: true},
		}},
	}
}

func TestPPPoEContinuityIndependentSequences(t *testing.T) {
	sec := int64(time.Second)
	for _, tc := range []struct {
		name string
		edit func(*PPObservation)
		want string
	}{
		{"supported-changes", func(b *PPObservation) {
			b.Rows[0].Address = "198.51.100.2"
			b.Rows[0].Caller = "new"
		}, "changed"},
		{"restart-epoch", func(b *PPObservation) { b.Provenance.Epoch = "restart" }, "ambiguous"},
		{"reused-uptime", func(b *PPObservation) {
			b.Rows[0].Uptime = PPInterval{Min: sec, Max: 2*sec - 1, Known: true}
		}, "ambiguous"},
		{"unknown-precision", func(b *PPObservation) { b.Rows[0].Uptime.Known = false }, "ambiguous"},
		{"long-gap-growing-uptime", func(b *PPObservation) {
			b.Provenance.Sample = PPInterval{Min: 100000 * sec, Max: 100000*sec + 1, Known: true}
			b.Rows[0].Sample = b.Provenance.Sample
		}, "ambiguous"},
		{"possible-router-reboot", func(b *PPObservation) {
			b.Provenance.RouterUptime = PPInterval{Min: sec, Max: 2*sec - 1, Known: true}
		}, "ambiguous"},
		{"clock-suspend-inconsistent", func(b *PPObservation) {
			b.Provenance.RouterUptime = PPInterval{Min: 200000 * sec, Max: 200001*sec - 1, Known: true}
		}, "ambiguous"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertCompareContinuityCase(t, tc.edit, tc.want)
		})
	}
	assertCompareDuplicateAndIncompatible(t)
}

func assertCompareContinuityCase(t *testing.T, edit func(*PPObservation), want string) {
	t.Helper()
	a, b := makeCompareObs("a", 100, 10000, 1000), makeCompareObs("b", 110, 10010, 1010)
	edit(b)
	ds, c := ComparePP(a, b)
	if c != OK || len(ds) != 1 || ds[0].Category != want {
		t.Fatalf("got %+v %s", ds, c)
	}
	if want == "changed" && (!ds[0].CallerChanged || !ds[0].AddressChanged) {
		t.Fatal("legitimate field changes lost")
	}
}

func assertCompareDuplicateAndIncompatible(t *testing.T) {
	t.Helper()
	a, b := makeCompareObs("a", 100, 10000, 1000), makeCompareObs("b", 110, 10010, 1010)
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
	scenarios := []string{"coherent-positive", "two-day-wall-gap", "missing-ended", "reversed-window"}
	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			assertWallAndMonotonicWindowScenario(t, scenario)
		})
	}
}

func assertWallAndMonotonicWindowScenario(t *testing.T, scenario string) {
	t.Helper()
	sec := int64(time.Second)
	obs := func(id string, at, router, session int64) *PPObservation {
		o := makeCompareObs(id, at, router, session)
		o.DeviceID = "d"
		o.Provenance.Epoch = "same-epoch"
		o.Provenance.Ended = PPTime(time.Unix(at+1, 0))
		o.Provenance.Sample = PPInterval{Min: at * sec, Max: at*sec + sec, Known: true}
		o.Rows[0].SessionID = "session"
		o.Rows[0].Login = "same"
		o.Rows[0].Sample = o.Provenance.Sample
		return o
	}
	a, b := obs("a", 100, 10000, 1000), obs("b", 110, 10010, 1010)
	b.Rows[0].Address = "198.51.100.2"
	b.Rows[0].Caller = "new"
	want := applyWallScenario(b, scenario)
	ds, c := ComparePP(a, b)
	if c != OK || len(ds) != 1 || ds[0].Category != want {
		t.Fatalf("%s got %+v code=%s want=%s", scenario, ds, c, want)
	}
	if want == "changed" && (!ds[0].AddressChanged || !ds[0].CallerChanged) {
		t.Fatal("legitimate field changes lost")
	}
	ds, c = ComparePP(b, b)
	if c != OK || len(ds) != 1 || ds[0].Category != "unchanged" || ds[0].BaseRow != ds[0].TargetRow {
		t.Fatal("same observation identity lost", ds, c)
	}
}

func applyWallScenario(b *PPObservation, scenario string) string {
	switch scenario {
	case "coherent-positive":
		return "changed"
	case "two-day-wall-gap":
		b.Provenance.Started = PPTime(time.Unix(110+2*86400, 0))
		b.Provenance.Ended = PPTime(time.Unix(111+2*86400, 0))
	case "missing-ended":
		b.Provenance.Ended = ""
	case "reversed-window":
		b.Provenance.Ended = PPTime(time.Unix(109, 0))
	}
	return "ambiguous"
}
