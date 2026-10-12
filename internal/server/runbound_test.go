package server

import (
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

func judgementFloor() time.Duration {
	return max(openaicompat.DefaultTimeout, ollama.DefaultTimeout)
}

func fillPhaseClaim() time.Duration {
	return loop.DerivationBound + loop.FillWait + judgementFloor()
}

func TestTheSubstanceWaitLeavesAWholeJudgementCallReachableInsideTheRunBound(t *testing.T) {
	t.Parallel()

	const wantRunBound = 10 * time.Minute
	const wantFillWait = 90 * time.Second

	if RunBound != wantRunBound {
		t.Fatalf("RunBound = %v, want %v", RunBound, wantRunBound)
	}
	if loop.FillWait != wantFillWait {
		t.Fatalf("loop.FillWait = %v, want %v", loop.FillWait, wantFillWait)
	}

	if claimed := fillPhaseClaim(); claimed >= RunBound {
		t.Fatalf("a turn spends the derivation bound %v, then one substance wait of %v, and still owes one judgement call at the adapters' own %v bound — %v against a run bound of %v. The wait runs before any judgement call, so a cold-start turn is cancelled before the model is asked anything",
			loop.DerivationBound, loop.FillWait, judgementFloor(), claimed, RunBound)
	}
}

func TestTheRunBoundStillAffordsRetrievalOnceTheSubstanceWaitAndOneJudgementCallAreCounted(t *testing.T) {
	t.Parallel()

	claimed := fillPhaseClaim()
	margin := RunBound - claimed

	if margin < divoid.DefaultTimeout {
		t.Fatalf("the derivation, the substance wait and one judgement call claim %v of the %v run bound, leaving %v — under the %v a single graph read is allowed, so retrieval has no room of its own", claimed, RunBound, margin, divoid.DefaultTimeout)
	}
}
