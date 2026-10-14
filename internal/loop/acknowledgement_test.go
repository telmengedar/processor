package loop

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const (
	notStoredLine   = "Your note in this request was not stored. Now write your answer to the request."
	ackNoteText     = "the deploy finished at 17:00"
	ackRefusedName  = "processor-run check 1"
	ackInvalidUTF8  = "bad \xff text"
	ackToolErrorMsg = "tool arguments could not be parsed: unexpected token"
)

func storedLine(text string) string {
	return "You stored this note in this request: \"" + text + "\". Now write your answer to the request."
}

func runWithCap(t *testing.T, graph *fakeGraph, notes *fakeNotes, maxCalls int, results ...JudgeResult) noteRun {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	turn.Notes = notes
	settings := DefaultSettings()
	settings.MaxModelCalls = maxCalls
	record, _, err := turn.RunWithOptions(t.Context(), RunOptions{Input: "hello", Subject: noteSubject, Settings: settings, View: noteView()})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	return noteRun{record: record, model: model, notes: notes, graph: graph}
}

func (r noteRun) userContent(call int) string {
	return userContentOf(r.model.calls[call])
}

func TestTheAcknowledgementsAreTheTextsTheDesignMeasured(t *testing.T) {
	t.Parallel()

	if got, want := acknowledgement(NoteAckStored, "x y"), "You stored this note in this request: \"x y\". Now write your answer to the request."; got != want {
		t.Errorf("the stored acknowledgement = %q, want %q", got, want)
	}
	if got, want := acknowledgement(NoteAckNotStored, ackNoteText), "Your note in this request was not stored. Now write your answer to the request."; got != want {
		t.Errorf("the not-stored acknowledgement = %q, want %q", got, want)
	}
	if got := acknowledgement(NoteAckNone, ackNoteText); got != "" {
		t.Errorf("the acknowledgement of no note round = %q, want none", got)
	}
}

func TestTheClosingIsTheSearchParagraphThenTheAcknowledgementEachItsOwnParagraph(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ search, ack, want string }{
		{"search", "ack", "search\n\nack"},
		{"", "ack", "ack"},
		{"search", "", "search"},
		{"", "", ""},
	} {
		if got := composeClosing(tc.search, tc.ack); got != tc.want {
			t.Errorf("composeClosing(%q, %q) = %q, want %q", tc.search, tc.ack, got, tc.want)
		}
	}
}

func TestACallAfterAStoredNoteEndsWithTheStoredLineQuotingTheTextAfterTheClause(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, baseGraph(), wantsNoteOf(ackNoteText, 0), answeredFinal())

	want := recallClause + "\n\n" + storedLine(ackNoteText)
	if got := run.model.calls[1].Trailer; got != want {
		t.Errorf("the closing of the call after a stored note = %q, want %q", got, want)
	}
	if got := run.userContent(1); !strings.HasSuffix(got, "\n\n"+want) {
		t.Errorf("the user content of the call after a stored note does not end with the clause and then the acknowledgement:\n%s", got)
	}
	if got := run.model.calls[0].Trailer; strings.Contains(got, "You stored") {
		t.Errorf("the call before the note round carries %q, want no acknowledgement", got)
	}
}

func TestAnUnlinkedNoteIsAcknowledgedAsStored(t *testing.T) {
	t.Parallel()

	run := runNotes(t, baseGraph(), noteView(), &fakeNotes{receipt: WriteReceipt{State: Unlinked, NodeID: noteNodeID}}, &fakeModel{results: []JudgeResult{wantsNoteOf(ackNoteText, 0), answeredFinal()}})

	if got := run.model.calls[1].Trailer; !strings.HasSuffix(got, storedLine(ackNoteText)) {
		t.Errorf("the closing after an unlinked note = %q, want the stored line", got)
	}
	if got, want := run.record.NoteAcks, []NoteAck{NoteAckNone, NoteAckStored}; !slices.Equal(got, want) {
		t.Errorf("noteAcks = %v, want %v", got, want)
	}
}

