# Architectural Document: Excluding Self-Produced Run Records at Fusion

> Repo path: `docs/architecture/self-produced-exclusion-at-fusion.md` (canonical copy — the DiVoid node
> carries the same bytes; an edit to one is not finished until the other matches it, P-40).
> Change: branch `fix/exclude-run-records-from-recall`, one predicate in `internal/loop/retrieve.go`.
> Measurement: **#13091** (six live runs; §6 is the crowding measurement) · Decision record: **#13092**
> (written against `main` `9d8da90`) · Review: **#13185** (PR #48, CF-1 is what commissioned this document).
> Project: **#10422** · Documents reconciled by this change: **#10532** (M1), **#10926** (M2), **#11235**
> (M3), **#11753** (anchor-grounded recall), **#12969** (hub pruning), **#12955** (substance-backed
> admission). Not reconciled because nothing in them is falsified: **#10904** (`run-record-fate.md`),
> **#10437** (`m0-service-skeleton.md`), **#10488** (`process-boundary-test-harness.md`),
> `anchor-stratified-corpus.md`.
> Standards applied: Design Contracts **#1136** §1 · Code Contracts **#114 §0** · retraction discipline
> **#11228** Lesson 3 · falsifier-cell constraint **#1220 §5 addendum 2026-09-05**.

---

## TL;DR

**What.** Record the decision, already shipped, that `fuse` drops every row this system wrote from the
candidate list — and reconcile the six design documents whose reasoning it falsifies.

**How.** One predicate in `fuse`'s `appendUnseen`, beside the anchor exclusion already there: a
`SelfProduced` row is refused a candidate slot. Placed **after `sourcesOf`**, so a survivor keeps the rank
the graph gave it; **not** in the adapter, which marks and drops nothing. The admission cut is retained.

**Cost.** Two instruments go quiet. `selfProducedCandidates` reads 0 by construction, so the crowding
quantity #13091 §6 measured is now recorded nowhere (§6). And the scope reserve no longer lands at fused
ranks 18–20 — the premise `hub-pruning-in-the-anchor-scope.md` retires Unit 3 on (§7).

**Rejected.** Over-fetching a deeper list: measured, and it buys nothing against 387 B of headroom while
adding to a recall that already discards ~1 MB of run-record bodies (§8.1).

---

## 1. Problem Statement

The harness writes one `session-log` run record per run into the same graph its recall query reads. Those
records embed the input verbatim, so they score highest for exactly the input that produced them. They were
admitted as candidates, ranked, and then cut at admission with the reason `self-produced` — but they spent
the candidate slot first.

