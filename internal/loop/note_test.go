package loop

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	noteSubject = 42
	noteRoot    = 500
	noteConfig  = 9
	noteNodeID  = 900
	noteType    = "thought"
)

type fakeNotes struct {
	requests []NoteRequest
	receipt  WriteReceipt
}

func (f *fakeNotes) WriteNote(_ context.Context, request NoteRequest) WriteReceipt {
	f.requests = append(f.requests, request)
	if f.receipt.State == "" {
		return WriteReceipt{State: Stored, NodeID: noteNodeID}
	}
	return f.receipt
}

func wantsNoteOf(text string, about int64) JudgeResult {
	return JudgeResult{Reason: WantsNote, RawReason: "tool_calls", NoteText: text, NoteAbout: about}
}

func noteView() *View {
	return &View{Config: noteConfig, Root: noteRoot, Note: &NoteConfig{Type: noteType}}
}

func noteGraph(shown ...int64) *fakeGraph {
	graph := baseGraph()
	for _, id := range shown {
		graph.candidates = append(graph.candidates, Candidate{ID: id, Type: noteType, Name: "shown", Similarity: 0.9, Content: "body", Substance: "substance"})
	}
	return graph
}

type noteRun struct {
	record Record
	model  *fakeModel
	notes  *fakeNotes
	graph  *fakeGraph
}

func runNotes(t *testing.T, graph *fakeGraph, view *View, notes *fakeNotes, model *fakeModel) noteRun {
	t.Helper()

	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	if notes != nil {
		turn.Notes = notes
	}
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: noteSubject, Settings: DefaultSettings(), View: view})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	return noteRun{record: record, model: model, notes: notes, graph: graph}
}

func runNoteResults(t *testing.T, graph *fakeGraph, results ...JudgeResult) noteRun {
	t.Helper()
	return runNotes(t, graph, noteView(), &fakeNotes{}, &fakeModel{results: results})
}

func (r noteRun) offeredOn(call int) []string {
	return r.model.calls[call].Offered
}

func (r noteRun) noteRound(t *testing.T) ToolCallRecord {
	t.Helper()

	var rounds []ToolCallRecord
	for _, call := range r.record.ToolCalls {
		if call.Tool == ToolNote {
			rounds = append(rounds, call)
		}
	}
	if len(rounds) != 1 {
		t.Fatalf("the record carries %d note rounds, want exactly one: %+v", len(rounds), r.record.ToolCalls)
	}
	return rounds[0]
}

func offeredNote(offered []string) bool {
	return slices.Contains(offered, ToolNote)
}

func TestTheNoteToolIsOfferedOnlyToARunWhoseConfigEnablesItAndWhoseTurnHoldsAPort(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		view  *View
		notes *fakeNotes
		want  bool
	}{
		"no config at all":                {nil, &fakeNotes{}, false},
		"a config without a note":         {&View{Config: noteConfig, Root: noteRoot}, &fakeNotes{}, false},
		"a note and no port":              {noteView(), nil, false},
		"a note, a port and a config key": {noteView(), &fakeNotes{}, true},
	}
	for name, tc := range cases {
		run := runNotes(t, noteGraph(), tc.view, tc.notes, &fakeModel{results: []JudgeResult{recallFor("more"), answeredFinal()}})

		if len(run.model.calls) != 2 {
			t.Fatalf("%s: the run made %d calls, want 2", name, len(run.model.calls))
		}
		for i := range run.model.calls {
			if got := offeredNote(run.offeredOn(i)); got != tc.want {
				t.Errorf("%s: call %d offered %v, the note tool offered = %v, want %v: the tool exists on a call only when the config enables it and the turn can file it", name, i+1, run.offeredOn(i), got, tc.want)
			}
		}
	}
}

