package loop

import (
	"context"
	"sync"
	"time"
)

// FillPort is the seam between the loop and substance creation; declared here because internal/loop cannot import internal/condense, and nil refuses every call with a recorded reason.
type FillPort interface {
	// Fill attempts one creation for id and returns the text a row can render.
	Fill(ctx context.Context, id int64) (FillResult, error)

	// Defer queues id for creation after the turn; it never blocks and reports nothing.
	Defer(id int64)
}

// FillResult is one creation attempt's outcome.
type FillResult struct {
	// Written is true when this attempt stored a substance.
	Written bool

	// Substance is the text a row can render: the one this attempt produced, or the one already present.
	Substance string

	Model  string
	Reason string
}

// FillOutcome is one creation attempt's record: filled, deferred, or the reason it was not.
type FillOutcome struct {
	ID       int64         `json:"id"`
	Phase    string        `json:"phase,omitempty"`
	Filled   bool          `json:"filled"`
	Deferred bool          `json:"deferred,omitempty"`
	Model    string        `json:"model,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Elapsed  time.Duration `json:"elapsed,omitempty"`
}

// FillWait is how long one substance step waits for its creations before it cancels and defers those still running.
const FillWait = 90 * time.Second

const (
	phaseInitial = "initial"
	phaseRecall  = "recall"
	phaseRead    = "read"
)

const (
	fillReasonPortAbsent  = "fill port absent"
	fillReasonWaitExpired = "fill wait expired"
)

type fillAttempt struct {
	index   int
	result  FillResult
	err     error
	elapsed time.Duration
}

type prepaidKey struct{}

type prepaid struct {
	mu    sync.Mutex
	phase string
}

func withPrepaid(ctx context.Context) context.Context {
	return context.WithValue(ctx, prepaidKey{}, &prepaid{})
}

func prepaidFrom(ctx context.Context) *prepaid {
	grant, _ := ctx.Value(prepaidKey{}).(*prepaid)
	return grant
}

func (p *prepaid) set(phase string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.phase = phase
	p.mu.Unlock()
}

func (p *prepaid) spend() (string, bool) {
	if p == nil {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	phase := p.phase
	p.phase = ""
	return phase, phase != ""
}

func lacksSubstance(c Candidate) bool {
	return c.Substance == ""
}

func (t *Turn) fillWait(ctx context.Context, settings RunSettings) time.Duration {
	wait := settings.FillWait
	if deadline, ok := ctx.Deadline(); ok {
		wait = min(wait, deadline.Sub(t.now())-settings.JudgementCost(t.Floors))
	}
	return max(wait, 0)
}

func (t *Turn) createSubstances(ctx context.Context, settings RunSettings, phase string, ids []int64) (map[int64]string, []FillOutcome) {
	if len(ids) == 0 || settings.Form == FormContent {
		return nil, nil
	}

	outcomes := make([]FillOutcome, len(ids))
	if t.Fill == nil {
		for i, id := range ids {
			outcomes[i] = FillOutcome{ID: id, Phase: phase, Reason: fillReasonPortAbsent}
		}
		return nil, outcomes
	}

	wait := t.fillWait(ctx, settings)
	finished := make([]bool, len(ids))
	created := make(map[int64]string, len(ids))

	if wait > 0 {
		t.awaitFills(ctx, phase, ids, wait, outcomes, finished, created)
	}

	if wait > 0 && ctx.Err() == nil {
		prepaidFrom(ctx).set(phase)
	}

	reason := fillReasonWaitExpired
	if ctx.Err() != nil {
		reason = BoundCause(context.Cause(ctx).Error())
	}
	for i, id := range ids {
		if finished[i] {
			continue
		}
		t.log().Warn("fill still running at the wait: cancelled and deferred", "id", id, "phase", phase, "wait", wait)
		outcomes[i] = FillOutcome{ID: id, Phase: phase, Deferred: true, Reason: reason}
	}

	return created, outcomes
}

func (t *Turn) awaitFills(ctx context.Context, phase string, ids []int64, wait time.Duration, outcomes []FillOutcome, finished []bool, created map[int64]string) {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan fillAttempt, len(ids))
	for i, id := range ids {
		go func() {
			started := time.Now()
			result, err := t.Fill.Fill(stepCtx, id)
			done <- fillAttempt{index: i, result: result, err: err, elapsed: time.Since(started).Round(time.Millisecond)}
		}()
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	for pending := len(ids); pending > 0; pending-- {
		select {
		case attempt := <-done:
			finished[attempt.index] = true
			outcomes[attempt.index] = t.fillOutcome(ids[attempt.index], phase, attempt)
			if text := attempt.result.Substance; text != "" {
				created[ids[attempt.index]] = text
			}
		case <-timer.C:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (t *Turn) fillOutcome(id int64, phase string, attempt fillAttempt) FillOutcome {
	result := attempt.result
	if result.Written {
		t.log().Info("fill wrote a substance", "id", id, "phase", phase, "model", result.Model, "elapsed", attempt.elapsed)
		return FillOutcome{ID: id, Phase: phase, Filled: true, Model: result.Model, Elapsed: attempt.elapsed}
	}
	if attempt.err != nil {
		t.log().Error("fill failed", "id", id, "phase", phase, "elapsed", attempt.elapsed, "error", attempt.err)
		return FillOutcome{ID: id, Phase: phase, Reason: BoundCause(attempt.err.Error()), Elapsed: attempt.elapsed}
	}
	return FillOutcome{ID: id, Phase: phase, Reason: BoundCause(result.Reason), Elapsed: attempt.elapsed}
}

func (t *Turn) deferFills(fills []FillOutcome) {
	if t.Fill == nil {
		return
	}
	for _, f := range fills {
		if f.Deferred {
			t.Fill.Defer(f.ID)
		}
	}
}

func missingSubstances(candidates []Candidate, skip func(Candidate) bool) []int64 {
	var ids []int64
	for _, c := range candidates {
		if lacksSubstance(c) && (skip == nil || !skip(c)) {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

func withCreated(candidates []Candidate, created map[int64]string) []Candidate {
	if len(created) == 0 {
		return candidates
	}
	filled := make([]Candidate, len(candidates))
	for i, c := range candidates {
		filled[i] = c
		if text, ok := created[c.ID]; ok && c.Substance == "" {
			filled[i].Substance = text
			filled[i].Created = true
		}
	}
	return filled
}
