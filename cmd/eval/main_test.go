package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
)

const runCorpus = `[
  {"id": "r01", "input": "what did the split change", "subject": 100, "stratum": "labelled",
   "required": [{"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "an answer that omits it is wrong"}]},
  {"id": "c01", "input": "a constructed control input", "subject": 100, "stratum": "control",
   "required": [{"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "a constructed row must be retrieved"}]}
]`

var oversizedRequiredNodeBody = strings.Repeat("q", loop.AssemblyByteBudget+1)

func oversizedControlCorpus() string {
	sum := sha256.Sum256([]byte(oversizedRequiredNodeBody))
	return strings.ReplaceAll(runCorpus, requiredNodeBodyHash, hex.EncodeToString(sum[:]))
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("the destination rejected the write")
}

func graphServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("query") != "" {
			fmt.Fprintf(w, `{"result":[{"id":200,"type":"documentation","name":"Alpha","similarity":0.81,"content":%q}],"total":1}`, requiredNodeBody)
			return
		}
		if r.URL.Query().Get("id") == "100" {
			fmt.Fprint(w, `{"result":[{"id":100,"type":"documentation","name":"Subject","content":"the subject body"}],"total":1}`)
			return
		}
		fmt.Fprint(w, `{"result":[],"total":0}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func graphServerWhoseControlNodeExceedsTheBudget(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("query") != "" {
			fmt.Fprintf(w, `{"result":[{"id":199,"type":"documentation","name":"Small","similarity":0.9,"content":"a body that fits"},{"id":200,"type":"documentation","name":"Alpha","similarity":0.81,"content":%q}],"total":2}`,
				oversizedRequiredNodeBody)
			return
		}
		if r.URL.Query().Get("id") == "100" {
			fmt.Fprint(w, `{"result":[{"id":100,"type":"documentation","name":"Subject","content":"the subject body"}],"total":1}`)
			return
		}
		fmt.Fprint(w, `{"result":[],"total":0}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func corpusFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func graphEnv(t *testing.T, url string) {
	t.Helper()

	t.Setenv("PROCESSOR_DIVOID_URL", url)
	t.Setenv("PROCESSOR_DIVOID_KEY", "a-test-key")
}

func TestRunWritesTheMachineResultToItsFirstStreamAndTheHumanSummaryToItsSecond(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	var machine, human bytes.Buffer

	code := run([]string{"-corpus", corpusFile(t, runCorpus)}, &machine, &human)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; the human stream was:\n%s", code, human.String())
	}

	var decoded eval.Result
	if err := json.Unmarshal(machine.Bytes(), &decoded); err != nil {
		t.Fatalf("the first stream did not carry the machine-readable result alone: %v\nit carried:\n%s", err, machine.String())
	}
	if decoded.RowCount != 2 || len(decoded.Rows) != 2 {
		t.Fatalf("RowCount = %d with %d rows, want 2 and 2", decoded.RowCount, len(decoded.Rows))
	}
	if decoded.Rows[0].Required[0].Verdict != eval.Admitted {
		t.Fatalf("Verdict = %q, want %q: the sweep did not reach the graph", decoded.Rows[0].Required[0].Verdict, eval.Admitted)
	}

	if strings.Contains(machine.String(), "retrieved 1/1") {
		t.Fatalf("the first stream carries the human summary:\n%s", machine.String())
	}
	if !strings.Contains(human.String(), "retrieved 1/1 (1.00)") {
		t.Fatalf("the second stream does not carry the human summary:\n%s", human.String())
	}
	if strings.Contains(human.String(), "corpusHash") {
		t.Fatalf("the second stream carries the machine-readable result:\n%s", human.String())
	}
}

func TestRunKeepsItsLogOutOfTheMachineStreamWhenAStepBeforeTheSweepFails(t *testing.T) {
	graphEnv(t, "http://127.0.0.1:9")

	var machine, human bytes.Buffer

	code := run([]string{"-corpus", filepath.Join(t.TempDir(), "absent.json")}, &machine, &human)

	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if machine.Len() != 0 {
		t.Fatalf("the machine stream received %d bytes of log output, want none:\n%s", machine.Len(), machine.String())
	}
	if !strings.Contains(human.String(), "corpus") {
		t.Fatalf("the human stream does not name the failure:\n%s", human.String())
	}
}

func TestRunExitsNonZeroWhenTheMeasurementCannotBeWritten(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	var human bytes.Buffer

	code := run([]string{"-corpus", corpusFile(t, runCorpus)}, failingWriter{}, &human)

	if code != exitError {
		t.Fatalf("exit code = %d, want %d: a sweep that emitted no measurement must not report success", code, exitError)
	}
	if !strings.Contains(human.String(), "render") {
		t.Fatalf("the human stream does not name the write that failed:\n%s", human.String())
	}
}

