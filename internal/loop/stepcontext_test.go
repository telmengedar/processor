package loop

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func offeringSystem(offered []string, conversation, silence bool, _ Form, _ ResponseMode) string {
	text := "offers:" + strings.Join(offered, ",")
	if conversation {
		text += ";conversation"
	}
	if silence {
		text += ";silence"
	}
	return text
}

func turnComposing(graph GraphPort, model ModelPort, files FilePort) *Turn {
	return NewTurn(graph, model, files, offeringSystem, "test-model", testLogger())
}

func weakRowOf(id int64, body string) Candidate {
	return Candidate{ID: id, Type: "documentation", Name: "Row", Similarity: 0.3, Substance: body}
}

func rowOf(id int64, body string) Candidate {
	return Candidate{ID: id, Type: "documentation", Name: "Row", Similarity: 0.9, Content: body, Substance: body}
}

func TestTheAnsweringCallOffersNoToolAndItsSystemTextNamesNone(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap()}
	turn := turnComposing(graphYieldingNewRowsToEveryRecall(), model, nil)

	if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != MaxModelCalls {
		t.Fatalf("the model was called %d times, want %d", len(model.calls), MaxModelCalls)
	}

	for i, call := range model.calls[:MaxModelCalls-1] {
		if !slices.Equal(call.Offered, []string{"recall", "readNode"}) || call.System != "offers:recall,readNode" {
			t.Errorf("call %d offers %v with system text %q, want recall and readNode and a system text composed from exactly those", i+1, call.Offered, call.System)
		}
	}

	answering := model.calls[MaxModelCalls-1]
	if len(answering.Offered) != 0 {
		t.Errorf("the answering call offers %v, want nothing", answering.Offered)
	}
	if answering.System != "offers:" {
		t.Errorf("the answering call's system text is %q, want the one composed from an empty tool set: a sentence about a tool the call withholds invites the model to emit one", answering.System)
	}
}

func TestWriteFileIsNotOfferedWithoutAWorkingDirectory(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		files FilePort
		want  []string
	}{
		{"without a working directory", nil, []string{"recall", "readNode"}},
		{"with a working directory", &fakeFiles{dir: "/runs/run-1"}, []string{"recall", "readNode", "writeFile"}},
	} {
		model := &fakeModel{results: []JudgeResult{answered("done")}}
		turn := turnComposing(baseGraph(), model, tc.files)

		if _, _, err := turn.Run(context.Background(), "hello", 42); err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}
		if got := model.calls[0].Offered; !slices.Equal(got, tc.want) {
			t.Errorf("%s: the first call offers %v, want %v", tc.name, got, tc.want)
		}
		if got, want := model.calls[0].System, "offers:"+strings.Join(tc.want, ","); got != want {
			t.Errorf("%s: the first call's system text is %q, want %q", tc.name, got, want)
		}
	}
}

func TestARoundsFoundRowsEnterTheNextCallsMemoryOnce(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{},
		{Candidates: []Candidate{rowOf(91, "first body")}},
		{Candidates: []Candidate{rowOf(91, "first body")}},
	}
	model := &fakeModel{results: []JudgeResult{recallFor("one"), recallFor("two"), answered("done")}}

	if _, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 3 {
		t.Fatalf("the model was called %d times, want 3", len(model.calls))
	}

	for i, want := range [][]int64{nil, {91}, {91}} {
		if got := candidateIDsIn(model.calls[i].Block); !slices.Equal(got, want) {
			t.Errorf("call %d carries candidates %v, want %v: a repeated round adds nothing to what the model holds", i+1, got, want)
		}
	}
}

func TestNothingButFoundRowsOfAToolRoundReachesTheNextCall(t *testing.T) {
	t.Parallel()

	const cause = "cause-unique-503"

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{},
		{Candidates: []Candidate{rowOf(91, "found-body-unique")}},
		{Err: errors.New(cause)},
	}
	files := &fakeFiles{dir: "/runs/run-1"}
	model := &fakeModel{results: []JudgeResult{
		recallFor("first-query-unique"),
		wantsWriteOf("notes-unique.md", "written-content-unique"),
		recallFor("second-query-unique"),
		answered("done"),
	}}

	if _, _, err := turnComposing(graph, model, files).Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 4 {
		t.Fatalf("the model was called %d times, want 4", len(model.calls))
	}

	last := model.calls[3]
	if !strings.Contains(last.Block, "found-body-unique") {
		t.Fatalf("test setup error: the last call's block does not carry the row the first round found:\n%s", last.Block)
	}
	for _, leaked := range []string{"first-query-unique", "second-query-unique", "notes-unique.md", "written-content-unique", cause, "error:", "wrote ", "write_file", "tool_calls"} {
		for name, surface := range map[string]string{"block": last.Block, "system text": last.System, "input": last.Input} {
			if strings.Contains(surface, leaked) {
				t.Errorf("the last call's %s carries %q: only what a round found is memory; the model's own queries, writes and the failures it met stay on the record:\n%s", name, leaked, surface)
			}
		}
	}
	if last.Input != "hello" {
		t.Errorf("the last call's input is %q, want the request unchanged", last.Input)
	}
}

