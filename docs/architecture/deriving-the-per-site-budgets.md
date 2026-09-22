# The Derivation of the Per-Site Output Budgets

> **Unit 2 (D-2) of** `the-budget-and-the-bound-were-never-reconciled.md`. That document is the ruling; this
> one is the record §14 Unit 2 step 2 asks for — *"each derived from the affordability property against the
> declared floor and the bound that binds. Record the derivation."*
> **Written at** `main` = `d5dee97`. **No number in the ruling was transcribed into code**; §14.1 forbids it,
> and every figure below is derived from declarations chosen here on their own merits.

---

## 1. The property, as implemented

The ruling states it once:

> **P1 (affordability).** For every model call the product can issue:
> `budget ÷ declared_floor_rate + prompt_allowance < bound_that_will_cut_it × safety_factor`.

Implemented in `internal/loop/budget.go` as `Afford`, checked at boot by `internal/ports.StateAffordability`,
and priced again at runtime by the loop's remaining-time guard (`Turn.judgementShortfall`).

### 1.1 One deviation, named

**The ruling writes `prompt_allowance` as a single declared duration. It is implemented as a per-site
quantity**: each site declares its own prompt ceiling in bytes, and the allowance is that ceiling divided by
a second declared floor, in bytes per second.

The reason is arithmetic, not taste. A single allowance cannot serve both ends of the range the product
already has:

- The **derivation** call sends its instructions and the run's input — thousands of bytes — inside a **30 s**
  bound.
- The **judgement** call sends the assembled block — tens of thousands of bytes — inside a bound an order of
  magnitude larger.

Any single allowance large enough to be honest about the judgement prompt exceeds the derivation's whole
bound, which would make the derivation site unaffordable at *any* budget and force it off — and D-4 rules
that the step survives. Any allowance small enough for the derivation site understates the judgement prompt
by a factor the check exists to catch. Two declared rates and per-site ceilings keep the inequality exactly
as the ruling states it, with an allowance that is derived rather than guessed.

---

## 2. What the operator declares

Two rates, both configuration, both reported at boot **as declarations rather than measurements**:

| Declaration | Variable | Shipped default | Why this number |
|---|---|---|---|
| generation floor | `PROCESSOR_MODEL_FLOOR_TOKENS_PER_SECOND` | **10 tok/s** | A round number chosen to sit below every generation rate this product has been observed at, so a host that fails it is pathological rather than merely slow. It is **not** any measured rate and is not derived from one |
| prompt floor | `PROCESSOR_MODEL_FLOOR_PROMPT_BYTES_PER_SECOND` | **3 000 B/s** | Same shape: a round, deliberately conservative assertion about prompt processing on the class of host this product targets |

**These are the product's declarations made on a deployment's behalf, not properties of the world.** A
deployment that knows better overrides them; a deployment that overrides them badly is told so at boot, with
the rate it would have to deliver instead. This is how C-1 is satisfied without pretending the rate is
unknowable — §5 of the ruling.

**Safety factor: 0.8**, a loop constant (`AffordabilitySafety`). A call site may plan to claim four fifths of
its bound; the remaining fifth is for everything the two declarations do not model — queueing, connection
setup, the tail of a slow draw.

---

## 3. The bound that binds each site

[derived-4] is the reason this is not simply the adapter's client bound. A judgement call is cut by whichever
of two bounds arrives first, and on a run that uses its call allowance the run bound arrives first:

```
judgement share = (RunBound − DerivationBound − MaxFills × FillBound) ÷ MaxModelCalls
                = (600 s − 30 s − 2 × 90 s) ÷ 6
                = 65 s

judgement bound = min(client bound 300 s, 65 s) = 65 s
```

The derivation and the fill are each cut by their own bound, which the run bound already reserves for them in
full: **30 s** and **90 s**.

**Every input to that arithmetic is a constant the product already declared.** Nothing new is asserted about
time; what is new is that the product now says out loud what its own bounds imply.

---

## 4. The arithmetic, per site

The affordable maximum is `floor × (bound × safety − prompt ÷ prompt floor)`.

| site | prompt ceiling | bound | allowed (×0.8) | prompt cost | generation time left | affordable max |
|---|---|---|---|---|---|---|
| derivation | 12 000 B | 30 s | 24 s | 4 s | 20 s | **200 tokens** |
| judgement | 80 000 B | 65 s | 52 s | 26.667 s | 25.333 s | **253 tokens** |

