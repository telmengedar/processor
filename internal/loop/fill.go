package loop

import "context"

// FillPort is the seam between the loop and substance generation; declared here because internal/loop cannot import internal/condense, and nil refuses every call with a recorded reason.
type FillPort interface {
	Fill(ctx context.Context, id int64) (FillResult, error)
}

// FillResult is one condensation attempt's outcome.
type FillResult struct {
	Written bool
	Model   string
	Reason  string
}

// FillOutcome is one candidate's fill decision for a run: filled, or the reason it was not.
type FillOutcome struct {
	ID     int64  `json:"id"`
	Filled bool   `json:"filled"`
	Model  string `json:"model,omitempty"`
	Reason string `json:"reason,omitempty"`
}

const (
	// FillSizeFloor is the size below which a candidate is not worth a fill attempt.
	FillSizeFloor = 8_000

	// MaxFills is the transition-only ceiling on fills attempted in one turn.
	MaxFills = 2

	// MaxFillContentBytes refuses a fill attempt before it ever calls the model.
	MaxFillContentBytes = 100_000
)

const (
	fillReasonBelowSizeGate  = "below size floor"
	fillReasonNoPressure     = "no pressure"
	fillReasonPortAbsent     = "fill port absent"
	fillReasonCeilingReached = "per-turn ceiling reached"
	fillReasonOversized      = "oversized"
)

func (t *Turn) fill(ctx context.Context, dispositions []Disposition) []FillOutcome {
	outcomes := make([]FillOutcome, 0, len(dispositions))
	fired := 0

	for _, d := range dispositions {
		if d.SubstanceAvailable {
			continue
		}

		switch {
		case d.Size < FillSizeFloor:
			outcomes = append(outcomes, FillOutcome{ID: d.ID, Reason: fillReasonBelowSizeGate})
		case d.CutReason != cutReasonByteBudget:
			outcomes = append(outcomes, FillOutcome{ID: d.ID, Reason: fillReasonNoPressure})
		case t.Fill == nil:
			outcomes = append(outcomes, FillOutcome{ID: d.ID, Reason: fillReasonPortAbsent})
		case fired >= MaxFills:
			outcomes = append(outcomes, FillOutcome{ID: d.ID, Reason: fillReasonCeilingReached})
		case d.Size > MaxFillContentBytes:
			outcomes = append(outcomes, FillOutcome{ID: d.ID, Reason: fillReasonOversized})
		default:
			fired++
			outcomes = append(outcomes, t.attemptFill(ctx, d.ID))
		}
	}

	return outcomes
}

func (t *Turn) attemptFill(ctx context.Context, id int64) FillOutcome {
	result, err := t.Fill.Fill(ctx, id)
	if err != nil {
		t.log().Error("fill failed", "id", id, "error", err)
		return FillOutcome{ID: id, Reason: BoundCause(err.Error())}
	}
	if result.Written {
		t.log().Info("fill wrote a substance", "id", id, "model", result.Model)
		return FillOutcome{ID: id, Filled: true, Model: result.Model}
	}
	return FillOutcome{ID: id, Reason: result.Reason}
}
