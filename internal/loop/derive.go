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

// MaxDerivedQueries caps how many derived queries one turn issues beside the input.
const MaxDerivedQueries = 5

// MaxPhraseBytes is the longest text, in UTF-8 bytes and inclusive, that is a query: a phrase naming what a step is about.
const MaxPhraseBytes = 256

// DerivationBound is the derivation step's own deadline, well below the run bound because its failure costs only a fallback.
const DerivationBound = 30 * time.Second

var errDerivationBound = errors.New("derivation bound")

var errDerivationUnusable = errors.New("the model returned no usable query")

const derivationInstructions = `You generate alternate search queries for a semantic retrieval system.

You are given ONE user request. Your job is to produce %d additional queries that a semantic (embedding-based) search engine could use to surface the specific documentation that answers that request. The request itself is already one query the search runs; your job is to add angles that request does not cover.

Think about what actually helps a semantic search here: the request is often phrased the way a confused or informal user would phrase it, while the documentation that answers it is phrased the way an author states a ruling, a mechanism, or a design decision. A good derived query bridges that gap -- it surfaces the underlying mechanism, the specific technical terms, the named concept, or the class of problem the request is really an instance of, using vocabulary closer to how documentation states things.

Rules:
- Do not restate or lightly reword the input request. Each query must approach the underlying information need from a genuinely different angle than the input and from each other.
- Do not answer the request. You are generating queries, not answers.
- Do not invent specifics (names, numbers, node ids) that are not implied by the request itself.
- Output exactly %d lines total:
  - The first %d lines are distinct, standalone questions (each ending in "?") that name a mechanism, concept, or specific terminology likely to appear in the answer.
  - The last line is a dense, keyword-style query (no question mark) combining the most salient technical terms an embedding search would key on.
- Output ONLY those %d lines. No numbering, no bullets, no quotes, no preamble, no commentary, no blank lines between them.

Examples of the desired shape (unrelated to the request you will be given):`

type derivationExemplar struct {
	input   string
	queries []string
}

var derivationExemplars = []derivationExemplar{
	{
		input: "Where is a Go test supposed to state what it is checking, and why not in a comment above it?",
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

const ignoredLinePrefix = "DATES:"

// DerivationPrompt renders the derivation instructions, the two format-only exemplars and the input as one prompt, with the input marked by a ===== REQUEST ===== banner so it is not mistaken for a third, unanswered exemplar.
func DerivationPrompt(input string) string {
	blocks := make([]string, 0, len(derivationExemplars)+2)
	blocks = append(blocks, fmt.Sprintf(derivationInstructions, MaxDerivedQueries, MaxDerivedQueries, MaxDerivedQueries-1, MaxDerivedQueries))

	for _, exemplar := range derivationExemplars {
		blocks = append(blocks, exemplar.input+"\n"+strings.Join(exemplar.queries, "\n"))
	}

	return strings.Join(append(blocks, "===== REQUEST =====\n"+input), "\n\n")
}

// IsPhrase reports whether text can be a graph query: non-blank, and at most MaxPhraseBytes bytes as it stands.
func IsPhrase(text string) bool {
	return strings.TrimSpace(text) != "" && len(text) <= MaxPhraseBytes
}

// ParseDerivation reads text as query lines, dropping reasoning artifacts, list decoration, blanks, any line that starts with DATES:, lines longer than a phrase, repeats and case-folded echoes of input, capped at MaxDerivedQueries.
func ParseDerivation(text, input string) []string {
	lines := strings.Split(derivationThinkBlock.ReplaceAllString(text, ""), "\n")

	remaining := make([]string, 0, len(lines))
	for _, line := range lines {
		if hasIgnoredPrefix(reduceDerivationLine(line)) {
			continue
		}
		remaining = append(remaining, line)
	}

	return parseQueryLines(remaining, input)
}

func reduceDerivationLine(line string) string {
	return strings.Trim(derivationLinePrefix.ReplaceAllString(strings.TrimSpace(line), ""), derivationQuoteCutset)
}

func hasIgnoredPrefix(reduced string) bool {
	return len(reduced) >= len(ignoredLinePrefix) && strings.EqualFold(reduced[:len(ignoredLinePrefix)], ignoredLinePrefix)
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
		if key == "" || seen[key] || !IsPhrase(query) {
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

// MergeQueries is input first when input is a phrase, then each derived query that is not already present; a longer input is content for the model and is never a query.
func MergeQueries(input string, derived []string) []string {
	queries := make([]string, 0, 1+len(derived))
	if IsPhrase(input) {
		queries = append(queries, input)
	}
	for _, query := range derived {
		if slices.Contains(queries, query) {
			continue
		}
		queries = append(queries, query)
	}
	return queries
}

// DeriveQueries derives queries from input under DerivationBound, returning the sampling the request carried, nil where none was issued; an error names the deadline that fired, when one did.
func DeriveQueries(ctx context.Context, model ModelPort, input string) ([]string, *Sampling, error) {
	return deriveQueries(ctx, model, input, DerivationBound)
}

func deriveQueries(ctx context.Context, model ModelPort, input string, bound time.Duration) ([]string, *Sampling, error) {
	bounded, cancel := context.WithTimeoutCause(ctx, bound, errDerivationBound)
	defer cancel()

	started := time.Now()
	result, err := model.Derive(bounded, DerivationPrompt(input), noOutputLimit)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil {
		return nil, result.Sampling, derivationFailure(bounded, bound, elapsed, err)
	}

	sampling := result.Sampling
	derived := ParseDerivation(result.Text, input)
	if len(derived) == 0 {
		return nil, sampling, fmt.Errorf("%w after %s", errDerivationUnusable, elapsed)
	}

	return derived, sampling, nil
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
