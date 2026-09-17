package fill

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/condense"
)

type fakeGraph struct {
	node  condense.Node
	found bool
	err   error

	setErr        error
	setSubstance  string
	setSubstances int
}

func (g *fakeGraph) NodeWithSubstance(context.Context, int64) (condense.Node, bool, error) {
	return g.node, g.found, g.err
}

func (g *fakeGraph) Content(context.Context, int64) (string, bool, error) {
	return g.node.Content, g.found, g.err
}

func (g *fakeGraph) SetSubstance(_ context.Context, _ int64, substance string) error {
	g.setSubstances++
	g.setSubstance = substance
	return g.setErr
}

type fakeModel struct {
	completion condense.Completion
	err        error
}

func (m *fakeModel) Condense(context.Context, string, int) (condense.Completion, error) {
	return m.completion, m.err
}

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestFillWritesASubstanceAndNamesTheProducingModel(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("the archive describes a long narrative body of text. ", 20)
	graph := &fakeGraph{
		found: true,
		node:  condense.Node{ID: 7, Name: "Doc", ContentType: "text/plain", Content: content},
	}
	model := &fakeModel{completion: condense.Completion{
		Text:         "a short condensed summary of the narrative, covering its central claim and consequences in brief",
		FinishReason: "stop",
		Model:        "gemma-3-12b-it",
	}}

	port := New(graph, model, fixedClock(time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)))

	result, err := port.Fill(context.Background(), 7)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if !result.Written {
		t.Fatalf("result = %+v, want Written", result)
	}
	if result.Model != "gemma-3-12b-it" {
		t.Fatalf("result.Model = %q, want the completion's own model name", result.Model)
	}
	if graph.setSubstances != 1 || graph.setSubstance != "a short condensed summary of the narrative, covering its central claim and consequences in brief" {
		t.Fatalf("graph.setSubstance = %q (called %d times), want the postprocessed completion written once", graph.setSubstance, graph.setSubstances)
	}
}

func TestFillReportsTheCondensersOwnSkipReason(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{
		found: true,
		node:  condense.Node{ID: 7, Name: "Run", ContentType: "application/json", Content: "irrelevant", SelfProduced: true},
	}
	port := New(graph, &fakeModel{}, nil)

	result, err := port.Fill(context.Background(), 7)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if result.Written {
		t.Fatalf("result = %+v, want a refusal, not a write", result)
	}
	if result.Reason != "self-produced" {
		t.Fatalf("result.Reason = %q, want the condenser's own self-produced gate named verbatim", result.Reason)
	}
}

func TestFillReportsAModelFailureWithItsCause(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{found: true, node: condense.Node{ID: 7, Name: "Doc", Content: "content"}}
	model := &fakeModel{err: errors.New("literal: connection reset")}
	port := New(graph, model, nil)

	result, err := port.Fill(context.Background(), 7)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if result.Written {
		t.Fatal("result.Written = true for a failed model call, want false")
	}
	if !strings.Contains(result.Reason, "connection reset") {
		t.Fatalf("result.Reason = %q, want it to carry the model's own failure cause", result.Reason)
	}
}

func TestFillReportsNodeAbsentWithoutCallingTheModel(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{found: false}
	model := &fakeModel{}
	port := New(graph, model, nil)

	result, err := port.Fill(context.Background(), 999)
	if err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if result.Written {
		t.Fatal("result.Written = true for an absent node, want false")
	}
	if result.Reason != "node absent" {
		t.Fatalf("result.Reason = %q, want %q", result.Reason, "node absent")
	}
}

func TestNewDefaultsTheClockWhenNilIsGiven(t *testing.T) {
	t.Parallel()

	port := New(&fakeGraph{}, &fakeModel{}, nil)
	if port.Now == nil {
		t.Fatal("port.Now is nil, want it defaulted to time.Now")
	}
	if port.Now().IsZero() {
		t.Fatal("port.Now() is zero, want the real clock")
	}
}
