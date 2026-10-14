package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const (
	colourInput  = "rugenforth: gangolf what is your favourite colour?"
	colourEchoE1 = "what is gangolf's favourite colour"
	colourEchoE2 = "your favourite colour?"
	colourReply  = "pink, obviously."
)

func derivedLines(queries ...string) string {
	return "DATES: none\n" + strings.Join(queries, "\n")
}

type echoRun struct {
	record Record
	model  *fakeModel
	graph  *fakeGraph
}

func runEchoing(t *testing.T, input string, derived string, view *View, graph *fakeGraph, results ...JudgeResult) echoRun {
	t.Helper()

	model := &fakeModel{results: results, derivedText: derived}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: input, Subject: readSubject, Settings: DefaultSettings(), View: view})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return echoRun{record: record, model: model, graph: graph}
}

func graphFindingNothing() *fakeGraph {
	graph := graphHoldingNodes()
	graph.recallQueue = []recallResponse{{}, {}, {}, {}, {}, {}}
	return graph
}

func requireNoEchoRound(t *testing.T, label string, run echoRun) {
	t.Helper()

	for _, round := range run.record.ToolCalls {
		if round.Source == ToolSourceEcho {
			t.Fatalf("%s: round %+v has source echo, want none", label, round)
		}
	}
	if len(run.model.calls) != 1 {
		t.Fatalf("%s: the model was called %d times, want 1: the answer is a reply", label, len(run.model.calls))
	}
	if run.record.NonReply != "" || !run.record.Outcome.Produced {
		t.Fatalf("%s: nonReply = %q, outcome = %+v, want the answer delivered", label, run.record.NonReply, run.record.Outcome)
	}
}

func TestAClauseCallAnsweringAQueryIsRecoveredAsTheRecallItWasAskedForWithSourceEcho(t *testing.T) {
	t.Parallel()

	run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE1), answered(colourReply))

	if run.record.Trailers[0] != TrailerClause {
		t.Fatalf("test setup error: the first call's trailer is %q, want %q", run.record.Trailers[0], TrailerClause)
	}
	round := onlyRound(t, run.record)
	if round.Tool != ToolRecall || round.Source != ToolSourceEcho || round.Query != colourEchoE1 || round.Error != "" {
		t.Fatalf("round = %+v, want a served recall of the answer verbatim with source %q", round, ToolSourceEcho)
	}
	if got := run.graph.recallCalls[primaryRecallCalls].Queries; len(got) != 1 || got[0] != colourEchoE1 {
		t.Fatalf("the graph was asked %v, want the answer as the one query", got)
	}
	if len(run.record.Trailers) != 2 || run.record.Trailers[1] != TrailerFound {
		t.Fatalf("trailers = %v, want the call after the recovered recall to carry %q", run.record.Trailers, TrailerFound)
	}
	if run.record.Answer != colourReply || run.record.NonReply != "" || run.record.Outcome.Verdict != VerdictDelivered || !run.record.Outcome.Produced {
		t.Fatalf("answer = %q, nonReply = %q, outcome = %+v, want the second call's reply delivered", run.record.Answer, run.record.NonReply, run.record.Outcome)
	}
}

func TestARecoveredQueryThatFindsNothingGivesTheBarrenTrailerAndTheNextCallAnswers(t *testing.T) {
	t.Parallel()

	run := runEchoing(t, colourInput, "", nil, graphFindingNothing(), answered("  "+colourEchoE1+" \n"), answered("no idea, honestly."))

	round := onlyRound(t, run.record)
	if round.Source != ToolSourceEcho || round.Query != colourEchoE1 || round.Yield != 0 {
		t.Fatalf("round = %+v, want an echo recall of the trimmed answer that yielded nothing", round)
	}
	if len(run.record.Trailers) != 2 || run.record.Trailers[1] != TrailerBarren {
		t.Fatalf("trailers = %v, want %q after a recovered recall that found nothing", run.record.Trailers, TrailerBarren)
	}
	if !strings.Contains(run.model.calls[1].Trailer, `"`+colourEchoE1+`"`) || strings.Contains(run.model.calls[1].Trailer, recallClause) {
		t.Fatalf("the second call's trailer is %q, want the barren line naming the query and no clause", run.model.calls[1].Trailer)
	}
	if run.record.Answer != "no idea, honestly." || run.record.NonReply != "" {
		t.Fatalf("answer = %q, nonReply = %q, want the second call's reply delivered", run.record.Answer, run.record.NonReply)
	}
}

