package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

type fileWrite struct {
	Dir     string
	Path    string
	Content string
}

type fakeFiles struct {
	dir       string
	openCalls int
	openErr   error

	writes   []fileWrite
	writeErr error
}

func (f *fakeFiles) OpenRun(context.Context) (string, error) {
	f.openCalls++
	if f.openErr != nil {
		return "", f.openErr
	}
	return f.dir, nil
}

func (f *fakeFiles) Write(_ context.Context, dir, path, content string) (int, error) {
	f.writes = append(f.writes, fileWrite{Dir: dir, Path: path, Content: content})
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(content), nil
}

func wantsWriteOf(path, content string) JudgeResult {
	return JudgeResult{Reason: WantsWrite, RawReason: "tool_calls", WritePath: path, WriteContent: content}
}

func answered(text string) JudgeResult {
	return JudgeResult{Answer: text, Reason: Answered, RawReason: "stop"}
}

func TestTurnRunHandsAWriteToolCallToTheWorkingDirectoryVerbatim(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("index.html", "<h1>hi</h1>"),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := fileWrite{Dir: "/runs/run-1", Path: "index.html", Content: "<h1>hi</h1>"}
	if len(files.writes) != 1 || files.writes[0] != want {
		t.Fatalf("files.writes = %+v, want exactly one write %+v", files.writes, want)
	}
	if record.Workspace != "/runs/run-1" {
		t.Fatalf("record.Workspace = %q, want the directory the run opened", record.Workspace)
	}
	if len(record.ToolCalls) != 1 {
		t.Fatalf("record.ToolCalls has %d entries, want 1", len(record.ToolCalls))
	}
	got := record.ToolCalls[0]
	if got.Tool != ToolWriteFile || got.Path != "index.html" || got.Bytes != len("<h1>hi</h1>") || got.Error != "" {
		t.Fatalf("record.ToolCalls[0] = %+v, want an accepted writeFile round for index.html of %d bytes", got, len("<h1>hi</h1>"))
	}
}

func TestTurnRunOpensOneWorkingDirectoryForTwoWriteRoundsOfTheSameRun(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("index.html", "first"),
		wantsWriteOf("style.css", "second"),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if files.openCalls != 1 {
		t.Fatalf("OpenRun was called %d times, want 1 — the run has one working directory, not one per write", files.openCalls)
	}
	if len(files.writes) != 2 {
		t.Fatalf("files.writes = %+v, want two writes", files.writes)
	}
	if files.writes[0].Dir != files.writes[1].Dir {
		t.Fatalf("the two writes went to %q and %q, want both in the same run directory", files.writes[0].Dir, files.writes[1].Dir)
	}
	if record.Workspace != "/runs/run-1" {
		t.Fatalf("record.Workspace = %q, want the directory both writes used", record.Workspace)
	}
}

func TestTurnRunNeverOpensAWorkingDirectoryForARunThatAsksForNoWrite(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{answered("just prose")}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "explain something", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if files.openCalls != 0 {
		t.Fatalf("OpenRun was called %d times, want 0 for a run that never asked to write", files.openCalls)
	}
	if record.Workspace != "" {
		t.Fatalf("record.Workspace = %q, want empty for a run that never asked to write", record.Workspace)
	}
}

func TestTurnRunRecordsAWriteRejectionReasonInsteadOfFailingTheRun(t *testing.T) {
	t.Parallel()

	const reason = "write rejected: path must be relative to the working directory"
	files := &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must be relative to the working directory", ErrWriteRejected)}
	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("/etc/passwd", "x"),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.StopReason.Reason != Answered {
		t.Fatalf("record.StopReason.Reason = %q, want the run to have carried on to an answer", record.StopReason.Reason)
	}
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != reason {
		t.Fatalf("record.ToolCalls = %+v, want one round carrying the refusal reason %q", record.ToolCalls, reason)
	}
}

func TestTurnRunShowsTheWriteRejectionReasonToTheModelOnTheFollowingCall(t *testing.T) {
	t.Parallel()

	const reason = "write rejected: path must not leave the working directory"
	files := &fakeFiles{dir: "/runs/run-1", writeErr: fmt.Errorf("%w: path must not leave the working directory", ErrWriteRejected)}
	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("../escape.html", "x"),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	if _, _, err := turn.Run(context.Background(), "make a page", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 2 {
		t.Fatalf("the model was called %d times, want 2", len(model.calls))
	}
	prior := model.calls[1].PriorTools
	if len(prior) != 1 || prior[0].Error != reason {
		t.Fatalf("second call's PriorTools = %+v, want the refusal reason %q replayed to the model", prior, reason)
	}
	if prior[0].Path != "../escape.html" || prior[0].Content != "x" {
		t.Fatalf("second call's PriorTools[0] = %+v, want the model's own path and content replayed with it", prior[0])
	}
}

func TestTurnRunReplacesANonRejectionWriteFailureWithAGenericSentenceAndLogsTheDetail(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logged, nil))

	files := &fakeFiles{dir: "/runs/run-1", writeErr: errors.New("no space left on device")}
	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("index.html", "x"),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", logger)

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	const wantGeneric = "file write failed"
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != wantGeneric {
		t.Fatalf("record.ToolCalls = %+v, want the generic sentence %q", record.ToolCalls, wantGeneric)
	}
	if strings.Contains(record.ToolCalls[0].Error, "no space left on device") {
		t.Fatal("the record carries the underlying filesystem error, want it kept off this surface")
	}
	if !strings.Contains(logged.String(), "no space left on device") {
		t.Fatalf("operator log = %q, want the underlying filesystem error named there", logged.String())
	}
}

