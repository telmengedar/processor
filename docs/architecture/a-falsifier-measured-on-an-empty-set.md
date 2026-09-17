# Architectural Document: A Falsifier Measured on an Empty Set — F-CAP restated

> **Suggested repo path:** `docs/architecture/a-falsifier-measured-on-an-empty-set.md`
> **Continues:** `the-reclaimed-budget-has-no-owner.md` (`ad3d721`) §9, and
> `what-a-block-is-worth-per-byte.md` (`main`, `5fb97ec`) §7.3 / §9.
> **Judged artefact:** `feat/the-reclaimed-budget-needs-an-owner` at `d4109cd` — the cap, `k = 5`,
> `payloadCap(budget) = budget / 5`, one new cut reason `oversized`, `PayloadCap` on the disposition.
> **Evidence:** QA's 25-row sweep at budget 60,000, independently re-derived from a captured candidate
> set against a second implementation of `admit`. Reproduced arithmetic in §2 is mine, computed from the
> published figures in `what-a-block-is-worth-per-byte.md` §4.2 / §4.3 / §7.3 / §6.3.
> **Out of scope, untouched:** §8.1, the form rule's predicate. Settled, not re-opened, not referenced
> except where the cap's recovery path depends on whether it fires.

---

## TL;DR

**F-CAP is mis-specified, and the reason is not that a 25-row summed instrument is a harsher bar. It is
that both of F-CAP's quantifiers range over empty sets, and the second emptiness is inside the metric that
defines the first.**

The implementer found one vacuity: `∀k ∈ ∅ . P(k)`. There is a second, and it is upstream. **Every
`0.0000` in §7.3's table is `max(∅)`.** The inversion is defined as *highest byte-budget-cut similarity
minus lowest admitted similarity*. At every cap on the plateau, the whole under-cap set fits the budget,
so **nothing is cut for byte budget at all** and the numerator's population is empty. §7.6 says this in
its own voice — *"the inversion vanishes because the byte budget stops binding, not because the boundary
became principled"* — and then §9 built a falsifier whose quantifier domain is *"every value of `k` that
removes the inversion"*, i.e. **every `k` at which the metric stops having anything to measure.**

I verified this against the source document's own numbers. Across twelve cells of §7.3, the correlation is
exact and without exception:

| | under-cap rows fit the budget | inversion reported |
|---|---|---|
| arm ONE, every cap in the table | **yes, all six** | **0.0000 at all six** |
| arm SIX, `budget/3` and `budget/4` | **no** (68,985 B against 56,592) | **0.0272** — non-zero |
| arm SIX, `budget/5`, `/6`, `/8`, `/10` | **yes** | **0.0000** |

**There is exactly one cell in the entire source document where the cap is measured against a budget that
still binds: arm SIX at `budget/3`–`budget/4`. There the cap moves the inversion from 0.0279 to 0.0272 —
a 2.5% reduction.**

**The 25-row corpus moves it from 0.8381 to 0.5148 — a 38.6% reduction.** The corpus is not a harsher
instrument that the cap failed. **It is the first non-vacuous measurement of the cap, and the cap performs
an order of magnitude better on it than on the only honest cell it previously had.**

| | ruling |
|---|---|
| **F-CAP** | **Restated (§5). Not to rescue the PR — the current configuration fails the restatement too.** The restatement is *harsher* on the boundary limb and unchanged on the required limb. |
| **The inversion metric** | **Wrong as defined.** Its cut set excludes `oversized`, which is the cap's own cut reason, so the cap improves the metric partly by relabelling its losses. The cut set must be *every eligible candidate not admitted*. **The published curve does not satisfy the restated falsifier and cannot be retrofitted to it.** |
| **The cap's mechanism** | **Ships. At the off position** — no cap in force, byte-identical, cut reasons unchanged. Unit 1's precedent and my own D3, applied evenly to both mechanisms instead of only to one. |
| **The cap's number** | **Does not ship alone.** `k = 5` and the form rule's dial move together, in a later change, gated on the restated F-CAP measured on the composition. |
| **Sequencing** | **My prior D2 is withdrawn.** *"Ship the cap first"* overrode `what-a-block-is-worth-per-byte.md` §12 without engaging it. §12's R1 — *"the cap loses a large document that was genuinely the answer"* — is exactly what fired. **Its ordering is reinstated.** |
| **`k`** | **5 survives, on a new criterion** (§7): argmin of inversion subject to required-loss at its achievable floor. **`k ≤ 7` becomes a hard constraint**, because at `k ≥ 8` the cap falls below node 10943's 8,030 B substance and the composition can no longer recover it. |
| **F-13 / the required metric** | **The benefit is proven — in the currency the harm was measured in — and unproven in required-node terms, which this design never claimed.** The required metric stays a veto. It is not promoted to the benefit gate. §6. |
| **§6.1 reclamation** | **Not brought forward.** It cannot recover an over-cap document; it addresses the form rule, not the cap. Its stated trigger is unchanged. |
| **New, and it constrains a dial nobody had constrained from below** | The composition recovers 10943 only if the form rule's threshold **strictly exceeds 0.2501**. My §5 predicted the threshold goes *below* 0.592. It is now bounded to **(0.2501, 0.592]** — and that bound rests on one document, which is itself a finding (§7.2). |

---

## 1. Problem Statement

