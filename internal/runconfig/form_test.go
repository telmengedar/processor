package runconfig

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestTheConfigsFormIsContentOrSubstanceAndAnythingElseIsRefused(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]loop.Form{`"content"`: loop.FormContent, `"substance"`: loop.FormSubstance} {
		config, err := Decode(`{"form": ` + text + `}`)
		if err != nil {
			t.Errorf("Decode(form %s): %v", text, err)
			continue
		}
		if config.Form != want {
			t.Errorf("Decode(form %s).Form = %q, want %q", text, config.Form, want)
		}
	}

	absent, err := Decode(`{}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if absent.Form != "" {
		t.Errorf("a config without the key decoded to form %q, want none", absent.Form)
	}

	for _, text := range []string{`"Content"`, `"SUBSTANCE"`, `" content"`, `"content "`, `""`, `"unavailable"`, `true`, `false`, `null`, `1`, `[]`, `["content"]`, `{}`} {
		_, err := Decode(`{"form": ` + text + `}`)
		if err == nil {
			t.Errorf("Decode(form %s) accepted it", text)
			continue
		}
		if !strings.Contains(err.Error(), `"form"`) {
			t.Errorf("Decode(form %s) = %q, want the error to name the key", text, err)
		}
	}
}

func TestAFormThatIsNoJSONStringIsRefusedAsNoStringNotAsAnUnknownValue(t *testing.T) {
	t.Parallel()

	for _, text := range []string{`true`, `false`, `1`, `[]`, `["content"]`, `{}`, `null`, ` null `} {
		_, err := Decode(`{"form": ` + text + `}`)
		if err == nil || !strings.Contains(err.Error(), "is not a JSON string") {
			t.Errorf("Decode(form %s) = %v, want the refusal to say the value is not a JSON string", text, err)
		}
	}
}
