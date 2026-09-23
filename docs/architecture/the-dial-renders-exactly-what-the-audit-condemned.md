# Architectural Document: The dial renders exactly what the audit condemned — what produces substance, what warrants it, and when the loop may render it

> **Status: proposal, revision 2. Read-only throughout.** No code changed, no inference run, no graph
> mutated, no git executed. Baseline: `main` = `73dfcce`, read on
> `feat/the-reclaimed-budget-needs-an-owner`. Every figure below is quoted from the document or node that
> measured it and attributed in place; **nothing is re-asserted as new measurement**
> (`how-we-know-a-change-helped.md` §12.5 applied to this document).
>
> **REVISION 2, same day — against #14712 round 2 (§§7–12), which landed while revision 1 was being
> written and partly reverses the premise revision 1 was briefed on.** Revision 1's central positive
> claim — *"the ratio is an economics dial, not a fidelity instrument"* — **is withdrawn**, and so is the
> **size ceiling** it proposed and the **provisional θ ≈ 0.15** it recommended. §13a records that error
> under its own heading rather than absorbing it quietly, because it is the same error §8.1 made and
> #14712 §4 caught, reproduced one level down by the person who had just finished naming it. **What
> survives, and survives strengthened, is §4.2's arithmetic, §4.4's empty intersection, §4.5's collision,
> D2, D3 and D4.** Struck material is struck in place, not deleted.
>
> **Supersedes nothing. Amends `what-goes-in-the-block.md` §8.1, §8.1.1 and §8.3, and the F-11 row of its
> §11.** §11's F-11 is *discharged* by this document, with a verdict.
>
> **Sources:** DiVoid **#14712** (F-11, rounds 1 **and 2**), **#14713** (F-CAP, the admission sweep),
> **#14282** (Unit 3's off-dial landing), **#12984** (the condensation-ratio measurement),
> **#14561** / `how-we-know-a-change-helped.md` (the compliance-independence rule),
> `what-goes-in-the-block.md`, `what-a-block-is-worth-per-byte.md`,
> `a-falsifier-measured-on-an-empty-set.md`, and the tree at `73dfcce`
> (`internal/loop/assemble.go`, `fill.go`, `turn.go`, `internal/condense/*`, `internal/fill/*`,
> `internal/divoid/substance.go`, `internal/divoid/write.go`, `internal/boot/config.go`).

---

## TL;DR

1. **F-11 is discharged with a verdict of NO, and the verdict is not "11 of 23 failed". It is an arithmetic
   fact about the dial.** Cross #14712 §3's verdicts against #12984's ratios on the same nodes: at the
   sweep's own recommended threshold **θ = 0.592, the eight eval-corpus substances the form rule renders are
   seven FAIL and one BORDERLINE. Zero PASS.** The first PASS enters only above θ = 0.780, by which point
   **nine of the eleven FAILs are already in.**

2. **Round 2 explains why, and the explanation generalises beyond the producer revision 1 blamed.**
   Whole-population screens over all 398 non-corpus substances establish: **accuracy tracks the producer;
   completeness tracks compression, and they are separable.** Enumerated-item loss by ratio band: **16%
   below 0.05 · 8% at 0.05–0.10 · 1% at 0.10–0.25.** **Completeness improves monotonically with ratio, for
   both producers.** So low ratio does not merely *correlate* with loss on one stratum — **compression is
   the mechanism of loss, everywhere.**

3. **Therefore the shipped rule has its sign backwards, and that is a design defect rather than a tuning
   problem.** §8.1 renders substance *wherever it is small enough*. The measurement says **small is the
   danger**. **The admissible region is a band — `floor ≤ ratio < threshold` — and a single-sided dial
   cannot express it. The more aggressively that dial is tuned, the more precisely it selects the
   substances that have lost content.**

4. **Both bounds have different owners, and only one of them is about fidelity.** The **floor** is
   completeness (measured knee ≈ **0.10**, above which item loss falls to 1%). The **ceiling** is
   **economics only** — a substance that saves little is not worth a form change — because completeness gets
   *better*, not worse, as the ratio rises. §8.1 gives fidelity reasons for both of its strata; both are
   wrong, and the upper one is wrong in the direction that makes the rule dangerous.

