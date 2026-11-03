package qualitymetrics

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Function struct {
	Name                          string
	Start, End, Lines, Complexity int
}
type Metric struct {
	Lines, LongLines, MaxFunction, MaxComplexity int
	Functions                                    []Function
}
type Report map[string]Metric

func authored(path string, entry os.DirEntry) bool {
	if !entry.IsDir() {
		return true
	}
	name := entry.Name()
	return !strings.HasPrefix(name, ".") && name != "node_modules" && name != "build" && name != "dist" && name != "docs" &&
		name != "wailsjs" && name != "vendor"
}
func MeasureSources() (Report, error) {
	result := Report{}
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !authored(path, entry) && path != "." {
			return filepath.SkipDir
		}
		if entry.IsDir() || !(strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts")) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m := Metric{Lines: len(strings.Split(strings.TrimSuffix(string(content), "\n"), "\n"))}
		for _, line := range strings.Split(string(content), "\n") {
			if len([]rune(line)) > 140 {
				m.LongLines++
			}
		}
		if strings.HasSuffix(path, ".go") {
			if err := measureGo(path, content, &m); err != nil {
				return err
			}
		}
		result[filepath.ToSlash(path)] = m
		return nil
	})
	return result, err
}
func measureGo(path string, content []byte, m *Metric) error {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, content, 0)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Body == nil {
			continue
		}
		start, end := positions.Position(f.Pos()).Line, positions.Position(f.End()).Line
		measured := Function{Name: f.Name.Name, Start: start, End: end, Lines: end - start + 1, Complexity: complexity(f.Body)}
		m.Functions = append(m.Functions, measured)
		m.MaxFunction = max(m.MaxFunction, measured.Lines)
		m.MaxComplexity = max(m.MaxComplexity, measured.Complexity)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.FuncLit)
		if !ok || literal.Body == nil {
			return true
		}
		start, end := positions.Position(literal.Pos()).Line, positions.Position(literal.End()).Line
		measured := Function{Name: fmt.Sprintf("function literal at %d", start), Start: start, End: end,
			Lines: end - start + 1, Complexity: complexity(literal.Body)}
		m.Functions = append(m.Functions, measured)
		m.MaxFunction = max(m.MaxFunction, measured.Lines)
		m.MaxComplexity = max(m.MaxComplexity, measured.Complexity)
		return true
	})

	return nil
}
func complexity(body ast.Node) int {
	count := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			count++
		case *ast.CaseClause:
			if len(v.List) > 0 {
				count++
			}
		case *ast.CommClause:
			if v.Comm != nil {
				count++
			}
		case *ast.BinaryExpr:
			if v.Op == token.LAND || v.Op == token.LOR {
				count++
			}
		}
		return true
	})
	return count
}
