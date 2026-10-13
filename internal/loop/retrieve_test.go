package loop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
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

func (g *linkedGraph) Recall(_ context.Context, request RecallRequest) ([]Candidate, error) {
	var reachable []int64
	for _, id := range request.Scope {
		reachable = append(reachable, g.edges[id]...)
	}

	slices.Sort(reachable)
	reachable = slices.Compact(reachable)

	candidates := make([]Candidate, 0, len(reachable))
	for _, id := range reachable {
		if len(candidates) == request.Limit {
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
	got, err := graph.Recall(context.Background(), RecallRequest{Queries: []string{"q"}, Limit: 100, Scope: scope})
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
	got, err := graph.Recall(context.Background(), RecallRequest{Queries: []string{"q"}, Limit: 100, Scope: scope})
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
	got, err := graph.Recall(context.Background(), RecallRequest{Queries: []string{"q"}, Limit: 100, Scope: scope})
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

type rankedGraph struct {
	global     []Candidate
	scoped     []Candidate
	neighbours []int64

	ignoresExclusion bool
	err              error

	calls []RecallRequest
}

func (g *rankedGraph) Node(_ context.Context, id int64) (Anchor, bool, error) {
	return Anchor{ID: id}, true, nil
}

func (g *rankedGraph) Neighbours(context.Context, int64) ([]int64, error) {
	return g.neighbours, nil
}

func (g *rankedGraph) Recall(_ context.Context, request RecallRequest) ([]Candidate, error) {
	g.calls = append(g.calls, request)
	if g.err != nil {
		return nil, g.err
	}

	rows := g.global
	if len(request.Scope) > 0 {
		rows = g.scoped
	}
	if !g.ignoresExclusion {
		rows = withoutExcluded(rows, request.Exclude)
	}
	return slices.Clone(rows[:min(request.Limit, len(rows))]), nil
}

func (g *rankedGraph) WriteRun(context.Context, Record) WriteReceipt {
	return WriteReceipt{State: NotStored}
}

func scored(id int64, similarity float64) Candidate {
	return Candidate{ID: id, Similarity: similarity}
}

func runRecord(id int64) Candidate {
	return Candidate{ID: id, Similarity: 0.9, SelfProduced: true}
}

func ranked(ids ...int64) []Candidate {
	candidates := make([]Candidate, len(ids))
	for i, id := range ids {
		candidates[i] = Candidate{ID: id, Similarity: 1 - float64(i)/100}
	}
	return candidates
}

func mustRetrieveCandidates(t *testing.T, graph GraphPort, queries []string, limit int) []Candidate {
	t.Helper()

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, queries, limit, RecallScopeReserve, UpdateWindow{}, quietLogger())
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	return got
}

func mustRetrieve(t *testing.T, graph GraphPort, queries []string, limit int) []int64 {
	t.Helper()

	return candidateIDs(mustRetrieveCandidates(t, graph, queries, limit))
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func sortedIDs(candidates []Candidate) []int64 {
	ids := candidateIDs(candidates)
	slices.Sort(ids)
	return ids
}

func TestRetrieveAsksTheGraphOnceGloballyForEveryQueryTogetherAtExactlyTheLimitItWasGiven(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)}
	queries := []string{"the input", "derived one", "derived two"}

	got := mustRetrieve(t, graph, queries, 6)

	var global []RecallRequest
	for _, call := range graph.calls {
		if len(call.Scope) == 0 {
			global = append(global, call)
		}
	}
	if len(global) != 1 {
		t.Fatalf("Retrieve made %d global recalls for %d queries, want 1: the graph averages the queries in one request", len(global), len(queries))
	}
	if !slices.Equal(global[0].Queries, queries) {
		t.Fatalf("the global recall carried queries %q, want all of %q in the order given", global[0].Queries, queries)
	}
	if global[0].Limit != 6 {
		t.Fatalf("the global recall asked for %d rows, want exactly the limit 6", global[0].Limit)
	}
	if len(got) != 6 {
		t.Fatalf("Retrieve returned %d candidates, want 6", len(got))
	}
}

func TestTheScopedRecallCarriesEveryQueryAndTheScopeButNotTheWindow(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(1, 2), scoped: ranked(3), neighbours: []int64{9, 7}}
	queries := []string{"the input", "derived one"}
	window := UpdateWindow{
		From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	}

	if _, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, queries, 6, RecallScopeReserve, window, quietLogger()); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(graph.calls) != 2 {
		t.Fatalf("Retrieve made %d recalls, want the global one and the scoped one", len(graph.calls))
	}
	global, scoped := graph.calls[0], graph.calls[1]
	if global.Window != window {
		t.Fatalf("the global recall carried window %+v, want the run's own %+v", global.Window, window)
	}
	if !slices.Equal(scoped.Queries, queries) {
		t.Fatalf("the scoped recall carried queries %q, want all of %q: a scoped ranking on one phrasing scores rows on a different metric than the global list it is merged with", scoped.Queries, queries)
	}
	if want := []int64{7, 9, 42}; !slices.Equal(scoped.Scope, want) {
		t.Fatalf("the scoped recall carried scope %v, want the subject and its neighbours ascending, %v", scoped.Scope, want)
	}
	if !scoped.Window.IsZero() {
		t.Fatalf("the scoped recall carried window %+v, want none: the run's date constraint was never asked of the subject's neighbourhood", scoped.Window)
	}
}

