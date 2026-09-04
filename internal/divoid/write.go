package divoid

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

// RunNodeType is the graph node type every run record is filed as.
const RunNodeType = "session-log"

// RunNamePrefix opens the name of every run node the loop writes.
const RunNamePrefix = "processor-run"

const (
	runContentType = "application/json"

	runNameInputRunes = 80
)

const (
	logWriteBackFailed  = "write-back failed"
	logRepairableOrphan = "repairable orphan"
	logUncollectedShell = "uncollected shell"
)

// IsRunRecord reports whether a graph row is a run record this system wrote.
func IsRunRecord(nodeType, name string) bool {
	return nodeType == RunNodeType && strings.HasPrefix(name, RunNamePrefix)
}

type createNodeRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type createNodeResponse struct {
	ID int64 `json:"id"`
}

// WriteRun files the record as one node linked to the subject and reports how far it got.
func (c *Client) WriteRun(ctx context.Context, record loop.Record) loop.WriteReceipt {
	body, err := json.Marshal(record)
	if err != nil {
		c.log().Error(logWriteBackFailed, "subject", record.Subject, "error", fmt.Errorf("divoid: encode run record: %w", err))
		return loop.WriteReceipt{State: loop.NotStored}
	}

	id, err := c.createRunNode(ctx, c.runName(record))
	if err != nil {
		c.log().Error(logWriteBackFailed, "subject", record.Subject, "error", err)
		return loop.WriteReceipt{State: loop.NotStored}
	}

	if err := c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), runContentType, body, nil); err != nil {
		c.log().Error(logWriteBackFailed, "subject", record.Subject, "node", id, "error", fmt.Errorf("divoid: set run node content: %w", err))
		c.discardShell(ctx, id)
		return loop.WriteReceipt{State: loop.NotStored}
	}

	if err := c.linkRunNode(ctx, id, record.Subject); err != nil {
		c.log().Error(logRepairableOrphan, "node", id, "subject", record.Subject, "error", fmt.Errorf("divoid: link run node to subject: %w", err))
		return loop.WriteReceipt{State: loop.Unlinked, NodeID: id}
	}

	return loop.WriteReceipt{State: loop.Stored, NodeID: id}
}

func (c *Client) discardShell(ctx context.Context, id int64) {
	if err := c.remove(ctx, fmt.Sprintf("/api/nodes/%d", id)); err != nil {
		c.log().Error(logUncollectedShell, "node", id, "error", err)
	}
}

func (c *Client) createRunNode(ctx context.Context, name string) (int64, error) {
	reqBody, err := json.Marshal(createNodeRequest{Type: RunNodeType, Name: name})
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
	return fmt.Sprintf("%s %s — %s", RunNamePrefix, c.now().UTC().Format(time.RFC3339), truncateRunes(record.Input, runNameInputRunes))
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
