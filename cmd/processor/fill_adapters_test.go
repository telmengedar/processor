package main

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

func TestNewFillModelBuildsTheOpenAICompatAdapterForTheOpenAICompatProtocol(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://condense.invalid/v1", ID: "m"}

	model, err := newFillModel(cfg, loop.Sampling{})
	if err != nil {
		t.Fatalf("newFillModel: %v", err)
	}
	if _, ok := model.(*fillOpenAICompatModel); !ok {
		t.Fatalf("newFillModel returned %T, want the OpenAI-compatible fill adapter", model)
	}
}

func TestNewFillModelBuildsTheOllamaAdapterForTheOllamaProtocol(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: boot.ProtocolOllama, URL: "http://condense.invalid", ID: "m"}

	model, err := newFillModel(cfg, loop.Sampling{})
	if err != nil {
		t.Fatalf("newFillModel: %v", err)
	}
	if _, ok := model.(*fillOllamaModel); !ok {
		t.Fatalf("newFillModel returned %T, want the native ollama fill adapter", model)
	}
}

func TestNewFillModelRefusesAProtocolItHasNoAdapterFor(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: "responses", URL: "http://condense.invalid", ID: "m"}

	model, err := newFillModel(cfg, loop.Sampling{})
	if err == nil {
		t.Fatalf("newFillModel returned %T with no error for a protocol it has no adapter for, want a refusal", model)
	}
	if model != nil {
		t.Fatalf("newFillModel returned %T alongside its error, want nil", model)
	}
	if !strings.Contains(err.Error(), "responses") {
		t.Fatalf("error = %q, want it to name the protocol it was given", err.Error())
	}
}

func TestNewFillPortIsNilWhenNoCondenseModelIsConfigured(t *testing.T) {
	t.Parallel()

	graph := divoid.NewClient("https://graph.example", "key", nil, nil)

	port, err := newFillPort(boot.ModelConfig{}, false, graph)
	if err != nil {
		t.Fatalf("newFillPort: %v", err)
	}
	if port != nil {
		t.Fatalf("newFillPort returned %T, want nil when configured is false", port)
	}
}

func TestNewFillPortBuildsAPortWhenConfigured(t *testing.T) {
	t.Parallel()

	graph := divoid.NewClient("https://graph.example", "key", nil, nil)
	cfg := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://condense.invalid/v1", ID: "gemma-3-12b-it"}

	port, err := newFillPort(cfg, true, graph)
	if err != nil {
		t.Fatalf("newFillPort: %v", err)
	}
	if port == nil {
		t.Fatal("newFillPort returned nil with configured = true, want a live port")
	}
}

func TestNewFillPortSurfacesAnUnsupportedProtocolAsABootError(t *testing.T) {
	t.Parallel()

	graph := divoid.NewClient("https://graph.example", "key", nil, nil)
	cfg := boot.ModelConfig{Protocol: "responses", URL: "http://condense.invalid", ID: "m"}

	port, err := newFillPort(cfg, true, graph)
	if err == nil {
		t.Fatalf("newFillPort returned %T with no error for a protocol it has no adapter for, want a refusal", port)
	}
	if port != nil {
		t.Fatalf("newFillPort returned %T alongside its error, want nil", port)
	}
}
