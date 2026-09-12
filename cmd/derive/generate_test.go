package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/eval"
	"github.com/telmengedar/processor/internal/loop"
)

const twoRowCorpus = `[
  {"id": "r01", "input": "What bounds one derivation call?", "subject": 100, "stratum": "labelled",
   "required": [{"node": 200, "hash": "1111111111111111111111111111111111111111111111111111111111111111", "why": "an answer lacking this would not name the deadline that fired"}]},
  {"id": "r02", "input": "Where does a run record land?", "subject": 101, "stratum": "labelled",
   "required": [{"node": 201, "hash": "2222222222222222222222222222222222222222222222222222222222222222", "why": "an answer lacking this would not name the write receipt"}]}
]`

const decoratedCompletion = `<think>
The request is What bounds one derivation call? and I should answer with queries.
</think>
1. How does the loop bound the derivation step?
- "Which deadline fires when a derivation stalls?"

What bounds one derivation call?
How does the loop bound the derivation step?
* What names the cause a fallback records?
2) Which context carries the derivation's own cause?
derivation bound deadline cause fallback
a sixth line the cap must refuse`

var decoratedCompletionQueries = []string{
	"How does the loop bound the derivation step?",
	"Which deadline fires when a derivation stalls?",
	"What names the cause a fallback records?",
	"Which context carries the derivation's own cause?",
	"derivation bound deadline cause fallback",
}

type scriptedModel struct {
	text     string
	err      error
	prompts  []string
	ceilings []int
}

func (m *scriptedModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	return loop.JudgeResult{}, errors.New("the generator never judges")
}

func (m *scriptedModel) Derive(_ context.Context, prompt string, maxOutputTokens int) (string, error) {
	m.prompts = append(m.prompts, prompt)
	m.ceilings = append(m.ceilings, maxOutputTokens)
	return m.text, m.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func corpusFrom(t *testing.T, body string) eval.Corpus {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the corpus failed: %v", err)
	}

	corpus, err := eval.Load(path)
	if err != nil {
		t.Fatalf("loading the corpus failed: %v", err)
	}
	return corpus
}

func baselinePinning(pinned map[string][]string) eval.Derivations {
	sources := make(map[string]string, len(pinned))
	for id := range pinned {
		sources[id] = eval.SourceHandAuthored
	}
	return eval.Derivations{Path: "baseline.json", Queries: pinned, Sources: sources}
}

func generateOne(t *testing.T, model loop.ModelPort, row blindRow) map[string][]string {
	t.Helper()

	generated, _, err := generate(context.Background(), model, []blindRow{row}, time.Minute, discardLogger())
	if err != nil {
		t.Fatalf("generating %s failed: %v", row.id, err)
	}
	return generated
}

func generateFailure(t *testing.T, ctx context.Context, model loop.ModelPort, row blindRow, timeout time.Duration) error {
	t.Helper()

	_, _, err := generate(ctx, model, []blindRow{row}, timeout, discardLogger())
	if err == nil {
		t.Fatalf("want an error generating %s, got nil", row.id)
	}
	return err
}

func TestTheGeneratorPinsExactlyTheQuerySetTheProductsOwnParseYieldsForADecoratedCompletion(t *testing.T) {
	row := blindRow{id: "r01", input: "What bounds one derivation call?"}

	generated := generateOne(t, &scriptedModel{text: decoratedCompletion}, row)

	if !slices.Equal(generated["r01"], decoratedCompletionQueries) {
		t.Fatalf("the generated set is\n  %q\nwant\n  %q — the fixture carries a reasoning block, three list decorations, a quoted line, a blank line, an echo of the input, a repeat and a sixth line past the cap, so any parse other than the product's own differs here",
			generated["r01"], decoratedCompletionQueries)
	}
}

