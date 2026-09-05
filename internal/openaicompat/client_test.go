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
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if captured.Path != "/chat/completions" {
		t.Fatalf("path = %q, want %q", captured.Path, "/chat/completions")
	}
}

func TestJudgeSendsNoAuthorizationHeaderWhenKeyIsEmpty(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

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
	c := NewClient(srv.URL, "model-x", "secret-key", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}
	const want = "Bearer secret-key"
	if captured.Auth != want {
		t.Fatalf("Authorization = %q, want %q", captured.Auth, want)
	}
}

func TestJudgeRequestBodyCarriesModelSystemBlockInputAndTool(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

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
	if got.Messages[1].Content != wantBlockInputContent {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got.Messages[1].Content, wantBlockInputContent)
	}
	if len(got.Tools) != 1 {
		t.Fatalf("tools has %d entries, want exactly 1 (design §2.4: one tool)", len(got.Tools))
	}
	if got.Tools[0].Type != "function" || got.Tools[0].Function.Name != wantToolName {
		t.Fatalf("tools[0] = %+v, want the recall function tool named %q", got.Tools[0], wantToolName)
	}
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

func float64Ptr(v float64) *float64 { return &v }

func TestJudgeRequestBodyOmitsTemperatureAndTopPWhenSamplingIsUnconfigured(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if _, present := got["temperature"]; present {
		t.Fatalf("request body carries %q, want it absent when Sampling.Temperature is nil; body=%s", "temperature", captured.Body)
	}
	if _, present := got["top_p"]; present {
		t.Fatalf("request body carries %q, want it absent when Sampling.TopP is nil; body=%s", "top_p", captured.Body)
	}
}

