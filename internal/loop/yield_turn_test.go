package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const repeatedRowCount = 5

func repeatedRows() []Candidate {
	rows := make([]Candidate, 0, repeatedRowCount)
	for i := range repeatedRowCount {
		id := int64(100 + i)
		rows = append(rows, Candidate{ID: id, Type: "documentation", Name: fmt.Sprintf("Repeated %d", id), Similarity: 0.9, Content: fmt.Sprintf("body of %d", id)})
	}
	return rows
}

func freshRows(round int) []Candidate {
	rows := make([]Candidate, 0, repeatedRowCount)
	for i := range repeatedRowCount {
		id := int64(1000*round + i)
		rows = append(rows, Candidate{ID: id, Type: "documentation", Name: fmt.Sprintf("Fresh %d", id), Similarity: 0.9, Content: fmt.Sprintf("body of %d", id)})
	}
	return rows
}

func newRowsPerRecall() []recallResponse {
	queue := []recallResponse{{}}
	for round := range MaxModelCalls {
		queue = append(queue, recallResponse{Candidates: freshRows(round + 1)})
	}
	return queue
}

func graphYieldingNewRowsToEveryRecall() *fakeGraph {
	graph := baseGraph()
	graph.recallQueue = newRowsPerRecall()
	return graph
}

func recallsDifferingOnlyByADateSuffix(n int) []JudgeResult {
	results := make([]JudgeResult, 0, n)
	for i := range n {
		results = append(results, JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: fmt.Sprintf("what changed 2026-09-%02d", 18+i)})
	}
	return results
}

func graphReturningTheSameRowsToEveryRecall() *fakeGraph {
	graph := baseGraph()
	graph.recallQueue = []recallResponse{{}}
	graph.candidates = repeatedRows()
	return graph
}

func dispatchedRecalls(graph *fakeGraph) int {
	return len(graph.recallCalls) - primaryRecallCalls
}

func yieldsOf(record Record) []int {
	yields := make([]int, 0, len(record.ToolCalls))
	for _, call := range record.ToolCalls {
		if call.Tool != ToolRecall || call.Error != "" {
			continue
		}
		yields = append(yields, call.Yield)
	}
	return yields
}

func TestFiveRecallsDifferingOnlyByADateSuffixStopBuyingRoundsOnceTwoInARowAddedNothing(t *testing.T) {
	t.Parallel()

	graph := graphReturningTheSameRowsToEveryRecall()
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := dispatchedRecalls(graph); got != 3 {
		t.Fatalf("the turn dispatched %d supplementary recalls, want 3: one that brought rows and two that proved the graph had no more to give", got)
	}
	if record.ModelCalls != 5 {
		t.Fatalf("the turn spent %d model calls, want 5: the bound must cost fewer calls than the call cap it replaces", record.ModelCalls)
	}
	if !record.RecallClosed {
		t.Fatal("the record does not say recall was closed, so nothing downstream can tell this run apart from one that simply stopped asking")
	}
	if record.CapReached {
		t.Fatal("the record blames the call cap; the bound that fired was the closing of recall, and the record must name the one that did")
	}
	if want := []int{repeatedRowCount, 0, 0}; !sameInts(yieldsOf(record), want) {
		t.Fatalf("the dispatched rounds recorded yields %v, want %v: the first brought rows and the rest returned what was already in front of the model", yieldsOf(record), want)
	}
	if len(record.ToolCalls) != 4 {
		t.Fatalf("the record carries %d rounds, want 4: three dispatched and the refusal the model was shown; the call that followed was offered no tool and asked for nothing", len(record.ToolCalls))
	}
	if refused := record.ToolCalls[3]; refused.Error != errRecallClosedToModel {
		t.Fatalf("the refused round carries error %q, want the sentence the model was shown", refused.Error)
	}
	if record.Outcome.Verdict != VerdictCurtailed {
		t.Fatalf("record.Outcome = %+v, want a curtailed verdict: a tool-dispatch bound ended the turn while the model was still asking", record.Outcome)
	}
}

