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

func TestRecallScopeIsTheSubjectAndItsNeighboursAscendingWithoutRepeats(t *testing.T) {
	t.Parallel()

	graph := &linkedGraph{edges: map[int64][]int64{300: {500, 100, 100}}}

	scope, err := RecallScope(context.Background(), graph, 300)
	if err != nil {
		t.Fatalf("RecallScope: %v", err)
	}

	want := []int64{100, 300, 500}
	if !slices.Equal(scope, want) {
		t.Fatalf("RecallScope = %v, want %v: the subject sorts between its own neighbours and one neighbour arrives twice, so dropping the subject, dropping the neighbours, dropping the sort or dropping the repeat-collapse each produce a different list from this one", scope, want)
	}
}

func TestRecallScopeCarriesTheSubjectOnceWhenASelfEdgeMakesItItsOwnNeighbour(t *testing.T) {
	t.Parallel()

	graph := &linkedGraph{edges: map[int64][]int64{300: {300}}}

	scope, err := RecallScope(context.Background(), graph, 300)
	if err != nil {
		t.Fatalf("RecallScope: %v", err)
	}

	want := []int64{300}
	if !slices.Equal(scope, want) {
		t.Fatalf("RecallScope = %v, want %v: a node linked to itself is returned as its own neighbour, so the subject reaches the scope twice and only the repeat-collapse stops it being sent to the graph twice", scope, want)
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

type fusionGraph struct {
	lists      map[string][]Candidate
	scoped     []Candidate
	neighbours []int64
	calls      []recallCall
}

func (g *fusionGraph) Node(_ context.Context, id int64) (Anchor, bool, error) {
	return Anchor{ID: id}, true, nil
}

func (g *fusionGraph) Neighbours(context.Context, int64) ([]int64, error) {
	return g.neighbours, nil
}

func (g *fusionGraph) Recall(_ context.Context, query string, limit int, scope []int64) ([]Candidate, error) {
	g.calls = append(g.calls, recallCall{Query: query, Limit: limit, Scope: scope})
	if len(scope) > 0 {
		return g.scoped, nil
	}
	return g.lists[query], nil
}

func (g *fusionGraph) WriteRun(context.Context, Record) WriteReceipt {
	return WriteReceipt{State: NotStored}
}

func ranked(ids ...int64) []Candidate {
	candidates := make([]Candidate, len(ids))
	for i, id := range ids {
		candidates[i] = Candidate{ID: id}
	}
	return candidates
}

func mustRetrieve(t *testing.T, graph GraphPort, queries []string, limit, reserve int) []int64 {
	t.Helper()

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, queries, limit, reserve)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	return candidateIDs(got)
}

func similarityGraph() *fusionGraph {
	return &fusionGraph{lists: map[string][]Candidate{
		"first": {
			{ID: 700, Similarity: 0.90},
			{ID: 300, Similarity: 0.10},
			{ID: 900, Similarity: 0.80},
		},
		"second": {
			{ID: 500, Similarity: 0.70},
			{ID: 300, Similarity: 0.10},
			{ID: 100, Similarity: 0.60},
		},
	}}
}

func TestFusionRanksTheNodeBothQueriesReturnedAboveEveryNodeOnlyOneReturnedDespiteItsLowestSimilarity(t *testing.T) {
	t.Parallel()

	got := mustRetrieve(t, similarityGraph(), []string{"first", "second"}, 5, 0)

	want := []int64{300, 700, 500, 900, 100}
	if !slices.Equal(got, want) {
		t.Fatalf("fused order = %v, want %v: node 300 carries the lowest similarity of the five and sits second in both lists, so sorting the union by similarity puts it last and taking either list alone puts it second, while summing its two reciprocal ranks is the only reading that puts it first", got, want)
	}
}

func reserveGraph() *fusionGraph {
	return &fusionGraph{
		lists:  map[string][]Candidate{"input": ranked(810, 220, 640, 130, 970, 350, 480, 760, 590, 20)},
		scoped: ranked(590, 20, 990),
	}
}

func TestTheScopeReserveAdmitsAScopedNodeTheFusedOrderPlacesBeyondTheCap(t *testing.T) {
	t.Parallel()

	got := mustRetrieve(t, reserveGraph(), []string{"input"}, 6, 2)

	if !slices.Contains(got, int64(590)) {
		t.Fatalf("retrieval returned %v, want node 590 among them: it stands ninth in the fused order and so falls outside a cap of six, and only slots held back from that cap before it is filled can carry it", got)
	}
}

func TestTheScopeReserveLeavesTheFusedOrderAheadOfItAtTheRanksItAlreadyHeld(t *testing.T) {
	t.Parallel()

	got := mustRetrieve(t, reserveGraph(), []string{"input"}, 6, 2)

	want := []int64{810, 220, 640, 130}
	if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Fatalf("retrieval returned %v, want it to open with %v: interleaving the scoped list into the fused one reaches the same nodes and demotes every one of them, which is a widening that costs the rows already answered correctly", got, want)
	}
}

