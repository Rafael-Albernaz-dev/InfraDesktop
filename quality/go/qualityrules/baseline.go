package qualityrules

import (
	"encoding/json"
	"fmt"
	"infradesktop/quality/go/qualitymetrics"
	"os"
	"strings"
)

// BaselineGrowth checks the numeric ceilings used by the quality gate: debt never grows.
// A key may leave only when its file is gone or measures without debt.
func BaselineGrowth(current, previous, measured qualitymetrics.Report) []string {
	var failures []string
	for path, before := range previous {
		after, exists := current[path]
		if !exists {
			if m, present := measured[path]; present && HasDebt(path, m) {
				failures = append(failures, fmt.Sprintf("baseline %s: key removed while the file still has debt", path))
			}
			continue
		}
		if grew(after.Lines, before.Lines, FileLimit(path)) || after.LongLines > before.LongLines ||
			grew(after.MaxFunction, before.MaxFunction, FunctionLimit) ||
			grew(after.MaxComplexity, before.MaxComplexity, ComplexityLimit) {
			failures = append(failures, fmt.Sprintf("baseline %s: ceiling increased", path))
		}
	}
	return failures
}

// grew is growth beyond both the old ceiling and the limit; growth below the limit is not debt.
func grew(after, before, limit int) bool { return after > before && after > limit }

func BaselineRatchet(measured qualitymetrics.Report) []string {
	raw, err := os.ReadFile("quality/baselines/quality-baseline.json")
	if err != nil {
		return []string{err.Error()}
	}
	old, err := gitOutput("show", "HEAD:"+headPath("quality/baselines/quality-baseline.json"))
	if err != nil {
		return []string{err.Error()}
	}
	var current, previous qualitymetrics.Report
	if err := json.Unmarshal(raw, &current); err != nil {
		return []string{err.Error()}
	}
	if err := json.Unmarshal(old, &previous); err != nil {
		return []string{err.Error()}
	}
	previous, err = remapHeadKeys(previous, "")
	if err != nil {
		return []string{err.Error()}
	}
	return append(BaselineGrowth(current, previous, measured), BaselineLean(measured, current)...)
}

// FileLimit is shared by the baseline ratchet and changed-file validation.
func FileLimit(path string) int {
	if strings.HasSuffix(path, ".tsx") {
		return 600
	}
	return 500
}
