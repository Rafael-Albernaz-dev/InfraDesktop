package qualityrules

import (
	"infradesktop/quality/go/qualitymetrics"
	"os"
	"os/exec"
	"strings"
	"testing"
)

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

func TestBaselineDebtNeverGrows(t *testing.T) {
	old := qualitymetrics.Report{"a.go": {Lines: 600, LongLines: 2, MaxFunction: 70, MaxComplexity: 20}}
	for _, after := range []qualitymetrics.Metric{
		{Lines: 601}, {LongLines: 3}, {MaxFunction: 71}, {MaxComplexity: 21},
	} {
		if len(BaselineGrowth(qualitymetrics.Report{"a.go": after}, old, nil)) == 0 {
			t.Fatal("grown debt accepted", after)
		}
	}
	for _, current := range []qualitymetrics.Report{old, {"a.go": {Lines: 9}, "new.go": {Lines: 999}}} {
		if len(BaselineGrowth(current, old, nil)) != 0 {
			t.Fatal("unchanged, reduced or new key refused")
		}
	}
}

func TestBaselineGrowthBelowTheLimitIsNotDebt(t *testing.T) {
	old := qualitymetrics.Report{"a.go": {Lines: 600, MaxFunction: 6, MaxComplexity: 3}}
	after := qualitymetrics.Report{"a.go": {Lines: 600, MaxFunction: FunctionLimit, MaxComplexity: ComplexityLimit}}
	if len(BaselineGrowth(after, old, nil)) != 0 {
		t.Fatal("growth within the function and complexity limits blocked")
	}
}

func TestBaselineKeyLeavesOnlyWithoutDebt(t *testing.T) {
	old := qualitymetrics.Report{"a.go": {Lines: 600}}
	if len(BaselineGrowth(qualitymetrics.Report{}, old, qualitymetrics.Report{"a.go": {Lines: 600}})) == 0 {
		t.Fatal("key of a file still in debt removed")
	}
	for _, measured := range []qualitymetrics.Report{{"a.go": {Lines: 100}}, {}} {
		if len(BaselineGrowth(qualitymetrics.Report{}, old, measured)) != 0 {
			t.Fatal("key of a clean or deleted file kept by force")
		}
	}
}

func TestBaselineLeanLocksTheGain(t *testing.T) {
	measured := qualitymetrics.Report{"debt.go": {Lines: 550, MaxFunction: 70}, "clean.go": {Lines: 40}}
	for name, baseline := range map[string]qualitymetrics.Report{
		"clean key":      {"debt.go": {Lines: 550, MaxFunction: 70}, "clean.go": {Lines: 40}},
		"gone key":       {"debt.go": {Lines: 550, MaxFunction: 70}, "gone.go": {Lines: 900}},
		"lines above":    {"debt.go": {Lines: 560, MaxFunction: 70}},
		"function above": {"debt.go": {Lines: 550, MaxFunction: 80}},
	} {
		if len(BaselineLean(measured, baseline)) == 0 {
			t.Fatal(name, "accepted")
		}
	}
	if got := BaselineLean(measured, Tightened(measured, qualitymetrics.Report{"debt.go": {}, "clean.go": {}})); len(got) != 0 {
		t.Fatal("tightened baseline rejected", got)
	}
	if tight := Tightened(measured, qualitymetrics.Report{"clean.go": {}}); len(tight) != 0 {
		t.Fatal("tighten added or kept a clean key", tight)
	}
}

func TestBaselineLinesMayGrowBelowFileLimit(t *testing.T) {
	old := qualitymetrics.Report{"a.go": {Lines: 10}}
	if len(BaselineGrowth(qualitymetrics.Report{"a.go": {Lines: 20}}, old, nil)) != 0 {
		t.Fatal("within-limit growth blocked")
	}
}

func stagedRenameFixture(t *testing.T) {
	t.Helper()
	isolateFromOuterRepository(t)
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

// isolateFromOuterRepository drops the GIT_* variables a hook exports (GIT_DIR,
// GIT_INDEX_FILE…). Inside a worktree's pre-commit they are absolute, and the fixture's
// git init/add/commit would otherwise write into the real repository being committed.
func isolateFromOuterRepository(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "GIT_") {
			continue
		}
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			// ignored: restoring the outer hook environment is best effort after the test.
			_ = os.Setenv(name, value)
		})
	}
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

// renamedMeasured is the measured debt of the staged-rename fixture: two long lines.
var renamedMeasured = qualitymetrics.Report{"frontend/src/app/new file.ts": {Lines: 10, LongLines: 2}}

func TestRenamedRatchetKeyKeepsHeadCeiling(t *testing.T) {
	stagedRenameFixture(t)
	writeGateFixture(t, "frontend/quality/quality-ratchet.json", `{"src/app/new file.ts":{"complexity":18}}`)
	if got := FrontendRatchet(); len(got) != 0 {
		t.Fatal(got)
	}
	writeGateFixture(t, "quality/baselines/quality-baseline.json", `{"frontend/src/app/new file.ts":{"lines":10,"LongLines":2}}`)
	if got := BaselineRatchet(renamedMeasured); len(got) != 0 {
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
	if got := BaselineRatchet(renamedMeasured); len(got) == 0 {
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
