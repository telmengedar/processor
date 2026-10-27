package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestMemoryOutcomesJoinPolicyAndExcludePrivateContent(t *testing.T) {
	const private = "PRIVATE_SENTINEL_3f7d1"
	cases := []struct {
		name            string
		view            *View
		result          JudgeResult
		receipt         WriteReceipt
		modelError      error
		outcome, status string
		writes          int
		node            int64
	}{
		{name: "stored", view: decisionView(), result: decisionResult(`{"action":"write","text":"` + private + `","about":null}`), receipt: WriteReceipt{State: Stored, NodeID: 901}, outcome: "write", status: "stored", writes: 1, node: 901},
		{name: "unlinked", view: decisionView(), result: decisionResult(`{"action":"write","text":"` + private + `","about":null}`), receipt: WriteReceipt{State: Unlinked, NodeID: 902}, outcome: "write", status: "unlinked", writes: 1, node: 902},
		{name: "storage failure", view: decisionView(), result: decisionResult(`{"action":"write","text":"` + private + `","about":null}`), receipt: WriteReceipt{State: NotStored}, outcome: "error", status: "notStored", writes: 1},
		{name: "invalid receipt", view: decisionView(), result: decisionResult(`{"action":"write","text":"` + private + `","about":null}`), receipt: WriteReceipt{State: Stored}, outcome: "error", status: "notStored", writes: 1},
		{name: "skip", view: decisionView(), result: decisionResult(`{"action":"skip","text":"","about":null}`), outcome: "skip", status: "abstained"},
		{name: "invalid", view: decisionView(), result: decisionResult(private), outcome: "invalid", status: "rejected"},
		{name: "refused", view: decisionView(), result: decisionResult(`{"action":"write","text":"processor-run ` + private + `","about":null}`), outcome: "invalid", status: "rejected"},
		{name: "model failure", view: decisionView(), modelError: errors.New("offline failure"), outcome: "error", status: "run_failed"},
		{name: "ordinary reply", view: noteView(), result: JudgeResult{Answer: "A normal reply.", Reason: Answered}, outcome: "skip", status: "not_requested"},
		{name: "disabled", view: &View{Config: 10}, result: JudgeResult{Answer: "A normal reply.", Reason: Answered}, outcome: "skip", status: "disabled"},
		{name: "tool write", view: noteView(), result: wantsNoteOf(private, 0), receipt: WriteReceipt{State: Stored, NodeID: 903}, outcome: "write", status: "stored", writes: 1, node: 903},
	}
	ids := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			model := &fakeModel{results: []JudgeResult{tc.result, answeredFinal()}, err: tc.modelError}
			graph := noteGraph()
			graph.node.Content = private
			tc.view.Identity = private
			notes := &fakeNotes{receipt: tc.receipt}
			turn := NewTurn(graph, model, nil, fixedSystem(private), "test", slog.New(slog.NewJSONHandler(&logs, nil)))
			turn.Notes = notes
			record, _, err := turn.Answer(context.Background(), RunOptions{Input: private, Subject: noteSubject, History: []Exchange{{Input: private, Reply: private}}, Settings: DefaultSettings(), View: tc.view})
			if (err != nil) != (tc.modelError != nil) || len(notes.requests) != tc.writes {
				t.Fatalf("err=%v writes=%d", err, len(notes.requests))
			}
			if tc.writes > 0 && !strings.Contains(notes.requests[0].Text, private) {
				t.Fatal("private write scenario did not reach storage")
			}
			if tc.name == "ordinary reply" && record.Answer != "A normal reply." {
				t.Fatal("ordinary reply changed")
			}
			var policy, outcome map[string]any
			count := 0
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var event map[string]any
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				switch event["msg"] {
				case "run memory policy":
					policy = event
				case "run memory outcome":
					outcome = event
					count++
				}
			}
			if count != 1 || policy == nil || outcome == nil {
				t.Fatalf("policy=%v outcome=%v count=%d", policy, outcome, count)
			}
			id, ok := policy["runID"].(string)
			if !ok || len(id) < 20 || outcome["runID"] != id || ids[id] {
				t.Fatalf("invalid correlation: %v %v", policy, outcome)
			}
			ids[id] = true
			if policy["config"] != float64(tc.view.Config) || policy["memoryEnabled"] != (tc.view.Note != nil) || outcome["outcome"] != tc.outcome || outcome["status"] != tc.status || outcome["noteType"] != policy["noteType"] {
				t.Fatalf("policy=%v outcome=%v", policy, outcome)
			}
			if tc.node > 0 {
				if outcome["node"] != float64(tc.node) {
					t.Fatalf("missing receipt: %v", outcome)
				}
			} else if _, exists := outcome["node"]; exists {
				t.Fatalf("false receipt: %v", outcome)
			}
			for key := range outcome {
				switch key {
				case "time", "level", "msg", "runID", "outcome", "status", "noteType", "node":
				default:
					t.Fatalf("unexpected outcome field %q", key)
				}
			}
			if strings.Contains(logs.String(), private) {
				t.Fatal("private sentinel leaked into logs")
			}
		})
	}
}

