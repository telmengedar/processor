package ollama

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	errNoNodeID               = "tool arguments named no node id to read"
	errRecoveredCallWithoutID = "the call in the response text was missing its id argument"
)

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Think    bool          `json:"think"`
	Tools    []wireTool    `json:"tools,omitempty"`
	Options  wireOptions   `json:"options"`
}

type wireOptions struct {
	NumPredict  int      `json:"num_predict"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
}

type wireToolCall struct {
	Function wireFunctionCall `json:"function"`
}

type wireFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
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
	Message         wireResponseMessage `json:"message"`
	DoneReason      string              `json:"done_reason"`
	PromptEvalCount *int                `json:"prompt_eval_count"`
	EvalCount       *int                `json:"eval_count"`
}

type wireResponseMessage struct {
	Content   string         `json:"content"`
	Thinking  string         `json:"thinking"`
	ToolCalls []wireToolCall `json:"tool_calls"`
}

type wireError struct {
	Error string `json:"error"`
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

func translate(wire chatResponse, offered []string) loop.JudgeResult {
	names := offeredWireNames(offered)

	result := loop.JudgeResult{
		Answer:         wire.Message.Content,
		RawReason:      wire.DoneReason,
		ReasoningBytes: len(wire.Message.Thinking),
		Usage:          translateUsage(wire.PromptEvalCount, wire.EvalCount),
	}
	for _, call := range wire.Message.ToolCalls {
		if !slices.Contains(names, call.Function.Name) {
			result.UnofferedToolCalls++
		}
	}

	if len(wire.Message.ToolCalls) > 0 {
		call := wire.Message.ToolCalls[0]

		if !slices.Contains(names, call.Function.Name) && (len(names) == 0 || isKnownWireTool(call.Function.Name)) {
			result.Reason = mapDoneReason(wire.DoneReason)
			return result
		}

		result.ToolSource = loop.ToolSourceNative

		switch call.Function.Name {
		case recallToolName:
			return translateRecall(result, call.Function.Arguments)
		case writeFileToolName:
			return translateWrite(result, call.Function.Arguments)
		case readNodeToolName:
			return translateRead(result, call.Function.Arguments)
		}

		result.Reason = loop.Unrecognised
		return result
	}

	if len(names) > 0 {
		if call, detected := recoverCall(wire.Message.Content); detected {
			return translateRecoveredCall(result, call, names)
		}
	}

	result.Reason = mapDoneReason(wire.DoneReason)
	return result
}

func translateRecoveredCall(result loop.JudgeResult, call recoveredCall, names []string) loop.JudgeResult {
	result.ToolSource = loop.ToolSourceContent

	if !slices.Contains(names, call.name) {
		result.Reason = loop.Unrecognised
		return result
	}

	switch call.name {
	case recallToolName:
		return recoverRecall(result, call)
	case writeFileToolName:
		return recoverWrite(result, call)
	case readNodeToolName:
		return recoverRead(result, call)
	}

	result.Reason = loop.Unrecognised
	return result
}

func recoverRecall(result loop.JudgeResult, call recoveredCall) loop.JudgeResult {
	result.Reason = loop.WantsRecall

	raw, present := call.parameters["query"]
	if !present {
		result.ToolError = "the call in the response text was missing its query argument"
		return result
	}

	query := strings.TrimSpace(raw)
	if query == "" {
		result.ToolError = "tool arguments had an empty query"
		return result
	}

	result.RecallQuery = query
	return result
}

func recoverWrite(result loop.JudgeResult, call recoveredCall) loop.JudgeResult {
	result.Reason = loop.WantsWrite

	path, hasPath := call.parameters["path"]
	content, hasContent := call.parameters["content"]
	if !hasPath || !hasContent {
		result.ToolError = "the call in the response text was missing its path or content argument"
		return result
	}

	result.WritePath = path
	result.WriteContent = content
	return result
}

func recoverRead(result loop.JudgeResult, call recoveredCall) loop.JudgeResult {
	result.Reason = loop.WantsRead

	raw, present := call.parameters["id"]
	if !present {
		result.ToolError = errRecoveredCallWithoutID
		return result
	}

	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		result.ToolError = fmt.Sprintf("%s: %q", errNoNodeID, raw)
		return result
	}

	result.ReadNodeID = id
	return result
}

func translateRead(result loop.JudgeResult, arguments json.RawMessage) loop.JudgeResult {
	result.Reason = loop.WantsRead

	var args readNodeToolArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
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

func translateRecall(result loop.JudgeResult, arguments json.RawMessage) loop.JudgeResult {
	result.Reason = loop.WantsRecall

	var args recallToolArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
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

func translateWrite(result loop.JudgeResult, arguments json.RawMessage) loop.JudgeResult {
	result.Reason = loop.WantsWrite

	var args writeFileToolArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
		result.ToolError = fmt.Sprintf("tool arguments could not be parsed: %v", err)
		return result
	}

	result.WritePath = args.Path
	result.WriteContent = args.Content
	return result
}

func mapDoneReason(reason string) loop.TerminalReason {
	switch reason {
	case "stop":
		return loop.Answered
	case "length":
		return loop.Truncated
	default:
		return loop.Unrecognised
	}
}

func translateUsage(promptEvalCount, evalCount *int) *loop.Usage {
	if promptEvalCount == nil || evalCount == nil {
		return nil
	}
	return &loop.Usage{
		InTokens:  *promptEvalCount,
		OutTokens: *evalCount,
	}
}

type recoveredCall struct {
	name       string
	parameters map[string]string
}

func recoverCall(content string) (recoveredCall, bool) {
	open := strings.Index(content, functionOpenMarker)
	if open < 0 {
		return recoveredCall{}, false
	}

	rest := content[open+len(functionOpenMarker):]
	nameEnd := strings.Index(rest, markerEndMarker)
	if nameEnd < 0 {
		return recoveredCall{parameters: map[string]string{}}, true
	}

	call := recoveredCall{name: strings.TrimSpace(rest[:nameEnd])}
	body := rest[nameEnd+len(markerEndMarker):]
	if end := strings.Index(body, functionCloseMarker); end >= 0 {
		body = body[:end]
	}
	call.parameters = recoverParameters(body)
	return call, true
}

func recoverParameters(body string) map[string]string {
	parameters := map[string]string{}

	for {
		open := strings.Index(body, parameterOpenMarker)
		if open < 0 {
			return parameters
		}
		body = body[open+len(parameterOpenMarker):]

		nameEnd := strings.Index(body, markerEndMarker)
		if nameEnd < 0 {
			return parameters
		}
		name := strings.TrimSpace(body[:nameEnd])
		body = body[nameEnd+len(markerEndMarker):]

		valueEnd := strings.Index(body, parameterCloseMarker)
		if valueEnd < 0 {
			return parameters
		}
		parameters[name] = trimOneNewline(body[:valueEnd])
		body = body[valueEnd+len(parameterCloseMarker):]
	}
}

func trimOneNewline(value string) string {
	return strings.TrimSuffix(strings.TrimPrefix(value, "\n"), "\n")
}
