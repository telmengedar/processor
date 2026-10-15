package loop

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func distinctSettings() RunSettings {
	return RunSettings{
		CandidateLimit:          3,
		AssemblyByteBudget:      4_000,
		SupplementaryByteBudget: 900,
		MaxModelCalls:           2,
		JudgementBudget:         301,
		AnsweringBudget:         407,
		FillWait:                7 * time.Second,
		HistoryExchangeLimit:    2,
		HistorySafetyLimit:      1_500,
	}
}

func runWithSettings(t *testing.T, graph *fakeGraph, model *fakeModel, settings RunSettings, history []Exchange) Record {
	t.Helper()

	turn := NewTurn(graph, model, nil, fixedSystem("system"), "test-model", testLogger())
	record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, History: history, Settings: settings})
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	return record
}

func bulky(id int64, size int) Candidate {
	body := strings.Repeat("x", size)
	return Candidate{ID: id, Type: "documentation", Name: "Bulky", Similarity: 0.9, Content: body, Substance: body}
}

func TestDefaultSettingsNameEveryDialAtTheValueTheLoopShipsAndSetNoOutputLimit(t *testing.T) {
	t.Parallel()

	want := RunSettings{
		CandidateLimit:          6,
		AssemblyByteBudget:      60_000,
		SupplementaryByteBudget: 20_000,
		MaxModelCalls:           6,
		FillWait:                90 * time.Second,
		HistoryExchangeLimit:    4,
		HistorySafetyLimit:      8_000,
	}

	if got := DefaultSettings(); got != want {
		t.Fatalf("DefaultSettings() = %+v, want %+v", got, want)
	}
	if got := reflect.TypeOf(RunSettings{}).NumField(); got != 10 {
		t.Fatalf("RunSettings has %d fields, want the 10 this test names: a new dial needs a default pinned here", got)
	}
	if got := DefaultSettings().PromptCeiling(); got != JudgementPromptCeiling {
		t.Fatalf("default PromptCeiling = %d, want the deployment-wide JudgementPromptCeiling %d", got, JudgementPromptCeiling)
	}
}

func TestSettingsLimitsCarriesEveryDialToItsOwnRecordedField(t *testing.T) {
	t.Parallel()

	want := Limits{
		CandidateLimit:          3,
		AssemblyByteBudget:      4_000,
		SupplementaryByteBudget: 900,
		MaxModelCalls:           2,
		ThinKnowledgeSimilarity: ThinKnowledgeSimilarity,
		JudgementBudget:         301,
		AnsweringBudget:         407,
		FillWait:                7 * time.Second,
		HistoryExchangeLimit:    2,
		HistorySafetyLimit:      1_500,
	}

	if got := distinctSettings().Limits(); got != want {
		t.Fatalf("Limits() = %+v, want %+v", got, want)
	}
}

func TestARunRecordsTheLimitsOfTheSettingsItWasGiven(t *testing.T) {
	t.Parallel()

	record := runWithSettings(t, baseGraph(), &fakeModel{}, distinctSettings(), nil)

	if want := distinctSettings().Limits(); record.Limits != want {
		t.Fatalf("record.Limits = %+v, want the given settings' %+v", record.Limits, want)
	}
}

func TestADefaultRunThroughTheOptionsFormRecordsTheSameAsRunDoes(t *testing.T) {
	t.Parallel()

	viaRun, _, err := newTurnWithGraph(baseGraph()).Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	viaOptions := runWithSettings(t, baseGraph(), &fakeModel{}, DefaultSettings(), nil)

	if viaRun.Limits != viaOptions.Limits {
		t.Fatalf("Run recorded %+v, the options form recorded %+v", viaRun.Limits, viaOptions.Limits)
	}
}

