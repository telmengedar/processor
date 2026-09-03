# Architectural Document: measuring the system by task outcome — the A/B at one memory state

> Repo path: `docs/architecture/task-outcome-ab.md` (canonical copy — the DiVoid node carries the same
> document verbatim).
> Origin: Toni's reframing on PR #14, routed as **#11069** · Prior ruling **#11065** · Blocking defect
> **#11071** · Project **#10422** · Map root **#10454**
> Predecessor consumed, **not** superseded: **#10926** / `docs/architecture/m2-retrieval-eval.md` — the
> retrieval eval harness. It stays canonical and every mechanism in it stays in force. This document changes
> which number is the **headline**, not whether that instrument is right. #10926 §16 revision 7 argues the
> split from the other side.
> Standards applied: Design Contracts **#1136** (§1 KISS/DRY/YAGNI load-bearing; §5 answered in §9) ·
> Go Code Contracts **#11034**, in particular **P-51** — every rationale below carries a named measurement or
> an explicit hedge **in the same sentence**.
> Baseline: `origin/main` at **`60f30cc`**, working branch `design/m2-revision-7`. The four first-hand
> readings this document rests on are labelled *(measured 2026-09-03)*; everything quoted from #10926 §3 or
> from #11069 is labelled as inherited and was **not** re-measured here.

---

## TL;DR

*The retrieval rates were never going to answer **did the system get better**. They answer **what did it
retrieve**, which is a different and cheaper question. This document designs the instrument for the first
one and rules that it does not replace the second — it consumes it.*

**What is designed.** A protocol, not a system: run a fixed set of tasks the memory demonstrably covers,
twice, under two configurations, at **one memory state**, and record result quality, steps and context usage
per arm. Every signal but *result quality* is already instrumented and carries **zero variance** at a fixed
state; result quality needs a human and is where all the noise lives.

**What blocks it, and it is not a refinement.** `internal/loop/turn.go:131` calls `WriteRun`
unconditionally *(measured 2026-09-03)*. An A/B of N tasks × 2 arms therefore writes 2N run records **into
the memory state the experiment exists to hold constant**, and — on #10926 §3.4's inherited measurement —
those records outrank real content and exceed the whole assembly budget. **The A/B is not runnable until
this is answered.** §4.2 answers it, and the answer requires **no change to `internal/loop`**: the runner
supplies a decorating `GraphPort` whose `WriteRun` files nothing and returns the existing `notStored`
receipt. No flag, no second code path, no new vocabulary.

**Held constant is proven, not asserted.** Every candidate disposition already carries an id and a content
hash. If both arms saw the same ids with the same hashes for each shared input, the state was constant *for
everything the experiment touched* — which is the only constancy it needs. This costs nothing new and
**depends on #11066** landing: without the retained candidate set the evidence is discarded and the
constancy claim is unfalsifiable.

**A graph snapshot is rejected, and not on cost.** *A snapshot you can query is a different retriever.*
Holding the memory state constant means holding node bodies **and the ranking over them** constant; copy the
nodes anywhere else and the ordering is that store's. A snapshot does not hold the state constant — it
replaces it.

**At n=1 this is a regression gate, not a measurement.** Toni called it a smoke test and that is exactly
right. §5 names five further things it cannot measure, **two of which are structural rather than statistical**
— it is blind to tasks whose answer the memory does not contain, because its own admission precondition
selects those out; and it produces a scalar over the whole pipeline, which is the single-number design
#10926 §1 already ruled out. **That is the argument that these two instruments layer rather than compete:
the cheap one is what makes the expensive one interpretable at an affordable sample size.**

**No new package, no new dependency, no runner built yet.** §10 says what is buildable today and what is not.

---

## 1. Problem Statement

Toni, verbatim, and this document is written to these words:

> *"yeah, i think for a test or comparision we need to think more what our system represents - the nature of
> this experiment is that it works differently than classic systems. So approach should probably be something
> like (brainstorming) - we have a well defined task where we know that our system has all the information to
> solve it, we let the system solve it and record result quality, number of steps, context usage and so on.
> This test we should do as kind of A/B test at one specific memory state - only that way we can measure
> whether anything got better. It still requires evaluating the result by more than just a mechanical gate and
> its always a smoke test, but its the only approach i see currently to actually measure something."*

And the message one step earlier, because it is the same thought arriving:

> *"okay - so its kind of a test set for our context building loop or something? - then we need to be aware
> that our memory is a living system. as soon as more fitting or more precise nodes exist it will probably
> include different nodes, at some point it might not include the expected ones. So its more of like an
> indicator telling us - we should check that and not outright a fail. The check then would tell us whether
> the signal is actually a red flag or whether we need to change the corpus."*

**The problem.** #10926 built an instrument that measures **retrieval** — which nodes came back, which
survived the budget — deterministically and for free. It does not measure whether the system *answered
better*. Nothing does. So a tuning change in M3 can be shown to move a retrieval rate and cannot be shown to
have helped, and the project's own claim (#10424) is about the system being better, not about a rate.

**Success criterion.** One: *after a configuration change, a reader can say whether the system got better at
a set of tasks, and can say what in the pipeline moved.* Both halves. The first alone is a scoreboard —
#10926 §1's test, verbatim, binds here too: *"would this instrument change anyone's mind?"* A scalar
end-to-end verdict would not, which is §5.4.

---

## 2. Scope and Non-Scope

### In scope

- The **protocol**: what is held constant, what varies, what is recorded, and how constancy is proven after
  the fact from evidence the runs already carry.
- The **judging protocol** for result quality — the one signal that needs a human (§4.3).
- The resolution of **#11071**, which blocks everything else (§4.2).
- The ruling on what this measures and what it structurally cannot (§5).
- What #10926 becomes under this framing (§7).

### Explicitly out of scope

| Not built | Why, and what would change that |
|---|---|
| **An A/B runner** | It needs the task set settled and #11066 landed. Building it now means building against an unsettled input. **Trigger:** #11066 merged and the task set agreed (§11 Q1) |
| **Automating the quality judgement** | It is a judgement, and #10926 §4.3's argument binds here verbatim: an explicit definition is strictly better than a smuggled one. An LLM-as-judge is a different instrument with its own validation problem, and adding it now would put an unvalidated measurement inside the measurement |
| **A graph snapshot, or any second store** | §4.1. A snapshot you can query is a different retriever |
| **Changing `internal/loop`** | §4.2's shape needs none, which is the reason it is the shape chosen. #10926 §2 already rules that a harness needing the loop to change is no longer measuring what runs, and that reason binds harder here, not less |
| **Retiring the corpus, the required-node key, or either retrieval rate** | §7. They are demoted from headline to explanation, which is a change of role and not of correctness |
| **Any change to the sweep's exit code** | #10926 §4.2 and §9.2 stand untouched |

---

## 3. Why this is a third layer, not a replacement

