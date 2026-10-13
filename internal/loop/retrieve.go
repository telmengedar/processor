package loop

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
)

const logLeakedRows = "the graph returned rows the recall asked it to exclude"

// Retrieve returns at most limit candidates: the graph's top rows for all queries together, with reserve slots held for rows the subject's neighbourhood ranks that the global top lacks, ordered by similarity; window bounds the global recall only.
func Retrieve(ctx context.Context, graph GraphPort, anchor Anchor, queries []string, limit, reserve int, window UpdateWindow, log *slog.Logger) ([]Candidate, error) {
	if len(queries) == 0 {
		return nil, nil
	}
	if log == nil {
		log = slog.Default()
	}

	exclude := []int64{anchor.ID}

	global, err := graph.Recall(ctx, RecallRequest{Queries: queries, Limit: limit, Exclude: exclude, Window: window})
	if err != nil {
		return nil, err
	}

	scope, err := RecallScope(ctx, graph, anchor.ID)
	if err != nil {
		return nil, err
	}

	scoped, err := graph.Recall(ctx, RecallRequest{Queries: queries, Limit: limit, Scope: scope, Exclude: exclude})
	if err != nil {
		return nil, err
	}

	globalList, globalLeaked := usableRows(global, anchor.ID)
	scopedList, scopedLeaked := usableRows(scoped, anchor.ID)
	if leaked := globalLeaked + scopedLeaked; leaked > 0 {
		log.Warn(logLeakedRows, "subject", anchor.ID, "rows", leaked)
	}

	chosen := choose(globalList, scopedList, limit, reserve)
	globalRanks, scopedRanks := ranksOf(global), ranksOf(scoped)
	for i := range chosen {
		chosen[i].Sources = sourcesFor(chosen[i].ID, globalRanks, scopedRanks)
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

func sourcesFor(id int64, globalRanks, scopedRanks map[int64]int) []Source {
	var sources []Source
	if rank, ok := globalRanks[id]; ok {
		sources = append(sources, Source{Rank: rank})
	}
	if rank, ok := scopedRanks[id]; ok {
		sources = append(sources, Source{Rank: rank, Scoped: true})
	}
	return sources
}

func choose(global, scoped []Candidate, limit, reserve int) []Candidate {
	reserve = min(max(reserve, 0), limit)

	taken := make(map[int64]bool, limit)
	out := make([]Candidate, 0, limit)

	take := func(row Candidate) bool {
		if taken[row.ID] {
			return false
		}
		taken[row.ID] = true
		out = append(out, row)
		return true
	}

	reserved := 0
	inGlobalTop := make(map[int64]bool, len(global))
	for _, row := range global {
		inGlobalTop[row.ID] = true
	}

	for _, row := range scoped {
		if reserved == reserve {
			break
		}
		if inGlobalTop[row.ID] {
			continue
		}
		if take(row) {
			reserved++
		}
	}

	for _, row := range global {
		if len(out) == limit {
			break
		}
		take(row)
	}

	return out
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
