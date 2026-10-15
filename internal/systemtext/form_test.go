package systemtext_test

import (
	"os"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/systemtext"
)

func TestTheContentFormSystemTextShowsPartsAsContentAndNeverSpeaksOfSubstance(t *testing.T) {
	t.Parallel()

	offered := []string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile, loop.ToolNote}
	for _, conversation := range []bool{false, true} {
		for _, silence := range []bool{false, true} {
			text := systemtext.Compose(offered, conversation, silence, loop.FormContent)

			if strings.Contains(strings.ToLower(text), "substance") {
				t.Errorf("conversation %v silence %v: the content-form system text speaks of a substance:\n%s", conversation, silence, text)
			}
			for _, want := range []string{
				"and is shown as its content: the text stored in memory. A part with nothing below its header has no stored text; its id, type and name are all that is known about it.",
				"A read tool is available. It returns one part of memory as its content, given the id that part is printed with. Use it when a part you have been shown names an id whose own content you need. Do not use it for a part the context block already carries.",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("conversation %v silence %v: the content-form system text lacks %q", conversation, silence, want)
				}
			}
		}
	}
}

func TestTheContentFormChangesOnlyTheTwoSentencesAndTheReadParagraph(t *testing.T) {
	t.Parallel()

	offered := []string{loop.ToolRecall, loop.ToolReadNode}
	substance := strings.Split(systemtext.Compose(offered, false, false, loop.FormSubstance), "\n\n")
	content := strings.Split(systemtext.Compose(offered, false, false, loop.FormContent), "\n\n")

	if len(substance) != len(content) {
		t.Fatalf("the forms compose %d and %d paragraphs, want the same count", len(substance), len(content))
	}
	var differing int
	for i := range substance {
		if substance[i] != content[i] {
			differing++
		}
	}
	if differing != 2 {
		t.Errorf("%d paragraphs differ between the forms, want exactly the context paragraph and the read paragraph", differing)
	}
}

func TestTheSubstanceFormSystemTextIsByteIdenticalToTheTextBeforeTheFormArgument(t *testing.T) {
	t.Parallel()

	golden, err := os.ReadFile("testdata/without_conversation_recall+readNode.txt")
	if err != nil {
		t.Fatalf("reading the golden text: %v", err)
	}
	offered := []string{loop.ToolRecall, loop.ToolReadNode}
	for _, form := range []loop.Form{loop.FormSubstance, ""} {
		if got := systemtext.Compose(offered, false, false, form); got != strings.TrimRight(string(golden), "\n") && got != string(golden) {
			t.Errorf("form %q composes a text that differs from the golden one written before the form argument:\n%s", form, got)
		}
	}
}
