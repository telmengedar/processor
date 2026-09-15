package loop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

const sidecarPath = "../eval/derivations.json"

const blindGenerated = "blind-generated"

const testBound = 20 * time.Millisecond

type sidecarRow struct {
	Row     string   `json:"row"`
	Queries []string `json:"queries"`
	Source  string   `json:"source"`
}

type deriveFake struct {
	text string
	err  error

	hold chan struct{}

	prompts  []string
	tokens   []int
	deadline time.Time
}

func (d *deriveFake) Judge(context.Context, JudgeInput) (JudgeResult, error) {
	return JudgeResult{Reason: Answered, RawReason: "stop"}, nil
}

func (d *deriveFake) Derive(ctx context.Context, prompt string, maxOutputTokens int) (string, error) {
	d.prompts = append(d.prompts, prompt)
	d.tokens = append(d.tokens, maxOutputTokens)
	d.deadline, _ = ctx.Deadline()

	if d.hold != nil {
		select {
		case <-d.hold:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return d.text, d.err
}

var derivationPromptTestNow = time.Date(2026, 9, 12, 8, 30, 0, 0, time.UTC)

func TestTheDerivationPromptCarriesTheInstructionsBothExemplarsAndEndsWithTheInputItself(t *testing.T) {
	t.Parallel()

	const input = "what did the split change"
	prompt := DerivationPrompt(input, derivationPromptTestNow)

	for _, want := range []string{
		"You generate alternate search queries for a semantic retrieval system.",
		"Examples of the desired shape (unrelated to the request you will be given):",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the derivation prompt does not carry %q, so the model is being asked for a shape nothing described", want)
		}
	}

	for _, exemplar := range derivationExemplars {
		if !strings.Contains(prompt, exemplar.input) {
			t.Fatalf("the derivation prompt drops exemplar input %q; the exemplars are how the six-line shape is taught", exemplar.input)
		}
		if !strings.Contains(prompt, "DATES: "+exemplar.dates) {
			t.Fatalf("the derivation prompt drops exemplar DATES line %q", "DATES: "+exemplar.dates)
		}
		for _, query := range exemplar.queries {
			if !strings.Contains(prompt, query) {
				t.Fatalf("the derivation prompt drops exemplar query %q, so an exemplar demonstrates a shorter set than the rules ask for", query)
			}
		}
	}

	if !strings.HasSuffix(prompt, "\n\n"+input) {
		t.Fatalf("the derivation prompt does not end with the input; a request buried above the exemplars is answered as one of them")
	}
}

func TestTheDerivationPromptAsksForExactlyAsManyLinesAsTheCapWillKeep(t *testing.T) {
	t.Parallel()

	prompt := DerivationPrompt("what did the split change", derivationPromptTestNow)

	for _, want := range []string{
		"produce 5 additional queries",
		"Output exactly 6 lines total:",
		"The next 4 lines are distinct",
		"Output ONLY those 6 lines:",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the derivation prompt does not say %q; asking for a count the parse will not keep spends the call on queries that are discarded", want)
		}
	}
}

func TestTheDerivationPromptStatesTheInstantInTheSameSpanTheJudgementPromptUses(t *testing.T) {
	t.Parallel()

	prompt := DerivationPrompt("what did the split change", derivationPromptTestNow)

	if !strings.Contains(prompt, "===== NOW =====\n"+derivationPromptTestNow.Format(time.RFC3339)) {
		t.Fatalf("the derivation prompt does not state the instant in the ===== NOW ===== span, so the model resolving \"today\" has nothing to resolve it against; prompt=%q", prompt)
	}
}

func TestTheDerivationPromptNamesNoProtocolTokenTheAdapterOwns(t *testing.T) {
	t.Parallel()

	prompt := strings.ToLower(DerivationPrompt("what did the split change", derivationPromptTestNow))

	for _, forbidden := range []string{"\"role\"", "assistant:", "json schema", "tool_call", "max_tokens"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("the derivation prompt names %q, which is the adapter's half of the seam and would make the loop's words protocol-specific", forbidden)
		}
	}
}

