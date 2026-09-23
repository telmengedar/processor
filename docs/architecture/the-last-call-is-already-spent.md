# Architectural Document: The Last Call Is Already Spent

> **Intended repo path:** `docs/architecture/the-last-call-is-already-spent.md`.
> **Project:** #10422. **Evidence:** #14675 (the probe), #13091 (n=6 at the same cap), #13720 (two runs),
> #14250 (the nudge measured at zero).
> **Authority consumed, not superseded:** #14630 (`the-budget-and-the-bound-were-never-reconciled.md`) —
> §8.3 and §14.1 bind this document; #13534 §16.1, §16.4, §16.5, §17.5; #14561 §11 alternative 6;
> **#14540** (`a-run-that-got-nothing-behaves-differently.md`) — its P1 verdict, its P2 yield bound and its
> open **D-3** are what this document answers.
> **Baseline read:** worktree `feat/the-reclaimed-budget-needs-an-owner` at **`dbc7fa9`** (#14630's Unit 2,
> committed, **not merged**), whose parent is `main` = `d5dee97`. Every repo fact below was read at `dbc7fa9`
> and every one that differs on `main` is marked. **Nothing was run. No model call, no container, no graph
> write.**
> **No measurement in this document is mine.** §3 states whose each one is. Three arithmetic inferences are
> mine and each is marked **[derived]** with its inputs.
>
> **Revision 2, 2026-09-23 — three claims corrected against the implementation. The implementation is right and
> this document was wrong.** D-3's `capReached` is set **before** the reserved call, on the sole condition that
> the call budget is spent, so the `curtailed` population **widens** rather than keeping its membership. §1.3.3,
> §10 and §13 are restated; **F-4 is retained struck with its correction**, because its *diagnosis* was wrong
> and not only its prediction. **No decision changes.** What changes is what this document claims about them.

---

## TL;DR

**The ruling, in one line: the loop's last call cannot dispatch a tool, the loop has always known that, and it
has never told the model or used the call.**

#14675 reads as *the model ran out of calls researching*. That is true and it is not the finding. The finding
is one step earlier:

> **Under a cap of six, a tool requested on the sixth call is never dispatched. The sixth call is made anyway,
> at full prompt cost, and its entire output is discarded.** The product's own README already states the
> consequence — *"a judgement-call cap of 6, so at most 5"* tool rounds — and the loop nonetheless offers the
> model two tools on a call whose tool request it has already decided to throw away.

So the loop's *willingness to dispatch* and its *offer of tools* disagree on exactly one call of every
curtailed run, and that one call is the one that could have carried the answer.

**The remedy is therefore not a new bound, a new budget, or a new phase.** It is applying the predicate the
loop already evaluates — *am I still willing to dispatch a tool?* — at request-build time instead of at
dispatch time.

| # | Decision | |
|---|---|---|
| **D-1** | **The loop offers tools on a call exactly when it is still willing to dispatch one.** The last call of a curtailed turn is issued with no tools. | the whole design |
| **D-2** | **On a tool-less call, no adapter may return a tool-wanting terminal** — not from a native tool field, not from content recovery. | closes the hole that would defeat D-1 |
| **D-3** | **The record keeps saying research was cut short**, and `curtailed` keeps its population. Reservation must not silently empty a verdict. | comparability |
| **D-4** | **A reserved call that fails does not fail the run.** The design is strictly additive: its worst case is today's behaviour. | no regression below 0 bytes |
| **D-5** | **The answering call declares its own output budget**, under #14630's D-2 rule — and the affordability arithmetic caps it at ~253 tokens. | the uncomfortable one |

**Cost, measured rather than asserted: on every recorded capped run — seven of them, two models, three
configurations — reservation costs zero dispatched research rounds**, because the call it reserves is a call
whose output is already thrown away. §3.4.

---

## 1. Problem Statement

### 1.1 The stated problem

#14675: a probe completes, retrieves competently over five productive rounds, reaches the call cap with a
recall still pending, and returns **0 bytes** with verdict `curtailed`, `produced=false`, `grounded=true`.

### 1.2 The real problem, restated

The brief's first question is *"is this a failure of the loop or of the model?"* — and it is the right first
question, because the answer decides whether anything should be built.

**It is the loop's.** Not because the model behaved badly — it did not; #14675's five queries are distinct,
progressively narrower, and every dispatched round returned rows the model had not seen. A model that
researches until it is stopped is behaving correctly **if it has no way to know it is about to be stopped**,
and it has none: nothing in the prompt, the system text or the tool surface carries the call budget, its
consumption, or the fact that this call's tool request is pre-refused.

So the defect is an **information asymmetry the loop creates and then punishes**:

> The loop knows exactly how many calls remain. It knows, before it builds the sixth request, that the sixth
> request's tool call will be discarded. It nonetheless sends the same two tool schemas it sent on call one,
> and then discards what comes back.

That is the same shape as #14630's ruling one level over: **two quantities that were never reconciled.** There
it was `MaxOutputTokens` and the client bound. Here it is **the call budget and the answer** — the loop bounds
how many calls a run may make and has no property that any of them produces prose.

### 1.3 Success criteria

1. A run that reaches its call budget **makes its last call with no tool available**, so the model's only
   reachable terminal is prose.
2. Research rounds actually dispatched **do not decrease** in the cap case. (§3.4 shows this is free.)
3. The record still says research was cut short, and the `curtailed` population **never empties and never
   shrinks**. Archived records keep computing exactly the verdict they compute today (C-5). Among new records
   the population **widens**, by exactly the runs that reach their last call and would have answered on it —
   runs recorded `delivered` today. **The widening is forced, not incidental:** D-1 issues that call with no
   tool list, so *"would the model still have wanted a tool?"* — the observable the narrow definition was read
   off — no longer exists, and the structural definition is the only one available. §7.3 states it; §13 carries
   it as a comparability caveat.
