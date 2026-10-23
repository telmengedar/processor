package ollama

import (
	"context"
	"encoding/json"
	"github.com/telmengedar/processor/internal/loop"
	"reflect"
	"testing"
)

func TestPrivateNoteWireSchemaPreservesRawDecisionAndRejectableNativeEvidence(t *testing.T) {
	t.Parallel()
	for _, native := range []bool{false, true} {
		decision := `{"action":"write","text":"read_node(71)","about":null}`
		message := map[string]any{"content": decision}
		if native {
			message["tool_calls"] = []any{map[string]any{"function": map[string]any{"name": "read_node", "arguments": nil}}}
		}
		raw, _ := json.Marshal(map[string]any{"message": message, "done_reason": "stop"})
		srv, captured := capturingServer(t, string(raw))
		client := NewClient(srv.URL, "model", "", loop.Sampling{}, srv.Client())
		input := loop.JudgeInput{System: "verbatim system", Input: "request", Offered: []string{loop.ToolReadNode}, NoteDecisionOutput: true}
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
		var actual, required any
		if err := json.Unmarshal(schema, &actual); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(`{"type":"object","properties":{"action":{"type":"string","enum":["write","skip"]},"text":{"type":"string"},"about":{"type":["integer","null"]}},"required":["action","text","about"],"additionalProperties":false}`), &required); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, required) {
			t.Fatalf("provider schema does not require the private decision contract: %s", schema)
		}
		if result.Answer != decision || result.ToolSource != "" || result.ReadNodeID != 0 {
			t.Fatalf("adapter recovered structured content: %+v", result)
		}
		if (result.UnofferedToolCalls == 1) != native {
			t.Fatalf("unexpected native call lost: %+v", result)
		}
		input.NoteDecisionOutput = false
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
