package ledger

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runarchive"
)

type dumped struct {
	instant string
	raw     string
}

func decoded(t *testing.T, id int64, name, raw string) runarchive.Entry {
	t.Helper()
	entry, skip, ok := runarchive.Decode(runarchive.Node{ID: id, Name: name, Content: raw})
	if !ok {
		t.Fatalf("fixture %d did not decode: %s %s", id, skip.Reason, skip.Detail)
	}
	return entry
}

func archiveOf(t *testing.T, selector string, raws ...string) runarchive.Archive {
	t.Helper()
	records := make([]dumped, 0, len(raws))
	for _, raw := range raws {
		records = append(records, dumped{instant: "2026-09-17T18:34:11Z", raw: raw})
	}
	return datedArchive(t, selector, records...)
}

func datedArchive(t *testing.T, selector string, records ...dumped) runarchive.Archive {
	t.Helper()
	entries := make([]runarchive.Entry, 0, len(records))
	for i, record := range records {
		name := ""
		if record.instant != "" {
			name = "processor-run " + record.instant + " — a run"
		}
		entries = append(entries, decoded(t, int64(i+1), name, record.raw))
	}
	return runarchive.Archive{Selector: selector, Entries: entries}
}

func readingFor(t *testing.T, result DialExerciseResult, name string) DialReading {
	t.Helper()
	for _, reading := range result.Readings {
		if reading.Dial == name {
			return reading
		}
	}
	t.Fatalf("no reading for dial %q in %+v", name, result.Readings)
	return DialReading{}
}

func failedOn(failures []Failure, subject string) bool {
	for _, failure := range failures {
		if failure.Subject == subject {
			return true
		}
	}
	return false
}

const newline = "\n"

func valueList(values []DialValue) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, value.Value)
	}
	return strings.Join(parts, ",")
}

func TestDialExerciseCallsADialUnrecordedWhenNoRecordCarriesItsFieldAtAll(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "records predating the dial",
		`{"limits":{"candidateLimit":20}}`,
		`{"limits":{"candidateLimit":20}}`,
	)

	result := DialExercise(archive)

	reading := readingFor(t, result, "SubstanceRatioThreshold")
	if reading.Verdict != DialUnrecorded {
		t.Fatalf("a dial no record carries is %q, want %q; decoding an absent field as its zero value would make it look recorded", reading.Verdict, DialUnrecorded)
	}
	if reading.RecordedIn != 0 || len(reading.Values) != 0 {
		t.Fatalf("an absent field is recorded nowhere and has no values, got %+v", reading)
	}
	if !failedOn(result.Failures, "SubstanceRatioThreshold") {
		t.Fatalf("an unrecorded dial must fail the property, got %+v", result.Failures)
	}
}

func TestDialExerciseCountsOnlyValuesTheRecordItselfCarries(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "one record carries the floor and one predates it",
		`{"limits":{"relevanceFloor":0.63}}`,
		`{"limits":{"candidateLimit":20}}`,
	)

	reading := readingFor(t, DialExercise(archive), "RelevanceFloor")

	if reading.Verdict != DialUnexercised {
		t.Fatalf("the floor took one recorded value and the other record carries no floor at all; verdict is %q, want %q", reading.Verdict, DialUnexercised)
	}
	if reading.RecordedIn != 1 {
		t.Fatalf("the floor is recorded in 1 record, got %d", reading.RecordedIn)
	}
}

func TestDialExerciseCallsADialExercisedOnlyOnTwoDistinctRecordedValues(t *testing.T) {
	t.Parallel()

	same := archiveOf(t, "two records at one value",
		`{"limits":{"maxModelCalls":6}}`,
		`{"limits":{"maxModelCalls":6}}`,
	)
	if got := readingFor(t, DialExercise(same), "MaxModelCalls").Verdict; got != DialUnexercised {
		t.Errorf("one recorded value is %q, want %q", got, DialUnexercised)
	}

	moved := archiveOf(t, "two records at two values",
		`{"limits":{"maxModelCalls":3}}`,
		`{"limits":{"maxModelCalls":6}}`,
	)
	result := DialExercise(moved)
	reading := readingFor(t, result, "MaxModelCalls")
	if reading.Verdict != DialExercised {
		t.Errorf("two recorded values are %q, want %q", reading.Verdict, DialExercised)
	}
	if valueList(reading.Values) != "3,6" {
		t.Errorf("recorded values are %v, want both verbatim", reading.Values)
	}
	if failedOn(result.Failures, "MaxModelCalls") {
		t.Errorf("an exercised dial must not fail the property, got %+v", result.Failures)
	}
}

