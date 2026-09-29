package openaicompat

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const wantReadDescription = "Read one part of memory in full. Takes one argument: id, the integer id that part is printed with."

func offeredTools(t *testing.T, in loop.JudgeInput) []wireTool {
	t.Helper()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	in.MaxOutputTokens = judgeBudget
	if _, err := c.Judge(context.Background(), in); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got chatRequest
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request: %v; body=%s", err, captured.Body)
	}
	return got.Tools
}

func toolNamed(tools []wireTool, name string) (wireTool, bool) {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return tool, true
		}
	}
	return wireTool{}, false
}

func priorRound(t *testing.T, exchange loop.ToolExchange) (name string, arguments string) {
	t.Helper()

	srv, captured := capturingServer(t, stopResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	_, err := c.Judge(context.Background(), loop.JudgeInput{
		System: "sys", Block: "block", Input: "in",
		PriorTools:      []loop.ToolExchange{exchange},
		MaxOutputTokens: judgeBudget,
	})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	var got chatRequest
	if err := json.Unmarshal(captured.Body, &got); err != nil {
		t.Fatalf("decode request: %v; body=%s", err, captured.Body)
	}
	for _, message := range got.Messages {
		if len(message.ToolCalls) == 0 {
			continue
		}
		return message.ToolCalls[0].Function.Name, message.ToolCalls[0].Function.Arguments
	}
	t.Fatalf("the request replays no tool call at all, so this guard would pass vacuously; body=%s", captured.Body)
	return "", ""
}

func translateNativeCall(t *testing.T, name, arguments string) loop.JudgeResult {
	t.Helper()

	response, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]any{
				"tool_calls": []any{map[string]any{
					"id":       "call-1",
					"type":     "function",
					"function": map[string]any{"name": name, "arguments": arguments},
				}},
			},
			"finish_reason": "tool_calls",
		}},
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	srv, _ := capturingServer(t, string(response))
	return judgeOnce(t, NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()))
}

func TestAJudgementCallOffersTheAddressedReadBesideRecallAndTheFileWrite(t *testing.T) {
	t.Parallel()

	tools := offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in"})

	for _, name := range []string{recallToolName, writeFileToolName, readNodeToolName} {
		if _, present := toolNamed(tools, name); !present {
			t.Fatalf("the judgement call offers no tool named %q; offered %d tools: a tool the model is never shown cannot be called", name, len(tools))
		}
	}

	read, _ := toolNamed(tools, readNodeToolName)
	if read.Type != "function" {
		t.Fatalf("the read tool is of type %q, want %q", read.Type, "function")
	}
	if read.Function.Description != wantReadDescription {
		t.Fatalf("the read tool's description is %q, want %q: the description is the whole of what tells the model what the one argument is", read.Function.Description, wantReadDescription)
	}

	var schema map[string]any
	if err := json.Unmarshal(read.Function.Parameters, &schema); err != nil {
		t.Fatalf("the read tool's parameters are not a JSON schema object: %v; parameters=%s", err, read.Function.Parameters)
	}
	want := map[string]any{
		"type":       "object",
		"properties": map[string]any{"id": map[string]any{"type": "integer"}},
		"required":   []any{"id"},
	}
	if !reflect.DeepEqual(schema, want) {
		t.Fatalf("the read tool's parameters are %#v, want %#v: the argument is one required integer and the schema is what says so", schema, want)
	}
}

func TestTheReservedAnsweringCallOffersNoToolIncludingTheAddressedRead(t *testing.T) {
	t.Parallel()

	offered := offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if _, present := toolNamed(offered, readNodeToolName); !present {
		t.Fatalf("test setup error: a call that withholds nothing offers no read tool, so the withheld arm below proves nothing")
	}

	withheld := offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in", WithholdTools: true})
	if len(withheld) != 0 {
		t.Fatalf("the reserved answering call carries %d tools, want none: a third tool must be withheld no less than the two that already were", len(withheld))
	}
}

func TestAPriorAddressedReadRoundIsReplayedAsTheReadToolAndNotAsARecall(t *testing.T) {
	t.Parallel()

	name, _ := priorRound(t, loop.ToolExchange{Tool: loop.ToolReadNode, NodeID: 71})

	if name == recallToolName {
		t.Fatal("a completed addressed read is announced back to the model as the recall tool, so every later call in the turn carries a false history of what the turn did")
	}
	if name != readNodeToolName {
		t.Fatalf("a completed addressed read is announced back as %q, want %q", name, readNodeToolName)
	}
}

func TestAPriorAddressedReadRoundIsReplayedWithTheIdItAskedForAndNoQueryArgument(t *testing.T) {
	t.Parallel()

	_, arguments := priorRound(t, loop.ToolExchange{Tool: loop.ToolReadNode, NodeID: 71})

	if strings.Contains(arguments, "query") {
		t.Fatalf("a completed addressed read is replayed with arguments %s, which carry a query: the round asked for an id and the replay must not invent a search the model never ran", arguments)
	}
	if arguments != `{"id":71}` {
		t.Fatalf("a completed addressed read is replayed with arguments %s, want the id it asked for", arguments)
	}
}

