package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubGraph is a minimal loop.GraphPort double for handler-level tests.
// The handler's own job is status codes and the error envelope (design
// §5.5, §9.3); the stub controls that outcome deterministically without
// depending on internal/divoid or a live graph.
type stubGraph struct {
	anchor     loop.Anchor
	found      bool
	nodeErr    error
	candidates []loop.Candidate
	recallErr  error

	writeReceipt loop.WriteReceipt

	recallSeq []stubRecallResponse
	recallIdx *int
}

type stubRecallResponse struct {
	candidates []loop.Candidate
	err        error
}

func (s stubGraph) Node(context.Context, int64) (loop.Anchor, bool, error) {
	return s.anchor, s.found, s.nodeErr
}

func (s stubGraph) Recall(context.Context, string, int, []int64) ([]loop.Candidate, error) {
	if s.recallIdx != nil {
		i := *s.recallIdx
		*s.recallIdx++
		if i < len(s.recallSeq) {
			return s.recallSeq[i].candidates, s.recallSeq[i].err
		}
	}
	return s.candidates, s.recallErr
}

func (s stubGraph) Neighbours(context.Context, int64) ([]int64, error) { return nil, nil }

func (s stubGraph) WriteRun(context.Context, loop.Record) loop.WriteReceipt {
	return s.writeReceipt
}

type stubModel struct {
	results []loop.JudgeResult
	err     error

	calls int
}

func (s *stubModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	if s.err != nil {
		return loop.JudgeResult{}, s.err
	}
	if len(s.results) == 0 {
		return loop.JudgeResult{Reason: loop.Answered, RawReason: "stop"}, nil
	}
	idx := s.calls
	if idx >= len(s.results) {
		idx = len(s.results) - 1
	}
	s.calls++
	return s.results[idx], nil
}

func newTestTurn(graph loop.GraphPort) *loop.Turn {
	return loop.NewTurn(graph, &stubModel{}, "system text", "test-model", testLogger())
}

