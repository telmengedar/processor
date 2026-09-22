package loop

import (
	"math"
	"testing"
	"time"
)

const (
	budgetStep          = 32
	requiredSpareShare  = 0.10
	slowFloorPerSecond  = 1.0
	quickFloorPerSecond = 10_000.0
)

func headroomOf(t *testing.T, name string, sites []CallSite, floors Floors) Headroom {
	t.Helper()
	for _, h := range Afford(sites, floors) {
		if h.Name == name {
			return h
		}
	}
	t.Fatalf("no call site named %q is in the affordability report", name)
	return Headroom{}
}

func spareShare(h Headroom) float64 {
	return float64(h.Spare) / float64(h.Allowed)
}

func loopOwnedSites() []CallSite {
	return CallSites(5*time.Minute, 10*time.Minute, 0)
}

func TestEveryCallSiteTheLoopOwnsIsAffordableAtTheProductsOwnDeclaredFloors(t *testing.T) {
	t.Parallel()

	for _, h := range Afford(loopOwnedSites(), Floors{}) {
		if !h.Affordable() {
			t.Fatalf("call site %q declares a budget of %d tokens and a prompt ceiling of %d bytes, which needs %v against the %v the safety factor leaves of its %v bound — a shipped budget must satisfy the affordability property at the product's own declaration",
				h.Name, h.Budget, h.PromptCeiling, h.Generation+h.Prompt, h.Allowed, h.Bound)
		}
	}
}

func TestNoCallSitesBudgetCouldBeOneStepLargerAndStillLeaveItsSpareShare(t *testing.T) {
	t.Parallel()

	sites := loopOwnedSites()

	for i, h := range Afford(sites, Floors{}) {
		if got := spareShare(h); got < requiredSpareShare {
			t.Fatalf("call site %q leaves %.3f of its allowed time spare, under the %.2f the shipped budgets were derived to keep", h.Name, got, requiredSpareShare)
		}

		sites[i].Budget += budgetStep
		next := Afford(sites, Floors{})[i]
		if spareShare(next) >= requiredSpareShare {
			t.Fatalf("call site %q would still leave %.3f spare at %d tokens, so %d is not the largest budget the property affords in steps of %d — the shipped value is below its own derivation",
				h.Name, spareShare(next), sites[i].Budget, h.Budget, budgetStep)
		}
		sites[i].Budget -= budgetStep
	}
}

func TestTheShippedBudgetsAreDistinctSoNoReportCanConfuseOneCallSiteForAnother(t *testing.T) {
	t.Parallel()

	if DerivationBudget == JudgementBudget {
		t.Fatalf("the derivation and judgement sites both declare %d tokens, so a report naming one is indistinguishable from a report naming the other", DerivationBudget)
	}
}

func TestTheJudgementBoundIsTheRunBoundsPerCallShareWhereTheClientBoundIsTheLooserOfTheTwo(t *testing.T) {
	t.Parallel()

	const want = 65 * time.Second

	if got := JudgementBound(5*time.Minute, 10*time.Minute); got != want {
		t.Fatalf("JudgementBound = %v, want %v — the run bound less the derivation and the whole fill phase, shared over the call cap, is what actually cuts a judgement call", got, want)
	}
}

func TestTheJudgementBoundIsTheClientBoundWhereThatIsTheTighterOfTheTwo(t *testing.T) {
	t.Parallel()

	const clientBound = 20 * time.Second

	if got := JudgementBound(clientBound, 10*time.Minute); got != clientBound {
		t.Fatalf("JudgementBound = %v, want the adapter's own %v — a bound tighter than the run's share is the one that cuts the call", got, clientBound)
	}
}

func TestAffordNamesTheGenerationRateAHostWouldHaveToDeliverForASiteItCannotAfford(t *testing.T) {
	t.Parallel()

	h := headroomOf(t, SiteJudgement, loopOwnedSites(), Floors{TokensPerSecond: slowFloorPerSecond, PromptBytesPerSecond: DefaultFloorPromptBytesPerSecond})

	if h.Affordable() {
		t.Fatalf("a judgement budget of %d tokens is reported affordable at %g tokens per second", h.Budget, slowFloorPerSecond)
	}

	const want = 7.578947
	if math.Abs(h.RequiredTokensPerSecond-want) > 1e-6 {
		t.Fatalf("RequiredTokensPerSecond = %v, want %v — the budget over whatever is left of the allowed time once the prompt is paid for", h.RequiredTokensPerSecond, want)
	}
	if h.RequiredTokensPerSecond <= slowFloorPerSecond {
		t.Fatalf("the rate the host would have to deliver (%v) is not above the rate the deployment declared (%v), so the report names no gap at all", h.RequiredTokensPerSecond, slowFloorPerSecond)
	}
}

