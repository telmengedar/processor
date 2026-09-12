package loop

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

const derivedInput = "what did the split change"

const distinctiveDerived = "which merge rule does the sweep share with the turn?"

func derivingGraph() *fakeGraph {
	return &fakeGraph{
		node:       Anchor{ID: 42, Type: "documentation", Name: "Subject", Content: "anchor body"},
		nodeFound:  true,
		candidates: []Candidate{{ID: 101, Name: "one", Content: "a"}, {ID: 102, Name: "two", Content: "b"}},
	}
}

func runDerivedTurn(t *testing.T, model *fakeModel) (*fakeGraph, Record) {
	t.Helper()

	graph := derivingGraph()
	record, _, err := NewTurn(graph, model, nil, "system", "test-model", testLogger()).Run(context.Background(), derivedInput, 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return graph, record
}

func unscopedQueries(graph *fakeGraph) []string {
	var queries []string
	for _, call := range graph.recallCalls {
		if len(call.Scope) == 0 {
			queries = append(queries, call.Query)
		}
	}
	return queries
}

func candidateIDsOf(record Record) []int64 {
	ids := make([]int64, len(record.Candidates))
	for i, d := range record.Candidates {
		ids[i] = d.ID
	}
	slices.Sort(ids)
	return ids
}

func TestTheDerivedQueriesAreIssuedToTheGraphAndNotMerelyComputed(t *testing.T) {
	t.Parallel()

	model := &fakeModel{derivedText: distinctiveDerived + "\nsecond derived?"}
	graph, record := runDerivedTurn(t, model)

	if !slices.Contains(unscopedQueries(graph), distinctiveDerived) {
		t.Fatalf("the graph was asked %q, and none of them is the derived query %q: a derivation that is computed and never issued produces the raw arm's ranking under the derived arm's name", unscopedQueries(graph), distinctiveDerived)
	}
	if want := []string{derivedInput, distinctiveDerived, "second derived?"}; !slices.Equal(record.Queries, want) {
		t.Fatalf("the record carries queries %q, want %q", record.Queries, want)
	}
	if len(model.derivePrompts) != 1 {
		t.Fatalf("the turn made %d derivation calls, want exactly 1", len(model.derivePrompts))
	}
}

func TestTheRawInputIsStillQueryZeroAfterADerivationSucceeds(t *testing.T) {
	t.Parallel()

	graph, record := runDerivedTurn(t, &fakeModel{derivedText: distinctiveDerived})

	if record.Queries[0] != derivedInput {
		t.Fatalf("query zero is %q, want the raw input %q: the scoped recall carries query zero, so a derived question at that index ranks the neighbourhood against something the caller never asked", record.Queries[0], derivedInput)
	}

	scoped := graph.recallCalls[len(graph.recallCalls)-1]
	if len(scoped.Scope) == 0 || scoped.Query != derivedInput {
		t.Fatalf("the scoped recall queried %q over scope %v, want the raw input", scoped.Query, scoped.Scope)
	}
}

func TestEveryDerivationFailureLeavesTheTurnWithTheCandidateSetARawOnlyRunProduces(t *testing.T) {
	t.Parallel()

	_, rawOnly := runDerivedTurn(t, &fakeModel{deriveErr: errors.New("derivation refused for this fixture")})
	want := candidateIDsOf(rawOnly)

	failures := []struct {
		name  string
		model *fakeModel
	}{
		{name: "transport error", model: &fakeModel{deriveErr: errors.New("openaicompat: request failed: dial tcp: connection refused")}},
		{name: "non-2xx", model: &fakeModel{deriveErr: errors.New("openaicompat: unexpected status 503: model is loading")}},
		{name: "bound fired", model: &fakeModel{deriveErr: context.DeadlineExceeded}},
		{name: "empty text", model: &fakeModel{derivedText: ""}},
		{name: "all blank lines", model: &fakeModel{derivedText: "\n   \n\t\n"}},
		{name: "only line echoes the input", model: &fakeModel{derivedText: strings.ToUpper(derivedInput)}},
	}

	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph, record := runDerivedTurn(t, tc.model)

			if got := []string{derivedInput}; !slices.Equal(record.Queries, got) {
				t.Fatalf("the record carries queries %q, want the raw input alone: a degraded derivation must fall back to today's shipped query set byte for byte", record.Queries)
			}
			if got := unscopedQueries(graph); !slices.Equal(got, []string{derivedInput}) {
				t.Fatalf("the graph was asked %q unscoped, want the raw input alone", got)
			}
			if got := candidateIDsOf(record); !slices.Equal(got, want) {
				t.Fatalf("the candidate set is %v, want %v: a fallback that merely returns non-nil can still deliver a different set than the raw arm", got, want)
			}
			if record.DerivationError == "" {
				t.Fatalf("the record carries no derivation cause, so a reader cannot tell whether the endpoint was unreachable, the answer was unparseable, or a bound fired")
			}
		})
	}
}