func TestHealth(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	NewHandler(newTestTurn(stubGraph{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}
	if body := rec.Body.String(); body != `{"status":"ok"}` {
		t.Fatalf("body = %q, want %q", body, `{"status":"ok"}`)
	}
}

func TestHealthUnregisteredPath(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)

	NewHandler(newTestTurn(stubGraph{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealthWrongMethod(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)

	NewHandler(newTestTurn(stubGraph{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("Allow = %q, want %q", allow, "GET, HEAD")
	}
}

// --- POST /runs ---

func postRuns(t *testing.T, turn *loop.Turn, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewBufferString(body))
	NewHandler(turn).ServeHTTP(rec, req)
	return rec
}

func postRunsWithContext(t *testing.T, ctx context.Context, turn *loop.Turn, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewBufferString(body)).WithContext(ctx)
	NewHandler(turn).ServeHTTP(rec, req)
	return rec
}

type deadlineGraph struct {
	nodeDeadline time.Time
	nodeBounded  bool
	writeBounded bool
}

func (g *deadlineGraph) Node(ctx context.Context, id int64) (loop.Anchor, bool, error) {
	g.nodeDeadline, g.nodeBounded = ctx.Deadline()
	return loop.Anchor{ID: id, Type: "documentation", Name: "Subject", Content: "anchor body"}, true, nil
}

func (g *deadlineGraph) Recall(context.Context, string, int, []int64) ([]loop.Candidate, error) {
	return nil, nil
}

func (g *deadlineGraph) Neighbours(context.Context, int64) ([]int64, error) { return nil, nil }

func (g *deadlineGraph) WriteRun(ctx context.Context, _ loop.Record) loop.WriteReceipt {
	_, g.writeBounded = ctx.Deadline()
	return loop.WriteReceipt{State: loop.Stored, NodeID: 999}
}

type blockingGraph struct{ runContextEnded bool }

func (g *blockingGraph) Node(ctx context.Context, _ int64) (loop.Anchor, bool, error) {
	select {
	case <-ctx.Done():
		g.runContextEnded = true
		return loop.Anchor{}, false, ctx.Err()
	case <-time.After(5 * time.Second):
		return loop.Anchor{}, false, errors.New("literal: the run context outlived the caller")
	}
}

func (g *blockingGraph) Recall(context.Context, string, int, []int64) ([]loop.Candidate, error) {
	return nil, nil
}

func (g *blockingGraph) Neighbours(context.Context, int64) ([]int64, error) { return nil, nil }

func (g *blockingGraph) WriteRun(context.Context, loop.Record) loop.WriteReceipt {
	return loop.WriteReceipt{State: loop.NotStored}
}

// runRecordWire is a local, literal-tagged decode target for the record —
// deliberately not loop.Record itself (CF-4). Decoding into the
// production type would move both sides of an assertion together on a
// field rename, which is the both-sides-move anti-pattern the design
// warns about (#10466 §14 step 1); a locally-declared struct with literal
// tags is what actually pins the wire shape, the same pattern already
// used for the error envelope in assertErrorCode below.
type runRecordWire struct {
	Input   string `json:"input"`
	Subject int64  `json:"subject"`
	Query   string `json:"query"`
	Anchor  struct {
		ID          int64  `json:"id"`
		Type        string `json:"type"`
		Name        string `json:"name"`
		Size        int    `json:"size"`
		ContentHash string `json:"contentHash"`
	} `json:"anchor"`
	Candidates []struct {
		Rank        int     `json:"rank"`
		ID          int64   `json:"id"`
		Type        string  `json:"type"`
		Name        string  `json:"name"`
		Similarity  float64 `json:"similarity"`
		Size        int     `json:"size"`
		ContentHash string  `json:"contentHash"`
		Included    bool    `json:"included"`
		CutReason   string  `json:"cutReason,omitempty"`
	} `json:"candidates"`
	Block string `json:"block"`

	Answer    string `json:"answer"`
	Model     string `json:"model"`
	ToolCalls []struct {
		Query   string `json:"query"`
		Error   string `json:"error"`
		Results []struct {
			Rank        int     `json:"rank"`
			ID          int64   `json:"id"`
			Type        string  `json:"type"`
			Name        string  `json:"name"`
			Similarity  float64 `json:"similarity"`
			Size        int     `json:"size"`
			ContentHash string  `json:"contentHash"`
			Included    bool    `json:"included"`
			CutReason   string  `json:"cutReason,omitempty"`
		} `json:"results"`
	} `json:"toolCalls"`
	ModelCalls int  `json:"modelCalls"`
	CapReached bool `json:"capReached"`
	Usage      []*struct {
		InTokens  int `json:"inTokens"`
		OutTokens int `json:"outTokens"`
	} `json:"usage"`
	StopReason struct {
		Reason string `json:"reason"`
		Raw    string `json:"raw"`
	} `json:"stopReason"`
	Written struct {
		State  string `json:"state"`
		NodeID int64  `json:"nodeId"`
	} `json:"written"`
	Limits struct {
		CandidateLimit          int `json:"candidateLimit"`
		AssemblyByteBudget      int `json:"assemblyByteBudget"`
		SupplementaryByteBudget int `json:"supplementaryByteBudget"`
		MaxModelCalls           int `json:"maxModelCalls"`
		MaxOutputTokens         int `json:"maxOutputTokens"`
	} `json:"limits"`
	Sampling struct {
		Temperature *float64 `json:"temperature"`
		TopP        *float64 `json:"topP"`
	} `json:"sampling"`
}

func TestRunsReturns200WithTheAssembledRecordOnSuccess(t *testing.T) {
	t.Parallel()

	turn := newTestTurn(stubGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:  true,
		candidates: []loop.Candidate{
			{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"},
		},
	})

	rec := postRuns(t, turn, `{"input":"what is going on","subject":42}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got runRecordWire
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	if got.Subject != 42 {
		t.Fatalf("record.Subject = %d, want 42", got.Subject)
	}
	if got.Query != "what is going on" {
		t.Fatalf("record.Query = %q, want the input verbatim", got.Query)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("record.Candidates has %d entries, want 1", len(got.Candidates))
	}
	if got.Block == "" {
		t.Fatal("record.Block is empty, want the assembled context")
	}
	// The two fields CF-4 named explicitly as unasserted: a rename of
	// either JSON key (candidates->kept, contentHash->hash) leaves these
	// zero under the literal-tagged struct above.
	if got.Anchor.ContentHash == "" {
		t.Fatal("record.anchor.contentHash is empty under the literal JSON tag, want it populated")
	}
	if got.Candidates[0].ContentHash == "" {
		t.Fatal("record.candidates[0].contentHash is empty under the literal JSON tag, want it populated")
	}
	if got.Candidates[0].Similarity != 0.5 {
		t.Fatalf("record.candidates[0].similarity = %v, want %v — not zeroed", got.Candidates[0].Similarity, 0.5)
	}
}

func TestRunsRecordWireCarriesUnitBFields(t *testing.T) {
	t.Parallel()

	graph := stubGraph{
		anchor:       loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:        true,
		candidates:   []loop.Candidate{{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"}},
		writeReceipt: loop.WriteReceipt{State: loop.Stored, NodeID: 4242},
	}
	temperature := 0.4
	model := &stubModel{results: []loop.JudgeResult{
		{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "the missing thing"},
		{
			Answer:    "the final answer",
			Reason:    loop.Answered,
			RawReason: "stop",
			Usage:     &loop.Usage{InTokens: 11, OutTokens: 22},
			Sampling:  loop.Sampling{Temperature: &temperature},
		},
	}}
	turn := loop.NewTurn(graph, model, "system text", "test-model-id", testLogger())

	rec := postRuns(t, turn, `{"input":"what is going on","subject":42}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got runRecordWire
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}

	if got.Answer != "the final answer" {
		t.Fatalf("record.answer = %q, want %q", got.Answer, "the final answer")
	}
	if got.Model != "test-model-id" {
		t.Fatalf("record.model = %q, want %q", got.Model, "test-model-id")
	}
	if got.ModelCalls != 2 {
		t.Fatalf("record.modelCalls = %d, want 2", got.ModelCalls)
	}
	if got.CapReached {
		t.Fatal("record.capReached = true, want false — the model answered on the second call")
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("record.toolCalls has %d entries, want 1", len(got.ToolCalls))
	}
	if got.ToolCalls[0].Query != "the missing thing" {
		t.Fatalf("record.toolCalls[0].query = %q, want %q", got.ToolCalls[0].Query, "the missing thing")
	}
	if len(got.ToolCalls[0].Results) != 1 || got.ToolCalls[0].Results[0].ID != 7 || got.ToolCalls[0].Results[0].Similarity != 0.5 {
		t.Fatalf("record.toolCalls[0].results = %+v, want [{7 0.5 ...}]", got.ToolCalls[0].Results)
	}
	if !got.ToolCalls[0].Results[0].Included {
		t.Fatal("record.toolCalls[0].results[0].included = false, want true — the tiny fixture body is well under SupplementaryByteBudget")
	}
	if got.ToolCalls[0].Results[0].ContentHash == "" {
		t.Fatal("record.toolCalls[0].results[0].contentHash is empty, want it populated — a supplementary result carries the same disposition columns as an assembly candidate")
	}
	if got.ToolCalls[0].Results[0].Rank != 1 {
		t.Fatalf("record.toolCalls[0].results[0].rank = %d, want 1", got.ToolCalls[0].Results[0].Rank)
	}
	if got.ToolCalls[0].Results[0].Type != "task" {
		t.Fatalf("record.toolCalls[0].results[0].type = %q, want %q", got.ToolCalls[0].Results[0].Type, "task")
	}
	if got.ToolCalls[0].Results[0].Name != "Cand" {
		t.Fatalf("record.toolCalls[0].results[0].name = %q, want %q", got.ToolCalls[0].Results[0].Name, "Cand")
	}
	if got.ToolCalls[0].Results[0].Size != len("candidate body") {
		t.Fatalf("record.toolCalls[0].results[0].size = %d, want %d", got.ToolCalls[0].Results[0].Size, len("candidate body"))
	}
	if len(got.Usage) != 2 {
		t.Fatalf("record.usage has %d entries, want 2 (one per model call)", len(got.Usage))
	}
	if got.Usage[0] != nil {
		t.Fatalf("record.usage[0] = %+v, want nil — the first (WantsRecall) call reported no usage", got.Usage[0])
	}
	if got.Usage[1] == nil || got.Usage[1].InTokens != 11 || got.Usage[1].OutTokens != 22 {
		t.Fatalf("record.usage[1] = %+v, want {11 22}", got.Usage[1])
	}
	if got.StopReason.Reason != "answered" || got.StopReason.Raw != "stop" {
		t.Fatalf("record.stopReason = %+v, want {answered stop}", got.StopReason)
	}
	if got.Written.State != "stored" {
		t.Fatalf("written.state = %q, want %q", got.Written.State, "stored")
	}
	if got.Written.NodeID != 4242 {
		t.Fatalf("written.nodeId = %d, want 4242", got.Written.NodeID)
	}
	wantLimits := struct {
		CandidateLimit          int
		AssemblyByteBudget      int
		SupplementaryByteBudget int
		MaxModelCalls           int
		MaxOutputTokens         int
	}{20, 60_000, 20_000, 3, 4_096}
	if got.Limits.CandidateLimit != wantLimits.CandidateLimit ||
		got.Limits.AssemblyByteBudget != wantLimits.AssemblyByteBudget ||
		got.Limits.SupplementaryByteBudget != wantLimits.SupplementaryByteBudget ||
		got.Limits.MaxModelCalls != wantLimits.MaxModelCalls ||
		got.Limits.MaxOutputTokens != wantLimits.MaxOutputTokens {
		t.Fatalf("record.limits = %+v, want %+v", got.Limits, wantLimits)
	}
	if got.Sampling.Temperature == nil || *got.Sampling.Temperature != 0.4 {
		t.Fatalf("record.sampling.temperature = %v, want a pointer to 0.4", got.Sampling.Temperature)
	}
	if got.Sampling.TopP != nil {
		t.Fatalf("record.sampling.topP = %v, want nil — the model never reported one", got.Sampling.TopP)
	}
}

func TestRunsRecordWireCarriesTheFailurePathFields(t *testing.T) {
	t.Parallel()

	graph := stubGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:  true,
		recallSeq: []stubRecallResponse{
			{candidates: []loop.Candidate{{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"}}},
			{candidates: []loop.Candidate{{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"}}},
			{err: errors.New("literal: 500 from graph")},
		},
		recallIdx:    new(int),
		writeReceipt: loop.WriteReceipt{State: loop.NotStored},
	}
	model := &stubModel{results: []loop.JudgeResult{
		{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "the missing budget row"},
		{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallError: "tool arguments could not be parsed: unexpected token"},
		{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "final desperate query", RecallError: "tool arguments could not be parsed: second malformed request"},
	}}
	turn := loop.NewTurn(graph, model, "system text", "test-model", testLogger())

	rec := postRuns(t, turn, `{"input":"what is going on","subject":42}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got runRecordWire
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}

	if got.ModelCalls != 3 {
		t.Fatalf("record.modelCalls = %d, want 3", got.ModelCalls)
	}
	if !got.CapReached {
		t.Fatal("record.capReached = false, want true — the model still wanted recall on the third and final call")
	}
	if len(got.ToolCalls) != 3 {
		t.Fatalf("record.toolCalls has %d entries, want 3", len(got.ToolCalls))
	}
	if got.ToolCalls[0].Query != "the missing budget row" {
		t.Fatalf("record.toolCalls[0].query = %q, want %q", got.ToolCalls[0].Query, "the missing budget row")
	}
	if got.ToolCalls[0].Error != "supplementary recall failed" {
		t.Fatalf("record.toolCalls[0].error = %q, want the generic sentence %q — not the raw transport error", got.ToolCalls[0].Error, "supplementary recall failed")
	}
	if got.ToolCalls[1].Error != "tool arguments could not be parsed: unexpected token" {
		t.Fatalf("record.toolCalls[1].error = %q, want %q", got.ToolCalls[1].Error, "tool arguments could not be parsed: unexpected token")
	}
	if got.ToolCalls[2].Error != "tool arguments could not be parsed: second malformed request" {
		t.Fatalf("record.toolCalls[2].error = %q, want %q", got.ToolCalls[2].Error, "tool arguments could not be parsed: second malformed request")
	}
	const round3Wire = `"error":"tool arguments could not be parsed: second malformed request","results":[]`
	if !strings.Contains(rec.Body.String(), round3Wire) {
		t.Fatalf("body does not contain %q — record.toolCalls[2].results must serialise as [] not null; body=%s", round3Wire, rec.Body.String())
	}
	if got.Written.State != "notStored" {
		t.Fatalf("written.state = %q, want %q — the closed vocabulary names the fate, no free text", got.Written.State, "notStored")
	}
	if got.Written.NodeID != 0 {
		t.Fatalf("written.nodeId = %d, want 0 — no node holds the record", got.Written.NodeID)
	}
	const wantReceiptWire = `"written":{"state":"notStored"}`
	if !strings.Contains(rec.Body.String(), wantReceiptWire) {
		t.Fatalf("body does not contain %s — the receipt names the fate and carries nothing else; body=%s", wantReceiptWire, rec.Body.String())
	}
}

func TestRunsToolCallsResultsCutReasonIsPopulatedAtTheWireLevel(t *testing.T) {
	t.Parallel()

	graph := stubGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:  true,
		candidates: []loop.Candidate{
			{ID: 7, Type: "task", Name: "Small", Content: "small body"},
			{ID: 8, Type: "task", Name: "Large", Content: strings.Repeat("x", 21_000)},
		},
	}
	model := &stubModel{results: []loop.JudgeResult{
		{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		{Answer: "final", Reason: loop.Answered, RawReason: "stop"},
	}}
	turn := loop.NewTurn(graph, model, "system text", "test-model", testLogger())

	rec := postRuns(t, turn, `{"input":"what is going on","subject":42}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got runRecordWire
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	if len(got.ToolCalls) != 1 || len(got.ToolCalls[0].Results) != 2 {
		t.Fatalf("record.toolCalls = %+v, want 1 round with 2 rows", got.ToolCalls)
	}
	small, large := got.ToolCalls[0].Results[0], got.ToolCalls[0].Results[1]
	if !small.Included || small.CutReason != "" {
		t.Fatalf("record.toolCalls[0].results[0] = %+v, want the small hit admitted with no cut reason", small)
	}
	if large.Included {
		t.Fatal("record.toolCalls[0].results[1].included = true, want false — the 21,000-byte hit exceeds SupplementaryByteBudget (20,000)")
	}
	if large.CutReason == "" {
		t.Fatal("record.toolCalls[0].results[1].cutReason is empty at the wire level, want it populated")
	}
}

// TestRunsCandidatesIsAnEmptyArrayNeverNullWhenThereAreNone pins W-5 at the
// wire level: a JSON-null candidates field is a broken contract for any
// consumer that assumes an array.
// TestRunsQueryReachesTheLoopVerbatim pins rule 4 (the query is the input
// verbatim) at the handler layer — the third layer this anti-pattern can
// hide at (CF-1 pinned it at the loop and adapter layers already). The
// fixture is deliberately not already trimmed/lower-cased: an
// already-normalised fixture would survive a TrimSpace/ToLower inserted
// here and prove nothing (W-10).
func TestRunsQueryReachesTheLoopVerbatim(t *testing.T) {
	t.Parallel()

	const rawInput = "  What Is Going ON  "

	turn := newTestTurn(stubGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:  true,
	})

	rec := postRuns(t, turn, `{"input":"`+rawInput+`","subject":42}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got runRecordWire
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	if got.Input != rawInput {
		t.Fatalf("record.Input = %q, want the request input verbatim %q (not trimmed or lower-cased)", got.Input, rawInput)
	}
	if got.Query != rawInput {
		t.Fatalf("record.Query = %q, want the request input verbatim %q (not trimmed or lower-cased)", got.Query, rawInput)
	}
}

func TestRunsCandidatesIsAnEmptyArrayNeverNullWhenThereAreNone(t *testing.T) {
	t.Parallel()

	turn := newTestTurn(stubGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:  true,
		// candidates left at its zero value (nil) — recall returned nothing.
	})

	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"candidates":[]`) {
		t.Fatalf("body = %s, want it to contain %q, not a null candidates field", rec.Body.String(), `"candidates":[]`)
	}
}

func TestRunsReturns400OnUnparseableBody(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, newTestTurn(stubGraph{}), `{not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

func TestRunsReturns400OnEmptyInput(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, newTestTurn(stubGraph{}), `{"input":"   ","subject":42}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

func TestRunsReturns400OnMissingSubject(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, newTestTurn(stubGraph{}), `{"input":"hello"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

// TestRunsReturns400OnNegativeSubject pins W-9: design line 374 requires
// "missing or malformed -> 400", and a negative id is malformed — it must
// not reach the graph.
func TestRunsReturns400OnNegativeSubject(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, newTestTurn(stubGraph{}), `{"input":"hello","subject":-1}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

// TestRunsReturns400OnOversizedBody pins W-6: the request body is bounded,
// not decoded unconditionally regardless of size.
func TestRunsReturns400OnOversizedBody(t *testing.T) {
	t.Parallel()

	oversized := `{"input":"` + strings.Repeat("x", 2<<20) + `","subject":42}`
	rec := postRuns(t, newTestTurn(stubGraph{}), oversized)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body length=%d", rec.Code, http.StatusBadRequest, rec.Body.Len())
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

func TestRunsReturns404WhenSubjectNotFound(t *testing.T) {
	t.Parallel()

	turn := newTestTurn(stubGraph{found: false})
	rec := postRuns(t, turn, `{"input":"hello","subject":999}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	assertErrorCode(t, rec, codeSubjectNotFound)
}

func TestRunsReturns502WhenAnchorReadFails(t *testing.T) {
	t.Parallel()

	turn := newTestTurn(stubGraph{nodeErr: errors.New("literal: dial tcp: connection refused")})
	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	assertErrorCode(t, rec, codeGraphUnavailable)
}

func TestRunsReturns502WhenRecallFails(t *testing.T) {
	t.Parallel()

	turn := newTestTurn(stubGraph{
		anchor:    loop.Anchor{ID: 42},
		found:     true,
		recallErr: errors.New("literal: 500 from graph"),
	})
	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	assertErrorCode(t, rec, codeGraphUnavailable)
}

func TestRunsReturns502WithModelUnavailableWhenTheModelCallFails(t *testing.T) {
	t.Parallel()

	graph := stubGraph{anchor: loop.Anchor{ID: 42, Content: "anchor body"}, found: true}
	turn := loop.NewTurn(graph, &stubModel{err: errors.New("literal: connection reset")}, "system text", "test-model", testLogger())

	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	assertErrorCode(t, rec, codeModelUnavailable)
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v; body=%s", err, rec.Body.String())
	}
	if envelope.Error.Code != want {
		t.Fatalf("error.code = %q, want %q", envelope.Error.Code, want)
	}
	if envelope.Error.Message == "" {
		t.Fatal("error.message is empty")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestRunsReturns504WhenTheRunContextExpiresBeforeAnAnswerExists(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	graph := &blockingGraph{}
	rec := postRunsWithContext(t, ctx, newTestTurn(graph), `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d — an expired run is the service hanging up, not an upstream failure, body=%s", rec.Code, http.StatusGatewayTimeout, rec.Body.String())
	}
	assertErrorCode(t, rec, codeRunDeadlineExceeded)
}

func TestRunsDoesNotReport502ForAnExpiredRunEvenThoughAGraphCallCarriedTheExpiry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	graph := &blockingGraph{}
	rec := postRunsWithContext(t, ctx, newTestTurn(graph), `{"input":"hello","subject":42}`)

	if rec.Code == http.StatusBadGateway {
		t.Fatalf("status = %d — the deadline surfaced disguised as the graph call in flight and was classified by error type; body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v; body=%s", err, rec.Body.String())
	}
	if envelope.Error.Code == codeGraphUnavailable || envelope.Error.Code == codeModelUnavailable {
		t.Fatalf("error.code = %q, want the run's own bound named — reporting our ceiling as an upstream failure sends the caller to fix the wrong thing", envelope.Error.Code)
	}
}

func TestRunsReports502NotTheDeadlineCodeWhenTheCallerMerelyDisconnects(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	graph := &blockingGraph{}
	rec := postRunsWithContext(t, ctx, newTestTurn(graph), `{"input":"hello","subject":42}`)

	if !graph.runContextEnded {
		t.Fatal("the run context outlived the caller's cancellation — the handler's ceiling is not derived from the request's own context")
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d — a cancelled request is not an expired run, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	assertErrorCode(t, rec, codeGraphUnavailable)
}

func parsePackageSources(t *testing.T, fset *token.FileSet) map[string]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = file
	}
	return files
}

func packageStringValues(files map[string]*ast.File) map[string]string {
	values := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range value.Names {
					if i >= len(value.Values) {
						continue
					}
					if literal, ok := stringValueOf(value.Values[i]); ok {
						values[ident.Name] = literal
					}
				}
			}
		}
	}
	return values
}

func stringValueOf(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(node.Value)
		return unquoted, err == nil
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return "", false
		}
		left, leftOK := stringValueOf(node.X)
		right, rightOK := stringValueOf(node.Y)
		return left + right, leftOK && rightOK
	}
	return "", false
}

func envelopeCodes(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	files := parsePackageSources(t, fset)
	values := packageStringValues(files)

	var codes []string
	for name, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee, ok := call.Fun.(*ast.Ident)
			if !ok || callee.Name != "writeError" || len(call.Args) < 3 {
				return true
			}

			where := fmt.Sprintf("%s:%d", name, fset.Position(call.Args[2].Pos()).Line)
			if literal, ok := stringValueOf(call.Args[2]); ok {
				codes = append(codes, literal)
				return true
			}
			named, ok := call.Args[2].(*ast.Ident)
			if !ok {
				t.Fatalf("%s: the code handed to writeError is neither a name nor a string, so what this endpoint can emit is unreadable here", where)
			}
			literal, resolved := values[named.Name]
			if !resolved {
				t.Fatalf("%s: %s does not resolve to a string declared in this package", where, named.Name)
			}
			codes = append(codes, literal)
			return true
		})
	}

	slices.Sort(codes)
	return slices.Compact(codes)
}

func TestTheErrorEnvelopeCanEmitExactlyFiveCodes(t *testing.T) {
	t.Parallel()

	want := []string{"graph_unavailable", "invalid_request", "model_unavailable", "run_deadline_exceeded", "subject_not_found"}

	if got := envelopeCodes(t); !slices.Equal(got, want) {
		t.Fatalf("the envelope can emit %v, want exactly %v — a sixth code is a design decision, not an addition", got, want)
	}
}

func TestEachErrorCodeConstantCarriesItsWireValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		got  string
		want string
	}{
		{codeInvalidRequest, "invalid_request"},
		{codeSubjectNotFound, "subject_not_found"},
		{codeGraphUnavailable, "graph_unavailable"},
		{codeModelUnavailable, "model_unavailable"},
		{codeRunDeadlineExceeded, "run_deadline_exceeded"},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("an error code constant carries %q, want %q", c.got, c.want)
		}
	}
}

