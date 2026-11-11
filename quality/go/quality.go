// quality measures authored sources and enforces a ratchet against an explicit baseline.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"infradesktop/quality/go/qualitymetrics"
	"infradesktop/quality/go/qualityrules"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type metric = qualitymetrics.Metric
type report = qualitymetrics.Report
type span = qualityrules.Span

// Slice codes are not behavior: a touched test named after one must be renamed.
var forbiddenTestFunction = regexp.MustCompile(`(?i:L0[0-9]|BF0[0-9]|Avulso|Live2|Piloto|CPU90)|Focal`)
var forbidden = regexp.MustCompile(`(?i)(^|_)(j|live2|piloto|cpu90|avulso|l06c|l06d|bf03|bf04)(_|$)`)

func main() {
	reportOnly := flag.Bool("report", false, "print quality measurements")
	baseline := flag.Bool("baseline", false, "record measurements before refactoring")
	tighten := flag.Bool("tighten", false, "rewrite the baseline to the debt that still exists")
	flag.Parse()
	measured, err := qualitymetrics.MeasureSources()
	fatal(err)
	if *baseline {
		writeBaseline(measured)
		return
	}
	raw, err := os.ReadFile("quality/baselines/quality-baseline.json")
	fatal(err)
	var previous report
	fatal(json.Unmarshal(raw, &previous))
	if *tighten {
		writeTightened(measured, previous)
		return
	}
	changed, err := qualityrules.ChangedSources()
	fatal(err)
	if *reportOnly {
		fatal(qualityrules.PrintReport(measured))
		return
	}
	failures := validate(measured, previous, changed)
	failures = append(failures, qualityrules.BaselineRatchet(measured)...)
	failures = append(failures, qualityrules.DuplicationGate()...)
	failures = append(failures, qualityrules.FrontendRatchet()...)
	failures = append(failures, qualityrules.LegacyGate(legacyRatchetOnly)...)
	for _, failure := range failures {
		fmt.Println(failure)
	}
	if len(failures) > 0 {
		fmt.Printf("quality: BLOCK (%d findings)\n", len(failures))
		os.Exit(1)
	}
	fmt.Println("quality: PASS")
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
func writeBaseline(measured report) {
	// ignored: Only existence matters before writing the first baseline.
	if _, err := os.Stat("quality/baselines/quality-baseline.json"); err == nil {
		fatal(fmt.Errorf("baseline already exists; refusing overwrite"))
	}
	raw, err := json.MarshalIndent(measured, "", "  ")
	fatal(err)
	fatal(os.WriteFile("quality/baselines/quality-baseline.json", append(raw, '\n'), 0644))
	fmt.Printf("baseline: %d files\n", len(measured))
}

// writeTightened keeps only the files still in debt, at their measured values. Keys of
// files renamed in the staged change follow the file.
func writeTightened(measured, previous report) {
	moved := map[string]string{}
	renames, err := qualityrules.StagedRenames()
	fatal(err)
	for _, pair := range renames {
		moved[pair[0]] = pair[1]
	}
	current := report{}
	for path, m := range previous {
		if renamed, ok := moved[path]; ok {
			path = renamed
		}
		current[path] = m
	}
	tight := qualityrules.Tightened(measured, current)
	raw, err := json.MarshalIndent(tight, "", "  ")
	fatal(err)
	fatal(os.WriteFile("quality/baselines/quality-baseline.json", append(raw, '\n'), 0644))
	fmt.Printf("baseline: %d files with debt (was %d keys)\n", len(tight), len(previous))
}
func overlaps(start, end int, changed []span) bool { return qualityrules.Overlaps(start, end, changed) }

func validate(current, previous report, changed map[string][]span) []string {
	var failures []string
	for path, m := range current {
		old, exists := previous[path]
		if exists {
			failures = append(failures, validateRatchet(path, m, old)...)
		}
		if spans, touched := changed[path]; touched {
			failures = append(failures, validateChanged(path, m, spans)...)
		}
	}
	return failures
}
func validateRatchet(path string, current, previous metric) []string {
	within := func(now, before, limit int) bool { return now <= before || now <= limit }
	if within(current.Lines, previous.Lines, qualityrules.FileLimit(path)) && current.LongLines <= previous.LongLines &&
		within(current.MaxFunction, previous.MaxFunction, qualityrules.FunctionLimit) &&
		within(current.MaxComplexity, previous.MaxComplexity, qualityrules.ComplexityLimit) {
		return nil
	}
	return []string{fmt.Sprintf("%s: ratchet worsened: lines %d→%d long %d→%d function %d→%d complexity %d→%d",
		path, previous.Lines, current.Lines, previous.LongLines, current.LongLines,
		previous.MaxFunction, current.MaxFunction, previous.MaxComplexity, current.MaxComplexity)}
}
func validateChanged(path string, m metric, spans []span) []string {
	var failures []string
	if strings.HasSuffix(path, ".go") {
		content, err := os.ReadFile(path)
		if err != nil {
			return []string{err.Error()}
		}
		failures = append(failures, qualityrules.GoLimits(path, content, spans)...)
	}
	if legacyRatchetOnly[path] {
		spans = radioFunctionSpans(m, spans)
	} else {
		limit := qualityrules.FileLimit(path)
		if (strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".tsx")) && m.Lines > limit {
			failures = append(failures, fmt.Sprintf("%s: QG2 file %d > %d", path, m.Lines, limit))
		}
	}
	if len(spans) == 0 {
		return failures
	}
	failures = append(failures, validateFunctions(path, m, spans)...)
	return append(failures, validateText(path, spans)...)
}

// Explicit user exception: only these reported non-radio legacy files use the
// original growth ratchet. Every radio file and named radio helper stays strict.
var legacyRatchetOnly = map[string]bool{
	"frontend/src/app/pages/equipment.tsx":              true,
	"frontend/src/app/pages/equipment.test.tsx":         true,
	"internal/modules/equipment/service.go":             true,
	"internal/adapters/storage/equipment.go":            true,
	"internal/adapters/storage/units.go":                true,
	"internal/adapters/storage/device_insights_test.go": true,
	"internal/testsupport/sshfixture/server.go":         true,
	"internal/adapters/presentation/equipment.go":       true,
}

func radioFunctionSpans(m metric, changed []span) []span {
	var strict []span
	for _, f := range m.Functions {
		if strings.Contains(strings.ToLower(f.Name), "radio") && overlaps(f.Start, f.End, changed) {
			strict = append(strict, span{Start: f.Start, End: f.End})
		}
	}
	return strict
}
func validateFunctions(path string, m metric, spans []span) []string {
	failures := []string{}
	if strings.HasSuffix(path, "_test.go") && forbidden.MatchString(strings.TrimSuffix(filepath.Base(path), ".go")) {
		failures = append(failures, path+": QG5 dispatch name")
	}
	for _, f := range m.Functions {
		if !overlaps(f.Start, f.End, spans) {
			continue
		}
		if f.Lines > qualityrules.FunctionLimit || f.Complexity > qualityrules.ComplexityLimit {
			failures = append(failures, fmt.Sprintf("%s:%d: QG2/QG3 %s lines=%d complexity=%d", path, f.Start, f.Name, f.Lines, f.Complexity))
		}
		if strings.HasPrefix(f.Name, "Test") && forbiddenTestFunction.MatchString(f.Name) {
			failures = append(failures, fmt.Sprintf("%s:%d: QG5 %s", path, f.Start, f.Name))
		}
	}
	return failures
}
func validateText(path string, spans []span) []string { return qualityrules.Text(path, spans) }
