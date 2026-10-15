package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestAssembleGoldenBlock is the milestone's central assertion (design
// §9.1, §9.2 step 3): a fixed candidate set in, one exact string out. The
// expected string is a literal, not built by calling renderBlock
// separately, so this is a true golden test of Assemble's own output.
func TestAssembleGoldenBlock(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 100, Type: "documentation", Name: "Vision", Substance: "the vision text"}
	candidates := []Candidate{
		{ID: 50, Type: "bug", Name: "Alpha", Similarity: 0.90, Substance: "alpha body"},
		{ID: 20, Type: "documentation", Name: "Bravo", Similarity: 0.85, Substance: "bravo body"},
		{ID: 300, Type: "task", Name: "Charlie", Similarity: 0.80, Substance: "charlie body"},
	}

	block, _ := Assemble(anchor, candidates, 60_000, FormSubstance)

	const wantBlock = `===== ANCHOR =====
id: 100
type: documentation
name: Vision

the vision text

===== CANDIDATE =====
id: 20
type: documentation
name: Bravo
similarity: 0.850

bravo body

===== CANDIDATE =====
id: 50
type: bug
name: Alpha
similarity: 0.900

alpha body

===== CANDIDATE =====
id: 300
type: task
name: Charlie
similarity: 0.800

charlie body
`

	if block != wantBlock {
		t.Fatalf("block =\n%q\nwant\n%q", block, wantBlock)
	}
}

func TestAssembleBlockOrderIsByIDNotByInputOrder(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 999, Type: "t", Name: "anchor", Substance: "anchor body"}

	rankOrderA := []Candidate{
		{ID: 3, Type: "t", Name: "c3", Similarity: 0.9, Substance: "cc3"},
		{ID: 1, Type: "t", Name: "c1", Similarity: 0.8, Substance: "cc1"},
		{ID: 2, Type: "t", Name: "c2", Similarity: 0.7, Substance: "cc2"},
	}
	rankOrderB := []Candidate{
		{ID: 2, Type: "t", Name: "c2", Similarity: 0.7, Substance: "cc2"},
		{ID: 3, Type: "t", Name: "c3", Similarity: 0.9, Substance: "cc3"},
		{ID: 1, Type: "t", Name: "c1", Similarity: 0.8, Substance: "cc1"},
	}

	blockA, _ := Assemble(anchor, rankOrderA, 60_000, FormSubstance)
	blockB, _ := Assemble(anchor, rankOrderB, 60_000, FormSubstance)

	if blockA != blockB {
		t.Fatalf("block changed when the same scored candidate set arrived in a different input order:\nA=%q\nB=%q", blockA, blockB)
	}
}

