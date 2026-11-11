package qualityrules

import (
	"fmt"
	"infradesktop/quality/go/qualitymetrics"
	"sort"
)

// Limits every authored Go/TS source meets once it carries no debt.
const (
	FunctionLimit   = 60
	ComplexityLimit = 15
)

// HasDebt reports whether a measured file is above any limit; only such files keep a baseline key.
func HasDebt(path string, m qualitymetrics.Metric) bool {
	return m.Lines > FileLimit(path) || m.LongLines > 0 || m.MaxFunction > FunctionLimit || m.MaxComplexity > ComplexityLimit
}

// BaselineLean keeps the baseline equal to the debt that still exists: a key without
// debt must go, and a ceiling above the measured debt must come down, so a gain is
// locked in the same change that made it. `make quality-tighten` rewrites the file.
func BaselineLean(measured, baseline qualitymetrics.Report) []string {
	var failures []string
	for _, path := range sortedPaths(baseline) {
		ceiling := baseline[path]
		m, exists := measured[path]
		switch {
		case !exists:
			failures = append(failures, fmt.Sprintf("baseline %s: file gone, remove key (make quality-tighten)", path))
		case !HasDebt(path, m):
			failures = append(failures, fmt.Sprintf("baseline %s: no debt left, remove key (make quality-tighten)", path))
		default:
			failures = append(failures, loosenedCeilings(path, m, ceiling)...)
		}
	}
	return failures
}

func loosenedCeilings(path string, m, ceiling qualitymetrics.Metric) []string {
	var failures []string
	check := func(name string, debt bool, measured, allowed int) {
		if debt && allowed > measured {
			failures = append(failures, fmt.Sprintf("baseline %s: %s ceiling %d above measured %d (make quality-tighten)",
				path, name, allowed, measured))
		}
	}
	check("lines", m.Lines > FileLimit(path), m.Lines, ceiling.Lines)
	check("long lines", m.LongLines > 0, m.LongLines, ceiling.LongLines)
	check("function", m.MaxFunction > FunctionLimit, m.MaxFunction, ceiling.MaxFunction)
	check("complexity", m.MaxComplexity > ComplexityLimit, m.MaxComplexity, ceiling.MaxComplexity)
	return failures
}

// Tightened is the baseline `make quality-tighten` writes: the measured values of the
// files that are already in the baseline and still carry debt. No key is ever added.
func Tightened(measured, baseline qualitymetrics.Report) qualitymetrics.Report {
	out := qualitymetrics.Report{}
	for path := range baseline {
		if m, exists := measured[path]; exists && HasDebt(path, m) {
			out[path] = qualitymetrics.Metric{Lines: m.Lines, LongLines: m.LongLines,
				MaxFunction: m.MaxFunction, MaxComplexity: m.MaxComplexity}
		}
	}
	return out
}

func sortedPaths(r qualitymetrics.Report) []string {
	paths := make([]string, 0, len(r))
	for path := range r {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
