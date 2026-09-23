package loop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

func researchToTheCap() []JudgeResult {
	results := make([]JudgeResult, 0, MaxModelCalls)
	for range MaxModelCalls {
		results = append(results, JudgeResult{Answer: "I will look that up before I answer.", Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"})
	}
	return results
}

func researchThenAnswer(answer string) []JudgeResult {
	results := researchToTheCap()
	results[MaxModelCalls-1] = answered(answer)
	return results
}

func capturingTurn(t *testing.T, model ModelPort) (*Turn, *bytes.Buffer) {
	t.Helper()
	var logged bytes.Buffer
	return NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", slog.New(slog.NewTextHandler(&logged, nil))), &logged
}

func withheldFlags(calls []JudgeInput) []bool {
	flags := make([]bool, len(calls))
	for i, call := range calls {
		flags[i] = call.WithholdTools
	}
	return flags
}

func TestTheCallTheLoopWouldNotDispatchAToolFromIsTheOnlyOneIssuedWithNoToolList(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap()}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := make([]bool, MaxModelCalls)
	want[MaxModelCalls-1] = true
	if got := withheldFlags(model.calls); !sameBools(got, want) {
		t.Fatalf("the turn withheld its tool list on calls %v, want %v: every call the loop would still dispatch a tool from must carry both tools, and only the last must carry none", got, want)
	}
}

func TestTheCallThatFollowsAClosedRecallIsIssuedWithNoToolList(t *testing.T) {
	t.Parallel()

	graph := graphReturningTheSameRowsToEveryRecall()
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !record.RecallClosed {
		t.Fatal("record.RecallClosed = false, want the yield account to have closed recall before the cap could fire")
	}
	if record.CapReached {
		t.Fatal("record.CapReached = true, want the record to name the bound that actually reserved the call")
	}

	want := make([]bool, record.ModelCalls)
	want[record.ModelCalls-1] = true
	if got := withheldFlags(model.calls); !sameBools(got, want) {
		t.Fatalf("the turn withheld its tool list on calls %v, want %v: a closed recall reserves the call that follows it, and reserving it means offering nothing rather than refusing afterwards", got, want)
	}
}

func TestAReservedCallProducesTheAnswerACappedRunUsedToReturnEmpty(t *testing.T) {
	t.Parallel()

	const answer = "Here is what the five rounds of retrieval support, and what remains open."

	model := &fakeModel{results: researchThenAnswer(answer)}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Answer != answer {
		t.Fatalf("record.Answer = %q, want %q: the call that used to be discarded is the one that carries the answer", record.Answer, answer)
	}
	if !record.Outcome.Produced {
		t.Fatalf("record.Outcome = %+v, want produced true over %d bytes of answer", record.Outcome, len(record.Answer))
	}
	if !record.CapReached || record.Outcome.Verdict != VerdictCurtailed {
		t.Fatalf("record.CapReached = %v and verdict = %q, want the run still reported as curtailed: producing an answer does not mean research ran to its own end", record.CapReached, record.Outcome.Verdict)
	}
	if record.ReservedCall.State != ReservedCallCompleted || record.ReservedCall.Error != "" {
		t.Fatalf("record.ReservedCall = %+v, want a completed call carrying no cause", record.ReservedCall)
	}
}

func TestReservingTheLastCallCostsNoDispatchedResearchRound(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	model := &fakeModel{results: researchToTheCap()}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := dispatchedRecalls(graph); got != MaxModelCalls-1 {
		t.Fatalf("the turn dispatched %d supplementary recalls, want %d: the reserved call is the one whose request was already thrown away, so reserving it must buy and lose nothing", got, MaxModelCalls-1)
	}
	if record.ModelCalls != MaxModelCalls {
		t.Fatalf("record.ModelCalls = %d, want %d: the reserved call replaces the call the turn already made, it does not add one", record.ModelCalls, MaxModelCalls)
	}
}

func TestARunThatAnswersBeforeItsLastCallIsOfferedBothToolsThroughout(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q1"},
		answered("the answer the run reached on its own"),
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := withheldFlags(model.calls); !sameBools(got, []bool{false, false}) {
		t.Fatalf("the turn withheld its tool list on calls %v, want neither: a run that has not reached its last call must keep both tools in front of the model", got)
	}
	if record.CapReached || record.Outcome.Verdict != VerdictDelivered {
		t.Fatalf("record.CapReached = %v and verdict = %q, want a delivered run untouched by reservation", record.CapReached, record.Outcome.Verdict)
	}
	for _, call := range model.calls {
		if call.MaxOutputTokens != JudgementBudget {
			t.Fatalf("a research call was issued %d output tokens, want the judgement site's own %d", call.MaxOutputTokens, JudgementBudget)
		}
	}
}

func TestTheReservedCallIsIssuedTheAnsweringSitesOwnBudget(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchThenAnswer("done")}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := model.calls[len(model.calls)-1].MaxOutputTokens; got != AnsweringBudget {
		t.Fatalf("the reserved call was issued %d output tokens, want the answering site's own %d: it is a different call site from a research call and declares its own budget", got, AnsweringBudget)
	}
	if record.Limits.AnsweringBudget != AnsweringBudget {
		t.Fatalf("record.Limits.AnsweringBudget = %d, want %d: a record that carries an answer must carry the budget that produced it", record.Limits.AnsweringBudget, AnsweringBudget)
	}
}

