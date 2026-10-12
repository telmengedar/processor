package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeFill struct {
	mu        sync.Mutex
	calls     []int64
	deferred  []int64
	results   map[int64]FillResult
	errs      map[int64]error
	blocked   map[int64]bool
	cancelled map[int64]bool
	onDefer   func()
}

func (f *fakeFill) Fill(ctx context.Context, id int64) (FillResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, id)
	blocked := f.blocked[id]
	err := f.errs[id]
	result, scripted := f.results[id]
	f.mu.Unlock()

	if blocked {
		<-ctx.Done()
		f.mu.Lock()
		if f.cancelled == nil {
			f.cancelled = map[int64]bool{}
		}
		f.cancelled[id] = true
		f.mu.Unlock()
		return FillResult{}, ctx.Err()
	}
	if err != nil {
		return FillResult{}, err
	}
	if scripted {
		return result, nil
	}
	return FillResult{Written: true, Substance: createdText(id), Model: "test-condense-model"}, nil
}

func (f *fakeFill) Defer(id int64) {
	f.mu.Lock()
	f.deferred = append(f.deferred, id)
	hook := f.onDefer
	f.mu.Unlock()

	if hook != nil {
		hook()
	}
}

func (f *fakeFill) called() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.calls...)
}

func (f *fakeFill) wasCancelled(id int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelled[id]
}

func (f *fakeFill) deferrals() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.deferred...)
}

func waitUntil(condition func() bool) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return condition()
}

func createdText(id int64) string {
	return fmt.Sprintf("created substance of %d", id)
}

func outcomeFor(t *testing.T, outcomes []FillOutcome, id int64) FillOutcome {
	t.Helper()

	for _, o := range outcomes {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("no fill outcome for %d in %+v", id, outcomes)
	return FillOutcome{}
}

func bareRow(id int64) Candidate {
	return Candidate{ID: id, Type: "documentation", Name: "Bare", Similarity: 0.9, Content: "the body of " + fmt.Sprint(id)}
}

func turnWithFill(graph *fakeGraph, model *fakeModel, port FillPort) *Turn {
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	turn.Fill = port
	return turn
}

func TestNoRenderedRowEverCarriesItsNodesContent(t *testing.T) {
	t.Parallel()

	const sentinel = "CONTENT-SENTINEL"

	graph := &fakeGraph{
		node:      Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: sentinel + "-anchor", Substance: "substance of the anchor"},
		nodeFound: true,
		recallQueue: []recallResponse{
			{Candidates: []Candidate{{ID: 10, Type: "documentation", Name: "Block", Similarity: 0.9, Content: sentinel + "-block", Substance: "substance of the block row"}}},
			{Candidates: []Candidate{{ID: 20, Type: "documentation", Name: "Recall", Similarity: 0.8, Content: sentinel + "-recall", Substance: "substance of the recall row"}}},
		},
		nodes: map[int64]nodeResponse{
			42: {Anchor: Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: sentinel + "-anchor", Substance: "substance of the anchor"}, Found: true},
			30: {Anchor: Anchor{ID: 30, Type: "documentation", Name: "Read", Content: sentinel + "-read", Substance: "substance of the read row"}, Found: true},
		},
	}
	model := &fakeModel{results: []JudgeResult{recallFor("more"), wantsRead(30), answeredFinal()}}

	record, _, err := turnWithFill(graph, model, &fakeFill{}).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(model.calls) != 3 {
		t.Fatalf("the model was called %d times, want 3: the recall and the read must both have been dispatched", len(model.calls))
	}
	for i, call := range model.calls {
		if strings.Contains(call.Block, sentinel) {
			t.Errorf("call %d rendered a node's content:\n%s", i+1, call.Block)
		}
	}
	for _, want := range []string{"substance of the anchor", "substance of the block row", "substance of the recall row", "substance of the read row"} {
		if !strings.Contains(model.calls[2].Block, want) {
			t.Errorf("the last call does not carry %q:\n%s", want, model.calls[2].Block)
		}
	}
	if strings.Contains(record.Block, sentinel) {
		t.Errorf("the recorded block carries a node's content:\n%s", record.Block)
	}
}

