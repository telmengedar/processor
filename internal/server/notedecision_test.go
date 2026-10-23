package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestPrivateNoteDecisionResponsesContainReceiptsButNeverChatOrFallback(t *testing.T) {
	t.Parallel()
	for _, verbose := range []bool{false, true} {
		for _, raw := range []string{`{"action":"write","text":"my private thought","about":null}`, `{"action":"skip","text":"","about":null}`, `write_note("a fake call")`, `{"action":"skip","text":"","about":777001}`} {
			graph := &configGraph{content: `{"note":{"type":"thought","mode":"decision"},"identity":"private policy"}`, configRoot: 8800}
			notes := &recordingNotes{}
			model := &stubModel{results: []loop.JudgeResult{{Answer: raw, Reason: loop.Answered, RawReason: "stop"}}}
			turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
			turn.Notes = notes
			target := "/runs"
			if verbose {
				target += "?verbose=true"
			}
			rec := postRunsAt(t, t.Context(), turn, target, configPostBodyHead+`77}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if string(body["reply"]) != `""` || string(body["produced"]) != "false" {
				t.Fatalf("private reply escaped: %s", rec.Body.String())
			}
			var receipt loop.NoteDecision
			if err := json.Unmarshal(body["noteDecision"], &receipt); err != nil {
				t.Fatal(err)
			}
			want := "rejected"
			if raw == `{"action":"write","text":"my private thought","about":null}` {
				want = "stored"
			}
			if raw == `{"action":"skip","text":"","about":null}` {
				want = "abstained"
			}
			if receipt.State != want || (len(notes.requests) == 1) != (want == "stored") || model.calls != 1 {
				t.Fatalf("receipt=%+v writes=%d calls=%d", receipt, len(notes.requests), model.calls)
			}
			if want == "stored" && receipt.Created != 6100 {
				t.Fatalf("created=%d", receipt.Created)
			}
			if verbose {
				if string(body["answer"]) != `""` || string(body["modelCalls"]) != "1" {
					t.Fatalf("verbose private output=%s", rec.Body.String())
				}
			}
		}
	}
}

func TestPrivateNoteDecisionModelFailureReturnsGatewayErrorWithoutWriting(t *testing.T) {
	t.Parallel()
	graph := &configGraph{content: `{"note":{"type":"thought","mode":"decision"}}`}
	model := &stubModel{err: errors.New("upstream failed")}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "test", testLogger())
	notes := &recordingNotes{}
	turn.Notes = notes
	rec := postRunsVerbose(t, turn, configPostBodyHead+`77}`)
	if rec.Code != http.StatusBadGateway || len(notes.requests) != 0 || len(graph.written) != 0 {
		t.Fatalf("code=%d writes=%d records=%d", rec.Code, len(notes.requests), len(graph.written))
	}
}