func TestAnAnswerThatIsAContiguousRunOfWordsOfOneInputLineIsRecoveredAndNothingLooser(t *testing.T) {
	t.Parallel()

	run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE2), answered(colourReply))
	if round := onlyRound(t, run.record); round.Source != ToolSourceEcho || round.Query != colourEchoE2 {
		t.Fatalf("round = %+v, want %q recovered as an echo recall: the input's own words", round, colourEchoE2)
	}

	multi := runEchoing(t, "first line here\nsecond line is longer", "", nil, graphYieldingNewRowsToEveryRecall(), answered("line is longer?"), answered(colourReply))
	if round := onlyRound(t, multi.record); round.Source != ToolSourceEcho {
		t.Fatalf("round = %+v, want a run inside the second input line recovered", round)
	}

	for _, answer := range []string{
		"favourite your colour?",
		"our favourite colour?",
		"here second line?",
		"line here\nsecond",
		"your colour?",
	} {
		run := runEchoing(t, "first line here\nsecond line is longer, your favourite colour?", "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer))
		requireNoEchoRound(t, answer, run)
	}
}

func TestAnAnswerEqualToADerivedQueryIsRecoveredWhateverItsEndingAndCase(t *testing.T) {
	t.Parallel()

	const derived = "What is the colour preference of Gangolf?"
	for _, answer := range []string{derived, strings.ToLower(derived)} {
		run := runEchoing(t, colourInput, derivedLines(derived, "keyword style query"), nil, graphYieldingNewRowsToEveryRecall(), answered(answer), answered(colourReply))

		round := onlyRound(t, run.record)
		if round.Source != ToolSourceEcho || round.Query != answer {
			t.Fatalf("%q: round = %+v, want a derived query written as the reply recovered as an echo recall", answer, round)
		}
	}

	run := runEchoing(t, colourInput, derivedLines(derived, "keyword style query"), nil, graphYieldingNewRowsToEveryRecall(), answered("What is the colour preference of Gangolf's friend?"))
	requireNoEchoRound(t, "a query that only resembles a derived one", run)
}

func TestAnEchoShapedAnswerOnACallThatCarriedNoClauseIsDelivered(t *testing.T) {
	t.Parallel()

	for _, text := range []string{colourEchoE1, colourEchoE2} {
		found := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), recallFor("something real"), answered(text))
		if found.record.Trailers[1] != TrailerFound {
			t.Fatalf("test setup error: the second call's trailer is %q, want %q", found.record.Trailers[1], TrailerFound)
		}

		barren := runEchoing(t, colourInput, "", nil, graphFindingNothing(), recallFor("something real"), answered(text))
		if barren.record.Trailers[1] != TrailerBarren {
			t.Fatalf("test setup error: the second call's trailer is %q, want %q", barren.record.Trailers[1], TrailerBarren)
		}

		for label, run := range map[string]echoRun{"found": found, "barren": barren} {
			if len(run.record.ToolCalls) != 1 || run.record.ToolCalls[0].Source == ToolSourceEcho || len(run.model.calls) != 2 {
				t.Fatalf("%s %q: rounds = %+v, calls = %d, want the one real recall and the text left alone", label, text, run.record.ToolCalls, len(run.model.calls))
			}
			if run.record.Answer != text || run.record.NonReply != "" || !run.record.Outcome.Produced {
				t.Fatalf("%s %q: answer = %q, nonReply = %q, verdict = %q, want it delivered unchanged: no earlier echo round", label, text, run.record.Answer, run.record.NonReply, run.record.Outcome.Verdict)
			}
		}
	}
}