type inspectingNotes struct {
	inspect func()
}

func (n inspectingNotes) WriteNote(context.Context, NoteRequest) WriteReceipt {
	n.inspect()
	return WriteReceipt{State: Stored, NodeID: 904}
}

func TestMemoryStorageOutcomeWaitsForReceiptAndDoesNotCarryIntoTheNextRun(t *testing.T) {
	var logs bytes.Buffer
	model := &fakeModel{results: []JudgeResult{decisionResult(`{"action":"write","text":"private thought","about":null}`), decisionResult(`{"action":"skip","text":"","about":null}`)}}
	turn := NewTurn(noteGraph(), model, nil, fixedSystem("system"), "test", slog.New(slog.NewJSONHandler(&logs, nil)))
	called := false
	turn.Notes = inspectingNotes{inspect: func() {
		called = true
		if !strings.Contains(logs.String(), `"msg":"run memory policy"`) || strings.Contains(logs.String(), `"msg":"run memory outcome"`) {
			t.Fatal("outcome emitted before storage returned")
		}
	}}
	options := RunOptions{Input: "review", Subject: noteSubject, Settings: DefaultSettings(), View: decisionView()}
	for i := 0; i < 2; i++ {
		if _, _, err := turn.Answer(t.Context(), options); err != nil {
			t.Fatal(err)
		}
	}
	if !called || turn.memoryOutcome != nil {
		t.Fatal("storage not exercised or shared turn mutated")
	}
	if strings.Count(logs.String(), `"msg":"run memory outcome"`) != 2 || strings.Count(logs.String(), `"status":"stored"`) != 1 || strings.Count(logs.String(), `"status":"abstained"`) != 1 {
		t.Fatalf("wrong run outcomes: %s", logs.String())
	}
}

func TestMemoryOutcomeCoversDuplicateUnavailableAndEarlyFailures(t *testing.T) {
	for _, scenario := range []string{"duplicate", "unavailable", "source overflow", "invalid config", "missing subject", "tool invalid", "tool not offered", "repeated tool"} {
		t.Run(scenario, func(t *testing.T) {
			var logs bytes.Buffer
			graph := noteGraph()
			notes := &fakeNotes{}
			view := decisionView()
			result := copyDecision(admittedNoteText, 71)
			var source *NoteSource
			wantOutcome, wantStatus := "error", "run_failed"
			switch scenario {
			case "duplicate":
				view = copyDecisionView()
				graph.candidates = []Candidate{scopedCopyCandidate()}
				wantOutcome, wantStatus = "skip", "duplicate"
			case "unavailable":
				wantStatus = "unavailable"
			case "source overflow":
				source = &NoteSource{Speaker: "speaker", Text: strings.Repeat("x", 8192)}
				result = copyDecision(strings.Repeat("x", 20*1024), noteSubject)
				wantOutcome, wantStatus = "invalid", "rejected"
			case "invalid config":
				view.ResponseMode = "invalid"
			case "missing subject":
				graph.nodeFound = false
			case "tool invalid":
				view = noteView()
				result = wantsNoteOf("private", 0)
				result.ToolError = "private invalid payload"
				wantOutcome, wantStatus = "invalid", "rejected"
			case "tool not offered":
				view = &View{Config: 11}
				result = wantsNoteOf("private", 0)
				wantOutcome, wantStatus = "invalid", "not_offered"
			case "repeated tool":
				view = noteView()
				result = wantsNoteOf("private", 0)
				wantOutcome, wantStatus = "write", "stored"
			}
			results := []JudgeResult{result, answeredFinal()}
			if scenario == "repeated tool" {
				results = []JudgeResult{result, result, answeredFinal()}
			}
			model := &fakeModel{results: results, ignoresWithheldToolList: true}
			turn := NewTurn(graph, model, nil, fixedSystem("system"), "test", slog.New(slog.NewJSONHandler(&logs, nil)))
			if scenario != "unavailable" {
				turn.Notes = notes
			}
			_, _, err := turn.Answer(t.Context(), RunOptions{Input: "review", Subject: noteSubject, Settings: DefaultSettings(), View: view, NoteSource: source})
			wantError := scenario == "invalid config" || scenario == "missing subject"
			if (err != nil) != wantError {
				t.Fatalf("error=%v", err)
			}
			if scenario == "repeated tool" {
				if len(notes.requests) != 1 {
					t.Fatalf("writes=%d", len(notes.requests))
				}
			} else if len(notes.requests) != 0 {
				t.Fatal("unexpected storage")
			}
			if !strings.Contains(logs.String(), `"outcome":"`+wantOutcome+`"`) || !strings.Contains(logs.String(), `"status":"`+wantStatus+`"`) || strings.Count(logs.String(), `"msg":"run memory policy"`) != 1 {
				t.Fatalf("wrong outcome: %s", logs.String())
			}
		})
	}
}

