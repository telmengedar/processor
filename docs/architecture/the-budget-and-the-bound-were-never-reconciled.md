# Architectural Document: The Output Budget and the Bound Were Never Reconciled

> **Intended repo path:** `docs/architecture/the-budget-and-the-bound-were-never-reconciled.md`.
> **Project:** #10422. **Unit:** #14624. **Evidence:** #14622.
> **Baseline read:** `main` = `e221b86`, read-only. Every repo fact below was read at that ref.
> **Consumes, does not supersede:** #14217 (`docs/architecture/a-successful-call-that-carried-nothing.md`),
> #14130, #11312 (`docs/architecture/what-a-failed-run-reports.md`), #13534 §16.1 / §16.5 / §17.5.
> **No measurement in this document is mine.** §3 states whose each one is and what it does and does not cover.
> Three arithmetic inferences *are* mine, and each is marked **[derived]** with its inputs.

---

## TL;DR

**The ruling, in one line: the product must stop buying output it cannot afford and does not read — and the
fix is a stated property, not a smaller number.**

#14624 asks which of five remedies wins. The answer is that **the list is framed around the wrong quantity.**
All five move a dial. The defect is not that any one dial is set wrongly; it is that

> **`MaxOutputTokens = 4_096` and `DefaultTimeout = 5 * time.Minute` are two independent compile-time
> constants that have never been reconciled with each other, and the product has no property that keeps them
> consistent on any host.**

Together they encode an undeclared assumption — **≥ 14.6 generated tokens per second at a 63 KB prompt** —
that nothing states, nothing checks, and nothing reports when it is violated. The host delivers less than
that. The run dies at 300.001 s and the operator is told the bound was hit, which is true and is not the
finding.

**The ruling, in four decisions:**

| # | Decision | Which of #14624's five |
|---|---|---|
| **D-1** | **Thinking is off on every call, and the product proves it is off.** The switch lives where the protocol has one; where it does not, the capability is probed at boot and reported. | **#4 wins as primary** — with the correction that it is *not* a one-line copy of `condense.go`, because the live adapter is openai-compat and openai-compat has no such key |
| **D-2** | **Every call site declares its own output budget, and boot refuses a budget the bound cannot afford at a declared floor rate.** The number 18.2 never enters the code; the *inequality* does. | **#2 wins in amended form** — rejected as stated (one lower global constant), accepted as per-site budgets under a checked property |
| **D-3** | **A failed call reports the rate it implies.** The timeout becomes the measurement that would have prevented it. | extends **#11312** |
| **D-4** | **Query derivation survives, right-sized — and its fallback becomes visible on the record.** | the second question |

**Rejected:** #3 (raise the bound) outright; #5 (a different model) as a remedy, escalated to Toni as a
product decision; #1 (decode the reasoning channel) *as the remedy* — it is re-ordered and re-motivated,
because it cannot return anything from a call that never returned.

**Named and not scheduled:** **streaming**, which is the only mechanism that dissolves the rate dependence
entirely, and is the precondition that would make raising a bound safe. §8.6.

---

## 1. Problem Statement

The processor arm cannot complete a turn at its shipped configuration when the retrieval block is full. Two
probe runs of the first reference-arm comparison died at exactly the adapter's client bound; the only run
that completed had an empty block. Query derivation timed out on 4 of 4 runs and has never once succeeded on
this configuration. Phase 4 cannot execute, and #13534 §16.5 rank 3 is blocked behind it.

**What has to be settled:** which remedy, why the others lose, whether the derivation step survives, and what
on the record becomes incomparable as a result.

### 1.1 The real problem, restated

The stated symptom is *a timeout*. The stated cause is *a reasoning model whose output the adapter discards*.
Both are true. Neither is the architectural problem, because both are facts about **this host and this
model**, and a remedy scoped to them is a remedy that a second host invalidates.

The architectural problem is one level up:

> **The product spends a resource (time) by declaring a budget in a different unit (tokens), and nothing in
> the product converts between them, states the conversion rate it assumes, or notices when the assumption
> fails.**

Every one of #14624's five options is an attempt to satisfy the conversion by guessing at one of its terms.
That is why none of them is obviously right, and why picking one would leave the next host to rediscover
this.

### 1.2 Success criteria

1. A run with a full block completes at the shipped configuration on this host, or **refuses at boot with a
   sentence naming what the host would have to deliver**.
2. No remedy hard-codes a measured rate as if it were a property of the world.
3. A run that still fails leaves a reader able to compute *what the host actually delivered* from the failure
   line alone.
4. Every figure that becomes incomparable is named here, before it is discovered later.
5. Phase 4 can run.

---

## 2. Scope & Non-Scope

### 2.1 In scope

- The relationship between output budgets and time bounds, at every call site the product has.
- Whether the product enables an output channel it does not read.
- What a call that dies at a bound reports.
- Whether the query-derivation step survives, and in what shape.
- Naming the comparability casualties.

### 2.2 Out of scope — declined explicitly

| Declined | Why |
|---|---|
| **`AssemblyByteBudget = 60_000` and what goes in the block** | #13534 §16.1 is a live direction with its own units; this ruling must not pre-empt it. **But see §9.4** — a smaller block relieves this bound too, and that is a second, independent reason for §16.5 rank 3 |
| **Retrieval and admission quality** | #14622 §4 is explicit: retrieval assembled the block and handed it over; the run died downstream. Nothing here bears on the memory substrate |
| **Whether the container behaves the same way** | #14573's parity check owns it. This ruling holds at the measuring runner; it makes no claim about the container |
| **Retries** | Settled at `m1-skeleton-loop.md` §10.5 and re-affirmed at #14217 §7.2. A retry of a call that timed out is a slower failure |
| **Rewriting the six report-form probes** | #14622 §7: they have still never been exercised. Unchanged by this ruling |
| **The reference arm's own configuration** | It runs in a working harness under no byte budget. §17.5 makes that a premise |

---

## 3. Measured Facts, and Whose They Are