func TestAssembleCutsWhatDoesNotFitAndBackFillsASmallerCandidateBehindIt(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}

	candidates := []Candidate{
		{ID: 10, Substance: strings.Repeat("x", 40)},
		{ID: 20, Substance: strings.Repeat("y", 40)},
		{ID: 30, Substance: strings.Repeat("z", 40)},
		{ID: 40, Substance: strings.Repeat("w", 5)},
	}
	const budget = 100

	total := 0
	for _, c := range candidates {
		total += len(c.Substance)
	}
	if total <= budget {
		t.Fatalf("test setup error: total candidate bytes %d does not exceed budget %d, so this test cannot observe a cut", total, budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	want := []struct {
		id       int64
		included bool
	}{
		{10, true},
		{20, true},
		{30, false},
		{40, true},
	}
	for i, w := range want {
		if dispositions[i].ID != w.id {
			t.Fatalf("dispositions[%d].ID = %d, want %d", i, dispositions[i].ID, w.id)
		}
		if dispositions[i].Included != w.included {
			t.Fatalf("dispositions[%d] (id %d) Included = %v, want %v", i, w.id, dispositions[i].Included, w.included)
		}
	}

	if !dispositions[3].Included {
		t.Fatal("candidate 40 fits the budget candidate 30 could not use and was cut anyway; admission must skip, not stop (design §6.3, corrected)")
	}
	if dispositions[2].CutReason == "" {
		t.Fatal("a cut candidate has an empty CutReason, want it recorded")
	}
}

func TestAssembleAdmitsCandidatesBehindOneThatDoesNotFit(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Substance: strings.Repeat("x", 200)},
		{ID: 20, Substance: strings.Repeat("y", 40)},
		{ID: 30, Substance: strings.Repeat("z", 30)},
	}
	const budget = 100

	if len(candidates[0].Substance) <= budget {
		t.Fatalf("test setup error: rank 1 is %d bytes against a budget of %d, so it does not exceed the whole budget and this test cannot discriminate", len(candidates[0].Substance), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if dispositions[0].Included {
		t.Fatal("dispositions[0] (id 10) was admitted although it exceeds the whole budget")
	}
	for _, i := range []int{1, 2} {
		if !dispositions[i].Included {
			t.Fatalf("dispositions[%d] (id %d) was cut although it fits; an unadmittable candidate must not block the ones behind it", i, dispositions[i].ID)
		}
	}
	for i := range dispositions {
		if dispositions[i].Rank != i+1 {
			t.Fatalf("dispositions[%d] (id %d) has Rank %d, want %d — rank is the candidate's position in the recall order, not its position among the admitted", i, dispositions[i].ID, dispositions[i].Rank, i+1)
		}
	}
}

// TestAssembleRecordsSizeAndHashForCutCandidatesToo pins design §9.4
// obligation 1: the record carries the whole candidate set, not the
// surviving subset. A cut candidate must still be sized and hashed.
func TestAssembleRecordsSizeAndHashForCutCandidatesToo(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 200), Substance: strings.Repeat("x", 200)},
		{ID: 20, Content: strings.Repeat("y", 80), Substance: strings.Repeat("y", 80)},
	}
	const budget = 50

	if len(candidates[0].Substance) <= budget || len(candidates[1].Substance) <= budget {
		t.Fatalf("test setup error: candidates are %d and %d bytes against budget %d; both must exceed it for the second to be cut", len(candidates[0].Substance), len(candidates[1].Substance), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if len(dispositions) != 2 {
		t.Fatalf("got %d dispositions, want 2 (every candidate, including cut ones)", len(dispositions))
	}
	cut := dispositions[1]
	if cut.ID != 20 {
		t.Fatalf("dispositions[1].ID = %d, want 20", cut.ID)
	}
	if cut.Included {
		t.Fatal("candidate 20 was included, want it recorded as cut")
	}
	if cut.Size != 80 {
		t.Fatalf("cut candidate Size = %d, want 80 — size must be recorded even when cut", cut.Size)
	}
	if cut.ContentHash == "" {
		t.Fatal("cut candidate ContentHash is empty, want it recorded even when cut")
	}
}

// TestAssembleDispositionsPreserveRankOrderNotIDOrder pins design §8.2:
// candidates[] stays in the rank order recall returned, distinct from the
// block's id order.
func TestAssembleDispositionsPreserveRankOrderNotIDOrder(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 300, Substance: "a"},
		{ID: 100, Substance: "b"},
		{ID: 200, Substance: "c"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	wantOrder := []int64{300, 100, 200}
	for i, id := range wantOrder {
		if dispositions[i].ID != id {
			t.Fatalf("dispositions[%d].ID = %d, want %d — candidates[] must stay in rank order", i, dispositions[i].ID, id)
		}
		if dispositions[i].Rank != i+1 {
			t.Fatalf("dispositions[%d].Rank = %d, want %d", i, dispositions[i].Rank, i+1)
		}
	}
}

func TestAssembleCutReasonEmptyWhenIncluded(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 1, Substance: "small"}}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	if !dispositions[0].Included {
		t.Fatal("test setup error: candidate was cut, want it included")
	}
	if dispositions[0].CutReason != "" {
		t.Fatalf("CutReason = %q for an included candidate, want empty", dispositions[0].CutReason)
	}
}

func TestAssembleContentHashIsDeterministicAndDistinguishesContent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	same1 := []Candidate{{ID: 1, Content: "same"}}
	same2 := []Candidate{{ID: 1, Content: "same"}}
	different := []Candidate{{ID: 1, Content: "different"}}

	_, d1 := Assemble(anchor, same1, 60_000, FormSubstance)
	_, d2 := Assemble(anchor, same2, 60_000, FormSubstance)
	_, d3 := Assemble(anchor, different, 60_000, FormSubstance)

	if d1[0].ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
	if d1[0].ContentHash != d2[0].ContentHash {
		t.Fatalf("identical content produced different hashes: %q vs %q", d1[0].ContentHash, d2[0].ContentHash)
	}
	if d1[0].ContentHash == d3[0].ContentHash {
		t.Fatal("different content produced the same hash")
	}
}

