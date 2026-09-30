package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const readSubject = 42

func graphHoldingNodes(nodes ...Anchor) *fakeGraph {
	held := map[int64]nodeResponse{
		readSubject: {Anchor: Anchor{ID: readSubject, Type: "documentation", Name: "Subject", Content: "anchor body"}, Found: true},
	}
	for _, node := range nodes {
		held[node.ID] = nodeResponse{Anchor: node, Found: true}
	}
	return &fakeGraph{nodes: held}
}

func readableNode(id int64, content string) Anchor {
	return Anchor{ID: id, Type: "documentation", Name: "Read me", Content: content}
}

func wantsRead(id int64) JudgeResult {
	return JudgeResult{Reason: WantsRead, RawReason: "tool_calls", ReadNodeID: id}
}

func recallFor(query string) JudgeResult {
	return JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: query}
}

func answeredFinal() JudgeResult {
	return answered("final")
}

func runWithResults(t *testing.T, graph *fakeGraph, results ...JudgeResult) Record {
	t.Helper()

	turn := NewTurn(graph, &fakeModel{results: results}, nil, "system", "test-model", testLogger())
	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return record
}

func onlyRound(t *testing.T, record Record) ToolCallRecord {
	t.Helper()

	if len(record.ToolCalls) != 1 {
		t.Fatalf("test setup error: the record carries %d rounds, want exactly one: %+v", len(record.ToolCalls), record.ToolCalls)
	}
	return record.ToolCalls[0]
}

func refusedRound(t *testing.T, record Record, want string) ToolCallRecord {
	t.Helper()

	round := onlyRound(t, record)
	if round.Tool != ToolReadNode {
		t.Fatalf("the round was recorded against tool %q, want %q", round.Tool, ToolReadNode)
	}
	if round.Error != want {
		t.Fatalf("the round carries error %q, want %q: the refusal is the only thing the model is shown for the round it spent", round.Error, want)
	}
	if len(round.Results) != 0 {
		t.Fatalf("a refused round carries %d result rows, want none: exactly one of the refusal and the rows may be set, or which of them renders is decided silently", len(round.Results))
	}
	return round
}

func TestAnAddressedReadsRowRendersItsContentAtEveryThresholdTheFormRuleCanTake(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 1000)
	candidate := candidateFromAnchor(readableNode(71, content))

	if candidate.Substance != "" {
		t.Fatalf("a candidate built from an addressed read carries %d bytes of substance, want none: a read that can return a condensed form is the whole defect this tool exists to close", len(candidate.Substance))
	}

	for _, threshold := range []SubstanceRatio{0, SubstanceRatioFloor / 2, SubstanceRatioFloor, SubstanceRatioFloor * 2, 0.5, 1, 2} {
		admitted, dispositions := admit([]Candidate{candidate}, SupplementaryByteBudget, 0, 0, BlockOccupancy, threshold)

		if len(admitted) != 1 || len(dispositions) != 1 {
			t.Fatalf("at threshold %v the read admitted %d rows and recorded %d, want one of each", threshold, len(admitted), len(dispositions))
		}
		if dispositions[0].Form != FormContent {
			t.Fatalf("at threshold %v the read's row has Form %q, want %q at every value either bound can take", threshold, dispositions[0].Form, FormContent)
		}
		if dispositions[0].RenderedSize != 1000 {
			t.Fatalf("at threshold %v the read's row has RenderedSize %d, want 1000", threshold, dispositions[0].RenderedSize)
		}
		if got := RenderToolResult(ToolExchange{Tool: ToolReadNode, NodeID: 71, Results: admitted, Dispositions: dispositions, SubstanceRatioThreshold: threshold}); !strings.Contains(got, content) {
			t.Fatalf("at threshold %v the round renders %d bytes that do not carry the node's content", threshold, len(got))
		}
	}
}

func TestTurnRunRefusesAnAddressedReadOfAnIdTheGraphDoesNotHold(t *testing.T) {
	t.Parallel()

	record := runWithResults(t, graphHoldingNodes(), wantsRead(71), answeredFinal())

	round := refusedRound(t, record, errNoSuchNode)
	if round.NodeID != 71 {
		t.Fatalf("the refused round records node id %d, want 71: a refusal nobody can attribute to an id teaches the reader nothing", round.NodeID)
	}
}

