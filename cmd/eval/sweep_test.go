package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
)

const requiredNodeBody = "the required node body"

const requiredNodeBodyHash = "6e353b77ce66521a105fcb7649b7fc9b32716025fa338b48a378ae4341eb04d6"

type recallCall struct {
	Query string
	Limit int
}

type fakeGraph struct {
	t *testing.T

	writeAllowed bool

	nodes      map[int64]loop.Anchor
	candidates []loop.Candidate

	nodeErr   error
	recallErr error

	recallCalls []recallCall
	nodeCalls   []int64
}

func newFakeGraph(t *testing.T) *fakeGraph {
	t.Helper()

	return &fakeGraph{
		t:     t,
		nodes: map[int64]loop.Anchor{100: {ID: 100, Type: "documentation", Name: "Subject", Content: "the subject body"}},
	}
}

func (f *fakeGraph) Node(_ context.Context, id int64) (loop.Anchor, bool, error) {
	f.nodeCalls = append(f.nodeCalls, id)
	if f.nodeErr != nil {
		return loop.Anchor{}, false, f.nodeErr
	}
	anchor, found := f.nodes[id]
	return anchor, found, nil
}

func (f *fakeGraph) Recall(_ context.Context, query string, limit int) ([]loop.Candidate, error) {
	f.recallCalls = append(f.recallCalls, recallCall{Query: query, Limit: limit})
	if f.recallErr != nil {
		return nil, f.recallErr
	}
	return f.candidates[:min(limit, len(f.candidates))], nil
}

func (f *fakeGraph) WriteRun(context.Context, loop.Record) loop.WriteReceipt {
	if !f.writeAllowed {
		f.t.Fatal("the sweep called WriteRun; the instrument must not mutate the substrate it measures")
	}
	return loop.WriteReceipt{State: loop.Stored, NodeID: 1}
}

type stubJudge struct{}

func (stubJudge) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	return loop.JudgeResult{Answer: "an answer", Reason: loop.Answered, RawReason: "stop"}, nil
}

func labelledRow(id string, required ...eval.Required) eval.Row {
	return eval.Row{ID: id, Input: "what did the split change", Subject: 100, Stratum: eval.StratumLabelled, Required: required}
}

func corpusOf(rows ...eval.Row) eval.Corpus {
	return eval.Corpus{Hash: "a-corpus-hash", Rows: rows}
}

func sweptAt() time.Time {
	return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
}

func mustSweep(t *testing.T, graph loop.GraphPort, corpus eval.Corpus) eval.Result {
	t.Helper()

	result, err := sweep(context.Background(), graph, corpus, sweptAt())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	return result
}

func summaryOf(t *testing.T, result eval.Result) string {
	t.Helper()

	var machine, human bytes.Buffer
	if err := eval.Render(result, "corpus.json", &machine, &human); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return human.String()
}

func TestSweepDispositionsEqualTheRecordDispositionsForTheSameAnchorAndCandidates(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.writeAllowed = true
	graph.candidates = []loop.Candidate{
		{ID: 200, Type: "documentation", Name: "Alpha", Similarity: 0.81, Content: strings.Repeat("a", loop.AssemblyByteBudget/2)},
		{ID: 201, Type: "documentation", Name: "Bravo", Similarity: 0.79, Content: strings.Repeat("b", loop.AssemblyByteBudget/2)},
		{ID: 202, Type: "documentation", Name: "Charlie", Similarity: 0.77, Content: strings.Repeat("c", loop.AssemblyByteBudget/2)},
	}

	row := labelledRow("r01", eval.Required{Node: 200, Hash: "h", Why: "w"})

	turn := loop.NewTurn(graph, stubJudge{}, "system text", "model-id", nil)
	record, _, err := turn.Run(context.Background(), row.Input, row.Subject)
	if err != nil {
		t.Fatalf("Turn.Run: %v", err)
	}

	dispositions, found, err := rowDispositions(context.Background(), graph, row)
	if err != nil {
		t.Fatalf("rowDispositions: %v", err)
	}
	if !found {
		t.Fatal("rowDispositions reported the subject as absent, want it found")
	}

	if !reflect.DeepEqual(dispositions, record.Candidates) {
		t.Fatalf("the sweep produced dispositions a real run would not:\nsweep  = %+v\nrecord = %+v", dispositions, record.Candidates)
	}
	if len(record.Candidates) != 3 {
		t.Fatalf("the fixture produced %d dispositions, want 3 so the comparison has something to disagree about", len(record.Candidates))
	}
	admitted := 0
	for _, d := range record.Candidates {
		if d.Included {
			admitted++
		}
	}
	if admitted != 2 {
		t.Fatalf("the fixture admitted %d of 3 candidates, want 2 so a different budget would change the answer", admitted)
	}
}