func TestRunExitsNonZeroWhenTheControlStratumMissedOnALiveSweep(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	body := strings.Replace(runCorpus, `"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "a constructed row must be retrieved"`,
		`"node": 999, "hash": "37002ae74ca517874ce657943da48e6069979427e09de958e9b7fad19ab5cac3", "why": "a constructed row must be retrieved"`, 1)

	if !strings.Contains(body, `"node": 999`) {
		t.Fatal("the corpus this test sweeps requires no node 999: the replace matched nothing, so the sweep is exercising the node the other rows require instead of one the graph does not hold")
	}

	var machine, human bytes.Buffer

	code := run([]string{"-corpus", corpusFile(t, body)}, &machine, &human)

	if code != exitError {
		t.Fatalf("exit code = %d, want %d when the control stratum did not read one", code, exitError)
	}
	if machine.Len() == 0 {
		t.Fatal("no measurement was emitted; an unverified sweep must still report what it saw")
	}
	if !strings.Contains(human.String(), "the control stratum did not verify retrieval: either the graph moved, the harness broke, or the stratum could not be scored at all") {
		t.Fatalf("the human stream does not raise the control alarm:\n%s", human.String())
	}
}

func TestRunExitsZeroAndBlamesTheBudgetWhenTheControlNodeDoesNotFitOnALiveSweep(t *testing.T) {
	server := graphServerWhoseControlNodeExceedsTheBudget(t)
	graphEnv(t, server.URL)

	var machine, human bytes.Buffer

	code := run([]string{"-corpus", corpusFile(t, oversizedControlCorpus())}, &machine, &human)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 when retrieval surfaced the control and the budget discarded it; the human stream was:\n%s", code, human.String())
	}

	var decoded eval.Result
	if err := json.Unmarshal(machine.Bytes(), &decoded); err != nil {
		t.Fatalf("the machine stream did not carry a result: %v\nit carried:\n%s", err, machine.String())
	}
	if got := decoded.Rows[1].Required[0].Verdict; got != eval.Cut {
		t.Fatalf("the control node's verdict = %q, want %q: the fixture did not reach the boundary this guard is about", got, eval.Cut)
	}

	if !strings.Contains(human.String(), "budget alarm: the control stratum was retrieved in full and cut by the budget. Retrieval is intact and this sweep's retrieved rate is trustworthy; the admitted rate is a reading of the assembler, not of the retriever.") {
		t.Fatalf("the human stream does not name the budget as the cause:\n%s", human.String())
	}
	if strings.Contains(human.String(), "either the graph moved, the harness broke, or the stratum could not be scored at all") {
		t.Fatalf("the human stream blames the instrument for what the budget did:\n%s", human.String())
	}
}

func TestRunExitsNonZeroWhenTheSummaryCannotBeWritten(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	var machine bytes.Buffer

	code := run([]string{"-corpus", corpusFile(t, runCorpus)}, &machine, failingWriter{})

	if code != exitError {
		t.Fatalf("exit code = %d, want %d: when the human stream is the stream that failed, the exit code is the only channel left to report it on", code, exitError)
	}
	if machine.Len() == 0 {
		t.Fatal("the measurement was not written, want the sweep to have emitted it before the summary write failed")
	}
}

