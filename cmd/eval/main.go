// Command eval sweeps a retrieval corpus against the graph and reports what retrieval delivered.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/eval"
)

const (
	exitUsage = 2
	exitError = 1
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, machine, human io.Writer) int {
	logger := slog.New(slog.NewTextHandler(human, nil))

	corpusPath, ok := parseFlags(args, human)
	if !ok {
		return exitUsage
	}

	graphCfg, err := boot.LoadGraph()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return exitError
	}

	corpus, err := eval.Load(corpusPath)
	if err != nil {
		logger.Error("corpus", "error", err)
		return exitError
	}

	graph := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)

	result, err := sweep(context.Background(), graph, corpus, time.Now().UTC())
	if err != nil {
		logger.Error("sweep", "error", err)
		return exitError
	}

	if err := eval.Render(result, corpusPath, machine, human); err != nil {
		logger.Error("render", "error", err)
		return exitError
	}

	return exitCodeFor(result)
}

func exitCodeFor(result eval.Result) int {
	if !result.ControlVerifiedRetrieval() {
		return exitError
	}
	return 0
}

func parseFlags(args []string, human io.Writer) (string, bool) {
	flags := flag.NewFlagSet("eval", flag.ContinueOnError)
	flags.SetOutput(human)
	corpusPath := flags.String("corpus", "", "path of the corpus file to sweep")

	if err := flags.Parse(args); err != nil {
		return "", false
	}
	if *corpusPath == "" {
		fmt.Fprintln(human, "-corpus is required but not set")
		flags.Usage()
		return "", false
	}
	return *corpusPath, true
}
