package loop

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const bigNode = 71

func wantsSegment(id int64, segment int) JudgeResult {
	read := wantsRead(id)
	read.ReadSegment = segment
	return read
}

func graphHoldingABigNode() (*fakeGraph, string) {
	content := markdownDocument(40, 1_900)
	return graphHoldingNodes(readableNode(bigNode, content)), content
}

func runTurn(t *testing.T, graph *fakeGraph, results ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: results}
	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return record, model
}

func TestAReadOfANodeLargerThanTheRoomReturnsItsOutline(t *testing.T) {
	t.Parallel()

	graph, content := graphHoldingABigNode()
	if len(content) <= SupplementaryByteBudget {
		t.Fatalf("test setup error: the node is %d bytes, so one read carries it and no outline is due", len(content))
	}

	record, model := runTurn(t, graph, wantsRead(bigNode), answeredFinal())

	round := onlyRound(t, record)
	if round.Error != "" {
		t.Fatalf("the read of a %d-byte node was refused with %q, want its outline served", len(content), round.Error)
	}
	if len(round.Results) != 1 || round.Results[0].Form != FormOutline || !round.Results[0].Included {
		t.Fatalf("the round carries %+v, want one included row in form %q", round.Results, FormOutline)
	}
	if got := round.Results[0]; got.Size != len(content) || got.ID != bigNode {
		t.Errorf("the outline row names id %d and size %d, want %d and %d: it describes the node, not itself", got.ID, got.Size, bigNode, len(content))
	}

	block := model.calls[1].Block
	for _, want := range []string{
		"form: outline\n",
		fmt.Sprintf("outline of part %d, ", bigNode),
		"1. Section 1 (",
		"40. Section 40 (",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("the next call's block lacks %q:\n%.600s", want, block)
		}
	}
	if strings.Contains(block, strings.Repeat("w", 100)) {
		t.Errorf("the next call's block carries body text of the node: an outline is the table of contents and nothing else")
	}
}

func TestAnOutlineIsServedSoItDoesNotReserveTheNextCall(t *testing.T) {
	t.Parallel()

	graph, _ := graphHoldingABigNode()
	record, model := runTurn(t, graph, wantsRead(bigNode), answeredFinal())

	if len(model.calls) != 2 {
		t.Fatalf("the model was called %d times, want 2", len(model.calls))
	}
	if got := model.calls[1].Offered; !slices.Equal(got, []string{"recall", "readNode"}) {
		t.Errorf("the call after a served outline offers %v, want recall and readNode: an outline is a result, and a refusal is the only thing that reserves the call", got)
	}
	if record.ReservedCall.State != "" || record.CapReached {
		t.Errorf("reservedCall = %+v, capReached = %v, want none", record.ReservedCall, record.CapReached)
	}
}

func TestASegmentReadAddsThatSegmentToWorkingMemory(t *testing.T) {
	t.Parallel()

	graph, content := graphHoldingABigNode()
	segments := Segments(content, SupplementaryByteBudget)

	record, model := runTurn(t, graph, wantsRead(bigNode), wantsSegment(bigNode, 3), wantsSegment(bigNode, 5), answeredFinal())

	if len(record.ToolCalls) != 3 {
		t.Fatalf("the record carries %d rounds, want 3: %+v", len(record.ToolCalls), record.ToolCalls)
	}
	second := record.ToolCalls[1]
	if second.Error != "" || second.Segment != 3 || second.NodeID != bigNode {
		t.Fatalf("the segment round is %+v, want node %d, segment 3 and no error", second, bigNode)
	}
	if len(second.Results) != 1 || second.Results[0].Form != FormSegment || second.Results[0].Segment != 3 || second.Results[0].RenderedSize != len(segments[2]) {
		t.Errorf("the segment round's row is %+v, want form %q, segment 3, rendered size %d", second.Results, FormSegment, len(segments[2]))
	}
	if second.Yield != 1 {
		t.Errorf("the segment round yielded %d, want 1: it put a row in front of the model for the first time", second.Yield)
	}

	if len(model.calls) != 4 {
		t.Fatalf("the model was called %d times, want 4", len(model.calls))
	}
	afterThird := model.calls[3].Block
	for _, want := range []string{segments[2], segments[4], "form: segment\nsegment: 3\n", "form: segment\nsegment: 5\n", "form: outline\n"} {
		if !strings.Contains(afterThird, want) {
			t.Errorf("the block after two segment reads lacks %.60q: a segment joins the memory beside the outline and the other segments", want)
		}
	}
	if strings.Contains(afterThird, segments[3]) {
		t.Errorf("the block carries segment 4 that nobody read")
	}
	outline, three, five := strings.Index(afterThird, "form: outline"), strings.Index(afterThird, "segment: 3"), strings.Index(afterThird, "segment: 5")
	if outline > three || three > five {
		t.Errorf("the memory renders outline, segment 3, segment 5 out of order (%d, %d, %d)", outline, three, five)
	}
}

