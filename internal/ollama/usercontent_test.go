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

		instantRFC3339 = "2026-03-04T05:06:07+02:00"
	)

	instant := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("test+02", 2*60*60))

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{System: "the system text", Block: block, Input: input, Now: instant, MaxOutputTokens: judgeBudget, Offered: allTools}
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
	if want := loop.RenderUserContent(block, input, instant, "", "", ""); got.Messages[1].Content != want {
		t.Fatalf("messages[1].Content = %q, want %q byte-exact", got.Messages[1].Content, want)
	}
	if !strings.Contains(got.Messages[1].Content, instantRFC3339) {
		t.Fatalf("messages[1].Content states no %q, so the instant the loop supplied never reached the wire; content=%q",
			instantRFC3339, got.Messages[1].Content)
	}
}

func TestTheUserMessageCarriesTheConversationSection(t *testing.T) {
	t.Parallel()

	const (
		block        = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		conversation = "===== EARLIER IN THIS CONVERSATION =====\nuser said: he is climbing again\nyou replied: bold of him"
		want         = "===== INPUT =====\nso he drops\n\n===== NOW =====\n2026-03-04T05:06:07+02:00\n\n\n" + block + "\n\n" + conversation + "\n\n===== INPUT =====\nso he drops"
	)

	instant := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("test+02", 2*60*60))

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{System: "the system text", Block: block, Input: "so he drops", Conversation: conversation, Now: instant, MaxOutputTokens: judgeBudget, Offered: allTools}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages has %d entries, want 2 (system, user): the conversation is part of the one user message", len(got.Messages))
	}
	if got.Messages[1].Content != want {
		t.Fatalf("messages[1].Content = %q, want %q", got.Messages[1].Content, want)
	}
}

func TestTheUserMessageEndsWithTheTrailerTheLoopSupplied(t *testing.T) {
	t.Parallel()

	const (
		block   = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		trailer = "the closing paragraph"
	)

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{System: "the system text", Block: block, Input: "so he drops", Trailer: trailer, MaxOutputTokens: judgeBudget, Offered: allTools}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if want := loop.RenderUserContent(block, "so he drops", time.Time{}, "", "", trailer); got.Messages[1].Content != want {
		t.Fatalf("messages[1].Content = %q, want %q", got.Messages[1].Content, want)
	}
	if !strings.HasSuffix(got.Messages[1].Content, "\n\n"+trailer) {
		t.Fatalf("messages[1].Content does not end with the trailer: %q", got.Messages[1].Content)
	}
}

func TestTheRecallClauseOfAJudgeInputReachesTheUserMessageAheadOfTheHistoryAndTheTrailerAfterTheRequest(t *testing.T) {
	t.Parallel()

	const (
		block        = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		conversation = "===== EARLIER IN THIS CONVERSATION =====\nuser said: hi"
		clause       = "the distinctive clause paragraph"
		trailer      = "the distinctive closing paragraph"
	)

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{System: "the system text", Block: block, Input: "so he drops", Conversation: conversation, Clause: clause, Trailer: trailer, MaxOutputTokens: judgeBudget, Offered: allTools}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	content := got.Messages[1].Content
	if want := loop.RenderUserContent(block, "so he drops", time.Time{}, conversation, clause, trailer); content != want {
		t.Fatalf("messages[1].Content = %q, want %q", content, want)
	}
	if strings.Count(content, clause) != 1 || strings.Index(content, clause) > strings.Index(content, conversation) {
		t.Errorf("the clause is not carried once ahead of the conversation section: %q", content)
	}
	if !strings.HasSuffix(content, "\n\n"+trailer) {
		t.Errorf("the user message does not end with the trailer: %q", content)
	}
}
