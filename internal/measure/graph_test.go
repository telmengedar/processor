package measure

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

const (
	eventNode          = "node"
	eventRecall        = "recall"
	eventWrite         = "write"
	eventTurnCompleted = "turnCompleted"
)

func testSystem([]string, bool, bool) string { return "the system text" }

const (
	testModelID = "test-model-id"
	testAnswer  = "the answer the model gave"
	testInput   = "where does the change land"
)

type graphEvent struct {
	kind    string
	subject int64
}

type recordingGraph struct {
	mu      sync.Mutex
	events  []graphEvent
	recalls []loop.RecallRequest

	anchor     loop.Anchor
	found      bool
	candidates []loop.Candidate
	receipt    loop.WriteReceipt
}

func newRecordingGraph() *recordingGraph {
	return &recordingGraph{
		anchor: loop.Anchor{ID: 101, Type: "documentation", Name: "the anchor", Substance: "the anchor body the run is about"},
		found:  true,
		candidates: []loop.Candidate{
			{ID: 202, Type: "documentation", Name: "a candidate", Similarity: 0.91, Substance: "the candidate body recall returned"},
		},
		receipt: loop.WriteReceipt{State: loop.Stored, NodeID: 900},
	}
}

func (g *recordingGraph) mark(kind string, subject int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.events = append(g.events, graphEvent{kind: kind, subject: subject})
}

func (g *recordingGraph) log() []graphEvent {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]graphEvent(nil), g.events...)
}

func (g *recordingGraph) recallCalls() []loop.RecallRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]loop.RecallRequest(nil), g.recalls...)
}

func (g *recordingGraph) Node(_ context.Context, id int64, _ loop.Filter) (loop.Anchor, bool, error) {
	g.mark(eventNode, id)
	return g.anchor, g.found, nil
}

func (g *recordingGraph) Recall(_ context.Context, request loop.RecallRequest) ([]loop.Candidate, error) {
	g.mark(eventRecall, 0)

	g.mu.Lock()
	g.recalls = append(g.recalls, request)
	g.mu.Unlock()

	return g.candidates, nil
}

func (g *recordingGraph) WriteRun(_ context.Context, record loop.Record) loop.WriteReceipt {
	g.mark(eventWrite, record.Subject)
	return g.receipt
}

type answeringModel struct {
	mu         sync.Mutex
	inputs     []loop.JudgeInput
	derivation string
}

func (m *answeringModel) Judge(_ context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.mu.Lock()
	m.inputs = append(m.inputs, in)
	m.mu.Unlock()

	return loop.JudgeResult{
		Answer:    testAnswer,
		Reason:    loop.Answered,
		RawReason: "stop",
		Usage:     &loop.Usage{InTokens: 11, OutTokens: 22},
	}, nil
}

