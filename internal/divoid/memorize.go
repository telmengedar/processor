package divoid

import (
	"context"
	"fmt"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/process"
)

func (c *Client) Memorize(ctx context.Context, request process.MemoryRequest) (process.MemoryResult, error) {
	target := request.Target
	if target.Root <= 0 || target.Anchor <= 0 || target.Type == "" || request.Text == "" {
		return process.MemoryResult{}, fmt.Errorf("invalid memory request")
	}
	written := c.WriteNote(ctx, loop.NoteRequest{Type: target.Type, Root: target.Root, Subject: target.Anchor, About: request.About, Name: loop.NoteName(request.Text), Text: request.Text})
	result := process.MemoryResult{State: string(written.State), NodeID: written.NodeID}
	if written.State != loop.Stored || written.NodeID <= 0 {
		return result, fmt.Errorf("memory write incomplete")
	}
	node, found, err := c.Node(ctx, written.NodeID, nil)
	if err != nil || !found || node.ID != written.NodeID || node.Type != target.Type || node.RootNodeID != target.Root || process.Hash(node.Content) != process.Hash(request.Text) {
		return result, fmt.Errorf("memory readback mismatch")
	}
	links, err := c.Links(ctx, written.NodeID)
	if err != nil {
		return result, fmt.Errorf("memory link readback failed")
	}
	for _, required := range []int64{target.Anchor, request.About} {
		if required <= 0 {
			continue
		}
		found := false
		for _, edge := range links {
			if edge.Source == written.NodeID && edge.Target == required || edge.Target == written.NodeID && edge.Source == required {
				found = true
			}
		}
		if !found {
			return result, fmt.Errorf("memory link readback mismatch")
		}
	}
	result.Confirmed = true
	result.ContentHash = process.Hash(node.Content)
	return result, nil
}
