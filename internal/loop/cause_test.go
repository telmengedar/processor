package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func runFailedLog(t *testing.T, turn *Turn, buf *strings.Builder) string {
	t.Helper()
	if _, _, err := turn.Run(context.Background(), "hello", 42); err == nil {
		t.Fatal("Run returned no error although the run was made to fail")
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, `msg="run failed"`) {
			return line
		}
	}
	return ""
}

func TestTurnRunLogsTheModelCallCauseWholeOnARunFailedRecord(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	const cause = "model double: upstream said the peg-native format did not match"
	turn := NewTurn(baseGraph(), &fakeModel{err: errors.New(cause)}, nil, "system", "test-model",
		slog.New(slog.NewTextHandler(&buf, nil)))

	line := runFailedLog(t, turn, &buf)
	if line == "" {
		t.Fatalf("operator log carries no run-failed record for a failed model call; log:\n%s", buf.String())
	}
	if !strings.Contains(line, cause) {
		t.Fatalf("the run-failed record names no cause for the failed model call, so the sentinel class is all it says: %s", line)
	}
}

func TestTurnRunFailedRecordNamesTheSubjectAndTheWallClock(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	turn := NewTurn(baseGraph(), &fakeModel{err: errors.New("model double: refused")}, nil, "system", "test-model",
		slog.New(slog.NewTextHandler(&buf, nil)))

	line := runFailedLog(t, turn, &buf)
	if line == "" {
		t.Fatalf("operator log carries no run-failed record at all; log:\n%s", buf.String())
	}
	if !strings.Contains(line, "subject=42") {
		t.Fatalf("the run-failed record names no subject: %s", line)
	}
	if !strings.Contains(line, "elapsed=") {
		t.Fatalf("the run-failed record names no elapsed time: %s", line)
	}
}

func TestTurnRunLogsTheAnchorReadCauseWholeOnARunFailedRecord(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	const cause = "graph double: dial tcp 10.0.0.55:443: connect: connection refused"
	graph := &fakeGraph{nodeErr: errors.New(cause)}
	turn := NewTurn(graph, &fakeModel{}, nil, "system", "test-model", slog.New(slog.NewTextHandler(&buf, nil)))

	line := runFailedLog(t, turn, &buf)
	if line == "" {
		t.Fatalf("operator log carries no run-failed record for a failed anchor read; log:\n%s", buf.String())
	}
	if !strings.Contains(line, cause) {
		t.Fatalf("the run-failed record names no cause for the failed anchor read: %s", line)
	}
}

func TestTurnRunLogsTheRetrievalCauseWholeOnARunFailedRecord(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	const cause = "graph double: recall returned 503 while draining"
	graph := baseGraph()
	graph.recallErr = errors.New(cause)
	turn := NewTurn(graph, &fakeModel{}, nil, "system", "test-model", slog.New(slog.NewTextHandler(&buf, nil)))

	line := runFailedLog(t, turn, &buf)
	if !strings.Contains(line, cause) {
		t.Fatalf("the run-failed record names no cause for the failed retrieval: %q", line)
	}
}

func TestTurnRunLogsNoRunFailedRecordWhenTheRunReachedAnAnswer(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	turn := NewTurn(baseGraph(), &fakeModel{}, nil, "system", "test-model", slog.New(slog.NewTextHandler(&buf, nil)))

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(buf.String(), `msg="run failed"`) {
		t.Fatalf("operator log carries a run-failed record for a run that answered; log:\n%s", buf.String())
	}
}

