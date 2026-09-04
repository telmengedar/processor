package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const corpusPathForTests = "internal/eval/corpus.json"

func render(t *testing.T, result Result) (machine, human string) {
	t.Helper()

	var machineBuf, humanBuf bytes.Buffer
	if err := Render(result, corpusPathForTests, &machineBuf, &humanBuf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return machineBuf.String(), humanBuf.String()
}

func resultWith(rows ...RowResult) Result {
	return Result{
		CorpusHash: "854847806c2db7327af358e4966916646de496abebedd033740dec3bcdae353d",
		SweptAt:    time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC),
		Limits:     Limits{CandidateLimit: loop.CandidateLimit, AssemblyByteBudget: loop.AssemblyByteBudget},
		RowCount:   len(rows),
		Rows:       rows,
	}
}

func oneMissOfEachKind() RowResult {
	return RowResult{
		Row:            "r01",
		Stratum:        StratumLabelled,
		Subject:        100,
		CandidateCount: 17,
		AdmittedCount:  7,
		AdmittedBytes:  32504,
		BudgetBytes:    loop.AssemblyByteBudget,
		TopSimilarity:  0.7143,
		Required: []NodeResult{
			{Node: 301, Verdict: Admitted, Rank: 1},
			{Node: 302, Verdict: Cut, Rank: 8},
			{Node: 303, Verdict: NotRetrieved},
		},
	}
}

func intactControlRow() RowResult {
	return RowResult{
		Row:      "c01",
		Stratum:  StratumControl,
		Subject:  400,
		Required: []NodeResult{{Node: 401, Verdict: Admitted, Rank: 1}},
	}
}

func controlRowScoring(verdict Verdict) RowResult {
	return RowResult{
		Row:            "c01",
		Stratum:        StratumControl,
		Subject:        400,
		CandidateCount: 17,
		AdmittedCount:  1,
		AdmittedBytes:  56302,
		BudgetBytes:    loop.AssemblyByteBudget,
		Required:       []NodeResult{{Node: 401, Verdict: verdict}},
	}
}

func mustContain(t *testing.T, output, want string) {
	t.Helper()

	if !strings.Contains(output, want) {
		t.Fatalf("output does not contain %q; output was:\n%s", want, output)
	}
}

func mustNotContain(t *testing.T, output, unwanted string) {
	t.Helper()

	if strings.Contains(output, unwanted) {
		t.Fatalf("output contains %q; output was:\n%s", unwanted, output)
	}
}

func TestReportPrintsTheNumeratorAndDenominatorBesideEveryRate(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	mustContain(t, human, "retrieved 2/3 (0.67)")
	mustContain(t, human, "admitted 1/3 (0.33)")
	mustContain(t, human, "retrieved 1/1 (1.00)")
	mustContain(t, human, "admitted 1/1 (1.00)")
}

func TestReportKeepsTheLabelledAndControlRatesSeparate(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	mustContain(t, human, "labelled")
	mustContain(t, human, "control")
	if strings.Contains(human, "3/4") {
		t.Fatalf("the summary sums the two strata into a single denominator; output was:\n%s", human)
	}
}

func TestReportNamesAllTwelveMissesInAFixtureWithTwelveMisses(t *testing.T) {
	t.Parallel()

	var rows []RowResult
	node := int64(301)
	for r := 1; r <= 4; r++ {
		row := RowResult{
			Row:            fmt.Sprintf("r%02d", r),
			Stratum:        StratumLabelled,
			CandidateCount: 17,
			BudgetBytes:    loop.AssemblyByteBudget,
		}
		for i := 0; i < 3; i++ {
			row.Required = append(row.Required, NodeResult{Node: node, Verdict: NotRetrieved})
			node++
		}
		rows = append(rows, row)
	}

	_, human := render(t, resultWith(rows...))

	for id := int64(301); id < 313; id++ {
		mustContain(t, human, fmt.Sprintf("#%d", id))
	}
	for r := 1; r <= 4; r++ {
		mustContain(t, human, fmt.Sprintf("r%02d", r))
	}
	mustContain(t, human, "retrieved 0/12 (0.00)")
}

func TestReportGivesACutMissTheRankAndTheByteUtilisationThatExplainIt(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	mustContain(t, human, "cut at rank 8")
	mustContain(t, human, fmt.Sprintf("k'=7, 32504/%d bytes admitted", loop.AssemblyByteBudget))
}

func TestReportGivesANotRetrievedMissTheCandidateCountAndTopSimilarity(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	mustContain(t, human, "notRetrieved")
	mustContain(t, human, "17 candidates, top similarity 0.7143")
}

