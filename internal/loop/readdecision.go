package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

// StructuredReadActions projects native retrieval capabilities into the structured action domain.
func StructuredReadActions(offered []string) []string {
	actions := []string{"answer"}
	if slices.Contains(offered, ToolRecall) {
		actions = append(actions, "recall")
	}
	if slices.Contains(offered, ToolReadNode) {
		actions = append(actions, "read")
	}
	return actions
}

const structuredRecallClause = "When none of the parts above states what this request asks for, do not say the information is missing: choose the recall action with a short description of what is missing."

// StructuredReadInstructions states the built-in output mechanics for the actions allowed on a call.
func StructuredReadInstructions(allowed []string) string {
	names := strings.Join(allowed, " and ")
	if len(allowed) > 2 {
		names = strings.Join(allowed[:len(allowed)-1], ", ") + ", and " + allowed[len(allowed)-1]
	}
	return `Return exactly one JSON object and nothing else, with exactly action, text, query, and id. The allowed actions on this call are ` + names + `. For answer, text is the actual nonblank reply for the person (or NOREPLY when silence is chosen), and query and id are null. For recall, text is "", query is a nonblank short description of missing information, and id is null. For read, text is "", query is null, and id is the positive integer id of the part to read. An answer is a reply, not a tool dispatch; select read or recall to request retrieval. Do not emit native tool calls, tool syntax, Markdown fences, extra fields or commentary outside this object. This output contract replaces response-format mechanics only; the conversation and memory-use instructions still apply.`
}

// StructuredReadSchema returns the built-in schema restricted to the actions permitted on this call.
func StructuredReadSchema(allowed []string) json.RawMessage {
	schema := struct {
		Type                 string         `json:"type"`
		Properties           map[string]any `json:"properties"`
		Required             []string       `json:"required"`
		AdditionalProperties bool           `json:"additionalProperties"`
	}{Type: "object", Properties: map[string]any{
		"action": map[string]any{"type": "string", "enum": allowed},
		"text":   map[string]any{"type": "string"},
		"query":  map[string]any{"type": []string{"string", "null"}},
		"id":     map[string]any{"type": []string{"integer", "null"}},
	}, Required: []string{"action", "text", "query", "id"}}
	raw, _ := json.Marshal(schema)
	return raw
}

func decodeReadDecision(result JudgeResult, allowed []string) (JudgeResult, error) {
	invalid := func() (JudgeResult, error) { return JudgeResult{}, errors.New("invalid structured read response") }
	if result.UnofferedToolCalls != 0 || result.ToolSource != "" || !utf8.ValidString(result.Answer) {
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
		if !ok || !slices.Contains([]string{"action", "text", "query", "id"}, key) {
			return invalid()
		}
		if _, present := fields[key]; present {
			return invalid()
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return invalid()
		}
		fields[key] = bytes.TrimSpace(value)
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || len(fields) != 4 {
		return invalid()
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid()
	}
	var action, text string
	if !bytes.HasPrefix(fields["action"], []byte(`"`)) || json.Unmarshal(fields["action"], &action) != nil || !slices.Contains(allowed, action) {
		return invalid()
	}
	if !bytes.HasPrefix(fields["text"], []byte(`"`)) || json.Unmarshal(fields["text"], &text) != nil {
		return invalid()
	}
	result.Answer = ""
	result.RecallQuery = ""
	result.ReadNodeID = 0
	result.ToolSource = ""
	switch action {
	case "answer":
		if strings.TrimSpace(text) == "" || !bytes.Equal(fields["query"], []byte("null")) || !bytes.Equal(fields["id"], []byte("null")) {
			return invalid()
		}
		result.Reason = Answered
		result.Answer = text
	case "read":
		var id int64
		if text != "" || !bytes.Equal(fields["query"], []byte("null")) || json.Unmarshal(fields["id"], &id) != nil || id <= 0 {
			return invalid()
		}
		result.Reason = WantsRead
		result.ReadNodeID = id
		result.ToolSource = ToolSourceStructured
	case "recall":
		var query string
		if text != "" || !bytes.Equal(fields["id"], []byte("null")) || !bytes.HasPrefix(fields["query"], []byte(`"`)) || json.Unmarshal(fields["query"], &query) != nil || !IsPhrase(query) {
			return invalid()
		}
		result.Reason = WantsRecall
		result.RecallQuery = query
		result.ToolSource = ToolSourceStructured
	default:
		return invalid()
	}
	return result, nil
}

func validateResponseMode(view *View) error {
	if view != nil && view.Note != nil && view.Note.Mode != "" && view.Note.Mode != NoteModeDecision {
		return fmt.Errorf("%w: unsupported note mode", ErrInvalidConfig)
	}
	if view == nil || view.ResponseMode == "" {
		return nil
	}
	if view.ResponseMode != ResponseStructuredRead || view.Note != nil {
		return fmt.Errorf("%w: structuredRead requires a supported responseMode and no note capability", ErrInvalidConfig)
	}
	return nil
}

func structuredReadTools(offered []string, recallClosed bool) []string {
	tools := []string{}
	if slices.Contains(offered, ToolRecall) && !recallClosed {
		tools = append(tools, ToolRecall)
	}
	if slices.Contains(offered, ToolReadNode) {
		tools = append(tools, ToolReadNode)
	}
	return tools
}

func (t *Turn) structuredReadCharge(conversation, silence bool, form Form) int {
	charge := 0
	for _, reserved := range []bool{false, true} {
		native := t.offeredTools(reserved, false)
		for _, closed := range []bool{false, true} {
			logical := structuredReadTools(native, closed)
			extra := max(0, len(t.composeSystem("", logical, conversation, silence, form, ResponseStructuredRead))-len(t.composeSystem("", native, conversation, silence, form, "")))
			extra += len(StructuredReadSchema(StructuredReadActions(logical)))
			if slices.Contains(logical, ToolRecall) {
				extra += max(0, len(structuredRecallClause)-len(recallClause))
			}
			charge = max(charge, extra)
		}
	}
	return charge
}
