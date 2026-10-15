package divoid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func noteRequest() loop.NoteRequest {
	return loop.NoteRequest{Type: "thought", Name: "a thought", Text: "\n  a thought, in full\nover two lines \n", Root: 500, Subject: 42, About: 7}
}

func failingNodeServer(t *testing.T, newNodeID int64, failAt ...int) (*httptest.Server, *[]recordedCall) {
	t.Helper()

	var mu sync.Mutex
	var calls []recordedCall

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		position := len(calls)
		calls = append(calls, recordedCall{Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})
		mu.Unlock()

		if slices.Contains(failAt, position) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if legOf(r) == legCreate {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": newNodeID})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestWriteNoteCreatesThenBodiesThenLinksTheSubjectThenTheAboutInThatOrder(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteNote(context.Background(), noteRequest())
	if receipt.State != loop.Stored || receipt.NodeID != 3100 {
		t.Fatalf("receipt = %+v, want stored at the id the create call returned", receipt)
	}

	want := []string{"POST /api/nodes", "POST /api/nodes/3100/content", "POST /api/nodes/3100/links", "POST /api/nodes/3100/links"}
	if got := methodPaths(*calls); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v: create, content, the subject's link, then the about's link", got, want)
	}

	if got := strings.TrimSpace(string((*calls)[2].Body)); got != "42" {
		t.Errorf("the first link targets %q, want the subject 42", got)
	}
	if got := strings.TrimSpace(string((*calls)[3].Body)); got != "7" {
		t.Errorf("the second link targets %q, want the about 7", got)
	}
}

func TestWriteNoteCreateCarriesTheRequestsTypeNameAndRootAndTheContentIsTheTextVerbatim(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	request := noteRequest()

	c.WriteNote(context.Background(), request)

	var created createNodeRequest
	if err := json.Unmarshal((*calls)[0].Body, &created); err != nil {
		t.Fatalf("decode create body: %v; body=%s", err, (*calls)[0].Body)
	}
	if created.Type != "thought" || created.Name != "a thought" || created.RootNodeID != 500 {
		t.Errorf("create = %+v, want type thought, name \"a thought\" and root 500", created)
	}
	if got := string((*calls)[1].Body); got != request.Text {
		t.Errorf("content = %q, want the text verbatim %q", got, request.Text)
	}
	if got := (*calls)[1].ContentType; got != "text/markdown; charset=utf-8" {
		t.Errorf("content type = %q, want text/markdown; charset=utf-8", got)
	}
}

func TestWriteNoteCreateBodyLeavesTheEmbeddingOnSoRecallCanFindTheNote(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	c.WriteNote(context.Background(), noteRequest())

	if strings.Contains(string((*calls)[0].Body), "skipEmbedding") {
		t.Errorf("create body = %s, want no skipEmbedding member: a note is found by semantic search", (*calls)[0].Body)
	}
}

func TestWriteNoteSendsNoRootNodeIdWhenTheRequestHasNone(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	request := noteRequest()
	request.Root = 0

	c.WriteNote(context.Background(), request)

	if strings.Contains(string((*calls)[0].Body), "rootNodeId") {
		t.Errorf("create body = %s, want no rootNodeId member for a root of zero", (*calls)[0].Body)
	}
}

func TestWriteNoteMakesNoSecondLinkWhenTheRequestHasNoAbout(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())
	request := noteRequest()
	request.About = 0

	receipt := c.WriteNote(context.Background(), request)

	want := []string{"POST /api/nodes", "POST /api/nodes/3100/content", "POST /api/nodes/3100/links"}
	if got := methodPaths(*calls); !slices.Equal(got, want) || receipt.State != loop.Stored {
		t.Fatalf("calls = %v receipt = %+v, want one link and a stored note", got, receipt)
	}
}

func TestWriteNoteFilesNoMoreEdgesThanTheBound(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	c.WriteNote(context.Background(), noteRequest())

	links := 0
	for _, call := range *calls {
		if strings.HasSuffix(call.Path, "/links") {
			links++
		}
	}
	if links != loop.MaxNoteEdges {
		t.Errorf("the note was filed with %d links, want %d: the subject and the one node it is about", links, loop.MaxNoteEdges)
	}
}

func TestWriteNoteDiscardsTheShellWhenTheContentFailsAndReportsNotStored(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100, legContent)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteNote(context.Background(), noteRequest())

	if receipt.State != loop.NotStored || receipt.NodeID != 0 {
		t.Errorf("receipt = %+v, want notStored with no id", receipt)
	}
	want := []string{"POST /api/nodes", "POST /api/nodes/3100/content", "DELETE /api/nodes/3100"}
	if got := methodPaths(*calls); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v: the shell the failed content leaves behind is removed and no link is made", got, want)
	}
}

func TestWriteNoteDiscardsTheShellEvenWhenTheTurnIsCancelledBetweenCreateAndContent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var mu sync.Mutex
	var calls []recordedCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, recordedCall{Method: r.Method, Path: r.URL.Path})
		mu.Unlock()

		switch legOf(r) {
		case legCreate:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 3100})
		case legContent:
			cancel()
			w.WriteHeader(http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteNote(ctx, noteRequest())

	if receipt.State != loop.NotStored || receipt.NodeID != 0 {
		t.Errorf("receipt = %+v, want notStored with no id", receipt)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"POST /api/nodes", "POST /api/nodes/3100/content", "DELETE /api/nodes/3100"}
	if got := methodPaths(calls); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v: a cancelled turn must still remove the empty shell it created", got, want)
	}
}

func TestWriteNoteReportsNotStoredWhenTheCreateFailsAndMakesNoFurtherCall(t *testing.T) {
	t.Parallel()

	srv, calls := writeServer(t, 3100, legCreate)
	c := NewClient(srv.URL, "k", srv.Client(), testLogger())

	receipt := c.WriteNote(context.Background(), noteRequest())

	if receipt.State != loop.NotStored || len(*calls) != 1 {
		t.Errorf("receipt = %+v after %d calls, want notStored after the one create", receipt, len(*calls))
	}
}

func TestWriteNoteKeepsTheNodeAsUnlinkedWhenALinkFailsAndStillTriesTheOtherLink(t *testing.T) {
	t.Parallel()

	for name, failAt := range map[string]int{"the subject's link": 2, "the about's link": 3} {
		srv, calls := failingNodeServer(t, 3100, failAt)
		logger, logged := capturingLogger()
		c := NewClient(srv.URL, "k", srv.Client(), logger)

		receipt := c.WriteNote(context.Background(), noteRequest())

		if receipt.State != loop.Unlinked || receipt.NodeID != 3100 {
			t.Errorf("%s failing: receipt = %+v, want unlinked at the created id", name, receipt)
		}
		if len(*calls) != 4 {
			t.Errorf("%s failing: %d calls were made, want 4: one failed link must not stop the other", name, len(*calls))
		}
		for _, call := range *calls {
			if call.Method == http.MethodDelete {
				t.Errorf("%s failing: the node was deleted, want it kept as a repairable orphan", name)
			}
		}
		if !strings.Contains(logged.String(), logRepairableOrphan) {
			t.Errorf("%s failing: the log does not carry %q: %s", name, logRepairableOrphan, logged.String())
		}
	}
}
