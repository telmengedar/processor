package loop

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

var allTools = []string{ToolRecall, ToolReadNode, ToolWriteFile}

func runRecalls(t *testing.T, graph *fakeGraph, results ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return record, model
}

func recallOf(query string) JudgeResult {
	return JudgeResult{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: query}
}

func answer() JudgeResult {
	return JudgeResult{Answer: "done", Reason: Answered, RawReason: "stop"}
}

func userContentOf(call JudgeInput) string {
	return RenderUserContent(call.Block, call.Input, call.Now, call.Window, call.Conversation, call.Clause, call.Trailer)
}

func TestTheClauseIsTheParagraphBeforeTheFinalRequestWhenRecallIsOffered(t *testing.T) {
	t.Parallel()

	_, model := runRecalls(t, baseGraph(), answer())

	content := userContentOf(model.calls[0])
	tail := recallClause + "\n\n===== INPUT =====\nhello"
	if !strings.HasSuffix(content, tail) {
		t.Fatalf("the user content does not end with the clause, one blank line and the request:\n%q", content)
	}
	if strings.Count(content, recallClause) != 1 {
		t.Fatalf("the clause appears %d times in the user content, want once", strings.Count(content, recallClause))
	}
}

func TestTheClauseNeverEntersTheSystemTextOrTheBlock(t *testing.T) {
	t.Parallel()

	_, model := runRecalls(t, baseGraph(), answer())

	call := model.calls[0]
	if strings.Contains(call.System, recallClause) || strings.Contains(call.Block, recallClause) {
		t.Fatalf("the clause leaked out of the trailer: system=%q block=%q", call.System, call.Block)
	}
}

func TestTheClauseIsTheTextTheDesignMeasured(t *testing.T) {
	t.Parallel()

	const measured = "When none of the parts above states what this request asks for, do not say the information is missing: call the recall tool with a short description of what is missing."
	if recallClause != measured {
		t.Fatalf("recallClause = %q, want the measured text %q", recallClause, measured)
	}
}

func TestTheReservedAnsweringCallCarriesNoTrailer(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	record, model := runRecalls(t, graph, recallsDifferingOnlyByADateSuffix(MaxModelCalls)...)

	if !record.CapReached || len(model.calls) != MaxModelCalls {
		t.Fatalf("the turn did not reach its call cap: capReached=%v calls=%d", record.CapReached, len(model.calls))
	}
	last := model.calls[len(model.calls)-1]
	if len(last.Offered) != 0 {
		t.Fatalf("the reserved call offered %v, want no tool", last.Offered)
	}
	if last.Trailer != "" {
		t.Fatalf("the reserved call carries trailer %q, want none", last.Trailer)
	}
	if last.Clause != "" || strings.Contains(userContentOf(last), recallClause) {
		t.Fatalf("the reserved call carries clause %q, want none: recall is not offered on it", last.Clause)
	}
	if got := record.Trailers[len(record.Trailers)-1]; got != TrailerNone {
		t.Fatalf("the record names the reserved call's trailer %q, want %q", got, TrailerNone)
	}
}

func TestABarrenRecallReplacesTheClauseWithALineNamingTheQuery(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{{}, {}}
	record, model := runRecalls(t, graph, recallOf("the pinkerton theory"), answer())

	if len(model.calls) != 2 {
		t.Fatalf("the turn made %d model calls, want 2", len(model.calls))
	}
	want := `You searched memory in this request for "the pinkerton theory", and it found nothing new. Answer now from what you have; where it does not say, say plainly that you do not know.`
	if got := model.calls[1].Trailer; got != want {
		t.Fatalf("the call after a barren recall carries trailer %q, want %q", got, want)
	}
	if content := userContentOf(model.calls[1]); strings.Contains(content, recallClause) {
		t.Fatalf("the clause is shown again after a barren recall:\n%q", content)
	}
	if want := []TrailerKind{TrailerClause, TrailerBarren}; !slices.Equal(record.Trailers, want) {
		t.Fatalf("record.Trailers = %v, want %v", record.Trailers, want)
	}
}