func TestTheCandidateLimitOfTheSettingsReachesEveryRecallOfTheRun(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	model := &fakeModel{results: []JudgeResult{recallFor("more"), answered("done")}}

	settings := DefaultSettings()
	settings.CandidateLimit = 3
	runWithSettings(t, graph, model, settings, nil)

	if len(graph.recallCalls) != 2 {
		t.Fatalf("test setup error: %d recalls, want the initial one and one supplementary", len(graph.recallCalls))
	}
	for i, call := range graph.recallCalls {
		if call.Limit != 3 {
			t.Errorf("recall %d asked the graph for %d rows, want the settings' 3", i+1, call.Limit)
		}
	}
}

func TestTheAssemblyBudgetOfTheSettingsDecidesWhatTheBlockAdmits(t *testing.T) {
	t.Parallel()

	newGraph := func() *fakeGraph {
		graph := baseGraph()
		graph.candidates = []Candidate{bulky(7, 2_000)}
		return graph
	}

	if record := runWithSettings(t, newGraph(), &fakeModel{}, DefaultSettings(), nil); !record.Candidates[0].Included {
		t.Fatalf("test setup error: the default budget cut a 2000 byte candidate: %+v", record.Candidates[0])
	}

	settings := DefaultSettings()
	settings.AssemblyByteBudget = 1_000
	record := runWithSettings(t, newGraph(), &fakeModel{}, settings, nil)
	if record.Candidates[0].Included || record.Candidates[0].CutReason != cutReasonByteBudget {
		t.Fatalf("candidate = %+v, want it cut for the settings' 1000 byte budget", record.Candidates[0])
	}
}

func TestTheSupplementaryBudgetOfTheSettingsBoundsARecallRound(t *testing.T) {
	t.Parallel()

	newGraph := func() *fakeGraph {
		graph := baseGraph()
		graph.recallQueue = []recallResponse{{}, {Candidates: []Candidate{bulky(8, 500)}}}
		return graph
	}
	round := func(settings RunSettings) ToolCallRecord {
		model := &fakeModel{results: []JudgeResult{recallFor("more"), answered("done")}}
		return onlyRound(t, runWithSettings(t, newGraph(), model, settings, nil))
	}

	if got := round(DefaultSettings()); !got.Results[0].Included {
		t.Fatalf("test setup error: the default round cut a 500 byte row: %+v", got.Results[0])
	}

	settings := DefaultSettings()
	settings.SupplementaryByteBudget = 100
	if got := round(settings); got.Results[0].Included {
		t.Fatalf("a 500 byte row was admitted into a round the settings bound at 100 bytes: %+v", got.Results[0])
	}
}

func TestTheSupplementaryBudgetOfTheSettingsBoundsAnAddressedRead(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes(readableNode(71, strings.Repeat("r", 500)))
	model := &fakeModel{results: []JudgeResult{wantsRead(71), answered("done")}}

	settings := DefaultSettings()
	settings.SupplementaryByteBudget = 100
	record := runWithSettings(t, graph, model, settings, nil)

	round := onlyRound(t, record)
	if want := "at most 100"; !strings.Contains(round.Error, want) {
		t.Fatalf("read error = %q, want it to state the settings' room, %q", round.Error, want)
	}
}

func TestThePromptCeilingOfTheSettingsBoundsWhatARoundMayAddToTheMemory(t *testing.T) {
	t.Parallel()

	graph := baseGraph()
	graph.node.Substance = strings.Repeat("a", 100)
	graph.recallQueue = []recallResponse{{}, {Candidates: []Candidate{bulky(8, 950)}}}
	model := &fakeModel{results: []JudgeResult{recallFor("more"), answered("done")}}

	settings := DefaultSettings()
	settings.AssemblyByteBudget = 0
	settings.SupplementaryByteBudget = 1_000
	round := onlyRound(t, runWithSettings(t, graph, model, settings, nil))

	if round.Results[0].Included {
		t.Fatalf("a 950 byte row was admitted though the settings' ceiling of 1000 bytes leaves 900 after the 100 byte anchor: %+v", round.Results[0])
	}
}

