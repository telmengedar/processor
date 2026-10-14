package ollama

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func answerOf(content string) chatResponse {
	return chatResponse{Message: wireResponseMessage{Content: content}, DoneReason: "stop"}
}

func everyLoopTool() []string {
	var names []string
	for _, tool := range toolTable {
		names = append(names, tool.loopName)
	}
	return names
}

var readOnlyTools = []string{loop.ToolRecall, loop.ToolReadNode}

func TestARecallWrittenAsTheOpeningOfTheAnswerIsARecallRequestWithTheTextualSource(t *testing.T) {
	t.Parallel()

	result := translate(answerOf("recall: the escalation policy"), []string{loop.ToolRecall, loop.ToolReadNode})

	if result.Reason != loop.WantsRecall || result.RecallQuery != "the escalation policy" || result.ToolError != "" {
		t.Fatalf("result = %+v, want a recall request for %q", result, "the escalation policy")
	}
	if result.ToolSource != loop.ToolSourceTextual {
		t.Fatalf("ToolSource = %q, want %q: a call written as text must not read as native or as the markup fallback", result.ToolSource, loop.ToolSourceTextual)
	}
	if result.UnhonouredCall != "" {
		t.Fatalf("UnhonouredCall = %q on a call that was honoured", result.UnhonouredCall)
	}
}

func TestEveryBraceAndQuoteSpellingOfARecallCarriesTheSameQuery(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		"recall{query: when we met}",
		`recall{query: "when we met"}`,
		`recall{"query": "when we met"}`,
		`recall: "when we met"`,
		"recall: 'when we met'",
	} {
		result := translate(answerOf(content), []string{loop.ToolRecall})

		if result.Reason != loop.WantsRecall || result.RecallQuery != "when we met" {
			t.Fatalf("%q -> %+v, want a recall for %q", content, result, "when we met")
		}
	}
}

func TestTheToolNameIsMatchedWithoutRegardToCase(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"Recall: x", "RECALL{query: x}"} {
		if result := translate(answerOf(content), []string{loop.ToolRecall}); result.Reason != loop.WantsRecall {
			t.Fatalf("%q -> %+v, want a recall request", content, result)
		}
	}
}

func TestWhitespaceBeforeTheOpeningCallIsIgnored(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"\n\nrecall: x", "  \t recall{query: x}", "\r\n recall: x"} {
		if result := translate(answerOf(content), []string{loop.ToolRecall}); result.Reason != loop.WantsRecall || result.RecallQuery != "x" {
			t.Fatalf("%q -> %+v, want a recall for x", content, result)
		}
	}
}

func TestOnlyTheFirstLineOfAColonCallIsItsArgument(t *testing.T) {
	t.Parallel()

	result := translate(answerOf("recall: the escalation policy\nI will look that up and tell you what I find."), []string{loop.ToolRecall})

	if result.RecallQuery != "the escalation policy" {
		t.Fatalf("RecallQuery = %q, want the first line alone: the prose after it is not part of the call", result.RecallQuery)
	}
}

func TestATextualReadOfAPositiveIdIsAReadAndAnythingElseIsAReadWithAToolError(t *testing.T) {
	t.Parallel()

	offered := []string{loop.ToolRecall, loop.ToolReadNode}

	for _, content := range []string{"read_node: 42", "read_node{id: 42}"} {
		result := translate(answerOf(content), offered)
		if result.Reason != loop.WantsRead || result.ReadNodeID != 42 || result.ToolError != "" || result.ToolSource != loop.ToolSourceTextual {
			t.Fatalf("%q -> %+v, want a textual read of 42", content, result)
		}
	}
	for _, content := range []string{"read_node: abc", "read_node: 0", "read_node: -3", "read_node{id: seven}"} {
		result := translate(answerOf(content), offered)
		if result.Reason != loop.WantsRead || result.ToolError == "" || result.ReadNodeID != 0 {
			t.Fatalf("%q -> %+v, want a read request carrying a tool error and no id", content, result)
		}
	}
}

