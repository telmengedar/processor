package divoid_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type recordingTransport struct {
	calls int
	url   string
	auth  string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls++
	rt.url = req.URL.String()
	rt.auth = req.Header.Get("Authorization")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"result":[{"id":42,"type":"task","name":"n","content":"b"}],"total":1}`))),
		Request:    req,
	}, nil
}

func TestClientBuiltByAnExternalKeyedLiteralSurvivesTheNilHTTPClientBranch(t *testing.T) {
	t.Parallel()

	c := divoid.Client{}

	_, found, err := c.Node(context.Background(), 42)
	if err == nil {
		t.Fatal("Node on a zero-value Client returned no error, want a transport error")
	}
	if found {
		t.Fatal("Node on a zero-value Client reported found=true")
	}
}

func TestClientBuiltByAnExternalKeyedLiteralSurvivesTheNilClockAndNilLoggerBranches(t *testing.T) {
	t.Parallel()

	c := divoid.Client{}

	receipt := c.WriteRun(context.Background(), loop.Record{Input: "hello", Subject: 42})
	if receipt.State != loop.NotStored {
		t.Fatalf("WriteRun on a zero-value Client reported %q, want %q", receipt.State, loop.NotStored)
	}
	if receipt.NodeID != 0 {
		t.Fatalf("WriteRun on a zero-value Client returned node id %d, want 0", receipt.NodeID)
	}
}

func TestNodeSendsItsRequestThroughTheHTTPClientSuppliedToNewClient(t *testing.T) {
	t.Parallel()

	rt := &recordingTransport{}
	c := divoid.NewClient("http://divoid.invalid", "supplied-key", &http.Client{Transport: rt}, testLogger())

	anchor, found, err := c.Node(context.Background(), 42)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if !found || anchor.ID != 42 {
		t.Fatalf("anchor = %+v, found = %v, want the row the supplied transport served", anchor, found)
	}
	if rt.calls != 1 {
		t.Fatalf("supplied transport saw %d calls, want 1", rt.calls)
	}
	if rt.auth != "Bearer supplied-key" {
		t.Fatalf("supplied transport saw Authorization %q, want the key given to NewClient", rt.auth)
	}
}
