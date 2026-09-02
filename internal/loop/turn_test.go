package loop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeGraph is a GraphPort test double, justified by the deletion
// experiment in design §9.2 seam 1: without it, Turn's tests for the
// not-found branch, the two failure branches and the recall query would
// have to run against a live, shared graph.
type fakeGraph struct {
	node      Anchor
	nodeFound bool
	nodeErr   error
	onNode    func()

	candidates []Candidate
	recallErr  error

	recallQueue []recallResponse

	recallCalls []recallCall

	recallQuery string
	recallLimit int

	writeRunCalled  bool
	writeRunRecord  Record
	writeRunCtx     context.Context
	writeRunReceipt WriteReceipt
}

type recallCall struct {
	Query string
	Limit int
}

type recallResponse struct {
	Candidates []Candidate
	Err        error
}

func (f *fakeGraph) Node(_ context.Context, _ int64) (Anchor, bool, error) {
	if f.onNode != nil {
		f.onNode()
	}
	return f.node, f.nodeFound, f.nodeErr
}

func (f *fakeGraph) Recall(_ context.Context, query string, limit int) ([]Candidate, error) {
	f.recallQuery = query
	f.recallLimit = limit
	f.recallCalls = append(f.recallCalls, recallCall{Query: query, Limit: limit})

	if len(f.recallQueue) > 0 {
		resp := f.recallQueue[0]
		f.recallQueue = f.recallQueue[1:]
		return resp.Candidates, resp.Err
	}
	return f.candidates, f.recallErr
}

func (f *fakeGraph) WriteRun(ctx context.Context, record Record) WriteReceipt {
	f.writeRunCalled = true
	f.writeRunRecord = record
	f.writeRunCtx = ctx
	return f.writeRunReceipt
}

type fakeModel struct {
	results []JudgeResult
	err     error

	beforeReturn func()

	calls []JudgeInput
}

func (f *fakeModel) Judge(_ context.Context, in JudgeInput) (JudgeResult, error) {
	f.calls = append(f.calls, in)
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	if f.err != nil {
		return JudgeResult{}, f.err
	}
	if len(f.results) == 0 {
		return JudgeResult{Reason: Answered, RawReason: "stop"}, nil
	}
	idx := len(f.calls) - 1
	if idx >= len(f.results) {
		idx = len(f.results) - 1
	}
	return f.results[idx], nil
}

func newTurnWithGraph(graph GraphPort) *Turn {
	return NewTurn(graph, &fakeModel{}, "system text", "test-model", testLogger())
}

func TestTurnRunReturnsSubjectNotFound(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: false}
	turn := newTurnWithGraph(graph)

	_, _, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("Run() err = %v, want ErrSubjectNotFound", err)
	}
}

func TestTurnRunWrapsNodeTransportFailureAsGraphUnavailable(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeErr: errors.New("literal: connection refused")}
	turn := newTurnWithGraph(graph)

	_, _, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable", err)
	}
}

// TestTurnRunDoesNotRecallWhenTheAnchorIsNotFound pins design §6.5: an
// anchor failure means the run has no subject, and recall must not run.
func TestTurnRunDoesNotRecallWhenTheAnchorIsNotFound(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: false}
	turn := newTurnWithGraph(graph)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err == nil {
		t.Fatal("Run() returned nil error for a missing subject, want ErrSubjectNotFound")
	}
	if graph.recallQuery != "" || graph.recallLimit != 0 {
		t.Fatalf("Recall was called (query=%q limit=%d) after the anchor was not found, want no call", graph.recallQuery, graph.recallLimit)
	}
}

func TestTurnRunWrapsRecallFailureAsGraphUnavailable(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{
		node:      Anchor{ID: 42, Content: "anchor body"},
		nodeFound: true,
		recallErr: errors.New("literal: 500 from graph"),
	}
	turn := newTurnWithGraph(graph)

	_, _, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable", err)
	}
}

