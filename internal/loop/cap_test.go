package loop

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

const testBlockOccupancy = 5

func admissionBeforeTheCeilingExisted(candidates []Candidate, budget, spent int, floor float64) (admitted []Candidate, dispositions []Disposition) {
	remaining := budget - spent
	if remaining < 0 {
		remaining = 0
	}

	dispositions = make([]Disposition, len(candidates))
	admitted = make([]Candidate, 0, len(candidates))

	cumulative := 0
	for i, c := range candidates {
		size := len(c.Content)
		d := Disposition{Rank: i + 1, ID: c.ID}

		switch {
		case c.SelfProduced:
			d.CutReason = cutReasonSelfProduced
		case c.Similarity < floor:
			d.CutReason = cutReasonBelowFloor
		case cumulative+size <= remaining:
			cumulative += size
			d.Included = true
			admitted = append(admitted, c)
		default:
			d.CutReason = cutReasonByteBudget
		}

		dispositions[i] = d
	}

	return admitted, dispositions
}

func blockOver(anchor Anchor, admitted []Candidate, considered int) string {
	rows := append([]Candidate(nil), admitted...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return renderBlock(anchor, rows, considered > 0)
}

func TestTheShippedBlockOccupancyLeavesNoCeilingInForce(t *testing.T) {
	t.Parallel()

	if got := payloadCap(AssemblyByteBudget, BlockOccupancy); got != noPayloadCeiling {
		t.Fatalf("the shipped occupancy puts the ceiling at %d, want none in force: the mechanism lands inert and the dial moves only on a measurement that has not been made", got)
	}
}

func TestWithNoCeilingInForceEveryBlockAndEveryReasonIsTheOneAdmissionGaveBeforeTheCeilingExisted(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name       string
		anchor     Anchor
		candidates []Candidate
		budget     int
		floor      float64
	}{
		{
			name:   "a row larger than the whole budget",
			anchor: Anchor{ID: 1, Type: "t", Name: "a"},
			candidates: []Candidate{
				{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 120_000)},
				{ID: 20, Similarity: 0.8, Content: "small body"},
			},
			budget: 60_000,
		},
		{
			name:   "every row over a fifth of the budget",
			anchor: Anchor{ID: 1, Type: "t", Name: "a"},
			candidates: []Candidate{
				{ID: 10, Similarity: 0.9, Content: strings.Repeat("a", 43_000)},
				{ID: 20, Similarity: 0.8, Content: strings.Repeat("b", 17_500)},
				{ID: 30, Similarity: 0.7, Content: strings.Repeat("c", 13_000)},
			},
			budget: 60_000,
		},
		{
			name:   "the floor and the self-produced cut mixed in",
			anchor: Anchor{ID: 1, Type: "t", Name: "a", Content: "anchor body"},
			candidates: []Candidate{
				{ID: 10, Similarity: 0.9, Content: strings.Repeat("a", 30_000)},
				{ID: 20, Similarity: 0.9, Content: strings.Repeat("b", 30_000), SelfProduced: true},
				{ID: 30, Similarity: 0.1, Content: strings.Repeat("c", 100)},
				{ID: 40, Similarity: 0.9, Content: strings.Repeat("d", 40_000)},
				{ID: 50, Similarity: 0.9, Content: strings.Repeat("e", 100)},
			},
			budget: 60_000,
			floor:  RelevanceFloor,
		},
		{
			name:       "an anchor that spends the budget on its own",
			anchor:     Anchor{ID: 1, Type: "t", Name: "a", Content: strings.Repeat("a", 72_400)},
			candidates: []Candidate{{ID: 10, Similarity: 0.9, Content: "small body"}},
			budget:     60_000,
		},
		{
			name:       "nothing to admit at all",
			anchor:     Anchor{ID: 1, Type: "t", Name: "a", Content: "anchor body"},
			candidates: nil,
			budget:     60_000,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			block, got := Assemble(scenario.anchor, scenario.candidates, scenario.budget, scenario.floor)
			wantAdmitted, want := admissionBeforeTheCeilingExisted(scenario.candidates, scenario.budget, len(scenario.anchor.Content), scenario.floor)

			if len(got) != len(want) {
				t.Fatalf("admission recorded %d rows, want %d", len(got), len(want))
			}
			for i := range want {
				if got[i].Included != want[i].Included || got[i].CutReason != want[i].CutReason {
					t.Fatalf("row %d (id %d) is included=%v reason=%q, want included=%v reason=%q: with no ceiling in force every reason must be the one the byte budget alone would have given, or the mechanism has changed the record while claiming to be inert", i, got[i].ID, got[i].Included, got[i].CutReason, want[i].Included, want[i].CutReason)
				}
				if got[i].CutReason == cutReasonOversized {
					t.Fatalf("row %d (id %d) was refused for its size although no ceiling is in force, which puts a row the byte budget owns into the condensation queue", i, got[i].ID)
				}
			}

			if wantBlock := blockOver(scenario.anchor, wantAdmitted, len(scenario.candidates)); block != wantBlock {
				t.Fatalf("block =\n%q\nwant\n%q\nwith no ceiling in force the rendered bytes must be the ones admission produced before the ceiling existed", block, wantBlock)
			}
		})
	}
}

