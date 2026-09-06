package divoid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type substanceCall struct {
	Method      string
	Path        string
	Query       string
	ContentType string
	Body        []byte
}

func substanceServer(t *testing.T, status int, response string) (*Client, *[]substanceCall) {
	t.Helper()

	var mu sync.Mutex
	var calls []substanceCall

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		calls = append(calls, substanceCall{
			Method:      r.Method,
			Path:        r.URL.Path,
			Query:       r.URL.RawQuery,
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != "" {
			_, _ = io.WriteString(w, response)
		}
	}))
	t.Cleanup(srv.Close)

	return NewClient(srv.URL, "a-key", srv.Client(), testLogger()), &calls
}

func TestTheCondensationProjectionRequestsSubstanceAlongsideContentInOneRoundTrip(t *testing.T) {
	client, calls := substanceServer(t, http.StatusOK, `{"result":[{"id":7,"type":"documentation","name":"A node","contentType":"text/markdown","content":"a body","substance":"a condensation"}],"total":1}`)

	node, found, err := client.NodeWithSubstance(context.Background(), 7)
	if err != nil || !found {
		t.Fatalf("want the node found, got found=%v err=%v", found, err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want exactly one round trip, got %d", len(*calls))
	}
	if !strings.Contains((*calls)[0].Query, "substance") {
		t.Fatalf("want substance in the projection, got query %q", (*calls)[0].Query)
	}
	if node.Substance != "a condensation" || node.Content != "a body" {
		t.Fatalf("want both representations carried, got %+v", node)
	}
}

func TestTheCondensationProjectionAnchorsOnTheRequestedIdRatherThanTheFirstRowReturned(t *testing.T) {
	client, _ := substanceServer(t, http.StatusOK, `{"result":[{"id":99,"name":"Another node","content":"the wrong body"},{"id":7,"name":"A node","content":"the right body"}],"total":2}`)

	node, found, err := client.NodeWithSubstance(context.Background(), 7)
	if err != nil || !found {
		t.Fatalf("want the node found, got found=%v err=%v", found, err)
	}
	if node.Content != "the right body" {
		t.Fatalf("want the row whose id was asked for, got %q", node.Content)
	}
}

func TestAMissingNodeIsReportedAsAbsenceRatherThanAsAnError(t *testing.T) {
	client, _ := substanceServer(t, http.StatusOK, `{"result":[],"total":0}`)

	_, found, err := client.NodeWithSubstance(context.Background(), 7)

	if err != nil {
		t.Fatalf("an empty result is absence, not an error, got %v", err)
	}
	if found {
		t.Fatalf("want found=false for an empty result")
	}
}

func TestTheContentRereadAsksForTheBodyAloneAndNotForSubstance(t *testing.T) {
	client, calls := substanceServer(t, http.StatusOK, `{"result":[{"id":7,"content":"a body"}],"total":1}`)

	body, found, err := client.Content(context.Background(), 7)
	if err != nil || !found {
		t.Fatalf("want the content found, got found=%v err=%v", found, err)
	}
	if body != "a body" {
		t.Fatalf("want the body, got %q", body)
	}
	if strings.Contains((*calls)[0].Query, "substance") {
		t.Fatalf("the compare-and-write reread must not pay for substance, got query %q", (*calls)[0].Query)
	}
}

func TestASubstanceWriteIsAJSONPatchThatReplacesTheSubstancePathAndNothingElse(t *testing.T) {
	client, calls := substanceServer(t, http.StatusOK, "")

	if err := client.SetSubstance(context.Background(), 7, "a condensation"); err != nil {
		t.Fatalf("want the write to succeed, got %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want exactly one call, got %d", len(*calls))
	}
	call := (*calls)[0]
	if call.Method != http.MethodPatch {
		t.Fatalf("want PATCH, got %s", call.Method)
	}
	if call.Path != "/api/nodes/7" {
		t.Fatalf("want the node route, got %s", call.Path)
	}
	if call.ContentType != jsonPatchContentType {
		t.Fatalf("want content type %q, got %q", jsonPatchContentType, call.ContentType)
	}

	var ops []patchOperation
	if err := json.Unmarshal(call.Body, &ops); err != nil {
		t.Fatalf("want a JSON-Patch array body, got %q: %v", call.Body, err)
	}
	if len(ops) != 1 {
		t.Fatalf("want exactly one operation, got %d", len(ops))
	}
	if ops[0].Op != "replace" || ops[0].Path != "/substance" || ops[0].Value != "a condensation" {
		t.Fatalf("want one replace of /substance, got %+v", ops[0])
	}
}

func TestTheSubstanceWriteReachesNoContentRouteAndCreatesOrDeletesNothing(t *testing.T) {
	client, calls := substanceServer(t, http.StatusOK, "")

	if err := client.SetSubstance(context.Background(), 7, "a condensation"); err != nil {
		t.Fatalf("want the write to succeed, got %v", err)
	}

	for _, call := range *calls {
		if strings.HasSuffix(call.Path, "/content") || strings.HasSuffix(call.Path, "/links") {
			t.Fatalf("the substance write reached %s, which it must never touch", call.Path)
		}
		if call.Method == http.MethodPost || call.Method == http.MethodDelete {
			t.Fatalf("the substance write issued a %s, which would create or delete a node", call.Method)
		}
	}
}

func TestABlankSubstanceIsRefusedBeforeAnyRequestIsMade(t *testing.T) {
	client, calls := substanceServer(t, http.StatusOK, "")

	if err := client.SetSubstance(context.Background(), 7, "   \n\t "); err == nil {
		t.Fatalf("want a blank substance refused")
	}
	if len(*calls) != 0 {
		t.Fatalf("a refused write must reach the graph not at all, got %d calls", len(*calls))
	}
}

func TestARejectedSubstanceWriteSurfacesTheStatusAndTheNodeItFailedOn(t *testing.T) {
	client, _ := substanceServer(t, http.StatusConflict, `{"error":"the node moved"}`)

	err := client.SetSubstance(context.Background(), 7, "a condensation")

	if err == nil {
		t.Fatalf("want an error on a non-2xx response")
	}
	if !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "7") {
		t.Fatalf("want the status and the node id in the error, got %v", err)
	}
}
