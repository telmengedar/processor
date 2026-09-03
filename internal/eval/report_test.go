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