func TestEachRecordedValueCarriesTheInstantRangeTheRecordsHoldingItSpan(t *testing.T) {
	t.Parallel()

	archive := datedArchive(t, "a dial edited between two spans",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-06T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
		dumped{instant: "2026-09-17T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
	)

	reading := readingFor(t, DialExercise(archive), "MaxModelCalls")

	if len(reading.Values) != 2 {
		t.Fatalf("want two recorded values, got %+v", reading.Values)
	}
	first, second := reading.Values[0], reading.Values[1]
	if first.Value != "3" || first.RecordedIn != 2 ||
		first.From.Format("2006-01-02") != "2026-09-02" || first.To.Format("2006-01-02") != "2026-09-06" {
		t.Errorf("first value spans %+v, want 3 over 2026-09-02 to 2026-09-06", first)
	}
	if second.Value != "6" || second.RecordedIn != 2 ||
		second.From.Format("2006-01-02") != "2026-09-07" || second.To.Format("2006-01-02") != "2026-09-17" {
		t.Errorf("second value spans %+v, want 6 over 2026-09-07 to 2026-09-17", second)
	}
}

func TestDisjointValueSpansReadAsSequentialAndOverlappingOnesAsInterleaved(t *testing.T) {
	t.Parallel()

	edited := datedArchive(t, "a constant edited between runs",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
	)
	if got := readingFor(t, DialExercise(edited), "MaxModelCalls").Succession; got != SuccessionSequential {
		t.Errorf("two values in disjoint spans are %q, want %q", got, SuccessionSequential)
	}

	swept := datedArchive(t, "a dial varied inside one span",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
		dumped{instant: "2026-09-09T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
	)
	if got := readingFor(t, DialExercise(swept), "MaxModelCalls").Succession; got != SuccessionInterleaved {
		t.Errorf("two values in overlapping spans are %q, want %q", got, SuccessionInterleaved)
	}
}

func TestAValueAnUndatedRecordCarriesMakesTheSuccessionUndatable(t *testing.T) {
	t.Parallel()

	archive := datedArchive(t, "one record states no instant",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{raw: `{"limits":{"maxModelCalls":6}}`},
	)

	reading := readingFor(t, DialExercise(archive), "MaxModelCalls")

	if reading.Succession != SuccessionUndatable {
		t.Fatalf("a value carried by an undated record cannot be ordered against another, got %q", reading.Succession)
	}
	for _, value := range reading.Values {
		if value.Value == "6" && value.Undated != 1 {
			t.Errorf("value 6 is carried by 1 undated record, got %+v", value)
		}
	}
}

func TestAnExercisedDialWhoseValuesDoNotShareASpanIsRenderedWithACaveat(t *testing.T) {
	t.Parallel()

	sequential := datedArchive(t, "a constant edited between runs",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
	)
	rendered := RenderProperties(DialExercise(sequential), MechanismObservation(sequential))
	if !strings.Contains(rendered, "CAVEAT") {
		t.Errorf("an exercised dial whose values occupy disjoint spans must be quoted with its caveat\n%s", rendered)
	}
	for _, want := range []string{"2026-09-02T10:00:00Z", "2026-09-07T10:00:00Z"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendered dial must carry the spans its caveat refers to, at the resolution the caveat was decided at; %q is missing%s%s", want, newline, rendered)
		}
	}

	undatable := datedArchive(t, "one record kept no name",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-08T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
		dumped{raw: `{"limits":{"maxModelCalls":6}}`},
	)
	renderedUndatable := RenderProperties(DialExercise(undatable), MechanismObservation(undatable))
	if !strings.Contains(renderedUndatable, "CAVEAT") || !strings.Contains(renderedUndatable, "do not order") {
		t.Errorf("an exercised dial whose values cannot be ordered is the reading most easily mistaken for a sweep, so its caveat must reach the rendered line%s%s", newline, renderedUndatable)
	}
	if !strings.Contains(renderedUndatable, "and 1 stating no instant") {
		t.Errorf("a value some of whose records state no instant must say how many, where the span is printed%s%s", newline, renderedUndatable)
	}

	interleaved := datedArchive(t, "a dial varied inside one span",
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
		dumped{instant: "2026-09-09T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
	)
	if strings.Contains(RenderProperties(DialExercise(interleaved), MechanismObservation(interleaved)), "CAVEAT") {
		t.Errorf("a dial varied inside one span carries no succession caveat")
	}
}

func TestDialExerciseDeclaresTheKnobNoRecordFieldCarriesAndCanNeverClearIt(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "every dial the record can carry, all at two values",
		`{"limits":{"candidateLimit":20,"assemblyByteBudget":60000,"supplementaryByteBudget":20000,"maxModelCalls":3,"maxOutputTokens":4096,"relevanceFloor":0,"maxFills":3,"fillSizeFloor":1,"maxFillContentBytes":1,"substanceRatioThreshold":0}}`,
		`{"limits":{"candidateLimit":21,"assemblyByteBudget":60001,"supplementaryByteBudget":20001,"maxModelCalls":6,"maxOutputTokens":4097,"relevanceFloor":0.63,"maxFills":4,"fillSizeFloor":2,"maxFillContentBytes":2,"substanceRatioThreshold":0.5}}`,
	)

	result := DialExercise(archive)

	reading := readingFor(t, result, "BlockOccupancy")
	if reading.Field != "" {
		t.Fatalf("BlockOccupancy is declared with a record field %q; it has none", reading.Field)
	}
	if reading.Verdict != DialUnrecorded {
		t.Fatalf("a knob with no record field is %q, want %q", reading.Verdict, DialUnrecorded)
	}
	if len(result.Failures) != 1 || result.Failures[0].Subject != "BlockOccupancy" {
		t.Fatalf("every other dial moved, so BlockOccupancy alone must fail, got %+v", result.Failures)
	}
}

func TestARecordWhoseOwnBytesCarryNoReadableLimitsIsNamedRatherThanSwallowed(t *testing.T) {
	t.Parallel()

	archive := runarchive.Archive{Selector: "one record was hand-edited", Entries: []runarchive.Entry{
		decoded(t, 1, "processor-run 2026-09-17T18:34:11Z — a run", `{"limits":{"candidateLimit":20,"assemblyByteBudget":60000,"maxModelCalls":6,"relevanceFloor":0.63}}`),
		{Node: 77, Raw: []byte(`{"limits":5}`)},
	}}

	result := DialExercise(archive)

	if len(result.Unreadable) != 1 || result.Unreadable[0] != 77 {
		t.Fatalf("the unreadable record must be named, got %v", result.Unreadable)
	}
	if !failedOn(result.Failures, "record 77") {
		t.Fatalf("an unreadable record must fail the property rather than be dropped, got %+v", result.Failures)
	}
	for _, name := range []string{"CandidateLimit", "AssemblyByteBudget", "MaxModelCalls", "RelevanceFloor"} {
		if got := readingFor(t, result, name).RecordedIn; got != 1 {
			t.Errorf("the unreadable record contributes to no dial, so %s is recorded in 1, got %d", name, got)
		}
	}
	for _, name := range []string{"MaxFills", "SubstanceRatioThreshold"} {
		if got := readingFor(t, result, name).RecordedIn; got != 0 {
			t.Errorf("neither record carries %s, so it is recorded in 0, got %d", name, got)
		}
	}
}

func TestTwoValuesMeetingAtOneInstantShareThatInstantAndReadAsInterleaved(t *testing.T) {
	t.Parallel()

	archive := datedArchive(t, "two values whose spans touch at a single instant",
		dumped{instant: "2026-09-07T12:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T12:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
	)

	reading := readingFor(t, DialExercise(archive), "MaxModelCalls")

	if reading.Succession != SuccessionInterleaved {
		t.Fatalf("spans that touch are not disjoint, so the succession is %q, want %q", reading.Succession, SuccessionInterleaved)
	}
}

func TestEveryDialFailureNamesTheSelectorAndPopulationItWasComputedOver(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "a named selector", `{"limits":{"candidateLimit":20}}`)

	result := DialExercise(archive)

	if result.Selector != "a named selector" || result.Population != 1 {
		t.Fatalf("result names selector %q over population %d", result.Selector, result.Population)
	}
	for _, failure := range result.Failures {
		if failure.Selector != "a named selector" || failure.Population != 1 {
			t.Fatalf("failure %q carries selector %q and population %d", failure.Subject, failure.Selector, failure.Population)
		}
	}
}

func TestMechanismObservationFailsForAMechanismNoRecordObserves(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "records carrying no fill", `{"input":"a run"}`, `{"input":"another run","fills":[]}`)

	result := MechanismObservation(archive)

	if len(result.Failures) != len(mechanisms) {
		t.Fatalf("no mechanism was observed, so all %d must fail, got %+v", len(mechanisms), result.Failures)
	}
	if !failedOn(result.Failures, "on-demand fill") || !failedOn(result.Failures, "substance form") {
		t.Fatalf("want both declared mechanisms named, got %+v", result.Failures)
	}
	if result.Population != 2 {
		t.Fatalf("population is %d, want 2", result.Population)
	}
}

