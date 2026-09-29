package systemtext

import (
	"strings"
	"testing"
)

func TestTheSystemTextStatesWhatACondensedMarkingMeansAndThatTheBodyIsFetchableById(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		`marked "form: substance"`,
		"condensed version",
		"the original says more",
		"carrying no such marking is whole",
	} {
		if !strings.Contains(Text, want) {
			t.Errorf("the system text does not carry %q: a part whose body the block replaced with a condensed form is indistinguishable from a whole one unless the instructions say what the marking means", want)
		}
	}

	for _, want := range []string{
		"A read tool is available.",
		"returns one part of memory in full",
		"given the id that part is printed with",
		"Do not use it for a part the context block already carries in full.",
	} {
		if !strings.Contains(Text, want) {
			t.Errorf("the system text does not carry %q: a tool the instructions never mention is one the model has no reason to reach for, and a tool it reaches for on a part it already holds in full spends a round on a refusal", want)
		}
	}
}
