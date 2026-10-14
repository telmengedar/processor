package systemtext_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"regexp"
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
	readToolFunc       = "readNodeTool"
	descriptionKey     = "Description"
)

var adapterWireFiles = []string{"../ollama/wire.go", "../openaicompat/wire.go"}

func TestTheSystemTextDescribesEveryToolTheJudgementCallOffers(t *testing.T) {
	t.Parallel()

	described := map[string]string{
		loop.ToolRecall:    "recall tool",
		loop.ToolReadNode:  "read tool",
		loop.ToolWriteFile: "file tool",
		loop.ToolNote:      "note tool",
	}
	nonToolConstants := map[string]bool{
		"ToolSourceNative":  true,
		"ToolSourceContent": true,
		"ToolSourceTextual": true,
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
		if !strings.Contains(systemtext.Compose([]string{tool}, false, false), offered) {
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
		loop.ToolNote:      "A note tool is available.",
	}
	every := []string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile, loop.ToolNote}

	for mask := 0; mask < 1<<len(every); mask++ {
		var offered []string
		for i, tool := range every {
			if mask&(1<<i) != 0 {
				offered = append(offered, tool)
			}
		}

		text := systemtext.Compose(offered, false, false)
		for _, tool := range every {
			if got, want := strings.Contains(text, phrases[tool]), slices.Contains(offered, tool); got != want {
				t.Errorf("offering %v: the system text mentions %q = %v, want %v: a tool the call does not declare must not be described, and one it declares must be", offered, tool, got, want)
			}
		}
	}
}

func TestTheSystemTextOfACallThatOffersNothingNamesNoToolAndNoToolResults(t *testing.T) {
	t.Parallel()

	text := strings.ToLower(systemtext.Compose(nil, false, false))

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

var bodyWord = regexp.MustCompile("(?i)(^|[^a-z])bod(y|ies)([^a-z]|$)")

var bodyPromises = []string{"in full", "full text", "full content", "complete content"}

func readParagraphOfTheSystemText(t *testing.T) string {
	t.Helper()

	for _, paragraph := range strings.Split(systemtext.Compose([]string{loop.ToolReadNode}, false, false), "\n\n") {
		if strings.HasPrefix(paragraph, "A read tool is available.") {
			return paragraph
		}
	}
	t.Fatalf("the system text of a call offering the read tool carries no paragraph introducing it, so this guard has nothing to hold to the substance-only account")
	return ""
}

func TestNoParagraphOfTheSystemTextOtherThanTheFileWriteSpeaksOfABodyOrPromisesFullContent(t *testing.T) {
	t.Parallel()

	if paragraph := readParagraphOfTheSystemText(t); !strings.Contains(paragraph, "as its substance") {
		t.Errorf("the read paragraph never says a read returns a part %q: %q", "as its substance", paragraph)
	}

	for _, conversation := range []bool{false, true} {
		composed := systemtext.Compose([]string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}, conversation, false)

		var kept []string
		for _, paragraph := range strings.Split(composed, "\n\n") {
			if !strings.HasPrefix(paragraph, "A file tool is available.") {
				kept = append(kept, paragraph)
			}
		}
		if len(kept) < 3 {
			t.Fatalf("conversation %v: the composed system text has %d paragraphs outside the file tool's, which is too few for this guard to be reading the whole text", conversation, len(kept))
		}
		text := strings.Join(kept, "\n\n")

		if bodyWord.MatchString(text) {
			t.Errorf("conversation %v: the system text speaks of a body: the model works with substances and is never given a body.\n%s", conversation, text)
		}
		for _, promise := range bodyPromises {
			if strings.Contains(strings.ToLower(text), promise) {
				t.Errorf("conversation %v: the system text promises %q: a part is shown as its substance or marked unavailable.\n%s", conversation, promise, text)
			}
		}
	}
}

func TestEveryAdapterDescribesTheReadAsTheSystemTextDoesAndTheTwoAgree(t *testing.T) {
	t.Parallel()

	const substanceVocabulary = "as its substance"

	if !strings.Contains(readParagraphOfTheSystemText(t), substanceVocabulary) {
		t.Fatalf("test setup error: the system text's read paragraph no longer says %q, so the adapters are being held to a phrase the model is not given", substanceVocabulary)
	}

	var descriptions []string
	for _, path := range adapterWireFiles {
		description := readToolDescription(t, path)
		descriptions = append(descriptions, description)

		if !strings.Contains(description, substanceVocabulary) {
			t.Errorf("%s declares the read as %q, which does not say %q: the system text tells the model a read returns a part's substance and the tool schema must say the same", path, description, substanceVocabulary)
		}
		if bodyWord.MatchString(description) {
			t.Errorf("%s declares the read as %q, which speaks of a body: the model works with substances", path, description)
		}
		for _, promise := range bodyPromises {
			if strings.Contains(strings.ToLower(description), promise) {
				t.Errorf("%s declares the read as %q, which promises %q: a read serves a part's substance or marks it unavailable and never serves its body", path, description, promise)
			}
		}
	}

	if descriptions[0] != descriptions[1] {
		t.Errorf("the adapters declare the read differently: %q in %s, %q in %s; both reach the same model and must say the same thing", descriptions[0], adapterWireFiles[0], descriptions[1], adapterWireFiles[1])
	}
}

func readToolDescription(t *testing.T, path string) string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s failed: %v", path, err)
	}

	var found []string
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Name.Name != readToolFunc {
			continue
		}
		ast.Inspect(function, func(node ast.Node) bool {
			pair, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := pair.Key.(*ast.Ident)
			if !ok || key.Name != descriptionKey {
				return true
			}
			literal, ok := pair.Value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				t.Fatalf("%s declares the %s of %s as something other than a string literal, so this guard cannot read it", path, descriptionKey, readToolFunc)
			}
			unquoted, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatalf("the %s in %s is not a quoted string: %v", descriptionKey, path, err)
			}
			found = append(found, unquoted)
			return true
		})
	}
	if len(found) != 1 {
		t.Fatalf("%s declares %d descriptions inside %s, want exactly 1, so this guard would compare against nothing or the wrong text", path, len(found), readToolFunc)
	}
	return found[0]
}
