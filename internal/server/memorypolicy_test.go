package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestAnswer_ResolvedRequestConfig_LogsOnlyEffectiveMemoryMetadata(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		config  string
		request string
		status  int
		want    map[string]any
	}{
		{
			name:    "private decision",
			config:  `{"identity":"secret-policy-prompt","note":{"type":"thought","mode":"decision"},"filter":{"type":["thought","knowledge"],"name":"secret-filter-value"}}`,
			request: `,"config":77,"noteSource":{"speaker":"secret-speaker","text":"secret-source-text"}`,
			status:  http.StatusOK,
			want:    map[string]any{"memoryEnabled": true, "config": float64(77), "noteType": "thought", "filterTypes": []any{"thought", "knowledge"}},
		},
		{
			name:    "custom note type",
			config:  `{"note":{"type":"custom-memory"},"filter":{"type":"custom-memory"}}`,
			request: `,"config":77`,
			status:  http.StatusOK,
			want:    map[string]any{"memoryEnabled": true, "config": float64(77), "noteType": "custom-memory", "filterTypes": []any{"custom-memory"}},
		},
		{
			name:    "read only policy",
			config:  `{"filter":{"type":["knowledge","thought"]}}`,
			request: `,"config":77`,
			status:  http.StatusOK,
			want:    map[string]any{"memoryEnabled": false, "config": float64(77), "noteType": "", "filterTypes": []any{"knowledge", "thought"}},
		},
		{
			name:    "unfiltered note policy",
			config:  `{"note":{"type":"thought"}}`,
			request: `,"config":77`,
			status:  http.StatusOK,
			want:    map[string]any{"memoryEnabled": true, "config": float64(77), "noteType": "thought", "filterTypes": []any{}},
		},
		{
			name:   "no selected policy",
			config: `{"note":{"type":"unused-policy"}}`,
			status: http.StatusOK,
			want:   map[string]any{"memoryEnabled": false, "noteType": "", "filterTypes": []any{}},
		},
		{
			name:    "malformed policy",
			config:  `{"note":{"type":"thought"},"filter":{"type":"knowledge"}}`,
			request: `,"config":77`,
			status:  http.StatusBadRequest,
		},
		{
			name:    "missing policy",
			request: `,"config":99`,
			status:  http.StatusNotFound,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			graph := &configGraph{content: test.config}
			model := &stubModel{results: []loop.JudgeResult{{Reason: loop.Answered, Answer: `{"action":"skip","text":"","about":null}`}}}
			var logged bytes.Buffer
			turn := loop.NewTurn(graph, model, nil, fixedSystem("secret-system-prompt"), "secret-model-key", slog.New(slog.NewJSONHandler(&logged, nil)))
			turn.Notes = &recordingNotes{}
			response := postRunsVerbose(t, turn, `{"input":"secret-user-text","subject":42,"history":[{"input":"secret-history-text","reply":"secret-history-reply"}]`+test.request+`}`)
			if response.Code != test.status {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if test.request != "" && (len(graph.nodeCalls) == 0 || !reflect.DeepEqual(graph.nodeCalls[0].filter, loop.Filter{"type": {"processor-config"}})) {
				t.Fatalf("config was not read first: %+v", graph.nodeCalls)
			}
			markers := 0
			outcomeMarkers := 0
			policyID, outcomeID := "", ""
			decoder := json.NewDecoder(&logged)
			for decoder.More() {
				var entry map[string]any
				if err := decoder.Decode(&entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] == "run memory outcome" {
					outcomeMarkers++
					outcomeID, _ = entry["runID"].(string)
					encoded, _ := json.Marshal(entry)
					if strings.Contains(string(encoded), "secret-") || strings.Contains(string(encoded), "anchor body") {
						t.Fatalf("outcome contains private content: %s", encoded)
					}
				}
				if entry["msg"] != "run memory policy" {
					continue
				}
				markers++
				encoded, _ := json.Marshal(entry)
				if strings.Contains(string(encoded), "secret-") || strings.Contains(string(encoded), "anchor body") {
					t.Fatalf("marker contains private content: %s", encoded)
				}
				policyID, _ = entry["runID"].(string)
				if len(policyID) < 20 {
					t.Fatalf("missing opaque run ID: %v", entry)
				}
				delete(entry, "runID")
				delete(entry, "time")
				delete(entry, "level")
				delete(entry, "msg")
				if !reflect.DeepEqual(entry, test.want) {
					t.Fatalf("marker=%v, want %v", entry, test.want)
				}
			}
			wantMarkers := 1
			if test.status != http.StatusOK {
				wantMarkers = 0
				if model.calls != 0 {
					t.Fatalf("unresolved config reached model: %d calls", model.calls)
				}
			}
			if outcomeMarkers != wantMarkers || (wantMarkers == 1 && policyID != outcomeID) {
				t.Fatalf("policyID=%q outcomeID=%q outcomes=%d want=%d", policyID, outcomeID, outcomeMarkers, wantMarkers)
			}
			if markers != wantMarkers {
				t.Fatalf("markers=%d, want %d", markers, wantMarkers)
			}
		})
	}
}
