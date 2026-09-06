package condense

import (
	"go/parser"
	"go/token"
	"reflect"
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

func TestTheGraphSeamTheTurnHoldsExposesNoSubstanceWrite(t *testing.T) {
	port := reflect.TypeOf((*loop.GraphPort)(nil)).Elem()

	for i := 0; i < port.NumMethod(); i++ {
		if strings.Contains(port.Method(i).Name, "Substance") {
			t.Fatalf("the turn's graph seam must expose no substance operation, found %s", port.Method(i).Name)
		}
	}
}