func TestEveryRecallRetrieveIssuesNamesTheAnchorAsExcluded(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(1), scoped: ranked(2)}

	mustRetrieve(t, graph, []string{"q"}, 6)

	for _, call := range graph.calls {
		if !slices.Equal(call.Exclude, []int64{42}) {
			t.Fatalf("a recall with scope %v excluded %v, want the anchor 42: the block renders it whole, so a slot spent on it is a slot lost", call.Scope, call.Exclude)
		}
	}
}

func TestAnApertureWhoseUnfilteredTopIsAllRunRecordsStillDeliversTheLimit(t *testing.T) {
	t.Parallel()

	global := []Candidate{scored(42, 0.99), runRecord(901), runRecord(902), runRecord(903), runRecord(904), runRecord(905)}
	global = append(global, ranked(11, 12, 13, 14, 15, 16)...)
	graph := &rankedGraph{global: global}

	got := mustRetrieveCandidates(t, graph, []string{"ill matched input"}, 6)

	if want := []int64{11, 12, 13, 14, 15, 16}; !slices.Equal(sortedIDs(got), want) {
		t.Fatalf("retrieval returned %v, want the six usable rows %v: when the graph ranks the anchor and run records first, only a recall that asks for their exclusion before the count still gets six rows", sortedIDs(got), want)
	}
}

func TestARowTheGraphWasToldToExcludeIsDroppedAndReported(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		ignoresExclusion: true,
		global:           []Candidate{scored(42, 0.99), runRecord(901), scored(11, 0.8), scored(12, 0.7)},
		scoped:           []Candidate{runRecord(902), scored(13, 0.6)},
	}
	var logged strings.Builder
	logger := slog.New(slog.NewTextHandler(&logged, nil))

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, RecallScopeReserve, UpdateWindow{}, logger)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if want := []int64{11, 12, 13}; !slices.Equal(sortedIDs(got), want) {
		t.Fatalf("retrieval returned %v, want %v: a graph that ignored the exclusion still must not hand the loop its anchor or a run record", sortedIDs(got), want)
	}
	if !strings.Contains(logged.String(), `msg="the graph returned rows the recall asked it to exclude"`) || !strings.Contains(logged.String(), "rows=3") {
		t.Fatalf("the operator log was %q, want the leak named with its count of 3: a backend that does not honour the exclusion is otherwise indistinguishable from one that does", logged.String())
	}
}

func TestARecallThatLeaksNothingLogsNoLeak(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(11, 12), scoped: ranked(13)}
	var logged strings.Builder

	if _, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, RecallScopeReserve, UpdateWindow{}, slog.New(slog.NewTextHandler(&logged, nil))); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if strings.Contains(logged.String(), "the graph returned rows") {
		t.Fatalf("the operator log was %q for a graph that honoured every exclusion, want silence: a warning that fires on every turn stops meaning anything on the one turn it matters", logged.String())
	}
}

