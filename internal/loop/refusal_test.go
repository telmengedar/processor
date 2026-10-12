package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

func answeringWithTool(call JudgeResult) JudgeResult {
	call.Answer = "the answer"
	return call
}

func graphHoldingAShownPart() *fakeGraph {
	held := readableNode(91, "a part the block already carries whole")
	graph := graphHoldingNodes(held)
	graph.candidates = []Candidate{{ID: held.ID, Type: held.Type, Name: held.Name, Similarity: 0.9, Content: held.Content}}
	return graph
}

func TestRepeatedRefusedReadsOfAPartAlreadyShownDoNotRunTheTurnToTheCap(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{answeringWithTool(wantsRead(91))}}

	record, _, err := turnComposing(graphHoldingAShownPart(), model, nil).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 2 {
		t.Errorf("the model was called %d times, want 2: the first request is refused and the next call answers", len(model.calls))
	}
	if record.CapReached || record.RecallClosed {
		t.Errorf("capReached=%v recallClosed=%v, want both false: the refusal reserved the call, not the budget or the yield account", record.CapReached, record.RecallClosed)
	}
	if record.ReservedCall.State != ReservedCallCompleted {
		t.Errorf("reservedCall.state = %q, want %q", record.ReservedCall.State, ReservedCallCompleted)
	}
	if record.Answer != "the answer" || !record.Outcome.Produced {
		t.Errorf("answer = %q, produced = %v, want the answer the reserved call gave", record.Answer, record.Outcome.Produced)
	}
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != "that part has already been shown to you in full in this turn" {
		t.Errorf("record.toolCalls = %+v, want exactly the one refused round", record.ToolCalls)
	}
}

func TestTheCallAfterARefusedReadOffersNoToolAndIsComposedForAnswering(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{wantsRead(91), answered("done")}}

	if _, _, err := turnComposing(graphHoldingAShownPart(), model, nil).Run(context.Background(), "hello", readSubject); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 2 {
		t.Fatalf("the model was called %d times, want 2", len(model.calls))
	}

	if first := model.calls[0]; !slices.Equal(first.Offered, []string{"recall", "readNode"}) || first.System != "offers:recall,readNode" {
		t.Errorf("the first call offers %v with system text %q, want recall and readNode", first.Offered, first.System)
	}
	if second := model.calls[1]; len(second.Offered) != 0 || second.System != "offers:" {
		t.Errorf("the call after the refusal offers %v with system text %q, want nothing and the empty-set composition", second.Offered, second.System)
	}
}

func TestEveryRefusedRetrievalRoundReservesTheNextCallForAnswering(t *testing.T) {
	t.Parallel()

	malformed := "tool arguments could not be parsed: unexpected token"
	tooLarge := readableNode(71, strings.Repeat("x", 25_000))

	for _, tc := range []struct {
		name  string
		graph func() *fakeGraph
		first JudgeResult
	}{
		{"a part already shown in full", graphHoldingAShownPart, wantsRead(91)},
		{"the request's own subject", func() *fakeGraph { return graphHoldingNodes() }, wantsRead(readSubject)},
		{"an id the graph does not hold", func() *fakeGraph { return graphHoldingNodes() }, wantsRead(72)},
		{"a part too large for one read", func() *fakeGraph { return graphHoldingNodes(tooLarge) }, wantsRead(71)},
		{"a malformed recall", func() *fakeGraph { return graphHoldingNodes() }, JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", ToolError: malformed}},
		{"a malformed read", func() *fakeGraph { return graphHoldingNodes() }, JudgeResult{Reason: WantsRead, RawReason: "tool_calls", ToolError: malformed}},
	} {
		model := &fakeModel{results: []JudgeResult{tc.first, answered("done")}}

		record, _, err := turnComposing(tc.graph(), model, nil).Run(context.Background(), "hello", readSubject)
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}

		if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error == "" {
			t.Fatalf("%s: test setup error: record.toolCalls = %+v, want one errored round", tc.name, record.ToolCalls)
		}
		if len(model.calls) != 2 || len(model.calls[1].Offered) != 0 {
			t.Errorf("%s: the model was called %d times and the last call offered %v, want 2 calls and nothing offered after the refusal", tc.name, len(model.calls), model.calls[len(model.calls)-1].Offered)
		}
		if record.ReservedCall.State != ReservedCallCompleted || record.CapReached {
			t.Errorf("%s: reservedCall.state = %q, capReached = %v, want a completed reserved call and no cap", tc.name, record.ReservedCall.State, record.CapReached)
		}
	}
}

func TestATransportFailedReadDoesNotReserveTheNextCall(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes()
	graph.nodes[71] = nodeResponse{Err: errors.New("graph down")}
	model := &fakeModel{results: []JudgeResult{wantsRead(71), answered("done")}}

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != "graph down" {
		t.Fatalf("test setup error: record.toolCalls = %+v, want the one failed read", record.ToolCalls)
	}
	if got := model.calls[1].Offered; !slices.Equal(got, []string{"recall", "readNode"}) {
		t.Errorf("the call after a transport-failed read offers %v, want recall and readNode: an outage is not a refusal, and a repeat may succeed", got)
	}
	if record.ReservedCall.State != "" {
		t.Errorf("reservedCall.state = %q, want none", record.ReservedCall.State)
	}
}

