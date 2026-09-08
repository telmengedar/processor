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

// fakeHTTPServer lets a test control exactly what the serve goroutine
// reports and exactly when, relative to Shutdown being invoked — the two
// events whose relative timing the real accept loop cannot be made to
// order deterministically. Serve blocks until Shutdown is called, which
// guarantees serve()'s outer select observes only ctx.Done() as ready (the
// cancellation branch, not the direct-error branch) and that serveErr is
// populated only afterward — precisely the window #10464 identified.
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

	// A literal, test-owned error — not derived from anything the
	// production path itself produces, so mutating the production code
	// cannot move this expectation along with it.
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

func writeBackGraphCalls(failed ...int) int {
	counter := &graphCallCounter{failed: failed}
	graph := divoid.NewClient("http://graph.test", "k", &http.Client{Transport: counter}, discardLogger())
	graph.WriteRun(context.Background(), loop.Record{Input: "a run in flight at shutdown", Subject: 42})
	return counter.calls
}

func writeBackGraphCallCeiling() int {
	ceiling := 0
	for _, p := range writeBackPaths {
		ceiling = max(ceiling, writeBackGraphCalls(p.failed...))
	}
	return ceiling
}

func TestTheWriteBackIssuesFourGraphCallsUnlessTheCreateOrTheContentWriteFails(t *testing.T) {
	t.Parallel()

	for _, p := range writeBackPaths {
		if got := writeBackGraphCalls(p.failed...); got != p.calls {
			t.Errorf("the write-back issued %d graph calls when %s, want %d", got, p.path, p.calls)
		}
	}
}

func TestTheDrainGraceIsTheRunBoundPlusTheMeasuredWriteBackCeilingPlusAStatedMargin(t *testing.T) {
	t.Parallel()

	const wantRunBound = 10 * time.Minute
	const wantGrace = 11*time.Minute + 15*time.Second

	if runBound != wantRunBound {
		t.Fatalf("runBound = %v, want %v", runBound, wantRunBound)
	}
	if shutdownGrace != wantGrace {
		t.Fatalf("shutdownGrace = %v, want %v", shutdownGrace, wantGrace)
	}

	calls := writeBackGraphCallCeiling()
	allowance := time.Duration(calls) * divoid.DefaultTimeout
	if derived := runBound + allowance + graceMargin; derived != shutdownGrace {
		t.Fatalf("runBound %v + a measured write-back ceiling of %d graph calls at %v each (%v) + margin %v = %v, but shutdownGrace is %v", runBound, calls, divoid.DefaultTimeout, allowance, graceMargin, derived, shutdownGrace)
	}
}

func TestTheDrainGraceKeepsAPositiveMarginOverTheBoundItMustCover(t *testing.T) {
	t.Parallel()

	if graceMargin <= 0 {
		t.Fatalf("graceMargin = %v, want a positive headroom — the margin is the whole reason the grace does not sit on its own bound", graceMargin)
	}
	covered := runBound + time.Duration(writeBackGraphCallCeiling())*divoid.DefaultTimeout
	if shutdownGrace <= covered {
		t.Fatalf("shutdownGrace = %v, but a run plus its write-back can take %v — the grace sits on its own bound with no headroom", shutdownGrace, covered)
	}
}
