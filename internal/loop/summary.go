package loop

import (
	"fmt"
	"maps"
	"slices"
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
	summaryFillRunes   = 72
	summaryLineRunes   = 96
	summaryErrorRunes  = 200
)

const (
	summaryExactByteBound = 10000

	summaryRunPrefix = "processor-run"
)

const (
	summaryNoProvider   = "[provider not recorded]"
	summaryAbsentField  = "—"
	summaryNoTool       = "[tool not recorded]"
	summaryBlockOmitted = "not rendered here"
	summaryNoFillModel  = "[model not recorded]"
	summaryRecallClosed = "recall closed"
)

type cutGroup struct {
	reason string
	rows   []Disposition
}

type fillGroup struct {
	reason string
	ids    []string
}

// RenderSummary projects one run record onto a compact summary of that run, reading only fields the record carries and never the assembled block.
func RenderSummary(record Record, at time.Time) string {
	var b strings.Builder

	renderSummaryHeader(&b, record, at)
	renderSummaryAssembly(&b, record)
	renderSummaryFills(&b, record)
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
	fmt.Fprintf(b, "limits   %d cands / %d B content / %d B suppl / %d calls / %s tok derive / %s tok judge%s\n",
		limits.CandidateLimit, limits.AssemblyByteBudget, limits.SupplementaryByteBudget,
		limits.MaxModelCalls, summaryBudget(limits.DerivationBudget), summaryBudget(limits.JudgementBudget), summarySampling(record.Sampling))

	if record.Workspace != "" {
		fmt.Fprintf(b, "workdir  %s\n", record.Workspace)
	}
}

func summaryBudget(tokens int) string {
	if tokens == 0 {
		return summaryAbsentField
	}
	return strconv.Itoa(tokens)
}

