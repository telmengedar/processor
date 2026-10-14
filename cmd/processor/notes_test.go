package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestTheServiceGivesTheTurnTheGraphClientAsItsNotePort(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing main.go failed: %v", err)
	}

	wired := false
	ast.Inspect(file, func(n ast.Node) bool {
		assignment, ok := n.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		target, ok := assignment.Lhs[0].(*ast.SelectorExpr)
		if !ok || target.Sel.Name != "Notes" {
			return true
		}
		if receiver, ok := target.X.(*ast.Ident); ok && receiver.Name == "turn" {
			if source, ok := assignment.Rhs[0].(*ast.Ident); ok && source.Name == "graph" {
				wired = true
			}
		}
		return true
	})

	if !wired {
		t.Fatal("the service never gives its turn the graph client as the note port, so no config can enable the note tool")
	}
}