func TestTheCallAfterAFruitfulRecallCarriesNoClosingParagraphYetTheRecordNamesItFound(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{{}, {Candidates: freshRows(1)}}
	record, model := runRecalls(t, graph, recallOf("rugenforth when stuck"), answer())

	if len(model.calls) != 2 {
		t.Fatalf("the turn made %d model calls, want 2", len(model.calls))
	}
	if got := model.calls[1].Trailer; got != "" {
		t.Fatalf("the call after a fruitful recall carries trailer %q, want none", got)
	}
	content := userContentOf(model.calls[1])
	if !strings.HasSuffix(content, "===== INPUT =====\nhello") {
		t.Fatalf("the user content does not end with the request:\n%q", content)
	}
	if strings.Contains(content, recallClause) || strings.Contains(content, searchedLine) {
		t.Fatalf("the clause or a search line is shown after a fruitful recall:\n%q", content)
	}
	if want := []TrailerKind{TrailerClause, TrailerFound}; !slices.Equal(record.Trailers, want) {
		t.Fatalf("record.Trailers = %v, want %v", record.Trailers, want)
	}
}

func TestARecallThatErroredLeavesTheClauseInPlace(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{{}, {Err: context.DeadlineExceeded}}
	_, model := runRecalls(t, graph, recallOf("anything"), answer())

	if got := model.calls[1].Clause; got != recallClause || model.calls[1].Trailer != "" {
		t.Fatalf("the call after a failed recall carries clause %q and trailer %q, want the clause alone: nothing was searched", got, model.calls[1].Trailer)
	}
}

func TestEveryModelCallHasOneTrailerKindInTheRecord(t *testing.T) {
	t.Parallel()

	graph := graphYieldingNewRowsToEveryRecall()
	record, _ := runRecalls(t, graph, recallsDifferingOnlyByADateSuffix(MaxModelCalls)...)

	want := []TrailerKind{TrailerClause, TrailerFound, TrailerFound, TrailerFound, TrailerFound, TrailerNone}
	if !slices.Equal(record.Trailers, want) {
		t.Fatalf("record.Trailers = %v, want %v", record.Trailers, want)
	}
	if len(record.Trailers) != len(record.Usage) {
		t.Fatalf("the record names %d trailers for %d model calls", len(record.Trailers), len(record.Usage))
	}
}

func TestASuccessfulReadOfANodeIsNotASearchSoTheNextCallStillCarriesTheClause(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes(readableNode(77, "the node body"))
	record, model := runRecalls(t, graph, wantsRead(77), answer())

	if len(record.ToolCalls) != 1 || record.ToolCalls[0].Tool != ToolReadNode || record.ToolCalls[0].Error != "" || record.ToolCalls[0].Yield == 0 {
		t.Fatalf("test setup error: want one successful read that put a row in front of the model, got %+v", record.ToolCalls)
	}
	if got := model.calls[1].Clause; got != recallClause || model.calls[1].Trailer != "" {
		t.Fatalf("the call after a successful read carries clause %q and trailer %q, want the clause alone: nothing was searched", got, model.calls[1].Trailer)
	}
	if want := []TrailerKind{TrailerClause, TrailerClause}; !slices.Equal(record.Trailers, want) {
		t.Fatalf("record.Trailers = %v, want %v", record.Trailers, want)
	}
}

func TestTrailerForOffersNothingWithoutTheRecallTool(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no tool":             nil,
		"an empty tool list":  {},
		"read and write only": {ToolReadNode, ToolWriteFile},
		"write only":          {ToolWriteFile},
	}
	for name, offered := range cases {
		text, kind := trailerFor(offered, nil)
		if text != "" || kind != TrailerNone {
			t.Errorf("%s: trailerFor = (%q, %q), want nothing", name, text, kind)
		}
	}
}

