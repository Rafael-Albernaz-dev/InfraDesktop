package qualityrules

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Span struct{ Start, End int }

func Overlaps(start, end int, changed []Span) bool {
	for _, span := range changed {
		if start <= span.End && end >= span.Start {
			return true
		}
	}
	return false
}

func Text(path string, spans []Span) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return []string{err.Error()}
	}
	var failures []string
	for i, line := range strings.Split(string(content), "\n") {
		if Overlaps(i+1, i+1, spans) && len([]rune(line)) > 140 {
			failures = append(failures, fmt.Sprintf("%s:%d: QG2 line >140", path, i+1))
		}
	}
	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
		return append(failures, frontendDomain(path, spans)...)
	}
	if !strings.HasSuffix(path, ".go") || isDomainModule(path) ||
		strings.HasPrefix(path, "scripts/") {
		return failures
	}
	return append(failures, goDomain(path, content, spans)...)
}

func isDomainModule(path string) bool {
	return strings.HasPrefix(path, "internal/modules/equipment/") ||
		strings.HasPrefix(path, "internal/modules/inventory/") ||
		strings.HasPrefix(path, "internal/modules/pppoe/") ||
		strings.HasPrefix(path, "internal/modules/radio/") ||
		strings.HasPrefix(path, "internal/modules/monitoring/")
}

func goDomain(path string, content []byte, spans []Span) []string {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, content, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var failures []string
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, node)
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		line := positions.Position(literal.Pos()).Line
		if watchedDomainLiteral(value) && Overlaps(line, line, spans) && !protocolToken(path, literal, parent, stack, value) {
			failures = append(failures, fmt.Sprintf("%s:%d: QG4 domain literal %q", path, line, value))
		}
		return true
	})
	return failures
}

// Wire keys and arbitrary device identifiers are not role/family values.
// The RouterOS `ap` flag is deliberately scoped to its observed parser location.
func protocolToken(path string, literal *ast.BasicLit, parent ast.Node, stack []ast.Node, value string) bool {
	if decodedJSONWireKey(path, literal, parent, stack) || wireFixtureValue(path, literal, parent, stack) {
		return true
	}
	if value != "ap" && value != "cpe" {
		return false
	}
	if field, ok := parent.(*ast.KeyValueExpr); ok && field.Value == literal {
		if name, ok := field.Key.(*ast.Ident); ok {
			switch name.Name {
			case "ID", "DeviceID", "AP", "CPE":
				return true
			}
		}
	}
	if path != "internal/adapters/routerssh/radio_standalone_parser.go" || value != "ap" {
		return false
	}
	index, ok := parent.(*ast.IndexExpr)
	if !ok || index.Index != literal {
		return false
	}
	source, ok := index.X.(*ast.Ident)
	return ok && source.Name == "peer"
}

func watchedDomainLiteral(value string) bool {
	switch value {
	case "auto", "auto-avulso", "unsupported", "routeros-wireless-v6", "airos-v6", "intelbras-wom",
		"ap", "cpe", "ptp", "ssh", "once", "measure", "live", "index", "UNSUPPORTED", "CONNECT_FAILED", "DEADLINE_EXCEEDED":
		return true
	}
	return strings.HasPrefix(value, "RADIO_") || strings.HasPrefix(value, "REMOTE_") || strings.HasPrefix(value, "SSH_")
}

func frontendDomain(path string, spans []Span) []string {
	raw, err := exec.Command("node", "scripts/quality-frontend.mjs", path).Output()
	if err != nil {
		return []string{path + ": QG4 frontend AST checker failed"}
	}
	var findings []struct {
		Line  int
		Value string
	}
	if err := json.Unmarshal(raw, &findings); err != nil {
		return []string{path + ": QG4 invalid AST checker result"}
	}
	var failures []string
	for _, finding := range findings {
		if Overlaps(finding.Line, finding.Line, spans) {
			failures = append(failures, fmt.Sprintf("%s:%d: QG4 frontend domain literal %q", path, finding.Line, finding.Value))
		}
	}
	return failures
}

// A raw JSON test producer is an independent wire oracle, not a typed domain input.
func wireFixtureValue(path string, literal *ast.BasicLit, parent ast.Node, stack []ast.Node) bool {
	if !strings.HasSuffix(path, "_test.go") || len(stack) < 3 {
		return false
	}
	field, ok := parent.(*ast.KeyValueExpr)
	if !ok || field.Value != literal {
		return false
	}
	key, ok := field.Key.(*ast.BasicLit)
	if !ok || key.Kind != token.STRING {
		return false
	}
	name, err := strconv.Unquote(key.Value)
	if err != nil {
		return false
	}
	switch name {
	case "mode", "family", "role", "transport":
	default:
		return false
	}
	composite, ok := stack[len(stack)-3].(*ast.CompositeLit)
	if !ok {
		return false
	}
	mapping, ok := composite.Type.(*ast.MapType)
	if !ok {
		return false
	}
	valueType, ok := mapping.Value.(*ast.Ident)
	return ok && valueType.Name == "any"
}
