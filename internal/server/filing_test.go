package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const filingProbeTimeout = 3 * time.Second

type heldFilingGraph struct {
	stubGraph

	reached   chan struct{}
	release   chan struct{}
	once      sync.Once
	started   atomic.Int32
	completed atomic.Int32
	filed     atomic.Value
}

func newHeldFilingGraph(t *testing.T) *heldFilingGraph {
	t.Helper()
	graph := &heldFilingGraph{stubGraph: replyGraph(), reached: make(chan struct{}, 8), release: make(chan struct{})}
	t.Cleanup(graph.let)
	return graph
}

func (g *heldFilingGraph) let() {
	g.once.Do(func() { close(g.release) })
}

func (g *heldFilingGraph) WriteRun(_ context.Context, record loop.Record) loop.WriteReceipt {
	g.started.Add(1)
	g.reached <- struct{}{}
	<-g.release
	g.filed.Store(record)
	g.completed.Add(1)
	return g.writeReceipt
}

func (g *heldFilingGraph) awaitReached(t *testing.T) {
	t.Helper()
	select {
	case <-g.reached:
	case <-time.After(filingProbeTimeout):
		t.Fatal("the filing was never attempted")
	}
}

func heldFilingTurn(graph loop.GraphPort) *loop.Turn {
	return loop.NewTurn(graph, &scriptedModel{step: answeringWith("an answer")}, nil, fixedSystem("system text"), "test-model", testLogger())
}

func postOver(url, target string) (*http.Response, error) {
	client := &http.Client{Timeout: filingProbeTimeout}
	return client.Post(url+target, "application/json", strings.NewReader(`{"input":"what is going on","subject":42}`))
}

