# Architectural Document: Anchor-Grounded Recall

**Status:** proposal · **Author:** Sarah (architect) · **Date:** 2026-09-05
**Prompted by:** #11414 (three step traces, Finding 2) · **Constrained by:** #11398, #11365, #11364
**Target unit:** the retrieval step of the turn (`internal/loop/retrieve.go`), one PR.

---

## 1. Problem Statement

For the task *"Add an HTTP endpoint to this service that returns the build version"*, the turn's recall
returned fourteen of seventeen slots as `HealthController.cs` documentation nodes from fourteen different
mamgo netcore services — a different project, a different language, a different repository — spread across
0.008 of similarity. The one document that answers the task, **#10466**, ranked 18th, roughly 0.06 *below*
every one of the fourteen wrong ones, and entered the candidate set only because slots 18–20 are reserved
for the anchor's two-hop neighbourhood.

The question this document answers is: **how should a turn's retrieval know what is relevant to this task?**

The finding that constrains every answer — stated in #11414 and independently confirmed here in §3 — is that
**no re-ranking of the returned list reaches the right document.** It is not mis-ordered among plausible
candidates; it is below fourteen wrong ones by a margin far larger than the spread separating them. Better
fusion, better embeddings, a longer candidate list: none of these reach it.

**The recommendation is that the constraint has a seam, and the seam is upstream of the list.** Re-ranking a
returned list cannot reach #10466. *Changing what was asked* can, and does — the same node moves from rank 18
to rank 2 when the query is grounded in the anchor the caller deliberately chose. This is not re-ranking, and
#11414's constraint does not cover it.

**Success criterion:** on the task shape #11414 exhibits, the node that answers the task is admitted into the
block rather than cut or narrowly rescued, without a project filter, without new metadata on the graph,
without a model call in the retrieval path, and without raising the byte budget.

---

## 2. Scope & Non-Scope

**In scope**

- How the query set for a turn's unscoped recall is constructed.
- Whether the anchor node may appear in its own candidate set.
- The consequences for the eval sweep, which shares the retrieval seam.

**Explicitly out of scope**

