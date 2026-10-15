package loop

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	contentSubject = 42
	contentMarker  = "C-"
	substanceMark  = "S-"
)

func contentNode(id int64, name string) Anchor {
	return Anchor{ID: id, Type: "knowledge", Name: name, Content: contentMarker + name, Substance: substanceMark + name}
}

func contentRow(id int64, name string) Candidate {
	return Candidate{ID: id, Type: "knowledge", Name: name, Similarity: 0.9, Content: contentMarker + name, Substance: substanceMark + name}
}

func contentGraph() *fakeGraph {
	return &fakeGraph{
		node:      contentNode(contentSubject, "anchor"),
		nodeFound: true,
		recallQueue: []recallResponse{
			{Candidates: []Candidate{contentRow(10, "block")}},
			{Candidates: []Candidate{contentRow(10, "block"), contentRow(20, "recall")}},
		},
		nodes: map[int64]nodeResponse{
			contentSubject: {Anchor: contentNode(contentSubject, "anchor"), Found: true},
			30:             {Anchor: contentNode(30, "read"), Found: true},
		},
	}
}

func bareContentGraph() *fakeGraph {
	bare := func(id int64, name string) Candidate {
		return Candidate{ID: id, Type: "knowledge", Name: name, Similarity: 0.9, Content: contentMarker + name}
	}
	return &fakeGraph{
		node:      Anchor{ID: contentSubject, Type: "knowledge", Name: "anchor", Content: contentMarker + "anchor"},
		nodeFound: true,
		recallQueue: []recallResponse{
			{Candidates: []Candidate{bare(10, "block")}},
			{Candidates: []Candidate{bare(20, "recall")}},
		},
		nodes: map[int64]nodeResponse{
			contentSubject: {Anchor: Anchor{ID: contentSubject, Type: "knowledge", Name: "anchor", Content: contentMarker + "anchor"}, Found: true},
			30:             {Anchor: Anchor{ID: 30, Type: "knowledge", Name: "read", Content: contentMarker + "read"}, Found: true},
		},
	}
}

func formReportingSystem(offered []string, _, _ bool, form Form) string {
	return "form=" + string(form)
}

func runForm(t *testing.T, graph *fakeGraph, fill FillPort, view *View, settings RunSettings, results ...JudgeResult) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: results}
	turn := NewTurn(graph, model, nil, formReportingSystem, "test-model", testLogger())
	turn.Fill = fill
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: contentSubject, Settings: settings, View: view})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	return record, model
}

func contentView() *View {
	return &View{Config: 9, Form: FormContent}
}

func everyDisposition(record Record) []Disposition {
	rows := append([]Disposition{}, record.Candidates...)
	for _, call := range record.ToolCalls {
		rows = append(rows, call.Results...)
	}
	return rows
}

func TestAContentRunRendersTheAnchorEveryCandidateEveryRecallRowAndEveryReadAsItsContent(t *testing.T) {
	t.Parallel()

	record, model := runForm(t, contentGraph(), nil, contentView(), DefaultSettings(), recallFor("more"), wantsRead(30), answeredFinal())

	for _, want := range []string{"C-anchor", "C-block"} {
		if !strings.Contains(record.Block, want) {
			t.Errorf("the initial block lacks %q: %s", want, record.Block)
		}
	}
	if len(model.calls) != 3 {
		t.Fatalf("the model was called %d times, want 3 (recall, read, answer)", len(model.calls))
	}
	if !strings.Contains(model.calls[1].Block, "C-recall") {
		t.Errorf("the block after the recall lacks the recall row's content: %s", model.calls[1].Block)
	}
	if !strings.Contains(model.calls[2].Block, "C-read") {
		t.Errorf("the block after the read lacks the read part's content: %s", model.calls[2].Block)
	}
	for i, call := range model.calls {
		if strings.Contains(call.Block, substanceMark) {
			t.Errorf("call %d was sent a block carrying a substance: %s", i, call.Block)
		}
	}
	if strings.Contains(record.Block, substanceMark) {
		t.Errorf("the recorded block carries a substance: %s", record.Block)
	}

	if record.Anchor.Form != FormContent || record.Anchor.RenderedSize != len("C-anchor") {
		t.Errorf("the anchor summary is %+v, want form content at the content's own size", record.Anchor)
	}
	rows := everyDisposition(record)
	if len(rows) < 3 {
		t.Fatalf("the record carries %d rows, want the candidate, the recall rows and the read", len(rows))
	}
	for _, row := range rows {
		if row.Form != FormContent {
			t.Errorf("row %d is recorded as form %q, want content", row.ID, row.Form)
		}
		if row.RenderedSize != row.Size {
			t.Errorf("row %d was charged %d bytes but its content is %d", row.ID, row.RenderedSize, row.Size)
		}
	}

	recall := record.ToolCalls[0].Results
	if len(recall) != 2 || !recall[0].Held || recall[0].ID != 10 {
		t.Errorf("the recall rows are %+v, want the part already shown held and the new one admitted", recall)
	}
}

