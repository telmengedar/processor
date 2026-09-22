package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/measure"
	"github.com/telmengedar/processor/internal/ports"
)

const (
	anchorBody    = "the anchor body this run is about"
	candidateBody = "the candidate body recall returned"
	modelAnswer   = "the answer the model gave"
	taskText      = "where does the change land, and what would make it wrong"
)

type writeRecorder struct {
	mu       sync.Mutex
	attempts []string
}

func (w *writeRecorder) record(method, path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.attempts = append(w.attempts, method+" "+path)
}

func (w *writeRecorder) all() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.attempts...)
}

func testGraph(t *testing.T, recorder *writeRecorder) *httptest.Server {
	t.Helper()
	return graphServing(t, recorder, map[int64]string{202: candidateBody})
}

func graphServing(t *testing.T, recorder *writeRecorder, bodies map[int64]string) *httptest.Server {
	t.Helper()

	ids := slices.Sorted(maps.Keys(bodies))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			recorder.record(r.Method, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":900}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		query := r.URL.Query()
		switch {
		case strings.HasSuffix(r.URL.Path, "/links"):
			fmt.Fprint(w, `{"result":[{"sourceId":101,"targetId":303}],"continue":null}`)
		case query.Get("query") != "":
			rows := make([]string, 0, len(ids))
			for i, id := range ids {
				rows = append(rows, fmt.Sprintf(`{"id":%d,"type":"documentation","name":"candidate %d","similarity":%v,"content":%q}`, id, id, 0.95-0.01*float64(i), bodies[id]))
			}
			fmt.Fprintf(w, `{"result":[%s],"total":%d}`, strings.Join(rows, ","), len(rows))
		case query.Get("id") != "" && query.Get("id") != "101":
			id, _ := strconv.ParseInt(query.Get("id"), 10, 64)
			fmt.Fprintf(w, `{"result":[{"id":%d,"type":"documentation","name":"candidate %d","contentType":"text/markdown","content":%q}],"total":1}`, id, id, bodies[id])
		default:
			fmt.Fprintf(w, `{"result":[{"id":101,"type":"documentation","name":"the anchor","contentType":"text/markdown","content":%q}],"total":1}`, anchorBody)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testCondenseModel(t *testing.T, substance string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"ai/test-condenser","choices":[{"message":{"content":%q},"finish_reason":"stop"}]}`, substance)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testModel(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"ai/test-served","choices":[{"message":{"content":%q},"finish_reason":"stop"}]}`, modelAnswer)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func bootEnv(t *testing.T, graphURL, modelURL string) {
	t.Helper()

	t.Setenv("PROCESSOR_DIVOID_URL", graphURL)
	t.Setenv("PROCESSOR_DIVOID_KEY", "a-key")
	t.Setenv("PROCESSOR_MODEL_URL", modelURL)
	t.Setenv("PROCESSOR_MODEL_ID", "ai/test-requested")
}

func condenseEnv(t *testing.T, condenseURL string) {
	t.Helper()

	t.Setenv("PROCESSOR_CONDENSE_MODEL_URL", condenseURL)
	t.Setenv("PROCESSOR_CONDENSE_MODEL_ID", "ai/test-condenser")
}

func runMeasure(t *testing.T, task string, args ...string) (int, measure.Result, string) {
	t.Helper()

	var machine, human strings.Builder
	code := run(args, strings.NewReader(task), &machine, &human)

	var result measure.Result
	if machine.Len() > 0 {
		if err := json.Unmarshal([]byte(machine.String()), &result); err != nil {
			t.Fatalf("the machine output did not decode: %v\n%s", err, machine.String())
		}
	}
	return code, result, human.String()
}

func TestAMeasuredRunReadsTheGraphOverHTTPAndSendsItNoWriteAtAll(t *testing.T) {
	recorder := &writeRecorder{}
	bootEnv(t, testGraph(t, recorder).URL, testModel(t).URL)

	code, result, human := runMeasure(t, taskText, "-subject", "101")

	if code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, human)
	}
	if attempts := recorder.all(); len(attempts) != 0 {
		t.Fatalf("the graph received %d writes from a measured run: %v", len(attempts), attempts)
	}
	if result.Written.State != loop.NotStored {
		t.Fatalf("the receipt reads %q, want %q", result.Written.State, loop.NotStored)
	}
	if result.Answer != modelAnswer {
		t.Fatalf("the result answers %q, want %q", result.Answer, modelAnswer)
	}
	if !strings.Contains(result.Block, anchorBody) || !strings.Contains(result.Block, candidateBody) {
		t.Fatalf("the block does not carry what the graph returned: %q", result.Block)
	}
	if len(result.Suppressed) != 1 || result.Suppressed[0].Subject != 101 || result.Suppressed[0].Size == 0 {
		t.Fatalf("want one sized suppressed write for subject 101, got %+v", result.Suppressed)
	}
}