func TestARowWithoutASubstanceRendersItsHeaderAndTheUnavailableLineAndNoBody(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "documentation", Name: "Anchor", Substance: "anchor substance"}
	candidates := []Candidate{{ID: 10, Type: "bug", Name: "Bare", Similarity: 0.5, Content: "CONTENT-SENTINEL"}}

	block, dispositions := Assemble(anchor, candidates, 60_000)

	const want = "===== CANDIDATE =====\nid: 10\ntype: bug\nname: Bare\nsimilarity: 0.500\nsubstance: unavailable\n"
	if !strings.Contains(block, want) {
		t.Fatalf("the block does not render the header and the unavailable line:\n%q\nwant it to contain:\n%q", block, want)
	}
	if strings.Contains(block, "CONTENT-SENTINEL") {
		t.Fatalf("the block rendered a body for a row without a substance:\n%s", block)
	}
	if dispositions[0].Form != FormUnavailable || dispositions[0].RenderedSize != 0 || !dispositions[0].Included {
		t.Fatalf("disposition = %+v, want an included row in form %q rendering 0 bytes", dispositions[0], FormUnavailable)
	}
}

func TestTheAnchorRendersItsSubstance(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "documentation", Name: "Anchor", Content: "CONTENT-SENTINEL", Substance: "anchor substance"}

	block, _ := Assemble(anchor, nil, 60_000)

	if !strings.HasPrefix(block, "===== ANCHOR =====\nid: 1\ntype: documentation\nname: Anchor\n\nanchor substance\n") {
		t.Fatalf("the anchor section does not render the substance:\n%q", block)
	}
	if strings.Contains(block, "CONTENT-SENTINEL") {
		t.Fatalf("the anchor section rendered the content:\n%s", block)
	}
}

func TestAnAnchorWithoutASubstanceRendersTheUnavailableLine(t *testing.T) {
	t.Parallel()

	block, _ := Assemble(Anchor{ID: 1, Type: "documentation", Name: "Anchor", Content: "CONTENT-SENTINEL"}, nil, 60_000)

	if !strings.HasPrefix(block, "===== ANCHOR =====\nid: 1\ntype: documentation\nname: Anchor\nsubstance: unavailable\n") {
		t.Fatalf("the anchor section does not render the unavailable line:\n%q", block)
	}
	if strings.Contains(block, "CONTENT-SENTINEL") {
		t.Fatalf("the anchor section rendered the content:\n%s", block)
	}
}

func TestTheAnchorIsChargedItsRenderedBytesNotItsContentLength(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Content: strings.Repeat("c", 10_000), Substance: strings.Repeat("s", 10)}
	candidates := []Candidate{{ID: 10, Substance: strings.Repeat("t", 80)}}

	_, dispositions := Assemble(anchor, candidates, 100)

	if !dispositions[0].Included {
		t.Fatalf("the candidate was cut as %q although the anchor's 10 rendered bytes and its own 80 fit a budget of 100: the anchor was charged its content length", dispositions[0].CutReason)
	}

	memory := newWorkingMemory(anchor, true, nil, nil)
	if memory.spent != 10 {
		t.Fatalf("the memory starts charged %d bytes, want the anchor's 10 rendered bytes", memory.spent)
	}
}

func TestTheFormRuleReadsNoContentLength(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "documentation", Name: "Anchor", Substance: "anchor substance"}
	short := []Candidate{{ID: 10, Type: "documentation", Name: "Row", Similarity: 0.9, Content: strings.Repeat("c", 100), Substance: "the same substance"}}
	long := []Candidate{{ID: 10, Type: "documentation", Name: "Row", Similarity: 0.9, Content: strings.Repeat("c", 10_000), Substance: "the same substance"}}

	blockShort, dispositionsShort := Assemble(anchor, short, 60_000)
	blockLong, dispositionsLong := Assemble(anchor, long, 60_000)

	if blockShort != blockLong {
		t.Fatalf("the block changed with the content's length:\nshort=%q\nlong=%q", blockShort, blockLong)
	}
	if dispositionsShort[0].Form != dispositionsLong[0].Form || dispositionsShort[0].RenderedSize != dispositionsLong[0].RenderedSize || dispositionsShort[0].Included != dispositionsLong[0].Included {
		t.Fatalf("the form decision changed with the content's length: %+v vs %+v", dispositionsShort[0], dispositionsLong[0])
	}
}

