# Architectural Document: What a Block Is Worth Per Byte

> **Repo path:** `docs/architecture/what-a-block-is-worth-per-byte.md` (canonical copy).
> **Driver:** **#13246** — *"Size-aware admission has no unit"*, which cites **#13093** §5 (the mechanism),
> **#13245** (the deterministic render probe's independent corroboration) and **#13238** (payload form).
> **Baseline:** `main` at **`e135e0a`**, worktree `design/what-a-block-is-worth-per-byte`.
> **Graph state:** every similarity, byte count and admission outcome in this document was read on
> **2026-09-17**, between roughly 10:0x and 10:4x, against the live graph (11,713–11,714 nodes reported
> by the search route across the session). The graph is live and unversioned. #13246's own figures are
> from **2026-09-07/08**; §4 re-derives them rather than inheriting them, and says which ones moved.
> **Instrument:** `internal/loop.Retrieve` and `internal/loop.Assemble` themselves, run against the live
> graph through `internal/divoid.Client` — the product's own code, no model call in the admission path.
> §4.1 states the probe and its one substitution.

---

## 0. Read this first — it is not this design, and it is urgent

**`#1487` clears the shipped relevance floor by 0.0001475, and the margin is real.**

| | value |
|---|---|
| `RelevanceFloor` (`internal/loop/turn.go:21`, PR #90) | **0.63** |
| `#1487` similarity, query = the verbatim yardstick input, **2026-09-17** | **0.6301475** |
| margin | **+0.0001475** |
| #13093's figure for the same node and query, **2026-09-07** | 0.630147 |

The figure **re-derives to every digit #13093 published**, ten days and an unknown number of graph writes
later. So does the rest of that table: `#6375` 0.65867996 against 0.658680, `#6371` 0.6456038 against
0.645604, `#10435` 0.6378142 against 0.637814, `#1804` 0.6886747 against 0.688675. The ranking on this
query is stable to seven significant figures across ten days.

**It is not a single-query artefact, and I checked the thing that could have made it one.** Since #13246
was written, query derivation shipped: the loop now asks the raw input **plus up to five model-derived
queries** (`internal/loop/derive.go`), and `fuseByReciprocalRank` charges the floor against the
**maximum** similarity a candidate reached across all of them. So the margin could have been dissolved by
a derived query scoring `#1487` higher. I generated the derived set through the product's own
`DerivationPrompt` against `qwen3-coder:30b` on `gangolf:11434` at temperature 0, and measured all six:

| query | `#1487` |
|---|---|
| raw input | **0.6301475** |
| *"What is the mechanism for initializing a new Git repository?"* | absent |
| *"How does one create a basic HTML file structure using command-line tools?"* | absent |
| *"What steps are involved in setting up a local web server for static files?"* | absent |
| *"What is the process for creating a minimal project scaffold with npm?"* | absent |
| *"barebones webpage repo initialization git html scaffold npm"* | absent |

**`#1487` appears in exactly one of the six lists.** Its floor-charged similarity is therefore
0.6301475, and the margin is 0.0001475.

**The absence is bracketed, and the bracket is tight.** The measuring instrument returns 50 rows where
the loop fetches 100, so `#1487` could sit at rank 51–100 in the five derived lists. It cannot matter:
each of those lists' rank-50 similarity is **0.5787 / 0.5913 / 0.5528 / 0.5900 / 0.6218**, all below
0.6301475. Anything beyond rank 50 is below those. **The maximum is 0.6301475 whatever the tail holds.**

**Two further facts, both measured, that change what the margin means.**

1. **On that query, `#1487` is the lowest-scoring row in the entire top 50 that clears the floor** —
   rank 37, and exactly 37 rows score ≥ 0.63. The floor's cut line does not merely pass near the one
   document whose admission coincided with correct output shape; **it passes through it**, on the
   admitting side, by 0.00015.
2. **It does not currently decide `#1487`'s fate, and that is worse news, not better.** Under the raw
   input alone `#1487` clears the floor and is then **cut by the byte budget** at rank 18 (§4.2). Under
   the shipped six-query derivation it **never becomes a candidate at all** — it does not appear in the
   20-row window. So the floor is not what is keeping `#1487` out of the block today. Two other
   mechanisms are already doing it, and the floor is a third tripwire armed behind them.

**What I am asserting and what I am not.** Asserted: the value, the margin, the single-list appearance,
the bracket. **Not asserted:** that 0.63 is the wrong number. #14112 chose it on a 25-row corpus
measurement and explicitly rejected 0.638 as *"a number fixed by the fourth decimal of a single sample on
a single graph state"*. The finding here is the symmetric one: **0.63 is within a fourth decimal of the
yardstick's key document**, on a graph that is live and unversioned, where a whitespace edit to `#1487`
or a re-embedding moves it. This is filed as **task #14211**; it is not resolved by this design and
this design does not touch the floor's value.

---

## TL;DR

**The 43 KB document is in the block because nothing has ever been allowed to ask what a memory may cost.
Two designs already answer half of that question each, and neither of them is shipped.**

The re-frame is correct and I had reached half of it independently: **#13246's fourth option is not an
option to weigh, it is a designed unit with an owner.** But the measurement says it is **half** the
answer, not the whole one, and the reason is structural rather than a coverage gap.

| | finding | status |
|---|---|---|
| **F1** | #13246's mechanism is **alive and re-derives today**. On the raw input query, `#6375` is admitted at rank 2, takes **77.2%** of admitted candidate bytes and **72.1%** of the block, and **85,317 B (60%) of above-floor material is stranded behind it**. #13246 said 76.6% and #13245 said 72%. | measured, §4.2 |
| **F2** | The defect is **worse than when it was filed**, on the axis that matters. The quality inversion — lowest admitted below highest byte-cut — was **0.0078** in #13093. It is **0.0218** on the raw input and **0.0279** on the production six-query shape today. | measured, §4.2–4.3 |
| **F3** | **The form rule (#13238 Unit 3) cannot close it, and not because of coverage.** #13238 §8.1 rules *"Why a ratio and not a size"*. **A ratio is a multiplier on an unbounded quantity.** At the ≥8 KB stratum median 0.298, `#6375` in substance form is still **22.8% of the block**; at the only in-stratum ratio ever actually measured (0.4625) it is **35.4%**. And a large node whose ratio is *bad* — #13238 names #11084 at 0.983 — is rendered as **content, at full size, by the rule's own design**. | derived from #13238's own numbers, §6.2 |
| **F4** | Projecting full Unit 2 coverage at #13238's own best stratum ratio onto the measured candidate set, the six-query block's inversion is **0.0272** against today's **0.0279**, and it is **0.0272 at the pessimistic ratio too** — the conclusion is insensitive to which ratio is used. The form rule fixes the **volume** (2.84× oversubscribed → 1.39–1.73×) and leaves the **boundary** where it found it. | projection over measured sizes, §6.3 |
| **F5** | **Today, Unit 3 alone changes the six-query block by nothing at all** — 1 of 19 above-floor candidates carries a substance, and it is cut either way. On the raw input it makes the block **worse** (8 admitted → 7). | measured, §6.1 |
| **F6** | A per-candidate size cap at **budget/5 = 12,000 B** removes the inversion on both query shapes, raises the raw-input block from **8 admitted to 14**, admits `#1487`, and drops the largest single share of the block from **77.2% to 16.4%**. | measured, §7.3 |

**The decision, in one line.** Ship the payload change first because it is the larger lever and it is
already designed; ship a per-candidate size cap as well, because **only the cap is a bound**, and the two
compose into one rule rather than competing — *the cap is the rule that decides when the loop may afford
details, which is the question Toni asked.*

**What I am not doing.** Not re-specifying #13238 Units 2 and 3 — they have an owner and a document.
Not inventing the cap: its ordering slot, its exclusion-not-truncation semantics and even its value
(12,000 B) are already specified in `retrieval-admission-and-the-empty-outcome.md` §5/§8.2/§9, demoted but
held alive in its slot by the aperture design §16.3 and by #14112 §6.1 ground 4. **This document's
contribution is the ruling on which answers #13246, the measurement that says the form rule alone does
not, and the composition contract between the two.**

---

## 1. Problem Statement

`admit` (`internal/loop/assemble.go`) walks candidates in rank order and takes each one whole while the
byte budget lasts. It has no opinion on what a single candidate costs. #13246 names the consequence and
this document must decide whether it needs a mechanism of its own.

The underlying need, stated as Toni stated it: *"why is there a 43kb document in there? — unless the loop
specifically wants details on that one it should only get pushed substance of memories."* That sentence
contains both halves of the answer — a **payload** question (*what form is pushed*) and a **budget**
question (*when are details affordable*) — and this project currently answers neither at the point where
the bytes are spent.

**Success is not "the block got smaller".** Success is: *no single memory can silently decide what the
rest of the block contains, and when one is too expensive to carry whole, the run says so.*

---

## 2. Scope & Non-Scope

**In scope**

- Ruling on which of #13246's four named shapes answers it, with the rejected ones named and why.
- The admission-side rule that bounds one candidate's share of the block, and its ordering.
- How that rule composes with the shipped relevance floor and with the unshipped form rule.
- The disclosure a capped candidate must carry, and what that disclosure is worth downstream.
- The falsifier that decides the rule and the constant it needs.

**Out of scope, explicitly**

| Excluded | Why |
|---|---|
| **#13238 Units 2 and 3 — generation and the form rule** | Designed, owned, and the primary remedy. This document sequences them and states what they do **not** reach; it re-specifies nothing. |
| **The relevance floor's value** | #14112 owns it. §0 is a finding filed against it, not a change to it. |
| **The self-produced exclusion** | Constraint. Untouched in both its sites. |
| **Truncation, `min(size, C)`, bounded-prefix rendering** | Rejected three times already — the demoted design's *"exclude, do not truncate — a truncated document silently lies about being complete"*, #11308's option-2 warning, and #12955 §10. This design excludes. |
| **Re-ordering candidates** | §5.1 rejects it. Rank order into `admit` is preserved. |
| **A byte-denominated aperture in `Retrieve`** | Rejected by the aperture design §5.2 on three grounds, one of which is that `Retrieve` would have to know `AssemblyByteBudget`. This design puts nothing byte-shaped in `Retrieve`. |
| **Raising `AssemblyByteBudget`** | Withdrawn under #11364 and refused again in `what-a-successful-run-withheld.md` §5.1. Not proposed. |
| **The unbudgeted render framing** | Known residual (178–2,444 B, mean 1,384 B). §10 records that it erodes this design's invariant at low budgets; it does not fix it. |

---

## 3. Assumptions & Constraints

| # | Constraint | Source |
|---|---|---|
| C1 | **`Assemble` is a pure function** — no I/O, no clock, no randomness. | Task constraint; `assemble.go:18`. |
| C2 | **Admission order is settled**: self-produced → floor → byte budget, the floor charging **nothing** to `cumulative`. | #14112 §6.1 D1, §9 step 3, §11 C3, guard F-4. |
| C3 | **A size rule's slot in that order is already specified** as `self-produced → floor → size cap → byte budget`, on the stated grounds *"no point measuring the size of noise"* and *"size cap before greedy byte admission, or one oversized document sets the boundary, as `#6375` did"*. | `retrieval-admission-and-the-empty-outcome.md` §9, demoted; kept alive in slot by aperture §16.3 and #14112 §6.1 ground 4. |
| C4 | **Every cut row keeps its own score and exactly one reason, and charges zero bytes.** This is why the floor lives in `admit` at all, and #14112's refusal of a config dial rests on it. | #14112 §6.1 ground 1, §6.4 D4, C2, C3. |
| C5 | **The rule must be over node *properties* — size, ratio, presence — never provenance.** | #13238 R5/S2; falsifier *"any branch reading `SelfProduced` or a node type"*. |
| C6 | **Exclude, never truncate.** | Demoted §8.2; #11308; #12955 §10. |
| C7 | **`admit` has two call sites** — the initial block at `AssemblyByteBudget` (60,000) and `dispatchRecall` at `SupplementaryByteBudget` (20,000). Any rule added to `admit` covers both automatically. | `turn.go:150`, `turn.go:430`. |
| C8 | **The form decision, when it ships, is made at admission in rank order over the chosen form.** | #13238 §7.2 steps 2–3. |
| C9 | **Render framing is unbudgeted** — 178–2,444 B, mean 1,384 B, and it does not shrink with the budget dial. | `m1-skeleton-loop.md` §8.4. |
| C10 | **Substance coverage is ~0.3% of the graph** (1 of 500 sampled 2026-09-08), plus 17 run records backfilled 2026-09-16 by the deterministic template. | #13238 §4.1; #14075 / PR #89. |

**Assumption I could not discharge.** #13238's ≥8 KB stratum median ratio **0.298** rests on **n = 6**.
The only ratio I could read live from a real candidate is `#13101` at **12,027 B → 5,562 B = 0.4625** —
which is #13238's own single real measurement (it recorded 11,961 B → 5,527 B = 0.462; the node has since
grown by 66 B and the ratio re-derives). **The one in-stratum measurement is 1.55× worse than the median
the projections use.** §6 carries both and marks which is which.

---

## 4. The mechanism, re-derived — measurement before design

### 4.1 The instrument

I ran the product's own `Retrieve` and `Assemble` against the live graph, with the real anchor `#10422`,
`CandidateLimit = 20`, `RecallScopeReserve = 3`, `AssemblyByteBudget = 60,000`, `RelevanceFloor = 0.63`,
no update window. **No model call occurs inside that path** — retrieval and admission are deterministic
given the graph, so this is a measurement, not a sample.

**The one substitution, named.** The six-query set is produced by a model call *upstream* of retrieval. I
made that call once, through the product's own `DerivationPrompt`, against `qwen3-coder:30b` on
`gangolf:11434`, temperature 0, and then **pinned the resulting five queries** for both probe runs. So
the query set is a sample of one derivation; everything downstream of it is deterministic. A second
derivation would produce a different candidate set. **Unbracketed:** I did not repeat the derivation, per
the standing request for restraint on the endpoint.

Two arms were run:

- **Arm ONE** — the raw input query alone. This is the world #13093 measured, and it is the control that
  says whether #13246's mechanism is still the same mechanism.
- **Arm SIX** — raw input plus the five derived queries. This is what production does today.

Anchor `#10422` is 3,408 B in both, so `remaining` is **56,592 B**.

### 4.2 Arm ONE — #13246's case, alive at the current graph state

| rank | id | similarity | size B | outcome |
|---:|---:|---:|---:|:--|
| 1 | 1804 | 0.6886747 | 1,111 | admitted |
| **2** | **6375** | **0.6586800** | **43,273** | **admitted** |
| 3 | 14122 | 0.6579131 | 4,915 | admitted |
| 4 | 13101 | 0.6564853 | 12,027 | byte budget |
| 5 | 6883 | 0.6551829 | 563 | admitted |
| 6 | 1836 | 0.6475001 | 2,259 | admitted |
| 7 | 6371 | 0.6456038 | 28,360 | byte budget |
| 8 | 11387 | 0.6433074 | 20,119 | byte budget |
| 9 | 2284 | 0.6432499 | 1,560 | admitted |
| 10 | 7388 | 0.6398459 | 5,166 | byte budget |
| 11 | 10435 | 0.6378142 | 6,173 | byte budget |
| 12 | 5576 | 0.6354123 | 430 | admitted |
| 13 | 8344 | 0.6346976 | 1,974 | admitted |
| 14 | 6882 | 0.6339107 | 661 | byte budget |
| 15 | 13189 | 0.6330465 | 4,282 | byte budget |
| 16 | 2313 | 0.6322134 | 2,961 | byte budget |
| 17 | 15 | 0.6308967 | 2,701 | byte budget |
| **18** | **1487** | **0.6301475** | **2,867** | **byte budget** |
| 19 | 10442 | 0.6276197 | 1,843 | below floor |
| 20 | 10437 | 0.6254291 | 80,470 | below floor |

**8 admitted of 20. 56,085 B of candidate content, 507 B spare.**

| claim | #13246 / #13093 / #13245, 2026-09-07/08 | here, 2026-09-17 |
|---|---|---|
| `#6375`'s share of admitted candidate bytes | 76.6% | **77.2%** (43,273 / 56,085) |
| `#6375`'s share of the 60,000-byte block | 72% | **72.1%** |
| material stranded behind it | 81% of eligible (64,078 B behind 12,304 B) | **60% of above-floor** — 18 rows totalling 141,402 B against a 56,592 B budget; 85,317 B never reaches the model |
| lowest admitted vs highest byte-cut | 0.6378 vs 0.6456 → **inversion 0.0078** | 0.6346976 (`#8344`) vs 0.6564853 (`#13101`) → **inversion 0.0218** |

**Every figure #13246 rests on re-derives. The quality inversion it names is 2.8× worse than when it was
filed.** That is the standing behaviour, not an artefact of one archive, and now not of one graph state
either.

Note row 20: **`#10437` is 80,470 B — larger than the entire block budget** — and it reached rank 20. It
was removed by the *relevance* floor, coincidentally. Nothing in `admit` today would otherwise have had an
opinion about a candidate that cannot fit under any circumstance.

### 4.3 Arm SIX — what production actually does, and why it is not better

| rank | id | similarity | size B | outcome |
|---:|---:|---:|---:|:--|
| 1 | 1804 | 0.6902502 | 1,111 | admitted |
| 2 | 7388 | 0.6544186 | 5,166 | admitted |
| 3 | 6041 | 0.6561438 | 2,993 | admitted |
| 4 | 10435 | 0.6395997 | 6,173 | admitted |
| 5 | 2284 | 0.6432499 | 1,560 | admitted |
| 6 | 7390 | 0.6380312 | 6,094 | admitted |
| 7 | 6934 | 0.6307371 | 13,369 | admitted |
| 8 | 1376 | 0.6542189 | 946 | admitted |
| 9 | 1836 | 0.6475001 | 2,259 | admitted |
| 10 | 10437 | 0.6254291 | 80,470 | below floor |
| 11 | 227 | 0.6421446 | 2,974 | admitted |
| **12** | **6375** | **0.6586800** | **43,273** | **byte budget** |
| 13 | 2313 | 0.6526580 | 2,961 | admitted |
| 14 | 6371 | 0.6529632 | 28,360 | byte budget |
| 15 | 6883 | 0.6551829 | 563 | admitted |
| 16 | 15 | 0.6308967 | 2,701 | admitted |
| 17 | 7389 | 0.6342202 | 3,173 | admitted |
| **18** | **14122** | **0.6579131** | **4,915** | **byte budget** |
| 19 | 13101 | 0.6564853 | 12,027 | byte budget |
| 20 | 11387 | 0.6433074 | 20,119 | byte budget |

**14 admitted of 20. 52,043 B, 4,549 B spare.**

Three things here that are new since #13246 and that a design must not get wrong:

1. **The aperture fix has shipped and it worked.** Zero of the twenty candidates are self-produced —
   `fuse` excludes them before they consume a slot. #13093's *"half the candidate window is the harness's
   own output"* and #13238's live claim that run records are *"thirteen of twenty candidate slots today"*
   are both **stale**; filed separately, not corrected here.
2. **`#6375` is no longer admitted on this arm** — eleven smaller documents ahead of it exhaust the
   budget first, and the 43 KB document is itself cut. **This is not the defect being fixed; it is the
   same defect with the dice landing differently.** Whether the largest document eats the block or is
   eaten by it is decided by where fusion happens to place it, which is exactly the property #13246
   objects to.
3. **The inversion is worse here than on arm ONE: 0.0279.** `#14122` — **4,915 B, similarity 0.6579, the
   second-highest score in the window** — is cut for byte budget, while `#6934` — **13,369 B, similarity
   0.6307, the lowest above the floor** — is admitted. A 4.9 KB document scoring at the top of the band
   loses its place to a 13.4 KB document scoring at the bottom of it, purely because of rank position.

**This is the sharpest single statement of the harm, and it needs no similarity signal to be damning.**
#13246 is right that ranking carries almost no discriminating signal across a 0.061-wide band — and that
is an argument against leaning on ranking, not an argument for ignoring size. When similarity cannot
discriminate, **bytes are the only thing left that can**, and today they are spent by accident of order.

### 4.4 Oversubscription, stated once

| arm | above-floor candidates | their total content | against `remaining` |
|---|---:|---:|---:|
| ONE | 18 | 141,402 B | **2.50×** |
| SIX | 19 | 160,737 B | **2.84×** |

Any design here is an allocation design. The budget cannot be made to fit; the only question is who
decides, on what basis, and whether the decision is disclosed.

---

## 5. The four shapes #13246 names, weighed

### 5.1 Similarity-per-byte ordering — **rejected**

Admit by value density rather than rank. Rejected on three grounds, the first of which is arithmetic.

**It is a size sort wearing a similarity costume.** Across arm ONE's above-floor band the similarity
numerator spans 0.6301–0.6587, a ratio of **1.045×**. The size denominator spans 430–43,273 B, a ratio of
**101×**. Density for `#6883` (0.6552 / 563) is **1.16 × 10⁻³**; for `#6375` (0.6587 / 43,273) it is
**1.52 × 10⁻⁵** — a **76× spread** produced almost entirely by the denominator. **The ordering is
determined by size to better than three significant figures.** #13246's own caution — *"a fix that leans
harder on the ranking is leaning on nothing"* — applies with full force: dividing a near-constant by a
wildly varying quantity does not create signal, it launders a size sort into one.

**It is the same arbitrariness with the sign flipped.** Under density the largest document is always last
and is therefore always the one cut. That is not a quality boundary either; it is a different accident.

**It re-orders, and re-ordering is a new selection rule.** #12955 §6.2 defers prefer-smallest-first to a
sweep flag precisely because it displaces; #13238 §8.2 records that abandoning `admitted ⊇ today's
admitted` costs a proof and buys a measurement. A full re-sort abandons it for every row at once, with no
compensating mechanism. And the invariant everything downstream leans on — that `admit` walks in rank
order — is stated in four documents.

### 5.2 Reserving budget for the tail — **rejected**

Hold back N bytes so last-ranked eligible documents are not structurally unreachable.

**It reserves for an arbitrary half.** #13246's decisive observation is that the eligible band is 0.061
wide with a mean neighbour gap of 0.0068 — *the tail is not meaningfully worse material than the head*. A
rule that privileges the tail is as unjustified as the one that privileges the head; it just fails
differently.

**It does not bound anything.** A reserve splits the budget into two greedy pools. One document larger
than the head pool still consumes all of it, and the arm SIX table shows the same inversion re-appearing
inside whichever pool is doing the work.

**It is a second tunable constant with no principle behind it**, which is the specific thing #12955 §6.4
refuses: *"every constant introduced here is a constant somebody later fits."* A cap has a statable
invariant (§7.2); a reserve has only a number.

### 5.3 Fixing it upstream in the payload — **adopted as the primary remedy, and it is not mine**

This is #13238 Units 2 and 3, and the re-frame is right that it is not a possibility to weigh: Unit 1 has
shipped, `cmd/condense` exists, and the form rule is specified. §6 is the measurement of what it reaches.
**Adopted, sequenced first, and not re-specified here.**

**But it does not close #13246**, for a reason that is structural rather than a coverage gap, and §6.3
measures it. It is therefore the primary remedy and not the only one.

### 5.4 A per-candidate size cap — **adopted as the bound**

Adopted. §7 specifies it. **It is not a fifth shape and it is not new**: the demoted
`retrieval-admission-and-the-empty-outcome.md` specified it at §5 rule 5, §8.2 (12,000 B, 20% of the
budget, citing `#6375` by name and by number), and §9 (its ordering slot and the reason for it). The
aperture design §16.3 explicitly left it standing in that slot. This document argues it back off the
demotion pile on evidence the demoted design did not have, and adds the composition contract with the
form rule, which did not exist when it was written.

**No fifth shape is invented.** The recommendation is two of the four, sequenced, with a stated seam.

---

## 6. Does the form rule dissolve it? — measured, and the answer is no

### 6.1 With substance as it actually exists today: no effect, or a negative one

`admit` already records `SubstanceAvailable` and `SubstanceSize` on every disposition, so substance
coverage on a real candidate set is directly readable. Measured 2026-09-17:

| arm | above-floor candidates | carrying a substance |
|---|---:|---:|
| SIX | 19 | **1** (`#13101`, 5,562 B against 12,027 B) |
| ONE | 18 | **2** (`#13101`; `#13189`, 1,109 B against 4,282 B) |

**`#6375` carries no substance.** Neither does `#6371`, `#11387`, `#6934`, or `#10437`. The five
documents that between them make the block's arithmetic impossible are exactly the five the form rule has
nothing to say about yet.

Applying #13238's form rule to today's data, with its own ≥8 KB gate:

| arm | today | form rule, substance as it exists |
|---|---|---|
| SIX | 14 admitted, 52,043 B | **14 admitted, 52,043 B — byte-for-byte identical** |
| ONE | 8 admitted, 56,085 B | **7 admitted, 56,515 B — one row worse** |

The single row of substance in arm SIX belongs to a candidate that is cut either way. In arm ONE,
rendering `#13101` at 5,562 B instead of 12,027 B frees space that greedy-by-rank immediately hands to a
larger document, and the block ends with **fewer** memories in it. **That is not an argument against the
form rule — it is the F-2 regression #13238 §8.2 knowingly accepted when it withdrew `admitted ⊇ today's
admitted`, showing up on live data at the first opportunity. It is also, precisely, the size-blindness
this document is about: the form rule frees bytes and the existing allocator wastes them.**

**So: Unit 3 cannot be evaluated on today's graph, and Unit 2 is the blocker, exactly as the re-frame
says.** The minimum Unit 2 must reach, read off arm SIX, is the five above-floor nodes ≥ 8 KB:
`#6375` (43,273), `#6371` (28,360), `#11387` (20,119), `#6934` (13,369), `#13101` (12,027, already done).
That is the whole gate — #13238's G1 fires at ≥ 8 KB and nowhere else.

### 6.2 With full coverage: the ratio is not a bound

**#13238 §8.1 rules the form rule to be a ratio rule, deliberately and with its reason stated:** *"Why a
ratio and not a size. Size is a proxy; the ratio is the thing."* It names #11084 at 3,587 B with a ratio
of **0.983** and #11228 at 6,820 B at **0.971** — nodes where condensation did nothing — and observes
that a size rule mis-handles both.

That is correct about *fidelity economics* and it has an unavoidable consequence for *budget*:

> **A ratio multiplies an unbounded quantity. It compresses; it cannot bound.**

Three consequences, all of them following from #13238's own numbers:

1. **A well-condensing large node is still large.** `#6375` at the stratum median 0.298 → ≈12,895 B,
   **22.8% of the block**. At the only in-stratum ratio ever measured live (0.4625, `#13101`) → ≈20,014 B,
   **35.4% of the block**. #13238's own §4.3 projection of `#6375` at ≈12,793 B uses 0.298, which is
   1.55× more optimistic than its own single real data point.
2. **A badly-condensing large node is rendered whole, by design.** The form rule's ratio threshold sends
   a poor ratio to content. A 43 KB node at #11084's 0.983 is admitted at 43 KB after Unit 3 ships,
   exactly as today. **The form rule has no opinion on that case by construction**, and nothing in Unit 2
   can create one, because Unit 2's output *is* the ratio.
3. **A node larger than the budget stays unreachable unless its substance happens to fit.** #12955 §3.3
   measured 4 of 460 cut candidates exceeding the whole budget, max 195,448 B;
   `what-a-successful-run-withheld.md` §5.1 found the three oversize corpus nodes all carrying
   `substance: null`. `#10437` at 80,470 B sat at rank 10 of arm SIX today.

### 6.3 The measurement that settles it

Projecting full Unit 2 coverage onto arm SIX — every above-floor node ≥ 8 KB rendered at a condensed
size, everything below the gate unchanged:

| scenario | admitted | bytes | **inversion** |
|---|---:|---:|---:|
| today | 14 | 52,043 | **0.0279** |
| form rule at stratum median **0.298** | 15 | 55,553 | **0.0272** |
| form rule at live in-stratum ratio **0.4625** | 12 | 56,036 | **0.0272** |
| **size cap alone, no payload change** | 14 | 43,589 | **0.0000** |
| form rule at 0.298 **+ cap** | 16 | 51,157 | **0.0126** |
| form rule at 0.4625 **+ cap** | 16 | 55,334 | **0.0000** |

And the oversubscription:

| | above-floor content | × `remaining` |
|---|---:|---:|
| today | 160,737 B | **2.84×** |
| form rule at 0.298 | 78,498 B | **1.39×** |
| form rule at 0.4625 | 97,769 B | **1.73×** |

**Read the mechanism, not the digits** — the two right-hand columns are projections built on #13238's
n = 6 stratum median and on a single live ratio, and they are marked as such throughout. The mechanism
they show is not sensitive to the ratio:

> **The form rule cuts the oversubscription roughly in half and leaves the quality inversion where it
> found it. The block is still 1.4–1.7× oversubscribed, admission must still choose, and it still chooses
> by rank against bytes.**

That is #13246's second harm surviving its first remedy intact. **The cap is what removes it, and the two
together are better than either.**

### 6.4 Answering the re-frame's three questions directly

1. **Does Unit 3 alone dissolve #13246?** **No.** On today's graph it changes arm SIX by nothing and makes
   arm ONE worse (§6.1). Under full coverage at its own best ratio it halves the volume harm and leaves
   the boundary harm at 0.0272 against today's 0.0279 (§6.3). The reason is structural: a ratio is not a
   bound (§6.2).
2. **What minimum of Unit 2 is needed for Unit 3 to bite?** The ≥ 8 KB stratum of what retrieval actually
   returns — five nodes on arm SIX, four on arm ONE. **And admission can name them itself**: §7.5.
3. **Is anything left over that needs size-aware admission?** **Yes, and it is the cheaper half.** The cap
   alone, with no payload change at all, removes the inversion on both arms today (§7.3). What it cannot
   do is *keep* the large document — it excludes where Unit 3 would condense. That asymmetry is the whole
   argument for shipping both, and it is why §7.4 makes the cap the **trigger** for the form rule rather
   than its competitor.

---

## 7. The design

### 7.1 Overview

```
                 candidates, in fusion rank order
                              │
                              ▼
   ┌──────────────────────────────────────────────────────┐
   │  admit  (pure; one pass; one reason per row)          │
   │                                                       │
   │   1. self-produced?        ── cut ──▶ "self-produced" │   (shipped)
   │   2. below the floor?      ── cut ──▶ "below floor"   │   (shipped, PR #90)
   │   3. choose the payload    ─────────▶ content|substance│  (#13238 Unit 3 — not shipped)
   │   4. payload over the cap? ── cut ──▶ "oversized"     │   ◀── THIS DESIGN
   │   5. fits the remainder?   ── admit ─▶ included        │   (shipped)
   │      else                  ── cut ──▶ "byte budget"   │
   │                                                       │
   │   steps 1–4 charge nothing to the running byte total   │
   └──────────────────────────────────────────────────────┘
```

**One rule, one new cut reason, no new tunable other than the occupancy number, no I/O, no re-ordering.**

### 7.2 The rule, and the invariant that gives it meaning

> **No candidate may be admitted whose rendered payload exceeds the block budget divided by the minimum
> block occupancy.**

The cap is **derived from whichever budget `admit` is given**, never stated as a second absolute constant.
That matters for three reasons: it does not create a duplicate of a ceiling that design #13522 exists to
stop being recopied (the aperture design found six sites, one origin and five restatements); it scales
automatically to the supplementary path (C7); and it survives the budget dial that #12955's own falsifier
demands, which an absolute byte constant does not.

**The invariant, which is what makes the number a decision rather than a fitting:**

> With the cap set at `budget / k`, **the block admits at least `k` candidates whenever at least `k`
> above-floor candidates exist.**

The reasoning: admission stops when the next payload does not fit, so the accumulated total then exceeds
`budget − budget/k`. Every admitted payload is at most `budget/k`. Therefore the number admitted exceeds
`k − 1`, hence is at least `k`. **The cap is not "a fraction that seemed right"; it is the statement "a
block always carries at least k memories", expressed in bytes.**

Two honest qualifications on that invariant, both stated rather than buried:

- **Render framing is unbudgeted (C9).** At mean 1,384 B of framing the guarantee is approximate at
  60,000 and degrades as the budget falls; at a 4,000-byte dial setting framing is roughly 78% of the
  budget and the invariant means nothing. It is a property of the current budget, not a theorem.
- **The anchor is charged before `admit` runs.** `remaining` is `budget − anchorSize`, so the cap as
  specified is a fraction of the *block* budget while the greedy fill spends the *remaining* budget. This
  is deliberate: the cap should be a stable property of a document, not a function of how large today's
  anchor happens to be. It makes the effective guarantee slightly weaker than `k` and I state it rather
  than defining the cap against `remaining` to make the arithmetic tidy.

### 7.3 The number

| cap | arm SIX admitted / inversion | arm ONE admitted / inversion |
|---:|:--|:--|
| none (today) | 14 / **0.0279** | 8 / **0.0218** |
| 20,000 (`budget/3`) | 14 / 0.0272 | 15 / 0.0000 |
| 15,000 (`budget/4`) | 14 / 0.0272 | 15 / 0.0000 |
| **12,000 (`budget/5`)** | **14 / 0.0000** | **14 / 0.0000** |
| 10,000 (`budget/6`) | 14 / 0.0000 | 14 / 0.0000 |
| 8,192 | 14 / 0.0000 | 14 / 0.0000 |
| 7,500 (`budget/8`) | 14 / 0.0000 | 14 / 0.0000 |
| 6,000 (`budget/10`) | **12** / 0.0000 | 13 / 0.0000 |

**Recommendation: `k = 5`, cap = 12,000 B at the 60,000-byte budget.**

Four reasons, in order of weight:

1. **It is the loosest value that works on both arms.** The plateau where the inversion vanishes on the
   binding arm is `[7,500, 12,000]`; 12,000 is its upper edge. **The cap's cost is exclusion, and
   exclusion is loss, so the weakest rule that achieves the property is the right one.**
2. **It is the number the demoted design already specified**, at §8.2, as 20% of the budget, arguing from
   `#6375` and #13093 §5 — the same document, a different method, the same answer. A constant that two
   independent derivations land on is meaningfully less fitted than one.
3. **The plateau is wide because this corpus has a size gap**, not because the value is robust. Arm SIX's
   above-floor sizes jump from 6,173 B straight to 12,027 B; every cap in `[7,500, 12,000]` is identical
   *on this data by accident*. **This is why §9's falsifier, not this table, sets the shipping value.**
4. **At 12,000 the cap sits above #13238's 8 KB condensation gate**, so every document the cap refuses is
   a document the form rule can do something about. §10 records where that stops being true.

### 7.4 How the cap and the form rule compose — the seam

This is the part neither existing document can contain, because it is about both.

> **The cap is evaluated against the payload the block is about to render, after the form rule has chosen
> it.**

Three consequences, and the third is the answer to Toni's question:

| case | today | with the cap alone | with the cap **and** the form rule |
|---|---|---|---|
| large node, good ratio, substance present | admitted whole, eats the block | **cut, `oversized`** | **admitted in substance form** |
| large node, no substance yet | admitted whole, eats the block | **cut, `oversized`** | cut, `oversized` — and named as work for Unit 2 (§7.5) |
| large node, bad ratio, substance present | admitted whole | cut, `oversized` | cut, `oversized` — the ratio rule keeps it as content, the cap refuses the content |

**The cap is therefore not a competitor to the payload change; it is its trigger.** Ordering the two
rules this way means:

- **Unit 3 turns "too expensive to carry" into "affordable in condensed form"** — a node over the cap in
  content but under it in substance is *admitted*, where the cap alone would have lost it. That is
  strictly better than either rule alone, and it is measured: §6.3's bottom two rows admit **16** where
  the cap alone admits 14 and the form rule alone admits 12–15.
- **The cap is the rule that decides when the loop may afford details.** Toni's sentence — *"unless the
  loop specifically wants details on that one it should only get pushed substance"* — is a budget
  statement, not a fidelity statement. The form rule cannot express it, because it decides on ratio and
  is indifferent to what the block can afford. The cap expresses exactly it: **details are a budgeted
  privilege, and the budget is one-fifth of the block.**

**Sequencing consequence.** Neither rule needs the other to ship. If the cap ships first, the `oversized`
reason simply exists and every capped row is a cut. If Unit 3 ships first, the cap slots in behind its
form choice with no change to it. **If the cap ships first, one thing must be true of it from day one:
the cap must be specified against "the payload", not against "the content", even while content is the
only payload there is.** Otherwise Unit 3 lands on a rule that silently means the wrong thing.

### 7.5 Disclosure, and what it is worth

C4 requires every cut row to keep its score and exactly one reason and to charge zero bytes. The cap
obeys all three: `oversized` is one reason, assigned only to rows that passed self-produced and the
floor, charging nothing.

It should carry, alongside the fields already on a disposition, **the cap in force** — because a stored
record must answer *"what would a different cap have done?"* from history, which is the property #14112
§6.4 used to refuse a configuration dial, and it is exactly as load-bearing here.

**And then the disclosure is worth more than an audit trail.** #13238's Unit 2 needs to know which nodes
to condense, and its G2 pressure gate is described in that document as *"Toni's 'a node it wants to
push'"*. **A row cut as `oversized` with no substance present is precisely that node, named, in every run
record, ranked by how often it recurs.** `#6375` appears in all five run records #13245 examined. So:

> **Admission's `oversized` disclosure is the work queue for condensation.**

I am not specifying that wiring — it belongs to #13238 Unit 2 — but the cap should be designed knowing
its cut reason is an input to another mechanism, and the run record is the seam.

### 7.6 What it does *not* do

**The cap does not make greedy-by-rank a quality ordering.** On both arms the measured inversion goes to
zero, and it is honest to say *why*: at `budget/5` on this data, **nothing is cut for byte budget at all**
— every above-floor document under the cap fits. The inversion vanishes because the byte budget stops
binding, not because the boundary became principled. On an input returning twenty documents of 6 KB each
the budget binds again and the inversion returns, undiminished.

**What the cap bounds is one specific and measured source of that inversion**: a single outsized document
setting the boundary for everything behind it. That is the source #13093, #13245 and #13246 all
independently identified, and it is the one the evidence supports fixing. It is not a claim to have made
admission smart.

---

## 8. Components & Responsibilities

| component | owns | does **not** own |
|---|---|---|
| **`Retrieve` / `fuse`** | eligibility, fusion, rank order | anything byte-shaped. It does not learn the budget, the anchor size, or the cap. The aperture design §5.2's rejection stands. |
| **`admit`** | **admission** — self-produced, floor, **the cap**, the byte budget, and one reason per row | ranking; generation; what the cap's number should be at any budget other than the one it is handed |
| **the form rule** (#13238, unshipped) | which payload is rendered, on ratio | affordability. It is indifferent to the budget by design. |
| **`cmd/condense` / Unit 2** (#13238) | producing substance | admission. It consumes `oversized` disclosures; it does not decide them. |
| **the run record** | disclosing every row's score, size, reason and the cap in force | — |

---

## 9. The falsifier

**`admitted ⊇ today's admitted` is deliberately violated by this design.** #13238 §8.2 already withdrew
that invariant and replaced it with F-2 (*no row may go from admitted to not-admitted*). **F-2 cannot
bind the cap either**, because making certain admitted rows not-admitted is the cap's entire purpose. So
the cap owes a replacement proof, and it must be at the level of *what the block was for*:

> **F-CAP (the one that rejects the recommendation).** Sweep `internal/eval/corpus.json`'s 25 rows with
> the cap on and off, at several budgets. **If any required document goes from admitted to
> not-admitted, the cap's value is wrong; if that holds at every value of `k` that removes the
> inversion, the mechanism is wrong and does not ship.** The instrument already exists — `eval.scoreOne`
> matches required node ids against dispositions — and needs no change.

Supporting guards:

| # | guard | pins |
|---|---|---|
| **G1** | a row cut as `oversized` charges nothing to the running total — a large over-cap row ahead of a small one that only fits if the large one charged nothing | C4; the mutant that increments before cutting passes everything else |
| **G2** | a row that is **both** below the floor and over the cap reports `below relevance floor` | C3's ordering, and C4's exactly-one-reason |
| **G3** | a row over the cap is present in the record with its own similarity, size and reason — not dropped | C4 |
| **G4** | with no candidate over the cap, output is byte-identical to today | the rule is inert where it should be |
| **G5** | the cap is derived from the budget `admit` is given — the supplementary path's cap differs from the initial path's by the ratio of their budgets | that no second copy of a ceiling is introduced |

**The discriminating fixture, not a passing test:** mutate the cap comparison to its likeliest wrong
neighbour — evaluating the cap against `remaining` instead of the block budget, or placing it before the
floor — and confirm G2 and G5 go red.

---

## 10. Risks

| # | risk | mitigation |
|---|---|---|
| **R1** | **The cap loses a large document that was genuinely the answer.** Exclusion is loss, and `#6375`-shaped nodes are cut outright until Unit 3 ships. | F-CAP measures exactly this on the corpus's labelled required documents. And §7.4: after Unit 3 the case turns from a loss into a condensed admission. **This risk is the argument for sequencing the payload change first, and it is why §12 does.** |
| **R2** | **Below a 49,152-byte budget, `budget/5` falls under #13238's 8 KB condensation gate** and the cap begins refusing documents the form rule will never condense, because G1 will not fill them. The seam in §7.4 silently stops working. | Stated as an open question (§11 Q3), not silently mitigated. #11364 says the budget *will* fall, so this is a scheduled collision, not a hypothetical. |
| **R3** | **The supplementary path gets a 4,000-byte cap** (`SupplementaryByteBudget / 5`), which is tighter than most documents. I have not measured `dispatchRecall`'s candidate sizes. | G5 pins the derivation; F-CAP must be run on that path too before the cap is default there. Q4. |
| **R4** | **`k` is fitted to two probes.** The plateau is wide because this corpus has a size gap between 6,173 B and 12,027 B, not because 12,000 is robust. | §7.3 reason 3 says so; F-CAP on 25 corpus rows sets the shipping value. The convergence with the demoted design's independently-derived 12,000 is corroboration, not proof. |
| **R5** | **The projections in §6.3 rest on n = 6.** The one live in-stratum ratio is 1.55× worse than the median used. | Both are carried side by side and the conclusion is shown to be insensitive to which is used. |
| **R6** | **The derived query set is a sample of one.** A different derivation produces a different arm SIX. | Stated in §4.1 as unbracketed. Arm ONE is deterministic and carries the load-bearing re-derivation of #13246. |
| **R7** | **The cap makes the byte budget non-binding on this corpus**, so the inversion reads as zero for a reason that will not generalise. | §7.6 says so explicitly rather than claiming the boundary was fixed. |

---

## 11. Open Questions — the ones that need Toni, not a measurement

**Q1. Is `#1487` sitting 0.0001475 above the relevance floor acceptable?** (§0.) A measurement cannot
answer it: the value is correct, the corpus that chose 0.63 is legitimate, and the collision is with a
different document on a different query. The question is whether a floor whose cut line passes through the
yardstick's key document, on a live unversioned graph, is a floor you want standing. **Urgent
independently of this design.**

**Q2. What is the minimum number of memories a block must carry?** That is what `k` means (§7.2). I
recommend 5 and the evidence supports anything in `[5, 8]`, but *"a block always carries at least five
memories"* is a product commitment, not a measurement.

**Q3. When the budget falls below ~49,000 bytes, does the occupancy guarantee win or does the cap floor
at the 8 KB condensation gate?** (R2.) Holding the guarantee means cutting documents nothing can condense;
flooring the cap means one document may take an ever-growing share as the budget shrinks — which is the
original defect returning at exactly the budget #11364 says is the product.

**Q4. Should the cap apply to the supplementary recall path at all?** (R3.) It does so automatically by
being derived. The alternative is to scope it to the initial block, which means two rules where there
could be one.

**Q5. Until Unit 2 runs, a capped document is *cut*, not condensed.** Toni's sentence implies he would
rather it were pushed in substance form. There is no substance to push on 18 of 19 candidates today. Is
cutting acceptable as an interim — it strictly improves the boundary now — or should the cap wait for
Unit 2, accepting the current behaviour for longer?

---

## 12. Implementation Guidance — ordered

**The ordering is the recommendation, not just a work plan.** The payload change is the larger lever and
the one that loses nothing; the cap is the bound and the one that can regress. Sequencing the payload
change first means the cap lands in a world where its cuts are mostly recoverable as condensed
admissions.

| # | unit | what it is | gate to the next |
|---|---|---|---|
| **0** | **The floor margin** | §0, filed as **#14211** against #14112 / the floor's value. Not part of any implementation branch; needs Toni (Q1). | — |
| **1** | **#13238 Unit 2 — generation** | Not this document's. The minimum target set is the ≥ 8 KB stratum of what retrieval actually returns; §6.1 names today's five. | Substance coverage on a real candidate set above 1 in 19. |
| **2** | **#13238 Unit 3 — the form rule** | Not this document's. | Its own F-2 and F-3. |
| **3** | **The cap — the rule** | One predicate in `admit`, one cut reason, one derived value, **specified against the rendered payload** even if content is the only payload at the time (§7.4). The cap in force recorded on the disposition (§7.5). No change to `Retrieve`, `fuse`, ranking, or the floor. | G1–G5 green, including the discriminating mutants. |
| **4** | **The cap — the constant** | Run F-CAP over the 25-row corpus at several budgets and several `k`, both paths. **Set `k` from that curve, not from §7.3's table.** | F-CAP green at the chosen `k`. |
| **5** | **The seam** | Wire `oversized` + substance-absent into Unit 2's target selection (§7.5). Belongs to #13238; named here so it is not lost. | — |

Units 3 and 4 are one PR; units 1, 2 and 5 are not this document's to schedule. **Unit 0 is already filed (#14211) and does not wait for any of it.**

---

## 13. What a user gets

Not *"admission becomes size-aware"*. Concretely, on the run whose block is currently three-quarters one
document — the yardstick input against subject `#10422`, measured today:

**Today.** The model is shown the anchor and **eight** documents. One of them — a German recruiter-portal
wireframe for an unrelated product, carrying about 615 lines of embedded HTML and CSS — is **43,273 of
the 56,085 candidate bytes: 77% of everything the run remembered.** Ten other documents that passed the
relevance floor are not in the prompt. Among them is `#1487`, the only document in the entire window that
is actually about building a website, cut for want of bytes that the wireframe is holding. The run record
says `byte budget exceeded` against each of them and does not say why the budget was gone.

**With the cap.** The model is shown the anchor and **fourteen** documents. The largest is 6,173 B —
**16% of the block instead of 77%**. `#1487` is in the prompt. The wireframe is not, and the run record
says `oversized`, with its size and the cap that refused it, so the reason the block looks the way it does
is legible instead of inferred.

**With the cap and the payload change together.** The model is shown **sixteen** documents, and the
wireframe is among them — in condensed form, at roughly a fifth of its length, next to the fifteen other
things the run remembered rather than instead of them.

**And the sentence that generalises past this one input:** today, whether your block is three-quarters one
document is decided by where fusion happened to place that document. After this, it is decided by a
number you can read.

---

## 14. Provenance

- **Measurements** (2026-09-17): arms ONE and SIX, produced by running `loop.Retrieve` and `loop.Assemble`
  against the live graph through `divoid.Client` with production constants. Similarity cross-checks via
  the graph's search route, `count=50`, which caps below the loop's 100 — bracketed in §0 and stated in
  §4.1. Node content sizes read from the content route. The cap sweep and the payload projections are
  arithmetic over those two tables.
- **The one model call**: `DerivationPrompt` for the yardstick input, `qwen3-coder:30b` on
  `gangolf:11434`, `/api/chat`, temperature 0, `num_predict` 4096. One call, not repeated.
- **Inherited and marked as such**: #13238's stratum ratios (0.298 at n = 6, 0.886 at n = 9), its coverage
  sample (2026-09-08), the demoted design's 12,000 B, #11365's cut-candidate distribution, #12984's audit.
- **Not measured here**: the 25-row corpus behaviour under the cap (that is F-CAP), the supplementary
  path's candidate sizes, the wire cost of anything.
- **Two live claims in other documents found stale** by arm SIX, filed as **task #14212** rather than
  corrected here: #13093's *"half the candidate window is the harness's own output"* and #13238's *"thirteen of
  twenty candidate slots today"* — the aperture fix has shipped and self-produced rows now consume **zero**
  candidate slots.
