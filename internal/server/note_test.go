package server

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

type recordingNotes struct {
	requests []loop.NoteRequest
}

func (n *recordingNotes) WriteNote(_ context.Context, request loop.NoteRequest) loop.WriteReceipt {
	n.requests = append(n.requests, request)
	return loop.WriteReceipt{State: loop.Stored, NodeID: 6100}
}

type noteWritingModel struct {
	offered [][]string
}

func (m *noteWritingModel) Derive(context.Context, string, int) (loop.DeriveResult, error) {
	return loop.DeriveResult{}, nil
}

func (m *noteWritingModel) Judge(_ context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.offered = append(m.offered, in.Offered)
	if slices.Contains(in.Offered, loop.ToolNote) {
		return loop.JudgeResult{Reason: loop.WantsNote, RawReason: "tool_calls", NoteText: "a thought of my own"}, nil
	}
	return loop.JudgeResult{Reason: loop.Answered, RawReason: "stop", Answer: "an answer"}, nil
}

func runWithNotes(t *testing.T, graph *configGraph, body string) (*httpResult, *noteWritingModel, *recordingNotes) {
	t.Helper()

	model := &noteWritingModel{}
	notes := &recordingNotes{}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system text"), "test-model", testLogger())
	turn.Notes = notes
	rec := postRunsVerbose(t, turn, body)
	return &httpResult{code: rec.Code, body: rec.Body.String()}, model, notes
}

func TestAnInvalidNoteIsAnInvalidConfigNamingTheKeyAndNothingRunsOrIsFiled(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"a string":                `{"note": "thought"}`,
		"no type":                 `{"note": {}}`,
		"a forbidden type":        `{"note": {"type": "session-log"}}`,
		"a type the filter hides": `{"filter": {"type": ["knowledge"]}, "note": {"type": "thought"}}`,
	} {
		graph := &configGraph{content: content}
		result, model, notes := runWithNotes(t, graph, configPostBodyHead+`77}`)

		if result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_config" {
			t.Errorf("%s: status = %d code = %q, want 400 invalid_config; body=%s", name, result.code, result.errorCode(t), result.body)
			continue
		}
		if !strings.Contains(result.errorMessage(), `"note"`) {
			t.Errorf("%s: the message %q does not name note", name, result.errorMessage())
		}
		if len(graph.recallCalls) != 0 || len(model.offered) != 0 || len(notes.requests) != 0 || len(graph.written) != 0 {
			t.Errorf("%s: the run went on: %d recalls, %d judgement calls, %d notes, %d records filed", name, len(graph.recallCalls), len(model.offered), len(notes.requests), len(graph.written))
		}
	}
}

func TestAValidNoteReachesTheTurnWhichOffersTheToolAndFilesItsNoteUnderTheConfigsRootAndType(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{"filter": {"type": ["thought", "knowledge"]}, "note": {"type": "thought"}}`, configRoot: 8800}
	result, model, notes := runWithNotes(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	if len(model.offered) == 0 || !slices.Contains(model.offered[0], loop.ToolNote) {
		t.Fatalf("the first call offered %v, want the note tool", model.offered)
	}

	want := loop.NoteRequest{Type: "thought", Name: "a thought of my own", Text: "a thought of my own", Root: 8800, Subject: configSubjectID}
	if len(notes.requests) != 1 || notes.requests[0] != want {
		t.Fatalf("the port received %+v, want exactly %+v", notes.requests, want)
	}

	var record loop.Record
	if err := json.Unmarshal([]byte(result.body), &record); err != nil {
		t.Fatalf("the verbose body is not a record: %v", err)
	}
	if record.Perspective == nil || record.Perspective.Note == nil || record.Perspective.Note.Type != "thought" {
		t.Errorf("the record carries %+v, want the note type in force", record.Perspective)
	}
}

func TestAConfigWithoutANoteNeverOffersTheToolEvenWhereTheDeploymentHoldsAPort(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{"filter": {"type": ["thought"]}}`}
	result, model, notes := runWithNotes(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	for _, offered := range model.offered {
		if slices.Contains(offered, loop.ToolNote) {
			t.Errorf("a call offered %v, want no note tool: it is on per config, never per deployment", offered)
		}
	}
	if len(notes.requests) != 0 {
		t.Errorf("the port received %+v, want nothing", notes.requests)
	}
}

func TestANoteFieldOnTheRequestBodyIsIgnored(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{}`}
	result, model, notes := runWithNotes(t, graph, `{"input":"hello","subject":42,"config":77,"note":{"type":"session-log"}}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	for _, offered := range model.offered {
		if slices.Contains(offered, loop.ToolNote) {
			t.Errorf("a call offered %v, want no note tool: a request cannot enable it", offered)
		}
	}
	if len(notes.requests) != 0 {
		t.Errorf("the port received %+v, want nothing", notes.requests)
	}
}
