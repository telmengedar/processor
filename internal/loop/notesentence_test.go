package loop

import (
	"slices"
	"strings"
	"testing"
)

func TestTheNoteSentenceIsTheTextTheDesignMeasured(t *testing.T) {
	t.Parallel()

	const measured = "When you are to note, remember or write something down, call the write_note tool with it instead of writing that you did."
	if NoteSentence != measured {
		t.Fatalf("NoteSentence = %q, want the measured text %q", NoteSentence, measured)
	}
}

func TestTheClosingIsTheSearchParagraphThenTheAcknowledgementThenTheNoteSentenceEachItsOwnParagraph(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ search, ack, sentence, want string }{
		{"search", "ack", "sentence", "search\n\nack\n\nsentence"},
		{"", "ack", "sentence", "ack\n\nsentence"},
		{"search", "", "sentence", "search\n\nsentence"},
		{"", "", "sentence", "sentence"},
		{"search", "ack", "", "search\n\nack"},
		{"", "", "", ""},
	} {
		if got := composeClosing(tc.search, tc.ack, tc.sentence); got != tc.want {
			t.Errorf("composeClosing(%q, %q, %q) = %q, want %q", tc.search, tc.ack, tc.sentence, got, tc.want)
		}
	}
}

func TestTheNoteSentenceIsChosenByTheOfferedListAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		offered []string
		want    string
	}{
		{nil, ""},
		{[]string{}, ""},
		{allTools, ""},
		{append(slices.Clone(allTools), ToolNote), NoteSentence},
		{[]string{ToolNote}, NoteSentence},
	} {
		if got := noteSentenceFor(tc.offered); got != tc.want {
			t.Errorf("noteSentenceFor(%v) = %q, want %q", tc.offered, got, tc.want)
		}
	}
}

func assertSentenceMatchesOffer(t *testing.T, name string, run noteRun) {
	t.Helper()

	for i, call := range run.model.calls {
		content := userContentOf(call)
		shown := strings.Contains(content, NoteSentence)
		offered := offeredNote(call.Offered)
		if shown != offered {
			t.Errorf("%s: call %d offered %v but the note sentence shown = %v: it belongs on exactly the calls that offer the note tool", name, i+1, call.Offered, shown)
		}
		if offered && !strings.HasSuffix(content, "\n\n"+NoteSentence) {
			t.Errorf("%s: call %d does not end with the note sentence as its own paragraph:\n%q", name, i+1, content)
		}
		if strings.Count(content, NoteSentence) > 1 {
			t.Errorf("%s: call %d carries the note sentence %d times", name, i+1, strings.Count(content, NoteSentence))
		}
	}
}

func TestTheNoteSentenceIsOnEveryCallThatOffersTheNoteToolAndOnNoOther(t *testing.T) {
	t.Parallel()

	runs := map[string]noteRun{
		"answered at once":     runNoteResults(t, baseGraph(), answeredFinal()),
		"after a barren":       runNoteResults(t, baseGraph(), recallFor("more"), recallFor("again"), answeredFinal()),
		"after a fruitful":     runNoteResults(t, graphYieldingNewRowsToEveryRecall(), recallFor("more"), answeredFinal()),
		"after a note":         runNoteResults(t, baseGraph(), wantsNoteOf(ackNoteText, 0), answeredFinal()),
		"after a refused note": runNoteResults(t, baseGraph(), wantsNoteOf("   ", 0), answeredFinal()),
		"to the cap":           runNoteResults(t, graphYieldingNewRowsToEveryRecall(), researchToTheCap()...),
		"a note on the cap":    runWithCap(t, baseGraph(), &fakeNotes{}, 2, wantsNoteOf(ackNoteText, 0), answeredFinal()),
	}
	for name, run := range runs {
		assertSentenceMatchesOffer(t, name, run)
	}

	capped := runs["to the cap"]
	if last := capped.model.calls[len(capped.model.calls)-1]; len(last.Offered) != 0 || strings.Contains(userContentOf(last), NoteSentence) {
		t.Errorf("the reserved call offered %v and carries the note sentence = %v, want neither", last.Offered, strings.Contains(userContentOf(last), NoteSentence))
	}
	if len(capped.model.calls) < 2 || !strings.Contains(userContentOf(capped.model.calls[0]), NoteSentence) {
		t.Errorf("the first call of a capped turn does not carry the note sentence")
	}
}

