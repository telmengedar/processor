package loop_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

type poisoningGraph struct {
	anchor  loop.Anchor
	base    []loop.Candidate
	written []loop.Candidate
	writes  int
	lastErr error
}

func (g *poisoningGraph) Node(context.Context, int64) (loop.Anchor, bool, error) {
	return g.anchor, true, nil
}

func (g *poisoningGraph) Recall(_ context.Context, _ string, limit int) ([]loop.Candidate, error) {
	rows := append(append([]loop.Candidate{}, g.written...), g.base...)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	for i, row := range rows {
		rows[i].SelfProduced = divoid.IsRunRecord(row.Type, row.Name)
	}
	return rows, nil
}

func (g *poisoningGraph) WriteRun(_ context.Context, record loop.Record) loop.WriteReceipt {
	body, err := json.Marshal(record)
	if err != nil {
		g.lastErr = err
		return loop.WriteReceipt{State: loop.NotStored}
	}

	g.writes++
	id := int64(9000 + g.writes)
	g.written = append([]loop.Candidate{{
		ID:         id,
		Type:       divoid.RunNodeType,
		Name:       fmt.Sprintf("%s 2026-09-04T12:00:0%dZ — %s", divoid.RunNamePrefix, g.writes, record.Input),
		Similarity: 0.99,
		Content:    string(body),
	}}, g.written...)

	return loop.WriteReceipt{State: loop.Stored, NodeID: id}
}

type answeringModel struct{}

func (answeringModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	return loop.JudgeResult{Answer: "an answer", Reason: loop.Answered, RawReason: "stop"}, nil
}

func TestTurnRunIsNotPoisonedByItsOwnPreviousRecord(t *testing.T) {
	t.Parallel()

	const input = "what did the split change"

	graph := &poisoningGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "the subject body"},
		base: []loop.Candidate{
			{ID: 7, Type: "documentation", Name: "A real document", Similarity: 0.81, Content: strings.Repeat("r", 59_000)},
			{ID: 8, Type: divoid.RunNodeType, Name: "a session log another agent wrote", Similarity: 0.74, Content: strings.Repeat("h", 900)},
		},
	}
	model := answeringModel{}
	turn := loop.NewTurn(graph, model, "system", "test-model", nil)

	first, _, err := turn.Run(context.Background(), input, 42)
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if graph.lastErr != nil {
		t.Fatalf("the double could not serialise the record it was handed: %v", graph.lastErr)
	}
	if admittedCount(first.Candidates) != 2 {
		t.Fatalf("test setup error: turn 1 admitted %d of %d candidates, want both", admittedCount(first.Candidates), len(first.Candidates))
	}

	recordID, recordSize := graph.written[0].ID, len(graph.written[0].Content)
	if recordSize <= loop.AssemblyByteBudget {
		t.Fatalf("test setup error: the record is %d bytes against a %d-byte budget; it must exceed the budget to reproduce the measured geometry", recordSize, loop.AssemblyByteBudget)
	}

	second, _, err := turn.Run(context.Background(), input, 42)
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}

	if second.Candidates[0].ID != recordID {
		t.Fatalf("test setup error: turn 2's rank-1 candidate is #%d, want turn 1's record #%d", second.Candidates[0].ID, recordID)
	}
	if admittedCount(second.Candidates) == 0 {
		t.Fatalf("turn 2 admitted nothing from %d candidates: the run record at rank 1 cut everything behind it", len(second.Candidates))
	}
	if got, want := admittedCount(second.Candidates), 2; got != want {
		t.Fatalf("turn 2 admitted %d of %d candidates, want %d — both real rows behind the record", got, len(second.Candidates), want)
	}
	if second.Candidates[0].Included {
		t.Fatal("turn 2 admitted its own previous run record")
	}
	if !second.Candidates[2].Included {
		t.Fatalf("turn 2 cut #%d, a session log another agent wrote: the record's own type must not be what excludes it", second.Candidates[2].ID)
	}

	self, budget := cutReasons(t)
	if second.Candidates[0].CutReason != self {
		t.Fatalf("turn 2's rank-1 record was cut with reason %q, want %q — the row this loop wrote must be cut by the self-produced rule, not merely as one more oversized candidate", second.Candidates[0].CutReason, self)
	}
	if self == budget {
		t.Fatal("test setup error: the two cut reasons are the same string, so this assertion cannot discriminate")
	}
}

func cutReasons(t *testing.T) (selfProduced, byteBudget string) {
	t.Helper()

	_, self := loop.Assemble(loop.Anchor{ID: 1}, []loop.Candidate{{ID: 1, Content: "x", SelfProduced: true}}, 100)
	_, over := loop.Assemble(loop.Anchor{ID: 1}, []loop.Candidate{{ID: 1, Content: strings.Repeat("x", 200)}}, 100)

	if self[0].CutReason == "" || over[0].CutReason == "" {
		t.Fatalf("a reference cut produced no reason: self-produced %q, oversized %q", self[0].CutReason, over[0].CutReason)
	}
	return self[0].CutReason, over[0].CutReason
}

func admittedCount(dispositions []loop.Disposition) int {
	admitted := 0
	for _, d := range dispositions {
		if d.Included {
			admitted++
		}
	}
	return admitted
}
