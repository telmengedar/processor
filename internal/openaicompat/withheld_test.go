package openaicompat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const nativeRecallOnStop = `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"recall","arguments":"{\"query\":\"what changed\"}"}}]},"finish_reason":"stop"}]}`

const callShapedText = "<function=write_file>\n<parameter=path>\nindex.html\n</parameter>\n<parameter=content>\nhello\n</parameter>\n</function>"

func responseWithContent(t *testing.T, content string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message":       map[string]any{"content": content},
			"finish_reason": "stop",
		}},
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return string(body)
}

func judgeWithheld(t *testing.T, c *Client) loop.JudgeResult {
	t.Helper()
	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, WithholdTools: true})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	return result
}

func judgeBoth(t *testing.T, response string) (offered, withheld loop.JudgeResult) {
	t.Helper()

	offeredServer, _ := capturingServer(t, response)
	offered = judgeOnce(t, NewClient(offeredServer.URL, "model-x", "", loop.Sampling{}, offeredServer.Client()))

	withheldServer, _ := capturingServer(t, response)
	withheld = judgeWithheld(t, NewClient(withheldServer.URL, "model-x", "", loop.Sampling{}, withheldServer.Client()))

	return offered, withheld
}

func TestAWithheldToolListLeavesTheToolsKeyOutOfTheRequestEntirely(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeWithheld(t, c)

	if _, present := topLevelKeys(t, captured.Body)["tools"]; present {
		t.Fatalf("the request carries a tools key on a call the loop would not dispatch a tool from; the withholding is the whole mechanism, and it happens at the wire:\n%s", captured.Body)
	}
}

func TestACallStillWillingToDispatchCarriesBothToolsAsItAlwaysHas(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	var got chatRequest
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request: %v; body=%s", err, captured.Body)
	}
	if len(got.Tools) != 2 {
		t.Fatalf("tools = %d, want 2: a call the loop would still dispatch a tool from must carry the tool list it always carried", len(got.Tools))
	}
}

func TestACallShapedLikeAToolCallInTheTextOfAToolLessCallIsNeverRecoveredIntoOne(t *testing.T) {
	t.Parallel()

	_, withheld := judgeBoth(t, responseWithContent(t, callShapedText))

	if withheld.Reason == loop.WantsWrite || withheld.Reason == loop.WantsRecall {
		t.Fatalf("a call issued with no tool list returned the tool-wanting terminal %q; recovering a call from the text of such a call hands the operator a blob of markup where the empty answer used to be, which is the failure the remedy itself would introduce", withheld.Reason)
	}
	if withheld.WritePath != "" || withheld.WriteContent != "" {
		t.Fatalf("a call issued with no tool list returned path %q and %d bytes of content, want neither: the text is the answer, not a request", withheld.WritePath, len(withheld.WriteContent))
	}
	if withheld.Answer != callShapedText {
		t.Fatalf("the answer of a tool-less call was rewritten to %q, want the response text as it arrived", withheld.Answer)
	}
}

func TestANativeToolCallOnAToolLessCallIsRecordedAsAFactAndNeverHonoured(t *testing.T) {
	t.Parallel()

	offered, withheld := judgeBoth(t, nativeRecallOnStop)

	if offered.Reason != loop.WantsRecall || offered.RecallQuery != "what changed" {
		t.Fatalf("with the tools offered the same response gave Reason %q and query %q, want a recall for %q", offered.Reason, offered.RecallQuery, "what changed")
	}
	if offered.UnofferedToolCalls != 0 {
		t.Fatalf("UnofferedToolCalls = %d on a call that offered the tool, want 0", offered.UnofferedToolCalls)
	}

	if withheld.Reason != loop.Answered {
		t.Fatalf("a call issued with no tool list returned Reason %q, want %q taken from the finish reason: an endpoint that answers a tool list it was never sent must not be obeyed", withheld.Reason, loop.Answered)
	}
	if withheld.RecallQuery != "" {
		t.Fatalf("a call issued with no tool list returned query %q, want none", withheld.RecallQuery)
	}
	if withheld.UnofferedToolCalls != 1 {
		t.Fatalf("UnofferedToolCalls = %d, want 1: the payload is recorded as a fact about the response even though nothing acts on it", withheld.UnofferedToolCalls)
	}
}
