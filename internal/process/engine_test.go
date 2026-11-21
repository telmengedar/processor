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
	"github.com/telmengedar/processor/internal/process"
	"github.com/telmengedar/processor/internal/server"
)

type graph struct {
	nodes        map[int64]loop.Anchor
	edges        []process.Edge
	recalls      int
	publications []int64
}

func newGraph() *graph {
	return &graph{nodes: map[int64]loop.Anchor{90: {ID: 90, Type: "knowledge", Name: "Gangolf", Content: "persona-sentinel"}}}
}
func (g *graph) Node(_ context.Context, id int64, _ loop.Filter) (loop.Anchor, bool, error) {
	n, ok := g.nodes[id]
	return n, ok, nil
}
func (g *graph) Recall(_ context.Context, r loop.RecallRequest) ([]loop.Candidate, error) {
	g.recalls++
	return []loop.Candidate{{ID: 91, Type: "thought", Content: "memory-sentinel", Similarity: .93}}, nil
}
func (g *graph) FindNodes(_ context.Context, name, t string) ([]loop.Anchor, error) {
	r := []loop.Anchor{}
	for _, n := range g.nodes {
		if n.Name == name && n.Type == t {
			r = append(r, n)
		}
	}
	return r, nil
}
func (g *graph) Links(_ context.Context, id int64) ([]process.Edge, error) {
	r := []process.Edge{}
	for _, e := range g.edges {
		if e.Source == id || e.Target == id {
			r = append(r, e)
		}
	}
	return r, nil
}
func (g *graph) CreateProcessNode(_ context.Context, t, n string) (int64, error) {
	id := int64(len(g.nodes) + 100)
	g.nodes[id] = loop.Anchor{ID: id, Type: t, Name: n}
	return id, nil
}
func (g *graph) SetProcessContent(_ context.Context, id int64, body string) error {
	n := g.nodes[id]
	n.Content = body
	g.nodes[id] = n
	g.publications = append(g.publications, id)
	return nil
}
func (g *graph) LinkProcessNodes(_ context.Context, e process.Edge) error {
	g.edges = append(g.edges, e)
	return nil
}

type modelFunc func(context.Context, completion.Request) (completion.Result, error)

func (f modelFunc) Complete(ctx context.Context, r completion.Request) (completion.Result, error) {
	return f(ctx, r)
}

func authored(t *testing.T, b process.Blueprint) (*graph, process.Publication) {
	t.Helper()
	g := newGraph()
	p, err := process.Import(context.Background(), g, b)
	if err != nil {
		t.Fatal(err)
	}
	if g.publications[len(g.publications)-1] != p.ProcessID {
		t.Fatal("root was not published last")
	}
	return g, p
}

func request(t *testing.T, g *graph, m completion.Model, body string) (int, process.Response) {
	t.Helper()
	h := server.NewHandler(&loop.Turn{}, &process.Engine{Graph: g, Model: m})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/process-runs", strings.NewReader(body)))
	var result process.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, result
}

func TestGangolfChatProcessReturnsReplyFromMessageAndPersona(t *testing.T) {
	t.Parallel()
	g, _ := authored(t, process.GangolfChat("knowledge"))
	calls := 0
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		wire, _ := json.Marshal(r.Messages)
		text := string(wire)
		if !strings.Contains(text, "message-sentinel") {
			t.Fatal("required message absent")
		}
		if calls == 1 {
			if len(r.Tools) != 2 || !strings.Contains(text, "memory-sentinel") || strings.Contains(text, "persona-sentinel") {
				t.Fatalf("assessment scope wrong: %s", text)
			}
			return completion.Result{Text: "public-assessment", FinishReason: "stop"}, nil
		}
		if len(r.Tools) != 0 || !strings.Contains(text, "persona-sentinel") || !strings.Contains(text, "public-assessment") || strings.Contains(text, "memory-sentinel") {
			t.Fatalf("composition scope wrong: %s", text)
		}
		return completion.Result{Text: "actual-reply", FinishReason: "stop"}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"message-sentinel","persona":90}}`)
	if status != 200 || r.Status != "completed" || r.Output["reply"] != "actual-reply" || calls != 2 || g.recalls != 1 {
		t.Fatalf("status=%d result=%+v calls=%d recalls=%d", status, r, calls, g.recalls)
	}
	keys := []string{}
	for _, visit := range r.Trace {
		keys = append(keys, visit.Key)
		if visit.Hash == "" || visit.InputHash == "" {
			t.Fatal("trace lacks hashes")
		}
	}
	if !reflect.DeepEqual(keys, []string{"load_message", "load_persona", "recall_context", "assess_situation", "compose_response", "reply"}) {
		t.Fatal(keys)
	}
}

func TestSelectedToolLoopPreservesOriginAndLatestResult(t *testing.T) {
	t.Parallel()
	g, _ := authored(t, process.GangolfChat("knowledge"))
	calls := 0
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		switch calls {
		case 1:
			return completion.Result{Calls: []completion.Call{{ID: "call-distinct", Name: "read_node", Arguments: json.RawMessage(`{"id":90}`)}}, FinishReason: "tool_calls"}, nil
		case 2:
			if len(r.Messages) != 4 || r.Messages[2].Calls[0].ID != "call-distinct" || r.Messages[3].Role != "tool" || r.Messages[3].CallID != "call-distinct" || !strings.Contains(r.Messages[3].Content, "persona-sentinel") {
				t.Fatalf("latest tool round lost: %+v", r.Messages)
			}
			return completion.Result{Text: "public brief", FinishReason: "stop"}, nil
		default:
			return completion.Result{Text: "reply", FinishReason: "stop"}, nil
		}
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hello","persona":90}}`)
	if status != 200 || calls != 3 || len(r.Trace) != 9 || r.Trace[4].Key != "execute_tool" || r.Trace[5].Key != "refresh_context" {
		t.Fatalf("status=%d %+v", status, r)
	}
}

