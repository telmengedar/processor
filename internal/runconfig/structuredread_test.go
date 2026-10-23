package runconfig

import (
	"github.com/telmengedar/processor/internal/loop"
	"testing"
)

func TestOnlyTheExplicitStructuredReadModeEnablesTheResponseContract(t *testing.T) {
	t.Parallel()
	config, err := Decode(`{"responseMode":"structuredRead","filter":{"type":"knowledge"},"identity":"verbatim policy"}`)
	if err != nil || config.ResponseMode != loop.ResponseStructuredRead || config.Identity != "verbatim policy" {
		t.Fatalf("config=%+v error=%v", config, err)
	}
	native, err := Decode(`{}`)
	if err != nil || native.ResponseMode != "" {
		t.Fatalf("omitted mode changed native default: %+v %v", native, err)
	}
	for _, raw := range []string{`null`, `false`, `7`, `{}`, `[]`, `""`, `"StructuredRead"`, `"structuredRead "`, `"native"`} {
		if _, err := Decode(`{"responseMode":` + raw + `}`); err == nil {
			t.Errorf("accepted malformed/unknown mode %s", raw)
		}
	}
	if _, err := Decode(`{"responseMode":"structuredRead","note":{"type":"thought"}}`); err == nil {
		t.Fatal("structured reads silently accepted a note capability")
	}
}
