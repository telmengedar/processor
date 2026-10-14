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

// ThinKnowledgeSimilarity is the best admitted similarity a block must reach to escape the thin-knowledge hint; it signals and never cuts.
const ThinKnowledgeSimilarity = 0.63

// Thin reports whether none of the admitted similarities reaches ThinKnowledgeSimilarity; a block that admitted nothing is thin.
func Thin(similarities []float64) bool {
	return len(similarities) == 0 || slices.Max(similarities) < ThinKnowledgeSimilarity
}

const (
	nudgeNone = "Seems you know nothing about this topic - or maybe you are asking the wrong question; try looking at it from a different angle.\n"
	nudgeThin = "Seems like your knowledge is still thin on the topic - your focus might be too narrow; try approaching the question from a different angle.\n"
)

// Assemble is a pure function: no I/O, no clock, no randomness.
func Assemble(anchor Anchor, candidates []Candidate, budget int) (block string, dispositions []Disposition) {
	admitted, dispositions := admit(candidates, budget, len(anchor.Substance))

	sort.Slice(admitted, func(i, j int) bool { return admitted[i].ID < admitted[j].ID })

	return renderBlock(anchor, admitted, len(candidates) > 0, true), dispositions
}

func admit(candidates []Candidate, budget, spent int) (admitted []Candidate, dispositions []Disposition) {
	return admitUnheld(candidates, budget, spent, nil)
}

func admitUnheld(candidates []Candidate, budget, spent int, held func(Candidate) bool) (admitted []Candidate, dispositions []Disposition) {
	remaining := budget - spent
	if remaining < 0 {
		remaining = 0
	}

	dispositions = make([]Disposition, len(candidates))
	admitted = make([]Candidate, 0, len(candidates))

	cumulative := 0
	for i, c := range candidates {
		form, rendered := renderedPayload(c)
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

func renderedPayload(c Candidate) (Form, string) {
	return substanceForm(c.Substance)
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

func renderSection(b *strings.Builder, c Candidate) {
	fmt.Fprintf(b, "===== %s =====\nid: %d\ntype: %s\nname: %s\n", sectionCandidate, c.ID, c.Type, c.Name)
	if !c.Read {
		fmt.Fprintf(b, "similarity: %.3f\n", c.Similarity)
	}
	form, rendered := renderedPayload(c)
	writePayload(b, form, rendered)
}

func writePayload(b *strings.Builder, form Form, rendered string) {
	if form == FormSubstance {
		fmt.Fprintf(b, "\n%s\n", rendered)
		return
	}
	b.WriteString(unavailableLine)
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

func renderBlock(anchor Anchor, admitted []Candidate, consideredAny bool, nudges bool) string {
	var b strings.Builder

	b.WriteString("===== ANCHOR =====\n")
	fmt.Fprintf(&b, "id: %d\ntype: %s\nname: %s\n", anchor.ID, anchor.Type, anchor.Name)
	form, rendered := substanceForm(anchor.Substance)
	writePayload(&b, form, rendered)

	for _, c := range admitted {
		b.WriteString("\n")
		renderSection(&b, c)
	}

	if len(admitted) == 0 && consideredAny {
		b.WriteString("\nresults were found, but none were included.\n")
	}

	if !nudges {
		return b.String()
	}

	switch {
	case len(admitted) == 0 && consideredAny:
		b.WriteString(nudgeNone)
	case len(admitted) == 0:
		b.WriteString("\n")
		b.WriteString(nudgeNone)
	case Thin(rankedSimilarities(admitted)):
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

func candidateFromAnchor(anchor Anchor) Candidate {
	return Candidate{ID: anchor.ID, Type: anchor.Type, Name: anchor.Name, Content: anchor.Content, Substance: anchor.Substance, Read: true}
}

func rankedSimilarities(candidates []Candidate) []float64 {
	similarities := make([]float64, 0, len(candidates))
	for _, c := range candidates {
		if !c.Read {
			similarities = append(similarities, c.Similarity)
		}
	}
	return similarities
}

func summarizeAnchor(anchor Anchor) AnchorSummary {
	form, rendered := substanceForm(anchor.Substance)
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
