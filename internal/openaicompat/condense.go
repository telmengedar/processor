package openaicompat

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
	Model            string        `json:"model"`
	Messages         []wireMessage `json:"messages"`
	MaxTokens        int           `json:"max_tokens"`
	Temperature      *float64      `json:"temperature,omitempty"`
	TopP             *float64      `json:"top_p,omitempty"`
	FrequencyPenalty float64       `json:"frequency_penalty"`
	PresencePenalty  float64       `json:"presence_penalty"`
}

type condenseResponse struct {
	Model   string       `json:"model"`
	Choices []wireChoice `json:"choices"`
}

// Condense runs one offline condensation call: a single user message, no tools, and the repetition penalties pinned to zero.
func (c *Client) Condense(ctx context.Context, prompt string, maxOutputTokens int) (CondenseResult, error) {
	body, err := json.Marshal(condenseRequest{
		Model:            c.modelID,
		Messages:         []wireMessage{{Role: "user", Content: prompt}},
		MaxTokens:        maxOutputTokens,
		Temperature:      c.sampling.Temperature,
		TopP:             c.sampling.TopP,
		FrequencyPenalty: condenseFrequencyPenalty,
		PresencePenalty:  condensePresencePenalty,
	})
	if err != nil {
		return CondenseResult{}, fmt.Errorf("openaicompat: encode condense request: %w", err)
	}

	sampling, err := decodeSentSampling(body)
	if err != nil {
		return CondenseResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return CondenseResult{}, fmt.Errorf("openaicompat: build request: %w", redacturl.Error(err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return CondenseResult{}, fmt.Errorf("openaicompat: request failed: %w", redacturl.Error(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return CondenseResult{}, fmt.Errorf("openaicompat: unexpected status %d: %s", resp.StatusCode, readUpstreamMessage(resp.Body))
	}

	var wire condenseResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return CondenseResult{}, fmt.Errorf("openaicompat: decode response: %w", err)
	}
	if len(wire.Choices) == 0 {
		return CondenseResult{}, fmt.Errorf("openaicompat: condense response has no choices")
	}

	choice := wire.Choices[0]
	result := CondenseResult{
		FinishReason: choice.FinishReason,
		Model:        wire.Model,
		Sampling:     sampling,
	}
	if choice.Message.Content != nil {
		result.Text = *choice.Message.Content
	}
	return result, nil
}

func decodeSentSampling(body []byte) (SentSampling, error) {
	var sent condenseRequest
	if err := json.Unmarshal(body, &sent); err != nil {
		return SentSampling{}, fmt.Errorf("openaicompat: read back condense request: %w", err)
	}
	return SentSampling{
		Temperature:      sent.Temperature,
		TopP:             sent.TopP,
		FrequencyPenalty: sent.FrequencyPenalty,
		PresencePenalty:  sent.PresencePenalty,
		MaxTokens:        sent.MaxTokens,
	}, nil
}
