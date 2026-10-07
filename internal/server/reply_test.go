package server

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

type scriptedModel struct {
	step  func(call int) (loop.JudgeResult, error)
	calls int
}

func (m *scriptedModel) Derive(context.Context, string, int) (string, error) { return "", nil }

func (m *scriptedModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	m.calls++
	return m.step(m.calls)
}

func answeringWith(answer string) func(int) (loop.JudgeResult, error) {
	return func(int) (loop.JudgeResult, error) {
		return loop.JudgeResult{Answer: answer, Reason: loop.Answered, RawReason: "stop"}, nil
	}
}

func replyGraph() stubGraph {
	return stubGraph{
		anchor:       loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		found:        true,
		candidates:   []loop.Candidate{{ID: 7, Type: "task", Name: "Cand", Similarity: 0.5, Content: "candidate body"}},
		writeReceipt: loop.WriteReceipt{State: loop.Stored, NodeID: 10525},
	}
}

func replyTurn(model loop.ModelPort) *loop.Turn {
	return loop.NewTurn(replyGraph(), model, nil, fixedSystem("system text"), "test-model", testLogger())
}

func postTo(t *testing.T, ctx context.Context, turn *loop.Turn, target string) *httptest.ResponseRecorder {
	t.Helper()
	return postRunsAt(t, ctx, turn, target, `{"input":"what is going on","subject":42}`)
}

func topLevelKeys(t *testing.T, rec *httptest.ResponseRecorder) (map[string]json.RawMessage, []string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return members, keys
}

func stringMember(t *testing.T, members map[string]json.RawMessage, key string) string {
	t.Helper()
	raw, present := members[key]
	if !present {
		t.Fatalf("the body has no %q member", key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("member %q is not a string: %v; raw=%s", key, err, raw)
	}
	return value
}

func TestRunsDefaultResponseCarriesOnlyTheReplyAndTheReceipt(t *testing.T) {
	t.Parallel()

	turn := replyTurn(&scriptedModel{step: answeringWith("The grip drops because the hold is not set.")})

	rec := postTo(t, context.Background(), turn, "/runs")

	members, keys := topLevelKeys(t, rec)
	if want := []string{"reply", "written"}; !slices.Equal(keys, want) {
		t.Fatalf("default body keys = %q, want %q", keys, want)
	}
	if got, want := string(members["written"]), `{"state":"stored","nodeId":10525}`; got != want {
		t.Fatalf("written = %s, want %s", got, want)
	}
}

func TestRunsVerboseResponseCarriesTheRecordTheReceiptAndTheReply(t *testing.T) {
	t.Parallel()

	turn := replyTurn(&scriptedModel{step: answeringWith("The grip drops because the hold is not set.")})

	rec := postTo(t, context.Background(), turn, "/runs?verbose=true")

	members, keys := topLevelKeys(t, rec)
	for _, want := range []string{"input", "query", "queries", "anchor", "candidates", "block", "answer", "model", "toolCalls", "modelCalls", "usage", "stopReason", "limits", "sampling", "outcome", "written", "reply"} {
		if !slices.Contains(keys, want) {
			t.Fatalf("verbose body keys = %q, missing %q", keys, want)
		}
	}
	if got, want := string(members["written"]), `{"state":"stored","nodeId":10525}`; got != want {
		t.Fatalf("written = %s, want %s", got, want)
	}
	if got, want := stringMember(t, members, "reply"), "The grip drops because the hold is not set."; got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
	if got, want := stringMember(t, members, "answer"), "The grip drops because the hold is not set."; got != want {
		t.Fatalf("answer = %q, want %q", got, want)
	}
}

func TestRunsReplyIsTheAnswerWhenTheRunProducedOne(t *testing.T) {
	t.Parallel()

	const answer = "  Sure — the grip slips when the hold is released too early.\n"
	turn := replyTurn(&scriptedModel{step: answeringWith(answer)})

	rec := postTo(t, context.Background(), turn, "/runs")

	members, _ := topLevelKeys(t, rec)
	if got := stringMember(t, members, "reply"); got != answer {
		t.Fatalf("reply = %q, want the answer verbatim %q, padding included", got, answer)
	}
}

func TestRunsReplyIsTheFallbackWhenTheRunProducedNothing(t *testing.T) {
	t.Parallel()

	const fallback = "Sorry — I couldn't put an answer together for that just now. Could you say a bit more, or ask it another way?"

	recallRound := func() loop.JudgeResult {
		return loop.JudgeResult{Reason: loop.WantsRecall, RawReason: "tool_calls", ToolError: "tool arguments could not be parsed: unexpected token"}
	}

	cases := []struct {
		name        string
		model       func() *scriptedModel
		timeout     func() time.Duration
		wantReserve string
		wantStalled bool
	}{
		{
			name:  "an empty answer",
			model: func() *scriptedModel { return &scriptedModel{step: answeringWith("")} },
		},
		{
			name:  "a whitespace-only answer",
			model: func() *scriptedModel { return &scriptedModel{step: answeringWith(" \n\t  ")} },
		},
		{
			name: "a reserved answering call that failed",
			model: func() *scriptedModel {
				return &scriptedModel{step: func(call int) (loop.JudgeResult, error) {
					if call < loop.MaxModelCalls {
						return recallRound(), nil
					}
					return loop.JudgeResult{}, errors.New("connection reset by peer")
				}}
			},
			wantReserve: "failed",
		},
		{
			name: "a turn stopped for time after a tool-wanting call",
			model: func() *scriptedModel {
				return &scriptedModel{step: func(call int) (loop.JudgeResult, error) {
					time.Sleep(700 * time.Millisecond)
					return loop.JudgeResult{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "q"}, nil
				}}
			},
			timeout:     func() time.Duration { return loop.JudgementCost(loop.Floors{}) + 500*time.Millisecond },
			wantStalled: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			newCtx := func() (context.Context, context.CancelFunc) {
				if c.timeout == nil {
					return context.WithCancel(context.Background())
				}
				return context.WithTimeout(context.Background(), c.timeout())
			}

			ctx, cancel := newCtx()
			defer cancel()
			verbose, _ := topLevelKeys(t, postTo(t, ctx, replyTurn(c.model()), "/runs?verbose=true"))

			var outcome struct {
				Produced bool `json:"produced"`
			}
			if err := json.Unmarshal(verbose["outcome"], &outcome); err != nil {
				t.Fatalf("decode outcome: %v", err)
			}
			if outcome.Produced {
				t.Fatalf("the fixture produced an answer (answer=%s), so it does not exercise the fallback", verbose["answer"])
			}
			var reserved struct {
				State string `json:"state"`
			}
			if raw, ok := verbose["reservedCall"]; ok {
				if err := json.Unmarshal(raw, &reserved); err != nil {
					t.Fatalf("decode reservedCall: %v", err)
				}
			}
			if reserved.State != c.wantReserve {
				t.Fatalf("reservedCall.state = %q, want %q: the fixture does not reach the record shape this row names", reserved.State, c.wantReserve)
			}
			if _, stalled := verbose["timeShortfall"]; stalled != c.wantStalled {
				t.Fatalf("timeShortfall present = %v, want %v: the fixture does not reach the record shape this row names", stalled, c.wantStalled)
			}

			ctx, cancel = newCtx()
			defer cancel()
			members, keys := topLevelKeys(t, postTo(t, ctx, replyTurn(c.model()), "/runs"))
			if want := []string{"reply", "written"}; !slices.Equal(keys, want) {
				t.Fatalf("default body keys = %q, want %q", keys, want)
			}
			if got := stringMember(t, members, "reply"); got != fallback {
				t.Fatalf("reply = %q, want the fallback sentence %q", got, fallback)
			}
		})
	}
}

