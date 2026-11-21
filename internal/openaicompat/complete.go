package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/redacturl"
)

type completeMessage struct {
	Role    string         `json:"role"`
	Content string         `json:"content"`
	Calls   []wireToolCall `json:"tool_calls,omitempty"`
	CallID  string         `json:"tool_call_id,omitempty"`
}

func (c *Client) Complete(ctx context.Context, in completion.Request) (completion.Result, error) {
	if in.Thinking {
		return completion.Result{}, fmt.Errorf("thinking=true is unsupported")
	}
	messages := make([]completeMessage, 0, len(in.Messages))
	for _, m := range in.Messages {
		w := completeMessage{Role: m.Role, Content: m.Content, CallID: m.CallID}
		for _, call := range m.Calls {
			w.Calls = append(w.Calls, wireToolCall{ID: call.ID, Type: "function", Function: wireFunctionCall{Name: call.Name, Arguments: string(call.Arguments)}})
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
		Model           string            `json:"model"`
		Messages        []completeMessage `json:"messages"`
		Tools           []wireTool        `json:"tools,omitempty"`
		MaxTokens       int               `json:"max_tokens,omitempty"`
		ReasoningEffort string            `json:"reasoning_effort"`
		Temperature     *float64          `json:"temperature,omitempty"`
		TopP            *float64          `json:"top_p,omitempty"`
	}{c.modelID, messages, tools, in.MaxTokens, reasoningEffortNone, temperature, c.sampling.TopP})
	if err != nil {
		return completion.Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatCompletionsRoute, bytes.NewReader(body))
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
	if len(wire.Choices) == 0 {
		return completion.Result{}, fmt.Errorf("completion has no choices")
	}
	choice := wire.Choices[0]
	result := completion.Result{FinishReason: choice.FinishReason, Usage: translateUsage(wire.Usage), Provider: loop.Provider{Adapter: adapterName, Endpoint: redacturl.URL(c.baseURL + chatCompletionsRoute)}}
	if choice.Message.Content != nil {
		result.Text = *choice.Message.Content
	}
	for _, call := range choice.Message.ToolCalls {
		result.Calls = append(result.Calls, completion.Call{ID: call.ID, Name: call.Function.Name, Arguments: json.RawMessage(call.Function.Arguments)})
	}
	return result, nil
}