func TestSweepRecallsWithTheRawInputVerbatimAndTheCandidateLimitTheLoopShips(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	row := labelledRow("r01", eval.Required{Node: 200, Hash: requiredNodeBodyHash, Why: "w"})
	mustSweep(t, graph, corpusOf(row))

	if len(graph.recallCalls) != 1 {
		t.Fatalf("the sweep ran %d recalls for one row, want exactly 1", len(graph.recallCalls))
	}
	if graph.recallCalls[0].Query != row.Input {
		t.Fatalf("recall query = %q, want the row input verbatim %q", graph.recallCalls[0].Query, row.Input)
	}
	if graph.recallCalls[0].Limit != loop.CandidateLimit {
		t.Fatalf("recall limit = %d, want the candidate limit the loop ships %d", graph.recallCalls[0].Limit, loop.CandidateLimit)
	}
}

func TestSweepFlagsAStaleRequiredNodeAndStillScoresItsRow(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	row := labelledRow("r01", eval.Required{Node: 200, Hash: "the-hash-it-carried-when-it-was-labelled", Why: "w"})
	result := mustSweep(t, graph, corpusOf(row))

	node := result.Rows[0].Required[0]
	if !node.Stale {
		t.Fatal("Stale is false where the required node moved under its label, want true")
	}
	if node.Verdict != eval.Admitted {
		t.Fatalf("Verdict = %q, want the stale row still scored as %q", node.Verdict, eval.Admitted)
	}
	if !result.Rows[0].Scored() {
		t.Fatal("a stale row is excluded from the rates, want it scored")
	}
	mustContainLine(t, summaryOf(t, result), "stale labels               1")
}

func TestSweepDoesNotFlagARequiredNodeWhoseLiveHashStillMatchesItsLabel(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	row := labelledRow("r01", eval.Required{Node: 200, Hash: requiredNodeBodyHash, Why: "w"})
	result := mustSweep(t, graph, corpusOf(row))

	if result.Rows[0].Required[0].Stale {
		t.Fatal("Stale is true where the live hash still matches the label, want false")
	}
}

func TestSweepExcludesFromBothRatesARowWhoseRequiredNodeNoLongerResolves(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	scored := labelledRow("r01", eval.Required{Node: 200, Hash: requiredNodeBodyHash, Why: "w"})
	gone := labelledRow("r02", eval.Required{Node: 999, Hash: "h", Why: "w"})

	result := mustSweep(t, graph, corpusOf(scored, gone))

	if result.Rows[1].Required[0].Verdict != eval.Unresolved {
		t.Fatalf("Verdict = %q, want %q for an id the graph no longer resolves", result.Rows[1].Required[0].Verdict, eval.Unresolved)
	}
	if result.Rows[1].Scored() {
		t.Fatal("the row carrying an unresolved required node is still scored, want it excluded")
	}

	human := summaryOf(t, result)
	mustContainLine(t, human, "retrieved 1/1 (1.00)")
	mustContainLine(t, human, "unresolved required nodes  1 (excluded from both rates)")
	if strings.Contains(human, "1/2") {
		t.Fatalf("an unresolved required node still counts toward a denominator; output was:\n%s", human)
	}
}

