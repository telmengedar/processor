package loop

import (
	"context"
	"strings"
	"testing"
)

func writeAction(path, content string) Action {
	return Action{Tool: ToolWriteFile, WritePath: path, WriteContent: content}
}

func wantsActions(actions ...Action) JudgeResult {
	return JudgeResult{Reason: WantsWrite, RawReason: "stop", Actions: actions}
}

func TestTurnRunCarriesOutEveryActionOfOneResponseRatherThanTheFirstAlone(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsActions(writeAction("index.html", "<h1>hi</h1>"), writeAction("README.md", "# hi"), writeAction("style.css", "body{}")),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(files.writes) != 3 {
		t.Fatalf("files.writes = %+v, want all three files written", files.writes)
	}
	if files.writes[0].Path != "index.html" || files.writes[1].Path != "README.md" || files.writes[2].Path != "style.css" {
		t.Fatalf("files.writes = %+v, want the order the response declared", files.writes)
	}
	if len(record.ToolCalls) != 3 {
		t.Fatalf("record.ToolCalls has %d entries, want 3", len(record.ToolCalls))
	}
	if record.ModelCalls != 2 {
		t.Fatalf("record.ModelCalls = %d, want 2 — three actions of one response cost one round, not three", record.ModelCalls)
	}
	if files.openCalls != 1 {
		t.Fatalf("files.openCalls = %d, want 1 working directory for the whole run", files.openCalls)
	}
}

func TestTurnRunStampsEveryActionOfOneResponseWithTheSameRoundNumber(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsActions(writeAction("a.html", "a"), writeAction("b.html", "b")),
		wantsActions(writeAction("c.html", "c")),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make pages", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 3 {
		t.Fatalf("record.ToolCalls has %d entries, want 3", len(record.ToolCalls))
	}
	want := []int{1, 1, 2}
	for i, w := range want {
		if record.ToolCalls[i].Round != w {
			t.Fatalf("record.ToolCalls[%d].Round = %d, want %d — the record must say which response declared each action", i, record.ToolCalls[i].Round, w)
		}
	}
}

func TestTurnRunCarriesOutTheActionsAfterOneThatFailedRatherThanAbandoningTheRound(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsActions(
			Action{Tool: ToolWriteFile, Error: "action arguments could not be parsed"},
			writeAction("second.html", "second"),
		),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(files.writes) != 1 || files.writes[0].Path != "second.html" {
		t.Fatalf("files.writes = %+v, want the second action still carried out", files.writes)
	}
	if len(record.ToolCalls) != 2 {
		t.Fatalf("record.ToolCalls has %d entries, want both the failed action and the one after it", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Error == "" {
		t.Fatal("record.ToolCalls[0].Error is empty, want the failed action's reason on the record")
	}
}

func TestTurnRunPutsContentItCouldNotReadOnTheRecordAsItsOwnRound(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{
		{Reason: Malformed, RawReason: "stop", Problems: []string{"an action block was opened and never closed (42 bytes): <processor-action>{\"name\""}},
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want the unread content recorded", len(record.ToolCalls))
	}
	got := record.ToolCalls[0]
	if got.Tool != ToolUnparsed {
		t.Fatalf("record.ToolCalls[0].Tool = %q, want %q", got.Tool, ToolUnparsed)
	}
	if !strings.Contains(got.Error, "never closed") {
		t.Fatalf("record.ToolCalls[0].Error = %q, want the reason it could not be read", got.Error)
	}
	if record.ModelCalls != 2 {
		t.Fatalf("record.ModelCalls = %d, want 2 — the model is told what could not be read and answers again", record.ModelCalls)
	}
}

func TestTurnRunShowsTheUnreadContentToTheModelOnTheFollowingCall(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{
		{Reason: Malformed, RawReason: "stop", Problems: []string{"an action block was opened and never closed"}},
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "make a page", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 2 {
		t.Fatalf("the model was called %d times, want 2", len(model.calls))
	}
	second := model.calls[1]
	if len(second.PriorTools) != 1 || second.PriorTools[0].Tool != ToolUnparsed {
		t.Fatalf("the second call's PriorTools = %+v, want the unread round replayed into it", second.PriorTools)
	}
	if !strings.Contains(second.PriorTools[0].Error, "never closed") {
		t.Fatalf("the replayed round's Error = %q, want the reason carried to the model", second.PriorTools[0].Error)
	}
}

func TestTurnRunStopsAtTheCallCapWhenEveryResponseOnlyCarriesContentItCannotRead(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{
		{Reason: Malformed, RawReason: "stop", Problems: []string{"unreadable one"}},
		{Reason: Malformed, RawReason: "stop", Problems: []string{"unreadable two"}},
		{Reason: Malformed, RawReason: "stop", Problems: []string{"unreadable three"}},
		{Reason: Malformed, RawReason: "stop", Problems: []string{"unreadable four"}},
	}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.ModelCalls != MaxModelCalls {
		t.Fatalf("record.ModelCalls = %d, want the cap at %d", record.ModelCalls, MaxModelCalls)
	}
	if !record.CapReached {
		t.Fatal("record.CapReached is false, want the cap recorded as the reason the run stopped")
	}
	if len(record.ToolCalls) != MaxModelCalls {
		t.Fatalf("record.ToolCalls has %d entries, want every unread response on the record", len(record.ToolCalls))
	}
}

func TestTurnRunRecordsEveryActionOfTheCappedResponseNotOnlyTheFirst(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsActions(writeAction("a.html", "a")),
		wantsActions(writeAction("b.html", "b")),
		wantsActions(writeAction("c.html", "c"), writeAction("d.html", "d")),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make pages", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !record.CapReached {
		t.Fatal("record.CapReached is false, want the cap recorded")
	}
	if len(record.ToolCalls) != 4 {
		t.Fatalf("record.ToolCalls has %d entries, want both capped actions of the final response recorded", len(record.ToolCalls))
	}
	capped := record.ToolCalls[2:]
	if capped[0].Path != "c.html" || capped[1].Path != "d.html" {
		t.Fatalf("the capped entries are %+v, want both undispatched paths", capped)
	}
	for i, tc := range capped {
		if tc.Error != errCallCapReached {
			t.Fatalf("the capped entry %d has Error = %q, want the cap named", i, tc.Error)
		}
	}
	if len(files.writes) != 2 {
		t.Fatalf("files.writes = %+v, want the capped response's writes never carried out", files.writes)
	}
}

func TestTurnRunRecordsBothTheActionAndTheUnreadContentOfOneResponse(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		{
			Reason:    WantsWrite,
			RawReason: "stop",
			Actions:   []Action{writeAction("index.html", "<h1>hi</h1>")},
			Problems:  []string{"an action block carried no action name"},
		},
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 2 {
		t.Fatalf("record.ToolCalls has %d entries, want the action and the unread block both recorded", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Tool != ToolUnparsed {
		t.Fatalf("record.ToolCalls[0].Tool = %q, want the unread block first in its round", record.ToolCalls[0].Tool)
	}
	if record.ToolCalls[1].Tool != ToolWriteFile || record.ToolCalls[1].Error != "" {
		t.Fatalf("record.ToolCalls[1] = %+v, want the readable action still carried out", record.ToolCalls[1])
	}
	if len(files.writes) != 1 {
		t.Fatalf("files.writes = %+v, want the readable action carried out despite the unread one", files.writes)
	}
}
