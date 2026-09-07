package loop

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTheRecordCarriesTheAdapterAndEndpointTheModelPortReported(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: true}
	model := &fakeModel{results: []JudgeResult{{
		Answer:    "an answer",
		Reason:    Answered,
		RawReason: "stop",
		Provider:  Provider{Adapter: "an-adapter", Endpoint: "http://host.invalid/a/route"},
	}}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.Provider.Adapter != "an-adapter" {
		t.Fatalf("Provider.Adapter = %q, want %q", record.Provider.Adapter, "an-adapter")
	}
	if record.Provider.Endpoint != "http://host.invalid/a/route" {
		t.Fatalf("Provider.Endpoint = %q, want %q", record.Provider.Endpoint, "http://host.invalid/a/route")
	}
}

func TestTheRecordsProviderComesFromTheAdapterAndNotFromTheModelIDTheTurnWasBuiltWith(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: true}
	model := &fakeModel{results: []JudgeResult{{
		Reason:    Answered,
		RawReason: "stop",
		Provider:  Provider{Adapter: "the-reporting-adapter", Endpoint: "http://reported.invalid/route"},
	}}}
	turn := NewTurn(graph, model, nil, "system", "the-configured-model-id", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.Model != "the-configured-model-id" {
		t.Fatalf("Model = %q, want the id the turn was configured with", record.Model)
	}
	if record.Provider.Adapter == record.Model {
		t.Fatalf("Provider.Adapter = %q, want the adapter's own claim rather than the configured model id", record.Provider.Adapter)
	}
	if record.Provider.Adapter != "the-reporting-adapter" {
		t.Fatalf("Provider.Adapter = %q, want %q", record.Provider.Adapter, "the-reporting-adapter")
	}
}

func TestTheProviderKeysAreSerialisedIntoTheStoredRecord(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(Record{Provider: Provider{Adapter: "an-adapter", Endpoint: "http://host.invalid/a/route"}})
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}

	var decoded struct {
		Provider struct {
			Adapter  string `json:"adapter"`
			Endpoint string `json:"endpoint"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode record: %v; body=%s", err, encoded)
	}
	if decoded.Provider.Adapter != "an-adapter" {
		t.Fatalf("provider.adapter = %q, want %q; body=%s", decoded.Provider.Adapter, "an-adapter", encoded)
	}
	if decoded.Provider.Endpoint != "http://host.invalid/a/route" {
		t.Fatalf("provider.endpoint = %q, want %q; body=%s", decoded.Provider.Endpoint, "http://host.invalid/a/route", encoded)
	}
}

func TestAToolRoundRecordsWhetherTheAdapterReadTheCallNativelyOrRecoveredItFromTheText(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: true}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "stop", RecallQuery: "a query", ToolSource: ToolSourceContent},
		{Answer: "done", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(record.ToolCalls) != 1 {
		t.Fatalf("toolCalls = %d, want 1", len(record.ToolCalls))
	}
	if record.ToolCalls[0].Source != ToolSourceContent {
		t.Fatalf("toolCalls[0].Source = %q, want %q — a round the endpoint never parsed must not read as one it did", record.ToolCalls[0].Source, ToolSourceContent)
	}
}

func TestANativeToolRoundIsRecordedAsNativeAndNotAsRecoveredText(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: true}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "stop", RecallQuery: "a query", ToolSource: ToolSourceNative},
		{Answer: "done", Reason: Answered, RawReason: "stop"},
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.ToolCalls[0].Source != ToolSourceNative {
		t.Fatalf("toolCalls[0].Source = %q, want %q", record.ToolCalls[0].Source, ToolSourceNative)
	}
}

func TestACappedToolRoundStillRecordsHowTheAdapterObtainedTheCall(t *testing.T) {
	t.Parallel()

	graph := &fakeGraph{nodeFound: true}
	model := &fakeModel{results: []JudgeResult{
		{Reason: WantsRecall, RawReason: "stop", RecallQuery: "one", ToolSource: ToolSourceContent},
		{Reason: WantsRecall, RawReason: "stop", RecallQuery: "two", ToolSource: ToolSourceContent},
		{Reason: WantsRecall, RawReason: "stop", RecallQuery: "three", ToolSource: ToolSourceContent},
	}}
	turn := NewTurn(graph, model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !record.CapReached {
		t.Fatal("CapReached is false, want the run to have hit the call cap")
	}
	last := record.ToolCalls[len(record.ToolCalls)-1]
	if last.Source != ToolSourceContent {
		t.Fatalf("the capped round's Source = %q, want %q — a round recorded but never dispatched still knows where its call came from", last.Source, ToolSourceContent)
	}
}

func TestARoundWithNoToolSourceLeavesTheKeyOutOfTheStoredRecordRatherThanWritingAnEmptyOne(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(ToolCallRecord{Tool: ToolRecall, Results: []Disposition{}})
	if err != nil {
		t.Fatalf("marshal tool call record: %v", err)
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &keys); err != nil {
		t.Fatalf("decode tool call record: %v; body=%s", err, encoded)
	}
	if _, present := keys["source"]; present {
		t.Fatalf("record carries an empty source key; body=%s", encoded)
	}
}

func TestAToolSourceIsSerialisedUnderItsOwnKey(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(ToolCallRecord{Tool: ToolWriteFile, Source: ToolSourceContent, Results: []Disposition{}})
	if err != nil {
		t.Fatalf("marshal tool call record: %v", err)
	}

	var decoded struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode tool call record: %v; body=%s", err, encoded)
	}
	if decoded.Source != "content" {
		t.Fatalf("source = %q, want %q; body=%s", decoded.Source, "content", encoded)
	}
}
