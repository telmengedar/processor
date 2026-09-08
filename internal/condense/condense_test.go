package condense

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

const proseContentType = "text/markdown; charset=utf-8"

var fixedClock = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }

type graphCall struct {
	Op        string
	Node      int64
	Substance string
}

type fakeGraph struct {
	nodes      map[int64]Node
	liveBody   map[int64]string
	readErr    map[int64]error
	rereadErr  map[int64]error
	writeErr   map[int64]error
	calls      []graphCall
	absentNode int64
}

func newFakeGraph(nodes ...Node) *fakeGraph {
	g := &fakeGraph{nodes: make(map[int64]Node, len(nodes))}
	for _, n := range nodes {
		g.nodes[n.ID] = n
	}
	return g
}

func (g *fakeGraph) NodeWithSubstance(_ context.Context, id int64) (Node, bool, error) {
	g.calls = append(g.calls, graphCall{Op: "read", Node: id})
	if err := g.readErr[id]; err != nil {
		return Node{}, false, err
	}
	node, ok := g.nodes[id]
	if !ok || id == g.absentNode {
		return Node{}, false, nil
	}
	return node, true, nil
}

func (g *fakeGraph) Content(_ context.Context, id int64) (string, bool, error) {
	g.calls = append(g.calls, graphCall{Op: "reread", Node: id})
	if err := g.rereadErr[id]; err != nil {
		return "", false, err
	}
	if body, ok := g.liveBody[id]; ok {
		return body, true, nil
	}
	node, ok := g.nodes[id]
	if !ok {
		return "", false, nil
	}
	return node.Content, true, nil
}

func (g *fakeGraph) SetSubstance(_ context.Context, id int64, substance string) error {
	g.calls = append(g.calls, graphCall{Op: "write", Node: id, Substance: substance})
	return g.writeErr[id]
}

func (g *fakeGraph) ops() []string {
	ops := make([]string, 0, len(g.calls))
	for _, c := range g.calls {
		ops = append(ops, c.Op)
	}
	return ops
}

func (g *fakeGraph) written(id int64) (string, bool) {
	for _, c := range g.calls {
		if c.Op == "write" && c.Node == id {
			return c.Substance, true
		}
	}
	return "", false
}

type fakeModel struct {
	text         string
	finishReason string
	model        string
	err          error
	prompts      []string
	maxTokens    []int
}

func (m *fakeModel) Condense(_ context.Context, prompt string, maxOutputTokens int) (Completion, error) {
	m.prompts = append(m.prompts, prompt)
	m.maxTokens = append(m.maxTokens, maxOutputTokens)
	if m.err != nil {
		return Completion{}, m.err
	}
	reason := m.finishReason
	if reason == "" {
		reason = finishReasonStop
	}
	name := m.model
	if name == "" {
		name = "ai/test"
	}
	return Completion{Text: m.text, FinishReason: reason, Model: name}, nil
}

func proseNode(id int64, content string) Node {
	return Node{ID: id, Type: "documentation", Name: "a node", ContentType: proseContentType, Content: content}
}

func runPass(t *testing.T, graph GraphPort, model ModelPort, opts Options, targets ...Target) Result {
	t.Helper()
	return Run(context.Background(), graph, model, targets, opts, fixedClock)
}

func onlySkip(t *testing.T, result Result) Skip {
	t.Helper()
	if len(result.Skipped) != 1 {
		t.Fatalf("want exactly one skip, got %d: %+v", len(result.Skipped), result.Skipped)
	}
	return result.Skipped[0]
}

func TestASelfProducedNodeIsSkippedBeforeAnyModelCallIsMade(t *testing.T) {
	node := proseNode(1, strings.Repeat("a", 1000))
	node.SelfProduced = true
	graph := newFakeGraph(node)
	model := &fakeModel{text: strings.Repeat("b", 200)}

	result := runPass(t, graph, model, Options{}, Target{Node: 1})

	if got := onlySkip(t, result).Reason; got != skipSelfProduced {
		t.Fatalf("want skip reason %q, got %q", skipSelfProduced, got)
	}
	if len(model.prompts) != 0 {
		t.Fatalf("a self-produced node must not reach the model, but %d calls were made", len(model.prompts))
	}
}

func TestANodeWhoseContentIsAbsentIsSkippedAndNeverWritten(t *testing.T) {
	graph := newFakeGraph(proseNode(1, "   \n  "))
	model := &fakeModel{text: "short"}

	result := runPass(t, graph, model, Options{}, Target{Node: 1})

	if got := onlySkip(t, result).Reason; got != skipContentAbsent {
		t.Fatalf("want skip reason %q, got %q", skipContentAbsent, got)
	}
	if _, wrote := graph.written(1); wrote {
		t.Fatalf("a node with no content must never be written to")
	}
}

