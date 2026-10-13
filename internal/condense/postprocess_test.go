package condense

import (
	"strings"
	"testing"
)

func TestAnOutermostFenceIsStrippedOnlyWhenBothItsHalvesArePresent(t *testing.T) {
	if got := Postprocess("```\nthe body\n```"); got != "the body" {
		t.Fatalf("want the fenced body alone, got %q", got)
	}
	if got := Postprocess("```markdown\nthe body\n```"); got != "the body" {
		t.Fatalf("want a labelled fence stripped too, got %q", got)
	}
	if got := Postprocess("```\nthe body"); got != "```\nthe body" {
		t.Fatalf("an unbalanced fence must be left alone, got %q", got)
	}
}

func TestAnInnerFenceSurvivesBecauseOnlyTheOutermostPairIsStripped(t *testing.T) {
	raw := "```\nintro\n```\ngo test ./...\n```\noutro\n```"

	got := Postprocess(raw)

	if !strings.Contains(got, "go test ./...") {
		t.Fatalf("want the fenced code sample preserved, got %q", got)
	}
}

func TestEmphasisPairsAreRemovedWhileTheTextBetweenThemSurvives(t *testing.T) {
	if got := Postprocess("a **bold** claim"); got != "a bold claim" {
		t.Fatalf("want the emphasis markers gone and the word kept, got %q", got)
	}
	if got := Postprocess("an unpaired ** marker"); got != "an unpaired ** marker" {
		t.Fatalf("an unpaired marker must be left alone, got %q", got)
	}
}

func TestEmphasisStrippingCorruptsAnExponentPairOnOneLineWhichIsTheKnownLimitOfTheRule(t *testing.T) {
	got := Postprocess("2**8 and 3**9")

	if got != "28 and 39" {
		t.Fatalf("this test pins the known corruption of the emphasis rule; behaviour changed to %q", got)
	}
}

func TestAnExponentWithNoClosingPairOnTheSameLineIsLeftAlone(t *testing.T) {
	if got := Postprocess("the value is 2**8"); got != "the value is 2**8" {
		t.Fatalf("a lone exponent must survive, got %q", got)
	}
}

func TestAnEmptyCondensationIsRejected(t *testing.T) {
	if got := Defect("a body", ""); got != skipEmptyCondensation {
		t.Fatalf("want %q, got %q", skipEmptyCondensation, got)
	}
}

func TestDefectAcceptsACondensationNoShorterThanItsContent(t *testing.T) {
	if got := Defect(strings.Repeat("a", 10), strings.Repeat("b", 1000)); got != "" {
		t.Fatalf("want no defect for a long condensation, got %q", got)
	}
}

func TestDefectAcceptsACondensationBelowFivePercent(t *testing.T) {
	if got := Defect(strings.Repeat("a", 1000), "b"); got != "" {
		t.Fatalf("want no defect for a one-byte condensation, got %q", got)
	}
}

func TestAPreambleIsDetectedOnTheFirstNonBlankLineAndNeverRewritten(t *testing.T) {

	for _, opening := range []string{"Here is the condensed document:", "below is the extract", "Sure, here you go", "Okay.", "This is the result", "I have condensed it"} {
		substance := opening + "\n" + strings.Repeat("b", 200)
		if got := Defect("a body", substance); got != skipPreamble {
			t.Fatalf("want %q for opening %q, got %q", skipPreamble, opening, got)
		}
	}
}

func TestThePreambleRuleMatchesAWordPrefixSoALineOpeningWithHeredityIsRejectedToo(t *testing.T) {
	substance := "Heredity is not discussed.\n" + strings.Repeat("b", 200)

	if got := Defect("a body", substance); got != skipPreamble {
		t.Fatalf("this test pins that the preamble rule matches on a word prefix, not a whole word; got %q", got)
	}
}

func TestACleanCondensationPassesEveryStorageRule(t *testing.T) {
	if got := Defect("a body", strings.Repeat("b", 200)); got != "" {
		t.Fatalf("want no defect, got %q", got)
	}
}

func rulingSections() []string {
	var lines []string
	for _, title := range []string{"a pull request body answers why", "end of arc starts at drafts", "review ledger query", "dispatch is a queue"} {
		lines = append(lines, "## RULING "+title, "The statement of "+title+".", "The reason given for "+title+".")
	}
	return lines
}

func loopedSubstance(rounds int, marker string) string {
	lines := []string{"# Operating contract", "A summary of the contract."}
	for i := 0; i < rounds; i++ {
		block := rulingSections()
		block[1] += " " + marker
		lines = append(lines, block...)
	}
	return strings.Join(lines, "\n")
}

