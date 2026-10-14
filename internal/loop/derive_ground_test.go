package loop

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

var groundingNow = time.Date(2026, 10, 14, 15, 9, 21, 0, time.UTC)

const groundingQuery = "which merge rule does the sweep share with the turn?"

func derivedUnderGrounding(t *testing.T, input, dates string, now time.Time) ([]string, UpdateWindow, string) {
	t.Helper()

	model := &deriveFake{text: dates + "\n" + groundingQuery}
	queries, window, discarded, err := deriveQueries(context.Background(), model, input, now, time.Hour)
	if err != nil {
		t.Fatalf("deriveQueries(%q, %q): %v", input, dates, err)
	}
	return queries, window, discarded
}

func dayWindow(zone *time.Location, fromYear int, fromMonth time.Month, fromDay, toYear int, toMonth time.Month, toDay int) UpdateWindow {
	return UpdateWindow{
		From: time.Date(fromYear, fromMonth, fromDay, 0, 0, 0, 0, zone),
		To:   time.Date(toYear, toMonth, toDay+1, 0, 0, 0, 0, zone),
	}
}

func TestARangeInAYearTheRequestNeitherWritesNorRunsInDoesNotBoundTheRun(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "how was the long talk yesterday?", "DATES: 2024-05-21..2024-05-21", groundingNow)

	if !window.IsZero() {
		t.Fatalf("window = %+v, want zero: a year the request never wrote and the run is not in must not bound retrieval", window)
	}
	if !strings.Contains(discarded, "2024-05-21..2024-05-21") || !strings.Contains(discarded, "start year 2024") {
		t.Fatalf("discarded = %q, want the range and the start-year rule it failed", discarded)
	}
}

func TestAnExplicitFarDateWrittenInTheRequestStillBoundsTheRun(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what happened in May 2024?", "DATES: 2024-05-01..2024-05-31", groundingNow)

	if want := dayWindow(time.UTC, 2024, 5, 1, 2024, 5, 31); window != want {
		t.Fatalf("window = %+v, want %+v: a far date the request itself writes is grounded", window, want)
	}
	if discarded != "" {
		t.Fatalf("discarded = %q, want none", discarded)
	}
}

func TestARelativeRangeInTheRunYearBoundsTheRun(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "how was the long talk yesterday?", "DATES: 2026-10-13..2026-10-13", groundingNow)

	if want := dayWindow(time.UTC, 2026, 10, 13, 2026, 10, 13); window != want {
		t.Fatalf("window = %+v, want %+v", window, want)
	}
	if discarded != "" {
		t.Fatalf("discarded = %q, want none", discarded)
	}
}

func TestEachEndpointYearIsGroundedOnItsOwn(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what changed since 2020?", "DATES: 2020-01-01..2026-10-14", groundingNow)

	if want := dayWindow(time.UTC, 2020, 1, 1, 2026, 10, 14); window != want {
		t.Fatalf("window = %+v, want %+v: the start is written and the end is the run year, and the years between need no grounding", window, want)
	}
	if discarded != "" {
		t.Fatalf("discarded = %q, want none", discarded)
	}
}

func TestAnUnwrittenStartYearDiscardsEvenWhenTheEndIsTheRunYear(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what changed recently?", "DATES: 2024-05-21..2026-10-14", groundingNow)

	if !window.IsZero() {
		t.Fatalf("window = %+v, want zero", window)
	}
	if !strings.Contains(discarded, "start year 2024") {
		t.Fatalf("discarded = %q, want the start-year rule", discarded)
	}
}

func TestAnUnwrittenEndYearDiscardsEvenWhenTheStartIsWritten(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what changed since 2024?", "DATES: 2024-01-01..2025-12-31", groundingNow)

	if !window.IsZero() {
		t.Fatalf("window = %+v, want zero", window)
	}
	if !strings.Contains(discarded, "end year 2025") {
		t.Fatalf("discarded = %q, want the end-year rule", discarded)
	}
}

func TestAYearInsideALongerDigitRunDoesNotGround(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"what is order 120245?", "what is order 20245?", "what is order 2024123?"} {
		_, window, discarded := derivedUnderGrounding(t, input, "DATES: 2024-05-21..2024-05-21", groundingNow)

		if !window.IsZero() || discarded == "" {
			t.Fatalf("input %q: window = %+v, discarded = %q, want the range discarded: only a run of exactly four digits is a written year", input, window, discarded)
		}
	}
}