func TestANodeThatAlreadyCarriesSubstanceIsSkippedUnlessForceIsGiven(t *testing.T) {
	node := proseNode(1, strings.Repeat("a", 1000))
	node.Substance = "an existing condensation"

	graph := newFakeGraph(node)
	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})
	if got := onlySkip(t, result).Reason; got != skipSubstancePresent {
		t.Fatalf("want skip reason %q, got %q", skipSubstancePresent, got)
	}

	forcedGraph := newFakeGraph(node)
	forced := runPass(t, forcedGraph, &fakeModel{text: strings.Repeat("b", 200)}, Options{Force: true}, Target{Node: 1})
	if len(forced.Provenance) != 1 {
		t.Fatalf("force must re-derive over an existing substance, got %d provenance rows", len(forced.Provenance))
	}
}

func TestASubstanceOfWhitespaceAloneCountsAsAbsentAndIsRegenerated(t *testing.T) {
	node := proseNode(1, strings.Repeat("a", 1000))
	node.Substance = "   \n\t "

	graph := newFakeGraph(node)
	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})

	if len(result.Provenance) != 1 {
		t.Fatalf("a whitespace-only substance must count as absent, got %d provenance rows and skips %+v", len(result.Provenance), result.Skipped)
	}
}

func TestANonProseContentTypeIsExcludedFromTheTargetSet(t *testing.T) {
	node := proseNode(1, strings.Repeat("a", 1000))
	node.ContentType = "application/json"

	result := runPass(t, newFakeGraph(node), &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})

	if got := onlySkip(t, result).Reason; got != skipNonProseContent {
		t.Fatalf("want skip reason %q, got %q", skipNonProseContent, got)
	}
}

func TestACondensationExactlyAsLongAsItsContentIsRejectedRatherThanWritten(t *testing.T) {
	content := strings.Repeat("a", 1000)
	graph := newFakeGraph(proseNode(1, content))
	model := &fakeModel{text: strings.Repeat("b", 1000)}

	result := runPass(t, graph, model, Options{}, Target{Node: 1})

	if got := onlySkip(t, result).Reason; got != skipNotShorter {
		t.Fatalf("want skip reason %q at exact equality, got %q", skipNotShorter, got)
	}
	if _, wrote := graph.written(1); wrote {
		t.Fatalf("a condensation that is not shorter must never be written")
	}
}

func TestACondensationOneByteShorterThanItsContentIsWritten(t *testing.T) {
	content := strings.Repeat("a", 1000)
	graph := newFakeGraph(proseNode(1, content))
	model := &fakeModel{text: strings.Repeat("b", 999)}

	result := runPass(t, graph, model, Options{}, Target{Node: 1})

	if len(result.Provenance) != 1 {
		t.Fatalf("want one provenance row, got %d with skips %+v", len(result.Provenance), result.Skipped)
	}
	if _, wrote := graph.written(1); !wrote {
		t.Fatalf("a condensation one byte shorter than its content must be written")
	}
}

func TestACondensationBelowTheFloorIsRejectedAndOneAtTheFloorIsKept(t *testing.T) {
	content := strings.Repeat("a", 1000)

	below := runPass(t, newFakeGraph(proseNode(1, content)), &fakeModel{text: strings.Repeat("b", 49)}, Options{}, Target{Node: 1})
	if got := onlySkip(t, below).Reason; got != skipBelowFloor {
		t.Fatalf("want skip reason %q at 49 of 1000 bytes, got %q", skipBelowFloor, got)
	}

	at := runPass(t, newFakeGraph(proseNode(1, content)), &fakeModel{text: strings.Repeat("b", 50)}, Options{}, Target{Node: 1})
	if len(at.Provenance) != 1 {
		t.Fatalf("50 of 1000 bytes is at the floor and must be kept, got skips %+v", at.Skipped)
	}
}

func TestATruncatedCondensationIsSkippedRatherThanStored(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))
	model := &fakeModel{text: strings.Repeat("b", 200), finishReason: "length"}

	result := runPass(t, graph, model, Options{}, Target{Node: 1})

	skip := onlySkip(t, result)
	if skip.Reason != skipTruncated {
		t.Fatalf("want skip reason %q, got %q", skipTruncated, skip.Reason)
	}
	if skip.Detail != "length" {
		t.Fatalf("want the endpoint's own finish reason recorded, got %q", skip.Detail)
	}
	if _, wrote := graph.written(1); wrote {
		t.Fatalf("a truncated condensation must never be written")
	}
}

