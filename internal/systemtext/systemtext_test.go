package systemtext

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestTheSystemTextStatesThatEveryPartIsShownAsItsSubstanceAndWhatAnUnavailableMarkingMeans(t *testing.T) {
	t.Parallel()

	text := Compose([]string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}, false, false, loop.FormSubstance)

	for _, want := range []string{
		"and is shown as its substance: a condensed form written for another model as its only source",
		`marked "substance: unavailable"`,
		"has no substance yet",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the system text does not carry %q: a part rendered as its header alone is indistinguishable from an empty one unless the instructions say what the marking means", want)
		}
	}
	for _, unwanted := range []string{"form: substance", "carrying no such marking is whole", "the original says more"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("the system text still carries %q, which describes a form no part renders in any more", unwanted)
		}
	}

	for _, want := range []string{
		"A read tool is available.",
		"returns one part of memory as its substance",
		"given the id that part is printed with",
		`to retry a part marked "substance: unavailable"`,
		"Do not use it for a part the context block already carries.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the system text does not carry %q: a tool the instructions never mention is one the model has no reason to reach for, and a tool it reaches for on a part it already holds spends a round on a refusal", want)
		}
	}
}

func TestTheBaseTextTellsTheModelToKeepTheBlocksOwnFramingOutOfItsAnswerRightAfterTheUsePartsBullet(t *testing.T) {
	t.Parallel()

	const (
		usePartsBullet = "- Use the parts that help you. Ignore the parts that do not."
		keepOutBullet  = "- Its ids, headers, labels and notes are for you alone; never copy them into your answer."
	)

	for _, offered := range [][]string{nil, {loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}} {
		text := Compose(offered, false, false, loop.FormSubstance)

		if strings.Count(text, keepOutBullet) != 1 {
			t.Errorf("offering %v, the system text does not carry the bullet %q exactly once", offered, keepOutBullet)
		}
		if !strings.Contains(text, usePartsBullet+"\n"+keepOutBullet+"\n") {
			t.Errorf("offering %v, the bullet does not follow the bullet %q on the very next line", offered, usePartsBullet)
		}
	}
}
