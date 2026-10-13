// Package ports builds the loop's model and fill ports from boot configuration.
package ports

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/fill"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

var (
	_ condense.GraphPort = (*fillGraphAdapter)(nil)
	_ condense.ModelPort = (*fillOpenAICompatModel)(nil)
	_ condense.ModelPort = (*fillOllamaModel)(nil)
)

type fillGraphAdapter struct {
	client *divoid.Client
}

func (g *fillGraphAdapter) NodeWithSubstance(ctx context.Context, id int64) (condense.Node, bool, error) {
	row, found, err := g.client.NodeWithSubstance(ctx, id)
	if err != nil || !found {
		return condense.Node{}, false, err
	}
	return condense.Node{
		ID:           row.ID,
		Type:         row.Type,
		Name:         row.Name,
		ContentType:  row.ContentType,
		Content:      row.Content,
		Substance:    row.Substance,
		SelfProduced: divoid.IsRunRecord(row.Type, row.Name),
	}, true, nil
}

func (g *fillGraphAdapter) Content(ctx context.Context, id int64) (string, bool, error) {
	return g.client.Content(ctx, id)
}

func (g *fillGraphAdapter) SetSubstance(ctx context.Context, id int64, substance string) error {
	return g.client.SetSubstance(ctx, id, substance)
}

type fillOpenAICompatModel struct {
	client *openaicompat.Client
}

func (m *fillOpenAICompatModel) Condense(ctx context.Context, prompt string, maxOutputTokens int) (condense.Completion, error) {
	result, err := m.client.Condense(ctx, prompt, maxOutputTokens)
	if err != nil {
		return condense.Completion{}, err
	}
	return condense.Completion{
		Text:         result.Text,
		FinishReason: result.FinishReason,
		Model:        result.Model,
		Sampling: condense.Sampling{
			Temperature:      result.Sampling.Temperature,
			TopP:             result.Sampling.TopP,
			FrequencyPenalty: result.Sampling.FrequencyPenalty,
			PresencePenalty:  result.Sampling.PresencePenalty,
			MaxTokens:        result.Sampling.MaxTokens,
		},
	}, nil
}

type fillOllamaModel struct {
	client *ollama.Client
}

func (m *fillOllamaModel) Condense(ctx context.Context, prompt string, maxOutputTokens int) (condense.Completion, error) {
	result, err := m.client.Condense(ctx, prompt, maxOutputTokens)
	if err != nil {
		return condense.Completion{}, err
	}
	return condense.Completion{
		Text:         result.Text,
		FinishReason: result.FinishReason,
		Model:        result.Model,
		Sampling: condense.Sampling{
			Temperature:      result.Sampling.Temperature,
			TopP:             result.Sampling.TopP,
			FrequencyPenalty: result.Sampling.FrequencyPenalty,
			PresencePenalty:  result.Sampling.PresencePenalty,
			MaxTokens:        result.Sampling.MaxTokens,
		},
	}, nil
}

func fillModel(cfg boot.ModelConfig, sampling loop.Sampling) (condense.ModelPort, error) {
	switch cfg.Protocol {
	case boot.ProtocolOpenAICompat:
		return &fillOpenAICompatModel{client: openaicompat.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil)}, nil
	case boot.ProtocolOllama:
		return &fillOllamaModel{client: ollama.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil)}, nil
	}
	return nil, fmt.Errorf("condense model protocol %q has no adapter in this binary", cfg.Protocol)
}

// FillGraph adapts client to the condense pass's own graph seam, which can read a node and write its substance and nothing else.
func FillGraph(client *divoid.Client) condense.GraphPort {
	return &fillGraphAdapter{client: client}
}

// FillModel returns the configuration the fill runs on: the condense model's where one is configured, else the judgement model's, which the second result reports.
func FillModel(condensing boot.ModelConfig, configured bool, judgement boot.ModelConfig) (boot.ModelConfig, bool) {
	if configured {
		return condensing, false
	}
	return judgement, true
}

// FillOnJudgementNotice is the one boot line stating that the fill runs on the judgement model's configuration, with the standing recommendation for a capable condense model.
const FillOnJudgementNotice = "no PROCESSOR_CONDENSE_MODEL_* configuration was found: the fill runs on the judgement model's configuration; a capable condense model is strongly recommended, because nothing checks a substance and a written substance stays until the node's content changes"

// Fill builds the loop's fill port over graph, running on cfg.
func Fill(cfg boot.ModelConfig, graph condense.GraphPort, logger *slog.Logger) (*fill.Port, error) {
	sampling := loop.Sampling{Temperature: cfg.Temperature, TopP: cfg.TopP}
	model, err := fillModel(cfg, sampling)
	if err != nil {
		return nil, err
	}
	return fill.New(graph, model, nil, logger), nil
}

// Model builds the model port that cfg's protocol names, sending sampling with every call.
func Model(cfg boot.ModelConfig, sampling loop.Sampling) (loop.ModelPort, error) {
	switch cfg.Protocol {
	case boot.ProtocolOpenAICompat:
		return openaicompat.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil), nil
	case boot.ProtocolOllama:
		return ollama.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil), nil
	}
	return nil, fmt.Errorf("model protocol %q has no adapter in this binary", cfg.Protocol)
}
