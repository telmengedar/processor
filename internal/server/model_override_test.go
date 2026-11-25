package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
)

type requestModelFill struct {
	model    string
	calls    *[]string
	wait     bool
	deferred *[]string
}

func (f *requestModelFill) WithModel(model string) (loop.FillPort, error) {
	selected := *f
	selected.model = model
	return &selected, nil
}

func (f *requestModelFill) Fill(ctx context.Context, _ int64) (loop.FillResult, error) {
	if f.wait {
		<-ctx.Done()
		return loop.FillResult{}, ctx.Err()
	}
	*f.calls = append(*f.calls, f.model)
	return loop.FillResult{Substance: "the selected fill", Model: f.model, Written: true}, nil
}

func (f *requestModelFill) Defer(int64) {
	*f.deferred = append(*f.deferred, f.model)
}

type selectableRequestModel struct{ *stubModel }

func (m *selectableRequestModel) WithModel(string) loop.ModelPort {
	return &selectableRequestModel{stubModel: m.stubModel}
}

type fixedModelFill struct{}

func (*fixedModelFill) Fill(context.Context, int64) (loop.FillResult, error) {
	return loop.FillResult{}, nil
}

func (*fixedModelFill) Defer(int64) {}

func TestPerRunModelReachesDerivationJudgementFillsAndRecordWithoutChangingDefaults(t *testing.T) {
	var mu sync.Mutex
	var models []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model    string            `json:"model"`
			Think    *bool             `json:"think"`
			Messages []json.RawMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode model request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Think == nil || *request.Think {
			t.Errorf("think = %v, want explicit false", request.Think)
		}
		mu.Lock()
		models = append(models, request.Model)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		content := "The requested change now selects its model at call time."
		if len(request.Messages) == 1 {
			content = "model choice"
		}
		_, _ = fmt.Fprintf(w, `{"model":%q,"message":{"role":"assistant","content":%q},"done":true,"done_reason":"stop"}`, request.Model, content)
	}))
	defer upstream.Close()

	model := ollama.NewClient(upstream.URL, "configured-model", "", loop.Sampling{}, upstream.Client())
	graph := stubGraph{anchor: loop.Anchor{ID: 42, Type: "documentation", Content: strings.Repeat("the body ", 50)}, found: true}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "configured-model", testLogger())
	var fills []string
	fill := &requestModelFill{model: "configured-fill", calls: &fills}
	turn.Fill = fill

	for _, tc := range []struct {
		member string
		want   string
	}{
		{`,"model":"selected-one"`, "selected-one"},
		{"", "configured-model"},
		{`,"model":"selected-two"`, "selected-two"},
		{`,"model":null`, "configured-model"},
	} {
		mu.Lock()
		before := len(models)
		mu.Unlock()
		rec := postRunsVerbose(t, turn, `{"input":"what changed","subject":42`+tc.member+`}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /runs: status=%d body=%s", rec.Code, rec.Body.String())
		}
		var record struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
			t.Fatalf("decode record: %v", err)
		}
		if record.Model != tc.want {
			t.Fatalf("record model=%q, want %q", record.Model, tc.want)
		}
		mu.Lock()
		got := append([]string(nil), models[before:]...)
		mu.Unlock()
		if !reflect.DeepEqual(got, []string{tc.want, tc.want}) {
			t.Fatalf("derivation and judgement models=%v, want [%s %s]", got, tc.want, tc.want)
		}
	}
	if !reflect.DeepEqual(fills, []string{"selected-one", "configured-fill", "selected-two", "configured-fill"}) {
		t.Fatalf("fill models=%v", fills)
	}
	if turn.Model != model || turn.ModelID != "configured-model" || turn.Fill != fill || fill.model != "configured-fill" {
		t.Fatal("the request changed the shared turn or fill defaults")
	}
}

func TestPerRunModelRejectsBlankMistypedAndUnsupportedOverridesBeforeModelCalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		member string
		ollama bool
		fill   bool
		want   string
	}{
		{"empty", `,"model":""`, false, false, "model must not be empty"},
		{"whitespace", `,"model":" \t "`, false, false, "model must not be empty"},
		{"mistyped", `,"model":17`, false, false, "request body could not be parsed as JSON"},
		{"unsupported model", `,"model":"selected"`, false, false, "configured model adapter"},
		{"unsupported fill", `,"model":"selected"`, true, true, "configured fill adapter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer upstream.Close()
			stub := &stubModel{}
			turn := replyTurn(stub)
			if tc.ollama {
				turn.Model = ollama.NewClient(upstream.URL, "default", "", loop.Sampling{}, upstream.Client())
			}
			if tc.fill {
				turn.Fill = &fixedModelFill{}
			}
			rec := postRuns(t, turn, `{"input":"what changed","subject":42`+tc.member+`}`)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s, want 400 containing %q", rec.Code, rec.Body.String(), tc.want)
			}
			assertErrorCode(t, rec, codeInvalidRequest)
			if calls.Load() != 0 || stub.calls != 0 {
				t.Fatalf("upstream calls=%d stub calls=%d, want none", calls.Load(), stub.calls)
			}
		})
	}
}

func TestPerRunModelReachesDeferredFillsOnBothResponsePaths(t *testing.T) {
	for _, target := range []string{"/runs", "/runs?verbose=true"} {
		t.Run(target, func(t *testing.T) {
			var deferred []string
			fill := &requestModelFill{model: "configured-fill", wait: true, deferred: &deferred}
			graph := stubGraph{anchor: loop.Anchor{ID: 42, Type: "documentation", Content: strings.Repeat("the body ", 50)}, found: true}
			model := &selectableRequestModel{stubModel: &stubModel{results: []loop.JudgeResult{{Answer: "The result is ready.", Reason: loop.Answered, RawReason: "stop"}}}}
			turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "configured-model", testLogger())
			turn.Fill = fill
			turn.Floors = loop.Floors{PromptBytesPerSecond: 160_000}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			rec := postRunsAt(t, ctx, turn, target, `{"input":"what changed","subject":42,"model":"selected-model"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("POST %s: status=%d body=%s", target, rec.Code, rec.Body.String())
			}
			if !reflect.DeepEqual(deferred, []string{"selected-model"}) {
				t.Fatalf("deferred models=%v, want [selected-model]", deferred)
			}
			if fill.model != "configured-fill" || turn.Fill != fill {
				t.Fatal("deferring a selected fill changed the shared default")
			}
		})
	}
}
