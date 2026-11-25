package loop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func decisionResult(raw string) JudgeResult {
	return JudgeResult{Answer: raw, Reason: Answered, RawReason: "stop", Usage: &Usage{InTokens: 19, OutTokens: 7}}
}
func decisionView() *View {
	v := noteView()
	v.Note.Mode = NoteModeDecision
	v.Identity = "verbatim private policy"
	return v
}

func TestPrivateNoteDecisionsPreserveScopeReceiptsAndNeverProduceChat(t *testing.T) {
	t.Parallel()
	for _, state := range []WriteState{Stored, Unlinked, NotStored} {
		t.Run(string(state), func(t *testing.T) {
			notes := &fakeNotes{receipt: WriteReceipt{State: state, NodeID: 901}}
			if state == NotStored {
				notes.receipt.NodeID = 0
			}
			model := &fakeModel{results: []JudgeResult{decisionResult(`{"action":"write","text":"Mara values restoring radio history.","about":71}`)}}
			run := runNotes(t, noteGraph(71), decisionView(), notes, model)
			r := run.record
			if len(model.calls) != 1 || len(notes.requests) != 1 {
				t.Fatalf("calls=%d writes=%d", len(model.calls), len(notes.requests))
			}
			req := notes.requests[0]
			if req.Subject != noteSubject || req.Root != noteRoot || req.Type != noteType || req.About != 71 || req.Text != "Mara values restoring radio history." {
				t.Fatalf("scope/text changed: %+v", req)
			}
			if r.NoteDecision == nil || r.NoteDecision.State != string(state) || r.NoteDecision.Created != notes.receipt.NodeID || r.Answer != "" || r.Outcome.Produced || r.Silent {
				t.Fatalf("private receipt/reply=%+v", r)
			}
			if len(r.Offered) != 1 || len(r.Offered[0]) != 0 || len(r.NoteAcks) != 0 || r.ModelCalls != 1 || len(r.Usage) != 1 {
				t.Fatalf("false call accounting: %+v", r)
			}
			call := model.calls[0]
			if !call.NoteDecisionOutput || call.StructuredAllowedActions != nil || call.Clause != "" || call.Trailer != "" || !strings.HasPrefix(call.System, decisionView().Identity+"\n\n") || strings.Contains(call.System, "system") {
				t.Fatalf("unexpected ordinary framework: %+v", call)
			}
			tool := run.noteRound(t)
			if tool.Source != ToolSourceStructured || tool.Written != state {
				t.Fatalf("provenance/receipt: %+v", tool)
			}
		})
	}
}

func TestPrivateNoteSkipAndRejectedDecisionsNeverReachStorage(t *testing.T) {
	t.Parallel()
	invalid := []string{`null`, `[]`, `{}`, `{"action":"write","text":"x"}`, `{"Action":"write","text":"x","about":null}`, `{"action":"write","text":"x","about":null,"extra":0}`, `{"action":"write","action":"skip","text":"x","about":null}`, `{"action":"write","text":"x","about":1.0}`, `{"action":"write","text":"x","about":9223372036854775808}`, `{"action":"write","text":"x","about":0}`, `{"action":"write","text":"x","about":-1}`, `{"action":"write","text":null,"about":null}`, `{"action":"write","text":" ","about":null}`, `{"action":"skip","text":"x","about":null}`, `{"action":"skip","text":"","about":42}`, `{"action":"skip","text":"","about":777001}`, `{"action":"write","text":"x","about":null} {}`, `{"action":"write","text":"x","about":null} trailing`, `write_note("x")`, `{"action":"write","text":"processor-run fake","about":null}`, "{\"action\":\"write\",\"text\":\"\xff\",\"about\":null}"}
	for _, raw := range append(invalid, `{"action":"skip","text":"","about":null}`) {
		run := runNotes(t, noteGraph(), decisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{decisionResult(raw)}})
		want := "rejected"
		if strings.Contains(raw, `"action":"skip","text":"","about":null`) {
			want = "abstained"
		}
		if len(run.model.calls) != 1 || len(run.notes.requests) != 0 || run.record.NoteDecision.State != want || run.record.NoteDecision.Created != 0 || run.record.Answer != "" {
			t.Fatalf("raw=%q calls=%d writes=%d receipt=%+v", raw, len(run.model.calls), len(run.notes.requests), run.record.NoteDecision)
		}
	}
}

