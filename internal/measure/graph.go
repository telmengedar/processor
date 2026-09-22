// Package measure runs the shipped turn under observation: its reads reach the graph and the record it produces never does.
package measure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

var _ loop.GraphPort = (*Graph)(nil)

const logRecordNotSized = "the suppressed record could not be sized"

// UnsizedRecord is the size a suppressed write reports when its record could not be encoded, and no record has it as a real size.
const UnsizedRecord = -1

// SuppressedWrite is one record WriteRun declined to file, and what filing it would have cost the graph.
type SuppressedWrite struct {
	Ordinal int   `json:"ordinal"`
	Subject int64 `json:"subject"`
	Size    int   `json:"size"`
}

// Graph decorates a graph port so that a run's reads reach the graph and its record never does.
type Graph struct {
	reads  loop.GraphPort
	clock  func() time.Time
	logger *slog.Logger

	mu         sync.Mutex
	suppressed []SuppressedWrite
}

// NewGraph decorates reads with clock dating the record it would have filed and logger taking what it declined to size; nil clock is the wall clock.
func NewGraph(reads loop.GraphPort, clock func() time.Time, logger *slog.Logger) *Graph {
	return &Graph{reads: reads, clock: clock, logger: logger}
}

// Node fetches the subject node by id through the decorated port, unchanged.
func (g *Graph) Node(ctx context.Context, id int64) (loop.Anchor, bool, error) {
	return g.reads.Node(ctx, id)
}

// Recall returns the decorated port's candidates for query, unchanged.
func (g *Graph) Recall(ctx context.Context, query string, limit int, scope []int64, window loop.UpdateWindow) ([]loop.Candidate, error) {
	return g.reads.Recall(ctx, query, limit, scope, window)
}

// Neighbours returns the decorated port's neighbourhood of id, unchanged.
func (g *Graph) Neighbours(ctx context.Context, id int64) ([]int64, error) {
	return g.reads.Neighbours(ctx, id)
}

// WriteRun files nothing: it records that the record was produced and how large it would have been, and reports that no node holds it.
func (g *Graph) WriteRun(_ context.Context, record loop.Record) loop.WriteReceipt {
	size, err := FiledSize(record, g.now())
	if err != nil {
		g.log().Error(logRecordNotSized, "subject", record.Subject, "error", err)
		size = UnsizedRecord
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.suppressed = append(g.suppressed, SuppressedWrite{Ordinal: len(g.suppressed) + 1, Subject: record.Subject, Size: size})

	return loop.WriteReceipt{State: loop.NotStored}
}

// Suppressed returns every record this port declined to file, in the order it saw them.
func (g *Graph) Suppressed() []SuppressedWrite {
	g.mu.Lock()
	defer g.mu.Unlock()

	return slices.Clone(g.suppressed)
}

// SuppressedSince returns the records this port declined to file after the first mark of them.
func (g *Graph) SuppressedSince(mark int) []SuppressedWrite {
	g.mu.Lock()
	defer g.mu.Unlock()

	if mark < 0 || mark > len(g.suppressed) {
		mark = len(g.suppressed)
	}
	return slices.Clone(g.suppressed[mark:])
}

// FiledSize is the byte length of the node content a run record would have been filed as, dated at.
func FiledSize(record loop.Record, at time.Time) (int, error) {
	encoded, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("measure: encode run record: %w", err)
	}

	return len(divoid.ComposeRunContent(loop.RenderSummary(record, at), encoded)), nil
}

func (g *Graph) now() time.Time {
	if g.clock == nil {
		return time.Now()
	}
	return g.clock()
}

func (g *Graph) log() *slog.Logger {
	if g.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return g.logger
}
