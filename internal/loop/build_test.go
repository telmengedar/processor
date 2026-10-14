package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const buildSentinel = "0f3a9c1d7be24a58a6c0d1e29b4f7a35c8e61d02"

func runForBuild(t *testing.T, build string) Record {
	t.Helper()

	graph := &fakeGraph{nodeFound: true}
	model := &fakeModel{results: []JudgeResult{{Answer: "an answer", Reason: Answered, RawReason: "stop"}}}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "the-configured-model-id", testLogger())
	turn.Build = build

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return record
}

func TestTheRecordCarriesTheBuildTheTurnWasStampedWithBesideTheModel(t *testing.T) {
	t.Parallel()

	record := runForBuild(t, buildSentinel)
	if buildSentinel == record.Model {
		t.Fatal("the build sentinel equals the model id, so a build copied from the model is indistinguishable from a correct one")
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got, want := string(members["build"]), `"`+buildSentinel+`"`; got != want {
		t.Fatalf("top-level build = %s, want %s", got, want)
	}
	if !strings.Contains(string(encoded), `"model":"the-configured-model-id","build":"`+buildSentinel+`"`) {
		t.Fatalf("build does not sit directly after model in the record; record=%s", encoded)
	}
}

func TestAnUnstampedTurnWritesNoBuildMemberAndNoPlaceholder(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(runForBuild(t, ""))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if raw, present := members["build"]; present {
		t.Fatalf("an unstamped turn wrote build = %s, want no build member at all", raw)
	}
	if strings.Contains(string(encoded), `"build"`) {
		t.Fatalf("an unstamped turn's record mentions build somewhere; record=%s", encoded)
	}
}

func TestALegacyRecordWithoutBuildDecodesEmptyAndReencodesWithoutIt(t *testing.T) {
	t.Parallel()

	var record Record
	if err := json.Unmarshal([]byte(`{"input":"x","answer":"a","model":"m1"}`), &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if record.Build != "" {
		t.Fatalf("Build = %q, want empty for a record that predates the member", record.Build)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"build"`) {
		t.Fatalf("a legacy record gained a build member on re-encoding; record=%s", encoded)
	}
}

func TestAStampedRecordRoundTripsItsBuildUnchanged(t *testing.T) {
	t.Parallel()

	var record Record
	if err := json.Unmarshal([]byte(`{"input":"x","answer":"a","model":"m1","build":"`+buildSentinel+`"}`), &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if record.Build != buildSentinel {
		t.Fatalf("Build = %q, want %q", record.Build, buildSentinel)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"build":"`+buildSentinel+`"`) {
		t.Fatalf("the build did not survive re-encoding; record=%s", encoded)
	}
}
