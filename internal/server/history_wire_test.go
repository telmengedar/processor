package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
	"github.com/telmengedar/processor/internal/systemtext"
)

type historyWireGraph struct {
	stubGraph
	readIDs []int64
	recalls int
}

func (g *historyWireGraph) Node(_ context.Context, id int64, _ loop.Filter) (loop.Anchor, bool, error) {
	g.readIDs = append(g.readIDs, id)
	return loop.Anchor{ID: id, Type: "knowledge", Name: "frozen", Content: "frozen context"}, true, nil
}

func (g *historyWireGraph) Recall(context.Context, loop.RecallRequest) ([]loop.Candidate, error) {
	g.recalls++
	return nil, nil
}

func (*historyWireGraph) WriteRun(context.Context, loop.Record) loop.WriteReceipt {
	panic("read-only history probe attempted run filing")
}

func TestAnswerHistoryRolesSurviveSelectionReadAndRecallAtFakeProviders(t *testing.T) {
	t.Parallel()
	for _, adapter := range []string{"ollama", "openaicompat"} {
		for _, mode := range []loop.ResponseMode{"", loop.ResponseStructuredRead} {
			t.Run(adapter+"/"+string(mode), func(t *testing.T) {
				var requests [][]byte
				responses := []string{"DATES: none\nMossTap ownership", `{"action":"read","text":"","query":null,"id":71}`, `{"action":"recall","text":"","query":"MossTap correction","id":null}`, `{"action":"answer","text":"That was my claim.","query":null,"id":null}`}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					index := len(requests)
					requests = append(requests, body)
					if index >= len(responses) {
						t.Error("unexpected extra model request")
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					content := responses[index]
					message := map[string]any{"content": content}
					if mode == "" && index > 0 {
						message["content"] = "That was my claim."
						if index < 3 {
							name, arguments := "read_node", map[string]any{"id": 71}
							if index == 2 {
								name, arguments = "recall", map[string]any{"query": "MossTap correction"}
							}
							var wireArguments any = arguments
							if adapter == "openaicompat" {
								encoded, _ := json.Marshal(arguments)
								wireArguments = string(encoded)
							}
							message["content"] = ""
							message["tool_calls"] = []any{map[string]any{"id": "fake-call", "type": "function", "function": map[string]any{"name": name, "arguments": wireArguments}}}
						}
					}
					w.Header().Set("Content-Type", "application/json")
					if adapter == "ollama" {
						_ = json.NewEncoder(w).Encode(map[string]any{"message": message, "done_reason": "stop"})
					} else {
						reason := "stop"
						if mode == "" && index > 0 && index < 3 {
							reason = "tool_calls"
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": reason}}})
					}
				}))
				t.Cleanup(server.Close)
				var model loop.ModelPort
				if adapter == "ollama" {
					model = ollama.NewClient(server.URL, "gemma4:latest", "", loop.Sampling{}, server.Client())
				} else {
					model = openaicompat.NewClient(server.URL, "gemma4:latest", "", loop.Sampling{}, server.Client())
				}
				graph := &historyWireGraph{}
				turn := loop.NewTurn(graph, model, nil, systemtext.ComposeForMode, "gemma4:latest", testLogger())
				history := []loop.Exchange{{Input: "viewer: MossTap is an ordinary chat emote", Reply: "You keep treating MossTap like a secret code"}, {Input: "viewer: That secret-code claim was yours; I called it ordinary."}}
				settings := loop.DefaultSettings()
				settings.HistoryExchangeLimit = 2
				record, _, err := turn.Answer(context.Background(), loop.RunOptions{Input: "viewer: Again: whose claim was the secret-code part?", Subject: 42, History: append([]loop.Exchange{{Input: "CUT OLDEST"}, {Input: "CUT ALSO"}}, history...), Settings: settings, View: &loop.View{Identity: "frozen identity", Form: loop.FormContent, ResponseMode: mode}})
				if err != nil {
					t.Fatal(err)
				}
				if record.Answer != "That was my claim." || record.ModelCalls != 3 || record.HistoryCut != 2 || !slices.Equal(record.History, history) || len(requests) != 4 || !slices.Equal(graph.readIDs, []int64{42, 71}) || graph.recalls != 2 {
					t.Fatalf("record=%+v requests=%d readIDs=%v recalls=%d", record, len(requests), graph.readIDs, graph.recalls)
				}
				for index, raw := range requests[1:] {
					var request struct {
						Messages []struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						} `json:"messages"`
					}
					if err := json.Unmarshal(raw, &request); err != nil {
						t.Fatal(err)
					}
					var roles []string
					for _, message := range request.Messages {
						roles = append(roles, message.Role)
					}
					if !slices.Equal(roles, []string{"system", "user", "assistant", "user", "user"}) {
						t.Fatalf("call %d roles=%v", index, roles)
					}
					if request.Messages[1].Content != history[0].Input || request.Messages[2].Content != history[0].Reply || request.Messages[3].Content != history[1].Input {
						t.Fatalf("call %d history changed: %+v", index, request.Messages)
					}
					current := request.Messages[4].Content
					if !strings.Contains(current, "frozen context") || strings.Count(current, "===== INPUT =====") != 2 || strings.Contains(current, history[0].Input) || strings.Contains(current, history[0].Reply) || strings.Contains(current, history[1].Input) || strings.Contains(current, "EARLIER IN THIS CONVERSATION") || strings.Contains(string(raw), "CUT OLDEST") || strings.Contains(string(raw), "CUT ALSO") {
						t.Fatalf("call %d duplicates history or loses current context: %s", index, raw)
					}
					if !strings.HasPrefix(request.Messages[0].Content, "frozen identity\n\n") || !strings.Contains(request.Messages[0].Content, "Your earlier replies may be mistaken") {
						t.Fatalf("call %d system=%q", index, request.Messages[0].Content)
					}
				}
			})
		}
	}
}
