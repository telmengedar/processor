package process_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/process"
)

func (g *graph) SetProcessTextContent(ctx context.Context, id int64, body string) error {
	return g.SetProcessContent(ctx, id, body)
}

func v2Blueprint(t *testing.T) process.Blueprint {
	t.Helper()
	b, err := process.GangolfV2("knowledge")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func draftBody(b *process.Blueprint, symbol string) map[string]any {
	for i := range b.Nodes {
		if b.Nodes[i].Symbol == symbol {
			return b.Nodes[i].Body.(map[string]any)
		}
	}
	panic(symbol)
}
func binding(source, path string) map[string]any {
	return map[string]any{"source": source, "path": path}
}
func symbolic(symbol string) map[string]any { return map[string]any{"nodeRef": symbol} }

func TestV2RequiredRenderedContextBoundBeforeProvider(t *testing.T) {
	t.Parallel()
	b := v2Blueprint(t)
	for i := range b.Nodes {
		if b.Nodes[i].Symbol == "assessment_template" {
			b.Nodes[i].Body = strings.Repeat("{{context.message}}", 400)
		}
	}
	g, _ := authored(t, b)
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		return completion.Result{Text: "reply"}, nil
	})
	r := (&process.Engine{Graph: g, Model: m}).Run(context.Background(), process.Request{Process: "Gangolf Chat v2", Arguments: map[string]any{"message": strings.Repeat("x", 100), "persona": 90}})
	if r.Error == nil || r.Error.Code != "context_bound" || calls != 0 {
		t.Fatalf("oversized rendered context reached provider: error=%v calls=%d", r.Error, calls)
	}
}

func TestGangolfV2EndToEndWithPrivateFixture(t *testing.T) {
	t.Parallel()
	b := v2Blueprint(t)
	g, p := authored(t, b)
	calls := 0
	if g.nodes[p.Members["assessment_template"]].Content[0] == '"' || process.Hash(g.nodes[p.Members["assessment_template"]].Content) != "ba1387eecc8511eefe1ff51d04856d8cca11f8994e1b372ae0f73f1a7354c786" {
		t.Fatal("raw approved Markdown did not round-trip")
	}
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		raw, _ := json.Marshal(r.Messages)
		text := string(raw)
		if calls == 1 {
			if len(r.Tools) != 2 || !strings.Contains(text, "memory-sentinel") || strings.Contains(text, "persona-sentinel") || !strings.Contains(r.Messages[0].Content, "# Recalled material") {
				t.Fatalf("assessment scope wrong: %s", text)
			}
			return completion.Result{Text: "public-brief-sentinel"}, nil
		}
		if len(r.Tools) != 0 || strings.Contains(text, "memory-sentinel") || !strings.Contains(text, "public-brief-sentinel") || !strings.Contains(text, "persona-sentinel") || !strings.Contains(r.Messages[0].Content, "# Persona") {
			t.Fatalf("response scope wrong: %s", text)
		}
		return completion.Result{Text: "final-v2-reply"}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"hello","persona":90}}`)
	if status != 200 || r.Status != "completed" || r.Output["reply"] != "final-v2-reply" || calls != 2 || g.recalls != 1 {
		t.Fatalf("%d %+v calls=%d", status, r, calls)
	}
	keys := []string{}
	for _, visit := range r.Trace {
		keys = append(keys, visit.Key)
		if visit.TransportHash == "" {
			t.Fatal("transport trace hash missing")
		}
	}
	if !reflect.DeepEqual(keys, []string{"recall_context", "assess_situation", "load_persona", "compose_response", "reply"}) || r.Trace[1].TemplateHash == "" || r.Trace[3].TemplateHash == r.Trace[1].TemplateHash {
		t.Fatalf("wrong graph trace: %+v", r.Trace)
	}
	calls = 0
	status, r = request(t, g, m, fmt.Sprintf(`{"processId":%d,"arguments":{"message":"hi","persona":90}}`, p.ProcessID))
	if status != 200 || r.Status != "completed" {
		t.Fatalf("ID invocation failed: %+v", r)
	}
}

func TestV2TemplateMutationAndInsertedPlaceholderDataRemainGraphOwned(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"\nfirst-template-marker\n", "\nchanged-template-marker\n"} {
		b := v2Blueprint(t)
		for i := range b.Nodes {
			if b.Nodes[i].Symbol == "assessment_template" {
				b.Nodes[i].Body = b.Nodes[i].Body.(string) + suffix
			}
		}
		g, _ := authored(t, b)
		calls := 0
		m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
			calls++
			if calls == 1 {
				if !strings.Contains(r.Messages[0].Content, suffix) || !strings.Contains(r.Messages[0].Content, "{{context.persona}}") || strings.Contains(r.Messages[0].Content, "persona-sentinel") {
					t.Fatal("template changed or inserted text recursively rendered")
				}
			}
			return completion.Result{Text: "reply"}, nil
		})
		status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"{{context.persona}}","persona":90}}`)
		if status != 200 {
			t.Fatalf("%d %+v", status, r)
		}
	}
}

