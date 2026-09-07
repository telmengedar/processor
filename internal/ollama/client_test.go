package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

func capturingServer(t *testing.T, respBody string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Path = r.URL.Path
		captured.Auth = r.Header.Get("Authorization")
		captured.ContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		captured.Body = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

type capturedRequest struct {
	Path        string
	Auth        string
	ContentType string
	Body        []byte
}

const doneResponse = `{"message":{"role":"assistant","content":"the answer"},"done":true,"done_reason":"stop"}`

func judgeOnce(t *testing.T, c *Client) loop.JudgeResult {
	t.Helper()
	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	return result
}

func topLevelKeys(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, body)
	}
	return keys
}

func decodedOptions(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	keys := topLevelKeys(t, body)
	raw, present := keys["options"]
	if !present {
		t.Fatalf("request carries no options object; body=%s", body)
	}
	var options map[string]json.RawMessage
	if err := json.Unmarshal(raw, &options); err != nil {
		t.Fatalf("decode options: %v; options=%s", err, raw)
	}
	return options
}

func TestJudgePostsToTheNativeChatRoute(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	if captured.Path != "/api/chat" {
		t.Fatalf("path = %q, want %q", captured.Path, "/api/chat")
	}
}

func TestJudgeSendsStreamFalseExplicitlySoTheEndpointRepliesWithOneJSONObject(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	var got struct {
		Stream *bool `json:"stream"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}
	if got.Stream == nil {
		t.Fatalf("request omits stream; body=%s — omitted means the endpoint streams and the single-object decode fails", captured.Body)
	}
	if *got.Stream {
		t.Fatalf("stream = true, want false")
	}
}

func TestJudgeNativeRequestCarriesModelSystemBlockInputAndBothTools(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "the-model-id", "", loop.Sampling{}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "the system text", Block: "the block", Input: "the input"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}

	const (
		wantRecallName        = "recall"
		wantRecallDescription = "Search memory for something the assembled context did not include. Takes one argument: query, a short description of what is missing."
		wantWriteName         = "write_file"
		wantWriteDescription  = "Write a file into the working directory set aside for this request. Takes two arguments: path, a relative path naming the file, and content, the file's full text."
	)

	if got.Model != "the-model-id" {
		t.Fatalf("model = %q, want %q", got.Model, "the-model-id")
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != "system" || got.Messages[0].Content != "the system text" {
		t.Fatalf("message 0 = %+v, want the system text under role system", got.Messages[0])
	}
	if got.Messages[1].Role != "user" {
		t.Fatalf("message 1 role = %q, want %q", got.Messages[1].Role, "user")
	}
	if !strings.Contains(got.Messages[1].Content, "the block") || !strings.Contains(got.Messages[1].Content, "the input") {
		t.Fatalf("user message = %q, want it to carry both the block and the input", got.Messages[1].Content)
	}
	if len(got.Tools) != 2 {
		t.Fatalf("tools = %d, want the recall tool and the write tool", len(got.Tools))
	}
	if got.Tools[0].Function.Name != wantRecallName || got.Tools[0].Function.Description != wantRecallDescription {
		t.Fatalf("tool 0 = %q/%q, want %q/%q", got.Tools[0].Function.Name, got.Tools[0].Function.Description, wantRecallName, wantRecallDescription)
	}
	if got.Tools[1].Function.Name != wantWriteName || got.Tools[1].Function.Description != wantWriteDescription {
		t.Fatalf("tool 1 = %q/%q, want %q/%q", got.Tools[1].Function.Name, got.Tools[1].Function.Description, wantWriteName, wantWriteDescription)
	}
	for i, tool := range got.Tools {
		if tool.Type != "function" {
			t.Fatalf("tool %d type = %q, want %q", i, tool.Type, "function")
		}
		var schema struct {
			Type       string          `json:"type"`
			Properties json.RawMessage `json:"properties"`
			Required   []string        `json:"required"`
		}
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
			t.Fatalf("tool %d parameters are not a JSON schema object: %v; parameters=%s", i, err, tool.Function.Parameters)
		}
		if schema.Type != "object" {
			t.Fatalf("tool %d parameter schema type = %q, want %q", i, schema.Type, "object")
		}
	}
	if len(got.Tools[1].Function.Parameters) == 0 || len(got.Tools[0].Function.Parameters) == 0 {
		t.Fatal("a tool was sent with no parameter schema")
	}
}

func TestJudgeSendsSamplingInsideOptionsAndNotAtTheRequestTopLevel(t *testing.T) {
	t.Parallel()

	temperature := 0.375
	topP := 0.875

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: &temperature, TopP: &topP}, srv.Client())

	judgeOnce(t, c)

	for _, key := range []string{"temperature", "top_p"} {
		if _, present := topLevelKeys(t, captured.Body)[key]; present {
			t.Fatalf("request carries %q at the top level; the native protocol reads sampling only from options, so a top-level copy is ignored — body=%s", key, captured.Body)
		}
	}

	options := decodedOptions(t, captured.Body)
	var gotTemperature, gotTopP float64
	if err := json.Unmarshal(options["temperature"], &gotTemperature); err != nil {
		t.Fatalf("options carries no numeric temperature: %v; options=%v", err, options)
	}
	if err := json.Unmarshal(options["top_p"], &gotTopP); err != nil {
		t.Fatalf("options carries no numeric top_p: %v; options=%v", err, options)
	}
	if gotTemperature != temperature {
		t.Fatalf("options.temperature = %v, want %v", gotTemperature, temperature)
	}
	if gotTopP != topP {
		t.Fatalf("options.top_p = %v, want %v", gotTopP, topP)
	}
}

func TestJudgeSendsTheOutputCapAsNumPredictBecauseTheNativeProtocolHasNoMaxTokens(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	if _, present := topLevelKeys(t, captured.Body)["max_tokens"]; present {
		t.Fatalf("request carries max_tokens at the top level, which the native protocol does not read; body=%s", captured.Body)
	}

	options := decodedOptions(t, captured.Body)
	if _, present := options["max_tokens"]; present {
		t.Fatalf("request carries max_tokens inside options, which is not the native spelling of the cap; options=%v", options)
	}
	var numPredict int
	if err := json.Unmarshal(options["num_predict"], &numPredict); err != nil {
		t.Fatalf("options carries no numeric num_predict: %v; options=%v", err, options)
	}
	if numPredict != loop.MaxOutputTokens {
		t.Fatalf("options.num_predict = %d, want %d", numPredict, loop.MaxOutputTokens)
	}
}

func TestJudgeOmitsTemperatureAndTopPFromOptionsWhenSamplingIsUnconfigured(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	options := decodedOptions(t, captured.Body)
	for _, key := range []string{"temperature", "top_p"} {
		if _, present := options[key]; present {
			t.Fatalf("options carries %q when nothing was configured for it; a zero is a value the endpoint acts on, not a way to spell unset — options=%v", key, options)
		}
	}
	if _, present := options["num_predict"]; !present {
		t.Fatalf("options omits num_predict, which is never optional; options=%v", options)
	}
}

func TestJudgeSendsAnExplicitZeroTemperatureInOptionsRatherThanOmittingIt(t *testing.T) {
	t.Parallel()

	zero := 0.0

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: &zero}, srv.Client())

	judgeOnce(t, c)

	options := decodedOptions(t, captured.Body)
	raw, present := options["temperature"]
	if !present {
		t.Fatalf("options omits temperature when it was configured at zero; options=%v", options)
	}
	if string(raw) != "0" {
		t.Fatalf("options.temperature = %s, want 0", raw)
	}
}

func TestJudgeSendsNoAuthorizationHeaderWhenTheNativeEndpointNeedsNoKey(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	if captured.Auth != "" {
		t.Fatalf("Authorization = %q, want no header sent", captured.Auth)
	}
}

func TestJudgeSendsAuthorizationBearerToTheNativeEndpointWhenKeyIsSet(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "native-secret", loop.Sampling{}, srv.Client())

	judgeOnce(t, c)

	const want = "Bearer native-secret"
	if captured.Auth != want {
		t.Fatalf("Authorization = %q, want %q", captured.Auth, want)
	}
}

func TestJudgeReplaysPriorToolRoundsAsAssistantToolCallsAndNamedToolResults(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{
		System: "sys",
		Block:  "block",
		Input:  "in",
		PriorTools: []loop.ToolExchange{{
			Tool:    loop.ToolWriteFile,
			Path:    "site/index.html",
			Content: "<h1>a page</h1>",
			Bytes:   15,
		}},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolName  string `json:"tool_name"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}

	if len(got.Messages) != 4 {
		t.Fatalf("messages = %d, want the system and user messages plus the assistant call and its result", len(got.Messages))
	}
	if got.Messages[2].Role != "assistant" {
		t.Fatalf("message 2 role = %q, want %q", got.Messages[2].Role, "assistant")
	}
	if len(got.Messages[2].ToolCalls) != 1 {
		t.Fatalf("message 2 carries %d tool calls, want 1", len(got.Messages[2].ToolCalls))
	}
	if got.Messages[2].ToolCalls[0].Function.Name != "write_file" {
		t.Fatalf("replayed call name = %q, want the wire spelling %q", got.Messages[2].ToolCalls[0].Function.Name, "write_file")
	}
	if got.Messages[3].Role != "tool" {
		t.Fatalf("message 3 role = %q, want %q", got.Messages[3].Role, "tool")
	}
	if got.Messages[3].ToolName != "write_file" {
		t.Fatalf("result message tool_name = %q, want %q — the native protocol pairs a result to its call by name, having no call id to pair by", got.Messages[3].ToolName, "write_file")
	}
	if got.Messages[3].Content != "wrote 15 bytes to site/index.html" {
		t.Fatalf("result message content = %q, want the write receipt", got.Messages[3].Content)
	}
}