func TestTheRoundsThatAddedNothingAreShownToTheModelAsRowsItAlreadyHasRatherThanAsResults(t *testing.T) {
	t.Parallel()

	graph := graphReturningTheSameRowsToEveryRecall()
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	final := model.calls[len(model.calls)-1]
	if len(final.PriorTools) != 4 {
		t.Fatalf("the final call was shown %d rounds, want 4", len(final.PriorTools))
	}

	if first := RenderToolResult(final.PriorTools[0]); !strings.Contains(first, sectionResult) {
		t.Fatalf("the round that brought rows was not presented to the model:\n%s", first)
	}
	for i, round := range final.PriorTools[1:3] {
		rendered := RenderToolResult(round)
		if !strings.Contains(rendered, "already been shown") {
			t.Fatalf("round %d returned only rows the model already held and was still presented as fresh results:\n%s", i+2, rendered)
		}
	}
}

func TestARunWhoseEveryRoundBringsNewRowsIsNeverRefusedAndStillEndsAtTheCallCap(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := dispatchedRecalls(graph); got != MaxModelCalls-1 {
		t.Fatalf("the turn dispatched %d supplementary recalls, want %d: a round bringing rows nobody had seen must never be refused", got, MaxModelCalls-1)
	}
	if record.RecallClosed {
		t.Fatal("recall was closed on a run whose every round brought rows the model had not seen")
	}
	if !record.CapReached {
		t.Fatal("a run that kept finding new rows did not reach the call cap, so the fixture is not exercising the bound it claims to")
	}
	if want := []int{5, 5, 5, 5, 5}; !sameInts(yieldsOf(record), want) {
		t.Fatalf("the dispatched rounds recorded yields %v, want %v", yieldsOf(record), want)
	}

	final := model.calls[len(model.calls)-1]
	for i, round := range final.PriorTools {
		if rendered := RenderToolResult(round); strings.Contains(rendered, "already been shown") {
			t.Fatalf("round %d brought rows the model had not seen and was still refused as a repeat:\n%s", i+1, rendered)
		}
	}
}

func TestOneRoundThatBringsSomethingBetweenTwoBarrenOnesKeepsRecallOpen(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{{}, {Candidates: repeatedRows()}, {Candidates: repeatedRows()}, {Candidates: freshRows(3)}, {Candidates: freshRows(3)}, {Candidates: freshRows(5)}}
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.RecallClosed {
		t.Fatalf("recall closed on a run whose barren rounds were not consecutive; the dispatched yields were %v", yieldsOf(record))
	}
	if want := []int{repeatedRowCount, 0, repeatedRowCount, 0, repeatedRowCount}; !sameInts(yieldsOf(record), want) {
		t.Fatalf("the dispatched rounds recorded yields %v, want %v", yieldsOf(record), want)
	}
}

func TestTheAnswerTheFinalJudgementProducesAfterRecallClosesIsKept(t *testing.T) {
	t.Parallel()

	const answer = "what I can say from the rows I already have"

	graph := graphReturningTheSameRowsToEveryRecall()
	results := append(recallsDifferingOnlyByADateSuffix(4), JudgeResult{Answer: answer, Reason: Answered, RawReason: "stop"})
	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.Answer != answer {
		t.Fatalf("record.Answer = %q, want the answer the final judgement produced: closing recall must not throw away what the run had", record.Answer)
	}
	if !record.Outcome.Produced {
		t.Fatal("the outcome says the run produced nothing, though the record carries an answer")
	}
	if record.Outcome.Verdict != VerdictCurtailed {
		t.Fatalf("record.Outcome = %+v, want curtailed: a bound the loop imposed ended this turn", record.Outcome)
	}
}

func TestTheRecallClosedBoundMakesTheVerdictCurtailedAndRecomputesFromTheRecordAlone(t *testing.T) {
	t.Parallel()

	record := Record{
		Answer:     "an answer",
		Candidates: []Disposition{{Rank: 1, ID: 11, Included: true}},
	}
	if outcome := ComputeOutcome(record); outcome.Verdict != VerdictDelivered {
		t.Fatalf("the same record without the bound computes %+v, want delivered: the assertions below would otherwise pass on a record that never closed recall", outcome)
	}

	record.RecallClosed = true
	outcome := ComputeOutcome(record)

	if !outcome.Curtailed {
		t.Fatal("a run the loop stopped by closing recall is not curtailed, so the verdict cannot see the bound the loop itself imposed")
	}
	if outcome.Verdict != VerdictCurtailed {
		t.Fatalf("ComputeOutcome = %+v, want a curtailed verdict", outcome)
	}
}