func TestAnOccupancyOfOneIsNotTheOffPositionBecauseItRelabelsAByteBudgetCut(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 120_000)}}

	_, atOne := admit(candidates, 60_000, 0, 0, 1)
	_, off := admit(candidates, 60_000, 0, 0, 0)

	if atOne[0].CutReason != cutReasonOversized {
		t.Fatalf("at an occupancy of one the row reports %q, want %q: the ceiling equals the budget there, which is the whole reason one is not the off position", atOne[0].CutReason, cutReasonOversized)
	}
	if off[0].CutReason != cutReasonByteBudget {
		t.Fatalf("with no ceiling in force the row reports %q, want %q", off[0].CutReason, cutReasonByteBudget)
	}
	if atOne[0].CutReason == off[0].CutReason {
		t.Fatalf("both spellings report %q, so this test can no longer tell the off position from a ceiling the size of the budget", off[0].CutReason)
	}
}

func TestAdmissionRefusesACandidateWhoseRenderedPayloadExceedsTheCap(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{
		{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 12_001)},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0, testBlockOccupancy)

	if dispositions[0].Included {
		t.Fatalf("a 12001-byte payload was admitted against a 60000-byte block budget, want it refused: the per-candidate ceiling is one fifth of the budget, so 12000 is the largest payload that may be carried")
	}
	if dispositions[0].CutReason != "oversized" {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, "oversized")
	}
}