func TestParseDerivationReturnsTheSidecarsOwnBlindGeneratedSetFromTheTextThatWouldHaveProducedIt(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("reading the pinned sidecar failed: %v", err)
	}

	var rows []sidecarRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("decoding the pinned sidecar failed: %v", err)
	}

	blind := 0
	for _, row := range rows {
		if row.Source != blindGenerated {
			continue
		}
		blind++

		got, window := ParseDerivation(strings.Join(row.Queries, "\n"), "an input no pinned query repeats", time.UTC)
		if !window.IsZero() {
			t.Fatalf("row %s: parsing text with no DATES line yielded window %+v, want zero", row.Row, window)
		}
		if !slices.Equal(got, row.Queries) {
			t.Fatalf("row %s: the product's parse yields %q from the text the generator wrote as %q; the sweep and the turn would then be measuring two different query sets", row.Row, got, row.Queries)
		}
	}

	if blind != 12 {
		t.Fatalf("the pinned sidecar carries %d blind-generated rows, want 12; a table over none of them passes without parsing anything", blind)
	}
}

func TestParseDerivationStripsListDecorationSurroundingQuotesAndReasoningArtifacts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "numbered",
			text: "1. first question?\n2) second question?\n3. dense keyword line",
			want: []string{"first question?", "second question?", "dense keyword line"},
		},
		{
			name: "bulleted",
			text: "- first question?\n* second question?\n• dense keyword line",
			want: []string{"first question?", "second question?", "dense keyword line"},
		},
		{
			name: "quoted",
			text: "\"first question?\"\n'second question?'\n“dense keyword line”",
			want: []string{"first question?", "second question?", "dense keyword line"},
		},
		{
			name: "reasoning block removed whole",
			text: "<think>\nthe user wants queries about splitting\n</think>\nfirst question?\ndense keyword line",
			want: []string{"first question?", "dense keyword line"},
		},
		{
			name: "blank lines between queries",
			text: "first question?\n\n   \n\nsecond question?",
			want: []string{"first question?", "second question?"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, _ := ParseDerivation(tc.text, "what did the split change", time.UTC)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ParseDerivation = %q, want %q: a query carrying its own decoration is embedded as that decoration and ranks against it", got, tc.want)
			}
		})
	}
}

func TestParseDerivationOfATextWhoseEveryLineIsBlankReturnsAnEmptySliceAndNotNil(t *testing.T) {
	t.Parallel()

	got, _ := ParseDerivation("\n   \n\t\n", "what did the split change", time.UTC)

	if got == nil {
		t.Fatalf("ParseDerivation returned nil for an all-blank text; a caller distinguishing nil from empty would read two outcomes where the design has one")
	}
	if len(got) != 0 {
		t.Fatalf("ParseDerivation = %q for an all-blank text, want no queries: a blank query ranks the whole graph by nothing", got)
	}
}

func TestParseDerivationDropsALineThatOnlyEchoesTheInputInAnotherCase(t *testing.T) {
	t.Parallel()

	const input = "What did the split change?"
	got, _ := ParseDerivation("WHAT DID THE SPLIT CHANGE?\na genuinely different angle?", input, time.UTC)

	want := []string{"a genuinely different angle?"}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseDerivation = %q, want %q: the input is already query zero, so keeping its echo issues one recall twice and doubles that list's weight in the reciprocal-rank sum", got, want)
	}
}

func TestParseDerivationDropsARepeatOfAQueryItAlreadyKept(t *testing.T) {
	t.Parallel()

	got, _ := ParseDerivation("first question?\nFirst Question?\nsecond question?", "what did the split change", time.UTC)

	want := []string{"first question?", "second question?"}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseDerivation = %q, want %q: the same query twice is one list summed twice", got, want)
	}
}

func TestParseDerivationKeepsNoMoreQueriesThanTheCapAdmits(t *testing.T) {
	t.Parallel()

	got, _ := ParseDerivation("q1?\nq2?\nq3?\nq4?\nq5?\nq6?\nq7?", "what did the split change", time.UTC)

	want := []string{"q1?", "q2?", "q3?", "q4?", "q5?"}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseDerivation = %q, want %q: an uncapped set is an uncapped fan-out of graph reads per turn", got, want)
	}
}

func TestMergeQueriesPutsTheRawInputFirstAndKeepsTheDerivedOrderBehindIt(t *testing.T) {
	t.Parallel()

	const input = "what did the split change"
	got := MergeQueries(input, []string{"first derived?", "second derived?"})

	want := []string{input, "first derived?", "second derived?"}
	if !slices.Equal(got, want) {
		t.Fatalf("MergeQueries = %q, want %q: the scoped recall carries query zero, so a derived query at index zero ranks the neighbourhood against a question the caller never asked", got, want)
	}
}

