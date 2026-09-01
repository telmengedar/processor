package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

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
}

func (s stubGraph) Node(context.Context, int64) (loop.Anchor, bool, error) {
	return s.anchor, s.found, s.nodeErr
}

func (s stubGraph) Recall(context.Context, string, int) ([]loop.Candidate, error) {
	return s.candidates, s.recallErr
}

func TestHealth(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	NewHandler(loop.NewTurn(stubGraph{})).ServeHTTP(rec, req)

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

	NewHandler(loop.NewTurn(stubGraph{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealthWrongMethod(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)

	NewHandler(loop.NewTurn(stubGraph{})).ServeHTTP(rec, req)

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
}

func TestRunsReturns200WithTheAssembledRecordOnSuccess(t *testing.T) {
	t.Parallel()

	turn := loop.NewTurn(stubGraph{
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

	turn := loop.NewTurn(stubGraph{
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

	turn := loop.NewTurn(stubGraph{
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

	rec := postRuns(t, loop.NewTurn(stubGraph{}), `{not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

func TestRunsReturns400OnEmptyInput(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, loop.NewTurn(stubGraph{}), `{"input":"   ","subject":42}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

func TestRunsReturns400OnMissingSubject(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, loop.NewTurn(stubGraph{}), `{"input":"hello"}`)

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

	rec := postRuns(t, loop.NewTurn(stubGraph{}), `{"input":"hello","subject":-1}`)

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
	rec := postRuns(t, loop.NewTurn(stubGraph{}), oversized)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body length=%d", rec.Code, http.StatusBadRequest, rec.Body.Len())
	}
	assertErrorCode(t, rec, codeInvalidRequest)
}

func TestRunsReturns404WhenSubjectNotFound(t *testing.T) {
	t.Parallel()

	turn := loop.NewTurn(stubGraph{found: false})
	rec := postRuns(t, turn, `{"input":"hello","subject":999}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	assertErrorCode(t, rec, codeSubjectNotFound)
}

func TestRunsReturns502WhenAnchorReadFails(t *testing.T) {
	t.Parallel()

	turn := loop.NewTurn(stubGraph{nodeErr: errors.New("literal: dial tcp: connection refused")})
	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	assertErrorCode(t, rec, codeGraphUnavailable)
}

func TestRunsReturns502WhenRecallFails(t *testing.T) {
	t.Parallel()

	turn := loop.NewTurn(stubGraph{
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