func TestShortRepliesAndSentencesStayRepliesOnAClauseCall(t *testing.T) {
	t.Parallel()

	const chat = "bob: lol that is funny\nrugenforth: gangolf what is your favourite colour?"
	for _, answer := range []string{
		"i don't have a favourite colour.",
		"hey rugenforth",
		"lol",
		"hi",
		"nah 😂",
		"yeah that was funny 😂",
		"you're still there? yeah, i'm here.",
		"that is wild!",
		"well i guess so…",
		"it is (probably) fine)",
		"i said \"no way\"",
		"i'm here",
		"it's fine",
		"don't care",
	} {
		run := runEchoing(t, chat, "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer))
		requireNoEchoRound(t, answer, run)
		if run.record.Answer != answer {
			t.Fatalf("%q: record.Answer = %q, want it verbatim", answer, run.record.Answer)
		}
	}
}

func TestAnUnfinishedPhraseNeedsThreeWordsAndAFragmentOfTheInputNeedsTwo(t *testing.T) {
	t.Parallel()

	recovered := map[string]string{
		"what level was that":      "hello there",
		"1 2 3":                    "hello there",
		"wo kann ich übernachten":  "hello there",
		"rugenforth gangolf":       "rugenforth: gangolf what is your favourite colour?",
		"lol that":                 "bob: lol that is funny",
		"what level":               "what level was that?",
		"it's gangolf's colour":    "hello there",
		"what's the point of this": "hello there",
	}
	for answer, input := range recovered {
		run := runEchoing(t, input, "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer), answered(colourReply))
		if round := onlyRound(t, run.record); round.Source != ToolSourceEcho {
			t.Fatalf("%q on input %q: round = %+v, want it recovered", answer, input, round)
		}
	}

	left := map[string]string{
		"what level": "hello there",
		"hello":      "hello",
		"lol":        "bob: lol that is funny",
		"colour":     "gangolf what is your favourite colour?",
	}
	for answer, input := range left {
		run := runEchoing(t, input, "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer))
		requireNoEchoRound(t, answer, run)
	}
}

func TestCallSyntaxAndPartReferencesAreNeverRecoveredAsAnEchoOnAClauseCall(t *testing.T) {
	t.Parallel()

	const input = "see id: 15 please 15"
	for _, answer := range []string{"read_node(4242)", "call:recall{query:x}<tool_call|>", "id: 15", "15", "ID:15", "  15\n"} {
		run := runEchoing(t, input, derivedLines("15", "id: 15"), nil, graphYieldingNewRowsToEveryRecall(), answered(answer))
		requireNoEchoRound(t, answer, run)
	}
}

func TestAnAnswerThatOpensWithACallTheAdapterDidNotHonourIsNotRecoveredAsAnEcho(t *testing.T) {
	t.Parallel()

	result := answered(colourEchoE1)
	result.UnhonouredCall = ToolRecall

	run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), result)

	if len(run.record.ToolCalls) != 0 || len(run.model.calls) != 1 {
		t.Fatalf("rounds = %+v, calls = %d, want none and 1: an unhonoured call is the adapter's to recover", run.record.ToolCalls, len(run.model.calls))
	}
	if run.record.NonReply != NonReplyToolText {
		t.Fatalf("nonReply = %q, want %q", run.record.NonReply, NonReplyToolText)
	}
}

func TestAMultiLineAnswerAndAnAnswerBeyondAPhraseAreDeliveredNotRecovered(t *testing.T) {
	t.Parallel()

	long := "what is gangolf's favourite" + strings.Repeat(" colour", MaxPhraseBytes/len(" colour")+1)
	if len(long) <= MaxPhraseBytes {
		t.Fatalf("test setup error: %d bytes is a phrase", len(long))
	}
	for _, answer := range []string{
		colourEchoE1 + "\nand some more words here",
		"\n" + colourEchoE2 + "\nyour favourite colour",
		long,
	} {
		run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer))
		requireNoEchoRound(t, answer, run)
	}

	exact := "what" + strings.Repeat(" colour", (MaxPhraseBytes-len("what"))/len(" colour"))
	if len(exact) > MaxPhraseBytes {
		t.Fatalf("test setup error: %d bytes is not a phrase", len(exact))
	}
	run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(exact), answered(colourReply))
	if round := onlyRound(t, run.record); round.Source != ToolSourceEcho || round.Query != exact {
		t.Fatalf("round = %+v, want the largest phrase recovered whole", round)
	}
}

