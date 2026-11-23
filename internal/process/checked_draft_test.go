package process_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/process"
)

func checkedBlueprint(t *testing.T) process.Blueprint {
	t.Helper()
	b := v2Blueprint(t)
	removed := []string{"select", "dispatch", "execute_tool", "refresh_context", "assess_again"}
	b.Nodes = slices.DeleteFunc(b.Nodes, func(n process.DraftNode) bool { return slices.Contains(removed, n.Symbol) })
	b.Edges = slices.DeleteFunc(b.Edges, func(e process.DraftEdge) bool {
		return slices.Contains(removed, e.From) || slices.Contains(removed, e.To) || e.Role == "tool" && e.To == "assess_situation"
	})
	return b
}

func TestCheckedDraftBranchesAndStageScopes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, first, second string
		repair, fail        bool
	}{
		{"direct", "ACCEPT", "", false, false},
		{"repair", "correct the unsupported claim", "ACCEPT", true, false},
		{"reject twice", "correct the unsupported claim", "still unsupported", true, true},
		{"first leading space", " ACCEPT", "ACCEPT", true, false},
		{"first trailing LF", "ACCEPT\n", "ACCEPT", true, false},
		{"first lowercase", "accept", "ACCEPT", true, false},
		{"second trailing LF", "correct the unsupported claim", "ACCEPT\n", true, true},
		{"second lowercase", "correct the unsupported claim", "accept", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := checkedBlueprint(t)
			g, p := authored(t, b)
			calls := 0
			assessment := "public-fact-sentinel; ACCEPT is data here"
			draft := "original-draft-sentinel; ACCEPT is data here"
			repaired := "revised-candidate-sentinel"
			m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
				calls++
				if len(r.Messages) != 2 || len(r.Tools) != 0 || r.Thinking {
					t.Fatalf("unexpected stage wire: %+v", r)
				}
				var data map[string]any
				if err := json.Unmarshal([]byte(r.Messages[1].Content), &data); err != nil {
					t.Fatal(err)
				}
				expected := map[string]any{"input": nil, "message": "message-sentinel"}
				var text string
				switch calls {
				case 1:
					return completion.Result{Text: assessment}, nil
				case 2:
					expected["assessment"], expected["persona"] = assessment, "persona-sentinel"
					text = draft
				case 3:
					expected["assessment"], expected["candidate"] = assessment, draft
					text = tc.first
				case 4:
					if !tc.repair {
						t.Fatal("direct accept reached repair")
					}
					expected["assessment"], expected["candidate"], expected["feedback"] = assessment, draft, tc.first
					text = repaired
				case 5:
					expected["assessment"], expected["candidate"] = assessment, repaired
					text = tc.second
				default:
					t.Fatal("extra judgement beyond bounded repair")
				}
				if !reflect.DeepEqual(data, expected) {
					t.Fatalf("stage %d context = %v, want %v", calls, data, expected)
				}
				if calls >= 3 {
					for _, forbidden := range []string{"persona-sentinel", "memory-sentinel"} {
						if strings.Contains(r.Messages[0].Content, forbidden) {
							t.Fatalf("stage %d admitted %s", calls, forbidden)
						}
					}
				}
				if calls == 5 && (strings.Contains(r.Messages[0].Content, "original-draft-sentinel") || strings.Contains(r.Messages[0].Content, "correct the unsupported claim")) {
					t.Fatal("recheck admitted old draft or critique")
				}
				return completion.Result{Text: text}, nil
			})
			status, r := request(t, g, m, `{"process":"Gangolf Chat v2","arguments":{"message":"message-sentinel","persona":90}}`)
			wantCalls := 3
			keys := []string{"recall_context", "assess_situation", "load_persona", "compose_response", "check_draft"}
			if tc.repair {
				wantCalls = 5
				keys = append(keys, "repair_response", "check_repair")
			}
			if tc.fail {
				if status == 200 || r.Status != "failed" || r.Error == nil || r.Error.Code != "invalid_transition" || len(r.Output) != 0 {
					t.Fatalf("final rejection returned reply: %d %+v", status, r)
				}
			} else {
				want, end := draft, "return_draft"
				if tc.repair {
					want, end = repaired, "return_repaired"
				}
				keys = append(keys, end)
				if status != 200 || r.Status != "completed" || !reflect.DeepEqual(r.Output, map[string]any{"reply": want}) {
					t.Fatalf("wrong accepted candidate: %d %+v", status, r)
				}
			}
			if calls != wantCalls || len(r.Trace) != len(keys) || g.recalls != 1 {
				t.Fatalf("wrong path size: calls=%d trace=%+v", calls, r.Trace)
			}
			for i, visit := range r.Trace {
				if visit.Key != keys[i] || visit.StepID != p.Members[keys[i]] {
					t.Fatalf("visit %d = %+v, want %s", i, visit, keys[i])
				}
				if i+1 < len(keys) && (visit.Edge == nil || visit.Edge.Target != p.Members[keys[i+1]]) {
					t.Fatalf("visit %s chose wrong edge: %+v", visit.Key, visit.Edge)
				}
				if visit.Kind == "judgement" {
					for _, admission := range visit.Context {
						if !admission.Included {
							t.Fatalf("required context not admitted: %+v", admission)
						}
					}
				}
			}
		})
	}
}

