package loop

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func runAnswering(t *testing.T, results ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, fixedSystem("system"), "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return record, model
}

func textualRecall(query string) JudgeResult {
	return JudgeResult{Answer: "recall: " + query, Reason: WantsRecall, RawReason: "stop", RecallQuery: query, ToolSource: ToolSourceTextual}
}

func TestATextualRecallIsDispatchedLikeANativeOneAndTheNextCallAnswers(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	model := &fakeModel{results: []JudgeResult{textualRecall("the escalation policy"), answered("It goes to the on-call lead.")}}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := dispatchedRecalls(graph); got != 1 {
		t.Fatalf("the graph served %d supplementary recalls, want 1: the textual call is dispatched like a native one", got)
	}
	if got := graph.recallCalls[primaryRecallCalls].Queries; len(got) != 1 || got[0] != "the escalation policy" {
		t.Fatalf("the supplementary recall asked %v, want the query the answer wrote", got)
	}
	if len(record.Trailers) != 2 || record.Trailers[1] != TrailerFound {
		t.Fatalf("trailers = %v, want the call after the search to carry %q", record.Trailers, TrailerFound)
	}
	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Source != ToolSourceTextual {
		t.Fatalf("toolCalls = %+v, want one round whose source is %q", record.ToolCalls, ToolSourceTextual)
	}
	if record.Answer != "It goes to the on-call lead." || record.NonReply != "" || !record.Outcome.Produced {
		t.Fatalf("answer = %q, nonReply = %q, outcome = %+v, want the next call's answer delivered", record.Answer, record.NonReply, record.Outcome)
	}
	if len(model.calls) != 2 {
		t.Fatalf("the model was called %d times, want 2", len(model.calls))
	}
}

func TestAnAnswerOpeningWithAFrameLineIsWithheldAsBlockText(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{
		"===== CANDIDATE =====\nid: 6840\ntype: thought\nname: x",
		"===== ANCHOR =====\nsomething",
		"\n  ===== EARLIER IN THIS CONVERSATION =====\nuser said: hi",
	} {
		record, _ := runAnswering(t, answered(answer))

		if record.NonReply != NonReplyBlockText {
			t.Fatalf("%q: nonReply = %q, want %q", answer, record.NonReply, NonReplyBlockText)
		}
		if record.Answer != answer {
			t.Fatalf("record.Answer = %q, want the model's text verbatim: %q", record.Answer, answer)
		}
		if record.Outcome.Verdict != VerdictWithheld || record.Outcome.Produced {
			t.Fatalf("%q: outcome = %+v, want verdict %q and produced false", answer, record.Outcome, VerdictWithheld)
		}
	}
}

func TestAnIdLineIsBlockTextOnlyWhenATypeLineFollowsIt(t *testing.T) {
	t.Parallel()

	record, _ := runAnswering(t, answered("id: 6840\ntype: thought\nname: x"))
	if record.NonReply != NonReplyBlockText {
		t.Fatalf("an id line and a type line: nonReply = %q, want %q", record.NonReply, NonReplyBlockText)
	}

	for _, answer := range []string{"id: 12 is my favourite.", "id: 12", "id: 12\nand that is all", "id: 12\n  type: not at the line start"} {
		record, model := runAnswering(t, answered(answer))
		if slices.Contains(candidateIDsIn(model.calls[0].Block), 12) {
			t.Fatalf("test setup error: part 12 was shown, and a lone id of a shown part is not delivered")
		}
		if record.NonReply != "" || !record.Outcome.Produced {
			t.Fatalf("%q: nonReply = %q, produced = %t, want it delivered as an ordinary reply", answer, record.NonReply, record.Outcome.Produced)
		}
	}
}

func TestAnAnswerOpeningWithEitherHistoryLabelIsBlockText(t *testing.T) {
	t.Parallel()

	for _, label := range []string{LabelUserSaid, LabelYouReplied, "You replied:", "User said:", "USER SAID:"} {
		record, _ := runAnswering(t, answered("  "+label+" something from the history"))

		if record.NonReply != NonReplyBlockText || record.Outcome.Verdict != VerdictWithheld {
			t.Fatalf("label %q: nonReply = %q, verdict = %q, want %q and %q", label, record.NonReply, record.Outcome.Verdict, NonReplyBlockText, VerdictWithheld)
		}
	}
}

func TestAnAnswerThatNamesAnUnhonouredTextualCallIsWithheldAsToolText(t *testing.T) {
	t.Parallel()

	result := answered("write_file{path: a.txt}")
	result.UnhonouredCall = ToolWriteFile

	record, _ := runAnswering(t, result)

	if record.NonReply != NonReplyToolText || record.Outcome.Verdict != VerdictWithheld || record.Outcome.Produced {
		t.Fatalf("nonReply = %q, outcome = %+v, want %q withheld", record.NonReply, record.Outcome, NonReplyToolText)
	}
}