func TestAFailedReservedCallEndsTheTurnWithTheRecordItAlreadyHoldsRatherThanFailingTheRun(t *testing.T) {
	t.Parallel()

	const cause = "literal: dial tcp 10.0.0.55:443: connect: connection refused"

	graph := graphYieldingNewRowsToEveryRecall()
	model := &fakeModel{results: researchToTheCap(), failOn: MaxModelCalls, failErr: errors.New(cause)}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v — a reserved call that fails must leave the run exactly where it stands today, not turn an honest curtailed verdict into an outage", err)
	}
	if record.ReservedCall.State != ReservedCallFailed {
		t.Fatalf("record.ReservedCall.State = %q, want %q", record.ReservedCall.State, ReservedCallFailed)
	}
	if !strings.Contains(record.ReservedCall.Error, cause) {
		t.Fatalf("record.ReservedCall.Error = %q, want the failure's own cause %q recorded as the reason no answer was produced", record.ReservedCall.Error, cause)
	}
	if record.Outcome.Verdict != VerdictCurtailed || record.Outcome.Produced {
		t.Fatalf("record.Outcome = %+v, want a curtailed run that produced nothing: the worst case of reserving the call is the behaviour that was already there", record.Outcome)
	}
	if !graph.writeRunCalled {
		t.Fatal("the record was never filed; a reserved call that fails must not cost the run the record the loop already holds")
	}
	if len(record.Usage) != record.ModelCalls {
		t.Fatalf("record.Usage has %d entries against %d model calls; the record promises one entry per call made", len(record.Usage), record.ModelCalls)
	}
}

func TestAFailedReservedCallDoesNotPromoteTheProseTheModelWroteBesideAnEarlierToolRequest(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap(), failOn: MaxModelCalls, failErr: errors.New("connection reset")}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Answer != "" {
		t.Fatalf("record.Answer = %q, want empty: the prose a model writes beside a tool request is a statement of intent, and presenting intent as a result is worse than the nothing it replaces", record.Answer)
	}
}

func TestAResearchCallThatFailsStillFailsTheRun(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap(), failOn: 2, failErr: errors.New("connection reset")}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "hello", 42); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("Run() err = %v, want ErrModelUnavailable: only the reserved call is allowed to fail without failing the run", err)
	}
}

func TestAToolWantedOnAReservedCallIsRefusedRatherThanDispatched(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	model := &fakeModel{ignoresWithheldToolList: true, results: researchToTheCap()}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := dispatchedRecalls(graph); got != MaxModelCalls-1 {
		t.Fatalf("the turn dispatched %d supplementary recalls, want %d: a call offered no tool must dispatch none, whatever the adapter hands back", got, MaxModelCalls-1)
	}
	refused := record.ToolCalls[len(record.ToolCalls)-1]
	if refused.Error != errReservedCallRefused {
		t.Fatalf("the refused round carries error %q, want the loop's own sentence %q", refused.Error, errReservedCallRefused)
	}
	if len(refused.Results) != 0 {
		t.Fatalf("the refused round carries %d results, want none: it never reached the graph", len(refused.Results))
	}
}

