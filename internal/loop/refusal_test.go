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
	graph.candidates = []Candidate{{ID: held.ID, Type: held.Type, Name: held.Name, Similarity: 0.9, Content: held.Content, Substance: held.Substance}}
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
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != "that part has already been shown to you in this turn" {
		t.Errorf("record.toolCalls = %+v, want exactly the one refused round", record.ToolCalls)
	}
}

func TestTheCallAfterARefusedReadOffersNoToolAndIsComposedForAnswering(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{wantsRead(91), answered("done")}}

	record, _, err := turnComposing(graphHoldingAShownPart(), model, nil).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 2 {
		t.Fatalf("the model was called %d times, want 2", len(model.calls))
	}
	assertOfferedMatchesCalls(t, record, model.calls)

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

func TestEveryRefusedWriteRoundReservesTheNextCallForAnswering(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		files func() *fakeFiles
		first JudgeResult
	}{
		{"a path the working directory rejects", func() *fakeFiles {
			return &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must not leave the working directory", ErrWriteRejected)}
		}, wantsWriteOf("../escape.html", "x")},
		{"a malformed write", func() *fakeFiles { return &fakeFiles{dir: "/runs/run-1"} },
			JudgeResult{Reason: WantsWrite, RawReason: "tool_calls", ToolError: "tool arguments could not be parsed: unexpected token"}},
		{"no working directory", func() *fakeFiles { return nil }, wantsWriteOf("page.html", "x")},
	} {
		model := &fakeModel{results: []JudgeResult{tc.first, answered("done")}}

		var files FilePort
		if f := tc.files(); f != nil {
			files = f
		}
		record, _, err := turnComposing(baseGraph(), model, files).Run(context.Background(), "make a page", 42)
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}

		if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error == "" {
			t.Fatalf("%s: test setup error: record.toolCalls = %+v, want one refused write", tc.name, record.ToolCalls)
		}
		if len(model.calls) != 2 || len(model.calls[1].Offered) != 0 {
			t.Errorf("%s: the model was called %d times and the last call offered %v, want 2 calls and nothing offered after the refusal", tc.name, len(model.calls), model.calls[len(model.calls)-1].Offered)
		}
		if record.ReservedCall.State != ReservedCallCompleted || record.CapReached {
			t.Errorf("%s: reservedCall.state = %q, capReached = %v, want a completed reserved call and no cap", tc.name, record.ReservedCall.State, record.CapReached)
		}
	}
}

func TestRepeatedRejectedWritesDoNotRunTheTurnToTheCap(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must not leave the working directory", ErrWriteRejected)}
	results := make([]JudgeResult, 0, MaxModelCalls+1)
	for range MaxModelCalls + 1 {
		results = append(results, wantsWriteOf("../escape.html", "x"))
	}
	model := &fakeModel{results: results}

	record, _, err := turnComposing(baseGraph(), model, files).Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 2 || record.CapReached {
		t.Errorf("the model was called %d times with capReached = %v, want exactly 2 calls and no cap", len(model.calls), record.CapReached)
	}
	if len(files.writes) != 1 {
		t.Errorf("the working directory received %d write attempts, want 1", len(files.writes))
	}
}

func TestAFilesystemFailureOfAWriteDoesNotReserveTheNextCall(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		files *fakeFiles
	}{
		{"a filesystem error on the write", &fakeFiles{dir: "/runs/run-1", writeErr: errors.New("disk full")}},
		{"a failure opening the run directory", &fakeFiles{openErr: errors.New("permission denied")}},
	} {
		model := &fakeModel{results: []JudgeResult{wantsWriteOf("page.html", "x"), answered("done")}}

		record, _, err := turnComposing(baseGraph(), model, tc.files).Run(context.Background(), "make a page", 42)
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}

		if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error == "" {
			t.Fatalf("%s: test setup error: record.toolCalls = %+v, want one failed write", tc.name, record.ToolCalls)
		}
		if got := model.calls[1].Offered; !slices.Equal(got, []string{"recall", "readNode", "writeFile"}) {
			t.Errorf("%s: the call after a failed write offers %v, want all three tools: a repeat may succeed", tc.name, got)
		}
		if record.ReservedCall.State != "" {
			t.Errorf("%s: reservedCall.state = %q, want none", tc.name, record.ReservedCall.State)
		}
	}
}

