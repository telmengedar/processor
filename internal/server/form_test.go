package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestAConfigsFormReachesTheRunAndIsEchoedAsThePerspectivesForm(t *testing.T) {
	t.Parallel()

	for content, want := range map[string]loop.Form{`{"form": "content"}`: loop.FormContent, `{"form": "substance"}`: loop.FormSubstance, `{}`: ""} {
		graph := &configGraph{content: content}
		result, _ := runWithConfig(t, graph, configPostBodyHead+`77}`)

		if result.code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200; body=%s", content, result.code, result.body)
		}
		var record loop.Record
		if err := json.Unmarshal([]byte(result.body), &record); err != nil {
			t.Fatalf("%s: the verbose body is not a record: %v", content, err)
		}
		if record.Perspective == nil || record.Perspective.Form != want {
			t.Errorf("%s: perspective = %+v, want form %q", content, record.Perspective, want)
		}
		if got := record.Limits.Form; got != want && !(want == loop.FormSubstance && got == "") {
			t.Errorf("%s: limits.form = %q, want %q", content, got, want)
		}
	}
}

func TestAConfigWithAnUnknownFormIsRefusedBeforeTheRunStarts(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{"form": "Content"}`}
	result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_config" {
		t.Fatalf("status = %d code = %q, want 400 invalid_config; body=%s", result.code, result.errorCode(t), result.body)
	}
	if model.calls() != 0 {
		t.Errorf("the model was called %d times", model.calls())
	}
}
