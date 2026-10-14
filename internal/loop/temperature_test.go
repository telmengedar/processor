package loop

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func temperatureOf(v float64) *float64 { return &v }

func runWithView(t *testing.T, model *fakeModel, view *View) Record {
	t.Helper()

	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, Settings: DefaultSettings(), View: view})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	return record
}

func TestEveryJudgementCallOfAViewWithATemperatureCarriesItIncludingTheToolCallsAndTheReservedAnsweringCall(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchThenAnswer("done")}
	record := runWithView(t, model, &View{Config: 1, Temperature: temperatureOf(0.7)})

	if !record.CapReached || record.ModelCalls != MaxModelCalls {
		t.Fatalf("modelCalls = %d and capReached = %v, want a run that dispatched tools up to a reserved last call", record.ModelCalls, record.CapReached)
	}
	if len(model.calls) != MaxModelCalls {
		t.Fatalf("the model saw %d judgement calls, want %d", len(model.calls), MaxModelCalls)
	}
	for i, call := range model.calls {
		if call.Temperature == nil || *call.Temperature != 0.7 {
			t.Errorf("judgement call %d (offering %v) carried temperature %v, want 0.7", i, call.Offered, call.Temperature)
		}
	}
	if last := model.calls[len(model.calls)-1]; len(last.Offered) != 0 {
		t.Errorf("the last call offered %v, want the reserved call that offers no tool", last.Offered)
	}
}

func TestAZeroTemperatureIsCarriedAsZeroAndNotAsNone(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{answered("done")}}
	runWithView(t, model, &View{Config: 1, Temperature: temperatureOf(0)})

	if got := model.calls[0].Temperature; got == nil || *got != 0 {
		t.Fatalf("the call carried temperature %v, want 0", got)
	}
}

func TestADerivationCallIsReportedAtTheDeploymentSamplingWhateverTemperatureTheViewAsks(t *testing.T) {
	t.Parallel()

	model := &fakeModel{
		derivedText:     "an earlier phrasing of the request",
		derivedSampling: Sampling{Temperature: temperatureOf(0.2)},
		results:         []JudgeResult{{Answer: "done", Reason: Answered, RawReason: "stop", Sampling: Sampling{Temperature: temperatureOf(0.7)}}},
	}
	record := runWithView(t, model, &View{Config: 1, Temperature: temperatureOf(0.7)})

	if record.DerivationSampling == nil || record.DerivationSampling.Temperature == nil || *record.DerivationSampling.Temperature != 0.2 {
		t.Errorf("derivationSampling = %+v, want the derivation's own 0.2", record.DerivationSampling)
	}
	if record.Sampling.Temperature == nil || *record.Sampling.Temperature != 0.7 {
		t.Errorf("sampling = %+v, want the judgement calls' 0.7", record.Sampling)
	}
	if record.Perspective == nil || record.Perspective.Temperature == nil || *record.Perspective.Temperature != 0.7 {
		t.Errorf("perspective = %+v, want the temperature the config asked for", record.Perspective)
	}
}

func TestTheModelPortOffersDerivationNoWayToReceiveATemperature(t *testing.T) {
	t.Parallel()

	method, ok := reflect.TypeOf((*ModelPort)(nil)).Elem().MethodByName("Derive")
	if !ok {
		t.Fatal("the model port has no Derive")
	}
	if got := method.Type.NumIn(); got != 3 {
		t.Fatalf("Derive takes %d parameters, want the context, the prompt and the output limit alone", got)
	}
}

