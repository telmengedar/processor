package openaicompat

import (
	"context"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestTheOpenAICompatAdapterRefusesAJudgementCallCarryingNoOutputBudgetRatherThanAskingTheEndpointForZeroTokens(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	_, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})

	if err == nil {
		t.Fatal("a judgement call carrying no output budget reached the endpoint; a budget of zero is a cap of zero tokens, not an absent cap")
	}
	if !strings.Contains(err.Error(), "no output budget") {
		t.Fatalf("err = %v, want it to name the missing budget", err)
	}
	if captured.Body != nil {
		t.Fatalf("the endpoint received a request body of %d bytes, want none at all: %s", len(captured.Body), captured.Body)
	}
}

func TestTheOpenAICompatAdapterSendsTheBudgetTheCallSiteGaveItAsMaxTokensRatherThanOneOfItsOwn(t *testing.T) {
	t.Parallel()

	const siteBudget = 311

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: siteBudget}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	body := string(captured.Body)
	if !strings.Contains(body, `"max_tokens":311`) {
		t.Fatalf("the request does not carry the call site's own budget of %d; body=%s", siteBudget, body)
	}
}
