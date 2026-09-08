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
| **And the condenser cannot serve the class that needs it most** | `condense.go:283` skips `SelfProduced`; `:296`'s `isProse` refuses `application/json`. **But the gates turned out not to be the constraint** — #13242 removed both and the pass still refused every record at **84 KB**, then fabricated when forced through. The obstacle is shape, and the remedy is §9.3.2's template, not a gate deletion. |

**The recommendation, in four lines.**

| | |
|---|---|
| **Unit 1 — See it** | Request `substance` in recall; carry it on `Candidate`; record which form was rendered. **No admission-policy change.** Small, and nothing downstream is measurable without it. |
| **Unit 2 — Generate it, on demand** | **A behaviour of the memory core, not a backfill campaign.** Retrieval finds a node it wants to push, finds no substance, and makes one. **Bounded by the cache — cost is proportional to novelty, not traffic** — plus a size gate, a pressure gate, and a transition-only ceiling. §7.4. **Prose nodes only: run records take the template, not the model (§9.3.1).** |
| **Unit 3 — Render it, size-gated** | Substance replaces content **where condensation actually compacts** — the ≥8 KB stratum, measured median ratio **0.298**. Below 4 KB the measured ratio is **0.886**: substance saves ~11 % and spends fidelity, so content stays. |
| **Unit 4 — The catalogue** | #13106 §8.3, unchanged and still third. Its payload *is* substance, so it is downstream of Unit 2 by construction, and its three-arm falsifier cannot run until the fill has warmed the twenty rows. |

