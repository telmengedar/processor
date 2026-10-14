package loop

import "strings"

// Verdict is the loop's closed set of accounts of what one run obtained.
type Verdict string

const (
	// VerdictCurtailed is a turn whose last call was reserved for answering, which the remaining-time guard stopped, or whose last call the endpoint stopped at length, rather than one that ran to its own end.
	VerdictCurtailed Verdict = "curtailed"
	// VerdictSilent is a run whose model chose silence on a run that offered it, and which no tool-dispatch bound curtailed.
	VerdictSilent Verdict = "silent"
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

	// Produced is true when the answer carries at least one non-whitespace character and the run did not choose silence.
	Produced bool `json:"produced"`

	// Silent is true when silence was offered and the model chose it.
	Silent bool `json:"silent,omitempty"`

	// Grounded is true when an admitted row reached the model, from the block or from a dispatched round.
	Grounded bool `json:"grounded"`

	// Curtailed is true when the turn's last call was reserved for answering, the remaining-time guard stopped the turn, or the endpoint stopped the last call at length, whatever the model then produced.
	Curtailed bool `json:"curtailed"`

	// Acted counts the tool rounds that completed without error, by tool; no verdict consumes it.
	Acted map[string]int `json:"acted,omitempty"`
}

// ComputeOutcome derives one run's outcome from that record alone, reading nothing the record does not carry.
func ComputeOutcome(record Record) Outcome {
	produced := producedText(record) && !record.Silent
	grounded := groundedInAdmittedRows(record)
	curtailed := record.ReservedCall.State != "" || record.CapReached || record.RecallClosed || record.TimeShortfall != "" || record.StopReason.Reason == Truncated

	return Outcome{
		Verdict:   verdictFor(produced, record.Silent, grounded, curtailed),
		Produced:  produced,
		Silent:    record.Silent,
		Grounded:  grounded,
		Curtailed: curtailed,
		Acted:     actedRounds(record),
	}
}

func verdictFor(produced, silent, grounded, curtailed bool) Verdict {
	switch {
	case curtailed:
		return VerdictCurtailed
	case silent:
		return VerdictSilent
	case !produced:
		return VerdictEmpty
	case !grounded:
		return VerdictUngrounded
	default:
		return VerdictDelivered
	}
}

// SilenceMarker is the text a model writes to decline a reply on a run that offered it.
const SilenceMarker = "NOREPLY"

func choseSilence(answer string) bool {
	return strings.Contains(strings.ToUpper(answer), strings.ToUpper(SilenceMarker))
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
