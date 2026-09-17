# The Reclaimed Budget Has No Owner — ruling §8 after F-2 and F-3

## TL;DR

**The form rule does not do what it was built to do, and §8.1 is not why.** §8.1 says which form is
rendered. It says nothing about what the reclaimed bytes are *for*, and `admit` — greedy first-fit in
rank order — is under no obligation to spend them on the rows the rule was built to un-starve. It spends
them on whatever comes next. On corpus row `r01` that was a 48,678 B node with no substance of its own.

**The fix is in admission, and half of it is already designed, measured and unshipped.**
`what-a-block-is-worth-per-byte.md` (on `main`, `5fb97ec`) independently reproduced this exact regression
on live data — *"arm ONE: 8 admitted → 7, one row worse"* — and diagnosed it in the same words this
document would have used: **"the form rule frees bytes and the existing allocator wastes them."** Its
remedy is a per-candidate size cap at `budget/k`, `k = 5`. **The two lines of work have collided and
nobody had said so out loud.** That is the single most important finding here.

| | ruling |
|---|---|
| **§8.1 (the form rule)** | **Unchanged. Do not rewrite, do not amend the predicate.** It is a correct statement about form. |
| **The defect** | **Allocation, not form.** Greedy first-fit is non-monotone under item shrinkage — a structural property, not a coverage artefact (§2 proves it survives full coverage). |
| **The remedy** | **The per-candidate size cap, shipped first**, exactly as `what-a-block-is-worth-per-byte.md` §7 specifies it. On the traced row it converts the failure into a strict improvement over *both* arms (§4). |
| **The form rule's PR** | **Land the mechanism at the dial's off position (threshold 0), not at 0.592.** Unit 1's precedent: ship the capability, change no bytes. Flip the dial after the cap, on a threshold F-3 re-selects. |
| **F-2** | **Binds on the candidate dispositions.** The required-node reading is a separate, weaker figure and is renamed F-2a so the two can never be confused again. |
| **F-3** | **Fired against an instrument that could not have shown an effect.** Admitted-row count is saturated by the budget; the curve is flat because the savings are re-spent, which *is* the defect. **Do not collapse the strata on this evidence.** |
| **R1 on a warm graph** | **F-1 qualifies a producer; the graph now holds artifacts with no producer attached.** Split the gate: F-1 (producer) + **F-11** (population). Neither touches §8.1; R5 stands. |

**What the evidence does not reach, stated once:** the corpus can currently see this unit's **harm** and
not its **benefit** — required-node admissions are 10 at every arm including off. Until a corpus row
exists whose required node is cut for byte budget and recovered by the rule, Unit 3 is a change whose
downside is measured and whose upside is projected. **F-13 is the cheapest way to find out, and it should
run before the dial leaves zero.**

## 1. The problem

`Assemble` renders every admitted candidate as its full content. One 43 KB document takes 77% of the
block and strands ten above-floor rows behind it. §8's form rule answers this by rendering a node's
`substance` where condensation materially compacts it.

Measured, the rule fires and the starvation does not lift. Worse: at the shipped budget, **8 of 25 corpus
rows lose between 1 and 6 previously-admitted candidates**, at every budget from 5 KB to 120 KB, and
moving the threshold does not help (8 lose at 0.592, 9 at 0.4, 6 at 0.2).

## 2. The defect is a property of greedy first-fit, and coverage does not close it

The measurement record attributes the regression to coverage gaps: rank 2 was *"a 48,678 B node with no
substance of its own."* **That is true of `r01` and false as a general claim.** Greedy first-fit is
non-monotone under item shrinkage regardless of what fraction of items shrink.

Worked counterexample, **every row carrying a substance**, no cap, budget 59,000 B:

| rank | content | rendered | rule off | rule on |
|---|---:|---:|---|---|
| A | 20,000 | **6,000** (good ratio) | admitted (20,000) | admitted (6,000) |
| B | 18,000 | 18,000 (bad ratio → content) | admitted (38,000) | admitted (24,000) |
| C | 15,000 | 15,000 | admitted (53,000) | admitted (39,000) |
| D | 15,000 | 15,000 | **cut** — 68,000 > 59,000 | **admitted** (54,000) |
| E | 6,000 | 6,000 | **admitted** (59,000) | **cut** — 60,000 > 59,000 |
| F | 3,000 | 3,000 | cut — 62,000 | admitted (57,000) |