func TestTheTurnLogsWhichConditionReservedItsLastCall(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name       string
		turn       func(t *testing.T) (*Turn, *bytes.Buffer)
		condition  string
		dispatched int
	}{
		{
			name: "the call budget was spent",
			turn: func(t *testing.T) (*Turn, *bytes.Buffer) {
				return capturingTurn(t, &fakeModel{results: researchToTheCap()})
			},
			condition:  string(reservedByCallCap),
			dispatched: MaxModelCalls - 1,
		},
		{
			name: "recall closed on consecutive barren rounds",
			turn: func(t *testing.T) (*Turn, *bytes.Buffer) {
				var logged bytes.Buffer
				model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
				return NewTurn(graphReturningTheSameRowsToEveryRecall(), model, nil, "system", "test-model", slog.New(slog.NewTextHandler(&logged, nil))), &logged
			},
			condition:  string(reservedByClosedRecall),
			dispatched: barrenRoundsToClose + 1,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			turn, logged := scenario.turn(t)
			if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
				t.Fatalf("Run: %v", err)
			}

			line := ""
			for _, candidate := range strings.Split(logged.String(), "\n") {
				if strings.Contains(candidate, "reserved its last judgement call") {
					line = candidate
				}
			}
			if line == "" {
				t.Fatalf("no line reports the reservation; an operator reading the log cannot tell a turn that answered from one that was made to:\n%s", logged.String())
			}
			if !strings.Contains(line, "condition="+scenario.condition) {
				t.Fatalf("the reservation line does not name %q as the condition that reserved the call:\n%s", scenario.condition, line)
			}
			if want := fmt.Sprintf("dispatched=%d", scenario.dispatched); !strings.Contains(line, want) {
				t.Fatalf("the reservation line does not carry %q, so it does not say how many rounds were actually dispatched before the reservation:\n%s", want, line)
			}
		})
	}
}

func TestACurtailedRunThatProducedAnAnswerIsStillCountedAmongTheCurtailed(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchThenAnswer("a grounded partial answer")}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	outcome := ComputeOutcome(record)
	if !outcome.Curtailed || !outcome.Produced || !outcome.Grounded {
		t.Fatalf("outcome = %+v, want curtailed, produced and grounded all true: reservation must not empty the population a verdict is read over", outcome)
	}
	if outcome.Verdict != VerdictCurtailed {
		t.Fatalf("verdict = %q, want %q recomputed from the record alone", outcome.Verdict, VerdictCurtailed)
	}
}

func TestTheSummaryReportsWhyAReservedCallLeftTheRunWithNothing(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Answer = ""
	record.CapReached = true
	record.ReservedCall = ReservedCall{State: ReservedCallFailed, Error: "literal: connection refused"}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "  reserved failed: literal: connection refused\n") {
		t.Fatalf("the summary does not say why the reserved call produced nothing; a reader of a run that got nothing cannot tell an unanswering model from an unreachable one:\n%s", summary)
	}
}

func TestAReservationTheRemainingTimeStopsBeforeItIsIssuedSaysTheCallWasNeverMade(t *testing.T) {
	t.Parallel()

	now := guardInstant
	model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
	model.beforeReturn = func() {
		if len(model.calls) >= barrenRoundsToClose+2 {
			now = guardInstant.Add(9*time.Minute + 59*time.Second)
		}
	}
	turn := turnWithClock(model, graphReturningTheSameRowsToEveryRecall(), func() time.Time { return now })

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(10*time.Minute))
	defer cancel()

	record, _, err := turn.Run(ctx, "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !record.RecallClosed || record.TimeShortfall == "" {
		t.Fatalf("the scenario did not reach the path under test: recallClosed=%v timeShortfall=%q", record.RecallClosed, record.TimeShortfall)
	}
	if got := withheldFlags(model.calls); slices.Contains(got, true) {
		t.Fatalf("a call was issued with its tool list withheld: %v — the reserved call is the one this scenario never gets to make", got)
	}
	if record.ReservedCall.State != ReservedCallUnmade {
		t.Fatalf("record.ReservedCall.State = %q, want %q: a turn that reserved a call and then stopped before issuing it reads, on every other field, exactly like one whose reserved call answered with nothing — which is the falsifier the design says sinks the mechanism", record.ReservedCall.State, ReservedCallUnmade)
	}
}

func TestAReservedCallThatAnsweredWithNothingIsDistinguishableFromOneNeverMade(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchThenAnswer("")}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Outcome.Produced {
		t.Fatalf("the scenario did not reach the path under test: the reserved call produced %q", record.Answer)
	}
	if record.ReservedCall.State != ReservedCallCompleted {
		t.Fatalf("record.ReservedCall.State = %q, want %q: a reserved call the endpoint answered is the one case where an empty answer falsifies the mechanism, and it must read as that rather than as a call that never ran", record.ReservedCall.State, ReservedCallCompleted)
	}
	if record.ReservedCall.Error != "" {
		t.Fatalf("record.ReservedCall.Error = %q, want empty: the call completed, it simply produced nothing", record.ReservedCall.Error)
	}
}

func TestTheSummaryReportsAReservedCallThatWasNeverMade(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Answer = ""
	record.RecallClosed = true
	record.TimeShortfall = "the run's remaining time cannot afford another judgement call"
	record.ReservedCall = ReservedCall{State: ReservedCallUnmade}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "  reserved unmade\n") {
		t.Fatalf("the summary does not say the reserved call was never made, so it reads as a run whose answering call produced nothing:\n%s", summary)
	}
}