func TestReportExcludesARowWithAnUnresolvedRequiredNodeFromBothDenominators(t *testing.T) {
	t.Parallel()

	unresolved := RowResult{
		Row:      "r02",
		Stratum:  StratumLabelled,
		Required: []NodeResult{{Node: 501, Verdict: Admitted, Rank: 1}, {Node: 502, Verdict: Unresolved}},
	}

	_, human := render(t, resultWith(oneMissOfEachKind(), unresolved, intactControlRow()))

	mustContain(t, human, "retrieved 2/3 (0.67)")
	mustContain(t, human, "unresolved required nodes  1 (excluded from both rates)")
	if strings.Contains(human, "/5") {
		t.Fatalf("an unresolved row still contributes to a denominator; output was:\n%s", human)
	}
}

func TestReportNamesARowTheSweepCouldNotScoreAtAll(t *testing.T) {
	t.Parallel()

	errored := RowResult{Row: "r09", Stratum: StratumLabelled, Subject: 999, Error: "subject not found"}

	_, human := render(t, resultWith(oneMissOfEachKind(), errored, intactControlRow()))

	mustContain(t, human, "row errors (not scored):")
	mustContain(t, human, "r09   subject not found")
	mustContain(t, human, "retrieved 2/3 (0.67)")
}

func TestReportBlamesTheInstrumentAndNotTheBudgetWhenAControlNodeWasNeverRetrieved(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), controlRowScoring(NotRetrieved)))

	mustContain(t, human, "the control stratum did not verify retrieval: either the graph moved, the harness broke, or the stratum could not be scored at all, and this sweep's labelled number is not trustworthy")
	mustContain(t, human, "misses (control):")
	mustContain(t, human, "#401")
	mustNotContain(t, human, "budget alarm")
}

func TestReportBlamesTheBudgetAndNotTheInstrumentWhenTheBudgetCutAControlNodeItRetrieved(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), controlRowScoring(Cut)))

	mustContain(t, human, "budget alarm: the control stratum was retrieved in full and cut by the budget. Retrieval is intact and this sweep's retrieved rate is trustworthy; the admitted rate is a reading of the assembler, not of the retriever.")
	mustContain(t, human, "control   retrieved 1/1 (1.00)   admitted 0/1 (0.00)")
	mustNotContain(t, human, "either the graph moved, the harness broke, or the stratum could not be scored at all")
}

func TestReportBlamesTheInstrumentWhenOneControlNodeWasCutAndAnotherWasNeverRetrieved(t *testing.T) {
	t.Parallel()

	mixed := resultWith(oneMissOfEachKind(), controlRowScoring(Cut),
		RowResult{Row: "c02", Stratum: StratumControl, Required: []NodeResult{{Node: 402, Verdict: NotRetrieved}}})

	_, human := render(t, mixed)

	mustContain(t, human, "control   retrieved 1/2 (0.50)   admitted 0/2 (0.00)")
	mustContain(t, human, "the control stratum did not verify retrieval: either the graph moved, the harness broke, or the stratum could not be scored at all, and this sweep's labelled number is not trustworthy")
	mustNotContain(t, human, "budget alarm")
	mustNotContain(t, human, "retrieved in full")
}

func TestReportDoesNotDenyTheRateItJustPrintedWhenAControlRowCouldNotBeScored(t *testing.T) {
	t.Parallel()

	unscored := resultWith(oneMissOfEachKind(), intactControlRow(),
		RowResult{Row: "c02", Stratum: StratumControl, Required: []NodeResult{{Node: 402, Verdict: Unresolved}}})

	_, human := render(t, unscored)

	mustContain(t, human, "control   retrieved 1/1 (1.00)   admitted 1/1 (1.00)")
	mustContain(t, human, "the control stratum did not verify retrieval: either the graph moved, the harness broke, or the stratum could not be scored at all, and this sweep's labelled number is not trustworthy")
	mustNotContain(t, human, "did not read 1.00")
	mustNotContain(t, human, "budget alarm")
}

func TestReportWarnsWhenTheCorpusCarriedNoControlStratumAtAll(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind()))

	mustContain(t, human, "no control stratum: this sweep verified nothing about itself, so a broken harness would report a plausible number")
}

func TestReportSaysNothingAlarmingWhenTheControlStratumReadOne(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	mustNotContain(t, human, "either the graph moved, the harness broke, or the stratum could not be scored at all")
	mustNotContain(t, human, "no control stratum")
	mustNotContain(t, human, "budget alarm")
}