func TestTurnRunDoesNotCallModelWhenRecallFails(t *testing.T) {
	t.Parallel()

	model := &fakeModel{}
	graph := &fakeGraph{
		node:      Anchor{ID: 42, Content: "anchor body"},
		nodeFound: true,
		recallErr: errors.New("literal: 500 from graph"),
	}
	turn := NewTurn(graph, model, "system text", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "hello", 42); !errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable", err)
	}
	if len(model.calls) != 0 {
		t.Fatalf("model was called %d times after a recall failure, want 0", len(model.calls))
	}
	if graph.writeRunCalled {
		t.Fatal("WriteRun was called after a recall failure, want nothing written")
	}
}

// TestTurnRunPassesInputVerbatimAsTheRecallQuery pins design S3: no
// rewriting, no expansion, no model between the input and the query.
func TestTurnRunPassesInputVerbatimAsTheRecallQuery(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{node: Anchor{ID: 42}, nodeFound: true}
	turn := newTurnWithGraph(graph)

	// Deliberately not already lowercase/trimmed/single-spaced (CF-1): a
	// fixture that already satisfies those normalizations can't fail when
	// Run silently applies one, so this pins that none of them happen.
	const input = "  Why DOES   the Assembler ignore SCOPE?  "
	record, _, err := turn.Run(context.Background(), input, 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if graph.recallQuery != input {
		t.Fatalf("recall query = %q, want the input verbatim %q", graph.recallQuery, input)
	}
	if record.Query != input {
		t.Fatalf("record.Query = %q, want %q", record.Query, input)
	}
	if record.Input != input {
		t.Fatalf("record.Input = %q, want %q", record.Input, input)
	}
}

func TestTurnRunUsesTheCandidateLimitConstant(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{node: Anchor{ID: 42}, nodeFound: true}
	turn := newTurnWithGraph(graph)

	if _, _, err := turn.Run(context.Background(), "q", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if graph.recallLimit != CandidateLimit {
		t.Fatalf("recall limit = %d, want CandidateLimit (%d)", graph.recallLimit, CandidateLimit)
	}
}

func TestTurnRunSummarizesTheFetchedAnchorIntoTheRecord(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{
		node:      Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "body text"},
		nodeFound: true,
	}
	turn := newTurnWithGraph(graph)

	record, _, err := turn.Run(context.Background(), "q", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Subject != 42 {
		t.Fatalf("record.Subject = %d, want 42", record.Subject)
	}
	if record.Anchor.ID != 42 || record.Anchor.Type != "documentation" || record.Anchor.Name != "Subject" {
		t.Fatalf("record.Anchor = %+v, want the fetched anchor summarized", record.Anchor)
	}
	if record.Anchor.Size != len("body text") {
		t.Fatalf("record.Anchor.Size = %d, want %d", record.Anchor.Size, len("body text"))
	}
}

// TestTurnRunTwoTurnsDoNotShareState pins design §6.5's falsifier: two
// Turn values built over independent graphs must not observe each other.
func TestTurnRunTwoTurnsDoNotShareState(t *testing.T) {
	t.Parallel()

	turnA := newTurnWithGraph(&fakeGraph{node: Anchor{ID: 1, Content: "a"}, nodeFound: true})
	turnB := newTurnWithGraph(&fakeGraph{node: Anchor{ID: 2, Content: "b"}, nodeFound: true})

	recA, _, err := turnA.Run(context.Background(), "qa", 1)
	if err != nil {
		t.Fatalf("turnA.Run: %v", err)
	}
	recB, _, err := turnB.Run(context.Background(), "qb", 2)
	if err != nil {
		t.Fatalf("turnB.Run: %v", err)
	}

	if recA.Subject == recB.Subject {
		t.Fatalf("both turns reported subject %d, want them independent", recA.Subject)
	}
}

func baseGraph() *fakeGraph {
	return &fakeGraph{node: Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"}, nodeFound: true}
}

func TestTurnRunRecordsTheModelsAnswerAndStopsAtOneCallWhenAnswered(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "the system text", "test-model-id", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Answer != "the answer" {
		t.Fatalf("record.Answer = %q, want %q", record.Answer, "the answer")
	}
	if record.Model != "test-model-id" {
		t.Fatalf("record.Model = %q, want %q", record.Model, "test-model-id")
	}
	if record.ModelCalls != 1 {
		t.Fatalf("record.ModelCalls = %d, want 1", record.ModelCalls)
	}
	if record.StopReason.Reason != Answered || record.StopReason.Raw != "stop" {
		t.Fatalf("record.StopReason = %+v, want {Answered stop}", record.StopReason)
	}
	if record.ToolCalls == nil || len(record.ToolCalls) != 0 {
		t.Fatalf("record.ToolCalls = %#v, want a non-nil empty slice", record.ToolCalls)
	}
	if record.CapReached {
		t.Fatal("record.CapReached = true, want false — the model answered on the first call, the cap never fired")
	}
	wantLimits := Limits{CandidateLimit: 20, AssemblyByteBudget: 60_000, SupplementaryByteBudget: 20_000, MaxModelCalls: 3, MaxOutputTokens: 4_096}
	if record.Limits != wantLimits {
		t.Fatalf("record.Limits = %+v, want %+v", record.Limits, wantLimits)
	}
	if len(model.calls) != 1 {
		t.Fatalf("model was called %d times, want 1", len(model.calls))
	}
	if model.calls[0].System != "the system text" {
		t.Fatalf("Judge System = %q, want the configured system text", model.calls[0].System)
	}
	if model.calls[0].Input != "hello" {
		t.Fatalf("Judge Input = %q, want %q", model.calls[0].Input, "hello")
	}
	if model.calls[0].Block != record.Block {
		t.Fatal("Judge Block did not match the record's assembled block")
	}
}

func TestTurnRunDispatchesRecallAndJudgesAgain(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{{ID: 99, Type: "task", Name: "Found", Similarity: 0.8, Content: "tool result body"}}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "the missing thing"},
		{Answer: "final answer", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.ModelCalls != 2 {
		t.Fatalf("record.ModelCalls = %d, want 2", record.ModelCalls)
	}
	if record.Answer != "final answer" {
		t.Fatalf("record.Answer = %q, want %q", record.Answer, "final answer")
	}
	if len(graph.recallCalls) != 2 {
		t.Fatalf("Recall was called %d times, want 2 (initial + one supplementary)", len(graph.recallCalls))
	}
	if graph.recallCalls[1].Query != "the missing thing" {
		t.Fatalf("supplementary recall query = %q, want %q", graph.recallCalls[1].Query, "the missing thing")
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Query != "the missing thing" || record.ToolCalls[0].Error != "" {
		t.Fatalf("record.ToolCalls[0] = %+v, want a successful round for %q", record.ToolCalls[0], "the missing thing")
	}
	if len(record.ToolCalls[0].Results) != 1 || record.ToolCalls[0].Results[0].ID != 99 || record.ToolCalls[0].Results[0].Similarity != 0.8 {
		t.Fatalf("record.ToolCalls[0].Results = %+v, want [{99 0.8}]", record.ToolCalls[0].Results)
	}
	if !record.ToolCalls[0].Results[0].Included {
		t.Fatal("record.ToolCalls[0].Results[0].Included = false, want true — the hit is well under SupplementaryByteBudget")
	}
	if record.CapReached {
		t.Fatal("record.CapReached = true, want false — the model answered on the second call, the cap never fired")
	}
	if len(model.calls[1].PriorRecalls) != 1 || model.calls[1].PriorRecalls[0].Query != "the missing thing" {
		t.Fatalf("second Judge call's PriorRecalls = %+v, want the completed round", model.calls[1].PriorRecalls)
	}
}

func TestTurnRunRecordsAMalformedToolRequestAsAnErrorFlaggedRoundAndContinues(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallError: "tool arguments could not be parsed"},
		{Answer: "answered anyway", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.ModelCalls != 2 {
		t.Fatalf("record.ModelCalls = %d, want 2 (the turn continues past the malformed round)", record.ModelCalls)
	}
	if record.Answer != "answered anyway" {
		t.Fatalf("record.Answer = %q, want the turn to reach the second call's answer", record.Answer)
	}
	if len(graph.recallCalls) != 1 {
		t.Fatalf("Recall was called %d times, want 1 (only the initial call — the malformed round must not reach Recall)", len(graph.recallCalls))
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1 (counted, not dropped)", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Error != "tool arguments could not be parsed" {
		t.Fatalf("record.ToolCalls[0].Error = %q, want the malformed reason", record.ToolCalls[0].Error)
	}
	if record.ToolCalls[0].Query != "" {
		t.Fatalf("record.ToolCalls[0].Query = %q, want empty for a malformed request", record.ToolCalls[0].Query)
	}
	if record.ToolCalls[0].Results == nil || len(record.ToolCalls[0].Results) != 0 {
		t.Fatalf("record.ToolCalls[0].Results = %#v, want a non-nil empty slice", record.ToolCalls[0].Results)
	}
}

func TestTurnRunRecordsASupplementaryRecallTransportFailureAsAnErrorFlaggedRound(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Err: errors.New("literal: 500 from graph")},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "answered anyway", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error == "" {
		t.Fatalf("record.ToolCalls = %+v, want one error-flagged round", record.ToolCalls)
	}
	if record.Answer != "answered anyway" {
		t.Fatal("a supplementary recall failure aborted the turn instead of continuing to the next judgement")
	}
}

func TestTurnRunStopsAtTheModelCallCapWithoutDispatchingAFinalRecall(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q1"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q2"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q3"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	const (
		wantModelCallCap      = 3
		wantInitialRecalls    = 1
		wantDispatchedRecalls = wantModelCallCap - 1
	)
	if record.ModelCalls != wantModelCallCap {
		t.Fatalf("record.ModelCalls = %d, want %d", record.ModelCalls, wantModelCallCap)
	}
	if len(model.calls) != wantModelCallCap {
		t.Fatalf("model was called %d times, want %d", len(model.calls), wantModelCallCap)
	}
	wantRecallCalls := wantInitialRecalls + wantDispatchedRecalls
	if len(graph.recallCalls) != wantRecallCalls {
		t.Fatalf("Recall was called %d times, want %d", len(graph.recallCalls), wantRecallCalls)
	}
	if record.StopReason.Reason != WantsRecall {
		t.Fatalf("record.StopReason.Reason = %q, want WantsRecall (the cap fired mid-request)", record.StopReason.Reason)
	}
	if !record.CapReached {
		t.Fatal("record.CapReached = false, want true — the cap fired while the model still wanted recall")
	}
	if len(record.ToolCalls) != wantModelCallCap {
		t.Fatalf("record.ToolCalls has %d entries, want %d (the cap-reached round counted too)", len(record.ToolCalls), wantModelCallCap)
	}
	last := record.ToolCalls[len(record.ToolCalls)-1]
	if last.Query != "q3" {
		t.Fatalf("record.ToolCalls[last].Query = %q, want %q — the model's final query must not be discarded", last.Query, "q3")
	}
	if len(last.Results) != 0 {
		t.Fatalf("record.ToolCalls[last].Results = %+v, want empty — the round was never dispatched", last.Results)
	}
}

func TestTurnRunRecordsTheFinalRecallQueryEvenWhenTheCapPreventsDispatch(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q1"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q2"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "the query that was never dispatched"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 3 {
		t.Fatalf("record.ToolCalls has %d entries, want 3 (2 dispatched + the cap-reached round)", len(record.ToolCalls))
	}
	last := record.ToolCalls[2]
	if last.Query != "the query that was never dispatched" {
		t.Fatalf("record.ToolCalls[2].Query = %q, want the model's final query preserved", last.Query)
	}
	if last.Error == "" {
		t.Fatal("record.ToolCalls[2].Error is empty, want the cap-reached round flagged")
	}
	if len(last.Results) != 0 {
		t.Fatalf("record.ToolCalls[2].Results = %+v, want empty — the round was never dispatched", last.Results)
	}
	if len(graph.recallCalls) != 3 {
		t.Fatalf("Recall was called %d times, want 3 (1 initial + 2 dispatched) — the final round must still not be dispatched", len(graph.recallCalls))
	}
}

func TestTurnRunDoesNotLeakTheGraphErrorDetailIntoTheSupplementaryRecallRound(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Err: errors.New("literal: dial tcp 10.0.0.55:443: connect: connection refused")},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "answered anyway", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if strings.Contains(record.ToolCalls[0].Error, "10.0.0.55") {
		t.Fatalf("record.ToolCalls[0].Error = %q, want no internal address disclosed", record.ToolCalls[0].Error)
	}
	if record.ToolCalls[0].Error == "" {
		t.Fatal("record.ToolCalls[0].Error is empty, want the failure still flagged")
	}
}

func TestTurnRunLogsTheDetailedRecallErrorWhileTheRecordStaysGeneric(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Err: errors.New("literal: dial tcp 10.0.0.55:443: connect: connection refused")},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "answered anyway", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", logger)

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	const wantGenericRecallError = "supplementary recall failed"
	if record.ToolCalls[0].Error != wantGenericRecallError {
		t.Fatalf("record.ToolCalls[0].Error = %q, want the generic sentence %q", record.ToolCalls[0].Error, wantGenericRecallError)
	}
	if !strings.Contains(logBuf.String(), "10.0.0.55") {
		t.Fatalf("operator log = %q, want it to carry the detailed error (including the address) that the record and prompt must not", logBuf.String())
	}
}

