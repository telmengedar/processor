package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/telmengedar/processor/internal/condense"
)

const oneRowCorpus = `[
  {"id": "r01", "input": "what does the ruling bind", "subject": 100, "stratum": "labelled",
   "required": [{"node": 200, "hash": "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6", "why": "an answer lacking this would present the convention as house style"}]}
]`

const nodeBody = "A long enough body to condense, stated once and then restated at length so that the condensation has something to remove. It repeats itself deliberately."

type graphRecorder struct {
	mu      sync.Mutex
	patches []string
}

func (g *graphRecorder) record(body string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.patches = append(g.patches, body)
}

func (g *graphRecorder) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.patches)
}

func testGraph(t *testing.T, recorder *graphRecorder) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			body, _ := io.ReadAll(r.Body)
			recorder.record(string(body))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"result":[{"id":200,"type":"documentation","name":"A ruling","contentType":"text/markdown","content":%q}],"total":1}`, nodeBody)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testModel(t *testing.T, text string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"ai/test-served","choices":[{"message":{"content":%q},"finish_reason":"stop"}]}`, text)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeCorpus(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the corpus failed: %v", err)
	}
	return path
}

func bootEnv(t *testing.T, graphURL, modelURL string) {
	t.Helper()

	t.Setenv("PROCESSOR_DIVOID_URL", graphURL)
	t.Setenv("PROCESSOR_DIVOID_KEY", "a-key")
	t.Setenv("PROCESSOR_MODEL_URL", modelURL)
	t.Setenv("PROCESSOR_MODEL_ID", "ai/test-requested")
}

func runCondense(t *testing.T, args ...string) (int, condense.Result, string) {
	t.Helper()

	var machine, human strings.Builder
	code := run(args, &machine, &human)

	var result condense.Result
	if machine.Len() > 0 {
		if err := json.Unmarshal([]byte(machine.String()), &result); err != nil {
			t.Fatalf("the machine output did not decode: %v\n%s", err, machine.String())
		}
	}
	return code, result, human.String()
}

func TestThePassCondensesTheCorpusRequiredNodeAndWritesItAsAJSONPatch(t *testing.T) {
	recorder := &graphRecorder{}
	bootEnv(t, testGraph(t, recorder).URL, testModel(t, "The ruling binds Go on this repo.").URL)
	corpus := writeCorpus(t, oneRowCorpus)

	code, result, _ := runCondense(t, "-corpus", corpus)

	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if len(result.Provenance) != 1 || result.Provenance[0].Node != 200 {
		t.Fatalf("want one provenance row for node 200, got %+v", result.Provenance)
	}
	if recorder.count() != 1 {
		t.Fatalf("want exactly one substance write, got %d", recorder.count())
	}
	if !strings.Contains(recorder.patches[0], `"/substance"`) {
		t.Fatalf("want the write to target /substance, got %s", recorder.patches[0])
	}
}

func TestThePassReportsTheModelTheEndpointServedRatherThanTheOneConfigured(t *testing.T) {
	bootEnv(t, testGraph(t, &graphRecorder{}).URL, testModel(t, "The ruling binds Go on this repo.").URL)
	corpus := writeCorpus(t, oneRowCorpus)

	_, result, human := runCondense(t, "-corpus", corpus)

	if result.Provenance[0].Model != "ai/test-served" {
		t.Fatalf("want the served model recorded, got %q", result.Provenance[0].Model)
	}
	if !strings.Contains(human, "ai/test-served") {
		t.Fatalf("want the served model in the report, got:\n%s", human)
	}
}

func TestTheAuditBundleReachesTheHumanChannelWithTheCorpusReasonInIt(t *testing.T) {
	bootEnv(t, testGraph(t, &graphRecorder{}).URL, testModel(t, "The ruling binds Go on this repo.").URL)
	corpus := writeCorpus(t, oneRowCorpus)

	_, _, human := runCondense(t, "-corpus", corpus)

	if !strings.Contains(human, "an answer lacking this would present the convention as house style") {
		t.Fatalf("want the pre-registered reason in the bundle, got:\n%s", human)
	}
	if !strings.Contains(human, "The ruling binds Go on this repo.") {
		t.Fatalf("want the generated substance in the bundle, got:\n%s", human)
	}
}

