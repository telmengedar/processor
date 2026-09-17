package loop

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

var derivationRoundTripNow = time.Date(2026, 9, 17, 11, 5, 0, 0, time.UTC)

func canonicalDerivationReply(datesLine string) string {
	lines := make([]string, 0, derivationTotalLines)
	lines = append(lines, datesLine)

	questionCount := derivationTotalLines - 2
	for i := 1; i <= questionCount; i++ {
		lines = append(lines, fmt.Sprintf("round trip question %d, distinct from every other line here?", i))
	}

	return strings.Join(append(lines, "round trip dense keyword line"), "\n")
}

func TestACompliantReplyToTheRenderedDerivationPromptRoundTripsThroughParseDerivationProvingShapeAgreementNotModelCompliance(t *testing.T) {
	t.Parallel()

	const input = "what does the round trip between the prompt and the parser actually cover"
	prompt := DerivationPrompt(input, derivationRoundTripNow)

	for _, want := range []string{
		fmt.Sprintf("produce %d additional queries", MaxDerivedQueries),
		fmt.Sprintf("Output exactly %d lines total:", derivationTotalLines),
		fmt.Sprintf("The next %d lines are distinct", MaxDerivedQueries-1),
		fmt.Sprintf("Output ONLY those %d lines:", derivationTotalLines),
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the rendered derivation prompt does not say %q; a reply built to what this prompt actually asks for has nothing to build against", want)
		}
	}

	t.Run("a dated DATES line yields the capped query count and a non-zero window", func(t *testing.T) {
		t.Parallel()

		reply := canonicalDerivationReply(dateLinePrefix + " 2026-09-17..2026-09-17")
		queries, window := ParseDerivation(reply, input, time.UTC)

		if len(queries) != MaxDerivedQueries {
			t.Fatalf("a %d-line reply compliant with the prompt's own stated shape parsed to %d queries, want %d", derivationTotalLines, len(queries), MaxDerivedQueries)
		}
		if window.IsZero() {
			t.Fatalf("a reply carrying a dated DATES line as its first content line parsed to a zero window, want a non-zero one")
		}
	})

	t.Run("a DATES: none line yields the same capped query count and a zero window", func(t *testing.T) {
		t.Parallel()

		reply := canonicalDerivationReply(dateLinePrefix + " none")
		queries, window := ParseDerivation(reply, input, time.UTC)

		if len(queries) != MaxDerivedQueries {
			t.Fatalf("a %d-line reply compliant with the prompt's own stated shape parsed to %d queries, want %d", derivationTotalLines, len(queries), MaxDerivedQueries)
		}
		if !window.IsZero() {
			t.Fatalf("a reply carrying \"DATES: none\" parsed to a non-zero window %+v, want zero", window)
		}
	})
}