func TestTheRunSummaryNamesTheRecallBoundBesideTheCapItDidNotReach(t *testing.T) {
	t.Parallel()

	record := Record{Answer: "an answer", Limits: Limits{MaxModelCalls: MaxModelCalls}, ModelCalls: 5}
	if without := RenderSummary(record, summaryInstant()); strings.Contains(without, "recall closed") {
		t.Fatalf("a run that did not close recall says it did:\n%s", without)
	}

	record.RecallClosed = true
	with := RenderSummary(record, summaryInstant())

	if !strings.Contains(with, "cap not reached, recall closed") {
		t.Fatalf("the summary reports the cap alone on a run a different bound ended, so an operator reads it as nothing having stopped the run:\n%s", with)
	}
}

func TestClosingRecallRaisesAnAlarmNamingTheBoundThatFired(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	turn := &Turn{logger: slog.New(slog.NewTextHandler(&logBuf, nil))}

	quiet := Record{Subject: 42, Answer: "an answer", Candidates: []Disposition{{Rank: 1, ID: 11, Included: true}}}
	quiet.Outcome = ComputeOutcome(quiet)
	turn.logFinished(quiet, WriteReceipt{}, 0)
	if strings.Contains(logBuf.String(), "recall was closed") {
		t.Fatalf("a run that never closed recall raised the alarm:\n%s", logBuf.String())
	}

	closed := quiet
	closed.RecallClosed = true
	closed.Outcome = ComputeOutcome(closed)
	turn.logFinished(closed, WriteReceipt{}, 0)

	if !strings.Contains(logBuf.String(), "recall was closed") {
		t.Fatalf("closing recall passed without an alarm; log:\n%s", logBuf.String())
	}
}

func TestEveryRoundTheLoopCalledBarrenIsBarrenUnderTheRecordOnlyReadingToo(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{{Candidates: repeatedRows()}, {Candidates: freshRows(1)}, {Candidates: repeatedRows()}, {Candidates: freshRows(3)}}
	results := append(recallsDifferingOnlyByADateSuffix(3), JudgeResult{Answer: "an answer", Reason: Answered, RawReason: "stop"})
	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	loopSaid := make([]bool, 0, len(record.ToolCalls))
	for _, yield := range yieldsOf(record) {
		loopSaid = append(loopSaid, yield == 0)
	}
	recordSaid := barrenRoundsFromTheRecordAlone(record)

	if want := []bool{false, true, false}; !sameBools(loopSaid, want) {
		t.Fatalf("the fixture produced barren pattern %v, want %v: without both kinds of round the comparison below cannot fail", loopSaid, want)
	}
	if !sameBools(loopSaid, recordSaid) {
		t.Fatalf("the loop called rounds barren %v while a reading of the record alone calls them %v; the mechanism and any instrument checking it would disagree about the same run", loopSaid, recordSaid)
	}
}

