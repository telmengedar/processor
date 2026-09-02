// Package divoid is the graph adapter: a client for the DiVoid API's node
// listing endpoint, used for the two read operations Unit A needs. It
// implements loop.GraphPort, which is declared in internal/loop and
// constructed here only as a value passed into main (design §5.2, §8.3).
package divoid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

// DefaultTimeout bounds a graph read when the caller does not supply its
// own *http.Client. http.DefaultClient carries no timeout, so a hung graph
// read would otherwise be bounded only by client disconnect (W-7).
const DefaultTimeout = 15 * time.Second

// Compile-time assertion that Client satisfies the port the loop declares.
var _ loop.GraphPort = (*Client)(nil)

// nodeFields and candidateFields are the response projections each
// operation needs (design C29, C21) and nothing more, so the graph does
// not serialize bytes nobody reads.
const (
	nodeFields      = "id,type,name,contentType,content"
	candidateFields = "id,type,name,similarity,content"
)

// Client is a client for one specific external system, not a generic
// "graph" (#6836's naming rule).
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	clock func() time.Time
}

// NewClient builds a Client against baseURL, authenticating with apiKey.
// httpClient is used verbatim when non-nil; otherwise a new *http.Client
// with DefaultTimeout applied is built (W-7) — never http.DefaultClient,
// which carries no timeout.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		clock:      time.Now,
		httpClient: httpClient,
	}
}

// row is the shape of one entry in the listing response's result array.
// One struct covers both operations' projections; fields absent from a
// given response's fields= projection simply decode to their zero value.
type row struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	ContentType string  `json:"contentType"`
	Similarity  float64 `json:"similarity"`
	Content     string  `json:"content"`
}

// listingResponse is the shape of GET /api/nodes, regardless of query.
type listingResponse struct {
	Result []row `json:"result"`
	Total  int   `json:"total"`
}

// Node fetches the subject node by id, with its content, over the listing
// route rather than the single-node route: a missing id returns 200 with
// an empty result (design C30), not 404 — the shape this method surfaces
// as found=false, never as an error. A caller that instead checked the
// status code on this route would silently accept a run with no subject.
//
// The listing route is queried by id but is not guaranteed to return
// exactly one row, so Node scans for the row whose id matches the
// requested id (W-4) rather than trusting Result[0] — anchoring on an
// arbitrary row would leave Record.Subject reporting the requested id
// while the anchor body came from a different node entirely.
func (c *Client) Node(ctx context.Context, id int64) (loop.Anchor, bool, error) {
	q := url.Values{}
	q.Set("id", strconv.FormatInt(id, 10))
	q.Set("fields", nodeFields)

	var resp listingResponse
	if err := c.get(ctx, q, &resp); err != nil {
		return loop.Anchor{}, false, err
	}

	for _, r := range resp.Result {
		if r.ID != id {
			continue
		}
		return loop.Anchor{
			ID:      r.ID,
			Type:    r.Type,
			Name:    r.Name,
			Content: r.Content,
		}, true, nil
	}
	return loop.Anchor{}, false, nil
}

// Recall runs one semantic query and returns up to limit candidates in the
// rank order the graph reported them. It does not re-sort.
func (c *Client) Recall(ctx context.Context, query string, limit int) ([]loop.Candidate, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("count", strconv.Itoa(limit))
	q.Set("fields", candidateFields)

	var resp listingResponse
	if err := c.get(ctx, q, &resp); err != nil {
		return nil, err
	}

	candidates := make([]loop.Candidate, len(resp.Result))
	for i, r := range resp.Result {
		candidates[i] = loop.Candidate{
			ID:         r.ID,
			Type:       r.Type,
			Name:       r.Name,
			Similarity: r.Similarity,
			Content:    r.Content,
		}
	}
	return candidates, nil
}

func (c *Client) get(ctx context.Context, query url.Values, out any) error {
	reqURL := c.baseURL + "/api/nodes?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("divoid: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("divoid: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("divoid: decode response: %w", err)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path, contentType string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("divoid: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("divoid: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("divoid: decode response: %w", err)
	}
	return nil
}
