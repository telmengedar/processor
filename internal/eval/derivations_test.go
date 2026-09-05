package eval

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const validDerivations = `[
  {"row": "r01", "queries": ["why would a mutation leave the suite green", "what makes an assertion unfalsifiable"]}
]`

func derivationCorpus() Corpus {
	return Corpus{Hash: "a-corpus-hash", Rows: []Row{
		{ID: "r01", Input: "the first input", Subject: 100, Stratum: StratumLabelled},
		{ID: "r02", Input: "the second input", Subject: 101, Stratum: StratumLabelled},
	}}
}

func writeDerivations(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "derivations.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func loadDerivationsMustFail(t *testing.T, body, wantSubstring string) {
	t.Helper()

	derivations, err := LoadDerivations(writeDerivations(t, body), derivationCorpus())
	if err == nil {
		t.Fatalf("LoadDerivations returned a nil error and %d rows, want an error containing %q", derivations.Rows(), wantSubstring)
	}
	if derivations.Rows() != 0 || derivations.Hash != "" {
		t.Fatalf("LoadDerivations returned %d rows and hash %q alongside an error, want a zero sidecar so a caller ignoring the error sweeps the raw arm rather than a half-read one", derivations.Rows(), derivations.Hash)
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), wantSubstring)
	}
}

func TestLoadDerivationsReturnsThePinnedQueriesAndTheSha256OfTheBytesTheyWereReadFrom(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	const wantHash = "45d2537b87bb6f780dd6840199e14420e5de5b57775a74d3eaf3e27e29a54e86"
	if derivations.Hash != wantHash {
		t.Fatalf("Hash = %q, want %q: two sweeps taken on two derivation sets are two arms, and a reading that cannot name which set produced it cannot be compared with anything later", derivations.Hash, wantHash)
	}
	if derivations.Rows() != 1 {
		t.Fatalf("Rows = %d, want 1", derivations.Rows())
	}
}

func TestQueriesForIsTheRowsOwnInputAheadOfEveryQueryPinnedForIt(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	got := derivations.QueriesFor(derivationCorpus().Rows[0])

	want := []string{"the first input", "why would a mutation leave the suite green", "what makes an assertion unfalsifiable"}
	if !slices.Equal(got, want) {
		t.Fatalf("QueriesFor = %q, want %q: the raw input is what the baseline ranked, so an arm that drops it in favour of its derivations is measuring a replacement rather than an addition", got, want)
	}
}

func TestQueriesForIsTheRowsInputAloneForARowTheSidecarPinnedNothingFor(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	got := derivations.QueriesFor(derivationCorpus().Rows[1])

	want := []string{"the second input"}
	if !slices.Equal(got, want) {
		t.Fatalf("QueriesFor = %q, want %q", got, want)
	}
}

func TestQueriesForOnASidecarThatWasNeverLoadedIsStillTheRowsOwnInput(t *testing.T) {
	t.Parallel()

	got := (Derivations{}).QueriesFor(derivationCorpus().Rows[0])

	want := []string{"the first input"}
	if !slices.Equal(got, want) {
		t.Fatalf("QueriesFor on the zero sidecar = %q, want %q: absent a sidecar every row sweeps on its own input, and a zero value that yields nothing turns the arm nobody asked for into a sweep that queries the graph with nothing", got, want)
	}
}

func TestQueriesForDropsAPinnedQueryThatOnlyRepeatsTheRowsOwnInput(t *testing.T) {
	t.Parallel()

	body := `[{"row": "r01", "queries": ["the first input", "a genuinely different question"]}]`
	derivations, err := LoadDerivations(writeDerivations(t, body), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	got := derivations.QueriesFor(derivationCorpus().Rows[0])

	want := []string{"the first input", "a genuinely different question"}
	if !slices.Equal(got, want) {
		t.Fatalf("QueriesFor = %q, want %q: issuing one query twice returns one list twice, and summing its reciprocal ranks twice doubles that list's weight against every other", got, want)
	}
}

func TestTheArmIsTheSidecarPathAndTheRawInputArmWhereThereIsNoSidecar(t *testing.T) {
	t.Parallel()

	path := writeDerivations(t, validDerivations)
	derivations, err := LoadDerivations(path, derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	if derivations.Arm() != path {
		t.Fatalf("Arm = %q, want the sidecar path %q", derivations.Arm(), path)
	}
	if (Derivations{}).Arm() != "raw-input" {
		t.Fatalf("the zero sidecar's Arm = %q, want %q", (Derivations{}).Arm(), "raw-input")
	}
}

func TestLoadDerivationsRejectsARowIdNoCorpusRowCarries(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[{"row": "r99", "queries": ["a question"]}]`, "r99")
}

func TestLoadDerivationsRejectsASecondEntryForOneRow(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[{"row": "r01", "queries": ["a"]}, {"row": "r01", "queries": ["b"]}]`, "not unique")
}

func TestLoadDerivationsRejectsARowCarryingNoQueryAtAll(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[{"row": "r01", "queries": []}]`, "no queries")
}

func TestLoadDerivationsRejectsABlankQuery(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[{"row": "r01", "queries": ["a question", "   "]}]`, "blank")
}

