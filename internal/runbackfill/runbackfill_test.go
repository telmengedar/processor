package runbackfill

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/divoid"
)

type fakeGraph struct {
	nodes           map[int64]Node
	contentWrites   map[int64][]byte
	substanceWrites map[int64]string
	setContentErr   error
	setSubstanceErr error
	calls           []string
}

func (g *fakeGraph) NodeWithSubstance(_ context.Context, id int64) (Node, bool, error) {
	g.calls = append(g.calls, "NodeWithSubstance")
	n, ok := g.nodes[id]
	return n, ok, nil
}

func (g *fakeGraph) Content(_ context.Context, id int64) (string, bool, error) {
	g.calls = append(g.calls, "Content")
	n, ok := g.nodes[id]
	return n.Content, ok, nil
}

func (g *fakeGraph) SetRunContent(_ context.Context, id int64, content []byte) error {
	g.calls = append(g.calls, "SetRunContent")
	if g.setContentErr != nil {
		return g.setContentErr
	}
	if g.contentWrites == nil {
		g.contentWrites = map[int64][]byte{}
	}
	g.contentWrites[id] = content
	n := g.nodes[id]
	n.Content = string(content)
	g.nodes[id] = n
	return nil
}

func (g *fakeGraph) SetSubstance(_ context.Context, id int64, substance string) error {
	g.calls = append(g.calls, "SetSubstance")
	if g.setSubstanceErr != nil {
		return g.setSubstanceErr
	}
	if g.substanceWrites == nil {
		g.substanceWrites = map[int64]string{}
	}
	g.substanceWrites[id] = substance
	return nil
}

const legacyRecordJSON = `{"input":"do the thing","subject":10422,"query":"","queries":["do the thing"],"anchor":{"id":10422,"type":"project","name":"processor","size":100},"candidates":[{"rank":1,"id":11,"type":"task","name":"Pitch-Site hosting","similarity":0.689,"size":1111,"contentHash":"c11","included":true},{"rank":2,"id":12,"type":"documentation","name":"Profilgenerator wireframe","similarity":0.659,"size":43300,"contentHash":"c12","included":true},{"rank":3,"id":15,"type":"documentation","name":"Something large","similarity":0.630,"size":20000,"contentHash":"c15","included":false,"cutReason":"byte budget exceeded"}],"block":"","answer":"done","model":"m1","provider":{"adapter":"a","endpoint":"e"},"toolCalls":[],"modelCalls":1,"capReached":false,"usage":null,"stopReason":{"reason":"answered","raw":"stop"},"limits":{"candidateLimit":20,"assemblyByteBudget":60000,"supplementaryByteBudget":20000,"maxModelCalls":6,"maxOutputTokens":4096},"sampling":{}}`

func TestBackfillOneLegacyRecordIsComposedByteIdentically(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		1: {
			ID:          1,
			Type:        divoid.RunNodeType,
			Name:        `processor-run 2026-09-02T11:35:08Z — do the thing`,
			ContentType: "application/json",
			Content:     legacyRecordJSON,
		},
	}}

	result := Run(context.Background(), graph, []int64{1}, Options{}, time.Now)

	if len(result.Skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", result.Skipped)
	}
	if len(result.Provenance) != 1 {
		t.Fatalf("want 1 provenance entry, got %d", len(result.Provenance))
	}

	p := result.Provenance[0]
	if !p.ContentWritten || !p.SubstanceWritten {
		t.Fatalf("want both writes to happen, got %+v", p)
	}
	if p.At.Format(time.RFC3339) != "2026-09-02T11:35:08Z" {
		t.Fatalf("at = %s, want the timestamp the name carries", p.At.Format(time.RFC3339))
	}

	wantContent := string(divoid.ComposeRunContent(p.Account, []byte(legacyRecordJSON)))
	gotContent := string(graph.contentWrites[1])
	if gotContent != wantContent {
		t.Fatalf("written content does not equal ComposeRunContent(account, original bytes)\ngot:  %q\nwant: %q", gotContent, wantContent)
	}

	if !strings.Contains(gotContent, legacyRecordJSON) {
		t.Fatalf("the original record bytes must appear verbatim (never re-marshalled) inside the composed content")
	}

	if graph.substanceWrites[1] != p.Account {
		t.Fatalf("substance write = %q, want the same account written to content", graph.substanceWrites[1])
	}
}