5. **On today's population the band is `0.10 ≤ ratio < 0.25`, and it lands on a striking coincidence:
   0.25 is just below the *entire* condense stratum** (its minimum is node 10943's **0.2501**). So the band
   admits the prose stratum's measured-safe region — including large nodes: 14642 at 24,861 B / 0.185 and
   13669 at 37,256 B / 0.229 both PASS — **and excludes every one of the 21 condense substances by
   arithmetic, with no producer field involved.** That coincidence is a proxy and is labelled as one
   (F-W3).

6. **Prohibitions have no safe ratio, and they are the highest-harm loss class.** Reader-directed
   prohibition retention: **7% · 12% · 12% · 55% (≥0.25)** for prose, **69%** for condense — and the screen
   over-counts by roughly a third. **A majority of `Do not` / `must not` clauses do not survive into the
   substance at any ratio the form rule would use**, and 14136 is the worked harm: *"Do not read this REJECT
   as a reason to hold the production fix overnight"* is absent, so a reader acting on the substance holds
   the fix. **This gets its own check, from the content side, because no ratio bound reaches it.**

7. **The trust mechanism is verification at render time over the pair — not an audit, not a marker, not a
   producer qualification — and round 2 makes the case decisive rather than argued.** §11.4: the graph
   carries **no producer field on a substance** and **no substance-to-content binding**. So no rule *can*
   key on producer even if S2 permitted it. Three checks, all deterministic over bytes already in hand,
   all compliance-independent (C1): **accuracy** (token containment), **directive retention**, **ratio
   band**. Plus a **constructive** tier for artifacts that are verbatim spans of their own content.

8. **The accuracy screen already exists and has already passed its positive control.** #14712 §8 ran it:
   over the 21 condense substances it returns **exactly** `PROCESSOR_DIVOVOID_URL` and
   `LllValidationService` and nothing else — the two corruptions §3.1 found by hand. **0 fabrications in
   398 non-corpus substances against 2 corruptions in 21 condense substances.** Its false-refusal cost is
   also measured: 51 flagged rows on the prose stratum, every one adjudicated, **none a fabrication** —
   and five were real repo symbols the content states only in prose, one of them *more precise than its own
   content*. A false refusal costs **bytes, not correctness**, so the screen refuses — with the
   normalisations the adjudication itself identifies.

9. **`cmd/condense` is narrowed, and revision 2 gives a better reason than revision 1 had.** It is **not**
   condemned as a producer class: at matched ratio it retains prohibitions *better* than the prose stratum
   (69% vs 55%). **Its real defect is that its completeness is purchased by not compressing.** At ratio
   ~0.72 it saves 28% — below any economics threshold that makes a form change worth making — and when
   pushed to where it would pay (≥ 8 KB → median 0.298) it is **5 of 5 clause-level FAIL**. **It is not bad
   at condensing; it is bad at condensing *enough*, and where it would pay it fails.** Its one remaining
   route into the band is an **extractive** form, which is also the only non-compliance-dependent repair
   available (D1, F-W7).

10. **The fill, as shipped, is a machine for manufacturing exactly the defective class, and it is inert only
    by configuration.** `FillSizeFloor = 8_000` fires it **only** above the size at which its producer
    (`condense.Run`, via `internal/fill/fill.go`) measured 0 of 5. It does not fire today only because
    `PROCESSOR_CONDENSE_MODEL_URL` is unset. **That must become off-by-construction.**

11. **The two falsifiers still have an empty intersection, and revision 2 sharpens it to 0.0001.**
    F-CAP limb 2 needs **θ > 0.2501** to rescue node 10943; the band's ceiling is **≤ 0.25**. Both bounds
    are set by the *same document from opposite sides*, and **10943 is an F-11 FAIL** — its substance stops
    five enumerated members early while still asserting *"the complete set is the symmetry table at the end
    of this note."* **The cap's recovery path is the silent substitution of a document that reads as
    finished and is not.** No cell of the two-dial grid is safe today.

12. **#14713's benefit and #14712's failures are the same artifacts.** Same 25 rows; the corpus's only
    substances are the condense output #14712 condemns. Its recommendation table is withdrawn as a basis
    for choosing θ or `k`. And the instrument is inverted: the corpus is **91% condense-produced** against
    a graph that is **90% agent-written** — it measures the stratum the product will not use.

13. **The 11 defective substances are not touched.** Eval-corpus nodes are instruments; the gate makes them
    inert without a byte changing; deleting them would destroy the reproducibility #14713 itself relied on.

14. **What ships, in order, each alone, each never worse than today:** the three checks at the off position
    (inert; their product is a continuous measurement that replaces F-11 as a gate) → the excerpt contract
    split out of `substance` → the fill's producer closed by construction → run-record backfill re-composed
    so its warrant becomes provable → a warranted substance population for the eval corpus as a **fixture,
    not a graph write** → only then the band, and only then the composition and `k`.

---

## 1. Problem Statement

`Node.Substance` is a condensed form of a node's content stored beside it on the DiVoid graph. The processor
loop may render a candidate's substance in place of its content when the substance is materially smaller —
the **form rule** (`what-goes-in-the-block.md` §8.1). It is the product's main lever on how many bytes a
retrieved memory block spends per admitted row.

It ships whole and switched off: `loop.SubstanceRatioThreshold = 0` and `loop.BlockOccupancy = 0`
(`internal/loop/assemble.go:39`, `internal/loop/turn.go:22`). §8.1.1 names **F-11** — an independent audit of
every live substance, at zero tolerance on any eval-corpus node — as the gate that *"must pass before the
dial leaves zero."*

**F-11 has run, in two rounds, and it does not pass (#14712).** The question is not *what value should the
dial take*. It is:

> **What produces substance, what guarantees it, and under what conditions may the loop render it in place
> of content?**

Round 1 made that a producer question. **Round 2 splits it in two, and the split is the design:**

> **Accuracy tracks the producer. Completeness tracks compression. They are separable.**

Which means the answer is not one mechanism but three, with different owners: a producer chosen and screened
for accuracy, a **two-sided** ratio band for completeness, and a content-side check for the one loss class
no ratio bound reaches.

**Success criteria for this document.** It must (a) settle the producer question with a decision, not a menu;
(b) give a trust mechanism that survives a growing population rather than passing a snapshot; (c) state where
the evidence changed under it and which of its own conclusions that killed; and (d) sequence the work so no
intermediate state is worse than today — the failure #14713 demonstrates when the cap is considered alone.

---

## 2. Scope & Non-Scope

### In scope

- The predicate under which the loop renders substance in place of content.
- The trust mechanism that replaces F-11 as a gate.
- The disposition of `cmd/condense`, of the 11 defective substances, of the byte-prefix stratum and of the
  run-record backfill stratum.
- The composition with the per-candidate payload cap (`BlockOccupancy`), where the two dials' admissible
  regions interact.
- The fill's producer, because the fill writes to the shared graph and is already merged.
- Sequencing, with the intermediate states named.

### Explicitly out of scope

- **A value for `k`.** #14713's grid was measured on a substance population that will not survive the gate
  (§4.3). D6 states the ordering.
- **The prompt.** No clause of #11373 is proposed, amended or defended here — C1: a fidelity guarantee that
  reduces to a prompt asking for fidelity is not a guarantee.
- **A change request against DiVoid.** No new field, no `substanceHash`, no sidecar. D3.
- **Mutating any eval-corpus node.** D2.
- **The candidate-slot crowding problem** (Q11, R19), **two-phase retrieval** (Q7), **the catalogue**
  (Unit 4), **`block` removal**. Unchanged.

---

## 3. Assumptions & Constraints

| # | Assumption or constraint | Basis | If wrong |
|---|---|---|---|
| **A1** | Both representations are in hand when the form is decided — `renderedPayload` sees `Content` and `Substance` (`internal/loop/assemble.go:116`) | Read at `73dfcce`; Unit 1 shipped, PR #58 | Every check must move to write time, and the 90% of producers we do not operate become unreachable |
| **A2** | A content write on DiVoid clears the node's substance | A6/A16, verified live twice (#12984, #14712 §1) | R3 fires. The checks degrade safely — they record the `contentHash` verified against — but §10's staleness claim weakens |
| **A3** | Substance is **not** embedded, so nothing a producer writes can move a rank | F-7 PASSED, #13241 | The fill becomes path-dependent on what earlier runs condensed. Standing guard, unchanged |
| **A4** | The eval corpus's 23 live substances are #12984's output, unchanged | #14712 §1; #14713 cross-checked six independently | §4.2's arithmetic is void. Load-bearing, and the one with two independent confirmations |
| **A5** | **The graph records no producer field on a substance, and no binding between a substance and the content it was written from** | #14712 §11.4, §5.2 | A provenance-keyed rule would become expressible. Nothing observed supports this, and D3 rests on it |
| **A6** | The population grows and changes membership without announcement — 440 today against a design that assumed 67; two lost between #12984 and round 1 | #14712 §1, §5.2 | The one-time-audit shape would be viable |
| **A7** | Round 2's screens over-count. Roughly a third of prohibition flags are paraphrases; numeric flags include tokeniser artefacts | #14712 §9, stated by the auditor | Every retention figure quoted here is a **floor** on retention, and a **ceiling** on loss. Design decisions are taken on the floor, which is the conservative direction |
| **C1** | **Compliance-independence.** Nothing may rest on instructing a model to behave | #14561 / `how-we-know-a-change-helped.md` §11 alt. 6 | — |
| **C2** | **S2 stands.** The form rule may not branch on who wrote the node | §8.3, R5 | — |
| **C3** | The measurement must not write, and must not read what a measurement wrote | `how-we-know-a-change-helped.md` §12.1, §12.10 | — |
| **C4** | Intermediate states must never be worse than today | Toni; demonstrated by #14713's cap-alone cells | — |

---

## 4. The measurements

### 4.1 The population is **four** producers sharing one field name

Round 1 found three; round 2 §7 splits the third and makes it four. **440 substances on a 12,215-node
graph** — not the 67 §8.1.1 assumed, a 6.7× scope error.

| stratum | n | producer | median ratio | accuracy | completeness |
|---|---|---|---|---|---|
| **condense** | 21 | `cmd/condense`, gemma-3-12b, read cold (#12984) | **0.72** | **2 corrupted identifiers** | 69% prohibition retention; **11 of 22 clause-level FAIL**; **5 of 5 FAIL at content ≥ 8 KB** |
| **prose, agent-written** | 376 | the agent that wrote the content, at node-creation time, via `divoid_create_documentation`'s `substance` | **0.096** | **0 fabrications, 0 corruptions** | 10% enumerated-item loss overall; **15% prohibition retention**; 6 PASS / 9 BORDERLINE / 3 FAIL of 18 judged |
| **run-record summariser** | 22 | `loop.RenderSummary` over raw-JSON content, via the backfill path. Deterministic code, no model | — | all flags derived from the JSON; **nothing originates outside it** | n/a — a projection, not a condensation |
| **byte-prefix** | 20 | the live write-back path: `internal/divoid/write.go:58` composes content as `account + fence + recordJSON` and writes `account` as substance | 0.023 | **fabrication-proof by construction** | F-10 verified 884 figures across 17 records, 0 mismatches |

**Two substances #12984 measured are gone** (11087, 11228), consistent with A10 — a content re-post clears a
perfectly good substance. **The audited set is not the live set, and nothing pins it.**

### 4.2 The ratio orders fidelity, and the shipped rule reads it backwards

**Revision 1 stated this as a producer-specific anti-proxy and concluded the ratio carries no fidelity
meaning. Round 2 shows the opposite and it is the more useful result: the ratio is *the* fidelity variable
on completeness, for every producer — with the sign §8.1 assumed inverted.** §13a records the error.

#### 4.2.1 On the eval corpus, as arithmetic

Cross #14712 §3's verdicts against #12984's Q5 ratios, node by node. This is a **join over two published
tables**, not a new measurement. Node 10897 carries a live substance but no published ratio; 11087 and 11228
lost theirs. All three excluded.

| ratio band | nodes | F-11 verdict |
|---|---|---|
| < 0.2501 | **10943** (0.250) | **FAIL** |
| 0.2501 – 0.592 | 10861 (.284), 10982 (.274), 11278 (.353), 10440 (.410), 11262 (.453), 11271 (.478), 10883 (.525) | **6 FAIL, 1 BORDERLINE** |
| 0.592 – 0.780 | 11221 (.617), 11125 (.635), 10444 (.677), 11142 (.767) | **2 FAIL, 2 BORDERLINE** |
| ≥ 0.780 | 10839 (.780), 10879 (.794), 10877 (.801), 11140 (.829), 10927 (.885), 10863 (.886), 11078 (.911), 11049 (.913), 10890 (.925), 11084 (.983) | **6 PASS, 2 FAIL, 1 BORDERLINE** |

- **At θ = 0.592 — #14713's own recommendation at every budget — the rendered set is seven FAIL, one
  BORDERLINE, zero PASS.**
- **The first PASS enters at θ > 0.780. By then nine of the eleven FAILs are already rendering.**
- Median ratio of the FAILs **0.478**; of the PASSes **0.911**. Every PASS is at or above 0.780.

#### 4.2.2 On the whole population, as measurement — and it is the same law

Round 2 §9, whole-stratum screens over all 376 prose substances:

| measure | < 0.05 | 0.05 – 0.10 | 0.10 – 0.25 | ≥ 0.25 |
|---|---|---|---|---|
| **enumerated items with no trace in the substance** (156 nodes, 618 items, 60 lost overall = 10%) | **16%** | 8% | **1%** | — |
| **reader-directed prohibitions retained** (218 nodes carrying one; 32 retained = 15%) | 7% | 12% | **12%** | **55%** |
| the same, condense stratum | — | — | — | **69%** |

**Three readings, each load-bearing:**

1. **Completeness improves monotonically with ratio, on both measures and for both producers.** There is no
   knee on the high side. **The only argument for an upper bound on the ratio is economics** — a substance
   that saves little is not worth a form change. **§8.1 gives a *fidelity* reason for its upper stratum
   (*"~11% of bytes is not worth a 1-in-24 fidelity risk"*) and it is backwards: the 0.886 stratum is the
   safe one.**

2. **There is a knee on the low side, and it is at ≈ 0.10.** Enumerated-item loss falls from 16% to 1%
   across it. **That knee is the floor of the admissible band**, and it is the only bound in this design
   that fidelity actually sets.

3. **Controlled for ratio, the producer advantage on completeness disappears and reverses.** Prose at
   ≥ 0.25 retains 55%; condense at the same ratios retains 69%. **The 15%-vs-69% headline is a compression
   effect, not a producer effect** — and revision 1, which had only round 1, read it as a producer effect.

#### 4.2.3 Accuracy is a different axis and it does not move with ratio

Round 2 §8's token screen — every numeric token and every code-shaped identifier in a substance required to
appear in its content — **with its sensitivity demonstrated rather than assumed**: over the 21 condense
substances it returns **exactly** `PROCESSOR_DIVOVOID_URL` (10861) and `LllValidationService` (11140) and
nothing else, which is precisely what §3.1 found by hand.

> **0 invented figures and 0 corrupted identifiers in 398 non-corpus substances; 2 corrupted identifiers in
> 21 condense substances.**

And the prose stratum is clean **at ratio 0.021**, while the condense pass corrupted an identifier and
inverted a normative rule **at 0.284**. **Accuracy tracks the producer. Completeness tracks compression.
They are separable, and they need separate mechanisms.**

**The screen's false-refusal cost is measured too, and it matters for the design.** 51 rows flagged across
the prose stratum, **every one adjudicated, none a fabrication**: case and plural variants, notation the
substance defines in its own first line, tokeniser artefacts, numeral-for-word, rounding — and **five real
repo symbols the content states only in prose**. 12966's content says *"Assembly renders the anchor with no
budget check"*; its substance says *"`renderBlock` writes the anchor with no budget check"* — **more precise
than its own content, and correct.** Four apparent numeric discrepancies **all resolved in the substance's
favour** against live records and cited designs.

### 4.3 Prohibitions have no safe ratio, and that is the highest-harm finding in round 2

Retention never exceeds 55% for prose at any band and 69% for condense. Even after discounting the screen's
~⅓ over-count (A7), **a majority of reader-directed `Do not` / `must not` / `may not` clauses do not reach
the substance at any ratio the form rule would use.**

The harm is not abstract, and it is not the harm a byte budget trades against:

- **14136 (prose, clause-level FAIL)** — content: *"**Do not read this REJECT as a reason to hold the
  production fix overnight** — read it as 'close eight lines in the file you are already editing, then
  ship'."* Absent from the substance. **A reader acting on the substance holds the fix.**
- **11262 (condense, FAIL)** — content: *"the guard **cannot be** \"the call returned candidates\"."*
  Substance: *"Guard: the call returned candidates."* **A prohibition rewritten as an instruction.**
- 13933 *"must not enter the PR body"*, 13501 *"File it as its own follow-up; do not fold it in"*,
  13459 *"Do not put a campaign-state decision in feedservice"* — all flagged, all actionable.

**A prohibition is the one clause class whose loss inverts behaviour rather than thinning it**, and it is
**mechanically detectable on the content side** — the screen already does it. So it gets its own check (§8.2,
check 2). No ratio bound reaches it, because retention rises with ratio and never becomes acceptable.

### 4.4 The two falsifiers have an empty intersection, and node 10943 sets both bounds

`a-falsifier-measured-on-an-empty-set.md` §7.2 derives **threshold ∈ (0.2501, 0.592]** for F-CAP limb 2 to
pass through the composition. The **lower** bound is node **10943** alone: content 32,105 B, substance
8,030 B, ratio 0.2501. The cap at `k ≤ 7` cuts it as `oversized` in content form; the composition's only way
to keep it is to render its substance; the strict `<` means the form rule fires on it only if θ > 0.2501.
That one document is also why `k ≤ 7` is a hard constraint.

**10943 is an F-11 FAIL, and it is the node §8.1.1's M4 named to audit *first*.** Its content enumerates the
instrument-failure family through mode 8, mode 9, mode 10, mode 10's guard and the correction trap; the
substance **stops at mode 7's companion — roughly 230 lines and five enumerated members absent** — while
still carrying *"**The complete set is the symmetry table at the end of this note.**"* Its mode-6 section is
referentially broken, and four impossibility claims are gone including the node's own title claim.

> **The composition does not *recover* node 10943. It substitutes, for a required document, an artifact that
> closes an enumeration early while asserting the enumeration is complete — and reads as finished.**

A cut is visible in the dispositions. A condemned substance is invisible: the header says `form: substance`
and the model is holding a document that claims to be whole. **On the one node where the two mechanisms
meet, the "recovery" is worse than the loss it repairs**, and F-CAP limb 2 cannot see it, because 10943
remains *admitted*.

**Revision 2 sharpens the gap to 0.0001.** The band's ceiling (§8.3) is **0.25**; F-CAP needs **> 0.2501**.
**Both bounds are set by the same document from opposite sides.**

| region | fidelity, on the measured population | F-CAP limb 2 |
|---|---|---|
| 0.10 ≤ ratio < 0.25 | **the admissible band** — 1% item loss, and below the entire condense stratum | **fails** — 10943 is cut and unrecoverable |
| ratio > 0.2501 | **renders 10943, a FAIL**, and re-admits the condense stratum wholesale | passes at θ ≥ 0.298 on this corpus |

**The intersection is empty, and it is not a tuning problem.** It is a statement about the *population*: the
boundary document's substance is defective, so the bound derived from it is a requirement to render a defect.
**Change the population and both constraints can be met at once** — 10943 carrying a substance whose
omissions are visible is both recoverable *and* not condemned. **The work is upstream of the dial.**

**Restatement discipline.** `a-falsifier-measured-on-an-empty-set.md` §5.1 binds: a falsifier may be
restated only before the re-measurement it will judge, in a direction derivable from the design's own claims
or strictly harsher, with the prompting failure recorded. **F-W5** restates limb 2 **harsher** — a required
document "recovered" by an unwarranted substance counts as lost — and the failure prompting it is recorded
above.

### 4.5 The fill's size floor and the fidelity cliff are the same boundary, pointing opposite ways

| fact | site |
|---|---|
| The fill fires only on candidates whose **content is ≥ 8,000 B** | `internal/loop/fill.go:31` — `FillSizeFloor = 8_000` |
| The fill's producer **is** `cmd/condense`'s pipeline | `internal/fill/fill.go:30` — `condense.Run(...)` |
| That producer's measured fidelity at content ≥ 8 KB is **0 of 5** | #14712 §4 |
| **And the prose stratum is at its most compressed in the same band** — median ratio 0.098 (8–16 KB), **0.051 (≥ 16 KB)**, below the completeness knee | #14712 §11.1 |

**G1 was derived from economics** (§7.4.3) and is correct as economics. **Fidelity runs the other way on the
same axis for both producers**, and the two were never crossed. The fill is gated to fire **only** where its
producer has never once been measured to succeed, and every fill writes to the **shared** graph — where
§7.4.5's durability asymmetry applies: *self-healing repairs absence, not wrongness*, and a bad substance is
durable **precisely on nodes whose content is static**, the regime §7.4.1 names as the fill's best case.

**It is inert today only because `PROCESSOR_CONDENSE_MODEL_URL` is unset** (`internal/boot/config.go:93`).
That is the correct *contract* and not a *guarantee*: the plan is for enabling the fill to become a config
change. **An environment variable is not a gate.**

### 4.6 #14713's benefit and #14712's failures are the same artifacts

Both ran on `internal/eval/corpus.json`, 25 rows. The corpus's required nodes are the 21 the condense pass
was pointed at; `corpus-anchor.json`'s 48 further nodes carry **no** substance. So every substance-render
#14713 counted — 47 at the recommended 60,000 / θ 0.592 / k 6 cell, 35 at 30,000, 26 at 15,000 — is a draw
from the eight-node subset §4.2.1 enumerates: **seven FAIL, one BORDERLINE, zero PASS.**

**This is not a criticism of #14713.** The sweep was run correctly, its falsifiability gate passed on every
row, and its **admission arithmetic is sound**. What cannot be carried forward is the **conclusion**: every
`reqCut = 0` in the recommended rows is purchased by rendering documents the audit condemns. **The
recommendation table is withdrawn as a basis for choosing θ or `k`** (D6).

**And the instrument is inverted.** The corpus's live substances are **91% condense-produced** (21 of 23);
the graph's are **90% agent-written** (398 of 440). **The instrument measures almost exclusively the stratum
the product will not use.** Cheapest thing on the critical path (U5).

### 4.7 What the evidence now supports, and what it still does not

**Settled by round 2, over the whole population rather than by sample:**

- Accuracy: **0 fabrications in 398** against **2 in 21**, with the screen's sensitivity demonstrated on a
  control.
- Completeness: ratio is the variable, for both producers, with a measured knee at ≈ 0.10.
- Age: **no trend**, flat and noisy across 2026-09-05 → 2026-09-22. *"The stratum is improving"* is off the
  table — it is a property, not a trajectory.
- Size: prose degrades with size **through ratio**; **accuracy does not degrade at any size**.

**Still not established:**

- **Per-node attribution.** A5: no producer field, so no individual substance can be attributed and no
  substance can be *proven* written in the same turn as its content. The proxies are strong and are proxies:
  **281 of 398 (71%)** have `lastUpdate == created`, and **eight substances carry facts absent from their
  own content that verify correct outside it** — four repo symbols, a similarity exact against a live run
  record, an addendum reconciliation, a status code exact against a cited design. *A compressor holding only
  the content could not have produced any of them.* **The data supports the mechanism; it cannot establish
  it per node** — which is precisely why D3 puts the guarantee on the artifact rather than on the producer.
- **417 substances remain unaudited at clause level.** The whole-population screens are mechanical; the
  clause-level judgements rest on 18 prose nodes and 22 condense nodes.
- **The binding.** §5's erasure finding (11087, 11228) is unaddressed and there is no `substanceHash`.

---

## 5. Architectural Overview

**One predicate changes, one field splits, and the dial gains a second side.** Retrieval, fusion, the
budget, the ports, the cap's seam and the record's shape all keep their form.

```
  ┌── the graph ────────────────────────────────────────────────────────────────┐
  │  content    — faithful, authored, always present                            │
  │  substance  — a DERIVED artifact. FOUR producers, ONE field, NO producer     │
  │               record anywhere on the graph (A5):                             │
  │                 · live write-back — a verbatim prefix          (20)          │
  │                 · backfilled run summaries — computed          (22)          │
  │                 · prose, written beside the content            (376)         │
  │                 · abstractive condensation read cold           (21) ← 11 FAIL │
  └─────────────────────────────────────────────────────────────────────────────┘
             │                                                ▲
   ranked +  │                                                │  producers
   addressed │                                                │   · write-back (constructive)
     reads   ▼                                                │   · authoring agents (not ours)
                                                              │   · the fill ── CLOSED until its
  ┌── internal/loop ──────────────────────────────────────────┤       producer can enter the band
  │  Retrieve → fuse → admit → RENDER                         │
  │                       │                                   │
  │                       └─ the payload seam (ONE call site) │
  │            ┌──────────────────────────────────────────────┴───────────┐
  │            │ 1. ACCURACY   — every code-shaped token and numeral in   │
  │            │                 the substance occurs in the content      │
  │            │ 2. DIRECTIVES — every reader-directed prohibition in the │
  │            │                 content has a counterpart                │
  │            │ 3. BAND       — floor ≤ ratio < threshold   ← TWO-SIDED  │
  │            │                 floor = completeness (≈0.10, measured)   │
  │            │                 threshold = ECONOMICS ONLY               │
  │            │ — or — CONSTRUCTIVE: verbatim spans of own content,      │
  │            │        elisions marked ⇒ 1 and 2 hold by construction,   │
  │            │        3 does not apply, form is `excerpt`               │
  │            └──────────────────────────────────────────────────────────┘
  │                       └─ the cap reads renderedSize, as today          │
  └────────────────────────────────────────────────────────────────────────┘
```

**The reframing in one line.** §5 of `what-goes-in-the-block.md` said the loop stops asking *which rows do I
drop?* and starts asking *in which form does each row travel?* This document adds the question between them —
***on what grounds may this row travel in that form?*** — and answers it with checks over bytes rather than
with a belief about a producer. **Round 2 adds the correction that the ratio is one of those grounds, and
that the shipped rule holds it upside down.**

**Why the checks sit at render time.** Three reasons, each sufficient:

1. **We do not operate 90% of the producers**, and **A5 says the graph records none of them**. There is no
   write path of ours to gate and no field to read.
2. **A population audit cannot stay passed.** Membership grew 67 → 440 unannounced; two members lost their
   substance between rounds. A per-use check has no snapshot to invalidate.
3. **Both representations are already in hand** (A1). §8.1 argues the ratio is *"available for free: both
   representations are in hand"*. Every check here is the same argument applied to fidelity.

---

## 6. Components & Responsibilities

| Component | Owns | Does **not** own |
|---|---|---|
| **`internal/divoid` (adapter)** | The wire projection, including `substance`. Unchanged | Any judgement about the bytes it supplies |
| **`internal/loop` — the payload seam** (`renderedPayload`) | **The whole form decision: accuracy, directives, band, form.** One pure function of the candidate's own bytes, remaining the single call site the cap and both renderers read through | Generation. Prompts. Who wrote the node (C2/S2) |
| **The three checks (new, inside the seam)** | **One question each, over two byte strings.** Pure, deterministic, no I/O, no clock | Fidelity in the sense of meaning. §8.2 states each check's bound in the same table as its guarantee |
| **`internal/loop` — `admit`** | The budget and the cap, over `renderedSize`. Unchanged | Which bytes those are |
| **The disposition record** | **Per candidate: each check's verdict and reason, beside `form` and `renderedSize`.** The unit's first product, and it exists before the dial moves | Deciding anything |
| **`internal/loop` — the fill** | Whether a node is worth filling (G1–G3), and recording the outcome | **What a fill may write.** D1 moves that to the producer's contract |
| **`internal/condense` / `cmd/condense`** | **Narrowed (D1):** the prompt, the postprocess gates, the audit bundle, the refusal to store a defective result — and being the reproducible instrument F-1 qualifies against | **The authority to put a renderable substance on the graph**, while its output sits outside the band |
| **`internal/runbackfill`** | Re-composing backfilled run records so their substance becomes a provable prefix (U4) | — |
| **`internal/eval` / `cmd/eval`** | The sweep, and a **warranted substance fixture set held in the repo** | **Writing substance to the graph. Ever** (C3, A13) |

**The genuinely new responsibility is the check set**, and its defining property is what it *cannot* do: it
cannot be satisfied by anything a producer says about itself. It reads the pair. That is the whole of its
compliance-independence, and A5 makes it the only option rather than the preferred one.

---

## 7. Interactions & Data Flow

### 7.1 Where the decision happens

Unchanged from §7.2 of `what-goes-in-the-block.md`, with the decision inside step 2 expanded:

| Step | What happens |
|---|---|
| 1 | Candidates arrive carrying content, substance (possibly absent), type, name, similarity |
| 2 | The payload seam decides the form: **(a)** substance or content absent ⇒ **content**; **(b)** a **constructive** decomposition verifies ⇒ **excerpt**, and stop; **(c)** the **accuracy** check fails ⇒ **content**, naming the offending token; **(d)** the **directive** check fails ⇒ **content**, naming the clause; **(e)** the ratio is **outside the band** ⇒ **content**, naming which side; **(f)** otherwise ⇒ **substance** |
| 3 | `admit` applies the cap and the budget to the chosen payload, exactly as today |
| 4 | The section renderer emits the chosen form with its header line, on **both** surfaces (F-12) |
| 5 | The record states, per candidate, `size`, `contentHash`, `form`, `renderedSize`, **and each check's verdict with its reason** |

**Step 5's addition is the unit's product and it exists at the off position.** At the off position no row
renders as substance and nothing about the block changes — but every disposition now carries *whether this
substance would have been renderable, and which check refused it*. **That converts F-11 from an audit
somebody has to run into a figure the sweep reports**, over whatever population retrieval actually touches,
forever. The same move Unit 1 made for coverage, applied to trust.

**Ordering within step 2 is not arbitrary.** Constructive first, because it makes 2(c)–(e) unnecessary rather
than passing them. Accuracy before directives before the band, because that is increasing order of expected
false-refusal rate, and the recorded reason should name the strongest objection rather than the first
cosmetic one.

**`size` and `contentHash` keep their present meaning** (R4, #12955 §6.4 verbatim). Verdicts are recorded
*beside* them and never rebase them.

### 7.2 What the checks read, and what they cannot

| They read | They cannot read |
|---|---|
| The candidate's `Content` and `Substance`, as bytes | The graph. The model. The clock. Any field naming a producer — **there is none (A5)** |
| Nothing else | Anything about who wrote the node (C2/S2) |

**Purity is load-bearing, not tidiness.** `Assemble` is documented as *"a pure function: no I/O, no clock, no
randomness"* (`internal/loop/assemble.go:43`) and `cmd/eval`'s determinism guarantee is a property of its
dependency closure (A13, R9). A check needing a graph read would put a network dependency inside the sweep's
inner loop and break both.

---

## 8. The rule, restated

### 8.1 The predicate

> **Render a candidate's substance in place of its content when, and only when:**
>
> 1. **Presence** — non-empty substance and non-empty content. *(unchanged)*
> 2. **Accuracy** — every code-shaped token and every numeral in the substance occurs in the content, under
>    the normalisations of §8.2. *(new)*
> 3. **Directives** — every reader-directed prohibition in the **content** has a counterpart in the
>    substance. *(new)*
> 4. **Band** — `floor ≤ ratio < threshold`, **two-sided**. *(the change)*
>
> **Or** render it as an **`excerpt`** when it verifies as a decomposition into verbatim spans of its own
> content with marked elisions, in which case 2 and 3 hold by construction and 4 does not apply (§8.5).
>
> **Nothing in this predicate mentions who wrote the node.** S2 is intact, and A5 makes it unbreakable:
> there is nothing on the graph to branch on.

### 8.2 The checks, each with its bound stated beside its guarantee

| # | check | what it makes impossible | what it does not touch | measured cost |
|---|---|---|---|---|
| **2** | **Accuracy — token containment.** Every code-shaped identifier (camelCase, PascalCase, snake_case, SCREAMING_SNAKE, backtick spans, dotted/slashed paths) and every numeral in the substance must occur in the content | **Fabricated and corrupted identifiers** — `LllValidationService`, `PROCESSOR_DIVOVOID_URL` — and **invented figures** | **Omission and inversion. Entirely.** 10861's reversed rule contains no token its content lacks and passes this check | **Sensitivity demonstrated:** exactly 2 of 2 on the condense control. **False refusals measured:** 51 flagged rows across 376 prose substances, none a fabrication. §8.2.1 |
| **3** | **Directives.** Every `Do not` / `must not` / `may not` clause addressed to a reader in the content must have a counterpart in the substance | **Silent loss of a prohibition** — the loss class that *inverts* behaviour rather than thinning it (§4.3) | Prohibitions expressed without those markers; and it cannot judge whether a counterpart means the same thing | Refuses a large fraction of directive-bearing substances at every ratio, by design — retention is 12%–55% measured (§4.2.2) and the screen over-counts by ~⅓ (A7) |
| **4** | **Band.** `floor ≤ ratio < threshold` | **Rendering a substance so compressed that enumerations have started vanishing** — 16% item loss below 0.05 against 1% at 0.10–0.25 | Anything about accuracy; and anything at all above the floor except economics | §8.3 |
| **★** | **Constructive.** The substance decomposes into verbatim spans of its own content, in order, separated by an explicit elision marker. Degenerate case: one span from offset 0 | **Fabrication and polarity inversion by construction**, and **omission becomes visible** — an elision marker states that something is missing, so an enumeration cannot close early while reading as complete, which is 10943's exact failure | **Which** spans were chosen. A misleading juxtaposition of two true spans is still possible | Free to check. Available today only to the 20-node live write-back stratum; U4 extends it to the 22 backfilled records |

**The honest bound on all of them, stated once and not softened.** *None of these certifies that the
substance is a faithful account of the content.* Check 2 bounds fabrication. Check 3 bounds the highest-harm
omission class. Check 4 bounds over-compression. The constructive tier additionally bounds inversion and
makes omission visible. **Nothing here bounds a bad *selection* of true material.** That residue is real, it
is what the three prose clause-level FAILs are made of, and if it is judged too large the correct answer is
that the form rule does not ship above zero — §13 states the condition rather than leaving it to be
re-argued.

**Why containment must be a subset test and not a similarity score.** A threshold on overlap is
unfalsifiable, trivially satisfied by restatement, and yields a number that reads like fidelity while
measuring word reuse — `how-we-know-a-change-helped.md` §11 alternative 5 rejected exactly that shape for
exactly that reason. A subset test either holds or names the token that broke it.

#### 8.2.1 The accuracy check refuses, and here is the arithmetic behind that ruling

**Measured precision on the whole population is poor: 2 true refusals against 51 flagged prose rows.** Taken
as a classifier that is a bad instrument. **Taken as a gate it is a good one, because the two outcomes have
different units:**

| outcome | consequence |
|---|---|
| **true refusal** | a corrupted identifier does not reach the model. The failure it prevents is **undetectable to the reader and durable** (§7.4.5's asymmetry) |
| **false refusal** | the row renders as **content** — exactly as today. The cost is **bytes, not correctness** |

**Ruling: it refuses.** And the adjudication in #14712 §8 supplies its own tuning, which must land with it:
fold case and plurals before comparing (6 flags); exempt notation the substance defines in its own first
line (4 flags); tokenise numerals properly — comma-separated id lists, ranges, `SHA-256`, numeral-for-word,
rounding (the bulk of the 40 numeric flags). **What no content-only check can ever fix is the five real repo
symbols the content states only in prose** — a substance that knows more than its content is refused, and
12966's is *more precise than its own content*. That residual false-refusal rate must be reported (F-W2); if
it stays large after the normalisations, the token class is drawn too wide.

### 8.3 The band — **the correction round 2 forces, and the defect it names**

> **`what-goes-in-the-block.md` §8.1 specifies a single-sided dial: render substance wherever the ratio is
> below a threshold. The measurement says the danger is at the *other* end. This is a design defect, not a
> tuning problem: the more aggressively that dial is tuned, the more precisely it selects the substances
> that have lost content.**

The two bounds have different owners and only one is about fidelity:

| bound | owner | value on today's population | derivation |
|---|---|---|---|
| **floor** | **completeness** | **0.10** | The measured knee (#14712 §9): enumerated-item loss **16% → 8% → 1%** across 0.05 and 0.10. Above it, loss is 1% |
| **threshold** | **economics, and nothing else** | **0.25** | Completeness improves monotonically with ratio (§4.2.2), so fidelity sets no upper bound. What sets one is that a substance saving under ~75% is not worth a form change. **0.25 also happens to sit just below the entire condense stratum (minimum 0.2501) — that is a proxy and is labelled as one** |

**What the band buys, on today's population:**

- It admits the prose stratum's measured-safe region — including large nodes. #14712 §10's two strongest
  prose PASSes are **14642 at 24,861 B / ratio 0.185** (all five warnings enumerated, every prohibition
  kept, every impossibility claim kept) and **13669 at 37,256 B / 0.229** (polarity-checked claim by claim).
  **Both are ≥ 8 KB and both sit inside the band.**
- It excludes the ≥ 16 KB prose band, whose median ratio is **0.051** — below the floor, and the least
  complete material in the stratum.
- It excludes **every one of the 21 condense substances by arithmetic**, with no producer field involved.
- **On the eval corpus it renders nothing at all**, because the corpus's only substances are condense-produced
  (§4.6). That is an instrument problem (U5), not a product problem.

**Why this replaces revision 1's size ceiling, and the ceiling is withdrawn.** Revision 1 proposed
`SubstanceCeiling = 8_000` on **content size**, reasoning from #14712 §4's monotone size-clustering.
Round 2 §11.1 shows the mechanism: **prose degrades with size *through ratio*, and accuracy does not degrade
with size at all.** So size is a proxy for the quantity that actually predicts loss, and the band keys on the
quantity itself. The ceiling would have refused 14642 and 13669 — two of the three strongest substances in
the judged sample — while admitting nothing the band does not already admit. **The ceiling was the right
instinct measured on the wrong axis.**

**What the band costs, stated plainly and not buried.** §4.3 of `what-goes-in-the-block.md` prices the
yardstick's seven candidates at **106,829 B in content form and ≈36,081 B in substance form**, with the
saving concentrated in three documents of 42,931 B, 28,031 B and 20,019 B. Whether those render depends
entirely on **where their substance's ratio falls**, which nobody has measured. **The band does not
categorically exclude them — that is its advantage over the ceiling — but it does not promise them either,
and §4.3's figures were derived from *condense-stratum* medians (0.298) that put them above the band's
threshold.** The prize is not refuted and it is not banked.

### 8.4 What the ratio dial is for now — **re-ruled, and revision 1's re-ruling withdrawn**

~~**The ratio threshold is an economics dial. It is not, and never was, a fidelity instrument.**~~
**Struck.** Revision 1 concluded that from round 1's within-producer inversion, and it is half right in a
way that produces the wrong mechanism: the *threshold* is economics, but the *ratio* is the fidelity variable
on completeness and therefore needs a bound the shipped rule has no place to put.

**The corrected statement:**

| quantity | what it decides |
|---|---|
| **accuracy check** | whether this artifact may stand in for its content at all — the **producer-tracked** failure |
| **directive check** | whether the highest-harm loss class has occurred — detectable from the **content** side, at any ratio |
| **ratio floor** | whether compression has gone far enough to start losing enumerations — **the fidelity bound the ratio actually sets** |
| **ratio threshold** | whether the substitution pays for itself in bytes — **economics, and only economics** |

**Consequence for the dial's derivation.** `0.592` was derived as the midpoint of #12984's two stratum
medians — i.e. from the **condense** stratum, which sits outside the band. **`0.592` has no derivation any
more**, and neither does `0.370` (already excluded as circular by §8.1.1). **Both bounds must be re-derived
from the distribution of ratios among substances that pass checks 2 and 3**, which is a figure the
disposition record will carry from U1 onward and which nobody has today.

### 8.5 The excerpt contract — the constructive strata do **not** share the band

**Answering the question directly: no.** A self-labelled excerpt and a condensation are different claims
about the same field, and one dial cannot govern both.

| | condensation | excerpt |
|---|---|---|
| what it claims | *this is an account of the whole node* | *this is a span of the node; the rest is withheld* |
| its ratio means | how much was removed — a fidelity quantity (§4.2.2) | how much of the head was kept — **no fidelity content whatever** |
| what it withholds | unknown to the reader | **the remainder, and the reader must be told** |
| under the band | 0.10 ≤ ratio < 0.25 | the run-record headers sit at **0.023**, below the floor — **the band would refuse the one stratum with a constructive guarantee and an independent audit behind it** |

That last row is the argument in one line. `internal/divoid/write.go:58` composes a run record's content as
`account + fence + recordJSON` and writes `account` as the substance, so **the prefix relation is a property
of the composition** and provable in one comparison. F-10 audited that class at 884 figures, 0 mismatches.
Rendering it under a header that says `form: substance` tells the model it holds a condensation of the record
when it holds the record's first page.

> **Ruling: `form: excerpt` is a distinct form with a distinct header line naming what is withheld, governed
> by the constructive check alone. No ratio bound reaches it, because no ratio question is being asked.**

**This also resolves a latent inconsistency in §9.2** — *"a run record may be admitted in substance form; it
may never be admitted in content form"* is a rule about a **class of node**, inside a design whose S2 says
the rule may not know about classes of node. **Under the excerpt form it becomes a rule about excerpts**,
which any node may carry. §9.2's guarantee is preserved and its exception disappears.

**The 22 backfilled run summaries are the instructive hard case, and they do not get an exemption.** Their
producer is deterministic code and their content is raw JSON, so their substance is **computed** — rounded
similarities, remaining-budget arithmetic, kB conversions — none of which appears verbatim in the content.
**They fail the accuracy check, and under A5 nothing on the graph lets the loop tell them from a model's
output.** Ruling: **they fail, and render as content** — which for an 80 KB JSON record means they are never
admitted, exactly as today, so nothing is lost. **The remedy is not an exemption; it is to make their warrant
provable** by re-composing them through the same `account + fence + record` shape the live path already uses.
That is U4: small, cheap, and it converts an unprovable guarantee into a provable one with no new mechanism.

---

## 9. Decisions

### D1 — `cmd/condense` is **narrowed**. Not retired, and not prompt-repaired. *(revised)*

**Revision 1 wrote this as a straight condemnation. Round 2 makes it two-sided, and the corrected version is
sharper.**

**What round 2 says in condense's favour, and it must be stated first because revision 1 got it wrong:**
at matched ratio the condense pass retains reader-directed prohibitions **better** than the prose
stratum — **69% against 55% at ratio ≥ 0.25** — and the 15%-vs-69% headline is a compression effect, not a
producer effect (§4.2.2). **The producer is not incompetent at retention.**

**What condemns it anyway, and this is the reconciliation of both rounds:**

> **Its completeness is purchased by not compressing.** At its median ratio of 0.72 it saves 28% — below any
> economics threshold that makes a form change worth making, i.e. **outside the band on the economics side**.
> When pushed to where it *would* pay (content ≥ 8 KB → median ratio 0.298) it is **5 of 5 clause-level
> FAIL**. **It is not bad at condensing; it is bad at condensing *enough*, and at the compression where it
> would pay, it fails.**

Add the axis that is producer-tracked and does not move with ratio: **2 corrupted identifiers in 21, against
0 in 398** (§4.2.3). **Clause-level: 11 FAIL of 22, against 3 FAIL of 18 for prose** — and the prose failures
are all the dropped-item family, while condense's include a **reversed normative rule** (10861), a
**prohibition rewritten as an instruction** (11262) and an **enumeration closing early while claiming
completeness** (10943).

**Retirement is still rejected.** It is the only **reproducible** producer, which is what F-1 needs as an
instrument (*"it is not a search for a model; it is a qualification of the one chosen"*) and what any
producer comparison needs as a control arm; 21 nodes carry its output and some are eval fixtures; and its
postprocess seam (`internal/condense/postprocess.go`, `Defect()`) is the project's existing example of a
mechanical acceptance check on a produced artifact — the exact shape §8.2 generalises.

**Prompt repair is still rejected, and it is C1 rather than pessimism.** The only lever on a compressor
reading content cold is the prompt; a prompt asking for fidelity is compliance-dependent; and it has been
tried at full strength — #12984 records that #11278's failure was *"#11373 §1's measured failure mode
reproduced exactly… surviving inside the very prompt whose Keep 2 clause was written to prevent it."*
#14712 §5.5 reached the same conclusion from eleven nodes.

**There is a real role, and round 2 makes it concrete rather than rhetorical.** **440 of 12,215 nodes carry
a substance. The other 96% have none, and their authors are gone** — a node whose author did not write a
substance will never acquire a prose one. **That population is the fill's entire subject, and no authoring
agent will ever serve it.** So a model producer has a reason to exist; what it needs is a way into the band.

**The one repair worth costing is therefore not a prompt — it is a different product.** An **extractive**
pass, whose output is a selection of verbatim spans with marked elisions, satisfies checks 2 and 3 by
construction, cannot invert a polarity it never rewrote, and makes 10943's exact failure inexpressible,
because the elision marker is where the missing members would be. **And it is the only route by which a model
producer reaches the band**: extraction can compress far below 0.72 without the losses being silent.
**Adopted as the recommended direction, filed with its own falsifier (F-W7), not adopted as a commitment** —
whether a model selects spans well at band ratios is unmeasured and the measurement is cheap.

### D2 — The 11 defective substances are **not touched** *(unchanged)*

**They stay on the graph, unmodified, and the checks make them inert without a byte changing.** Four reasons:

1. **They sit on eval-corpus nodes, which are measurement instruments.** Mutating them changes what every
   prior and future sweep measures, mid-programme (`how-we-know-a-change-helped.md` §12.1).
2. **Deletion destroys reproducibility that has already been used.** #14713 cross-checked its candidate sizes
   against #12984's table on six nodes precisely because those artifacts were still there; #14712 established
   that nothing drifted by the same route.
3. **Inertness-by-mechanism covers the 417 unaudited; deletion covers 11.**
4. **They are already inert**, and nothing in the sequence moves the dial before the checks exist.

**Round 2 makes the disposition cleaner than revision 1 could.** Under revision 1's containment-plus-ceiling
scheme, five of the eleven (10927, 11125, 11142, 11262, 11278) passed both, and two of those would render at
θ = 0.592. **Under the band, all 21 condense substances are excluded by arithmetic** — the stratum's minimum
ratio is 0.2501 and the band's threshold is 0.25. **The eleven become unrenderable without any rule that
knows they exist.**

**Two obligations follow, and they are not mutations:** the condemnation stays recorded outside the node
(#14712 is that record, with every verdict and its quoted clause), and a sweep's substance-render counts must
be read as *what the checks refused* as well as *what rendered*, or a clean cell gets quoted as evidence the
population is clean.

### D3 — Durable trust is **verification at render time** — and round 2 makes it the only option *(strengthened)*

| shape | rejected because |
|---|---|
| **A one-time population audit (F-11 as specified)** | **Structurally incapable of staying passed.** 67 → 440 unannounced; two members lost their substances between rounds; 417 remain unaudited at clause level and the set grows with every `divoid_create_documentation` call anywhere on the graph. **F-11 was the right instrument and the wrong shape of gate** — it produced the finding that reframes the problem and can never be the thing that holds the line |
| **A per-node provenance marker on the graph** | **There is nowhere to put it and nothing to check it with. A5, measured: the graph records no producer field on a substance, and no substance-to-content binding.** Revision 1 rejected this as compliance-dependent; round 2 shows it is also *unavailable*. Any marker a producer writes is a claim by that producer about itself, and the F-11 failure mode is precisely a producer that believes it succeeded — *"every failing substance reads as finished."* #12955 §3.2's sidecar ledger is already withdrawn |
| **Producer qualification alone (F-1)** | **It reaches a producer we operate; 90% of the population has one we do not, and 96% of the graph has none at all.** R1's own wording with the scale corrected from 67 to 398. F-1 stays necessary for the fill and for `cmd/condense`; it is not sufficient and cannot be made so |
| **A staleness signal** | **Solves a problem that is not the one measured.** A2: a content write clears substance, so the dangerous direction is unobserved and would fire R3. What was observed is the safe direction — substance disappearing, which degrades to content. Worth having as a standing guard on an external dependency (§10), not as the trust mechanism |
| **A write-time proxy — `lastUpdate == created`** | **71% of 398 satisfy it (§4.7) and it is the strongest attribution evidence available. It is still a proxy, it is graph metadata rather than a property of the artifact, and it is not in the recall projection.** It belongs in a research note, not in a gate |

**What is adopted:**

> **A substance may be rendered only if, at the moment of rendering and with both byte strings in hand,
> deterministic checks over the pair return clean. Every verdict and its reason are recorded on the
> disposition — always, including at the off position, including for rows that were cut.**

- **Per-use, so it never goes stale.** No snapshot; the 441st substance is checked the first time it is
  considered, and so is the 4,000th.
- **Producer-blind in mechanism, producer-discriminating in effect.** It asks nothing about who wrote
  anything (S2, C2, and A5 leaves nothing to ask), and it happens to admit the constructive strata, admit the
  prose stratum inside the band, and refuse the condense stratum by arithmetic.
- **Compliance-independent (C1).** It reads bytes. No producer can assert its way past it.
- **Free.** ≤ 20 candidates, bytes already fetched, one pass, no network, no model, inside a function already
  documented as pure. Smaller than the `contentHash` the same function already computes on every candidate.
- **It turns the audit into an instrument.** F-11 asked a person to read 440 documents once. The checks report
  a verdict per candidate per turn, forever, over exactly the population retrieval touches — the only
  population the product's behaviour depends on. **That is what replaces F-11 as a gate.**

**Where a staleness signal does belong.** The disposition already carries `contentHash`. Recording verdicts
beside it lets a sweep report whether a node's substance kept passing against a changing content hash —
**without a new graph field and without an ask against DiVoid.**

### D4 — The constructive strata get their own form and do not share the band *(unchanged; extended)*

Adopted as §8.5, extended by round 2's discovery of the 22-node backfill stratum: its guarantee is real and
**unprovable from bytes**, so it fails, renders as content, loses nothing (an 80 KB record is never admitted
in content form anyway), and is repaired by re-composition rather than by exemption (U4).

### D5 — ~~The size ceiling~~ **The band** is what the size clustering points at *(revised)*

~~**`SubstanceCeiling` — a bound on the content size above which no substance renders.**~~ **Withdrawn.**
Round 2 §11.1: prose degrades with size **through ratio**, and **accuracy does not degrade with size at
all**. Size is a proxy; the ratio is the quantity. The ceiling would have refused the two strongest large
prose substances in the judged sample (14642, 13669) while admitting nothing the band does not.

**The split answer to the size question, which is what the measurement actually supports:**

| axis | does it degrade with size? |
|---|---|
| **accuracy** | **No.** Zero fabrications at any size in the prose stratum |
| **completeness** | **Yes — but through ratio.** Median ratio 0.174 (< 4 KB) → 0.144 → 0.098 → **0.051 (≥ 16 KB)**, with both retention measures falling alongside |

**So the producer rescues fidelity-of-fact at scale and does not rescue completeness at scale**, and the floor
is what catches the second without a size rule.

### D6 — The dials are coupled, both orderings are blocked, and neither may move first *(unchanged)*

**#14713 established that the cap cannot ship alone:** at θ = 0 it cuts required documents at every budget
and several occupancies (60,000: `reqCut` 1 at k=3,4,5; 2 at k=6; 3 at k=8), clearing only at θ ≥ 0.298.
That re-establishes `a-falsifier-measured-on-an-empty-set.md` §7.3, which had already withdrawn its own D2
reversal and reinstated `what-a-block-is-worth-per-byte.md` §12's payload-first ordering on R1's exact shape.

**What this document adds is that the form rule cannot ship first either**, because every θ that helps the
cap renders condemned artifacts (§4.2.1, §4.4). **Both orderings are blocked, and the deadlock is a property
of the substance population, not of the two mechanisms.**

> **Ruling. Neither dial leaves its off position until the eval corpus carries a substance population whose
> members can pass the checks. The cap stays at its off position (F-CAP limb 4) throughout, and `k` is
> chosen after the re-run, never from #14713's grid.**

Consistent with `a-falsifier-measured-on-an-empty-set.md` §11's closing rule — *"no number off its off
position before M5 is green on the composition"* — and it makes that document's M4 reachable: **M4 has run,
it failed, and the successor is not M5 but a population repair** (U5).

### D7 — F-11's verdict, and what replaces it *(unchanged)*

**F-11 is discharged: verdict NO.** It is **not** re-run at a different tolerance and **not** narrowed to be
passable. It is **retired as a gate and retained as a finding**; F-W1–F-W4 (§11) stand in front of the dial
from now on.

The reason to retire rather than repeat: its own §5.1 records that the gate's scope was wrong by 6.7× and the
population is not pinned. A second run produces a second snapshot of a set that changed while it ran. **An
audit found what an audit is good at finding — that the population is four objects and the ratio orders them
in a direction the rule reads backwards — and that finding is what the gate should have been all along.**

### D8 — The single-sided dial is a defect and must not be shipped even at a "safe" value *(new)*

Stated as its own decision because it will otherwise read as a tuning recommendation.

> **A one-sided `ratio < threshold` rule cannot express the admissible region, and no value of that
> threshold makes it safe.** Lowering it improves the byte economics *and monotonically worsens the expected
> completeness of what renders*. **The dial's two directions pull the same way on value and opposite ways on
> safety, which is the signature of a missing bound rather than a badly chosen one.**

**Consequence:** any change that moves `SubstanceRatioThreshold` off zero **must** introduce the floor in the
same change. Shipping the threshold alone — even at a conservative value — is shipping the defect with a
number that hides it. **F-W8's off-position guard must therefore assert both bounds**, or it guards half a
mechanism.

---

## 10. Cross-Cutting Concerns

| Concern | Ruling |
|---|---|
| **Determinism** | **Preserved, and the checks are chosen to preserve it.** Pure, byte-only, no I/O — so `Assemble` keeps its documented contract and `cmd/eval`'s closure guarantee (A13, R9) is untouched |
| **Staleness** | A2 holds: a content write clears substance, so the dangerous direction is unobserved and the fallback is content. **The external dependency is now guarded from our side** — verdicts are recorded beside `contentHash`, so divergence becomes visible in the record instead of silent |
| **Fidelity** | **No longer routed to the prompt.** C1 forbids resting there and the failure survived the clause written against it. It is routed to **what a checker can prove about bytes** (checks 2, 3), **what compression predicts** (check 4), and **what the reader can see is missing** (the elision marker, the excerpt header) |
| **Observability** | Every disposition gains three verdicts with reasons, for admitted **and cut** rows — Unit 1's reason: *a row that was considered and dropped is exactly the row a coverage question is about* |
| **Prompt-surface symmetry** | F-12 stays closed by construction: one section renderer writes both surfaces, so `form: substance` and `form: excerpt` appear on the block and the tool result alike. **`form: excerpt` must carry what is withheld**, or the contract is a label |
| **The unguarded wire** | §8.1.1 records one: `dispatchRecall` copies the dial onto the tool exchange, and at the shipped dial the zero value and the constant coincide so no test sees a missing copy (`internal/loop/turn.go:537`). **The checks inherit it and make it worse** — a dropped copy means admission charges under the checks while the tool result renders without them. The dial-move change must make **the band and the check configuration injectable together**, or it creates a second unguarded wire beside the first |
| **Error handling** | A refusal is **not an error**. It is a form decision with a recorded reason, like a missing substance. Nothing fails, retries, or logs at error level; the row renders as content |
| **Cost** | One pass over bytes already in memory, ≤ 20 candidates, twice per turn |

---

## 11. Falsifiers, with named consequences

| # | Falsifier | Fires against | Consequence if it fires |
|---|---|---|---|
| **F-W1** | **Positive control on the accuracy check, reproduced in the loop.** The in-loop check, run over the 21 condense substances, **must** return **exactly** `PROCESSOR_DIVOVOID_URL` (10861) and `LllValidationService` (11140) — the same two rows #14712 §8's offline screen returns, and nothing else | U1, before any figure from the check is quoted | **The offline screen has already passed this control; the in-loop implementation must reproduce it.** If it returns fewer, the check is blind; if more, its class is too wide. `how-we-know-a-change-helped.md` §12.11: *an instrument whose guard cannot fail is not an instrument*. No figure published until green |
| **F-W2** | **False-refusal rate after normalisation.** Report accuracy-check refusals on the prose stratum, by cause, after the §8.2.1 normalisations | U1 | The baseline is 51 flagged rows across 376, **none a fabrication**. If normalisation does not bring it well below that, the token class is drawn too wide and must be narrowed — the cost is bytes, but a gate that refuses a large fraction of the only usable stratum is not a gate, it is an off switch |
| **F-W3** | **The band's ceiling is partly a producer proxy.** 0.25 sits just below the condense stratum's minimum of 0.2501. Report both strata's ratio distributions on every sweep | §8.3's threshold | The moment the strata's distributions overlap, the ceiling stops separating producers and **must be justified on economics alone or re-derived**. The floor is unaffected — it has an independent measured derivation |
| **F-W4** | **No fill writes a substance that fails the checks** at the tier its node requires | U3, and standing | The fill is writing into the shared graph the exact class §4.5 identifies. **A correctness failure, not a tuning question** — the producer is wrong and must be replaced, not re-prompted (D1) |
| **F-W5** | **F-CAP limb 2, restated harsher.** A required document that goes from *admitted in content form* to *admitted in substance form where that substance fails the checks* counts as **lost**, exactly as if cut | The composition, and `k` | **Node 10943 fires this today.** Under the current wording it does not, because 10943 remains admitted — which is the blindness §4.4 documents. Restated under §5.1's discipline: before the re-measurement, strictly harsher, prompting failure recorded |
| **F-W6** | **The floor's value.** Re-derive the enumerated-item knee from the checks' own recorded verdicts once U1 has run over live traffic, and compare against #14712 §9's 16/8/1 banding | The floor | If the knee is not at ≈ 0.10 on live traffic, the floor moves — **by measurement, never by argument**, and never upward without re-deriving the economics threshold with it, since narrowing the band from both sides can empty it |
| **F-W7** | **Extraction reaches the band.** An extractive pass over the same 25 corpus nodes must reach a median ratio **inside `[0.10, 0.25)`** and pass the constructive check on every node it writes | D1's recommended direction | If it cannot compress into the band, extraction buys verifiability at no byte saving and no model producer has a route in — **the fill then has no supply and D1's "producer of last resort" role is empty**. If the constructive check fails on its own producer's output, the producer is not extractive and the claim is void |
| **F-W8** | **Off-position identity, and it must assert *both* bounds.** With the checks present, the floor at its off position and the threshold at zero, the block is **byte-identical** to today and **every disposition's cut reason is identical to today's** | U1, U2 | Same shape as #14282's guard and F-CAP limb 4, with D8's addition. #14282 measured that a single-arm guard at zero *"would have been decoration"*; **a guard asserting only the threshold would be decoration for the floor** |
| **F-W9** | **The instrument is representative.** The corpus's substance population must cover the strata in proportions that let the sweep say something about the graph | U5 | Today the corpus is 91% condense-produced against a graph that is 90% agent-written (§4.6). A sweep on the current fixtures measures a stratum the product will not use; its cells may not be quoted as product behaviour |
| **F-W10** | **Directive-check yield.** Report how many substances the directive check refuses, and hand-adjudicate a sample against the ~⅓ paraphrase rate #14712 §9 measured | U1 | If nearly every directive-bearing substance is refused *and* adjudication shows most are paraphrases, the check is measuring wording rather than meaning and must be narrowed to unambiguous markers. **If adjudication confirms the losses are real, that is the finding**, and it bounds how much of the graph the form rule can ever serve |
| **F-W11** | **F-7 re-run, unchanged.** Substance must remain unembedded (A3) | The fill, standing | If DiVoid ever embeds substance, retrieval becomes path-dependent on what earlier runs condensed and §7.4.4's structural repair is insufficient. Four calls; nothing would announce the change |

**Retired by this document:** **F-11**, discharged with verdict NO (D7). **Amended:** F-CAP limb 2 → F-W5.
**Unchanged and still open:** F-1, F-2, F-2a, F-3, F-4, F-5, F-8. **F-3 gains two preconditions** — it must
set **both** bounds, and it cannot set either from a population the checks will refuse (F-W9).

---

## 12. Risks & Mitigations

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| **W1** | **The checks are read as a fidelity certificate.** They bound fabrication, one omission class, and over-compression. They do not certify a true account | §8.2 states each bound in the same table as its guarantee; the disposition records which check passed, not *verified* | Any report, PR body or node describing a checked substance as *verified*, *audited* or *faithful* |
| **W2** | **The floor is quietly lowered to recover bytes.** It is the one bound whose loosening directly buys the thing the product wants | F-W6 makes moving it conditional on a re-derivation from live verdicts; D8 makes it non-optional | The floor moved with no re-derived knee cited |
| **W3** | **θ or `k` is set from #14713's grid.** The obvious source, conditioned on a population that will not render | D6; F-W9; §4.6's withdrawal | Any θ or `k` justified by #14713 without a re-run under the checks |
| **W4** | **The fill is enabled by configuration before its producer can enter the band**, writing the ≥ 8 KB class at 0-of-5 measured fidelity into the shared graph | U3 makes the closure structural rather than environmental; F-W4 | `PROCESSOR_CONDENSE_MODEL_URL` set while the fill's producer is abstractive |
| **W5** | **The accuracy check refuses a substance that knows more than its content** — five measured cases, one *more precise than its own content* (12966) | **Accepted, with the cost named.** The refusal costs bytes, not correctness (§8.2.1). No content-only check can ever fix it | F-W2 |
| **W6** | **The directive check is defeated by paraphrase**, and a substance that says *"same unit of work, one clause"* passes for *"Do not expand this"* | It is not: a paraphrase produces a **false refusal**, not a false pass. The failure direction is safe | F-W10 |
| **W7** | **The band is read as producer selection** and someone proposes tightening it to exclude a producer rather than to bound compression | §8.3 labels the ceiling's producer coincidence as a proxy; F-W3 retires it the moment the distributions overlap | Any argument for a band value that cites a producer rather than a measured loss rate |
| **W8** | **The 11 defective substances are "cleaned up" by a well-meaning later pass** | D2. It belongs in *what must not happen*, because tidying a known-bad set is the most natural thing in the world to do | Any write to an eval-corpus node's substance outside a deliberate, recorded re-baseline |
| **W9** | **S2 is read as broken.** The checks look like provenance and are not | The predicate names no author; the checks take two byte strings (§7.2). **S2 is about the node's author; the checks are about the artifact's properties — and A5 leaves nothing to branch on even if one wanted to** | Any branch in the payload seam reading `SelfProduced`, a node type, or an owner |
| **W10** | **A substance passes every check and is still a bad selection of true material.** The three prose clause-level FAILs are this shape | **Real, and not closed by this design.** Bounded by the constructive tier where available and by the directive check where the loss is a prohibition; otherwise it is the residue §13 rules on | Any judged FAIL rendering after U6 |
| **W11** | **This document is cited as clearing the dial to move.** It does the opposite: it adds three checks, adds a second bound, and withdraws the value | §14's sequence; D6; D8 | Any change moving `SubstanceRatioThreshold` off zero that does not introduce the floor in the same change and cite U1–U5 as landed |

---

## 13. What is now settled, and the one condition still open

Revision 1 pre-registered a branch on the deeper sample. **The sample has landed, so the branch resolves
rather than waits.**

**Resolved in favour of shipping something:**

- **Accuracy is a producer property and the good producer is 90% of the graph** — 0 fabrications in 398,
  measured over the whole stratum with a demonstrated-sensitivity screen. **This is strong enough to build
  on.**
- **Completeness has a measured knee and the region above it is usable** — 1% item loss at 0.10–0.25, with
  two independently judged PASSes at 24,861 B and 37,256 B inside it. **The form rule is not confined to
  small nodes**, which was revision 1's pessimistic branch and is now withdrawn.
- **Age is flat**, so nothing is waiting for the stratum to improve.

**Resolved against the shape revision 1 proposed:**

- The **size ceiling** is withdrawn (D5). The **economics-only reading of the ratio** is withdrawn (§8.4).
  The **provisional θ ≈ 0.15** is withdrawn and recorded as an error (§13a).

**The one condition still open, and it is the honest limit of this design:**

> **Reader-directed prohibitions do not survive at any ratio the form rule would use** — 12% retention inside
> the band, 55% at ≥ 0.25, both before the ~⅓ paraphrase discount. **Check 3 turns that from a silent loss
> into a refusal**, which is the right failure direction, but it may refuse so much of the directive-bearing
> population that the rule serves only the nodes that carry no instructions.

**F-W10 is what decides it, and the decision rule is stated now rather than after the number is known:**

| F-W10 reports | ruling |
|---|---|
| most refusals are paraphrases | narrow the check to unambiguous markers and proceed |
| most refusals are real losses, and directive-bearing nodes are a minority of retrieved traffic | proceed; the rule serves the rest and refuses the rest honestly |
| most refusals are real losses, and directive-bearing nodes are the bulk of retrieved traffic | **the form rule does not earn its complexity. `SubstanceRatioThreshold` stays at zero, the crowding problem belongs to the cap alone, and the payload lever is closed** — with extraction (F-W7) as the only route that could re-open it |

### 13a. My own error, recorded

**Revision 1 concluded that *"the ratio is an economics dial, not a fidelity instrument"* and recommended a
provisional θ ≈ 0.15 as an upper bound, on the reasoning that it separates the good producer from the bad
one. Round 2 shows that region — below 0.10 — is where 16% of enumerated findings and ~85% of prohibitions
leave no trace.**

**The recommendation selected for the failure mode.** Three things follow and belong on the record rather
than absorbed quietly, in the shape `a-falsifier-measured-on-an-empty-set.md` §13 set:

1. **It is the same error §8.1 made, reproduced by the person who had just finished naming it.** §4 of
   #14712 says *"the stratification that makes the mechanism pay for itself selects precisely the substances
   that fail."* Revision 1 quoted that, then chose a bound on the same axis, in the same direction, one level
   down — because the quantity it was optimising (*separate the producers*) happened to point the same way
   as the quantity that fails (*compress harder*).
2. **The error ran in the direction that flattered the design.** θ ≈ 0.15 made the mechanism look shippable
   with a small, clean number. An error favourable to one's own conclusion is the worse kind, because it is
   the kind nobody audits. It was caught by a measurement that had been asked for, which is the only reason
   it was caught.
3. **The general lesson, and it is the reason D8 exists:** *a bound chosen because it separates two
   populations is a proxy, and a proxy adopted without an independent derivation will eventually select for
   whatever else correlates with it.* The floor in §8.3 has an independent derivation (the measured knee).
   The ceiling does not have a fully independent one, which is why F-W3 exists and why §8.3 labels it.

**Revision 1's structure survives because its falsifiers were written to be falsified.** The claim that the
ratio is an anti-proxy was correct as arithmetic and wrong as a conclusion; what caught it was that the
document had already asked for the measurement that killed it.

---

## 14. Migration and sequencing

**Six units. Each ships alone. No intermediate state is worse than today** — the ordering is derived from
the failure #14713 demonstrates when the cap is considered alone.

| # | Unit | What it changes for a user | Why it cannot be later |
|---|---|---|---|
| **U1** | **The three checks, at the off position.** The payload seam gains accuracy, directives and the band (floor and threshold both at their off positions); dispositions gain verdicts and reasons | **Nothing.** The block is byte-identical and every cut reason unchanged (F-W8) | It is the instrument every later unit is measured with. Its day-one product is the figure F-11 tried to produce by audit, computed continuously over the population retrieval touches — including **F-W2** and **F-W10**, which §13's ruling depends on |
| **U2** | **The excerpt contract.** `form: excerpt` split from `form: substance`, governed by the constructive check alone, no band, header names what is withheld | **Nothing at the off position** | It must exist before any dial moves, or the first dial move renders run-record headers under a label that misdescribes them — and the band would refuse them at 0.023 anyway |
| **U3** | **The fill's producer, closed by construction.** The fill may write only a substance that passes the checks; an abstractive producer therefore cannot be wired in | **Strictly better.** Off-by-environment becomes off-by-construction | §4.5. The only unit that removes an existing hazard rather than adding a capability, and independent of everything else |
| **U4** | **Re-compose the backfilled run records** so their substance is a provable prefix of their content, as the live write-back path already produces | **Nothing today** (an 80 KB record is not admitted in content form regardless) | Small, cheap, and it converts 22 nodes from an unprovable guarantee to a provable one with no new mechanism (§8.5) |
| **U5** | **A warranted substance population for the eval corpus — as a repo fixture, never a graph write.** Re-derive the corpus nodes' substances through a producer that can enter the band, held beside `derivations.json` with pinned hashes | **Nothing.** Instrument work | **The unblocking unit.** §4.4's empty intersection is a property of the corpus's substance population; §4.6's 91%/90% inversion means the current fixtures measure the wrong stratum. Until this lands, no sweep cell means anything about the product and neither bound has a derivation |
| **U6** | **Both bounds, together, and then the composition re-measured.** The band leaves its off position — **floor and threshold in the same change (D8)**; F-3 re-derives both from the checked population; the sweep is re-run under the checks; **only then** is `k` chosen and the cap moved | The first real change | Every input it needs is produced by U1–U5 |

**Three things must travel inside U6** rather than after it, because all three are cheap now and expensive
later: **the floor ships with the threshold** (D8); **the band and the check configuration become injectable
into `Turn` together** (§10's unguarded wire — otherwise the dial move creates a second one); and **the cap's
own move off its off position is the same change**, per `a-falsifier-measured-on-an-empty-set.md` §12's M-f:
*"one change moves both numbers."*

**What ships to the product owner at each point**, so the *"it has failed to ship for weeks"* pressure meets
something real: **U1** delivers the first continuous measurement of substance trustworthiness the project has
ever had, over the live population, at zero risk — and it is what decides §13's open condition. **U3** removes
a live hazard. **U4** and **U5** make the instruments honest. **The concept's value is not deferred by this
sequence; the *dial* is, and the dial was never the thing that produced value.**

---

## 15. Open Questions

| # | Question | Blocking? | Recommendation |
|---|---|---|---|
| **QW1** | **What exactly is a "code-shaped token"?** The accuracy check's power and its false-refusal rate both live here | **No** — U1 ships a narrow definition and F-W2 reports the rate | #14712 §8's adjudication is the specification: backtick spans, `SCREAMING_SNAKE`, `mixedCaps` with an internal capital, dotted/slashed paths, numerals — **with case and plural folding, an exemption for notation the substance defines in its own first line, and proper handling of comma-separated id lists, ranges, `SHA-256`, numeral-for-word and rounding**. Widen only on measured false negatives |
| **QW2** | **What exactly is a "reader-directed prohibition"?** Check 3's whole behaviour | No; F-W10 measures it | Start with the markers #14712 §9 used — `Do not` / `must not` / `may not` — and nothing else. A wider class buys refusals nobody can adjudicate |
| **QW3** | **Can a model produce an extractive condensation *inside the band*?** D1's last-resort role rests on it | No — a measurement, F-W7 | Run it on the same 25 nodes #12984 used. One pass, one comparison, the corpus is already instrumented |
| **QW4** | **Should the elision marker carry a size?** *"[… 230 lines elided …]"* tells a reader more than *"[…]"*, and 10943's failure is a reader not knowing something was missing | No | Carry it. Free at production time, and it is the difference between *something is missing* and *a lot is missing* |
| **QW5** | **Is `internal/measure`'s write-suppressing condense port the right home for U5's fixture generation?** It already models a fill's success without writing | No | Likely, and it would mean U5 needs no new write path at all. Confirm before briefing U5 |
| **QW6** | **Does the band's ceiling survive contact with live traffic?** F-W3's proxy, and whether 0.25 is defensible on economics alone | No | Report both strata's ratio distributions per sweep from U1 onward, free |
| **QW7** | **What is the prose stratum's ratio distribution *inside* the band?** Everything about how much the form rule can ever serve depends on how many substances land in `[0.10, 0.25)` — and the stratum's median is **0.096, just below the floor** | **It bounds the whole prize** | Compute it in U1 from the disposition record. **If most of the stratum sits below the floor, the rule serves a minority of the population it was built for, and that must be known before U6, not after** |
| **QW8** | **Does the run record's substance remain a byte-prefix under every composition path?** `ComposeRunContent` makes it so; nothing pins it | No | A guard asserting the prefix relation on the composed content, in the same package. One relation, and it is that class's entire warrant |

---

## 16. Implementation Guidance for the Next Agent

**No code in this document. Each unit is its own branch and its own PR.**

### U1 — the three checks, inert

1. Put them **inside the payload seam** (`renderedPayload`), not beside it. One call site is what makes the
   composition with the cap a property rather than a convention (§8.1.1).
2. Keep them **pure** — two byte strings in, verdicts and reasons out. No graph, no clock, no model, no field
   naming a producer (§7.2, and A5 says there is none).
3. Implement in the order §7.1 gives, which is increasing expected false-refusal rate, so the recorded reason
   names the strongest objection.
4. The **band is two-sided from the first line of code** (D8). A floor added later is a second change to the
   same predicate and a second chance to ship the defect.
5. Record verdicts on **every** disposition, admitted and cut alike.
6. **Guard the off position on both bounds, at two thresholds.** #14282 measured that a single-arm guard at
   zero *"would have been decoration"*; a guard asserting only the threshold is decoration for the floor.
7. **Run F-W1 before quoting any figure.** The offline screen already returns exactly 10861 and 11140 on the
   condense control; the in-loop check must reproduce that, exactly.
8. Publish **F-W2**, **F-W10** and **QW7's distribution** in the same PR body. Together they are the first
   honest numbers on the population since the audit, and §13's open condition turns on them.

### U2 — the excerpt contract

1. A third `Form`, not a flag on the second.
2. Its header names **what is withheld**, not merely that something is.
3. Governed by the constructive check only. **No ratio bound reaches it** (D4) — at 0.023 the band would
   refuse the one stratum with an independent audit behind it.
4. Add QW8's guard in the same change.

### U3 — the fill's producer, closed by construction

1. The fill may write only what the checks accept. **Do not express this as an environment check** — the
   point is to replace off-by-environment with off-by-construction (§4.5).
2. The refusal is a **recorded reason** on the `FillOutcome`, in the existing vocabulary's shape; a silent
   no-op makes the gate invisible (§7.4.4).
3. F-1 remains a gate on any producer the fill uses. Necessary, and per D3 not sufficient.

### U4 — re-compose the backfilled run records

1. Use the live path's composition (`account + fence + record`) so the prefix relation holds by construction.
2. It is a **content** rewrite on 22 run-record nodes. They are not eval-corpus nodes and not instruments —
   **confirm that before writing anything**, because D2's prohibition is absolute for corpus nodes.

### U5 — a warranted corpus population, as a fixture

1. **Nothing is written to the graph.** The fixture lives in the repo beside `derivations.json`, pinned by
   hash — the pattern #14713's provenance table already shows (25/25 rows pinned, harness refuses otherwise).
2. Check whether `internal/measure`'s write-suppressing condense port is the vehicle (QW5) before building.
3. Cover the strata in proportions that let the sweep say something about the graph (F-W9).
4. The corpus's 23 live substances stay on the graph untouched (D2). The fixture is an **arm**, not a
   replacement.

### U6 — both bounds, and the re-measurement

1. Floor and threshold in **one** change (D8).
2. Make the band **and** the check configuration injectable into `Turn` in the same change, or a second
   unguarded wire is created beside the one §8.1.1 records.
3. Re-run F-3 against the checked population, deriving **both** bounds. **`0.592` has no derivation any
   more**, and neither does `0.370`.
4. Re-run the sweep under the checks, then choose `k`. **#14713's grid may not be used** (D6, W3).
5. Move both numbers off their off positions in one change, per §12's M-f.
6. Report **F-W5** on that run, not the original limb 2.

### What must not happen

- **No `SubstanceRatioThreshold` off zero without the floor in the same change** (D8, W11). Shipping the
  threshold alone is shipping the defect with a number that hides it.
- **No substance rendered without every check's verdict recorded**, at any value.
- **No write to an eval-corpus node's substance** outside a deliberate, recorded re-baseline (D2, W8).
  Tidying the eleven is the most natural mistake available here.
- **No θ or `k` derived from #14713's grid** (D6, W3), and **no re-use of `0.592`** without a new derivation.
- **No prompt change presented as a fidelity remedy** (C1, D1). The failure already survived the clause
  written to prevent it.
- **No provenance branch in the payload seam** — no `SelfProduced`, no node type, no owner (C2, S2, W9).
  **A5 says there is nothing to branch on; that is a guarantee to preserve, not a gap to fill.**
- **No fill enabled while its producer is abstractive** (U3, F-W4).
- **No description of a checked substance as *verified*, *audited* or *faithful*** (W1).
- **No exemption for the backfilled run summaries** because their producer is known-good. The loop cannot
  tell them from a model's output and A5 says it never will; the remedy is re-composition (U4).
- **No citation of this document as the crowding fix.** §4.8 measured that crowding is name-driven (R19), and
  §8.3 makes no promise about the large documents a crowding argument reaches for.
- **No claim that F-11 can be re-run to a pass.** It is discharged with a verdict (D7).

---

## 17. Provenance

processor, 2026-09-23, read-only. Baseline `main` = `73dfcce`, read on
`feat/the-reclaimed-budget-needs-an-owner`. **Revision 2** incorporates #14712 §§7–12, which landed after
revision 1 was drafted.

**Read in the tree:** `internal/loop/assemble.go`, `internal/loop/fill.go`, `internal/loop/turn.go`,
`internal/loop/budget.go`, `internal/loop/summary.go`, `internal/condense/condense.go`,
`internal/condense/postprocess.go`, `internal/fill/fill.go`, `internal/divoid/substance.go`,
`internal/divoid/write.go`, `internal/boot/config.go`, `internal/eval/corpus.json`,
`docs/architecture/what-goes-in-the-block.md` (§4, §5–§7.4, §8, §9.1–9.2, §10–§16),
`docs/architecture/a-falsifier-measured-on-an-empty-set.md` (§5, §7, §11, §12),
`docs/architecture/what-a-block-is-worth-per-byte.md` (headings, §7.4 as quoted by §8.1.1).

**Fetched live from DiVoid:** **#14712** (both rounds), **#14713**, **#14282**, **#12984**, **#14561**.

**Nothing was run.** No inference, no sweep, no container, no graph write, no git. Every figure is attributed
to the node or document that measured it. **The one derivation this document performs is a join** — #14712
§3's verdicts against #12984's Q5 ratio table, node by node (§4.2.1) — marked as a join, over two published
tables, reproducible by anyone holding both. Per `a-falsifier-measured-on-an-empty-set.md` §13's own lesson,
**a derived table in an architectural document is a hypothesis with the typography of a result**: §4.2.1's
table is arithmetic over published figures and needs no re-run, but every *consequence* drawn from it in
§4.4 and §4.6 about admission behaviour is a prediction about a sweep that has not been run under the checks,
and F-W9 is what would falsify it. **§13a records where revision 1's own reasoning failed and what caught
it.**