The cap shipped for measurement at `k = 5`. On the 25-row corpus at the product budget it loses one
required document, node **10943** (row `r10`, 32,105 B, rank 1, similarity 0.79526). That document is
larger than `budget/k` for every `k ≥ 2`, so **no calibration of the cap admits it.** F-CAP reads:

> *"If any required document goes from admitted to not-admitted, the cap's value is wrong; if that holds
> at every value of `k` that removes the inversion, the mechanism is wrong and does not ship."*

No `k` removes the inversion on this instrument. The implementer argued the condemning clause therefore
cannot fire; QA answered that `∀k ∈ ∅ . P(k)` is vacuously **true**, so on the implementer's own premise
the clause fires. Read the other way, clause 2 is inapplicable and clause 1 stands unrebutted and
unremediable. **Under either reading the PR loses, and F-CAP can neither pass nor fire cleanly.**

F-CAP is not an optional guard. `admitted ⊇ today's admitted` was withdrawn by #13238 §8.2 and replaced
by F-2; F-2 cannot bind the cap, because making admitted rows not-admitted is the cap's purpose. **F-CAP
is the replacement proof this design owes.** A design cannot ship with its only replacement proof in an
undecidable state.

**The question I am asked:** F-CAP was set by a single-arm measurement and is being read against a 25-row
summed instrument, where *"removes the inversion"* requires 25 rows at zero simultaneously. **Is that the
right bar, or is the falsifier mis-specified for the instrument it is now judged on?**

---

## 2. The finding: the plateau was vacuous, and the corpus is the first honest measurement

### 2.1 What the inversion is

Per §4.2's own definition: **lowest admitted similarity versus highest byte-cut similarity.** A single
number per block, measuring whether the byte boundary is also a quality boundary — *"a 4.9 KB document
scoring at the top of the band loses its place to a 13.4 KB document scoring at the bottom of it, purely
because of rank position"* (§4.3). Good metric. The defect is in its **cut set**.

### 2.2 Every `0.0000` in §7.3 is `max(∅)`

I recomputed the under-cap population at each cap from §4.2's and §4.3's published size columns, against
`remaining = 56,592 B`:

| arm | cap | under-cap rows | their total bytes | fits `remaining`? | §7.3 reports admitted | §7.3 reports inversion |
|---|---:|---:|---:|---|---:|---:|
| ONE | 20,000 | 15 | 49,650 | **yes** | **15** | **0.0000** |
| ONE | 15,000 | 15 | 49,650 | **yes** | **15** | **0.0000** |
| ONE | 12,000 | 14 | 37,623 | **yes** | **14** | **0.0000** |
| ONE | 10,000 | 14 | 37,623 | **yes** | **14** | **0.0000** |
| ONE | 7,500 | 14 | 37,623 | **yes** | **14** | **0.0000** |
| ONE | 6,000 | 13 | 31,450 | **yes** | **13** | **0.0000** |
| SIX | 20,000 | 16 | **68,985** | **no** | 14 | **0.0272** |
| SIX | 15,000 | 16 | **68,985** | **no** | 14 | **0.0272** |
| SIX | 12,000 | 14 | 43,589 | **yes** | **14** | **0.0000** |
| SIX | 10,000 | 14 | 43,589 | **yes** | **14** | **0.0000** |
| SIX | 7,500 | 14 | 43,589 | **yes** | **14** | **0.0000** |
| SIX | 6,000 | 12 | 31,322 | **yes** | **12** | **0.0000** |

**Twelve cells. The admitted counts reproduce to the row and the byte totals reproduce to the byte** —
§6.3's *"size cap alone… 43,589"* is the arm SIX `budget/5` line above. And the correlation is perfect and
exceptionless: **inversion is `0.0000` in exactly the ten cells where the byte-budget-cut set is empty,
and non-zero in exactly the two where it is not.**

§7.6 already confessed the mechanism in words: *"at `budget/5` on this data, **nothing is cut for byte
budget at all** — every above-floor document under the cap fits. The inversion vanishes because the byte
budget stops binding, not because the boundary became principled."* What nobody noticed is that **§9 then
made that vacuity the quantifier domain of the falsifier.** *"Every value of `k` that removes the
inversion"* means, operationally, *"every value of `k` at which the budget stops binding"*.

### 2.3 So the two vacuities are one vacuity

- The implementer's: `{k : inversion(k) = 0}` is empty on the corpus, so `∀k ∈ ∅ . P(k)` is vacuously true.
- The upstream one: on the arms, `{k : inversion(k) = 0}` was non-empty **only because `inversion(k)` was
  itself `max(∅)` there.**

F-CAP's domain was never a set of well-behaved values. It was the set of configurations at which the
measurement had nothing to measure. **A falsifier whose domain is defined by a vacuity cannot fire cleanly
and cannot be passed cleanly.** That is not the 25-row corpus being harsh. That is a defect present in
§9 the day it was written, which the corpus is the first instrument sharp enough to expose.

### 2.4 The corpus is better than what it replaced, not worse

| measurement | cap measured against a **binding** budget? | inversion, off → capped | reduction |
|---|---|---|---|
| arm ONE, every cap | **no** — vacuous | 0.0218 → `max(∅)` | undefined |
| arm SIX, `budget/5`–`/10` | **no** — vacuous | 0.0279 → `max(∅)` | undefined |
| **arm SIX, `budget/3`–`/4`** | **yes — the only such cell in the source document** | **0.0279 → 0.0272** | **2.5%** |
| **the 25-row corpus, `budget/5`** | **yes, on 20 of 25 rows** | **0.8381 → 0.5148** | **38.6%** |
| the 25-row corpus, `budget/10` | yes, on 3 of 25 rows | 0.8381 → 0.0429 | 94.9% |

