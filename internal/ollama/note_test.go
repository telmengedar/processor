package ollama

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

var toolsWithNote = []string{loop.ToolRecall, loop.ToolWriteFile, loop.ToolReadNode, loop.ToolNote}

func judgeOffering(t *testing.T, body string, offered []string) loop.JudgeResult {
	t.Helper()

	srv, _ := capturingServer(t, body)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())
	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: offered})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	return result
}

func nativeCallBody(t *testing.T, name, arguments string) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"role":       "assistant",
			"tool_calls": []any{map[string]any{"function": map[string]any{"name": name, "arguments": json.RawMessage(arguments)}}},
		},
		"done":        true,
		"done_reason": "tool_calls",
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return string(body)
}

func TestTheNoteToolIsDeclaredOnlyOnACallThatOffersItWithTextRequiredAndAboutOptional(t *testing.T) {
	t.Parallel()

	const wantName = "write_note"

	if noteToolName != wantName {
		t.Fatalf("this adapter offers the note tool as %q, want %q: the wire name is a contract with the endpoint and the other adapter", noteToolName, wantName)
	}

	if _, present := toolNamed(offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in", Offered: allTools}), wantName); present {
		t.Errorf("a call that did not offer the note tool declared %q", wantName)
	}

	tool, present := toolNamed(offeredTools(t, loop.JudgeInput{System: "sys", Block: "block", Input: "in", Offered: toolsWithNote}), wantName)
	if !present {
		t.Fatalf("a call that offered the note tool did not declare %q", wantName)
	}

	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
		t.Fatalf("decode the schema: %v", err)
	}
	if len(schema.Properties) != 2 || schema.Properties["text"].Type != "string" || schema.Properties["about"].Type != "integer" {
		t.Errorf("the note schema declares %+v, want exactly text (string) and about (integer): the model chooses no type, name or link", schema.Properties)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "text" {
		t.Errorf("the note schema requires %v, want text alone", schema.Required)
	}
}

func TestANativeNoteCallTranslatesItsTextAndAboutAndIgnoresEveryOtherArgument(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		arguments string
		text      string
		about     int64
	}{
		"text and about":                     {`{"text":"a thought","about":12}`, "a thought", 12},
		"text alone":                         {`{"text":"a thought"}`, "a thought", 0},
		"a null about":                       {`{"text":"a thought","about":null}`, "a thought", 0},
		"extra arguments are ignored":        {`{"text":"a thought","about":3,"type":"session-log","name":"x","links":[1,2,3]}`, "a thought", 3},
		"an empty text is the loop's to say": {`{"text":""}`, "", 0},
		"multi-line text kept verbatim":      {"{\"text\":\"one\\ntwo \"}", "one\ntwo ", 0},
	} {
		result := judgeOffering(t, nativeCallBody(t, "write_note", tc.arguments), toolsWithNote)

		if result.Reason != loop.WantsNote || result.NoteText != tc.text || result.NoteAbout != tc.about || result.ToolError != "" {
			t.Errorf("%s: result = reason %q text %q about %d error %q, want %q, %q, %d and no error", name, result.Reason, result.NoteText, result.NoteAbout, result.ToolError, loop.WantsNote, tc.text, tc.about)
		}
		if result.ToolSource != loop.ToolSourceNative {
			t.Errorf("%s: ToolSource = %q, want native", name, result.ToolSource)
		}
	}
}

func TestANativeNoteCallWithoutTextOrWithAnAboutThatIsNoPositiveIntegerIsAToolError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ arguments, want string }{
		"no text":                 {`{"about":5}`, errNoNoteText},
		"an empty object":         {`{}`, errNoNoteText},
		"a null text":             {`{"text":null}`, errNoNoteText},
		"a numeric text":          {`{"text":5}`, "tool arguments could not be parsed"},
		"a string about":          {`{"text":"t","about":"12"}`, errBadNoteAbout},
		"a zero about":            {`{"text":"t","about":0}`, errBadNoteAbout},
		"a negative about":        {`{"text":"t","about":-3}`, errBadNoteAbout},
		"a fractional about":      {`{"text":"t","about":1.5}`, errBadNoteAbout},
		"a boolean about":         {`{"text":"t","about":true}`, errBadNoteAbout},
		"a list about":            {`{"text":"t","about":[1,2]}`, errBadNoteAbout},
		"arguments not an object": {`"text"`, "tool arguments could not be parsed"},
	} {
		result := judgeOffering(t, nativeCallBody(t, "write_note", tc.arguments), toolsWithNote)

		if result.Reason != loop.WantsNote || !strings.HasPrefix(result.ToolError, tc.want) {
			t.Errorf("%s: result = reason %q error %q, want %q with an error beginning %q", name, result.Reason, result.ToolError, loop.WantsNote, tc.want)
		}
		if result.NoteText != "" || result.NoteAbout != 0 {
			t.Errorf("%s: result carries text %q about %d, want neither on a malformed call", name, result.NoteText, result.NoteAbout)
		}
	}
}

