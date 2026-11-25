package loop

import "time"

const (
	// DerivationPromptCeiling declares the bytes charged for one derivation call: its instructions and exemplars, an input allowance, and the default selected conversation with its guidance.
	DerivationPromptCeiling = 12_000 + HistorySafetyLimit + len(derivationConversationGuidance) + len("\n\n\n\n")
	// JudgementPromptCeiling bounds the bytes one judgement call sends: the working memory, which is the assembled block plus one supplementary round and never more.
	JudgementPromptCeiling = AssemblyByteBudget + SupplementaryByteBudget
)

const noOutputLimit = 0

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
	// SiteAnswering names the reserved answering call site.
	SiteAnswering = "answering"
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

// CallSite is one model call the product can issue: the output budget it declares (0 for none), the prompt it may send, and the bound that will cut it.
type CallSite struct {
	Name          string
	Budget        int
	PromptCeiling int
	Bound         time.Duration
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

// JudgementBound is the bound that cuts one judgement call: the adapter's own client bound, or the run bound's per-call share once the derivation and one substance wait are spent, whichever is smaller.
func JudgementBound(clientBound, runBound time.Duration) time.Duration {
	return min(clientBound, (runBound-DerivationBound-FillWait)/MaxModelCalls)
}

// CallSites is every site a run issues a model call at.
func CallSites(clientBound, runBound time.Duration) []CallSite {
	judgement := JudgementBound(clientBound, runBound)
	return []CallSite{
		{Name: SiteDerivation, PromptCeiling: DerivationPromptCeiling, Bound: DerivationBound},
		{Name: SiteJudgement, PromptCeiling: JudgementPromptCeiling, Bound: judgement},
		{Name: SiteAnswering, PromptCeiling: JudgementPromptCeiling, Bound: judgement},
	}
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

// JudgementCost is what one judgement call at the default settings is declared to need at floors: its prompt ceiling at the prompt floor, as the default settings carry no output budget.
func JudgementCost(floors Floors) time.Duration {
	return DefaultSettings().JudgementCost(floors)
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
