package action

import (
	"encoding/json"
	"strings"
	"testing"
)

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func onlyCall(t *testing.T, result Result) Call {
	t.Helper()
	if len(result.Problems) != 0 {
		t.Fatalf("Problems = %v, want none", result.Problems)
	}
	if len(result.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(result.Calls))
	}
	return result.Calls[0]
}

func writeArguments(t *testing.T, call Call) writeArgs {
	t.Helper()
	var args writeArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		t.Fatalf("decoding arguments %s: %v", call.Arguments, err)
	}
	return args
}

func TestParseKeepsAClosingDelimiterThatSitsInsideTheActionsOwnContent(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `{"name":"write_file","arguments":{"path":"a.html","content":"before` + Close + `after"}}` + Close)

	args := writeArguments(t, onlyCall(t, result))
	if args.Content != "before"+Close+"after" {
		t.Fatalf("Content = %q, want the closing delimiter preserved inside it", args.Content)
	}
}

func TestParseKeepsAnOpeningDelimiterThatSitsInsideTheActionsOwnContent(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `{"name":"write_file","arguments":{"path":"a.html","content":"` + Open + `"}}` + Close)

	args := writeArguments(t, onlyCall(t, result))
	if args.Content != Open {
		t.Fatalf("Content = %q, want %q", args.Content, Open)
	}
}

func TestParseKeepsBracesThatSitInsideTheActionsOwnContent(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `{"name":"write_file","arguments":{"path":"a.json","content":"{\"a\": {\"b\": 1}}"}}` + Close)

	args := writeArguments(t, onlyCall(t, result))
	if args.Content != `{"a": {"b": 1}}` {
		t.Fatalf("Content = %q, want the braces preserved", args.Content)
	}
}

func TestParseReturnsBothActionsOfATwoActionResponseInTheOrderTheyWereWritten(t *testing.T) {
	t.Parallel()

	response := Open + `{"name":"write_file","arguments":{"path":"first.html","content":"1"}}` + Close +
		"\n" + Open + `{"name":"write_file","arguments":{"path":"second.html","content":"2"}}` + Close

	result := Parse(response)

	if len(result.Problems) != 0 {
		t.Fatalf("Problems = %v, want none", result.Problems)
	}
	if len(result.Calls) != 2 {
		t.Fatalf("len(Calls) = %d, want 2", len(result.Calls))
	}
	if got := writeArguments(t, result.Calls[0]).Path; got != "first.html" {
		t.Fatalf("Calls[0] path = %q, want %q", got, "first.html")
	}
	if got := writeArguments(t, result.Calls[1]).Path; got != "second.html" {
		t.Fatalf("Calls[1] path = %q, want %q", got, "second.html")
	}
}

func TestParseReportsATruncatedActionAsAProblemAndDeclaresNoCall(t *testing.T) {
	t.Parallel()

	response := "I will write it now.\n" + Open + `{"name":"write_file","arguments":{"path":"a.html","content":"<!DOCTYPE`

	result := Parse(response)

	if len(result.Calls) != 0 {
		t.Fatalf("len(Calls) = %d, want 0 — a truncated block declares nothing", len(result.Calls))
	}
	if len(result.Problems) != 1 {
		t.Fatalf("len(Problems) = %d, want 1", len(result.Problems))
	}
	if !strings.Contains(result.Problems[0].Text, "<!DOCTYPE") {
		t.Fatalf("Problem.Text = %q, want the unread text carried in it", result.Problems[0].Text)
	}
	if result.Prose != "I will write it now." {
		t.Fatalf("Prose = %q, want the text written before the block", result.Prose)
	}
}

func TestParseReportsAnActionWhoseJSONIsCompleteButNeverClosedAsAProblem(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `{"name":"write_file","arguments":{"path":"a.html","content":"x"}}`)

	if len(result.Calls) != 0 {
		t.Fatalf("len(Calls) = %d, want 0 — the block was never closed", len(result.Calls))
	}
	if len(result.Problems) != 1 || result.Problems[0].Reason != reasonUnterminated {
		t.Fatalf("Problems = %+v, want one unterminated block", result.Problems)
	}
}

func TestParseRejectsTextBetweenTheActionObjectAndTheClosingDelimiter(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `{"name":"recall","arguments":{"query":"q"}} and also this` + Close)

	if len(result.Calls) != 0 {
		t.Fatalf("len(Calls) = %d, want 0", len(result.Calls))
	}
	if len(result.Problems) != 1 || result.Problems[0].Reason != reasonUnterminated {
		t.Fatalf("Problems = %+v, want one unterminated block", result.Problems)
	}
}