func TestTheWorkingMemoryNeverExceedsTheJudgementPromptCeiling(t *testing.T) {
	t.Parallel()

	const (
		ceiling    = 80_000
		anchorBody = "anchor body"
	)

	body := func(n int) string { return strings.Repeat("#", n) }

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{rowOf(11, body(14_000)), rowOf(12, body(14_000)), rowOf(13, body(14_000)), rowOf(14, body(14_000))}},
		{Candidates: []Candidate{rowOf(21, body(15_000)), rowOf(22, body(15_000)), rowOf(23, body(15_000))}},
		{Candidates: []Candidate{rowOf(31, body(15_000)), rowOf(32, body(15_000)), rowOf(33, body(15_000))}},
	}
	model := &fakeModel{results: []JudgeResult{recallFor("one"), recallFor("two"), answered("done")}}

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 3 {
		t.Fatalf("the model was called %d times, want 3", len(model.calls))
	}

	for i, call := range model.calls {
		if held := len(anchorBody) + strings.Count(call.Block, "#"); held > ceiling {
			t.Errorf("call %d carries %d content bytes, over the %d the judgement prompt is declared to hold", i+1, held, ceiling)
		}
	}
	if held := strings.Count(model.calls[2].Block, "#"); held != 71_000 {
		t.Errorf("the last call carries %d bytes of retrieved content, want 71000: the block's 56000 plus the one 15000-byte row the first round had room for, and nothing from the second", held)
	}

	second := record.ToolCalls[1]
	if len(second.Results) != 3 {
		t.Fatalf("the second round recorded %d rows, want the 3 it returned", len(second.Results))
	}
	for _, row := range second.Results {
		if row.Included || row.CutReason != "byte budget exceeded" {
			t.Errorf("row %d of the second round was Included=%v with cut reason %q, want it cut as %q: the memory had 8989 bytes of room left", row.ID, row.Included, row.CutReason, "byte budget exceeded")
		}
	}
}

func TestNoCallBlockTellsTheModelToLookFromAnotherAngle(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		rows []Candidate
	}{
		{"an empty memory", nil},
		{"a thin memory", []Candidate{weakRowOf(7, "a")}},
	} {
		graph := baseGraph()
		graph.recallQueue = []recallResponse{{Candidates: tc.rows}}
		graph.candidates = nil
		model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}

		record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}
		if !record.RecallClosed || len(model.calls) < 3 {
			t.Fatalf("%s: test setup error: %d calls, recall closed %v, want recall closed after at least 3 calls", tc.name, len(model.calls), record.RecallClosed)
		}

		for i, call := range model.calls {
			for _, former := range []string{"asking the wrong question", "knowledge is still thin", "different angle"} {
				if strings.Contains(call.Block, former) {
					t.Errorf("%s: call %d carries %q in its block, but the block states what was found and gives no advice:\n%s", tc.name, i+1, former, call.Block)
				}
			}
		}
	}
}

func TestTheFirstCallCarriesTheBlockTheRecordKeeps(t *testing.T) {
	t.Parallel()

	oversized := rowOf(5, strings.Repeat("#", AssemblyByteBudget+1))
	oversized.Similarity = 0.95

	for _, tc := range []struct {
		name string
		rows []Candidate
		cut  bool
	}{
		{"admitted rows in descending id order", []Candidate{rowOf(7, "a"), rowOf(3, "b")}, false},
		{"an oversized cut row ranked above an admitted one", []Candidate{oversized, rowOf(7, "the admitted body")}, true},
		{"every row cut as oversized", []Candidate{oversized}, true},
		{"no row at all", nil, false},
	} {
		graph := baseGraph()
		graph.recallQueue = []recallResponse{{Candidates: tc.rows}}
		model := &fakeModel{results: []JudgeResult{answered("done")}}

		record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}

		if model.calls[0].Block != record.Block {
			t.Errorf("%s: the first call's block differs from the block the record keeps:\ncall:   %q\nrecord: %q", tc.name, model.calls[0].Block, record.Block)
		}
		if tc.cut && !slices.ContainsFunc(record.Candidates, func(d Disposition) bool { return d.ID == 5 && !d.Included && d.CutReason != "" }) {
			t.Errorf("%s: row 5 was not recorded as cut, so the case does not exercise a cut row: %+v", tc.name, record.Candidates)
		}
	}
}

