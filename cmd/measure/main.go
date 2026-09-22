// Command measure runs one turn under observation: it reads the live graph, takes its task text on standard input, and files no record.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/measure"
	"github.com/telmengedar/processor/internal/ports"
	"github.com/telmengedar/processor/internal/systemtext"
	"github.com/telmengedar/processor/internal/workspace"
)

const (
	exitUsage = 2
	exitError = 1
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, task io.Reader, machine, human io.Writer) int {
	logger := slog.New(slog.NewTextHandler(human, nil))

	subject, ok := parseFlags(args, human)
	if !ok {
		return exitUsage
	}

	input, ok := readTask(task, human)
	if !ok {
		return exitUsage
	}

	runner, err := build(logger)
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return exitError
	}

	result, err := runner.Run(context.Background(), input, subject)
	if err != nil {
		logger.Error("measured run", "subject", subject, "error", err)
		return exitError
	}

	if err := json.NewEncoder(machine).Encode(result); err != nil {
		logger.Error("result", "error", err)
		return exitError
	}

	return 0
}

func build(logger *slog.Logger) (*measure.Runner, error) {
	graphCfg, err := boot.LoadGraph()
	if err != nil {
		return nil, err
	}

	modelCfg, err := boot.LoadModel()
	if err != nil {
		return nil, err
	}

	condenseCfg, condenseConfigured, err := boot.LoadCondenseModel()
	if err != nil {
		return nil, err
	}

	workspaceDir, err := boot.LoadWorkspaceDir()
	if err != nil {
		return nil, err
	}

	model, err := ports.Model(modelCfg, loop.Sampling{Temperature: modelCfg.Temperature, TopP: modelCfg.TopP})
	if err != nil {
		return nil, err
	}

	client := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)
	substances := measure.NewCondenseGraph(ports.FillGraph(client))

	fillPort, err := ports.Fill(condenseCfg, condenseConfigured, substances)
	if err != nil {
		return nil, err
	}
	if fillPort == nil {
		logger.Info("the fill is off: no PROCESSOR_CONDENSE_MODEL_* configuration was found, and a measured run with the fill off is not the configuration the service runs")
	}

	var files loop.FilePort
	if workspaceDir != "" {
		files = workspace.New(workspaceDir)
	}

	graph := measure.NewGraph(client, nil, logger)

	return measure.NewRunner(graph, substances, model, files, fillPort, systemtext.Text, modelCfg.ID, logger), nil
}

func readTask(task io.Reader, human io.Writer) (string, bool) {
	text, err := io.ReadAll(task)
	if err != nil {
		fmt.Fprintf(human, "the task text could not be read from standard input: %v\n", err)
		return "", false
	}
	if strings.TrimSpace(string(text)) == "" {
		fmt.Fprintln(human, "the task text is read from standard input and this one is empty")
		return "", false
	}
	return string(text), true
}

func parseFlags(args []string, human io.Writer) (subject int64, ok bool) {
	flags := flag.NewFlagSet("measure", flag.ContinueOnError)
	flags.SetOutput(human)
	id := flags.Int64("subject", 0, "id of the anchor node the run is about; the task text itself is read from standard input")

	if err := flags.Parse(args); err != nil {
		return 0, false
	}
	if *id <= 0 {
		fmt.Fprintf(human, "-subject is %d, and a node id is a positive integer\n", *id)
		flags.Usage()
		return 0, false
	}
	return *id, true
}