func TestTheCallCapAndOutputBudgetsOfTheSettingsGovernTheJudgementCalls(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{recallFor("more"), recallFor("more still")}}
	settings := DefaultSettings()
	settings.MaxModelCalls = 2
	settings.JudgementBudget = 301
	settings.AnsweringBudget = 407

	record := runWithSettings(t, baseGraph(), model, settings, nil)

	if record.ModelCalls != 2 || !record.CapReached {
		t.Fatalf("modelCalls = %d capReached = %v, want the settings' cap of 2 reached", record.ModelCalls, record.CapReached)
	}
	var budgets []int
	for _, call := range model.calls {
		budgets = append(budgets, call.MaxOutputTokens)
	}
	if !slices.Equal(budgets, []int{301, 407}) {
		t.Fatalf("calls were issued with output budgets %v, want the judgement budget 301 and then the answering budget 407", budgets)
	}
}

func TestTheHistoryLimitsOfTheSettingsDecideWhatTheTurnIsShown(t *testing.T) {
	t.Parallel()

	history := exchangesNumbered(1, 5)

	settings := DefaultSettings()
	settings.HistoryExchangeLimit = 2
	record := runWithSettings(t, baseGraph(), &fakeModel{}, settings, history)
	if len(record.History) != 2 || record.HistoryCut != 3 {
		t.Fatalf("history used = %d cut = %d, want the settings' 2 newest and 3 cut", len(record.History), record.HistoryCut)
	}

	settings = DefaultSettings()
	settings.HistorySafetyLimit = len(renderConversation(history[3:]))
	record = runWithSettings(t, baseGraph(), &fakeModel{}, settings, history)
	if len(record.History) != 2 || record.History[0].Input != "said 4" {
		t.Fatalf("history used = %+v, want the two newest exchanges that fit the settings' safety limit", record.History)
	}
}

func TestTheJudgementBudgetOfTheSettingsPricesTheRemainingTimeGuard(t *testing.T) {
	t.Parallel()

	deadline := guardInstant.Add(60 * time.Second)
	run := func(settings RunSettings) (Record, WriteReceipt, error) {
		turn := turnWithClock(&fakeModel{}, baseGraph(), func() time.Time { return guardInstant })
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		return turn.RunWithOptions(ctx, RunOptions{Input: "hello", Subject: 42, Settings: settings})
	}

	if _, _, err := run(DefaultSettings()); err != nil {
		t.Fatalf("the default budget was refused 60 seconds: %v", err)
	}

	settings := DefaultSettings()
	settings.JudgementBudget = 1_024
	_, _, err := run(settings)
	if !errors.Is(err, ErrRunTimeExhausted) {
		t.Fatalf("err = %v, want the raised budget refused where the default was allowed", err)
	}
	if !strings.Contains(err.Error(), "1024 output tokens") {
		t.Fatalf("the refusal does not state the run's own budget: %v", err)
	}
}

func TestAJudgementBudgetOfFiveHundredPricesTheGuardAtFiveHundredTokensPlusThePrompt(t *testing.T) {
	t.Parallel()

	run := func(remaining time.Duration) error {
		turn := turnWithClock(&fakeModel{}, baseGraph(), func() time.Time { return guardInstant })
		ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(remaining))
		defer cancel()
		settings := DefaultSettings()
		settings.JudgementBudget = 500
		_, _, err := turn.RunWithOptions(ctx, RunOptions{Input: "hello", Subject: 42, Settings: settings})
		return err
	}

	if err := run(80 * time.Second); err != nil {
		t.Fatalf("80 seconds were refused for a call needing 50 seconds of generation and 26.7 of prompt: %v", err)
	}
	err := run(70 * time.Second)
	if !errors.Is(err, ErrRunTimeExhausted) {
		t.Fatalf("err = %v, want 70 seconds refused: 500 tokens at 10 per second and the prompt need 76.7", err)
	}
	if !strings.Contains(err.Error(), "500 output tokens") {
		t.Fatalf("the refusal does not state the run's own output limit: %v", err)
	}
}

