package divoid

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/process"
)

func (c *Client) Memorize(ctx context.Context, request process.MemoryRequest) (process.MemoryResult, error) {
	target := request.Target
	if target.Root <= 0 || target.Anchor <= 0 || target.Type == "" || request.Text == "" || target.Access != nil && (*target.Access < 0 || *target.Access > 3) {
		return process.MemoryResult{}, fmt.Errorf("invalid memory request")
	}
	written := c.writeNote(ctx, loop.NoteRequest{Type: target.Type, Root: target.Root, Subject: target.Anchor, About: request.About, Name: loop.NoteName(request.Text), Text: request.Text}, target.Access)
	result := process.MemoryResult{State: string(written.State), NodeID: written.NodeID}
	if written.State != loop.Stored || written.NodeID <= 0 {
		return result, fmt.Errorf("memory write incomplete")
	}
	fields := nodeFields
	if target.Access != nil {
		fields += ",access"
	}
	node, found, err := c.nodeRow(ctx, written.NodeID, nil, fields)
	if err != nil || !found || node.ID != written.NodeID || node.Type != target.Type || node.RootNodeID == nil || *node.RootNodeID != target.Root || process.Hash(node.Content) != process.Hash(request.Text) {
		return result, fmt.Errorf("memory readback mismatch")
	}
	if target.Access != nil {
		access, valid := nodeAccess(node.Access)
		if !valid || access != *target.Access {
			return result, fmt.Errorf("memory access readback mismatch")
		}
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

func nodeAccess(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return 0, false
	}
	var value int
	if json.Unmarshal(raw, &value) == nil {
		return value, value >= 0 && value <= 3
	}
	var name string
	if json.Unmarshal(raw, &name) != nil {
		return 0, false
	}
	switch name {
	case "None":
		return 0, true
	case "Read":
		return 1, true
	case "Write":
		return 2, true
	case "Read, Write":
		return 3, true
	}
	return 0, false
}
