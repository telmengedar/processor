package loop

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

func offeredMember(t *testing.T, record Record) json.RawMessage {
	t.Helper()

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	raw, present := members["offered"]
	if !present {
		t.Fatalf("the record has no offered member: %s", encoded)
	}
	return raw
}

func TestTheRecordNamesTheToolsEachCallOfferedAndAnEmptyListForTheReservedCall(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	settings.MaxModelCalls = 4
	model := &fakeModel{results: []JudgeResult{recallFor("first"), wantsNoteOf("a thought", 0), recallFor("second"), answeredFinal()}}
	turn := NewTurn(noteGraph(), model, nil, fixedSystem("system"), "test-model", testLogger())
	turn.Notes = &fakeNotes{}

	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: noteSubject, Settings: settings, View: noteView()})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if record.ModelCalls != 4 || !record.CapReached {
		t.Fatalf("the run made %d calls with the cap reached = %v, want 4 calls ending on the reserved call", record.ModelCalls, record.CapReached)
	}

	withNote := []string{ToolRecall, ToolReadNode, ToolNote}
	withoutNote := []string{ToolRecall, ToolReadNode}
	want := [][]string{withNote, withNote, withoutNote, {}}
	if len(record.Offered) != len(want) {
		t.Fatalf("offered = %v, want one entry per model call: %v", record.Offered, want)
	}
	for i := range want {
		if !slices.Equal(record.Offered[i], want[i]) {
			t.Errorf("call %d offered %v on the record, want %v", i+1, record.Offered[i], want[i])
		}
		if !slices.Equal(record.Offered[i], model.calls[i].Offered) {
			t.Errorf("call %d: the record says %v was offered, the call offered %v", i+1, record.Offered[i], model.calls[i].Offered)
		}
	}

	const wantJSON = `[["recall","readNode","note"],["recall","readNode","note"],["recall","readNode"],[]]`
	if got := string(offeredMember(t, record)); got != wantJSON {
		t.Errorf("offered encodes as %s, want %s: the reserved call is an empty list, never null and never missing", got, wantJSON)
	}
}

func TestTheRecordOfATurnWithoutANoteConfigStillCarriesOneOfferedEntryPerCall(t *testing.T) {
	t.Parallel()

	record, _ := runRecalls(t, baseGraph(), recallFor("more"), answeredFinal())

	if len(record.Offered) != record.ModelCalls || record.ModelCalls != 2 {
		t.Fatalf("offered = %v for %d model calls, want one entry per call", record.Offered, record.ModelCalls)
	}
	for i, offered := range record.Offered {
		if !slices.Equal(offered, []string{ToolRecall, ToolReadNode}) {
			t.Errorf("call %d offered %v, want recall and readNode: no file port and no note config", i+1, offered)
		}
	}
}

func TestTheRecordNamesTheFileToolOnlyWhenTheTurnCanWrite(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{answeredFinal()}}
	turn := NewTurn(baseGraph(), model, &fakeFiles{dir: "/runs/1"}, fixedSystem("system"), "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := [][]string{{ToolRecall, ToolReadNode, ToolWriteFile}}; len(record.Offered) != 1 || !slices.Equal(record.Offered[0], want[0]) {
		t.Fatalf("offered = %v, want %v", record.Offered, want)
	}
}

func TestALegacyRecordWithoutOfferedDecodesWithNoEntries(t *testing.T) {
	t.Parallel()

	var record Record
	if err := json.Unmarshal([]byte(`{"input":"x","answer":"a","model":"m1","modelCalls":2,"trailers":["clause","found"]}`), &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if record.Offered != nil {
		t.Fatalf("Offered = %v, want none for a record that predates the member", record.Offered)
	}
	if len(record.Trailers) != 2 {
		t.Fatalf("Trailers = %v, want the legacy members untouched", record.Trailers)
	}
}

func TestARecordRoundTripsItsOfferedListsIncludingTheEmptyOne(t *testing.T) {
	t.Parallel()

	var record Record
	if err := json.Unmarshal([]byte(`{"offered":[["recall","readNode","note"],[]]}`), &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(record.Offered) != 2 || record.Offered[1] == nil || len(record.Offered[1]) != 0 {
		t.Fatalf("Offered = %#v, want two entries, the second an empty non-nil list", record.Offered)
	}
	if got, want := string(offeredMember(t, record)), `[["recall","readNode","note"],[]]`; got != want {
		t.Fatalf("offered re-encodes as %s, want %s", got, want)
	}
}

func assertOfferedMatchesCalls(t *testing.T, record Record, calls []JudgeInput) {
	t.Helper()

	if len(record.Offered) != record.ModelCalls || len(record.Offered) != len(calls) {
		t.Fatalf("record.Offered has %d entries for %d model calls and %d issued calls, want one per call", len(record.Offered), record.ModelCalls, len(calls))
	}
	for i := range calls {
		if !slices.Equal(record.Offered[i], calls[i].Offered) {
			t.Errorf("call %d: the record says %v was offered, the call offered %v", i+1, record.Offered[i], calls[i].Offered)
		}
	}
	if last := record.Offered[len(record.Offered)-1]; last == nil || len(last) != 0 {
		t.Errorf("the reserved call recorded %#v, want an empty non-nil list", last)
	}
}