// TestAssembleDispositionsRecordSimilarity pins CF-3: the disposition's
// similarity must be exactly what recall reported, for both an admitted
// and a cut candidate — recall@k needs the score, and a silently-zeroed
// score is retroactively uncomputable for every run ever written.
func TestAssembleDispositionsRecordSimilarity(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.734521, Substance: "included"},
		{ID: 20, Similarity: 0.100001, Substance: strings.Repeat("z", 1000)},
	}
	const budget = 20 // only candidate 10 fits; candidate 20 is cut

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if !dispositions[0].Included {
		t.Fatal("test setup error: candidate 10 was cut, want it included")
	}
	if dispositions[0].Similarity != 0.734521 {
		t.Fatalf("dispositions[0].Similarity = %v, want %v — the recorded score must be recall's, not zeroed", dispositions[0].Similarity, 0.734521)
	}
	if dispositions[1].Included {
		t.Fatal("test setup error: candidate 20 was included, want it cut")
	}
	if dispositions[1].Similarity != 0.100001 {
		t.Fatalf("dispositions[1].Similarity = %v, want %v — a cut candidate's similarity must still be recorded, not zeroed", dispositions[1].Similarity, 0.100001)
	}
}

// TestAssembleAdmitsACandidateThatExactlyFillsTheRemainingBudget pins W-1:
// admission at exactly the budget boundary is inclusive (cumulative+size
// <= budget), not exclusive.
func TestAssembleAdmitsACandidateThatExactlyFillsTheRemainingBudget(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Substance: strings.Repeat("x", 100)},
	}
	const budget = 100 // exactly the candidate's size

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if !dispositions[0].Included {
		t.Fatal("candidate exactly at the byte budget was cut, want it included — admission is <=, not <")
	}
	if dispositions[0].CutReason != "" {
		t.Fatalf("CutReason = %q for a candidate exactly at budget, want empty", dispositions[0].CutReason)
	}
}

func TestAssembleChargesTheAnchorBodyAgainstTheBudgetBeforeAnyCandidate(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Substance: strings.Repeat("a", 1_000)}
	candidates := []Candidate{
		{ID: 10, Substance: strings.Repeat("x", 50)},
	}
	const budget = 1_000

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if dispositions[0].Included {
		t.Fatal("candidate was admitted although the anchor alone already consumed the whole budget")
	}
	if dispositions[0].CutReason != cutReasonByteBudget {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, cutReasonByteBudget)
	}
}

func TestAssembleAdmitsACandidateIntoTheRoomLeftAfterTheAnchor(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Substance: strings.Repeat("a", 40)}
	candidates := []Candidate{
		{ID: 10, Substance: strings.Repeat("x", 60)},
	}
	const budget = 100

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if !dispositions[0].Included {
		t.Fatal("candidate sized to exactly the room left after the anchor was cut, want it admitted")
	}
}

func TestAssembleFloorsTheCandidateBudgetAtZeroWhenTheAnchorAloneExceedsIt(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "big", Substance: strings.Repeat("a", 100)}
	candidates := []Candidate{{ID: 10, Substance: "x"}}
	const budget = 50

	block, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	if dispositions[0].Included {
		t.Fatal("a one-byte candidate was cut for a run where the anchor alone already exceeds the budget, want it cut, not a negative-budget panic or a stray admission")
	}
	if dispositions[0].CutReason != cutReasonByteBudget {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, cutReasonByteBudget)
	}
	if strings.Contains(block, "CANDIDATE") {
		t.Fatalf("block contains a candidate section although nothing was admitted:\n%s", block)
	}
}

// TestAssembleContentHashIsSha256HexOfTheBodyExactly pins W-3: the hash is
// bound to sha256-of-the-body specifically, computed independently here so
// salting, sha512, or hashing the anchor's name instead of its content all
// fail this test. The hash is a cross-system verifiable claim.
func TestAssembleContentHashIsSha256HexOfTheBodyExactly(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Name: "AnchorName", Content: "anchor body text"}
	candidates := []Candidate{
		{ID: 10, Name: "CandName", Content: "candidate body text"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)
	anchorSummary := summarizeAnchor(anchor, FormSubstance)

	wantCandidateHash := sha256Hex(candidates[0].Content)
	if dispositions[0].ContentHash != wantCandidateHash {
		t.Fatalf("candidate ContentHash = %q, want sha256(body) = %q — not salted, not sha512, not hashing the name", dispositions[0].ContentHash, wantCandidateHash)
	}

	wantAnchorHash := sha256Hex(anchor.Content)
	if anchorSummary.ContentHash != wantAnchorHash {
		t.Fatalf("anchor ContentHash = %q, want sha256(body) = %q — not hashing the anchor's name", anchorSummary.ContentHash, wantAnchorHash)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestAssembleCutReasonHasTheExactWording pins W-8: the cut reason's
// wording is a recorded literal, not just a non-empty string.
func TestAssembleCutReasonHasTheExactWording(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Substance: strings.Repeat("x", 200)}}
	const budget = 10

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	const want = "byte budget exceeded"
	if dispositions[0].CutReason != want {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, want)
	}
}