func TestAContentRunSendsEachCallTheFormItIsHeldTo(t *testing.T) {
	t.Parallel()

	_, model := runForm(t, contentGraph(), nil, contentView(), DefaultSettings(), recallFor("more"), answeredFinal())
	for i, call := range model.calls {
		if call.System != "form=content" {
			t.Errorf("call %d was composed for %q, want form=content", i, call.System)
		}
	}

	_, model = runForm(t, contentGraph(), nil, nil, DefaultSettings(), recallFor("more"), answeredFinal())
	for i, call := range model.calls {
		if call.System != "form=" {
			t.Errorf("call %d of a run without a config was composed for %q, want the empty form", i, call.System)
		}
	}
}

func TestAContentRunNeverCallsTheFillPortInAnyPhaseAndRecordsNoFills(t *testing.T) {
	t.Parallel()

	port := &fakeFill{}
	record, _ := runForm(t, bareContentGraph(), port, contentView(), DefaultSettings(), recallFor("more"), wantsRead(30), answeredFinal())

	if calls := port.called(); len(calls) != 0 {
		t.Errorf("the fill port was asked to fill %v on a content run", calls)
	}
	if deferred := port.deferrals(); len(deferred) != 0 {
		t.Errorf("the fill port was handed %v to defer on a content run", deferred)
	}
	if len(record.Fills) != 0 {
		t.Errorf("the record carries fills %+v", record.Fills)
	}
	for _, call := range record.ToolCalls {
		if len(call.Results) == 0 {
			t.Errorf("round %q admitted nothing, so the phase it stands for was not exercised: %+v", call.Tool, call)
		}
	}

	control := &fakeFill{}
	runForm(t, bareContentGraph(), control, nil, DefaultSettings(), recallFor("more"), wantsRead(30), answeredFinal())
	if len(control.called()) == 0 {
		t.Fatalf("test setup error: the same graph on the default path never reached the fill port, so the counting port proves nothing")
	}
}

func TestAContentRunReachesNoCondensationLogic(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var touched []string
	port := &fakeFill{onDefer: func() { mu.Lock(); touched = append(touched, "defer"); mu.Unlock() }, blocked: map[int64]bool{10: true, 20: true, 30: true, contentSubject: true}}
	settings := DefaultSettings()
	settings.FillWait = 20 * time.Millisecond

	record, _ := runForm(t, bareContentGraph(), port, contentView(), settings, recallFor("more"), wantsRead(30), answeredFinal())

	if len(touched) != 0 || len(port.called()) != 0 {
		t.Errorf("condensation was reached: defers %v, fills %v", touched, port.called())
	}
	rows := everyDisposition(record)
	rows = append(rows, Disposition{Form: record.Anchor.Form})
	for _, row := range rows {
		if row.Form == FormSubstance || row.Form == FormUnavailable {
			t.Errorf("a row of a content run is recorded as %q", row.Form)
		}
	}
}