func TestARunWithNoViewOrAViewWithoutATemperatureSendsNoPerCallTemperatureAndRecordsNone(t *testing.T) {
	t.Parallel()

	for name, view := range map[string]*View{"no view": nil, "a view without a temperature": {Config: 1}} {
		results := researchThenAnswer("done")
		for i := range results {
			results[i].Sampling = Sampling{Temperature: temperatureOf(0.2)}
		}
		model := &fakeModel{results: results, derivedText: "an earlier phrasing", derivedSampling: Sampling{Temperature: temperatureOf(0.2)}}
		record := runWithView(t, model, view)

		for i, call := range model.calls {
			if call.Temperature != nil {
				t.Errorf("%s: judgement call %d carried temperature %v, want none", name, i, *call.Temperature)
			}
		}
		if record.Perspective != nil && record.Perspective.Temperature != nil {
			t.Errorf("%s: perspective carries temperature %v", name, *record.Perspective.Temperature)
		}
		if record.DerivationSampling == nil || !reflect.DeepEqual(*record.DerivationSampling, record.Sampling) {
			t.Errorf("%s: derivationSampling = %+v, want it equal to sampling %+v when no config chose a temperature", name, record.DerivationSampling, record.Sampling)
		}
	}
}

func TestAViewRecordsItsTemperatureOnlyWhenTheConfigAskedForOne(t *testing.T) {
	t.Parallel()

	without, err := json.Marshal(View{Config: 1})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(without), "temperature") {
		t.Errorf("a view without a temperature encodes as %s", without)
	}

	with, err := json.Marshal(View{Config: 1, Temperature: temperatureOf(0)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(with), `"temperature":0`) {
		t.Errorf("a view asking for temperature 0 encodes as %s, want the key present", with)
	}
}

func TestARecordWithoutDerivationSamplingDecodesAndEncodesWithoutIt(t *testing.T) {
	t.Parallel()

	var archived Record
	if err := json.Unmarshal([]byte(`{"input":"hello","sampling":{"temperature":0.2}}`), &archived); err != nil {
		t.Fatalf("an archived record without the field does not decode: %v", err)
	}
	if archived.DerivationSampling != nil {
		t.Fatalf("derivationSampling = %+v, want absent for a record that predates it", archived.DerivationSampling)
	}

	encoded, err := json.Marshal(archived)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "derivationSampling") {
		t.Fatalf("a record that held no derivation sampling encodes one: %s", encoded)
	}

	archived.DerivationSampling = &Sampling{Temperature: temperatureOf(0.2)}
	encoded, err = json.Marshal(archived)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"derivationSampling":{"temperature":0.2}`) {
		t.Fatalf("a record with derivation sampling encodes as %s", encoded)
	}
}

func TestTheSummaryNamesTheDerivationSamplingOnlyWhenItDiffersFromTheJudgementCalls(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		sampling, derivation *Sampling
		want                 string
	}{
		"they agree":                      {&Sampling{Temperature: temperatureOf(0.2)}, &Sampling{Temperature: temperatureOf(0.2)}, "192 tok judge  temp=0.2\n"},
		"no derivation sampling recorded": {&Sampling{Temperature: temperatureOf(0.7)}, nil, "192 tok judge  temp=0.7\n"},
		"the temperatures differ":         {&Sampling{Temperature: temperatureOf(0.7)}, &Sampling{Temperature: temperatureOf(0.2)}, "192 tok judge  temp=0.7, derivation temp=0.2\n"},
		"the top p values differ":         {&Sampling{Temperature: temperatureOf(0.7), TopP: temperatureOf(0.9)}, &Sampling{Temperature: temperatureOf(0.7)}, "192 tok judge  temp=0.7 topP=0.9, derivation temp=0.7\n"},
		"the derivation sent nothing":     {&Sampling{Temperature: temperatureOf(0.7)}, &Sampling{}, "192 tok judge  temp=0.7, derivation sent none\n"},
		"only the derivation differs":     {&Sampling{}, &Sampling{Temperature: temperatureOf(0.2)}, "192 tok judge  derivation temp=0.2\n"},
	}
	for name, tc := range cases {
		record := summaryRecord()
		record.Sampling = *tc.sampling
		record.DerivationSampling = tc.derivation

		if summary := RenderSummary(record, summaryInstant()); !strings.Contains(summary, tc.want) {
			t.Errorf("%s: the limits line does not end in %q.\nsummary:\n%s", name, tc.want, summary)
		}
	}
}
