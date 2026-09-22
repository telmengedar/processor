package ports

import (
	"bytes"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/loop"
)

const reportRunBound = 10 * time.Minute

func declared(tokensPerSecond, promptBytesPerSecond float64) boot.FloorsConfig {
	return boot.FloorsConfig{TokensPerSecond: &tokensPerSecond, PromptBytesPerSecond: &promptBytesPerSecond}
}

func report(t *testing.T, floorsConfig boot.FloorsConfig, fillConfigured bool) []string {
	t.Helper()

	floors, sites, err := Affordability(boot.ProtocolOpenAICompat, floorsConfig, reportRunBound, fillConfigured)
	if err != nil {
		t.Fatalf("Affordability: %v", err)
	}

	var logged bytes.Buffer
	StateAffordability(slog.New(slog.NewTextHandler(&logged, nil)), floors, sites)

	return strings.Split(strings.TrimRight(logged.String(), "\n"), "\n")
}

func lineFor(t *testing.T, lines []string, site string) string {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, "site="+site) {
			return line
		}
	}
	t.Fatalf("no boot line reports the %q call site; lines=%v", site, lines)
	return ""
}

func attribute(t *testing.T, line, key string) string {
	t.Helper()
	for _, field := range strings.Fields(line) {
		if name, value, ok := strings.Cut(field, "="); ok && name == key {
			return value
		}
	}
	t.Fatalf("the boot line carries no %s attribute; line=%s", key, line)
	return ""
}

func floatAttribute(t *testing.T, line, key string) float64 {
	t.Helper()
	value, err := strconv.ParseFloat(attribute(t, line, key), 64)
	if err != nil {
		t.Fatalf("the %s attribute is %q, which is not a rate a reader can act on: %v", key, attribute(t, line, key), err)
	}
	return value
}

func warnings(lines []string) []string {
	var warned []string
	for _, line := range lines {
		if strings.Contains(line, "level=WARN") {
			warned = append(warned, line)
		}
	}
	return warned
}

func TestBootWarnsAboutNothingWhereTheDeclaredFloorsAffordEveryCallSiteIncludingTheFill(t *testing.T) {
	t.Parallel()

	lines := report(t, declared(1_000, 100_000), true)

	if warned := warnings(lines); len(warned) != 0 {
		t.Fatalf("a deployment declaring floors that afford every site was warned about %d of them, which is a check firing on a configuration that is fine: %v", len(warned), warned)
	}
	for _, site := range []string{loop.SiteDerivation, loop.SiteJudgement, loop.SiteFill} {
		line := lineFor(t, lines, site)
		if !strings.Contains(line, "level=INFO") {
			t.Fatalf("the %s site is not reported at INFO on a configuration that affords it; line=%s", site, line)
		}
		if headroom := attribute(t, line, "headroom"); headroom == "" || strings.HasPrefix(headroom, "-") {
			t.Fatalf("the %s site reports a headroom of %q, so the report states no margin an operator can read; line=%s", site, headroom, line)
		}
	}
}

func TestBootWarnsAboutAJudgementCallItsDeclaredFloorCannotAffordAndNamesTheRateTheHostWouldHaveToDeliver(t *testing.T) {
	t.Parallel()

	const declaredTokensPerSecond = 1.0

	lines := report(t, declared(declaredTokensPerSecond, 3_000), false)
	line := lineFor(t, lines, loop.SiteJudgement)

	if !strings.Contains(line, "level=WARN") {
		t.Fatalf("a judgement budget the declared floor cannot afford was reported at %s, not as a warning; line=%s", attribute(t, line, "level"), line)
	}

	required := floatAttribute(t, line, "requiredTokensPerSecond")
	if required <= declaredTokensPerSecond {
		t.Fatalf("the report names %v tokens per second as what the host would have to deliver, which is not above the %v it declared: a check that only says the host is too slow is the defect this check exists to remove; line=%s",
			required, declaredTokensPerSecond, line)
	}
	if got := floatAttribute(t, line, "declaredTokensPerSecond"); got != declaredTokensPerSecond {
		t.Fatalf("the report states the deployment declared %v tokens per second, want %v", got, declaredTokensPerSecond)
	}
	if !strings.Contains(line, "would have to deliver at least") {
		t.Fatalf("the warning does not say in words what the host would have to deliver; line=%s", line)
	}
	if shortfall := attribute(t, line, "shortfall"); strings.HasPrefix(shortfall, "-") {
		t.Fatalf("the shortfall reads %q, which is a headroom wearing the wrong name; line=%s", shortfall, line)
	}
}

func TestTheRateTheHostWouldHaveToDeliverIsComputedRatherThanEchoedBackFromTheDeclaration(t *testing.T) {
	t.Parallel()

	first := floatAttribute(t, lineFor(t, report(t, declared(1, 3_000), false), loop.SiteJudgement), "requiredTokensPerSecond")
	second := floatAttribute(t, lineFor(t, report(t, declared(2, 3_000), false), loop.SiteJudgement), "requiredTokensPerSecond")

	if first != second {
		t.Fatalf("two deployments declaring different generation floors are told they need %v and %v tokens per second; the rate a budget needs is a property of the budget and the bound, not of what the operator declared", first, second)
	}
}

