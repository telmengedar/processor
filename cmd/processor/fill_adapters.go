package main

import (
	"context"
	"fmt"

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

func newFillModel(cfg boot.ModelConfig, sampling loop.Sampling) (condense.ModelPort, error) {
	switch cfg.Protocol {
	case boot.ProtocolOpenAICompat:
		return &fillOpenAICompatModel{client: openaicompat.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil)}, nil
	case boot.ProtocolOllama:
		return &fillOllamaModel{client: ollama.NewClient(cfg.URL, cfg.ID, cfg.Key, sampling, nil)}, nil
	}
	return nil, fmt.Errorf("condense model protocol %q has no adapter in this binary", cfg.Protocol)
}

func newFillPort(cfg boot.ModelConfig, configured bool, graph *divoid.Client) (loop.FillPort, error) {
	if !configured {
		return nil, nil
	}
	sampling := loop.Sampling{Temperature: cfg.Temperature, TopP: cfg.TopP}
	model, err := newFillModel(cfg, sampling)
	if err != nil {
		return nil, err
	}
	return fill.New(&fillGraphAdapter{client: graph}, model, nil), nil
}
