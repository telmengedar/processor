package divoid

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

var scalarArrayAndBoolean = loop.Filter{
	"type":     {"thought", "knowledge"},
	"path":     {"[id:5|6]/[]"},
	"nostatus": {"true"},
	"severity": {"3", "4"},
	"name":     {" Mixed Case% "},
}

func queryOf(t *testing.T, raw string) url.Values {
	t.Helper()

	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return q
}

func TestRecallSendsEveryFilterKeyAsRepeatedKeysWithValuesByteEqualToTheConfigs(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Filter: scalarArrayAndBoolean}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q := queryOf(t, rec.query)
	for name, want := range scalarArrayAndBoolean {
		if got := q[name]; !reflect.DeepEqual(got, want) {
			t.Errorf("the recall sent %s=%q, want %q: an array is one repeated key per element, joined or reformatted values are a different filter", name, got, want)
		}
	}
}

func TestNodeSendsEveryFilterKeyAsRepeatedKeysBesideItsOwnIdAndFields(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, _, err := c.Node(context.Background(), 71, scalarArrayAndBoolean); err != nil {
		t.Fatalf("Node: %v", err)
	}

	q := queryOf(t, rec.query)
	for name, want := range scalarArrayAndBoolean {
		if got := q[name]; !reflect.DeepEqual(got, want) {
			t.Errorf("the id read sent %s=%q, want %q", name, got, want)
		}
	}
	if got := q["id"]; !reflect.DeepEqual(got, []string{"71"}) {
		t.Errorf("the id read sent id=%q, want exactly 71", got)
	}
	if q.Get("fields") != nodeFields {
		t.Errorf("the id read sent fields=%q, want %q", q.Get("fields"), nodeFields)
	}
}

func TestNodeWithoutAFilterSendsOnlyTheIdAndFields(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	if _, _, err := c.Node(context.Background(), 71, nil); err != nil {
		t.Fatalf("Node: %v", err)
	}

	want := url.Values{"id": {"71"}, "fields": {nodeFields}}.Encode()
	if rec.query != want {
		t.Fatalf("an unfiltered id read sent %q, want %q byte-identical", rec.query, want)
	}
}

func TestMinSimilarityTravelsOnARecallAndNeverOnAnIdRead(t *testing.T) {
	t.Parallel()

	filter := loop.Filter{"type": {"thought"}, "minSimilarity": {"0.4"}}

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Filter: filter}); err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if got := queryOf(t, rec.query)["minSimilarity"]; !reflect.DeepEqual(got, []string{"0.4"}) {
		t.Errorf("the recall sent minSimilarity=%q, want 0.4", got)
	}

	if _, _, err := c.Node(context.Background(), 71, filter); err != nil {
		t.Fatalf("Node: %v", err)
	}
	q := queryOf(t, rec.query)
	if _, sent := q["minSimilarity"]; sent {
		t.Errorf("the id read sent minSimilarity=%q: the graph answers 400 to it without a query, which would fail every addressed read", q["minSimilarity"])
	}
	if q.Get("type") != "thought" {
		t.Errorf("the id read sent type=%q, want the rest of the filter", q.Get("type"))
	}
}

func TestAConfigsExclusionsAreAppendedToTheSubjectAndTheRunRecordsNeverReplacingThem(t *testing.T) {
	t.Parallel()

	filter := loop.Filter{"excludeId": {"5", "6"}, "excludeName": {"draft-%"}}

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Exclude: []int64{42}, Filter: filter}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q := queryOf(t, rec.query)
	if got, want := q["excludeId"], []string{"42", "5", "6"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the recall sent excludeId=%q, want %q: the subject first, the config's after it", got, want)
	}
	if got, want := q["excludeName"], []string{RunNamePrefix + "%", "draft-%"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the recall sent excludeName=%q, want %q: the run-record pattern first, the config's after it", got, want)
	}
}

