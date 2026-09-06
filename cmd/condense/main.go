// Command condense derives substance for a named, bounded set of graph nodes, offline and never inside a turn.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/openaicompat"
)

const (
	exitUsage = 2
	exitError = 1
)

const condenseTimeout = 30 * time.Minute

type options struct {
	corpusPath string
	ids        []int64
	force      bool
	dryRun     bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, machine, human io.Writer) int {
	logger := slog.New(slog.NewTextHandler(human, nil))

	opts, ok := parseFlags(args, human)
	if !ok {
		return exitUsage
	}

	graphCfg, err := boot.LoadGraph()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return exitError
	}

	modelCfg, err := boot.LoadModel()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return exitError
	}

	targets, err := resolveTargets(opts)
	if err != nil {
		logger.Error("targets", "error", err)
		return exitError
	}

	graph := &graphAdapter{client: divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)}
	sampling := loop.Sampling{Temperature: modelCfg.Temperature, TopP: modelCfg.TopP}
	modelHTTP := &http.Client{Timeout: condenseTimeout}
	model := &modelAdapter{client: openaicompat.NewClient(modelCfg.URL, modelCfg.ID, modelCfg.Key, sampling, modelHTTP)}

	result := condense.Run(context.Background(), graph, model, targets,
		condense.Options{Force: opts.force, DryRun: opts.dryRun}, time.Now)

	if err := condense.Render(result, machine, human); err != nil {
		logger.Error("render", "error", err)
		return exitError
	}

	if result.OperationalFailures() > 0 {
		return exitError
	}
	return 0
}

func resolveTargets(opts options) ([]condense.Target, error) {
	if opts.corpusPath == "" {
		return condense.TargetsFromIDs(opts.ids), nil
	}

	corpus, err := eval.Load(opts.corpusPath)
	if err != nil {
		return nil, err
	}
	return condense.TargetsFromCorpus(corpus), nil
}

func parseFlags(args []string, human io.Writer) (options, bool) {
	flags := flag.NewFlagSet("condense", flag.ContinueOnError)
	flags.SetOutput(human)
	corpus := flags.String("corpus", "", "path of the corpus file whose required nodes are the target set")
	ids := flags.String("ids", "", "comma-separated node ids to condense, as an alternative to -corpus")
	force := flags.Bool("force", false, "re-derive substance on nodes that already carry one")
	dryRun := flags.Bool("dry-run", false, "condense and report without writing anything to the graph")

	if err := flags.Parse(args); err != nil {
		return options{}, false
	}

	if (*corpus == "") == (*ids == "") {
		fmt.Fprintln(human, "exactly one of -corpus and -ids is required")
		flags.Usage()
		return options{}, false
	}

	parsed, err := parseIDs(*ids)
	if err != nil {
		fmt.Fprintln(human, err)
		return options{}, false
	}

	return options{corpusPath: *corpus, ids: parsed, force: *force, dryRun: *dryRun}, true
}

func parseIDs(raw string) ([]int64, error) {
	if raw == "" {
		return nil, nil
	}

	fields := strings.Split(raw, ",")
	ids := make([]int64, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("-ids contains %q, which is not a node id", trimmed)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("-ids is set but names no node id")
	}
	return ids, nil
}