func TestMergeQueriesDropsADerivedQueryThatRepeatsTheRawInputExactly(t *testing.T) {
	t.Parallel()

	const input = "what did the split change"
	got := MergeQueries(input, []string{input, "a different angle?"})

	want := []string{input, "a different angle?"}
	if !slices.Equal(got, want) {
		t.Fatalf("MergeQueries = %q, want %q", got, want)
	}
}

func TestMergeQueriesOnNoDerivedQueriesIsTheRawInputAlone(t *testing.T) {
	t.Parallel()

	const input = "what did the split change"

	for _, derived := range [][]string{nil, {}} {
		got := MergeQueries(input, derived)
		if want := []string{input}; !slices.Equal(got, want) {
			t.Fatalf("MergeQueries(%q, %#v) = %q, want %q: the fallback must be today's shipped query set byte for byte", input, derived, got, want)
		}
	}
}

func TestDeriveQueriesSendsTheRenderedPromptAndReturnsWhatTheTextParsesTo(t *testing.T) {
	t.Parallel()

	const input = "what did the split change"
	model := &deriveFake{text: "first derived?\nsecond derived?"}

	got, window, err := DeriveQueries(context.Background(), model, input, derivationPromptTestNow)
	if err != nil {
		t.Fatalf("DeriveQueries returned %v, want the parsed queries", err)
	}

	if want := []string{"first derived?", "second derived?"}; !slices.Equal(got, want) {
		t.Fatalf("DeriveQueries = %q, want %q", got, want)
	}
	if !window.IsZero() {
		t.Fatalf("DeriveQueries returned window %+v for text carrying no DATES line, want zero", window)
	}
	if len(model.prompts) != 1 {
		t.Fatalf("the derivation made %d model calls, want exactly 1: a second is a retry, which doubles the latency of the step whose cheapness is its justification", len(model.prompts))
	}
	if model.prompts[0] != DerivationPrompt(input, derivationPromptTestNow) {
		t.Fatalf("the derivation sent a prompt other than the rendered one, so what the model was asked is not what this package's tests pin")
	}
	if model.tokens[0] != 4096 {
		t.Fatalf("the derivation asked for %d output tokens, want the turn's own budget of 4096", model.tokens[0])
	}
}

func TestDeriveQueriesBoundsTheCallAtThirtySecondsAndNotAtTheAdaptersOwnTimeout(t *testing.T) {
	t.Parallel()

	model := &deriveFake{text: "first derived?"}

	if _, _, err := DeriveQueries(context.Background(), model, "what did the split change", derivationPromptTestNow); err != nil {
		t.Fatalf("DeriveQueries returned %v", err)
	}

	if model.deadline.IsZero() {
		t.Fatalf("the derivation call carried no deadline, so a hung endpoint holds the turn for the adapter's own five-minute bound")
	}
	if remaining := time.Until(model.deadline); remaining <= 29*time.Second || remaining > 30*time.Second {
		t.Fatalf("the derivation call carried %s of deadline, want just under 30s: a step whose failure is free must be bounded well below the step whose failure is not", remaining)
	}
}

func TestDeriveQueriesNamesItsOwnBoundAndTheElapsedWhenTheDerivationOutlastsIt(t *testing.T) {
	t.Parallel()

	model := &deriveFake{hold: make(chan struct{})}

	_, _, err := deriveQueries(context.Background(), model, "what did the split change", derivationPromptTestNow, testBound)
	if err == nil {
		t.Fatalf("a derivation that outlasted its bound returned no error")
	}

	if !strings.Contains(err.Error(), "the derivation's own 20ms bound expired after") {
		t.Fatalf("the cause reads %q; it must name the derivation's own bound and its elapsed, because both live deadlines surface as the same error value", err)
	}
}

func TestDeriveQueriesNamesTheEnclosingRunBoundWhenTheParentDeadlineIsWhatExpired(t *testing.T) {
	t.Parallel()

	model := &deriveFake{hold: make(chan struct{})}

	ctx, cancel := context.WithTimeout(context.Background(), testBound)
	defer cancel()

	_, _, err := deriveQueries(ctx, model, "what did the split change", derivationPromptTestNow, time.Hour)
	if err == nil {
		t.Fatalf("a derivation whose enclosing run expired returned no error")
	}

	if !strings.Contains(err.Error(), "the enclosing run bound expired during derivation after") {
		t.Fatalf("the cause reads %q; a run whose own ten minutes ran out mid-derivation must not be recorded as a fast step that took too long, and the two are one error value apart", err)
	}
}