func TestMechanismObservationClearsAMechanismOneRecordObserves(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "one record carries a fill",
		`{"input":"a run"}`,
		`{"input":"a filled run","fills":[{"id":7}]}`,
	)

	result := MechanismObservation(archive)

	if failedOn(result.Failures, "on-demand fill") {
		t.Fatalf("one observation clears the mechanism, got %+v", result.Failures)
	}
	for _, reading := range result.Readings {
		if reading.Mechanism == "on-demand fill" && reading.ObservedIn != 1 {
			t.Fatalf("fill observed in %d records, want 1", reading.ObservedIn)
		}
	}
}

func TestMechanismObservationSeesASubstanceFormInASupplementaryRoundAsWellAsInTheBlock(t *testing.T) {
	t.Parallel()

	inRound := runarchive.Archive{Selector: "a supplementary round rendered a substance", Entries: []runarchive.Entry{{
		Record: loop.Record{ToolCalls: []loop.ToolCallRecord{{Tool: loop.ToolRecall, Results: []loop.Disposition{{ID: 1, Form: loop.FormSubstance}}}}},
	}}}
	if failedOn(MechanismObservation(inRound).Failures, "substance form") {
		t.Errorf("a substance rendered in a supplementary round is an observation")
	}

	inBlock := runarchive.Archive{Selector: "the block rendered a substance", Entries: []runarchive.Entry{{
		Record: loop.Record{Candidates: []loop.Disposition{{ID: 1, Form: loop.FormSubstance}}},
	}}}
	if failedOn(MechanismObservation(inBlock).Failures, "substance form") {
		t.Errorf("a substance rendered in the block is an observation")
	}

	neither := runarchive.Archive{Selector: "content everywhere", Entries: []runarchive.Entry{{
		Record: loop.Record{Candidates: []loop.Disposition{{ID: 1, Form: loop.FormContent}}},
	}}}
	if !failedOn(MechanismObservation(neither).Failures, "substance form") {
		t.Errorf("a record that rendered only content observes no substance form")
	}
}

