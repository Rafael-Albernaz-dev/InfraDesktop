package qualityrules

import (
	"go/ast"
	"strconv"
	"strings"
)

// Decoded JSON field names are protocol keys, not role or family values.
// Resolve the identifier's actual declaration so a shadowed domain map cannot
// acquire this exception merely by reusing the name of a JSON map.
func decodedJSONWireKey(path string, literal *ast.BasicLit, parent ast.Node, stack []ast.Node) bool {
	if !strings.HasSuffix(path, "_test.go") {
		return false
	}
	index, ok := parent.(*ast.IndexExpr)
	if !ok || index.Index != literal {
		return false
	}
	name, ok := index.X.(*ast.Ident)
	if !ok || name.Obj == nil {
		return false
	}
	declaration, ok := name.Obj.Decl.(*ast.ValueSpec)
	if !ok {
		return false
	}
	mapping, ok := declaration.Type.(*ast.MapType)
	if !ok {
		return false
	}
	key, ok := mapping.Key.(*ast.Ident)
	if !ok || key.Name != "string" {
		return false
	}
	return isJSONRawMessage(mapping.Value, stack)
}
func isJSONRawMessage(value ast.Expr, stack []ast.Node) bool {
	selector, ok := value.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "RawMessage" {
		return false
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	file, ok := stack[0].(*ast.File)
	if !ok {
		return false
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "encoding/json" {
			continue
		}
		name := "json"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if owner.Name == name {
			return true
		}
	}
	return false
}
