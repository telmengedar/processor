package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const cutReasonByteBudget = "byte budget exceeded"

// Assemble is a pure function: no I/O, no clock, no randomness. Given the
// anchor and the candidates recall returned (in rank/score order,
// descending), it decides which candidates fit within budget bytes of body
// content and renders the block sent to the model.
//
// Admission is a stop, not a skip (design §6.3): the first candidate whose
// body would push the cumulative admitted size over budget is cut, and so
// is every candidate after it. Smaller, lower-ranked candidates are never
// back-filled into space a larger one left unused — that would break the
// invariant that "included" means "outranked everything excluded". The
// anchor itself is exempt from the budget (design R4).
//
// The block is rendered sorted by node id ascending — never by score
// (design §6.3): ranking decides entry, a total order decides position.
// Id order is a total order over distinct nodes; score order is not, so a
// block sorted by score would reshuffle (and a byte-exact golden test
// would go flaky) the moment two scores tie or one shifts in the ninth
// decimal, for a reason that has nothing to do with the code.
func Assemble(anchor Anchor, candidates []Candidate, budget int) (block string, dispositions []Disposition) {
	admitted, dispositions := admit(candidates, budget)

	sort.Slice(admitted, func(i, j int) bool { return admitted[i].ID < admitted[j].ID })

	return renderBlock(anchor, admitted), dispositions
}

func admit(candidates []Candidate, budget int) (admitted []Candidate, dispositions []Disposition) {
	dispositions = make([]Disposition, len(candidates))
	admitted = make([]Candidate, 0, len(candidates))

	cumulative := 0
	stopped := false
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
		}

		if !stopped && cumulative+size <= budget {
			cumulative += size
			d.Included = true
			admitted = append(admitted, c)
		} else {
			stopped = true
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