| Layer | What it answers | Cost | Variance at a fixed memory state |
|---|---|---|---|
| **Task-outcome A/B** | did this configuration change make the system better | model calls + human judgement | **high** — the model is the only variance source, and it is the whole of it |
| **The retrieval sweep (`cmd/eval`)** | what did each arm retrieve, admit and cut | 0 model calls, ~70 graph GETs (#10926 §10.1, inherited) | **none** |
| **The required-node key** | which of two different retrievals was *better* | one line per task, at authoring | n/a |

**The key is not a rival artifact — it is Toni's own precondition written down.** His precondition is *"we
know that our system has all the information to solve it."* You cannot establish that without knowing *what*
information and *where it lives*, which is the labelling act. A version that performs it and keeps no record
is not weaker in what it asserts; it is the same assertion with the evidence discarded — which is exactly
#10926 §4.3's ruling against constructed rows, quoted here because it binds unchanged:

> *"Under (3) the labeller's definition is 'the node I wrote the question from' — which is a worse definition
> precisely because it is invisible. **An explicit definition is strictly better than a smuggled one.**"*

**Three of Toni's four signals are already instrumented, and they are the three with no noise.**

| His signal | Where it already lives | Variance at a fixed state |
|---|---|---|
| context usage | `admittedBytes` / `budgetBytes`, `candidateCount`, `k′` — #10926 §8.2 | **zero**, given the config |
| number of steps | `ModelCalls`, `ToolCalls`, `CapReached`, `StopReason` — on `loop.Record` *(measured 2026-09-03: all five are fields set in `turn.go` before the record is returned)* | low, model-dependent |
| what was in context | `candidates[]` — on the M1 record today, and on the eval result once **#11066** lands | **zero** |
| result quality | nothing. Needs a human | **high** |

So "context usage" is not a thing to build. It is `admittedBytes`, and it shipped. **The cheap instrument
computes the only signals in the whole experiment that carry no noise**, which is §5.5's argument for why the
expensive one is not interpretable without it.

---

## 4. The experiment

### 4.1 What is held constant, and how that is proven

**Held constant:** the memory state — the graph's node bodies and the ranking over them — and the task set.
**Varied:** exactly one configuration difference per comparison.

**The rejected way to achieve it is a snapshot, and the rejection does not rest on cost.** #10926 §11.2
originally rejected a snapshot as *"a substantial mechanism for a problem that has not yet bitten"*. That is
a cost argument, and it does not survive this document — here a fixed state is not a confound to control, it
is the experimental design. The rejection survives on a different and stronger argument:

> **A snapshot you can query is a different retriever.** Holding the memory state constant means holding node
> bodies **and the ranking over them** constant. #10926 §11.1 records C22's measurement that DiVoid's ranking
> is bit-stable for a fixed `(graph state, query)` — inherited, not re-measured here — and a reimplementation
> carries no such guarantee. Copy the nodes into any other store and the embedding and the similarity
> ordering are that store's, not DiVoid's. **So a snapshot does not hold the memory state constant; it
> replaces it**, and the arms would be measured against a retriever this system does not ship.

**What is sufficient is free: run the arms back-to-back and prove constancy afterwards from evidence the runs
already carry.** Every `loop.Disposition` carries an id and a content hash *(measured 2026-09-03:
`Disposition` is the element type of `Record.Candidates`, and `assemble.go` computes a content hash for every
candidate — #10926 §6.5 states the same economy for the sweep)*. If both arms saw the same ids with the same
hashes for each shared input, **the state was constant for everything the experiment touched** — which is the
only constancy the experiment needs, and a stronger claim than "the graph did not change", which is not true
of a shared substrate and cannot be made true.

This is #10926 §6.5's economy one level up, and it **requires #11066**: without the retained candidate set the
evidence is discarded at the end of `BuildRow` and the constancy claim is unfalsifiable. `stale` therefore
keeps its mechanism and changes its job — **from detecting drift under a label across sweeps, to proving two
arms saw one graph.**

**The constancy check invalidates a run; it does not prevent one.** DiVoid is a substrate other sessions
write to, so a peer write can land between arm A and arm B — this is reasoning from the graph being shared,
not a measurement of how often it happens, and nobody has measured that. The consequence is operational and
belongs in the protocol rather than in a mechanism: **a comparison whose constancy check fails is discarded
and re-run, and the discard rate is itself worth recording** — if it is high, the arms must be interleaved
per task rather than run arm-at-a-time.

### 4.2 The blocking precondition — no record reaches the graph, and the shape that achieves it

**The defect, measured first-hand.** `internal/loop/turn.go:131` reads
`receipt := t.Graph.WriteRun(context.WithoutCancel(ctx), record)` — unconditional, no flag, no branch
*(measured 2026-09-03)*. Every full-loop run files one `session-log` record into the graph it reads from. On
#10926 §3.4's inherited measurement those records outrank real content for inputs resembling past runs, and
#10897 at 70,660 B exceeds the entire 60,000 B assembly budget.

> **So the single most likely outcome of a naive A/B is that arm B is measured against a graph arm A
> polluted, in the way most damaging to arm B's retrieval — and the design would report a configuration
> difference that is an ordering artifact.**

**#11071 rules the constraint and leaves the shape to the implementer.** Choosing between competing shapes is
design, so the shape is ruled here; the constraint is #11071's and is not touched.

**The shape: the runner supplies a decorating `GraphPort`. `internal/loop` does not change.**

```
                    ┌──────────────────────────────┐
  A/B runner ──────►│  measuringGraph              │
                    │    Node    ──► pass through  ├──► divoid.Client ──► DiVoid
                    │    Recall  ──► pass through  │
                    │    WriteRun ─► record the    │
                    │       fact and the size;     │
                    │       file nothing;          │
                    │       return {NotStored}     │
                    └──────────────────────────────┘
                              ▲
                    loop.NewTurn(measuringGraph, …) — the shipped Turn, unmodified
```

Four properties, each checkable:

1. **No change to `internal/loop`, and therefore no drift.** #10926 §9.3's no-drift property is preserved *by
   construction* rather than by a test: there is exactly one turn implementation and the measured run executes
   it. A suppression flag on the turn would create the second code path #11071 names as the thing to avoid.
2. **No new vocabulary.** `WriteState` is a closed set of three and `NotStored` is documented as *"no node
   holds the record"* *(measured 2026-09-03, `internal/loop/types.go:92-94`)* — which is precisely true of a
   suppressed write. Nothing is added to a shipped closed set for an instrument's convenience.
3. **No knob, and therefore no #1136 §3 problem.** There is no flag to leave permanently off. The named
   operator #11071 asks for is the runner itself, which is a real caller with a real reason.
4. **It is a second implementation of an existing seam, not a new abstraction.** `GraphPort` is declared in
   `internal/loop/turn.go` with three methods and is already implemented by `internal/divoid`
   *(measured 2026-09-03)*. #1136 §6's one-implementation anti-pattern does not fire on adding the second.

**Two alternatives, rejected with the reason:**

| Alternative | Rejected because |
|---|---|
| A suppression flag on `Turn` | It is the second code path through the turn that #11071 names, and it is a knob with exactly one caller — #1136 §3. It also has to be threaded through `NewTurn`, which every existing caller then carries |
| Move `WriteRun` out of `Turn.Run` into the caller | Defensible — `Turn.Run` already returns `(Record, WriteReceipt, error)` and `internal/server/routes.go`'s `handleRuns` is its only production caller *(measured 2026-09-03)*. Rejected on scope: it changes `internal/loop`'s contract and splits `logFinished`'s single log line, to buy a property the decorator gets for free. **It becomes the right shape only if a second production caller ever needs a different record fate** |

**The records are discarded, not deferred.** #11071's constraint is *"no record reaches the graph before every
arm of a comparison has run"*, which a discard satisfies. Filing them afterwards would add 2N records of
50,000–110,000 B *(size range inherited from #11071, not re-measured here)* to a graph #10926 §3.4 already
shows is polluted by exactly this content — and #10926 §17 open question 2 has already taken the matching
default for sweep results: *the harness writes files; a human files the interesting ones.* The same answer
binds. The experiment's records are the experiment's data, not the system's memory; they live in the runner's
result file, and anyone who wants one in the graph files it by hand with a reason.

### 4.3 The judging protocol — the only place a human is required

Toni: *"It still requires evaluating the result by more than just a mechanical gate."* Correct, and the
judgement needs the same discipline #10926 §4.3 imposes on the labeller, for the same reason.

| Rule | Why it is not ceremony |
|---|---|
| **The rubric is written before the arms are run**, per task: what a correct answer must contain, and what makes one materially incomplete | Otherwise the rubric is written after seeing the outputs, and it will be written to fit them. This is #10926 §4.3's smuggled-definition argument applied to the judge instead of the labeller |
| **The judge does not know which output is which arm.** Outputs are presented unlabelled and in a shuffled order | The person running an A/B made the change and wants it to work. Without this, the A/B measures the judge's expectation, and it is the only variance source that a larger `n` does **not** shrink |
| **The verdict is one of three: A better, B better, indistinguishable** — never a score | A score invites averaging across tasks, which manufactures a resolution the sample size does not have (#10926 §6.3's residual, one level up) |
| **`indistinguishable` is a real and expected outcome**, not a failed measurement | A protocol with no way to say *nothing happened* reports a winner every time |

**Not built: an automated judge.** §2 says why. If one is ever added it is a new instrument that must itself
be validated against blinded human verdicts on this same task set — which is a reason to keep the human
verdicts as a record from the first run onward, at zero cost.

---

## 5. What this cannot measure

Toni named one limit — *"its always a smoke test"* — and it must not stand as the only one, because it is the
one a larger sample fixes and **three of the five below are structural**: no sample size, no rubric and no
amount of care removes them.

### 5.1 At n=1 it is a regression gate, not a measurement *(statistical)*

With the memory state fixed and retrieval deterministic, the model is the only variance source. So at n=1 the
protocol answers *did this change break something obvious*, which is what a smoke test is. **Reporting its
output as "arm A is better" is a claim n=1 cannot support.** The signals that *do* support a claim at n=1 are
the zero-variance ones — context usage, what was retrieved, what was cut — which is the M2 harness.

### 5.2 It cannot measure accumulation — the project's central question *(structural)*

An A/B at a fixed memory state measures **configuration**. #10424's claim is about a *memory substrate* —
that having and keeping more memory makes the agent better. To measure that, the memory state would have to
be the **variable**. **Holding it fixed is precisely what makes the A/B clean, and precisely what blinds it to
the thing the project exists to demonstrate.**

That question is answerable only across time, on a moving graph, against labels naming what an answer should
draw on — which is the artifact #10926's corpus already is, and the reason its cross-time machinery is kept
rather than retired.

> **Two questions, two instruments.** *Did this change help?* → the A/B at a fixed state. *Does more memory
> help?* → the corpus across time, with the label rot #11065 ruled on and the honesty #10926 §11.2 states.

### 5.3 It is blind to tasks the memory does not cover — by its own admission rule *(structural)*

This one is not named in #11069 and is the sharpest of the five. Toni's precondition — *"we know that our
system has all the information to solve it"* — **selects the task set for cases where retrieval can succeed.**
Every task in the set is one the memory already covers. So the protocol is structurally incapable of seeing
the failure mode where **the memory does not contain the answer at all**, which for a memory system is the
failure that matters most.

**The falsifier for this claim, stated as #1220 §5 requires:** an input class that would break it is a task
whose required nodes exist but are *unreachable* by the recall query — the memory has the information and the
retriever cannot find it. Those tasks pass the admission precondition on a reading of the graph and fail in
the run, so they are **not** excluded, and the A/B does see them. The class genuinely excluded is narrower and
worth stating precisely: **tasks for which no node exists.** They are excluded by construction, they are the
population the substrate is supposed to shrink over time, and nothing here counts them.

**What to do about it:** nothing, in this design. It is a stated limit, not a defect — the same shape as
#10926 §11.2's. The instrument that would see it is a corpus of questions *no* node answers, and the honest
thing is to name that it does not exist rather than to imply coverage.

### 5.4 It cannot attribute — and alone it is the single-scalar design #10926 already rejected *(structural)*

A task-outcome verdict is one bit over the whole pipeline: retrieval, assembly, prompt, model, tool loop,
write-back. When arm A beats arm B it says nothing about **which stage moved**. Applied to this document,
#10926 §1's test — *"would this instrument change anyone's mind?"* — has the same answer it had there: a bare
verdict is a scoreboard, and *"arm A won"* changes nobody's mind about what to do next.

**That is the whole argument for the three layers.** The A/B says *whether*; the sweep says *where*; the
required-node key says *which retrieval was better*. Reported alone, the A/B fails the test that produced
#10926's design.

### 5.5 It cannot price itself — a configuration that wins by spending more still wins *(structural)*

A configuration that improves outcomes by consuming three times the budget or three times the model calls
"wins" the comparison. The A/B has no cost axis; it has an outcome axis. What makes cost visible is again the
zero-variance record — `admittedBytes`, `ModelCalls`, `ToolCalls` — read beside the verdict. **A verdict
quoted without the context-usage figures beside it is a result whose price nobody looked at**, and that is a
reporting rule, not a mechanism.

---

## 6. Cost, and the n=1 ruling

Using #10926 §10.2's inherited per-run figures (a one-call run measured at 15,327 in / 130 out; a two-call run
at 11,446 + 16,508 in) — **not re-measured here** — eleven tasks × 2 arms × *n* repeats:

| | Value |
|---|---|
| n = 1 | ~**440,000** input tokens, ~22 runs |
| n = 3 | ~**1.3 M** input tokens, ~66 runs |

**The n=3 figure matches the ~1.35 M composition path #10926 §10.2 priced and declined as out of scope for
M2.** So the proposal has to clear a bar this project already set: **at n=1 it clears it easily; at n=3 it
does not**, and n=3 is what would be needed to make an outcome claim rather than a regression check. That
tension is real and is not resolved here — §11 Q2 carries it.

---

## 7. What #10926 becomes

| Component | Fate |
|---|---|
| **The harness (`cmd/eval`)** | **Alive, promoted.** Model-free observation of steps 1–6 is exactly the per-arm deterministic record this protocol needs. Not a diagnostic *beside* the experiment — a component *of* it |
| **The corpus inputs** | **Alive, unchanged.** Real questions with subjects, authored against a stated definition, are task inputs |
| **`required[]`** | **Alive, re-purposed.** It converts *the arms retrieved differently* into *one retrieved better*. Without it an A/B can see a difference and not its direction |
| **The two rates** | **Demoted from headline to explanation.** Definitions unchanged |
| **The retrieved/admitted split** | **Alive on its original argument**, which never depended on being the headline: when arm A beats arm B, *"A retrieved it and B did not"* and *"both retrieved it, B cut it"* remain opposite diagnoses with opposite remedies (#10926 §5) |
| **`hash` / `stale`** | **Promoted.** Job changes from cross-time drift detection to proving two arms saw one graph (§4.1) |
| **The control stratum, and #10926 revision 6's exit split** | **Alive, unchanged.** The harness still verifies itself on every sweep |
| **#10926 §6.3's "start at 30 rows" sizing** | **Suspended, not deleted.** The standard-error argument sized a *headline* rate. #10926 revision 7 strikes the growth instruction and keeps the arithmetic, which stays true and stays the reason the rate cannot be quoted as a headline |
| **#10926 §6.5's cross-sweep comparability rule** | **Alive, and now the only route to the question §5.2 says this instrument cannot ask** |

**Does this change M3?** In shape, not in content. M3 becomes **two loops at different frequencies**, and
neither instrument is superseded:

- **Inner loop — the sweep.** Change a knob, sweep, read the rates and the candidate sets. Minutes, zero model
  calls, no currency, no variance. This is where candidate changes are **found**.
- **Outer loop — the A/B.** Confirm the ones that look real against task outcome at one memory state. Slow,
  costly, noisy. This is where changes are **accepted**.

Running the outer loop on every knob twiddle is unaffordable; running only the inner loop is measuring the
proxy. **R13's exclusion — #10926 §11.3's designated first tuning decision — is a good first exercise of
exactly this**, and the ordering claim survives the reframe intact.

---

## 8. Risks and mitigations

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| A1 | **The experiment contaminates its own memory state** by filing run records between arms | §4.2's decorating port; records are discarded, not deferred | a measured run's result carries a `WriteReceipt` whose state is not `notStored` |
| A2 | **The state moved between arms** because a peer session wrote to the shared graph | §4.1's after-the-fact constancy proof over ids and content hashes; a failed check discards the comparison | two arms report different content hashes for the same candidate id and the comparison is quoted anyway |
| A3 | **The judge's expectation leaks into the verdict** | §4.3: rubric before the run, blinded and shuffled outputs, a three-valued verdict | a verdict recorded without the rubric that preceded it, or with the arms identified to the judge |
| A4 | **An n=1 result is reported as a measurement** | §5.1 states the ruling; §6 prices what a measurement would cost | *"arm A is better"* appears in any artifact whose run count is 1 |
| A5 | **A configuration wins by spending more** | §5.5: the verdict is reported beside `admittedBytes` and `ModelCalls`, which are free | a verdict quoted without the context-usage figures beside it |
| A6 | **The task set drifts toward tasks the system happens to do well** | The set is a named, versioned list and a change to it breaks comparability exactly as a corpus edit does (#10926 §6.5, and its E10 gaming row) | the task set changes in the same commit as a configuration change |

---

## 9. Pre-Design Checklist (#1136 §5), answered in order

**KISS / DRY / YAGNI**

| Item | Answer |
|---|---|
| No new type mirroring an existing type | ✓ None proposed. The decorating port implements an **existing** interface (`loop.GraphPort`) |
| No abstraction with one implementation | ✓ The seam already exists with one production implementation; this adds the second |
| Nothing justified by "we might need X later" | ✓ The runner, the automated judge and the cost axis each carry a **named trigger**, not a hope (§2, §4.3, §11) |
| No deprecation period, feature flag, compatibility shim or transition window | ✓ None. §4.2 exists specifically to avoid a flag |
| `block_size × site_count` quoted for any inline-vs-extract call | ✓ No such call. No production code is specified by this document |
| **Solution sized against the problem statement** (#1220 RULING 2026-09-03) | ✓ Stated out loud: the ask is two paragraphs of brainstorming, so this is a **protocol document, not a system design** — no phases, no runner architecture, no config surface, and **zero new mechanisms in `internal/loop`**. The one structure proposed is a decorator over an existing interface, and it exists to *delete* a mechanism (the flag) rather than to add one |

**Existing systems first**

| Item | Answer |
|---|---|
| Existing surface audited | ✓ `loop.GraphPort`, `loop.WriteState`, `loop.Disposition`, `Record.ModelCalls`/`ToolCalls`/`StopReason`, and `cmd/eval`'s whole output are **reused**; nothing is re-derived |
| The concrete reason a new layer cannot live on an existing surface | ✓ No new layer. The protocol is a document; the one code shape is a second implementation of a declared port |
| Concrete 4-week decision for any new persisted data | ✓ None is proposed here. The runner's result file is deferred with the runner (§10) |
| Consumer chain recursed for every field | ✓ No new fields. §3's signal table names the reader of each existing one |

**Configurability**

| Item | Answer |
|---|---|
| Every new knob has a named operator or environment difference | ✓ **No knob.** §4.2 rejects the suppression flag for exactly this reason |
| No telemetry-then-tune compound | ✓ None |
| Magic numbers stay `const` | ✓ No new constants |

**Less is better**

| Item | Answer |
|---|---|
| Delete / merge / inline test run on every element | ✓ Three elements exist: the decorating port (delete it and the experiment contaminates itself — A1), the constancy proof (delete it and A2 is undetectable), the blinded rubric (delete it and A3 is unmeasurable). Deferred-then-filed records were **deleted** in favour of discard (§4.2); an automated judge was **deleted** (§2); a cost axis was **deleted** in favour of reading figures that already exist (§5.5) |
| Trade-offs named where the complex option won | ✓ The simple option won everywhere. The one place a simpler option was rejected is *"move `WriteRun` to the caller"*, and §4.2 names why the decorator is simpler still |
| Radical-clean chosen where nothing is consumed | ✓ Records are discarded rather than buffered-and-filed |
| Reader inventories explicit | ✓ §3's table and §7's table are the inventories |

**Document discipline**

| Item | Answer |
|---|---|
| Cites Code Contracts and Design Contracts as load-bearing | ✓ Header; **#11034 P-51** is applied per-sentence rather than cited and forgotten |
| Out-of-scope listed explicitly | ✓ §2, six rows, each with a trigger or a reason |
| No multi-paragraph rationale for things that obviously stay | ✓ |
| Predecessor designs banner-marked where superseded | ✓ **#10926 is not superseded and gets no banner.** It stays canonical and every mechanism in it stays in force; what changes is which number is the headline. #1136 §5's banner rule binds an **end-to-end** supersession, and this is not one. #10926 §16 revision 7 records the same ruling from its side, and #10926's header carries a forward pointer here so neither document can be read as the whole picture |

---

## 10. Implementation order

**Nothing here is buildable today, and that is the finding rather than an omission.** Two things gate it, in
this order:

| # | Gate | Owner |
|---|---|---|
| 1 | **#11066** — retain the candidate set in the eval result. §4.1's constancy proof is unfalsifiable without it | filed, blocked on #10926 revision 7 landing |
| 2 | **The task set.** Eleven labelled corpus rows exist; whether they are the A/B's task set is §11 Q1 and needs Toni | open |

**Then, and only then:** the runner. Its shape is one unit — the decorating port of §4.2, the per-arm record,
the constancy check, and the result file — and it is not specified further here, because specifying a runner
against an unsettled task set is designing for a shape nobody has agreed to.

**#11071 is dischargeable independently of all of this** and should be, because it is a defect in its own
right: a loop that files a record unconditionally has no way to be run under observation. The decorating port
of §4.2 is its whole implementation and touches no shipped file.

---

## 11. Open questions

1. **Is the A/B's task set the eleven corpus rows, or a different set?** They were authored to test
   *retrieval*, not to be *tasks a system solves* — the two overlap heavily and are not the same thing. A
   corpus row is a question with a subject; an A/B task additionally needs a rubric (§4.3) and needs to be
   worth solving end-to-end. **Recommendation:** start with the eleven, write rubrics for them, and let the
   rubric-writing surface the ones that are not really tasks. It costs nothing and it is the same
   author-against-the-schema move that produced #10926 revision 6.
2. **Is n=1 acceptable as the standing protocol?** §6 prices n=3 at the figure #10926 already declined for M2.
   **Recommendation:** n=1 as the routine gate, with n=3 reserved for a change someone actually intends to
   claim as an improvement. That makes the sample size a per-decision choice rather than a fixed policy, and
   §5.1's ruling is what keeps it honest.
3. **Who judges?** §4.3 requires blinding, which means the judge cannot be the person who made the change,
   which for a one-person project is a real constraint rather than a formality. **Recommendation:** Toni
   judges, and the arms are shuffled by the runner so the labelling is genuinely withheld — which is a
   property of the runner and should be stated in its brief before it is built.