func TestLoadDerivationsRejectsAQueryRepeatedInsideOneRow(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[{"row": "r01", "queries": ["a question", "a question"]}]`, "repeats")
}

func TestLoadDerivationsRejectsASidecarHoldingNoRows(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[]`, "no rows")
}

func TestUnpinnedNamesTheCorpusRowsTheSidecarDoesNotPin(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	got := derivations.Unpinned(derivationCorpus())

	want := []string{"r02"}
	if !slices.Equal(got, want) {
		t.Fatalf("Unpinned = %q, want %q: LoadDerivations only checks that every sidecar row resolves to a corpus row, so a corpus row the sidecar never mentions passes validation silently and sweeps on raw input alone", got, want)
	}
}

func TestUnpinnedPreservesCorpusOrderWhenMultipleRowsAreUnpinned(t *testing.T) {
	t.Parallel()

	corpus := Corpus{Hash: "a-corpus-hash", Rows: []Row{
		{ID: "r01", Input: "the first input", Subject: 100, Stratum: StratumLabelled},
		{ID: "r02", Input: "the second input", Subject: 101, Stratum: StratumLabelled},
		{ID: "r03", Input: "the third input", Subject: 102, Stratum: StratumLabelled},
		{ID: "r04", Input: "the fourth input", Subject: 103, Stratum: StratumLabelled},
	}}
	body := `[{"row": "r02", "queries": ["a question about the second row"]}]`
	derivations, err := LoadDerivations(writeDerivations(t, body), corpus)
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	got := derivations.Unpinned(corpus)

	want := []string{"r01", "r03", "r04"}
	if !slices.Equal(got, want) {
		t.Fatalf("Unpinned = %q, want %q in corpus order", got, want)
	}
}

func TestUnpinnedIsEmptyWhenTheSidecarCoversEveryCorpusRow(t *testing.T) {
	t.Parallel()

	corpus := Corpus{Hash: "a-corpus-hash", Rows: []Row{
		{ID: "r01", Input: "the first input", Subject: 100, Stratum: StratumLabelled},
		{ID: "r02", Input: "the second input", Subject: 101, Stratum: StratumLabelled},
		{ID: "r03", Input: "the third input", Subject: 102, Stratum: StratumLabelled},
	}}
	body := `[
	  {"row": "r01", "queries": ["a question about the first row"]},
	  {"row": "r02", "queries": ["a question about the second row"]},
	  {"row": "r03", "queries": ["a question about the third row"]}
	]`
	derivations, err := LoadDerivations(writeDerivations(t, body), corpus)
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}
	if got := derivations.Rows(); got != len(corpus.Rows) {
		t.Fatalf("the fixture pins %d of %d rows, want all of them: this test's assertion only means something if the sidecar genuinely covers the whole corpus", got, len(corpus.Rows))
	}

	if got := derivations.Unpinned(corpus); !slices.Equal(got, []string{}) {
		t.Fatalf("Unpinned = %q, want none: every corpus row carries a pinned query set", got)
	}
}

func TestUnpinnedOnTheZeroSidecarReportsNoGapBecauseTheRawInputArmPinsNothingByDefinition(t *testing.T) {
	t.Parallel()

	if got := (Derivations{}).Unpinned(derivationCorpus()); got != nil {
		t.Fatalf("Unpinned on the zero sidecar = %q, want nil: the raw-input arm never pins any row, and that is not a coverage gap to warn about", got)
	}
}

func TestLoadDerivationsReportsTheAbsentFileRatherThanSweepingTheRawArmUnderItsName(t *testing.T) {
	t.Parallel()

	_, err := LoadDerivations(filepath.Join(t.TempDir(), "absent.json"), derivationCorpus())
	if err == nil {
		t.Fatal("LoadDerivations returned a nil error for a path that holds no file, want the read failure")
	}
}

func TestLoadDerivationsAcceptsARowThatNamesNoSource(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}
	if derivations.Rows() != 1 {
		t.Fatalf("Rows = %d, want 1: an absent source must not be treated as an invalid row", derivations.Rows())
	}
}

func TestLoadDerivationsRejectsASourceOutsideTheClosedSet(t *testing.T) {
	t.Parallel()

	loadDerivationsMustFail(t, `[{"row": "r01", "queries": ["a question"], "source": "guessed"}]`, "closed set")
}

func asymmetricSourceCorpus() Corpus {
	return Corpus{Hash: "a-corpus-hash", Rows: []Row{
		{ID: "r01", Input: "the first input", Subject: 100, Stratum: StratumLabelled},
		{ID: "r02", Input: "the second input", Subject: 101, Stratum: StratumLabelled},
		{ID: "r03", Input: "the third input", Subject: 103, Stratum: StratumLabelled},
		{ID: "c01", Input: "the control input", Subject: 102, Stratum: StratumControl},
	}}
}

