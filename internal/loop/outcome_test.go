package loop

import (
	"encoding/json"
	"maps"
	"testing"
)

func outcomeRecord() Record {
	return Record{
		Answer:     "an answer the run produced",
		Candidates: []Disposition{{Rank: 1, ID: 11, Included: true}},
		ToolCalls:  []ToolCallRecord{},
		StopReason: StopReason{Reason: Answered, Raw: "stop"},
	}
}

func TestAnAnswerOfNothingButWhitespaceCountsAsHavingProducedNothing(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Answer = " \t\n "

	outcome := ComputeOutcome(record)

	if outcome.Produced {
		t.Fatalf("an answer of %q is reported as produced; produced is at least one non-whitespace character", record.Answer)
	}
	if outcome.Verdict != VerdictEmpty {
		t.Fatalf("verdict = %q, want %q for a run whose whole answer is whitespace", outcome.Verdict, VerdictEmpty)
	}
}

func TestASingleNonWhitespaceCharacterIsEnoughToCountAsProduced(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Answer = " \n. \t"

	outcome := ComputeOutcome(record)

	if !outcome.Produced {
		t.Fatalf("an answer of %q is reported as having produced nothing; one non-whitespace character is the whole bar", record.Answer)
	}
	if outcome.Verdict != VerdictDelivered {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictDelivered)
	}
}

func TestAnAdmittedBlockCandidateGroundsTheRun(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{
		{Rank: 1, ID: 11, CutReason: "byte budget exceeded"},
		{Rank: 2, ID: 12, Included: true},
	}

	if outcome := ComputeOutcome(record); !outcome.Grounded {
		t.Fatalf("a run whose block admitted a candidate is reported ungrounded; outcome = %+v", outcome)
	}
}

func TestAnAdmittedRowFromADispatchedRoundGroundsARunWhoseBlockAdmittedNothing(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}
	record.ToolCalls = []ToolCallRecord{{Tool: ToolRecall, Query: "the missing thing", Results: []Disposition{{Rank: 1, ID: 21, Included: true}}}}

	outcome := ComputeOutcome(record)

	if !outcome.Grounded {
		t.Fatalf("a row a dispatched recall admitted mid-turn grounds nothing; outcome = %+v", outcome)
	}
	if outcome.Verdict != VerdictDelivered {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictDelivered)
	}
}

const archivedCapCause = "call cap reached"

func TestAnAdmittedRowInsideAnErroredRoundGroundsNothingBecauseItNeverReachedTheModel(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}
	record.ToolCalls = []ToolCallRecord{{
		Tool:    ToolRecall,
		Query:   "the missing thing",
		Error:   archivedCapCause,
		Results: []Disposition{{Rank: 1, ID: 21, Included: true}},
	}}

	outcome := ComputeOutcome(record)

	if outcome.Grounded {
		t.Fatalf("a round that ended in an error was counted as grounding the run; its rows were never put in front of the model; outcome = %+v", outcome)
	}
	if outcome.Verdict != VerdictUngrounded {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictUngrounded)
	}
}

func TestARunWhoseEveryRowWasCutIsNotGrounded(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{
		{Rank: 1, ID: 11, CutReason: "below relevance floor"},
		{Rank: 2, ID: 12, CutReason: "self-produced"},
	}
	record.ToolCalls = []ToolCallRecord{{Tool: ToolRecall, Query: "more", Results: []Disposition{{Rank: 1, ID: 21, CutReason: "byte budget exceeded"}}}}

	if outcome := ComputeOutcome(record); outcome.Grounded {
		t.Fatalf("a run that admitted no row anywhere is reported grounded; outcome = %+v", outcome)
	}
}

func TestATruncatedAnswerIsNotCurtailedSoARecordCanReadUngroundedAndTruncatedAtOnce(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}
	record.StopReason = StopReason{Reason: Truncated, Raw: "length"}
	record.Limits = Limits{JudgementBudget: JudgementBudget}

	outcome := ComputeOutcome(record)

	if outcome.Curtailed {
		t.Fatalf("an output bound was read as curtailing the turn; only a tool-dispatch bound curtails; outcome = %+v", outcome)
	}
	if outcome.Verdict != VerdictUngrounded {
		t.Fatalf("verdict = %q, want %q so that the verdict and the terminal reason each keep their own fact", outcome.Verdict, VerdictUngrounded)
	}
}

