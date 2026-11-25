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
	turn := NewTurn(baseGraph(), model, nil, fixedSystem("system"), "test-model", slog.New(slog.NewTextHandler(&logged, nil)))

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

func TestThinkingReachesCallCapAndPrivateDecision(t *testing.T) {
	model := &fakeModel{results: researchThenAnswer("done")}
	record := runWithView(t, model, &View{Config: 1, Thinking: true})
	if !record.CapReached || len(model.calls) != MaxModelCalls || len(model.calls[len(model.calls)-1].Offered) != 0 {
		t.Fatalf("did not reach reserved cap: %+v", record)
	}
	for i, call := range model.calls {
		if !call.Thinking {
			t.Errorf("call %d thinking=false", i)
		}
	}
	view := decisionView()
	view.Thinking = true
	model = &fakeModel{results: []JudgeResult{decisionResult(`{"action":"skip","text":"","about":null}`)}}
	runNotes(t, noteGraph(), view, &fakeNotes{}, model)
	if len(model.calls) != 1 || !model.calls[0].Thinking {
		t.Fatalf("private judgement=%+v", model.calls)
	}
}

func TestStructuredThinkingReachesToolSelectionAndReservedAnswer(t *testing.T) {
	model := &fakeModel{results: []JudgeResult{structuredRead(42), structuredAnswer("done")}}
	record := runWithView(t, model, &View{Config: 1, Thinking: true, ResponseMode: ResponseStructuredRead})
	if len(model.calls) != 2 || record.ModelCalls != 2 || len(model.calls[0].StructuredAllowedActions) < 2 || len(model.calls[1].StructuredAllowedActions) != 1 {
		t.Fatalf("no structured reserved path: %+v", model.calls)
	}
	for i, call := range model.calls {
		if !call.Thinking {
			t.Errorf("call %d thinking=false", i)
		}
	}
}

func TestRequestedThinkingDoesNotWarnThatSuppressionWasIgnored(t *testing.T) {
	var logged bytes.Buffer
	model := &fakeModel{results: []JudgeResult{{Answer: "The answer is complete.", Reason: Answered, RawReason: "stop", ReasoningBytes: 331}}}
	turn := NewTurn(baseGraph(), model, nil, fixedSystem("system"), "test-model", slog.New(slog.NewTextHandler(&logged, nil)))
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, Settings: DefaultSettings(), View: &View{Thinking: true}})
	if err != nil || record.LastResponse == nil || record.LastResponse.ReasoningBytes != 331 {
		t.Fatalf("accounting=%+v error=%v", record, err)
	}
	if strings.Contains(logged.String(), "ignored it") {
		t.Fatalf("requested reasoning logged as ignored suppression: %s", logged.String())
	}
}
