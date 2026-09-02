package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

// capturingServer returns an httptest.Server that records the last
// request's body and headers and answers with respBody (already valid
// JSON) on every call.
func capturingServer(t *testing.T, respBody string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Path = r.URL.Path
		captured.Auth = r.Header.Get("Authorization")
		captured.ContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		captured.Body = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

type capturedRequest struct {
	Path        string
	Auth        string
	ContentType string
	Body        []byte
}

const stopResponse = `{"choices":[{"message":{"content":"the answer"},"finish_reason":"stop"}]}`

func TestJudgePostsToChatCompletions(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if captured.Path != "/chat/completions" {
		t.Fatalf("path = %q, want %q", captured.Path, "/chat/completions")
	}
}

// TestJudgeSendsNoAuthorizationHeaderWhenKeyIsEmpty pins design §8.1: an
// absent key means an absent header, not an empty one — the local-runtime
// case the ruling exists for.
func TestJudgeSendsNoAuthorizationHeaderWhenKeyIsEmpty(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if captured.Auth != "" {
		t.Fatalf("Authorization = %q, want no header sent", captured.Auth)
	}
}

func TestJudgeSendsAuthorizationBearerWhenKeyIsSet(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "secret-key", srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}
	const want = "Bearer secret-key"
	if captured.Auth != want {
		t.Fatalf("Authorization = %q, want %q", captured.Auth, want)
	}
}

