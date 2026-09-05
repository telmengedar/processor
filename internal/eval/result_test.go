package eval

import (
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

func anchorNode() loop.Anchor {
	return loop.Anchor{ID: 100, Type: "documentation", Name: "Subject", Content: "the subject body"}
}

func labelledRow(required ...Required) Row {
	return Row{ID: "r01", Input: "an input", Subject: 100, Stratum: StratumLabelled, Required: required}
}

func TestSweepReportsAShutoutWhenEveryCandidateWasOversized(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 200, Content: strings.Repeat("x", loop.AssemblyByteBudget+1)},
		{ID: 201, Content: strings.Repeat("y", loop.AssemblyByteBudget+1)},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 201, Hash: "h", Why: "w"}), dispositions)

	if !got.Shutout {
		t.Fatal("Shutout is false where two candidates were retrieved and none admitted, want true")
	}
	if got.CandidateCount != 2 {
		t.Fatalf("CandidateCount = %d, want 2", got.CandidateCount)
	}
	if got.AdmittedCount != 0 {
		t.Fatalf("AdmittedCount = %d, want 0", got.AdmittedCount)
	}
	if got.Required[0].Verdict != Cut {
		t.Fatalf("Verdict = %q, want %q rather than a retrieval failure", got.Required[0].Verdict, Cut)
	}
}

func TestSweepReportsNoShutoutWhenTheCandidateSetItselfWasEmpty(t *testing.T) {
	t.Parallel()

	_, dispositions := loop.Assemble(anchorNode(), nil, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 201, Hash: "h", Why: "w"}), dispositions)

	if got.Shutout {
		t.Fatal("Shutout is true where recall returned nothing at all, want false: that is a retrieval failure and not an admission one")
	}
}

func TestSweepRecordsThatTheAnchorAlsoAppearedAmongTheCandidates(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 100, Content: "the subject body"},
		{ID: 201, Content: "another body"},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 201, Hash: "h", Why: "w"}), dispositions)

	if !got.AnchorWasCandidate {
		t.Fatal("AnchorWasCandidate is false where the subject id was also candidate rank 1, want true")
	}
	if !got.AnchorAdmittedAsCandidate {
		t.Fatal("AnchorAdmittedAsCandidate is false where that candidate was admitted, want true")
	}
}

func TestSweepRecordsAnAnchorCandidateTheBudgetCutAsPresentButNotAdmitted(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 201, Content: strings.Repeat("x", loop.AssemblyByteBudget)},
		{ID: 100, Content: "the subject body"},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 201, Hash: "h", Why: "w"}), dispositions)

	if !got.AnchorWasCandidate {
		t.Fatal("AnchorWasCandidate is false where the subject id appeared at rank 2, want true")
	}
	if got.AnchorAdmittedAsCandidate {
		t.Fatal("AnchorAdmittedAsCandidate is true where the budget cut that candidate, want false")
	}
}

func TestSweepRecordsNoAnchorDuplicationWhenTheSubjectIsNotAmongTheCandidates(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{{ID: 201, Content: "another body"}}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 201, Hash: "h", Why: "w"}), dispositions)

	if got.AnchorWasCandidate || got.AnchorAdmittedAsCandidate {
		t.Fatalf("anchor duplication reported as %v/%v where the subject was not a candidate, want false/false",
			got.AnchorWasCandidate, got.AnchorAdmittedAsCandidate)
	}
}

func TestSweepRecordsTheAdmittedByteTotalBesideTheBudget(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 200, Content: "aaaa"},
		{ID: 201, Content: "bbbbb"},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 200, Hash: "h", Why: "w"}), dispositions)

	if got.AdmittedBytes != 9 {
		t.Fatalf("AdmittedBytes = %d, want 9 for a four-byte and a five-byte body", got.AdmittedBytes)
	}
	if got.BudgetBytes != loop.AssemblyByteBudget {
		t.Fatalf("BudgetBytes = %d, want the assembly byte budget %d", got.BudgetBytes, loop.AssemblyByteBudget)
	}
	if got.AdmittedCount != 2 {
		t.Fatalf("AdmittedCount = %d, want 2", got.AdmittedCount)
	}
}