func TestSweepReportsAMissForARequiredNodeTheQueryCannotSurface(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.nodes[300] = loop.Anchor{ID: 300, Type: "documentation", Name: "Unreachable", Content: "a body recall never returns"}
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	row := labelledRow("r01", eval.Required{Node: 300, Hash: "h", Why: "an answer without it is wrong"})
	result := mustSweep(t, graph, corpusOf(row))

	node := result.Rows[0].Required[0]
	if node.Verdict != eval.NotRetrieved {
		t.Fatalf("Verdict = %q, want %q: an unsatisfiable row reported as a hit means the harness is lying", node.Verdict, eval.NotRetrieved)
	}

	human := summaryOf(t, result)
	mustContainLine(t, human, "retrieved 0/1 (0.00)")
	mustContainLine(t, human, "#300")
	mustContainLine(t, human, "r01")
}

func TestSweepReportsARowWhoseSubjectDoesNotResolveWithoutScoringIt(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	row := eval.Row{ID: "r09", Input: "an input", Subject: 777, Stratum: eval.StratumLabelled,
		Required: []eval.Required{{Node: 200, Hash: requiredNodeBodyHash, Why: "w"}}}

	result := mustSweep(t, graph, corpusOf(row))

	if result.Rows[0].Error != rowErrorSubjectNotFound {
		t.Fatalf("Error = %q, want %q", result.Rows[0].Error, rowErrorSubjectNotFound)
	}
	if result.Rows[0].Scored() {
		t.Fatal("a row whose subject does not resolve is scored, want it excluded")
	}
	if len(graph.recallCalls) != 0 {
		t.Fatalf("the sweep ran %d recalls for a row with no subject, want none", len(graph.recallCalls))
	}
	mustContainLine(t, summaryOf(t, result), "r09   subject not found")
}

func TestSweepAbortsRatherThanReportingAnEmptyMeasurementWhenTheGraphReadFails(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.nodeErr = errors.New("graph unavailable")

	_, err := sweep(context.Background(), graph, corpusOf(labelledRow("r01", eval.Required{Node: 200, Hash: "h", Why: "w"})), sweptAt())
	if err == nil {
		t.Fatal("sweep returned a nil error while the graph was unreachable, want an error rather than a plausible zero score")
	}
	if !strings.Contains(err.Error(), "r01") {
		t.Fatalf("error = %q, want it to name the row that failed", err.Error())
	}
}

func TestSweepAbortsWhenRecallItselfFails(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.recallErr = errors.New("graph unavailable")

	_, err := sweep(context.Background(), graph, corpusOf(labelledRow("r01", eval.Required{Node: 200, Hash: "h", Why: "w"})), sweptAt())
	if err == nil {
		t.Fatal("sweep returned a nil error while recall was failing, want an error")
	}
}

func TestSweepReadsTheGraphAndNeverWritesToIt(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	rows := []eval.Row{
		labelledRow("r01", eval.Required{Node: 200, Hash: requiredNodeBodyHash, Why: "w"}),
		labelledRow("r02", eval.Required{Node: 999, Hash: "h", Why: "w"}),
	}
	mustSweep(t, graph, corpusOf(rows...))

	wantNodeCalls := []int64{100, 100, 999}
	if !reflect.DeepEqual(graph.nodeCalls, wantNodeCalls) {
		t.Fatalf("node calls = %v, want %v: one per subject, plus one per required node that was not retrieved", graph.nodeCalls, wantNodeCalls)
	}
	if len(graph.recallCalls) != 2 {
		t.Fatalf("recall calls = %d, want one per row", len(graph.recallCalls))
	}
}

func TestSweepCarriesTheCorpusHashAndRowCountIntoItsResult(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(t)
	graph.candidates = []loop.Candidate{{ID: 200, Content: requiredNodeBody}}

	rows := []eval.Row{
		labelledRow("r01", eval.Required{Node: 200, Hash: requiredNodeBodyHash, Why: "w"}),
		labelledRow("r02", eval.Required{Node: 200, Hash: requiredNodeBodyHash, Why: "w"}),
	}
	result := mustSweep(t, graph, corpusOf(rows...))

	if result.CorpusHash != "a-corpus-hash" {
		t.Fatalf("CorpusHash = %q, want the hash of the corpus swept", result.CorpusHash)
	}
	if result.RowCount != 2 || len(result.Rows) != 2 {
		t.Fatalf("RowCount = %d with %d rows, want 2 and 2", result.RowCount, len(result.Rows))
	}
	for i, want := range []string{"r01", "r02"} {
		if result.Rows[i].Row != want {
			t.Fatalf("Rows[%d].Row = %q, want %q", i, result.Rows[i].Row, want)
		}
	}
}

