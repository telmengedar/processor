package ports

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/boot"
)

func statedLine(t *testing.T, path, protocol string) string {
	t.Helper()

	var logged bytes.Buffer
	if err := StateThinkingSuppression(slog.New(slog.NewTextHandler(&logged, nil)), path, protocol); err != nil {
		t.Fatalf("stating the suppression in force for %q failed: %v", protocol, err)
	}
	line := logged.String()
	if strings.Count(strings.TrimRight(line, "\n"), "\n") != 0 {
		t.Fatalf("boot stated %d lines, want exactly one an operator can read at a glance; log=%s", strings.Count(line, "\n"), line)
	}
	return line
}

func TestTheControlInForceOnTheNativeOllamaProtocolIsThatProtocolsOwnThinkKey(t *testing.T) {
	t.Parallel()

	suppression, err := ThinkingSuppression(boot.ProtocolOllama)
	if err != nil {
		t.Fatalf("ThinkingSuppression: %v", err)
	}
	if suppression.Control != "think=false" {
		t.Fatalf("control = %q, want %q: the native protocol defines the key, and the openai-compatible spelling is silently ignored here", suppression.Control, "think=false")
	}
	if suppression.Standing != "protocol key" {
		t.Fatalf("standing = %q, want %q: this control is the native protocol's own, and a standing that reads like the other adapter's states the inverse of what is true here", suppression.Standing, "protocol key")
	}
	if strings.Contains(suppression.Standing, "endpoint capability") {
		t.Fatalf("standing = %q calls the native protocol's own key an endpoint capability", suppression.Standing)
	}
}

func TestTheControlInForceOnTheOpenAICompatProtocolIsAnEndpointCapabilityAndIsNotStatedAsProtocol(t *testing.T) {
	t.Parallel()

	suppression, err := ThinkingSuppression(boot.ProtocolOpenAICompat)
	if err != nil {
		t.Fatalf("ThinkingSuppression: %v", err)
	}
	if suppression.Control != "reasoning_effort=none" {
		t.Fatalf("control = %q, want %q: the other two spellings this protocol accepts are taken and silently ignored", suppression.Control, "reasoning_effort=none")
	}
	if !strings.Contains(suppression.Standing, "endpoint capability") {
		t.Fatalf("standing = %q, want it to state that the endpoint provides this control, not the protocol", suppression.Standing)
	}
	if suppression.Standing == "protocol key" {
		t.Fatalf("standing = %q presents an endpoint extension as protocol, which is the one thing this control may never claim", suppression.Standing)
	}
}

func TestTheTwoProtocolsCarryDifferentControlsBecauseEachProtocolIgnoresTheOthersSpelling(t *testing.T) {
	t.Parallel()

	native, err := ThinkingSuppression(boot.ProtocolOllama)
	if err != nil {
		t.Fatalf("ThinkingSuppression: %v", err)
	}
	compat, err := ThinkingSuppression(boot.ProtocolOpenAICompat)
	if err != nil {
		t.Fatalf("ThinkingSuppression: %v", err)
	}
	if native.Control == compat.Control {
		t.Fatalf("both protocols report %q as the control in force, and one of them is therefore sending a control its endpoint ignores", native.Control)
	}
	if native.Standing == compat.Standing {
		t.Fatalf("both protocols report the standing %q, so the boot line cannot tell an operator which of the two is an endpoint extension and which is the protocol's own key", native.Standing)
	}
}

func TestNoControlIsReportedInForceForAProtocolWithNoAdapter(t *testing.T) {
	t.Parallel()

	suppression, err := ThinkingSuppression("responses")
	if err == nil {
		t.Fatalf("a protocol with no adapter reported %+v as the control in force, want a refusal rather than a claim nothing sends", suppression)
	}
	if !strings.Contains(err.Error(), "responses") {
		t.Fatalf("error = %q, want it to name the protocol it was given", err.Error())
	}
}

func TestBootStatesTheControlInForceOnOneLineNamingTheProtocolTheControlAndItsStanding(t *testing.T) {
	t.Parallel()

	line := statedLine(t, "judgement", boot.ProtocolOpenAICompat)

	for _, want := range []string{"judgement", boot.ProtocolOpenAICompat, "reasoning_effort=none", "endpoint capability"} {
		if !strings.Contains(line, want) {
			t.Fatalf("the boot line does not carry %q, so an operator reading it cannot tell whether suppression is active or by what means; line=%s", want, line)
		}
	}
}

func TestTheStatedLineFollowsTheProtocolItWasGivenRatherThanNamingOneControlForBoth(t *testing.T) {
	t.Parallel()

	line := statedLine(t, "judgement", boot.ProtocolOllama)

	if !strings.Contains(line, "think=false") {
		t.Fatalf("the boot line for the native protocol does not name its control; line=%s", line)
	}
	if strings.Contains(line, "reasoning_effort") {
		t.Fatalf("the boot line for the native protocol names the other protocol's control; line=%s", line)
	}
}

func TestBootStatesNothingAndReportsWhyForAProtocolWithNoAdapter(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	err := StateThinkingSuppression(slog.New(slog.NewTextHandler(&logged, nil)), "judgement", "responses")
	if err == nil {
		t.Fatal("boot stated a suppression for a protocol with no adapter, want a refusal")
	}
	if logged.Len() != 0 {
		t.Fatalf("boot stated %q for a protocol with no adapter, which claims a control nothing sends", logged.String())
	}
}

func TestTheStatedLineNamesTheCallPathTheControlGovernsSoTwoConfiguredProtocolsAreTellableApart(t *testing.T) {
	t.Parallel()

	judgement := statedLine(t, "judgement", boot.ProtocolOllama)
	fill := statedLine(t, "fill", boot.ProtocolOpenAICompat)

	if !strings.Contains(judgement, "path=judgement") || !strings.Contains(fill, "path=fill") {
		t.Fatalf("the stated lines do not name the call path each control governs, so a deployment running two protocols cannot be read off the log; judgement=%s fill=%s", judgement, fill)
	}
	if strings.Contains(judgement, "reasoning_effort") || strings.Contains(fill, "think=false") {
		t.Fatalf("a stated line names the control belonging to the other path; judgement=%s fill=%s", judgement, fill)
	}
}