func TestRetrievalStillLeavesOutTheFusedNodesTheReserveDisplaced(t *testing.T) {
	t.Parallel()

	got := mustRetrieve(t, reserveGraph(), []string{"input"}, 6, 2)

	if len(got) != 6 {
		t.Fatalf("retrieval returned %d candidates for a cap of 6: %v", len(got), got)
	}
	for _, displaced := range []int64{970, 350} {
		if slices.Contains(got, displaced) {
			t.Fatalf("retrieval returned %v, and node %d stands fifth or sixth in the fused order: the reserve is slots taken out of the cap rather than added to it, so a result that keeps the whole fused prefix and the scoped list as well is a wider retrieval than the one that was measured", got, displaced)
		}
	}
}

func TestTheScopeReserveStopsAtItsOwnSizeRatherThanFillingWhatTheFusedListLeftEmpty(t *testing.T) {
	t.Parallel()

	graph := &fusionGraph{
		lists:  map[string][]Candidate{"input": ranked(810, 220, 640)},
		scoped: ranked(990, 880, 770, 660),
	}

	got := mustRetrieve(t, graph, []string{"input"}, 6, 2)

	want := []int64{810, 220, 640, 990, 880}
	if !slices.Equal(got, want) {
		t.Fatalf("retrieval returned %v, want %v: four scoped nodes are available and the cap has room for all of them, so a reserve that grows to fill the room is a scoped ranking with a fused prefix rather than the allocation that was measured", got, want)
	}
}

func TestRetrieveIssuesOneWholeGraphRecallPerQueryAndOneMoreRankedInsideTheAnchorsScope(t *testing.T) {
	t.Parallel()

	graph := &fusionGraph{
		lists:      map[string][]Candidate{"the input": ranked(810), "derived one": ranked(220), "derived two": ranked(640)},
		neighbours: []int64{7},
	}

	mustRetrieve(t, graph, []string{"the input", "derived one", "derived two"}, 6, 2)

	if len(graph.calls) != 4 {
		t.Fatalf("Recall was called %d times for three queries, want 4: one whole-graph ranking per query and one ranked inside the anchor's scope", len(graph.calls))
	}
	for i, want := range []string{"the input", "derived one", "derived two"} {
		if graph.calls[i].Query != want {
			t.Fatalf("recall %d queried %q, want %q verbatim", i, graph.calls[i].Query, want)
		}
		if len(graph.calls[i].Scope) != 0 {
			t.Fatalf("recall %d carried scope %v, want none: a derived query confined to the anchor's neighbourhood cannot reach the crowding that a whole-graph ranking of it is there to escape", i, graph.calls[i].Scope)
		}
	}
	if !slices.Equal(graph.calls[3].Scope, []int64{7, 42}) {
		t.Fatalf("the scoped recall carried scope %v, want %v: the anchor together with its neighbours", graph.calls[3].Scope, []int64{7, 42})
	}
}

func TestTheScopedRecallCarriesTheRawInputRatherThanADerivedQuery(t *testing.T) {
	t.Parallel()

	graph := &fusionGraph{lists: map[string][]Candidate{"the input": ranked(810), "derived one": ranked(220)}}

	mustRetrieve(t, graph, []string{"the input", "derived one"}, 6, 2)

	scoped := graph.calls[len(graph.calls)-1]
	if scoped.Query != "the input" {
		t.Fatalf("the scoped recall queried %q, want the raw input %q: the reserve exists to compensate for a request whose own words rank badly against the whole graph, and ranking a derived question inside the neighbourhood instead answers a question the caller did not ask", scoped.Query, "the input")
	}
}

func TestRetrieveReportsTheNeighbourFailureRatherThanRankingTheWholeGraphTwice(t *testing.T) {
	t.Parallel()

	graph := &failingNeighbourGraph{}

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"the input"}, 6, 2)
	if err == nil {
		t.Fatalf("Retrieve returned %v and a nil error, want the failure: a scope the links route could not supply leaves the reserved slots holding a second copy of the whole-graph ranking, which reads on every rate exactly like a neighbourhood that had nothing to add", candidateIDs(got))
	}
}

func TestRetrieveReadsTheGraphNotAtAllWhenThereIsNoQueryToIssue(t *testing.T) {
	t.Parallel()

	graph := &fusionGraph{lists: map[string][]Candidate{"": ranked(810)}, scoped: ranked(990)}

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, nil, 6, 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Retrieve returned %v for an empty query set, want none", candidateIDs(got))
	}
	if len(graph.calls) != 0 {
		t.Fatalf("Retrieve made %d graph reads for an empty query set, want none: the scoped recall has no query to carry, and issuing it with the empty string ranks the whole neighbourhood by nothing", len(graph.calls))
	}
}

