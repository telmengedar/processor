package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const admittedNoteText = "JunoMemory_f58fd5b4b does free smartphone tutorials for older neighbors on Wednesday evenings since 2019 to combat isolation."

func copyDecisionView() *View {
	view := decisionView()
	view.Form = FormContent
	return view
}

func scopedCopyCandidate() Candidate {
	return Candidate{ID: 71, Type: noteType, Name: "remembered", Similarity: 0.9, Content: admittedNoteText, Substance: admittedNoteText, RootNodeID: noteRoot, DirectSubjectID: noteSubject}
}

func copyDecision(text string, about int64) JudgeResult {
	raw, _ := json.Marshal(map[string]any{"action": "write", "text": text, "about": about})
	return decisionResult(string(raw))
}

func TestExactAboutCopyRejectsWithoutStorageOrCreatedReceipt(t *testing.T) {
	t.Parallel()
	graph := noteGraph()
	graph.candidates = []Candidate{scopedCopyCandidate()}
	run := runNotes(t, graph, copyDecisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{copyDecision(admittedNoteText, 71)}})
	if len(run.model.calls) != 1 || !strings.Contains(run.model.calls[0].Block, admittedNoteText) || len(run.record.Candidates) != 1 || !run.record.Candidates[0].Included {
		t.Fatalf("copy target was not admitted: %+v", run.record)
	}
	tool := run.noteRound(t)
	if run.record.NoteDecision.State != "rejected" || run.record.NoteDecision.Created != 0 || len(run.notes.requests) != 0 || tool.Source != ToolSourceStructured || tool.Written != "" || tool.Created != 0 || tool.Error != "that exact note is already supplied for this subject: nothing was stored" || tool.Text != admittedNoteText || tool.About != 71 || run.record.Answer != "" || run.record.Outcome.Produced {
		t.Fatalf("false duplicate outcome: receipt=%+v tool=%+v writes=%+v", run.record.NoteDecision, tool, run.notes.requests)
	}
}

func TestExactAboutCopyRequiresMatchingTypeRootAndSubjectProof(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*Candidate){
		"wrong root":      func(c *Candidate) { c.RootNodeID++ },
		"missing root":    func(c *Candidate) { c.RootNodeID = 0 },
		"negative root":   func(c *Candidate) { c.RootNodeID = -1 },
		"wrong type":      func(c *Candidate) { c.Type = "other" },
		"missing subject": func(c *Candidate) { c.DirectSubjectID = 0 },
		"wrong subject":   func(c *Candidate) { c.DirectSubjectID++ },
	} {
		t.Run(name, func(t *testing.T) {
			c := scopedCopyCandidate()
			change(&c)
			graph := noteGraph()
			graph.candidates = []Candidate{c}
			run := runNotes(t, graph, copyDecisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{copyDecision(admittedNoteText, 71)}})
			if len(run.record.Candidates) != 1 || !run.record.Candidates[0].Included || len(run.notes.requests) != 1 || run.record.NoteDecision.State != "stored" {
				t.Fatalf("unproved target rejected: %+v", run.record)
			}
		})
	}
}

func TestStructuredNoteWithNewTextAboutAnAdmittedNoteStillWrites(t *testing.T) {
	t.Parallel()
	graph := noteGraph()
	graph.candidates = []Candidate{scopedCopyCandidate()}
	text := admittedNoteText + " She also teaches on Saturdays."
	run := runNotes(t, graph, copyDecisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{copyDecision(text, 71)}})
	if len(run.notes.requests) != 1 || run.notes.requests[0].Text != text || run.notes.requests[0].About != 71 || run.record.NoteDecision.State != "stored" {
		t.Fatalf("new text lost: %+v", run.record)
	}
}

func TestStructuredNoteWithOneTrailingSpaceDifferenceFromAdmittedContentStillWrites(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"proposal has trailing space", "stored content has trailing space"} {
		t.Run(name, func(t *testing.T) {
			candidate := scopedCopyCandidate()
			text := admittedNoteText + " "
			if name == "stored content has trailing space" {
				candidate.Content += " "
				text = admittedNoteText
			}
			graph := noteGraph()
			graph.candidates = []Candidate{candidate}
			run := runNotes(t, graph, copyDecisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{copyDecision(text, 71)}})
			if len(run.model.calls) != 1 || !strings.Contains(run.model.calls[0].Block, candidate.Content) || len(run.record.Candidates) != 1 || run.record.Candidates[0].ID != 71 || !run.record.Candidates[0].Included {
				t.Fatalf("space-different target was not admitted: %+v", run.record)
			}
			tool := run.noteRound(t)
			if len(run.notes.requests) != 1 || run.notes.requests[0].Text != text || run.notes.requests[0].About != 71 || run.record.NoteDecision.State != "stored" || tool.Text != text || tool.Error != "" || tool.Written != Stored {
				t.Fatalf("space-different text lost: receipt=%+v tool=%+v writes=%+v", run.record.NoteDecision, tool, run.notes.requests)
			}
		})
	}
}

