package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	shutdownGrace     = 5 * time.Second
	readHeaderTimeout = 5 * time.Second
)

// httpServer is the subset of *http.Server that serve depends on. Extracted
// so tests can substitute a fake and deterministically drive the serve
// goroutine's outcome relative to shutdown, instead of relying on the real
// accept loop's timing.
type httpServer interface {
	Serve(ln net.Listener) error
	Shutdown(ctx context.Context) error
}

// Serve runs handler on the already-bound listener ln until ctx is cancelled, then drains and closes ln before returning.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	logger.Info("listening", "addr", ln.Addr().String())

	return serve(ctx, ln, srv, logger)
}

func serve(ctx context.Context, ln net.Listener, srv httpServer, logger *slog.Logger) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown incomplete", "error", err)
		return err
	}

	// The select above chose the cancellation branch, but that tells us
	// nothing about what the serve goroutine actually reported: if a
	// genuine serve failure and the context's cancellation became ready in
	// the same window, the runtime may have picked this branch even though
	// serveErr does not hold the shutdown sentinel. Only the sentinel is
	// swallowed here — anything else is a real error and must propagate,
	// same as the direct branch above.
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	logger.Info("shutdown complete")
	return nil
}
