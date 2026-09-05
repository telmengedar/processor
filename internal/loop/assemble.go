package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
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
			Rank:        i + 1,
			ID:          c.ID,
			Type:        c.Type,
			Name:        c.Name,
			Similarity:  c.Similarity,
			Size:        size,
			ContentHash: contentHash(c.Content),
			Sources:     c.Sources,
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
