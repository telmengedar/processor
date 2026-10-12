package loop

import (
	"strings"
	"testing"
)

const composedTestBudget = 60_000

func TestAdmitRefusesTheCeilingAgainstTheRenderedFormSoAContentSizedRefusalBecomesASubstanceSizedFit(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 20_000)
	substance := strings.Repeat("z", 5_000)
	candidates := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	ceiling := payloadCap(composedTestBudget, testBlockOccupancy)
	if len(content) <= ceiling || len(substance) > ceiling {
		t.Fatalf("test setup error: content %d and substance %d against a ceiling of %d; the content must bust it and the substance must fit, or the two composition orders admit the same row and this test separates nothing", len(content), len(substance), ceiling)
	}

	_, dispositions := admit(candidates, composedTestBudget, 0, 0, testBlockOccupancy, formRuleTestThreshold)

	if !dispositions[0].Included {
		t.Fatalf("the row was cut as %q; the form rule runs first and the ceiling is evaluated against the payload the block is about to render, so a row over the ceiling in content form and under it in substance form is admitted", dispositions[0].CutReason)
	}
	if dispositions[0].Form != FormSubstance {
		t.Fatalf("Form = %q, want %q", dispositions[0].Form, FormSubstance)
	}
	if dispositions[0].RenderedSize != len(substance) {
		t.Fatalf("RenderedSize = %d, want %d - the bytes the ceiling compared and the bytes the budget was charged are one quantity", dispositions[0].RenderedSize, len(substance))
	}
}

func TestAdmitNeverCutsARowAsOversizedWhileTheSizeItRecordsFitsTheCeilingItRecords(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 20_000)
	candidates := []Candidate{
		{ID: 10, Content: content, Substance: strings.Repeat("z", 5_000)},
		{ID: 20, Content: content, Substance: strings.Repeat("z", 19_000)},
		{ID: 30, Content: content},
		{ID: 40, Content: strings.Repeat("y", 500)},
	}

	_, dispositions := admit(candidates, composedTestBudget, 0, 0, testBlockOccupancy, formRuleTestThreshold)

	oversized := 0
	for _, d := range dispositions {
		if d.CutReason != cutReasonOversized {
			continue
		}
		oversized++
		if d.RenderedSize <= d.PayloadCap {
			t.Errorf("#%d was cut as %q at RenderedSize %d against a PayloadCap of %d; a record that refuses a row for a size it does not carry cannot be read back, and the ceiling was evaluated against something other than the rendered payload", d.ID, d.CutReason, d.RenderedSize, d.PayloadCap)
		}
	}
	if oversized == 0 {
		t.Fatal("test setup error: no row was cut as oversized, so this guard passed vacuously")
	}
}

func TestAdmitChargesTheBudgetAndTheCeilingTheSameRenderedPayload(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 20_000)
	substance := strings.Repeat("z", 5_000)
	candidates := []Candidate{{ID: 10, Content: content, Substance: substance}}
	const budget = 30_000

	ceiling := payloadCap(budget, testBlockOccupancy)
	if len(content) <= ceiling {
		t.Fatalf("test setup error: content %d does not bust the ceiling of %d", len(content), ceiling)
	}
	if len(substance) > ceiling {
		t.Fatalf("test setup error: substance %d busts the ceiling of %d", len(substance), ceiling)
	}

	admitted, dispositions := admit(candidates, budget, budget-len(substance), 0, testBlockOccupancy, formRuleTestThreshold)

	if len(admitted) != 1 {
		t.Fatalf("the row was cut as %q with exactly its rendered form's worth of budget left; the budget is charged the same payload the ceiling refused against", dispositions[0].CutReason)
	}
	if dispositions[0].RenderedSize != len(substance) {
		t.Fatalf("RenderedSize = %d, want %d", dispositions[0].RenderedSize, len(substance))
	}
}

func TestAdmitCutsNoRowAsOversizedAtTheShippedBlockOccupancyWhateverTheDial(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 50_000), Substance: strings.Repeat("z", 100)},
		{ID: 20, Content: strings.Repeat("x", 50_000)},
		{ID: 30, Content: strings.Repeat("y", 10)},
	}

	for _, threshold := range []SubstanceRatio{SubstanceRatioThreshold, formRuleTestThreshold, 1} {
		_, dispositions := admit(candidates, composedTestBudget, 0, 0, BlockOccupancy, threshold)

		for _, d := range dispositions {
			if d.CutReason == cutReasonOversized {
				t.Errorf("#%d was cut as %q at a block occupancy of %d, which leaves no ceiling in force; no row may be refused for its size while the cap ships off, whatever the form rule selected", d.ID, d.CutReason, BlockOccupancy)
			}
			if d.PayloadCap != noPayloadCeiling {
				t.Errorf("#%d records a PayloadCap of %d at a block occupancy of %d, want %d", d.ID, d.PayloadCap, BlockOccupancy, noPayloadCeiling)
			}
		}
	}
}

func TestAssembleRendersAByteIdenticalBlockAtBothShippedDialsWhateverASubstanceSaysAboutTheRow(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a", Content: "anchor body"}
	content := strings.Repeat("bravo body ", 3_000)

	if len(content) <= composedTestBudget/5 {
		t.Fatalf("test setup error: the row is %d bytes and would fit a ceiling of one fifth of the budget; it must bust the ceiling the cap would impose or the arms agree for a reason this test is not about", len(content))
	}

	bare := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content}}
	condensed := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content, Substance: "z"}}

	blockBare, dispositionsBare := Assemble(anchor, bare, composedTestBudget, 0, SubstanceRatioThreshold)
	blockCondensed, dispositionsCondensed := Assemble(anchor, condensed, composedTestBudget, 0, SubstanceRatioThreshold)

	if blockBare != blockCondensed {
		t.Fatalf("at both shipped dials the block changed when the candidate carried a substance:\nbare=%q\ncondensed=%q", blockBare, blockCondensed)
	}
	if !dispositionsBare[0].Included || !dispositionsCondensed[0].Included {
		t.Fatalf("a row of %d bytes was cut at both shipped dials (bare %q, condensed %q); neither the form rule nor the ceiling may move a row while both dials sit at their off positions", len(content), dispositionsBare[0].CutReason, dispositionsCondensed[0].CutReason)
	}
	if strings.Contains(blockCondensed, formHeaderKey+": ") {
		t.Fatalf("the block carries a form marker at the shipped dial, where no candidate renders as substance:\n%q", blockCondensed)
	}
	if dispositionsCondensed[0].RenderedSize != len(content) {
		t.Fatalf("RenderedSize = %d, want %d", dispositionsCondensed[0].RenderedSize, len(content))
	}
}