**E goes from admitted to not-admitted at 100% coverage.** So *"coverage will rise, and the regression is
a coverage artefact"* is not an argument that survives.

**The general statement, which §8 should carry:** the form rule reduces some items' charge; greedy
first-fit's admitted set is not monotone in item sizes, because freed budget can admit an item that was
previously refused and that item can consume more than was freed. **Nothing in §8.1 binds the freed bytes
to the rows they were freed for, and `admit` has no reason to.**

## 3. A second design on `main` already measured this and reached the same diagnosis

`what-a-block-is-worth-per-byte.md`, committed as `5fb97ec`:

| its finding | figure |
|---|---|
| **F5** | *"Unit 3 alone changes the six-query block by nothing at all… On the raw input it makes the block **worse** (8 admitted → 7)."* |
| **§6.1** | *"Rendering `#13101` at 5,562 B instead of 12,027 B frees space that greedy-by-rank immediately hands to a larger document… **the form rule frees bytes and the existing allocator wastes them.**"* |
| **F3/§6.2** | *"A ratio multiplies an unbounded quantity. It compresses; it cannot bound."* |
| **F6/§7.3** | A cap at `budget/5 = 12,000 B` removes the quality inversion on both arms, raises the raw-input block from **8 admitted to 14**, and drops the largest single share from **77.2% to 16.4%**. |
| **§7.4** | *"The cap is not a competitor to the payload change; it is its trigger."* A node over the cap in content and under it in substance is **admitted**, where the cap alone loses it. |

**Two documents, two instruments, two authors, one mechanism.** Neither cited the other. That convergence
is stronger evidence than either measurement alone, and it is why this document proposes no new mechanism.

## 4. What the cap does to the traced row

Derived from the measurement record's own published figures for `r01` (cap = 12,000 B):

| rank | content | substance | today | form rule alone | cap alone | **cap + form** |
|---|---:|---:|---|---|---|---|
| 1 | 9,037 | 2,565 | admitted | admitted | admitted | admitted (2,565) |
| 2 | 48,678 | — | cut, byte budget | **admitted — eats the block** | **cut, oversized** | **cut, oversized** |
| 3 | 17,578 | 1,129 | admitted | admitted | **cut, oversized** | **admitted (1,129)** |
| 4–11 | 26,705 | — | admitted | **cut, byte budget** | admitted | admitted |
| | | | **8 rows** | **4 rows** | **7 rows** | **8 rows, ≈23 KB spare** |

1. **The cap alone converts the failure into a pass on this row.**
2. **The cap alone has its own cost:** it loses a 17.5 KB row that is admitted today.
3. **Only the composition keeps everything** — all eight of today's rows, the blocker refused, and
   genuinely reclaimed budget for ranks 12+.

## 5. F-3's curve is flat because the instrument is saturated

The metric is *candidate rows admitted*, and the budget is binding — the measurement record says so in its
own voice: *"admitted bytes barely move (+0.8%). The budget is the binding constraint, so bytes the rule
frees are immediately re-spent rather than banked."*

**A metric saturated by the defect cannot measure the dial that feeds the defect.** That is not evidence
the strata are decoration; it is §2's finding read off a different column.

What the curve *does* say: **rows rendered lossily rise 29 → 67 across the sweep while admitted rows stay
within 4 of each other.** Between two thresholds equal on outcome, the lower carries less fidelity
exposure — a decision rule made of measured quantities. It points **below** 0.592, and cannot be applied
until the allocator stops swallowing the savings.

**Fixing admission is also what makes F-3 measurable.** The two findings are the same finding.

## 6. The decision

> **§8.1 is correct and stays as written. The reclaimed budget has no owner, and giving it one is an
> admission change. The per-candidate size cap already specified in `what-a-block-is-worth-per-byte.md`
> §7 is that change, and it ships before the form rule's dial leaves zero.**

| # | Ruling | Why |
|---|---|---|
| **D1** | **§8.1 unchanged.** No size conjunct, no coverage conjunct, no fill conjunct, no provenance conjunct. | Every proposal that amends it either fails to close the mechanism or buys closure with a worse rule. |
| **D2** | **Ship the cap first**, exactly as §7 specifies: after form choice, before the byte budget, one new cut reason, charging zero, carrying the cap in force on the disposition. | It is measured to remove the inversion on both live arms *today*. **Shipping Unit 3 first ships a measured regression in exchange for an unmeasured benefit.** |
| **D3** | **Land the form rule at the dial's off position.** | Unit 1's precedent: *"the block is byte-identical whether or not a candidate carries a substance"* — the property that made it safe to ship alone. |

