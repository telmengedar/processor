package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const viewSubject = 42

type nodeCall struct {
	id     int64
	filter Filter
}

type viewGraph struct {
	visibleWith func(id int64, filter Filter) bool

	recallErr   error
	subjectRoot int64

	nodeCalls   []nodeCall
	recallCalls []RecallRequest
	written     []Record
}

func (g *viewGraph) Node(_ context.Context, id int64, filter Filter) (Anchor, bool, error) {
	g.nodeCalls = append(g.nodeCalls, nodeCall{id: id, filter: filter})
	if g.visibleWith != nil && !g.visibleWith(id, filter) {
		return Anchor{}, false, nil
	}
	return Anchor{ID: id, Type: "thought", Name: fmt.Sprintf("node %d", id), Content: "body", Substance: "substance", RootNodeID: g.subjectRoot}, true, nil
}

func (g *viewGraph) Recall(_ context.Context, request RecallRequest) ([]Candidate, error) {
	g.recallCalls = append(g.recallCalls, request)
	if g.recallErr != nil {
		return nil, g.recallErr
	}
	id := int64(1000 + len(g.recallCalls))
	return []Candidate{{ID: id, Type: "thought", Name: "row", Similarity: 0.9, Content: "body", Substance: "substance"}}, nil
}

func (g *viewGraph) WriteRun(_ context.Context, record Record) WriteReceipt {
	g.written = append(g.written, record)
	return WriteReceipt{State: Stored, NodeID: 9}
}

func runUnder(t *testing.T, graph *viewGraph, view *View, results ...JudgeResult) (Record, *fakeModel, error) {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: viewSubject, Settings: DefaultSettings(), View: view})
	return record, model, err
}

var gangolfView = &View{Config: 77, Filter: Filter{"type": {"thought", "knowledge"}, "minSimilarity": {"0.4"}, "nostatus": {"true"}}}

func TestARunWithoutAConfigSendsNoFilterAtAnySiteAndItsRecordCarriesNoPerspective(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{}
	record, _, err := runUnder(t, graph, nil, recallFor("more"), wantsRead(71), answeredFinal())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != 2 {
		t.Fatalf("the run made %d recalls, want the first and one supplementary", len(graph.recallCalls))
	}
	for i, call := range graph.recallCalls {
		if call.Filter != nil {
			t.Errorf("recall %d carried the filter %v, want none on a run without a config", i, call.Filter)
		}
	}
	for _, call := range graph.nodeCalls {
		if call.filter != nil {
			t.Errorf("the read of %d carried the filter %v, want none on a run without a config", call.id, call.filter)
		}
	}

	if record.Perspective != nil {
		t.Fatalf("the record carries the perspective %+v, want none", record.Perspective)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "perspective") {
		t.Fatalf("the record of a run without a config encodes a perspective key: %s", encoded)
	}
}

func TestAConfiguredRunForwardsTheFilterOnTheFirstRecallAndOnTheModelsRecallAlike(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{}
	if _, _, err := runUnder(t, graph, gangolfView, recallFor("more"), answeredFinal()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.recallCalls) != 2 {
		t.Fatalf("the run made %d recalls, want the first and one supplementary", len(graph.recallCalls))
	}
	for site, i := range map[string]int{"first recall": 0, "supplementary recall": 1} {
		if got := graph.recallCalls[i].Filter; !reflect.DeepEqual(got, gangolfView.Filter) {
			t.Errorf("the %s carried the filter %v, want %v", site, got, gangolfView.Filter)
		}
	}
}

func TestAnAddressedReadUnderAConfigIsMadeByTheGraphUnderTheFilterAndAnExcludedNodeReadsAsAMissingId(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{visibleWith: func(id int64, filter Filter) bool { return id == viewSubject || len(filter) == 0 }}
	record, _, err := runUnder(t, graph, gangolfView, wantsRead(71), answeredFinal())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	round := refusedRound(t, record, errNoSuchNode)
	if round.NodeID != 71 {
		t.Fatalf("the refused round names node %d, want 71", round.NodeID)
	}

	var read *nodeCall
	for i := range graph.nodeCalls {
		if graph.nodeCalls[i].id == 71 {
			read = &graph.nodeCalls[i]
		}
	}
	if read == nil {
		t.Fatal("the graph was never asked for node 71: a refusal made without asking is a client-side judgement")
	}
	if !reflect.DeepEqual(read.filter, gangolfView.Filter) {
		t.Fatalf("the addressed read carried the filter %v, want %v", read.filter, gangolfView.Filter)
	}
}

func TestAnAddressedReadOfANodeInsideTheFilterIsShown(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{visibleWith: func(_ int64, filter Filter) bool {
		return reflect.DeepEqual(filter, gangolfView.Filter) || len(filter) == 0
	}}
	record, _, err := runUnder(t, graph, gangolfView, wantsRead(71), answeredFinal())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	round := onlyRound(t, record)
	if round.Error != "" || len(round.Results) != 1 || round.Results[0].ID != 71 {
		t.Fatalf("the read of a node inside the filter was recorded as %+v, want it shown", round)
	}
}

