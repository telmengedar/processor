# Architectural Document: What Goes In The Block — content, substance, or a catalogue

> **Repo path `docs/architecture/what-goes-in-the-block.md`, DiVoid node #13238.** The two are the same
> document byte for byte, and an edit to one is not finished until the other matches it (P-40). Runnable:
> `divoid_download_content(13238, <path>)`, SHA-256 it, compare against
> `git cat-file blob <ref>:docs/architecture/what-goes-in-the-block.md`.
>
> **Baseline: `main` at `b021fdc`, read-only.** Every `file:line` resolved against that tree. PR #48
> (`fix/exclude-run-records-from-recall`, tip `d37a297`) is approved and unmerged; differences are stated.
>
> **Live-graph measurements taken 2026-09-08** against `https://divoid.mamgo.io/api`. The graph mutates;
> every figure carries its date.
>
> Project **#10422** · Map root **#10454** · Vision **#10424** · constrained-context vector **#11364**
> Prior art this document argues with: **#12955** (`substance-backed-admission.md`, two-pass admission) ·
> **#13106** (`retrieval-admission-and-the-empty-outcome.md`, Unit 3 the catalogue) · **#12984** (Unit A
> measured: the ratio and the fidelity audit) · **#13203** (PR #48's decision record) · **#13237**
> (superseded in part, §14) · **#13091**, **#13093**, **#13101** (the failing product) · **#11141** (the
> original self-poisoning finding) · type vocabulary **#9**, Types group **#29**, `type: session-log`
> **#59** · API surface **#8** · run-record fate **#10904** · M1 **#10532**

---

## TL;DR

**Toni's question is the right one and it is not about exclusion.** *What does the loop put in the block —
full content, `substance`, or a catalogue the model expands on request?* Under substance there is nothing
to exclude: a relevant node earns its slot, an irrelevant one is degraded by similarity. Exclusion is a
workaround for a payload defect.

**And his sharper point lands.** There is no principled difference between a peer's session log and ours.
The difference is **shape** — theirs is prose that *could* carry substance, ours is `application/json` that
cannot — and shape is fixable. Provenance was never a reason. **This document withdraws that distinction.**

**The measurement that decides the sequencing, taken today:**