func TestTheDerivationCauseReachesTheRecordAndIsAbsentWhenTheDerivationSucceeded(t *testing.T) {
	t.Parallel()

	const distinctiveCause = "ollama: unexpected status 418: the fixture's own sentence"

	_, failed := runDerivedTurn(t, &fakeModel{deriveErr: errors.New(distinctiveCause)})
	if !strings.Contains(failed.DerivationError, distinctiveCause) {
		t.Fatalf("the record carries %q, which does not contain the adapter's own sentence %q", failed.DerivationError, distinctiveCause)
	}

	_, derived := runDerivedTurn(t, &fakeModel{derivedText: distinctiveDerived})
	if derived.DerivationError != "" {
		t.Fatalf("a run whose derivation succeeded carries the cause %q; an empty string is what a dropped field decodes to, so this is the assertion that discriminates", derived.DerivationError)
	}
}

func TestADerivationCauseLongerThanTheCarriedBoundIsBoundedOnTheRecord(t *testing.T) {
	t.Parallel()

	_, record := runDerivedTurn(t, &fakeModel{deriveErr: errors.New(strings.Repeat("x", CarriedCauseRunes*2))})

	if got := len([]rune(record.DerivationError)); got != CarriedCauseRunes {
		t.Fatalf("the record carries a %d-rune derivation cause, want it bounded to %d: an unbounded cause on a record the graph embeds pushes the fields after it past the embedding cap", got, CarriedCauseRunes)
	}
}

func TestTheDerivationCallIsNotChargedToTheTurnsJudgementBudget(t *testing.T) {
	t.Parallel()

	_, derived := runDerivedTurn(t, &fakeModel{derivedText: distinctiveDerived})
	_, fellBack := runDerivedTurn(t, &fakeModel{deriveErr: errors.New("derivation refused for this fixture")})

	if derived.ModelCalls != fellBack.ModelCalls {
		t.Fatalf("a turn that derived reports %d model calls against %d for one that fell back: charging derivation to the judgement budget makes a turn's reasoning budget depend on whether its query was derived", derived.ModelCalls, fellBack.ModelCalls)
	}
	if derived.ModelCalls != 1 {
		t.Fatalf("a turn that derived and answered reports %d model calls, want 1 judgement call", derived.ModelCalls)
	}
	if derived.Limits.MaxModelCalls != fellBack.Limits.MaxModelCalls {
		t.Fatalf("the judgement cap moved between the two arms: %d against %d", derived.Limits.MaxModelCalls, fellBack.Limits.MaxModelCalls)
	}
}

func TestRenderSummaryCarriesTheDerivationCauseOnlyWhenTheQuerySetFellBack(t *testing.T) {
	t.Parallel()

	const cause = "the derivation's own 30s bound expired after 30s"

	fellBack := RenderSummary(Record{Queries: []string{derivedInput}, DerivationError: cause}, summaryInstant())
	if !strings.Contains(fellBack, cause) {
		t.Fatalf("the summary of a fallback run does not carry %q, so an operator reading it sees one query and no reason for it:\n%s", cause, fellBack)
	}

	derived := RenderSummary(Record{Queries: []string{derivedInput, distinctiveDerived}}, summaryInstant())
	if strings.Contains(derived, "derivation:") {
		t.Fatalf("the summary of a run that derived carries a derivation line:\n%s", derived)
	}
}