func TestReportCountsTheAnchorDuplicationAndSelfProducedCandidatesItFound(t *testing.T) {
	t.Parallel()

	row := oneMissOfEachKind()
	row.AnchorWasCandidate = true
	row.AnchorAdmittedAsCandidate = true
	row.SelfProducedCandidates = 4
	row.Shutout = true

	_, human := render(t, resultWith(row, intactControlRow()))

	mustContain(t, human, "anchor also a candidate    1/1 rows (admitted as a candidate in 1)")
	mustContain(t, human, "self-produced candidates   4 across 1 rows")
	mustContain(t, human, "shutouts                   1/1 rows")
}

func TestResultRowsAreEmittedInCorpusOrder(t *testing.T) {
	t.Parallel()

	third := oneMissOfEachKind()
	third.Row = "r03"
	first := oneMissOfEachKind()
	first.Row = "r01"
	second := oneMissOfEachKind()
	second.Row = "r02"

	machine, _ := render(t, resultWith(third, first, second))

	var decoded Result
	if err := json.Unmarshal([]byte(machine), &decoded); err != nil {
		t.Fatalf("the machine-readable result does not decode: %v", err)
	}
	for i, want := range []string{"r03", "r01", "r02"} {
		if decoded.Rows[i].Row != want {
			t.Fatalf("Rows[%d].Row = %q, want %q: rows must stay in corpus order so two results diff", i, decoded.Rows[i].Row, want)
		}
	}
}

func TestRenderWritesTheResultToItsMachineWriterAndTheSummaryToItsHumanWriter(t *testing.T) {
	t.Parallel()

	machine, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	var decoded Result
	if err := json.Unmarshal([]byte(machine), &decoded); err != nil {
		t.Fatalf("the machine writer did not receive decodable JSON: %v", err)
	}
	if len(decoded.Rows) != 2 {
		t.Fatalf("the machine writer received %d rows, want 2", len(decoded.Rows))
	}
	if decoded.CorpusHash == "" {
		t.Fatal("the machine writer received a result with no corpus hash")
	}

	if strings.Contains(human, "corpusHash") {
		t.Fatalf("the human writer received the machine-readable document; output was:\n%s", human)
	}
	if !strings.Contains(human, "retrieved") {
		t.Fatalf("the human writer received no summary; output was:\n%s", human)
	}
	if strings.Contains(machine, "retrieved 2/3") {
		t.Fatalf("the machine writer received the human summary; output was:\n%s", machine)
	}
}

func TestReportHeaderNamesTheCorpusItsRowCountsAndTheLimits(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(oneMissOfEachKind(), intactControlRow()))

	mustContain(t, human, "corpus "+corpusPathForTests)
	mustContain(t, human, "2 rows (1 labelled, 1 control)")
	mustContain(t, human, "hash 85484780")
	mustContain(t, human, fmt.Sprintf("limits candidateLimit=%d assemblyByteBudget=%d", loop.CandidateLimit, loop.AssemblyByteBudget))
}

type refusingWriter struct{}

func (refusingWriter) Write([]byte) (int, error) {
	return 0, errors.New("the destination rejected the write")
}

func TestRenderReportsAFailureToWriteTheMeasurement(t *testing.T) {
	t.Parallel()

	var human bytes.Buffer

	err := Render(resultWith(oneMissOfEachKind(), intactControlRow()), corpusPathForTests, refusingWriter{}, &human)

	if err == nil {
		t.Fatal("Render returned a nil error when the measurement could not be written, want an error")
	}
	if !strings.Contains(err.Error(), "write the measurement") {
		t.Fatalf("error = %q, want it to name the measurement as the write that failed", err.Error())
	}
}

func TestRenderReportsAFailureToWriteTheSummary(t *testing.T) {
	t.Parallel()

	var machine bytes.Buffer

	err := Render(resultWith(oneMissOfEachKind(), intactControlRow()), corpusPathForTests, &machine, refusingWriter{})

	if err == nil {
		t.Fatal("Render returned a nil error when the summary could not be written, want an error: the human stream is not a stream whose failure is free to discard")
	}
	if !strings.Contains(err.Error(), "write the summary") {
		t.Fatalf("error = %q, want it to name the summary as the write that failed", err.Error())
	}
	if machine.Len() == 0 {
		t.Fatal("the measurement was not written, want it emitted before the summary failed")
	}
}

