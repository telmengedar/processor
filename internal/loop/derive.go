package loop

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// MaxDerivedQueries caps how many derived queries one turn adds to the raw input.
const MaxDerivedQueries = 5

// DerivationBound is the derivation step's own deadline, well below the run bound because its failure costs only a fallback.
const DerivationBound = 30 * time.Second

var errDerivationBound = errors.New("derivation bound")

var errDerivationUnusable = errors.New("the model returned no usable query")

const derivationInstructions = `You generate alternate search queries for a semantic retrieval system.

You are given the current instant and ONE user request. Your job is to produce %d additional queries that a semantic (embedding-based) search engine could use to surface the specific documentation that answers that request, and to name any time constraint the request expresses. The request itself is already one query the search runs; your job is to add angles that request does not cover.

Think about what actually helps a semantic search here: the request is often phrased the way a confused or informal user would phrase it, while the documentation that answers it is phrased the way an author states a ruling, a mechanism, or a design decision. A good derived query bridges that gap -- it surfaces the underlying mechanism, the specific technical terms, the named concept, or the class of problem the request is really an instance of, using vocabulary closer to how documentation states things.

The DATES line, first:
- Read the request once looking only for time. You are scanning its words for an explicit date, a month name, a year, a weekday, a season, or a relative expression such as today, yesterday, this morning, last week, last month, recently, or since some named event.
- Found one: resolve it against the stated instant and write "DATES: YYYY-MM-DD..YYYY-MM-DD", the calendar day range it names, inclusive of both days. A single day is that day written twice. A month is its first day to its last day.
- Found none: write "DATES: none".
- "DATES: none" asserts that the request names no time at all. It is not the safe default and it is wrong for any request that names one, however briefly and whatever the request is otherwise about.
- The DATES line is where time is handled. Having put it there, write every query as though the request named no time at all: no date, no month, no year and no day-word on any query line.
- The examples below show the format of a reply. They do not show the range of requests this rule covers: every request that names a time gets a range, whatever its subject, wording or shape.

Rules:
- Do not restate or lightly reword the input request. Each query must approach the underlying information need from a genuinely different angle than the input and from each other.
- Do not answer the request. You are generating queries, not answers.
- Do not invent specifics (names, numbers, node ids) that are not implied by the request itself.
- A query line must never contain a date, a month, a year, or a day-word. A semantic search matches a date only where that date appears as verbatim text, which is nowhere. Time constraints go on the DATES line and nowhere else.
- Output exactly %d lines total:
  - The first line is the DATES line described above.
  - The next %d lines are distinct, standalone questions (each ending in "?") that name a mechanism, concept, or specific terminology likely to appear in the answer.
  - The last line is a dense, keyword-style query (no question mark) combining the most salient technical terms an embedding search would key on.
- Output ONLY those %d lines: the DATES line, then the queries. No numbering, no bullets, no quotes, no preamble, no commentary, no blank lines between them.

Examples of the desired shape (unrelated to the request you will be given):`

type derivationExemplar struct {
	input   string
	dates   string
	queries []string
}

var derivationExemplars = []derivationExemplar{
	{
		input: "Where is a Go test supposed to state what it is checking, and why not in a comment above it?",
		dates: "none",
		queries: []string{
			"How does a Go test carry what it pins?",
			"Why are comments not the place to state a test's intent?",
			"What does the comment contract say about test files?",
			"What names a test's intent so it survives a refactor?",
			"Go test name intent comment contract",
		},
	},
	{
		input: "What happened to a request that was still being worked on when the service was told to stop on 2026-09-05?",
		dates: "2026-09-05..2026-09-05",
		queries: []string{
			"How does the server drain requests in flight during shutdown?",
			"Is a run cancelled or allowed to finish when the process is told to stop?",
			"What bounds the graceful shutdown period?",
			"What happens to write-back while the process is shutting down?",
			"graceful shutdown in-flight request drain timeout",
		},
	},
}

var (
	derivationLinePrefix = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s*`)
	derivationThinkBlock = regexp.MustCompile(`(?is)<think>.*?</think>`)
)

const derivationQuoteCutset = "\"“”'"

const derivationTotalLines = MaxDerivedQueries + 1

const dateLinePrefix = "DATES:"

var dateRangePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.\.(\d{4}-\d{2}-\d{2})$`)

// DerivationPrompt renders the derivation instructions, the stated instant, the two format-only exemplars and the input as one prompt, with the input marked by a ===== REQUEST ===== banner so it is not mistaken for a third, unanswered exemplar.
func DerivationPrompt(input string, now time.Time) string {
	blocks := make([]string, 0, len(derivationExemplars)+3)
	blocks = append(blocks, fmt.Sprintf(derivationInstructions, MaxDerivedQueries, derivationTotalLines, MaxDerivedQueries-1, derivationTotalLines))

	if !now.IsZero() {
		blocks = append(blocks, "===== NOW =====\n"+now.Format(time.RFC3339))
	}

	for _, exemplar := range derivationExemplars {
		blocks = append(blocks, exemplar.input+"\n"+dateLinePrefix+" "+exemplar.dates+"\n"+strings.Join(exemplar.queries, "\n"))
	}

	return strings.Join(append(blocks, "===== REQUEST =====\n"+input), "\n\n")
}