func TestV2BadTemplateReferencesFailBeforeModelEffects(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing", "reversed", "wrong_type", "duplicate", "unknown_placeholder", "malformed", "wrong_source"} {
		t.Run(mode, func(t *testing.T) {
			b := v2Blueprint(t)
			switch mode {
			case "missing":
				for i, e := range b.Edges {
					if e.From == "assessment_template" && e.To == "assess_situation" {
						b.Edges = append(b.Edges[:i], b.Edges[i+1:]...)
						break
					}
				}
			case "reversed":
				for i, e := range b.Edges {
					if e.From == "assessment_template" && e.To == "assess_situation" {
						b.Edges[i].From, b.Edges[i].To = e.To, e.From
						break
					}
				}
			case "wrong_type":
				for i, e := range b.Edges {
					if e.From == "assessment_template" && e.To == "assess_situation" {
						b.Edges[i].From = "dispatch"
						break
					}
				}
			case "duplicate":
				b.Nodes = append(b.Nodes, process.DraftNode{Symbol: "other_template", Type: "process-prompt-template", Name: "other", Body: "{{context.message}}"})
				b.Edges = append(b.Edges, process.DraftEdge{From: "other_template", To: "assess_situation", Role: "prompt"})
			case "unknown_placeholder", "malformed":
				for i := range b.Nodes {
					if b.Nodes[i].Symbol == "assessment_template" {
						if mode == "malformed" {
							b.Nodes[i].Body = "{{context.message"
						} else {
							b.Nodes[i].Body = "{{context.unknown}}"
						}
					}
				}
			case "wrong_source":
				draftBody(&b, "assess_situation")["exports"] = map[string]any{"historical": binding("state", "recall_context")}
			}
			g := newGraph()
			_, err := process.Import(context.Background(), g, b)
			if err == nil || len(g.nodes) != 1 || g.recalls != 0 {
				t.Fatalf("invalid graph published: %v", err)
			}
		})
	}
}

func TestV2ChangedTemplateOrReferenceInvalidatesSnapshot(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"template", "reference"} {
		t.Run(mode, func(t *testing.T) {
			g, p := authored(t, v2Blueprint(t))
			if mode == "template" {
				id := p.Members["assessment_template"]
				n := g.nodes[id]
				n.Content += "changed"
				g.nodes[id] = n
			} else {
				for i := range g.edges {
					if g.edges[i].Source == p.Members["assessment_template"] {
						g.edges = append(g.edges[:i], g.edges[i+1:]...)
						break
					}
				}
			}
			status, r := request(t, g, nil, `{"process":"Gangolf Chat v2","arguments":{"message":"hello","persona":90}}`)
			if status == 200 || r.Error == nil || g.recalls != 0 || len(r.Trace) != 0 {
				t.Fatalf("changed publication executed: %d %+v", status, r)
			}
		})
	}
}

func TestV2TransportDropsPriorPayloadAndRejectsUnexportedRead(t *testing.T) {
	t.Parallel()
	b := v2Blueprint(t)
	body := draftBody(&b, "load_persona")
	body["exports"].(map[string]any)["assessment"] = binding("transport", "recalled")
	g, _ := authored(t, b)
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		return completion.Result{Text: `{"exports":{"recalled":"model-forged-retention"},"receipts":[{"nodeId":123}]}`}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"hi","persona":90}}`)
	if status == 200 || r.Error.Code != "invalid_binding" || calls != 1 || len(r.Trace) != 3 {
		t.Fatalf("historical read or model retention worked: %d %+v calls=%d", status, r, calls)
	}
}

func TestV2TransportBoundStopsHiddenOutgoingPayload(t *testing.T) {
	t.Parallel()
	b := v2Blueprint(t)
	draftBody(&b, "assess_situation")["exports"] = map[string]any{"large": strings.Repeat("x", 40000)}
	g, _ := authored(t, b)
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		return completion.Result{Text: "brief"}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"hi","persona":90}}`)
	if status == 200 || r.Error.Code != "transport_bound" || calls != 1 || len(r.Trace) != 2 {
		t.Fatalf("large bus accepted: %d %+v", status, r)
	}
}

