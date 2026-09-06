package openaicompat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestJudgeDecodesAWriteToolCallAsWantsWriteWithThePathAndContent(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"site/index.html\",\"content\":\"<h1>hi</h1>\"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want WantsWrite", result.Reason)
	}
	if result.WritePath != "site/index.html" {
		t.Fatalf("WritePath = %q, want %q", result.WritePath, "site/index.html")
	}
	if result.WriteContent != "<h1>hi</h1>" {
		t.Fatalf("WriteContent = %q, want %q", result.WriteContent, "<h1>hi</h1>")
	}
	if result.RecallQuery != "" {
		t.Fatalf("RecallQuery = %q, want empty — this call was for the file tool", result.RecallQuery)
	}
	if result.ToolError != "" {
		t.Fatalf("ToolError = %q, want empty for a well-formed tool call", result.ToolError)
	}
}

func TestJudgeFlagsUnparseableArgumentsOnAWriteToolCallWithoutInventingAPath(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"write_file","arguments":"not json"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want WantsWrite", result.Reason)
	}
	if result.ToolError == "" {
		t.Fatal("ToolError is empty, want the parse failure surfaced")
	}
	if result.WritePath != "" || result.WriteContent != "" {
		t.Fatalf("WritePath = %q and WriteContent = %q, want both empty when the arguments did not parse", result.WritePath, result.WriteContent)
	}
}

func TestJudgeAcceptsAWriteToolCallWithEmptyContentRatherThanCallingItMalformed(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"empty.txt\",\"content\":\"\"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.WantsWrite || result.ToolError != "" {
		t.Fatalf("Reason = %q and ToolError = %q, want an accepted write of an empty file", result.Reason, result.ToolError)
	}
	if result.WritePath != "empty.txt" {
		t.Fatalf("WritePath = %q, want %q", result.WritePath, "empty.txt")
	}
}

func TestJudgeTreatsACallToAToolThatWasNeverOfferedAsUnrecognised(t *testing.T) {
	t.Parallel()

	resp := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"run_shell","arguments":"{\"cmd\":\"rm -rf /\"}"}}]},"finish_reason":"tool_calls"}]}`
	srv, _ := capturingServer(t, resp)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want Unrecognised — the loop dispatches nothing for a tool it never offered", result.Reason)
	}
	if result.RecallQuery != "" || result.WritePath != "" || result.WriteContent != "" {
		t.Fatalf("result = %+v, want no tool arguments carried off an unoffered tool", result)
	}
	if result.RawReason != "tool_calls" {
		t.Fatalf("RawReason = %q, want the endpoint's raw value preserved", result.RawReason)
	}
}

func TestJudgeReplaysAPriorWriteRoundAsTheWriteToolAndItsReceipt(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{
		System: "sys", Block: "block", Input: "in",
		PriorTools: []loop.ToolExchange{
			{Tool: loop.ToolWriteFile, Path: "index.html", Content: "<h1>hi</h1>", Bytes: 11},
		},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	messages := decodeReplayedMessages(t, captured.Body)
	if len(messages) != 4 {
		t.Fatalf("messages has %d entries, want 4 (system, user, assistant, tool)", len(messages))
	}

	assistant, tool := messages[2], messages[3]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("messages[2] = %+v, want an assistant message with one tool call", assistant)
	}
	if assistant.ToolCalls[0].Function.Name != "write_file" {
		t.Fatalf("messages[2] tool call function = %q, want %q", assistant.ToolCalls[0].Function.Name, "write_file")
	}

	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(assistant.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("decode replayed arguments: %v; raw=%s", err, assistant.ToolCalls[0].Function.Arguments)
	}
	if args.Path != "index.html" || args.Content != "<h1>hi</h1>" {
		t.Fatalf("replayed arguments = %+v, want the path and content the model originally sent", args)
	}

	if tool.Role != "tool" || tool.ToolCallID != assistant.ToolCalls[0].ID {
		t.Fatalf("messages[3] = %+v, want a tool message whose tool_call_id matches messages[2]'s call id", tool)
	}
	if !contains(tool.Content, "11") || !contains(tool.Content, "index.html") {
		t.Fatalf("messages[3].Content = %q, want the byte count and the path reported back", tool.Content)
	}
}

func TestJudgeReplaysARefusedWriteRoundAsTheRefusalTheModelMustSee(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	const refusal = "write rejected: path must not leave the working directory"
	in := loop.JudgeInput{
		System: "sys", Block: "block", Input: "in",
		PriorTools: []loop.ToolExchange{
			{Tool: loop.ToolWriteFile, Path: "../escape.html", Content: "x", Error: refusal},
		},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	messages := decodeReplayedMessages(t, captured.Body)
	if len(messages) != 4 {
		t.Fatalf("messages has %d entries, want 4", len(messages))
	}
	if !contains(messages[3].Content, refusal) {
		t.Fatalf("messages[3].Content = %q, want the refusal %q shown to the model", messages[3].Content, refusal)
	}
	if contains(messages[3].Content, "wrote ") {
		t.Fatalf("messages[3].Content = %q, want no write receipt on a refused round", messages[3].Content)
	}
}

type replayedMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
	ToolCalls  []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func decodeReplayedMessages(t *testing.T, body []byte) []replayedMessage {
	t.Helper()

	var got struct {
		Messages []replayedMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, body)
	}
	return got.Messages
}
