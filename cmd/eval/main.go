// Command eval sweeps a retrieval corpus against the graph and reports what retrieval delivered.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
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

	corpusPath, derivationsPath, ok := parseFlags(args, human)
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

	derivations, err := loadDerivations(derivationsPath, corpus)
	if err != nil {
		logger.Error("derivations", "error", err)
		return exitError
	}
	warnOnUnpinnedRows(logger, derivations, corpus)

	graph := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)

	result, err := sweep(context.Background(), graph, corpus, derivations, time.Now().UTC())
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

func loadDerivations(path string, corpus eval.Corpus) (eval.Derivations, error) {
	if path == "" {
		return eval.Derivations{}, nil
	}
	return eval.LoadDerivations(path, corpus)
}

// warnOnUnpinnedRows names, on the operator stream, any corpus row the
// derivation sidecar does not pin. LoadDerivations validates that every
// sidecar row resolves to a corpus row; it cannot validate the reverse
// without the corpus that grew past it, which is exactly what happened when
// a corpus grew from thirteen rows to twenty-five and the sidecar did not
// grow with it. Left unreported, those rows sweep on raw input on every arm
// and are identical between arms by construction, diluting every rate the
// sweep prints without anything saying so. This is a warning, not a hard
// failure: it is a defect in the corpus/sidecar pair, not in the sweep the
// operator asked for, and failing the run would block every existing
// invocation until every sidecar is regenerated in full.
func warnOnUnpinnedRows(logger *slog.Logger, derivations eval.Derivations, corpus eval.Corpus) {
	unpinned := derivations.Unpinned(corpus)
	if len(unpinned) == 0 {
		return
	}
	logger.Warn("derivations", "unpinned", len(unpinned), "of", len(corpus.Rows),
		"rows", strings.Join(unpinned, ","))
}

func parseFlags(args []string, human io.Writer) (corpusPath, derivationsPath string, ok bool) {
	flags := flag.NewFlagSet("eval", flag.ContinueOnError)
	flags.SetOutput(human)
	corpus := flags.String("corpus", "", "path of the corpus file to sweep")
	derivations := flags.String("derivations", "", "path of the pinned derivation sidecar to sweep with; absent, every row is swept on its own input alone")

	if err := flags.Parse(args); err != nil {
		return "", "", false
	}
	if *corpus == "" {
		fmt.Fprintln(human, "-corpus is required but not set")
		flags.Usage()
		return "", "", false
	}
	return *corpus, *derivations, true
}
