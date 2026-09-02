package divoid

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	legCreate  = "create"
	legContent = "content"
	legLink    = "link"
	legDiscard = "discard"
)

type recordedCall struct {
	Method      string
	Path        string
	ContentType string
	Body        []byte
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type safeBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func capturingLogger() (*slog.Logger, *safeBuffer) {
	buf := &safeBuffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

func legOf(r *http.Request) string {
	switch {
	case r.Method == http.MethodDelete:
		return legDiscard
	case strings.HasSuffix(r.URL.Path, "/content"):
		return legContent
	case strings.HasSuffix(r.URL.Path, "/links"):
		return legLink
	default:
		return legCreate
	}
}

func writeServer(t *testing.T, newNodeID int64, failing ...string) (*httptest.Server, *[]recordedCall) {
	t.Helper()

	fails := make(map[string]bool, len(failing))
	for _, leg := range failing {
		fails[leg] = true
	}

	var mu sync.Mutex
	var calls []recordedCall

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		leg := legOf(r)

		mu.Lock()
		calls = append(calls, recordedCall{Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})
		mu.Unlock()

		if fails[leg] {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("UPSTREAM-BODY-MARKER the graph's own words"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if leg == legCreate {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": newNodeID})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func methodPaths(calls []recordedCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.Method + " " + c.Path
	}
	return out
}

func sampleRecord(subject int64) loop.Record {
	return loop.Record{Input: "what changed", Subject: subject, Answer: "the answer"}
}

func TestWriteRunIssuesTheThreePOSTsInOrder(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 10525)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteRun(context.Background(), sampleRecord(42))
	if receipt.NodeID != 10525 {
		t.Fatalf("receipt.NodeID = %d, want 10525 (the id the create call returned)", receipt.NodeID)
	}

	want := []string{"POST /api/nodes", "POST /api/nodes/10525/content", "POST /api/nodes/10525/links"}
	if got := methodPaths(*calls); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

func TestWriteRunSetsContentTypeOnTheContentPOST(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	c.WriteRun(context.Background(), sampleRecord(42))

	const wantContentType = "application/json"
	contentCall := (*calls)[1]
	if contentCall.ContentType != wantContentType {
		t.Fatalf("content POST Content-Type = %q, want %q", contentCall.ContentType, wantContentType)
	}
}

func TestWriteRunContentBodyIsTheRecordAsJSON(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	record := sampleRecord(42)
	c.WriteRun(context.Background(), record)

	var decoded loop.Record
	if err := json.Unmarshal((*calls)[1].Body, &decoded); err != nil {
		t.Fatalf("decode content body as loop.Record: %v; body=%s", err, (*calls)[1].Body)
	}
	if decoded.Answer != record.Answer || decoded.Subject != record.Subject {
		t.Fatalf("decoded content body = %+v, want it to carry the record's fields", decoded)
	}
}

func TestWriteRunStoredBodyCarriesNoWriteReceiptKey(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 4242)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	c.WriteRun(context.Background(), sampleRecord(42))

	stored := string((*calls)[1].Body)
	for _, absent := range []string{`"written"`, `"state"`, `"nodeId"`, "4242"} {
		if strings.Contains(stored, absent) {
			t.Fatalf("stored body contains %q, want the record alone with nothing about its own filing; body=%s", absent, stored)
		}
	}
}

func TestWriteRunLinkBodyIsTheBareSubjectID(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	c.WriteRun(context.Background(), sampleRecord(777))

	linkBody := strings.TrimSpace(string((*calls)[2].Body))
	if linkBody != "777" {
		t.Fatalf("link POST body = %q, want the bare target id %q, not a wrapping object", linkBody, "777")
	}
}

func TestWriteRunCreateBodyCarriesTheAdapterChosenTypeAndNameNotTheCaller(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 1)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	c.clock = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

	c.WriteRun(context.Background(), sampleRecord(42))

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
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	fixed := time.Date(2026, 9, 1, 12, 30, 0, 0, time.UTC)
	c.clock = func() time.Time { return fixed }

	record := sampleRecord(42)
	record.Input = "what changed in the assembler"
	c.WriteRun(context.Background(), record)

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
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	c.clock = func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }

	const wantNameInputRuneBound = 80

	record := sampleRecord(42)
	record.Input = strings.Repeat("x", wantNameInputRuneBound+50)
	c.WriteRun(context.Background(), record)

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

func TestWriteRunReportsStoredWithTheNodeIDWhenAllThreeCallsLand(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 10525)
	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)

	receipt := c.WriteRun(context.Background(), sampleRecord(42))

	if receipt.State != loop.Stored {
		t.Fatalf("receipt.State = %q, want %q", receipt.State, loop.Stored)
	}
	if receipt.NodeID != 10525 {
		t.Fatalf("receipt.NodeID = %d, want 10525", receipt.NodeID)
	}
	for _, c := range *calls {
		if c.Method == http.MethodDelete {
			t.Fatalf("a DELETE was issued on the fully successful path: %s %s", c.Method, c.Path)
		}
	}
	for _, unwanted := range []string{"repairable orphan", "uncollected shell", "write-back failed"} {
		if strings.Contains(log.String(), unwanted) {
			t.Fatalf("operator log carries %q on a fully successful write; log:\n%s", unwanted, log.String())
		}
	}
}

