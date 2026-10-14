package ollama

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func floatPtr(v float64) *float64 { return &v }

func sentSamplingOf(t *testing.T, body []byte) loop.Sampling {
	t.Helper()
	options := decodedOptions(t, body)
	var sampling loop.Sampling
	for name, target := range map[string]**float64{"temperature": &sampling.Temperature, "top_p": &sampling.TopP} {
		raw, present := options[name]
		if !present {
			continue
		}
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode options.%s: %v", name, err)
		}
		*target = &value
	}
	return sampling
}

func TestAJudgementCallsOwnTemperatureReplacesTheConfiguredOneInOptionsAndLeavesTopPAlone(t *testing.T) {
	t.Parallel()

	for _, chosen := range []float64{0, 0.7, 1.5, 50} {
		srv, captured := capturingServer(t, doneResponse)
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.95)}, srv.Client())

		if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "s", Input: "in", Temperature: floatPtr(chosen)}); err != nil {
			t.Fatalf("Judge: %v", err)
		}

		got := sentSamplingOf(t, captured.Body)
		if got.Temperature == nil || *got.Temperature != chosen {
			t.Errorf("options.temperature = %v, want %v", got.Temperature, chosen)
		}
		if got.TopP == nil || *got.TopP != 0.95 {
			t.Errorf("options.top_p = %v, want the configured 0.95 untouched", got.TopP)
		}
		if _, atTop := topLevelKeys(t, captured.Body)["temperature"]; atTop {
			t.Errorf("the request carries temperature at the top level: this protocol spells it under options; body=%s", captured.Body)
		}
	}
}

func TestAJudgementCallWithoutATemperatureSendsTheConfiguredOne(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: floatPtr(0.2)}, srv.Client())

	if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "s", Input: "in"}); err != nil {
		t.Fatalf("Judge: %v", err)
	}

	got := sentSamplingOf(t, captured.Body)
	if got.Temperature == nil || *got.Temperature != 0.2 || got.TopP != nil {
		t.Fatalf("sent sampling = %+v, want the configured temperature 0.2 and no top_p", got)
	}
}

func TestAJudgementResultReportsTheSamplingTheRequestCarriedWithAndWithoutAnOverride(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured loop.Sampling
		chosen     *float64
	}{
		"no override":                      {configured: loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}},
		"an override":                      {configured: loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}, chosen: floatPtr(0.7)},
		"an override of zero":              {configured: loop.Sampling{Temperature: floatPtr(0.2)}, chosen: floatPtr(0)},
		"an override over nothing":         {chosen: floatPtr(1)},
		"nothing configured or overridden": {},
	}
	for name, tc := range cases {
		srv, captured := capturingServer(t, doneResponse)
		c := NewClient(srv.URL, "model-x", "", tc.configured, srv.Client())

		result, err := c.Judge(context.Background(), loop.JudgeInput{System: "s", Input: "in", Temperature: tc.chosen})
		if err != nil {
			t.Fatalf("%s: Judge: %v", name, err)
		}
		if sent := sentSamplingOf(t, captured.Body); !reflect.DeepEqual(result.Sampling, sent) {
			t.Errorf("%s: the result reports %+v, the request carried %+v", name, result.Sampling, sent)
		}
	}
}

func TestADerivationCallSendsAndReportsTheConfiguredSamplingWhateverTheJudgementCallsChose(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, nativeCondenseOK)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}, srv.Client())

	result, err := c.Derive(context.Background(), "derive from this", 64)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	want := loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}
	if sent := sentSamplingOf(t, captured.Body); !reflect.DeepEqual(sent, want) {
		t.Errorf("the derivation request carried %+v, want the configured %+v", sent, want)
	}
	if !reflect.DeepEqual(result.Sampling, want) {
		t.Errorf("Derive reports %+v, want what the request carried, %+v", result.Sampling, want)
	}
}