func loadAsymmetricSourceDerivations(t *testing.T) (Derivations, Corpus) {
	t.Helper()

	corpus := asymmetricSourceCorpus()
	body := `[
	  {"row": "r01", "queries": ["a question"], "source": "hand-authored"},
	  {"row": "r02", "queries": ["another question"], "source": "hand-authored"},
	  {"row": "r03", "queries": ["a third question"], "source": "blind-generated"},
	  {"row": "c01", "queries": ["a control question"], "source": "hand-authored"}
	]`
	derivations, err := LoadDerivations(writeDerivations(t, body), corpus)
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}
	return derivations, corpus
}

func TestHandAuthoredRowsCountsOnlyLabelledRowsTheSidecarMarksHandAuthored(t *testing.T) {
	t.Parallel()

	derivations, corpus := loadAsymmetricSourceDerivations(t)

	if got := derivations.HandAuthoredRows(corpus); got != 2 {
		t.Fatalf("HandAuthoredRows = %d, want 2: c01 is hand-authored but not labelled, r03 is labelled but blind-generated, and only r01/r02 are both -- a fixture with equal hand-authored and blind-generated counts cannot tell this function apart from one that counts the wrong source, which is why the two counts here differ", got)
	}
}

func TestBlindGeneratedRowsCountsOnlyLabelledRowsTheSidecarMarksBlindGenerated(t *testing.T) {
	t.Parallel()

	derivations, corpus := loadAsymmetricSourceDerivations(t)

	if got := derivations.BlindGeneratedRows(corpus); got != 1 {
		t.Fatalf("BlindGeneratedRows = %d, want 1: only r03 is labelled and blind-generated", got)
	}
}

func TestBlindGeneratedRowsIsZeroWhenNoRowNamesASource(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	if got := derivations.BlindGeneratedRows(derivationCorpus()); got != 0 {
		t.Fatalf("BlindGeneratedRows = %d, want 0: a sidecar entry that names no source cannot be counted as blind-generated", got)
	}
}

func TestBlindGeneratedRowsOnTheZeroSidecarIsZero(t *testing.T) {
	t.Parallel()

	if got := (Derivations{}).BlindGeneratedRows(derivationCorpus()); got != 0 {
		t.Fatalf("BlindGeneratedRows on the zero sidecar = %d, want 0", got)
	}
}

func TestSourcesRecordedIsTrueWhenAnyLabelledRowNamesASource(t *testing.T) {
	t.Parallel()

	derivations, corpus := loadAsymmetricSourceDerivations(t)

	if !derivations.SourcesRecorded(corpus) {
		t.Fatal("SourcesRecorded = false, want true: r01, r02 and r03 all name a source")
	}
}

func TestSourcesRecordedIsFalseWhenNoRowNamesASource(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	if derivations.SourcesRecorded(derivationCorpus()) {
		t.Fatal("SourcesRecorded = true, want false: validDerivations names no source for r01")
	}
}

func TestSourcesRecordedOnTheZeroSidecarIsFalse(t *testing.T) {
	t.Parallel()

	if (Derivations{}).SourcesRecorded(derivationCorpus()) {
		t.Fatal("SourcesRecorded on the zero sidecar = true, want false")
	}
}

func TestRealSidecarCarriesElevenHandAuthoredAndTwelveBlindGeneratedLabelledRows(t *testing.T) {
	t.Parallel()

	corpus, err := Load("corpus.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	derivations, err := LoadDerivations("derivations.json", corpus)
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	if got := derivations.Rows(); got != 25 {
		t.Fatalf("Rows = %d, want 25: the real sidecar's coverage has drifted from 25/25 -- this test guards the composition claim, not just its shape, so update it deliberately if coverage genuinely changed", got)
	}
	if got := derivations.HandAuthoredRows(corpus); got != 11 {
		t.Fatalf("HandAuthoredRows = %d, want 11: r01-r11 are the hand-authored labelled rows the combiner was selected against -- if this moved, either the real sidecar's provenance was edited or the counting logic broke, and a green suite must not hide either", got)
	}
	if got := derivations.BlindGeneratedRows(corpus); got != 12 {
		t.Fatalf("BlindGeneratedRows = %d, want 12: r12-r23 are the blind-generated labelled rows added after the design was selected", got)
	}
}

func TestHandAuthoredRowsIsZeroWhenNoRowNamesASource(t *testing.T) {
	t.Parallel()

	derivations, err := LoadDerivations(writeDerivations(t, validDerivations), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	if got := derivations.HandAuthoredRows(derivationCorpus()); got != 0 {
		t.Fatalf("HandAuthoredRows = %d, want 0: a sidecar entry that names no source cannot be counted as hand-authored", got)
	}
}

func TestHandAuthoredRowsOnTheZeroSidecarIsZero(t *testing.T) {
	t.Parallel()

	if got := (Derivations{}).HandAuthoredRows(derivationCorpus()); got != 0 {
		t.Fatalf("HandAuthoredRows on the zero sidecar = %d, want 0", got)
	}
}
