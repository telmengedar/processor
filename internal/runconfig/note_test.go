package runconfig

import (
	"strings"
	"testing"
)

func TestDecodeReadsTheNoteTypeAndAConfigWithoutANoteCarriesNone(t *testing.T) {
	t.Parallel()

	got, err := Decode(`{"note": {"type": "thought"}}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Note == nil || got.Note.Type != "thought" {
		t.Fatalf("Decode carried the note %+v, want the type thought", got.Note)
	}

	for _, content := range []string{`{}`, `{"silence": true}`, `{"filter": {"type": "thought"}}`} {
		got, err := Decode(content)
		if err != nil {
			t.Fatalf("Decode(%q): %v", content, err)
		}
		if got.Note != nil {
			t.Errorf("Decode(%q) carried the note %+v, want none: a note is never on unless the config says so", content, got.Note)
		}
	}
}

func TestDecodeRefusesAMalformedNoteNamingTheKeyAndSetsNoLengthLimitOnTheType(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"a string":          `{"note": "thought"}`,
		"a boolean":         `{"note": true}`,
		"null":              `{"note": null}`,
		"an empty object":   `{"note": {}}`,
		"a blank type":      `{"note": {"type": "  "}}`,
		"an empty type":     `{"note": {"type": ""}}`,
		"a numeric type":    `{"note": {"type": 5}}`,
		"a null type":       `{"note": {"type": null}}`,
		"a padded type":     `{"note": {"type": " thought"}}`,
		"an extra member":   `{"note": {"type": "thought", "name": "x"}}`,
		"a member in place": `{"note": {"kind": "thought"}}`,
	} {
		_, err := Decode(content)
		if err == nil {
			t.Errorf("%s: Decode accepted %s", name, content)
			continue
		}
		if !strings.Contains(err.Error(), `"note"`) {
			t.Errorf("%s: the refusal reads %q, want it to name the note key", name, err)
		}
	}

	long := strings.Repeat("a", 4096)
	got, err := Decode(`{"note": {"type": "` + long + `"}}`)
	if err != nil {
		t.Fatalf("a type of %d bytes was refused: %v: processor sets no length limit of its own on the type", len(long), err)
	}
	if got.Note == nil || got.Note.Type != long {
		t.Fatalf("a type of %d bytes decoded to %+v", len(long), got.Note)
	}
}

func TestDecodeRefusesAConfigAndARunRecordAsTheNoteTypeWithOrWithoutAFilter(t *testing.T) {
	t.Parallel()

	for _, forbidden := range []string{"processor-config", "session-log", "Session-Log", "PROCESSOR-CONFIG"} {
		for name, filter := range map[string]string{
			"no filter":                   ``,
			"a filter admitting":          `, "filter": {"type": ["` + forbidden + `", "thought"]}`,
			"a filter without a type key": `, "filter": {"status": "open"}`,
		} {
			content := `{"note": {"type": "` + forbidden + `"}` + filter + `}`
			_, err := Decode(content)
			if err == nil {
				t.Errorf("%s: Decode accepted the forbidden note type in %s", name, content)
				continue
			}
			if !strings.Contains(err.Error(), `"note"`) || !strings.Contains(err.Error(), "never write") {
				t.Errorf("%s: the refusal reads %q, want it to name the note key and say the model may never write the type", name, err)
			}
		}
	}
}

func TestDecodeHoldsTheNoteTypeToTheTypesTheFilterAdmits(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		content string
		wantErr bool
	}{
		"a type among several":      {`{"filter": {"type": ["thought", "knowledge"]}, "note": {"type": "thought"}}`, false},
		"the second of several":     {`{"filter": {"type": ["knowledge", "thought"]}, "note": {"type": "thought"}}`, false},
		"a single admitted type":    {`{"filter": {"type": "thought"}, "note": {"type": "thought"}}`, false},
		"a type the filter hides":   {`{"filter": {"type": ["thought", "knowledge"]}, "note": {"type": "memo"}}`, true},
		"a different case":          {`{"filter": {"type": ["thought"]}, "note": {"type": "Thought"}}`, true},
		"a filter with no type key": {`{"filter": {"status": "open"}, "note": {"type": "memo"}}`, false},
		"no filter":                 {`{"note": {"type": "memo"}}`, false},
	} {
		_, err := Decode(tc.content)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: Decode accepted %s: a note the run's own filter hides could never be recalled", name, tc.content)
			} else if !strings.Contains(err.Error(), `"note"`) || !strings.Contains(err.Error(), "recalled") {
				t.Errorf("%s: the refusal reads %q, want it to name the note key and say the note would not be recalled", name, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: Decode refused %s: %v", name, tc.content, err)
		}
	}
}