func renderSummaryAssembly(b *strings.Builder, record Record) {
	admitted, cut := splitDispositions(record.Candidates)

	fmt.Fprintf(b, "\nASSEMBLY  %s, %d candidates -> %d admitted / %d cut\n",
		summaryPlural(len(record.Queries), "query", "queries"),
		len(record.Candidates), len(admitted), len(record.Candidates)-len(admitted))

	if !record.Window.IsZero() {
		fmt.Fprintf(b, "  %s\n", windowLine(record.Window))
	}

	for i, query := range record.Queries {
		fmt.Fprintf(b, "  q%d: %s\n", i, summaryTrunc(query, summaryQueryRunes))
	}

	if record.DerivationError != "" {
		fmt.Fprintf(b, "  derivation: %s\n", summaryTrunc(record.DerivationError, summaryErrorRunes))
	}

	fmt.Fprintf(b, "  admitted (%d, %s of %d B remaining):\n",
		len(admitted), summaryBytes(sumRenderedSizes(admitted)), summaryRemainingAfterAnchor(record.Limits, record.Anchor.Size))
	for _, d := range admitted {
		fmt.Fprintf(b, "    #%-6d %.3f %-13s %8s%s  %s\n",
			d.ID, d.Similarity, summaryTrunc(d.Type, summaryTypeRunes), summaryBytes(d.RenderedSize), summaryForm(d), summaryTrunc(d.Name, summaryNameRunes))
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
	head := fmt.Sprintf("%s%s (%d, %s):", indent, group.reason, len(group.rows), summaryBytes(sumRenderedSizes(group.rows)))
	renderIDGroup(b, head, dispositionIDs(group.rows), indent)
}

func renderIDGroup(b *strings.Builder, head string, ids []string, indent string) {
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

func renderSummaryFills(b *strings.Builder, record Record) {
	filled, refused := splitFills(record.Fills)

	limits := record.Limits
	fmt.Fprintf(b, "\nFILLS  %d filled / %d refused  (ceiling %d, floor %s, max %s)\n",
		len(filled), len(record.Fills)-len(filled), limits.MaxFills, summaryBytes(limits.FillSizeFloor), summaryBytes(limits.MaxFillContentBytes))

	for _, f := range filled {
		model := f.Model
		if model == "" {
			model = summaryNoFillModel
		}
		fmt.Fprintf(b, "  #%d written by %s\n", f.ID, summaryTrunc(model, summaryFillRunes))
	}

	for _, group := range refused {
		head := fmt.Sprintf("  %s (%d):", summaryTrunc(group.reason, summaryFillRunes), len(group.ids))
		renderIDGroup(b, head, group.ids, "  ")
	}
}

func splitFills(fills []FillOutcome) (filled []FillOutcome, refused []fillGroup) {
	at := make(map[string]int, len(fills))

	for _, f := range fills {
		if f.Filled {
			filled = append(filled, f)
			continue
		}

		id := "#" + strconv.FormatInt(f.ID, 10)
		if i, seen := at[f.Reason]; seen {
			refused[i].ids = append(refused[i].ids, id)
			continue
		}
		at[f.Reason] = len(refused)
		refused = append(refused, fillGroup{reason: f.Reason, ids: []string{id}})
	}

	return filled, refused
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
		len(results), len(admitted), summaryBytes(sumRenderedSizes(admitted)), cutText)

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

	fmt.Fprintf(b, "\nOUTCOME  %s (raw %q), %d/%d model calls, cap %s%s\n",
		record.StopReason.Reason, record.StopReason.Raw, record.ModelCalls, record.Limits.MaxModelCalls, reached, summaryRecallBound(record))

	if record.TimeShortfall != "" {
		fmt.Fprintf(b, "  stopped  %s\n", summaryTrunc(record.TimeShortfall, summaryErrorRunes))
	}

	if record.ReservedCall.State != "" {
		fmt.Fprintf(b, "  reserved %s\n", summaryReservedCall(record.ReservedCall))
	}

	renderSummaryVerdict(b, ComputeOutcome(record))
	renderSummaryAnswer(b, record)
	renderSummaryTokens(b, record.Usage)

	fmt.Fprintf(b, "  [block  %d B assembled, %s]\n", len(record.Block), summaryBlockOmitted)
}

func summaryReservedCall(reserved ReservedCall) string {
	if reserved.Error == "" {
		return string(reserved.State)
	}
	return string(reserved.State) + ": " + summaryTrunc(reserved.Error, summaryErrorRunes)
}

func summaryRecallBound(record Record) string {
	if !record.RecallClosed {
		return ""
	}
	return ", " + summaryRecallClosed
}

func renderSummaryVerdict(b *strings.Builder, outcome Outcome) {
	fmt.Fprintf(b, "  verdict  %s (produced %s, grounded %s, curtailed %s)\n",
		outcome.Verdict, summaryYesNo(outcome.Produced), summaryYesNo(outcome.Grounded), summaryYesNo(outcome.Curtailed))

	if len(outcome.Acted) == 0 {
		return
	}

	tools := slices.Sorted(maps.Keys(outcome.Acted))
	parts := make([]string, len(tools))
	for i, tool := range tools {
		name := tool
		if name == "" {
			name = summaryNoTool
		}
		parts[i] = fmt.Sprintf("%s %d", name, outcome.Acted[tool])
	}
	fmt.Fprintf(b, "  acted    %s\n", strings.Join(parts, ", "))
}

func summaryYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func renderSummaryAnswer(b *strings.Builder, record Record) {
	if record.Answer == "" {
		b.WriteString("  answer   EMPTY (0 B)\n")
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
	in, out, reports := 0, 0, 0
	perCall := make([]string, len(usage))
	for i, u := range usage {
		if u == nil {
			perCall[i] = "?"
			continue
		}
		in += u.InTokens
		out += u.OutTokens
		reports++
		perCall[i] = strconv.Itoa(u.OutTokens)
	}

	if reports == 0 {
		b.WriteString("  tokens   not recorded\n")
		return
	}

	fmt.Fprintf(b, "  tokens   %d in / %d out over %d calls  (out per call: %s)\n",
		in, out, reports, strings.Join(perCall, ", "))
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

func sumRenderedSizes(dispositions []Disposition) int {
	total := 0
	for _, d := range dispositions {
		total += d.RenderedSize
	}
	return total
}

func summaryForm(d Disposition) string {
	if d.Form != FormSubstance {
		return ""
	}
	return fmt.Sprintf(" %s of %s", FormSubstance, summaryBytes(d.Size))
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