func TestASegmentAlreadyHeldIsRefusedAndReserves(t *testing.T) {
	t.Parallel()

	graph, _ := graphHoldingABigNode()
	record, model := runTurn(t, graph, wantsSegment(bigNode, 2), wantsSegment(bigNode, 2), answeredFinal())

	if len(record.ToolCalls) != 2 {
		t.Fatalf("the record carries %d rounds, want 2", len(record.ToolCalls))
	}
	repeat := record.ToolCalls[1]
	if repeat.Error != errAlreadyReadInFull || len(repeat.Results) != 0 {
		t.Errorf("the second read of segment 2 is %+v, want the refusal %q and no rows", repeat, errAlreadyReadInFull)
	}
	if len(model.calls) != 3 || len(model.calls[2].Offered) != 0 {
		t.Errorf("the call after the refusal offers %v (call count %d), want nothing: a refusal reserves the next call for answering", model.calls[len(model.calls)-1].Offered, len(model.calls))
	}
}

func TestADifferentSegmentOfTheSameNodeIsNotAlreadyHeld(t *testing.T) {
	t.Parallel()

	graph, _ := graphHoldingABigNode()
	record, _ := runTurn(t, graph, wantsSegment(bigNode, 2), wantsSegment(bigNode, 3), answeredFinal())

	for i, round := range record.ToolCalls {
		if round.Error != "" {
			t.Errorf("round %d was refused with %q: segments are held apart", i+1, round.Error)
		}
	}
}

func TestAnOutOfRangeSegmentIsRefused(t *testing.T) {
	t.Parallel()

	graph, _ := graphHoldingABigNode()
	record, model := runTurn(t, graph, wantsSegment(bigNode, 41), answeredFinal())

	round := onlyRound(t, record)
	if want := "that part has 40 segments and none is numbered 41"; round.Error != want || len(round.Results) != 0 {
		t.Errorf("the round is %+v, want the refusal %q and no rows", round, want)
	}
	if round.Segment != 41 {
		t.Errorf("the refused round records segment %d, want 41: the record says what the model asked for", round.Segment)
	}
	if len(model.calls) != 2 || len(model.calls[1].Offered) != 0 {
		t.Errorf("the call after the refusal offers %v, want nothing", model.calls[len(model.calls)-1].Offered)
	}
}

func TestANodeOfExactlyOneReadIsServedWholeAndOneByteMoreIsOutlined(t *testing.T) {
	t.Parallel()

	exact := readableNode(bigNode, strings.Repeat("x", SupplementaryByteBudget))
	record, _ := runTurn(t, graphHoldingNodes(exact), wantsRead(bigNode), answeredFinal())
	if round := onlyRound(t, record); round.Error != "" || round.Results[0].Form != FormContent {
		t.Errorf("a node of exactly %d bytes gave %+v, want it served whole", SupplementaryByteBudget, round)
	}

	over := readableNode(bigNode, strings.Repeat("x", SupplementaryByteBudget+1))
	record, _ = runTurn(t, graphHoldingNodes(over), wantsRead(bigNode), answeredFinal())
	if round := onlyRound(t, record); round.Error != "" || round.Results[0].Form != FormOutline {
		t.Errorf("a node of %d bytes gave %+v, want its outline", SupplementaryByteBudget+1, round)
	}
}

func TestAnOutlineReadTwiceReplacesItAndIsHeldOnce(t *testing.T) {
	t.Parallel()

	graph, _ := graphHoldingABigNode()
	_, model := runTurn(t, graph, wantsRead(bigNode), wantsRead(bigNode), answeredFinal())

	if len(model.calls) != 3 {
		t.Fatalf("the model was called %d times, want 3: both outline reads dispatch and the third call answers", len(model.calls))
	}
	if got := strings.Count(model.calls[2].Block, "form: outline\n"); got != 1 {
		t.Errorf("the block carries the outline %d times after it was read twice, want once", got)
	}
}

