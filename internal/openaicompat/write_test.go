package openaicompat

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/action"
	"github.com/telmengedar/processor/internal/loop"
)

func responseSaying(t *testing.T, content, finish string) string {
	t.Helper()
	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("encoding the response content: %v", err)
	}
	return `{"choices":[{"message":{"content":` + string(encoded) + `,"tool_calls":[]},"finish_reason":"` + finish + `"}]}`
}

func judgeSaying(t *testing.T, content, finish string) loop.JudgeResult {
	t.Helper()
	srv, _ := capturingServer(t, responseSaying(t, content, finish))
	c := NewClient(srv.URL, "m", "", loop.Sampling{}, srv.Client())

	result, err := c.Judge(context.Background(), loop.JudgeInput{})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	return result
}

func block(body string) string {
	return action.Open + "\n" + body + "\n" + action.Close
}

func onlyAction(t *testing.T, result loop.JudgeResult) loop.Action {
	t.Helper()
	if len(result.Problems) != 0 {
		t.Fatalf("Problems = %v, want none", result.Problems)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("len(Actions) = %d, want 1", len(result.Actions))
	}
	return result.Actions[0]
}

func TestJudgeReadsAWriteActionOutOfTheResponseTextWhenToolCallsIsEmptyAndTheFinishReasonIsStop(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, block(`{"name":"write_file","arguments":{"path":"site/index.html","content":"<h1>hi</h1>"}}`), "stop")

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want WantsWrite — the endpoint reported no structured call and said it stopped", result.Reason)
	}
	act := onlyAction(t, result)
	if act.Tool != loop.ToolWriteFile {
		t.Fatalf("Tool = %q, want %q", act.Tool, loop.ToolWriteFile)
	}
	if act.WritePath != "site/index.html" {
		t.Fatalf("WritePath = %q, want %q", act.WritePath, "site/index.html")
	}
	if act.WriteContent != "<h1>hi</h1>" {
		t.Fatalf("WriteContent = %q, want %q", act.WriteContent, "<h1>hi</h1>")
	}
	if act.Error != "" {
		t.Fatalf("Error = %q, want empty for a well-formed action", act.Error)
	}
}

func TestJudgeReadsARecallActionOutOfTheResponseText(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, block(`{"name":"recall","arguments":{"query":"the missing thing"}}`), "stop")

	if result.Reason != loop.WantsRecall {
		t.Fatalf("Reason = %q, want WantsRecall", result.Reason)
	}
	act := onlyAction(t, result)
	if act.RecallQuery != "the missing thing" {
		t.Fatalf("RecallQuery = %q, want %q", act.RecallQuery, "the missing thing")
	}
}

func TestJudgeReadsBothActionsOfATwoActionResponseInTheOrderTheyWereWritten(t *testing.T) {
	t.Parallel()

	content := block(`{"name":"write_file","arguments":{"path":"index.html","content":"a"}}`) +
		"\n" + block(`{"name":"write_file","arguments":{"path":"README.md","content":"b"}}`)

	result := judgeSaying(t, content, "stop")

	if len(result.Problems) != 0 {
		t.Fatalf("Problems = %v, want none", result.Problems)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("len(Actions) = %d, want 2 — the second action must not be dropped", len(result.Actions))
	}
	if result.Actions[0].WritePath != "index.html" || result.Actions[1].WritePath != "README.md" {
		t.Fatalf("Actions = %+v, want index.html then README.md", result.Actions)
	}
}

func TestJudgeKeepsTheProseAsTheAnswerWhenAResponseBothSpeaksAndActs(t *testing.T) {
	t.Parallel()

	content := "Creating the page now.\n" + block(`{"name":"write_file","arguments":{"path":"index.html","content":"<h1>x</h1>"}}`)

	result := judgeSaying(t, content, "stop")

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want WantsWrite — an accompanying sentence does not end the turn", result.Reason)
	}
	if result.Answer != "Creating the page now." {
		t.Fatalf("Answer = %q, want the prose alone", result.Answer)
	}
	if strings.Contains(result.Answer, action.Open) {
		t.Fatal("Answer still carries the action block")
	}
	if onlyAction(t, result).WritePath != "index.html" {
		t.Fatalf("WritePath = %q, want index.html", result.Actions[0].WritePath)
	}
}

func TestJudgeFlagsUnreadableActionArgumentsWithoutInventingAPath(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, block(`{"name":"write_file","arguments":"not an object"}`), "stop")

	if result.Reason != loop.WantsWrite {
		t.Fatalf("Reason = %q, want WantsWrite", result.Reason)
	}
	act := onlyAction(t, result)
	if act.Error == "" {
		t.Fatal("Error is empty, want the argument failure surfaced")
	}
	if act.WritePath != "" || act.WriteContent != "" {
		t.Fatalf("WritePath = %q and WriteContent = %q, want both empty when the arguments did not parse", act.WritePath, act.WriteContent)
	}
}

