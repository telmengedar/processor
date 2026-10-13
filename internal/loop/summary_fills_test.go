package loop

import (
	"strings"
	"testing"
)

func TestRenderSummaryCountsFillsPerPhase(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{
		{ID: 11, Phase: phaseInitial, Filled: true, Model: "gemma-3-12b-it"},
		{ID: 12, Phase: phaseInitial, Deferred: true, Reason: fillReasonWaitExpired},
		{ID: 13, Phase: phaseRecall, Reason: "condensation empty"},
		{ID: 14, Phase: phaseRead, Filled: true, Model: "gemma-3-12b-it"},
	}

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"FILLS  2 filled / 1 failed / 1 deferred\n",
		"  initial: 1 filled / 0 failed / 1 deferred\n",
		"  recall: 0 filled / 1 failed / 0 deferred\n",
		"  read: 1 filled / 0 failed / 0 deferred\n",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not state %q.\nsummary:\n%s", want, summary)
		}
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

func TestRenderSummaryCountsFilledFailedAndDeferredByReadingTheOutcomesItWasGiven(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{
		{ID: 11, Filled: true, Model: "gemma-3-12b-it"},
		{ID: 12, Reason: "condensation empty"},
		{ID: 13, Reason: "condensation empty"},
		{ID: 14, Deferred: true, Reason: fillReasonWaitExpired},
	}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "FILLS  1 filled / 2 failed / 1 deferred") {
		t.Fatalf("the fills line does not count one filled against two failed and one deferred.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryGroupsEachRefusedFillUnderTheReasonThatOutcomeRecords(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = []FillOutcome{
		{ID: 12, Reason: "condensation empty"},
		{ID: 14, Reason: fillReasonWaitExpired, Deferred: true},
		{ID: 13, Reason: "condensation empty"},
	}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "  condensation empty (2): #12 #13\n") {
		t.Fatalf("the two rows refused for the same reason are not grouped under it.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "  fill wait expired (1): #14\n") {
		t.Fatalf("the deferral is not reported under its own reason.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryReportsAFillSectionEvenWhenNoCandidateLackedASubstance(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Fills = nil

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "FILLS  0 filled / 0 failed / 0 deferred") {
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
		record.Fills = append(record.Fills, FillOutcome{ID: int64(100000 + i), Reason: "condensation empty"})
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
