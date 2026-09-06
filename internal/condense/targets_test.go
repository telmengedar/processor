package condense

import (
	"testing"

	"github.com/telmengedar/processor/internal/eval"
)

func corpusOf(rows ...eval.Row) eval.Corpus {
	return eval.Corpus{Rows: rows}
}

func TestEveryRequiredNodeBecomesOneTargetAscendingByIdWhateverOrderTheRowsAreIn(t *testing.T) {
	corpus := corpusOf(
		eval.Row{ID: "r02", Required: []eval.Required{{Node: 900, Why: "second"}}},
		eval.Row{ID: "r01", Required: []eval.Required{{Node: 100, Why: "first"}}},
	)

	targets := TargetsFromCorpus(corpus)

	if len(targets) != 2 {
		t.Fatalf("want two targets, got %d", len(targets))
	}
	if targets[0].Node != 100 || targets[1].Node != 900 {
		t.Fatalf("want targets ascending by node id, got %d then %d", targets[0].Node, targets[1].Node)
	}
}

func TestANodeRequiredByTwoRowsBecomesOneTargetCarryingBothPreRegisteredReasons(t *testing.T) {
	corpus := corpusOf(
		eval.Row{ID: "r01", Required: []eval.Required{{Node: 100, Why: "the first reason"}}},
		eval.Row{ID: "r02", Required: []eval.Required{{Node: 100, Why: "the second reason"}}},
	)

	targets := TargetsFromCorpus(corpus)

	if len(targets) != 1 {
		t.Fatalf("want the node deduplicated into one target, got %d", len(targets))
	}
	if len(targets[0].Why) != 2 || len(targets[0].Rows) != 2 {
		t.Fatalf("want both reasons and both row ids carried, got %+v", targets[0])
	}
	if targets[0].Rows[0] != "r01" || targets[0].Rows[1] != "r02" {
		t.Fatalf("want the row ids in corpus order, got %v", targets[0].Rows)
	}
}

func TestARowWithSeveralRequiredNodesProducesATargetForEachOfThem(t *testing.T) {
	corpus := corpusOf(eval.Row{ID: "r01", Required: []eval.Required{
		{Node: 300, Why: "a"}, {Node: 100, Why: "b"}, {Node: 200, Why: "c"},
	}})

	targets := TargetsFromCorpus(corpus)

	if len(targets) != 3 {
		t.Fatalf("want three targets, got %d", len(targets))
	}
	for i, want := range []int64{100, 200, 300} {
		if targets[i].Node != want {
			t.Fatalf("want target %d to be node %d, got %d", i, want, targets[i].Node)
		}
	}
}

func TestAnExplicitIdListIsDeduplicatedAndSortedAndCarriesNoPreRegisteredReason(t *testing.T) {
	targets := TargetsFromIDs([]int64{900, 100, 900})

	if len(targets) != 2 {
		t.Fatalf("want two targets after deduplication, got %d", len(targets))
	}
	if targets[0].Node != 100 || targets[1].Node != 900 {
		t.Fatalf("want ascending ids, got %d then %d", targets[0].Node, targets[1].Node)
	}
	if len(targets[0].Why) != 0 {
		t.Fatalf("an explicit id carries no pre-registered reason, got %v", targets[0].Why)
	}
}