func TestMemoryReservedNoteRefusalIsInvalidAndPreservesConfirmedStorage(t *testing.T) {
	for _, scenario := range []string{"enabled", "disabled", "stored", "unlinked"} {
		t.Run(scenario, func(t *testing.T) {
			const private = "PRIVATE_RESERVED_NOTE_SENTINEL"
			var logs bytes.Buffer
			view := noteView()
			settings := DefaultSettings()
			settings.MaxModelCalls = 1
			result := wantsNoteOf(private, 0)
			results := []JudgeResult{result}
			notes := &fakeNotes{}
			wantOutcome, wantStatus := "invalid", "not_offered"
			if scenario == "disabled" {
				view = &View{Config: 11}
			}
			if scenario == "stored" || scenario == "unlinked" {
				settings.MaxModelCalls = 2
				results = append(results, result)
				notes.receipt = WriteReceipt{State: WriteState(scenario), NodeID: 905}
				wantOutcome, wantStatus = "write", scenario
			}
			model := &fakeModel{results: results, ignoresWithheldToolList: true}
			turn := NewTurn(noteGraph(), model, nil, fixedSystem("system"), "test", slog.New(slog.NewJSONHandler(&logs, nil)))
			turn.Notes = notes
			record, _, err := turn.Answer(t.Context(), RunOptions{Input: "review", Subject: noteSubject, Settings: settings, View: view})
			if err != nil {
				t.Fatal(err)
			}
			if len(model.calls) != settings.MaxModelCalls || len(model.calls[len(model.calls)-1].Offered) != 0 || len(record.ToolCalls) != settings.MaxModelCalls || record.ToolCalls[len(record.ToolCalls)-1].Error != errReservedCallRefused {
				t.Fatalf("reserved refusal not reached: calls=%+v rounds=%+v", model.calls, record.ToolCalls)
			}
			if len(notes.requests) != settings.MaxModelCalls-1 {
				t.Fatalf("writes=%d", len(notes.requests))
			}
			var outcome map[string]any
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var event map[string]any
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
				if event["msg"] == "run memory outcome" {
					outcome = event
				}
			}
			if outcome["outcome"] != wantOutcome || outcome["status"] != wantStatus {
				t.Fatalf("outcome=%v want=%s/%s", outcome, wantOutcome, wantStatus)
			}
			if settings.MaxModelCalls == 2 {
				if outcome["node"] != float64(905) {
					t.Fatalf("lost storage receipt: %v", outcome)
				}
			} else if _, exists := outcome["node"]; exists {
				t.Fatalf("false storage receipt: %v", outcome)
			}
			if strings.Contains(logs.String(), private) {
				t.Fatal("private note leaked")
			}
		})
	}
}

func TestMemoryFailedStorageSurvivesLaterUnofferedNoteRequests(t *testing.T) {
	for _, reserved := range []bool{false, true} {
		t.Run(map[bool]string{false: "unoffered", true: "reserved"}[reserved], func(t *testing.T) {
			const private = "PRIVATE_FAILED_NOTE_SENTINEL"
			var logs bytes.Buffer
			settings := DefaultSettings()
			if reserved {
				settings.MaxModelCalls = 2
			}
			result := wantsNoteOf(private, 0)
			model := &fakeModel{results: []JudgeResult{result, result, answeredFinal()}, ignoresWithheldToolList: true}
			notes := &fakeNotes{receipt: WriteReceipt{State: NotStored}}
			turn := NewTurn(noteGraph(), model, nil, fixedSystem("system"), "test", slog.New(slog.NewJSONHandler(&logs, nil)))
			turn.Notes = notes
			record, _, err := turn.Answer(t.Context(), RunOptions{Input: "review", Subject: noteSubject, Settings: settings, View: noteView()})
			if err != nil {
				t.Fatal(err)
			}
			if len(notes.requests) != 1 || len(record.ToolCalls) != 2 || record.ToolCalls[0].Written != NotStored || record.ToolCalls[0].Error != errNoteNotStored {
				t.Fatalf("failed storage scenario not reached: writes=%d rounds=%+v", len(notes.requests), record.ToolCalls)
			}
			wantRefusal := errNoteNotOffered
			if reserved {
				wantRefusal = errReservedCallRefused
			}
			if offersNote(model.calls[1].Offered) || record.ToolCalls[1].Error != wantRefusal {
				t.Fatalf("second note not refused: calls=%+v rounds=%+v", model.calls, record.ToolCalls)
			}
			var outcome map[string]any
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var event map[string]any
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
				if event["msg"] == "run memory outcome" {
					outcome = event
				}
			}
			if outcome["outcome"] != "error" || outcome["status"] != "notStored" {
				t.Fatalf("failed storage overwritten: %v", outcome)
			}
			if _, exists := outcome["node"]; exists {
				t.Fatalf("false receipt: %v", outcome)
			}
			if strings.Contains(logs.String(), private) {
				t.Fatal("private note leaked")
			}
		})
	}
}
