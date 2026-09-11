package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/telmengedar/processor/internal/redacturl"
)

const (
	condenseFrequencyPenalty = 0.0
	condensePresencePenalty  = 0.0
	condenseThinking         = false
)

// SentSampling is the sampling one call carried, decoded back out of the request bytes rather than restated from configuration.
type SentSampling struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	FrequencyPenalty float64  `json:"frequencyPenalty"`
	PresencePenalty  float64  `json:"presencePenalty"`
	MaxTokens        int      `json:"maxTokens"`
}

// CondenseResult is one condensation call's outcome, carrying the model the endpoint reported rather than the one requested.
type CondenseResult struct {
	Text         string
	FinishReason string
	Model        string
	Sampling     SentSampling
}

type condenseRequest struct {
	Model    string          `json:"model"`
	Messages []wireMessage   `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Options  condenseOptions `json:"options"`
}

type condenseOptions struct {
	NumPredict       int      `json:"num_predict"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty"`
	PresencePenalty  float64  `json:"presence_penalty"`
}

type condenseResponse struct {
	Model      string              `json:"model"`
	Message    wireResponseMessage `json:"message"`
	DoneReason string              `json:"done_reason"`
}

// Condense runs one offline condensation call: a single user message, no tools, the repetition penalties pinned to zero, and the reasoning stream switched off.
func (c *Client) Condense(ctx context.Context, prompt string, maxOutputTokens int) (CondenseResult, error) {
	body, err := json.Marshal(condenseRequest{
		Model:    c.modelID,
		Messages: []wireMessage{{Role: "user", Content: prompt}},
		Stream:   false,
		Think:    condenseThinking,
		Options: condenseOptions{
			NumPredict:       maxOutputTokens,
			Temperature:      c.sampling.Temperature,
			TopP:             c.sampling.TopP,
			FrequencyPenalty: condenseFrequencyPenalty,
			PresencePenalty:  condensePresencePenalty,
		},
	})
	if err != nil {
		return CondenseResult{}, fmt.Errorf("ollama: encode condense request: %w", err)
	}

	sampling, err := decodeSentSampling(body)
	if err != nil {
		return CondenseResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatRoute, bytes.NewReader(body))
	if err != nil {
		return CondenseResult{}, fmt.Errorf("ollama: build request: %w", redacturl.Error(err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return CondenseResult{}, fmt.Errorf("ollama: request failed: %w", redacturl.Error(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return CondenseResult{}, fmt.Errorf("ollama: unexpected status %d: %s", resp.StatusCode, readUpstreamMessage(resp.Body))
	}

	var wire condenseResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return CondenseResult{}, fmt.Errorf("ollama: decode response: %w", err)
	}

	return CondenseResult{
		Text:         wire.Message.Content,
		FinishReason: wire.DoneReason,
		Model:        wire.Model,
		Sampling:     sampling,
	}, nil
}

// Derive runs one query-derivation call, returning the completion text alone.
func (c *Client) Derive(ctx context.Context, prompt string, maxOutputTokens int) (string, error) {
	result, err := c.Condense(ctx, prompt, maxOutputTokens)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func decodeSentSampling(body []byte) (SentSampling, error) {
	var sent condenseRequest
	if err := json.Unmarshal(body, &sent); err != nil {
		return SentSampling{}, fmt.Errorf("ollama: read back condense request: %w", err)
	}
	return SentSampling{
		Temperature:      sent.Options.Temperature,
		TopP:             sent.Options.TopP,
		FrequencyPenalty: sent.Options.FrequencyPenalty,
		PresencePenalty:  sent.Options.PresencePenalty,
		MaxTokens:        sent.Options.NumPredict,
	}, nil
}