### 3.1 Not mine — the operator's, at #14622, 2026-09-22

| Fact | Value | Covers |
|---|---|---|
| generation rate, direct host probe | **18.2 tok/s** (2,514 tokens / 138.2 s) | one call, one moment, **prompt length not recorded** |
| prompt processing, 63 KB, cold | 19.7 s | one call |
| P1 / P2 failures | `5m00.001s` each, requests 63,673 B and 95,033 B | n=2 |
| the completing run | empty block, 20 candidates all cut, **105 output tokens**, 56 s | n=1 |
| derivation timeouts | 30.001 / 30.001 / 30.000 / 30.001 s | 4 of 4 today, plus 2 of 3 newest archived |
| model residency | `qwen3.8:27b`, 14.6 GB of 20.4 GB in VRAM | not the all-RAM pathology of #13534 §8 |

### 3.2 Not mine — #14130's, and note what request shape each was taken under

| Arm | `think` unset | `think: false` |
|---|---|---|
| judgement, short prompt, 4 seeds | content 389–439 B, **11.5–24.7 s** | content 299–369 B, **2.6–3.5 s** |
| derivation, seeded 1–5 | 5/5 produced the demanded six lines | **3/5** produced six lines |
| derivation, one call | `eval_count` **1,469**, thinking 6,705 chars, content empty | content in ~2.3 s |
| judgement, random seed, n=12 | **empty content on 2 of 12** | — |

