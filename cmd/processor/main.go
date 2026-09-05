// Command processor is the Processor service entry point.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/telmengedar/processor/internal/boot"
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

	addr, err := boot.LoadHTTPAddr()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	graphCfg, err := boot.LoadGraph()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	modelCfg, err := boot.LoadModel()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "addr", addr, "error", err)
		return 1
	}

	sampling := loop.Sampling{Temperature: modelCfg.Temperature, TopP: modelCfg.TopP}

	graph := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)
	model := openaicompat.NewClient(modelCfg.URL, modelCfg.ID, modelCfg.Key, sampling, nil)
	turn := loop.NewTurn(graph, model, systemText, modelCfg.ID, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Serve(ctx, ln, server.NewHandler(turn), logger); err != nil {
		logger.Error("serve", "error", err)
		return 1
	}

	return 0
}
