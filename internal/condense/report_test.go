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
