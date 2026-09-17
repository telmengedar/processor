package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const formRuleTestThreshold SubstanceRatio = 0.592

func TestAssembleRendersTheSubstanceWhenItIsMateriallySmallerThanTheContent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)
	candidates := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	block, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

	block, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

	if dispositions[0].Form != FormContent {
		t.Fatalf("Form = %q at a ratio of exactly the threshold, want %q - materially smaller is strictly below the threshold, not at it", dispositions[0].Form, FormContent)
	}
}

func TestAssembleRendersTheContentWhenNoSubstanceWasGenerated(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 1000)}}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

	block, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

	block, _ := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

	_, dispositions := Assemble(anchor, candidates, budget, 0, formRuleTestThreshold)

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

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

func TestAssembleThresholdRaisedRendersSubstanceForARowALowerThresholdLeavesAsContent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 900)}}

	_, atLower := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)
	_, atRaised := Assemble(anchor, candidates, 60_000, 0, 0.95)

	if atLower[0].Form != FormContent {
		t.Fatalf("Form = %q at a threshold of %v, want %q", atLower[0].Form, formRuleTestThreshold, FormContent)
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

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

	_, dispositions := Assemble(anchor, candidates, 60_000, 0, formRuleTestThreshold)

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

func TestTheShippedSubstanceRatioThresholdIsTheOffPosition(t *testing.T) {
	t.Parallel()

	if SubstanceRatioThreshold != 0 {
		t.Fatalf("SubstanceRatioThreshold = %v, want 0 - the rule ships with its dial off, and the block it renders stays byte-identical to the one rendered before the rule existed", SubstanceRatioThreshold)
	}
}

func TestAssembleRendersAByteIdenticalBlockAtThresholdZeroWhetherOrNotACandidateCarriesASubstance(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a", Content: "anchor body"}
	const budget = 60_000

	for _, content := range []string{strings.Repeat("bravo body ", 100), ""} {
		t.Run(fmt.Sprintf("contentBytes=%d", len(content)), func(t *testing.T) {
			t.Parallel()

			if len(anchor.Content)+len(content) > budget {
				t.Fatalf("test setup error: anchor and content are %d bytes against budget %d; the row must fit or it renders in neither arm and the comparison separates nothing", len(anchor.Content)+len(content), budget)
			}

			withoutSubstance := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content}}
			withSubstance := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: content, Substance: "z"}}

			blockWithout, dispositionsWithout := Assemble(anchor, withoutSubstance, budget, 0, 0)
			blockWith, dispositionsWith := Assemble(anchor, withSubstance, budget, 0, 0)

			if !dispositionsWithout[0].Included || !dispositionsWith[0].Included {
				t.Fatalf("test setup error: the candidate was cut (without a substance %v, with one %v); a cut row renders in neither arm and the comparison separates nothing", dispositionsWithout[0].Included, dispositionsWith[0].Included)
			}
			if blockWith != blockWithout {
				t.Fatalf("at a threshold of 0 the block changed when the candidate carried a substance:\nwithout=%q\nwith=%q", blockWithout, blockWith)
			}
			if dispositionsWith[0].RenderedSize != len(content) {
				t.Fatalf("RenderedSize = %d at a threshold of 0, want %d - the off position charges the content it renders, whatever substance arrived beside it", dispositionsWith[0].RenderedSize, len(content))
			}
		})
	}
}

func TestAssembleRendersContentForAZeroLengthSubstanceSoTheOffPositionNeverRendersAnEmptyPayload(t *testing.T) {
	t.Parallel()

	const body = "a content body"

	for _, threshold := range []SubstanceRatio{0, formRuleTestThreshold} {
		t.Run(fmt.Sprintf("threshold=%v", threshold), func(t *testing.T) {
			t.Parallel()

			candidates := []Candidate{{ID: 10, Content: body, Substance: ""}}

			block, dispositions := Assemble(Anchor{ID: 1}, candidates, 60_000, 0, threshold)

			if dispositions[0].Form != FormContent {
				t.Fatalf("Form = %q for a zero-length substance at a threshold of %v, want %q - presence is a string test taken before the ratio, and a ratio of zero would otherwise select an empty payload", dispositions[0].Form, threshold, FormContent)
			}
			if dispositions[0].RenderedSize != len(body) {
				t.Fatalf("RenderedSize = %d at a threshold of %v, want %d", dispositions[0].RenderedSize, threshold, len(body))
			}
			if !strings.Contains(block, body) {
				t.Fatalf("the block does not carry the content at a threshold of %v; a zero-length substance rendered in its place", threshold)
			}
		})
	}
}

