package loop

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func runSource(t *testing.T, source *NoteSource, graph *fakeGraph, result JudgeResult) noteRun {
	t.Helper()
	notes := &fakeNotes{}
	model := &fakeModel{results: []JudgeResult{result}}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test", testLogger())
	turn.Notes = notes
	record, _, err := turn.Answer(t.Context(), RunOptions{Input: "review", Subject: noteSubject, Settings: DefaultSettings(), View: copyDecisionView(), NoteSource: source})
	if err != nil {
		t.Fatal(err)
	}
	return noteRun{record: record, model: model, notes: notes, graph: graph}
}

func TestSourcePreservesEscapedQuotesNewlinesNonASCIIAndSurrogatePairs(t *testing.T) {
	t.Parallel()
	var source NoteSource
	if err := json.Unmarshal([]byte(`{"speaker":"Zoë \"gardener\"","text":"line one\nlocally adapted seeds 🌱 \ud83c\udf31 \\ud800"}`), &source); err != nil {
		t.Fatal(err)
	}
	if source.Text != "line one\nlocally adapted seeds 🌱 🌱 \\ud800" {
		t.Fatalf("decoded=%q", source.Text)
	}
	run := runSource(t, &source, noteGraph(), copyDecision("summary", noteSubject))
	if len(run.notes.requests) != 1 {
		t.Fatalf("writes=%d", len(run.notes.requests))
	}
	var retained NoteSource
	text := run.notes.requests[0].Text
	if err := json.Unmarshal([]byte(strings.SplitN(text, "Reported source evidence:\n", 2)[1]), &retained); err != nil || retained != source {
		t.Fatalf("retained=%+v err=%v", retained, err)
	}
	if run.noteRound(t).Text != text || run.noteRound(t).Bytes != len(text) {
		t.Fatalf("record differs: %+v", run.noteRound(t))
	}
}

func TestSourceSkipInvalidDecisionAndFinalOverflowNeverWrite(t *testing.T) {
	t.Parallel()
	source := &NoteSource{Speaker: "Sora", Text: strings.Repeat("x", 8192)}
	for _, result := range []JudgeResult{decisionResult(`{"action":"skip","text":"","about":null}`), decisionResult(`{"action":"write","text":"","about":null}`), copyDecision(strings.Repeat("x", 20*1024), noteSubject)} {
		run := runSource(t, source, noteGraph(), result)
		if len(run.model.calls) != 1 || !run.model.calls[0].NoteDecisionOutput {
			t.Fatalf("scenario not judged: %+v", run.model.calls)
		}
		want := "rejected"
		if strings.Contains(result.Answer, `"skip"`) {
			want = "abstained"
		}
		if len(run.notes.requests) != 0 || run.record.NoteDecision.State != want || run.record.NoteDecision.Created != 0 {
			t.Fatalf("unexpected write: %+v", run)
		}
	}
}

func TestSourceRawAndComposedExactCopiesKeepScopedRefusal(t *testing.T) {
	t.Parallel()
	source := &NoteSource{Speaker: "Sora", Text: "original source"}
	encoded, _ := json.Marshal(source)
	final := admittedNoteText + "\n\nReported source evidence:\n" + string(encoded)
	for _, content := range []string{admittedNoteText, final} {
		for _, scope := range []string{"same", "type", "root", "subject"} {
			candidate := scopedCopyCandidate()
			candidate.Content = content
			switch scope {
			case "type":
				candidate.Type = "other"
			case "root":
				candidate.RootNodeID++
			case "subject":
				candidate.DirectSubjectID++
			}
			graph := noteGraph()
			graph.candidates = []Candidate{candidate}
			run := runSource(t, source, graph, copyDecision(admittedNoteText, 71))
			if len(run.model.calls) != 1 || len(run.record.Candidates) != 1 || !run.record.Candidates[0].Included || !strings.Contains(run.model.calls[0].Block, content) {
				t.Fatalf("copy not admitted: %+v", run.record)
			}
			if scope == "same" {
				if len(run.notes.requests) != 0 || run.record.NoteDecision.State != "rejected" || run.noteRound(t).Error != "that exact note is already supplied for this subject: nothing was stored" {
					t.Fatalf("copy escaped: %+v", run)
				}
			} else if len(run.notes.requests) != 1 || run.notes.requests[0].Text != final || run.record.NoteDecision.State != "stored" {
				t.Fatalf("scope broadened: %+v", run)
			}
		}
	}
}

func TestDirectSourceValidationPrecedesAllGraphAndModelWork(t *testing.T) {
	t.Parallel()
	for _, source := range []*NoteSource{{Speaker: "\xff", Text: "x"}, {Speaker: "", Text: "x"}, {Speaker: "Sora", Text: "\xff"}, {Speaker: strings.Repeat("x", 257), Text: "x"}, {Speaker: "Sora", Text: strings.Repeat("x", 8193)}, {Speaker: "Sora", Text: "x"}} {
		for _, view := range []*View{nil, noteView(), decisionView()} {
			if view != nil && view.Note.Mode == NoteModeDecision && source.Speaker == "Sora" && source.Text == "x" {
				continue
			}
			graph := noteGraph()
			model := &fakeModel{}
			notes := &fakeNotes{}
			turn := NewTurn(graph, model, nil, fixedSystem("system"), "test", testLogger())
			turn.Notes = notes
			_, _, err := turn.Answer(t.Context(), RunOptions{Input: "review", Subject: noteSubject, Settings: DefaultSettings(), View: view, NoteSource: source})
			if !errors.Is(err, ErrInvalidNoteSource) || len(graph.nodeCalls) != 0 || len(graph.recallCalls) != 0 || len(model.calls) != 0 || len(notes.requests) != 0 {
				t.Fatalf("invalid direct input proceeded: err=%v graph=%+v model=%+v", err, graph, model)
			}
		}
	}
}

func TestSourceFinalContentCeilingAcceptsExactLimitAndRejectsOneMoreByte(t *testing.T) {
	t.Parallel()
	source := &NoteSource{Speaker: strings.Repeat("s", 256), Text: strings.Repeat("x", 8192)}
	encoded, _ := json.Marshal(source)
	overhead := len("\n\nReported source evidence:\n") + len(encoded)
	for _, extra := range []int{0, 1} {
		proposal := strings.Repeat("a", 20*1024-overhead+extra)
		run := runSource(t, source, noteGraph(), copyDecision(proposal, noteSubject))
		if len(run.model.calls) != 1 || len(run.model.calls[0].Offered) != 0 {
			t.Fatalf("missing private selection: %+v", run.model.calls)
		}
		if extra == 0 {
			if len(run.notes.requests) != 1 || len(run.notes.requests[0].Text) != 20*1024 || run.record.NoteDecision.State != "stored" {
				t.Fatalf("exact ceiling rejected: %+v", run)
			}
		} else if len(run.notes.requests) != 0 || run.record.NoteDecision.State != "rejected" {
			t.Fatalf("oversize stored: %+v", run)
		}
	}
}
