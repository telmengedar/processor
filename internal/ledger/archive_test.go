package ledger

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/runarchive"
)

const archiveDirEnv = "PROCESSOR_RUN_ARCHIVE_DIR"

const accountFirstLinePrefix = "processor-run at "

func dumpedNode(file string, content string) runarchive.Node {
	id, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(file, "node"), ".raw"), 10, 64)

	name := ""
	first := content
	if at := strings.IndexByte(content, '\n'); at != -1 {
		first = content[:at]
	}
	if strings.HasPrefix(first, accountFirstLinePrefix) {
		name = "processor-run " + strings.TrimPrefix(first, accountFirstLinePrefix) + " — a dumped run"
	}

	return runarchive.Node{ID: id, Name: name, Content: content}
}

func archiveFromDir(t *testing.T, dir string) runarchive.Archive {
	t.Helper()

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	nodes := make([]runarchive.Node, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".raw") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", file.Name(), err)
		}
		nodes = append(nodes, dumpedNode(file.Name(), string(content)))
	}
	return runarchive.Read("every dumped run record in "+dir, nodes)
}

func TestTheDumpedArchiveDecodesOrNamesTheRuleThatStoppedEveryRecord(t *testing.T) {
	dir := os.Getenv(archiveDirEnv)
	if dir == "" {
		t.Skipf("set %s to a directory of dumped run-record node contents to read the archive", archiveDirEnv)
	}

	archive := archiveFromDir(t, dir)

	t.Logf("decoded %d records, skipped %d", len(archive.Entries), len(archive.Skipped))
	for _, skip := range archive.Skipped {
		if skip.Reason == "" {
			t.Errorf("a record was dropped with no named reason: %+v", skip)
		}
		t.Logf("SKIP #%d: %s %s", skip.Node, skip.Reason, skip.Detail)
	}
	if len(archive.Entries) == 0 {
		t.Fatalf("no record decoded out of %s", dir)
	}

	undated := 0
	for _, entry := range archive.Entries {
		if !entry.Dated {
			undated++
		}
	}
	t.Logf("%d of %d dumped records carry no node name, so the dump states no run instant for them", undated, len(archive.Entries))

	for _, entry := range Over(archive) {
		t.Logf("#%-6d %s verdict=%-10s admitted=%2d dispatched=%d productive=%d barren=%d refused=%d accepted=%-5t unproductive=%-5t productiveRequery=%-5t refusedRequery=%t",
			entry.Node, renderInstant(entry.At, entry.Dated), entry.Outcome.Verdict,
			entry.Requery.AdmittedRows, entry.Requery.DispatchedRounds, entry.Requery.ProductiveRounds,
			entry.Requery.BarrenRounds, entry.Requery.RefusedRounds,
			entry.Requery.AcceptedInsufficiency, entry.Requery.UnproductiveRequery,
			entry.Requery.ProductiveRequery, entry.Requery.RefusedRequery)
	}

	t.Logf("\n%s", RenderFiring(DispositionFiring(archive)))
	t.Logf("\n%s", RenderProperties(DialExercise(archive), MechanismObservation(archive)))
}

func renderInstant(at time.Time, dated bool) string {
	if !dated {
		return "instant unknown    "
	}
	return at.Format("2006-01-02T15:04:05Z")
}
