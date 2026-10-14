package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

var silenceOffered = &View{Config: 77, Silence: true}

func runSilence(t *testing.T, graph *fakeGraph, view *View, results ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, offeringSystem, "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, Settings: DefaultSettings(), View: view})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return record, model
}

func TestARunWithoutAConfigOffersNoSilenceAndItsRecordAndOutcomeEncodeNoSilentKey(t *testing.T) {
	t.Parallel()

	record, model := runSilence(t, baseGraph(), nil, answered(SilenceMarker))

	for i, call := range model.calls {
		if call.System != "offers:recall,readNode" {
			t.Errorf("call %d carried the system text %q, want one composed without silence", i+1, call.System)
		}
	}
	if record.Silent || record.Outcome.Silent {
		t.Errorf("record.Silent = %v and outcome.Silent = %v, want both false on a run that offered no silence", record.Silent, record.Outcome.Silent)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, key := range []string{`"silent"`, `"silence"`, `"perspective"`} {
		if strings.Contains(string(encoded), key) {
			t.Errorf("the record of a run without a config encodes %s: %s", key, encoded)
		}
	}
}

func TestAConfiguredRunThatDoesNotSetSilenceOffersNoneAndEncodesNoSilenceKey(t *testing.T) {
	t.Parallel()

	record, model := runSilence(t, baseGraph(), &View{Config: 77}, answered("a reply"))

	if got := model.calls[0].System; strings.Contains(got, "silence") {
		t.Errorf("the system text %q names silence, want none when the config did not set it", got)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"silence"`) || strings.Contains(string(encoded), `"silent"`) {
		t.Errorf("the record of a configured run without silence encodes a silence key: %s", encoded)
	}
}

func TestAnAnswerCarryingTheMarkerIsDeliveredVerbatimWhenSilenceWasNotOffered(t *testing.T) {
	t.Parallel()

	for name, view := range map[string]*View{"no config": nil, "config without silence": {Config: 77}} {
		record, _ := runSilence(t, baseGraph(), view, answered("the marker is "+SilenceMarker))

		if record.Silent || record.Outcome.Silent {
			t.Errorf("%s: the run is silent, want a marker in the answer ignored when silence was not offered", name)
		}
		if !record.Outcome.Produced || record.Outcome.Verdict == VerdictSilent {
			t.Errorf("%s: outcome = %+v, want a produced answer", name, record.Outcome)
		}
	}
}

func TestAnAnswerOfTheMarkerOnARunThatOfferedSilenceIsAChosenSilenceAndTheRunIsStillFiled(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	record, model := runSilence(t, graph, silenceOffered, answered(SilenceMarker))

	if !strings.Contains(model.calls[0].System, ";silence") {
		t.Errorf("the system text %q does not carry silence on a run that offered it", model.calls[0].System)
	}
	if !record.Silent || !record.Outcome.Silent {
		t.Fatalf("record.Silent = %v and outcome.Silent = %v, want both true", record.Silent, record.Outcome.Silent)
	}
	if record.Outcome.Produced {
		t.Error("outcome.Produced = true, want false: a chosen silence leaves nothing to deliver")
	}
	if record.Outcome.Verdict != VerdictSilent {
		t.Errorf("outcome.Verdict = %q, want %q", record.Outcome.Verdict, VerdictSilent)
	}
	if !graph.writeRunCalled || !graph.writeRunRecord.Silent {
		t.Errorf("writeRunCalled = %v, filed silent = %v, want the silent run filed as a record", graph.writeRunCalled, graph.writeRunRecord.Silent)
	}
	if record.Perspective == nil || !record.Perspective.Silence {
		t.Errorf("the record's perspective is %+v, want it to say silence was offered", record.Perspective)
	}
}

func TestTheMarkerIsRecognisedWhateverItsCaseSpacingOrSpeakerPrefix(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{strings.ToLower(SilenceMarker), "  " + SilenceMarker + " \n", "Gangolf: " + SilenceMarker, `"` + SilenceMarker + `."`, "NoReply"} {
		record, _ := runSilence(t, baseGraph(), silenceOffered, answered(answer))

		if !record.Silent {
			t.Errorf("answer %q was not recognised as a chosen silence", answer)
		}
	}
}

func TestAMarkerInsideProseIsStillASilenceAndTheRecordKeepsTheRawAnswer(t *testing.T) {
	t.Parallel()

	answer := "Nothing to add here. " + SilenceMarker + " Carry on."
	record, _ := runSilence(t, baseGraph(), silenceOffered, answered(answer))

	if !record.Silent || record.Outcome.Produced {
		t.Errorf("silent = %v and produced = %v, want a marker mixed with prose withheld whole", record.Silent, record.Outcome.Produced)
	}
	if record.Answer != answer {
		t.Errorf("record.Answer = %q, want the raw answer %q", record.Answer, answer)
	}
}

func TestABlankAnswerOnARunThatOfferedSilenceIsAFailureAndNeverASilence(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"", " \n\t "} {
		record, _ := runSilence(t, baseGraph(), silenceOffered, answered(answer))

		if record.Silent || record.Outcome.Silent {
			t.Errorf("answer %q was taken for a chosen silence, want a failure", answer)
		}
		if record.Outcome.Produced || record.Outcome.Verdict != VerdictEmpty {
			t.Errorf("answer %q gave the outcome %+v, want not produced and verdict %q", answer, record.Outcome, VerdictEmpty)
		}
	}
}

func TestASilenceChosenOnTheReservedAnsweringCallIsRecognisedAndTheVerdictStaysCurtailed(t *testing.T) {
	t.Parallel()

	record, model := runSilence(t, graphYieldingNewRowsToEveryRecall(), silenceOffered, researchThenAnswer(SilenceMarker)...)

	if !record.CapReached {
		t.Fatal("record.CapReached = false, want the last call reserved for answering")
	}
	last := model.calls[len(model.calls)-1]
	if len(last.Offered) != 0 || !strings.Contains(last.System, ";silence") {
		t.Errorf("the reserved call offered %v with system text %q, want no tools and the silence paragraph", last.Offered, last.System)
	}
	if !record.Silent || !record.Outcome.Silent {
		t.Errorf("record.Silent = %v and outcome.Silent = %v, want the silence recognised on the reserved call", record.Silent, record.Outcome.Silent)
	}
	if record.Outcome.Verdict != VerdictCurtailed {
		t.Errorf("outcome.Verdict = %q, want %q: a curtailed turn keeps its verdict beside the silence", record.Outcome.Verdict, VerdictCurtailed)
	}
}

func TestComputeOutcomeReadsTheRecordedFactAndNeverTheAnswersProse(t *testing.T) {
	t.Parallel()

	if outcome := ComputeOutcome(Record{Answer: SilenceMarker}); outcome.Silent || !outcome.Produced || outcome.Verdict == VerdictSilent {
		t.Errorf("outcome = %+v for a record whose answer holds the marker but which has no silent fact, want an ordinary produced answer", outcome)
	}

	outcome := ComputeOutcome(Record{Answer: "quiet", Silent: true})
	if !outcome.Silent || outcome.Produced || outcome.Verdict != VerdictSilent {
		t.Errorf("outcome = %+v for a record carrying the silent fact, want silent, not produced, verdict %q", outcome, VerdictSilent)
	}
}

func TestACurtailedOutcomeOutranksSilenceInTheVerdictAndKeepsBothFlags(t *testing.T) {
	t.Parallel()

	outcome := ComputeOutcome(Record{Answer: SilenceMarker, Silent: true, CapReached: true})

	if outcome.Verdict != VerdictCurtailed || !outcome.Silent || !outcome.Curtailed {
		t.Errorf("outcome = %+v, want verdict %q with silent and curtailed both set", outcome, VerdictCurtailed)
	}
}

func TestAChosenSilenceIsLoggedAsInformationAndRaisesNoDeliveryWarning(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	turn := NewTurn(baseGraph(), &fakeModel{results: []JudgeResult{answered(SilenceMarker)}}, nil, offeringSystem, "test-model", slog.New(slog.NewTextHandler(&logged, nil)))
	if _, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, Settings: DefaultSettings(), View: silenceOffered}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if strings.Contains(logged.String(), "did not deliver") {
		t.Errorf("the log carries the delivery warning for a chosen silence: %s", logged.String())
	}
	if !strings.Contains(logged.String(), "chose silence") {
		t.Errorf("the log does not record that the run chose silence: %s", logged.String())
	}
}

func TestTheSummaryNamesASilenceOnlyWhenTheRunChoseOne(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	silent, _ := runSilence(t, baseGraph(), silenceOffered, answered(SilenceMarker))
	spoken, _ := runSilence(t, baseGraph(), silenceOffered, answered("a reply"))

	if got := RenderSummary(silent, at); !strings.Contains(got, "verdict  silent (produced no") || !strings.Contains(got, "silent yes") {
		t.Errorf("the summary of a silent run does not name the silence:\n%s", got)
	}
	if got := RenderSummary(spoken, at); strings.Contains(got, "silent yes") {
		t.Errorf("the summary of a run that replied names a silence:\n%s", got)
	}
}

func TestTheOutcomeOfAnArchivedRecordWithoutTheSilentFieldDecodesAsNotSilent(t *testing.T) {
	t.Parallel()

	var record Record
	if err := json.Unmarshal([]byte(`{"answer":"`+SilenceMarker+`","outcome":{"verdict":"delivered","produced":true,"grounded":true,"curtailed":false}}`), &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if record.Silent || record.Outcome.Silent || record.Outcome.Verdict != VerdictDelivered {
		t.Errorf("record.Silent = %v and outcome = %+v, want an archived record read as it was written", record.Silent, record.Outcome)
	}
}
