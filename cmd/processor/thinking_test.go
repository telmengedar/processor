package main

import (
	"strings"
	"testing"
)

func bootWith(t *testing.T, protocol string) (int, string) {
	t.Helper()
	return bootWithFill(t, protocol, "")
}

func bootWithFill(t *testing.T, protocol, fillProtocol string) (int, string) {
	t.Helper()

	t.Setenv("PROCESSOR_HTTP_ADDR", "127.0.0.1:-1")
	t.Setenv("PROCESSOR_DIVOID_URL", "https://graph.invalid")
	t.Setenv("PROCESSOR_DIVOID_KEY", "a-key")
	t.Setenv("PROCESSOR_MODEL_URL", "http://model.invalid/v1")
	t.Setenv("PROCESSOR_MODEL_ID", "ai/test-requested")
	if protocol != "" {
		t.Setenv("PROCESSOR_MODEL_PROTOCOL", protocol)
	}
	if fillProtocol != "" {
		t.Setenv("PROCESSOR_CONDENSE_MODEL_URL", "http://fill.invalid/v1")
		t.Setenv("PROCESSOR_CONDENSE_MODEL_ID", "ai/test-condenser")
		t.Setenv("PROCESSOR_CONDENSE_MODEL_PROTOCOL", fillProtocol)
	}

	var human strings.Builder
	return run(&human), human.String()
}

func TestTheServiceStatesTheSuppressionControlInForceForItsConfiguredProtocol(t *testing.T) {
	code, human := bootWith(t, "")

	if code == 0 {
		t.Fatalf("the service bound an unbindable address, so this run says nothing about what it stated first; log=%s", human)
	}
	if !strings.Contains(human, "reasoning_effort=none") {
		t.Fatalf("the service states no suppression control at boot, so an operator cannot tell whether its model calls suppress reasoning; log=%s", human)
	}
}

func TestTheServiceStatesTheNativeSuppressionControlWhenTheNativeProtocolIsConfigured(t *testing.T) {
	code, human := bootWith(t, "ollama")

	if code == 0 {
		t.Fatalf("the service bound an unbindable address, so this run says nothing about what it stated first; log=%s", human)
	}
	if !strings.Contains(human, "think=false") {
		t.Fatalf("the service states the wrong protocol's suppression control at boot; log=%s", human)
	}
	if strings.Contains(human, "reasoning_effort") {
		t.Fatalf("the service names the other protocol's control alongside the native one; log=%s", human)
	}
}

func TestTheServiceStatesAControlForEveryProtocolItActuallySendsOnNotOnlyTheJudgementOne(t *testing.T) {
	code, human := bootWithFill(t, "ollama", "openai-compat")

	if code == 0 {
		t.Fatalf("the service bound an unbindable address, so this run says nothing about what it stated first; log=%s", human)
	}
	if !strings.Contains(human, "think=false") {
		t.Fatalf("the service states no control for its judgement protocol; log=%s", human)
	}
	if !strings.Contains(human, "reasoning_effort=none") {
		t.Fatalf("the fill runs on a second protocol whose control the service never states, so the log names one of two mechanisms in force; log=%s", human)
	}
	if !strings.Contains(human, "path=judgement") || !strings.Contains(human, "path=fill") {
		t.Fatalf("the two stated controls cannot be told apart by the path they govern; log=%s", human)
	}
}

func TestTheServiceStatesTheFillControlOnTheJudgementModelWhenNoSecondModelIsConfigured(t *testing.T) {
	code, human := bootWith(t, "ollama")

	if code == 0 {
		t.Fatalf("the service bound an unbindable address; log=%s", human)
	}
	if !strings.Contains(human, "path=judgement") {
		t.Fatalf("the service states no judgement control at all, so the absence of a fill one says nothing; log=%s", human)
	}
	if !strings.Contains(human, "path=fill") || !strings.Contains(human, "the fill runs on the judgement model's configuration") {
		t.Fatalf("the service states no fill control on the judgement model's configuration, so its fill calls carry a reasoning setting nobody read; log=%s", human)
	}
	for _, phrase := range []string{"a capable condense model is strongly recommended", "nothing checks a substance", "a written substance stays until the node's content changes"} {
		if !strings.Contains(human, phrase) {
			t.Fatalf("the boot line that says the fill runs on the judgement model does not carry %q, so an operator who skipped the condense model is never told that nothing checks a substance and a weak one stands; log=%s", phrase, human)
		}
	}
}