func TestTurnRunPreservesBothTheMappedAndRawStopReason(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{{Reason: Unrecognised, RawReason: "some-vendor-string"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.StopReason.Reason != Unrecognised {
		t.Fatalf("record.StopReason.Reason = %q, want Unrecognised", record.StopReason.Reason)
	}
	if record.StopReason.Raw != "some-vendor-string" {
		t.Fatalf("record.StopReason.Raw = %q, want the endpoint's own string preserved", record.StopReason.Raw)
	}
}

func TestTurnRunLeavesUsageAbsentWhenTheModelReportedNone(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{{Reason: Answered, RawReason: "stop", Usage: nil}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.Usage) != 1 {
		t.Fatalf("record.Usage = %+v, want one entry (one model call)", record.Usage)
	}
	if record.Usage[0] != nil {
		t.Fatalf("record.Usage[0] = %+v, want nil (absent)", record.Usage[0])
	}
}

func TestTurnRunCarriesUsageWhenTheModelReportedIt(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	usage := &Usage{InTokens: 100, OutTokens: 20}
	model := &fakeModel{results: []JudgeResult{{Reason: Answered, RawReason: "stop", Usage: usage}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.Usage) != 1 || record.Usage[0] == nil || *record.Usage[0] != *usage {
		t.Fatalf("record.Usage = %+v, want [%+v]", record.Usage, usage)
	}
}

func TestTurnRunUsageArrayLengthAlwaysEqualsModelCalls(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{{ID: 2, Content: "tool result"}}},
	}
	usage1 := &Usage{InTokens: 10, OutTokens: 1}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q", Usage: usage1},
		{Answer: "final", Reason: Answered, RawReason: "stop", Usage: nil},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.Usage) != record.ModelCalls {
		t.Fatalf("len(record.Usage) = %d, record.ModelCalls = %d, want them equal", len(record.Usage), record.ModelCalls)
	}
	if record.Usage[0] == nil || *record.Usage[0] != *usage1 {
		t.Fatalf("record.Usage[0] = %+v, want %+v (the first call's reported usage)", record.Usage[0], usage1)
	}
	if record.Usage[1] != nil {
		t.Fatalf("record.Usage[1] = %+v, want nil — the second call reported none, and the first call's usage must not be summed or repeated into it", record.Usage[1])
	}
}

