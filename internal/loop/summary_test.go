package loop

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func summaryInstant() time.Time {
	return time.Date(2026, 9, 7, 15, 20, 1, 0, time.UTC)
}

func summaryRecord() Record {
	return Record{
		Input:   "Generate a new barebones webpage and a repo for it.",
		Subject: 41,
		Queries: []string{"Generate a new barebones webpage and a repo for it."},
		Anchor: AnchorSummary{
			ID:   41,
			Type: "project",
			Name: "Processor — memory-substrate agent harness",
			Size: 3408,
		},
		Candidates: []Disposition{
			{Rank: 1, ID: 11, Type: "task", Name: "Pitch-Site hosting", Similarity: 0.689, Size: 1111, Included: true},
			{Rank: 2, ID: 12, Type: "documentation", Name: "Profilgenerator wireframe", Similarity: 0.659, Size: 43300, Included: true},
			{Rank: 3, ID: 13, Type: "session-log", Name: "processor-run earlier", Similarity: 0.640, Size: 87880, CutReason: "self-produced"},
			{Rank: 4, ID: 14, Type: "session-log", Name: "processor-run earlier still", Similarity: 0.638, Size: 83181, CutReason: "self-produced"},
			{Rank: 5, ID: 15, Type: "documentation", Name: "Something large", Similarity: 0.630, Size: 20000, CutReason: "byte budget exceeded"},
		},
		Block:      strings.Repeat("assembled block body ", 100),
		Answer:     "the webpage is written",
		Model:      "qwen3-coder:30b",
		Provider:   Provider{Adapter: "ollama", Endpoint: "http://127.0.0.1:11434/api/chat"},
		ModelCalls: 3,
		Usage:      []*Usage{{InTokens: 70000, OutTokens: 30}, nil, {InTokens: 1414, OutTokens: 409}},
		StopReason: StopReason{Reason: Answered, Raw: "stop"},
		Limits: Limits{
			CandidateLimit:          20,
			AssemblyByteBudget:      60000,
			SupplementaryByteBudget: 20000,
			MaxModelCalls:           6,
			MaxOutputTokens:         4096,
		},
	}
}

