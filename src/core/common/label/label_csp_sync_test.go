package label

// Tag calls to a CSP with no TagHandler are answered with an error every time,
// and CB-Spider builds a fresh SDK connection per call because it caches none —
// so the gate has to sit on every path that reaches it, not only the reads.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEveryCSPTagPathIsGated(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "label.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	gated := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "isCSPSyncEnabled" {
				gated[fn.Name.Name] = true
			}
			return true
		})
	}

	for _, name := range []string{
		"UpdateCSPResourceLabel",
		"RemoveCSPResourceLabel",
		"ListCSPResourceLabel",
		"MergeCSPResourceLabel",
	} {
		if !gated[name] {
			t.Errorf("%s reaches the CSP tag API without calling isCSPSyncEnabled", name)
		}
	}
}
