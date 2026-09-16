package loop

import (
	"cmp"
	"context"
	"slices"
)

const (
	fusionRankConstant    = 10
	recallOverfetchFactor = 5
)

// Retrieve asks the graph for more rows than it returns, fusing them into at most limit candidates, never the anchor and never a row this system wrote; window bounds every per-query recall and never the scoped one.
func Retrieve(ctx context.Context, graph GraphPort, anchor Anchor, queries []string, limit, reserve int, window UpdateWindow) ([]Candidate, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	fetch := limit * recallOverfetchFactor

	lists := make([][]Candidate, 0, len(queries))
	for _, query := range queries {
		list, err := graph.Recall(ctx, query, fetch, nil, window)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}

	scope, err := RecallScope(ctx, graph, anchor.ID)
	if err != nil {
		return nil, err
	}

	scoped, err := graph.Recall(ctx, queries[0], fetch, scope, UpdateWindow{})
	if err != nil {
		return nil, err
	}

	return attribute(fuse(lists, scoped, anchor.ID, limit, reserve), sourcesOf(lists, scoped)), nil
}

func sourcesOf(lists [][]Candidate, scoped []Candidate) map[int64][]Source {
	sources := make(map[int64][]Source)

	for query, list := range lists {
		for rank, candidate := range list {
			sources[candidate.ID] = append(sources[candidate.ID], Source{Query: query, Rank: rank + 1})
		}
	}

	for rank, candidate := range scoped {
		sources[candidate.ID] = append(sources[candidate.ID], Source{Query: 0, Scoped: true, Rank: rank + 1})
	}

	return sources
}

func attribute(candidates []Candidate, sources map[int64][]Source) []Candidate {
	for i := range candidates {
		candidates[i].Sources = sources[candidates[i].ID]
	}
	return candidates
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

func fuse(lists [][]Candidate, scoped []Candidate, anchor int64, limit, reserve int) []Candidate {
	fused := fuseByReciprocalRank(lists)
	scoped = slices.Clone(scoped)
	reconcileScopedSimilarity(fused, scoped)
	reserve = min(max(reserve, 0), limit)

	taken := map[int64]bool{anchor: true}
	out := make([]Candidate, 0, limit)

	appendUnseen := func(candidate Candidate) bool {
		if taken[candidate.ID] || candidate.SelfProduced {
			return false
		}
		taken[candidate.ID] = true
		out = append(out, candidate)
		return true
	}

	for _, candidate := range fused {
		if len(out) == limit-reserve {
			break
		}
		appendUnseen(candidate)
	}

	reserved := 0
	for _, candidate := range scoped {
		if reserved == reserve || len(out) == limit {
			break
		}
		if appendUnseen(candidate) {
			reserved++
		}
	}

	for _, candidate := range fused {
		if len(out) == limit {
			break
		}
		appendUnseen(candidate)
	}

	return out
}

func fuseByReciprocalRank(lists [][]Candidate) []Candidate {
	scores := make(map[int64]float64)
	seen := make(map[int64]int)
	order := make([]Candidate, 0)

	for _, list := range lists {
		for rank, candidate := range list {
			scores[candidate.ID] += 1 / float64(fusionRankConstant+rank+1)
			if idx, ok := seen[candidate.ID]; ok {
				order[idx].Similarity = max(order[idx].Similarity, candidate.Similarity)
				continue
			}
			seen[candidate.ID] = len(order)
			order = append(order, candidate)
		}
	}

	slices.SortStableFunc(order, func(a, b Candidate) int {
		return cmp.Compare(scores[b.ID], scores[a.ID])
	})

	return order
}

func reconcileScopedSimilarity(fused []Candidate, scoped []Candidate) {
	index := make(map[int64]int, len(fused))
	for i, c := range fused {
		index[c.ID] = i
	}

	for i, c := range scoped {
		idx, ok := index[c.ID]
		if !ok {
			continue
		}
		best := max(fused[idx].Similarity, c.Similarity)
		fused[idx].Similarity = best
		scoped[i].Similarity = best
	}
}
