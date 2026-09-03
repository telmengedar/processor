package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/server"
)

const storedNodeID = 10525

type answeringModel struct{}

func (answeringModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	return loop.JudgeResult{
		Answer:    "the answer the model gave",
		Reason:    loop.Answered,
		RawReason: "stop",
		Usage:     &loop.Usage{InTokens: 11, OutTokens: 22},
	}, nil
}

type storedBody struct {
	mu   sync.Mutex
	body []byte
}

func (s *storedBody) set(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body = b
}

func (s *storedBody) get() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.body
}

type authRecorder struct {
	mu     sync.Mutex
	header string
}

func (a *authRecorder) record(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.header = r.Header.Get("Authorization")
}

func (a *authRecorder) get() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.header
}

func graphServer(t *testing.T, stored *storedBody, failing string) (*httptest.Server, *authRecorder) {
	t.Helper()

	auth := &authRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.record(r)

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("id") != "" {
				_, _ = w.Write([]byte(`{"result":[{"id":42,"type":"documentation","name":"Subject","content":"anchor body"}],"total":1}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":[{"id":7,"type":"task","name":"Cand","similarity":0.5,"content":"candidate body"}],"total":1}`))
			return
		}

		switch {
		case r.URL.Path == "/api/nodes":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": storedNodeID})
		case strings.HasSuffix(r.URL.Path, "/content"):
			body, _ := io.ReadAll(r.Body)
			stored.set(body)
			if failing == "content" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			if failing == "link" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, auth
}

func runOneTurn(t *testing.T, failing string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()

	stored := &storedBody{}
	graphSrv, _ := graphServer(t, stored, failing)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	graph := divoid.NewClient(graphSrv.URL, "k", graphSrv.Client(), logger)
	turn := loop.NewTurn(graph, answeringModel{}, systemText, "test-model-id", logger)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewBufferString(`{"input":"what changed","subject":42}`))
	server.NewHandler(turn).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	return rec, stored.get()
}

func topLevelMembers(t *testing.T, b []byte) ([]string, map[string]string) {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(b))
	open, err := dec.Token()
	if err != nil {
		t.Fatalf("read opening token: %v; body=%s", err, b)
	}
	if delim, ok := open.(json.Delim); !ok || delim != '{' {
		t.Fatalf("body does not open with an object: %v; body=%s", open, b)
	}

	var order []string
	values := map[string]string{}
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			t.Fatalf("read key token: %v; body=%s", err, b)
		}
		key, ok := keyToken.(string)
		if !ok {
			t.Fatalf("object key is not a string: %v; body=%s", keyToken, b)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatalf("read value for %q: %v; body=%s", key, err, b)
		}
		order = append(order, key)
		values[key] = string(raw)
	}
	return order, values
}

func TestTheStoredBodyIsTheResponseBodyMinusTheWriteReceiptAndNothingElse(t *testing.T) {
	t.Parallel()

	rec, stored := runOneTurn(t, "")
	if len(stored) == 0 {
		t.Fatal("the graph never received a content body, so this test compared nothing")
	}

	responseOrder, responseValues := topLevelMembers(t, rec.Body.Bytes())
	storedOrder, storedValues := topLevelMembers(t, stored)

	const receiptKey = "written"

	var withoutReceipt []string
	for _, key := range responseOrder {
		if key == receiptKey {
			continue
		}
		withoutReceipt = append(withoutReceipt, key)
	}

	if len(withoutReceipt) != len(responseOrder)-1 {
		t.Fatalf("the response has no %q key at all; keys=%v", receiptKey, responseOrder)
	}
	if len(storedOrder) != len(withoutReceipt) {
		t.Fatalf("stored keys %v, response keys minus %q %v — the two differ by more than the one key", storedOrder, receiptKey, withoutReceipt)
	}
	for i, key := range withoutReceipt {
		if storedOrder[i] != key {
			t.Fatalf("stored key[%d] = %q, response key[%d] = %q — the two bodies carry different keys or a different order", i, storedOrder[i], i, key)
		}
		if storedValues[key] != responseValues[key] {
			t.Fatalf("key %q differs: stored %s, response %s", key, storedValues[key], responseValues[key])
		}
	}
}

func TestTheStoredBodyCarriesNoWriteReceiptWhileTheResponseDoes(t *testing.T) {
	t.Parallel()

	rec, stored := runOneTurn(t, "")

	_, storedValues := topLevelMembers(t, stored)
	if _, present := storedValues["written"]; present {
		t.Fatalf("stored body carries a written key: %s", stored)
	}

	_, responseValues := topLevelMembers(t, rec.Body.Bytes())
	const wantReceipt = `{"state":"stored","nodeId":10525}`
	if responseValues["written"] != wantReceipt {
		t.Fatalf("response written = %s, want %s", responseValues["written"], wantReceipt)
	}
}

func TestAnUnlinkedRecordIsStoredWholeAndTheResponseNamesTheNodeHoldingIt(t *testing.T) {
	t.Parallel()

	rec, stored := runOneTurn(t, "link")

	_, responseValues := topLevelMembers(t, rec.Body.Bytes())
	const wantReceipt = `{"state":"unlinked","nodeId":10525}`
	if responseValues["written"] != wantReceipt {
		t.Fatalf("response written = %s, want %s", responseValues["written"], wantReceipt)
	}

	_, storedValues := topLevelMembers(t, stored)
	if storedValues["answer"] != `"the answer the model gave"` {
		t.Fatalf("stored answer = %s, want the complete record kept despite the missing edge", storedValues["answer"])
	}
	if _, present := storedValues["written"]; present {
		t.Fatalf("stored body carries a written key: %s", stored)
	}
}

func TestAnUnfiledRecordStillReachesTheCallerWithTheFateNamed(t *testing.T) {
	t.Parallel()

	rec, _ := runOneTurn(t, "content")

	_, responseValues := topLevelMembers(t, rec.Body.Bytes())
	const wantReceipt = `{"state":"notStored"}`
	if responseValues["written"] != wantReceipt {
		t.Fatalf("response written = %s, want %s — no node id, because no node holds the record", responseValues["written"], wantReceipt)
	}
	if responseValues["answer"] != `"the answer the model gave"` {
		t.Fatalf("response answer = %s, want the record delivered to the caller as the only surviving copy", responseValues["answer"])
	}
}
