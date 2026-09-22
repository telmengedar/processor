package loop

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func runWithReasoning(t *testing.T, reasoningBytes int) string {
	t.Helper()

	var logged bytes.Buffer
	model := &fakeModel{results: []JudgeResult{{
		Answer:         "the answer",
		Reason:         Answered,
		RawReason:      "stop",
		ReasoningBytes: reasoningBytes,
		Provider:       Provider{Adapter: "some-adapter"},
	}}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", slog.New(slog.NewTextHandler(&logged, nil)))

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return logged.String()
}

func lineCarrying(logged, substring string) (string, bool) {
	for _, line := range strings.Split(logged, "\n") {
		if strings.Contains(line, substring) {
			return line, true
		}
	}
	return "", false
}

func TestTheTurnReportsAResponseThatCarriedReasoningOnARequestThatSuppressedIt(t *testing.T) {
	t.Parallel()

	logged := runWithReasoning(t, 331)

	line, found := lineCarrying(logged, "reasoningBytes=331")
	if !found {
		t.Fatalf("no line reports how much arrived on the suppressed channel, so an endpoint that took the control and ignored it looks exactly like one that honoured it; log=%s", logged)
	}
	if !strings.Contains(line, "some-adapter") {
		t.Fatalf("the reporting line does not name the adapter whose control was ignored; line=%s", line)
	}
	if !strings.Contains(line, "level=WARN") {
		t.Fatalf("the reporting line is level %q, want WARN: at any lower level it reads as ordinary progress beside the run's own INFO lines; line=%s", levelOf(line), line)
	}
}

func levelOf(line string) string {
	for _, field := range strings.Fields(line) {
		if strings.HasPrefix(field, "level=") {
			return strings.TrimPrefix(field, "level=")
		}
	}
	return ""
}

func TestTheTurnReportsNothingWhenTheSuppressionHeldAndTheResponseCarriedNoReasoning(t *testing.T) {
	t.Parallel()

	logged := runWithReasoning(t, 0)

	if strings.Contains(logged, "reasoningBytes") {
		t.Fatalf("a response that legitimately carried no reasoning was reported as a failure; log=%s", logged)
	}
	if !strings.Contains(logged, "run finished") {
		t.Fatalf("the run did not finish, so the absence of the reasoning report proves nothing; log=%s", logged)
	}
}
