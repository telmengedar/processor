package loop

import (
	"context"
	"slices"
)

func Retrieve(ctx context.Context, graph GraphPort, query string, limit int) ([]Candidate, error) {
	return graph.Recall(ctx, query, limit, nil)
}

// RecallScope is the subject together with its neighbours, so a recall ranked inside it reaches two hops.
func RecallScope(ctx context.Context, graph GraphPort, subject int64) ([]int64, error) {
	neighbours, err := graph.Neighbours(ctx, subject)
	if err != nil {
		return nil, err
	}

	scope := make([]int64, 0, len(neighbours)+1)
	scope = append(scope, subject)
	scope = append(scope, neighbours...)
	slices.Sort(scope)

	return slices.Compact(scope), nil
}