func TestAWholeReadEvictsEveryOtherPartOfTheSameNode(t *testing.T) {
	t.Parallel()

	const id = 91
	content := strings.Repeat("c", 1_000)
	anchor := Anchor{ID: id, Type: "documentation", Name: "Bravo", Content: content}

	memory := newWorkingMemory(Anchor{ID: 1}, false, formRuleTestThreshold, nil, nil)
	for _, part := range []struct {
		form    Form
		segment int
		body    string
	}{
		{FormOutline, 0, "the outline"},
		{FormSegment, 1, "segment one"},
		{FormSegment, 2, "segment two"},
	} {
		memory.hold(partCandidate(anchor, part.form, part.segment, part.body), part.form, len(part.body))
	}
	other := partCandidate(Anchor{ID: 92, Type: "documentation", Name: "Other"}, FormSegment, 1, "another node's segment")
	memory.hold(other, FormSegment, len("another node's segment"))

	memory.hold(candidateFromAnchor(anchor), FormContent, len(content))

	rendered := memory.render(false)
	for _, gone := range []string{"the outline", "segment one", "segment two"} {
		if strings.Contains(rendered, gone) {
			t.Errorf("after a whole read the memory still carries %q:\n%s", gone, rendered)
		}
	}
	if !strings.Contains(rendered, content) || !strings.Contains(rendered, "another node's segment") {
		t.Errorf("the whole read lost the node's body or another node's segment:\n%s", rendered)
	}
	if want := len(content) + len("another node's segment"); memory.spent != want {
		t.Errorf("memory.spent = %d, want %d: an evicted part must stop being charged", memory.spent, want)
	}
}

func TestAnOutlineDoesNotEvictASubstanceOfTheSameNode(t *testing.T) {
	t.Parallel()

	const id = 91
	content := strings.Repeat("c", 1_000)
	substance := strings.Repeat("z", 300)
	anchor := Anchor{ID: id, Type: "documentation", Name: "Bravo", Content: content}

	memory := memoryHolding(t, Anchor{ID: 1}, formRuleTestThreshold, []Candidate{{ID: id, Type: "documentation", Name: "Bravo", Content: content, Substance: substance}})
	memory.hold(partCandidate(anchor, FormOutline, 0, "the outline"), FormOutline, len("the outline"))

	rendered := memory.render(false)
	if !strings.Contains(rendered, substance) || !strings.Contains(rendered, "the outline") {
		t.Errorf("the outline evicted the substance, or was not held beside it:\n%s", rendered)
	}
	if want := len(substance) + len("the outline"); memory.spent != want {
		t.Errorf("memory.spent = %d, want %d", memory.spent, want)
	}
}

func TestAnOutlineOrSegmentRowIsRenderedAndChargedByItsOwnBodyAndNeverAsWhole(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 91, Type: "documentation", Name: "Bravo", Content: strings.Repeat("c", 50_000)}
	row := partCandidate(anchor, FormSegment, 4, "the fourth segment")

	form, payload := renderedPayload(row, formRuleTestThreshold)
	if form != FormSegment || payload != "the fourth segment" {
		t.Errorf("renderedPayload = %q, %q, want the segment form and its own body: admission and the renderer must read one seam", form, payload)
	}

	d := partDisposition(anchor, FormSegment, 4, "the fourth segment")
	if d.RenderedSize != len("the fourth segment") || d.Size != 50_000 || d.Segment != 4 || !d.Included {
		t.Errorf("disposition = %+v, want rendered size 18, node size 50000, segment 4, included", d)
	}
}

func TestTheLastSegmentOfANodeIsReadableAndOnePastItIsNot(t *testing.T) {
	t.Parallel()

	graph, content := graphHoldingABigNode()
	segments := Segments(content, SupplementaryByteBudget)

	record, _ := runTurn(t, graph, wantsSegment(bigNode, len(segments)), answeredFinal())

	round := onlyRound(t, record)
	if round.Error != "" || len(round.Results) != 1 || round.Results[0].Segment != len(segments) {
		t.Errorf("the read of the last segment (%d) is %+v, want it served", len(segments), round)
	}
}

func TestAWholeReadHasTheRoomEveryHeldPartOfThatNodeFrees(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 5, Type: "documentation", Name: "Five"}
	memory := newWorkingMemory(Anchor{ID: 1, Content: strings.Repeat("a", JudgementPromptCeiling-1_000)}, false, formRuleTestThreshold, nil, nil)
	memory.hold(partCandidate(anchor, FormOutline, 0, "o"), FormOutline, 300)
	memory.hold(partCandidate(anchor, FormSegment, 1, "s"), FormSegment, 200)

	if got, want := memory.roomToReplace(5, SupplementaryByteBudget), 1_000-500+500; got != want {
		t.Errorf("a whole read of node 5 has room %d, want %d: the outline and the segment it replaces are given back", got, want)
	}
	if got, want := memory.roomToReplace(6, SupplementaryByteBudget), 500; got != want {
		t.Errorf("a whole read of a node holding nothing has room %d, want %d", got, want)
	}
	if got, want := memory.roomToReplacePart(keyOf(5, FormSegment, 1), SupplementaryByteBudget), 700; got != want {
		t.Errorf("a re-read of segment 1 has room %d, want %d: only that slot is given back", got, want)
	}
}