func TestTheContentHashIsRereadImmediatelyBeforeTheWriteAndTheWriteIsSkippedWhenItMoved(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))
	graph.liveBody = map[int64]string{1: strings.Repeat("z", 1000)}

	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})

	if got := onlySkip(t, result).Reason; got != skipContentMoved {
		t.Fatalf("want skip reason %q, got %q", skipContentMoved, got)
	}
	if _, wrote := graph.written(1); wrote {
		t.Fatalf("a substance derived from superseded content must never be written")
	}
}

func TestTheRereadPrecedesTheWriteSoAContentChangeCannotLandBetweenThem(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))

	runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})

	ops := graph.ops()
	want := []string{"read", "reread", "write"}
	if len(ops) != len(want) {
		t.Fatalf("want graph operations %v, got %v", want, ops)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("want graph operations %v, got %v", want, ops)
		}
	}
}

func TestADryRunCondensesAndReportsWithoutTouchingTheGraph(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))

	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{DryRun: true}, Target{Node: 1})

	if len(result.Provenance) != 1 {
		t.Fatalf("a dry run must still report what it would write, got %d provenance rows", len(result.Provenance))
	}
	if result.Provenance[0].Written {
		t.Fatalf("a dry run must not report the substance as written")
	}
	for _, op := range graph.ops() {
		if op == "write" || op == "reread" {
			t.Fatalf("a dry run must issue no write and no compare-and-write reread, got %v", graph.ops())
		}
	}
}

func TestOneNodesModelFailureIsIsolatedAndTheRestOfThePassStillRuns(t *testing.T) {
	graph := newFakeGraph(
		proseNode(1, strings.Repeat("a", 1000)),
		proseNode(2, strings.Repeat("a", 1000)),
	)
	failing := &failFirstModel{text: strings.Repeat("b", 200)}

	result := Run(context.Background(), graph, failing, []Target{{Node: 1}, {Node: 2}}, Options{}, fixedClock)

	if len(result.Provenance) != 1 || result.Provenance[0].Node != 2 {
		t.Fatalf("the second node must still be condensed after the first failed, got %+v", result.Provenance)
	}
	if got := onlySkip(t, result).Reason; got != skipModelFailed {
		t.Fatalf("want skip reason %q, got %q", skipModelFailed, got)
	}
}

type failFirstModel struct {
	text  string
	calls int
}

func (m *failFirstModel) Condense(_ context.Context, _ string, _ int) (Completion, error) {
	m.calls++
	if m.calls == 1 {
		return Completion{}, errors.New("the endpoint refused the call")
	}
	return Completion{Text: m.text, FinishReason: finishReasonStop, Model: "ai/test"}, nil
}

func TestAWriteRejectionIsReportedAsAnOperationalFailureAndARuleSkipIsNot(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))
	graph.writeErr = map[int64]error{1: errors.New("the graph rejected the patch")}

	failed := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})
	if failed.OperationalFailures() != 1 {
		t.Fatalf("a rejected write is an operational failure, got %d", failed.OperationalFailures())
	}

	ruled := runPass(t, newFakeGraph(proseNode(2, strings.Repeat("a", 1000))), &fakeModel{text: strings.Repeat("b", 1000)}, Options{}, Target{Node: 2})
	if ruled.OperationalFailures() != 0 {
		t.Fatalf("a rule skip is not an operational failure, got %d", ruled.OperationalFailures())
	}
}

func TestTheProvenanceRowCarriesTheHashOfTheContentTheCondensationWasDerivedFrom(t *testing.T) {
	content := strings.Repeat("a", 1000)
	graph := newFakeGraph(proseNode(1, content))

	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})

	if result.Provenance[0].ContentHash != contentHash(content) {
		t.Fatalf("want the hash of the content, got %q", result.Provenance[0].ContentHash)
	}
	if result.Provenance[0].Ratio != 0.2 {
		t.Fatalf("want ratio 0.2 for 200 bytes of 1000, got %v", result.Provenance[0].Ratio)
	}
}

func TestTheRatioSummaryReportsTheDistributionOverEveryCondensationThePassGenerated(t *testing.T) {
	graph := newFakeGraph(
		proseNode(1, strings.Repeat("a", 1000)),
		proseNode(2, strings.Repeat("a", 1000)),
		proseNode(3, strings.Repeat("a", 1000)),
	)

	result := Run(context.Background(), graph, &varyingModel{sizes: []int{100, 300, 200}},
		[]Target{{Node: 1}, {Node: 2}, {Node: 3}}, Options{}, fixedClock)

	got := result.Ratio
	if got.Count != 3 || got.Min != 0.1 || got.Median != 0.2 || got.Max != 0.3 {
		t.Fatalf("want count 3, min 0.1, median 0.2, max 0.3, got %+v", got)
	}
	if math.Abs(got.Mean-0.2) > 1e-9 {
		t.Fatalf("want mean 0.2, got %v", got.Mean)
	}
}