func TestRenderNamesWhichOfTheTwoStreamsFailedRatherThanReportingAWriteFailure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	measurement := Render(resultWith(oneMissOfEachKind()), corpusPathForTests, refusingWriter{}, &buf)
	summary := Render(resultWith(oneMissOfEachKind()), corpusPathForTests, &buf, refusingWriter{})

	if measurement == nil || summary == nil {
		t.Fatal("Render returned a nil error for one of the two streams, want an error for both")
	}
	if measurement.Error() == summary.Error() {
		t.Fatalf("both stream failures report %q, want the failed stream named", measurement.Error())
	}
}

func mustContainLine(t *testing.T, output, want string) {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		if strings.TrimLeft(line, " ") == want {
			return
		}
	}
	t.Fatalf("no line of the output reads %q; output was:\n%s", want, output)
}

func missWithFiveCandidates() RowResult {
	return RowResult{
		Row:            "r01",
		Stratum:        StratumLabelled,
		Subject:        100,
		CandidateCount: 5,
		AdmittedCount:  3,
		AdmittedBytes:  32504,
		BudgetBytes:    loop.AssemblyByteBudget,
		TopSimilarity:  0.95,
		Required:       []NodeResult{{Node: 303, Verdict: NotRetrieved}},
		Candidates: []loop.Disposition{
			{Rank: 1, ID: 401, Similarity: 0.62, Included: true},
			{Rank: 2, ID: 402, Similarity: 0.91, Included: true},
			{Rank: 3, ID: 403, Similarity: 0.77, Included: true},
			{Rank: 7, ID: 404, Similarity: 0.95},
			{Rank: 8, ID: 405, Similarity: 0.48},
		},
	}
}

func missCutWithExactlyTwoCandidatesAbove() RowResult {
	return RowResult{
		Row:            "r22",
		Stratum:        StratumLabelled,
		Subject:        100,
		CandidateCount: 4,
		AdmittedCount:  1,
		AdmittedBytes:  56302,
		BudgetBytes:    loop.AssemblyByteBudget,
		TopSimilarity:  0.93,
		Required:       []NodeResult{{Node: 502, Verdict: Cut, Rank: 5}},
		Candidates: []loop.Disposition{
			{Rank: 1, ID: 511, Similarity: 0.66, Included: true},
			{Rank: 2, ID: 512, Similarity: 0.88},
			{Rank: 5, ID: 502, Similarity: 0.81},
			{Rank: 9, ID: 514, Similarity: 0.93},
		},
	}
}

func TestReportNamesTheThreeHighestRankedCandidatesThatOutrankedAMiss(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(missWithFiveCandidates(), intactControlRow()))

	mustContainLine(t, human, "outranked by   #401 (0.62)  #402 (0.91)  #403 (0.77)")
}

func TestReportNamesOnlyTheTwoCandidatesThatExistWhenFewerThanThreeOutrankedTheMiss(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(missCutWithExactlyTwoCandidatesAbove(), intactControlRow()))

	mustContainLine(t, human, "outranked by   #511 (0.66)  #512 (0.88)")
}

func TestReportNamesWhatOutrankedTheMissForEveryMissVerdictAlike(t *testing.T) {
	t.Parallel()

	cut := missWithFiveCandidates()
	cut.Row = "r02"
	cut.Required = []NodeResult{{Node: 304, Verdict: Cut, Rank: 4}}
	unresolved := missWithFiveCandidates()
	unresolved.Row = "r03"
	unresolved.Required = []NodeResult{{Node: 305, Verdict: Unresolved}}

	_, human := render(t, resultWith(missWithFiveCandidates(), cut, unresolved, intactControlRow()))

	if got := strings.Count(human, "outranked by"); got != 3 {
		t.Fatalf("the summary carries %d outranked-by lines for three misses whose verdicts are notRetrieved, cut and unresolved, want 3: the line is one rule that does not branch on verdict; output was:\n%s", got, human)
	}
}

func TestReportNamesNoOutrankingCandidatesForARowWhoseRequiredNodeWasAdmitted(t *testing.T) {
	t.Parallel()

	admitted := missWithFiveCandidates()
	admitted.Required = []NodeResult{{Node: 403, Verdict: Admitted, Rank: 3}}

	_, human := render(t, resultWith(admitted, intactControlRow()))

	if strings.Contains(human, "outranked by") {
		t.Fatalf("the summary names what outranked a required node that was admitted; the line belongs to a miss and to nothing else. Output was:\n%s", human)
	}
}

func TestReportNamesNoOutrankingCandidatesForAMissThatNothingOutranked(t *testing.T) {
	t.Parallel()

	rankOne := missCutWithExactlyTwoCandidatesAbove()
	rankOne.Required = []NodeResult{{Node: 511, Verdict: Cut, Rank: 1}}

	_, human := render(t, resultWith(rankOne, intactControlRow()))

	if strings.Contains(human, "outranked by") {
		t.Fatalf("the summary carries an outranked-by line for a miss at rank 1, which no candidate outranked; an empty list is not a line. Output was:\n%s", human)
	}
}