func TestTheFillSiteIsReportedOnlyWhereACondensationModelIsConfigured(t *testing.T) {
	t.Parallel()

	with := report(t, declared(1_000, 100_000), true)
	without := report(t, declared(1_000, 100_000), false)

	if len(with) != len(without)+1 {
		t.Fatalf("configuring a condensation model changed the report by %d lines, want exactly one; with=%v without=%v", len(with)-len(without), with, without)
	}
	for _, line := range without {
		if strings.Contains(line, "site="+loop.SiteFill) {
			t.Fatalf("a deployment with no condensation model is told about a fill site it will never call; line=%s", line)
		}
	}
}

func TestBootStatesTheDeclaredFloorsThemselvesAsDeclarationsRatherThanAsMeasurements(t *testing.T) {
	t.Parallel()

	lines := report(t, declared(13, 4_400), false)

	if !strings.Contains(lines[0], "tokensPerSecond=13") || !strings.Contains(lines[0], "promptBytesPerSecond=4400") {
		t.Fatalf("the first boot line does not state the floors this deployment declared; line=%s", lines[0])
	}
	if !strings.Contains(lines[0], "not measurements") {
		t.Fatalf("the first boot line does not say that these rates are the deployment's assertions rather than readings taken from it; line=%s", lines[0])
	}
}

func TestADeploymentThatDeclaresNoFloorIsCheckedAgainstTheProductsOwnDeclaration(t *testing.T) {
	t.Parallel()

	floors, _, err := Affordability(boot.ProtocolOpenAICompat, boot.FloorsConfig{}, reportRunBound, false)
	if err != nil {
		t.Fatalf("Affordability: %v", err)
	}

	want := loop.Floors{TokensPerSecond: loop.DefaultFloorTokensPerSecond, PromptBytesPerSecond: loop.DefaultFloorPromptBytesPerSecond}
	if floors != want {
		t.Fatalf("floors = %+v, want the product's own declaration %+v", floors, want)
	}
}

func TestADeploymentsOwnDeclarationIsPreferredToTheProductsAndReachesTheCheck(t *testing.T) {
	t.Parallel()

	floors, _, err := Affordability(boot.ProtocolOpenAICompat, declared(41, 5_300), reportRunBound, false)
	if err != nil {
		t.Fatalf("Affordability: %v", err)
	}

	want := loop.Floors{TokensPerSecond: 41, PromptBytesPerSecond: 5_300}
	if floors != want {
		t.Fatalf("floors = %+v, want the deployment's own declaration %+v", floors, want)
	}
}

func TestNoAffordabilityIsClaimedForAProtocolThisBinaryHasNoAdapterFor(t *testing.T) {
	t.Parallel()

	if _, err := ClientBound("responses"); err == nil {
		t.Fatal("a protocol with no adapter reported a client bound, which is a bound nothing enforces")
	}

	_, sites, err := Affordability("responses", boot.FloorsConfig{}, reportRunBound, false)
	if err == nil {
		t.Fatalf("a protocol with no adapter produced %d call sites, want a refusal", len(sites))
	}
	if !strings.Contains(err.Error(), "responses") {
		t.Fatalf("err = %v, want it to name the protocol it was given", err)
	}
}

func TestTheFillSitesBudgetIsTheLargestOneCondensationCanBeIssuedWith(t *testing.T) {
	t.Parallel()

	if got := FillBudgetCeiling(); got <= loop.JudgementBudget {
		t.Fatalf("the fill site is checked at %d tokens, which is no larger than the judgement site's %d — a ceiling sized from the largest content a fill accepts cannot be that small", got, loop.JudgementBudget)
	}
}

func TestTheFillSitesCeilingIsNotAffordableAtTheProductsOwnDeclaredFloorsAndIsReportedRatherThanRaised(t *testing.T) {
	t.Parallel()

	lines := report(t, boot.FloorsConfig{}, true)
	line := lineFor(t, lines, loop.SiteFill)

	if warned := warnings(lines); len(warned) != 0 {
		t.Fatalf("the default shape of a fill-configured deployment raises %d warnings on every boot: %v — a permanent alarm is one a reader learns to filter, and the site it would hide is the judgement site this check exists for", len(warned), warned)
	}
	if !strings.Contains(line, "level=INFO") {
		t.Fatalf("the fill site is not reported at INFO; its budget is sized per node by the condensation pass and is not the deployment's to set, so nothing an operator can do would clear a warning about it; line=%s", line)
	}
	if !strings.Contains(line, "not this deployment's to set") {
		t.Fatalf("the fill line does not say why an unaffordable result here is reported rather than raised; line=%s", line)
	}
	if strings.HasPrefix(attribute(t, line, "shortfall"), "-") {
		t.Fatalf("the fill line reports a negative shortfall, which is a headroom wearing the wrong name; line=%s", line)
	}
	if required := floatAttribute(t, line, "requiredTokensPerSecond"); required <= loop.DefaultFloorTokensPerSecond {
		t.Fatalf("the fill line names %v tokens per second as the rate the host would need, which is not above the %v the product declares; the number must survive the drop to INFO", required, loop.DefaultFloorTokensPerSecond)
	}
}

