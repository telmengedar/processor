package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestRunsStoresExactAttributedSoraSourceDespiteLossySummaryAndMatchesDirectOptions(t *testing.T) {
	t.Parallel()
	source := loop.NoteSource{Speaker: "Sora", Text: "SoraMemory_memory-host-173c7797-direct-20261023-01: I have run our neighbourhood seed exchange since 2021, on the first Saturday of every month. I keep it going because sharing locally adapted seeds helps neighbours grow food without spending much money."}
	summary := "Sora runs the neighbourhood seed exchange, which started in 2021, on the first Saturday of every month, to help neighbours grow food affordably using local seeds."
	encoded, _ := json.Marshal(source)
	raw, _ := json.Marshal(map[string]any{"action": "write", "text": summary, "about": nil})
	want := summary + "\n\nReported source evidence:\n" + string(encoded)
	graph := &configGraph{content: `{"note":{"type":"thought","mode":"decision"}}`, configRoot: 8800}
	notes := &recordingNotes{}
	model := &stubModel{results: []loop.JudgeResult{{Answer: string(raw), Reason: loop.Answered}}}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "test", testLogger())
	turn.Notes = notes
	body := `{"input":"private review","subject":42,"config":77,"noteSource":` + string(encoded) + `}`
	response := postRunsVerbose(t, turn, body)
	if response.Code != http.StatusOK || model.calls != 1 || len(notes.requests) != 1 {
		t.Fatalf("status=%d calls=%d writes=%d body=%s", response.Code, model.calls, len(notes.requests), response.Body.String())
	}
	request := notes.requests[0]
	if request.Text != want || request.Root != 8800 || request.Subject != 42 || request.Type != "thought" {
		t.Fatalf("stored=%+v", request)
	}
	var retained loop.NoteSource
	if err := json.Unmarshal([]byte(strings.SplitN(request.Text, "Reported source evidence:\n", 2)[1]), &retained); err != nil || retained != source || !strings.Contains(retained.Text, "locally adapted seeds") {
		t.Fatalf("lost original source: %+v err=%v", retained, err)
	}
	var record loop.Record
	if err := json.Unmarshal(response.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.NoteDecision.State != "stored" || record.NoteDecision.Created != 6100 || record.Answer != "" || record.Outcome.Produced || len(record.ToolCalls) != 1 {
		t.Fatalf("record=%+v", record)
	}
	if record.ToolCalls[0].Text != want || record.ToolCalls[0].Bytes != len(want) || record.ToolCalls[0].Written != loop.Stored || record.ToolCalls[0].Created != 6100 || !strings.Contains(response.Body.String(), `"reply":""`) {
		t.Fatalf("receipt/body mismatch: %s", response.Body.String())
	}
	model.results = []loop.JudgeResult{{Answer: string(raw), Reason: loop.Answered}}
	notes.requests = nil
	direct, _, err := turn.Answer(t.Context(), loop.RunOptions{Input: "private review", Subject: 42, Settings: loop.DefaultSettings(), View: &loop.View{Root: 8800, Note: &loop.NoteConfig{Type: "thought", Mode: loop.NoteModeDecision}}, NoteSource: &source})
	if err != nil || len(notes.requests) != 1 || notes.requests[0] != request || direct.ToolCalls[0].Text != want {
		t.Fatalf("direct disagreement: err=%v notes=%+v record=%+v", err, notes.requests, direct)
	}
}

func TestRunsRejectsMalformedOversizedAndIncompatibleSourceBeforeSubjectOrInference(t *testing.T) {
	t.Parallel()
	cases := []string{`null`, `[]`, `{}`, `{"speaker":7,"text":"x"}`, `{"speaker":"Sora","text":""}`, `{"speaker":" ","text":"x"}`, `{"speaker":"Sora","text":"\ud800"}`, `{"speaker":"\udc00","text":"x"}`, "{\"speaker\":\"Sora\",\"text\":\"\xff\"}", `{"speaker":"` + strings.Repeat("é", 129) + `","text":"x"}`, `{"speaker":"Sora","text":"` + strings.Repeat("x", 8193) + `"}`}
	for _, config := range []string{`{"note":{"type":"thought","mode":"decision"}}`, `{"note":{"type":"thought"}}`, `{}`} {
		inputs := append([]string{}, cases...)
		if !strings.Contains(config, `"decision"`) {
			inputs = append(inputs, `{"speaker":"Sora","text":"source"}`)
		}
		for _, input := range inputs {
			graph := &configGraph{content: config}
			model := &countingModel{}
			notes := &recordingNotes{}
			turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "test", testLogger())
			turn.Notes = notes
			response := postRunsVerbose(t, turn, `{"input":"review","subject":42,"config":77,"noteSource":`+input+`}`)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"invalid_request"`) {
				t.Fatalf("input=%q config=%s status=%d body=%s", input, config, response.Code, response.Body.String())
			}
			if len(graph.nodeCalls) != 1 || graph.nodeCalls[0].id != 77 || len(graph.recallCalls) != 0 || model.calls() != 0 || len(notes.requests) != 0 || len(graph.written) != 0 {
				t.Fatalf("invalid source reached run: nodes=%+v calls=%d notes=%+v", graph.nodeCalls, model.calls(), notes.requests)
			}
		}
	}
}
