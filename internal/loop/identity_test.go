package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const identityText = "You are a participant named Quill.\n\nSpeak as yourself."

func layeredSystem(offered []string, _, _ bool, _ Form, _ ResponseMode) string {
	return "product for " + strings.Join(offered, ",") + "\n\nsecond paragraph"
}

func runWithIdentity(t *testing.T, graph *fakeGraph, model *fakeModel, system func([]string, bool, bool, Form, ResponseMode) string, view *View, settings RunSettings) Record {
	t.Helper()

	turn := NewTurn(graph, model, nil, system, "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, Settings: settings, View: view})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	return record
}

func TestEveryJudgementCallCarriesTheIdentityThenABlankLineThenTheProductText(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap()}
	record := runWithIdentity(t, graphYieldingNewRowsToEveryRecall(), model, layeredSystem, &View{Config: 9, Identity: identityText}, DefaultSettings())

	if !record.CapReached || len(model.calls) != MaxModelCalls {
		t.Fatalf("the run made %d calls with the cap reached = %v, want %d calls reaching the reserved answering call", len(model.calls), record.CapReached, MaxModelCalls)
	}
	if last := model.calls[len(model.calls)-1]; len(last.Offered) != 0 {
		t.Fatalf("the last call offered %v, want the reserved call that offers no tool", last.Offered)
	}
	for i, call := range model.calls {
		want := identityText + "\n\n" + layeredSystem(call.Offered, false, false, FormSubstance, "")
		if call.System != want {
			t.Errorf("call %d carried the system text %q, want %q", i, call.System, want)
		}
	}
}

func TestALongMultibyteIdentityReachesEveryJudgementCallVerbatim(t *testing.T) {
	t.Parallel()

	identity := strings.Repeat("Du bist Überzeugt 世.\n", 700)
	if len(identity) < 10_000 {
		t.Fatalf("the fixture is %d bytes, want at least 10000", len(identity))
	}
	model := &fakeModel{results: researchToTheCap()}
	record := runWithIdentity(t, graphYieldingNewRowsToEveryRecall(), model, layeredSystem, &View{Config: 9, Identity: identity}, DefaultSettings())

	if record.Perspective == nil || record.Perspective.Identity != identity {
		t.Errorf("the record's perspective does not carry the identity verbatim")
	}
	for i, call := range model.calls {
		if want := identity + "\n\n" + layeredSystem(call.Offered, false, false, FormSubstance, ""); call.System != want {
			t.Errorf("call %d carried a system text of %d bytes, want the identity verbatim then the product text (%d bytes)", i, len(call.System), len(want))
		}
	}
}

func TestWithoutAnIdentityTheSystemTextIsTheProductTextByteForByte(t *testing.T) {
	t.Parallel()

	for name, view := range map[string]*View{"no config": nil, "a config without an identity": {Config: 9}} {
		model := &fakeModel{results: []JudgeResult{recallFor("more"), answeredFinal()}}
		runWithIdentity(t, baseGraph(), model, layeredSystem, view, DefaultSettings())

		if len(model.calls) != 2 {
			t.Fatalf("%s: the run made %d calls, want 2", name, len(model.calls))
		}
		for i, call := range model.calls {
			if want := layeredSystem(call.Offered, false, false, FormSubstance, ""); call.System != want {
				t.Errorf("%s: call %d carried %q, want %q with no leading separator", name, i, call.System, want)
			}
		}
	}
}

func TestAnIdentityWithoutAProductTextIsTheWholeSystemTextWithNoTrailingSeparator(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{answeredFinal()}}
	runWithIdentity(t, baseGraph(), model, nil, &View{Config: 9, Identity: identityText}, DefaultSettings())

	if got := model.calls[0].System; got != identityText {
		t.Fatalf("the system text is %q, want the identity alone %q", got, identityText)
	}
}

func TestDerivationNeverSeesTheIdentity(t *testing.T) {
	t.Parallel()

	model := &fakeModel{derivedText: "a derived query", results: []JudgeResult{answeredFinal()}}
	runWithIdentity(t, baseGraph(), model, layeredSystem, &View{Config: 9, Identity: identityText}, DefaultSettings())

	if len(model.derivePrompts) == 0 {
		t.Fatal("the run made no derivation call, so the guard observed nothing")
	}
	for i, prompt := range model.derivePrompts {
		if strings.Contains(prompt, "Quill") {
			t.Errorf("derivation prompt %d carries the identity: %q", i, prompt)
		}
	}
}

