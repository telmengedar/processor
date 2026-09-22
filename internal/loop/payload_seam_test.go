package loop

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func parseTheLoopSources(t *testing.T) map[string]*ast.FuncDecl {
	t.Helper()

	notATest := func(info fs.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }

	packages, err := parser.ParseDir(token.NewFileSet(), ".", notATest, 0)
	if err != nil {
		t.Fatalf("parsing the loop package failed: %v", err)
	}

	functions := make(map[string]*ast.FuncDecl)
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Body != nil {
					functions[fn.Name.Name] = fn
				}
			}
		}
	}
	if len(functions) == 0 {
		t.Fatalf("parsed no functions from the loop package, so this guard would pass vacuously")
	}
	return functions
}

func callsByName(fn *ast.FuncDecl) map[string]bool {
	called := make(map[string]bool)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok {
				called[ident.Name] = true
			}
		}
		return true
	})
	return called
}

func readsACandidatesBody(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "Content" && selector.Sel.Name != "Substance") {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "anchor" {
			return true
		}
		found = true
		return false
	})
	return found
}

func TestOnlyAdmissionAndThePayloadSeamReadACandidatesBody(t *testing.T) {
	t.Parallel()

	allowed := []string{"admit", "renderedPayload"}
	functions := parseTheLoopSources(t)

	for name, fn := range functions {
		if !readsACandidatesBody(fn) {
			continue
		}
		if !slices.Contains(allowed, name) {
			t.Errorf("%s reads a candidate's Content or Substance directly; only %v may, because the cap is charged against renderedPayload and a renderer that reaches past it writes bytes nobody counted, in either form", name, allowed)
		}
	}

	for _, name := range allowed {
		fn, ok := functions[name]
		if !ok {
			t.Fatalf("%s is not in the loop package at all, so the list this guard permits is stale and the guard admits whatever replaced it", name)
		}
		if !readsACandidatesBody(fn) {
			t.Errorf("%s no longer reads a candidate's Content or Substance, so permitting it here widens the guard for nothing", name)
		}
	}
}

func reaches(functions map[string]*ast.FuncDecl, from, target string) bool {
	seen := map[string]bool{}
	pending := []string{from}
	for len(pending) > 0 {
		name := pending[0]
		pending = pending[1:]
		if name == target {
			return true
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		fn, ok := functions[name]
		if !ok {
			continue
		}
		for called := range callsByName(fn) {
			pending = append(pending, called)
		}
	}
	return false
}

func TestTheBlockAndTheToolResultBothTakeTheirRowsFromThePayloadSeam(t *testing.T) {
	t.Parallel()

	functions := parseTheLoopSources(t)

	if _, ok := functions["renderedPayload"]; !ok {
		t.Fatalf("renderedPayload is not in the loop package at all, so this guard would pass vacuously")
	}

	for _, name := range []string{"renderBlock", "RenderToolResult"} {
		if _, ok := functions[name]; !ok {
			t.Fatalf("%s is not in the loop package at all, so this guard would pass vacuously", name)
		}
		if !reaches(functions, name, "renderedPayload") {
			t.Errorf("%s reaches renderedPayload by no path, so the bytes it writes and the bytes admission charged against the ceiling are two independent quantities that agree only by accident", name)
		}
	}
}