func TestTheScopeReserveAddsAtMostTwoRowsTheGlobalRecallLackedAndNeverDisplacesItsTopFour(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		global: ranked(1, 2, 3, 4, 5, 6),
		scoped: ranked(21, 22, 23, 24),
	}

	got := mustRetrieveCandidates(t, graph, []string{"q"}, 6)

	if want := []int64{1, 2, 3, 4, 21, 22}; !slices.Equal(sortedIDs(got), want) {
		t.Fatalf("retrieval returned %v, want %v: the global top 4 are never displaced, and the neighbourhood takes exactly the 2 reserved slots when it has rows the global list lacks", sortedIDs(got), want)
	}
}

func TestTheReservedSlotsGoToScopedRowsAbsentFromTheWholeGlobalTop(t *testing.T) {
	t.Parallel()

	for name, scoped := range map[string][]Candidate{
		"a scoped row inside the first four global rows": ranked(4, 21, 22),
		"a scoped row at global rank five":               ranked(5, 21, 22),
		"a scoped row at global rank six":                ranked(6, 21, 22),
	} {
		graph := &rankedGraph{global: ranked(1, 2, 3, 4, 5, 6), scoped: scoped}

		got := mustRetrieveCandidates(t, graph, []string{"q"}, 6)

		if want := []int64{1, 2, 3, 4, 21, 22}; !slices.Equal(sortedIDs(got), want) {
			t.Fatalf("%s: retrieval returned %v, want %v: a scoped row the global top six already holds adds nothing, so it must not spend a reserved slot", name, sortedIDs(got), want)
		}
	}
}

func TestARankIsThePositionInTheGraphsListEvenWhenALeakedRowStoodAheadOfIt(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		ignoresExclusion: true,
		global:           []Candidate{scored(42, 0.99), scored(11, 0.8)},
		scoped:           []Candidate{runRecord(901), scored(12, 0.7)},
	}

	got := mustRetrieveCandidates(t, graph, []string{"q"}, 6)

	want := map[int64][]Source{11: {{Rank: 2}}, 12: {{Rank: 2, Scoped: true}}}
	for _, candidate := range got {
		if !slices.Equal(candidate.Sources, want[candidate.ID]) {
			t.Fatalf("node %d carries sources %+v, want %+v: the rank is the position in the list the graph returned, and a net-filtered count would hide that something stood ahead", candidate.ID, candidate.Sources, want[candidate.ID])
		}
	}
	if len(got) != 2 {
		t.Fatalf("retrieval returned %v, want the two kept rows", candidateIDs(got))
	}
}

func TestTheScopeReserveStopsAtItsOwnSizeWhenTheGlobalListIsShort(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		global: ranked(1, 2),
		scoped: ranked(21, 22, 23, 24, 25, 26),
	}

	got := mustRetrieveCandidates(t, graph, []string{"q"}, 6)

	if want := []int64{1, 2, 21, 22}; !slices.Equal(sortedIDs(got), want) {
		t.Fatalf("retrieval returned %v, want %v: the neighbourhood is held to its reserve, so a short global list is not topped up from the scope", sortedIDs(got), want)
	}
}

func TestAScopedRecallThatAddsNothingLeavesTheGlobalTopUnchanged(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		global: ranked(1, 2, 3, 4, 5, 6),
		scoped: ranked(2, 5),
	}

	got := mustRetrieve(t, graph, []string{"q"}, 6)

	if want := []int64{1, 2, 3, 4, 5, 6}; !slices.Equal(slices.Sorted(slices.Values(got)), want) {
		t.Fatalf("retrieval returned %v, want the global top six %v: a neighbourhood that holds nothing the global list lacks must not cost the aperture a slot", got, want)
	}
}

func TestCandidatesFromBothRecallsAreOrderedBySimilarity(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		global: []Candidate{scored(1, 0.70), scored(2, 0.65), scored(3, 0.64), scored(4, 0.63), scored(5, 0.62), scored(6, 0.61)},
		scoped: []Candidate{scored(20, 0.69), scored(21, 0.66)},
	}

	got := mustRetrieve(t, graph, []string{"q"}, 6)

	if want := []int64{1, 20, 21, 2, 3, 4}; !slices.Equal(got, want) {
		t.Fatalf("retrieval returned %v, want %v by similarity descending: both lists score the same queries, so one number orders them, and the byte budget is spent in this order", got, want)
	}
}

