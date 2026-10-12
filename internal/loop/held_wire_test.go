package loop

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAHeldDispositionSerialisesTheKeyHeldAsTrueAndAnUnheldOneCarriesNoHeldKeyAtAll(t *testing.T) {
	t.Parallel()

	held, err := json.Marshal(Disposition{ID: 11, Included: true, Held: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(held), `"held":true`) {
		t.Errorf("a held disposition serialised as %s, want it to carry the literal key and value %q", held, `"held":true`)
	}

	unheld, err := json.Marshal(Disposition{ID: 52, Included: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(unheld), "held") {
		t.Errorf("an unheld disposition serialised as %s, want no held key at all: a record that holds nothing keeps the shape it had before the field existed", unheld)
	}
}

func TestADispositionDecodesTheHeldKeyFromTheWireAndReadsAnAbsentOneAsNotHeld(t *testing.T) {
	t.Parallel()

	var held, legacy Disposition
	if err := json.Unmarshal([]byte(`{"id":11,"included":true,"held":true}`), &held); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"id":11,"included":true}`), &legacy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !held.Held {
		t.Error("the wire key held:true decoded as not held")
	}
	if legacy.Held {
		t.Error("a record written before the field existed decoded as held")
	}
}