func TestTurnRunRefusesEveryWriteWhenTheRunHasNoWorkingDirectoryConfigured(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("index.html", "x"),
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	const want = "no working directory is configured"
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != want {
		t.Fatalf("record.ToolCalls = %+v, want one round refused with %q", record.ToolCalls, want)
	}
	if record.Workspace != "" {
		t.Fatalf("record.Workspace = %q, want empty when no working directory exists", record.Workspace)
	}
}

func TestTurnRunRecordsAMalformedWriteRequestWithoutOpeningTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsWrite, RawReason: "tool_calls", ToolError: "tool arguments could not be parsed"},
		answered("done"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if files.openCalls != 0 || len(files.writes) != 0 {
		t.Fatalf("OpenRun called %d times and %d writes attempted, want the working directory untouched by a request that never parsed", files.openCalls, len(files.writes))
	}
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Error != "tool arguments could not be parsed" {
		t.Fatalf("record.ToolCalls = %+v, want the parse failure recorded", record.ToolCalls)
	}
	if record.ToolCalls[0].Tool != ToolWriteFile {
		t.Fatalf("record.ToolCalls[0].Tool = %q, want the round still named as the write tool", record.ToolCalls[0].Tool)
	}
}

func TestTurnRunCountsTheCappingWriteRoundWithoutDispatchingIt(t *testing.T) {
	t.Parallel()

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		wantsWriteOf("a.html", "one"),
		wantsWriteOf("b.html", "two"),
		wantsWriteOf("c.html", "three"),
	}}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make three pages", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !record.CapReached {
		t.Fatal("record.CapReached is false, want true — the third call still wanted the tool")
	}
	if len(files.writes) != 2 {
		t.Fatalf("files.writes = %+v, want exactly two dispatched writes under a cap of %d model calls", files.writes, MaxModelCalls)
	}
	if len(record.ToolCalls) != 3 {
		t.Fatalf("record.ToolCalls has %d entries, want 3 — the capped round is counted", len(record.ToolCalls))
	}
	capped := record.ToolCalls[2]
	if capped.Tool != ToolWriteFile || capped.Path != "c.html" || capped.Error != "call cap reached" {
		t.Fatalf("record.ToolCalls[2] = %+v, want the undispatched write named with the cap reason", capped)
	}
}

func TestRunRecordNamesTheToolOfEveryRoundSoARecallIsNotReadAsAWrite(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{{Candidates: []Candidate{{ID: 7, Name: "Found", Content: "body"}}}}
	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "the missing thing"},
		wantsWriteOf("index.html", "<h1>hi</h1>"),
		answered("done"),
	}}
	turn := NewTurn(graph, model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "make a page", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}

	var decoded struct {
		Workspace string `json:"workspace"`
		ToolCalls []struct {
			Tool  string `json:"tool"`
			Query string `json:"query"`
			Path  string `json:"path"`
			Bytes int    `json:"bytes"`
		} `json:"toolCalls"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode record: %v; body=%s", err, encoded)
	}

	if len(decoded.ToolCalls) != 2 {
		t.Fatalf("toolCalls has %d entries, want 2; body=%s", len(decoded.ToolCalls), encoded)
	}
	if decoded.ToolCalls[0].Tool != "recall" || decoded.ToolCalls[0].Query != "the missing thing" || decoded.ToolCalls[0].Bytes != 0 {
		t.Fatalf("toolCalls[0] = %+v, want the recall round named and carrying its query and no bytes", decoded.ToolCalls[0])
	}
	if decoded.ToolCalls[1].Tool != "writeFile" || decoded.ToolCalls[1].Path != "index.html" || decoded.ToolCalls[1].Bytes != len("<h1>hi</h1>") {
		t.Fatalf("toolCalls[1] = %+v, want the write round named and carrying its path and byte count", decoded.ToolCalls[1])
	}
	if decoded.Workspace != "/runs/run-1" {
		t.Fatalf("workspace = %q, want the run directory on the record", decoded.Workspace)
	}
}