func TestDeriveQueriesCarriesTheAdaptersOwnSentenceWhenTheCallFailsWithNoDeadlineInPlay(t *testing.T) {
	t.Parallel()

	model := &deriveFake{err: errors.New("openaicompat: unexpected status 503: model is loading")}

	_, _, err := deriveQueries(context.Background(), model, "what did the split change", derivationPromptTestNow, time.Hour)
	if err == nil {
		t.Fatalf("a failed derivation call returned no error")
	}

	if !strings.Contains(err.Error(), "model is loading") {
		t.Fatalf("the cause reads %q and drops the endpoint's own sentence; the operator responses to an unreachable host and to an unparseable answer are different", err)
	}
	if strings.Contains(err.Error(), "bound expired") {
		t.Fatalf("the cause reads %q and blames a bound no deadline reached", err)
	}
}

func TestParseDerivationYieldsNoWindowForEveryMalformedDatesValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
	}{
		{name: "none", text: "DATES: none\nfirst question?"},
		{name: "absent — no line matches the prefix at all", text: "first question?\nsecond question?"},
		{name: "garbage value", text: "DATES: sometime soon\nfirst question?"},
		{name: "inverted range", text: "DATES: 2026-09-15..2026-09-10\nfirst question?"},
		{name: "invalid calendar date", text: "DATES: 2026-13-40..2026-13-40\nfirst question?"},
		{name: "one bound only", text: "DATES: 2026-09-12\nfirst question?"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, window := ParseDerivation(tc.text, "what did the split change", time.UTC)
			if !window.IsZero() {
				t.Fatalf("ParseDerivation(%q) returned window %+v, want zero: a wrong window must never be constructed from an unparseable line", tc.text, window)
			}
		})
	}
}

func TestParseDerivationResolvesADayRangeInTheLocationItIsGiven(t *testing.T) {
	t.Parallel()

	const text = "DATES: 2026-09-12..2026-09-12\nfirst question?"

	zoneA := time.FixedZone("test+02", 2*60*60)
	zoneB := time.FixedZone("test-05", -5*60*60)

	_, windowA := ParseDerivation(text, "what did the split change", zoneA)
	_, windowB := ParseDerivation(text, "what did the split change", zoneB)

	if windowA.From.Equal(windowB.From) {
		t.Fatalf("the same DATES line resolved to the same instant in two different zones (%s): a parser reading time.Local or forcing UTC would do exactly this", windowA.From.Format(time.RFC3339))
	}
	if !windowA.From.Equal(time.Date(2026, 9, 12, 0, 0, 0, 0, zoneA)) {
		t.Fatalf("windowA.From = %s, want midnight of the 12th in zoneA", windowA.From.Format(time.RFC3339))
	}
	if !windowB.From.Equal(time.Date(2026, 9, 12, 0, 0, 0, 0, zoneB)) {
		t.Fatalf("windowB.From = %s, want midnight of the 12th in zoneB", windowB.From.Format(time.RFC3339))
	}
}

type directiveSpelling struct {
	name string
	line func(value string) string
}

func directiveSpellings() []directiveSpelling {
	return []directiveSpelling{
		{"bare", func(v string) string { return "DATES: " + v }},
		{"bullet dash", func(v string) string { return "- DATES: " + v }},
		{"bullet star", func(v string) string { return "* DATES: " + v }},
		{"bullet dot", func(v string) string { return "• DATES: " + v }},
		{"numbered dot", func(v string) string { return "1. DATES: " + v }},
		{"numbered paren", func(v string) string { return "1) DATES: " + v }},
		{"quote straight", func(v string) string { return "\"DATES: " + v + "\"" }},
		{"quote curly", func(v string) string { return "“DATES: " + v + "”" }},
		{"quote single", func(v string) string { return "'DATES: " + v + "'" }},
		{"lower-case", func(v string) string { return "dates: " + v }},
		{"mixed-case", func(v string) string { return "Dates: " + v }},
	}
}