**Read that table before deciding anything else.** The corpus did not fail to reproduce §7.3's property.
**§7.3 never measured that property.** Against the one cell where the source document measured the cap
honestly, the corpus shows the cap working roughly fifteen times better, plus +66 admitted documents
(169 → 235), mean largest share 39.50% → 21.46%, and rows above 50% share 5 → 1.

**The cap works. The falsifier was written against a number that was not there.**

---

## 3. Scope & Non-Scope

**In scope.** The specification of F-CAP; what the cap must prove and against which instrument; the
shipping configuration and its sequencing; `k`; the status of the required-node metric after F-13; the
bearing of the `r05` anchor defect on any corpus-wide aggregate.

**Not in scope, and not touched.**

- **§8.1, the form rule's predicate.** Settled. This document references only *whether* it fires on one
  node, never *what it says*.
- **The `r05` anchor defect itself.** Pre-existing, not this unit's. §9 rules only on what it does to the
  instrument.
- **`what-a-block-is-worth-per-byte.md` §5's four rejected shapes.** Not re-litigated. §8's new
  alternatives are distinct from them and are named as such.
- **The form rule's threshold value.** §7.2 establishes a *lower bound* on it. Selecting it remains F-3's.

---

## 4. Assumptions & Constraints

| # | assumption | confidence | what breaks if wrong |
|---|---|---|---|
| **A1** | QA's inversion uses §4.2's definition and counts **only** `byte budget exceeded` rows in the numerator, excluding `oversized`. | **High.** §6.3's *"size cap alone → 0.0000"* on arm SIX is only reachable this way: `#6375` at 0.65868, the second-highest similarity in that window, is cut by the cap there, and counting it would give ≈0.028, not zero. | If `oversized` *is* already counted, §5's limb 3 is already satisfiable as written and the published curve is honest — but the curve must still be re-run, because A1 must be *established*, not assumed. **This is the first thing to check and it is cheap.** |
| **A2** | `remaining = budget − anchor`, and the cap is `budget / k`, not `remaining / k`. | **Certain** — `assemble.go` at `d4109cd`, and §7.2 states the choice deliberately. | — |
| **A3** | Node 10943's 8,030 B substance exists in the graph now and is a genuine condensation of its 32,105 B content. | **Reported, not verified.** | §7's entire recovery path. **This is F-11's zero-tolerance clause landing on a named node.** |
| **A4** | Every corpus row carries exactly one required node; 25 rows, 23 labelled + 2 control. | **Verified** against `internal/eval/corpus.json`. | — |
| **A5** | `r05` is the only degenerate row (anchor ≥ budget). | **Unverified — assumed false until enumerated.** Only `r05` was reported, but nobody swept for others. | Every per-row mean in the QA table. §9. |
| **C1** | The block budget is 60,000 B and #11364 says it will fall. | stated | §7.1's `k ≤ 7` constraint is budget-relative; at a lower budget the substance may exceed the cap. T3. |

---

## 5. The restated falsifier

### 5.1 The discipline first, because this is the part that can be abused

A falsifier that has fired may be restated. It may not be *loosened* to escape the firing. The rule I hold
myself to here, and which should bind the next person in this position:

> **A falsifier may be restated only (a) before the re-measurement it will judge, (b) in a direction that
> is derivable from the design's own stated claims without reference to the observed numbers, or strictly
> harsher, and (c) with the failure that prompted the restatement recorded alongside it.**

**This restatement does not rescue the current PR.** The configuration on the branch — cap at `k = 5`,
form rule at the dial's off position — **fails limb 2 under the old wording and under the new, identically.**
What changes is not the verdict on that configuration. What changes is *which configuration is the one
that ships*, and whether the old wording's condemnation extended to the **mechanism** or only to the
**number**. Under the restatement it reaches only the number — and it reaches it just as hard.

### 5.2 What replaces "removes the inversion"

Dropped entirely, with the reason on the record: **it was never a property of a binding budget.** The cap's
own §7.6 predicted its absence on any corpus where the budget binds, and the corpus is such a corpus. The
honest pass condition on a heterogeneous multi-row instrument is **not** a threshold on a sum. A sum over
25 rows is dominated by whichever rows have the widest similarity spread and cannot have a bar set on it
that means anything. **The inversion is reported as a distribution and gated directionally.**

### 5.3 F-CAP, restated

> **F-CAP (restated). Measured on the 25 corpus rows at the product budget, on the shipping
> configuration — the mechanism as it will actually run, not on any one half of it.**

**Limb 0 — the cut set, which is a definition and not a gate.** The inversion's numerator is taken over
**every candidate that passed eligibility and was not admitted, whatever its cut reason** — `oversized`
included. `self-produced` and `below relevance floor` rows are excluded: they are not eligible material,
and the boundary under measurement is the *byte* boundary among eligible material.
*Rationale, and it is not negotiable:* `oversized` is the cap's own cut reason. A metric that excludes it
lets the mechanism improve its own score by relabelling its losses, and an `oversized` cut of a rank-1
0.79526 document is not the absence of an inversion — **it is the most severe inversion the window can
express**, the single best candidate excluded while worse ones are carried.

