package loop

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func canonicalDerivationReply(datesLine string) string {
	lines := make([]string, 0, derivationTotalLines)
	lines = append(lines, datesLine)

	questionCount := derivationTotalLines - 2
	for i := 1; i <= questionCount; i++ {
		lines = append(lines, fmt.Sprintf("round trip question %d, distinct from every other line here?", i))
	}

	return strings.Join(append(lines, "round trip dense keyword line"), "\n")
}

func assertRoundTripsToTheCap(t *testing.T, reply, input string) UpdateWindow {
	t.Helper()

	if got := len(strings.Split(reply, "\n")) - 1; got != MaxDerivedQueries {
		t.Fatalf("the compliant reply carries %d non-directive lines, want exactly %d: parseQueryLines will silently discard the rest", got, MaxDerivedQueries)
	}

	queries, window := ParseDerivation(reply, input, time.UTC)
	if len(queries) != MaxDerivedQueries {
		t.Fatalf("a %d-line reply compliant with the prompt's own stated shape parsed to %d queries, want %d", derivationTotalLines, len(queries), MaxDerivedQueries)
	}
	return window
}

func TestAReplyCompliantWithTheDerivationPromptsOwnConstantsRoundTripsThroughParseDerivationProvingShapeAgreementNotModelCompliance(t *testing.T) {
	t.Parallel()

	const input = "what does the round trip between the prompt and the parser actually cover"

	t.Run("a dated DATES line yields the capped query count and a non-zero window", func(t *testing.T) {
		t.Parallel()

		reply := canonicalDerivationReply(dateLinePrefix + " 2026-09-17..2026-09-17")
		window := assertRoundTripsToTheCap(t, reply, input)

		if window.IsZero() {
			t.Fatalf("a reply carrying a dated DATES line as its first content line parsed to a zero window, want a non-zero one")
		}
	})

	t.Run("a DATES: none line and a value ParseDerivation cannot resolve both yield the capped query count and a zero window", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"none", "not a recognisable date range"} {
			t.Run(value, func(t *testing.T) {
				t.Parallel()

				reply := canonicalDerivationReply(dateLinePrefix + " " + value)
				window := assertRoundTripsToTheCap(t, reply, input)

				if !window.IsZero() {
					t.Fatalf("a reply carrying DATES: %s parsed to a non-zero window %+v, want zero", value, window)
				}
			})
		}
	})
}