4. Nothing in the mechanism depends on the model taking advice (#14561 §11 alt 6).
5. The design's failure mode is today's behaviour, never worse.

---

## 2. Scope & Non-Scope

### 2.1 In scope

- What the loop offers on the last call of a turn it will not let continue.
- How the three curtailing conditions — the call cap, the closed recall, the remaining-time guard — converge
  on one reserved answering call.
- What the record must carry so that reservation does not erase a verdict.
- The output budget the answering call declares, and what the affordability check permits it to be.

### 2.2 Out of scope — declined explicitly

| declined | why |
|---|---|
| **Raising `MaxModelCalls`** | #14630 §14.1 forbids raising a bound; §8.3's four reasons stand unchanged; and §3.5 shows the branch's own affordability check now refuses 7 arithmetically. |
| **Any change to retrieval, admission, the floor, the form rule or the block** | #14675 shows all of them working. #13534 §16.1's direction (smaller, more focused) is the agenda and is orthogonal: it changes how many rounds research *needs*, never what the last call is *for*. |
| **Telling the model its remaining budget** | §9 A-5. Compliance-dependent, the family measured at zero (#14250), and it adds a variable to phase 4. |
| **A planner that decomposes the task** | #13534 §16.5 names it open and unordered. A planner would change how the budget is spent; it does not change that the last call is spent on nothing. |
| **Detecting a claimed-but-absent tool action** | #14250's hallucinated write. Real, adjacent, unowned — filed as a follow-up in §19, not solved here. |
| **Streaming** | #14630 §8.6: named, not scheduled. It is the reopening condition for every bound question including this one. |

---

## 3. Measured Facts, and Whose They Are

### 3.1 Not mine — the operator's, at #14675, 2026-09-22

One probe, `main` = `d5dee97`, model `qwen3.8:27b` via `openai-compat`, write-suppressing port, receipt
`notStored`. Verdict `curtailed`, `produced=false`, `grounded=true`, `modelCalls=6`, `capReached=true`,
`answer` 0 bytes, `stopReason` `wantsRecall`, elapsed 129 s. Five dispatched rounds with yields 1, 1, 2, 1, 1
and admitted counts 2, 3, 3, 2, 3; the sixth refused at the cap. `recallClosed=false`, correctly.

### 3.2 Not mine — #13091's, 2026-09-07, and it is the only multi-run measurement at this cap

n=6, one task text, `qwen3-coder:30b` via ollama, `main` = `9d8da90`, same `MaxModelCalls = 6`.

| trajectory | runs | calls | terminal | `capReached` | answer |
|---|---:|---:|---|---|---|
| A | **4** | 6 | `wantsWrite` | **true** | **empty** |
| B | 1 | 5 | `answered` | false | prose |
| C | 1 | 6 | `answered` | false | prose |

Two facts carry forward. **Four of six ended at the cap with an empty answer** — and the terminal was
`wantsWrite`, not `wantsRecall`, so the shape is not recall-specific. **Trajectory C finished on the last call
the cap allows**, so the margin between a delivered run and a silent one was a single call.

### 3.3 Not mine — #13720's, 2026-09-12, two runs

Records #13718 (host) and #13719 (container), byte-identical behaviour. Six recalls, every one returning
nothing, `capReached=true`, `stopReason` `wantsRecall`, empty answer. This is the run pair that motivated the
yield bound, and it is the **opposite** shape from #14675 — barren where #14675 was productive — reaching the
**same** terminal.

### 3.4 Mine — read at `dbc7fa9` [derived-1]

`internal/loop/turn.go`'s judgement cycle increments the call count, makes the call, and then — when the model
wants a tool — dispatches **only if the count is below `MaxModelCalls`**. At the cap it records an undispatched
exchange carrying the cause *"call cap reached"* and breaks. The call was made; its request was discarded.

> **[derived-1] Therefore the last call of every capped run is, today, already a call whose tool request
> cannot be honoured.** Reserving it costs **zero dispatched research rounds**. This is arithmetic on the
> shipped control flow, not a prediction.

Checked against every recorded capped run: **#13091 trajectory A (4 runs)**, **#13720 (2 runs)**, **#14675
(1 probe)** — **7 of 7** ended with a final call whose tool request was never dispatched and whose answer was
empty. Two further recorded runs (#13091 B and C) reached their own terminal and are untouched by the change.

`README.md:265` already states the consequence — *"up to a judgement-call cap of 6, so at most 5"* rounds.
**The property is documented. The loop does not act on it, and the model is never told.**

### 3.5 Mine — the affordability arithmetic at `dbc7fa9` [derived-2]

Constants read: `RunBound` 10 min, `DerivationBound` 30 s, `MaxFills` 2 × `FillBound` 90 s, client bound
5 min, `JudgementBudget` 192, `JudgementPromptCeiling` 80 000 B, safety factor 0.8, product-declared floors
10 tokens/s and 3 000 prompt bytes/s. The judgement bound is the smaller of the client bound and the run
bound's per-call share after derivation and the fill phase.

| | cap = 6 (shipped) | cap = 7 |
|---|---:|---:|
| share of the run bound | 65.00 s | 55.71 s |
| allowed (× 0.8) | 52.00 s | 44.57 s |
| declared cost (19.20 s generation + 26.67 s prompt) | 45.87 s | 45.87 s |
| **spare** | **+6.13 s** | **−1.30 s** |

> **[derived-2] Raising the cap to seven makes the judgement site fail the affordability check the branch just
> shipped**, at the product's own declared floors, and boot would WARN naming the rate the host would have to
> deliver. The prohibition in #14630 §14.1 is therefore no longer only a policy: **the product now refuses the
> raise arithmetically.** Unit 2 turned `MaxModelCalls` from a dial into a term.

> **[derived-3] The most an answering call can be given at cap 6 and the declared floors is ~253 output
> tokens** — 52.00 s allowed less 26.67 s of prompt, at 10 tokens/s. `JudgementBudget` is 192, so the headroom
> is about 61 tokens. **The product can currently afford roughly one kilobyte of prose per run.** §7.5 and §17
> own what follows from that.

### 3.6 What is **not** measured, stated so nobody reads a gap as a zero

1. **Whether this model, on this configuration, produces usable prose when no tool is offered.** Zero
   observations. No judgement call has ever been issued without a tool list.
2. **Whether either endpoint accepts a request whose message history contains tool calls and whose body
   carries no tool list.** Zero observations. Both adapters mark the field omit-empty, so the shape is
   expressible; nothing has sent it.
3. **The remaining-time guard's firing rate.** It exists only on this unmerged branch. No record has ever
   carried a time shortfall.
4. **The rate at which runs reach the cap.** §3.2–3.3 give a *shape count* across three configurations and two
   models, not a rate. See §15.
5. **The 17-of-20 figure in the `MaxModelCalls` comment.** It entered with the 3→6 raise. I could locate no
   node carrying that measurement, and the only multi-run measurement at cap 6 (#13091) records 2 of 6. Filed
   in §19 rather than argued here; **nothing in this document rests on either figure.**

---

## 4. Assumptions & Constraints

| | |
|---|---|
| **C-1** | **A bound may not be raised** (#14630 §14.1), reopening condition streaming, unbuilt. |
| **C-2** | **No mechanism may depend on the model taking advice** (#14561 §11 alt 6 — *instructing the model is compliance-dependent and was measured at zero*). *The brief cites this as #14561 §6.6.9; §6.6.9 is a different rule, about the reference arm becoming the spec. The principle is unchanged and is cited here at its actual location.* |
| **C-3** | **The restrictive environment is a premise, not a handicap** (#13534 §17.5). A remedy that works by giving the loop more room is barred by §16.1 in the same breath. |
| **C-4** | **The adapters must not diverge in capability** without the split being ruled (`what-the-adapters-may-share.md`; #14630's C-5). This is why D-1 withholds the tool list rather than sending an openai-compat-only directive. |
| **C-5** | **The outcome verdict is recomputable from an archived record** (#14540 F-2). Any record-shape change must keep old records computable. |
| **A-1** | *Assumed:* an endpoint that is sent no tool list emits no native tool call. This is the mechanism's entire load-bearing assumption and §14 P-1 measures it before implementation. |
| **A-2** | *Assumed:* a model holding five rounds of retrieved rows and no tool can write something a reader can act on. **Unmeasured** (§3.6 item 1), and the honest statement of the risk is §16 R-2. |

---

## 5. Architectural Overview

Nothing new is added beside the loop. One predicate moves.

```
                       today                                    proposed
        ┌──────────────────────────────────┐      ┌──────────────────────────────────┐
        │ build request                    │      │ build request                    │
        │   tools: [recall, writeFile]     │      │   tools: offered ? [..] : NONE   │  <- the change
        │   ALWAYS                         │      │   offered = willing-to-dispatch  │
        └───────────────┬──────────────────┘      └───────────────┬──────────────────┘
                        v                                          v
        ┌──────────────────────────────────┐      ┌──────────────────────────────────┐
        │ model call                       │      │ model call                       │
        └───────────────┬──────────────────┘      └───────────────┬──────────────────┘
                        v                                          v
        ┌──────────────────────────────────┐      ┌──────────────────────────────────┐
        │ wants tool?                      │      │ wants tool?  (impossible when    │
        │   yes -> willing to dispatch?    │      │   no tool was offered - D-2)     │
        │          yes -> dispatch         │      │   yes -> dispatch                │
        │          NO  -> DISCARD, break   │      │   no  -> answer, break           │
        └──────────────────────────────────┘      └──────────────────────────────────┘
                        ^                                          ^
                        └── the wasted call                        └── the answering call
```

**`willing-to-dispatch` is not a new quantity.** It is the disjunction the loop already computes, one step
earlier:

| condition | today, evaluated after the call | proposed, evaluated before it |
|---|---|---|
| the call count has reached `MaxModelCalls` | discard the request, `capReached` | offer no tools |
| recall is closed on two barren rounds | one more call *with* tools, refuse whatever it asks | that call is the answering call |
| the remaining time cannot afford another call | break with no further call at all | if it affords exactly one, that one is the answering call |

**The three conditions converge on one mechanism.** That is the point: no fourth thing is added beside the
yield account, the verdict and the cap — the loop's existing *final-call* path is generalised to all three
triggers and made to withhold rather than to refuse.

---

## 6. Components & Responsibilities

| component | owns | does **not** own |
|---|---|---|
| **the judgement cycle** (`internal/loop`) | deciding, before each call, whether it remains willing to dispatch a tool; reserving the last call; recording which condition reserved it | what the model writes; whether the prose is any good |
| **the model seam** | carrying, per call, whether tools are offered | choosing when they are withheld |
| **each protocol adapter** | omitting its tool list when none is offered, and never returning a tool-wanting terminal from such a call (D-2) | the reservation policy, which is uniform across protocols |
| **the yield account** | closing recall on consecutive barren rounds | anything about budget consumption |
| **the outcome verdict** | reporting, after the fact, what the run obtained | intervening; it stays a pure function of the record |
| **boot affordability** | reporting per-site headroom, including the answering site | changing a budget at runtime |

---

## 7. The Decisions

### 7.1 D-1 — The loop offers tools exactly when it is willing to dispatch one

**Statement.** Before each judgement call the loop determines whether it would dispatch a tool requested by
that call. Where it would not, the call is issued **with no tool offered**, and the model's only reachable
terminal is prose. Where it would, nothing changes.

**Why this and not a threshold.** Every alternative in the brief is a choice of *when* to stop offering tools.
D-1 is the only one whose trigger needs no number: it is derived from bounds that already exist, and it
coincides exactly with the moment the loop has already decided the answer to that question. A threshold at any
other point would buy research rounds back at the price of a distribution nobody has measured (§9 A-4).

**Why it is compliance-independent.** A model that is sent no tool schema has no tool to call. The guarantee is
structural, at the wire, and does not rest on the model reading or accepting anything. C-2 is satisfied by
construction rather than by hope. **This is the mechanism #14540 §15 named as the designed escalation** — *the
only remaining mechanism that does not depend on the model's agreement* — applied in the mirror direction:
that document contemplated forcing a tool, this one forbids one.

**Why withholding and not a directive.** Omitting the tool list is protocol-uniform: both adapters already
mark the field omit-empty, so the shape costs no new capability and no probe, satisfying C-4. An
openai-compat-only directive would put the two adapters on different mechanisms for the same behaviour, which
is exactly the split `what-the-adapters-may-share.md` governs. The directive is named as the **fallback** if
P-1 shows an endpoint rejecting a tool-less request (§16 R-1).

**What it costs, stated precisely.** In the cap case: **nothing** — [derived-1]. In the recall-closed case:
nothing; that call is already refused today. In the time-guard case: **one research round**, and that is a real
cost carried in §12.2.

### 7.2 D-2 — On a tool-less call, no adapter may return a tool-wanting terminal

Without this, D-1 is defeated by the adapters' own recovery paths.

- One adapter **recovers a tool call from the response text** when the endpoint reports none. A model that
  writes a call-shaped object in prose would therefore still produce a tool-wanting terminal — and the loop
  would take the raw text as the answer, so the operator receives a JSON blob where the 0 bytes used to be.
  That is the *worse-than-nothing* outcome the brief bars, produced by the remedy itself.
- An endpoint could also return a native tool call despite being offered none.

**The rule:** a call issued with no tools yields prose. A tool-shaped payload arriving anyway is **recorded as
a fact about the response** and never becomes a tool request; the terminal is taken from the finish reason.
Content recovery is not attempted on such a call.

**Where the guarantee lives, and the limit of the guard that defends it.** *Stated 2026-09-23, from the
implementation branch as read by the operator — not from `dbc7fa9`, and not a measurement of mine.* D-2 is
enforced in **two layers of unequal strength**, and this document's other statements of it (§6's adapter row,
§10's `adapter → loop` invariant) read as though the guarantee were each adapter's to keep. It is not.

- **The loop is where the guarantee lives, and it is total.** `internal/loop/turn.go` refuses a tool-wanting
  terminal arriving from a reserved call, records the round as refused, and WARNs naming the adapter, the
  reserving condition and the tool that was asked for. It is keyed on **whether the call was reserved**, not on
  which adapter answered — so it holds for both shipped adapters, for any serialisation, and for a third
  adapter added later **by construction rather than by that adapter's compliance**. This is what makes D-2 the
  same kind of structural guarantee as D-1: it does not rest on anyone remembering it.
- **The adapter guard is a standing regression guard over one serialisation.** The openai-compat adapter has no
  shipped recovery path, and the guard defending that fires only if a future recovery parses the same
  call-shaped syntax its fixture uses. **A recovery keyed on a different serialisation, placed ahead of the
  withholding check, would pass it unnoticed.** The guard is therefore narrower than the claim it is named
  after, and its passing is weaker evidence than its name suggests.

**The consequence is a degradation, not a breach.** Because the backstop is total, the worst such a future
recovery can produce is a refused round, a WARN and no prose — **today's behaviour plus a log line, never a
dispatch**, which is D-4's property holding over a hole in D-2's other layer. That is why this is stated as a
limit here and filed as **#14709** rather than treated as a defect in the design.

**For whoever adds a recovery path later:** do not read the adapter guard as the thing that keeps D-2. Ordering
your path relative to the withholding check is load-bearing, and getting it wrong is caught by the loop —
silently, as a WARN on a run that produced nothing, which is the shape nobody goes looking for.

### 7.3 D-3 — The record keeps saying research was cut short

Reservation removes the *observable* on which curtailment is currently detected: at the cap the model can no
longer want a tool, so a naive implementation makes `capReached` unreachable, `curtailed` empty, and the
verdict silently changes meaning across the archive. C-5 forbids that.

**The rule:** curtailment becomes a **structural** fact rather than a behavioural one. The loop reserved its
last call for its own reasons; that is what is recorded, and it is exactly as deterministic as what is recorded
today.

- `CapReached` keeps its name and its mapping into `curtailed`; its definition sharpens from *"the cap was hit
  while the model still wanted a tool"* to *"the turn's last call was reserved because the call budget was
  spent."* It is the same event observed one call earlier.
- `RecallClosed` and the time shortfall are unchanged in meaning.
- The verdict formula is unchanged. **No new verdict term** — `curtailed` with `produced=true` is already
  fully legible under #14540 §6.2, and R3 of that document warns specifically against ossifying vocabulary.
- **One new record fact is needed and only one:** whether the reserved call was made and completed. §14 F-3
  must distinguish *the model produced nothing with no tool available* from *the reserved call never happened*,
  and no existing field separates them.

**What disappears:** capped runs no longer carry a final tool exchange bearing *"call cap reached"*. Any
analysis keyed on that string over new records reads zero. §13.

### 7.4 D-4 — A reserved call that fails does not fail the run

A reserved call is one more model call and can fail like any other. Today a judgement failure aborts the whole
turn with `model unavailable`, discarding the record. If the reserved call did that, a run that today returns
an honest `curtailed` verdict with 0 bytes would instead return an error — **a regression below the current
state, produced by the fix.**

**The rule:** a failed or rejected reserved call terminates the turn with the record the loop already holds,
exactly as today, and the failure is recorded as the reason no answer was produced. **The design is strictly
additive: its worst case is the present behaviour.** This is what makes it safe to ship ahead of the
measurement in §15 that cannot yet be afforded.

### 7.5 D-5 — The answering call declares its own output budget

Under #14630's D-2, every call site declares its own budget. The reserved call is a **different site** from a
research call: a research call emits a tool request of a few dozen tokens, and 192 was sized against a
population dominated by those. The reserved call is now **the only call in a curtailed run that produces
prose**, and it inherits a budget chosen for something else.

**The rule:** the answering call declares its own named budget, reported at boot with the other sites.
**[derived-3] bounds it at about 253 tokens** at the declared floors — so this is a declaration of roughly one
kilobyte of prose, not a free parameter.

**It is a declared constant, not a value derived at boot from the deployment's floors.** A boot-derived budget
would make two deployments produce different-sized answers for reasons unrelated to the change under test,
which breaks phase 4's comparability (#14630 §10) — and #14630 §8.4 already rejected making the product's own
behaviour a function of host measurements. Per #14630 §14.1, **the number is not transcribed from this
document**: it comes from Unit 0's probe against the declared floor, and the affordability check reports it.

---

## 8. Which Existing Mechanism Almost Covers This, and Why Each Falls Short

The brief asks this directly, and the answer is the reason no fourth mechanism is proposed.

| mechanism | how close it gets | why it falls short |
|---|---|---|
| **The yield account** | It is the loop's only existing *stop researching* rule, and it already sets a final-call flag. | It is keyed on **what comes back**; the defect is keyed on **what is left to spend**. #14675 had zero barren rounds, so it correctly did not fire — and a version that fired there would have cut off productive retrieval, which is worse. The two are orthogonal by construction and both are needed. |
| **The outcome verdict** | It named this failure correctly and unaided — `curtailed`, `produced=false`, `grounded=true` — which is exactly what it was built for. | It is a **pure function of the record, computed after the judgement cycle returns.** Making it intervene would turn a reporter into a controller and destroy its recomputability over archived records (#14540 F-2). Reporting honestly is its whole job and it did it. |
| **The call cap** | It is the closest by far: it already computes *may this request be dispatched?* and already answers *no* on the last call. | It answers that question **one call too late** — after paying for the call, and after telling the model it had tools. It bounds *how many* calls, and has never bounded *what the last one is for*. **D-1 is a repair to this mechanism, not an addition beside it.** |
| **The final-call flag on the closed-recall path** | It is already 80 % of the remedy: one more call, then stop, tool request refused. **This is the fourth thing, and it is already built.** | It is triggered only by barren yield, and it **refuses after the fact instead of withholding beforehand** — so the model still spends that call asking. D-1 generalises its trigger and fixes its mechanism. |
| **The remaining-time guard** (unmerged) | It is the only mechanism that already reasons about *affording another call*. | It affords **one** call and then stops, so a run it curtails also returns nothing. It needs to afford **two** while research is still permitted. §7.1 / §19 Unit 2. |

---

## 9. Alternatives Rejected

### A-1 — Raise `MaxModelCalls` from 6 to 7 — **rejected, and now arithmetically refused**

#14630 §8.3's four reasons stand verbatim: it destroys the slow-versus-hung distinction, it works by giving the
product more room (against #13534 §16.1), there is no room to give, and it does not fix the thing — the next
task that needs one more round reaches the new wall. **[derived-2] adds a fifth that is not a policy argument:
the branch's own boot check reports the judgement site unaffordable at seven**, because the cap divides the run
bound. Raising the cap makes each call *less* affordable. Reopening condition: streaming, unbuilt.

### A-2 — Keep the last non-empty prose seen in an earlier round and return it as the answer — **rejected**

Attractive because it is free and the text already exists: prose arrives alongside tool calls and the loop
**overwrites it on every call**. #13091 trajectory A shows what that text is:

> *"I'll generate a new barebones webpage and create a repository for it… Let me first check if there are any
> specific requirements…"*

That is a **statement of intent**, and promoting it to the answer presents intent as result — strictly worse
than 0 bytes, which at least cannot mislead. **Rejected as the answer.** Recording it as a diagnostic fact is
harmless and unowned; §19 files it rather than shipping it.

### A-3 — Budget calls per phase, N for gathering and M for answering — **rejected as unevidenced structure**

The only evidence about M is that **every observed answer arrived in a single call** (#13091 B at call 5, C at
call 6; every delivered archived run). So M = 1, and at M = 1 the design **is** D-1 with N = cap − 1 — the same
mechanism carrying extra vocabulary, an extra constant and a phase boundary that is itself a claim about how
the loop should work. **It is not wrong; it is D-1 with unearned machinery.** It reopens if a measurement ever
shows an answer needing two calls.

### A-4 — Refuse tools once a fraction of the budget is consumed — **rejected until someone measures the distribution**

Symmetrical with the yield bound and honest about it. It loses on the number: it needs a threshold, and the
only thing that would justify one is *the distribution of calls a run needs before it can answer*, which
nobody has measured. **D-1 is this option at the one threshold that requires no measurement**, because at that
threshold the round being withheld is a round that could not have been dispatched anyway. Any earlier threshold
trades real research for an unmeasured gain.

### A-5 — Tell the model how much budget remains, or that it should stop and answer — **rejected**

Compliance-dependent (C-2). The nudge family was **measured at zero** on this model class (#14250: three live
runs, three wordings, zero tool calls), and #14561 §11 alt 6 rules that *the loop's own statement about the run
is the loop's to make*. It also adds a prompt variable immediately before phase 4, which #14630 §10 exists to
prevent. **Not barred forever** — it may ship later as its own unit with its own falsifier, *after* D-1, so that
its effect is separable from the mechanism's.

### A-6 — Send a directive that forbids tool use, rather than omitting the tool list — **rejected as primary**

Same intent, but the directive is an openai-compat field the other adapter does not have, so it would split the
two adapters' mechanisms for one behaviour (C-4). Omitting the list is uniform and adds no capability probe.
**Retained as the named fallback** if P-1 shows an endpoint rejecting a tool-less request whose history
contains tool calls.

### A-7 — Fail the run instead of returning an answerless 200 — **rejected**

This is #14540's open **D-3**, and it is now answerable. Failing destroys the honest verdict #11312 and P1 were
built to produce, and makes a working-but-curtailed run indistinguishable from an outage — the exact confusion
#11312 exists to prevent, and #14630 §8.3 reason 1 again. **D-3 is hereby answered in the recommended
direction — return the partial with the verdict — and D-1 makes the partial worth returning.**

### A-8 — Change nothing; the verdict already reports it honestly — **rejected, and it deserves the hearing**

The verdict *is* honest, the operator *is* told, and #13534 §16.4 says the loop is the smallest of the three
problems. It loses on #13534 §17.5: the bar is **binary — does it work at all, or does it fail completely on
some class of task** — and a run that returns nothing fails it for whatever class this is. It also loses on
cost: the alternative is not "spend a call to maybe get an answer", it is "**stop discarding a call already
paid for**". A do-nothing that leaves a purchased call unused is not the cheap option.

### A-9 — Shrink the block so fewer rounds are needed — **not an alternative**

It is #13534 §16.1's standing direction and it is already the agenda. It changes how many rounds research needs
and never changes what the last call is for. **Orthogonal, not substitutable**; a smaller block plus D-1 is
strictly better than either.

---

## 10. Contracts & Interfaces (Abstract)

| contract | input | output | invariant |
|---|---|---|---|
| **loop → model seam, per call** | everything sent today, plus **whether any tool is offered on this call** | one judgement result | when no tool is offered, the result's terminal is never tool-wanting (D-2) |
| **adapter → wire** | the above | a request carrying a tool list only when tools are offered | the omission is the mechanism; no protocol-specific directive is required for the primary path |
| **adapter → loop, tool-less call** | the endpoint's response | prose, a terminal from the finish reason, usage, provider | a tool-shaped payload is recorded, never honoured, never promoted to the answer |
| **loop → record** | the turn's own decisions | today's fields, plus **whether the last call was a reserved answering call and whether it completed** | `curtailed` never empties (D-3) and every old record stays computable, unchanged, from its own fields (C-5). Its membership over **new** records **widens** by the runs that answer on a reserved call — forced by D-1, and carried as a comparability caveat in §13 |
| **boot → operator** | declared floors, call sites | one line per site including the answering site | unchanged in form from the shipped affordability report |

**No new closed set is introduced.** The terminal reasons, the verdicts, the write states and the tool names
are all unchanged.

---

## 11. Cross-Cutting Concerns

- **Observability.** One line when the loop reserves its answering call, naming which condition reserved it and
  how many rounds were dispatched. The existing non-delivery WARN is unchanged and still fires.
- **Error handling.** D-4. A reserved call that fails leaves the run exactly where it is today.
- **Determinism.** Reservation is a function of the call count, the yield account and the deadline arithmetic —
  all already deterministic. **No measurement of the host enters the decision** (#14630 §8.4 / C-7).
- **Security, concurrency, idempotency.** Untouched. No new I/O, no new persistence, no new external surface.
- **Cost.** In the cap case: zero additional calls, zero additional prompt bytes, zero additional latency. The
  reserved call replaces a call that is made today.
- **The prompt is unchanged.** No new sentence, no new nudge, no new instruction (A-5).

---

## 12. Quality Attributes & Trade-offs

### 12.1 What improves

1. A run that exhausts its budget **produces something**, which is the difference between the two sides of
   #13534 §17.5's binary bar.
2. A call the product pays for on every curtailed run **stops being discarded**.
3. The loop stops presenting a capability it has already decided to refuse — the honesty property this project
   keeps applying to its records, applied to its requests.
4. Three curtailing conditions get **one** terminal behaviour instead of three different ones.

### 12.2 What it costs

| | |
|---|---|
| **In the cap case** | Nothing. [derived-1]. |
| **In the recall-closed case** | Nothing; that call is refused today. One incidental behaviour is repaired rather than preserved: today a closed recall also refuses a *write* on the final call, which is unrelated to recall. Under D-1 the withholding is uniform and intentional. |
| **In the time-guard case** | **One research round**, genuinely. A run whose remaining time affords two calls will spend the second answering rather than researching. Unmeasured in frequency (§3.6 item 3), which is why it is a separate unit. |
| **A pending write is lost** | A run curtailed mid-write cannot make its last write — **exactly as today**, where that write is refused. Restated so nobody reads the change as causing it. |
| **Comparability** | §13. |

### 12.3 The trade-off that is genuinely uncomfortable

**A short, curtailed answer can be confidently wrong in a way 0 bytes cannot.** An operator reading ~1 KB of
prose composed from an incomplete research trail may act on it; an operator reading nothing investigates. The
verdict still says `curtailed`, and `curtailed` with `produced=true` is legible — but a verdict in a record is
weaker protection than an empty answer in front of a human.

**It is not mine to settle.** §17.

---

## 13. Comparability — Which Figures Die

| figure | fate |
|---|---|
| `stopReason` on capped runs | **Changes.** Today `wantsRecall` / `wantsWrite`; afterwards a terminal derived from the finish reason. Any comparison of stop reasons across the change is void. |
| the final tool exchange bearing *"call cap reached"* | **Disappears** from new records. Analyses counting it read zero rather than reporting a change. |
| `capReached` / `curtailed` membership | **Widens.** Today a run is `capReached` only if it reached the cap **and** still wanted a tool; afterwards it is `capReached` if the call budget was spent, full stop — set before the reserved call. So every run that reaches its last call joins, **including runs that would have answered on that call and been recorded `delivered`**. #13091 trajectory C — 6 calls, terminal `answered`, `capReached=false`, prose — is one, and is `curtailed` on this branch. **The widening is forced:** D-1 leaves that call no tool list, so the behavioural half of the old definition has no observable left, and the structural definition is the only one available. The two predicates are therefore **not the same predicate**, and any before/after comparison of `capReached` or `curtailed` counts compares two definitions — exactly as with `stopReason` above. D-3 guarantees the population does not silently *empty*; it does not hold membership fixed, and an implementation that held it fixed would have to reconstruct the observable D-1 removes. |
| the `delivered` rate | **Not comparable across the change** on any population that includes runs reaching their last call. Such a run is `curtailed` afterwards whether or not it produced prose, so a `delivered` count taken after the change is taken over a narrower definition than one taken before, and a drop in it is a relabelling rather than a loss. **Compare `produced`, or answer bytes, instead** — those mean the same thing on both sides of the change, which is the property `delivered` no longer has. |
| the empty-answer rate on capped runs | **This is the quantity under test.** It is not comparable across the change; it is the change. |
| output-token totals | **Rise** on curtailed runs, by up to one answering budget. Any before/after token comparison must say which side is which. |
| every retrieval, admission and block figure | **Untouched.** Nothing before the judgement cycle changes. |

---

## 14. Falsifiers — How We Would Know It Helped, as Properties

**Two run before implementation. Neither costs a graph write.**

- **F-1 — the premise, over the archive, zero model calls.** For every archived record with `capReached=true`:
  the final tool-call entry carries the cap cause and the answer is empty. **Prediction: all of them.** If any
  capped record carries substantive prose, the *"the last call is spent on nothing"* premise is weaker than
  §3.4 claims and the cost argument must be restated. **This is the F-1-shaped gate this project requires, and
  it is free.**
- **P-1 — the mechanism, two model calls.** One judgement call per adapter, issued with no tool list, against a
  message history that contains prior tool exchanges, on the measuring runner with writes suppressed. It
  answers three things at once: does the endpoint accept the request (A-1, §3.6 item 2); does the model return
  prose (A-2, §3.6 item 1); does the content-recovery path trigger (D-2). **Implementation does not start until
  P-1 has run.**

**After implementation, as properties on the record:**

- **F-2.** No new record may carry a tool request that was refused for want of budget — because none can be
  made. Mechanically checkable per record.
- **F-3.** `curtailed=true` with `produced=false` may occur **only** where the reserved call failed to
  complete. If a completed reserved call still yields no prose, **A-2 is false and the mechanism does not
  work** — that is the falsifier that sinks the design, and it is readable off a single record.
- **F-4 — the regression guard.** ~~No run that is `delivered` today becomes non-`delivered`. Should be
  vacuously true, since reservation touches only the last call of a run that is already curtailed; if it is
  not, something dispatches differently and the change is broader than designed.~~

  > **Corrected 2026-09-23, against the implementation — and the *diagnosis* was wrong, not only the
  > prediction.** Runs that are `delivered` today **do** become non-`delivered`, and that is correct behaviour:
  > `capReached` is now set before the reserved call whenever the budget is spent, so a run that reaches its
  > last call is `curtailed` whether or not it would have answered there. **#13091 trajectory C** — 6 calls,
  > terminal `answered`, `capReached=false`, prose — is `delivered` today and `curtailed` on this branch. The
  > document cites that run in §3.2 and still wrote a falsifier it violates.
  >
  > **The diagnosis is the part worth keeping visible.** F-4 as written told the reader that a firing meant
  > *something dispatches differently*. Nothing dispatches differently: trajectory C makes the same six calls
  > and the same five dispatched rounds, and the only change is which predicate labels the sixth. A reader
  > running F-4 as written would see it fire, go hunting a dispatch change that does not exist, and find
  > nothing. **A falsifier that fires for a reason its own text misattributes is worse than one that never
  > fires**, because it spends the reader's attention on a phantom and then exhausts their trust in the rest of
  > the list. It is retained struck for that reason and not because the prediction is interesting.

  **F-4 restated — keyed on content and on dispatch, never on the verdict label.** Three properties, each
  mechanically checkable per record:

  1. **No run produces less.** A run that produces prose today still produces prose: `produced` never goes
     true → false. **Relabelling is not a regression** — relabelling is D-3, and §13 carries it for phase 4.
     This is the property F-4 was reaching for and named wrongly.
  2. **No run researches less.** For any run that does **not** reach its last call, the dispatched round count,
     the call count and the terminal are unchanged. **This is where "something dispatches differently" is
     observable, and it is the only place it is** — reservation must be invisible to every run with budget
     left.
  3. **No call is reserved early.** A record carries a reserved call only where the budget was spent, recall
     was closed, or the time guard fired. A reserved call on a run that still had research budget is the shape
     that would make the change genuinely broader than designed, and it is the one to hunt.

**What no property can check:** whether the prose is *right*. A reserved answer that is fluent and wrong is
invisible to every mechanical test here. That is #13534 §17.6's adjudicator's question and therefore **a phase-4
finding, never a gate.**

**Phase 4's dependency, stated.** An arm that returns 0 bytes produces a report with nothing in it, and six
probes against it measure one fact six times. **This unit is phase 4's gate**, in the same position #14630's
Unit 1 held.

---

## 15. What One Probe Cannot Establish, and What the Measurement Would Cost

**#14675 is n=1** — one task, one graph state, one model, one protocol, one day. It establishes that *this* run
spent its whole budget on productive research and produced nothing. On its own it establishes nothing about
frequency, and a design justified by frequency alone would be resting on it.

**This design is not justified by frequency.** It is justified by [derived-1], which is arithmetic on the
shipped control flow and holds for n = 1 and n = 1000 alike: *the call being reserved is a call already thrown
away.* The probe tells us the case is live; the control flow tells us the remedy is free.

**What the records do give, and what it is not.** Seven recorded runs across three configurations and two
models ended at the cap with an empty answer, and two reached their own terminal. **That is a shape count, not
a rate** — the runs differ in task, model, date and code, so they cannot be pooled. Anyone quoting "7 of 9"
would be quoting a number this document does not assert.

**What would have to be measured before a *threshold* could be chosen honestly** — i.e. before A-4 or A-3
become available:

| measurement | why | affordable? |
|---|---|---|
| **the distribution of calls a run needs before it answers**, over a varied task set | the only thing that could justify refusing tools *earlier* than the last call | **No, not now.** #14561 §11 alt 4 prices this class: one endpoint, 20 s–5 min per run, tasks barred while a measurement is in flight; n=3 over eleven tasks was already declined at ~1.3 M input tokens. |
| **whether a model with no tool offered produces usable prose** (A-2) | the mechanism's load-bearing assumption | **Yes — two model calls.** P-1. |
| **whether the endpoints accept a tool-less request with tool history** | A-1 | **Yes — the same two calls.** P-1. |
| **the time guard's firing rate** | sizes Unit 2's real cost | **No** — it needs long runs at operating context, and the guard is not yet merged. Unit 2 ships on argument with its cost stated, or waits. |

> **The honest summary: the two things that must be true for this design to work cost two model calls, and the
> one thing that would let us pick a cleverer design cannot currently be afforded. That asymmetry is itself the
> argument for the design that needs no number.**

---

## 16. Risks & Mitigations

| | risk | mitigation |
|---|---|---|
| **R-1** | An endpoint rejects a request whose history carries tool calls but whose body carries no tool list | **P-1 measures it before implementation.** Fallback is A-6's directive on the protocol that has one; if neither works on an adapter, that adapter keeps today's behaviour and the boot line says so. |
| **R-2** | The model produces no prose even with no tool available, and the change achieves nothing | F-3 detects it off a single record. The change is still not a regression (D-4), so the cost of being wrong is the implementation, not the product. |
| **R-3** | A curtailed answer is fluent and wrong, and misleads an operator who would have investigated 0 bytes | §12.3, §17. Verdict unchanged and `produced` recorded separately; and it is escalated rather than decided. |
| **R-4** | The model claims a tool action it never took — #14250's hallucinated write, now **more** likely because the last call can no longer act | Real, and **not solved here**. The record carries the tool exchanges and the workspace, so the claim is detectable; nothing detects it today. Filed in §19. |
| **R-5** | Reservation empties the `curtailed` population and a later reader concludes curtailment stopped happening | D-3 is exactly this mitigation, and F-1's archive pass establishes the before-population. |
| **R-6** | The 253-token ceiling [derived-3] makes the answer too short to be worth producing | Real. It is §17's second question, and it is a consequence of bounds already ruled, not of this design. |
| **R-7** | This unit ships inert, like the five #14540 R4 names | It cannot: the change is on the request path of every curtailed run, and F-2 fires or the code did not land. **No new dial, no new constant that can be parked at zero** — except the answering budget, which boot reports. |
| **R-8** | Textual conflict with anything else editing the judgement cycle | Same file as #14630's Unit 2, which is unmerged on this branch. **Sequence, do not bundle:** this unit rebases onto Unit 2, never beside it. |

---

## 17. The Decision That Is Toni's

**Two questions, both recommended, neither taken.**

**1. Is a short, curtailed, possibly incomplete answer preferable to an honest zero bytes?**

This is a product question about what the harness is, and it is the same shape as #14540's D-1. **Recommendation:
yes.** #13534 §17.5 sets a binary bar — *does it work at all* — and a run returning nothing is on the wrong side
of it for whatever class of task provoked it. The verdict remains honest, `produced` remains recorded, and the
operator is not being told the run went well. **But §12.3's discomfort is real**, and if the answer is *no*,
this whole design reduces to filing the finding and D-1 is not built.

**2. Is roughly one kilobyte of prose enough for an answer to be worth producing?**

[derived-3] caps the answering budget near 253 tokens at the declared floors. That is a consequence of bounds
already ruled — the run bound, the cap, the prompt ceiling, the declared floors — and this design does not move
any of them. **Recommendation: ship it and let phase 4 judge the length.** *Good enough to solve the task given
a restrictive environment* is §17.5's own standard, and one kilobyte of grounded prose is a fair test of it.
The alternative — a larger answering budget — requires either a higher declared floor (a deployment
declaration, not a product change) or streaming (§8.6, unbuilt).

---

## 18. Open Questions

1. **Does the reserved call's prompt keep the full 80 KB block?** Assumed yes, because dropping it would change
   what the model answers from and make the answer incomparable to the research it did. Cheap to revisit once
   #13534 §16.1's smaller-block work lands; **not a dial to add now.**
2. **Should the record carry the prose seen on earlier rounds?** A-2 bars it becoming the answer. As a
   diagnostic field it is free and might be informative. **Deferred, not rejected** — it belongs to whoever owns
   R-4.
3. **What sources the `MaxModelCalls` comment's 17-of-20 figure?** §3.6 item 5. Nothing here rests on it, and a
   comment carrying an unlocatable measurement is a comment-contract matter.
4. **Does the time-guard variant (Unit 2) ship before it has ever been observed to fire?** §15 says its cost
   cannot be sized yet. Recommendation: ship it, because the property *every curtailed run gets an answering
   call* with a hole in one of three branches is the kind of almost-property this project keeps rejecting — but
   ship it **separately**, so the hole and the fix are distinguishable in the record.

---

## 19. Implementation Guidance — Ordered Units

**No code in this document. Each unit is one PR, in this order, per the one-feature-one-PR rule.**
**This unit stacks on #14630's Unit 2 (`dbc7fa9`), which is not yet merged. Rebase; do not bundle.**

**Unit 0 — the two measurements, no product change.**
1. **F-1** over every archived record with `capReached=true`. Zero model calls. Record the result before
   anything is written.
2. **P-1** — one tool-less judgement call per adapter, with tool history, on the measuring runner with writes
   suppressed. Two model calls, no graph mutation. **If P-1 fails on an adapter, stop and re-brief:** A-6's
   fallback changes the design's shape and is not an implementer's call.

**Unit 1 — the reserved answering call (D-1, D-2, D-3, D-4).**
1. The judgement seam carries, per call, whether tools are offered.
2. The loop determines willingness to dispatch **before** the call rather than after, and offers tools
   accordingly; the cap trigger and the closed-recall trigger converge on it.
3. Both adapters omit their tool list when none is offered, and **neither returns a tool-wanting terminal from
   such a call** — including the content-recovery path.
4. The record carries whether the last call was a reserved answering call and whether it completed;
   `CapReached`'s documentation is restated per D-3.
5. A failed reserved call terminates the turn with the record in hand (D-4).
6. One log line naming the reserving condition.

**⇢ Phase 4 may run after Unit 1**, with §13's comparability statement in its result.

**Unit 2 — the remaining-time guard reserves for two (§7.1, §12.2).** The guard permits a research round only
while remaining time affords the research call **and** the answering call; where it affords exactly one, that
one is the answering call. Arithmetic reported in the shortfall text as it already is.

**Unit 3 — the answering call's own budget (D-5).** A named site with a declared budget, reported by the boot
affordability check alongside the others. **The number comes from #14630's Unit 0 against the declared floor,
never from [derived-3]**, which is shown here only so the ceiling is checkable.

**Filed, not worked** — `task` nodes linked to #10422 and to this document, per #13534 §2:

- **#14697** — a model that claims a tool action it never took is undetected (R-4, #14250's second finding).
  More exposed after Unit 1, because the reserved call is precisely the one at which a model that wants to
  write cannot.
- **#14699** — the `MaxModelCalls` comment cites a 17-of-20 measurement that cannot be located (§3.6 item 5).
- **#14700** — the loop overwrites the prose written alongside each tool call, so a curtailed run discards
  every word it produced (§18 item 2). **Not** a proposal to return it as the answer — A-2 stands.
- **#14709** — D-2's adapter guard is keyed on **one serialisation**, so it is narrower than the claim it is
  named after; the withholding invariant — *a call issued with no tools yields a non-tool-wanting terminal* —
  should be asserted across response shapes against both adapters, in the shape `what-the-adapters-may-share.md`
  §3.5 already requires (§7.2). **Low priority by construction, and the reason is the point:** the loop's
  backstop is total and adapter-agnostic, so the gap degrades to today's behaviour plus a WARN rather than a
  dispatch, and the adapter it guards has no shipped recovery path. It is reachable only by a future change —
  which is exactly the change §7.2's stated limit exists to warn against.

### 19.1 What an implementer must not do

- **Do not raise `MaxModelCalls`.** #14630 §14.1, and [derived-2] — the branch's own boot check refuses it.
- **Do not transcribe any number from this document into code.** [derived-1] through [derived-3] are arithmetic
  on the record and on the shipped control flow, shown so the reasoning is checkable.
- **Do not add a sentence to the prompt.** A-5. The mechanism is the withholding; adding wording in the same
  unit makes the phase-4 comparison carry two variables.
- **Do not make the outcome verdict intervene.** It is a pure function of the record and must stay one (C-5).
- **Do not let the reserved call's failure fail the run.** D-4 is the property that keeps this change from ever
  being worse than today.
- **Do not leave one tool offered on the reserved call.** A reserved call that can still write is a call that
  can still end without prose, which is the defect wearing a smaller coat.
- **Do not start before Unit 0.** Both measurements are cheap and one of them is free.