func TestTurnRunWrapsModelFailureAsModelUnavailableAndWritesNothing(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{err: errors.New("literal: connection reset")}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	_, _, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("Run() err = %v, want ErrModelUnavailable", err)
	}
	if graph.writeRunCalled {
		t.Fatal("WriteRun was called after a model failure, want nothing written")
	}
}

func TestTurnRunWritesTheRecordAndReportsTheReceiptTheAdapterReturned(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.writeRunReceipt = WriteReceipt{State: Stored, NodeID: 999}
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	_, receipt, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !graph.writeRunCalled {
		t.Fatal("WriteRun was not called")
	}
	if receipt != (WriteReceipt{State: Stored, NodeID: 999}) {
		t.Fatalf("receipt = %+v, want {stored 999} verbatim from the adapter", receipt)
	}
	if graph.writeRunRecord.Answer != "ok" || graph.writeRunRecord.Subject != 42 {
		t.Fatalf("the record handed to WriteRun = %+v, want the completed record", graph.writeRunRecord)
	}
}

func TestTurnRunReportsEachWriteStateVerbatimAndInterpretsNone(t *testing.T) {
	t.Parallel()

	cases := []WriteReceipt{
		{State: Stored, NodeID: 999},
		{State: Unlinked, NodeID: 999},
		{State: NotStored},
	}

	for _, want := range cases {
		t.Run(string(want.State), func(t *testing.T) {
			t.Parallel()

			graph := baseGraph()
			graph.writeRunReceipt = want
			model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
			turn := NewTurn(graph, model, "system", "test-model", testLogger())

			_, receipt, err := turn.Run(context.Background(), "hello", 42)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if receipt != want {
				t.Fatalf("receipt = %+v, want %+v", receipt, want)
			}
		})
	}
}

