package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

var markupAnswers = []string{
	"<|tool_call>",
	"<|TOOL_CALL>  ",
	"<|tool_call> hello there",
	"<|tool_call>call: hello",
	"<|tool_call>recalling the old days",
	"<|tool_call|>",
	"<|tool_cal",
	"<tool_call|>",
	"hello there <tool_call|>",
	"I said <tool_call|> lol",
	"call:recall{query:x}<tool_call|>",
}

func requireCallMarkupWithheld(t *testing.T, label, answer string, record Record) {
	t.Helper()

	if record.NonReply != NonReplyCallMarkup {
		t.Fatalf("%s: nonReply = %q, want %q", label, record.NonReply, NonReplyCallMarkup)
	}
	if record.Outcome.Verdict != VerdictWithheld || record.Outcome.Produced {
		t.Fatalf("%s: outcome = %+v, want verdict %q and produced false", label, record.Outcome, VerdictWithheld)
	}
	if record.Answer != answer {
		t.Fatalf("%s: record.Answer = %q, want the model's text verbatim: %q", label, record.Answer, answer)
	}
}

func TestAFinalAnswerCarryingCallMarkupIsWithheldOnAClauseCall(t *testing.T) {
	t.Parallel()

	for _, answer := range markupAnswers {
		run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer))

		if len(run.record.Trailers) != 1 || run.record.Trailers[0] != TrailerClause {
			t.Fatalf("%q: trailers = %v, test setup error: want one clause call", answer, run.record.Trailers)
		}
		requireCallMarkupWithheld(t, "clause call "+answer, answer, run.record)
		for _, round := range run.record.ToolCalls {
			if round.Source == ToolSourceEcho {
				t.Fatalf("%q: round %+v has source echo, want markup never recovered as an echo", answer, round)
			}
		}
	}
}

func TestAFinalAnswerCarryingCallMarkupIsWithheldOnAFoundCall(t *testing.T) {
	t.Parallel()

	for _, answer := range markupAnswers {
		run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), recallFor("a phrase"), answered(answer))

		if got := run.record.Trailers; len(got) != 2 || got[1] != TrailerFound {
			t.Fatalf("%q: trailers = %v, test setup error: want the answering call to be a found call", answer, got)
		}
		requireCallMarkupWithheld(t, "found call "+answer, answer, run.record)
	}
}

func TestCallMarkupIsRecognisedInAnyCase(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"<|TOOL_CALL>", "<TOOL_CALL|>", "x <Tool_Call|> y"} {
		record, _ := runAnswering(t, answered(answer))
		requireCallMarkupWithheld(t, answer, answer, record)
	}
}

func TestCallMarkupIsRecognisedAnywhereInTheAnswer(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"I said <tool_call|> lol", "hello there <tool_call|>", "<tool_call|> hello there", "line one\nline two <tool_call|>"} {
		record, _ := runAnswering(t, answered(answer))
		requireCallMarkupWithheld(t, answer, answer, record)
	}
}

func TestAnAnswerThatAnEarlierArmOwnsKeepsItsKindWhateverMarkupItCarries(t *testing.T) {
	t.Parallel()

	unhonoured := answered("call:write_note{text:hi}<tool_call|>")
	unhonoured.UnhonouredCall = ToolNote
	record, _ := runAnswering(t, unhonoured)
	if record.NonReply != NonReplyToolText {
		t.Fatalf("unhonoured call: nonReply = %q, want %q", record.NonReply, NonReplyToolText)
	}

	results := researchToTheCap()
	results[len(results)-1].Answer = "one more <tool_call|>"
	model := &fakeModel{results: results, ignoresWithheldToolList: true}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, fixedSystem("system"), "test-model", testLogger())
	reserved, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reserved.ReservedCall.State != ReservedCallCompleted || reserved.NonReply != NonReplyToolRequest {
		t.Fatalf("reserved call: state = %q, nonReply = %q, want a tool request keeping %q", reserved.ReservedCall.State, reserved.NonReply, NonReplyToolRequest)
	}

	block, _ := runAnswering(t, answered("===== CANDIDATE =====\nlater <tool_call|>"))
	if block.NonReply != NonReplyBlockText {
		t.Fatalf("frame line: nonReply = %q, want %q", block.NonReply, NonReplyBlockText)
	}
}

func TestPartReferenceAndQueryEchoKeepTheirKindOverTheMarkupArm(t *testing.T) {
	t.Parallel()

	shown := func(id int64) bool { return id == 12 }
	if got := classifyNonReply("id: 12", false, "", shown, neverAnEcho); got != NonReplyPartReference {
		t.Errorf("a shown id = %q, want %q", got, NonReplyPartReference)
	}
	if got := classifyNonReply("what is it <tool_call|>", false, "", shown, func(string) bool { return true }); got != NonReplyQueryEcho {
		t.Errorf("an echo = %q, want %q: the markup arm is last", got, NonReplyQueryEcho)
	}
}