func firstRoundErrorOfAFailedWrite(t *testing.T, writeErr error) string {
	t.Helper()

	files := &fakeFiles{dir: "/runs/run-1", writeErr: writeErr}
	model := &fakeModel{results: []JudgeResult{wantsWriteOf("index.html", "x"), answered("done")}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	return record.ToolCalls[0].Error
}

func TestTurnRunARejectedWriteAndAnUnrecognisedWriteCarryDifferentCauses(t *testing.T) {
	t.Parallel()

	const reason = "path must not leave the working directory"
	const osCause = "no space left on device"

	rejected := firstRoundErrorOfAFailedWrite(t, fmt.Errorf("%w: %s", ErrWriteRejected, reason))
	unrecognised := firstRoundErrorOfAFailedWrite(t, errors.New(osCause))

	if want := ErrWriteRejected.Error() + ": " + reason; rejected != want {
		t.Fatalf("a rejected write recorded %q, want the workspace's own reason %q", rejected, want)
	}
	if unrecognised != osCause {
		t.Fatalf("an unrecognised write recorded %q, want the operating system's own cause %q", unrecognised, osCause)
	}
	if rejected == unrecognised {
		t.Fatalf("both write failures recorded %q, so the rejected/unrecognised distinction has been erased rather than fixed", rejected)
	}
}

func TestTurnRunOpeningTheWorkingDirectoryCarriesItsOwnCause(t *testing.T) {
	t.Parallel()

	const cause = "mkdir /data/runs/run-4149672001: permission denied"
	files := &fakeFiles{openErr: errors.New(cause)}
	model := &fakeModel{results: []JudgeResult{wantsWriteOf("index.html", "x"), answered("done")}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Error != cause {
		t.Fatalf("record.ToolCalls[0].Error = %q, want the directory failure's own cause %q", record.ToolCalls[0].Error, cause)
	}
}

func TestTurnRunStillReportsTheCallCapAsTheLoopsOwnSentence(t *testing.T) {
	t.Parallel()

	wantsRecall := JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"}
	model := &fakeModel{results: []JudgeResult{wantsRecall, wantsRecall, wantsRecall, wantsRecall, wantsRecall, wantsRecall}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !record.CapReached {
		t.Fatal("record.CapReached = false, want the cap reached")
	}
	if len(record.ToolCalls) == 0 {
		t.Fatal("record.ToolCalls is empty, want the capped round recorded")
	}
	last := record.ToolCalls[len(record.ToolCalls)-1]
	if last.Error != errCallCapReached {
		t.Fatalf("the capped round recorded %q, want the loop's own sentence %q — the loop authored this failure", last.Error, errCallCapReached)
	}
}

func TestTurnRunStillReportsAnAbsentWorkingDirectoryAsTheLoopsOwnSentence(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{wantsWriteOf("index.html", "x"), answered("done")}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Error != errNoWorkingDirectory {
		t.Fatalf("the refused write recorded %q, want the loop's own sentence %q — the loop authored this failure", record.ToolCalls[0].Error, errNoWorkingDirectory)
	}
}

func TestBoundCauseCarriesACauseOfExactlyTheBoundWhole(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("a", CarriedCauseRunes)
	if got := BoundCause(cause); got != cause {
		t.Fatalf("BoundCause returned %d runes for a cause of exactly the %d-rune bound, want it carried whole",
			len([]rune(got)), CarriedCauseRunes)
	}
}

func TestBoundCauseTruncatesACauseOneRunePastTheBoundToTheBound(t *testing.T) {
	t.Parallel()

	got := BoundCause(strings.Repeat("a", CarriedCauseRunes+1))
	if n := len([]rune(got)); n != CarriedCauseRunes {
		t.Fatalf("BoundCause returned %d runes for a cause one rune past the bound, want %d", n, CarriedCauseRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("BoundCause returned a %d-rune cause that does not end in an ellipsis, so a truncated cause does not say it was truncated", len([]rune(got)))
	}
}

func TestBoundCauseCountsRunesRatherThanBytes(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("é", CarriedCauseRunes)
	if got := BoundCause(cause); got != cause {
		t.Fatalf("BoundCause returned %d runes for a cause of %d multibyte runes (%d bytes), want the bound measured in runes",
			len([]rune(got)), CarriedCauseRunes, len(cause))
	}
}

func TestTurnRunBoundsACauseLongerThanTheBoundBeforeItReachesTheRound(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("b", CarriedCauseRunes+400)
	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{{ID: 1, Content: "initial"}}},
		{Err: errors.New(cause)},
	}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"},
		answered("done"),
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if n := len([]rune(record.ToolCalls[0].Error)); n != CarriedCauseRunes {
		t.Fatalf("the round carried %d runes of a %d-rune cause, want it bounded to %d", n, len([]rune(cause)), CarriedCauseRunes)
	}
}

func TestTurnRunBoundsAToolErrorPassedThroughFromTheModelsOwnRound(t *testing.T) {
	t.Parallel()

	toolError := strings.Repeat("c", CarriedCauseRunes+1)
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", ToolError: toolError},
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if n := len([]rune(record.ToolCalls[0].Error)); n != CarriedCauseRunes {
		t.Fatalf("the passed-through tool error carried %d runes, want it bounded to %d like every other cause", n, CarriedCauseRunes)
	}
}

func TestTurnRunBoundsAWriteToolErrorPassedThroughFromTheModelsOwnRound(t *testing.T) {
	t.Parallel()

	toolError := strings.Repeat("d", CarriedCauseRunes+1)
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsWrite, RawReason: "tool_calls", ToolError: toolError},
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, &fakeFiles{dir: "/runs/run-1"}, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	if n := len([]rune(record.ToolCalls[0].Error)); n != CarriedCauseRunes {
		t.Fatalf("the passed-through write tool error carried %d runes, want it bounded to %d like every other cause", n, CarriedCauseRunes)
	}
}

func TestTurnRunBoundsAToolErrorTheCallCapRefused(t *testing.T) {
	t.Parallel()

	toolError := strings.Repeat("e", CarriedCauseRunes+1)
	wantsRecall := JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "q"}
	capped := JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", ToolError: toolError}
	model := &fakeModel{results: []JudgeResult{wantsRecall, wantsRecall, wantsRecall, wantsRecall, wantsRecall, capped}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.ToolCalls) != MaxModelCalls {
		t.Fatalf("record.ToolCalls has %d entries, want %d", len(record.ToolCalls), MaxModelCalls)
	}
	last := record.ToolCalls[len(record.ToolCalls)-1]
	if n := len([]rune(last.Error)); n != CarriedCauseRunes {
		t.Fatalf("the capped round's passed-through tool error carried %d runes, want it bounded to %d like every other cause", n, CarriedCauseRunes)
	}
}

