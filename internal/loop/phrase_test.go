package loop

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

const historyToken = "zqxjwv-the-earlier-line"

func phraseOf(bytes int) string {
	return strings.Repeat("x", bytes)
}

func multibytePhrase(bytes int) string {
	return strings.Repeat("ä", bytes/2)
}

func runPhraseTurn(t *testing.T, model *fakeModel, input string, history []Exchange) (*fakeGraph, Record, error) {
	t.Helper()

	graph := derivingGraph()
	record, _, err := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger()).RunWithHistory(context.Background(), input, 42, history)
	return graph, record, err
}

func TestAnInputOfExactlyThePhraseBoundInBytesMadeOfMultibyteCharactersIsSentVerbatimAsTheQuery(t *testing.T) {
	t.Parallel()

	input := " " + multibytePhrase(MaxPhraseBytes-2) + " "
	if len(input) != MaxPhraseBytes || len([]rune(input)) >= MaxPhraseBytes {
		t.Fatalf("test setup error: input is %d bytes and %d runes", len(input), len([]rune(input)))
	}

	graph, record, err := runPhraseTurn(t, &fakeModel{deriveErr: errors.New("no derivation in this fixture")}, input, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := recalledQueries(graph); !slices.Equal(got, []string{input}) {
		t.Fatalf("the graph was asked %q, want the input verbatim, padding included: a phrase at the bound is a query", got)
	}
	if record.InputNotPhrase != "" || record.Query != input {
		t.Fatalf("the record names a phrase input as not a query (%q) or lost its query (%q)", record.InputNotPhrase, record.Query)
	}
}

func TestAnInputOneByteOverThePhraseBoundWithFewerRunesThanTheBoundIsNotSentAsAQuery(t *testing.T) {
	t.Parallel()

	input := multibytePhrase(MaxPhraseBytes) + "x"
	if len(input) != MaxPhraseBytes+1 || len([]rune(input)) >= MaxPhraseBytes {
		t.Fatalf("test setup error: input is %d bytes and %d runes", len(input), len([]rune(input)))
	}

	graph, _, err := runPhraseTurn(t, &fakeModel{deriveErr: errors.New("no derivation in this fixture")}, input, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != 0 {
		t.Fatalf("the graph received %d recall requests with queries %q, want none: the bound is in bytes, and the request carries bytes", len(graph.recallCalls), recalledQueries(graph))
	}
}

func TestALongInputWithDerivationFailedCompletesTheTurnWithNoSemanticFirstLookupAndSaysWhy(t *testing.T) {
	t.Parallel()

	input := phraseOf(4096)
	model := &fakeModel{deriveErr: errors.New("the derivation endpoint is down")}

	graph, record, err := runPhraseTurn(t, model, input, nil)
	if err != nil {
		t.Fatalf("Run: %v, want a completed turn: a long input is content and never fails the turn", err)
	}

	if len(graph.recallCalls) != 0 {
		t.Fatalf("the graph received %d recall requests with queries %q, want none", len(graph.recallCalls), recalledQueries(graph))
	}
	if record.Queries == nil || len(record.Queries) != 0 {
		t.Fatalf("record.Queries = %#v, want an empty, non-nil list", record.Queries)
	}
	if record.Query != "" {
		t.Fatalf("record.Query is %d bytes, want empty: the input was not a query", len(record.Query))
	}
	if !strings.Contains(record.InputNotPhrase, "4096") || !strings.Contains(record.InputNotPhrase, "256") {
		t.Fatalf("record.InputNotPhrase = %q, want it to name the input's byte length and the bound", record.InputNotPhrase)
	}
	if record.DerivationError == "" {
		t.Fatalf("the record carries no derivation cause for a failed derivation")
	}
	if len(model.calls) == 0 || model.calls[0].Input != input {
		t.Fatalf("the model was not handed the whole long input: a long input is content for the model")
	}
	if !strings.Contains(record.Block, "anchor body") {
		t.Fatalf("the block does not carry the topic node: %q", record.Block)
	}
}

func TestALongInputWithDerivationOnIsSearchedByTheDerivedPhrasesAloneInOrder(t *testing.T) {
	t.Parallel()

	input := phraseOf(3000)
	model := &fakeModel{derivedText: "first derived phrase?\nsecond derived phrase?\nthird derived keywords"}

	graph, record, err := runPhraseTurn(t, model, input, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"first derived phrase?", "second derived phrase?", "third derived keywords"}
	if !slices.Equal(record.Queries, want) {
		t.Fatalf("record.Queries = %q, want exactly the derived phrases %q", record.Queries, want)
	}
	if got := recalledQueries(graph); !slices.Equal(got, want) {
		t.Fatalf("the graph was asked %q, want exactly the derived phrases %q and not the input", got, want)
	}
	if record.InputNotPhrase == "" {
		t.Fatalf("the record does not say the input was not sent as a query")
	}
}

func TestADerivedLineLongerThanAPhraseIsDroppedAndTheOthersKeepTheirOrder(t *testing.T) {
	t.Parallel()

	atBound := phraseOf(MaxPhraseBytes)
	text := "short one?\n" + phraseOf(MaxPhraseBytes+1) + "\n" + atBound + "\nshort two?"

	derived, _ := ParseDerivation(text, "the input", nil)

	if want := []string{"short one?", atBound, "short two?"}; !slices.Equal(derived, want) {
		t.Fatalf("derived = %q, want the over-long line dropped, the line at the bound kept, and the order unchanged: %q", derived, want)
	}
}

func TestAnOverlongDerivedLineDoesNotCountTowardTheDerivedQueryCap(t *testing.T) {
	t.Parallel()

	lines := []string{phraseOf(MaxPhraseBytes + 1)}
	for i := range MaxDerivedQueries {
		lines = append(lines, "phrase "+string(rune('a'+i))+"?")
	}

	derived, _ := ParseDerivation(strings.Join(lines, "\n"), "the input", nil)

	if len(derived) != MaxDerivedQueries {
		t.Fatalf("derived %d queries from %d lines of which one was over-long, want %d", len(derived), len(lines), MaxDerivedQueries)
	}
}

func recallingModel(query string) *fakeModel {
	return &fakeModel{
		deriveErr: errors.New("no derivation in this fixture"),
		results: []JudgeResult{
			{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: query},
			{Answer: "done", Reason: Answered, RawReason: "stop"},
		},
	}
}

func TestAModelRecallQueryOverThePhraseBoundMakesNoGraphCallAndTheRefusalStatesTheBound(t *testing.T) {
	t.Parallel()

	query := phraseOf(MaxPhraseBytes + 1)

	graph, record, err := runPhraseTurn(t, recallingModel(query), "the input", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != primaryRecallCalls {
		t.Fatalf("the graph received %d recall requests, want only the first lookup", len(graph.recallCalls))
	}
	call := record.ToolCalls[0]
	if !strings.Contains(call.Error, "is 257 bytes") || !strings.Contains(call.Error, "at most 256") {
		t.Fatalf("the refusal = %q, want it to say the query is 257 bytes and the bound is at most 256, each against its own number", call.Error)
	}
	if strings.Index(call.Error, "257 bytes") > strings.Index(call.Error, "at most 256") {
		t.Fatalf("the refusal = %q, want the length named before the bound", call.Error)
	}
	if strings.Contains(call.Error, "Name in a few words") {
		t.Fatalf("the refusal = %q, want no instruction: the model never reads it", call.Error)
	}
	if call.Query != "" {
		t.Fatalf("the refused query is recorded on the tool call (%d bytes), want it left off the record", len(call.Query))
	}
	if record.ReservedCall.State != ReservedCallCompleted {
		t.Fatalf("reserved call state = %q, want %q: a refused phrase takes the refused-call path, which reserves the next call for answering", record.ReservedCall.State, ReservedCallCompleted)
	}
	if call.Yield != 0 || record.RecallClosed {
		t.Fatalf("the refused call counted as a barren round: yield %d, recall closed %v", call.Yield, record.RecallClosed)
	}
}

func TestAModelRecallQueryOfExactlyThePhraseBoundIsDispatchedVerbatim(t *testing.T) {
	t.Parallel()

	query := multibytePhrase(MaxPhraseBytes)

	graph, record, err := runPhraseTurn(t, recallingModel(query), "the input", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != primaryRecallCalls+1 {
		t.Fatalf("the graph received %d recall requests, want the first lookup and the model's own", len(graph.recallCalls))
	}
	if got := graph.recallCalls[primaryRecallCalls].Queries; !slices.Equal(got, []string{query}) {
		t.Fatalf("the supplementary recall carried %q, want the model's query verbatim", got)
	}
	if record.ReservedCall.State != "" {
		t.Fatalf("reserved call state = %q after a dispatched recall, want none: only a refusal reserves the next call", record.ReservedCall.State)
	}
	if record.ToolCalls[0].Error != "" {
		t.Fatalf("a query at the bound was refused: %q", record.ToolCalls[0].Error)
	}
}

func TestABlankModelRecallQueryIsRefusedWithoutAGraphCall(t *testing.T) {
	t.Parallel()

	graph, record, err := runPhraseTurn(t, recallingModel("  \t "), "the input", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != primaryRecallCalls {
		t.Fatalf("the graph received %d recall requests, want only the first lookup", len(graph.recallCalls))
	}
	if !strings.Contains(record.ToolCalls[0].Error, "blank") {
		t.Fatalf("the refusal = %q, want it to say the query is blank", record.ToolCalls[0].Error)
	}
	if !strings.HasSuffix(record.ToolCalls[0].Error, "nothing was looked up") {
		t.Fatalf("the refusal = %q, want it to end at the fact that nothing was looked up, with no instruction after it: the model never reads it", record.ToolCalls[0].Error)
	}
}

func TestNoEarlierUserLineReachesAnyGraphQueryOnTheFirstLookupOrTheSupplementaryRecall(t *testing.T) {
	t.Parallel()

	history := []Exchange{{Input: "what did " + historyToken + " say", Reply: "an earlier reply"}}
	model := recallingModel("a resolved phrase naming the person")
	model.deriveErr = nil
	model.derivedText = "a derived angle?"

	graph, record, err := runPhraseTurn(t, model, "what about him", history)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != primaryRecallCalls+1 {
		t.Fatalf("the graph received %d recall requests, want the first lookup and the supplementary recall", len(graph.recallCalls))
	}
	for _, query := range recalledQueries(graph) {
		if strings.Contains(query, historyToken) {
			t.Fatalf("a graph query carries the earlier user line: %q", query)
		}
	}
	if len(record.History) != 1 || !strings.Contains(model.calls[0].Conversation, historyToken) {
		t.Fatalf("test setup error: the history did not reach the model, so the absence above proves nothing")
	}
}

func TestMergeQueriesLeavesALongInputOutAndKeepsAPhraseFirst(t *testing.T) {
	t.Parallel()

	if got := MergeQueries(phraseOf(MaxPhraseBytes+1), []string{"a", "b"}); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("MergeQueries with a long input = %q, want the derived phrases alone", got)
	}
	if got := MergeQueries("in", []string{"a", "in", "a"}); !slices.Equal(got, []string{"in", "a"}) {
		t.Fatalf("MergeQueries with a phrase input = %q, want the input first and no repeats", got)
	}
}

func TestIsPhraseRefusesBlankTextAndAcceptsExactlyTheBound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text string
		want bool
	}{
		{"", false},
		{" \t\n", false},
		{"a", true},
		{phraseOf(MaxPhraseBytes), true},
		{phraseOf(MaxPhraseBytes + 1), false},
		{phraseOf(MaxPhraseBytes) + "\n", false},
		{" " + phraseOf(MaxPhraseBytes), false},
	}
	for _, tc := range cases {
		if got := IsPhrase(tc.text); got != tc.want {
			t.Errorf("IsPhrase(%d bytes %q) = %v, want %v", len(tc.text), tc.text[:min(len(tc.text), 8)], got, tc.want)
		}
	}
}

func TestRenderSummaryNamesAnInputThatWasNotSentAsAQueryOnlyWhenItWasNot(t *testing.T) {
	t.Parallel()

	_, long, err := runPhraseTurn(t, &fakeModel{deriveErr: errors.New("down")}, phraseOf(600), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := RenderSummary(long, time.Time{}); !strings.Contains(got, "  input: the input is 600 bytes") {
		t.Fatalf("the summary of a long-input run does not name it:\n%s", got)
	}

	_, short, err := runPhraseTurn(t, &fakeModel{deriveErr: errors.New("down")}, "short input", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := RenderSummary(short, time.Time{}); strings.Contains(got, "  input: ") {
		t.Fatalf("the summary of a phrase-input run carries an input line:\n%s", got)
	}
}

func TestTheInputNotPhraseNoteIsEncodedAheadOfTheAnchorSoItStaysInsideTheGraphsEmbeddingWindow(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(Record{Input: phraseOf(300), InputNotPhrase: "a note"})
	if err != nil {
		t.Fatalf("marshalling the record failed: %v", err)
	}

	encoded := string(body)
	note := strings.Index(encoded, `"inputNotPhrase"`)
	anchor := strings.Index(encoded, `"anchor"`)

	if note < 0 || anchor < 0 || note > anchor {
		t.Fatalf("the encoded record carries inputNotPhrase at %d and anchor at %d; the note must be present and ahead of the candidates the graph's embedding window cuts off:\n%s", note, anchor, encoded)
	}
}
