package ollama

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func sentOptions(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var raw struct {
		Options map[string]any `json:"options"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("the request body is not JSON: %v\n%s", err, body)
	}
	return raw.Options
}

func TestTheNativeAdapterSendsNoNumPredictKeyForAJudgementCallCarryingNoOutputLimit(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if value, present := sentOptions(t, captured.Body)["num_predict"]; present {
		t.Fatalf("the request carries num_predict=%v for a call with no output limit; absence is how the wire says none, and 0 may be read as generate nothing; body=%s", value, captured.Body)
	}
}

func TestTheNativeAdapterSendsTheLimitAClientGaveAsNumPredictVerbatim(t *testing.T) {
	t.Parallel()

	const limit = 311

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: limit}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if got := sentOptions(t, captured.Body)["num_predict"]; got != float64(limit) {
		t.Fatalf("num_predict = %v, want %d; body=%s", got, limit, captured.Body)
	}
}

func TestTheNativeAdapterSendsNoNumPredictKeyForACondensationOrDerivationCallCarryingNoOutputLimit(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	result := condenseOnce(t, c, 0)
	if _, present := sentOptions(t, captured.Body)["num_predict"]; present {
		t.Fatalf("the condensation request carries num_predict; body=%s", captured.Body)
	}
	if result.Sampling.MaxTokens != 0 {
		t.Fatalf("the read-back reports %d max tokens for a call that sent none", result.Sampling.MaxTokens)
	}

	if _, err := c.Derive(context.Background(), "derive from this", 0); err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if _, present := sentOptions(t, captured.Body)["num_predict"]; present {
		t.Fatalf("the derivation request carries num_predict; body=%s", captured.Body)
	}
}
