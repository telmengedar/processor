package openaicompat

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const nativeWriteOnStop = `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"a.txt\",\"content\":\"x\"}"}}]},"finish_reason":"stop"}]}`

func TestEveryJudgementCallCarriesExactlyTheSystemAndOneUserMessage(t *testing.T) {
	t.Parallel()

	for name, offered := range map[string][]string{"offering every tool": allTools, "offering nothing": nil} {
		srv, captured := capturingServer(t, stopResponse)
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

		in := loop.JudgeInput{System: "the system text", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: offered}
		if _, err := c.Judge(context.Background(), in); err != nil {
			t.Fatalf("%s: Judge: %v", name, err)
		}

		var got struct {
			Messages []struct {
				Role       string          `json:"role"`
				Content    string          `json:"content"`
				ToolCalls  json.RawMessage `json:"tool_calls"`
				ToolCallID string          `json:"tool_call_id"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(captured.Body, &got); err != nil {
			t.Fatalf("%s: decode request body: %v; body=%s", name, err, captured.Body)
		}

		if len(got.Messages) != 2 {
			t.Fatalf("%s: the request carries %d messages, want exactly the system message and one user message: a tool-call or tool-result message is the transcript the model imitates on a call that offers no tool; body=%s", name, len(got.Messages), captured.Body)
		}
		if got.Messages[0].Role != "system" || got.Messages[0].Content != "the system text" {
			t.Fatalf("%s: messages[0] = %+v, want the system message carrying the text the loop composed", name, got.Messages[0])
		}
		if want := "===== INPUT =====\nin\n\n\nblock\n===== INPUT =====\nin"; got.Messages[1].Role != "user" || got.Messages[1].Content != want {
			t.Fatalf("%s: messages[1] = %+v, want the user message %q", name, got.Messages[1], want)
		}
		if len(got.Messages[1].ToolCalls) != 0 || got.Messages[1].ToolCallID != "" {
			t.Fatalf("%s: the user message carries tool fields: %+v", name, got.Messages[1])
		}
	}
}

func TestTheToolsDeclaredOnACallAreExactlyTheToolsItsInputOffers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		offered []string
		want    []string
	}{
		{"nothing offered", nil, nil},
		{"recall alone", []string{loop.ToolRecall}, []string{"recall"}},
		{"read alone", []string{loop.ToolReadNode}, []string{"read_node"}},
		{"write alone", []string{loop.ToolWriteFile}, []string{"write_file"}},
		{"retrieval without write", []string{loop.ToolRecall, loop.ToolReadNode}, []string{"read_node", "recall"}},
		{"every tool", allTools, []string{"read_node", "recall", "write_file"}},
	}

	for _, tc := range cases {
		srv, captured := capturingServer(t, stopResponse)
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

		in := loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: tc.offered}
		if _, err := c.Judge(context.Background(), in); err != nil {
			t.Fatalf("%s: Judge: %v", tc.name, err)
		}

		var got chatRequest
		if err := json.Unmarshal(captured.Body, &got); err != nil {
			t.Fatalf("%s: decode request: %v; body=%s", tc.name, err, captured.Body)
		}

		var declared []string
		for _, tool := range got.Tools {
			declared = append(declared, tool.Function.Name)
		}
		slices.Sort(declared)

		if !slices.Equal(declared, tc.want) {
			t.Errorf("%s: the request declares %v, want %v: the declaration is the request's half of what the call offers, and the system text is derived from the other half", tc.name, declared, tc.want)
		}
	}
}

func TestACallToAToolTheRequestDidNotOfferIsCountedUnoffered(t *testing.T) {
	t.Parallel()

	retrievalOnly := []string{loop.ToolRecall, loop.ToolReadNode}

	srv, _ := capturingServer(t, nativeWriteOnStop)
	withheldWrite, err := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()).Judge(context.Background(),
		loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: retrievalOnly})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if withheldWrite.UnofferedToolCalls != 1 {
		t.Fatalf("UnofferedToolCalls = %d, want 1: a write call on a request that did not declare the write tool is a fact about the response", withheldWrite.UnofferedToolCalls)
	}
	if withheldWrite.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want %q taken from the finish reason: a call to a tool the request never offered must not be obeyed", withheldWrite.Reason, loop.Answered)
	}
	if withheldWrite.WritePath != "" || withheldWrite.WriteContent != "" {
		t.Fatalf("the unoffered call's arguments were read: path %q, %d bytes", withheldWrite.WritePath, len(withheldWrite.WriteContent))
	}

	srv, _ = capturingServer(t, nativeWriteOnStop)
	offeredWrite := judgeOnce(t, NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()))
	if offeredWrite.Reason != loop.WantsWrite || offeredWrite.UnofferedToolCalls != 0 {
		t.Fatalf("with the write tool offered the same response gave Reason %q and %d unoffered calls, want %q and none: the fixture must be a call this adapter honours, or the arm above proves nothing", offeredWrite.Reason, offeredWrite.UnofferedToolCalls, loop.WantsWrite)
	}
}

const nativeRecallAndWriteOnStop = `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"q\"}"}},{"id":"call-2","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"a.txt\",\"content\":\"x\"}"}}]},"finish_reason":"stop"}]}`

func TestEveryReturnedCallOutsideTheOfferedSetIsCountedNotOnlyTheFirst(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		offered    []string
		wantCount  int
		wantReason loop.TerminalReason
	}{
		{"retrieval offered, the second call is the unoffered one", []string{loop.ToolRecall, loop.ToolReadNode}, 1, loop.WantsRecall},
		{"nothing offered, both calls are unoffered", nil, 2, loop.Answered},
	} {
		srv, _ := capturingServer(t, nativeRecallAndWriteOnStop)
		result, err := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()).Judge(context.Background(),
			loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: tc.offered})
		if err != nil {
			t.Fatalf("%s: Judge: %v", tc.name, err)
		}

		if result.UnofferedToolCalls != tc.wantCount {
			t.Errorf("%s: UnofferedToolCalls = %d, want %d", tc.name, result.UnofferedToolCalls, tc.wantCount)
		}
		if result.Reason != tc.wantReason {
			t.Errorf("%s: Reason = %q, want %q", tc.name, result.Reason, tc.wantReason)
		}
	}
}