func TestTurnRunRecordSerialisesWithNoKeyAboutItsOwnFiling(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.writeRunReceipt = WriteReceipt{State: Stored, NodeID: 4242}
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	for _, absent := range []string{`"written"`, `"state"`, `"nodeId"`, "4242"} {
		if strings.Contains(string(body), absent) {
			t.Fatalf("record JSON contains %q, want a record that says nothing about its own filing; body=%s", absent, body)
		}
	}
	if !strings.Contains(string(body), `"answer":"ok"`) {
		t.Fatalf("record JSON = %s, want it to still carry the turn's own keys", body)
	}
}

func TestTurnRunFilesTheRecordAfterTheRequestContextIsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	graph := baseGraph()
	graph.writeRunReceipt = WriteReceipt{State: Stored, NodeID: 999}
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	model.beforeReturn = cancel
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	_, receipt, err := turn.Run(ctx, "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !graph.writeRunCalled {
		t.Fatal("WriteRun was not called after the caller went away, want the record filed anyway")
	}
	if graph.writeRunCtx.Err() != nil {
		t.Fatalf("WriteRun was handed a context reporting %v, want one detached from the request's cancellation", graph.writeRunCtx.Err())
	}
	if receipt.State != Stored {
		t.Fatalf("receipt.State = %q, want %q", receipt.State, Stored)
	}
	if ctx.Err() == nil {
		t.Fatal("the request context was never cancelled, so this run never exercised the detachment")
	}
}