func TestAFinalAnswerThatIsATextualCallsOwnTextIsWithheldWhenTheTimeGuardStopsTheTurnAfterIt(t *testing.T) {
	t.Parallel()

	instant := guardInstant
	model := &fakeModel{results: []JudgeResult{textualRecall("more please")}}
	model.beforeReturn = func() { instant = guardInstant.Add(9*time.Minute + 40*time.Second) }
	turn := turnWithClock(model, baseGraph(), func() time.Time { return instant })

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(10*time.Minute))
	defer cancel()

	record, _, err := turn.Run(ctx, "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.TimeShortfall == "" {
		t.Fatal("test setup error: the time guard did not stop the turn")
	}
	if record.Answer != "recall: more please" || record.NonReply != NonReplyToolRequest {
		t.Fatalf("answer = %q, nonReply = %q, want the call text kept verbatim and withheld as %q", record.Answer, record.NonReply, NonReplyToolRequest)
	}
	if record.Outcome.Produced {
		t.Fatalf("outcome = %+v, want produced false: the text of a call that asked for a tool is the call, not an answer", record.Outcome)
	}
}

func TestABlankAnswerIsNeverWithheldWhateverCallItCameFrom(t *testing.T) {
	t.Parallel()

	instant := guardInstant
	round := JudgeResult{Reason: WantsRecall, RawReason: "stop", RecallQuery: "more please", ToolSource: ToolSourceNative}
	model := &fakeModel{results: []JudgeResult{round}}
	model.beforeReturn = func() { instant = guardInstant.Add(9*time.Minute + 40*time.Second) }
	turn := turnWithClock(model, baseGraph(), func() time.Time { return instant })

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(10*time.Minute))
	defer cancel()

	record, _, err := turn.Run(ctx, "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.TimeShortfall == "" {
		t.Fatal("test setup error: the time guard did not stop the turn")
	}
	if record.Answer != "" || record.NonReply != "" {
		t.Fatalf("answer = %q, nonReply = %q, want a blank answer left without a non-reply", record.Answer, record.NonReply)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"nonReply"`) {
		t.Fatalf("record = %s, want no nonReply key on a blank answer", encoded)
	}
}

func TestAToolWantingResultOnTheReservedCallIsWithheldAsAToolRequest(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap(), ignoresWithheldToolList: true}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, fixedSystem("system"), "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.ReservedCall.State != ReservedCallCompleted {
		t.Fatalf("test setup error: reservedCall = %+v, want a completed reserved call", record.ReservedCall)
	}
	if record.NonReply != NonReplyToolRequest || record.Outcome.Produced {
		t.Fatalf("nonReply = %q, outcome = %+v, want the reserved call's tool-wanting text withheld as %q", record.NonReply, record.Outcome, NonReplyToolRequest)
	}
}

func TestOrdinaryRepliesThatMerelyResembleProcessorSyntaxAreDelivered(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{
		"Gangolf: hey",
		"Recall that we met in May.",
		"the id: field is the node's number.",
		"You replied earlier, I think.",
		"==== not a frame ====",
		"A line with ===== inside =====",
		"I said user said: nothing.",
	} {
		record, _ := runAnswering(t, answered(answer))

		if record.NonReply != "" || !record.Outcome.Produced || record.Outcome.Verdict == VerdictWithheld {
			t.Fatalf("%q: nonReply = %q, outcome = %+v, want an ordinary delivered reply", answer, record.NonReply, record.Outcome)
		}
	}
}

func TestAStoredNonReplyRecomputesAsWithheldFromTheRecordAloneAndYieldsToCurtailedAndSilent(t *testing.T) {
	t.Parallel()

	withheld := outcomeRecord()
	withheld.NonReply = NonReplyBlockText

	silent := withheld
	silent.Silent = true

	curtailed := withheld
	curtailed.CapReached = true

	ungrounded := withheld
	ungrounded.Candidates = nil

	for name, c := range map[string]struct {
		record Record
		want   Verdict
	}{
		"a non-reply":                 {withheld, VerdictWithheld},
		"a non-reply and silence":     {silent, VerdictSilent},
		"a non-reply and a bound":     {curtailed, VerdictCurtailed},
		"a non-reply with no grounds": {ungrounded, VerdictWithheld},
	} {
		encoded, err := json.Marshal(c.record)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var decoded Record
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}

		if got := ComputeOutcome(decoded); got.Verdict != c.want {
			t.Fatalf("%s: verdict = %q, want %q", name, got.Verdict, c.want)
		}
	}
	if ComputeOutcome(withheld).Produced {
		t.Fatal("a withheld record reports produced true")
	}
}

func TestARecordWithNoNonReplySerialisesWithNoNonReplyKey(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(outcomeRecord())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "nonReply") {
		t.Fatalf("a record with no non-reply carries a nonReply key: %s", encoded)
	}

	withheld := outcomeRecord()
	withheld.NonReply = NonReplyToolText
	encoded, err = json.Marshal(withheld)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"nonReply":"toolText"`) {
		t.Fatalf("a withheld record does not carry its kind: %s", encoded)
	}
}
