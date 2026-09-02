package divoid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestNodeConstructsTheListingQueryWithIDAndFields(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery, gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", srv.Client())
	if _, _, err := c.Node(context.Background(), 42); err != nil {
		t.Fatalf("Node: %v", err)
	}

	const wantPath = "/api/nodes"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if q.Get("id") != "42" {
		t.Fatalf("query id = %q, want %q", q.Get("id"), "42")
	}
	if q.Get("fields") == "" {
		t.Fatal("query fields is empty, want a fields projection")
	}

	const wantAuth = "Bearer test-key"
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	const wantAccept = "application/json"
	if gotAccept != wantAccept {
		t.Fatalf("Accept = %q, want %q", gotAccept, wantAccept)
	}
}

// TestNodeReturnsNotFoundOnEmptyResultWithStatus200 pins design C30: a
// missing id via the listing form returns 200 with an empty result, never
// 404. A status-code-based not-found check would pass this test wrongly
// (it would never inspect the body) — TestNodeOnNon200TreatsItAsAnError
// below is the mutation-style counterpart that shows the failure such a
// check would miss.
func TestNodeReturnsNotFoundOnEmptyResultWithStatus200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	_, found, err := c.Node(context.Background(), 99999999)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if found {
		t.Fatal("Node reported found=true for an empty result, want false")
	}
}

func TestNodeDecodesAFoundResultIncludingContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 42, "type": "documentation", "name": "Vision", "contentType": "text/markdown", "content": "the body text"},
			},
			"total": 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	anchor, found, err := c.Node(context.Background(), 42)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if !found {
		t.Fatal("Node reported found=false for a non-empty result, want true")
	}
	if anchor.ID != 42 || anchor.Type != "documentation" || anchor.Name != "Vision" || anchor.Content != "the body text" {
		t.Fatalf("anchor = %+v, unexpected", anchor)
	}
}

// TestNodeOnNon200TreatsItAsAnErrorNeverAsNotFound is the mutation check
// for the C30 discrimination: a 401 must surface as an error (which Turn
// maps to graph_unavailable/502), never as found=false (which Turn would
// map to subject_not_found/404). A status-code-based not-found check would
// fail this test.
func TestNodeOnNon200TreatsItAsAnErrorNeverAsNotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"authorization_invalidtoken","text":"API key not recognised"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-key", srv.Client())
	_, found, err := c.Node(context.Background(), 42)
	if err == nil {
		t.Fatal("Node returned nil error for a 401 response, want an error")
	}
	if found {
		t.Fatal("Node reported found=true on a 401 response")
	}
}

func TestRecallConstructsTheQueryWithTextAndCount(t *testing.T) {
	t.Parallel()

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	// Deliberately not already lowercase/trimmed/single-spaced (CF-1): a
	// fixture that already satisfies those normalizations can't fail when
	// the adapter silently applies one.
	const wantQueryText = "  Why DOES   the Assembler ignore SCOPE?  "
	if _, err := c.Recall(context.Background(), wantQueryText, 20); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if q.Get("query") != wantQueryText {
		t.Fatalf("query = %q, want the input verbatim %q", q.Get("query"), wantQueryText)
	}
	if q.Get("count") != "20" {
		t.Fatalf("count = %q, want %q", q.Get("count"), "20")
	}

	// CF-2: assert the exact key set, not merely that query/count are
	// present. A key set assertion is what catches an adapter that adds a
	// scope filter (e.g. type=documentation) alongside them.
	wantKeys := []string{"query", "count", "fields"}
	if len(q) != len(wantKeys) {
		t.Fatalf("query has keys %v, want exactly %v", keysOf(q), wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := q[k]; !ok {
			t.Fatalf("query is missing key %q, got keys %v", k, keysOf(q))
		}
	}
}

func keysOf(v url.Values) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	return keys
}

// TestRecallReturnsEveryCandidateTheServerSentUnfiltered pins CF-2's other
// half: Recall must return every row the server sent, in the order sent —
// no scope filter, no similarity floor, no truncation of the result set
// independent of the requested limit (design R2).
func TestRecallReturnsEveryCandidateTheServerSentUnfiltered(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 1, "type": "t", "name": "n1", "similarity": 0.90, "content": "c1"},
				{"id": 2, "type": "t", "name": "n2", "similarity": 0.30, "content": "c2"},
				{"id": 3, "type": "t", "name": "n3", "similarity": 0.05, "content": "c3"},
				{"id": 4, "type": "t", "name": "n4", "similarity": 0.01, "content": "c4"},
				{"id": 5, "type": "t", "name": "n5", "similarity": 0.001, "content": "c5"},
			},
			"total": 5,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	got, err := c.Recall(context.Background(), "q", 20)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	wantIDs := []int64{1, 2, 3, 4, 5}
	if len(got) != len(wantIDs) {
		t.Fatalf("got %d candidates, want %d — every row the server sent, unfiltered by similarity and untruncated", len(got), len(wantIDs))
	}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Fatalf("candidate[%d].ID = %d, want %d", i, got[i].ID, want)
		}
	}
}