type varyingModel struct {
	sizes []int
	calls int
}

func (m *varyingModel) Condense(_ context.Context, _ string, _ int) (Completion, error) {
	size := m.sizes[m.calls]
	m.calls++
	return Completion{Text: strings.Repeat("b", size), FinishReason: finishReasonStop, Model: "ai/test"}, nil
}

func TestTheAuditEntryPairsThePreRegisteredReasonWithTheSubstanceTheNodeNowCarries(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))
	why := "an answer lacking this would present X, when the truth is Y"

	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{},
		Target{Node: 1, Rows: []string{"r01"}, Why: []string{why}})

	if len(result.Audit) != 1 {
		t.Fatalf("want one audit entry, got %d", len(result.Audit))
	}
	entry := result.Audit[0]
	if entry.Why[0] != why {
		t.Fatalf("want the pre-registered reason carried verbatim, got %q", entry.Why[0])
	}
	if entry.Substance != strings.Repeat("b", 200) {
		t.Fatalf("want the generated substance in the audit entry, got %d bytes", len(entry.Substance))
	}
	if entry.Origin != originGenerated {
		t.Fatalf("want origin %q, got %q", originGenerated, entry.Origin)
	}
}

func TestAnAuditEntryIsStillProducedForARequiredNodeThatEndedWithNoSubstance(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))
	model := &fakeModel{text: strings.Repeat("b", 1000)}

	result := runPass(t, graph, model, Options{}, Target{Node: 1, Rows: []string{"r01"}, Why: []string{"a reason"}})

	if len(result.Audit) != 1 {
		t.Fatalf("want one audit entry even where nothing was written, got %d", len(result.Audit))
	}
	if result.Audit[0].Origin != originAbsent {
		t.Fatalf("want origin %q, got %q", originAbsent, result.Audit[0].Origin)
	}
}

func TestATargetCarryingNoPreRegisteredReasonProducesNoAuditEntry(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))

	result := runPass(t, graph, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})

	if len(result.Audit) != 0 {
		t.Fatalf("want no audit entry without a pre-registered reason, got %d", len(result.Audit))
	}
}

func TestTheOutputTokenCeilingHasAFloorAndOtherwiseTracksTheInputSize(t *testing.T) {
	if got := maxOutputTokens(strings.Repeat("a", 100)); got != minOutputTokens {
		t.Fatalf("want the floor %d for a small node, got %d", minOutputTokens, got)
	}
	if got := maxOutputTokens(strings.Repeat("a", 100000)); got <= minOutputTokens {
		t.Fatalf("want a ceiling above the floor for a large node, got %d", got)
	}
}

func TestAnEmptyContentTypeIsTreatedAsProseAndAnImageOneIsNot(t *testing.T) {
	if !isProse("") {
		t.Fatalf("an absent content type must not exclude a node from the target set")
	}
	if !isProse("TEXT/Markdown") {
		t.Fatalf("the content type comparison must be case-insensitive")
	}
	if isProse("image/png") {
		t.Fatalf("a non-prose content type must be excluded")
	}
}

func TestATruncatedCondensationIsAnOperationalFailureBecauseThePassExhaustedItsOwnOutputBudget(t *testing.T) {
	graph := newFakeGraph(proseNode(1, strings.Repeat("a", 1000)))
	model := &fakeModel{text: strings.Repeat("b", 200), finishReason: "length"}

	result := runPass(t, graph, model, Options{}, Target{Node: 1})

	if len(result.Provenance) != 0 {
		t.Fatalf("want nothing condensed, got %+v", result.Provenance)
	}
	if result.OperationalFailures() != 1 {
		t.Fatalf("a pass that condensed nothing must not report a clean run, got %d operational failures", result.OperationalFailures())
	}
}

