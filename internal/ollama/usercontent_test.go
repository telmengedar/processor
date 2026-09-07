package ollama

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestJudgeNativeUserMessageComesFromTheSharedRendererSoTheProvidersCannotDrift(t *testing.T) {
	t.Parallel()

	const (
		block = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
		input = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast\n"
	)

	srv, captured := capturingServer(t, doneResponse)
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
		t.Fatalf("messages = %d, want 2 (system, user)", len(got.Messages))
	}
	if want := loop.RenderUserContent(block, input); got.Messages[1].Content != want {
		t.Fatalf("user message = %q, want %q byte-exact", got.Messages[1].Content, want)
	}
}
