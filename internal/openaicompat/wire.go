package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Tools       []wireTool    `json:"tools,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
}

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

type chatResponse struct {
	Choices []wireChoice `json:"choices"`
	Usage   *wireUsage   `json:"usage"`
}

type wireChoice struct {
	Message      wireResponseMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

type wireResponseMessage struct {
	Content   *string        `json:"content"`
	ToolCalls []wireToolCall `json:"tool_calls"`
}

type wireUsage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
}

type wireError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

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

func translate(wire chatResponse) (loop.JudgeResult, error) {
	if len(wire.Choices) == 0 {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: response has no choices")
	}
	choice := wire.Choices[0]

	result := loop.JudgeResult{
		Usage: translateUsage(wire.Usage),
	}
	if choice.Message.Content != nil {
		result.Answer = *choice.Message.Content
	}

	if len(choice.Message.ToolCalls) > 0 {
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

func translateUsage(u *wireUsage) *loop.Usage {
	if u == nil || u.PromptTokens == nil || u.CompletionTokens == nil {
		return nil
	}
	return &loop.Usage{
		InTokens:  *u.PromptTokens,
		OutTokens: *u.CompletionTokens,
	}
}