func TestABarrenServedRecallLeavesTheNextCallOfferingTools(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{recallFor("nothing new"), answered("done")}}

	graph := baseGraph()
	graph.candidates = repeatedRows()

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != "" || record.ToolCalls[0].Yield != 0 {
		t.Fatalf("test setup error: record.toolCalls = %+v, want one served barren recall", record.ToolCalls)
	}
	if got := model.calls[1].Offered; !slices.Equal(got, []string{"recall", "readNode"}) {
		t.Errorf("the call after a barren served recall offers %v, want recall and readNode: barren rounds stay with the two-round closure", got)
	}
}

func TestAFailedWriteDoesNotReserveTheNextCall(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must not leave the working directory", ErrWriteRejected)}
	model := &fakeModel{results: []JudgeResult{wantsWriteOf("../escape.html", "x"), answered("done")}}

	record, _, err := turnComposing(baseGraph(), model, files).Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error == "" {
		t.Fatalf("test setup error: record.toolCalls = %+v, want one rejected write", record.ToolCalls)
	}
	if got := model.calls[1].Offered; !slices.Equal(got, []string{"recall", "readNode", "writeFile"}) {
		t.Errorf("the call after a rejected write offers %v, want all three tools: writes are not retrieval refusals", got)
	}
}

func TestARefusalOnTheFifthCallReservesTheSixthWithoutReachingTheCap(t *testing.T) {
	t.Parallel()

	results := append(recallsDifferingOnlyByADateSuffix(MaxModelCalls-2), wantsRead(readSubject), answered("done"))
	model := &fakeModel{results: results}

	graph := graphYieldingNewRowsToEveryRecall()
	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != MaxModelCalls || len(record.ToolCalls) != MaxModelCalls-1 {
		t.Fatalf("test setup error: %d calls and %d rounds, want %d and %d", len(model.calls), len(record.ToolCalls), MaxModelCalls, MaxModelCalls-1)
	}
	if record.ToolCalls[MaxModelCalls-2].Error == "" {
		t.Fatalf("test setup error: the fifth round is %+v, want the refusal", record.ToolCalls[MaxModelCalls-2])
	}
	if record.CapReached {
		t.Error("capReached = true, want false: the sixth call was reserved by the refusal on the fifth, so the budget was not what spent it")
	}
	if record.ReservedCall.State != ReservedCallCompleted || len(model.calls[MaxModelCalls-1].Offered) != 0 {
		t.Errorf("reservedCall.state = %q and the sixth call offered %v, want completed and nothing", record.ReservedCall.State, model.calls[MaxModelCalls-1].Offered)
	}
}

func TestNothingAboutARefusalReachesTheAnsweringCall(t *testing.T) {
	t.Parallel()

	rows := make([]Candidate, 0, ThinKnowledgeThreshold)
	for i := range ThinKnowledgeThreshold {
		rows = append(rows, rowOf(int64(200+i), fmt.Sprintf("body-%d", i)))
	}
	held := rows[0]

	graph := graphHoldingNodes(readableNode(held.ID, held.Content))
	graph.candidates = rows
	model := &fakeModel{results: []JudgeResult{wantsRead(held.ID), answered("done")}}

	if _, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", readSubject); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 2 {
		t.Fatalf("test setup error: the model was called %d times, want 2", len(model.calls))
	}

	if model.calls[1].Block != model.calls[0].Block {
		t.Errorf("the answering call's block differs from the call before it, which carried no search nudge:\ncall 1: %q\ncall 2: %q", model.calls[0].Block, model.calls[1].Block)
	}
	for name, surface := range map[string]string{"block": model.calls[1].Block, "system text": model.calls[1].System, "input": model.calls[1].Input} {
		for _, leaked := range []string{"already been shown", "refus", "error", "readNode", "read_node"} {
			if strings.Contains(surface, leaked) {
				t.Errorf("the answering call's %s carries %q: nothing about a refusal may reach any call", name, leaked)
			}
		}
	}
}

func TestARefusalReservedTurnIsRecordedAsCurtailedAndRaisesTheDidNotDeliverWarning(t *testing.T) {
	t.Parallel()

	var logged strings.Builder
	model := &fakeModel{results: []JudgeResult{wantsRead(91), answered("the answer")}}
	turn := NewTurn(graphHoldingAShownPart(), model, nil, offeringSystem, "test-model", slog.New(slog.NewTextHandler(&logged, nil)))

	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.CapReached || record.RecallClosed || record.ReservedCall.State != ReservedCallCompleted {
		t.Fatalf("test setup error: capReached=%v recallClosed=%v reservedCall=%q, want a turn reserved by the refusal alone", record.CapReached, record.RecallClosed, record.ReservedCall.State)
	}
	if record.Outcome.Verdict != VerdictCurtailed || !record.Outcome.Curtailed {
		t.Errorf("outcome = %+v, want verdict %q and curtailed: the model still wanted a tool round and was denied one", record.Outcome, VerdictCurtailed)
	}
	if !strings.Contains(logged.String(), "the run did not deliver") || !strings.Contains(logged.String(), "verdict=curtailed") {
		t.Errorf("a refusal-reserved turn raised no did-not-deliver warning naming verdict=curtailed; log:\n%s", logged.String())
	}
}
