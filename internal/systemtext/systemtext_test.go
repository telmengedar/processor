package systemtext

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestTheSystemTextStatesWhatACondensedMarkingMeansAndThatTheBodyIsFetchableById(t *testing.T) {
	t.Parallel()

	text := Compose([]string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile})

	for _, want := range []string{
		`marked "form: substance"`,
		"condensed version",
		"the original says more",
		"carrying no such marking is whole",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the system text does not carry %q: a part whose body the block replaced with a condensed form is indistinguishable from a whole one unless the instructions say what the marking means", want)
		}
	}

	for _, want := range []string{
		"A read tool is available.",
		"returns one part of memory in full",
		"given the id that part is printed with",
		"Do not use it for a part the context block already carries in full.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the system text does not carry %q: a tool the instructions never mention is one the model has no reason to reach for, and a tool it reaches for on a part it already holds in full spends a round on a refusal", want)
		}
	}
}

func TestTheReadParagraphDescribesOutlinesAndSegmentsOnlyWhenReadIsOffered(t *testing.T) {
	t.Parallel()

	const outlineSentence = "A part too large to be returned in full is answered with its outline"
	const segmentSentence = "give the same id together with the segment's number"

	offered := Compose([]string{loop.ToolRecall, loop.ToolReadNode})
	for _, want := range []string{outlineSentence, segmentSentence} {
		if !strings.Contains(offered, want) {
			t.Errorf("a call that offers the read tool is not told %q: a model that is never told an oversized part answers with an outline cannot read past it", want)
		}
	}

	for _, withheld := range [][]string{nil, {loop.ToolRecall}, {loop.ToolWriteFile}} {
		text := Compose(withheld)
		for _, forbidden := range []string{outlineSentence, segmentSentence} {
			if strings.Contains(text, forbidden) {
				t.Errorf("offering %v: the system text still carries %q, a sentence about a tool the call does not offer", withheld, forbidden)
			}
		}
	}
}

func TestTheBaseTextNamesWhatAnOutlineAndASegmentMarkingMeanWhateverIsOffered(t *testing.T) {
	t.Parallel()

	for _, offered := range [][]string{nil, {loop.ToolReadNode}} {
		text := Compose(offered)
		for _, want := range []string{`"form: outline"`, `"form: segment"`, "only the list of that part's numbered segments", "only that one numbered segment"} {
			if !strings.Contains(text, want) {
				t.Errorf("offering %v: the system text lacks %q: the answering call still reads a memory that holds an outline or a segment, and an unmarked partial body reads as a whole one", offered, want)
			}
		}
	}
}