func TestTheGeneratorSendsTheProductsOwnDerivationPromptWithTheRowsInputAsItsLastBlock(t *testing.T) {
	row := blindRow{id: "r01", input: "What bounds one derivation call?"}
	model := &scriptedModel{text: decoratedCompletion}

	generateOne(t, model, row)

	if len(model.prompts) != 1 {
		t.Fatalf("want exactly one derivation call, got %d", len(model.prompts))
	}
	prompt := model.prompts[0]
	if !strings.HasPrefix(prompt, "You generate alternate search queries for a semantic retrieval system.") {
		t.Fatalf("the prompt does not open with the shipped instruction text, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "graceful shutdown in-flight request drain timeout") {
		t.Fatalf("the prompt does not carry the shipped exemplars, got:\n%s", prompt)
	}
	if !strings.HasSuffix(prompt, "\n\n"+row.input) {
		t.Fatalf("the prompt does not end with the row's input as its own block, got:\n%s", prompt)
	}
}

func TestTheGeneratorAsksForTheTurnsOwnOutputCeilingRatherThanACeilingOfItsOwn(t *testing.T) {
	model := &scriptedModel{text: decoratedCompletion}

	generateOne(t, model, blindRow{id: "r01", input: "What bounds one derivation call?"})

	if !slices.Equal(model.ceilings, []int{loop.MaxOutputTokens}) {
		t.Fatalf("the derivation calls asked for %v output tokens, want one call at the turn's own ceiling %d", model.ceilings, loop.MaxOutputTokens)
	}
}

func TestTheGeneratorFailsTheWholeBatchWhenOneRowsCompletionParsesToNoQueryAtAll(t *testing.T) {
	rows := []blindRow{{id: "r01", input: "What bounds one derivation call?"}}

	err := generateFailure(t, context.Background(), &scriptedModel{text: "\n   \n\t\n"}, rows[0], time.Minute)

	if !strings.Contains(err.Error(), "r01") {
		t.Fatalf("the error does not name the row that failed, got %v", err)
	}
}

func TestTheGeneratorFailsTheWholeBatchWhenTheModelCallItselfFails(t *testing.T) {
	rows := []blindRow{{id: "r01", input: "What bounds one derivation call?"}}

	err := generateFailure(t, context.Background(), &scriptedModel{err: errors.New("the endpoint refused")}, rows[0], time.Minute)

	if !strings.Contains(err.Error(), "the endpoint refused") {
		t.Fatalf("the error does not carry the call's own cause, got %v", err)
	}
}

func TestTheGeneratorsOwnTimeoutNamesItselfAndNotTheEnclosingDeadlineWhenItIsWhatExpired(t *testing.T) {
	row := blindRow{id: "r02", input: "Where does a run record land?"}

	err := generateFailure(t, context.Background(), blockingModel{}, row, time.Millisecond)

	if !strings.Contains(err.Error(), "the derivation's own 1ms bound expired") {
		t.Fatalf("the error does not name the bound that fired, and a batch has two live deadlines, got %v", err)
	}
	if !strings.Contains(err.Error(), "r02") {
		t.Fatalf("the error does not name the row whose call was cut, got %v", err)
	}
}

func TestTheEnclosingDeadlineNamesItselfRatherThanTheGeneratorsOwnTimeoutWhenItIsWhatExpired(t *testing.T) {
	row := blindRow{id: "r02", input: "Where does a run record land?"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := generateFailure(t, ctx, blockingModel{}, row, time.Hour)

	if !strings.Contains(err.Error(), "the enclosing run bound expired during derivation") {
		t.Fatalf("a cancelled parent is reported as the generator's own timeout, which is the one thing §7.4 says the operator must be able to tell apart, got %v", err)
	}
}

type blockingModel struct{}

func (blockingModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	return loop.JudgeResult{}, errors.New("the generator never judges")
}

func (blockingModel) Derive(ctx context.Context, _ string, _ int) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Second):
		return "", errors.New("no deadline reached this call in five seconds, so nothing bounded it")
	}
}

func TestTheGeneratorsTargetsWithoutOnlyAreExactlyTheRowsTheBaselineLeavesUnpinned(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	baseline := baselinePinning(map[string][]string{"r01": {"a pinned query"}})

	targets, err := selectTargets(blindRows(corpus), baseline, "", false)

	if err != nil {
		t.Fatalf("selecting targets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].id != "r02" {
		t.Fatalf("want only the unpinned row r02, got %+v", targets)
	}
}

func TestTheGeneratorRefusesAnOnlyThatNamesAnAlreadyPinnedRowWithoutForce(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	baseline := baselinePinning(map[string][]string{"r01": {"a pinned query"}})

	targets, err := selectTargets(blindRows(corpus), baseline, "r01,r02", false)

	if err == nil {
		t.Fatal("want a refusal when -only names a pinned row and -force is absent, got nil")
	}
	if !strings.Contains(err.Error(), "r01") || strings.Contains(err.Error(), "r02") {
		t.Fatalf("the refusal must name the pinned row and only the pinned row, got %v", err)
	}
	if targets != nil {
		t.Fatalf("a refused selection must produce no targets, got %+v", targets)
	}
}

func TestTheGeneratorAcceptsAnOnlyThatNamesAnAlreadyPinnedRowWhenForceIsSet(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	baseline := baselinePinning(map[string][]string{"r01": {"a pinned query"}})

	targets, err := selectTargets(blindRows(corpus), baseline, "r02,r01", true)

	if err != nil {
		t.Fatalf("-force must lift the refusal, got %v", err)
	}
	if len(targets) != 2 || targets[0].id != "r01" || targets[1].id != "r02" {
		t.Fatalf("want both named rows in corpus order and not in the order -only spelled them, got %+v", targets)
	}
}

func TestTheGeneratorRefusesAnOnlyNamingARowOutsideTheCorpusEvenWithForce(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)

	_, err := selectTargets(blindRows(corpus), baselinePinning(nil), "r01,r99", true)

	if err == nil {
		t.Fatal("want a refusal when -only names a row the corpus does not carry, got nil")
	}
	if !strings.Contains(err.Error(), "r99") {
		t.Fatalf("the refusal does not name the unknown row, got %v", err)
	}
}

