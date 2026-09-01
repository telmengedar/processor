package loop

import (
	"context"
	"errors"
	"testing"
)

// fakeGraph is a GraphPort test double, justified by the deletion
// experiment in design §9.2 seam 1: without it, Turn's tests for the
// not-found branch, the two failure branches and the recall query would
// have to run against a live, shared graph.
type fakeGraph struct {
	node      Anchor
	nodeFound bool
	nodeErr   error

	candidates []Candidate
	recallErr  error

	recallQuery string
	recallLimit int
}

func (f *fakeGraph) Node(_ context.Context, _ int64) (Anchor, bool, error) {
	return f.node, f.nodeFound, f.nodeErr
}

func (f *fakeGraph) Recall(_ context.Context, query string, limit int) ([]Candidate, error) {
	f.recallQuery = query
	f.recallLimit = limit
	return f.candidates, f.recallErr
}

func TestTurnRunReturnsSubjectNotFound(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: false}
	turn := NewTurn(graph)

	_, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("Run() err = %v, want ErrSubjectNotFound", err)
	}
}

func TestTurnRunWrapsNodeTransportFailureAsGraphUnavailable(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeErr: errors.New("literal: connection refused")}
	turn := NewTurn(graph)

	_, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable", err)
	}
}

// TestTurnRunDoesNotRecallWhenTheAnchorIsNotFound pins design §6.5: an
// anchor failure means the run has no subject, and recall must not run.
func TestTurnRunDoesNotRecallWhenTheAnchorIsNotFound(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: false}
	turn := NewTurn(graph)

	if _, err := turn.Run(context.Background(), "hello", 42); err == nil {
		t.Fatal("Run() returned nil error for a missing subject, want ErrSubjectNotFound")
	}
	if graph.recallQuery != "" || graph.recallLimit != 0 {
		t.Fatalf("Recall was called (query=%q limit=%d) after the anchor was not found, want no call", graph.recallQuery, graph.recallLimit)
	}
}

func TestTurnRunWrapsRecallFailureAsGraphUnavailable(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{
		node:      Anchor{ID: 42, Content: "anchor body"},
		nodeFound: true,
		recallErr: errors.New("literal: 500 from graph"),
	}
	turn := NewTurn(graph)

	_, err := turn.Run(context.Background(), "hello", 42)
	if !errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable", err)
	}
}

// TestTurnRunPassesInputVerbatimAsTheRecallQuery pins design S3: no
// rewriting, no expansion, no model between the input and the query.
func TestTurnRunPassesInputVerbatimAsTheRecallQuery(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{node: Anchor{ID: 42}, nodeFound: true}
	turn := NewTurn(graph)

	// Deliberately not already lowercase/trimmed/single-spaced (CF-1): a
	// fixture that already satisfies those normalizations can't fail when
	// Run silently applies one, so this pins that none of them happen.
	const input = "  Why DOES   the Assembler ignore SCOPE?  "
	record, err := turn.Run(context.Background(), input, 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if graph.recallQuery != input {
		t.Fatalf("recall query = %q, want the input verbatim %q", graph.recallQuery, input)
	}
	if record.Query != input {
		t.Fatalf("record.Query = %q, want %q", record.Query, input)
	}
	if record.Input != input {
		t.Fatalf("record.Input = %q, want %q", record.Input, input)
	}
}

func TestTurnRunUsesTheCandidateLimitConstant(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{node: Anchor{ID: 42}, nodeFound: true}
	turn := NewTurn(graph)

	if _, err := turn.Run(context.Background(), "q", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if graph.recallLimit != CandidateLimit {
		t.Fatalf("recall limit = %d, want CandidateLimit (%d)", graph.recallLimit, CandidateLimit)
	}
}

func TestTurnRunSummarizesTheFetchedAnchorIntoTheRecord(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{
		node:      Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "body text"},
		nodeFound: true,
	}
	turn := NewTurn(graph)

	record, err := turn.Run(context.Background(), "q", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Subject != 42 {
		t.Fatalf("record.Subject = %d, want 42", record.Subject)
	}
	if record.Anchor.ID != 42 || record.Anchor.Type != "documentation" || record.Anchor.Name != "Subject" {
		t.Fatalf("record.Anchor = %+v, want the fetched anchor summarized", record.Anchor)
	}
	if record.Anchor.Size != len("body text") {
		t.Fatalf("record.Anchor.Size = %d, want %d", record.Anchor.Size, len("body text"))
	}
}

// TestTurnRunTwoTurnsDoNotShareState pins design §6.5's falsifier: two
// Turn values built over independent graphs must not observe each other.
func TestTurnRunTwoTurnsDoNotShareState(t *testing.T) {
	t.Parallel()

	turnA := NewTurn(&fakeGraph{node: Anchor{ID: 1, Content: "a"}, nodeFound: true})
	turnB := NewTurn(&fakeGraph{node: Anchor{ID: 2, Content: "b"}, nodeFound: true})

	recA, err := turnA.Run(context.Background(), "qa", 1)
	if err != nil {
		t.Fatalf("turnA.Run: %v", err)
	}
	recB, err := turnB.Run(context.Background(), "qb", 2)
	if err != nil {
		t.Fatalf("turnB.Run: %v", err)
	}

	if recA.Subject == recB.Subject {
		t.Fatalf("both turns reported subject %d, want them independent", recA.Subject)
	}
}