**For this model, `think` unset and `think: true` returned byte-identical content and thinking lengths on
every seed** (#14130, 2026-09-16 correction). There is no third behaviour. Unset means on.

### 3.3 Mine — read at `e221b86`

| Fact | Site |
|---|---|
| `MaxOutputTokens = 4_096`, a **compile-time constant**, not configuration | `internal/loop/turn.go:19` |
| both adapters' `DefaultTimeout = 5 * time.Minute`, **compile-time constants** | `internal/{ollama,openaicompat}/client.go:19` |
| `runBound = 10 * time.Minute` | `internal/server/server.go:13` |
| `DerivationBound = 30 * time.Second`; `FillBound = 90 * time.Second`; `MaxFills = 2`; `MaxModelCalls = 6` | `derive.go:17`, `fill.go:41`, `fill.go:34`, `turn.go:17` |
| `ModelPort.Derive` **takes** a budget parameter; `ModelPort.Judge` **does not** — the adapter reads the package constant directly | `turn.go:86-89`; `openaicompat/client.go:84`; `ollama/client.go:95` |
| the derivation call is issued with `MaxOutputTokens` — the **same 4096 as the judge** | `derive.go:239` |
| `internal/ollama/condense.go` sets `Think: false`; `Derive` routes through `Condense`, so **ollama's derivation arm is protected** | `condense.go:16,64,116` |
| **`internal/openaicompat/condense.go` has no thinking control of any kind.** Neither does `openaicompat/wire.go`'s `chatRequest` | `condense.go:34-42`, `wire.go:11-17` |
| **neither adapter's response struct has a reasoning field.** `openaicompat.wireResponseMessage` = `Content`, `ToolCalls`. `ollama.wireResponseMessage` = `Content`, `ToolCalls` | `openaicompat/wire.go:98-101`, `ollama/wire.go:101-104` |
| #14217 is **merged as a design and not implemented**: no `Reasoning`, no `AnswerSource`, no `thinking` anywhere under `internal/` | `git grep` over `internal/`, 0 hits |
| `Limits.MaxOutputTokens` is recorded **once per run**; `Usage` is per call and carries **only** `inTokens` / `outTokens` — **no elapsed** | `types.go:209-223`, `types.go:166-170`, `types.go:265` |
| the judge failure line already carries model, endpoint, request bytes, elapsed, client bound | `openaicompat/client.go:70-74` |

**The line that decides the shape of D-1:** the run under investigation used **openai-compat** (#14622 §8).
#14624 option 4 reads *"as `internal/ollama/condense.go` already does"* — and that precedent sits on **the
other adapter**, on **the other call path**. On the adapter actually in service there is no key to set,
because the OpenAI chat-completions protocol does not define one. **Option 4 is not a config flip; it is a
capability question.** #14217 §3.4 says this in one sentence and it has not been acted on: *"The
openai-compat derivation arm is protected by nothing."*

### 3.4 Three inferences that are mine **[derived]**

**[derived-1] The shipped pair encodes an undeclared minimum of ≈ 14.6 tok/s.**
Inputs: budget 4096 tokens; client bound 300 s; prompt processing 19.7 s at 63 KB.
`4096 / (300 − 19.7) = 14.61 tok/s`. The isolated probe measured 18.2. **The shipped configuration has a
25 % margin against a number nobody wrote down.**

**[derived-2] The effective rate at operating context is below 14.6 tok/s — so the 18.2 probe overstates the
real cost by at least 20 %.**
Inputs: `max_tokens` is sent on every request, so generation cannot exceed 4096 tokens; P1 nonetheless had
not finished at 300 s on a 63,673 B request. Whether it had reached 4096 or not, time-to-4096 exceeded
`300 − 19.7` s, hence `r < 14.6`. **The consequence is the important half: any budget computed from 18.2
would still have failed.** This is the arithmetic reason #14624's option 2, *as stated*, loses.

**[derived-3] The derivation step's declared pair has never been satisfiable on a host of this class.**
Inputs: budget 4096 (the same constant, `derive.go:239`); bound 30 s. `4096 / 30 = 136.5 tok/s` — **7.5× the
measured rate.** The cap is a ceiling rather than a target, so the binding cost today is thinking (1,469
tokens ⇒ ≈ 81 s at 18.2 tok/s, **2.7× its own bound** — which is why it fails deterministically, 4 of 4).
But the *declared* worst case has been incoherent since the step was written, on every host this product has
ever run against.

### 3.5 A fourth inference, about the bounds themselves **[derived-4]**

The three bounds are not a hierarchy. They are three independent constants that do not nest:

| worst case inside one run bound | |
|---|---|
| derivation | 30 s |
| fills, `MaxFills = 2 × FillBound` | 180 s |
| judgement, `MaxModelCalls = 6 × 300 s` | 1,800 s |
| **total** | **2,010 s against a 600 s run bound — 3.35×** |

**So on any host where a judge call approaches its own bound, the effective call cap is 1, not 6, and nothing
in the product says so.** One judge call at 300 s plus two fills plus derivation is 510 s, leaving 90 s for
the five remaining calls. `MaxModelCalls = 6` is documented as *"the measured knee"* (`turn.go:17`) — a
property of model behaviour — and it silently is not the thing that ends a slow run.

This matters beyond bookkeeping: it means **a per-call budget cannot be chosen against the client bound
alone.** It has to be chosen against whichever bound will actually cut the call, which is the run bound once
a couple of calls have been spent.

### 3.6 What is **not** measured, stated so nobody reads a gap as a zero

- **The prompt length of the 18.2 tok/s probe.** Generation rate degrades with KV-cache length; a rate taken
  at a short prompt is not the rate at 16k tokens of context. [derived-2] says the operating rate is lower;
  it does not say by how much, or how much of the gap is context length versus draw variance.
- **n = 1** on the rate. Seed, temperature and repeat count are not on the record.
- **Whether the configured endpoint honours any thinking switch over openai-compat.** Unprobed. This is the
  gate on D-1 and §14 makes it step one.
- **Whether thinking-off costs judgement quality on a full 60 KB block.** #14130's 4–8× latency figures were
  taken on a short context-and-question prompt. The *direction* of the answer-size finding (same size, 4–8×
  the time) is all the evidence there is.
- **Whether any of this is a host property.** #14624's falsifier stands, unexecuted.

---

## 4. Assumptions & Constraints

| # | Constraint | Standing |
|---|---|---|
| **C-1** | A number measured on one host at one moment must not be hard-coded as a property | **#14624, binding** |
| **C-2** | A failure that reports one opaque code is not a measurement | **#11312 / #13534 §8, binding** |
| **C-3** | The restrictive environment is a premise, not a handicap; *"use a bigger model"* is not an answer | **#13534 §17.5, binding** |
| **C-4** | Smaller, more focused context — not bigger budgets | **#13534 §16.1, binding** |
| **C-5** | Both adapters must behave the same for the same endpoint behaviour; `what-the-adapters-may-share.md` §4 governs what may be shared | binding |
| **C-6** | `internal/loop` gains no provider knowledge | binding |
| **C-7** | Run-to-run reproducibility is what makes the measure-from-failures loop viable at all (#13534 §11) — a change that makes a run's behaviour depend on unrecorded history costs that | binding, and it is why §8.4 rejects an adaptive budget |
| **C-8** | Changing the model or the thinking mode makes prior figures incomparable, and that must be **stated, not discovered** | **#14624, binding — §10 discharges it** |
| **A-1** | The endpoint exposes *some* mechanism to suppress reasoning over at least one of the two protocols it serves | **assumed, unverified.** If false, §8.5's ladder ends at *"this host cannot serve this product"*, which is a legitimate outcome and is #14624's own falsifier clause |

---

## 5. Architectural Overview

The change introduces **one new idea and no new components**: an *affordability property* that every model
call must satisfy, checked where the configuration is read rather than where the call is made.

```
                       declared by the operator, reported at boot
                   ┌──────────────────────────────────────────────┐
                   │  floor generation rate  (tokens per second)   │
                   │  prompt allowance       (seconds)            │
                   └────────────────────┬─────────────────────────┘
                                        │
      ┌─────────────────────────────────┴──────────────────────────────┐
      │        AFFORDABILITY CHECK  —  boot time, no host required      │
      │                                                                 │
      │   for each call site:   budget/floor + prompt  <  bound × safety │
      └───┬───────────────────┬─────────────────────┬──────────────────┘
          │                   │                     │
    ┌─────▼─────┐      ┌──────▼──────┐       ┌──────▼──────┐
    │ derivation│      │  judgement  │       │    fill     │
    │ budget: S │      │  budget: M  │       │  budget: F  │
    │ bound: 30s│      │ bound: min( │       │ bound: 90s  │
    │           │      │  client,    │       │             │
    │           │      │  remaining) │       │             │
    └─────┬─────┘      └──────┬──────┘       └──────┬──────┘
          │                   │                     │
          └───────────────────┴─────────────────────┘
                              │
                   ┌──────────▼───────────┐
                   │  adapter — sends the │   thinking suppressed by the
                   │  budget it was given │   mechanism the protocol offers,
                   │  and no channel it   │   named at boot (§8.5 ladder)
                   │  cannot read         │
                   └──────────┬───────────┘
                              │
              success ────────┴──────── failure at a bound
                 │                              │
        record carries budget,          failure line carries budget,
        out-tokens and elapsed          floor assumed, elapsed, bound
        per call  (D-3)                 ⇒ the implied achieved rate
                                          is computable from the line
```

**The insight the diagram encodes:** *a token budget is a proxy for a time budget, and the proxy needs a
rate.* Today the product uses the proxy and never states the rate. This ruling makes the rate a **declared
property of the deployment** rather than a measured constant of the world — which is how C-1 is satisfied
without pretending the rate is unknowable. §8.6 names the change that removes the proxy entirely.

---

## 6. Components & Responsibilities

| Component | Owns | Changes | Does **not** own |
|---|---|---|---|
| **boot / configuration** | reading the deployment's declarations | gains the floor rate and the prompt allowance; **gains the affordability check and the boot line that reports its headroom**; gains the thinking-capability probe and reports which mechanism is in force | any call site's budget *value*, which belongs to the loop |
| **`internal/loop` — the budget owner** | each call site's output budget as a named, per-site value | `MaxOutputTokens` is replaced by one budget per call site; the loop performs the **pre-call remaining-time guard** | how a provider spells a thinking key, or a reasoning field (C-6) |
| **`internal/loop` — `derive.go`** | the query set, the window, and now the **provenance** of the query set | its budget is sized to its own output; its fallback becomes a recorded fact, not only a log line | whether the step runs at all — that is configuration |
| **`internal/ollama`** | ollama's wire vocabulary | carries the thinking key on the judgement request as it already does on condensation; accepts a per-call budget | what "off" *means* |
| **`internal/openaicompat`** | openai-compat's wire vocabulary | carries an **endpoint-specific extension** for thinking suppression, sent only when configured; accepts a per-call budget | pretending an extension is the protocol |
| **`internal/server`** | status codes and the envelope | **nothing** | — |

**Single-responsibility note.** The floor rate is a *deployment* fact and lives with configuration. The
budget is a *product* fact and lives with the loop. The mechanism for switching a channel off is a *protocol*
fact and lives in the adapter. No component owns two of the three, and that is what makes the ruling portable
to a second host.

---

## 7. The Decisions

### 7.1 D-1 — Thinking is off on every call, and the product proves it is off

> **The product does not read the reasoning channel. Until it does, it must not enable it.** Every request
> the product issues suppresses reasoning by whatever mechanism the protocol in use provides; the mechanism
> in force is named at boot; and the response path grows an assertion that nothing arrived on the channel
> that was supposed to be off.

**Why this is the primary and not one of the others.** It is the only remedy that removes cost rather than
redistributing it. The other four either buy less answer (2), buy more time (3), change what is being
measured (5), or make an answer readable that was never returned (1). **Reasoning tokens on this product are
pure waste by construction** — the response structs have no field for them at `e221b86`, so every one of them
is generated, paid for in wall-clock, and dropped. That is indefensible at *any* bound, on *any* host, which
is what makes it the decision that does not depend on the 18.2 measurement being representative.

**The magnitude, from #14130:** the judgement arm returns an answer of the *same size* 4–8× slower with
thinking on. The derivation arm spent 1,469 tokens on reasoning and returned empty content.

**The correction #14624's option 4 needs.** The precedent it cites is real and is on the **ollama** adapter's
**condense** path. The live configuration is **openai-compat**, where no such key exists in the protocol.
So D-1 is implemented as a **ladder**, ruled in order (§8.5), not as a flag.

**What it does not fix.** D-1 alone leaves the budget/bound pair unreconciled. A non-thinking model that
chooses to write 4,096 tokens of answer still costs 225 s at 18.2 tok/s and more at the operating rate. D-1
makes the shipped configuration *probably* affordable; D-2 makes it *provably* so.

**Cost, stated:** on the derivation arm, `think: false` produced the demanded six lines on **3 of 5** seeds
against 5 of 5 with thinking on (#14130). That is a real regression in format compliance — **against a step
that currently succeeds 0 of 4.** The comparison is 3/5 against 0/4, and #14217 §9 already owns the
format-shape question.

### 7.2 D-2 — Every call site declares its own budget, and the bound must afford it

> **`MaxOutputTokens` is retired as a single global constant. Each call site declares the budget its own
> output needs. Boot checks, for every site, that the budget is affordable inside the bound that will cut it,
> at a floor generation rate the deployment declares — and reports the resulting headroom. The loop performs
> the same check against the run's *remaining* time immediately before each judgement call.**

**The property, stated once:**

> **P1 (affordability).** For every model call the product can issue:
> `budget ÷ declared_floor_rate + prompt_allowance < bound_that_will_cut_it × safety_factor`.

**Why this satisfies C-1 and a smaller number does not.** 18.2 tok/s never enters the code. What enters is
(a) the **inequality**, which is a property and is checkable with no host present, and (b) a **declaration**
of what the operator asserts this deployment delivers, which lives in configuration, is reported at boot, and
is *expected* to differ between hosts. A host that cannot meet its own declaration is named at boot with the
number it would have to deliver — which is precisely the *"absence of a stated minimum"* that #14624's
falsifier says the task is about if the defect turns out to be a host property. **This ruling supplies the
stated minimum either way, so the falsifier's two branches converge on the same remedy.**

**Why the budgets must be per site, not one number lowered.** [derived-3]: the derivation step sends the
*judge's* 4096-token budget against a 30 s bound, implying a rate no local host of this class delivers. A
step whose output is six lines and a step whose output is a report do not have the same budget, and folding
them into one constant guarantees that at least one of them is wrong. The seam already exists on one side —
`ModelPort.Derive` takes a budget parameter; `ModelPort.Judge` does not and reads the package constant. **The
contract change is to make `Judge` symmetrical with `Derive`.**

**What the budgets should be, illustratively and not as constants to bake in.** Solving P1 at the *observed*
rate and against the bound that actually binds ([derived-4]: the run bound, not the client bound):

| site | output it actually produces | order of magnitude implied |
|---|---|---|
| derivation | six lines | low hundreds of tokens |
| fill / condense | one substance | sized from the node, per #14268 — which is a separate open defect and is **not** re-decided here |
| judgement | a report, or a tool call | **high hundreds, not four thousand** |

The judgement figure is the surprising one and it is worth stating plainly: **at this host's rate, a run that
is allowed six judgement calls inside a 600-second run bound can afford roughly 600–800 output tokens per
call, not 4,096.** That is not a preference; it is the arithmetic of the bounds the product already declares.
**The implementer derives the shipped values from P1 against the declared floor, and records the derivation —
they are not transcribed from this table.**

**The pleasing consequence, and it should not be oversold.** A judgement budget in the high hundreds is
*toward* #13534 §16.1's direction rather than against it. But §16.1 is about the **block**, not the answer,
and this ruling does not touch `AssemblyByteBudget`. The alignment is real and it is not a discharge of §16.1.

**The uncomfortable consequence, and it is an open question, not a hidden cost.** A report-form probe (§14571)
asks the model to *return relevant memories*. It is not established that a several-hundred-token answer is
enough for that task, and a processor arm that answers too briefly fails §17.5's bar as surely as one that
times out. §12 Q-2 carries this, and §8.4 names the lever if it binds.

### 7.3 D-3 — A failed call reports the rate it implies

> **Every model-call failure at a bound carries, in addition to what #11312 already delivered, the budget the
> call was issued with and the floor rate the configuration assumed — so that the rate the host actually
> achieved is computable from the failure line alone.**

**Why this is not a diagnostics nicety, for the second time.** #11312's whole finding was that the
information was in hand at the moment of failure and was thrown away. This is the same shape one layer out:
at 300.001 s the product knows the budget it asked for, the rate it assumed, the bytes it sent and the time
it took. Everything needed to say *"this host delivered at most X tok/s, against a configuration that assumed
Y"* is present, and today the operator is told only that a 5-minute bound was hit. **Recovering [derived-2]
took a human reading a node and doing division.**

> **P3.** A model-call failure at a bound is itself a rate measurement. Falsified by any timeout line from
> which the implied achieved rate cannot be computed without reading configuration held elsewhere.

**This is what makes C-1 survivable in practice.** The declared floor is a guess on a new host; the first
failure on that host corrects it, in the failure's own words. A configuration that is wrong fails *once*,
legibly, instead of four times, opaquely.

**The record half.** `Usage` carries in/out tokens per call and no elapsed (`types.go:166`). A successful
slow run is therefore as mute about rate as a failed one was. **`Usage` gains the elapsed time and the budget
that call was issued with** — which makes the rate derivable from every *stored* run too, and turns the
archive into a rate history nobody has to instrument for.

### 7.4 D-4 — The derivation step survives, right-sized, and its fallback stops being invisible

**The ruling: it survives.** Three reasons, in order of weight.

1. **The evidence for dropping it was produced by a retired instrument.** Its measured value — *"retrieved
   flat, admitted −0.09"*, #11288 arm D — is a `recall@k`-family score against the hand-pinned corpus that
   #13534 §16.3 **explicitly retired as the primary instrument**, on the grounds that it scores a living
   graph against one reading fixed at one moment. #13534 §16.3 also records that the corpus measured the
   relevant metric *flat across every arm including off*. **A step cannot be retired on a number from an
   instrument the product owner retired.** It can be retired on a judgement under the new instrument — which
   is phase 4, which has not run.
2. **The step is failing for two configuration defects, both of which this ruling fixes anyway.** Thinking is
   on (D-1; ≈ 81 s of reasoning against a 30 s bound — deterministic, which is exactly the 4-of-4 observed)
   and its budget is the judge's ([derived-3]). Dropping a step because a defect elsewhere makes it fail
   retires the wrong thing.
3. **It is the product's only answer to #13534 §3.** *"Optimising a measured quantity whose input is
   unmeasured"* is the anti-pattern the briefing exists to stop, and §4.2 was built to stop it. Removing it
   restores the anti-pattern by hand.

**But its cost must become conditional, and its failure must stop being silent.**

- **Budget sized to its own output** (D-2). A six-line answer does not carry the judge's budget.
- **The step is governed by P1 like every other site.** If its budget cannot be afforded inside its bound at
  the declared floor, **boot says so and the step is configured off** — rather than the product paying 30
  seconds per run to discover it at runtime, 4 runs out of 4.
- **The fallback becomes a recorded fact.** Today `turn.go:215` warns *"the query set fell back to the raw
  input alone"* into a log. The consequence is that **every retrieval figure taken on this configuration is
  silently a raw-input retrieval**, and #14622 §5 had to establish that by reading run logs. This is exactly
  the #13594 / #14217 family — *a successful run that withheld what it dropped* — and the remedy is the one
  that family has already ruled:

> **P4 (query provenance).** Every stored record states whether its query set was derived or fell back, and
> on what cause. Falsified by a stored record whose query provenance is recoverable only from the log.

**Rejected for derivation — raising its bound.** It buys a step of unmeasured value with time taken from the
same run bound that the judgement call cannot fit in ([derived-4]). At the current cost (≈ 81 s with thinking
on) the bound would have to triple, and the run would spend 13 % of its total budget before a single
candidate is retrieved. D-1 removes the cost instead of financing it.

**Rejected for derivation — a live rate check before running it.** It is §8.4's adaptive budget wearing a
gate, and it inherits the same objection: the run's behaviour becomes a function of the host's mood, which
costs C-7's reproducibility for a saving D-1 already delivers.

---

## 8. Alternatives Rejected

### 8.1 #14624 option 1 — decode the reasoning channel, as the remedy — **rejected as the remedy; re-ordered and re-motivated as a unit**

**Why it cannot be the remedy.** #14217's promotion mechanism recovers an answer that arrived on the wrong
channel of a **response**. P1 and P2 produced no response. There is no channel to promote when the HTTP call
died at the transport bound. The defect it fixes (2 empty answers in 12) and the defect that blocks phase 4
(2 timeouts in 2 on full blocks) are different failures with different causes.

**Why #14217's own recommended order is superseded, and it should be said out loud.** #14217 §11 recommends
*"this design first, #14130 second — the reverse order makes #14130 decide decision 1 under a risk this design
removes."* That reasoning was correct under #14217's evidence, where the risk of thinking-on was *an empty
answer one call in six*. #14622's evidence changes the risk to *the run does not complete*, and promotion
does not mitigate that. **The order inverts on new evidence, not on disagreement.**

**Why it survives, with a better job than it had.** Once thinking is off, a populated reasoning field is
**the proof that D-1's switch did not take effect** — which is far more valuable than a recovery path,
because D-1's mechanism on openai-compat is an unverified endpoint extension (A-1). #14217 §5.3 already
observed the free-assertion property on the condense path; D-1 generalises it:

> **P2 (no unread channel).** No request the product issues may enable an output channel the response path
> does not read. Falsified by a captured request that permits reasoning, or by a populated reasoning field on
> a run configured with reasoning off.

**So #14217 is not deferred — it is re-scoped from a recovery mechanism to a verification mechanism**, and
that is the honest reading of what it is worth after #14622.

### 8.2 #14624 option 2 — lower `MaxOutputTokens` — **rejected as stated; accepted as D-2**

**As stated** — pick a smaller global number that fits at the measured rate — it loses on two counts:

- **It bakes a host-and-moment measurement into a compile-time constant**, which C-1 forbids and which
  #14624 names as the trap.
- **It would not have worked.** [derived-2]: the real failures imply an effective rate below 14.6 tok/s at
  operating context, while the probe says 18.2. A budget sized at, say, 80 % of `18.2 × 280` would still have
  exceeded the wall. **A number derived from the probe is already wrong by at least 20 % and nothing in the
  product would have said so.**

**Accepted in amended form as D-2**, where the constant becomes a per-site declaration under a checked
inequality. The difference is not cosmetic: the amended form is falsifiable without a host, survives a change
of host, and reports its own assumption at boot.

### 8.3 #14624 option 3 — raise the client bound — **rejected**

Four independent reasons, any one sufficient:

1. **It destroys the distinction #11312 exists to preserve.** A slow run and a hung one become the same
   observation, and the project has already paid twice for that confusion (#11312's three instances).
2. **It works by giving the product more room**, which is the direction #13534 §16.1 rules against — and it
   would be conceding the *"dump-a-lot"* posture the product exists to challenge, at the level of time
   instead of bytes.
3. **There is no room to give.** [derived-4]: the run bound already cannot contain one judge call's own
   bound plus the fills. Raising the client bound does not raise the run bound; it makes the *outer* wall the
   one that cuts, which is a strictly worse failure because the run bound's message names no model call.
4. **It does not fix the thing.** With thinking on and no budget discipline, a reasoning model spends
   whatever bound it is given. Raising the wall moves the failure, and the next block that is 30 % larger
   reaches it.

**The one condition that would reopen it:** streaming (§8.6). Under streaming, a raised bound is safe because
liveness is observable — a silent connection and a slow one are distinguishable by construction. **The
prohibition on raising a bound is therefore not absolute; it is conditional on a mechanism nobody has built.**

### 8.4 An adaptive, rate-measured budget — **rejected**, and it is the closest call in this document

The attractive version: measure the achieved rate from each completed call and carry an EWMA into the next
call's budget, so the product tunes itself to any host and never hard-codes a rate at all.

**Rejected on C-7.** It makes a run's behaviour a function of unrecorded history. #13534 §11 established that
**retrieval reproducibility is what makes the whole measure-from-failures loop viable** — two runs of an
identical input shared 19 of 20 candidates and the same top hit to four decimals — and §14 added the rule
that *a measurement whose baseline lives in mutable shared state needs a control, not a tighter window.* An
adaptive budget puts the product's own behaviour into mutable shared state, **during the arc whose entire
purpose is an A/B comparison against a reference arm.** It is the right idea at the wrong moment.

**What would reopen it:** phase 4 complete, and a second host on the record showing that a single declared
floor cannot serve both. Then the adaptation has a measured need rather than an anticipated one, and P3's
failure-derived rate is already the data it would run on.

**The half of it that is accepted anyway:** the loop performs a **remaining-time guard** immediately before
each judgement call — if the run's own deadline cannot afford the declared budget, the call is not made and
the run terminates with a reason naming the arithmetic. That is deterministic given the same timings, it
degrades gracefully instead of dying at a wall, and it is the part that actually protects [derived-4]'s
third-call-onwards case.

### 8.5 #14624 option 5 — a non-reasoning model — **rejected as the remedy; escalated to Toni as §11**

**As a remedy it loses on C-8 alone:** every figure on record was taken on this family, and swapping the
model discards them all to achieve an effect D-1 achieves *within* the family. It is the largest blast radius
for an outcome available at a smaller one.

**But the two are closer than the list makes them look, and that has to be said.** A reasoning model with
reasoning suppressed is behaviourally a non-reasoning model. So D-1 is implemented as a ladder, and **the
ladder's last rung is option 5 by another name:**

| rung | mechanism | standing |
|---|---|---|
| **1** | **Native protocol.** The endpoint serves both protocols. Switch `PROCESSOR_MODEL_PROTOCOL` to the native one and carry the thinking key on the judgement request as the condense path already does. **A first-class protocol key, not an extension.** Configuration plus one field on one adapter | **preferred** |
| **2** | **Endpoint extension over openai-compat**, sent only when configured, **probed at boot** and reported: the boot line states whether the endpoint honoured it. P2 is the runtime check that it kept honouring it | acceptable; it must never be presented as protocol |
| **3** | **Neither is available** ⇒ the host cannot serve this product at these bounds with this model. **That is a stated minimum and a blocker to report**, which is #14624's falsifier branch, and #11312's standing rule about this host | the honest floor |

**Rung 1 is the recommendation**, and note what it costs: switching protocol changes the adapter under test,
and #14622's figures were taken on openai-compat. §10 books that casualty.

### 8.6 Streaming — **named, not scheduled; the only principled long-term answer**

Both adapters send `stream: false`. Under a streamed response:

- **The rate dependence disappears.** The product stops converting a time budget into a token budget and
  spends the time budget directly: stop generating when the time is gone. C-1 is satisfied *structurally*
  rather than by declaration, because no rate is ever assumed.
- **A slow run becomes distinguishable from a hung one by construction**, which is #11312's requirement met
  by the transport rather than by a message.
- **A partial answer survives.** Today a 299-second generation yields nothing; streamed, it yields 299
  seconds of answer and a truncation marker.

**Not scheduled**, because it rewrites both adapters' response handling and tool-call assembly across chunks,
and phase 4 is blocked now. **But it is the reopening condition for §8.3**, and it should be the first thing
reached for the next time anyone's answer to a bound is to move it.

### 8.7 Two options nobody listed, rejected in one line each

- **Shrink the block to make the call cheaper.** Real (§9.4) and owned by #13534 §16.1 / §16.5 rank 3. It is
  a retrieval decision reached for as a latency fix, and taking it here would settle §16.1 by side effect.
- **Retry the timed-out call.** Settled: `m1-skeleton-loop.md` §10.5, #14217 §7.2, #11312 *"a retry would have
  turned a fast clear failure into a slow one"*. A retry of a deterministic timeout is a slower timeout.

---

## 9. Quality Attributes & Trade-offs

### 9.1 What improves

| Attribute | How |
|---|---|
| **Availability of a completed run** | the binding cost (reasoning) is removed and the remaining cost is bounded by a checked property rather than by hope |
| **Diagnosability** | P3 makes every failure a rate measurement; P4 makes every success state its query provenance |
| **Portability to a second host** | the host's capability is declared, checked and reported at boot — the whole content of *"a stated minimum"* |
| **Falsifiability** | P1 is checkable with **no host present**, which is the first property in this area that does not need the model running |

### 9.2 What it costs

| Cost | Size | Mitigation |
|---|---|---|
| Answers may be **truncated** where they previously were not | unknown; a several-hundred-token judgement budget is a large change from 4,096 | `Truncated` is already a terminal reason (`types.go:134`) and is reported. §12 Q-2 measures whether it binds |
| **Judgement quality with reasoning off** is unmeasured on a full block | unknown; #14130's only evidence is a short prompt | phase 4's first arm measures it, with the confound declared (§10) |
| Derivation **format compliance** falls to 3/5 | measured, #14130 | against 0/4 today; #14217 §9 owns the shape question |
| **One more thing to configure** | small | boot reports it; a wrong value fails once, legibly (P3) |

### 9.3 The trade-off that is genuinely uncomfortable

**D-2 buys completion with brevity, and brevity is the axis phase 4 measures.** §17.4 names *size* as a KPI
and §16.1 says a smaller answer for the same substance is a *win*. But *too small to carry the substance* is
a loss that looks exactly like a win on that KPI. **The adjudicator of §17.6 compares information
sufficiency, not size**, so it is the right instrument to catch this — which is a reason to trust phase 4 to
detect it rather than to pre-empt it with a larger budget. **Stated, not hidden.**

### 9.4 A consequence for #13534 §16.5 rank 3, and it cuts in its favour

A smaller block reduces prompt-processing time directly (19.7 s at 63 KB, cold) and reduces generation-rate
degradation indirectly, since generation slows as the KV cache grows. **So the substance path is not only the
product's core concept — it is also a latency remedy for exactly the wall this ruling is about.** Magnitude
unmeasured; direction unambiguous. **This is a second, independent argument for rank 3 that the ranking did
not have.**

---

## 10. Comparability — Which Figures Die (C-8 discharged)

**Named here, before anyone discovers them later.**

| Change | Figures that die | Figures that survive |
|---|---|---|
| **D-1, thinking off** | every **latency** and **output-token** figure taken on `qwen3.8:27b` with reasoning on — including **#14247** and **#14249**'s 4,096-token burns, #14130's `think`-unset column, #14622's 18.2 tok/s *as a predictor of run cost*, and the empty-content rates (2/12, 2/5) | **retrieval and admission figures** — they are computed before the judgement call and do not depend on the model's reasoning. #13534 §11's reproducibility finding is untouched |
| **D-1 via ladder rung 1 (protocol switch)** | additionally, every figure taken **on the openai-compat adapter** for this model, including #14622's two failures as adapter-attributed facts | the loop-level figures, which are adapter-independent by C-5 |
| **D-2, per-site budgets** | **answer-length and answer-quality** comparisons across the change. A run that answers in 600 tokens is not comparable to one that answered in 4,096 | token *accounting* — `Usage` gains fields, loses none |
| **D-4, derivation actually runs** | **every retrieval figure taken on this configuration**, because all of them were silently raw-input retrievals (#14622 §5). This is a **gain**: they were measuring a fallback nobody chose | #11288 arm D's figure — which is *already* retired by #13534 §16.3 and must not be resurrected to argue either way |
| **Option 5, a different model family** | **everything.** This is the reason it is not the remedy | nothing model-dependent |

**The rule for the first post-change measurement:** phase 4's first result must state, in its own provenance
section, that it was taken with reasoning suppressed and per-site budgets in force — and it must **not** be
compared against the three newest archived runs on any latency or output-token axis. **It may be compared on
retrieval**, and it is the first run in this project's history for which that comparison means what it says,
because it is the first with a query set that was actually derived.

---

## 11. The Decision That Is Toni's

**Which model family this product targets.** #14624 puts option 5 on the list and it is not an engineering
call — it decides what every future figure is a figure *about*. Three coherent positions:

| | Position | Gains | Costs | Comparability |
|---|---|---|---|---|
| **A** | **Keep `qwen3.8:27b`, reasoning suppressed** | affordable within the family; §17.5's restrictive premise honoured; most recent figures share a family | reasoning is a capability being switched off without measuring what it was worth on a full block | recent runs survive on retrieval; die on latency |
| **B** | **Keep it, reasoning on, and finance it** | nothing is switched off; #14217's promotion recovers the empty draws | requires §8.3, which this ruling rejects, or streaming, which is unbuilt; run time triples | best — but the runs still do not complete today |
| **C** | **A non-reasoning model** (e.g. back to `qwen3-coder:30b`, which #14130 records as non-thinking and which served #13472's 15,304-token prompt in 87.9 s) | the largest and most certain effect; no dependence on A-1 | every figure on the current family dies; a second model-change later costs it again | worst against the three newest runs; **best** against the older archive |

**My recommendation: A**, with C as the named fallback if A-1 fails (§8.5 rung 3). A is reversible, keeps the
family, and reaches C's behaviour without paying C's comparability bill. **A and C differ mainly in whether
the switch is reachable** — which is a one-probe question, and §14 step 1 answers it before anything is built.

**Also Toni's, and unchanged by this ruling:** #14217 Q-A — *is a reasoning transcript an acceptable thing to
hand a caller as the answer?* Worth noting that **D-1 makes it moot in the default configuration**, which is
a reason to answer it later rather than now.

---

## 12. Open Questions

**Needing a measurement, not Toni:**

- **Q-1 (gates D-1).** Does the configured endpoint honour a thinking-suppression mechanism, over which
  protocol, and under what spelling? One probe. §14 step 1. **If the answer is *neither*, §11 collapses to C.**
- **Q-2 (gates D-2's values).** Is a several-hundred-token judgement budget enough for the report form of
  #14571's probes? The lever if it is not: `MaxModelCalls` is the *other* term in [derived-4]'s inequality —
  **fewer calls buy a larger budget each**, and a report-form task plausibly terminates in one or two calls
  rather than six. That trade is available and is not taken here for want of a measurement.
- **Q-3.** What is the generation rate **at operating context length**, with n > 1? [derived-2] says it is
  below the probe; nothing says by how much, and the declared floor should be set from this, not from 18.2.
  §14 step 0.
- **Q-4.** Does #14268 (the condensation budget sized from input length, charged for reasoning too) change
  once D-1 lands? It is the same mechanism on a third call site and is **not** re-decided here.

**Not knowable yet, and stated as such:**

- **Whether 18.2 tok/s is representative.** n=1, prompt length unrecorded, no seed. It is one draw.
- **What a second host would change.** Unknown, and #14624's falsifier is unexecuted. **But D-2 makes the
  question cheap rather than open-ended**: a second host declares its own floor, boot reports the headroom,
  and a host that cannot serve the configuration is named at boot instead of at 300.001 s. **The falsifier's
  two branches converge on this design either way** — if the defect is a host property, the remedy is a
  stated minimum, and §7.2 *is* the stated minimum.
- **Whether reasoning was buying judgement quality.** Nobody measured it on a full block, and D-1 switches it
  off. That is a knowing trade, taken because unread output cannot be defended at any price.

---

## 13. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| A-1 is false — the endpoint honours no thinking switch on either protocol | §8.5 rung 3: report it as a stated minimum and escalate to §11's C. Q-1 answers this before code is written |
| The extension key is sent, silently ignored, and the product believes reasoning is off | **P2** is exactly this guard: the reasoning field is read for the sole purpose of asserting it is empty. This is #14217 re-motivated |
| The declared floor is set optimistically on a new host | **P3**: the first failure reports the rate the host actually achieved. One legible failure, not four opaque ones |
| Per-site budgets are chosen from this document's illustrative table rather than derived | §7.2 states that they are **derived from P1 against the declared floor and recorded**. A reviewer checks the derivation, not the number |
| Phase 4 runs on the changed configuration and its result is compared against the archive | §10 is the pre-declared casualty list, and §10's last paragraph is the instruction for the run's provenance section |
| The ruling is read as permission to move a bound later | §8.3 is conditional on §8.6, and §8.6 is unbuilt. **Both halves must be quoted together** |

---

## 14. Implementation Guidance — Ordered Units

**No code in this document. Each unit is one PR, in this order, per the one-feature-one-PR rule.**

**Unit 0 — a measurement, not a change.** One probe at **operating context length** (≥ 60 KB prompt),
n ≥ 3, recording prompt tokens, completion tokens and elapsed separately. Answers Q-3 and supplies the number
the declared floor is set from. **No product change. Cheap, and it removes the largest unknown in this
document.** Run it alongside Q-1's probe in the same session.

**Unit 1 — thinking off, and proven off (D-1). This is phase 4's gate.**
1. Probe Q-1 against the configured endpoint over **both** protocols before writing anything.
2. Implement the highest rung of §8.5's ladder the probe supports.
3. Boot states which mechanism is in force, in one line, by name.
4. Add the response-side reasoning field on both adapters **for the assertion, not for recovery** (P2) —
   which is #14217 §5.3's adapter half, with the promotion half deferred to Unit 5.

**Unit 2 — per-site budgets and the affordability check (D-2).**
1. `ModelPort.Judge` takes its budget as an input, symmetrical with `Derive`.
2. One named budget per call site, each **derived** from P1 against the declared floor and the bound that
   binds ([derived-4]: the run bound, not only the client bound). Record the derivation.
3. Boot performs the P1 check over all sites and **reports the headroom per site**; it refuses, or warns
   loudly and names the number the host would have to deliver.
4. The loop's pre-call **remaining-time guard** (§8.4's accepted half).
5. `Limits` records the per-site budgets, not one `maxOutputTokens`.

**⇢ Phase 4 may run after Unit 2**, with §10's provenance statement in its result.

**Unit 3 — the failure reports the rate it implies (D-3).** The failure line gains budget and assumed floor;
`Usage` gains elapsed and the budget that call carried. Extends #11312's merged work; does not reopen it.

**Unit 4 — query provenance on the record (D-4 / P4).** Derived-or-fell-back, and the cause, on the stored
record rather than only in the log. #13594's family; small.

**Unit 5 — #14217's promotion half.** Now genuinely optional and correctly ordered: with reasoning off it is
a recovery path for a case that should not arise, which is the right time to build it and the wrong time to
prioritise it.

**Named, not scheduled — streaming (§8.6).** The reopening condition for every bound question.

### 14.1 What an implementer must not do

- **Do not transcribe any number from this document into code.** [derived-1] through [derived-4] are
  arithmetic on the record, shown so the reasoning is checkable. The shipped values come from P1 against the
  declared floor, and the floor comes from Unit 0.
- **Do not present the openai-compat extension as protocol.** It is an endpoint capability, probed and
  reported. C-5 governs the split; `what-the-adapters-may-share.md` §4 already rules where each piece lives.
- **Do not raise a bound.** §8.3, and its one reopening condition is unbuilt.
- **Do not drop the derivation step.** §7.4, and the number that would argue for it comes from a retired
  instrument.
