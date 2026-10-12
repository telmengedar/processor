package ollama

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

	srv, captured := capturingServer(t, doneResponse)
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

func translateNativeCall(t *testing.T, name, arguments string) loop.JudgeResult {
	t.Helper()

	response, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"role": "assistant",
			"tool_calls": []any{map[string]any{
				"function": map[string]any{"name": name, "arguments": json.RawMessage(arguments)},
			}},
		},
		"done":        true,
		"done_reason": "tool_calls",
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	srv, _ := capturingServer(t, string(response))
	return judgeOnce(t, NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()))
}

func TestAJudgementCallOffersTheAddressedReadBesideRecallAndTheFileWrite(t *testing.T) {
	t.Parallel()

	tools := offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in", Offered: allTools})

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

	offered := offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in", Offered: allTools})
	if _, present := toolNamed(offered, readNodeToolName); !present {
		t.Fatalf("test setup error: a call that withholds nothing offers no read tool, so the withheld arm below proves nothing")
	}

	withheld := offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if len(withheld) != 0 {
		t.Fatalf("the reserved answering call carries %d tools, want none: a third tool must be withheld no less than the two that already were", len(withheld))
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
		wantNoID          = "tool arguments named no node id to read"
		wantMissingInText = "the call in the response text was missing its id argument"
		wantUnparseable   = "tool arguments could not be parsed: "
	)

	native := []struct {
		name      string
		arguments string
		wantCause string
		prefix    bool
	}{
		{"no id at all", `{}`, wantNoID, false},
		{"a zero id", `{"id":0}`, wantNoID, false},
		{"a negative id", `{"id":-71}`, wantNoID, false},
		{"a textual id", `{"id":"71"}`, wantUnparseable, true},
	}
	for _, c := range native {
		result := translateNativeCall(t, readNodeToolName, c.arguments)
		if result.Reason != loop.WantsRead {
			t.Errorf("%s: Reason is %q, want %q: the call was still a read, and a read the loop drops is a round the model never learns the fate of", c.name, result.Reason, loop.WantsRead)
		}
		if result.ReadNodeID != 0 {
			t.Errorf("%s: ReadNodeID is %d, want 0", c.name, result.ReadNodeID)
		}
		assertCause(t, c.name, result.ToolError, c.wantCause, c.prefix)
	}

	recovered := []struct {
		name      string
		content   string
		wantCause string
	}{
		{"a recovered call with no id", readCallInText(""), wantMissingInText},
		{"a recovered call with a textual id", readCallInText("eight thousand"), wantNoID + `: "eight thousand"`},
		{"a recovered call with a zero id", readCallInText("0"), wantNoID + `: "0"`},
		{"a recovered call with a negative id", readCallInText("-71"), wantNoID + `: "-71"`},
	}
	for _, c := range recovered {
		srv, _ := capturingServer(t, responseWithContent(t, c.content))
		result := judgeOnce(t, NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()))

		if result.Reason != loop.WantsRead {
			t.Errorf("%s: Reason is %q, want %q", c.name, result.Reason, loop.WantsRead)
		}
		if result.ReadNodeID != 0 {
			t.Errorf("%s: ReadNodeID is %d, want 0", c.name, result.ReadNodeID)
		}
		assertCause(t, c.name, result.ToolError, c.wantCause, false)
	}
}

func assertCause(t *testing.T, name, got, want string, prefix bool) {
	t.Helper()

	if got == "" {
		t.Errorf("%s: the adapter reported no tool error at all, so the loop is asked to read whatever id fell out of the parse", name)
		return
	}
	if prefix {
		if !strings.HasPrefix(got, want) {
			t.Errorf("%s: the cause is %q, want it to begin %q. A cause is what the model acts on, so it is pinned against a literal here; the tail is the JSON decoder's and is not ours to spell", name, got, want)
		}
		return
	}
	if got != want {
		t.Errorf("%s: the cause is %q, want %q verbatim. Asserted only as non-empty, one malformed read's cause can be swapped for another's — or for a sentence about a failed file write — and the model is told to fix the wrong thing", name, got, want)
	}
}

func readCallInText(id string) string {
	if id == "" {
		return "<function=read_node></function>"
	}
	return "<function=read_node><parameter=id>" + id + "</parameter></function>"
}

func TestARecoveredAddressedReadCarriesTheIdTheResponseTextNamedEvenWhereItIsPaddedWithWhitespace(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, id string }{
		{"the id alone", "71"},
		{"a space on each side", " 71 "},
		{"a tab before it", "\t71"},
		{"a space and a newline around it", " 71\n"},
	} {
		srv, _ := capturingServer(t, responseWithContent(t, readCallInText(c.id)))
		result := judgeOnce(t, NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client()))

		if result.Reason != loop.WantsRead || result.ReadNodeID != 71 {
			t.Errorf("%s: a read call recovered from the response text gave Reason %q and ReadNodeID %d, want %q and 71. The recovered path is the path a model takes when it never emitted a tool call at all, so its arguments arrive as prose and carry whatever spacing the model wrote; refusing them for their padding refuses the sloppiest caller on the sloppiest path", c.name, result.Reason, result.ReadNodeID, loop.WantsRead)
		}
		if result.ToolError != "" {
			t.Errorf("%s: the adapter reported the cause %q, want none", c.name, result.ToolError)
		}
		if result.ToolSource != loop.ToolSourceContent {
			t.Errorf("%s: a recovered read call gave ToolSource %q, want %q", c.name, result.ToolSource, loop.ToolSourceContent)
		}
	}
}

func TestTheWireNameOfTheAddressedReadIsTheOneTheModelIsToldToCall(t *testing.T) {
	t.Parallel()

	const wantName = "read_node"

	if readNodeToolName != wantName {
		t.Fatalf("this adapter offers the addressed read as %q, want %q. The wire name is a contract with the endpoint and with the other adapter, so it is pinned against a literal here: every other assertion in this package names the same constant on both sides and cannot see it drift", readNodeToolName, wantName)
	}
	if _, present := toolNamed(offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in", Offered: allTools}), wantName); !present {
		t.Fatalf("no tool named %q reaches the wire", wantName)
	}
}
