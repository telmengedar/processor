package condense

import (
	"sort"

	"github.com/telmengedar/processor/internal/eval"
)

// TargetsFromCorpus resolves the corpus's required nodes into one target each, ascending by id, carrying every pre-registered reason its rows attached.
func TargetsFromCorpus(corpus eval.Corpus) []Target {
	byNode := make(map[int64]*Target, len(corpus.Rows))
	ids := make([]int64, 0, len(corpus.Rows))

	for _, row := range corpus.Rows {
		for _, required := range row.Required {
			target, seen := byNode[required.Node]
			if !seen {
				target = &Target{Node: required.Node}
				byNode[required.Node] = target
				ids = append(ids, required.Node)
			}
			target.Rows = append(target.Rows, row.ID)
			target.Why = append(target.Why, required.Why)
		}
	}

	return collect(byNode, ids)
}

// TargetsFromIDs resolves an explicit id list into targets, deduplicated and ascending, with no pre-registered reason attached.
func TargetsFromIDs(ids []int64) []Target {
	byNode := make(map[int64]*Target, len(ids))
	order := make([]int64, 0, len(ids))

	for _, id := range ids {
		if _, seen := byNode[id]; seen {
			continue
		}
		byNode[id] = &Target{Node: id}
		order = append(order, id)
	}

	return collect(byNode, order)
}

func collect(byNode map[int64]*Target, ids []int64) []Target {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	targets := make([]Target, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, *byNode[id])
	}
	return targets
}