func TestTurnRunRendersTheBlockAtTheDialItsOwnRecordDeclares(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)
	graph := baseGraph()
	graph.candidates = []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Similarity: 0.9, Content: content, Substance: substance}}
	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, nil, "the system text", "test-model-id", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.Candidates) != 1 || !record.Candidates[0].Included {
		t.Fatalf("test setup error: record.Candidates = %+v, want the one candidate admitted; a cut row renders nothing and this test can then observe no form at all", record.Candidates)
	}
	if !record.Candidates[0].SubstanceAvailable {
		t.Fatal("test setup error: the candidate reached admission carrying no substance, so this turn never read a dial and pins nothing")
	}

	wantBlock, _ := Assemble(graph.node, graph.candidates, AssemblyByteBudget, RelevanceFloor, record.Limits.SubstanceRatioThreshold)
	if record.Block != wantBlock {
		t.Fatalf("the run rendered a block its own record cannot account for at the dial the record states (%v):\ngot  %q\nwant %q", record.Limits.SubstanceRatioThreshold, record.Block, wantBlock)
	}
	if record.Candidates[0].Form != FormContent || record.Candidates[0].RenderedSize != 1000 {
		t.Fatalf("Form = %q at RenderedSize %d, want %q at 1000 - the shipped dial is off and a turn renders content for every candidate whatever substance arrived beside it", record.Candidates[0].Form, record.Candidates[0].RenderedSize, FormContent)
	}
	if !strings.Contains(record.Block, content) {
		t.Fatal("the block does not carry the candidate's content at the dial's off position")
	}
	if strings.Contains(record.Block, substance) {
		t.Fatal("the block carries the candidate's substance at the dial's off position, and the record states a dial of zero beside it")
	}
}

func TestTurnRunRendersASupplementaryHitAtTheDialItsAdmissionCharged(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)
	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Similarity: 0.9, Content: "initial"}}},
		{Candidates: []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Similarity: 0.9, Content: content, Substance: substance}}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 || len(record.ToolCalls[0].Results) != 1 {
		t.Fatalf("test setup error: record.ToolCalls = %+v, want one round carrying the one supplementary hit", record.ToolCalls)
	}

	charged := record.ToolCalls[0].Results[0]
	if !charged.Included || !charged.SubstanceAvailable {
		t.Fatalf("test setup error: the supplementary hit was admitted %v carrying a substance %v, want both true or the round read no dial", charged.Included, charged.SubstanceAvailable)
	}
	if charged.Form != FormContent || charged.RenderedSize != 1000 {
		t.Fatalf("the round charged Form %q at RenderedSize %d, want %q at 1000 - the supplementary path runs the same dial as the block and it ships off", charged.Form, charged.RenderedSize, FormContent)
	}

	if len(model.calls) < 2 || len(model.calls[1].PriorTools) != 1 {
		t.Fatalf("test setup error: the second judgement carries %d completed rounds, want 1", len(model.calls[1].PriorTools))
	}
	rendered := RenderToolResult(model.calls[1].PriorTools[0])
	if !strings.Contains(rendered, content) {
		t.Fatal("the tool result does not carry the content the round charged for")
	}
	if strings.Contains(rendered, substance) {
		t.Fatal("the tool result carries the substance although the round charged the content; the supplementary path must not charge one form and render another")
	}
}

func TestRenderToolResultRendersTheFormItsAdmissionChargedForAtTheSameDial(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)
	candidates := []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget, 0, formRuleTestThreshold)
	if !dispositions[0].Included || dispositions[0].Form != FormSubstance {
		t.Fatalf("test setup error: the candidate was admitted %v as Form %q, want admitted as %q or the divergence this test looks for cannot arise", dispositions[0].Included, dispositions[0].Form, FormSubstance)
	}

	got := RenderToolResult(ToolExchange{
		Tool:                    ToolRecall,
		Query:                   "q",
		Results:                 admitted,
		Dispositions:            dispositions,
		SubstanceRatioThreshold: formRuleTestThreshold,
	})

	want := "===== RESULT =====\nid: 91\ntype: documentation\nname: Bravo\n\n" + substance + "\n"
	if got != want {
		t.Fatalf("a round that charged %d bytes rendered:\n%q\nwant:\n%q", dispositions[0].RenderedSize, got, want)
	}
}

func TestRenderToolResultRendersTheContentAtTheOffPositionForAResultCarryingASubstance(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1000)
	substance := strings.Repeat("z", 300)

	got := RenderToolResult(ToolExchange{
		Tool:    ToolRecall,
		Query:   "q",
		Results: []Candidate{{ID: 91, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}},
	})

	want := "===== RESULT =====\nid: 91\ntype: documentation\nname: Bravo\n\n" + content + "\n"
	if got != want {
		t.Fatalf("at the dial's off position the tool result rendered:\n%q\nwant:\n%q", got, want)
	}
}
