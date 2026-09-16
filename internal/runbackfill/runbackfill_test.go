package runbackfill

import (
	"context"
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
}

func (g *fakeGraph) NodeWithSubstance(_ context.Context, id int64) (Node, bool, error) {
	n, ok := g.nodes[id]
	return n, ok, nil
}

func (g *fakeGraph) Content(_ context.Context, id int64) (string, bool, error) {
	n, ok := g.nodes[id]
	return n.Content, ok, nil
}

func (g *fakeGraph) SetRunContent(_ context.Context, id int64, content []byte) error {
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
	if g.setSubstanceErr != nil {
		return g.setSubstanceErr
	}
	if g.substanceWrites == nil {
		g.substanceWrites = map[int64]string{}
	}
	g.substanceWrites[id] = substance
	return nil
}

const legacyRecordJSON = `{"input":"do the thing","subject":10422,"query":"","queries":["do the thing"],"anchor":{"id":10422,"type":"project","name":"processor","size":100},"candidates":[],"block":"","answer":"done","model":"m1","provider":{"adapter":"a","endpoint":"e"},"toolCalls":[],"modelCalls":1,"capReached":false,"usage":null,"stopReason":{"reason":"answered","raw":"stop"},"limits":{"candidateLimit":20,"assemblyByteBudget":60000,"supplementaryByteBudget":20000,"maxModelCalls":6,"maxOutputTokens":4096},"sampling":{}}`

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