func TestPrivateNoteRejectsUnexpectedNativeCallsAndTruncation(t *testing.T) {
	t.Parallel()
	for _, change := range []func(*JudgeResult){func(r *JudgeResult) { r.UnofferedToolCalls = 1 }, func(r *JudgeResult) { r.ToolSource = ToolSourceNative }, func(r *JudgeResult) { r.Reason = WantsNote }, func(r *JudgeResult) { r.RawReason = "length" }} {
		result := decisionResult(`{"action":"write","text":"a thought","about":null}`)
		change(&result)
		run := runNotes(t, noteGraph(), decisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{result}, ignoresWithheldToolList: true})
		if len(run.notes.requests) != 0 || run.record.NoteDecision.State != "rejected" {
			t.Fatalf("unexpected side effect: %+v", run)
		}
	}
}

func TestPrivateNoteKeepsVisibleAboutGuardAndNormalizesSubject(t *testing.T) {
	t.Parallel()
	for _, about := range []string{"999", "42", "null"} {
		run := runNotes(t, noteGraph(), decisionView(), &fakeNotes{}, &fakeModel{results: []JudgeResult{decisionResult(`{"action":"write","text":"a thought","about":` + about + `}`)}})
		if len(run.notes.requests) != 1 || run.notes.requests[0].About != 0 || run.noteRound(t).AboutDropped != (about == "999") {
			t.Fatalf("about=%s request=%+v record=%+v", about, run.notes.requests, run.record)
		}
	}
}

func TestPrivateNotePortFailureAndTimeBudgetNeverBecomeAbstention(t *testing.T) {
	t.Parallel()
	turn := NewTurn(noteGraph(), &fakeModel{err: errors.New("model failed")}, nil, fixedSystem("system"), "test", testLogger())
	turn.Notes = &fakeNotes{}
	opts := RunOptions{Input: "hello", Subject: noteSubject, Settings: DefaultSettings(), View: decisionView()}
	if _, _, err := turn.Answer(context.Background(), opts); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("error=%v", err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
	defer cancel()
	model := &fakeModel{}
	turn.Model = model
	if _, _, err := turn.Answer(ctx, opts); !errors.Is(err, ErrRunTimeExhausted) || len(model.calls) != 0 {
		t.Fatalf("error=%v calls=%d", err, len(model.calls))
	}
}

func TestPrivateNoteWithoutStoragePortRejectsWrite(t *testing.T) {
	t.Parallel()
	run := runNotes(t, noteGraph(), decisionView(), nil, &fakeModel{results: []JudgeResult{decisionResult(`{"action":"write","text":"a thought","about":null}`)}})
	if run.record.NoteDecision.State != "rejected" || len(run.record.ToolCalls) != 0 {
		t.Fatalf("receipt=%+v", run.record)
	}
}

func TestPrivateNoteContractBytesArePricedBeforeTheSingleJudgement(t *testing.T) {
	t.Parallel()
	now := time.Now()
	settings := DefaultSettings()
	floors := Floors{}.Resolved()
	base := settings.JudgementCost(floors)
	extra := rateSeconds(float64(len(noteDecisionInstructions)+len(NoteDecisionSchema())), floors.PromptBytesPerSecond)
	for _, private := range []bool{false, true} {
		model := &fakeModel{results: []JudgeResult{decisionResult(`{"action":"skip","text":"","about":null}`)}}
		turn := turnWithClock(model, noteGraph(), func() time.Time { return now })
		turn.Notes = &fakeNotes{}
		ctx, cancel := context.WithDeadline(context.Background(), now.Add(base+extra/2))
		view := noteView()
		if private {
			view = decisionView()
		}
		_, _, err := turn.Answer(ctx, RunOptions{Input: "hello", Subject: noteSubject, Settings: settings, View: view})
		cancel()
		if private {
			if !errors.Is(err, ErrRunTimeExhausted) || len(model.calls) != 0 {
				t.Fatalf("private contract escaped pricing: err=%v calls=%d", err, len(model.calls))
			}
		} else if err != nil || len(model.calls) != 1 {
			t.Fatalf("ordinary allowance changed: err=%v calls=%d", err, len(model.calls))
		}
	}
}

func TestPrivateNoteContractTimeIsReservedBeforeWaitingForSubstance(t *testing.T) {
	t.Parallel()
	now := time.Now()
	turn := turnWithClock(&fakeModel{}, noteGraph(), func() time.Time { return now })
	settings := DefaultSettings()
	base := settings.JudgementCost(turn.Floors)
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(base+time.Second))
	defer cancel()
	nativeWait := turn.fillWait(ctx, settings)
	settings.judgementExtraPromptBytes = 3000
	privateWait := turn.fillWait(ctx, settings)
	if nativeWait != time.Second || privateWait != 0 {
		t.Fatalf("native wait=%s private wait=%s", nativeWait, privateWait)
	}
}