func TestTextThatMerelyResemblesCallMarkupOrCallNamesIsDelivered(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"call: me later", "recall: x", "the tool_call field", "<tool>", "use the tool, call me"} {
		if got := classifyNonReply(answer, false, "", func(int64) bool { return false }, neverAnEcho); got != "" {
			t.Errorf("classifyNonReply(%q) = %q, want none: only markup is held by the markup arm", answer, got)
		}
	}
	for _, answer := range []string{"call: me later", "recall: x"} {
		record, _ := runAnswering(t, answered(answer))
		if record.NonReply != "" || !record.Outcome.Produced {
			t.Errorf("%q: nonReply = %q, produced = %t, want it delivered on an adapter that left it as an answer", answer, record.NonReply, record.Outcome.Produced)
		}
	}
}

func TestEveryEntryOfTheMarkupListWithholdsAndIsRefusedByEcho(t *testing.T) {
	t.Parallel()

	reader := newEchoReader("hello", nil)
	for entry, text := range map[string]string{
		"<tool_call":  "tell me about <tool_calls today",
		"tool_call|>": "tell me about x tool_call|> today",
		"<|tool":      "tell me about <|tool_cal today",
		"tool_call>":  "tell me about x tool_call> today",
	} {
		if got := classifyNonReply(text, false, "", func(int64) bool { return false }, neverAnEcho); got != NonReplyCallMarkup {
			t.Errorf("%q (entry %q): kind = %q, want %q", text, entry, got, NonReplyCallMarkup)
		}
		if shape := reader.shapeOf(text, false); shape != "" {
			t.Errorf("%q (entry %q): echo shape = %q, want none", text, entry, shape)
		}

		unmarked := strings.NewReplacer("<tool_calls", "tool calls", "tool_call|>", "tool call", "<|tool_cal", "tool cal", "tool_call>", "tool call").Replace(text)
		if shape := reader.shapeOf(unmarked, false); shape != echoUnfinishedPhrase {
			t.Fatalf("%q (entry %q): echo shape = %q, test setup error: want the markup-free text to be an unfinished phrase", unmarked, entry, shape)
		}

		run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(text))
		if len(run.record.Trailers) != 1 || run.record.Trailers[0] != TrailerClause {
			t.Fatalf("%q (entry %q): trailers = %v, test setup error: want one clause call", text, entry, run.record.Trailers)
		}
		requireCallMarkupWithheld(t, "entry "+entry, text, run.record)
		for _, round := range run.record.ToolCalls {
			if round.Source == ToolSourceEcho {
				t.Fatalf("%q (entry %q): round %+v has source echo, want markup refused by echo", text, entry, round)
			}
		}
	}
}

func TestAReservedAnsweringCallThatWritesCallMarkupIsCurtailedAndCarriesTheKind(t *testing.T) {
	t.Parallel()

	record, _ := runAnswering(t, researchThenAnswer("<tool_call|>")...)

	if record.ReservedCall.State != ReservedCallCompleted {
		t.Fatalf("test setup error: reservedCall = %+v, want a completed reserved call", record.ReservedCall)
	}
	if record.NonReply != NonReplyCallMarkup || record.Outcome.Verdict != VerdictCurtailed || record.Outcome.Produced {
		t.Fatalf("nonReply = %q, outcome = %+v, want %q beside a curtailed verdict and produced false", record.NonReply, record.Outcome, NonReplyCallMarkup)
	}
}

func TestAMarkupAnswerOnARunThatChoseSilenceStaysSilentAndKeepsTheKind(t *testing.T) {
	t.Parallel()

	run := runEchoing(t, colourInput, "", silenceOffered, graphYieldingNewRowsToEveryRecall(), answered("NOREPLY <tool_call|>"))

	if run.record.Outcome.Verdict != VerdictSilent || run.record.NonReply != NonReplyCallMarkup || run.record.Outcome.Produced {
		t.Fatalf("verdict = %q, nonReply = %q, produced = %t, want silent beside %q", run.record.Outcome.Verdict, run.record.NonReply, run.record.Outcome.Produced, NonReplyCallMarkup)
	}
}

func TestARecordWhoseAnswerHasNoMarkupCarriesNoNonReplyKey(t *testing.T) {
	t.Parallel()

	record, _ := runAnswering(t, answered("It goes to the on-call lead."))

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "nonReply") || !record.Outcome.Produced {
		t.Fatalf("record = %s, want a delivered reply with no nonReply key", encoded)
	}

	withheld, _ := runAnswering(t, answered("<tool_call|>"))
	encoded, err = json.Marshal(withheld)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"nonReply":"callMarkup"`) {
		t.Fatalf("record = %s, want the kind on the wire", encoded)
	}
}
