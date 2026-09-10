package condense

import (
	"encoding/json"
	"strings"
	"testing"
)

func renderedPass(t *testing.T, result Result) (string, string) {
	t.Helper()

	var machine, human strings.Builder
	if err := Render(result, &machine, &human); err != nil {
		t.Fatalf("want the render to succeed, got %v", err)
	}
	return machine.String(), human.String()
}

func passWithOneAudit(substance string, origin string) Result {
	return Result{
		TargetCount: 1,
		Provenance: []Provenance{{
			Node: 100, Name: "A ruling", ContentSize: 1000, SubstanceSize: 200, Ratio: 0.2,
			Model: "ai/gemma3", Sampling: Sampling{Temperature: floatValue(0), MaxTokens: 2048}, Written: true,
		}},
		Audit: []AuditEntry{{
			Node: 100, Name: "A ruling", Rows: []string{"r01"}, Origin: origin,
			Why:         []string{"an answer lacking this would present X, when the truth is Y"},
			ContentSize: 1000, SubstanceSize: 200, Ratio: 0.2, Substance: substance,
		}},
		Ratio: RatioSummary{Count: 1, Min: 0.2, Median: 0.2, Mean: 0.2, Max: 0.2},
	}
}

func floatValue(f float64) *float64 { return &f }

func TestTheAuditBundlePrintsEveryPreRegisteredReasonBesideTheSubstanceInFull(t *testing.T) {
	_, human := renderedPass(t, passWithOneAudit("the whole condensation, every byte of it", originGenerated))

	if !strings.Contains(human, "an answer lacking this would present X, when the truth is Y") {
		t.Fatalf("want the pre-registered reason in the bundle, got:\n%s", human)
	}
	if !strings.Contains(human, "the whole condensation, every byte of it") {
		t.Fatalf("want the substance in the bundle, got:\n%s", human)
	}
}

func TestTheAuditBundleStatesNoVerdictOfItsOwn(t *testing.T) {
	_, human := renderedPass(t, passWithOneAudit("a condensation", originGenerated))

	for _, verdict := range []string{"PASS", "FAIL", "passed", "failed"} {
		if strings.Contains(human, verdict) {
			t.Fatalf("the bundle is evidence and must render no verdict, found %q in:\n%s", verdict, human)
		}
	}
}

func TestARequiredNodeCarryingNoSubstanceIsShownAsSuchRatherThanOmitted(t *testing.T) {
	result := passWithOneAudit("", originAbsent)
	result.Audit[0].SubstanceSize = 0

	_, human := renderedPass(t, result)

	if !strings.Contains(human, "this node carries no substance") {
		t.Fatalf("want an explicit absence line, got:\n%s", human)
	}
}

func TestTheRatioTableReportsEveryNodeAndTheAggregateBeneathIt(t *testing.T) {
	_, human := renderedPass(t, passWithOneAudit("a condensation", originGenerated))

	if !strings.Contains(human, "0.200") {
		t.Fatalf("want the per-node ratio, got:\n%s", human)
	}
	if !strings.Contains(human, "median 0.200") {
		t.Fatalf("want the aggregate median, got:\n%s", human)
	}
}

func TestTheSamplingSectionSaysUnsetRatherThanZeroForAParameterThatWasNeverSent(t *testing.T) {
	_, human := renderedPass(t, passWithOneAudit("a condensation", originGenerated))

	if !strings.Contains(human, "top_p "+unsetSampling) {
		t.Fatalf("want an unsent top_p reported as unset, got:\n%s", human)
	}
}

func TestTheOutputCeilingRangeIsReportedBecauseItVariesWithEachNodesSize(t *testing.T) {
	result := passWithOneAudit("a condensation", originGenerated)
	result.Provenance = append(result.Provenance, Provenance{Node: 200, Sampling: Sampling{MaxTokens: 9000}})

	_, human := renderedPass(t, result)

	if !strings.Contains(human, "max_tokens 2048..9000") {
		t.Fatalf("want the ceiling range across the pass, got:\n%s", human)
	}
}

func TestTheSkipSectionGroupsNodesUnderTheRuleThatStoppedThem(t *testing.T) {
	result := passWithOneAudit("a condensation", originGenerated)
	result.Skipped = []Skip{
		{Node: 200, Reason: skipSubstancePresent},
		{Node: 300, Reason: skipSubstancePresent},
		{Node: 400, Reason: skipNotShorter},
	}

	_, human := renderedPass(t, result)

	if !strings.Contains(human, skipSubstancePresent) || !strings.Contains(human, "200 300") {
		t.Fatalf("want the two nodes grouped under one rule, got:\n%s", human)
	}
	if !strings.Contains(human, skipNotShorter) {
		t.Fatalf("want every rule listed, got:\n%s", human)
	}
}