func TestANoteCallOnACallThatDidNotOfferTheToolIsNotHonoured(t *testing.T) {
	t.Parallel()

	result := judgeOffering(t, nativeCallBody(t, "write_note", `{"text":"a thought"}`), allTools)

	if result.Reason == loop.WantsNote || result.NoteText != "" {
		t.Errorf("result = reason %q text %q, want the call dropped: a tool the request did not offer is never dispatched", result.Reason, result.NoteText)
	}
	if result.UnofferedToolCalls != 1 {
		t.Errorf("UnofferedToolCalls = %d, want 1", result.UnofferedToolCalls)
	}
}

func noteCallInText(parameters ...string) string {
	return "<function=write_note>" + strings.Join(parameters, "") + "</function>"
}

func TestANoteCallRecoveredFromTheResponseTextCarriesTheSameTextAndAboutAsANativeOne(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		content string
		text    string
		about   int64
	}{
		"text and about":              {noteCallInText("<parameter=text>\na thought\n</parameter>", "<parameter=about>\n12\n</parameter>"), "a thought", 12},
		"text alone":                  {noteCallInText("<parameter=text>\na thought\n</parameter>"), "a thought", 0},
		"multi-line text":             {noteCallInText("<parameter=text>\nline one\nline two\n</parameter>"), "line one\nline two", 0},
		"padded about":                {noteCallInText("<parameter=text>x</parameter>", "<parameter=about> 12 </parameter>"), "x", 12},
		"padded text kept verbatim":   {noteCallInText("<parameter=text>\n  indented, with trailing space \n</parameter>"), "  indented, with trailing space ", 0},
		"an empty about":              {noteCallInText("<parameter=text>x</parameter>", "<parameter=about></parameter>"), "x", 0},
		"a null about":                {noteCallInText("<parameter=text>x</parameter>", "<parameter=about>null</parameter>"), "x", 0},
		"an empty text":               {noteCallInText("<parameter=text></parameter>"), "", 0},
		"text with surrounding prose": {"I will note that.\n" + noteCallInText("<parameter=text>x</parameter>"), "x", 0},
	} {
		srv, _ := capturingServer(t, responseWithContent(t, tc.content))
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())
		result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: toolsWithNote})
		if err != nil {
			t.Fatalf("%s: Judge: %v", name, err)
		}

		if result.Reason != loop.WantsNote || result.NoteText != tc.text || result.NoteAbout != tc.about || result.ToolError != "" {
			t.Errorf("%s: result = reason %q text %q about %d error %q, want %q, %q, %d and no error", name, result.Reason, result.NoteText, result.NoteAbout, result.ToolError, loop.WantsNote, tc.text, tc.about)
		}
		if result.ToolSource != loop.ToolSourceContent {
			t.Errorf("%s: ToolSource = %q, want content", name, result.ToolSource)
		}
	}
}

func TestARecoveredNoteCallWithoutTextOrWithABadAboutIsAToolError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ content, want string }{
		"no text":            {noteCallInText("<parameter=about>5</parameter>"), errRecoveredNoteWithoutText},
		"no parameters":      {"<function=write_note></function>", errRecoveredNoteWithoutText},
		"a word as about":    {noteCallInText("<parameter=text>x</parameter>", "<parameter=about>abc</parameter>"), errBadNoteAbout},
		"a zero about":       {noteCallInText("<parameter=text>x</parameter>", "<parameter=about>0</parameter>"), errBadNoteAbout},
		"a negative about":   {noteCallInText("<parameter=text>x</parameter>", "<parameter=about>-4</parameter>"), errBadNoteAbout},
		"a fractional about": {noteCallInText("<parameter=text>x</parameter>", "<parameter=about>1.5</parameter>"), errBadNoteAbout},
		"a quoted about":     {noteCallInText("<parameter=text>x</parameter>", `<parameter=about>"12"</parameter>`), errBadNoteAbout},
	} {
		srv, _ := capturingServer(t, responseWithContent(t, tc.content))
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())
		result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: toolsWithNote})
		if err != nil {
			t.Fatalf("%s: Judge: %v", name, err)
		}

		if result.Reason != loop.WantsNote || !strings.HasPrefix(result.ToolError, tc.want) {
			t.Errorf("%s: result = reason %q error %q, want %q with an error beginning %q", name, result.Reason, result.ToolError, loop.WantsNote, tc.want)
		}
	}
}

func TestARecoveredNoteCallOnACallThatDidNotOfferTheToolIsNotHonoured(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, responseWithContent(t, noteCallInText("<parameter=text>x</parameter>")))
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())
	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in", MaxOutputTokens: judgeBudget, Offered: allTools})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if result.Reason == loop.WantsNote || result.NoteText != "" {
		t.Errorf("result = reason %q text %q, want the recovered call refused as one the request did not offer", result.Reason, result.NoteText)
	}
}

func TestTheLoopsNoteSentenceNamesTheToolByThisAdaptersWireName(t *testing.T) {
	t.Parallel()

	if !strings.Contains(loop.NoteSentence, noteToolName) {
		t.Errorf("the note sentence %q does not contain this adapter's wire name %q for the note tool", loop.NoteSentence, noteToolName)
	}
}
