package ports

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

func TestTheModelPortBuildsTheOpenAICompatAdapterForTheOpenAICompatProtocol(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: boot.ProtocolOpenAICompat, URL: "http://model.invalid/v1", ID: "m"}

	model, err := Model(cfg, loop.Sampling{})
	if err != nil {
		t.Fatalf("building the model port failed: %v", err)
	}
	if _, ok := model.(*openaicompat.Client); !ok {
		t.Fatalf("the model port came back as %T, want the OpenAI-compatible adapter", model)
	}
}

func TestTheModelPortBuildsTheOllamaAdapterForTheOllamaProtocol(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: boot.ProtocolOllama, URL: "http://model.invalid", ID: "m"}

	model, err := Model(cfg, loop.Sampling{})
	if err != nil {
		t.Fatalf("building the model port failed: %v", err)
	}
	if _, ok := model.(*ollama.Client); !ok {
		t.Fatalf("the model port came back as %T, want the native ollama adapter", model)
	}
}

func TestTheModelPortRefusesToGuessAnAdapterForAProtocolItDoesNotImplement(t *testing.T) {
	t.Parallel()

	cfg := boot.ModelConfig{Protocol: "responses", URL: "http://model.invalid", ID: "m"}

	model, err := Model(cfg, loop.Sampling{})
	if err == nil {
		t.Fatalf("the model port came back as %T with no error for a protocol it has no adapter for, want a refusal rather than a defaulted adapter", model)
	}
	if model != nil {
		t.Fatalf("the model port came back as %T alongside its error, want nil", model)
	}
	if !strings.Contains(err.Error(), "responses") {
		t.Fatalf("error = %q, want it to name the protocol it was given", err.Error())
	}
}
