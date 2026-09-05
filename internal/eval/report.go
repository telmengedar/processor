package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

const hashPrefixLength = 8

const outrankedCandidateCount = 3

const (
	alarmControlAbsent = "no control stratum: this sweep verified nothing about itself, so a broken harness would report a plausible number"
	alarmControlBroken = "the control stratum did not verify retrieval: either the graph moved, the harness broke, or the stratum could not be scored at all, and this sweep's labelled number is not trustworthy"
	alarmControlBudget = "budget alarm: the control stratum was retrieved in full and cut by the budget. Retrieval is intact and this sweep's retrieved rate is trustworthy; the admitted rate is a reading of the assembler, not of the retriever."
)

type rate struct {
	retrieved int
	admitted  int
	total     int
}

// Render writes the machine-readable result to machine and the human summary to human.
func Render(result Result, corpusPath string, machine, human io.Writer) error {
	encoder := json.NewEncoder(machine)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("eval: write the measurement: %w", err)
	}

	summary := &latchingWriter{w: human}
	writeSummary(result, corpusPath, summary)
	if summary.err != nil {
		return fmt.Errorf("eval: write the summary: %w", summary.err)
	}

	return nil
}

type latchingWriter struct {
	w   io.Writer
	err error
}

func (l *latchingWriter) Write(p []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}

	n, err := l.w.Write(p)
	l.err = err
	return n, err
}

func writeSummary(result Result, corpusPath string, w io.Writer) {
	fmt.Fprintf(w, "corpus %s - %d rows (%d labelled, %d control), hash %s\n",
		corpusPath, result.RowCount, countRows(result, StratumLabelled), countRows(result, StratumControl), shortHash(result.CorpusHash))
	fmt.Fprintf(w, "arm %s\n", armLine(result))
	fmt.Fprintf(w, "limits candidateLimit=%d assemblyByteBudget=%d recallScopeReserve=%d\n\n",
		result.Limits.CandidateLimit, result.Limits.AssemblyByteBudget, result.Limits.RecallScopeReserve)

	writeRate(w, StratumLabelled, rateOf(result, StratumLabelled))
	writeRate(w, StratumControl, rateOf(result, StratumControl))

	if alarm := controlAlarm(result); alarm != "" {
		fmt.Fprintf(w, "\n%s\n", alarm)
	}

	writeMisses(w, result, StratumLabelled)
	writeMisses(w, result, StratumControl)
	writeRowErrors(w, result)
	writeDiagnostics(w, result)
}

func armLine(result Result) string {
	if result.Arm == ArmRawInput {
		return ArmRawInput
	}
	if !result.ProvenanceRecorded {
		return fmt.Sprintf("%s - %d/%d rows derived, provenance unrecorded, hash %s",
			result.Arm, result.DerivedRows, result.RowCount, shortHash(result.DerivationHash))
	}
	return fmt.Sprintf("%s - %d/%d rows derived, %d hand-authored, %d blind-generated, hash %s",
		result.Arm, result.DerivedRows, result.RowCount, result.HandAuthoredRows, result.BlindGeneratedRows, shortHash(result.DerivationHash))
}

func controlAlarm(result Result) string {
	if countRows(result, StratumControl) == 0 {
		return alarmControlAbsent
	}
	if !result.ControlVerifiedRetrieval() {
		return alarmControlBroken
	}
	if result.controlWasCut() {
		return alarmControlBudget
	}
	return ""
}

func writeRate(w io.Writer, stratum string, r rate) {
	fmt.Fprintf(w, "%-9s retrieved %d/%d (%s)   admitted %d/%d (%s)\n",
		stratum, r.retrieved, r.total, ratio(r.retrieved, r.total), r.admitted, r.total, ratio(r.admitted, r.total))
}

func writeMisses(w io.Writer, result Result, stratum string) {
	lines := missLines(result, stratum)
	if len(lines) == 0 {
		return
	}

	fmt.Fprintf(w, "\nmisses (%s):\n", stratum)
	for _, line := range lines {
		fmt.Fprintf(w, "%s\n", line)
	}
}

func missLines(result Result, stratum string) []string {
	var lines []string
	for _, row := range result.Rows {
		if row.Stratum != stratum {
			continue
		}
		for _, node := range row.Required {
			if node.Verdict == Admitted {
				continue
			}
			lines = append(lines, missLine(row, node))
			if outranked := outrankedLine(row, node); outranked != "" {
				lines = append(lines, outranked)
			}
		}
	}
	return lines
}

func missPrefix(row RowResult, node NodeResult) string {
	return fmt.Sprintf("  %-5s #%-9d ", row.Row, node.Node)
}