func TestEqualSimilaritiesKeepAStableOrderByID(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: []Candidate{scored(5, 0.7), scored(3, 0.7), scored(4, 0.7)}}

	got := mustRetrieve(t, graph, []string{"q"}, 6)

	if want := []int64{3, 4, 5}; !slices.Equal(got, want) {
		t.Fatalf("retrieval returned %v, want %v: equal scores must not leave the order to the graph's tie-break", got, want)
	}
}

func TestARowBothRecallsReturnedNamesBothWithTheirRanks(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		global: ranked(1, 2, 3),
		scoped: ranked(9, 3, 8),
	}

	got := mustRetrieveCandidates(t, graph, []string{"q"}, 6)

	want := map[int64][]Source{
		1: {{Rank: 1}},
		3: {{Rank: 3}, {Rank: 2, Scoped: true}},
		9: {{Rank: 1, Scoped: true}},
	}
	for _, candidate := range got {
		if w, ok := want[candidate.ID]; ok && !slices.Equal(candidate.Sources, w) {
			t.Fatalf("node %d carries sources %+v, want %+v: a row both recalls returned is evidence from two arms, and a record that keeps only one cannot say which reached it", candidate.ID, candidate.Sources, w)
		}
	}
}

func TestRetrieveReturnsTheGraphsSimilarityUnchanged(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: []Candidate{scored(300, 0.71), scored(100, 0.70)}}

	got := mustRetrieveCandidates(t, graph, []string{"a", "b"}, 6)

	if got[0].ID != 300 || got[0].Similarity != 0.71 || got[1].ID != 100 || got[1].Similarity != 0.70 {
		t.Fatalf("Retrieve returned %+v, want the graph's rows with the mean similarity it reported", got)
	}
}

func TestRetrieveReadsTheGraphNotAtAllWhenThereIsNoQueryToIssue(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(810)}

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, nil, 6, RecallScopeReserve, UpdateWindow{}, quietLogger())
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Retrieve returned %v for an empty query set, want none", candidateIDs(got))
	}
	if len(graph.calls) != 0 {
		t.Fatalf("Retrieve made %d graph reads for an empty query set, want none", len(graph.calls))
	}
}

func TestRetrieveReportsAGraphFailureRatherThanAnEmptyAperture(t *testing.T) {
	t.Parallel()

	failure := errors.New("literal: 500 from graph")
	graph := &rankedGraph{err: failure}

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, RecallScopeReserve, UpdateWindow{}, quietLogger())

	if !errors.Is(err, failure) {
		t.Fatalf("Retrieve returned error %v, want the graph's own %v", err, failure)
	}
	if got != nil {
		t.Fatalf("Retrieve returned %v beside a failure, want none", candidateIDs(got))
	}
}

func TestRetrieveReportsTheNeighbourFailureRatherThanRankingTheWholeGraphTwice(t *testing.T) {
	t.Parallel()

	graph := &failingNeighbourGraph{}

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, RecallScopeReserve, UpdateWindow{}, quietLogger())

	if err == nil {
		t.Fatalf("Retrieve returned %v and a nil error, want the links failure", candidateIDs(got))
	}
}

func TestACandidateCarriesItsSubstanceThroughRetrieveUnchanged(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: []Candidate{{ID: 300, Similarity: 0.9, Substance: "the condensed form"}, {ID: 400, Similarity: 0.8}}}

	got := mustRetrieveCandidates(t, graph, []string{"only"}, 6)

	substance := make(map[int64]string, len(got))
	for _, candidate := range got {
		substance[candidate.ID] = candidate.Substance
	}

	if substance[300] != "the condensed form" {
		t.Fatalf("node 300 carries Substance %q, want %q to survive Retrieve unchanged", substance[300], "the condensed form")
	}
	if substance[400] != "" {
		t.Fatalf("node 400 carries Substance %q, want the zero value", substance[400])
	}
}

func TestASourceIsWrittenWithTheKeysTheStepTraceReadsAndOmitsScopedWhenFalse(t *testing.T) {
	t.Parallel()

	plain, err := json.Marshal(Source{Rank: 3})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	scoped, err := json.Marshal(Source{Rank: 2, Scoped: true})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if string(plain) != `{"rank":3}` || string(scoped) != `{"rank":2,"scoped":true}` {
		t.Fatalf("sources are written as %s and %s, want {\"rank\":3} and {\"rank\":2,\"scoped\":true}: scripts/step_trace.py tells this format from the earlier one by the absence of a query key", plain, scoped)
	}
}
