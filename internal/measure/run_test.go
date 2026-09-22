package measure

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

func TestTheRecordAMeasuredRunProducesIsTheRecordTheShippedTurnProducesAndOnlyItsFateDiffers(t *testing.T) {
	measuredLive := newRecordingGraph()
	_, runner := measuredRunner(measuredLive)

	measured, err := runner.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the measured run failed: %v", err)
	}

	shippedLive := newRecordingGraph()
	turn := loop.NewTurn(shippedLive, &answeringModel{}, nil, testSystem, testModelID, testLogger())

	shipped, shippedReceipt, err := turn.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the undecorated run failed: %v", err)
	}

	if shippedReceipt.State != loop.Stored {
		t.Fatalf("the undecorated run reports %q, and this test needs a run that really filed to contrast against", shippedReceipt.State)
	}
	if measured.Written.State != loop.NotStored {
		t.Fatalf("the measured run reports %q, want %q", measured.Written.State, loop.NotStored)
	}

	measuredRecord, shippedRecord := measured.Record, shipped
	measuredRecord.Now, shippedRecord.Now = time.Time{}, time.Time{}

	if !reflect.DeepEqual(measuredRecord, shippedRecord) {
		t.Fatalf("the measured run produced a different record than the undecorated one\nmeasured: %+v\nshipped:  %+v", measuredRecord, shippedRecord)
	}
}

func TestAMeasuredRunEmitsTheAnswerTheBlockAndTheRecordItDeclinedToFile(t *testing.T) {
	live := newRecordingGraph()
	_, runner := measuredRunner(live)

	result, err := runner.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the measured run failed: %v", err)
	}

	if result.Answer != testAnswer {
		t.Fatalf("the result answers %q, want %q", result.Answer, testAnswer)
	}
	if strings.TrimSpace(result.Block) == "" {
		t.Fatal("the result carries no assembled block")
	}
	if result.Input != testInput || result.Subject != 101 {
		t.Fatalf("the result records input %q against subject %d, want %q against 101", result.Input, result.Subject, testInput)
	}
	if result.Model != testModelID {
		t.Fatalf("the result names model %q, want %q", result.Model, testModelID)
	}
	if len(result.Suppressed) != 1 || result.Suppressed[0].Size == 0 {
		t.Fatalf("want one sized suppressed write alongside the record, got %+v", result.Suppressed)
	}
}

func TestTheRunnerCarriesEveryRunsSuppressedWriteWhileEachResultCarriesOnlyItsOwn(t *testing.T) {
	live := newRecordingGraph()
	_, runner := measuredRunner(live)

	first, err := runner.Run(context.Background(), testInput, 101)
	if err != nil {
		t.Fatalf("the first measured run failed: %v", err)
	}
	second, err := runner.Run(context.Background(), testInput, 202)
	if err != nil {
		t.Fatalf("the second measured run failed: %v", err)
	}

	if len(first.Suppressed) != 1 || first.Suppressed[0].Ordinal != 1 {
		t.Fatalf("want the first result to carry its own suppressed write alone, got %+v", first.Suppressed)
	}
	if len(second.Suppressed) != 1 || second.Suppressed[0].Ordinal != 2 {
		t.Fatalf("want the second result to carry its own suppressed write alone, got %+v", second.Suppressed)
	}
	if all := runner.Suppressed(); len(all) != 2 {
		t.Fatalf("want both suppressed writes on the runner, got %d", len(all))
	}
}
