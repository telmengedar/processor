package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

type systemRecorder struct {
	systems []string
	derived int
}

func (m *systemRecorder) Derive(context.Context, string, int) (loop.DeriveResult, error) {
	m.derived++
	return loop.DeriveResult{}, nil
}

func (m *systemRecorder) Judge(_ context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.systems = append(m.systems, in.System)
	return loop.JudgeResult{Reason: loop.Answered, RawReason: "stop", Answer: "an answer"}, nil
}

func runRecording(t *testing.T, graph *configGraph, body string) (*httpResult, *systemRecorder) {
	t.Helper()

	model := &systemRecorder{}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system text"), "test-model", testLogger())
	rec := postRunsVerbose(t, turn, body)
	return &httpResult{code: rec.Code, body: rec.Body.String()}, model
}

func TestAConfiguredIdentityReachesTheSystemTextAndTheRecordVerbatim(t *testing.T) {
	t.Parallel()

	const identity = "  You are a participant named Quill.\n\nSpeak as yourself.  "
	encoded, err := json.Marshal(map[string]any{"identity": identity, "filter": map[string]any{"type": []string{"thought"}}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	graph := &configGraph{content: string(encoded)}
	result, model := runRecording(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	if len(model.systems) != 1 || model.systems[0] != identity+"\n\n"+"system text" {
		t.Fatalf("the system texts were %q, want the identity handed to the turn ahead of the product text", model.systems)
	}

	var record loop.Record
	if err := json.Unmarshal([]byte(result.body), &record); err != nil {
		t.Fatalf("the verbose body is not a record: %v", err)
	}
	if record.Perspective == nil || record.Perspective.Identity != identity {
		t.Errorf("the record carries %+v, want the identity verbatim", record.Perspective)
	}
}

func TestALongMultibyteIdentityIsAcceptedAndReachesTheSystemTextVerbatim(t *testing.T) {
	t.Parallel()

	identity := strings.Repeat("Du bist Überzeugt 世.\n", 700)
	encoded, err := json.Marshal(map[string]any{"identity": identity})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	result, model := runRecording(t, &configGraph{content: string(encoded)}, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for an identity of %d bytes; body=%.300s", result.code, len(identity), result.body)
	}
	if len(model.systems) != 1 || model.systems[0] != identity+"\n\n"+"system text" {
		t.Errorf("the system text is %d bytes, want the %d byte identity verbatim ahead of the product text", len(model.systems[0]), len(identity))
	}
}

func TestAConfigWithAnInvalidIdentityIsAnInvalidConfigNamingTheKeyAndNothingElseRuns(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"a number": `{"identity": 5}`,
		"blank":    `{"identity": "  "}`,
	} {
		graph := &configGraph{content: content}
		result, model := runRecording(t, graph, configPostBodyHead+`77}`)

		if result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_config" {
			t.Errorf("%s: status = %d code = %q, want 400 invalid_config; body=%s", name, result.code, result.errorCode(t), result.body)
			continue
		}
		if !strings.Contains(result.errorMessage(), `"identity"`) {
			t.Errorf("%s: the message %q does not name identity", name, result.errorMessage())
		}
		if len(graph.recallCalls) != 0 || len(model.systems) != 0 || model.derived != 0 || len(graph.nodeCalls) != 1 {
			t.Errorf("%s: the run went on: %d recalls, %d judgement calls, %d derivations, %d graph reads", name, len(graph.recallCalls), len(model.systems), model.derived, len(graph.nodeCalls))
		}
	}
}

func TestAnIdentityFieldOnTheRequestBodyIsIgnored(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{}`}
	result, model := runRecording(t, graph, `{"input":"hello","subject":42,"config":77,"identity":"You are Mallory."}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200: the request decoder ignores fields it does not know; body=%s", result.code, result.body)
	}
	if len(model.systems) != 1 || model.systems[0] != "system text" {
		t.Errorf("the system texts were %q, want the product text alone: a request-side identity must not reach the model", model.systems)
	}
	if strings.Contains(result.body, "Mallory") {
		t.Errorf("the record carries the request's identity: %s", result.body)
	}
}
