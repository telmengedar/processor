package process

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"
)

func exposedNodeIDs(value any) []int64 {
	ids := []int64{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case *receiptHandle:
			ids = append(ids, x.NodeID)
		case map[string]any:
			for key, v := range x {
				if key == "id" || key == "ID" {
					if id, ok := nodeID(v); ok {
						ids = append(ids, id)
					}
				}
				walk(v)
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		default:
			// Normalize graph-port structs without serializing away runtime
			// receipt authority inside the transport containers above.
			switch normalized := jsonValue(v).(type) {
			case map[string]any:
				walk(normalized)
			case []any:
				walk(normalized)
			}
		}
	}
	walk(value)
	return ids
}

func (e *Engine) toolV2(ctx context.Context, tool Tool, args map[string]any, r *transportRuntime, exposed []int64) any {
	if tool.Builtin != "memorize" {
		return e.tool(ctx, tool, args)
	}
	text, ok := args["text"].(string)
	if !ok || !utf8.ValidString(text) || strings.TrimSpace(text) == "" {
		return map[string]any{"ok": false, "error": failure("tool_input", "memorize requires nonblank Unicode text")}
	}
	about := int64(0)
	if value, exists := args["about"]; exists {
		var valid bool
		about, valid = nodeID(value)
		if !valid || !slices.Contains(exposed, about) {
			return map[string]any{"ok": false, "error": failure("tool_input", "about must be an exposed node reference")}
		}
	}
	writer, ok := e.Graph.(MemoryWriter)
	if !ok {
		return map[string]any{"ok": false, "error": failure("tool_unavailable", "memory writer unavailable")}
	}
	observed, err := writer.Memorize(ctx, MemoryRequest{Target: *tool.Memory, Text: text, About: about})
	if err != nil || !observed.Confirmed || observed.State != "stored" || observed.NodeID <= 0 || observed.ContentHash != Hash(text) {
		observed.Confirmed = false
		return map[string]any{"ok": false, "error": failure("memory_write_incomplete", "memory write or required readback was not confirmed"), "repair": observed}
	}
	receipt := &receiptHandle{MemoryReceipt: MemoryReceipt{Operation: "memorize", NodeID: observed.NodeID, ContentHash: observed.ContentHash, StepID: r.step, Visit: r.visit}}
	r.receipts = append(r.receipts, receipt)
	r.fresh = append(r.fresh, receipt)
	return map[string]any{"ok": true, "value": receipt}
}