func TestV2NativeRoundRequiresExportedProvenance(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"explicit", "omitted", "forged", "consumed", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			b := v2Blueprint(t)
			if mode == "omitted" || mode == "forged" {
				decision := map[string]any{"kind": binding("result", "kind"), "toolId": binding("result", "toolId"), "args": binding("result", "args")}
				if mode == "forged" {
					decision["capability"] = "model-cannot-forge-this"
				}
				draftBody(&b, "assess_situation")["exports"] = map[string]any{"decision": decision}
			}
			if mode == "consumed" {
				draftBody(&b, "assess_again")["exports"].(map[string]any)["stale"] = binding("transport", "round")
			}
			g, _ := authored(t, b)
			g.nodes[91] = loop.Anchor{ID: 91, Type: "thought", Content: "looked-up-sentinel"}
			calls := 0
			m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
				calls++
				if calls == 1 {
					id := "native-call-sentinel"
					if mode == "oversized" {
						id = strings.Repeat("x", 40000)
					}
					return completion.Result{Calls: []completion.Call{{ID: id, Name: "read_node", Arguments: json.RawMessage(`{"id":91}`)}}}, nil
				}
				if calls == 2 {
					raw, _ := json.Marshal(r.Messages)
					if len(r.Messages) != 4 || r.Messages[3].CallID != "native-call-sentinel" || !strings.Contains(string(raw), "looked-up-sentinel") || strings.Contains(string(raw), "memory-sentinel") {
						t.Fatalf("wrong scoped round: %s", raw)
					}
				} else {
					raw, _ := json.Marshal(r.Messages)
					if strings.Contains(string(raw), "native-call-sentinel") {
						t.Fatal("consumed call leaked into later stage")
					}
				}
				return completion.Result{Text: "reply"}, nil
			})
			status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"hi","persona":90}}`)
			if mode == "explicit" {
				if status != 200 || calls != 3 {
					t.Fatalf("explicit loop failed: %d %+v", status, r)
				}
			} else if mode == "oversized" {
				if status == 200 || r.Error.Code != "transport_bound" || calls != 1 {
					t.Fatal("private handle hid oversized call metadata")
				}
			} else if mode == "consumed" {
				if status == 200 || r.Error.Code != "capability_revoked" || calls != 2 {
					t.Fatalf("consumed handle survived: %+v calls=%d", r, calls)
				}
			} else if status == 200 || calls != 1 {
				t.Fatalf("invalid handle executed: %d %+v calls=%d", status, r, calls)
			}
		})
	}
}

func TestV2FailureNeverBecomesLLMRecovery(t *testing.T) {
	t.Parallel()
	g, p := authored(t, v2Blueprint(t))
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		return completion.Result{}, fmt.Errorf("provider failure")
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"hi","persona":90}}`)
	if status == 200 || r.Error.Code != "model_unavailable" || calls != 1 {
		t.Fatalf("failure recovery: %+v calls=%d", r, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = (&process.Engine{Graph: g, Model: m}).Run(ctx, process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"message": "hi", "persona": 90}})
	if r.Error.Code != "process_cancelled" || calls != 1 || len(r.Trace) != 0 {
		t.Fatal("cancelled run made a recovery call")
	}
}

func TestV2GraphTemplatesReachActualProviderWireWithoutRuntimeInstructions(t *testing.T) {
	t.Parallel()
	g, _ := authored(t, v2Blueprint(t))
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Model    string `json:"model"`
			Think    bool   `json:"think"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []json.RawMessage `json:"tools"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || r.URL.Path != "/api/chat" || body.Model != "gemma4:latest" || body.Think || len(body.Messages) != 2 {
			t.Fatal("provider wire changed")
		}
		if calls == 1 {
			if len(body.Tools) != 2 || !strings.HasPrefix(body.Messages[0].Content, "# Task\nAssess the message") || strings.Contains(body.Messages[0].Content, "persona-sentinel") {
				t.Fatal("assessment template/wire wrong")
			}
			_, _ = w.Write([]byte(`{"message":{"content":"wire public brief"},"done_reason":"stop"}`))
		} else {
			if len(body.Tools) != 0 || !strings.Contains(body.Messages[0].Content, "wire public brief") || !strings.Contains(body.Messages[0].Content, "persona-sentinel") || strings.Contains(body.Messages[0].Content, "memory-sentinel") {
				t.Fatal("response template/wire wrong")
			}
			_, _ = w.Write([]byte(`{"message":{"content":"wire final reply"},"done_reason":"stop"}`))
		}
	}))
	defer srv.Close()
	status, response := request(t, g, ollama.NewClient(srv.URL, "gemma4:latest", "", loop.Sampling{}, srv.Client()), `{"process":"Gangolf Chat v2","arguments":{"message":"hi","persona":90}}`)
	if status != 200 || response.Output["reply"] != "wire final reply" || calls != 2 {
		t.Fatalf("wire pipeline failed: %d %+v", status, response)
	}
}

