package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func condenseServer(t *testing.T, response string) (*httptest.Server, *[]byte) {
	t.Helper()

	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)

	return srv, &captured
}

func sentBody(t *testing.T, captured []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("the request body did not decode: %v", err)
	}
	return body
}

func floatPtr(f float64) *float64 { return &f }

const condenseOK = `{"model":"ai/gemma3","choices":[{"message":{"content":"the condensation"},"finish_reason":"stop"}]}`

func TestTheCondensationCallOffersNoToolSoTheModelCannotAnswerWithARecallRequest(t *testing.T) {
	srv, captured := condenseServer(t, condenseOK)
	client := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{}, srv.Client())

	if _, err := client.Condense(context.Background(), "condense this", 2048); err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	if _, present := sentBody(t, *captured)["tools"]; present {
		t.Fatalf("the condensation call must carry no tools, got %s", *captured)
	}
}

func TestTheCondensationCallSendsTheWholePromptAsASingleUserMessage(t *testing.T) {
	srv, captured := condenseServer(t, condenseOK)
	client := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{}, srv.Client())

	if _, err := client.Condense(context.Background(), "condense this", 2048); err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	messages, _ := sentBody(t, *captured)["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("want exactly one message, got %d", len(messages))
	}
	message, _ := messages[0].(map[string]any)
	if message["role"] != "user" || message["content"] != "condense this" {
		t.Fatalf("want the prompt as one user message, got %v", message)
	}
}

func TestBothRepetitionPenaltiesAreSentExplicitlyAtZeroBecauseAnyNonZeroValueShedsRepeatedIdentifiers(t *testing.T) {
	srv, captured := condenseServer(t, condenseOK)
	client := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{}, srv.Client())

	if _, err := client.Condense(context.Background(), "condense this", 2048); err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	body := sentBody(t, *captured)
	for _, key := range []string{"frequency_penalty", "presence_penalty"} {
		value, present := body[key]
		if !present {
			t.Fatalf("want %s sent explicitly, got %s", key, *captured)
		}
		if value != float64(0) {
			t.Fatalf("want %s pinned to 0, got %v", key, value)
		}
	}
}

func TestTheConfiguredSamplingReachesTheWireAndAnAbsentTopPIsOmittedRatherThanZeroed(t *testing.T) {
	srv, captured := condenseServer(t, condenseOK)
	client := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{Temperature: floatPtr(0)}, srv.Client())

	if _, err := client.Condense(context.Background(), "condense this", 4096); err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	body := sentBody(t, *captured)
	if body["temperature"] != float64(0) {
		t.Fatalf("want temperature 0 on the wire, got %v", body["temperature"])
	}
	if _, present := body["top_p"]; present {
		t.Fatalf("an unset top_p must be omitted, not sent as zero, got %s", *captured)
	}
	if body["max_tokens"] != float64(4096) {
		t.Fatalf("want the caller's output ceiling on the wire, got %v", body["max_tokens"])
	}
}

func TestTheSamplingIsReportedBackFromTheRequestBytesRatherThanFromConfiguration(t *testing.T) {
	srv, _ := condenseServer(t, condenseOK)
	client := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}, srv.Client())

	result, err := client.Condense(context.Background(), "condense this", 4096)
	if err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	if result.Sampling.Temperature == nil || *result.Sampling.Temperature != 0.2 {
		t.Fatalf("want temperature 0.2 read back, got %v", result.Sampling.Temperature)
	}
	if result.Sampling.TopP == nil || *result.Sampling.TopP != 0.9 {
		t.Fatalf("want top_p 0.9 read back, got %v", result.Sampling.TopP)
	}
	if result.Sampling.MaxTokens != 4096 {
		t.Fatalf("want the output ceiling read back, got %d", result.Sampling.MaxTokens)
	}
}