func TestWriteRunReportsNotStoredAndAttemptsNothingFurtherWhenTheCreateFails(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 10525, legCreate)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteRun(context.Background(), sampleRecord(42))

	if receipt.State != loop.NotStored {
		t.Fatalf("receipt.State = %q, want %q", receipt.State, loop.NotStored)
	}
	if receipt.NodeID != 0 {
		t.Fatalf("receipt.NodeID = %d, want 0 — no node exists to name", receipt.NodeID)
	}
	want := []string{"POST /api/nodes"}
	if got := methodPaths(*calls); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v — nothing exists to body, link or discard", got, want)
	}
}

func TestWriteRunDiscardsTheBodylessShellAndReportsNotStoredWhenTheContentWriteFails(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 10525, legContent)
	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)

	receipt := c.WriteRun(context.Background(), sampleRecord(42))

	if receipt.State != loop.NotStored {
		t.Fatalf("receipt.State = %q, want %q", receipt.State, loop.NotStored)
	}
	if receipt.NodeID != 0 {
		t.Fatalf("receipt.NodeID = %d, want 0 — no node holds the record", receipt.NodeID)
	}
	want := []string{"POST /api/nodes", "POST /api/nodes/10525/content", "DELETE /api/nodes/10525"}
	if got := methodPaths(*calls); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v — the shell is discarded and the link is never attempted", got, want)
	}
	if strings.Contains(log.String(), "uncollected shell") {
		t.Fatalf("operator log reports an uncollected shell although the discard succeeded; log:\n%s", log.String())
	}
}

func TestWriteRunDiscardsOnlyTheNodeItsOwnCreateReturned(t *testing.T) {
	t.Parallel()

	const createdID = 10525
	const subject = 42

	srv, calls := writeServer(t, createdID, legContent)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	c.WriteRun(context.Background(), sampleRecord(subject))

	var deletes []string
	for _, call := range *calls {
		if call.Method == http.MethodDelete {
			deletes = append(deletes, call.Path)
		}
	}
	want := []string{"/api/nodes/10525"}
	if !slices.Equal(deletes, want) {
		t.Fatalf("DELETEs = %v, want %v — exactly the node this run created, never the subject %d", deletes, want, subject)
	}
}

func TestWriteRunKeepsTheCompleteRecordAndReportsUnlinkedWhenTheLinkFails(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 10525, legLink)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteRun(context.Background(), sampleRecord(42))

	if receipt.State != loop.Unlinked {
		t.Fatalf("receipt.State = %q, want %q", receipt.State, loop.Unlinked)
	}
	if receipt.NodeID != 10525 {
		t.Fatalf("receipt.NodeID = %d, want 10525 — the node holding the complete record is named", receipt.NodeID)
	}
	want := []string{"POST /api/nodes", "POST /api/nodes/10525/content", "POST /api/nodes/10525/links"}
	if got := methodPaths(*calls); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v — a node holding the record is never discarded", got, want)
	}
}

