package qualityrules

import (
	"infradesktop/quality/go/qualitymetrics"
	"os"
	"os/exec"
	"path/filepath"
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
		{Lines: 501}, {LongLines: 3}, {MaxFunction: 7}, {MaxComplexity: 4},
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

func TestBaselineMissingKeyRequiresDeletedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists.go")
	if err := os.WriteFile(path, []byte("package fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	old := qualitymetrics.Report{path: {Lines: 10}}
	if len(BaselineGrowth(qualitymetrics.Report{}, old)) == 0 {
		t.Fatal("existing file key removed")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if len(BaselineGrowth(qualitymetrics.Report{}, old)) != 0 {
		t.Fatal("deleted file blocked")
	}
}
func TestBaselineLinesMayGrowBelowFileLimit(t *testing.T) {
	old := qualitymetrics.Report{"a.go": {Lines: 10}}
	if len(BaselineGrowth(qualitymetrics.Report{"a.go": {Lines: 20}}, old)) != 0 {
		t.Fatal("within-limit growth blocked")
	}
}

func stagedRenameFixture(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, dir := range []string{"frontend/src/app", "frontend/quality", "quality/baselines", "quality/go"} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"frontend/src/old file.ts":                "export const label = 'synthetic';\n",
		"frontend/quality/quality-ratchet.json":   `{"src/old file.ts":{"complexity":18}}`,
		"quality/baselines/quality-baseline.json": `{"frontend/src/old file.ts":{"lines":10,"LongLines":2}}`,
		"quality/go/quality.go":                   `package fixture; var legacyRatchetOnly = map[string]bool{"frontend/src/old file.ts":true}`,
	}
	for name, content := range files {
		writeGateFixture(t, name, content)
	}
	runGateGit(t, "init", "-q")
	runGateGit(t, "add", ".")
	runGateGit(t, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-qm", "baseline")
	runGateGit(t, "mv", "frontend/src/old file.ts", "frontend/src/app/new file.ts")
}
func runGateGit(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func writeGateFixture(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestRenamedRatchetKeyKeepsHeadCeiling(t *testing.T) {
	stagedRenameFixture(t)
	writeGateFixture(t, "frontend/quality/quality-ratchet.json", `{"src/app/new file.ts":{"complexity":18}}`)
	if got := FrontendRatchet(); len(got) != 0 {
		t.Fatal(got)
	}
	writeGateFixture(t, "quality/baselines/quality-baseline.json", `{"frontend/src/app/new file.ts":{"lines":10,"LongLines":2}}`)
	if got := BaselineRatchet(); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestRenamedRatchetKeyStillRejectsGrowth(t *testing.T) {
	stagedRenameFixture(t)
	writeGateFixture(t, "frontend/quality/quality-ratchet.json", `{"src/app/new file.ts":{"complexity":19}}`)
	if got := FrontendRatchet(); len(got) == 0 {
		t.Fatal("renamed ceiling grew")
	}
	writeGateFixture(t, "quality/baselines/quality-baseline.json", `{"frontend/src/app/new file.ts":{"lines":10,"LongLines":3}}`)
	if got := BaselineRatchet(); len(got) == 0 {
		t.Fatal("renamed baseline ceiling grew")
	}
}
func TestNewRatchetKeyWithoutStagedRenameIsRejected(t *testing.T) {
	stagedRenameFixture(t)
	runGateGit(t, "restore", "--staged", ".")
	writeGateFixture(t, "frontend/quality/quality-ratchet.json", `{"src/app/new file.ts":{"complexity":18}}`)
	if got := FrontendRatchet(); len(got) == 0 {
		t.Fatal("unstaged rename accepted as approved ceiling")
	}
}
func TestRenamedLegacyExceptionKeepsHeadAuthority(t *testing.T) {
	stagedRenameFixture(t)
	if got := LegacyGate(map[string]bool{"frontend/src/app/new file.ts": true}); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestNewLegacyExceptionWithoutRenameIsRejected(t *testing.T) {
	stagedRenameFixture(t)
	if got := LegacyGate(map[string]bool{"frontend/src/app/unrelated.ts": true}); len(got) == 0 {
		t.Fatal("new legacy exception accepted")
	}
}
func TestHeadPathFollowsStagedRename(t *testing.T) {
	stagedRenameFixture(t)
	if got := headPath("frontend/src/app/new file.ts"); got != "frontend/src/old file.ts" {
		t.Fatalf("headPath = %q", got)
	}
	if got := headPath("frontend/quality/quality-ratchet.json"); got != "frontend/quality/quality-ratchet.json" {
		t.Fatalf("unrenamed path changed: %q", got)
	}
}
