# Architectural Document: The Relevance Floor Is an Admission Rule

**Task:** #14055, filed by #14054 §20 item 1.
**Baseline:** `main` = `aec8d3f` (worktree `design/the-aperture-has-no-relevance-floor`).
**Graph state:** every figure in this document was read on **2026-09-16, 14:3x–14:5x**, and the reading is
**bracketed** — §5.1. It is a different graph state from #14055's, which was read on 2026-09-16 earlier in
the day; the graph is live and unversioned and the two are **not** comparable (#11235 §9.1a).

---

## TL;DR

**The product harm is in the block, not in the aperture, and that decides where the floor goes.**

On *"Write a file called arithmetic.txt … 17 multiplied by 23 …"* — the input the product briefing §11
measured as worst — the twenty rows the loop puts in front of the model today score **0.6345 … 0.6071**,
and not one of them is about arithmetic. A floor at **0.63** leaves **two**. On the 25-row eval corpus the
same floor costs **nothing**: 13 of 25 required documents still reach the block, exactly as with no floor,
and no row's block loses a single slot below 14 of 20.

Four decisions, each with the alternative it rejects:

| # | decision | rejected |
|---|---|---|
| **D1** | The floor is an **admission** rule in `admit` (`internal/loop/assemble.go`), after the self-produced cut and before the byte budget | the graph-side **`minSimilarity`** parameter, and a selection-time cut in `fuse` |
| **D2** | **0.63**, from the sweep in §5.7 | **0.70** (the demoted design's number — measured here to cost 4 of the 13 documents retrieval delivers and to empty the block on 5 of 25 rows); and **0.638**, the zero-cost maximum, rejected as fitted to one control row's fourth decimal |
| **D3** | No new terminal reason and no re-query loop. The block **states** that nothing was admitted, mirroring the sentence the tool-result path already carries | the demoted `retrieval-admission-and-the-empty-outcome.md` Unit 2, whose premise — *a floor makes the empty outcome common* — is **measured false at 0.63** (0 of 25 rows go empty) |
| **D4** | A `const` in `internal/loop`, recorded in `Limits` | a sweep dial / config knob — unnecessary, because with D1 every stored record makes **every other floor value computable after the fact** |

**One prerequisite, and it is not optional.** `Candidate.Similarity` today carries the score from
**whichever query first surfaced the row** — an artifact of query order since derivation merged (PR #74).
On **48 %** of aperture rows that number is **below** the row's best, by a mean of **0.0294** and up to
**0.1023**. A floor read against it at 0.63 would cut **3 of the 13** documents retrieval delivers. Unit 1
fixes the field; Unit 2 adds the floor.

**And the ordering constraint holds, but not for the reason #14055 expected.** The floor is **necessary
and not sufficient** to permit relaxing the self-produced exclusion: on #14054 §4.5's own delta applied to
a figure re-derived here, a reshaped run record lands near **0.82** on its own input — above every floor
in the swept grid. §13.

---

## 1. Problem Statement

`GET /api/nodes?query=` ranks the whole graph and returns its best *n* **regardless of how bad the best
is**. The loop asks for 100 rows per query, fuses them to 20, and puts as many as fit into the block. When
the graph holds nothing relevant, that procedure still produces twenty rows and a full block.

The product consequence, in the terms §11 of the briefing set: *does the answer he gets get better, and
would he be able to tell?* Today, on an input the graph cannot answer, **no** and **no** — the run looks
identical to a successful one, and the model is handed tens of kilobytes of confident-looking material
that has nothing to do with the task.

**Success criteria.**

1. On an input the graph answers badly, the material reaching the model shrinks to what actually matched,
   or to nothing.
2. On an input the graph answers well, **nothing measurably changes**.
3. A reader of a stored run record can see which rows the floor cut and at what score, so the floor's
   value stays re-derivable from history.
4. No existing detector loses its premise.

---

## 2. Scope & Non-Scope

**In scope**

- Where the floor is enforced, what its value is, and how it is recorded.
- The correctness prerequisite in `Candidate.Similarity` that the floor exposes.
- What the block says when the floor admits nothing.
- The interaction with #13601's exhaustion detector, its premise **A5**, and its acceptance relation
  **AC-2**.

**Out of scope, explicitly**

- **Relaxing the self-produced exclusion.** §13 states why this design gates it and does not perform it.
  `internal/loop/assemble.go:53` and `internal/loop/retrieve.go:90` are untouched, and
  `TestFuseExcludesExactlyWhatAdmitWouldCutAsSelfProduced` must stay green **without being edited**.
- A terminal reason for clarification, a re-query loop, a catalogue — the demoted
  `retrieval-admission-and-the-empty-outcome.md` Units 2 and 3 stay demoted (§6.3).
- `RecallScopeReserve`'s width — #13606 owns it, and this design deliberately moves one constant, not two.
- The over-fetch factor and the two-phase fetch — #13601 §13.2 and §19 own them; this design does not end
  the bridge and does not extend it.
- `internal/divoid/write.go` and `scripts/compare.py`. This document specifies no change to either; they
  are in flight on #14054 Unit 1, and Unit 1's reader sweep (§17) must check the tree before touching
  `compare.py` and hand the edit over if it is dirty from another hand (P-50).
- Retrieval **latency** and wire **bytes**. Both are real and neither is measured on this system, and
  #13534 §3 bars optimising a quantity whose input is unmeasured. §6.1 records that this is the one place
  where the rejected alternative would have won, and why the win is not claimable.

---

## 3. Assumptions & Constraints

| # | statement | confidence |
|---|---|---|
| **A1** | DiVoid's listing route supports `minSimilarity`, AND-ed into the predicate before the single `Where()` call (#6115) | **Verified, §5.5** — at four values, id-for-id |
| **A2** | Retrieval is reproducible enough for a before/after to be attributable (#13592, #13534 §11) | **Re-confirmed, §5.1** — 27 of 27 arms byte-identical across a 12-minute bracket |
| **A3** | `admit` sees every candidate `Retrieve` returns, and writes a `Disposition` for each, including its similarity and cut reason (`internal/loop/assemble.go:31-67`) | **High**, read from the tree |
| **A4** | `fuseByReciprocalRank` keeps the **first-seen** candidate struct, so `Candidate.Similarity` is the score under whichever list first held the row (`internal/loop/retrieve.go:130-139`) | **High**, read from the tree and measured at §5.6 |
| **A5′** | #13601 §6's **A5** — *an unscoped recall at `count=100` always returns 100 rows, because the route applies no relevance floor* — is a premise this design must **not** break. D1 preserves it; the rejected graph-side placement would have falsified it by construction | **binding** |
| **A6** | The eval corpus's 25 rows are the well-answered arm; the sidecar pins 6 queries per row (raw input + 5), which is what `cmd/eval` itself issues (`internal/eval/derivations.go:94`) | **High** |
| **A7** | No model call is required to measure a floor. Recall similarity is a property of query text against the graph; the pinned sidecar supplies the query sets the harness uses. **No inference was run, locally or on `gangolf:11434`** | stated so the absence is not read as an omission |

---

## 4. The one figure that is inherited rather than re-derived

Stated first, and on its own, because #13987's rule is that re-deriving a measurement and inheriting its
meaning are different acts.

> **#14054 §4.5's *"replacing the JSON body with the prose account raises similarity to a repeat of the
> record's own input by +0.1272 to +0.1288 on #13599"* could not be re-derived at this graph state.** Its
> arm C was measured on probe node **#14051**, which returns **404** today — the probes were deleted as
> §4.4's method says. Nothing in the graph now carries that shape.

What **was** re-derived, and agrees exactly: **#13599 scores 0.6933 on a verbatim repeat of its own
input**, the same value to four decimal places as §4.5's arm A cell. So the instrument agrees on the half
that is still measurable, and §13 applies the inherited delta to that re-derived figure while saying, each
time, that the delta is inherited.

**Filed:** the re-take needs a durable probe or the post-backfill live nodes — §15 item 3.

---

## 5. The measurements this design rests on

### 5.1 The instrument, and the bracket

Every recall in this document was issued against the **production route the loop itself uses** —
`GET /api/nodes` with `query`, `count=100`, `fields=id,type,name,similarity` and, for the scoped arm,
`linkedto` — i.e. the parameter set at `internal/divoid/client.go:163`. The fusion, the reserve and the
exclusion were reproduced from `internal/loop/retrieve.go` line for line. Measuring through a different
client would have measured a different path.

**The bracket (#11235 §9.1a).** Reading **A** (the floor sweep) and reading **B** (the raw-list collection)
were taken ~12 minutes apart over the same 27 arms. Compared as **unfloored aperture id lists, in order**:

> **27 of 27 arms byte-identical. `A == C`.**

Per §9.1a this is **necessary, not sufficient** — an add-and-remove inside the window also yields `A == C`.
It is recorded as what it is.

**And one comparison in this document needs no bracket at all.** The floor sweep (§5.7) evaluates **every
floor value against one set of readings**. The arms are not separate measurements; they are one
measurement cut at fourteen thresholds. A graph movement cannot separate them, because there is nothing
between them to move.

### 5.2 What #14055 records, re-derived — one figure holds, one cannot be quoted

#14055's measurement is described as taken on *"write 17 × 23 into a file"*. **The product never ran that
string.** #13599's stored `input` reads:

> `Write a file called arithmetic.txt into your workspace containing the result of 17 multiplied by 23, and nothing else.`

Both were measured, along with three further phrasings, on the unscoped route at `count=100`:

| query | run records in the raw top 20 | top-20 band | #13599's own similarity |
|---|---|---|---|
| **#13599's real input** | **19** | 0.6933 … 0.6339 | **0.6933** |
| `write 17 × 23 into a file` | **19** | 0.6254 … 0.5940 | 0.6254 |
| `Write 17 × 23 into a file` | 18 | 0.6278 … 0.5953 | 0.6278 |
| `write 17 x 23 into a file` | 19 | 0.6252 … 0.5928 | 0.6252 |
| `write 17 multiplied by 23 into a file` | 19 | 0.6569 … 0.6081 | 0.6569 |

**The structural claim re-derives cleanly and robustly: 19 of 20, on four of five phrasings.** The corpus
also re-derives — **39 run records**, paged in full.

**The band does not.** #14055 records **0.6164 – 0.6641**; no phrasing tried here reproduces it, and the
five results span **0.5928 – 0.6933** depending only on how the task is worded. The graph is live and
unversioned, so this is **not a contradiction** — it is a reading at another graph state, and §9.1a bars
the comparison. It is reported as a finding for one reason:

> **The band is the number a floor would be chosen against, and the paraphrase's band sits 0.04–0.07 below
> the product's own.** A floor calibrated on *"write 17 × 23 into a file"* would have been set roughly
> 0.05 too low and would have cut nothing. **Every figure below is taken on the input the product ran.**

### 5.3 What the aperture actually holds today, on the ill-matched arm

#14055's 19-of-20 is a property of the **raw listing**. After #13601 the self-produced rows never reach the
aperture — so the question the product cares about is what fills the twenty slots *instead*. Reproduced
through `fuse`:

| | |
|---|---|
| raw unscoped list, `count=100` | 100 rows, **0.6933 … 0.5946**; **34** are run records |
| raw top 20 | 19 run records (0.6933 … 0.6339) + **one** real row, `#12991` (a task) at **0.6345** |
| **today's aperture**, after the exclusion | **20 rows, 0.6345 … 0.6071** |

> **The exclusion pushed the aperture *below* the rows it removed.** The twenty rows the model now sees
> top out at **0.6345** — under the 0.6339–0.6933 band of the records that were cut. #13601 traded
> nineteen rows that were ineligible for nineteen rows that are irrelevant and score lower, and **eleven of them are admitted** into the block
> — the briefing's own post-#13601 measurement (§14: *ill-matched arm … admitted 3 → 11*), which is the
> figure to quote rather than #13601 §13.2's ~7.9 estimate.

That is the defect stated in product terms: on a task the graph holds nothing for, the model is handed
**eleven documents and tens of kilobytes**, top-ranked row `#12991` — *"Nothing pins that
`PROCESSOR_WORKSPACE_DIR` reaches …"* — and asked to multiply 17 by 23.

### 5.4 The well-answered arm, and the distribution the floor is drawn against

All 25 corpus rows, each with its pinned six-query set, fused exactly as `cmd/eval` does. **Best similarity
each required document reaches across its row's queries**, and whether it is in today's aperture:

| best sim | row | required | in today's aperture |
|---:|---|---|---|
| 0.6158 | r21 | #11142 | no |
| 0.6169 | r07 | #10883 | no |
| **0.6382** | **c02** | **#10444** | **yes — the lowest one that is** |
| 0.6602 | r12 | #11262 | yes |
| 0.6632 | r13 | #11078 | no |
| 0.6650 | r08 | #10839 | yes |
| 0.6782 | r16 | #11278 | no |
| 0.6826 | r02 | #10879 | yes |
| 0.6864 | r14 | #11125 | no |
| 0.6900 | r23 | #11140 | no |
| 0.6975 | r17 | #11221 | no |
| 0.7009 | r04 | #10440 | yes |
| 0.7022 | r09 | #10926 | yes |
| 0.7089 | r03 | #10877 | yes |
| 0.7118 | r06 | #10890 | yes |
| 0.7282 | r18 | #11228 | no |
| 0.7330 | r22 | #11271 | no |
| 0.7331 | r11 | #10982 | yes |
| 0.7438 | r01 | #10861 | yes |
| 0.7664 | r05 | #10927 | yes |
| 0.7912 | c01 | #10863 | yes |
| 0.7953 | r10 | #10943 | yes |
| — | r15 / r19 / r20 | #11049 / #11084 / #11087 | **in no list at all** |

**Baseline retrieval is 13 of 25**, and twelve rows already miss before any floor exists. Three of those
twelve miss because their required document appears in **none** of the 600 rows their six queries fetch —
a ceiling on what any admission rule can do, filed in §15.

**The honest reading, and it re-confirms #13093 §6 rather than overturning it.** Two required documents
(0.6158, 0.6169) sit **below** the ill-matched arm's noise ceiling of 0.6345. The populations overlap.
**No floor separates relevant from irrelevant on this corpus** — that finding stands. What has changed is
narrower and is enough: among documents the system **actually delivers**, the lowest sits at **0.6382**,
which is **0.0037** above the ill-matched arm's best row. §6.2 says plainly what may and may not be built
on a 0.0037 margin.

### 5.5 The three placements are behaviourally identical — measured, not argued

**First, `minSimilarity` was verified against the production route** on the ill-matched arm, at four
values, by comparing the floored response to a client-side cut of the same unfloored 100-row list:

| floor | rows returned | `total` | identical, id-for-id, to the client-side cut |
|---|---|---|---|
| 0.70 | 0 | 0 | **yes** |
| 0.65 | 17 | 17 | **yes** |
| 0.62 | 28 | 28 | **yes** |
| 0.60 | 70 | 70 | **yes** |

(`total` is **post-floor**, so a floored request reports how many rows *passed* and can never report how
many were cut. That matters for D1.)

**Then three placements were computed over the same cached lists**, across all 27 arms and twelve floors:

- **ADMIT** — lists and fusion untouched; `admit` refuses rows below the floor.
- **FUSE** — `appendUnseen` refuses them; RRF scores computed from unfloored lists.
- **GRAPH** — each list truncated before fusion, which the table above shows *is* `minSimilarity`.

| floor | ill-matched rows reaching the block (ADMIT / FUSE / GRAPH) | corpus rows whose required doc reaches the block, of 25 |
|---|---|---|
| 0.00 | 20 / 20 / 20 | 13 / 13 / 13 |
| 0.60 | 20 / 20 / 20 | 13 / 13 / 13 |
| 0.62 | 7 / 7 / 7 | 13 / 13 / 13 |
| 0.63 | 2 / 2 / 2 | 13 / 13 / 13 |
| 0.64 | 0 / 0 / 0 | 12 / 12 / 12 |
| 0.68 | 0 / 0 / 0 | 10 / 10 / 10 |
| 0.70 | 0 / 0 / 0 | 9 / 9 / 9 |
| 0.75 | 0 / 0 / 0 | 3 / 3 / 3 |

> **The three placements agree in every cell, on both arms, at every floor.**

So **D1 is not a behavioural question**. #14055 predicted the difference would be *"what the run record can
say about rows that were never returned"*; the measurement confirms that prediction **and rules out any
other difference**. D1 is therefore decided entirely on disclosure and on which premises survive — §6.1.

**One thing the measurement also killed, and it is recorded because the reasoning was mine and it was
wrong.** I expected a similarity floor to be composition-neutral, on the ground that it truncates a
*suffix* of each list and so cannot compact a survivor's rank — the property #13601 §13.3 **R-H** and guard
**G-8** were written about. Measured across the corpus at floors ≥ 0.58, the GRAPH placement produces
**168 order inversions** and **152 arms into which a new id enters**, because a row floored out of *one*
list loses that list's RRF contribution and is demoted relative to rows that were not. **R-H's concern
applies to a floor after all.** It costs nothing here only because D1 never touches fusion.

### 5.6 The prerequisite: `Candidate.Similarity` is an artifact of query order

`fuseByReciprocalRank` appends a candidate **the first time any list holds it** and keeps that struct
(`internal/loop/retrieve.go:130-139`). Lists are visited in query order. So the `similarity` a candidate
carries — and therefore the `similarity` written into every `Disposition`, every stored run record, and
`internal/eval/result.go:105`'s `TopSimilarity` — is the score under **whichever query happened to come
first**, not the row's best.

Measured over the 500 aperture rows of the 25 corpus arms:

| | |
|---|---|
| rows where first-seen < best across the row's queries | **238 of 500 — 48 %** |
| mean shortfall on those rows | **0.0294** |
| largest shortfall | **0.1023** (r06, `#10461`: 0.6266 reported, 0.7289 best) |

**Before derivation merged there was one query and the field was well-defined.** PR #74 (2026-09-12) made
it ambiguous and nothing said so. Every record stored since understates about half its rows.

**What that costs the floor, measured:**

| floor | required docs kept, **best-across-queries** | required docs kept, **field as it stands** | median block rows (best) | (field as it stands) |
|---|---|---|---|---|
| 0.60 | 13 | 13 | 20 | 20 |
| 0.62 | 13 | **11** | 20 | 20 |
| **0.63** | **13** | **10** | **20** | 19 |
| 0.65 | 12 | 9 | 20 | 17 |
| 0.70 | 9 | 4 | 6 | 3 |

> **Shipping the floor against the field as it stands would cost 3 of the 13 documents retrieval
> delivers — 23 % — for no reason other than the order the queries happen to be in.** The fix is Unit 1
> and it precedes the floor.

### 5.7 The sweep

Admission placement, best-across-queries similarity, both arms, one set of readings.

| floor | ill-matched: rows in block | paraphrase arm | corpus: required docs kept /25 | corpus block rows: median / min / rows going empty |
|---|---|---|---|---|
| none | 20 | 20 | 13 | 20 / 20 / 0 |
| 0.600 | 20 | 0 | 13 | 20 / 18 / 0 |
| 0.610 | 16 | 0 | 13 | 20 / 17 / 0 |
| 0.620 | 7 | 0 | 13 | 20 / 17 / 0 |
| **0.630** | **2** | **0** | **13** | **20 / 14 / 0** |
| 0.632 | 1 | 0 | 13 | 20 / 13 / 0 |
| 0.635 | 0 | 0 | 13 | 20 / 13 / 0 |
| 0.638 | 0 | 0 | 13 | 20 / 12 / 0 |
| 0.640 | 0 | 0 | **12** | 20 / 12 / 0 |
| 0.650 | 0 | 0 | 12 | 20 / 7 / 0 |
| 0.660 | 0 | 0 | 12 | 19 / 4 / 0 |
| 0.680 | 0 | 0 | 10 | 12 / 0 / **3** |
| 0.700 | 0 | 0 | **9** | **6 / 0 / 5** |
| 0.720 | 0 | 0 | 5 | — |
| 0.750 | 0 | 0 | 3 | — |

**The knee is exactly c02's 0.6382**, and it is a cliff one row wide.

**The scoped reserve was measured separately**, because exempting it was a live alternative: across the 27
arms the reserve delivers **81 rows**, of which **16 (20 %)** fall below 0.63; **one** required document
arrives via the reserve and it is **above** 0.63. So holding the reserve to the same floor costs nothing
measurable and removes sixteen weak neighbourhood rows from blocks. It is held to the same floor (§6.1).

---

## 6. The four decisions

### 6.1 D1 — where the floor lives: **`admit`**

> **The floor is an admission rule. It is applied in `admit`, after the self-produced cut and before the
> byte budget. `Retrieve`, `fuse` and `Recall` are not touched.**

**Rejected: the graph-side `minSimilarity` parameter.** Also rejected: a selection-time cut in
`appendUnseen`.

Since §5.5 shows all three produce the same block, the decision rests on four things, and the first is
decisive.

**1. Only `admit` can disclose the cut.** `admit` already writes a `Disposition` per candidate carrying
`id`, `type`, `name`, `similarity`, `size`, `contentHash`, `sources` and `cutReason`. A floor there adds
one `cutReason` value and **every floored row stays fully described in the stored record**. Under the other
two placements the rows vanish: `Retrieve` returns a short slice and nothing names what is missing. The
graph-side form is worse still — §5.5 measured that `total` is post-floor, so the response cannot even
report a count.

**This is what makes the floor's value re-derivable from history.** With `admit`, any future question of
the form *"what would 0.66 have done?"* is answerable by reading stored records. Under the other two, it
is answerable only by re-running the product against a graph that has since moved — which is precisely the
trap #1811 names (*"designing it blind is the same calibration-trap … wait for measurement"*) reinstalled
one layer up. **D4 depends on this property.**

**2. Only `admit` leaves #13601's premise A5 standing.** A5 is *an unscoped recall at `count=100` always
returns 100 rows, because the route applies no relevance floor* — and it is what makes
`len(candidates) < CandidateLimit` an **exact** exhaustion detector rather than a noisy one. A graph-side
floor falsifies A5 **by construction**, and a selection-time floor makes a short aperture ambiguous between
*"the over-fetch bridge expired"* and *"the graph held nothing"* — two events with different correct
responses. Either would turn #13601's detector into noise on exactly the inputs where it fires most, and
would falsify its acceptance relation **AC-2** (`len(after.candidates) == CandidateLimit`) as well.

Under D1 the aperture still delivers its full width, A5 is untouched, AC-2 still holds, and the two events
stay separated **by construction rather than by a new field**: a short *candidate list* still means
headroom pressure; an empty *admitted set* means the graph had nothing. #13601 §13.3 **R-D** deleted a
"rows excluded" counter on the delete test, and this design does not reintroduce it.

**3. Only `admit` keeps the fusion path untouched.** §5.5 measured that a pre-fusion floor reorders
survivors and pulls in rows the unfloored aperture never held — the R-H shape #13601 §13.3 prohibits and
G-8 detects. `admit` runs after fusion is finished and cannot perturb it. **G-8 stays green without being
consulted**, which is the strongest form of not interacting with a prohibition.

**4. It is where the prior design already put it.** #13601 §16.3 moved exactly **one** rule —
self-produced — across the retrieval/admission boundary, on the stated ground that it is an *eligibility*
property, and said the remaining five rules *"are untouched and stay where that design puts them"*. The
floor is one of the five. **Moving it now would reopen a boundary that was drawn deliberately and with a
reason that still holds**: provenance is knowable before ranking; relevance *is* the ranking.

**Where the rejected alternative would have won, and why the win is not claimable.** A graph-side floor
would cut wire bytes: on the ill-matched arm the over-fetch pulls 100 bodies per query and a 0.63 floor
would return 2. That is real. It is also **unmeasured** — no retrieval-latency or byte figure exists for
this system — and #13534 §3 is a standing ruling against optimising a quantity whose input has not been
measured. #13601 §13.3 **R-F** rejected the two-phase fetch on the identical ground and filed it with a
trigger. **This design does the same and does not double-count the argument**: §15 item 5.

**The floor applies uniformly, including to scoped-reserve arrivals.** Exempting the reserve was
considered and rejected: an exemption is a slot guaranteed to a row *regardless of how bad it is*, which is
the shape this whole design removes. §5.7 measured the cost of uniformity at zero required documents.

### 6.2 D2 — the value: **0.63**

> **`RelevanceFloor = 0.63`.**

**Rejected: 0.70.** It is the demoted design's number and the operator's stated noise floor, and it was
**never measured against this corpus**. Measured here (§5.7) it costs **4 of the 13** documents retrieval
delivers, drops the median block from 20 rows to **6**, and **empties the block on 5 of 25 rows**. That is
not a floor; it is a different product. It is reported as a disagreement with a prior design's proposed
value — **OQ-1** puts the product question back to Toni rather than deciding it here.

**Rejected: 0.638**, the largest value with zero measured cost. It sits inside a window **0.0037** wide,
bounded above by one control row's required document (c02 / `#10444` at 0.6382) and below by one row of
the ill-matched arm (`#12991` at 0.6345). **A number fixed by the fourth decimal of a single sample on a
single graph state is a calibration trap**, and #1811's discipline is the reason not to take it.

**Why 0.63 is the pick.**

| | |
|---|---|
| cost on the well-answered arm | **zero.** 13 of 25 required documents kept — identical to no floor. No row's block falls below 14 of 20. No row goes empty |
| effect on the badly-answered arm | **20 rows → 2.** Eleven admitted documents (briefing §14) become at most two |
| clearance | **0.0082** below the lowest required document the system actually delivers, so ordinary graph drift does not immediately start cutting evidence |
| shape | two decimals, on the grid, not fitted to any row's fourth decimal |

**What 0.63 does not claim.** It is **not** a semantic separator — §5.4 re-confirms that the populations
overlap, and #13093 §6's ruling that a floor cannot be the ranking mechanism stands untouched. Its job is
narrower and is the one the briefing asks for: **when the graph has nothing, stop filling the block.**

### 6.3 D3 — the empty or short aperture: **say so in the block; build nothing else**

> **When `admit` admits nothing, `renderBlock` emits the anchor followed by one line stating that
> retrieval returned nothing above the relevance floor. No new terminal reason, no re-query loop, no
> catalogue.**

**Rejected: `retrieval-admission-and-the-empty-outcome.md` Unit 2** (a `NeedsClarification` terminal
reason plus a bounded re-query loop), which remains demoted.

**Its premise is measured false at the chosen value.** #14055 states that *"a floor makes the empty outcome
common rather than hypothetical"*. At 0.63, across 25 corpus rows, **zero** blocks go empty and the
smallest holds 14 rows. The empty outcome becomes common at **0.68 and above** (3 of 25), which is a
consequence of 0.70, not of a floor. **The two questions meet at 0.70 and do not meet at 0.63.**

**And its own gate is unrun.** That design requires a two-population differential — clarification rate must
*separate* inputs the graph can answer from inputs it cannot, or the outcome is either over-asking or
nominal. It also records that population A *"demonstrably does not exist"* on the corpus it was written
against. **That is no longer true** — §5.4 is population A, 13 rows deep, and §5.3 is population B. So the
gate is now runnable, which is a reason to run it before building the outcome, not a reason to skip it.
**OQ-2.**

**What is built instead, and why it is not nothing.** The tool-result path already carries this exact
sentence — `RenderToolResult` emits *"results were found, but none were included."* when dispositions exist
and results do not (`internal/loop/assemble.go:109`, #13594 Unit 2). The **block** path has no
equivalent: it renders an anchor and stops, and an absence of `===== CANDIDATE =====` sections is something
the model must infer. Adding the statement is **DRY with a decision already taken**, not a new mechanism —
and it is the "would he be able to tell" half of the briefing §11 replacement test, which a silent empty
block fails.

**Existing signals are sufficient for the operator and are checked rather than assumed.** With D1 the
candidate list still holds 20 rows, so `turn.go:236`'s shutout WARN — guarded on
`len(Candidates) > 0 && cut == len(Candidates)` — **fires** on a fully-floored run. The under-delivery WARN
at `turn.go:245` keeps its exact premise. No new WARN is added; §12's KISS table records the delete test.

### 6.4 D4 — `const`, not a dial

> **`const RelevanceFloor = 0.63` in `internal/loop`, beside `CandidateLimit`; surfaced in `Limits` as
> `relevanceFloor` so every record states the number that governed it.**

**Rejected: a sweep dial or config knob.**

#1136 §3's three gates — a named operator who will tune it, a real environment difference, a secret — are
**all three unmet**, and the fourth argument is the decisive one:

> **A dial would buy the ability to re-measure the floor, and D1 already gives that away for free.**
> Because `admit` records every candidate's similarity and its cut reason, **every stored record already
> answers what any other floor would have done.** A dial would let you re-run an experiment whose answer is
> already in the file.

This is #1136 §3's *telemetry-then-tune compound* refused in both halves: no knob, and no audit column
either — the disposition rows that carry the answer exist for other reasons and are not being added for
this one.

Mechanically it also matters that `cmd/eval` reads `loop.CandidateLimit` and its siblings **directly as
constants** (`internal/eval/result.go:71`). A dial would need new plumbing through `Assemble`, `sweep` and
`Result` to reach the one place that wants it. `Assemble` already takes `budget` as a parameter; the floor
travels the same way from the same constant, which is DRY with the shape already there.

**The trigger for revisiting it, named rather than left to "later":** when **#14054 Unit 2**'s backfill
lands, 39 records change shape and the distribution moves (§13). At that point the floor's value is
re-derivable **from stored records alone**, with no sweep and no dial. §15 item 1.

---

## 7. Architectural Overview

```
  input ──► DeriveQueries ──► queries[0..n]
                                  │
                    ┌─────────────┴──────────────┐
                    ▼                            ▼
              Recall × n (unscoped)        Recall (scoped)          ── UNCHANGED
                    │                            │                     no floor,
                    └──────────► fuse ◄──────────┘                     no minSimilarity,
                                  │                                    A5 intact
                                  │  · RRF over unfloored lists
                                  │  · anchor + self-produced refused
                                  │  · Similarity := best across the
                                  │    lists that returned the row  ◄── UNIT 1
                                  ▼
                        candidates[] — always up to CandidateLimit
                                  │
                                  ▼
                               admit                                ◄── UNIT 2
                                  │
                  ┌───────────────┼────────────────┬──────────────┐
             self-produced   below floor       fits budget    over budget
                  │               │                 │              │
            cutReason:      cutReason:          included      cutReason:
          "self-produced"  "below relevance      = true      "byte budget
                             floor"                           exceeded"
                                  │
                                  ▼
                            renderBlock
                    anchor, then admitted rows —
                    or anchor + the empty statement   ◄── UNIT 2
```

**The whole change is inside `internal/loop`.** No port widens, no adapter changes, no new round trip, no
new type.

---

## 8. Components & Responsibilities

| component | owns, after this change | does **not** own |
|---|---|---|
| `internal/divoid.Client.Recall` | issuing the recall the loop asks for | **any floor.** It gains no `minSimilarity` — D1 |
| `Retrieve` / `fuse` | eligibility (anchor, self-produced), fusion order, the reserve, and — new — **reporting each candidate's best similarity across the queries that returned it** | relevance. It refuses no row for being weak |
| `admit` | the ordered admission policy: self-produced, **then the relevance floor**, then the byte budget — and a `Disposition` for every candidate either way | ranking, fetching, and what the block *looks* like |
| `renderBlock` | the block's layout, including **the statement that nothing was admitted** | deciding what is admitted |
| `Limits` | stating the constants that governed the run, now including `relevanceFloor` | holding a tunable value |
| `Turn`'s summary block | the two existing WARNs, unchanged | a third WARN for the floor — deleted, §12 |

---

## 9. Interactions & Data Flow

**The changed path, conceptually.**

1. `Retrieve` fetches and fuses exactly as today. **One thing is added:** as the fused order is built, each
   candidate's reported similarity becomes the **maximum** over every list that returned it, rather than
   the value from the first. Ranks, RRF scores, the reserve and the exclusion are all unchanged — this
   changes a *reported field*, never a *decision*, and that separation is what keeps it outside R-H.
2. `Assemble` receives the candidates and — as it already does for the byte budget — the floor.
3. `admit` walks the candidates in rank order and classifies each into one of four states. **The floor sits
   between the self-produced cut and the budget**, and a row it refuses charges **nothing** against the
   running byte total, exactly as a self-produced row charges nothing today.
4. `renderBlock` renders the anchor, then the admitted rows. When there are none, it renders the anchor and
   the statement.
5. The record carries: the floor in `Limits`, and every candidate with its similarity, its inclusion and
   its cut reason.

**Ordering inside `admit` is load-bearing and is the same reason the demoted design gave.** Self-produced
first, because a run record that also happens to be below the floor should be reported as what it
structurally is. Floor before budget, because a row refused for irrelevance must not consume headroom that
a relevant row behind it could use — which is the whole mechanism by which the ill-matched arm's block
shrinks.

---

## 10. Data Model (Conceptual)

No entity is added. Two fields change meaning or arrive:

| where | field | change |
|---|---|---|
| `Candidate` / `Disposition` | `similarity` | **meaning corrected**: the row's best similarity across the queries that returned it, rather than the first one's. Unit 1 |
| `Limits` | `relevanceFloor` | **new**: the floor that governed this run. The doc comment reading *"the five constants"* becomes six |

`Disposition.cutReason` gains one value in its open set of strings. It is a string today
(`"self-produced"`, `"byte budget exceeded"`); this adds `"below relevance floor"` and introduces no enum,
no parallel constant holder and no mirror type.

**Backward reading of stored records.** Records written before this change carry no `relevanceFloor` and a
first-seen `similarity`. Any tool that re-derives a counterfactual floor from history must treat
pre-change records as **understating** about half their rows by a mean of 0.0294 (§5.6). That is stated
here so the limitation is a known property rather than a silent one; nothing is migrated.

---

## 11. Contracts & Interfaces (Abstract)

| contract | statement | why it is checkable |
|---|---|---|
| **C1** | For every candidate the loop reports, its stated similarity is the **greatest** value any of that run's recalls returned for it, and is never a value no recall returned | the run record carries `sources[]` naming every query and rank that produced the row; the similarity must be consistent with that set |
| **C2** | Every candidate `Retrieve` returns appears in `candidates[]` with exactly one outcome: included, or cut with exactly one reason | already true; the floor must not create a second reason on one row |
| **C3** | A candidate cut below the floor contributes **zero** bytes to the running budget | the admitted byte total must equal the sum of the sizes of the rows marked included, and nothing else |
| **C4** | The count of candidates is unaffected by the floor | `len(candidates) == CandidateLimit` whenever the fetch was not exhausted — #13601's **AC-2**, preserved |
| **C5** | The block contains a `CANDIDATE` section for exactly the included rows, and when there are none it contains a statement to that effect rather than an unremarked absence | the block is stored verbatim in the record |
| **C6** | The floor that governed a run is stated in that run's record | a reader must never have to know which build produced a record to know what floor it ran under |

---

## 12. Cross-Cutting Concerns, and the KISS accounting (#1136 §4)

**Observability.** Two WARNs exist and both keep their exact premises under D1 (§6.1 point 2). No third is
added.

**Security / secrets.** Nothing.

**Determinism.** `Assemble` stays a pure function — no I/O, no clock, no randomness. The floor is a
constant argument, like the budget.

**Concurrency, retries, idempotency.** Unchanged; no new I/O.

**Consistency.** The floor is applied to a set of rows already in hand, so it has no interaction with graph
mutation during a run.

| element | can it be deleted? | can it be merged? | can it be inlined? |
|---|---|---|---|
| the floor clause in `admit` | **no** — it is the feature | it **is** the merge: one more case in the switch that already holds the self-produced and budget cases | it is one case |
| the best-similarity correction | **no** — §5.6 measures the floor costing 3 of 13 documents without it, and D4's whole argument rests on records being re-floorable | it merges into the existing dedup pass in `fuseByReciprocalRank`; no new traversal | it is where the candidate is already being kept or skipped |
| `Limits.RelevanceFloor` | **no** — C6. A record that does not state its floor cannot be read years later, and #13534 §9's *"a completion word is a claim about a set"* is the same failure one field over | it joins the five constants already there | — |
| the empty-block statement | **no** — the tool path has it and the block path does not, and the asymmetry is the defect | it merges into `renderBlock`, one branch | it is one branch |
| a **third WARN** for "the floor cut everything" | **yes — deleted.** The shutout WARN already fires on that exact arm (§6.3), and adding a second signal for one event is how a log stops being read | — | — |
| a **count of floored rows** in the record | **yes — deleted.** Computable from `candidates[]`, which already carries every row's reason. #13601 §13.3 **R-D**, same test, same answer | — | — |
| a **config knob / sweep dial** | **yes — deleted.** D4 | — | — |
| a **`minSimilarity` parameter** on `Recall` | **yes — deleted.** D1; §5.5 measures it buying no behavioural difference | — | — |
| a **helper shared** by `fuse`'s exclusion and `admit`'s floor | **yes — deleted.** They are different predicates on different properties; 1 line × 1 site, far below #1267's ~15–20 | — | — |

**#1136 §5 checklist, walked.** No new type. No new abstraction. No element justified by "we might need X".
No deprecation period, feature flag or shim. No new persisted data point beyond one constant that C6
requires. Every knob gate in §3 is unmet, so the value stays a `const`. Out-of-scope items are enumerated
in §2. Both Code Contracts (#114) and Design Contracts (#1136) are load-bearing on this document.

---

## 13. The ordering constraint, and the bad news for it

> **This design gates any relaxation of the self-produced exclusion. It does not perform one, and it does
> not propose one.** `internal/loop/assemble.go:53` and `internal/loop/retrieve.go:90` are untouched, and
> `TestFuseExcludesExactlyWhatAdmitWouldCutAsSelfProduced` must stay green **without being edited** — if it
> needs editing, something has gone outside this design's scope.

The combination to avoid is *admissible run records* **plus** *no relevance floor*. This design removes the
second half. **It does not make the first half safe**, and the measurement says so:

| figure | status |
|---|---|
| #13599 on a verbatim repeat of its own input, as it exists today | **0.6933 — re-derived here**, agreeing with #14054 §4.5 to four decimals |
| #14054 §4.5's measured delta from reshaping that record's body | **+0.1272 — inherited, not re-derivable** (§4): probe #14051 returns 404 |
| the implied position of a reshaped record on that input | **≈ 0.82** |

**0.82 is above every value in the swept grid, and above every required document in the corpus except
r10's 0.7953.** So after #14054 Unit 2's backfill, a run record admitted on relevance would sit at the top
of the ill-matched arm's aperture rather than at the bottom — **the floor is necessary and is not
sufficient.**

This does not weaken the ordering; it sharpens it. #14054 §13.2 says the remaining gate is *"the relevance
floor in §20, and it is a gate on the aperture, not on the record"*. The measurement says the gate does not
close on the floor alone. **Whatever eventually admits run records needs an argument this design cannot
supply**, and §15 item 2 files that rather than folding it in.

---

## 14. Risks, and the falsifier each one is caught by

> **No Go was run for this document.** Every guard below is a **prediction and an obligation on the unit**,
> per #1220 §5. The unit is not done until each has been run and its observed output recorded in the PR.

| # | risk | falsifier |
|---|---|---|
| **R-1** | **0.63 is fitted to one graph state.** The ill-matched arm's ceiling and c02's document are 0.0037 apart, and both move | a sweep after #14054 Unit 2's backfill shows a required document the corpus delivers falling below 0.63. **Trigger, not hypothetical** — §15 item 1 |
| **R-2** | **The corpus is not the product.** 25 rows over one graph, none of them a real task Toni gave | the first container run after this lands reports a block the operator judges too thin. The floor is one `const` and the reversal is one edit |
| **R-3** | **The best-similarity correction changes a reported number many things read**, including `internal/eval/result.go:105`'s `TopSimilarity` and `scripts/step_trace.py` | an eval sweep's reported `topSimilarity` rises on rows whose retrieval verdicts did not change — which is the *expected* direction and must be **recorded rather than treated as a regression**. A row whose verdict changes is a finding |
| **R-4** | **The floor silently becomes the byte budget's problem.** Rows freed by the floor let bigger rows in behind them | the ill-matched arm's admitted **byte** total rises after the change. Measured expectation: it **falls**, because the floor removes rows rather than reordering them — #13594 owns the budget half either way |
| **R-5** | **A model handed an empty block does something worse than a model handed noise** — Gangolf's #1317 shape, where a new gate made the model stop acting rather than act differently | a container run on the ill-matched input returns a worse artefact than today's. **This is the risk OQ-1 is really about** |
| **R-6** | **The two WARNs become indistinguishable** if a later change moves the floor to retrieval | `len(candidates) < CandidateLimit` fires on a run where the fetch returned 100 rows and nothing was self-produced |

### Guards, each naming the premise that makes it discriminate (#11034 P-42)

| # | guard | premise |
|---|---|---|
| **F-1** | `TestACandidateCarriesItsBestSimilarityAcrossEveryQueryThatReturnedIt` — two lists holding the same id at different similarities, the **lower** one first; assert the reported value is the higher | Against `aec8d3f` this fixture reports the lower value, so a no-op implementation reddens it. **It is the only guard that fails today** |
| **F-2** | `TestTheBestSimilarityCorrectionLeavesTheFusedOrderExactlyAsItWas` — dual-arm over **two** lists, before and after the correction; assert identical returned ids **in identical order** | This is what keeps the correction out of R-H's class. A similarity is a reported field and a rank is a decision; a guard that only checked the ids would pass an implementation that re-sorted by the corrected value |
| **F-3** | `TestARowBelowTheRelevanceFloorIsCutWithItsOwnReasonRatherThanSilentlyDropped` — assert the row is present in `candidates[]`, `included == false`, and the reason is the floor's | Distinguishes D1 from the two rejected placements, under which the row is absent entirely. An assertion on the admitted set alone cannot see the difference |
| **F-4** | `TestARowCutBelowTheFloorChargesNothingAgainstTheByteBudget` — a large below-floor row ahead of a small above-floor one that only fits if the large one charged nothing | C3. An implementation that cut the row *after* incrementing `cumulative` passes F-3 and fails this |
| **F-5** | `TestTheRelevanceFloorIsAppliedAfterTheSelfProducedCutAndBeforeTheByteBudget` — one row that is self-produced **and** below the floor, one that is below the floor **and** oversize; assert each reason | The ordering is unobservable on rows that trip only one rule, so the fixture has to be rows that trip two |
| **F-6** | `TestAScopedReserveArrivalIsHeldToTheSameFloorAsAFusedOne` — a reserve row below the floor; assert it is cut | Pins §6.1's uniformity decision against the tempting exemption, which every other guard would accept |
| **F-7** | `TestTheApertureStillDeliversItsFullWidthWhenTheFloorCutsEveryRow` — every candidate below the floor; assert `len(candidates) == CandidateLimit` and every one cut | **A5 and AC-2.** This is the guard that fails the moment anyone moves the floor into `fuse` or onto `Recall`, which is the one regression this design most wants detected |
| **F-8** | `TestTheBlockStatesThatNothingWasAdmittedRatherThanRenderingTheAnchorAlone` — assert the rendered block contains the statement | C5. A test asserting "no CANDIDATE sections" passes today |
| **F-9** | `TestTheRecordCarriesTheFloorThatGovernedTheRun` | C6 |
| **F-10** | `TestFuseExcludesExactlyWhatAdmitWouldCutAsSelfProduced` — **existing, G-4 of #13601. Must stay green and must not be edited** | §13. If this unit needs to touch it, the unit has left its scope |

**Falsifier for this table itself:** *any row whose named guard would still pass against an implementation
lacking the claimed property.* **F-2, F-4, F-6 and F-7 exist only because an earlier row failed it** — F-2
because ids alone cannot see a re-sort; F-4 because F-3 cannot see the accumulator; F-6 because no other
guard visits the reserve; F-7 because every guard above it is satisfied by a `fuse`-side floor.

**Mechanical pre-submit check, to be run and its output recorded rather than asserted:**

```
grep -rn "TestACandidateCarriesItsBest\|TestTheBestSimilarityCorrectionLeaves\|TestARowBelowTheRelevanceFloor\|TestARowCutBelowTheFloorCharges\|TestTheRelevanceFloorIsAppliedAfter\|TestAScopedReserveArrivalIsHeld\|TestTheApertureStillDeliversItsFullWidth\|TestTheBlockStatesThatNothingWasAdmitted\|TestTheRecordCarriesTheFloor" internal/loop/
```

Nine names, nine hits. Re-run **after** the implementation round, not before.

---

## 15. What this does not fix — filed, not solved

| # | item | disposition |
|---|---|---|
| 1 | **The floor's value must be re-read after #14054 Unit 2's backfill.** 39 records change shape and §13's figures say the distribution moves upward. **Re-derivable from stored records alone** after Unit 2 of this design | **#14085**, open |
| 2 | **The floor is not a sufficient gate on relaxing the self-produced exclusion** (§13). Whatever admits run records needs an argument this design cannot supply | **#14082**, open |
| 3 | **#14054 §4.5's +0.1288 is not re-derivable** — probe #14051 is 404 (§4). The claim's direction is load-bearing for §13 and currently rests on a deleted node | **#14083**, open. Cheapest close is to wait for #14054 Unit 2, which makes the delta measurable on live nodes |
| 4 | **Three of 25 corpus rows' required documents appear in no recall list at all** — r15/#11049, r19/#11084, r20/#11087 — across six queries at `count=100`. No admission rule can reach them, and they bound what the corpus can measure | **#14084**, open |
| 5 | **The wire cost of the over-fetch is unmeasured** (§6.1). If it is ever measured, the graph-side floor and #13601 §19's two-phase fetch become the same conversation and should be had once, not twice | note on #13601 §19; not a new task |
| 6 | **#14055's band cannot be quoted** (§5.2), and the figures it records were taken on a paraphrase of the input rather than the input. The structural claim it rests on re-derives; the number does not | **#14086**, with the five-phrasing table. To be carried back to **#14055** and **#14054 §4.5** as dated notes |
| 7 | **`Disposition.similarity` in every record stored since PR #74 understates about half its rows** (§5.6). Unit 1 stops it recurring; nothing repairs the history | **#14081**, open — the prerequisite, and a standalone defect |

---

## 16. Open Questions — the two that need Toni rather than a measurement

**OQ-1 — 0.70 or 0.63? This is a product preference and it is not mine to take.**
0.70 is the operator's own stated noise floor and the demoted design's number. Measured on this corpus it
costs 4 of the 13 documents retrieval delivers and empties the block on 5 of 25 rows. **0.63 keeps every
document and still cuts the ill-matched block from twenty rows to two.** The choice between them is the
choice between *a harness that would rather refuse than answer weakly* and *a harness that answers with
whatever genuinely matched*. This design ships 0.63 because it is the value with a measured cost of zero;
if the preference is the former, the number changes and nothing else in this document does.

**OQ-2 — should the clarification outcome be built, and is its gate worth running now?**
§6.3 declines to build it because at 0.63 the empty case does not occur on the corpus. But its
two-population differential is now **runnable for the first time** — §5.4 is population A and §5.3 is
population B, and the demoted design recorded that population A did not exist. Running that gate is a
half-day of container runs and it would settle a question that has been open since 2026-09-07. **It is not
this unit, and it should not be filed as "later" without a decision.**

*(A third question — whether the floor should apply to the supplementary `dispatchRecall` path as well as
the initial assembly — is not open: it should, for the same reason, and it falls out for free because that
path also runs through `admit`. It is named here only so its absence from the table is not read as an
oversight.)*

---

## 17. Implementation Guidance for the Next Agent

**No code in this document. Two units, two branches, two PRs** (#8385's one-feature-one-PR rule).

### Unit 1 — a candidate reports its best similarity, not its first

**Ships first and alone.** It is a defect fix with standalone worth — §5.6 — and it is the prerequisite
that makes Unit 2's number honest and D4's argument true.

1. In `fuseByReciprocalRank` (`internal/loop/retrieve.go:125`), when a list holds a candidate already seen,
   keep the **greater** of the two similarities on the retained struct. **Change nothing about `scores`,
   the sort, or the order** — F-2 is the guard that this stayed a reported field and did not become a
   decision.
2. Decide and state whether the scoped list participates. **It should** — it is a recall like any other and
   its similarity is a real reading — but `fuse` receives `scoped` separately from `lists`, so this is an
   explicit choice rather than a consequence. §5.7's reserve measurement assumes it does.
3. Add **F-1** and **F-2**.
4. **Sweep for readers of the field, and re-derive the list rather than trusting §5.6's.** Known:
   `internal/eval/result.go:105` (`TopSimilarity`), `internal/eval/report.go:166`,
   `scripts/step_trace.py:382`. **`scripts/compare.py` is in flight on #14054 Unit 1 — check its state
   before touching it and hand the edit over if the tree is dirty from another hand** (P-50).
5. Run a sweep before and after and **report the `topSimilarity` movement**, per R-3. Do not gate on it.

**Do not change** `assemble.go`, `client.go`, or anything about admission in this unit.

### Unit 2 — the floor

**Its own PR, after Unit 1 is on `main`.**

1. Add `RelevanceFloor = 0.63` beside `CandidateLimit` in `internal/loop/turn.go:12`, with a comment that
   states **what it was measured against** — the two arms, and the fact that 0.638 was the zero-cost
   maximum and was not taken.
2. Add the cut reason beside `cutReasonByteBudget` and `cutReasonSelfProduced`
   (`internal/loop/assemble.go:12`).
3. Pass the floor into `admit` the way `budget` already travels, and add its case to the switch —
   **after** self-produced, **before** the budget case, charging nothing to `cumulative`.
4. Add `RelevanceFloor` to `Limits` and to the run summary; update the *"five constants"* comment
   (`internal/loop/types.go:169`).
5. Add the empty-block statement to `renderBlock`. **Match the tool path's existing sentence in spirit**,
   and do not invent a second vocabulary for the same event.
6. Add **F-3 … F-9**. Run **F-10** and confirm it is green **without having been edited**.
7. Re-run `cmd/eval` on both arms and record: required-document retention on the corpus (expected **13**,
   unchanged) and the ill-matched arm's admitted row count and byte total (expected to **fall**).

### The order, and the one thing that must not happen

**Nothing in either unit relaxes the self-produced exclusion**, and nothing should until §15 item 2 is
answered — which §13 shows this design does **not** answer. If a round of this work finds itself editing
`retrieve.go:90`, `assemble.go:53`, `IsRunRecord`, or `TestFuseExcludesExactlyWhatAdmitWouldCutAsSelfProduced`,
it has left the scope of this document.

---

## 18. What a user gets

Not *"the aperture gains a floor"*.

**Today**, if Toni hands the container a task the graph holds nothing for — *"write a file containing 17
multiplied by 23"* — the loop retrieves twenty documents scoring 0.6345 and below, admits eleven of
them, and puts tens of kilobytes about workspace environment variables, repo-map reconciles and PR-scope
rulings in front of the model, immediately above the request. The model then does the task with all of
that in its context, and the run looks exactly like a successful one. Nothing in the answer, the log or the
record says the memory was useless.

**After this change**, the same task produces a block with the anchor and **at most two** documents — and
when the graph has nothing at all, the block says so in one line. The model gets the task and almost
nothing else, which is the true state of the world. On the thirteen corpus tasks the graph *does*
answer, **nothing changes at all**: the same twenty candidates, the same documents, the same block.

And when an answer is bad, the run record now shows **why**: each row that did not reach the model carries
its own score and the reason it was cut. That is the second half of the briefing's own test —
*would he be able to tell?* — and it is the half the product has been failing.
