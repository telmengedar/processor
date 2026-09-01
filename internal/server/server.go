package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	shutdownGrace     = 5 * time.Second
	readHeaderTimeout = 5 * time.Second
)

// Serve runs handler on the already-bound listener ln until ctx is cancelled, then drains and closes ln before returning.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	logger.Info("listening", "addr", ln.Addr().String())

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

	<-serveErr

	logger.Info("shutdown complete")
	return nil
}
