package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	cutReasonByteBudget   = "byte budget exceeded"
	cutReasonSelfProduced = "self-produced"
	cutReasonBelowFloor   = "below relevance floor"
)

const thinKnowledgeThreshold = 5

const (
	nudgeNone = "Seems you know nothing about this topic - or maybe you are asking the wrong question; try looking at it from a different angle.\n"
	nudgeThin = "Seems like your knowledge is still thin on the topic - your focus might be too narrow; try approaching the question from a different angle.\n"
)

const nudgeNarrowQuery = "The graph has matches for this, but they did not fit the budget - narrow the query.\n"

const nudgeEscalate = "This does not appear to be in the graph - say so plainly and name what is missing, rather than repeating the same recall.\n"

// SubstanceRatio is a substance's byte length as a fraction of its content's.
type SubstanceRatio float64

// SubstanceRatioThreshold is the form rule's shipped dial, zero: no ratio is below it, so every candidate renders as content until a later change raises it.
const SubstanceRatioThreshold SubstanceRatio = 0

const formHeaderKey = "form"

// Assemble is a pure function: no I/O, no clock, no randomness.
func Assemble(anchor Anchor, candidates []Candidate, budget int, floor float64, threshold SubstanceRatio) (block string, dispositions []Disposition) {
	remaining := budget - len(anchor.Content)
	if remaining < 0 {
		remaining = 0
	}

	admitted, dispositions := admit(candidates, remaining, floor, threshold)

	sort.Slice(admitted, func(i, j int) bool { return admitted[i].ID < admitted[j].ID })

	return renderBlock(anchor, admitted, len(candidates) > 0, threshold), dispositions
}

func renderedForm(c Candidate, threshold SubstanceRatio) (Form, string) {
	if c.Substance == "" || c.Content == "" {
		return FormContent, c.Content
	}
	if SubstanceRatio(float64(len(c.Substance))/float64(len(c.Content))) < threshold {
		return FormSubstance, c.Substance
	}
	return FormContent, c.Content
}

func admit(candidates []Candidate, budget int, floor float64, threshold SubstanceRatio) (admitted []Candidate, dispositions []Disposition) {
	dispositions = make([]Disposition, len(candidates))
	admitted = make([]Candidate, 0, len(candidates))

	cumulative := 0
	for i, c := range candidates {
		form, rendered := renderedForm(c, threshold)

		d := Disposition{
			Rank:               i + 1,
			ID:                 c.ID,
			Type:               c.Type,
			Name:               c.Name,
			Similarity:         c.Similarity,
			Size:               len(c.Content),
			ContentHash:        contentHash(c.Content),
			Sources:            c.Sources,
			SubstanceAvailable: c.Substance != "",
			SubstanceSize:      len(c.Substance),
			Form:               form,
			RenderedSize:       len(rendered),
		}

		switch {
		case c.SelfProduced:
			d.CutReason = cutReasonSelfProduced
		case c.Similarity < floor:
			d.CutReason = cutReasonBelowFloor
		case cumulative+len(rendered) <= budget:
			cumulative += len(rendered)
			d.Included = true
			admitted = append(admitted, c)
		default:
			d.CutReason = cutReasonByteBudget
		}

		dispositions[i] = d
	}

	return admitted, dispositions
}

// RenderUserContent composes the user message: the request, the instant it states and the window it was bounded to when one applied, the assembled block, then the same request again.
func RenderUserContent(block, input string, now time.Time, window UpdateWindow) string {
	request := "===== INPUT =====\n" + input

	var b strings.Builder

	b.WriteString(request)
	if !now.IsZero() {
		b.WriteString("\n\n===== NOW =====\n")
		b.WriteString(now.Format(time.RFC3339))
		if !window.IsZero() {
			b.WriteString("\n")
			b.WriteString(windowLine(window))
		}
	}
	b.WriteString("\n\n\n")
	b.WriteString(block)
	b.WriteString("\n")
	b.WriteString(request)

	return b.String()
}

func windowLine(w UpdateWindow) string {
	const dateLayout = "2006-01-02"
	from := w.From.Format(dateLayout)
	to := w.To.AddDate(0, 0, -1).Format(dateLayout)
	return fmt.Sprintf("retrieval is limited to nodes updated %s … %s", from, to)
}

// RenderToolResult renders one completed tool round as the text the model is shown for it.
func RenderToolResult(r ToolExchange) string {
	if r.Error != "" {
		return "error: " + r.Error
	}
	if r.Tool == ToolWriteFile {
		return fmt.Sprintf("wrote %d bytes to %s", r.Bytes, r.Path)
	}
	if len(r.Results) == 0 {
		base := "no additional results found."
		if len(r.Dispositions) > 0 {
			base = "results were found, but none were included."
		}
		if r.Tool != ToolRecall {
			return base
		}
		if anyCutForByteBudget(r.Dispositions) {
			return base + "\n" + nudgeNarrowQuery
		}
		return base + "\n" + nudgeEscalate
	}

	var b strings.Builder
	for i, c := range r.Results {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "===== RESULT =====\nid: %d\ntype: %s\nname: %s\n\n%s\n", c.ID, c.Type, c.Name, c.Content)
	}
	return b.String()
}

func anyCutForByteBudget(dispositions []Disposition) bool {
	for _, d := range dispositions {
		if d.CutReason == cutReasonByteBudget {
			return true
		}
	}
	return false
}

// renderBlock renders the fixed layout of design §6.3: the anchor first
// (the run's stable subject), then the admitted candidates ascending by
// id (the volatile part).
func renderBlock(anchor Anchor, admitted []Candidate, consideredAny bool, threshold SubstanceRatio) string {
	var b strings.Builder

	b.WriteString("===== ANCHOR =====\n")
	fmt.Fprintf(&b, "id: %d\ntype: %s\nname: %s\n\n", anchor.ID, anchor.Type, anchor.Name)
	b.WriteString(anchor.Content)
	b.WriteString("\n")

	for _, c := range admitted {
		form, rendered := renderedForm(c, threshold)

		b.WriteString("\n===== CANDIDATE =====\n")
		fmt.Fprintf(&b, "id: %d\ntype: %s\nname: %s\n", c.ID, c.Type, c.Name)
		if form == FormSubstance {
			fmt.Fprintf(&b, "%s: %s\n", formHeaderKey, form)
		}
		b.WriteString("\n")
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	if len(admitted) == 0 && consideredAny {
		b.WriteString("\nresults were found, but none were included.\n")
	}

	switch {
	case len(admitted) == 0 && consideredAny:
		b.WriteString(nudgeNone)
	case len(admitted) == 0:
		b.WriteString("\n")
		b.WriteString(nudgeNone)
	case len(admitted) < thinKnowledgeThreshold:
		b.WriteString("\n")
		b.WriteString(nudgeThin)
	}

	return b.String()
}

// contentHash is the only field in the record whose value accrues later,
// deliberately (design §7): a record of ids alone rots as the nodes
// change, and without the hash the record looks precise while quietly
// lying.
func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func summarizeAnchor(anchor Anchor) AnchorSummary {
	return AnchorSummary{
		ID:          anchor.ID,
		Type:        anchor.Type,
		Name:        anchor.Name,
		Size:        len(anchor.Content),
		ContentHash: contentHash(anchor.Content),
	}
}