func TestARefusedWriteReservedTurnIsRecordedAsCurtailed(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must not leave the working directory", ErrWriteRejected)}
	model := &fakeModel{results: []JudgeResult{wantsWriteOf("../escape.html", "x"), answered("the answer")}}

	record, _, err := turnComposing(baseGraph(), model, files).Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.CapReached || record.RecallClosed || record.ReservedCall.State != ReservedCallCompleted {
		t.Fatalf("test setup error: capReached=%v recallClosed=%v reservedCall=%q, want a turn reserved by the refusal alone", record.CapReached, record.RecallClosed, record.ReservedCall.State)
	}
	if record.Outcome.Verdict != VerdictCurtailed || !record.Outcome.Curtailed {
		t.Errorf("outcome = %+v, want verdict %q and curtailed", record.Outcome, VerdictCurtailed)
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

	rows := make([]Candidate, 0, CandidateLimit)
	for i := range CandidateLimit {
		rows = append(rows, rowOf(int64(200+i), fmt.Sprintf("body-%d", i)))
	}
	held := rows[0]

	graph := graphHoldingNodes(readableNode(held.ID, held.Substance))
	graph.candidates = rows
	model := &fakeModel{results: []JudgeResult{wantsRead(held.ID), answered("done")}}

	if _, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", readSubject); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 2 {
		t.Fatalf("test setup error: the model was called %d times, want 2", len(model.calls))
	}

	if model.calls[1].Block != model.calls[0].Block {
		t.Errorf("the answering call's block differs from the call before it:\ncall 1: %q\ncall 2: %q", model.calls[0].Block, model.calls[1].Block)
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

func fieldsOfACall(call JudgeInput) map[string]string {
	return map[string]string{
		"system":       call.System,
		"block":        call.Block,
		"input":        call.Input,
		"conversation": call.Conversation,
		"clause":       call.Clause,
		"trailer":      call.Trailer,
		"offered":      strings.Join(call.Offered, "\n"),
	}
}

func TestNoRefusalOrToolErrorTextReachesAnyFieldOfAFollowingCall(t *testing.T) {
	t.Parallel()

	malformed := "tool arguments could not be parsed: unexpected token"
	withNotes := func(notes *fakeNotes, results ...JudgeResult) (Record, []JudgeInput) {
		run := runNotes(t, noteGraph(), noteView(), notes, &fakeModel{results: results})
		return run.record, run.model.calls
	}
	plain := func(graph *fakeGraph, files FilePort, subject int64, results ...JudgeResult) (Record, []JudgeInput) {
		model := &fakeModel{results: results}
		record, _, err := turnComposing(graph, model, files).Run(context.Background(), "hello", subject)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return record, model.calls
	}

	for _, tc := range []struct {
		name string
		run  func() (Record, []JudgeInput)
	}{
		{"a malformed recall", func() (Record, []JudgeInput) {
			return plain(baseGraph(), nil, 42, JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", ToolError: malformed}, answered("done"))
		}},
		{"a recall refused because recall is closed", func() (Record, []JudgeInput) {
			return plain(graphReturningTheSameRowsToEveryRecall(), nil, 42, recallFor("first"), recallFor("second"), recallFor("third"), recallFor("fourth"), answered("done"))
		}},
		{"a read of a part already shown", func() (Record, []JudgeInput) {
			return plain(graphHoldingAShownPart(), nil, readSubject, wantsRead(91), answered("done"))
		}},
		{"a read of an id the graph does not hold", func() (Record, []JudgeInput) {
			return plain(graphHoldingNodes(), nil, readSubject, wantsRead(72), answered("done"))
		}},
		{"a write the files port rejected", func() (Record, []JudgeInput) {
			files := &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must not leave the working directory", ErrWriteRejected)}
			return plain(baseGraph(), files, 42, wantsWriteOf("../escape.html", "x"), answered("done"))
		}},
		{"a malformed write", func() (Record, []JudgeInput) {
			files := &fakeFiles{dir: "/runs/run-1"}
			return plain(baseGraph(), files, 42, JudgeResult{Reason: WantsWrite, RawReason: "tool_calls", ToolError: malformed}, answered("done"))
		}},
		{"a write that failed for an unrecognised cause", func() (Record, []JudgeInput) {
			files := &fakeFiles{dir: "/runs/run-1", writeErr: errors.New("no space left on device")}
			return plain(baseGraph(), files, 42, wantsWriteOf("index.html", "x"), answered("done"))
		}},
		{"a note that was refused", func() (Record, []JudgeInput) {
			return withNotes(&fakeNotes{}, wantsNoteOf("   ", 0), answered("done"))
		}},
		{"a note the graph did not store", func() (Record, []JudgeInput) {
			return withNotes(&fakeNotes{receipt: WriteReceipt{State: NotStored}}, wantsNoteOf("a thought", 0), answered("done"))
		}},
	} {
		record, calls := tc.run()

		if len(record.ToolCalls) == 0 || len(calls) != len(record.ToolCalls)+1 {
			t.Errorf("%s: test setup error: %d calls for %d rounds, want one more call than rounds", tc.name, len(calls), len(record.ToolCalls))
			continue
		}
		errored := 0
		for round, tool := range record.ToolCalls {
			if tool.Error == "" {
				continue
			}
			errored++
			for later := round + 1; later < len(calls); later++ {
				for field, text := range fieldsOfACall(calls[later]) {
					if strings.Contains(text, tool.Error) {
						t.Errorf("%s: the %s of call %d carries round %d's error text %q, want it in no field of any later call:\n%s", tc.name, field, later+1, round+1, tool.Error, text)
					}
				}
			}
		}
		if errored == 0 {
			t.Errorf("%s: test setup error: no round recorded an error, so nothing was checked", tc.name)
		}
	}
}
