package loop

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	summaryInputRunes  = 200
	summaryAnchorRunes = 56
	summaryQueryRunes  = 88
	summaryRecallRunes = 72
	summaryTypeRunes   = 13
	summaryNameRunes   = 46
	summaryAnswerRunes = 180
	summaryLineRunes   = 96
	summaryErrorRunes  = 200
)

const (
	summaryExactByteBound = 10000

	summaryRunPrefix = "processor-run"
)

const (
	summaryNoProvider       = "[provider not recorded]"
	summaryNoTool           = "[tool not recorded]"
	summaryAnsweredYetEmpty = "  <-- terminal reason says answered"
	summaryBlockOmitted     = "not rendered here"
)

type cutGroup struct {
	reason string
	rows   []Disposition
}

// RenderSummary projects one run record onto a compact summary of that run, reading only fields the record carries and never the assembled block.
func RenderSummary(record Record, at time.Time) string {
	var b strings.Builder

	renderSummaryHeader(&b, record, at)
	renderSummaryAssembly(&b, record)
	renderSummaryTools(&b, record)
	renderSummaryOutcome(&b, record)

	return b.String()
}

func renderSummaryHeader(b *strings.Builder, record Record, at time.Time) {
	fmt.Fprintf(b, "%s at %s\n", summaryRunPrefix, at.UTC().Format(time.RFC3339))
	fmt.Fprintf(b, "input    %s\n", summaryTrunc(record.Input, summaryInputRunes))
	fmt.Fprintf(b, "subject  #%d %s %q (%s)\n",
		record.Anchor.ID, record.Anchor.Type, summaryTrunc(record.Anchor.Name, summaryAnchorRunes), summaryBytes(record.Anchor.Size))

	if record.Provider.Adapter == "" && record.Provider.Endpoint == "" {
		fmt.Fprintf(b, "model    %s   %s\n", record.Model, summaryNoProvider)
	} else {
		fmt.Fprintf(b, "model    %s via %s %s\n", record.Model, record.Provider.Adapter, record.Provider.Endpoint)
	}

	limits := record.Limits
	fmt.Fprintf(b, "limits   %d cands / %d B content / %d B suppl / %d calls / %d tok%s\n",
		limits.CandidateLimit, limits.AssemblyByteBudget, limits.SupplementaryByteBudget,
		limits.MaxModelCalls, limits.MaxOutputTokens, summarySampling(record.Sampling))

	if record.Workspace != "" {
		fmt.Fprintf(b, "workdir  %s\n", record.Workspace)
	}
}

func renderSummaryAssembly(b *strings.Builder, record Record) {
	admitted, cut := splitDispositions(record.Candidates)

	fmt.Fprintf(b, "\nASSEMBLY  %s, %d candidates -> %d admitted / %d cut\n",
		summaryPlural(len(record.Queries), "query", "queries"),
		len(record.Candidates), len(admitted), len(record.Candidates)-len(admitted))

	for i, query := range record.Queries {
		fmt.Fprintf(b, "  q%d: %s\n", i, summaryTrunc(query, summaryQueryRunes))
	}

	if record.DerivationError != "" {
		fmt.Fprintf(b, "  derivation: %s\n", summaryTrunc(record.DerivationError, summaryErrorRunes))
	}

	fmt.Fprintf(b, "  admitted (%d, %s of %d B remaining):\n",
		len(admitted), summaryBytes(sumDispositionSizes(admitted)), summaryRemainingAfterAnchor(record.Limits, record.Anchor.Size))
	for _, d := range admitted {
		fmt.Fprintf(b, "    #%-6d %.3f %-13s %8s  %s\n",
			d.ID, d.Similarity, summaryTrunc(d.Type, summaryTypeRunes), summaryBytes(d.Size), summaryTrunc(d.Name, summaryNameRunes))
	}

	if len(cut) == 0 {
		return
	}

	fmt.Fprintf(b, "  cut (%d):\n", len(record.Candidates)-len(admitted))
	for _, group := range cut {
		renderCutGroup(b, group, "    ")
	}
}