func (m *answeringModel) Derive(context.Context, string, int) (string, error) {
	if m.derivation != "" {
		return m.derivation, nil
	}
	return "a second phrasing of the task?\na third phrasing of the task?", nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func fixedClock() time.Time {
	return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
}

func measuredRunner(live loop.GraphPort) (*Graph, *Runner) {
	return measuredRunnerWith(live, &answeringModel{})
}

func measuredRunnerWith(live loop.GraphPort, model *answeringModel) (*Graph, *Runner) {
	graph := NewGraph(live, fixedClock, testLogger())
	return graph, NewRunner(graph, NewCondenseGraph(newRecordingCondenseGraph()), model, nil, nil, testSystem, testModelID, testLogger())
}

type writingFill struct {
	graph     *CondenseGraph
	substance string
}

func (f *writingFill) Fill(ctx context.Context, id int64) (loop.FillResult, error) {
	if err := f.graph.SetSubstance(ctx, id, f.substance); err != nil {
		return loop.FillResult{}, err
	}
	return loop.FillResult{Written: true, Substance: f.substance, Model: "test-condenser"}, nil
}

func (f *writingFill) Defer(int64) {}

func crowdedGraph() *recordingGraph {
	live := newRecordingGraph()
	live.candidates = []loop.Candidate{
		{ID: 202, Type: "documentation", Name: "the candidate that fits", Similarity: 0.95, Content: strings.Repeat("the first candidate body. ", 1600)},
		{ID: 303, Type: "documentation", Name: "the candidate cut for want of room", Similarity: 0.94, Content: strings.Repeat("the second candidate body. ", 1600)},
	}
	return live
}

func fillingRunner(live loop.GraphPort) (*CondenseGraph, *Runner) {
	graph := NewGraph(live, fixedClock, testLogger())
	substances := NewCondenseGraph(newRecordingCondenseGraph())
	fill := &writingFill{graph: substances, substance: testSubstance}

	return substances, NewRunner(graph, substances, &answeringModel{}, nil, fill, testSystem, testModelID, testLogger())
}

func countEvents(events []graphEvent, kind string) int {
	count := 0
	for _, e := range events {
		if e.kind == kind {
			count++
		}
	}
	return count
}

func lastEventIndex(events []graphEvent, kind string) int {
	last := -1
	for i, e := range events {
		if e.kind == kind {
			last = i
		}
	}
	return last
}

func TestNoRecordReachesTheGraphBeforeEveryTurnOfTheComparisonHasCompleted(t *testing.T) {
	live := newRecordingGraph()
	_, runner := measuredRunner(live)

	for _, subject := range []int64{101, 202} {
		if _, err := runner.Run(context.Background(), testInput, subject); err != nil {
			t.Fatalf("the measured run for subject %d failed: %v", subject, err)
		}
		live.mark(eventTurnCompleted, subject)
	}

	events := live.log()

	lastCompleted := lastEventIndex(events, eventTurnCompleted)
	if lastCompleted < 0 {
		t.Fatal("no turn recorded its completion in the graph's own event order, so the ordering this test asserts was never observable")
	}
	if completions := countEvents(events, eventTurnCompleted); completions != 2 {
		t.Fatalf("want two completions in the event order, got %d", completions)
	}

	for i, e := range events {
		if e.kind == eventWrite && i <= lastCompleted {
			t.Fatalf("the graph saw a write for subject %d at position %d of its event order, before the last turn completed at position %d", e.subject, i, lastCompleted)
		}
	}
}

func TestAMeasuredRunFilesNoRecordAtAllAndItsReceiptSaysNoNodeHoldsIt(t *testing.T) {
	live := newRecordingGraph()
	_, runner := measuredRunner(live)

	for _, subject := range []int64{101, 202} {
		result, err := runner.Run(context.Background(), testInput, subject)
		if err != nil {
			t.Fatalf("the measured run for subject %d failed: %v", subject, err)
		}
		if result.Written.State != loop.NotStored {
			t.Fatalf("the receipt for subject %d reads %q, want %q", subject, result.Written.State, loop.NotStored)
		}
		if result.Written.NodeID != 0 {
			t.Fatalf("the receipt for subject %d names node %d, and a record nothing holds has no node", subject, result.Written.NodeID)
		}
		if live.receipt.NodeID == 0 {
			t.Fatal("the graph this run was decorated over files records at node zero, so the receipt could not have named a node either way and the assertion above is vacuous")
		}
	}

	if writes := countEvents(live.log(), eventWrite); writes != 0 {
		t.Fatalf("the graph saw %d writes, and a measured run files nothing at all", writes)
	}
}

func TestAMeasuredRunsReadsStillReachTheGraphAndItsBlockCarriesWhatTheyReturned(t *testing.T) {
	live := newRecordingGraph()
	_, runner := measuredRunner(live)

	result, err := runner.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the measured run failed: %v", err)
	}

	events := live.log()
	for _, kind := range []string{eventNode, eventRecall} {
		if countEvents(events, kind) == 0 {
			t.Fatalf("the graph saw no %s read, and a measured run reads the graph exactly as a filed run does", kind)
		}
	}

	if result.Anchor.ID != live.anchor.ID {
		t.Fatalf("the record anchors on node %d, want the node the graph returned, %d", result.Anchor.ID, live.anchor.ID)
	}
	if !strings.Contains(result.Block, live.anchor.Substance) {
		t.Fatal("the block does not carry the anchor body the graph returned")
	}
	if !strings.Contains(result.Block, live.candidates[0].Substance) {
		t.Fatal("the block does not carry the candidate body recall returned")
	}
	if len(result.Candidates) != 1 || result.Candidates[0].ID != live.candidates[0].ID || !result.Candidates[0].Included {
		t.Fatalf("want the recalled candidate admitted into the block, got %+v", result.Candidates)
	}
}

