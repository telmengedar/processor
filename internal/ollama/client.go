// Package ollama is the model adapter for ollama's native chat protocol.
package ollama

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
	"github.com/telmengedar/processor/internal/redacturl"
)

// DefaultTimeout bounds the model call when the caller supplies no *http.Client.
const DefaultTimeout = 5 * time.Minute

const adapterName = "ollama"

const chatRoute = "/api/chat"

const (
	recallToolName    = "recall"
	writeFileToolName = "write_file"
)

const (
	functionOpenMarker   = "<function="
	functionCloseMarker  = "</function>"
	parameterOpenMarker  = "<parameter="
	parameterCloseMarker = "</parameter>"
	markerEndMarker      = ">"
)

var _ loop.ModelPort = (*Client)(nil)

// Client is a client for ollama's native chat protocol.
type Client struct {
	baseURL    string
	modelID    string
	apiKey     string
	sampling   loop.Sampling
	httpClient *http.Client
}

// NewClient builds a Client against baseURL, requesting modelID and sampling on every call.
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
	endpoint := c.baseURL + chatRoute

	result, requestBytes, err := c.judge(ctx, in, endpoint)
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("ollama: model=%s endpoint=%s request=%d B elapsed=%s (client bound %s): %w",
			c.modelID, redacturl.URL(endpoint), requestBytes,
			time.Since(started).Round(time.Millisecond), c.client().Timeout, err)
	}
	return result, nil
}

func (c *Client) judge(ctx context.Context, in loop.JudgeInput, endpoint string) (loop.JudgeResult, int, error) {
	reqBody := chatRequest{
		Model:    c.modelID,
		Messages: buildMessages(in),
		Stream:   false,
		Tools:    []wireTool{recallTool(), writeFileTool()},
		Options: wireOptions{
			NumPredict:  loop.MaxOutputTokens,
			Temperature: c.sampling.Temperature,
			TopP:        c.sampling.TopP,
		},
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
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return loop.JudgeResult{}, len(body), fmt.Errorf("decode response: %w", err)
	}

	result := translate(wire)
	result.Sampling = c.sampling
	result.Provider = loop.Provider{Adapter: adapterName, Endpoint: redacturl.URL(endpoint)}
	return result, len(body), nil
}

func readUpstreamMessage(r io.Reader) string {
	body, _ := io.ReadAll(io.LimitReader(r, 4096))
	var e wireError
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return e.Error
	}
	return string(body)
}
