package openaicompat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestJudgeSendsAUserMessageByteEqualToRenderUserContentOfTheSameBlockAndInput(t *testing.T) {
	t.Parallel()

	const (
		block = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		input = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast\n"
	)

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "the system text", Block: block, Input: input}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages has %d entries, want 2 (system, user)", len(got.Messages))
	}
	if got.Messages[1].Role != "user" {
		t.Fatalf("messages[1].Role = %q, want %q", got.Messages[1].Role, "user")
	}
	if want := loop.RenderUserContent(block, input); got.Messages[1].Content != want {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got.Messages[1].Content, want)
	}
}

func TestJudgeSendsAToolResultByteEqualToRenderToolResultOfTheSameExchange(t *testing.T) {
	t.Parallel()

	exchange := loop.ToolExchange{
		Tool:  loop.ToolRecall,
		Query: "what is missing",
		Results: []loop.Candidate{
			{ID: 5, Type: "task", Name: "Found", Content: "found body"},
			{ID: 9, Type: "documentation", Name: "Also Found", Content: "second found body"},
		},
	}

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{System: "the system text", Block: "block", Input: "in", PriorTools: []loop.ToolExchange{exchange}}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if len(got.Messages) != 4 {
		t.Fatalf("messages has %d entries, want 4 (system, user, assistant, tool)", len(got.Messages))
	}
	if got.Messages[3].Role != "tool" {
		t.Fatalf("messages[3].Role = %q, want %q", got.Messages[3].Role, "tool")
	}
	if want := loop.RenderToolResult(exchange); got.Messages[3].Content != want {
		t.Fatalf("messages[3].Content = %q, want %q byte-exact", got.Messages[3].Content, want)
	}
}
