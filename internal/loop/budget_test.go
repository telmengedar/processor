package loop

import (
	"math"
	"testing"
	"time"
)

const (
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
	return CallSites(5*time.Minute, 10*time.Minute)
}

func limitedSites(budget int) []CallSite {
	sites := loopOwnedSites()
	for i := range sites {
		sites[i].Budget = budget
	}
	return sites
}

func TestEveryCallSiteTheLoopOwnsIsAffordableAtTheProductsOwnDeclaredFloors(t *testing.T) {
	t.Parallel()

	for _, h := range Afford(loopOwnedSites(), Floors{}) {
		if !h.Affordable() {
			t.Fatalf("call site %q declares a budget of %d tokens and a prompt ceiling of %d bytes, which needs %v against the %v the safety factor leaves of its %v bound — a shipped site must satisfy the affordability property at the product's own declaration",
				h.Name, h.Budget, h.PromptCeiling, h.Generation+h.Prompt, h.Allowed, h.Bound)
		}
	}
}

func TestEveryCallSiteTheLoopOwnsDeclaresNoOutputBudget(t *testing.T) {
	t.Parallel()

	for _, site := range loopOwnedSites() {
		if site.Budget != 0 {
			t.Fatalf("call site %q declares an output budget of %d tokens; processor sets no output limit on any call", site.Name, site.Budget)
		}
	}
}

func TestAnUnlimitedSiteIsAffordableExactlyWhenItsPromptFitsTheShareOfItsBoundTheSafetyFactorLeaves(t *testing.T) {
	t.Parallel()

	for _, site := range loopOwnedSites() {
		allowed := time.Duration(float64(site.Bound) * AffordabilitySafety)
		exact := float64(site.PromptCeiling) / allowed.Seconds()

		fits := headroomOf(t, site.Name, []CallSite{site}, Floors{TokensPerSecond: slowFloorPerSecond, PromptBytesPerSecond: exact * 1.001})
		if !fits.Affordable() {
			t.Fatalf("call site %q is reported unaffordable at a prompt rate of %.2f B/s, just above the %.2f B/s its prompt needs; generation is not priced where there is no output limit", site.Name, exact*1.001, exact)
		}
		if fits.Generation != 0 {
			t.Fatalf("call site %q prices generation at %v with no output limit", site.Name, fits.Generation)
		}

		short := headroomOf(t, site.Name, []CallSite{site}, Floors{TokensPerSecond: quickFloorPerSecond, PromptBytesPerSecond: exact * 0.999})
		if short.Affordable() {
			t.Fatalf("call site %q is reported affordable at a prompt rate of %.2f B/s, just below the %.2f B/s its prompt needs", site.Name, exact*0.999, exact)
		}
		if short.RequiredTokensPerSecond != 0 {
			t.Fatalf("call site %q names a required generation rate of %v with no output limit", site.Name, short.RequiredTokensPerSecond)
		}
	}
}

func TestTheJudgementBoundIsTheRunBoundsPerCallShareWhereTheClientBoundIsTheLooserOfTheTwo(t *testing.T) {
	t.Parallel()

	const want = 80 * time.Second

	if got := JudgementBound(5*time.Minute, 10*time.Minute); got != want {
		t.Fatalf("JudgementBound = %v, want %v — the run bound less the derivation and one substance wait, shared over the call cap, is what actually cuts a judgement call", got, want)
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

	h := headroomOf(t, SiteJudgement, limitedSites(192), Floors{TokensPerSecond: slowFloorPerSecond, PromptBytesPerSecond: DefaultFloorPromptBytesPerSecond})

	if h.Affordable() {
		t.Fatalf("a judgement budget of %d tokens is reported affordable at %g tokens per second", h.Budget, slowFloorPerSecond)
	}

	const want = 5.142857
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

func TestOneJudgementCallWithNoOutputLimitIsPricedAtItsPromptCeilingAlone(t *testing.T) {
	t.Parallel()

	floors := Floors{TokensPerSecond: 8, PromptBytesPerSecond: 2_000}

	want := time.Duration(float64(JudgementPromptCeiling) / 2_000 * float64(time.Second))

	if got := JudgementCost(floors); got != want {
		t.Fatalf("JudgementCost = %v, want %v — the prompt ceiling at the prompt floor and no generation term", got, want)
	}
}
