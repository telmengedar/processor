package loop

import "time"

const (
	// DerivationBudget is the output budget one query-derivation call is issued with.
	DerivationBudget = 160
	// JudgementBudget is the output budget one judgement call is issued with.
	JudgementBudget = 192
)

const (
	// DerivationPromptCeiling bounds the bytes one derivation call sends: its instructions and exemplars, plus the run's input.
	DerivationPromptCeiling = 12_000
	// JudgementPromptCeiling bounds the bytes one judgement call sends: the assembled block, plus one supplementary round carried back into the prompt.
	JudgementPromptCeiling = AssemblyByteBudget + SupplementaryByteBudget
)

// AffordabilitySafety is the share of a bound one call site's declared cost may claim, leaving the rest for everything the declaration does not model.
const AffordabilitySafety = 0.8

const (
	// DefaultFloorTokensPerSecond is the generation rate the product declares where the deployment declares none.
	DefaultFloorTokensPerSecond = 10.0
	// DefaultFloorPromptBytesPerSecond is the prompt-processing rate the product declares where the deployment declares none.
	DefaultFloorPromptBytesPerSecond = 3_000.0
)

const (
	// SiteDerivation names the query-derivation call site.
	SiteDerivation = "derivation"
	// SiteJudgement names the judgement call site.
	SiteJudgement = "judgement"
	// SiteFill names the on-demand condensation call site.
	SiteFill = "fill"
)

// Floors is what a deployment declares its model endpoint delivers; it is a declaration, never a measurement.
type Floors struct {
	TokensPerSecond      float64
	PromptBytesPerSecond float64
}

// Resolved returns f with the product's own declaration standing in wherever the deployment declared nothing usable.
func (f Floors) Resolved() Floors {
	if f.TokensPerSecond <= 0 {
		f.TokensPerSecond = DefaultFloorTokensPerSecond
	}
	if f.PromptBytesPerSecond <= 0 {
		f.PromptBytesPerSecond = DefaultFloorPromptBytesPerSecond
	}
	return f
}

// CallSite is one model call the product can issue: the output budget it declares, the prompt it may send, and the bound that will cut it.
type CallSite struct {
	Name          string
	Budget        int
	PromptCeiling int
	Bound         time.Duration

	// Deferred marks a site whose budget this product does not choose, so an unaffordable result is a fact to report rather than a condition the deployment can act on.
	Deferred bool
}

// Headroom is one call site's affordability at a deployment's declared floors.
type Headroom struct {
	CallSite

	Generation time.Duration
	Prompt     time.Duration
	Allowed    time.Duration
	Spare      time.Duration

	// RequiredTokensPerSecond is the generation rate that would afford this budget at the declared prompt floor, zero where no rate does.
	RequiredTokensPerSecond float64
	// RequiredPromptBytesPerSecond is the prompt rate that would afford this prompt at the declared generation floor, zero where no rate does.
	RequiredPromptBytesPerSecond float64
}

// Affordable reports whether the site's declared cost fits inside the share of its bound the safety factor leaves.
func (h Headroom) Affordable() bool {
	return h.Spare > 0
}

// JudgementBound is the bound that cuts one judgement call: the adapter's own client bound, or the run bound's per-call share once the derivation and the fill phase are spent, whichever is smaller.
func JudgementBound(clientBound, runBound time.Duration) time.Duration {
	return min(clientBound, (runBound-DerivationBound-MaxFills*FillBound)/MaxModelCalls)
}

// CallSites is every site a run issues a model call at; a fillBudget of zero or less leaves the fill site out, which is the shape of a deployment configured with no condensation model.
func CallSites(clientBound, runBound time.Duration, fillBudget int) []CallSite {
	sites := []CallSite{
		{Name: SiteDerivation, Budget: DerivationBudget, PromptCeiling: DerivationPromptCeiling, Bound: DerivationBound},
		{Name: SiteJudgement, Budget: JudgementBudget, PromptCeiling: JudgementPromptCeiling, Bound: JudgementBound(clientBound, runBound)},
	}
	if fillBudget > 0 {
		sites = append(sites, CallSite{Name: SiteFill, Budget: fillBudget, PromptCeiling: MaxFillContentBytes, Bound: FillBound, Deferred: true})
	}
	return sites
}

// Afford reports every site's headroom at floors: what its budget and its prompt cost at the declared rates, against the share of its bound the safety factor leaves.
func Afford(sites []CallSite, floors Floors) []Headroom {
	resolved := floors.Resolved()

	report := make([]Headroom, 0, len(sites))
	for _, site := range sites {
		generation := rateSeconds(float64(site.Budget), resolved.TokensPerSecond)
		prompt := rateSeconds(float64(site.PromptCeiling), resolved.PromptBytesPerSecond)
		allowed := time.Duration(float64(site.Bound) * AffordabilitySafety)

		report = append(report, Headroom{
			CallSite:                     site,
			Generation:                   generation,
			Prompt:                       prompt,
			Allowed:                      allowed,
			Spare:                        allowed - generation - prompt,
			RequiredTokensPerSecond:      requiredRate(float64(site.Budget), allowed-prompt),
			RequiredPromptBytesPerSecond: requiredRate(float64(site.PromptCeiling), allowed-generation),
		})
	}
	return report
}

// JudgementCost is what one judgement call is declared to need at floors: its output budget at the generation floor, plus its prompt ceiling at the prompt floor.
func JudgementCost(floors Floors) time.Duration {
	resolved := floors.Resolved()
	return rateSeconds(float64(JudgementBudget), resolved.TokensPerSecond) + rateSeconds(float64(JudgementPromptCeiling), resolved.PromptBytesPerSecond)
}

func rateSeconds(quantity, rate float64) time.Duration {
	return time.Duration(quantity / rate * float64(time.Second))
}

func requiredRate(quantity float64, within time.Duration) float64 {
	if within <= 0 {
		return 0
	}
	return quantity / within.Seconds()
}
