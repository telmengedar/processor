package measure

import (
	"context"
	"log/slog"

	"github.com/telmengedar/processor/internal/loop"
)

// Result is one measured run: the record the shipped turn produced, the receipt saying no node holds it, and every mutation the run withheld from the graph.
type Result struct {
	loop.Record
	Written    loop.WriteReceipt     `json:"written"`
	Suppressed []SuppressedWrite     `json:"suppressed"`
	Substances []SuppressedSubstance `json:"substances"`
}

// Runner executes measured runs one at a time: one shipped turn over ports that read the graph and mutate nothing.
type Runner struct {
	graph      *Graph
	substances *CondenseGraph
	turn       *loop.Turn
}

// NewRunner builds a runner whose turn reads graph, fills through substances, judges model with system and modelID, and writes files through files.
func NewRunner(graph *Graph, substances *CondenseGraph, model loop.ModelPort, files loop.FilePort, fill loop.FillPort, system, modelID string, logger *slog.Logger) *Runner {
	turn := loop.NewTurn(graph, model, files, system, modelID, logger)
	turn.Fill = fill

	return &Runner{graph: graph, substances: substances, turn: turn}
}

// Run executes one measured turn for input against subject.
func (r *Runner) Run(ctx context.Context, input string, subject int64) (Result, error) {
	records, substances := len(r.graph.Suppressed()), len(r.Substances())

	record, receipt, err := r.turn.Run(ctx, input, subject)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Record:     record,
		Written:    receipt,
		Suppressed: r.graph.SuppressedSince(records),
		Substances: r.substancesSince(substances),
	}, nil
}

// Suppressed returns every record this runner's turns declined to file, in the order they were produced.
func (r *Runner) Suppressed() []SuppressedWrite {
	return r.graph.Suppressed()
}

// Substances returns every substance this runner's fills generated and declined to write, in the order they were produced.
func (r *Runner) Substances() []SuppressedSubstance {
	if r.substances == nil {
		return nil
	}
	return r.substances.Substances()
}

func (r *Runner) substancesSince(mark int) []SuppressedSubstance {
	if r.substances == nil {
		return nil
	}
	return r.substances.SubstancesSince(mark)
}
