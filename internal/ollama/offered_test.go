package ollama

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const nativeWriteResponse = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"write_file","arguments":{"path":"a.txt","content":"x"}}}]},"done":true,"done_reason":"stop"}`

func TestEveryJudgementCallCarriesExactlyTheSystemAndOneUserMessage(t *testing.T) {
	t.Parallel()

	for name, offered := range map[string][]string{"offering every tool": allTools, "offering nothing": nil} {
		srv, captured := capturingServer(t, doneResponse)
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

		in := loop.JudgeInput{System: "the system text", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: offered}
		if _, err := c.Judge(context.Background(), in); err != nil {
			t.Fatalf("%s: Judge: %v", name, err)
		}

		var got struct {
			Messages []struct {
				Role      string          `json:"role"`
				Content   string          `json:"content"`
				ToolCalls json.RawMessage `json:"tool_calls"`
				ToolName  string          `json:"tool_name"`
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
		if len(got.Messages[1].ToolCalls) != 0 || got.Messages[1].ToolName != "" {
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
		srv, captured := capturingServer(t, doneResponse)
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

	srv, _ := capturingServer(t, nativeWriteResponse)
	withheldWrite, err := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()).Judge(context.Background(),
		loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: retrievalOnly})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if withheldWrite.UnofferedToolCalls != 1 {
		t.Fatalf("UnofferedToolCalls = %d, want 1: a write call on a request that did not declare the write tool is a fact about the response", withheldWrite.UnofferedToolCalls)
	}
	if withheldWrite.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want %q taken from the done reason: a call to a tool the request never offered must not be obeyed", withheldWrite.Reason, loop.Answered)
	}
	if withheldWrite.WritePath != "" || withheldWrite.WriteContent != "" {
		t.Fatalf("the unoffered call's arguments were read: path %q, %d bytes", withheldWrite.WritePath, len(withheldWrite.WriteContent))
	}

	srv, _ = capturingServer(t, nativeWriteResponse)
	offeredWrite := judgeOnce(t, NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()))
	if offeredWrite.Reason != loop.WantsWrite || offeredWrite.UnofferedToolCalls != 0 {
		t.Fatalf("with the write tool offered the same response gave Reason %q and %d unoffered calls, want %q and none: the fixture must be a call this adapter honours, or the arm above proves nothing", offeredWrite.Reason, offeredWrite.UnofferedToolCalls, loop.WantsWrite)
	}
}

func TestACallShapedLikeAToolCallInTheTextIsNotRecoveredIntoAToolTheRequestDidNotOffer(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, responseWithContent(t, capturedUnparsedWrite))
	result, err := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()).Judge(context.Background(),
		loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: []string{loop.ToolRecall, loop.ToolReadNode}})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if result.Reason == loop.WantsWrite || result.WritePath != "" || result.WriteContent != "" {
		t.Fatalf("a write call in the response text was recovered on a request that did not offer the write tool: %+v", result)
	}
	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want %q: markup naming a tool the request never offered is neither an answer nor a request", result.Reason, loop.Unrecognised)
	}
}

const nativeRecallAndWriteResponse = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"recall","arguments":{"query":"q"}}},{"function":{"name":"write_file","arguments":{"path":"a.txt","content":"x"}}}]},"done":true,"done_reason":"stop"}`

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
		srv, _ := capturingServer(t, nativeRecallAndWriteResponse)
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

func TestATextAnswerOfAToolLessCallThatLooksLikeCallMarkupIsAnsweredAndNeverRecovered(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, responseWithContent(t, capturedUnparsedWrite))
	result, err := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()).Judge(context.Background(),
		loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if result.Reason != loop.Answered || result.RawReason != "stop" || result.ToolSource != "" {
		t.Fatalf("a tool-less call whose text is call markup gave Reason %q, raw %q, tool source %q, want %q, %q and none: recovery runs only where a tool was offered, and the record's terminal must say the call answered", result.Reason, result.RawReason, result.ToolSource, loop.Answered, "stop")
	}
	if result.Answer != capturedUnparsedWrite {
		t.Fatalf("the answer was rewritten to %q, want the response text as it arrived", result.Answer)
	}
}