func contractBody() string {
	lines := []string{"# Operating contract", "The full contract.", "## Intro", "How to read it.", "Read it once."}
	lines = append(lines, rulingSections()...)
	lines = append(lines, "## Later rulings", "Something the loop never reaches.", "Neither does this.")
	return strings.Join(lines, "\n")
}

func TestDefectRejectsAThreeLinePassageTheCondensationRepeatsMoreOftenThanItsBody(t *testing.T) {
	body := "intro\nthe first line\nthe second line\nthe third line\noutro"
	substance := "intro\nthe first line\nthe second line\nthe third line\nmiddle\nthe first line\nthe second line\nthe third line\nbetween\nthe first line\nthe second line\nthe third line"

	if got := Defect(body, substance); got != skipRepeatedPassage {
		t.Fatalf("want %q for a block repeated three times against once in the body, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectRejectsFourRulingsRepeatedEightTimesWhenTheBodyStatesThemOnce(t *testing.T) {
	if got := Defect(contractBody(), loopedSubstance(8, "")); got != skipRepeatedPassage {
		t.Fatalf("want %q for four rulings repeated eight times, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectAcceptsTheSameDocumentCondensedWithoutALoop(t *testing.T) {
	if got := Defect(contractBody(), loopedSubstance(1, "")); got != "" {
		t.Fatalf("want no defect for the rulings stated once, got %q", got)
	}
}

func TestDefectAcceptsAPassageRepeatedNoMoreOftenThanItsBodyRepeatsIt(t *testing.T) {
	block := "| ---- | ---- |\n| row | row |\n| end | end |"
	body := "first table\n" + block + "\nsecond table\n" + block
	substance := "first table\n" + block + "\nsecond table\n" + block

	if got := Defect(body, substance); got != "" {
		t.Fatalf("want no defect for a block the body repeats as often, got %q", got)
	}
}

func TestDefectAcceptsATwoLinePassageRepeatedMoreOftenThanItsBody(t *testing.T) {
	body := "recall result: none\nthe node was not found\nthe end"
	substance := "call 1\nrecall result: none\nthe node was not found\ncall 2\nrecall result: none\nthe node was not found\ncall 3\nrecall result: none\nthe node was not found"

	if got := Defect(body, substance); got != "" {
		t.Fatalf("want no defect for a pair repeated across calls, got %q", got)
	}
}

func TestDefectRejectsOneLineRepeatedFourTimesInARow(t *testing.T) {
	body := "before\nthe same line\nafter"
	substance := "before\nthe same line\nthe same line\nthe same line\nthe same line\nafter"

	if got := Defect(body, substance); got != skipRepeatedPassage {
		t.Fatalf("want %q for a line repeated four times in a row, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectComparesPassagesIgnoringBlankLinesIndentationAndCarriageReturns(t *testing.T) {
	cases := []struct {
		name  string
		plain string
		body  string
	}{
		{"carriage returns", "alphaline\nbeta line\ngamma line", "x\r\nalpha\rline\r\nbeta line\r\ngamma line\r\ny\r\nalpha\rline\r\nbeta line\r\ngamma line\r\n"},
		{"indentation and spacing", "alpha line\nbeta line\ngamma line", "x\n    alpha   line\n\tbeta  line  \n  gamma line\ny\n    alpha   line\n\tbeta  line  \n  gamma line\n"},
		{"blank lines", "alpha line\nbeta line\ngamma line", "x\nalpha line\n\nbeta line\n\n\ngamma line\ny\nalpha line\n\nbeta line\n\n\ngamma line\n"},
	}

	for _, c := range cases {
		substance := "x\n" + c.plain + "\ny\n" + c.plain
		if got := Defect(c.body, substance); got != "" {
			t.Fatalf("%s: want no defect when the body repeats the block as often, got %q", c.name, got)
		}
	}
}

func TestDefectComparesPassagesWithTheBodysEmphasisStripped(t *testing.T) {
	body := "x\n**Why:** the reason\nthe next line\nthe last line\ny\n**Why:** the reason\nthe next line\nthe last line"
	substance := "x\nWhy: the reason\nthe next line\nthe last line\ny\nWhy: the reason\nthe next line\nthe last line"

	if got := Defect(body, substance); got != "" {
		t.Fatalf("want no defect when the body repeats the block under emphasis, got %q", got)
	}
}
