package divoid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/telmengedar/processor/internal/redacturl"
)

const (
	substanceNodeFields = "id,type,name,contentType,content,substance"
	contentOnlyFields   = "id,content"
)

const (
	substancePath        = "/substance"
	substanceOp          = "replace"
	jsonPatchContentType = "application/json-patch+json"
)

// SubstanceNode is one node as the offline condensation pass reads it: the body to condense from, and the substance that may already stand on it.
type SubstanceNode struct {
	ID          int64
	Type        string
	Name        string
	ContentType string
	Content     string
	Substance   string
}

type patchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// NodeWithSubstance fetches one node's condensation projection by id, reporting absence as found=false rather than as an error.
func (c *Client) NodeWithSubstance(ctx context.Context, id int64) (SubstanceNode, bool, error) {
	r, found, err := c.projectedRow(ctx, id, substanceNodeFields)
	if err != nil || !found {
		return SubstanceNode{}, false, err
	}
	return SubstanceNode{
		ID:          r.ID,
		Type:        r.Type,
		Name:        r.Name,
		ContentType: r.ContentType,
		Content:     r.Content,
		Substance:   r.Substance,
	}, true, nil
}

// Content re-reads one node's body alone, which is what the pass compares against immediately before it writes.
func (c *Client) Content(ctx context.Context, id int64) (string, bool, error) {
	r, found, err := c.projectedRow(ctx, id, contentOnlyFields)
	if err != nil || !found {
		return "", false, err
	}
	return r.Content, true, nil
}

// SetSubstance replaces one node's substance, and is structurally incapable of touching its content or of creating or deleting a node.
func (c *Client) SetSubstance(ctx context.Context, id int64, substance string) error {
	if strings.TrimSpace(substance) == "" {
		return fmt.Errorf("divoid: refusing to write a blank substance to node %d", id)
	}

	body, err := json.Marshal([]patchOperation{{Op: substanceOp, Path: substancePath, Value: substance}})
	if err != nil {
		return fmt.Errorf("divoid: encode substance patch: %w", err)
	}

	if err := c.patch(ctx, fmt.Sprintf("%s/%d", nodesPath, id), jsonPatchContentType, body); err != nil {
		return fmt.Errorf("divoid: write substance to node %d: %w", id, err)
	}
	return nil
}

func (c *Client) projectedRow(ctx context.Context, id int64, fields string) (row, bool, error) {
	q := url.Values{}
	q.Set("id", strconv.FormatInt(id, 10))
	q.Set("fields", fields)

	var resp listingResponse
	if err := c.get(ctx, nodesPath, q, &resp); err != nil {
		return row{}, false, err
	}

	for _, r := range resp.Result {
		if r.ID == id {
			return r, true, nil
		}
	}
	return row{}, false, nil
}

func (c *Client) patch(ctx context.Context, path, contentType string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", redacturl.Error(err))
	}
	req.Header.Set("Content-Type", contentType)
	return c.send(req, nil)
}
