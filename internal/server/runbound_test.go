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
	return loop.DerivationBound + time.Duration(loop.MaxFills)*loop.FillBound + judgementFloor()
}

func TestTheFillPhaseLeavesAWholeJudgementCallReachableInsideTheRunBound(t *testing.T) {
	t.Parallel()

	const wantRunBound = 10 * time.Minute
	const wantFillBound = 90 * time.Second

	if runBound != wantRunBound {
		t.Fatalf("runBound = %v, want %v", runBound, wantRunBound)
	}
	if loop.FillBound != wantFillBound {
		t.Fatalf("loop.FillBound = %v, want %v", loop.FillBound, wantFillBound)
	}

	fills := time.Duration(loop.MaxFills) * loop.FillBound

	if claimed := fillPhaseClaim(); claimed >= runBound {
		t.Fatalf("a turn spends the derivation bound %v, then up to %d fills at %v each (%v), and still owes one judgement call at the adapters' own %v bound — %v against a run bound of %v. The fill runs before any judgement call, so a cold-start turn is cancelled before the model is asked anything",
			loop.DerivationBound, loop.MaxFills, loop.FillBound, fills, judgementFloor(), claimed, runBound)
	}
}

func TestTheRunBoundStillAffordsRetrievalOnceTheFillPhaseAndOneJudgementCallAreCounted(t *testing.T) {
	t.Parallel()

	claimed := fillPhaseClaim()
	margin := runBound - claimed

	if margin < divoid.DefaultTimeout {
		t.Fatalf("the derivation, the fill phase and one judgement call claim %v of the %v run bound, leaving %v — under the %v a single graph read is allowed, so retrieval has no room of its own", claimed, runBound, margin, divoid.DefaultTimeout)
	}
}
