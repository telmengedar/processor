package runconfig

import (
	"strings"
	"testing"
)

func TestDecodeKeepsEveryFiniteNonNegativeTemperatureAsWrittenWithNoUpperBound(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]float64{"0": 0, "0.0": 0, "0.7": 0.7, "1": 1, "2.5": 2.5, "50": 50, "1e3": 1000} {
		config, err := Decode(`{"temperature": ` + text + `}`)
		if err != nil {
			t.Errorf("Decode(temperature %s): %v", text, err)
			continue
		}
		if config.Temperature == nil || *config.Temperature != want {
			t.Errorf("Decode(temperature %s).Temperature = %v, want %v", text, config.Temperature, want)
		}
	}
}

func TestDecodeKeepsAZeroTemperatureDistinctFromNone(t *testing.T) {
	t.Parallel()

	zero, err := Decode(`{"temperature": 0}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	none, err := Decode(`{}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if zero.Temperature == nil {
		t.Fatal("a temperature of 0 decoded to none: greedy decoding the client chose must not read as the deployment's value")
	}
	if none.Temperature != nil {
		t.Fatalf("a config without the key decoded to %v, want none", *none.Temperature)
	}
}

func TestDecodeRefusesATemperatureThatIsNoNonNegativeFiniteNumberNamingTheKey(t *testing.T) {
	t.Parallel()

	for _, text := range []string{`"0.7"`, `true`, `false`, `null`, `[]`, `[0.7]`, `{}`, `-0.1`, `-1`, `1e400`, `-1e400`} {
		_, err := Decode(`{"temperature": ` + text + `}`)
		if err == nil {
			t.Errorf("Decode(temperature %s) accepted it", text)
			continue
		}
		if !strings.Contains(err.Error(), `"temperature"`) {
			t.Errorf("Decode(temperature %s) = %q, want the error to name the key", text, err)
		}
	}
}

func TestDecodeReadsNoTemperatureAsNoneAndLeavesTheOtherKeysAsTheyWere(t *testing.T) {
	t.Parallel()

	config, err := Decode(`{"filter": {"type": "thought"}, "silence": true, "identity": "I am Gangolf", "note": {"type": "thought"}}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if config.Temperature != nil {
		t.Fatalf("Temperature = %v for a config without the key, want none", *config.Temperature)
	}

	with, err := Decode(`{"filter": {"type": "thought"}, "silence": true, "identity": "I am Gangolf", "note": {"type": "thought"}, "temperature": 0.7}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if with.Temperature == nil || *with.Temperature != 0.7 {
		t.Fatalf("Temperature = %v, want 0.7", with.Temperature)
	}
	if with.Filter["type"][0] != "thought" || !with.Silence || with.Identity != "I am Gangolf" || with.Note == nil || with.Note.Type != "thought" {
		t.Fatalf("the temperature changed the other keys: %+v", with)
	}
}

func TestDecodeStillRefusesAnUnknownKeyBesideATemperature(t *testing.T) {
	t.Parallel()

	_, err := Decode(`{"temperature": 0.7, "topP": 0.9}`)
	if err == nil || !strings.Contains(err.Error(), `"topP"`) {
		t.Fatalf("Decode = %v, want the unknown key refused by name", err)
	}
}

func TestDecodeSaysWhyANumberTooLargeForADoubleIsRefused(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"1e400", "1E400"} {
		_, err := Decode(`{"temperature": ` + text + `}`)
		if err == nil || !strings.Contains(err.Error(), "does not fit a double") {
			t.Errorf("Decode(temperature %s) = %v, want the refusal to say the number does not fit a double", text, err)
		}
	}
}
