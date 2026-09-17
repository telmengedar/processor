package loop

import (
	"context"
	"strings"
	"testing"
)

func fillPressureGraph() *fakeGraph {
	graph := baseGraph()
	graph.candidates = []Candidate{
		{ID: 91, Type: "documentation", Name: "admitted", Similarity: 0.9, Content: strings.Repeat("a", 11_900)},
		{ID: 93, Type: "documentation", Name: "also admitted", Similarity: 0.89, Content: strings.Repeat("c", 11_900)},
		{ID: 94, Type: "documentation", Name: "also admitted", Similarity: 0.88, Content: strings.Repeat("d", 11_900)},
		{ID: 95, Type: "documentation", Name: "also admitted", Similarity: 0.87, Content: strings.Repeat("e", 11_900)},
		{ID: 96, Type: "documentation", Name: "also admitted", Similarity: 0.86, Content: strings.Repeat("f", 11_900)},
		{ID: 92, Type: "documentation", Name: "wants to be pushed", Similarity: 0.8, Content: strings.Repeat("b", FillSizeFloor)},
	}
	return graph
}

func TestTurnRunCarriesTheFillOutcomesIntoTheRecordItWrites(t *testing.T) {
	t.Parallel()

	graph := fillPressureGraph()
	port := &fakeFill{results: map[int64]FillResult{92: {Written: true, Model: "gemma-3-12b-it"}}}
	turn := NewTurn(graph, &fakeModel{}, nil, "system", "test-model", testLogger())
	turn.Fill = port

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.Fills) == 0 {
		t.Fatal("record.Fills is empty after a run whose candidate list holds a byte-budget cut above the size floor: the fill is not wired into Run at all, and the whole behaviour is disconnected without a single test noticing")
	}

	filled := outcomeFor(t, record.Fills, 92)
	if !filled.Filled || filled.Model != "gemma-3-12b-it" {
		t.Fatalf("record.Fills entry for the cut candidate = %+v, want it filled and naming the producing model", filled)
	}
	if got := outcomeFor(t, record.Fills, 91); got.Filled || got.Reason != fillReasonNoPressure {
		t.Fatalf("record.Fills entry for the admitted candidate = %+v, want a no-pressure refusal", got)
	}
	if len(port.calls) != 1 || port.calls[0] != 92 {
		t.Fatalf("port.calls = %v, want exactly one attempt, for the cut candidate alone", port.calls)
	}
}

func TestTurnRunFilesTheSameFillOutcomesItReturns(t *testing.T) {
	t.Parallel()

	graph := fillPressureGraph()
	turn := NewTurn(graph, &fakeModel{}, nil, "system", "test-model", testLogger())
	turn.Fill = &fakeFill{}

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !graph.writeRunCalled {
		t.Fatal("the run was never written")
	}
	written := graph.writeRunRecord.Fills
	if len(written) != len(record.Fills) {
		t.Fatalf("the filed record carries %d fill outcomes, the returned record %d: the graph is told a different story about what this turn generated", len(written), len(record.Fills))
	}
	for i, want := range record.Fills {
		if written[i] != want {
			t.Fatalf("filed fill outcome %d = %+v, returned %+v", i, written[i], want)
		}
	}
}

func TestTurnRunFillsNothingWhenNoCandidateIsUnderByteBudgetPressure(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = []Candidate{{ID: 91, Similarity: 0.9, Content: strings.Repeat("a", FillSizeFloor)}}
	port := &fakeFill{}
	turn := NewTurn(graph, &fakeModel{}, nil, "system", "test-model", testLogger())
	turn.Fill = port

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(port.calls) != 0 {
		t.Fatalf("port.calls = %v, want none: everything the graph returned fits the block, so nothing wants to be pushed", port.calls)
	}
	if got := outcomeFor(t, record.Fills, 91); got.Filled {
		t.Fatalf("record.Fills entry = %+v, want a refusal recorded rather than a fill", got)
	}
}
