package loop

import (
	"fmt"
	"strings"
	"testing"
)

func nearTheCeilingHolding(node Anchor) *fakeGraph {
	body := func(n int) string { return strings.Repeat("#", n) }

	graph := graphHoldingNodes(node)
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{rowOf(11, body(14_000)), rowOf(12, body(14_000)), rowOf(13, body(14_000)), rowOf(14, body(14_000))}},
		{Candidates: []Candidate{rowOf(21, body(19_990))}},
	}
	return graph
}

func runNearTheCeiling(t *testing.T, node Anchor, reads ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	results := append([]JudgeResult{recallFor("fill")}, reads...)
	record, model := runTurn(t, nearTheCeilingHolding(node), append(results, answeredFinal())...)

	if fill := record.ToolCalls[0]; fill.Error != "" || len(fill.Results) != 1 || !fill.Results[0].Included {
		t.Fatalf("test setup error: the filling recall is %+v, want its one row admitted so 3999 bytes of room are left", fill)
	}
	return record, model
}

func sectionsOf(sizes ...int) string {
	var b strings.Builder
	for i, size := range sizes {
		fmt.Fprintf(&b, "## Part %d\n\n%s\n\n", i+1, strings.Repeat("s", size))
	}
	return b.String()
}

func TestAReadOfANodeThatFitsOneReadButNotTheRoomLeftReturnsItsOutline(t *testing.T) {
	t.Parallel()

	content := sectionsOf(5_000, 5_000, 5_000)
	if len(content) > SupplementaryByteBudget {
		t.Fatalf("test setup error: the node is %d bytes, which is more than one read", len(content))
	}

	record, model := runNearTheCeiling(t, readableNode(bigNode, content), wantsRead(bigNode))

	round := record.ToolCalls[1]
	if round.Error != "" || len(round.Results) != 1 || round.Results[0].Form != FormOutline {
		t.Fatalf("the read of a %d-byte node with 3999 bytes of room is %+v, want its outline served", len(content), round)
	}
	if !strings.Contains(model.calls[2].Block, "form: outline\n") {
		t.Errorf("the next call's block carries no outline")
	}
	if len(model.calls[2].Offered) == 0 {
		t.Errorf("the call after a served outline offers nothing: an outline is a result, not a refusal")
	}
}

func TestAnOutlineIsRefusedWhereTheCeilingLeavesLessRoomThanItNeeds(t *testing.T) {
	t.Parallel()

	sizes := make([]int, 300)
	for i := range sizes {
		sizes[i] = 20
	}
	content := sectionsOf(sizes...)
	segments := Segments(content, SupplementaryByteBudget)
	outline := len(Outline(bigNode, "Read me", len(content), segments))
	if outline <= 3_999 || outline > SupplementaryByteBudget {
		t.Fatalf("test setup error: the outline is %d bytes, which is not between the room left and one read", outline)
	}

	record, model := runNearTheCeiling(t, readableNode(bigNode, content), wantsRead(bigNode))

	round := record.ToolCalls[1]
	if want := fmt.Sprintf("that part is %d bytes and one read carries at most 3999, so it cannot be shown in full", outline); round.Error != want || len(round.Results) != 0 {
		t.Errorf("the round is %+v, want the refusal %q: the outline is charged against the ceiling like any row, and the refusal names the outline's own size", round, want)
	}
	if len(model.calls[2].Offered) != 0 {
		t.Errorf("the call after the refusal offers %v, want nothing", model.calls[2].Offered)
	}
}

func TestASegmentIsRefusedWhereTheCeilingLeavesLessRoomThanItNeedsAndAnotherThatFitsIsServed(t *testing.T) {
	t.Parallel()

	content := sectionsOf(5_000, 3_000)
	segments := Segments(content, SupplementaryByteBudget)
	if len(segments) != 2 || len(segments[0]) <= 3_999 || len(segments[1]) > 3_999 {
		t.Fatalf("test setup error: segment sizes %d and %d, want one over the 3999 bytes of room and one within", len(segments[0]), len(segments[1]))
	}

	refused, _ := runNearTheCeiling(t, readableNode(bigNode, content), wantsSegment(bigNode, 1))
	round := refused.ToolCalls[1]
	if want := fmt.Sprintf("that part is %d bytes and one read carries at most 3999, so it cannot be shown in full", len(segments[0])); round.Error != want || len(round.Results) != 0 {
		t.Errorf("the round is %+v, want the refusal %q: the segment is charged against the ceiling and the refusal names the segment's size, not the node's", round, want)
	}

	served, _ := runNearTheCeiling(t, readableNode(bigNode, content), wantsSegment(bigNode, 2))
	if round := served.ToolCalls[1]; round.Error != "" || len(round.Results) != 1 || round.Results[0].Segment != 2 {
		t.Errorf("the read of the segment that fits is %+v, want it served", round)
	}
}

func TestASegmentOfANodeAlreadyHeldWholeIsRefusedAndReserves(t *testing.T) {
	t.Parallel()

	record, model := runTurn(t, graphHoldingAShownPart(), wantsSegment(91, 1), answeredFinal())

	round := refusedRound(t, record, errAlreadyReadInFull)
	if round.Segment != 1 {
		t.Errorf("the refused round records segment %d, want 1", round.Segment)
	}
	if len(model.calls) != 2 || len(model.calls[1].Offered) != 0 {
		t.Errorf("the call after the refusal offers %v, want nothing", model.calls[len(model.calls)-1].Offered)
	}
}

func TestTheRenderedAccountNamesASegmentAndAnOutlineReadRound(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{
		{Tool: ToolReadNode, Source: ToolSourceNative, NodeID: 71, Segment: 3, Results: []Disposition{{Rank: 1, ID: 71, Size: 57_902, RenderedSize: 7_000, Included: true, Form: FormSegment, Segment: 3}}},
		{Tool: ToolReadNode, Source: ToolSourceNative, NodeID: 72, Results: []Disposition{{Rank: 1, ID: 72, Size: 57_902, RenderedSize: 1_800, Included: true, Form: FormOutline}}},
		{Tool: ToolReadNode, Source: ToolSourceNative, NodeID: 73, Results: []Disposition{{Rank: 1, ID: 73, Size: 900, RenderedSize: 900, Included: true, Form: FormContent}}},
	}

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"1 " + ToolReadNode + " [native]  #71 segment 3\n",
		"2 " + ToolReadNode + " [native]  #72 outline\n",
		"3 " + ToolReadNode + " [native]  #73\n",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("the account lacks %q:\n%s", want, summary)
		}
	}
}
