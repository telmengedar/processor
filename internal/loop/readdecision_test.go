package loop

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func structuredAnswer(text string) JudgeResult {
	raw, _ := json.Marshal(map[string]any{"action": "answer", "text": text, "query": nil, "id": nil})
	return JudgeResult{Answer: string(raw), RawReason: "stop"}
}
func structuredRead(id int64) JudgeResult {
	raw, _ := json.Marshal(map[string]any{"action": "read", "text": "", "query": nil, "id": id})
	return JudgeResult{Answer: string(raw), RawReason: "stop"}
}
func structuredRecall(query string) JudgeResult {
	raw, _ := json.Marshal(map[string]any{"action": "recall", "text": "", "query": query, "id": nil})
	return JudgeResult{Answer: string(raw), RawReason: "stop"}
}
func structuredOptions() RunOptions {
	return RunOptions{Input: "what is the pickup code?", Subject: 42, Settings: DefaultSettings(), View: &View{Config: 7, ResponseMode: ResponseStructuredRead, Form: FormContent, Identity: "verbatim policy"}}
}

func TestStructuredDecisionsRejectAmbiguousMalformedOrDisallowedActions(t *testing.T) {
	t.Parallel()
	allowed := []string{"answer", "recall", "read"}
	for _, raw := range []string{
		`null`, `[]`, `{}`, `{"action":"read","text":"","query":null}`, `{"action":"read","text":"","query":null,"id":9,"extra":0}`,
		`{"action":"answer","action":"read","text":"","query":null,"id":9}`, `{"action":"read","text":"","query":null,"id":9,"id":10}`,
		`{"action":"read","text":"","query":null,"id":1.0}`, `{"action":"read","text":"","query":null,"id":9223372036854775808}`, `{"action":"read","text":"","query":null,"id":0}`,
		`{"action":"read","text":"","query":null,"id":null}`, `{"action":"read","text":null,"query":null,"id":9}`, `{"action":"read","text":"promise","query":null,"id":9}`,
		`{"action":"answer","text":" ","query":null,"id":null}`, `{"action":"answer","text":"done","query":"x","id":null}`,
		`{"action":"recall","text":"","query":" ","id":null}`, `{"action":"note","text":"done","query":null,"id":null}`,
		"```json\n{\"action\":\"answer\",\"text\":\"done\",\"query\":null,\"id\":null}\n```",
		`{"action":"answer","text":"done","query":null,"id":null} trailing`,
		`{"action":"answer","text":"done","query":null,"id":null} {}`,
		"{\"action\":\"answer\",\"text\":\"\xff\",\"query\":null,\"id\":null}",
	} {
		if _, err := decodeReadDecision(JudgeResult{Answer: raw}, allowed); err == nil {
			t.Errorf("accepted invalid decision %q", raw)
		}
	}
	if _, err := decodeReadDecision(structuredRead(9), []string{"answer"}); err == nil {
		t.Fatal("reserved answer-only call accepted read")
	}
	for _, result := range []JudgeResult{{Answer: structuredAnswer("done").Answer, UnofferedToolCalls: 1}, {Answer: structuredAnswer("done").Answer, ToolSource: ToolSourceNative}} {
		if _, err := decodeReadDecision(result, allowed); err == nil {
			t.Fatal("native call alongside JSON was accepted")
		}
	}
	if _, err := decodeReadDecision(structuredRecall(strings.Repeat("x", 257)), allowed); err == nil {
		t.Fatal("query beyond the existing phrase domain accepted")
	}
	if got, err := decodeReadDecision(structuredRecall(strings.Repeat("x", 256)), allowed); err != nil || len(got.RecallQuery) != 256 {
		t.Fatalf("valid phrase boundary rejected: %+v %v", got, err)
	}
}

func TestStructuredReadActuallyFetchesAbsorbsAndAnswersWithTheNewFact(t *testing.T) {
	t.Parallel()
	graph := graphHoldingNodes(readableNode(71, "the pickup code is COPPER-7"))
	graph.nodes[42] = nodeResponse{Anchor: readableNode(42, "pickup arrangements are in node 71"), Found: true}
	model := &fakeModel{results: []JudgeResult{structuredRead(71), structuredAnswer("COPPER-7")}}
	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), structuredOptions())
	if err != nil {
		t.Fatal(err)
	}
	if record.Answer != "COPPER-7" || record.ModelCalls != 2 || len(model.calls) != 2 {
		t.Fatalf("answer/calls: %+v", record)
	}
	if !slices.Equal(graph.nodeCalls, []int64{42, 71}) {
		t.Fatalf("actual addressed node calls=%v", graph.nodeCalls)
	}
	if strings.Contains(model.calls[0].Block, "COPPER-7") || !strings.Contains(model.calls[1].Block, "COPPER-7") {
		t.Fatal("fact did not cross the actual read/working-memory boundary")
	}
	for i, call := range model.calls {
		if len(call.Offered) != 0 || !slices.Equal(call.StructuredAllowedActions, []string{"answer", "recall", "read"}) {
			t.Fatalf("call %d native/structured offers=%v/%v", i, call.Offered, call.StructuredAllowedActions)
		}
		if !strings.HasPrefix(call.System, "verbatim policy\n\n") {
			t.Fatal("caller identity was rewritten")
		}
	}
	if len(record.Offered) != 2 || len(record.Offered[0]) != 0 || len(record.StructuredAllowedActions) != 2 {
		t.Fatal("record misstates wire/native offers")
	}
	round := onlyRound(t, record)
	if round.Tool != ToolReadNode || round.NodeID != 71 || round.Source != ToolSourceStructured || len(round.Results) != 1 {
		t.Fatalf("read provenance/receipt=%+v", round)
	}
}