func barrenRoundsFromTheRecordAlone(record Record) []bool {
	visible := make(map[int64]bool)
	for _, d := range record.Candidates {
		if d.Included {
			visible[d.ID] = true
		}
	}

	barren := make([]bool, 0, len(record.ToolCalls))
	for _, call := range record.ToolCalls {
		if call.Tool != ToolRecall || call.Error != "" {
			continue
		}
		unseen := false
		for _, d := range call.Results {
			if !d.Included {
				continue
			}
			if !visible[d.ID] {
				unseen = true
			}
			visible[d.ID] = true
		}
		barren = append(barren, !unseen)
	}
	return barren
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameBools(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAToolWantedOnTheReservedCallAfterRecallClosedIsRecordedRatherThanDispatched(t *testing.T) {
	t.Parallel()

	queries := recallsDifferingOnlyByADateSuffix(MaxModelCalls)
	graph := graphReturningTheSameRowsToEveryRecall()
	model := &fakeModel{ignoresWithheldToolList: true, results: queries}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	pending := record.ToolCalls[len(record.ToolCalls)-1]
	if pending.Query != queries[record.ModelCalls-1].RecallQuery {
		t.Fatalf("the last round carries query %q, want the one the final judgement asked for, %q: a request the loop refused must not vanish from the record", pending.Query, queries[record.ModelCalls-1].RecallQuery)
	}
	if pending.Error != errReservedCallRefused {
		t.Fatalf("the request the loop never dispatched carries error %q, want the reservation that refused it named as the loop's own sentence", pending.Error)
	}
	if len(pending.Results) != 0 {
		t.Fatalf("the undispatched request carries %d results, want none: it never reached the graph", len(pending.Results))
	}
	if record.StopReason.Reason != WantsRecall {
		t.Fatalf("record.StopReason.Reason = %q, want WantsRecall: the run ended with the model still asking", record.StopReason.Reason)
	}
}

func TestAWriteRequestedAfterRecallClosedIsStillDispatched(t *testing.T) {
	t.Parallel()

	const page = "<html></html>"

	files := &fakeFiles{dir: "/runs/run-1"}
	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "what changed 2026-09-18"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "what changed 2026-09-19"},
		wantsWriteOf("index.html", page),
		answered("done"),
	}}
	turn := NewTurn(graph, model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(files.writes) != 1 {
		t.Fatalf("the working directory took %d writes, want 1: closing recall must not refuse an unrelated tool", len(files.writes))
	}
	written := record.ToolCalls[len(record.ToolCalls)-1]
	if written.Tool != ToolWriteFile {
		t.Fatalf("the last round was recorded as %q, want a write: the model asked to write and the loop answered about recall", written.Tool)
	}
	if written.Error != "" {
		t.Fatalf("the write round carries error %q, want none", written.Error)
	}
	if written.Bytes != len(page) {
		t.Fatalf("the write round recorded %d bytes, want %d", written.Bytes, len(page))
	}
	if record.RecallClosed {
		t.Fatal("a turn whose model asked to write after two barren recall rounds is recorded as having had recall closed, though it never asked to recall again")
	}
	if record.Outcome.Verdict == VerdictCurtailed {
		t.Fatalf("record.Outcome = %+v, want no tool-dispatch bound on a run that dispatched everything it asked for", record.Outcome)
	}
}

func TestTwoRecallRoundsThatFailedAtTransportDoNotCloseRecall(t *testing.T) {
	t.Parallel()

	unavailable := recallResponse{Err: errors.New("literal: 500 from graph")}
	graph := baseGraph()
	graph.recallQueue = []recallResponse{{}, unavailable, unavailable, unavailable}
	model := &fakeModel{results: append(recallsDifferingOnlyByADateSuffix(3), answered("done"))}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := dispatchedRecalls(graph); got != 3 {
		t.Fatalf("the turn dispatched %d supplementary recalls, want 3: a recall that failed at transport is not evidence the graph has nothing, so two flaky rounds must not close recall", got)
	}
	if record.RecallClosed {
		t.Fatal("recall was closed by rounds that never reached the graph, so a graph outage reads to the loop as an exhausted graph")
	}
	for i, call := range record.ToolCalls {
		if call.Error == "" {
			t.Fatalf("round %d was recorded without an error though its recall failed", i+1)
		}
	}
}

func TestAGraphWithNothingToGiveClosesRecallAfterTwoEmptyRoundsRatherThanRunningToTheCap(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.ModelCalls != 4 {
		t.Fatalf("the turn spent %d model calls, want 4: a graph returning nothing twice must not buy the rest of the call budget", record.ModelCalls)
	}
	if got := dispatchedRecalls(graph); got != 2 {
		t.Fatalf("the turn dispatched %d supplementary recalls, want 2", got)
	}
	if !record.RecallClosed {
		t.Fatal("a run whose recalls returned nothing twice does not say recall was closed")
	}
	if record.CapReached {
		t.Fatal("the record blames the call cap on a run that never reached it")
	}
	if want := []int{0, 0}; !sameInts(yieldsOf(record), want) {
		t.Fatalf("the dispatched rounds recorded yields %v, want %v", yieldsOf(record), want)
	}

	final := model.calls[len(model.calls)-1]
	if len(final.PriorTools) != 3 {
		t.Fatalf("the final call was shown %d rounds, want 3", len(final.PriorTools))
	}
	for i, round := range final.PriorTools[:2] {
		rendered := RenderToolResult(round)
		if !strings.Contains(rendered, "no additional results found") {
			t.Fatalf("round %d returned nothing at all and was not said to have:\n%s", i+1, rendered)
		}
		if strings.Contains(rendered, "already been shown") {
			t.Fatalf("round %d returned nothing at all and was described as rows the model already holds:\n%s", i+1, rendered)
		}
	}
	if closed := RenderToolResult(final.PriorTools[2]); !strings.Contains(closed, "recall is closed") {
		t.Fatalf("the third round did not tell the model recall is closed:\n%s", closed)
	}
}
