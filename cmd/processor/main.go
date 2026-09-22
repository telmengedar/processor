// Command processor is the Processor service entry point.
package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ports"
	"github.com/telmengedar/processor/internal/server"
	"github.com/telmengedar/processor/internal/systemtext"
	"github.com/telmengedar/processor/internal/workspace"
)

func main() {
	os.Exit(run(os.Stderr))
}

func run(human io.Writer) int {
	logger := slog.New(slog.NewTextHandler(human, nil))

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

	condenseCfg, condenseConfigured, err := boot.LoadCondenseModel()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	workspaceDir, err := boot.LoadWorkspaceDir()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	if err := ports.StateThinkingSuppression(logger, "judgement", modelCfg.Protocol); err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	if condenseConfigured {
		if err := ports.StateThinkingSuppression(logger, "fill", condenseCfg.Protocol); err != nil {
			logger.Error("boot configuration", "error", err)
			return 1
		}
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "addr", addr, "error", err)
		return 1
	}

	sampling := loop.Sampling{Temperature: modelCfg.Temperature, TopP: modelCfg.TopP}

	model, err := ports.Model(modelCfg, sampling)
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}

	var files loop.FilePort
	if workspaceDir != "" {
		files = workspace.New(workspaceDir)
	}

	graph := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)
	turn := loop.NewTurn(graph, model, files, systemtext.Text, modelCfg.ID, logger)

	fillPort, err := ports.Fill(condenseCfg, condenseConfigured, ports.FillGraph(graph))
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return 1
	}
	turn.Fill = fillPort
	if fillPort == nil {
		logger.Info("the fill is off: no PROCESSOR_CONDENSE_MODEL_* configuration was found")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Serve(ctx, ln, server.NewHandler(turn), logger); err != nil {
		logger.Error("serve", "error", err)
		return 1
	}

	return 0
}
