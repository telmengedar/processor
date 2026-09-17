package loop

import (
	"strings"
	"testing"
)

func TestAdmissionRefusesACandidateWhoseRenderedPayloadExceedsTheCap(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 12_001)},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0)

	if dispositions[0].Included {
		t.Fatalf("a 12001-byte payload was admitted against a 60000-byte block budget, want it refused: the per-candidate ceiling is one fifth of the budget, so 12000 is the largest payload that may be carried")
	}
	if dispositions[0].CutReason != "oversized" {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, "oversized")
	}
}

func TestAdmissionCarriesACandidateExactlyAtTheCapAndTheLargestOneBelowIt(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}

	for _, size := range []int{11_999, 12_000} {
		candidates := []Candidate{{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", size)}}

		_, dispositions := Assemble(anchor, candidates, 60_000, 0)

		if !dispositions[0].Included {
			t.Fatalf("a %d-byte payload was refused against a 60000-byte block budget, want it carried: the ceiling is 12000 and admission at it is inclusive, so a tighter ceiling loses a row the budget can afford", size)
		}
		if dispositions[0].CutReason != "" {
			t.Fatalf("CutReason = %q for a %d-byte payload against a 60000-byte block budget, want it empty", dispositions[0].CutReason, size)
		}
	}
}

func TestARowRefusedForItsSizeChargesNothingToTheRunningByteTotal(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.99, Content: strings.Repeat("o", 12_001)},
		{ID: 20, Similarity: 0.98, Content: strings.Repeat("a", 11_000)},
		{ID: 30, Similarity: 0.97, Content: strings.Repeat("b", 11_000)},
		{ID: 40, Similarity: 0.96, Content: strings.Repeat("c", 11_000)},
		{ID: 50, Similarity: 0.95, Content: strings.Repeat("d", 11_000)},
		{ID: 60, Similarity: 0.94, Content: strings.Repeat("e", 11_000)},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0)

	if dispositions[0].Included {
		t.Fatalf("the 12001-byte row was admitted, want it refused for its size")
	}
	for i := 1; i < len(dispositions); i++ {
		if !dispositions[i].Included {
			t.Fatalf("dispositions[%d] (id %d) was cut: five 11000-byte rows total 55000 and fit a 60000-byte budget only if the row refused for its size was never charged", i, dispositions[i].ID)
		}
	}
}

func TestARowBothBelowTheFloorAndOverTheCapReportsTheFloor(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.2, Content: strings.Repeat("x", 40_000)},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0.63)

	if dispositions[0].CutReason != "below relevance floor" {
		t.Fatalf("CutReason = %q, want %q: a row that is both irrelevant and too large is cut for its relevance, because the size rule is asked only about rows that cleared the floor", dispositions[0].CutReason, "below relevance floor")
	}
}

func TestARowRefusedForItsSizeKeepsItsScoreItsSizeAndItsRankInTheRecord(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := []Candidate{
		{ID: 10, Type: "documentation", Name: "Big", Similarity: 0.812345, Content: strings.Repeat("x", 43_273)},
		{ID: 20, Type: "task", Name: "Small", Similarity: 0.71, Content: "small body"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0)

	if len(dispositions) != 2 {
		t.Fatalf("dispositions has %d entries, want 2: a row refused for its size stays in the record, or a reader cannot tell a refusal from a retrieval miss", len(dispositions))
	}

	got := dispositions[0]
	if got.Rank != 1 || got.ID != 10 || got.Name != "Big" || got.Similarity != 0.812345 || got.Size != 43_273 {
		t.Fatalf("the row refused for its size is recorded as %+v, want rank 1, id 10, name Big, similarity 0.812345 and size 43273", got)
	}
	if got.Included || got.CutReason != "oversized" {
		t.Fatalf("the row refused for its size is recorded as %+v, want it cut with reason %q", got, "oversized")
	}
}

func TestTheReasonForARowRefusedForItsSizeIsNotTheByteBudgetReason(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.99, Content: strings.Repeat("o", 12_001)},
		{ID: 20, Similarity: 0.98, Content: strings.Repeat("a", 11_900)},
		{ID: 30, Similarity: 0.97, Content: strings.Repeat("b", 11_900)},
		{ID: 40, Similarity: 0.96, Content: strings.Repeat("c", 11_900)},
		{ID: 50, Similarity: 0.95, Content: strings.Repeat("d", 11_900)},
		{ID: 60, Similarity: 0.94, Content: strings.Repeat("e", 11_900)},
		{ID: 70, Similarity: 0.93, Content: strings.Repeat("f", 11_900)},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0)

	if dispositions[0].CutReason != "oversized" {
		t.Fatalf("dispositions[0].CutReason = %q, want %q: the row exceeded the per-candidate ceiling on its own, before any budget was spent", dispositions[0].CutReason, "oversized")
	}
	if dispositions[6].CutReason != "byte budget exceeded" {
		t.Fatalf("dispositions[6].CutReason = %q, want %q: the row is small enough to carry and arrived after the budget was spent", dispositions[6].CutReason, "byte budget exceeded")
	}
	if dispositions[0].CutReason == dispositions[6].CutReason {
		t.Fatalf("both rows report %q: a reader cannot tell a document refused for its own size from one refused because the rows ahead of it spent the budget, and the two want different remedies", dispositions[0].CutReason)
	}
}