func candidatesOnTheWire(t *testing.T, machine, row string) string {
	t.Helper()

	var decoded struct {
		Rows []struct {
			Row        string          `json:"row"`
			Candidates json.RawMessage `json:"candidates"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(machine), &decoded); err != nil {
		t.Fatalf("the machine result does not decode: %v; body was:\n%s", err, machine)
	}
	for _, r := range decoded.Rows {
		if r.Row == row {
			return string(r.Candidates)
		}
	}
	t.Fatalf("the machine result carries no row %q; body was:\n%s", row, machine)
	return ""
}

func lineContaining(t *testing.T, output, want string) string {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("no line of the output contains %q; output was:\n%s", want, output)
	return ""
}

func TestTheMachineResultCarriesTheRetainedCandidateSetUnderItsOwnWireKey(t *testing.T) {
	t.Parallel()

	machine, _ := render(t, resultWith(missWithFiveCandidates(), intactControlRow()))

	var decoded struct {
		Rows []struct {
			Candidates []struct {
				Rank       int     `json:"rank"`
				ID         int64   `json:"id"`
				Similarity float64 `json:"similarity"`
				Included   bool    `json:"included"`
			} `json:"candidates"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(machine), &decoded); err != nil {
		t.Fatalf("the machine result does not decode: %v; body was:\n%s", err, machine)
	}

	got := decoded.Rows[0].Candidates
	if len(got) != 5 {
		t.Fatalf("the row carries %d candidates under the wire key \"candidates\", want 5: the retained evidence has to reach the result file, and a reader outside this package has only the key to find it by. Body was:\n%s", len(got), machine)
	}
	if got[3].Rank != 7 || got[3].ID != 404 || got[3].Similarity != 0.95 || got[3].Included {
		t.Fatalf("the fourth candidate on the wire is rank %d #%d at %v included=%v, want rank 7 #404 at 0.95 included=false: rank, id, similarity and admission all have to survive the encoding",
			got[3].Rank, got[3].ID, got[3].Similarity, got[3].Included)
	}
}

func TestTheMachineResultKeepsARowThatSweptNothingDistinctFromARowThatNeverSwept(t *testing.T) {
	t.Parallel()

	_, dispositions := loop.Assemble(anchorNode(), nil, loop.AssemblyByteBudget)
	swept := BuildRow(labelledRow(Required{Node: 201, Hash: "h", Why: "w"}), dispositions)
	neverSwept := RowResult{Row: "r99", Stratum: StratumLabelled, Subject: 999, Error: "subject not found"}

	machine, _ := render(t, resultWith(swept, neverSwept, intactControlRow()))

	if got := candidatesOnTheWire(t, machine, "r01"); got != "[]" {
		t.Fatalf("a row whose recall returned nothing carries %q on the wire, want \"[]\": it swept and found none, which is not the same fact as never having swept. Body was:\n%s", got, machine)
	}
	if got := candidatesOnTheWire(t, machine, "r99"); got != "null" {
		t.Fatalf("a row the sweep could not reach carries %q on the wire, want \"null\": it produced no candidate set at all, which is not the same fact as an empty one. Body was:\n%s", got, machine)
	}
}

func TestReportIndentsTheOutrankedLineUnderTheVerdictItExplains(t *testing.T) {
	t.Parallel()

	_, human := render(t, resultWith(missWithFiveCandidates(), intactControlRow()))

	miss := lineContaining(t, human, "notRetrieved")
	outranked := lineContaining(t, human, "outranked by")

	if got := strings.Index(miss, "notRetrieved"); got != 19 {
		t.Fatalf("the miss line starts its verdict at column %d, want 19; the outranked-by line indents to that same column, so the two move together or the block stops lining up. Line was:\n%q", got, miss)
	}
	if got := strings.Index(outranked, "outranked by"); got != 19 {
		t.Fatalf("the outranked-by line starts its label at column %d, want 19: it sits under the verdict it explains, which is the relation design section 8.3's specimen shows. The column is this tree's, not the specimen's -- the specimen indents to 15. Line was:\n%q", got, outranked)
	}
	if got := strings.Index(outranked[19:], "#"); got != 15 {
		t.Fatalf("the outranked-by line starts its candidates %d columns after its label, want 15: the payload sits under the verdict's payload. Line was:\n%q", got, outranked)
	}
}