// TestJudgeRequestBodyCarriesModelSystemBlockInputAndTool pins the
// request construction (design §6.6 D1-D6) at the wire level: the model
// id sent is exactly what NewClient was given, the system/user messages
// carry the framing, and exactly one tool is declared.
func TestJudgeRequestBodyCarriesModelSystemBlockInputAndTool(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "the-model-id", "", srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "the system text", Block: "the block", Input: "the input"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}

	// Every "want" value below is a literal, not the package's own
	// constant (design §14 step 1): a shared constant moves both sides of
	// the assertion together on a value change and the test can never
	// fail — see TestDefaultTimeoutIsNotDivoidsTimeout's comment for the
	// same discipline applied to the timeout (CF-2).
	const (
		wantMaxTokens         = 4096
		wantToolName          = "recall"
		wantToolDescription   = "Search memory for something the assembled context did not include. Takes one argument: query, a short description of what is missing."
		wantBlockInputContent = "the block\n===== INPUT =====\nthe input"
	)

	if got.Model != "the-model-id" {
		t.Fatalf("model = %q, want %q", got.Model, "the-model-id")
	}
	if got.MaxTokens != wantMaxTokens {
		t.Fatalf("max_tokens = %d, want %d", got.MaxTokens, wantMaxTokens)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages has %d entries, want 2 (system, user)", len(got.Messages))
	}
	if got.Messages[0].Role != "system" || got.Messages[0].Content != "the system text" {
		t.Fatalf("messages[0] = %+v, want the system role carrying the system text verbatim", got.Messages[0])
	}
	if got.Messages[1].Role != "user" {
		t.Fatalf("messages[1].Role = %q, want %q", got.Messages[1].Role, "user")
	}
	// Byte-exact (design §9.1 stage 3a, §14 step 9), not a substring
	// check: a substring check cannot catch the "===== INPUT ====="
	// separator being removed and the two halves collapsed into one run
	// of text (W-3).
	if got.Messages[1].Content != wantBlockInputContent {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got.Messages[1].Content, wantBlockInputContent)
	}
	if len(got.Tools) != 1 {
		t.Fatalf("tools has %d entries, want exactly 1 (design §2.4: one tool)", len(got.Tools))
	}
	if got.Tools[0].Type != "function" || got.Tools[0].Function.Name != wantToolName {
		t.Fatalf("tools[0] = %+v, want the recall function tool named %q", got.Tools[0], wantToolName)
	}
	// The tool's description and parameter schema are the whole of what
	// the model is told the tool does — previously asserted nowhere
	// (W-3).
	if got.Tools[0].Function.Description != wantToolDescription {
		t.Fatalf("tools[0].function.description = %q, want %q", got.Tools[0].Function.Description, wantToolDescription)
	}
	var gotParams map[string]any
	if err := json.Unmarshal(got.Tools[0].Function.Parameters, &gotParams); err != nil {
		t.Fatalf("decode tools[0].function.parameters: %v; raw=%s", err, got.Tools[0].Function.Parameters)
	}
	wantParams := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
		"required": []any{"query"},
	}
	if !reflect.DeepEqual(gotParams, wantParams) {
		t.Fatalf("tools[0].function.parameters = %#v, want %#v", gotParams, wantParams)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// TestJudgeReconstructsPriorRecallsAsAssistantAndToolMessages pins design
// §9.1 stage 3a/3c: the whole conversation is rebuilt from PriorRecalls on
// every call, as an assistant tool-call message followed by a matching
// tool-result message — for both a successful round and an error-flagged
// one. Round 1 carries two candidates, not one (W-1): a single-candidate
// fixture cannot tell "renders everything it was given" from "renders a
// subset" — §6.4a's closing paragraph forbids exactly that gap, and a
// renderer that quietly truncated to the first hit would still pass a
// one-candidate fixture.
func TestJudgeReconstructsPriorRecallsAsAssistantAndToolMessages(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", srv.Client())

	in := loop.JudgeInput{
		System: "sys", Block: "block", Input: "in",
		PriorRecalls: []loop.RecallExchange{
			{Query: "first query", Results: []loop.Candidate{
				{ID: 5, Type: "task", Name: "Found", Content: "found body"},
				{ID: 9, Type: "documentation", Name: "Also Found", Content: "second found body"},
			}},
			{Error: "tool arguments could not be parsed"},
		},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
			ToolCalls  []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}

	// system, user, then 2 rounds * (assistant + tool) = 6 total.
	if len(got.Messages) != 6 {
		t.Fatalf("messages has %d entries, want 6", len(got.Messages))
	}

	round1Assistant, round1Tool := got.Messages[2], got.Messages[3]
	if round1Assistant.Role != "assistant" || len(round1Assistant.ToolCalls) != 1 {
		t.Fatalf("messages[2] = %+v, want an assistant message with one tool call", round1Assistant)
	}
	if round1Assistant.ToolCalls[0].Function.Name != recallToolName {
		t.Fatalf("messages[2] tool call function = %q, want %q", round1Assistant.ToolCalls[0].Function.Name, recallToolName)
	}
	if !contains(round1Assistant.ToolCalls[0].Function.Arguments, "first query") {
		t.Fatalf("messages[2] tool call arguments = %q, want it to carry the original query", round1Assistant.ToolCalls[0].Function.Arguments)
	}
	if round1Tool.Role != "tool" || round1Tool.ToolCallID != round1Assistant.ToolCalls[0].ID {
		t.Fatalf("messages[3] = %+v, want a tool message whose tool_call_id matches messages[2]'s call id", round1Tool)
	}
	if !contains(round1Tool.Content, "found body") {
		t.Fatalf("messages[3].Content = %q, want it to carry the recalled body", round1Tool.Content)
	}
	if !contains(round1Tool.Content, "second found body") {
		t.Fatalf("messages[3].Content = %q, want it to carry the SECOND candidate's body too — a renderer that truncated to one hit would still pass without this assertion (W-1)", round1Tool.Content)
	}

	round2Tool := got.Messages[5]
	if !contains(round2Tool.Content, "tool arguments could not be parsed") {
		t.Fatalf("messages[5].Content = %q, want the error surfaced to the model", round2Tool.Content)
	}
}

func TestJudgeDecodesATextOnlyResponseAsAnswered(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[{"message":{"content":"hello there"},"finish_reason":"stop"}]}`)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want Answered", result.Reason)
	}
	if result.Answer != "hello there" {
		t.Fatalf("Answer = %q, want %q", result.Answer, "hello there")
	}
	if result.RawReason != "stop" {
		t.Fatalf("RawReason = %q, want %q", result.RawReason, "stop")
	}
}

func TestJudgeDecodesATruncatedResponse(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[{"message":{"content":"cut off"},"finish_reason":"length"}]}`)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.Truncated {
		t.Fatalf("Reason = %q, want Truncated", result.Reason)
	}
}

func TestJudgeDecodesAContentFilterResponseAsRefused(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[{"message":{"content":null},"finish_reason":"content_filter"}]}`)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.Refused {
		t.Fatalf("Reason = %q, want Refused", result.Reason)
	}
	if result.Answer != "" {
		t.Fatalf("Answer = %q, want empty for a null content field", result.Answer)
	}
}

// TestJudgeMapsAnUnknownFinishReasonToUnrecognisedAndPreservesTheRawValue
// pins design §6.6's D5: M1 depends on the field existing, not on its
// values — an endpoint inventing its own vocabulary must not error the
// call, and the raw string must survive for the record.
func TestJudgeMapsAnUnknownFinishReasonToUnrecognisedAndPreservesTheRawValue(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[{"message":{"content":"?"},"finish_reason":"some-vendor-reason"}]}`)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want Unrecognised", result.Reason)
	}
	if result.RawReason != "some-vendor-reason" {
		t.Fatalf("RawReason = %q, want the raw value preserved", result.RawReason)
	}
}

