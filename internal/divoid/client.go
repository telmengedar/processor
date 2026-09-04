// Package divoid is the DiVoid API client implementing loop.GraphPort: the run's graph reads and write-back.
package divoid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
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

const (
	nodesPath = "/api/nodes"
	linksPath = "/api/nodes/links"
)

const linksPageSize = 500

// Client is a client for one specific external system, not a generic
// "graph" (#6836's naming rule).
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	clock func() time.Time

	logger *slog.Logger
}

// NewClient builds a Client against baseURL with apiKey, httpClient (nil for the default) and logger.
func NewClient(baseURL, apiKey string, httpClient *http.Client, logger *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		clock:      time.Now,
		httpClient: httpClient,
		logger:     logger,
	}
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

func (c *Client) client() *http.Client {
	if c.httpClient == nil {
		return defaultHTTPClient()
	}
	return c.httpClient
}

func (c *Client) log() *slog.Logger {
	if c.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return c.logger
}

func (c *Client) now() time.Time {
	if c.clock == nil {
		return time.Now()
	}
	return c.clock()
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

type linkRow struct {
	SourceID int64 `json:"sourceId"`
	TargetID int64 `json:"targetId"`
}

type linksResponse struct {
	Result   []linkRow `json:"result"`
	Continue *int      `json:"continue"`
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
	if err := c.get(ctx, nodesPath, q, &resp); err != nil {
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

// Recall returns up to limit candidates in the graph's own rank order, never re-sorted; an empty scope ranks the whole graph.
func (c *Client) Recall(ctx context.Context, query string, limit int, scope []int64) ([]loop.Candidate, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("count", strconv.Itoa(limit))
	q.Set("fields", candidateFields)
	for _, id := range scope {
		q.Add("linkedto", strconv.FormatInt(id, 10))
	}

	var resp listingResponse
	if err := c.get(ctx, nodesPath, q, &resp); err != nil {
		return nil, err
	}

	candidates := make([]loop.Candidate, len(resp.Result))
	for i, r := range resp.Result {
		candidates[i] = loop.Candidate{
			ID:           r.ID,
			Type:         r.Type,
			Name:         r.Name,
			Similarity:   r.Similarity,
			Content:      r.Content,
			SelfProduced: IsRunRecord(r.Type, r.Name),
		}
	}
	return candidates, nil
}

// Neighbours returns the other endpoint of every edge incident to id, ascending and deduplicated.
func (c *Client) Neighbours(ctx context.Context, id int64) ([]int64, error) {
	seen := make(map[int64]struct{})

	for cursor := 0; ; {
		q := url.Values{}
		q.Set("ids", strconv.FormatInt(id, 10))
		q.Set("count", strconv.Itoa(linksPageSize))
		if cursor > 0 {
			q.Set("continue", strconv.Itoa(cursor))
		}

		var resp linksResponse
		if err := c.get(ctx, linksPath, q, &resp); err != nil {
			return nil, err
		}

		for _, edge := range resp.Result {
			other := edge.SourceID
			if other == id {
				other = edge.TargetID
			}
			seen[other] = struct{}{}
		}

		if len(resp.Result) == 0 || resp.Continue == nil || *resp.Continue <= cursor {
			break
		}
		cursor = *resp.Continue
	}

	neighbours := make([]int64, 0, len(seen))
	for n := range seen {
		neighbours = append(neighbours, n)
	}
	slices.Sort(neighbours)

	return neighbours, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := c.baseURL + path + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client().Do(req)
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
	req.Header.Set("Content-Type", contentType)
	return c.send(req, out)
}

func (c *Client) remove(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", err)
	}
	return c.send(req, nil)
}

func (c *Client) send(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client().Do(req)
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
