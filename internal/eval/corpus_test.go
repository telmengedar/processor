package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const validCorpus = `[
  {
    "id": "r01",
    "input": "what does the boot split change for a graph-only caller",
    "subject": 100,
    "stratum": "labelled",
    "required": [
      {"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}
    ]
  }
]`

func writeCorpus(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func loadMustFail(t *testing.T, body, wantSubstring string) {
	t.Helper()

	corpus, err := Load(writeCorpus(t, body))
	if err == nil {
		t.Fatalf("Load returned a nil error and %d rows, want an error containing %q", len(corpus.Rows), wantSubstring)
	}
	if len(corpus.Rows) != 0 {
		t.Fatalf("Load returned %d rows alongside an error, want none", len(corpus.Rows))
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), wantSubstring)
	}
}

func TestLoadReturnsTheRowsAndTheSha256OfTheBytesTheyWereReadFrom(t *testing.T) {
	t.Parallel()

	corpus, err := Load(writeCorpus(t, validCorpus))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	const wantHash = "b03ac6b57d9bb9c19983c908719b825ba866b91f7fbb941102fb0ce8a5db541a"
	if corpus.Hash != wantHash {
		t.Fatalf("Hash = %q, want %q", corpus.Hash, wantHash)
	}
	if len(corpus.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(corpus.Rows))
	}

	row := corpus.Rows[0]
	if row.ID != "r01" || row.Subject != 100 || row.Stratum != StratumLabelled {
		t.Fatalf("row = %+v, want id r01, subject 100, stratum labelled", row)
	}
	if row.Input != "what does the boot split change for a graph-only caller" {
		t.Fatalf("Input = %q, want the input verbatim", row.Input)
	}
	if len(row.Required) != 1 || row.Required[0].Node != 200 || row.Required[0].Hash != "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6" {
		t.Fatalf("Required = %+v, want one entry for node 200 with the sha256 it was labelled at", row.Required)
	}
	if row.Required[0].Why == "" {
		t.Fatal("Required[0].Why is empty, want the labeller's reason carried through")
	}
}

func TestLoadRejectsARowWhoseRequiredSetContainsItsOwnSubject(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `"node": 200`, `"node": 100`, 1)

	loadMustFail(t, body, "own subject")
}

func TestLoadRejectsARowRequiringMoreThanThreeNodes(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus,
		`{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}`,
		`{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "w"},
      {"node": 201, "hash": "b58c4e192231190d6da9f48db2b2dadafd336e96f2bc3cfc7195f90781aa5717", "why": "w"},
      {"node": 202, "hash": "c6ee9b4217c6ef2558cfac355cb79ba53f9fd06101e45af03f3ff8ffcca7d8bc", "why": "w"},
      {"node": 203, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "w"}`, 1)

	loadMustFail(t, body, "required nodes")
}

func TestLoadRejectsARowRequiringNoNodesAtAll(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus,
		`{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}`,
		``, 1)

	loadMustFail(t, body, "required nodes")
}

