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

	// validDerivations pins only r01; derivationCorpus also carries r02.
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

func TestUnpinnedIsEmptyWhenTheSidecarCoversEveryCorpusRow(t *testing.T) {
	t.Parallel()

	body := `[
	  {"row": "r01", "queries": ["a question about the first row"]},
	  {"row": "r02", "queries": ["a question about the second row"]}
	]`
	derivations, err := LoadDerivations(writeDerivations(t, body), derivationCorpus())
	if err != nil {
		t.Fatalf("LoadDerivations: %v", err)
	}

	if got := derivations.Unpinned(derivationCorpus()); len(got) != 0 {
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
