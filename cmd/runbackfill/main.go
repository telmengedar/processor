// Command runbackfill re-composes existing run-record nodes into the account-led shape internal/divoid.WriteRun produces.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/runbackfill"
)

const (
	exitUsage = 2
	exitError = 1
)

type options struct {
	ids       []int64
	force     bool
	dryRun    bool
	backupDir string
}

type backupRecord struct {
	Node        int64  `json:"node"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
	Substance   string `json:"substance"`
	CapturedAt  string `json:"capturedAt"`
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

	client := divoid.NewClient(graphCfg.URL, graphCfg.Key, nil, logger)
	graph := &graphAdapter{client: client}

	runOpts := runbackfill.Options{Force: opts.force, DryRun: opts.dryRun}
	if !opts.dryRun {
		if err := os.MkdirAll(opts.backupDir, 0o755); err != nil {
			logger.Error("backup", "error", err)
			return exitError
		}
		runOpts.Backup = func(_ context.Context, node runbackfill.Node) error {
			return writeBackupFile(opts.backupDir, node)
		}
	}

	result := runbackfill.Run(context.Background(), graph, opts.ids, runOpts, time.Now)

	if err := runbackfill.Render(result, machine, human); err != nil {
		logger.Error("render", "error", err)
		return exitError
	}

	if result.OperationalFailures() > 0 {
		return exitError
	}
	return 0
}

func writeBackupFile(dir string, node runbackfill.Node) error {
	rec := backupRecord{
		Node:        node.ID,
		Type:        node.Type,
		Name:        node.Name,
		ContentType: node.ContentType,
		Content:     node.Content,
		Substance:   node.Substance,
		CapturedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	path := filepath.Join(dir, strconv.FormatInt(node.ID, 10)+".json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create backup file %s: %w", path, err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	encErr := enc.Encode(rec)
	closeErr := f.Close()
	if encErr != nil {
		return fmt.Errorf("write backup file %s: %w", path, encErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close backup file %s: %w", path, closeErr)
	}
	return nil
}

func parseFlags(args []string, human io.Writer) (options, bool) {
	flags := flag.NewFlagSet("runbackfill", flag.ContinueOnError)
	flags.SetOutput(human)
	ids := flags.String("ids", "", "comma-separated node ids to backfill (required)")
	force := flags.Bool("force", false, "re-compose nodes that already carry the account-led shape")
	dryRun := flags.Bool("dry-run", false, "compose and report without writing anything to the graph")
	backupDir := flags.String("backup-dir", "", "directory to capture each target node's original content/substance into before writing; required unless -dry-run")

	if err := flags.Parse(args); err != nil {
		return options{}, false
	}

	parsed, err := parseIDs(*ids)
	if err != nil {
		fmt.Fprintln(human, err)
		return options{}, false
	}

	if *backupDir == "" && !*dryRun {
		fmt.Fprintln(human, "-backup-dir is required for a live (non-dry-run) pass — refusing to write without a rollback capture")
		return options{}, false
	}

	return options{ids: parsed, force: *force, dryRun: *dryRun, backupDir: *backupDir}, true
}

func parseIDs(raw string) ([]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("-ids is required and names no node id")
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