func TestAScopedNodeTheFusedPrefixAlreadyHoldsDoesNotSpendAReservedSlot(t *testing.T) {
	t.Parallel()

	graph := &fusionGraph{
		lists:  map[string][]Candidate{"input": ranked(810, 220, 640, 130, 970, 350, 480, 760, 590, 20)},
		scoped: ranked(810, 590, 20),
	}

	got := mustRetrieve(t, graph, []string{"input"}, 6, 2)

	want := []int64{810, 220, 640, 130, 590, 20}
	if !slices.Equal(got, want) {
		t.Fatalf("retrieval returned %v, want %v: node 810 heads the fused order and the scope returns it as well, so charging that repeat against the reserve spends one of two slots on a node the result already carries and leaves node 20 outside the cap", got, want)
	}
}

func dampingGraph() *fusionGraph {
	return &fusionGraph{lists: map[string][]Candidate{
		"first": {
			{ID: 700, Similarity: 0.99},
			{ID: 11, Similarity: 0.91}, {ID: 12, Similarity: 0.90}, {ID: 13, Similarity: 0.89},
			{ID: 14, Similarity: 0.88}, {ID: 15, Similarity: 0.87}, {ID: 16, Similarity: 0.86},
			{ID: 300, Similarity: 0.01},
		},
		"second": {
			{ID: 21, Similarity: 0.85}, {ID: 22, Similarity: 0.84}, {ID: 23, Similarity: 0.83},
			{ID: 24, Similarity: 0.82}, {ID: 25, Similarity: 0.81}, {ID: 26, Similarity: 0.80},
			{ID: 27, Similarity: 0.79},
			{ID: 300, Similarity: 0.01},
		},
	}}
}

func TestFusionPutsANodeTwoQueriesReturnedEighthAboveANodeOneQueryReturnedFirst(t *testing.T) {
	t.Parallel()

	got := mustRetrieve(t, dampingGraph(), []string{"first", "second"}, 20, 0)

	twice := slices.Index(got, int64(300))
	once := slices.Index(got, int64(700))
	if twice < 0 || once < 0 {
		t.Fatalf("fused order = %v, want both node 300 and node 700 among them", got)
	}
	if twice > once {
		t.Fatalf("fused order = %v, and node 700 stands above node 300: node 300 was returned eighth by both queries and node 700 first by one, and how far a rank has to fall before two of them outweigh one top hit is what the damping term decides", got)
	}
}

func deepDampingGraph() *fusionGraph {
	return &fusionGraph{lists: map[string][]Candidate{
		"first": {
			{ID: 700, Similarity: 0.99},
			{ID: 101, Similarity: 0.90},
			{ID: 102, Similarity: 0.89},
			{ID: 103, Similarity: 0.88},
			{ID: 104, Similarity: 0.87},
			{ID: 105, Similarity: 0.86},
			{ID: 106, Similarity: 0.85},
			{ID: 107, Similarity: 0.84},
			{ID: 108, Similarity: 0.83},
			{ID: 109, Similarity: 0.82},
			{ID: 110, Similarity: 0.81},
			{ID: 111, Similarity: 0.80},
			{ID: 400, Similarity: 0.01},
		},
		"second": {
			{ID: 201, Similarity: 0.89},
			{ID: 202, Similarity: 0.88},
			{ID: 203, Similarity: 0.87},
			{ID: 204, Similarity: 0.86},
			{ID: 205, Similarity: 0.85},
			{ID: 206, Similarity: 0.84},
			{ID: 207, Similarity: 0.83},
			{ID: 208, Similarity: 0.82},
			{ID: 209, Similarity: 0.81},
			{ID: 210, Similarity: 0.80},
			{ID: 211, Similarity: 0.79},
			{ID: 212, Similarity: 0.78},
			{ID: 400, Similarity: 0.01},
		},
	}}
}

func TestFusionKeepsANodeOneQueryReturnedFirstAboveANodeTwoQueriesReturnedThirteenth(t *testing.T) {
	t.Parallel()

	got := mustRetrieve(t, deepDampingGraph(), []string{"first", "second"}, 30, 0)

	once := slices.Index(got, int64(700))
	twice := slices.Index(got, int64(400))
	if once < 0 || twice < 0 {
		t.Fatalf("fused order = %v, want both node 700 and node 400 among them", got)
	}
	if once > twice {
		t.Fatalf("fused order = %v, and node 400 stands above node 700: node 400 was returned thirteenth by both queries and node 700 first by one, and this is the far side of the same threshold its sibling guard bounds from below, so the two together fix how far the damping may travel in either direction", got)
	}
}
