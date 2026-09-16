package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const legacyRecordJSON = `{"input":"do the thing","subject":10422,"query":"","queries":["do the thing"],"anchor":{"id":10422,"type":"project","name":"processor","size":100},"candidates":[],"block":"","answer":"done","model":"m1","provider":{"adapter":"a","endpoint":"e"},"toolCalls":[],"modelCalls":1,"capReached":false,"usage":null,"stopReason":{"reason":"answered","raw":"stop"},"limits":{"candidateLimit":20,"assemblyByteBudget":60000,"supplementaryByteBudget":20000,"maxModelCalls":6,"maxOutputTokens":4096},"sampling":{}}`

type fakeNode struct {
	Type, Name, ContentType, Content, Substance string
}

type fakeGraphServer struct {
	mu               sync.Mutex
	nodes            map[int64]fakeNode
	contentPosts     map[int64]string
	contentPostTypes map[int64]string
	substancePatches map[int64]string
}

func newFakeGraphServer(nodes map[int64]fakeNode) *fakeGraphServer {
	return &fakeGraphServer{
		nodes:            nodes,
		contentPosts:     map[int64]string{},
		contentPostTypes: map[int64]string{},
		substancePatches: map[int64]string{},
	}
}

func (s *fakeGraphServer) start(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (s *fakeGraphServer) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/nodes":
		s.serveGet(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/content"):
		s.serveContentPost(w, r)
	case r.Method == http.MethodPatch:
		s.servePatch(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *fakeGraphServer) serveGet(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	node, found := s.nodes[id]
	w.Header().Set("Content-Type", "application/json")
	if !found {
		fmt.Fprint(w, `{"result":[],"total":0}`)
		return
	}
	fmt.Fprintf(w, `{"result":[{"id":%d,"type":%q,"name":%q,"contentType":%q,"content":%q,"substance":%q}],"total":1}`,
		id, node.Type, node.Name, node.ContentType, node.Content, node.Substance)
}

func (s *fakeGraphServer) serveContentPost(w http.ResponseWriter, r *http.Request) {
	id := idFromPath(r.URL.Path, "/content")
	body, _ := io.ReadAll(r.Body)
	s.contentPosts[id] = string(body)
	s.contentPostTypes[id] = r.Header.Get("Content-Type")
	n := s.nodes[id]
	n.Content = string(body)
	s.nodes[id] = n
	w.WriteHeader(http.StatusOK)
}

func (s *fakeGraphServer) servePatch(w http.ResponseWriter, r *http.Request) {
	id := idFromPath(r.URL.Path, "")
	var ops []struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value"`
	}
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &ops)
	for _, op := range ops {
		if op.Path == "/substance" {
			s.substancePatches[id] = op.Value
			n := s.nodes[id]
			n.Substance = op.Value
			s.nodes[id] = n
		}
	}
	w.WriteHeader(http.StatusOK)
}

func idFromPath(path, suffix string) int64 {
	trimmed := strings.TrimPrefix(strings.TrimSuffix(path, suffix), "/api/nodes/")
	id, _ := strconv.ParseInt(trimmed, 10, 64)
	return id
}

func bootEnv(t *testing.T, graphURL string) {
	t.Helper()
	t.Setenv("PROCESSOR_DIVOID_URL", graphURL)
	t.Setenv("PROCESSOR_DIVOID_KEY", "a-key")
}

func runBackfillMain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()

	var machine, human strings.Builder
	code := run(args, &machine, &human)
	return code, machine.String(), human.String()
}

func TestParseFlagsRequiresBackupDirForALiveRunButNotForADryRunOrABackedUpRun(t *testing.T) {
	var human strings.Builder
	if _, ok := parseFlags([]string{"-ids", "1"}, &human); ok {
		t.Fatalf("want a live run with no -backup-dir rejected as usage")
	}

	human.Reset()
	opts, ok := parseFlags([]string{"-ids", "1", "-dry-run"}, &human)
	if !ok {
		t.Fatalf("want a dry run with no -backup-dir accepted, got: %s", human.String())
	}
	if opts.backupDir != "" {
		t.Fatalf("want backupDir empty for a dry run, got %q", opts.backupDir)
	}

	human.Reset()
	opts, ok = parseFlags([]string{"-ids", "1", "-backup-dir", "some-dir"}, &human)
	if !ok {
		t.Fatalf("want a live run carrying -backup-dir accepted, got: %s", human.String())
	}
	if opts.backupDir != "some-dir" {
		t.Fatalf("want backupDir %q, got %q", "some-dir", opts.backupDir)
	}
}

