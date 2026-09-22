# Architectural Document: How We Know A Change Helped — measuring a living graph without an answer key

> **Origin:** #14525 (the task, reframed by Toni 2026-09-22) · state assessment **#14524** · authority
> **#13534 §16**, in particular §16.1 (*60 KB of full documents is the strategy we exist to challenge*) and
> §16.3 (*the corpus asks the wrong question*) · project **#10422** · map root **#10454**.
> **Predecessor consumed, not superseded:** **#11092** — the task-outcome A/B at one memory state. Its
> two-loop structure, its judging discipline and its write-suppression shape survive intact and are
> re-founded here on a substrate that works. Its **required-node key** does not.
> **Baseline:** `main` = `3f4ea35`; read on `feat/a-run-that-got-nothing-behaves-differently` at `63abb09`
> (PR #102, P1 the outcome verdict). Everything cited was read in this tree or fetched live from DiVoid.
> **Nothing was run.** No inference, no container, no sweep, no graph write. This document is an argument
> from records and code.
> **Repo home when the first unit ships:** `docs/architecture/how-we-know-a-change-helped.md`.
>
> **REVISION 2, 2026-09-22 — against #13534 §17.** Toni ruled on this document's D-1 and **rejected its
> recommendation**. A mechanical rank-fidelity gate is out; the objective is a **comparison against a working
> reference** — an isolated agent in this harness, handed the same task in report form. The rejection
> generalises past the one decision and §4.4 records why, including the fact that the recommendation broke a
> rule this document had already written down. **Changed:** §4.4 (new), §5, §6.6 (new), §7, §9.5 (new), §10.3,
> §11 alt 8, §12.8 (new), §13, §15, §16 R-11/R-12/R-13, §17 D-1 and D-8/D-9/D-10, §18 items 7–8, §19 M4 and
> M7a. **Unchanged and confirmed unaffected:** §4.1's rule, the ledger (§6.2), the seam (§4), the hash-identity
> comparison (§8). §4.4 states the boundary explicitly so the ruling is not over-read.
>
> **REVISION 2.1 — D-8, D-9 and D-10 are closed, so nothing in §17 is waiting on Toni.** D-9 and D-10 taken
> by the coordinator as recommended; **D-8 is done and filed as #14571** — the six report-form probes and the
> blinded adjudication prompt. **#14571 reports one defect in §6.6.5 of this document** — the verdict
> vocabulary is asymmetric under blinding — recorded there with its recommended repair rather than repaired
> in place.
>
> **REVISION 2.2 — the vocabulary repair is ACCEPTED and APPLIED (§6.6.5).** `reference failed` added;
> both *thinner* labels reclassified from adjudicated verdicts to **derived** ones; withholding the size
> figures from the adjudicator promoted from a fact about the prompt to a **stated property** (§6.6.6).
> Also: **§19's milestone boundary amended** to the seam the shipped ledger artifact actually has, with the
> claim-versus-did predicates split out as **#14572**; **§6.2 gains a naming rule** for ledger predicates and
> **§12.9 records the permanent self-contamination** of any measurement-shaped probe.
>
> **REVISION 2.3 — §12.10 rules #14580** (the fill port writes past the suppressing decorator): the constraint
> widens from *no record* to **no graph mutation**, the mechanism is a **per-seam decorator** and the guard is
> a **transport-level zero-non-GET assertion written in the fill-ON configuration**. **§12.11 adds V-13**, the
> positive control — three instances in one day of a guard that cannot fail — and **§12.12 collects the
> falsifier register** so V-numbered references resolve.

---

## TL;DR

*The apparatus is fine. The readout is wrong.*

`cmd/eval` already runs the **production** retrieval and admission functions against the **live** graph with
**zero model calls** and byte-identical reproducibility across arms. That is a good instrument. It then
throws the block away and collapses everything it measured into two rates against a hand-pinned required
set — and those rates are flat at 11/23 and 9/23 for twelve days while the same sweep, at candidate level,
measured 169 → 187 admitted rows and traced a six-row loss on `r01` to its exact mechanism (#14281).
**Nothing failed to move. The number failed to see it.**

So the strategy is not *"replace the corpus"*. It is:

1. **Retire the readout, keep the apparatus.** `recall@k` against `required[]` stops being a rate, a
   headline, and an exit code. The per-row disposition set it was computed from becomes the output.
2. **Make the run record the measurement substrate.** Every run already writes a complete, self-describing
   record: every candidate the query returned with its rank, similarity, content hash, byte size, substance
   size, rendered size, form and cut reason; the whole block verbatim; every tool round with its own
   dispositions; the dial snapshot the run executed under. **A record is a snapshot of the graph restricted
   to what one question reached** — which is exactly the thing Toni says to judge against.
3. **Split gates from findings, and never let one wear the other's clothes.** A *gate* is mechanical,
   recomputable from a record alone, and **may only ever fail** — it can prove a run was insufficient and
   can never prove one was sufficient. A *finding* is an adjudicated reading of an actual result, produces
   no exit code, and blocks nothing.
4. **Replace the answer key with an adjudicated triage that has three exits.** A written expectation that
   disagrees with a result resolves to *regression* / *the probe is stale* / *the graph found something
   better* — and the last two **must edit the probe in the same act**. Instrument drift becomes an audited
   write instead of a silently decaying constant.
5. **Replace A/B/A with per-candidate hash identity.** A/B/A brackets 25 of 150 neighbourhoods arm-vs-arm
   with the error in the unsafe direction, by its own admission (`m3-derived-recall.md` §9.1a). Every record
   already carries a content hash per candidate. Comparing arms on the **union of ids both touched** voids
   the rows that moved rather than the whole triple, is exact rather than probabilistic, and costs nothing.
6. **Narrow the parked dials offline; choose between them fresh.** `SubstanceRatioThreshold`,
   `BlockOccupancy` and `RelevanceFloor` are all parameters of `admit`, and `admit`'s decision needs only
   scalars the record already carries — `Similarity`, `Size`, `SubstanceSize`, `Rank`. So a replay over the
   archive can **enumerate** what each dial value does to block composition and collapse a continuous range
   into a handful of **distinct** outcomes — on #14281's own curve six settings produced three. **It may not
   select among them.** #13534 §17.3 binds: both arms fresh, no arm served from an archived record. Replay is
   the search-space narrower; the choosing happens live.
7. **The objective is a working reference, not a number.** Per **#13534 §17**, which *rejected* this
   document's first recommendation: processor's result is compared against an **isolated agent in this
   harness** given the same task in report form — *given task X, return the relevant memories from DiVoid*.
   Three loose KPIs — **topic match, size, open questions** — read for the **shape of a gap**, never summed
   into a score. The bar is *works at all*, then *good enough under a restrictive environment*. **Parity is
   not the target and beating the reference is not the target.**
8. **And the comparison is on information, not on nodes.** #13534 §17.6: *two results are equivalent when
   they carry the same substantial information for the task, regardless of which nodes carried it.* **So every
   set-agreement metric is barred** — overlap, Jaccard, *how many of the reference's nodes did processor also
   find* — because a run reaching the same informational state through entirely different nodes is a
   **success** that a set metric reports as near-total failure. That is the **third** rejection of one shape
   (§4.4), and the invariant behind all three is that **the substrate is diffuse, so anything measuring its
   surface measures noise.** The comparison is therefore an adjudicated verdict from a closed vocabulary,
   blinded, carrying its reasoning — and the reasoning is what a human spot-checks.

**What needs a model stays small and stays last.** Report probes, both arms fresh, n=1 for the binary bar.
**We will never make a statistical claim about answer quality.** We buy attributability by removing variance
from everything except the model — not by buying sample size at a single endpoint that takes 20 s to 5 min
per run. **And the reference arm is itself stochastic**, which costs n=1 the ability to size a gap; §13.3
states exactly what survives that and what does not.

**And one thing this cannot reach, stated up front:** #14478's substance fidelity failures — 6 of 38, with a
negation flip that inverts a live rule — are **not visible to any run-level instrument**, because the defect
is in a stored artifact, not in a run. §14.3 says what does see it, and it is a different instrument.

---

## 1. Problem Statement

**The product cannot tell whether a change to it helped, and the reason is not a missing instrument — it is
a category error in the one we have.**

The visible symptom is an ordering deadlock. `SubstanceRatioThreshold = 0` and `BlockOccupancy = 0` are both
merged, both fully wired, both recorded on every record, and both inert — each explicitly waiting on a
measurement to choose its value. The measurement they were to be selected against is required-node
admissions, and #14281 measured that metric **flat at 10 across all six arms including the arm with the rule
off**. So the mechanism the product exists to demonstrate (#13534 §16.2 — *"it's a core concept I want to
see and we are at it for a while without it being in the product"*) is blocked on a number the instrument is
structurally incapable of producing.

The underlying cause is Toni's, verbatim, and it is the brief for this document:

> *"Who determines these 25 required documents and what makes it deterministically correct? The whole idea is
> to look into a living graph, results will vary and depending on the question you get different results. The
> question should never be → are the nodes magically tagged required in the result, but more like — is the
> result for the snapshot of the graph now correct, does it contain correct matches, are the matches enough
> to solve the task, how does the loop handle the result, does it requery or does it take insufficient
> information as given? We are in exploration mode, there is no given truth — we can give out expectations,
> but they can fail and the behavior could still be right → depends on our analysis of the actual result."*

**The problem this document solves:** design an approach to knowing whether a change made the product better,
for a system whose substrate is a living graph and whose tasks are natural language — one that is repeatable
without a person reading every run, that is affordable at one model endpoint, and that does not smuggle a
judgement inside something shaped like a gate.

**Success criterion, and it is #13534 §11's test applied to the instrument itself:** *if this ships, does the
answer Toni gets get better — and would he be able to tell?* For a measurement strategy the second half is
the whole of it. An instrument that cannot distinguish the arms fails it by construction, which is precisely
what #14281 recorded.

---

## 2. Scope & Non-Scope

### In scope

- The **seam** that decides what is mechanical and what is judged, and why it sits where it does (§4) —
  including **the invariant behind three separate rejections** of a measured-surface instrument (§4.4).
- The **five instruments** and the one substrate they share (§5–§7), including the **reference arm** and the
  **adjudicator** #13534 §17 introduced (§6.6).
- The **comparison regimes** — how two arms are compared when the substrate moves under them (§8).
- The **reading procedure** that replaces the answer key, as a repeatable process with recorded exits (§9).
- What happens to `cmd/eval`, `corpus.json`, `corpus-anchor.json`, `derivations.json`, `compare.py`,
  `smoke.py` and `required[]` (§10).
- **Cost, per instrument, with who runs it** (§13).
- The **record test**: would this have caught what we already know we missed (§14).
- What is **unknowable** today and what it depends on (§18).

### Explicitly out of scope

| Not designed here | Why, and what would change it |
|---|---|
| **A bigger or repaired pinned corpus** | #13534 §16.3 retires it as the primary instrument, and §10.2 gives the structural argument as well as the ruling. **No case for one is made here.** Trigger to revisit: a demonstration that label rot is bounded on this substrate, which #14084's 3-of-25 unreachable required documents argues against |
| ~~**An automated judge of answer quality**~~ | **MOVED INTO SCOPE by #13534 §17.6.** Revision 1 excluded it as an unvalidated measurement inside the measurement; the ruling establishes that the only stable quantity on a diffuse substrate is the information, and comparing information **is** a judgement — so the judge is required rather than optional. §6.6.6 designs it, §11 alternative 2 records the overturn, and what stays excluded is a **scored** judge |
| **A ranking, rating or aggregate of answer quality** | §6.6.5 rules 3 and 4. A 0–10 score, a similarity number, or *"equivalent on N of 6"* as a headline is barred — a scalar is a target and a target is a thing to tune toward. **No trigger; this one does not come back** |
| **A graph snapshot or second store** | #11092 §4.1's argument is unchanged and is restated in §11 alternative 3: *a snapshot you can query is a different retriever.* The one legitimate snapshot is a record, because it is the real retriever's own output |
| **A statistical claim about answer quality** | §13.3. We will not make one, at any n. The position is #11092 §5.1's, carried forward and made explicit rather than hedged |
| **Changing what `internal/loop` does** | Every instrument here reads what the loop already writes, or re-executes a function the loop already calls. The one exception is the write-suppressing port, which is a second implementation of a declared seam and changes no shipped file (§12.3) |
| **The planner, the tool framing, the substance path itself** | #13534 §16.5 ranks those 1, 2 and 3. This is rank 4, and its whole job is to make 3 selectable |
| **Per-stage timing in the record** | The record carries no durations at all — elapsed is logged, not recorded. That is a real gap for cost accounting and it is filed, not fixed here (§17 D-7) |

---

## 3. Assumptions & Constraints

**Measured or cited. Flagged where uncertain.**

| # | Constraint | Source |
|---|---|---|
| C1 | **The graph is shared, mutable and unversioned.** It carries no hash, moves between arms inside one session (8/23 → 9/23 on unchanged code, 65 minutes apart), and peer sessions write to it | `m3-derived-recall.md` §9.1a; #13702/#13703 |
| C2 | **Every run writes a record into the graph it reads from**, unconditionally, surviving request cancellation. ~29 % of an aperture is self-produced rows; the share is **anti-correlated** with the memory being useful (0, 1, 9, 17 across five runs) | `turn.go:202`; #13534 §11 |
| C3 | **Retrieval is bit-reproducible at a fixed graph state.** Two runs of one input shared 19 of 20 candidates and the same top hit to four decimals; six sweep arms returned byte-identical candidate id lists on every row | #13534 §11; #14281 |
| C4 | **`cmd/eval` is structurally barred from the model adapters** by `dependency_closure_test.go`, which greps `go list -deps` for `internal/condense`, `internal/fill`, `internal/openaicompat`, `internal/ollama`. **A model-in-the-loop instrument is a new binary, and that boundary is deliberate** | `cmd/eval/dependency_closure_test.go` |
| C5 | **One model endpoint**, `gangolf:11434`. A real run takes 20 s to 5 min. The host is unversioned and unmonitored and has already produced a **3.4× swing on identical input** — 87.9 s, then a 302 s timeout, with nothing in the product changed, because the model was serving from system RAM rather than VRAM | #13534 §8 |
| C6 | **Tasks must not run while a measurement is in flight**, and test runs plant nodes a later run retrieves — a scaffolding node scored 0.82 on a subsequent run's input | #13592; #14250 |
| C7 | **There is no CI.** No `.github` directory exists. Every guard's red reaches a human on a good day and nobody on a bad one | #13429 |
| C8 | **Every retrieval and budget knob is a compile-time constant.** `internal/boot/config.go` carries no retrieval or budget setting at all. `cmd/eval` exposes exactly one dial as a flag (`-substance-ratio`); `RelevanceFloor` and `BlockOccupancy` have **no arm mechanism whatsoever** | `turn.go:11-24`; `cmd/eval/main.go:95-116` |
| C9 | **The archive is 42 records** at `6e132a3`, parseable: `internal/runbackfill` already decodes a run node's fenced JSON back into a `loop.Record`, with named skip reasons for every shape it cannot | `internal/runbackfill/runbackfill.go` |
| C10 | **A single operator.** Blinding is a real constraint rather than a formality, and any protocol requiring a second human judge is unimplementable | #11092 §11 Q3 |
| C11 | **The reference harness is unversioned and stochastic.** Claude Code's model, system prompt and tool surface change without notice, and an agent's answer to one probe varies run to run. It is a **working reference, not a ground truth** | #13534 §17.3 |
| C12 | **The substrate is diffuse: retrieval from a living graph has no canonical answer.** Not a required set, not an ordering, not a node set. Only the information is stable enough to compare | #13534 §17.6 |
| C13 | **The store holds this project's own measurement apparatus**, and keeps accumulating it. Any probe about measurement, retrieval quality or the instrument is **self-contaminating by construction** — permanently, not as a passing state | §12.9 |

**Assumptions I am not certain of, and which the first unit would settle:**

- **A1.** That all 42 archived records decode cleanly enough to replay. F-1 (§14.4 of the P1 design) computed
  a verdict over all 42 and reported **no schema-drift handling was needed at all**, so this is well
  supported, but it was a verdict over four facts rather than a full replay.
- **A2.** That the archive will keep growing from *real* traffic rather than from measurement traffic. If it
  does not, the ledger becomes a census of a synthetic population (§16 R-4).
- **A3.** That `gangolf` remains the only endpoint. Two endpoints would make the model an uncontrolled
  variable across arms and would invalidate every model-in-the-loop comparison not run back-to-back.

---

## 4. The one ruling everything else follows from

Before any mechanism: the seam.

> **A run record states what the loop did. It never states what the content meant. Every instrument that
> stays on the record side is mechanical, cheap, recomputable and safe. Every instrument that crosses to the
> meaning side is a judgement, and dressing it as a number is the failure this whole task exists to name.**

`recall@k` crossed that seam. It looks mechanical — it is a count over a closed set with a deterministic
rule — but its *input*, the required set, is somebody's reading of what a question means, fixed at a moment,
scored against a substrate that moves. **It is a judgement wearing a measurement's clothes**, and that is
exactly why it went flat without anyone noticing: a judgement cannot fail loudly.

Three consequences, and they are the design.

### 4.1 Mechanical instruments may only fail, never pass

A gate that asserts *"this run obtained nothing usable"* is sound: the record proves it. A gate that asserts
*"this run obtained enough"* is not, at any threshold, because *enough for what* is a reading of the task.

So every mechanical predicate in this design is a **necessary condition stated negatively**. Green means
*nothing mechanical is wrong*; it never means *this was good*. This asymmetry is not a modesty gesture — it
is what lets gates be absolute and automatic without ever being wrong about quality.

The repo has already discovered this from the other side. `report.go`'s own alarm disowns its headline on
every run where it fires: *"the admitted rate is a reading of the assembler, not of the retriever."* And the
README says it outright: *"a green build proves the instrument works while proving nothing about what it
measures."* Both are correct and neither is currently acted on.

### 4.2 Toni's four questions split cleanly across the seam

| his question | side of the seam | instrument |
|---|---|---|
| **1. Is the result correct for the graph as it is now?** | meaning | adjudicated reading, per row (§9 step 3) |
| **2. Are the matches sufficient to solve the task?** | meaning — **with a mechanical necessary condition** | insufficiency is a gate; sufficiency is a reading (§9 step 4) |
| **3. How does the loop handle what it got?** | **record** | the ledger — fully mechanical (§6.2) |
| **4. Does it requery, or take insufficient information as given?** | **record** | the ledger — fully mechanical, and nothing computes it today (§6.2) |

**Half of the reframe is answerable with no judge, no key and no model**, from fields already written. That
is the reason P1's shape — a verdict computed only from facts the loop holds, never from the answer's prose —
is not a cheap first step before the real work. **It is the correct shape for that entire half.**

### 4.3 The ceiling of mechanical scoring, named exactly

Asked directly: *is P1's shape the ceiling of what mechanical scoring can reach?* It is the ceiling **on the
meaning side** and it is nowhere near the ceiling on the record side.

- **Still unbuilt and fully mechanical:** requery behaviour, yield accounting, claimed-versus-dispatched
  actions, cost per admitted row, block composition, aperture waste, dial exercise, mechanism observation.
  None of these needs a model, a key or a human. §6.2 enumerates them.
- **Not reachable, ever:** relevance, sufficiency, correctness. There is no clever encoding that crosses the
  seam. Two known attempts to cross it and what each becomes:
  - **Required-node lists** → a fixed reading scored against a moving substrate. Retired.
  - **Lexical "evidence use"** (#14525 tier 2, *did any admitted node's content reach the answer*) →
    **rejected**, §11 alternative 5. It is gameable by restatement, it is not an overlap threshold anyone can
    defend, and it produces a number that reads as *the answer used the memory* while measuring word reuse.
- **The one legitimate residue on the far side is syntactic, not semantic: citation.** Whether the answer
  *names* an admitted node's id or title is a fact about characters, not about meaning. Record it. It
  supports *the answer did not cite anything admitted* — a finding worth raising — and it must never be read
  as *the answer used the memory*, which it cannot establish.

### 4.4 The rule this document broke in its own first recommendation

**Recorded rather than quietly repaired, because the near-miss is worth more than a clean edit.**

Revision 1 stated §4.1 — *mechanical instruments may only ever fail, never pass* — and then, thirteen
sections later, recommended a **mechanical rank-fidelity gate** as the objective a dial is selected against.
Toni rejected it (#13534 §17.2) on the rule this document had already written down:

> §16.3 retired `recall@k` because it scores a living graph against one fixed reading. **A rank-fidelity gate
> is the same move one level down** — it swaps somebody's reading of *which documents are required* for
> somebody's reading of *which ordering is right*, and puts arithmetic on top. **The input is still a
> judgement**, and a judgement dressed as a number cannot fail loudly. That is what let retrieval sit flat for
> twelve days.

**Using a mechanical instrument to *select* a configuration is using it to pass one.** That is the exact
violation, and it was invisible from inside the recommendation because the arithmetic on top was sound. The
document named the dangerous shape in §4 and then produced an instance of it in §17 — which is the strongest
available evidence that naming a failure mode does not immunise you against it.

**Two things the ruling does not reach, and the boundary is worth stating sharply before somebody over-reads
it:**

- **The ledger is untouched.** Its predicates state what the loop *did* and score no content. Every one
  asserts a negative — *this run obtained nothing*, *the loop asked for nothing after a shutout*, *this dial
  has never taken a second value*. None passes a configuration; none reads a document for meaning. §17 rules
  on the objective of a **quality comparison**, which the ledger does not make. M1/M2 proceed unchanged.
- **Mechanical gates as such survive.** What is out is a mechanical gate standing in for a judgement about
  *whether the result was good*. A gate refusing a run that admitted nothing is not that and never was —
  §4.1's asymmetry is exactly the line, and the ruling enforces it rather than overturning it.

**And it has now happened a third time, one level further down.** #13534 §17.6 ruled out **node-set
agreement** — overlap, Jaccard against the reference's set, *how many of the reference's nodes did processor
also find* — on the same grounds. Three rejections of one shape:

| level | what was scored | why it failed |
|---|---|---|
| **§16.3** | which documents are **required** | somebody's reading, fixed at a moment, against a moving graph |
| **§17.2** | which **ordering** is right | the same reading one level down, with arithmetic on top |
| **§17.6** | which **nodes** both arms share | scores the carrier, not the cargo |

> **The invariant all three share, and it is the thing that stops a fourth: the substrate is diffuse, so
> anything that measures its surface measures noise.** Retrieval from a living graph has **no canonical
> answer** — not a required set, not an ordering, not a node set. Only the **information** is stable enough
> to compare, and comparing information is a judgement.

### 4.4a The same shape, a second time — and it was the ruling that carried it

**Recorded here because one instance reads as a lapse and two read as a property of the work.**

§4.4 is this document's own recommendation breaking a rule it had already stated. The second instance runs
the other way: **the ruled verdict vocabulary (§6.6.5) could not express `reference failed`** — the exact
evidence R-11's falsifier depends on — and nobody saw it until the adjudication prompt was **built** and the
blinded judge turned out to have no way to emit it.

| instance | who | caught by |
|---|---|---|
| a mechanical gate proposed as the objective (§4.4) | this document | the ruling that rejected it |
| a verdict set that cannot record its own falsifier's evidence (§6.6.5) | the ruling | **building the instrument** |

> **A falsifier whose evidence has no label is not a falsifier**, and that is invisible from inside the
> specification — it becomes visible at the moment something has to emit the value. This is #13534 §14's rule
> arriving one layer earlier than usual: *every substantive finding came from an instrument being checked,
> not from an instrument running* — here, from an instrument being **written**.

**The practical consequence for the phase order:** it is a second argument for #14561's own habit of
computing a verdict over the archive before writing the production path (P1's F-1, M2's guidance). **F-1
moved P1's specification twice. Building the prompt moved this one once.** Neither cost anything, because
neither had shipped.

**The generalisation to carry forward:** before proposing any instrument, ask **what is its input?** If the
input is a list, an ordering, a node set, a threshold or a rubric a person authored about meaning, the
instrument is a judgement, and no arithmetic downstream changes that. **The only honest options are to
adjudicate it openly, or to compare against something that already works.** §6.6 is the second, and it does
the first inside it.

**The positive form of the invariant, because a rule stated only as a prohibition invites a fourth
workaround:** a run that reaches the same informational state through **entirely different nodes** is a
**success**. Any instrument that cannot say so is measuring the wrong thing, whatever it is called.

---

## 5. Architectural Overview

One substrate, five instruments, two loops, one adjudication.

**The fifth arrived with #13534 §17 and is the outer loop's objective**: a reference arm, judged on
information sufficiency (§6.6). It is drawn inside the outer loop below rather than beside the other four,
because it is not an instrument the inner loop can run.

```
                          ┌────────────────────────────────────────────┐
                          │   THE ARCHIVE  (substrate, already exists) │
                          │   run-record nodes in DiVoid, 42 today     │
                          │   each: dispositions + hashes + block +    │
                          │   tool rounds + dial snapshot + outcome    │
                          └───────────────┬────────────────────────────┘
                                          │  decoded by internal/runbackfill
        ┌─────────────────────────────────┼─────────────────────────────────┐
        │                                 │                                 │
        ▼                                 ▼                                 ▼
┌───────────────┐              ┌─────────────────────┐            ┌──────────────────┐
│ ① THE LEDGER  │              │ ② THE REPLAY        │            │ ④ THE READING    │
│ pure fn of    │              │ re-runs admit over  │            │ adjudicated, per │
│ one record    │              │ archived scalars at │            │ result, 5 steps, │
│ (or of a      │              │ a different dial    │            │ 3 exits on a     │
│  named set)   │              │                     │            │ failed prior     │
│               │              │ 0 model, 0 graph    │            │                  │
│ 0 model,      │              │ (hash-gated variant │            │ human judgement  │
│ 0 graph       │              │  fetches content)   │            │ — here and here  │
│               │              │                     │            │   only           │
│ GATES         │              │ GATES + deltas      │            │ FINDINGS         │
└───────┬───────┘              └──────────┬──────────┘            └────────▲─────────┘
        │                                 │                                │
        │      ┌──────────────────────────┴──────────────┐                 │
        │      │ ③ THE SWEEP (cmd/eval, re-readout)      │                 │
        │      │ live graph, production Retrieve+Assemble│                 │
        │      │ 0 model calls, per-row dispositions     │                 │
        │      │ arms compared on candidate-id hashes    │                 │
        │      └──────────────────────┬──────────────────┘                 │
        │                             │                                    │
        └─────────────┬───────────────┴────────────────┐                   │
                      ▼                                ▼                   │
             ┌──────────────────┐            ┌───────────────────────┐     │
             │  INNER LOOP      │            │  OUTER LOOP           │     │
             │  finds changes   │───────────▶│  accepts changes      │─────┘
             │  minutes, free   │            │  processor vs the    │
             │  unattended      │            │  reference arm, fresh │
             └──────────────────┘            │  quiet window, bundle │
                                             └───────────────────────┘
```

**The inner loop finds; the outer loop accepts.** That structure is #11092 §7's and it survives the reframe
without a word changed. What changes is that the inner loop no longer reports a rate against a key, and the
outer loop no longer needs a corpus to be runnable — it needs **report probes, a reference arm and an
adjudicator**.

**What the outer loop compares against is not this document's to choose, and #13534 §17 chose it:** an
isolated agent in this harness, given the same task in report form, run fresh. §6.6 is the component; §4.4 is
why the alternative this document first proposed was wrong.

**The artifact that crosses between them is the Bundle** (§7.5): everything one measurement touched, with
its window, its costs, its hashes and its arms, addressable and requotable. A figure that is not in a bundle
is not a measurement — which is the direct architectural answer to #14524's own correction, where a count
inside a report *"reads exactly like a measurement and is not one"* and was quoted forward into a design's
motivation and into a conversation before anybody recomputed it.

---

## 6. Components & Responsibilities

### 6.1 The Archive — substrate, not component

**Owns:** nothing. It is what the product already produces.

**What makes it the right substrate, and this is the load-bearing observation of the whole design:**

A `Disposition` carries `Rank`, `ID`, `Type`, `Name`, `Similarity`, `Size`, `ContentHash`, `Included`,
`CutReason`, `Sources`, `SubstanceAvailable`, `SubstanceSize`, `PayloadCap`, `Form`, `RenderedSize` — for
**every candidate recall returned, admitted or cut**. The record additionally carries the derived query set,
the derivation error, the time window, the anchor summary with its own hash, the **entire assembled block
verbatim**, every tool round with its own nested dispositions, the usage array per model call, the stop
reason with the endpoint's raw string, the provider and endpoint, and `Limits` — the nine-constant snapshot
of the configuration the run executed under.

Therefore:

> **A run record is a snapshot of the graph restricted to exactly what one question reached, taken by the
> real retriever, timestamped, immutable, and self-describing about the configuration that read it.**

That is precisely the object Toni's framing asks us to judge against — *"is the result for the snapshot of
the graph now correct"* — and we have 42 of them, free, already written.

**What it does NOT own:** truth. A record proves what the loop saw and did. It proves nothing about whether
what it saw was right, and no amount of reading records changes that.

**One property of the archive that must be stated because it bounds everything built on it:** it is a
**census, not a sample**. Records exist for runs somebody chose to make — twelve of the twenty-five recent
ones are the same task text, an operator sweep over workspace mounts. Aggregates over the archive describe
what happened, never what would happen. Every ledger figure must name its selector for this reason (§12.5).

### 6.2 The Ledger — mechanical predicates over records

**Owns:** every claim about the run's behaviour that is decidable from the record alone.
**Does NOT own:** any claim about content, relevance, sufficiency or correctness. Any claim requiring the
graph, the clock, or a second record — unless it is declared an *archive-level* predicate and names its
selector.

**Already shipped, and it is the first entry:** P1's `Outcome` — `curtailed` / `empty` / `ungrounded` /
`delivered`, from three predicates (`produced`, `grounded`, `curtailed`) plus `acted` recorded beside them
and consumed by no verdict. It is computed at `turn.go:200`, before write-back, and recomputed rather than
read back when rendered — so an archived record with no stored outcome still yields one.

**What the ledger adds, grouped by which of Toni's questions it answers.**

**Q3 — how does the loop handle what it got:**

| predicate | mechanically defined as | fires today |
|---|---|---|
| **claim-action divergence** | the answer's prose asserts a write while `acted["writeFile"]` is zero | #14247. **A flag for adjudication, never a truthfulness verdict** — #13065's correction binds: the reader's expectation about what a word means is not the system's defect |
| **cited-not-admitted** | the answer names a node id or title that appears in no admitted disposition | unmeasured |
| **admitted-none-cited** | at least one row admitted, no admitted row named in the answer | unmeasured |
| **aperture waste** | share of candidates cut as `self-produced`, per run | ~29 % mean, 85 % worst |
| **top-rank starvation** | the rank-1 candidate cut for want of room | already WARNed, never counted |

**Q4 — does it requery, or take insufficient information as given** — *nothing in the repo computes any of
these, and they are the direct instrument for the question Toni says no current instrument asks*:

| disposition | mechanically defined as |
|---|---|
| **accepted insufficiency** | the block admitted zero rows (or fewer than `thinKnowledgeThreshold`) **and** the run emitted zero recall rounds. The loop was handed nothing and asked for nothing |
| **unproductive requery** | two or more dispatched recall rounds whose yield — rows not already visible to the model — was empty |
| **productive requery** | a dispatched recall round that admitted at least one row not already in the block |
| **refused requery** | a recall round carrying `Error` (cap reached, or P2's recall-closed refusal), which grounds nothing and was never put in front of the model |

**Archive-level predicates — these are the ones that would have caught the twelve-day pattern:**

| predicate | defined as | status today |
|---|---|---|
| **dial exercise** | every dial in `Limits` has taken ≥2 distinct values across the selector, or is declared unexercised with a reason | **fails** on `SubstanceRatioThreshold` and `BlockOccupancy` |
| **mechanism observation** | every record field a merged mechanism can write is non-empty in ≥1 record within N runs of merge | **fails** on `Fills` — zero records carry one |
| **cost per admitted row** | input tokens ÷ admitted rows, per run and per selector | 1,236,520 input tokens bought 15,686 output tokens over 25 runs; never expressed per row |

**One rule on how a predicate is named, and it is not cosmetic.**

> **A ledger predicate is named for the fact it computes, never for the worth of what it found.** If the name
> contains a word that could appear in an argument about whether the behaviour was *good* — *productive*,
> *wasteful*, *pointless*, *correct*, *accepted* — **the name is doing the arguing**, and a reader will quote
> it forward as a finding.

**This is not hypothetical and it did not survive first contact.** `ProductiveRequery` / `UnproductiveRequery`
compute a fact about **yield** — whether a dispatched round returned rows the model had not already seen. They
say nothing about whether those rows helped, which is on the far side of §4's seam and unreachable. The first
implementation's own filed note already read *"it requeried usefully once and then four times pointlessly"* —
**the banned inference, made from the name, before the unit shipped.**

A name is an input to every future reader's reasoning, and §4.4's lesson applies to it exactly: **a mechanical
fact wearing an evaluative name cannot fail loudly.** It simply gets quoted forward, which is what happened to
the 7-of-25 figure.

*Ruling:* **rename to the fact — yield, or its absence — as a follow-up unit, not a retrofit into an open PR.**
The identifiers came from this document, so the churn is this document's to pay for. *Falsified by* any
predicate whose name, replaced by its own definition, would change a reader's verdict.

**Two further disciplines the ledger must be built with, and both come from this project's own scars:**

1. **Recomputability.** Every predicate is a pure function of one record, or of a named set of records. P1
   already enforces this with a test that a poisoned stored outcome is ignored in favour of recomputation.
   **This is what makes a figure requotable rather than trusted**, and it is the direct fix for #14524's
   7-of-25 error propagating into a design's motivation.
2. **Firing.** A predicate that has never fired over the archive is marked *unfired*, with its population
   stated. P1 already does this for `ungrounded` — population of one, 40 of 42 records predating the
   mechanism that produces its principal cause, so the honest rate is 1 of 3 and **three is not a
   denominator anyone may quote forward** — and it carries a kill condition. Generalise both.

### 6.3 The Replay — re-execution of a deterministic stage at a different configuration

**Owns:** the arm comparison for every change downstream of `Retrieve`.
**Does NOT own:** anything requiring a new query, and therefore nothing about retrieval itself.

**The finding that makes this the centre of the design:** `admit`'s decision — self-produced, then the
relevance floor, then the per-row ceiling, then the running budget — consumes only **scalars the record
already carries**: `SelfProduced` (recoverable from the cut reason and the type/name predicate),
`Similarity`, and the byte lengths of content and substance. It never needs the text.

Therefore **selecting `SubstanceRatioThreshold`, `BlockOccupancy` and `RelevanceFloor` requires no graph
access, no model call, no new corpus and no live run.** It is arithmetic over 42 archived records, and it is
immune to substrate drift by construction because the substrate is not consulted.

Two modes:

| mode | needs | what it can decide | substrate exposure |
|---|---|---|---|
| **Offline replay** | the record's scalars only | which rows are admitted, at what charge, at any dial setting; the full disposition delta between two settings, per row | **none** |
| **Hash-gated replay** | re-fetch each candidate body by id; keep the row only if its live hash equals the recorded `ContentHash` | what the block's **text** becomes — rendering, form selection, composition, prompt size | per-row, **exactly detected**; a moved row is void, not failed |

**The self-check that makes the replay believable, and it is the whole of #13534 §14's rule
(*prove the instrument fired before believing its zero*):**

> **Replay fidelity: replaying any archived record's admission at that record's own recorded `Limits` must
> reproduce that record's recorded dispositions exactly — every `Included`, every `CutReason`, every
> `RenderedSize`.** If it does not, the replay is wrong and no delta it produces may be quoted.

This is free, it runs over the whole archive, and it is the difference between a replay and a
re-implementation. It also pins the replay to the production function rather than a copy of its logic — the
`payload_seam_test.go` AST guard already establishes that `renderedPayload` is the single site where the
form decision could ever change, so a replay calling that function inherits the guarantee.

### 6.4 The Sweep — `cmd/eval`, apparatus kept, readout replaced

**Owns:** the arm comparison for changes to **retrieval** — the query set, the aperture, fusion, the scope
reserve, the window — which cannot be replayed because they issue queries the archive never issued.
**Does NOT own:** any rate, any headline, any verdict about whether a retrieval was *better*.

**What the apparatus already is, and it is good:** it calls the production `Retrieve` and the production
`Assemble` with the production constants, against the live graph, with **zero model calls** — enforced three
ways: structurally by the dependency-closure grep, by substituting a pinned file where production calls the
model, and by never constructing a `Turn` at all. It produced byte-identical candidate id lists across six
arms. It aborts rather than reporting a partial measurement. It verifies itself on every run via the control
stratum.

**What changes — and it is only the readout:**

| element | fate |
|---|---|
| `Retrieve` / `Assemble` invocation, the abort-on-row-error rule, the corpus and derivation hashes in the header | **unchanged** |
| **The control stratum and its exit code** | **kept, and promoted.** It is the instrument checking itself, not a measurement of the product. `alarmControlAbsent` — *"this sweep verified nothing about itself, so a broken harness would report a plausible number"* — is exactly #13534 §14's rule already implemented. Exit 1 on control failure stays |
| **`retrieved` and `admitted` rates over labelled rows** | **removed as output.** Not demoted, not kept as explanation — **removed**, because a printed rate will be quoted (#14524's own correction is the proof) |
| **The labelled-row contribution to the exit code** | already absent, and stays absent |
| **`required[]`** | **kept, unscored** (§10.2) |
| **The per-row disposition set** | **promoted to the output.** This is what was always being measured; it was being collapsed |
| **`Scored()`'s row-dropping rule** | **removed with the rates it gated.** One unresolved node dropping a whole row from both denominators is a rate-protection mechanism; with no rate there is nothing to protect, and an unresolved node becomes a triage trigger instead |
| **`Stale` (recorded hash ≠ live hash)** | **promoted from a flag to a trigger.** A stale label is the graph telling you the expectation was authored against content that has moved — §9 step 5's exit (b) |
| **Arm comparison** | **new: candidate-id hash identity** over the union both arms touched (§8) |

**And one thing that exists, is right, and has no reporter:** `corpus-anchor.json` holds 32 rows as 16
matched pairs — same input, same required set, differing only in anchor class — validated by a contract that
refuses a pair whose arms share a class (*"the comparison holds the anchor fixed and varies nothing"*) or
whose inputs differ (*"a difference between its arms is not attributable to the anchor"*). **`Pair`,
`Anchor` and `AnchorTitle` are read by the validator and by nothing else.** Sweeping it today produces one
blended rate over both arms and exits 1 for want of a control stratum.

That is the thesis in miniature: **the right comparison shape was authored, validated and never given a
readout.** The within-pair delta — same input, same required set, one variable, both arms at one graph state
in one sweep — is the strongest comparison this project has available, and it is three lines of reporting
away. §12.4 gives it its readout.

### 6.5 The Reading — the adjudicated analysis that replaces the answer key

**Owns:** every claim about meaning. Relevance, sufficiency, correctness, and the triage of a failed
expectation.
**Does NOT own:** any exit code. **A reading never gates anything.** It produces findings, and a finding is
filed, argued with and acted on by people, not by a build.

Its procedure, its three exits and its blinding are §9, which is long enough to be its own section.

### 6.6 The Reference Arm — the objective, and it compares information rather than nodes

**Owns:** the standard processor's result is judged against.
**Does NOT own:** truth, a target, a score, or a node set to agree with.

**#13534 §17.3 gives the mechanism:** the reference is an **isolated agent in this harness** (Claude Code +
DiVoid), handed the same task **in report form** — *given task X, return the relevant memories from DiVoid* —
and its answer is the other arm. The axiom that licenses it: **this harness works acceptably with DiVoid.**
Not that it is right; that it works.

**#13534 §17.6 gives the thing being compared, and it is stronger:**

> **Two results are equivalent when they carry the same substantial information for the task — regardless of
> which nodes carried it.**

> **The reference is a witness, not a standard.** It is evidence that material sufficient for this task exists
> in the graph and is reachable. It is not the shape that material has to take. That distinction is the whole
> of §6.6.9, and it is why an arm can be *sufficient* on a probe the reference failed.

#### 6.6.1 The equivalence is on information, and every set metric is barred

**A run that reaches the same informational state through entirely different nodes is a success.** So:

> **No node-set agreement metric may enter this instrument.** Not overlap, not Jaccard against the
> reference's set, not *how many of the reference's nodes did processor also find*. Each scores **node
> identity**, which is explicitly not the thing that matters, and each reports a success as a near-total
> failure.

This is §4.4's invariant applied at its third level: the substrate is diffuse, so anything measuring its
surface measures noise. **Only the information is stable enough to compare, and comparing information is a
judgement.** The reference's node ids appear in the bundle as evidence for the adjudicator to read; they never
appear in a comparison.

#### 6.6.2 Why report form, and why that is a constraint rather than a detail

Both arms must produce **the same shape of artifact**, for two independent reasons, either sufficient alone:

1. **Blinding is impossible otherwise.** An adjudicator handed a prose answer with citations and a bulleted
   memory report knows instantly which is which, and §9.2's structural blinding becomes decoration.
2. **The KPIs do not survive a shape mismatch.** *Size* compared between a 4 KB prose answer and a 60 KB
   assembled block is a category error; *open questions* compared between an answer and a retrieval report is
   not a comparison at all.

So the comparison probe is a **memory-retrieval-report task**, and that bounds what the comparison covers:
**it tests the memory substrate, not the tool loop and not the planner.** That is exactly #13534 §16.4's
ruling — *memory stays the agenda* — and it is a feature rather than a gap, because the tool loop's behaviour
is already covered mechanically and for free by the ledger.

#### 6.6.3 Two probe classes, answering different questions

| class | arms | judged how | answers |
|---|---|---|---|
| **Report probe** | processor **and** the reference, both fresh, blinded | the adjudicated verdict (§6.6.5) with the KPIs as its texture | *does the memory carry enough substantial information to act on the task* |
| **Task probe** | processor only — there is no reference arm for *"write this file"* | the ledger, plus an unblinded reading | *how does the loop handle what it got, and does it requery* (#14525 Q3/Q4) |

**Neither replaces the other**, and conflating them is precisely how a corpus came to score retrieval and be
quoted about answers.

#### 6.6.4 The KPIs, and they are read semantically or not at all

#13534 §17.4 names three, deliberately loose. **§17.6 fixes how each is read**, and an operational definition
in terms of the reference's nodes re-creates the defect inside the fix:

| KPI | read as | **never** read as |
|---|---|---|
| **topic match** | *does this material address what the task is about* — judged, three values: on topic / partial / **missed entirely** | do the two node sets overlap |
| **size** | **mechanical** — bytes each arm returned, both raw figures, as a ratio | a quality proxy in either direction |
| **open questions** | *what would a competent reader still need in order to act* — judged, counted **each way**, never differenced | a count of the reference's nodes that processor missed |

**`size` is the only mechanical one, and it is not a neutral axis.** §16.1: if the reference answers the same
task from a fraction of the bytes, **that is the finding**, not a footnote. An order of magnitude larger for
the same substantial information is the outcome this project exists to make impossible.

**The structural guard, and this section cannot do without it:**

> **No KPI may be summed, averaged, weighted or combined — with another KPI, or across probes.** There is no
> scalar anywhere in this instrument, because a scalar is a target and a target is a thing to tune toward.

The reference runs a far larger model under **no byte budget** and will "win" on most axes most of the time. A
design that reports that as a score has built a leaderboard measuring the wrong thing — and the leaderboard
would then be optimised. #13534 §17.4: *they exist to detect a complete failure or a large gap, never to
rank.*

#### 6.6.5 The verdict vocabulary — closed, symmetric, and deliberately not a scale

The P1 precedent binds: **a closed verdict set with stated rules is auditable where prose is not.** The
difference here is that the verdict is *judged* rather than computed, so it must additionally **carry its
reasoning**, and the reasoning is the artifact a human spot-checks.

| verdict | means |
|---|---|
**Adjudicated — the judge emits exactly one of these, position-neutral, and the runner de-shuffles it:**

| verdict | means |
|---|---|
| **`equivalent`** | both arms carry the same substantial information for the task. **Different nodes are irrelevant to this verdict** |
| **`processor insufficient`** | the reference carries substantial information processor lacks, and a competent reader could **not** act on processor's material alone |
| **`reference insufficient`** | the converse. **This verdict must be reachable, or the reference is a ground truth in practice whatever the prose says** |
| **`both insufficient`** | neither arm carries enough. The finding is about the **graph**, not about either arm, and it is the most informative outcome the instrument can produce |
| **`processor failed`** | missed the topic entirely, or returned nothing usable. **§17.5's binary bar, and the only verdict that is a result on its own** |
| **`reference failed`** | the converse. **Added by the 2026-09-22 repair, and it is the load-bearing addition** — see below |
| **`both failed`** | neither arm addressed the task |

**Derived — the runner computes these; the judge cannot and must not emit them:**

| label | derived as |
|---|---|
| **`processor sufficient, thinner`** | verdict `equivalent` **and** the mechanical `size` ratio materially favouring processor |
| **`reference sufficient, thinner`** | verdict `equivalent` **and** the ratio reversed — **processor carrying the same substantial information at many times the bytes, which is §16.1's central finding** |

**Why the repair, and why `reference failed` is the load-bearing half.** The set as first ruled was
asymmetric: it could say the reference carried more and could not say the reference **failed**. But
`reference failed` is **exactly the evidence R-11's falsifier depends on** — *the reference is treated as a
ground truth in practice unless it is observed to be wrong* — so the vocabulary could not record the one
thing the design most needs to see. **A falsifier whose evidence has no label is not a falsifier.**

**Why both *thinner* labels are derived rather than judged.** They name an arm, which a blinded judge cannot
do, and *thinner* is a size comparison the judge is deliberately not shown (§6.6.6). Deriving them is
**strictly better than adjudicating them**: the size half becomes exact rather than impressionistic, the
judge stays blind, and §16.1's finding is computed rather than opined.

**The interim rule, until the repair is in the runner:** a bundle reports an unlabelled outcome as the
neutral verdict **plus a flagged note**, and **never rounds it to the nearest label.** Rounding is precisely
what would make R-11 unfalsifiable, because a `reference failed` rounded into `reference insufficient` never
fires as the thing it is.

**Four rules on the vocabulary:**

1. **`equivalent` is the ceiling.** There is no verdict above it and none is added. **An instrument with no
   expressible state better than *equivalent* cannot be optimised past sufficiency** — which is §6.6.9's
   first mechanism and the reason the set is shaped this way.
2. **Every verdict carries its reasoning**, in prose, naming what substantial information each arm did and
   did not carry. A verdict without reasoning is not a verdict; it is an opinion with a label.
3. **No verdict is a number and no verdicts are aggregated.** A 0–10 rating is barred. So is *"processor was
   equivalent on 4 of 6"* as a headline — the six are reported individually, each with its reasoning.
4. **The verdict is a finding, never a gate** (§6.5). It produces no exit code and blocks nothing.

#### 6.6.6 The adjudicator, its blinding, and where the variance actually sits

**§17.6 requires a *"kind of intelligent"* adjudicator** — a model comparing two reports for substantive
sufficiency against the task, not a function comparing two lists. Its judgement **is** the instrument.

**This partially overturns §11 alternative 2**, and the overturn is stated rather than smuggled: revision 1
rejected an LLM judge as primary because it *"puts an unvalidated measurement inside the measurement"*. Under
§17.6 that objection loses its force, because the alternative it preferred — a mechanical comparison — has
been ruled to measure noise. **The judge is now required, not optional.** What survives of the objection is
the validation obligation, and it is discharged differently than by agreement statistics:

| obligation | how it is met |
|---|---|
| **the judge must not know which arm is processor** | **structural blinding** — the adjudicator receives a projection that cannot carry the arm label, shuffle key held by the runner. The same shape `generate_derivations.py` already uses. Blinding by projection, not by care |
| **the judge must not mark its own work** | the adjudicator is a **separate dispatch from the reference arm**, with no shared context. An agent that produced one of the reports may not judge them |
| **the judge must be auditable** | rule 2 of §6.6.5: the verdict carries its reasoning, and **the reasoning is what a human spot-checks** — not the verdict, and not a distribution of verdicts |
| **the judge must be identified** | model, harness and date recorded per adjudication, exactly as §6.6.10 requires of the reference arm |

> **Property B-1 — the adjudicator is never shown the size figures.** One arm works under a byte budget and
> the other under none, so **a byte count identifies the arm on its own** and defeats the blinding without
> anyone naming an arm. `size` stays #13534 §17.4's KPI and is computed by the runner **outside** the
> adjudication, which is also what makes both *thinner* labels derivable (§6.6.5).
>
> This is stated as a property of the instrument rather than as a fact about one prompt, because a fact about
> a prompt is lost the first time the prompt is rewritten. *Falsified by* any adjudication whose input carried
> a byte count, a token count, a rank list, or any other quantity that differs systematically between the
> arms.

**Where the variance sits, and this is the argument that makes n=1 affordable.** The obvious objection to an
LLM reference is that two runs return different memory sets, so a measured gap may be a draw from the
reference's own spread. **That objection only bites a comparison keyed on node identity.** Under
information-sufficiency, two reference runs returning different nodes that carry the same substance are
**equivalent**, and the arms' variance never reaches the verdict.

> **The residual variance is in the adjudicator, not in the arms.** That is a smaller and more tractable
> problem, and it is the one blinding is actually for. It is also why the arms need no repeat: §13.3 states
> what that buys and what it does not.

#### 6.6.7 The free control the reference arm brings with it

`compare.py` already runs a **transcript arm** — the same model, the same task text, posted straight to the
endpoint with no system message, no context block and no recall tool. That arm now does double duty, and the
second job is the more valuable one:

> **A report probe on which the reference arm does not beat the no-memory arm is a probe where memory is not
> the variable.** Discard it or rewrite it; it cannot support a finding about a memory substrate in either
> direction.

That is the reference-arm analogue of the sweep's control stratum — the instrument checking itself before its
output is believed, which is #13534 §14's rule obeyed for free. Under §17.6 it is judged the same way as
everything else: *does the no-memory arm carry the same substantial information*, never *did it return the
same nodes*.

#### 6.6.8 The bar, and the first reportable result

**Parity is not the target and beating the reference is not the target.** The restrictive environment — a
small local model, a 60 KB block, six calls — is a **premise, not a handicap to apologise for.** The question
is whether the memory substrate gets a *constrained* agent far enough to do the job.

So the first reportable result is **binary and cheap**: *does it work at all, or does it fail completely on
some class of task* — the `processor failed` verdict, and nothing else. Quality gradations come after that and
not before.

#### 6.6.9 What stops the reference arm becoming the spec

**This is the risk that would quietly destroy the project's thesis**, and #17.6 does not dissolve it — it
sharpens it, because an information-sufficiency comparison against this harness's output is still a comparison
against this harness. Five mechanisms. **Four are structural; the fifth is an accepted residue and is named as
one.**

1. **The scale saturates at `equivalent`, and nothing above it is expressible.** §6.6.5 rule 1. There is no
   verdict for *closer to the reference than last time*, so there is no state the instrument rewards
   approaching. **An instrument that cannot express "more similar" cannot be optimised toward similarity.**
2. **The question asked is sufficiency for the task, not similarity to the reference.** The adjudicator is
   asked *could a competent reader act on this material*, with the reference present as a **witness** that
   sufficient material exists — not as the answer. `reference insufficient` and `both insufficient` are
   reachable verdicts precisely so that this is true in practice and not only in the prose.
3. **There is no scalar to descend and no node set to converge on.** §6.6.4's guard plus §6.6.1's bar on set
   metrics. A verdict lattice is not a gradient and information equivalence is not a distance.
4. **`size` runs the opposite direction, structurally.** The reference has no byte budget; this product's
   thesis is that smaller and more focused is better (§16.1). **On the one axis where the arms compare
   numerically, the product's goal is to diverge from the reference, not to converge on it.** That is
   anti-alignment built into the instrument rather than a discipline applied to it.
5. **Accepted residue, not closed.** If every probe is one this harness happens to answer well, processor is
   shaped toward this harness's competence profile even with no scalar, no set metric and a saturating scale
   — because the probe set itself was chosen against the reference's strengths. **Two partial bounds, and
   they do not eliminate it:** R-3's *at least one probe the graph answers badly*, and a standing requirement
   that **at least one probe must be one the reference is adjudicated to get wrong** — a reference never
   observed to fail is being treated as a ground truth whatever the document says. **Carried as R-11.**

#### 6.6.10 The reference is unversioned too, and that has to be recorded

`gangolf` is an uncontrolled variable (§12.3) and **so is this harness** — its model, its system prompt and
its tool surface all change without notice. **Every reference run and every adjudication records the harness,
the model identifier and the date**, and a comparison quoting either without them is not requotable. §18 item
7 states what stays unknown until the adjudicator's own agreement with a human has been spot-checked.

---

## 7. Data Model (Conceptual)

Eight entities. One exists; seven are new, and five of those are documents or judgements rather than code.

| entity | owns | lifecycle | where it lives |
|---|---|---|---|
| **Record** | what one run did | written once by the loop, immutable | a `session-log` node in DiVoid, JSON in a fence, human summary as substance and body |
| **Predicate** | one mechanical claim, its definition, its population, whether it has fired, and its kill condition | versioned with the code | the ledger; a predicate that changes definition is a **new** predicate with a new name, never an edited one |
| **Replay delta** | the per-row disposition difference between two configurations over one selector, plus the fidelity check's result | ephemeral; quoted only inside a bundle | bundle |
| **Probe** | a task, a subject, a **written statement of what an answer must be able to do and why the memory is required for it**, and — unscored — the nodes its author believed carry the answer, with the hashes they were authored against. **Carries its class: report or task (§6.6.3)** | versioned; **every edit names the reading that caused it** | repo file; seeded from `scripts/compare_tasks.json`, which already carries exactly these fields under `whyMemoryIsRequired` and `answerNodes` |
| **Reference report** | one reference arm's answer to one report probe, with the harness, model identifier and date that produced it | written once per comparison, **never reused** — §17.3 forbids a stale arm | bundle |
| **Adjudication** | one blinded verdict from §6.6.5's closed set, **with its reasoning**, plus the three KPI readings, plus the adjudicator's own harness/model/date | written once per report probe per comparison; a changed mind is a second adjudication that supersedes | bundle, and filed to DiVoid with the reading |
| **Reading** | one adjudicated analysis of one result: per-row match calls, a sufficiency call, and an exit for every failed prior | written once, never edited; a changed mind is a second reading that supersedes | DiVoid, linked to the record it reads and the probe it exercises |
| **Bundle** | everything one measurement touched: the records, the ledger output, the replay deltas, the arm hash comparison, the measurement window, the cost figures, the model and endpoint, and the readings made against it | written once at the end of a measurement | DiVoid, linked to the change under test |

**Relationships that carry weight:**

- A **Probe** is exercised by many **Records**; a Record belongs to at most one Probe (a real task run belongs
  to none, and most of the archive is real runs — which is correct and is the point).
- A **Reading** references exactly one Record and at most one Probe. A reading of a real run with no probe is
  legitimate and is how #14247 was found.
- A **Bundle** is the unit of quotation. **A figure not in a bundle is not a measurement.**
- The **Probe's prior** (`answerNodes` / `required[]`) has no scoring relationship to anything. It is an
  input to step 5 of a reading and to nothing else.

**What deliberately has no entity: a score.** There is no field anywhere in this model that holds a number
representing answer quality, because there is no procedure that could compute one honestly.

---

## 8. Comparing two arms when the substrate moves

This is the question the brief asks directly, and the answer is that the existing rule does not generalise
and does not need to.

### 8.1 What A/B/A buys, and its own stated limit

`m3-derived-recall.md` §9.1a: take arm A, arm B, then A again; `A ≠ C` **voids the triple rather than failing
the change**. The section then states two limits against itself, both correct:

- **It is one-sided.** `A ≠ C ⟹ dirty` is licensed; `A == C ⟹ clean` is not, because movement added and
  reversed inside the window also yields `A == C`. A single page of the graph showed 14 missing ids across a
  514-id span.
- **It brackets only A's queries.** The derived arm issues 150 distinct query strings across 25 rows; the raw
  arm issues 25; **125 are issued by one arm and never the other.** So an arm-versus-arm use brackets 25 of
  150 neighbourhoods — **16.7 %** — and the unbracketed error direction is **false acceptance**, the
  dangerous one.

### 8.2 Why it cannot be extended to a model-in-the-loop comparison

Worse, not better, and for a structural reason:

1. **The arms' own tool rounds issue queries neither bracket sees.** A supplementary recall is composed by
   the model at run time; the two arms will not agree on them, and a bracket repeating A's *initial* queries
   says nothing about the neighbourhoods B's third recall round read.
2. **The arms write.** Every run files a record into the graph it reads from (C2), so arm B reads a graph arm
   A moved — the failure `compare.py` measured for real: *"task 3 retrieved the records tasks 1 and 2 had
   just written, minutes earlier, on unrelated text."*
3. **A bracket costs a full extra arm.** At 20 s–5 min per run and one endpoint, a third arm is a 50 % cost
   increase on the most expensive instrument for a check that is one-sided and 16.7 % covered.

### 8.3 The replacement: per-candidate hash identity

Every disposition carries `ContentHash`. The corpus's `Required` rows already carry the hash they were
authored against, and `Score` already computes `Stale` by comparing them. **The mechanism exists; it is used
as a footnote on a rate.**

> **Two readings are comparable on the rows whose content hash is identical across them. Rows whose hash
> differs are void — not failed — and are named. The comparison proceeds on what remains, and reports what it
> dropped.**

Compared against A/B/A, on every axis:

| | A/B/A bracket | per-candidate hash identity |
|---|---|---|
| **what it detects** | that *something* moved in A's neighbourhoods | **exactly which rows** moved, in every neighbourhood either arm touched |
| **coverage, arm-vs-arm** | 16.7 % | **100 % of what was read** — a row not read cannot affect a comparison |
| **granularity of the void** | the whole triple | one row |
| **one-sidedness** | `A == C` does not license *clean* | **an identical hash is identity**, not a probe for it |
| **cost** | one extra full arm | **zero** — the hashes are already on the record |
| **works with a model in the loop** | no (§8.2) | **yes** — it reads what the run recorded, whatever queries produced it |

**The residue, stated honestly:** hash identity certifies the rows that were *read*. It cannot see a node
that appeared in the graph between the arms and **should** have been retrieved by B but was not, or was
retrieved by B and not by A because it did not exist yet. That is a **population** change, not a content
change, and no bracket detects it either — A/B/A would report `A ≠ C` only if the new node displaced
something in A's own top-20. The available mitigation is cheap and partial: **compare the candidate id sets,
not only the hashes of the intersection, and treat a set difference as a finding to adjudicate rather than a
void.** A row present to one arm and absent to the other is exactly the *"more fitting nodes now exist"* case
Toni names, and it is a reading, not a fault.

### 8.4 Where A/B/A survives

**As the zero-delta self-check only** — two binaries, same arm, byte-identical query arrays on every row,
complete bracket coverage, and a false rejection as its worst outcome. It is cheap, conservative, and
genuinely fixed there. Keep it for that, and only that. Retire it as the arm-versus-arm mechanism.

### 8.5 The three regimes, as a routing rule

| regime | the change under test sits | mechanism | substrate exposure | cost |
|---|---|---|---|---|
| **R1** | downstream of `Retrieve`, decidable from scalars — admission order, the floor, the ceiling, the form rule, the budget | **offline replay** over the archive | **none** | seconds |
| **R2** | downstream of `Retrieve`, needs content — block text, rendering, prompt composition, payload form | **hash-gated replay**; void the moved rows | per-row, exact | ~1 min |
| **R3** | retrieval itself, or loop behaviour needing a model | **live paired**, back-to-back, compared on candidate-id hash identity over the union | per-row exact, plus the population residue of §8.3 | minutes (sweep) to an hour (probes) |

**The routing rule is the first thing an implementer applies**, and it is what makes the strategy affordable:
most changes this project makes are R1, and R1 costs nothing and cannot be wrong about drift.

---

## 9. The Reading — what replaces a pinned answer key

Toni: *"we can give out expectations, but they can fail and the behavior could still be right → depends on
our analysis of the actual result."* This section makes *our analysis of the actual result* a procedure with
a fixed shape and recorded outputs, so that it is repeatable by someone other than the person who wrote the
expectation, and so that it accumulates.

### 9.1 Five steps, fixed order, each with a recorded output

| # | step | mechanical or judged | output | may it block? |
|---|---|---|---|---|
| **1** | **Validity** | **mechanical** | valid / void, with the reason | **yes** — a void reading is not made |
| **2** | **Behaviour** | **mechanical** | the ledger's verdicts and dispositions for this run | **yes** — as gates, independently |
| **3** | **Match**, per admitted row | **judged** | per row: a real match for the question asked, or not, with one line of reason | **no** |
| **4** | **Sufficiency**, per run | **judged** | sufficient / thin-but-usable / insufficient | **no** |
| **5** | **Expectation triage**, only where the probe's prior and steps 3–4 disagree | **judged** | one of three exits, §9.3 | **no** |

**Step 1 is what stops a reading being made of an unmeasurable run**, and it is entirely mechanical:

- the run's write receipt is `notStored` (it was a measurement and did not pollute), **or** it is a real run
  and is not being used as an arm;
- the record carries a complete `Limits` snapshot;
- replay fidelity holds for this record;
- the run's timestamp does not fall inside another bundle's measurement window;
- the self-produced share is below a stated bound, **or** the reading declares that it is measuring the
  crowding rather than the content.

The last one matters more than it looks. The crowding is **anti-correlated with the memory being useful** —
0, 1, 9, 17 self-produced rows across five runs, taking 85 % of the aperture exactly where the graph holds
least. **A probe set built from well-matched inputs reports that defect as absent.** So a validity condition
that silently excludes crowded runs would systematically hide the worst failure mode; hence the second limb,
which makes measuring it a declared intent rather than an accident.

**Step 3 is per row, not aggregate.** *"Is the result correct for the graph as it is now"* is a question
about the rows that came back, and averaging it destroys the only thing a reader can act on. A row judged *a
real match* and a row judged *plausible but about something else* have different remedies — the first indicts
the budget if it was cut and the prompt if it was admitted; the second indicts the query.

**Step 4 is the one sufficiency call.** Three values, and *thin-but-usable* is a real outcome rather than a
hedge. The mechanical half of this question — insufficiency — is a gate in step 2 (shutout, thin block,
top-rank starvation, accepted insufficiency). **Step 4 can only ever be read after those pass**, because a
run the ledger already proved insufficient does not need a judge.

### 9.2 Where human judgement enters, and where it must not

**Must not:** steps 1 and 2; every gate; the replay; the hash comparison; the cost figures; which rows are
void. These are absolute, automatic, and would be corrupted by a person's opinion about the change.

**Must:** steps 3, 4, 5. Nothing else can make these calls, and pretending otherwise is how `recall@k`
happened.

**Must not, in a different sense — the adjudicator must not know which arm is which.** For arm-versus-arm
readings, blinding is required, and this repo has already built the right shape for it once:
`generate_derivations.py` enforces blindness **structurally**, by projecting every corpus row down to
`{id, input}` before it can reach the model, so `required`, `subject`, `hash` and `why` *cannot* leak. The
reading protocol takes the same shape: **the adjudicator receives a projection of the bundle that structurally
cannot carry the arm label**, with a shuffle key held by the runner. Blinding by projection, not blinding by
care — which is the difference between a property and an intention, and a single operator (C10) needs the
property.

### 9.3 The triage — three exits, and two of them edit the instrument

This is the mechanism that makes *"an expectation that fails is a finding, not automatically a failure"*
operational rather than aspirational.

When the probe's written prior disagrees with steps 3–4, the reading **must** record exactly one of:

| exit | means | what it obliges, in the same act |
|---|---|---|
| **(a) regression** | the system got worse; the prior was and remains right | file it as a defect against the change |
| **(b) the probe is stale** | the prior was authored against content or a graph that has moved | **rewrite the prior**, naming this reading as the cause |
| **(c) better and different** | the graph now holds a better answer than the prior named | **promote the new nodes into the prior**, naming this reading as the cause |

**Exits (b) and (c) are not permitted to be conclusions without an edit.** That is the whole discipline:
under a pinned key, drift is a silent, accumulating wrongness that reports itself as regression — three of 25
required documents appear in none of 600+ fetched rows, which reads as retrieval failure and is at least
partly the corpus disagreeing with the graph. Under this triage, **drift becomes an audited write with a
named cause**, and the rate of (b) and (c) exits is itself the measurement of how fast the instrument is
decaying. If that rate is high, the probe set is the thing to fix. Nobody can currently answer that question
about `corpus.json` at all.

**The trigger that makes this cheap:** `Stale` already computes, per required node, whether the live content
hash differs from the one the label was authored against. **A stale prior is a scheduled exit-(b)
candidate, detectable with no model, no judge and no run.** Promote it from a footnote on a miss line to a
first-class prompt for triage.

### 9.4 Why this is a process rather than a person reading runs

Four properties, each of which the current practice lacks:

1. **A fixed shape**, so two readings of two runs are comparable and a reading made in October can be
   compared with one made today.
2. **Mandatory exits**, so no disagreement is left unresolved and no probe decays silently.
3. **Accumulation** — readings are nodes, linked to the record and the probe, so the twentieth reading can be
   argued against the third. This is also what would eventually validate an automated judge (§11
   alternative 2), at zero extra cost from the first reading onward.
4. **Bounded human time.** The agent produces the bundle; the human reads a blinded projection. Toni's
   involvement is bounded to the point of a claim, not to every run.

### 9.5 Where the reference arm enters the reading

**On a report probe, steps 3 and 4 are not made by a person reading two documents.** They are made by the
adjudicator of §6.6.6 — a model, blinded, producing a verdict from a closed set **with its reasoning** — and
the human's job moves from *making every call* to **spot-checking the reasoning**. That is what makes the
outer loop affordable at the cadence §13.2 prices it at.

The five steps are unchanged in shape and change in who performs them:

| step | task probe | report probe |
|---|---|---|
| **1 Validity** | mechanical, unchanged | mechanical, **plus**: both arms fresh (§17.3), neither served from an archive; the reference arm beat the no-memory arm (§6.6.7); reference and adjudicator identified (§6.6.10) |
| **2 Behaviour** | the ledger | the ledger, on processor's arm only — the reference has no run record |
| **3 Match** | judged per row by a human | **adjudicated**: does this material address what the task is about (§6.6.4), never *do the node sets overlap* |
| **4 Sufficiency** | judged by a human | **adjudicated**: one verdict from §6.6.5, with reasoning |
| **5 Expectation triage** | three exits, §9.3 | three exits, **unchanged** — with the reference's report standing where the probe's written prior stands. A disagreement between processor and the reference is a *finding to adjudicate*, not a defect, because the reference is a witness and not a standard |

**Step 5 is the place the whole design would fail quietly if it were dropped.** Under a ground-truth reading,
*processor disagreed with the reference* is a regression. Under §17.6 it is one of three things, and **two of
them edit something other than processor**: the probe is stale, or the reference is the one that is wrong.
The `reference insufficient` verdict is what makes the second reachable.

---

## 10. What happens to what exists

### 10.1 Nothing is deleted except a rate and a row-gate

| artifact | fate | why |
|---|---|---|
| `cmd/eval` binary, its abort rule, its hashes, its arm identity | **kept** | §6.4 |
| The control stratum and exit 1 | **kept and promoted** | it is the self-check #13534 §14 asks for |
| The two labelled rates, `rateOf`, the rate lines | **removed** | §4.1. A printed rate gets quoted |
| `Scored()`'s whole-row drop on one unresolved node | **removed with the rates** | it exists to protect a denominator that no longer exists |
| `corpus.json` rows | **kept as probes** | `input` + `subject` + `required[].why` is already a probe |
| `required[]` | **kept, unscored** — §10.2 | |
| `Stale` | **promoted to a triage trigger** | §9.3 |
| `corpus-anchor.json`'s 16 matched pairs | **kept, and given the readout it never had** | §12.4 |
| `derivations.json` and its blind/hand provenance split | **kept** | it is a genuine arm axis and its blinding is structural |
| `scripts/compare.py` | **kept, and promoted to the seed of the outer loop** | §10.3 |
| `scripts/smoke.py` | **kept** | it is the only instrument that measures self-poisoning, and C2 is not going away |
| `scripts/step_trace.py` | **kept** | *"the deliverable is visibility, not improvement"* is the right charter for it |

### 10.2 `required[]` — demoted to an unscored prior, and the argument for not deleting it

Toni retired it as *the* question. He did not say the authoring work was worthless, and it is not: each entry
carries `why` — a written statement of why that node is needed for that question — and a hash pinning the
content it was authored against.

**The recommendation is to keep it, unscored, renamed in the reporting to what it actually is:** *the nodes
this probe's author believed carry the answer, as of a date*. It appears in a bundle as one column beside the
actual result, so an adjudicator can see agreement and disagreement and rule on which is right. It is never
counted, never rated, and never touches an exit code.

**The argument, against the obvious objection that this is the key by another name:** the key's defect was
never that someone wrote down an expectation. It was three specific things, and demotion removes all three —
(i) it was *scored*, so disagreement produced a number instead of a question; (ii) it was *aggregated*, so
which row disagreed was invisible; (iii) it had *no repair path*, so drift accumulated as apparent
regression. Under §9.3 an expectation that disagrees gets triaged and the instrument gets edited.

**Deleting it instead is defensible and costs one thing:** #11092 §7 is right that without a prior, an
arm-versus-arm comparison can see that the arms retrieved *differently* and cannot see which retrieved
*better* without a fresh judgement every time. Keeping it unscored buys that direction back for free. This is
Toni's call, D-2 in §17; the recommendation is *demote*, because it is strictly cheaper than delete and
carries no rate.

### 10.3 `compare.py` is already most of the outer loop, and nobody has said so

It runs six probes, each carrying `subject`, `answerNodes` and a multi-paragraph `whyMemoryIsRequired`; it
runs each twice — once through the whole product, once as a bare model call with no context block and no
tools, with the output bound matched across arms and a refusal if it cannot be; it attributes a loss to a
stage; it guards against reading its own prior records two ways; it reports retrieval-slot displacement when
it happens anyway; and **it prints no verdict on purpose**, because *"a script that scored these outputs
would be inventing a measurement. It prints the evidence; a human reads it."*

That is this document's thesis, implemented, in Python, eighteen days ago.

**What it is missing, and it is exactly five things:** the **reference arm** (§6.6); write suppression (so
its own arms stop polluting); blinding (it labels the arms); a recorded window; and a bundle to file. **Give
it those five and it is the outer loop** — there is no case for building a second one in Go.

**Three arms, each answering a different question, and the third is now free:**

| pair | question | source |
|---|---|---|
| `SUBSTRATE` vs `TRANSCRIPT` | *is the harness worth having at all* — does memory beat no memory | #11092 §5 says the fixed-state A/B structurally cannot ask this; `compare.py` already does |
| `SUBSTRATE` vs `REFERENCE` | *does it work, and where is the gap large* | #13534 §17.3, judged per §17.6 |
| `REFERENCE` vs `TRANSCRIPT` | **the control**: is memory the variable on this probe at all | §6.6.7, free — both arms already run |

**The probes need a report-form variant.** The six existing tasks are do-this-and-explain tasks; §6.6.2
requires both arms to produce one shape. That is a rewrite of six task strings, not a new corpus.

---

## 11. Competent alternatives, and why each lost

**1. Grow or repair the pinned corpus.** More rows, refreshed required sets, periodic re-authoring.
**Lost on three counts, and the ruling is only the first.** (i) #13534 §16.3 retires it. (ii) Structurally,
label rot on this substrate is not a sampling problem a larger n fixes — 3 of 25 required documents appear in
none of 600+ fetched rows, two of the three identities a rate depends on expired inside two days, and the
graph moved under a baseline overnight and again inside a single 65-minute session. (iii) Economically, the
maintenance cost is unbounded and falls on one person. **If the answer had to be a pinned corpus, the
argument would have to be that the graph's rate of change is low enough for a key to stay true between
re-authorings — and every measurement on record says the opposite.**

**2. LLM-as-judge, scoring answers end to end.** **~~Lost as the primary instrument~~ — OVERTURNED IN PART
by #13534 §17.6**, and the overturn is recorded rather than edited away.

Revision 1 rejected a model judge because it *"puts an unvalidated measurement inside the measurement"*
(#11092 §2). **That objection only stands if a mechanical alternative exists, and §17.6 ruled that it does
not:** the only stable quantity to compare on a diffuse substrate is the information, and comparing
information is a judgement. **So the judge is required, not optional** — §6.6.6.

| revision 1's concern | status under §17.6 |
|---|---|
| an unvalidated measurement inside the measurement | **converted, not dismissed.** Validation is by **human spot-check of the reasoning** each verdict carries (§6.6.5 rule 2), not by agreement statistics — readings accumulate from the first one, so the statistics stay available later |
| it agrees with whichever answer is more fluent | **real, and only partly mitigated by blinding.** Named as **R-12** rather than answered |
| it competes for the single model endpoint | **withdrawn.** The reference arm and the adjudicator run in *this* harness, not on `gangolf`, so they contend for no local capacity (§13.2) |

**What stays rejected is a *scored* judge.** A 0–10 rating, a similarity score, or any aggregate across probes
is barred by §6.6.5 rules 3 and 4. The judge emits **one verdict from a closed set, with reasoning, per
probe** — the P1 shape applied to a judged quantity rather than a computed one.

**3. Snapshot the graph and measure against a frozen copy.** **Lost on #11092 §4.1's argument, which is
unchanged and is not a cost argument:** holding the memory state constant means holding node bodies **and the
ranking over them** constant. Copy the nodes anywhere and the embedding and similarity ordering are that
store's. **A snapshot does not hold the state constant; it replaces it**, and the arms would be measured
against a retriever this product does not ship. The one legitimate snapshot is a run record, because it is
the real retriever's own output.

**4. Buy confidence with sample size.** **Lost on arithmetic.** One endpoint, 20 s–5 min per run, tasks
barred while a measurement is in flight. n=3 over eleven tasks was already priced at ~1.3 M input tokens and
declined. **The alternative chosen instead is to remove variance rather than average over it:** retrieval is
bit-reproducible at a fixed state (C3), replay is deterministic, and the archive is free — so the only
irreducible variance is the model's, and we decline to make claims that need it averaged away.

**5. Mechanical "evidence use" — did an admitted node's content reach the answer.** **Lost, and this is the
most tempting one.** Any implementation is an overlap threshold nobody can defend; it is trivially satisfied
by a model restating the block; and its output is a number that reads as *the memory helped* while measuring
word reuse. It is `recall@k`'s exact failure mode one level downstream. **What survives is the syntactic
residue: citation** (§4.3) — recorded, and never read as use.

**6. Have the loop score its own run and disclose it in the answer.** **Lost.** #14535 proposed the surface
and it is right about the surface; the mechanism is wrong. Instructing the model to disclose is
compliance-dependent and **was measured at zero**. The loop's own statement about the run is the loop's to
make, which is why P1 puts the verdict in the summary, a WARN and a record field, and not in the prose.

**7. Keep reading runs by hand and change nothing.** **This one deserves to be taken seriously, because it
produced every good finding this project has.** #14247 was found by a human reading a replay who knew there
is no repo tool. #14524's own wrong figures were caught by a brief that said *identify that set yourself
rather than taking the count on faith*. **It loses as a strategy for two reasons and neither is quality:** it
does not scale past one person, and it cannot see a twelve-day flat line, because nobody re-reads what
already looked fine. **It is not rejected — it is promoted and given a shape.** §9 *is* hand-reading, made
repeatable, blinded, bounded and recorded — and §6.6.6 moves the routine half of it to a model so the human
reads reasoning rather than raw pairs.

**8. Score processor against the reference — node overlap, Jaccard, "how many of the reference's nodes did it
also find".** **Lost, and it is the most tempting proposal in this document**, because it looks exactly like
the comparison §17.3 asks for, it is mechanical, and it is free. **#13534 §17.6 rules it out directly:** it
scores **node identity**, and a run that reaches the same informational state through entirely different nodes
is a **success** that a set metric reports as near-total failure. It is §4.4's invariant at its third level,
and it is the one an implementer will re-invent under deadline. **The reference's node ids belong in the
bundle as evidence for the adjudicator to read; they never belong in a comparison.**

---

## 12. Cross-Cutting Concerns

### 12.1 Isolation — the measurement must not write, and must not read what a measurement wrote

Two halves, and only one is solvable.

**Write suppression is solved and designed and has never been built.** #11092 §4.2's decorating graph port:
the runner supplies a port whose write-back files nothing and returns the existing `notStored` receipt, and
`internal/loop` does not change — no flag, no second code path through the turn, no addition to a shipped
closed set. **This is the gate on every model-in-the-loop measurement**, it touches no shipped file, and
#11071 has been open since 2026-09-03.

> **The consequence for which surface a measured arm runs on, stated here because it is easy to deduce
> backwards and get wrong.** `notStored` is a **failure outcome, not a mode**, and `Turn.Run` writes
> unconditionally — so a run through the **container's HTTP surface can never satisfy this gate**, and every
> probe after the first would read a store the earlier probes moved. **A measured arm therefore runs on a
> measuring runner that constructs the turn with the decorator, not on the container.** Full statement,
> including what that bounds out and the parity check it earns: **#14571 §10.1–10.2**.
>
> **And the decorator is not enough on its own:** §12.10 rules that the fill port writes past it, widens the
> constraint from *no record* to **no graph mutation**, and puts the falsifier at the transport where it
> catches a port nobody has written yet.
>
> **What it bounds out, in one line:** a measured comparison says nothing about whether the **shipped
> container** answers the same way — boot, configuration, the HTTP path, the workspace mount and the drain
> are all outside it, and that is where three of this project's real defects have lived.

**Read isolation is not fully solvable and must not be pretended away.** Prior *real* runs are in the graph
and belong there — they are memory. What can be done:

- measurement-produced records never enter (write suppression);
- every record already carries `SelfProduced` per candidate and the sweep already counts it, so the crowding
  is **measured per run** and is a validity input (§9.1) rather than a silent confound;
- `smoke.py` already measures the two-turn self-poisoning case and its default input is chosen to match no
  corpus row, so a smoke run cannot poison a later sweep. Keep that discipline as a property of probe text.

### 12.2 The measurement window

One graph, one endpoint, one operator. A window is an **exclusive, declared, recorded interval** — not a lock
service, which would be a mechanism for a problem a protocol solves at this scale.

Recorded in the bundle: start, end, what ran, which endpoint and model. Two mechanical checks follow, both
computable from record timestamps already in the graph:

- **no two bundles' windows overlap**;
- **no run record that is not part of a bundle carries a timestamp inside a bundle's window** — which is
  #13592's *do not run tasks while a measurement is in flight*, converted from a discipline into a checkable
  property of the archive.

### 12.3 The model endpoint is the largest uncontrolled confound, and it must be recorded rather than claimed

C5 is not a footnote. The same model, on the same prompt size, answered in **87.9 s** and then failed at a
**302 s** wall two hours later with nothing in the product changed, because it was serving from system RAM
rather than VRAM. **No instrument here can control that**, and the honest response is to record enough to
detect it:

- every bundle records the provider, endpoint, model id and the per-call usage array (all already on the
  record);
- a bundle whose two arms differ materially in per-call latency distribution is **suspect and says so**.

**This is detection, not control, and the design does not claim more.** It is also an argument for the
routing rule in §8.5: R1 and R2 never touch the endpoint, so most changes are measured without ever exposing
themselves to this confound.

### 12.4 Within-pair comparison — the strongest comparison available, three lines of reporting away

`corpus-anchor.json`'s contract already guarantees the hard part: same input, same required set, differing in
exactly one variable, both arms swept **in one sweep at one graph state**. That removes the substrate from the
comparison more completely than any bracket can, because there is no window between the arms at all.

Give it its readout: **report the per-pair disposition delta**, not a blended rate — which rows the `place`
arm admitted that the `work` arm did not and conversely, with ranks and similarities. Then the pattern is a
generalisation worth stating: **where a change can be expressed as a variable inside a matched pair, prefer
that to running two sweeps.** It is the cheapest and most attributable comparison this project can make and
it has been sitting unrun since it was authored.

### 12.5 Quotation discipline — the direct fix for the 7-of-25 error

Every figure in a report or a PR body names three things: **the predicate, the selector, and when it was
computed.** A figure that cannot be recomputed from those three is not quotable, and a figure quoted forward
from another document is quoted with its source's three, never re-asserted as new.

This is not ceremony. #14524's §7 carried two wrong figures, in good faith, from a careful investigation;
both were quoted forward into a design's motivation and into a conversation; and eleven plausible readings of
the claim were later tested and **none yields seven**. The cost of that was a design phase argued on a defect
rate six times its true value. **Recomputability is the property that makes quoting safe**, and §6.2 builds it
in at the predicate level for exactly this reason.

### 12.6 Provenance on derived artifacts

Not a run concern, and it belongs here because its absence made a whole population unauditable. #14281: *"Who
wrote them is not answerable from the graph"* — DiVoid has no substance provenance field, so the 194
substances cannot be attributed to a qualified model, and R1's falsifier (*any block containing a
substance-form candidate before F-1 passes*) is unevaluable. **The property is: no derived artifact without
provenance** — which model, which prompt version, when. It converts an unanswerable question into a query and
it is the precondition for §14.3.

### 12.7 Error handling and the meaning of a void

Three outcomes, kept distinct, because collapsing them is how instruments lie:

| outcome | means | what follows |
|---|---|---|
| **fail** | a gate's negative condition is met | the change is wrong, or the instrument is; investigate |
| **void** | the comparison could not be made — a moved hash, an overlapping window, a broken replay fidelity | **re-take. Never report as a result.** A void rate that rises is itself a finding |
| **unfired** | the predicate's population is empty or too small | state the population; do not report a rate over it |

### 12.8 Freshness, and what it costs the replay

**#13534 §17.3: both arms fresh, against the current graph. No arm may be served from an archived record.**
That is a hard constraint on §6.3 and it changes what the replay is for:

| replay may | replay may not |
|---|---|
| **enumerate** what a dial value does to block composition, over the archive | **select** a dial value |
| **collapse a continuous range into equivalence classes** of distinct block composition — on #14281's curve, six settings produced three distinct outcomes | stand in for a comparison, at either arm |
| **falsify itself** (replay fidelity, §6.3) and falsify a change as **inert** (V-5) | assert that a configuration is better |

**Why the constraint is right and not merely imposed.** A record is a snapshot of a graph state that has
passed. Comparing today's configuration against it compares a change *and* the substrate's drift, with no way
to separate them — and the hash-identity mechanism of §8 detects that drift precisely because the drift is
real. **Replay is exact about what a dial does to rows it has already seen; it is silent about what the graph
would return now.** Only a fresh run answers the second, and the second is what a comparison is about.

**What that buys, stated as the size of the saving rather than as a claim.** A dial with a continuous range
cannot be explored live at 20 s–5 min per run. Reduced by replay to three or four **distinct** block
compositions, it can. **The replay is the search-space narrower; the fresh paired comparison is the
selector.** That is the whole of its surviving role, and it is smaller than revision 1 claimed.

### 12.9 The store records its own measurement apparatus, and that is permanent

**Found while selecting the probe set (#14571 §6), and it generalises well past that choice.**

Two probes were dropped from the carry-over because they ask about measurement — *where does a design go*,
*the rate went up* — and **the store now contains #14561, which is about exactly that.** Running them would
retrieve this document and measure the instrument's own footprint rather than the substrate.

> **The contamination arrived the moment this design was filed, it grows with every reading and bundle filed
> after it, and nothing removes it — because the material is genuinely relevant.** A recall for *how do we
> know a change helped* **should** return this document. That is the store working.

**It is the semantic twin of C2, one level up, and the two have opposite remedies:**

| | C2 — the product crowds its own aperture | C13 — the project crowds its own probe space |
|---|---|---|
| what pollutes | run records the loop wrote | designs, rulings, assessments, readings and bundles **we** wrote |
| is it relevant to the query? | **no** — it ranks on surface resemblance to the input | **yes** — it is the best answer to the question asked |
| remedy | mechanical: a flag on the row, and it is already cut | **none available**, and none should be wanted |

**Three consequences, and the third is the uncomfortable one:**

1. **No probe may be about measurement, retrieval quality, or this instrument.** Stated as an invariant on
   the probe set at #14571 §2, not left to the judgement of whoever adds the seventh probe.
2. **The ledger's archive inherits the same shape from the other direction** (R-4): measurement runs write
   records, so the census drifts toward describing the instrument. Write suppression (§12.1) closes that half;
   nothing closes this one.
3. **Improvement on a measurement-shaped probe would look like regression.** The more carefully this project
   writes about how it measures, the more of a measurement probe's aperture its own documents take — so a
   probe of that shape gets *worse* as the work gets *better*. **A metric that moves against the thing it
   measures is not a weak metric, it is an inverted one**, and the only safe response is to not have one.

**Why this is filed as a constraint rather than a risk.** A risk has a mitigation and a falsifier. This has
neither: it is a property of building a memory system whose memory contains the design of the memory system,
and it will be as true in a year as it is now. **The useful response is to recognise the shape on sight** —
any instrument whose subject matter the store documents is measuring its own footprint, and that is a reason
to choose a different probe, never a reason to prune the store.

---

### 12.10 RULING — no graph **mutation**, and the decorator goes on every mutating seam

**Raised as #14580**, routed rather than picked by the implementer, and correctly so: three shapes competed
and they are genuinely different.

#### The defect

The suppressing decorator wraps the **turn's** graph seam. The **fill port** is built from the raw client and
writes `SetSubstance` straight past it — and that is the port's **normal success path**, not an edge.

**It lands in the worst possible place.** A substance written during arm A changes, for arm B on the same
candidate: `SubstanceAvailable`, the form rule's choice, `RenderedSize`, admission, and the `Fills` outcome.
**Those are exactly the quantities phase 4 varies.** It is #11071's arm-A-pollutes-arm-B artifact arriving
through the fill channel instead of the record channel.

#### Ruling 1 — the constraint is widened, and this is the half that outlives the fix

> **#11071 as written:** *no **record** reaches the graph before every arm of a comparison has run.*
> **As it now binds:** **no graph mutation of any kind may originate from a measured run — record, substance,
> link, or anything a future port adds — from the opening of a measurement window to its close.**

**This is a widening, not a clarification.** #11071 was written when `WriteRun` was the only write in the
tree. The fill is the second. **There will be a third**, and a constraint phrased in terms of one noun will
miss it exactly as this one did.

#### Ruling 2 — shape 1: a decorating `condense.GraphPort` that suppresses `SetSubstance`

**The fill keeps running.** Four reasons, and the first is the one that decides it.

1. **It is the only shape that generalises.** The defect is not *the fill writes*; it is *a port built outside
   the decorator writes*. Shape 2 must be re-litigated for every future port, and each time the cheap answer
   is *turn it off* — which erodes the measured configuration until it is no longer the product. Shape 1 is
   **one rule applied N times**, and it is the rule #11071 already made for the first seam.
2. **Shape 2 makes the measurement's fidelity rest on a claim about the code that must be re-verified on
   every change** — *"generation does not affect the block"*. That is precisely the kind of statement this
   project has a §12.5 discipline for, because it goes stale and gets quoted forward. **Shape 1 does not need
   the claim to be true.**
3. **The discarded model call costs the measurement nothing in block terms.** The fill cannot help the run
   that pays for it — the block is built before the fill runs and judgement uses that block. So discarding the
   output changes nothing the comparison reads. What it costs is **wall time on the contended endpoint**,
   which is a *scheduling* cost. **A validity cost is not tradeable against a scheduling cost**, and shape 2
   trades exactly that way.
4. **It is the only shape under which `Fills` can appear in a measured record at all.** §6.2's
   `mechanism observation` property **fails today on `Fills`** — zero records in 42, for a mechanism merged
   2026-09-17. Shape 2 guarantees the instrument keeps that failure inside itself, which is a strange thing
   for an instrument to guarantee.

**Cost, stated rather than waved at:** `MaxFills = 2` bounds it to two condense calls per run, and **the
bundle records the fills spent**, so the cost is visible rather than surprising. If it proves prohibitive the
answer is to narrow the probe set, never to drop the decorator.

#### Shape 2, rejected — and one correction to the argument made for it

**Rejected** on rulings 1–2 above. But #14575 §2's argument *for* needing the fill on — *"with fill off,
candidates that would have been condensed render as content or get cut — a different block"* — **is not
right as stated for a single turn**, and the correction matters because the ruling should not rest on it:

- **Within one turn the fill cannot change its own initial block.** The block is assembled, *then* the fill
  runs, *then* judgement uses the already-assembled block. The form rule reads `SubstanceAvailable` off the
  **retrieved candidate**, which comes from the graph, not from the fill port.
- **So §2's claim is true across runs and not within one** — production's steady state has substances the
  fill wrote on *earlier* runs, and a measured arm reads whatever the graph holds either way.
- **One intra-run channel does exist and must be checked before the guard is written:** a **supplementary
  recall round** occurs after the fill, so a later round could re-retrieve a node the fill just wrote.

**Either answer strengthens shape 1.** If the supplementary channel is real, shape 2 is unsound outright. If
it is not, shape 2 is merely fragile for reason 2. **Verify it; do not inherit it from this paragraph.**

#### Shape 3, rejected — relocated rather than dismissed

Accepting the writes fails §17.3 directly: a graph arm A mutated is not the graph arm B was meant to read,
and the mutation is on the measured quantities. **But the thing shape 3 wants is real** — *measure the product
as it actually behaves, writes included.* That is a **longitudinal** measurement on the **container** surface
across many runs, and it **cannot be run as an A/B at all**, because the arms contaminate each other by
construction. Same disposition as §10.2's parity check: a different instrument, filed separately, not folded
into a window whose premise it violates.

#### Ruling 3 — the mechanism is per-seam; the **guard** is at the transport

**Two layers, and they are not redundant:**

| layer | what it is | why not the other one's job |
|---|---|---|
| **per-seam decorator** — one per mutating port | **the mechanism** | each port gets **its own modelled not-written outcome** (`notStored` for the record seam, the fill's own for substance). A port whose write is refused at the transport instead sees an **error**, and the loop's error handling then behaves differently — which changes the measured behaviour |
| **transport-level assertion: zero non-GET requests** | **the falsifier** | it needs nobody to enumerate the ports. **This is what catches the third mutating port, written by someone who never reads this document** |

> **The guard must be written in the fill-ON configuration.** A measured run against a stub graph with a
> condense model configured and a candidate over the fill floor cut for want of room, asserting **zero
> non-GET requests**. The present assertion is written where `PROCESSOR_CONDENSE_MODEL_URL` is never set —
> **the one configuration in which the hole is closed** — and is structurally incapable of seeing this.

---

### 12.11 An instrument whose guard cannot fail is not an instrument — the positive control

**Three instances, one day, three different shapes** — which is what promotes it from a review note to a
property:

| instance | the shape |
|---|---|
| the ledger's self-satisfying guard | the assertion is true of the code **and** of its absence |
| the ordering guard's write-event marker | **every** assertion about it is *the count is zero*, so deleting the marker from the fake leaves the suite green |
| the measure suite's zero-non-GET assertion | written in the **one configuration where the hole is closed** (§12.10) |

> **V-13 — every zero-valued assertion must be shown capable of being non-zero.** Remove the thing it counts,
> watch it go red, put it back. **One mutation, run once, recorded.** An assertion of the form *the count is
> zero* is satisfied identically by a working guard and by an absent one, and nothing downstream can tell
> which it got.

**Why this belongs in the measurement strategy and not only in #8385's guard discipline.** There, a test is
scaffolding around the product, and an untestable test is a weak test. **Here the instruments are the
product** — so a guard that cannot fail is not a weak test, it is a **false measurement**, and it reports a
clean zero forever with nothing to distinguish it from a true one.

**This document already had the rule at two altitudes and did not generalise it**, which is why the third
instance got through: §6.2's firing discipline (*a predicate that has never fired is marked unfired, with its
population stated*) and M3's instruction (*a property that does not fail on merge day is not the property*)
are **the same rule**, stated twice about predicates and never about the guards under them. V-13 states it
once, for both.

**And the two findings of §12.10 and this section are one finding seen twice.** The fill's write was
invisible **because** the guard that should have caught it was written where it cannot fire. **Fix the guard
and it catches the third mutating port by itself** — which is the whole argument for putting the falsifier at
the transport rather than in an enumeration somebody has to maintain.

### 12.12 Falsifier register

**Distributed through the document and collected here so a reference resolves.** Each is a property of the
instrument, not a pattern to match.

| # | property | falsified by |
|---|---|---|
| **V-1** | **Replay fidelity.** Replaying a record's admission at that record's own recorded `Limits` reproduces its dispositions exactly | any archived record whose replay disagrees with itself |
| **V-2** | **Recomputability.** Every ledger predicate is a pure function of one record, or of a named selector | a predicate needing the graph, the clock, or an unnamed set |
| **V-3** | **Dial exercise.** Every dial in `Limits` has taken ≥2 distinct values across the selector, or is declared unexercised | *fails today* on `SubstanceRatioThreshold` and `BlockOccupancy` |
| **V-4** | **Mechanism observation.** Every record field a merged mechanism can write is non-empty in ≥1 record | *fails today* on `Fills` |
| **V-5** | **Non-null delta.** A change under test produces a disposition delta over the archive, or is declared **inert** on it | shipping a change whose replay delta was never computed |
| **V-6** | **Gate asymmetry.** No mechanical predicate asserts a positive about content quality (§4.1) | any gate whose green is quoted as evidence an answer was good |
| **V-7** | **Measurement isolation.** No graph mutation originates from a measured run — record, substance, or anything a future port adds (§12.10) | any non-GET request issued inside a measurement window |
| **V-8** | **Window exclusivity.** No two bundles' windows overlap, and no unbundled run's timestamp falls inside one | any overlap, computable from record timestamps |
| **V-9** | **Expectation exit.** No reading carries a failed expectation without one of §9.3's three exits | any reading recorded without an exit |
| **V-10** | **Probe provenance.** Every probe edit names the reading that caused it | an unattributed probe diff |
| **V-11** | **Quotation.** Every figure names its predicate, its selector and when it was computed (§12.5) | a bare number — the 7-of-25 failure |
| **V-12** | **Blinding.** No arm-versus-arm reading was made by a reader who knew which arm was which | an adjudication recorded without a shuffle key, or whose input carried a byte count (§6.6.6 B-1) |
| **V-13** | **Positive control.** Every zero-valued assertion has been shown capable of being non-zero (§12.11) | a guard whose counted thing can be deleted with the suite still green |

---

## 13. Cost, and who runs it

### 13.1 The affordability invariant

> **No routine instrument may require a model call.** Only the claim-time tier pays, and it pays in
> minutes-per-claim rather than runs-per-conclusion.

This is what makes the strategy viable at one endpoint, and it is achievable because §8.5's routing rule puts
most changes in R1.

### 13.2 Per instrument

| instrument | touches | marginal cost | model calls | who | cadence |
|---|---|---|---|---|---|
| **Ledger** | the archive (42 records) | seconds | **0** | agent, unattended | every change; every merge |
| **Offline replay** (R1) | the same records, scalars only | seconds | **0** | agent, unattended | any admission / floor / ceiling / form / budget change |
| **Hash-gated replay** (R2) | records + ≤20 node reads each | ~1 min over the archive | **0** | agent, unattended | any change to block text or rendering |
| **Live sweep** (R3, retrieval) | 25 rows × ≤6 queries, production `Retrieve` + `Assemble` | minutes; ~750 graph reads per arm | **0** | agent; needs the quiet window | retrieval changes only |
| **Processor arm** (R3, outcome) | 6 report probes × the dial options under test | **20 s – 5 min per run**, serialised on `gangolf`. 6 probes × 3 options ⇒ 18 runs ⇒ **6–90 min wall** | 18–45 | agent; needs the quiet window | **only at a claim** |
| **Reference arm** (R3) | 6 report probes, one isolated agent dispatch each | agent dispatches **in this harness** — **no `gangolf` capacity at all**, so it does not contend with the processor arm for the endpoint | 0 local | agent; reads the graph, so the window still applies | **only at a claim** |
| **Adjudication** | 6 blinded pairs | one dispatch per pair, separate from the reference arm | 0 local | agent, blinded | **only at a claim** |
| **Reading** | one bundle | **spot-check of 6 reasonings**, not 6 raw pairs | — | Toni | **only at a claim** |

**Three cost facts worth stating separately, because each changes a decision:**

1. **The load-bearing number is still the top two rows.** The ledger and the offline replay cost seconds, no
   model, no graph and no quiet window — and they are what turns a continuous dial into three options the
   expensive tier can actually run (§12.8).
2. **The two arms do not contend.** Processor runs on `gangolf`; the reference and the adjudicator run in
   this harness. The window (§12.2) applies to both because both read the graph, but the **endpoint** is not
   the bottleneck it is for a same-endpoint A/B.
3. **The human cost fell.** Revision 1 priced 20–60 minutes of Toni reading six blinded pairs. Under §6.6.6
   he reads **six verdicts and their reasoning**, and spot-checks the reasoning rather than re-deriving the
   verdict. That is the difference between the outer loop being affordable per claim and being affordable
   only occasionally.

### 13.3 n, and the claim we will not make

n=1 as the routine gate; n=3 as the ceiling we will ever pay for a claim; **and no statistical claim about
answer quality at any n.** #11092 §5.1's ruling stands: with the state fixed and retrieval deterministic, the
model is the only variance source, so n=1 answers *did this break something obvious* — which is what a smoke
test is — and reporting it as *"arm A is better"* is a claim n=1 cannot support.

**The obvious objection to an LLM reference, and why §17.6 dissolves most of it.** A reference agent is
stochastic: two runs on one probe return different memory sets, so a measured gap might be a draw from the
reference's own spread. **That objection is entirely a function of comparing node identity.** Under
information-sufficiency, two reference runs returning **different nodes carrying the same substance are
`equivalent`**, and the arms' variance never reaches the verdict.

> **The variance moves from the arms to the adjudicator.** That is a smaller and more tractable problem, it is
> the one blinding is actually for, and it is the reason the arms need no repeat. **This is what makes n=1
> affordable, and it arrived from the ruling rather than from a concession.**

**What that does *not* buy, stated plainly.** A reference spread baseline was going to be a precondition for
any gap claim; under §17.6 it is no longer needed for the **verdict**, and it is still the only way to know
how often the reference is *itself* insufficient on a probe it should handle. That is now a question about
**probe quality** rather than about instrument noise, and §6.6.7's control answers most of it for free.

**The adjudicator's own reliability is the residual, and it is unmeasured.** §18 item 7. The available and
cheap mitigation is the one already in the design: the verdict carries its reasoning, a human spot-checks the
reasoning, and a disagreement between the spot-check and the verdict is recorded as a reading in its own
right. **Twenty of those make the judge's agreement measurable; nothing before them does.**

What we do claim instead, and can support: **attributable statements about mechanism** (this dial changes
these rows, here, by this much — from a deterministic replay), and **evidence-backed findings about answers**
(this run's answer did not draw on the memory, and here is the block it was given). Neither needs a p-value
and both change what to do next, which is #10926 §1's test: *would this instrument change anyone's mind?*

---

## 14. The record test — would this have caught what we already missed

The brief's own standard: *an approach that would not have surfaced these is not the approach.* Four cases,
answered honestly, including the one where the answer is no.

### 14.1 Retrieval flat at 0.478 / 0.391 for twelve days while five mechanisms merged

**Caught, three separate ways, and the third is the general one.**

1. **The rate was never the thing that was flat.** The same sweep at candidate level measured 169 → 183–187
   admitted rows and traced `r01`'s six-row loss to its exact mechanism: rank 1 charging 2,565 B instead of
   9,037 B frees exactly enough headroom for a 48,678 B rank-2 row with no substance of its own to consume
   the entire remaining budget. **Removing the rate and promoting the dispositions surfaces that
   immediately**, because the delta is per-row and cannot be flat unless nothing moved.
2. **`dial exercise` fires today.** `SubstanceRatioThreshold` and `BlockOccupancy` take exactly one value
   across all 42 records. That is a mechanical property over data already held, and it is precisely the
   statement *this merged mechanism has never been exercised*.
3. **`mechanism observation` fires today.** `Fills` appears in zero records. The on-demand fill merged
   2026-09-17 and **has never appeared in a recorded run**.

Points 2 and 3 are the general answer: the twelve-day pattern is not five separate oversights, it is one
missing property — **a merged mechanism that has never been observed in a run is a defect of the merge, not a
nuance of the roadmap** — and it is checkable in seconds with no model, no graph and no judgement.

### 14.2 A run claiming a file write it never made

**Caught, and correctly framed.** P1's `Outcome` already records `acted` per tool from error-free rounds;
#14525 tier 1 crosses the claim against it. #14247 is `ungrounded` **and** `truncated` — a pair that says more
than either word alone.

**The framing matters as much as the catch, and #13065's correction binds:** this is a **claim-action
divergence flag for adjudication**, not a truthfulness verdict. The same investigation that found #14247 also
produced *"it asserts it created a repository it has no tool to create"* — which was withdrawn, because
*repository* means three different things in three task domains and the reading under which the claim was
false was imported by the reader. **A mechanical flag routed to a reading is right; a mechanical flag routed
to a verdict would have shipped that error as a metric.** That is the seam of §4 doing its job.

### 14.3 The substance population failing 6 of 38 on fidelity, with a negation flip

**Not caught. Not by any instrument in this document, and saying so is the point.**

The defect is in a **stored artifact**, not in a run. A substance whose polarity is inverted relative to its
source — licensing exactly the comments the content forbids — produces a perfectly well-formed run: rows
admitted, block assembled, answer delivered, `delivered` verdict, every gate green. **The run-level ledger is
structurally blind to it**, and would have been at any threshold.

What sees it is a different instrument, and it has three parts:

1. **Provenance** (§12.6). Without it, 194 substances cannot even be stratified by who wrote them, which is
   why the population was assumed to be 67 and was 194 — *"every strata median used to argue a threshold was
   computed over a different set than exists."*
2. **A sampled artifact audit, adjudicated** — source read in full against substance, against a stated
   criterion. This is what #14478 actually was, and its result (6 failures in 38) is the reason a threshold
   cannot be argued from medians yet.
3. **A fidelity gate before a population is trusted, not after it is consumed.** The substance path's own
   gate F-1 is still open, and its falsifier is *"any fill at all, before F-1 passes."*

**The honest consequence for ordering:** measuring block composition by replay tells you which rows a dial
admits. It does **not** tell you whether the substance in those rows says what its source said. **Those are
two instruments and the second is a precondition for trusting the first's output in production** — not for
running it, which is why the dial selection in §15 phase 1 is still worth doing now.

### 14.4 Two figures quoted forward into a design's motivation

**Caught, by §12.5 and §6.2's recomputability.** `stopReason: "answered"` on 7 of 25 runs producing nothing
was 1 of 42, and eleven plausible readings were tested before that was established. Under this design the
figure would have been a named predicate over a named selector, recomputable in seconds by anyone reading the
document — which is exactly how F-1 caught it, by being told to identify the set itself rather than take the
count on faith. **The property generalises that brief into a rule.**

---

## 15. Migration / Rollout

Six phases. Each ships alone, each is useful alone, and the two that unblock the deadlock are first.

| # | phase | what it delivers | gates on | cost to build |
|---|---|---|---|---|
| **0** | **The ledger over the archive** | the requery dispositions (Q4 — nothing computes them today), claim-action divergence, cost per admitted row, and the two archive-level properties `dial exercise` and `mechanism observation`. **Falsifiable before any code is written**, by computing it over the 42 records — which is exactly how P1's F-1 was run, and it moved that specification | nothing. `internal/runbackfill` already decodes records | small |
| **1** | **The offline replay + replay fidelity** | **narrows `SubstanceRatioThreshold`, `BlockOccupancy` and `RelevanceFloor` to a handful of distinct block compositions**, offline, over 42 records, with no graph and no model. **It does not select among them** — §12.8. What it delivers is the thing that makes phase 4's live comparison affordable at all | phase 0's record decoding | small |
| **2** | **The sweep re-readout** | per-row dispositions as the output; rates and `Scored()` removed; control kept; arm comparison by candidate-id hash identity; **and the matched-pair readout `corpus-anchor.json` has been waiting for** (§12.4) | nothing; independent of 0 and 1 | small |
| **3** | **The write-suppressing port + the window** | the precondition for every model-in-the-loop measurement; #11071, designed 2026-09-03, never built; touches no shipped file | nothing | small |
| **4** | **The outer loop** — `compare.py` plus the **reference arm**, the **adjudicator**, blinding by projection, the window, and the bundle | report probes run fresh on both arms, a blinded verdict with reasoning per probe, a filed bundle. **This is where a dial is actually selected** | 3 | medium |
| **5** | **Judge assistance** — *trigger-gated, not scheduled* | a model pre-formats a reading for a human to correct | **≥20 recorded readings** to validate against, and a measured quiet-window cost | deferred |

**Why this order.** Phases 0–2 are all *seconds, no model, no graph-write, unattended* and together they
convert every R1 and R2 change in the project from unmeasurable to measured. Phase 3 is small, old, designed
and is the hard gate on everything expensive.

**The deadlock claim, restated at its true size after §17.3.** Revision 1 said phase 1 discharges the ordering
deadlock of §1. **It does not, and the correction matters because somebody would otherwise ship a dial on
it.** Phase 1 turns a continuous dial into three or four distinct options; **phase 4 chooses between them,
fresh**. So the deadlock is discharged by **1 + 4 together**, and phase 1's honest contribution is that it
makes phase 4 cheap enough to run — which is a real contribution and a smaller one.

**The practical consequence for sequencing:** phase 4 is now on the critical path for #13534 §16.5 rank 3,
where revision 1 had it as the last and most optional phase. It is still the only phase that costs real time,
and by the time it runs, four of its five inputs already exist.

**What does not gate on any of this:** #13534 §16.5's ranks 1 and 2 — the loop noticing itself and the
prompt/tool framing. They are cheap and direct and should not wait behind an instrument.

---

## 16. Risks & Mitigations

| # | risk | mitigation | falsifier |
|---|---|---|---|
| **R-1** | **The ledger becomes a ritual** — predicates that always pass, green forever, nobody reads it | every predicate declares its population and whether it has fired; unfired predicates carry a kill condition, as P1's `ungrounded` already does at a population of one | any predicate reported as passing without a stated population, or carried for more than its kill condition's window without firing |
| **R-2** | **A gate is written that asserts a positive about quality** | §4.1 is a hard rule, not a preference | any gate whose green is quoted as evidence that an answer was good |
| **R-3** | **The probe set drifts toward what the system does well** (#11092 A6) | probes are versioned; every edit names the reading that caused it; **and at least one probe must be one the graph answers badly** — the crowding defect is anti-correlated with the memory being useful, so a well-matched probe set reports it as absent | the probe set changes in the same commit as a configuration change, or no probe in the set produces a shutout |
| **R-4** | **The archive becomes a census of measurement traffic** rather than of real work | write suppression (phase 3); the window-overlap property; bundles name their runs | any archived record whose write receipt is not `notStored` appears inside a bundle's window |
| **R-5** | **The replay diverges from production** and reports deltas the product would not produce | **replay fidelity** (§6.3): replay at the record's own recorded `Limits` reproduces its dispositions exactly, over the whole archive, every time | any archived record whose replay at its own dial disagrees with its recorded dispositions |
| **R-6** | **The adjudicator's expectation leaks into the reading** | blinding **by projection**, not by care — the same structural shape `generate_derivations.py` already uses; shuffle key held by the runner | any arm-versus-arm reading recorded without a shuffle key |
| **R-7** | **A dial is selected on the replay alone**, which §17.3 forbids and which would make the selection a claim about a graph state that has passed | §12.8 splits the roles — replay narrows to equivalence classes, the fresh paired comparison selects; M4's own guidance states that a PR shipping a dial on replay output has made §4.4's error | **a dial value shipped whose selection cites no fresh paired comparison** |
| **R-8** | **The endpoint moves under a model-in-the-loop comparison** — measured, 3.4× on identical input | record provider, endpoint, model and per-call usage; flag a latency-distribution mismatch between arms; **and prefer R1/R2, which never touch it** | a bundle quoted without its endpoint and model |
| **R-9** | **A void is reported as a result** | §12.7's three-way distinction; a void is re-taken and the void rate is itself recorded | any comparison quoted whose void rows were not named |
| **R-10** | **The strategy is built and the substance path still does not ship**, because measuring became the work | phases 0–2 are all *small*; phase 1 narrows the dial and phase 4 selects it, and **both are on the critical path for §16.5 rank 3** | phase 1 does not produce a small set of distinct dial options within its first run, **or** phase 4 slips past it without a selection |
| **R-11** | **The reference arm becomes the spec**, and the project's thesis becomes uncheckable because the target is the thing it set out to challenge | §6.6.9's four structural mechanisms — the scale saturates at `equivalent`, the question asked is sufficiency not similarity, there is no scalar and no node set to converge on, and `size` runs the opposite direction. **Plus a standing probe requirement:** at least one probe the graph answers badly, and at least one the reference is adjudicated to get wrong | **`reference insufficient` and `both insufficient` never fire across twenty adjudications** — at which point the reference is a ground truth in practice, whatever the document says |
| **R-12** | **The adjudicator agrees with whichever report reads better**, rather than with whichever carries the substantial information | structural blinding (§6.6.6); the verdict carries its reasoning and the reasoning is spot-checked; `size` is reported mechanically beside the verdict so a fluent-but-larger report cannot hide its cost | a spot-check disagrees with the verdict and the disagreement is not recorded as a reading in its own right |
| **R-13** | **A KPI or a verdict gets aggregated**, and the instrument becomes a leaderboard pointed at the wrong target | §6.6.4's guard and §6.6.5 rules 3 and 4, stated as prohibitions rather than preferences | any artifact quoting a mean, a total, a rating, or *"equivalent on N of 6"* as a headline |
| **R-14** | **A node-set metric is re-invented under deadline**, because it is mechanical, free, and looks exactly like the comparison that was asked for | §4.4's three-rejection table and its invariant; §11 alternative 8 names the proposal explicitly so it is recognisable when it reappears | any comparison, report or PR body quoting overlap, Jaccard, or a count of shared nodes between the arms |

---

## 17. Decisions that are Toni's

Each has a recommendation. None of them should stall the work — **phases 0 and 2 depend on none of them**,
and phase 1's only dependency is D-1.

| # | decision | options | recommendation |
|---|---|---|---|
| **D-1** | ~~What objective does the replay select a dial against~~ | — | **DECIDED BY TONI, #13534 §17 — and this document's recommendation was REJECTED.** A mechanical rank-fidelity gate is out: it is §16.3's error one level down (§4.4). The objective is a **comparison against a working reference**, judged on **information sufficiency**, never on node identity (§17.6). §6.6 is the component, §12.8 is what the replay is reduced to, and D-8/D-9/D-10 are the decisions the ruling opened in its place. **No further input needed on D-1** |
| **D-2** | **`required[]`: demote or delete** | demote to an unscored prior; delete outright | **demote.** §10.2. It buys back comparison *direction* for free and carries no rate. Delete is defensible and costs that |
| **D-3** | **The probe set** | reuse `compare_tasks.json`'s six; author new; use `corpus.json`'s 23 | **reuse the six, and add two the graph answers badly** — a shutout-producing probe is the only way to see the crowding defect, which is invisible on well-matched inputs. Authoring rubrics for `corpus.json`'s rows was #11092 Q1's recommendation and the six already have them |
| **D-4** | **Who adjudicates, and when** | per run; per merge; per claim | **per claim only.** The agent produces the bundle unattended; the blinded reading is 20–60 min and happens when someone wants to assert an improvement |
| **D-5** | **n for a claim** | 1 / 3 / more | **n=1 routine, n=3 for a claim someone intends to assert, never more.** §13.3 |
| **D-6** | **May a failed expectation ever block a merge?** | yes / no | **no, never.** Only mechanical gates block. This is the structural expression of *an expectation that fails is a finding, not automatically a failure* |
| **D-7** | **Does the record gain per-stage timings?** | yes / no | **yes, and it is cheap** — elapsed is already computed and only logged. Without it, cost accounting is tokens-only and the endpoint confound (§12.3) is detectable only by re-deriving latency from outside. Filed as a separate unit; it gates nothing here |
| **D-8** | ~~What exactly is the report-form task text?~~ | — | **DONE, filed as #14571.** Six probes — four carried over, **two thin** (one *adjacent*, one *near-absent*, which is the crowding probe) — plus the wrapper, the anchor rule, the blinded adjudication prompt, the de-shuffle, and six falsifiers on the probe set itself. **The two measurement probes of `compare_tasks.json` were dropped**: the graph now contains this document, so they would measure the instrument's own footprint. **Phase 4 is unblocked** |
| **D-9** | ~~Which model adjudicates~~ | — | **DECIDED as recommended** (coordinator, #1176 Rule 9): a capable model in this harness, **dispatched separately from the reference arm** so neither sees the other's framing, blinded by projection, and **Toni spot-checks the reasoning rather than the verdicts.** Prompt at #14571 §8 |
| **D-10** | ~~Which probe is the one the reference is expected to get wrong~~ | — | **DECIDED as recommended** (coordinator, #1176 Rule 9): **wait for one; do not author one.** Authoring a probe to make the reference fail is authoring the answer. **R-11's fourth limb stays unmitigated and every bundle says so** — and #14571 §9.4 records that the ruled vocabulary cannot currently even express `reference failed`, which is the evidence this decision waits for |

---

## 18. What cannot be known, and what it depends on

Stated as the brief asks: a design that pretends to close every question is worth less than one that maps the
boundary.

**1. Whether a smaller block answers as well.** This is §16.1's central claim and the product's whole thesis,
and it is **not answerable by any instrument in this document**. The replay can tell you exactly what the
block becomes at a given dial; it cannot tell you whether the answer holds. **Depends on:** the substance path
shipping into consumption (§16.5 rank 3), *and* §14.3's fidelity gate — because a smaller block made of
substances that invert their sources answers worse for a reason no dial explains. Until both, phase 1 selects
a dial by a fresh paired comparison against the reference arm (§6.6), on the narrowed option set §12.8
produces — and **the claim that the resulting block is smaller *and* still sufficient is exactly what that
comparison can carry, while the claim that it improves answers on real tasks is not made.**

**2. Whether the probe set's difficulty is representative.** Six tasks, authored by us, against a graph we
wrote. There is no population to sample from and no way to establish representativeness. **This is not
fixable**, and the mitigation is to say so and to include at least one probe the graph answers badly (R-3),
not to imply coverage.

**3. Whether the model endpoint is stable across a comparison.** §12.3. Detection, not control, and the
detection is weak — a latency-distribution mismatch is suggestive, not conclusive. **Depends on:** something
outside this project. The architectural response is the routing rule, which keeps most measurement off the
endpoint entirely.

**4. What the rate of instrument drift is.** §9.3's exit (b) and (c) counts are the measurement of how fast
the probes decay, and **there is no prior for it** — nobody can currently say how stale `corpus.json` is,
which is itself the finding. It becomes knowable after roughly a dozen readings, and it is the number that
decides whether probes are a sustainable instrument on this substrate at all. **If exit (b) dominates, the
probe set is not the right instrument and this design's §9 needs revisiting** — that is this document's own
falsifier and it is stated deliberately.

**5. Whether the archive stays a census of real work.** A2/R-4. If the ratio of measurement runs to real runs
rises, ledger aggregates describe the instrument rather than the product. Detectable from bundle membership
once phase 3 ships; **not detectable today.**

**6. Whether any of this changes the answer Toni gets.** Honestly: **phases 0–3 change what we can see, not
what the product does.** They pass #13534 §11's test on its second half only — *would he be able to tell* —
and they pass it by construction, because telling is what they are. The first half, *does the answer get
better*, is discharged by **phase 1 and phase 4 together** (§15), not by phase 1 alone as revision 1 claimed.
**That is the correct division of labour for a rank-4 unit whose whole job is to make rank 3 selectable**, and
it should not be dressed up as more.

**7. How reliable the adjudicator is.** §17.6 makes a model judgement the instrument, and **its agreement with
a human has never been measured on this task.** It is the residual variance of the whole design (§13.3) and
there is no prior for it. **Depends on:** roughly twenty recorded adjudications with their reasoning
spot-checked — at which point agreement becomes measurable. Before that, the honest statement is that the
verdict is auditable (the reasoning is there) and its reliability is unquantified. **The mitigation is not a
number; it is that a human can read why.**

**8. Whether the probe set is aligned to the reference's strengths.** R-11's accepted residue. Even with no
scalar, no set metric and a scale that saturates at `equivalent`, a probe set chosen from tasks this harness
answers well shapes processor toward this harness's competence profile. **This is not detectable from inside
the instrument** — every probe would pass — and it becomes detectable only when a probe the reference gets
wrong exists to compare against (D-10). **Named, bounded, and not closed.**

---

## 19. Implementation Guidance — architectural milestones

Ordered. Each is a unit and each is its own PR.

**M1 — The record reader is a shared component, not a script.**
`internal/runbackfill` already decodes a run node's fenced JSON into a record, with named skip reasons for
every shape it cannot. The ledger, the replay and the bundle all need exactly that. **Extract the decoding
half so there is one reader**, and give it an archive selector (by id set, by date range, by dial value).
*Property:* every one of the 42 archived records either decodes or produces a named skip reason; the count of
each is reported. A silent drop is the failure.

**M2 — The ledger, with the requery dispositions first.**
The four Q4 dispositions (§6.2) are the unit that answers the question Toni says no instrument asks.

> **Boundary amended 2026-09-22, on the shipped artifact rather than on the plan.** M1–M3 as first written
> drew the line in the wrong place: the archive-level properties were briefed under M2, so the first ledger
> PR arrived as M1 + the requery dispositions + all of M3, and QA rejected it on scope. **The brief was
> wrong, not the implementation** — and extracting sound, independently reproduced code to satisfy a
> numbering buys no review value, so the milestone moves rather than the artifact.
>
> **The seam the artifact actually has, and it is a real one:**
>
> | unit | what it reads |
> |---|---|
> | **M1–M3 as shipped** | what the loop recorded about **its own mechanics** — the reader, the four requery dispositions, and the two archive-level properties that prove the reader is an instrument |
> | **split out as #14572** | what a run **claimed** against what it **did** — claim-action divergence and the two citation predicates |
>
> **That is a seam and not a convenience.** The three in #14572 read the **answer text** against the
> mechanics, which puts them on §4.3's one legitimate residue of the far side — syntactic facts about
> characters, routed to a reading and never to a verdict (§14.2). Everything in the shipped unit stays wholly
> on the record side. **A boundary drawn where the kind of instrument changes is worth more than one drawn
> where a numbering falls.**
*Run the whole thing over the 42 records before writing the production path* — P1's F-1 did exactly this and
it **moved the specification**, twice, before any code existed. Expect the same here and treat a specification
change as the falsifier working.
*Property:* every predicate is a pure function of one record or of a named selector; a poisoned stored value
is ignored in favour of recomputation.

**M3 — The archive-level properties.**
`dial exercise` and `mechanism observation`. Both fail today, on `SubstanceRatioThreshold`, `BlockOccupancy`
and `Fills`. **A property that does not fail on merge day is not the property** — verify each fires against
the current archive before shipping it. **This is V-13 (§12.11) at the predicate altitude**, and V-13 is the
same instruction for the guards underneath: remove what the assertion counts, watch it go red, put it back.

**M4 — Replay fidelity, then the offline replay.**
**In that order, and the order is the point.** Build the fidelity check first and run it over the archive; a
replay that cannot reproduce a record at its own recorded dial is a re-implementation and its deltas are
fiction. Only then compute the dial grids.
*Deliverable:* a disposition delta per record per dial value for `SubstanceRatioThreshold`, `BlockOccupancy`
and `RelevanceFloor`, **collapsed into equivalence classes of distinct block composition.** Six settings
producing three distinct outcomes is reported as three options, not six.
*What it is not:* a selection. §12.8 and #13534 §17.3 — the replay narrows, M7a chooses, fresh. **A PR that
ships a dial value on M4's output alone has made the error §4.4 records.**

**M5 — The sweep re-readout.**
Remove `rateOf`, the two rate lines and `Scored()`'s row-drop. Keep the control, the alarm and exit 1. Emit
per-row dispositions. Add arm comparison by candidate-id hash identity over the union. Add the per-pair delta
for `corpus-anchor.json` — which is the smallest change in this list and unlocks 16 matched pairs that have
never been read.
*Property:* no rate is printed anywhere, and `required[]` appears only as an unscored column.

**M6 — The write-suppressing port and the window.**
#11071's decorating port, as #11092 §4.2 specifies it: no change to `internal/loop`, no flag, no addition to
`WriteState`. Plus the window record and its two overlap properties.
*Property:* no record produced under the port reaches the graph, and every measurement run's receipt is
`notStored`.

**M7 — The outer loop.**
Take `compare.py` as the base. Add: the suppressing port on its substrate arm; blinding by projection with a
runner-held shuffle key; the window; the bundle. Do **not** rebuild it in Go — C4 bars `cmd/eval` from the
adapters, and a new Go binary would be re-deriving a working harness for no stated gain.
*Property:* the adjudicator's input structurally cannot carry the arm label.

**M7a — The reference arm and the adjudicator.**
**The probes and the prompt are written and filed: #14571.** One isolated agent dispatch per probe in this
harness; one **separate** blinded
adjudication dispatch per pair, emitting a verdict from §6.6.5's closed set **with its reasoning**; the
harness, model and date recorded on each. Runs on the existing `REFERENCE` vs `TRANSCRIPT` control before any
verdict is believed (§6.6.7).
*Properties, and the second is the one an implementer will be tempted to break:*
- **No node-set comparison appears anywhere in this milestone.** The reference's ids are evidence in the
  bundle for the adjudicator to read, never an input to a metric (§11 alternative 8, R-14).
- **`reference insufficient` and `both insufficient` are reachable and are exercised in the first run's
  test fixtures.** A verdict set whose asymmetric members can never fire is a ground truth with extra words.
- **Nothing is aggregated.** Six probes produce six verdicts, each with reasoning, each reported separately.

**M8 — The reading, as a filed artifact.**
A fixed five-step shape, three exits on a failed prior, exits (b) and (c) obliged to edit the probe in the
same act with the reading named as cause. Filed to DiVoid, linked to the record and the probe.
*Property:* no reading is complete while a failed expectation carries no exit; no probe edit exists without a
reading that names it.

---

## 20. Provenance

processor, 2026-09-22, read-only. Baseline `main` = `3f4ea35`; read on
`feat/a-run-that-got-nothing-behaves-differently` at `63abb09`. Sources: the repo at that tree
(`internal/loop/types.go`, `assemble.go`, `turn.go`, `outcome.go`, `internal/eval/*`, `cmd/eval/*`,
`internal/runbackfill/*`, `scripts/compare.py`, `scripts/step_trace.py`, `scripts/compare_tasks.json`,
`docs/architecture/m3-derived-recall.md` §9, `docs/architecture/a-run-that-got-nothing-behaves-differently.md`,
`VISION.md`, `README.md`) and DiVoid **#14525**, **#14524**, **#13534** (including **§17**, the ruling that
rejected revision 1's D-1, and **§17.6**, the sharpening that moved the equivalence relation from nodes to
information), **#14281**, **#11092**, **#13065**.

**Nothing was run.** No inference, no container, no sweep, no graph write, no task. Every figure quoted is
attributed to the document or file that measured it, and none is re-asserted as new — which is §12.5 applied
to this document itself.
