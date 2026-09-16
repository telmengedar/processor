package main

import (
	"context"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/runbackfill"
)

var _ runbackfill.GraphPort = (*graphAdapter)(nil)

type graphAdapter struct {
	client *divoid.Client
}

func (g *graphAdapter) NodeWithSubstance(ctx context.Context, id int64) (runbackfill.Node, bool, error) {
	row, found, err := g.client.NodeWithSubstance(ctx, id)
	if err != nil || !found {
		return runbackfill.Node{}, false, err
	}
	return runbackfill.Node{
		ID:          row.ID,
		Type:        row.Type,
		Name:        row.Name,
		ContentType: row.ContentType,
		Content:     row.Content,
		Substance:   row.Substance,
	}, true, nil
}

func (g *graphAdapter) Content(ctx context.Context, id int64) (string, bool, error) {
	return g.client.Content(ctx, id)
}

func (g *graphAdapter) SetRunContent(ctx context.Context, id int64, content []byte) error {
	return g.client.SetRunContent(ctx, id, content)
}

func (g *graphAdapter) SetSubstance(ctx context.Context, id int64, substance string) error {
	return g.client.SetSubstance(ctx, id, substance)
}
