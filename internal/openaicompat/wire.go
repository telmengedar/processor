package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/telmengedar/processor/internal/action"
	"github.com/telmengedar/processor/internal/loop"
)

const problemExcerptBytes = 1024

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
}

type recallArguments struct {
	Query string `json:"query"`
}

type writeFileArguments struct {
	Path    string `json:"path"`
	Content string `json:"content"`
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
	Content *string `json:"content"`
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

	for _, round := range groupRounds(in.PriorTools) {
		if declared := renderDeclarations(round); declared != "" {
			messages = append(messages, wireMessage{Role: "assistant", Content: declared})
		}
		messages = append(messages, wireMessage{Role: "user", Content: renderResults(round)})
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

func groupRounds(exchanges []loop.ToolExchange) [][]loop.ToolExchange {
	var rounds [][]loop.ToolExchange
	for i, e := range exchanges {
		if i > 0 && exchanges[i-1].Round == e.Round {
			rounds[len(rounds)-1] = append(rounds[len(rounds)-1], e)
			continue
		}
		rounds = append(rounds, []loop.ToolExchange{e})
	}
	return rounds
}

func renderDeclarations(round []loop.ToolExchange) string {
	var blocks []string
	for _, e := range round {
		if e.Tool == loop.ToolUnparsed {
			continue
		}
		blocks = append(blocks, action.Open+"\n"+declaration(e)+"\n"+action.Close)
	}
	return strings.Join(blocks, "\n")
}

func declaration(e loop.ToolExchange) string {
	encoded, err := json.Marshal(map[string]any{"name": wireActionName(e.Tool), "arguments": declaredArguments(e)})
	if err != nil {
		return ""
	}
	return string(encoded)
}

func declaredArguments(e loop.ToolExchange) any {
	if e.Tool == loop.ToolWriteFile {
		return writeFileArguments{Path: e.Path, Content: e.Content}
	}
	return recallArguments{Query: e.Query}
}

func wireActionName(tool string) string {
	if tool == loop.ToolWriteFile {
		return action.WriteFile
	}
	return action.Recall
}

func renderResults(round []loop.ToolExchange) string {
	var b strings.Builder
	for i, e := range round {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "===== ACTION RESULT =====\naction: %s\n%s\n", resultHeading(e), renderToolResult(e))
	}
	return b.String()
}

func resultHeading(e loop.ToolExchange) string {
	if e.Tool == loop.ToolUnparsed {
		return "not read"
	}
	return wireActionName(e.Tool)
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

	content := ""
	if choice.Message.Content != nil {
		content = *choice.Message.Content
	}

	parsed := action.Parse(content)
	result := loop.JudgeResult{
		Answer:    parsed.Prose,
		RawReason: choice.FinishReason,
		Usage:     translateUsage(wire.Usage),
	}

	for _, p := range parsed.Problems {
		result.Problems = append(result.Problems, describeProblem(p))
	}

	unknown := false
	for _, call := range parsed.Calls {
		act, offered := translateCall(call)
		if !offered {
			unknown = true
			result.Problems = append(result.Problems, fmt.Sprintf("the response asked for %q, which is not an action this harness offers", call.Name))
			continue
		}
		result.Actions = append(result.Actions, act)
	}

	result.Reason = reasonFor(result, len(parsed.Problems) > 0, unknown, choice.FinishReason)
	return result, nil
}

func reasonFor(result loop.JudgeResult, unreadable, unknown bool, finish string) loop.TerminalReason {
	if len(result.Actions) > 0 {
		if result.Actions[0].Tool == loop.ToolWriteFile {
			return loop.WantsWrite
		}
		return loop.WantsRecall
	}
	if len(result.Problems) == 0 {
		return mapFinishReason(finish)
	}
	if finish == "length" {
		return loop.Truncated
	}
	if unreadable {
		return loop.Malformed
	}
	if unknown {
		return loop.Unrecognised
	}
	return loop.Malformed
}

func translateCall(call action.Call) (loop.Action, bool) {
	switch call.Name {
	case action.Recall:
		return recallAction(call.Arguments), true
	case action.WriteFile:
		return writeAction(call.Arguments), true
	}
	return loop.Action{}, false
}

func recallAction(arguments json.RawMessage) loop.Action {
	act := loop.Action{Tool: loop.ToolRecall}

	var args recallArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
		act.Error = fmt.Sprintf("action arguments could not be parsed: %v", err)
		return act
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		act.Error = "action arguments had an empty query"
		return act
	}

	act.RecallQuery = query
	return act
}

func writeAction(arguments json.RawMessage) loop.Action {
	act := loop.Action{Tool: loop.ToolWriteFile}

	var args writeFileArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
		act.Error = fmt.Sprintf("action arguments could not be parsed: %v", err)
		return act
	}

	act.WritePath = args.Path
	act.WriteContent = args.Content
	return act
}

func describeProblem(p action.Problem) string {
	excerpt := p.Text
	ellipsis := ""
	if len(excerpt) > problemExcerptBytes {
		excerpt = strings.ToValidUTF8(excerpt[:problemExcerptBytes], "")
		ellipsis = "…"
	}
	return fmt.Sprintf("%s (%d bytes): %s%s", p.Reason, len(p.Text), excerpt, ellipsis)
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