func TestTheNoteToolIsOfferedAfterTheFileToolAndNeverOnTheReservedCall(t *testing.T) {
	t.Parallel()

	notes := &fakeNotes{}
	model := &fakeModel{results: researchToTheCap()}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, &fakeFiles{dir: "/runs/1"}, fixedSystem("system"), "test-model", testLogger())
	turn.Notes = notes

	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: noteSubject, Settings: DefaultSettings(), View: noteView()})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if !record.CapReached || len(model.calls) != MaxModelCalls {
		t.Fatalf("the run made %d calls with the cap reached = %v, want %d calls reaching the reserved call", len(model.calls), record.CapReached, MaxModelCalls)
	}

	for i, call := range model.calls[:MaxModelCalls-1] {
		if want := []string{ToolRecall, ToolReadNode, ToolWriteFile, ToolNote}; !slices.Equal(call.Offered, want) {
			t.Errorf("call %d offered %v, want %v", i+1, call.Offered, want)
		}
	}
	if last := model.calls[MaxModelCalls-1]; len(last.Offered) != 0 {
		t.Errorf("the reserved answering call offered %v, want no tool: the note tool is never offered on the call that only answers", last.Offered)
	}
}

func TestAModelThatAsksForANoteOnTheReservedCallStoresNothingAndTheRoundSaysSo(t *testing.T) {
	t.Parallel()

	results := researchToTheCap()
	results[MaxModelCalls-1] = wantsNoteOf("a late thought", 0)
	model := &fakeModel{results: results, ignoresWithheldToolList: true}
	run := runNotes(t, graphYieldingNewRowsToEveryRecall(), noteView(), &fakeNotes{}, model)

	if len(run.notes.requests) != 0 {
		t.Fatalf("the reserved call's note request reached the port: %+v", run.notes.requests)
	}
	round := run.noteRound(t)
	if round.Error != errReservedCallRefused || round.Text != "a late thought" {
		t.Errorf("the reserved call's note round = %+v, want the reserved-call refusal with the text on the record", round)
	}
}

func TestTheNoteToolIsWithdrawnForTheRestOfTheTurnWhateverBecameOfTheNote(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		first   JudgeResult
		receipt WriteReceipt
	}{
		"a stored note":                  {wantsNoteOf("a thought", 0), WriteReceipt{State: Stored, NodeID: noteNodeID}},
		"an unlinked note":               {wantsNoteOf("a thought", 0), WriteReceipt{State: Unlinked, NodeID: noteNodeID}},
		"a note the graph did not store": {wantsNoteOf("a thought", 0), WriteReceipt{State: NotStored}},
		"a note refused by validation":   {wantsNoteOf("   ", 0), WriteReceipt{}},
		"a malformed note call":          {JudgeResult{Reason: WantsNote, RawReason: "tool_calls", ToolError: "tool arguments carried no text to store"}, WriteReceipt{}},
	}
	for name, tc := range cases {
		model := &fakeModel{results: []JudgeResult{tc.first, recallFor("more"), recallFor("again"), answeredFinal()}}
		run := runNotes(t, graphYieldingNewRowsToEveryRecall(), noteView(), &fakeNotes{receipt: tc.receipt}, model)

		if len(model.calls) != 4 {
			t.Fatalf("%s: the run made %d calls, want 4", name, len(model.calls))
		}
		if !offeredNote(run.offeredOn(0)) {
			t.Errorf("%s: call 1 offered %v, want the note tool", name, run.offeredOn(0))
		}
		for i := 1; i < len(model.calls); i++ {
			if offeredNote(run.offeredOn(i)) {
				t.Errorf("%s: call %d offered %v, want the note tool withdrawn: the model is never shown what became of its note, so only withdrawal can stop a second one", name, i+1, run.offeredOn(i))
			}
		}
	}
}

func TestAModelThatKeepsAskingForNotesFilesExactlyOneAndTheRestAreRecordedAsNotOffered(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{wantsNoteOf("first", 0), wantsNoteOf("second", 0), wantsNoteOf("third", 0)}, ignoresWithheldToolList: true}
	run := runNotes(t, noteGraph(), noteView(), &fakeNotes{}, model)

	if len(run.notes.requests) != 1 || run.notes.requests[0].Text != "first" {
		t.Fatalf("the port received %+v, want exactly the first note", run.notes.requests)
	}

	var errs []string
	for _, call := range run.record.ToolCalls {
		errs = append(errs, call.Error)
	}
	if len(errs) < 2 || errs[0] != "" || errs[1] != errNoteNotOffered {
		t.Errorf("the rounds carry the errors %q, want the first filed and the second refused as not offered", errs)
	}
}

