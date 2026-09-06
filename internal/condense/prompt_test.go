package condense

import (
	"regexp"
	"strings"
	"testing"
)

func TestThePromptCarriesTheNodesNameAsTheDocumentTitleAndItsBodyBetweenTheTags(t *testing.T) {
	prompt := Prompt("A ruling on comments", "the body of the node")

	if !strings.Contains(prompt, `<document title="A ruling on comments">`) {
		t.Fatalf("want the node name as the document title, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "the body of the node\n</document>") {
		t.Fatalf("want the body immediately before the closing tag, got:\n%s", prompt)
	}
}

func TestThePromptEndsWithTheDocumentSoNoInstructionFollowsTheBody(t *testing.T) {
	prompt := Prompt("a name", "a body")

	if !strings.HasSuffix(prompt, "</document>") {
		t.Fatalf("want the document last, got a prompt ending %q", prompt[len(prompt)-40:])
	}
}

func TestTheSupersessionClauseIsAppendedOnlyWhereTheBodyRecordsASupersession(t *testing.T) {
	const marker = "what it replaced"

	if strings.Contains(Prompt("n", "plain prose that triggers no clause at all"), marker) {
		t.Fatalf("the supersession clause must not be sent to a body that triggers no rule")
	}
	for _, body := range []string{"a ~~struck~~ claim", "Correction 2026-09-05", "superseded by a later ruling", "previously read as follows"} {
		if !strings.Contains(Prompt("n", body), marker) {
			t.Fatalf("want the supersession clause for body %q", body)
		}
	}
}

func TestTheDerivableDataClauseIsAppendedOnlyWhereTheBodyCarriesATable(t *testing.T) {
	const marker = "the rule is the substance"

	if strings.Contains(Prompt("n", "prose that merely mentions a | pipe mid-line"), marker) {
		t.Fatalf("a pipe that does not open a line must not trigger the derivable-data clause")
	}
	if !strings.Contains(Prompt("n", "intro\n| a | b |\n|---|---|\n"), marker) {
		t.Fatalf("want the derivable-data clause for a body carrying a markdown table")
	}
}

func TestBothConditionalClausesAppearTogetherWhereTheBodyTriggersBoth(t *testing.T) {
	prompt := Prompt("n", "a ~~struck~~ claim\n| a | b |\n")

	if !strings.Contains(prompt, "what it replaced") || !strings.Contains(prompt, "the rule is the substance") {
		t.Fatalf("want both conditional clauses, got:\n%s", prompt)
	}
}

func TestTheClauseListMatchesTheClausesThePromptActuallyCarried(t *testing.T) {
	if got := Clauses("plain prose"); len(got) != 0 {
		t.Fatalf("want no clauses named for plain prose, got %v", got)
	}
	got := Clauses("a ~~struck~~ claim\n| a | b |\n")
	if len(got) != 2 || got[0] != clauseSupersession || got[1] != clauseDerivableData {
		t.Fatalf("want both clauses named in order, got %v", got)
	}
}

func TestThePromptNamesNoNodeOrTicketAnywhereInIt(t *testing.T) {
	reference := regexp.MustCompile(`#\d+`)
	prompt := Prompt("a name", "a body")

	if match := reference.FindString(prompt); match != "" {
		t.Fatalf("the prompt must carry no node or ticket reference, found %q", match)
	}
}
