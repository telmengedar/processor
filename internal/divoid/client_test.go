package divoid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

type recordedRequests struct {
	requests int
	path     string
	query    string
}

func recordingClient(t *testing.T) (*Client, *recordedRequests) {
	t.Helper()

	rec := &recordedRequests{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.requests++
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	t.Cleanup(srv.Close)

	return NewClient(srv.URL, "k", srv.Client(), testLogger()), rec
}

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
				{"id": 42, "type": "documentation", "name": "Vision", "contentType": "text/markdown", "content": "the body text", "substance": "the condensed text"},
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
	if anchor.Substance != "the condensed text" {
		t.Fatalf("anchor.Substance = %q, want the substance the graph returned", anchor.Substance)
	}
}

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

func TestRecallSendsTheQueryTextVerbatimWithCountAndFieldsAndNoOtherKey(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	const wantQueryText = "  Why DOES   the Assembler ignore SCOPE?  "
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{wantQueryText}, Limit: 6}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, err := url.ParseQuery(rec.query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", rec.query, err)
	}
	if q.Get("query") != wantQueryText {
		t.Fatalf("query = %q, want the input verbatim %q", q.Get("query"), wantQueryText)
	}
	if q.Get("count") != "6" {
		t.Fatalf("count = %q, want %q", q.Get("count"), "6")
	}
	if _, present := q["queries"]; present {
		t.Fatalf("a single query sent the queries key %v, want query alone: the graph takes query for one and queries for two or more", q["queries"])
	}

	wantKeys := []string{"query", "count", "fields", "excludeName"}
	if len(q) != len(wantKeys) {
		t.Fatalf("query has keys %v, want exactly %v", keysOf(q), wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := q[k]; !ok {
			t.Fatalf("query is missing key %q, got keys %v", k, keysOf(q))
		}
	}
}

func TestAZeroWindowSendsTheSameQueryStringAsBeforeTheWindowExisted(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	want := url.Values{"query": {"q"}, "count": {"6"}, "fields": {candidateFields}, "excludeName": {"processor-run%"}}.Encode()
	if rec.query != want {
		t.Fatalf("a zero window sent %q, want the encoded query string byte-identical to %q: a zero window must not acquire updatedFrom, updatedTo, an empty string, or any sentinel value", rec.query, want)
	}
}

func TestANonZeroWindowSendsBothUpdatedFromAndUpdatedTo(t *testing.T) {
	t.Parallel()

	window := loop.UpdateWindow{
		From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.FixedZone("test+02", 2*60*60)),
		To:   time.Date(2026, 9, 13, 0, 0, 0, 0, time.FixedZone("test+02", 2*60*60)),
	}

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Window: window}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, err := url.ParseQuery(rec.query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", rec.query, err)
	}

	wantFrom := window.From.Format(time.RFC3339)
	wantTo := window.To.Format(time.RFC3339)
	if q.Get("updatedFrom") != wantFrom {
		t.Fatalf("updatedFrom = %q, want %q", q.Get("updatedFrom"), wantFrom)
	}
	if q.Get("updatedTo") != wantTo {
		t.Fatalf("updatedTo = %q, want %q", q.Get("updatedTo"), wantTo)
	}
}

func keysOf(v url.Values) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	return keys
}

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
	got, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
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
	got, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
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
	got, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
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
	got, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
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

func TestRecallRequestsSubstanceInTheFieldsProjection(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, err := url.ParseQuery(rec.query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", rec.query, err)
	}
	fields := strings.Split(q.Get("fields"), ",")
	if !slices.Contains(fields, "substance") {
		t.Fatalf("fields = %q, want it to include %q — a candidate that never requests substance can never carry one", q.Get("fields"), "substance")
	}
}