func TestARangeBeginningAfterTheRunInstantDoesNotBoundTheRun(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what will change tomorrow?", "DATES: 2026-10-15..2026-10-15", groundingNow)

	if !window.IsZero() {
		t.Fatalf("window = %+v, want zero: a range wholly in the future can only empty retrieval", window)
	}
	if !strings.Contains(discarded, "begins after the run's instant") {
		t.Fatalf("discarded = %q, want the future rule", discarded)
	}
}

func TestARangeThatIncludesTodayBoundsTheRun(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what changed today?", "DATES: 2026-10-14..2026-10-14", groundingNow)

	if want := dayWindow(time.UTC, 2026, 10, 14, 2026, 10, 14); window != want {
		t.Fatalf("window = %+v, want %+v: today begins before the run's instant even though its end is after", window, want)
	}
	if discarded != "" {
		t.Fatalf("discarded = %q, want none", discarded)
	}
}

func TestTheRunYearIsTheYearInTheZoneTheRunStates(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("test+02", 2*60*60)
	now := time.Date(2027, 1, 1, 0, 30, 0, 0, zone)
	if now.UTC().Year() != 2026 {
		t.Fatalf("fixture error: the instant must fall in 2026 in UTC, got %d", now.UTC().Year())
	}

	_, window, discarded := derivedUnderGrounding(t, "what changed today?", "DATES: 2027-01-01..2027-01-01", now)

	if want := dayWindow(zone, 2027, 1, 1, 2027, 1, 1); window != want {
		t.Fatalf("window = %+v, want %+v: the year of the instant in its own zone is the run year", window, want)
	}
	if discarded != "" {
		t.Fatalf("discarded = %q, want none", discarded)
	}
}

func TestARunThatStatesNoInstantKeepsTheRangeItWasGiven(t *testing.T) {
	t.Parallel()

	_, window, discarded := derivedUnderGrounding(t, "what changed on 2026-09-12?", "DATES: 2026-09-12..2026-09-12", time.Time{})

	if window.IsZero() || discarded != "" {
		t.Fatalf("window = %+v, discarded = %q, want the range kept: with no instant there is nothing to ground against", window, discarded)
	}
}

func TestADiscardedRangeKeepsTheDerivedQueries(t *testing.T) {
	t.Parallel()

	model := &deriveFake{text: "DATES: 2024-05-21..2024-05-21\nq1?\nq2?\nq3?\nq4?\nq5?"}
	queries, window, discarded, err := deriveQueries(context.Background(), model, "how was the long talk yesterday?", groundingNow, time.Hour)
	if err != nil {
		t.Fatalf("deriveQueries: %v", err)
	}

	if !window.IsZero() || discarded == "" {
		t.Fatalf("window = %+v, discarded = %q, want the range discarded", window, discarded)
	}
	if want := []string{"q1?", "q2?", "q3?", "q4?", "q5?"}; !slices.Equal(queries, want) {
		t.Fatalf("queries = %q, want %q: the range was wrong, not the queries", queries, want)
	}
}

func TestAMalformedDatesValueIsNotReportedAsDiscarded(t *testing.T) {
	t.Parallel()

	for _, dates := range []string{
		"DATES: none",
		"DATES: sometime soon",
		"DATES: 2026-09-15..2026-09-10",
		"DATES: 2026-13-40..2026-13-40",
		"DATES: 2026-09-12",
	} {
		_, window, discarded := derivedUnderGrounding(t, "what changed", dates, groundingNow)

		if !window.IsZero() || discarded != "" {
			t.Fatalf("%q: window = %+v, discarded = %q, want no window and no discard note: unparsed is not discarded", dates, window, discarded)
		}
	}

	_, _, discarded := derivedUnderGrounding(t, "what changed", "first question?", groundingNow)
	if discarded != "" {
		t.Fatalf("a text with no DATES line carries discard note %q", discarded)
	}
}

type groundedRun struct {
	graph  *fakeGraph
	model  *fakeModel
	record Record
}

