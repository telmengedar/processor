package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestTheServiceStartsTheDeferredFillWorker(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing main.go failed: %v", err)
	}

	started := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Start" {
			if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "fillPort" {
				started = true
			}
		}
		return true
	})

	if !started {
		t.Fatal("the service never starts its fill port's worker, so every creation deferred past a turn's wait is queued and never run")
	}
}