// TestJudgeDecodesAToolCallAsWantsRecallWithTheParsedQuery pins the
// presence-of-a-tool-call signal (design §6.6 D6): WantsRecall follows the
// tool call, not the finish_reason string.
func TestJudgeDecodesAToolCallAsWantsRecallWithTheParsedQuery(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"the missing thing\"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want WantsRecall", result.Reason)
	}
	if result.RecallQuery != "the missing thing" {
		t.Fatalf("RecallQuery = %q, want %q", result.RecallQuery, "the missing thing")
	}
	if result.RecallError != "" {
		t.Fatalf("RecallError = %q, want empty for a well-formed tool call", result.RecallError)
	}
}

// TestJudgeFlagsUnparseableToolArgumentsAsAMalformedRecallRequest pins
// design §6.4: a malformed tool input must not be a Judge error — it is
// an error-flagged result the loop dispatches as a recorded round.
func TestJudgeFlagsUnparseableToolArgumentsAsAMalformedRecallRequest(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"not json"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want WantsRecall", result.Reason)
	}
	if result.RecallError == "" {
		t.Fatal("RecallError is empty, want the parse failure surfaced")
	}
	if result.RecallQuery != "" {
		t.Fatalf("RecallQuery = %q, want empty when the arguments did not parse", result.RecallQuery)
	}
}

// TestJudgeFlagsAnEmptyQueryArgumentAsAMalformedRecallRequest covers the
// well-formed-JSON-but-useless-value case: {"query":""} parses fine but
// carries nothing to search for.
func TestJudgeFlagsAnEmptyQueryArgumentAsAMalformedRecallRequest(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"   \"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.RecallError == "" {
		t.Fatal("RecallError is empty, want an empty/whitespace-only query flagged")
	}
}

// TestJudgeLeavesUsageAbsentWhenTheResponseHasNoUsageObject and
// TestJudgeDecodesUsageWhenPresentEvenIfAllZero together pin design §6.5:
// usage is absent, never zero-filled. A decoder that defaults a missing
// usage object to a populated struct of zeros would pass the first test
// only by coincidence and would fail to distinguish it from the second.
func TestJudgeLeavesUsageAbsentWhenTheResponseHasNoUsageObject(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Usage != nil {
		t.Fatalf("Usage = %+v, want nil when the response carries no usage object", result.Usage)
	}
}

func TestJudgeDecodesUsageWhenPresentEvenIfAllZero(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":"a"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Usage == nil {
		t.Fatal("Usage is nil, want a present-but-zero usage object decoded as present")
	}
}

func TestJudgeDecodesNonZeroUsage(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":"a"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	// W-4: named for the direction of travel — in/out — and there is no
	// total. total_tokens (15 in the fixture) must not survive into
	// either field: it is the sum of the other two on every endpoint that
	// reports it, so the ruling drops it rather than let an adapter
	// fabricate it on an endpoint that reports two counts and no total.
	if result.Usage == nil || result.Usage.InTokens != 10 || result.Usage.OutTokens != 5 {
		t.Fatalf("Usage = %+v, want {InTokens:10 OutTokens:5}", result.Usage)
	}
}

// TestJudgeTreatsAUsageObjectReportingOnlyOneCountAsAbsentInWhole pins
// design §8.3's revision 3 ruling (W-4): an endpoint reporting one count
// and not the other has not been observed on any runtime, so until one is,
// such an object is recorded absent in whole rather than half zero-filled.
func TestJudgeTreatsAUsageObjectReportingOnlyOneCountAsAbsentInWhole(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":"a"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10}}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Usage != nil {
		t.Fatalf("Usage = %+v, want nil — a usage object reporting only one of the two counts must not be half zero-filled", result.Usage)
	}
}