func TestAnAddressedReadIsBoundedByTheWorkingMemoryCeilingToo(t *testing.T) {
	t.Parallel()

	body := func(n int) string { return strings.Repeat("#", n) }

	graph := graphHoldingNodes(readableNode(71, body(15_000)))
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{rowOf(11, body(14_000)), rowOf(12, body(14_000)), rowOf(13, body(14_000)), rowOf(14, body(14_000))}},
		{Candidates: []Candidate{rowOf(21, body(15_000))}},
	}
	model := &fakeModel{results: []JudgeResult{recallFor("one"), wantsRead(71), answered("done")}}

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	read := record.ToolCalls[1]
	if want := "that part's substance is 15000 bytes and one read carries at most 8989, so it cannot be shown"; read.Error != want {
		t.Fatalf("the read of a 15000-byte part with 8989 bytes of memory left was recorded with error %q, want %q", read.Error, want)
	}
	if held := strings.Count(model.calls[2].Block, "#"); held != 71_000 {
		t.Fatalf("the last call carries %d bytes of retrieved content, want 71000: the refused read must add nothing", held)
	}
}

func nearTheCeiling(secondRound []Candidate) *fakeGraph {
	body := func(n int) string { return strings.Repeat("#", n) }

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{rowOf(11, body(8_000)), rowOf(12, body(16_000)), rowOf(13, body(16_000)), rowOf(14, body(16_000))}},
		{Candidates: []Candidate{rowOf(21, body(15_000))}},
		{Candidates: secondRound},
	}
	return graph
}

func TestAHeldRowCostsNoRoomToTheRowsBesideIt(t *testing.T) {
	t.Parallel()

	body := func(n int) string { return strings.Repeat("#", n) }

	graph := nearTheCeiling([]Candidate{rowOf(11, body(8_000)), rowOf(52, body(2_000))})
	model := &fakeModel{results: []JudgeResult{recallFor("one"), recallFor("two"), answered("done")}}

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	second := record.ToolCalls[1]
	if len(second.Results) != 2 || second.Results[0].ID != 11 || second.Results[1].ID != 52 {
		t.Fatalf("test setup error: the second round recorded %+v, want rows 11 then 52", second.Results)
	}
	if !second.Results[1].Included || second.Results[1].CutReason != "" {
		t.Errorf("row 52 (2000 bytes) was Included=%v with cut reason %q, want it admitted: the memory had 8989 bytes of room and the 8000-byte row beside it was already held", second.Results[1].Included, second.Results[1].CutReason)
	}

	last := model.calls[2].Block
	if ids := candidateIDsIn(last); !slices.Equal(ids, []int64{11, 12, 13, 14, 21, 52}) {
		t.Errorf("the last call carries candidates %v, want [11 12 13 14 21 52]", ids)
	}
	if held := strings.Count(last, "#"); held != 73_000 {
		t.Errorf("the last call carries %d bytes of retrieved content, want 73000: 71000 held plus the 2000-byte row", held)
	}
}