func TestTurnRunRefusesAnAddressedReadOfTheRequestsOwnSubject(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes()
	record := runWithResults(t, graph, wantsRead(readSubject), answeredFinal())

	refusedRound(t, record, errReadOfSubject)
	if len(graph.nodeCalls) != 1 {
		t.Fatalf("the graph was asked for a node %d times, want once: the subject check needs only the id, and spending a round-trip to learn what the block already carries is the cost this ordering exists to avoid", len(graph.nodeCalls))
	}
}

func TestTurnRunRefusesASecondAddressedReadOfANodeAlreadyShownInFull(t *testing.T) {
	t.Parallel()

	shown := readableNode(71, "the body the block already carries")
	graph := graphHoldingNodes(shown)
	graph.candidates = []Candidate{{ID: shown.ID, Type: shown.Type, Name: shown.Name, Similarity: 0.9, Content: shown.Content}}

	record := runWithResults(t, graph, wantsRead(shown.ID), answeredFinal())

	if len(record.Candidates) != 1 || !record.Candidates[0].Included {
		t.Fatalf("test setup error: the block admitted %+v, want the one row admitted or the account never saw it", record.Candidates)
	}

	round := refusedRound(t, record, errAlreadyReadInFull)
	if round.Bytes != 0 {
		t.Fatalf("the refused round charged %d bytes, want none: refusing costs one round and one sentence, which is the whole reason it beats serving the bytes again", round.Bytes)
	}
	if record.ModelCalls != 2 {
		t.Fatalf("the turn spent %d model calls, want 2: a refused read still costs the round it asked on", record.ModelCalls)
	}
}

func TestTurnRunRefusesAnAddressedReadLargerThanTheBlockBudgetAndNamesBothByteCountsInThatOrder(t *testing.T) {
	t.Parallel()

	const wantRefusal = "that part is 30001 bytes and one read carries at most 30000, so it cannot be shown in full"

	if AssemblyByteBudget != 30_000 {
		t.Fatalf("test setup error: one read carries %d bytes, and the sentence this guard expects spells the pair as literals on purpose, because an expectation built from the same constant the sentence is built from reads the same whichever order the two numbers are formatted in", AssemblyByteBudget)
	}

	oversized := readableNode(71, strings.Repeat("x", 30_001))
	record := runWithResults(t, graphHoldingNodes(oversized), wantsRead(oversized.ID), answeredFinal())

	refusedRound(t, record, wantRefusal)
}

func TestTurnRunRefusesAnAddressedReadTheGraphCouldNotServeRatherThanFailingTheRun(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes()
	graph.nodes[71] = nodeResponse{Err: errors.New("the graph is unreachable")}

	record := runWithResults(t, graph, wantsRead(71), answeredFinal())

	round := onlyRound(t, record)
	if !strings.Contains(round.Error, "the graph is unreachable") {
		t.Fatalf("the round carries error %q, want the transport cause carried to the model", round.Error)
	}
	if record.Answer != "final" {
		t.Fatalf("record.Answer = %q, want the answer the turn went on to produce: a graph that cannot serve one read degrades to a refusal, never to a failed run", record.Answer)
	}
}

func TestTurnRunRecordsAnAddressedReadsRowAsContentFormCarryingNoSubstance(t *testing.T) {
	t.Parallel()

	served := readableNode(71, strings.Repeat("x", 1200))
	record := runWithResults(t, graphHoldingNodes(served), wantsRead(served.ID), answeredFinal())

	round := onlyRound(t, record)
	if len(round.Results) != 1 {
		t.Fatalf("the served round records %d rows, want one; error %q", len(round.Results), round.Error)
	}

	row := round.Results[0]
	if !row.Included {
		t.Fatalf("the read's row was cut for %q, want it admitted", row.CutReason)
	}
	if row.Form != FormContent {
		t.Fatalf("the read's row has Form %q, want %q", row.Form, FormContent)
	}
	if row.SubstanceAvailable || row.SubstanceSize != 0 {
		t.Fatalf("the read's row reports a substance of %d bytes available %v, want neither: an addressed read fetches a body the graph never condensed", row.SubstanceSize, row.SubstanceAvailable)
	}
	if row.Size != 1200 || row.RenderedSize != 1200 {
		t.Fatalf("the read's row records Size %d at RenderedSize %d, want 1200 for both", row.Size, row.RenderedSize)
	}
}

