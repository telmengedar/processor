package ollama

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const capturedUnparsedWrite = "<function=write_file>\n<parameter=path>\nindex.html\n</parameter>\n<parameter=content>\n<!DOCTYPE html>\n<html>\n<head>\n    <title>My Website</title>\n</head>\n<body>\n    <h1>Welcome to My Website</h1>\n</body>\n</html>\n</parameter>\n</function>\n</tool_call>"

const capturedWrittenPage = "<!DOCTYPE html>\n<html>\n<head>\n    <title>My Website</title>\n</head>\n<body>\n    <h1>Welcome to My Website</h1>\n</body>\n</html>"

func responseWithContent(t *testing.T, content string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"message":     map[string]any{"role": "assistant", "content": content},
		"done":        true,
		"done_reason": "stop",
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return string(body)
}

func judgeContent(t *testing.T, content string) loop.JudgeResult {
	t.Helper()
	srv, _ := capturingServer(t, responseWithContent(t, content))
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())
	return judgeOnce(t, c)
}

func TestJudgeRecoversAWriteCallLeftInTheResponseTextWhenTheEndpointReportedNoToolCalls(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, capturedUnparsedWrite)

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want %q — the endpoint reported stop with an empty tool-call field and a complete call sitting in the text", result.Reason, loop.WantsWrite)
	}
	if result.ToolError != "" {
		t.Fatalf("ToolError = %q, want none", result.ToolError)
	}
	if result.WritePath != "index.html" {
		t.Fatalf("WritePath = %q, want %q", result.WritePath, "index.html")
	}
	if result.WriteContent != capturedWrittenPage {
		t.Fatalf("WriteContent = %q, want the page verbatim %q", result.WriteContent, capturedWrittenPage)
	}
}

func TestARecoveredCallKeepsAngleBracketsInsideItsValueRatherThanStoppingAtTheFirstOne(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, capturedUnparsedWrite)

	if !strings.HasPrefix(result.WriteContent, "<!DOCTYPE html>") {
		t.Fatalf("WriteContent = %q, want it to open with the doctype; stopping at the first angle bracket truncates every file this recovers", result.WriteContent)
	}
	if !strings.HasSuffix(result.WriteContent, "</html>") {
		t.Fatalf("WriteContent = %q, want it to close with the html tag; the value ends at the parameter terminator, not at a nested closing tag", result.WriteContent)
	}
	if strings.Contains(result.WriteContent, "</parameter>") || strings.Contains(result.WriteContent, "</function>") {
		t.Fatalf("WriteContent = %q, want the call's own markers excluded from the file", result.WriteContent)
	}
	if strings.Contains(result.WriteContent, "tool_call") {
		t.Fatalf("WriteContent = %q, want the trailing stray close tag excluded from the file", result.WriteContent)
	}
}

func TestARecoveredValueLosesExactlyOneFramingNewlineAtEachEndAndKeepsTheRest(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "<function=write_file>\n<parameter=path>\nnotes.txt\n</parameter>\n<parameter=content>\n\n  indented first line\ntrailing blank follows\n\n</parameter>\n</function>")

	const want = "\n  indented first line\ntrailing blank follows\n"
	if result.WriteContent != want {
		t.Fatalf("WriteContent = %q, want %q — trimming whitespace rather than one framing newline destroys leading indentation and trailing blank lines", result.WriteContent, want)
	}
}

func TestARecoveredEmptyValueIsAnEmptyFileRatherThanAMissingArgument(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "<function=write_file>\n<parameter=path>\nempty.txt\n</parameter>\n<parameter=content>\n\n</parameter>\n</function>")

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsWrite)
	}
	if result.ToolError != "" {
		t.Fatalf("ToolError = %q, want none — the argument was emitted and its value is empty", result.ToolError)
	}
	if result.WriteContent != "" {
		t.Fatalf("WriteContent = %q, want empty", result.WriteContent)
	}
}

func TestJudgeRecoversARecallCallLeftInTheResponseText(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "<function=recall>\n<parameter=query>\nthe escalation policy\n</parameter>\n</function>")

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsRecall)
	}
	if result.RecallQuery != "the escalation policy" {
		t.Fatalf("RecallQuery = %q, want %q", result.RecallQuery, "the escalation policy")
	}
}

