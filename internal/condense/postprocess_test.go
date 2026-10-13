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
	if got := Defect(""); got != skipEmptyCondensation {
		t.Fatalf("want %q, got %q", skipEmptyCondensation, got)
	}
}

func TestDefectAcceptsACondensationNoShorterThanItsContent(t *testing.T) {
	if got := Defect(strings.Repeat("b", 1000)); got != "" {
		t.Fatalf("want no defect for a long condensation, got %q", got)
	}
}

func TestDefectAcceptsACondensationBelowFivePercent(t *testing.T) {
	if got := Defect("b"); got != "" {
		t.Fatalf("want no defect for a one-byte condensation, got %q", got)
	}
}

func TestAPreambleIsDetectedOnTheFirstNonBlankLineAndNeverRewritten(t *testing.T) {

	for _, opening := range []string{"Here is the condensed document:", "below is the extract", "Sure, here you go", "Okay.", "This is the result", "I have condensed it"} {
		substance := opening + "\n" + strings.Repeat("b", 200)
		if got := Defect(substance); got != skipPreamble {
			t.Fatalf("want %q for opening %q, got %q", skipPreamble, opening, got)
		}
	}
}

func TestThePreambleRuleMatchesAWordPrefixSoALineOpeningWithHeredityIsRejectedToo(t *testing.T) {
	substance := "Heredity is not discussed.\n" + strings.Repeat("b", 200)

	if got := Defect(substance); got != skipPreamble {
		t.Fatalf("this test pins that the preamble rule matches on a word prefix, not a whole word; got %q", got)
	}
}

func TestACleanCondensationPassesEveryStorageRule(t *testing.T) {
	if got := Defect(strings.Repeat("b", 200)); got != "" {
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

func TestDefectRejectsABlockTheCondensationStatesThreeTimes(t *testing.T) {
	substance := "intro\nthe first line\nthe second line\nthe third line\nmiddle\nthe first line\nthe second line\nthe third line\nbetween\nthe first line\nthe second line\nthe third line"

	if got := Defect(substance); got != skipRepeatedPassage {
		t.Fatalf("want %q for a block stated three times, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectRejectsFourRulingsRepeatedEightTimes(t *testing.T) {
	if got := Defect(loopedSubstance(8, "")); got != skipRepeatedPassage {
		t.Fatalf("want %q for four rulings repeated eight times, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectAcceptsTheSameDocumentCondensedWithoutALoop(t *testing.T) {
	if got := Defect(loopedSubstance(1, "")); got != "" {
		t.Fatalf("want no defect for the rulings stated once, got %q", got)
	}
}

func TestDefectRejectsATwoLinePassageStatedTwice(t *testing.T) {
	if got := Defect("x\nA\nB\ny\nA\nB\nz"); got != skipRepeatedPassage {
		t.Fatalf("want %q for a pair stated twice apart, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectAcceptsASingleLineThatRecursUnderDifferentHeadings(t *testing.T) {
	substance := "### 7.1 First\n- Verdict: reaffirm rejection.\n### 7.2 Second\n- Verdict: reaffirm rejection."

	if got := Defect(substance); got != "" {
		t.Fatalf("want no defect for one line recurring under different headings, got %q", got)
	}
}

func TestDefectRejectsOneLineRepeatedFourTimesInARow(t *testing.T) {
	substance := "before\nthe same line\nthe same line\nthe same line\nthe same line\nafter"

	if got := Defect(substance); got != skipRepeatedPassage {
		t.Fatalf("want %q for a line repeated four times in a row, got %q", skipRepeatedPassage, got)
	}
}

func TestDefectIgnoresTableSeparatorsAndCodeFences(t *testing.T) {
	cases := []struct {
		name      string
		substance string
	}{
		{"tables", "| Name | Value |\n| ---- | ---- |\n| alpha | 1 |\nThe second table follows.\n| Name | Value |\n| ---- | ---- |\n| beta | 2 |"},
		{"fences", "## Example\n```go\nfirst()\n```\nSome prose.\n## Example\n```go\nsecond()\n```"},
	}

	for _, c := range cases {
		if got := Defect(c.substance); got != "" {
			t.Fatalf("%s: want no defect when only markup lines recur, got %q", c.name, got)
		}
	}
}

func TestDefectFindsARepeatThroughBlankLinesIndentationAndCarriageReturns(t *testing.T) {
	cases := []struct {
		name   string
		first  string
		second string
	}{
		{"carriage returns", "alphaline\nbeta line", "alpha\rline\r\nbeta line"},
		{"indentation and spacing", "alpha line\nbeta line", "    alpha   line\n\tbeta  line  "},
		{"blank lines", "alpha line\nbeta line", "alpha line\n\n\nbeta line"},
	}

	for _, c := range cases {
		substance := "x\n" + c.first + "\ny\n" + c.second
		if got := Defect(substance); got != skipRepeatedPassage {
			t.Fatalf("%s: want %q for the same pair written differently, got %q", c.name, skipRepeatedPassage, got)
		}
	}
}

func TestDefectCountsLinesOfDigitsAndOfNonLatinLettersAsContent(t *testing.T) {
	cases := []struct {
		name      string
		substance string
	}{
		{"digits", "x\n1 2 3\n4 5 6\ny\n1 2 3\n4 5 6"},
		{"non-latin letters", "x\nпервая строка\nвторая строка\ny\nпервая строка\nвторая строка"},
	}

	for _, c := range cases {
		if got := Defect(c.substance); got != skipRepeatedPassage {
			t.Fatalf("%s: want %q for a pair of content lines stated twice, got %q", c.name, skipRepeatedPassage, got)
		}
	}
}
