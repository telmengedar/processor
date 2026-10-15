package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

const cutReasonByteBudget = "byte budget exceeded"

// ThinKnowledgeSimilarity is the best admitted similarity a block must reach to escape being thin; the ledger reads it, and it never cuts.
const ThinKnowledgeSimilarity = 0.63

// Thin reports whether none of the admitted similarities reaches ThinKnowledgeSimilarity; a block that admitted nothing is thin.
func Thin(similarities []float64) bool {
	return len(similarities) == 0 || slices.Max(similarities) < ThinKnowledgeSimilarity
}

// Assemble is a pure function: no I/O, no clock, no randomness.
func Assemble(anchor Anchor, candidates []Candidate, budget int, form Form) (block string, dispositions []Disposition) {
	admitted, dispositions := admit(candidates, budget, anchorCharge(anchor, form), form)

	sort.Slice(admitted, func(i, j int) bool { return admitted[i].ID < admitted[j].ID })

	return renderBlock(anchor, admitted, len(candidates) > 0, form), dispositions
}

func admit(candidates []Candidate, budget, spent int, form Form) (admitted []Candidate, dispositions []Disposition) {
	return admitUnheld(candidates, budget, spent, nil, form)
}

func admitUnheld(candidates []Candidate, budget, spent int, held func(Candidate) bool, runForm Form) (admitted []Candidate, dispositions []Disposition) {
	remaining := budget - spent
	if remaining < 0 {
		remaining = 0
	}

	dispositions = make([]Disposition, len(candidates))
	admitted = make([]Candidate, 0, len(candidates))

	cumulative := 0
	for i, c := range candidates {
		form, rendered := renderedPayload(runForm, c)
		payload := len(rendered)

		d := Disposition{
			Rank:               i + 1,
			ID:                 c.ID,
			Type:               c.Type,
			Name:               c.Name,
			Similarity:         c.Similarity,
			Size:               len(c.Content),
			ContentHash:        contentHash(c.Content),
			Sources:            c.Sources,
			SubstanceAvailable: c.Substance != "" && !c.Created,
			Form:               form,
			RenderedSize:       payload,
		}
		if d.SubstanceAvailable {
			d.SubstanceSize = len(c.Substance)
		}

		switch {
		case held != nil && held(c):
			d.Included = true
			d.Held = true
			admitted = append(admitted, c)
		case cumulative+payload <= remaining:
			cumulative += payload
			d.Included = true
			admitted = append(admitted, c)
		default:
			d.CutReason = cutReasonByteBudget
		}

		dispositions[i] = d
	}

	return admitted, dispositions
}

func renderedPayload(runForm Form, c Candidate) (Form, string) {
	if runForm == FormContent {
		return FormContent, c.Content
	}
	return substanceForm(c.Substance)
}

func anchorCharge(anchor Anchor, runForm Form) int {
	_, rendered := renderedPayload(runForm, candidateFromAnchor(anchor))
	return len(rendered)
}

func substanceForm(substance string) (Form, string) {
	if substance == "" {
		return FormUnavailable, ""
	}
	return FormSubstance, substance
}

const (
	sectionCandidate = "CANDIDATE"
	unavailableLine  = "substance: unavailable\n"
)

func renderSection(b *strings.Builder, c Candidate, runForm Form) {
	fmt.Fprintf(b, "===== %s =====\nid: %d\ntype: %s\nname: %s\n", sectionCandidate, c.ID, c.Type, c.Name)
	if !c.Read {
		fmt.Fprintf(b, "similarity: %.3f\n", c.Similarity)
	}
	form, rendered := renderedPayload(runForm, c)
	writePayload(b, form, rendered)
}

func writePayload(b *strings.Builder, form Form, rendered string) {
	switch {
	case form == FormUnavailable:
		b.WriteString(unavailableLine)
	case rendered != "":
		fmt.Fprintf(b, "\n%s\n", rendered)
	}
}

// RenderUserContent composes the user message: the request, the instant it states and the window it was bounded to when one applied, the assembled block, the earlier conversation when there is one, the same request again, then the trailer when there is one.
func RenderUserContent(block, input string, now time.Time, window UpdateWindow, conversation, trailer string) string {
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
	if conversation != "" {
		b.WriteString("\n\n")
		b.WriteString(conversation)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(request)
	if trailer != "" {
		b.WriteString("\n\n")
		b.WriteString(trailer)
	}

	return b.String()
}

func windowLine(w UpdateWindow) string {
	const dateLayout = "2006-01-02"
	from := w.From.Format(dateLayout)
	to := w.To.AddDate(0, 0, -1).Format(dateLayout)
	return fmt.Sprintf("retrieval is limited to nodes updated %s … %s", from, to)
}

func renderBlock(anchor Anchor, admitted []Candidate, consideredAny bool, form Form) string {
	var b strings.Builder

	b.WriteString("===== ANCHOR =====\n")
	fmt.Fprintf(&b, "id: %d\ntype: %s\nname: %s\n", anchor.ID, anchor.Type, anchor.Name)
	anchorForm, rendered := renderedPayload(form, candidateFromAnchor(anchor))
	writePayload(&b, anchorForm, rendered)

	for _, c := range admitted {
		b.WriteString("\n")
		renderSection(&b, c, form)
	}

	if len(admitted) == 0 && consideredAny {
		b.WriteString("\nresults were found, but none were included.\n")
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

func candidateFromAnchor(anchor Anchor) Candidate {
	return Candidate{ID: anchor.ID, Type: anchor.Type, Name: anchor.Name, Content: anchor.Content, Substance: anchor.Substance, Read: true}
}

func summarizeAnchor(anchor Anchor, runForm Form) AnchorSummary {
	form, rendered := renderedPayload(runForm, candidateFromAnchor(anchor))
	return AnchorSummary{
		ID:           anchor.ID,
		Type:         anchor.Type,
		Name:         anchor.Name,
		Size:         len(anchor.Content),
		ContentHash:  contentHash(anchor.Content),
		Form:         form,
		RenderedSize: len(rendered),
	}
}
