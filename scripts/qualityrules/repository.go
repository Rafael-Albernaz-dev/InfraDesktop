package qualityrules

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"infradesktop/scripts/qualitydup"
	"infradesktop/scripts/qualitymetrics"
	"os"
	"strconv"
	"strings"
)

func duplication() (int, int, error) {
	count, err := qualitydup.Measure("frontend/src", "internal")
	if err != nil {
		return 0, 0, err
	}
	raw, err := os.ReadFile("scripts/qualitydup/baseline.json")
	if err != nil {
		return 0, 0, err
	}
	var ceiling int
	err = json.Unmarshal(raw, &ceiling)
	return count, ceiling, err
}
func DuplicationGate() []string {
	count, ceiling, err := duplication()
	if err != nil {
		return []string{err.Error()}
	}
	var failures []string
	if qualitydup.Exceeds(count, ceiling) {
		failures = append(failures, "G7 duplicated windows increased")
	}
	if raw, err := gitOutput("show", "HEAD:scripts/qualitydup/baseline.json"); err == nil {
		var old int
		if json.Unmarshal(raw, &old) != nil || ceiling > old {
			failures = append(failures, "G7 ceiling increased")
		}
	}
	return failures
}
func PrintReport(measured qualitymetrics.Report) error {
	count, ceiling, err := duplication()
	if err != nil {
		return err
	}
	fmt.Printf("duplication: %d windows; ceiling %d\n", count, ceiling)
	for path, m := range measured {
		if strings.HasPrefix(path, "frontend/") {
			continue
		}
		fmt.Printf("%s lines=%d function=%d complexity=%d\n", path, m.Lines, m.MaxFunction, m.MaxComplexity)
	}
	return nil
}
func LegacyGate(current map[string]bool) []string {
	raw, err := gitOutput("show", "HEAD:scripts/quality.go")
	if err != nil {
		return []string{err.Error()}
	}
	old := legacyKeys(raw)
	var failures []string
	for path := range current {
		if !old[path] {
			failures = append(failures, "G6 new legacyRatchetOnly entry: "+path)
		}
	}
	return failures
}
func legacyKeys(source []byte) map[string]bool {
	keys := map[string]bool{}
	file, err := parser.ParseFile(token.NewFileSet(), "quality.go", source, 0)
	if err != nil {
		return keys
	}
	ast.Inspect(file, func(node ast.Node) bool {
		value, ok := node.(*ast.ValueSpec)
		if !ok || len(value.Names) != 1 || value.Names[0].Name != "legacyRatchetOnly" {
			return true
		}
		literal, ok := value.Values[0].(*ast.CompositeLit)
		if !ok {
			return false
		}
		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.BasicLit)
			if !ok {
				continue
			}
			name, err := strconv.Unquote(key.Value)
			if err == nil {
				keys[name] = true
			}
		}
		return false
	})
	return keys
}