func TestTurnRunServesInFullByAddressedReadARowItsBlockShowedAsSubstance(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("x", 25_000)
	substance := strings.Repeat("z", 2_000)
	graph := graphHoldingNodes(readableNode(10, content))
	graph.candidates = []Candidate{{ID: 10, Type: "documentation", Name: "Bravo", Similarity: 0.9, Content: content, Substance: substance}}
	model := &fakeModel{results: []JudgeResult{wantsRead(10), answeredFinal()}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(record.Candidates) != 1 || !record.Candidates[0].Included || record.Candidates[0].Form != FormSubstance {
		t.Fatalf("test setup error: the block's disposition is %+v, want the one candidate admitted as substance", record.Candidates)
	}

	round := onlyRound(t, record)
	if round.Tool != ToolReadNode || round.Error != "" {
		t.Fatalf("the read round is %+v, want it served with no refusal", round)
	}
	if len(round.Results) != 1 || round.Results[0].Form != FormContent || round.Results[0].RenderedSize != len(content) {
		t.Fatalf("the read round recorded %+v, want the row admitted as content at its full length", round.Results)
	}

	if len(model.calls) < 2 || len(model.calls[1].PriorTools) != 1 {
		t.Fatalf("test setup error: the second judgement carries %d completed rounds, want 1", len(model.calls[1].PriorTools))
	}
	rendered := RenderToolResult(model.calls[1].PriorTools[0])
	if !strings.Contains(rendered, content) {
		t.Fatal("the read did not return the row's full content")
	}
	if strings.Contains(rendered, substance) {
		t.Fatal("the read returned the substance the block showed, not the full content it exists to recover")
	}
	if strings.Contains(rendered, "\n"+formHeaderKey+":") {
		t.Fatal("the read result carries a form marking; an addressed read is always content and never marked")
	}
}

func TestTurnRunRecordsTheIdOfEveryAddressedReadRoundItDispatched(t *testing.T) {
	t.Parallel()

	first := readableNode(71, "first body")
	second := readableNode(72, "second body")
	record := runWithResults(t, graphHoldingNodes(first, second), wantsRead(first.ID), wantsRead(second.ID), answeredFinal())

	if len(record.ToolCalls) != 2 {
		t.Fatalf("test setup error: the record carries %d rounds, want two: %+v", len(record.ToolCalls), record.ToolCalls)
	}
	for i, want := range []int64{first.ID, second.ID} {
		round := record.ToolCalls[i]
		if round.Error != "" {
			t.Fatalf("round %d was refused with %q, want it served", i+1, round.Error)
		}
		if round.NodeID != want {
			t.Fatalf("round %d records node id %d, want %d: without it no reader can tell which part a read round fetched", i+1, round.NodeID, want)
		}
	}
}

func TestAnAddressedReadRoundNeverCountsTowardsClosingRecall(t *testing.T) {
	t.Parallel()

	served := readableNode(71, "a body only the read reaches")
	graph := graphHoldingNodes(served)
	graph.candidates = repeatedRows()

	record := runWithResults(t, graph,
		recallFor("what the block already carries"),
		wantsRead(served.ID),
		recallFor("what the block already carries, again"),
		recallFor("what the block already carries, a third time"),
		recallFor("the round the closure is read on"),
	)

	if len(record.ToolCalls) < 2 || record.ToolCalls[0].Tool != ToolRecall || record.ToolCalls[0].Error != "" || record.ToolCalls[0].Yield != 0 {
		t.Fatalf("test setup error: the first supplementary round is %+v, want a dispatched recall whose yield is 0 — unless a barren recall really precedes the read, the state this guard is named for is never built and its closure assertion cannot fail", record.ToolCalls)
	}
	if read := record.ToolCalls[1]; read.Tool != ToolReadNode || read.Error != "" || read.Yield != 1 {
		t.Fatalf("the read round is recorded as %+v, want a served read whose yield is 1", read)
	}

	if !record.RecallClosed {
		t.Fatalf("recall never closed, so this guard observed the closing rule on no round at all: %+v", record.ToolCalls)
	}
	if len(record.ToolCalls) != 5 {
		t.Fatalf("recall closed after %d rounds, want 5 — one barren recall, the read, two more barren recalls and the round the refusal was shown on. Closing sooner means the read was counted towards the barren run it interrupted, and the rule closes on two *consecutive* barren rounds", len(record.ToolCalls))
	}
	if want := []int{0, 0, 0}; !sameInts(yieldsOf(record), want) {
		t.Fatalf("the dispatched recalls recorded yields %v, want %v: all three must be barren, or the round that closed recall was not a barren one", yieldsOf(record), want)
	}
	if closed := record.ToolCalls[4]; closed.Error != errRecallClosedToModel {
		t.Fatalf("the last round carries error %q, want the sentence the model was shown when recall closed", closed.Error)
	}
}

func TestAServedAddressedReadPutsItsRowInFrontOfTheModelForTheFirstTimeInTheTurn(t *testing.T) {
	t.Parallel()

	content := "the whole body"
	condensed := Disposition{ID: 71, ContentHash: contentHash(content), Form: FormSubstance, Included: true}
	inFull := Disposition{ID: 71, ContentHash: contentHash(content), Form: FormContent, Included: true}

	account := newYieldAccount([]Disposition{condensed})

	if account.alreadyShown(inFull) {
		t.Fatal("a row the block showed only as a condensed form counts as already shown in full, so the one round that genuinely adds bytes would be refused")
	}
	if yield := account.round([]Disposition{inFull}); yield != 1 {
		t.Fatalf("reading in full a row the block showed condensed yielded %d, want 1: the account keys on the form as well as the body, or the round that adds the missing bytes scores as barren", yield)
	}
	if !account.alreadyShown(inFull) {
		t.Fatal("a row just shown in full is not recorded as shown in full, so a second read of it would be served rather than refused")
	}
}

func TestATurnDispatchesAReadWantingTerminalToTheAddressedReadAndNotToRecall(t *testing.T) {
	t.Parallel()

	served := readableNode(71, "a body")
	graph := graphHoldingNodes(served)
	record := runWithResults(t, graph, wantsRead(served.ID), answeredFinal())

	round := onlyRound(t, record)
	if round.Tool != ToolReadNode {
		t.Fatalf("a read-wanting terminal was dispatched as tool %q, want %q", round.Tool, ToolReadNode)
	}
	if round.Query != "" {
		t.Fatalf("the read round records query %q, want none: dispatched as a recall it would search for the empty string and record that it had", round.Query)
	}
	if len(graph.recallCalls) != primaryRecallCalls {
		t.Fatalf("the graph served %d recalls, want the %d the turn's own assembly issues: a read must not reach the recall path", len(graph.recallCalls), primaryRecallCalls)
	}
	if got := graph.nodeCalls; len(got) != 2 || got[1] != served.ID {
		t.Fatalf("the graph was asked for nodes %v, want the subject then the id the model named", got)
	}
}

func TestARefusedAddressedReadRendersItsReasonAndCarriesNoResultRow(t *testing.T) {
	t.Parallel()

	refused := ToolExchange{Tool: ToolReadNode, NodeID: 71, Error: errNoSuchNode, Dispositions: []Disposition{}}

	rendered := RenderToolResult(refused)
	if rendered != "error: "+errNoSuchNode {
		t.Fatalf("a refused read renders as %q, want the refusal on the branch that already carries one", rendered)
	}
	if len(refused.Results) != 0 {
		t.Fatalf("test setup error: the refused exchange carries %d rows", len(refused.Results))
	}

	served := readableNode(71, "a body")
	round := onlyRound(t, runWithResults(t, graphHoldingNodes(served), wantsRead(served.ID), answeredFinal()))
	if round.Error != "" || len(round.Results) != 1 {
		t.Fatalf("a served read carries error %q over %d rows, want no error and one row: exactly one of the two may ever be set, because which of them renders is decided by whether the error is empty", round.Error, len(round.Results))
	}
}

func TestAnAddressedReadOfARecordThisSystemWroteIsServedBecauseItsIdWasNamedRatherThanRanked(t *testing.T) {
	t.Parallel()

	own := readableNode(71, "a record this system wrote")
	graph := graphHoldingNodes(own)
	graph.candidates = []Candidate{{ID: own.ID, Type: own.Type, Name: own.Name, Similarity: 0.9, Content: own.Content, SelfProduced: true}}

	record := runWithResults(t, graph, wantsRead(own.ID), answeredFinal())

	if len(record.Candidates) != 0 {
		t.Fatalf("test setup error: retrieval kept %+v, want the self-produced row rejected before a slot was spent, or the read proves nothing about the exclusion", record.Candidates)
	}

	round := onlyRound(t, record)
	if round.Error != "" || len(round.Results) != 1 || !round.Results[0].Included {
		t.Fatalf("reading a self-produced record by id was refused with %q over %+v: the exclusion keeps such a row from displacing a ranked candidate, and a named id displaces nothing", round.Error, round.Results)
	}
}

func TestTheRenderedAccountNamesAnAddressedReadRoundByItsIdRatherThanAnEmptyPath(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{
		{Tool: ToolReadNode, Source: ToolSourceNative, NodeID: 71, Results: []Disposition{{Rank: 1, ID: 71, Size: 1200, RenderedSize: 1200, Included: true}}},
		{Tool: ToolReadNode, Source: ToolSourceNative, NodeID: 72, Error: errNoSuchNode, Results: []Disposition{}},
	}

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"1 " + ToolReadNode + " [native]  #71",
		"2 " + ToolReadNode + " [native]  #72",
		"ERROR: " + errNoSuchNode,
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry %q; a read round falling to the write branch prints an empty path and no id, and a reader cannot then tell what was read.\nsummary:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, ToolReadNode+" [native]  \"\"") {
		t.Fatalf("a read round is rendered with an empty quoted path.\nsummary:\n%s", summary)
	}
}