// TestJudgeOnNon2xxReturnsAnError pins the status check itself (W-2). The
// fixture body deliberately decodes as a perfectly valid completion — the
// previous 401 fixture had zero choices, so `translate` errored on the
// empty-choices path regardless of whether the status check ran at all,
// and the test passed for the wrong reason (M18: disabling the status
// check outright left this test green). A 500 whose body is a valid
// `chatResponse` can only fail here if the status is actually checked.
// TestJudgeReadsOnlyTheFirstChoiceIgnoringAnyOthers pins D4 (design §6.6):
// "reads the first choice and no other." Every other fixture in this file
// has exactly one choice, so none of them can discriminate an
// implementation that read the wrong index (W-5) — this one carries two
// choices with different content and a different finish_reason so a wrong
// index fails both assertions.
func TestJudgeReadsOnlyTheFirstChoiceIgnoringAnyOthers(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":"first choice answer"},"finish_reason":"stop"},{"message":{"content":"second choice answer"},"finish_reason":"length"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Answer != "first choice answer" {
		t.Fatalf("Answer = %q, want the first choice's answer, not the second's", result.Answer)
	}
	if result.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want Answered (the first choice's finish_reason, not the second choice's Truncated)", result.Reason)
	}
}

// TestJudgeDecodesAToolCallAsWantsRecallEvenWhenFinishReasonSaysStop pins
// the ruling on WantsRecall's discriminator (design §6.6 D5/D6, QA's M10):
// several local runtimes send finish_reason:"stop" alongside a populated
// tool_calls array, so the decision must follow the tool call's presence,
// not the finish_reason string. Every other tool-call fixture in this file
// sets finish_reason:"tool_calls" together with the tool call, so none of
// them can discriminate a finish_reason-driven implementation from a
// tool-call-presence one — this is the fixture that can.
func TestJudgeDecodesAToolCallAsWantsRecallEvenWhenFinishReasonSaysStop(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"the missing thing\"}"}}]},"finish_reason":"stop"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want WantsRecall — a tool call's presence must win even when finish_reason says %q", result.Reason, "stop")
	}
	if result.RecallQuery != "the missing thing" {
		t.Fatalf("RecallQuery = %q, want %q", result.RecallQuery, "the missing thing")
	}
}

func TestJudgeOnNon2xxReturnsAnError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(stopResponse))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "m", "", srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{}); err == nil {
		t.Fatal("Judge returned nil error for a 500 response with a valid completion body, want an error from the status check")
	}
}

func TestJudgeOnUnreachableHostReturnsAnError(t *testing.T) {
	t.Parallel()

	c := NewClient("http://127.0.0.1:1", "m", "", nil)
	if _, err := c.Judge(context.Background(), loop.JudgeInput{}); err == nil {
		t.Fatal("Judge returned nil error against an unreachable host, want an error")
	}
}

func TestJudgeOnEmptyChoicesReturnsAnError(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[]}`)
	c := NewClient(srv.URL, "m", "", srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{}); err == nil {
		t.Fatal("Judge returned nil error for a response with no choices, want an error")
	}
}

// TestNewClientDefaultsToATimeoutBoundHTTPClientWhenNoneIsSupplied pins
// design §8.4a: the model adapter's timeout is its own constant — never
// http.DefaultClient, which has none.
func TestNewClientDefaultsToATimeoutBoundHTTPClientWhenNoneIsSupplied(t *testing.T) {
	t.Parallel()

	c := NewClient("http://example.invalid", "m", "k", nil)
	if c.httpClient == http.DefaultClient {
		t.Fatal("NewClient(nil) used http.DefaultClient, which has no timeout")
	}
	if c.httpClient.Timeout <= 0 {
		t.Fatalf("httpClient.Timeout = %v, want a positive timeout", c.httpClient.Timeout)
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Fatalf("httpClient.Timeout = %v, want DefaultTimeout (%v)", c.httpClient.Timeout, DefaultTimeout)
	}
}

// TestDefaultTimeoutIsNotDivoidsTimeout pins design §8.4a's whole point
// against a literal, not merely against its own constant (which would
// move with any mutation to its value and catch nothing): the model
// adapter's timeout must be generous — sized for a slow local
// generation — and specifically must not be 15 seconds, which is
// divoid.DefaultTimeout and would turn the ruling's own target (a local
// model on CPU, which can take minutes) into a service that never
// completes a run.
func TestDefaultTimeoutIsNotDivoidsTimeout(t *testing.T) {
	t.Parallel()

	const divoidDefaultTimeout = 15 * time.Second
	if DefaultTimeout == divoidDefaultTimeout {
		t.Fatalf("DefaultTimeout = %v, must not equal divoid.DefaultTimeout (%v) — design §8.4a requires its own, more generous constant", DefaultTimeout, divoidDefaultTimeout)
	}
	if DefaultTimeout < time.Minute {
		t.Fatalf("DefaultTimeout = %v, want at least a minute — generous by intent for a slow local generation", DefaultTimeout)
	}
}