func TestTheDefaultReplyIsCompleteBeforeTheRecordIsFiled(t *testing.T) {
	t.Parallel()

	graph := newHeldFilingGraph(t)
	srv := httptest.NewServer(NewHandler(heldFilingTurn(graph)))
	t.Cleanup(func() {
		graph.let()
		srv.Close()
	})

	resp, err := postOver(srv.URL, "/runs")
	if err != nil {
		t.Fatalf("POST /runs: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("the reply body was not complete while the filing was held: %v", err)
	}

	var got struct {
		Reply    string `json:"reply"`
		Produced bool   `json:"produced"`
	}
	if err := json.Unmarshal(body, &got); err != nil || got.Reply != "an answer" || !got.Produced {
		t.Fatalf("body = %s (%v), want the answer and produced", body, err)
	}
	if done := graph.completed.Load(); done != 0 {
		t.Fatalf("%d filings had completed when the reply arrived, want none: the reply must not wait for them", done)
	}

	graph.awaitReached(t)
	graph.let()
	awaitCount(t, &graph.completed, 1)
	if filed, _ := graph.filed.Load().(loop.Record); filed.Answer != "an answer" {
		t.Fatalf("the filed record answer = %q, want the answer the caller received", filed.Answer)
	}
}

func awaitCount(t *testing.T, counter *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(filingProbeTimeout)
	for counter.Load() != want {
		if time.Now().After(deadline) {
			t.Fatalf("count = %d, want %d", counter.Load(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if got := counter.Load(); got != want {
		t.Fatalf("count = %d after settling, want exactly %d", got, want)
	}
}

func TestTheVerboseReplyWaitsForTheFilingAndCarriesItsReceipt(t *testing.T) {
	t.Parallel()

	graph := newHeldFilingGraph(t)
	srv := httptest.NewServer(NewHandler(heldFilingTurn(graph)))
	t.Cleanup(func() {
		graph.let()
		srv.Close()
	})

	type result struct {
		body []byte
		err  error
	}
	arrived := make(chan result, 1)
	go func() {
		resp, err := postOver(srv.URL, "/runs?verbose=true")
		if err != nil {
			arrived <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		arrived <- result{body: body, err: err}
	}()

	graph.awaitReached(t)
	select {
	case early := <-arrived:
		t.Fatalf("the verbose reply arrived while the filing was held: %s (%v)", early.body, early.err)
	case <-time.After(300 * time.Millisecond):
	}

	graph.let()
	var final result
	select {
	case final = <-arrived:
	case <-time.After(filingProbeTimeout):
		t.Fatal("the verbose reply never arrived after the filing was released")
	}
	if final.err != nil {
		t.Fatalf("verbose POST: %v", final.err)
	}
	var got struct {
		Written loop.WriteReceipt `json:"written"`
	}
	if err := json.Unmarshal(final.body, &got); err != nil || got.Written != graph.writeReceipt {
		t.Fatalf("verbose written = %+v (%v), want the receipt the filing returned %+v", got.Written, err, graph.writeReceipt)
	}
}

func TestAShutdownAfterTheReplyWaitsForTheFiling(t *testing.T) {
	t.Parallel()

	graph := newHeldFilingGraph(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, ln, NewHandler(heldFilingTurn(graph)), discardLogger()) }()

	resp, err := postOver("http://"+ln.Addr().String(), "/runs")
	if err != nil {
		t.Fatalf("POST /runs: %v", err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("read reply: %v", err)
	}
	_ = resp.Body.Close()
	graph.awaitReached(t)

	cancel()
	select {
	case err := <-served:
		t.Fatalf("Serve returned (%v) while the filing was still held: the drain no longer covers it", err)
	case <-time.After(300 * time.Millisecond):
	}
	if done := graph.completed.Load(); done != 0 {
		t.Fatalf("%d filings completed before the release", done)
	}

	graph.let()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("Serve = %v, want nil", err)
		}
	case <-time.After(filingProbeTimeout):
		t.Fatal("Serve did not return after the filing was released")
	}
	if done := graph.completed.Load(); done != 1 {
		t.Fatalf("%d filings completed by the time Serve returned, want 1", done)
	}
}

func TestAFailedFilingLeavesTheDefaultResponseUnchanged(t *testing.T) {
	t.Parallel()

	bodyFor := func(receipt loop.WriteReceipt) (string, string) {
		graph := replyGraph()
		graph.writeReceipt = receipt
		rec := postTo(t, context.Background(), loop.NewTurn(graph, &scriptedModel{step: answeringWith("an answer")}, nil, fixedSystem("system text"), "test-model", testLogger()), "/runs")
		return rec.Body.String(), rec.Header().Get("Content-Length")
	}

	stored, storedLength := bodyFor(loop.WriteReceipt{State: loop.Stored, NodeID: 10525})
	lost, lostLength := bodyFor(loop.WriteReceipt{State: loop.NotStored})

	if stored != lost {
		t.Fatalf("the default body depends on the filing: stored %q, notStored %q", stored, lost)
	}
	if storedLength != lostLength || storedLength == "" {
		t.Fatalf("Content-Length = %q and %q, want one equal declared length", storedLength, lostLength)
	}
}

type deafWriter struct {
	http.ResponseWriter
}

func (deafWriter) Write([]byte) (int, error) { return 0, errors.New("the caller went away") }

func TestTheRecordIsFiledWhenTheCallerDisconnectsWhileTheRunIsAnswering(t *testing.T) {
	t.Parallel()

	graph := newHeldFilingGraph(t)
	graph.let()
	ctx, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	model := &scriptedModel{step: func(int) (loop.JudgeResult, error) {
		disconnect()
		return loop.JudgeResult{Answer: "an answer", Reason: loop.Answered, RawReason: "stop"}, nil
	}}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system text"), "test-model", testLogger())
	req := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewBufferString(`{"input":"what is going on","subject":42}`)).WithContext(ctx)

	NewHandler(turn).ServeHTTP(deafWriter{httptest.NewRecorder()}, req)

	if ctx.Err() == nil {
		t.Fatal("the request context was never cancelled, so no disconnect was modelled")
	}
	if got := graph.completed.Load(); got != 1 {
		t.Fatalf("%d filings completed after the caller disconnected, want 1", got)
	}
}
