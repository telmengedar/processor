package loop

import (
	"context"
	"errors"
	"testing"
)

type fakeFill struct {
	calls   []int64
	results map[int64]FillResult
	errs    map[int64]error
}

func (f *fakeFill) Fill(_ context.Context, id int64) (FillResult, error) {
	f.calls = append(f.calls, id)
	if f.errs != nil {
		if err, ok := f.errs[id]; ok {
			return FillResult{}, err
		}
	}
	if f.results != nil {
		if r, ok := f.results[id]; ok {
			return r, nil
		}
	}
	return FillResult{Written: true, Model: "test-condense-model"}, nil
}

func eligibleRow(id int64) Disposition {
	return Disposition{ID: id, Included: false, CutReason: cutReasonByteBudget, Size: FillSizeFloor}
}

func TestFillNeverAttemptsACandidateThatAlreadyCarriesASubstance(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{{ID: 1, Included: false, CutReason: cutReasonByteBudget, Size: FillSizeFloor, SubstanceAvailable: true}}
	outcomes := turn.fill(context.Background(), rows)

	for _, o := range outcomes {
		if o.ID == 1 {
			t.Fatalf("outcomes = %+v, want no entry at all for a candidate that already carries a substance", outcomes)
		}
	}
	if len(fill.calls) != 0 {
		t.Fatalf("fill.calls = %v, want the port never called for a candidate that already has a substance", fill.calls)
	}
}

func TestFillRecordsNoPressureForAnAdmittedCandidate(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{
		{ID: 1, Included: true, Size: FillSizeFloor},
		eligibleRow(2),
	}
	outcomes := turn.fill(context.Background(), rows)

	got := outcomeFor(t, outcomes, 1)
	if got.Filled || got.Reason != fillReasonNoPressure {
		t.Fatalf("outcome = %+v, want a no-pressure refusal for the admitted row", got)
	}
	if len(fill.calls) != 1 || fill.calls[0] != 2 {
		t.Fatalf("fill.calls = %v, want exactly one attempt, for id 2 alone", fill.calls)
	}
}

func TestFillRecordsNoPressureForACandidateCutForAnotherReason(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{{ID: 1, Included: false, CutReason: cutReasonSelfProduced, Size: FillSizeFloor}}
	outcomes := turn.fill(context.Background(), rows)

	got := outcomeFor(t, outcomes, 1)
	if got.Filled || got.Reason != fillReasonNoPressure {
		t.Fatalf("outcome = %+v, want a no-pressure refusal for a non-byte-budget cut", got)
	}
	if len(fill.calls) != 0 {
		t.Fatalf("fill.calls = %v, want the port never called", fill.calls)
	}
}

func TestFillRecordsBelowSizeGateAndNeverCallsThePort(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{{ID: 1, Included: false, CutReason: cutReasonByteBudget, Size: FillSizeFloor - 1}}
	outcomes := turn.fill(context.Background(), rows)

	got := outcomeFor(t, outcomes, 1)
	if got.Filled || got.Reason != fillReasonBelowSizeGate {
		t.Fatalf("outcome = %+v, want a below-size-gate refusal", got)
	}
	if len(fill.calls) != 0 {
		t.Fatalf("fill.calls = %v, want the port never called below the size gate", fill.calls)
	}
}

func TestTheSizeGateWinsOverPortAbsence(t *testing.T) {
	t.Parallel()

	turn := &Turn{Fill: nil, logger: testLogger()}

	rows := []Disposition{{ID: 1, Included: false, CutReason: cutReasonByteBudget, Size: FillSizeFloor - 1}}
	outcomes := turn.fill(context.Background(), rows)

	got := outcomeFor(t, outcomes, 1)
	if got.Reason != fillReasonBelowSizeGate {
		t.Fatalf("outcome.Reason = %q, want %q even with the port absent", got.Reason, fillReasonBelowSizeGate)
	}
}

func TestFillRefusesWithPortAbsentReasonWhenNoCondensationModelIsConfigured(t *testing.T) {
	t.Parallel()

	turn := &Turn{Fill: nil, logger: testLogger()}

	outcomes := turn.fill(context.Background(), []Disposition{eligibleRow(1)})

	got := outcomeFor(t, outcomes, 1)
	if got.Filled || got.Reason != fillReasonPortAbsent {
		t.Fatalf("outcome = %+v, want a port-absent refusal", got)
	}
}