func TestASilenceChosenOnAClauseCallIsSilentAndNeverRecovered(t *testing.T) {
	t.Parallel()

	const declining = "NOREPLY not worth answering"

	on := runEchoing(t, colourInput, "", silenceOffered, graphYieldingNewRowsToEveryRecall(), answered(declining))
	if len(on.record.ToolCalls) != 0 || len(on.model.calls) != 1 {
		t.Fatalf("rounds = %+v, calls = %d, want none and 1: a declined reply is not a query", on.record.ToolCalls, len(on.model.calls))
	}
	if !on.record.Silent || on.record.Outcome.Verdict != VerdictSilent || on.record.NonReply != "" {
		t.Fatalf("silent = %t, verdict = %q, nonReply = %q, want the run silent", on.record.Silent, on.record.Outcome.Verdict, on.record.NonReply)
	}

	off := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(declining), answered(colourReply))
	if round := onlyRound(t, off.record); round.Source != ToolSourceEcho {
		t.Fatalf("round = %+v, want the text recovered on a run that offered no silence: it declines nothing there", round)
	}
}

func TestAnEchoWrittenAgainAfterARecoveredEchoIsWithheldAsQueryEcho(t *testing.T) {
	t.Parallel()

	for _, again := range []string{colourEchoE1, "what is your favourite colour", colourEchoE2} {
		run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE1), answered(again))

		if round := onlyRound(t, run.record); round.Source != ToolSourceEcho {
			t.Fatalf("test setup error: round = %+v, want the first echo recovered", round)
		}
		if run.record.NonReply != NonReplyQueryEcho || run.record.Outcome.Produced || run.record.Outcome.Verdict != VerdictWithheld {
			t.Fatalf("%q: nonReply = %q, outcome = %+v, want %q withheld", again, run.record.NonReply, run.record.Outcome, NonReplyQueryEcho)
		}
		if run.record.Answer != again {
			t.Fatalf("record.Answer = %q, want the model's text verbatim: %q", run.record.Answer, again)
		}
	}
}

func TestAReplyAfterARecoveredEchoIsDeliveredAndASilenceAfterItStaysSilent(t *testing.T) {
	t.Parallel()

	reply := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE1), answered("probably pink."))
	if reply.record.NonReply != "" || !reply.record.Outcome.Produced {
		t.Fatalf("nonReply = %q, outcome = %+v, want the reply delivered", reply.record.NonReply, reply.record.Outcome)
	}

	silent := runEchoing(t, colourInput, "", silenceOffered, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE1), answered("NOREPLY not worth answering"))
	if silent.record.NonReply != "" || silent.record.Outcome.Verdict != VerdictSilent {
		t.Fatalf("nonReply = %q, verdict = %q, want the silence kept", silent.record.NonReply, silent.record.Outcome.Verdict)
	}
}