func TestJudgeFlagsAnEmptyQueryArgumentAsAMalformedRecallRequest(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, block(`{"name":"recall","arguments":{"query":"   "}}`), "stop")

	act := onlyAction(t, result)
	if act.Error == "" {
		t.Fatal("Error is empty, want a whitespace-only query refused")
	}
	if act.RecallQuery != "" {
		t.Fatalf("RecallQuery = %q, want empty", act.RecallQuery)
	}
}

func TestJudgeAcceptsAWriteActionWithEmptyContentRatherThanCallingItMalformed(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, block(`{"name":"write_file","arguments":{"path":"empty.txt","content":""}}`), "stop")

	act := onlyAction(t, result)
	if act.Error != "" {
		t.Fatalf("Error = %q, want an accepted write of an empty file", act.Error)
	}
	if act.WritePath != "empty.txt" {
		t.Fatalf("WritePath = %q, want %q", act.WritePath, "empty.txt")
	}
}

func TestJudgeReportsAnActionNamingSomethingNeverOfferedAsUnrecognisedAndDeclaresNoAction(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, block(`{"name":"run_shell","arguments":{"cmd":"rm -rf /"}}`), "stop")

	if result.Reason != loop.Unrecognised {
		t.Fatalf("Reason = %q, want Unrecognised", result.Reason)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("Actions = %+v, want none", result.Actions)
	}
	if len(result.Problems) != 1 || !strings.Contains(result.Problems[0], "run_shell") {
		t.Fatalf("Problems = %v, want the refused name on the record", result.Problems)
	}
}

func TestJudgeReportsATruncatedActionAsTruncatedAndKeepsTheUnreadTextOnTheResult(t *testing.T) {
	t.Parallel()

	content := "Writing it now.\n" + action.Open + "\n" + `{"name":"write_file","arguments":{"path":"index.html","content":"<!DOCTYPE html`

	result := judgeSaying(t, content, "length")

	if result.Reason != loop.Truncated {
		t.Fatalf("Reason = %q, want Truncated — the endpoint said it ran out of room", result.Reason)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("Actions = %+v, want none from a block that never finished", result.Actions)
	}
	if len(result.Problems) != 1 || !strings.Contains(result.Problems[0], "<!DOCTYPE html") {
		t.Fatalf("Problems = %v, want the unread text carried on the result", result.Problems)
	}
}

func TestJudgeReportsAnUnreadableActionAsMalformedWhenTheEndpointSaidItStoppedCleanly(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, action.Open+"\n"+`{"name": write_file}`+"\n"+action.Close, "stop")

	if result.Reason != loop.Malformed {
		t.Fatalf("Reason = %q, want Malformed", result.Reason)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("Actions = %+v, want none", result.Actions)
	}
	if len(result.Problems) != 1 {
		t.Fatalf("Problems = %v, want exactly one", result.Problems)
	}
}

func TestJudgeTreatsAResponseWithNoActionAsAnsweredCarryingNoProblem(t *testing.T) {
	t.Parallel()

	result := judgeSaying(t, "There is nothing in the block about that.", "stop")

	if result.Reason != loop.Answered {
		t.Fatalf("Reason = %q, want Answered", result.Reason)
	}
	if len(result.Actions) != 0 || len(result.Problems) != 0 {
		t.Fatalf("Actions = %+v and Problems = %v, want neither", result.Actions, result.Problems)
	}
	if result.Answer != "There is nothing in the block about that." {
		t.Fatalf("Answer = %q, want the answer verbatim", result.Answer)
	}
}

func TestJudgeCarriesAWriteWhoseContentHoldsTheClosingDelimiterThroughUnharmed(t *testing.T) {
	t.Parallel()

	content := block(`{"name":"write_file","arguments":{"path":"a.html","content":"before` + action.Close + `after"}}`)

	act := onlyAction(t, judgeSaying(t, content, "stop"))
	if act.WriteContent != "before"+action.Close+"after" {
		t.Fatalf("WriteContent = %q, want the closing delimiter preserved inside the file", act.WriteContent)
	}
}

func TestJudgeBoundsTheExcerptOfAnUnreadableActionAndStillStatesItsWholeSize(t *testing.T) {
	t.Parallel()

	filler := strings.Repeat("x", 5000)
	content := action.Open + "\n" + `{"name":"write_file","arguments":{"path":"a.html","content":"` + filler

	result := judgeSaying(t, content, "stop")

	if len(result.Problems) != 1 {
		t.Fatalf("Problems = %v, want exactly one", result.Problems)
	}
	problem := result.Problems[0]
	if len(problem) > problemExcerptBytes+512 {
		t.Fatalf("the recorded problem is %d bytes, want the excerpt bounded", len(problem))
	}
	if !strings.Contains(problem, strconv.Itoa(len(content))) {
		t.Fatalf("problem = %q, want the whole unread size stated even though the excerpt is bounded", problem)
	}
}