func TestNoRuleThePassAppliesIsCountedAsAnOperationalFailure(t *testing.T) {
	content := strings.Repeat("a", 1000)

	selfProduced := proseNode(3, content)
	selfProduced.SelfProduced = true
	nonProse := proseNode(4, content)
	nonProse.ContentType = "application/json"
	carrying := proseNode(5, content)
	carrying.Substance = "an existing condensation"
	moved := newFakeGraph(proseNode(6, content))
	moved.liveBody = map[int64]string{6: strings.Repeat("z", 1000)}

	cases := []struct {
		reason string
		result Result
	}{
		{skipNodeAbsent, runPass(t, newFakeGraph(), &fakeModel{text: "unreached"}, Options{}, Target{Node: 1})},
		{skipContentAbsent, runPass(t, newFakeGraph(proseNode(2, "   \n  ")), &fakeModel{text: "unreached"}, Options{}, Target{Node: 2})},
		{skipSelfProduced, runPass(t, newFakeGraph(selfProduced), &fakeModel{text: "unreached"}, Options{}, Target{Node: 3})},
		{skipNonProseContent, runPass(t, newFakeGraph(nonProse), &fakeModel{text: "unreached"}, Options{}, Target{Node: 4})},
		{skipSubstancePresent, runPass(t, newFakeGraph(carrying), &fakeModel{text: "unreached"}, Options{}, Target{Node: 5})},
		{skipContentMoved, runPass(t, moved, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 6})},
		{skipEmptyCondensation, runPass(t, newFakeGraph(proseNode(7, content)), &fakeModel{text: ""}, Options{}, Target{Node: 7})},
		{skipNotShorter, runPass(t, newFakeGraph(proseNode(8, content)), &fakeModel{text: strings.Repeat("b", 1000)}, Options{}, Target{Node: 8})},
		{skipBelowFloor, runPass(t, newFakeGraph(proseNode(9, content)), &fakeModel{text: strings.Repeat("b", 49)}, Options{}, Target{Node: 9})},
		{skipPreamble, runPass(t, newFakeGraph(proseNode(10, content)), &fakeModel{text: "Here is the condensation you asked for, set out below."}, Options{}, Target{Node: 10})},
	}

	for _, c := range cases {
		if got := onlySkip(t, c.result).Reason; got != c.reason {
			t.Fatalf("want skip reason %q, got %q", c.reason, got)
		}
		if c.result.OperationalFailures() != 0 {
			t.Fatalf("%q is a rule the pass applied, not a failure of the pass, yet it counted %d", c.reason, c.result.OperationalFailures())
		}
	}
}

func TestEverySkipWhoseCauseIsThePassItselfIsCountedAsAnOperationalFailure(t *testing.T) {
	content := strings.Repeat("a", 1000)

	unreadable := newFakeGraph(proseNode(1, content))
	unreadable.readErr = map[int64]error{1: errors.New("the graph refused the read")}
	unwritable := newFakeGraph(proseNode(3, content))
	unwritable.writeErr = map[int64]error{3: errors.New("the graph rejected the patch")}

	cases := []struct {
		reason string
		result Result
	}{
		{skipReadFailed, runPass(t, unreadable, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 1})},
		{skipModelFailed, runPass(t, newFakeGraph(proseNode(2, content)), &fakeModel{err: errors.New("the endpoint refused the call")}, Options{}, Target{Node: 2})},
		{skipWriteFailed, runPass(t, unwritable, &fakeModel{text: strings.Repeat("b", 200)}, Options{}, Target{Node: 3})},
		{skipTruncated, runPass(t, newFakeGraph(proseNode(4, content)), &fakeModel{text: strings.Repeat("b", 200), finishReason: "length"}, Options{}, Target{Node: 4})},
	}

	for _, c := range cases {
		if got := onlySkip(t, c.result).Reason; got != c.reason {
			t.Fatalf("want skip reason %q, got %q", c.reason, got)
		}
		if c.result.OperationalFailures() != 1 {
			t.Fatalf("%q is the pass failing rather than a rule it applied, yet it counted %d", c.reason, c.result.OperationalFailures())
		}
	}
}

func TestTheOutputCeilingIsOneTokenPerInputTokenRatherThanHalfOfThem(t *testing.T) {
	if got := maxOutputTokens(strings.Repeat("a", 100000)); got != 23810 {
		t.Fatalf("want a ceiling of 23810 tokens for a 100000 byte node, got %d", got)
	}
	if got := maxOutputTokens(strings.Repeat("a", 100)); got != 2048 {
		t.Fatalf("want the floor of 2048 tokens for a 100 byte node, got %d", got)
	}
}

func TestTheCeilingHandedToTheModelIsTheOneTheNodesOwnSizeProduces(t *testing.T) {
	model := &fakeModel{text: strings.Repeat("b", 200)}

	runPass(t, newFakeGraph(proseNode(1, strings.Repeat("a", 100000))), model, Options{}, Target{Node: 1})

	if len(model.maxTokens) != 1 || model.maxTokens[0] != 23810 {
		t.Fatalf("want one call carrying a ceiling of 23810, got %v", model.maxTokens)
	}
}
