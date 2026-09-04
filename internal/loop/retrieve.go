package loop

import "context"

func Retrieve(ctx context.Context, graph GraphPort, query string, limit int) ([]Candidate, error) {
	return graph.Recall(ctx, query, limit)
}
