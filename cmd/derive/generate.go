package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
)

const sidecarFileMode = 0o644

const starterCutset = `,.:;"'`

var questionStarters = []string{
	"how", "why", "what", "is", "are", "was", "were", "does", "do", "did",
	"can", "could", "should", "would", "will", "where", "who", "whom",
	"which", "when", "must", "shall", "may", "might",
}

type blindRow struct {
	id    string
	input string
}

func blindRows(corpus eval.Corpus) []blindRow {
	rows := make([]blindRow, 0, len(corpus.Rows))
	for _, row := range corpus.Rows {
		rows = append(rows, blindRow{id: row.ID, input: row.Input})
	}
	return rows
}

func loadBaseline(path string, corpus eval.Corpus) (eval.Derivations, error) {
	if path == "" {
		return eval.Derivations{}, nil
	}
	return eval.LoadDerivations(path, corpus)
}

func selectTargets(rows []blindRow, baseline eval.Derivations, only string, force bool) ([]blindRow, error) {
	if only == "" {
		return unpinnedRows(rows, baseline), nil
	}

	wanted, err := namedRowIDs(only, rows)
	if err != nil {
		return nil, err
	}

	if pinned := pinnedAmong(wanted, baseline); len(pinned) > 0 && !force {
		return nil, fmt.Errorf(
			"-only names rows the baseline already pins: %s; pass -force to regenerate them, which replaces the measured query set those rows were pinned against so a sweep of the result is not comparable to one taken with the baseline",
			strings.Join(pinned, ","))
	}

	targets := make([]blindRow, 0, len(wanted))
	for _, row := range rows {
		if slices.Contains(wanted, row.id) {
			targets = append(targets, row)
		}
	}
	return targets, nil
}

func namedRowIDs(only string, rows []blindRow) ([]string, error) {
	var wanted, unknown []string
	for _, field := range strings.Split(only, ",") {
		id := strings.TrimSpace(field)
		switch {
		case id == "":
		case !slices.ContainsFunc(rows, func(row blindRow) bool { return row.id == id }):
			unknown = append(unknown, id)
		case !slices.Contains(wanted, id):
			wanted = append(wanted, id)
		}
	}

	if len(unknown) > 0 {
		return nil, fmt.Errorf("-only names rows that are not in the corpus: %s", strings.Join(unknown, ","))
	}
	if len(wanted) == 0 {
		return nil, errors.New("-only is set but names no corpus row")
	}
	return wanted, nil
}

func unpinnedRows(rows []blindRow, baseline eval.Derivations) []blindRow {
	var targets []blindRow
	for _, row := range rows {
		if _, pinned := baseline.Queries[row.id]; !pinned {
			targets = append(targets, row)
		}
	}
	return targets
}

func pinnedAmong(wanted []string, baseline eval.Derivations) []string {
	var pinned []string
	for _, id := range wanted {
		if _, ok := baseline.Queries[id]; ok {
			pinned = append(pinned, id)
		}
	}
	return pinned
}

func generate(ctx context.Context, model loop.ModelPort, targets []blindRow, timeout time.Duration, logger *slog.Logger) (map[string][]string, []string, error) {
	generated := make(map[string][]string, len(targets))
	var short []string

	for _, row := range targets {
		queries, err := loop.DeriveQueriesWithin(ctx, model, row.input, timeout)
		if err != nil {
			return nil, nil, fmt.Errorf("row %s: %w", row.id, err)
		}

		for _, problem := range shapeProblems(queries) {
			logger.Warn("shape", "row", row.id, "problem", problem)
		}
		if len(queries) != loop.MaxDerivedQueries {
			short = append(short, row.id)
		}
		logger.Info("generated", "row", row.id, "queries", len(queries))

		generated[row.id] = queries
	}
	return generated, short, nil
}

func shapeProblems(queries []string) []string {
	if len(queries) != loop.MaxDerivedQueries {
		return []string{fmt.Sprintf("expected %d queries, got %d", loop.MaxDerivedQueries, len(queries))}
	}

	var problems []string
	for i, query := range queries[:len(queries)-1] {
		if !looksLikeAQuestion(query) {
			problems = append(problems, fmt.Sprintf("line %d should be phrased as a question: %q", i+1, query))
		}
	}
	if last := queries[len(queries)-1]; looksLikeAQuestion(last) {
		problems = append(problems, fmt.Sprintf("line %d should be a dense keyword line, not phrased as a question: %q", len(queries), last))
	}
	return problems
}

func looksLikeAQuestion(query string) bool {
	trimmed := strings.TrimSpace(query)
	if strings.HasSuffix(trimmed, "?") {
		return true
	}
	first, _, _ := strings.Cut(trimmed, " ")
	return slices.Contains(questionStarters, strings.ToLower(strings.Trim(first, starterCutset)))
}

func mergeSidecar(baseline eval.Derivations, generated map[string][]string, corpus eval.Corpus) []eval.Derivation {
	merged := make([]eval.Derivation, 0, len(corpus.Rows))
	for _, row := range corpus.Rows {
		if queries, regenerated := generated[row.ID]; regenerated {
			merged = append(merged, eval.Derivation{Row: row.ID, Queries: queries, Source: eval.SourceBlindGenerated})
			continue
		}
		if carried, pinned := baseline.Queries[row.ID]; pinned {
			merged = append(merged, eval.Derivation{Row: row.ID, Queries: carried, Source: baseline.Sources[row.ID]})
		}
	}
	return merged
}

func writeSidecar(path string, entries []eval.Derivation) error {
	var buf bytes.Buffer
	if err := encodeSidecar(&buf, entries); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), sidecarFileMode)
}

func encodeSidecar(w io.Writer, entries []eval.Derivation) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}
