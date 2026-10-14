package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const shownPartID = 71

func neverAnEcho(string) bool { return false }

func graphShowingAPart() *fakeGraph {
	shown := readableNode(shownPartID, "the body the block already carries")
	graph := graphHoldingNodes(shown)
	graph.candidates = []Candidate{{ID: shown.ID, Type: shown.Type, Name: shown.Name, Similarity: 0.9, Content: shown.Content, Substance: shown.Substance}}
	return graph
}

func runShowingAPart(t *testing.T, results ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graphShowingAPart(), model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.Candidates) != 1 || !record.Candidates[0].Included {
		t.Fatalf("test setup error: the block admitted %+v, want part %d shown", record.Candidates, shownPartID)
	}
	return record, model
}

func TestAnAnswerThatIsOnlyTheIdOfAShownPartBecomesARefusedReadAndTheReservedCallAnswers(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"71", "id: 71", "ID:71", "Id:   71", "  71\n", "\tid: 71 \n"} {
		record, model := runShowingAPart(t, answered(answer), answered("It is the escalation policy."))

		round := refusedRound(t, record, errAlreadyShownThisTurn)
		if round.Source != ToolSourceReference || round.NodeID != shownPartID {
			t.Fatalf("%q: round source = %q, node = %d, want %q and %d", answer, round.Source, round.NodeID, ToolSourceReference, shownPartID)
		}
		if record.ModelCalls != 2 || len(model.calls) != 2 {
			t.Fatalf("%q: the turn made %d model calls, want 2: the refused read earns the answering call", answer, record.ModelCalls)
		}
		if len(model.calls[1].Offered) != 0 {
			t.Fatalf("%q: the answering call offered %v, want no tool", answer, model.calls[1].Offered)
		}
		if record.Answer != "It is the escalation policy." || record.NonReply != "" || !record.Outcome.Produced {
			t.Fatalf("%q: answer = %q, nonReply = %q, outcome = %+v, want the second call's reply delivered", answer, record.Answer, record.NonReply, record.Outcome)
		}
		if record.ReservedCall.State != ReservedCallCompleted {
			t.Fatalf("%q: reserved call = %+v, want it completed", answer, record.ReservedCall)
		}
	}
}

func TestAnAnswerThatIsOnlyTheIdOfTheSubjectIsReadThroughTheSamePathAndRefusedAsTheSubject(t *testing.T) {
	t.Parallel()

	record, _ := runShowingAPart(t, answered("id: 42"), answered("final"))

	round := refusedRound(t, record, errReadOfSubject)
	if round.Source != ToolSourceReference || round.NodeID != readSubject {
		t.Fatalf("round source = %q, node = %d, want %q and %d", round.Source, round.NodeID, ToolSourceReference, readSubject)
	}
}

func TestANumberThatWasNotShownIsAReplyAndNoReadIsMade(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"15", "id: 15", "0", "id: 0", "99999999999999999999"} {
		record, model := runShowingAPart(t, answered(answer))

		if len(record.ToolCalls) != 0 || len(model.calls) != 1 {
			t.Fatalf("%q: %d rounds and %d calls, want none and 1: a number that was not shown is a reply", answer, len(record.ToolCalls), len(model.calls))
		}
		if record.Answer != answer || record.NonReply != "" || !record.Outcome.Produced || record.Outcome.Verdict != VerdictDelivered {
			t.Fatalf("%q: answer = %q, nonReply = %q, outcome = %+v, want it delivered", answer, record.Answer, record.NonReply, record.Outcome)
		}
	}
}

func TestAShownIdWithAnythingElseAroundItIsAReply(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"id: 71 is the one.", "the id is 71.", "#71", "node 71", "71.", "id:\n71", "71\n72", "id: 71\nid: 71", "id 71", "71 71", "-71", "+71", "id: 7 1."} {
		record, model := runShowingAPart(t, answered(answer))

		if len(record.ToolCalls) != 0 || len(model.calls) != 1 || record.NonReply != "" || !record.Outcome.Produced {
			t.Fatalf("%q: rounds = %d, calls = %d, nonReply = %q, produced = %t, want a delivered reply", answer, len(record.ToolCalls), len(model.calls), record.NonReply, record.Outcome.Produced)
		}
	}
}

func TestAShownIdAnsweredOnACallThatOfferedNoReadIsWithheldAsPartReference(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"71", "id: 71"} {
		record, model := runShowingAPart(t, wantsRead(shownPartID), answered(answer))

		if len(model.calls) != 2 || len(model.calls[1].Offered) != 0 {
			t.Fatalf("%q: test setup error: the second call offered %v, want a reserved call with no tool", answer, model.calls[len(model.calls)-1].Offered)
		}
		if len(record.ToolCalls) != 1 {
			t.Fatalf("%q: the record carries %d rounds, want only the first call's: the reserved call offers no read to make", answer, len(record.ToolCalls))
		}
		if record.NonReply != NonReplyPartReference || record.Outcome.Produced {
			t.Fatalf("%q: nonReply = %q, produced = %t, want %q withheld", answer, record.NonReply, record.Outcome.Produced, NonReplyPartReference)
		}
		if record.Answer != answer {
			t.Fatalf("record.Answer = %q, want the model's text verbatim: %q", record.Answer, answer)
		}
		if record.Outcome.Verdict != VerdictCurtailed {
			t.Fatalf("%q: verdict = %q, want %q: a reserved call keeps its verdict and the kind rides beside it", answer, record.Outcome.Verdict, VerdictCurtailed)
		}
	}
}