func TestJudgeReplaysToolArgumentsAsAJSONObjectRatherThanAnEncodedString(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	in := loop.JudgeInput{
		System: "sys",
		Block:  "block",
		Input:  "in",
		PriorTools: []loop.ToolExchange{{
			Tool:    loop.ToolWriteFile,
			Path:    "site/index.html",
			Content: "<h1>a page</h1>",
			Bytes:   15,
		}},
	}
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got struct {
		Messages []struct {
			ToolCalls []struct {
				Function struct {
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request body: %v; body=%s", err, captured.Body)
	}

	arguments := got.Messages[2].ToolCalls[0].Function.Arguments
	if len(arguments) == 0 || arguments[0] != '{' {
		t.Fatalf("replayed arguments = %s, want a JSON object; a leading quote means they were encoded as a string, which is the other protocol's shape", arguments)
	}

	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		t.Fatalf("replayed arguments do not decode as an object: %v; arguments=%s", err, arguments)
	}
	if args.Path != "site/index.html" {
		t.Fatalf("replayed path = %q, want %q", args.Path, "site/index.html")
	}
	if args.Content != "<h1>a page</h1>" {
		t.Fatalf("replayed content = %q, want %q", args.Content, "<h1>a page</h1>")
	}
}

func TestJudgeDecodesATextOnlyNativeResponseAsAnswered(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"message":{"role":"assistant","content":"a plain prose answer"},"done":true,"done_reason":"stop"}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.Answered)
	}
	if result.Answer != "a plain prose answer" {
		t.Fatalf("Answer = %q, want %q", result.Answer, "a plain prose answer")
	}
	if result.RawReason != "stop" {
		t.Fatalf("RawReason = %q, want %q", result.RawReason, "stop")
	}
}

