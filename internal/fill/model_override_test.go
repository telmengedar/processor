package fill

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/condense"
)

type selectableFillModel struct {
	id    string
	calls chan string
}

func (m *selectableFillModel) WithModel(id string) condense.ModelPort {
	return &selectableFillModel{id: id, calls: m.calls}
}

func (m *selectableFillModel) Condense(context.Context, string, int) (condense.Completion, error) {
	m.calls <- m.id
	return condense.Completion{Text: "the condensed text", FinishReason: "stop", Model: m.id}, nil
}

func TestPerRunFillModelPersistsThroughSynchronousAndDeferredWorkWithoutChangingDefault(t *testing.T) {
	graph := newFakeGraph(prose(1), prose(2), prose(3), prose(4))
	model := &selectableFillModel{id: "configured-fill", calls: make(chan string, 4)}
	port := New(graph, model, nil, nil)
	selected, err := port.WithModel("selected-fill")
	if err != nil {
		t.Fatalf("WithModel: %v", err)
	}

	result, err := selected.Fill(context.Background(), 1)
	if err != nil || !result.Written || result.Model != "selected-fill" {
		t.Fatalf("selected Fill=%+v, error=%v", result, err)
	}
	result, err = port.Fill(context.Background(), 2)
	if err != nil || !result.Written || result.Model != "configured-fill" {
		t.Fatalf("default Fill=%+v, error=%v", result, err)
	}
	selected.Defer(3)
	selected.Defer(3)
	port.Defer(4)
	if len(port.queued) != 2 {
		t.Fatalf("queued jobs=%d, want two distinct nodes with duplicate selected job removed", len(port.queued))
	}
	if port.Model != model || model.id != "configured-fill" {
		t.Fatal("selection changed the shared fill model")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port.Start(ctx)
	for range 4 {
		select {
		case <-graph.wrote:
		case <-time.After(5 * time.Second):
			t.Fatal("the shared worker did not finish both deferred jobs")
		}
	}
	var calls []string
	for range 4 {
		select {
		case id := <-model.calls:
			calls = append(calls, id)
		default:
			t.Fatalf("missing model calls; got %v", calls)
		}
	}
	if !reflect.DeepEqual(calls, []string{"selected-fill", "configured-fill", "selected-fill", "configured-fill"}) {
		t.Fatalf("synchronous and deferred model calls=%v", calls)
	}
	if graph.readCount() != 4 {
		t.Fatalf("graph reads=%d, want four creations without duplicate work", graph.readCount())
	}
}

func TestPerRunFillModelRejectsAnAdapterWithoutModelSelection(t *testing.T) {
	model := &fakeModel{}
	port := New(newFakeGraph(prose(1)), model, nil, nil)
	selected, err := port.WithModel("selected")
	if err == nil || selected != nil {
		t.Fatalf("WithModel=%T, error=%v, want an unsupported-adapter error", selected, err)
	}
	if model.called() != 0 || port.Model != model {
		t.Fatal("unsupported selection called or replaced the configured model")
	}
}