func TestAShownIdAnsweredOnTheCallCapReservedCallIsWithheldWithoutARead(t *testing.T) {
	t.Parallel()

	results := make([]JudgeResult, 0, MaxModelCalls)
	for i := range MaxModelCalls - 1 {
		results = append(results, recallFor("a phrase "+strings.Repeat("x", i+1)))
	}
	results = append(results, answered("id: 71"))

	graph := graphShowingAPart()
	graph.recallQueue = []recallResponse{{Candidates: graph.candidates}}
	for round := range MaxModelCalls {
		graph.recallQueue = append(graph.recallQueue, recallResponse{Candidates: freshRows(round + 1)})
	}
	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !record.CapReached {
		t.Fatalf("test setup error: the cap was not reached: %d calls", record.ModelCalls)
	}
	for _, round := range record.ToolCalls {
		if round.Source == ToolSourceReference {
			t.Fatalf("round %+v is a reference read, want none on a call that offered no read", round)
		}
	}
	if record.NonReply != NonReplyPartReference {
		t.Fatalf("nonReply = %q, want %q", record.NonReply, NonReplyPartReference)
	}
}

func TestAShownIdFromACallThatAlsoNamedAnUnhonouredCallIsToolTextNotAReadRequest(t *testing.T) {
	t.Parallel()

	result := answered("71")
	result.UnhonouredCall = ToolWriteFile

	record, _ := runShowingAPart(t, result)

	if len(record.ToolCalls) != 0 || record.NonReply != NonReplyToolText {
		t.Fatalf("rounds = %d, nonReply = %q, want no round and %q", len(record.ToolCalls), record.NonReply, NonReplyToolText)
	}
}

func TestARecordWithoutAReferenceSerialisesWithoutItsVocabulary(t *testing.T) {
	t.Parallel()

	record, _ := runShowingAPart(t, recallFor("a phrase"), answered("a reply"))

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "reference") || strings.Contains(string(encoded), "partReference") {
		t.Fatalf("the record mentions a reference although the run had none: %s", encoded)
	}
}

func TestAShownLoneIdIsNoLongerDeliveredButAnUnshownOneStillIs(t *testing.T) {
	t.Parallel()

	shown := func(id int64) bool { return id == 12 }

	for answer, want := range map[string]NonReply{
		"id: 12":                 NonReplyPartReference,
		"12":                     NonReplyPartReference,
		"id: 13":                 "",
		"13":                     "",
		"id: 12 is my favourite": "",
		"id: 12\ntype: thought":  NonReplyBlockText,
	} {
		if got := classifyNonReply(answer, false, "", shown, neverAnEcho); got != want {
			t.Errorf("classifyNonReply(%q) = %q, want %q", answer, got, want)
		}
	}
	if got := classifyNonReply("id: 12", true, "", shown, neverAnEcho); got != NonReplyToolRequest {
		t.Errorf("a tool request that wrote a shown id = %q, want %q: the call kinds come first", got, NonReplyToolRequest)
	}
	if got := classifyNonReply("12", false, ToolRecall, shown, neverAnEcho); got != NonReplyToolText {
		t.Errorf("an unhonoured call that wrote a shown id = %q, want %q", got, NonReplyToolText)
	}
}

func TestAToolRequestWhoseContentIsAShownIdKeepsItsNativeRoundAndIsNeverReadAsAReference(t *testing.T) {
	t.Parallel()

	recall := recallFor("a phrase")
	write := wantsWriteOf("notes.md", "what I have so far")
	for name, request := range map[string]JudgeResult{"recall": recall, "write": write} {
		request.Answer = "id: 71"
		request.ToolSource = ToolSourceNative

		model := &fakeModel{results: []JudgeResult{request, answered("a reply")}}
		turn := NewTurn(graphShowingAPart(), model, nil, fixedSystem("system"), "test-model", testLogger())
		turn.Files = &fakeFiles{dir: "/runs/run-1"}
		record, _, err := turn.Run(context.Background(), "hello", readSubject)
		if err != nil {
			t.Fatalf("%s: Run: %v", name, err)
		}

		if len(record.ToolCalls) != 1 {
			t.Fatalf("%s: the record carries %d rounds, want the native request's one: %+v", name, len(record.ToolCalls), record.ToolCalls)
		}
		round := record.ToolCalls[0]
		wantTool := map[string]string{"recall": ToolRecall, "write": ToolWriteFile}[name]
		if round.Tool != wantTool || round.Source != ToolSourceNative {
			t.Fatalf("%s: round = tool %q source %q, want %q and %q: the native request must not be rewritten into a read", name, round.Tool, round.Source, wantTool, ToolSourceNative)
		}
	}
}
