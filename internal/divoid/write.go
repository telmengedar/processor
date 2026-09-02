package divoid

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

// runNodeType, runNamePrefix and runContentType are design §8.3's written
// node contract, fixed rather than configured: the adapter alone chooses
// type, name and edge — the loop supplies no structure, only the record.
const (
	runNodeType    = "session-log" // #10424 §5.7's own name for the narrative tier
	runNamePrefix  = "processor-run"
	runContentType = "application/json"

	// runNameInputRunes bounds the input excerpt in the written node's
	// name (design §8.3: "a bounded prefix of the input, so a graph
	// listing is legible without opening anything"). Measured in runes,
	// not bytes, so a multi-byte-rune input isn't truncated mid-character.
	runNameInputRunes = 80
)

type createNodeRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type createNodeResponse struct {
	ID int64 `json:"id"`
}

// WriteRun implements loop.GraphPort's write operation: one node, its
// body, and one plain edge to the subject — three POSTs (design C32).
// The caller (Turn.Run) supplies only the record; every structural
// decision below is this adapter's alone (design §8.3).
func (c *Client) WriteRun(ctx context.Context, record loop.Record) (int64, error) {
	body, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("divoid: encode run record: %w", err)
	}

	id, err := c.createRunNode(ctx, c.runName(record))
	if err != nil {
		return 0, err
	}
	if err := c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), runContentType, body, nil); err != nil {
		return 0, fmt.Errorf("divoid: set run node content: %w", err)
	}
	if err := c.linkRunNode(ctx, id, record.Subject); err != nil {
		return 0, fmt.Errorf("divoid: link run node to subject: %w", err)
	}

	return id, nil
}

func (c *Client) createRunNode(ctx context.Context, name string) (int64, error) {
	reqBody, err := json.Marshal(createNodeRequest{Type: runNodeType, Name: name})
	if err != nil {
		return 0, fmt.Errorf("divoid: encode run node create: %w", err)
	}

	var resp createNodeResponse
	if err := c.post(ctx, "/api/nodes", "application/json", reqBody, &resp); err != nil {
		return 0, fmt.Errorf("divoid: create run node: %w", err)
	}
	return resp.ID, nil
}

// linkRunNode posts the bare target id as the body (design C32) — an
// undirected edge, per #7216: a plain link is correct here because
// nothing in the existing vocabulary names this relationship and coining
// a verb for a milestone's single edge is exactly the over-decoration
// #7216 warns against (design §8.3's written node contract).
func (c *Client) linkRunNode(ctx context.Context, id, target int64) error {
	body, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("divoid: encode link target: %w", err)
	}
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/links", id), "application/json", body, nil)
}

// runName is deterministic from the run: a fixed prefix, the timestamp,
// and a bounded prefix of the input (design §8.3), so a graph listing is
// legible without opening anything.
func (c *Client) runName(record loop.Record) string {
	return fmt.Sprintf("%s %s — %s", runNamePrefix, c.clock().UTC().Format(time.RFC3339), truncateRunes(record.Input, runNameInputRunes))
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
