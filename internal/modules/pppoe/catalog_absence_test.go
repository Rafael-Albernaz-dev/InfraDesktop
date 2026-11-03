package pppoe

import (
	"testing"
	"time"
)

func TestPPPoEDynamicAbsentReappearContextAndRetention(t *testing.T) {
	at := time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC)
	dynamic, up := true, true
	old := makeCatalogReading(at, []PDRow{
		{ID: "dynamic", Name: "X", Kind: "interface", Dynamic: &dynamic, Running: &up},
		{ID: "static", Kind: "interface"},
	})
	next := makeCatalogReading(at.Add(time.Minute), nil)
	if c := ReconcileCatalogAbsences(old, next); c != OK ||
		len(next.Rows) != 1 ||
		next.Rows[0].NotSeenSince != old.Provenance.Ended ||
		next.AbsentInterfaces != 1 ||
		next.Rows[0].Running != nil {
		t.Fatal("absent dynamic was lost or remained current", c, next)
	}
	again := makeCatalogReading(at.Add(2*time.Minute), []PDRow{
		{ID: "dynamic", Kind: "interface", Dynamic: &dynamic, Running: &up},
	})
	if c := ReconcileCatalogAbsences(next, again); c != OK ||
		len(again.Rows) != 1 ||
		again.Rows[0].NotSeenSince != "" {
		t.Fatal("reappeared dynamic stayed absent", c, again)
	}
	assertCatalogAbsenceBoundaries(t, at, old, next)
}

func makeCatalogReading(stamp time.Time, rows []PDRow) *PDDiagnostic {
	return &PDDiagnostic{
		DeviceID: "X",
		Scope:    PDCatalog,
		Quality:  PPQuality{Status: "complete", Terminated: true, MembershipChecked: true},
		Provenance: PPProvenance{
			Started: PPTime(stamp), Ended: PPTime(stamp.Add(time.Second)),
			Projection: "catalog", Codec: "1", Capabilities: "1",
			Connection: "endpoint", Version: "6.49.18",
		},
		Rows: rows,
	}
}

func assertCatalogAbsenceBoundaries(t *testing.T, at time.Time, old, next *PDDiagnostic) {
	t.Helper()
	expired := makeCatalogReading(at.Add(25*time.Hour), nil)
	ReconcileCatalogAbsences(next, expired)
	if len(expired.Rows) != 0 {
		t.Fatal("absence exceeded24h")
	}
	other := makeCatalogReading(at.Add(time.Minute), nil)
	other.Provenance.Connection = "different"
	ReconcileCatalogAbsences(old, other)
	if len(other.Rows) != 0 {
		t.Fatal("absence crossed context")
	}
	partial := makeCatalogReading(at.Add(time.Minute), nil)
	partial.Quality.Status = "partial"
	if ReconcileCatalogAbsences(old, partial) != Partial {
		t.Fatal("partial proved absence")
	}
}
