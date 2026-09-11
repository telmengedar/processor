# Architectural Document: The Aperture Spends Slots Admission Refuses

> Repo path: `docs/architecture/the-aperture-spends-slots-admission-refuses.md` (canonical copy — the
> DiVoid node carries the same document verbatim, under **#11034 P-40** parity).
> DiVoid node: **#13601**.
> **Parity contract (#1220 §3), stated as a check rather than as a number:** node **#13601** holds the
> **LF bytes of this file**, verified by downloading the node and comparing the sha256 — not the byte
> count, since two files of equal length are not the same file. The digest is reported with the
> publish rather than written inside the document, because a digest inside the file it hashes moves
> every time it is written. This repo's `.gitattributes` carries `*.md text eol=lf`, so the working
> tree, the committed blob and the node are the **same LF bytes on any machine**; the CRLF divergence
> #1220 §3 measured does not arise here, and that is a property of *this repo's* `.gitattributes`
> rather than a general one.
> Task: **#13593**. Measurement: **#13592**. Run records read directly: **#13591**, **#13598**, **#13599**,
> **#13600**. Product briefing: **#13534**, including §10 and its §11 correction.
> Standards applied: Design Contracts **#1136** (§5 walked as §17 below), Code Contracts **#114 §0** and
> **§4** via the Go annex **#10861**, Go contract set **#11034**, DRY threshold **#1267**, bouncing grade
> **#1380**, briefing template **#1220** — all **cited, not restated**.
> Baseline: `main` at **`0df5c14`** (`git rev-parse HEAD` returns `0df5c14843f19486d2d32709bcf3eead76397bac`,
> working tree clean). Every code fact in §2 was read out of that tree; every graph fact was measured
> against the live graph on 2026-09-11 and the command is published beside it.

---

## TL;DR

**What.** Stop the candidate aperture spending its twenty slots on rows admission rejects before it
looks at them. Two edits, both in `internal/loop/retrieve.go`.

**How.** (1) `fuse`'s `appendUnseen` closure rejects a self-produced row exactly as it already rejects
the anchor — one clause, in the one place all three fill passes admit through. (2) `Retrieve` asks the
graph for `limit x 5` rows on every recall it issues, while still returning at most `limit`. The fetch
count and the aperture stop being the same number.

**Not a judgement about run records.** The predicate is *inherited from `admit`*, which already rejects
them unconditionally before any test of content. Nothing here says self-produced memory is worth less.

**Cost.** The fetch rises from 20 to 100 rows on **every** arm. Bodies per recall: ~2.8 MB predicted on
an ill-matched input against **1,085,093 B measured** today; on a well-matched one, **143,089 B measured**
over 20 rows (~7,154 B/row), so of order 700 KB — the per-row figure is measured, the extrapolation to
ranks 21–100 is not. No new type, no new record field, no change to `admit`, the budget or the block. **One WARN** in `turn.go` fires when the multiplier was not enough — the detector
for the one claim here that has an expiry.

**Rejected.** Excluding at the graph query — `type=` is include-only, and the nearest approximation
would drop **926 legitimate `session-log` nodes to remove 37**.

---

## 1. Problem Statement

`Retrieve` rations twenty candidate slots without knowing the one rule that makes a row worthless to the
stage it is rationing them for. `admit` rejects a self-produced row **first, unconditionally, and before
any test of its size or content** (`internal/loop/assemble.go:52-53`). `fuse`
(`internal/loop/retrieve.go:77-118`) contains no `SelfProduced` predicate. So a row whose fate is already
decided, deterministically and knowably in advance, displaces a row that could have been used.

The measured cost is not uniform, and its distribution is the reason this is ranked first rather than
filed: **the crowding is anti-correlated with the memory being useful.** On an input the graph answers
well it costs nothing. On an input the graph answers badly it takes most of the aperture — which is
exactly the input on which the few real rows that exist matter most.

**Goal state, in one sentence (#13534 §7):** *a run's candidate aperture delivers rows the run can
actually use, on the inputs where the graph has least to offer.*

---

## 2. The measured state at `0df5c14`

### 2.1 The mechanism, read out of the tree

| fact | where |
|---|---|
| `fuse` has no `SelfProduced` predicate; its only per-row exclusion is the anchor, seeded into `taken` | `internal/loop/retrieve.go:81` |
| The only production read of `Candidate.SelfProduced` is inside `admit`, after `fuse` has applied `limit` | `internal/loop/assemble.go:52` |
| `admit` cuts it **before** the byte test, so it is charged nothing and can never be admitted at any budget | `internal/loop/assemble.go:51-60` |
| The graph adapter sets the flag from type **and** name prefix — `session-log` alone is not the predicate | `internal/divoid/write.go:33-34`, `internal/divoid/client.go:185` |
| `Recall` sends `count = limit`, so the fetch and the aperture are the same number | `internal/divoid/client.go:165`, `internal/loop/retrieve.go:19` and `:31` |
| Every stored run record is linked to its subject, so every record is inside that subject's `RecallScope` | `internal/divoid/write.go:72`, `internal/loop/retrieve.go:62-74` |

**Sweep for the predicate, published as run:**

```
grep -rn "SelfProduced\|self-produced\|IsRunRecord" --include=*.go . | grep -v "_test.go" | grep -v "\.claude/"
```

**20 hits across 8 files**, counted by running the string above and not by recalling it:

| file | hits | what they are |
|---|---|---|
| `internal/loop/types.go` | 2 | the field's declaration and its doc line |
| `internal/loop/assemble.go` | 3 | the cut reason and the two lines of the only production read on the admission path |
| `internal/divoid/client.go` | 1 | the classifier that sets the flag |
| `internal/divoid/write.go` | 2 | `IsRunRecord` and its doc line |
| `internal/eval/result.go` | 3 | the sweep diagnostic (§15) |
| `internal/eval/report.go` | 3 | the same diagnostic's rendering (§15) |
| `internal/condense/condense.go` | 5 | **a different field on a different type** — `condense.SubstanceNode.SelfProduced`, the offline pass's own skip reason. Not `loop.Candidate.SelfProduced`, and untouched by this design |
| `cmd/condense/adapters.go` | 1 | that pass's composition root, same distinction |

### 2.2 What the aperture costs, measured from the stored records

Both records were downloaded from the graph and read directly rather than taken from #13592's tables.
**Published as run** (`_13599.json` / `_13598.json` are the node bodies of #13599 and #13598):

```
python -c "import json,sys;c=json.load(open(sys.argv[1],encoding='utf-8'))['candidates'];print(len(c),sum(1 for x in c if x.get('cutReason')=='self-produced'),sum(1 for x in c if x.get('cutReason')=='byte budget exceeded'),sum(1 for x in c if x.get('included')),sum(x['size'] for x in c),sum(x['size'] for x in c if x.get('cutReason')=='self-produced'))" _13599.json
```

| run | input | candidates | self-produced | cut on size | admitted | candidate bytes fetched | bytes on self-produced |
|---|---|---|---|---|---|---|---|
| **#13599** | *"Write a file called arithmetic.txt … 17 multiplied by 23, and nothing else."* | 20 | **17** | **0** | **3** | **1,085,093** | **1,067,689 (98.4 %)** |
| **#13598** | *"Write a file called go-comment-rules.md … and say where those rules are written down."* | 20 | **0** | 9 | **11** | 143,089 | 0 |

Two things follow that #13592's tables do not state:

- **On #13599 the byte budget cut nothing.** Zero rows were refused for size. The aperture was the
  *entire* binding constraint, and it delivered three usable rows totalling 17,404 B against a 56,592 B
  candidate budget. The run was not starved of bytes; it was starved of rows.
- **The seventeen records arrived in an unbroken block at ranks 1–17, every one of them from the
  unscoped list at unscoped ranks 1–17.** `fuse` fills to `limit - reserve` = 17 from the fused order,
  so the fill consumed exactly the record block and stopped; unscoped ranks 18 and 19 were never
  reached. The three surviving rows came from the scope reserve.

### 2.3 Two figures in the handed evidence do not hold, and one is not a constant

Reported per the brief rather than absorbed.

1. **#13593's closing line says the seventeen records are *"70–100 KB each"*. Measured from #13599's own
   dispositions: 37,163 – 77,554 B, median 66,804** — wrong at both ends, since none reaches 100 KB and
   five are below 70 KB. The 100,350 B figure belongs to **#13598's own node**, verified by download
   (`bytes_written: 100350`), not to any member of that set.

   > **Corrected in this document, same day, and the correction is more instructive than the figure.**
   > An earlier revision of this row also indicted **#13592 §3's *"66–72 kB each"***. **That sentence is
   > right.** §3 describes **#13591's nine** rows, not #13599's seventeen, and those nine measure
   > `66771 68304 68320 66804 69354 69871 70660 71102 71723` — **66,771 – 71,723 B**, exactly what it
   > claims. Re-derived here from #13591's own body rather than accepted from the correction.
   >
   > **The error was symmetrical with the one it was correcting, and that is the finding:** #13593's line
   > took a range measured over nine rows and let it travel to a sentence about seventeen; this document
   > took a range measured over seventeen and applied the correction to a sentence about nine. **A range
   > is a property of a set and does not survive being moved to another one.** Both moves read as
   > ordinary reuse of an already-measured figure, which is why neither author saw it — and why the
   > instrument is re-deriving the figure over *the named set*, never checking whether the numbers look
   > plausible. Both sets are quoted beside their ranges on **#13592**.

   This design uses the measured range for the set it names and does not rest on either quoted one.
2. **The self-produced share is not stable, and 17 is already stale.** Re-issuing #13599's input
   verbatim against the live graph on 2026-09-11 returns **19 of the top 20** as `processor-run` records,
   not 17 — the three runs taken later that evening joined the band. **Any figure of this kind is a
   property of a graph at a moment**, and this design treats it as a moving number rather than a constant.

Everything else in the brief verified exactly: 20/17/0/3 and 20/0/9/11 above, `fuse`'s missing predicate,
`assemble.go:52-53` as the only production read, #13600's own-predecessor arrival at rank 2 / 0.7500 /
100,350 B, and the reproducibility of retrieval.

### 2.4 The record corpus, and the shape of the band

Measured on the live graph, 2026-09-11. **Published as run** (MCP `divoid_list`, then `divoid_search`):

| measurement | call | result |
|---|---|---|
| run records in the graph | `divoid_list(type=["session-log"], name=["processor-run%"], count=1)` | **`total: 37`** |
| all `session-log` nodes | `divoid_list(type=["session-log"], count=1)` | **`total: 963`** |
| nodes in the graph | any `divoid_search` | **`total: 11140`** |
| subject #10422's direct neighbours | `divoid_list(linkedto=[10422], count=1)` | **`total: 57`** |
| …of which run records | `divoid_list(linkedto=[10422], type=["session-log"], name=["processor-run%"], count=1)` | **`total: 19`** |

And the band itself, ranked by the two verbatim inputs:

| probe | call | records in top 20 | in ranks 21–50 | 20th non-record row |
|---|---|---|---|---|
| ill-matched | `divoid_search(query=<#13599's input verbatim>, count=60)` | **19** | **6** | **rank 44** |
| well-matched | `divoid_search(query=<#13598's input verbatim>, count=45)` | **2** | **0** | rank 22 |

> **A paraphrase is a different measurement.** A first pass used a summary label of #13598's input
> (*"the Go comment rules, and where they are written down"*) and returned a materially different set —
> the top hit at 0.7607 against the run's own 0.7653, and the run's predecessor at rank 10 rather than
> rank 2. The published probes use the inputs read out of the records' `input` field.

**Three properties the band has, and all three matter:**

- **It is top-heavy and short.** On the ill-matched arm, 19 of 20 records sit in the first 20 ranks and
  only 6 more appear in the next 30. Beyond the block, density falls to ~20 %.
- **It is bounded by the corpus, not by the fetch.** Only 37 records exist. Fetching deeper cannot pull
  more than 37 of them, and their bytes are therefore a bounded, not a scaling, cost.
- **The corpus grows by one record per run and by nothing else.** 2026-09-10 alone produced roughly
  fifteen. **19 of subject #10422's 57 direct neighbours are already run records**, and that ratio rises
  monotonically because a run adds a neighbour and never removes one. The scope reserve's own pool is
  therefore the most exposed surface, not the least.

### 2.5 The supplementary aperture has the same defect, and it is observed

`dispatchRecall` (`internal/loop/turn.go:353`) calls `Recall(query, CandidateLimit, nil)` directly and
hands the result to `admit` (`:359`). It does not go through `fuse`. **On #13598, two of four
supplementary recall rounds returned a self-produced row each**, cut at admission having spent a slot of
that round's twenty. Structural, and measured.

---

## 3. Scope and Non-Scope

**In scope.** How many rows `Retrieve` asks the graph for; which of them may become candidates; where the
self-produced predicate is applied; what the run record consequently stops carrying; and the two-armed
measurement that says whether the change worked.

**Out of scope, explicitly:**

1. **`admit`, the byte budget, the block layout, `MaxModelCalls`, the query set.** Not one line of
   `assemble.go` changes, and the only change in `turn.go` is the added WARN in its run-summary block
   (§12, §14). §16 states the interactions with **#13585** (PR #67), which
   changes the query set, and with **#13594**.
2. **#13594 — the oversize top-ranked candidate, and the block never disclosing what was cut.** Separate
   task, separate PR. Interaction in §16.2; this document states it and stops.
3. **What a run record should be**, given it is a first-class competitor in its own graph. Named in §19
   and filed, per the brief; **not designed here**.
4. **The supplementary aperture** (§2.5). It does not pass through `fuse`, so this change does not reach
   it, and reaching it means either a second predicate site or routing it through `Retrieve` — which
   **#11263** ruled against. Named in §19 and filed.
5. **A similarity floor, an exhaustion outcome, an admission-policy extraction.** All belong to
   `docs/architecture/retrieval-admission-and-the-empty-outcome.md`, which is **designed and not
   implemented** and demoted by #13534 §5. Interaction in §16.3.
6. **Retrieval or admission tuning of any other kind**, per #13534 §5.

---

## 4. The ruling this design must not violate, and the reason it does not

Toni, holding PR #48 in draft:

> *"Memory is always worth something — self produced is not less worth than memory of other agents,
> should that be the case then its not memory but just noise and shouldn't even be in DiVoid. So its
> about what is the shape of the memory and does it actually contain substantial information."*

**This design proposes no judgement about run records at all.** That is not a rhetorical defence; it is
the mechanism:

> The predicate applied at `fuse` is **inherited from `admit`**, which already rejects a self-produced row
> unconditionally and before any test of its content. The run's decision about whether such a row may be
> read is already made, is unchanged by this design, and is not re-argued here. What changes is only
> **where the already-made decision is applied**, and therefore whether a row that can never be used gets
> to displace one that can.

Two consequences of taking that seriously, and both are stated rather than smuggled:

- **If run records are memory worth reading, the defect is that `admit` refuses them** — and that is a
  different design, about the record's shape and about degraded admission. It is named in §19 and filed.
  This document does not pre-empt it, and it does not depend on its outcome: whichever way it lands, an
  aperture that spends slots on rows the *current* rule refuses is wrong.
- **The two predicates must move together.** If `admit` later stops refusing, `fuse` must stop excluding
  in the same change, or the aperture silently keeps excluding rows the block would now accept. §14 G-4
  is the guard for exactly that, and its premise is that nothing else couples the two sites.

**What the measured evidence actually indicts is size and volume, not authorship.** Seventeen objects
with a median of 66,804 B, arriving in an unbroken block at ranks 1–17. A single 66 KB record at rank 1
would cost one slot and nobody would have filed a task.

---

## 5. Is the aperture the right place to act at all?

The brief asks this rather than assuming it, and the answer is yes — but the instrument that is wrong is
not the one the question names.

**The reframe, and it is this design's central claim:**

> The candidate limit is fine. What is wrong is that **the graph is asked for exactly as many rows as the
> aperture holds**, so every row the loop discards between fetch and admission shrinks the aperture below
> its own constant. `count` and `limit` are two different quantities that happen to share a number, and
> the sharing is what makes any downstream filter unable to recover a slot.

That distinction decides between the three shapes #13593 sketched, and it is why filtering alone is not
enough:

| shape | what it does on #13599's measured arm |
|---|---|
| **1. Filter at fusion, fetch unchanged** | The unscoped list *itself* was records at ranks 1–17. Filtering recovers unscoped ranks 18–19 and whatever the scoped list holds — roughly **six** rows, not twenty. It makes #13585 §17 row 3's sentence true while leaving the property that sentence was relied on for still false. |
| **2. Over-fetch and filter** | Recovers a full aperture, because the band is bounded and short (§2.4). Costs bytes for rows that are discarded. |
| **3. Exclude at the graph query** | **Not expressible.** See below. |

### 5.1 Shape 3 is dead on measurement, not on argument

`GET /api/nodes` takes `type` as an **inclusion** list (`type | string[] | Filter by type name(s)`,
DiVoid #8). There is no negation, and the `path` grammar's keys (`id`, `type`, `name`, `status`) carry
alternation but no negation either. The predicate this needs is
`type == "session-log" AND name LIKE "processor-run%"`, negated — no combination of the documented
parameters expresses it.

The nearest approximation is excluding the `session-log` type. Measured: **963 `session-log` nodes exist
and 37 of them are run records.** That approximation discards **926 legitimate memory nodes written by
other agents to remove 37** — and it is the same over-broad move `m1-skeleton-loop.md` R13 already ruled
against on this exact ground, after human-written session logs came back ranks 2 and 3 in a failing run.
It is also the one shape that would deserve the bounce §4 describes, because *type* is a proxy for
authorship in a way the real predicate is not.

**What would make shape 3 available** is a negation predicate on the graph's listing route. That is a
DiVoid-side change in another repo, it is the only shape whose cost does not grow with the record corpus,
and it is filed in §19.

### 5.2 The byte-denominated aperture, considered and rejected

The brief offers the frame that *"20 rows of unbounded size is a strange aperture for a byte-budgeted
block."* It is a fair observation and the remedy does not follow from it. Rejected on three grounds:

- **It would not have helped the measured case.** On #13599 the byte budget cut nothing. A retrieval
  stage that stopped fetching once it had budget-worth of bytes would have stopped after **one** 71 KB
  record and delivered a *worse* aperture, not a better one.
- **It duplicates a ceiling that already has one owner and five copies too many.** `Retrieve` would have
  to know `AssemblyByteBudget` and the anchor's size — the precise arithmetic that design **#13522**
  exists to stop being recomputed, after a stem-keyed sweep found it at **six sites: one origin and five
  restatements**, three of them Go (#13522's own substance; the sites are recorded on #11340, #13507
  and #10995).
- **It is the wrong unit for the actual failure.** The rows lost here are lost because they are
  *ineligible*, not because they are *large*. Size is #13594's subject.

---

## 6. Assumptions and Constraints

| # | assumption | status |
|---|---|---|
| A1 | `admit` continues to reject self-produced rows unconditionally and first | true at `0df5c14`; **coupled by G-4**, not assumed |
| A2 | The graph's semantic ranking is reproducible run to run | measured — #13598/#13600 shared 19 of 20 candidates and the same top hit to four decimals (#13592). **This is the premise the whole two-armed acceptance rests on**; without it a before/after delta is not attributable |
| A3 | `Recall`'s `count` accepts values well above 20 | documented cap is **500** (DiVoid #8); this design uses 100 |
| A4 | A record's `SelfProduced` flag is set from type **and** name prefix, so a foreign `session-log` is never excluded | `internal/divoid/write.go:33-34`; pinned by the existing `TestSweepCountsOnlyRunRecordsAsSelfProducedAndNotOtherSessionLogs` |
| A5 | The graph holds far more than 100 nodes, so an unscoped recall at `count=100` always returns 100 rows | **11,140** when this was written, **11,146** when QA re-measured hours later, **11,148** on re-check. **The figure moves; the premise does not** — and the drift is the same mechanism §18's baseline rule is about. This is what makes §14's exhaustion detector exact |
| A6 | The record corpus grows by one node per run and shrinks never | `internal/divoid/write.go:87`; no deletion path exists |

**Constraint that is a ruling, not a preference:** §4. **Constraint that is a contract:** #114 §4 via
#10861 — the implementation adds no fencing banner and no multi-line body comment; #1380 makes that
bouncing-grade.

---

## 7. Architectural Overview

```
                     TODAY                                        AFTER
  +-----------------------------------+        +-----------------------------------+
  | Recall(query, count = 20)         |        | Recall(query, count = 20 x 5)     |
  |   20 rows, bodies included        |        |   100 rows, bodies included       |
  +---------------+-------------------+        +---------------+-------------------+
                  v                                            v
  +-----------------------------------+        +-----------------------------------+
  | fuse: RRF, reserve, backfill      |        | fuse: RRF, reserve, backfill      |
  |   appendUnseen rejects:           |        |   appendUnseen rejects:           |
  |     - the anchor                  |        |     - the anchor                  |
  |                                   |        |     - a SELF-PRODUCED row  <--+   |
  |   caps at limit = 20              |        |   caps at limit = 20              |
  +---------------+-------------------+        +---------------+-------------------+
                  v                                            v
        20 candidates, of which                       20 candidates, every one of
        up to 17 measured unusable                    which admission may consider
                  v                                            v
  +-----------------------------------+        +-----------------------------------+
  | admit: self-produced -> cut       |        | admit: UNCHANGED                  |
  |        bytes -> admit or cut      |        |   its self-produced arm stays     |
  +-----------------------------------+        |   live via dispatchRecall (2.5)   |
                                               +-----------------------------------+
```

**The whole change is two edits in one file, plus one WARN in a second.** No new type, no new function,
no new exported name, no new record field, no new configuration. `assemble.go`, `admit`, `Assemble`,
`renderBlock`, the block layout, the budget, `GraphPort`, `internal/divoid`, `cmd/eval` and both adapters
are unchanged; `turn.go` gains one guarded log line in its run-summary block and nothing else.

### 7.1 Why the exclusion belongs in `appendUnseen` specifically

`fuse` has three fill passes — fused to `limit - reserve`, up to `reserve` unseen scoped rows, then
backfill — and **all three admit through one `appendUnseen` closure**. That is not incidental: it is the
mechanism PR #39 chose so the anchor exclusion would cover all three paths with no per-pass condition and
no branch a later pass could be added around (#11259). A self-produced row needs exactly the same
coverage for exactly the same reason.

The reserve pass increments `reserved` only when `appendUnseen` returns true, so a rejected scoped row
**does not spend a reserved slot** — correct by construction rather than by a second condition.

### 7.2 The anchor is the precedent, and it is exact

The project has already ruled, once, on where a structurally-inadmissible row is excluded. The anchor is
**not** excluded in `admit`; it is excluded in `fuse`, deliberately, and #10849 records why: `admit` is
also called by the supplementary path where no exclusion has happened, so putting it there would make one
of the two sites redundant and the redundancy invisible.

**The anchor and a self-produced row are the same class** — unconditionally excluded, for reasons
independent of the budget, knowable before ranking. One is excluded at `fuse` and the other at `admit`,
and that inconsistency is the defect. This design does not introduce a boundary; it finishes one.

It also inherits the precedent's accepted cost, and §15 states it in the same terms #11259 used.

---

## 8. Components and Responsibilities

| Component | Owns, after this change | Does **not** own |
|---|---|---|
| `Retrieve` (`internal/loop/retrieve.go`) | How many rows to ask the graph for; forming the candidate set; **eligibility** — which rows may be candidates at all (the anchor, and rows admission refuses unconditionally) | Any budgeted decision. It does not know `AssemblyByteBudget`, the anchor's size, or a similarity floor |
| `fuse` | Ranking, the reserve, the cap, and the single `appendUnseen` gate all three passes share | Fetching. Rendering. Deciding what fits |
| `admit` (`internal/loop/assemble.go`) | **Admission** — what fits the budget — and the self-produced rejection on the path that still reaches it | The aperture. It is handed whatever it is handed |
| `internal/divoid`'s `Recall` | Issuing the query at the count it is given, and classifying each row as self-produced or not. **It still drops nothing — it marks** | Deciding how many rows are enough |
| The run record | Every candidate the aperture offered admission, admitted or cut | Rows that were never eligible. It does not carry the anchor today, and it will not carry excluded records (§15) |

**The vocabulary distinction this design makes explicit, and it is the naming half of the fix**
(#1220 §2's real-structures rule, whose reference is #6836): *eligibility* is whether a row may be a
candidate; *admission* is whether a candidate fits. The
self-produced rule is an eligibility rule that has been filed under admission, and every symptom in §2 is
downstream of that misfiling.

---

## 9. Interactions and Data Flow

One turn, unchanged in shape, with the two numbers separated:

1. `Run` calls `Retrieve(ctx, graph, anchor, queries, CandidateLimit, RecallScopeReserve)` — **the call
   site is byte-identical**; `Retrieve` derives the fetch count from `limit` internally, so no caller
   passes a new argument and `cmd/eval/sweep.go` inherits the behaviour unchanged. The only edit to
   `turn.go` is the WARN, far downstream in the run-summary block.
2. `Retrieve` computes `fetch = limit * recallOverfetchFactor` and issues one unscoped `Recall(query, fetch, nil)`
   per query, then builds the scope, then one `Recall(queries[0], fetch, scope)`. **The over-fetch applies
   to the scoped recall too** — §2.4 measures the scope seed as the most record-dense pool available, and
   the reserve is only three slots wide.
3. `sourcesOf` walks the same, longer lists. A row that `fuse` excludes still gets `Source` entries built
   for it; they are simply never attached, because `attribute` looks up by the ids in `out`. No change.
4. `fuse` ranks by reciprocal rank over the full lists, then fills through `appendUnseen`, which now
   rejects the anchor and any self-produced row. Returns at most `limit`.
5. `Assemble` and `admit` receive a set of the same maximum size as today, every member of which admission
   may actually consider.

**Graph call count is unchanged at `1 + N + 1`.** This design adds no round trip; it widens the ones that
already happen.

---

## 10. Data Model (Conceptual)

**No change.** `Candidate`, `Disposition`, `Record` and `Limits` are untouched.

`Limits` was the one candidate for a new member — a `recallFetchCount` recording which fetch a stored
record ran under, for cross-run comparability. **Rejected on the delete test (#1136 §4) with the math:**
the value is carried by **two mirror structs** (`loop.Limits` and `internal/eval`'s own `Limits` at
`internal/eval/result.go:11`) and rendered at two more sites (`internal/loop/summary.go:65`,
`internal/eval/report.go:65`), so it is **1 int x 5 sites** for a value no decision reads — the tuning
decision it would inform is answered instead, and earlier, by §14's exhaustion detector, which costs
nothing. #1136 §3's telemetry-then-tune compound is the shape this avoids.

---

## 11. Contracts and Interfaces (Abstract)

| Contract | Before | After |
|---|---|---|
| `Retrieve` signature | `(ctx, graph, anchor, queries, limit, reserve) ([]Candidate, error)` | **unchanged** |
| `Retrieve` semantics | fuses one unscoped recall per query with one scoped recall, returns at most `limit`, never the anchor | …**and never a row this system wrote**, and asks the graph for more rows than `limit` so the survivors can still fill it |
| `GraphPort.Recall` | returns up to `count` rows in the graph's rank order, marking each self-produced or not; drops nothing | **unchanged in every respect** |
| `admit` | cuts self-produced first, then applies the byte budget | **unchanged in code**; its self-produced arm is reached only from `dispatchRecall` after this lands |
| The run record's `candidates[]` | every row `fuse` returned, admitted or cut, including self-produced cuts | every row `fuse` returned — which no longer includes self-produced rows, exactly as it has never included the anchor |

**Invariant this design adds, and G-4 is its guard:** *the set of rows `fuse` excludes as self-produced is
exactly the set `admit` would cut as self-produced.* Two sites, one rule; nothing in the compiler or in a
diff couples them.

**Invariant it preserves:** `queries[0]` remains the scoped recall's query — the reserve still ranks the
caller's own words inside the neighbourhood.

---

## 12. Cross-Cutting Concerns

- **Determinism.** `Retrieve` gains no clock, no randomness and no state. The turn stays a computable
  function of (subject, queries, graph rows, constants).
- **Errors.** No new failure mode. A recall at `count=100` fails the same way a recall at `count=20`
  does, and `Retrieve` already returns the first error unchanged.
- **Memory.** The lists held simultaneously grow. Worst measured arm: candidate bodies were 1,085,093 B
  for 20 rows; at 100 rows the record share saturates at the whole corpus (37 x ~65 KB, about 2.4 MB)
  plus ~63 real rows. With #13585's six-query fan-out this is held across seven lists. **Bounded,
  quantified in §14 R-2, and not measured against a wall-clock figure that does not exist.**
- **Comments.** #114 §4 via #10861, whose table has a row for each identifier class rather than one
  length cap. **Doc comments on unexported identifiers: *"Default none, same as any other code."*** So
  the new constant gets **no** doc comment — its **name** carries the meaning, which is why §18 step 1
  specifies a name that reads as a multiplier. Measured in `internal/loop` at `0df5c14`: **0 of the
  package's unexported constants carry one**, `fusionRankConstant` included. Exported godoc is a
  different row (*one tight line*), and §18 step 4 gives `Retrieve`'s the length bound **#13607 W-4** asked for.
  #1380 makes a violation bouncing-grade.
- **Observability.** **One WARN is added**, beside the existing shutout WARN at `internal/loop/turn.go:189`
  and guarded by the same shape: `len(record.Candidates) < record.Limits.CandidateLimit`. It is the
  emitter for §14's exhaustion detector and therefore the named detector for §13.2's universal — **not an
  operator-facing disclosure feature**, and §16.2 states the distinction so the next reader does not read
  it as a duplicate of #13594 and delete one of them. Nothing else is logged that is not logged today.

---

## 13. Quality Attributes and Trade-offs

### 13.1 What is bought, and where the aperture stops improving

The filter is the correctness half; the over-fetch is an amplifier. **The two degrade independently and
gracefully**, which is the property that makes a fixed headroom safe:

> At **any** over-fetch factor, including none at all, the aperture is strictly better than today: a
> shorter but clean set of usable rows beats a full set of which most are dead. The factor decides *how
> full*, never *whether correct*.

That is why §14's exhaustion condition is a tuning signal and not an incident.

### 13.2 The over-fetch factor, and the honest statement of its lifetime

**`recallOverfetchFactor = 5`, so `fetch = 100`.** Basis, all measured in §2.4:

- On the worst arm available, the 20th non-record row sits at **rank 44**. `fetch = 100` clears it by 56
  ranks.
- **37 records exist in total**, so at `fetch = 100` the headroom (80) exceeds the entire corpus: today
  the aperture cannot be starved by this cause on *any* input.
- On the well-matched arm the extra 80 rows are never reached by `fuse`, so **the over-fetch** changes
  nothing there (§14 R-1). **The filter does**, and by a measured amount: that input's top 20 holds
  **2** run records today (§2.4), so the aperture loses two ineligible rows and gains two deeper
  eligible ones. **The arm is a control on regression, not on identity** — §18 states what it asserts
  instead, and why identity was the wrong thing to ask for.

**And the claim's falsifier, stated because it is a universal (#1220 §5):** *an input for which more than
80 of the graph's top 100 rows are `processor-run` records.* That requires **at least 81 records**, i.e.
**44 more than exist.**

**How long that is, measured rather than estimated.** An earlier revision said *"roughly fifteen runs on
2026-09-10 alone — that is weeks, not years"*. **Both halves were wrong**, and it was the one graph figure
in this document published without its command. **Published as run:**

```
divoid_list(type=["session-log"], name=["processor-run%"], count=50, fields=["id","name","created"])
```

`total: 37`, all 37 rows returned, counted by `created` date:

| date | run records |
|---|---|
| 2026-09-02 | 2 |
| 2026-09-05 | 3 |
| 2026-09-06 | 3 |
| 2026-09-07 | 9 |
| **2026-09-10** | **20** |

| basis | rate | time to 44 more |
|---|---|---|
| the measured peak day — **2026-09-10, 20 runs, not fifteen** | 20/day | **2.2 days** |
| mean over the **5 days that had any runs** | 7.4/day | **5.9 days** |
| mean over the **9-day calendar span** | 4.1/day | **10.7 days** |

**So the honest range is 2.2 to 10.7 days, and no basis yields "weeks".** The peak is the one a safety
margin is sized against, because runs arrive in bursts — **four of the nine days carry none**, and one
day carries more than half the corpus.

**A five-fold overstatement of a bridge's remaining life is the figure that gets quoted into a follow-up
brief**, and the direction being safe for the decision is not a mitigation when the section is titled
*the honest statement of its lifetime.* **It also strengthens the case for the WARN rather than weakening
it:** a bridge with days of life needs its expiry detector more than one with weeks.

**A claim with an expiry needs something that fires when it expires.** Left alone, the multiplier can
become insufficient and the only symptom is a quieter aperture that still looks like a full run. The
detector below, and the WARN that emits it, are how this bridge announces that it has expired instead of
degrading invisibly. **That is why the detector is part of this design rather than a later nicety.**

**So this is a bridge and the document says so rather than implying permanence.** Three things end the
trade, all filed in §19: a negation predicate on the graph's listing route (the only shape whose cost
does not grow with the corpus), a change to what a run record *is*, or a two-phase fetch. None of them is
this task.

### 13.3 Rejected alternatives

| # | Alternative | Why not |
|---|---|---|
| **R-A** | **Filter at `fuse`, leave the fetch at `limit`** — #13593's shape 1, and what #13585 §17 row 3 believed already happened | Measured insufficient: on #13599 the unscoped list *was* records at ranks 1–17, so this recovers roughly six rows of twenty. It would make the *sentence* true and leave the *property* false — the exact failure mode the corrected §17 row 3 was written to end. **§14 G-2 detects it**, which is why that guard asserts the issued count rather than the returned set |
| **R-B** | **Exclude at the graph query** — #13593's shape 3 | Not expressible (§5.1). The nearest approximation drops **926 legitimate nodes to remove 37**, and it is authorship-by-proxy, which §4 bars |
| **R-C** | **Keep the excluded rows in the returned slice, uncounted against `limit`**, so `admit` still writes their `cutReason` and every existing diagnostic keeps working | **The strongest alternative, and it was close.** It preserves the record's completeness, `internal/eval`'s `SelfProducedCandidates`, and the prose in `compare.py` / `step_trace.py`. Rejected because it makes `Retrieve` return a slice with two kinds of element distinguished only by a flag the consumer must know to check — a compromise shape (#1136 §4) that preserves the very naming error that produced the bug: *a row admission can never use is not a candidate.* The anchor settled this question six days ago and is not carried in `candidates[]` either. Two-kinds-in-one-slice is also how the aperture came to be denominated in the wrong unit in the first place |
| **R-D** | **Record the number of rows excluded**, as a leading indicator of headroom pressure | Deleted on the delete test. §14's exhaustion detector answers the only decision that reads it, needs no field, and is computable from every record already stored. #1136 §3's telemetry-then-tune compound |
| **R-E** | **Adaptive re-fetch** — fetch `limit`, count the records, re-issue wider if short | Converges only by luck in one step, because the widened band is also crowded; a loop needs a bound and the bound is the same magic number, chosen worse. It adds a round trip exactly on the crowded runs, which are the slow ones |
| **R-F** | **Two-phase fetch** — rank on a body-less projection at a large count, filter, then hydrate exactly `limit` rows by id | **The right answer to a different problem**, and the successor named in §13.2. `IsRunRecord` needs only `type` and `name`, both of which a cheap projection carries, so headroom would become nearly free and today's wire cost would *fall*. Rejected now on three grounds: it makes `Candidate` a two-state object that is a footgun in `admit` (an unhydrated row has `len(Content) == 0` and is admitted for free); it adds a port method and a round trip; and **the cost it optimises is unmeasured** — no retrieval-latency figure exists for this system, and #13534 §3 is a standing ruling against optimising a quantity whose input has not been measured. Filed in §19 with its trigger |
| **R-G** | **A byte-denominated aperture** | §5.2 |
| **R-H** | **Pre-filter the lists before `fuseByReciprocalRank`** rather than at `appendUnseen` | Removing rows before scoring compacts every surviving row's rank, which changes RRF scores. #13585 §17 row 5 already records that every retrieval constant is untested under live fusion; adding a second uncontrolled ranking variable in the same window is how a measurement stops being attributable. Filtering at selection changes *which rows are taken* and nothing about *how they are ranked*. **This prohibition has its own guard, G-8** — an earlier revision stated it with nothing that fires on its violation, which is the shape §13.2 itself argues against |

### 13.4 KISS accounting (#1136 §4)

| element | can it be deleted? | can it be merged? | can it be inlined? |
|---|---|---|---|
| the `appendUnseen` clause | no — it is the fix | it *is* the merge: it joins the anchor exclusion already there | it is one clause |
| `recallOverfetchFactor` | no — without it the aperture under-delivers (R-A) | no | it is one const; #1136 §3 keeps it a `const`, not config: no operator tunes it, no environment differs, it is not a secret |
| the WARN (§14) | **no** — it is the detector §13.2's universal needs, and the *only* signal in the empty-aperture case, where the existing shutout WARN's `> 0` guard keeps it silent | it **is** the merge: one more guarded line in the run-summary block that already holds the shutout WARN, not a new observability surface | it is one guarded line |
| a `RetrievalStats` return | **yes** — deleted | — | — |
| a `Limits.recallFetchCount` | **yes** — deleted, 1 int x 5 sites | — | — |
| an `inadmissible(c)` helper shared by `fuse` and `admit` | **yes** — deleted. 1 line x 2 sites = **2**, far below #1267's ~15–20 threshold. The coupling risk it would address is closed by **G-4**, a guard, rather than by an indirection | — | — |

---

## 14. Risks and Falsifiers

> **No mutation below has been executed.** This document's author ran no Go. Every falsifier is therefore
> stated as a **prediction and an obligation on the unit**, per #1220 §5's rule that a cell may only
> report an observed output someone has quoted. **The unit is not done until each has been run and its
> observed output recorded in the PR.** Where a claim rests on a measurement that *was* taken, §2 quotes
> the command.

| # | Risk | Mitigation | Falsifier — **predicted, not yet observed** |
|---|---|---|---|
| **R-1** | **The over-fetch reorders the well-matched arm.** If widening the fetch changed which rows emerge on inputs that already work, the change would trade one regression for another. **Scoped to the over-fetch alone** — the filter moves that set by design, and an earlier revision conflated the two, which is what made this row's falsifier unsatisfiable | Read out of `fuseByReciprocalRank` (`internal/loop/retrieve.go:120-141`): each id's score depends only on its own rank in each list, and `SortStableFunc` places new low-scoring ids below. **With a single query the top prefix is provably unchanged.** G-5 pins it as a dual-arm relation over the *fetch width only*, holding the filter constant | **G-5's two arms must return equal ids.** A differing set falsifies *this* claim. It is **not** falsified by the whole change moving the candidate set — that is the filter working, and §18's relations are what test it |
| **R-2** | **Bytes and memory grow.** 1,085,093 B of candidate bodies measured on one recall today; ~2.8 MB predicted at `fetch=100`, held across `N+1` lists | Bounded by the corpus, not the fetch (§2.4). Quantified rather than hidden. **Not** optimised, because no latency figure exists (#13534 §3) | A measured turn wall-clock regression attributable to retrieval. **None exists today in either direction** — stated as unmeasured, not as safe |
| **R-3** | **With #13585's fan-out the fused order *can* change.** With N>1 a deeper list adds cross-query agreement a shallower one did not carry, so R-1's proof does **not** extend past a single query | Stated rather than claimed away. It is plausibly an improvement — more agreement evidence — but it is a behaviour change and must not be reported as neutral | G-5 is explicitly scoped to N=1 and says so. If a future arm asserts neutrality at N>1, that assertion is false by construction |
| **R-4** | **The predicates drift.** `fuse` and `admit` each read `c.SelfProduced`; if one changes, the other is silently wrong in whichever direction | **G-4** | **Named, and it is not reachable from this change.** Both sites read the same boolean, so G-4 holds by construction for **every** implementation this design can produce — a green here is not a result (#11034 P-27; #10466's *"a guard you have not seen fail is decoration"*). The mutation that establishes it is **alter `admit`'s own predicate** — e.g. make it admit a self-produced row below some size — **and assert G-4 reddens.** That mutation is outside this design's diff and is the one #13602 will actually make, which is the whole point of the tripwire. **Run it as a throwaway mutation in this round** so the guard is witnessed once rather than trusted forever |
| **R-5** | **The headroom expires** (§13.2) | The detector below, and three filed successors | — |

### The exhaustion detector, and the premise that makes it exact

> **`len(record.candidates) < record.limits.candidateLimit`**

Computable from every record already stored, and **emitted as a WARN** from the run-summary block in
`Run`, beside the existing shutout WARN at `internal/loop/turn.go:189`. It fires exactly when the
headroom failed.

**The two WARNs are siblings and neither subsumes the other.** The shutout WARN is guarded by
`len(record.Candidates) > 0 && cut == len(record.Candidates)` — *we had candidates and admitted none.*
This one says *we could not fill the aperture.* Both can be true, and **in the degenerate case they come
apart in the direction that matters**: when every fetched row is self-produced the candidate set is
**empty** after this change, so the shutout WARN's `> 0` guard keeps it silent and this WARN is the
**only** signal that anything happened.

> **And that is the argument, which is stronger than the one an earlier revision made.** It read *"today
> it would pass in total silence"*, and **that is false.** Walked at `0df5c14`: `fuse` returns 20
> records, `admit` cuts all 20, and `turn.go:188`'s guard reads `len(record.Candidates) > 0 && cut ==
> len(record.Candidates)` — `20 > 0` and `20 == 20`, so **the shutout WARN fires today.** What Unit 1
> does is **remove an existing WARN-level signal** on that arm, by emptying the candidate set the guard
> tests. **G-7 restores it.** *"Nobody would notice today"* and *"this change would stop anyone
> noticing"* are different justifications and only the second is true — a negative claim about current
> behaviour, of exactly the class that has to be walked against the code rather than reasoned about.

**Not reached by `cmd/eval`**, which calls `Retrieve` directly and never enters `Run` — the sweep's
equivalent is `RowResult.CandidateCount` against `Limits.CandidateLimit`, already recorded
(`internal/eval/result.go:87`) and needing no change.

**Premise (A5):** an unscoped recall at `count=100` against an 11,140-node graph always returns 100 rows —
the route ranks the whole graph and applies no relevance floor. So a short candidate list can only mean
that more than `fetch - limit` of the fetched rows were excluded. **Without that premise the detector
would also fire on a small graph**, and the reading would be wrong rather than merely noisy.

### Guards, each naming the premise that makes it discriminate (#11034 P-42)

| # | Guard | Premise — why it discriminates |
|---|---|---|
| **G-1** | `TestARunRecordDoesNotSpendACandidateSlotItCanNeverUse` — a fused list whose top rows are self-produced and whose tail is real; assert the returned ids are exactly the real ones | Against `0df5c14` this fixture returns the records, so a no-op implementation reddens it. It is the only guard that fails today |
| **G-2** | `TestRetrieveAsksTheGraphForMoreRowsThanTheApertureHolds` — assert the recorded `Limit` is `limit * 5` on **every** recorded call, the scoped one included | **`fusionGraph.Recall` ignores its `limit` argument** and returns the whole fixture list, so **no assertion over the returned candidate set can see the fetch count**. Only the recorded call can. This is the guard that separates the full fix from R-A |
| **G-3** | `TestASelfProducedRowDoesNotSpendAReservedSlot` — scoped list `[record, real, real, real]`, `reserve = 3`; assert three real rows reserved | `reserved` increments only on `appendUnseen`'s true return. An implementation that filtered *after* the reserve loop would reserve two and pass every other guard |
| **G-4** | `TestFuseExcludesExactlyWhatAdmitWouldCutAsSelfProduced` — over one candidate set, assert the ids `fuse` drops equal the ids `admit` marks `cutReasonSelfProduced` | The two predicates live at two sites and **nothing else couples them** — not the compiler, not a diff, not any other test. It is the guard for R-4 and for §4's second consequence: when `admit` stops refusing, this test is what forces `fuse` to stop excluding in the same change |
| **G-5** | `TestTheFusedPrefixDoesNotMoveWhenOneQueryReturnsMoreRows` — dual-arm: the same 20-row list, and that list followed by 40 further rows; assert equal returned ids. **Single query only** | It asserts a **relation between two arms of one implementation**, so it cannot be satisfied by a hard-coded expectation. Scoped to N=1 deliberately: R-3 makes it false at N>1, and a guard that quietly claimed otherwise would fire on compliant code once #13585 lands |
| **G-6** | `TestTheApertureIsEmptyRatherThanBackfilledWhenEveryFetchedRowIsSelfProduced` — every fetched row is a record; assert an **empty** candidate set | Pins the graceful-degradation choice against the tempting defensive implementation that back-fills with records rather than return nothing. That implementation passes G-1 through G-5. **The name states what the guard pins and predicates nothing of the excluded rows** — an earlier revision called the alternative *polluted*, which is §4's forbidden judgement in the one channel that ships: under #10861 a Go test name is the sole carrier of intent, and #13602 would have had to rename it |
| **G-7** | `TestAShortApertureIsWarnedEvenWhenNothingWasCutBecauseNothingWasFetched` — a turn whose fetched rows are all self-produced; assert the WARN fires | **The existing shutout WARN cannot cover this case**, because its guard requires `len(record.Candidates) > 0` and the degenerate aperture is empty. A test that asserted "some warning fired" would pass against the pre-existing WARN on a *non*-empty shutout and never exercise the new one; the fixture has to be the empty-aperture arm specifically |
| **G-8** | `TestTheExclusionLeavesFusionRanksAsTheGraphReportedThemAcrossTwoQueries` — dual-arm over **two** lists: exclude at `appendUnseen` (correct) versus pre-filter the lists before scoring (R-H's shape); assert the returned order differs, and that the correct arm matches the graph's reported ranks | **This is R-H's detector, and G-5 cannot be it.** With **one** list, scores are strictly decreasing in rank, so removing rows preserves the survivors' order and the two arms return **identical** ids — R-H is behaviourally undetectable at N=1, which is why G-5 is scoped there and why an earlier revision left this prohibition with nothing that fires on it. With **two** lists a row's score is the sum over lists of `1/(10+rank+1)`, and pre-filtering compacts ranks **in one list only**, so scores rise by different amounts and the fused order can flip. **Worked fixture:** `p` at 0-based rank 3 of list A behind three excluded rows scores `1/14 = 0.0714`; `q` at rank 2 of list B scores `1/13 = 0.0769`, so `q` outranks `p`. Pre-filtering lifts `p` to rank 0 (`1/11 = 0.0909`) and **`p` overtakes `q`**. `fuse` takes `lists [][]Candidate`, so this is writable today — `cmd/eval/sweep.go` already exercises N>1 (#11259) — and it does **not** wait on #13585 |

**Falsifier for this table itself:** *any row whose named guard would still pass against an
implementation lacking the claimed property.* **Three rows exist only because an earlier row failed it:**
G-2, because a fake that ignores `limit` cannot see the fetch, so no assertion over G-1's returned set
reaches it; G-6, because none of G-1..G-5 sees the degenerate arm; and **G-8, because R-H was a
prohibition with nothing that fires on its violation** — found by running this falsifier over the
*prohibitions* rather than only over the guards, which is the pass that had not been run.

**And one row is honest about not being reachable from here:** **G-4** holds by construction for every
implementation this design can produce, so its own falsifier lives outside this diff and §14 R-4 names
the mutation rather than leaving the cell empty. A green there is not a result until that mutation has
been seen to redden it.

**Mechanical pre-submit check, to be run and its output recorded, not asserted:**

```
grep -rn "TestARunRecordDoesNotSpend\|TestRetrieveAsksTheGraph\|TestASelfProducedRowDoesNotSpend\|TestFuseExcludesExactly\|TestTheFusedPrefixDoesNotMove\|TestTheApertureIsEmptyRather\|TestAShortApertureIsWarned\|TestTheExclusionLeavesFusionRanks" internal/loop/
```

**Eight** names, eight hits, all in `internal/loop`. A name in this document's body is a claim; scope the
extraction to the body, because a name in a revision history is a record. **Re-run this after the
implementation round, not before** — a resolution claim is not durable across a rename the document itself
asked for.

---

## 15. What this silences, and it is stated in the same terms the anchor precedent was

PR #39 excluded the anchor at `fuse` and made `internal/eval`'s `AnchorWasCandidate` /
`AnchorAdmittedAsCandidate` structurally false for every sweep row — recorded on **#11259**, not
discovered later. This change owes the same accounting.

**Structurally silent after this lands:**

| what | where | why it is not dead |
|---|---|---|
| `RowResult.SelfProducedCandidates` | `internal/eval/result.go:33,102,161-162` | It reads `Disposition`s, and `dispatchRecall`'s path still produces self-produced dispositions (§2.5). `cmd/eval` never runs the model, so **for a sweep it is now zero by construction** — a property of the code, not a measurement of the graph |
| `"self-produced candidates  N across M rows"` | `internal/eval/report.go:210-244` | Same. It will print `0 across 0 rows` for every sweep |

**Not silenced:** `admit`'s `cutReasonSelfProduced` arm stays live and reachable via `dispatchRecall`.
**It must not be deleted as dead code** — the same reasoning #10849 records for why the anchor exclusion
was kept out of `admit`.

**The property that must become true throughout the repository**, stated as a property rather than as the
list of sections this author happened to find (#1220, 2026-09-10):

> **Self-produced rows are excluded from the candidate aperture before ranking. No artifact may state or
> assume that they consume candidate slots, that they appear as a cut in a stored `candidates[]` from an
> *initial* assembly, or that the effective candidate limit is reduced by them.** The statement remains
> true of the *supplementary* aperture, which this change does not reach. Sweep the whole tree for that
> property; the occurrences below are **known, not exhaustive**, and the diagnostic to run beside "is this
> still true?" is **"is this an enumeration, and has the set grown or shrunk?"**

| site | what it says today |
|---|---|
| `scripts/compare.py:1068-1069` | *"recall returns a fixed number of rows, so a self-produced row …"* — the displacement claim, now false of the initial round |
| `scripts/compare.py:825, 832, 861, 877` | cut-explanation prose naming `assemble.go`'s two reasons |
| `scripts/step_trace.py:218, 400` | the self-produced cut explanation |
| `docs/architecture/m1-skeleton-loop.md:684-690, 1677 (R13)` | *"run records still occupy retrieval slots out of the candidate limit"* — R13's surviving live risk, now closed for the initial aperture |
| `docs/architecture/anchor-grounded-recall.md:360 (R4), :482 (Q3), :993, :1344` | Q3 asked *"exclude from the ranking rather than cut at admission?"* and answered **"No, but it should be a task."** This design is that task's answer, and it reverses Q3 |
| `docs/architecture/m2-retrieval-eval.md:966` and `m3-derived-recall.md:448 (R4), :916` | the self-produced count as a live sweep and smoke diagnostic |
| `docs/architecture/retrieval-admission-and-the-empty-outcome.md:332-346` | its §7 table: *"Retrieve … Does not own: Any admission decision"* and the run-record row's *"including everything cut"* |
| **#13585 §17 row 3** | now records, correctly, that self-produced rows **do** consume the limit. That becomes stale in the opposite direction — §16.1 |
| **`scripts/compare_tasks.json:21`** | **a grading key whose expected answer is that "an answer that names the self-produced cut and slot displacement is RIGHT."** After this lands it grades a correct answer as wrong. Its own text predicted this: *"it will need a third [revision] if #11141 is ever corrected in place."* **Unit 2** |

---

## 16. Interaction with work in flight

### 16.1 #13585 — the query the graph is asked (PR #67, open, implementation not started)

**The substance is disjoint; the files are not, and an earlier revision of this section claimed they
were.** #13585 §16 Unit 1 edits `turn.go`, `derivations.go`, both adapters and `internal/eval`; it states
*"Not one line inside [`Retrieve`] changes"* and its Do-not list names `Retrieve`, `fuse` and `admit`.
**That is a bound on #13585's implementer, not on every unit**, and an earlier revision read it as the
latter — asserting *"nothing in this design reaches anything on #13585's Do-not list"*, which is **false
against this document's own TL;DR**: the two edits are in `Retrieve` and `fuse`, the first two items on
that list. It sat **one** line from the correctly-stated half of its own sentence — *"and nothing in #13585 reaches
`retrieve.go`"*, the next line at revision 2's L670. **Three** lines is the distance to the `turn.go` half
at L672, which is a different sentence and is where §16.1's later use of the figure is correct. An earlier
revision attached the right number to the wrong referent: **a locator is a measurement** (#11034 P-51), and
this one was checkable against `4a0daeb`, which still carries revision 2. What actually holds is the half that matters: **nothing in #13585 reaches
`retrieve.go`** — its six steps move `MergeQueries`, `DerivationPrompt`/`ParseDerivation`,
`ModelPort.Derive`, `DeriveQueries`, `Run` and the timeout, and its G-2 *cites* `retrieve.go:31` without
editing it (§16.1 point 4). **Neither unit changes anything the other depends on.**

**But both edit `turn.go`**, so whichever lands second rebases. The overlap is small and locatable:
#13585 adds its derivation call, its `Info`/`Warn` pair and a summary line; this design adds **one
guarded log line in `Run`'s run-summary block**, beside the existing shutout WARN at `turn.go:189`.
**Whether the two land in the same function is not decidable from #13585 as written** — its step 5
specifies *"one `Info` on success and one `Warn` … one summary line"* without saying where the summary
line goes, and if it lands in `logFinished`'s `attrs` slice (`turn.go:170-186`) the two changes edit the
same function rather than merely the same file. **A merge conflict is possible either way; a semantic
conflict is not**, and the consequence does not change with the answer.

Recorded rather than repaired silently, because the sentence it replaced was true when written and was
falsified by the ruling that added the WARN — **a correction round's own new prose is where this class of
defect lands** (#1220 §5, 2026-09-01). The repair was made by sweeping the bare term `turn.go` across the
whole document rather than the sections the change obviously reached.

> **Unverifiable claim, kept and labelled.** That sweep was reported as finding *"six sites, two obvious
> and four not."* **Revision 1 is not reachable from any ref** — the branch carries a single commit whose
> content is revision 2, and the node holds the current revision — so the pre-correction text cannot be
> re-read and **the count is unverified rather than disputed.** QA's own sweep of revision 2 returns 14
> lines / 15 occurrences, none asserting `turn.go` is unedited, which confirms the repair was complete
> without corroborating the count. Left in place with its status named, rather than defended or deleted.

> **And the same correction was owed one section over and was not made.** The `turn.go` half was repaired
> completely; the Do-not-list half, three lines above it, stayed inverted until QA found it. **The remedy
> went to the term the finding named rather than to the claim the finding was about** — #11034 **P-52**,
> and the second instance of this shape in two days (#13564 §17 records three more in one arc). The
> operational form P-52 prescribes is *write the row before the list*: the row here is **every statement
> asserting which files or symbols this change does not reach**, and a sweep built from that row reaches
> the Do-not-list sentence, which a sweep built from the term `turn.go` cannot.

Four semantic interactions, all one-directional:

1. **§17 row 3 becomes stale in the good direction.** It currently states the truth — self-produced rows
   consume the limit, and *"nothing here mitigates it"*. When this lands, something does. The amendment
   is a Unit 1 deliverable (§18) and it must state the **property**, not patch the row: after this
   change the row's subject survives only for the *supplementary* aperture and for the fetch-side
   headroom, both of which are named in §19.
2. **The fetch multiplies with the fan-out.** #13585 issues up to six unscoped recalls where there is one
   today. Graph calls go `1+N+1`; rows fetched go `(N+1) x 100`. §14 R-2's byte figure is per recall and
   must be multiplied, not re-derived.
3. **Sufficiency survives the fan-out; neutrality does not.** Each list is filtered independently, and
   the derived queries of an ill-matched input are themselves ill-matched, so every list carries the same
   band and RRF over filtered lists yields at least as many distinct usable ids. **But R-3: the fused
   *order* can change at N>1**, and G-5 is scoped to N=1 for that reason.
4. **#13585 G-2 cites `retrieve.go:31`** as the line carrying `queries[0]` into the scoped recall. This
   change inserts a `fetch` computation above it, **so that citation's line number moves**. Re-resolving
   it belongs to whichever of the two lands second; it is named here so neither assumes.

### 16.2 #13594 — the oversize candidate, and the block that does not disclose

**This change makes #13594's part B smaller, not larger.** Part B's subject is *"whenever any candidate is
dropped — on size or as self-produced — the model is not told."* After this, a self-produced row is not a
dropped candidate; it was never a candidate, exactly as the anchor is not. **So part B narrows to
candidates cut on size, which is its actual subject**, and one of its two cut classes disappears rather
than needing disclosure.

> **The class narrows and the volume rises, and a #13594 owner needs both numbers here rather than one
> section away.** On #13599's arm, predicted from the measured figures in §18:
>
> | | before (measured) | after (predicted) |
> |---|---|---|
> | candidates | 20 | 20 |
> | cut as **self-produced** | **17** | **0** |
> | cut **on size** | **0** | **~12** |
> | admitted | **3** | **~8** |
> | total unusable | **17** | **12** |
>
> **It is a relocation plus a net gain, not a new cost.** The twelve rows now cut on size were **never
> fetched before** — they enter the aperture only because the over-fetch put them there; **no
> previously-admitted row is lost to the byte budget** (§18's condition: the replacements land ahead of
> the admitted set on this arm, and the admitted set grows anyway); admitted rises **3 → ~8**; and total
> unusable **falls 17 → 12**. **But part B's disclosure volume goes from zero size-cuts to about twelve**,
> on the arm where the operator has least other signal — which is the number that sizes the work, and it
> did not exist before this change.

The two changes are independent in code and can land in either order.

### The WARN is not #13594's part B, and the distinction is the reason both exist

An earlier revision of this design left the exhaustion signal to #13594 on the grounds that operator
disclosure is #13594's. **The boundary is real and this falls on the other side of it.** The operator
ruled it, and the reason is worth stating so the next reader does not read the two as duplicates and
delete one:

| | #13594 part B | the WARN in §14 |
|---|---|---|
| **what it is about** | **the answer** — *"what you are reading was assembled without the highest-ranked node"* | **this design's own mechanism** — *"the over-fetch multiplier was not enough"* |
| **the rows it concerns** | rows that were **fetched and cut** | rows that were **never fetched** |
| **what the reader does with it** | decides whether to trust a result | decides whether to raise a constant |
| **who else would notice** | — | **nobody.** #13594 cannot see it, because there is no disposition for a row that never arrived |

**So it has no other owner**, and it is the detector §13.2's universal would otherwise lack. It is
deliberately **not** operator-facing disclosure, it says nothing about the answer, and it must not be
folded into whatever #13594 builds.

### 16.3 `retrieval-admission-and-the-empty-outcome.md` — designed, not implemented, demoted

Its §7 component table draws the boundary *"`Retrieve` … does not own any admission decision"*, and its
ordered admission policy lists self-produced as rule 1 alongside a floor, an oversize rule and the budget.
**This design moves that one rule across that boundary, deliberately**, on the ground set out in §8: it is
an *eligibility* rule, not an admission one, and the anchor — which that same table already assigns to
`Retrieve` — is its exact structural twin. The remaining five rules are untouched and stay where that
design puts them.

That document also notes its floor must be applied *after* the self-produced cut, because run records
score 0.6750–0.7359 and would pass a 0.70 floor. **After this change that ordering constraint is moot for
the initial assembly** — no record reaches the floor — and it remains live for `dispatchRecall`.

### 16.4 #13522 — the ceiling that governed admission (PR #63 merged docs-only at `0df5c14`)

No interaction. This design introduces no second copy of any ceiling and reads neither
`AssemblyByteBudget` nor the anchor's size in `retrieve.go`. §5.2 rejects the one shape that would have.

---

## 17. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**
- No new type. No new abstraction. No new exported name. §13.4 records four elements considered and
  deleted.
- No element justified by "we might need X later"; §13.2 states the bridge's lifetime rather than
  implying permanence, and every successor is filed (§19) rather than built.
- No deprecation period, flag, or shim.
- DRY math quoted for both inline-vs-extract calls: the shared predicate helper at **1 line x 2 sites = 2**
  (below #1267's threshold; the coupling risk is closed by G-4, a guard, not an indirection), and the
  rejected `Limits` field at **1 int x 5 sites**.

**Existing systems first**
- The exclusion goes into `appendUnseen`, the closure that already carries the anchor exclusion for
  exactly this purpose (§7.1). No new mechanism.
- No new persisted data point (§10). The one considered is deleted with its math.
- No field justified by "an existing reader projects it".

**Configurability**
- `recallOverfetchFactor` stays a `const`. No operator tunes it, no environment differs, it is not a secret
  (#1136 §3's concrete rule). It is not paired with an audit column — §13.3 R-D deletes exactly that
  compound.

**Less is better**
- Every element passed delete / merge / inline (§13.4).
- The trade-off is named explicitly where the simpler shape loses: R-A is simpler and measured
  insufficient; R-C preserves more and is rejected on naming, with the anchor as precedent.
- Reader inventory in §15 covers **both** Go references and string-literal / prose references, and is
  labelled **known-not-exhaustive** with the property to sweep for.

**Document discipline**
- Cites #114 (via #10861) and #1136 as load-bearing; scope and non-scope both explicit (§3).
- No design is superseded end-to-end here. `anchor-grounded-recall.md`'s **Q3 is answered and reversed**,
  and the amendment is a Unit 1 deliverable rather than a later sweep (§15, §18).

---

## 18. Implementation Guidance for the Next Agent

**Two units, two PRs, in this order. No git and no `gh` — return the working tree.**

### Unit 1 — the aperture stops spending slots on rows admission refuses *(this is the feature)*

| step | what | acceptance |
|---|---|---|
| 1 | **One unexported const in `internal/loop/retrieve.go`:** the over-fetch multiplier, value **5**, named `recallOverfetchFactor`. **No doc comment** — #10861's table gives unexported identifiers *"default none"*, so the name is the whole carrier and must read as a **multiplier rather than an addend**; the `Factor` suffix is what carries that, and it is not decoration. *(An earlier revision made this point by contrasting the suffixed name against the unsuffixed one — and a global rename then rewrote the contrast term into the recommended term, leaving the cell asserting that its own chosen name was not the name. The repair states the requirement positively, so there is no second identifier for a rename to swallow: **P-44**, check one hop out from what you rewrote.)* | It is `const`, unexported, **comment-free**, and lives beside `fusionRankConstant`, which also carries none. No config variable, no env read, no `Limits` field. **Falsifier:** `grep -B2 -n "recallOverfetchFactor" internal/loop/retrieve.go` shows no `//` line above the declaration |
| 2 | **`Retrieve` computes the fetch count once and passes it to every recall it issues** — the `len(queries)` unscoped calls **and** the scoped call. `limit` continues to bound `fuse` | **G-2.** Assert the count on *every* recorded call, the scoped one included. The scoped one is the easy miss and §2.4 measures it as the most record-dense pool |
| 3 | **`appendUnseen` rejects a self-produced candidate**, beside the anchor check it already performs. Nothing else in `fuse` changes — not the three passes, not the `len(out)` fill conditions, not `fuseByReciprocalRank` | **G-1, G-3, G-6.** Do **not** pre-filter the lists (R-H): that changes RRF ranks |
| 4 | **Update `Retrieve`'s godoc** to say it never returns a row this system wrote, and that it asks for more rows than it returns. **It stays one line and must not grow**: measured at `0df5c14` it is **167 chars** (`retrieve.go:11`) and **the longest one-line godoc in the package**; the next is `Source` (`types.go:31`) at 165, and the one-line exported godocs span **36–167**. **The class the counts range over, stated because two rounds disagreed on a number without either side defining it:** an *exported declaration with a doc block* here means a `func`, `type`, `const` or `var` declaration, or a struct field, carrying a contiguous `//` group immediately above it — **excluding interface methods and excluding the package doc comment**, which #10861 gives its own table row. On that class: **63** exported doc blocks, **57** one-line, **6** multi-line, **2** unexported. **Include** interface methods (`internal/loop` has 7 — 6 one-line, 1 multi-line, across `GraphPort`, `FilePort`, `ModelPort`) and it is **70 / 63 / 7**; add the package doc at `types.go:1` and the unexported count is **3**. Both classings are defensible and the arithmetic between them is exact, which is why naming the class closes it and quoting two totals did not (**#13609 W-9**). **Every figure step 4 depends on is invariant across all three classings** — the 36–167 range, `Retrieve` at 167 as the maximum, `Source` at 165, and **0** unexported constants. *(Citations here name the **doc-block** line, not the declaration line: `Retrieve`'s block opens at `retrieve.go:11` and declares at `:12`. A claim about a doc block's length should point at the block.)* So the two new properties are added by **trimming what is already there**, not by appending — the scope reserve and the fusion are documented in `fuse` and in §7, and the godoc does not owe them | The sentence names both new properties **and is ≤ 167 chars**. A godoc that names only the exclusion describes R-A; one that grows past 167 makes the package's longest godoc longer, which is what #10861's *one tight line* and #1219's *"markedly longer than the untouched members around it"* between them forbid |
| 5 | **Make the property in §15 true across the tree**, not the list of sites in §15. Sweep for it; §15's table is a search result, not a specification | **The pass is mechanical.** Extract every `file:line` and node-id citation on every file the branch touches — not only the files edited — and resolve each against `0df5c14` and against its node |
| 6 | **Amend, as a Unit 1 deliverable rather than a follow-up:** `anchor-grounded-recall.md`'s **Q3** (answered and reversed), `m1-skeleton-loop.md`'s **R13** surviving-risk clause, and **#13585 §17 row 3**. Each amendment states the property that is now true and what survives of the old claim for the *supplementary* aperture | No amendment may say "self-produced rows no longer consume candidate slots" without the qualifier, because `dispatchRecall` still spends them (§2.5) |
| 7 | **One WARN in `turn.go`'s run-summary block**, guarded by `len(record.Candidates) < record.Limits.CandidateLimit`, placed beside the existing shutout WARN at `turn.go:189`. It is the emitter for §14's detector. Its message says the aperture under-delivered and carries both numbers; it says nothing about the answer | **G-7.** The fixture is the **empty**-aperture arm specifically — the existing shutout WARN's `> 0` guard keeps it silent there, so a test asserting merely that "a warning fired" would pass against the old one |
| 8 | **Run every predicted falsifier in §14 and record its observed output in the PR** — **including R-4's, which requires temporarily altering `admit`'s own predicate and reverting it**, because G-4 cannot redden against anything this diff contains. Any that cannot be made to redden is reported as *"no runnable falsifier established"* rather than as passing | §14's preamble. A predicted mutation is not evidence |

**Do not:** edit anything in `turn.go` except the one WARN in its run-summary block — not the constants,
not the `Retrieve` call site, not `dispatchRecall`. Do not edit
`assemble.go`, `admit`, `Assemble`, `renderBlock`, the byte budget,
`MaxModelCalls`, the supplementary budget, `GraphPort`, `internal/divoid`, `cmd/eval`,
any corpus row, any sidecar row, or `scripts/compare_tasks.json`. Do not delete `admit`'s
`cutReasonSelfProduced` arm — it is reached from `dispatchRecall` (§15). Do not add a `Limits` field, a
config variable, or a shared `inadmissible` helper.

### Unit 2 — the grading key stops grading on the mechanism that was removed *(its own PR, after)*

`scripts/compare_tasks.json:21` is a **measurement fixture** whose expected answer asserts slot
displacement as the RIGHT answer. It is separated from Unit 1 deliberately: changing a grading key is a
decision about the instrument, and bundling it with the change it measures is how a rig comes to certify
its own diff. It must be revised against the tree Unit 1 produced, not against this document.

### How the fix is measured — two arms, both required

**The premise (A2), stated because everything rests on it:** retrieval is reproducible — #13598 and
#13600 shared 19 of 20 candidates for an identical input and the same top hit to four decimal places
(#13592). That is what makes a before/after delta attributable to the change rather than to noise.
**Without it there is no measurement here, only two numbers.**

### The baseline is a freshly-taken run, never a stored record — and the two arms do *not* share a graph state

**An earlier revision used #13598's and #13599's stored records as the "before" and asked the
well-matched arm for an unchanged candidate id set. Both halves were wrong, and they were wrong for one
reason.**

> **Running the product mutates the corpus its own measurements are taken against.** Every completed run
> files a record and links it to the subject, so the graph a measurement is taken against is not the graph
> the previous measurement was taken against (#13592).

#13598's stored record shows **0 self-produced** candidates because at the time it ran, neither its own
record nor #13600's existed. Both exist now, and both rank in that input's top 20 — **measured 2 in
§2.4 and re-derived by QA at ranks 2 and 3, similarities 0.7500 and 0.7449.** So an identity criterion
against that record was satisfiable only on a graph that no longer exists, and **unsatisfiable in
principle** on any graph containing the input's own prior runs — a set that grows with use.

Selecting the one of three observations that had no record in its top 20 is also the exact sampling
error #13592 and #13534 §11 warn about, **applied to the control arm instead of the treatment arm.**

**So the baseline is taken fresh, back to back with the after arm**, on `main`.

> ~~**So the baseline is taken in the same session, immediately before the change**, on `main`, against the
> same graph. That is this project's established rule, not a new one: #13585 Unit 1 step 1 requires *"the
> same rates as a sweep taken immediately before the change, in the same session. Not against a rate from
> another day — the graph is live and unversioned."*~~
>
> **Corrected 2026-09-11 (#13703, from #13702 W-3). Two things were wrong, and the second is the one that
> matters here.**
>
> **First, the borrowed rule was itself defective.** *"Same session"* was written in #11235 §9.1 against a
> baseline that moved **overnight**, and it bounds cross-day comparison only. Measured 2026-09-11: an
> unchanged binary read `admitted 8/23` at `18:33:10Z` and `9/23` at `19:38:16Z` — **inside one session**.
> #13585 has been corrected and **#11235 §9.1a is now the canonical statement**; this document points at it
> and does not restate it (#11034 P-52 — restating is how the defect arrived here).
>
> **Second, and specific to this document: the conclusion contradicted this subsection's own opening
> premise.** That block quote — *"Running the product mutates the corpus its own measurements are taken
> against … the graph a measurement is taken against is not the graph the previous measurement was taken
> against"* — says consecutive measurements do **not** share a graph. **"Against the same graph" is
> precisely what it rules out.** These two arms are **runs of the product**, not sweeps, so each one writes — and unlike
> #13585's sweeps they cannot be bracketed by a repeat, because the repeat would write again. **#13585's
> A-B-A control is unavailable here, and no amount of clock discipline substitutes for it.**

**What this section can claim instead, and it is enough.** The two arms are **known** to see different
graph states, differing by the records the arms themselves file — #13592 measured the size of that effect
at **19 of 20 candidates shared** for an identical input, the single delta being the new record displacing
one row. **So the acceptance relations must be invariant under that mutation rather than assume it away**,
and AC-1 and AC-2 below are stated as relations between the arms for exactly that reason. **AC-1's basis is
argued under the headings "Why AC-1 holds, at the aperture" and "Why AC-1 survives the arms not sharing a
graph state"; the second is load-bearing, not a remark, and it names the one residual this does not
close.**

**And the stored records keep a different job.** A stored record is the reproducible *instrument* for what
happened when it ran — #13599's dispositions still say 17 of 20 however the corpus grows. What it is not
is a *control* for a change measured later. §2.2's figures stand; they are history, not a baseline.

### What each arm asserts — two relations that must hold, and one measurement that is reported

> **Label note.** These are **AC-**numbered because §6 already owns `A1`–`A6` for *assumptions*, and this
> section cites §6’s **A5**. An earlier revision numbered the acceptance relations `A1`–`A3`, which put two
> meanings of `A1` in one document — found by sweeping this document’s own label namespace rather than by
> review.

Both arms are run against the container on a real graph, not a fixture, and both use the **verbatim**
inputs read out of the records' own `input` field.

| # | must hold | status |
|---|---|---|
| **AC-1** | `baseline` minus `after`, as **candidate** id sets, is **exactly** the baseline's `cutReason: self-produced` ids | **Provable, and proven.** Both directions — every ineligible row gone, **and nothing else lost.** Truth independent of how many records the graph holds |
| **AC-2** | `len(after.candidates) == CandidateLimit` | **Provable** under **A5** (§6). The same expression as §14's exhaustion detector |

**Why AC-1 holds, at the aperture.** Removing rows from a ranked list cannot demote a row that remains, so
every eligible baseline candidate is still within the top `limit` of a pool that only got easier to rank
in; and the reserve pass increments `reserved` only on `appendUnseen`'s true return, so a skipped
self-produced scoped row does not consume a reserved slot an eligible one would have taken. QA
constructed the pass-1 / pass-2 / pass-3 cases against `0df5c14` and could not make a non-self-produced
baseline row disappear.

**Why AC-1 survives the arms not sharing a graph state** *(added 2026-09-11, #13703 — this was previously
carried by the false claim that the arms ran "against the same graph")*. The known mutation between the
arms is **the baseline arm's own record**, and a `processor-run` record is **self-produced by
construction**. The `after` arm skips self-produced rows on all three of `fuse`'s fill passes, so that new
record **cannot enter `after`'s candidate list and therefore cannot displace an eligible row from it.** The
arms differ by a row that one arm was always going to exclude — which is why AC-1's status column reads
*"truth independent of how many records the graph holds"*, and that line is now doing work rather than
reassuring.

**The residual this does not close, stated rather than argued away.** `Retrieve` over-fetches and `fuse`
filters, so an extra self-produced row in the graph shifts every eligible row **one position deeper in the
raw recall**. An eligible row sitting exactly at the fetch boundary could therefore fall out of `after`'s
pool without being self-produced, which would put it in `baseline − after` and **fail AC-1**. **Bounded,
not eliminated:** §13.2's headroom is the margin — at `fetch = 100` with **37** records in the whole graph
(the figure and the `divoid_list` query that produced it are quoted in §2.4), the eligible pool is at
least 62 against a `limit` of 20, so the boundary sits far from any eligible row the arms care about. **This is the same
headroom the WARN in §14 watches**, and if that WARN ever fires, AC-1 is no longer safe either — which is
the one coupling between the expiry detector and the acceptance relations, and it was not previously
stated.

**Falsified by:** an AC-1 failure whose missing row is **not** `cutReason: self-produced` in the baseline
and **not** at the fetch boundary. That would mean the mutation between the arms is something other than
the arms' own records, and this section's whole basis is wrong.

### The admitted set is reported, not gated — and the withdrawn claim is worth reading

An earlier revision asserted a third relation: *"`after`'s admitted id set **contains** the baseline's"*,
and called it **guaranteed**. **It is not, and the error was not where the argument looked.**

The `admit` half of that argument is sound and survives: `cumulative` is incremented only in the
`case cumulative+size <= budget` arm and the `case c.SelfProduced` arm precedes it
(`internal/loop/assemble.go:51-60`), so **removing self-produced rows alone would leave every survivor's
arithmetic identical.** The two clauses that actually carried the conclusion were both about `fuse`, and
both were false:

| the claim | what the code does |
|---|---|
| *"the freed slots are filled at the tail"* | **They are filled at the head.** `fuse` pass 1 fills positions **1–17** from `fused` in rank order and pass 2 appends the reserve at **18–20** (`internal/loop/retrieve.go:93-108`). A skipped row makes pass 1 walk deeper into `fused` and append the replacement **into positions 1–17**, ahead of the reserve rows |
| *"where they can only add"* | **`cumulative` is one shared accumulator.** A row admitted at an earlier position subtracts its own size from the headroom at every later position. There is no per-row budget |

**And the same section already contradicted it.** The well-matched magnitude cell computed **780 B** of
remaining budget. A binding budget and *"can only add"* cannot both be true.

**The worked consequence on the ill-matched arm.** #13599's baseline candidate positions **1–17 are the
seventeen records, every one charged nothing**; its **entire admitted set** is at positions **18, 19, 20**
— ids 12988 (12,559 B), 13542 (3,286 B), 11358 (1,559 B). After the change those seventeen positions hold
**seventeen real rows that cost bytes**: at the measured mean of **~7,154 B/row** (#13598's 20 rows /
143,089 B) that is **~121,600 B of demand against 39,188 B of slack**. Skip semantics leave residual gaps
rather than certain failure — which is exactly why it is not a proof. 12988 is least exposed (pass 1
picks it up third among eligible rows, at low cumulative); **13542 and 11358 are the exposed pair.**

> **Correction, round 3 (#13616 CF-9): an earlier revision extended this finding to the well-matched arm
> and the extension was wrong.** It claimed the two records sit at candidate positions 2–3 so their
> replacements insert demand *"ahead of most of the baseline's admissions"*, and concluded containment was
> *"expected to fail on both arms"*. **Both halves are false, and the second was self-defeating on its own
> figure.**
>
> **Where the new rows actually land: positions 16–17, not 2–3.** `fuse` pass 1 fills positions 1–17 from
> `fused` **in rank order** (`internal/loop/retrieve.go:93-98`). Skipping two records makes it walk two
> ranks deeper, so **positions 1–15 hold the same rows in the same order** as the baseline's positions 1,
> 4…17, and the two genuinely new rows — fused ranks 18 and 19 — enter at **16 and 17**, ahead of only the
> three scoped-reserve rows. And because the removed records charged nothing, **`cumulative` at position 16
> after the change equals the baseline's `cumulative` after position 17.**
>
> **The figure argued against the conclusion.** `admit`'s `default` arm sets `CutReason` without touching
> `cumulative` (`internal/loop/assemble.go:58-59`) and there is no latch, so **a row cut on size charges
> zero and displaces nothing.** Two rows of ~7,154 B against 780 B of slack are *cut*, not admitted — so
> the very number the argument rested on is the case where its conclusion does not follow. **An estimate
> was doing a proof's work one layer above where the hedge had been placed.**
>
> **And the split QA recorded as unmeasured is measurable from the stored run.** Computed from #13598's
> `candidates[]`: **all 55,812 admitted bytes sit at positions 1–13**; positions 14–20 admit **nothing**,
> and every row there is already cut on size (2,381 / 1,496 / 1,502 / 2,365 / 4,860 / 32,449 / 3,492 B —
> each above the 780 B that remains). So on this arm the insertion point sits **strictly behind every
> admission**, nothing admitted lies behind it to displace, and **containment holds.** Not *undetermined*
> — measured, for this input.

**The condition that decides it, which is what the withdrawal should have named all along:**

> **Containment fails exactly when the aperture's replacement rows land *ahead* of rows the baseline
> admitted.** Measured **true** on the ill-matched arm — seventeen replacements at positions 1–17 ahead of
> an admitted set that lives entirely at 18–20 — and measured **false** on the well-matched arm, where two
> replacements at 16–17 sit behind an admitted set that ends at position 13.

**The withdrawal stands, and on firmer ground than the blanket claim it replaces.** It rests on the
ill-matched arm alone: 17 × 7,154 = **121,618 B** of demand against **39,188 B** of slack, with the early
replacements genuinely admitted at low cumulative. A relation that is measured to fail on one arm and hold
on the other is not an acceptance relation — and stating the *condition* is worth more than asserting the
failure, because it tells an implementer which arm to look at.

**So the admitted set is a reported measurement, not an acceptance gate:**

| # | reported on both arms, baseline and after | expected direction, with its premise |
|---|---|---|
| **M1** | the admitted **id sets** | **Composition shifts on both arms.** A row admitted only because free riders sat ahead of it can be displaced |
| **M2** | the admitted **byte totals** and row counts | **Ill-matched: up, substantially** — 39,188 B of slack was going unused because the aperture had only three eligible rows to offer. **Well-matched: flat** — the budget was already 98.6 % spent, so nothing can improve there, and the row count may fall while bytes hold |

**Nothing is gated on M1 or M2**, and that is the point: an implementer who meets a displaced admission
**reports it rather than weakening the arm.** This also closes **#13607 W-5** properly — *"admitted well above 3"*
was unadjudicable, and the remedy was to move the admitted figure **out of acceptance** rather than to
invent a threshold for it. *(Precision, because an earlier revision put this wrongly: the figure **was** an
acceptance clause in revision 2 — it sat in a column headed "after — required", which is exactly what made
W-5 a finding rather than a preference. Saying it "was never one" is right about where it belongs and false
about where it was.)*

**Where a displaced admission belongs, and it is not here.** When twenty real rows contend for 56,592 B
at a measured mean of ~7,154 B/row, **roughly eight fit and twelve are cut on size** (56,592 / 7,154 =
7.9). **That is #13594's subject** — oversize
candidates against the byte budget — surfacing where the aperture previously hid it behind seventeen
free riders. **This change converts an aperture defect into a budget defect on the ill-matched arm, and
the budget defect already has an owner.** Stating the interaction and stopping (§16.2).

### What the relations rule out — evaluated in the arms they are declared in

**The table below is evaluated on the two acceptance arms, because that is where AC-1 and AC-2 are
asserted.** An earlier revision evaluated it as a property of the implementations in the abstract, which
made one cell claim coverage the arms cannot deliver (#13616 CF-8).

| implementation | AC-1 | AC-2 | separated by |
|---|---|---|---|
| no change | **fails** — the difference is empty while the baseline has self-produced rows | passes (20) | **the arms** |
| **filter only, no over-fetch (R-A)** | passes | **fails** — 5 or 6, re-derived in §13.3 | **the arms** |
| **back-fills with records rather than returning short (G-6's shape)** | **passes** | **passes** | **G-6, a unit guard — not the arms** |
| **filter + over-fetch (this design)** | passes | passes | — |

**Why the back-fill shape is invisible to both arms, and it is arithmetic rather than judgement.** That
implementation is defined by its *shortfall remedy*: it back-fills with records only when the eligible pool
cannot fill `limit`. At `fetch = 100`, with at most **37** records in the whole graph and one anchor, the
eligible pool is **at least 100 − 37 − 1 = 62** against a `limit` of **20** — and `fuse`'s pass 3 runs only
`if len(out) < limit` after pass 2 (`internal/loop/retrieve.go:110-115`). **The back-fill branch is dead
code on both arms**, so that implementation is observationally identical to the correct one there: AC-1
passes, AC-2 passes. It is separated on the **degenerate** arm — every fetched row self-produced — which is
**G-6's unit fixture, not an acceptance arm.**

**So the honest division of labour:** the two integration relations separate two of the three wrong
implementations; the third is separated by a unit guard, and §14 is where that coverage lives. **The design
is not under-guarded** — what was wrong was the acceptance section claiming coverage that belongs to §14.

> **The shape this was:** an inherited table cell that became load-bearing when a new claim was hung on it.
> The cell had read *"fails"* since the table was written; revision 4 promoted it into the sentence
> establishing that withdrawing containment cost no coverage, and nobody read the result column back
> against the mechanism. **#1219's §6 addendum of 2026-09-07 states it exactly:** *"A coverage table has
> two columns and P-41 audits one of them. The guard's **name** is greppable, so it gets checked. The
> guard's **result** is a measurement you already took, and nothing prompts you to read your own matrix
> back against it."* This document runs P-41 over its guard names (§14) and had never run anything over
> its acceptance table's result column.

**And the withdrawn relation was worse than non-discriminating — it was anti-discriminating.** Evaluated
against the code rather than as the table recorded it:

| implementation | containment |
|---|---|
| no change | **passes** — `after` equals `baseline` |
| filter only (R-A) | **passes** — ~6 candidates, ~20 KB, all three baseline admissions fit under 56,592 B |
| back-fill shape | **not guaranteed** — identical to this design on both arms |
| **filter + over-fetch (this design)** | **not guaranteed** — #13609 CF-7 |

**It would have reddened on the correct implementation and stayed green on doing nothing.** That is a
considerably stronger argument for withdrawing it than *"it discriminated nothing"*, and it is the one to
carry: a relation can fail to discriminate, or it can discriminate **backwards**, and only the second one
actively rewards the wrong change. §14's falsifier rule (*a row that cannot discriminate is a wish*) has no
cell for the backwards case, and this is the instance that says it should.

**Re-measure the corpus figures before running.** §2.4's numbers are a property of the graph on
2026-09-11 and the corpus grows daily; the count of `processor-run` nodes is one call.

---

## 19. What this does not fix — filed, not solved

| # | Not fixed | Filed as |
|---|---|---|
| 1 | **What a run record should be**, given it is a first-class competitor in its own graph: 37,163–77,554 B of JSON per run, ranking 0.7500 against its own input, growing with what its run admitted — **so a successful run writes a larger competitor than an unsuccessful one.** Per §4's ruling, if these are memory worth having then the defect is that `admit` refuses them; if they are not, they should not be produced in that shape. Either answer is a design. **Note for whoever takes it:** #13594 records that the two oversize *designs* cut on #13591 carried no substance, while the run records in the same set carried ~2.2–2.4 KB of it — so run records are the one class that already has a cheaper form, and nothing reads it | **#13602** |
| 2 | **The supplementary aperture** (§2.5). `dispatchRecall` spends `CandidateLimit` slots the same way and does not pass through `fuse`; **observed twice on #13598**. Fixing it means a second predicate site — which would make **three** places that must agree where this design **raises it from one to two** (§11, §13.4) — or routing through `Retrieve`, which #11263 ruled against | **#13603** |
| 3 | **A negation predicate on the graph's listing route.** The only shape whose cost does not grow with the record corpus, and the one that would let `count = limit` be correct again. DiVoid-side, another repo. It also carries a finding of its own: **DiVoid #8 documents neither `query=`, nor the `similarity` it returns, nor `substance` in `fields`**, all three of which this product uses in production | **#13604** |
| 4 | **The two-phase fetch** (R-F). Its trigger is a *measured* retrieval-latency or memory figure — most likely after #13585's fan-out multiplies §14 R-2 by `N+1`. **Take the measurement before taking the task** | **#13605** |
| 5 | **Oversize candidates and the undisclosed cut** | **#13594** |
| 6 | **Every retrieval constant is untested under live fusion** — #13585 §17 row 5. `recallOverfetchFactor` joins `CandidateLimit`, `RecallScopeReserve` and the budget in that set, and §14 R-3 is its specific hazard | #13585 §17 row 5 |

---

## 20. Open Questions — both raised, both answered, recorded with their reasons

**Neither is open any more.** They are kept here rather than deleted because the *reasons* bind the next
change, and one of them changed this document.

1. **Should Unit 1 also raise `RecallScopeReserve`? No — raised and deferred, with a trigger.** The scope
   seed is measurably the most record-dense pool available — **19 of subject #10422's 57 direct
   neighbours**, rising by one per run — and the reserve is three slots wide. Two reasons to wait, and
   the second is the sharper one:
   - **Attributability.** Moving two constants in one change makes neither attributable. That is cheap
     rather than burdensome here, precisely because **retrieval is reproducible** (A2): a separate
     measurement of the reserve is a small clean experiment, not a deferral.
   - **Unit 1 changes the answer to the question.** After it lands the scoped recall over-fetches too, so
     the reserve's three slots stop being competed for by rows that would be discarded anyway. **Raising
     it now would tune a constant against a measurement this change invalidates.**

   **Trigger, filed as #13606:** once Unit 1 is on `main`, measure what the three reserved slots actually
   contain — readable from each candidate's `sources`, where a reserved arrival carries `scoped: true` —
   and *then* decide whether three is right.

2. **Is `len(candidates) < CandidateLimit` worth a WARN? Yes, and it is now part of this design** —
   §12, §14, §16.2, and Unit 1 step 7. An earlier revision left it to #13594 on the grounds that operator
   disclosure belongs there; **that was the wrong side of a real boundary.** #13594 part B is about the
   *answer*; this is an internal health signal about *this design's own central constant*, concerning
   rows that were never fetched — which nothing in #13594 can see. §16.2 carries the full distinction.

   **And it is what §13.2's universal was missing.** A claim with a stated expiry needs something that
   fires when it expires; without it the multiplier can silently become insufficient and the only symptom
   is a quieter aperture that still looks like a full run.

---

## 21. A note on how this document records its own corrections

Three revisions have each left dated *"an earlier revision…"* markers at the sites they corrected, and
that practice is worth one rule, because a round of this document got the rule wrong about itself.

> **"Left visible" is a claim about the document's current text, and it needs the same grep as any other
> claim about the text.**

A fix round reported three self-found defects as *left visible*. Checked against the shipped revision:
one was (the doc-block count, with both figures and the other extractor named); one had been **silently
corrected** before anyone could see it, which costs nothing but is not visibility; and one was **present
and unmarked**, which is the only shape of the three that costs a reader anything — it read as a
confident measurement with nothing flagging it. **One label was applied to three different states.**

So: a site is *visible* only if the current text says something happened there. Where a correction is
worth recording, it carries its marker; where it is not, it is simply fixed and not claimed. Both are
fine, and **the claim has to match which one was done.**

