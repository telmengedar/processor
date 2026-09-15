package loop

import (
	"context"
	"encoding/json"
	"strings"
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

var recordInstantLocal = time.Date(2026, 9, 12, 10, 30, 0, 0, time.FixedZone("test+02", 2*60*60))

const recordInstantUTC = "2026-09-12T08:30:00Z"

func TestTheRunRecordCarriesTheSameInstantTheAssembledPromptStates(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "more please"},
		{Answer: "done", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(promptClockGraph(), model, nil, "system text", "test-model", testLogger())

	reads := 0
	turn.clock = func() time.Time {
		at := recordInstantLocal.Add(time.Duration(reads) * time.Hour)
		reads++
		return at
	}

	record, _, err := turn.Run(context.Background(), "what changed today", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if reads != 1 {
		t.Fatalf("the turn read its clock %d times, want 1: two reads let the record and the prompt state different instants", reads)
	}

	if got := record.Now.Format(time.RFC3339); got != recordInstantUTC {
		t.Fatalf("record.Now reads %s, want %s: a run that resolved \"today\" leaves no trace of which day unless the record states it, in UTC as the prompt does",
			got, recordInstantUTC)
	}
	if len(model.calls) != 2 {
		t.Fatalf("the turn made %d judgement calls, want 2", len(model.calls))
	}
	for i, call := range model.calls {
		if got := call.Now.Format(time.RFC3339); got != recordInstantUTC {
			t.Fatalf("judgement call %d states %s while the record states %s; the record is only diagnostic if it is the prompt's own instant",
				i+1, got, recordInstantUTC)
		}
	}
}

func TestTheRunRecordsInstantIsOnTheWireUnderTheKeyNow(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(Record{Now: recordInstantLocal.UTC()})
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}

	if want := `"now":"` + recordInstantUTC + `"`; !strings.Contains(string(body), want) {
		t.Fatalf("the record wire carries no %s; body=%s", want, body)
	}
}

func TestARecordCarryingNoInstantOmitsTheNowKeyEntirely(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(Record{Input: "no instant was supplied"})
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}

	if strings.Contains(string(body), `"now"`) {
		t.Fatalf("the record wire carries a now key for a run whose prompt stated no instant; body=%s", body)
	}
	if strings.Contains(string(body), "0001-01-01") {
		t.Fatalf("the record wire states the zero time as though it were a date; body=%s", body)
	}
}
