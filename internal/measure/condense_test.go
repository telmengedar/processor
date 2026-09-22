package measure

import (
	"context"
	"reflect"
	"testing"

	"github.com/telmengedar/processor/internal/condense"
)

const (
	eventNodeWithSubstance = "nodeWithSubstance"
	eventContent           = "content"
	eventSetSubstance      = "setSubstance"
)

const (
	filledNodeID   = 707
	filledBody     = "the body the fill was asked to condense, long enough to be worth condensing at all"
	filledSubstuff = "the substance the fill generated"
	testSubstance  = "the substance a fired fill generated and this run withheld"
)

type recordingCondenseGraph struct {
	events  []string
	written map[int64]string

	node  condense.Node
	found bool
}

func newRecordingCondenseGraph() *recordingCondenseGraph {
	return &recordingCondenseGraph{
		written: map[int64]string{},
		node:    condense.Node{ID: filledNodeID, Type: "documentation", Name: "a candidate", ContentType: "text/markdown", Content: filledBody},
		found:   true,
	}
}

func (g *recordingCondenseGraph) NodeWithSubstance(context.Context, int64) (condense.Node, bool, error) {
	g.events = append(g.events, eventNodeWithSubstance)
	return g.node, g.found, nil
}

func (g *recordingCondenseGraph) Content(context.Context, int64) (string, bool, error) {
	g.events = append(g.events, eventContent)
	return g.node.Content, g.found, nil
}

func (g *recordingCondenseGraph) SetSubstance(_ context.Context, id int64, substance string) error {
	g.events = append(g.events, eventSetSubstance)
	g.written[id] = substance
	return nil
}

func countStrings(all []string, want string) int {
	count := 0
	for _, s := range all {
		if s == want {
			count++
		}
	}
	return count
}

func TestAnUndecoratedCondenseGraphWritesTheSubstanceItIsGivenAndTheDoubleRecordsThatWrite(t *testing.T) {
	live := newRecordingCondenseGraph()

	if err := live.SetSubstance(context.Background(), filledNodeID, filledSubstuff); err != nil {
		t.Fatalf("the undecorated port refused the write: %v", err)
	}

	if got := countStrings(live.events, eventSetSubstance); got != 1 {
		t.Fatalf("the double recorded %d substance writes for one write, so its marker cannot witness a write at all", got)
	}
	if live.written[filledNodeID] != filledSubstuff {
		t.Fatalf("the double stored %q, want %q", live.written[filledNodeID], filledSubstuff)
	}
}

func TestTheFillReadsInFullThroughTheDecoratorWhileNoSubstanceReachesTheGraph(t *testing.T) {
	live := newRecordingCondenseGraph()
	graph := NewCondenseGraph(live)

	node, found, err := graph.NodeWithSubstance(context.Background(), filledNodeID)
	if err != nil || !found {
		t.Fatalf("the decorated read failed: found %v, err %v", found, err)
	}
	if !reflect.DeepEqual(node, live.node) {
		t.Fatalf("the decorated read returned %+v, want the node the graph holds, %+v", node, live.node)
	}

	body, found, err := graph.Content(context.Background(), filledNodeID)
	if err != nil || !found || body != filledBody {
		t.Fatalf("the decorated re-read returned %q (found %v, err %v), want the body the graph holds", body, found, err)
	}

	if err := graph.SetSubstance(context.Background(), filledNodeID, filledSubstuff); err != nil {
		t.Fatalf("the decorator reported a failure the fill would have handled as one: %v", err)
	}

	if got := countStrings(live.events, eventSetSubstance); got != 0 {
		t.Fatalf("the graph saw %d substance writes, and a measured run mutates nothing", got)
	}
	if len(live.written) != 0 {
		t.Fatalf("the graph holds %d written substances, want none", len(live.written))
	}
	if countStrings(live.events, eventNodeWithSubstance) != 1 || countStrings(live.events, eventContent) != 1 {
		t.Fatalf("the fill's reads did not reach the graph in full: %v", live.events)
	}
}

func TestASuppressedSubstanceCarriesItsNodeItsOrdinalAndTheBytesWithheld(t *testing.T) {
	graph := NewCondenseGraph(newRecordingCondenseGraph())

	for _, node := range []int64{filledNodeID, filledNodeID + 1} {
		if err := graph.SetSubstance(context.Background(), node, filledSubstuff); err != nil {
			t.Fatalf("the decorator reported a failure for node %d: %v", node, err)
		}
	}

	want := []SuppressedSubstance{
		{Ordinal: 1, Node: filledNodeID, Size: len(filledSubstuff)},
		{Ordinal: 2, Node: filledNodeID + 1, Size: len(filledSubstuff)},
	}
	if got := graph.Substances(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the suppressed substances are %+v, want %+v", got, want)
	}
	if got := graph.SubstancesSince(1); !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("the window after the first substance is %+v, want %+v", got, want[1:])
	}
}