func TestRunsCeilsARunTheCallerNeverBoundedAtTenMinutes(t *testing.T) {
	t.Parallel()

	graph := &deadlineGraph{}

	rec := postRuns(t, newTestTurn(graph), `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d — the ceiling must not cut an ordinary run, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !graph.nodeBounded {
		t.Fatal("the turn ran on a context carrying no deadline, and the request carried none of its own — the handler applies no ceiling at all")
	}

	const wantCeiling = 10 * time.Minute
	const observationSlack = 1 * time.Second
	if remaining := time.Until(graph.nodeDeadline); remaining <= wantCeiling-observationSlack || remaining > wantCeiling {
		t.Fatalf("the run's ceiling leaves %v, want %v less at most the %v this observation itself can cost", remaining, wantCeiling, observationSlack)
	}
}

func TestRunsCeilingStopsAtTheAnswerAndDoesNotReachTheWriteBack(t *testing.T) {
	t.Parallel()

	graph := &deadlineGraph{}

	rec := postRuns(t, newTestTurn(graph), `{"input":"hello","subject":42}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !graph.nodeBounded {
		t.Fatal("the run was never bounded, so this test cannot show what the bound excludes")
	}
	if graph.writeBounded {
		t.Fatal("the write-back inherited the run's ceiling; design §8.4 bounds everything up to and including the answer, and the filing comes after it")
	}
}