func TestTheDerivationCauseIsEncodedAheadOfTheAnchorSoItStaysInsideTheGraphsEmbeddingWindow(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(Record{Input: derivedInput, Queries: []string{derivedInput}, DerivationError: "a cause"})
	if err != nil {
		t.Fatalf("marshalling the record failed: %v", err)
	}

	encoded := string(body)
	cause := strings.Index(encoded, `"derivationError"`)
	anchor := strings.Index(encoded, `"anchor"`)

	if cause < 0 || anchor < 0 {
		t.Fatalf("the encoded record carries derivationError at %d and anchor at %d; both must be present:\n%s", cause, anchor, encoded)
	}
	if cause > anchor {
		t.Fatalf("the encoded record puts derivationError after the anchor; the candidates that follow run to tens of kilobytes, so a field declared past them is outside the window the graph embeds and is unreachable by the search that would find it")
	}
}

func TestTheSummaryBoundsALongDerivationCauseWithTheSameTruncatorTheToolErrorsUse(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("z", summaryErrorRunes*2)
	rendered := RenderSummary(Record{Queries: []string{derivedInput}, DerivationError: long}, summaryInstant())

	line := ""
	for _, candidate := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(strings.TrimSpace(candidate), "derivation:") {
			line = strings.TrimSpace(candidate)
		}
	}
	if line == "" {
		t.Fatalf("the summary carries no derivation line for a long cause:\n%s", rendered)
	}

	if strings.Contains(line, long) {
		t.Fatalf("the summary printed the whole %d-rune cause; the summary is the compact projection and a cause this long displaces everything an operator reads it for", len([]rune(long)))
	}
	if got := len([]rune(line)); got > summaryErrorRunes+len("derivation: ") {
		t.Fatalf("the derivation line runs to %d runes, want it bounded by the same truncator the tool-call errors already use", got)
	}
	if !strings.Contains(line, strings.Repeat("z", summaryErrorRunes-1)) {
		t.Fatalf("the derivation line is bounded tighter than the tool-call errors are; a second bound for the same kind of text is a second idiom to keep in step: %s", line)
	}
}

func TestTheOperatorLogTimesTheDerivationStepOnBothPathsAndCarriesTheWholeCauseOnFallback(t *testing.T) {
	t.Parallel()

	const longCause = "ollama: request failed: " + "a sentence far longer than the record's own bound keeps, repeated: "

	var fallbackLog strings.Builder
	graph := derivingGraph()
	_, _, err := NewTurn(graph, &fakeModel{deriveErr: errors.New(strings.Repeat(longCause, 12))}, nil, "system", "test-model",
		slog.New(slog.NewTextHandler(&fallbackLog, nil))).Run(context.Background(), derivedInput, 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	fallback := fallbackLog.String()
	if !strings.Contains(fallback, "the query set fell back to the raw input alone") {
		t.Fatalf("the operator log carries no warning for a run that fell back:\n%s", fallback)
	}
	if !strings.Contains(fallback, strings.Repeat(longCause, 12)) {
		t.Fatalf("the operator log bounded the cause; stderr is the operator's own stream and the bound exists for the durable destinations:\n%s", fallback)
	}
	if !strings.Contains(fallback, "elapsed=") {
		t.Fatalf("the fallback log does not time the derivation step:\n%s", fallback)
	}

	var successLog strings.Builder
	_, _, err = NewTurn(derivingGraph(), &fakeModel{derivedText: distinctiveDerived}, nil, "system", "test-model",
		slog.New(slog.NewTextHandler(&successLog, nil))).Run(context.Background(), derivedInput, 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	success := successLog.String()
	line := ""
	for _, candidate := range strings.Split(success, "\n") {
		if strings.Contains(candidate, "queries derived") {
			line = candidate
		}
	}
	if line == "" {
		t.Fatalf("the operator log says nothing about a derivation that succeeded:\n%s", success)
	}
	if !strings.Contains(line, "elapsed=") {
		t.Fatalf("the success path is not timed, so the question of what this step actually costs on a healthy host stays unanswerable: %s", line)
	}
	if !strings.Contains(line, "derived=1") {
		t.Fatalf("the success log does not say how many queries were derived: %s", line)
	}
}
