package process_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/process"
)

func TestImportSuccessProducesLoadableDefinitionAtDestinationByteBoundary(t *testing.T) {
	b := process.PythonExample()
	low, high := 0, process.MaxDefinitionBytes
	for low+1 < high {
		n := (low + high) / 2
		tool := b.Nodes[0].Body.(process.Tool)
		tool.Python = strings.Repeat("x", n)
		b.Nodes[0].Body = tool
		g := newGraph()
		if _, err := process.Import(context.Background(), g, b); err == nil {
			low = n
		} else {
			high = n
		}
	}
	if low == 0 || high-low != 1 {
		t.Fatal("near-limit import boundary was not reached")
	}
	tool := b.Nodes[0].Body.(process.Tool)
	tool.Python = strings.Repeat("x", low)
	b.Nodes[0].Body = tool
	g := newGraph()
	publication, err := process.Import(context.Background(), g, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = process.Load(context.Background(), g, process.Request{ProcessID: publication.ProcessID}); err != nil {
		t.Fatalf("successful import is not loadable at destination IDs: %v", err)
	}
	tool.Python = strings.Repeat("x", high)
	b.Nodes[0].Body = tool
	g = newGraph()
	publication, err = process.Import(context.Background(), g, b)
	if err == nil {
		t.Fatal("oversized destination definition was accepted")
	}
	if publication.ProcessID > 0 && g.nodes[publication.ProcessID].Content != "" {
		t.Fatal("invalid destination definition root was published")
	}
}

func TestToolRoundIdentityCannotExceedJudgementContextBound(t *testing.T) {
	t.Parallel()
	g, _ := authored(t, process.GangolfChat("knowledge"))
	calls := 0
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		if calls == 1 {
			return completion.Result{Calls: []completion.Call{{ID: strings.Repeat("i", 40000), Name: "read_node", Arguments: json.RawMessage(`{"id":90}`)}}}, nil
		}
		size := 0
		for _, message := range r.Messages {
			size += len(message.Content) + len(message.CallID) + len(message.ToolName)
			for _, call := range message.Calls {
				size += len(call.ID) + len(call.Name) + len(call.Arguments)
			}
		}
		if size > process.MaxContextBytes {
			t.Fatalf("message data %d bytes exceeds context bound %d", size, process.MaxContextBytes)
		}
		return completion.Result{Text: "reply"}, nil
	})
	_, response := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hi","persona":90}}`)
	if response.Status != "completed" || calls != 3 {
		t.Fatalf("tool loop did not complete: %+v calls=%d", response, calls)
	}
	if response.Trace[6].Context[1].Included {
		t.Fatal("oversized optional tool round was admitted")
	}
}

func TestSelectedValueMismatchCannotDispatch(t *testing.T) {
	t.Parallel()
	b := process.GangolfChat("knowledge")
	for i := range b.Nodes {
		if b.Nodes[i].Symbol == "execute_tool" {
			b.Nodes[i].Body.(map[string]any)["input"] = map[string]any{"kind": "tool", "toolId": map[string]any{"nodeRef": "read"}, "args": map[string]any{"id": 999}}
		}
	}
	g, _ := authored(t, b)
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		if calls == 1 {
			return completion.Result{Calls: []completion.Call{{ID: "x", Name: "read_node", Arguments: json.RawMessage(`{"id":90}`)}}}, nil
		}
		return completion.Result{Text: "reply"}, nil
	})
	_, response := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hi","persona":90}}`)
	if response.Error == nil || response.Error.Code != "tool_not_allowed" || calls != 1 || len(response.Trace) != 5 || response.Trace[4].Key != "execute_tool" {
		t.Fatalf("mismatched selection dispatched: status=%s error=%v calls=%d", response.Status, response.Error, calls)
	}
}

func TestCancellationStopsBeforeVisitEffects(t *testing.T) {
	t.Parallel()
	g, publication := authored(t, process.GangolfChat("knowledge"))
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		return completion.Result{Text: "reply"}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := (&process.Engine{Graph: g, Model: m}).Run(ctx, process.Request{ProcessID: publication.ProcessID, Arguments: map[string]any{"message": "hi", "persona": 90}})
	if response.Error == nil || response.Error.Code != "process_cancelled" || calls != 0 || g.recalls != 0 || len(response.Trace) != 0 {
		t.Fatalf("cancelled run executed: status=%s error=%v calls=%d recalls=%d visits=%d", response.Status, response.Error, calls, g.recalls, len(response.Trace))
	}
}
