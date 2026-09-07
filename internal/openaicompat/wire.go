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

type writeFileToolArguments struct {
	Path    string `json:"path"`
	Content string `json:"content"`
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

func writeFileTool() wireTool {
	return wireTool{
		Type: "function",
		Function: wireFunction{
			Name:        writeFileToolName,
			Description: "Write a file into the working directory set aside for this request. Takes two arguments: path, a relative path naming the file, and content, the file's full text.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string"},
					"content": {"type": "string"}
				},
				"required": ["path", "content"]
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
		{Role: "user", Content: loop.RenderUserContent(in.Block, in.Input)},
	}

	for i, r := range in.PriorTools {
		callID := fmt.Sprintf("call-%d", i+1)

		messages = append(messages,
			wireMessage{
				Role: "assistant",
				ToolCalls: []wireToolCall{{
					ID:   callID,
					Type: "function",
					Function: wireFunctionCall{
						Name:      wireToolName(r.Tool),
						Arguments: toolArguments(r),
					},
				}},
			},
			wireMessage{
				Role:       "tool",
				ToolCallID: callID,
				Content:    renderToolResult(r),
			},
		)
	}

	return messages
}

func wireToolName(tool string) string {
	if tool == loop.ToolWriteFile {
		return writeFileToolName
	}
	return recallToolName
}

func toolArguments(r loop.ToolExchange) string {
	if r.Tool == loop.ToolWriteFile {
		encoded, _ := json.Marshal(writeFileToolArguments{Path: r.Path, Content: r.Content})
		return string(encoded)
	}
	encoded, _ := json.Marshal(recallToolArguments{Query: r.Query})
	return string(encoded)
}

func renderToolResult(r loop.ToolExchange) string {
	if r.Error != "" {
		return "error: " + r.Error
	}
	if r.Tool == loop.ToolWriteFile {
		return fmt.Sprintf("wrote %d bytes to %s", r.Bytes, r.Path)
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
		result.RawReason = choice.FinishReason
		result.ToolSource = loop.ToolSourceNative
		call := choice.Message.ToolCalls[0]

		switch call.Function.Name {
		case recallToolName:
			return translateRecall(result, call.Function.Arguments), nil
		case writeFileToolName:
			return translateWrite(result, call.Function.Arguments), nil
		}

		result.Reason = loop.Unrecognised
		return result, nil
	}

	result.RawReason = choice.FinishReason
	result.Reason = mapFinishReason(choice.FinishReason)
	return result, nil
}

func translateRecall(result loop.JudgeResult, arguments string) loop.JudgeResult {
	result.Reason = loop.WantsRecall

	var args recallToolArguments
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		result.ToolError = fmt.Sprintf("tool arguments could not be parsed: %v", err)
		return result
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		result.ToolError = "tool arguments had an empty query"
		return result
	}

	result.RecallQuery = query
	return result
}

func translateWrite(result loop.JudgeResult, arguments string) loop.JudgeResult {
	result.Reason = loop.WantsWrite

	var args writeFileToolArguments
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		result.ToolError = fmt.Sprintf("tool arguments could not be parsed: %v", err)
		return result
	}

	result.WritePath = args.Path
	result.WriteContent = args.Content
	return result
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
