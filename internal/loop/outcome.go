package loop

import "strings"

// Verdict is the loop's closed set of accounts of what one run obtained.
type Verdict string

const (
	// VerdictCurtailed is a tool-dispatch bound that ended the turn while the model still wanted a tool.
	VerdictCurtailed Verdict = "curtailed"
	// VerdictEmpty is a run no tool-dispatch bound stopped that nonetheless produced no text.
	VerdictEmpty Verdict = "empty"
	// VerdictUngrounded is a produced answer that no admitted row fed.
	VerdictUngrounded Verdict = "ungrounded"
	// VerdictDelivered is a produced answer fed by at least one admitted row, under no tool-dispatch bound.
	VerdictDelivered Verdict = "delivered"
)

// Outcome is the loop's own account of one run, derived from the record's own fields and never from the answer's prose.
type Outcome struct {
	Verdict Verdict `json:"verdict"`

	// Produced is true when the answer carries at least one non-whitespace character.
	Produced bool `json:"produced"`

	// Grounded is true when an admitted row reached the model, from the block or from a dispatched round.
	Grounded bool `json:"grounded"`

	// Curtailed is true when a tool-dispatch bound ended the turn while the model still wanted a tool.
	Curtailed bool `json:"curtailed"`

	// Acted counts the tool rounds that completed without error, by tool; no verdict consumes it.
	Acted map[string]int `json:"acted,omitempty"`
}

// ComputeOutcome derives one run's outcome from that record alone, reading nothing the record does not carry.
func ComputeOutcome(record Record) Outcome {
	produced := producedText(record)
	grounded := groundedInAdmittedRows(record)
	curtailed := record.CapReached || record.RecallClosed

	return Outcome{
		Verdict:   verdictFor(produced, grounded, curtailed),
		Produced:  produced,
		Grounded:  grounded,
		Curtailed: curtailed,
		Acted:     actedRounds(record),
	}
}

func verdictFor(produced, grounded, curtailed bool) Verdict {
	switch {
	case curtailed:
		return VerdictCurtailed
	case !produced:
		return VerdictEmpty
	case !grounded:
		return VerdictUngrounded
	default:
		return VerdictDelivered
	}
}

func producedText(record Record) bool {
	return strings.TrimSpace(record.Answer) != ""
}

func groundedInAdmittedRows(record Record) bool {
	for _, d := range record.Candidates {
		if d.Included {
			return true
		}
	}
	for _, call := range record.ToolCalls {
		if call.Error != "" {
			continue
		}
		for _, d := range call.Results {
			if d.Included {
				return true
			}
		}
	}
	return false
}

func actedRounds(record Record) map[string]int {
	var counts map[string]int
	for _, call := range record.ToolCalls {
		if call.Error != "" {
			continue
		}
		if counts == nil {
			counts = make(map[string]int, len(record.ToolCalls))
		}
		counts[call.Tool]++
	}
	return counts
}
