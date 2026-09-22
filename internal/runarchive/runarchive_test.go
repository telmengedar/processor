package runarchive

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const runName = "processor-run 2026-09-17T18:34:11Z — tell me when our garbage is collected next"

func recordJSON(t *testing.T, record loop.Record) string {
	t.Helper()
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("encode record: %v", err)
	}
	return string(encoded)
}

func fenced(account, record string) string {
	return account + "\n---\n```json\n" + record + "\n```\n"
}

func TestReadDecodesABareJSONRecord(t *testing.T) {
	t.Parallel()

	body := recordJSON(t, loop.Record{Input: "bare", Subject: 10422})

	archive := Read("one bare record", []Node{{ID: 1, Name: runName, Content: body}})

	if len(archive.Entries) != 1 || len(archive.Skipped) != 0 {
		t.Fatalf("want one entry and no skip, got %d entries and %+v", len(archive.Entries), archive.Skipped)
	}
	if archive.Entries[0].Fenced {
		t.Errorf("a record that parses whole must not be reported as fenced")
	}
	if archive.Entries[0].Record.Input != "bare" {
		t.Errorf("decoded input is %q, want %q", archive.Entries[0].Record.Input, "bare")
	}
}

func TestReadReturnsTheFencedBytesVerbatimRatherThanAReMarshalOfWhatTheyDecodedTo(t *testing.T) {
	t.Parallel()

	body := `{"input":"fenced","limits":{"candidateLimit":20},"anUnknownFieldNoRecordTypeCarries":7}`

	archive := Read("one fenced record", []Node{{ID: 2, Name: runName, Content: fenced("an account", body)}})

	if len(archive.Entries) != 1 {
		t.Fatalf("want one entry, got %d with skips %+v", len(archive.Entries), archive.Skipped)
	}
	if !archive.Entries[0].Fenced {
		t.Errorf("a record recovered from a fence must be reported as fenced")
	}
	if got := string(archive.Entries[0].Raw); got != body {
		t.Fatalf("raw bytes are not the record's own\ngot:  %s\nwant: %s", got, body)
	}
}

func TestReadNamesTheRuleThatStoppedEveryNodeItDidNotDecode(t *testing.T) {
	t.Parallel()

	nodes := []Node{
		{ID: 1, Name: runName, Content: recordJSON(t, loop.Record{Input: "fine"})},
		{ID: 2, Name: runName, Content: "   "},
		{ID: 3, Name: runName, Content: "an account with no record in it at all"},
		{ID: 4, Name: "a node nobody named like a run", Content: recordJSON(t, loop.Record{Input: "fine"})},
	}

	archive := Read("a mixed read", nodes)

	if len(archive.Entries)+len(archive.Skipped) != len(nodes) {
		t.Fatalf("%d nodes in, %d entries and %d skips out; a node that is neither is a silent drop",
			len(nodes), len(archive.Entries), len(archive.Skipped))
	}
	want := map[int64]string{2: SkipContentAbsent, 3: SkipUnrecognisedShape, 4: SkipNameUnparseable}
	for _, skip := range archive.Skipped {
		if skip.Reason == "" {
			t.Errorf("node %d was skipped with no named reason", skip.Node)
		}
		if skip.Reason != want[skip.Node] {
			t.Errorf("node %d skipped as %q, want %q", skip.Node, skip.Reason, want[skip.Node])
		}
		delete(want, skip.Node)
	}
	if len(want) != 0 {
		t.Errorf("these nodes were expected to be skipped and were not: %v", want)
	}
}

func TestReadParsesTheRunInstantFromTheNodeName(t *testing.T) {
	t.Parallel()

	archive := Read("one record", []Node{{ID: 1, Name: runName, Content: recordJSON(t, loop.Record{})}})

	want := time.Date(2026, 9, 17, 18, 34, 11, 0, time.UTC)
	if got := archive.Entries[0].At; !got.Equal(want) {
		t.Fatalf("run instant is %s, want %s", got, want)
	}
}

func TestDecodeTakesTheLastFencedBlockWhenTheAccountAboveItCarriesAFenceOfItsOwn(t *testing.T) {
	t.Parallel()

	account := "an account quoting an earlier record\n```json\n{\"input\":\"the quoted one\"}\n```\nand then the real one"
	body := `{"input":"the real one"}`

	entry, skip, ok := Decode(Node{ID: 1, Name: runName, Content: fenced(account, body)})

	if !ok {
		t.Fatalf("decode refused: %+v", skip)
	}
	if entry.Record.Input != "the real one" {
		t.Fatalf("decoded input is %q, want the last fenced block", entry.Record.Input)
	}
}

func TestANodeCarryingNoNameDecodesUndatedRatherThanBeingSkippedOrDatedToTheZeroInstant(t *testing.T) {
	t.Parallel()

	entry, skip, ok := Decode(Node{ID: 9, Content: recordJSON(t, loop.Record{Input: "a dump that lost the node name"})})

	if !ok {
		t.Fatalf("a node with no name still carries a decodable record: %+v", skip)
	}
	if entry.Dated {
		t.Fatalf("a node that states no instant must decode undated, got At %s", entry.At)
	}
	if !entry.At.IsZero() {
		t.Fatalf("an unknown instant is not an early one; At is %s", entry.At)
	}
	if entry.Record.Input != "a dump that lost the node name" {
		t.Errorf("the record itself must still decode, got %q", entry.Record.Input)
	}
}

func TestANodeWhoseNameExistsAndDoesNotParseIsStillSkipped(t *testing.T) {
	t.Parallel()

	_, skip, ok := Decode(Node{ID: 9, Name: "a node nobody named like a run", Content: recordJSON(t, loop.Record{})})

	if ok {
		t.Fatalf("a name that exists and does not parse is anomalous and must be skipped")
	}
	if skip.Reason != SkipNameUnparseable {
		t.Fatalf("skipped as %q, want %q", skip.Reason, SkipNameUnparseable)
	}
}

func TestAnEntryDecodedFromANamedNodeIsDated(t *testing.T) {
	t.Parallel()

	entry, _, ok := Decode(Node{ID: 1, Name: runName, Content: recordJSON(t, loop.Record{})})

	if !ok || !entry.Dated {
		t.Fatalf("a node whose name states an instant decodes dated, got %+v", entry)
	}
	want := time.Date(2026, 9, 17, 18, 34, 11, 0, time.UTC)
	if !entry.At.Equal(want) {
		t.Fatalf("run instant is %s, want %s", entry.At, want)
	}
}