func TestTheCarriedCauseBoundIsFiveHundredAndTwelveRunes(t *testing.T) {
	t.Parallel()

	if got := len([]rune(BoundCause(strings.Repeat("a", 513)))); got != 512 {
		t.Fatalf("a 513-rune cause was bounded to %d runes, want 512 — the bound the design derives for the response, the record and the model", got)
	}
	if got := BoundCause(strings.Repeat("a", 512)); len([]rune(got)) != 512 || strings.HasSuffix(got, "…") {
		t.Fatalf("a 512-rune cause was not carried whole, so the bound is below 512")
	}
}

func writeRoundLog(t *testing.T, writeErr, openErr error) string {
	t.Helper()

	var buf strings.Builder
	files := &fakeFiles{dir: "/runs/run-1", writeErr: writeErr, openErr: openErr}
	model := &fakeModel{results: []JudgeResult{wantsWriteOf("index.html", "x"), answered("done")}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", slog.New(slog.NewTextHandler(&buf, nil)))

	if _, _, err := turn.Run(context.Background(), "make a page", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return buf.String()
}

func TestTurnRunLeavesARejectedWriteOutOfTheOperatorsErrorLogWhileLoggingAnUnrecognisedOne(t *testing.T) {
	t.Parallel()

	const reason = "path must not leave the working directory"
	rejected := writeRoundLog(t, fmt.Errorf("%w: %s", ErrWriteRejected, reason), nil)
	unrecognised := writeRoundLog(t, errors.New("no space left on device"), nil)

	if !strings.Contains(unrecognised, `msg="file write failed"`) {
		t.Fatalf("an unrecognised write failure logged no operator error, so this guard cannot discriminate; log:\n%s", unrecognised)
	}
	if strings.Contains(rejected, `msg="file write failed"`) {
		t.Fatalf("a write the workspace refused was logged as an operator-level failure; a refusal is the model's to correct and never fails the run; log:\n%s", rejected)
	}
}

func TestTurnRunLogsTheCauseWhenOpeningTheWorkingDirectoryFails(t *testing.T) {
	t.Parallel()

	const cause = "mkdir /data/runs/run-4149672001: permission denied"
	logged := writeRoundLog(t, nil, errors.New(cause))

	if !strings.Contains(logged, `msg="opening the run working directory failed"`) {
		t.Fatalf("a failed working-directory open logged no operator error; log:\n%s", logged)
	}
	if !strings.Contains(logged, cause) {
		t.Fatalf("the operator log names no cause for the failed working-directory open; log:\n%s", logged)
	}
}

func TestTurnRunLogsARunFailedRecordWhenTheSubjectResolvesToNothing(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	turn := NewTurn(&fakeGraph{nodeFound: false}, &fakeModel{}, nil, "system", "test-model",
		slog.New(slog.NewTextHandler(&buf, nil)))

	line := runFailedLog(t, turn, &buf)
	if line == "" {
		t.Fatalf("a run whose subject resolved to nothing ended without a run-failed record; log:\n%s", buf.String())
	}
	if !strings.Contains(line, ErrSubjectNotFound.Error()) {
		t.Fatalf("the run-failed record does not name why the run ended: %s", line)
	}
}
