package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	writeNodeID int64

	writeErr error

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

func (s stubGraph) Recall(context.Context, string, int) ([]loop.Candidate, error) {
	if s.recallIdx != nil {
		i := *s.recallIdx
		*s.recallIdx++
		if i < len(s.recallSeq) {
			return s.recallSeq[i].candidates, s.recallSeq[i].err
		}
	}
	return s.candidates, s.recallErr
}

func (s stubGraph) WriteRun(context.Context, loop.Record) (int64, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return s.writeNodeID, nil
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
		NodeID int64  `json:"nodeId"`
		Error  string `json:"error"`
	} `json:"written"`
	Limits struct {
		CandidateLimit          int `json:"candidateLimit"`
		AssemblyByteBudget      int `json:"assemblyByteBudget"`
		SupplementaryByteBudget int `json:"supplementaryByteBudget"`
		MaxModelCalls           int `json:"maxModelCalls"`
		MaxOutputTokens         int `json:"maxOutputTokens"`
	} `json:"limits"`
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
		anchor:      loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:       true,
		candidates:  []loop.Candidate{{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"}},
		writeNodeID: 4242,
	}
	model := &stubModel{results: []loop.JudgeResult{
		{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "the missing thing"},
		{
			Answer:    "the final answer",
			Reason:    loop.Answered,
			RawReason: "stop",
			Usage:     &loop.Usage{InTokens: 11, OutTokens: 22},
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
	if got.Written.NodeID != 4242 {
		t.Fatalf("record.written.nodeId = %d, want 4242", got.Written.NodeID)
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
}

func TestRunsRecordWireCarriesTheFailurePathFields(t *testing.T) {
	t.Parallel()

	graph := stubGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:  true,
		recallSeq: []stubRecallResponse{
			{candidates: []loop.Candidate{{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"}}},
			{err: errors.New("literal: 500 from graph")},
		},
		recallIdx:   new(int),
		writeNodeID: 4242,
		writeErr:    errors.New(`divoid: request failed: Post "http://graph.internal:9099/api/nodes": dial tcp 10.4.4.4:9099: connect: connection refused`),
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
	if got.Written.Error != "write-back failed" {
		t.Fatalf("record.written.error = %q, want the generic sentence %q — not the graph adapter's raw error", got.Written.Error, "write-back failed")
	}
	for _, secret := range []string{"graph.internal", "10.4.4.4", "/api/nodes"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("response body discloses %q from the write-back error; body=%s", secret, rec.Body.String())
		}
	}
	if got.Written.NodeID != 0 {
		t.Fatalf("record.written.nodeId = %d, want 0 — the write failed, so no node id was assigned", got.Written.NodeID)
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