func TestAnEchoShapedFinalAnswerWithoutAnEarlierEchoRoundIsDelivered(t *testing.T) {
	t.Parallel()

	found := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), recallFor("something real"), answered(colourEchoE1))
	if found.record.NonReply != "" || found.record.Outcome.Verdict != VerdictDelivered || found.record.Answer != colourEchoE1 {
		t.Fatalf("nonReply = %q, verdict = %q, answer = %q, want it delivered", found.record.NonReply, found.record.Outcome.Verdict, found.record.Answer)
	}

	results := make([]JudgeResult, 0, MaxModelCalls)
	graph := graphHoldingNodes()
	for i := range MaxModelCalls - 1 {
		results = append(results, recallFor("a phrase "+strings.Repeat("x", i+1)))
		graph.recallQueue = append(graph.recallQueue, recallResponse{Candidates: freshRows(i + 1)})
	}
	results = append(results, answered(colourEchoE2))
	reserved := runEchoing(t, colourInput, "", nil, graph, results...)
	if !reserved.record.CapReached || reserved.record.NonReply != "" || reserved.record.Answer != colourEchoE2 {
		t.Fatalf("capReached = %t, nonReply = %q, answer = %q, want the reserved call's text delivered", reserved.record.CapReached, reserved.record.NonReply, reserved.record.Answer)
	}
}

func TestAnEchoRecallRefusedAsClosedKeepsItsSourceAndTheReservedCallIsHeldToTheBackstop(t *testing.T) {
	t.Parallel()

	closing := recallsDifferingOnlyByADateSuffix(3)
	closing = append(closing, JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: colourEchoE1, ToolSource: ToolSourceEcho})

	again := runEchoing(t, colourInput, "", nil, graphReturningTheSameRowsToEveryRecall(), append(closing, answered(colourEchoE1))...)
	if !again.record.RecallClosed || len(again.record.ToolCalls) != 4 {
		t.Fatalf("test setup error: recallClosed = %t, rounds = %d, want the fourth round refused as closed", again.record.RecallClosed, len(again.record.ToolCalls))
	}
	if closed := again.record.ToolCalls[3]; closed.Error != errRecallClosed || closed.Source != ToolSourceEcho || closed.Query != colourEchoE1 {
		t.Fatalf("closed round = %+v, want the refusal carrying source %q", closed, ToolSourceEcho)
	}
	if again.record.NonReply != NonReplyQueryEcho {
		t.Fatalf("nonReply = %q, want %q: the reserved call wrote the echo again", again.record.NonReply, NonReplyQueryEcho)
	}

	reply := runEchoing(t, colourInput, "", nil, graphReturningTheSameRowsToEveryRecall(), append(closing, answered("no idea."))...)
	if reply.record.NonReply != "" || reply.record.Answer != "no idea." {
		t.Fatalf("nonReply = %q, answer = %q, want the reserved call's reply delivered", reply.record.NonReply, reply.record.Answer)
	}
}

func TestARecordWithoutAnEchoRoundEncodesAsBefore(t *testing.T) {
	t.Parallel()

	plain := runEchoing(t, "hello", "", nil, graphYieldingNewRowsToEveryRecall(), recallFor("a real phrase"), answered("a reply."))
	encoded, err := json.Marshal(plain.record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, text := range []string{`"nonReply"`, `"echo"`, `queryEcho`} {
		if strings.Contains(string(encoded), text) {
			t.Fatalf("the record of a run without an echo encodes %s: %s", text, encoded)
		}
	}

	echoed := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE1), answered(colourEchoE1))
	encoded, err = json.Marshal(echoed.record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, text := range []string{`"source":"echo"`, `"nonReply":"queryEcho"`, `"verdict":"withheld"`, `"produced":false`} {
		if !strings.Contains(string(encoded), text) {
			t.Fatalf("the record of a withheld echo lacks %s: %s", text, encoded)
		}
	}
}

