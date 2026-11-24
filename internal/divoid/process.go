package divoid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/process"
)

func (c *Client) FindNodes(ctx context.Context, name, nodeType string) ([]loop.Anchor, error) {
	q := url.Values{"name": {name}, "type": {nodeType}, "count": {"500"}, "fields": {nodeFields}}
	nodes := []loop.Anchor{}
	seen := map[int]bool{}
	for {
		var page listingResponse
		if err := c.get(ctx, nodesPath, q, &page); err != nil {
			return nil, err
		}
		for _, row := range page.Result {
			if row.Name == name && row.Type == nodeType {
				nodes = append(nodes, loop.Anchor{ID: row.ID, Name: row.Name, Type: row.Type, Content: row.Content})
				if len(nodes) > 1 {
					return nodes, nil
				}
			}
		}
		if page.Continue == nil {
			return nodes, nil
		}
		if seen[*page.Continue] {
			return nil, fmt.Errorf("nonadvancing node continuation")
		}
		seen[*page.Continue] = true
		q.Set("continue", strconv.Itoa(*page.Continue))
	}
}

func (c *Client) Links(ctx context.Context, id int64) ([]process.Edge, error) {
	q := url.Values{"ids": {strconv.FormatInt(id, 10)}, "count": {"500"}}
	edges := []process.Edge{}
	seen := map[int]bool{}
	for {
		var page struct {
			Result   []process.Edge `json:"result"`
			Continue *int           `json:"continue"`
		}
		if err := c.get(ctx, nodesPath+"/links", q, &page); err != nil {
			return nil, err
		}
		for _, edge := range page.Result {
			if edge.Source == id || edge.Target == id {
				edges = append(edges, edge)
			}
		}
		if page.Continue == nil {
			return edges, nil
		}
		if seen[*page.Continue] {
			return nil, fmt.Errorf("nonadvancing link continuation")
		}
		seen[*page.Continue] = true
		q.Set("continue", strconv.Itoa(*page.Continue))
	}
}

func (c *Client) CreateProcessNode(ctx context.Context, nodeType, name string) (int64, error) {
	return c.createNode(ctx, nodeType, name, 0, true, "process definition")
}

func (c *Client) SetProcessContent(ctx context.Context, id int64, content string) error {
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), "application/json; charset=utf-8", []byte(content), nil)
}

func (c *Client) SetProcessTextContent(ctx context.Context, id int64, content string) error {
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), "text/markdown; charset=utf-8", []byte(content), nil)
}

func (c *Client) LinkProcessNodes(ctx context.Context, edge process.Edge) error {
	q := url.Values{"linkType": {edge.LinkType}, "context": {edge.Context}}
	body, err := json.Marshal(edge.Target)
	if err != nil {
		return err
	}
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/links?%s", edge.Source, q.Encode()), "application/json", body, nil)
}