func TestTheNoteIsFiledWithTheConfigsTypeAndRootTheRequestsSubjectAndTheTextByteForByte(t *testing.T) {
	t.Parallel()

	const text = "\n  First line   with\tspaces \nsecond line, with ünicode — and a trailing newline\n"
	call := wantsNoteOf(text, 0)
	call.RecallQuery = "a different query"
	call.WritePath = "../elsewhere"
	call.WriteContent = "other content"
	run := runNoteResults(t, noteGraph(), call, answeredFinal())

	want := NoteRequest{Type: noteType, Name: "First line with spaces", Text: text, Root: noteRoot, Subject: noteSubject, About: 0}
	if len(run.notes.requests) != 1 || run.notes.requests[0] != want {
		t.Fatalf("the port received %+v, want exactly %+v: the type and root are the config's, the subject the request's, the text the model's verbatim and the name derived from it", run.notes.requests, want)
	}
}

func TestANoteOfAConfigWithoutARootIsFiledWithNone(t *testing.T) {
	t.Parallel()

	view := &View{Config: noteConfig, Note: &NoteConfig{Type: noteType}}
	run := runNotes(t, noteGraph(), view, &fakeNotes{}, &fakeModel{results: []JudgeResult{wantsNoteOf("a thought", 0), answeredFinal()}})

	if len(run.notes.requests) != 1 || run.notes.requests[0].Root != 0 {
		t.Fatalf("the port received %+v, want a request with no root", run.notes.requests)
	}
}

func TestNoteNameIsTheFirstNonBlankLineCollapsedAndCutToEightyRunesWithAnEllipsis(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ text, want string }{
		"one line":            {"a thought", "a thought"},
		"collapsed":           {"a \t thought\u00a0 here", "a thought here"},
		"the first line only": {"first\nsecond", "first"},
		"after blank lines":   {"\n \n  third line\nnext", "third line"},
		"exactly at the cut":  {strings.Repeat("é", 80), strings.Repeat("é", 80)},
		"one rune past":       {strings.Repeat("é", 81), strings.Repeat("é", 80) + "…"},
	} {
		if got := NoteName(tc.text); got != tc.want {
			t.Errorf("%s: NoteName = %q, want %q", name, got, tc.want)
		}
	}
}

func TestAnAboutIsLinkedOnlyWhenTheTurnShowedItAndIsDroppedNotRefusedOtherwise(t *testing.T) {
	t.Parallel()

	rows := []recallResponse{
		{Candidates: []Candidate{{ID: 7, Type: noteType, Name: "initial", Similarity: 0.9, Content: "body", Substance: "substance"}}},
		{Candidates: []Candidate{{ID: 8, Type: noteType, Name: "recalled", Similarity: 0.8, Content: "body", Substance: "substance"}}},
	}

	cases := map[string]struct {
		results     []JudgeResult
		about       int64
		wantAbout   int64
		wantDropped bool
	}{
		"a row of the initial block":  {[]JudgeResult{wantsNoteOf("t", 7), answeredFinal()}, 7, 7, false},
		"a row a recall brought":      {[]JudgeResult{recallFor("more"), wantsNoteOf("t", 8), answeredFinal()}, 8, 8, false},
		"an id never shown":           {[]JudgeResult{wantsNoteOf("t", 999), answeredFinal()}, 999, 0, true},
		"an id a recall would return": {[]JudgeResult{wantsNoteOf("t", 8), answeredFinal()}, 8, 0, true},
		"the subject":                 {[]JudgeResult{wantsNoteOf("t", noteSubject), answeredFinal()}, noteSubject, 0, false},
		"none":                        {[]JudgeResult{wantsNoteOf("t", 0), answeredFinal()}, 0, 0, false},
	}
	for name, tc := range cases {
		graph := baseGraph()
		graph.recallQueue = slices.Clone(rows)
		run := runNoteResults(t, graph, tc.results...)

		if len(run.notes.requests) != 1 {
			t.Fatalf("%s: the port received %d requests, want 1: a note whose link is dropped is still written", name, len(run.notes.requests))
		}
		if got := run.notes.requests[0]; got.About != tc.wantAbout || got.Subject != noteSubject {
			t.Errorf("%s: the request linked about %d and subject %d, want about %d and subject %d", name, got.About, got.Subject, tc.wantAbout, noteSubject)
		}
		round := run.noteRound(t)
		if round.About != tc.about || round.AboutDropped != tc.wantDropped || round.Error != "" {
			t.Errorf("%s: the round = about %d dropped %v error %q, want about %d dropped %v and no error", name, round.About, round.AboutDropped, round.Error, tc.about, tc.wantDropped)
		}
	}
}