func TestBackfillOneAlreadyBackfilledIsSkippedUnlessForced(t *testing.T) {
	account := "processor-run at 2026-09-02T11:35:08Z\nsomething\n"
	composed := divoid.ComposeRunContent(account, []byte(legacyRecordJSON))

	graph := &fakeGraph{nodes: map[int64]Node{
		2: {
			ID:      2,
			Type:    divoid.RunNodeType,
			Name:    `processor-run 2026-09-02T11:35:08Z — do the thing`,
			Content: string(composed),
		},
	}}

	result := Run(context.Background(), graph, []int64{2}, Options{}, time.Now)
	if len(result.Provenance) != 0 {
		t.Fatalf("want no writes without -force, got %+v", result.Provenance)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != skipAlreadyBackfilled {
		t.Fatalf("want a single %q skip, got %+v", skipAlreadyBackfilled, result.Skipped)
	}

	forced := Run(context.Background(), graph, []int64{2}, Options{Force: true}, time.Now)
	if len(forced.Skipped) != 0 {
		t.Fatalf("want -force to proceed, got skips %+v", forced.Skipped)
	}
	if len(forced.Provenance) != 1 || forced.Provenance[0].RecordSize != len(legacyRecordJSON) {
		t.Fatalf("forced recompose should extract the same record bytes from the existing fence, got %+v", forced.Provenance)
	}
}

func TestBackfillOneDryRunWritesNothing(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		3: {
			ID:      3,
			Type:    divoid.RunNodeType,
			Name:    `processor-run 2026-09-02T11:35:08Z — do the thing`,
			Content: legacyRecordJSON,
		},
	}}

	result := Run(context.Background(), graph, []int64{3}, Options{DryRun: true}, time.Now)
	if len(result.Provenance) != 1 {
		t.Fatalf("want 1 provenance entry from a dry run, got %d", len(result.Provenance))
	}
	if result.Provenance[0].ContentWritten || result.Provenance[0].SubstanceWritten {
		t.Fatalf("dry run must not write: %+v", result.Provenance[0])
	}
	if len(graph.contentWrites) != 0 || len(graph.substanceWrites) != 0 {
		t.Fatalf("dry run touched the graph: content=%v substance=%v", graph.contentWrites, graph.substanceWrites)
	}
}

func TestBackfillOneSkipsNonRunRecords(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		4: {ID: 4, Type: "documentation", Name: "some doc", Content: "{}"},
	}}
	result := Run(context.Background(), graph, []int64{4}, Options{}, time.Now)
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != skipNotRunRecord {
		t.Fatalf("want %q skip, got %+v", skipNotRunRecord, result.Skipped)
	}
}

func TestBackfillOneRefusesToWriteWhenALiveRecheckFindsContentChangedSinceItWasRead(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		5: {
			ID:      5,
			Type:    divoid.RunNodeType,
			Name:    `processor-run 2026-09-02T11:35:08Z — do the thing`,
			Content: legacyRecordJSON,
		},
	}}
	orig := graph.nodes[5]
	changedContent := strings.Replace(legacyRecordJSON, "do the thing", "something else", 1)

	movedGraph := &movedContentGraph{fakeGraph: graph, snapshot: orig, live: changedContent}
	result := Run(context.Background(), movedGraph, []int64{5}, Options{}, time.Now)
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != skipContentMoved {
		t.Fatalf("want %q skip, got %+v", skipContentMoved, result.Skipped)
	}
}

type movedContentGraph struct {
	*fakeGraph
	snapshot Node
	live     string
}

func (g *movedContentGraph) NodeWithSubstance(_ context.Context, id int64) (Node, bool, error) {
	return g.snapshot, true, nil
}

func (g *movedContentGraph) Content(_ context.Context, id int64) (string, bool, error) {
	return g.live, true, nil
}

