package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const nativeCondenseOK = `{"model":"ai/gemma4-served","message":{"role":"assistant","content":"the condensation"},"done":true,"done_reason":"stop"}`

func condenseOnce(t *testing.T, c *Client, maxOutputTokens int) CondenseResult {
	t.Helper()
	result, err := c.Condense(context.Background(), "condense this", maxOutputTokens)
	if err != nil {
		t.Fatalf("Condense: %v", err)
	}
	return result
}

func TestTheNativeCondensationCallPostsToTheSameChatRouteTheJudgementCallUses(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	if captured.Path != "/api/chat" {
		t.Fatalf("path = %q, want %q", captured.Path, "/api/chat")
	}
}

func TestTheNativeCondensationCallSwitchesThinkingOffSoNoReasoningStreamDrawsOnTheOutputBudget(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	raw, present := topLevelKeys(t, captured.Body)["think"]
	if !present {
		t.Fatalf("request carries no think key, so a thinking model reasons at the output budget's expense; body=%s", captured.Body)
	}
	var think bool
	if err := json.Unmarshal(raw, &think); err != nil {
		t.Fatalf("think is not a boolean: %v; raw=%s", err, raw)
	}
	if think {
		t.Fatalf("think = true, want false")
	}
}

func TestTheNativeJudgementCallSendsNoThinkKeyBecauseOnlyTheCondensationCallSuppressesReasoning(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	if _, present := topLevelKeys(t, captured.Body)["think"]; present {
		t.Fatalf("the judgement request carries a think key, which the condensation path alone was meant to set; body=%s", captured.Body)
	}
}

func TestTheNativeCondensationCallOffersNoToolSoTheModelCannotAnswerWithARecallRequest(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	if _, present := topLevelKeys(t, captured.Body)["tools"]; present {
		t.Fatalf("the condensation call must carry no tools; body=%s", captured.Body)
	}
}

func TestTheNativeCondensationCallSendsTheWholePromptAsASingleUserMessageAndStreamFalse(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4-requested", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	var sent struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &sent); err != nil {
		t.Fatalf("the request body did not decode: %v; body=%s", err, captured.Body)
	}

	if sent.Model != "ai/gemma4-requested" {
		t.Fatalf("model = %q, want %q", sent.Model, "ai/gemma4-requested")
	}
	if sent.Stream {
		t.Fatalf("stream = true; the pass reads one JSON object and never a stream")
	}
	if len(sent.Messages) != 1 {
		t.Fatalf("want exactly one message, got %d; body=%s", len(sent.Messages), captured.Body)
	}
	if sent.Messages[0].Role != "user" || sent.Messages[0].Content != "condense this" {
		t.Fatalf("message = %+v, want the whole prompt as one user message", sent.Messages[0])
	}
}

func TestTheNativeCondensationCallSendsTheCallersOutputCeilingAsNumPredict(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 23810)

	if _, present := topLevelKeys(t, captured.Body)["max_tokens"]; present {
		t.Fatalf("request carries max_tokens, which the native protocol does not read; body=%s", captured.Body)
	}
	var numPredict int
	if err := json.Unmarshal(decodedOptions(t, captured.Body)["num_predict"], &numPredict); err != nil {
		t.Fatalf("options carries no numeric num_predict: %v; body=%s", err, captured.Body)
	}
	if numPredict != 23810 {
		t.Fatalf("options.num_predict = %d, want 23810", numPredict)
	}
}

func TestBothRepetitionPenaltiesAreSentInsideTheNativeOptionsAtZeroBecauseAnyNonZeroValueShedsRepeatedIdentifiers(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	options := decodedOptions(t, captured.Body)
	for _, key := range []string{"frequency_penalty", "presence_penalty"} {
		raw, present := options[key]
		if !present {
			t.Fatalf("options omits %q, so the endpoint's own default applies; options=%v", key, options)
		}
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("options.%s is not numeric: %v; raw=%s", key, err, raw)
		}
		if value != 0 {
			t.Fatalf("options.%s = %v, want 0", key, value)
		}
	}
}