func TestARunStoppedAtTheCallCapWithNoTextReadsCurtailedRatherThanEmpty(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Answer = ""
	record.CapReached = true
	record.StopReason = StopReason{Reason: WantsRecall, Raw: "tool_calls"}

	outcome := ComputeOutcome(record)

	if outcome.Verdict != VerdictCurtailed {
		t.Fatalf("verdict = %q, want %q: the run is not empty because the model had nothing to say, it is empty because the loop stopped it while it was still asking, and the verdict reports the cause and not the symptom",
			outcome.Verdict, VerdictCurtailed)
	}
	if outcome.Produced {
		t.Fatalf("produced = true on a run with no answer; the curtailed verdict must not hide that the run produced nothing")
	}
}

func TestARunStoppedAtTheCallCapWithoutGroundingReadsCurtailedRatherThanUngrounded(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}
	record.CapReached = true

	if outcome := ComputeOutcome(record); outcome.Verdict != VerdictCurtailed {
		t.Fatalf("verdict = %q, want %q: a bound that fired outranks an answer nothing fed", outcome.Verdict, VerdictCurtailed)
	}
}

func TestARunThatProducedNothingUnderNoBoundReadsEmptyRatherThanUngrounded(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Answer = ""
	record.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}

	if outcome := ComputeOutcome(record); outcome.Verdict != VerdictEmpty {
		t.Fatalf("verdict = %q, want %q: nothing was produced, so nothing can be said about what fed it", outcome.Verdict, VerdictEmpty)
	}
}

func TestAProducedAnswerNoAdmittedRowFedReadsUngrounded(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}

	if outcome := ComputeOutcome(record); outcome.Verdict != VerdictUngrounded {
		t.Fatalf("verdict = %q, want %q: this system's memory contributed nothing to this answer", outcome.Verdict, VerdictUngrounded)
	}
}

func TestAProducedAnswerAnAdmittedRowFedUnderNoBoundReadsDelivered(t *testing.T) {
	t.Parallel()

	if outcome := ComputeOutcome(outcomeRecord()); outcome.Verdict != VerdictDelivered {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictDelivered)
	}
}

func TestTheVerdictReportsTheEarliestBindingCauseOverEveryCombinationOfTheThreePredicates(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		produced  bool
		grounded  bool
		curtailed bool
		want      Verdict
	}{
		{false, false, false, VerdictEmpty},
		{false, false, true, VerdictCurtailed},
		{false, true, false, VerdictEmpty},
		{false, true, true, VerdictCurtailed},
		{true, false, false, VerdictUngrounded},
		{true, false, true, VerdictCurtailed},
		{true, true, false, VerdictDelivered},
		{true, true, true, VerdictCurtailed},
	} {
		record := outcomeRecord()
		record.Answer = ""
		if c.produced {
			record.Answer = "text"
		}
		record.Candidates = []Disposition{{Rank: 1, ID: 11, Included: c.grounded, CutReason: "below relevance floor"}}
		record.CapReached = c.curtailed

		outcome := ComputeOutcome(record)

		if outcome.Verdict != c.want {
			t.Fatalf("produced=%t grounded=%t curtailed=%t -> verdict %q, want %q",
				c.produced, c.grounded, c.curtailed, outcome.Verdict, c.want)
		}
		if outcome.Produced != c.produced || outcome.Grounded != c.grounded || outcome.Curtailed != c.curtailed {
			t.Fatalf("the three predicates are recorded as produced=%t grounded=%t curtailed=%t, want %t/%t/%t; each one stays legible beside the verdict",
				outcome.Produced, outcome.Grounded, outcome.Curtailed, c.produced, c.grounded, c.curtailed)
		}
	}
}

func TestActedCountsTheRoundsThatCompletedWithoutErrorUnderTheirOwnToolNames(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.ToolCalls = []ToolCallRecord{
		{Tool: ToolRecall, Query: "first"},
		{Tool: ToolWriteFile, Path: "index.html", Bytes: 11},
		{Tool: ToolRecall, Query: "second"},
	}

	acted := ComputeOutcome(record).Acted

	if want := map[string]int{ToolRecall: 2, ToolWriteFile: 1}; !maps.Equal(acted, want) {
		t.Fatalf("acted = %v, want %v", acted, want)
	}
}

