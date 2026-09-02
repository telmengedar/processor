package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

// This file is the whole of the protocol's wire shape (design §6.6's
// D1-D6, the subset M1 depends on) and the translation into and out of
// the loop's own vocabulary (design §9.1 stage 3c, §9.2). Nothing declared
// here is exported past Client.Judge — see the package doc's rule.

// --- request ---

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []wireMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
	Tools     []wireTool    `json:"tools,omitempty"`
}

// wireMessage covers every role D3 depends on: system, user, assistant,
// tool. Content is a plain string on the way out — M1 never sends a null
// content field, only decodes one on the way in (design §6.6's second
// divergence).
type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function wireFunctionCall `json:"function"`
}

type wireFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// recallToolArguments is the one tool's argument shape (design §2.4): a
// single query string, the same operation Assemble already performs.
type recallToolArguments struct {
	Query string `json:"query"`
}

func recallTool() wireTool {
	return wireTool{
		Type: "function",
		Function: wireFunction{
			Name:        recallToolName,
			Description: "Search memory for something the assembled context did not include. Takes one argument: query, a short description of what is missing.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {"query": {"type": "string"}},
				"required": ["query"]
			}`),
		},
	}
}

// --- response ---

type chatResponse struct {
	Choices []wireChoice `json:"choices"`
	Usage   *wireUsage   `json:"usage"`
}

type wireChoice struct {
	Message      wireResponseMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

// wireResponseMessage's Content is a pointer because D-list's second
// divergence is real: text content may be null when a tool call is
// present, and that must decode as absent, not as a required field.
type wireResponseMessage struct {
	Content   *string        `json:"content"`
	ToolCalls []wireToolCall `json:"tool_calls"`
}

// wireUsage's two counts are pointers so the decoder can tell "the field
// was absent" from "the field was present and zero" (design §8.3, revision
// 3 from #10821 W-4). total_tokens is deliberately not decoded at all: it
// is the sum of the other two on every endpoint that reports it, so it
// carries nothing translateUsage cannot compute, and carrying it forced a
// fabricated total on any endpoint that reports two counts and no total.
type wireUsage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
}

// wireError is the shape C41 measured on an error response: an object
// under "error" with a "message" field, among others M1 does not read.
type wireError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// --- request construction ---

// buildMessages reconstructs the whole conversation from scratch on every
// call (design §9.1 stage 3a: M1 sends one fixed request shape rather
// than adapting to the endpoint, which is also what keeps the request
// deterministic). Each completed recall round becomes an assistant
// message carrying one tool call, followed by the matching tool-result
// message — synthesized locally with a sequential id, never a value the
// endpoint issued on a prior call, because the reference protocol is
// itself stateless per request: nothing here depends on an id the
// endpoint chose.
func buildMessages(in loop.JudgeInput) []wireMessage {
	messages := []wireMessage{
		{Role: "system", Content: in.System},
		{Role: "user", Content: buildUserContent(in.Block, in.Input)},
	}

	for i, r := range in.PriorRecalls {
		callID := fmt.Sprintf("recall-%d", i+1)

		args := recallToolArguments{Query: r.Query}
		argsJSON, _ := json.Marshal(args)

		messages = append(messages,
			wireMessage{
				Role: "assistant",
				ToolCalls: []wireToolCall{{
					ID:   callID,
					Type: "function",
					Function: wireFunctionCall{
						Name:      recallToolName,
						Arguments: string(argsJSON),
					},
				}},
			},
			wireMessage{
				Role:       "tool",
				ToolCallID: callID,
				Content:    renderRecallResult(r),
			},
		)
	}

	return messages
}

func buildUserContent(block, input string) string {
	var b strings.Builder
	b.WriteString(block)
	b.WriteString("\n===== INPUT =====\n")
	b.WriteString(input)
	return b.String()
}

// renderRecallResult renders one completed recall round's outcome as the
// tool-result message content the model sees — the same anchor/candidate
// framing Assemble already uses for legibility, or the error, never both.
func renderRecallResult(r loop.RecallExchange) string {
	if r.Error != "" {
		return "error: " + r.Error
	}
	if len(r.Results) == 0 {
		return "no additional results found."
	}

	var b strings.Builder
	for i, c := range r.Results {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "===== RESULT =====\nid: %d\ntype: %s\nname: %s\n\n%s\n", c.ID, c.Type, c.Name, c.Content)
	}
	return b.String()
}

// --- response translation ---

// translate turns one wire response into the loop's vocabulary (design
// §9.1 stage 3c, §9.2). The presence of a tool call in the message is
// what decides WantsRecall — not the finish_reason string — because
// design §6.6's D5 depends only on a terminal-reason field existing, not
// on its values, and a tool call's presence is the one signal every
// implementation of D6 must agree on.
func translate(wire chatResponse) (loop.JudgeResult, error) {
	if len(wire.Choices) == 0 {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: response has no choices")
	}
	// M1 reads the first choice and no other (design D4).
	choice := wire.Choices[0]

	result := loop.JudgeResult{
		Usage: translateUsage(wire.Usage),
	}
	if choice.Message.Content != nil {
		result.Answer = *choice.Message.Content
	}

	if len(choice.Message.ToolCalls) > 0 {
		// M1 ships one tool (design §2.4): only the first requested call is
		// acted on if an endpoint ever returns more than one.
		result.Reason = loop.WantsRecall
		result.RawReason = choice.FinishReason

		var args recallToolArguments
		raw := choice.Message.ToolCalls[0].Function.Arguments
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			result.RecallError = fmt.Sprintf("tool arguments could not be parsed: %v", err)
			return result, nil
		}
		query := strings.TrimSpace(args.Query)
		if query == "" {
			result.RecallError = "tool arguments had an empty query"
			return result, nil
		}
		result.RecallQuery = query
		return result, nil
	}

	result.RawReason = choice.FinishReason
	result.Reason = mapFinishReason(choice.FinishReason)
	return result, nil
}

// mapFinishReason maps a finish reason to the loop's closed set (design
// §8.3) once a tool call is known not to be present. Anything the
// reference implementation and local runtimes are not both known to send
// falls into Unrecognised, with the raw value preserved in the record
// rather than the loop guessing at an endpoint's private vocabulary.
func mapFinishReason(reason string) loop.TerminalReason {
	switch reason {
	case "stop":
		return loop.Answered
	case "length":
		return loop.Truncated
	case "content_filter":
		return loop.Refused
	default:
		return loop.Unrecognised
	}
}

// translateUsage maps the wire usage object into the loop's in/out
// vocabulary (design §8.3, revision 3 from #10821 W-4). Absence stays at
// the object level: a usage object reporting only one of the two counts
// has not been observed on any runtime, so — until one is — it is recorded
// absent in whole rather than half zero-filled, exactly like a missing
// usage object altogether.
func translateUsage(u *wireUsage) *loop.Usage {
	if u == nil || u.PromptTokens == nil || u.CompletionTokens == nil {
		return nil
	}
	return &loop.Usage{
		InTokens:  *u.PromptTokens,
		OutTokens: *u.CompletionTokens,
	}
}
