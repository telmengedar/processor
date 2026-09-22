// Package runbackfill re-composes existing run-record nodes into the account-led shape internal/divoid.WriteRun produces.
package runbackfill

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runarchive"
)

const (
	skipNodeAbsent           = "node absent"
	skipNotRunRecord         = "not a run record"
	skipContentAbsent        = runarchive.SkipContentAbsent
	skipAlreadyBackfilled    = "already backfilled"
	skipUnrecognisedShape    = runarchive.SkipUnrecognisedShape
	skipNameUnparseable      = runarchive.SkipNameUnparseable
	skipReadFailed           = "graph read failed"
	skipContentMoved         = "content changed since it was read, write refused"
	skipBackupFailed         = "backup failed, write refused"
	skipWriteContentFailed   = "content write failed"
	skipWriteSubstanceFailed = "substance write failed (content was written)"
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

// Options are the pass's switches: re-derive over an already-backfilled record, run without writing, and where to capture a node's bytes before they are overwritten.
type Options struct {
	Force  bool
	DryRun bool
	Backup func(ctx context.Context, node Node) error
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

	raw, record, alreadyBackfilled, err := runarchive.Classify(node.Content)
	if err != nil {
		return nil, &Skip{Node: id, Reason: skipUnrecognisedShape, Detail: err.Error()}
	}
	if alreadyBackfilled && !opts.Force {
		return nil, &Skip{Node: id, Reason: skipAlreadyBackfilled}
	}

	at, err := runarchive.ParseRunInstant(node.Name)
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

	if opts.Backup != nil {
		backedUp := Node{ID: id, Type: node.Type, Name: node.Name, ContentType: node.ContentType, Content: live, Substance: node.Substance}
		if err := opts.Backup(ctx, backedUp); err != nil {
			return nil, &Skip{Node: id, Reason: skipBackupFailed, Detail: err.Error()}
		}
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
