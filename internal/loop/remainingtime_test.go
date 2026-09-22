package loop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var guardInstant = time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)

func turnWithClock(model ModelPort, graph GraphPort, clock func() time.Time) *Turn {
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())
	turn.clock = clock
	return turn
}

func TestARunWhoseRemainingTimeCannotAffordOneJudgementCallMakesNoneAndNamesTheArithmetic(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{{Answer: "never asked for", Reason: Answered, RawReason: "stop"}}}
	turn := turnWithClock(model, baseGraph(), func() time.Time { return guardInstant })

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(5*time.Second))
	defer cancel()

	_, _, err := turn.Run(ctx, "hello", 42)

	if err == nil {
		t.Fatal("a run with five seconds left made a judgement call priced at tens of seconds and reported success")
	}
	if !errors.Is(err, ErrRunTimeExhausted) {
		t.Fatalf("err = %v, want it to wrap ErrRunTimeExhausted", err)
	}
	if errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("err = %v is filed as a model failure, but the model was never called: nothing about it was unavailable", err)
	}
	if len(model.calls) != 0 {
		t.Fatalf("the model was called %d times, want 0 — the guard exists so the call is not made at all", len(model.calls))
	}
	for _, want := range []string{"192 output tokens", "10 tokens per second", "80000 prompt bytes", "3000 bytes per second"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal does not state %q, so a reader cannot recompute what the run could not afford; err = %v", want, err)
		}
	}
}

func TestARunWhoseRemainingTimeAffordsAJudgementCallMakesIt(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := turnWithClock(model, baseGraph(), func() time.Time { return guardInstant })

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(10*time.Minute))
	defer cancel()

	record, _, err := turn.Run(ctx, "hello", 42)

	if err != nil {
		t.Fatalf("Run: %v — a run holding the whole run bound must not be refused by the remaining-time guard", err)
	}
	if record.Answer != "the answer" {
		t.Fatalf("record.Answer = %q, want the model's own answer", record.Answer)
	}
	if record.TimeShortfall != "" {
		t.Fatalf("record.TimeShortfall = %q on a run that had the whole run bound in front of it", record.TimeShortfall)
	}
	if record.Outcome.Curtailed {
		t.Fatalf("outcome = %+v, want a run nothing curtailed", record.Outcome)
	}
}

func TestARunThatRunsOutOfAffordableTimeMidTurnKeepsItsAnswerAndRecordsWhatStoppedIt(t *testing.T) {
	t.Parallel()

	instant := guardInstant
	model := &fakeModel{results: []JudgeResult{{Answer: "what it had so far", Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "more please"}}}
	model.beforeReturn = func() { instant = guardInstant.Add(9*time.Minute + 30*time.Second) }

	turn := turnWithClock(model, baseGraph(), func() time.Time { return instant })

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(10*time.Minute))
	defer cancel()

	record, _, err := turn.Run(ctx, "hello", 42)

	if err != nil {
		t.Fatalf("Run: %v — a turn that already has an answer degrades rather than failing", err)
	}
	if len(model.calls) != 1 {
		t.Fatalf("the model was called %d times, want 1 — the second call is the one the run could not afford", len(model.calls))
	}
	if record.Answer != "what it had so far" {
		t.Fatalf("record.Answer = %q, want the answer the affordable call produced", record.Answer)
	}
	if record.TimeShortfall == "" {
		t.Fatal("record.TimeShortfall is empty on a run the remaining-time guard stopped, so the record claims an ordinary ending")
	}
	if record.CapReached {
		t.Fatal("record.CapReached = true on a run that made one of six calls, so the record names the wrong bound")
	}
	if record.Outcome.Verdict != VerdictCurtailed {
		t.Fatalf("verdict = %q, want %q", record.Outcome.Verdict, VerdictCurtailed)
	}
	if got := record.TimeShortfall; !strings.Contains(got, "remaining time cannot afford") || !strings.Contains(got, "192 output tokens") {
		t.Fatalf("record.TimeShortfall reads %q, want it to name the guard and the arithmetic behind it", got)
	}
}

func TestTheRemainingTimeGuardStandsDownOnAContextThatCarriesNoDeadlineAtAll(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := turnWithClock(model, baseGraph(), func() time.Time { return guardInstant })

	record, _, err := turn.Run(context.Background(), "hello", 42)

	if err != nil {
		t.Fatalf("Run: %v — a context with no deadline states no remaining time, and a guard cannot price what is not stated", err)
	}
	if len(model.calls) != 1 {
		t.Fatalf("the model was called %d times, want 1", len(model.calls))
	}
	if record.TimeShortfall != "" {
		t.Fatalf("record.TimeShortfall = %q on a run whose context declared no deadline", record.TimeShortfall)
	}
}

func TestEveryJudgementCallCarriesTheJudgementSitesOwnBudget(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 1 {
		t.Fatalf("the model was called %d times, want 1", len(model.calls))
	}
	if got := model.calls[0].MaxOutputTokens; got != 192 {
		t.Fatalf("the judgement call asked for %d output tokens, want the judgement site's own budget of 192 rather than a budget the adapter reads for itself", got)
	}
}
