package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// NoteDecision reports the private action's storage outcome independently of chat delivery.
type NoteDecision struct {
	State   string `json:"state"`
	Created int64  `json:"created,omitempty"`
}

const noteDecisionInstructions = `This is a private memory decision, not a chat reply. Return exactly one JSON object with exactly action, text, and about, and no other text. Choose action write with nonblank text and about null or a positive integer ID; choose action skip with text "" and about null. Do not emit native tool calls, tool syntax, Markdown fences, or storage claims. The caller's policy determines whether and what to remember; this contract defines output mechanics only.`

// NoteDecisionSchema returns the built-in private write-or-skip output schema.
func NoteDecisionSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["write","skip"]},"text":{"type":"string"},"about":{"type":["integer","null"]}},"required":["action","text","about"],"additionalProperties":false}`)
}

func noteDecisionMode(view *View) bool {
	return view != nil && view.Note != nil && view.Note.Mode == NoteModeDecision
}

func decodeNoteDecision(result JudgeResult) (JudgeResult, bool, error) {
	invalid := func() (JudgeResult, bool, error) {
		return JudgeResult{}, false, errors.New("invalid structured note decision")
	}
	if !utf8.ValidString(result.Answer) || result.UnofferedToolCalls != 0 || result.ToolSource != "" || wantsTool(result.Reason) || result.ToolError != "" || result.Reason == Truncated || result.RawReason == "length" {
		return invalid()
	}
	decoder := json.NewDecoder(strings.NewReader(result.Answer))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return invalid()
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return invalid()
		}
		key, ok := token.(string)
		if !ok || (key != "action" && key != "text" && key != "about") {
			return invalid()
		}
		if _, exists := fields[key]; exists {
			return invalid()
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return invalid()
		}
		fields[key] = bytes.TrimSpace(value)
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || len(fields) != 3 {
		return invalid()
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid()
	}
	var action, text string
	if !bytes.HasPrefix(fields["action"], []byte(`"`)) || json.Unmarshal(fields["action"], &action) != nil || !bytes.HasPrefix(fields["text"], []byte(`"`)) || json.Unmarshal(fields["text"], &text) != nil {
		return invalid()
	}
	var about int64
	if !bytes.Equal(fields["about"], []byte("null")) {
		if json.Unmarshal(fields["about"], &about) != nil || about <= 0 {
			return invalid()
		}
	}
	switch action {
	case "skip":
		if text != "" || about != 0 {
			return invalid()
		}
		return JudgeResult{}, false, nil
	case "write":
		if strings.TrimSpace(text) == "" {
			return invalid()
		}
		return JudgeResult{NoteText: text, NoteAbout: about}, true, nil
	default:
		return invalid()
	}
}

func (t *Turn) judgeNoteDecision(ctx context.Context, settings RunSettings, memory *workingMemory, input, conversation, identity string, subject int64, now time.Time, window UpdateWindow, scope noteScope, temperature *float64) (judgement, error) {
	if shortfall := t.judgementShortfall(ctx, settings); shortfall != "" {
		return judgement{}, fmt.Errorf("%w: %s", ErrRunTimeExhausted, shortfall)
	}
	system := noteDecisionInstructions
	if identity != "" {
		system = identity + paragraphBreak + system
	}
	result, err := t.Model.Judge(ctx, JudgeInput{System: system, Block: memory.render(), Input: input, Conversation: conversation, Now: now, Window: window, MaxOutputTokens: settings.JudgementBudget, Offered: []string{}, NoteDecisionOutput: true, Temperature: temperature})
	if err != nil {
		return judgement{}, fmt.Errorf("%w: %v", ErrModelUnavailable, err)
	}
	judged := judgement{modelCalls: 1, offered: [][]string{{}}, usages: []*Usage{result.Usage}, sampling: result.Sampling, provider: result.Provider, stop: StopReason{Reason: result.Reason, Raw: result.RawReason}, lastResponse: &LastResponse{ReasoningBytes: result.ReasoningBytes, UnofferedToolCalls: result.UnofferedToolCalls}, noteDecision: &NoteDecision{State: "rejected"}}
	decision, write, err := decodeNoteDecision(result)
	if err != nil {
		return judged, nil
	}
	if !write {
		judged.noteDecision.State = "abstained"
		return judged, nil
	}
	if !scope.enabled || t.Notes == nil {
		return judged, nil
	}
	if memory.exactAboutCopy(decision, scope, subject) {
		judged.toolCalls = toolCallRecords([]ToolExchange{{Tool: ToolNote, Text: decision.NoteText, About: decision.NoteAbout, Bytes: len(decision.NoteText), Dispositions: []Disposition{}, ToolSource: ToolSourceStructured, Error: "that exact note is already supplied for this subject: nothing was stored"}})
		return judged, nil
	}
	exchange := t.writeNote(ctx, decision, scope, subject, memory)
	exchange.ToolSource = ToolSourceStructured
	judged.toolCalls = toolCallRecords([]ToolExchange{exchange})
	switch exchange.Written {
	case Stored, Unlinked:
		judged.noteDecision.State = string(exchange.Written)
		judged.noteDecision.Created = exchange.Created
	case NotStored:
		judged.noteDecision.State = string(NotStored)
	}
	return judged, nil
}
