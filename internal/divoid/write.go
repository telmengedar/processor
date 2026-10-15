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
const RunNamePrefix = loop.RunNamePrefix

const (
	runContentType = "text/markdown; charset=utf-8"

	runNameInputRunes = 80
)

const runContentFenceOpen = "\n---\n```json\n"
const runContentFenceClose = "\n```\n"

const (
	logWriteBackFailed  = "write-back failed"
	logRepairableOrphan = "repairable orphan"
	logUncollectedShell = "uncollected shell"
	logSummaryNotStored = "run record stored without its summary"
	logNoteFailed       = "note write-back failed"
)

// IsRunRecord reports whether a graph row is a run record this system wrote.
func IsRunRecord(nodeType, name string) bool {
	return nodeType == RunNodeType && strings.HasPrefix(name, RunNamePrefix)
}

type createNodeRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`

	RootNodeID int64 `json:"rootNodeId,omitempty"`

	SkipEmbedding bool `json:"skipEmbedding,omitempty"`
}

type createNodeResponse struct {
	ID int64 `json:"id"`
}

// WriteRun files the record as one node linked to the subject and reports how far it got.
func (c *Client) WriteRun(ctx context.Context, record loop.Record) loop.WriteReceipt {
	recordJSON, err := json.Marshal(record)
	if err != nil {
		c.log().Error(logWriteBackFailed, "subject", record.Subject, "error", fmt.Errorf("divoid: encode run record: %w", err))
		return loop.WriteReceipt{State: loop.NotStored}
	}

	at := c.now()
	account := loop.RenderSummary(record, at)
	content := ComposeRunContent(account, recordJSON)

	id, err := c.createRunNode(ctx, c.runName(record, at), runRoot(record))
	if err != nil {
		c.log().Error(logWriteBackFailed, "subject", record.Subject, "error", err)
		return loop.WriteReceipt{State: loop.NotStored}
	}

	if err := c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), runContentType, content, nil); err != nil {
		c.log().Error(logWriteBackFailed, "subject", record.Subject, "node", id, "error", fmt.Errorf("divoid: set run node content: %w", err))
		c.discardShell(ctx, id)
		return loop.WriteReceipt{State: loop.NotStored}
	}

	if err := c.SetSubstance(ctx, id, account); err != nil {
		c.log().Error(logSummaryNotStored, "node", id, "subject", record.Subject, "error", err)
	}

	if err := c.linkNode(ctx, id, record.Subject); err != nil {
		c.log().Error(logRepairableOrphan, "node", id, "subject", record.Subject, "error", fmt.Errorf("divoid: link run node to subject: %w", err))
		return loop.WriteReceipt{State: loop.Unlinked, NodeID: id}
	}

	return loop.WriteReceipt{State: loop.Stored, NodeID: id}
}

// ComposeRunContent composes account and record exactly as WriteRun's own composition does.
func ComposeRunContent(account string, record []byte) []byte {
	var b strings.Builder
	b.WriteString(account)
	b.WriteString(runContentFenceOpen)
	b.Write(record)
	b.WriteString(runContentFenceClose)
	return []byte(b.String())
}

// SetRunContent posts content to an existing run node with WriteRun's own content type.
func (c *Client) SetRunContent(ctx context.Context, id int64, content []byte) error {
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), runContentType, content, nil)
}

func (c *Client) discardShell(ctx context.Context, id int64) {
	if err := c.remove(ctx, fmt.Sprintf("/api/nodes/%d", id)); err != nil {
		c.log().Error(logUncollectedShell, "node", id, "error", err)
	}
}

func (c *Client) createRunNode(ctx context.Context, name string, root int64) (int64, error) {
	return c.createNode(ctx, RunNodeType, name, root, true, "run node")
}

func (c *Client) createNode(ctx context.Context, nodeType, name string, root int64, skipEmbedding bool, what string) (int64, error) {
	reqBody, err := json.Marshal(createNodeRequest{Type: nodeType, Name: name, RootNodeID: root, SkipEmbedding: skipEmbedding})
	if err != nil {
		return 0, fmt.Errorf("divoid: encode %s create: %w", what, err)
	}

	var resp createNodeResponse
	if err := c.post(ctx, "/api/nodes", "application/json", reqBody, &resp); err != nil {
		return 0, fmt.Errorf("divoid: create %s: %w", what, err)
	}
	return resp.ID, nil
}

// WriteNote files the note as one node with its text as content, linked to the subject and, when the request names one, to a second node, and reports how far it got.
func (c *Client) WriteNote(ctx context.Context, request loop.NoteRequest) loop.WriteReceipt {
	id, err := c.createNode(ctx, request.Type, request.Name, request.Root, false, "note node")
	if err != nil {
		c.log().Error(logNoteFailed, "subject", request.Subject, "error", err)
		return loop.WriteReceipt{State: loop.NotStored}
	}

	if err := c.post(ctx, fmt.Sprintf("/api/nodes/%d/content", id), runContentType, []byte(request.Text), nil); err != nil {
		c.log().Error(logNoteFailed, "subject", request.Subject, "node", id, "error", fmt.Errorf("divoid: set note node content: %w", err))
		c.discardShell(context.WithoutCancel(ctx), id)
		return loop.WriteReceipt{State: loop.NotStored}
	}

	targets := []int64{request.Subject}
	if request.About > 0 {
		targets = append(targets, request.About)
	}

	state := loop.Stored
	for _, target := range targets[:min(len(targets), loop.MaxNoteEdges)] {
		if err := c.linkNode(ctx, id, target); err != nil {
			c.log().Error(logRepairableOrphan, "node", id, "target", target, "error", fmt.Errorf("divoid: link note node: %w", err))
			state = loop.Unlinked
		}
	}
	return loop.WriteReceipt{State: state, NodeID: id}
}

func runRoot(record loop.Record) int64 {
	if record.Perspective == nil || record.Perspective.Root <= 0 {
		return 0
	}
	return record.Perspective.Root
}

func (c *Client) linkNode(ctx context.Context, id, target int64) error {
	body, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("divoid: encode link target: %w", err)
	}
	return c.post(ctx, fmt.Sprintf("/api/nodes/%d/links", id), "application/json", body, nil)
}

func (c *Client) runName(record loop.Record, at time.Time) string {
	return fmt.Sprintf("%s %s — %s", RunNamePrefix, at.UTC().Format(time.RFC3339), loop.TruncateRunes(record.Input, runNameInputRunes))
}
