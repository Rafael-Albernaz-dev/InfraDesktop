package qualityrules

import (
	"infradesktop/scripts/qualitymetrics"
	"testing"
)

func TestTypeScriptBaselineOnlyBlocksHistoricalSource(t *testing.T) {
	if !ExistingTSWithoutBaseline(true, false) {
		t.Fatal("historical source accepted")
	}
	if ExistingTSWithoutBaseline(false, false) {
		t.Fatal("new source rejected")
	}
}
func TestRatchetCannotAddOrRaiseCeilings(t *testing.T) {
	old := map[string]map[string]int{"a.ts": {"complexity": 18}}
	for _, current := range []map[string]map[string]int{
		{"a.ts": {"complexity": 19}}, {"b.ts": {"complexity": 18}}, {"a.ts": {"max-params": 7}},
	} {
		if len(RatchetGrowth(current, old)) == 0 {
			t.Fatal("growth accepted")
		}
	}
	if len(RatchetGrowth(map[string]map[string]int{"a.ts": {"complexity": 15}}, old)) != 0 {
		t.Fatal("reduction refused")
	}
}
func TestTouchedGoParametersAndDiscardNeedReason(t *testing.T) {
	source := []byte("package fixture\nfunc f(a,b,c,d,e,f,g int) { _ = a }\n")
	if len(GoLimits("fixture.go", source, []Span{{1, 9}})) != 2 {
		t.Fatal("parameters or discard escaped")
	}
	valid := []byte("package fixture\nfunc f(a int) {\n // ignored: fixture only\n _ = a\n}\n")
	if len(GoLimits("fixture.go", valid, []Span{{1, 9}})) != 0 {
		t.Fatal("documented discard refused")
	}
}

func TestLegacyExceptionKeysAreReadFromHeadSource(t *testing.T) {
	source := []byte("package fixture\nvar legacyRatchetOnly = map[string]bool{\"existing\":true}\n")
	keys := legacyKeys(source)
	if !keys["existing"] || keys["new"] {
		t.Fatal("legacy keys not closed")
	}
}

func TestBaselineExistingCeilingsOnlyDecrease(t *testing.T) {
	old := qualitymetrics.Report{"a": {Lines: 10, LongLines: 2, MaxFunction: 6, MaxComplexity: 3}}
	for _, after := range []qualitymetrics.Metric{
		{Lines: 11}, {LongLines: 3}, {MaxFunction: 7}, {MaxComplexity: 4},
	} {
		if len(BaselineGrowth(qualitymetrics.Report{"a": after}, old)) == 0 {
			t.Fatal("increased ceiling accepted", after)
		}
	}
	for _, current := range []qualitymetrics.Report{
		old, {"a": {Lines: 9}, "new": {Lines: 999}}, {},
	} {
		if len(BaselineGrowth(current, old)) != 0 {
			t.Fatal("unchanged, reduced or new key refused")
		}
	}
}