func TestADryRunWritesNothingToTheGraphButStillReportsTheRatio(t *testing.T) {
	recorder := &graphRecorder{}
	bootEnv(t, testGraph(t, recorder).URL, testModel(t, "The ruling binds Go on this repo.").URL)
	corpus := writeCorpus(t, oneRowCorpus)

	code, result, _ := runCondense(t, "-corpus", corpus, "-dry-run")

	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if recorder.count() != 0 {
		t.Fatalf("a dry run must issue no write, got %d", recorder.count())
	}
	if result.Ratio.Count != 1 {
		t.Fatalf("a dry run must still report the ratio, got %+v", result.Ratio)
	}
}

func TestAnExplicitIdListIsAcceptedInsteadOfACorpus(t *testing.T) {
	bootEnv(t, testGraph(t, &graphRecorder{}).URL, testModel(t, "The ruling binds Go on this repo.").URL)

	code, result, _ := runCondense(t, "-ids", "200")

	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if len(result.Audit) != 0 {
		t.Fatalf("an explicit id carries no pre-registered reason, so it produces no audit entry, got %d", len(result.Audit))
	}
}

func TestNeitherTargetFlagAndBothTargetFlagsAreEachRejectedAsUsage(t *testing.T) {
	corpus := writeCorpus(t, oneRowCorpus)

	if code, _, _ := runCondense(t); code != exitUsage {
		t.Fatalf("want exit %d when no target is named, got %d", exitUsage, code)
	}
	if code, _, _ := runCondense(t, "-corpus", corpus, "-ids", "200"); code != exitUsage {
		t.Fatalf("want exit %d when both targets are named, got %d", exitUsage, code)
	}
}

func TestAnIdListThatIsNotNumericIsRejectedAsUsage(t *testing.T) {
	if code, _, _ := runCondense(t, "-ids", "200,notanid"); code != exitUsage {
		t.Fatalf("want exit %d for a malformed id list, got %d", exitUsage, code)
	}
}

func TestAModelFailureLeavesTheGraphUntouchedAndExitsNonZero(t *testing.T) {
	recorder := &graphRecorder{}
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	bootEnv(t, testGraph(t, recorder).URL, failing.URL)
	corpus := writeCorpus(t, oneRowCorpus)

	code, result, _ := runCondense(t, "-corpus", corpus)

	if code != exitError {
		t.Fatalf("want exit %d when every call failed, got %d", exitError, code)
	}
	if recorder.count() != 0 {
		t.Fatalf("a failed model call must produce no write, got %d", recorder.count())
	}
	if result.OperationalFailures() != 1 {
		t.Fatalf("want one operational failure, got %d", result.OperationalFailures())
	}
}

func TestACondensationNoShorterThanItsContentIsARuleSkipAndStillExitsZero(t *testing.T) {
	recorder := &graphRecorder{}
	bootEnv(t, testGraph(t, recorder).URL, testModel(t, nodeBody+" and then some more").URL)
	corpus := writeCorpus(t, oneRowCorpus)

	code, result, _ := runCondense(t, "-corpus", corpus)

	if code != 0 {
		t.Fatalf("a rule skip is not an operational failure and must exit 0, got %d", code)
	}
	if recorder.count() != 0 {
		t.Fatalf("want no write, got %d", recorder.count())
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("want one skip, got %+v", result.Skipped)
	}
}

func TestBootConfigurationThatNamesTheGraphApiPathIsRejectedBeforeAnythingIsWritten(t *testing.T) {
	recorder := &graphRecorder{}
	graph := testGraph(t, recorder)
	bootEnv(t, graph.URL+"/api", testModel(t, "a condensation").URL)
	corpus := writeCorpus(t, oneRowCorpus)

	code, _, _ := runCondense(t, "-corpus", corpus)

	if code != exitError {
		t.Fatalf("want exit %d for a graph url carrying the api path, got %d", exitError, code)
	}
	if recorder.count() != 0 {
		t.Fatalf("want no write, got %d", recorder.count())
	}
}