func TestTurnRunDoesNotFailTheRunWhenNoNodeHoldsTheRecord(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.writeRunReceipt = WriteReceipt{State: NotStored}
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, receipt, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run() returned an error for an unfiled record, want nil error and the fate named in the receipt: %v", err)
	}
	if receipt.State != NotStored {
		t.Fatalf("receipt.State = %q, want %q", receipt.State, NotStored)
	}
	if receipt.NodeID != 0 {
		t.Fatalf("receipt.NodeID = %d, want 0 — no node holds the record", receipt.NodeID)
	}
	if record.Answer != "ok" {
		t.Fatalf("record.Answer = %q, want the answer still returned to the caller", record.Answer)
	}
}

func TestTurnRunAdmitsSupplementaryHitsByRankOrderAndCutsTheRest(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{
			{ID: 91, Similarity: 0.9, Content: strings.Repeat("a", 9_000)},
			{ID: 92, Similarity: 0.8, Content: strings.Repeat("b", 9_000)},
			{ID: 93, Similarity: 0.7, Content: strings.Repeat("c", 5_000)},
			{ID: 94, Similarity: 0.6, Content: strings.Repeat("d", 100)},
		}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	round := record.ToolCalls[0]
	if len(round.Results) != 4 {
		t.Fatalf("round.Results has %d entries, want 4 — every row the round returned, admitted or cut", len(round.Results))
	}
	wantIncluded := []bool{true, true, false, false}
	wantOrder := []int64{91, 92, 93, 94}
	for i, want := range wantIncluded {
		if round.Results[i].ID != wantOrder[i] {
			t.Fatalf("round.Results[%d].ID = %d, want %d — position stays rank order, not id order (design §6.4a)", i, round.Results[i].ID, wantOrder[i])
		}
		if round.Results[i].Included != want {
			t.Fatalf("round.Results[%d] (id %d) Included = %v, want %v", i, wantOrder[i], round.Results[i].Included, want)
		}
	}
	if round.Results[3].Included {
		t.Fatal("rank 4 (smaller than the leftover budget) was back-filled; admission must stop, not skip, exactly like the block (design §6.4a)")
	}
	if round.Results[2].CutReason == "" || round.Results[3].CutReason == "" {
		t.Fatal("a cut supplementary hit has an empty CutReason, want it recorded")
	}
	if round.Results[2].Size != 5_000 {
		t.Fatalf("round.Results[2].Size = %d, want 5000 — a cut row must still carry the size that caused the cut", round.Results[2].Size)
	}

	seen := model.calls[1].PriorRecalls[0].Results
	if len(seen) != 2 || seen[0].ID != 91 || seen[1].ID != 92 {
		t.Fatalf("PriorRecalls[0].Results = %+v, want the two admitted candidates [91 92], in rank order", seen)
	}
}

