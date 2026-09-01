package loop

import (
	"context"
	"errors"
	"fmt"
)

// CandidateLimit and AssemblyByteBudget are constants, not configuration
// (design §8.4): no operator tunes them, and milestone 2 is the named
// event at which they become measurable.
const (
	CandidateLimit     = 20
	AssemblyByteBudget = 60_000
)

// ErrSubjectNotFound is returned when the subject id resolves to nothing —
// the graph's empty-result shape (design C30), not a transport error.
var ErrSubjectNotFound = errors.New("subject not found")

// ErrGraphUnavailable wraps any failure reading the graph: transport,
// authentication, or a non-2xx status.
var ErrGraphUnavailable = errors.New("graph unavailable")

// GraphPort is the seam between the loop and the graph. Declared here,
// implemented by internal/divoid, constructed in main (design §5.2, §8.3).
// Unit A needs only the two read operations; the write operation arrives
// with unit B, together with its own consumer (design §2.3).
type GraphPort interface {
	// Node fetches the subject node by id, with its content. found is
	// false when the id resolves to nothing.
	Node(ctx context.Context, id int64) (anchor Anchor, found bool, err error)

	// Recall runs one semantic query and returns up to limit candidates in
	// the rank order the graph returned them. The port does not re-sort.
	Recall(ctx context.Context, query string, limit int) ([]Candidate, error)
}

// Turn is one run: fetch the anchor, recall candidates, assemble the
// block. Unit A stops there — no model call, no write-back (design §2.3).
// A Turn holds no mutable state, so two runs never interfere (design §6.5
// falsifier: any package-level mutable variable in internal/loop).
type Turn struct {
	Graph GraphPort
}

// NewTurn builds a Turn over graph.
func NewTurn(graph GraphPort) *Turn {
	return &Turn{Graph: graph}
}

// Run executes one turn for input against subject. Request decoding and
// validation are the caller's job (design §5.5 — the handler owns policy,
// the loop does not); Run starts from an already-validated input and
// subject id.
//
// The query text passed to recall is input, verbatim — no rewriting, no
// expansion, no model (design S3).
func (t *Turn) Run(ctx context.Context, input string, subject int64) (Record, error) {
	anchor, found, err := t.Graph.Node(ctx, subject)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
	}
	if !found {
		return Record{}, ErrSubjectNotFound
	}

	candidates, err := t.Graph.Recall(ctx, input, CandidateLimit)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
	}

	block, dispositions := Assemble(anchor, candidates, AssemblyByteBudget)

	return Record{
		Input:      input,
		Subject:    subject,
		Query:      input,
		Anchor:     summarizeAnchor(anchor),
		Candidates: dispositions,
		Block:      block,
	}, nil
}
