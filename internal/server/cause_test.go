package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
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
	return envelope.Error.Message
}

func TestRunsReports502WithTheModelCallsOwnCauseBehindTheClassSentence(t *testing.T) {
	t.Parallel()

	const cause = "openaicompat: model=ai/llama3.2 endpoint=http://host.example/v1/chat/completions request=76490 B elapsed=3.7s (client bound 5m0s): unexpected status 500: the endpoint rejected the request"
	graph := stubGraph{anchor: loop.Anchor{ID: 42, Content: "anchor body"}, found: true}
	turn := loop.NewTurn(graph, &stubModel{err: errors.New(cause)}, nil, "system text", "test-model", testLogger())

	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}

	want := "the model call did not complete: " + cause
	if got := errorMessage(t, rec); got != want {
		t.Fatalf("error.message = %q, want the class sentence followed by the model call's own cause %q", got, want)
	}
}

func TestRunsReports502WithTheAnchorReadsOwnCauseBehindTheClassSentence(t *testing.T) {
	t.Parallel()

	const cause = "divoid: request failed: Get \"http://graph.example/api/nodes/42\": dial tcp: connection refused"
	turn := newTestTurn(stubGraph{nodeErr: errors.New(cause)})

	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}

	want := "the graph could not be read: " + cause
	if got := errorMessage(t, rec); got != want {
		t.Fatalf("error.message = %q, want the class sentence followed by the graph read's own cause %q", got, want)
	}
}

func TestRunsBoundsTheCauseItPutsInTheErrorEnvelope(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("z", loop.CarriedCauseRunes+400)
	graph := stubGraph{anchor: loop.Anchor{ID: 42, Content: "anchor body"}, found: true}
	turn := loop.NewTurn(graph, &stubModel{err: errors.New(cause)}, nil, "system text", "test-model", testLogger())

	rec := postRuns(t, turn, `{"input":"hello","subject":42}`)
	carried := strings.TrimPrefix(errorMessage(t, rec), "the model call did not complete: ")

	if n := len([]rune(carried)); n != loop.CarriedCauseRunes {
		t.Fatalf("the envelope carried %d runes of a %d-rune cause, want it bounded to %d", n, len([]rune(cause)), loop.CarriedCauseRunes)
	}
}

func TestRunsReportsAnUnrecognisedSentinelByNameRatherThanBlamingTheGraph(t *testing.T) {
	t.Parallel()

	unrecognised := errors.New("workspace quota exhausted")

	got := withCause("the graph could not be read", unrecognised, loop.ErrGraphUnavailable)
	if !strings.Contains(got, unrecognised.Error()) {
		t.Fatalf("the default arm reported %q for an unrecognised sentinel, want it to name %q rather than a component that was never involved",
			got, unrecognised.Error())
	}
}

func TestWithCauseReportsTheClassAloneWhenTheSentinelCarriesNoCause(t *testing.T) {
	t.Parallel()

	const class = "the model call did not complete"
	if got := withCause(class, loop.ErrModelUnavailable, loop.ErrModelUnavailable); got != class {
		t.Fatalf("withCause returned %q for a bare sentinel, want the class sentence alone %q", got, class)
	}
}

func TestWithCauseDropsTheSentinelsOwnTextFromInFrontOfTheCause(t *testing.T) {
	t.Parallel()

	wrapped := errors.New(loop.ErrModelUnavailable.Error() + ": ollama: request failed: connection reset")

	got := withCause("the model call did not complete", wrapped, loop.ErrModelUnavailable)
	if strings.Contains(got, loop.ErrModelUnavailable.Error()) {
		t.Fatalf("error.message = %q, want the sentinel's own text replaced by the class sentence rather than repeated behind it", got)
	}
	if !strings.HasSuffix(got, "ollama: request failed: connection reset") {
		t.Fatalf("error.message = %q, want it to end in the cause the sentinel wrapped", got)
	}
}

func TestRunsKeepsEveryErrorCodeAndStatusUnchangedWhileCarryingTheCause(t *testing.T) {
	t.Parallel()

	graph := stubGraph{anchor: loop.Anchor{ID: 42, Content: "anchor body"}, found: true}

	cases := []struct {
		name   string
		turn   *loop.Turn
		body   string
		status int
		code   string
	}{
		{"invalid request", newTestTurn(graph), `{"input":"","subject":42}`, http.StatusBadRequest, codeInvalidRequest},
		{"subject not found", newTestTurn(stubGraph{found: false}), `{"input":"hello","subject":42}`, http.StatusNotFound, codeSubjectNotFound},
		{"graph unavailable", newTestTurn(stubGraph{nodeErr: errors.New("literal: refused")}), `{"input":"hello","subject":42}`, http.StatusBadGateway, codeGraphUnavailable},
		{"model unavailable", loop.NewTurn(graph, &stubModel{err: errors.New("literal: reset")}, nil, "system text", "test-model", testLogger()), `{"input":"hello","subject":42}`, http.StatusBadGateway, codeModelUnavailable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := postRuns(t, c.turn, c.body)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, c.status, rec.Body.String())
			}
			assertErrorCode(t, rec, c.code)
		})
	}
}

func TestRunsAddsNoCauseToTheSubjectNotFoundMessageTheLoopItselfAuthored(t *testing.T) {
	t.Parallel()

	rec := postRuns(t, newTestTurn(stubGraph{found: false}), `{"input":"hello","subject":42}`)

	const want = "the subject node was not found"
	if got := errorMessage(t, rec); got != want {
		t.Fatalf("error.message = %q, want the loop's own complete sentence %q — nobody else authored this failure", got, want)
	}
}

func TestRunsAddsNoCauseToTheRunDeadlineMessageTheLoopItselfAuthored(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	rec := postRunsWithContext(t, ctx, newTestTurn(&blockingGraph{}), `{"input":"hello","subject":42}`)

	const want = "the run did not produce an answer within the service's time limit"
	if got := errorMessage(t, rec); got != want {
		t.Fatalf("error.message = %q, want the loop's own complete sentence %q — the service's own ceiling authored this failure", got, want)
	}
}