func TestOrdinaryContextDoesNotImplicitlyRestoreToolCallArguments(t *testing.T) {
	t.Parallel()
	b := process.GangolfChat("knowledge")
	for _, node := range b.Nodes {
		if node.Symbol == "assess_situation" {
			items := node.Body.(map[string]any)["context"].([]any)
			delete(items[1].(map[string]any), "toolResult")
		}
	}
	g, _ := authored(t, b)
	calls := 0
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		if calls == 1 {
			return completion.Result{Calls: []completion.Call{{ID: "hidden-call-sentinel", Name: "read_node", Arguments: json.RawMessage(`{"id":90}`)}}}, nil
		}
		raw, _ := json.Marshal(r.Messages)
		if len(r.Messages) != 2 || strings.Contains(string(raw), "hidden-call-sentinel") {
			t.Fatal("ordinary context restored hidden call metadata")
		}
		return completion.Result{Text: "reply"}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hi","persona":90}}`)
	if status != 200 || calls != 3 {
		t.Fatalf("%d %+v", status, r)
	}
}

func TestUnofferedAndMultipleToolsNeverExecute(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"not_offered", "multiple"} {
		t.Run(name, func(t *testing.T) {
			g, _ := authored(t, process.GangolfChat("knowledge"))
			m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
				calls := []completion.Call{{ID: "x", Name: "unknown", Arguments: json.RawMessage(`{}`)}}
				if name == "multiple" {
					calls = append(calls, calls[0])
				}
				return completion.Result{Calls: calls}, nil
			})
			status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hello","persona":90}}`)
			if status == 200 || r.Error == nil || len(r.Trace) != 4 || r.Trace[3].Key != "assess_situation" || g.recalls != 1 {
				t.Fatalf("unexpected dispatch status=%d %+v", status, r)
			}
		})
	}
}

func TestDefinitionHashOrTransitionChangeStopsBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"member", "transition"} {
		t.Run(change, func(t *testing.T) {
			g, p := authored(t, process.GangolfChat("knowledge"))
			if change == "member" {
				id := p.Members["assess_situation"]
				n := g.nodes[id]
				n.Content += " "
				g.nodes[id] = n
			} else {
				g.edges[0].Context = `{"kind":"process-transition","when":{"path":"ok","equals":true}}`
			}
			m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
				t.Fatal("model executed for changed definition")
				return completion.Result{}, nil
			})
			status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hello","persona":90}}`)
			if status != 400 || r.Error == nil || r.Error.Code != "definition_changed" || len(r.Trace) != 0 || g.recalls != 0 {
				t.Fatalf("%d %+v", status, r)
			}
		})
	}
}

func TestRunningSnapshotIgnoresLaterDefinitionChanges(t *testing.T) {
	t.Parallel()
	g, p := authored(t, process.GangolfChat("knowledge"))
	calls := 0
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		if calls == 1 {
			n := g.nodes[p.Members["compose_response"]]
			n.Content = `broken`
			g.nodes[n.ID] = n
			g.edges = nil
			return completion.Result{Text: "brief"}, nil
		}
		return completion.Result{Text: "pinned reply"}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hello","persona":90}}`)
	if status != 200 || r.Output["reply"] != "pinned reply" || calls != 2 {
		t.Fatalf("%d %+v", status, r)
	}
}

