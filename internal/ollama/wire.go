package ollama

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	errNoNodeID                 = "tool arguments named no node id to read"
	errRecoveredCallWithoutID   = "the call in the response text was missing its id argument"
	errNoNoteText               = "tool arguments carried no text to store"
	errBadNoteAbout             = "tool arguments named an about that is not a positive integer id"
	errRecoveredNoteWithoutText = "the call in the response text was missing its text argument"
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
	NumPredict  int      `json:"num_predict,omitempty"`
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

type noteToolArguments struct {
	Text  *string         `json:"text"`
	About json.RawMessage `json:"about"`
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
			Description: "Read one part of memory as its substance. Takes one argument: id, the integer id that part is printed with.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {"id": {"type": "integer"}},
				"required": ["id"]
			}`),
		},
	}
}

func noteTool() wireTool {
	return wireTool{
		Type: "function",
		Function: wireFunction{
			Name:        noteToolName,
			Description: "Store one short text of yours in memory, where later requests can find it. Accepts one note per request. Takes two arguments: text, the note, and optionally about, the integer id of the one part of memory you have been shown that the note is about.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {"type": "string"},
					"about": {"type": "integer"}
				},
				"required": ["text"]
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
		{Role: "user", Content: loop.RenderUserContent(in.Block, in.Input, in.Now, in.Window, in.Conversation, in.Trailer)},
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
	{loop.ToolNote, noteToolName, noteTool},
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
			return flagOpeningCall(result, wire.Message.Content)
		}

		result.ToolSource = loop.ToolSourceNative

		switch call.Function.Name {
		case recallToolName:
			return translateRecall(result, call.Function.Arguments)
		case writeFileToolName:
			return translateWrite(result, call.Function.Arguments)
		case readNodeToolName:
			return translateRead(result, call.Function.Arguments)
		case noteToolName:
			return translateNote(result, call.Function.Arguments)
		}

		result.Reason = loop.Unrecognised
		return flagOpeningCall(result, wire.Message.Content)
	}

	if len(names) > 0 {
		if call, detected := recoverCall(wire.Message.Content); detected {
			return translateRecoveredCall(result, call, names, loop.ToolSourceContent)
		}
	}

	return translateTextualCall(result, wire.Message.Content, wire.DoneReason, names)
}

func flagOpeningCall(result loop.JudgeResult, content string) loop.JudgeResult {
	if tool, _, opens := openingToolCall(content); opens {
		result.UnhonouredCall = tool.loopName
	}
	return result
}

func translateTextualCall(result loop.JudgeResult, content, doneReason string, names []string) loop.JudgeResult {
	tool, rest, opens := openingToolCall(content)
	if !opens {
		result.Reason = mapDoneReason(doneReason)
		return result
	}

	recoverable := slices.IndexFunc(textualRecoverable, func(r textualRecovery) bool { return r.loopName == tool.loopName })
	if recoverable < 0 || !slices.Contains(names, tool.wireName) {
		result.Reason = mapDoneReason(doneReason)
		result.UnhonouredCall = tool.loopName
		return result
	}

	parameter := textualRecoverable[recoverable].parameter
	call := recoveredCall{name: tool.wireName, parameters: map[string]string{}}
	if value, present := textualArgument(rest, parameter); present {
		call.parameters[parameter] = value
	}
	return translateRecoveredCall(result, call, names, loop.ToolSourceTextual)
}

type textualRecovery struct {
	loopName  string
	parameter string
}

var textualRecoverable = []textualRecovery{
	{loop.ToolRecall, "query"},
	{loop.ToolReadNode, "id"},
}

func openingToolCall(content string) (toolBinding, string, bool) {
	text := strings.TrimLeftFunc(content, unicode.IsSpace)

	for _, tool := range toolTable {
		end := len(tool.wireName)
		if len(text) <= end || !strings.EqualFold(text[:end], tool.wireName) {
			continue
		}
		if delimiter := text[end]; delimiter == ':' || delimiter == '{' {
			return tool, text[end:], true
		}
	}
	return toolBinding{}, "", false
}

func textualArgument(rest, parameter string) (string, bool) {
	if rest[0] == ':' {
		line, _, _ := strings.Cut(rest[1:], "\n")
		return unquoted(strings.TrimSpace(line)), true
	}

	end := strings.IndexByte(rest, '}')
	if end < 0 {
		return "", false
	}
	key, value, hasValue := strings.Cut(rest[1:end], ":")
	if !hasValue || unquoted(strings.TrimSpace(key)) != parameter {
		return "", false
	}
	return unquoted(strings.TrimSpace(value)), true
}

func unquoted(value string) string {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1]
	}
	return value
}

func translateRecoveredCall(result loop.JudgeResult, call recoveredCall, names []string, source loop.ToolSource) loop.JudgeResult {
	result.ToolSource = source

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
	case noteToolName:
		return recoverNote(result, call)
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

func recoverNote(result loop.JudgeResult, call recoveredCall) loop.JudgeResult {
	result.Reason = loop.WantsNote

	text, present := call.parameters["text"]
	if !present {
		result.ToolError = errRecoveredNoteWithoutText
		return result
	}

	about, err := aboutOf(json.RawMessage(strings.TrimSpace(call.parameters["about"])))
	if err != nil {
		result.ToolError = err.Error()
		return result
	}

	result.NoteText = text
	result.NoteAbout = about
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

func translateNote(result loop.JudgeResult, arguments json.RawMessage) loop.JudgeResult {
	result.Reason = loop.WantsNote

	var args noteToolArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
		result.ToolError = fmt.Sprintf("tool arguments could not be parsed: %v", err)
		return result
	}
	if args.Text == nil {
		result.ToolError = errNoNoteText
		return result
	}

	about, err := aboutOf(args.About)
	if err != nil {
		result.ToolError = err.Error()
		return result
	}

	result.NoteText = *args.Text
	result.NoteAbout = about
	return result
}

func aboutOf(raw json.RawMessage) (int64, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return 0, nil
	}

	var about int64
	if err := json.Unmarshal(raw, &about); err != nil || about <= 0 {
		return 0, fmt.Errorf("%s: %s", errBadNoteAbout, text)
	}
	return about, nil
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
