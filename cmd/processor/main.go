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
	"github.com/telmengedar/processor/internal/openaicompat"
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

	// httpClient is nil so each adapter applies its own default timeout —
	// the timeout policy lives in exactly one place per adapter, not
	// duplicated here (W-13). The model adapter's default is its own
	// constant, generous by intent, and deliberately not the graph
	// adapter's (design §8.4a): a local model on CPU can take minutes,
	// where divoid.DefaultTimeout would turn the ruling's own target into
	// a service that never completes a run.
	graph := divoid.NewClient(cfg.divoidURL, cfg.divoidKey, nil)
	model := openaicompat.NewClient(cfg.modelURL, cfg.modelID, cfg.modelKey, nil)
	// logger is the same value server.Serve already receives (design
	// §10.3's one diagnostic channel) — a supplementary-recall transport
	// failure's detail goes here, never into the model prompt or the
	// written record (design §6.4a, #10821's open finding).
	turn := loop.NewTurn(graph, model, systemText, cfg.modelID, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Serve(ctx, ln, server.NewHandler(turn), logger); err != nil {
		logger.Error("serve", "error", err)
		return 1
	}

	return 0
}
