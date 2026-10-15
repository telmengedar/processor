package systemtext_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/systemtext"
)

func paragraphsOf(text string) []string {
	return strings.Split(text, "\n\n")
}

func TestComposeWithoutSilenceNamesNeitherTheMarkerNorSilence(t *testing.T) {
	t.Parallel()

	for _, offered := range [][]string{nil, {loop.ToolRecall}, {loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}} {
		for _, conversation := range []bool{false, true} {
			text := systemtext.Compose(offered, conversation, false, loop.FormSubstance)

			if strings.Contains(strings.ToUpper(text), loop.SilenceMarker) || strings.Contains(strings.ToLower(text), "silent") {
				t.Errorf("offered %v, conversation %v: the system text names silence although it was not offered:\n%s", offered, conversation, text)
			}
		}
	}
}

func TestComposeWithSilenceAddsExactlyOneParagraphLastButOneAndChangesNothingElse(t *testing.T) {
	t.Parallel()

	for _, offered := range [][]string{nil, {loop.ToolRecall, loop.ToolReadNode}, {loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}} {
		for _, conversation := range []bool{false, true} {
			without := paragraphsOf(systemtext.Compose(offered, conversation, false, loop.FormSubstance))
			with := paragraphsOf(systemtext.Compose(offered, conversation, true, loop.FormSubstance))

			if len(with) != len(without)+1 {
				t.Fatalf("offered %v, conversation %v: %d paragraphs with silence and %d without, want exactly one more", offered, conversation, len(with), len(without))
			}

			at := len(with) - 2
			if !strings.Contains(with[at], loop.SilenceMarker) {
				t.Errorf("offered %v, conversation %v: the paragraph before the last is %q, want the silence paragraph naming the marker", offered, conversation, with[at])
			}
			if rest := slices.Delete(slices.Clone(with), at, at+1); !slices.Equal(rest, without) {
				t.Errorf("offered %v, conversation %v: the other paragraphs differ from the system text without silence", offered, conversation)
			}
			if got := strings.Count(strings.Join(with, "\n\n"), loop.SilenceMarker); got != 1 {
				t.Errorf("offered %v, conversation %v: the marker appears %d times, want once", offered, conversation, got)
			}
		}
	}
}

func TestTheSilenceParagraphStatesTheMechanismAndNoPolicyOnWhenToBeSilent(t *testing.T) {
	t.Parallel()

	with := paragraphsOf(systemtext.Compose(nil, false, true, loop.FormSubstance))
	got := with[len(with)-2]

	want := "You may choose not to reply. To stay silent, write " + loop.SilenceMarker + " as your whole answer, with nothing before or after it. Staying silent this way is a valid answer, not a failure, and nothing is shown for it."
	if got != want {
		t.Errorf("the silence paragraph is %q, want %q", got, want)
	}

	for _, policy := range []string{"address", "name", "must", "otherwise", "chat", "someone else", "onlooker"} {
		if strings.Contains(strings.ToLower(got), policy) {
			t.Errorf("the silence paragraph contains %q, which is a statement of when to be silent and not of how", policy)
		}
	}
}
