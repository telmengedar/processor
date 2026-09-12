package loop

import (
	"context"
	"testing"
	"time"
)

var promptClockBase = time.Date(2026, 9, 12, 8, 30, 0, 0, time.UTC)

func advancingClock(step time.Duration) func() time.Time {
	reads := 0
	return func() time.Time {
		at := promptClockBase.Add(time.Duration(reads) * step)
		reads++
		return at
	}
}

func promptClockGraph() *fakeGraph {
	return &fakeGraph{
		node:            Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		nodeFound:       true,
		candidates:      []Candidate{{ID: 7, Type: "task", Name: "Row", Similarity: 0.9, Content: "row body"}},
		writeRunReceipt: WriteReceipt{State: Stored, NodeID: 1},
	}
}

func TestEveryJudgementStepOfOneTurnStatesTheOneInstantTheClockWasReadFor(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "more please"},
		{Answer: "done", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(promptClockGraph(), model, nil, "system text", "test-model", testLogger())
	turn.clock = advancingClock(time.Hour)

	if _, _, err := turn.Run(context.Background(), "what changed today", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 2 {
		t.Fatalf("the turn made %d judgement calls, want 2 — a single-call turn cannot show whether the clock is read once", len(model.calls))
	}
	for i, call := range model.calls {
		if !call.Now.Equal(promptClockBase) {
			t.Fatalf("judgement call %d states %s, want %s: the prompt's instant must be read once for the turn, not once per call",
				i+1, call.Now.Format(time.RFC3339), promptClockBase.Format(time.RFC3339))
		}
	}
}

func TestATurnBuiltByNewTurnStatesARealInstantRatherThanTheZeroTime(t *testing.T) {
	t.Parallel()

	model := &fakeModel{}
	turn := NewTurn(promptClockGraph(), model, nil, "system text", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "what changed today", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 1 {
		t.Fatalf("the turn made %d judgement calls, want 1", len(model.calls))
	}
	if model.calls[0].Now.IsZero() {
		t.Fatalf("a turn built by NewTurn states no instant, so the shipped prompt would carry none")
	}
	if model.calls[0].Now.Before(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("a turn built by NewTurn states %s, which is not a wall-clock reading", model.calls[0].Now.Format(time.RFC3339))
	}
}