func TestActedExcludesARoundThatCarriedAnError(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.ToolCalls = []ToolCallRecord{
		{Tool: ToolRecall, Query: "dispatched"},
		{Tool: ToolRecall, Query: "refused", Error: archivedCapCause},
	}

	acted := ComputeOutcome(record).Acted

	if want := map[string]int{ToolRecall: 1}; !maps.Equal(acted, want) {
		t.Fatalf("acted = %v, want %v; a round that errored is not a round that acted", acted, want)
	}
}

func TestActedGroupsARoundWithNoRecordedToolUnderAnEmptyNameRatherThanInventingOne(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.ToolCalls = []ToolCallRecord{{Query: "a round from before the tool name was written down"}}

	acted := ComputeOutcome(record).Acted

	if want := map[string]int{"": 1}; !maps.Equal(acted, want) {
		t.Fatalf("acted = %v, want %v", acted, want)
	}
}

func TestActedMovesNoVerdict(t *testing.T) {
	t.Parallel()

	without := outcomeRecord()
	with := outcomeRecord()
	with.ToolCalls = []ToolCallRecord{{Tool: ToolRecall, Query: "a round that admitted nothing", Results: []Disposition{}}}

	quiet, busy := ComputeOutcome(without), ComputeOutcome(with)

	if len(quiet.Acted) != 0 || len(busy.Acted) == 0 {
		t.Fatalf("test setup error: acted = %v and %v, want the second to have acted and the first not to", quiet.Acted, busy.Acted)
	}
	if quiet.Verdict != busy.Verdict {
		t.Fatalf("the verdict moved from %q to %q on a record differing only in what it acted; no verdict may consume acted", quiet.Verdict, busy.Verdict)
	}
}

func TestEveryVerdictIsRecomputableFromItsOwnRecordAfterARoundTripThroughJson(t *testing.T) {
	t.Parallel()

	curtailed := outcomeRecord()
	curtailed.Answer = ""
	curtailed.CapReached = true

	empty := outcomeRecord()
	empty.Answer = ""

	ungrounded := outcomeRecord()
	ungrounded.Candidates = []Disposition{{Rank: 1, ID: 11, CutReason: "below relevance floor"}}

	for _, record := range []Record{outcomeRecord(), curtailed, empty, ungrounded} {
		record.Outcome = ComputeOutcome(record)

		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal record: %v", err)
		}

		var decoded Record
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("decode record: %v; body=%s", err, encoded)
		}

		recomputed := ComputeOutcome(decoded)
		if recomputed.Verdict != record.Outcome.Verdict {
			t.Fatalf("the stored verdict %q recomputes to %q from the record's own fields; body=%s", record.Outcome.Verdict, recomputed.Verdict, encoded)
		}
		if !maps.Equal(recomputed.Acted, decoded.Outcome.Acted) {
			t.Fatalf("acted recomputes to %v against the stored %v; body=%s", recomputed.Acted, decoded.Outcome.Acted, encoded)
		}
	}
}

func TestTheVerdictIsDerivedFromTheRecordsFactsAndNeverReadBackFromTheStoredOutcome(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.Outcome = Outcome{Verdict: VerdictCurtailed, Curtailed: true}

	if outcome := ComputeOutcome(record); outcome.Verdict != VerdictDelivered {
		t.Fatalf("verdict = %q, want %q: a stored outcome is a rendering of the facts, never an input to them", outcome.Verdict, VerdictDelivered)
	}
}

func TestARunTheRemainingTimeGuardStoppedIsCurtailedLikeAnyOtherBoundThatEndedTheTurnEarly(t *testing.T) {
	t.Parallel()

	record := outcomeRecord()
	record.TimeShortfall = "the run's remaining time cannot afford another judgement call: 3s left, and one call needs 45.866s"

	outcome := ComputeOutcome(record)

	if !outcome.Curtailed {
		t.Fatalf("a run stopped short of its call cap by the remaining-time guard reads as an ordinary ending; outcome = %+v", outcome)
	}
	if outcome.Verdict != VerdictCurtailed {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictCurtailed)
	}

	record.TimeShortfall = ""
	if ComputeOutcome(record).Curtailed {
		t.Fatal("the same run reads as curtailed with no shortfall recorded, so the shortfall is not what the verdict turns on")
	}
}
