package measure

import (
	"context"
	"slices"
	"sync"

	"github.com/telmengedar/processor/internal/condense"
)

var _ condense.GraphPort = (*CondenseGraph)(nil)

// SuppressedSubstance is one substance the fill generated and this port declined to write.
type SuppressedSubstance struct {
	Ordinal int   `json:"ordinal"`
	Node    int64 `json:"node"`
	Size    int   `json:"size"`
}

// CondenseGraph decorates a condense graph port so the fill runs and reads in full and no substance reaches the graph.
type CondenseGraph struct {
	reads condense.GraphPort

	mu         sync.Mutex
	suppressed []SuppressedSubstance
}

// NewCondenseGraph decorates reads so every read passes through and no substance is ever written.
func NewCondenseGraph(reads condense.GraphPort) *CondenseGraph {
	return &CondenseGraph{reads: reads}
}

// NodeWithSubstance fetches id with its substance through the decorated port, unchanged.
func (g *CondenseGraph) NodeWithSubstance(ctx context.Context, id int64) (condense.Node, bool, error) {
	return g.reads.NodeWithSubstance(ctx, id)
}

// Content re-reads id's body through the decorated port, unchanged.
func (g *CondenseGraph) Content(ctx context.Context, id int64) (string, bool, error) {
	return g.reads.Content(ctx, id)
}

// SetSubstance writes nothing: it records the substance's node and size and reports the success the fill would have been told.
func (g *CondenseGraph) SetSubstance(_ context.Context, id int64, substance string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.suppressed = append(g.suppressed, SuppressedSubstance{Ordinal: len(g.suppressed) + 1, Node: id, Size: len(substance)})

	return nil
}

// Substances returns every substance this port declined to write, in the order it saw them.
func (g *CondenseGraph) Substances() []SuppressedSubstance {
	g.mu.Lock()
	defer g.mu.Unlock()

	return slices.Clone(g.suppressed)
}

// SubstancesSince returns the substances this port declined to write after the first mark of them.
func (g *CondenseGraph) SubstancesSince(mark int) []SuppressedSubstance {
	g.mu.Lock()
	defer g.mu.Unlock()

	if mark < 0 || mark > len(g.suppressed) {
		mark = len(g.suppressed)
	}
	return slices.Clone(g.suppressed[mark:])
}
