package loop

import (
	"fmt"
	"strings"
	"testing"
)

func canonicalDerivationReply() string {
	lines := make([]string, 0, MaxDerivedQueries)

	questionCount := MaxDerivedQueries - 1
	for i := 1; i <= questionCount; i++ {
		lines = append(lines, fmt.Sprintf("round trip question %d, distinct from every other line here?", i))
	}

	return strings.Join(append(lines, "round trip dense keyword line"), "\n")
}

func TestAReplyCompliantWithTheDerivationPromptsOwnConstantsRoundTripsThroughParseDerivationProvingShapeAgreementNotModelCompliance(t *testing.T) {
	t.Parallel()

	const input = "what does the round trip between the prompt and the parser actually cover"
	reply := canonicalDerivationReply()

	if got := len(strings.Split(reply, "\n")); got != MaxDerivedQueries {
		t.Fatalf("the compliant reply carries %d lines, want exactly %d: parseQueryLines will silently discard the rest", got, MaxDerivedQueries)
	}

	queries := ParseDerivation(reply, input)
	if len(queries) != MaxDerivedQueries {
		t.Fatalf("a %d-line reply compliant with the prompt's own stated shape parsed to %d queries, want %d", MaxDerivedQueries, len(queries), MaxDerivedQueries)
	}
}
