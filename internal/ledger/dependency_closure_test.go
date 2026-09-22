package ledger

import (
	"os/exec"
	"strings"
	"testing"
)

var forbiddenInstrumentDependencies = []string{
	"github.com/telmengedar/processor/internal/condense",
	"github.com/telmengedar/processor/internal/fill",
	"github.com/telmengedar/processor/internal/openaicompat",
	"github.com/telmengedar/processor/internal/ollama",
}

func TestTheLedgerDependencyClosureCarriesNoModelAdapter(t *testing.T) {
	t.Parallel()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}

	out, err := exec.Command(goBin, "list", "-deps", ".", "../runarchive").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}

	deps := string(out)
	for _, forbidden := range forbiddenInstrumentDependencies {
		if strings.Contains(deps, forbidden) {
			t.Errorf("the ledger and the record reader close over %s; no mechanical instrument may reach a model adapter, because no mechanical instrument may cost a model call", forbidden)
		}
	}
}
