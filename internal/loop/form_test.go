package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestAssembleRendersTheSubstanceWhenItIsMateriallySmallerThanTheContent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)
	candidates := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	block, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	if dispositions[0].Form != FormSubstance {
		t.Fatalf("Form = %q for a 300-byte substance against 1000 bytes of content, want %q", dispositions[0].Form, FormSubstance)
	}
	if dispositions[0].RenderedSize != 300 {
		t.Fatalf("RenderedSize = %d, want 300 - the bytes the block carries, not the content's 1000", dispositions[0].RenderedSize)
	}
	if !strings.Contains(block, substance) {
		t.Fatal("the block does not carry the substance although the form rule selected it")
	}
	if strings.Contains(block, content) {
		t.Fatal("the block carries the full content although the form rule selected the substance")
	}
}

func TestAssembleRendersTheContentWhenTheSubstanceIsNotMateriallySmaller(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 900)
	candidates := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	block, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	if dispositions[0].Form != FormContent {
		t.Fatalf("Form = %q for a 900-byte substance against 1000 bytes of content, want %q - a tenth off is not materially smaller", dispositions[0].Form, FormContent)
	}
	if dispositions[0].RenderedSize != 1000 {
		t.Fatalf("RenderedSize = %d, want 1000", dispositions[0].RenderedSize)
	}
	if !strings.Contains(block, content) {
		t.Fatal("the block does not carry the content although the form rule selected it")
	}
	if strings.Contains(block, substance) {
		t.Fatal("the block carries the substance although the form rule selected the content")
	}
}

func TestAssembleRendersTheContentWhenTheRatioSitsExactlyOnTheThreshold(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 592)}}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	if dispositions[0].Form != FormContent {
		t.Fatalf("Form = %q at a ratio of exactly the shipped threshold, want %q - materially smaller is strictly below the threshold, not at it", dispositions[0].Form, FormContent)
	}
}

func TestAssembleRendersTheContentWhenNoSubstanceWasGenerated(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 1000)}}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	if dispositions[0].Form != FormContent {
		t.Fatalf("Form = %q for a candidate carrying no substance, want %q", dispositions[0].Form, FormContent)
	}
	if dispositions[0].RenderedSize != 1000 {
		t.Fatalf("RenderedSize = %d, want 1000", dispositions[0].RenderedSize)
	}
}

func TestAssembleRendersTheContentWhenTheContentIsEmptyAndASubstanceIsPresent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: "", Substance: strings.Repeat("z", 300)}}

	block, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	if dispositions[0].Form != FormContent {
		t.Fatalf("Form = %q for a candidate with empty content, want %q - nothing is materially smaller than nothing", dispositions[0].Form, FormContent)
	}
	if dispositions[0].RenderedSize != 0 {
		t.Fatalf("RenderedSize = %d, want 0", dispositions[0].RenderedSize)
	}
	if strings.Contains(block, "zzz") {
		t.Fatal("the block fell back to the substance because the content was empty; an empty body is the node's content and is what renders")
	}
}

func TestAssembleMarksASubstanceRenderedCandidateAndLeavesAContentOneUnmarked(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a", Content: "anchor body"}
	candidates := []Candidate{
		{ID: 10, Type: "documentation", Name: "Bravo", Content: strings.Repeat("x", 100), Substance: "condensed!"},
		{ID: 20, Type: "task", Name: "Charlie", Content: "charlie body"},
	}

	block, _ := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	const wantBlock = "===== ANCHOR =====\n" +
		"id: 1\n" +
		"type: t\n" +
		"name: a\n" +
		"\n" +
		"anchor body\n" +
		"\n" +
		"===== CANDIDATE =====\n" +
		"id: 10\n" +
		"type: documentation\n" +
		"name: Bravo\n" +
		"form: substance\n" +
		"\n" +
		"condensed!\n" +
		"\n" +
		"===== CANDIDATE =====\n" +
		"id: 20\n" +
		"type: task\n" +
		"name: Charlie\n" +
		"\n" +
		"charlie body\n" +
		"\n" +
		"Seems like your knowledge is still thin on the topic - your focus might be too narrow; try approaching the question from a different angle.\n"

	if block != wantBlock {
		t.Fatalf("block =\n%q\nwant\n%q", block, wantBlock)
	}
}