func TestAnAddressedReadWantedOnTheReservedCallIsRefusedAndNamedAsTheReadTool(t *testing.T) {
	t.Parallel()

	nodes := make([]Anchor, 0, MaxModelCalls)
	results := make([]JudgeResult, 0, MaxModelCalls)
	for i := range MaxModelCalls {
		id := int64(100 + i)
		nodes = append(nodes, readableNode(id, "body of the part asked for"))
		results = append(results, wantsRead(id))
	}

	graph := graphHoldingNodes(nodes...)
	model := &fakeModel{ignoresWithheldToolList: true, results: results}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !record.CapReached || len(record.ToolCalls) != MaxModelCalls {
		t.Fatalf("test setup error: the turn recorded %d rounds with the cap reached %v, want %d rounds and the cap reached", len(record.ToolCalls), record.CapReached, MaxModelCalls)
	}

	refused := record.ToolCalls[len(record.ToolCalls)-1]
	if refused.Error != errReservedCallRefused {
		t.Fatalf("the round the reserved call asked for carries error %q, want the loop's own sentence", refused.Error)
	}
	if refused.Tool != ToolReadNode {
		t.Fatalf("a read the reserved call asked for is recorded against tool %q, want %q: named as a recall, the record says the model asked for a search it never asked for", refused.Tool, ToolReadNode)
	}
	if refused.NodeID != int64(100+MaxModelCalls-1) {
		t.Fatalf("the refused round records node id %d, want %d: the id the model asked for is the only thing that says what it wanted", refused.NodeID, 100+MaxModelCalls-1)
	}
}