func TestCheckedDraftBoundAndCancellationHaveNoRecovery(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"context", "transport", "cancel", "binding"} {
		t.Run(mode, func(t *testing.T) {
			b := checkedBlueprint(t)
			if mode == "binding" {
				draftBody(&b, "check_draft")["context"].([]any)[1].(map[string]any)["value"] = binding("transport", "missing")
			}
			g, p := authored(t, b)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			m := modelFunc(func(context.Context, completion.Request) (completion.Result, error) {
				calls++
				text := "public-brief"
				if calls == 2 {
					text = "draft"
					if mode == "context" {
						text = strings.Repeat("d", 14000)
					}
					if mode == "cancel" {
						cancel()
					}
				}
				if calls == 3 {
					text = strings.Repeat("f", 40000)
				}
				if calls > 3 {
					t.Fatal("failed check triggered recovery model")
				}
				return completion.Result{Text: text}, nil
			})
			r := (&process.Engine{Graph: g, Model: m}).Run(ctx, process.Request{ProcessID: p.ProcessID, Arguments: map[string]any{"message": "hi", "persona": 90}})
			want, wantCalls := "context_bound", 2
			switch mode {
			case "transport":
				want, wantCalls = "transport_bound", 3
			case "cancel":
				want = "process_cancelled"
			case "binding":
				want = "invalid_binding"
			}
			if r.Status != "failed" || r.Error == nil || r.Error.Code != want || calls != wantCalls || len(r.Output) != 0 {
				t.Fatalf("unexpected bounded failure: %+v calls=%d", r, calls)
			}
		})
	}
}

func TestCheckedDraftTemplateReferencesFailBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing repair", "reversed checker", "wrong checker type"} {
		t.Run(mode, func(t *testing.T) {
			b := checkedBlueprint(t)
			switch mode {
			case "missing repair":
				b.Edges = slices.DeleteFunc(b.Edges, func(e process.DraftEdge) bool { return e.From == "repair_template" })
			case "reversed checker":
				for i := range b.Edges {
					if b.Edges[i].From == "checker_template" && b.Edges[i].To == "check_repair" {
						b.Edges[i].From, b.Edges[i].To = b.Edges[i].To, b.Edges[i].From
					}
				}
			case "wrong checker type":
				for i := range b.Nodes {
					if b.Nodes[i].Symbol == "checker_template" {
						b.Nodes[i].Type = "knowledge"
					}
				}
			}
			g := newGraph()
			if _, err := process.Import(context.Background(), g, b); err == nil || len(g.nodes) != 1 || g.recalls != 0 {
				t.Fatalf("invalid check/repair reference reached effects: err=%v nodes=%d recalls=%d", err, len(g.nodes), g.recalls)
			}
		})
	}
}