func TestAMeasuredRunsRecallsCarryTheSameQueriesLimitAndWindowAnUndecoratedRunCarries(t *testing.T) {
	const derivation = "DATES: 2026-09-01..2026-09-10\na second phrasing of the task?"
	measuredLive := newRecordingGraph()
	_, runner := measuredRunnerWith(measuredLive, &answeringModel{derivation: derivation})
	if _, err := runner.Run(context.Background(), testInput, 101); err != nil {
		t.Fatalf("the measured run failed: %v", err)
	}

	shippedLive := newRecordingGraph()
	turn := loop.NewTurn(shippedLive, &answeringModel{derivation: derivation}, nil, testSystem, testModelID, testLogger())
	if _, _, err := turn.Run(context.Background(), testInput, 101); err != nil {
		t.Fatalf("the undecorated run failed: %v", err)
	}

	measured, shipped := measuredLive.recallCalls(), shippedLive.recallCalls()
	if len(measured) == 0 {
		t.Fatal("the measured run issued no recall at all")
	}
	if len(measured) != len(shipped) {
		t.Fatalf("the measured run issued %d recalls, the undecorated run %d", len(measured), len(shipped))
	}
	windowed := 0
	for _, recall := range shipped {
		if from, to := recall.Window.From.Format(time.DateOnly), recall.Window.To.Format(time.DateOnly); from == "2026-09-01" && to == "2026-09-11" {
			windowed++
		}
	}
	if windowed == 0 {
		t.Fatalf("no undecorated recall carries the derived window, got %+v: the fixture must carry a window for this comparison to see one", shipped)
	}
	for i := range measured {
		if !reflect.DeepEqual(measured[i], shipped[i]) {
			t.Fatalf("recall %d differs: measured %+v, undecorated %+v", i, measured[i], shipped[i])
		}
	}
}

func TestARunThatNeverProducedARecordSuppressesNothing(t *testing.T) {
	live := newRecordingGraph()
	live.found = false
	graph, runner := measuredRunner(live)

	if _, err := runner.Run(context.Background(), testInput, 101); err == nil {
		t.Fatal("the run reported no error for a subject the graph does not hold")
	}

	if suppressed := graph.Suppressed(); len(suppressed) != 0 {
		t.Fatalf("want no suppressed write for a run that produced no record, got %+v", suppressed)
	}
	if writes := countEvents(live.log(), eventWrite); writes != 0 {
		t.Fatalf("the graph saw %d writes for a run that produced no record", writes)
	}
}

type postedContent struct {
	mu    sync.Mutex
	bytes int
}

func (p *postedContent) set(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bytes = n
}

func (p *postedContent) get() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bytes
}

func sizeableRecord() loop.Record {
	return loop.Record{
		Input:      testInput,
		Subject:    101,
		Now:        fixedClock(),
		Query:      testInput,
		Queries:    []string{testInput, "a derived query"},
		Anchor:     loop.AnchorSummary{ID: 101, Type: "documentation", Name: "the anchor", Size: 32, ContentHash: "a-hash"},
		Candidates: []loop.Disposition{{Rank: 1, ID: 202, Type: "documentation", Name: "a candidate", Similarity: 0.91, Size: 34, Included: true, Form: loop.FormSubstance, RenderedSize: 34}},
		Fills:      []loop.FillOutcome{},
		Block:      strings.Repeat("the block the run assembled. ", 64),
		Answer:     testAnswer,
		Model:      testModelID,
		Provider:   loop.Provider{Adapter: "openai-compat", Endpoint: "http://endpoint/v1"},
		ToolCalls:  []loop.ToolCallRecord{},
		ModelCalls: 1,
		Usage:      []*loop.Usage{{InTokens: 11, OutTokens: 22}},
		StopReason: loop.StopReason{Reason: loop.Answered, Raw: "stop"},
		Limits:     loop.Limits{CandidateLimit: loop.CandidateLimit, AssemblyByteBudget: loop.AssemblyByteBudget},
	}
}

func TestASuppressedWritesSizeIsTheByteLengthTheGraphWouldHaveReceivedForThatRecord(t *testing.T) {
	posted := &postedContent{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/content") {
			body, _ := io.ReadAll(r.Body)
			posted.set(len(body))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":900}`)
	}))
	defer srv.Close()

	record := sizeableRecord()

	receipt := divoid.NewClient(srv.URL, "a-key", nil, testLogger()).WriteRun(context.Background(), record)
	if receipt.State != loop.Stored {
		t.Fatalf("the shipped adapter reported %q, and this test needs the write it would really have made", receipt.State)
	}
	if posted.get() == 0 {
		t.Fatal("the shipped adapter posted no content, so there is nothing to compare the reported size against")
	}

	size, err := FiledSize(record, fixedClock())
	if err != nil {
		t.Fatalf("sizing the record failed: %v", err)
	}
	if size != posted.get() {
		t.Fatalf("the suppressed write reports %d bytes, the graph received %d", size, posted.get())
	}
}