func recordingGraphServer(t *testing.T, queries *[]string) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if query := r.URL.Query().Get("query"); query != "" {
			mu.Lock()
			*queries = append(*queries, query)
			mu.Unlock()
			fmt.Fprintf(w, `{"result":[{"id":200,"type":"documentation","name":"Alpha","similarity":0.81,"content":%q}],"total":1}`, requiredNodeBody)
			return
		}
		if r.URL.Query().Get("id") == "100" {
			fmt.Fprint(w, `{"result":[{"id":100,"type":"documentation","name":"Subject","content":"the subject body"}],"total":1}`)
			return
		}
		fmt.Fprint(w, `{"result":[],"total":0}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRunIssuesEveryPinnedDerivationAndNamesTheArmItSweptOnBothStreams(t *testing.T) {
	var queries []string
	server := recordingGraphServer(t, &queries)
	graphEnv(t, server.URL)

	sidecar := filepath.Join(t.TempDir(), "derivations.json")
	body := `[{"row": "r01", "queries": ["why would a mutation leave the suite green"]}]`
	if err := os.WriteFile(sidecar, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var machine, human bytes.Buffer
	code := run([]string{"-corpus", corpusFile(t, runCorpus), "-derivations", sidecar}, &machine, &human)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; the human stream was:\n%s", code, human.String())
	}

	if !slices.Contains(queries, "why would a mutation leave the suite green") {
		t.Fatalf("the graph was asked %q, and the pinned derivation is not among them: a sidecar that is read and never issued produces the raw arm's ranking under the derived arm's hash", queries)
	}

	var decoded eval.Result
	if err := json.Unmarshal(machine.Bytes(), &decoded); err != nil {
		t.Fatalf("decode the machine stream: %v", err)
	}
	const wantHash = "9d2da394c35147b5e33a3fc1518b34c346e2c9e5b59d1d76517d79b9cc470fe7"
	if decoded.Arm != sidecar {
		t.Fatalf("Arm = %q, want the sidecar path %q", decoded.Arm, sidecar)
	}
	if decoded.DerivationHash != wantHash {
		t.Fatalf("DerivationHash = %q, want the sha256 of the sidecar bytes: a reading that cannot name the query set it ran cannot be compared against the one taken beside it", decoded.DerivationHash)
	}
	if decoded.DerivedRows != 1 {
		t.Fatalf("DerivedRows = %d, want 1 of the two corpus rows", decoded.DerivedRows)
	}
	if !strings.Contains(human.String(), "1/2 rows derived") {
		t.Fatalf("the human summary does not name the arm it swept on, or does not name it against the full corpus:\n%s", human.String())
	}
}

func TestRunWarnsOnTheOperatorStreamAloneWhenTheSidecarLeavesCorpusRowsUnpinned(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	// runCorpus carries two rows, r01 and c01; this sidecar pins only r01,
	// so c01 sweeps on raw input on both arms without anything in the
	// output saying so unless this warning fires.
	sidecar := filepath.Join(t.TempDir(), "derivations.json")
	body := `[{"row": "r01", "queries": ["why would a mutation leave the suite green"]}]`
	if err := os.WriteFile(sidecar, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var machine, human bytes.Buffer
	code := run([]string{"-corpus", corpusFile(t, runCorpus), "-derivations", sidecar}, &machine, &human)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; the human stream was:\n%s", code, human.String())
	}

	if !strings.Contains(human.String(), "unpinned=1") || !strings.Contains(human.String(), "rows=c01") {
		t.Fatalf("the operator stream does not name the unpinned row: a corpus row the sidecar never mentions still validates and sweeps on raw input on both arms, silently diluting every rate the sweep prints. Human stream was:\n%s", human.String())
	}
	if strings.Contains(machine.String(), "unpinned") {
		t.Fatalf("the coverage warning leaked into the machine stream, which a consumer decodes as the eval.Result schema:\n%s", machine.String())
	}
}

func TestRunSweepsTheRawInputArmAndSaysSoWhenNoSidecarIsGiven(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	var machine, human bytes.Buffer
	code := run([]string{"-corpus", corpusFile(t, runCorpus)}, &machine, &human)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; the human stream was:\n%s", code, human.String())
	}

	var decoded eval.Result
	if err := json.Unmarshal(machine.Bytes(), &decoded); err != nil {
		t.Fatalf("decode the machine stream: %v", err)
	}
	if decoded.Arm != "raw-input" {
		t.Fatalf("Arm = %q, want %q", decoded.Arm, "raw-input")
	}
	if decoded.DerivationHash != "" {
		t.Fatalf("DerivationHash = %q, want empty: no sidecar was read, and a hash on this reading would name an arm that was never swept", decoded.DerivationHash)
	}
	if !strings.Contains(human.String(), "arm raw-input") {
		t.Fatalf("the human summary does not name the raw-input arm:\n%s", human.String())
	}
}

func TestRunRefusesToSweepWhenTheSidecarNamesARowTheCorpusDoesNotCarry(t *testing.T) {
	server := graphServer(t)
	graphEnv(t, server.URL)

	sidecar := filepath.Join(t.TempDir(), "derivations.json")
	if err := os.WriteFile(sidecar, []byte(`[{"row": "r99", "queries": ["a question"]}]`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var machine, human bytes.Buffer
	code := run([]string{"-corpus", corpusFile(t, runCorpus), "-derivations", sidecar}, &machine, &human)

	if code != exitError {
		t.Fatalf("exit code = %d, want %d: a sidecar row that matches nothing sweeps its corpus row on the raw input while the result names a derived arm", code, exitError)
	}
	if machine.Len() != 0 {
		t.Fatalf("the machine stream carries a result for a sweep that never ran:\n%s", machine.String())
	}
}