func TestTheOutcomeCountsAnAddressedReadRoundAndTheRowItGrounded(t *testing.T) {
	t.Parallel()

	served := readableNode(71, "the body the answer rests on")
	graph := graphHoldingNodes(served)
	graph.candidates = nil

	record := runWithResults(t, graph, wantsRead(served.ID), answeredFinal())

	if len(record.Candidates) != 0 {
		t.Fatalf("test setup error: the block admitted %+v, want nothing so that only the read can ground the answer", record.Candidates)
	}
	if got := record.Outcome.Acted[ToolReadNode]; got != 1 {
		t.Fatalf("the outcome counted %d rounds against %q, want 1: the account of what a run did is derived from the tool name alone, and a new tool must need no new arithmetic", got, ToolReadNode)
	}
	if !record.Outcome.Grounded {
		t.Fatalf("the outcome reports the answer ungrounded although a read put an admitted row in front of the model: %+v", record.Outcome)
	}
	if record.Outcome.Verdict != VerdictDelivered {
		t.Fatalf("record.Outcome.Verdict = %q, want %q", record.Outcome.Verdict, VerdictDelivered)
	}
}

func TestTurnRunRefusesAnAddressedReadTheAdapterRejectedWithItsOwnCauseAndAsksTheGraphForNothing(t *testing.T) {
	t.Parallel()

	const cause = "tool arguments named no node id to read"

	graph := graphHoldingNodes(readableNode(71, "a body this turn must never fetch"))
	rejected := JudgeResult{Reason: WantsRead, RawReason: "tool_calls", ToolError: cause}

	record := runWithResults(t, graph, rejected, answeredFinal())

	round := refusedRound(t, record, cause)
	if round.NodeID != 0 {
		t.Fatalf("the refused round records node id %d, want 0: the adapter rejected the call precisely because it named none", round.NodeID)
	}
	if len(graph.nodeCalls) != 1 || graph.nodeCalls[0] != readSubject {
		t.Fatalf("the graph was asked for %v, want the subject alone: a read the adapter already rejected carries no id, so fetching one spends a round-trip to learn nothing and then answers with a cause that sends the model to try a different id when it named none at all", graph.nodeCalls)
	}
}

