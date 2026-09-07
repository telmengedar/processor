// Command processor is the Processor service entry point.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
	"github.com/telmengedar/processor/internal/server"
	"github.com/telmengedar/processor/internal/workspace"
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

	workspaceDir, err := boot.LoadWorkspaceDir()
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

	model, err := newModel(modelCfg, sampling)
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	var files loop.FilePort
	if workspaceDir != "" {
		files = workspace.New(workspaceDir)
	}

	graph := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)
	turn := loop.NewTurn(graph, model, files, systemText, modelCfg.ID, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Serve(ctx, ln, server.NewHandler(turn), logger); err != nil {
		logger.Error("serve", "error", err)
		return 1
	}

	return 0
}

func newModel(cfg boot.ModelConfig, sampling loop.Sampling) (loop.ModelPort, error) {
	switch cfg.Protocol {
	case boot.ProtocolOpenAICompat:
		return openaicompat.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil), nil
	case boot.ProtocolOllama:
		return ollama.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil), nil
	}
	return nil, fmt.Errorf("model protocol %q has no adapter in this binary", cfg.Protocol)
}