func TestTheAnchorIsReadWithoutTheFilterEvenUnderAConfig(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{}
	if _, _, err := runUnder(t, graph, gangolfView, answeredFinal()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(graph.nodeCalls) == 0 || graph.nodeCalls[0].id != viewSubject {
		t.Fatalf("the first read was %+v, want the subject's", graph.nodeCalls)
	}
	if graph.nodeCalls[0].filter != nil {
		t.Fatalf("the anchor was read under the filter %v, want none: the caller names the subject itself", graph.nodeCalls[0].filter)
	}
}

func TestAConfiguredRunWhoseFirstRecallTheGraphRejectsFailsAsAnInvalidConfigBeforeAnyJudgementCall(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{recallErr: fmt.Errorf("divoid: unexpected status 414: %w", ErrGraphRejected)}
	_, model, err := runUnder(t, graph, gangolfView, answeredFinal())

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Run() err = %v, want ErrInvalidConfig", err)
	}
	if errors.Is(err, ErrGraphUnavailable) {
		t.Fatalf("Run() err = %v also reads as an unavailable graph: a refused filter is not an outage", err)
	}
	if len(model.calls) != 0 {
		t.Fatalf("the model was called %d times, want none", len(model.calls))
	}
	if len(graph.written) != 0 {
		t.Fatalf("a run that never started wrote %d records", len(graph.written))
	}
}

func TestARejectionOfTheFirstRecallOfARunWithoutAConfigStaysAnUnavailableGraph(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{recallErr: fmt.Errorf("divoid: unexpected status 400: %w", ErrGraphRejected)}
	_, _, err := runUnder(t, graph, nil, answeredFinal())

	if !errors.Is(err, ErrGraphUnavailable) || errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable and not ErrInvalidConfig: nothing the run carried can have caused the refusal", err)
	}
}

func TestARejectionOfTheFirstRecallOfAConfigCarryingNoFilterStaysAnUnavailableGraph(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{recallErr: fmt.Errorf("divoid: unexpected status 414: %w", ErrGraphRejected)}
	_, _, err := runUnder(t, graph, &View{Config: 77}, answeredFinal())

	if !errors.Is(err, ErrGraphUnavailable) || errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable and not ErrInvalidConfig: a config with no filter cannot have caused the refusal", err)
	}
}

func TestAConfiguredRunWhoseGraphFailsToAnswerTheFirstRecallStaysAnUnavailableGraph(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{recallErr: errors.New("divoid: unexpected status 503: down")}
	_, _, err := runUnder(t, graph, gangolfView, answeredFinal())

	if !errors.Is(err, ErrGraphUnavailable) || errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Run() err = %v, want ErrGraphUnavailable and not ErrInvalidConfig", err)
	}
}

func TestAConfiguredRunsRecordCarriesTheConfigIdAndTheFilterVerbatimInTheRecordItFiles(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{}
	record, _, err := runUnder(t, graph, gangolfView, answeredFinal())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !reflect.DeepEqual(record.Perspective, gangolfView) {
		t.Fatalf("the record carries the perspective %+v, want %+v", record.Perspective, gangolfView)
	}
	if len(graph.written) != 1 || !reflect.DeepEqual(graph.written[0].Perspective, gangolfView) {
		t.Fatalf("the filed record carries %+v, want the perspective %+v", graph.written, gangolfView)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Record
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back.Perspective, gangolfView) {
		t.Fatalf("the perspective came back as %+v after an archive round trip, want %+v", back.Perspective, gangolfView)
	}
}

func TestAViewMarshalsItsRootOnlyWhenItHasOneAndDecodesAnOldViewAsUnrooted(t *testing.T) {
	t.Parallel()

	rooted, err := json.Marshal(View{Config: 77, Root: 9001})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(rooted), `"root":9001`) {
		t.Errorf("a rooted view marshalled as %s, want it to carry root 9001", rooted)
	}

	unrooted, err := json.Marshal(View{Config: 77})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(unrooted), "root") {
		t.Errorf("an unrooted view marshalled as %s, want no root key", unrooted)
	}

	var old View
	if err := json.Unmarshal([]byte(`{"config":77,"filter":{"type":["thought"]}}`), &old); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if old.Root != 0 || old.Config != 77 {
		t.Errorf("an old view decoded as %+v, want config 77 and no root", old)
	}
}

func TestARunWithoutAConfigWhoseSubjectHasARootStillRecordsNoPerspective(t *testing.T) {
	t.Parallel()

	graph := &viewGraph{subjectRoot: 31}
	record, _, err := runUnder(t, graph, nil, answeredFinal())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if record.Perspective != nil {
		t.Errorf("the record carries the perspective %+v, want none for a run without a config", record.Perspective)
	}
	if len(graph.written) != 1 || graph.written[0].Perspective != nil {
		t.Errorf("the written record was %+v, want no perspective", graph.written)
	}
}