func TestTheSubstanceStepCreatesBeforeAssemblyAndTheBlockCarriesTheCreatedText(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.node.Substance = ""
	graph.candidates = []Candidate{bareRow(91), {ID: 92, Type: "documentation", Name: "Has one", Similarity: 0.8, Substance: "already condensed"}}
	port := &fakeFill{}
	model := &fakeModel{}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := port.called(); len(got) != 2 {
		t.Fatalf("the port was asked for %v, want the anchor and the one candidate that lacked a substance", got)
	}
	block := model.calls[0].Block
	for _, want := range []string{createdText(42), createdText(91), "already condensed"} {
		if !strings.Contains(block, want) {
			t.Errorf("the first call's block does not carry %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "unavailable") {
		t.Errorf("the first call's block renders a row as unavailable although its creation succeeded:\n%s", block)
	}

	created := outcomeFor(t, record.Fills, 91)
	if !created.Filled || created.Phase != phaseInitial || created.Model != "test-condense-model" {
		t.Fatalf("the outcome for the created row = %+v, want filled in phase %q", created, phaseInitial)
	}
	if record.Candidates[0].SubstanceAvailable || record.Candidates[0].Form != FormSubstance {
		t.Fatalf("the disposition = %+v, want it to record that no substance was available at retrieval and the form rendered is substance", record.Candidates[0])
	}
	if record.Anchor.Form != FormSubstance || record.Anchor.RenderedSize != len(createdText(42)) {
		t.Fatalf("the anchor summary = %+v, want the created substance as its rendered form", record.Anchor)
	}
}

func TestTheSubstanceStepRunsOnARecallRoundsUnheldRows(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: nil},
		{Candidates: []Candidate{bareRow(91)}},
	}
	port := &fakeFill{}
	model := &fakeModel{results: []JudgeResult{recallFor("more"), answeredFinal()}}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := port.called(); len(got) != 1 || got[0] != 91 {
		t.Fatalf("the port was asked for %v, want the recall row alone", got)
	}
	if !strings.Contains(model.calls[1].Block, createdText(91)) {
		t.Fatalf("the call after the recall round does not carry the created text:\n%s", model.calls[1].Block)
	}
	created := outcomeFor(t, record.Fills, 91)
	if !created.Filled || created.Phase != phaseRecall {
		t.Fatalf("the outcome = %+v, want filled in phase %q", created, phaseRecall)
	}
	round := record.ToolCalls[0]
	if len(round.Results) != 1 || round.Results[0].Form != FormSubstance {
		t.Fatalf("the round recorded %+v, want the row in form %q", round.Results, FormSubstance)
	}
}

func TestARecallRoundDoesNotRetryARowTheMemoryAlreadyHoldsUnavailable(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{bareRow(91)}},
		{Candidates: []Candidate{bareRow(91)}},
	}
	port := &fakeFill{errs: map[int64]error{91: errors.New("model call failed")}}
	model := &fakeModel{results: []JudgeResult{recallFor("more"), answeredFinal()}}

	if _, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := port.called(); len(got) != 1 {
		t.Fatalf("the port was asked for %v, want one attempt: a row held unavailable is not retried by a recall that returns it again", got)
	}
}