Prompt ceilings, and where each comes from:

- **derivation — 12 000 B.** Its instructions and exemplars measure 4 155 B at this ref; the ceiling leaves
  more room again for the run's input. Guarded by `TestTheDerivationPromptCeilingLeavesRoomForARunsInputBesideItsOwnInstructions`.
- **judgement — 80 000 B** = `AssemblyByteBudget + SupplementaryByteBudget`: the assembled block, plus one
  supplementary round carried back into the prompt. It is a **declared** ceiling, not a stacked worst case —
  §6 says what the worst case does instead.

### 4.1 The rounding rule, stated so it can be checked

> **Each shipped budget is the largest multiple of 32 tokens that satisfies P1 while leaving at least 10 % of
> the site's allowed time spare.**

| site | shipped budget | generation | total need | spare | spare share |
|---|---|---|---|---|---|
| derivation | **160** | 16 s | 20 s | 4 s | 16.7 % |
| judgement | **192** | 19.2 s | 45.867 s | 6.133 s | 11.8 % |

One step up fails in both cases — derivation at 192 leaves 3.3 %, judgement at 224 leaves 5.6 %. That is
what `TestNoCallSitesBudgetCouldBeOneStepLargerAndStillLeaveItsSpareShare` asserts, in both directions: a
budget too large fails the spare share, and a budget too small fails because the next step up would have
passed.

---

## 5. Why the judgement figure is below the ruling's illustration

§7.2 illustrates *"roughly 600–800 output tokens per call"*. The shipped value is **192**, and the difference
is entirely in the inputs, not in the method:

1. §7.2 solved P1 at the **observed** 18.2 tok/s. This derivation solves it at a **declared floor of 10**,
   which is the point of the declaration — [derived-2] establishes that the operating rate is *below* 14.6,
   so a budget derived from 18.2 would still fail.
2. §7.2's illustration does not charge prompt processing against each of the six calls. This derivation does:
   at the declared prompt floor, one judgement prompt costs 26.667 s of a 52 s allowance — **more than half
   the site's budget is spent before a token is generated.**

**That second line is the uncomfortable finding of this unit, and it is not a rounding detail.** It says the
shipped configuration spends most of its per-call time re-reading a block it has already sent. The ruling
names the levers (§9.4, §16.1's smaller block; §8.6's streaming) and this unit is allowed to pull none of
them.

### 5.1 The evidence that arrived after the ruling, and what it does and does not license

