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