func TestTheGeneratorRefusesAnOnlyThatResolvesToNoRowIdAtAll(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)

	_, err := selectTargets(blindRows(corpus), baselinePinning(nil), " , ", false)

	if err == nil {
		t.Fatal("want a refusal when -only names nothing, got nil")
	}
}

func TestTheGeneratorsMergeStampsEveryRegeneratedRowBlindGeneratedAndKeepsTheCarriedRowsOwnSource(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	baseline := baselinePinning(map[string][]string{"r01": {"a hand-authored query"}})

	merged := mergeSidecar(baseline, map[string][]string{"r02": {"a regenerated query"}}, corpus)

	if len(merged) != 2 {
		t.Fatalf("want both rows in the merged sidecar, got %+v", merged)
	}
	if merged[0].Row != "r01" || merged[1].Row != "r02" {
		t.Fatalf("want the merged sidecar in corpus order, got %q then %q", merged[0].Row, merged[1].Row)
	}
	if merged[0].Source != eval.SourceHandAuthored || !slices.Equal(merged[0].Queries, []string{"a hand-authored query"}) {
		t.Fatalf("a carried row must keep its own queries and source, got %+v", merged[0])
	}
	if merged[1].Source != eval.SourceBlindGenerated || !slices.Equal(merged[1].Queries, []string{"a regenerated query"}) {
		t.Fatalf("a regenerated row must carry the regenerated queries stamped blind-generated, got %+v", merged[1])
	}
}

func TestTheGeneratorsMergeGivesARegeneratedSetPrecedenceOverTheSameRowsBaselinePin(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	baseline := baselinePinning(map[string][]string{"r01": {"the query the baseline pinned"}})

	merged := mergeSidecar(baseline, map[string][]string{"r01": {"the query this run regenerated"}}, corpus)

	if len(merged) != 1 || merged[0].Row != "r01" {
		t.Fatalf("want one entry for the overlapping row, got %+v", merged)
	}
	if !slices.Equal(merged[0].Queries, []string{"the query this run regenerated"}) {
		t.Fatalf("the row is both pinned and regenerated and carries %q; the baseline winning here makes -force spend a model call and write the baseline back", merged[0].Queries)
	}
	if merged[0].Source != eval.SourceBlindGenerated {
		t.Fatalf("the regenerated row carries source %q, want %q re-stamped", merged[0].Source, eval.SourceBlindGenerated)
	}
}

func TestTheGeneratorsTargetsWithOnlyNamingAnUnpinnedRowSucceedWithoutForce(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	baseline := baselinePinning(map[string][]string{"r01": {"a pinned query"}})

	targets, err := selectTargets(blindRows(corpus), baseline, "r02", false)

	if err != nil {
		t.Fatalf("-only naming a row the baseline leaves unpinned needs no -force, got %v", err)
	}
	if len(targets) != 1 || targets[0].id != "r02" {
		t.Fatalf("want the one named unpinned row, got %+v", targets)
	}
}

func TestTheGeneratorsMergeLeavesARowNeitherPinnedNorRegeneratedOutOfTheSidecar(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)

	merged := mergeSidecar(baselinePinning(nil), map[string][]string{"r02": {"a regenerated query"}}, corpus)

	if len(merged) != 1 || merged[0].Row != "r02" {
		t.Fatalf("want only the regenerated row, got %+v", merged)
	}
}

func TestTheGeneratorsWrittenSidecarLoadsBackThroughTheProductsOwnValidatorUnchanged(t *testing.T) {
	corpus := corpusFrom(t, twoRowCorpus)
	path := filepath.Join(t.TempDir(), "regenerated.json")
	entries := []eval.Derivation{{Row: "r01", Queries: decoratedCompletionQueries, Source: eval.SourceBlindGenerated}}

	if err := writeSidecar(path, entries); err != nil {
		t.Fatalf("writing the sidecar failed: %v", err)
	}

	loaded, err := eval.LoadDerivations(path, corpus)
	if err != nil {
		t.Fatalf("the written sidecar did not load back: %v", err)
	}
	if !slices.Equal(loaded.Queries["r01"], decoratedCompletionQueries) {
		t.Fatalf("the sidecar read back %q, want %q", loaded.Queries["r01"], decoratedCompletionQueries)
	}
	if loaded.Sources["r01"] != eval.SourceBlindGenerated {
		t.Fatalf("the sidecar read back source %q, want %q", loaded.Sources["r01"], eval.SourceBlindGenerated)
	}
}

