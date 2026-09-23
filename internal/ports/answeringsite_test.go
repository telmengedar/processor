package ports

import (
	"strconv"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestBootReportsTheAnsweringCallAsACallSiteOfItsOwn(t *testing.T) {
	t.Parallel()

	lines := report(t, declared(10, 3_000), false)

	line := lineFor(t, lines, loop.SiteAnswering)
	if strings.Contains(line, "level=WARN") {
		t.Fatalf("the answering site is reported unaffordable at the product's own declared floors, so the budget it declares is one the product cannot pay for:\n%s", line)
	}
	if !strings.Contains(line, "budget="+strconv.Itoa(loop.AnsweringBudget)) {
		t.Fatalf("the answering site's line does not name the budget it declares; a site reported without its budget cannot be checked against the bound that cuts it:\n%s", line)
	}
	if judgement := lineFor(t, lines, loop.SiteJudgement); judgement == line {
		t.Fatal("the answering call is reported on the judgement site's own line, so the reserved call has no budget of its own to report")
	}
}

func TestTheAnsweringSiteIsReportedUnaffordableWhereTheDeclaredFloorsCannotPayForIt(t *testing.T) {
	t.Parallel()

	line := lineFor(t, report(t, declared(1, 3_000), false), loop.SiteAnswering)

	if !strings.Contains(line, "level=WARN") {
		t.Fatalf("a deployment declaring a tenth of the product's generation floor is told nothing about the answering site, so the one budget this unit adds can be parked below its bound unreported:\n%s", line)
	}
	if !strings.Contains(line, "requiredTokensPerSecond=") {
		t.Fatalf("the answering site's shortfall does not name the rate the host would have to deliver:\n%s", line)
	}
}
