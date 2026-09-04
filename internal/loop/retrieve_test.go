package loop

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type linkedGraph struct {
	edges map[int64][]int64
}

func (g *linkedGraph) Node(_ context.Context, id int64) (Anchor, bool, error) {
	return Anchor{ID: id}, true, nil
}

func (g *linkedGraph) Neighbours(_ context.Context, id int64) ([]int64, error) {
	return slices.Sorted(slices.Values(g.edges[id])), nil
}

func (g *linkedGraph) Recall(_ context.Context, _ string, limit int, scope []int64) ([]Candidate, error) {
	var reachable []int64
	for _, id := range scope {
		reachable = append(reachable, g.edges[id]...)
	}

	slices.Sort(reachable)
	reachable = slices.Compact(reachable)

	candidates := make([]Candidate, 0, len(reachable))
	for _, id := range reachable {
		if len(candidates) == limit {
			break
		}
		candidates = append(candidates, Candidate{ID: id})
	}
	return candidates, nil
}

func (g *linkedGraph) WriteRun(context.Context, Record) WriteReceipt {
	return WriteReceipt{State: NotStored}
}

func chainGraph() *linkedGraph {
	return &linkedGraph{edges: map[int64][]int64{
		100: {200, 400},
		200: {100, 300},
		300: {200, 500},
		400: {100},
		500: {300},
	}}
}

func candidateIDs(candidates []Candidate) []int64 {
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}
	return ids
}

func TestRecallScopeIsTheSubjectFollowedByItsNeighboursAscendingWithoutRepeats(t *testing.T) {
	t.Parallel()

	graph := &linkedGraph{edges: map[int64][]int64{300: {500, 100, 300}}}

	scope, err := RecallScope(context.Background(), graph, 300)
	if err != nil {
		t.Fatalf("RecallScope: %v", err)
	}

	want := []int64{100, 300, 500}
	if !slices.Equal(scope, want) {
		t.Fatalf("RecallScope = %v, want %v ascending and without repeats: this slice becomes a recall's linkedto list, so an order that depends on where the subject happens to sort makes one graph state produce more than one request, and a self-edge that survives sends the subject twice", scope, want)
	}
}

func TestARecallScopedToTheSubjectReachesANodeTwoHopsOutAndNotOnlyDirectNeighbours(t *testing.T) {
	t.Parallel()

	graph := chainGraph()

	scope, err := RecallScope(context.Background(), graph, 100)
	if err != nil {
		t.Fatalf("RecallScope: %v", err)
	}
	got, err := graph.Recall(context.Background(), "q", CandidateLimit, scope)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if !slices.Contains(candidateIDs(got), int64(300)) {
		t.Fatalf("a recall scoped to %v returned %v, want node 300 among them: node 300 is linked to the subject's neighbour rather than to the subject, so a scope holding the subject alone cannot rank it", scope, candidateIDs(got))
	}
}

func TestARecallScopedToTheSubjectStillReachesTheSubjectsOwnDirectNeighbours(t *testing.T) {
	t.Parallel()

	graph := chainGraph()

	scope, err := RecallScope(context.Background(), graph, 100)
	if err != nil {
		t.Fatalf("RecallScope: %v", err)
	}
	got, err := graph.Recall(context.Background(), "q", CandidateLimit, scope)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if !slices.Contains(candidateIDs(got), int64(400)) {
		t.Fatalf("a recall scoped to %v returned %v, want node 400 among them: node 400 is linked only to the subject, so dropping the subject from its own scope loses every neighbour that hangs off it alone", scope, candidateIDs(got))
	}
}

func TestARecallScopedToTheSubjectLeavesOutANodeThreeHopsAway(t *testing.T) {
	t.Parallel()

	graph := chainGraph()

	scope, err := RecallScope(context.Background(), graph, 100)
	if err != nil {
		t.Fatalf("RecallScope: %v", err)
	}
	got, err := graph.Recall(context.Background(), "q", CandidateLimit, scope)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if slices.Contains(candidateIDs(got), int64(500)) {
		t.Fatalf("a recall scoped to %v returned %v, and node 500 is three hops from the subject: widening retrieval is only worth measuring against what it still leaves out, and a scope built from the neighbours' neighbours reaches this node while still passing every test that only asks whether new nodes surface", scope, candidateIDs(got))
	}
}

func TestRecallScopeReportsTheNeighbourFailureRatherThanScopingToTheSubjectAlone(t *testing.T) {
	t.Parallel()

	graph := &failingNeighbourGraph{}

	scope, err := RecallScope(context.Background(), graph, 100)
	if err == nil {
		t.Fatalf("RecallScope returned scope %v and a nil error, want the failure: a scope silently narrowed to the subject alone ranks inside one node's edges and reads exactly like a subject whose neighbours are gone", scope)
	}
}

type failingNeighbourGraph struct{ linkedGraph }

func (g *failingNeighbourGraph) Neighbours(context.Context, int64) ([]int64, error) {
	return nil, errors.New("literal: the links route refused")
}

func TestRetrieveSendsNoScopeSoThePrimaryRecallRanksTheWholeGraph(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{}

	if _, err := Retrieve(context.Background(), graph, "the input", CandidateLimit); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(graph.recallCalls) != 1 {
		t.Fatalf("Recall was called %d times, want 1", len(graph.recallCalls))
	}
	if len(graph.recallCalls[0].Scope) != 0 {
		t.Fatalf("the primary recall carried scope %v, want none: this is the ranking the corpus rates are measured on, and confining it to the subject's neighbourhood changes what every row retrieves without changing a single rate's name", graph.recallCalls[0].Scope)
	}
}
