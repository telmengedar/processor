package divoid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

type recordedPost struct {
	Path        string
	ContentType string
	Body        []byte
}

func writeServer(t *testing.T, newNodeID int64) (*httptest.Server, *[]recordedPost) {
	t.Helper()
	var calls []recordedPost
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, recordedPost{Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/nodes" {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": newNodeID})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func sampleRecord(subject int64) loop.Record {
	return loop.Record{Input: "what changed", Subject: subject, Answer: "the answer"}
}

func TestWriteRunIssuesTheThreePOSTsInOrder(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 10525)
	c := NewClient(srv.URL, "k", srv.Client())

	id, err := c.WriteRun(context.Background(), sampleRecord(42))
	if err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if id != 10525 {
		t.Fatalf("WriteRun id = %d, want 10525 (the id the create call returned)", id)
	}

	if len(*calls) != 3 {
		t.Fatalf("got %d POSTs, want 3 (create, content, link): %+v", len(*calls), *calls)
	}
	wantPaths := []string{"/api/nodes", "/api/nodes/10525/content", "/api/nodes/10525/links"}
	for i, want := range wantPaths {
		if (*calls)[i].Path != want {
			t.Fatalf("call[%d].Path = %q, want %q", i, (*calls)[i].Path, want)
		}
	}
}

func TestWriteRunSetsContentTypeOnTheContentPOST(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())

	if _, err := c.WriteRun(context.Background(), sampleRecord(42)); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	const wantContentType = "application/json"
	contentCall := (*calls)[1]
	if contentCall.ContentType != wantContentType {
		t.Fatalf("content POST Content-Type = %q, want %q", contentCall.ContentType, wantContentType)
	}
}

func TestWriteRunContentBodyIsTheRecordAsJSON(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())

	record := sampleRecord(42)
	if _, err := c.WriteRun(context.Background(), record); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	var decoded loop.Record
	if err := json.Unmarshal((*calls)[1].Body, &decoded); err != nil {
		t.Fatalf("decode content body as loop.Record: %v; body=%s", err, (*calls)[1].Body)
	}
	if decoded.Answer != record.Answer || decoded.Subject != record.Subject {
		t.Fatalf("decoded content body = %+v, want it to carry the record's fields", decoded)
	}
}

func TestWriteRunLinkBodyIsTheBareSubjectID(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())

	if _, err := c.WriteRun(context.Background(), sampleRecord(777)); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	linkBody := strings.TrimSpace(string((*calls)[2].Body))
	if linkBody != "777" {
		t.Fatalf("link POST body = %q, want the bare target id %q, not a wrapping object", linkBody, "777")
	}
}

func TestWriteRunCreateBodyCarriesTheAdapterChosenTypeAndNameNotTheCaller(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())
	c.clock = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

	if _, err := c.WriteRun(context.Background(), sampleRecord(42)); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	var created createNodeRequest
	if err := json.Unmarshal((*calls)[0].Body, &created); err != nil {
		t.Fatalf("decode create body: %v; body=%s", err, (*calls)[0].Body)
	}
	const wantNodeType = "session-log"
	if created.Type != wantNodeType {
		t.Fatalf("create Type = %q, want %q (design §8.3's written node contract)", created.Type, wantNodeType)
	}
	if created.Name == "" {
		t.Fatal("create Name is empty, want a deterministic name")
	}
}

func TestRunNameIsDeterministicFromPrefixTimestampAndInput(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())
	fixed := time.Date(2026, 9, 1, 12, 30, 0, 0, time.UTC)
	c.clock = func() time.Time { return fixed }

	record := sampleRecord(42)
	record.Input = "what changed in the assembler"
	if _, err := c.WriteRun(context.Background(), record); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	var created createNodeRequest
	if err := json.Unmarshal((*calls)[0].Body, &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	const wantNamePrefix = "processor-run"
	want := wantNamePrefix + " " + fixed.Format(time.RFC3339) + " — what changed in the assembler"
	if created.Name != want {
		t.Fatalf("name = %q, want %q", created.Name, want)
	}
}

func TestRunNameTruncatesALongInputWithABoundedPrefix(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())
	c.clock = func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }

	const wantNameInputRuneBound = 80

	record := sampleRecord(42)
	record.Input = strings.Repeat("x", wantNameInputRuneBound+50)
	if _, err := c.WriteRun(context.Background(), record); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	var created createNodeRequest
	if err := json.Unmarshal((*calls)[0].Body, &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if strings.Contains(created.Name, strings.Repeat("x", wantNameInputRuneBound+1)) {
		t.Fatalf("name = %q, want the input excerpt bounded at %d runes", created.Name, wantNameInputRuneBound)
	}
	if !strings.Contains(created.Name, strings.Repeat("x", wantNameInputRuneBound)) {
		t.Fatalf("name = %q, want it to contain the full %d-rune bound, not less", created.Name, wantNameInputRuneBound)
	}
}

func TestWriteRunOnCreateFailureMakesNoFurtherCalls(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "k", srv.Client())

	if _, err := c.WriteRun(context.Background(), sampleRecord(42)); err == nil {
		t.Fatal("WriteRun returned nil error when the create call failed, want an error")
	}
	if calls != 1 {
		t.Fatalf("server saw %d calls, want exactly 1 (create) — content and link must not be attempted", calls)
	}
}

func TestWriteRunOnContentFailureDoesNotAttemptTheLink(t *testing.T) {
	t.Parallel()

	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.URL.Path == "/api/nodes" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "k", srv.Client())

	if _, err := c.WriteRun(context.Background(), sampleRecord(42)); err == nil {
		t.Fatal("WriteRun returned nil error when the content call failed, want an error")
	}
	if len(calls) != 2 {
		t.Fatalf("server saw %d calls %v, want exactly 2 (create, content) — link must not be attempted", len(calls), calls)
	}
}

func TestWriteRunAuthenticatesWithBearer(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/nodes" {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "test-key", srv.Client())

	if _, err := c.WriteRun(context.Background(), sampleRecord(42)); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	const want = "Bearer test-key"
	if gotAuth != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth, want)
	}
}
