package completion_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

func TestBothProvidersSendDynamicStepToolsAndPairedResults(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"ollama", "openai"} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]json.RawMessage
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Fatal("invalid request JSON")
				}
				var tools []struct {
					Type     string          `json:"type"`
					Function completion.Tool `json:"function"`
				}
				_ = json.Unmarshal(body["tools"], &tools)
				if len(tools) != 1 || tools[0].Function.Name != "custom_graph_tool" || tools[0].Function.Description != "distinct description" || !strings.Contains(string(tools[0].Function.Parameters), "payload") {
					t.Fatalf("dynamic tools missing: %s", body["tools"])
				}
				var messages []map[string]json.RawMessage
				_ = json.Unmarshal(body["messages"], &messages)
				if len(messages) != 4 || string(messages[0]["content"]) != `"exact-step-prompt"` || string(messages[3]["role"]) != `"tool"` || string(messages[3]["tool_call_id"]) != `"prior-call-id"` || !strings.Contains(string(messages[2]["tool_calls"]), "custom_graph_tool") {
					t.Fatalf("tool round missing: %s", body["messages"])
				}
				if protocol == "ollama" {
					if r.URL.Path != "/api/chat" || string(body["think"]) != "false" || string(messages[3]["tool_name"]) != `"custom_graph_tool"` {
						t.Fatal("native protocol differs")
					}
					_, _ = w.Write([]byte(`{"message":{"content":"native-text","tool_calls":[{"function":{"name":"custom_graph_tool","arguments":{"payload":"observed"}}}]},"done_reason":"stop","prompt_eval_count":17,"eval_count":9}`))
				} else {
					if r.URL.Path != "/chat/completions" || string(body["reasoning_effort"]) != `"none"` {
						t.Fatal("compatible protocol differs")
					}
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"compatible-text","tool_calls":[{"id":"new-call-id","type":"function","function":{"name":"custom_graph_tool","arguments":"{\"payload\":\"observed\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":17,"completion_tokens":9}}`))
				}
			}))
			defer srv.Close()
			var model completion.Model
			if protocol == "ollama" {
				model = ollama.NewClient(srv.URL, "gemma4:latest", "", loop.Sampling{}, srv.Client())
			} else {
				model = openaicompat.NewClient(srv.URL, "gemma4:latest", "", loop.Sampling{}, srv.Client())
			}
			prior := completion.Call{ID: "prior-call-id", Name: "custom_graph_tool", Arguments: json.RawMessage(`{"payload":"earlier"}`)}
			result, err := model.Complete(context.Background(), completion.Request{Messages: []completion.Message{{Role: "system", Content: "exact-step-prompt"}, {Role: "user", Content: "only-this-input"}, {Role: "assistant", Calls: []completion.Call{prior}}, {Role: "tool", CallID: prior.ID, ToolName: prior.Name, Content: `{"ok":true}`}}, Tools: []completion.Tool{{Name: "custom_graph_tool", Description: "distinct description", Parameters: json.RawMessage(`{"type":"object","properties":{"payload":{"type":"string"}},"required":["payload"],"additionalProperties":false}`)}}})
			if err != nil || calls != 1 || len(result.Calls) != 1 || result.Calls[0].Name != "custom_graph_tool" || !strings.Contains(string(result.Calls[0].Arguments), "observed") || result.Provider.Adapter == "" || result.Usage == nil {
				t.Fatalf("completion result=%+v err=%v", result, err)
			}
		})
	}
}

func TestGenericProviderFailuresAreReturnedWithoutResponsePayload(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("private-response-sentinel"))
	}))
	defer srv.Close()
	for _, model := range []completion.Model{ollama.NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client()), openaicompat.NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())} {
		_, err := model.Complete(context.Background(), completion.Request{})
		if err == nil || strings.Contains(err.Error(), "private-response-sentinel") {
			t.Fatalf("error=%v", err)
		}
	}
}
