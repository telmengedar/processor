package main

import (
	"encoding/json"
	"fmt"
	"io"
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

const answerKeySentinel = "SENTINEL-WHY-THE-MODEL-MUST-NEVER-SEE"

const oneRowBaseline = `[
  {"row": "r02", "queries": ["a pinned query the baseline holds"], "source": "hand-authored"}
]`

type promptRecorder struct {
	mu     sync.Mutex
	bodies []string
}

func (r *promptRecorder) record(body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bodies = append(r.bodies, body)
}

func (r *promptRecorder) read() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.bodies)
}

func recordingEndpoint(t *testing.T, completion string) (string, *promptRecorder) {
	t.Helper()

	recorder := &promptRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorder.record(string(body))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"ai/test-served","choices":[{"message":{"content":%q},"finish_reason":"stop"}]}`, completion)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, recorder
}

func modelEnv(t *testing.T, url string) {
	t.Helper()

	t.Setenv("PROCESSOR_MODEL_URL", url)
	t.Setenv("PROCESSOR_MODEL_ID", "ai/test-requested")
}

func fileIn(t *testing.T, dir, name, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s failed: %v", name, err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s failed: %v", path, err)
	}
	return string(raw)
}

func runDerive(t *testing.T, args ...string) (int, string, string) {
	t.Helper()

	var machine, human strings.Builder
	code := run(args, &machine, &human)
	return code, machine.String(), human.String()
}

func corpusWithSentinel(rowID, input string, subject, node int64, hash string) string {
	return fmt.Sprintf(`[
  {"id": %q, "input": %q, "subject": %d, "stratum": "labelled",
   "required": [{"node": %d, "hash": %q, "why": %q}]}
]`, rowID, input, subject, node, hash, answerKeySentinel)
}

func TestTheGeneratorWritesTheRegeneratedSidecarAtTheOutPathAndLeavesTheBaselineByteIdentical(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)
	out := filepath.Join(dir, "regenerated.json")
	url, _ := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)
	before := readFile(t, baseline)

	code, _, human := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", out)

	if code != 0 {
		t.Fatalf("want exit 0, got %d:\n%s", code, human)
	}
	if after := readFile(t, baseline); after != before {
		t.Fatalf("the baseline sidecar changed under a generation run; it is the query set every arm-versus-arm figure was measured against\nbefore:\n%s\nafter:\n%s", before, after)
	}

	regenerated, err := eval.LoadDerivations(out, corpusFrom(t, twoRowCorpus))
	if err != nil {
		t.Fatalf("the regenerated sidecar did not load: %v", err)
	}
	if !slices.Equal(regenerated.Queries["r01"], decoratedCompletionQueries) {
		t.Fatalf("r01 was pinned %q, want the parsed completion %q", regenerated.Queries["r01"], decoratedCompletionQueries)
	}
	if regenerated.Sources["r01"] != eval.SourceBlindGenerated {
		t.Fatalf("r01 was pinned as %q, want %q", regenerated.Sources["r01"], eval.SourceBlindGenerated)
	}
	if !slices.Equal(regenerated.Queries["r02"], []string{"a pinned query the baseline holds"}) {
		t.Fatalf("r02 was not carried through from the baseline, got %q", regenerated.Queries["r02"])
	}
}

func TestTheGeneratorRefusesAnOutPathThatResolvesToTheBaselineSidecarAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)
	url, _ := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)
	before := readFile(t, baseline)

	spelled := filepath.Join(dir, "sub", "..", "derivations.json")
	code, _, human := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", spelled)

	if code != exitUsage {
		t.Fatalf("want exit %d when -out resolves to the baseline, got %d:\n%s", exitUsage, code, human)
	}
	if after := readFile(t, baseline); after != before {
		t.Fatalf("a refused run still changed the baseline:\n%s", after)
	}
	if !strings.Contains(human, "is the baseline sidecar") {
		t.Fatalf("the refusal does not name -out as the baseline, and the word appears elsewhere in the sentence, so nothing else in this run distinguishes the two:\n%s", human)
	}
}

func TestTheGeneratorAcceptsAnOutPathBesideTheBaselineInTheSameDirectory(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)
	out := filepath.Join(dir, "sub", "..", "derivations.regenerated.json")
	url, _ := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)

	code, _, human := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", out)

	if code != 0 {
		t.Fatalf("a path beside the baseline is not the baseline and must be accepted, got %d:\n%s", code, human)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("the regenerated sidecar was not written: %v", err)
	}
}

func TestTheGeneratorOverwritesAnExistingOutFileThatIsNotTheBaseline(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)
	out := fileIn(t, dir, "regenerated.json", "[]\n")
	url, _ := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)

	code, _, human := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", out)

	if code != 0 {
		t.Fatalf("a second run over its own previous output is not a run over the baseline and must be accepted, got %d:\n%s", code, human)
	}
	if readFile(t, out) == "[]\n" {
		t.Fatal("the existing -out file was left as it was, so a second generation silently produced nothing")
	}
}

func TestTheGeneratorRejectsAnAbsentOutPathAsUsage(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)

	if code, _, human := runDerive(t, "-corpus", corpus, "-derivations", baseline); code != exitUsage {
		t.Fatalf("want exit %d when no destination is named, got %d:\n%s", exitUsage, code, human)
	}
}

func TestTheGeneratorsDryRunWritesTheSidecarToStdoutAndCreatesNoFile(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)
	out := filepath.Join(dir, "regenerated.json")
	url, _ := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)

	code, machine, human := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", out, "-dry-run")

	if code != 0 {
		t.Fatalf("want exit 0, got %d:\n%s", code, human)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a dry run must create no file at -out, stat returned %v", err)
	}

	var entries []eval.Derivation
	if err := json.Unmarshal([]byte(machine), &entries); err != nil {
		t.Fatalf("the dry run's stdout did not decode as a sidecar: %v\n%s", err, machine)
	}
	if len(entries) != 2 || entries[0].Row != "r01" || entries[0].Source != eval.SourceBlindGenerated {
		t.Fatalf("the dry run did not report the sidecar it would write, got %+v", entries)
	}
}

func TestTheGeneratorLeavesNoFileBehindWhenTheModelCallFails(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", oneRowBaseline)
	out := filepath.Join(dir, "regenerated.json")

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	modelEnv(t, failing.URL)

	code, _, _ := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", out)

	if code != exitError {
		t.Fatalf("want exit %d when the derivation call failed, got %d", exitError, code)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a failed batch must write no sidecar, stat returned %v", err)
	}
}

func TestTheGeneratorTreatsEveryCorpusRowAsUnpinnedWhenNoBaselineIsNamed(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	out := filepath.Join(dir, "regenerated.json")
	url, recorder := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)

	code, _, human := runDerive(t, "-corpus", corpus, "-derivations", "", "-out", out)

	if code != 0 {
		t.Fatalf("want exit 0, got %d:\n%s", code, human)
	}
	if len(recorder.read()) != 2 {
		t.Fatalf("want one derivation call per corpus row, got %d", len(recorder.read()))
	}
}

func TestTheGeneratorWritesNothingAndExitsZeroWhenTheBaselinePinsEveryRow(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	baseline := fileIn(t, dir, "derivations.json", `[
  {"row": "r01", "queries": ["a pinned query the baseline holds"], "source": "hand-authored"},
  {"row": "r02", "queries": ["another pinned query"], "source": "hand-authored"}
]`)
	out := filepath.Join(dir, "regenerated.json")
	url, recorder := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)

	code, _, _ := runDerive(t, "-corpus", corpus, "-derivations", baseline, "-out", out)

	if code != 0 {
		t.Fatalf("want exit 0 when there is nothing to generate, got %d", code)
	}
	if len(recorder.read()) != 0 {
		t.Fatalf("a run with no target must spend no model call, got %d", len(recorder.read()))
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a run with no target must write no sidecar, stat returned %v", err)
	}
}

func TestTheGeneratorSendsTheSameBytesForTwoCorpusRowsThatDifferOnlyOutsideIdAndInput(t *testing.T) {
	const input = "What bounds one derivation call?"

	bare := fmt.Sprintf(`[
  {"id": "r01", "input": %q, "subject": 100, "stratum": "labelled",
   "required": [{"node": 200, "hash": "1111111111111111111111111111111111111111111111111111111111111111", "why": %q}]}
]`, input, answerKeySentinel)
	loaded := fmt.Sprintf(`[
  {"id": "r01", "input": %q, "subject": 777, "anchor": "place", "anchorTitle": "a durable location", "stratum": "control",
   "required": [
     {"node": 888, "hash": "3333333333333333333333333333333333333333333333333333333333333333", "why": %q},
     {"node": 889, "hash": "4444444444444444444444444444444444444444444444444444444444444444", "why": %q}
   ]}
]`, input, answerKeySentinel, answerKeySentinel)

	bareBodies := bodiesFor(t, bare)
	loadedBodies := bodiesFor(t, loaded)

	if len(bareBodies) != 1 || len(loadedBodies) != 1 {
		t.Fatalf("want one derivation call per corpus, got %d and %d", len(bareBodies), len(loadedBodies))
	}
	if bareBodies[0] != loadedBodies[0] {
		t.Fatalf("a second required node, an anchor class and title, a control stratum and a different subject changed what the model was sent\nbare:\n%s\nloaded:\n%s", bareBodies[0], loadedBodies[0])
	}
	if strings.Contains(loadedBodies[0], answerKeySentinel) {
		t.Fatalf("the row's own reason for requiring a node reached the model; both corpora carry it verbatim, so this is the leak the byte comparison above cannot see:\n%s", loadedBodies[0])
	}
}

func bodiesFor(t *testing.T, corpusBody string) []string {
	t.Helper()

	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", corpusBody)
	out := filepath.Join(dir, "regenerated.json")
	url, recorder := recordingEndpoint(t, decoratedCompletion)
	modelEnv(t, url)

	if code, _, human := runDerive(t, "-corpus", corpus, "-derivations", "", "-out", out); code != 0 {
		t.Fatalf("want exit 0, got %d:\n%s", code, human)
	}
	return recorder.read()
}

func TestTheGeneratorCarriesTheProductsOwnDerivationPromptOnToTheWire(t *testing.T) {
	const input = "What bounds one derivation call?"
	bodies := bodiesFor(t, corpusWithSentinel("r01", input, 100, 200, "1111111111111111111111111111111111111111111111111111111111111111"))

	var sent struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(bodies[0]), &sent); err != nil {
		t.Fatalf("the request body did not decode: %v\n%s", err, bodies[0])
	}
	if len(sent.Messages) != 1 {
		t.Fatalf("want one message on the wire, got %d", len(sent.Messages))
	}
	if sent.Messages[0].Content != loop.DerivationPrompt(input) {
		t.Fatalf("the wire carried a prompt the product does not produce:\n%s", sent.Messages[0].Content)
	}
	if !strings.HasPrefix(sent.Messages[0].Content, "You generate alternate search queries for a semantic retrieval system.") {
		t.Fatalf("the wire's prompt does not open with the shipped instruction text:\n%s", sent.Messages[0].Content)
	}
}

func TestTheGeneratorReportsABootConfigurationRefusalAsAnErrorExitAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	out := filepath.Join(dir, "regenerated.json")
	t.Setenv("PROCESSOR_MODEL_PROTOCOL", "a-protocol-with-no-adapter")
	modelEnv(t, "http://127.0.0.1:1")

	code, _, _ := runDerive(t, "-corpus", corpus, "-derivations", "", "-out", out)

	if code != exitError {
		t.Fatalf("want exit %d when boot refuses the configuration, got %d", exitError, code)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a run that never reached a model must write no sidecar, stat returned %v", err)
	}
}

func TestTheGeneratorReachesTheNativeOllamaAdapterWhenThatProtocolIsConfigured(t *testing.T) {
	dir := t.TempDir()
	corpus := fileIn(t, dir, "corpus.json", twoRowCorpus)
	out := filepath.Join(dir, "regenerated.json")

	var routes []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		routes = append(routes, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"ai/test-served","message":{"content":%q},"done":true,"done_reason":"stop"}`, decoratedCompletion)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("PROCESSOR_MODEL_PROTOCOL", "ollama")
	modelEnv(t, srv.URL)

	code, _, human := runDerive(t, "-corpus", corpus, "-derivations", "", "-out", out)

	if code != 0 {
		t.Fatalf("want exit 0, got %d:\n%s", code, human)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(routes) == 0 || routes[0] != "/api/chat" {
		t.Fatalf("want the native chat route, got %v", routes)
	}
}