func TestTheConfiguredSamplingReachesTheNativeOptionsAndAnAbsentTopPIsOmittedRatherThanZeroed(t *testing.T) {
	t.Parallel()

	temperature := 0.375

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{Temperature: &temperature}, srv.Client())

	condenseOnce(t, c, 4096)

	options := decodedOptions(t, captured.Body)
	var gotTemperature float64
	if err := json.Unmarshal(options["temperature"], &gotTemperature); err != nil {
		t.Fatalf("options carries no numeric temperature: %v; options=%v", err, options)
	}
	if gotTemperature != temperature {
		t.Fatalf("options.temperature = %v, want %v", gotTemperature, temperature)
	}
	if _, present := options["top_p"]; present {
		t.Fatalf("an unset top_p must be omitted, not sent as zero; options=%v", options)
	}
}

func TestTheNativeSamplingIsReportedBackFromTheRequestBytesRatherThanFromConfiguration(t *testing.T) {
	t.Parallel()

	temperature := 0.375
	topP := 0.875

	srv, _ := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{Temperature: &temperature, TopP: &topP}, srv.Client())

	result := condenseOnce(t, c, 23810)

	if result.Sampling.Temperature == nil || *result.Sampling.Temperature != temperature {
		t.Fatalf("want temperature %v read back, got %v", temperature, result.Sampling.Temperature)
	}
	if result.Sampling.TopP == nil || *result.Sampling.TopP != topP {
		t.Fatalf("want top_p %v read back, got %v", topP, result.Sampling.TopP)
	}
	if result.Sampling.MaxTokens != 23810 {
		t.Fatalf("want the output ceiling read back, got %d", result.Sampling.MaxTokens)
	}
	if result.Sampling.FrequencyPenalty != 0 || result.Sampling.PresencePenalty != 0 {
		t.Fatalf("want both penalties read back at zero, got %v and %v", result.Sampling.FrequencyPenalty, result.Sampling.PresencePenalty)
	}
}

func TestTheNativeCondensationTakesTheModelAndTheTextFromTheEndpointsAnswerRatherThanFromWhatWasAskedFor(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4-requested", "", loop.Sampling{}, srv.Client())

	result := condenseOnce(t, c, 4096)

	if result.Model != "ai/gemma4-served" {
		t.Fatalf("want the model the endpoint reported, got %q", result.Model)
	}
	if result.Text != "the condensation" {
		t.Fatalf("want the message content as the condensation, got %q", result.Text)
	}
}

func TestTheNativeDoneReasonIsCarriedThroughAsTheFinishReasonUntranslated(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"model":"m","message":{"content":"cut off"},"done":true,"done_reason":"length"}`)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result := condenseOnce(t, c, 4096)

	if result.FinishReason != "length" {
		t.Fatalf("want the endpoint's own done reason, got %q", result.FinishReason)
	}
}

func TestANonSuccessStatusFromTheNativeEndpointSurfacesItsErrorStringRatherThanAnEmptyCondensation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"model does not support thinking"}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	_, err := c.Condense(context.Background(), "condense this", 4096)
	if err == nil {
		t.Fatalf("want an error when the native endpoint refuses the call")
	}
	if got := err.Error(); !strings.Contains(got, "model does not support thinking") {
		t.Fatalf("want the endpoint's own message surfaced, got %q", got)
	}
}

func TestTheNativeCondensationCallSendsNoAuthorizationHeaderWhenTheEndpointNeedsNoKey(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	if captured.Auth != "" {
		t.Fatalf("Authorization = %q, want it absent when no key is configured", captured.Auth)
	}
}

func TestTheNativeCondensationCallSendsAuthorizationBearerWhenAKeyIsConfigured(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "m", "a-key", loop.Sampling{}, srv.Client())

	condenseOnce(t, c, 4096)

	if captured.Auth != "Bearer a-key" {
		t.Fatalf("Authorization = %q, want %q", captured.Auth, "Bearer a-key")
	}
}
