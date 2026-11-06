package qualityrules

import (
	"encoding/json"
	"fmt"
	"infradesktop/scripts/qualitymetrics"
	"os"
)

// BaselineGrowth checks the numeric ceilings used by the quality gate.
func BaselineGrowth(current, previous qualitymetrics.Report) []string {
	var failures []string
	for path, before := range previous {
		after, exists := current[path]
		if !exists {
			continue
		}
		if after.Lines > before.Lines || after.LongLines > before.LongLines ||
			after.MaxFunction > before.MaxFunction || after.MaxComplexity > before.MaxComplexity {
			failures = append(failures, fmt.Sprintf("baseline %s: ceiling increased", path))
		}
	}
	return failures
}

func BaselineRatchet() []string {
	raw, err := os.ReadFile("scripts/quality-baseline.json")
	if err != nil {
		return []string{err.Error()}
	}
	old, err := gitOutput("show", "HEAD:scripts/quality-baseline.json")
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
	return BaselineGrowth(current, previous)
}