func TestAssembleRecordsSubstanceAvailableAndSizeWhenPresent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: "a body", Substance: "condensed"},
		{ID: 20, Content: "b body", Substance: "x"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	if !dispositions[0].SubstanceAvailable {
		t.Fatal("SubstanceAvailable = false for a candidate that carried a substance, want true")
	}
	if dispositions[0].SubstanceSize != len("condensed") {
		t.Fatalf("SubstanceSize = %d, want %d — the substance's own byte length, not the content's", dispositions[0].SubstanceSize, len("condensed"))
	}
	if !dispositions[1].SubstanceAvailable {
		t.Fatal("SubstanceAvailable = false for a candidate whose substance is a single byte, want true — presence is non-empty, not a length threshold")
	}
	if dispositions[1].SubstanceSize != len("x") {
		t.Fatalf("SubstanceSize = %d, want %d for a one-byte substance", dispositions[1].SubstanceSize, len("x"))
	}
}

func TestAssembleRecordsSubstanceAbsentAsTheZeroValueNotAnError(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: "a body"}}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	if dispositions[0].SubstanceAvailable {
		t.Fatal("SubstanceAvailable = true for a candidate with no Substance, want false")
	}
	if dispositions[0].SubstanceSize != 0 {
		t.Fatalf("SubstanceSize = %d for a candidate with no Substance, want 0", dispositions[0].SubstanceSize)
	}
}

func TestAssembleRecordsSubstanceForACutCandidateToo(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Substance: strings.Repeat("x", 15)},
		{ID: 20, Content: strings.Repeat("y", 80), Substance: "condensed form"},
	}
	const budget = 20

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	cut := dispositions[1]
	if cut.Included {
		t.Fatal("candidate 20 was included, want it recorded as cut")
	}
	if !cut.SubstanceAvailable {
		t.Fatal("a cut candidate's SubstanceAvailable is false although it carried a substance, want it recorded regardless of the admit-or-cut outcome")
	}
	if cut.SubstanceSize != len("condensed form") {
		t.Fatalf("cut candidate SubstanceSize = %d, want %d", cut.SubstanceSize, len("condensed form"))
	}
}

func TestAssembleAdmissionChargesOnlySubstanceBytesNeverContentBytes(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 10_000), Substance: strings.Repeat("s", 40)},
		{ID: 20, Content: strings.Repeat("y", 10_000), Substance: strings.Repeat("t", 40)},
	}
	const budget = 100

	_, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

	for i := range dispositions {
		if !dispositions[i].Included {
			t.Fatalf("dispositions[%d] (id %d) was cut although its substance fits the budget; admission must charge Substance bytes, never Content bytes", i, dispositions[i].ID)
		}
	}
}

func TestAssembleRendersIdenticalBlockRegardlessOfContent(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a", Substance: "anchor body"}

	short := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: "bravo", Substance: "condensed"}}
	long := []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Content: strings.Repeat("bravo", 5_000), Substance: "condensed"}}

	blockShort, _ := Assemble(anchor, short, 60_000, FormSubstance)
	blockLong, _ := Assemble(anchor, long, 60_000, FormSubstance)

	if blockShort != blockLong {
		t.Fatalf("block changed when a candidate's content changed:\nshort=%q\nlong=%q", blockShort, blockLong)
	}
}

func TestDispositionSubstanceFieldsSerializeEvenAtTheZeroValue(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: "a body"}}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	encoded, err := json.Marshal(dispositions[0])
	if err != nil {
		t.Fatalf("marshal disposition: %v", err)
	}
	for _, want := range []string{`"substanceAvailable":false`, `"substanceSize":0`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("disposition JSON = %s, want it to contain %q", encoded, want)
		}
	}
}

