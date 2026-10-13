package loop

import (
	"strings"
	"testing"
)

func memoryHolding(t *testing.T, anchor Anchor, candidates []Candidate) *workingMemory {
	t.Helper()

	admitted, dispositions := admit(candidates, composedTestBudget, len(anchor.Substance))
	return newWorkingMemory(anchor, len(candidates) > 0, admitted, dispositions)
}

func TestTheMemoryAndTheAssembledBlockRenderASubstanceAlike(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1_000)
	substance := strings.Repeat("z", 300)
	candidates := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	block, dispositions := Assemble(Anchor{ID: 1}, candidates, composedTestBudget)
	if dispositions[0].Form != FormSubstance {
		t.Fatalf("test setup error: Form = %q, want %q", dispositions[0].Form, FormSubstance)
	}

	memory := memoryHolding(t, Anchor{ID: 1}, candidates)

	for _, surface := range []struct {
		name     string
		rendered string
	}{{name: "assembled block", rendered: block}, {name: "memory", rendered: memory.render(true)}} {
		if !strings.Contains(surface.rendered, substance) || strings.Contains(surface.rendered, content) {
			t.Errorf("the %s does not carry the substance alone:\n%s", surface.name, surface.rendered)
		}
	}
}

func TestTheMemoryCarriesExactlyTheBytesItsAdmissionCharged(t *testing.T) {
	t.Parallel()

	substance := strings.Repeat("z", 3_000)
	candidates := []Candidate{
		{ID: 91, Type: "documentation", Name: "Bravo", Substance: substance},
		{ID: 92, Type: "documentation", Name: "Charlie", Substance: strings.Repeat("y", SupplementaryByteBudget)},
	}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget, 0)
	if len(admitted) != 1 || admitted[0].ID != 91 {
		t.Fatalf("test setup error: admitted %d rows %v, want the first alone", len(admitted), dispositionIDs(dispositions))
	}

	memory := newWorkingMemory(Anchor{ID: 1}, true, admitted, dispositions)
	rendered := memory.render(false)

	if strings.Count(rendered, substance) != 1 {
		t.Fatalf("the memory does not carry the substance its admission charged for exactly once:\n%s", rendered)
	}
	if memory.spent != 3_000 {
		t.Fatalf("memory.spent = %d, want 3000: the memory is charged what admission charged", memory.spent)
	}
}

func TestTheMemoryChargesTheAnchorItsRenderedBytes(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Content: strings.Repeat("x", 9_000), Substance: strings.Repeat("z", 400)}

	memory := newWorkingMemory(anchor, false, nil, nil)

	if memory.spent != 400 {
		t.Fatalf("memory.spent = %d, want 400: the anchor is charged the substance it renders", memory.spent)
	}
}

func TestTheMemoryRendersAnUnavailableRowAsItsHeaderAndTheUnavailableLineAndChargesItNothing(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: strings.Repeat("x", 1_000)}}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget, 0)
	if dispositions[0].Form != FormUnavailable {
		t.Fatalf("test setup error: Form = %q, want %q", dispositions[0].Form, FormUnavailable)
	}

	memory := newWorkingMemory(Anchor{ID: 1}, true, admitted, dispositions)

	want := "===== CANDIDATE =====\nid: 91\ntype: documentation\nname: Bravo\nsimilarity: 0.000\nsubstance: unavailable\n"
	if got := memory.render(false); !strings.HasSuffix(got, want) {
		t.Fatalf("the memory renders an unavailable row differently:\n%q\nwant it to end with:\n%q", got, want)
	}
	if memory.spent != 0 {
		t.Fatalf("memory.spent = %d, want 0: an unavailable row has no body to charge", memory.spent)
	}
}

func TestASubstanceReplacesTheUnavailableFormOfTheSameRow(t *testing.T) {
	t.Parallel()

	substance := strings.Repeat("z", 300)

	memory := memoryHolding(t, Anchor{ID: 1, Substance: "anchor"}, []Candidate{{ID: 91, Type: "documentation", Name: "Bravo"}})
	if !strings.Contains(memory.render(false), "substance: unavailable") {
		t.Fatalf("test setup error: the memory does not hold the unavailable form to begin with:\n%s", memory.render(false))
	}

	created := []Candidate{candidateFromAnchor(Anchor{ID: 91, Type: "documentation", Name: "Bravo", Substance: substance})}
	admitted, dispositions := admit(created, SupplementaryByteBudget, 0)
	memory.absorb(admitted, dispositions)

	rendered := memory.render(false)
	if strings.Contains(rendered, "substance: unavailable") || strings.Count(rendered, substance) != 1 {
		t.Fatalf("after the substance exists the memory still carries the unavailable form, or carries the row twice:\n%s", rendered)
	}
	if memory.spent != 306 {
		t.Fatalf("memory.spent = %d, want 306", memory.spent)
	}
}

func TestAnUnavailableFormNeverReplacesTheSubstanceOfTheSameRow(t *testing.T) {
	t.Parallel()

	substance := strings.Repeat("z", 300)

	memory := memoryHolding(t, Anchor{ID: 1, Substance: "anchor"}, []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Substance: substance}})

	unavailable := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo"}}
	admitted, dispositions := admit(unavailable, SupplementaryByteBudget, 0)
	memory.absorb(admitted, dispositions)

	if rendered := memory.render(false); !strings.Contains(rendered, substance) || strings.Contains(rendered, "substance: unavailable") {
		t.Fatalf("a recall that returned an unavailable form of a row already held with its substance replaced it:\n%s", rendered)
	}
}
