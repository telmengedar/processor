package loop

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// NotesPerTurn is how many note rounds a turn offers the note tool for.
const NotesPerTurn = 1

// MaxNoteEdges is how many links one note is filed with.
const MaxNoteEdges = 2

const (
	noteNameRunes    = 80
	noteSummaryRunes = 80
)

const (
	errNoteBlank      = "that note is blank: nothing was stored"
	errNoteNotUTF8    = "that note is not valid UTF-8: nothing was stored"
	errNoteRunName    = "that note's name would begin with the name prefix of a run record, which recall leaves out: nothing was stored"
	errNoteNotOffered = "the note tool was not offered on this call, so nothing was stored"
	errNoteNotStored  = "the note could not be stored"
)

// NoteRequest is one note to file: the node type, name and text, the root it is filed under, the subject it is linked to and, when non-zero, the one further node it is linked to.
type NoteRequest struct {
	Type    string
	Name    string
	Text    string
	Root    int64
	Subject int64
	About   int64
}

// NotePort is the seam between the loop and the graph for filing a note.
type NotePort interface {
	// WriteNote files the note and reports where it landed; the adapter alone orders the writes and decides what a partial failure leaves behind.
	WriteNote(ctx context.Context, request NoteRequest) WriteReceipt
}

type noteScope struct {
	nodeType string
	root     int64
	enabled  bool
}

func noteScopeOf(view *View) noteScope {
	if view == nil || view.Note == nil {
		return noteScope{}
	}
	return noteScope{nodeType: view.Note.Type, root: view.Root, enabled: true}
}

// NoteName derives a note's node name from its text: the first non-blank line with its whitespace collapsed, cut to a bounded number of runes.
func NoteName(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			return TruncateRunes(strings.Join(fields, " "), noteNameRunes)
		}
	}
	return ""
}

// TruncateRunes cuts s to at most limit runes and marks a cut with an ellipsis.
func TruncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

func refuseNote(text, name string) string {
	switch {
	case !utf8.ValidString(text):
		return errNoteNotUTF8
	case strings.TrimSpace(text) == "":
		return errNoteBlank
	case strings.HasPrefix(strings.ToLower(name), RunNamePrefix):
		return errNoteRunName
	}
	return ""
}

func (t *Turn) dispatchNote(ctx context.Context, result JudgeResult, scope noteScope, offered bool, subject int64, memory *workingMemory) ToolExchange {
	exchange := ToolExchange{Tool: ToolNote, Text: result.NoteText, About: result.NoteAbout, Bytes: len(result.NoteText), Dispositions: []Disposition{}}

	if result.ToolError != "" {
		exchange.Error = BoundCause(result.ToolError)
		return exchange
	}
	if !offered || !scope.enabled || t.Notes == nil {
		exchange.Error = errNoteNotOffered
		return exchange
	}

	name := NoteName(result.NoteText)
	if refusal := refuseNote(result.NoteText, name); refusal != "" {
		exchange.Error = refusal
		return exchange
	}

	about := result.NoteAbout
	switch {
	case about <= 0 || about == subject:
		about = 0
	case !memory.shows(about):
		about = 0
		exchange.AboutDropped = true
	}

	receipt := t.Notes.WriteNote(ctx, NoteRequest{
		Type:    scope.nodeType,
		Name:    name,
		Text:    result.NoteText,
		Root:    scope.root,
		Subject: subject,
		About:   about,
	})
	exchange.Written = receipt.State
	exchange.Created = receipt.NodeID
	if receipt.State == NotStored {
		exchange.Error = errNoteNotStored
		t.log().Warn("a note was not stored: the run goes on without it", "subject", subject)
		return exchange
	}

	t.log().Info("note written", "node", receipt.NodeID, "state", string(receipt.State), "bytes", len(result.NoteText))
	return exchange
}

func noteSummary(call ToolCallRecord) string {
	state := string(call.Written)
	if state == "" {
		state = "refused"
	}
	created := ""
	if call.Created != 0 {
		created = fmt.Sprintf("  #%d", call.Created)
	}
	return fmt.Sprintf("%s%s  %q", state, created, summaryTrunc(call.Text, noteSummaryRunes))
}

func offersNote(offered []string) bool {
	return slices.Contains(offered, ToolNote)
}
