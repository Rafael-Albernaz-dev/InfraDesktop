package qualityrules

import "testing"

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
