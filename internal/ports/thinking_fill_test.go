package ports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/condense"
	"github.com/telmengedar/processor/internal/fill"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
)

type thinkingFillGraph struct {
	mu        sync.Mutex
	substance string
	wrote     chan struct{}
}

func (g *thinkingFillGraph) Node(context.Context, int64, loop.Filter) (loop.Anchor, bool, error) {
	return loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: strings.Repeat("source material ", 40)}, true, nil
}
func (g *thinkingFillGraph) Recall(context.Context, loop.RecallRequest) ([]loop.Candidate, error) {
	return nil, nil
}
func (g *thinkingFillGraph) WriteRun(context.Context, loop.Record) loop.WriteReceipt {
	return loop.WriteReceipt{State: loop.NotStored}
}
func (g *thinkingFillGraph) NodeWithSubstance(context.Context, int64) (condense.Node, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return condense.Node{ID: 42, Type: "documentation", Name: "Subject", Content: strings.Repeat("source material ", 40), Substance: g.substance}, true, nil
}
func (g *thinkingFillGraph) Content(context.Context, int64) (string, bool, error) {
	return strings.Repeat("source material ", 40), true, nil
}
func (g *thinkingFillGraph) SetSubstance(_ context.Context, _ int64, text string) error {
	g.mu.Lock()
	g.substance = text
	g.mu.Unlock()
	g.wrote <- struct{}{}
	return nil
}

func TestCondensationStaysOffForThinkingChat(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "synchronous", true: "deferred"}[deferred], func(t *testing.T) {
			var mu sync.Mutex
			var stages []string
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire struct {
					Think    *bool             `json:"think"`
					Messages []json.RawMessage `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
					t.Error(err)
				}
				if wire.Think == nil {
					t.Error("think omitted")
					http.Error(w, "think omitted", 400)
					return
				}
				stage := "fill"
				if *wire.Think {
					stage = "derive"
					if len(wire.Messages) > 1 {
						stage = "judge"
					}
				}
				mu.Lock()
				stages = append(stages, stage)
				mu.Unlock()
				text := "compact stored substance"
				if stage == "judge" {
					text = "The visible answer is complete."
				}
				json.NewEncoder(w).Encode(map[string]any{"model": "gemma4:latest", "message": map[string]string{"content": text}, "done_reason": "stop"})
			}))
			defer endpoint.Close()
			client := ollama.NewClient(endpoint.URL, "gemma4:latest", "", loop.Sampling{}, endpoint.Client())
			graph := &thinkingFillGraph{wrote: make(chan struct{}, 1)}
			port := fill.New(graph, &fillOllamaModel{client: client}, nil, nil)
			turn := loop.NewTurn(graph, client, nil, func([]string, bool, bool, loop.Form, loop.ResponseMode) string { return "system" }, "gemma4:latest", nil)
			turn.Fill = port
			settings := loop.DefaultSettings()
			if deferred {
				settings.FillWait = 0
			}
			record, _, err := turn.RunWithOptions(context.Background(), loop.RunOptions{Input: "hello", Subject: 42, Settings: settings, View: &loop.View{Config: 77, Thinking: true}})
			if err != nil {
				t.Fatal(err)
			}
			if len(record.Fills) != 1 || record.Fills[0].Deferred != deferred || record.Fills[0].Filled == deferred {
				t.Fatalf("fills=%+v", record.Fills)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if deferred {
				port.Start(ctx)
			}
			select {
			case <-graph.wrote:
			case <-time.After(5 * time.Second):
				t.Fatal("fill was not stored")
			}
			mu.Lock()
			got := strings.Join(stages, ",")
			mu.Unlock()
			want := "derive,fill,judge"
			if deferred {
				want = "derive,judge,fill"
			}
			if got != want {
				t.Fatalf("wire stages=%s want=%s calls=%+v", got, want, record.ToolCalls)
			}
		})
	}
}
