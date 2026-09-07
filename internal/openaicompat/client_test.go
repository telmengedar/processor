package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/action"
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

func TestJudgeRequestBodyCarriesModelSystemBlockAndInputAndOffersNoProviderTools(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "the system text", Block: "the block", Input: "the input"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(captured.Body, &raw); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if _, present := raw["tools"]; present {
		t.Fatalf("the request declares provider tools, want none: %s", captured.Body)
	}

	got := decodeMessages(t, captured.Body)

	const (
		wantMaxTokens         = 4096
		wantBlockInputContent = "the block\n===== INPUT =====\nthe input"
	)

	var envelope struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.Unmarshal(captured.Body, &envelope); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if envelope.Model != "the-model-id" {
		t.Fatalf("model = %q, want %q", envelope.Model, "the-model-id")
	}
	if envelope.MaxTokens != wantMaxTokens {
		t.Fatalf("max_tokens = %d, want %d", envelope.MaxTokens, wantMaxTokens)
	}
	if len(got) != 2 {
		t.Fatalf("messages has %d entries, want 2 (system, user)", len(got))
	}
	if got[0].Role != "system" || got[0].Content != "the system text" {
		t.Fatalf("messages[0] = %+v, want the system role carrying the system text verbatim", got[0])
	}
	if got[1].Role != "user" {
		t.Fatalf("messages[1].Role = %q, want %q", got[1].Role, "user")
	}
	if got[1].Content != wantBlockInputContent {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got[1].Content, wantBlockInputContent)
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

type sentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func decodeMessages(t *testing.T, body []byte) []sentMessage {
	t.Helper()
	var got struct {
		Messages []sentMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, body)
	}
	return got.Messages
}

func TestJudgePutsAPriorRoundBackAsAnActionBlockAndItsResultWithoutUsingTheToolRole(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{
		System: "sys", Block: "block", Input: "in",
		PriorTools: []loop.ToolExchange{
			{Round: 1, Tool: loop.ToolRecall, Query: "first query", Results: []loop.Candidate{
				{ID: 5, Type: "task", Name: "Found", Content: "found body"},
				{ID: 9, Type: "documentation", Name: "Also Found", Content: "second found body"},
			}},
			{Round: 2, Tool: loop.ToolRecall, Error: "action arguments could not be parsed"},
		},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	got := decodeMessages(t, captured.Body)
	if len(got) != 6 {
		t.Fatalf("messages has %d entries, want 6", len(got))
	}

	for i, m := range got {
		if m.Role == "tool" {
			t.Fatalf("messages[%d] uses the tool role, which this protocol does not use", i)
		}
	}

	if got[2].Role != "assistant" {
		t.Fatalf("messages[2].Role = %q, want assistant", got[2].Role)
	}
	if !contains(got[2].Content, action.Open) || !contains(got[2].Content, action.Close) {
		t.Fatalf("messages[2].Content = %q, want the round replayed as an action block", got[2].Content)
	}
	if !contains(got[2].Content, "first query") {
		t.Fatalf("messages[2].Content = %q, want it to carry the original query", got[2].Content)
	}
	if got[3].Role != "user" {
		t.Fatalf("messages[3].Role = %q, want user", got[3].Role)
	}
	if !contains(got[3].Content, "found body") {
		t.Fatalf("messages[3].Content = %q, want it to carry the recalled body", got[3].Content)
	}
	if !contains(got[3].Content, "second found body") {
		t.Fatalf("messages[3].Content = %q, want it to carry the SECOND candidate's body too — a renderer that truncated to one hit would still pass without this assertion", got[3].Content)
	}
	if !contains(got[5].Content, "action arguments could not be parsed") {
		t.Fatalf("messages[5].Content = %q, want the error surfaced to the model", got[5].Content)
	}
}

func TestJudgeReplaysTwoActionsOfOneRoundAsASingleAssistantTurnCarryingBothBlocks(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{
		System: "sys", Block: "block", Input: "in",
		PriorTools: []loop.ToolExchange{
			{Round: 1, Tool: loop.ToolWriteFile, Path: "index.html", Content: "<h1>a</h1>", Bytes: 10},
			{Round: 1, Tool: loop.ToolWriteFile, Path: "README.md", Content: "# a", Bytes: 3},
		},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	got := decodeMessages(t, captured.Body)
	if len(got) != 4 {
		t.Fatalf("messages has %d entries, want 4 — two actions of one round are one assistant turn and one result turn", len(got))
	}
	if strings.Count(got[2].Content, action.Open) != 2 {
		t.Fatalf("messages[2].Content = %q, want both action blocks in the one assistant turn", got[2].Content)
	}
	if !contains(got[3].Content, "index.html") || !contains(got[3].Content, "README.md") {
		t.Fatalf("messages[3].Content = %q, want both results", got[3].Content)
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