func TestTheSummaryNamesAnEchoRoundByItsSourceAndAWithheldEchoByItsVerdict(t *testing.T) {
	t.Parallel()

	run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), answered(colourEchoE1), answered(colourEchoE1))

	summary := RenderSummary(run.record, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	if !strings.Contains(summary, "recall [echo]") {
		t.Fatalf("the summary does not name the round's source:\n%s", summary)
	}
	if !strings.Contains(summary, "verdict  withheld (produced no") {
		t.Fatalf("the summary does not say the turn was withheld:\n%s", summary)
	}
}

func TestAToolRequestThatCarriedEchoShapedTextKeepsItsOwnCallAndItsOwnSource(t *testing.T) {
	t.Parallel()

	request := recallFor("a phrase the model chose")
	request.Answer = colourEchoE1
	request.ToolSource = ToolSourceNative

	run := runEchoing(t, colourInput, "", nil, graphYieldingNewRowsToEveryRecall(), request, answered(colourReply))

	round := onlyRound(t, run.record)
	if round.Source != ToolSourceNative || round.Query != "a phrase the model chose" {
		t.Fatalf("round = %+v, want the model's own native recall left alone", round)
	}
}

func TestTheShapeNamesTheRuleThatMatchedAndIsEmptyForAReply(t *testing.T) {
	t.Parallel()

	reader := newEchoReader("bob: where is the report\nsecond line", []string{"bob: where is the report", "What is the report location?"})

	for answer, want := range map[string]string{
		"what is the report":           echoUnfinishedPhrase,
		"where is the report":          echoUnfinishedPhrase,
		"the report?":                  echoInputFragment,
		"What is the report location?": echoDerivedQuery,
		"what is the report location?": echoDerivedQuery,
		"where is the report?":         echoInputFragment,
		"fine.":                        "",
		"":                             "",
		"   ":                          "",
	} {
		if got := reader.shapeOf(answer, false); got != want {
			t.Errorf("shapeOf(%q) = %q, want %q", answer, got, want)
		}
	}
}

func TestCallSyntaxAndBlockSyntaxHeldInTheInputIsNeverRecoveredAsAnEcho(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{
		"read_node(4242)",
		"read_node(4242) please",
		"Recall: what is the colour",
		"write_note{text: a note here}",
		"call:recall{query:what is the colour}",
		"what is the colour <tool_call|>",
		"===== CANDIDATE ===== the colour",
		"you replied: the colour is pink",
		"what is ===== the colour",
	} {
		run := runEchoing(t, answer+"\nrugenforth: hello", derivedLines(answer), nil, graphYieldingNewRowsToEveryRecall(), answered(answer))

		for _, round := range run.record.ToolCalls {
			if round.Source == ToolSourceEcho {
				t.Fatalf("%q: round %+v has source echo, want call and block syntax left alone", answer, round)
			}
		}
	}
}

func TestCallLikeTextWithASpaceIsRecoveredAsAnEchoRecallAndNeverDelivered(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"read_node 6840", "recall gangolf colour", "write_file notes today"} {
		run := runEchoing(t, "hello there", "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer), answered(colourReply))

		round := onlyRound(t, run.record)
		if round.Tool != ToolRecall || round.Source != ToolSourceEcho || round.Query != answer {
			t.Fatalf("%q: round = %+v, want it recovered as an echo recall of the text", answer, round)
		}
		if run.record.Answer != colourReply || run.record.NonReply != "" {
			t.Fatalf("%q: answer = %q, nonReply = %q, want the next call's reply and not the text", answer, run.record.Answer, run.record.NonReply)
		}

		again := runEchoing(t, "hello there", "", nil, graphYieldingNewRowsToEveryRecall(), answered(answer), answered(answer))
		if again.record.NonReply != NonReplyQueryEcho || again.record.Outcome.Produced {
			t.Fatalf("%q: written again, nonReply = %q, produced = %t, want it withheld", answer, again.record.NonReply, again.record.Outcome.Produced)
		}
	}
}
