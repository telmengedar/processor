package loop

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
)

const logLeakedRows = "the graph returned rows the recall asked it to exclude"

// Retrieve returns at most limit candidates: the graph's top rows for all queries together, ordered by similarity; window bounds the recall.
func Retrieve(ctx context.Context, graph GraphPort, anchor Anchor, queries []string, limit int, window UpdateWindow, log *slog.Logger) ([]Candidate, error) {
	if len(queries) == 0 {
		return nil, nil
	}
	if log == nil {
		log = slog.Default()
	}

	rows, err := graph.Recall(ctx, RecallRequest{Queries: queries, Limit: limit, Exclude: []int64{anchor.ID}, Window: window})
	if err != nil {
		return nil, err
	}

	chosen, leaked := usableRows(rows, anchor.ID)
	if leaked > 0 {
		log.Warn(logLeakedRows, "subject", anchor.ID, "rows", leaked)
	}

	ranks := ranksOf(rows)
	for i := range chosen {
		chosen[i].Sources = []Source{{Rank: ranks[chosen[i].ID]}}
	}
	slices.SortStableFunc(chosen, func(a, b Candidate) int {
		if c := cmp.Compare(b.Similarity, a.Similarity); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})

	return chosen, nil
}

func usableRows(rows []Candidate, anchor int64) (usable []Candidate, dropped int) {
	usable = make([]Candidate, 0, len(rows))
	for _, row := range rows {
		if row.ID == anchor || row.SelfProduced {
			dropped++
			continue
		}
		usable = append(usable, row)
	}
	return usable, dropped
}

func ranksOf(rows []Candidate) map[int64]int {
	ranks := make(map[int64]int, len(rows))
	for position, row := range rows {
		ranks[row.ID] = position + 1
	}
	return ranks
}