func TestSweepCountsOnlyRunRecordsAsSelfProducedAndNotOtherSessionLogs(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 200, Type: divoid.RunNodeType, Name: divoid.RunNamePrefix + " 2026-09-02T10:00:00Z what changed", Content: "a"},
		{ID: 201, Type: divoid.RunNodeType, Name: "a session log another agent wrote", Content: "b"},
		{ID: 202, Type: "documentation", Name: divoid.RunNamePrefix + " lookalike", Content: "c"},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 200, Hash: "h", Why: "w"}), dispositions)

	if got.SelfProducedCandidates != 1 {
		t.Fatalf("SelfProducedCandidates = %d, want 1: the node type and the name prefix must both match", got.SelfProducedCandidates)
	}
}

func TestSweepRecordsTheTopSimilarityOfTheCandidateSet(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 200, Similarity: 0.6388, Content: "a"},
		{ID: 201, Similarity: 0.6704, Content: "b"},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 200, Hash: "h", Why: "w"}), dispositions)

	if got.TopSimilarity != 0.6704 {
		t.Fatalf("TopSimilarity = %v, want 0.6704", got.TopSimilarity)
	}
}

func TestResultHeaderCarriesTheCorpusHashAndTheLoopLimits(t *testing.T) {
	t.Parallel()

	corpus := Corpus{Hash: "a-corpus-hash", Rows: []Row{labelledRow(), labelledRow()}}
	sweptAt := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

	got := NewResult(corpus, Derivations{}, sweptAt)

	if got.CorpusHash != "a-corpus-hash" {
		t.Fatalf("CorpusHash = %q, want the hash of the corpus that produced the result", got.CorpusHash)
	}
	if !got.SweptAt.Equal(sweptAt) {
		t.Fatalf("SweptAt = %v, want %v", got.SweptAt, sweptAt)
	}
	if got.RowCount != 2 {
		t.Fatalf("RowCount = %d, want 2", got.RowCount)
	}
	if got.Limits.CandidateLimit != loop.CandidateLimit {
		t.Fatalf("Limits.CandidateLimit = %d, want the candidate limit the loop ships %d", got.Limits.CandidateLimit, loop.CandidateLimit)
	}
	if got.Limits.AssemblyByteBudget != loop.AssemblyByteBudget {
		t.Fatalf("Limits.AssemblyByteBudget = %d, want the byte budget the loop ships %d", got.Limits.AssemblyByteBudget, loop.AssemblyByteBudget)
	}
	if got.Limits.RecallScopeReserve != loop.RecallScopeReserve {
		t.Fatalf("Limits.RecallScopeReserve = %d, want the scope reserve the loop ships %d: two sweeps taken under different reserves are two arms, and a result that does not carry the reserve cannot say which one it is", got.Limits.RecallScopeReserve, loop.RecallScopeReserve)
	}
	if len(got.Rows) != 0 {
		t.Fatalf("len(Rows) = %d, want an empty slice a sweep appends to in corpus order", len(got.Rows))
	}
}