func TestTheModelIsTakenFromTheEndpointsAnswerRatherThanFromTheModelThatWasAskedFor(t *testing.T) {
	srv, _ := condenseServer(t, `{"model":"ai/gemma3-served","choices":[{"message":{"content":"the condensation"},"finish_reason":"stop"}]}`)
	client := NewClient(srv.URL, "ai/gemma3-requested", "", loop.Sampling{}, srv.Client())

	result, err := client.Condense(context.Background(), "condense this", 2048)
	if err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	if result.Model != "ai/gemma3-served" {
		t.Fatalf("want the model the endpoint reported, got %q", result.Model)
	}
}

func TestTheEndpointsFinishReasonIsCarriedThroughUntranslated(t *testing.T) {
	srv, _ := condenseServer(t, `{"model":"m","choices":[{"message":{"content":"cut off"},"finish_reason":"length"}]}`)
	client := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := client.Condense(context.Background(), "condense this", 2048)
	if err != nil {
		t.Fatalf("want the call to succeed, got %v", err)
	}

	if result.FinishReason != "length" {
		t.Fatalf("want the endpoint's own finish reason, got %q", result.FinishReason)
	}
}

func TestAResponseCarryingNoChoiceIsAnErrorRatherThanAnEmptyCondensation(t *testing.T) {
	srv, _ := condenseServer(t, `{"model":"m","choices":[]}`)
	client := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	if _, err := client.Condense(context.Background(), "condense this", 2048); err == nil {
		t.Fatalf("want an error when the endpoint returns no choice")
	}
}

func TestTheJudgementCallStillCarriesItsToolsSoTheCondensationPathChangedNothingForTheTurn(t *testing.T) {
	srv, captured := condenseServer(t, `{"choices":[{"message":{"content":"an answer"},"finish_reason":"stop"}]}`)
	client := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	if _, err := client.Judge(context.Background(), loop.JudgeInput{System: "s", Block: "b", Input: "i"}); err != nil {
		t.Fatalf("want the judgement call to succeed, got %v", err)
	}

	body := sentBody(t, *captured)
	if _, present := body["tools"]; !present {
		t.Fatalf("the turn's judgement call must still offer the recall tool, got %s", *captured)
	}
	if _, present := body["frequency_penalty"]; present {
		t.Fatalf("the turn's judgement call must not have gained the condensation penalties, got %s", *captured)
	}
}

func TestOpenAICompatDeriveReturnsTheCompletionTextAloneOverTheCondenseRequest(t *testing.T) {
	t.Parallel()

	srv, captured := condenseServer(t, condenseOK)
	c := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{}, srv.Client())

	text, err := c.Derive(context.Background(), "derive from this", 512)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if text != "the condensation" {
		t.Fatalf("Derive returned %q, want the completion text alone", text)
	}

	body := sentBody(t, *captured)
	if _, hasTools := body["tools"]; hasTools {
		t.Fatalf("the derivation request carried tools; a derivation is one user message and nothing the model can call")
	}
	if body["frequency_penalty"] != 0.0 || body["presence_penalty"] != 0.0 {
		t.Fatalf("the derivation request carried repetition penalties %v/%v, want both pinned to zero as the condensation request pins them", body["frequency_penalty"], body["presence_penalty"])
	}
	if body["max_tokens"] != float64(512) {
		t.Fatalf("the derivation request asked for %v output tokens, want the 512 the caller bounded it at", body["max_tokens"])
	}
}

func TestOpenAICompatDeriveReportsTheEndpointsOwnFailureRatherThanEmptyText(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "model is loading")
	}))
	t.Cleanup(srv.Close)

	text, err := NewClient(srv.URL, "ai/gemma3", "", loop.Sampling{}, srv.Client()).Derive(context.Background(), "derive from this", 512)
	if err == nil {
		t.Fatalf("Derive returned %q and no error for a 503; a non-2xx read as empty text is a fallback with no cause", text)
	}
	if !strings.Contains(err.Error(), "model is loading") {
		t.Fatalf("Derive reported %q, which drops the endpoint's own sentence", err)
	}
}
