package divoid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
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

	c := NewClient(srv.URL, "test-key", srv.Client(), testLogger())
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
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

	c := NewClient(srv.URL, "bad-key", srv.Client(), testLogger())
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	// Deliberately not already lowercase/trimmed/single-spaced (CF-1): a
	// fixture that already satisfies those normalizations can't fail when
	// the adapter silently applies one.
	const wantQueryText = "  Why DOES   the Assembler ignore SCOPE?  "
	if _, err := c.Recall(context.Background(), wantQueryText, 20, nil); err != nil {
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Recall(context.Background(), "q", 20, nil)
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

func TestRecallMarksOnlyTheRunRecordsThisClientWrote(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 1, "type": RunNodeType, "name": RunNamePrefix + " 2026-09-04T10:00:00Z what changed", "similarity": 0.9, "content": "a"},
				{"id": 2, "type": RunNodeType, "name": "a session log another agent wrote", "similarity": 0.8, "content": "b"},
				{"id": 3, "type": "documentation", "name": RunNamePrefix + " lookalike", "similarity": 0.7, "content": "c"},
			},
			"total": 3,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Recall(context.Background(), "q", 20, nil)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	want := []bool{true, false, false}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].SelfProduced != w {
			t.Fatalf("candidate[%d] (#%d %q %q) SelfProduced = %v, want %v: the node type and the name prefix must both match",
				i, got[i].ID, got[i].Type, got[i].Name, got[i].SelfProduced, w)
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Recall(context.Background(), "q", 20, nil)
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Recall(context.Background(), "q", 20, nil)
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	if _, err := c.Recall(context.Background(), "q", 20, nil); err == nil {
		t.Fatal("Recall returned nil error for a 500 response, want an error")
	}
}

func TestNodeOnUnreachableHostReturnsAnError(t *testing.T) {
	t.Parallel()

	c := NewClient("http://127.0.0.1:1", "k", nil, testLogger())
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
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

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
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

	c := NewClient("http://example.invalid", "k", nil, testLogger())
	if c.httpClient == http.DefaultClient {
		t.Fatal("NewClient(nil, testLogger()) used http.DefaultClient, which has no timeout")
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

func TestRecallSendsOneLinkedToParameterPerScopeIDBecauseAnUnknownScopeKeyIsSilentlyIgnored(t *testing.T) {
	t.Parallel()

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	if _, err := c.Recall(context.Background(), "q", 20, []int64{7, 11, 13}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}

	want := []string{"7", "11", "13"}
	got := q["linkedto"]
	if len(got) != len(want) {
		t.Fatalf("linkedto = %v, want one value per scope id %v: the graph ignores a scope key it does not know and answers with the whole-graph ranking, so a wrong key or a collapsed list is an unscoped search that reads as a scoped one", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("linkedto[%d] = %q, want scope id %q", i, got[i], id)
		}
	}

	wantKeys := []string{"query", "count", "fields", "linkedto"}
	if len(q) != len(wantKeys) {
		t.Fatalf("query has keys %v, want exactly %v", keysOf(q), wantKeys)
	}
}

func TestRecallSendsNoScopeKeyAtAllForAnEmptyScopeSoTheRankingStaysWholeGraph(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		scope []int64
	}{
		{name: "nil", scope: nil},
		{name: "empty", scope: []int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result":[],"total":0}`))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "k", srv.Client(), testLogger())
			if _, err := c.Recall(context.Background(), "q", 20, tc.scope); err != nil {
				t.Fatalf("Recall: %v", err)
			}

			q, err := url.ParseQuery(gotQuery)
			if err != nil {
				t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
			}
			if _, present := q["linkedto"]; present {
				t.Fatalf("linkedto = %v for an empty scope, want the key absent: the graph reads an empty scope value as no filter and returns the whole-graph ranking, so emitting the key claims a restriction the answer does not have", q["linkedto"])
			}
		})
	}
}

func TestNeighboursReadsTheFarEndpointOfEveryEdgeInEitherDirectionCountingARepeatedPairOnce(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"sourceId": 500, "targetId": 42},
				{"sourceId": 42, "targetId": 9},
				{"sourceId": 42, "targetId": 9},
				{"sourceId": 77, "targetId": 42},
			},
			"total": 4,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Neighbours(context.Background(), 42)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	want := []int64{9, 77, 500}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbours = %v, want %v: node 9 arrives as a target and nodes 77 and 500 as sources, so a reader that takes one endpoint by position drops half the neighbourhood, and the repeated 42-9 pair must widen the scope by one entry rather than two", got, want)
	}
}

func TestNeighboursReturnsTheScopeAscendingSoOneEdgeSetAlwaysYieldsOneScope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"sourceId": 42, "targetId": 900},
				{"sourceId": 42, "targetId": 3},
				{"sourceId": 42, "targetId": 71},
				{"sourceId": 42, "targetId": 12},
				{"sourceId": 42, "targetId": 500},
				{"sourceId": 42, "targetId": 8},
				{"sourceId": 42, "targetId": 44},
			},
			"total": 7,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Neighbours(context.Background(), 42)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	want := []int64{3, 8, 12, 44, 71, 500, 900}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbours = %v, want %v ascending: the rows arrive unordered and this slice becomes the linkedto list of a recall, so an arrival-ordered scope makes the same graph produce a different request from one run to the next", got, want)
	}
}