func TestTheNoteSentenceFollowsTheSearchStateAsItsOwnLastParagraph(t *testing.T) {
	t.Parallel()

	clause := runNoteResults(t, baseGraph(), answeredFinal())
	if got, want := clause.model.calls[0].Trailer, recallClause+"\n\n"+NoteSentence; got != want {
		t.Errorf("the closing with no search = %q, want the clause and then the note sentence %q", got, want)
	}

	found := runNoteResults(t, graphYieldingNewRowsToEveryRecall(), recallFor("more"), answeredFinal())
	if got := found.model.calls[1].Trailer; got != NoteSentence {
		t.Errorf("the closing after a fruitful search = %q, want the note sentence alone", got)
	}

	barren := runNoteResults(t, baseGraph(), recallFor("more"), answeredFinal())
	got := barren.model.calls[1].Trailer
	if !strings.HasPrefix(got, searchedLine) || !strings.HasSuffix(got, barrenLine+"\n\n"+NoteSentence) {
		t.Errorf("the closing after a barren search = %q, want the barren paragraph and then the note sentence", got)
	}
}

func TestTheNoteSentenceTrailsTheAcknowledgementWhenBothAreSupplied(t *testing.T) {
	t.Parallel()

	got := composeClosing(recallClause, acknowledgement(NoteAckStored, ackNoteText), NoteSentence)
	if want := recallClause + "\n\n" + storedLine(ackNoteText) + "\n\n" + NoteSentence; got != want {
		t.Errorf("the closing with every part = %q, want %q", got, want)
	}
}

func TestNoCallAfterANoteRoundCarriesTheNoteSentence(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, graphYieldingNewRowsToEveryRecall(), wantsNoteOf(ackNoteText, 0), recallFor("more"), answeredFinal())

	if len(run.model.calls) != 3 {
		t.Fatalf("the run made %d calls, want 3", len(run.model.calls))
	}
	for i := 1; i < len(run.model.calls); i++ {
		if strings.Contains(run.model.calls[i].Trailer, NoteSentence) {
			t.Errorf("call %d, after the note round, carries the note sentence: %q", i+1, run.model.calls[i].Trailer)
		}
	}
}

func TestATurnThatCannotFileANoteShowsNoNoteSentence(t *testing.T) {
	t.Parallel()

	withoutConfig, model := runRecalls(t, baseGraph(), recallOf("more"), answer())
	if withoutConfig.ModelCalls != 2 {
		t.Fatalf("the run made %d calls, want 2", withoutConfig.ModelCalls)
	}
	for i, call := range model.calls {
		if strings.Contains(userContentOf(call), NoteSentence) {
			t.Errorf("call %d of a turn without a note config carries the note sentence", i+1)
		}
	}

	withoutPort := runNotes(t, baseGraph(), noteView(), nil, &fakeModel{results: []JudgeResult{answeredFinal()}})
	if strings.Contains(userContentOf(withoutPort.model.calls[0]), NoteSentence) {
		t.Errorf("a turn whose config enables notes but holds no port carries the note sentence")
	}
}

func TestACallThatOffersTheNoteToolAlwaysOffersRecallToo(t *testing.T) {
	t.Parallel()

	for _, files := range []bool{false, true} {
		turn := NewTurn(baseGraph(), &fakeModel{}, nil, fixedSystem("system"), "test-model", testLogger())
		turn.Notes = &fakeNotes{}
		if files {
			turn.Files = &fakeFiles{dir: "/runs/1"}
		}
		for _, reserved := range []bool{false, true} {
			for _, noteOpen := range []bool{false, true} {
				offered := turn.offeredTools(reserved, noteOpen)
				if offersNote(offered) && !slices.Contains(offered, ToolRecall) {
					t.Errorf("files %v reserved %v noteOpen %v: offered %v carries the note tool without recall, which would make the note sentence the only closing", files, reserved, noteOpen, offered)
				}
			}
		}
	}
}