**Limb 1 — occupancy. Per row. The product commitment, and a theorem, not a hope.**
For every **non-degenerate** row: `admitted ≥ min(eligibleCount, floor(remaining / cap))`.
A row is **degenerate** when `remaining < cap`; degenerate rows are enumerated with their anchor sizes,
reported by name, and excluded from every aggregate.
*Fails if:* any non-degenerate row admits fewer. *Concretely fails if:* the cap is evaluated against
`remaining` instead of `budget`, or placed before the floor, or an anchor swallows the budget.

**Limb 2 — the required veto. Per document. Absolute, at the shipping configuration only.**
No required document may go admitted → not-admitted between current production behaviour and the shipping
configuration.
*Fails today, on the cap alone, on node 10943.* Must be re-run on the composition.
*Escalation, replacing the vacuous quantifier:* if it fails on the shipping configuration, **name every
lost document, its substance size, and the `(k, threshold)` region that would recover it.** If that region
is empty, **the number does not ship** — and the mechanism is referred back with a named, non-empty
obligation rather than condemned by a quantifier over nothing.

**Limb 3 — the boundary. Directional, on limb 0's cut set. Three numbers, never a sum:** rows inverted,
median per-row inversion, **maximum** per-row inversion.
Pass requires **both**:
- **(a)** rows inverted falls, **and**
- **(b)** the **maximum per-row inversion does not rise** above its value with the mechanism off.

*Rationale for (b), derived from the design's claim and not from the curve:* the cap's claim (§7.6) is that
it *bounds one measured source* of inversion — a single outsized document setting the boundary for
everything behind it. **If the mechanism creates a worse boundary violation on some row than anything
present today, it has not bounded that source; it has moved it.** That is exactly what an `oversized` cut
of a top-ranked document does, and under limb 0 the metric can now see it.

**Limb 4 — inertness at the off position.** With no cap in force, the block is byte-identical to today
**and every disposition's cut reason is identical to today's.** Not merely the bytes: the run record's
reasons are the input to Unit 2's condensation work queue (§7.5), so a reason that changes at the off
position pollutes a downstream mechanism silently. *Fails if:* the off position is expressed as `k = 1`
(cap = budget), under which a candidate larger than the budget is relabelled from `byte budget exceeded`
to `oversized`. **The off position must be "no cap in force", with `PayloadCap` recorded as absent.**

### 5.4 Every limb can fail — demonstrated, not asserted

| limb | a concrete world in which it fails | does it fail today? |
|---|---|---|
| **1** | an anchor consumes the budget, or the cap is computed from `remaining` | **yes, degenerately, on `r05`** — and reporting that is the point |
| **2** | a required document exceeds the cap and carries no usable substance | **yes, on the cap alone** — node 10943 |
| **3(a)** | the cap excludes so much that rows fall below occupancy and new inversions appear | no at `k ≤ 7`; at `k = 10` required loss reaches 4 |
| **3(b)** | a top-ranked high-similarity document is cut `oversized` with nothing recovering it | **very likely yes on the cap alone, once limb 0 is applied** — 10943 at 0.79526 is the highest similarity anywhere in the corpus report |
| **4** | the off position is `k = 1` rather than "no cap" | **yes, as currently specified** |

**Three of five limbs fail against the branch as it stands.** That is what a falsifier is supposed to look
like after it has been restated honestly.

---

## 6. F-13: which is it — wrong gate, or unproven benefit?

The constraint I was given: at the product budget the corpus cannot show this work's benefit. One required
node is unreachable by any allocator; one is cut at rank 1 by a 72,400 B anchor against a 60,000 B budget;
the third is rank 18. Benefit lands entirely on unlabelled rows. *"That is either an argument that the
required-node metric is the wrong gate, or an argument that the benefit is unproven. Say which."*

**Neither, exactly — and the correct answer is available only because my own §8 Seam 3 already committed
me to half of it.**

Seam 3 says: *"the rule's motivating harm is the starvation of unlabelled rows, so a metric blind to them
is blind to the thing this exists for."* I wrote that to stop a passing F-2a excusing a failing F-2. **The
symmetric statement is now forced on me:**

> **You cannot count unlabelled losses as harm and refuse to count unlabelled gains as benefit.**

The harm this unit exists to fix was stated in unlabelled currency: *"one 43 KB document takes 77% of the
block and strands ten above-floor rows behind it."* Not one of those ten is a labelled required node. The
benefit is measured in the same currency: **+66 admitted documents across 25 rows, mean largest share
39.50% → 21.46%, rows above 50% share 5 → 1, inversion −38.6% on the honest cut set's predecessor.**

So:

1. **The benefit is proven, in the currency in which the harm was stated.** It is not "unproven". It is
   proven in unlabelled admissions, which is what the design claimed and the only thing it claimed.
2. **The benefit is unproven in required-node terms and will stay that way on this corpus**, for the three
   structural reasons F-13 found. That is not a defect in the mechanism; it is a statement about the
   corpus's labelling, and it was predicted — my own §11.2, *"nobody has measured the benefit"*, and F-13
   was filed to find out. **F-13 worked. It returned an answer nobody wanted, which is the only kind of
   answer a cheap gate is worth running for.**
3. **The required-node metric is therefore not the wrong gate. It is the wrong *benefit* gate, and the
   right *veto*.** It keeps its absolute force in limb 2 and is never promoted to carry the positive case.
   A veto that cannot show benefit is still a perfectly good veto; that is what a veto is.

