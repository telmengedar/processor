package loop

import (
	"fmt"
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
		Queries: []string{"barebones webpage repo scaffold"},
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
		"2 writeFile [content]  \"index.html\"  502 B",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry %q.\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryRendersALegacyRoundWithNoToolFieldAsARecallRatherThanABlankLine(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{
		Query: "Go comment-discipline annex ruling on package doc comments",
		Results: []Disposition{
			{Rank: 1, ID: 41, Size: 5000, Included: true},
			{Rank: 2, ID: 42, Size: 90000, CutReason: "byte budget exceeded"},
		},
	}}

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"1 [tool not recorded]  \"Go comment-discipline annex ruling on package doc comments\"",
		"-> 2 results, 1 admitted (5000 B) / cut: byte budget exceeded 1",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("a legacy round whose JSON carries no tool member is not rendered as the recall it plainly is.\nwant: %q\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryRendersALegacyRoundWithNeitherToolNorQueryNorPathAsUnrecordedRatherThanInventingAShape(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{}}

	summary := RenderSummary(record, summaryInstant())

	line, found := summaryLineWithPrefix(summary, "  1 ")
	if !found {
		t.Fatalf("a round with no tool, query or path produced no round line at all.\nsummary:\n%s", summary)
	}
	if line != "  1 [tool not recorded]" {
		t.Fatalf("a round with nothing recorded is not rendered as bare and unrecorded; want %q, got %q.\nsummary:\n%s", "  1 [tool not recorded]", line, summary)
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

func TestRenderSummaryQuotesAWritePathSoANewlineInItCannotForgeAnExtraRoundLine(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{
		Tool:   ToolWriteFile,
		Source: ToolSourceNative,
		Path:   "hello.html\n  9 writeFile [native]  C:/Windows/System32/evil.dll  4096 B",
		Bytes:  120,
	}}

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, "\n  9 writeFile") {
		t.Fatalf("a newline embedded in the write path forged an extra round line.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "TOOLS  1 round") {
		t.Fatalf("the round count no longer matches the single call the record carries.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryTruncatesAWritePathAtSeventyTwoRunes(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	longPath := strings.Repeat("a", 100)
	record.ToolCalls = []ToolCallRecord{{
		Tool:   ToolWriteFile,
		Source: ToolSourceNative,
		Path:   longPath,
		Bytes:  120,
	}}

	summary := RenderSummary(record, summaryInstant())

	want := `"` + strings.Repeat("a", 71) + "…" + `"`
	if !strings.Contains(summary, want) {
		t.Fatalf("a 100-rune write path is not truncated at 72 runes; want the quoted excerpt %q.\nsummary:\n%s", want, summary)
	}
	if strings.Contains(summary, longPath) {
		t.Fatalf("the full 100-rune write path appears in the summary untruncated.\nsummary:\n%s", summary)
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

func TestRenderSummaryEchoesAShortInputWholeAndBoundsALongOne(t *testing.T) {
	t.Parallel()

	short := summaryRecord()

	shortSummary := RenderSummary(short, summaryInstant())

	if !strings.Contains(shortSummary, "input    "+short.Input+"\n") {
		t.Fatalf("the summary does not echo the run's %d-rune input whole.\nsummary:\n%s", len([]rune(short.Input)), shortSummary)
	}

	long := summaryRecord()
	long.Input = strings.Repeat("z", 5000)

	longSummary := RenderSummary(long, summaryInstant())

	line, found := summaryLineWithPrefix(longSummary, "input    ")
	if !found {
		t.Fatalf("a 5000-rune input produced no input line at all.\nsummary:\n%s", longSummary)
	}
	if got := len([]rune(line)) - len([]rune("input    ")); got != 200 {
		t.Fatalf("the input line carries %d runes of a 5000-rune input, want it bounded at 200.\nline: %s", got, line)
	}
}

func TestRenderSummaryFlattensWhitespaceSoOneRecordedFactStaysOnOneLine(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Input = "first line\n\tsecond   line\nthird line"

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "input    first line second line third line\n") {
		t.Fatalf("the input's own newlines survive into the summary, so one fact spans several lines and every line below it names something other than what precedes it.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryIdentifiesTheSubjectByIdTypeNameAndSize(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	const want = `subject  #41 project "Processor — memory-substrate agent harness" (3408 B)`
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not identify the subject as %q; a record whose anchor is unidentifiable cannot be read back against the node it was about.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryNamesTheAdapterAndEndpointThatServedTheRun(t *testing.T) {
	t.Parallel()

	record := summaryRecord()

	summary := RenderSummary(record, summaryInstant())

	want := "model    " + record.Model + " via " + record.Provider.Adapter + " " + record.Provider.Endpoint
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not carry %q; which adapter reached which endpoint is what makes a run reproducible.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryStatesEveryLimitTheRunWasBoundBy(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	const want = "limits   20 cands / 60000 B content / 20000 B suppl / 6 calls / 4096 tok"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q, the five limits this run was bound by.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryLabelsTheAssemblyBudgetAsContentRatherThanBlockSoItIsNotComparedToTheAssembledBlock(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Block = strings.Repeat("x", 60379)

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, "B block") {
		t.Fatalf("the limits line still names the content budget a block budget, inviting a reader to compare it against the assembled block's byte count directly below.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "60000 B content") {
		t.Fatalf("the limits line does not name the assembly budget as a content budget.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "60379 B assembled") {
		t.Fatalf("the block line no longer states the block's byte count.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryReportsTemperatureAndTopPEachUnderItsOwnName(t *testing.T) {
	t.Parallel()

	temperature, topP := 0.2, 0.95
	record := summaryRecord()
	record.Sampling = Sampling{Temperature: &temperature, TopP: &topP}

	summary := RenderSummary(record, summaryInstant())

	const want = "4096 tok  temp=0.2 topP=0.95"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q; the two values differ so that reporting one under the other's name is visible.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryLeavesTheSamplingFieldsOutWhenTheRunSetNeither(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	if !strings.Contains(summary, "4096 tok\n") {
		t.Fatalf("the limits line does not end at the token cap on a run that set no sampling parameter.\nsummary:\n%s", summary)
	}
	if strings.Contains(summary, "temp=") || strings.Contains(summary, "topP=") {
		t.Fatalf("the summary names a sampling parameter the run left to the endpoint's default.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryNamesTheWorkingDirectoryOnlyWhenTheRunOpenedOne(t *testing.T) {
	t.Parallel()

	without := RenderSummary(summaryRecord(), summaryInstant())

	if strings.Contains(without, "workdir") {
		t.Fatalf("a run that opened no working directory is given a workdir line anyway.\nsummary:\n%s", without)
	}

	record := summaryRecord()
	record.Workspace = "/runs/2026-09-07T15-20-01Z"

	with := RenderSummary(record, summaryInstant())

	if !strings.Contains(with, "workdir  "+record.Workspace+"\n") {
		t.Fatalf("the summary does not name %q, the directory this run's file writes went to.\nsummary:\n%s", record.Workspace, with)
	}
}

func TestRenderSummaryPrintsEveryDerivedQueryUnderItsOwnIndexAndCountsThem(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Queries = []string{"first derived query", "second derived query", "third derived query"}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "ASSEMBLY  3 queries,") {
		t.Fatalf("the summary does not count the three queries assembly was given.\nsummary:\n%s", summary)
	}
	for i, query := range record.Queries {
		want := fmt.Sprintf("  q%d: %s\n", i, query)
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry %q; which query was issued is what makes the candidates below it auditable.\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryHeadsTheAdmittedListWithItsCountAndBytesAgainstTheSpaceRemainingAfterTheAnchor(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	const want = "  admitted (2, 44.4 kB of 56592 B remaining):"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not head the admitted list %q; the anchor's 3408 B is already spent against the 60000 B budget, leaving 56592 B for candidates.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryDenominatorReflectsTrueHeadroomOnceTheAnchorTookItsShare(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Limits.AssemblyByteBudget = 60000
	record.Anchor.Size = 59500
	record.Candidates = []Disposition{
		{Rank: 1, ID: 11, Type: "task", Name: "small", Similarity: 0.9, Size: 494, Included: true},
	}

	summary := RenderSummary(record, summaryInstant())

	const want = "  admitted (1, 494 B of 500 B remaining):"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q; a reader computing 500-494=6 B should reach the true headroom without opening the record. A denominator of 60000 (forgetting the anchor) would read 59506 B headroom instead of 6 B.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryFloorsTheRemainingAfterAnchorAtZeroWhenTheAnchorAloneExceedsTheAssemblyBudget(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Limits.AssemblyByteBudget = 1000
	record.Anchor.Size = 2000
	record.Candidates = []Disposition{
		{Rank: 1, ID: 11, Type: "task", Name: "small", Similarity: 0.9, Size: 500, Included: true},
	}

	summary := RenderSummary(record, summaryInstant())

	const want = "  admitted (1, 500 B of 0 B remaining):"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q; an anchor of 2000 B against a 1000 B budget leaves no headroom, and the line must floor at 0 B rather than print a negative remainder.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryGivesEachAdmittedCandidateItsSimilarityTypeSizeAndName(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	for _, want := range []string{
		"    #11     0.689 task            1111 B  Pitch-Site hosting",
		"    #12     0.659 documentation  43.3 kB  Profilgenerator wireframe",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry the admitted row %q; an id with no similarity, type, size or name cannot be judged as an admission.\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryHeadsTheCutListWithHowManyCandidatesWereCut(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	const want = "  cut (3):"
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not head the cut list %q; three of the five dispositions record a cut.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryWrapsALongCutGroupsIdsAcrossLinesNoneOverNinetySixRunes(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.Candidates = nil
	for i := range 40 {
		record.Candidates = append(record.Candidates, Disposition{
			Rank: i + 1, ID: int64(1000000 + i), Size: 10, CutReason: "byte budget exceeded",
		})
	}

	summary := RenderSummary(record, summaryInstant())

	carrying := 0
	for _, line := range strings.Split(summary, "\n") {
		if !strings.Contains(line, "#1000") {
			continue
		}
		carrying++
		if got := len([]rune(line)); got > 96 {
			t.Fatalf("a cut group's ids run to %d runes on one line, above the 96 the renderer wraps at.\nline: %s", got, line)
		}
	}
	if carrying < 2 {
		t.Fatalf("forty cut ids reached %d line(s), so this fixture never exercised wrapping.\nsummary:\n%s", carrying, summary)
	}
	for _, d := range record.Candidates {
		id := "#" + strconv.FormatInt(d.ID, 10)
		if !strings.Contains(summary, id) {
			t.Fatalf("wrapping dropped cut candidate %s.\nsummary:\n%s", id, summary)
		}
	}
}

func TestRenderSummaryStatesTheTerminalReasonTheEndpointsRawStringAndTheCallCount(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(summaryRecord(), summaryInstant())

	const want = `OUTCOME  answered (raw "stop"), 3/6 model calls, cap not reached`
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q; how a run ended and how many calls it spent are the two facts a reader weighs the answer against.\nsummary:\n%s", want, summary)
	}
}

func TestRenderSummaryReportsTheCallCapAsReachedWhenTheRunHitIt(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.CapReached = true

	summary := RenderSummary(record, summaryInstant())

	if strings.Contains(summary, "cap not reached") {
		t.Fatalf("a run that hit the call cap is reported as not having reached it.\nsummary:\n%s", summary)
	}
	if !strings.Contains(summary, "cap reached") {
		t.Fatalf("a run that hit the call cap does not say so.\nsummary:\n%s", summary)
	}
}

func TestRenderSummarySaysARecallRoundRecordedNoResultsRatherThanShowingNone(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{Tool: ToolRecall, Source: ToolSourceNative, Query: "nothing matched"}}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "      -> no results recorded\n") {
		t.Fatalf("a recall round that returned nothing is rendered as though it were never asked.\nsummary:\n%s", summary)
	}
}

func TestRenderSummarySaysNoneAdmittedWhenEveryRecallResultWasCut(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{Tool: ToolRecall, Source: ToolSourceNative, Query: "all too large", Results: []Disposition{
		{Rank: 1, ID: 31, Size: 90000, CutReason: "byte budget exceeded"},
	}}}

	summary := RenderSummary(record, summaryInstant())

	for _, want := range []string{
		"      -> 1 results, 0 admitted (0 B) / cut: byte budget exceeded 1\n",
		"      (none admitted)\n",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary does not carry %q; a supplementary recall that admitted nothing is the case a reader most needs told.\nsummary:\n%s", want, summary)
		}
	}
}

func TestRenderSummaryOfTheLargestRunTheseLimitsPermitStaysUnderFourKilobytes(t *testing.T) {
	t.Parallel()

	temperature, topP := 0.2, 0.95
	record := summaryRecord()
	record.Workspace = "/runs/2026-09-07T15-20-01Z"
	record.Sampling = Sampling{Temperature: &temperature, TopP: &topP}
	record.Answer = strings.Repeat("answer prose ", 500)

	record.Queries = nil
	for i := range record.Limits.MaxModelCalls {
		record.Queries = append(record.Queries, fmt.Sprintf("q%d %s", i, strings.Repeat("a long derived query ", 10)))
	}

	record.Candidates = nil
	for i := range record.Limits.CandidateLimit {
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

	record.ToolCalls = nil
	for range record.Limits.MaxModelCalls {
		record.ToolCalls = append(record.ToolCalls, ToolCallRecord{
			Tool:    ToolRecall,
			Source:  ToolSourceNative,
			Query:   strings.Repeat("a long recall query ", 10),
			Results: record.Candidates,
		})
	}

	summary := RenderSummary(record, summaryInstant())

	const bound = 4096
	if len(summary) > bound {
		t.Fatalf("the summary of the largest run these limits permit is %d B, above the %d B bound.\nsummary:\n%s", len(summary), bound, summary)
	}
}

func TestRenderSummaryNamesTheNodesASupplementaryRecallAdmitted(t *testing.T) {
	t.Parallel()

	record := summaryRecord()
	record.ToolCalls = []ToolCallRecord{{Tool: ToolRecall, Source: ToolSourceNative, Query: "repository creation", Results: []Disposition{
		{Rank: 1, ID: 21, Size: 400, Included: true},
		{Rank: 2, ID: 23, Size: 700, Included: true},
		{Rank: 3, ID: 22, Size: 90000, CutReason: "byte budget exceeded"},
	}}}

	summary := RenderSummary(record, summaryInstant())

	if !strings.Contains(summary, "      #21 #23\n") {
		t.Fatalf("the summary counts what a supplementary recall admitted without naming it; which node came in mid-run is the same decision the assembly list exists to record.\nsummary:\n%s", summary)
	}
}

func TestRenderSummaryStatesTheAnswersByteCountAboveTheExcerptItPrints(t *testing.T) {
	t.Parallel()

	record := summaryRecord()

	summary := RenderSummary(record, summaryInstant())

	want := fmt.Sprintf("  answer   %d B\n", len(record.Answer))
	if !strings.Contains(summary, want) {
		t.Fatalf("the summary does not state %q; the excerpt below it stands for the whole answer only if the whole answer's size is given.\nsummary:\n%s", want, summary)
	}
}

func summaryLineWithPrefix(summary, prefix string) (string, bool) {
	for _, line := range strings.Split(summary, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line, true
		}
	}
	return "", false
}

func summaryWithToolError(cause string) string {
	record := Record{ToolCalls: []ToolCallRecord{{Tool: ToolRecall, Query: "q", Error: cause}}}
	return RenderSummary(record, summaryInstant())
}

func TestRenderSummaryCarriesAnEightyNineRuneCauseWholeWhereTheOldWidthCutIt(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("x", 89)
	if got := summaryWithToolError(cause); !strings.Contains(got, "ERROR: "+cause+"\n") {
		t.Fatalf("the summary cut an 89-rune cause that the widened error line must carry whole; summary:\n%s", got)
	}
}

func TestRenderSummaryCarriesACauseOfExactlyTheErrorWidthWhole(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("x", summaryErrorRunes)
	if got := summaryWithToolError(cause); !strings.Contains(got, "ERROR: "+cause+"\n") {
		t.Fatalf("the summary cut a cause of exactly the %d-rune error width; summary:\n%s", summaryErrorRunes, got)
	}
}

func TestRenderSummaryTruncatesACauseOneRunePastTheErrorWidth(t *testing.T) {
	t.Parallel()

	cause := strings.Repeat("x", summaryErrorRunes+1)
	summary := summaryWithToolError(cause)

	line := ""
	for _, candidate := range strings.Split(summary, "\n") {
		if strings.Contains(candidate, "ERROR: ") {
			line = strings.TrimPrefix(strings.TrimSpace(candidate), "ERROR: ")
		}
	}
	if line == "" {
		t.Fatalf("the summary rendered no error line at all; summary:\n%s", summary)
	}
	if n := len([]rune(line)); n != summaryErrorRunes {
		t.Fatalf("the summary rendered %d runes of a cause one rune past the error width, want %d", n, summaryErrorRunes)
	}
}

func TestRenderSummaryKeepsTheQueryExcerptAtItsOwnNarrowerWidth(t *testing.T) {
	t.Parallel()

	query := strings.Repeat("q", summaryQueryRunes+1)
	summary := RenderSummary(Record{Queries: []string{query}}, summaryInstant())

	line := ""
	for _, candidate := range strings.Split(summary, "\n") {
		if strings.HasPrefix(strings.TrimSpace(candidate), "q0: ") {
			line = strings.TrimPrefix(strings.TrimSpace(candidate), "q0: ")
		}
	}
	if line == "" {
		t.Fatalf("the summary rendered no query line at all; summary:\n%s", summary)
	}
	if n := len([]rune(line)); n != summaryQueryRunes {
		t.Fatalf("the summary rendered %d runes of a query one rune past the query width, want %d — the error line's widening reached the query excerpt too", n, summaryQueryRunes)
	}
}

func TestTheSummarysErrorWidthIsTwoHundredRunes(t *testing.T) {
	t.Parallel()

	if got := summaryWithToolError(strings.Repeat("x", 200)); !strings.Contains(got, "ERROR: "+strings.Repeat("x", 200)+"\n") {
		t.Fatalf("a 200-rune cause did not render whole, so the summary's error width is below the 200 the design derives; summary:\n%s", got)
	}
	if got := summaryWithToolError(strings.Repeat("x", 201)); strings.Contains(got, "ERROR: "+strings.Repeat("x", 201)+"\n") {
		t.Fatalf("a 201-rune cause rendered whole, so the summary's error width is above 200; summary:\n%s", got)
	}
}

func TestTheSummarysQueryWidthStaysAtEightyEightRunes(t *testing.T) {
	t.Parallel()

	summary := RenderSummary(Record{Queries: []string{strings.Repeat("q", 88)}}, summaryInstant())
	if !strings.Contains(summary, "q0: "+strings.Repeat("q", 88)+"\n") {
		t.Fatalf("an 88-rune query did not render whole, so the query width dropped below 88; summary:\n%s", summary)
	}
	wider := RenderSummary(Record{Queries: []string{strings.Repeat("q", 89)}}, summaryInstant())
	if strings.Contains(wider, "q0: "+strings.Repeat("q", 89)+"\n") {
		t.Fatalf("an 89-rune query rendered whole, so the error line's widening reached the query excerpt; summary:\n%s", wider)
	}
}
