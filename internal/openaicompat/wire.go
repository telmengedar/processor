package openaicompat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

const errNoNodeID = "tool arguments named no node id to read"

type chatRequest struct {
	Model           string        `json:"model"`
	Messages        []wireMessage `json:"messages"`
	MaxTokens       int           `json:"max_tokens"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Tools           []wireTool    `json:"tools,omitempty"`
	Temperature     *float64      `json:"temperature,omitempty"`
	TopP            *float64      `json:"top_p,omitempty"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
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

type readNodeToolArguments struct {
	ID int64 `json:"id"`
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

func readNodeTool() wireTool {
	return wireTool{
		Type: "function",
		Function: wireFunction{
			Name:        readNodeToolName,
			Description: "Read one part of memory in full. Takes one argument: id, the integer id that part is printed with.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {"id": {"type": "integer"}},
				"required": ["id"]
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
	Reasoning string         `json:"reasoning"`
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
	return []wireMessage{
		{Role: "system", Content: in.System},
		{Role: "user", Content: loop.RenderUserContent(in.Block, in.Input, in.Now, in.Window)},
	}
}

type toolBinding struct {
	loopName string
	wireName string
	declare  func() wireTool
}

var toolTable = []toolBinding{
	{loop.ToolRecall, recallToolName, recallTool},
	{loop.ToolWriteFile, writeFileToolName, writeFileTool},
	{loop.ToolReadNode, readNodeToolName, readNodeTool},
}

func declaredTools(offered []string) []wireTool {
	var declared []wireTool
	for _, tool := range toolTable {
		if slices.Contains(offered, tool.loopName) {
			declared = append(declared, tool.declare())
		}
	}
	return declared
}

func offeredWireNames(offered []string) []string {
	var names []string
	for _, tool := range toolTable {
		if slices.Contains(offered, tool.loopName) {
			names = append(names, tool.wireName)
		}
	}
	return names
}

func isKnownWireTool(name string) bool {
	return slices.ContainsFunc(toolTable, func(tool toolBinding) bool { return tool.wireName == name })
}

func translate(wire chatResponse, offered []string) (loop.JudgeResult, error) {
	if len(wire.Choices) == 0 {
		return loop.JudgeResult{}, fmt.Errorf("response has no choices")
	}
	choice := wire.Choices[0]
	names := offeredWireNames(offered)

	result := loop.JudgeResult{
		RawReason:      choice.FinishReason,
		ReasoningBytes: len(choice.Message.Reasoning),
		Usage:          translateUsage(wire.Usage),
	}
	if choice.Message.Content != nil {
		result.Answer = *choice.Message.Content
	}
	for _, call := range choice.Message.ToolCalls {
		if !slices.Contains(names, call.Function.Name) {
			result.UnofferedToolCalls++
		}
	}

	if len(choice.Message.ToolCalls) > 0 {
		call := choice.Message.ToolCalls[0]

		if !slices.Contains(names, call.Function.Name) && (len(names) == 0 || isKnownWireTool(call.Function.Name)) {
			result.Reason = mapFinishReason(choice.FinishReason)
			return result, nil
		}

		result.ToolSource = loop.ToolSourceNative

		switch call.Function.Name {
		case recallToolName:
			return translateRecall(result, call.Function.Arguments), nil
		case writeFileToolName:
			return translateWrite(result, call.Function.Arguments), nil
		case readNodeToolName:
			return translateRead(result, call.Function.Arguments), nil
		}

		result.Reason = loop.Unrecognised
		return result, nil
	}

	result.RawReason = choice.FinishReason
	result.Reason = mapFinishReason(choice.FinishReason)
	return result, nil
}

func translateRead(result loop.JudgeResult, arguments string) loop.JudgeResult {
	result.Reason = loop.WantsRead

	var args readNodeToolArguments
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		result.ToolError = fmt.Sprintf("tool arguments could not be parsed: %v", err)
		return result
	}
	if args.ID <= 0 {
		result.ToolError = errNoNodeID
		return result
	}

	result.ReadNodeID = args.ID
	return result
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
