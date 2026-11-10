package qualityrules

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

func GoLimits(path string, content []byte, spans []Span) []string {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, content, parser.ParseComments)
	if err != nil {
		return []string{err.Error()}
	}
	var failures []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			if n.Body != nil && Overlaps(positions.Position(n.Pos()).Line, positions.Position(n.End()).Line, spans) {
				failures = append(failures, parameterLimit(path, n.Type.Params)...)
			}
		case *ast.FuncLit:
			if Overlaps(positions.Position(n.Pos()).Line, positions.Position(n.End()).Line, spans) {
				failures = append(failures, parameterLimit(path, n.Type.Params)...)
			}
		case *ast.AssignStmt:
			line := positions.Position(n.Pos()).Line
			if !strings.HasSuffix(path, "_test.go") && Overlaps(line, line, spans) &&
				discardsValue(n.Lhs) && !ignoredComment(file, positions, line) {
				failures = append(failures, fmt.Sprintf("%s:%d: discarded value needs // ignored: reason", path, line))
			}
		}
		return true
	})
	return failures
}
func parameterLimit(path string, fields *ast.FieldList) []string {
	count := 0
	for _, field := range fields.List {
		count += max(1, len(field.Names))
	}
	if count > 6 {
		return []string{fmt.Sprintf("%s: function has %d parameters, maximum 6", path, count)}
	}
	return nil
}
func discardsValue(values []ast.Expr) bool {
	for _, value := range values {
		if id, ok := value.(*ast.Ident); ok && id.Name == "_" {
			return true
		}
	}
	return false
}
func ignoredComment(file *ast.File, positions *token.FileSet, line int) bool {
	for _, group := range file.Comments {
		if end := positions.Position(group.End()).Line; end == line || end == line-1 {
			for _, comment := range group.List {
				text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "// ignored:"))
				if strings.HasPrefix(comment.Text, "// ignored:") && text != "" {
					return true
				}
			}
		}
	}
	return false
}
