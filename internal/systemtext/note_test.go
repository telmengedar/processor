package systemtext_test

import (
	"os"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/systemtext"
)

func TestTheNoteParagraphFollowsTheFileParagraphByteForByte(t *testing.T) {
	t.Parallel()

	golden, err := os.ReadFile("testdata/with_note_recall+readNode+writeFile+note.txt")
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	if strings.Contains(string(golden), "\r") {
		t.Fatal("the fixture holds a carriage return, so it was not stored with LF line endings")
	}

	got := systemtext.Compose([]string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile, loop.ToolNote}, false, false)
	if got != string(golden) {
		t.Errorf("the system text of a call offering every tool is\n%q\nwant\n%q", got, golden)
	}
}

func TestTheNoteParagraphNamesNoVoiceAndGivesNoReasonToWrite(t *testing.T) {
	t.Parallel()

	text := strings.ToLower(systemtext.Compose([]string{loop.ToolNote}, false, false))

	for _, persona := range []string{"first person", "your voice", "in character", "you should", "always", "whenever", "remember", "feel", "thought", "important"} {
		if strings.Contains(text, persona) {
			t.Errorf("the system text of a call offering the note tool contains %q: how a note sounds and when to write one belong to the caller's identity, never to the product text", persona)
		}
	}
}