func TestWriteRunLogsTheRepairableOrphanWithItsNodeIDOnlyWhenTheLinkFailed(t *testing.T) {
	t.Parallel()

	failedSrv, _ := writeServer(t, 10525, legLink)
	failedLogger, failedLog := capturingLogger()
	NewClient(failedSrv.URL, "k", failedSrv.Client(), failedLogger).WriteRun(context.Background(), sampleRecord(42))

	if !strings.Contains(failedLog.String(), `msg="repairable orphan"`) {
		t.Fatalf("operator log has no repairable-orphan record after a failed link; log:\n%s", failedLog.String())
	}
	if !strings.Contains(failedLog.String(), "node=10525") {
		t.Fatalf("repairable-orphan record does not carry the node id a human needs to repair it; log:\n%s", failedLog.String())
	}

	okSrv, _ := writeServer(t, 10525)
	okLogger, okLog := capturingLogger()
	NewClient(okSrv.URL, "k", okSrv.Client(), okLogger).WriteRun(context.Background(), sampleRecord(42))

	if strings.Contains(okLog.String(), "repairable orphan") {
		t.Fatalf("operator log reports a repairable orphan after a fully successful write; log:\n%s", okLog.String())
	}
}

func TestWriteRunStillReportsNotStoredWhenTheDiscardItselfFails(t *testing.T) {
	t.Parallel()

	srv, _ := writeServer(t, 10525, legContent, legDiscard)
	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)

	receipt := c.WriteRun(context.Background(), sampleRecord(42))

	if receipt.State != loop.NotStored {
		t.Fatalf("receipt.State = %q, want %q — the record's fate is unchanged by whether the litter was collected", receipt.State, loop.NotStored)
	}
	if receipt.NodeID != 0 {
		t.Fatalf("receipt.NodeID = %d, want 0 — the surviving shell holds no record", receipt.NodeID)
	}
	if !strings.Contains(log.String(), `msg="uncollected shell"`) {
		t.Fatalf("operator log has no uncollected-shell record after a failed discard; log:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "node=10525") {
		t.Fatalf("uncollected-shell record does not carry the node id a human needs to remove it; log:\n%s", log.String())
	}
}

func TestWriteRunKeepsTheUpstreamDiagnosisInTheLogAndOutOfTheReceipt(t *testing.T) {
	t.Parallel()

	srv, _ := writeServer(t, 10525, legCreate)
	logger, log := capturingLogger()
	c := NewClient(srv.URL, "k", srv.Client(), logger)

	receipt := c.WriteRun(context.Background(), sampleRecord(42))

	rendered, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	for _, secret := range []string{"UPSTREAM-BODY-MARKER", srv.URL, "/api/nodes"} {
		if strings.Contains(string(rendered), secret) {
			t.Fatalf("receipt %s discloses %q", rendered, secret)
		}
	}
	if !strings.Contains(log.String(), "UPSTREAM-BODY-MARKER") {
		t.Fatalf("operator log does not carry the upstream body the receipt must not; log:\n%s", log.String())
	}
	if !strings.Contains(log.String(), `msg="write-back failed"`) {
		t.Fatalf("operator log has no write-back-failed record; log:\n%s", log.String())
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
	c := NewClient(srv.URL, "test-key", srv.Client(), testLogger())

	c.WriteRun(context.Background(), sampleRecord(42))

	const want = "Bearer test-key"
	if gotAuth != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestWriteRunDiscardAuthenticatesWithBearer(t *testing.T) {
	t.Parallel()

	var deleteAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/nodes" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "test-key", srv.Client(), testLogger())

	c.WriteRun(context.Background(), sampleRecord(42))

	const want = "Bearer test-key"
	if deleteAuth != want {
		t.Fatalf("DELETE Authorization = %q, want %q — the discard is an authenticated call like every other", deleteAuth, want)
	}
}