func TestTurnRunRefusesAReadOfANodeLargerThanTheBlockCouldCarryEvenWhereItWasAlsoCutFromTheBlockItself(t *testing.T) {
	t.Parallel()

	const tooLarge = AssemblyByteBudget + 1

	big := readableNode(71, strings.Repeat("x", tooLarge))
	graph := graphHoldingNodes(big)
	graph.candidates = []Candidate{{ID: big.ID, Type: big.Type, Name: big.Name, Similarity: 0.9, Content: big.Content}}

	record := runWithResults(t, graph, wantsRead(big.ID), answeredFinal())

	if len(record.Candidates) != 1 || record.Candidates[0].Included {
		t.Fatalf("test setup error: the block's disposition of the row is %+v, want it cut for size, or a cut row cannot be told apart here from one already shown", record.Candidates)
	}

	refusedRound(t, record, fmt.Sprintf(errNodeTooLargeFormat, tooLarge, AssemblyByteBudget))
}

func TestOnlyTheRetrievalToolsAreAccountedSoAFileWriteCannotCloseRecall(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = repeatedRows()
	files := &fakeFiles{dir: "/runs/run-1"}

	model := &fakeModel{results: []JudgeResult{
		recallFor("what the block already carries"),
		wantsWriteOf("notes.md", "what I have so far"),
		recallFor("what the block already carries, again"),
		recallFor("the round the closure is read on"),
	}}
	turn := NewTurn(graph, model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) < 2 || record.ToolCalls[0].Yield != 0 || record.ToolCalls[1].Tool != ToolWriteFile {
		t.Fatalf("test setup error: the first two rounds are %+v, want a barren recall then a file write", record.ToolCalls)
	}
	if !record.RecallClosed {
		t.Fatalf("recall never closed, so this guard observed the closing rule on no round at all: %+v", record.ToolCalls)
	}
	if len(record.ToolCalls) != 4 {
		t.Fatalf("recall closed after %d rounds, want 4 — two barren recalls with a file write between them and the round the refusal was shown on. Closing sooner means the write was accounted as a retrieval round: it carries no rows, so it would score as barren and a file write alone could shut recall for the turn", len(record.ToolCalls))
	}
	if write := record.ToolCalls[1]; write.Yield != 0 {
		t.Fatalf("the file-write round is recorded with yield %d, want 0: a round that retrieves nothing puts no row in front of the model and has no yield to report", write.Yield)
	}
}