func TestAdmissionCarriesACandidateExactlyAtTheCapAndTheLargestOneBelowIt(t *testing.T) {
	t.Parallel()

	for _, size := range []int{11_999, 12_000} {
		candidates := []Candidate{{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", size)}}

		_, dispositions := admit(candidates, 60_000, 0, 0, testBlockOccupancy)

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

	candidates := []Candidate{
		{ID: 10, Similarity: 0.99, Content: strings.Repeat("o", 12_001)},
		{ID: 20, Similarity: 0.98, Content: strings.Repeat("a", 11_000)},
		{ID: 30, Similarity: 0.97, Content: strings.Repeat("b", 11_000)},
		{ID: 40, Similarity: 0.96, Content: strings.Repeat("c", 11_000)},
		{ID: 50, Similarity: 0.95, Content: strings.Repeat("d", 11_000)},
		{ID: 60, Similarity: 0.94, Content: strings.Repeat("e", 11_000)},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0, testBlockOccupancy)

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

	candidates := []Candidate{
		{ID: 10, Similarity: 0.2, Content: strings.Repeat("x", 40_000)},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0.63, testBlockOccupancy)

	if dispositions[0].CutReason != "below relevance floor" {
		t.Fatalf("CutReason = %q, want %q: a row that is both irrelevant and too large is cut for its relevance, because the size rule is asked only about rows that cleared the floor", dispositions[0].CutReason, "below relevance floor")
	}
}

func TestARowRefusedForItsSizeKeepsItsScoreItsSizeAndItsRankInTheRecord(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{
		{ID: 10, Type: "documentation", Name: "Big", Similarity: 0.812345, Content: strings.Repeat("x", 43_273)},
		{ID: 20, Type: "task", Name: "Small", Similarity: 0.71, Content: "small body"},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0, testBlockOccupancy)

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

	candidates := []Candidate{
		{ID: 10, Similarity: 0.99, Content: strings.Repeat("o", 12_001)},
		{ID: 20, Similarity: 0.98, Content: strings.Repeat("a", 11_900)},
		{ID: 30, Similarity: 0.97, Content: strings.Repeat("b", 11_900)},
		{ID: 40, Similarity: 0.96, Content: strings.Repeat("c", 11_900)},
		{ID: 50, Similarity: 0.95, Content: strings.Repeat("d", 11_900)},
		{ID: 60, Similarity: 0.94, Content: strings.Repeat("e", 11_900)},
		{ID: 70, Similarity: 0.93, Content: strings.Repeat("f", 11_900)},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0, testBlockOccupancy)

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

	_, block := admit(candidates, 60_000, 0, 0, testBlockOccupancy)
	if !block[0].Included || block[0].PayloadCap != 12_000 {
		t.Fatalf("against a 60000-byte budget the row is recorded as %+v, want it admitted under a ceiling of 12000", block[0])
	}

	_, supplementary := admit(candidates, 20_000, 0, 0, testBlockOccupancy)
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

	_, dispositions := admit(candidates, 60_000, len(anchor.Content), 0, testBlockOccupancy)

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

	candidates := []Candidate{
		{ID: 10, Similarity: 0.9, Content: "admitted body"},
		{ID: 20, Similarity: 0.9, Content: strings.Repeat("x", 12_001)},
		{ID: 30, Similarity: 0.1, Content: "below the floor"},
		{ID: 40, Similarity: 0.9, Content: "self produced body", SelfProduced: true},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0.63, testBlockOccupancy)

	for i, d := range dispositions {
		if d.PayloadCap != 12_000 {
			t.Fatalf("dispositions[%d] (id %d) states a ceiling of %d, want 12000: a stored record answers what a different ceiling would have done only if every row names the one it ran under", i, d.ID, d.PayloadCap)
		}
	}
}

func TestEveryRowStatesThatNoCeilingWasInForceWhenNoneWas(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{
		{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 43_000)},
		{ID: 20, Similarity: 0.9, Content: "admitted body"},
	}

	_, dispositions := admit(candidates, 60_000, 0, 0, BlockOccupancy)

	for i, d := range dispositions {
		if d.PayloadCap != noPayloadCeiling {
			t.Fatalf("dispositions[%d] (id %d) states a ceiling of %d, want none recorded: a row judged without a ceiling must not name one, or a reader takes an inert mechanism for an applied one", i, d.ID, d.PayloadCap)
		}
	}
}

func TestTheRecordNamesTheCeilingWhenOneWasInForceAndOmitsItWhenNoneWas(t *testing.T) {
	t.Parallel()

	candidates := []Candidate{{ID: 10, Similarity: 0.9, Content: strings.Repeat("x", 5_000)}}

	_, capped := admit(candidates, 60_000, 0, 0, testBlockOccupancy)
	cappedJSON, err := json.Marshal(capped[0])
	if err != nil {
		t.Fatalf("marshalling the capped row: %v", err)
	}
	if !strings.Contains(string(cappedJSON), `"payloadCap":12000`) {
		t.Fatalf("the capped row serialises as %s, want it to name the 12000-byte ceiling it ran under", cappedJSON)
	}

	_, uncapped := admit(candidates, 60_000, 0, 0, BlockOccupancy)
	uncappedJSON, err := json.Marshal(uncapped[0])
	if err != nil {
		t.Fatalf("marshalling the uncapped row: %v", err)
	}
	if strings.Contains(string(uncappedJSON), "payloadCap") {
		t.Fatalf("the row judged without a ceiling serialises as %s, want the field absent: a recorded zero reads as a ceiling of no bytes, which is a different claim from no ceiling at all", uncappedJSON)
	}
}

func TestFiveRowsAtTheCeilingExactlyExhaustABudgetTheAnchorHasNotTouched(t *testing.T) {
	t.Parallel()

	candidates := make([]Candidate, 0, 10)
	for i := range 10 {
		candidates = append(candidates, Candidate{ID: int64(10 * (i + 1)), Similarity: 0.9, Content: strings.Repeat("x", 12_000)})
	}

	_, dispositions := admit(candidates, 60_000, 0, 0, testBlockOccupancy)

	admitted := 0
	for _, d := range dispositions {
		if d.Included {
			admitted++
		}
	}
	if admitted != 5 {
		t.Fatalf("the block carried %d rows of the ten offered, want 5: the ceiling is one fifth of the block budget, so five rows at the ceiling exactly exhaust a budget no anchor has spent from — the count holds only under that precondition, and an anchor of its own is enough to lose it", admitted)
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

	admitted, dispositions := admit(candidates, 60_000, len(anchor.Content), 0, testBlockOccupancy)
	block := blockOver(anchor, admitted, len(candidates))

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
