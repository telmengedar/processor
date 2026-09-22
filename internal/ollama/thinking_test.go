package ollama

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/telmengedar/processor/internal/loop"
)

const nativeReasoningText = "first I weigh the block — then the question — and only then do I answer"

const nativeReasonedResponse = `{"message":{"role":"assistant","content":"the answer","thinking":"first I weigh the block — then the question — and only then do I answer"},"done":true,"done_reason":"stop"}`

func sentThinkKey(t *testing.T, body []byte) bool {
	t.Helper()
	raw, present := topLevelKeys(t, body)["think"]
	if !present {
		t.Fatalf("request carries no think key, so a thinking model reasons at the output budget's expense; body=%s", body)
	}
	var think bool
	if err := json.Unmarshal(raw, &think); err != nil {
		t.Fatalf("think is not a boolean: %v; raw=%s", err, raw)
	}
	return think
}

func TestTheNativeJudgementCallSwitchesThinkingOffSoNoReasoningStreamDrawsOnTheOutputBudget(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	if sentThinkKey(t, captured.Body) {
		t.Fatalf("think = true on the judgement request, want false; body=%s", captured.Body)
	}
}

func TestTheControlThisAdapterReportsByNameIsTheOneItsJudgementRequestActuallyCarries(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	key, value, split := strings.Cut(ThinkingSuppression, "=")
	if !split {
		t.Fatalf("ThinkingSuppression = %q, want a key=value pair an operator can look for in a request", ThinkingSuppression)
	}
	raw, present := topLevelKeys(t, captured.Body)[key]
	if !present {
		t.Fatalf("the boot line names %q as the control in force but the request carries no %q key; body=%s", ThinkingSuppression, key, captured.Body)
	}
	if got := strings.Trim(string(raw), `"`); got != value {
		t.Fatalf("the boot line names %q but the request sends %s=%s, so the line an operator reads is not what the endpoint is told", ThinkingSuppression, key, got)
	}
}

func TestTheNativeAdapterReportsTheReasoningAnEndpointReturnedWhenItTookTheSuppressionControlAndIgnoredIt(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeReasonedResponse)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if sentThinkKey(t, captured.Body) {
		t.Fatalf("this endpoint was meant to be told thinking is off and then ignore it; body=%s", captured.Body)
	}
	if len(nativeReasoningText) == utf8.RuneCountInString(nativeReasoningText) {
		t.Fatalf("the fixture is pure ASCII, so it cannot tell a byte count from a rune count; reasoning=%q", nativeReasoningText)
	}
	if result.ReasoningBytes != len(nativeReasoningText) {
		t.Fatalf("ReasoningBytes = %d, want %d: an endpoint that takes the control and reasons anyway is indistinguishable from one that honoured it unless the channel is read",
			result.ReasoningBytes, len(nativeReasoningText))
	}
}

func TestTheNativeAdapterLeavesTheReasoningOutOfTheAnswerWhenTheEndpointIgnoredTheSuppressionControl(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, nativeReasonedResponse)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Answer != "the answer" {
		t.Fatalf("Answer = %q, want the content the endpoint returned: the reasoning channel is read to assert it is empty, never to supply an answer", result.Answer)
	}
	if strings.Contains(result.Answer, nativeReasoningText) {
		t.Fatalf("Answer carries the reasoning text, which promotes a channel this step only asserts on; answer=%q", result.Answer)
	}
}

func TestTheNativeAdapterReportsNoReasoningWhenTheResponseCarriesNoneAndStillReturnsTheAnswer(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "ai/gemma4", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.ReasoningBytes != 0 {
		t.Fatalf("ReasoningBytes = %d on a response carrying no reasoning, want 0: a suppression that held must not be reported as a failure", result.ReasoningBytes)
	}
	if result.Answer != "the answer" || result.Reason != loop.Answered {
		t.Fatalf("result = %+v, want the ordinary answered outcome: suppression in force must not disturb a normal call", result)
	}
}