func TestThePromptCeilingOfTheSettingsPricesTheRemainingTimeGuard(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	settings.AssemblyByteBudget = 30_000
	settings.SupplementaryByteBudget = 10_000

	turn := turnWithClock(&fakeModel{}, baseGraph(), func() time.Time { return guardInstant })
	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(20*time.Second))
	defer cancel()

	if _, _, err := turn.RunWithOptions(ctx, RunOptions{Input: "hello", Subject: 42, Settings: settings}); err != nil {
		t.Fatalf("a run whose own 40000 byte ceiling is affordable in 20 seconds was refused: %v", err)
	}
	if _, _, err := turn.RunWithOptions(ctx, RunOptions{Input: "hello", Subject: 42, Settings: DefaultSettings()}); !errors.Is(err, ErrRunTimeExhausted) {
		t.Fatalf("err = %v, want the 80000 byte default refused in 20 seconds, so the case above proves the ceiling is the run's own", err)
	}
}

func TestTheFillWaitOfTheSettingsBoundsAndPricesTheSubstanceWait(t *testing.T) {
	t.Parallel()

	turn := newTurnWithGraph(baseGraph())
	turn.Floors = Floors{TokensPerSecond: 100, PromptBytesPerSecond: 10_000}
	turn.clock = func() time.Time { return guardInstant }

	settings := DefaultSettings()
	settings.FillWait = 7 * time.Second
	if got := turn.fillWait(context.Background(), settings); got != 7*time.Second {
		t.Errorf("fillWait with no deadline = %v, want the settings' 7s", got)
	}

	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(20*time.Second))
	defer cancel()

	settings = DefaultSettings()
	settings.JudgementBudget = 1_000
	if got := turn.fillWait(ctx, settings); got != 2*time.Second {
		t.Errorf("fillWait with 20s left = %v, want 2s: 20s less the 18s one 1000 token call costs at these floors", got)
	}
}

func TestTheFillWaitOfTheSettingsGovernsTheSubstanceStepOfEveryPhase(t *testing.T) {
	t.Parallel()

	bareNode := Anchor{ID: 71, Type: "documentation", Name: "Bare", Content: "the body of 71"}

	cases := []struct {
		phase   string
		id      int64
		graph   func() *fakeGraph
		results []JudgeResult
	}{
		{
			phase: phaseInitial,
			id:    42,
			graph: func() *fakeGraph {
				graph := baseGraph()
				graph.node.Substance = ""
				return graph
			},
		},
		{
			phase: phaseRecall,
			id:    8,
			graph: func() *fakeGraph {
				graph := baseGraph()
				graph.recallQueue = []recallResponse{{}, {Candidates: []Candidate{bareRow(8)}}}
				return graph
			},
			results: []JudgeResult{recallFor("more"), answered("done")},
		},
		{
			phase:   phaseRead,
			id:      71,
			graph:   func() *fakeGraph { return graphHoldingNodes(bareNode) },
			results: []JudgeResult{wantsRead(71), answered("done")},
		},
	}

	for _, c := range cases {
		outcome := func(settings RunSettings) FillOutcome {
			turn := turnWithFill(c.graph(), &fakeModel{results: c.results}, &fakeFill{})
			record, _, err := turn.RunWithOptions(context.Background(), RunOptions{Input: "hello", Subject: 42, Settings: settings})
			if err != nil {
				t.Fatalf("%s: RunWithOptions: %v", c.phase, err)
			}
			return outcomeFor(t, record.Fills, c.id)
		}

		if got := outcome(DefaultSettings()); !got.Filled {
			t.Fatalf("%s: test setup error: the default wait did not fill %d: %+v", c.phase, c.id, got)
		}

		settings := DefaultSettings()
		settings.FillWait = 0
		if got := outcome(settings); !got.Deferred || got.Filled {
			t.Errorf("%s: with a fill wait of zero %d was not deferred: %+v", c.phase, c.id, got)
		}
	}
}