func TestRecallDecodesSubstanceWhenPresentAndLeavesItEmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{"id": 1, "type": "t", "name": "carries one", "content": "full body one", "substance": "the condensed form"},
				{"id": 2, "type": "t", "name": "carries none", "content": "full body two"},
			},
			"total": 2,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	got, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2", len(got))
	}
	if got[0].Substance != "the condensed form" {
		t.Fatalf("candidate[0].Substance = %q, want the value the graph sent", got[0].Substance)
	}
	if got[1].Substance != "" {
		t.Fatalf("candidate[1].Substance = %q, want the zero value for a row that omitted the key entirely, not an error and not a placeholder", got[1].Substance)
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
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6}); err == nil {
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

func TestNodeRequestsEveryAnchorFieldInTheFieldsProjection(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, _, err := c.Node(context.Background(), 42); err != nil {
		t.Fatalf("Node: %v", err)
	}

	q, err := url.ParseQuery(rec.query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", rec.query, err)
	}
	requested := []string{}
	for _, field := range strings.Split(q.Get("fields"), ",") {
		requested = append(requested, strings.ToLower(strings.TrimSpace(field)))
	}

	carried := []string{}
	for _, field := range reflect.VisibleFields(reflect.TypeOf(loop.Anchor{})) {
		if field.Anonymous || !field.IsExported() {
			continue
		}
		carried = append(carried, field.Name)
	}
	if len(carried) == 0 {
		t.Fatal("the anchor type reports no field a projection could ever fill, so this guard would assert nothing")
	}
	for _, name := range carried {
		if !slices.Contains(requested, strings.ToLower(name)) {
			t.Fatalf("fields = %q, want it to cover every field the anchor carries; %q is missing — a field the projection never requests arrives empty from a real graph, and every id the run records is the requested one, so nothing downstream disagrees", q.Get("fields"), name)
		}
	}
}

func TestTheNon200ErrorBodyBoundIsFourThousandAndNinetySixBytesMeasuredAgainstALiteral(t *testing.T) {
	t.Parallel()

	const filler = "Z"
	const bodySize = 200000
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat(filler, bodySize)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	_, _, err := c.Node(context.Background(), 42)
	if err == nil {
		t.Fatal("Node returned nil error for a 500 response, want an error")
	}

	carried := strings.Count(err.Error(), filler)
	if carried != 4096 {
		t.Fatalf("the error carried %d bytes of a %d byte response body, want exactly 4096 — the bound the next judgement call's prompt is held to, so an off-by-one and a doubling are both defects", carried, bodySize)
	}
}

func TestNewClientDropsATrailingSlashSoTheReadPathNeverDoubles(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "k", srv.Client(), testLogger())
	if _, _, err := c.Node(context.Background(), 42); err != nil {
		t.Fatalf("Node: %v", err)
	}

	const wantPath = "/api/nodes"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q — a base URL configured with a trailing slash must not double the separator", gotPath, wantPath)
	}
}

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

func TestDefaultTimeoutIsFifteenSecondsMeasuredAgainstALiteralNotAgainstItself(t *testing.T) {
	t.Parallel()

	const want = 15 * time.Second
	if DefaultTimeout != want {
		t.Fatalf("DefaultTimeout = %v, want %v", DefaultTimeout, want)
	}
}

func TestRecallSendsSeveralQueriesAsOneRequestWithARepeatedQueriesKeyAndNeverSplitsAValueOnItsCommas(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	want := []string{"first phrasing", "second, with a comma, and a bracket [x]", "third phrasing"}
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: want, Limit: 6}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if rec.requests != 1 {
		t.Fatalf("Recall issued %d requests for %d queries, want 1: the graph averages the queries in one statement", rec.requests, len(want))
	}

	q, err := url.ParseQuery(rec.query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", rec.query, err)
	}
	if !slices.Equal(q["queries"], want) {
		t.Fatalf("queries = %q, want %q in order, one key per value", q["queries"], want)
	}
	if _, present := q["query"]; present {
		t.Fatalf("several queries sent the query key %v too, want queries alone: the graph refuses both together", q["query"])
	}
	if q.Get("count") != "6" {
		t.Fatalf("count = %q, want %q", q.Get("count"), "6")
	}
	if len(q) != 4 {
		t.Fatalf("query has keys %v, want exactly queries, count, fields and excludeName", keysOf(q))
	}
}

