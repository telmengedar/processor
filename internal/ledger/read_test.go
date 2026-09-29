package ledger

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestMechanismObservationDeclaresTheAddressedReadAMechanismTheArchiveCannotYetAnswer(t *testing.T) {
	t.Parallel()

	const mechanism = "addressed read"

	predating := archiveOfRecords("records filed before the tool existed",
		loop.Record{ToolCalls: []loop.ToolCallRecord{{Tool: loop.ToolRecall, Query: "what changed"}}},
		loop.Record{ToolCalls: []loop.ToolCallRecord{{Tool: loop.ToolWriteFile, Path: "index.html"}}},
	)

	result := MechanismObservation(predating)
	if !failedOn(result.Failures, mechanism) {
		t.Fatalf("an archive that predates the tool must report it unobserved rather than cleared, got %+v", result.Failures)
	}
	for _, reading := range result.Readings {
		if reading.Mechanism == mechanism && reading.ObservedIn != 0 {
			t.Fatalf("the read is reported observed in %d records of an archive that carries none", reading.ObservedIn)
		}
	}

	served := archiveOfRecords("one run read a part by id",
		loop.Record{ToolCalls: []loop.ToolCallRecord{{Tool: loop.ToolReadNode, NodeID: 71, Results: []loop.Disposition{{ID: 71, Included: true}}}}},
	)
	if failedOn(MechanismObservation(served).Failures, mechanism) {
		t.Fatalf("one dispatched read clears the mechanism, got %+v", MechanismObservation(served).Failures)
	}

	refused := archiveOfRecords("one run asked for a part that does not exist",
		loop.Record{ToolCalls: []loop.ToolCallRecord{{Tool: loop.ToolReadNode, NodeID: 71, Error: "no part of memory has that id", Results: []loop.Disposition{}}}},
	)
	if failedOn(MechanismObservation(refused).Failures, mechanism) {
		t.Fatalf("a refused read is still the model reaching for the tool, which is the question the reading answers, got %+v", MechanismObservation(refused).Failures)
	}

	if !strings.Contains(RenderProperties(DialExercise(predating), MechanismObservation(predating)), mechanism) {
		t.Fatalf("the rendered properties do not name the unobserved read at all, so nothing reports whether the tool earns the prompt bytes it costs")
	}
}

func TestTheRequeryAccountDoesNotCountAnAddressedReadAsARecallRound(t *testing.T) {
	t.Parallel()

	read := loop.ToolCallRecord{Tool: loop.ToolReadNode, NodeID: 71, Results: []loop.Disposition{{ID: 71, Included: true}}}

	if isRecall(read) {
		t.Fatal("an addressed read counts as a recall round, so every requery statistic in the ledger is computed over rounds that ran a different tool against a different argument")
	}

	requery := ComputeRequery(loop.Record{ToolCalls: []loop.ToolCallRecord{read}})
	if requery.DispatchedRounds != 0 || requery.ProductiveRounds != 0 || requery.BarrenRounds != 0 {
		t.Fatalf("a turn whose only round was an addressed read reports %+v, want no requery round of any kind", requery)
	}
}