| Out of scope | Why, and where it belongs |
|---|---|
| Compaction at admission | Ruled and queued: #11365 §3. Independent of this change and complementary to it. |
| Near-duplicate collapse | Real, distinct from compaction (#11414 is right about this), and rejected *as the answer to this problem* in §9.4. A later unit. |
| Raising `AssemblyByteBudget` | #11364 makes the budget the product. Raising it concedes the question. |
| Partitioning the graph per project | The shared graph is deliberate and is not changing. A design requiring partition answers a different question. |
| #11414 Finding 1 (a task answered as a question, reported `answered`) | A distinct defect in the judgement/terminal contract. §11 notes the one point where it touches this design. |
| Anything requiring a model call inside retrieval | M3 §4.3 fenced retrieval as model-free by construction. This design honours that fence. |

---

## 3. What I verified myself, and what changed my recommendation

The brief instructed me to establish the constraint rather than inherit it. I did, on the live graph on
2026-09-05. Retrieval is deterministic given a graph state, so these rank observations do not depend on
sampling. **The graph has moved since #11414 was written** — a run record of t1 (#11391) now ranks 6th on t1's
own recall and has joined the anchor's neighbourhood, which is #11141's self-poisoning, live.

### 3.1 The constraint holds

Re-running t1's unscoped recall at depth 22 reproduces #11414 almost exactly: **17 of 22 rows carry
`rootNodeId = 3526`** — one group, *Repo Map — mamgo-backend* — at 0.7358 → 0.7237. **#10466 is not in the
first 22 at all.** The constraint is real and is not an artefact of one run.

### 3.2 The scoped arm already knows the answer — and that is a trap

Running t1's query restricted to the anchor's existing two-hop scope returns **#10466 at rank 1**, at the same
0.6708 that was globally 18th. The two-hop scope of anchor #10456 contains **34 nodes total**, all Processor
material. Repeated on t3's anchor: #10466 again at scoped rank 1. On t2's anchor: the required #10435 at
scoped rank 4.

This looks like a decisive argument for giving the scoped arm a larger share of the candidate order. **It is
not, and I nearly recommended it before checking.** `docs/architecture/m3-derived-recall.md` §4.2 records that
this family was measured on 2026-09-04 and rejected:

| Variant | Measured | Verdict |
|---|---|---|
| Reserve at 5, 7, 10 slots | 12/13 — no gain; at 10, r02 falls out | rejected |
| Rank-interleave, 50/50 fused vs scoped | 13/13, **but r01 drops 1→8, r02 4→7, r04 3→7** | rejected as a coincidence bought with general degradation |

And the later admission curve (#11365 §1 — budget 78% spent by rank 6; P(admit) 1.00 at rank 1, 0.39 at 9–10,
0.09 at 12) makes that rank cost **worse** than it looked when the rejection was written. The 2026-09-04
ruling was more right than it knew. **Any fixed re-allocation between the two arms is a cancellation, not a
gain** — the same disease #11365 §5 diagnosed in the per-candidate size bound.

### 3.3 The mechanism that is not in the rejected family

Every one of #11414's three tasks contains an **unresolved reference to the anchor**: *"this service"*, *"this
repository"*, *"this codebase"*. The embedding of such a sentence has no way to know what *this* is, so it
lands in the neighbourhood of every health-controller document in every codebase — which is literally what
the sentence is about, absent its referent.

**The anchor is the referent, and the caller chose it deliberately.** The turn uses it for exactly two things:
it is rendered at the head of the block, and it contributes three tail slots via its neighbourhood. It is
never used to say *what the input is about*.

Composing the anchor's identity into the query text resolves the reference **into the embedding**, upstream of
the list. Measured, unscoped, on the live graph:

| Task | Required node | Today | Anchor-grounded query |
|---|---|---|---|
| **t1** *"Add an HTTP endpoint to this service…"* | #10466 | **18** (survived only by the reserve) | **2** (0.7567) |
| **t2** *"Set up continuous integration for this repository."* | #10435 | **18** (cut on byte budget) | **5** (0.7044) |
| **t3** *"…where should a function… live in this codebase"* | — | #10466 at 13, cut on budget | not in top 8; **all eight head slots are project material** |

On t1 the fourteen `HealthController.cs` nodes fall to ranks 13–20 — **below the answer** — while the two
genuinely relevant cross-project task nodes (#10824, #10900) are *preserved* at ranks 4 and 5. On t2 the
Pooshit/mamgo CI cluster is almost entirely displaced; one foreign node (#9399) survives at rank 9 and ranks
1–8 are Processor.

**t3 is reported as a partial result and is not claimed as a win.** The head became project material; the
specific document did not surface in the first eight.

### 3.4 The harm check, run before recommending

The rows most at risk are the sweep rows whose required node is admitted at rank 1 today.

| Row | Required | Today | Anchor-grounded | Reading |
|---|---|---|---|---|
| **r01** | #10861 | rank 1, admitted | **rank 2** — and rank 1 is the anchor itself | **Held.** With the anchor excluded (§6.2) it is rank 1, unchanged. |
| **r10** | #10943 (32,105 B) | rank 1, admitted | **rank 4** | **At risk.** This is the row that killed the size-bound proposal in #11365 §5. |
| **r21** | #11142 | unreached at depth 400 | still unreached | **No gain.** Thin, not crowded — as #11365 §8 classifies it. |

r01 and r10 are *self-contained questions* with no deixis. r21 is a thin row. This is the predicted pattern,
and it is the basis of the explanation-falsifier in §12.3.

### 3.5 A finding that becomes a precondition

In **five of six** grounded queries I ran, the anchor itself returned at rank 1 or 2. Assembly already renders
the anchor and then admits it again as a candidate — a known defect, and precisely r05's required node
(#10927). Today it is a probabilistic waste. **Under anchor-grounded queries it becomes near-certain**, and on
r05 — whose anchor is 70,660 B — it would consume the whole block twice over.

**Excluding the anchor from its own candidate set is therefore not adjacent polish; it is a precondition of
this change and belongs in the same unit.**

---

## 4. Assumptions & Constraints

| # | Assumption / constraint | Confidence |
|---|---|---|
| A1 | The graph is shared across every project and will remain so. | Given by the operator. |
| A2 | The subject/anchor is chosen deliberately and is usually correct. | Stated in the brief. **This design increases the cost of a wrong anchor** — see R2. |
| A3 | Anchor names are short and topical. Observed range across every anchor inspected: ~10–110 characters. | Measured on the anchors used here; **not** established across the whole graph. Open question Q1. |
| A4 | The retrieval path stays model-free. | M3 §4.3, deliberate. Honoured. |
| A5 | The byte budget is the product, not a constraint to relax. | #11364. |
| A6 | Recall is deterministic given a graph state. | Confirmed — every rank figure here is reproducible against one graph state, not against a later one. |
| A7 | Similarity alone cannot separate the answer from the ballast in the returned list. | #11414, reproduced in §3.1. **This design does not contradict it** — it changes the query, not the ranking. |

---

## 5. Architectural Overview

The change adds **one query source** and **one exclusion** to the existing retrieval step. Nothing else moves.

```
  input, subject
      │
      ├─ Graph.Node(subject) ──────────────► anchor        (unchanged)
      │
      ▼
  ┌───────────────────────────────────────────────────────────────────┐
  │  RETRIEVAL STEP                                                    │
  │                                                                    │
  │   queries the caller supplied ────────────┐                        │
  │                                            ├── unscoped fan-out    │
  │   ANCHOR-GROUNDED QUERY  ◄── NEW ─────────┘   (one recall each)    │
  │     composed here, from the anchor this step already receives      │
  │                                                                    │
  │   ── reciprocal-rank fusion over the unscoped lists ──             │
  │      (already implemented; INERT IN A TURN TODAY — #11398)         │
  │                                                                    │
  │   anchor-scoped recall ──► 3 reserved tail slots  (UNCHANGED)      │
  │                                                                    │
  │   ── EXCLUDE THE ANCHOR'S OWN ID  ◄── NEW ──                       │
  └───────────────────────────────────────────────────────────────────┘
      │
      ▼
  Assemble → judge → write back                          (UNCHANGED)
```

**Three properties make this the shape I would defend.**

1. **It is upstream of the constraint.** #11414's finding is about re-ranking a returned list. This changes
   what is asked. The measured effect (§3.3) is what that distinction buys.
2. **It activates machinery that already exists and is currently doing nothing.** #11398 established that
   reciprocal-rank fusion is *inert in a turn*, because the turn passes exactly one query and fusion over one
   list is order-preserving. This change gives the turn a second list — **the first real job the product's
   fusion step has ever had** — and narrows the turn/sweep ranking divergence #11398 §2 names as a defect in
   the instrument.
3. **It retains the raw arm rather than replacing it.** This is what bounds the damage on r10: fusion scores
   *agreement*, so a node at raw rank 1 and grounded rank 4 carries more mass than a node at raw rank 2 and
   grounded rank 13. The grounded query is a **vote, not a substitution.**

---

## 6. Components & Responsibilities

### 6.1 The query-composition rule (new, pure)

- **Owns:** turning an anchor and the caller's query set into the query set actually issued.
- **Does not own:** issuing recalls, fusion, scope construction, admission.
- **Placement:** inside the retrieval step, **not** in the turn and **not** in the sweep. This is the same
  argument M3 §4.3 already made for scope construction — *"the caller passes the anchor and never a scope, so
  exactly one copy exists in the tree"*. A caller-builds reading would put a second copy in the sweep and the
  instrument would stop measuring the product. **Both callers inherit the behaviour for free.**
- **Composition:** the anchor's identifying text (its name, optionally its type) joined with the caller's
  primary query, as one additional query. **Bounded by construction** — the anchor's *body* is never used, so
  a 70 KB anchor contributes the same handful of characters as a small one.
- **Purity:** no I/O, no clock, no randomness — on `Assemble`'s discipline.

### 6.2 The anchor exclusion (new)

- **Owns:** guaranteeing the anchor's id does not appear among the candidates the retrieval step returns.
- **Why it is here and not in assembly:** the candidate list is also the record's `candidates[]`, and a
  disposition row for a node already rendered whole at the head of the block is a record that misreports what
  the block contains. Excluding at the source keeps one truth.
- **Not owned:** removing the anchor from the *block*. The anchor renders whole and is never cut (#11335,
  ruled by #11365 §4).

### 6.3 Unchanged, deliberately

The scoped arm and its reserve of three; reciprocal-rank fusion and its constant; the candidate limit; the
byte budget; admission, the skip rule, the self-produced cut; the anchor-first block layout; the
supplementary-recall loop; every terminal reason. **No constant is retuned and no threshold is introduced.**

---

## 7. Interactions & Data Flow

For one turn, in order:

1. The turn resolves the subject to an anchor. *(unchanged)*
2. The turn hands the retrieval step the anchor and its query set — for a turn, the raw input alone.
3. **The retrieval step composes the grounded query from the anchor it was given.** *(new)*
4. It issues one unscoped recall per query — now two for a turn, where it was one.
5. It constructs the anchor scope and issues one scoped recall on the primary query. *(unchanged)*
6. It fuses the unscoped lists by reciprocal rank. **For a turn this is now a real fusion of two lists rather
   than an identity transform over one.**
7. It fills the head from the fused order, reserves three tail slots for the scoped list, backfills.
   *(unchanged)*
8. **It removes the anchor's own id wherever it appears.** *(new)*
9. Assembly, judgement and write-back proceed untouched.

**Cost:** exactly one additional graph read per turn. No model call. No new endpoint, no new field on any node,
no schema change, no write.

---

## 8. Contracts & Interfaces (Abstract)

| Contract | Statement |
|---|---|
| **Composition is total** | Every anchor yields a grounded query. An anchor with an empty name degrades to the raw query, which duplicates an existing list; fusion over duplicate lists is order-preserving, so the run reduces to today's behaviour rather than failing. |
| **Composition is bounded** | The grounded query's length is a function of the anchor's *identity*, never its body. No anchor size can make the query large. |
| **Composition is deterministic** | Same anchor, same input, same query — a precondition of the sweep remaining a reproducible instrument, and of pinned derivations remaining pinnable. |
| **The raw arm is preserved** | The caller's queries are always issued. The grounded query is added, never substituted. This is what bounds regression to fusion mass rather than replacement. |
| **The anchor is absent from candidates** | The returned candidate list contains no row whose id is the anchor's, on any path — fused, reserved, or backfilled. |
| **Record fidelity** | The record must carry which queries were issued and, per candidate, which query returned it and at what rank. Without this, a reader cannot tell a node that surfaced on both arms from one that scraped in on the reserved scope slot — and #11066's rule applies: evidence not written into the result is unrecoverable, because the next read queries a different graph. **This obligation is already stated in M3 §4.4 and is not new; this change makes it binding rather than anticipatory.** |
| **Seam parity** | The turn and the sweep obtain the grounded query by the same construction. Neither may compose its own. |

---

## 9. Quality Attributes & Trade-offs — and the alternatives rejected

### 9.1 What is traded

| Attribute | Effect |
|---|---|
| **Retrieval quality on deictic tasks** | Large measured gain: rank 18 → 2 and 18 → 5 (§3.3). |
| **Retrieval quality on self-contained questions** | Approximately neutral, with one measured demotion (r10, 1 → 4). |
| **Latency / cost** | One extra graph read per turn. No model call. |
| **Sensitivity to anchor choice** | **Increased, deliberately.** See R2. |
| **Complexity** | One pure composition rule and one exclusion. No new constant, no threshold, no knob. |

### 9.2 Rejected — hard project or scope membership as a filter

**Rejected on measurement, not taste.** The only membership field the graph exposes on a recall row is
`rootNodeId`, and it is **null on the majority of the class we want to keep**: #10435 (t2's own answer),
#10824 and #10900 (t1's two genuinely relevant task nodes), and every QA review and session-log node in the
Processor neighbourhood. A hard filter deletes them. Under the grounded query those same nodes survive at
ranks 4 and 5 *without* any filter.

*Would win if:* cross-project answers were never needed and grouping were total across the graph. Both are
false, and the first is ruled out by the shared-graph constraint.

### 9.3 Rejected — membership as a rank signal or additive boost

Same null-coverage problem: a boost on a field absent from half the target class fires on the wrong half. It
is also a fitted constant, and this project's own history (#11365 §5) is that fitted constants on this corpus
cancel rather than pay.

*Would win if:* grouping were made total — a substrate change the operator has ruled out.

### 9.4 Rejected — near-duplicate collapse as the answer here

#11414 is right that deduplication is a distinct mechanism from compaction, and that fourteen near-identical
documents compacted individually are still fourteen near-identical documents. But **measured against t2 it
does not fire**: t2's ranks 1–13 are NuGet, GitHub-Actions and CI nodes from `Pooshit.Json`,
`Pooshit.AudioSynth`, `Pooscript`, mamgo-jobs and messe-frontend — *topically* clustered, not near-duplicates.
Collapse fixes t1 and not t2, so it is not the general mechanism. It also requires a similarity threshold,
which is the knob this design avoids.

*Verdict:* **deferred, not dismissed.** It is a byte-efficiency mechanism and belongs after compaction
(#11365 §3), scored on documents-per-block with admitted-not-down as its guard.

### 9.5 Rejected — rebalancing the two arms

Measured and rejected on 2026-09-04 in every form tried: reserve at 5, 7, 10; and the 50/50 rank-interleave.
See §3.2. #11365 §1's admission curve strengthens that rejection.

*Would win if:* the answer were reliably inside the anchor's two-hop scope. Measured false — the required node
is a 1-hop neighbour on **2 of 14** sweep miss rows (#11365 §6).

### 9.6 Rejected — deeper candidate list, better fusion, better embeddings

#11414's core finding, reproduced in §3.1; and #11365 §5 measured a five-fold deeper candidate limit moving
`retrieved` only, never `admitted`, with all six new retrievals arriving cut.

### 9.7 Not proposed — derived queries

**This is the family the brief warns about, and the distinction must be exact.** #11365 §6 measured and
rejected **model-derived reformulations of the input, applied to the scoped arm**: 0 of 14 miss rows reached
the reserved slots under any query, and the symmetric fused variant changed nothing end to end.

The grounded query is **not** that mechanism. It is not model-generated, requires no sidecar, is a
deterministic function of a node the step already holds, and is applied to the **unscoped** fan-out. It is a
new query *source*, not a reformulation of the existing one.

That said, one warning from that work transfers and is recorded rather than dismissed: **#11365 §8 found that
fusion demoted r13, r16 and r23 because RRF scores agreement, and five queries derived from one input agree
about that input's concrete surface.** With two lists rather than six the effect is far weaker, but it is the
mechanism by which this change could quietly harm a row, and F2 is where it would show.

---

## 10. Risks & Mitigations

| # | Risk | Mitigation |
|---|---|---|
| **R1** | **r10 loses its answer.** #10943 demotes 1 → 4 (measured) and is 32,105 B; at rank 4 behind three predecessors it may not fit. This is the exact trade that killed the size bound. | Pre-registered as the **hard falsifier** F2. Not mitigated away — if it fires, the change is reverted or restricted. |
| **R2** | **A wrong anchor now steers half the fan-out**, where today it costs only three reserve slots. | Partly intended: A2 says the anchor is chosen deliberately. Bounded by retaining the raw arm — a wrong anchor loses fusion mass, it does not replace the list. Recorded as a genuine increase in sensitivity, not argued away. |
| **R3** | **Anchor names that are long, generic, or uninformative.** A generic name is a no-op; a very long one could swamp the input's signal. | Only identity text is used, never the body. Q1 asks for the name-length distribution across the graph before this is called bounded in general rather than in the anchors measured. |
| **R4** | **Self-poisoning compounds.** A run record of this input already ranks 6th on t1's own recall and has joined the anchor's neighbourhood (#11141, live in §3). A grounded query names the anchor, and run-record names embed the input — so grounded queries may match run records *more* strongly. | The self-produced cut already exists at admission. **But it is a cut, not a rank exclusion**, so poisoned rows still consume candidate slots. Flagged as Q3; not solved here. |
| **R5** | **The sweep's baselines are superseded.** Both callers change ranking, so 11/23 retrieved and 9/23 admitted stop being the comparison point. | Intended — the instrument should measure the product (#11142). Both arms must be re-run and the new baseline recorded in the same session, not inferred. |
| **R6** | **Design-document parity.** This adds a row to the combiner table in M3 §4.2 and changes §4.1's shape diagram and §4.3's placement table. | M3 is `docs/architecture/m3-derived-recall.md`, graph node **#11235**. M1 (#10532) is touched only descriptively. **Q4: is M3 under the same P-40 parity rule as M1?** If so this needs a parity publish. |

---

## 11. The measurement that decides it

This project's standing rule is testing over theory, and it has been burned repeatedly by plausible mechanisms
that measured flat. **All four falsifiers below are pre-registered here, before the change exists.**

### 11.1 F1 — the effect. Deterministic, no model, runnable today.

Re-run the three #11414 tasks through the rank instrument and read, for each, the required node's **position
in the candidate order handed to admission** and its **admit/cut disposition**.

- **Prediction:** t1's #10466 and t2's #10435 are both **admitted**, from "survived only by the reserve" and
  "cut on byte budget" respectively.
- **Falsifier:** *if either remains cut, the change does not do the thing it is proposed for.* Rank movement
  alone is not the claim — admission is.

### 11.2 F2 — the harm guard. The existing sweep corpus, used for the one thing it is good at.

Run the 23-row sweep on both arms at one graph state, in one session.

- **Prediction:** `admitted` does not fall below **9/23**, and specifically **r01 and r10 both keep their
  required node admitted.**
- **Falsifier (hard):** *if r10 loses #10943, anchor grounding is displacing answers at the head exactly as the
  per-candidate size bound did, and the change is rejected.* This is a live measured risk (§3.4), not a
  formality.
- **Secondary falsifier:** if any row currently admitted at fused rank ≤ 4 loses its required node, the change
  is rejected regardless of what it gains elsewhere.

### 11.3 F3 — the explanation. The falsifier that separates a design from a lucky tweak.

Partition every corpus row by whether its input contains an **unresolved reference to the anchor** (*"this
service"*, *"this repository"*, *"our harness"*) or is a **self-contained question**. Label the partition
before running.

- **Prediction:** rank improvement is **concentrated on the deictic partition and approximately zero on the
  self-contained partition**, because the mechanism claimed is reference resolution.
- **Falsifier:** *if improvement is uniform across both partitions, the deixis explanation is wrong.* The
  effect may still be real — but the design's account of **when it applies** would be false, and the next unit
  would be built on a wrong model of the system. This must be looked for, not assumed away.

### 11.4 F4 — the standing claim, free on every future run.

*Reciprocal-rank fusion is no longer inert in a turn* (#11398), and *no candidate list contains the anchor's
own id.* Both are true by construction; one counterexample retires them.

---

## 12. What the existing measurement can and cannot support — stated plainly

**The brief asked for honesty here, and this is the most useful thing in the document.**

**#11414 is three chosen tasks, one run each.** It is an existence proof, not a distribution. My reproduction
today is deterministic on *retrieval only*, against *one* graph state — and the graph has already moved under
it (§3). **No figure here is a rate.**

**I have measured rank, not answers.** Whether a better-ranked block produces a better answer is unmeasured by
everything above and needs the comparison protocol (#11092 / #11319 / #11349), which is single-draw and
model-dependent.

**The 23-row sweep corpus cannot provide positive evidence for this change, and using it as though it could
would be a measurement error.** #11365 §2 records that it has **two** admission failures in twenty-three rows;
#11365 §6 records that the required node is a 1-hop neighbour on **2 of 14** miss rows. Its rows are
*self-contained questions* — **exactly the partition on which F3 predicts approximately zero effect.** Running
this change on that corpus and reporting a flat result would be measuring the mechanism on the population where
it is predicted not to fire, and would falsify nothing.

**It can provide evidence *against*.** That is F2, and that is the correct and only use of it here.

**Therefore: a new instrument is required before this change — or any successor to it — can be judged
positively.** This is a legitimate deliverable in its own right and I recommend it as the unit immediately
after this one:

- **Shape:** anchored, project-situated **task** rows — imperative work-on-this-codebase inputs, the shape
  #11414 exhibits and the shape the product is actually for — with required nodes pre-registered before any
  run.
- **Size and spread:** enough rows to be a distribution rather than an existence proof, spanning **at least two
  projects**, so cross-project crowding is present *by construction* rather than by luck.
- **Discipline:** authored blind, under #11101's corpus-growth rule. **#11360's trap covers r13 and r16–r23 —
  these must be new rows, not regenerations of rows that were seen to miss.** The twelve diagnosis rows are
  burned; anything measured on them from here is selection.
- **Why it cannot be deferred:** the *next* retrieval decision after this one is not decidable on any
  instrument the project owns. This change is shippable now only because its harm guard (F2) runs on the
  existing corpus and its effect (F1) is already measured at the rank level on the three tasks. That is a
  one-time affordance and it does not extend to the unit after.

---

## 13. Open Questions

| # | Question | Why it matters | Blocking? |
|---|---|---|---|
| **Q1** | What is the distribution of node-name lengths across the graph? | A3 assumes anchor identity text is short. Measured on the anchors used here (~10–110 chars); not established generally. If a long tail exists, composition needs a stated bound. | No — a bound can be added without redesign. |
| **Q2** | Should the anchor's **type** be composed in alongside its name? | Type may sharpen a generically-named anchor and may add noise to a well-named one. Untested. | No — decide by measurement in F1's run; both variants are one composition rule apart. |
| **Q3** | Should self-produced run records be excluded from the **ranking** rather than cut at **admission**? | They currently consume candidate slots before being cut (R4), and grounded queries may match them more strongly because run-record names embed the input. | No, but it should be a task. |
| **Q4** | Is `docs/architecture/m3-derived-recall.md` (#11235) under the same P-40 parity rule as M1 (#10532)? | This change edits M3 §4.1, §4.2, §4.3. If parity applies, the operator handles the publish. | **Yes — needs an answer before the PR is opened.** |
| **Q5** | Does the operator want the sweep's pinned derivation sidecar re-pinned after this lands? | The grounded query is composed inside the retrieval step, so it is *not* a sidecar entry — but the sweep's arm identity and reported hashes change. | No — but the new baseline must be recorded, per R5. |

---

## 14. Implementation Guidance for the Next Agent

**One feature, one PR. This is Unit 1 and it stands alone.**

### Unit 1 — anchor-grounded recall (this document)

Ordered milestones. No step introduces a tunable constant.

1. **Exclude the anchor from its own candidate set**, on every path — fused, reserved, backfilled. Land this
   *first*: it is a precondition (§3.5), it is independently correct today, and landing it first means the
   grounding change is measured against a clean baseline rather than against a defect it would amplify.
2. **Add the query-composition rule** as a pure construct inside the retrieval step — deterministic, bounded by
   the anchor's identity text, never its body, total over every anchor including the empty-name degradation.
3. **Issue the grounded query as one additional unscoped recall**, fused with the caller's lists by the existing
   reciprocal-rank step. The scoped arm, its reserve, and every other constant are untouched.
4. **Extend the record** to carry the queries issued and, per candidate, which query returned it at what rank
   (§8, record fidelity). Without this the change is unmeasurable after the fact and F3 cannot be scored.
5. **Confirm both callers inherit the behaviour from one construction** — the turn and the sweep. A second copy
   in the sweep is the drift hazard M3 §4.3 was written to close, and it would be invisible to any
   single-package test.
6. **Run F2 before F1.** The harm guard is the gate. If r10 loses #10943, stop and report; do not proceed to
   tune around it.
7. **Run F1 and F3**, and record the result — including a flat or negative one — as a linked node.

**Worth on its own, independent of any successor:** t1's answer moves 18 → 2 and t2's 18 → 5 (measured);
r01 holds; the product's fusion step does real work for the first time (#11398); the turn/sweep ranking
divergence narrows; and the latent double-anchor defect (#10927 / r05) is closed.

### Unit 2 — the task-shaped corpus (§12)

Not optional, and not a documentation task. It is the instrument without which the unit after this one cannot
be judged. Author blind, pre-register required nodes, span two or more projects, honour #11360's trap.

### Unit 3 — hub pruning in the anchor scope

**Measured motivation, not speculation.** t2's two-hop scope is **362 nodes** and its scoped list still
returned mamgo and Pooshit CI material at ranks 1–3 — because the project node links to `person Toni` (#10),
and two hops through a person reaches every project that person runs. t1's scope, which passes through no
person, is **34 nodes** and is entirely clean. Pruning hub nodes from the neighbour set sharpens the scoped arm
this unit deliberately leaves alone. Predicted: t2's scoped rank 4 → 1 or 2. Should be measured, not assumed.

### Unit 4 — near-duplicate collapse at admission

After compaction (#11365 §3). Scored on documents-per-block, guarded on admitted-not-down. §9.4 for why it is
not the answer to *this* problem.

---

## 15. Refs

Measured traces **#11414** · fusion inert in a turn **#11398** · admission triage **#11365** · constrained
context **#11364** · vision **#10424** · self-poisoning **#11141** · the loop **#10850** · M3 design **#11235**
· M1 design **#10532** · shared seam **#11259** · corpus growth **#11101** · corpus trap **#11360** · anchor
budgeting **#11335** · nodes larger than the budget **#11308** · comparison protocol **#11092** / **#11319** /
**#11349** · repo map root **#10454**.
