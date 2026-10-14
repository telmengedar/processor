package loop

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Verdict is the loop's closed set of accounts of what one run obtained.
type Verdict string

const (
	// VerdictCurtailed is a turn whose last call was reserved for answering, which the remaining-time guard stopped, or whose last call the endpoint stopped at length, rather than one that ran to its own end.
	VerdictCurtailed Verdict = "curtailed"
	// VerdictSilent is a run whose model chose silence on a run that offered it, and which no tool-dispatch bound curtailed.
	VerdictSilent Verdict = "silent"
	// VerdictWithheld is a run whose final answer was not a reply: tool syntax or processor's own framing written as speech.
	VerdictWithheld Verdict = "withheld"
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

	// Produced is true when the answer carries at least one non-whitespace character, is a reply, and the run did not choose silence.
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
	produced := producedText(record) && !record.Silent && record.NonReply == ""
	grounded := groundedInAdmittedRows(record)
	curtailed := record.ReservedCall.State != "" || record.CapReached || record.RecallClosed || record.TimeShortfall != "" || record.StopReason.Reason == Truncated

	return Outcome{
		Verdict:   verdictFor(produced, record.Silent, record.NonReply != "", grounded, curtailed),
		Produced:  produced,
		Silent:    record.Silent,
		Grounded:  grounded,
		Curtailed: curtailed,
		Acted:     actedRounds(record),
	}
}

func verdictFor(produced, silent, withheld, grounded, curtailed bool) Verdict {
	switch {
	case curtailed:
		return VerdictCurtailed
	case silent:
		return VerdictSilent
	case withheld:
		return VerdictWithheld
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

// NonReply is the closed set of reasons a turn's final answer is not a reply.
type NonReply string

const (
	// NonReplyToolRequest is a final answer that came from a call which asked for a tool.
	NonReplyToolRequest NonReply = "toolRequest"
	// NonReplyToolText is a final answer that opens with a tool call the adapter did not honour.
	NonReplyToolText NonReply = "toolText"
	// NonReplyBlockText is a final answer that opens with processor's own block or history syntax.
	NonReplyBlockText NonReply = "blockText"
	// NonReplyPartReference is a final answer that is only the id of a part shown in its call, on a call that offered no read to make of it.
	NonReplyPartReference NonReply = "partReference"
	// NonReplyQueryEcho is a final answer with the shape of a query rather than a reply, in a turn that already recovered such an answer as a recall.
	NonReplyQueryEcho NonReply = "queryEcho"
)

const typePrefix = "type: "

var (
	frameLine = regexp.MustCompile(`^===== .+ =====$`)
	idLine    = regexp.MustCompile(`^id: [0-9]+$`)

	partReferenceAnswer = regexp.MustCompile(`^(?i:id:[ \t]*)?([0-9]+)$`)
)

func classifyNonReply(answer string, lastWasToolRequest bool, unhonoured string, shown func(int64) bool, echoed func(string) bool) NonReply {
	if strings.TrimSpace(answer) == "" {
		return ""
	}
	switch {
	case lastWasToolRequest:
		return NonReplyToolRequest
	case unhonoured != "":
		return NonReplyToolText
	case opensWithProcessorSyntax(answer):
		return NonReplyBlockText
	case isPartReference(answer, shown):
		return NonReplyPartReference
	case echoed(answer):
		return NonReplyQueryEcho
	default:
		return ""
	}
}

func isPartReference(answer string, shown func(int64) bool) bool {
	id, ok := partReferenceOf(answer)
	return ok && shown(id)
}

func partReferenceOf(answer string) (int64, bool) {
	match := partReferenceAnswer.FindStringSubmatch(strings.TrimSpace(answer))
	if match == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(match[1], 10, 64)
	return id, err == nil && id > 0
}

func opensWithProcessorSyntax(answer string) bool {
	opening := strings.TrimLeftFunc(answer, unicode.IsSpace)
	first, rest, _ := strings.Cut(opening, "\n")
	first = strings.TrimRight(first, "\r")

	if frameLine.MatchString(first) {
		return true
	}
	if hasPrefixFold(opening, LabelUserSaid) || hasPrefixFold(opening, LabelYouReplied) {
		return true
	}
	return idLine.MatchString(first) && strings.HasPrefix(rest, typePrefix)
}

func hasPrefixFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}
