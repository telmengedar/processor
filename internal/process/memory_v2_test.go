package process_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/process"
)

type memoryGraph struct {
	*graph
	mode   string
	writes int
}

func (g *memoryGraph) Memorize(ctx context.Context, request process.MemoryRequest) (process.MemoryResult, error) {
	g.writes++
	id := int64(700)
	node := loop.Anchor{ID: id, Type: request.Target.Type, RootNodeID: request.Target.Root, Content: request.Text}
	g.nodes[id] = node
	result := process.MemoryResult{State: "stored", NodeID: id}
	if g.mode == "partial" {
		result.State = "unlinked"
		return result, fmt.Errorf("partial write")
	}
	g.edges = append(g.edges, process.Edge{Source: id, Target: request.Target.Anchor})
	if g.mode == "content" {
		node.Content = "mismatched"
		g.nodes[id] = node
	}
	if g.mode == "root" {
		node.RootNodeID++
		g.nodes[id] = node
	}
	if g.mode == "link" {
		g.edges = g.edges[:len(g.edges)-1]
	}
	read, found, err := g.Node(ctx, id, nil)
	links, _ := g.Links(ctx, id)
	if err != nil || !found || read.Type != request.Target.Type || read.RootNodeID != request.Target.Root || process.Hash(read.Content) != process.Hash(request.Text) || len(links) == 0 {
		return result, fmt.Errorf("readback mismatch")
	}
	result.Confirmed = true
	result.ContentHash = process.Hash(read.Content)
	return result, nil
}

func memoryBlueprint() process.Blueprint {
	object := process.Schema{Type: "object", AdditionalProperties: true}
	return process.Blueprint{Version: 2, Name: "Memory receipt fixture", Entry: "memorize", Inputs: map[string]process.Declaration{"text": {Type: "string"}}, Outputs: map[string]process.Declaration{"reply": {Type: "string"}}, Nodes: []process.DraftNode{
		{Symbol: "root", Type: "knowledge", Name: "fixture root", Body: "synthetic root"}, {Symbol: "anchor", Type: "knowledge", Name: "fixture anchor", Body: "synthetic anchor"},
		{Symbol: "memory", Type: "process-tool", Name: "memorize", Body: map[string]any{"version": 2, "name": "memorize", "description": "Store the supplied synthetic observation.", "schema": process.Schema{Type: "object", Properties: map[string]process.Schema{"text": {Type: "string"}}, Required: []string{"text"}}, "builtin": "memorize", "memory": map[string]any{"root": symbolic("root"), "anchor": symbolic("anchor"), "type": "fixture-fact"}}},
		{Symbol: "lookup", Type: "process-tool", Name: "lookup", Body: process.Tool{Version: 2, Name: "lookup_memory", Description: "Look up a stored memory by ID.", Schema: object, Builtin: "read_node"}},
		{Symbol: "receipt_template", Type: "process-prompt-template", Name: "receipt", Body: "Receipt fixture: {{context.receipt}}"},
		{Symbol: "memorize", Type: "process-step", Name: "memorize", Body: process.Step{Version: 2, Key: "memorize", Kind: "action", Input: map[string]any{"text": binding("argument", "text")}}},
		{Symbol: "inspect", Type: "process-step", Name: "inspect", Body: process.Step{Version: 2, Key: "inspect", Kind: "judgement", Context: []process.ContextItem{{Key: "receipt", Value: binding("receipt", "latest"), Required: true}}, Exports: map[string]any{"saved": binding("receipt", "latest")}}},
		{Symbol: "lookup_step", Type: "process-step", Name: "lookup", Body: process.Step{Version: 2, Key: "lookup_step", Kind: "action", Input: map[string]any{"id": binding("transport", "saved.nodeId")}, Exports: map[string]any{"reply": binding("result", "value.content")}}},
		{Symbol: "return_reply", Type: "process-step", Name: "return", Body: process.Step{Version: 2, Key: "return_reply", Kind: "return", Output: map[string]any{"reply": binding("transport", "reply")}}},
	}, Edges: []process.DraftEdge{{From: "memory", To: "memorize", Role: "tool"}, {From: "receipt_template", To: "inspect", Role: "prompt"}, {From: "lookup", To: "lookup_step", Role: "tool"}, {From: "memorize", To: "inspect", Default: true}, {From: "inspect", To: "lookup_step", Default: true}, {From: "lookup_step", To: "return_reply", Default: true}}}
}