func TestNeighboursFollowsTheContinueCursorSoAPagedNeighbourhoodArrivesWhole(t *testing.T) {
	t.Parallel()

	var gotCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCursors = append(gotCursors, r.URL.Query().Get("continue"))
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("continue") == "" {
			_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":8}],"total":2,"continue":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":64}],"total":2}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Neighbours(context.Background(), 42)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	want := []int64{8, 64}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbours = %v, want %v: node 64 is only on the second page, so a single read silently returns a neighbourhood smaller than the node has and the narrowed scope is indistinguishable from a node with fewer edges", got, want)
	}
	if len(gotCursors) != 2 || gotCursors[1] != "1" {
		t.Fatalf("continue values sent = %v, want the first page unset then the cursor the first page reported (\"1\")", gotCursors)
	}
}

func TestNeighboursStopsWhenTheCursorRepeatsRatherThanRereadingOnePageForever(t *testing.T) {
	t.Parallel()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"text":"the client re-read a page whose cursor had not advanced"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":8}],"total":1,"continue":1}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Neighbours(context.Background(), 42)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	want := []int64{8}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbours = %v, want %v", got, want)
	}
	if calls != 2 {
		t.Fatalf("the links route was read %d times, want 2: the second page reports the cursor the first page already served, and a client that trusts the cursor rather than its own progress reads that page until the context ends", calls)
	}
}

func TestNeighboursConstructsTheLinksQueryWithTheNodeIDAndAnExplicitPageSize(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	if _, err := c.Neighbours(context.Background(), 42); err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	const wantPath = "/api/nodes/links"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q — the edges of a node are not on the node listing route", gotPath, wantPath)
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if q.Get("ids") != "42" {
		t.Fatalf("ids = %q, want the requested node %q", q.Get("ids"), "42")
	}
	if q.Get("count") != "500" {
		t.Fatalf("count = %q, want an explicitly requested page size: relying on the route's own default makes the size of a neighbourhood a property of the server's configuration rather than of this client", q.Get("count"))
	}

	wantKeys := []string{"ids", "count"}
	if len(q) != len(wantKeys) {
		t.Fatalf("query has keys %v, want exactly %v on the first page", keysOf(q), wantKeys)
	}
}

func TestNeighboursStopsAndWarnsWhenAPageIsEmptyThoughTheGraphOffersAnotherCursor(t *testing.T) {
	t.Parallel()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		switch r.URL.Query().Get("continue") {
		case "":
			_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":8}],"total":2,"continue":1}`))
		case "1":
			_, _ = w.Write([]byte(`{"result":[],"total":2,"continue":2}`))
		default:
			_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":64}],"total":2}`))
		}
	}))
	defer srv.Close()

	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)
	got, err := c.Neighbours(context.Background(), 42)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	if calls != 2 {
		t.Fatalf("the links route was read %d times, want 2: an empty page carrying a cursor is the graph declining to serve the rest, and a client that keeps asking walks the cursor forward over empty pages for as long as the graph keeps offering one", calls)
	}
	want := []int64{8}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbours = %v, want %v", got, want)
	}
	if !strings.Contains(log.String(), logPartialNeighbourhood) {
		t.Fatalf("the operator log was %q, want it to carry %q: the caller receives a neighbourhood the graph never finished serving, and at step 3 that scope silently narrows a recall rather than failing it", log.String(), logPartialNeighbourhood)
	}
}

func TestNeighboursWarnsWhenItStopsOnACursorThatDidNotAdvance(t *testing.T) {
	t.Parallel()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"text":"the client kept asking for a cursor that never advanced"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":8}],"total":9,"continue":1}`))
	}))
	defer srv.Close()

	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)
	if _, err := c.Neighbours(context.Background(), 42); err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	if !strings.Contains(log.String(), logPartialNeighbourhood) {
		t.Fatalf("the operator log was %q, want it to carry %q: stopping on a repeated cursor is the right move and it still returns fewer neighbours than the graph holds, so the only thing separating it from a complete read is this line", log.String(), logPartialNeighbourhood)
	}
}

func TestNeighboursSaysNothingWhenTheGraphItselfReportsTheLastPage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("continue") == "" {
			_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":8}],"total":2,"continue":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":[{"sourceId":42,"targetId":64}],"total":2}`))
	}))
	defer srv.Close()

	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)
	got, err := c.Neighbours(context.Background(), 42)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	if !slices.Equal(got, []int64{8, 64}) {
		t.Fatalf("Neighbours = %v, want [8 64]", got)
	}
	if strings.Contains(log.String(), logPartialNeighbourhood) {
		t.Fatalf("a complete two-page read logged %q: every paged neighbourhood ends on a page with no cursor, so warning there warns on every ordinary read and the line stops meaning anything on the one read where it matters", logPartialNeighbourhood)
	}
}
