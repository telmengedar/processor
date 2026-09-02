package divoid

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	runNodeType    = "session-log"
	runNamePrefix  = "processor-run"
	runContentType = "application/json"

	runNameInputRunes = 80
)

type createNodeRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type createNodeResponse struct {
	ID int64 `json:"id"`
}

// WriteRun creates the run node, sets its body and links it to the subject.
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

func (c *Client) linkRunNode(ctx context.Context, id, target int64) error {
	body, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("divoid: encode link target: %w", err)
	}
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/links", id), "application/json", body, nil)
}

func (c *Client) runName(record loop.Record) string {
	return fmt.Sprintf("%s %s — %s", runNamePrefix, c.now().UTC().Format(time.RFC3339), truncateRunes(record.Input, runNameInputRunes))
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
