package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestServeServesThenShutsDownCleanly(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ln, NewHandler(newTestTurn(stubGraph{})), discardLogger())
	}()

	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("GET /health while serving: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status while serving = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v after cancellation, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return within 5s of context cancellation")
	}

	if _, err := http.Get("http://" + addr + "/health"); err == nil {
		t.Fatal("GET /health succeeded after shutdown, want the listener closed")
	}
}

type fakeHTTPServer struct {
	serveErr       error
	shutdownCalled chan struct{}
}

func newFakeHTTPServer(serveErr error) *fakeHTTPServer {
	return &fakeHTTPServer{
		serveErr:       serveErr,
		shutdownCalled: make(chan struct{}),
	}
}

func (f *fakeHTTPServer) Serve(net.Listener) error {
	<-f.shutdownCalled
	return f.serveErr
}

func (f *fakeHTTPServer) Shutdown(context.Context) error {
	close(f.shutdownCalled)
	return nil
}

func TestServeCancellationBranchPropagatesGenuineServeError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fake accept failure: literal boom")
	fake := newFakeHTTPServer(wantErr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, fake, discardLogger()) }()

	cancel()

	select {
	case got := <-done:
		select {
		case <-fake.shutdownCalled:
		default:
			t.Fatal("Shutdown was never called; the cancellation branch was not exercised")
		}
		if !errors.Is(got, wantErr) {
			t.Fatalf("serve() returned %v, want %v (a genuine serve error must propagate even though cancellation was selected)", got, wantErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return within 5s of context cancellation")
	}
}

func TestServeCancellationBranchSwallowsShutdownSentinel(t *testing.T) {
	t.Parallel()

	fake := newFakeHTTPServer(http.ErrServerClosed)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, fake, discardLogger()) }()

	cancel()

	select {
	case got := <-done:
		select {
		case <-fake.shutdownCalled:
		default:
			t.Fatal("Shutdown was never called; the cancellation branch was not exercised")
		}
		if got != nil {
			t.Fatalf("serve() returned %v, want nil (the shutdown sentinel must still be swallowed on the cancellation branch)", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return within 5s of context cancellation")
	}
}

func TestServeOnUnusableListenerReturnsNonNilError(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err = Serve(context.Background(), ln, NewHandler(newTestTurn(stubGraph{})), discardLogger())
	if err == nil {
		t.Fatal("Serve returned nil for an already-closed listener, want a non-nil error")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve returned the shutdown sentinel for a startup failure: %v", err)
	}
}

const graceMargin = divoid.DefaultTimeout

type graphCallCounter struct {
	calls  int
	failed []int
}

func (g *graphCallCounter) RoundTrip(*http.Request) (*http.Response, error) {
	g.calls++

	status := http.StatusOK
	if slices.Contains(g.failed, g.calls) {
		status = http.StatusInternalServerError
	}

	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"id":1}`)),
	}, nil
}

var writeBackPaths = []struct {
	path   string
	failed []int
	calls  int
}{
	{"every leg lands", nil, 4},
	{"the create fails", []int{1}, 1},
	{"the content write fails and the discard lands", []int{2}, 3},
	{"the content write fails and the discard fails too", []int{2, 3}, 3},
	{"the substance write fails and the link still lands", []int{3}, 4},
	{"the link fails", []int{4}, 4},
}

func writeBackRecords() map[string]loop.Record {
	unrooted := loop.Record{Input: "a run in flight at shutdown", Subject: 42}
	rooted := unrooted
	rooted.Perspective = &loop.View{Config: 77, Root: 9001}
	return map[string]loop.Record{"unrooted": unrooted, "rooted": rooted}
}

func writeBackGraphCalls(record loop.Record, failed ...int) int {
	counter := &graphCallCounter{failed: failed}
	graph := divoid.NewClient("http://graph.test", "k", &http.Client{Transport: counter}, discardLogger())
	graph.WriteRun(context.Background(), record)
	return counter.calls
}

func writeBackGraphCallCeiling() int {
	ceiling := 0
	for _, record := range writeBackRecords() {
		for _, p := range writeBackPaths {
			ceiling = max(ceiling, writeBackGraphCalls(record, p.failed...))
		}
	}
	return ceiling
}

func TestTheWriteBackIssuesFourGraphCallsUnlessTheCreateOrTheContentWriteFails(t *testing.T) {
	t.Parallel()

	for kind, record := range writeBackRecords() {
		for _, p := range writeBackPaths {
			if got := writeBackGraphCalls(record, p.failed...); got != p.calls {
				t.Errorf("the write-back of a %s record issued %d graph calls when %s, want %d", kind, got, p.path, p.calls)
			}
		}
	}
}

func TestTheDrainGraceIsTheRunBoundPlusTheMeasuredWriteBackCeilingPlusAStatedMargin(t *testing.T) {
	t.Parallel()

	const wantRunBound = 10 * time.Minute
	const wantGrace = 11*time.Minute + 15*time.Second

	if RunBound != wantRunBound {
		t.Fatalf("RunBound = %v, want %v", RunBound, wantRunBound)
	}
	if shutdownGrace != wantGrace {
		t.Fatalf("shutdownGrace = %v, want %v", shutdownGrace, wantGrace)
	}

	calls := writeBackGraphCallCeiling()
	allowance := time.Duration(calls) * divoid.DefaultTimeout
	if derived := RunBound + allowance + graceMargin; derived != shutdownGrace {
		t.Fatalf("RunBound %v + a measured write-back ceiling of %d graph calls at %v each (%v) + margin %v = %v, but shutdownGrace is %v", RunBound, calls, divoid.DefaultTimeout, allowance, graceMargin, derived, shutdownGrace)
	}
}

func TestTheDrainGraceKeepsAPositiveMarginOverTheBoundItMustCover(t *testing.T) {
	t.Parallel()

	if graceMargin <= 0 {
		t.Fatalf("graceMargin = %v, want a positive headroom — the margin is the whole reason the grace does not sit on its own bound", graceMargin)
	}
	covered := RunBound + time.Duration(writeBackGraphCallCeiling())*divoid.DefaultTimeout
	if shutdownGrace <= covered {
		t.Fatalf("shutdownGrace = %v, but a run plus its write-back can take %v — the grace sits on its own bound with no headroom", shutdownGrace, covered)
	}
}
