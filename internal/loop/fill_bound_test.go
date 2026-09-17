package loop

import (
	"context"
	"strings"
	"testing"
	"time"
)

type deadlineFill struct {
	deadline time.Time
	hadOne   bool
}

func (f *deadlineFill) Fill(ctx context.Context, _ int64) (FillResult, error) {
	f.deadline, f.hadOne = ctx.Deadline()
	return FillResult{Written: true, Model: "test-condense-model"}, nil
}

const fillTestGrace = 2 * time.Second

type blockingFill struct{}

func (f *blockingFill) Fill(ctx context.Context, _ int64) (FillResult, error) {
	select {
	case <-ctx.Done():
		return FillResult{Reason: "model call failed: " + ctx.Err().Error()}, nil
	case <-time.After(fillTestGrace):
		return FillResult{Written: true, Model: "an attempt nothing ever interrupted"}, nil
	}
}

func TestFillGivesEachAttemptADeadlineOfItsOwnRatherThanTheWholeRuns(t *testing.T) {
	t.Parallel()

	port := &deadlineFill{}
	turn := &Turn{Fill: port, logger: testLogger()}

	turn.fill(context.Background(), []Disposition{eligibleRow(1)})
	after := time.Now()

	if !port.hadOne {
		t.Fatal("the fill port was called on a context carrying no deadline at all: two attempts at the model adapter's own five-minute default consume the whole run bound before a judgement call is made")
	}
	if port.deadline.After(after.Add(FillBound)) {
		t.Fatalf("the attempt's deadline is %v past the call, beyond the %v fill bound", port.deadline.Sub(after), FillBound)
	}
}

func TestFillNeverExtendsADeadlineTheRunAlreadyCarries(t *testing.T) {
	t.Parallel()

	const runsOwnBound = 20 * time.Millisecond

	port := &deadlineFill{}
	turn := &Turn{Fill: port, logger: testLogger()}

	ctx, cancel := context.WithTimeout(context.Background(), runsOwnBound)
	defer cancel()

	turn.fill(ctx, []Disposition{eligibleRow(1)})
	after := time.Now()

	if !port.hadOne {
		t.Fatal("the fill port was called on a context carrying no deadline at all")
	}
	if port.deadline.After(after.Add(runsOwnBound)) {
		t.Fatalf("the attempt's deadline is %v past the call, beyond the enclosing run's own %v: the fill replaced the run's deadline instead of tightening it", port.deadline.Sub(after), runsOwnBound)
	}
}

func TestFillRecordsItsOwnBoundAsTheRefusalWhenAnAttemptOverrunsIt(t *testing.T) {
	t.Parallel()

	port := &blockingFill{}
	turn := &Turn{Fill: port, logger: testLogger()}

	got := turn.attemptFill(context.Background(), 1, time.Millisecond)

	if got.Filled {
		t.Fatalf("outcome = %+v: the attempt ran to completion %v after its own %v bound should have cut it, so the fill carries no deadline of its own", got, fillTestGrace, time.Millisecond)
	}
	if got.Reason != fillReasonBoundExpired {
		t.Fatalf("outcome.Reason = %q, want %q", got.Reason, fillReasonBoundExpired)
	}
}

func TestAnAttemptEndedByTheRunsOwnCancellationIsNotReportedAsTheFillsBound(t *testing.T) {
	t.Parallel()

	port := &blockingFill{}
	turn := &Turn{Fill: port, logger: testLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := turn.attemptFill(ctx, 1, time.Minute)

	if got.Reason == fillReasonBoundExpired {
		t.Fatalf("outcome.Reason = %q, but the fill's own bound never expired — the enclosing run ended", got.Reason)
	}
	if !strings.Contains(got.Reason, "model call failed") {
		t.Fatalf("outcome.Reason = %q, want the port's own account of why it stopped", got.Reason)
	}
}

func TestFillBoundsAnOverlongCondenserRefusalBeforeItReachesTheRecord(t *testing.T) {
	t.Parallel()

	upstream := "model call failed: unexpected status 502: " + strings.Repeat("x", 4096)

	fill := &fakeFill{results: map[int64]FillResult{1: {Reason: upstream}}}
	turn := &Turn{Fill: fill, logger: testLogger()}

	outcomes := turn.fill(context.Background(), []Disposition{eligibleRow(1)})

	got := outcomeFor(t, outcomes, 1)
	if length := len([]rune(got.Reason)); length != CarriedCauseRunes {
		t.Fatalf("outcome.Reason is %d runes, want it bounded to %d: the condenser's refusal carries up to 4 KB of an upstream error body into a record that is written to the graph", length, CarriedCauseRunes)
	}
	if !strings.HasPrefix(got.Reason, "model call failed: unexpected status 502: ") {
		t.Fatalf("outcome.Reason = %q, want the head of the condenser's own refusal kept", got.Reason)
	}
}

func TestFillLeavesAGateRefusalUnchangedByTheCauseBound(t *testing.T) {
	t.Parallel()

	turn := &Turn{Fill: nil, logger: testLogger()}

	outcomes := turn.fill(context.Background(), []Disposition{eligibleRow(1)})

	got := outcomeFor(t, outcomes, 1)
	if got.Reason != fillReasonPortAbsent {
		t.Fatalf("outcome.Reason = %q, want %q", got.Reason, fillReasonPortAbsent)
	}
}
