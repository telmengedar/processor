# Architectural Document: Retrieval Admission and the Empty Outcome

**Status:** design, not implemented. **Author:** sarah-software-architect, 2026-09-07.
**Repo parity.** This file is under **P-40 parity** with its DiVoid node. The two must move together;
the operator publishes both sides.

**Binds under #12961 M-1.** Every mechanism proposed here names its own differential falsifier in the same
passage that states it, runnable before the mechanism is commissioned. Where a falsifier has *already run*,
that is said and the result cited. Where one has *not*, it is named as an obligation on the unit and the
unit is not to be started until it has fired.

**Reasoned from a failing product (#12978).** The product fails, the failure is measured six ways
(**#13091**, **#13093**, **#13101**), and each unit below is tied to the specific measured mechanism it
addresses. No unit here is polish.

---

## 1. Problem Statement

The harness's yardstick task — *"Generate a new barebones webpage and a repo for it."* — is a 51-character
request that arrives as the **last 51 characters of a 58,806-character user message**. Everything before it
is retrieved memory. Four of six live runs produced a Go project and zero HTML; the two that produced a
webpage did so on a **79-byte** margin of assembly budget (#13093 §1).

Three independently measured facts define the problem:

| # | Fact | Source |
|---|---|---|
| 1 | Ranked with a sane type filter, **nothing in the graph reaches 0.70 on this task text**. Ceiling **0.6887**. Every document the harness admitted was below the operator's noise floor. | operator measurement |
| 2 | **A similarity floor alone cannot fix it.** The one useful document, `#1487`, scores **0.6301** — the *lowest* of all twenty candidates. Any floor that removes the Go build spec `#10435` (0.6378) removes `#1487` first. | #13093 §6 |
| 3 | **The largest single lever is not similarity at all.** Moving the request out of the tail flips the outcome 0/3 → 3/3 webpage and drops Go output to zero. Emptying the recall result flips the *untouched* baseline 0/3 → 2/2. | #13101 |

So the system has two defects that look like one:

- **It cannot tell a hit from noise** — the score band across ten eligible documents is 0.061 wide, mean
  neighbour gap 0.0068 (#13093 §5). Similarity is not carrying a decision.
- **It cannot fail.** There is no path by which "I have nothing useful" becomes an outcome. The loop's own
  closed set of terminal reasons (`Answered`, `WantsRecall`, `Truncated`, `Refused`, `WantsWrite`,
  `Unrecognised`) contains **no member for "I need clarification"**. Even if the model produced that
  behaviour, the harness would record it as `Answered`.

**Success criteria.** Both outcomes reachable and both measurable: the harness **acts** when it has what it
needs, and **asks** when it does not, and a run record makes it possible to tell afterwards which one
happened and whether it was right.

> ### 1a. The baseline caveat — how to read every measurement in this document
>
> **Operator ruling, 2026-09-07.** *"We shouldn't draw hard conclusions based on a loop where several points
> obviously go against our vision and plan. Currently it's a foundation for measurement and the baseline
> shows a bad result — but we have steps in our vision which should improve that, so let's follow them."*
>
> The loop that produced #13091/#13093/#13101 **violates several of the vision's own commitments**: it
> pushes full bodies where condensation was always intended, it has no floor, it has no empty outcome, and
> it injects the anchor twice. It is a **foundation for measurement**, not a realisation of the design.
>
> **Therefore:** measurements *of* the baseline are valid and are used throughout. Conclusions *from* the
> baseline about **what the intended design can achieve** are not licensed and are not drawn here. Where a
> finding depends on the current loop being representative, it is marked **[baseline]** and must be re-read
> after the unit that changes the relevant commitment.
>
> The three places this bites hardest:
> - **The 0.061-wide score band** (#13093 §5) is measured against *one query text*, formed by a prompt whose
>   defect Unit 1 removes. **[baseline]**
> - **"A floor alone cannot work"** is a true statement about *that admitted set*. It rules out a floor as a
>   *primary* mechanism; it does not bound what a floor does alongside better queries and a catalogue.
>   **[baseline]**
> - **The crowding arithmetic** (one 43 KB document taking 76.6% of budget) is measured on **uncondensed
>   bodies**. §8.3 estimates that condensation alone shrinks that document to ≈12.8 KB. **[baseline]**

---

## 2. Scope & Non-Scope

**In scope.** Candidate admission (what enters the block and why); the supplementary-recall round; where the
request sits in the user message; a representable clarification outcome; the instrument that lets the two
outcomes be told apart.

**Out of scope, named so that declining them does not lose them.**

- **Embedding quality, re-ranking models, query expansion by a second model.** The score band is narrow, but
  no evidence here says the embedding is the fault — §4 shows a *self-formed* query reaches 0.73–0.76 on the
  same corpus. Changing the embedding is a much larger purchase against an untested account (M-1).
- **The `ANCHOR` / `CANDIDATE` vocabulary.** Explicitly excluded from Units 1–3 and explained in §4. This is
  a departure from the operator's stated point 2 and the reason is measured, not preferential.
- **Changing the condensation pass (`internal/condense`, PR #44).** This design **consumes** it and must
  not fork it. **Running** it over the retrievable corpus is *in* scope as Unit 3's prerequisite (§8.3, Q5);
  altering how it condenses is not. See §5 constraint C4.
- **The write-back self-poisoning problem** (#11141) beyond the exclusions already shipped.
- **Multi-turn conversation.** The loop is one turn.

---

## 3. What the evidence already rules out — do not re-propose

This section exists so that no later reader re-derives a dead account. Each row is a *measured* exclusion.

Read with §1a: these exclusions are **measured on the baseline**. Each rules out an account *as an
explanation of the observed failure*. None of them bounds what the intended design achieves once the
vision's own commitments — condensation, a floor, an empty outcome, a single injection of the anchor — are
actually in place.

| Ruled out | Why | Source |
|---|---|---|
| **A similarity floor as the *primary* fix** | The one useful document is the lowest-scoring eligible candidate. A floor drawn anywhere between 0.6301 and 0.6378 removes the *good* document before the *bad* one. A floor above 0.6887 admits nothing. **Rules out floor-as-mechanism; does not rule out a floor** alongside better queries and a catalogue — which is how Unit 2 uses it. **[baseline]** | #13093 §6 |
| **"The graph holds nothing relevant, so clarification was correct here"** | `#6309` (0.760) and `#6311` (0.732) are directly relevant and reachable **by a query the system formed unaided**. The correct behaviour on this task was *retrieve better*, not *give up*. | #13093 §4 |
| **"The matches were too weak"** | The two runs that *succeeded* had the **lower** mean admitted similarity (0.65300 vs 0.65518) and the **lower** floor (0.6301 vs 0.6378). The account dies on its own differential. | #13093 §3 |
| **Re-labelling the anchor in the system text** | Arm D rewrites the exact sentence that mislabels the anchor as *"the subject of this request"* and changes nothing (0/3 webpage in PRIMARY). Under CONTROL it writes a page titled for a *different graph node*. Telling the model the block may be irrelevant does not tell it where the request is. | #13101 arm D |
| **"Any prompt perturbation would work"** | Arm E deletes a whole system sentence and changes nothing, 0/5 across both conditions. The two arms that move are the two that move the instruction. The shape account is not vacuous. | #13101 arm E |
| **Crowding (byte volume) as a sufficient explanation** | Arm F removes 43 KB and flips the artefact in PRIMARY — real work — but under CONTROL it **denies the request exists** while the request sits in its own user message. Crowding relief is not robust on its own. | #13101 arm F |

---

## 3a. What Gangolf actually established about thresholds

The operator asked for prior thinking in Gangolf (#902) rather than inference from two titles. Mined
directly; ~30 node bodies read. **Both starting pointers turned out to need correction**, and the most
valuable finding was in neither of them.

> **How this section binds — operator ruling, 2026-09-07.** *"Rules for Gangolf could not apply for
> Processor since Gangolf is a reactive chatbot, not a harness. We take experiences there as anchors but
> don't necessarily come to the same conclusion."*
>
> Gangolf is a **reference, not a blueprint**. Everything below is evidence from a *different product with
> a different job*: a reactive chatbot whose purpose is to keep a conversation going, versus a harness whose
> purpose is to execute a task correctly or decline. Where Gangolf's *mechanism* failed, the lesson
> transfers. Where Gangolf made a *product decision*, it is evidence to reason about, **never a constraint
> this design must justify itself against.** Each finding below is marked accordingly.

**The two named pointers, corrected.**

| Pointer | What the title says | What the body establishes |
|---|---|---|
| **#483** | "Stage2MinCosine 0.5 too tight" — read as a Gangolf recall threshold | **Not Gangolf.** `rootNodeId 69` = project *Backend* (mamgo). It is the **jobmatchservice applicant/job rematch funnel's** Stage-2 cosine gate. No connection to the memory loop. |
| **#1811** | "merge ceiling at 0.90–0.95" | Proposes **0.92** as a strawman and then **forbids picking a number**: *"#1810 must ship and run in production for ≥ 1 week before this is designed… designing it blind is the same calibration-trap #1568 warns about. **Wait for measurement.**"* The number that shipped is **0.94**, under a different key (`Divoid:DedupMergeCeiling`), and it lives in **#1814**, not #1811. |

**The four findings that bear on this design.**

**G1 — A similarity band expressed as a prompt rule was measured, and the model did not follow it.**
*Mechanism failure — the lesson transfers.*
Round 5 (#1085) shipped a complete band system in the prompt: *"> 0.7 = update/bridge, 0.5–0.7 = apropos to
disambiguate, ≤ 0.5 = spawn."* Round 6 (#1128) measured the result: *"Round 5 was **prompt-only** (R1–R7)
and the model didn't follow them — the vesper #926 direct thought-fan grew from ~14 pre-merge to **41
post-merge**… The graph is degenerating into a star."* Enforcement moved into the backend in round 6.

**What transfers is the lesson, not a prohibition.** Gangolf's failure was putting the *rule* in the prompt,
not having a threshold. A harness enforces admission **in code** — which is where Unit 2's floor was always
going to live. **G1 therefore argues *for* backend enforcement; it is not an argument against a floor.** It
is the single most transferable thing in the Gangolf record precisely because it is about a mechanism that
demonstrably failed, not about a product decision.

**G2 — Gangolf independently observed the score-ordering failure that #13093 found.**
#1814's calibration anchors are Gangolf's entire empirical similarity evidence: paraphrases (true merges) at
**0.955 / 0.954 / 0.939**, one at **0.901**; a subject-swap non-duplicate (true non-merge) at **0.91**.
So **a true paraphrase at 0.901 sits below a true non-duplicate at 0.91 — the bands overlap.** Gangolf
recorded this honestly (*"0.03 gap between bands; 0.94 sits cleanly in the gap"*, with the 0.901 case
knowingly sacrificed as a false negative). This is the same phenomenon as `#1487` at 0.6301 ranking below
`#10435` at 0.6378: **on both corpora, score order does not respect the semantic distinction the threshold
is being asked to make.** It is not a Processor-specific artefact.

**G3 — Gangolf's only recall-admission floor is 0.5, and it ships.**
*Product decision — evidence, not a benchmark.*
The eager-context-seed pass (#1104 → #1105 → #1106) gates what enters the model's context at
`SeedSimilarityThreshold = 0.5f`, top-3 topic hits.

**That number is a reactive chatbot admitting loosely so conversation stays fluid.** For Gangolf, a
marginal hit that turns out irrelevant costs a slightly odd remark; for a harness, it costs a wrong
artefact built confidently. The two products are buying different things with the same parameter, so
**0.70 does not have to justify itself against 0.5.** The right question — Q7, reframed — is *what floor
does a task-executing harness need*, and Gangolf's number is evidence to reason about rather than an answer.
What it does establish concretely: a shipped floor at 0.5 did not make Gangolf useless, so the region is not
absurd, and the burden on 0.70 is calibration (Q8), not comparison.

Gangolf also handles the bad-query case by **skipping retrieval entirely** rather than reformulating —
`IsChitchat(text)` is `length <= 5 || allowlist.Contains(trimmed)` — a crude ancestor of Unit 2's empty
case, and a reminder that "don't retrieve" was reachable there without "say you don't know."

**G4 — Gangolf has no "I don't know" outcome, by design, and the one guard that made an action harder made
the model stop acting.**
*Half product decision, half mechanism failure — and only the second half transfers.*
The loop's canonical rule (#1025) is the opposite of clarification: *"Every iteration with no memorize is a
stale iteration… If the user touched a substantive topic and `remember` returned nothing useful, your
`respond` should be a probe."*

**That absence is a product decision of a product whose job is to keep talking, and it is not precedent for
this design.** A harness must be able to decline — that is the whole point of Unit 2, and the operator has
ruled it so. Gangolf's silence on the question says nothing about whether a harness should have the outcome.

**What does transfer is #1317**, and it is a mechanism finding: when round 6 added a hard saturation guard,
*"the model now sometimes **opts out of memorization entirely** (visible as Toni's 'memorizing far less than
expected') rather than restructure the graph"* — and the hard-rejection path was later removed (#1319).
**The failure mode of a new gate was not the model doing the wrong thing; it was the model doing nothing.**
Unit 2 adds a gate of the same family. That risk is carried into §12 R7 on its own merits, independent of
Gangolf's product decision about clarification.

**Where the Gangolf tree is silent** — confirmed by scoped search plus body reads, not assumed:
no observed score *distribution* over a population; no two-stage summaries-then-bodies relevance funnel; no
clarification outcome; no derived-vs-raw query comparison (it uses the latest user message verbatim and
never tested an alternative); no finding that self-authored nodes pollute *recall results*.

**One Gangolf finding cut against Unit 3 as originally drafted, and the operator has answered it.** #956
records that the model had been ranking and linking on **names only**, because the listing endpoint
returned metadata without bodies — and Toni called that *the bug*: *"I'm not sure whether he is fed the
content of the thoughts he remembers… Now that he fills his thoughts he should also fully remember them."*
#1052 restates it: *"the model has only names to go on. For ambiguous cases … this is asking for a guess."*

**The objection is real and it is answered by changing the payload, not by overriding the finding.** The
operator's ruling: *"True, names only usually don't lead to an active lookup and might mislead. But we have
substance as a field — it's about not dumping noise at the agent and trying to be more focused. Better
matching nodes → good. But also not providing prose novels is another thing, and we already have the
substance field as a tool here."*

So the catalogue carries **`substance`**, not names. Gangolf established that *names cannot support a
decision*; it established nothing about a condensed body, which did not exist there. **Substance is the
middle term** between the two things Gangolf actually compared, and adopting it means Unit 3 is not "moving
back" to the state #956 called a bug. See §8.3, where this becomes a prerequisite with a cost.

---

## 4. Where this design departs from the operator's five points, and why

The operator's direction is the specification. Two points are refined by measurement that landed after it
was written (#13101). Both refinements are stated here rather than applied silently.

**Point 2 — the `ANCHOR` / `CANDIDATE` labelling — is deferred out of all three units.**
The operator's objection is that this vocabulary reads as directive rather than as retrieved material. That
readability objection may well be right. But **arm D tested exactly this and it did not move the outcome**,
and arm E establishes that the null is not vacuous. Sequencing a relabelling unit as a *fix for the Go
output* would commission work against an account whose differential test has already fired against it — a
straight M-1 clause 3 violation. Recommendation: **hold point 2 until Units 1–3 have shipped**, then revisit
it as a legibility change judged on legibility, not on artefact correctness.

**Point 5 — "clarification is not currently a possible outcome" — is right in substance and wrong in one
detail, and the detail changes the fix.**
The system text *does* contain a nudge: *"If you still do not have enough information, say so plainly and
say what is missing."* And the model *can* execute it — under CONTROL, arm F refused with *"there is no
specific request for such a task in the given context."* So the behaviour is reachable. What is missing is
not the nudge. It is that

1. clarification has **no terminal reason**, so it cannot be recorded, counted, or tested; and
2. arm F's refusal was **misdirected** — it could not find the request, which was in its own message.

Point 5 and point 4 are therefore the same defect seen from two sides, and clarification cannot be made
reliable before the request is legible. This is the load-bearing reason Unit 1 precedes Unit 2.

**Point 1 keeps its priority.** "Get the actual hits under control first" is Unit 1 and Unit 2. The one
prompt-shape change that rides along in Unit 1 (§8.2) is included because it is **72 characters**, because
its falsifier has already fired in its favour, and because — critically — **it changes the queries the model
forms**. Arm A's first recall query was *"Processor project repository structure and web page requirements"*;
arm C's was *"repository creation process for barebones webpage"*. A threshold calibrated against queries
produced by the broken prompt would be calibrated against the wrong population. Fixing the fence first is
what makes the threshold measurement in Unit 2 honest.

---

## 5. Assumptions & Constraints

| # | Constraint | Confidence |
|---|---|---|
| **C1** | The graph API supports a server-side minimum-similarity parameter. | **High** — the operator ran the yardstick text with `minSimilarity=0.7` and got `total: 0`. |
| **C2** | `substance` is **not populated** on the general graph. Checked on the four real yardstick candidates — `#1804`, `#6375`, `#10435`, `#1487` — **all four return `substance: null`.** | **High**, directly measured. Per the operator's ruling this is a **prerequisite of Unit 3, not a limitation on it**: the catalogue's payload is substance, and generating it over the retrievable corpus is part of Unit 3's cost (§8.3). |
| **C3** | `MaxModelCalls = 6` is the measured knee (17 of 20 sampled tasks reach their own terminal). Arm F burned **3 of 5** calls on recall alone. | High. Unit 3 adds round trips and will press this ceiling; see §9 R3. |
| **C4** | `internal/condense` (PR #44) and the `selfpoison` worktree are in flight elsewhere. | High. This design must not fork them — it **consumes** the pass and schedules a coverage run over the retrievable corpus as Unit 3's prerequisite (Q5). |
| **C5** | Cross-session non-determinism in the endpoint is real and **unexplained** (#13091 §5b; #13101 confirms reload is *not* the source, 12/12 token-identical). | High. Every falsifier below must be read as "consistent with", never "proof of", at these sample sizes. |
| **A1** | The corpus is a live working graph, not a fixture. Documents will be added and edited between measurement rounds. | Assumption. If false — if a frozen snapshot is available — every falsifier here gets materially cheaper. **Open question Q1.** |

---

## 6. Architectural Overview

The turn today is a straight line with one loop, and admission happens in exactly one place with exactly two
rules (self-produced, then bytes).

```
                            ┌──────────────────────────────────────────┐
  input ──┬──> Retrieve ───>│ Assemble: cut self-produced, then greedy │──> block
          │    (fused,      │ by rank until bytes run out              │
          │     anchor      └──────────────────────────────────────────┘
          │     excluded)                                                    │
          │                                                                  v
          └──> anchor ────────────────────────────────────────────>  [block][INPUT]
                                                                             │
                                                                             v
                                                                         Judge ──> answer
                                                                             │
                                                          wantsRecall ───────┤
                                                                             v
                                              dispatchRecall: Recall(query, nil)
                                              ** no anchor exclusion **
                                              ** no already-seen exclusion **
                                              full bodies, 20 KB budget
                                                                             │
                                                                             └──> back to Judge
```

Three measured defects live on that diagram:

- **The request is at the far end of the block** (`buildUserContent` appends `===== INPUT =====` *after*
  58 KB of memory). It is 0.4% of the message, at 99.6% depth.
- **`dispatchRecall` bypasses `Retrieve`'s exclusion discipline entirely.** It calls `Recall` directly with
  a `nil` scope and no exclusion set, so **the anchor comes back**, in full, *after* the request in
  conversation order. #13101 measured this as at least as load-bearing as prompt shape.
- **Admission has no similarity rule and no size rule.** One 43 KB document at eligible-rank 2 consumed
  76.6% of the budget; 81% of eligible material could not fit behind it (#13093 §5).

The target shape introduces one new concept — an **admission policy** that is a named, recorded, testable
thing rather than two inline `switch` arms — and one new outcome.

```
                    ┌─────────────────── ADMISSION POLICY ───────────────────┐
                    │  1. self-produced      -> cut  (already shipped)       │
  candidates ──────>│  2. below floor        -> cut  (Unit 2)                │──> admitted
                    │  3. is the anchor      -> cut  (Unit 1)                │
                    │  4. already in block   -> cut  (Unit 1, recall round)  │
                    │  5. oversized          -> cut  (Unit 2)                │
                    │  6. byte budget spent  -> cut  (already shipped)       │
                    └───────────────────────────────────────────────────────┘
                                              │
                          admitted == 0 ──────┴────── admitted > 0
                                 │                          │
                                 v                          v
                         RETRIEVAL EXHAUSTION          normal judgement
                                 │
                    bounded re-query (<= 2) ──> still empty ──> NeedsClarification
```

The single most important structural claim in this document: **the empty case is not an error path.** It is
a first-class outcome with its own terminal reason, its own record fields, and its own tests. A floor that
produces an empty block without a designed destination for it converts a wrong answer into a worse one.

---

## 7. Components & Responsibilities

| Component | Owns | Does **not** own |
|---|---|---|
| **Retrieve** (`internal/loop/retrieve.go`) | Forming the candidate set: fusion across queries, the scoped reserve, exclusion of the anchor **and, since 2026-09-11, of any row this system wrote**. How many rows to ask the graph for, which is no longer the same number as the limit it returns. | ~~Any admission decision. It reports what the graph returned, in the graph's order.~~ **AMENDED 2026-09-11 (`docs/architecture/the-aperture-spends-slots-admission-refuses.md`, #13601): any *budgeted* decision.** One rule crossed this boundary deliberately — the self-produced rule, on the ground that it is an *eligibility* rule rather than an admission one, and that the anchor, which this same row already assigns to `Retrieve`, is its exact structural twin. The remaining rules below are untouched and stay where this document puts them. |
| **Admission policy** (new, extracted from `assemble.go`) | The *ordered* rule set above, and a reason for every cut. It is a pure function of (candidates, exclusion set, budget, floor, size cap). | Fetching anything. Deciding what to do when it admits nothing. |
| **Assemble** | Rendering the block from what the policy admitted. | Deciding what is admitted. |
| **Supplementary recall** (`dispatchRecall`) | Running the model's own query and applying **the same admission policy**, with the exclusion set seeded from what is already in the block. | Having its own, second, weaker set of rules. This is the defect. |
| **Exhaustion handler** (new, Unit 2) | Deciding, when admission yields nothing, whether to re-query mechanically or to terminate as `NeedsClarification`. Bounded and recorded. | Writing the clarification text. That is the model's. |
| **Catalogue** (new, Unit 3) | Presenting `id / type / name / similarity / substance` rows and accepting a fetch-by-id request. | Choosing which rows matter — that is the model's job, and the whole point of the unit. Generating substance; that is the condensation pass's, run offline (Q5). |
| **Run record** | Carrying every candidate the query returned, its score, its size and its disposition — **including everything cut** — so that recall@k stays computable retroactively. **Amended 2026-09-11 (#13601): every candidate the *aperture offered admission*. A row that was never eligible is not carried, exactly as the anchor has never been carried.** | Judging. |

The invariant that ties them together: **there is exactly one admission policy, and both the initial
assembly and every supplementary recall go through it.** Today there are effectively two, and the second one
is missing three rules. Almost every symptom in §1 is downstream of that.

---

## 8. The sequenced design

Three units. Each states its account, its **differential** falsifier, what it is worth alone, and what it
needs from the unit before.

### 8.1 Unit 1 — Stop the anchor arriving twice, and put the request where it can be read

**Ships first.** Two changes, both small, both aimed at the same measured mechanism: the request losing to
the anchor.

**1a — The supplementary recall must obey the admission policy.**
`dispatchRecall` calls the graph directly and applies only the self-produced and byte rules. It must instead
carry an **exclusion set** — the anchor id, plus every id already rendered into the block, plus every id
returned by an earlier recall round in the same turn — and apply the same ordered policy as initial
assembly.

*Contract detail that is easy to get wrong and expensive if missed:* a suppressed row must be **visible to
the model as suppressed**, not silently absent. The result must say, per suppressed id, that the document is
already in its context block. Silence invites re-querying, and the call budget is 6 — arm F already spent 3
of 5 calls on recall. A suppression the model cannot see is a suppression it will fight.

**1b — Restate the request at the head of the user message, keeping the tail copy.**
This is arm C: `+72 characters`. Not "move" — **restate**. Arm B moves it and relapses into anchor-mimicking
Go at calls 4–5, writing a `main.go` that prints a greeting and does not serve the `index.html` it had just
written. Arm C never does. Retaining the tail copy costs nothing and is the cleaner arm.

**Account.** The model builds the anchor's project rather than the user's artefact because the anchor is
(i) the largest and most authoritative-looking text in the message, (ii) announced as *"the subject of this
request"*, and (iii) **delivered a second time through `recall`, after the request in conversation order** —
so with every tool round the request recedes further and the anchor comes closer.

**Differential falsifier — 1b: already run, already fired in favour.** #13101 arms D and E are the
differential. E deletes an entire system sentence and moves nothing (0/5). D rewrites the exact mislabelling
sentence and moves nothing in PRIMARY. Only the two position arms move. *"Any perturbation works"* is
excluded. **This falsifier is discharged; 1b may be commissioned.**

**Differential falsifier — 1a: NOT yet run. Must fire before 1a is built.**
The naive gate — *"does removing the anchor from the recall result produce a webpage?"* — is **not a gate**:
#13101 already shows emptying the *entire* recall result does that, so a pass would not distinguish "the
anchor specifically was the problem" from "less text was the problem". The differential is:

> Replay the captured wire bodies three ways: (i) recall result unchanged; (ii) recall result with **the
> anchor `#10422` scrubbed**; (iii) recall result with a **size-matched non-anchor decoy scrubbed** —
> a different candidate of ~3.4 KB, structure and ordering preserved.
>
> **Fires against the account if (ii) and (iii) move the outcome equally**, which would show the effect is
> byte volume rather than the anchor's identity. Fires *in favour* only if (ii) moves and (iii) does not.

Cheap: it reuses the replay instrument #13101 already built and the archive it already captured. **No new
harness run is needed.**

**Worth alone.** High and immediate. 1b is the largest measured prompt lever (0/3 → 3/3 webpage, Go bytes
3092 → 0, the token "Processor" disappearing from the output). 1a addresses the largest lever *measured
anywhere in the round*, including outside the six arms. Neither depends on the other, and the unit is
valuable even if Units 2 and 3 never ship.

**Needs from earlier units.** Nothing. This is the entry point.

**Cost.** Small. 1b is a change to how the user message is composed plus one system-text sentence. 1a
threads an exclusion set from assembly into the recall dispatch and extends the suppression vocabulary by
one reason. The genuine risk is behavioural, not structural: suppression must be legible (above), or the
model burns its call budget re-asking.

---

### 8.2 Unit 2 — A floor, and somewhere for the empty case to go

**Ships second, and ships as one unit.** The floor and the clarification outcome are not separable: **the
floor is the mechanism that creates the empty case.** Shipping a floor without a designed destination for
its output would turn a wrong answer into a broken run. This is a refinement of the operator's ordering, not
a departure from it — point 1 still comes before points 3 and 5's remainder.

**Three admission rules, ordered.**

| Rule | Value | Why this value |
|---|---|---|
| **Similarity floor** | **0.70**, applied **after** the self-produced cut | The operator's stated noise floor, and the number their own measurement used. Applying it *after* the self-produced cut is mandatory: run records score **0.7359–0.6750**, so a floor applied first would fill the window with above-floor junk that is then discarded anyway. |
| **Per-candidate size cap** | **12,000 bytes** (20% of the 60,000 budget) | Directly targets #13093 §5 mechanism 3. `#6375` at 43,057 B took 76.6% of admitted bytes and pushed the decision boundary onto a 79-byte margin. **Exclude, do not truncate** — a truncated document silently lies about being complete. |
| **Re-query budget** | **at most 2** mechanical re-queries before terminating | Bounded by C3: the call ceiling is 6 and recall rounds are expensive. |

**On why a floor is defensible here when #13093 rules out a floor *alone*.** The distinction is what the
floor is *for*. It is **not** a ranking device — §3 establishes it cannot be one on this corpus. Its job is
to **make retrieval failure visible.** Today a failed retrieval and a successful one are indistinguishable
downstream: both produce ~55 KB of confident-looking material. A floor converts the failure into an
observable event. That is only an improvement if the system can *do* something with the event — which is why
this unit contains the destination as well as the gate, and why it must not ship without it.

On the yardstick, a 0.70 floor admits **nothing**. That is the intended and correct behaviour under the
operator's own specification, and per #13093 §4 the right response is **retrieve better** (`#6309`/`#6311`
sit at 0.73–0.76 for a self-formed query), *then* clarify if that also fails.

**Three properties the floor must have. The first two are lessons Gangolf's mechanisms teach; the third is
this design's own obligation.** Gangolf is a reference, not a blueprint (§3a) — none of these is a
constraint imported wholesale from a reactive chatbot.

1. **It must be mechanical, not prompted.** G1 is production-observed: Gangolf shipped the identical band
   idea as a *prompt rule* and the model ignored it, at measured cost (thought-fan 14 → 41). A harness
   enforces admission in code, which is where this floor was always going to live. **G1 is an argument for
   backend enforcement, not against having a floor.**
2. **It must be one reversible config value.** #1814's shipped design is the pattern to copy: one float key,
   *"reversibility via setting > 1.0"*. Adopt that shape, including the escape hatch — it is what makes R1
   and R7 recoverable in one edit.
3. **The number is calibrated for one query on one corpus, and Unit 1 changes the query.** #1811's
   discipline (*"designing it blind is the same calibration-trap… wait for measurement"*) and #483's
   counter-example (*"the value was Sarah's heuristic from design #502, never empirically calibrated"* — and
   it later had to move) both apply as *method*, not as numbers. We are **not** blind: #13093 is the
   measurement. But it measures *one* query text, produced by a prompt Unit 1 fixes. **Binding obligation:
   re-read the score distribution after Unit 1 ships, before the floor's value is treated as settled**
   (Q8). **[baseline]**

**On Gangolf's 0.5 (G3), explicitly: it is not a benchmark 0.70 must justify itself against.** That number
is a reactive chatbot admitting loosely to keep conversation fluid; a marginal hit costs it an odd remark,
where it costs a harness a confidently-built wrong artefact. The question is not *"why is Processor 0.20
higher"* but **"what floor does a task-executing harness need"** — Q7. Gangolf's number is evidence bearing
on that question, not an answer to it.

**The clarification outcome must be representable before it can be reliable.** Add a terminal reason —
`NeedsClarification` — to the loop's closed set. This is the sharpest available statement of the operator's
point 5: the nudge already exists in the system text and the model can already execute it (arm F under
CONTROL), but **an outcome the type system cannot name cannot be recorded, counted, gated, or tuned
toward.** Today a clarification would be filed as `Answered`, indistinguishable from a confident wrong
answer. Everything else in point 5 is downstream of that.

The run record gains: the floor in `Limits`; every below-floor candidate with its score (so the
counterfactual stays computable); the re-query count and each derived query; and the terminal reason.
**Record everything that was cut.** The operator wants to see both results — a record that only shows the
admitted set makes the empty case un-auditable.

**Differential falsifier — NOT yet run. Must fire before Unit 2 is built.**
The naive gate — *"does it clarify when the graph has nothing?"* — is **not a gate**. A system prompted hard
enough to clarify will clarify on everything, including tasks for which it has ample memory, and that
failure mode is invisible to a one-population test. The differential is **two populations**:

> **Population A** — inputs for which the graph demonstrably holds sufficient memory above the floor.
> **Population B** — inputs for which the graph demonstrably holds nothing above the floor.
>
> The gate asks whether **clarification rate separates A from B**. It **fires against the design** if the
> system clarifies on A as readily as on B (over-asking — the floor made it useless), *or* acts on B as
> readily as on A (the outcome is nominal and never taken). Only a separation supports the design.

Note what this requires: **population A must exist**, and on the current corpus it demonstrably does not for
the yardstick task — the ceiling is 0.6887. That is precisely what makes the seeding question in §10 an
instrument obligation rather than a nice-to-have.

**Worth alone.** Moderate on its own; high in combination. Alone it converts silent wrong answers into
honest refusals — a real gain in trustworthiness, and a real loss in coverage until Unit 3 or better queries
arrive. It is the unit that makes the system's failures *legible*, which is the precondition for tuning
anything.

**Needs from Unit 1.** Two things, both hard dependencies:
1. **Honest calibration.** Unit 1b changes the queries the model forms (arm A vs arm C, §4). Calibrating a
   floor against the broken prompt's query distribution measures the wrong population.
2. **A meaningful empty case.** Arm F's CONTROL refusal — *"there is no specific request for such a task"*
   while the request sat in its own message — shows that with the fence unfixed, the system cannot
   distinguish *"I have no memory"* from *"I have no request."* Clarification built on top of that is
   clarification about the wrong thing.

**Cost.** Medium. A query parameter (server-side, already supported — C1), a new terminal reason threaded
through the record and the adapters, a bounded re-query loop, a system-text section, and new record fields.
The re-query loop is the only genuinely new control flow and it must be bounded and recorded.

---

### 8.3 Unit 3 — The catalogue

**Ships third.** The operator's point 3: stop pushing full bodies; send a **catalogue** first and let the
model ask for what it wants.

**The catalogue row is `id` / `type` / `name` / `similarity` / `substance`.** Operator ruling, 2026-09-07 —
*"we already have the substance field as a tool here… it's about not dumping noise at the agent and trying
to be more focused. Better matching nodes → good. But also not providing prose novels is another thing."*
Not names alone, and not full bodies. Substance is the middle term, and choosing it is what answers the
Gangolf objection (§3a) rather than overriding it: #956 established that *names* cannot support a decision,
and said nothing about a condensed body, which did not exist there.

**Account, and it is stronger than a bytes argument.** The single useful document on the yardstick,
`#1487`, is titled **"Mamgo.io-Website mit Claude bauen"** — *build the Mamgo.io website with Claude*. By
similarity it ranked **19th of 20** and was cut for bytes in four of six runs. By title it is the obvious
pick for *"generate a webpage and a repo."* Meanwhile `#10435` — *"Processor M0: Go service skeleton"* —
scored **higher** and is transparently about something else.

> **The claim: on this corpus, a condensed catalogue carries decision signal that similarity ordering does
> not.** That is not a bandwidth argument; it is a *ranking* argument, and it is the one thing that might
> work where a floor provably cannot, because it does not depend on the score ordering that §3 rules out.

**Prerequisite, not a limitation — and it is part of the cost.** C2 records that `substance` is **null** on
all four real yardstick candidates. With substance as the payload, that is no longer a constraint the design
works around: **it is work Unit 3 requires before it can run at all.** The generator exists — the
condensation pass shipped in PR #44 (`cmd/condense`, `internal/condense`), with a measured median ratio of
**0.722**, and **0.298 for nodes above 8 KB**, which is precisely the class doing the crowding.

Applying those reported ratios to the yardstick's admitted set gives a useful estimate — **derived, not
measured, and marked as such**: `#6375` 43,057 B → ≈12.8 KB; the five sub-8 KB documents 12,070 B → ≈8.7 KB;
admitted candidate bytes ≈**21.5 KB against 54.9 KB today**. On that arithmetic, roughly the whole eligible
window would fit inside the existing budget. **Two consequences.** First, the condensation pass attacks
#13093 §5 mechanism 3 (one 43 KB document taking 76.6% of the budget) largely *on its own*, independent of
the catalogue — so it may be worth sequencing ahead of the catalogue on its own merit. Second, it means the
generation pass must cover the **retrievable** corpus, not just the eval corpus it was built against, and
that coverage is a real, schedulable cost with a real runtime.

**Differential falsifier — NOT yet run. Must fire before Unit 3 is built.**
The naive gate — *"does the model pick `#1487` from the catalogue?"* — is **not a gate**: arm F already
shows that simply removing bulk flips the artefact, so a pass would not distinguish *catalogue signal* from
*crowding relief*, and crowding relief is already known. With substance as the payload the differential now
needs **three** arms, because there are two distinct things to rule out:

> Present the same candidate list as a selection task, three ways, with row count, ordering and structure
> preserved exactly:
> (i) **opaque labels** — `node 1487, task, 2867 bytes`, no semantic content at all;
> (ii) **names only** — the state #956 called the bug;
> (iii) **name + substance** — the proposal.
>
> **Fires against the account if (iii) ties (i)** — the gain is crowding relief, already measured by arm F,
> and the catalogue's content is doing nothing. **Also fires against if (iii) ties (ii)** — substance adds
> nothing over the title, the generation cost buys nothing, and the Gangolf objection stands unanswered.
> Supports the design only if **(iii) > (ii) > (i)**.

The (ii) arm is what makes this a genuine test of the operator's ruling rather than a restatement of it. It
is cheap, and all three arms run **before any harness change**, against the captured candidate list — but
(iii) requires substance generated for the twenty candidates first, which is the smallest useful slice of
the prerequisite above and a sensible way to buy it incrementally (M-2).

**Worth alone.** Potentially the highest ceiling of the three, and the only one that attacks the ranking
problem head-on — but also the one whose central claim is **entirely untested today**. It should not be
started before its falsifier fires. Note that its prerequisite is *separately* valuable: condensation
coverage improves every unit's byte arithmetic whether or not the catalogue is ever built.

**Needs from Units 1 and 2.**
- From **Unit 1**: the exclusion discipline. A catalogue that lists the anchor, or lists documents already
  in the block, re-creates the exact defect at lower cost per row and higher row count.
- From **Unit 2**: the floor and the empty outcome. A catalogue of twenty sub-threshold rows is a menu of
  noise; the model will pick from it because picking is what it was asked to do. The floor is what makes an
  empty catalogue possible, and the empty outcome is where an empty catalogue goes.

**Cost.** Largest of the three, and most of it is not code.

1. **Substance generation over the retrievable corpus** — the prerequisite above. A pass that exists but has
   not been pointed at this population. Runtime, model spend, and a coverage criterion someone has to
   define: *retrievable* is not the same set as *the eval corpus*, and the difference is the whole graph.
2. **Round trips** — the part that is easy to miss. Catalogue → select → fetch → answer is three calls
   before any work begins, against `MaxModelCalls = 6` (C3), in a system where arm F already burned 3 of 5
   calls on recall alone. **Unit 3 almost certainly requires the call ceiling raised**; measured, not
   assumed, and it changes the cost of every run. See Q3.
3. **Code** — a fetch-by-id tool, a new terminal reason, a second budget dimension.

Item 1 is the one to schedule first and the one most likely to be underestimated. It is also the item that
pays off independently (see *Worth alone*).

---

## 9. Cross-Cutting Concerns

**Ordering is a contract, not an implementation detail.** The admission rules are ordered and the order is
load-bearing: self-produced before floor (or run records at 0.72–0.74 pass a 0.70 floor and consume the
window) — **amended 2026-09-11 (`docs/architecture/the-aperture-spends-slots-admission-refuses.md`,
#13601): that first constraint is moot for the initial assembly, where no run record reaches the floor
because none reaches admission at all, and it stays live for the supplementary aperture, which does not
pass through `fuse`** — floor before size cap (no point measuring the size of noise), size cap before greedy byte
admission (or one oversized document sets the boundary, as `#6375` did). This ordering belongs in the
policy's contract and in its tests.

**Auditability is the product here, not a by-product.** The run record must carry every candidate the query
returned, with score, size, and disposition — *including everything cut* — and now also the floor, the
derived queries, the re-query count, and the terminal reason. #13093 was only possible because dispositions
for cut candidates were already recorded. Every rule added here removes a document from the block; if it
also removes it from the record, the next analysis of this kind becomes impossible.

**Idempotency and the exclusion set.** The exclusion set grows monotonically within a turn: anchor, then
block contents, then each recall round's results. It never shrinks. This must hold even when a recall round
errors — a failed round that clears the set would let the anchor back in on the next one.

**Failure modes.** Graph unavailable during a re-query: terminate the re-query loop and fall through to the
existing graph-unavailable path; do **not** silently report exhaustion, which would file a retrieval outage
as a corpus fact. Floor configured above every score in the corpus: the system clarifies on everything —
detectable only by the two-population gate in §8.2, which is why that gate is mandatory.

**Consistency model.** Unchanged: one turn, one read of the graph per query, no transactions. The corpus is
live (A1) and scores may move between runs; the record's content hashes are what make a past run
reinterpretable.

---

## 10. On seeding synthetic blueprints — instrument construction, held to the same standard

The operator proposes seeding documentation or blueprints — how we set up a repo, the boilerplate webpage we
usually build — so that there is memory we *know* matches and *know* is sufficient.

**What it buys.** It is not a convenience. §8.2's differential gate requires **population A** — inputs for
which the graph demonstrably holds sufficient memory above the floor — and on the current corpus, for the
yardstick task, **population A is empty** (ceiling 0.6887). Without seeding, Unit 2's falsifier cannot run,
and under M-1 clause 4 Unit 2 cannot then be commissioned except on an explicit statement that it is being
bought without a gate. **Seeding is the instrument the gate needs.**

**What it costs.** An authoring pass, plus a permanent change to the corpus under measurement. Every
subsequent retrieval reading is against a graph that now contains documents written to be retrieved.

**What would make it a machine for confirming itself** — the question asked directly, answered directly.
Three mechanisms, each with a guard stated as an obligation:

| Self-confirmation risk | Guard |
|---|---|
| **The blueprint is written from the task text**, so its embedding scores high *by construction*. The floor then passes it because it was authored to pass, and the measurement reports the seeding rather than the retrieval. | **Author the blueprint against the domain, never against the query.** The author must not have the yardstick string in view. Checkable by a third party: no n-gram overlap with the task text beyond common vocabulary. |
| **The blueprint is the only thing above the floor**, so "the model used the blueprint" is guaranteed rather than observed. | **Seed at least one decoy blueprint** in an adjacent-but-wrong domain — *how we set up a Python CLI repo* — authored to the same standard and length. If retrieval cannot separate the right blueprint from the decoy, the instrument reports nothing about retrieval and the reading is void. |
| **"Sufficient" is defined by what was seeded**, because the same author writes the blueprint and the success criterion. | **Register the success criterion before seeding**, and **hold out a task the blueprint does not cover** as population B. If the seeded corpus makes the system act confidently on B too, the seeding taught it to always act — which the two-population gate detects and nothing else does. |

**Buy the authoring pass separately from the artefact (M-2).** Deciding, document by document, what a
*sufficient* blueprint must actually contain — for this graph, at this floor — is the part that yields
findings whether or not the corpus is ever finished. #12961 M-2 records that in a comparable round every one
of four units came from the authoring pass and none from the artefact. File what the pass forces open before
committing to the full set.

---

## 11. Quality Attributes & Trade-offs

| Attribute | Effect | Trade-off accepted |
|---|---|---|
| **Correctness** | The primary gain. Units 1 and 2 remove three measured mechanisms by which the anchor displaces the request. | Unit 2 trades coverage for honesty: on the current corpus a 0.70 floor admits nothing on the yardstick. Runs that previously produced *something* will produce a question. **That is the intended change**, and it is reversible by lowering one number. |
| **Latency / cost** | Unit 1 is neutral. Unit 2 adds up to 2 re-queries. Unit 3 adds a round trip per turn. | Unit 3's round trips press `MaxModelCalls` (C3) and are the main reason it ships last. |
| **Observability** | Substantially improved: floor, derived queries, re-query count, terminal reason, and every cut candidate in the record. | Larger records. Given #11141 (records poisoning the next turn) this is not free — but records are already excluded as self-produced, and the exclusion is tested. |
| **Simplicity** | Admission becomes one named, ordered, testable policy instead of two divergent inline rule sets. | One new abstraction. It earns its place: the divergence between the two current rule sets is the direct cause of the anchor's second injection. |

**Alternatives considered and rejected.** *Re-ranking with a second model call* — rejected: larger purchase,
untested account, and §3 shows the problem is not purely ordering. *Truncating oversized candidates instead
of excluding them* — rejected: a truncated document presents as complete and will be reasoned from as
complete. *Dropping the anchor entirely* — rejected: it is the harness's core inversion, and #13101 arm D
shows that removing the anchor's *authority* without giving the request a *position* just makes the model
adopt a different graph node.

---

## 12. Risks & Mitigations

| # | Risk | Mitigation |
|---|---|---|
| **R1** | **Over-asking.** The floor makes clarification the common outcome and the harness becomes useless. | The two-population gate (§8.2) is designed to detect exactly this and is mandatory before build. The floor is one number and is trivially reversible. |
| **R2** | **Suppression fights the model.** 1a hides documents; the model re-queries for them and burns the call budget. | Suppressed ids are rendered as explicitly suppressed with a reason, never silently dropped (§8.1). |
| **R3** | **Unit 3 exhausts the call budget** before producing anything. | Measure the ceiling before building; treat raising `MaxModelCalls` as part of Unit 3's cost, not a free adjustment (Q3). |
| **R4** | **Falsifiers read as proof at n = 2–3.** C5: cross-session non-determinism is real and unexplained; #13101 confirms reload is *not* its source. | Every result stated as *consistent with*, never *proof of*. Any single flipped row invalidates a row, not the design. Q2 tracks the underlying cause. |
| **R5** | **Seeding contaminates the corpus** and every later retrieval reading. | Guards in §10, and seeded documents must be identifiable as seeded so any reading can be recomputed with them excluded. |
| **R6** | **P-40 parity drift** between this file and its node. | Both sides move together, operator publishes; bytes and sha256 reported with delivery. |
| **R7** | **The model opts out rather than adapts.** Gangolf's round-6 saturation guard produced exactly this: *"the model now sometimes **opts out of memorization entirely**… rather than restructure the graph"* (#1317), and the hard-rejection path was later removed (#1319). This is a **mechanism** finding about what new gates do — it carries on its own merits, and is independent of Gangolf's separate *product decision* to have no clarification outcome, which is not precedent here (§3a G4). Unit 2 adds a gate of the same family; the failure mode to expect is not wrong action but **no action**. | The two-population gate (§8.2) detects over-asking but **not** silent under-production. Add a third reading: the **rate at which the model produces no artefact and no clarification** — a null outcome is a distinct failure and must be counted separately from both. Follow #1814's reversibility shape so the gate is disabled by one value. |

---

## 13. Open Questions

| # | Question | Why it matters |
|---|---|---|
| **Q1** | Is a **frozen corpus snapshot** available for measurement, or is every reading against the live graph (A1)? | Every falsifier here gets materially cheaper and more repeatable against a snapshot. Also decides whether R5 is containable. |
| **Q2** | What is the source of the cross-session non-determinism in #13091 §5b? #13101 rules out model reload (12/12 token-identical). | It bounds the confidence of every measurement in this document. |
| **Q3** | Is raising `MaxModelCalls` above 6 acceptable, and at what cost? | Unit 3 needs ~3 calls before work begins; C3 says 6 is the measured knee. Unit 3 may be infeasible without an answer. |
| **Q4** | Should the floor ship **enforcing** at 0.70, or **instrumented-but-non-enforcing** for one round first? | Recommendation: **enforcing**, with #1814's reversibility escape hatch. A non-enforcing floor never produces the empty case, so it never shows the operator the second of the two results they asked to see. Stated as a recommendation because it is the operator's risk to take. |
| **Q7** | **What floor does a task-executing harness need?** Not *"why is Processor 0.20 above Gangolf"* — the operator has ruled that comparison out (§3a G3). The real question is what an *acting* system's admission threshold should be when a marginal hit costs a wrongly-built artefact rather than an odd conversational remark. | Gangolf's 0.5 is evidence bearing on the question, not an answer: it establishes the region is not absurd for a *reactive* product. The operator's 0.70 floor and 0.85 aspiration are the harness-side hypothesis. Q8's post-Unit-1 re-read is the measurement that should settle it. |
| **Q8** | Does the operator want the §8.2 floor value **re-read after Unit 1** before it is treated as settled, as §8.2 constraint 3 obliges? | Unit 1b measurably changes the queries the model forms. Skipping the re-read reproduces the #483 pattern — a heuristic value never empirically calibrated, which later had to move. |
| **Q5** | **What is the coverage criterion and the runtime for pointing the condensation pass (PR #44) at the *retrievable* corpus, rather than the eval corpus it was built against?** | Now a **prerequisite of Unit 3**, not a nice-to-have (§8.3). It also has standalone value: on the reported ratios (0.722 median, 0.298 above 8 KB) it substantially dissolves the crowding mechanism on its own, which may justify sequencing it ahead of the catalogue. Someone must define *retrievable* — it is not the eval corpus, and the difference is the whole graph. |
| **Q6** | Point 2 (the `ANCHOR`/`CANDIDATE` vocabulary) is deferred per §4. Does the operator accept deferral, or want it shipped as a legibility change judged on legibility alone? | It is the one operator point this design does not act on, and the reason is measured (arm D). |

---

## 14. Implementation Guidance for the Next Agent

Ordered. **Do not start a unit whose falsifier has not fired.**

**Phase 0 — run the falsifiers that are not yet run.** Replays against the archive
`promptshape-v8k3-bodies.zip` and the captured candidate list. No harness change, no new harness run.
1. §8.1 anchor-scrub vs size-matched-decoy-scrub (gates Unit 1a).
2. §8.3 three-arm catalogue selection — opaque / names-only / name+substance (gates Unit 3). Arm (iii)
   needs substance generated for **the twenty captured candidates only** — the smallest useful slice of
   Unit 3's prerequisite, and the right way to buy it incrementally (M-2).
3. §8.2's gate needs population A, which needs §10 seeding — so run §10's **authoring pass** here, file what
   it forces open, and only then decide whether the full corpus is bought (M-2).

**Phase 1 — Unit 1.** 1b first (it is 72 characters and its falsifier is already discharged), then 1a.
Extract the admission policy as a named ordered rule set as part of 1a rather than after it; the extraction
is what makes 1a a two-line change instead of a duplicated rule set.

**Phase 2 — Unit 2.** Floor, size cap, exclusion of the empty case as an error. Add the terminal reason
first and thread it end to end — including the record and the adapters — *before* wiring the re-query loop,
so that the outcome is recordable from the first run that produces it. **Re-read the score distribution
after Phase 1 lands and before the floor's value is fixed** (§8.2 constraint 3, Q8). The floor is a
mechanical admission rule; it is never expressed to the model as a rule it is asked to apply (G1).

**Phase 3 — Unit 3.** Only after Q3 and Q5 are answered. **Substance generation over the retrievable corpus
comes first and is schedulable independently** — it improves every unit's byte arithmetic whether or not the
catalogue is built, so it need not wait for the catalogue's falsifier to fire.

**Throughout.** Every rule that removes a document from the block must still record it, with its score, its
size, and its reason. That property is what made #13093 possible and it is the single most valuable thing
this codebase currently has.

---

## 15. Provenance

- **#13091** — six live runs on the yardstick.
- **#13093** — per-candidate scores, the byte arithmetic, and the ruling that a floor alone cannot work.
- **#13101** — six prompt-shape arms; the INPUT-fence result, the D/E nulls, and the recall re-injection
  lever.
- **#12961** — Method Contracts, M-1 and M-2, under which every falsifier above is named.
- **#12978** — polish is reasoned from a failing product.
- **Operator rulings, 2026-09-07**, recorded inline where they bind: Gangolf is a **reference, not a
  blueprint** (§3a); the catalogue's payload is **`substance`**, and generating it is a **prerequisite**
  rather than a limitation (§8.3); and **no hard conclusion about the design's ceiling** may be drawn from a
  baseline that violates the vision's own commitments (§1a).
- **Gangolf (#902)**, §3a — read directly, ~30 node bodies: **#1085** (the 0.5/0.7 prompt band system),
  **#1128** (its measured failure), **#1104 / #1105 / #1106** (the shipped 0.5 seed floor), **#1811** (the
  "wait for measurement" discipline), **#1814 / #1815 / #1817** (the 0.94 ceiling as shipped, and its
  calibration anchors and reversibility shape), **#1025** (probe-don't-admit-ignorance), **#1317 / #1319**
  (the guard that made the model stop acting), **#956 / #1052** (names-only was the bug), **#1353**
  (similarity exists only on `?query=` searches). **#483 is *not* a Gangolf node** — `rootNodeId 69`,
  mamgo jobmatchservice — and is cited above only as a cautionary precedent about uncalibrated heuristics.
- Verbatim prompt: `C:\dev\claude\_scratch\actual-prompt\02-prompt-verbatim.txt` (60,925 B).
- Code read: `cmd/processor/system_text.go`, `internal/loop/{assemble,retrieve,turn,types}.go`,
  `internal/divoid/{client,substance}.go`, `internal/openaicompat/wire.go`.
- C2 measured directly against the graph on 2026-09-07: `substance` null on `#1804`, `#6375`, `#10435`,
  `#1487`.