func TestANoteOfAnySizeIsFiledWholeAndOnlyBlankInvalidOrRunNamedTextIsRefused(t *testing.T) {
	t.Parallel()

	accepted := map[string]string{
		"10 KB of ASCII":        strings.Repeat("a", 10*1024),
		"10 KB of multi-byte":   strings.Repeat("é", 5*1024),
		"many lines":            strings.Repeat("a line of a longer note\n", 1000),
		"a single character":    "x",
		"a run name mid-text":   "the processor-run prefix is only refused at the start",
		"a prefix-like start":   "processor runs are what recall leaves out",
		"the prefix, truncated": "processor-ru is not the prefix",
	}
	for name, text := range accepted {
		run := runNoteResults(t, noteGraph(), wantsNoteOf(text, 0), answeredFinal())
		if len(run.notes.requests) != 1 {
			t.Errorf("%s: the port received %d requests, want the note filed", name, len(run.notes.requests))
			continue
		}
		if got := run.notes.requests[0].Text; got != text {
			t.Errorf("%s: the port received %d bytes of text, want the %d bytes the model wrote, byte for byte", name, len(got), len(text))
		}
		if round := run.noteRound(t); round.Error != "" || round.Text != text || round.Bytes != len(text) {
			t.Errorf("%s: the round carries %d bytes of text, Bytes %d and error %q, want an accepted note carrying its text whole and %d bytes", name, len(round.Text), round.Bytes, round.Error, len(text))
		}
	}

	refused := map[string]struct{ text, wantError string }{
		"empty":                       {"", errNoteBlank},
		"spaces and newlines":         {"  \n\t \u00a0 \n", errNoteBlank},
		"invalid UTF-8":               {"a thought\xff", errNoteNotUTF8},
		"the run prefix":              {"processor-run 2026-10-14 — what changed", errNoteRunName},
		"the run prefix, upper case":  {"PROCESSOR-RUN notes", errNoteRunName},
		"the run prefix after blanks": {"\n  processor-run   x\nmore", errNoteRunName},
	}
	for name, tc := range refused {
		run := runNoteResults(t, noteGraph(), wantsNoteOf(tc.text, 0), answeredFinal())

		if len(run.notes.requests) != 0 {
			t.Errorf("%s: the port received %+v, want no call for a refused note", name, run.notes.requests)
			continue
		}
		round := run.noteRound(t)
		if !strings.Contains(round.Error, tc.wantError) {
			t.Errorf("%s: the round error = %q, want it to contain %q", name, round.Error, tc.wantError)
		}
		if round.Written != "" || round.Created != 0 {
			t.Errorf("%s: the round = %+v, want nothing written", name, round)
		}
	}
}

func TestARefusedNoteDoesNotReserveTheNextCallForAnswering(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, graphYieldingNewRowsToEveryRecall(), wantsNoteOf(" ", 0), recallFor("more"), answeredFinal())

	if len(run.model.calls) != 3 {
		t.Fatalf("the run made %d calls, want 3", len(run.model.calls))
	}
	if !slices.Contains(run.offeredOn(1), ToolRecall) {
		t.Errorf("call 2 offered %v, want recall still offered: a bad note must not cost the model its remaining recall", run.offeredOn(1))
	}
	if run.record.ReservedCall.State != "" || run.record.CapReached {
		t.Errorf("the record reserved a call (%+v, cap %v), want none", run.record.ReservedCall, run.record.CapReached)
	}
	if got := run.record.ToolCalls[1]; got.Tool != ToolRecall || got.Error != "" {
		t.Errorf("the second round = %+v, want a dispatched recall", got)
	}
}