func TestLoadRejectsARequiredEntryMissingItsReasonOrItsHash(t *testing.T) {
	t.Parallel()

	missingHash := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`, `"hash": ""`, 1)
	loadMustFail(t, missingHash, "carries no hash")

	missingWhy := strings.Replace(validCorpus,
		`"why": "an answer that omits it cannot name the loader a graph-only caller uses"`,
		`"why": ""`, 1)
	loadMustFail(t, missingWhy, "carries no reason")
}

func TestLoadOnAMalformedFileReturnsAnErrorAndNoRows(t *testing.T) {
	t.Parallel()

	loadMustFail(t, `[{"id": "r01", `, "decode corpus")
}

func TestLoadOnAnEmptyCorpusReturnsAnErrorNotAnEmptySweep(t *testing.T) {
	t.Parallel()

	loadMustFail(t, `[]`, "the corpus is empty")
}

func TestLoadRejectsARowWhoseStratumIsOutsideTheClosedSet(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `"stratum": "labelled"`, `"stratum": "adversarial"`, 1)

	loadMustFail(t, body, "outside the closed set")
}

func TestLoadRejectsTwoRowsSharingOneID(t *testing.T) {
	t.Parallel()

	oneRow := strings.TrimPrefix(strings.TrimSuffix(validCorpus, "]"), "[")
	body := "[" + oneRow + "," + oneRow + "]"

	loadMustFail(t, body, "not unique")
}

func TestLoadRejectsARowWithAnEmptyInput(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `"input": "what does the boot split change for a graph-only caller"`, `"input": ""`, 1)

	loadMustFail(t, body, "input is empty")
}

func TestLoadRejectsARowWithoutASubject(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `"subject": 100`, `"subject": 0`, 1)

	loadMustFail(t, body, "is not a node id")
}

func TestLoadRejectsARowWithoutAnID(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `"id": "r01"`, `"id": ""`, 1)

	loadMustFail(t, body, "carries no id")
}

func TestLoadAcceptsAControlRowAsWellAsALabelledOne(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `"stratum": "labelled"`, `"stratum": "control"`, 1)

	corpus, err := Load(writeCorpus(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if corpus.Rows[0].Stratum != StratumControl {
		t.Fatalf("Stratum = %q, want %q", corpus.Rows[0].Stratum, StratumControl)
	}
}

func TestLoadReturnsAnErrorWhenTheCorpusFileIsMissing(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil {
		t.Fatal("Load returned a nil error for a missing file, want an error")
	}
	if !strings.Contains(err.Error(), "read corpus") {
		t.Fatalf("error = %q, want it to name the read that failed", err.Error())
	}
}

func TestLoadRejectsARequiredHashThatIsNotLowercaseSha256Hex(t *testing.T) {
	t.Parallel()

	tooShort := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`, `"hash": "0f0f"`, 1)
	loadMustFail(t, tooShort, "not lowercase sha256 hex")

	outsideTheAlphabet := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`,
		`"hash": "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"`, 1)
	loadMustFail(t, outsideTheAlphabet, "not lowercase sha256 hex")

	uppercase := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`,
		`"hash": "5DA7E2B28E10A231E97B202DD241D9DF0E4A897AC6F5CCB5169C0B8492908CD6"`, 1)
	loadMustFail(t, uppercase, "not lowercase sha256 hex")

	sixtyThree := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`,
		`"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd"`, 1)
	loadMustFail(t, sixtyThree, "not lowercase sha256 hex")

	sha512Digest := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`,
		`"hash": "72441e95f1e8587430198a6c6f2fa6134da4d7278b7efcc5bcff4af657f78d99196a4a42ebbf71dec6bc1421a84adbb6a0ffb653a2905bf93f2a9a1790481df7"`, 1)
	loadMustFail(t, sha512Digest, "not lowercase sha256 hex")
}

func TestLoadAcceptsAsARequiredHashTheContentHashAssembleProduces(t *testing.T) {
	t.Parallel()

	_, dispositions := loop.Assemble(
		loop.Anchor{ID: 100, Type: "documentation", Name: "the anchor", Content: "the anchor body"},
		[]loop.Candidate{{ID: 200, Type: "documentation", Name: "the required node", Content: "the body it carried when it was labelled"}},
		1000)
	produced := dispositions[0].ContentHash

	body := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`, `"hash": "`+produced+`"`, 1)

	corpus, err := Load(writeCorpus(t, body))
	if err != nil {
		t.Fatalf("Load rejected the content hash the sweep compares against: %v", err)
	}
	if corpus.Rows[0].Required[0].Hash != produced {
		t.Fatalf("Hash = %q, want the produced content hash %q carried through", corpus.Rows[0].Required[0].Hash, produced)
	}
}

func TestLoadAcceptsAnAllDigitAndAnAllLetterSha256Hash(t *testing.T) {
	t.Parallel()

	for _, hash := range []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	} {
		body := strings.Replace(validCorpus, `"hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6"`, `"hash": "`+hash+`"`, 1)

		corpus, err := Load(writeCorpus(t, body))
		if err != nil {
			t.Fatalf("Load rejected the legitimate hash %q: %v", hash, err)
		}
		if corpus.Rows[0].Required[0].Hash != hash {
			t.Fatalf("Hash = %q, want %q carried through", corpus.Rows[0].Required[0].Hash, hash)
		}
	}
}

func TestLoadRejectsARowListingTheSameRequiredNodeTwice(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}`,
		`{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "the reason the labeller gave"},
      {"node": 200, "hash": "b58c4e192231190d6da9f48db2b2dadafd336e96f2bc3cfc7195f90781aa5717", "why": "a second reason for the very same node"}`, 1)

	loadMustFail(t, body, "listed twice")
}

