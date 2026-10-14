package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func sentSamplingOf(t *testing.T, body []byte) loop.Sampling {
	t.Helper()
	keys := topLevelKeys(t, body)
	var sampling loop.Sampling
	for name, target := range map[string]**float64{"temperature": &sampling.Temperature, "top_p": &sampling.TopP} {
		raw, present := keys[name]
		if !present {
			continue
		}
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		*target = &value
	}
	return sampling
}

func TestAJudgementCallsOwnTemperatureReplacesTheConfiguredOneAtTheTopLevelAndLeavesTopPAlone(t *testing.T) {
	t.Parallel()

	for _, chosen := range []float64{0, 0.7, 1.5, 50} {
		srv, captured := capturingServer(t, stopResponse)
		c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.95)}, srv.Client())

		if _, err := c.Judge(context.Background(), loop.JudgeInput{System: "s", Input: "in", Temperature: floatPtr(chosen)}); err != nil {
			t.Fatalf("Judge: %v", err)
		}

		got := sentSamplingOf(t, captured.Body)
		if got.Temperature == nil || *got.Temperature != chosen {
			t.Errorf("temperature = %v, want %v", got.Temperature, chosen)
		}
		if got.TopP == nil || *got.TopP != 0.95 {
			t.Errorf("top_p = %v, want the configured 0.95 untouched", got.TopP)
		}
		if _, nested := topLevelKeys(t, captured.Body)["options"]; nested {
			t.Errorf("the request carries an options object: this protocol spells sampling at the top level; body=%s", captured.Body)
		}
	}
}

func TestAJudgementCallWithoutATemperatureSendsTheConfiguredOne(t *testing.T) {
	t.Parallel()

	srv, captured := capturingServer(t, stopResponse)
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
		srv, captured := capturingServer(t, stopResponse)
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

	srv, captured := capturingServer(t, condenseOK)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}, srv.Client())

	result, err := c.Derive(context.Background(), "derive from this", 64)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	want := loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}
	if sent := sentSamplingOf(t, captured.Body); !reflect.DeepEqual(sent, want) {
		t.Errorf("the derivation request carried %+v, want the configured %+v", sent, want)
	}
	if result.Sampling == nil || !reflect.DeepEqual(*result.Sampling, want) {
		t.Errorf("Derive reports %+v, want what the request carried, %+v", result.Sampling, want)
	}
}

func TestADerivationCallThatFailedAfterItsRequestWasSentStillReportsTheSamplingItCarried(t *testing.T) {
	t.Parallel()

	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(refusing.Close)

	gone := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	goneURL, goneClient := gone.URL, gone.Client()
	gone.Close()

	configured := loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}
	cases := map[string]*Client{
		"a non-success status":    NewClient(refusing.URL, "model-x", "", configured, refusing.Client()),
		"a connection that fails": NewClient(goneURL, "model-x", "", configured, goneClient),
	}
	for name, c := range cases {
		result, err := c.Derive(context.Background(), "derive from this", 64)
		if err == nil {
			t.Fatalf("%s: Derive returned no error", name)
		}
		if result.Sampling == nil || !reflect.DeepEqual(*result.Sampling, configured) {
			t.Errorf("%s: Derive reports %+v beside the error, want the %+v its request carried", name, result.Sampling, configured)
		}
		if result.Text != "" {
			t.Errorf("%s: Derive returned the text %q beside an error", name, result.Text)
		}
	}
}

func TestADerivationCallWhoseRequestNeverLeftReportsNoSampling(t *testing.T) {
	t.Parallel()

	c := NewClient("http://[::1", "model-x", "", loop.Sampling{Temperature: floatPtr(0.2)}, nil)

	result, err := c.Derive(context.Background(), "derive from this", 64)
	if err == nil {
		t.Fatal("Derive returned no error for a base URL no request can be built against")
	}
	if result.Sampling != nil {
		t.Fatalf("Derive reports %+v for a request that was never sent, want none", result.Sampling)
	}
}

func TestADerivationCallWhoseResponseCannotBeReadStillReportsTheSamplingItCarried(t *testing.T) {
	t.Parallel()

	configured := loop.Sampling{Temperature: floatPtr(0.2), TopP: floatPtr(0.9)}
	for name, body := range map[string]string{
		"a 200 whose body is not JSON": "this is not json",
		"a 200 with no choices":        `{"model":"m","choices":[]}`,
	} {
		srv, _ := capturingServer(t, body)
		result, err := NewClient(srv.URL, "model-x", "", configured, srv.Client()).Derive(context.Background(), "derive from this", 64)
		if err == nil {
			t.Fatalf("%s: Derive returned no error", name)
		}
		if result.Sampling == nil || !reflect.DeepEqual(*result.Sampling, configured) {
			t.Errorf("%s: Derive reports %+v beside the error, want the %+v its request carried", name, result.Sampling, configured)
		}
	}
}