func TestParseCarriesTheWholeUnreadTailIntoTheProblemRatherThanStoppingAtTheFirstBadByte(t *testing.T) {
	t.Parallel()

	response := Open + `{"name":"write_file","arguments":{"path":` + "\ntrailing words the parser could not attribute"

	result := Parse(response)

	if len(result.Problems) != 1 {
		t.Fatalf("len(Problems) = %d, want 1", len(result.Problems))
	}
	if !strings.Contains(result.Problems[0].Text, "trailing words the parser could not attribute") {
		t.Fatalf("Problem.Text = %q, want the whole unread tail", result.Problems[0].Text)
	}
}

func TestParseReportsAnActionBlockHoldingSomethingOtherThanAnObject(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `"write_file"` + Close)

	if len(result.Calls) != 0 {
		t.Fatalf("len(Calls) = %d, want 0", len(result.Calls))
	}
	if len(result.Problems) != 1 || result.Problems[0].Reason != reasonNotObject {
		t.Fatalf("Problems = %+v, want one non-object block", result.Problems)
	}
}

func TestParseReportsAnActionBlockWithNoNameAndKeepsReadingTheBlocksAfterIt(t *testing.T) {
	t.Parallel()

	response := Open + `{"arguments":{"query":"q"}}` + Close +
		Open + `{"name":"recall","arguments":{"query":"second"}}` + Close

	result := Parse(response)

	if len(result.Problems) != 1 || result.Problems[0].Reason != reasonNoName {
		t.Fatalf("Problems = %+v, want one unnamed block", result.Problems)
	}
	if len(result.Calls) != 1 || result.Calls[0].Name != Recall {
		t.Fatalf("Calls = %+v, want the block after the unnamed one still read", result.Calls)
	}
}

func TestParseTreatsAResponseWithNoActionBlockAsProseAlone(t *testing.T) {
	t.Parallel()

	result := Parse("There is nothing in the context block about that, so I cannot say.")

	if len(result.Calls) != 0 || len(result.Problems) != 0 {
		t.Fatalf("Calls = %+v and Problems = %+v, want neither", result.Calls, result.Problems)
	}
	if result.Prose != "There is nothing in the context block about that, so I cannot say." {
		t.Fatalf("Prose = %q, want the answer verbatim", result.Prose)
	}
}

func TestParseKeepsTheProseOnBothSidesOfAnActionAndLeavesTheBlockOutOfIt(t *testing.T) {
	t.Parallel()

	response := "Creating the page.\n" + Open + `{"name":"write_file","arguments":{"path":"a.html","content":"x"}}` + Close + "\nDone."

	result := Parse(response)

	call := onlyCall(t, result)
	if writeArguments(t, call).Path != "a.html" {
		t.Fatalf("path = %q, want a.html", writeArguments(t, call).Path)
	}
	if result.Prose != "Creating the page.\n\nDone." {
		t.Fatalf("Prose = %q, want the text either side of the block with the block removed", result.Prose)
	}
	if strings.Contains(result.Prose, Open) {
		t.Fatal("Prose still carries the action block")
	}
}

func TestParseReadsAnActionWrittenOnASingleLineWithNoSurroundingWhitespace(t *testing.T) {
	t.Parallel()

	result := Parse(Open + `{"name":"recall","arguments":{"query":"what the block is missing"}}` + Close)

	if onlyCall(t, result).Name != Recall {
		t.Fatalf("Name = %q, want %q", result.Calls[0].Name, Recall)
	}
}

func TestParseKeepsTheArgumentsOfAnActionEncodedRatherThanFlatteningThem(t *testing.T) {
	t.Parallel()

	result := Parse(Open + "\n" + `{"name":"recall","arguments":{"query":"q","extra":[1,2]}}` + "\n" + Close)

	call := onlyCall(t, result)
	if !strings.Contains(string(call.Arguments), `"extra"`) {
		t.Fatalf("Arguments = %s, want the members the response wrote", call.Arguments)
	}
}

func TestProtocolStatesBothActionNamesAndBothDelimiters(t *testing.T) {
	t.Parallel()

	for _, want := range []string{Open, Close, Recall, WriteFile, "path", "content", "query"} {
		if !strings.Contains(Protocol, want) {
			t.Fatalf("Protocol does not state %q, so the model is not told what the parser reads", want)
		}
	}
}
