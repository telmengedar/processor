package loop

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// testLogger is a *slog.Logger that discards everything — the right
// default for tests that are not themselves about logging (design §6.4a's
// operator-log requirement is pinned separately, against a logger that
// captures instead of discards).
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

	// candidates/recallErr answer every Recall call that has no queued
	// response waiting — the common case of one relevant call per test.
	candidates []Candidate
	recallErr  error

	// recallQueue, when non-empty, is consumed one entry per Recall call,
	// in call order — for tests where the initial recall and a
	// supplementary recall must answer differently.
	recallQueue []recallResponse

	recallCalls []recallCall // every Recall call, in order

	// recallQuery/recallLimit mirror the most recent Recall call — kept
	// for the existing single-call tests below that read them directly.
	recallQuery string
	recallLimit int

	writeRunCalled bool
	writeRunRecord Record
	writeRunNodeID int64
	writeRunErr    error
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

func (f *fakeGraph) WriteRun(_ context.Context, record Record) (int64, error) {
	f.writeRunCalled = true
	f.writeRunRecord = record
	return f.writeRunNodeID, f.writeRunErr
}

// fakeModel is a ModelPort test double, justified the same way as
// fakeGraph (design §9.2 seam 1, re-run under the ruling): it lets the
// loop's dispatch, cap and write-back logic be tested offline without a
// live, nondeterministic model.
type fakeModel struct {
	// results is consumed one per Judge call, in order; once exhausted the
	// last entry repeats, so a single-entry fixture is enough for the
	// common one-call case.
	results []JudgeResult
	err     error

	calls []JudgeInput // every call's input, in order
}

func (f *fakeModel) Judge(_ context.Context, in JudgeInput) (JudgeResult, error) {
	f.calls = append(f.calls, in)
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

// newTurnWithGraph builds a Turn over graph with a model double that
// answers immediately with no tool use — the right default for tests that
// are about assembly or the graph, not the model.
func newTurnWithGraph(graph GraphPort) *Turn {
	return NewTurn(graph, &fakeModel{}, "system text", "test-model", testLogger())
}

func TestTurnRunReturnsSubjectNotFound(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: false}
	turn := newTurnWithGraph(graph)

	_, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("Run() err = %v, want ErrSubjectNotFound", err)
	}
}

func TestTurnRunWrapsNodeTransportFailureAsGraphUnavailable(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeErr: errors.New("literal: connection refused")}
	turn := newTurnWithGraph(graph)

	_, err := turn.Run(context.Background(), "hello", 42)
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

	if _, err := turn.Run(context.Background(), "hello", 42); err == nil {
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

	_, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable", err)
	}
}