func TestAffordLeavesNoRequiredGenerationRateWhereThePromptAloneClaimsTheWholeBound(t *testing.T) {
	t.Parallel()

	h := headroomOf(t, SiteJudgement, loopOwnedSites(), Floors{TokensPerSecond: quickFloorPerSecond, PromptBytesPerSecond: 1})

	if h.Affordable() {
		t.Fatalf("a site whose prompt alone needs %v against %v allowed is reported affordable", h.Prompt, h.Allowed)
	}
	if h.RequiredTokensPerSecond != 0 {
		t.Fatalf("RequiredTokensPerSecond = %v, want 0 — no generation rate affords a budget whose prompt has already spent the bound", h.RequiredTokensPerSecond)
	}
	if h.RequiredPromptBytesPerSecond <= 0 {
		t.Fatalf("RequiredPromptBytesPerSecond = %v, want the rate the host would have to read the prompt at", h.RequiredPromptBytesPerSecond)
	}
}

func TestADeclaredFloorTheDeploymentSuppliesIsUsedRatherThanTheProductsOwn(t *testing.T) {
	t.Parallel()

	declared := Floors{TokensPerSecond: 37, PromptBytesPerSecond: 4_100}

	if got := declared.Resolved(); got != declared {
		t.Fatalf("Resolved() = %+v, want the deployment's own declaration %+v", got, declared)
	}
}

func TestAFloorTheDeploymentDeclaresNothingUsableForFallsBackToTheProductsOwnDeclaration(t *testing.T) {
	t.Parallel()

	want := Floors{TokensPerSecond: DefaultFloorTokensPerSecond, PromptBytesPerSecond: DefaultFloorPromptBytesPerSecond}

	for _, declared := range []Floors{{}, {TokensPerSecond: -1, PromptBytesPerSecond: -1}} {
		if got := declared.Resolved(); got != want {
			t.Fatalf("Resolved() of %+v = %+v, want the product's own declaration %+v", declared, got, want)
		}
	}
}

func TestTheFillSiteIsCheckedOnlyWhereACondensationModelIsConfigured(t *testing.T) {
	t.Parallel()

	const fillBudget = 4_242

	without := CallSites(5*time.Minute, 10*time.Minute, 0)
	for _, site := range without {
		if site.Name == SiteFill {
			t.Fatalf("the fill site is in the report of a deployment that configured no condensation model: %+v", site)
		}
	}

	with := CallSites(5*time.Minute, 10*time.Minute, fillBudget)
	if len(with) != len(without)+1 {
		t.Fatalf("configuring a condensation model added %d call sites, want exactly one", len(with)-len(without))
	}
	last := with[len(with)-1]
	if last.Name != SiteFill || last.Budget != fillBudget || last.Bound != FillBound {
		t.Fatalf("the fill site reads %+v, want the configured budget of %d bounded at the fill's own %v", last, fillBudget, FillBound)
	}
}

func TestTheDerivationPromptCeilingLeavesRoomForARunsInputBesideItsOwnInstructions(t *testing.T) {
	t.Parallel()

	instructions := len(DerivationPrompt("", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))

	if instructions >= DerivationPromptCeiling {
		t.Fatalf("the derivation's own instructions are %d bytes against a declared ceiling of %d, so the ceiling prices no input at all", instructions, DerivationPromptCeiling)
	}
	if room := DerivationPromptCeiling - instructions; room < instructions {
		t.Fatalf("the derivation prompt ceiling leaves %d bytes for the run's input beside %d bytes of instructions, less than the instructions themselves", room, instructions)
	}
}

func TestOneJudgementCallIsPricedAtItsOwnBudgetAndPromptCeilingTogether(t *testing.T) {
	t.Parallel()

	floors := Floors{TokensPerSecond: 8, PromptBytesPerSecond: 2_000}

	want := time.Duration(float64(JudgementBudget)/8*float64(time.Second)) + time.Duration(float64(JudgementPromptCeiling)/2_000*float64(time.Second))

	if got := JudgementCost(floors); got != want {
		t.Fatalf("JudgementCost = %v, want %v — the output budget at the generation floor plus the prompt ceiling at the prompt floor", got, want)
	}
}
