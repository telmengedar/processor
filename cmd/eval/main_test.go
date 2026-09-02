package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/eval"
)

const runCorpus = `[
  {"id": "r01", "input": "what did the split change", "subject": 100, "stratum": "labelled",
   "required": [{"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "an answer that omits it is wrong"}]},
  {"id": "c01", "input": "a constructed control input", "subject": 100, "stratum": "control",
   "required": [{"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "a constructed row must read one"}]}
]`

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

	body := strings.Replace(runCorpus, `"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "a constructed row must read one"`,
		`"node": 999, "hash": "0f0f", "why": "a constructed row must read one"`, 1)

	var machine, human bytes.Buffer

	code := run([]string{"-corpus", corpusFile(t, body)}, &machine, &human)

	if code != exitError {
		t.Fatalf("exit code = %d, want %d when the control stratum did not read one", code, exitError)
	}
	if machine.Len() == 0 {
		t.Fatal("no measurement was emitted; an unverified sweep must still report what it saw")
	}
	if !strings.Contains(human.String(), "the control stratum did not read 1.00") {
		t.Fatalf("the human stream does not raise the control alarm:\n%s", human.String())
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
