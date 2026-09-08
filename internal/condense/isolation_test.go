package condense

import (
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const loopPackageDir = "../loop"

func TestTheLoopPackageImportsNothingFromTheOfflineCondensationPass(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), loopPackageDir, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing the loop package failed: %v", err)
	}
	if len(packages) == 0 {
		t.Fatalf("parsed no packages at %s, so this guard would pass vacuously", loopPackageDir)
	}

	for name, pkg := range packages {
		for path, file := range pkg.Files {
			for _, imported := range file.Imports {
				if strings.Contains(imported.Path.Value, "/condense") {
					t.Fatalf("%s in package %s imports %s, which would put generation inside a turn", path, name, imported.Path.Value)
				}
			}
		}
	}
}

func TestTheModelSeamTheTurnHoldsExposesJudgementAloneAndNoWayToGenerate(t *testing.T) {
	port := reflect.TypeOf((*loop.ModelPort)(nil)).Elem()

	if port.NumMethod() != 1 {
		t.Fatalf("want exactly one method on the turn's model seam, got %d", port.NumMethod())
	}
	if got := port.Method(0).Name; got != "Judge" {
		t.Fatalf("want the turn's only model method to be Judge, got %s", got)
	}
}

func TestTheGraphSeamTheTurnHoldsOffersTheseFourOperationsAndNoOtherTheTurnCouldAim(t *testing.T) {
	port := reflect.TypeOf((*loop.GraphPort)(nil)).Elem()

	got := make([]string, port.NumMethod())
	for i := range got {
		got[i] = port.Method(i).Name
	}

	want := []string{"Neighbours", "Node", "Recall", "WriteRun"}
	if !slices.Equal(got, want) {
		t.Fatalf("the turn's graph seam offers %v, want exactly %v; each of the four is scoped to the run in hand, and a fifth would be an operation the turn could aim at a node of its own choosing", got, want)
	}
}
