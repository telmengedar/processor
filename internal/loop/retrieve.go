package loop

import (
	"cmp"
	"context"
	"slices"
	"strings"
)

const fusionRankConstant = 10

// Retrieve returns at most limit candidates, never the anchor itself, and the queries it issued: the caller's plus one grounded in the anchor.
func Retrieve(ctx context.Context, graph GraphPort, anchor Anchor, queries []string, limit, reserve int) (Retrieval, error) {
	if len(queries) == 0 {
		return Retrieval{}, nil
	}

	issued := issuedQueries(anchor, queries)

	lists := make([][]Candidate, 0, len(issued))
	for _, query := range issued {
		list, err := graph.Recall(ctx, query, limit, nil)
		if err != nil {
			return Retrieval{}, err
		}
		lists = append(lists, list)
	}

	scope, err := RecallScope(ctx, graph, anchor.ID)
	if err != nil {
		return Retrieval{}, err
	}

	scoped, err := graph.Recall(ctx, issued[0], limit, scope)
	if err != nil {
		return Retrieval{}, err
	}

	candidates := fuse(lists, scoped, anchor.ID, limit, reserve)

	return Retrieval{Queries: issued, Candidates: attribute(candidates, sourcesOf(lists, scoped))}, nil
}

func issuedQueries(anchor Anchor, queries []string) []string {
	issued := slices.Clone(queries)

	grounded := groundedQuery(anchor, queries[0])
	if slices.Contains(issued, grounded) {
		return issued
	}

	return append(issued, grounded)
}

func groundedQuery(anchor Anchor, query string) string {
	identity := strings.TrimSpace(anchor.Name)
	if identity == "" {
		return query
	}
	return identity + "\n" + query
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
	reserve = min(max(reserve, 0), limit)

	taken := map[int64]bool{anchor: true}
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