func TestAMeasuredRunKeepsItsLogOutOfTheMachineStream(t *testing.T) {
	bootEnv(t, testGraph(t, &writeRecorder{}).URL, testModel(t).URL)

	var machine, human strings.Builder
	if code := run([]string{"-subject", "101"}, strings.NewReader(taskText), &machine, &human); code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, human.String())
	}

	if !json.Valid([]byte(strings.TrimSpace(machine.String()))) {
		t.Fatalf("the machine stream is not the result alone: %s", machine.String())
	}
}

func TestASubjectThatIsNotAPositiveNodeIdIsRejectedBeforeAnythingIsRead(t *testing.T) {
	for _, args := range [][]string{{}, {"-subject", "0"}, {"-subject", "-3"}} {
		code, _, human := runMeasure(t, taskText, args...)
		if code != exitUsage {
			t.Fatalf("args %v: want exit %d, got %d\n%s", args, exitUsage, code, human)
		}
	}
}

func TestAnEmptyTaskTextIsRejectedRatherThanRunAgainstTheGraph(t *testing.T) {
	recorder := &writeRecorder{}
	bootEnv(t, testGraph(t, recorder).URL, testModel(t).URL)

	code, _, human := runMeasure(t, "   \n\t ", "-subject", "101")

	if code != exitUsage {
		t.Fatalf("want exit %d, got %d\n%s", exitUsage, code, human)
	}
	if attempts := recorder.all(); len(attempts) != 0 {
		t.Fatalf("the graph received %d writes for a task that never ran: %v", len(attempts), attempts)
	}
}

func TestTheGraphServerRecordsAWriteWhenOneIsActuallyMade(t *testing.T) {
	recorder := &writeRecorder{}
	srv := testGraph(t, recorder)

	client := divoid.NewClient(srv.URL, "a-key", nil, nil)
	if err := ports.FillGraph(client).SetSubstance(context.Background(), 202, "a substance"); err != nil {
		t.Fatalf("the undecorated fill graph refused the write: %v", err)
	}
	if receipt := client.WriteRun(context.Background(), loop.Record{Subject: 101}); receipt.State != loop.Stored {
		t.Fatalf("the undecorated client reported %q, want %q", receipt.State, loop.Stored)
	}

	attempts := recorder.all()
	if len(attempts) == 0 {
		t.Fatal("the graph server recorded no write for a substance patch and a filed record, so its recorder cannot witness a write at all and every zero-write assertion in this package is vacuous")
	}
	if !slices.Contains(attempts, "PATCH /api/nodes/202") {
		t.Fatalf("the recorder did not witness the substance patch: %v", attempts)
	}
}

func TestAMeasuredRunWithTheFillOnCondensesACandidateAndStillSendsTheGraphNoWriteAtAll(t *testing.T) {
	recorder := &writeRecorder{}
	bodies := map[int64]string{
		202: strings.Repeat("the first candidate body, long enough to fill the budget on its own. ", 600),
		303: strings.Repeat("the second candidate body, cut for want of room and worth condensing. ", 600),
	}
	bootEnv(t, graphServing(t, recorder, bodies).URL, testModel(t).URL)
	condenseEnv(t, testCondenseModel(t, strings.Repeat("a condensed account of the second candidate. ", 80)).URL)

	code, result, human := runMeasure(t, taskText, "-subject", "101")

	if code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, human)
	}
	if attempts := recorder.all(); len(attempts) != 0 {
		t.Fatalf("the graph received %d writes from a measured run with the fill on: %v", len(attempts), attempts)
	}

	filled := false
	for _, fill := range result.Fills {
		if fill.Filled {
			filled = true
		}
	}
	if !filled {
		t.Fatalf("no fill fired, so this guard is written in the configuration where the hole is closed: %+v", result.Fills)
	}
	if len(result.Substances) != 1 || result.Substances[0].Node != 303 || result.Substances[0].Size == 0 {
		t.Fatalf("want one sized substance withheld from node 303, got %+v", result.Substances)
	}
	if result.Written.State != loop.NotStored {
		t.Fatalf("the receipt reads %q, want %q", result.Written.State, loop.NotStored)
	}
}
