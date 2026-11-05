package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
)

func TestInvalidThinkingFailsBeforeModelActivity(t *testing.T) {
	for _, value := range []string{`null`, `"true"`, `0`, `[]`, `{}`} {
		graph := &configGraph{content: `{"thinking":` + value + `}`}
		result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)
		if result.code != http.StatusBadRequest || result.errorCode(t) != codeInvalidConfig || !strings.Contains(result.errorMessage(), `"thinking"`) || model.calls() != 0 || len(graph.recallCalls) != 0 {
			t.Fatalf("thinking=%s response=%+v model calls=%d recalls=%d", value, result, model.calls(), len(graph.recallCalls))
		}
	}
}

func TestConfiguredThinkingReachesDerivationAndEveryJudgement(t *testing.T) {
	for _, config := range []string{`{}`, `{"thinking":false}`, `{"thinking":true}`} {
		t.Run(config, func(t *testing.T) {
			want := strings.Contains(config, `true`)
			var stages []string
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire struct {
					Think *bool             `json:"think"`
					Tools []json.RawMessage `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
					t.Error(err)
				}
				if wire.Think == nil || *wire.Think != want {
					t.Errorf("wire think=%v want %v", wire.Think, want)
				}
				content := "derived retrieval query"
				switch len(stages) {
				case 0:
					stages = append(stages, "derive")
				case 1:
					stages = append(stages, "tool")
					if len(wire.Tools) == 0 {
						t.Error("tool selection offered no tools")
					}
					io.WriteString(w, `{"message":{"tool_calls":[{"function":{"name":"read_node","arguments":{"id":42}}}]},"done_reason":"stop"}`)
					return
				default:
					stages = append(stages, "reserved")
					if len(wire.Tools) != 0 {
						t.Error("reserved answer still offered tools")
					}
					content = "visible answer"
				}
				json.NewEncoder(w).Encode(map[string]any{"message": map[string]any{"content": content, "thinking": "secret reasoning must stay out"}, "done_reason": "stop"})
			}))
			defer endpoint.Close()
			graph := &configGraph{content: config}
			model := ollama.NewClient(endpoint.URL, "gemma4:latest", "", loop.Sampling{}, endpoint.Client())
			turn := loop.NewTurn(graph, model, nil, fixedSystem("system"), "gemma4:latest", testLogger())
			response := postRunsVerbose(t, turn, configPostBodyHead+`77}`)
			if response.Code != http.StatusOK {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
			var record loop.Record
			if err := json.Unmarshal(response.Body.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if strings.Join(stages, ",") != "derive,tool,reserved" || record.ModelCalls != 2 || record.Perspective == nil || record.Perspective.Thinking != want {
				t.Fatalf("stages=%v record=%+v", stages, record)
			}
			if record.Answer != "visible answer" || record.LastResponse == nil || record.LastResponse.ReasoningBytes != len("secret reasoning must stay out") {
				t.Fatalf("answer/accounting=%+v", record)
			}
			for _, call := range graph.recallCalls {
				for _, q := range call.Queries {
					if strings.Contains(q, "secret reasoning") {
						t.Fatalf("reasoning in query: %s", q)
					}
				}
			}
		})
	}
}

type thinkingExpectation struct{}
type thinkingTransport struct {
	arrivals chan struct{}
	release  chan struct{}
}

func (tr *thinkingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	var wire struct {
		Think    *bool             `json:"think"`
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
		return nil, err
	}
	want := r.Context().Value(thinkingExpectation{}).(bool)
	if wire.Think == nil || *wire.Think != want {
		return nil, fmt.Errorf("think=%v want=%v", wire.Think, want)
	}
	content := "visible answer"
	if len(wire.Messages) == 1 {
		tr.arrivals <- struct{}{}
		select {
		case <-tr.release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		content = "derived query"
	}
	body := `{"message":{"content":"` + content + `"},"done_reason":"stop"}`
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestConcurrentThinkingRunsDoNotShareState(t *testing.T) {
	const runs = 24
	transport := &thinkingTransport{arrivals: make(chan struct{}, runs), release: make(chan struct{})}
	client := ollama.NewClient("http://native.invalid", "gemma4:latest", "", loop.Sampling{}, &http.Client{Transport: transport})
	var wg sync.WaitGroup
	errors := make(chan string, runs)
	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			config := []string{`{"thinking":true}`, `{"thinking":false}`, `{}`, `{}`}[i%4]
			graph := &configGraph{content: config}
			turn := loop.NewTurn(graph, client, nil, fixedSystem("system"), "gemma4:latest", testLogger())
			ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), thinkingExpectation{}, i%4 == 0), time.Minute)
			defer cancel()
			body := configPostBodyHead + `77}`
			if i%4 == 3 {
				body = `{"input":"hello","subject":42}`
			}
			request := httptest.NewRequest(http.MethodPost, "/runs?verbose=true", strings.NewReader(body)).WithContext(ctx)
			response := httptest.NewRecorder()
			NewHandler(turn).ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				errors <- response.Body.String()
			}
		}(i)
	}
	for i := 0; i < runs; i++ {
		select {
		case <-transport.arrivals:
		case <-time.After(5 * time.Second):
			t.Fatal("derivations did not overlap")
		}
	}
	close(transport.release)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