func TestTheIdentityLeavesExactlyItsOwnBytesLessRoomForASupplementaryRound(t *testing.T) {
	t.Parallel()

	admitted := func(identity string, rowBytes int) bool {
		graph := baseGraph()
		graph.node.Substance = strings.Repeat("a", 100)
		graph.recallQueue = []recallResponse{{}, {Candidates: []Candidate{bulky(8, rowBytes)}}}
		model := &fakeModel{results: []JudgeResult{recallFor("more"), answered("done")}}

		settings := DefaultSettings()
		settings.AssemblyByteBudget = 0
		settings.SupplementaryByteBudget = 1_000
		record := runWithIdentity(t, graph, model, fixedSystem("system"), &View{Config: 9, Identity: identity}, settings)
		return onlyRound(t, record).Results[0].Included
	}

	identity := strings.Repeat("i", 100)
	if !admitted("", 900) {
		t.Fatal("without an identity a 900 byte row was refused, so the room arithmetic of this test is off")
	}
	if admitted("", 901) {
		t.Fatal("without an identity a 901 byte row was admitted, so the room arithmetic of this test is off")
	}
	if !admitted(identity, 800) {
		t.Error("with a 100 byte identity a 800 byte row was refused: the identity cost more than its own bytes")
	}
	if admitted(identity, 801) {
		t.Error("with a 100 byte identity a 801 byte row was admitted: the identity was not charged to working memory")
	}
}

func TestTheIdentityIsChargedOnceSoALaterSupplementaryRoundLosesNoMoreThanItsOwnBytes(t *testing.T) {
	t.Parallel()

	const firstRow = 300
	admittedInSecondRound := func(identity string, rowBytes int) bool {
		graph := baseGraph()
		graph.node.Substance = strings.Repeat("a", 100)
		graph.recallQueue = []recallResponse{{}, {Candidates: []Candidate{bulky(8, firstRow)}}, {Candidates: []Candidate{bulky(9, rowBytes)}}}
		model := &fakeModel{results: []JudgeResult{recallFor("more"), recallFor("again"), answered("done")}}

		settings := DefaultSettings()
		settings.AssemblyByteBudget = 0
		settings.SupplementaryByteBudget = 1_000
		record := runWithIdentity(t, graph, model, fixedSystem("system"), &View{Config: 9, Identity: identity}, settings)
		if len(record.ToolCalls) != 2 {
			t.Fatalf("the record carries %d rounds, want the two recall rounds", len(record.ToolCalls))
		}
		if !record.ToolCalls[0].Results[0].Included {
			t.Fatalf("the first round refused its %d byte row, so the room arithmetic of this test is off", firstRow)
		}
		return record.ToolCalls[1].Results[0].Included
	}

	identity := strings.Repeat("i", 100)
	if !admittedInSecondRound("", 600) || admittedInSecondRound("", 601) {
		t.Fatal("without an identity the second round's room is not 600 bytes, so the room arithmetic of this test is off")
	}
	if !admittedInSecondRound(identity, 500) {
		t.Error("with a 100 byte identity a 500 byte row was refused in the second round: the identity was charged more than once")
	}
	if admittedInSecondRound(identity, 501) {
		t.Error("with a 100 byte identity a 501 byte row was admitted in the second round: the identity was not charged")
	}
}

func TestTheRecordStatesTheIdentityVerbatimAndOmitsTheKeyWhenThereIsNone(t *testing.T) {
	t.Parallel()

	with := runWithIdentity(t, baseGraph(), &fakeModel{results: []JudgeResult{answeredFinal()}}, fixedSystem("system"), &View{Config: 9, Identity: identityText}, DefaultSettings())
	if with.Perspective == nil || with.Perspective.Identity != identityText {
		t.Fatalf("the record carries the perspective %+v, want the identity verbatim", with.Perspective)
	}
	encoded, err := json.Marshal(with)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.Perspective.Identity != identityText {
		t.Fatalf("the record does not round-trip its identity: %v, %+v", err, decoded.Perspective)
	}

	without, err := json.Marshal(View{Config: 9, Filter: Filter{"type": {"thought"}}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(without), "identity") {
		t.Errorf("a view without an identity encodes as %s, want no identity key", without)
	}

	var archived View
	if err := json.Unmarshal([]byte(`{"config":9,"filter":{"type":["thought"]}}`), &archived); err != nil || archived.Identity != "" {
		t.Errorf("a view archived before the identity existed decodes to %+v, %v, want it unchanged", archived, err)
	}
}
