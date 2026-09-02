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

// recordedPost is one POST the write path made, captured in call order —
// the shape TestWriteRunIssuesTheThreePOSTsInOrder pins design C32's
// "three POSTs" against.
type recordedPost struct {
	Path        string
	ContentType string
	Body        []byte
}

// writeServer answers every POST /api/nodes with a fixed new id, and every
// other POST with 200 and an empty body — enough for WriteRun's happy
// path — while recording each call in order.
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

// TestWriteRunIssuesTheThreePOSTsInOrder pins design C32: create, then set
// content, then link — against the id the create call returned.
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

// TestWriteRunSetsContentTypeOnTheContentPOST pins C32's "content type as
// a header" on the body-setting call specifically, not on every call.
func TestWriteRunSetsContentTypeOnTheContentPOST(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())

	if _, err := c.WriteRun(context.Background(), sampleRecord(42)); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	// A literal, not runContentType (design §14 step 1): a shared constant
	// moves both sides together and can never fail on a value change.
	// This one is an external contract besides — application/json is
	// what makes the written record decodable by the graph at all (CF-2,
	// design §14 step 11).
	const wantContentType = "application/json"
	contentCall := (*calls)[1]
	if contentCall.ContentType != wantContentType {
		t.Fatalf("content POST Content-Type = %q, want %q", contentCall.ContentType, wantContentType)
	}
}

// TestWriteRunContentBodyIsTheRecordAsJSON pins §7/§8.3: "Body: The run
// record."
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

// TestWriteRunLinkBodyIsTheBareSubjectID pins C32: "link (POST
// /api/nodes/{id}/links with the bare target id as the body)" — not an
// object wrapping it.
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

// TestWriteRunCreateBodyCarriesTheAdapterChosenTypeAndNameNotTheCaller
// pins design §8.3: "The adapter chooses type, name and edge. The caller
// supplies no structure." loop.Record carries no type or name field at
// all — this test would fail to compile if WriteRun tried to source
// either from the record, which is itself evidence the caller supplies
// none.
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
	// A literal, not runNodeType (design §14 step 1): this is an external
	// contract — #10424 §5.7's own name for the narrative tier — that can
	// be silently changed today with a green suite if asserted against
	// the constant that defines it (CF-2).
	const wantNodeType = "session-log"
	if created.Type != wantNodeType {
		t.Fatalf("create Type = %q, want %q (§10424 §5.7's narrative tier)", created.Type, wantNodeType)
	}
	if created.Name == "" {
		t.Fatal("create Name is empty, want a deterministic name")
	}
}

// TestRunNameIsDeterministicFromPrefixTimestampAndInput pins design §8.3:
// "a fixed prefix, the timestamp, and a bounded prefix of the input."
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
	// A literal prefix, not runNamePrefix (design §14 step 1; CF-2).
	const wantNamePrefix = "processor-run"
	want := wantNamePrefix + " " + fixed.Format(time.RFC3339) + " — what changed in the assembler"
	if created.Name != want {
		t.Fatalf("name = %q, want %q", created.Name, want)
	}
}

// TestRunNameTruncatesALongInputWithABoundedPrefix pins the "bounded
// prefix" half of §8.3's name contract — a fixture whose input is longer
// than the bound, so a truncation that silently stopped truncating would
// fail this test (the record's raw Input is unbounded; only the name
// excerpt is bounded).
func TestRunNameTruncatesALongInputWithABoundedPrefix(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client())
	c.clock = func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }

	// A literal bound, not runNameInputRunes (design §14 step 1; CF-2):
	// the previous version asserted the constant against itself on both
	// sides, so M7b (80 -> 40) stayed green.
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

// TestWriteRunOnCreateFailureMakesNoFurtherCalls pins that a failure at
// any step stops the sequence rather than attempting the next call on a
// node that may not exist.
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

// TestWriteRunOnContentFailureDoesNotAttemptTheLink pins the same
// stop-on-failure rule at the second step.
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