func TestLoadAcceptsThreeDistinctRequiredNodesOnOneRow(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus, `{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}`,
		`{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "the first reason"},
      {"node": 201, "hash": "b58c4e192231190d6da9f48db2b2dadafd336e96f2bc3cfc7195f90781aa5717", "why": "the second reason"},
      {"node": 202, "hash": "c6ee9b4217c6ef2558cfac355cb79ba53f9fd06101e45af03f3ff8ffcca7d8bc", "why": "the third reason"}`, 1)

	corpus, err := Load(writeCorpus(t, body))
	if err != nil {
		t.Fatalf("Load rejected three distinct required nodes, which the cap permits: %v", err)
	}

	var nodes []int64
	for _, req := range corpus.Rows[0].Required {
		nodes = append(nodes, req.Node)
	}
	if len(nodes) != 3 || nodes[0] != 200 || nodes[1] != 201 || nodes[2] != 202 {
		t.Fatalf("Required nodes = %v, want [200 201 202] in corpus order", nodes)
	}
}

const validPairedCorpus = `[
  {
    "id": "b01p",
    "pair": "b01",
    "input": "pin the wrapped shutdown sentinel in the server package",
    "subject": 100,
    "anchor": "place",
    "anchorTitle": "internal/server/ - the HTTP surface and its lifecycle",
    "stratum": "labelled",
    "required": [
      {"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it calls a passing test a discriminating one"}
    ]
  },
  {
    "id": "b01w",
    "pair": "b01",
    "input": "pin the wrapped shutdown sentinel in the server package",
    "subject": 101,
    "anchor": "work",
    "anchorTitle": "Pin three unpinned axes in the server package",
    "stratum": "labelled",
    "required": [
      {"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it calls a passing test a discriminating one"}
    ]
  }
]`

func mutatePairedCorpus(t *testing.T, old, replacement string) string {
	t.Helper()

	mutated := strings.Replace(validPairedCorpus, old, replacement, 1)
	if mutated == validPairedCorpus {
		t.Fatalf("replacing %q with %q left the fixture unchanged, so the case under test was never built", old, replacement)
	}
	return mutated
}

func appendToPairedCorpus(t *testing.T, row string) string {
	t.Helper()

	const tail = "\n]"
	trimmed := strings.TrimSuffix(validPairedCorpus, tail)
	if trimmed == validPairedCorpus {
		t.Fatal("the paired fixture does not end in the closing bracket this append relies on, so the case under test was never built")
	}
	return trimmed + ",\n" + row + tail
}

func TestLoadCarriesTheAnchorClassAndThePinnedTitleOfEveryRow(t *testing.T) {
	t.Parallel()

	corpus, err := Load(writeCorpus(t, validPairedCorpus))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(corpus.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(corpus.Rows))
	}
	place, work := corpus.Rows[0], corpus.Rows[1]
	if place.Anchor != AnchorPlace || work.Anchor != AnchorWork {
		t.Fatalf("anchor classes = %q and %q, want %q and %q", place.Anchor, work.Anchor, AnchorPlace, AnchorWork)
	}
	if place.Pair != "b01" || work.Pair != "b01" {
		t.Fatalf("pairs = %q and %q, want both rows in b01", place.Pair, work.Pair)
	}
	if place.AnchorTitle != "internal/server/ - the HTTP surface and its lifecycle" {
		t.Fatalf("AnchorTitle = %q, want the title the class was judged from, verbatim", place.AnchorTitle)
	}
	if work.AnchorTitle != "Pin three unpinned axes in the server package" {
		t.Fatalf("AnchorTitle = %q, want the title the class was judged from, verbatim", work.AnchorTitle)
	}
}

func TestLoadAcceptsARowThatCarriesNoAnchorClassAtAll(t *testing.T) {
	t.Parallel()

	corpus, err := Load(writeCorpus(t, validCorpus))
	if err != nil {
		t.Fatalf("Load rejected an unclassified corpus, which predates the anchor stratification: %v", err)
	}
	if corpus.Rows[0].Anchor != "" || corpus.Rows[0].Pair != "" {
		t.Fatalf("row = %+v, want an empty anchor class and no pair", corpus.Rows[0])
	}
}

func TestLoadRejectsAnAnchorClassOutsideTheClosedSet(t *testing.T) {
	t.Parallel()

	loadMustFail(t, mutatePairedCorpus(t, `"anchor": "place"`, `"anchor": "Place"`), "outside the closed set")
}

