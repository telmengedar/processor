package runbackfill

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Render writes the pass result as JSON to machine and as the operator-facing report to human.
func Render(result Result, machine, human io.Writer) error {
	encoder := json.NewEncoder(machine)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("runbackfill: encode result: %w", err)
	}

	writeHeader(human, result)
	writeProvenance(human, result)
	writeSkips(human, result)
	writeAccounts(human, result)
	return nil
}

func writeHeader(w io.Writer, result Result) {
	mode := "write"
	if result.DryRun {
		mode = "dry-run"
	}
	forced := ""
	if result.Forced {
		forced = " (forced)"
	}
	fmt.Fprintf(w, "run-record backfill %s%s — %s\n", mode, forced, result.RanAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(w, "targets %d · composed %d · skipped %d · operational failures %d\n\n",
		result.TargetCount, len(result.Provenance), len(result.Skipped), result.OperationalFailures())
}

func writeProvenance(w io.Writer, result Result) {
	if len(result.Provenance) == 0 {
		fmt.Fprintln(w, "nothing was composed")
		fmt.Fprintln(w)
		return
	}

	fmt.Fprintf(w, "  %-8s %-24s %10s %10s %10s  %-9s %-9s %s\n",
		"node", "at", "record", "account", "new", "backfld?", "written?", "name")
	for _, p := range result.Provenance {
		fmt.Fprintf(w, "  %-8d %-24s %10d %10d %10d  %-9v %-9s %s\n",
			p.Node, p.At.Format("2006-01-02T15:04:05Z"), p.RecordSize, p.AccountSize, p.NewContentSize,
			p.AlreadyBackfilled, writtenState(p), truncate(p.Name, 60))
	}
	fmt.Fprintln(w)
}

func writtenState(p Provenance) string {
	switch {
	case p.ContentWritten && p.SubstanceWritten:
		return "both"
	case p.ContentWritten:
		return "content"
	default:
		return "no (dry-run)"
	}
}

func writeSkips(w io.Writer, result Result) {
	if len(result.Skipped) == 0 {
		fmt.Fprintln(w, "nothing was skipped")
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

	fmt.Fprintln(w, "skipped")
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

func writeAccounts(w io.Writer, result Result) {
	if len(result.Provenance) == 0 {
		return
	}
	fmt.Fprintln(w, "rendered accounts, in full — this is what a reviewer checks against each record")
	fmt.Fprintln(w, strings.Repeat("=", 78))
	for i, p := range result.Provenance {
		fmt.Fprintf(w, "[%d/%d] node %d · %s\n", i+1, len(result.Provenance), p.Node, p.Name)
		fmt.Fprintln(w, strings.Repeat("-", 78))
		fmt.Fprintln(w, p.Account)
		fmt.Fprintln(w, strings.Repeat("=", 78))
	}
}

func truncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