func TestASuppressedWriteCarriesTheSubjectTheOrdinalAndTheSizeOfEachRecordInTurn(t *testing.T) {
	live := newRecordingGraph()
	graph, runner := measuredRunner(live)

	for _, subject := range []int64{101, 202} {
		result, err := runner.Run(context.Background(), testInput, subject)
		if err != nil {
			t.Fatalf("the measured run for subject %d failed: %v", subject, err)
		}
		if len(result.Suppressed) != 1 {
			t.Fatalf("want one suppressed write in the result for subject %d, got %d", subject, len(result.Suppressed))
		}
		if result.Suppressed[0].Subject != subject {
			t.Fatalf("the suppressed write names subject %d, want %d", result.Suppressed[0].Subject, subject)
		}
		if result.Suppressed[0].Size <= len(result.Block) {
			t.Fatalf("the suppressed write reports %d bytes for a record whose block alone is %d", result.Suppressed[0].Size, len(result.Block))
		}
	}

	suppressed := graph.Suppressed()
	if len(suppressed) != 2 {
		t.Fatalf("want both runs' suppressed writes on the graph, got %d", len(suppressed))
	}
	if suppressed[0].Ordinal != 1 || suppressed[1].Ordinal != 2 {
		t.Fatalf("want the suppressed writes ordinalled 1 then 2, got %d then %d", suppressed[0].Ordinal, suppressed[1].Ordinal)
	}
	if suppressed[0].Subject != 101 || suppressed[1].Subject != 202 {
		t.Fatalf("want the suppressed writes in the order the runs produced them, got subjects %d then %d", suppressed[0].Subject, suppressed[1].Subject)
	}
}

func TestAnUndecoratedTurnFilesItsRecordAndTheFakeGraphRecordsThatWrite(t *testing.T) {
	live := newRecordingGraph()
	turn := loop.NewTurn(live, &answeringModel{}, nil, testSystem, testModelID, testLogger())

	if _, _, err := turn.Run(context.Background(), testInput, 101); err != nil {
		t.Fatalf("the undecorated run failed: %v", err)
	}

	if got := countEvents(live.log(), eventWrite); got != 1 {
		t.Fatalf("the fake graph recorded %d writes for one filed record, so its write marker cannot witness a write at all and every zero-write assertion in this package is vacuous", got)
	}
}

func TestARecordThatCannotBeEncodedIsReportedAsUnsizedRatherThanAsZeroBytes(t *testing.T) {
	record := sizeableRecord()
	record.Candidates[0].Similarity = math.NaN()

	if _, err := FiledSize(record, fixedClock()); err == nil {
		t.Fatal("sizing a record the graph could not have been sent reported no error")
	}

	graph := NewGraph(newRecordingGraph(), fixedClock, testLogger())
	if receipt := graph.WriteRun(context.Background(), record); receipt.State != loop.NotStored {
		t.Fatalf("the receipt reads %q, want %q even for a record that could not be sized", receipt.State, loop.NotStored)
	}

	suppressed := graph.Suppressed()
	if len(suppressed) != 1 || suppressed[0].Size != UnsizedRecord {
		t.Fatalf("the suppressed write reports %+v, want one entry sized %d", suppressed, UnsizedRecord)
	}
}

func TestAMeasuredRunWhoseFillWroteNothingStillReportsNoSubstanceWithheld(t *testing.T) {
	_, runner := measuredRunner(newRecordingGraph())

	result, err := runner.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the measured run failed: %v", err)
	}

	if len(result.Substances) != 0 {
		t.Fatalf("want no substance withheld by a run whose fill never fired, got %+v", result.Substances)
	}
}

func TestAMeasuredTurnRendersTheCreatedSubstanceAndWritesNothing(t *testing.T) {
	live := crowdedGraph()
	substances, runner := fillingRunner(live)

	result, err := runner.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the measured run failed: %v", err)
	}

	if got := strings.Count(result.Block, testSubstance); got != 2 {
		t.Fatalf("the block carries the created substance %d times, want 2 (one per candidate; the anchor already carried its own):\n%s", got, result.Block)
	}
	if strings.Contains(result.Block, "unavailable") {
		t.Fatalf("a row rendered as unavailable although its creation succeeded:\n%s", result.Block)
	}

	for _, got := range [][]SuppressedSubstance{result.Substances, substances.Substances()} {
		nodes := make([]int64, 0, len(got))
		for _, withheld := range got {
			if withheld.Size != len(testSubstance) {
				t.Fatalf("a withheld substance has size %d, want %d", withheld.Size, len(testSubstance))
			}
			nodes = append(nodes, withheld.Node)
		}
		slices.Sort(nodes)
		if want := []int64{202, 303}; !slices.Equal(nodes, want) {
			t.Fatalf("the withheld substances belong to %v, want %v", nodes, want)
		}
	}
	if writes := countEvents(live.log(), eventWrite); writes != 0 {
		t.Fatalf("the graph saw %d writes from a run whose fill fired", writes)
	}
}
