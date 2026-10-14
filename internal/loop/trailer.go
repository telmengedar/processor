package loop

import (
	"slices"
	"strings"
)

// TrailerKind names which closing paragraph a judgement call's user message carried.
type TrailerKind string

const (
	// TrailerNone is a call whose user message carried no closing paragraph.
	TrailerNone TrailerKind = "none"
	// TrailerClause is a call told to search when no part states what the request asks for.
	TrailerClause TrailerKind = "clause"
	// TrailerFound is a call told what its earlier search found, then given the clause.
	TrailerFound TrailerKind = "found"
	// TrailerBarren is a call told its earlier search found nothing new, with no clause.
	TrailerBarren TrailerKind = "barren"
)

const (
	recallClause = "When none of the parts above states what this request asks for, do not say the information is missing: call the recall tool with a short description of what is missing."
	searchedLine = "You searched memory in this request for "
	foundLine    = "; what it found is included above."
	barrenLine   = ", and it found nothing new. Answer now from what you have; where it does not say, say plainly that you do not know."
)

type recallSearch struct {
	query string
	yield int
}

func trailerFor(offered []string, searches []recallSearch) (string, TrailerKind) {
	if !slices.Contains(offered, ToolRecall) {
		return "", TrailerNone
	}
	if len(searches) == 0 {
		return recallClause, TrailerClause
	}

	last := searches[len(searches)-1]
	if last.yield > 0 {
		return searchedLine + quoted(last.query) + foundLine + "\n\n" + recallClause, TrailerFound
	}

	var barren []string
	for i := len(searches) - 1; i >= 0 && searches[i].yield == 0; i-- {
		barren = append(barren, quoted(searches[i].query))
	}
	slices.Reverse(barren)
	return searchedLine + strings.Join(barren, ", ") + barrenLine, TrailerBarren
}

func quoted(query string) string {
	return `"` + query + `"`
}