func TestTheGeneratorsWrittenSidecarUsesLineFeedsAndLeavesAnAmpersandUnescaped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "regenerated.json")
	entries := []eval.Derivation{{Row: "r01", Queries: []string{"fusion & admission <ranking>"}, Source: eval.SourceBlindGenerated}}

	if err := writeSidecar(path, entries); err != nil {
		t.Fatalf("writing the sidecar failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the sidecar back failed: %v", err)
	}
	if strings.Contains(string(raw), "\r") {
		t.Fatalf("the sidecar carries a carriage return, and its sha256 is taken over these bytes:\n%q", raw)
	}
	if !strings.Contains(string(raw), "fusion & admission <ranking>") {
		t.Fatalf("the sidecar escaped a query's own characters, so the pinned text is not the text the model produced:\n%s", raw)
	}
	if !strings.HasSuffix(string(raw), "]\n") {
		t.Fatalf("the sidecar does not end in exactly one newline after the closing bracket:\n%q", raw)
	}
}

func TestTheShapeCheckReportsAKeywordLinePhrasedAsAQuestion(t *testing.T) {
	problems := shapeProblems([]string{
		"How does the loop bound the derivation step?",
		"Which deadline fires when a derivation stalls?",
		"What names the cause a fallback records?",
		"Which context carries the derivation's own cause?",
		"How does the ranking system prevent item cycling in top positions",
	})

	if len(problems) != 1 {
		t.Fatalf("want exactly one shape problem, got %v", problems)
	}
	if !strings.Contains(problems[0], "line 5") {
		t.Fatalf("the problem does not name the offending line, got %q", problems[0])
	}
}

func TestTheShapeCheckReportsNothingForFourQuestionsFollowedByAKeywordLine(t *testing.T) {
	if problems := shapeProblems(decoratedCompletionQueries); len(problems) != 0 {
		t.Fatalf("the shipped shape must produce no problem, got %v", problems)
	}
}

func TestTheShapeCheckReportsAQuestionLineThatIsNotPhrasedAsOne(t *testing.T) {
	problems := shapeProblems([]string{
		"How does the loop bound the derivation step?",
		"derivation bound deadline",
		"What names the cause a fallback records?",
		"Which context carries the derivation's own cause?",
		"derivation bound deadline cause fallback",
	})

	if len(problems) != 1 || !strings.Contains(problems[0], "line 2") {
		t.Fatalf("want one problem naming line 2, got %v", problems)
	}
}

func TestTheShapeCheckReportsAKeywordLineEndingInAQuestionMarkThatOpensWithNoInterrogative(t *testing.T) {
	problems := shapeProblems([]string{
		"How does the loop bound the derivation step?",
		"Which deadline fires when a derivation stalls?",
		"What names the cause a fallback records?",
		"Which context carries the derivation's own cause?",
		"the deadline that fires when a derivation stalls?",
	})

	if len(problems) != 1 || !strings.Contains(problems[0], "line 5") {
		t.Fatalf("a last line that opens with no interrogative is still a question when it ends in a mark, and the mark is the only thing that says so here, got %v", problems)
	}
}

func TestTheShapeCheckReportsEveryProblemInASetThatCarriesMoreThanOne(t *testing.T) {
	problems := shapeProblems([]string{
		"derivation bound deadline",
		"Which deadline fires when a derivation stalls?",
		"What names the cause a fallback records?",
		"Which context carries the derivation's own cause?",
		"How does the loop bound the derivation step?",
	})

	if len(problems) != 2 {
		t.Fatalf("want both the unquestioned line 1 and the questioning line 5 reported, got %v", problems)
	}
	if !strings.Contains(problems[0], "line 1") || !strings.Contains(problems[1], "line 5") {
		t.Fatalf("want the problems in line order, got %v", problems)
	}
}

func TestTheShapeCheckReportsACountOtherThanTheProductsOwnCap(t *testing.T) {
	problems := shapeProblems([]string{"How does the loop bound the derivation step?"})

	if len(problems) != 1 || !strings.Contains(problems[0], "got 1") {
		t.Fatalf("want one problem naming the count it got, got %v", problems)
	}
}
