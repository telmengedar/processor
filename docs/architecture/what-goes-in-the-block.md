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
| **But the graph has almost no substance to give** | Sampled live: **1 of 500** random nodes, **6 of 500** documentation, **0 of 500** session-logs, **0 of 500** tasks. On the yardstick's own top-20, **1 of 20**. `cmd/condense` has only ever been pointed at the 25 required nodes of the eval corpus (#12984). **That is not a data gap — it is the absence of a behaviour** (§7.3), and it is what Unit 2 becomes. |
| **Where it would pay is exactly where the crowding is** | The yardstick's seven real candidates total **106,829 B against a 60,000-byte budget** — they cannot all fit. Applying #12984's measured stratum ratios: **36,081 B in substance form, all seven fit, 23,919 B spare.** (One row is a real measurement: #13101, 11,961 B → 5,527 B, ratio 0.462.) |
| **And the condenser refuses the class that needs it most** | `internal/condense/condense.go:283` skips `SelfProduced`; `:296`'s `isProse` refuses `application/json`. **A run record is refused twice.** A third gate is measured, not designed: at 195 KB, node #10926 could not be condensed at all — truncated and refused (#12984), and run records are 77–88 KB. |

**The recommendation, in four lines.**

| | |
|---|---|
| **Unit 1 — See it** | Request `substance` in recall; carry it on `Candidate`; record which form was rendered. **No admission-policy change.** Small, and nothing downstream is measurable without it. |
| **Unit 2 — Generate it, on demand** | **A behaviour of the memory core, not a backfill campaign.** Retrieval finds a node it wants to push, finds no substance, and makes one. **Bounded by the cache — cost is proportional to novelty, not traffic** — plus a size gate, a pressure gate, and a transition-only ceiling. §7.4. |
| **Unit 3 — Render it, size-gated** | Substance replaces content **where condensation actually compacts** — the ≥8 KB stratum, measured median ratio **0.298**. Below 4 KB the measured ratio is **0.886**: substance saves ~11 % and spends fidelity, so content stays. |
| **Unit 4 — The catalogue** | #13106 §8.3, unchanged and still third. Its payload *is* substance, so it is downstream of Unit 2 by construction, and its three-arm falsifier cannot run until the fill has warmed the twenty rows. |

**This crosses an invariant, and the crossing is the design's main claim.** `internal/condense/condense.go:1`
says the pass *"is never reachable from a turn"*. Two things make moving it defensible.

**What bounds a turn's spend is the cache, not a cap.** Substance is stored on the node, so **cost is
proportional to novelty, not to traffic** — a presence check in an active area, one condensation per *new*
node above the size gate, paid once ever. Three regimes, priced separately in §7.4.1. **But we are entirely
inside the transition** — 0 of 500 session-logs, 0 of 500 tasks — so the first run on a novel area pays
about **three fills, ~90 s**, against a turn that runs 33–81 s. *"It converges to free"* is true and is the
wrong sentence to read before the first run (§7.4.2).

**What keeps the instrument clean is structural, and measured rather than asserted.** #12955's **primary**
objection was never latency — it was **determinism**. `cmd/eval`'s entire dependency closure is
`boot, loop, divoid, eval`: **no model adapter, no `condense`, so the sweep cannot fill.** Put the fill
behind a port the loop declares and only `cmd/processor` constructs — exactly as `FilePort` already works,
and forced anyway, since `loop` importing `condense` is a compile cycle — and **the instrument is
structurally incapable of contaminating the substrate it measures** (§7.4.4).

**The fill's model is a defined requirement per role, not an open question.** The turn's model must support
**tool calling** — a model without it cannot act. The fill's model must be a capable **semantic
compressor** — a model without it cannot summarise without inventing. Same shape of requirement, defined
rather than discovered; nobody benchmarks whether the turn's model can call tools. **The asymmetry is why
this bar sits higher: a bad turn produces a bad answer and it is transient — the run ends and nothing
survives. A bad substance produces a bad fact that persists on the graph, consumed as truth by later runs
that never reach the node it came from.** #12984 is what that looks like when it happens: the single FAIL
dropped a *negation*, which reads as a fact rather than as an omission. **F-1 is the check that the defined
model meets the bar**, re-run whenever the model or prompt changes — and **no provenance machinery is
proposed**, because the answer to *what if the model is wrong* is to define the requirement, not to
instrument for having failed to. One structural consequence follows: `cmd/processor` and `cmd/condense`
both call `boot.LoadModel()` today and differ only because they are separate **processes** — exactly how
#12984 ran gemma while #13091 ran qwen. **One process erases that**, so the fill takes its own
configuration, and **absent one the fill is off** rather than silently falling back to the tool-calling
model (§7.4.7).

**And the property that matters most is not coverage — it is self-healing.** Substance is derived state, so
it is lost whenever content changes, and **our own parity sync is half an operation**: it republishes a
body and drops the derived form. Verified today — **#13203 `null`, #12955 `null`, and #13238, this very
document, `null`.** Three for three, including the design that argues substance is the thing we lack. Under
the fill that repairs itself: the next retrieval that wants the node notices the absence and fills it. **No
guard, no re-derive step in any procedure, no discipline anyone has to remember**, and it holds for every
cause — never generated, invalidated by a legitimate edit, dropped by a republish, lost in a migration.
It also means **§4.1's 1-in-500 is a floor on the problem, not a measure of it** (§7.4.5).

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
| A13 | **`cmd/eval` cannot make a model call.** Its whole dependency closure is `internal/boot`, `internal/loop`, `internal/divoid`, `internal/eval` — no `openaicompat`, no `ollama`, no `condense` | `go list -deps ./cmd/eval` at `da01ee8`. Certain, and §7.4.4's determinism repair rests on it |
| A14 | **`internal/loop` cannot import `internal/condense`** — it is a cycle. `internal/condense/targets.go:6` imports `internal/eval`, which imports `internal/divoid`, which imports `internal/loop` | `go list -deps` at `da01ee8`. Certain, and it forces the fill to be a port the loop declares |
| A15 | The project already has an absent-by-default port that refuses with a recorded reason: `FilePort`, declared at `internal/loop/turn.go:60`, documented *"nil refuses every write"* at `:81`, guarded at `:317` | Read from source. Certain — it is the precedent §7.4.4 follows |
| A16 | **Substance is derived state and is invalidated whenever content is written.** In every case observed on real work the content had genuinely changed, so the invalidation was **correct** | #12984 also measured the degenerate byte-identical case; nobody performs one, so it is a curiosity rather than an exposure (§7.4.5). Certain |
| A17 | **UNKNOWN: whether `substance` participates in similarity ranking.** If it does, a fill changes *retrieval* and not merely *rendering*, and §7.4.4's argument weakens sharply | **Not established.** F-7 is the gate and it must fire before any fill ships |
| A18 | **The turn and the condenser share one model configuration.** `cmd/processor/main.go:41` and `cmd/condense/main.go:56` both call `boot.LoadModel()`, reading the same `PROCESSOR_MODEL_*` members | Read from source at `f774c37`. Certain. They differ today only because they are separate **processes** — #12984 ran `ai/gemma3` at `:12434`, #13091 ran `qwen3-coder:30b` at `:11434` |
| A19 | **The condensation pass knows which model produced each substance and drops it at the graph boundary.** `Provenance` carries `Model` and `Sampling` (`internal/condense/condense.go:107-108`), but `SetSubstance` writes exactly one patch op — `/substance` (`internal/divoid/substance.go:67-81`) | Read from source at `f774c37`. Certain. **Recorded, no longer load-bearing:** it was checked while costing provenance-on-the-node, which §7.4.7 withdraws |
| A20 | **DiVoid has no field for substance provenance.** A node carries `substance` as an opaque string; `PATCH /api/nodes/{id}` exposes no provenance path | **#8** and the MCP patch surface. Certain. **Recorded, no longer load-bearing** — same reason as A19 |

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

**And these numbers are a floor, not a measurement.** Part of the zero is not *never generated* but
*generated and then discarded* — a design document's substance is written once and then dropped by the next
parity re-sync, with three confirmed instances in §7.4.5, one of them this document. The true shortfall is
larger than the sample shows, and it is why §7.4.5 treats self-healing as the property that matters rather
than coverage.

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
   ranked +  │ Unit 1: ask for BOTH                   │ Unit 2: THE FILL (§7.4)
   addressed │                                        │  a memory-core behaviour:
     reads   ▼                                        │  no substance where one is
                                                      │  wanted → make it, once,
                                                      │  behind a port cmd/eval
                                                      │  structurally cannot build
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

**Why the units are in this order.** Unit 1 is a precondition for observing anything. Unit 2 is what puts
substance in the graph at all, and it pays off under every payload rule, so it is not gated on choosing one.
Unit 3 is the rule, and it cannot be measured before Unit 2 has warmed something to measure it on. Unit 4's
payload is substance, so it is downstream of Unit 2 by construction — and #13106 §8.3 already says so.

**Unit 2 is no longer a scheduled pass over a declared corpus. It is a behaviour**, and that is the change
this revision makes: coverage is discovered by retrieval rather than planned by a person, which is why §15's
Q1 could be struck instead of answered.

---

## 6. Components & Responsibilities

| Component | Owns | Does **not** own |
|---|---|---|
| **`internal/divoid` (adapter)** | The wire projection of a read — **including asking for `substance`**. Classifying a row's kind by its type | Which representation is rendered. It supplies both and chooses neither |
| **`internal/loop` — `Retrieve`** | The candidate list, complete with both representations | The form rule |
| **`internal/loop` — `admit`** | The byte budget, over **rendered** bytes | Which bytes those are |
| **`internal/loop` — the form rule (new)** | **The single decision: content or substance, per candidate.** One pure function of node properties (S2) | Generation, fidelity, prompts |
| **`internal/loop` — `renderBlock`** | Emitting the chosen form, **marked** (S5) | The choice |
| **The fill port (new)** | **Declared by `internal/loop`** (it must be — importing `internal/condense` is a cycle, A14). One operation: *this node has no substance and I want to push it; make one.* Absent by default; every refusal carries a reason | Deciding *whether* to fill — that is the gate set (§7.4.3). It also never decides what is rendered |
| **`cmd/condense` / `internal/condense`** | The condensation itself: the prompt, the audit, the refusal to store a defective result. Reachable **both** offline and, via the port, from a turn | The gates, the ceiling, and the graph-wide campaign it is no longer asked to run |
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

### 7.3 Where generation happens — the invariant, and why this document moves it

**A previous revision of this document said generation never happens inside a turn, adopting #12955's
rejection without qualification. That is withdrawn.** Toni's mechanism is a behaviour, not a campaign:

> *"it's actually a task of our memory core — it sees a matching node it wants to push, it sees that there
> is no substance available, it creates substance. Obviously there is no substance available yet — no one
> but us knows about it."*

**The reframe is correct, and it changes what §4.1's measurement means.** 1-in-500 is not a data gap to be
closed once. It is **the absence of a behaviour**. We invented the field, so of course nothing carries one;
a backfill treats the symptom and leaves the system in the same state for every node created after it runs.
Under a behaviour, coverage grows where retrieval actually goes and nowhere else, a node nobody recalls
never costs a condensation, and **Q1 dissolves — the retrievable corpus is whatever retrieval retrieves,
discovered rather than declared.**

**The invariant this crosses is ours, and `internal/condense/condense.go:1` states it in terms:** *"Package
condense is the offline pass that derives a node's substance from its content, and is never reachable from
a turn."* §7.4 is what replaces it.

### 7.4 The fill — condensation as a memory-core behaviour

**This is the first behaviour of the memory core.** The map (#10454) records that this repository has none.
This unit is where one starts, and that should be claimed openly rather than smuggled in as a retrieval
tweak.

#### 7.4.1 The economics, which are not what a per-turn cap would suggest

> *"condensation and substance have an implicit cache — as long as the content isn't changed it stays on the
> graph and doesn't need to be recreated — yes if your process reaches into unexplored memory you have to
> condense a lot and that takes a bit, but if you are in active areas of static truth it's basically free
> and reduced to a simple check."* — Toni, 2026-09-08

**That is right, and it is a better bound than any cap, because it is structural rather than imposed.** A
condensation is a pure function of a node's content, cached in the graph in the substance field itself.
**Cost is therefore proportional to novelty, not to traffic.** Three regimes, and the design names them
separately because they behave nothing alike:

| regime | what a turn pays | bound |
|---|---|---|
| **Steady state** — an active area of static truth | a **presence check**. Zero model calls | self-limiting: converges to free |
| **Cold start** — retrieval reaches into unexplored memory | one condensation per *new* node above the size gate, paid **once, ever** | the count of distinct unmet nodes, which shrinks monotonically |
| **Churn** — content changes, so the derived form is correctly discarded | re-condensation of the affected node, on next use | **self-healing, not self-limiting — §7.4.5** |

**The framing this replaces was mine and it was wrong.** An earlier draft priced the worst case as *twenty
candidates, twenty condensations* against a per-turn budget. That is a per-**turn** cost only if nothing is
cached; it is in fact a per-**node** cost paid once. On a stable working area the second run and every run
after it fire zero fills.

#### 7.4.2 But we are entirely inside the transition, and that must not be averaged away

**The graph is 100 % cold start today.** §4.1: 1 of 500 random nodes, 6 of 500 documentation, **0 of 500
session-logs, 0 of 500 tasks**. So the convergence argument is a claim about **where this ends up**, not
about **what the next run costs**, and the two must not be conflated in anyone's planning.

**The expected shape of the transition, from this project's own numbers.** On the yardstick, three of the
seven real candidates are ≥ 8 KB (§4.3). At #12984's measured ~31 s per node (13 minutes for 25):

| run | fills | added latency | against a turn that runs 33–81 s (#13091 §1) |
|---|---|---|---|
| first, on a novel area | ~3 | ~90 s | roughly a doubling to tripling |
| second, same area | **0** | **0** | unchanged |
| a year in, static area | 0 | 0 | a presence check |

**Read that as a transition cost with a shape, not as an amortised average.** *"It converges to free"* is
true and is a misleading sentence to put in front of someone about to make the first run.

#### 7.4.3 The three options, and the recommendation

| | Option | Ruling |
|---|---|---|
| 1 | **Synchronous fill** — retrieval blocks on condensing what it wants to push | **ADOPTED.** §7.4.1's cache is what makes it defensible; the gates below make it shippable |
| 2 | **Lazy trigger, asynchronous fill** — notice the gap, enqueue, use what is there this time | **REJECTED** on two grounds, §7.4.6 |
| 3 | **A bound on the synchronous case** | Not an alternative to 1 — it *is* 1. Three gates, and the third is demoted from what an earlier draft made it |

**Three gates, and their roles are different.** Two are economic — do not spend where it does not pay. One
is a safety bound for the transition, and it is explicitly **not** what bounds steady state, because
§7.4.1's cache already does that.

| # | Gate | Value | Role |
|---|---|---|---|
| **G1** | **Size** — fill only where condensation is measured to pay | content **≥ 8 KB** | *Economic.* §4.2: median ratio 0.298 at ≥ 8 KB against 0.886 below 4 KB. Below the gate a fill spends a model call to save ~11 % and buys a fidelity risk. Not a new concept — it is the form rule's own stratum (§8.1) |
| **G2** | **Pressure** — fill only when the budget is actually exceeded | content-form admission < candidate count | *Economic.* **This is Toni's "a node it *wants to push*"** — wanting to push means competing for a slot it cannot get. If everything fits as content there is nothing to gain and no fill fires |
| **G3** | **Per-turn fill ceiling** | recommend **2**, and **retire or raise it once coverage crosses a threshold** | *Safety, transition-only.* **Demoted.** It does not bound steady state — the cache does. It bounds **one cold-start turn**, so a first run into unexplored memory cannot spend ten minutes. Q8 |

**G3 is a transition instrument and the design says so**, because a permanent constant that exists to
survive a temporary regime is how a system ends up with limits nobody can explain. Its retirement condition
belongs in the same commit that introduces it: when the coverage metric (Q5) shows the working set is warm,
raise it or delete it.

**Fills must not consume `MaxModelCalls`.** That cap governs the model's *reasoning* budget — six calls in
which to answer. A fill is infrastructure, not judgement. Spending a judgement call on a condensation would
make a turn's reasoning budget depend on how condensed the graph happens to be, which is exactly the
coupling this project keeps eliminating elsewhere. **Separate counter, separate ceiling, both in the
record.**

#### 7.4.4 The seam, and why the instrument stays clean

This is #12955's **primary** objection to in-turn generation, and it was never latency — it was
**determinism**: every A/B here needs byte-identical candidate lists across arms, and a turn that writes
into the substrate destroys that.

**The loop declares a fill port; it cannot do otherwise.** Importing `internal/condense` from
`internal/loop` is a compile cycle (A14). The forced shape is the right one and it is already this
project's: **`FilePort`** (A15) — declared by the consumer, implemented outside, **nil means the capability
is absent and every call is refused with a recorded reason**, so the shape of a trace does not change with
the environment, only the outcome does (#10454 on PR #43).

| binary | constructs a fill port? | consequence |
|---|---|---|
| `cmd/processor` | **yes** | turns fill |
| `cmd/eval` | **cannot** — no model adapter anywhere in its closure (A13) | **the sweep reads substance and creates none** |

**That is the determinism repair, and it is structural rather than procedural.** Every A/B stays runnable:
pin the substrate, sweep both arms, no fill can fire, candidate lists stay byte-identical — which is
#11365's stated requirement. It is a stronger guarantee than the convention it replaces, because a
convention can be forgotten and a dependency closure cannot.

**What is genuinely lost, stated plainly.** Two *turns* on the same input now differ: the first fills, the
second reads. That is new and real. Three things bound it: it was **already true** (#13091 §5(a) measured a
single run record changing a block); it **converges** rather than diverges, because a filled node stays
filled; and the record says which fills fired, so a trace **explains** the difference instead of hiding it.

**The refusal contract matters more than it looks.** A fill that does not happen must say why — port absent,
size gate, pressure gate, ceiling reached, condenser refused (§9.3), model failed. A silent no-op would make
§9.3's two gates invisible at exactly the moment they bite.

#### 7.4.5 Substance loss, and why the fill makes it self-healing

**An earlier revision of this document called this an invalidation trap and proposed a change request
against DiVoid. Both are withdrawn.** The claim was that substance is cleared on a content *write* rather
than a content *change*, so a byte-identical republish destroys it. The mechanism is real (#12984 measured
it) and the exposure is not, because nobody republishes identical bytes. Toni's ruling:

> *"why would you write content byte identically? … replace the same text with the same, that's the most
> expensive noop I've ever heard of… in 'makes sense' paths this is really just an unnecessary operational
> check."*

**A hash gate would defend against a caller doing something no caller should do. No ask against DiVoid.**

**The real defect is ours, and checking for it found it.** Every invalidation observed on real work was
**correct** — the content genuinely changed, so the derived form was genuinely stale. What is wrong is that
**our parity sync is half an operation**: it republishes the body and drops the derived form on the floor.

**Measured on the live graph, 2026-09-08:**

| node | what it is | `substance` |
|---|---|---|
| **#13203** | the PR #48 decision record, re-synced across four review rounds | **`null`** — written once at creation, cleared four times, never written back |
| **#12955** | **the design document about substance-backed admission** | **`null`** |
| **#13238** | **this document** | **`null`** |

Three for three, including the two that argue substance is the thing this project is missing. Nobody was
careless; the procedure simply has no step for it, and a derived form with no owner decays to null.

**There are two ways to fix that, and only one of them is architecture.**

| | Fix | Why not / why |
|---|---|---|
| A rule | *"the parity sync must also re-derive substance"* | One more step in a procedure, forgotten the first time somebody is in a hurry — and it covers **our** syncs only, not migrations, not manual edits, not a peer agent rewriting a node |
| A behaviour | **the fill** | The next retrieval that wants the node notices the absence and makes one. **Nothing to remember, and it does not care what caused the loss** |

**So the fill's strongest property is not coverage. It is that substance loss is self-healing wherever it
comes from** — never generated, correctly invalidated by an edit, dropped by a republish, destroyed in a
migration, cleared by hand. Coverage says *the graph gets warmer*; self-healing says *there is no way to
lose substance that the system does not repair on next use*. The second is strictly stronger, and it is the
honest answer to *what happens when substance goes stale* — an answer this design would otherwise have to
give as process.

**The boundary of that claim, stated because it is the boundary of this document's strongest argument.**
**Self-healing repairs *absence*. It does not repair *wrongness*.** A substance that exists but is wrong —
a load-bearing negation dropped, a qualifier lost — is not missing, so the fill sees nothing to do and
heals it into nothing. It is **stable, indistinguishable from good substance, and consumed as fact**. That
That is the **durability asymmetry**, and it is why the generating model is a **defined requirement**
rather than a setting (§7.4.7).

**One mechanism does repair it, and its coverage is exactly inverted from where the design wants it.** A16:
a content write clears substance, so a bad substance is destroyed the moment its node is edited. **Therefore
bad substance is durable precisely on nodes whose content is static** — and *"active areas of static truth"*
is the regime §7.4.1 names as the fill's best case. **The exposure concentrates exactly where the benefit
is claimed.** That is not an argument against the fill; it is why §7.4.7's requirement is defined up front
and qualified by F-1 before the model is allowed to write.

**This document therefore declines to add a "the sync must re-derive substance" requirement.** Naming it
would be process where a behaviour already suffices, and process is exactly what gets forgotten — as the
three rows above demonstrate.

**And it changes what §4.1 measures.** Some of that zero is not *never generated* but *generated and then
destroyed by our own workflow*, with three confirmed instances above. **1-in-500 is a floor on the problem,
not a measurement of it.**

#### 7.4.6 Why not asynchronous

Option 2 is rejected on two grounds, and the second is the one that matters.

1. **It does not preserve the invariant.** A queue a turn writes to *is* reachable from a turn; it defers
   the crossing, it does not avoid it. #12955 ruled on this exact option: *"keeps a non-deterministic writer
   coupled to the turn that read it, and introduces a queue, a worker lifecycle, and a failure mode inside a
   service that currently has none. All the offline pass's benefits, none of its isolation."* Nothing in the
   new evidence overturns that, and §7.4.4 achieves the isolation async was reaching for — without the
   worker.
2. **It does not satisfy the intent.** Toni's mechanism is that *this* turn pushes the node. Async means the
   turn that discovered the gap renders content or cuts the node exactly as today, and only a later run
   benefits. On a repeated task that converges; **on a new question about a new area — precisely the
   cold-start regime where the gap exists — it never closes in time.** It optimises the case that already
   worked.

#### 7.4.7 The generating model is a defined requirement, not an open question

> *"the model question is something we have to just define. A good model makes a good working process, a
> bad model generates wild confusing actions — but in the end that's the same as for the loop — a model
> without tool support is worthless — a chat model can not implement code, only that here it's one level
> more important as the result lives on the graph."* — Toni, 2026-09-08

**This is a requirement per role, stated the way the turn's model requirement already is.** Not a study, not
a benchmark, not a matrix of candidates to evaluate.

| role | required capability | a model without it |
|---|---|---|
| **the turn's model** | **tool calling** and instruction following | cannot act — it can describe writing a file, never write one |
| **the fill's model** | **semantic compression**: capable, gemma-class; explicitly **not a small cheap chat model** | cannot summarise without inventing — it produces a confident paraphrase with facts missing |

Both are **defined, not discovered.** Nobody benchmarks whether the turn's model can call tools; it either
is that kind of model or it is the wrong one. The fill's requirement has exactly that shape.

**The asymmetry, and it is the reason this bar sits higher than the turn's.** *"One level more important as
the result lives on the graph."* **A bad turn produces a bad answer and it is transient** — the run ends,
the answer is judged, nothing survives. **A bad substance produces a bad fact that persists**, is consumed
as truth by every later run, and reaches runs that never touch the node it came from. The blast radius of a
bad turn is one turn; the blast radius of a bad substance is the graph.

**#12984 is what that failure looks like when it happens.** The zero-tolerance audit failed 23/1/1, and the
FAIL (#11278) dropped *"which a mutation matrix cannot do because it tests single omissions"* — a
**negation binding two mechanisms**. An omission a reader would notice is survivable; **a polarity
inversion that reads as a fact is not**, and it is durable. #11373 §1 predicted the mechanism — *a
compressor discriminates by salience, not by load-bearing-ness* — and it happened inside the prompt written
to prevent it.

**F-1 is the check that a defined model actually meets the defined bar**, and it is the only instrument here
that can tell an adequate semantic model from an inadequate one. It re-runs whenever the fill model or the
prompt changes. **It is not a search for a model; it is a qualification of the one chosen.**

**No provenance machinery is proposed.** An earlier revision costed three ways to attribute a substance to
the model that wrote it — a header line in the payload, a field on the node, a generator-identity node —
as a hedge against getting the model wrong. **That is withdrawn.** The answer to *what if the model is
inadequate* is to define the requirement and qualify against it, not to instrument for having failed to.
A19 and A20 record what was checked and no longer drive a recommendation. What remains is free and is not
machinery: **the run record names the model that produced each fill**, exactly as it already names the
model that produced the answer — a record accounting for its own composition, which is `run-record-fate.md`
(#10904)'s existing thesis rather than a new mechanism.

**The configuration consequence stands, and it is structural rather than defensive.** A18:
`cmd/processor/main.go:41` and `cmd/condense/main.go:56` **both call `boot.LoadModel()`**, reading the same
`PROCESSOR_MODEL_*` members. They differ today **only because they are separate processes** — which is how
#12984 ran `ai/gemma3` at `:12434` while #13091 ran `qwen3-coder:30b` at `:11434`. **Under the fill they are
one process and that separation silently disappears.** Two roles, two requirements, therefore **two
configurations**: a fifth boot loader and its own `PROCESSOR_CONDENSE_MODEL_*` members.

**And it does not fall back to the turn's model. Absent a configured condensation model, the fill is off.**

| | Ruling |
|---|---|
| **Fall back to `PROCESSOR_MODEL_*`** | **Rejected.** A silent fallback to the tool-calling model is the failure the requirement exists to prevent: the system would keep working, condense with whatever chat model happened to be configured, and write the result to the graph as fact |
| **Absent config ⇒ capability absent** | **Adopted.** It is `FilePort`'s own contract (A15) — nil means every call is refused **with a recorded reason**, so a trace reads *"no condensation model configured"* rather than quietly degrading. The requirement becomes enforceable by construction: you cannot accidentally get substance from the turn's model |

**Second port, or second configuration on the same one?** **Neither, exactly: one port on the loop side, its
own model client behind the seam.** The loop asks *fill this node*; which model answers is not the loop's
concern and must not enter its interface — the same inversion `GraphPort`, `ModelPort` and `FilePort`
already discharge. A second `ModelPort` on `Turn` would put provider configuration into the loop's
constructor for a capability the loop does not reason about.

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
| **Determinism** | **Preserved where it is load-bearing, and lost where it is not.** The *instrument* is clean by construction: `cmd/eval` has no model adapter in its closure, so a sweep cannot fill (A13, §7.4.4), and candidate lists stay byte-identical across arms. Two *turns* on the same input now differ on first encounter; that converges, is recorded, and was already true of run records (#13091 §5(a)) |
| **Staleness** | A6/A16: the graph clears substance whenever content is written, so a substance is never older than its content and #12955 §3.2's sidecar ledger stays withdrawn. **A cleared substance means the form rule falls back to content — degradation, never a wrong render.** Under the fill it is also **temporary**: the next retrieval that wants the node re-derives it (§7.4.5), so staleness needs no procedure |
| **Fidelity** | §4.5. Zero-tolerance gate, currently failing. Routed to the prompt (#11373), gating Unit 3 |
| **Observability** | The record gains the form and the rendered size (§7.2 step 5); `size` and `contentHash` keep their meaning. A sweep can then report *bytes saved by form* and *rows whose form changed*, which is what makes F-2 measurable |
| **Error handling** | Substance absent is not an error; it is the common case today. A condensation that fails is not stored (#12984's refusal of the truncated 195 KB node is the correct behaviour and should stay) |
| **Coverage as an operational concern** | **Not a pass at all — a behaviour** (§7.3). Coverage grows where retrieval goes; a node nobody recalls is never condensed. What must still be watched is the **warmth of the working set**, because it is what retires G3 and what §7.4.5's trap silently destroys. **Nothing measures it today** (Q5) |
| **Cost** | **Proportional to novelty, not to traffic** (§7.4.1). Steady state is a presence check; cold start is ~31 s per new node above the gate (#12984), paid once. **Today the graph is 100 % cold** (§7.4.2), so early runs pay near the worst case — a transition cost with a shape, never an amortised average |
| **Security / access** | Unchanged. Substance is a projection of a node the caller can already read |

---

## 11. What must be measured, and what would sink each unit

| # | Gate | Fires against | Status |
|---|---|---|---|
| **F-1** | **Model qualification.** #12984's audit, re-run at zero tolerance: every required node's substance must support its pre-registered `why`. **It is not a one-time release gate — it is the instrument that qualifies a *model*, and it re-runs whenever the fill model or the prompt changes** (§7.4.7) | **Unit 2 *and* Unit 3, and every model change thereafter.** A single FAIL disqualifies that model. **Note it now gates generation, not only rendering** — under the fill an unqualified model writes to the graph as a side effect of serving traffic, where the offline pass could be re-run and its output discarded | **FAILING** — #12984, 23/1/1, on `ai/gemma3`. That result qualifies *that model with that prompt*, nothing else |
| **F-2** | **The regression check that replaces the withdrawn invariant.** Sweep the corpus at several budgets with the form rule on and off; no row may go from *admitted* to *not admitted* | **Unit 3.** Any such row is either a bug or the threshold is wrong | Not run |
| **F-3** | **The threshold curve.** Bytes reclaimed and rows admitted, as a function of the ratio threshold | Sets §8.1's dial. If the curve is flat, the stratum distinction is decoration and a single rule is simpler | Not run |
| **F-4** | **Convergence, not coverage.** Run the same task twice against a cold area: run 1 fills, **run 2 must fire zero fills and reach the same or a better admitted set** | **Unit 2.** If run 2 still fills, the cache is not doing what §7.4.1 claims and the whole economic argument collapses to per-turn cost | Not run. **~0.3 % coverage today** (§4.1) |
| **F-5** | **#13106 §8.3's three-arm differential** — opaque labels vs names-only vs name+substance | **Unit 4.** Ties against either weaker arm sink the catalogue's central claim | Not run; **requires Unit 2 for the twenty rows** |
| **F-6** | **Run-record substance is worth reading.** Condense a run record and have a reader that did not write it judge whether the substance supports *what that run did and why* | **§9.2's form rule as applied to run records — and now the fill path too.** With the memory core condensing on demand, `condense.go:283`'s `SelfProduced` skip and `:296`'s `application/json` refusal are the two rules that make the core **structurally unable** to condense the class Toni most wants condensed. If the substance is vacuous, exclusion stays a provenance rule and §9 is wrong | Not run — **and it is the cheapest of the eight** |
| **F-7** | **Does `substance` participate in similarity ranking?** Record a node's similarity for a fixed query, write a substance, re-query | **The fill, outright.** If ranking moves, a fill changes *retrieval* and not merely *rendering*, retrieval becomes path-dependent, and §7.4.4's repair is insufficient — the sweep would still read a substrate that turns have re-ranked | Not run. **A17 is unknown and this must fire first** |
| **F-8** | **The transition's real shape.** Instrument fills per turn and their wall clock over a cold working area | **G3's value, and §7.4.2's estimate.** If a first run fires far more than ~3 fills, or a fill costs far more than ~31 s, the ceiling is set wrong and the latency claim is wrong with it | Not run — the numbers in §7.4.2 are derived from #12984, not measured on this path |

**Two run first, and in this order.**

**F-7 is the hard gate**, because it is the only one that can invalidate the *mechanism* rather than a
value. If substance participates in ranking, the fill changes what retrieval returns, not merely what
assembly renders, and §7.4.4's structural repair does not reach it — the sweep would be reading a substrate
that live turns have silently re-ranked. It is one node, one query, one write, one re-query.

**F-6 is the cheapest and it decides the most contested section.** One node, one condensation, one reader,
and it settles whether §9 is right. It cannot run until the `SelfProduced` gate is removed, which is one
line — and under the fill that same line is what decides whether the memory core can serve run records at
all.

---

## 12. Risks & Mitigations

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| R1 | **Substance ships before fidelity is clean** and the model is confidently told something a lossy pass mangled | F-1 gates **Unit 2 as well as Unit 3** — the fill *writes* to the shared graph, so an unqualified model contaminates the substrate whether or not anything renders it yet. Unit 1 alone is safe: it renders nothing new and writes nothing | Any block containing a substance-form candidate, **or any fill at all**, before F-1 passes |
| R2 | **Unit 2 is treated as a one-off migration.** Coverage decays as the graph grows | §10 names it a recurring pass; Q5 asks for the coverage metric | Coverage measured once and never again |
| R3 | **A stale substance renders in place of correct content** | A6: the server clears substance on a content write, verified live. Fallback is content, never a wrong render | A substance surviving a content edit |
| R4 | **`contentHash` is re-based onto rendered bytes**, marking every substance-rendered required node stale | §7.2, adopted from #12955 §6.4 verbatim, with the reason | A sweep reporting corpus-wide staleness after Unit 3 |
| R5 | **The form rule acquires a provenance clause** — "except for nodes we wrote" | S2; §8.3. The rule is a pure function of size, ratio and presence | Any branch in the form rule reading `SelfProduced` or a node type |
| R6 | **A peer session log is excluded** by a rule aimed at run records | Nothing here keys on `session-log`; #11387 is the named live occupant (§4.7) | Any predicate reaching the `session-log` type |
| R7 | **`block` is dropped from the record without #10904 being re-ruled**, or dropped in a way that makes a 15–25 KB transcript *admissible in content form* for the first time | §9.3 item 3 states the constraint; §9.2's form rule is the guard that must exist first | A record shrinking below the budget while §9.2 is unimplemented |
| R8 | **Unit 4 is started before Unit 2** and its falsifier cannot run | F-5's dependency is stated; #13106 §8.3 says the same | A catalogue arm running against rows with null substance |
| R9 | **The sweep acquires the ability to fill** — a model adapter enters `cmd/eval`'s closure for some unrelated reason, and every A/B silently starts measuring a substrate it is mutating | A13 is the guarantee, and it should be pinned by a test asserting the closure rather than left as a convention | `go list -deps ./cmd/eval` naming any model adapter or `internal/condense` |
| R10 | **Substance is lost and stays lost**, because the sync that republishes a body has no step that re-derives it — measured three times over (§7.4.5) | **The fill is the mitigation, and it is the reason not to add a procedural one.** Under it the loss repairs on next use, whatever caused it | Substance still `null` on a re-synced design document *after* the fill ships and that node has been retrieved |
| R11 | **The steady-state argument is quoted as the cost** and someone plans against a free fill on a cold graph | §7.4.2 states the transition separately and gives its shape; F-8 measures it | Any plan citing *"basically free"* without naming the cold-start regime |
| R12 | **A fill fires below the size gate**, spending a model call and a fidelity risk to save ~11 % | G1, and #12984's 0.886 median below 4 KB is the number | A fill recorded against a candidate under 8 KB |
| R13 | **A bad substance is invisible and durable.** It is present, so the fill sees nothing to repair; it is consumed as fact by runs that never touch its node; and it survives exactly on the static nodes the fill is built to serve (§7.4.5) | **The requirement, not machinery.** §7.4.7 defines the capability the fill's model must have, and **F-1 qualifies that model before it is allowed to write.** The durability asymmetry is why the bar sits above the turn's | An unqualified model runs a fill — i.e. F-1 not re-run after the fill model or prompt changed |
| R14 | **The fill silently inherits the turn's model**, because both call `boot.LoadModel()` today (A18) and one process makes that invisible | §7.4.7: separate configuration, **no fallback**, capability absent when unconfigured | A fill recorded with the same model id the turn used, absent explicit operator intent |
| R15 | **A model is swapped for latency** and *fact* quietly changes meaning | §7.4.7 records the floor **as a class with its reason**, and F-1 re-runs on model change | A fill model changed with no F-1 re-run cited |

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
| **Q1** | ~~What is the "retrievable corpus"?~~ **ANSWERED — the question dissolves.** Under §7.3 the retrievable corpus is *whatever retrieval retrieves*, discovered rather than declared. There is no set to define, no campaign to schedule, and a node nobody recalls is never condensed | **No longer blocking** | Struck rather than deleted: it was the open question that made Unit 2 a scheduling problem, and the reframe is what closed it |
| **Q2** | **The ratio threshold** in §8.1 | No — it ships behind a dial | Set it by F-3's curve, not by argument |
| **Q3** | **Does the condenser get a JSON path, or does the record get a prose projection?** (§9.3 item 2) | **Yes, for F-6** | The prose projection — it is reusable and does not couple the condenser to a schema |
| **Q4** | **Does `block` leave the stored record?** Requires #10904 to re-rule PR #8's stored-vs-response invariant | Only for §9.3 item 3 | Yes, but only after §9.2's form rule exists — otherwise a 15–25 KB transcript becomes admissible for the first time (R7) |
| **Q5** | **Who measures substance coverage, and how often?** Nothing does today | No | A line in the sweep report; it is one query |
| **Q6** | **`fields=substance` works on the listing route and is undocumented in #8** | No | One line in #8 by whoever touches it next. Named because A4 rests on it |
| **Q7** | **Two-phase retrieval** — rank without bodies, then batch-fetch the survivors. Measured 1,236,611 B → 115,382 B on the yardstick (§7.1) | No | Its own unit, any time. It composes with every payload rule and depends on none of them |
| **Q8** | **G3's value, and its retirement condition** (§7.4.3). Recommended 2, on a three-fill estimate that is derived rather than measured | No — but it ships with the fill | Set it from **F-8**, and write the retirement condition into the same commit. It is a transition instrument, not a constant |
| **Q10** | ~~How is a substance attributed to the model that produced it?~~ **WITHDRAWN — no provenance machinery.** It was raised as a hedge against choosing the model badly; the answer is to **define the requirement and qualify against it** (§7.4.7), not to instrument for having failed to | No | Struck rather than deleted, because a later reader will reach for provenance the same way. A19/A20 record what was checked. What remains is free: the run record names the model behind each fill, as it already does for the answer |
| **Q9** | ~~Should DiVoid invalidate substance on a content-hash change rather than on a content write?~~ **WITHDRAWN — no ask against DiVoid.** Every invalidation observed on real work was correct, and the byte-identical case it would defend is one no caller should perform (§7.4.5) | No | Struck rather than deleted, because a later reader will have the same idea. The answer is that the defect was ours, not DiVoid's, and the fill repairs it without a rule |

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

### Unit 2 — the fill *(a behaviour, not a campaign; gated on F-7)*

1. **Run F-7 before writing any of this.** If substance participates in ranking, stop and re-design — the
   determinism repair in §7.4.4 does not cover that case (A17).
2. **Declare the fill port in `internal/loop`.** It cannot live anywhere else: importing `internal/condense`
   from `internal/loop` is a compile cycle (A14). Follow `FilePort` exactly (A15) — nil is a legitimate
   construction, and it refuses every call **with a recorded reason**, never silently.
3. Implement it outside the loop, over `internal/condense`'s existing `ModelPort`/`GraphPort` seams. The
   condensation logic is not rewritten; it gains a second caller.
4. Wire it in `cmd/processor` **only**. `cmd/eval` must keep a closure with no model adapter — that is not a
   convention to remember, it is the guarantee (A13), and a test asserting the closure is cheap.
5. Implement the three gates (§7.4.3) as a single decision with a recorded outcome per candidate: filled,
   or the reason it was not. **G3 ships with its retirement condition written down** (Q8).
6. **Fills get their own counter and their own ceiling**, never `MaxModelCalls`.
7. **Add a fifth boot loader for the condensation model** — its own `PROCESSOR_CONDENSE_MODEL_*` members,
   read at the one environment site like everything else. **No fallback to `PROCESSOR_MODEL_*`**: absent
   config means the fill port is nil and every call is refused with a recorded reason (§7.4.7). The cheap
   guard is a test that the fill is **off** when only the turn's model is configured.
8. **Do not run the fill with a model that has not passed F-1** (§7.4.7). The floor is a class — a capable
   semantic model, gemma-class — not a pin, and the audit is what converts the class into a decision.
9. **Record the producing model on every fill** in the run record (§7.4.7) — as the record already names the
   model behind the answer. This is a record accounting for its own composition (#10904), **not provenance
   machinery**; none is proposed and none should be added.
10. **Rule the oversized case.** #12984 refused #10926 at 195,448 B rather than store a truncated
   condensation, and that refusal is correct. Under the fill this now happens *inside a turn*, so the
   refusal must be fast and recorded rather than merely correct. #11373 §5c hands the policy to this binary.
11. Publish **F-4** (convergence) and **F-8** (transition shape). §7.4.2's numbers are derived from #12984
   and have never been measured on this path.
12. **Do not present the steady state as the cost.** The graph is 100 % cold (§7.4.2); the first runs pay
   near the worst case and whoever runs them should be told so.

### Unit 3 — the form rule *(gated on F-1: a qualified generating model)*

1. **Do not start until F-1 passes at zero tolerance.** The prompt fix is Kim's (#11373); §4.5 is the brief.
2. Implement §8.1 as a pure function of size, ratio and presence. **No provenance branch** (R5).
3. Threshold behind a sweep dial; set it from F-3.
4. `size` and `contentHash` keep their meaning (§7.2, R4). Add form and rendered size.
5. Mark every substance-rendered candidate with one header line. It names the form; it does not editorialise
   (#12955 §6.3 — how the system text should treat a marked block is Kim's Q4, not this design's).
6. Run **F-2** and publish it. The withdrawn invariant made regression impossible; this measurement is what
   replaces the proof.

### Unit 3a — run records become condensable *(smallest, and it decides §9 — pull it forward)*

**Under the fill this stops being a side-branch.** `internal/condense/condense.go:283`'s `SelfProduced` skip
and `:296`'s `application/json` refusal now sit *inside the memory core's own fill path*, which means they
are the two rules making the core **structurally unable** to condense the class Toni most wants condensed —
a prior run's own account of itself. **Run this before committing to Unit 2's shape**, not after.

1. Remove the `SelfProduced` gate at `internal/condense/condense.go:283`. One line.
2. Resolve **Q3**, then **run F-6** on a single record. **This is the cheapest experiment in the document
   and it decides whether §9 is right.** If a run record's substance turns out to be vacuous, exclusion
   stays a provenance rule and §9 is wrong — say so, and correct this document in place.
3. Only after F-6: state §9.2's form rule as a guard, and consider **Q4**.
4. Whatever F-6 returns, the fill's **refusal contract** (§7.4.4) must surface a condenser skip as a stated
   reason. A gate that silently declines is the failure mode this unit exists to expose.

### Unit 4 — the catalogue

**#13106 §8.3, unchanged.** Do not start before the fill has warmed the twenty rows F-5 needs, and
note that #13106 already establishes it probably requires the call ceiling raised.

### What must not happen

- No substance rendered into a block before **F-1** passes.
- No provenance clause in the form rule (S2, R5).
- No predicate keying on the `session-log` type (R6) — #11387 is its live occupant.
- No `block` removal before §9.2's form rule exists and **#10904** has re-ruled (R7).
- **No fill before F-7 fires.** A17 is unknown and it is the one gate that can invalidate the mechanism.
- **No fill port constructed in `cmd/eval`**, and no model adapter admitted to its dependency closure — that
  closure *is* the determinism guarantee (A13, §7.4.4).
- **No fill counted against `MaxModelCalls`** (§7.4.3).
- **No silent fill refusal.** Every skipped fill carries its reason, or §9.3's two gates become invisible.
- No claim that the steady state is free without saying we are entirely inside the transition (§7.4.2).
- **No fill from a model that has not passed F-1**, and **no fallback to the turn's model** (§7.4.7).
- **No substance written without the run record naming the model that produced it** (§7.4.7).