func TestASiteWhoseBudgetTheDeploymentCanActOnIsStillRaisedRatherThanOnlyReported(t *testing.T) {
	t.Parallel()

	lines := report(t, declared(1, 3_000), true)

	judgement := lineFor(t, lines, loop.SiteJudgement)
	if !strings.Contains(judgement, "level=WARN") {
		t.Fatalf("dropping the deferred site to INFO also silenced the judgement site, which is the one a deployment can act on; line=%s", judgement)
	}
	if strings.Contains(judgement, "not this deployment's to set") {
		t.Fatalf("the judgement site is reported with the deferred site's framing, so a condition the operator owns reads as one nobody owns; line=%s", judgement)
	}
	if fill := lineFor(t, lines, loop.SiteFill); strings.Contains(fill, "level=WARN") {
		t.Fatalf("the fill site is raised as a warning even where the deployment declared a floor it cannot meet; line=%s", fill)
	}
}

func judgementSentence(t *testing.T, tokensPerSecond, promptBytesPerSecond float64) string {
	t.Helper()
	return lineFor(t, report(t, declared(tokensPerSecond, promptBytesPerSecond), false), loop.SiteJudgement)
}

func TestWhereBothARateAndAPromptRateWouldAffordTheSiteTheReportNamesBoth(t *testing.T) {
	t.Parallel()

	line := judgementSentence(t, 6.4, 2666.67)

	if floatAttribute(t, line, "requiredTokensPerSecond") <= 0 || floatAttribute(t, line, "requiredPromptBytesPerSecond") <= 0 {
		t.Fatalf("this fixture is meant to leave both halves fixable on their own, and one of them reads as the sentinel; line=%s", line)
	}
	for _, want := range []string{"would have to deliver at least", "or process the prompt at"} {
		if !strings.Contains(line, want) {
			t.Fatalf("the report does not state %q where both rates would afford the site; line=%s", want, line)
		}
	}
}

func TestWhereOnlyAFasterGenerationRateWouldAffordTheSiteTheReportSaysSoRatherThanPrintingTheSentinel(t *testing.T) {
	t.Parallel()

	line := judgementSentence(t, 2, 3_000)

	if got := floatAttribute(t, line, "requiredPromptBytesPerSecond"); got != 0 {
		t.Fatalf("this fixture is meant to leave the prompt half unfixable, and requiredPromptBytesPerSecond reads %v; line=%s", got, line)
	}
	if strings.Contains(line, "at 0 bytes per second") {
		t.Fatalf("the report prints the zero sentinel as a prompt rate, so a clause every host on earth already exceeds reads as the fix; line=%s", line)
	}
	if !strings.Contains(line, "would have to deliver at least") {
		t.Fatalf("the report does not name the generation rate the host would have to deliver; line=%s", line)
	}
	if !strings.Contains(line, "no prompt rate affords it on its own") {
		t.Fatalf("the report does not say that no prompt rate affords the site, which is what the zero sentinel means; line=%s", line)
	}
}

func TestWhereOnlyAFasterPromptRateWouldAffordTheSiteTheReportNamesThatRateAlone(t *testing.T) {
	t.Parallel()

	line := judgementSentence(t, 10_000, 1_000)

	if got := floatAttribute(t, line, "requiredTokensPerSecond"); got != 0 {
		t.Fatalf("this fixture is meant to leave the generation half unfixable, and requiredTokensPerSecond reads %v; line=%s", got, line)
	}
	if !strings.Contains(line, "no generation rate affords it on its own") {
		t.Fatalf("the report does not say that no generation rate affords the site; line=%s", line)
	}
	if !strings.Contains(line, "process the prompt at") {
		t.Fatalf("the report does not name the prompt rate the host would have to read at; line=%s", line)
	}
	if strings.Contains(line, "would have to deliver at least 0") {
		t.Fatalf("the report prints the zero sentinel as a generation rate; line=%s", line)
	}
}

func TestWhereNeitherRateAffordsTheSiteOnItsOwnTheReportSaysThatRatherThanNamingZero(t *testing.T) {
	t.Parallel()

	line := judgementSentence(t, 1, 100)

	if floatAttribute(t, line, "requiredTokensPerSecond") != 0 || floatAttribute(t, line, "requiredPromptBytesPerSecond") != 0 {
		t.Fatalf("this fixture is meant to leave both halves unfixable on their own; line=%s", line)
	}
	if !strings.Contains(line, "neither a faster generation rate nor a faster prompt rate affords it alone") {
		t.Fatalf("the report does not state that each half already claims the whole bound, so both sentinels print as rates; line=%s", line)
	}
	if strings.Contains(line, "at 0 bytes per second") || strings.Contains(line, "at least 0.0 tokens per second") {
		t.Fatalf("the report prints a zero sentinel as a rate; line=%s", line)
	}
}
