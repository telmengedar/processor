package fill

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/condense"
)

type fakeGraph struct {
	mu    sync.Mutex
	nodes map[int64]condense.Node
	set   map[int64]string
	wrote chan int64
	reads int
}

func newFakeGraph(nodes ...condense.Node) *fakeGraph {
	g := &fakeGraph{nodes: map[int64]condense.Node{}, set: map[int64]string{}, wrote: make(chan int64, 16)}
	for _, n := range nodes {
		g.nodes[n.ID] = n
	}
	return g
}

func (g *fakeGraph) NodeWithSubstance(_ context.Context, id int64) (condense.Node, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.reads++
	n, ok := g.nodes[id]
	return n, ok, nil
}

func (g *fakeGraph) readCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reads
}

func (g *fakeGraph) Content(_ context.Context, id int64) (string, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	n, ok := g.nodes[id]
	return n.Content, ok, nil
}

func (g *fakeGraph) SetSubstance(_ context.Context, id int64, substance string) error {
	g.mu.Lock()
	g.set[id] = substance
	n := g.nodes[id]
	n.Substance = substance
	g.nodes[id] = n
	g.mu.Unlock()

	g.wrote <- id
	return nil
}

func (g *fakeGraph) written(id int64) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	text, ok := g.set[id]
	return text, ok
}

type fakeModel struct {
	mu    sync.Mutex
	calls int
	text  string
	err   error
}

func (m *fakeModel) Condense(_ context.Context, _ string, _ int) (condense.Completion, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()

	if m.err != nil {
		return condense.Completion{}, m.err
	}
	return condense.Completion{Text: m.text, FinishReason: "stop", Model: "test-condense-model"}, nil
}

func (m *fakeModel) called() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func prose(id int64) condense.Node {
	return condense.Node{ID: id, Type: "documentation", Name: "A node", Content: strings.Repeat("the body ", 50)}
}

func TestACreatedSubstanceIsWrittenBackThroughThePort(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(prose(1))
	port := New(graph, &fakeModel{text: "the condensed text"}, nil)

	result, err := port.Fill(context.Background(), 1)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}

	if !result.Written || result.Substance != "the condensed text" || result.Model != "test-condense-model" {
		t.Fatalf("result = %+v, want a written substance carrying the text and the producing model", result)
	}
	if text, ok := graph.written(1); !ok || text != "the condensed text" {
		t.Fatalf("the graph holds %q (written %v), want the substance written back to the node", text, ok)
	}
}

func TestASubstanceAlreadyPresentIsReturnedUnwritten(t *testing.T) {
	t.Parallel()

	node := prose(1)
	node.Substance = "somebody else got there first"
	graph := newFakeGraph(node)
	model := &fakeModel{text: "never asked for"}
	port := New(graph, model, nil)

	result, err := port.Fill(context.Background(), 1)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}

	if result.Written || result.Substance != "somebody else got there first" {
		t.Fatalf("result = %+v, want the present substance returned and not marked written", result)
	}
	if model.called() != 0 {
		t.Fatalf("the model was called %d times, want none: the node already carries a substance", model.called())
	}
}

func TestAProducerSkipIsReportedAsAReasonWithNoSubstance(t *testing.T) {
	t.Parallel()

	port := New(newFakeGraph(prose(1)), &fakeModel{text: "Here is the condensed text"}, nil)

	result, err := port.Fill(context.Background(), 1)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}

	if result.Written || result.Substance != "" || !strings.Contains(result.Reason, "preamble") {
		t.Fatalf("result = %+v, want no substance and the producer's reason", result)
	}
}

func TestAModelFailureIsReportedAsAReasonWithNoSubstance(t *testing.T) {
	t.Parallel()

	port := New(newFakeGraph(prose(1)), &fakeModel{err: errors.New("endpoint down")}, nil)

	result, err := port.Fill(context.Background(), 1)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}

	if result.Written || result.Substance != "" || !strings.Contains(result.Reason, "endpoint down") {
		t.Fatalf("result = %+v, want no substance and the cause", result)
	}
}

func TestTheDeferredWorkerCondensesQueuedNodesAfterTheTurnAndDeduplicatesThem(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(prose(1), prose(2))
	model := &fakeModel{text: "the condensed text"}
	port := New(graph, model, nil)

	port.Defer(1)
	port.Defer(1)
	port.Defer(2)

	if model.called() != 0 {
		t.Fatalf("the model was called %d times by Defer itself, want none: deferring only queues", model.called())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port.Start(ctx)

	for range 2 {
		select {
		case <-graph.wrote:
		case <-time.After(5 * time.Second):
			t.Fatal("the worker did not write the queued nodes")
		}
	}

	time.Sleep(200 * time.Millisecond)
	if model.called() != 2 {
		t.Fatalf("the model was called %d times, want 2: node 1 was deferred twice and is condensed once", model.called())
	}
	if graph.readCount() != 2 {
		t.Fatalf("the graph was read %d times, want 2: a node deferred twice is queued once, not read twice", graph.readCount())
	}
	for _, id := range []int64{1, 2} {
		if _, ok := graph.written(id); !ok {
			t.Fatalf("node %d was not written", id)
		}
	}
}

func TestAPortThatWasNeverStartedOnlyRecordsWhatIsDeferred(t *testing.T) {
	t.Parallel()

	model := &fakeModel{text: "the condensed text"}
	port := New(newFakeGraph(prose(1)), model, nil)

	port.Defer(1)

	time.Sleep(100 * time.Millisecond)
	if model.called() != 0 {
		t.Fatalf("the model was called %d times by a port nobody started", model.called())
	}
	if len(port.queued) != 1 {
		t.Fatalf("the port holds %d queued nodes, want the one deferred", len(port.queued))
	}
}

func TestDeferNeverBlocksEvenWhenNothingIsDrainingTheQueue(t *testing.T) {
	t.Parallel()

	port := New(newFakeGraph(), &fakeModel{}, nil)

	done := make(chan struct{})
	go func() {
		for id := range int64(1000) {
			port.Defer(id)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Defer blocked")
	}
}

func TestTheWorkerStopsWithItsContext(t *testing.T) {
	t.Parallel()

	graph := newFakeGraph(prose(1))
	model := &fakeModel{text: "the condensed text"}
	port := New(graph, model, nil)

	ctx, cancel := context.WithCancel(context.Background())
	port.Start(ctx)
	cancel()

	time.Sleep(50 * time.Millisecond)
	port.Defer(1)
	time.Sleep(100 * time.Millisecond)

	if model.called() != 0 {
		t.Fatalf("the model was called %d times after the worker's context ended", model.called())
	}
}