func TestATranslatedAddressedReadCarriesTheIdItWasCalledWith(t *testing.T) {
	t.Parallel()

	result := translateNativeCall(t, readNodeToolName, `{"id":71}`)

	if result.Reason != loop.WantsRead {
		t.Fatalf("a native read call gave Reason %q, want %q", result.Reason, loop.WantsRead)
	}
	if result.ReadNodeID != 71 {
		t.Fatalf("a native read call gave ReadNodeID %d, want 71", result.ReadNodeID)
	}
	if result.ToolError != "" {
		t.Fatalf("a well-formed read call reported the tool error %q, want none", result.ToolError)
	}
	if result.RecallQuery != "" {
		t.Fatalf("a native read call gave RecallQuery %q, want none: the two tools' arguments must not leak into one another", result.RecallQuery)
	}
	if result.ToolSource != loop.ToolSourceNative {
		t.Fatalf("a native read call gave ToolSource %q, want %q", result.ToolSource, loop.ToolSourceNative)
	}
}

func TestATranslatedAddressedReadWithoutANumericIdIsAToolErrorAndNeverAReadOfNodeZero(t *testing.T) {
	t.Parallel()

	const (
		wantNoID        = "tool arguments named no node id to read"
		wantUnparseable = "tool arguments could not be parsed: "
	)

	for _, c := range []struct {
		name      string
		arguments string
		wantCause string
		prefix    bool
	}{
		{"no id at all", `{}`, wantNoID, false},
		{"a zero id", `{"id":0}`, wantNoID, false},
		{"a negative id", `{"id":-71}`, wantNoID, false},
		{"a textual id", `{"id":"71"}`, wantUnparseable, true},
	} {
		result := translateNativeCall(t, readNodeToolName, c.arguments)
		if result.Reason != loop.WantsRead {
			t.Errorf("%s: Reason is %q, want %q: the call was still a read, and a read the loop drops is a round the model never learns the fate of", c.name, result.Reason, loop.WantsRead)
		}
		if result.ReadNodeID != 0 {
			t.Errorf("%s: ReadNodeID is %d, want 0", c.name, result.ReadNodeID)
		}

		switch {
		case result.ToolError == "":
			t.Errorf("%s: the adapter reported no tool error at all, so the loop is asked to read whatever id fell out of the parse", c.name)
		case c.prefix && !strings.HasPrefix(result.ToolError, c.wantCause):
			t.Errorf("%s: the cause is %q, want it to begin %q. A cause is what the model acts on, so it is pinned against a literal here; the tail is the JSON decoder's and is not ours to spell", c.name, result.ToolError, c.wantCause)
		case !c.prefix && result.ToolError != c.wantCause:
			t.Errorf("%s: the cause is %q, want %q verbatim. Asserted only as non-empty, a malformed read's cause can be replaced by a sentence about a failed file write and the model is told to fix the wrong thing", c.name, result.ToolError, c.wantCause)
		}
	}
}

func TestAPriorRecallRoundIsReplayedAsTheRecallToolWithTheQueryItAskedFor(t *testing.T) {
	t.Parallel()

	name, arguments := priorRound(t, loop.ToolExchange{Tool: loop.ToolRecall, Query: "what changed"})

	if name == readNodeToolName {
		t.Fatal("a completed recall round is announced back to the model as the addressed read, so every recall in every turn is relabelled: the fall-through arm now serves recall by default, and a default arm that names the wrong tool falsifies more of the turn's history than the read case it was widened for")
	}
	if name != recallToolName {
		t.Fatalf("a completed recall round is announced back as %q, want %q", name, recallToolName)
	}
	if arguments != `{"query":"what changed"}` {
		t.Fatalf("a completed recall round is replayed with arguments %s, want the query it asked for: replayed as an id, every recall in the turn loses the query that produced its rows and the model is shown a search it never ran", arguments)
	}
}

func TestAPriorFileWriteRoundIsReplayedAsTheFileWriteWithItsPathAndContent(t *testing.T) {
	t.Parallel()

	name, arguments := priorRound(t, loop.ToolExchange{Tool: loop.ToolWriteFile, Path: "notes.md", Content: "what I have so far"})

	if name != writeFileToolName {
		t.Fatalf("a completed file write is announced back as %q, want %q", name, writeFileToolName)
	}
	if arguments != `{"path":"notes.md","content":"what I have so far"}` {
		t.Fatalf("a completed file write is replayed with arguments %s, want the path and content it asked for", arguments)
	}
}

func TestTheWireNameOfTheAddressedReadIsTheOneTheModelIsToldToCall(t *testing.T) {
	t.Parallel()

	const wantName = "read_node"

	if readNodeToolName != wantName {
		t.Fatalf("this adapter offers the addressed read as %q, want %q. The wire name is a contract with the endpoint and with the other adapter, so it is pinned against a literal here: every other assertion in this package names the same constant on both sides and cannot see it drift", readNodeToolName, wantName)
	}
	if _, present := toolNamed(offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in"}), wantName); !present {
		t.Fatalf("no tool named %q reaches the wire", wantName)
	}
}