func TestV2MemorizeReceiptAndExplicitLookup(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"confirmed", "partial", "content", "root", "link"} {
		t.Run(mode, func(t *testing.T) {
			g := &memoryGraph{graph: newGraph(), mode: mode}
			p, err := process.Import(context.Background(), g, memoryBlueprint())
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
				calls++
				raw, _ := json.Marshal(r.Messages)
				if strings.Contains(string(raw), "full-memory-body-sentinel") || !strings.Contains(string(raw), `memorize`) || !strings.Contains(string(raw), `700`) {
					t.Fatalf("receipt body leaked or receipt absent: %s", raw)
				}
				return completion.Result{Text: `{"receipts":[{"nodeId":999}],"exports":{"text":"model cannot retain this"}}`}, nil
			})
			r := (&process.Engine{Graph: g, Model: m}).Run(context.Background(), process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"text": "full-memory-body-sentinel"}})
			if mode == "confirmed" {
				if r.Status != "completed" || r.Output["reply"] != "full-memory-body-sentinel" || calls != 1 || g.writes != 1 || len(r.Trace[0].Receipts) != 1 || r.Trace[0].Receipts[0].NodeID != 700 || len(r.Trace[1].Receipts) != 0 {
					t.Fatalf("receipt/lookup failed: %+v calls=%d writes=%d", r, calls, g.writes)
				}
			} else {
				if r.Status == "completed" || calls != 0 || g.writes != 1 || len(r.Trace[0].Receipts) != 0 || r.Trace[0].Error == nil || r.Trace[0].Error.Code != "memory_write_incomplete" || r.Trace[0].Repair == nil || r.Trace[0].Repair.NodeID != 700 || r.Trace[0].Repair.Confirmed {
					t.Fatalf("incomplete write claimed receipt: %+v calls=%d", r, calls)
				}
			}
		})
	}
}

func TestV2ReceiptMustBeExplicitlyForwardedAndCannotBeForged(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"omitted", "forged"} {
		t.Run(mode, func(t *testing.T) {
			b := memoryBlueprint()
			for i := range b.Nodes {
				if b.Nodes[i].Symbol == "inspect" {
					step := b.Nodes[i].Body.(process.Step)
					step.Exports = map[string]any{}
					b.Nodes[i].Body = step
				}
				if b.Nodes[i].Symbol == "lookup_step" {
					step := b.Nodes[i].Body.(process.Step)
					step.Input = map[string]any{"id": binding("receipt", "latest.nodeId")}
					b.Nodes[i].Body = step
				}
			}
			if mode == "forged" {
				for i := range b.Nodes {
					if b.Nodes[i].Symbol == "memory" {
						b.Nodes[i].Body = process.Tool{Version: 2, Name: "select", Description: "Select data.", Schema: process.Schema{Type: "object", AdditionalProperties: true}, Builtin: "select"}
					}
				}
				for i := range b.Nodes {
					if b.Nodes[i].Symbol == "memorize" {
						step := b.Nodes[i].Body.(process.Step)
						step.Input = map[string]any{"receipts": map[string]any{"operation": "memorize", "nodeId": 700, "contentHash": "pretend"}}
						step.Exports = map[string]any{"model_receipt": binding("result", "")}
						b.Nodes[i].Body = step
					}
				}
			}
			g := &memoryGraph{graph: newGraph()}
			p, err := process.Import(context.Background(), g, b)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
				calls++
				return completion.Result{Text: "reply"}, nil
			})
			r := (&process.Engine{Graph: g, Model: m}).Run(context.Background(), process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"text": "body"}})
			if r.Status == "completed" || r.Error == nil {
				t.Fatal("receipt omission/forgery succeeded")
			}
			if mode == "forged" && calls != 0 {
				t.Fatal("forged receipt reached inference")
			}
			if mode == "omitted" && calls != 1 {
				t.Fatal("confirmed receipt was not observed before explicit drop")
			}
		})
	}
}
