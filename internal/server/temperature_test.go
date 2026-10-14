package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
)

type temperatureModel struct {
	countingModel
	temperatures []*float64
}

func (m *temperatureModel) Judge(ctx context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.temperatures = append(m.temperatures, in.Temperature)
	return m.countingModel.Judge(ctx, in)
}

func TestAConfigWithABadTemperatureIsRefusedNamingTheKeyBeforeAnyModelCallOrRecall(t *testing.T) {
	t.Parallel()

	for _, text := range []string{`"0.7"`, `-0.5`, `null`, `true`, `1e400`} {
		graph := &configGraph{content: `{"temperature": ` + text + `}`}
		result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)

		if result.code != http.StatusBadRequest || result.errorCode(t) != codeInvalidConfig {
			t.Errorf("temperature %s: status = %d, code = %q, want 400 %s; body=%s", text, result.code, result.errorCode(t), codeInvalidConfig, result.body)
		}
		if !strings.Contains(result.errorMessage(), `"temperature"`) {
			t.Errorf("temperature %s: the message %q does not name the key", text, result.errorMessage())
		}
		if model.calls() != 0 || len(graph.recallCalls) != 0 {
			t.Errorf("temperature %s: the refused run made %d model calls and %d recalls, want none", text, model.calls(), len(graph.recallCalls))
		}
	}
}

func TestAConfigWithATemperatureReachesTheJudgementCallsAndTheRecord(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{"temperature": 0.7}`}
	model := &temperatureModel{}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system text"), "test-model", testLogger())

	rec := postRunsVerbose(t, turn, configPostBodyHead+`77}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	if len(model.temperatures) == 0 {
		t.Fatal("the model saw no judgement call")
	}
	for i, temperature := range model.temperatures {
		if temperature == nil || *temperature != 0.7 {
			t.Errorf("judgement call %d carried temperature %v, want 0.7", i, temperature)
		}
	}

	var record loop.Record
	if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
		t.Fatalf("the verbose body is not a record: %v", err)
	}
	if record.Perspective == nil || record.Perspective.Temperature == nil || *record.Perspective.Temperature != 0.7 {
		t.Errorf("perspective = %+v, want the temperature the config asked for", record.Perspective)
	}
}

func TestAnEndpointThatRefusesTheTemperatureEndsTheRunAsModelUnavailableCarryingItsOwnMessage(t *testing.T) {
	t.Parallel()

	const refusal = "temperature 50 is outside the range this endpoint accepts"
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"`+refusal+`"}`)
	}))
	t.Cleanup(endpoint.Close)

	graph := &configGraph{content: `{"temperature": 50}`}
	model := ollama.NewClient(endpoint.URL, "model-x", "", loop.Sampling{}, endpoint.Client())
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system text"), "test-model", testLogger())

	rec := postRunsVerbose(t, turn, configPostBodyHead+`77}`)

	result := &httpResult{code: rec.Code, body: rec.Body.String()}
	if result.code != http.StatusBadGateway || result.errorCode(t) != codeModelUnavailable {
		t.Fatalf("status = %d, code = %q, want 502 %s; body=%s", result.code, result.errorCode(t), codeModelUnavailable, result.body)
	}
	if !strings.Contains(result.errorMessage(), refusal) {
		t.Fatalf("the message %q drops the endpoint's own reason", result.errorMessage())
	}
}