func TestFillAttemptsEligibleCandidatesUpToTheCeilingAndRefusesTheRest(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{eligibleRow(1), eligibleRow(2), eligibleRow(3)}
	outcomes := turn.fill(context.Background(), rows)

	if len(fill.calls) != MaxFills {
		t.Fatalf("fill.calls = %v, want exactly MaxFills (%d) attempts", fill.calls, MaxFills)
	}
	for _, id := range []int64{1, 2} {
		if got := outcomeFor(t, outcomes, id); !got.Filled {
			t.Fatalf("outcome for id %d = %+v, want it filled — it is within the ceiling", id, got)
		}
	}
	third := outcomeFor(t, outcomes, 3)
	if third.Filled || third.Reason != fillReasonCeilingReached {
		t.Fatalf("outcome for id 3 = %+v, want a ceiling-reached refusal", third)
	}
}

func TestFillRecordsTheProducingModelOnASuccessfulWrite(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{results: map[int64]FillResult{1: {Written: true, Model: "gemma-3-12b-it"}}}
	turn := &Turn{Fill: fill, logger: testLogger()}

	outcomes := turn.fill(context.Background(), []Disposition{eligibleRow(1)})

	got := outcomeFor(t, outcomes, 1)
	if !got.Filled || got.Model != "gemma-3-12b-it" {
		t.Fatalf("outcome = %+v, want it filled and naming the producing model", got)
	}
	if got.Reason != "" {
		t.Fatalf("outcome.Reason = %q, want empty on a successful fill", got.Reason)
	}
}

func TestFillSurfacesTheCondensersOwnRefusalReasonWhenItFitsTheCarriedCauseBound(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{results: map[int64]FillResult{1: {Reason: "self-produced"}}}
	turn := &Turn{Fill: fill, logger: testLogger()}

	outcomes := turn.fill(context.Background(), []Disposition{eligibleRow(1)})

	got := outcomeFor(t, outcomes, 1)
	if got.Filled || got.Reason != "self-produced" {
		t.Fatalf("outcome = %+v, want the condenser's own reason surfaced unchanged", got)
	}
}

func TestFillWrapsATransportFailureAsABoundedReasonWithoutFailingTheRun(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{errs: map[int64]error{1: errors.New("literal: connection reset")}}
	turn := &Turn{Fill: fill, logger: testLogger()}

	outcomes := turn.fill(context.Background(), []Disposition{eligibleRow(1)})

	got := outcomeFor(t, outcomes, 1)
	if got.Filled {
		t.Fatal("outcome.Filled = true for a port that returned an error, want false")
	}
	if got.Reason != "literal: connection reset" {
		t.Fatalf("outcome.Reason = %q, want the port's own failure cause bounded, not swallowed", got.Reason)
	}
}

func TestFillFastRefusesOversizedContentWithoutCallingThePort(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{{ID: 1, Included: false, CutReason: cutReasonByteBudget, Size: MaxFillContentBytes + 1}}
	outcomes := turn.fill(context.Background(), rows)

	got := outcomeFor(t, outcomes, 1)
	if got.Filled || got.Reason != fillReasonOversized {
		t.Fatalf("outcome = %+v, want an oversized refusal", got)
	}
	if len(fill.calls) != 0 {
		t.Fatalf("fill.calls = %v, want the port never called for oversized content — the refusal must be fast", fill.calls)
	}
}

func TestFillNeverExceedsItsOwnCeilingEvenWhenOneAttemptRefuses(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{results: map[int64]FillResult{1: {Reason: "non-prose content type"}}}
	turn := &Turn{Fill: fill, logger: testLogger()}

	rows := []Disposition{eligibleRow(1), eligibleRow(2), eligibleRow(3)}
	outcomes := turn.fill(context.Background(), rows)

	if len(fill.calls) != MaxFills {
		t.Fatalf("fill.calls = %v, want exactly MaxFills (%d) attempts even though the first refused", fill.calls, MaxFills)
	}
	third := outcomeFor(t, outcomes, 3)
	if third.Reason != fillReasonCeilingReached {
		t.Fatalf("outcome for id 3 = %+v, want ceiling-reached: the ceiling counts attempts, not writes", third)
	}
}

func TestFillNeverAttemptsAnythingWhenDispositionsAreEmpty(t *testing.T) {
	t.Parallel()

	fill := &fakeFill{}
	turn := &Turn{Fill: fill, logger: testLogger()}

	outcomes := turn.fill(context.Background(), nil)
	if outcomes == nil {
		t.Fatal("outcomes is nil for an empty candidate set, want a non-nil empty slice")
	}
	if len(outcomes) != 0 || len(fill.calls) != 0 {
		t.Fatalf("outcomes = %+v, fill.calls = %v, want both empty", outcomes, fill.calls)
	}
}

func outcomeFor(t *testing.T, outcomes []FillOutcome, id int64) FillOutcome {
	t.Helper()
	for _, o := range outcomes {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("outcomes = %+v, want an entry for id %d", outcomes, id)
	return FillOutcome{}
}
