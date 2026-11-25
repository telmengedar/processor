package runconfig

import (
	"github.com/telmengedar/processor/internal/loop"
	"testing"
)

func TestNoteDecisionModeRequiresExplicitKnownModeAndCompatibleConfig(t *testing.T) {
	t.Parallel()
	cfg, err := Decode(`{"note":{"type":"thought","mode":"decision"}}`)
	if err != nil || cfg.Note.Mode != loop.NoteModeDecision {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
	cfg, err = Decode(`{"note":{"type":"thought"}}`)
	if err != nil || cfg.Note.Mode != "" {
		t.Fatalf("native default changed: %+v %v", cfg, err)
	}
	for _, mode := range []string{`null`, `true`, `1`, `{}`, `[]`, `""`, `"Decision"`, `"decision "`, `"native"`} {
		if _, err := Decode(`{"note":{"type":"thought","mode":` + mode + `}}`); err == nil {
			t.Errorf("accepted mode %s", mode)
		}
	}
	if _, err := Decode(`{"responseMode":"structuredRead","note":{"type":"thought","mode":"decision"}}`); err == nil {
		t.Fatal("incompatible contracts accepted")
	}
}
