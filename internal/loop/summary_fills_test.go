package loop

import (
	"strings"
	"testing"
)

func TestRenderSummaryStatesTheThreeFillLimitsTheRunWasBoundBy(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	const want = "(ceiling 2, floor 8000 B, max 100.0 kB)"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q: three of the record's eight limits are in the struct and absent from the account it renders.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryNamesTheModelBehindEveryFillItReports(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	if !strings.Contains(summary, "#15 written by gemma-3-12b-it") {
		t.Fatalf("the summary does not name the model that produced the substance this run generated; the record names the model behind the answer and owes the same account of its own composition.\nsummary:\n%s", summary)
	}
}

func TestRenderSummarySaysModelNotRecordedRatherThanNamingAnEmptyOne(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{{ID: 15, Filled: true}}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "#15 written by "+summaryNoFillModel) {
		t.Fatalf("a fill that recorded no model renders as though it named one.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryCountsFilledAgainstRefusedByReadingTheOutcomesItWasGiven(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{
		{ID: 11, Filled: true, Model: "gemma-3-12b-it"},
		{ID: 12, Reason: "below size floor"},
		{ID: 13, Reason: "below size floor"},
		{ID: 14, Reason: "per-turn ceiling reached"},
	}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "FILLS  1 filled / 3 refused") {
		t.Fatalf("the fills line does not count one filled against three refused.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryGroupsEachRefusedFillUnderTheReasonThatOutcomeRecords(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{
		{ID: 12, Reason: "below size floor"},
		{ID: 14, Reason: "per-turn ceiling reached"},
		{ID: 13, Reason: "below size floor"},
	}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "  below size floor (2): #12 #13\n") {
		t.Fatalf("the two rows refused for the same reason are not grouped under it.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "  per-turn ceiling reached (1): #14\n") {
		t.Fatalf("the ceiling refusal is not reported under its own reason.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryReportsAFillSectionEvenWhenNoCandidateLackedASubstance(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = nil

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "FILLS  0 filled / 0 refused") {
		t.Fatalf("a run that fired no fill renders no fills line at all, so a reader cannot tell it from a run whose fills were never recorded.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryBoundsALongFillRefusalRatherThanRenderingItWhole(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{{ID: 15, Reason: strings.Repeat("y", CarriedCauseRunes)}}

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, strings.Repeat("y", summaryFillRunes+1)) {
		t.Fatalf("the summary renders a fill refusal past its own %d-rune width.\nsummary:\n%s", summaryFillRunes, summary)
	}
	if !strings.Contains(summary, strings.Repeat("y", summaryFillRunes-1)+"…") {
		t.Fatalf("the summary does not mark the fill refusal it truncated.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryWrapsALongRefusedFillGroupsIdsAcrossLinesNoneOverNinetySixRunes(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = nil
	for i := range record.Limits.CandidateLimit {
		record.Fills = append(record.Fills, FillOutcome{ID: int64(100000 + i), Reason: "below size floor"})
	}

	summary := RenderSummary(record, summaryInstant())

	for _, line := range strings.Split(summary, "\n") {
		if length := len([]rune(line)); length > summaryLineRunes {
			t.Fatalf("a fills line runs to %d runes, over the summary's own %d.\nline: %s", length, summaryLineRunes, line)
		}
	}
	for _, id := range []string{"#100000", "#100019"} {
		if !strings.Contains(summary, id) {
			t.Fatalf("the wrapped fill group dropped %s.\nsummary:\n%s", id, summary)
		}
	}
}
