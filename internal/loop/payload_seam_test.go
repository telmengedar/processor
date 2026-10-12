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

func rootedAtTheAnchorOrAResult(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name == "anchor" || x.Name == "result"
	case *ast.SelectorExpr:
		return x.Sel.Name == "result"
	}
	return false
}

func readsACandidatesField(fn *ast.FuncDecl, field string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != field {
			return true
		}
		if rootedAtTheAnchorOrAResult(selector.X) {
			return true
		}
		found = true
		return false
	})
	return found
}

func TestOnlyAdmissionAndThePayloadSeamReadACandidatesBody(t *testing.T) {
	t.Parallel()

	allowed := map[string][]string{
		"Content":   {"admitUnheld"},
		"Substance": {"admitUnheld", "renderedPayload", "lacksSubstance", "withCreated"},
	}
	functions := parseTheLoopSources(t)

	for field, readers := range allowed {
		for name, fn := range functions {
			if readsACandidatesField(fn, field) && !slices.Contains(readers, name) {
				t.Errorf("%s reads a candidate's %s directly; only %v may, because admission charges what renderedPayload returns and a renderer that reaches past it writes bytes nobody counted", name, field, readers)
			}
		}

		for _, name := range readers {
			fn, ok := functions[name]
			if !ok {
				t.Fatalf("%s is not in the loop package at all, so the list this guard permits is stale and the guard admits whatever replaced it", name)
			}
			if !readsACandidatesField(fn, field) && field == "Content" {
				t.Errorf("%s no longer reads a candidate's %s, so permitting it here widens the guard for nothing", name, field)
			}
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

func TestTheBlockTakesItsRowsFromThePayloadSeam(t *testing.T) {
	t.Parallel()

	functions := parseTheLoopSources(t)

	if _, ok := functions["renderedPayload"]; !ok {
		t.Fatalf("renderedPayload is not in the loop package at all, so this guard would pass vacuously")
	}

	for _, name := range []string{"renderBlock"} {
		if _, ok := functions[name]; !ok {
			t.Fatalf("%s is not in the loop package at all, so this guard would pass vacuously", name)
		}
		if !reaches(functions, name, "renderedPayload") {
			t.Errorf("%s reaches renderedPayload by no path, so the bytes it writes and the bytes admission charged are two independent quantities that agree only by accident", name)
		}
	}
}