func TestTurnRunSupplementaryAdmissionStaysInRankOrderEvenWhenIDsDescend(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{
			{ID: 300, Similarity: 0.9, Content: "third-ranked-by-id-descending"},
			{ID: 200, Similarity: 0.8, Content: "second"},
			{ID: 100, Similarity: 0.7, Content: "first"},
		}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	round := record.ToolCalls[0]
	wantOrder := []int64{300, 200, 100}
	if len(round.Results) != 3 {
		t.Fatalf("round.Results has %d entries, want 3", len(round.Results))
	}
	for i, id := range wantOrder {
		if round.Results[i].ID != id {
			t.Fatalf("round.Results[%d].ID = %d, want %d — an id-sort here (like the block's) would produce [100 200 300], not rank order", i, round.Results[i].ID, id)
		}
		if !round.Results[i].Included {
			t.Fatalf("round.Results[%d] (id %d) Included = false, want true — all three fit comfortably under the round budget", i, id)
		}
	}

	seen := model.calls[1].PriorRecalls[0].Results
	if len(seen) != 3 || seen[0].ID != 300 || seen[1].ID != 200 || seen[2].ID != 100 {
		t.Fatalf("PriorRecalls[0].Results = %+v, want [300 200 100] in rank order, not resorted by id", seen)
	}
}

func TestTurnRunASupplementaryRoundAdmittingNothingIsNotAnErrorAndRecordsEveryRowCut(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{
			{ID: 91, Content: strings.Repeat("a", 25_000)},
		}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Answer != "final" {
		t.Fatal("a round admitting nothing must not be treated as an error — the turn must still reach the model's answer")
	}
	round := record.ToolCalls[0]
	if round.Error != "" {
		t.Fatalf("round.Error = %q, want empty — an oversized hit is not a transport failure", round.Error)
	}
	if len(round.Results) != 1 || round.Results[0].Included {
		t.Fatalf("round.Results = %+v, want the one row present and cut, not admitted despite being the single best hit", round.Results)
	}
	if round.Results[0].CutReason == "" {
		t.Fatal("round.Results[0].CutReason is empty, want the budget cut recorded")
	}

	seen := model.calls[1].PriorRecalls[0].Results
	if len(seen) != 0 {
		t.Fatalf("PriorRecalls[0].Results has %d entries, want 0 — nothing was admitted", len(seen))
	}
}

func TestTurnRunAdmitsASupplementaryHitExactlyAtTheRoundBudget(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{{ID: 91, Content: strings.Repeat("a", SupplementaryByteBudget)}}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	round := record.ToolCalls[0]
	if len(round.Results) != 1 || !round.Results[0].Included {
		t.Fatalf("round.Results = %+v, want the hit exactly at SupplementaryByteBudget included", round.Results)
	}
}

func TestTheWorstCaseGraphDerivedPromptCeilingIsOneHundredThousandBytes(t *testing.T) {
	t.Parallel()

	const wantCeiling = 100_000
	gotCeiling := AssemblyByteBudget + SupplementaryByteBudget*(MaxModelCalls-1)
	if gotCeiling != wantCeiling {
		t.Fatalf("AssemblyByteBudget + SupplementaryByteBudget*(MaxModelCalls-1) = %d, want %d (design §8.4's stated ceiling)", gotCeiling, wantCeiling)
	}
}

