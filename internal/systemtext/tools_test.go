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
		if !strings.Contains(systemtext.Compose([]string{tool}), offered) {
			t.Errorf("the system text never offers the %q tool as %q; every tool the call offers is introduced there in that one neutral register, or the model is left to infer a tool from its schema alone, or to read an emphasis between them that the call does not intend", tool, offered)
		}
	}
}

func TestTheSystemTextNamesExactlyTheOfferedTools(t *testing.T) {
	t.Parallel()

	phrases := map[string]string{
		loop.ToolRecall:    "A recall tool is available.",
		loop.ToolReadNode:  "A read tool is available.",
		loop.ToolWriteFile: "A file tool is available.",
	}
	every := []string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}

	for mask := 0; mask < 1<<len(every); mask++ {
		var offered []string
		for i, tool := range every {
			if mask&(1<<i) != 0 {
				offered = append(offered, tool)
			}
		}

		text := systemtext.Compose(offered)
		for _, tool := range every {
			if got, want := strings.Contains(text, phrases[tool]), slices.Contains(offered, tool); got != want {
				t.Errorf("offering %v: the system text mentions %q = %v, want %v: a tool the call does not declare must not be described, and one it declares must be", offered, tool, got, want)
			}
		}
	}
}

func TestTheSystemTextOfACallThatOffersNothingNamesNoToolAndNoToolResults(t *testing.T) {
	t.Parallel()

	text := strings.ToLower(systemtext.Compose(nil))

	for _, forbidden := range []string{"tool", "recall", "search process", "query"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("the system text of a call that offers nothing contains %q: the answering call describes only answering, and a sentence about a tool the call withholds is what invites the model to emit one", forbidden)
		}
	}

	if !strings.Contains(text, "base it on the context block and on the request") {
		t.Errorf("the system text of a call that offers nothing does not tell the model what to base its answer on: %q", text)
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