func TestTheCapIsAFractionOfTheBudgetAdmissionIsHandedRatherThanASecondConstant(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 5_000)}}

	_, block := admit(candidates, 60_000, 0, 0)
	if !block[0].Included || block[0].PayloadCap != 12_000 {
		t.Fatalf("against a 60000-byte budget the row is recorded as %+v, want it admitted under a ceiling of 12000", block[0])
	}

	_, supplementary := admit(candidates, 20_000, 0, 0)
	if supplementary[0].Included || supplementary[0].PayloadCap != 4_000 {
		t.Fatalf("against a 20000-byte budget the same row is recorded as %+v, want it refused under a ceiling of 4000: the ceiling is derived from the budget admission is handed, so a smaller budget carries a smaller ceiling", supplementary[0])
	}
	if supplementary[0].CutReason != "oversized" {
		t.Fatalf("CutReason = %q, want %q", supplementary[0].CutReason, "oversized")
	}
}

func TestTheCapIsAFractionOfTheBlockBudgetNotOfTheRoomLeftAfterTheAnchor(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a", Content: strings.Repeat("a", 30_000)}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 11_000)},
		{ID: 20, Similarity: 0.8, Content: strings.Repeat("y", 11_000)},
		{ID: 30, Similarity: 0.7, Content: strings.Repeat("z", 11_000)},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0)

	for _, i := range []int{0, 1} {
		if !dispositions[i].Included {
			t.Fatalf("dispositions[%d] (id %d) is %+v, want it admitted: an 11000-byte row is under a 12000-byte ceiling whatever the anchor happens to weigh, and a ceiling that shrank with the anchor would make a document's admissibility a property of today's subject", i, dispositions[i].ID, dispositions[i])
		}
		if dispositions[i].PayloadCap != 12_000 {
			t.Fatalf("dispositions[%d] (id %d) ran under a ceiling of %d, want 12000", i, dispositions[i].ID, dispositions[i].PayloadCap)
		}
	}
	if dispositions[2].Included || dispositions[2].CutReason != "byte budget exceeded" {
		t.Fatalf("dispositions[2] (id 30) is %+v, want it cut for the byte budget: the ceiling is a fraction of the block budget while the fill spends only the room left after the anchor", dispositions[2])
	}
}

func TestEveryRowStatesTheCapItWasJudgedUnder(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.9, Content: "admitted body"},
		{ID: 20, Similarity: 0.9, Content: strings.Repeat("x", 12_001)},
		{ID: 30, Similarity: 0.1, Content: "below the floor"},
		{ID: 40, Similarity: 0.9, Content: "self produced body", SelfProduced: true},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0.63)

	for i, d := range dispositions {
		if d.PayloadCap != 12_000 {
			t.Fatalf("dispositions[%d] (id %d) states a ceiling of %d, want 12000: a stored record answers what a different ceiling would have done only if every row names the one it ran under", i, d.ID, d.PayloadCap)
		}
	}
}

func TestTheBlockCarriesFiveRowsWheneverFiveCandidatesClearTheFloor(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}
	candidates := make([]Candidate, 0, 10)
	for i := range 10 {
		candidates = append(candidates, Candidate{ID: int64(10 * (i + 1)), Similarity: 0.9, Content: strings.Repeat("x", 12_000)})
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, 0)

	admitted := 0
	for _, d := range dispositions {
		if d.Included {
			admitted++
		}
	}
	if admitted != 5 {
		t.Fatalf("the block carried %d rows of the ten offered, want 5: the ceiling exists to state that a block always carries at least five memories, and a block that carries fewer has lost the property the number was chosen for", admitted)
	}
}

func TestTheCapChangesNothingWhenNoCandidateExceedsIt(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 100, Type: "documentation", Name: "Vision", Content: "the vision text"}
	candidates := []Candidate{
		{ID: 50, Type: "bug", Name: "Alpha", Similarity: 0.90, Content: "alpha body"},
		{ID: 20, Type: "documentation", Name: "Bravo", Similarity: 0.85, Content: "bravo body"},
		{ID: 300, Type: "task", Name: "Charlie", Similarity: 0.80, Content: "charlie body"},
	}

	block, dispositions := Assemble(anchor, candidates, 60_000, 0)

	const wantBlock = `===== ANCHOR =====
id: 100
type: documentation
name: Vision

the vision text

===== CANDIDATE =====
id: 20
type: documentation
name: Bravo

bravo body

===== CANDIDATE =====
id: 50
type: bug
name: Alpha

alpha body

===== CANDIDATE =====
id: 300
type: task
name: Charlie

charlie body

Seems like your knowledge is still thin on the topic - your focus might be too narrow; try approaching the question from a different angle.
`

	if block != wantBlock {
		t.Fatalf("block =\n%q\nwant\n%q\nthe rule must be inert where nothing reaches it", block, wantBlock)
	}
	for i, d := range dispositions {
		if !d.Included || d.CutReason != "" {
			t.Fatalf("dispositions[%d] (id %d) is %+v, want it admitted with no reason recorded", i, d.ID, d)
		}
	}
}