func runGroundedTurn(t *testing.T, input, dates string) groundedRun {
	t.Helper()

	model := &fakeModel{
		derivedText: dates + "\n" + groundingQuery,
		results: []JudgeResult{
			{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "a later angle"},
			{Answer: "done", Reason: Answered, RawReason: "stop"},
		},
	}
	graph := derivingGraph()
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	turn.clock = func() time.Time { return groundingNow }

	record, _, err := turn.Run(context.Background(), input, 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return groundedRun{graph: graph, model: model, record: record}
}

func TestADiscardedRangeReachesNeitherRetrievalNorAnyRecall(t *testing.T) {
	t.Parallel()

	run := runGroundedTurn(t, "how was the long talk yesterday?", "DATES: 2024-05-21..2024-05-21")

	if len(run.graph.recallCalls) != 2 {
		t.Fatalf("the graph saw %d recalls, want the primary retrieval and the judge's recall", len(run.graph.recallCalls))
	}
	for i, call := range run.graph.recallCalls {
		if !call.Window.IsZero() {
			t.Fatalf("recall %d carried window %+v, want zero", i, call.Window)
		}
	}
	if !run.record.Window.IsZero() {
		t.Fatalf("record.Window = %+v, want zero", run.record.Window)
	}
}

func TestAGroundedRangeReachesRetrievalAndEveryRecall(t *testing.T) {
	t.Parallel()

	run := runGroundedTurn(t, "how was the long talk yesterday?", "DATES: 2026-10-13..2026-10-13")

	want := dayWindow(time.UTC, 2026, 10, 13, 2026, 10, 13)
	if len(run.graph.recallCalls) != 2 {
		t.Fatalf("the graph saw %d recalls, want 2", len(run.graph.recallCalls))
	}
	for i, call := range run.graph.recallCalls {
		if call.Window != want {
			t.Fatalf("recall %d carried window %+v, want %+v", i, call.Window, want)
		}
	}
}

func TestTheRecordAndTheSummaryShowADiscardedRangeAndNothingOnAnAcceptedOne(t *testing.T) {
	t.Parallel()

	discarded := runGroundedTurn(t, "how was the long talk yesterday?", "DATES: 2024-05-21..2024-05-21").record
	if !strings.Contains(discarded.WindowDiscarded, "2024-05-21..2024-05-21") {
		t.Fatalf("record.WindowDiscarded = %q, want the discarded range", discarded.WindowDiscarded)
	}
	body, err := json.Marshal(discarded)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"windowDiscarded":`) {
		t.Fatalf("the record JSON does not carry windowDiscarded: %s", body)
	}
	if summary := RenderSummary(discarded, groundingNow); !strings.Contains(summary, "  window: "+discarded.WindowDiscarded) {
		t.Fatalf("the summary does not show the discarded range.\nsummary:\n%s", summary)
	}

	accepted := runGroundedTurn(t, "how was the long talk yesterday?", "DATES: 2026-10-13..2026-10-13").record
	if accepted.WindowDiscarded != "" {
		t.Fatalf("an accepted range carries discard note %q", accepted.WindowDiscarded)
	}
	body, err = json.Marshal(accepted)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(body), "windowDiscarded") {
		t.Fatalf("an accepted run's record JSON carries the windowDiscarded key: %s", body)
	}
	if summary := RenderSummary(accepted, groundingNow); strings.Contains(summary, "  window: ") {
		t.Fatalf("an accepted run's summary carries a discard line.\nsummary:\n%s", summary)
	}
}

func TestTheJudgeIsToldOfNoFilterWhenTheRangeWasDiscarded(t *testing.T) {
	t.Parallel()

	run := runGroundedTurn(t, "how was the long talk yesterday?", "DATES: 2024-05-21..2024-05-21")

	if len(run.model.calls) == 0 {
		t.Fatalf("the judge was never called")
	}
	for i, call := range run.model.calls {
		content := RenderUserContent(call.Block, call.Input, call.Now, call.Window, call.Conversation, call.Trailer)
		if strings.Contains(content, "retrieval is limited") || strings.Contains(content, "2024") {
			t.Fatalf("judge call %d content mentions the discarded range:\n%s", i, content)
		}
	}
}
