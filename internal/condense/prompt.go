package condense

import (
	"regexp"
	"strings"
)

const basePrompt = `Condense the document below.

Your output is stored and later given to another model as its only source on this
document. No person reads it. It need not be readable; it must be complete and exact.

Remove everything that carries no content — narration, transitions, hedging,
restatement — then say what remains in the fewest words that keep it exact. Data that
survives, survives verbatim: numbers, dates, identifiers, paths, symbol names, hashes,
versions, quoted literals. Never round, generalise or re-word one.

Keep the force of every claim: its negations and prohibitions, its conditions and
exceptions, its quantifiers, and any ranking or precedence it sets between
alternatives. A rule stripped of its "not", its "only", or its ordering is a different
rule.

Follow the document's own order and grouping.

Any question the document can answer, your output must answer the same way — from
anywhere in it.
`

const outputClause = `
Your entire response is the condensed document, from its first line of content.
`

const supersessionClause = `
Where the document records that a claim replaced an earlier one, all three are
substance: what holds now, what it replaced, and why that matters.
`

const derivableDataClause = `
Where the text states a rule and then illustrates it with data that rule generates,
the rule is the substance and the instances are illustration. Data the text cannot
regenerate is substance itself.
`

const documentOpen = `
<document title="`

const documentMiddle = `">
`

const documentClose = `
</document>`

var (
	supersessionPattern  = regexp.MustCompile(`(?i)~~|correction|previously read|superseded`)
	derivableDataPattern = regexp.MustCompile(`(?m)^\|`)
)

// Prompt builds the condensation prompt for one node, appending each conditional clause the body's own shape calls for.
func Prompt(name, content string) string {
	var b strings.Builder
	b.WriteString(basePrompt)
	if supersessionPattern.MatchString(content) {
		b.WriteString(supersessionClause)
	}
	if derivableDataPattern.MatchString(content) {
		b.WriteString(derivableDataClause)
	}
	b.WriteString(outputClause)
	b.WriteString(documentOpen)
	b.WriteString(name)
	b.WriteString(documentMiddle)
	b.WriteString(content)
	b.WriteString(documentClose)
	return b.String()
}

// Clauses names the conditional clauses one body triggers, so a provenance row records which prompt produced its substance.
func Clauses(content string) []string {
	clauses := make([]string, 0, 2)
	if supersessionPattern.MatchString(content) {
		clauses = append(clauses, clauseSupersession)
	}
	if derivableDataPattern.MatchString(content) {
		clauses = append(clauses, clauseDerivableData)
	}
	return clauses
}

const (
	clauseSupersession  = "supersession"
	clauseDerivableData = "derivable-data"
)