func TestJudgeMapsDoneReasonLengthToTruncated(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"message":{"role":"assistant","content":"half an ans"},"done":true,"done_reason":"length"}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.Truncated {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.Truncated)
	}
	if result.RawReason != "length" {
		t.Fatalf("RawReason = %q, want %q", result.RawReason, "length")
	}
}

func TestJudgeMapsAnUnknownDoneReasonToUnrecognisedAndPreservesTheRawValue(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"message":{"role":"assistant","content":"x"},"done":true,"done_reason":"unload"}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.Unrecognised)
	}
	if result.RawReason != "unload" {
		t.Fatalf("RawReason = %q, want the endpoint's own word %q", result.RawReason, "unload")
	}
}

func TestJudgeDecodesAWriteCallWhoseArgumentsArriveAsAJSONObjectRatherThanAnEncodedString(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_yg4mhi5z","function":{"index":0,"name":"write_file","arguments":{"path":"site/index.html","content":"<h1>a page</h1>"}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsWrite)
	}
	if result.ToolError != "" {
		t.Fatalf("ToolError = %q, want none — object arguments are the native shape, not a malformed string", result.ToolError)
	}
	if result.WritePath != "site/index.html" {
		t.Fatalf("WritePath = %q, want %q", result.WritePath, "site/index.html")
	}
	if result.WriteContent != "<h1>a page</h1>" {
		t.Fatalf("WriteContent = %q, want %q", result.WriteContent, "<h1>a page</h1>")
	}
}

