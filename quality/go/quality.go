// quality measures authored sources and enforces a ratchet against an explicit baseline.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"infradesktop/quality/go/qualitymetrics"
	"infradesktop/quality/go/qualityrules"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type metric = qualitymetrics.Metric
type report = qualitymetrics.Report
type span = qualityrules.Span

var forbiddenTestFunction = regexp.MustCompile(`(?i)(L07|Avulso|Live2|Piloto|CPU90|L06C|L06D|BF03|BF04)`)
var forbidden = regexp.MustCompile(`(?i)(^|_)(j|live2|piloto|cpu90|avulso|l06c|l06d|bf03|bf04)(_|$)`)

func main() {
	reportOnly := flag.Bool("report", false, "print quality measurements")
	baseline := flag.Bool("baseline", false, "record measurements before refactoring")
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
	changed, err := qualityrules.ChangedSources()
	fatal(err)
	if *reportOnly {
		fatal(qualityrules.PrintReport(measured))
		return
	}
	failures := validate(measured, previous, changed)
	failures = append(failures, qualityrules.BaselineRatchet()...)
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
func overlaps(start, end int, changed []span) bool { return qualityrules.Overlaps(start, end, changed) }

func validate(current, previous report, changed map[string][]span) []string {
	var failures []string
	for path, m := range current {
		old, exists := previous[path]
		if !exists {
			if origin, renamed := renamedRadioTestOrigins[path]; renamed {
				old, exists = previous[origin]
			}
		}
		if exists {
			failures = append(failures, validateRatchet(path, m, old)...)
		}
		if spans, touched := changed[path]; touched {
			failures = append(failures, validateChanged(path, m, exists, spans)...)
		}
	}
	return failures
}
func validateRatchet(path string, current, previous metric) []string {
	if (current.Lines <= previous.Lines || current.Lines <= qualityrules.FileLimit(path)) && current.LongLines <= previous.LongLines &&
		current.MaxFunction <= previous.MaxFunction && current.MaxComplexity <= previous.MaxComplexity {
		return nil
	}
	return []string{fmt.Sprintf("%s: ratchet worsened: lines %d→%d long %d→%d function %d→%d complexity %d→%d",
		path, previous.Lines, current.Lines, previous.LongLines, current.LongLines,
		previous.MaxFunction, current.MaxFunction, previous.MaxComplexity, current.MaxComplexity)}
}
func validateChanged(path string, m metric, baselineExists bool, spans []span) []string {
	var failures []string
	if strings.HasSuffix(path, ".ts") {
		err := exec.Command("git", "cat-file", "-e", "HEAD:"+path).Run()
		if qualityrules.ExistingTSWithoutBaseline(err == nil, baselineExists) {
			failures = append(failures, path+": BLOCK initial TypeScript baseline absent")
		}
	}
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
		if f.Lines > 60 || f.Complexity > 15 {
			failures = append(failures, fmt.Sprintf("%s:%d: QG2/QG3 %s lines=%d complexity=%d", path, f.Start, f.Name, f.Lines, f.Complexity))
		}
		if strings.HasPrefix(f.Name, "Test") && forbiddenTestFunction.MatchString(f.Name) {
			failures = append(failures, fmt.Sprintf("%s:%d: QG5 %s", path, f.Start, f.Name))
		}
	}
	return failures
}
func validateText(path string, spans []span) []string { return qualityrules.Text(path, spans) }

// Behavior renames retain the entry metrics of their original file.
var renamedRadioTestOrigins = map[string]string{
	"internal/adapters/routerssh/radio_standalone_identity_test.go":        "internal/adapters/routerssh/radio_avulso_peer_test.go",
	"internal/adapters/routerssh/radioparse/radio_station_parser_test.go":  "internal/adapters/routerssh/radio_avulso_test.go",
	"internal/adapters/routerssh/radio_reopen_handshake_test.go":           "internal/adapters/routerssh/radio_j_test.go",
	"internal/adapters/routerssh/radioparse/radio_partial_failure_test.go": "internal/adapters/routerssh/radio_partial_j_test.go",
	"internal/modules/equipment/radio_standalone_service_test.go":          "internal/modules/equipment/radio_avulso_test.go",
	"internal/modules/equipment/radio_cpu_gate_test.go":                    "internal/modules/equipment/radio_cpu90_test.go",
	"internal/modules/equipment/radio_measurement_gate_test.go":            "internal/modules/equipment/radio_live2_test.go",
	"internal/modules/equipment/radio_provenance_test.go":                  "internal/modules/equipment/radio_piloto_test.go",
	"internal/adapters/ipc/radio_standalone_test.go":                       "internal/adapters/ipc/radio_avulso_test.go",
	"internal/adapters/ipc/radio_unsupported_family_test.go":               "internal/adapters/ipc/radio_piloto_test.go",
	"internal/adapters/storage/equipment_radio_credentials_test.go":        "internal/adapters/storage/equipment_j_test.go",
}