func TestAssembleChargesTheRenderedSubstanceSoARowTooLargeInContentFormStillFits(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 300)}}
	const budget = 400

	if len(candidates[0].Content) <= budget {
		t.Fatalf("test setup error: content is %d bytes against budget %d; it must exceed it or the test cannot tell which form was charged", len(candidates[0].Content), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget, 0, SubstanceRatioThreshold)

	if !dispositions[0].Included {
		t.Fatal("a candidate whose 300-byte substance fits a 400-byte budget was cut; admission charges the form that renders, not the content")
	}
	if dispositions[0].RenderedSize != 300 {
		t.Fatalf("RenderedSize = %d, want 300", dispositions[0].RenderedSize)
	}
}

func TestAssembleFormRuleReadsNeitherTheCandidatesTypeNorItsProvenance(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)
	candidates := []Candidate{
		{ID: 10, Type: "documentation", Content: content, Substance: substance},
		{ID: 20, Type: "session-log", Content: content, Substance: substance},
		{ID: 30, Type: "session-log", Content: content, Substance: substance, SelfProduced: true},
		{ID: 40, Type: "task", Content: content, Substance: substance},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	for i, d := range dispositions {
		if d.Form != FormSubstance {
			t.Fatalf("dispositions[%d] (id %d, type %q, selfProduced %v) has Form %q, want %q - the rule is over size, ratio and presence alone", i, d.ID, d.Type, candidates[i].SelfProduced, d.Form, FormSubstance)
		}
		if d.RenderedSize != 300 {
			t.Fatalf("dispositions[%d] (id %d) has RenderedSize %d, want 300", i, d.ID, d.RenderedSize)
		}
	}
}

func TestAssembleThresholdZeroRendersContentForEveryCandidate(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 300)},
		{ID: 20, Content: strings.Repeat("x", 1000), Substance: "z"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, 0)

	for i, d := range dispositions {
		if d.Form != FormContent {
			t.Fatalf("dispositions[%d] (id %d) has Form %q at a threshold of 0, want %q - the dial's zero position renders content for every row", i, d.ID, d.Form, FormContent)
		}
		if d.RenderedSize != 1000 {
			t.Fatalf("dispositions[%d] (id %d) has RenderedSize %d at a threshold of 0, want 1000", i, d.ID, d.RenderedSize)
		}
	}
}

func TestAssembleThresholdRaisedRendersSubstanceForARowTheShippedDialLeavesAsContent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 900)}}

	_, atShipped := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)
	_, atRaised := Assemble(anchor, candidates, 60_000, 0, 0.95)

	if atShipped[0].Form != FormContent {
		t.Fatalf("Form = %q at the shipped dial, want %q", atShipped[0].Form, FormContent)
	}
	if atRaised[0].Form != FormSubstance {
		t.Fatalf("Form = %q at a threshold of 0.95, want %q - a ratio of 0.9 falls below it", atRaised[0].Form, FormSubstance)
	}
}

func TestAssembleKeepsSizeAndContentHashOnTheContentWhenItRendersTheSubstance(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	content := strings.Repeat("x", 1000)
	candidates := []Candidate{{ID: 10, Content: content, Substance: strings.Repeat("z", 300)}}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	if dispositions[0].Form != FormSubstance {
		t.Fatalf("test setup error: Form = %q, want %q or this test cannot observe what the hash was taken over", dispositions[0].Form, FormSubstance)
	}
	if dispositions[0].Size != 1000 {
		t.Fatalf("Size = %d, want 1000 - size stays the content's byte count whichever form renders", dispositions[0].Size)
	}

	sum := sha256.Sum256([]byte(content))
	want := hex.EncodeToString(sum[:])
	if dispositions[0].ContentHash != want {
		t.Fatalf("ContentHash = %q, want %q - the hash of the content, never of the rendered substance, or every substance-rendered node reads as stale", dispositions[0].ContentHash, want)
	}
}

func TestDispositionFormAndRenderedSizeSerializeForEveryCandidate(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 300)},
		{ID: 20, Content: "a body"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, SubstanceRatioThreshold)

	for i, want := range []string{`"form":"substance","renderedSize":300`, `"form":"content","renderedSize":6`} {
		encoded, err := json.Marshal(dispositions[i])
		if err != nil {
			t.Fatalf("marshal disposition %d: %v", i, err)
		}
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("disposition JSON = %s, want it to contain %q", encoded, want)
		}
	}
}
