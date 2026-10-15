package systemtext_test

import (
	"context"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/systemtext"
)

type bareGraph struct{}

func (bareGraph) Node(_ context.Context, id int64, _ loop.Filter) (loop.Anchor, bool, error) {
	return loop.Anchor{ID: id, Type: "documentation", Name: "Subject", Substance: "body"}, true, nil
}

func (bareGraph) Recall(context.Context, loop.RecallRequest) ([]loop.Candidate, error) {
	return nil, nil
}

func (bareGraph) WriteRun(context.Context, loop.Record) loop.WriteReceipt {
	return loop.WriteReceipt{State: loop.Stored, NodeID: 5}
}

type systemRecorder struct{ systems []string }

func (m *systemRecorder) Derive(context.Context, string, int) (loop.DeriveResult, error) {
	return loop.DeriveResult{}, nil
}

func (m *systemRecorder) Judge(_ context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.systems = append(m.systems, in.System)
	return loop.JudgeResult{Reason: loop.Answered, RawReason: "stop", Answer: "an answer"}, nil
}

func TestTheIdentityIsSeparatedFromTheRealProductTextByTheSeparatorComposeUsesBetweenItsParagraphs(t *testing.T) {
	t.Parallel()

	const identity = "You are a participant named Quill."
	model := &systemRecorder{}
	turn := loop.NewTurn(bareGraph{}, model, nil, systemtext.Compose, "test-model", nil)
	if _, _, err := turn.RunWithOptions(context.Background(), loop.RunOptions{Input: "hello", Subject: 42, Settings: loop.DefaultSettings(), View: &loop.View{Config: 9, Identity: identity}}); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}

	product := systemtext.Compose([]string{loop.ToolRecall, loop.ToolReadNode}, false, false, loop.FormSubstance)
	paragraphs := strings.Split(product, "\n\n")
	if len(paragraphs) < 2 {
		t.Fatalf("the product text has %d paragraphs, want several joined by one separator", len(paragraphs))
	}
	for i, paragraph := range paragraphs {
		if paragraph == "" || strings.HasPrefix(paragraph, "\n") || strings.HasSuffix(paragraph, "\n") {
			t.Fatalf("paragraph %d of the product text is %q: Compose joins its paragraphs by more than one blank line, so the identity separator no longer matches", i, paragraph)
		}
	}

	if len(model.systems) != 1 {
		t.Fatalf("the run made %d judgement calls, want 1", len(model.systems))
	}
	if want := identity + "\n\n" + product; model.systems[0] != want {
		t.Fatalf("the system text is %q, want the identity, one blank line, then the product text %q", model.systems[0], want)
	}
}

func TestARunThatCarriesBothTheIdentityAndSilenceGetsTheIdentityThenTheProductTextWithTheSilenceParagraph(t *testing.T) {
	t.Parallel()

	const identity = "You are a participant named Quill."
	model := &systemRecorder{}
	turn := loop.NewTurn(bareGraph{}, model, nil, systemtext.Compose, "test-model", nil)
	view := &loop.View{Config: 9, Silence: true, Identity: identity}
	if _, _, err := turn.RunWithOptions(context.Background(), loop.RunOptions{Input: "hello", Subject: 42, Settings: loop.DefaultSettings(), View: view}); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}

	offered := []string{loop.ToolRecall, loop.ToolReadNode}
	silent := systemtext.Compose(offered, false, true, loop.FormSubstance)
	if silent == systemtext.Compose(offered, false, false, loop.FormSubstance) {
		t.Fatal("the silence paragraph does not change the product text, so this test cannot tell it present from absent")
	}

	if len(model.systems) != 1 {
		t.Fatalf("the run made %d judgement calls, want 1", len(model.systems))
	}
	if want := identity + "\n\n" + silent; model.systems[0] != want {
		t.Fatalf("the system text is %q, want the identity, one blank line, then the product text with the silence paragraph %q", model.systems[0], want)
	}
}
