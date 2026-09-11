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

You will be given ONE user request. Your job is to produce %d additional queries that a semantic (embedding-based) search engine could use to surface the specific documentation that answers that request. The request itself is already one query the search runs; your job is to add angles that request does not cover.

Think about what actually helps a semantic search here: the request is often phrased the way a confused or informal user would phrase it, while the documentation that answers it is phrased the way an author states a ruling, a mechanism, or a design decision. A good derived query bridges that gap -- it surfaces the underlying mechanism, the specific technical terms, the named concept, or the class of problem the request is really an instance of, using vocabulary closer to how documentation states things.

Rules:
- Do not restate or lightly reword the input request. Each query must approach the underlying information need from a genuinely different angle than the input and from each other.
- Do not answer the request. You are generating queries, not answers.
- Do not invent specifics (names, numbers, node ids) that are not implied by the request itself.
- Output exactly %d lines:
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
		input: "What happens to a request that is still being worked on when the service is told to stop?",
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

// DerivationPrompt renders the derivation instructions, the two format-only exemplars and input as one prompt.
func DerivationPrompt(input string) string {
	blocks := make([]string, 0, len(derivationExemplars)+2)
	blocks = append(blocks, fmt.Sprintf(derivationInstructions, MaxDerivedQueries, MaxDerivedQueries, MaxDerivedQueries-1, MaxDerivedQueries))

	for _, exemplar := range derivationExemplars {
		blocks = append(blocks, exemplar.input+"\n"+strings.Join(exemplar.queries, "\n"))
	}

	return strings.Join(append(blocks, input), "\n\n")
}

// ParseDerivation reads text as one query per line, dropping reasoning artifacts, list decoration, blanks, repeats and case-folded echoes of input, capped at MaxDerivedQueries.
func ParseDerivation(text, input string) []string {
	seen := map[string]bool{derivationKey(input): true}
	queries := make([]string, 0, MaxDerivedQueries)

	for _, line := range strings.Split(derivationThinkBlock.ReplaceAllString(text, ""), "\n") {
		if len(queries) == MaxDerivedQueries {
			break
		}

		query := strings.Trim(derivationLinePrefix.ReplaceAllString(strings.TrimSpace(line), ""), derivationQuoteCutset)
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

// DeriveQueries asks model for queries derived from input under DerivationBound, returning a cause naming which deadline fired when one did.
func DeriveQueries(ctx context.Context, model ModelPort, input string) ([]string, error) {
	return deriveQueries(ctx, model, input, DerivationBound)
}

func deriveQueries(ctx context.Context, model ModelPort, input string, bound time.Duration) ([]string, error) {
	bounded, cancel := context.WithTimeoutCause(ctx, bound, errDerivationBound)
	defer cancel()

	started := time.Now()
	text, err := model.Derive(bounded, DerivationPrompt(input), MaxOutputTokens)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil {
		return nil, derivationFailure(bounded, bound, elapsed, err)
	}

	derived := ParseDerivation(text, input)
	if len(derived) == 0 {
		return nil, fmt.Errorf("%w after %s", errDerivationUnusable, elapsed)
	}

	return derived, nil
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
