package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestAMeasuredRunNeverStartsTheDeferredFillWorker(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing main.go failed: %v", err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Start" {
			t.Errorf("a measured run calls Start at %v: deferred creations between measured turns would load the shared host and contaminate the timing being measured", fset.Position(call.Pos()))
		}
		return true
	})
}
