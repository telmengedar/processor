package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

type conversationRecordingModel struct {
	conversations []string
}

func (m *conversationRecordingModel) Derive(context.Context, string, int) (string, error) {
	return "", nil
}

func (m *conversationRecordingModel) Judge(_ context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.conversations = append(m.conversations, in.Conversation)
	return loop.JudgeResult{Answer: "ok", Reason: loop.Answered, RawReason: "stop"}, nil
}

func TestRunsPassesTheHistoryToTheTurnOldestFirst(t *testing.T) {
	t.Parallel()

	model := &conversationRecordingModel{}
	body := `{"input":"so he drops","subject":42,"history":[{"input":"first said","reply":"first answered"},{"input":"second said"},{"input":"third said","reply":"third answered"}]}`

	rec := postRunsAt(t, context.Background(), replyTurn(model), "/runs?verbose=true", body)

	members, _ := topLevelKeys(t, rec)
	var history []struct {
		Input string `json:"input"`
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(members["history"], &history); err != nil {
		t.Fatalf("decode history %s: %v", members["history"], err)
	}
	if len(history) != 3 || history[0].Input != "first said" || history[0].Reply != "first answered" || history[1].Input != "second said" || history[1].Reply != "" || history[2].Input != "third said" {
		t.Fatalf("record history = %+v, want the three sent exchanges oldest first", history)
	}
	if len(model.conversations) != 1 {
		t.Fatalf("the model was called %d times, want 1", len(model.conversations))
	}
	if want := "===== EARLIER IN THIS CONVERSATION =====\nuser: first said\nyou: first answered\nuser: second said\nuser: third said\nyou: third answered"; model.conversations[0] != want {
		t.Fatalf("the model saw the conversation %q, want %q", model.conversations[0], want)
	}
}

func TestRunsReturns400OnAMalformedHistory(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		history string
	}{
		{"not an array", `"he left"`},
		{"an object instead of an array", `{"input":"he left"}`},
		{"an element that is not an object", `["he left"]`},
		{"a missing input", `[{"reply":"bold"}]`},
		{"a null input", `[{"input":null}]`},
		{"a blank input", `[{"input":"   "}]`},
		{"a non-string input", `[{"input":5}]`},
		{"a non-string reply", `[{"input":"he left","reply":5}]`},
		{"a bad element after a good one", `[{"input":"fine"},{"input":""}]`},
	} {
		rec := postRunsAt(t, context.Background(), replyTurn(&scriptedModel{step: answeringWith("never")}), "/runs", `{"input":"so he drops","subject":42,"history":`+tc.history+`}`)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d; body=%s", tc.name, rec.Code, http.StatusBadRequest, rec.Body.String())
			continue
		}
		var envelope errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Errorf("%s: decode error envelope: %v; body=%s", tc.name, err, rec.Body.String())
			continue
		}
		if envelope.Error.Code != "invalid_request" || !strings.Contains(envelope.Error.Message, "history") {
			t.Errorf("%s: error = %+v, want code invalid_request and a message naming history", tc.name, envelope.Error)
		}
	}
}

func TestRunsTreatsAnAbsentNullOrEmptyHistoryAsNoHistory(t *testing.T) {
	t.Parallel()

	for _, history := range []string{``, `,"history":null`, `,"history":[]`} {
		model := &conversationRecordingModel{}

		rec := postRunsAt(t, context.Background(), replyTurn(model), "/runs?verbose=true", `{"input":"so he drops","subject":42`+history+`}`)

		members, keys := topLevelKeys(t, rec)
		for _, key := range []string{"history", "historyCut"} {
			if _, present := members[key]; present {
				t.Errorf("history %q: the record carries %q, keys=%v", history, key, keys)
			}
		}
		if len(model.conversations) != 1 || model.conversations[0] != "" {
			t.Errorf("history %q: the model saw the conversations %q, want one empty", history, model.conversations)
		}
	}
}

func TestRunsTreatsANullReplyAsNoReply(t *testing.T) {
	t.Parallel()

	model := &conversationRecordingModel{}

	rec := postRunsAt(t, context.Background(), replyTurn(model), "/runs", `{"input":"so he drops","subject":42,"history":[{"input":"said","reply":null}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if want := "===== EARLIER IN THIS CONVERSATION =====\nuser: said"; len(model.conversations) != 1 || model.conversations[0] != want {
		t.Fatalf("the model saw the conversations %q, want one reading %q", model.conversations, want)
	}
}
