package main

import (
	"context"

	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/openaicompat"
)

var (
	_ condense.GraphPort = (*graphAdapter)(nil)
	_ condense.ModelPort = (*modelAdapter)(nil)
)

type graphAdapter struct {
	client *divoid.Client
}

func (g *graphAdapter) NodeWithSubstance(ctx context.Context, id int64) (condense.Node, bool, error) {
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

func (g *graphAdapter) Content(ctx context.Context, id int64) (string, bool, error) {
	return g.client.Content(ctx, id)
}

func (g *graphAdapter) SetSubstance(ctx context.Context, id int64, substance string) error {
	return g.client.SetSubstance(ctx, id, substance)
}

type modelAdapter struct {
	client *openaicompat.Client
}

func (m *modelAdapter) Condense(ctx context.Context, prompt string, maxOutputTokens int) (condense.Completion, error) {
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
