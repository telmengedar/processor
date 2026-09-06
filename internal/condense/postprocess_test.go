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

func TestAnEmptyCondensationIsRejectedBeforeAnyLengthRuleIsConsulted(t *testing.T) {
	if got := Defect(strings.Repeat("a", 1000), ""); got != skipEmptyCondensation {
		t.Fatalf("want %q, got %q", skipEmptyCondensation, got)
	}
}

func TestTheNotShorterRuleIsCheckedBeforeTheFloorSoAnOversizeCondensationReportsTheRightReason(t *testing.T) {
	if got := Defect(strings.Repeat("a", 10), strings.Repeat("b", 1000)); got != skipNotShorter {
		t.Fatalf("want %q, got %q", skipNotShorter, got)
	}
}

func TestAPreambleIsDetectedOnTheFirstNonBlankLineAndNeverRewritten(t *testing.T) {
	content := strings.Repeat("a", 1000)

	for _, opening := range []string{"Here is the condensed document:", "below is the extract", "Sure, here you go", "Okay.", "This is the result", "I have condensed it"} {
		substance := opening + "\n" + strings.Repeat("b", 200)
		if got := Defect(content, substance); got != skipPreamble {
			t.Fatalf("want %q for opening %q, got %q", skipPreamble, opening, got)
		}
	}
}

func TestThePreambleRuleMatchesAWordPrefixSoALineOpeningWithHeredityIsRejectedToo(t *testing.T) {
	content := strings.Repeat("a", 1000)
	substance := "Heredity is not discussed.\n" + strings.Repeat("b", 200)

	if got := Defect(content, substance); got != skipPreamble {
		t.Fatalf("this test pins that the preamble rule matches on a word prefix, not a whole word; got %q", got)
	}
}

func TestACleanCondensationInsideBothBoundsPassesEveryStorageRule(t *testing.T) {
	if got := Defect(strings.Repeat("a", 1000), strings.Repeat("b", 200)); got != "" {
		t.Fatalf("want no defect, got %q", got)
	}
}
