package loop

import (
	"strings"
	"testing"
)

func memoryHolding(t *testing.T, anchor Anchor, threshold SubstanceRatio, candidates []Candidate) *workingMemory {
	t.Helper()

	admitted, dispositions := admit(candidates, composedTestBudget, len(anchor.Content), 0, BlockOccupancy, threshold)
	return newWorkingMemory(anchor, len(candidates) > 0, threshold, admitted, dispositions)
}

func TestTheMemoryAndTheAssembledBlockMarkASubstanceRenderedRowAlike(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1_000)
	substance := strings.Repeat("z", 300)
	candidates := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	block, dispositions := Assemble(Anchor{ID: 1}, candidates, composedTestBudget, 0, formRuleTestThreshold)
	if dispositions[0].Form != FormSubstance {
		t.Fatalf("test setup error: Form = %q, want %q", dispositions[0].Form, FormSubstance)
	}

	memory := memoryHolding(t, Anchor{ID: 1}, formRuleTestThreshold, candidates)

	marker := "form: substance"
	for _, surface := range []struct {
		name     string
		rendered string
	}{{name: "assembled block", rendered: block}, {name: "memory", rendered: memory.render(true)}} {
		if !strings.Contains(surface.rendered, marker) {
			t.Errorf("the %s renders a condensed payload without %q; a surface that presents a condensation unmarked tells the model it is holding the node:\n%s", surface.name, marker, surface.rendered)
		}
		if !strings.Contains(surface.rendered, substance) || strings.Contains(surface.rendered, content) {
			t.Errorf("the %s does not carry the form its admission charged for:\n%s", surface.name, surface.rendered)
		}
	}
}

func TestTheMemoryCarriesExactlyTheBytesItsAdmissionChargedUnderBothMechanisms(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 20_000)
	substance := strings.Repeat("z", 3_000)
	candidates := []Candidate{
		{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance},
		{ID: 92, Type: "documentation", Name: "Charlie", Content: content},
	}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget, 0, 0, testBlockOccupancy, formRuleTestThreshold)
	if len(admitted) != 1 || admitted[0].ID != 91 {
		t.Fatalf("test setup error: admitted %d rows %v, want the condensed one alone", len(admitted), dispositionIDs(dispositions))
	}

	memory := newWorkingMemory(Anchor{ID: 1}, true, formRuleTestThreshold, admitted, dispositions)
	rendered := memory.render(false)

	if strings.Count(rendered, substance) != 1 {
		t.Fatalf("the memory does not carry the substance its admission charged for exactly once:\n%s", rendered)
	}
	if strings.Contains(rendered, content) {
		t.Fatalf("the memory carries content bytes nobody charged against the ceiling:\n%s", rendered)
	}
	if memory.spent != 3_000 {
		t.Fatalf("memory.spent = %d, want 3000: the memory is charged what admission charged", memory.spent)
	}
}

func TestTheMemoryRendersBothSidesOfTheFormBandAsTheirAdmissionChargedThem(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1000)
	banded := strings.Repeat("z", 300)
	overCompressed := strings.Repeat("y", 50)
	candidates := []Candidate{
		{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: banded},
		{ID: 92, Type: "documentation", Name: "Charlie", Content: content, Substance: overCompressed},
	}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget, 0, 0, BlockOccupancy, formRuleTestThreshold)
	if dispositions[0].Form != FormSubstance || dispositions[1].Form != FormContent {
		t.Fatalf("test setup error: admission charged %q then %q, want %q then %q", dispositions[0].Form, dispositions[1].Form, FormSubstance, FormContent)
	}

	got := newWorkingMemory(Anchor{ID: 1}, true, formRuleTestThreshold, admitted, dispositions).render(false)

	want := "===== CANDIDATE =====\nid: 91\ntype: documentation\nname: Bravo\nform: substance\n\n" + banded + "\n" +
		"\n===== CANDIDATE =====\nid: 92\ntype: documentation\nname: Charlie\n\n" + content + "\n"
	if !strings.HasSuffix(got, want) {
		t.Fatalf("the memory applies a different band from the one its admission charged at:\n%q\nwant it to end with:\n%q", got, want)
	}
}

func TestAFullReadReplacesTheCondensedFormOfTheSameRow(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1_000)
	substance := strings.Repeat("z", 300)
	condensed := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	memory := memoryHolding(t, Anchor{ID: 1}, formRuleTestThreshold, condensed)
	if !strings.Contains(memory.render(false), substance) {
		t.Fatalf("test setup error: the memory does not hold the condensed form to begin with:\n%s", memory.render(false))
	}

	read := []Candidate{candidateFromAnchor(Anchor{ID: 91, Type: "documentation", Name: "Bravo", Content: content})}
	admitted, dispositions := admit(read, SupplementaryByteBudget, 0, 0, BlockOccupancy, formRuleTestThreshold)
	memory.absorb(admitted, dispositions)

	rendered := memory.render(false)
	if strings.Contains(rendered, substance) || strings.Count(rendered, content) != 1 {
		t.Fatalf("after a full read the memory still carries the condensed form, or carries the row twice:\n%s", rendered)
	}
	if strings.Count(rendered, "id: 91\n") != 1 {
		t.Fatalf("the memory carries row 91 %d times, want once:\n%s", strings.Count(rendered, "id: 91\n"), rendered)
	}
	if memory.spent != 1_000 {
		t.Fatalf("memory.spent = %d, want 1000: the replaced form must stop being charged", memory.spent)
	}
}

func TestACondensedFormNeverReplacesTheFullFormOfTheSameRow(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1_000)
	substance := strings.Repeat("z", 300)

	memory := memoryHolding(t, Anchor{ID: 1}, formRuleTestThreshold, []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content}})

	condensed := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}
	admitted, dispositions := admit(condensed, SupplementaryByteBudget, 0, 0, BlockOccupancy, formRuleTestThreshold)
	if dispositions[0].Form != FormSubstance {
		t.Fatalf("test setup error: Form = %q, want %q", dispositions[0].Form, FormSubstance)
	}
	memory.absorb(admitted, dispositions)

	if rendered := memory.render(false); strings.Contains(rendered, substance) || !strings.Contains(rendered, content) {
		t.Fatalf("a recall that returned the condensed form of a row already held whole replaced it:\n%s", rendered)
	}
}