func TestBackfillOneCallsContentThenBackupThenSetRunContentInThatOrder(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		6: {
			ID:          6,
			Type:        divoid.RunNodeType,
			Name:        `processor-run 2026-09-02T11:35:08Z — do the thing`,
			ContentType: "application/json",
			Content:     legacyRecordJSON,
		},
	}}

	var backedUp Node
	opts := Options{Backup: func(_ context.Context, node Node) error {
		graph.calls = append(graph.calls, "Backup")
		backedUp = node
		return nil
	}}

	result := Run(context.Background(), graph, []int64{6}, opts, time.Now)

	if len(result.Skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", result.Skipped)
	}

	want := []string{"NodeWithSubstance", "Content", "Backup", "SetRunContent", "SetSubstance"}
	if len(graph.calls) != len(want) {
		t.Fatalf("call order = %v, want %v", graph.calls, want)
	}
	for i := range want {
		if graph.calls[i] != want[i] {
			t.Fatalf("call order = %v, want %v", graph.calls, want)
		}
	}

	if backedUp.Content != legacyRecordJSON {
		t.Fatalf("backed-up content = %q, want the original bytes about to be overwritten", backedUp.Content)
	}
	if backedUp.ContentType != "application/json" {
		t.Fatalf("backed-up content type = %q, want the node's own", backedUp.ContentType)
	}
}

func TestBackfillOneRefusesToWriteWhenTheBackupFails(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		7: {
			ID:      7,
			Type:    divoid.RunNodeType,
			Name:    `processor-run 2026-09-02T11:35:08Z — do the thing`,
			Content: legacyRecordJSON,
		},
	}}

	opts := Options{Backup: func(context.Context, Node) error {
		return fmt.Errorf("disk full")
	}}
	result := Run(context.Background(), graph, []int64{7}, opts, time.Now)

	if len(result.Skipped) != 1 || result.Skipped[0].Reason != skipBackupFailed {
		t.Fatalf("want a single %q skip, got %+v", skipBackupFailed, result.Skipped)
	}
	if len(graph.contentWrites) != 0 || len(graph.substanceWrites) != 0 {
		t.Fatalf("a failed backup must produce no write, got content=%v substance=%v", graph.contentWrites, graph.substanceWrites)
	}
}

func TestBackfillOneLegacyRecordAccountsTheBytesItsCandidatesCharged(t *testing.T) {
	graph := &fakeGraph{nodes: map[int64]Node{
		8: {
			ID:          8,
			Type:        divoid.RunNodeType,
			Name:        `processor-run 2026-09-02T11:35:08Z — do the thing`,
			ContentType: "application/json",
			Content:     legacyRecordJSON,
		},
	}}

	result := Run(context.Background(), graph, []int64{8}, Options{}, time.Now)

	if len(result.Skipped) != 0 || len(result.Provenance) != 1 {
		t.Fatalf("want one clean backfill, got skipped=%+v provenance=%+v", result.Skipped, result.Provenance)
	}

	account := graph.substanceWrites[8]
	if account == "" {
		t.Fatalf("no substance was written, so there is no account to read")
	}

	for _, want := range []string{
		"admitted (2, 44.4 kB of 59900 B remaining):",
		"byte budget exceeded (1, 20.0 kB):",
	} {
		if !strings.Contains(account, want) {
			t.Fatalf("the account written to the graph is missing %q - a record written before the form rule existed carries no rendered size, and the account must report the bytes it charged rather than zero:\n%s", want, account)
		}
	}

	for _, row := range []struct {
		id    string
		bytes string
	}{
		{id: "#11", bytes: "1111 B"},
		{id: "#12", bytes: "43.3 kB"},
	} {
		line := accountLineFor(t, account, row.id)
		if !strings.Contains(line, row.bytes) {
			t.Fatalf("the %s row reads %q, want the %s its size records - a legacy candidate has no rendered size and must be charged its content's own length", row.id, line, row.bytes)
		}
	}
}

func accountLineFor(t *testing.T, account, id string) string {
	t.Helper()

	for _, line := range strings.Split(account, "\n") {
		if strings.Contains(line, id+" ") {
			return line
		}
	}
	t.Fatalf("the account carries no row for %s:\n%s", id, account)
	return ""
}
