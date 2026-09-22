package ports

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/fill"
	"github.com/telmengedar/processor/internal/loop"
)

func TestTheFillModelBuildsTheOpenAICompatAdapterForTheOpenAICompatProtocol(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://condense.invalid/v1", ID: "m"}

	model, err := fillModel(cfg, loop.Sampling{})
	if err != nil {
		t.Fatalf("building the fill model failed: %v", err)
	}
	if _, ok := model.(*fillOpenAICompatModel); !ok {
		t.Fatalf("the fill model came back as %T, want the OpenAI-compatible fill adapter", model)
	}
}

func TestTheFillModelBuildsTheOllamaAdapterForTheOllamaProtocol(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: boot.ProtocolOllama, URL: "http://condense.invalid", ID: "m"}

	model, err := fillModel(cfg, loop.Sampling{})
	if err != nil {
		t.Fatalf("building the fill model failed: %v", err)
	}
	if _, ok := model.(*fillOllamaModel); !ok {
		t.Fatalf("the fill model came back as %T, want the native ollama fill adapter", model)
	}
}

func TestTheFillModelRefusesAProtocolItHasNoAdapterFor(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: "responses", URL: "http://condense.invalid", ID: "m"}

	model, err := fillModel(cfg, loop.Sampling{})
	if err == nil {
		t.Fatalf("the fill model came back as %T with no error for a protocol it has no adapter for, want a refusal", model)
	}
	if model != nil {
		t.Fatalf("the fill model came back as %T alongside its error, want nil", model)
	}
	if !strings.Contains(err.Error(), "responses") {
		t.Fatalf("error = %q, want it to name the protocol it was given", err.Error())
	}
}

func TestTheFillPortIsNilWhenNoCondenseModelIsConfigured(t *testing.T) {
	t.Parallel()

	graph := FillGraph(divoid.NewClient("https://graph.example", "key", nil, nil))

	port, err := Fill(boot.ModelConfig{}, false, graph)
	if err != nil {
		t.Fatalf("building the fill port failed: %v", err)
	}
	if port != nil {
		t.Fatalf("the fill port came back as %T, want nil when configured is false", port)
	}
}

func TestTheFillPortIsALivePortOverTheGraphItWasGivenWhenACondenseModelIsConfigured(t *testing.T) {
	graph := FillGraph(divoid.NewClient("https://graph.example", "key", nil, nil))
	cfg := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://condense.invalid/v1", ID: "gemma-3-12b-it"}

	port, err := Fill(cfg, true, graph)
	if err != nil {
		t.Fatalf("building the fill port failed: %v", err)
	}
	if port == nil {
		t.Fatal("the fill port came back nil with a condense model configured, want a live port")
	}

	built, ok := port.(*fill.Port)
	if !ok {
		t.Fatalf("the fill port came back as %T, want the port that carries the graph it was built over", port)
	}
	if built.Graph != graph {
		t.Fatalf("the fill port reads %#v, want the graph it was given, %#v — a port that substitutes its own graph can write past a decorator on this seam", built.Graph, graph)
	}
}

func TestTheFillPortSurfacesAnUnsupportedProtocolAsABootError(t *testing.T) {
	t.Parallel()

	graph := FillGraph(divoid.NewClient("https://graph.example", "key", nil, nil))
	cfg := boot.ModelConfig{Protocol: "responses", URL: "http://condense.invalid", ID: "m"}

	port, err := Fill(cfg, true, graph)
	if err == nil {
		t.Fatalf("the fill port came back as %T with no error for a protocol it has no adapter for, want a refusal", port)
	}
	if port != nil {
		t.Fatalf("the fill port came back as %T alongside its error, want nil", port)
	}
}