func TestTrailerForWithAToolListThatOffersRecallGivesTheClauseBeforeASearch(t *testing.T) {
	t.Parallel()

	text, kind := trailerFor(allTools, nil)
	if text != recallClause || kind != TrailerClause {
		t.Fatalf("trailerFor = (%q, %q), want the clause", text, kind)
	}
}

func TestTrailerForTakesItsStateFromTheLastSearchOnly(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		searches []recallSearch
		kind     TrailerKind
	}{
		"fruitful then barren": {[]recallSearch{{"a", 3}, {"b", 0}}, TrailerBarren},
		"barren then fruitful": {[]recallSearch{{"a", 0}, {"b", 2}}, TrailerFound},
	}
	for name, tc := range cases {
		if _, kind := trailerFor(allTools, tc.searches); kind != tc.kind {
			t.Errorf("%s: kind = %q, want %q", name, kind, tc.kind)
		}
	}
}

func TestTrailerForAfterAFruitfulSearchIsEmptyAndNamedFound(t *testing.T) {
	t.Parallel()

	cases := map[string][]recallSearch{
		"one fruitful search":             {{"a", 3}},
		"a barren search then a fruitful": {{"a", 0}, {"b", 2}},
		"two fruitful searches":           {{"a", 1}, {"b", 1}},
	}
	for name, searches := range cases {
		text, kind := trailerFor(allTools, searches)
		if text != "" || kind != TrailerFound {
			t.Errorf("%s: trailerFor = (%q, %q), want an empty trailer named %q", name, text, kind, TrailerFound)
		}
	}
}

func TestTrailerForJoinsTheTrailingBarrenQueriesInOrderAndSkipsEarlierFruitfulOnes(t *testing.T) {
	t.Parallel()

	text, _ := trailerFor(allTools, []recallSearch{{"fruitful", 2}, {"first", 0}, {"second", 0}})
	if want := `You searched memory in this request for "first", "second", and it found nothing new.`; !strings.HasPrefix(text, want) {
		t.Fatalf("trailer = %q, want it to start %q", text, want)
	}
	if strings.Contains(text, "fruitful") {
		t.Fatalf("trailer names a query that found rows: %q", text)
	}
}

func TestTrailerForNamesAnEmptyQueryAsItIsRatherThanDroppingIt(t *testing.T) {
	t.Parallel()

	text, _ := trailerFor(allTools, []recallSearch{{"", 0}})
	if !strings.HasPrefix(text, `You searched memory in this request for "", `) {
		t.Fatalf("trailer = %q", text)
	}
}

const trailerTestRequest = "the request \r\n\t "

func TestRenderUserContentWithoutATrailerIsByteIdenticalToTheLayoutBeforeTheTrailer(t *testing.T) {
	t.Parallel()

	instant := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	for _, conversation := range []string{"", "===== EARLIER IN THIS CONVERSATION =====\nuser said: hi"} {
		got := RenderUserContent("BLOCK\n", trailerTestRequest, instant, UpdateWindow{}, conversation, "", "")

		want := "===== INPUT =====\n" + trailerTestRequest + "\n\n===== NOW =====\n2026-03-04T05:06:07Z\n\n\nBLOCK\n"
		if conversation != "" {
			want += "\n\n" + conversation + "\n"
		}
		want += "\n===== INPUT =====\n" + trailerTestRequest

		if got != want {
			t.Fatalf("with no trailer the user content =\n%q\nwant\n%q", got, want)
		}
	}
}

func TestRenderUserContentPutsTheTrailerAfterTheRequestAndChangesNothingBeforeIt(t *testing.T) {
	t.Parallel()

	instant := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	plain := RenderUserContent("BLOCK\n", trailerTestRequest, instant, UpdateWindow{}, "", "", "")
	got := RenderUserContent("BLOCK\n", trailerTestRequest, instant, UpdateWindow{}, "", "", "TRAILER")

	if want := plain + "\n\nTRAILER"; got != want {
		t.Fatalf("user content =\n%q\nwant\n%q", got, want)
	}
}