func TestAContentRunChargesTheBudgetTheContentBytesForTheAnchorAndEveryRow(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: contentSubject, Type: "knowledge", Name: "anchor", Content: strings.Repeat("a", 30), Substance: "aaaaa"}
	rows := []Candidate{
		{ID: 10, Type: "knowledge", Name: "wide", Similarity: 0.9, Content: strings.Repeat("w", 40), Substance: "wwwww"},
		{ID: 11, Type: "knowledge", Name: "narrow", Similarity: 0.8, Content: strings.Repeat("n", 20), Substance: "nnnnn"},
	}

	_, dispositions := Assemble(anchor, rows, 60, FormContent)
	if dispositions[0].Included || dispositions[0].CutReason != cutReasonByteBudget {
		t.Errorf("the wide row is %+v, want it cut: the anchor's 30 and its own 40 content bytes exceed 60, which substance bytes would not", dispositions[0])
	}
	if !dispositions[1].Included || dispositions[1].RenderedSize != 20 {
		t.Errorf("the narrow row is %+v, want it admitted at its 20 content bytes", dispositions[1])
	}

	_, substance := Assemble(anchor, rows, 60, FormSubstance)
	if !substance[0].Included || !substance[1].Included {
		t.Fatalf("test setup error: on the substance path both rows fit (%+v), so the content charge above proves nothing", substance)
	}
}

func TestAContentRunsWorkingMemoryStartsFromTheAnchorsContentBytes(t *testing.T) {
	t.Parallel()

	graph := contentGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{contentRow(10, "block")}},
		{Candidates: []Candidate{{ID: 20, Type: "knowledge", Name: "bulky", Similarity: 0.9, Content: strings.Repeat("r", 85), Substance: "rrrrr"}}},
	}
	wide := contentNode(contentSubject, "anchor")
	wide.Content = strings.Repeat("a", 50)
	graph.nodes[contentSubject] = nodeResponse{Anchor: wide, Found: true}
	view := &View{Config: 9, Form: FormContent, Identity: strings.Repeat("i", 20)}
	settings := DefaultSettings()
	settings.AssemblyByteBudget = 60
	settings.SupplementaryByteBudget = 100

	record, _ := runForm(t, graph, nil, view, settings, recallFor("more"), answeredFinal())

	round := record.ToolCalls[0]
	if len(round.Results) != 1 || round.Results[0].Included {
		t.Fatalf("the bulky recall row is %+v, want it cut: the anchor's 50, the block row's 7 and the identity's 20 bytes leave 83 of the 160-byte ceiling, which its 85 content bytes exceed", round.Results)
	}
}

func TestAnEmptyContentRendersItsHeaderAloneAndNeverAsUnavailable(t *testing.T) {
	t.Parallel()

	anchor := Anchor{ID: 1, Type: "knowledge", Name: "bare anchor", Substance: "a substance the content run never shows"}
	rows := []Candidate{{ID: 2, Type: "knowledge", Name: "empty", Similarity: 0.5, Substance: "never shown"}}

	block, dispositions := Assemble(anchor, rows, 1000, FormContent)

	want := "===== ANCHOR =====\nid: 1\ntype: knowledge\nname: bare anchor\n\n===== CANDIDATE =====\nid: 2\ntype: knowledge\nname: empty\nsimilarity: 0.500\n"
	if block != want {
		t.Errorf("block = %q, want %q", block, want)
	}
	if strings.Contains(block, "unavailable") {
		t.Errorf("an empty content was written as unavailable: %q", block)
	}
	if d := dispositions[0]; d.Form != FormContent || d.RenderedSize != 0 || !d.Included {
		t.Errorf("the empty row is %+v, want form content, size 0, included", d)
	}
	if summary := summarizeAnchor(anchor, FormContent); summary.Form != FormContent || summary.RenderedSize != 0 {
		t.Errorf("the empty anchor is summarised as %+v, want form content, size 0", summary)
	}
}