// TestRecallPreservesReturnedOrderWithoutResorting pins design §8.3: "Rank
// order as returned. The adapter does not re-sort."
func TestRecallPreservesReturnedOrderWithoutResorting(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 300, "type": "t", "name": "first-by-rank", "similarity": 0.9, "content": "c1"},
				{"id": 100, "type": "t", "name": "second-by-rank", "similarity": 0.5, "content": "c2"},
				{"id": 200, "type": "t", "name": "third-by-rank", "similarity": 0.1, "content": "c3"},
			},
			"total": 3,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	got, err := c.Recall(context.Background(), "q", 20)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	wantIDs := []int64{300, 100, 200}
	if len(got) != len(wantIDs) {
		t.Fatalf("got %d candidates, want %d", len(got), len(wantIDs))
	}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Fatalf("candidate[%d].ID = %d, want %d — Recall must not re-sort", i, got[i].ID, want)
		}
	}
}

func TestRecallDecodesSimilarityAndContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 7, "type": "task", "name": "Foo", "similarity": 0.664142, "content": "foo body"},
			},
			"total": 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	got, err := c.Recall(context.Background(), "q", 20)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if got[0].ID != 7 || got[0].Type != "task" || got[0].Name != "Foo" || got[0].Content != "foo body" {
		t.Fatalf("candidate = %+v, unexpected", got[0])
	}
	const wantSimilarity = 0.664142
	if got[0].Similarity != wantSimilarity {
		t.Fatalf("similarity = %v, want %v", got[0].Similarity, wantSimilarity)
	}
}

func TestRecallOnNon200ReturnsAnError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"internal"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	if _, err := c.Recall(context.Background(), "q", 20); err == nil {
		t.Fatal("Recall returned nil error for a 500 response, want an error")
	}
}

func TestNodeOnUnreachableHostReturnsAnError(t *testing.T) {
	t.Parallel()

	c := NewClient("http://127.0.0.1:1", "k", nil)
	if _, _, err := c.Node(context.Background(), 1); err == nil {
		t.Fatal("Node returned nil error against an unreachable host, want an error")
	}
}

// TestNodeAnchorsOnTheRowMatchingTheRequestedIDNotResultZero pins W-4: on a
// multi-row response, Node must anchor on the row whose id matches the
// requested id, not on Result[0]. The matching row sits in the middle —
// neither first nor last — so this also catches an "always take the last
// row" variant, not just "always take the first".
func TestNodeAnchorsOnTheRowMatchingTheRequestedIDNotResultZero(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 999, "type": "wrong", "name": "Not This One", "content": "wrong body"},
				{"id": 42, "type": "documentation", "name": "Vision", "content": "the body text"},
				{"id": 777, "type": "wrong", "name": "Nor This One", "content": "also wrong"},
			},
			"total": 3,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	anchor, found, err := c.Node(context.Background(), 42)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if !found {
		t.Fatal("Node reported found=false when a matching row was present, want true")
	}
	if anchor.ID != 42 || anchor.Name != "Vision" {
		t.Fatalf("anchor = %+v, want the row whose id matches the requested id (42), not an arbitrary row", anchor)
	}
}

// TestNodeReportsNotFoundWhenNoRowMatchesTheRequestedID is W-4's
// complement: a non-empty result that contains no row for the requested id
// must not be silently accepted as a match.
func TestNodeReportsNotFoundWhenNoRowMatchesTheRequestedID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 999, "type": "wrong", "name": "Not This One", "content": "wrong body"},
			},
			"total": 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client())
	_, found, err := c.Node(context.Background(), 42)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if found {
		t.Fatal("Node reported found=true for a result with no row matching the requested id")
	}
}

// TestNewClientDefaultsToATimeoutBoundHTTPClientWhenNoneIsSupplied pins
// W-7: http.DefaultClient has no timeout, so a hung graph read would be
// bounded only by client disconnect. A nil httpClient must still produce a
// bounded one.
func TestNewClientDefaultsToATimeoutBoundHTTPClientWhenNoneIsSupplied(t *testing.T) {
	t.Parallel()

	c := NewClient("http://example.invalid", "k", nil)
	if c.httpClient == http.DefaultClient {
		t.Fatal("NewClient(nil) used http.DefaultClient, which has no timeout")
	}
	if c.httpClient.Timeout <= 0 {
		t.Fatalf("httpClient.Timeout = %v, want a positive timeout", c.httpClient.Timeout)
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Fatalf("httpClient.Timeout = %v, want DefaultTimeout (%v)", c.httpClient.Timeout, DefaultTimeout)
	}
}

// TestDefaultTimeoutIsFifteenSeconds pins DefaultTimeout's magnitude (W-13)
// against a literal, not against the DefaultTimeout constant itself — a
// comparison of the constant to itself would move with a mutation to its
// value and catch nothing (the same both-sides-move anti-pattern CF-4
// named at the wire layer). main no longer sets its own copy of the
// timeout (W-13): it passes a nil httpClient and relies on this single
// value, so this is now the one place the magnitude is reachable and worth
// pinning.
func TestDefaultTimeoutIsFifteenSeconds(t *testing.T) {
	t.Parallel()

	const want = 15 * time.Second
	if DefaultTimeout != want {
		t.Fatalf("DefaultTimeout = %v, want %v", DefaultTimeout, want)
	}
}
