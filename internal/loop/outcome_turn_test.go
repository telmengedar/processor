package loop

import (
	"context"
	"log/slog"
	"maps"
	"strings"
	"testing"
)

func sameOutcome(a, b Outcome) bool {
	return a.Verdict == b.Verdict && a.Produced == b.Produced &&
		a.Grounded == b.Grounded && a.Curtailed == b.Curtailed && maps.Equal(a.Acted, b.Acted)
}

func notDeliveredLogLine(log string) string {
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, `msg="the run did not deliver`) {
			return line
		}
	}
	return ""
}

func TestTurnRunPutsTheOutcomeOnTheRecordItFiles(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = []Candidate{{ID: 100, Type: "documentation", Name: "Doc", Similarity: 0.9, Content: "body"}}
	model := &fakeModel{results: []JudgeResult{{Answer: "the answer", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.Outcome.Verdict != VerdictDelivered {
		t.Fatalf("record.Outcome = %+v, want a delivered verdict on a run that answered from an admitted row", record.Outcome)
	}
	if !graph.writeRunCalled {
		t.Fatal("WriteRun was not called, so nothing can be said about what was filed")
	}
	if filed := graph.writeRunRecord.Outcome; !sameOutcome(filed, record.Outcome) {
		t.Fatalf("the filed record carries outcome %+v against the returned %+v; an operator reads the filed one", filed, record.Outcome)
	}
}

func TestTurnRunRecordsAnOutcomeThatMatchesWhatItsOwnFieldsRecompute(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = []Candidate{{ID: 100, Type: "documentation", Name: "Doc", Similarity: 0.9, Content: "body"}}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "more"},
		{Answer: "the answer", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	stored := record.Outcome
	record.Outcome = Outcome{}
	recomputed := ComputeOutcome(record)

	if stored.Verdict != recomputed.Verdict || stored.Produced != recomputed.Produced ||
		stored.Grounded != recomputed.Grounded || stored.Curtailed != recomputed.Curtailed {
		t.Fatalf("the stored outcome %+v does not match the %+v its own record recomputes; the loop would be writing down a judgement nobody can check", stored, recomputed)
	}
}

func TestTheNotDeliveredWarnFiresOnTheVerdictTheRecordCarriesAndNamesTheFactsUnderIt(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	turn := &Turn{logger: slog.New(slog.NewTextHandler(&logBuf, nil))}

	record := Record{Subject: 42, Answer: "", CapReached: true}
	record.Outcome = ComputeOutcome(record)

	turn.logFinished(record, WriteReceipt{}, 0)

	warning := notDeliveredLogLine(logBuf.String())
	if warning == "" {
		t.Fatalf("a run stopped at the call cap having produced nothing raised no alarm; log:\n%s", logBuf.String())
	}
	for _, want := range []string{"verdict=curtailed", "produced=false", "grounded=false", "curtailed=true"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("the alarm does not carry %q; it was:\n%s", want, warning)
		}
	}
}

func TestTheNotDeliveredWarnIsSilentOnARunThatDelivered(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	turn := &Turn{logger: slog.New(slog.NewTextHandler(&logBuf, nil))}

	record := Record{Subject: 42, Answer: "the answer", Candidates: []Disposition{{Rank: 1, ID: 11, Included: true}}}
	record.Outcome = ComputeOutcome(record)

	turn.logFinished(record, WriteReceipt{}, 0)

	if warning := notDeliveredLogLine(logBuf.String()); warning != "" {
		t.Fatalf("a run that delivered raised the did-not-deliver alarm:\n%s", warning)
	}
}

func TestTheNotDeliveredWarnFiresOnAProducedAnswerNoAdmittedRowFed(t *testing.T) {
	t.Parallel()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	graph := baseGraph()
	graph.candidates = []Candidate{{ID: 100, Type: "documentation", Name: "Big", Similarity: 0.9, Content: strings.Repeat("x", AssemblyByteBudget+1)}}
	model := &fakeModel{results: []JudgeResult{{Answer: "an answer from nothing this system held", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, nil, "system", "test-model", logger)

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Outcome.Verdict != VerdictUngrounded {
		t.Fatalf("record.Outcome = %+v, want an ungrounded verdict on a run whose every row was cut", record.Outcome)
	}

	warning := notDeliveredLogLine(logBuf.String())
	if warning == "" {
		t.Fatalf("a run whose memory contributed nothing raised no alarm; log:\n%s", logBuf.String())
	}
	if !strings.Contains(warning, "verdict=ungrounded") {
		t.Fatalf("the alarm does not name the verdict that fired; it was:\n%s", warning)
	}
}
