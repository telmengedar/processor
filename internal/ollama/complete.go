package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/redacturl"
)

type completeMessage struct {
	Role    string         `json:"role"`
	Content string         `json:"content"`
	Calls   []wireToolCall `json:"tool_calls,omitempty"`
	Name    string         `json:"tool_name,omitempty"`
	CallID  string         `json:"tool_call_id,omitempty"`
}

func (c *Client) Complete(ctx context.Context, in completion.Request) (completion.Result, error) {
	messages := make([]completeMessage, 0, len(in.Messages))
	for _, m := range in.Messages {
		w := completeMessage{Role: m.Role, Content: m.Content}
		for _, call := range m.Calls {
			w.Calls = append(w.Calls, wireToolCall{ID: call.ID, Function: wireFunctionCall{Name: call.Name, Arguments: call.Arguments}})
		}
		if m.Role == "tool" {
			w.Name = m.ToolName
			w.CallID = m.CallID
		}
		messages = append(messages, w)
	}
	tools := make([]wireTool, 0, len(in.Tools))
	for _, tool := range in.Tools {
		tools = append(tools, wireTool{Type: "function", Function: wireFunction{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters}})
	}
	temperature := c.sampling.Temperature
	if in.Temperature != nil {
		temperature = in.Temperature
	}
	body, err := json.Marshal(struct {
		Model    string            `json:"model"`
		Messages []completeMessage `json:"messages"`
		Tools    []wireTool        `json:"tools,omitempty"`
		Stream   bool              `json:"stream"`
		Think    bool              `json:"think"`
		Options  wireOptions       `json:"options"`
	}{c.modelID, messages, tools, false, in.Thinking, wireOptions{NumPredict: in.MaxTokens, Temperature: temperature, TopP: c.sampling.TopP}})
	if err != nil {
		return completion.Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatRoute, bytes.NewReader(body))
	if err != nil {
		return completion.Result{}, fmt.Errorf("build completion request: %w", redacturl.Error(err))
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return completion.Result{}, fmt.Errorf("completion transport: %w", redacturl.Error(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return completion.Result{}, fmt.Errorf("completion status %d", resp.StatusCode)
	}
	var wire chatResponse
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return completion.Result{}, fmt.Errorf("completion response exceeds bound or could not be read")
	}
	if err = json.Unmarshal(raw, &wire); err != nil {
		return completion.Result{}, fmt.Errorf("decode completion: %w", err)
	}
	result := completion.Result{Text: wire.Message.Content, FinishReason: wire.DoneReason, Usage: translateUsage(wire.PromptEvalCount, wire.EvalCount), Provider: loop.Provider{Adapter: adapterName, Endpoint: redacturl.URL(c.baseURL + chatRoute)}}
	for i, call := range wire.Message.ToolCalls {
		id := call.ID
		if id == "" {
			id = "ollama-" + strconv.Itoa(i)
		}
		result.Calls = append(result.Calls, completion.Call{ID: id, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return result, nil
}
