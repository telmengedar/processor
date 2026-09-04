package loop

import (
	"cmp"
	"context"
	"slices"
)

const fusionRankConstant = 10

// Retrieve issues one unscoped recall per query plus one recall of the first
// query ranked inside the anchor's two-hop scope, and returns at most limit
// candidates: the reciprocal-rank fusion of the unscoped lists, with the last
// reserve slots of the cap held for the scoped list.
func Retrieve(ctx context.Context, graph GraphPort, anchor Anchor, queries []string, limit, reserve int) ([]Candidate, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	lists := make([][]Candidate, 0, len(queries))
	for _, query := range queries {
		list, err := graph.Recall(ctx, query, limit, nil)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}

	scope, err := RecallScope(ctx, graph, anchor.ID)
	if err != nil {
		return nil, err
	}

	scoped, err := graph.Recall(ctx, queries[0], limit, scope)
	if err != nil {
		return nil, err
	}

	return fuse(lists, scoped, limit, reserve), nil
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

func fuse(lists [][]Candidate, scoped []Candidate, limit, reserve int) []Candidate {
	fused := fuseByReciprocalRank(lists)
	reserve = min(max(reserve, 0), limit)

	taken := make(map[int64]bool, limit)
	out := make([]Candidate, 0, limit)

	appendUnseen := func(candidate Candidate) bool {
		if taken[candidate.ID] {
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
	seen := make(map[int64]bool)
	order := make([]Candidate, 0)

	for _, list := range lists {
		for rank, candidate := range list {
			scores[candidate.ID] += 1 / float64(fusionRankConstant+rank+1)
			if seen[candidate.ID] {
				continue
			}
			seen[candidate.ID] = true
			order = append(order, candidate)
		}
	}

	slices.SortStableFunc(order, func(a, b Candidate) int {
		return cmp.Compare(scores[b.ID], scores[a.ID])
	})

	return order
}
