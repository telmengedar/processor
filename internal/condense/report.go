package condense

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

const unsetSampling = "(unset)"

// Render writes the pass result as JSON to machine and as the operator-facing audit bundle to human.
func Render(result Result, machine, human io.Writer) error {
	encoder := json.NewEncoder(machine)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("condense: encode result: %w", err)
	}

	writeHeader(human, result)
	writeSampling(human, result)
	writeRatios(human, result)
	writeSkips(human, result)
	writeAudit(human, result)
	return nil
}

func writeHeader(w io.Writer, result Result) {
	mode := "write"
	if result.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(w, "condensation pass %s — %s\n", mode, result.RanAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(w, "targets %d · condensed %d · skipped %d · operational failures %d\n\n",
		result.TargetCount, len(result.Provenance), len(result.Skipped), result.OperationalFailures())
}

func writeSampling(w io.Writer, result Result) {
	fmt.Fprintln(w, "model and sampling, read back from the call rather than from configuration")
	if len(result.Provenance) == 0 {
		fmt.Fprintln(w, "  no call completed, so nothing was read back")
		fmt.Fprintln(w)
		return
	}

	for _, line := range distinctSampling(result.Provenance) {
		fmt.Fprintf(w, "  %s\n", line)
	}
	low, high := outputCeilingRange(result.Provenance)
	fmt.Fprintf(w, "  max_tokens %d..%d\n\n", low, high)
}

func outputCeilingRange(provenance []Provenance) (int, int) {
	low, high := provenance[0].Sampling.MaxTokens, provenance[0].Sampling.MaxTokens
	for _, p := range provenance[1:] {
		if p.Sampling.MaxTokens < low {
			low = p.Sampling.MaxTokens
		}
		if p.Sampling.MaxTokens > high {
			high = p.Sampling.MaxTokens
		}
	}
	return low, high
}

func distinctSampling(provenance []Provenance) []string {
	seen := make(map[string]bool, len(provenance))
	lines := make([]string, 0, 1)
	for _, p := range provenance {
		line := fmt.Sprintf("model %s · temperature %s · top_p %s · frequency_penalty %s · presence_penalty %s",
			orUnset(p.Model), floatOrUnset(p.Sampling.Temperature), floatOrUnset(p.Sampling.TopP),
			trimFloat(p.Sampling.FrequencyPenalty), trimFloat(p.Sampling.PresencePenalty))
		if seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}

func writeRatios(w io.Writer, result Result) {
	fmt.Fprintln(w, "ratio — len(substance) / len(content), per node")
	if len(result.Provenance) == 0 {
		fmt.Fprintln(w, "  nothing was condensed")
		fmt.Fprintln(w)
		return
	}

	fmt.Fprintf(w, "  %-8s %-10s %10s %10s %8s  %s\n", "node", "rows", "content", "substance", "ratio", "clauses")
	for _, p := range result.Provenance {
		fmt.Fprintf(w, "  %-8d %-10s %10d %10d %8.3f  %s\n",
			p.Node, rowsFor(result, p.Node), p.ContentSize, p.SubstanceSize, p.Ratio, strings.Join(p.Clauses, "+"))
	}
	fmt.Fprintf(w, "  count %d · min %.3f · median %.3f · mean %.3f · max %.3f\n\n",
		result.Ratio.Count, result.Ratio.Min, result.Ratio.Median, result.Ratio.Mean, result.Ratio.Max)
}

func writeSkips(w io.Writer, result Result) {
	fmt.Fprintln(w, "skipped, by rule")
	if len(result.Skipped) == 0 {
		fmt.Fprintln(w, "  nothing was skipped")
		fmt.Fprintln(w)
		return
	}

	byReason := make(map[string][]string)
	reasons := make([]string, 0)
	for _, s := range result.Skipped {
		if _, seen := byReason[s.Reason]; !seen {
			reasons = append(reasons, s.Reason)
		}
		byReason[s.Reason] = append(byReason[s.Reason], strconv.FormatInt(s.Node, 10))
	}
	sort.Strings(reasons)

	for _, reason := range reasons {
		nodes := byReason[reason]
		fmt.Fprintf(w, "  %-40s %3d   %s\n", reason, len(nodes), strings.Join(nodes, " "))
	}

	for _, s := range result.Skipped {
		if s.Detail != "" {
			fmt.Fprintf(w, "  node %d · %s · %s\n", s.Node, s.Reason, s.Detail)
		}
	}
	fmt.Fprintln(w)
}

func writeAudit(w io.Writer, result Result) {
	fmt.Fprintf(w, "fidelity audit — %d required nodes, each pre-registered reason beside the substance the node now carries\n", len(result.Audit))
	fmt.Fprintln(w, "the verdict is the reviewer's; this bundle is evidence and states none")
	fmt.Fprintln(w)

	for i, entry := range result.Audit {
		fmt.Fprintf(w, "[%d/%d] node %d · rows %s · origin %s · %d B from %d B · ratio %.3f\n",
			i+1, len(result.Audit), entry.Node, strings.Join(entry.Rows, ","), entry.Origin,
			entry.SubstanceSize, entry.ContentSize, entry.Ratio)
		fmt.Fprintf(w, "name: %s\n", entry.Name)
		for _, why := range entry.Why {
			fmt.Fprintf(w, "why:  %s\n", why)
		}
		fmt.Fprintln(w, "substance:")
		if entry.Substance == "" {
			fmt.Fprintln(w, "  (none — this node carries no substance)")
		} else {
			for _, line := range strings.Split(entry.Substance, "\n") {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
		fmt.Fprintln(w, strings.Repeat("-", 78))
	}
}

func rowsFor(result Result, node int64) string {
	for _, entry := range result.Audit {
		if entry.Node == node {
			return strings.Join(entry.Rows, ",")
		}
	}
	return ""
}

func orUnset(s string) string {
	if s == "" {
		return unsetSampling
	}
	return s
}

func floatOrUnset(f *float64) string {
	if f == nil {
		return unsetSampling
	}
	return trimFloat(*f)
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
