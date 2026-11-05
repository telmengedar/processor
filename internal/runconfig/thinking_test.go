package runconfig

import (
	"strings"
	"testing"
)

func TestThinkingConfigRequiresBoolean(t *testing.T) {
	for body, want := range map[string]bool{`{}`: false, `{"thinking":false}`: false, `{"thinking":true}`: true} {
		config, err := Decode(body)
		if err != nil || config.Thinking != want {
			t.Errorf("Decode(%s) = %+v, %v", body, config, err)
		}
	}
	for _, value := range []string{`null`, `"true"`, `0`, `1`, `[]`, `{}`} {
		if _, err := Decode(`{"thinking":` + value + `}`); err == nil || !strings.Contains(err.Error(), `"thinking"`) {
			t.Errorf("thinking %s: %v", value, err)
		}
	}
}