func TestAssembleEmptyCandidatesRendersTheAnchorAlone(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "solo", Substance: "just the anchor"}

	block, dispositions := Assemble(anchor, nil, 60_000, FormSubstance)

	if len(dispositions) != 0 {
		t.Fatalf("got %d dispositions for no candidates, want 0", len(dispositions))
	}
	// W-5: a nil slice serializes as JSON null, not []; dispositions must
	// be a non-nil empty slice even when there are no candidates.
	if dispositions == nil {
		t.Fatal("dispositions is nil for no candidates, want a non-nil empty slice (JSON null vs [])")
	}
	const want = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
	if block != want {
		t.Fatalf("block = %q, want %q", block, want)
	}
}

func TestTheAssembledBlockDisclosesOnlyTheAdmittedRowsAndWhetherAnythingWasWithheld(t *testing.T) {
	t.Parallel()

	t.Run("partial admission", func(t *testing.T) {
		t.Parallel()

		anchor := Anchor{ID: 1, Type: "t", Name: "anchor", Substance: "anchor body"}
		candidates := []Candidate{
			{ID: 10, Substance: strings.Repeat("x", 40)},
			{ID: 20, Substance: strings.Repeat("y", 40)},
			{ID: 900, Substance: strings.Repeat("s", 60)},
			{ID: 30, Substance: strings.Repeat("z", 40)},
		}
		const budget = 90

		fullBlock, dispositions := Assemble(anchor, candidates, budget, FormSubstance)

		admittedIDs := make(map[int64]bool, len(dispositions))
		for _, d := range dispositions {
			if d.Included {
				admittedIDs[d.ID] = true
			}
		}
		if len(admittedIDs) == 0 || len(admittedIDs) == len(candidates) {
			t.Fatalf("test setup error: %d of %d candidates admitted, want a real cut so the two calls below have something to disagree about", len(admittedIDs), len(candidates))
		}

		admittedOnly := make([]Candidate, 0, len(admittedIDs))
		for _, c := range candidates {
			if admittedIDs[c.ID] {
				admittedOnly = append(admittedOnly, c)
			}
		}
		admittedOnlyBlock, _ := Assemble(anchor, admittedOnly, budget, FormSubstance)

		if fullBlock != admittedOnlyBlock {
			t.Fatalf("assembling the full candidate list produced a different block than assembling only the rows it admitted: full=\n%q\nadmitted-only=\n%q\nthe block must disclose nothing about a withheld row — its reason, its id, its size, or the bare fact that it exists", fullBlock, admittedOnlyBlock)
		}
	})

	t.Run("two shutouts with different withheld rows render the same block", func(t *testing.T) {
		t.Parallel()

		anchor := Anchor{ID: 2, Type: "t", Name: "anchor", Substance: "anchor body"}
		shutoutA := []Candidate{
			{ID: 40, Substance: "a body longer than the budget"},
			{ID: 50, Substance: strings.Repeat("w", 200)},
		}
		shutoutB := []Candidate{
			{ID: 900, Substance: "a different body longer than the budget"},
			{ID: 901, Similarity: 0.9, Substance: strings.Repeat("v", 500)},
		}
		const budget = 20

		blockA, dispositionsA := Assemble(anchor, shutoutA, budget, FormSubstance)
		blockB, dispositionsB := Assemble(anchor, shutoutB, budget, FormSubstance)

		for _, d := range dispositionsA {
			if d.Included {
				t.Fatalf("test setup error: shutout A admitted id %d, want a genuine shutout — every candidate cut", d.ID)
			}
		}
		for _, d := range dispositionsB {
			if d.Included {
				t.Fatalf("test setup error: shutout B admitted id %d, want a genuine shutout — every candidate cut", d.ID)
			}
		}

		if blockA != blockB {
			t.Fatalf("two shutouts with a different candidate count, different ids and different sizes produced different blocks: A=\n%q\nB=\n%q\nthe block must disclose nothing about a withheld row — its reason, its id, its size, or how many there were — only that something was", blockA, blockB)
		}
	})
}

func TestTheBlockStatesThatNothingWasAdmittedRatherThanRenderingTheAnchorAlone(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "solo", Substance: "anchor body"}
	candidates := []Candidate{{ID: 10, Similarity: 0.1, Substance: "small"}}

	block, dispositions := Assemble(anchor, candidates, 5, FormSubstance)

	if len(dispositions) == 0 || dispositions[0].Included {
		t.Fatal("test setup error: the candidate was not cut, so the block has nothing to disclose")
	}
	const want = "results were found, but none were included."
	if !strings.Contains(block, want) {
		t.Fatalf("block = %q, want it to contain %q when the budget admits nothing", block, want)
	}
}