func mustContainLine(t *testing.T, output, want string) {
	t.Helper()

	if !strings.Contains(output, want) {
		t.Fatalf("output does not contain %q; output was:\n%s", want, output)
	}
}

func TestRunReportsTheMissingCorpusFlagAndSweepsNothing(t *testing.T) {
	t.Parallel()

	var machine, human bytes.Buffer

	code := run(nil, &machine, &human)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(human.String(), "-corpus") {
		t.Fatalf("output = %q, want it to name the flag that is missing", human.String())
	}
	if machine.Len() != 0 {
		t.Fatalf("the machine writer received %d bytes for a failed invocation, want none", machine.Len())
	}
}

func TestRunReportsAnUnknownFlagWithoutSweeping(t *testing.T) {
	t.Parallel()

	var machine, human bytes.Buffer

	code := run([]string{"-nonesuch"}, &machine, &human)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if machine.Len() != 0 {
		t.Fatalf("the machine writer received %d bytes for a failed invocation, want none", machine.Len())
	}
}

func aSweepWhoseOnlyControlNodeScored(verdict eval.Verdict) eval.Result {
	return eval.Result{Rows: []eval.RowResult{
		{Stratum: eval.StratumControl, Required: []eval.NodeResult{{Node: 401, Verdict: verdict}}},
	}}
}

func TestTheSweepExitsNonZeroWhenAControlNodeWasNeverRetrieved(t *testing.T) {
	t.Parallel()

	if got := exitCodeFor(aSweepWhoseOnlyControlNodeScored(eval.NotRetrieved)); got != exitError {
		t.Fatalf("exit code = %d, want %d: an unattended sweep must signal that retrieval itself could not be verified", got, exitError)
	}
}

func TestTheSweepExitsZeroWhenTheBudgetCutAControlNodeTheRetrieverSurfaced(t *testing.T) {
	t.Parallel()

	if got := exitCodeFor(aSweepWhoseOnlyControlNodeScored(eval.Cut)); got != 0 {
		t.Fatalf("exit code = %d, want 0: a cut control is the assembler being measured, not the instrument failing", got)
	}
}

func TestTheSweepExitsNonZeroWhenAControlRowCouldNotBeScoredAtAll(t *testing.T) {
	t.Parallel()

	if got := exitCodeFor(aSweepWhoseOnlyControlNodeScored(eval.Unresolved)); got != exitError {
		t.Fatalf("exit code = %d, want %d: a sweep whose self-check did not run has no self-check", got, exitError)
	}
}

func TestTheSweepExitsNonZeroWhenOneControlNodeWasCutAndAnotherWasNeverRetrieved(t *testing.T) {
	t.Parallel()

	mixed := eval.Result{Rows: []eval.RowResult{
		{Stratum: eval.StratumControl, Required: []eval.NodeResult{{Node: 401, Verdict: eval.Cut}}},
		{Stratum: eval.StratumControl, Required: []eval.NodeResult{{Node: 402, Verdict: eval.NotRetrieved}}},
	}}

	if got := exitCodeFor(mixed); got != exitError {
		t.Fatalf("exit code = %d, want %d: one cut control does not excuse another the retriever never surfaced", got, exitError)
	}
}

func TestTheSweepExitsZeroWhenTheControlStratumReadOneAndOnlyLabelledRowsMissed(t *testing.T) {
	t.Parallel()

	intact := eval.Result{Rows: []eval.RowResult{
		{Stratum: eval.StratumControl, Required: []eval.NodeResult{{Node: 401, Verdict: eval.Admitted, Rank: 1}}},
		{Stratum: eval.StratumLabelled, Required: []eval.NodeResult{{Node: 301, Verdict: eval.NotRetrieved}}},
	}}

	if got := exitCodeFor(intact); got != 0 {
		t.Fatalf("exit code = %d, want 0: a low labelled score is a measurement, not an instrument failure", got)
	}
}