func TestRunsVerboseWithNoValueMeansTrue(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"/runs?verbose", "/runs?verbose=", "/runs?verbose=1", "/runs?verbose=TRUE"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			turn := replyTurn(&scriptedModel{step: answeringWith("an answer")})

			members, _ := topLevelKeys(t, postTo(t, context.Background(), turn, target))
			if _, present := members["block"]; !present {
				t.Fatalf("%s returned the chat body, want the full record", target)
			}
		})
	}
}

func TestRunsVerboseFalseKeepsTheDefaultBody(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"/runs?verbose=false", "/runs?verbose=0"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			turn := replyTurn(&scriptedModel{step: answeringWith("an answer")})

			_, keys := topLevelKeys(t, postTo(t, context.Background(), turn, target))
			if want := []string{"reply", "written"}; !slices.Equal(keys, want) {
				t.Fatalf("%s body keys = %q, want %q", target, keys, want)
			}
		})
	}
}

func TestRunsReturns400OnAnUnparseableVerboseValue(t *testing.T) {
	t.Parallel()

	model := &scriptedModel{step: answeringWith("an answer")}

	rec := postTo(t, context.Background(), replyTurn(model), "/runs?verbose=maybe")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorCode(t, rec, codeInvalidRequest)
	if model.calls != 0 {
		t.Fatalf("the model was called %d times for a request that was refused", model.calls)
	}
}

func assertDeclaresJSONInUTF8(t *testing.T, contentType string) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || !strings.EqualFold(params["charset"], "utf-8") {
		t.Fatalf("Content-Type = %q, want application/json declaring charset utf-8: a client that defaults to a legacy code page garbles the non-ASCII reply text without the charset", contentType)
	}
}

func TestRunsDeclaresUTF8OnEveryJSONBodyItActuallySends(t *testing.T) {
	t.Parallel()

	answering := func() *loop.Turn { return replyTurn(&scriptedModel{step: answeringWith("an answer")}) }
	failingModel := func() *loop.Turn {
		return replyTurn(&scriptedModel{step: func(int) (loop.JudgeResult, error) { return loop.JudgeResult{}, errors.New("boom") }})
	}

	cases := []struct {
		name   string
		turn   *loop.Turn
		target string
		within time.Duration
		status int
	}{
		{"the default 200 body", answering(), "/runs", 0, http.StatusOK},
		{"the verbose 200 body", answering(), "/runs?verbose=true", 0, http.StatusOK},
		{"the 400 envelope", answering(), "/runs?verbose=maybe", 0, http.StatusBadRequest},
		{"the 404 envelope", newTestTurn(stubGraph{found: false}), "/runs", 0, http.StatusNotFound},
		{"the 502 envelope", failingModel(), "/runs", 0, http.StatusBadGateway},
		{"the 504 envelope", answering(), "/runs", 20 * time.Second, http.StatusGatewayTimeout},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(c.turn)
			if c.within > 0 {
				inner := handler
				handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx, cancel := context.WithTimeout(r.Context(), c.within)
					defer cancel()
					inner.ServeHTTP(w, r.WithContext(ctx))
				})
			}
			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			resp, err := http.Post(srv.URL+c.target, "application/json", strings.NewReader(`{"input":"what is going on","subject":42}`))
			if err != nil {
				t.Fatalf("POST %s: %v", c.target, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != c.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, c.status)
			}
			assertDeclaresJSONInUTF8(t, resp.Header.Get("Content-Type"))
		})
	}
}
