package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// RunBound is the deadline one run is given, and the bound every model call inside it ultimately shares.
const RunBound = 10 * time.Minute

const (
	shutdownGrace     = 11*time.Minute + 15*time.Second
	readHeaderTimeout = 5 * time.Second
)

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

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	logger.Info("shutdown complete")
	return nil
}
