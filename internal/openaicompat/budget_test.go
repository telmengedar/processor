package openaicompat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func sentWireKeys(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("the request body is not JSON: %v\n%s", err, body)
	}
	return raw
}

func TestTheOpenAICompatAdapterSendsNoMaxTokensKeyForAJudgementCallCarryingNoOutputLimit(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if value, present := sentWireKeys(t, captured.Body)["max_tokens"]; present {
		t.Fatalf("the request carries max_tokens=%v for a call with no output limit; absence is how the wire says none, and 0 may be read as generate nothing; body=%s", value, captured.Body)
	}
}

func TestTheOpenAICompatAdapterSendsTheLimitAClientGaveAsMaxTokensVerbatim(t *testing.T) {
	t.Parallel()

	const limit = 311

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: limit}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if got := sentWireKeys(t, captured.Body)["max_tokens"]; got != float64(limit) {
		t.Fatalf("max_tokens = %v, want %d; body=%s", got, limit, captured.Body)
	}
}

func TestTheOpenAICompatAdapterSendsNoMaxTokensKeyForACondensationOrDerivationCallCarryingNoOutputLimit(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, condenseOK)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	result, err := c.Condense(context.Background(), "condense this", 0)
	if err != nil {
		t.Fatalf("Condense: %v", err)
	}
	if _, present := sentWireKeys(t, captured.Body)["max_tokens"]; present {
		t.Fatalf("the condensation request carries max_tokens; body=%s", captured.Body)
	}
	if result.Sampling.MaxTokens != 0 {
		t.Fatalf("the read-back reports %d max tokens for a call that sent none", result.Sampling.MaxTokens)
	}

	if _, err := c.Derive(context.Background(), "derive from this", 0); err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if _, present := sentWireKeys(t, captured.Body)["max_tokens"]; present {
		t.Fatalf("the derivation request carries max_tokens; body=%s", captured.Body)
	}
}