func TestAFileWriteWantedOnTheReservedCallIsStillNamedAsTheFileWrite(t *testing.T) {
	t.Parallel()

	results := make([]JudgeResult, 0, MaxModelCalls)
	for range MaxModelCalls {
		results = append(results, wantsWriteOf("notes.md", "what I have so far"))
	}

	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{ignoresWithheldToolList: true, results: results}
	turn := NewTurn(baseGraph(), model, files, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !record.CapReached || len(record.ToolCalls) != MaxModelCalls {
		t.Fatalf("test setup error: the turn recorded %d rounds with the cap reached %v, want %d rounds and the cap reached", len(record.ToolCalls), record.CapReached, MaxModelCalls)
	}

	refused := record.ToolCalls[len(record.ToolCalls)-1]
	if refused.Error != errReservedCallRefused {
		t.Fatalf("the round the reserved call asked for carries error %q, want the loop's own sentence", refused.Error)
	}
	if refused.Tool != ToolWriteFile {
		t.Fatalf("a write the reserved call asked for is recorded against tool %q, want %q: widening this naming to a third tool must not cost the two it already served", refused.Tool, ToolWriteFile)
	}
	if refused.Path != "notes.md" {
		t.Fatalf("the refused round records path %q, want the path the model asked for", refused.Path)
	}
}

func TestAServedAddressedReadRendersTheWholePartWithItsIdTypeAndNameAndNoFormMarking(t *testing.T) {
	t.Parallel()

	body := "the part's whole body, which is what a read is for"
	served := readableNode(71, body)
	graph := graphHoldingNodes(served)
	model := &fakeModel{results: []JudgeResult{wantsRead(served.ID), answeredFinal()}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round := onlyRound(t, record); round.Error != "" || len(round.Results) != 1 {
		t.Fatalf("test setup error: the round carries error %q over %d rows, want one served row", round.Error, len(round.Results))
	}
	if len(model.calls) < 2 || len(model.calls[1].PriorTools) != 1 {
		t.Fatalf("test setup error: the second judgement carries %d completed rounds, want 1", len(model.calls[1].PriorTools))
	}

	rendered := RenderToolResult(model.calls[1].PriorTools[0])

	for _, want := range []string{"id: 71", "type: documentation", "name: Read me", body} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("the text the model is shown for the read round does not carry %q. A round the record says admitted a row, rendered as anything but that row, is the charge-versus-render divergence in its purest form: the budget was spent and the model got nothing.\nrendered:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, formHeaderKey+":") {
		t.Fatalf("the read's section carries a %q line; an addressed read returns the part whole, so there is no form to declare.\nrendered:\n%s", formHeaderKey, rendered)
	}
}

func TestTheRefusalsAnAddressedReadCanReturnAreTheSentencesTheModelIsShown(t *testing.T) {
	t.Parallel()

	shown := readableNode(71, "the body the block already carries")
	withBlockRow := func() *fakeGraph {
		graph := graphHoldingNodes(shown)
		graph.candidates = []Candidate{{ID: shown.ID, Type: shown.Type, Name: shown.Name, Similarity: 0.9, Content: shown.Content}}
		return graph
	}

	cases := []struct {
		name  string
		graph *fakeGraph
		want  string
		asked int64
	}{
		{"an id the graph does not hold", graphHoldingNodes(), "no part of memory has that id", 72},
		{"the request's own subject", graphHoldingNodes(), "that id is this request's own subject, and the context block already carries it in full", readSubject},
		{"a part already shown in full", withBlockRow(), "that part has already been shown to you in full in this turn", shown.ID},
	}

	for _, c := range cases {
		record := runWithResults(t, c.graph, wantsRead(c.asked), answeredFinal())
		round := onlyRound(t, record)
		if round.Error != c.want {
			t.Errorf("%s is refused with %q, want %q verbatim. A refusal is product text the model acts on, so the wording is pinned against a literal here rather than against the constant the message is built from", c.name, round.Error, c.want)
		}
	}
}

func TestTurnRunBoundsAnAddressedReadsCauseBeforeItReachesTheRound(t *testing.T) {
	t.Parallel()

	t.Run("the cause the adapter handed the round", func(t *testing.T) {
		t.Parallel()

		cause := strings.Repeat("e", CarriedCauseRunes+400)
		record := runWithResults(t, graphHoldingNodes(),
			JudgeResult{Reason: WantsRead, RawReason: "tool_calls", ToolError: cause},
			answeredFinal(),
		)

		round := onlyRound(t, record)
		if n := len([]rune(round.Error)); n != CarriedCauseRunes {
			t.Fatalf("the round carried %d runes of a %d-rune adapter cause, want it bounded to %d like every other cause the prompt carries", n, len([]rune(cause)), CarriedCauseRunes)
		}
	})

	t.Run("the cause the graph handed the round", func(t *testing.T) {
		t.Parallel()

		cause := strings.Repeat("f", CarriedCauseRunes+400)
		graph := graphHoldingNodes()
		graph.nodes[71] = nodeResponse{Err: errors.New(cause)}

		record := runWithResults(t, graph, wantsRead(71), answeredFinal())

		round := onlyRound(t, record)
		if n := len([]rune(round.Error)); n != CarriedCauseRunes {
			t.Fatalf("the round carried %d runes of a %d-rune graph cause, want it bounded to %d: a read is the second dispatch to carry a foreign string into the prompt, and the bound is stated for the first one by name", n, len([]rune(cause)), CarriedCauseRunes)
		}
	})
}