**Measured, #13091 §6, six live runs on 2026-09-07:** 10 of 20 candidate slots went to run records at ranks
1–8, 10 and 11. Re-derived by review on the same day (#13185 §4): **13 of 20**, at ranks 1–11, 13 and 14.
The population grew 10 → 13 in one day, and each record is 76–88 KB.

**The goal:** stop spending candidate slots on rows that can never enter the block, without losing the rank
the graph assigned to the rows that survive, and without moving domain policy behind a port that test
doubles implement.

**Success criterion:** zero run records in the candidate list, every surviving candidate carrying the rank
the unfiltered graph ranking gave it, and the admission-time refusal still in place for any row that
reaches `Assemble` by another route.

---

## 2. Scope & Non-Scope

**In scope.** The candidate list `Retrieve` returns to `Turn.Run` and to `cmd/eval/sweep.go`. The decision
record for the placement. The reconciliation of the design documents this reverses.

**Out of scope, deliberately.**

| Not this change | Why |
|---|---|
| The supplementary recall path (`dispatchRecall`) | It does not call `Retrieve`. §5 states the asymmetry rather than closing it, because closing it is a different decision with a different reader |
| The admission-time `self-produced` cut | Retained unchanged. It is the second refusal and the only one left on the supplementary path |
| Recording the excluded count | A new field on `Record`, which is its own unit. §6 names it and prices it |
| Deleting or scoping run records in the graph | #13091 §7's housekeeping is an operator action, not a loop behaviour |
| Whether run records should be recallable *at all* | M1 §6.3's *"self-recall is ruled a capability worth having; the run record is the wrong vehicle for it"* is untouched |

---

## 3. Assumptions & Constraints

| # | Assumption | Confidence |
|---|---|---|
| A1 | `Candidate.SelfProduced` is set by the graph adapter as `divoid.IsRunRecord(type, name)` — node type `session-log` **and** name prefix `processor-run`, both required | Read from `internal/divoid/client.go` and `internal/divoid/write.go` at `bb09a2c`. Certain |
| A2 | The adapter drops nothing; it marks. M1 §8.3's recall row states this as a contract and it is unchanged by this design | Read from the source. Certain |
| A3 | `admit` cuts `SelfProduced` before the byte test, so a run record has never been admissible at any rank | Read from `internal/loop/assemble.go`. Certain, and it is why M3 §7's R4 falsifier was never runnable |
| A4 | `CandidateLimit` = 20, `RecallScopeReserve` = 3 | Read from the source at `bb09a2c` |
| A5 | The graph is live, shared and mutating. Every candidate-set figure below is a reading of one substrate at one instant and is labelled with it | Hard constraint, and the reason §4's benefit figures carry their hedge in the same sentence |

---

## 4. The mechanism, and where it sits

One predicate, in the closure `fuse` already uses for all three of its passes:

> A candidate is refused a slot if its id is already taken **or** if it is a row this system wrote.

`appendUnseen` is the single call site for the fused first pass (capped at `limit − reserve`), the scope
reserve (where the return value gates `reserved++`), and the fused backfill. One predicate, three passes,
no repetition — which is what keeps this inside #1136 §1 rather than needing a helper.

**Three placements were available and two were rejected. The argument is #13092's and is reproduced rather
than paraphrased:**

| Placement | Ruling |
|---|---|
| In `fuse`, after `sourcesOf` is built from the **unfiltered** lists | **Chosen.** A surviving candidate keeps *the rank the graph gave it*, not the rank it inherits once the records ahead of it are dropped |
| In `Retrieve`, before fusing | **Rejected.** Filtering before `sourcesOf` renumbers every rank and makes the crowding unreadable in every run record written afterwards |
| Behind `GraphPort.Recall`, in the adapter | **Rejected.** `Candidate.SelfProduced` is documented as *"the graph adapter sets it"* — adapter classifies, loop decides. Moving the exclusion behind that seam puts a domain policy where test doubles also implement it, and makes the flag it sets always false |

**The consequence that is easy to miss, and it is the one §7 turns on.** The first fused pass now stops at
`limit − reserve` = 17 **or when the fused list runs out of rows it will accept, whichever comes first**.
Write `n` for the number of unscoped fused rows that are not run records, capped at 17. The reserve's
arrivals land at fused ranks `n+1 … n+3`, and the returned list is `≤ limit`, not `= limit`.

**That formula was derived from the code and then checked against both independent measurements, which is
the only reason it is stated as a formula rather than as an observation.** #13092 read 10 records in a
top-20, giving `n` = 10 and a list of 13 — it reported 13. #13185 §4 read 13 records in a top-20, giving
`n` = 7 and a list of 10 — it reported 10, with `Scoped:true` sources on exactly ranks 8, 9 and 10. Two
different substrates, two different values of `n`, both predicted.

**What was measured, each figure with the substrate it was read on.**

| | #13092, morning of 2026-09-07 | #13185 §4, same day, hours later |
|---|---|---|
| candidates before → after | 20 → **13** | 20 → **10** |
| run records among them | 10 → **0** | 13 → **0** |
| admitted before → after | 6 → **8** | **3 → 3** |
| admitted content | 54,939 B → 56,205 B | **56,411 B → 56,411 B** |
| unused budget | 1,653 B → 387 B | **181 B → 181 B** |

**Read the mechanism column, not the benefit column.** The mechanism reproduced exactly on both readings —
zero records in the candidate list, and the `n + 3` decomposition confirmed live (ranks 1–7 carrying
`Scoped:false`, ranks 8–10 carrying `Scoped:true` only). **The benefit did not reproduce at all**, because
a single 43,273 B document saturates 76.5 % of the budget in both states and a new 12,027 B node took the
remaining headroom: on the later substrate the change alters the admitted set by nothing. Any sentence
quoting 6 → 8 without that hedge is making a claim about a graph that no longer exists.

**The claim this design is entitled to make** is therefore not *"more documents get admitted"*. It is:
**the real-candidate share of a fixed-width list stops falling as the harness runs.** The record population
grew 10 → 13 in one day; without the exclusion the effective candidate limit falls every day, and the
answer degrades gracefully to fewer candidates instead of failing.

---

## 5. What this does not reach — and the asymmetry is now the live risk

`Turn.dispatchRecall` calls `t.Graph.Recall(...)` directly and then `admit`. It never calls `Retrieve`.
**So the supplementary recall path is untouched:** run records still arrive, are still ranked, are still
cut at admission with the `self-produced` reason, and are still recorded in `toolCalls[].results[]`.
#13091 §6 measured 10 of that round's 20 rows self-produced and nothing here moves it.

This is deliberate, and it is not free:

- It is why M1 §8.2's `toolCalls[]` row is **still literally true** where its `candidates[]` row is not,
  and why the `self-produced` cut reason is not dead vocabulary.
- It is the only place the crowding evidence M1 §6.3 wanted still accumulates — see §6.
- It is a **shape asymmetry**: two paths that both call `admit` now disagree about what may reach it. A
  future reader of either path alone will draw the wrong conclusion about the other. That is the cost of
  not extending the change, and it is stated rather than argued away.

`hub-pruning-in-the-anchor-scope.md` §13 O2 already named this path as unscoped and unmeasured. It is now
also **unfiltered**, and that is recorded there.

**Not proposed here.** Routing `dispatchRecall` through a shared exclusion is the obvious next move and it
is not obviously right: the supplementary round has its own budget, its own reader, and the only surviving
instance of the evidence M1 §6.3 asked for. It needs its own decision, taken against a measurement of what
the supplementary round actually delivers — which no instrument the project owns can produce today, because
the sweep calls no model.

---

## 6. What this costs the instruments, and the remedy shape

`internal/eval/result.go` counts `SelfProducedCandidates` from a row's dispositions. Excluded rows never
become dispositions, so **the metric reads 0 by construction on every sweep**, and
`internal/eval/report.go`'s diagnostic line has silently inverted meaning: from *how bad is the crowding*
to *did the exclusion fail*. Both are useful; they are not the same instrument.

**The quantity that is now recorded nowhere** is the one #13091 §6 was computed from: how many of the rows
the graph returned were this system's own paperwork. It is not in the run record, not in the sweep result,
and not in the operator log.

**Remedy shape, deferred, and priced.** One integer on `Record` — the count of rows the recall returned and
the exclusion dropped — carried out of `Retrieve` beside the candidate list, and reported by the sweep
beside `selfProducedCandidates`. That is a new field on a type M2 §8.2 specifies and a change to
`Retrieve`'s signature, which makes it a separate unit and not a rider on a two-line predicate.

**Why deferred rather than guarded now.** Under P-3 the blast radius is a diagnostic reading zero, not a
wrong answer: the model's input is unaffected either way. What was wrong before this document existed is
that the change of meaning was recorded **only** in a PR body and in #13092 — neither a durable repo
artifact. It is now recorded in three: here, `m2-retrieval-eval.md` §8.2 and §12 E4, and
`m1-skeleton-loop.md` §8.2.

**Falsifier for the deferral:** a sweep is read as evidence about crowding, or two sweeps taken either side
of a graph change are compared on `selfProducedCandidates` and the comparison is quoted. Either means the
integer should have been built.

---

## 7. The downstream consequence nobody was looking for: the scope reserve moved

`hub-pruning-in-the-anchor-scope.md` §4.1 property 3 read: *"The first fused pass stops at
`limit − reserve` = 17. So reserve occupants are always at fused ranks 18, 19, 20."* Under §4 above, that
is `n+1 … n+3`, and #13185 §4 measured `n` = 7 with scoped-only arrivals at fused ranks **8, 9 and 10**.

Its reasoning runs through that band throughout. The five below are the load-bearing ones, and one of
them is a guard; §11's re-derivation rule is how to find the rest, and there were eleven:

| What it said | Status |
|---|---|
| Unit 3's delivery surface is *"at most three candidate slots at fused ranks 18–20"* | Count holds, position does not |
| *"The reserve occupies ranks 18–20, strictly inside F4's forbidden band"* (§5.1) | **The inference fails.** F4 forbids rank > 16; the reserve now lands below it, so F4 no longer excludes the reserve either way |
| The gated claim in §6 | First premise false; left verbatim because it is a pre-registration |
| D1, the promotion counterfactual | Still runnable over the archived census, which records the old placement. No longer a counterfactual over production |
| §11's reopening condition — *"admission rate of candidates at fused ranks 18–20 exceeds 0.25"* | **Could no longer fire.** Restated against the reserve's actual slots, threshold and registration date unchanged |

**A second effect on the same channel, smaller and in the same direction.** Before this change a
self-produced row arriving through the *scoped* recall consumed a reserve slot and was then cut at
admission; `appendUnseen` now refuses it and `reserved++` does not fire
(`TestARecordThisSystemWroteSpendsNoReservedSlotEither`). So the reserve delivers up to three *admissible*
scoped rows where it previously delivered up to three rows of which some could be records. **#12966's
count of 57 scoped-only arrivals was taken under the old behaviour** and is a second reason its yield
figures do not describe the shipped loop.

**What this does and does not establish.** It does not say the anchor's scope channel now reaches an
answer. One live data point bears on it and only one: the reserve moved from ranks 19–20 to 8–10 and
`admitted` did not change (3 → 3, #13185 §4), on **one row, one substrate**. That is the *shape* of D1's
outcome row 1, not D1. What it does establish is that the retirement of Unit 3 rests on measurements taken
against code that no longer exists, and must be re-derived before it is acted on.

**The generalisable finding, and it is the reason this section exists at all.** A change to *retrieval*
moved a *placement* that an unrelated ruling had treated as structural — because the ruling's premise was
`limit − reserve = 17` and the pass had silently acquired a second stopping condition. **A derived
constant is a claim about the code path, not about the constants**, and it expires when anything on that
path gains a branch.

---

## 8. Alternatives rejected

### 8.1 Over-fetching a deeper candidate list — measured, and it loses

The unscoped top **40** contains exactly the same 10 records (all in the top 11, on the morning substrate),
so a 2× over-fetch would have delivered a genuine 20 admissible candidates. Rejected anyway:

- **Residual budget after admission was 387 B.** Ranks 21–40 would be considered and cut for bytes
  essentially without exception. The purchase is nil.
- **The cost is not nil.** One unscoped recall already transfers **805,047 bytes** of run-record content
  (~1.05 MB at the 13-record population #13185 measured) and discards all of it. Over-fetching adds to that.
- **The factor is a guess against an unbounded population.** Records accumulate; a factor that works at 13
  fails at 40. Filtering in place degrades to fewer candidates rather than silently failing.

### 8.2 A query-level exclusion — what M1 §11 R13 proposed

R13's remedy was *"excluding the run-record type is one query parameter"*. Rejected on M1 §6.3's own
surviving ground: node **type** alone cannot carry the distinction, because human-written session logs
share the type and were ranks 2 and 3 in the failing run. The shipped predicate is type **and** name
prefix, and it runs in the loop, so the graph's ranking is never asked to change.

### 8.3 Excluding at the adapter, or before `sourcesOf`

Both in §4's table. The first moves policy behind a port; the second destroys the rank evidence.

### 8.4 Doing nothing and raising `CandidateLimit`

`CandidateLimit` was measured at #11365 §5 to move `retrieved` only, never `admitted`. It also scales the
transferred bytes linearly against a population that grows on its own.

---

## 9. Coverage — the guard, not the mechanism

Every row names a test. The falsifier column names **only mutations whose observed output is quoted in
#13185 §3**; no cell predicts a result nobody ran.

| # | Property | Guard | Falsifier — observed |
|---|---|---|---|
| C1 | A run record spends no candidate slot, and the rows behind it move up | `TestARecordThisSystemWroteSpendsNoCandidateSlotAndTheRowsBehindItMoveUp` | M1, revert the predicate: RED (#13185 §3) |
| C2 | A skipped record spends no **reserved** slot — `reserved++` does not fire | `TestARecordThisSystemWroteSpendsNoReservedSlotEither` | M4, a skipped record still spends a reserved slot: RED, **and alone** |
| C3 | A record is not backfilled into a slot the reserve left empty | `TestARecordThisSystemWroteIsNotBackfilledIntoASlotTheReserveLeftEmpty` | M3, exclusion on the first pass only: RED |
| C4 | A surviving candidate keeps the rank the graph gave it, not the rank it inherits | `TestACandidateKeepsTheRankTheGraphGaveItRatherThanTheRankItInheritsWhenARecordAheadOfItIsDropped` | M5, move the filter into `Retrieve` ahead of `sourcesOf`: RED, **and alone**. This is §4's placement argument, pinned |
| C5 | The exclusion is by type **and** name prefix, never type alone | `TestARowCarryingTheRunNodeTypeWithoutTheRunNamePrefixIsStillAdmittedAsACandidate` | **No runnable falsifier established.** M6 (exclude by node type) reddened five tests, but four of them set no `Type` in their fixtures, so M6 degenerated to the revert for those four and their reds carry no information about the criterion. The one test written for the criterion is named here; no mutation against it has been run and quoted |
| C6 | A record cannot be admitted even when it fits, and is refused before the byte test | `TestAssembleReportsSelfProducedRatherThanBudgetForAFittingRunRecord`, `TestAssembleCutsSelfProducedCandidatesWithoutChargingTheBudget` | **No runnable falsifier established.** These guards predate this change and `internal/loop/assemble.go` is not in its diff; every mutation quoted in #13185 §3 was applied to `internal/loop/retrieve.go` and none of them reaches these two |
| C7 | End to end: a turn is not poisoned by its own previous record | `TestTurnRunIsNotPoisonedByItsOwnPreviousRecord` | M2, filter *after* the limit: RED |

**Falsifier for the table itself:** any row whose named guard would still pass against an implementation
lacking the claimed property. C5 is reported as a gap rather than filled, which is what that question is
for.

**Two limits of this table, stated.** (a) It covers the primary path only; nothing here guards the
supplementary path, because nothing there changed and no instrument reaches it (§5). (b) C7's guard fails
under a retrieval-side type mutant at a **turn-1 setup assertion**, upstream of the criterion assertion —
so its red sends a reader to the fixture, not to the criterion (#13185 W-3). C5 exists because of that.

---

## 10. Risks

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| R1 | **The exclusion regresses and nobody notices**, because the metric that would show it reads 0 in both the healthy and the broken case only by sign | `selfProducedCandidates` non-zero is now the alarm (M2 §12 E4), and C1–C4 guard the predicate | a sweep reports `selfProducedCandidates > 0` and no one treats it as a defect |
| R2 | **The crowding quantity is gone and a later decision is taken without it** | §6 names the gap, the remedy and its price, in three durable artifacts | a proposal about candidate crowding cites #13091 §6 as if it were current |
| R3 | **The supplementary path is assumed to be filtered** because the primary one is | §5, plus `hub-pruning…` §13 O2 and M1 §8.2's *do not sweep it with this one* note | a document or a test asserts that no run record reaches `admit` |
| R4 | **A ruling is acted on whose rank-band premise moved** | §7, and the five in-place corrections in `hub-pruning-in-the-anchor-scope.md` | any work is commissioned on the anchor-scope channel citing "ranks 18–20" |
| R5 | **The candidate list is shorter and something assumed 20** | M2 §5.3's `notRetrieved` verdict and §9's Guard-3 invariant are corrected; `CandidateCount` was always `len(dispositions)` | a document, a report line or a test asserts exactly 20 candidates |
| R6 | **A required node that is a run record reads `notRetrieved` forever** | Stated as a property in M2 §5.3. None of `corpus.json`'s 25 required ids is a run record today | a corpus row is added whose required node is a `processor-run` node |

---

## 11. Reconciliation — the rule that produced the site list, not a claim about its completeness

**Do not read the list below as an inventory.** It is what these layers reached; sweep the complement.

**The re-derivation rule, runnable by anyone.** All greps resolved against `bb09a2c`, never against a
`main` checkout — nothing in grep output says which tree it read.

1. **Bare terms, however generic:** `self.?produced`, `selfProducedCandidates`, `run record`, `run-record`,
   `processor-run`, `IsRunRecord`, `self.?poison`.
2. **Negations and absence claims:** *every row*, *all 20*, *unfiltered*, *drops nothing*, *not a rank
   exclusion*, *still retrieved / ranked / recorded*, *cut at admission rather than*.
3. **Contract and invariant blocks read in full rather than grepped** — M1 §8.2's field table and §8.3's
   port table, M2 §5.3's verdict table and §8.2's diagnostic table, M3 §4.3's component table,
   `substance-backed-admission.md` §6.2 and §8.1. This layer is what showed that §8.1's *"each carrying … the self-produced flag"* is
   **still true** (the adapter marks and drops nothing) while M1 §8.2's *"every row the query returned"* is
   not.
4. **Second-order consequence claims that never name the subject at all.** Ask *what else would be true if
   run records still filled the list?* The answer that mattered: *the fused pass always fills 17, so the
   reserve sits at ranks 18–20.* `hub-pruning-in-the-anchor-scope.md` contains **no occurrence of
   "self-produced"** and is the document this change damages most. Terms used: `candidate slot`,
   `candidate limit`, `18–20`, `out of 20`, `twenty candidates`, `rank > 16`, `reserve`.
5. **Every `.go:N` citation in all ten `docs/architecture` files re-resolved against `bb09a2c`, and each
   resolved line compared against the sentence it supports** — resolving is not enough, a citation can
   resolve to the wrong mechanism. Found **six** rotted citations, **all pre-existing and none caused by
   this change**; left untouched under one-feature-one-PR and reported for a follow-up (§12 Q4). The eight
   that resolve correctly include `substance-backed-admission.md`'s `client.go:34` and `result.go:71–72`
   / `:89`.

6. **Every DiVoid node id a document cites *as* a file's node, resolved against that node.** Added
   because this round shipped a header, a ledger row and a cross-reference all naming **#12822** as
   `anchor-grounded-recall.md`'s node. It is not: **#12822 is a 12,378-byte *ruling about* that document,
   and the document is #11753.** The id came from a ref line reading *"anchor-grounded-recall **ruling**
   #12822"*, where the load-bearing word was `ruling` — a document and a ruling about it carry
   near-identical names. **This is layer 5's defect class one level up:** a reference that resolves
   cleanly to the wrong target while looking checked. The check is `divoid_get_node(id)` and a comparison
   of its byte length against the blob; an 86,288-byte discrepancy is not subtle. **The answer was already
   on the branch** — `anchor-grounded-recall.md` §17.5 reads *"verifies this document against #11753"* —
   which makes this the cheapest available check and the one nobody ran. **And the token may not be swept
   globally:** `substance-backed-admission.md` carries **six** references to #12822 and every one is
   **correct**, citing the ruling as an evidential standard rather than as a file. Replacing the token
   everywhere would break six true statements to fix three false ones.

**What these layers structurally cannot reach.** (a) A claim carried only by a *number* — a table quoting
"20 candidates" as a measured historical reading is indistinguishable from one asserting current behaviour,
and only reading the surrounding prose separates them; several were left as dated records on that judgement
and a different reader may classify them differently. (b) Anything in the **test suite or the eval corpus**
asserting a property this predicate changes — the sweep covered `docs/` only, per the brief's boundary.
(c) Claims in **DiVoid nodes that are not repo documents** — #11365, #12966, #12968 and #11141 are cited
throughout as evidence and were not swept. (d) The **PR body**, which is the orchestrator's artifact.

**Sites carrying a correction, by document.** Each is a strike-in-place or an in-line amendment with a
dated note and a forward pointer that travels; nothing was silently rewritten and nothing was deleted.
**This table is a map for a reader, not a coverage claim** — it was itself incomplete on first writing:
running layer 4 again over the *edited* files found seven further rank-band sites in `hub-pruning` and
`anchor-grounded-recall` that the first pass had not reached, and they are included here only because that
second pass ran. Assume an eighth.

| Document | Node | Sites |
|---|---|---|
| `m1-skeleton-loop.md` | #10532 | TL;DR gloss; §1 success criterion S2; §6.3 the *cut-at-admission* rationale; §8.2 `candidates[]`; §11 R13 |
| `m2-retrieval-eval.md` | #10926 | TL;DR ordering promise; §5.3 `notRetrieved` verdict; §8.2 diagnostic row and delete test; §9 Guard 3's invariant; §11.3 (two); §12 E4; §13 G-19 |
| `m3-derived-recall.md` | #11235 | §4.3 `Retrieve` row; §7 R4 |
| `anchor-grounded-recall.md` | #11753 | §6.2; §10 R4; §13 Q3; §14 Unit 3's RETIRED banner; §16.8; §18.4.3; §18.7 (two); §18.8 |
| `hub-pruning-in-the-anchor-scope.md` | #12969 | §2 items 1 and 2; §4.1 diagram, property 3, Consequence; §5.1; §5.4 displacement bullet; §6; §6.1; §7 opening, table row and item 2; §8 table row and closing; §11; §13 O2 |
| `substance-backed-admission.md` | #12955 | §6.2 pass 1 and pass 2; §8.4 K6 |

**Statements deliberately left alone, because they are true and annotating a true statement is the failure
mode this project has already paid for.** M1 §9.4 obligation 1 (it constrains the record against the
candidate set, and never forbade this change — §6.3's *inference* from it is what was struck); M1's
correction-set lines *"the record still carries every candidate"*; M1 §6.1 step 4 and §5's diagram, which
stand as dated record of what M1 shipped and are superseded in M3, per M1's own 2026-09-05 correction set
(*"§6.2 stands as dated record of what M1 shipped and is superseded there, not here"*);
`substance-backed-admission.md` §8.1's adapter contract; its §15.3 note on the run-record **anchor**
(#12967), which this predicate does not touch; `anchor-grounded-recall.md` §16.3, §17.2(c) and §16.8's
Unit D, which are about oversized candidates, not records. **On Unit D, note the disagreement rather than
the strengthening:** #13092 reported that after the exclusion a node of 80,470 B (#10437) reached candidate
rank 13 — a slot the records had been hiding — while #13185 §4, hours later, found #10437 at scoped rank 6
and never a candidate at all. One reading, not reproduced. Unit D's own argument (#11753 §16.8) does not
depend on either.

---

## 12. Open Questions

| # | Question | Blocking? |
|---|---|---|
| **Q1** | Should `dispatchRecall` share the exclusion? §5 argues it needs its own decision and its own measurement, and no instrument reaches it. Leaving the asymmetry is a choice, not an oversight | No |
| **Q2** | Is the excluded-count integer (§6) worth a unit now, or after the next decision that would want it? Recommendation: **after** — build it when a proposal needs the number, so it is built to that reader | No |
| **Q3** | Does the retirement of Unit 3 survive re-derivation on the new placement (§7)? It rests on three measurements taken against superseded code. Recommendation: **re-run the §11 reopening measurement as restated before any anchor-scope work**, not before | No — but it blocks acting on `hub-pruning-in-the-anchor-scope.md` |
| **Q4** | **Six `file:line` citations are rotted at `bb09a2c`, all pre-existing and none caused by this change.** `m2-retrieval-eval.md`: `turn.go:131` for *"calls `WriteRun` unconditionally"* (the call is at `:159`); `report.go:208`, `:210` and `:211-212` for three print statements (they are at `:221`, `:223` and `:224-225`; `:208`–`:212` are struct fields). `m3-derived-recall.md`: `result.go:25-26` for `admittedCount`/`admittedBytes` (they are at `:27-28`), and `report.go:123-127` for *"`writeMisses` skips a row whose verdict is `Admitted`"* — that skip is at `:129-130` and lives in `missLines`, not `writeMisses`, so the citation names the wrong function as well as the wrong lines. Left untouched here under one-feature-one-PR. Follow-up task? | No |

---

## 13. Implementation Guidance for the Next Agent

**Nothing is to be implemented from this document.** The code shipped on
`fix/exclude-run-records-from-recall`; this is its decision record and the reconciliation of the documents
it falsified. If any of §12 is taken up, each is its own unit and its own PR, in this order: Q3 (a
measurement, no code), Q2 (one field, `Retrieve` signature, sweep report), Q1 (a decision that needs Q2's
instrument first, because otherwise the supplementary path goes dark the same way the primary one did).