func renderCutGroup(b *strings.Builder, group cutGroup, indent string) {
	head := fmt.Sprintf("%s%s (%d, %s):", indent, group.reason, len(group.rows), summaryBytes(sumDispositionSizes(group.rows)))
	ids := dispositionIDs(group.rows)

	if oneLine := head + " " + strings.Join(ids, " "); len([]rune(oneLine)) <= summaryLineRunes {
		b.WriteString(oneLine + "\n")
		return
	}

	b.WriteString(head + "\n")
	line := indent + "  "
	for _, id := range ids {
		if len([]rune(line))+len(id) > summaryLineRunes {
			b.WriteString(strings.TrimRight(line, " ") + "\n")
			line = indent + "  "
		}
		line += id + " "
	}
	if strings.TrimSpace(line) != "" {
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
}

func renderSummaryTools(b *strings.Builder, record Record) {
	fmt.Fprintf(b, "\nTOOLS  %s\n", summaryPlural(len(record.ToolCalls), "round", "rounds"))

	for i, call := range record.ToolCalls {
		source := ""
		if call.Source != "" {
			source = fmt.Sprintf(" [%s]", call.Source)
		}

		tool := call.Tool
		if tool == "" {
			tool = summaryNoTool
		}

		switch {
		case call.Tool == ToolRecall, call.Tool == "" && call.Query != "":
			fmt.Fprintf(b, "  %d %s%s  %q\n", i+1, tool, source, summaryTrunc(call.Query, summaryRecallRunes))
			renderRecallResults(b, call.Results)
		case call.Tool == "" && call.Path == "":
			fmt.Fprintf(b, "  %d %s%s\n", i+1, tool, source)
		default:
			fmt.Fprintf(b, "  %d %s%s  %q  %s\n", i+1, tool, source, summaryTrunc(call.Path, summaryRecallRunes), summaryBytes(call.Bytes))
		}

		if call.Error != "" {
			fmt.Fprintf(b, "      ERROR: %s\n", summaryTrunc(call.Error, summaryErrorRunes))
		}
	}
}

func renderRecallResults(b *strings.Builder, results []Disposition) {
	if len(results) == 0 {
		b.WriteString("      -> no results recorded\n")
		return
	}

	admitted, cut := splitDispositions(results)

	reasons := make([]string, 0, len(cut))
	for _, group := range cut {
		reasons = append(reasons, fmt.Sprintf("%s %d", group.reason, len(group.rows)))
	}
	cutText := ""
	if len(reasons) > 0 {
		cutText = " / cut: " + strings.Join(reasons, ", ")
	}

	fmt.Fprintf(b, "      -> %d results, %d admitted (%s)%s\n",
		len(results), len(admitted), summaryBytes(sumDispositionSizes(admitted)), cutText)

	if len(admitted) == 0 {
		b.WriteString("      (none admitted)\n")
		return
	}
	b.WriteString("      " + strings.Join(dispositionIDs(admitted), " ") + "\n")
}

func renderSummaryOutcome(b *strings.Builder, record Record) {
	reached := "not reached"
	if record.CapReached {
		reached = "reached"
	}

	fmt.Fprintf(b, "\nOUTCOME  %s (raw %q), %d/%d model calls, cap %s\n",
		record.StopReason.Reason, record.StopReason.Raw, record.ModelCalls, record.Limits.MaxModelCalls, reached)

	renderSummaryAnswer(b, record)
	renderSummaryTokens(b, record.Usage)

	fmt.Fprintf(b, "  [block  %d B assembled, %s]\n", len(record.Block), summaryBlockOmitted)
}

func renderSummaryAnswer(b *strings.Builder, record Record) {
	if record.Answer == "" {
		flag := ""
		if record.StopReason.Reason == Answered {
			flag = summaryAnsweredYetEmpty
		}
		fmt.Fprintf(b, "  answer   EMPTY (0 B)%s\n", flag)
		return
	}

	excerpt, truncated := summaryExcerpt(record.Answer, summaryAnswerRunes)
	fmt.Fprintf(b, "  answer   %s\n", summaryBytes(len(record.Answer)))
	fmt.Fprintf(b, "    | %s\n", excerpt)
	if truncated {
		fmt.Fprintf(b, "    | (excerpt; full answer is %d B in the record)\n", len(record.Answer))
	}
}

func renderSummaryTokens(b *strings.Builder, usage []*Usage) {
	in, out := 0, 0
	perCall := make([]string, 0, len(usage))
	for _, u := range usage {
		if u == nil {
			continue
		}
		in += u.InTokens
		out += u.OutTokens
		perCall = append(perCall, strconv.Itoa(u.OutTokens))
	}

	if len(perCall) == 0 {
		b.WriteString("  tokens   not recorded\n")
		return
	}

	fmt.Fprintf(b, "  tokens   %d in / %d out over %d calls  (out per call: %s)\n",
		in, out, len(perCall), strings.Join(perCall, ", "))
}

func splitDispositions(dispositions []Disposition) (admitted []Disposition, cut []cutGroup) {
	at := make(map[string]int, len(dispositions))

	for _, d := range dispositions {
		if d.Included {
			admitted = append(admitted, d)
			continue
		}

		reason := d.CutReason

		if i, seen := at[reason]; seen {
			cut[i].rows = append(cut[i].rows, d)
			continue
		}
		at[reason] = len(cut)
		cut = append(cut, cutGroup{reason: reason, rows: []Disposition{d}})
	}

	return admitted, cut
}

func dispositionIDs(dispositions []Disposition) []string {
	ids := make([]string, len(dispositions))
	for i, d := range dispositions {
		ids[i] = "#" + strconv.FormatInt(d.ID, 10)
	}
	return ids
}

func sumDispositionSizes(dispositions []Disposition) int {
	total := 0
	for _, d := range dispositions {
		total += d.Size
	}
	return total
}

func summaryRemainingAfterAnchor(limits Limits, anchorSize int) int {
	remaining := limits.AssemblyByteBudget - anchorSize
	if remaining < 0 {
		return 0
	}
	return remaining
}

func summarySampling(sampling Sampling) string {
	var bits []string
	if sampling.Temperature != nil {
		bits = append(bits, fmt.Sprintf("temp=%g", *sampling.Temperature))
	}
	if sampling.TopP != nil {
		bits = append(bits, fmt.Sprintf("topP=%g", *sampling.TopP))
	}
	if len(bits) == 0 {
		return ""
	}
	return "  " + strings.Join(bits, " ")
}

func summaryPlural(n int, singular, plural string) string {
	if n == 1 {
		return strconv.Itoa(n) + " " + singular
	}
	return strconv.Itoa(n) + " " + plural
}

func summaryBytes(n int) string {
	if n < summaryExactByteBound {
		return strconv.Itoa(n) + " B"
	}
	return fmt.Sprintf("%.1f kB", float64(n)/1000)
}

func summaryTrunc(s string, limit int) string {
	text, _ := summaryExcerpt(s, limit)
	return text
}

func summaryExcerpt(s string, limit int) (string, bool) {
	flat := strings.Join(strings.Fields(s), " ")
	runes := []rune(flat)
	if len(runes) <= limit {
		return flat, false
	}
	return string(runes[:limit-1]) + "…", true
}
