package loop_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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

func (g *poisoningGraph) Recall(_ context.Context, _ string, limit int, _ []int64) ([]loop.Candidate, error) {
	rows := append(append([]loop.Candidate{}, g.written...), g.base...)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	for i, row := range rows {
		rows[i].SelfProduced = divoid.IsRunRecord(row.Type, row.Name)
	}
	return rows, nil
}

func (g *poisoningGraph) Neighbours(context.Context, int64) ([]int64, error) { return nil, nil }

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
	turn := loop.NewTurn(graph, model, nil, "system", "test-model", nil)

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

	ids := dispositionIDs(second.Candidates)
	if slices.Contains(ids, recordID) {
		t.Fatalf("turn 2's candidate set is %v and still carries #%d, the record turn 1 wrote: the graph ranks it first, admission refuses it before reading a byte of it, and a slot spent on it is a slot no row the run could have read ever reached", ids, recordID)
	}
	if want := []int64{7, 8}; !slices.Equal(ids, want) {
		t.Fatalf("turn 2's candidate set is %v, want %v: dropping the record must leave the rows ranked behind it standing, in the order the graph reported them — and #8 is a session log another agent wrote, carrying the record's own node type, so a predicate that keys on the type alone loses it from this set too", ids, want)
	}
	if got, want := admittedCount(second.Candidates), 2; got != want {
		t.Fatalf("turn 2 admitted %d of %d candidates, want %d — both real rows", got, len(second.Candidates), want)
	}
}

func dispositionIDs(dispositions []loop.Disposition) []int64 {
	ids := make([]int64, len(dispositions))
	for i, d := range dispositions {
		ids[i] = d.ID
	}
	return ids
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