func TestTheSubstanceStepRunsOnAnAddressedReadOfANodeWithoutASubstance(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes(Anchor{ID: 71, Type: "documentation", Name: "Read me", Content: "CONTENT-SENTINEL"})
	port := &fakeFill{}
	model := &fakeModel{results: []JudgeResult{wantsRead(71), answeredFinal()}}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(model.calls[1].Block, createdText(71)) || strings.Contains(model.calls[1].Block, "CONTENT-SENTINEL") {
		t.Fatalf("the call after the read does not carry the created substance alone:\n%s", model.calls[1].Block)
	}
	created := outcomeFor(t, record.Fills, 71)
	if !created.Filled || created.Phase != phaseRead {
		t.Fatalf("the outcome = %+v, want filled in phase %q", created, phaseRead)
	}
	round := onlyRound(t, record)
	if round.Error != "" || len(round.Results) != 1 || round.Results[0].Form != FormSubstance {
		t.Fatalf("the read round = %+v, want one served row in form %q", round, FormSubstance)
	}
}

func TestACreationStillRunningAtTheWaitIsCancelledDeferredAndRenderedUnavailable(t *testing.T) {
	t.Parallel()

	now := time.Now()
	graph := baseGraph()
	graph.candidates = []Candidate{bareRow(91), bareRow(92)}
	port := &fakeFill{blocked: map[int64]bool{91: true}}
	model := &fakeModel{}
	turn := turnWithFill(graph, model, port)
	turn.clock = func() time.Time { return now }

	ctx, cancel := context.WithDeadline(context.Background(), now.Add(JudgementCost(turn.Floors)+50*time.Millisecond))
	defer cancel()

	record, _, err := turn.Run(ctx, "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	slow := outcomeFor(t, record.Fills, 91)
	if slow.Filled || !slow.Deferred || slow.Reason != fillReasonWaitExpired {
		t.Fatalf("the outcome of the slow creation = %+v, want it deferred with reason %q", slow, fillReasonWaitExpired)
	}
	if !waitUntil(func() bool { return port.wasCancelled(91) }) {
		t.Fatal("the slow creation was not cancelled at the wait")
	}
	if got := port.deferrals(); len(got) != 1 || got[0] != 91 {
		t.Fatalf("deferred %v, want the slow node alone", got)
	}
	block := model.calls[0].Block
	if !strings.Contains(block, "id: 91\ntype: documentation\nname: Bare\nsimilarity: 0.900\nsubstance: unavailable\n") {
		t.Fatalf("the slow row is not rendered unavailable:\n%s", block)
	}
	if !strings.Contains(block, createdText(92)) {
		t.Fatalf("the fast row's created text is missing from the block:\n%s", block)
	}
}

func TestADeferralReachesThePortOnlyAfterTheRunIsFiled(t *testing.T) {
	t.Parallel()

	now := time.Now()
	graph := baseGraph()
	graph.candidates = []Candidate{bareRow(91)}
	port := &fakeFill{blocked: map[int64]bool{91: true}}
	turn := turnWithFill(graph, &fakeModel{}, port)
	turn.clock = func() time.Time { return now }

	filedWhenDeferred := false
	port.onDefer = func() { filedWhenDeferred = graph.writeRunCalled }

	ctx, cancel := context.WithDeadline(context.Background(), now.Add(JudgementCost(turn.Floors)+20*time.Millisecond))
	defer cancel()

	if _, _, err := turn.Run(ctx, "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(port.deferrals()) != 1 {
		t.Fatalf("deferred %v, want one node", port.deferrals())
	}
	if !filedWhenDeferred {
		t.Fatal("the node was handed to the deferred worker before the run was filed: the worker would compete with the turn's own calls")
	}
}

func TestTheWaitNeverSpendsTheTimeOneJudgementCallNeeds(t *testing.T) {
	t.Parallel()

	now := time.Now()
	turn := &Turn{clock: func() time.Time { return now }, logger: testLogger()}
	cost := JudgementCost(turn.Floors)

	far, cancelFar := context.WithDeadline(context.Background(), now.Add(cost+10*FillWait))
	defer cancelFar()
	if got := turn.fillWait(far); got != FillWait {
		t.Fatalf("fillWait with a distant deadline = %v, want %v", got, FillWait)
	}

	near, cancelNear := context.WithDeadline(context.Background(), now.Add(cost+3*time.Second))
	defer cancelNear()
	if got := turn.fillWait(near); got != 3*time.Second {
		t.Fatalf("fillWait with a deadline %v past one judgement call's cost = %v, want exactly that margin", 3*time.Second, got)
	}

	tight, cancelTight := context.WithDeadline(context.Background(), now.Add(cost/2))
	defer cancelTight()
	if got := turn.fillWait(tight); got != 0 {
		t.Fatalf("fillWait with less time left than one judgement call needs = %v, want none", got)
	}

	if got := turn.fillWait(context.Background()); got != FillWait {
		t.Fatalf("fillWait with no deadline = %v, want %v", got, FillWait)
	}
}

func TestAStepWithNoTimeToSpareStartsNoCreationAndDefersEveryRow(t *testing.T) {
	t.Parallel()

	now := time.Now()
	port := &fakeFill{}
	turn := &Turn{Fill: port, clock: func() time.Time { return now }, logger: testLogger()}

	ctx, cancel := context.WithDeadline(context.Background(), now.Add(JudgementCost(turn.Floors)/2))
	defer cancel()

	created, outcomes := turn.createSubstances(ctx, phaseRecall, []int64{1, 2})

	if len(created) != 0 || len(port.called()) != 0 {
		t.Fatalf("created %v after calls %v, want no creation started with no time to spare", created, port.called())
	}
	for _, id := range []int64{1, 2} {
		if got := outcomeFor(t, outcomes, id); !got.Deferred {
			t.Fatalf("outcome for %d = %+v, want it deferred", id, got)
		}
	}
}

func TestAFailedCreationRendersUnavailableAndTheTurnProceeds(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = []Candidate{bareRow(91), bareRow(92)}
	port := &fakeFill{
		errs:    map[int64]error{91: errors.New("model call failed: " + strings.Repeat("x", 4096))},
		results: map[int64]FillResult{92: {Reason: "condensation empty"}},
	}
	model := &fakeModel{}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if record.ModelCalls != 1 {
		t.Fatalf("the turn made %d judgement calls, want the one it proceeded to", record.ModelCalls)
	}
	failed := outcomeFor(t, record.Fills, 91)
	if failed.Filled || failed.Deferred || !strings.HasPrefix(failed.Reason, "model call failed") {
		t.Fatalf("outcome = %+v, want a plain failure carrying the cause", failed)
	}
	if length := len([]rune(failed.Reason)); length != CarriedCauseRunes {
		t.Fatalf("the carried cause is %d runes, want it bounded to %d", length, CarriedCauseRunes)
	}
	if got := outcomeFor(t, record.Fills, 92); got.Reason != "condensation empty" {
		t.Fatalf("outcome = %+v, want the producer's own reason", got)
	}
	if got := port.deferrals(); len(got) != 0 {
		t.Fatalf("deferred %v, want nothing: a deterministic rejection would only be reproduced", got)
	}
	if strings.Count(model.calls[0].Block, "substance: unavailable") != 2 {
		t.Fatalf("both failed rows should render unavailable:\n%s", model.calls[0].Block)
	}
}

func TestAStepWithNoFillPortRecordsThePortAbsentReasonForEveryRow(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = []Candidate{bareRow(91)}
	turn := NewTurn(graph, &fakeModel{}, nil, fixedSystem("system"), "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := outcomeFor(t, record.Fills, 91); got.Filled || got.Reason != fillReasonPortAbsent {
		t.Fatalf("outcome = %+v, want %q", got, fillReasonPortAbsent)
	}
}

func TestAPresentSubstanceTheProducerFoundIsRenderedAndNotCountedAsWritten(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.candidates = []Candidate{bareRow(91)}
	port := &fakeFill{results: map[int64]FillResult{91: {Substance: "written by another turn", Reason: "substance present"}}}
	model := &fakeModel{}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(model.calls[0].Block, "written by another turn") {
		t.Fatalf("the block does not carry the substance the producer found:\n%s", model.calls[0].Block)
	}
	if got := outcomeFor(t, record.Fills, 91); got.Filled {
		t.Fatalf("outcome = %+v, want it not counted as written by this turn", got)
	}
}

func TestAReadOfAnUnavailableRowWhoseSubstanceNowExistsIsServed(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes()
	graph.recallQueue = []recallResponse{{Candidates: []Candidate{bareRow(91)}}}
	port := &fakeFill{errs: map[int64]error{91: errors.New("model call failed")}}
	model := &fakeModel{results: []JudgeResult{wantsRead(91), answeredFinal()}}

	graph.nodes[91] = nodeResponse{Anchor: Anchor{ID: 91, Type: "documentation", Name: "Bare", Content: bareRow(91).Content, Substance: "condensed since"}, Found: true}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	round := onlyRound(t, record)
	if round.Error != "" || len(round.Results) != 1 || round.Results[0].Form != FormSubstance {
		t.Fatalf("the read round = %+v, want the row served in form %q", round, FormSubstance)
	}
	if got := port.called(); len(got) != 1 {
		t.Fatalf("the port was asked for %v, want the initial attempt alone: the read found the substance already present", got)
	}
	if !strings.Contains(model.calls[1].Block, "condensed since") {
		t.Fatalf("the call after the read does not carry the substance:\n%s", model.calls[1].Block)
	}
}

func TestAReadOfARowThatIsStillUnavailableTriesTheCreationAgainAndIsThenRefusedAsAlreadyShown(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes(Anchor{ID: 91, Type: "documentation", Name: "Bare", Content: "the body of 91"})
	graph.recallQueue = []recallResponse{{Candidates: []Candidate{bareRow(91)}}}
	port := &fakeFill{errs: map[int64]error{91: errors.New("model call failed")}}
	model := &fakeModel{results: []JudgeResult{wantsRead(91), answeredFinal()}}

	record, _, err := turnWithFill(graph, model, port).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := port.called(); len(got) != 2 {
		t.Fatalf("the port was asked for %v, want the initial attempt and the retry the read makes", got)
	}
	round := onlyRound(t, record)
	if round.Error != errAlreadyReadInFull {
		t.Fatalf("the read round carries %q, want the already-shown refusal that ends the turn's retrieval", round.Error)
	}
	if len(record.Fills) != 2 || record.Fills[1].Phase != phaseRead {
		t.Fatalf("record.Fills = %+v, want both attempts recorded, the second in phase %q", record.Fills, phaseRead)
	}
}

func TestTheSubstanceStepNeverAsksForARecordThisSystemWrote(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	own := bareRow(91)
	own.SelfProduced = true
	graph.candidates = []Candidate{own, bareRow(92)}
	graph.ignoresExclusion = true
	port := &fakeFill{}

	if _, _, err := turnWithFill(graph, &fakeModel{}, port).Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := port.called(); len(got) != 1 || got[0] != 92 {
		t.Fatalf("the port was asked for %v, want the one row that is not a record this system wrote", got)
	}
}

func TestADeferralIsHandedOverEvenWhenTheJudgementFails(t *testing.T) {
	t.Parallel()

	now := time.Now()
	graph := baseGraph()
	graph.candidates = []Candidate{bareRow(91)}
	port := &fakeFill{blocked: map[int64]bool{91: true}}
	model := &fakeModel{err: errors.New("endpoint down")}
	turn := turnWithFill(graph, model, port)
	turn.clock = func() time.Time { return now }

	ctx, cancel := context.WithDeadline(context.Background(), now.Add(JudgementCost(turn.Floors)+20*time.Millisecond))
	defer cancel()

	if _, _, err := turn.Run(ctx, "hello", 42); err == nil {
		t.Fatal("Run succeeded although the judgement call failed")
	}

	if got := port.deferrals(); len(got) != 1 || got[0] != 91 {
		t.Fatalf("deferred %v, want the slow node: a failed turn still leaves its cancelled creation to the worker", got)
	}
}

func TestMissingSubstancesNamesNoRecordThisSystemWrote(t *testing.T) {
	t.Parallel()

	own := bareRow(1)
	own.SelfProduced = true

	got := missingSubstances([]Candidate{own, bareRow(2), {ID: 3, Substance: "has one"}}, nil)

	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("missingSubstances = %v, want only the row that lacks a substance and is not a record this system wrote", got)
	}
}
