package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/loop"
)

func TestPerRunOllamaFillSelectionReachesTheWireAndPreservesConfiguredSamplingAndDefault(t *testing.T) {
	var mu sync.Mutex
	var models []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model   string `json:"model"`
			Think   *bool  `json:"think"`
			Options struct {
				Temperature float64 `json:"temperature"`
				TopP        float64 `json:"top_p"`
			} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode fill request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/api/chat" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Errorf("fill path=%q authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		if request.Think == nil || *request.Think || request.Options.Temperature != 0.37 || request.Options.TopP != 0.81 {
			t.Errorf("fill controls=%+v, want thinking off and configured sampling", request)
		}
		mu.Lock()
		models = append(models, request.Model)
		mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"model":%q,"message":{"content":"the condensed text"},"done_reason":"stop"}`, request.Model)
	}))
	defer upstream.Close()
	temperature, topP := 0.37, 0.81
	model, err := fillModel(boot.ModelConfig{Protocol: boot.ProtocolOllama, URL: upstream.URL, ID: "configured-fill", Key: "fixture-key"}, loop.Sampling{Temperature: &temperature, TopP: &topP})
	if err != nil {
		t.Fatalf("fillModel: %v", err)
	}
	selector, ok := model.(interface {
		WithModel(string) condense.ModelPort
	})
	if !ok {
		t.Fatalf("Ollama fill adapter %T cannot select a model", model)
	}
	selected := selector.WithModel("selected-fill")
	for _, tc := range []struct {
		model condense.ModelPort
		want  string
	}{{selected, "selected-fill"}, {model, "configured-fill"}} {
		result, err := tc.model.Condense(context.Background(), "condense this", 0)
		if err != nil || result.Model != tc.want {
			t.Fatalf("Condense model=%q, error=%v, want %q", result.Model, err, tc.want)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(models, []string{"selected-fill", "configured-fill"}) {
		t.Fatalf("fill wire models=%v", models)
	}
}
