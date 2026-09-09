package loop

import "testing"

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