func TestLoadRejectsAClassifiedAnchorThatPinsNoTitle(t *testing.T) {
	t.Parallel()

	loadMustFail(t, mutatePairedCorpus(t, `"anchorTitle": "internal/server/ - the HTTP surface and its lifecycle"`, `"anchorTitle": ""`),
		"no title is pinned")
}

func TestLoadRejectsAPinnedAnchorTitleThatRecordsNoClass(t *testing.T) {
	t.Parallel()

	loadMustFail(t, mutatePairedCorpus(t, `"anchor": "place",`, `"anchor": "",`), "no class is recorded")
}

func TestLoadRejectsACorpusInWhichOnlySomeRowsCarryAnAnchorClass(t *testing.T) {
	t.Parallel()

	body := mutatePairedCorpus(t, `"anchor": "work",
    "anchorTitle": "Pin three unpinned axes in the server package",
`, "")
	loadMustFail(t, body, "only partly stratified")
}

func TestLoadRejectsACorpusInWhichOnlySomeRowsCarryAPair(t *testing.T) {
	t.Parallel()

	loadMustFail(t, mutatePairedCorpus(t, `"pair": "b01",
    "input": "pin the wrapped shutdown sentinel in the server package",
    "subject": 101`, `"input": "pin the wrapped shutdown sentinel in the server package",
    "subject": 101`), "not half of")
}

func TestLoadRejectsAPairWhoseTwoRowsShareOneAnchorClass(t *testing.T) {
	t.Parallel()

	loadMustFail(t, mutatePairedCorpus(t, `"anchor": "work"`, `"anchor": "place"`), "varies nothing")
}

func TestLoadRejectsAPairWhoseTwoRowsCarryDifferentInputs(t *testing.T) {
	t.Parallel()

	body := mutatePairedCorpus(t, `"input": "pin the wrapped shutdown sentinel in the server package",
    "subject": 101`, `"input": "pin the lifecycle log records in the server package",
    "subject": 101`)
	loadMustFail(t, body, "two different inputs")
}

func TestLoadRejectsAPairWhoseTwoRowsAreScoredAgainstDifferentRequiredNodes(t *testing.T) {
	t.Parallel()

	body := mutatePairedCorpus(t, `{"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it calls a passing test a discriminating one"}
    ]
  }
]`, `{"node": 201, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it calls a passing test a discriminating one"}
    ]
  }
]`)
	loadMustFail(t, body, "different required sets")
}

func TestLoadRejectsAPairThatHoldsMoreThanTwoRows(t *testing.T) {
	t.Parallel()

	body := appendToPairedCorpus(t, `  {
    "id": "b01x",
    "pair": "b01",
    "input": "pin the wrapped shutdown sentinel in the server package",
    "subject": 102,
    "anchor": "place",
    "anchorTitle": "internal/server/ - the HTTP surface and its lifecycle",
    "stratum": "labelled",
    "required": [
      {"node": 200, "hash": "5da7e2b28e10a231e97b202dd241d9df0e4a897ac6f5ccb5169c0b8492908cd6", "why": "an answer that omits it calls a passing test a discriminating one"}
    ]
  }`)
	loadMustFail(t, body, "are not both present")
}

func TestTheShippedAnchorStratifiedCorpusIsClassifiedPairedAndAnchoredOnDistinctNodes(t *testing.T) {
	t.Parallel()

	corpus, err := Load("corpus-anchor.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	pairs, subjects := map[string]bool{}, map[int64]string{}
	for _, row := range corpus.Rows {
		if row.Anchor != AnchorPlace && row.Anchor != AnchorWork {
			t.Fatalf("row %q carries anchor class %q, want every row of this corpus classified", row.ID, row.Anchor)
		}
		if row.Pair == "" || row.AnchorTitle == "" || row.Stratum != StratumLabelled {
			t.Fatalf("row %q = %+v, want a pair, a pinned anchor title and the labelled stratum", row.ID, row)
		}
		if owner, taken := subjects[row.Subject]; taken {
			t.Fatalf("rows %q and %q share anchor %d, which lets one anchor dominate the population the strata are drawn from", owner, row.ID, row.Subject)
		}
		subjects[row.Subject] = row.ID
		pairs[row.Pair] = true
	}

	const wantPairs = 16
	if len(pairs) < wantPairs {
		t.Fatalf("%d pairs, want at least %d so each stratum carries a distribution rather than an existence proof", len(pairs), wantPairs)
	}
}