func TestV2TemplateEscapesAndMissingFieldsOrInvalidUnicode(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"escapes", "missing", "invalid_template", "invalid_value"} {
		t.Run(mode, func(t *testing.T) {
			b := v2Blueprint(t)
			for i := range b.Nodes {
				if b.Nodes[i].Symbol == "assessment_template" {
					switch mode {
					case "escapes":
						b.Nodes[i].Body = "{{{{literal}}}} {{context.message}}"
					case "missing":
						b.Nodes[i].Body = "{{context.message.absent}}"
					case "invalid_template":
						b.Nodes[i].Body = string([]byte{255})
					case "invalid_value":
						b.Nodes[i].Body = "{{context.message}}"
					}
				}
			}
			g := newGraph()
			p, err := process.Import(context.Background(), g, b)
			if mode == "invalid_template" {
				if err == nil || len(g.nodes) != 1 {
					t.Fatal("invalid Unicode normalized during import")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
				calls++
				if calls == 1 && r.Messages[0].Content != "{{literal}} hi" {
					t.Fatal("escape rendering changed")
				}
				return completion.Result{Text: "reply"}, nil
			})
			message := "hi"
			if mode == "invalid_value" {
				message = string([]byte{255})
			}
			response := (&process.Engine{Graph: g, Model: m}).Run(context.Background(), process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"message": message, "persona": 90}})
			if mode == "escapes" {
				if response.Status != "completed" || calls != 2 {
					t.Fatal(response.Error)
				}
			} else if response.Status == "completed" || calls != 0 || response.Error.Code != "invalid_template" {
				t.Fatalf("bad template value reached model: %+v calls=%d", response, calls)
			}
		})
	}
}

func TestV2LoopsStopAtTheirFirstBindingExecutionCap(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"action", "judgement"} {
		t.Run(kind, func(t *testing.T) {
			b := process.Blueprint{Version: 2, Name: "bound fixture", Entry: "a", Inputs: map[string]process.Declaration{"message": {Type: "string"}}, Outputs: map[string]process.Declaration{"reply": {Type: "string"}}}
			source := "template"
			role := "prompt"
			if kind == "action" {
				source = "select"
				role = "tool"
				b.Nodes = append(b.Nodes, process.DraftNode{Symbol: source, Name: source, Type: "process-tool", Body: process.Tool{Version: 2, Name: "select", Description: "Select current data.", Builtin: "select", Schema: process.Schema{Type: "object", AdditionalProperties: true}}})
			} else {
				b.Nodes = append(b.Nodes, process.DraftNode{Symbol: source, Name: source, Type: "process-prompt-template", Body: "{{context.input}}"})
			}
			for _, key := range []string{"a", "b", "c"} {
				b.Nodes = append(b.Nodes, process.DraftNode{Symbol: key, Name: key, Type: "process-step", Body: process.Step{Version: 2, Key: key, Kind: kind, Input: map[string]any{}}})
				b.Edges = append(b.Edges, process.DraftEdge{From: source, To: key, Role: role})
			}
			b.Edges = append(b.Edges, process.DraftEdge{From: "a", To: "b", Default: true}, process.DraftEdge{From: "b", To: "c", Default: true}, process.DraftEdge{From: "c", To: "a", Default: true})
			g, p := authored(t, b)
			calls := 0
			m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
				calls++
				return completion.Result{Text: "reply"}, nil
			})
			response := (&process.Engine{Graph: g, Model: m}).Run(context.Background(), process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"message": "hi"}})
			if kind == "action" {
				if response.Error.Code != "dispatch_limit" || calls != 0 || len(response.Trace) != 17 {
					t.Fatalf("dispatch bound: %+v", response)
				}
			} else if response.Error.Code != "judgement_limit" || calls != 8 || len(response.Trace) != 9 {
				t.Fatalf("judgement bound: %+v calls=%d", response, calls)
			}
		})
	}
}
