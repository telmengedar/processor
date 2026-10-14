package runconfig

import (
	"reflect"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestDecodeReadsGangolfsConfigIntoRepeatedValuesInOrder(t *testing.T) {
	t.Parallel()

	got, err := Decode(`{"filter": {"type": ["thought", "knowledge"]}}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if want := (loop.Filter{"type": {"thought", "knowledge"}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode = %v, want %v", got, want)
	}
}

func TestDecodeEncodesScalarsAsOneValueEachAndKeepsNumbersAndBooleansAsTheirJSONText(t *testing.T) {
	t.Parallel()

	got, err := Decode(`{"filter": {"status": "open", "severity": [3, 4.50], "nostatus": true, "minSimilarity": 0.40, "linkedto": 10422}}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	want := loop.Filter{
		"status":        {"open"},
		"severity":      {"3", "4.50"},
		"nostatus":      {"true"},
		"minSimilarity": {"0.40"},
		"linkedto":      {"10422"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode = %v, want %v: a number is forwarded as its own text, never re-rendered", got, want)
	}
}

func TestDecodeOfAConfigWithoutAFilterIsValidAndCarriesNone(t *testing.T) {
	t.Parallel()

	for _, content := range []string{`{}`, " {}\n"} {
		got, err := Decode(content)
		if err != nil {
			t.Errorf("Decode(%q): %v", content, err)
		}
		if got != nil {
			t.Errorf("Decode(%q) = %v, want no filter", content, got)
		}
	}
}

func TestDecodeRefusesEveryOwnedKeyInAnyCase(t *testing.T) {
	t.Parallel()

	for _, owned := range []string{"query", "queries", "count", "continue", "sort", "descending", "fields", "id"} {
		for _, spelled := range []string{owned, strings.ToUpper(owned), strings.ToUpper(owned[:1]) + owned[1:]} {
			_, err := Decode(`{"filter": {"` + spelled + `": [1]}}`)
			if err == nil {
				t.Errorf("Decode accepted the owned key %q", spelled)
				continue
			}
			if !strings.Contains(err.Error(), `"`+spelled+`"`) || !strings.Contains(err.Error(), "sets itself") {
				t.Errorf("the refusal of %q reads %q, want it to name the key and say processor sets it itself", spelled, err)
			}
		}
	}
}

func TestDecodeRefusesAnyNameOffTheListNamingIt(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"tpye", "Type", "TYPE", "types", "rootNodeID", "updatedfrom", "excludeid", "fieldz", ""} {
		_, err := Decode(`{"filter": {"` + name + `": ["thought"]}}`)
		if err == nil {
			t.Errorf("Decode accepted the name %q", name)
			continue
		}
		if !strings.Contains(err.Error(), `"`+name+`"`) {
			t.Errorf("the refusal of %q reads %q, want it to name the key", name, err)
		}
	}
}

func TestDecodeAcceptsEveryListedNameWithAScalarValue(t *testing.T) {
	t.Parallel()

	for _, name := range forwardedNames {
		got, err := Decode(`{"filter": {"` + name + `": "v"}}`)
		if err != nil {
			t.Errorf("Decode refused the listed name %q: %v", name, err)
			continue
		}
		if !reflect.DeepEqual(got, loop.Filter{name: {"v"}}) {
			t.Errorf("Decode kept %v for the name %q, want it forwarded verbatim", got, name)
		}
	}
}

func TestDecodeRefusesAValueThatWouldForwardNoFilterOrAShapeItCannotEncode(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an empty array":                   `{"filter": {"type": []}}`,
		"an object value":                  `{"filter": {"type": {"a": 1}}}`,
		"a null value":                     `{"filter": {"type": null}}`,
		"an object in an array":            `{"filter": {"type": [{"a": 1}]}}`,
		"a null in an array":               `{"filter": {"type": ["thought", null]}}`,
		"a nested array":                   `{"filter": {"type": [["thought"]]}}`,
		"a blank scalar":                   `{"filter": {"type": ""}}`,
		"a blank array element":            `{"filter": {"type": [""]}}`,
		"a blank element after a real one": `{"filter": {"type": ["thought", ""]}}`,
		"a whitespace-only scalar":         `{"filter": {"type": "   "}}`,
		"a whitespace-only element":        `{"filter": {"type": [" \t\n "]}}`,
	}
	for name, content := range cases {
		if _, err := Decode(content); err == nil {
			t.Errorf("Decode accepted %s", name)
		} else if !strings.Contains(err.Error(), `"type"`) {
			t.Errorf("the refusal of %s reads %q, want it to name the key", name, err)
		}
	}
}

func TestDecodeRefusesAnEmptyOrNonObjectFilterAndAnUnknownTopLevelKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an empty filter":              `{"filter": {}}`,
		"a null filter":                `{"filter": null}`,
		"an array filter":              `{"filter": ["type"]}`,
		"a string filter":              `{"filter": "type"}`,
		"an unknown top-level key":     `{"filtr": {"type": ["thought"]}}`,
		"an extra top-level key":       `{"filter": {"type": ["thought"]}, "identity": "x"}`,
		"the filter key in wrong case": `{"Filter": {"type": ["thought"]}}`,
	}
	for name, content := range cases {
		if _, err := Decode(content); err == nil {
			t.Errorf("Decode accepted %s", name)
		}
	}
}

func TestDecodeRefusesContentThatIsNotOneJSONObject(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", "not json", "null", "[]", `"x"`, `{"filter": {"type": ["a"]}} trailing`, `{"filter": {"type": ["a"]}}{}`, `{"filter": `} {
		if _, err := Decode(content); err == nil {
			t.Errorf("Decode accepted %q", content)
		}
	}
}

func TestTheConfigIsReadUnderAFilterOnItsOwnNodeType(t *testing.T) {
	t.Parallel()

	if got, want := ReadFilter(), (loop.Filter{"type": {"processor-config"}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadFilter = %v, want %v", got, want)
	}
}

func TestDecodeNamesTheFirstViolationInKeyOrderWhateverOrderTheConfigWroteThem(t *testing.T) {
	t.Parallel()

	for i := 0; i < 20; i++ {
		_, err := Decode(`{"filter": {"zzz": 1, "mmm": 1, "aaa": 1}}`)
		if err == nil || !strings.Contains(err.Error(), `"aaa"`) {
			t.Fatalf("Decode err = %v, want the violation of the first key in sorted order, \"aaa\"", err)
		}
	}
}

func TestDecodeKeepsABooleanFalseAndAValueWithInnerOrOuterSpacesBecauseTheAuthorChoseThem(t *testing.T) {
	t.Parallel()

	got, err := Decode(`{"filter": {"nostatus": false, "name": " Mixed Case% "}}`)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if want := (loop.Filter{"nostatus": {"false"}, "name": {" Mixed Case% "}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode = %v, want %v", got, want)
	}
}

func TestDecodeRefusesABlankValueOnAnyKeyNamingIt(t *testing.T) {
	t.Parallel()

	for _, name := range forwardedNames {
		_, err := Decode(`{"filter": {"` + name + `": ""}}`)
		if err == nil || !strings.Contains(err.Error(), `"`+name+`"`) {
			t.Errorf("a blank value on %q gave err = %v, want a refusal naming the key", name, err)
		}
	}
}