| | |
|---|---|
| **We shipped `substance` and never asked for it** | `candidateFields = "id,type,name,similarity,content"` (`internal/divoid/client.go:34`). `loop.Candidate` has no substance member. `assemble.go` renders `c.Content`, always. We asked DiVoid for the field (#11367), DiVoid shipped it (#11371), we designed admission around it (#12955) — and the loop has never requested it. **The listing route accepts `fields=…,substance` and returns it; verified live today.** |
| **But the graph has almost no substance to give** | Sampled live: **1 of 500** random nodes, **6 of 500** documentation, **0 of 500** session-logs, **0 of 500** tasks. On the yardstick's own top-20, **1 of 20**. `cmd/condense` has only ever been pointed at the 25 required nodes of the eval corpus (#12984). **Coverage, not admission policy, is the binding constraint.** |
| **Where it would pay is exactly where the crowding is** | The yardstick's seven real candidates total **106,829 B against a 60,000-byte budget** — they cannot all fit. Applying #12984's measured stratum ratios: **36,081 B in substance form, all seven fit, 23,919 B spare.** (One row is a real measurement: #13101, 11,961 B → 5,527 B, ratio 0.462.) |
| **And the condenser refuses the class that needs it most** | `internal/condense/condense.go:283` skips `SelfProduced`; `:296`'s `isProse` refuses `application/json`. **A run record is refused twice.** A third gate is measured, not designed: at 195 KB, node #10926 could not be condensed at all — truncated and refused (#12984), and run records are 77–88 KB. |

**The recommendation, in four lines.**

| | |
|---|---|
| **Unit 1 — See it** | Request `substance` in recall; carry it on `Candidate`; record which form was rendered. **No admission-policy change.** Small, and nothing downstream is measurable without it. |
| **Unit 2 — Generate it** | Point `cmd/condense` at the *retrievable* corpus, not the eval corpus. Mostly runtime and model spend, not code. **It pays off under all three payload designs**, which is why it is not gated on choosing between them. |
| **Unit 3 — Render it, size-gated** | Substance replaces content **where condensation actually compacts** — the ≥8 KB stratum, measured median ratio **0.298**. Below 4 KB the measured ratio is **0.886**: substance saves ~11 % and spends fidelity, so content stays. |
| **Unit 4 — The catalogue** | #13106 §8.3, unchanged and still third. Its payload *is* substance, so it is downstream of Unit 2 by construction, and its three-arm falsifier cannot run until Unit 2 has covered the twenty rows. |

**Where I disagree with #12955, explicitly.** Its invariant — *substance may stand in for content only where
content would have been absent* — was the right first move **under zero fidelity evidence**. That evidence
now exists and cuts both ways. It **vindicates** the caution: the fidelity audit is zero-tolerance and it
**failed**, 23 PASS / 1 FAIL, and the failure dropped a load-bearing negation (#12984). It also **refutes
the mechanism**: at a 60,000-byte budget, **20 of 24 generated substances (83 %) exceed the largest leftover
ever measured**, so two-pass admission has almost nothing it can admit. **A design whose safety comes from
being inert is not safe, it is absent.** Toni's position is materially less conservative and materially more
useful; the rule that survives both is **size-gated substitution**, which spends the fidelity risk only
where it buys 70 % of a node's bytes and never where it buys 11 %.

**On exclusion, the position changes.** PR #48 should still merge — a 62 KB transcript must never be
rendered. But the rule is recast from **provenance** to **form**: *a run record may be admitted in substance
form and never in content form.* That answers *"how is other agents' logs real memory and ours unreal?"* —
it isn't; ours had no readable shape. **The substance of a prior run may be the most valuable memory in the
graph** — *"a prior run of this exact task wrote a Go project instead of a webpage"* is precisely the
"history compressed to substantial facts" Toni describes — **and the condenser is currently configured to
refuse it.** §9.

**Type change: still right, different reason.** Not to exclude — to describe. A JSON run transcript is not a
narrative work arc (`session-log`'s registered definition, #59, rules it out in terms). **Migration is 17
rows and the vocabulary is convention; §13 is one paragraph, not an open question.**

---

## 1. Problem Statement

Every candidate that reaches the model reaches it as its **full body**. On the yardstick task the seven
genuinely relevant nodes total 106,829 B against a 60,000-byte budget, so the assembler cuts most of them,
and a single 42,931 B document takes 71 % of the budget on its own. Around that, the harness's own run
records — 77–88 KB each, thirteen of twenty candidate slots today — crowd the list further.

The project's response so far has been **exclusion**: cut the rows that cannot fit. That treats a symptom.
Toni's challenge names the disease:

> *"here we still have the substance field — like with all other memories you should only use substance and
> not the full memory dump… in the end substance of a session log is the best context you can get for your
> current work — a history compressed to substantial facts. As soon as they don't fit your scope anymore
> automatically removed by semantics."*
>
> *"so in general i'm not even sure we should actually exclude session logs… only that they dump the whole
> noise while we have the chance to only include the important stuff and automatically degrade irrelevant
> stuff. Btw. how is other agents logs real memory and ours are unreal memory? … something sounds fishy
> here"*

**Both halves are correct.** The payload is the design; exclusion is a consequence of getting it wrong. And
the "real memory / unreal memory" distinction this project has been operating on was arbitrary — it named a
provenance where the actual difference is a shape.

**The question this document answers:** what form does a candidate take in the block, under what rule, and
what has to be true before that rule can be applied?

**Success criteria.**

| # | Criterion |
|---|---|
| S1 | The loop can obtain and render a candidate's `substance`. It cannot today, at all. |
| S2 | The payload rule is stated as a rule over **node properties**, never over **who wrote the node**. |
| S3 | Substance is rendered only where it is measured to compact materially, and never where it merely trades fidelity for ~11 % of bytes. |
| S4 | A run record is eligible for the block on the same terms as any other node, once it has a form that can be read. |
| S5 | No node is rendered in a lossy form without the record saying so, and without `size`/`contentHash` keeping their present meaning. |
| S6 | The design says what must be measured before each unit ships, and what result would sink it. |

---

## 2. Scope & Non-Scope

**In scope.** The payload contract between retrieval, assembly and the model. The rule selecting a
candidate's rendered form. What has to be generated before that rule does anything. The recast of the
run-record exclusion from a provenance rule to a form rule. The node type run records are filed under.

**Out of scope.**

| Not this design | Why |
|---|---|
| The condensation **prompt** | Kim's. #11373 is the spec; #12984's F-B failure is a prompt defect and §11 routes it there |
| Deriving recall queries, fusion, the scope reserve | Unchanged; #11235 and #13203 own them |
| The similarity floor and the empty outcome | #13106 Units 1–2. This design composes with them and replaces neither |
| Raising `MaxModelCalls` | Only Unit 4 needs it (#13106 §8.3 Q3). Named, not decided here |
| Condensing the **anchor** | #12955's Unit D. The anchor is not budget-tested and needs its own falsifier |
| A distilled per-run memory artifact written by a *second* pass | §9.4 explains why it is no longer needed as a separate concept once run records are condensable |

---

## 3. Assumptions & Constraints

| # | Statement | Provenance / confidence |
|---|---|---|
| A1 | Recall requests `id,type,name,similarity,content`. **`substance` is not requested** | `internal/divoid/client.go:34`. Certain |
| A2 | `loop.Candidate` has no substance member; `internal/loop` has no production reference to substance | `internal/loop/types.go:14-31`; grep of production files. Certain |
| A3 | `renderBlock` writes `c.Content` for every admitted candidate | `internal/loop/assemble.go`. Certain |
| A4 | **The listing route accepts `substance` in `fields` and returns it**, omitting the key when unset — the same convention as `content` | Verified live 2026-09-08 on the exact route `Recall` uses; also `internal/divoid/substance.go:15`. Certain. **Not documented in #8** (Q6) |
| A5 | Substance round-trips byte-exact; the write is a JSON-Patch on `PATCH /api/nodes/{id}` | #12984, "Verified against the live graph". Certain |
| A6 | Re-posting byte-identical content **clears** an existing substance | #12984 (A10 re-confirmed live). Certain, and it is §10's staleness story |
| A7 | `cmd/condense` skips `SelfProduced` (`internal/condense/condense.go:283`) and non-prose content types (`:296`, `:298` — `text/*` or empty) | Read from source. Certain |
| A8 | Run records are written type `session-log`, name prefix `processor-run`, content-type `application/json` | `internal/divoid/write.go:14`, `:17`, `:20`, `:80`, `:101`. Certain |
| A9 | `Record.Block` is the entire assembled prompt block | `internal/loop/types.go:170`. Certain |
| A10 | `CandidateLimit = 20`, `AssemblyByteBudget = 60_000`, `SupplementaryByteBudget = 20_000`, `MaxModelCalls = 6` | `internal/loop/turn.go:12-18`. Certain |
| A11 | The graph is live, shared and mutating; every candidate-set figure is one substrate at one instant | Hard constraint (#12955 A5) |
| A12 | A new node type needs no schema change; #9 sanctions new types explicitly, asking only for a linked note | **#9**. Certain |

---

## 4. The measurements this design rests on

### 4.1 The loop cannot see substance, and the graph has almost none

**Coverage, sampled live 2026-09-08** (`fields=id,substance`, `count=500` per stratum):

| population | sampled | of total | carrying substance |
|---|---|---|---|
| any type | 500 | 10,804 | **1** |
| `documentation` | 500 | 6,064 | **6** |
| `session-log` | 500 | 930 | **0** |
| `task` | 500 | 2,445 | **0** |

**On the yardstick query's own top 20: one row (#13101).** The condensation pass exists, works, and has
been run once — over the 25 required nodes of `internal/eval/corpus.json` (#12984). It has never been
pointed at the graph.

**This is the constraint that orders everything below.** Choose any payload rule you like; today it would
render content for nineteen of twenty rows because there is nothing else to render. **The payload argument
cannot be settled by argument. It is blocked on generation.**

### 4.2 What substance costs and buys — measured, and strongly size-dependent

From #12984 (n=24 generated, gemma-3-12b-it at temperature 0, prompt #11373 §8 P1):

| | |
|---|---|
| median ratio `len(substance)/len(content)` | **0.722** (mean 0.655, min 0.250, max 0.983) |
| content **≥ 8,000 B** (n=6) | median **0.298** |
| content **< 4,000 B** (n=9) | median **0.886** |

**The prompt is a removal operation with an identity fixed point** (#11373 §4): on an already-tight node
there is nothing to remove. That is not a defect, and it is the single most useful fact in this design —
**substance compacts big narrative nodes by ~70 % and small dense nodes by ~11 %.** A payload rule that
ignores the stratum spends the fidelity risk uniformly for wildly non-uniform gain.

### 4.3 The yardstick, priced both ways

The seven non-run-record candidates in today's unfiltered top 20, with content measured live and substance
either measured or derived from §4.2's stratum medians:

| node | type | content | substance form |
|---|---|---|---|
| #6375 | documentation | 42,931 B | ≈12,793 B *(derived, r = 0.298)* |
| #6371 | documentation | 28,031 B | ≈8,353 B *(derived, r = 0.298)* |
| #11387 | session-log | 20,019 B | ≈5,965 B *(derived, r = 0.298)* |
| #13101 | documentation | 11,961 B | **5,527 B — measured, r = 0.462** |
| #1836 | task | 2,227 B | ≈1,973 B *(derived, r = 0.886)* |
| #1804 | task | 1,104 B | ≈978 B *(derived, r = 0.886)* |
| #6883 | task | 556 B | ≈492 B *(derived, r = 0.886)* |
| **total** | | **106,829 B** | **≈36,081 B** |

**Against a 60,000-byte budget: today they cannot all fit; in substance form all seven fit with 23,919 B
spare.** Six of the seven figures are derived from measured stratum medians and are marked as such; one is
a real measurement and it lands inside the expected band. **This is Toni's argument, quantified, on the
project's own failing task.**

**Read the mechanism, not the digits** (#13203 §4's discipline). The claim is that substance moves the
eligible set from *does not fit* to *fits with room*. The exact bytes will differ per run and per substrate.

### 4.4 The condenser refuses run records twice, and near-misses a third time

| gate | site | effect on a run record |
|---|---|---|
| `SelfProduced` | `internal/condense/condense.go:283` | skipped before any model call |
| `isProse` — `text/*` or empty | `:296`, `:298` | `application/json` refused |
| **size** — measured, not designed | #12984 skips | #10926 at 195,448 B hit the output ceiling, returned `finish_reason: length`, and was **refused rather than stored truncated**. Run records are 77–88 KB, in the same direction |

**So the class of node with the worst shape in the graph is the one class the condenser structurally cannot
improve.** That is the concrete form of Toni's *"something sounds fishy here"*: we built a compressor,
declined to run it on our own output, and then excluded our own output for being uncompressed.

### 4.5 The fidelity audit failed, and it is zero-tolerance

#12984's F-B: **23 PASS · 1 FAIL · 1 NO SUBSTANCE.** Node #11278's substance dropped the clause *"which a
mutation matrix cannot do because it tests single omissions"* — a **negation binding two mechanisms**. The
sibling node #11271 kept its equivalent sentence, so the loss is inconsistent rather than systematic.

**This is #11373 §1's own predicted failure mode — *a compressor discriminates by salience, not by
load-bearing-ness* — surviving inside the prompt written to prevent it.** #12955 makes F-B a ship gate at
zero tolerance. **It did not pass, and no unit below may treat substance as faithful.** §11 routes this to
the prompt, where it belongs, and gates Unit 3 on it.

### 4.6 A run record is a transcript, and that is why it has no substance to give

Four records decomposed by JSON member:

| node | total | `block` | share | total − `block` |
|---|---|---|---|---|
| #13031 | 87,880 | 62,541 | **71 %** | 25,339 |
| #13154 | 84,089 | 62,036 | **74 %** | 22,053 |
| #11384 | 81,548 | 62,541 | **77 %** | 19,007 |
| #13032 | 77,554 | 62,333 | **80 %** | 15,221 |

`Record.Block` is a verbatim copy of a previous context window — the anchor's full body plus every admitted
candidate's full body. **Three quarters of a run record is a copy of nodes the graph already holds.** A
condensation of that is a condensation of other nodes, which is why the shape has to change before the
condenser can produce anything worth having (§9.3).

### 4.7 Seventeen records, 930 session logs, and one live occupant of the line

`?type=session-log&name=processor-run%` → **17**, dated 2026-09-02 to 2026-09-07. Against **930**
`session-log` nodes, so **913 are human- or peer-written**, and **none of the 500 sampled carries a
substance** (§4.1).

**Rank 20 of the yardstick's unfiltered list is #11387 — a peer-written session log about Processor
traces, 20,019 B.** It is real memory by anyone's definition, it is uncondensed like everything else, and
it is the reason no rule in this document may key on the `session-log` type.

---

## 5. Architectural Overview

**One seam moves: the payload.** Retrieval, fusion, the budget and the ports keep their shape.

```
  ┌── the graph ─────────────────────────────────────────────────────────┐
  │  every node may carry TWO representations:                           │
  │     content    — faithful, authored, always present                  │
  │     substance  — lossy, generated, present only where generated      │
  └──────────────────────────────────────────────────────────────────────┘
             │                                        ▲
   ranked +  │ Unit 1: ask for BOTH                   │ Unit 2: cmd/condense
   addressed │                                        │  offline, deterministic,
     reads   ▼                                        │  over the RETRIEVABLE corpus
  ┌── internal/loop ─────────────────────────────────────────────────────┐
  │  Retrieve → fuse → admit → RENDER                                    │
  │                              │                                       │
  │                              ├─ Unit 3: FORM RULE                    │
  │                              │    substance where it compacts        │
  │                              │    content otherwise                  │
  │                              │    marked, always (S5)                │
  │                              │                                       │
  │                              └─ Unit 4: catalogue + expand (#13106)  │
  └──────────────────────────────────────────────────────────────────────┘
```

**The reframing in one line.** Today the loop asks *which rows do I drop?* After Unit 3 it asks *in which
form does each row travel?*, and dropping becomes what the budget does to whatever is left — which is
Toni's *"automatically degrade irrelevant stuff"*.

**Why the units are in this order.** Unit 1 is a precondition for observing anything. Unit 2 is the binding
constraint (§4.1) and pays off under every payload rule, so it is not gated on choosing one. Unit 3 is the
rule, and it cannot be measured before Unit 2. Unit 4's payload is substance, so it is downstream of Unit 2
by construction — and #13106 §8.3 already says so.

---

## 6. Components & Responsibilities

| Component | Owns | Does **not** own |
|---|---|---|
| **`internal/divoid` (adapter)** | The wire projection of a read — **including asking for `substance`**. Classifying a row's kind by its type | Which representation is rendered. It supplies both and chooses neither |
| **`internal/loop` — `Retrieve`** | The candidate list, complete with both representations | The form rule |
| **`internal/loop` — `admit`** | The byte budget, over **rendered** bytes | Which bytes those are |
| **`internal/loop` — the form rule (new)** | **The single decision: content or substance, per candidate.** One pure function of node properties (S2) | Generation, fidelity, prompts |
| **`internal/loop` — `renderBlock`** | Emitting the chosen form, **marked** (S5) | The choice |
| **`cmd/condense` / `internal/condense`** | Generating substance offline, deterministically, over a named target set. Refusing to store a defective condensation | Being reachable from a turn. #12955's determinism argument stands unchanged and unchallenged |
| **The condensation prompt (#11373)** | Fidelity | Everything else. §4.5's failure is here |

**The one responsibility that is genuinely new** is the **form rule**, and it must be a pure function of
*node properties* — size, presence of substance, ratio — and never of *who wrote the node*. That is S2, and
it is the structural answer to *"how is other agents' logs real memory and ours unreal?"*: after Unit 3
there is no rule in the system that can ask.

---

## 7. Interactions & Data Flow

### 7.1 The read

`Recall`'s projection gains `substance`. Nothing else about the ranked read changes: same route, same
ordering, same limit, same scope handling. **The adapter supplies both representations and marks nothing as
preferred.**

**Cost, and it is not free.** A ranked read today transfers 1,236,611 B on the yardstick query (measured
live; #13091 §6 measured 805,047 B a day earlier — the population grows). Adding `substance` adds to that
while coverage is near zero, and *reduces* it once coverage is real only if content is dropped from the
projection. **Two-phase retrieval — rank without bodies (3,399 B measured), then fetch the survivors'
bodies in one batched addressed read (111,983 B measured for the seven) — costs 115,382 B, 9.3 % of
today's, and composes with everything here.** It is not required by any unit below, and it is the obvious
companion; **Q7**.

### 7.2 The form decision, and where it sits

The form is decided **at admission**, not at retrieval, because it is the budget that makes it matter:

| Step | What happens |
|---|---|
| 1 | Candidates arrive carrying content, substance (possibly absent), type, name, similarity |
| 2 | For each candidate in rank order, the **form rule** (§8) selects content or substance |
| 3 | `admit` tests `cumulative + len(rendered) ≤ remaining`, exactly as today, over the **chosen** form |
| 4 | `renderBlock` emits the chosen form with a header line naming it |
| 5 | The record states, per candidate, `size` (content bytes), `contentHash` (of content), the form, and the rendered size |

**Step 5 is #12955 §6.4 adopted verbatim and it is not negotiable.** `size` and `contentHash` keep their
present meaning, because `eval.scoreOne` compares `contentHash` against the corpus's labelled hash; hashing
a rendered substance instead would mark **every** substance-rendered required node stale — a false rot
signal indistinguishable by inspection from a real one.

### 7.3 What generation does not do

**Generation never happens inside a turn**, and #12955's rejection of in-turn generation is adopted without
qualification: it destroys the determinism every measurement in this project rests on, and it puts up to
twenty model calls before assembly against a six-call cap. Toni's policy — *absent substance is the
trigger* — is right; the **place** is the offline pass. This design does not reopen it.

---

## 8. The form rule

### 8.1 The rule

For each candidate, in rank order:

> **Render `substance` when it is present and materially smaller than content. Render `content`
> otherwise.**

"Materially smaller" is a **threshold on the ratio**, and §4.2 is why it exists rather than being a purity
concern:

| stratum | measured median ratio | rendered form | reason |
|---|---|---|---|
| substance absent | — | **content** | nothing else exists |
| ratio ≥ threshold (small dense nodes; measured median 0.886 below 4 KB) | 0.886 | **content** | ~11 % of bytes is not worth a 1-in-24 fidelity risk (§4.5) |
| ratio < threshold (large narrative nodes; measured median 0.298 at ≥ 8 KB) | 0.298 | **substance** | ~70 % of bytes, and this is the crowding stratum |

**The threshold is a tunable, not a constant to be argued about.** It must ship behind a sweep dial and be
set by the curve, not by taste. A defensible starting point is the midpoint of the two measured strata; the
measurement, not this document, decides it (§11, F-3).

**Why a ratio and not a size.** Size is a proxy; the ratio is the thing. #12984's #11084 is 3,587 B with a
ratio of **0.983** — a small node where condensation did nothing — and its #11228 is 6,820 B at **0.971**.
A size rule would mis-handle both. The ratio is available for free: both representations are in hand.

### 8.2 Where this departs from #12955, and why

#12955 §7.2 states the invariant:

> *"A candidate is rendered as substance only in a turn where, without substance, it would have been
> rendered not at all."*

**That invariant is withdrawn here, deliberately and with the reason stated.**

**Why it was right.** It bounds fidelity risk by construction: a lossy artifact is compared against
*absence*, never against a faithful node, so `admitted ⊇ today's admitted` and no regression is possible.
Written before any fidelity evidence existed, that was the correct first move, and #12955 says as much.

**Why it does not survive its own measurement.** #12984 measured the leftover arithmetic the invariant
depends on: **20 of the 24 generated substances (83 %) exceed the largest leftover ever recorded** across 23
rows (#11365 §1: 26–2,230 B). Only four fit at all, each only at the very top of the range. **At a
60,000-byte budget, two-pass admission has almost nothing it can admit.** Its safety is real and its effect
is nil.

**Why the replacement is not simply "always substance".** Toni's framing — *only use substance, not the
full memory dump* — would render substance on the 0.886 stratum too, spending the measured fidelity risk to
save 11 % on the majority of rows. §4.2 says that trade is bad, and #12984's single FAIL says the risk is
not theoretical.

**So the rule that survives both positions is size-gated substitution**, and #12955 already named it:

> *"What the invariant costs, stated plainly. It forecloses the case where a faithful condensation would
> have been strictly better than a full node… That case is real, it is the larger prize, and this design
> cannot capture it. Capturing it requires exactly the fidelity evidence Unit A produces, and it is Unit
> E."*

**Toni is commissioning Unit E.** Its stated release condition is fidelity evidence from Unit A. That
evidence exists (#12984) and it is **one FAIL short of clean**. §11's F-1 is therefore a hard gate: **Unit 3
does not ship until F-B passes at zero tolerance on a re-run.** That is not inherited caution; it is
#12955's own gate, applied to the unit it was written for.

**What is lost by withdrawing the invariant, stated plainly.** `admitted ⊇ today's admitted` no longer holds
by construction, so a regression becomes possible rather than impossible, and the clean falsifier #12955
built its case on becomes a measurement instead of a proof. That is a real cost and §11's F-2 is what
replaces it.

### 8.3 The rule is over properties, never provenance (S2)

Nothing in §8.1 mentions who wrote a node. **That is the design's answer to Toni's fishiness challenge**,
and it is structural rather than rhetorical: after Unit 3, the system has no place to put a rule of the
form *"nodes we wrote are treated differently"*, because the only rule is a function of size, ratio and
presence.

---

## 9. Exclusion, recast: from provenance to form

### 9.1 The distinction that is withdrawn

This project has been operating on *"a peer's session log is real memory; ours is not"*. **There is no
principled basis for it and it is withdrawn.** #13237 and its predecessors defended the exclusion on
provenance; the defensible statement is about shape:

| | peer session log | run record |
|---|---|---|
| content type | `text/markdown` | `application/json` |
| shape | prose narrative | serialised struct, 71–80 % of it a copy of a prior prompt (§4.6) |
| carries a lesson | by definition (#59) | no |
| condensable today | **yes** — and none has been condensed (§4.1) | **no** — refused twice (§4.4) |

**Both are equally unusable in the block today, for the same reason: neither has a substance.** The peer's
log is 20,019 B of prose that will be cut for bytes; ours is 84,089 B of JSON that is cut by rule. The
difference in treatment is an accident of which cut fires first.

### 9.2 The rule that replaces it

> **A run record may be admitted in substance form. It may never be admitted in content form.**

This is a **form** rule, and it falls out of §8.1 without a special case as soon as run records have a
substance: a 77–88 KB body against a 60,000-byte budget can never be admitted anyway. The explicit
statement exists as a guard for the interval before coverage, and for the case in §9.3 where the record
shrinks.

**PR #48 should still merge**, unchanged. It stops a 62 KB transcript spending a candidate slot, that is
correct today, and Unit 3 is what eventually makes it a statement about form rather than about us.

**What survives of the crowding argument, and what does not.** Toni is right that under substance the
*byte* problem dissolves. The *slot* problem is a separate claim and it is weaker than this project has
been asserting: thirteen of twenty slots go to rows that score 0.722–0.736 because they contain the query
verbatim. **But that is an argument about what those rows say, and once they have a substance it becomes
testable rather than decidable in advance** — which is the honest position, and not the one #13237 took.

### 9.3 What it takes to give a run record a substance

Three obstacles, in the order they must be removed:

1. **The `SelfProduced` gate** (`condense.go:283`). Delete it. It exists to prevent re-admitting a
   transcript; §9.2's form rule does that job properly. **This is the single most consequential line in the
   design.**
2. **The `isProse` gate** (`:296`). A record is `application/json`. Two ways out, and they are not
   equivalent: teach the condenser to accept a **known** JSON shape with its own prompt (narrow, and it
   couples the condenser to the record's schema), or give the record a prose projection. **Recommendation:
   the second** — see item 3, which supplies it for free.
3. **The record's shape.** §4.6: three quarters of a record is a verbatim copy of nodes the graph already
   holds. Condensing that produces a condensation of *other nodes*, which is worthless as memory of the
   run. **What a run's memory should say is what the record already contains outside `block`**: the input,
   the queries, which rows were admitted and cut and why, the answer, the terminal reason, whether the cap
   was hit. That is 15–25 KB of structured fact, and it is exactly what *"a prior run of this task wrote a
   Go project instead of a webpage, because the block held no webpage-related node"* is derived from.

**This is where the earlier "distilled second artifact" idea collapses into something simpler.** A separate
per-run memory node is unnecessary: the record already holds the facts, and the condenser already exists to
distil facts into prose. **What is missing is a prose-shaped projection of the record for the condenser to
read.** Whether that is achieved by dropping `block` from the stored record, storing it separately, or
rendering a prose sidecar is an implementation choice with one architectural constraint attached: PR #8's
ruling that the stored body is the response body minus exactly one key is owned by **#10904** and must be
re-ruled before it is broken (Q4).

### 9.4 The prize, stated so it is not lost

Toni: *"in the end substance of a session log is the best context you can get for your current work — a
history compressed to substantial facts."*

**On this project's own evidence that is very likely true, and the strongest single example is one we
measured.** #13091 §5(a) found that a *single* run record entering the candidate list changed the assembled
block by one row and thereby changed the outcome from *"no webpage"* to *"a webpage"* — while the record
itself was cut and never reached the model. It influenced the run by displacement alone. **A record whose
substance said what that run actually did would be the most directly relevant node in the graph for the
next attempt at the same task** — and today it is refused twice by the one component built to produce it.

---

## 10. Cross-Cutting Concerns

| Concern | Ruling |
|---|---|
| **Determinism** | Preserved absolutely. Generation stays offline (§7.3); a turn reads a value that is already there. Every A/B in this project depends on it |
| **Staleness** | A6: re-posting identical content clears the substance, so the graph guarantees a substance is never older than its content. #12955 §3.2 verified this live and withdrew the sidecar ledger it made unnecessary. **A cleared substance means the form rule falls back to content — degradation, never a wrong render** |
| **Fidelity** | §4.5. Zero-tolerance gate, currently failing. Routed to the prompt (#11373), gating Unit 3 |
| **Observability** | The record gains the form and the rendered size (§7.2 step 5); `size` and `contentHash` keep their meaning. A sweep can then report *bytes saved by form* and *rows whose form changed*, which is what makes F-2 measurable |
| **Error handling** | Substance absent is not an error; it is the common case today. A condensation that fails is not stored (#12984's refusal of the truncated 195 KB node is the correct behaviour and should stay) |
| **Coverage as an operational concern** | Unit 2 is a recurring pass, not a migration. New nodes arrive uncondensed; the graph's substance coverage is a number someone must watch. **Nothing measures it today** (Q5) |
| **Cost** | Generation is model spend proportional to the retrievable corpus. #12984: ~13 minutes for 25 nodes. The retrievable corpus is four orders of magnitude larger and the criterion for it is undefined (Q1) |
| **Security / access** | Unchanged. Substance is a projection of a node the caller can already read |

---

## 11. What must be measured, and what would sink each unit

| # | Gate | Fires against | Status |
|---|---|---|---|
| **F-1** | **Fidelity, re-run at zero tolerance** after the prompt is fixed. Every required node's substance must support its pre-registered `why` | **Unit 3.** A single FAIL blocks substitution outright — this is #12955's own gate | **FAILING** — #12984, 23/1/1 |
| **F-2** | **The regression check that replaces the withdrawn invariant.** Sweep the corpus at several budgets with the form rule on and off; no row may go from *admitted* to *not admitted* | **Unit 3.** Any such row is either a bug or the threshold is wrong | Not run |
| **F-3** | **The threshold curve.** Bytes reclaimed and rows admitted, as a function of the ratio threshold | Sets §8.1's dial. If the curve is flat, the stratum distinction is decoration and a single rule is simpler | Not run |
| **F-4** | **The coverage number.** What fraction of the *retrievable* corpus carries a substance, before and after Unit 2 | **Unit 2.** If a pass over the retrievable corpus cannot reach useful coverage in a schedulable time, Units 3 and 4 are both blocked and should be re-planned, not started | **~0.3 % today** (§4.1) |
| **F-5** | **#13106 §8.3's three-arm differential** — opaque labels vs names-only vs name+substance | **Unit 4.** Ties against either weaker arm sink the catalogue's central claim | Not run; **requires Unit 2 for the twenty rows** |
| **F-6** | **Run-record substance is worth reading.** Condense a run record and have a reader that did not write it judge whether the substance supports *what that run did and why* | **§9.2's form rule as applied to run records.** If the substance is unreadable or vacuous, exclusion stays a provenance rule and this document is wrong about §9 | Not run — **and it is the cheapest of the six** |

**F-6 is the one to run first.** It is a single node, one condensation, one reader, and it decides whether
§9 — the most contested part of this document — is right. It cannot run until the `SelfProduced` gate is
removed, which is one line.

---

## 12. Risks & Mitigations

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| R1 | **Substance ships before fidelity is clean** and the model is confidently told something a lossy pass mangled | F-1 is a hard gate on Unit 3. Unit 1 renders nothing new; Unit 2 writes to the graph but changes no block | Any block containing a substance-form candidate before F-1 passes |
| R2 | **Unit 2 is treated as a one-off migration.** Coverage decays as the graph grows | §10 names it a recurring pass; Q5 asks for the coverage metric | Coverage measured once and never again |
| R3 | **A stale substance renders in place of correct content** | A6: the server clears substance on a content write, verified live. Fallback is content, never a wrong render | A substance surviving a content edit |
| R4 | **`contentHash` is re-based onto rendered bytes**, marking every substance-rendered required node stale | §7.2, adopted from #12955 §6.4 verbatim, with the reason | A sweep reporting corpus-wide staleness after Unit 3 |
| R5 | **The form rule acquires a provenance clause** — "except for nodes we wrote" | S2; §8.3. The rule is a pure function of size, ratio and presence | Any branch in the form rule reading `SelfProduced` or a node type |
| R6 | **A peer session log is excluded** by a rule aimed at run records | Nothing here keys on `session-log`; #11387 is the named live occupant (§4.7) | Any predicate reaching the `session-log` type |
| R7 | **`block` is dropped from the record without #10904 being re-ruled**, or dropped in a way that makes a 15–25 KB transcript *admissible in content form* for the first time | §9.3 item 3 states the constraint; §9.2's form rule is the guard that must exist first | A record shrinking below the budget while §9.2 is unimplemented |
| R8 | **Unit 4 is started before Unit 2** and its falsifier cannot run | F-5's dependency is stated; #13106 §8.3 says the same | A catalogue arm running against rows with null substance |

---

## 13. The node type, and the seventeen rows

**The type change is right, and for the reason Toni gave rather than the one I gave before.** Not to
exclude — **to describe what the node is.** *"types are free — it's a class of document — if the node
content IS something else, then give it another type."* A serialised run transcript is not *"a narrative
record of a meaningful work arc… what was learned"* (#59). Filing it as `session-log` is a false statement
about the node, independent of any filter.

Change the constant at `internal/divoid/write.go:14`; add a `type: run-record` note under the Types group
**#29**, per #9's own instruction to flag a new type. **The seventeen existing rows: node type is not
patchable through the API (`PATCH /api/nodes/{id}` takes `/name`, `/status`, `/X`, `/Y`), so they are
either deleted or left as legacy — the vocabulary is convention and what matters is that it reflects
truth from here on.** One caveat worth thirty seconds: `internal/eval/corpus.json` row **`r05`** anchors on
**#10897**, which is a run record, so deleting that one row breaks a corpus row.

---

## 14. What the earlier analysis got wrong

**Reported, not edited — #13237 is Toni's node.**

| # | Claim | Correction |
|---|---|---|
| 1 | The node answers *"why not track the written ids?"* and concludes *"the mechanism survives"* | Its two measurements are **correct and stand**: a run retrieves at `turn.go:119` before it writes at `:159`, and the crowding rows came from other processes. But it defends **exclusion** as the mechanism, when exclusion is a workaround for a payload defect |
| 2 | *"A dedicated graph-side marker would be stronger than both, **at the cost of a schema change nobody has needed yet**"* | Wrong twice. A distinct node **type** needs no schema change (#9, A12), and it *is* needed — not as a marker but because the current type is a false description (§13) |
| 3 | *"Where the genuine complexity actually is — not in the marking. In the two paths."* | The two-path asymmetry is real (#13203 §5) and it is not the finding. **The finding is that the loop never asks for `substance`** (§4.1) — a capability this project requested, received, designed around, and did not wire up |
| 4 | The framing throughout: peers' session logs are memory, ours are not | **Withdrawn** (§9.1). The difference is shape, not provenance, and shape is fixable |
| 5 | `retrieve.go:85` cited as `main` | At `b021fdc` that line reads `if taken[candidate.ID] {`. The `|| candidate.SelfProduced` clause exists only on `d37a297` (PR #48, unmerged) |

**And one against #12955**, stated in §8.2 rather than here because it is a disagreement rather than an
error: its bounding invariant is withdrawn, on the evidence of the very measurement it commissioned.

---

## 15. Open Questions

| # | Question | Blocking? | Recommendation |
|---|---|---|---|
| **Q1** | **What is the "retrievable corpus"?** Unit 2's target set. Everything? Everything above a size? Everything a recall could plausibly return? #13106 §8.3 flags it as *"the difference is the whole graph"* | **Yes, for Unit 2** | Start from the size stratum where condensation actually pays: **nodes ≥ 8 KB** (median ratio 0.298). That is where the crowding is and where the spend is justified |
| **Q2** | **The ratio threshold** in §8.1 | No — it ships behind a dial | Set it by F-3's curve, not by argument |
| **Q3** | **Does the condenser get a JSON path, or does the record get a prose projection?** (§9.3 item 2) | **Yes, for F-6** | The prose projection — it is reusable and does not couple the condenser to a schema |
| **Q4** | **Does `block` leave the stored record?** Requires #10904 to re-rule PR #8's stored-vs-response invariant | Only for §9.3 item 3 | Yes, but only after §9.2's form rule exists — otherwise a 15–25 KB transcript becomes admissible for the first time (R7) |
| **Q5** | **Who measures substance coverage, and how often?** Nothing does today | No | A line in the sweep report; it is one query |
| **Q6** | **`fields=substance` works on the listing route and is undocumented in #8** | No | One line in #8 by whoever touches it next. Named because A4 rests on it |
| **Q7** | **Two-phase retrieval** — rank without bodies, then batch-fetch the survivors. Measured 1,236,611 B → 115,382 B on the yardstick (§7.1) | No | Its own unit, any time. It composes with every payload rule and depends on none of them |

---

## 16. Implementation Guidance for the Next Agent

**No code in this document. Each unit is its own branch and its own PR.**

### Unit 1 — the loop can see substance *(ships first, changes no block)*

1. Add `substance` to the recall projection (`internal/divoid/client.go:34`). The route already accepts it
   (A4); an unknown field returns 400, so a wiring error is loud.
2. Carry it on `loop.Candidate`. Absent is the common case and must be the zero value, not an error.
3. Carry it through fusion and admission untouched. **Render nothing differently yet.**
4. Record, per candidate: whether a substance was present and its size. **That is Unit 1's whole product** —
   it turns F-4 from an offline probe into something a sweep reports.
5. Guard: a candidate set carrying no substance anywhere produces a byte-identical block to today's. This is
   #12955's pass-1 identity property, and it is the reason Unit 1 is safe to ship alone.

### Unit 2 — coverage *(the binding constraint; mostly not code)*

1. Answer **Q1**. Recommendation: nodes ≥ 8 KB first.
2. Extend `cmd/condense`'s target resolution from a corpus or an id list to that set. Idempotent and
   `-force`-guarded already.
3. **Rule the oversized case.** #12984 refused #10926 at 195,448 B rather than storing a truncated
   condensation, and that refusal is correct. A set defined by size will hit more of them; the pass needs an
   answer beyond refusing, and #11373 §5c hands it to this binary.
4. Run it, and publish **F-4**: coverage before and after, wall clock, spend.
5. **Do not start Unit 3 on the strength of a plan to run this.** Run it.

### Unit 3 — the form rule *(gated on F-1)*

1. **Do not start until F-1 passes at zero tolerance.** The prompt fix is Kim's (#11373); §4.5 is the brief.
2. Implement §8.1 as a pure function of size, ratio and presence. **No provenance branch** (R5).
3. Threshold behind a sweep dial; set it from F-3.
4. `size` and `contentHash` keep their meaning (§7.2, R4). Add form and rendered size.
5. Mark every substance-rendered candidate with one header line. It names the form; it does not editorialise
   (#12955 §6.3 — how the system text should treat a marked block is Kim's Q4, not this design's).
6. Run **F-2** and publish it. The withdrawn invariant made regression impossible; this measurement is what
   replaces the proof.

### Unit 3a — run records become condensable *(smallest, and it decides §9)*

1. Remove the `SelfProduced` gate at `internal/condense/condense.go:283`. One line.
2. Resolve **Q3**, then **run F-6** on a single record. **This is the cheapest experiment in the document
   and it decides whether §9 is right.** If a run record's substance turns out to be vacuous, exclusion
   stays a provenance rule and §9 is wrong — say so, and correct this document in place.
3. Only after F-6: state §9.2's form rule as a guard, and consider **Q4**.

### Unit 4 — the catalogue

**#13106 §8.3, unchanged.** Do not start before Unit 2 delivers substance for the twenty rows F-5 needs, and
note that #13106 already establishes it probably requires the call ceiling raised.

### What must not happen

- No substance rendered into a block before **F-1** passes.
- No provenance clause in the form rule (S2, R5).
- No predicate keying on the `session-log` type (R6) — #11387 is its live occupant.
- No `block` removal before §9.2's form rule exists and **#10904** has re-ruled (R7).
- No in-turn generation (§7.3).
