package loop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

type probeGraph struct{ recallN int }

func (g *probeGraph) Node(ctx context.Context, id int64) (loop.Anchor, bool, error) {
	return loop.Anchor{ID: id, Type: "documentation", Name: "S", Content: "anchor"}, true, nil
}
func (g *probeGraph) Recall(ctx context.Context, q string, limit int, scope []int64) ([]loop.Candidate, error) {
	g.recallN++
	if g.recallN == 1 {
		return []loop.Candidate{{ID: 1, Type: "task", Name: "c", Similarity: 0.9, Content: "body"}}, nil
	}
	return nil, errors.New("literal: transport blew up")
}
func (g *probeGraph) Neighbours(context.Context, int64) ([]int64, error) { return nil, nil }

func (g *probeGraph) WriteRun(ctx context.Context, r loop.Record) loop.WriteReceipt {
	return loop.WriteReceipt{State: loop.Stored, NodeID: 1}
}

type probeModel struct{ n int }

func (m *probeModel) Judge(ctx context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	m.n++
	if m.n == 1 {
		return loop.JudgeResult{Reason: loop.WantsRecall, RawReason: "tool_calls", RecallQuery: "q"}, nil
	}
	return loop.JudgeResult{Answer: "done", Reason: loop.Answered, RawReason: "stop"}, nil
}

func TestTurnBuiltByAnExternalKeyedLiteralSurvivesTheNilLoggerBranch(t *testing.T) {
	t.Parallel()

	turn := loop.Turn{Graph: &probeGraph{}, Model: &probeModel{}, System: "sys", ModelID: "m"}

	rec, receipt, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.ToolCalls) != 1 || rec.ToolCalls[0].Error == "" {
		t.Fatalf("record.ToolCalls = %+v, want one error-flagged round", rec.ToolCalls)
	}
	if receipt.State != loop.Stored {
		t.Fatalf("receipt.State = %q, want %q — the per-run log pair must not panic on the nil-logger path either", receipt.State, loop.Stored)
	}
}
