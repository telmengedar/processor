// Command derive regenerates a derivation sidecar through the product's own prompt, port and parse.
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
	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

const (
	exitUsage = 2
	exitError = 1
)

const defaultCallTimeout = 180 * time.Second

type options struct {
	corpusPath      string
	derivationsPath string
	outPath         string
	only            string
	force           bool
	dryRun          bool
	timeout         time.Duration
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
	if writesOverBaseline(opts.outPath, opts.derivationsPath) {
		fmt.Fprintf(human, "-out %q is the baseline sidecar %q; the baseline is the query set every arm-versus-arm figure was measured against, so a regenerated set is written beside it and never over it\n",
			opts.outPath, opts.derivationsPath)
		return exitUsage
	}

	corpus, err := eval.Load(opts.corpusPath)
	if err != nil {
		logger.Error("corpus", "error", err)
		return exitError
	}

	baseline, err := loadBaseline(opts.derivationsPath, corpus)
	if err != nil {
		logger.Error("derivations", "error", err)
		return exitError
	}

	targets, err := selectTargets(blindRows(corpus), baseline, opts.only, opts.force)
	if err != nil {
		logger.Error("targets", "error", err)
		return exitError
	}
	if len(targets) == 0 {
		logger.Info("nothing to generate", "rows", 0, "reason", "every requested row is already pinned by the baseline")
		return 0
	}

	model, err := bootModel()
	if err != nil {
		logger.Error("boot configuration", "error", err)
		return exitError
	}

	generated, err := generate(context.Background(), model, targets, opts.timeout, logger)
	if err != nil {
		logger.Error("generate", "error", err)
		return exitError
	}

	return emit(opts, mergeSidecar(baseline, generated, corpus), len(generated), machine, logger)
}

func emit(opts options, merged []eval.Derivation, generated int, machine io.Writer, logger *slog.Logger) int {
	if opts.dryRun {
		if err := encodeSidecar(machine, merged); err != nil {
			logger.Error("encode", "error", err)
			return exitError
		}
		logger.Info("dry run", "written", false, "rows", len(merged), "generated", generated)
		return 0
	}

	if err := writeSidecar(opts.outPath, merged); err != nil {
		logger.Error("write", "error", err)
		return exitError
	}
	logger.Info("sidecar written", "path", opts.outPath, "rows", len(merged), "generated", generated)
	return 0
}

func bootModel() (loop.ModelPort, error) {
	cfg, err := boot.LoadModel()
	if err != nil {
		return nil, err
	}

	sampling := loop.Sampling{Temperature: cfg.Temperature, TopP: cfg.TopP}
	switch cfg.Protocol {
	case boot.ProtocolOpenAICompat:
		return openaicompat.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil), nil
	case boot.ProtocolOllama:
		return ollama.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil), nil
	}
	return nil, fmt.Errorf("model protocol %q has no adapter in this binary", cfg.Protocol)
}

func writesOverBaseline(outPath, baselinePath string) bool {
	if baselinePath == "" {
		return false
	}

	out, err := os.Stat(outPath)
	if err != nil {
		return false
	}
	baseline, err := os.Stat(baselinePath)
	if err != nil {
		return false
	}
	return os.SameFile(out, baseline)
}

func parseFlags(args []string, human io.Writer) (options, bool) {
	flags := flag.NewFlagSet("derive", flag.ContinueOnError)
	flags.SetOutput(human)
	corpus := flags.String("corpus", "internal/eval/corpus.json", "path of the corpus file whose rows are derived for")
	derivations := flags.String("derivations", "internal/eval/derivations.json", "path of the sidecar read as the baseline; empty treats every corpus row as unpinned")
	out := flags.String("out", "", "path the regenerated sidecar is written to; required, and never the baseline")
	only := flags.String("only", "", "comma-separated row ids to generate, default: every row the baseline leaves unpinned")
	force := flags.Bool("force", false, "allow -only to regenerate rows the baseline already pins")
	dryRun := flags.Bool("dry-run", false, "write the regenerated sidecar to stdout instead of to -out")
	timeout := flags.Duration("timeout", defaultCallTimeout, "bound on one derivation call")

	if err := flags.Parse(args); err != nil {
		return options{}, false
	}
	if *out == "" && !*dryRun {
		fmt.Fprintln(human, "-out is required but not set")
		flags.Usage()
		return options{}, false
	}

	return options{
		corpusPath:      *corpus,
		derivationsPath: *derivations,
		outPath:         *out,
		only:            *only,
		force:           *force,
		dryRun:          *dryRun,
		timeout:         *timeout,
	}, true
}
