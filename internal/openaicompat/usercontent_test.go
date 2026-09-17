package openaicompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

func TestJudgeSendsAUserMessageByteEqualToRenderUserContentOfTheSameBlockAndInput(t *testing.T) {
	t.Parallel()

	const (
		block = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		input = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast\n"

		instantRFC3339 = "2026-03-04T05:06:07+02:00"
	)

	instant := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("test+02", 2*60*60))

	srv, captured := capturingServer(t, stopResponse)
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
		t.Fatalf("messages has %d entries, want 2 (system, user)", len(got.Messages))
	}
	if got.Messages[1].Role != "user" {
		t.Fatalf("messages[1].Role = %q, want %q", got.Messages[1].Role, "user")
	}
	if want := loop.RenderUserContent(block, input, instant, loop.UpdateWindow{}); got.Messages[1].Content != want {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got.Messages[1].Content, want)
	}
	if !strings.Contains(got.Messages[1].Content, instantRFC3339) {
		t.Fatalf("messages[1].Content states no %q, so the instant the loop supplied never reached the wire; content=%q",
			instantRFC3339, got.Messages[1].Content)
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

func TestJudgeCarriesTheBlockNudgeAndEveryToolResultNudgeInOneRequestWhenRecallKeepsComingUpEmpty(t *testing.T) {
	t.Parallel()

	anchor := loop.Anchor{ID: 1, Type: "t", Name: "solo", Content: "anchor body"}
	block, _ := loop.Assemble(anchor, nil, 60_000, 0)

	emptyRecall := loop.ToolExchange{Tool: loop.ToolRecall, Query: "still nothing"}

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{
		System:     "the system text",
		Block:      block,
		Input:      "in",
		PriorTools: []loop.ToolExchange{emptyRecall, emptyRecall, emptyRecall},
	}
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
	if len(got.Messages) != 8 {
		t.Fatalf("messages has %d entries, want 8 (system, user, 3x(assistant, tool))", len(got.Messages))
	}
	if !strings.Contains(got.Messages[1].Content, block) {
		t.Fatalf("messages[1].Content = %q, want it to embed the block byte-exact: %q", got.Messages[1].Content, block)
	}

	wantToolContent := loop.RenderToolResult(emptyRecall)
	toolMessages := 0
	for _, m := range got.Messages {
		if m.Role != "tool" {
			continue
		}
		toolMessages++
		if m.Content != wantToolContent {
			t.Fatalf("tool message content = %q, want %q byte-exact", m.Content, wantToolContent)
		}
	}
	if toolMessages != 3 {
		t.Fatalf("got %d tool messages, want 3 - one per empty recall, co-present with the block's own nudge in the same request", toolMessages)
	}
}