### 6.1 The reclamation invariant — the named fallback, not adopted now

The cap **bounds** the regression; it does not eliminate it. A residual case exists where every row is
under the cap and a row is still lost. The available guarantee:

> **The reclamation invariant.** With the cap and the candidate list held fixed, the form rule may only
> add rows to the admitted set; it may never remove one.

**Mechanism:** admission runs as two passes over the same ranked list. The *baseline* pass applies
self-produced, floor and cap exactly as today and tests the byte budget against each row's **content**
size — so its admitted set is by construction identical to the rule-off set. The *reclamation* pass then
computes actual consumption from **rendered** sizes and walks only the byte-budget-cut rows, in rank
order, admitting any whose rendered payload fits the remainder.

**Why not now:** its subject has already been given up (the cap deliberately violates
`admitted ⊇ today's admitted`); it adds a second allocation concept to a function three documents insist
stays one pass with one reason per row; and on the one row traced, the cap closes the mechanism outright.

**The trigger is written down rather than left to judgement:** if F-2 still reports any row going
admitted → not-admitted **with the cap in force**, adopt the reclamation pass.

## 7. The alternatives, by name

**Option A — ship as-is behind the dial.** *Loses, and it is a way of not answering.* §8.2 did not
withdraw the invariant; it **converted** it — *"That is a real cost and §11's F-2 is what replaces it."*
Citing the withdrawal to excuse failing F-2 argues that the replacement for the proof is satisfied by the
absence of the proof. And coverage does not close it (§2). **Decisive third limb:** the rule's own
motivation is the starvation of *unlabelled* rows. **You cannot cite the starvation of unlabelled rows as
the reason for the rule and then dismiss the loss of unlabelled rows as not counting.**

**Option B — gate on coverage.** *Loses on its premise.* §2 shows the regression survives 100% coverage,
so there is no fill rate above which the rule behaves as intended. It also makes `Assemble` a function of
the corpus, so two identical turns render differently because the graph moved.

**Option C — change what admission does with reclaimed space.** **ADOPTED.** The form decisions are
identical across arms; only the admitted set differs; the mechanism is budget re-spend.

**Option D — collapse the strata.** *Loses on the instrument, not the idea* (§5). What would be lost: the
threshold is the only place the design expresses measured fidelity risk. A single rule spends a 1-in-24
fidelity risk on the 0.886 stratum to save ~11% of bytes — the trade §8.2 explicitly rejected.

**Option E — couple the rule to the fill.** *Loses on effectiveness, checked against the traced row.*
`r01`'s rank 1 is 9,037 B, above `FillSizeFloor = 8_000`, so it still renders as substance and rank 2
still fits. **The coupling does not touch the mechanism.** The intuition behind it is right and is
answered by the cap.

## 8. The four seams, settled

**Seam 1 — empty content with a substance present.** *The rule does not fire. Content form.* The predicate
is *"materially smaller than content"*; with no content there is no comparison, and a rule stated over a
ratio must not invent a value for an undefined one. It also keeps `contentHash` from pairing the empty
string's hash with non-empty rendered bytes — R4's failure by another door. Already observable:
`substanceAvailable: true, size: 0, form: content, renderedSize: 0`.

**Seam 2 — the boundary is strict.** The reason is not the boundary — it is **the off position**. Under
`≤`, a zero-length substance on non-empty content yields `0 ≤ 0` and would render an empty payload at
threshold 0, the one state the dial depends on being inert. **The paired string presence test is
load-bearing**, and a later refactor that folds it into the ratio breaks the off position silently.

**Seam 3 — F-2 binds on the admitted candidate set.** It fails. The required-node reading is renamed
**F-2a** and reported alongside, never as a substitute — the rule's motivating harm is the starvation of
unlabelled rows, so a metric blind to them is blind to the thing this exists for. **F-2a passing while
F-2 fails is informative, not exculpatory:** the loss landed on rows the corpus does not label.