func TestNewResultCarriesTheHandAuthoredAndBlindGeneratedRowCountsAlongsideTheDerivedOne(t *testing.T) {
	t.Parallel()

	corpus := Corpus{Hash: "a-corpus-hash", Rows: []Row{
		{ID: "r01", Input: "an input", Subject: 100, Stratum: StratumLabelled},
		{ID: "r02", Input: "another input", Subject: 101, Stratum: StratumLabelled},
		{ID: "r03", Input: "a third input", Subject: 103, Stratum: StratumLabelled},
	}}
	derivations := Derivations{
		Path:    "internal/eval/derivations.json",
		Hash:    "a-derivation-hash",
		Queries: map[string][]string{"r01": {"q1"}, "r02": {"q2"}, "r03": {"q3"}},
		Sources: map[string]string{"r01": SourceHandAuthored, "r02": SourceHandAuthored, "r03": SourceBlindGenerated},
	}

	got := NewResult(corpus, derivations, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	if got.DerivedRows != 3 {
		t.Fatalf("DerivedRows = %d, want 3", got.DerivedRows)
	}
	if got.HandAuthoredRows != 2 {
		t.Fatalf("HandAuthoredRows = %d, want 2: r01 and r02 are marked hand-authored -- a fixture with equal hand-authored and blind-generated counts cannot tell this field apart from one wired to the wrong source", got.HandAuthoredRows)
	}
	if got.BlindGeneratedRows != 1 {
		t.Fatalf("BlindGeneratedRows = %d, want 1: only r03 is marked blind-generated", got.BlindGeneratedRows)
	}
	if !got.ProvenanceRecorded {
		t.Fatal("ProvenanceRecorded = false, want true: every row names a source")
	}
}

func TestNewResultLeavesProvenanceUnrecordedWhenTheSidecarNamesNoSource(t *testing.T) {
	t.Parallel()

	corpus := Corpus{Hash: "a-corpus-hash", Rows: []Row{
		{ID: "r01", Input: "an input", Subject: 100, Stratum: StratumLabelled},
	}}
	derivations := Derivations{
		Path:    "internal/eval/derivations.json",
		Hash:    "a-derivation-hash",
		Queries: map[string][]string{"r01": {"q1"}},
		Sources: map[string]string{"r01": ""},
	}

	got := NewResult(corpus, derivations, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	if got.ProvenanceRecorded {
		t.Fatal("ProvenanceRecorded = true, want false: r01 names no source")
	}
	if got.HandAuthoredRows != 0 || got.BlindGeneratedRows != 0 {
		t.Fatalf("HandAuthoredRows/BlindGeneratedRows = %d/%d, want 0/0 alongside ProvenanceRecorded=false", got.HandAuthoredRows, got.BlindGeneratedRows)
	}
}

func TestARowIsNotScoredWhenOneOfItsRequiredNodesNoLongerResolves(t *testing.T) {
	t.Parallel()

	row := RowResult{Required: []NodeResult{{Node: 1, Verdict: Admitted}, {Node: 2, Verdict: Unresolved}}}

	if row.Scored() {
		t.Fatal("Scored is true for a row carrying an unresolved required node, want false")
	}
}

func TestARowIsNotScoredWhenTheSweepCouldNotFetchItsSubject(t *testing.T) {
	t.Parallel()

	row := RowResult{Error: "subject not found"}

	if row.Scored() {
		t.Fatal("Scored is true for a row that errored, want false")
	}
}

func controlStratumWhoseSecondNodeScored(verdict Verdict) Result {
	return Result{Rows: []RowResult{
		{Stratum: StratumControl, Required: []NodeResult{{Node: 1, Verdict: Admitted}}},
		{Stratum: StratumControl, Required: []NodeResult{{Node: 2, Verdict: verdict}}},
	}}
}

func TestControlVerifiedRetrievalIsFalseWhenTheCorpusCarriesNoControlRowAtAll(t *testing.T) {
	t.Parallel()

	result := Result{Rows: []RowResult{{Stratum: StratumLabelled, Required: []NodeResult{{Node: 1, Verdict: Admitted}}}}}

	if result.ControlVerifiedRetrieval() {
		t.Fatal("ControlVerifiedRetrieval is true for a sweep with no control stratum, want false: an absent self-check is not a passing one")
	}
}

func TestControlVerifiedRetrievalIsTrueWhenTheBudgetCutAControlNodeTheRetrieverSurfaced(t *testing.T) {
	t.Parallel()

	result := controlStratumWhoseSecondNodeScored(Cut)

	if !result.ControlVerifiedRetrieval() {
		t.Fatal("ControlVerifiedRetrieval is false where a control node was retrieved and cut, want true: the budget is not the retriever")
	}
}

func TestControlVerifiedRetrievalIsFalseWhenAControlNodeWasNeverRetrieved(t *testing.T) {
	t.Parallel()

	result := controlStratumWhoseSecondNodeScored(NotRetrieved)

	if result.ControlVerifiedRetrieval() {
		t.Fatal("ControlVerifiedRetrieval is true where a control node reached no candidate row, want false: retrieval is the ceiling under every number in the sweep")
	}
}

func TestControlVerifiedRetrievalIsFalseWhenAControlRowCarriesAnUnresolvedRequiredNode(t *testing.T) {
	t.Parallel()

	result := controlStratumWhoseSecondNodeScored(Unresolved)

	if result.ControlVerifiedRetrieval() {
		t.Fatal("ControlVerifiedRetrieval is true where a control row could not be scored at all, want false: an absent self-check is not a passing one")
	}
}

func TestControlVerifiedRetrievalIsTrueWhenEveryControlRowAdmittedEveryRequiredNode(t *testing.T) {
	t.Parallel()

	result := Result{Rows: []RowResult{
		{Stratum: StratumControl, Required: []NodeResult{{Node: 1, Verdict: Admitted}}},
		{Stratum: StratumLabelled, Required: []NodeResult{{Node: 2, Verdict: NotRetrieved}}},
	}}

	if !result.ControlVerifiedRetrieval() {
		t.Fatal("ControlVerifiedRetrieval is false where every control node was admitted, want true: a labelled miss must not break the self-check")
	}
}

func TestSweepRetainsEveryCandidateItSawInRankOrderIncludingTheOnesTheBudgetCut(t *testing.T) {
	t.Parallel()

	candidates := []loop.Candidate{
		{ID: 401, Similarity: 0.62, Content: "aaa"},
		{ID: 402, Similarity: 0.91, Content: "bbbb"},
		{ID: 403, Similarity: 0.77, Content: strings.Repeat("x", loop.AssemblyByteBudget)},
		{ID: 404, Similarity: 0.48, Content: strings.Repeat("c", loop.AssemblyByteBudget)},
	}
	_, dispositions := loop.Assemble(anchorNode(), candidates, loop.AssemblyByteBudget)

	got := BuildRow(labelledRow(Required{Node: 403, Hash: "h", Why: "w"}), dispositions)

	if len(got.Candidates) != 4 {
		t.Fatalf("len(Candidates) = %d, want 4: the row retained %d of the 4 candidates the sweep produced, and AdmittedCount is %d",
			len(got.Candidates), len(got.Candidates), got.AdmittedCount)
	}

	wantID := []int64{401, 402, 403, 404}
	wantSimilarity := []float64{0.62, 0.91, 0.77, 0.48}
	wantIncluded := []bool{true, true, false, false}
	for i := range wantID {
		if got.Candidates[i].ID != wantID[i] || got.Candidates[i].Rank != i+1 {
			t.Fatalf("Candidates[%d] is #%d at rank %d, want #%d at rank %d: the retained set must keep the rank order the graph returned rather than any re-sort of it",
				i, got.Candidates[i].ID, got.Candidates[i].Rank, wantID[i], i+1)
		}
		if got.Candidates[i].Similarity != wantSimilarity[i] {
			t.Fatalf("Candidates[%d].Similarity = %v, want %v: the similarity that ranked the candidate must be retained beside its id",
				i, got.Candidates[i].Similarity, wantSimilarity[i])
		}
		if got.Candidates[i].Included != wantIncluded[i] {
			t.Fatalf("Candidates[%d] (#%d) has Included = %v, want %v: a candidate the byte budget cut is retained exactly as an admitted one is",
				i, got.Candidates[i].ID, got.Candidates[i].Included, wantIncluded[i])
		}
	}
}