func TestAReadOfAPartAlreadyShownAsContentIsRefusedAsAlreadyShown(t *testing.T) {
	t.Parallel()

	graphWithTenAsNode := contentGraph()
	graphWithTenAsNode.nodes[10] = nodeResponse{Anchor: contentNode(10, "block"), Found: true}
	record, _ := runForm(t, graphWithTenAsNode, nil, contentView(), DefaultSettings(), wantsRead(10), answeredFinal())

	round := refusedRound(t, record, errAlreadyShownThisTurn)
	if round.NodeID != 10 {
		t.Errorf("the refused round asked for %d, want 10", round.NodeID)
	}
}

func TestAReadTooLargeForTheRoundIsRefusedNamingTheContentOnAContentRunAndTheSubstanceOtherwise(t *testing.T) {
	t.Parallel()

	big := Anchor{ID: 32, Type: "knowledge", Name: "big", Content: strings.Repeat("c", 30000), Substance: strings.Repeat("s", 30000)}
	graph := func() *fakeGraph {
		g := contentGraph()
		g.nodes[32] = nodeResponse{Anchor: big, Found: true}
		return g
	}

	content, _ := runForm(t, graph(), nil, contentView(), DefaultSettings(), wantsRead(32), answeredFinal())
	refusedRound(t, content, "that part's content is 30000 bytes and one read carries at most 20000, so it cannot be shown")

	const wantSubstance = "that part's substance is 30000 bytes and one read carries at most 20000, so it cannot be shown"

	absent, _ := runForm(t, graph(), nil, nil, DefaultSettings(), wantsRead(32), answeredFinal())
	refusedRound(t, absent, wantSubstance)

	explicit, _ := runForm(t, graph(), nil, &View{Config: 9, Form: FormSubstance}, DefaultSettings(), wantsRead(32), answeredFinal())
	refusedRound(t, explicit, wantSubstance)
}

func TestAContentRunsRecordAndSummaryStateTheForm(t *testing.T) {
	t.Parallel()

	content, _ := runForm(t, contentGraph(), nil, contentView(), DefaultSettings(), answeredFinal())
	substance, _ := runForm(t, contentGraph(), nil, nil, DefaultSettings(), answeredFinal())
	explicit, _ := runForm(t, contentGraph(), nil, &View{Config: 9, Form: FormSubstance}, DefaultSettings(), answeredFinal())

	if content.Limits.Form != FormContent {
		t.Errorf("limits.form = %q, want content", content.Limits.Form)
	}
	if content.Perspective == nil || content.Perspective.Form != FormContent {
		t.Errorf("perspective = %+v, want it to echo the config's form", content.Perspective)
	}
	if !strings.Contains(RenderSummary(content, content.Now), " form=content\n") {
		t.Errorf("the summary of a content run does not state the form on its limits line:\n%s", RenderSummary(content, content.Now))
	}

	for name, record := range map[string]Record{"absent": substance, "explicit substance": explicit} {
		encoded, err := json.Marshal(record.Limits)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(encoded), `"form"`) {
			t.Errorf("%s: the limits of a substance run carry a form: %s", name, encoded)
		}
		if strings.Contains(RenderSummary(record, record.Now), "form=") {
			t.Errorf("%s: the summary of a substance run states a form", name)
		}
		if len(record.Fills) != 0 {
			t.Errorf("%s: unexpected fills %+v", name, record.Fills)
		}
		for _, row := range everyDisposition(record) {
			if row.Form != FormSubstance {
				t.Errorf("%s: row %d is %q, want substance", name, row.ID, row.Form)
			}
		}
	}
	if substance.Block != explicit.Block {
		t.Errorf("an explicit substance form changed the block:\n%s\nversus\n%s", explicit.Block, substance.Block)
	}
}
