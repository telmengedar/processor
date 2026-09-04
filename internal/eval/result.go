package eval

import (
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

// Limits are the two loop constants a sweep is governed by.
type Limits struct {
	CandidateLimit     int `json:"candidateLimit"`
	AssemblyByteBudget int `json:"assemblyByteBudget"`
}

// RowResult is one row's outcome, the diagnostics that make its number actionable,
// and the candidate set they were derived from.
type RowResult struct {
	Row     string `json:"row"`
	Stratum string `json:"stratum"`
	Subject int64  `json:"subject"`

	CandidateCount int `json:"candidateCount"`
	AdmittedCount  int `json:"admittedCount"`
	AdmittedBytes  int `json:"admittedBytes"`
	BudgetBytes    int `json:"budgetBytes"`

	AnchorWasCandidate        bool    `json:"anchorWasCandidate"`
	AnchorAdmittedAsCandidate bool    `json:"anchorAdmittedAsCandidate"`
	SelfProducedCandidates    int     `json:"selfProducedCandidates"`
	Shutout                   bool    `json:"shutout"`
	TopSimilarity             float64 `json:"topSimilarity"`

	Required   []NodeResult       `json:"required"`
	Candidates []loop.Disposition `json:"candidates"`
	Error      string             `json:"error,omitempty"`
}

// Result is one sweep: what produced it, and one entry per corpus row in corpus order.
type Result struct {
	CorpusHash string      `json:"corpusHash"`
	SweptAt    time.Time   `json:"sweptAt"`
	Limits     Limits      `json:"limits"`
	RowCount   int         `json:"rowCount"`
	Rows       []RowResult `json:"rows"`
}

// NewResult opens a result for corpus, carrying its hash and the loop limits a sweep runs under.
func NewResult(corpus Corpus, sweptAt time.Time) Result {
	return Result{
		CorpusHash: corpus.Hash,
		SweptAt:    sweptAt,
		Limits:     Limits{CandidateLimit: loop.CandidateLimit, AssemblyByteBudget: loop.AssemblyByteBudget},
		RowCount:   len(corpus.Rows),
		Rows:       make([]RowResult, 0, len(corpus.Rows)),
	}
}

// BuildRow turns the dispositions one row produced into that row's result entry.
func BuildRow(row Row, dispositions []loop.Disposition) RowResult {
	result := RowResult{
		Row:            row.ID,
		Stratum:        row.Stratum,
		Subject:        row.Subject,
		CandidateCount: len(dispositions),
		Candidates:     dispositions,
		BudgetBytes:    loop.AssemblyByteBudget,
		Required:       Score(row.Required, dispositions),
	}

	for _, d := range dispositions {
		if d.Included {
			result.AdmittedCount++
			result.AdmittedBytes += d.Size
		}
		if d.ID == row.Subject {
			result.AnchorWasCandidate = true
			result.AnchorAdmittedAsCandidate = result.AnchorAdmittedAsCandidate || d.Included
		}
		if selfProduced(d) {
			result.SelfProducedCandidates++
		}
		if d.Similarity > result.TopSimilarity {
			result.TopSimilarity = d.Similarity
		}
	}

	result.Shutout = result.CandidateCount > 0 && result.AdmittedCount == 0
	return result
}

// Scored reports whether the row contributes to the two rates.
func (r RowResult) Scored() bool {
	if r.Error != "" {
		return false
	}
	for _, node := range r.Required {
		if node.Verdict == Unresolved {
			return false
		}
	}
	return true
}

// ControlVerifiedRetrieval reports whether a control stratum was present, scored, and retrieved in full.
func (r Result) ControlVerifiedRetrieval() bool {
	controls := 0
	for _, row := range r.Rows {
		if row.Stratum != StratumControl {
			continue
		}
		controls++
		if !row.Scored() {
			return false
		}
		for _, node := range row.Required {
			if node.Verdict == NotRetrieved {
				return false
			}
		}
	}
	return controls > 0
}

func (r Result) controlWasCut() bool {
	for _, row := range r.Rows {
		if row.Stratum != StratumControl {
			continue
		}
		for _, node := range row.Required {
			if node.Verdict == Cut {
				return true
			}
		}
	}
	return false
}

func selfProduced(d loop.Disposition) bool {
	return divoid.IsRunRecord(d.Type, d.Name)
}
