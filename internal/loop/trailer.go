package loop

import (
	"slices"
	"strings"
)

// TrailerKind names the search state a judgement call was made in, which decides its search paragraph.
type TrailerKind string

const (
	// TrailerNone is a call not offered recall, so it carries no closing paragraph.
	TrailerNone TrailerKind = "none"
	// TrailerClause is a call told to search when no part states what the request asks for.
	TrailerClause TrailerKind = "clause"
	// TrailerFound is a call that follows a search which brought rows, and carries no closing paragraph.
	TrailerFound TrailerKind = "found"
	// TrailerBarren is a call told its earlier search found nothing new, with no clause.
	TrailerBarren TrailerKind = "barren"
)

// NoteAck names which acknowledgement of a note round a judgement call carried.
type NoteAck string

const (
	// NoteAckNone is a call that follows no note round, so it carries no acknowledgement.
	NoteAckNone NoteAck = "none"
	// NoteAckStored is a call told that its turn's note was stored.
	NoteAckStored NoteAck = "stored"
	// NoteAckNotStored is a call told that its turn's note was not stored, without a cause.
	NoteAckNotStored NoteAck = "notStored"
)

// NoteSentence is the last paragraph of every judgement call that offers the note tool; it names the tool as the adapters' wire does.
const NoteSentence = "When you are to note, remember or write something down, call the write_note tool with it instead of writing that you did."

const (
	recallClause = "When none of the parts above states what this request asks for, do not say the information is missing: call the recall tool with a short description of what is missing."
	searchedLine = "You searched memory in this request for "
	barrenLine   = ", and it found nothing new. Answer now from what you have; where it does not say, say plainly that you do not know."

	storedAckOpening    = `You stored this note in this request: "`
	storedAckClosing    = `". Now write your answer to the request.`
	notStoredAckLine    = "Your note in this request was not stored. Now write your answer to the request."
	closingParagraphGap = "\n\n"
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
		return "", TrailerFound
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

func acknowledgement(ack NoteAck, noteText string) string {
	switch ack {
	case NoteAckStored:
		return storedAckOpening + noteText + storedAckClosing
	case NoteAckNotStored:
		return notStoredAckLine
	}
	return ""
}

func noteSentenceFor(offered []string) string {
	if offersNote(offered) {
		return NoteSentence
	}
	return ""
}

func composeClosing(search, acknowledgement, sentence string) string {
	var parts []string
	for _, part := range []string{search, acknowledgement, sentence} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, closingParagraphGap)
}