func TestThePromptCeilingOfTheSettingsBoundsAnAddressedRead(t *testing.T) {
	t.Parallel()

	graph := graphHoldingNodes(readableNode(71, strings.Repeat("r", 950)))
	graph.nodes[42] = nodeResponse{Anchor: Anchor{ID: 42, Type: "documentation", Name: "Subject", Substance: strings.Repeat("a", 100)}, Found: true}
	model := &fakeModel{results: []JudgeResult{wantsRead(71), answered("done")}}

	settings := DefaultSettings()
	settings.AssemblyByteBudget = 0
	settings.SupplementaryByteBudget = 1_000
	round := onlyRound(t, runWithSettings(t, graph, model, settings, nil))

	if want := "at most 900"; !strings.Contains(round.Error, want) {
		t.Fatalf("read error = %q, want it to state the 900 bytes the settings' ceiling leaves, %q", round.Error, want)
	}
}

func TestTheRefusalNamesThePromptCeilingOfTheSettingsTheRunWasGiven(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	settings.AssemblyByteBudget = 30_000
	settings.SupplementaryByteBudget = 10_000

	turn := turnWithClock(&fakeModel{}, baseGraph(), func() time.Time { return guardInstant })
	ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(5*time.Second))
	defer cancel()

	_, _, err := turn.RunWithOptions(ctx, RunOptions{Input: "hello", Subject: 42, Settings: settings})
	if !errors.Is(err, ErrRunTimeExhausted) {
		t.Fatalf("err = %v, want the run refused with 5 seconds left", err)
	}
	if !strings.Contains(err.Error(), "40000 prompt bytes") {
		t.Fatalf("the refusal does not state the run's own 40000 byte ceiling: %v", err)
	}
}

func TestTheLimitsAreRecordedWithoutAReserveBecauseNoRecallHoldsOne(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(DefaultSettings().Limits())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if strings.Contains(string(encoded), "recallScopeReserve") {
		t.Fatalf("the recorded limits are %s, want no reserve key: a record that states a reserve of zero describes a mechanism the turn no longer has", encoded)
	}
}

func TestAFreshRecordsLimitsCarryNoOutputBudgetKeyAndAnArchivedRecordsStillDecode(t *testing.T) {
	t.Parallel()

	fresh, err := json.Marshal(DefaultSettings().Limits())
	if err != nil {
		t.Fatalf("marshalling the limits failed: %v", err)
	}
	for _, key := range []string{"derivationBudget", "judgementBudget", "answeringBudget"} {
		if strings.Contains(string(fresh), key) {
			t.Fatalf("a run that sent no output limit states %q in its limits: %s", key, fresh)
		}
	}

	var archived Limits
	const stored = `{"candidateLimit":20,"derivationBudget":160,"judgementBudget":192,"answeringBudget":192}`
	if err := json.Unmarshal([]byte(stored), &archived); err != nil {
		t.Fatalf("an archived record's limits no longer decode: %v", err)
	}
	if archived.DerivationBudget != 160 || archived.JudgementBudget != 192 || archived.AnsweringBudget != 192 {
		t.Fatalf("archived limits decoded to %+v, want the budgets the record carries", archived)
	}

	sent := DefaultSettings()
	sent.JudgementBudget = 500
	limited, err := json.Marshal(sent.Limits())
	if err != nil {
		t.Fatalf("marshalling the limits failed: %v", err)
	}
	if !strings.Contains(string(limited), `"judgementBudget":500`) {
		t.Fatalf("a run that sent a limit of 500 does not state it: %s", limited)
	}
}
