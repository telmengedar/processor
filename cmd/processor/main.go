// Command processor is the Processor service entry point.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/server"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := loadBootConfig(os.LookupEnv)
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	ln, err := net.Listen("tcp", cfg.httpAddr)
	if err != nil {
		logger.Error("listen", "addr", cfg.httpAddr, "error", err)
		return 1
	}

	// httpClient is nil so the graph client applies its own default
	// (divoid.DefaultTimeout) — the timeout policy lives in exactly one
	// place, not duplicated here and in the adapter (W-13). That default
	// still bounds a hung graph read; it is never http.DefaultClient,
	// which carries no timeout (W-7).
	graph := divoid.NewClient(cfg.divoidURL, cfg.divoidKey, nil)
	turn := loop.NewTurn(graph)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Serve(ctx, ln, server.NewHandler(turn), logger); err != nil {
		logger.Error("serve", "error", err)
		return 1
	}

	return 0
}
