package systemtext_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/systemtext"
)

const (
	loopPackageDir     = "../loop"
	toolConstantPrefix = "Tool"
)

func TestTheSystemTextDescribesEveryToolTheJudgementCallOffers(t *testing.T) {
	t.Parallel()

	described := map[string]string{
		loop.ToolRecall:    "recall tool",
		loop.ToolReadNode:  "read tool",
		loop.ToolWriteFile: "file tool",
	}
	nonToolConstants := map[string]bool{
		"ToolSourceNative":  true,
		"ToolSourceContent": true,
	}

	var declared []string
	for name, value := range toolConstantsTheLoopDeclares(t) {
		if !nonToolConstants[name] {
			declared = append(declared, value)
		}
	}
	slices.Sort(declared)

	if !slices.Equal(declared, slices.Sorted(maps.Keys(described))) {
		t.Fatalf("the loop's Tool-prefixed constants classify as the tools %v and this guard carries a phrase for %v; every such constant is either a tool the system text has to describe or one of the named non-tools, so a new one reds here until it is classified either way", declared, slices.Sorted(maps.Keys(described)))
	}

	for tool, phrase := range described {
		offered := "A " + phrase + " is available."
		if !strings.Contains(systemtext.Text, offered) {
			t.Errorf("the system text never offers the %q tool as %q; every tool the call offers is introduced there in that one neutral register, or the model is left to infer a tool from its schema alone, or to read an emphasis between them that the call does not intend", tool, offered)
		}
	}
}

func toolConstantsTheLoopDeclares(t *testing.T) map[string]string {
	t.Helper()

	packages, err := parser.ParseDir(token.NewFileSet(), loopPackageDir, func(file fs.FileInfo) bool {
		return !strings.HasSuffix(file.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing the loop package at %s failed: %v", loopPackageDir, err)
	}
	if len(packages) == 0 {
		t.Fatalf("parsed no packages at %s, so this guard would compare the system text against an empty tool set", loopPackageDir)
	}

	declared := map[string]string{}
	for _, parsed := range packages {
		for _, file := range parsed.Files {
			for _, decl := range file.Decls {
				declaration, ok := decl.(*ast.GenDecl)
				if !ok || declaration.Tok != token.CONST {
					continue
				}
				for _, spec := range declaration.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range value.Names {
						if !strings.HasPrefix(name.Name, toolConstantPrefix) {
							continue
						}
						if i >= len(value.Values) {
							t.Fatalf("the loop declares %s with no value of its own, so this guard cannot read what it would classify", name.Name)
						}
						literal, ok := value.Values[i].(*ast.BasicLit)
						if !ok || literal.Kind != token.STRING {
							t.Fatalf("the loop declares %s as something other than a string literal, so this guard cannot read what it would classify", name.Name)
						}
						unquoted, err := strconv.Unquote(literal.Value)
						if err != nil {
							t.Fatalf("the value of %s in the loop package is not a quoted string: %v", name.Name, err)
						}
						declared[name.Name] = unquoted
					}
				}
			}
		}
	}

	return declared
}
