// Package runbackfill re-composes existing run-record nodes into the account-led shape internal/divoid.WriteRun produces.
package runbackfill

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

const (
	skipNodeAbsent           = "node absent"
	skipNotRunRecord         = "not a run record"
	skipContentAbsent        = "content absent"
	skipAlreadyBackfilled    = "already backfilled"
	skipUnrecognisedShape    = "content is neither bare JSON nor a recognisable fenced record"
	skipNameUnparseable      = "node name does not carry a parseable timestamp"
	skipReadFailed           = "graph read failed"
	skipContentMoved         = "content changed since it was read, write refused"
	skipWriteContentFailed   = "content write failed"
	skipWriteSubstanceFailed = "substance write failed (content was written)"
)

var runNamePattern = regexp.MustCompile(`^processor-run (\S+) — `)

const (
	fenceOpenNeedle  = "\n```json\n"
	fenceCloseNeedle = "\n```"
)

// Node is one graph row as the pass reads it.
type Node struct {
	ID          int64
	Type        string
	Name        string
	ContentType string
	Content     string
	Substance   string
}

// GraphPort is the seam between the pass and the graph.
type GraphPort interface {
	NodeWithSubstance(ctx context.Context, id int64) (Node, bool, error)
	Content(ctx context.Context, id int64) (string, bool, error)
	SetRunContent(ctx context.Context, id int64, content []byte) error
	SetSubstance(ctx context.Context, id int64, substance string) error
}

// Options are the two switches the pass takes: re-derive over an already-backfilled record, and run without writing.
type Options struct {
	Force  bool
	DryRun bool
}

// Provenance is one node's recompose, including the rendered account.
type Provenance struct {
	Node              int64     `json:"node"`
	Name              string    `json:"name"`
	At                time.Time `json:"at"`
	RecordSize        int       `json:"recordSize"`
	AccountSize       int       `json:"accountSize"`
	NewContentSize    int       `json:"newContentSize"`
	Account           string    `json:"account"`
	AlreadyBackfilled bool      `json:"alreadyBackfilled"`
	ContentWritten    bool      `json:"contentWritten"`
	SubstanceWritten  bool      `json:"substanceWritten"`
}

