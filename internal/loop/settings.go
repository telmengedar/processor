package loop

import "time"

// RunSettings is the tuning one run is held to; start from DefaultSettings, because the zero value budgets nothing.
type RunSettings struct {
	CandidateLimit          int
	AssemblyByteBudget      int
	SupplementaryByteBudget int
	MaxModelCalls           int
	JudgementBudget         int
	AnsweringBudget         int
	FillWait                time.Duration
	HistoryExchangeLimit    int
	HistorySafetyLimit      int
}

// DefaultSettings returns the settings every run has had since the loop shipped: the package constants, field for field.
func DefaultSettings() RunSettings {
	return RunSettings{
		CandidateLimit:          CandidateLimit,
		AssemblyByteBudget:      AssemblyByteBudget,
		SupplementaryByteBudget: SupplementaryByteBudget,
		MaxModelCalls:           MaxModelCalls,
		JudgementBudget:         JudgementBudget,
		AnsweringBudget:         AnsweringBudget,
		FillWait:                FillWait,
		HistoryExchangeLimit:    HistoryExchangeLimit,
		HistorySafetyLimit:      HistorySafetyLimit,
	}
}

// PromptCeiling bounds the bytes one judgement call of this run sends: the assembled block plus one supplementary round.
func (s RunSettings) PromptCeiling() int {
	return s.AssemblyByteBudget + s.SupplementaryByteBudget
}

// JudgementCost is what one judgement call of this run is declared to need at floors.
func (s RunSettings) JudgementCost(floors Floors) time.Duration {
	resolved := floors.Resolved()
	return rateSeconds(float64(s.JudgementBudget), resolved.TokensPerSecond) + rateSeconds(float64(s.PromptCeiling()), resolved.PromptBytesPerSecond)
}

// Limits is what a run's record states governed it: these settings plus the constants no run can change.
func (s RunSettings) Limits() Limits {
	return Limits{
		CandidateLimit:          s.CandidateLimit,
		AssemblyByteBudget:      s.AssemblyByteBudget,
		SupplementaryByteBudget: s.SupplementaryByteBudget,
		MaxModelCalls:           s.MaxModelCalls,
		DerivationBudget:        DerivationBudget,
		JudgementBudget:         s.JudgementBudget,
		AnsweringBudget:         s.AnsweringBudget,
		ThinKnowledgeSimilarity: ThinKnowledgeSimilarity,
		FillWait:                s.FillWait,
		HistoryExchangeLimit:    s.HistoryExchangeLimit,
		HistorySafetyLimit:      s.HistorySafetyLimit,
	}
}

// RunOptions is everything one run is given.
type RunOptions struct {
	Input    string
	Subject  int64
	History  []Exchange
	Settings RunSettings

	// View is the config the run is held to, nil for a run that has none.
	View *View
}

func (o RunOptions) filter() Filter {
	if o.View == nil {
		return nil
	}
	return o.View.Filter
}

func (o RunOptions) identity() string {
	if o.View == nil {
		return ""
	}
	return o.View.Identity
}