**What this costs, stated once so nobody has to rediscover it:** the corpus can veto this unit and cannot
endorse it. Every positive claim rests on aggregate unlabelled measures that no label validates. **That is
an argument for building a corpus row whose required document is cut for byte budget behind an outsized
neighbour and recovered by the cap** — the row that would let the required metric speak on the benefit
side. It is a corpus-authoring task, it is cheap, it is not this unit's, and without it every future
size-aware change will land in exactly this position. **File it.**

---

## 7. `k`, and the constraint nobody had written down

### 7.1 `k = 5` survives, on a criterion — and the criterion is new

§7.3's criterion was *"the loosest value that works"*, where "works" meant "reaches zero". Nothing reaches
zero, so that criterion is dead with the plateau. The replacement, derivable from the design's claims:

> **Choose the `k` that minimises the inversion, subject to required-loss being at its achievable floor,
> and subject to every lost required document remaining recoverable by the composition.**

Against the measured curve:

| k | required lost | inversion | admitted | mean largest share | verdict |
|---:|---:|---:|---:|---:|---|
| 2 | 1 | 0.6901 | 183 | 35.00% | dominated by k=5 on every column |
| 3 | 1 | 0.6674 | 194 | 30.68% | dominated by k=5 on every column |
| 4 | 1 | 0.6201 | 215 | 25.67% | dominated by k=5 on every column |
| **5** | **1** | **0.5148** | **235** | **21.46%** | **chosen** |
| 6 | 2 | 0.4061 | 243 | 19.92% | second required loss, recoverability unknown |
| 7 | 3 | 0.2191 | 245 | 19.17% | third loss; cap 8,571 still ≥ 8,030 |
| 8 | 3 | 0.2092 | 240 | 18.60% | **cap 7,500 < 8,030 — the composition can no longer recover 10943** |
| 10 | 4 | 0.0429 | 227 | 17.49% | same, and admitted falls |

**`k = 5` dominates `k ∈ {2,3,4}` on every single column at identical required cost.** It is not a
compromise; it is the strict optimum of that region. And the number is unchanged from §7.3, which matters:
**I am not moving the constant to escape the failure.** The old justification is dead, the constant it
produced is re-derived from a live criterion, and it lands in the same place. Two independent derivations
already agreed on 12,000 (§7.3 reason 2); this is a third.

**`k ≤ 7` is a hard constraint, and it is new.** At `k ≥ 8` the cap (7,500 B) falls below node 10943's
8,030 B substance, so the one document limb 2 condemns the cap for losing becomes unrecoverable *even by
the composition*. Any future argument for a tighter cap must first answer that.

### 7.2 The threshold now has a lower bound, and it is fragile

Node 10943: content 32,105 B, substance 8,030 B, **ratio 0.2501**. §8 Seam 2 fixed the boundary as strict.
So the form rule fires on this node only if **threshold > 0.2501**.

My §5 recorded a prediction that the threshold belongs **below** 0.592. It is now bounded on both sides:

> **threshold ∈ (0.2501, 0.592]**, if limb 2 is to pass through the composition on this corpus.

At the candidate values: **0.592 fires, 0.4 fires, 0.2 does not.** A threshold of 0.2 loses 10943 under the
composition exactly as the cap alone does.

**And that bound is fragile, which is itself the finding.** It rests on **one document**. If 10943's
substance is regenerated slightly longer, or the node's content shortens, the bound moves and the dial
follows it. **Pinning a system-wide dial to one corpus document is not a way to choose a dial.** The right
move is to select the threshold from the **distribution of content-to-substance ratios across the required
documents**, not from the argmax of a set of size one — and the bound above should be treated as a *floor
under discussion*, not as the answer. F-3 owns that selection; this document owns the observation that F-3
now has a constraint from below that it did not have yesterday.

### 7.3 The composition is not circular, because the order was a choice and I am changing it

The question put to me: *does 10943's recoverable substance change what the cap must prove alone, or is it
circular given the form rule ships after?*

**It would be circular if the cap had to ship first by necessity. It does not — and "ship the cap first"
was my ruling, not a constraint.** `what-a-block-is-worth-per-byte.md` §12 ordered the payload change
first, and said why in R1: *"The cap loses a large document that was genuinely the answer… This risk is
the argument for sequencing the payload change first, and it is why §12 does."* My D2 reversed that
ordering on the grounds that the cap's benefit was measured and the form rule's was not.

**R1 fired, on the exact shape it named. The reversal is withdrawn.** The cap does not have to prove
anything alone, because it will not run alone. **What it must prove alone is only that it is inert at the
off position (limb 4).**

---

## 8. The alternatives, by name

