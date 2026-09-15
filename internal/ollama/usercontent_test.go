package ollama

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

func TestJudgeSendsANativeUserMessageByteEqualToRenderUserContentOfTheSameBlockAndInput(t *testing.T) {
	t.Parallel()

	const (
		block = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		input = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast\n"

		instantUTC = "2026-03-04T03:06:07Z"
	)

	instant := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("test+02", 2*60*60))

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{System: "the system text", Block: block, Input: input, Now: instant}
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
	if len(got.Messages) != 2 {
		t.Fatalf("messages = %d, want 2 (system, user)", len(got.Messages))
	}
	if got.Messages[1].Role != "user" {
		t.Fatalf("messages[1].Role = %q, want %q", got.Messages[1].Role, "user")
	}
	if want := loop.RenderUserContent(block, input, instant); got.Messages[1].Content != want {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got.Messages[1].Content, want)
	}
	if !strings.Contains(got.Messages[1].Content, instantUTC) {
		t.Fatalf("messages[1].Content states no %q, so the instant the loop supplied never reached the wire; content=%q",
			instantUTC, got.Messages[1].Content)
	}
}

func TestJudgeSendsANativeToolResultByteEqualToRenderToolResultOfTheSameExchange(t *testing.T) {
	t.Parallel()

	exchange := loop.ToolExchange{
		Tool:  loop.ToolRecall,
		Query: "what is missing",
		Results: []loop.Candidate{
			{ID: 5, Type: "task", Name: "Found", Content: "found body"},
			{ID: 9, Type: "documentation", Name: "Also Found", Content: "second found body"},
		},
	}

	srv, captured := capturingServer(t, doneResponse)
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
		t.Fatalf("messages = %d, want 4 (system, user, assistant, tool)", len(got.Messages))
	}
	if got.Messages[3].Role != "tool" {
		t.Fatalf("messages[3].Role = %q, want %q", got.Messages[3].Role, "tool")
	}
	if want := loop.RenderToolResult(exchange); got.Messages[3].Content != want {
		t.Fatalf("messages[3].Content = %q, want %q byte-exact", got.Messages[3].Content, want)
	}
}
