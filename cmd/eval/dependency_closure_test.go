package main

import (
	"os/exec"
	"strings"
	"testing"
)

var forbiddenFillDependencies = []string{
	"github.com/telmengedar/processor/internal/condense",
	"github.com/telmengedar/processor/internal/fill",
	"github.com/telmengedar/processor/internal/openaicompat",
	"github.com/telmengedar/processor/internal/ollama",
}

func TestCmdEvalDependencyClosureCarriesNoModelAdapter(t *testing.T) {
	t.Parallel()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}

	out, err := exec.Command(goBin, "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps .: %v\n%s", err, out)
	}

	deps := string(out)
	for _, forbidden := range forbiddenFillDependencies {
		if strings.Contains(deps, forbidden) {
			t.Errorf("cmd/eval's dependency closure contains %s — cmd/eval must never be able to construct a model adapter or a fill port (design §7.4.4)", forbidden)
		}
	}
}