func TestARecoveredCallIsMarkedAsComingFromTheResponseTextAndNotFromTheEndpoint(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, capturedUnparsedWrite)

	if result.ToolSource != loop.ToolSourceContent {
		t.Fatalf("ToolSource = %q, want %q — a run carried by the fallback must be distinguishable from one the endpoint parsed", result.ToolSource, loop.ToolSourceContent)
	}
}

func TestACallTheEndpointReportedIsMarkedNativeAndNotAsRecoveredText(t *testing.T) {
	t.Parallel()

	const body = `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"write_file","arguments":{"path":"index.html","content":"<h1>hi</h1>"}}}]},"done":true,"done_reason":"stop"}`

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.ToolSource != loop.ToolSourceNative {
		t.Fatalf("ToolSource = %q, want %q", result.ToolSource, loop.ToolSourceNative)
	}
}

func TestProseCarryingNoCallShapeIsAnsweredAndIsNotMarkedWithAnyToolSource(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "The context block does not name the escalation policy, so I cannot state it.")

	if result.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want %q — declining to act is a prose answer, not a recovered call", result.Reason, loop.Answered)
	}
	if result.ToolSource != "" {
		t.Fatalf("ToolSource = %q, want none on a response that carried no call at all", result.ToolSource)
	}
}

func TestATruncatedCallInTheResponseTextIsRecordedAsAFailedRoundRatherThanReadAsProse(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "<function=write_file>\n<parameter=path>\nindex.html\n</parameter>\n<parameter=content>\n<!DOCTYPE html>")

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want %q — a call the endpoint left unparsed and this adapter could not complete is a failed tool round, and answering with the raw markup would report it as success", result.Reason, loop.WantsWrite)
	}
	if result.ToolError == "" {
		t.Fatal("ToolError is empty for a call whose content argument was never terminated, want the round recorded as failed")
	}
	if result.WritePath != "" || result.WriteContent != "" {
		t.Fatalf("result = %+v, want no write argument invented from an incomplete call", result)
	}
	if result.ToolSource != loop.ToolSourceContent {
		t.Fatalf("ToolSource = %q, want %q even on the failure", result.ToolSource, loop.ToolSourceContent)
	}
}

func TestACallInTheResponseTextNamingAToolThatWasNeverOfferedIsUnrecognisedRatherThanProse(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "<function=delete_file>\n<parameter=path>\nindex.html\n</parameter>\n</function>")

	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.Unrecognised)
	}
	if result.WritePath != "" {
		t.Fatalf("WritePath = %q, want nothing read from a tool this adapter never offered", result.WritePath)
	}
}

func TestARecoveredRecallCallMissingItsQueryArgumentIsRecordedAsAFailedRound(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, "<function=recall>\n</function>")

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want %q", result.Reason, loop.WantsRecall)
	}
	if result.RecallQuery != "" {
		t.Fatalf("RecallQuery = %q, want none invented", result.RecallQuery)
	}
	if result.ToolError == "" {
		t.Fatal("ToolError is empty for a recovered call carrying no query, want the round recorded as failed")
	}
}

func TestTheFallbackIsNotConsultedWhenTheEndpointAlreadyReportedACall(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"role":    "assistant",
			"content": capturedUnparsedWrite,
			"tool_calls": []any{map[string]any{
				"function": map[string]any{
					"name":      "write_file",
					"arguments": map[string]string{"path": "native.html", "content": "the native call"},
				},
			}},
		},
		"done":        true,
		"done_reason": "stop",
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	srv, _ := capturingServer(t, string(body))
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)

	if result.WritePath != "native.html" {
		t.Fatalf("WritePath = %q, want %q — the reported call wins over anything the text also happens to carry", result.WritePath, "native.html")
	}
	if result.ToolSource != loop.ToolSourceNative {
		t.Fatalf("ToolSource = %q, want %q", result.ToolSource, loop.ToolSourceNative)
	}
}

func TestARecoveredCallKeepsTheResponseTextAsTheAnswerSoTheRecordShowsWhatArrived(t *testing.T) {
	t.Parallel()

	result := judgeContent(t, capturedUnparsedWrite)

	if result.Answer != capturedUnparsedWrite {
		t.Fatalf("Answer = %q, want the response text verbatim so a reader can see the markup the endpoint failed to parse", result.Answer)
	}
}
