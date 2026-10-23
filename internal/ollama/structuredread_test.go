package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/telmengedar/processor/internal/loop"
	"testing"
)

func TestStructuredReadWireSchemaAndNativeCallsAreKeptOutOfRecovery(t *testing.T) {
	t.Parallel()
	for _, native := range []bool{false, true} {
		decision := `{"action":"answer","text":"read_node(71)","query":null,"id":null}`
		message := map[string]any{"content": decision}
		if native {
			message["tool_calls"] = []any{map[string]any{"function": map[string]any{"name": "read_node", "arguments": nil}}}
		}
		raw, _ := json.Marshal(map[string]any{"message": message, "done_reason": "stop"})
		srv, captured := capturingServer(t, string(raw))
		client := NewClient(srv.URL, "model", "", loop.Sampling{}, srv.Client())
		input := loop.JudgeInput{System: "verbatim system", Input: "request", Offered: []string{loop.ToolReadNode}, StructuredAllowedActions: []string{"answer", "read"}}
		result, err := client.Judge(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		keys := topLevelKeys(t, captured.Body)
		if _, ok := keys["format"]; !ok {
			t.Fatal("schema missing")
		}
		if _, ok := keys["tools"]; ok {
			t.Fatal("structured request sent native tools")
		}
		var schema json.RawMessage
		schema = keys["format"]
		if !bytes.Equal(schema, loop.StructuredReadSchema(input.StructuredAllowedActions)) {
			t.Fatalf("wrong restricted schema: %s", schema)
		}
		if result.Answer != decision || result.ToolSource != "" || result.ReadNodeID != 0 {
			t.Fatalf("adapter recovered structured content: %+v", result)
		}
		if (result.UnofferedToolCalls == 1) != native {
			t.Fatalf("unexpected native call lost: %+v", result)
		}
		input.StructuredAllowedActions = nil
		if _, err := client.Judge(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		keys = topLevelKeys(t, captured.Body)
		if _, ok := keys["tools"]; !ok {
			t.Fatal("native control did not actually offer tools")
		}
		if _, ok := keys["format"]; ok {
			t.Fatal("native default gained schema")
		}
	}
}
