package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// TestAssembleGoldenBlock is the milestone's central assertion (design
// §9.1, §9.2 step 3): a fixed candidate set in, one exact string out. The
// expected string is a literal, not built by calling renderBlock
// separately, so this is a true golden test of Assemble's own output.
func TestAssembleGoldenBlock(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 100, Type: "documentation", Name: "Vision", Content: "the vision text"}
	candidates := []Candidate{
		{ID: 50, Type: "bug", Name: "Alpha", Similarity: 0.90, Content: "alpha body"},
		{ID: 20, Type: "documentation", Name: "Bravo", Similarity: 0.85, Content: "bravo body"},
		{ID: 300, Type: "task", Name: "Charlie", Similarity: 0.80, Content: "charlie body"},
	}

	block, _ := Assemble(anchor, candidates, 60_000)

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
`

	if block != wantBlock {
		t.Fatalf("block =\n%q\nwant\n%q", block, wantBlock)
	}
}

// TestAssembleBlockOrderIsByIDNotByInputOrder pins design §6.3 directly: a
// score reshuffle (a tie, or a shift in the ninth decimal, or ranking
// noise between two calls) must not move a byte in the rendered block, as
// long as the admitted candidate *set* is unchanged.
func TestAssembleBlockOrderIsByIDNotByInputOrder(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 999, Type: "t", Name: "anchor", Content: "anchor body"}

	rankOrderA := []Candidate{
		{ID: 3, Type: "t", Name: "c3", Similarity: 0.9, Content: "cc3"},
		{ID: 1, Type: "t", Name: "c1", Similarity: 0.8, Content: "cc1"},
		{ID: 2, Type: "t", Name: "c2", Similarity: 0.7, Content: "cc2"},
	}
	rankOrderB := []Candidate{
		{ID: 1, Type: "t", Name: "c1", Similarity: 0.75, Content: "cc1"},
		{ID: 2, Type: "t", Name: "c2", Similarity: 0.72, Content: "cc2"},
		{ID: 3, Type: "t", Name: "c3", Similarity: 0.70, Content: "cc3"},
	}

	blockA, _ := Assemble(anchor, rankOrderA, 60_000)
	blockB, _ := Assemble(anchor, rankOrderB, 60_000)

	if blockA != blockB {
		t.Fatalf("block changed when the same candidate set arrived in a different rank order:\nA=%q\nB=%q", blockA, blockB)
	}
}

func TestAssembleCutsWhatDoesNotFitAndBackFillsASmallerCandidateBehindIt(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "a"}

	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 40)},
		{ID: 20, Content: strings.Repeat("y", 40)},
		{ID: 30, Content: strings.Repeat("z", 40)},
		{ID: 40, Content: strings.Repeat("w", 5)},
	}
	const budget = 100

	total := 0
	for _, c := range candidates {
		total += len(c.Content)
	}
	if total <= budget {
		t.Fatalf("test setup error: total candidate bytes %d does not exceed budget %d, so this test cannot observe a cut", total, budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget)

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
		{ID: 10, Content: strings.Repeat("x", 200)},
		{ID: 20, Content: strings.Repeat("y", 40)},
		{ID: 30, Content: strings.Repeat("z", 30)},
	}
	const budget = 100

	if len(candidates[0].Content) <= budget {
		t.Fatalf("test setup error: rank 1 is %d bytes against a budget of %d, so it does not exceed the whole budget and this test cannot discriminate", len(candidates[0].Content), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget)

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

func TestAssembleCutsSelfProducedCandidatesWithoutChargingTheBudget(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 40), SelfProduced: true},
		{ID: 20, Content: strings.Repeat("y", 40)},
		{ID: 30, Content: strings.Repeat("z", 40)},
	}
	const budget = 100

	if len(candidates[0].Content) > budget {
		t.Fatalf("test setup error: the self-produced row is %d bytes against a budget of %d, so the byte rule would cut it anyway", len(candidates[0].Content), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget)

	if dispositions[0].Included {
		t.Fatal("dispositions[0] (id 10) is self-produced and was admitted, want it cut")
	}
	for _, i := range []int{1, 2} {
		if !dispositions[i].Included {
			t.Fatalf("dispositions[%d] (id %d) was cut; two 40-byte rows fit a 100-byte budget only if the self-produced row's 40 bytes were never charged", i, dispositions[i].ID)
		}
	}
}

func TestAssembleReportsSelfProducedRatherThanBudgetForAFittingRunRecord(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: "a body that fits easily", SelfProduced: true}}
	const budget = 1_000

	if len(candidates[0].Content) > budget {
		t.Fatalf("test setup error: the row is %d bytes against a budget of %d; both rule orders cut it and the test cannot discriminate", len(candidates[0].Content), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget)

	if dispositions[0].Included {
		t.Fatal("a self-produced candidate that fits the budget was admitted, want it cut")
	}
	if dispositions[0].CutReason == cutReasonByteBudget {
		t.Fatalf("CutReason = %q for a row that fits the budget; the self-produced rule must be applied before the byte rule", dispositions[0].CutReason)
	}
	if dispositions[0].CutReason != cutReasonSelfProduced {
		t.Fatalf("CutReason = %q, want the self-produced reason", dispositions[0].CutReason)
	}
}

// TestAssembleRecordsSizeAndHashForCutCandidatesToo pins design §9.4
// obligation 1: the record carries the whole candidate set, not the
// surviving subset. A cut candidate must still be sized and hashed.
func TestAssembleRecordsSizeAndHashForCutCandidatesToo(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 200)},
		{ID: 20, Content: strings.Repeat("y", 80)},
	}
	const budget = 50

	if len(candidates[0].Content) <= budget || len(candidates[1].Content) <= budget {
		t.Fatalf("test setup error: candidates are %d and %d bytes against budget %d; both must exceed it for the second to be cut", len(candidates[0].Content), len(candidates[1].Content), budget)
	}

	_, dispositions := Assemble(anchor, candidates, budget)

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
		{ID: 300, Content: "a"},
		{ID: 100, Content: "b"},
		{ID: 200, Content: "c"},
	}

	_, dispositions := Assemble(anchor, candidates, 60_000)

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
	candidates := []Candidate{{ID: 1, Content: "small"}}

	_, dispositions := Assemble(anchor, candidates, 60_000)

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

	_, d1 := Assemble(anchor, same1, 60_000)
	_, d2 := Assemble(anchor, same2, 60_000)
	_, d3 := Assemble(anchor, different, 60_000)

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
		{ID: 10, Similarity: 0.734521, Content: "included"},
		{ID: 20, Similarity: 0.100001, Content: strings.Repeat("z", 1000)},
	}
	const budget = 20 // only candidate 10 fits; candidate 20 is cut

	_, dispositions := Assemble(anchor, candidates, budget)

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
		{ID: 10, Content: strings.Repeat("x", 100)},
	}
	const budget = 100 // exactly the candidate's size

	_, dispositions := Assemble(anchor, candidates, budget)

	if !dispositions[0].Included {
		t.Fatal("candidate exactly at the byte budget was cut, want it included — admission is <=, not <")
	}
	if dispositions[0].CutReason != "" {
		t.Fatalf("CutReason = %q for a candidate exactly at budget, want empty", dispositions[0].CutReason)
	}
}

// TestAssembleDoesNotChargeTheAnchorBodyAgainstTheBudget pins W-2 (design
// R4): the anchor is exempt from the byte budget. A large anchor body must
// not eat into the budget candidates are admitted against.
func TestAssembleDoesNotChargeTheAnchorBodyAgainstTheBudget(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Content: strings.Repeat("a", 1_000)}
	candidates := []Candidate{
		{ID: 10, Content: strings.Repeat("x", 50)},
	}
	const budget = 50 // fits the candidate alone; would already be blown if the anchor counted

	_, dispositions := Assemble(anchor, candidates, budget)

	if !dispositions[0].Included {
		t.Fatal("candidate was cut even though it alone fits the budget — the anchor body must be exempt from the budget (design R4)")
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

	_, dispositions := Assemble(anchor, candidates, 60_000)
	anchorSummary := summarizeAnchor(anchor)

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
	candidates := []Candidate{{ID: 10, Content: strings.Repeat("x", 200)}}
	const budget = 10

	_, dispositions := Assemble(anchor, candidates, budget)

	const want = "byte budget exceeded"
	if dispositions[0].CutReason != want {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, want)
	}
}

func TestAssembleSelfProducedCutReasonHasTheExactWording(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1}
	candidates := []Candidate{{ID: 10, Content: "small", SelfProduced: true}}

	_, dispositions := Assemble(anchor, candidates, 60_000)

	const want = "self-produced"
	if dispositions[0].CutReason != want {
		t.Fatalf("CutReason = %q, want %q", dispositions[0].CutReason, want)
	}
}

func TestAssembleEmptyCandidatesRendersAnchorOnly(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "t", Name: "solo", Content: "just the anchor"}

	block, dispositions := Assemble(anchor, nil, 60_000)

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