// Skip is one node the pass did not fully write to, and the rule or failure that stopped it.
type Skip struct {
	Node   int64  `json:"node"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

// Result is one pass's whole output.
type Result struct {
	RanAt       time.Time    `json:"ranAt"`
	DryRun      bool         `json:"dryRun"`
	Forced      bool         `json:"forced"`
	TargetCount int          `json:"targetCount"`
	Provenance  []Provenance `json:"provenance"`
	Skipped     []Skip       `json:"skipped"`
}

// OperationalFailures counts the skips that are a failure of the pass rather than a rule it applied on purpose.
func (r Result) OperationalFailures() int {
	failures := 0
	for _, s := range r.Skipped {
		switch s.Reason {
		case skipNotRunRecord, skipContentAbsent, skipAlreadyBackfilled:
			continue
		default:
			failures++
		}
	}
	return failures
}

// Run recomposes every target node, isolating each node's failure from the rest of the pass.
func Run(ctx context.Context, graph GraphPort, ids []int64, opts Options, now func() time.Time) Result {
	result := Result{
		RanAt:       now().UTC(),
		DryRun:      opts.DryRun,
		Forced:      opts.Force,
		TargetCount: len(ids),
		Provenance:  make([]Provenance, 0, len(ids)),
		Skipped:     make([]Skip, 0, len(ids)),
	}

	for _, id := range ids {
		prov, skip := backfillOne(ctx, graph, id, opts)
		if prov != nil {
			result.Provenance = append(result.Provenance, *prov)
		}
		if skip != nil {
			result.Skipped = append(result.Skipped, *skip)
		}
	}

	return result
}

func backfillOne(ctx context.Context, graph GraphPort, id int64, opts Options) (*Provenance, *Skip) {
	node, found, err := graph.NodeWithSubstance(ctx, id)
	if err != nil {
		return nil, &Skip{Node: id, Reason: skipReadFailed, Detail: err.Error()}
	}
	if !found {
		return nil, &Skip{Node: id, Reason: skipNodeAbsent}
	}
	if !divoid.IsRunRecord(node.Type, node.Name) {
		return nil, &Skip{Node: id, Reason: skipNotRunRecord, Detail: fmt.Sprintf("type=%s name=%q", node.Type, node.Name)}
	}
	if strings.TrimSpace(node.Content) == "" {
		return nil, &Skip{Node: id, Reason: skipContentAbsent}
	}

	raw, record, alreadyBackfilled, err := classify(node.Content)
	if err != nil {
		return nil, &Skip{Node: id, Reason: skipUnrecognisedShape, Detail: err.Error()}
	}
	if alreadyBackfilled && !opts.Force {
		return nil, &Skip{Node: id, Reason: skipAlreadyBackfilled}
	}

	at, err := parseRunAt(node.Name)
	if err != nil {
		return nil, &Skip{Node: id, Reason: skipNameUnparseable, Detail: err.Error()}
	}

	account := loop.RenderSummary(record, at)
	newContent := divoid.ComposeRunContent(account, raw)

	prov := Provenance{
		Node:              id,
		Name:              node.Name,
		At:                at.UTC(),
		RecordSize:        len(raw),
		AccountSize:       len(account),
		NewContentSize:    len(newContent),
		Account:           account,
		AlreadyBackfilled: alreadyBackfilled,
	}

	if opts.DryRun {
		return &prov, nil
	}

	live, found, err := graph.Content(ctx, id)
	if err != nil {
		return nil, &Skip{Node: id, Reason: skipReadFailed, Detail: err.Error()}
	}
	if !found || live != node.Content {
		return nil, &Skip{Node: id, Reason: skipContentMoved}
	}

	if err := graph.SetRunContent(ctx, id, newContent); err != nil {
		return nil, &Skip{Node: id, Reason: skipWriteContentFailed, Detail: err.Error()}
	}
	prov.ContentWritten = true

	if err := graph.SetSubstance(ctx, id, account); err != nil {
		return &prov, &Skip{Node: id, Reason: skipWriteSubstanceFailed, Detail: err.Error()}
	}
	prov.SubstanceWritten = true

	return &prov, nil
}

func classify(content string) (raw []byte, record loop.Record, alreadyBackfilled bool, err error) {
	var legacy loop.Record
	if err := json.Unmarshal([]byte(content), &legacy); err == nil {
		return []byte(content), legacy, false, nil
	}

	fenced, ok := extractLastFencedJSON(content)
	if !ok {
		return nil, loop.Record{}, false, fmt.Errorf("content has no closing json fence and does not parse whole as a Record")
	}

	var backfilled loop.Record
	if err := json.Unmarshal(fenced, &backfilled); err != nil {
		return nil, loop.Record{}, false, fmt.Errorf("last fenced json block does not parse as a Record: %w", err)
	}
	return fenced, backfilled, true, nil
}

func extractLastFencedJSON(content string) ([]byte, bool) {
	openAt := strings.LastIndex(content, fenceOpenNeedle)
	if openAt == -1 {
		return nil, false
	}
	start := openAt + len(fenceOpenNeedle)

	closeAt := strings.Index(content[start:], fenceCloseNeedle)
	if closeAt == -1 {
		return nil, false
	}
	return []byte(content[start : start+closeAt]), true
}

func parseRunAt(name string) (time.Time, error) {
	m := runNamePattern.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, fmt.Errorf("name %q does not match \"processor-run <RFC3339> — ...\"", name)
	}
	at, err := time.Parse(time.RFC3339, m[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("name %q carries %q, which is not RFC3339: %w", name, m[1], err)
	}
	return at, nil
}
