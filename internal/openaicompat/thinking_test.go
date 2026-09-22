package openaicompat

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/telmengedar/processor/internal/loop"
)

const compatReasoningText = "first I weigh the block — then the question — and only then do I answer"

const compatReasonedResponse = `{"choices":[{"message":{"content":"the answer","reasoning":"first I weigh the block — then the question — and only then do I answer"},"finish_reason":"stop"}]}`

func assertReasoningEffortNone(t *testing.T, body []byte) {
	t.Helper()
	raw, present := topLevelKeys(t, body)["reasoning_effort"]
	if !present {
		t.Fatalf("request carries no reasoning_effort key, so a reasoning model reasons at the output budget's expense; body=%s", body)
	}
	var effort string
	if err := json.Unmarshal(raw, &effort); err != nil {
		t.Fatalf("reasoning_effort is not a string: %v; raw=%s", err, raw)
	}
	if effort != "none" {
		t.Fatalf("reasoning_effort = %q, want %q: any other level still buys reasoning", effort, "none")
	}
}

func assertNoThinkKey(t *testing.T, body []byte) {
	t.Helper()
	if _, present := topLevelKeys(t, body)["think"]; present {
		t.Fatalf("the request carries a think key, which this protocol takes and silently ignores, leaving reasoning on while the log claims it is off; body=%s", body)
	}
}

func TestTheJudgementCallSendsTheEndpointsReasoningEffortControlSoNoReasoningDrawsOnTheOutputBudget(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	assertReasoningEffortNone(t, captured.Body)
}

func TestTheJudgementCallSendsNoThinkKeyBecauseThisProtocolTakesItAndIgnoresIt(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	assertNoThinkKey(t, captured.Body)
	if _, present := topLevelKeys(t, captured.Body)["reasoning_effort"]; !present {
		t.Fatalf("the same reading of the same body finds no reasoning_effort key either, so this absence proves nothing; body=%s", captured.Body)
	}
}

func TestTheCondensationCallSendsTheEndpointsReasoningEffortControlAndNoThinkKey(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, `{"model":"served","choices":[{"message":{"content":"the condensation"},"finish_reason":"stop"}]}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	condenseOnce(t, c)

	assertReasoningEffortNone(t, captured.Body)
	assertNoThinkKey(t, captured.Body)
}

func TestTheControlThisAdapterReportsByNameIsTheOneItsJudgementRequestActuallyCarries(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

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

func TestTheAdapterReportsTheReasoningAnEndpointReturnedWhenItTookTheSuppressionControlAndIgnoredIt(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, compatReasonedResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	assertReasoningEffortNone(t, captured.Body)
	if len(compatReasoningText) == utf8.RuneCountInString(compatReasoningText) {
		t.Fatalf("the fixture is pure ASCII, so it cannot tell a byte count from a rune count; reasoning=%q", compatReasoningText)
	}
	if result.ReasoningBytes != len(compatReasoningText) {
		t.Fatalf("ReasoningBytes = %d, want %d: an endpoint that takes the control and reasons anyway is indistinguishable from one that honoured it unless the channel is read",
			result.ReasoningBytes, len(compatReasoningText))
	}
}

func TestTheAdapterLeavesTheReasoningOutOfTheAnswerWhenTheEndpointIgnoredTheSuppressionControl(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, compatReasonedResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Answer != "the answer" {
		t.Fatalf("Answer = %q, want the content the endpoint returned: the reasoning channel is read to assert it is empty, never to supply an answer", result.Answer)
	}
	if strings.Contains(result.Answer, compatReasoningText) {
		t.Fatalf("Answer carries the reasoning text, which promotes a channel this step only asserts on; answer=%q", result.Answer)
	}
}

func TestTheAdapterReportsNoReasoningWhenTheResponseCarriesNoneAndStillReturnsTheAnswer(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.ReasoningBytes != 0 {
		t.Fatalf("ReasoningBytes = %d on a response carrying no reasoning, want 0: a suppression that held must not be reported as a failure", result.ReasoningBytes)
	}
	if result.Answer != "the answer" || result.Reason != loop.Answered {
		t.Fatalf("result = %+v, want the ordinary answered outcome: suppression in force must not disturb a normal call", result)
	}
}
