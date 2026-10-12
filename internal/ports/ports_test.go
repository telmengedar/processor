package ports

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
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

func TestBootBuildsTheFillPortOverTheJudgementModelWhenNoCondenseModelIsConfigured(t *testing.T) {
	t.Parallel()

	judgement := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://judge.invalid/v1", ID: "judge-model"}

	cfg, fromJudgement := FillModel(boot.ModelConfig{}, false, judgement)
	if !fromJudgement || cfg != judgement {
		t.Fatalf("FillModel = %+v (from judgement %v), want the judgement model's configuration", cfg, fromJudgement)
	}

	graph := FillGraph(divoid.NewClient("https://graph.example", "key", nil, nil))
	port, err := Fill(cfg, graph)
	if err != nil {
		t.Fatalf("building the fill port failed: %v", err)
	}
	if port == nil {
		t.Fatal("the fill port came back nil with no condense model configured, want a live port over the judgement model")
	}
}

func TestACondenseModelWhenConfiguredIsTheOneTheFillRunsOn(t *testing.T) {
	t.Parallel()

	judgement := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://judge.invalid/v1", ID: "judge-model"}
	condensing := boot.ModelConfig{Protocol: boot.ProtocolOllama, URL: "http://condense.invalid", ID: "condense-model"}

	cfg, fromJudgement := FillModel(condensing, true, judgement)
	if fromJudgement || cfg != condensing {
		t.Fatalf("FillModel = %+v (from judgement %v), want the condense model's configuration", cfg, fromJudgement)
	}
}

func TestTheFillPortIsALivePortOverTheGraphItWasGivenWhenACondenseModelIsConfigured(t *testing.T) {
	graph := FillGraph(divoid.NewClient("https://graph.example", "key", nil, nil))
	cfg := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://condense.invalid/v1", ID: "gemma-3-12b-it"}

	port, err := Fill(cfg, graph)
	if err != nil {
		t.Fatalf("building the fill port failed: %v", err)
	}
	if port == nil {
		t.Fatal("the fill port came back nil with a condense model configured, want a live port")
	}

	if port.Graph != graph {
		t.Fatalf("the fill port reads %#v, want the graph it was given, %#v — a port that substitutes its own graph can write past a decorator on this seam", port.Graph, graph)
	}
}

func TestTheFillPortSurfacesAnUnsupportedProtocolAsABootError(t *testing.T) {
	t.Parallel()

	graph := FillGraph(divoid.NewClient("https://graph.example", "key", nil, nil))
	cfg := boot.ModelConfig{Protocol: "responses", URL: "http://condense.invalid", ID: "m"}

	port, err := Fill(cfg, graph)
	if err == nil {
		t.Fatalf("the fill port came back as %T with no error for a protocol it has no adapter for, want a refusal", port)
	}
	if port != nil {
		t.Fatalf("the fill port came back as %T alongside its error, want nil", port)
	}
}