func TestRenderSummaryLeavesTheAssembledBlockOutOfItsOutputEntirely(t *testing.T) {
	t.Parallel()

	const marker = "BLOCK-BODY-MARKER"

	record := summaryRecord()
	record.Block = strings.Repeat(marker+" ", 12000)

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, marker) {
		t.Fatalf("the summary carries the block's text; the block is %d B of other nodes' bodies and must not be reproduced.\nsummary:\n%s", len(record.Block), summary)
	}
	if !strings.Contains(summary, record.Input) {
		t.Fatalf("the summary does not carry the run's input, so its silence about the block proves nothing.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryDoesNotGrowWithTheBlockItRefusesToRender(t *testing.T) {
	t.Parallel()

	small := summaryRecord()
	small.Block = ""

	large := summaryRecord()
	large.Block = strings.Repeat("x", 200000)

	smallSummary := RenderSummary(small, summaryInstant())
	largeSummary := RenderSummary(large, summaryInstant())

	if len(largeSummary)-len(smallSummary) > len("200000") {
		t.Fatalf("the summary grew by %d B when the block grew by %d B; only the block's byte count may differ.\nsmall:\n%s\nlarge:\n%s",
			len(largeSummary)-len(smallSummary), len(large.Block), smallSummary, largeSummary)
	}
}

func TestRenderSummaryStatesTheBlocksByteCount(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Block = strings.Repeat("x", 60379)

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "60379 B assembled") {
		t.Fatalf("the summary does not report the block's 60379 B.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryCountsAdmittedAndCutByReadingTheDispositionsItWasGiven(t *testing.T) {
	t.Parallel()

	record := summaryRecord()

	summary := RenderSummary(record, summaryInstant())

	const want = "5 candidates -> 2 admitted / 3 cut"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q, the split the five dispositions actually record.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryGroupsEachCutCandidateUnderTheReasonThatCandidateRecords(t *testing.T) {
	t.Parallel()

	record := summaryRecord()

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"self-produced (2, 171.1 kB): #13 #14",
		"byte budget exceeded (1, 20.0 kB): #15",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry the cut group %q.\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryNamesEveryCandidateWhetherAdmittedOrCut(t *testing.T) {
	t.Parallel()

	record := summaryRecord()

	summary := RenderSummary(record, summaryInstant())

	for _, d := range record.Candidates {
		id := "#" + strconv.FormatInt(d.ID, 10)
		if !strings.Contains(summary, id) {
			t.Fatalf("candidate %s (included=%v) is absent from the summary; which node was starved is the decision the record exists to answer.\nsummary:\n%s", id, d.Included, summary)
		}
	}
}

func TestRenderSummaryGivesACutCandidateNoReasonWordingTheRecordDidNotCarry(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Candidates = []Disposition{{Rank: 1, ID: 99, Size: 10}}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "(no reason recorded) (1, 10 B): #99") {
		t.Fatalf("a cut with no recorded reason is not reported as unrecorded.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryExcerptsALongAnswerAndStatesItsFullByteCount(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Answer = strings.Repeat("a", 5000)

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, strings.Repeat("a", 500)) {
		t.Fatalf("the summary carries at least 500 characters of a 5000 B answer, want a bounded excerpt.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "full answer is 5000 B in the record") {
		t.Fatalf("the summary excerpts the answer without stating the full 5000 B it stands for.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryPrintsAShortAnswerWholeWithNoExcerptNotice(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Answer = "the webpage is written"

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "| the webpage is written") {
		t.Fatalf("a 22 B answer is not printed in full.\nsummary:\n%s", summary)
	}
	if strings.Contains(summary, "excerpt;") {
		t.Fatalf("a 22 B answer is announced as an excerpt.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryFlagsAnEmptyAnswerAgainstATerminalReasonOfAnswered(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Answer = ""
	record.StopReason = StopReason{Reason: Answered, Raw: "stop"}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "EMPTY (0 B)  <-- terminal reason says answered") {
		t.Fatalf("an empty answer under a terminal reason of answered is not flagged; the contradiction is the one thing a reader needs.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryLeavesAnEmptyAnswerUnflaggedWhenTheTerminalReasonExpectsOne(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Answer = ""
	record.StopReason = StopReason{Reason: WantsWrite, Raw: "stop"}

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, "terminal reason says answered") {
		t.Fatalf("an empty answer under %q is flagged as a contradiction, want the flag only under %q.\nsummary:\n%s", WantsWrite, Answered, summary)
	}
	if !strings.Contains(summary, "EMPTY (0 B)") {
		t.Fatalf("the summary does not report the empty answer at all.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryStampsTheInstantItWasGiven(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 7, 15, 20, 1, 0, time.UTC)

	summary := RenderSummary(summaryRecord(), at)

	if !strings.HasPrefix(summary, "processor-run at 2026-09-07T15:20:01Z\n") {
		t.Fatalf("the summary does not open with the instant it was handed.\nsummary:\n%s", summary)
	}
}

func TestRenderSummarySumsTokensOverTheCallsTheEndpointActuallyReported(t *testing.T) {
	t.Parallel()

	record := summaryRecord()

	summary := RenderSummary(record, summaryInstant())

	const want = "tokens   71414 in / 439 out over 2 calls  (out per call: 30, 409)"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q; three usage slots were given and one is nil.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryReportsTokensAsUnrecordedRatherThanAsZero(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Usage = []*Usage{nil, nil}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "tokens   not recorded") {
		t.Fatalf("usage the endpoint never reported is not reported as unrecorded.\nsummary:\n%s", summary)
	}
	if strings.Contains(summary, "0 in / 0 out") {
		t.Fatalf("usage the endpoint never reported is rendered as a measured zero.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryNamesTheToolOfEveryRoundInTheOrderTheRunTookThem(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{
		{Tool: ToolRecall, Source: ToolSourceNative, Query: "repository creation", Results: []Disposition{
			{Rank: 1, ID: 21, Size: 400, Included: true},
			{Rank: 2, ID: 22, Size: 90000, CutReason: "byte budget exceeded"},
		}},
		{Tool: ToolWriteFile, Source: ToolSourceContent, Path: "index.html", Bytes: 502},
	}

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"TOOLS  2 rounds",
		"1 recall [native]  \"repository creation\"",
		"-> 2 results, 1 admitted (400 B) / cut: byte budget exceeded 1",
		"2 writeFile [content]  index.html  502 B",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry %q.\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryCarriesAToolRoundsErrorRatherThanReportingTheRoundAsClean(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{Tool: ToolWriteFile, Path: "index.html", Error: "call cap reached"}}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "ERROR: call cap reached") {
		t.Fatalf("a failed tool round is rendered without its error.\nsummary:\n%s", summary)
	}
}

func TestRenderSummarySaysProviderNotRecordedRatherThanNamingAnEmptyEndpoint(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Provider = Provider{}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "qwen3-coder:30b   [provider not recorded]") {
		t.Fatalf("a record written before the provider was captured does not say so.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryOfAFullTwentyCandidateRunStaysUnderTwoAndAHalfKilobytes(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Candidates = nil
	for i := range 20 {
		record.Candidates = append(record.Candidates, Disposition{
			Rank:       i + 1,
			ID:         int64(100 + i),
			Type:       "documentation",
			Name:       strings.Repeat("a long node name that will be bounded by the renderer ", 3),
			Similarity: 0.6,
			Size:       40000,
			Included:   i < 8,
			CutReason:  "byte budget exceeded",
		})
	}
	record.ToolCalls = []ToolCallRecord{
		{Tool: ToolRecall, Source: ToolSourceNative, Query: strings.Repeat("a long recall query ", 10), Results: record.Candidates},
		{Tool: ToolWriteFile, Source: ToolSourceNative, Path: "index.html", Bytes: 502},
	}
	record.Answer = strings.Repeat("answer prose ", 500)

	summary := RenderSummary(record, summaryInstant())

	const bound = 2560
	if len(summary) > bound {
		t.Fatalf("the summary of a full run is %d B, above the %d B bound the recall slot is worth.\nsummary:\n%s", len(summary), bound, summary)
	}
}