**Seam 4 — R1 splits.** F-1 qualifies a **producer**; the graph holds **artifacts with no producer
attached**. The fill merged 2026-09-17; the 67 substances carry `lastUpdate` 2026-09-16 — **they are not
ours**. Add **F-11**, a one-time population audit. **R5 does not change:** R5 governs the *node*, artifact
trust governs the *derived form*, and artifact trust is expressed as a system-level gate on whether the
dial may leave zero, never as a per-candidate branch. **Residual, accepted explicitly:** rendering a
third-party substance inherits third-party trust, and content cannot be unfaithful to itself where a
substance can. There is no mechanism that closes this without a provenance field the graph lacks.

## 9. What must be measured

| # | Gate | Statement |
|---|---|---|
| **F-13** | **Can the instrument see the prize?** Required-node admissions are 10 at every arm including off, and 2 labelled + 1 control required node are retrieved and not admitted at every arm. Identify them; state what cut each, whether it carries a substance, and whether any arm admits it. **Cheapest of all, and it discriminates between Option B and Option C.** |
| **F-CAP** | *(exists, unchanged)* Sweep the corpus with the cap on and off at several budgets. Any required document going admitted → not-admitted means the cap's value is wrong. |
| **F-2** | *(restated)* With the cap in force and the candidate list fixed, sweep with the form rule on and off. **Zero candidates may go admitted → not-admitted.** If any remain, adopt §6.1's reclamation pass. |
| **F-2a** | *(renamed)* No required node goes admitted → not-admitted. Reported with F-2, never instead. |
| **F-3** | *(restated)* Three columns — candidate rows admitted, required-node verdicts, **and rows rendered lossily** — against the **capped** allocator. Falsifier re-armed. |
| **F-11** | *(new)* An independent reader checks each of the 67 live substances against its own content. **Zero tolerance** on any node the eval corpus depends on. **Must pass before the dial leaves zero.** |

**Order: F-13 → F-CAP → F-2 / F-2a / F-3 against the capped allocator → F-11 → dial off zero.**

## 10. Milestones

- **M0** — run **F-13**. One read of a sweep that has already run. If the corpus cannot see the prize,
  that fact goes in the PR body before anything ships.
- **M1** — the form rule lands at the off position. *(Done: `da0568e`.)*
- **M2** — **the cap**, specified **against the rendered payload, not against content**, from the day it
  ships — even while content is the only payload there is.
- **M3** — re-measure F-2, F-2a, F-3 against the capped allocator. **If F-2 still reports a loss, adopt
  §6.1 as M3a** — the trigger is stated in advance so it is not a judgement call.
- **M4** — **F-11**, the population audit.
- **M5** — the dial leaves zero, to a value F-3 selected.

**What must not happen:** no dial off zero before F-11 and F-2 are green; no cap whose rule is written
against content; no collapse of the strata citing the pre-cap curve; no dial value derived from the 67
before they are audited; no reclamation pass before F-2 has fired against the capped allocator; **no
presentation of the cap and the form rule as alternatives.**

## 11. Where the evidence does not reach

1. **Seven of the eight regressing rows have not been traced.** `r01` is the only one with a mechanism
   attached. F-2 after the cap is what turns expectation into knowledge.
2. **Nobody has measured the benefit.** Required-node admissions are 10 at every arm including off. **F-13
   first.**
3. **§4's composition table is derived arithmetic**, not a sweep. It should be reproduced by running both
   mechanisms, not quoted from here.
4. **The provenance of the 67 is uninvestigated, not unanswerable.** That distinction matters: this is
   work nobody has done, not a wall.
5. **The threshold's eventual value is not decided here.** §5 records a prediction (below 0.592) precisely
   so it can be falsified.

## 12. Open questions

| # | Question | Blocking? |
|---|---|---|
| **N1** | Who wrote the 67, in fact? Not answerable from the graph; likely answerable from what we ran. | No — F-11 audits the artifacts regardless. |
| **N2** | Should a contentless candidate be admitted at all? It charges zero and renders an empty section. | No — raise against `admit`, not §8. |
| **N3** | `what-a-block-is-worth-per-byte.md` has no owner and no branch, and D2 makes it the critical path. | **Yes, for sequencing.** |
| **N4** | §11's F-3 wording should name the saturation caveat. | No — one sentence. |
| **N5** | Rank order as the arbiter of reclaimed space. The cap bounds the damage; it does not make the spend principled. | No — its own unit, after F-2 is green. |

**In one line.** The form rule as specified does not do what it was built to do — not because §8.1 is
wrong, but because §8.1 is silent on what the savings are for, and a greedy allocator will spend them on
the next large row. **§8 needs amending, not rewriting**, and the amendment belongs to admission.