**Option 1 — restate F-CAP for this instrument. ADOPTED (§5), and it does not save the branch.** The
restatement is harsher on the boundary limb (limb 0 makes the cap's own cut reason count against it),
unchanged on the required limb, and it replaces the vacuous quantifier with a named escalation obligation.
Three of its five limbs fail against `d4109cd`.

**Option 2 — ship the cap enabled at `k = 5` as it stands, accepting the required loss.** *Loses on my own
words.* D2 refused to ship the form rule because doing so *"ships a measured regression in exchange for an
unmeasured benefit."* The cap's position is genuinely better — its benefit **is** measured (§6) — but it
still ships a measured loss of a required document **whose recovery is already designed, specified and
half-built.** Shipping a loss you have written the fix for is not a trade; it is impatience. *(This option
has a legitimate advocate and the call is partly Toni's — §10 T1.)*

**Option 3 — bring §6.1's reclamation pass forward.** *Loses as an answer to this question, though it may
still be adopted on its own trigger.* §6.1's baseline pass applies *"self-produced, floor and cap exactly
as today"*, so 10943 — 32,105 B of content against a 12,000 B cap — **is cut `oversized` in the baseline
pass and never reaches the reclamation walk.** The reclamation pass cannot recover an over-cap document;
it recovers rows the *form rule's* byte savings displaced. What it genuinely does is make the form rule's
dial safe to move by construction, removing F-2 as a blocker on the composition — so it is an **accelerator
of the path to the composition, not a substitute for the cap.** F-11 still gates the dial regardless, so it
shortens nothing that is actually on the critical path today. **Its trigger in §6.1 is unchanged: adopt if
F-2 still reports a loss with the cap in force.**

**Option 4 — the corpus is the wrong instrument; §7.3's property needs live arms with a size gap.**
*Loses, and it is the most dangerous option on the list.* §7.3 reason 3 already said the plateau existed
*because* arm ONE has a size gap between 6,173 B and 12,027 B — *"every cap in `[7,500, 12,000]` is
identical on this data by accident"* — and §9 said *"this is why the falsifier, not this table, sets the
shipping value."* **The corpus lacking that gap is the corpus being a better instrument, not a worse one.**
§2 shows the arms' property was `max(∅)`. Returning to the arms means returning to a measurement that
cannot see the thing it reports. **Rejected, and the general form should be written down: when a purpose-
built instrument contradicts the ad-hoc probe a design was drafted against, the burden is on the probe.**

**Option 5 — the cap does not ship; reclamation carries the whole fix.** *Loses on mechanism.* Reclamation
recovers rows displaced by the form rule's savings. With the form rule at the dial's off position there are
no savings, so reclamation is a no-op. The originating harm — *"one 43 KB document takes 77% of the block
and strands ten above-floor rows"* — is untouched by it at any dial setting, because nothing in the form
rule or its reclamation pass bounds a single document's share. §6.2's finding stands: *"a ratio multiplies
an unbounded quantity. It compresses; it cannot bound."*

**Option 6 — exempt the top-ranked candidate (or any candidate above a similarity threshold) from the
cap.** *Named because it is the obvious move to save 10943, and it loses decisively.* It reinstates the
defect verbatim: one document may again take the whole block whenever fusion places it first. §13's closing
claim — *"today, whether your block is three-quarters one document is decided by where fusion happened to
place that document; after this, it is decided by a number you can read"* — is **exactly** what a rank-1
exemption un-does. It also cannot be stated as an invariant: occupancy collapses to `1` whenever the
exempt document is large. *Distinct from §5's four shapes; rejected on its own terms.*

**Option 7 — a soft cap: admit an over-cap document only if `k − 1` others can still fit behind it.**
*Named because it is the reclamation idea applied to the cap, and it is the strongest of the losers.* It
would save 10943 (32,105 B leaves ~24,500 B for four more). It loses on two counts. First, it converts the
guarantee from a bound on **share** to a bound on **count**, and share is the measured harm — rows above
50% share, 5 → 1, is the headline the soft cap gives back (10943 alone would sit at 53.5% of the budget).
Second, it is §5.2's rejected reserve with the sign flipped: *"a reserve splits the budget into two greedy
pools"* and introduces a second tunable with no invariant behind it. **The cap's whole claim to be a
decision rather than a fitting is §7.2's invariant**; a soft cap has a number and no theorem.

**Option 8 — ADOPTED. The cap's mechanism lands at its off position; the number and the dial move
together, later, gated on the restated F-CAP measured on the composition.**

Why it wins:

- **It applies my own D3 evenly.** D3 landed the form rule at the dial's off position on Unit 1's
  precedent — *"ship the capability, change no bytes"*. The cap was exempted from that discipline for no
  reason other than that its measurement looked better at the time. It no longer does.
- **It reinstates §12's ordering** without discarding the work: the cap's code, guards and disclosure land
  now; its number lands with the form rule's, which is where §7.4's seam always said the two belong.
- **It is the only option under which every claim that ships is a claim that was measured on the
  configuration that ships.**
- **Nothing is lost by waiting.** The off position is byte-identical, so the delay costs zero behaviour.

**What it requires that today's branch does not have:** a genuine off position (limb 4), i.e. "no cap in
force" rather than `k = 1`, with `PayloadCap` recorded as absent and cut reasons unchanged.

---

## 9. The instrument itself: degenerate rows, and why no aggregate is currently safe

`r05` carries a **72,400 B anchor against a 60,000 B budget.** `remaining` clamps to zero, no candidate is
ever admitted, and the anchor renders in full — **the block ships at 72 KB against a 60 KB budget.**

Not this unit's defect, and I do not rule on it. What it does to the measurement is mine:

1. **The byte budget is not an invariant of the block.** It is an invariant of the *candidate fill* only.
   Every statement of the form "the block is at most 60,000 B" in any design in this repo is false as
   written. **That belongs in a task, not in a footnote.**
2. **The occupancy invariant is vacuous there** — `floor(remaining / cap) = 0` — so limb 1 passes on `r05`
   by admitting nothing. Limb 1 therefore **reports** degenerate rows by name rather than letting them pass
   silently; a limb that is satisfied by a block that violates its own budget must say so out loud.
3. **`r05` consumes a denominator slot while contributing zero to every numerator.** A single degenerate
   row dilutes every per-row mean by 4%. `mean largest share`, `admitted rows`, `rows inverted` and the
   inversion sum are all computed over 25 rows, at least one of which the mechanism cannot touch.
4. **Nobody has swept for others (A5).** Rows with `0 < remaining < cap` are also degenerate for limb 1 and
   would not be noticed by looking for `remaining = 0`.

> **No corpus-wide aggregate is interpretable until the degenerate set is enumerated.** That is one pass
> over the corpus computing `anchorSize` against `budget`, and it must precede the re-measurement rather
> than accompany it.

And a specific consequence worth chasing: **`rows > 50% share` is 1 at every `k` from 2 to 10.** If that
one row is `r05` — where the anchor is 100% of the block and no cap can act — then **the cap removes every
such row it is capable of acting on, 5 → 0**, which is a materially stronger result than `5 → 1`. Cheap to
check, and it is the cap's best measured outcome or a genuine residual defect. One or the other.

---

## 10. Where the call is Toni's, not mine

**T1 — the impatience trade, which is Q2 wearing new clothes.** §11 Q2 already has his name on it: *"what
is the minimum number of memories a block must carry?"* The question has now acquired a price tag, and it
is a product judgement a measurement cannot settle:

> At `k = 5`, on this corpus, the cap trades **one required document — recoverable by the form rule, but
> not today** — for **+66 admitted documents, a 38.6% reduction in summed inversion, and the largest
> document's share falling from 39.5% to 21.5%**. The recovery requires the form rule's dial to leave zero,
> which requires F-11 to pass on 67 substances of unestablished provenance.
>
> **Do we take the trade now, or hold the number until the recovery is live?**

**My recommendation is to hold** (Option 8), because the loss is measured, the recovery is designed, and
waiting costs zero behaviour. **But Option 2 is defensible and I am not going to pretend otherwise:** the
cap's benefit is measured, the form rule's was not, and F-11 has no owner or date. If F-11 is going to
sit for weeks, holding a measured +66 to protect one document whose substance already exists is a real
cost. **That is a scheduling judgement about F-11, which is his and not mine.**

**T2 — Q2 proper, restated with what is now known.** `k` is the sentence *"a block always carries at least
`k` memories."* §7.3 recommended 5 and supported `[5, 8]`. **That range is now `[5, 7]`**: at `k ≥ 8` the
cap falls below the one substance the composition depends on. The evidence supports 5; 6 and 7 buy more
inversion reduction for more required losses whose recoverability is unmeasured (§11 M2).

**T3 — Q3, with a number attached for the first time.** *"When the budget falls below ~49,000 bytes, does
the occupancy guarantee win or does the cap floor at the 8 KB condensation gate?"* Now concrete: **at
`k = 5`, any budget below `8,030 × 5 = 40,150 B` puts 10943's substance over the cap and the composition
stops recovering it.** #11364 says the budget will fall. **This is a dated collision with a number on it.**

**T4 — not a decision, a disclosure.** The corpus can veto this unit and cannot endorse it (§6). Every
positive claim rests on unlabelled aggregates. If that is not acceptable as a standard of evidence for
shipping, the honest answer is not to weaken the claim — it is to author the corpus row that would let the
required metric speak for the benefit, and to say so in the PR body either way.

---

## 11. What must be measured before the cap's number ships

**Ordered. Each gate names the decision it unblocks, so a cheap one is never run after an expensive one.**

| # | measurement | unblocks | cost |
|---|---|---|---|
| **M0** | **Establish A1: does QA's inversion count `oversized` rows in its numerator?** One read of the re-implementation. | Everything. If it excludes them, the entire published curve measures a metric the cap can improve by relabelling, and nothing downstream is interpretable. | minutes |
| **M1** | **Enumerate the degenerate set** (§9): every row's `anchorSize` against `budget`, flagging `remaining ≤ 0` and `remaining < cap`. Report `rows > 50% share` **by row id**. | Any corpus-wide aggregate at all. Also settles whether the cap clears 5 → 0 or 5 → 1. | one pass |
| **M2** | **Re-run the sweep under limb 0's cut set**, cap off and at `k ∈ {2,…,10}`, reporting **rows inverted / median / maximum** and — for one configuration — **both cut-set definitions side by side**, so the relabelling share of the published 0.8381 → 0.5148 is quantified rather than argued. Identify the **second** required document lost at `k = 6` and report its substance size. | Limb 3. `k = 6`'s admissibility (T2). | one sweep |
| **M3** | **Verify limb 1 per row** at the chosen `k`, non-degenerate rows only. | The product commitment behind `k`. | free with M2 |
| **M4** | **F-11, narrowed and made urgent: audit node 10943's substance first**, against its own content, before the other 66. It is a required node of the eval corpus, §8 Seam 4's zero-tolerance clause names exactly that class, and §7's entire recovery path is one unverified 8,030 B blob (A3). | The dial leaving zero. **F-11 is now on the critical path; it was not before.** | one read, then 66 |
| **M5** | **Run the composition** — cap at the chosen `k` **and** the form rule at a threshold `> 0.2501` — and re-run F-CAP limbs 1–3 **on that configuration**. This is also §11.3's outstanding request: **§4's composition table is derived arithmetic and must be reproduced, not quoted.** | Limb 2, and the number shipping at all. | one sweep |
| **M6** | **F-2 / F-2a / F-3 against the composed allocator**, per the prior ruling. If F-2 still reports a loss, **adopt §6.1 as M3a** — trigger unchanged. | The dial's final value. | one sweep |

**What must not happen:** no number off its off position before M5 is green on the composition; **no
inversion figure quoted without saying which cut set produced it**; no corpus-wide mean quoted before M1;
no `k` above 7 without re-opening §7.1; no threshold at or below 0.2501 while 10943 is the binding
document; and **no retrofitting of the published curve to the restated falsifier** — limb 0 changed the
metric, so the existing numbers do not satisfy it and cannot be made to.

---

## 12. Milestones

- **M-a — the branch, corrected.** `d4109cd` lands with the cap **at its off position**: no cap in force,
  `PayloadCap` absent, cut reasons byte-identical to today (limb 4). The rule stays specified **against the
  rendered payload** (§7.4's day-one requirement, already correct in `renderedPayload`). G1–G5 green with
  the cap injected by test rather than by the shipped constant. **`minBlockOccupancy = 5` becomes a test
  fixture, not a production constant, until M5.**
- **M-b — M0 and M1.** Two cheap reads. Either may invalidate the evidence base; neither takes a day.
- **M-c — M2 and M3.** The honest curve, and `k` confirmed or moved on §7.1's criterion.
- **M-d — M4.** F-11, node 10943 first.
- **M-e — M5 and M6.** The composition measured as a composition. **§4's table reproduced or struck.**
- **M-f — one change moves both numbers**: `k` off its off position and the form rule's dial off zero,
  together, with the PR body carrying the restated F-CAP's three reported numbers and the degenerate-row
  list.

**Filed separately, not gating:** the `r05` anchor defect (§9 — a block exceeding its own budget); the
corpus row that would let the required metric measure benefit (§6); N3's ownership of
`what-a-block-is-worth-per-byte.md`, now more acute since its §12 has been reinstated over my D2.

---

## 13. My own error, recorded

**§4 of `the-reclaimed-budget-has-no-owner.md` predicted "cap alone: 7 rows" on `r01`. Run, it is 11.**

The table was derived arithmetic from published figures, and §11.3 flagged it as such and asked for exactly
this reproduction. The flag worked; the correction arrived; the process functioned. Three things follow and
I want them on the record rather than absorbed quietly:

1. **The error runs in the direction that flatters my own recommendation.** I understated how well the cap
   performs alone. An error favourable to one's own conclusion is the worse kind, because it is the kind
   nobody audits. It was caught by a measurement I asked for, which is the only reason it was caught.
2. **The §4 numbers are struck, not amended.** Every cell in that table is derived the same way, so one
   wrong cell condemns the method rather than the cell. **The mechanism it illustrates — cap alone loses a
   row the composition keeps — survives, because the composition's advantage over the cap alone does not
   depend on the arithmetic being right about how many rows each admits.** But no further reasoning may
   quote §4's figures. M5 replaces them with a run.
3. **This is the second time this document family has made a claim from a table rather than a sweep**, and
   both times the table was wrong in a way that favoured the person holding it — §7.3's plateau (§2 here)
   and §4's composition. **The pattern is worth naming: a derived table in an architectural document is a
   hypothesis with the typography of a result.** §11.3 was right to flag it; the fix is that a design
   should mark derived cells in a way that survives being quoted out of the section that disclaimed them.

---

## 14. Open Questions

| # | question | blocking? |
|---|---|---|
| **N6** | Does QA's inversion count `oversized`? (A1 / M0.) | **Yes — everything downstream.** |
| **N7** | How many degenerate rows are there, and is the single `> 50% share` row `r05`? (M1.) | **Yes for any aggregate.** |
| **N8** | Is node 10943's 8,030 B substance among "the 67", and is it faithful? (A3 / M4.) | **Yes for the recovery path.** |
| **N9** | Which required document is the second loss at `k = 6`, and does it carry a recoverable substance? | No — decides whether T2's range is `[5,7]` or narrower. |
| **N10** | Should the form rule's threshold be selected from the distribution of required-document ratios rather than from the single binding node? (§7.2.) | No — but it is the right way to stop a dial being pinned to one row. |
| **N11** | The block can exceed its own byte budget when the anchor does (§9). Which invariant is wrong — the budget's scope, or the anchor's exemption from it? | No, for this unit. **Yes for any future claim that the block is bounded.** |
| **N3** | *(carried)* `what-a-block-is-worth-per-byte.md` has no owner and no branch — and its §12 has now been reinstated over my ruling, which makes the ownership gap sharper, not softer. | **Yes, for sequencing.** |

---

**In one line.** The falsifier could neither pass nor fire because both its quantifiers ranged over empty
sets — the implementer found one, and the other was `max(∅)` hiding inside the metric that defined the
first. **The corpus did not fail to reproduce §7.3's property; §7.3 never measured it.** The cap works, it
works better on the honest instrument than on the probe it was drafted against, and the one document it
loses is the exact case the composition was built for — so the mechanism ships inert, the number ships with
the dial, and F-CAP is restated in the only direction a fired falsifier may be restated: harsher.