func TestJudgeRequestBodyCarriesTemperatureAndTopPWhenConfigured(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	sampling := loop.Sampling{Temperature: float64Ptr(0.37), TopP: float64Ptr(0.91)}
	c := NewClient(srv.URL, "m", "", sampling, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Temperature *float64 `json:"temperature"`
		TopP        *float64 `json:"top_p"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if got.Temperature == nil || *got.Temperature != 0.37 {
		t.Fatalf("temperature = %v, want 0.37", got.Temperature)
	}
	if got.TopP == nil || *got.TopP != 0.91 {
		t.Fatalf("top_p = %v, want 0.91", got.TopP)
	}
}

func TestJudgeRequestBodyCarriesAnExplicitZeroTemperatureRatherThanOmittingIt(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	sampling := loop.Sampling{Temperature: float64Ptr(0)}
	c := NewClient(srv.URL, "m", "", sampling, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	temperature, present := got["temperature"]
	if !present {
		t.Fatalf("request body omits temperature for an explicit zero value, want it sent as 0; body=%s", captured.Body)
	}
	if temperature != float64(0) {
		t.Fatalf("temperature = %v, want 0", temperature)
	}
}

func TestJudgeResultCarriesTheSamplingTheClientWasConfiguredWith(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, stopResponse)
	sampling := loop.Sampling{Temperature: float64Ptr(0.42)}
	c := NewClient(srv.URL, "m", "", sampling, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Sampling.Temperature == nil || *result.Sampling.Temperature != 0.42 {
		t.Fatalf("result.Sampling.Temperature = %v, want 0.42", result.Sampling.Temperature)
	}
	if result.Sampling.TopP != nil {
		t.Fatalf("result.Sampling.TopP = %v, want nil — TopP was never configured", result.Sampling.TopP)
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

func TestJudgeReconstructsPriorRecallsAsAssistantAndToolMessages(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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

	if len(got.Messages) != 6 {
		t.Fatalf("messages has %d entries, want 6", len(got.Messages))
	}

	round1Assistant, round1Tool := got.Messages[2], got.Messages[3]
	if round1Assistant.Role != "assistant" || len(round1Assistant.ToolCalls) != 1 {
		t.Fatalf("messages[2] = %+v, want an assistant message with one tool call", round1Assistant)
	}
	const wantRecallToolName = "recall"
	if round1Assistant.ToolCalls[0].Function.Name != wantRecallToolName {
		t.Fatalf("messages[2] tool call function = %q, want %q", round1Assistant.ToolCalls[0].Function.Name, wantRecallToolName)
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
		t.Fatalf("messages[3].Content = %q, want it to carry the SECOND candidate's body too — a renderer that truncated to one hit would still pass without this assertion", round1Tool.Content)
	}

	round2Tool := got.Messages[5]
	if !contains(round2Tool.Content, "tool arguments could not be parsed") {
		t.Fatalf("messages[5].Content = %q, want the error surfaced to the model", round2Tool.Content)
	}
}

func TestJudgeDecodesATextOnlyResponseAsAnswered(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[{"message":{"content":"hello there"},"finish_reason":"stop"}]}`)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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

func TestJudgeMapsAnUnknownFinishReasonToUnrecognisedAndPreservesTheRawValue(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[{"message":{"content":"?"},"finish_reason":"some-vendor-reason"}]}`)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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

func TestJudgeDecodesAToolCallAsWantsRecallWithTheParsedQuery(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"the missing thing\"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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

func TestJudgeFlagsUnparseableToolArgumentsAsAMalformedRecallRequest(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"not json"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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

func TestJudgeFlagsAnEmptyQueryArgumentAsAMalformedRecallRequest(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"   \"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.RecallError == "" {
		t.Fatal("RecallError is empty, want an empty/whitespace-only query flagged")
	}
}

func TestJudgeLeavesUsageAbsentWhenTheResponseHasNoUsageObject(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Usage == nil || result.Usage.InTokens != 10 || result.Usage.OutTokens != 5 {
		t.Fatalf("Usage = %+v, want {InTokens:10 OutTokens:5}", result.Usage)
	}
}

func TestJudgeTreatsAUsageObjectReportingOnlyOneCountAsAbsentInWhole(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":"a"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10}}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Usage != nil {
		t.Fatalf("Usage = %+v, want nil — a usage object reporting only one of the two counts must not be half zero-filled", result.Usage)
	}
}

func TestJudgeReadsOnlyTheFirstChoiceIgnoringAnyOthers(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":"first choice answer"},"finish_reason":"stop"},{"message":{"content":"second choice answer"},"finish_reason":"length"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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

func TestJudgeDecodesAToolCallAsWantsRecallEvenWhenFinishReasonSaysStop(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"the missing thing\"}"}}]},"finish_reason":"stop"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

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
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{}); err == nil {
		t.Fatal("Judge returned nil error for a 500 response with a valid completion body, want an error from the status check")
	}
}

func TestJudgeOnUnreachableHostReturnsAnError(t *testing.T) {
	t.Parallel()

	c := NewClient("http://127.0.0.1:1", "m", "", loop.Sampling{}, nil)
	if _, err := c.Judge(context.Background(), loop.JudgeInput{}); err == nil {
		t.Fatal("Judge returned nil error against an unreachable host, want an error")
	}
}

func TestJudgeOnEmptyChoicesReturnsAnError(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"choices":[]}`)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{}); err == nil {
		t.Fatal("Judge returned nil error for a response with no choices, want an error")
	}
}

func TestNewModelClientDefaultsToATimeoutBoundHTTPClientWhenNoneIsSupplied(t *testing.T) {
	t.Parallel()

	c := NewClient("http://example.invalid", "m", "k", loop.Sampling{}, nil)
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

func TestTheNilHTTPClientFallbackYieldsATimeoutBoundClientNotMerelyANonNilOne(t *testing.T) {
	t.Parallel()

	c := Client{}
	if got := c.client(); got.Timeout != DefaultTimeout {
		t.Fatalf("zero-value Client's fallback client Timeout = %v, want DefaultTimeout (%v) — an unbounded fallback trades a panic for a hang", got.Timeout, DefaultTimeout)
	}
}

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