func TestANoteRoundIsNotRetrievalItAbsorbsNoRowAddsNoYieldAndLeavesTheTrailerOfTheNextCallAlone(t *testing.T) {
	t.Parallel()

	withNote := runNoteResults(t, noteGraph(7), wantsNoteOf("a thought", 7), answeredFinal())
	without := runNoteResults(t, noteGraph(7), answeredFinal())

	if len(withNote.model.calls) != 2 || len(without.model.calls) != 1 {
		t.Fatalf("the runs made %d and %d calls, want 2 and 1", len(withNote.model.calls), len(without.model.calls))
	}
	if got, want := withNote.model.calls[1].Block, without.model.calls[0].Block; got != want {
		t.Errorf("the block after a note round differs from the block of a run without one:\n%q\nwant\n%q", got, want)
	}
	if got, want := withNote.model.calls[1].Trailer, without.model.calls[0].Trailer; got != want {
		t.Errorf("the trailer after a note round = %q, want %q: a note is not a search", got, want)
	}
	if got, want := withNote.record.Trailers, []TrailerKind{TrailerClause, TrailerClause}; !slices.Equal(got, want) {
		t.Errorf("the trailers = %v, want %v", got, want)
	}
	if round := withNote.noteRound(t); round.Yield != 0 || len(round.Results) != 0 {
		t.Errorf("the note round = %+v, want no yield and no result rows", round)
	}
}

func TestANoteRoundBetweenBarrenRecallsDoesNotCountTowardsClosingRecall(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, graphReturningTheSameRowsToEveryRecall(), recallFor("first"), recallFor("second"), wantsNoteOf("a thought", 0), recallFor("third"), answeredFinal())

	if run.record.RecallClosed {
		t.Fatal("recall was closed: the note round counted as a barren round, but only a recall round that brought nothing new does")
	}
	rounds := run.record.ToolCalls
	if len(rounds) != 4 || rounds[3].Tool != ToolRecall || rounds[3].Error != "" {
		t.Fatalf("the rounds = %+v, want the third recall dispatched after the note", rounds)
	}
}

func TestANoteRoundNeitherCountsAsAFailedRetrievalNorChangesWhatTheOutcomeSaysAboutIt(t *testing.T) {
	t.Parallel()

	withNote := runNoteResults(t, noteGraph(7), wantsNoteOf("a thought", 0), answered("the answer"))
	without := runNoteResults(t, noteGraph(7), answered("the answer"))

	got, want := withNote.record.Outcome, without.record.Outcome
	if got.Verdict != want.Verdict || got.Grounded != want.Grounded || got.Curtailed != want.Curtailed || got.Produced != want.Produced {
		t.Errorf("the outcome with a note round = %+v, want the verdict, grounding and curtailment of the run without one: %+v", got, want)
	}
	if !maps.Equal(got.Acted, map[string]int{ToolNote: 1}) {
		t.Errorf("Acted = %v, want exactly one completed note round", got.Acted)
	}

	refused := runNoteResults(t, noteGraph(7), wantsNoteOf(" ", 0), answered("the answer"))
	if len(refused.record.Outcome.Acted) != 0 {
		t.Errorf("Acted = %v for a refused note, want none: only rounds that completed without error are counted", refused.record.Outcome.Acted)
	}
}

func TestANoteRoundCarriesTheTextTheAboutTheCreatedIdAndWhereItLandedOnTheRecord(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, noteGraph(7), wantsNoteOf("a thought, verbatim", 7), answeredFinal())

	round := run.noteRound(t)
	want := ToolCallRecord{Tool: ToolNote, Bytes: len("a thought, verbatim"), Results: []Disposition{}, Text: "a thought, verbatim", About: 7, Created: noteNodeID, Written: Stored}
	gotJSON, _ := json.Marshal(round)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("the note round = %s, want %s", gotJSON, wantJSON)
	}
	if run.record.Perspective == nil || run.record.Perspective.Note == nil || run.record.Perspective.Note.Type != noteType {
		t.Errorf("the record's perspective = %+v, want it to state the note type in force", run.record.Perspective)
	}
}