func namedCandidates(n int) []Candidate {
	candidates := make([]Candidate, n)
	for i := range n {
		candidates[i] = Candidate{ID: int64(i + 1), Type: "t", Name: fmt.Sprintf("c%d", i+1), Similarity: 1, Substance: "body"}
	}
	return candidates
}

func TestTheBlockOfNothingConsideredIsTheAnchorAlone(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "anchor", Substance: "anchor body"}

	block, dispositions := Assemble(anchor, nil, 60_000, FormSubstance)

	if len(dispositions) != 0 {
		t.Fatalf("test setup error: got %d dispositions, want 0 admitted for this boundary", len(dispositions))
	}
	const want = "===== ANCHOR =====\nid: 1\ntype: t\nname: anchor\n\nanchor body\n"
	if block != want {
		t.Fatalf("block = %q, want %q: a block that found nothing says nothing about it", block, want)
	}
}

func TestTheBlockOfNothingIncludedEndsWithTheCutSentenceAlone(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "anchor", Substance: "anchor body"}
	candidates := []Candidate{{ID: 10, Similarity: 0.1, Substance: "small"}}

	block, dispositions := Assemble(anchor, candidates, 5, FormSubstance)

	if len(dispositions) == 0 || dispositions[0].Included {
		t.Fatal("test setup error: the candidate was not cut, so this boundary is not exercised")
	}

	const want = "===== ANCHOR =====\nid: 1\ntype: t\nname: anchor\n\nanchor body\n\nresults were found, but none were included.\n"
	if block != want {
		t.Fatalf("block = %q, want %q: the cut sentence is the last thing the block says", block, want)
	}
}

func TestAThinBlockCarriesNoSentenceAboutItsThinness(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "anchor", Substance: "anchor body"}
	candidates := []Candidate{{ID: 10, Type: "t", Name: "c", Similarity: 0.40, Substance: "body"}}

	block, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	if len(dispositions) != 1 || !dispositions[0].Included {
		t.Fatal("test setup error: the weak row was not admitted, so this boundary is not exercised")
	}
	const want = "===== ANCHOR =====\nid: 1\ntype: t\nname: anchor\n\nanchor body\n\n===== CANDIDATE =====\nid: 10\ntype: t\nname: c\nsimilarity: 0.400\n\nbody\n"
	if block != want {
		t.Fatalf("block = %q, want %q: a weak match is rendered like any other and the block ends with it", block, want)
	}
}

func TestThinFollowsTheBestSimilarityNotTheRowCount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		similarities []float64
		thin         bool
	}{
		{"no row", nil, true},
		{"one weak row", []float64{0.40}, true},
		{"six weak rows", []float64{0.40, 0.45, 0.50, 0.55, 0.60, 0.62}, true},
		{"one strong row", []float64{0.90}, false},
		{"one strong row among weak ones", []float64{0.40, 0.45, 0.50, 0.55, 0.60, 0.70}, false},
		{"a best row exactly at the threshold", []float64{0.40, ThinKnowledgeSimilarity}, false},
	}

	for _, tc := range cases {
		if got := Thin(tc.similarities); got != tc.thin {
			t.Errorf("%s: Thin = %v, want %v: thinness is a property of the best similarity, not of how many rows there are", tc.name, got, tc.thin)
		}
	}
}

