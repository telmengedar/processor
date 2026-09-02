package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

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
		done <- Serve(ctx, ln, NewHandler(loop.NewTurn(stubGraph{})), discardLogger())
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

	err = Serve(context.Background(), ln, NewHandler(loop.NewTurn(stubGraph{})), discardLogger())
	if err == nil {
		t.Fatal("Serve returned nil for an already-closed listener, want a non-nil error")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve returned the shutdown sentinel for a startup failure: %v", err)
	}
}
