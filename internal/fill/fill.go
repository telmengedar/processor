// Package fill implements loop.FillPort over internal/condense, creating a missing substance in the turn or after it.
package fill

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/loop"
)

var _ loop.FillPort = (*Port)(nil)

// Port implements loop.FillPort over internal/condense, one node at a time; deferred nodes wait in its queue until Start runs the worker.
type Port struct {
	Graph condense.GraphPort
	Model condense.ModelPort
	Now   func() time.Time
	Log   *slog.Logger

	mu     sync.Mutex
	queued []int64
	wake   chan struct{}
}

// New builds a Port over graph and model, defaulting now to time.Now and logger to a discarding logger when nil.
func New(graph condense.GraphPort, model condense.ModelPort, now func() time.Time, logger *slog.Logger) *Port {
	if now == nil {
		now = time.Now
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Port{Graph: graph, Model: model, Now: now, Log: logger, wake: make(chan struct{}, 1)}
}

// Fill condenses id's content into a substance and stores it, or reports why it did not; a substance already present is returned unwritten.
func (p *Port) Fill(ctx context.Context, id int64) (loop.FillResult, error) {
	out := condense.Condense(ctx, p.Graph, p.Model, condense.Target{Node: id}, condense.Options{}, p.Now)

	if out.Provenance != nil && out.Provenance.Written {
		return loop.FillResult{Written: true, Substance: out.Audit.Substance, Model: out.Provenance.Model}, nil
	}
	if present, ok := out.Present(); ok {
		return loop.FillResult{Substance: present, Reason: skipReason(*out.Skip)}, nil
	}
	if out.Skip != nil {
		return loop.FillResult{Reason: skipReason(*out.Skip)}, nil
	}
	return loop.FillResult{Reason: "fill produced no outcome"}, nil
}

// Defer queues id for the worker unless it is already queued; it never blocks.
func (p *Port) Defer(id int64) {
	p.mu.Lock()
	if !slices.Contains(p.queued, id) {
		p.queued = append(p.queued, id)
	}
	p.mu.Unlock()

	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Start runs the worker that condenses queued nodes one at a time until ctx ends; a Port never started only records what is deferred.
func (p *Port) Start(ctx context.Context) {
	go p.work(ctx)
}

func (p *Port) work(ctx context.Context) {
	for {
		id, ok := p.next()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-p.wake:
			}
			continue
		}
		if ctx.Err() != nil {
			return
		}
		p.fillDeferred(ctx, id)
	}
}

func (p *Port) fillDeferred(ctx context.Context, id int64) {
	started := p.Now()
	result, _ := p.Fill(ctx, id)
	elapsed := p.Now().Sub(started)

	if result.Written {
		p.Log.Info("deferred substance written", "node", id, "model", result.Model, "elapsed", elapsed)
		return
	}
	p.Log.Warn("deferred substance not written", "node", id, "model", result.Model, "elapsed", elapsed, "reason", result.Reason)
}

func (p *Port) next() (int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queued) == 0 {
		return 0, false
	}
	id := p.queued[0]
	p.queued = p.queued[1:]
	return id, true
}

func skipReason(skip condense.Skip) string {
	if skip.Detail == "" {
		return skip.Reason
	}
	return skip.Reason + ": " + skip.Detail
}