**`block` leaves the record — for three reasons plus a fourth that is a retrieval improvement, and explicitly not for the one everybody reaches for.** `Record.Block` is
**73.6 %** of an 84 KB record and holds other nodes' bodies verbatim, for content the graph already keeps
behind an edge. Toni: *"duplicating content in a graph is complete nonsense — that's what edges are for."*
The principle stands and Q4 answers **yes, drop it** — **on storage grounds, not retrieval grounds.** The
discriminating test: a literal taken from **inside** `block` returns **0 run records in the top 30** (the
node that owns the text, #6375, is top at 0.7855), while the yardstick input — which the record's **name**
carries verbatim — returns **13, holding ranks 1–11, 13 and 14**. **The crowding is name-driven, not
content-driven; removing `block` will not fix it, because the name still matches** (§4.8, §9.5). That is a
correction to every prior account in this project, this document included, and **the name is a retrieval
surface nothing here treats as one** — named as Q11 and left as its own unit.

**But content is still a retrieval defect — a different one, and an earlier revision of this document missed
it by generalising the result above.** Two more queries (#13274, re-run here): a **generic activity**
description returns **13 records holding ranks 1–13 contiguously** inside a 1.8 % band — findable as *"a
run"*, mutually indistinguishable — while a description of **one run's most distinctive outcome**, uniquely
true of #13034, returns it **not at all**. A record's embedding is 73.6 % other nodes' bodies, so what makes
it *this* run contributes almost nothing. **Crowding is *the wrong rows appear*; skew is *the right row
cannot be recognised*. `block` causes the second, not the first**, which makes its removal a **retrieval
improvement** and not the cleanup an earlier revision called it (§4.8.1, §9.5).

**And the invariant that blocked it is re-ruled here (§9.6), not deferred.** #10904 required the stored
record to carry what the response carried. **Amended, not abolished:** the response keeps `block`, the
stored body replaces it with `blockBytes`, and the invariant becomes a named list of **two** divergences
whose test still reddens on a third. The census ran before the proposal, not after: **`internal/eval` and
`cmd/eval` never fetch a record body at all**, and the only stored-record reader in the repo
(`scripts/compare.py:279`) reads **`input` alone**. Everything that reads `block` reads it from the
**response**, which does not change. And storing block *ordering* — the one thing #13245 called
unrecoverable — is **not needed**: `assemble.go:25` sorts admitted candidates ascending by id before
rendering, so block order is the anchor followed by every `Included` disposition sorted by id, derivable
from fields already stored.

**Both gates have now run, and the negative one is the useful one.**

**F-7 passed (#13241).** A probe carrying content about arctic terns and a substance about quantum error
correction returns at **rank 1 / 0.7801** for its content's topic and **does not return at all** for its
substance's — on `divoid_search` and on the exact route `Recall` builds. **Substance is stored and served
but not embedded**, so a fill changes what assembly renders and never what retrieval returns; §7.4.4's
structural argument holds, and **coverage changes the block, never the candidate set**. It becomes a
standing guard rather than a settled fact: nothing would announce it if DiVoid ever started embedding.

**F-6 came back negative and corrects this document's own sequencing (#13242).** With both gates removed,
`cmd/condense` refuses every run record at **84 KB** while reporting `operational failures 0`; with thinking
disabled it produces a digest of *other nodes*, **fabricates a count** (ten where the record says twenty),
and invents figures the prompt forbids rounding. **Dropping `block` was tested and does not fix it** — the
remainder is a **data table**, and condensing a data table is transcription. An earlier revision billed
*"remove the `SelfProduced` gate — one line"* as the cheap first experiment; **it is not, because a one-line
change that converts a refusal into a plausible-looking wrong fact on a permanent node is not cheap.**
**Shape first, gates last.**

**So the fill splits by node class, and this is the round's main change.** Prose nodes keep the model fill —
that is what #12984 measured at 23 PASS / 1 FAIL. **Run records get deterministic rendering: a template over
the record's own fields, no model at all** (§9.3.2). A template cannot fabricate a count, needs no F-1
qualification, costs nothing, and closes Q3 and Q4 outright.

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
| A17 | **MEASURED: `substance` does not participate in similarity ranking.** It is stored and served but **not embedded**, so writing one changes what assembly renders and never what retrieval returns | **#13241**, 2026-09-08. A probe carrying content about arctic terns and substance about quantum error correction: the content topic returns it at **rank 1, sim 0.7801** (next hit 0.5558); the substance topic does not return it at all, whole field at the ~0.57 noise floor. Run on `divoid_search` **and** on `GET /api/nodes?query=`, the exact route `Recall` builds — both agree. **Standing guard, not settled: re-run when the fill ships** (F-7) |
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

### 4.4 The condenser refuses run records twice — and the gates are not why it cannot serve them

| gate | site | effect on a run record |
|---|---|---|
| `SelfProduced` | `internal/condense/condense.go:283` | skipped before any model call |
| `isProse` — `text/*` or empty | `:296`, `:298` | `application/json` refused |
| **size** — measured, not designed | #12984 skips | #10926 at 195,448 B hit the output ceiling, returned `finish_reason: length`, and was **refused rather than stored truncated** |

**So the class of node with the worst shape in the graph is the one class the condenser structurally cannot
improve.** That is the concrete form of Toni's *"something sounds fishy here"*: we built a compressor,
declined to run it on our own output, and then excluded our own output for being uncompressed.

> **Corrected 2026-09-08 by F-6 (#13242), and the correction matters more than the original point.** An
> earlier revision called the size row a *near-miss* — *"run records are 77–88 KB, in the same direction"*.
> **It is not a near-miss and 195 KB is not an outlier: the pass refuses run records at 84 KB, across the
> whole class**, and it does so while reporting `operational failures 0`. More importantly, the framing
> above — *the gates are what stop us* — is **wrong**. Removing both gates changes the outcome from
> *refused* to *fabricated* (§9.3). The gates are a boundary between two node classes, not an obstacle;
> §9.3.1 keeps run records off the model path deliberately.

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

### 4.8 Crowding and skew are different failures, and `block` causes only the second

**Measured 2026-09-08 on `GET /api/nodes?query=`, the route `Recall` builds.** Two queries, `count=30`,
chosen to separate the two candidate explanations for why run records hold the top of the list.

| query | where that text lives in a run record | run records in top 30 |
|---|---|---|
| a literal from **inside `block`** — *"High-Fidelity interaktives Wireframe für Profilgenerator Phase 1"*, which belongs to **#6375** | duplicated verbatim in the record's **content** | **0.** Top hit is **#6375 itself at 0.7855** |
| the yardstick input — *"Generate a new barebones webpage and a repo for it."* | in the record's **name**, and also in its content | **13**, holding **ranks 1–11, 13 and 14**, 0.675–0.7359 |

**A run record is named `processor-run <timestamp> — <the input, truncated to 80 runes>`**
(`internal/divoid/write.go:101`). So a repeat of an input is a near-exact lexical match against the *name*,
while 60 KB of duplicated bodies inside the record contribute **nothing measurable** — the node that owns
that text outranks the record that copied it, and the record does not appear at all.

**Why that is consistent with F-7 rather than in tension with it.** Content *is* embedded (#13241 measured
that on a probe). But a ~60 KB record spanning eight unrelated topics has a **diffuse** embedding: no single
topic inside it outranks the node actually about that topic. **Dilution, not exclusion.**

> **Removing `block` would not fix the crowding — the name would still match.**

**This corrects an attribution every prior account in this project made**, including earlier revisions of
this document: the top-of-list records were credited to similarity on their *content*, or to the exclusion
policy. **Neither is the mechanism.** Toni's own hypothesis on reading a record — that content duplication is
*"probably the strongest reason for your 'poisoning' earlier"* — was the discriminating test's target and
**did not hold**.

#### 4.8.1 But content is still a retrieval defect — a different one

**An earlier revision of this document generalised the result above into *content is not a retrieval
problem*. That does not follow, and #13274 measured why.** Toni named the gap:

> *"doesn't matter whether the embedding actually hits on the topic, it definitely does not hit where it
> should because the content drives the actual 'thing' the session log represents away."*

**Two more queries, same route, `count=20`, re-run independently for this document on 2026-09-08:**

| query | run records in top 20 |
|---|---|
| **generic activity** — *"a harness run that wrote index.html, styles.css, README.md and .gitignore into a workspace and answered that it created a barebones webpage"* | **13, holding ranks 1–13 contiguously**, 0.7256–0.7435 — a **0.0179** spread |
| **one run's distinctive outcome** — *"a run where the terminal reason said answered but the answer came back empty after three model calls"*, uniquely true of **#13034** | **0. #13034 does not return at all** |

**Neither result is the one you would want.** The first says a run record is findable as *"a run"* — thirteen
of them, occupying the entire head of the result set inside a 1.8 % similarity band, mutually
indistinguishable. The second says a record is **not** findable as *"the run where X happened"*, even when X
is described exactly and is true of precisely one record.

**The cause is the same 73.6 %.** A record's embedding is mostly other nodes' bodies, so the fields that make
it *this* run — the tool sequence, the stop reason, the admitted/cut split, an empty answer against a
terminal reason of `answered` — contribute almost nothing. **The dump does not merely fail to help; it
displaces the record's own identity.**

> **Crowding and skew are different failures. Crowding is *the wrong rows appear*; skew is *the right row
> cannot be recognised*.** §4.8 measures the first and is silent on the second. For a system whose substrate
> is memory, skew is the more serious of the two — and it is the one `block` causes.

**Both measurements stand; only the inference from the first was wrong.** This is the second time in two
rounds that a correct measurement was generalised one step too far, and the shape is the same both times:
a discriminating test rules out *one* mechanism and gets read as ruling out a *class*.

**Reproduction note, because the numbers differ from #13274's.** Re-running its exact query strings gives
**ranks 1–13 contiguous with a 0.0179 spread**, where #13274 reports *"13, holding ranks 1–4"* at a 0.0035
spread. Thirteen rows cannot occupy four ranks, so that cell is internally inconsistent; the figures above
are the ones measured for this document and are the stronger claim on the rank half. **The skew half — zero
returns for the distinctive-outcome query, #13034 absent — reproduced exactly**, and it is the load-bearing
half.

**The name is therefore a retrieval surface, and nothing in this design or in #12955, #13106 or #13203
treats it as one.** Named here as a mechanism with its numbers; **deliberately not addressed** — it is its
own question and its own unit (Q11).

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

**The premise is now measured rather than assumed (A17, #13241).** Substance is stored and served but
**not embedded**, so a fill cannot move a rank. That closes the one hole this argument had: retrieval does
not become path-dependent on what an earlier run happened to condense.

**A corollary worth stating on its own, because it separates two effects that would otherwise be
confounded: coverage changes the block, never the candidate set.** An admission change observed after a
fill has some other cause and must not be attributed to the fill.

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

#### 7.4.8 Four defects in `cmd/condense` the fill inherits

**Found by F-6 while probing the run-record class (#13242 §5), but none is specific to it.** The fill calls
the same pipeline, so it inherits all four. They are recorded here rather than left in the probe because
three of them are the difference between *the fill works* and *the fill silently produces nothing*.

| # | Defect | Consequence for the fill |
|---|---|---|
| **D1** | **The pass reports success while producing nothing.** `condensed 0 · skipped N · operational failures 0`, ratio 0.000 | The fourth instance of *instrument reports clean while measuring nothing* in this project. Under the fill it is worse than an offline report: a refusal is per-turn and invisible unless §7.4.4's refusal contract names it, which is now load-bearing rather than tidy |
| **D2** | **The pass cannot use a non-thinking adapter.** `cmd/condense/main.go:71` constructs `openaicompat.NewClient` **unconditionally**; it loads and validates `PROCESSOR_MODEL_PROTOCOL` through `boot.LoadModel()` and then **discards it**, while `cmd/processor/main.go:89-90` branches on the same field | OpenAI-compat carries no `think` parameter, so the pass cannot suppress a reasoning stream. **The single change that turned *produces nothing* into *produces something* is unreachable from the shipped binary.** The fill must reach the ollama adapter, which means honouring the protocol field the loader already parses |
| **D3** | **`maxOutputTokens` is sized as if reasoning tokens do not exist.** The budget is 0.5 × input tokens; a thinking model charges reasoning against the same ceiling | **Every thinking model trips `skipTruncated` on large inputs regardless of capability.** Measured: ~33,000 characters of reasoning consumed the entire 9,831–9,956-token budget without emitting one character of content. This disqualifies models by accident rather than by the bar §7.4.7 defines |
| **D4** | **`outputTokenFraction = 0.5` inverts on dense structured input.** It granted 2,259–2,459 tokens for input that cannot compress below roughly 1:1 | The formula assumes prose. It is right for the class §9.3.1 keeps on the model path and wrong for anything tabular — which is one more reason run records leave that path entirely |

**D2 and D3 are prerequisites, not follow-ups.** Until the pass can select a non-thinking adapter and size
its budget around reasoning tokens, a fill against a capable model is indistinguishable from a fill against
an incapable one: both return nothing and both report success. **Fix them before F-1 is re-run**, or F-1
measures the harness rather than the model.

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
| condensable by the model pass | **yes** — and none has been condensed (§4.1) | **no, and not for the reason it looked like.** Gates aside, the pass refuses at 84 KB and fabricates when forced (§9.3). Run records take the template instead (§9.3.2) |

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
*byte* problem dissolves. The *slot* problem is separate, and §4.8 has since located it precisely: the rows
holding ranks 1–11 are there because the input is in their **name**, not because 60 KB of copied bodies sits
in their content. **So the slot problem is real, is untouched by anything in §9.5, and is untouched by
substance either** — A17 says a substance cannot move a rank. What a substance changes is what the slot
*costs*: an accurate 1–2 KB summary instead of an excluded 84 KB transcript. **Whether that is worth a slot
is testable and is the honest live question**, and it is not the one #13237 answered.

### 9.3 What it takes to give a run record a substance — measured, and the ordering was backwards

**F-6 ran and came back negative (#13242). It does not sink the fill; it corrects this section.** An
earlier revision billed *"remove the `SelfProduced` gate — one line"* as the cheap first experiment and
told the next agent to pull it forward. **That was wrong, and the correction is the finding:**

> **A one-line change that converts a refusal into a plausible-looking wrong fact on a permanent node is
> not a cheap experiment.**

**What was measured.** With **both** gates removed, `cmd/condense` condensed **nothing** — all three run
records refused as `condensation truncated`, ratio 0.000, **while reporting `operational failures 0`.**
#12984's 195 KB refusal is not a size outlier: it happens at **84 KB**, across the whole class. With
thinking disabled and the pipeline replicated byte for byte, the pass does produce output — and the output
is worse than nothing:

| what it produced | why |
|---|---|
| **A digest of *other nodes*.** The substances faithfully summarise #10422, #1804, #6375, #13101 — all already in the graph, in better form | `block` is 71–72 % of a record and holds other nodes' bodies (§4.6) |
| **Two of three never mention the run at all**; the third gives it ~8 % of its text | there is little else in the input to summarise |
| **A fabricated count stated as fact** — *"returns 10 results"* where the record says **20** | the model took the first ten, reordered them, and asserted a total |
| **Measured byte values replaced by invented integers** — `0/3092`, `1002/302`, `1471/0` rendered as `0/3`, `1/3`, `1/0`, violating the prompt's own *"never round"* | and a sibling condensation of the same source got them right: **two condensations disagree** |
| **Every decision-relevant fact absent** — the admitted/cut split, `stopReason`, `modelCalls`, `capReached`, `usage` | none of it is prose, so a prose compressor discards it |

**And dropping `block` does not fix it — that was tested, not assumed.** On the block-stripped projection
(19–21 KB) the model **transcribed** rather than condensed — JSON to YAML, verbatim — and hit `done=length`
at candidate rank 11–13 of 20, never reaching `answer`, `toolCalls`, `stopReason`, `modelCalls` or
`limits`: the only members that describe the run. The reason is structural, and it is the sentence to keep:

> **A run record's non-`block` half is a data table, not prose. Condensing a data table is transcription.**

`candidates` is 20 × (id, name, similarity, size, 64-char hash, cutReason, sources), and the prompt's own
fidelity clause — *identifiers, paths and hashes survive verbatim* — **forbids compressing it**. Dropping
`block` is necessary and nowhere near sufficient.

#### 9.3.1 The fill splits by node class, because F-6 tested only the hardest one

**This is the architectural consequence and it should not be read as a setback.** #12984 condensed **prose
documents** at 23 PASS / 1 FAIL. F-6 condensed **run records** and got nothing usable. Those are different
inputs to the same pipeline, and the pipeline is right for one of them.

| class | mechanism | basis |
|---|---|---|
| **Prose nodes** — `documentation`, `session-log`, `task`, the ≥ 8 KB stratum §4.2 measures | **The model fill, as designed in §7.4.** Unchanged | #12984: 23 PASS / 1 FAIL, median ratio 0.298 at ≥ 8 KB |
| **Run records** | **Deterministic rendering — a template over the record's own fields, no model at all.** §9.3.2 | #13242: the model path produces a digest of other nodes, fabricates counts, and truncates before reaching the fields that matter |

**The gates are not the binding constraint and must be re-sequenced accordingly. Shape first, gates last.**
Removing `SelfProduced` and `isProse` changes the outcome from *refused* to *wrong*, and *refused* is the
safer of the two while nothing better exists. **They come out when there is a mechanism that produces
something worth writing — not before.**

#### 9.3.2 Deterministic rendering, and why it is better than the model path for this class

**Recommended.** Almost everything that makes a run worth remembering is **already a field**: the input,
the subject, the derived queries, how many candidates were admitted and cut and under which reasons, the
tool sequence, the answer, the terminal reason, the model-call count, whether the cap was hit. Rendering
those through a fixed template produces one or two kilobytes of accurate prose.

| | Deterministic rendering | The model path |
|---|---|---|
| **Can it fabricate a count?** | **No — structurally impossible.** A template reads `len(candidates)`; it cannot assert ten where there are twenty | It did, on the first record tried (#13242 §3) |
| **Determinism** | Total. No sampling, no model, no A/B exposure whatsoever | Two condensations of one source disagreed |
| **Cost** | **Zero model calls.** No fill latency, no ceiling, no cold-start regime for this class | ~31 s per node, and every thinking model trips the truncation guard (§7.4.8) |
| **F-1 dependency** | **None** — no model, no qualification needed | Blocked; see F-1's stated blocker |
| **What it cannot do** | Summarise the answer's *prose*, or say anything the record does not contain. Include the answer verbatim, truncated with a marker | Could in principle — and on this class demonstrably does not |

**It also resolves what §9.3 previously called an implementation choice.** The prose projection the
condenser was supposed to read does not need to exist for a condenser to read: **the template is the
projection, and its output is written directly as the record's `substance`.** No new node type, no second
artifact, and **no dependency on Q4 in either direction**: a template reads fields and ignores `block`, so
it neither requires §9.5's removal nor is blocked by it. #10904's re-ruling gates that removal, not this.

**One consequence that A17 forces into the open, and it sharpens the question this document opened with.**
Substance is **not embedded** (#13241), so giving a run record a substance **cannot change how it ranks**.
A record will still rank near the top on a repeat of its own input, because its *content* still contains
that input verbatim. What changes is only what a slot costs: **an accurate ~1–2 KB summary of the prior
run instead of an excluded 84 KB transcript.** So the live question is no longer *should we exclude our own
records* but **is an accurate summary of the last attempt worth one candidate slot** — which is a question
about value, is testable, and is a far better question than the one PR #48 was arguing about.

### 9.4 The prize, stated so it is not lost

Toni: *"in the end substance of a session log is the best context you can get for your current work — a
history compressed to substantial facts."*

**On this project's own evidence that is very likely true, and the strongest single example is one we
measured.** #13091 §5(a) found that a *single* run record entering the candidate list changed the assembled
block by one row and thereby changed the outcome from *"no webpage"* to *"a webpage"* — while the record
itself was cut and never reached the model. It influenced the run by displacement alone. **A record whose
substance said what that run actually did would be the most directly relevant node in the graph for the
next attempt at the same task.**

**The prize stands; the vehicle changed.** An earlier revision ended this section *"and today it is refused
twice by the one component built to produce it"*, which read as though the two gates were all that stood in
the way. **F-6 measured otherwise (#13242):** removing them yields a digest of other nodes with a fabricated
count, so the condenser was never the component that could produce this. **§9.3.2's template is**, and it
produces the summary above from fields the record already carries, without a model and without the ability
to invent one.

### 9.5 `block` should leave the record — storage, renderability, **and retrieval**, but not crowding

> *"The session log itself dumps the full content of the nodes into it. There is absolutely no reason for
> that — it's redundant to record content again. It's a bad memory strategy… duplicating content in a graph
> in general is complete nonsense — that's what edges are for. I thought about it and found these nodes for
> that reason — am I interested in content? Okay, let's traverse the graph."* — Toni, 2026-09-08

**The principle is right and needs no defence from this document.** A graph whose nodes copy each other's
bodies has stopped being a graph; the edge already answers *where is that content*, and the copy can only
go stale against the node it copied. `Record.Block` is **73.6 % of an 84 KB record** and holds other nodes'
bodies **verbatim** — header, id, type, name and full body, for content the record already has an edge to.

**Four independent reasons to drop it, and one that is not on the list. The fourth and the struck one are
adjacent and must not be blurred: one is a retrieval claim that is true, the other is a retrieval claim
that is false.**

| | Reason | Standing |
|---|---|---|
| 1 | **Redundant storage.** ~60 KB of verbatim duplicate per record, of content the graph already holds behind an edge | Toni's principle, above |
| 2 | **It is what makes a record unrenderable.** A prose compressor handed 73.6 % other-nodes' bodies produces a digest of those nodes — exactly what #13242 measured | §9.3 |
| 3 | **It is what makes a record unadmittable.** 77–88 KB against a 60,000-byte budget, so the exclusion in PR #48 exists to stop something that could never have fitted | §4.6 |
| **4** | **It displaces the record's identity in the record's own embedding — so removal is a *retrieval improvement*.** A record is findable as *"a run"* (13 of them across ranks 1–13, a 1.8 % band) and **not** as *"the run where X happened"*: a query describing exactly one record's most distinctive outcome returns it **not at all** | **§4.8.1**, #13274 |
| — | ~~**It causes the crowding.**~~ | **NOT A REASON. Measured false** — §4.8: a literal from inside `block` returns **zero** run records; the input, which the *name* carries, returns thirteen |

**Reason 4 and the struck reason are both about retrieval and they are not the same claim.** Reason 4 says
the dump keeps the **right row out**; the struck one says it pulls the **wrong rows in**. The first is
measured true, the second measured false, and the distinction is exactly the crowding/skew split in §4.8.1.
**Anyone reading this table quickly will collapse them — do not let a summary of this change do that.**

**So this is not cleanup and must not be scheduled as cleanup.** An earlier revision of this section called
it storage hygiene, which undersells it: removal is what makes a run record retrievable **as itself**, and
that is the whole point of a memory substrate holding one.

**And the struck row stays struck.** *State it plainly wherever this change is described: dropping `block`
does not fix the crowding, the name still matches (Q11), and anyone citing this as the crowding fix has
stopped looking one step too early.*

**Falsifier for reason 4, and it cannot be run yet.** #13274 measures the **defect**, not the **fix**:
whether a record written *without* `block` becomes findable by its outcome needs a record written after the
change, or a constructed one. **Until that runs, reason 4 is a well-supported inference and not a measured
outcome** — the test is to write one such record and re-run the distinctive-outcome query against it.

**One live interaction worth stating, because the two facts sit on the same node and pull opposite ways.**
PR #51 (`004afa8`, in flight as of 2026-09-08) gives every new record a **deterministic summary of its own
run**. That unit is unaffected by any of this — it reads fields and never touches `block`. But from it
onward a record carries **an accurate substance while its content still holds the 73.6 % dump**, and
**retrieval sees the content**, because substance is not embedded (A17, #13241). So the summary improves
what a record *renders* and changes nothing about what it *is findable as*. Reason 4 is what closes that
gap; the summary does not.

**What this does not change.** The template (§9.3.2) reads fields and ignores `block`, so it works either
way and does not depend on this.

**And the invariant that stood in the way is re-ruled in §9.6 rather than left as a dependency.** #10904's
*stored body is the response body minus exactly one key* is **amended, not abolished**: the response keeps
`block`, the stored body carries `blockBytes` instead, and the guard test is renamed and extended so a
*third* divergence still reddens it. The operator instruments read `block` from the **response**
(`scripts/smoke.py:241-244`, `scripts/step_trace.py:700`), so nothing loses access; what was decided is
whether the two bodies may differ by more than the write receipt, and §9.6.5 says how far.

---

### 9.6 Re-ruling #10904's stored-vs-response invariant

**This section re-rules an invariant settled in another design document.** #10904 §8.1, decided on PR #8,
states it, and a test at `cmd/processor/artifacts_test.go:161` enforces it:

> *"The stored node's body and the HTTP response body carry the same record, byte-for-byte, in every key
> that describes the run. The response body carries exactly one key more: the write receipt. The stored body
> is the response body minus that one key, and nothing else differs."*

**Ruling: amend it. `block` leaves the stored body and stays in the response; one integer replaces it. The
invariant is not abolished — it is narrowed to a named list of two divergences, and its test keeps its job.**

#### 9.6.1 What the invariant was for, and why this is not a violation of it

**Read #10904's own description of the guard, because it decides the shape of the answer:** the test is one
where *"a new field added to either side **without a decision** reddens it."* **The invariant is a
change-control tripwire, not a claim about what a reader needs.** It exists so the two bodies cannot drift
silently.

**A considered divergence therefore does not violate it — it is the thing the tripwire exists to summon.**
This section is that decision. What must keep being prevented is a *third* divergence appearing because
someone added a field and nobody noticed.

#### 9.6.2 What the stored record owes a reader that the response did not

PR #8 answered *"the same bytes"*, and **that was right when it was made, because the two readers were the
same reader.** The record was the only account of a run. Three things have changed, none of which existed
when the invariant was ruled:

| | Then | Now |
|---|---|---|
| the only account of a run | the record | **plus a deterministic ~2 KB summary on every record** — PR #51 (`004afa8`, on `main` at `2b07bed`): `WriteRun` calls `loop.RenderSummary` and stores it as the node's substance |
| what identifies an admitted node | the copied body | `candidates[]` carries **every** candidate's id, name, similarity, size, `contentHash`, cut reason and sources |
| where the body itself lives | the copy | the node, **behind an edge the record already has** |

**The two readers have separated, and they want opposite things:**

| | The **response** reader | The **stored** reader |
|---|---|---|
| who | an operator debugging *this* run, synchronously, workspace still on disk | a later run, or a human walking the graph |
| asks | *what exactly did the model see?* | *what happened, and what did it cost?* |
| wants `block` | **yes** — `scripts/smoke.py:241-244` prints it verbatim; `scripts/step_trace.py:700` reconstructs the turn from it | **no** — 60 KB of other nodes' bodies, each already behind an edge, and §4.8.1 measures that carrying them **destroys the record's own identity in its embedding** |

**That is the answer to the question posed.** The stored record owes a reader an **account of the run**; the
response owes a reader a **reproduction of the prompt**. PR #8 could not tell those apart because nothing
had yet forced them apart. **They are now different artifacts for different readers, and the invariant that
fused them has outlived the condition that made it true.**

#### 9.6.3 What breaks — established before proposing, not after

**Census run against `main` at `2b07bed` (post PR #51), re-confirmed against `b021fdc`.**

| reader | reads the **stored** record? | what it needs |
|---|---|---|
| **`internal/eval`, `cmd/eval`** | **No — it never fetches a record body at all.** Its single reference is `internal/eval/result.go:162`, `divoid.IsRunRecord(d.Type, d.Name)`, which **classifies a candidate row a sweep retrieved** | nothing |
| **`scripts/compare.py:279`** (`find_prior_run`) | **Yes — the only one** | **`record["input"]` alone.** It parses the body and compares one field |
| `scripts/smoke.py`, `scripts/step_trace.py` | No — both read the **response** of `POST /runs` | unaffected |
| `scripts/compare.py` elsewhere | No — the response again. Note `:497` is `len(record["block"])`: **a byte count, never the text** | see §9.6.4 |

> **Nothing that reads the stored record reads `block`. The one instrument that touches `block` at all takes
> its length.**

**The brief asked specifically what `internal/eval` needs. Measured answer: nothing from the stored record.**
It loads corpora, sweeps retrieval, and classifies rows by type and name. Removal cannot reach it.

**On reproducing a run's exact prompt — no identified reader requires it, and the stored block does not
reliably provide it anyway.** The block holds node bodies *as they were at run time*; the graph holds them
as they are now. A reproduction is exact only until any admitted node changes, which is precisely why
`contentHash` exists (#10904 §7: *"a record of ids alone rots as the nodes change"*). **The stored block is a
snapshot with no stated lifetime and no reader who has claimed it.** Where exact-prompt reproduction was
actually needed, it was done by capturing request bodies on the wire — #13091 archived 70 of them to a zip —
not by reading a graph node. **If someone does need it, say who and why and this reopens. Absent that it is
a capability nobody has asked for, at 60 KB per record.**

#### 9.6.4 The three options weighed — and one rests on a claim that is false

| | Option | Ruling |
|---|---|---|
| **a** | **Drop `block` entirely** | **Adopted**, together with (b)'s byte count |
| **b** | **Store a hash, or a byte count, only** | **Byte count adopted; hash rejected.** A hash serves verification of a reproduction nobody performs, and per-candidate `contentHash` already carries drift detection. The byte count is different: it is the one derived quantity an instrument actually uses (`compare.py:497`), and it is **not** exactly reconstructible — the block's framing (`===== ANCHOR =====`, `===== CANDIDATE =====`, the `id/type/name` headers) appears in no field. Reconstruction would be approximate; an integer is exact and costs eight bytes against sixty thousand |
| **c** | **Store the block ordering**, since `candidates[]` is in similarity order and #13101 makes position the largest lever | **Rejected — the premise is false, and I read the code rather than the claim.** `internal/loop/assemble.go:25` sorts admitted candidates **ascending by id** and hands that slice to `renderBlock`, which writes the anchor and then the slice in order. **So block order is exactly: the anchor, then every disposition with `Included == true`, ascending by id.** Fully derivable from fields already stored; no field needed |

> **Correction to #13245 finding 3.** It states *"block ordering is the honest cost of not rendering
> `block`… `candidates` is in similarity order, which is not necessarily block order — and it needs a field,
> not a longer render."* The observation is right and the conclusion does not follow: block order is not
> *candidates* order, but it is a **known deterministic transform** of it. **No field is needed.** This
> removes what would otherwise have been the strongest reason to keep part of the block.

#### 9.6.5 The amended invariant, stated so #10904 can adopt it verbatim

> **The stored node's body and the HTTP response body carry the same record in every key that describes the
> run, with exactly two named exceptions: the response carries the *write receipt*, which the stored body
> does not; and the response carries *`block`*, which the stored body replaces with `blockBytes`, its length
> in bytes. Nothing else differs.**

**The test survives and keeps its purpose.** `TestTheStoredBodyIsTheResponseBodyMinusTheWriteReceiptAndNothingElse`
(`cmd/processor/artifacts_test.go:161`) is renamed and extended to strip both named keys before comparing.
**A third divergence, added without a decision, still reddens it** — which is the whole value of the
invariant, preserved rather than spent.

**Consequences, stated plainly:**

- **Forward-only.** Existing records keep their `block`; nothing rewrites them.
- **It does not fix the crowding** (§4.8, §9.5's struck row). The name still matches; Q11 remains the lever.
- **It must not land before §9.2's form rule exists** — a record without `block` is 15–25 KB and would be
  admissible **in content form** for the first time (R7).
- **The exclusion must not be lifted before this lands** (R7a): while the dump is stored, the exclusion is
  the only thing preventing a record's content being copied into a later record's block.

#### 9.6.6 If the invariant should have stood

**It should not, and the case for keeping it deserves stating so the ruling is not mistaken for
inevitability.** The strongest version: *the record is the graph's only durable account of what a run was
given, and a system whose premise is "substrate is memory" should not store a summary of its own input while
discarding the input.*

**It fails on the measurement rather than on the argument.** §4.8.1 shows the stored block does not preserve
the run's account — it **destroys** it, displacing the record's identity in its own embedding so that a query
naming one run's unique outcome returns nothing. **A copy that makes the original unfindable is not an
archive.** And the premise cuts the other way: the bodies are in the graph, behind edges, which is what a
memory substrate is *for*. **The block is the one part of the record that does not trust the graph to hold
what the graph already holds.**

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
| **F-1** | **Model qualification.** #12984's audit, re-run at zero tolerance: every required node's substance must support its pre-registered `why`. **It is not a one-time release gate — it is the instrument that qualifies a *model*, and it re-runs whenever the fill model or the prompt changes** (§7.4.7) | **Unit 2 *and* Unit 3, and every model change thereafter.** A single FAIL disqualifies that model. **Note it now gates generation, not only rendering** — under the fill an unqualified model writes to the graph as a side effect of serving traffic, where the offline pass could be re-run and its output discarded | **OPEN, with a stated blocker.** #12984 read 23/1/1 on `ai/gemma3`, qualifying *that model with that prompt* and nothing else. **#13242 could not advance it: `gemma4:31b` meets the bar and cannot complete one record inside the 30-minute timeout — 15.4 of 22.8 GB in VRAM, ~32 % on CPU — while the 26B MoE that was fast enough is the model that produced the fabrications.** F-1 needs a host where the 31B fits in VRAM, and §7.4.8's D2/D3 fixed first |
| **F-2** | **The regression check that replaces the withdrawn invariant.** Sweep the corpus at several budgets with the form rule on and off; no row may go from *admitted* to *not admitted* | **Unit 3.** Any such row is either a bug or the threshold is wrong | Not run |
| **F-3** | **The threshold curve.** Bytes reclaimed and rows admitted, as a function of the ratio threshold | Sets §8.1's dial. If the curve is flat, the stratum distinction is decoration and a single rule is simpler | Not run |
| **F-4** | **Convergence, not coverage.** Run the same task twice against a cold area: run 1 fills, **run 2 must fire zero fills and reach the same or a better admitted set** | **Unit 2.** If run 2 still fills, the cache is not doing what §7.4.1 claims and the whole economic argument collapses to per-turn cost | Not run. **~0.3 % coverage today** (§4.1) |
| **F-5** | **#13106 §8.3's three-arm differential** — opaque labels vs names-only vs name+substance | **Unit 4.** Ties against either weaker arm sink the catalogue's central claim | Not run; **requires Unit 2 for the twenty rows** |
| **F-6** | **Run-record substance is worth reading.** Condense a run record and have a reader that did not write it judge whether the substance supports *what that run did and why* | **§9.2's form rule as applied to run records, and the sequencing of §9.3** | **ANSWERED, NEGATIVE — #13242.** With both gates removed the pass refuses all three records at 84 KB while reporting `operational failures 0`; with thinking disabled it yields a digest of *other nodes*, a fabricated count (10 where the record says 20), and invented figures where the prompt forbids rounding. Dropping `block` was tested and does not fix it — the remainder is a data table and the model transcribes it. **This does not sink the fill; it moves run records off the model path (§9.3.1) and re-sequences the gates last (§9.3)** |
| **F-7** | **Does `substance` participate in similarity ranking?** A probe node whose content and substance carry unrelated topics; query each topic in turn, on both instruments | **The fill, outright** — if ranking moved, retrieval would become path-dependent and §7.4.4's repair would be insufficient | **PASSED — #13241.** Content topic rank 1 / 0.7801; substance topic absent, field at the noise floor; `divoid_search` and `GET /api/nodes?query=` agree. **Now a standing guard rather than a gate: it is invalidated silently if DiVoid ever re-embeds on substance write, and nothing in the graph would announce that. Four calls; re-run when the fill ships** |
| **F-8** | **The transition's real shape.** Instrument fills per turn and their wall clock over a cold working area | **G3's value, and §7.4.2's estimate.** If a first run fires far more than ~3 fills, or a fill costs far more than ~31 s, the ceiling is set wrong and the latency claim is wrong with it | Not run — the numbers in §7.4.2 are derived from #12984, not measured on this path |
| **F-9** | **Does removing `block` make a record findable by its outcome?** Write (or construct) one record without `block`, then run §4.8.1's distinctive-outcome query against it | **§9.5's reason 4.** #13274 measures the *defect*; this measures the *fix*. If the record still does not return, the dump was not what displaced its identity and reason 4 is wrong — leaving only reasons 1–3 | Not run, **and it cannot be until a record is written after the change.** Reason 4 stands as inference until then |

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
| R7 | **`block` is dropped without #10904 re-ruling the stored-vs-response invariant**, or dropped in a way that makes a 15–25 KB record admissible **in content form** for the first time | **Reinstated — an earlier revision retired this and §9.5 puts the change back on the path.** Q4 names the #10904 dependency; §9.2's form rule (*substance form only, never content form*) is the guard that must exist first | A record shrinking below the budget while §9.2 is unimplemented |
| **R7a** | **The exclusion is removed before `block` is**, switching on a latent **recursion**: a record carrying another node's content is admitted into a later record's block, and that content is copied a second time | **This is why §9.5's ordering is a correctness requirement, not a preference.** It does not fire today **only because the exclusion is total** — cut at fusion (PR #48) and again at `admit` (§9.2). Either alone is load-bearing while `block` remains | A record's `block` containing a `===== CANDIDATE =====` section whose body is itself a run record |
| R19 | **This change is cited as the crowding fix.** It is the natural reading and it is wrong — §4.8 measured the crowding to be name-driven | §9.5 strikes it as a reason in the same table that lists the real ones; Q11 carries the actual lever | Any PR body, node or design citing `block` removal against candidate-slot crowding |
| R8 | **Unit 4 is started before Unit 2** and its falsifier cannot run | F-5's dependency is stated; #13106 §8.3 says the same | A catalogue arm running against rows with null substance |
| R9 | **The sweep acquires the ability to fill** — a model adapter enters `cmd/eval`'s closure for some unrelated reason, and every A/B silently starts measuring a substrate it is mutating | A13 is the guarantee, and it should be pinned by a test asserting the closure rather than left as a convention | `go list -deps ./cmd/eval` naming any model adapter or `internal/condense` |
| R10 | **Substance is lost and stays lost**, because the sync that republishes a body has no step that re-derives it — measured three times over (§7.4.5) | **The fill is the mitigation, and it is the reason not to add a procedural one.** Under it the loss repairs on next use, whatever caused it | Substance still `null` on a re-synced design document *after* the fill ships and that node has been retrieved |
| R11 | **The steady-state argument is quoted as the cost** and someone plans against a free fill on a cold graph | §7.4.2 states the transition separately and gives its shape; F-8 measures it | Any plan citing *"basically free"* without naming the cold-start regime |
| R12 | **A fill fires below the size gate**, spending a model call and a fidelity risk to save ~11 % | G1, and #12984's 0.886 median below 4 KB is the number | A fill recorded against a candidate under 8 KB |
| R13 | **A bad substance is invisible and durable.** It is present, so the fill sees nothing to repair; it is consumed as fact by runs that never touch its node; and it survives exactly on the static nodes the fill is built to serve (§7.4.5) | **The requirement, not machinery.** §7.4.7 defines the capability the fill's model must have, and **F-1 qualifies that model before it is allowed to write.** The durability asymmetry is why the bar sits above the turn's | An unqualified model runs a fill — i.e. F-1 not re-run after the fill model or prompt changed |
| R14 | **The fill silently inherits the turn's model**, because both call `boot.LoadModel()` today (A18) and one process makes that invisible | §7.4.7: separate configuration, **no fallback**, capability absent when unconfigured | A fill recorded with the same model id the turn used, absent explicit operator intent |
| R15 | **A model is swapped for latency** and *fact* quietly changes meaning | §7.4.7 records the floor **as a class with its reason**, and F-1 re-runs on model change | A fill model changed with no F-1 re-run cited |
| R16 | **DiVoid starts embedding substance** and F-7's discharge silently becomes false, making retrieval path-dependent on what earlier runs condensed | F-7 is retained as a **standing guard**, four calls, re-run when the fill ships (A17, #13241) | A substance-topic query returning its node |
| R17 | **The fill is judged against the harness rather than the model.** D2 and D3 (§7.4.8) make a capable model and an incapable one both return nothing and both report success | Fix D2/D3 before F-1 is re-run; D1's silent-success shape is the tell | An F-1 result quoted from a run where `PROCESSOR_MODEL_PROTOCOL` was discarded |
| R18 | **A run record's substance is trusted because it reads well.** #13242 produced fluent prose that summarised the wrong nodes and asserted a count that was wrong | §9.3.2's template cannot fabricate; and any *model*-produced substance for this class is out of scope by §9.3.1 | A run-record substance written by a model path |

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
| **Q3** | ~~Does the condenser get a JSON path, or does the record get a prose projection?~~ **ANSWERED BY MEASUREMENT — neither.** #13242 shows the model path yields a digest of other nodes and fabricates counts on this class, and the block-stripped remainder is a data table it transcribes rather than condenses | **No longer blocking** | **Deterministic rendering** (§9.3.2). The template *is* the projection, it needs no condenser and no model, and it cannot invent a figure |
| **Q4** | ~~Does `block` leave the stored record?~~ **CLOSED — §9.6 re-rules #10904's invariant.** `block` leaves the stored body, stays in the response, and is replaced by `blockBytes`. The amended invariant names exactly two divergences and keeps its test. **The blocker Q4 named is discharged by this document rather than deferred to #10904** | No | Adopt §9.6.5's wording into #10904 §8.1 and rename the guard test. Forward-only; nothing rewrites existing records |
| **Q5** | **Who measures substance coverage, and how often?** Nothing does today | No | A line in the sweep report; it is one query |
| **Q6** | **`fields=substance` works on the listing route and is undocumented in #8** | No | One line in #8 by whoever touches it next. Named because A4 rests on it |
| **Q7** | **Two-phase retrieval** — rank without bodies, then batch-fetch the survivors. Measured 1,236,611 B → 115,382 B on the yardstick (§7.1) | No | Its own unit, any time. It composes with every payload rule and depends on none of them |
| **Q8** | **G3's value, and its retirement condition** (§7.4.3). Recommended 2, on a three-fill estimate that is derived rather than measured | No — but it ships with the fill | Set it from **F-8**, and write the retirement condition into the same commit. It is a transition instrument, not a constant |
| **Q10** | ~~How is a substance attributed to the model that produced it?~~ **WITHDRAWN — no provenance machinery.** It was raised as a hedge against choosing the model badly; the answer is to **define the requirement and qualify against it** (§7.4.7), not to instrument for having failed to | No | Struck rather than deleted, because a later reader will reach for provenance the same way. A19/A20 record what was checked. What remains is free: the run record names the model behind each fill, as it already does for the answer |
| **Q11** | **The record's *name* is a retrieval surface and nothing treats it as one.** §4.8: `processor-run <ts> — <input truncated to 80 runes>` (`write.go:101`) makes a repeat of an input a near-exact lexical match, and that — not content — is what holds ranks 1–11 | **Not blocking anything here** | **Named, deliberately not addressed.** It is its own unit with its own falsifier, and widening this document into it would bury the finding. Neither this design nor #12955, #13106 or #13203 treats a name as ranked text |
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

### Unit 2 — the fill *(a behaviour, not a campaign; F-7 discharged, F-1 open)*

1. **F-7 is discharged (#13241) — substance is not embedded, so the fill cannot move a rank.** Carry it
   forward as a **standing guard**: re-run the four-call probe when the fill ships, because a change to
   DiVoid's embedding would invalidate every downstream conclusion here and announce nothing (A17).
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

### Unit 3a — run records get a rendered substance *(shape first, gates last)*

**F-6 has run and the ordering in an earlier revision was backwards (#13242).** Removing the two gates does
not produce a usable substance; it produces a digest of other nodes with a fabricated count. **Do not start
with the gates.**

1. **Build the template** (§9.3.2): a deterministic rendering of the record's own fields — input, subject,
   queries, admitted/cut counts with their reasons, tool sequence, terminal reason, model calls,
   `capReached`, and the answer verbatim or truncated with a marker. **No model.** The load-bearing property
   is that a template reads `len(candidates)` and therefore cannot assert ten where there are twenty.
2. **Write its output as the record's `substance`** at write-back time. The template **reads fields and
   ignores `block`**, so it works whether or not `block` is still stored — it neither requires nor blocks
   §9.5's removal, and the two units are independent.
3. **Only then reconsider the gates.** `condense.go:283` and `:296` keep run records off the *model* path,
   which after F-6 is where they should be. They are not obstacles to remove; they are the boundary between
   the two classes in §9.3.1 — and if the template writes the substance directly, the condenser never needs
   to see a run record at all.
4. **Judge the result the way F-6 judged the model's:** a reader who did not write it, checking every count
   and figure against the record. A template earns trust by construction, but only once.
5. The fill's **refusal contract** (§7.4.4) must still surface a condenser skip as a stated reason. D1
   (§7.4.8) is that failure mode already observed offline.

### Unit 4 — the catalogue

**#13106 §8.3, unchanged.** Do not start before the fill has warmed the twenty rows F-5 needs, and
note that #13106 already establishes it probably requires the call ceiling raised.

### What must not happen

- No substance rendered into a block before **F-1** passes.
- No provenance clause in the form rule (S2, R5).
- No predicate keying on the `session-log` type (R6) — #11387 is its live occupant.
- **No `block` removal before §9.2's form rule exists** (R7). #10904's invariant is re-ruled in §9.6 and no longer blocks it — but the amended wording and the renamed guard test must land **with** the removal, not after it, or the tripwire is spent rather than preserved.
- **No removal of the run-record exclusion before `block` is gone** (R7a). While the dump is still stored, the exclusion is the only thing stopping a record's content being copied into a later record's block. **That ordering is a correctness requirement, not a preference.**
- **No description of `block` removal that presents it as the crowding fix.** §4.8 measured that it is not (R19).
- **No fill shipped without re-running F-7's probe** (#13241). It passed once; it is silently invalidated if DiVoid ever embeds substance, and nothing would announce that.
- **No fill port constructed in `cmd/eval`**, and no model adapter admitted to its dependency closure — that
  closure *is* the determinism guarantee (A13, §7.4.4).
- **No fill counted against `MaxModelCalls`** (§7.4.3).
- **No silent fill refusal.** Every skipped fill carries its reason, or §9.3's two gates become invisible.
- No claim that the steady state is free without saying we are entirely inside the transition (§7.4.2).
- **No fill from a model that has not passed F-1**, and **no fallback to the turn's model** (§7.4.7).
- **No substance written without the run record naming the model that produced it** (§7.4.7).
