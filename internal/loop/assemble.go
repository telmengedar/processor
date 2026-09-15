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
)

// Assemble is a pure function: no I/O, no clock, no randomness.
func Assemble(anchor Anchor, candidates []Candidate, budget int) (block string, dispositions []Disposition) {
	remaining := budget - len(anchor.Content)
	if remaining < 0 {
		remaining = 0
	}

	admitted, dispositions := admit(candidates, remaining)

	sort.Slice(admitted, func(i, j int) bool { return admitted[i].ID < admitted[j].ID })

	return renderBlock(anchor, admitted), dispositions
}

func admit(candidates []Candidate, budget int) (admitted []Candidate, dispositions []Disposition) {
	dispositions = make([]Disposition, len(candidates))
	admitted = make([]Candidate, 0, len(candidates))

	cumulative := 0
	for i, c := range candidates {
		size := len(c.Content)

		d := Disposition{
			Rank:               i + 1,
			ID:                 c.ID,
			Type:               c.Type,
			Name:               c.Name,
			Similarity:         c.Similarity,
			Size:               size,
			ContentHash:        contentHash(c.Content),
			Sources:            c.Sources,
			SubstanceAvailable: c.Substance != "",
			SubstanceSize:      len(c.Substance),
		}

		switch {
		case c.SelfProduced:
			d.CutReason = cutReasonSelfProduced
		case cumulative+size <= budget:
			cumulative += size
			d.Included = true
			admitted = append(admitted, c)
		default:
			d.CutReason = cutReasonByteBudget
		}

		dispositions[i] = d
	}

	return admitted, dispositions
}

// RenderUserContent composes the user message: the request, the instant it states, the assembled block, then the same request again.
func RenderUserContent(block, input string, now time.Time) string {
	request := "===== INPUT =====\n" + input

	var b strings.Builder

	b.WriteString(request)
	if !now.IsZero() {
		b.WriteString("\n\n===== NOW =====\n")
		b.WriteString(now.UTC().Format(time.RFC3339))
	}
	b.WriteString("\n\n\n")
	b.WriteString(block)
	b.WriteString("\n")
	b.WriteString(request)

	return b.String()
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
		if len(r.Dispositions) > 0 {
			return "results were found, but none were included."
		}
		return "no additional results found."
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

// renderBlock renders the fixed layout of design §6.3: the anchor first
// (the run's stable subject), then the admitted candidates ascending by
// id (the volatile part).
func renderBlock(anchor Anchor, admitted []Candidate) string {
	var b strings.Builder

	b.WriteString("===== ANCHOR =====\n")
	fmt.Fprintf(&b, "id: %d\ntype: %s\nname: %s\n\n", anchor.ID, anchor.Type, anchor.Name)
	b.WriteString(anchor.Content)
	b.WriteString("\n")

	for _, c := range admitted {
		b.WriteString("\n===== CANDIDATE =====\n")
		fmt.Fprintf(&b, "id: %d\ntype: %s\nname: %s\n\n", c.ID, c.Type, c.Name)
		b.WriteString(c.Content)
		b.WriteString("\n")
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
