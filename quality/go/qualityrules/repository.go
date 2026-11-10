package qualityrules

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"infradesktop/quality/go/qualitydup"
	"infradesktop/quality/go/qualitymetrics"
	"maps"
	"os"
	"strconv"
	"strings"
)

func duplication() (int, int, error) {
	count, err := qualitydup.Measure("frontend/src", "internal")
	if err != nil {
		return 0, 0, err
	}
	raw, err := os.ReadFile("quality/go/qualitydup/baseline.json")
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
	if raw, err := gitOutput("show", "HEAD:"+headPath("quality/go/qualitydup/baseline.json")); err == nil {
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
	raw, err := gitOutput("show", "HEAD:"+headPath("quality/go/quality.go"))
	if err != nil {
		return []string{err.Error()}
	}
	old, err := remapHeadKeys(legacyKeys(raw), "")
	if err != nil {
		return []string{err.Error()}
	}
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

// Only staged Git renames may transfer the authority of an existing HEAD key.
func remapHeadKeys[T any](previous map[string]T, prefix string) (map[string]T, error) {
	renames, err := stagedHeadRenames()
	if err != nil {
		return nil, err
	}
	result := maps.Clone(previous)
	for _, rename := range renames {
		from, to := rename[0], rename[1]
		if !strings.HasPrefix(from, prefix) || !strings.HasPrefix(to, prefix) {
			continue
		}
		from, to = strings.TrimPrefix(from, prefix), strings.TrimPrefix(to, prefix)
		value, exists := previous[from]
		if !exists {
			continue
		}
		// ignored: Membership alone determines a conflicting HEAD key.
		if _, collision := previous[to]; collision {
			return nil, fmt.Errorf("staged rename collides with HEAD key: %s", to)
		}
		delete(result, from)
		result[to] = value
	}
	return result, nil
}
func stagedHeadRenames() ([][2]string, error) {
	raw, err := gitOutput("diff", "--cached", "-M", "--name-status", "-z", "--diff-filter=R", "HEAD")
	if err != nil || len(raw) == 0 {
		return nil, err
	}
	fields := strings.Split(string(raw), "\x00")
	if fields[len(fields)-1] != "" || (len(fields)-1)%3 != 0 {
		return nil, fmt.Errorf("malformed staged Git renames")
	}
	var renames [][2]string
	for i := 0; i < len(fields)-1; i += 3 {
		status, from, to := fields[i], fields[i+1], fields[i+2]
		if !strings.HasPrefix(status, "R") || from == "" || to == "" {
			return nil, fmt.Errorf("invalid staged Git rename")
		}
		// ignored: Parsing validates the score; the value does not authorize a key.
		if _, err := strconv.Atoi(status[1:]); err != nil {
			return nil, err
		}
		renames = append(renames, [2]string{from, to})
	}
	return renames, nil
}

// headPath gives the HEAD path of a file staged as a rename, so a gate that moves
// still compares against its own HEAD version in the commit that moves it.
func headPath(path string) string {
	renames, err := stagedHeadRenames()
	if err != nil {
		return path
	}
	for _, rename := range renames {
		if rename[1] == path {
			return rename[0]
		}
	}
	return path
}