func TestStructuredSuppliedAnswersAndExplicitPromisesNeverTriggerLookupRecovery(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"COPPER-7", "I'll check node 71 for you.", "recall: pickup code", "71"} {
		graph := graphHoldingNodes(readableNode(71, "COPPER-7"))
		graph.nodes[42] = nodeResponse{Anchor: readableNode(42, "COPPER-7 already supplied"), Found: true}
		model := &fakeModel{results: []JudgeResult{structuredAnswer(text)}}
		record, _, err := NewTurn(graph, model, nil, fixedSystem("system"), "model", testLogger()).RunWithOptions(context.Background(), structuredOptions())
		if err != nil {
			t.Fatal(err)
		}
		if len(graph.nodeCalls) != 1 || len(graph.recallCalls) != 1 || len(record.ToolCalls) != 0 || record.ModelCalls != 1 {
			t.Fatalf("explicit answer %q became a tool dispatch: %+v", text, record.ToolCalls)
		}
		if record.Answer != text || strings.Contains(record.Answer, `"action"`) {
			t.Fatalf("decoded answer=%q", record.Answer)
		}
	}
}

func TestInvalidStructuredResponsesFailBeforeAnySupplementaryGraphSideEffect(t *testing.T) {
	t.Parallel()
	for _, result := range []JudgeResult{{Answer: `{"action":"read","text":"","query":null,"id":71,"extra":true}`}, {Answer: `read_node(71)`}, {Answer: structuredRead(71).Answer, UnofferedToolCalls: 1}} {
		graph := graphHoldingNodes(readableNode(71, "new fact"))
		model := &fakeModel{results: []JudgeResult{result}}
		_, _, err := NewTurn(graph, model, nil, fixedSystem("system"), "model", testLogger()).RunWithOptions(context.Background(), structuredOptions())
		if !errors.Is(err, ErrModelUnavailable) || len(graph.nodeCalls) != 1 || len(graph.recallCalls) != 1 || len(model.calls) != 1 {
			t.Fatalf("invalid decision escaped model-error path: %v nodes%v recalls%d calls%d", err, graph.nodeCalls, len(graph.recallCalls), len(model.calls))
		}
	}
}

func TestStructuredReservedCallsAreAnswerOnlyAndInvalidReservedResponseKeepsFailureAccounting(t *testing.T) {
	t.Parallel()
	for _, last := range []JudgeResult{structuredAnswer("no code known"), structuredRead(71)} {
		graph := graphHoldingNodes()
		model := &fakeModel{results: []JudgeResult{structuredRead(999), last}}
		record, _, err := NewTurn(graph, model, nil, fixedSystem("system"), "model", testLogger()).RunWithOptions(context.Background(), structuredOptions())
		if err != nil {
			t.Fatal(err)
		}
		if len(model.calls) != 2 || !slices.Equal(model.calls[1].StructuredAllowedActions, []string{"answer"}) || len(model.calls[1].Offered) != 0 {
			t.Fatalf("reserved capability escaped: %+v", model.calls)
		}
		if len(graph.nodeCalls) != 2 || record.ModelCalls != 2 {
			t.Fatal("reserved response caused another read or lost accounting")
		}
		if last.Answer == structuredRead(71).Answer && (record.ReservedCall.State != ReservedCallFailed || record.Answer != "") {
			t.Fatalf("invalid reserved response not recorded as failed: %+v", record.ReservedCall)
		}
	}
}

func TestStructuredCallCapAndMemoryChargeKeepTheExistingBounds(t *testing.T) {
	t.Parallel()
	graph := graphHoldingNodes()
	model := &fakeModel{results: []JudgeResult{structuredAnswer("final")}}
	turn := NewTurn(graph, model, nil, fixedSystem("native and structured builder"), "model", testLogger())
	options := structuredOptions()
	options.Settings.MaxModelCalls = 1
	record, _, err := turn.RunWithOptions(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if !record.CapReached || len(model.calls) != 1 || !slices.Equal(model.calls[0].StructuredAllowedActions, []string{"answer"}) {
		t.Fatal("call cap did not reserve the same final call")
	}
	before := options.Settings.PromptCeiling()
	charge := turn.structuredReadCharge(false, false, FormContent)
	if charge < len(StructuredReadSchema([]string{"answer", "recall", "read"})) || before != 80000 {
		t.Fatalf("structured overhead uncharged or default ceiling moved: %d / %d", charge, before)
	}
}

func TestStructuredRecallClosureWithdrawsRecallButKeepsAddressedReads(t *testing.T) {
	t.Parallel()
	graph := graphReturningTheSameRowsToEveryRecall()
	model := &fakeModel{results: []JudgeResult{structuredRecall("pickup arrangements"), structuredRecall("pickup arrangements yesterday"), structuredRecall("pickup arrangements today"), structuredAnswer("unknown")}}
	record, _, err := NewTurn(graph, model, nil, fixedSystem("system"), "model", testLogger()).RunWithOptions(context.Background(), structuredOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(yieldsOf(record), []int{repeatedRowCount, 0, 0}) || dispatchedRecalls(graph) != 3 || len(model.calls) != 4 {
		t.Fatalf("closure scenario did not occur: closed=%v recalls=%d calls=%d", record.RecallClosed, dispatchedRecalls(graph), len(model.calls))
	}
	if !slices.Equal(model.calls[3].StructuredAllowedActions, []string{"answer", "read"}) {
		t.Fatalf("closed recall capabilities=%v", model.calls[3].StructuredAllowedActions)
	}
	if strings.Contains(model.calls[3].Clause, "choose the recall action") {
		t.Fatal("closed recall still requested by framework")
	}
}