func TestEveryCallAfterAStoredNoteCarriesTheLineWhateverTheSearchStateIs(t *testing.T) {
	t.Parallel()

	fruitful := runNoteResults(t, graphYieldingNewRowsToEveryRecall(), wantsNoteOf(ackNoteText, 0), recallFor("more"), answeredFinal())
	if got, want := fruitful.model.calls[2].Trailer, storedLine(ackNoteText); got != want {
		t.Errorf("the closing after a fruitful search following a note = %q, want the acknowledgement alone %q", got, want)
	}

	barren := runNoteResults(t, baseGraph(), wantsNoteOf(ackNoteText, 0), recallFor("more"), answeredFinal())
	got := barren.model.calls[2].Trailer
	if !strings.HasPrefix(got, searchedLine) || !strings.HasSuffix(got, barrenLine+"\n\n"+storedLine(ackNoteText)) {
		t.Errorf("the closing after a barren search following a note = %q, want the barren line and then the acknowledgement", got)
	}
}

func TestTheReservedAnsweringCallAfterANoteCarriesTheAcknowledgementAsItsOnlyClosingParagraph(t *testing.T) {
	t.Parallel()

	run := runWithCap(t, baseGraph(), &fakeNotes{}, 2, wantsNoteOf(ackNoteText, 0), answeredFinal())

	if len(run.model.calls) != 2 {
		t.Fatalf("the run made %d calls, want 2", len(run.model.calls))
	}
	if offered := run.offeredOn(1); len(offered) != 0 {
		t.Fatalf("the reserved call offered %v, want nothing", offered)
	}
	if got, want := run.model.calls[1].Trailer, storedLine(ackNoteText); got != want {
		t.Errorf("the closing of the reserved call = %q, want %q", got, want)
	}
	if got, want := run.record.NoteAcks, []NoteAck{NoteAckNone, NoteAckStored}; !slices.Equal(got, want) {
		t.Errorf("noteAcks = %v, want %v", got, want)
	}
}

func TestTheReservedCallAfterANoteThatWasNotStoredCarriesTheNotStoredLine(t *testing.T) {
	t.Parallel()

	run := runWithCap(t, baseGraph(), &fakeNotes{receipt: WriteReceipt{State: NotStored}}, 2, wantsNoteOf(ackNoteText, 0), answeredFinal())

	if got := run.model.calls[1].Trailer; got != notStoredLine {
		t.Errorf("the closing of the reserved call = %q, want %q", got, notStoredLine)
	}
	if got, want := run.record.NoteAcks, []NoteAck{NoteAckNone, NoteAckNotStored}; !slices.Equal(got, want) {
		t.Errorf("noteAcks = %v, want %v", got, want)
	}
}

func TestEveryWayANoteCanFailLeavesEveryLaterCallWithTheCauseFreeLineAndNeitherTheTextNorTheError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		notes  *fakeNotes
		result JudgeResult
		text   string
	}{
		{"an adapter tool error", &fakeNotes{}, JudgeResult{Reason: WantsNote, RawReason: "tool_calls", NoteText: ackNoteText, ToolError: ackToolErrorMsg}, ackNoteText},
		{"blank text", &fakeNotes{}, wantsNoteOf("   ", 0), "   "},
		{"invalid UTF-8", &fakeNotes{}, wantsNoteOf(ackInvalidUTF8, 0), ackInvalidUTF8},
		{"a run name", &fakeNotes{}, wantsNoteOf(ackRefusedName, 0), ackRefusedName},
		{"a receipt that says not stored", &fakeNotes{receipt: WriteReceipt{State: NotStored}}, wantsNoteOf(ackNoteText, 0), ackNoteText},
	} {
		run := runNotes(t, graphYieldingNewRowsToEveryRecall(), noteView(), tc.notes, &fakeModel{results: []JudgeResult{tc.result, recallFor("more"), answeredFinal()}})

		if len(run.model.calls) != 3 {
			t.Errorf("%s: the run made %d calls, want 3", tc.name, len(run.model.calls))
			continue
		}
		round := run.record.ToolCalls[0]
		if round.Error == "" {
			t.Errorf("%s: the note round carries no error, want the failure recorded", tc.name)
		}
		if got, want := run.record.NoteAcks, []NoteAck{NoteAckNone, NoteAckNotStored, NoteAckNotStored}; !slices.Equal(got, want) {
			t.Errorf("%s: noteAcks = %v, want %v", tc.name, got, want)
		}
		for later := 1; later < 3; later++ {
			if !strings.HasSuffix(run.model.calls[later].Trailer, notStoredLine) {
				t.Errorf("%s: the closing of call %d = %q, want it to end with %q", tc.name, later+1, run.model.calls[later].Trailer, notStoredLine)
			}
			if strings.Contains(run.model.calls[later].Trailer, "You stored") {
				t.Errorf("%s: call %d claims a stored note", tc.name, later+1)
			}
			for field, text := range fieldsOfACall(run.model.calls[later]) {
				if strings.TrimSpace(tc.text) != "" && strings.Contains(text, tc.text) {
					t.Errorf("%s: the %s of call %d carries the note's text %q", tc.name, field, later+1, tc.text)
				}
				if strings.Contains(text, round.Error) || strings.Contains(text, ackToolErrorMsg) {
					t.Errorf("%s: the %s of call %d carries the round's error text", tc.name, field, later+1)
				}
			}
		}
	}
}

