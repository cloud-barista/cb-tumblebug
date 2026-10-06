package label

// Tag calls to a CSP with no TagHandler are answered with an error every time,
// and CB-Spider builds a fresh SDK connection per call because it caches none —
// so the gate has to sit on every path that reaches it, not only the reads.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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

func TestCspTagSyncEnvControl(t *testing.T) {
	// Save original env vars
	origEnable := os.Getenv("TB_ENABLE_CSP_TAG_SYNC")
	origSync := os.Getenv("TB_SYNC_CSP_TAG")
	defer func() {
		os.Setenv("TB_ENABLE_CSP_TAG_SYNC", origEnable)
		os.Setenv("TB_SYNC_CSP_TAG", origSync)
	}()

	// 1. Default should be false (disabled)
	os.Unsetenv("TB_ENABLE_CSP_TAG_SYNC")
	os.Unsetenv("TB_SYNC_CSP_TAG")
	if isCspTagSyncEnabled() {
		t.Errorf("expected isCspTagSyncEnabled() to be false by default")
	}
	if isCSPSyncEnabled("vm", "non-existent-conn") {
		t.Errorf("expected isCSPSyncEnabled() to be false when env is unset")
	}

	// 2. TB_ENABLE_CSP_TAG_SYNC=true enables it
	os.Setenv("TB_ENABLE_CSP_TAG_SYNC", "true")
	if !isCspTagSyncEnabled() {
		t.Errorf("expected isCspTagSyncEnabled() to be true with TB_ENABLE_CSP_TAG_SYNC=true")
	}

	// 3. Case insensitivity ("True")
	os.Setenv("TB_ENABLE_CSP_TAG_SYNC", "True")
	if !isCspTagSyncEnabled() {
		t.Errorf("expected isCspTagSyncEnabled() to be true with TB_ENABLE_CSP_TAG_SYNC=True")
	}

	// 4. "1" enables it
	os.Setenv("TB_ENABLE_CSP_TAG_SYNC", "1")
	if !isCspTagSyncEnabled() {
		t.Errorf("expected isCspTagSyncEnabled() to be true with TB_ENABLE_CSP_TAG_SYNC=1")
	}

	// 5. Fallback TB_SYNC_CSP_TAG=true
	os.Unsetenv("TB_ENABLE_CSP_TAG_SYNC")
	os.Setenv("TB_SYNC_CSP_TAG", "true")
	if !isCspTagSyncEnabled() {
		t.Errorf("expected isCspTagSyncEnabled() to be true with TB_SYNC_CSP_TAG=true")
	}

	// 6. Explicitly false
	os.Setenv("TB_ENABLE_CSP_TAG_SYNC", "false")
	if isCspTagSyncEnabled() {
		t.Errorf("expected isCspTagSyncEnabled() to be false with TB_ENABLE_CSP_TAG_SYNC=false")
	}
}