func TestExactCopyForAnotherAboutTargetDoesNotCollapseDistinctMeaning(t *testing.T) {
	t.Parallel()
	graph := noteGraph(72)
	graph.candidates = append(graph.candidates, scopedCopyCandidate())
	run := runNotes(t, graph, copyDecisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{copyDecision(admittedNoteText, 72)}})
	if len(run.notes.requests) != 1 || run.notes.requests[0].About != 72 || run.record.NoteDecision.State != "stored" {
		t.Fatalf("other about collapsed: %+v", run.record)
	}
}

func TestDirectScopeProofSurvivesCandidateAdmission(t *testing.T) {
	t.Parallel()
	for _, form := range []Form{FormContent, FormSubstance} {
		for _, available := range []bool{false, true} {
			for _, admitted := range []bool{false, true} {
				c := scopedCopyCandidate()
				if !available {
					c.Substance = ""
				}
				_, dispositions := Assemble(Anchor{}, []Candidate{c}, 1000, form)
				dispositions[0].Included = admitted
				memory := newWorkingMemory(1000, Anchor{}, true, includedCandidates([]Candidate{c}, dispositions), dispositions, form)
				if got := memory.exactAboutCopy(JudgeResult{NoteText: admittedNoteText, NoteAbout: 71}, noteScope{nodeType: noteType, root: noteRoot}, noteSubject); got != (form == FormContent && admitted) {
					t.Fatalf("form=%s available=%v admitted=%v copy=%v held=%+v", form, available, admitted, got, memory.rows)
				}
			}
		}
	}
}

func TestNativeNoteCopyRetainsExistingBehavior(t *testing.T) {
	t.Parallel()
	graph := noteGraph()
	graph.candidates = []Candidate{scopedCopyCandidate()}
	run := runNoteResults(t, graph, wantsNoteOf(admittedNoteText, 71), answeredFinal())
	if len(run.notes.requests) != 1 || run.notes.requests[0].Text != admittedNoteText || run.noteRound(t).Written != Stored {
		t.Fatalf("native note changed: %+v", run.record)
	}
}

func TestDecisionScopeRequestIsInitialAndModeOnly(t *testing.T) {
	t.Parallel()
	for _, private := range []bool{false, true} {
		view := noteView()
		if private {
			view = copyDecisionView()
		}
		graph := noteGraph()
		model := &fakeModel{results: []JudgeResult{decisionResult(`{"action":"skip","text":"","about":null}`)}}
		run := runNotes(t, graph, view, &fakeNotes{}, model)
		want := int64(0)
		if private {
			want = noteSubject
		}
		if len(graph.recallCalls) != 1 || graph.recallCalls[0].NoteContextSubject != want {
			t.Fatalf("private=%v recalls=%+v", private, graph.recallCalls)
		}
		if private && (run.record.NoteDecision.State != "abstained" || len(run.notes.requests) != 0) {
			t.Fatalf("skip changed: %+v", run.record)
		}
	}
	graph := noteGraph()
	_, err := Retrieve(context.Background(), graph, Anchor{ID: noteSubject}, []string{"query"}, 10, UpdateWindow{}, nil, testLogger())
	if err != nil || len(graph.recallCalls) != 1 || graph.recallCalls[0].NoteContextSubject != 0 {
		t.Fatalf("native Retrieve changed: %v %+v", err, graph.recallCalls)
	}
}

func TestCopyGuardLeavesValidSkipAsAbstainedEvenWithAnAdmittedExactNote(t *testing.T) {
	t.Parallel()
	graph := noteGraph()
	graph.candidates = []Candidate{scopedCopyCandidate()}
	run := runNotes(t, graph, copyDecisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{decisionResult(`{"action":"skip","text":"","about":null}`)}})
	if len(run.model.calls) != 1 || len(run.record.Candidates) != 1 || !run.record.Candidates[0].Included {
		t.Fatalf("missing admitted skip scenario: %+v", run.record)
	}
	if run.record.NoteDecision.State != "abstained" || run.record.NoteDecision.Created != 0 || len(run.notes.requests) != 0 || len(run.record.ToolCalls) != 0 {
		t.Fatalf("skip mislabeled: %+v", run.record)
	}
}

func TestExactCopyWithoutPositiveConfiguredRootStillFollowsTheWriter(t *testing.T) {
	t.Parallel()
	graph := noteGraph()
	candidate := scopedCopyCandidate()
	candidate.RootNodeID = 0
	graph.candidates = []Candidate{candidate}
	view := copyDecisionView()
	view.Root = 0
	run := runNotes(t, graph, view, &fakeNotes{}, &fakeModel{results: []JudgeResult{copyDecision(admittedNoteText, 71)}})
	if len(run.notes.requests) != 1 || run.notes.requests[0].Root != 0 || run.record.NoteDecision.State != "stored" {
		t.Fatalf("unrooted copy rejected: %+v", run.record)
	}
}