func TestARunWithoutANoteRoundSerialisesNoNoteKeyAnywhereAndAnArchivedRecordStillDecodes(t *testing.T) {
	t.Parallel()

	run := runNotes(t, graphYieldingNewRowsToEveryRecall(), &View{Config: noteConfig}, &fakeNotes{}, &fakeModel{results: []JudgeResult{recallFor("more"), answeredFinal()}})

	encoded, err := json.Marshal(run.record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var generic struct {
		Perspective map[string]json.RawMessage   `json:"perspective"`
		ToolCalls   []map[string]json.RawMessage `json:"toolCalls"`
	}
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, present := generic.Perspective["note"]; present {
		t.Errorf("the perspective of a run without a note carries a note key: %v", generic.Perspective)
	}
	if len(generic.ToolCalls) != 1 {
		t.Fatalf("the record carries %d rounds, want the one recall", len(generic.ToolCalls))
	}
	for _, key := range []string{"text", "about", "aboutDropped", "created", "written"} {
		if _, present := generic.ToolCalls[0][key]; present {
			t.Errorf("a recall round serialises the key %q: the note members must be absent from every round that was not a note round", key)
		}
	}

	const archived = `{"perspective":{"config":9,"silence":true},"toolCalls":[{"tool":"recall","bytes":0,"results":[],"yield":0},{"tool":"writeFile","path":"a","bytes":3,"results":[],"yield":0}]}`
	var decoded Record
	if err := json.Unmarshal([]byte(archived), &decoded); err != nil {
		t.Fatalf("an archived record no longer decodes: %v", err)
	}
	if len(decoded.ToolCalls) != 2 || decoded.Perspective == nil || decoded.Perspective.Note != nil {
		t.Errorf("the archived record decoded to %+v, want its rounds and no note", decoded)
	}
}

func TestAPortFailureDoesNotFailTheRunWhichStillAnswersAndFilesItsRecord(t *testing.T) {
	t.Parallel()

	notes := &fakeNotes{receipt: WriteReceipt{State: NotStored}}
	model := &fakeModel{results: []JudgeResult{wantsNoteOf("a thought", 0), answered("still answered")}}
	run := runNotes(t, noteGraph(), noteView(), notes, model)

	if run.record.Answer != "still answered" || !run.record.Outcome.Produced {
		t.Errorf("the run answered %q (produced %v), want its answer despite the failed note", run.record.Answer, run.record.Outcome.Produced)
	}
	if !run.graph.writeRunCalled {
		t.Error("the run record was not filed after a failed note")
	}
	round := run.noteRound(t)
	if round.Written != NotStored || round.Created != 0 || round.Error != errNoteNotStored {
		t.Errorf("the note round = %+v, want notStored with no created id and the failure named", round)
	}
}

func TestAnUnlinkedNoteIsKeptAndReportedAsUnlinkedWithItsId(t *testing.T) {
	t.Parallel()

	notes := &fakeNotes{receipt: WriteReceipt{State: Unlinked, NodeID: 777}}
	run := runNotes(t, noteGraph(), noteView(), notes, &fakeModel{results: []JudgeResult{wantsNoteOf("a thought", 0), answeredFinal()}})

	round := run.noteRound(t)
	if round.Written != Unlinked || round.Created != 777 || round.Error != "" {
		t.Errorf("the note round = %+v, want unlinked with its id and no error: the node exists and is a repairable orphan", round)
	}
}

func TestTheSummaryNamesAStoredNoteWithItsStateIdAndTheFirstEightyRunesOfItsText(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, noteGraph(), wantsNoteOf(strings.Repeat("é", 100), 0), answeredFinal())

	summary := RenderSummary(run.record, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	want := `  1 note  stored  #900  "` + strings.Repeat("é", 79) + `…"`
	if !strings.Contains(summary, want) {
		t.Errorf("the summary does not carry the line %q:\n%s", want, summary)
	}

	refused := runNoteResults(t, noteGraph(), wantsNoteOf(" ", 0), answeredFinal())
	if line := `  1 note  refused  ""`; !strings.Contains(RenderSummary(refused.record, time.Time{}), line) {
		t.Errorf("the summary of a refused note does not carry %q", line)
	}
}