func TestRecallWithNoQueriesFailsWithoutAskingTheGraph(t *testing.T) {
	t.Parallel()

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: nil, Limit: 6}); err == nil {
		t.Fatal("Recall with no query returned no error: the graph would answer a request carrying neither key with an unranked listing that reads as a ranking")
	}
	if requests != 0 {
		t.Fatalf("Recall with no query issued %d requests, want none", requests)
	}
}

func TestRecallWithNoPositiveLimitFailsWithoutAskingTheGraph(t *testing.T) {
	t.Parallel()

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	for _, limit := range []int{0, -1} {
		if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: limit}); err == nil {
			t.Fatalf("Recall with limit %d returned no error: a request that forgot its limit would ask the graph for no rows or an unbounded listing", limit)
		}
	}
	if requests != 0 {
		t.Fatalf("Recall with no positive limit issued %d requests, want none", requests)
	}
}

func TestEveryRecallAsksTheGraphToExcludeRunRecordsAndTheAnchor(t *testing.T) {
	t.Parallel()

	for name, queries := range map[string][]string{"one query": {"q"}, "several queries": {"a", "b"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, rec := recordingClient(t)
			if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: queries, Limit: 6, Exclude: []int64{42}}); err != nil {
				t.Fatalf("Recall: %v", err)
			}

			q, err := url.ParseQuery(rec.query)
			if err != nil {
				t.Fatalf("ParseQuery(%q): %v", rec.query, err)
			}
			if got := q["excludeName"]; !slices.Equal(got, []string{"processor-run%"}) {
				t.Fatalf("excludeName = %q, want the run-record name pattern alone: the graph leaves these rows out before the count, and a recall that does not ask spends its slots on them", got)
			}
			if got := q.Get("count"); got != "6" {
				t.Fatalf("count = %q, want the limit %q alone: the graph leaves the excluded rows out before it counts, so the request must not pad the count to cover them", got, "6")
			}
			if got := q["excludeId"]; !slices.Equal(got, []string{"42"}) {
				t.Fatalf("excludeId = %q, want the anchor id: the block renders the anchor whole, so a slot spent on it is a slot lost", got)
			}
		})
	}
}

func TestARecallWithNoExcludedIdsSendsNoExcludeIdKey(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, _ := url.ParseQuery(rec.query)
	if _, present := q["excludeId"]; present {
		t.Fatalf("excludeId = %v with nothing to exclude, want the key absent", q["excludeId"])
	}
}

func TestSeveralExcludedIdsGoOutAsRepeatedKeysInTheOrderGiven(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Exclude: []int64{42, 7}}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, _ := url.ParseQuery(rec.query)
	if got := q.Get("count"); got != "6" {
		t.Fatalf("count = %q, want the limit %q alone with two excluded ids: the graph leaves the excluded rows out before it counts", got, "6")
	}
	if got := q["excludeId"]; !slices.Equal(got, []string{"42", "7"}) {
		t.Fatalf("excludeId = %q, want one repeated key per id, 42 then 7: the graph reads a repeated key as a list", got)
	}
}

func TestARecallCarriesNoGraphConstraintOfItsOwnWhateverTheRequestHolds(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	request := loop.RecallRequest{
		Queries: []string{"the input", "a derived phrasing"},
		Limit:   6,
		Exclude: []int64{42},
		Window:  loop.UpdateWindow{From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)},
	}
	if _, err := c.Recall(context.Background(), request); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q, err := url.ParseQuery(rec.query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", rec.query, err)
	}
	for _, key := range []string{"linkedto", "path"} {
		if _, present := q[key]; present {
			t.Fatalf("the recall carried %s = %v, want the key absent: processor chooses no region of the graph for a recall, so the whole graph is ranked", key, q[key])
		}
	}
}