type directiveLeakFixture struct {
	name        string
	text        string
	mustSurvive []string
}

func directiveLeakFixtures() []directiveLeakFixture {
	const value = "2026-09-12..2026-09-12"

	place := map[string]func(line string) string{
		"first":  func(line string) string { return line + "\nfirst question?\nsecond question?" },
		"middle": func(line string) string { return "first question?\n" + line + "\nsecond question?" },
		"last":   func(line string) string { return "first question?\nsecond question?\n" + line },
	}
	placements := []string{"first", "middle", "last"}

	var fixtures []directiveLeakFixture
	for _, placement := range placements {
		for _, spelling := range directiveSpellings() {
			fixtures = append(fixtures, directiveLeakFixture{
				name:        placement + " placement, " + spelling.name + " spelling",
				text:        place[placement](spelling.line(value)),
				mustSurvive: []string{"first question?", "second question?"},
			})
		}
	}

	fixtures = append(fixtures,
		directiveLeakFixture{
			name:        "two directives, first valid on line one",
			text:        "DATES: 2026-09-12..2026-09-12\nDATES: 2026-09-15..2026-09-15\nfirst question?",
			mustSurvive: []string{"first question?"},
		},
		directiveLeakFixture{
			name:        "three directives, mixed spelling",
			text:        "DATES: 2026-09-12..2026-09-12\nDATES: none\ndates: 2026-09-20..2026-09-21\nfirst question?",
			mustSurvive: []string{"first question?"},
		},
	)

	for _, v := range []struct{ name, value string }{
		{"none", "none"},
		{"garbage value", "sometime soon"},
		{"inverted range", "2026-09-15..2026-09-10"},
		{"invalid calendar date", "2026-13-40..2026-13-40"},
		{"one bound only", "2026-09-12"},
		{"valid range", "2026-09-12..2026-09-12"},
	} {
		fixtures = append(fixtures, directiveLeakFixture{
			name:        "value: " + v.name,
			text:        "DATES: " + v.value + "\nfirst question?",
			mustSurvive: []string{"first question?"},
		})
	}

	return fixtures
}

