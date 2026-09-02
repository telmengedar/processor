package eval

import (
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func threeDispositions() []loop.Disposition {
	return []loop.Disposition{
		{Rank: 1, ID: 101, Similarity: 0.81, Size: 100, ContentHash: "hash-of-101", Included: true},
		{Rank: 2, ID: 202, Similarity: 0.79, Size: 200, ContentHash: "hash-of-202", Included: true},
		{Rank: 3, ID: 303, Similarity: 0.77, Size: 300, ContentHash: "hash-of-303", CutReason: "byte budget exceeded"},
	}
}

func TestScoreClassifiesARequiredNodeAdmittedIntoTheBlock(t *testing.T) {
	t.Parallel()

	got := Score([]Required{{Node: 101, Hash: "hash-of-101", Why: "w"}}, threeDispositions())

	if len(got) != 1 {
		t.Fatalf("len(Score) = %d, want 1", len(got))
	}
	if got[0].Verdict != Admitted {
		t.Fatalf("Verdict = %q, want %q", got[0].Verdict, Admitted)
	}
	if got[0].Rank != 1 {
		t.Fatalf("Rank = %d, want 1", got[0].Rank)
	}
	if got[0].Stale {
		t.Fatal("Stale is true for a node whose live hash matches its label, want false")
	}
}

func TestScoreClassifiesARequiredNodeCutByTheBudgetAndKeepsItsRank(t *testing.T) {
	t.Parallel()

	got := Score([]Required{{Node: 303, Hash: "hash-of-303", Why: "w"}}, threeDispositions())

	if got[0].Verdict != Cut {
		t.Fatalf("Verdict = %q, want %q", got[0].Verdict, Cut)
	}
	if got[0].Rank != 3 {
		t.Fatalf("Rank = %d, want 3", got[0].Rank)
	}
}

func TestScoreClassifiesARequiredNodeAbsentFromEveryCandidateRow(t *testing.T) {
	t.Parallel()

	got := Score([]Required{{Node: 909, Hash: "hash-of-909", Why: "w"}}, threeDispositions())

	if got[0].Verdict != NotRetrieved {
		t.Fatalf("Verdict = %q, want %q", got[0].Verdict, NotRetrieved)
	}
	if got[0].Rank != 0 {
		t.Fatalf("Rank = %d, want 0 for a node that has no rank", got[0].Rank)
	}
	if got[0].Stale {
		t.Fatal("Stale is true for a node that was never retrieved, want false")
	}
}

func TestScoreDistinguishesCutFromNotRetrievedOnOtherwiseIdenticalRows(t *testing.T) {
	t.Parallel()

	required := []Required{
		{Node: 303, Hash: "hash-of-303", Why: "w"},
		{Node: 909, Hash: "hash-of-909", Why: "w"},
	}

	got := Score(required, threeDispositions())

	if got[0].Verdict == got[1].Verdict {
		t.Fatalf("both misses scored %q, want cut and notRetrieved to stay distinct", got[0].Verdict)
	}
	if got[0].Verdict != Cut {
		t.Fatalf("Verdict of the retrieved-but-cut node = %q, want %q", got[0].Verdict, Cut)
	}
	if got[1].Verdict != NotRetrieved {
		t.Fatalf("Verdict of the never-retrieved node = %q, want %q", got[1].Verdict, NotRetrieved)
	}
}

func TestScoreFlagsARequiredNodeWhoseLiveHashHasMovedUnderItsLabel(t *testing.T) {
	t.Parallel()

	got := Score([]Required{{Node: 101, Hash: "the-hash-at-labelling-time", Why: "w"}}, threeDispositions())

	if got[0].Verdict != Admitted {
		t.Fatalf("Verdict = %q, want the row still scored as %q", got[0].Verdict, Admitted)
	}
	if !got[0].Stale {
		t.Fatal("Stale is false for a node whose live hash differs from its label, want true")
	}
}

func TestScoreTakesTheBestRankedOccurrenceOfARequiredNodeThatAppearsTwice(t *testing.T) {
	t.Parallel()

	dispositions := []loop.Disposition{
		{Rank: 1, ID: 101, ContentHash: "hash-of-101", Included: true},
		{Rank: 2, ID: 101, ContentHash: "hash-of-101", CutReason: "byte budget exceeded"},
	}

	got := Score([]Required{{Node: 101, Hash: "hash-of-101", Why: "w"}}, dispositions)

	if got[0].Verdict != Admitted {
		t.Fatalf("Verdict = %q, want %q from the better-ranked occurrence", got[0].Verdict, Admitted)
	}
	if got[0].Rank != 1 {
		t.Fatalf("Rank = %d, want 1", got[0].Rank)
	}
}

func TestScoreReturnsOneVerdictPerRequiredNodeInTheOrderTheyWereLabelled(t *testing.T) {
	t.Parallel()

	required := []Required{
		{Node: 303, Hash: "hash-of-303", Why: "w"},
		{Node: 101, Hash: "hash-of-101", Why: "w"},
		{Node: 909, Hash: "hash-of-909", Why: "w"},
	}

	got := Score(required, threeDispositions())

	if len(got) != 3 {
		t.Fatalf("len(Score) = %d, want 3", len(got))
	}
	for i, want := range []int64{303, 101, 909} {
		if got[i].Node != want {
			t.Fatalf("got[%d].Node = %d, want %d", i, got[i].Node, want)
		}
	}
}
