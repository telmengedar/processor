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

	const (
		realDocument      = int64(7)
		foreignSessionLog = int64(8)
	)

	graph := &poisoningGraph{
		anchor: loop.Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "the subject body"},
		base: []loop.Candidate{
			{ID: realDocument, Type: "documentation", Name: "A real document", Similarity: 0.81, Content: strings.Repeat("r", 59_000)},
			{ID: foreignSessionLog, Type: divoid.RunNodeType, Name: "a session log another agent wrote", Similarity: 0.74, Content: strings.Repeat("h", 900)},
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

	for _, d := range second.Candidates {
		if d.ID == recordID {
			t.Fatalf("turn 2 carried its own turn-1 record #%d as candidate %d of %d: recall ranks it above every real row, so a record the block refuses only at admission has already spent the slot the row behind it needed", recordID, d.Rank, len(second.Candidates))
		}
	}
	if got, want := admittedCount(second.Candidates), 2; got != want {
		t.Fatalf("turn 2 admitted %d of %d candidates, want %d — the real document and the session log another agent wrote", got, len(second.Candidates), want)
	}
	if foreign := dispositionOf(t, second.Candidates, foreignSessionLog); !foreign.Included {
		t.Fatalf("turn 2 cut #%d, a session log another agent wrote, with reason %q: it carries the same node type as the record and only the name tells them apart, so excluding the type is a wider rule than the one measured", foreign.ID, foreign.CutReason)
	}

	self, budget := cutReasons(t)
	if self == budget {
		t.Fatal("test setup error: the two cut reasons are the same string, so this assertion cannot discriminate")
	}

	_, direct := loop.Assemble(graph.anchor, []loop.Candidate{{ID: recordID, Content: "x", SelfProduced: true}}, loop.AssemblyByteBudget)
	if direct[0].Included || direct[0].CutReason != self {
		t.Fatalf("assembly handed a record this system wrote admitted it with reason %q, want it cut as %q: retrieval now keeps such a row out of the candidate list, and admission is the second refusal that still has to hold for one arriving by any other route", direct[0].CutReason, self)
	}
}

func dispositionOf(t *testing.T, dispositions []loop.Disposition, id int64) loop.Disposition {
	t.Helper()

	for _, d := range dispositions {
		if d.ID == id {
			return d
		}
	}

	t.Fatalf("no candidate carries id %d among the %d turn 2 returned", id, len(dispositions))
	return loop.Disposition{}
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
