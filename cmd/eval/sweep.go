package main

import (
	"context"
	"fmt"
	"time"

	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
)

const rowErrorSubjectNotFound = "subject not found"

func sweep(ctx context.Context, graph loop.GraphPort, corpus eval.Corpus, sweptAt time.Time) (eval.Result, error) {
	result := eval.NewResult(corpus, sweptAt)

	for _, row := range corpus.Rows {
		rowResult, err := sweepRow(ctx, graph, row)
		if err != nil {
			return eval.Result{}, fmt.Errorf("row %s: %w", row.ID, err)
		}
		result.Rows = append(result.Rows, rowResult)
	}

	return result, nil
}

func sweepRow(ctx context.Context, graph loop.GraphPort, row eval.Row) (eval.RowResult, error) {
	dispositions, found, err := rowDispositions(ctx, graph, row)
	if err != nil {
		return eval.RowResult{}, err
	}
	if !found {
		return eval.RowResult{Row: row.ID, Stratum: row.Stratum, Subject: row.Subject, Error: rowErrorSubjectNotFound}, nil
	}

	result := eval.BuildRow(row, dispositions)

	if err := resolveMisses(ctx, graph, result.Required); err != nil {
		return eval.RowResult{}, err
	}

	return result, nil
}

func rowDispositions(ctx context.Context, graph loop.GraphPort, row eval.Row) ([]loop.Disposition, bool, error) {
	anchor, found, err := graph.Node(ctx, row.Subject)
	if err != nil || !found {
		return nil, found, err
	}

	candidates, err := graph.Recall(ctx, row.Input, loop.CandidateLimit)
	if err != nil {
		return nil, true, err
	}

	_, dispositions := loop.Assemble(anchor, candidates, loop.AssemblyByteBudget)
	return dispositions, true, nil
}

func resolveMisses(ctx context.Context, graph loop.GraphPort, required []eval.NodeResult) error {
	for i, node := range required {
		if node.Verdict != eval.NotRetrieved {
			continue
		}

		_, found, err := graph.Node(ctx, node.Node)
		if err != nil {
			return err
		}
		if !found {
			required[i].Verdict = eval.Unresolved
		}
	}
	return nil
}
