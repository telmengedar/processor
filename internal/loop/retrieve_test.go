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

func candidateIDs(candidates []Candidate) []int64 {
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}
	return ids
}

type rankedGraph struct {
	global []Candidate

	ignoresExclusion bool
	err              error

	calls []RecallRequest
}

func (g *rankedGraph) Node(_ context.Context, id int64, _ Filter) (Anchor, bool, error) {
	return Anchor{ID: id}, true, nil
}

func (g *rankedGraph) Recall(_ context.Context, request RecallRequest) ([]Candidate, error) {
	g.calls = append(g.calls, request)
	if g.err != nil {
		return nil, g.err
	}

	rows := g.global
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

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, queries, limit, UpdateWindow{}, nil, quietLogger())
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

func TestRetrieveIssuesExactlyOneRecallCarryingEveryQueryTheWindowAndTheFullLimit(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)}
	queries := []string{"the input", "derived one", "derived two"}
	window := UpdateWindow{
		From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	}

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, queries, 6, window, nil, quietLogger())
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(graph.calls) != 1 {
		t.Fatalf("Retrieve made %d graph recalls for %d queries, want exactly 1: the graph averages the queries in one request and a turn's retrieval is one recall", len(graph.calls), len(queries))
	}
	call := graph.calls[0]
	if !slices.Equal(call.Queries, queries) {
		t.Fatalf("the recall carried queries %q, want all of %q in the order given", call.Queries, queries)
	}
	if call.Limit != 6 {
		t.Fatalf("the recall asked for %d rows, want the whole limit 6: no slot is held back for another list", call.Limit)
	}
	if call.Window != window {
		t.Fatalf("the recall carried window %+v, want the run's own %+v", call.Window, window)
	}
	if len(got) != 6 {
		t.Fatalf("Retrieve returned %d candidates, want 6", len(got))
	}
}

func TestTheRecallRetrieveIssuesNamesTheAnchorAsExcluded(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{global: ranked(1)}

	mustRetrieve(t, graph, []string{"q"}, 6)

	if len(graph.calls) != 1 {
		t.Fatalf("retrieval issued %d recalls, want 1 to inspect", len(graph.calls))
	}
	for _, call := range graph.calls {
		if !slices.Equal(call.Exclude, []int64{42}) {
			t.Fatalf("a recall excluded %v, want the anchor 42: the block renders it whole, so a slot spent on it is a slot lost", call.Exclude)
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
		global:           []Candidate{scored(42, 0.99), runRecord(901), scored(11, 0.8), scored(12, 0.7), runRecord(902), scored(13, 0.6)},
	}
	var logged strings.Builder
	logger := slog.New(slog.NewTextHandler(&logged, nil))

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, UpdateWindow{}, nil, logger)
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

	graph := &rankedGraph{global: ranked(11, 12)}
	var logged strings.Builder

	if _, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, UpdateWindow{}, nil, slog.New(slog.NewTextHandler(&logged, nil))); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if strings.Contains(logged.String(), "the graph returned rows") {
		t.Fatalf("the operator log was %q for a graph that honoured every exclusion, want silence: a warning that fires on every turn stops meaning anything on the one turn it matters", logged.String())
	}
}

func TestARankIsThePositionInTheGraphsListEvenWhenALeakedRowStoodAheadOfIt(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		ignoresExclusion: true,
		global:           []Candidate{scored(42, 0.99), runRecord(901), scored(12, 0.7)},
	}

	got := mustRetrieveCandidates(t, graph, []string{"q"}, 6)

	want := map[int64][]Source{12: {{Rank: 3}}}
	for _, candidate := range got {
		if !slices.Equal(candidate.Sources, want[candidate.ID]) {
			t.Fatalf("node %d carries sources %+v, want %+v: the rank is the position in the list the graph returned, and a net-filtered count would hide that something stood ahead", candidate.ID, candidate.Sources, want[candidate.ID])
		}
	}
	if len(got) != 1 {
		t.Fatalf("retrieval returned %v, want the one kept row", candidateIDs(got))
	}
}

func TestCandidatesAreOrderedBySimilarityWhateverOrderTheGraphListedThem(t *testing.T) {
	t.Parallel()

	graph := &rankedGraph{
		global: []Candidate{scored(2, 0.65), scored(1, 0.70), scored(4, 0.63), scored(3, 0.64), scored(6, 0.61), scored(5, 0.62)},
	}

	got := mustRetrieve(t, graph, []string{"q"}, 6)

	if want := []int64{1, 2, 3, 4, 5, 6}; !slices.Equal(got, want) {
		t.Fatalf("retrieval returned %v, want %v by similarity descending: one number orders the rows, and the byte budget is spent in this order", got, want)
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

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, nil, 6, UpdateWindow{}, nil, quietLogger())
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

	got, err := Retrieve(context.Background(), graph, Anchor{ID: 42}, []string{"q"}, 6, UpdateWindow{}, nil, quietLogger())

	if !errors.Is(err, failure) {
		t.Fatalf("Retrieve returned error %v, want the graph's own %v", err, failure)
	}
	if got != nil {
		t.Fatalf("Retrieve returned %v beside a failure, want none", candidateIDs(got))
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

func TestASourceIsWrittenWithTheKeyTheStepTraceReads(t *testing.T) {
	t.Parallel()

	plain, err := json.Marshal(Source{Rank: 3})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if string(plain) != `{"rank":3}` {
		t.Fatalf("a source is written as %s, want {\"rank\":3}: scripts/step_trace.py reads the rank of each source", plain)
	}
}

func TestARecordedSourceFromAScopedRecallStillDecodes(t *testing.T) {
	t.Parallel()

	var archived []Source
	if err := json.Unmarshal([]byte(`[{"rank":3},{"rank":2,"scoped":true}]`), &archived); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if want := []Source{{Rank: 3}, {Rank: 2}}; !slices.Equal(archived, want) {
		t.Fatalf("an archived source list decoded as %+v, want %+v: records filed while a scoped recall existed must stay readable", archived, want)
	}
}
