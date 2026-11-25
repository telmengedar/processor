package process_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/process"
)

type accessMemoryGraph struct {
	*graph
	writer   process.MemoryWriter
	requests []process.MemoryRequest
}

func (g *accessMemoryGraph) Memorize(ctx context.Context, request process.MemoryRequest) (process.MemoryResult, error) {
	g.requests = append(g.requests, request)
	return g.writer.Memorize(ctx, request)
}

func accessMemoryBlueprint(access string) process.Blueprint {
	b := memoryBlueprint()
	b.Nodes = b.Nodes[:3]
	if access != "" {
		b.Nodes[2].Body.(map[string]any)["memory"].(map[string]any)["access"] = json.RawMessage(access)
	}
	b.Outputs = map[string]process.Declaration{"nodeId": {Type: "integer"}}
	b.Nodes = append(b.Nodes,
		process.DraftNode{Symbol: "memorize", Type: "process-step", Name: "memorize", Body: process.Step{Version: 2, Key: "memorize", Kind: "action", Input: map[string]any{"text": binding("argument", "text")}, Exports: map[string]any{"saved": binding("receipt", "latest")}}},
		process.DraftNode{Symbol: "done", Type: "process-step", Name: "done", Body: process.Step{Version: 2, Key: "done", Kind: "return", Output: map[string]any{"nodeId": binding("transport", "saved.nodeId")}}},
	)
	b.Edges = []process.DraftEdge{{From: "memory", To: "memorize", Role: "tool"}, {From: "memorize", To: "done", When: &process.Predicate{Path: "ok", Equals: true}}}
	return b
}

func TestMemoryAccessDefinitionRejectsInvalidFlagsBeforePublication(t *testing.T) {
	for _, access := range []string{"", "0", "1", "2", "3", "-1", "4", "1.5", `"None"`, "true", "null"} {
		t.Run("access="+access, func(t *testing.T) {
			b := accessMemoryBlueprint(access)
			g := newGraph()
			_, err := process.Import(context.Background(), g, b)
			valid := access == "" || access == "0" || access == "1" || access == "2" || access == "3"
			if valid {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || len(g.nodes) != 1 || len(g.publications) != 0 {
				t.Fatalf("invalid access reached publication: err=%v nodes=%d publications=%d", err, len(g.nodes), len(g.publications))
			}
		})
	}
}

func TestMemoryAccessReceiptRequiresActualAdapterConfirmation(t *testing.T) {
	for _, mode := range []string{"private", "mismatch", "missing", "omitted"} {
		t.Run(mode, func(t *testing.T) {
			creates := 0
			var root, anchor int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					if r.URL.Path == "/api/nodes" {
						creates++
						var body map[string]json.RawMessage
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						if mode == "omitted" {
							if _, present := body["access"]; present {
								t.Error("omitted access changed the create payload")
							}
						} else if string(body["access"]) != "0" {
							t.Error("explicit private access absent from first create")
						}
						_, _ = w.Write([]byte(`{"id":900}`))
					}
					return
				}
				if r.URL.Path == "/api/nodes/links" {
					_ = json.NewEncoder(w).Encode(map[string]any{"result": []process.Edge{{Source: 900, Target: anchor}}})
					return
				}
				row := map[string]any{"id": 900, "type": "fixture-fact", "rootNodeId": root, "content": "synthetic private memory"}
				if mode == "private" {
					row["access"] = "None"
				} else if mode == "mismatch" {
					row["access"] = "Read, Write"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{row}})
			}))
			defer srv.Close()
			g := &accessMemoryGraph{graph: newGraph(), writer: divoid.NewClient(srv.URL, "", srv.Client(), nil)}
			access := "0"
			if mode == "omitted" {
				access = ""
			}
			p, err := process.Import(context.Background(), g, accessMemoryBlueprint(access))
			if err != nil {
				t.Fatal(err)
			}
			root, anchor = p.Members["root"], p.Members["anchor"]
			r := (&process.Engine{Graph: g}).Run(context.Background(), process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"text": "synthetic private memory"}})
			if creates != 1 || len(g.requests) != 1 {
				t.Fatalf("write retried or omitted: creates=%d requests=%d", creates, len(g.requests))
			}
			requested := g.requests[0].Target.Access
			if mode == "omitted" && requested != nil || mode != "omitted" && (requested == nil || *requested != 0) {
				t.Fatalf("authored access not forwarded: %v", requested)
			}
			if mode == "private" || mode == "omitted" {
				if r.Status != "completed" || r.Output["nodeId"] != json.Number("900") || len(r.Trace[0].Receipts) != 1 || r.Trace[0].Receipts[0].NodeID != 900 {
					t.Fatalf("confirmed write lost receipt: %+v", r)
				}
			} else if r.Status != "failed" || r.Output != nil || len(r.Trace[0].Receipts) != 0 || r.Trace[0].Repair == nil || r.Trace[0].Repair.NodeID != 900 || r.Trace[0].Repair.Confirmed {
				t.Fatalf("unconfirmed allocation issued receipt or lost repair ID: %+v", r)
			}
		})
	}
}
