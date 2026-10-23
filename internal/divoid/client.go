// Package divoid is the DiVoid API client implementing loop.GraphPort: the run's graph reads and write-back.
package divoid

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/redacturl"
)

// DefaultTimeout bounds a graph read when the caller does not supply its
// own *http.Client. http.DefaultClient carries no timeout, so a hung graph
// read would otherwise be bounded only by client disconnect (W-7).
const DefaultTimeout = 15 * time.Second

// Compile-time assertion that Client satisfies the port the loop declares.
var _ loop.GraphPort = (*Client)(nil)

const (
	nodeFields      = "id,type,name,content,substance,rootNodeId"
	candidateFields = "id,type,name,similarity,content,substance"
)

const (
	nodesPath = "/api/nodes"

	queriesParam       = "queries"
	excludeIDParam     = "excludeId"
	excludeNameParam   = "excludeName"
	updatedFromParam   = "updatedFrom"
	updatedToParam     = "updatedTo"
	minSimilarityParam = "minSimilarity"
)

var errNoQuery = errors.New("divoid: recall needs at least one query")

var errNoLimit = errors.New("divoid: recall needs a positive limit")

const errorBodyBytes = 4096

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
	Substance   string  `json:"substance"`
	RootNodeID  *int64  `json:"rootNodeId"`
}

// listingResponse is the shape of GET /api/nodes, regardless of query.
type listingResponse struct {
	Result   []row `json:"result"`
	Total    int   `json:"total"`
	Continue *int  `json:"continue"`
}

type rejectedError struct {
	status int
	body   string
}

func (e *rejectedError) Error() string {
	return fmt.Sprintf("divoid: unexpected status %d: %s", e.status, e.body)
}

func (e *rejectedError) Is(target error) bool {
	return target == loop.ErrGraphRejected
}

func appendFilter(q url.Values, filter loop.Filter, recall bool) {
	for _, name := range slices.Sorted(maps.Keys(filter)) {
		switch {
		case name == minSimilarityParam && !recall:
			continue
		case name == updatedFromParam || name == updatedToParam:
			q[name] = slices.Clone(filter[name])
		default:
			for _, value := range filter[name] {
				q.Add(name, value)
			}
		}
	}
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
func (c *Client) Node(ctx context.Context, id int64, filter loop.Filter) (loop.Anchor, bool, error) {
	q := url.Values{}
	appendFilter(q, filter, false)
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
		anchor := loop.Anchor{
			ID:        r.ID,
			Type:      r.Type,
			Name:      r.Name,
			Content:   r.Content,
			Substance: r.Substance,
		}
		if r.RootNodeID != nil {
			anchor.RootNodeID = *r.RootNodeID
		}
		return anchor, true, nil
	}
	return loop.Anchor{}, false, nil
}

// Recall returns the graph's top candidates for the request in its own rank order, never re-sorted; several queries are one request ranked by their mean similarity, and a request without a query or a positive limit fails before any request is sent.
func (c *Client) Recall(ctx context.Context, request loop.RecallRequest) ([]loop.Candidate, error) {
	queries, window := request.Queries, request.Window
	if len(queries) == 0 {
		return nil, errNoQuery
	}
	if request.Limit <= 0 {
		return nil, errNoLimit
	}

	q := url.Values{}
	if len(queries) == 1 {
		q.Set("query", queries[0])
	} else {
		q[queriesParam] = queries
	}
	q.Set("count", strconv.Itoa(request.Limit))
	q.Set("fields", candidateFields)
	directSubject := int64(0)
	if values := request.Filter["linkedto"]; request.NoteContextSubject > 0 && len(values) == 1 && values[0] == strconv.FormatInt(request.NoteContextSubject, 10) {
		directSubject = request.NoteContextSubject
		q.Set("fields", candidateFields+",rootNodeId")
	}
	for _, id := range request.Exclude {
		q.Add(excludeIDParam, strconv.FormatInt(id, 10))
	}
	q.Set(excludeNameParam, RunNamePrefix+"%")
	appendFilter(q, request.Filter, true)
	if !window.IsZero() {
		if _, set := q[updatedFromParam]; !set {
			q.Set(updatedFromParam, window.From.Format(time.RFC3339))
		}
		if _, set := q[updatedToParam]; !set {
			q.Set(updatedToParam, window.To.Format(time.RFC3339))
		}
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
			Substance:    r.Substance,
			SelfProduced: IsRunRecord(r.Type, r.Name),
		}
		if directSubject > 0 {
			candidates[i].DirectSubjectID = directSubject
			if r.RootNodeID != nil {
				candidates[i].RootNodeID = *r.RootNodeID
			}
		}
	}
	return candidates, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := c.baseURL + path + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", redacturl.Error(err))
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("divoid: request failed: %w", redacturl.Error(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyBytes))
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestURITooLong {
			return &rejectedError{status: resp.StatusCode, body: string(body)}
		}
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
		return fmt.Errorf("divoid: build request: %w", redacturl.Error(err))
	}
	req.Header.Set("Content-Type", contentType)
	return c.send(req, out)
}

func (c *Client) remove(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("divoid: build request: %w", redacturl.Error(err))
	}
	return c.send(req, nil)
}

func (c *Client) send(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("divoid: request failed: %w", redacturl.Error(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyBytes))
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
