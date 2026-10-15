package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	placementInput         = "alice: is it still raining there"
	placementHistoryBanner = "===== EARLIER IN THIS CONVERSATION ====="
	placementFinalInput    = "===== INPUT =====\n" + placementInput
)

var placementInstant = time.Date(2026, 10, 15, 19, 21, 12, 0, time.UTC)

func placementFile(t *testing.T, name string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func renderPlacement(t *testing.T, withHistory bool, clause, trailer string) string {
	t.Helper()

	conversation := ""
	if withHistory {
		conversation = placementFile(t, "syn.conversation.txt")
	}
	return RenderUserContent(placementFile(t, "syn.block.txt"), placementInput, placementInstant, UpdateWindow{}, conversation, clause, trailer)
}

func TestTheClauseIsRenderedByteForByteAsTheMeasuredArmWithHistory(t *testing.T) {
	t.Parallel()

	got := renderPlacement(t, true, recallClause, "")
	if want := placementFile(t, "arm_syn_Q.user.txt"); got != want {
		t.Fatalf("user content differs from the measured arm:\n%q\nwant\n%q", got, want)
	}
}

func TestTheClauseIsRenderedByteForByteAsTheMeasuredArmWithoutHistory(t *testing.T) {
	t.Parallel()

	got := renderPlacement(t, false, recallClause, "")
	if want := placementFile(t, "arm_syn_Q_nohistory.user.txt"); got != want {
		t.Fatalf("user content differs from the measured arm:\n%q\nwant\n%q", got, want)
	}
}

func TestTheClauseSitsAheadOfTheHistorySectionAndNothingFollowsTheFinalRequest(t *testing.T) {
	t.Parallel()

	got := renderPlacement(t, true, recallClause, "")

	if strings.Index(got, recallClause) > strings.Index(got, placementHistoryBanner) {
		t.Errorf("the clause follows the history section:\n%q", got)
	}
	if !strings.HasSuffix(got, placementFinalInput) {
		t.Errorf("the user content does not end with the final request:\n%q", got)
	}
	if strings.Count(got, recallClause) != 1 {
		t.Errorf("the clause appears %d times, want once", strings.Count(got, recallClause))
	}
}

func TestTheClauseSitsAheadOfTheFinalRequestWhenThereIsNoHistory(t *testing.T) {
	t.Parallel()

	got := renderPlacement(t, false, recallClause, "")

	if !strings.HasSuffix(got, recallClause+"\n\n"+placementFinalInput) {
		t.Errorf("the user content does not end with the clause, a blank line and the final request:\n%q", got)
	}
}

func TestATurnWithoutTheClauseRendersAsTheShippedLayout(t *testing.T) {
	t.Parallel()

	for _, withHistory := range []bool{true, false} {
		withoutClause := renderPlacement(t, withHistory, "", "")
		name := "arm_syn_B_nohistory.user.txt"
		if withHistory {
			name = "arm_syn_B.user.txt"
		}
		shipped := strings.TrimSuffix(placementFile(t, name), "\n\n"+recallClause)

		if withoutClause != shipped {
			t.Errorf("history=%v: user content without the clause =\n%q\nwant the shipped layout without its closing paragraph\n%q", withHistory, withoutClause, shipped)
		}
	}
}

func TestTheClosingLinesStayAfterTheFinalRequestWhateverTheClause(t *testing.T) {
	t.Parallel()

	const closing = searchedLine + `"x"` + barrenLine

	for _, withHistory := range []bool{true, false} {
		for _, clause := range []string{"", recallClause} {
			got := renderPlacement(t, withHistory, clause, closing)
			if !strings.HasSuffix(got, placementFinalInput+"\n\n"+closing) {
				t.Errorf("history=%v clause=%q: the closing lines do not follow the final request:\n%q", withHistory, clause, got)
			}
			if present := strings.Contains(got, recallClause); present != (clause != "") {
				t.Errorf("history=%v clause=%q: the clause is present in the user content = %v", withHistory, clause, present)
			}
		}
	}
}

func TestOnlyTheClauseMovesWhenARunCarriesBothTheClauseAndANoteAcknowledgement(t *testing.T) {
	t.Parallel()

	run := runNoteResults(t, baseGraph(), wantsNoteOf(ackNoteText, 0), answeredFinal())

	call := run.model.calls[1]
	if call.Clause != recallClause || call.Trailer != storedLine(ackNoteText) {
		t.Fatalf("clause=%q trailer=%q, want the clause alone in Clause and the acknowledgement alone in Trailer", call.Clause, call.Trailer)
	}
	if strings.Contains(call.Trailer, recallClause) {
		t.Errorf("the trailer carries the clause: %q", call.Trailer)
	}
}

func TestTheCallAfterABarrenSearchCarriesNoClause(t *testing.T) {
	t.Parallel()

	_, model := runRecalls(t, baseGraph(), recallOf("anything"), answer())

	if got := model.calls[1].Clause; got != "" {
		t.Errorf("the call after a barren search carries clause %q, want none", got)
	}
	if !strings.HasPrefix(model.calls[1].Trailer, searchedLine) {
		t.Errorf("the barren line left the trailer: %q", model.calls[1].Trailer)
	}
}