// TestTurnRunDoesNotCallModelWhenRecallFails pins design §6.5's own
// standout row: "Recall fails | 502, no model call, nothing written" —
// "assembly failure is run failure", so a model call on an empty context
// (the confident-answer-from-nothing failure this project exists to
// prevent) must never happen.
func TestTurnRunDoesNotCallModelWhenRecallFails(t *testing.T) {
	t.Parallel()

	model := &fakeModel{}
	graph := &fakeGraph{
		node:      Anchor{ID: 42, Content: "anchor body"},
		nodeFound: true,
		recallErr: errors.New("literal: 500 from graph"),
	}
	turn := NewTurn(graph, model, "system text", "test-model", testLogger())

	if _, err := turn.Run(context.Background(), "hello", 42); !errors.Is(err, ErrGraphUnavailable) {
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
	record, err := turn.Run(context.Background(), input, 42)
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

	if _, err := turn.Run(context.Background(), "q", 42); err != nil {
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

	record, err := turn.Run(context.Background(), "q", 42)
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

	recA, err := turnA.Run(context.Background(), "qa", 1)
	if err != nil {
		t.Fatalf("turnA.Run: %v", err)
	}
	recB, err := turnB.Run(context.Background(), "qb", 2)
	if err != nil {
		t.Fatalf("turnB.Run: %v", err)
	}

	if recA.Subject == recB.Subject {
		t.Fatalf("both turns reported subject %d, want them independent", recA.Subject)
	}
}

// --- unit B: the model call and write-back ---

func baseGraph() *fakeGraph {
	return &fakeGraph{node: Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"}, nodeFound: true}
}

// TestTurnRunRecordsTheModelsAnswerAndStopsAtOneCallWhenAnswered pins the
// ordinary path: a model that answers immediately ends the turn in one
// call, and every unit-B field the design assigns to that step is
// populated.
func TestTurnRunRecordsTheModelsAnswerAndStopsAtOneCallWhenAnswered(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "the system text", "test-model-id", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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
	// The record's Limits field carries the five constants that governed
	// this run (design §8.2, revision 3 from #10821 CF-4) — literals on the
	// expected side (design §14 step 1), not the package's own constants,
	// so a rename or a value change on either side of the assertion cannot
	// move both sides together.
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

// TestTurnRunDispatchesRecallAndJudgesAgain pins the tool cycle (design
// §6.4): a WantsRecall result is dispatched through Graph.Recall, the
// result is handed back on the next Judge call, and the turn ends when
// the model then answers.
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

	record, err := turn.Run(context.Background(), "hello", 42)
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
	// W-7: toolCalls[].results now carries the same columns candidates[]
	// does — a small well-under-budget hit must be admitted and reported so.
	if !record.ToolCalls[0].Results[0].Included {
		t.Fatal("record.ToolCalls[0].Results[0].Included = false, want true — the hit is well under SupplementaryByteBudget")
	}
	if record.CapReached {
		t.Fatal("record.CapReached = true, want false — the model answered on the second call, the cap never fired")
	}
	// The second Judge call must see the completed round.
	if len(model.calls[1].PriorRecalls) != 1 || model.calls[1].PriorRecalls[0].Query != "the missing thing" {
		t.Fatalf("second Judge call's PriorRecalls = %+v, want the completed round", model.calls[1].PriorRecalls)
	}
}

// TestTurnRunRecordsAMalformedToolRequestAsAnErrorFlaggedRoundAndContinues
// pins design §6.4: "A malformed tool input returns an error-flagged tool
// result and the turn continues; it is counted, not dropped." The
// malformed round must not reach Graph.Recall at all.
func TestTurnRunRecordsAMalformedToolRequestAsAnErrorFlaggedRoundAndContinues(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallError: "tool arguments could not be parsed"},
		{Answer: "answered anyway", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunRecordsASupplementaryRecallTransportFailureAsAnErrorFlaggedRound
// covers the case design §6.4 does not name explicitly by number but whose
// shape it establishes: a well-formed tool request whose recall itself
// fails must not abort a turn that has already produced a judgement — it
// is recorded the same way a malformed request is (an autonomous decision;
// see the implementation notes).
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

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunStopsAtTheModelCallCapWithoutDispatchingAFinalRecall pins
// design §6.4/§8.4: reaching MaxModelCalls while the model still wants
// recall is not an error — the turn ends, and record.modelCalls ==
// MaxModelCalls is how the record shows the cap fired.
func TestTurnRunStopsAtTheModelCallCapWithoutDispatchingAFinalRecall(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q1"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q2"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q3"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// A literal, not MaxModelCalls (design §14 step 1; CF-2): asserting
	// the production constant against itself on both sides means the
	// test passes for any cap >= 2 — TestDefaultTimeoutIsNotDivoidsTimeout
	// is the model for this discipline (I-3), applied here.
	const wantModelCallCap = 3
	if record.ModelCalls != wantModelCallCap {
		t.Fatalf("record.ModelCalls = %d, want %d", record.ModelCalls, wantModelCallCap)
	}
	if len(model.calls) != wantModelCallCap {
		t.Fatalf("model was called %d times, want %d", len(model.calls), wantModelCallCap)
	}
	// One initial recall plus (cap-1) dispatched supplementary rounds —
	// the recall requested by the *last* call is never dispatched.
	wantRecallCalls := wantModelCallCap // 1 initial + (cap-1) supplementary
	if len(graph.recallCalls) != wantRecallCalls {
		t.Fatalf("Recall was called %d times, want %d", len(graph.recallCalls), wantRecallCalls)
	}
	if record.StopReason.Reason != WantsRecall {
		t.Fatalf("record.StopReason.Reason = %q, want WantsRecall (the cap fired mid-request)", record.StopReason.Reason)
	}
	// W-7: capReached is its own explicit field, not left for a reader to
	// derive from ModelCalls == MaxModelCalls.
	if !record.CapReached {
		t.Fatal("record.CapReached = false, want true — the cap fired while the model still wanted recall")
	}
	// CF-3: the model's final query is still recorded even though it was
	// never dispatched — counted, not dropped (design §2.4, §6.4).
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

// TestTurnRunRecordsTheFinalRecallQueryEvenWhenTheCapPreventsDispatch pins
// CF-3 directly (design §2.4, §6.4): the recall query the model asked for
// on the call that hits the cap is still recorded as a round — counted,
// not dropped — even though it is never dispatched through Graph.Recall.
// Before the fix, the loop broke before appending this round at all, so
// the query the model asked hardest for (the one that hit the cap) was
// the one datum silently discarded.
func TestTurnRunRecordsTheFinalRecallQueryEvenWhenTheCapPreventsDispatch(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q1"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q2"},
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "the query that was never dispatched"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunDoesNotLeakTheGraphErrorDetailIntoTheSupplementaryRecallRound
// pins W-6: a Graph.Recall transport failure during the tool cycle must
// not echo the underlying error's text — which can embed the graph's
// request URL — into the record or the next model prompt. Not a
// credential leak (the DiVoid key travels as a header, never a query
// parameter), but still an internal address that should not propagate to
// a model prompt or a written record.
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

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunLogsTheDetailedRecallErrorWhileTheRecordStaysGeneric pins
// #10521's open finding: the W-6 fix made the detailed transport error
// disappear everywhere, because turn.go had no logger at all. Generic in
// the model prompt and the record (design §8.5's rule, extended by
// §6.4a) — an untrusted reader and a shared graph — and detailed on the
// operator's own diagnostic channel, so a DNS failure, a timeout and a
// 500 stay distinguishable to whoever operates the deployment.
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

	record, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.ToolCalls[0].Error != errSupplementaryRecallFailed {
		t.Fatalf("record.ToolCalls[0].Error = %q, want the generic sentence %q", record.ToolCalls[0].Error, errSupplementaryRecallFailed)
	}
	if !strings.Contains(logBuf.String(), "10.0.0.55") {
		t.Fatalf("operator log = %q, want it to carry the detailed error (including the address) that the record and prompt must not", logBuf.String())
	}
}

// TestTurnRunPreservesBothTheMappedAndRawStopReason pins design §8.2:
// "Two values, deliberately." Dropping the raw value from the record and
// keeping only the mapped one must fail this test.
func TestTurnRunPreservesBothTheMappedAndRawStopReason(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{{Reason: Unrecognised, RawReason: "some-vendor-string"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunLeavesUsageAbsentWhenTheModelReportedNone and
// TestTurnRunCarriesUsageWhenTheModelReportedIt together pin design §6.5:
// usage is absent, never zero-filled — a zero is a measurement, an
// absence is the truth. Revision 3 (W-1) widens Record.Usage to one entry
// per model call, so "absent" is a nil entry at that call's index, not a
// nil slice.
func TestTurnRunLeavesUsageAbsentWhenTheModelReportedNone(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{{Reason: Answered, RawReason: "stop", Usage: nil}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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

	record, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.Usage) != 1 || record.Usage[0] == nil || *record.Usage[0] != *usage {
		t.Fatalf("record.Usage = %+v, want [%+v]", record.Usage, usage)
	}
}

// TestTurnRunUsageArrayLengthAlwaysEqualsModelCalls pins design §8.2, W-1:
// the loop aggregates nothing — one usage entry per model call, in order,
// with a run where some calls report and others do not left legible as
// exactly that, rather than under- or over-counted.
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

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunWrapsModelFailureAsModelUnavailableAndWritesNothing pins
// design §6.5: "Model call fails ... | 502, nothing written."
func TestTurnRunWrapsModelFailureAsModelUnavailableAndWritesNothing(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{err: errors.New("literal: connection reset")}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	_, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("Run() err = %v, want ErrModelUnavailable", err)
	}
	if graph.writeRunCalled {
		t.Fatal("WriteRun was called after a model failure, want nothing written")
	}
}

// TestTurnRunWritesTheRecordAndReportsTheNodeID pins the successful
// write-back path (design §8.3's write port): the loop supplies the
// record; the adapter's reported node id lands in Written.
func TestTurnRunWritesTheRecordAndReportsTheNodeID(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.writeRunNodeID = 999
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !graph.writeRunCalled {
		t.Fatal("WriteRun was not called")
	}
	if record.Written.NodeID != 999 || record.Written.Error != "" {
		t.Fatalf("record.Written = %+v, want {NodeID:999}", record.Written)
	}
	if graph.writeRunRecord.Answer != "ok" || graph.writeRunRecord.Subject != 42 {
		t.Fatalf("the record handed to WriteRun = %+v, want the completed record", graph.writeRunRecord)
	}
}

// TestTurnRunReturns200EquivalentWhenWriteBackFailsWithTheFailureNamed
// pins design §6.5: "Write-back fails | 200, with the failure named in
// the record" — Run must return the record with no error, not fail the
// whole request over a write-back problem.
func TestTurnRunReturns200EquivalentWhenWriteBackFailsWithTheFailureNamed(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.writeRunErr = errors.New("literal: graph write timed out")
	model := &fakeModel{results: []JudgeResult{{Answer: "ok", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run() returned an error for a write-back failure, want nil error and the failure named in the record: %v", err)
	}
	if record.Written.NodeID != 0 {
		t.Fatalf("record.Written.NodeID = %d, want 0 on a write failure", record.Written.NodeID)
	}
	if record.Written.Error == "" {
		t.Fatal("record.Written.Error is empty, want the write failure named")
	}
}

// --- CF-4: the supplementary-recall budget (design §6.4a) ---

// TestTurnRunAdmitsSupplementaryHitsByRankOrderAndCutsTheRest pins design
// §6.4a: the same admission rule as the assembled block — rank order, stop
// rather than skip, no back-fill — applied per round, with position
// staying rank order (not re-sorted by id, unlike the block). Every row is
// still recorded, admitted or cut, with the size that caused the cut.
func TestTurnRunAdmitsSupplementaryHitsByRankOrderAndCutsTheRest(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	// rank 1 and 2 fit comfortably; rank 3 alone exceeds what's left of a
	// 20,000-byte round budget, so it and rank 4 (which would otherwise
	// fit in the leftover space) must both be cut — a stop, not a skip.
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{
			{ID: 91, Similarity: 0.9, Content: strings.Repeat("a", 9_000)}, // rank 1: fits, cumulative 9,000
			{ID: 92, Similarity: 0.8, Content: strings.Repeat("b", 9_000)}, // rank 2: fits, cumulative 18,000
			{ID: 93, Similarity: 0.7, Content: strings.Repeat("c", 5_000)}, // rank 3: 18,000+5,000 > 20,000, cut
			{ID: 94, Similarity: 0.6, Content: strings.Repeat("d", 100)},   // rank 4: would fit the 2,000 leftover, must still be cut
		}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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

	// What the model actually saw (the next Judge call's PriorRecalls) must
	// carry only the admitted subset, in rank order — the loop admits, the
	// adapter renders (design §6.4a).
	seen := model.calls[1].PriorRecalls[0].Results
	if len(seen) != 2 || seen[0].ID != 91 || seen[1].ID != 92 {
		t.Fatalf("PriorRecalls[0].Results = %+v, want the two admitted candidates [91 92], in rank order", seen)
	}
}

// TestTurnRunSupplementaryAdmissionStaysInRankOrderEvenWhenIDsDescend is the
// test that actually discriminates rank order from id order (design
// §6.4a): the admitted set above happened to arrive with ascending ids, so
// an implementation that (wrongly) re-sorted by id, like the block does,
// would have passed it by coincidence. Here every admitted id is smaller
// than the one before it, so an id-sort and a rank-order return produce
// visibly different sequences.
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

	record, err := turn.Run(context.Background(), "hello", 42)
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

// TestTurnRunASupplementaryRoundAdmittingNothingIsNotAnErrorAndRecordsEveryRowCut
// pins design §6.5's new row: when every hit in a round is larger than
// SupplementaryByteBudget, the round admits nothing — not an error, and
// not silence. The record carries all rows cut with the reason, and no
// oversized hit is admitted anyway (the anchor's R4 exemption, repeated on
// the path that demonstrated why it is a defect, must not reappear here).
func TestTurnRunASupplementaryRoundAdmittingNothingIsNotAnErrorAndRecordsEveryRowCut(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Candidates: []Candidate{
			{ID: 91, Content: strings.Repeat("a", 25_000)}, // alone exceeds the 20,000 round budget
		}},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, "system", "test-model", testLogger())

	record, err := turn.Run(context.Background(), "hello", 42)
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

	// The model must not see the oversized body — PriorRecalls' admitted
	// set is empty for this round.
	seen := model.calls[1].PriorRecalls[0].Results
	if len(seen) != 0 {
		t.Fatalf("PriorRecalls[0].Results has %d entries, want 0 — nothing was admitted", len(seen))
	}
}

// TestTurnRunAdmitsASupplementaryHitExactlyAtTheRoundBudget pins the same
// inclusive boundary design §6.4a states is identical to §6.3's: <=, not <.
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

	record, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	round := record.ToolCalls[0]
	if len(round.Results) != 1 || !round.Results[0].Included {
		t.Fatalf("round.Results = %+v, want the hit exactly at SupplementaryByteBudget included", round.Results)
	}
}

// TestTheCeilingArithmeticEqualsTheStatedLiteral pins design §8.4's stated
// ceiling — 100,000 bytes — against the arithmetic of the three constants
// that produce it (design §14 step 10, §6.4a). The literal is the design's
// defended value; the right-hand side uses the real production constants,
// so moving any one of AssemblyByteBudget, SupplementaryByteBudget or
// MaxModelCalls without re-deriving §8.4's window table turns this red.
func TestTheCeilingArithmeticEqualsTheStatedLiteral(t *testing.T) {
	t.Parallel()

	const wantCeiling = 100_000
	gotCeiling := AssemblyByteBudget + SupplementaryByteBudget*(MaxModelCalls-1)
	if gotCeiling != wantCeiling {
		t.Fatalf("AssemblyByteBudget + SupplementaryByteBudget*(MaxModelCalls-1) = %d, want %d (design §8.4's stated ceiling)", gotCeiling, wantCeiling)
	}
}