var leakCheckDecoration = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s*`)

const leakCheckQuoteCutset = "\"“”'"

func queryStillLooksLikeADirective(query string) bool {
	reduced := strings.Trim(leakCheckDecoration.ReplaceAllString(strings.TrimSpace(query), ""), leakCheckQuoteCutset)
	return len(reduced) >= len(dateLinePrefix) && strings.EqualFold(reduced[:len(dateLinePrefix)], dateLinePrefix)
}

func TestTheDatesLineNeverAppearsInTheQuerySetWhateverItsValuePlacementOrSpelling(t *testing.T) {
	t.Parallel()

	for _, fixture := range directiveLeakFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			got, _ := ParseDerivation(fixture.text, "what did the split change", time.UTC)

			for _, query := range got {
				if queryStillLooksLikeADirective(query) {
					t.Fatalf("ParseDerivation(%q) returned query %q: a recognised directive survived into the query set", fixture.text, query)
				}
			}
			for _, want := range fixture.mustSurvive {
				if !slices.Contains(got, want) {
					t.Fatalf("ParseDerivation(%q) returned %q, want it still to carry %q: only a recognised directive line is ever excluded", fixture.text, got, want)
				}
			}
		})
	}
}

func TestADateReachesTheQuerySetOnlyFromALineTheParserDoesNotRecogniseAsTheDirective(t *testing.T) {
	t.Parallel()

	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

	t.Run("a model-authored query line legitimately carrying a date survives behind DATES: none", func(t *testing.T) {
		t.Parallel()

		got, _ := ParseDerivation("DATES: none\nwhat changed on 2026-09-12?\nsecond question?", "what did the split change", time.UTC)
		if !slices.ContainsFunc(got, datePattern.MatchString) {
			t.Fatalf("ParseDerivation returned %q, want a surviving query carrying 2026-09-12: dropping it enforces derive.go's rule at the parse boundary, which L8 rules against", got)
		}
	})

	t.Run("a directive wearing decoration this file does not strip survives, date and all", func(t *testing.T) {
		t.Parallel()

		got, window := ParseDerivation("**DATES: 2026-09-12..2026-09-12**\nfirst question?", "what did the split change", time.UTC)
		if !window.IsZero() {
			t.Fatalf("window = %+v, want zero: an unrecognised directive must not name a window either", window)
		}
		if !slices.ContainsFunc(got, datePattern.MatchString) {
			t.Fatalf("ParseDerivation returned %q, want the mangled directive line to survive carrying its date: L9 is an accepted limit, not a silently fixed one", got)
		}
	})

	for _, fixture := range directiveLeakFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			got, _ := ParseDerivation(fixture.text, "what did the split change", time.UTC)
			for _, query := range got {
				if datePattern.MatchString(query) {
					t.Fatalf("ParseDerivation(%q) returned query %q carrying a date: a recognised directive must never leak its date into the query set", fixture.text, query)
				}
			}
		})
	}
}

func TestOnlyAFirstContentLineDirectiveNamesTheWindowAndNoneDoesWhenTwoAppear(t *testing.T) {
	t.Parallel()

	wantRange := UpdateWindow{
		From: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	}

	t.Run("a directive that is not the first content line yields zero", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("first question?\nDATES: 2026-09-12..2026-09-12", "what did the split change", time.UTC)
		if !window.IsZero() {
			t.Fatalf("window = %+v, want zero: a directive that is not the first content line may be a candidate the model discarded — this is the round-1 relaxation QA falsified", window)
		}
	})

	t.Run("two directive lines yield zero even when the first is valid on line one", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("DATES: 2026-09-12..2026-09-12\nDATES: 2026-09-15..2026-09-15\nfirst question?", "what did the split change", time.UTC)
		if !window.IsZero() {
			t.Fatalf("window = %+v, want zero: the output does not say which of two directives is the answer — a first-wins implementation fails here", window)
		}
	})

	t.Run("a decorated directive on line one yields its range", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("- DATES: 2026-09-12..2026-09-12\nfirst question?", "what did the split change", time.UTC)
		if window != wantRange {
			t.Fatalf("window = %+v, want %+v: decoration this file already strips must not hide the directive from the window gate", window, wantRange)
		}
	})

	t.Run("a lower-case directive on line one yields its range", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("dates: 2026-09-12..2026-09-12\nfirst question?", "what did the split change", time.UTC)
		if window != wantRange {
			t.Fatalf("window = %+v, want %+v: case must not hide the directive from the window gate", window, wantRange)
		}
	})

	t.Run("zero directive lines yield zero", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("first question?\nsecond question?", "what did the split change", time.UTC)
		if !window.IsZero() {
			t.Fatalf("window = %+v, want zero: there is no directive to name one from", window)
		}
	})

	t.Run("a decoration-only line before the directive still yields its range", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("-\nDATES: 2026-09-12..2026-09-12\nfirst question?", "what did the split change", time.UTC)
		if window != wantRange {
			t.Fatalf("window = %+v, want %+v: a line that reduces to nothing is not a content line, so it cannot block the directive from being the first one", window, wantRange)
		}
	})

	t.Run("a code fence before the directive yields zero", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("```\nDATES: 2026-09-12..2026-09-12\nfirst question?", "what did the split change", time.UTC)
		if !window.IsZero() {
			t.Fatalf("window = %+v, want zero: a code fence is content — neither decoration nor a quote character — so it is the first content line and the directive is not", window)
		}
	})

	t.Run("a line of listed characters that does not reduce to empty still counts as content", func(t *testing.T) {
		t.Parallel()

		_, window := ParseDerivation("- -\nDATES: 2026-09-12..2026-09-12\nfirst question?", "what did the split change", time.UTC)
		if !window.IsZero() {
			t.Fatalf("window = %+v, want zero: \"- -\" reduces to \"-\", which is non-empty, so it is a content line and the directive on the next line is not the first one — an implementation that skips any line built only from listed characters fails here", window)
		}
	})
}

func TestDeriveQueriesRefusesATextThatParsesToNoQueryAtAll(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"", "\n  \n", "what did the split change"} {
		got, _, err := deriveQueries(context.Background(), &deriveFake{text: text}, "what did the split change", derivationPromptTestNow, time.Hour)
		if err == nil {
			t.Fatalf("a derivation yielding %q returned %q rather than a cause; a model that refuses returns something rather than erroring", text, got)
		}
		if !strings.Contains(err.Error(), "no usable query") {
			t.Fatalf("the cause for %q reads %q, which does not say the text was unusable", text, err)
		}
	}
}