func TestNoteAcksRecordWhatEachCallShowedNotWhatTheTurnEndedWith(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, graphYieldingNewRowsToEveryRecall(), recallFor("first"), wantsNoteOf(ackNoteText, 0), recallFor("second"), answeredFinal())

	if got, want := run.record.NoteAcks, []NoteAck{NoteAckNone, NoteAckNone, NoteAckStored, NoteAckStored}; !slices.Equal(got, want) {
		t.Errorf("noteAcks = %v, want %v", got, want)
	}
	for i, call := range run.model.calls {
		shown := strings.Contains(call.Trailer, "You stored")
		if shown != (run.record.NoteAcks[i] == NoteAckStored) {
			t.Errorf("call %d: acknowledgement shown = %v but noteAcks[%d] = %q", i+1, shown, i, run.record.NoteAcks[i])
		}
	}
}

func TestATurnWithoutANoteRoundRecordsNoneForEveryCallAndShowsNoAcknowledgement(t *testing.T) {
	t.Parallel()

	configured := runNoteResults(t, graphYieldingNewRowsToEveryRecall(), recallFor("first"), answeredFinal())
	plain, model := runRecalls(t, graphYieldingNewRowsToEveryRecall(), recallOf("first"), answer())

	if got, want := configured.record.NoteAcks, []NoteAck{NoteAckNone, NoteAckNone}; !slices.Equal(got, want) {
		t.Errorf("noteAcks of a configured turn without a note round = %v, want %v", got, want)
	}
	if got, want := plain.NoteAcks, []NoteAck{NoteAckNone, NoteAckNone}; !slices.Equal(got, want) {
		t.Errorf("noteAcks of an unconfigured turn = %v, want %v", got, want)
	}
	for i, call := range model.calls {
		if call.Trailer != configured.model.calls[i].Trailer {
			t.Errorf("call %d: the closing differs between a configured and an unconfigured turn: %q and %q", i+1, configured.model.calls[i].Trailer, call.Trailer)
		}
	}
}

func TestNoteAcksAreOneEntryPerCallUnderTheirKeyAndARecordWithoutThemStillDecodes(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, baseGraph(), wantsNoteOf(ackNoteText, 0), answeredFinal())
	raw, err := json.Marshal(run.record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got, want := string(keys["noteAcks"]), `["none","stored"]`; got != want {
		t.Errorf("the noteAcks key = %s, want %s", got, want)
	}
	if len(run.record.NoteAcks) != run.record.ModelCalls {
		t.Errorf("noteAcks has %d entries for %d model calls", len(run.record.NoteAcks), run.record.ModelCalls)
	}

	var archived Record
	if err := json.Unmarshal([]byte(`{"trailers":["clause"]}`), &archived); err != nil {
		t.Fatalf("a record without noteAcks does not decode: %v", err)
	}
	if archived.NoteAcks != nil {
		t.Errorf("a record without noteAcks decoded to %v, want none", archived.NoteAcks)
	}
}