func TestABoundTheConfigSetsReplacesTheDerivedOneAndTheOtherBoundStaysDerived(t *testing.T) {
	t.Parallel()

	window := loop.UpdateWindow{
		From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	}
	filter := loop.Filter{"updatedFrom": {"2026-01-01T00:00:00Z"}}

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Window: window, Filter: filter}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q := queryOf(t, rec.query)
	if got, want := q["updatedFrom"], []string{"2026-01-01T00:00:00Z"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the recall sent updatedFrom=%q, want exactly the config's %q: a derived bound that replaced it would widen the view, and both would be two values", got, want)
	}
	if got, want := q["updatedTo"], []string{window.To.Format(time.RFC3339)}; !reflect.DeepEqual(got, want) {
		t.Errorf("the recall sent updatedTo=%q, want the derived %q: a bound the config leaves unset is still the window's", got, want)
	}
}

func TestAnUpdatedToTheConfigSetsReplacesTheDerivedOneAndUpdatedFromStaysDerived(t *testing.T) {
	t.Parallel()

	window := loop.UpdateWindow{
		From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	}
	filter := loop.Filter{"updatedTo": {"2026-02-02T00:00:00Z"}}

	c, rec := recordingClient(t)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Window: window, Filter: filter}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q := queryOf(t, rec.query)
	if got, want := q["updatedTo"], []string{"2026-02-02T00:00:00Z"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the recall sent updatedTo=%q, want exactly the config's %q: the derived bound would widen the view past it", got, want)
	}
	if got, want := q["updatedFrom"], []string{window.From.Format(time.RFC3339)}; !reflect.DeepEqual(got, want) {
		t.Errorf("the recall sent updatedFrom=%q, want the derived %q", got, want)
	}
}

func TestAConfigBoundWithoutADerivedWindowIsSentAsItIs(t *testing.T) {
	t.Parallel()

	c, rec := recordingClient(t)
	filter := loop.Filter{"updatedTo": {"2026-02-02T00:00:00Z"}}
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6, Filter: filter}); err != nil {
		t.Fatalf("Recall: %v", err)
	}

	q := queryOf(t, rec.query)
	if got := q["updatedTo"]; !reflect.DeepEqual(got, []string{"2026-02-02T00:00:00Z"}) {
		t.Errorf("the recall sent updatedTo=%q, want the config's", got)
	}
	if _, sent := q["updatedFrom"]; sent {
		t.Errorf("the recall sent updatedFrom=%q though neither the config nor a window set it", q["updatedFrom"])
	}
}

func rejectingClient(t *testing.T, status int) *Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"code":"bad","text":"the path could not be parsed"}`))
	}))
	t.Cleanup(srv.Close)

	return NewClient(srv.URL, "k", srv.Client(), testLogger())
}

func TestAFourHundredLevelAnswerIsATypedRejectionWhoseMessageKeepsTheGraphsWords(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusBadRequest, http.StatusRequestURITooLong} {
		c := rejectingClient(t, status)
		_, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
		if !errors.Is(err, loop.ErrGraphRejected) {
			t.Errorf("status %d: Recall err = %v, want a rejection", status, err)
			continue
		}
		if want := "the path could not be parsed"; !strings.Contains(err.Error(), want) {
			t.Errorf("status %d: the error reads %q, want the graph's message %q", status, err, want)
		}
	}
}

func TestAnAnswerThatSaysNothingAboutTheFilterIsNotARejection(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusRequestTimeout, http.StatusTooManyRequests} {
		c := rejectingClient(t, status)
		_, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6})
		if err == nil || errors.Is(err, loop.ErrGraphRejected) {
			t.Errorf("status %d: Recall err = %v, want a failure that is not a rejection of the filter", status, err)
		}
	}
}

func TestAServerErrorAndATransportFailureAreNotRejections(t *testing.T) {
	t.Parallel()

	c := rejectingClient(t, http.StatusBadGateway)
	if _, err := c.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6}); err == nil || errors.Is(err, loop.ErrGraphRejected) {
		t.Errorf("a 502 gave err = %v, want a failure that is not a rejection", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := srv.Client()
	base := srv.URL
	srv.Close()
	dead := NewClient(base, "k", client, testLogger())
	if _, err := dead.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 6}); err == nil || errors.Is(err, loop.ErrGraphRejected) {
		t.Errorf("a refused connection gave err = %v, want a failure that is not a rejection", err)
	}
}