func TestRenderedPropertiesNameEveryFailureItsSelectorAndItsPopulation(t *testing.T) {
	t.Parallel()

	archive := archiveOf(t, "the whole archive", `{"limits":{"candidateLimit":20}}`)

	rendered := RenderProperties(DialExercise(archive), MechanismObservation(archive))

	for _, want := range []string{"the whole archive", "BlockOccupancy", "SubstanceRatioThreshold", "on-demand fill", "substance form", "unrecorded", "FAIL"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered properties do not name %q\n%s", want, rendered)
		}
	}
}

func TestAValueSpanIsTheEarliestAndLatestDatedRecordCarryingIt(t *testing.T) {
	t.Parallel()

	archive := datedArchive(t, "records handed over out of order, one of them undated",
		dumped{instant: "2026-09-17T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-02T10:00:00Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-20T10:00:00Z", raw: `{"limits":{"maxModelCalls":6}}`},
	)

	reading := readingFor(t, DialExercise(archive), "MaxModelCalls")

	var three DialValue
	for _, value := range reading.Values {
		if value.Value == "3" {
			three = value
		}
	}
	if three.From.Format("2006-01-02") != "2026-09-02" {
		t.Errorf("the span starts at the earliest dated record carrying the value, not the first one handed over; got %s", three.From)
	}
	if three.To.Format("2006-01-02") != "2026-09-17" {
		t.Errorf("the span ends at the latest dated record carrying the value; got %s", three.To)
	}
	if three.Undated != 1 || three.RecordedIn != 3 {
		t.Errorf("the value is carried by 3 records, one of which states no instant; got %+v", three)
	}
}

func TestTheRenderedSpanDistinguishesTwoInstantsFallingOnOneDay(t *testing.T) {
	t.Parallel()

	archive := datedArchive(t, "a constant edited partway through a day",
		dumped{instant: "2026-09-07T01:45:22Z", raw: `{"limits":{"maxModelCalls":3}}`},
		dumped{instant: "2026-09-07T14:37:56Z", raw: `{"limits":{"maxModelCalls":6}}`},
	)

	reading := readingFor(t, DialExercise(archive), "MaxModelCalls")
	if reading.Succession != SuccessionSequential {
		t.Fatalf("the two instants are disjoint, so the succession is %q, want %q", reading.Succession, SuccessionSequential)
	}

	rendered := RenderProperties(DialExercise(archive), MechanismObservation(archive))
	for _, want := range []string{"2026-09-07T01:45:22Z", "2026-09-07T14:37:56Z"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("a reader checking %q against the printed spans must see them apart rather than as one day overlapping itself; %q is missing%s%s",
				SuccessionSequential, want, newline, rendered)
		}
	}
}