func TestJudgeDecodesANativeRecallCallWithTheParsedQuery(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"recall","arguments":{"query":"the escalation policy"}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsRecall)
	}
	if result.RecallQuery != "the escalation policy" {
		t.Fatalf("RecallQuery = %q, want %q", result.RecallQuery, "the escalation policy")
	}
}

func TestJudgeReadsANativeToolCallEvenThoughDoneReasonSaysStop(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"recall","arguments":{"query":"a missing fact"}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q — the endpoint reports done_reason stop on a tool call, so mapping the reason before reading the calls loses every tool round", result.Reason, loop.WantsRecall)
	}
}

func TestJudgeFlagsUnparseableNativeToolArgumentsAsAMalformedRecallRequest(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"recall","arguments":"query=a string, not an object"}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsRecall)
	}
	if result.RecallQuery != "" {
		t.Fatalf("RecallQuery = %q, want none invented from arguments that did not parse", result.RecallQuery)
	}
	if !strings.Contains(result.ToolError, "could not be parsed") {
		t.Fatalf("ToolError = %q, want it to say the arguments could not be parsed", result.ToolError)
	}
}

func TestJudgeFlagsAnEmptyQueryArgumentOnANativeRecallCallAsMalformed(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"recall","arguments":{"query":"   "}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsRecall)
	}
	if result.RecallQuery != "" {
		t.Fatalf("RecallQuery = %q, want none", result.RecallQuery)
	}
	if result.ToolError != "tool arguments had an empty query" {
		t.Fatalf("ToolError = %q, want the empty-query message", result.ToolError)
	}
}

func TestJudgeAcceptsANativeWriteCallWithEmptyContentRatherThanCallingItMalformed(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"write_file","arguments":{"path":"empty.txt","content":""}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsWrite)
	}
	if result.ToolError != "" {
		t.Fatalf("ToolError = %q, want none — an empty file is a file", result.ToolError)
	}
	if result.WritePath != "empty.txt" {
		t.Fatalf("WritePath = %q, want %q", result.WritePath, "empty.txt")
	}
}

func TestJudgeTreatsANativeCallToAToolThatWasNeverOfferedAsUnrecognised(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"delete_file","arguments":{"path":"site/index.html"}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.Unrecognised)
	}
	if result.WritePath != "" || result.RecallQuery != "" {
		t.Fatalf("result = %+v, want no tool argument read off a tool this adapter never offered", result)
	}
}

func TestJudgeDecodesUsageFromPromptEvalCountAndEvalCount(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"x"},"done":true,"done_reason":"stop","prompt_eval_count":323,"eval_count":33}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Usage == nil {
		t.Fatal("Usage is absent, want both counts read off the native names")
	}
	if result.Usage.InTokens != 323 {
		t.Fatalf("InTokens = %d, want 323 — the prompt count, not the completion count", result.Usage.InTokens)
	}
	if result.Usage.OutTokens != 33 {
		t.Fatalf("OutTokens = %d, want 33 — the completion count, not the prompt count", result.Usage.OutTokens)
	}
}

func TestJudgeLeavesUsageAbsentWhenTheNativeResponseReportsNeitherCount(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"message":{"role":"assistant","content":"x"},"done":true,"done_reason":"stop"}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Usage != nil {
		t.Fatalf("Usage = %+v, want absent rather than zero-filled", result.Usage)
	}
}

func TestJudgeTreatsANativeResponseReportingOnlyOneCountAsAbsentInWhole(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"message":{"role":"assistant","content":"x"},"done":true,"done_reason":"stop","prompt_eval_count":323}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Usage != nil {
		t.Fatalf("Usage = %+v, want absent — a half-reported pair is not a usage report", result.Usage)
	}
}