func TestTurnRunLogsRunStartedBeforeTheAnchorRead(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	atNode := ""
	graph.onNode = func() { atNode = logBuf.String() }
	turn := NewTurn(graph, &fakeModel{}, "system", "test-model", logger)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(atNode, `msg="run started"`) {
		t.Fatalf("the log at the moment of the anchor read was %q, want the run-started record already in it", atNode)
	}
}

func TestTurnRunStartedRecordCarriesTheInputLengthAndNeverTheInputText(t *testing.T) {
	t.Parallel()

	const secretInput = "INPUT-TEXT-MARKER what the caller actually asked"

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	turn := NewTurn(baseGraph(), &fakeModel{}, "system", "test-model", logger)

	if _, _, err := turn.Run(context.Background(), secretInput, 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logBuf.String()
	if strings.Contains(out, "INPUT-TEXT-MARKER") {
		t.Fatalf("operator log carries the input text; log:\n%s", out)
	}
	if !strings.Contains(out, "inputLength=48") {
		t.Fatalf("operator log does not carry inputLength=48 for a %d-byte input; log:\n%s", len(secretInput), out)
	}
	if !strings.Contains(out, "subject=42") {
		t.Fatalf("run-started record does not carry the subject id; log:\n%s", out)
	}
}

func TestTurnRunFinishedRecordCarriesTheReceiptCountsAndWallClock(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	graph.candidates = []Candidate{
		{ID: 7, Type: "task", Name: "Small", Similarity: 0.9, Content: "small body"},
		{ID: 8, Type: "task", Name: "Large", Similarity: 0.8, Content: strings.Repeat("x", AssemblyByteBudget+1)},
		{ID: 9, Type: "task", Name: "AlsoLarge", Similarity: 0.7, Content: strings.Repeat("y", AssemblyByteBudget+1)},
	}
	graph.writeRunReceipt = WriteReceipt{State: Stored, NodeID: 999}
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop", Usage: &Usage{InTokens: 11, OutTokens: 22}}}}
	turn := NewTurn(graph, model, "system", "test-model-id", logger)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logBuf.String()
	for _, want := range []string{
		`msg="run finished"`,
		"receipt=stored",
		"node=999",
		"candidates=3",
		"cut=2",
		"modelCalls=1",
		"model=test-model-id",
		"usageReports=1",
		"inTokens=11",
		"outTokens=22",
		"elapsed=",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("run-finished record does not carry %q; log:\n%s", want, out)
		}
	}
}

func TestTurnRunFinishedRecordNamesANodeOnlyWhenOneExists(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	graph.writeRunReceipt = WriteReceipt{State: NotStored}
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "system", "test-model", logger)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logBuf.String()
	if !strings.Contains(out, "receipt=notStored") {
		t.Fatalf("run-finished record does not carry receipt=notStored; log:\n%s", out)
	}
	if strings.Contains(out, "node=") {
		t.Fatalf("run-finished record names a node although none holds the record; log:\n%s", out)
	}
}

func TestTurnRunFinishedRecordSumsUsageAcrossCallsAndCountsTheCallsThatReportedIt(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	graph.candidates = []Candidate{{ID: 7, Type: "task", Name: "C", Similarity: 0.9, Content: "body"}}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "more"},
		{Answer: "ok", Reason: Answered, RawReason: "stop", Usage: &Usage{InTokens: 100, OutTokens: 7}},
	}}
	turn := NewTurn(graph, model, "system", "test-model", logger)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logBuf.String()
	for _, want := range []string{"modelCalls=2", "usageReports=1", "inTokens=100", "outTokens=7"} {
		if !strings.Contains(out, want) {
			t.Fatalf("run-finished record does not carry %q; log:\n%s", want, out)
		}
	}
}

func TestTurnRunLogsNoRunFinishedWhenTheRunNeverReachedAnAnswer(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	model := &fakeModel{err: errors.New("literal: connection reset")}
	turn := NewTurn(graph, model, "system", "test-model", logger)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err == nil {
		t.Fatal("Run returned no error although the model call failed")
	}

	out := logBuf.String()
	if !strings.Contains(out, `msg="run started"`) {
		t.Fatalf("operator log has no run-started record; log:\n%s", out)
	}
	if strings.Contains(out, `msg="run finished"`) {
		t.Fatalf("operator log carries a run-finished record for a run that never reached an answer; log:\n%s", out)
	}
}