func TestParseIDs(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []int64
		wantErr bool
	}{
		{name: "single id", raw: "10897", want: []int64{10897}},
		{name: "multiple ids", raw: "10897,10898,11384", want: []int64{10897, 10898, 11384}},
		{name: "whitespace around ids", raw: " 10897 , 10898 ", want: []int64{10897, 10898}},
		{name: "trailing comma is a skipped empty field, not an error", raw: "10897,", want: []int64{10897}},
		{name: "empty string is rejected", raw: "", wantErr: true},
		{name: "whitespace-only string is rejected", raw: "   ", wantErr: true},
		{name: "commas with nothing between them name no id", raw: ",,", wantErr: true},
		{name: "one non-numeric field is rejected", raw: "10897,notanid", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIDs(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseIDs(%q): want an error, got %v", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIDs(%q): unexpected error: %v", tc.raw, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseIDs(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseIDs(%q) = %v, want %v", tc.raw, got, tc.want)
				}
			}
		})
	}
}

func TestALiveRunWithoutBackupDirIsRejectedBeforeAnyNetworkCall(t *testing.T) {
	code, _, human := runBackfillMain(t, "-ids", "10897")

	if code != exitUsage {
		t.Fatalf("want exit %d, got %d: %s", exitUsage, code, human)
	}
	if !strings.Contains(human, "-backup-dir is required") {
		t.Fatalf("want the refusal reason in the report, got: %s", human)
	}
}

func TestADryRunNeedsNoBackupDirAndWritesNothingToTheGraph(t *testing.T) {
	server := newFakeGraphServer(map[int64]fakeNode{
		10897: {Type: "session-log", Name: `processor-run 2026-09-02T11:35:08Z — do the thing`, ContentType: "application/json", Content: legacyRecordJSON},
	})
	bootEnv(t, server.start(t))

	code, _, _ := runBackfillMain(t, "-ids", "10897", "-dry-run")

	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.contentPosts) != 0 || len(server.substancePatches) != 0 {
		t.Fatalf("a dry run must issue no write, got content=%v substance=%v", server.contentPosts, server.substancePatches)
	}
}

func TestALiveRunBacksUpBeforeWritingAndBackfillsTheNode(t *testing.T) {
	server := newFakeGraphServer(map[int64]fakeNode{
		10897: {Type: "session-log", Name: `processor-run 2026-09-02T11:35:08Z — do the thing`, ContentType: "application/json", Content: legacyRecordJSON},
	})
	bootEnv(t, server.start(t))
	backupDir := t.TempDir()

	code, _, human := runBackfillMain(t, "-ids", "10897", "-backup-dir", backupDir)

	if code != 0 {
		t.Fatalf("want exit 0, got %d: %s", code, human)
	}

	backupBytes, err := os.ReadFile(filepath.Join(backupDir, "10897.json"))
	if err != nil {
		t.Fatalf("backup file was not written: %v", err)
	}
	var backedUp backupRecord
	if err := json.Unmarshal(backupBytes, &backedUp); err != nil {
		t.Fatalf("backup file did not decode: %v", err)
	}
	if backedUp.Content != legacyRecordJSON {
		t.Fatalf("backed-up content = %q, want the original bytes", backedUp.Content)
	}
	if backedUp.ContentType != "application/json" {
		t.Fatalf("backed-up content type = %q, want the original", backedUp.ContentType)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.contentPostTypes[10897] != "text/markdown; charset=utf-8" {
		t.Fatalf("posted content type = %q, want WriteRun's own", server.contentPostTypes[10897])
	}
	if !strings.Contains(server.contentPosts[10897], legacyRecordJSON) {
		t.Fatalf("posted content does not carry the original record bytes verbatim: %s", server.contentPosts[10897])
	}
	if server.substancePatches[10897] == "" {
		t.Fatalf("want a non-empty substance patch")
	}
}