func TestNoCandidateIsCutForItsSimilarity(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "anchor", Substance: "anchor body"}
	candidates := []Candidate{
		{ID: 10, Similarity: 0.60, Substance: "just under the old floor"},
		{ID: 20, Similarity: 0.05, Substance: "barely related"},
		{ID: 30, Similarity: -0.10, Substance: "opposed"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	for _, d := range dispositions {
		if !d.Included || d.CutReason != "" {
			t.Fatalf("disposition %d is %+v, want it admitted: a top-N aperture hands the model its best rows with their scores and cuts none for being weak", d.ID, d)
		}
	}
}

func TestEachCandidateSectionStatesItsSimilarityAndTheAnchorDoesNot(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "anchor", Substance: "anchor body"}
	candidates := []Candidate{
		{ID: 10, Type: "t", Name: "first", Similarity: 0.7126, Substance: "one"},
		{ID: 20, Type: "t", Name: "second", Similarity: 0.5, Substance: "two"},
	}

	block, _ := Assemble(anchor, candidates, 60_000, FormSubstance)

	for _, want := range []string{"name: first\nsimilarity: 0.713\n", "name: second\nsimilarity: 0.500\n"} {
		if !strings.Contains(block, want) {
			t.Errorf("block = %q, want it to contain %q: the model judges a row by its score, and a score it is not shown is a judgement it cannot make", block, want)
		}
	}
	anchorSection := block[:strings.Index(block, "===== CANDIDATE")]
	if strings.Contains(anchorSection, "similarity") {
		t.Errorf("the anchor section is %q, want no similarity: the anchor was not ranked", anchorSection)
	}
}

func TestARowFetchedByIDCarriesNoSimilarityLine(t *testing.T) {
	t.Parallel()

	read := candidateFromAnchor(Anchor{ID: 7, Type: "t", Name: "read", Substance: "body"})
	var b strings.Builder
	renderSection(&b, read, FormSubstance)

	if strings.Contains(b.String(), "similarity") {
		t.Fatalf("section = %q, want no similarity line: a row read by id was never scored", b.String())
	}
}

func TestDispositionFormAndRenderedSizeSerializeForEveryCandidate(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 1000), Substance: strings.Repeat("z", 300)},
		{ID: 20, Content: "a body"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000, FormSubstance)

	for i, want := range []string{`"form":"substance","renderedSize":300`, `"form":"unavailable","renderedSize":0`} {
		encoded, err := json.Marshal(dispositions[i])
		if err != nil {
			t.Fatalf("marshal disposition %d: %v", i, err)
		}
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("disposition JSON = %s, want it to contain %q", encoded, want)
		}
	}
}

func TestDispositionDecodedFromARecordWrittenBeforeTheFormRuleChargesItsContentLength(t *testing.T) {
	t.Parallel()

	const legacy = `{"rank":1,"id":10,"size":1000,"contentHash":"h","included":true}`

	var d Disposition
	if err := json.Unmarshal([]byte(legacy), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if d.RenderedSize != 1000 {
		t.Fatalf("RenderedSize = %d for a record carrying none, want 1000 - every candidate in such a record was charged its content's own byte length, and a zero here reports a block that carried nothing", d.RenderedSize)
	}
}

func TestDispositionDecodedFromARecordThatStatesARenderedSizeKeepsIt(t *testing.T) {
	t.Parallel()

	const written = `{"rank":1,"id":10,"size":1000,"form":"substance","renderedSize":300,"included":true}`

	var d Disposition
	if err := json.Unmarshal([]byte(written), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if d.RenderedSize != 300 {
		t.Fatalf("RenderedSize = %d, want 300 - the back-compat shim must not overwrite a size the record states", d.RenderedSize)
	}
}

func TestAnAnchorSummaryDecodedFromARecordWrittenBeforeTheAnchorRenderedItsSubstanceChargesItsContentLength(t *testing.T) {
	t.Parallel()

	var a AnchorSummary
	if err := json.Unmarshal([]byte(`{"id":1,"size":900,"contentHash":"h"}`), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if a.RenderedSize != 900 {
		t.Fatalf("RenderedSize = %d for a record carrying none, want 900", a.RenderedSize)
	}
}

func TestAnAnchorSummaryThatStatesAnUnavailableFormKeepsItsZeroRenderedSize(t *testing.T) {
	t.Parallel()

	var a AnchorSummary
	if err := json.Unmarshal([]byte(`{"id":1,"size":900,"contentHash":"h","form":"unavailable","renderedSize":0}`), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if a.RenderedSize != 0 {
		t.Fatalf("RenderedSize = %d, want 0: an unavailable anchor rendered no body, whatever its content weighs", a.RenderedSize)
	}
}

func TestADispositionThatStatesAnUnavailableFormKeepsItsZeroRenderedSize(t *testing.T) {
	t.Parallel()

	var d Disposition
	if err := json.Unmarshal([]byte(`{"rank":1,"id":10,"size":1000,"contentHash":"h","included":true,"form":"unavailable","renderedSize":0}`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if d.RenderedSize != 0 {
		t.Fatalf("RenderedSize = %d, want 0: an unavailable row rendered no body, whatever its content weighs", d.RenderedSize)
	}
}