func TestNoToolOutsideTheAllowListIsEverHonouredFromText(t *testing.T) {
	t.Parallel()

	offered := everyLoopTool()
	checked := 0
	for _, tool := range toolTable {
		if slices.Contains(readOnlyTools, tool.loopName) {
			continue
		}
		checked++

		for _, content := range []string{tool.wireName + ": anything", tool.wireName + "{path: a.txt, content: x}", strings.ToUpper(tool.wireName) + ": anything"} {
			result := translate(answerOf(content), offered)

			if result.Reason != loop.Answered {
				t.Fatalf("%s: %q -> Reason %q, want %q: a side-effecting tool is never dispatched from text", tool.loopName, content, result.Reason, loop.Answered)
			}
			if result.UnhonouredCall != tool.loopName {
				t.Fatalf("%s: %q -> UnhonouredCall %q, want the tool the answer wrote", tool.loopName, content, result.UnhonouredCall)
			}
			if result.ToolSource != "" {
				t.Fatalf("%s: %q -> ToolSource %q on an unhonoured call", tool.loopName, content, result.ToolSource)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no tool-table row lies outside the allow-list, so nothing was checked")
	}
	if !slices.Contains(offered, loop.ToolNote) || !slices.Contains(offered, loop.ToolWriteFile) {
		t.Fatalf("offered = %v, want the table-driven check to cover the write and note tools", offered)
	}
}

func TestATextualCallIsHonouredOnlyOnACallThatOfferedThatTool(t *testing.T) {
	t.Parallel()

	for name, offered := range map[string][]string{"nothing offered": nil, "only read_node offered": {loop.ToolReadNode}} {
		result := translate(answerOf("recall: x"), offered)

		if result.Reason != loop.Answered || result.RecallQuery != "" || result.ToolSource != "" {
			t.Fatalf("%s: %+v, want the answer kept and no recall request", name, result)
		}
		if result.UnhonouredCall != loop.ToolRecall {
			t.Fatalf("%s: UnhonouredCall = %q, want %q: a call to a tool this call did not offer is still flagged to the loop", name, result.UnhonouredCall, loop.ToolRecall)
		}
	}
}

func nativeCallBesideText(content, name string) chatResponse {
	return chatResponse{
		Message: wireResponseMessage{
			Content:   content,
			ToolCalls: []wireToolCall{{Function: wireFunctionCall{Name: name, Arguments: json.RawMessage(`{"query":"x"}`)}}},
		},
		DoneReason: "stop",
	}
}

func TestATextualCallIsFlaggedBesideANativeCallToAToolThisCallDidNotOffer(t *testing.T) {
	t.Parallel()

	result := translate(nativeCallBesideText("recall: secret plans", recallToolName), nil)
	if result.Reason != loop.Answered || result.UnhonouredCall != loop.ToolRecall {
		t.Fatalf("%+v, want an answer flagged %q", result, loop.ToolRecall)
	}

	prose := translate(nativeCallBesideText("Sure, here you go.", recallToolName), nil)
	if prose.UnhonouredCall != "" {
		t.Fatalf("prose beside a native call -> UnhonouredCall %q, want none", prose.UnhonouredCall)
	}
}

func TestATextualCallIsFlaggedBesideANativeCallToAToolTheAdapterDoesNotKnow(t *testing.T) {
	t.Parallel()

	result := translate(nativeCallBesideText("write_file: x", "frobnicate"), []string{loop.ToolRecall, loop.ToolReadNode})
	if result.Reason != loop.Unrecognised || result.UnhonouredCall != loop.ToolWriteFile {
		t.Fatalf("%+v, want it unrecognised and flagged %q", result, loop.ToolWriteFile)
	}
}

func TestSpeechThatOnlyResemblesAToolCallIsNotRecognised(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		"recalls: x",
		"recall x",
		"recall : x",
		"I recall: x",
		"Gangolf: hi",
		"recall",
		"read_nodes{id: 3}",
		"I will check.\nrecall{query: x}",
		"",
	} {
		result := translate(answerOf(content), everyLoopTool())

		if result.Reason != loop.Answered || result.UnhonouredCall != "" || result.ToolSource != "" {
			t.Fatalf("%q -> %+v, want it passed through as prose", content, result)
		}
	}
}

func TestANativeCallOutranksTextAndMarkupOutranksAnOpeningCall(t *testing.T) {
	t.Parallel()

	native := chatResponse{
		Message: wireResponseMessage{
			Content:   "recall: from the text",
			ToolCalls: []wireToolCall{{Function: wireFunctionCall{Name: recallToolName, Arguments: json.RawMessage(`{"query":"from the call"}`)}}},
		},
		DoneReason: "stop",
	}
	if result := translate(native, []string{loop.ToolRecall}); result.RecallQuery != "from the call" || result.ToolSource != loop.ToolSourceNative {
		t.Fatalf("native + text -> %+v, want the native call", result)
	}

	markup := "recall: from the opening\n<function=recall>\n<parameter=query>\nfrom the markup\n</parameter>\n</function>"
	if result := translate(answerOf(markup), []string{loop.ToolRecall}); result.RecallQuery != "from the markup" || result.ToolSource != loop.ToolSourceContent || result.UnhonouredCall != "" {
		t.Fatalf("markup + opening call -> %+v, want the markup call with source %q", result, loop.ToolSourceContent)
	}
}

func TestABraceBodyNamingNoQueryOrLeftOpenIsAMissingQueryNeverAStandInQuery(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"recall{q: X}", "recall{query: X", "recall{X}"} {
		result := translate(answerOf(content), []string{loop.ToolRecall})

		if result.Reason != loop.WantsRecall || result.ToolError == "" || result.RecallQuery != "" {
			t.Fatalf("%q -> %+v, want a recall request with a tool error and no query", content, result)
		}
		if !strings.Contains(result.ToolError, "missing its query") {
			t.Fatalf("%q -> ToolError %q, want the missing-argument error", content, result.ToolError)
		}
	}
}

func TestARecallWithNothingAfterTheColonIsAnEmptyQueryError(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"recall:", "recall:   \nprose", `recall: ""`} {
		result := translate(answerOf(content), []string{loop.ToolRecall})

		if result.Reason != loop.WantsRecall || result.RecallQuery != "" || !strings.Contains(result.ToolError, "empty query") {
			t.Fatalf("%q -> %+v, want a recall request with the empty-query error", content, result)
		}
	}
}

func TestATextualCallIsFlaggedBesideMarkupForAToolThisCallDidNotOffer(t *testing.T) {
	t.Parallel()

	content := "recall: x\n<function=write_file><parameter=path>a</parameter></function>"
	result := translate(answerOf(content), []string{loop.ToolRecall, loop.ToolReadNode})
	if result.Reason != loop.Unrecognised || result.ToolSource != loop.ToolSourceContent || result.UnhonouredCall != loop.ToolRecall {
		t.Fatalf("%+v, want it unrecognised from content and flagged %q", result, loop.ToolRecall)
	}
}
