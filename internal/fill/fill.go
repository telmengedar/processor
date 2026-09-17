// Package fill implements loop.FillPort as a second caller of internal/condense.Run.
package fill

import (
	"context"
	"time"

	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/loop"
)

var _ loop.FillPort = (*Port)(nil)

// Port implements loop.FillPort over internal/condense.Run, one node at a time.
type Port struct {
	Graph condense.GraphPort
	Model condense.ModelPort
	Now   func() time.Time
}

// New builds a Port over graph and model, defaulting now to time.Now when nil.
func New(graph condense.GraphPort, model condense.ModelPort, now func() time.Time) *Port {
	if now == nil {
		now = time.Now
	}
	return &Port{Graph: graph, Model: model, Now: now}
}

// Fill condenses id's content into a substance and stores it, or reports why it did not.
func (p *Port) Fill(ctx context.Context, id int64) (loop.FillResult, error) {
	result := condense.Run(ctx, p.Graph, p.Model, []condense.Target{{Node: id}}, condense.Options{}, p.Now)

	if len(result.Provenance) == 1 && result.Provenance[0].Written {
		return loop.FillResult{Written: true, Model: result.Provenance[0].Model}, nil
	}
	if len(result.Skipped) == 1 {
		return loop.FillResult{Reason: skipReason(result.Skipped[0])}, nil
	}
	return loop.FillResult{Reason: "fill produced no outcome"}, nil
}

func skipReason(skip condense.Skip) string {
	if skip.Detail == "" {
		return skip.Reason
	}
	return skip.Reason + ": " + skip.Detail
}