func TestAReadOfAHeldRowWhoseBodyChangedIsCreditedTheBytesItReplaces(t *testing.T) {
	t.Parallel()

	body := func(n int) string { return strings.Repeat("#", n) }

	graph := graphHoldingNodes(readableNode(11, strings.Repeat("x", 9_000)))
	graph.recallQueue = []recallResponse{
		{Candidates: []Candidate{rowOf(11, body(14_000)), rowOf(12, body(14_000)), rowOf(13, body(14_000)), rowOf(14, body(14_000))}},
		{Candidates: []Candidate{rowOf(21, body(15_000))}},
	}
	model := &fakeModel{results: []JudgeResult{recallFor("one"), wantsRead(11), answered("done")}}

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", readSubject)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if read := record.ToolCalls[1]; read.Error != "" {
		t.Fatalf("the read of row 11 (9000 bytes, replacing 14000 held) was refused with %q: the memory had 8989 bytes free plus the 14000 the replacement frees", read.Error)
	}
	last := model.calls[2].Block
	if got := strings.Count(last, "x"); got != 9_000 {
		t.Errorf("the last call carries %d bytes of the re-read body, want 9000", got)
	}
	if got := strings.Count(last, "#"); got != 57_000 {
		t.Errorf("the last call carries %d bytes of the old content, want 57000: three 14000-byte rows and row 21, and none of the replaced body", got)
	}
	if ids := candidateIDsIn(last); !slices.Equal(ids, []int64{11, 12, 13, 14, 21}) {
		t.Errorf("the last call carries candidates %v, want each of [11 12 13 14 21] once", ids)
	}
}

func TestALaterReturnOfAHeldRowWithAChangedBodyReplacesIt(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.recallQueue = []recallResponse{
		{},
		{Candidates: []Candidate{rowOf(91, "the-old-body")}},
		{Candidates: []Candidate{rowOf(91, "the-new-body")}},
	}
	model := &fakeModel{results: []JudgeResult{recallFor("one"), recallFor("two"), answered("done")}}

	if _, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42); err != nil {
		t.Fatalf("Run: %v", err)
	}

	last := model.calls[2].Block
	if strings.Contains(last, "the-old-body") || strings.Count(last, "the-new-body") != 1 {
		t.Errorf("after a node changed mid-turn the last call's block is not the new body once:\n%s", last)
	}
	if ids := candidateIDsIn(last); !slices.Equal(ids, []int64{91}) {
		t.Errorf("the last call carries candidates %v, want row 91 once", ids)
	}
}

func TestAHeldIdReturnedWithAGrowingBodyIsStillChargedAndBoundedByTheCeiling(t *testing.T) {
	t.Parallel()

	body := func(n int) string { return strings.Repeat("#", n) }

	graph := nearTheCeiling([]Candidate{rowOf(11, body(14_000))})
	model := &fakeModel{results: []JudgeResult{recallFor("one"), recallFor("two"), answered("done")}}

	record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	row := record.ToolCalls[1].Results[0]
	if row.ID != 11 || row.Included || row.CutReason != "byte budget exceeded" {
		t.Errorf("row 11 returned with 14000 bytes against 8000 held was recorded as %+v, want it cut as %q: its body is not what is held, so it is charged, and 8989 bytes of room cannot take it", row, "byte budget exceeded")
	}
	if held := strings.Count(model.calls[2].Block, "#"); held != 71_000 {
		t.Errorf("the last call carries %d bytes of retrieved content, want 71000: the grown body must not have entered the memory past the ceiling", held)
	}
}

func TestAHeldRowIsNeverRecordedAsCutWhileItIsInTheNextBlock(t *testing.T) {
	t.Parallel()

	body := func(n int) string { return strings.Repeat("#", n) }

	lowSimilarity := rowOf(21, body(15_000))
	lowSimilarity.Similarity = 0.3

	for _, tc := range []struct {
		name  string
		again []Candidate
	}{
		{"for want of bytes", []Candidate{rowOf(21, body(15_000))}},
		{"at a lower similarity", []Candidate{lowSimilarity}},
	} {
		graph := nearTheCeiling(tc.again)
		model := &fakeModel{results: []JudgeResult{recallFor("one"), recallFor("two"), answered("done")}}

		record, _, err := turnComposing(graph, model, nil).Run(context.Background(), "hello", 42)
		if err != nil {
			t.Fatalf("%s: Run: %v", tc.name, err)
		}

		row := record.ToolCalls[1].Results[0]
		if row.ID != 21 || !row.Included || row.CutReason != "" || !row.Held {
			t.Errorf("%s: row 21 was recorded as %+v, want Included and Held with no cut reason: it is already held and is in the next block, so the record must not claim it was cut", tc.name, row)
		}
		if ids := candidateIDsIn(model.calls[2].Block); !slices.Contains(ids, 21) {
			t.Fatalf("%s: test setup error: row 21 is not in the last call's block %v", tc.name, ids)
		}
		if record.ToolCalls[1].Yield != 0 {
			t.Errorf("%s: the repeated round's yield is %d, want 0: it put nothing in front of the model that was not already there", tc.name, record.ToolCalls[1].Yield)
		}
	}
}
