package loop

import (
	"strings"
	"testing"
)

const (
	yieldHashA = "aaaa"
	yieldHashB = "bbbb"
)

func shownRow(id int64, hash string) Disposition {
	return Disposition{ID: id, ContentHash: hash, Included: true}
}

func cutRow(id int64, hash string) Disposition {
	return Disposition{ID: id, ContentHash: hash, CutReason: cutReasonBelowFloor}
}

func TestAYieldAccountCountsARowTheBlockAlreadyCarriesAsNothingTheRoundAdded(t *testing.T) {
	t.Parallel()

	account := newYieldAccount([]Disposition{shownRow(10, yieldHashA), shownRow(20, yieldHashA)})

	if yield := account.round([]Disposition{shownRow(10, yieldHashA), shownRow(20, yieldHashA)}); yield != 0 {
		t.Fatalf("a round returning only rows the block already carried reported a yield of %d, want 0: the loop would pay for a round that showed the model nothing", yield)
	}
	if yield := account.round([]Disposition{shownRow(10, yieldHashA), shownRow(30, yieldHashA)}); yield != 1 {
		t.Fatalf("a round carrying one row nobody had seen reported a yield of %d, want 1", yield)
	}
}

func TestAYieldAccountCountsOnlyTheRowsAdmissionPutInFrontOfTheModel(t *testing.T) {
	t.Parallel()

	account := newYieldAccount(nil)

	if yield := account.round([]Disposition{cutRow(10, yieldHashA), cutRow(20, yieldHashA)}); yield != 0 {
		t.Fatalf("a round whose every row was cut reported a yield of %d, want 0: a cut row never reached the model and cannot be something the round added", yield)
	}
	if yield := account.round([]Disposition{cutRow(10, yieldHashA), shownRow(20, yieldHashA)}); yield != 1 {
		t.Fatalf("a round that admitted one of its two rows reported a yield of %d, want 1", yield)
	}
	if yield := account.round([]Disposition{shownRow(10, yieldHashA)}); yield != 1 {
		t.Fatalf("a row cut in an earlier round and admitted in this one reported a yield of %d, want 1: the model is seeing it for the first time", yield)
	}
}

func TestAYieldAccountCountsARowWhoseContentChangedSinceItWasShownAsNew(t *testing.T) {
	t.Parallel()

	account := newYieldAccount([]Disposition{shownRow(10, yieldHashA)})

	if yield := account.round([]Disposition{shownRow(10, yieldHashB)}); yield != 1 {
		t.Fatalf("the same id carrying different content reported a yield of %d, want 1: the account errs toward granting the round, never toward refusing fresh content", yield)
	}
	if yield := account.round([]Disposition{shownRow(10, yieldHashB)}); yield != 0 {
		t.Fatalf("that same changed row, returned again unchanged, reported a yield of %d, want 0", yield)
	}
}

func TestRecallClosesOnTheSecondConsecutiveBarrenRoundAndNotOnTheFirst(t *testing.T) {
	t.Parallel()

	account := newYieldAccount([]Disposition{shownRow(10, yieldHashA)})

	if account.closed() {
		t.Fatal("recall was closed before any round ran at all")
	}
	account.round([]Disposition{shownRow(10, yieldHashA)})
	if account.closed() {
		t.Fatal("recall closed after one barren round; the bound is two, so a single unlucky query ends the turn's recall")
	}
	account.round([]Disposition{shownRow(10, yieldHashA)})
	if !account.closed() {
		t.Fatal("recall stayed open after two consecutive barren rounds, so the loop keeps buying rounds that cannot add anything")
	}
}

func TestARoundThatAddsSomethingResetsTheBarrenRunSoRecallStaysOpen(t *testing.T) {
	t.Parallel()

	account := newYieldAccount(nil)

	account.round([]Disposition{})
	account.round([]Disposition{shownRow(10, yieldHashA)})
	account.round([]Disposition{shownRow(10, yieldHashA)})

	if account.closed() {
		t.Fatal("recall closed on two barren rounds with a productive round between them; the bound is two consecutive barren rounds, and a run still finding new rows must keep them")
	}

	account.round([]Disposition{shownRow(10, yieldHashA)})
	if !account.closed() {
		t.Fatal("recall stayed open once the two barren rounds were consecutive")
	}
}

func TestARoundThatAdmittedOnlyRowsAlreadyShownSaysSoInsteadOfPresentingThemAsResults(t *testing.T) {
	t.Parallel()

	exchange := ToolExchange{
		Tool:       ToolRecall,
		Query:      "the query",
		Results:    []Candidate{{ID: 10, Type: "documentation", Name: "Doc", Content: "zebrafish"}, {ID: 20, Type: "documentation", Name: "Other", Content: "zebrafish"}},
		NothingNew: true,
	}

	rendered := RenderToolResult(exchange)

	if strings.Contains(rendered, sectionResult) {
		t.Fatalf("a round whose every row the model had already been shown still rendered them as results:\n%s", rendered)
	}
	if strings.Contains(rendered, "zebrafish") {
		t.Fatalf("the rendering repeated the row content the model already holds:\n%s", rendered)
	}
	if !strings.Contains(rendered, "2 results") {
		t.Fatalf("the rendering does not name how many rows it is declining to present again:\n%s", rendered)
	}
}

func TestARoundThatBroughtSomethingNewStillRendersItsRows(t *testing.T) {
	t.Parallel()

	exchange := ToolExchange{
		Tool:    ToolRecall,
		Query:   "the query",
		Results: []Candidate{{ID: 10, Type: "documentation", Name: "Doc", Content: "zebrafish"}},
		Yield:   1,
	}

	rendered := RenderToolResult(exchange)

	if !strings.Contains(rendered, sectionResult) || !strings.Contains(rendered, "zebrafish") {
		t.Fatalf("a round that brought a row the model had not seen did not present it:\n%s", rendered)
	}
}

func TestTheRefusedRecallRoundTellsTheModelRecallIsClosedRatherThanReturningNothing(t *testing.T) {
	t.Parallel()

	rendered := RenderToolResult(closedRecallExchange(JudgeResult{Reason: WantsRecall, RecallQuery: "the query"}))

	if !strings.Contains(rendered, "recall is closed") {
		t.Fatalf("the refusal does not tell the model recall is closed; it was:\n%s", rendered)
	}
	if strings.Contains(rendered, "no additional results found") {
		t.Fatalf("the refusal reads as an empty graph rather than as a bound the loop imposed:\n%s", rendered)
	}
}
