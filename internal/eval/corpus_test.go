package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validCorpus = `[
  {
    "id": "r01",
    "input": "what does the boot split change for a graph-only caller",
    "subject": 100,
    "stratum": "labelled",
    "required": [
      {"node": 200, "hash": "0f0f", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}
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
		t.Fatalf("Load returned a nil error and %d rows, want an error", len(corpus.Rows))
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

	const wantHash = "854847806c2db7327af358e4966916646de496abebedd033740dec3bcdae353d"
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
	if len(row.Required) != 1 || row.Required[0].Node != 200 || row.Required[0].Hash != "0f0f" {
		t.Fatalf("Required = %+v, want one entry for node 200 with hash 0f0f", row.Required)
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
		`{"node": 200, "hash": "0f0f", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}`,
		`{"node": 200, "hash": "a", "why": "w"},
      {"node": 201, "hash": "a", "why": "w"},
      {"node": 202, "hash": "a", "why": "w"},
      {"node": 203, "hash": "a", "why": "w"}`, 1)

	loadMustFail(t, body, "required nodes")
}

func TestLoadRejectsARowRequiringNoNodesAtAll(t *testing.T) {
	t.Parallel()

	body := strings.Replace(validCorpus,
		`{"node": 200, "hash": "0f0f", "why": "an answer that omits it cannot name the loader a graph-only caller uses"}`,
		``, 1)

	loadMustFail(t, body, "required nodes")
}

func TestLoadRejectsARequiredEntryMissingItsReasonOrItsHash(t *testing.T) {
	t.Parallel()

	missingHash := strings.Replace(validCorpus, `"hash": "0f0f"`, `"hash": ""`, 1)
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
