package ledger

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runarchive"
)

func archiveOfRecords(selector string, records ...loop.Record) runarchive.Archive {
	entries := make([]runarchive.Entry, 0, len(records))
	for i, record := range records {
		entries = append(entries, runarchive.Entry{Node: int64(i + 1), Record: record})
	}
	return runarchive.Archive{Selector: selector, Entries: entries}
}

func firingFor(t *testing.T, result FiringResult, name string) Firing {
	t.Helper()
	for _, firing := range result.Firings {
		if firing.Disposition == name {
			return firing
		}
	}
	t.Fatalf("no firing for %q in %+v", name, result.Firings)
	return Firing{}
}

func TestDispositionFiringCountsEveryDispositionOverTheSelectionItNames(t *testing.T) {
	t.Parallel()

	archive := archiveOfRecords("four runs",
		loop.Record{Candidates: admitted(1, 2)},
		loop.Record{Candidates: admitted(1, 2, 3, 4, 5), ToolCalls: []loop.ToolCallRecord{recall("new", admitted(9))}},
		loop.Record{Candidates: admitted(1, 2, 3, 4, 5), ToolCalls: []loop.ToolCallRecord{
			recall("old", admitted(1)), recall("old again", admitted(2)),
		}},
		loop.Record{Candidates: admitted(1, 2, 3, 4, 5, 6)},
	)

	result := DispositionFiring(archive)

	if result.Selector != "four runs" || result.Population != 4 {
		t.Fatalf("result names selector %q over population %d", result.Selector, result.Population)
	}
	if got := firingFor(t, result, "acceptedInsufficiency").FiredIn; got != 1 {
		t.Errorf("acceptedInsufficiency fired in %d records, want 1", got)
	}
	if got := firingFor(t, result, "productiveRequery").FiredIn; got != 1 {
		t.Errorf("productiveRequery fired in %d records, want 1", got)
	}
	if got := firingFor(t, result, "unproductiveRequery").FiredIn; got != 1 {
		t.Errorf("unproductiveRequery fired in %d records, want 1", got)
	}
	if result.NoDisposition != 1 {
		t.Errorf("the run with a full block and no round fired nothing, so want 1, got %d", result.NoDisposition)
	}
}

func TestADispositionNoRecordFiredIsMarkedUnfiredRatherThanReportedAsZero(t *testing.T) {
	t.Parallel()

	archive := archiveOfRecords("two runs, neither refused", loop.Record{Candidates: admitted(1)}, loop.Record{Candidates: admitted(1)})

	result := DispositionFiring(archive)

	refused := firingFor(t, result, "refusedRequery")
	if !refused.Unfired || refused.FiredIn != 0 {
		t.Fatalf("a disposition nothing fired is unfired, got %+v", refused)
	}
	if fired := firingFor(t, result, "acceptedInsufficiency"); fired.Unfired {
		t.Fatalf("a disposition two records fired is not unfired, got %+v", fired)
	}
}

func TestEveryDispositionCarriesItsOwnDefinitionSoTheCountReadsWithoutTheCode(t *testing.T) {
	t.Parallel()

	result := DispositionFiring(archiveOfRecords("one run", loop.Record{Candidates: admitted(1)}))

	for _, firing := range result.Firings {
		if strings.TrimSpace(firing.Definition) == "" {
			t.Errorf("%q carries no definition", firing.Disposition)
		}
	}
}

func TestRenderedFiringNamesThePopulationAndStatesAnUnfiredDispositionAsUnfired(t *testing.T) {
	t.Parallel()

	archive := archiveOfRecords("two runs, neither refused", loop.Record{Candidates: admitted(1)}, loop.Record{Candidates: admitted(1)})

	rendered := RenderFiring(DispositionFiring(archive))

	for _, want := range []string{"two runs, neither refused", "population 2", "UNFIRED", "acceptedInsufficiency", "no disposition"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered firing does not carry %q\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "usefully") || strings.Contains(rendered, "pointless") || strings.Contains(rendered, "wasted") {
		t.Errorf("the rendered report states what a round yielded, never what that was worth\n%s", rendered)
	}
}

func TestEveryRenderedDispositionCarriesTheDefinitionItsCountMeans(t *testing.T) {
	t.Parallel()

	archive := archiveOfRecords("one run", loop.Record{Candidates: admitted(1)})

	rendered := RenderFiring(DispositionFiring(archive))

	for _, firing := range DispositionFiring(archive).Firings {
		if !strings.Contains(rendered, firing.Definition) {
			t.Errorf("the rendered line for %q does not carry its definition, so the count reads as a verdict rather than as what was counted%s%s",
				firing.Disposition, "\n", rendered)
		}
	}
}