func missLine(row RowResult, node NodeResult) string {
	switch node.Verdict {
	case Cut:
		return missPrefix(row, node) + fmt.Sprintf("cut at rank %-4d k'=%d, %d/%d bytes admitted%s",
			node.Rank, row.AdmittedCount, row.AdmittedBytes, row.BudgetBytes, staleSuffix(node))
	case Unresolved:
		return missPrefix(row, node) + "unresolved     the row is excluded from both rates"
	default:
		return missPrefix(row, node) + fmt.Sprintf("notRetrieved   %d candidates, top similarity %.4f%s",
			row.CandidateCount, row.TopSimilarity, staleSuffix(node))
	}
}

func outrankedLine(row RowResult, node NodeResult) string {
	above := outranking(row, node)
	if len(above) == 0 {
		return ""
	}

	named := make([]string, 0, len(above))
	for _, d := range above {
		named = append(named, fmt.Sprintf("#%d (%.2f)", d.ID, d.Similarity))
	}
	return strings.Repeat(" ", len(missPrefix(row, node))) + "outranked by   " + strings.Join(named, "  ")
}

func outranking(row RowResult, node NodeResult) []loop.Disposition {
	above := make([]loop.Disposition, 0, outrankedCandidateCount)
	for _, d := range row.Candidates {
		if node.Rank != 0 && d.Rank >= node.Rank {
			continue
		}
		above = append(above, d)
		if len(above) == outrankedCandidateCount {
			break
		}
	}
	return above
}

func staleSuffix(node NodeResult) string {
	if node.Stale {
		return ", label stale"
	}
	return ""
}

func writeRowErrors(w io.Writer, result Result) {
	errored := false
	for _, row := range result.Rows {
		if row.Error == "" {
			continue
		}
		if !errored {
			fmt.Fprint(w, "\nrow errors (not scored):\n")
			errored = true
		}
		fmt.Fprintf(w, "  %-5s %s\n", row.Row, row.Error)
	}
}

type diagnostics struct {
	rows               int
	anchorWasCandidate int
	anchorAdmitted     int
	selfProducedRows   int
	selfProduced       int
	shutouts           int
	stale              int
	unresolved         int
}

func writeDiagnostics(w io.Writer, result Result) {
	d := collectDiagnostics(result)

	fmt.Fprint(w, "\ndiagnostics (labelled rows):\n")
	fmt.Fprintf(w, "  %-26s %d/%d rows (admitted as a candidate in %d)\n", "anchor also a candidate", d.anchorWasCandidate, d.rows, d.anchorAdmitted)
	fmt.Fprintf(w, "  %-26s %d across %d rows\n", "self-produced candidates", d.selfProduced, d.selfProducedRows)
	fmt.Fprintf(w, "  %-26s %d/%d rows\n", "shutouts", d.shutouts, d.rows)
	fmt.Fprintf(w, "  %-26s %d\n", "stale labels", d.stale)
	fmt.Fprintf(w, "  %-26s %d (excluded from both rates)\n", "unresolved required nodes", d.unresolved)
}

func collectDiagnostics(result Result) diagnostics {
	var d diagnostics

	for _, row := range result.Rows {
		if row.Stratum != StratumLabelled {
			continue
		}
		d.rows++
		if row.AnchorWasCandidate {
			d.anchorWasCandidate++
		}
		if row.AnchorAdmittedAsCandidate {
			d.anchorAdmitted++
		}
		if row.SelfProducedCandidates > 0 {
			d.selfProducedRows++
			d.selfProduced += row.SelfProducedCandidates
		}
		if row.Shutout {
			d.shutouts++
		}
		for _, node := range row.Required {
			if node.Stale {
				d.stale++
			}
			if node.Verdict == Unresolved {
				d.unresolved++
			}
		}
	}

	return d
}

func rateOf(result Result, stratum string) rate {
	var r rate
	for _, row := range result.Rows {
		if row.Stratum != stratum || !row.Scored() {
			continue
		}
		for _, node := range row.Required {
			r.total++
			switch node.Verdict {
			case Admitted:
				r.retrieved++
				r.admitted++
			case Cut:
				r.retrieved++
			}
		}
	}
	return r
}

func countRows(result Result, stratum string) int {
	count := 0
	for _, row := range result.Rows {
		if row.Stratum == stratum {
			count++
		}
	}
	return count
}

func ratio(numerator, denominator int) string {
	if denominator == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", float64(numerator)/float64(denominator))
}

func shortHash(hash string) string {
	if len(hash) <= hashPrefixLength {
		return hash
	}
	return hash[:hashPrefixLength]
}