func TestJudgeDecodesNativeUsageWhenPresentEvenIfAllZero(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, `{"message":{"role":"assistant","content":"x"},"done":true,"done_reason":"stop","prompt_eval_count":0,"eval_count":0}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Usage == nil {
		t.Fatal("Usage is absent for a response that reported both counts at zero, want a zero report rather than no report")
	}
	if result.Usage.InTokens != 0 || result.Usage.OutTokens != 0 {
		t.Fatalf("Usage = %+v, want both counts zero", result.Usage)
	}
}

func TestJudgeReadsOnlyTheFirstNativeToolCallIgnoringAnyOthers(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"recall","arguments":{"query":"the first call"}}},{"function":{"name":"write_file","arguments":{"path":"second.txt","content":"the second call"}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q from the first call", result.Reason, loop.WantsRecall)
	}
	if result.RecallQuery != "the first call" {
		t.Fatalf("RecallQuery = %q, want %q", result.RecallQuery, "the first call")
	}
	if result.WritePath != "" {
		t.Fatalf("WritePath = %q, want nothing read from the second call", result.WritePath)
	}
}

func TestJudgeSurfacesTheNativeErrorStringWhichIsNotAnErrorObject(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"model 'no-such-model:1b' not found"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "no-such-model:1b", "", loop.Sampling{}, srv.Client())

	_, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err == nil {
		t.Fatal("Judge returned no error for a 404, want one")
	}
	if !strings.Contains(err.Error(), "model 'no-such-model:1b' not found") {
		t.Fatalf("error = %q, want it to carry the endpoint's own message, which the native protocol sends as a bare string", err)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want it to name the status", err)
	}
}

func TestJudgeOnUnreachableNativeHostReturnsAnError(t *testing.T) {
	t.Parallel()

	c := NewClient("http://127.0.0.1:1", "model-x", "", loop.Sampling{}, &http.Client{})

	_, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err == nil {
		t.Fatal("Judge returned no error against an unreachable host, want one")
	}
}

func TestJudgeResultCarriesTheSamplingTheNativeClientWasConfiguredWith(t *testing.T) {
	t.Parallel()

	temperature := 0.375
	topP := 0.875

	srv, _ := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: &temperature, TopP: &topP}, srv.Client())

	result := judgeOnce(t, c)

	if result.Sampling.Temperature == nil || *result.Sampling.Temperature != temperature {
		t.Fatalf("Sampling.Temperature = %v, want %v", result.Sampling.Temperature, temperature)
	}
	if result.Sampling.TopP == nil || *result.Sampling.TopP != topP {
		t.Fatalf("Sampling.TopP = %v, want %v", result.Sampling.TopP, topP)
	}
}

func TestJudgeNamesTheOllamaAdapterAndTheNativeRouteItPostedTo(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.Provider.Adapter != "ollama" {
		t.Fatalf("Provider.Adapter = %q, want %q", result.Provider.Adapter, "ollama")
	}
	want := srv.URL + "/api/chat"
	if result.Provider.Endpoint != want {
		t.Fatalf("Provider.Endpoint = %q, want %q — the full route, not the base the operator configured", result.Provider.Endpoint, want)
	}
}

func TestNewOllamaClientDefaultsToATimeoutBoundHTTPClientWhenNoneIsSupplied(t *testing.T) {
	t.Parallel()

	c := NewClient("http://model.invalid", "model-x", "", loop.Sampling{}, nil)

	if c.httpClient == nil {
		t.Fatal("httpClient is nil after NewClient was given none, want the fallback client")
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Fatalf("fallback client timeout = %v, want %v — a non-nil client with no timeout trades a fast loud failure for an unbounded hang", c.httpClient.Timeout, DefaultTimeout)
	}
}

func TestTheZeroValueOllamaClientsAccessorAlsoYieldsATimeoutBoundClientNotMerelyANonNilOne(t *testing.T) {
	t.Parallel()

	c := Client{}

	got := c.client()
	if got == nil {
		t.Fatal("client() returned nil on a zero-value Client, want the fallback client")
	}
	if got.Timeout != DefaultTimeout {
		t.Fatalf("fallback client timeout = %v, want %v", got.Timeout, DefaultTimeout)
	}
}

func TestOllamaDefaultTimeoutIsAPositiveBound(t *testing.T) {
	t.Parallel()

	if DefaultTimeout <= 0 {
		t.Fatalf("DefaultTimeout = %v, want a positive duration", DefaultTimeout)
	}
	if DefaultTimeout != 5*time.Minute {
		t.Fatalf("DefaultTimeout = %v, want 5m", DefaultTimeout)
	}
}
