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

	"github.com/telmengedar/processor/internal/loop"
)

// DefaultTimeout bounds the model call when the caller supplies no *http.Client.
const DefaultTimeout = 5 * time.Minute

const recallToolName = "recall"

var _ loop.ModelPort = (*Client)(nil)

// Client is a client for the OpenAI-compatible chat-completions protocol.
type Client struct {
	baseURL    string
	modelID    string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a Client against baseURL, requesting modelID on every call.
func NewClient(baseURL, modelID, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		modelID:    modelID,
		apiKey:     apiKey,
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
	reqBody := chatRequest{
		Model:     c.modelID,
		Messages:  buildMessages(in),
		MaxTokens: loop.MaxOutputTokens,
		Tools:     []wireTool{recallTool()},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: unexpected status %d: %s", resp.StatusCode, readUpstreamMessage(resp.Body))
	}

	var wire chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: decode response: %w", err)
	}

	return translate(wire)
}

func readUpstreamMessage(r io.Reader) string {
	body, _ := io.ReadAll(io.LimitReader(r, 4096))
	var e wireError
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return string(body)
}