func TestTheMachineOutputIsTheWholeResultAsJSON(t *testing.T) {
	machine, _ := renderedPass(t, passWithOneAudit("a condensation", originGenerated))

	var decoded Result
	if err := json.Unmarshal([]byte(machine), &decoded); err != nil {
		t.Fatalf("want decodable JSON on the machine channel, got %v", err)
	}
	if len(decoded.Audit) != 1 || decoded.Audit[0].Substance != "a condensation" {
		t.Fatalf("want the audit carried into the JSON, got %+v", decoded.Audit)
	}
	if decoded.Provenance[0].ContentSize != 1000 {
		t.Fatalf("want the provenance carried into the JSON, got %+v", decoded.Provenance)
	}
}

func TestAPassThatCondensedNothingSaysSoRatherThanPrintingAnEmptyTable(t *testing.T) {
	_, human := renderedPass(t, Result{TargetCount: 3})

	if !strings.Contains(human, "nothing was condensed") {
		t.Fatalf("want an explicit empty-pass line, got:\n%s", human)
	}
	if !strings.Contains(human, "no call completed") {
		t.Fatalf("want the sampling section to say nothing was read back, got:\n%s", human)
	}
}
func headingOver(t *testing.T, human, reason string) string {
	t.Helper()

	heading := ""
	for _, line := range strings.Split(human, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			heading = line
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), reason) {
			return heading
		}
	}

	t.Fatalf("want %q listed among the skips, got:\n%s", reason, human)
	return ""
}

func TestATruncatedCondensationIsNotListedUnderAHeadingThatCallsItARuleThePassApplied(t *testing.T) {
	result := Result{TargetCount: 2, Skipped: []Skip{
		{Node: 200, Reason: skipTruncated, Detail: "length"},
		{Node: 300, Reason: skipSubstancePresent},
	}}

	_, human := renderedPass(t, result)

	truncated := headingOver(t, human, skipTruncated)
	if strings.Contains(truncated, "rule") {
		t.Fatalf("a truncation is the pass exhausting its own output budget, yet the report files it under %q", truncated)
	}
	if rule := headingOver(t, human, skipSubstancePresent); rule == truncated {
		t.Fatalf("a rule the pass applied and a failure of the pass share the heading %q", truncated)
	}
}

func TestEverySkipIsListedUnderTheHeadingThatMatchesHowTheResultCountsIt(t *testing.T) {
	reasons := []string{
		skipNodeAbsent, skipContentAbsent, skipSubstancePresent, skipSelfProduced,
		skipNonProseContent, skipEmptyCondensation, skipNotShorter, skipBelowFloor,
		skipPreamble, skipContentMoved, skipTruncated, skipReadFailed,
		skipModelFailed, skipWriteFailed,
	}

	result := Result{TargetCount: len(reasons)}
	for i, reason := range reasons {
		result.Skipped = append(result.Skipped, Skip{Node: int64(100 + i), Reason: reason})
	}

	_, human := renderedPass(t, result)

	for _, reason := range reasons {
		want := ruleSkipHeading
		if (Result{Skipped: []Skip{{Reason: reason}}}).OperationalFailures() == 1 {
			want = failureSkipHeading
		}
		if got := headingOver(t, human, reason); got != want {
			t.Fatalf("the report files %q under %q while the count places it under %q", reason, got, want)
		}
	}
}

func TestASkipReasonNoConstantNamesIsAnnouncedToTheOperatorAsAFailureOfThePass(t *testing.T) {
	unclassified := "an unclassified skip reason"
	result := Result{TargetCount: 2, Skipped: []Skip{
		{Node: 700, Reason: unclassified},
		{Node: 800, Reason: skipSubstancePresent},
	}}

	_, human := renderedPass(t, result)

	if got := headingOver(t, human, unclassified); got != failureSkipHeading {
		t.Fatalf("a reason nothing has classified must be announced as a failure of the pass, yet the report files it under %q", got)
	}
	if got := headingOver(t, human, skipSubstancePresent); got != ruleSkipHeading {
		t.Fatalf("want the named rule beside it under %q, got %q", ruleSkipHeading, got)
	}
}

func TestAPassWhoseSkipsAreAllRulesPrintsNoHeadingClaimingAFailureOfThePass(t *testing.T) {
	result := Result{TargetCount: 1, Skipped: []Skip{{Node: 200, Reason: skipSubstancePresent}}}

	_, human := renderedPass(t, result)

	if strings.Contains(human, failureSkipHeading) {
		t.Fatalf("no skip here is a failure of the pass, yet the report announces a section of them:\n%s", human)
	}
}