func TestInvalidArgumentsAndAmbiguousNamesStopBeforeTools(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"process":"Gangolf Chat","arguments":{"message":"","persona":90}}`, `{"process":"Gangolf Chat","arguments":{"message":"hi","persona":"90"}}`, `{"process":"Gangolf Chat","arguments":{"message":"hi","persona":91}}`, `{"process":"Gangolf Chat","arguments":{"message":{},"persona":90}}`, `{"processId":1,"process":"Gangolf Chat","arguments":{}}`} {
		g, _ := authored(t, process.GangolfChat("knowledge"))
		status, r := request(t, g, nil, body)
		if status != 400 || r.Error == nil || g.recalls != 0 {
			t.Fatalf("%d %+v", status, r)
		}
	}
	g, _ := authored(t, process.GangolfChat("knowledge"))
	g.nodes[500] = loop.Anchor{ID: 500, Type: "process", Name: "Gangolf Chat"}
	status, r := request(t, g, nil, `{"process":"Gangolf Chat","arguments":{"message":"hi","persona":90}}`)
	if status != 400 || r.Error.Code != "process_not_unique" {
		t.Fatalf("%d %+v", status, r)
	}
}

func TestGraphEdgesChangeExecutionWithoutEngineChanges(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"first", "second"} {
		t.Run(target, func(t *testing.T) {
			b := process.PythonExample()
			b.Nodes = []process.DraftNode{{Symbol: "select", Type: "process-tool", Name: "select", Body: process.Tool{Version: 1, Name: "select", Description: "select", Schema: process.Schema{Type: "object", AdditionalProperties: true}, Builtin: "select"}}, {Symbol: "start", Type: "process-step", Name: "start", Body: map[string]any{"version": 1, "key": "start", "kind": "action", "tool": map[string]any{"nodeRef": "select"}, "input": map[string]any{"route": target}}}, {Symbol: "first", Type: "process-step", Name: "first", Body: process.Step{Version: 1, Key: "first", Kind: "return", Output: map[string]any{"reply": "first"}}}, {Symbol: "second", Type: "process-step", Name: "second", Body: process.Step{Version: 1, Key: "second", Kind: "return", Output: map[string]any{"reply": "second"}}}}
			b.Entry = "start"
			b.Edges = []process.DraftEdge{{From: "start", To: "first", When: &process.Predicate{Path: "route", Equals: "first"}}, {From: "start", To: "second", When: &process.Predicate{Path: "route", Equals: "second"}}}
			g, p := authored(t, b)
			r := (&process.Engine{Graph: g}).Run(context.Background(), process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"text": "irrelevant"}})
			if r.Status != "completed" || r.Output["reply"] != target {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestConditionalLoopStopsAtDispatchLimit(t *testing.T) {
	t.Parallel()
	g, _ := authored(t, process.GangolfChat("knowledge"))
	calls := 0
	m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
		calls++
		return completion.Result{Calls: []completion.Call{{ID: fmt.Sprint(calls), Name: "read_node", Arguments: json.RawMessage(`{"id":90}`)}}}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hello","persona":90}}`)
	if status == 200 || r.Error.Code != "dispatch_limit" || calls != 5 {
		t.Fatalf("%d %+v calls%d", status, r, calls)
	}
}

func TestInvalidImportDoesNotAllocateNodes(t *testing.T) {
	t.Parallel()
	for _, alter := range []func(*process.Blueprint){func(b *process.Blueprint) {
		b.Edges = append(b.Edges, process.DraftEdge{From: "load_persona", To: "load_message", Default: true})
	}, func(b *process.Blueprint) {
		b.Nodes[0].Body = map[string]any{"version": 1, "name": "bad", "description": "bad", "schema": map[string]any{"type": "object", "oneOf": []any{}}, "builtin": "select"}
	}, func(b *process.Blueprint) { b.Edges[0].To = "missing" }} {
		g := newGraph()
		b := process.GangolfChat("knowledge")
		alter(&b)
		_, err := process.Import(context.Background(), g, b)
		if err == nil || len(g.nodes) != 1 {
			t.Fatalf("err=%v nodes=%d", err, len(g.nodes))
		}
	}
}