// ParseDerivation reads text as query lines with an optional DATES directive among them, dropping reasoning artifacts, list decoration, blanks, repeats and case-folded echoes of input, capped at MaxDerivedQueries. loc resolves the window.
func ParseDerivation(text, input string, loc *time.Location) ([]string, UpdateWindow) {
	lines := strings.Split(derivationThinkBlock.ReplaceAllString(text, ""), "\n")
	window := windowFromDirectiveLines(lines, loc)

	remaining := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := directiveValue(reduceDerivationLine(line)); ok {
			continue
		}
		remaining = append(remaining, line)
	}

	return parseQueryLines(remaining, input), window
}

func windowFromDirectiveLines(lines []string, loc *time.Location) UpdateWindow {
	directiveCount := 0
	firstContentIsDirective := false
	firstContentSeen := false
	var value string

	for _, line := range lines {
		reduced := reduceDerivationLine(line)
		if reduced == "" {
			continue
		}
		v, ok := directiveValue(reduced)
		if !firstContentSeen {
			firstContentSeen = true
			firstContentIsDirective = ok
		}
		if ok {
			directiveCount++
			value = v
		}
	}

	if directiveCount != 1 || !firstContentIsDirective {
		return UpdateWindow{}
	}
	return resolveDatesValue(value, loc)
}

func reduceDerivationLine(line string) string {
	return strings.Trim(derivationLinePrefix.ReplaceAllString(strings.TrimSpace(line), ""), derivationQuoteCutset)
}

func directiveValue(reduced string) (string, bool) {
	if len(reduced) < len(dateLinePrefix) || !strings.EqualFold(reduced[:len(dateLinePrefix)], dateLinePrefix) {
		return "", false
	}
	return strings.TrimSpace(reduced[len(dateLinePrefix):]), true
}

func resolveDatesValue(value string, loc *time.Location) UpdateWindow {
	if value == "none" {
		return UpdateWindow{}
	}

	m := dateRangePattern.FindStringSubmatch(value)
	if m == nil {
		return UpdateWindow{}
	}

	from, err := time.ParseInLocation("2006-01-02", m[1], loc)
	if err != nil {
		return UpdateWindow{}
	}
	to, err := time.ParseInLocation("2006-01-02", m[2], loc)
	if err != nil {
		return UpdateWindow{}
	}
	to = to.AddDate(0, 0, 1)

	if !to.After(from) {
		return UpdateWindow{}
	}
	return UpdateWindow{From: from, To: to}
}

func parseQueryLines(lines []string, input string) []string {
	seen := map[string]bool{derivationKey(input): true}
	queries := make([]string, 0, MaxDerivedQueries)

	for _, line := range lines {
		if len(queries) == MaxDerivedQueries {
			break
		}

		query := reduceDerivationLine(line)
		key := derivationKey(query)
		if key == "" || seen[key] {
			continue
		}

		seen[key] = true
		queries = append(queries, query)
	}

	return queries
}

func derivationKey(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

// MergeQueries is input first, then each derived query that is not already present.
func MergeQueries(input string, derived []string) []string {
	queries := []string{input}
	for _, query := range derived {
		if slices.Contains(queries, query) {
			continue
		}
		queries = append(queries, query)
	}
	return queries
}

// DeriveQueries asks model for queries and a retrieval window derived from input and now under DerivationBound, returning a cause naming which deadline fired when one did.
func DeriveQueries(ctx context.Context, model ModelPort, input string, now time.Time) ([]string, UpdateWindow, error) {
	return deriveQueries(ctx, model, input, now, DerivationBound)
}

func deriveQueries(ctx context.Context, model ModelPort, input string, now time.Time, bound time.Duration) ([]string, UpdateWindow, error) {
	bounded, cancel := context.WithTimeoutCause(ctx, bound, errDerivationBound)
	defer cancel()

	started := time.Now()
	text, err := model.Derive(bounded, DerivationPrompt(input, now), DerivationBudget)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil {
		return nil, UpdateWindow{}, derivationFailure(bounded, bound, elapsed, err)
	}

	derived, window := ParseDerivation(text, input, now.Location())
	if len(derived) == 0 {
		return nil, UpdateWindow{}, fmt.Errorf("%w after %s", errDerivationUnusable, elapsed)
	}

	return derived, window, nil
}

func derivationFailure(ctx context.Context, bound, elapsed time.Duration, err error) error {
	switch cause := context.Cause(ctx); {
	case errors.Is(cause, errDerivationBound):
		return fmt.Errorf("the derivation's own %s bound expired after %s: %w", bound, elapsed, err)
	case errors.Is(cause, context.DeadlineExceeded):
		return fmt.Errorf("the enclosing run bound expired during derivation after %s: %w", elapsed, err)
	case cause != nil:
		return fmt.Errorf("the enclosing context ended during derivation after %s (%v): %w", elapsed, cause, err)
	default:
		return fmt.Errorf("the derivation call failed after %s: %w", elapsed, err)
	}
}
