package routerssh_test

import (
	"encoding/json"
	"fmt"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func radiusWithServices(services string) string {
	return strings.Replace(radiusBody("7.26"), "\tppp\tfalse", "\t"+services+"\ttrue", 1)
}

func TestPPDiagnosticRadiusServiceSets(t *testing.T) {
	for _, services := range []string{"", "ppp;hotspot"} {
		o, c := pppoeparse.ParsePPDiagnostic(equipment.PDRadius, radiusWithServices(services))
		if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != 2 ||
			o.Rows[1].Services != strings.ReplaceAll(services, ";", ",") {
			t.Fatal("valid empty/explicit service set rejected", services, o, c)
		}
	}
}

// Since f390a6a an unknown service keeps the batch and is shown as "desconhecido"
// (see TestPPPoERadiusUnknownRTTAndService); the raw value is never exposed as known.
func TestPPDiagnosticRadiusUnknownServiceKeepsBatch(t *testing.T) {
	for _, services := range []string{"unknown-canary", "ppp,unknown", "ppp,", "+ppp"} {
		o, c := pppoeparse.ParsePPDiagnostic(equipment.PDRadius, radiusWithServices(services))
		if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != 2 {
			t.Fatal("unknown radius service aborted the batch", services, o, c)
		}
		encoded, _ := json.Marshal(o)
		leaked := strings.Contains(string(encoded), "canary") || strings.Contains(string(encoded), "+ppp")
		if o.Rows[1].Services != "desconhecido" || leaked {
			t.Fatal("unknown radius service exposed as known", services, o.Rows[1].Services)
		}
	}
}

func TestPPDiagnosticCatalogFailuresAreNotRows(t *testing.T) {
	for _, tc := range []struct{ body, status string }{
		{strings.Replace(catalogBody("7.26"), "Z\t7\tcomplete", "Z\t7\tchurn", 1), "partial"},
		{strings.Split(catalogBody("7.26"), "Z\t")[0], "partial"},
		{"D\t1\tcatalog\t7.26\t2d1h1s\t2049\nZ\t0\trows\t2d1h1s\n", "limited"},
		{strings.Replace(catalogBody("7.26"), "100-115,120,122", "100-1", 1), "partial"},
	} {
		o, c := pppoeparse.ParsePPDiagnostic(equipment.PDCatalog, tc.body)
		if c != equipment.OK || o.Quality.Status != tc.status || len(o.Rows) != 0 {
			t.Fatal(o, c)
		}
	}
}

func TestPPDiagnosticRanges(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		valid       bool
	}{
		{"120,100-115;114-119", "100-120", true}, {"?", "?", true}, {"", "", true}, {"0", "", false},
		{"4095", "", false}, {"1,+2", "", false}, {"1-2-3", "", false}, {"2-1", "", false}, {"1,", "", false},
	} {
		got, ok := pppoeparse.ParsePDRange(tc.value)
		if ok != tc.valid || ok && got != tc.want {
			t.Fatal(tc, got, ok)
		}
	}
}

func TestPPDiagnosticDynamicCatalogUsesFullRowLimit(t *testing.T) {
	var body strings.Builder
	fmt.Fprintf(&body, "D\t1\tcatalog\t7.26\t2d1h1s\t%d\n", equipment.PPMaxRows)
	for i := 0; i < equipment.PPMaxRows; i++ {
		fmt.Fprintf(&body, "I\t*%x\tdynamic-%d\tpppoe-in\ttrue\tfalse\ttrue\n", i+1, i)
	}
	fmt.Fprintf(&body, "Z\t%d\tcomplete\t2d1h1s\n", equipment.PPMaxRows)
	o, c := pppoeparse.ParsePPDiagnostic(equipment.PDCatalog, body.String())
	if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != equipment.PPMaxRows {
		t.Fatal("dynamic catalog incorrectly limited to128", c, o.Quality)
	}
}
