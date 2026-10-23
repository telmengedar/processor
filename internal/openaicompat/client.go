// Package openaicompat is the model adapter for the OpenAI-compatible chat-completions protocol.
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/redacturl"
)

// DefaultTimeout bounds the model call when the caller supplies no *http.Client.
const DefaultTimeout = 5 * time.Minute

const adapterName = "openai-compat"

// ThinkingSuppression names the control this adapter sends on every request to keep the model's reasoning channel off.
const ThinkingSuppression = "reasoning_effort=none"

// ThinkingSuppressionStanding states where that control comes from, which for this adapter is the endpoint rather than the protocol.
const ThinkingSuppressionStanding = "endpoint capability, beyond the protocol"

const reasoningEffortNone = "none"

const chatCompletionsRoute = "/chat/completions"

const errorBodyBytes = 4096

const (
	recallToolName    = "recall"
	writeFileToolName = "write_file"
	readNodeToolName  = "read_node"
	noteToolName      = "write_note"
)

var _ loop.ModelPort = (*Client)(nil)

// Client is a client for the OpenAI-compatible chat-completions protocol.
type Client struct {
	baseURL    string
	modelID    string
	apiKey     string
	sampling   loop.Sampling
	httpClient *http.Client
}

// NewClient builds a Client against baseURL, requesting modelID and sampling on every call, a judgement call's own temperature replacing the sampling's for that call.
func NewClient(baseURL, modelID, apiKey string, sampling loop.Sampling, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		modelID:    modelID,
		apiKey:     apiKey,
		sampling:   sampling,
		httpClient: httpClient,
	}
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

func (c *Client) client() *http.Client {
	if c.httpClient == nil {
		return defaultHTTPClient()
	}
	return c.httpClient
}

// Judge runs one judgement step against the endpoint.
func (c *Client) Judge(ctx context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	started := time.Now()
	endpoint := c.baseURL + chatCompletionsRoute

	result, requestBytes, err := c.judge(ctx, in, endpoint)
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: model=%s endpoint=%s request=%d B elapsed=%s (client bound %s): %w",
			c.modelID, redacturl.URL(endpoint), requestBytes,
			time.Since(started).Round(time.Millisecond), c.client().Timeout, err)
	}
	return result, nil
}

func (c *Client) judge(ctx context.Context, in loop.JudgeInput, endpoint string) (loop.JudgeResult, int, error) {
	sampling := c.callSampling(in)
	reqBody := chatRequest{
		Model:           c.modelID,
		Messages:        buildMessages(in),
		MaxTokens:       in.MaxOutputTokens,
		ReasoningEffort: reasoningEffortNone,
		Temperature:     sampling.Temperature,
		TopP:            sampling.TopP,
	}
	if in.NoteDecisionOutput {
		reqBody.ResponseFormat = &wireResponseFormat{Type: "json_schema", JSONSchema: wireJSONSchema{Name: "note_decision", Strict: true, Schema: loop.NoteDecisionSchema()}}
	} else if in.StructuredAllowedActions != nil {
		reqBody.ResponseFormat = &wireResponseFormat{Type: "json_schema", JSONSchema: wireJSONSchema{Name: "structured_read", Strict: true, Schema: loop.StructuredReadSchema(in.StructuredAllowedActions)}}
	} else {
		reqBody.Tools = declaredTools(in.Offered)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return loop.JudgeResult{}, 0, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return loop.JudgeResult{}, len(body), fmt.Errorf("build request: %w", redacturl.Error(err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return loop.JudgeResult{}, len(body), fmt.Errorf("request failed: %w", redacturl.Error(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return loop.JudgeResult{}, len(body), fmt.Errorf("unexpected status %d: %s", resp.StatusCode, readUpstreamMessage(resp.Body))
	}

	var wire chatResponse
	if in.NoteDecisionOutput || in.StructuredAllowedActions != nil {
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return loop.JudgeResult{}, len(body), fmt.Errorf("read structured response: %w", err)
		}
		if !utf8.Valid(raw) {
			return loop.JudgeResult{}, len(body), fmt.Errorf("structured response is not UTF-8")
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			return loop.JudgeResult{}, len(body), fmt.Errorf("decode response: %w", err)
		}
	} else if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return loop.JudgeResult{}, len(body), fmt.Errorf("decode response: %w", err)
	}

	var result loop.JudgeResult
	if in.NoteDecisionOutput || in.StructuredAllowedActions != nil {
		if len(wire.Choices) == 0 {
			return loop.JudgeResult{}, len(body), fmt.Errorf("response has no choices")
		}
		choice := wire.Choices[0]
		var content string
		if choice.Message.Content != nil {
			content = *choice.Message.Content
		}
		result = loop.JudgeResult{Answer: content, RawReason: choice.FinishReason, Reason: loop.Answered, ReasoningBytes: len(choice.Message.Reasoning), Usage: translateUsage(wire.Usage), UnofferedToolCalls: len(choice.Message.ToolCalls)}
	} else {
		result, err = translate(wire, in.Offered)
		if err != nil {
			return loop.JudgeResult{}, len(body), err
		}
	}
	result.Sampling = sampling
	result.Provider = loop.Provider{Adapter: adapterName, Endpoint: redacturl.URL(endpoint)}
	return result, len(body), nil
}

func (c *Client) callSampling(in loop.JudgeInput) loop.Sampling {
	sampling := c.sampling
	if in.Temperature != nil {
		sampling.Temperature = in.Temperature
	}
	return sampling
}

func readUpstreamMessage(r io.Reader) string {
	body, _ := io.ReadAll(io.LimitReader(r, errorBodyBytes))
	var e wireError
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return string(body)
}
