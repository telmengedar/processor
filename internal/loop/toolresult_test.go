package loop

import (
	"strings"
	"testing"
)

func TestRenderToolResultRendersARecordedErrorBehindItsPrefix(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{Tool: ToolRecall, Query: "anything", Error: "tool arguments could not be parsed"})

	want := "error: tool arguments could not be parsed"

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRenderToolResultRendersAWriteRoundAsAReceiptNamingTheByteCountAndThePath(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{Tool: ToolWriteFile, Path: "site/index.html", Content: "<h1>a page</h1>", Bytes: 15})

	want := "wrote 15 bytes to site/index.html"

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRenderToolResultRendersARecallThatFoundNothingAsOneSentenceRatherThanAnEmptyString(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{Tool: ToolRecall, Query: "nothing matches this"})

	want := "no additional results found."

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRenderToolResultRendersOneRecalledCandidateAsADelimitedSectionCarryingItsIdentityAndBody(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{
		Tool:    ToolRecall,
		Query:   "the query",
		Results: []Candidate{{ID: 5, Type: "task", Name: "Found", Content: "found body"}},
	})

	want := "===== RESULT =====\nid: 5\ntype: task\nname: Found\n\nfound body\n"

	if got != want {
		t.Fatalf("tool result =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderToolResultPutsExactlyOneNewlineBetweenTwoRecalledCandidatesSections(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{
		Tool:  ToolRecall,
		Query: "the query",
		Results: []Candidate{
			{ID: 5, Type: "task", Name: "Found", Content: "found body"},
			{ID: 9, Type: "documentation", Name: "Also Found", Content: "second found body"},
		},
	})

	want := "===== RESULT =====\nid: 5\ntype: task\nname: Found\n\nfound body\n" +
		"\n" +
		"===== RESULT =====\nid: 9\ntype: documentation\nname: Also Found\n\nsecond found body\n"

	if got != want {
		t.Fatalf("tool result =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderToolResultSaysResultsWereFoundAndNoneFitWhenAdmissionCutThemAll(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{
		Tool:  ToolRecall,
		Query: "the query",
		Dispositions: []Disposition{
			{Rank: 1, ID: 987654321, Type: "documentation", Name: "Vermilion Ledger Of Tides", Size: 67312, CutReason: cutReasonByteBudget},
			{Rank: 2, ID: 987654322, Type: "documentation", Name: "Cobalt Almanac Of Hedges", Size: 96555, CutReason: cutReasonByteBudget},
		},
	})

	want := "results were found, but none were included."

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRenderToolResultSaysResultsWereFoundWhenEveryRowWasCutAsSelfProducedRatherThanForBytes(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{
		Tool:  ToolRecall,
		Query: "the query",
		Dispositions: []Disposition{
			{Rank: 1, ID: 987654323, Type: "run-record", Name: "Amber Transcript Of Gates", Size: 2200, CutReason: cutReasonSelfProduced},
			{Rank: 2, ID: 987654324, Type: "run-record", Name: "Slate Transcript Of Gates", Size: 2400, CutReason: cutReasonSelfProduced},
		},
	})

	want := "results were found, but none were included."

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRenderToolResultNamesNoUnadmittedNodeInTheAllCutSentence(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{
		Tool:  ToolRecall,
		Query: "the query",
		Dispositions: []Disposition{
			{Rank: 1, ID: 987654321, Type: "documentation", Name: "Vermilion Ledger Of Tides", Size: 67312, CutReason: cutReasonByteBudget},
		},
	})

	if strings.Contains(got, "987654321") {
		t.Fatalf("tool result %q names the id of a row the model cannot fetch", got)
	}
	if strings.Contains(got, "Vermilion Ledger Of Tides") {
		t.Fatalf("tool result %q names the name of a row the model cannot fetch", got)
	}
}

func TestRenderToolResultSaysNothingWasFoundWhenAnEmptyRecallCarriedAnEmptyDispositionSliceRatherThanNil(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{Tool: ToolRecall, Query: "nothing matches this", Dispositions: []Disposition{}})

	want := "no additional results found."

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRenderToolResultRendersTheErrorRatherThanTheAllCutSentenceWhenTheExchangeCarriesBoth(t *testing.T) {
	t.Parallel()

	got := RenderToolResult(ToolExchange{
		Tool:  ToolRecall,
		Query: "the query",
		Error: "supplementary recall failed",
		Dispositions: []Disposition{
			{Rank: 1, ID: 987654321, Type: "documentation", Name: "Vermilion Ledger Of Tides", Size: 67312, CutReason: cutReasonByteBudget},
		},
	})

	want := "error: supplementary recall failed"

	if got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}