Measured on `d5dee97` after Unit 1 (recorded at **#14675**, one probe): a full-block run **completed in
129 s** having used **273 output tokens**, where the same shape previously died at the five-minute client
bound having spent the whole 4 096. *The rate did not improve; the model stopped.*

That is evidence the ruling did not have, and it bears on this derivation in exactly one direction: **a
budget in the low hundreds is not obviously starving a call site whose observed consumption across a whole
run was 273 tokens.** It licenses nothing stronger. It is n = 1, it is a total rather than a per-call figure,
and **no number in it is in the code** — 192 comes from the inequality, not from 273.

**What it does not settle** is §12 Q-2: whether a several-hundred-token answer is enough for a report-form
probe. That question is now live and cheap to answer, because a run that wants more room will say so —
`stopReason: truncated` on the record, rather than a wall at a bound.

---

## 6. What the boot check does and does not do

**It reports every site; it raises an alarm only where the deployment can act on one; it does not refuse.**
All three are permitted by §14 Unit 2 step 3, which requires the headroom be *reported* per site and says
nothing about the level it is reported at.

**Why not refuse.** A pessimistic declaration is a wrong guess, not an outage. The floors are operator
assertions; a service that will not start because an operator was cautious converts a guess into downtime.

**Why a site can be unaffordable and still not raise an alarm.** A `CallSite` carries `Deferred`, marking a
site whose budget **this product does not choose**. The fill site is the one such site today: §7.2 defers its
budget to #14268 — sized per node from the content, by `internal/condense`, against no bound at all — and its
ceiling is **not affordable at any plausible declared floor**. A WARN there would fire on every start of every
fill-configured deployment, forever, naming a condition **no reader can act on**; and the cost of that is
specific rather than aesthetic — the day the *judgement* site becomes unaffordable, filtering `site=fill` is
already muscle memory, and the alarm this unit exists to make meaningful is the one that gets filtered with
it. So a deferred site is reported at INFO, with the reason it is not raised in the line itself, and with the
required rates intact. Everything an operator can actually change still raises a WARN.

Three guards hold that shape:
`TestTheFillSitesCeilingIsNotAffordableAtTheProductsOwnDeclaredFloorsAndIsReportedRatherThanRaised` (the
default fill-configured boot raises **zero** warnings, and the fill line still carries the rate),
`TestASiteWhoseBudgetTheDeploymentCanActOnIsStillRaisedRatherThanOnlyReported` (dropping the deferred site to
INFO did not silence the judgement site), and
`TestBootWarnsAboutNothingWhereTheDeclaredFloorsAffordEveryCallSiteIncludingTheFill` (no warning on a
configuration that is fine). The day #14268 lands, the first of those reddens rather than quietly going stale.

**The report names the rate the host would have to deliver** — `requiredTokensPerSecond`, and its prompt-side
twin. That is the discriminating half: a check that only says *too slow* is the defect this unit exists to
remove.

**Both required rates are sentinels when no rate on that side affords the site**, and the sentence must never
print a sentinel as a rate: *"process the prompt at 0 bytes per second"* is a clause every host already
satisfies, so it reads as trivially fixable at exactly the moment nothing on that side can fix it. The
sentence therefore branches on **both** sentinels, and each of its four branches is pinned by a literal
fragment (`TestWhereBothARateAndAPromptRateWouldAffordTheSiteTheReportNamesBoth`,
`TestWhereOnlyAFasterGenerationRateWouldAffordTheSiteTheReportSaysSoRatherThanPrintingTheSentinel`,
`TestWhereOnlyAFasterPromptRateWouldAffordTheSiteTheReportNamesThatRateAlone`,
`TestWhereNeitherRateAffordsTheSiteOnItsOwnTheReportSaysThatRatherThanNamingZero`).

### 6.1 The stacked worst case, and why the check does not use it

The judgement prompt's true ceiling over a whole turn is the block plus *every* supplementary round —
`AssemblyByteBudget + (MaxModelCalls − 1) × SupplementaryByteBudget`. At the declared prompt floor that alone
exceeds the 65 s share, so a check built on it would refuse every configuration and discriminate nothing.

**The runtime guard covers that case instead.** Before each judgement call the loop prices one call at the
declared floors and compares it against the run's own remaining time; a call the run cannot afford is not
made, and the turn ends carrying the arithmetic on the record (`timeShortfall`) rather than dying at a wall.
That is §8.4's accepted half, and it is what makes the boot check's declared ceiling honest rather than
optimistic.

**A shortfall is filed under its own sentinel.** `ErrRunTimeExhausted`, not `ErrModelUnavailable`: the model
was never called, so nothing about it was unavailable, and a 502 *"the model call did not complete"* would be
the ruling's own complaint one layer out — true, and not the finding. It surfaces as **504** under the
existing `run_deadline_exceeded` code, whose message names that no call was made and carries the arithmetic.
The code is deliberately not a sixth one: the envelope's closed set is a design decision this unit is not
taking, and a consumer switching on the code wants the same answer for both — the run ran out of time.

---

## 7. What this unit does not fix

- **`MaxModelCalls` is now the binding constraint**, per #14675: the run spends all six calls on recall and
  produces no answer. §14.1 forbids touching a bound, the remedy is a design question with competing shapes,
  and it is filed. Nothing here makes it better or worse: this unit changes what one call may *generate*, not
  how many calls a turn may make.
- **The fill site's budget** — #14268, above.
- **Prompt cost itself** — §9.4 / §16.1's smaller block, and §8.6's streaming. Named, not scheduled.

## 8. Reading a record written before this change

`Limits` gains `derivationBudget` and `judgementBudget` and loses `maxOutputTokens`. A record stored before
this change carries neither new key, and JSON decoding leaves both at zero — so the run summary renders an
absent budget as **an em dash, never as `0`**. A budget of zero is a claim that the run was allowed to
generate nothing, which no run has ever been; §3.6 of the ruling is titled *"stated so nobody reads a gap as a
zero"* and this is the same obligation one field along. `ledger.DialExercise` already distinguishes absence
from value by reading the raw record, and needs nothing here beyond its two new dial rows.
