# Architectural Document: A Run That Got Nothing Behaves Differently

> Repo path: `docs/architecture/a-run-that-got-nothing-behaves-differently.md` · DiVoid node **#14540**
> (parity published at merge; this file is canonical until then).
> Project **#10422** · Product briefing **#13534** (§10's replacement test is the one this answers to).
> **Baseline: `main` at `6e132a3`, working tree clean.** Every repo fact below was read out of that tree.
> Evidence consumed: state assessment **#14524** · floor-shutout WARN gap **#14535** · nudge measured at
> zero **#14250** · the escalate branch that lies **#14242** · the reasoning channel **#14130** · the
> missing output instrument **#14525** · the task/terminal vocabulary ruling **#13065**.
> Prior designs consumed, not superseded: `reframe-before-you-escalate.md` (the nudge tier, PR #98) ·
> `retrieval-admission-and-the-empty-outcome.md` §8.2 (the unshipped `NeedsClarification` outcome) ·
> `what-a-successful-run-withheld.md` (#13674) · `what-the-adapters-may-share.md` (#13345).

---

> **Revision 3, 2026-09-22 — P1 is implemented and open as PR #102, and QA re-ran F-1 against the
> production computation rather than the reference script, reproducing it exactly (§13).** This revision
> settles the one question review raised that this document had left to inference: §6.4 now **decides** what
> becomes of the pre-existing shutout WARN, says what distinguishes the two alarms for an operator reading
> logs, and corrects a conflation in its own subsumption claim.
>
> **Revision 2, 2026-09-22 — F-1 ran against all 42 archived records before implementation, and it moved
> this document.** Two specification repairs in §6.2 (`curtailed` restricted to tool-dispatch bounds;
> `curtailed` ordered above `empty`), `acted` demoted from predicate to recorded fact, and four premise
> figures in §1 corrected in place with the originals struck. **F-1 now passes.** Method and result: §13.

## TL;DR

**The brief.** *"Does the loop requery or does it take insufficient information as given? We are in
exploration mode, there is no given truth — we can give out expectations, but they can fail and the
behavior could still be right."*

**The answer this design gives.** The loop's floor, its update window and its self-produced exclusion are
**expectations**. When one of them removes the entire population, that is evidence about the expectation,
not about the graph. A run that got nothing therefore does three things it cannot do today: it **re-admits
under the rule it can safely relax**, it **refuses a round that cannot add anything**, and it **states
mechanically what it achieved** — independent of what the model says it achieved.

**The constraint that shapes everything.** Three live runs, three different block states, three different
nudges, **zero `recall` calls in any of them** (#14250). So:

> **Compliance independence.** No mechanism in this unit may depend on the model taking advice for its
> value. Each one must be worth its cost even if the model ignores it entirely.

That rule is what separates this design from the nudge tier, and it is what disqualifies four otherwise
competent alternatives in §11.

**Three phases, in this order, each independently shippable.**

| | what it is | compliance-dependent? | verifiable today? |
|---|---|---|---|
| **P1 — the outcome** | the run states mechanically what it obtained, produced and did | no | **yes — run 2026-09-22 over the 42 archived records before implementation. It failed, moved the specification, and now passes (§13)** |
| **P2 — the no-gain round** | a recall round whose yield is empty is recorded as such and does not buy another round | no | yes, from the record (rounds and tokens not spent) |
| **P3 — the relaxed re-admission** | a rule that excluded everything is relaxed once, and the record carries both attempts | no | partly — the block stops being empty is measurable; *the answer is better* is not, and depends on **#14525** |

**Costs zero model calls and zero prompt growth.** The relaxed re-admission is a pure re-run of admission
over candidates already in hand; only the window relaxation costs one extra retrieval, and retrieval is
graph I/O, not model tokens. **The assembly byte budget is never raised by this design** — §9.

**Not in this unit, and why:** the reasoning channel (#14130), the discarded tool calls beyond the first,
and the semantic half of "did it do the thing" (#14525). §10 says which of the brief's five gaps ship
separately, and contests one of the brief's own nominations.

---

## 1. Problem Statement

A run that did not get what it needed is today **indistinguishable, on every surface an operator meets**,
from a run that did.

**Four measured instances, at `main` = `6e132a3`.** The figures below were corrected on 2026-09-22 after
F-1 ran (§13). The original readings are **struck rather than removed**: they were quoted forward from the
state assessment once already, and the next reader should be able to see that it happened.

| # | run | what happened | what the record said |
|---|---|---|---|
| 1 | **#14247** | all twenty candidates cut below the relevance floor; block carried the anchor plus *"Seems you know nothing about this topic…"*; model answered from parametric knowledge; **zero tool rounds**; claimed a file write that never happened | ~~`stopReason: answered`~~ → **`stopReason: truncated`**, raw `length`, and the answer is **4,193 B, not empty**. **No WARN at all** — `logFinished`'s rank-1 warning tests `cutForWantOfRoom`, which covers byte-budget and oversized and **not** below-floor (#14535) |
| 2 | **#13718 / #13719** | five `recall` calls differing only by a date suffix, **each returning the identical 5 nodes and the identical 19,662 bytes**, to the 6-call cap | `answer: ""` after **183,286 input tokens** each, with `capReached: true` and `stopReason: wantsRecall`. **The two records are byte-identical in every measured dimension** — same rounds, same included rows, same usage array. **One phenomenon sampled twice, not two data points** |
| 3 | ~~**#14249** and two others~~ → **#14249, alone** | empty answer having spent the whole 4,096-token output budget | ~~`stopReason: answered`~~ → **`stopReason: truncated`**. Exactly **two** records in 42 ever reached `MaxOutputTokens` — #14247 and #14249 — both `truncated`, and **only #14249 was empty** |
| 4 | ~~7 of 25 recent runs~~ → **1 of 42**, namely #13034 | `stopReason: "answered"` having produced nothing | the only record in 42 where the stop reason actively mislabels the run. **Six** runs produced nothing in total; **five of those six already carry a non-`answered` reason**, and **four of the six carry `capReached: true`** |

**The corrected figures weaken §6.1's original argument without weakening the phase**, and §6.1 is restated
at the strength the records actually support rather than quietly reworded.

**The common structure is not "the graph was empty."** In #14247 the graph returned twenty rows and a rule
refused all of them. In #13718 the graph returned five useful nodes and the loop paid for them five times.
In #14249 the model call completed and carried nothing across. In each case **a stage delivered less than
the run's own rules expected, and no component held both halves of that comparison.**

**Success criterion, stated as the goal state the brief names:** a run that did not get what it needed
behaves differently from one that did, the difference is visible to the operator without being told where
to look, and the loop acted on it before the operator ever sees it.

**The test this design answers to**, quoted rather than paraphrased (#13534 §10):

> *"if this ships, does the answer he gets get better — and would he be able to tell? The second half is
> not decoration."*

**P1 answers the second half outright. P2 answers the first half in the narrow, measurable sense of not
spending 183,286 input tokens on five identical result sets. P3 makes a plausible claim on the first half
that this repo cannot currently verify, and §13 says so rather than asserting it.**

---

## 2. Is "insufficient" one concept or four?

The brief asks it directly. **One concept, observed at three stages by three components holding three
different objects — plus a fourth thing that is not in this family at all.**

**The concept:** *a run must be able to distinguish what it obtained from what it assumed.* Insufficiency
is never a property of the graph. It is the relation between what a stage delivered and what the run's own
rules expected of it. That framing is Toni's, not a reading of it: expectations can fail and the behaviour
can still be right.

**Why it cannot be one detector.** The three situations are observed over different objects, at different
moments, by components with different information, and — decisively — **at different points in the spend**:

| stage | the object | who holds it | when | cost already sunk when it fires |
|---|---|---|---|---|
| **retrieval** | the candidate set and its dispositions | `Turn`, after `Assemble` | before the first model call | graph calls only — **zero model tokens** |
| **round** | this round's results against everything already shown | `Turn.judge`, after `dispatchRecall` | between model calls | one round, ~5,242 input tokens for the *next* one |
| **call** | the response text | the adapter/`JudgeResult` boundary | on every call | the full output budget |

A single detector would have to run after all three, which is after the money is spent. **Each detector must
sit where its remedy is still affordable.** That is the argument for three, and it is an argument about
economics, not taxonomy.

**The fourth situation — "it claimed an action it never took" — is not an insufficiency and is rejected
from this unit (§10).** It is a divergence between the model's prose and the record. Detecting it requires
reading prose; refusing an answer on a false positive is worse than the defect. It belongs to **#14525**,
and P1 supplies exactly the fields that instrument would otherwise have to infer.

---

## 3. Scope and Non-Scope

**In scope.**

1. What the loop does when its own admission rules excluded every candidate.
2. What the loop does when a recall round returned nothing the model does not already have.
3. What a run record and a run summary say about what the run mechanically obtained, produced and did.

**Out of scope, named so that declining them does not lose them.**

- **The relevance floor's value, the aperture's depth, the byte budget's size.** All three are measured,
  all three are demoted by #13534 §5, and none is touched here. This design *reads* the floor; it never
  re-argues it.
- **Compaction / substance-backed admission** (#11308, `substance-backed-admission.md`). The owner of the
  byte-budget cut. §9 refuses to relax the byte budget precisely so that this unit does not quietly
  become that one.
- **The nudge tier's wording**, including the branch that lies on a self-produced shutout (#14242). Still
  open, still owned there. This design **demotes the nudges' role** (§12) without editing them.
- **The reasoning channel** (#14130) and **the discarded tool calls** — §10.
- **Answer correctness, and whether a claimed action occurred** — #14525 tiers 2 and 3.
- **Query re-derivation as a remedy.** Measured to buy nothing (#11288 arm D: retrieved flat, admitted
  **−0.09**) and it costs a model call. §11 A4.

---

## 4. Assumptions and Constraints

| | |
|---|---|
| **C1** | `MaxModelCalls = 6` — at most five tool dispatches per run. Unchanged by this design. |
| **C2** | Prompt growth is **+5,242 input tokens per judgement round**; every prior tool result is replayed in full. **No mechanism here adds a judgement round.** |
| **C3** | Retrieval is `GraphPort` I/O only. `Retrieve` and `Assemble` make **zero model calls**. This is the design's central economic asset. |
| **C4** | `Assemble` and `RenderToolResult` are pure — no clock, no I/O, no turn state. `reframe-before-you-escalate.md` §6 made that a standing constraint. This design honours it: every new judgement lives in `Turn`. |
| **C5** | The block is built **once**, before the judge loop starts, and is fixed for the turn. A relaxed re-admission therefore has to happen **before** the first model call or not at all. |
| **C6** | Toni's standing direction: *"the whole idea of the project is to think in smaller steps, smaller but focused context."* §9 is this design's answer to it. |
| **C7** | The tool declarations (`recallTool`, `writeFileTool`) are **deliberately duplicated** across both adapters (#13345). Any change to a tool description lands in both, by design, and is not a DRY defect. |
| **A1** | *Assumed:* the 42 archived run records carry enough fields to compute P1's outcome retroactively. **§13 makes this a pre-ship check, not an assumption to build on.** |
| **A2** | *Assumed:* graph latency for one extra retrieval attempt is acceptable against runs that complete in ~21.6 s under a 5-minute model bound. Flagged as R6. |

---

## 5. Architectural Overview

```
                 ┌──────────────────────────────────────────────────────────┐
  input ────────>│  derive → Retrieve → Assemble                            │  zero model calls
                 │                         │                                │  (C3)
                 │              ┌──────────┴───────────┐                    │
                 │              │  EXCLUSION READING   │  P3 — new          │
                 │              │  admitted == 0 ?     │                    │
                 │              │  which rule did it?  │                    │
                 │              └──────────┬───────────┘                    │
                 │                         │                                │
                 │        ┌────────────────┼────────────────┐               │
                 │        │                │                │               │
                 │   window in force   all below floor   budget / self-     │
                 │   & nothing back        │             produced           │
                 │        │                │                │               │
                 │   drop window      re-admit without   NO RELAXATION      │
                 │   + re-Retrieve    the floor          (recorded)         │
                 │   (1 graph pass)   (pure, free)                          │
                 │        └────────────────┴────────────────┘               │
                 │                         │                                │
                 │                    final block                           │
                 └─────────────────────────┬────────────────────────────────┘
                                           v
                 ┌──────────────────────────────────────────────────────────┐
                 │  judge loop (<= 6 calls)                                 │
                 │    ┌─────────────────────────────────────────────┐       │
                 │    │  YIELD LEDGER   P2 — new                    │       │
                 │    │  what is already visible to the model:      │       │
                 │    │  block rows ∪ every prior round's rows      │       │
                 │    │                                             │       │
                 │    │  round yield = results \ ledger             │       │
                 │    │  yield empty  -> recorded, no gain          │       │
                 │    │  2nd empty    -> recall closed for the turn │       │
                 │    └─────────────────────────────────────────────┘       │
                 └─────────────────────────┬────────────────────────────────┘
                                           v
                 ┌──────────────────────────────────────────────────────────┐
                 │  OUTCOME   P1 — new                                      │
                 │  a pure function of fields the record already carries:   │
                 │  produced? grounded? acted? curtailed?                   │
                 │  -> one closed-set verdict, in the record, the summary,  │
                 │     and one WARN                                         │
                 └──────────────────────────────────────────────────────────┘
```

**Nothing in the diagram is a prompt.** The two render functions keep exactly the job they have.

---

## 6. P1 — The run states what it obtained, and it is not the model's word for it

### 6.1 What is wrong

`StopReason.Reason` is derived from the endpoint's `done_reason` plus the tool intent. `Answered` therefore
means **"the endpoint stopped normally and asked for no tool."** It says nothing about whether text came
back, whether any candidate reached the model, or whether any tool actually ran.

~~Hence `answered` on 7 of 25 runs that produced nothing.~~ **Corrected 2026-09-22, after F-1 ran against
all 42 records (§13): the true figure is 1 of 42** — #13034. At that rate *"the stop reason lies"* is not a
defect worth building a phase on, so the argument is restated rather than repaired.

**What the records do support is different, and stronger: no single field names the outcome, so the
operator has to synthesise one on every run.** Six runs in 42 produced nothing. On **five of those six the
stop reason is accurate** — `wantsRecall`, `wantsWrite`, `truncated` — and still does not say the run
achieved nothing, because *achieving nothing* is a conjunction of an empty answer with a bound that fired,
and **no field holds the conjunction**. Four of the six carry `capReached: true`, correctly, which is a
fifth field the reader must already know to consult. And #14247's `truncated` is exactly right while
nothing anywhere says its memory contributed zero.

So this phase is **not a correction to a lying field. It is the absence of a field** — and it is answerable
mechanically, because every input it needs is already recorded.

**The code already noticed this situation once and had nowhere to put it.** `summary.go` carries a literal
marker, `"  <-- terminal reason says answered"`, appended when the answer is empty and the reason is
`Answered`. That is an ad-hoc annotation of exactly the contradiction this phase promotes to a first-class,
computed field.

### 6.2 The shape

The record gains an **Outcome**: the loop's own account of the run, computed **only** from facts the loop
holds, never from the answer's prose. **Three predicates feed the verdict; a fourth fact is recorded for a
consumer outside this unit.**

| predicate | mechanically defined as | already recorded today? |
|---|---|---|
| **produced** | the answer contains **at least one non-whitespace character**. The whitespace clause is immaterial on the present corpus — no archived record carries a whitespace-only answer — and is pinned here so that implementation does not decide it | yes — `Record.Answer` |
| **grounded** | at least one candidate was admitted into the block, **or** at least one **dispatched** recall round returned an admitted row. **A round carrying an error contributes nothing**, which covers the cap-reached placeholder and P2's recall-closed refusal — neither was ever put in front of the model | yes — `Candidates[].Included`, `ToolCalls[].Results[].Included`, `ToolCalls[].Error` |
| **curtailed** | **a tool-dispatch bound ended the turn while the model still wanted a tool**: today `MaxModelCalls` reached with a pending tool request, and after P2 the recall-closed bound as well. **`MaxOutputTokens` is not one of them** — see below | yes — `CapReached`; extended by P2 |

**Recorded beside the verdict but consuming no rule: `acted`** — the count of tool rounds that completed
without error, by tool. The first draft called it a fourth predicate and **no verdict row consumed it**,
which F-1 caught. It stays on the record because **#14525's tier-1 instrument needs it** — crossing a
claimed action against a dispatched one is precisely that instrument's job — and it is *not* a predicate
here because this unit deliberately never reads the answer's prose (§2). Naming it correctly is the whole
fix; a fifth verdict resting on it would be #14525's work done in the wrong place.

**Why `MaxOutputTokens` is excluded from `curtailed` — decided on merit, before its corpus effect was
known.** A call cap that fires ends the turn **with the model's intent unsatisfied**: it asked for a tool
and was refused. An output bound that fires ends the **answer** mid-sentence while the turn itself
completed normally: the model was delivering, not asking. Those are different situations for an operator,
and they are **orthogonal** — a truncated answer can be grounded or ungrounded, and that distinction is
worth keeping. Truncation is therefore **not a verdict**. It is already carried, exactly and separately, by
`StopReason.Reason = Truncated`, so a record may read `ungrounded` **and** `truncated` — which is what
#14247 is, and that pair says more than either word alone could.

**The corpus consequence, stated as a consequence and not used as the argument:** folding the output bound
into `curtailed` makes `ungrounded` fire **zero times in 42 records**, because #14247 is its only member
and truncation would swallow it. That follows from the wrong definition; it is not the reason to reject it.

**The verdict** is a closed set, evaluated in this precedence so that exactly one applies. **The ordering
rule is: report the earliest-binding cause, never the most visible symptom** — each rung is *less
prevented* than the one above it.

| verdict | fires when | what it does **not** claim |
|---|---|---|
| `curtailed` | a **tool-dispatch** bound ended the turn while the model still wanted a tool | that the partial answer is useless, or that anything was produced — `produced` is recorded separately |
| `empty` | not **curtailed**, and not **produced** | that the model failed — the text may have been on an undecoded reasoning channel (#14130) |
| `ungrounded` | **produced**, and **grounded** is false | **that the answer is wrong.** #14247's shader was correct and useful |
| `delivered` | **produced** and **grounded**, no tool-dispatch bound fired | that the answer is right — nothing here scores that |

**`curtailed` above `empty` is a repair, and F-1 forced it.** The first draft ordered `empty` first, which
made #13718/#13719 — `answer: ""` **and** `capReached: true` — structurally unable to reach `curtailed`,
contradicting §13's own named expectation. No record can satisfy both orders, so one had to go.
**`curtailed` wins because it is the cause and `empty` is the symptom:** those runs are not empty because
the model had nothing to say, they are empty because the loop stopped them at the sixth call while they
were still asking. **A curtailed run with no text loses nothing by being called curtailed** — `produced` is
a recorded predicate, so `curtailed` with `produced: false` is fully legible on the record, and F-2
guarantees it stays recomputable. Under the repaired precedence the 42 records split **`curtailed` 4,
`empty` 2, `ungrounded` 1, `delivered` 35.**

**The swap takes nothing from the marker it replaces, and that is a contract rather than a coincidence.**
`CapReached` is true *only* when the call cap was hit while the model still wanted a tool — so
`stopReason: answered` and `capReached: true` **cannot co-occur**, an `answered` run having by definition
asked for nothing. **`curtailed` therefore cannot take a record that `summary.go`'s old `answered`-yet-empty
marker would have flagged**, and #13034, the only such record in 42, lands on `empty` under either order.
This is written down because it is exactly the sort of fact every future reader of the precedence table
otherwise re-derives from scratch.

**`ungrounded` is the load-bearing one and it is deliberately not an accusation.** #13065's correction is
binding here: the harness must not dress a defensible behaviour as a defect. `ungrounded` states one fact —
*this system's memory contributed nothing to this answer* — which is precisely the situation Toni wants
visible, and is orthogonal to whether the answer was any good.

**And it has a population of one, which has to be said plainly rather than left implicit.** #14247 is the
only produced-but-ungrounded run in 42. Two readings of that number, and neither is the comfortable one:

- *"The class is rare, so the verdict is not worth having."* **Refuted by when the corpus was taken.** The
  relevance floor shipped in PR #90 on 2026-09-16, and **all 56 below-floor cuts in the corpus come from
  the three runs after it.** Forty of the forty-two records predate the mechanism that produces this
  verdict's principal cause. Over the population that could exhibit it, the rate is **1 of 3**.
- *"So the rate is a third."* **Equally unsupportable.** Three runs is not a denominator anyone may quote
  forward, and this document does not. The honest statement is the narrow one: **the verdict has one
  instance, the corpus could not have held many more, and the next few post-floor runs decide it.**

**Kill condition, counted forward from a measured baseline rather than from a bare promise.** The baseline
is the complete non-`delivered` set at `6e132a3`, tabulated in §13: **`curtailed` #13031, #13040, #13718,
#13719 · `empty` #13034, #14249 · `ungrounded` #14247**, against 35 `delivered`. If `ungrounded` does not
fire again across the next ten runs taken after the floor, **demote it**: drop it from the verdict set,
keep `grounded` as a recorded predicate, and let `delivered` cover both. That costs one line and loses
nothing, because F-2 keeps every old record recomputable under the replacement vocabulary.

### 6.3 Where the verdict lands, and why not in the answer

Three surfaces, chosen against the measurement in `what-a-successful-run-withheld.md` §2 that *a new key
in a 70 KB JSON body is not a signal*:

1. **The run summary** (`summary.go`), which is what is written to the record node and what an operator
   actually reads. The existing ad-hoc marker collapses into this line.
2. **One WARN** in `logFinished`, on a non-`delivered` verdict, naming the verdict and the reason it fired.
3. **A top-level record field**, so the value is auditable and recomputable rather than only rendered.

**Not in the answer prose.** #14535 proposes *"say it in the answer, not only the log"* as the option that
touches the product, and it is right about the surface and wrong about the mechanism: putting it in the
answer means instructing the model to disclose it, which is compliance-dependent and was measured at zero.
**The verdict is the loop's statement about the run, and the loop is the party that can make it truthfully.**

### 6.4 This resolves #14535 better than widening the predicate

**First, a correction to this section's own claim.** It read *"#14535 names three shapes … the verdict
subsumes all three"*, which conflates two different threes. #14535's three **shapes** are three candidate
*remedies* — widen the predicate, add a distinct WARN, say it in the answer. What the verdict subsumes is
the three **cut reasons**, plus a fourth case none of the remedies covered. Stated correctly:

`ungrounded` fires on a floor shutout, on a self-produced shutout, on a byte-budget shutout **and** on a
run that retrieved nothing at all — one signal, with the discriminating facts recorded beside it. Of
#14535's three remedies it takes the third's **target** (the surface an operator actually meets) and
rejects its **mechanism** (§6.3: the answer's prose is the model's to write, not the loop's).

**Second — the question review raised, decided here rather than left to inference. Both existing WARNs
survive, and neither is redundant.**

- The **rank-1 `cutForWantOfRoom` WARN** is untouched. It answers *the best match was too big*, which no
  verdict asks.
- The **shutout WARN** — *"assembly admitted no candidate: the block carried the anchor alone"* — also
  survives. It fires alongside the new non-`delivered` WARN on a produced total shutout, and review asked
  whether that is one condition alarmed twice. **It is not: the two are not coextensive in either
  direction.**

| situation | shutout WARN | verdict WARN | what the pair tells the operator |
|---|---|---|---|
| retrieval returned **no candidates at all** | **silent** — its guard requires `len(Candidates) > 0` | `ungrounded` | the failure was upstream of admission; there was nothing to cut |
| candidates returned, all cut, **a supplementary recall then admitted a row** | **fires** | silent — the run is `delivered` | **the recovery worked.** Initial assembly failed and the run got there anyway |
| candidates returned, all cut, nothing recovered it | fires | `ungrounded` | assembly failed and stayed failed. This is #14247 |

**How that table was established, because only one of its rows has an instance.** The non-coextensiveness
is **structural, not empirical**: the shutout WARN reads `record.Candidates` behind a `len(...) > 0` guard,
and `grounded` additionally reads `ToolCalls[].Results[].Included`, so each can be true while the other is
false by construction. Checked against the 42 records, **row 3 has exactly one instance (#14247) and rows 1
and 2 have none** — no archived run retrieved zero candidates, and none was rescued by a supplementary
recall.

**Which alarm answers which question.** The shutout WARN is a **stage** alarm — *did initial assembly
produce a usable block?*, asked once, before the model ran. The verdict WARN is an **outcome**
alarm — *what did the whole run end up with?*, asked once, after every round. That is §2's partition
applied to the log rather than to the detectors, and it is why either can fire without the other.

**Row two is the load-bearing one, and it is why the shutout WARN must not be deleted as part of P1:** it
is the only signal that says *the initial block was empty and something rescued it*. **It has never fired
in 42 records**, and that is the point rather than an objection — **P2 and P3 exist to make it fire**, a
relaxed re-admission being precisely an initial assembly that admitted nothing followed by a run that
proceeds anyway. Deleting the stage alarm now would remove the instrument that shows P3 working, one phase
before P3 ships — and would do it on the evidence that the instrument has never yet had anything to report.

**What P3 owes it, and P1 does not.** Once the ladder can rescue an empty assembly, the shutout WARN's own
wording — *"the block carried the anchor alone"* — becomes **false on exactly the runs it is most worth
reading**, because the block will carry the relaxed rows. **Amending that sentence belongs to P3, in P3's
own PR**, beside the relaxation label it needs anyway. It is not folded into P1, which is merged-ready and
whose claim about the WARNs is correct as this section now states it.

---

## 7. P2 — A round that cannot add anything does not buy another round

### 7.1 What is wrong

#13718/#13719: five recalls, five identical result sets, **183,286 input tokens each**, `answer: ""`. The
model cannot see that it is repeating itself — `buildMessages` renders each round as its own
`assistant`/`tool` pair, and **identical results in a transcript read as confirmation, not repetition.**
The loop can see it and does not look.

**These are one observation, not two.** F-1 found the two records **byte-identical in every measured
dimension** — same rounds, same included rows, same usage array. P2 therefore rests on **one** sampled
phenomenon, and F-8 (§13) is the measurement that would give it a second.

### 7.2 The detector keys on results, not on query text

**This is the decision the evidence forces.** The five queries differed — by a date suffix — and the five
result sets were byte-identical. A detector keyed on query-string identity would have caught **none** of
the measured runs. So:

> **The yield ledger.** The turn accumulates the identity of every row already visible to the model: the
> rows admitted into the block, plus the rows returned by every prior round. A row's identity is
> `(id, contentHash)` — both already recorded on every `Disposition`. **A round's yield is the set of rows
> it returned that are not already in the ledger.**

A round with an empty yield added nothing, by construction, whatever its query said.

The hash arm is deliberate and its failure direction is deliberate: a node whose content changed between
rounds reads as **new**, so the loop errs toward granting the round. The opposite error — refusing a round
that would have brought genuinely fresh content — is the expensive one.

### 7.3 What the loop does about it

Two responses, and **only the first is compliance-independent, which is why it is the one that carries the
unit**:

1. **After the second consecutive empty-yield recall round, recall is closed for the turn.** Further recall
   requests are answered by the loop with a statement that recall is exhausted, without a graph call and
   without granting another round. The turn proceeds to a final judgement and terminates. The verdict
   becomes `curtailed`, and the record says which bound fired.
   **This holds whether or not the model cooperates.** On #13718 it converts five wasted rounds into two,
   and removes roughly three rounds of replayed prompt — on the order of 15,000 input tokens, in a run that
   spent 183,286 and returned nothing.

2. **The empty-yield round's rendered result states the fact**, in place of today's report sentence: this
   query returned rows the model has already been shown, and names how many. That is a **fact the model
   demonstrably cannot have** — it cannot compare its rounds — and is therefore categorically different
   from the nudges, which offered advice the model disagreed with. It is nonetheless compliance-dependent
   in its *effect*, and this design does not lean on it: it is a truthful rendering of state the loop now
   holds, and its value if the model ignores it is zero, which is acceptable because its cost is also zero.

**This also removes the remaining falsehood on the tool-result path.** `what-a-successful-run-withheld.md`
fixed *"no additional results found"* for the case where rows were found and all cut. The empty-yield case
is the same defect one level out: rows were found, some were admitted, and **every one of them was already
in front of the model.** Today that renders as a full `RESULT` section set — an affirmative presentation of
novelty that is false.

---

## 8. P3 — A rule that excluded everything is relaxed once

### 8.1 The principle

> **A total exclusion is a statement about the rule, not about the graph.**

A floor that cuts 20 of 20 has not established that the graph is empty. It has established that the floor
is mis-set for this input — which is exactly what #14250 measured: top similarity **0.6154** against a
floor of **0.63**, a **0.0146** miss, and the block went out carrying *"Seems you know nothing about this
topic."* The graph held twenty rows; the loop told the model it held none.

This is also the specific harm `retrieval-admission-and-the-empty-outcome.md` §8.2 predicted in advance and
forbade:

> *"Shipping a floor without a designed destination for its output would turn a wrong answer into a broken
> run."*

The floor shipped in PR #90. **The destination did not.** All 56 below-floor cuts in the 42-run corpus come
from the three runs since, and none of them warned. This phase is the destination, arriving late.

**One correction to that attribution, found by F-1.** #14247 did not only lose to the floor. Its record
carries a `derivationError` — the 30 s derivation bound expired — so it retrieved on **the raw input as its
only query**, and the twenty rows the floor then refused were the product of the weaker query set. Both
facts hold and the first does not displace the second: the floor refused rows that existed, *and* the
retrieval that produced them was degraded. **P3's claim on that run is bounded accordingly** — the floor
rung admits twenty real rows retrieved by a raw-input query, which is better than an empty block and is not
the same as admitting what a completed derivation would have found.

### 8.2 The relaxation ladder — deterministic, at most one pass

| the exclusion the loop reads | the relaxation | cost | why this one |
|---|---|---|---|
| **retrieval returned nothing** (or nothing that survives) **and a window was in force** | drop the window, retrieve again | **one retrieval pass** (graph I/O, zero model tokens) | the window is the single most model-derived expectation in the run: a date range inferred from the input's wording by the derivation call. It is the constraint most likely to be wrong and the only one that can empty *retrieval* rather than admission |
| **every returned row cut `below relevance floor`** | re-admit the same candidates with the floor removed | **free — a pure re-run of admission over rows already in hand, zero I/O** | §8.1. Rank order is preserved; only the confidence gate is dropped |
| **every returned row cut `self-produced`** | **none** | — | relaxing it re-opens self-poisoning (#11141). The loop instead records that the matching material exists and was excluded by policy — which is the true statement #14242 says the escalate branch currently gets wrong |
| **every returned row cut for `byte budget` / `oversized`** | **none** | — | the material is real, present, and too large. The owner is compaction (#11308), not this unit. §9 |

The ladder runs **at most once**: the window relaxation at most once, the floor relaxation at most once,
and the window relaxation may be followed by the floor relaxation when dropping the window yields rows that
are all below floor. **No chain, no loop, no retry budget** — there is no third rule this design is willing
to relax, so there is nothing for a chain to do.

**A failed derivation is a fourth failed expectation, and it gets no rung — deliberately.** The remedy
would be to re-derive, which §11 A4 rejects on measurement: it costs a model call, buys nothing (#11288
arm D, admitted **−0.09**), and on #14247 the bound it would run under had already expired once. So the
ladder does not act on it. What the exclusion reading **must** do is record it, so that an `ungrounded` run
whose derivation failed is distinguishable from one whose derivation succeeded. That needs no new field —
`Record.DerivationError` already carries it, on 2 of the 42 records — only that the reading names it.

### 8.3 What the block says about it

The block carries **provenance, not advice**: the rows were admitted under a relaxed rule, and the best
match scored *X* against a floor of *Y*. This is a statement of fact about how the block was built, in the
same category as the `id / type / name` header each section already carries.

**It is not the prompt patch #13065 forbids.** That ruling is against scolding the model into caution, and
its test is whether the mechanism's value survives the model ignoring it. Here it does: **the payload is
the rows.** A model that reads the provenance sentence and a model that skips it both receive twenty rows
of real graph content where they previously received an empty block and a sentence telling them the topic
is unknown.

### 8.4 What the record carries

The record gains an **attempt chain**: each retrieval/admission attempt with its full dispositions, its
relaxation label (or none), and which attempt produced the block.

**`Record.Candidates` keeps its present meaning — the dispositions of the attempt that produced the block.**
Additive only; no existing field changes meaning, and every existing consumer (`summary.go`,
`runbackfill`, the response body, the eval's record readers) continues to read what it reads today. This
preserves the retroactive-computability obligation `what-a-successful-run-withheld.md` places on the
record: the counterfactual — *what the unrelaxed pass would have produced* — stays computable from the
record forever, which is what makes P3 measurable at all.

Attempts add ~20 rows of metadata per attempt. They add **no content bytes**: the block is rendered once.

---

## 9. Why this is not "respond to insufficiency by pushing more context"

Toni's standing direction is *smaller but focused context*, and a design that answers emptiness with volume
is answering the wrong question. Three commitments, stated as properties so they can be checked:

1. **The assembly byte budget is never raised by this design.** The relaxed re-admission runs against the
   *same* `AssemblyByteBudget`. A relaxed block is bounded exactly as an ordinary block is, and a relaxed
   run's block is indistinguishable in size from an everyday successful run's.
2. **No mechanism adds a judgement round.** P2 *removes* rounds; P3 runs entirely before the first model
   call; P1 adds a computed field. Prompt growth (C2) is untouched or reduced.
3. **The direction of the change is from an empty block to a normal one, never from a normal block to a
   fuller one.** Every relaxation fires only on a **total** exclusion. On a run that admitted even one row,
   nothing in P3 fires at all.

**The aperture is not deepened and the budget is not raised** — both were measured and both are refused:
raising the budget buys exactly one row; deepening the aperture 20 → 100 moves admitted 0.39 → 0.43 with
all six new retrievals arriving `cut` (#14524 §2).

---

## 10. Which of the brief's five gaps belong in this unit

The brief nominates items 3, 4 and 5 as possible independent one-liners. **Two of those nominations are
right, and one of them is not a one-liner at all.**

| # | gap | in this unit? | ruling |
|---|---|---|---|
| **1** | identical recall repeated to the cap, nothing notices | **yes — P2** | it is the measured centre of the brief |
| **2** | a shutout taken as given | **yes — P3 + P1** | ditto |
| **3** | the reasoning channel neither requested nor decoded (#14130) | **no — ship separately, and it may ship first** | it is a **wire-format defect in the adapters**, orthogonal to loop behaviour: it changes what one adapter puts into the answer field and nothing else. **The two compose cleanly and neither blocks the other:** when the text is stranded on the reasoning channel, the loop genuinely received nothing usable, so P1's `empty` verdict is *correct* — fixing #14130 lowers the incidence without invalidating the detector. Designed and measured already; it needs implementation, not design |
| **4** | only `ToolCalls[0]` is read; the rest discarded | **split, and the brief's nomination is contested** | **(4a) recording the discard is a genuine one-liner and should ship now**: today the truncation is silent in both adapters, and one recorded count turns an invisible loss into a fact. **(4b) actually dispatching a batch is NOT a one-liner and must not be shipped as one.** `JudgeResult` carries exactly one call (scalar query/path/content) and `Turn.judge` dispatches exactly one per round; a batch changes the round accounting, the cap arithmetic (C1) and the prompt growth per round (C2). That is a design, and it is not this one |
| **5** | `stopReason: "answered"` cannot distinguish did-the-thing from did-nothing | **split — the mechanical half is P1; the semantic half is #14525** | the mechanical half (*was there output, was anything admitted, did a tool run*) needs no judgement and belongs here, because it is the instrument that makes P2 and P3 observable. The semantic half (*did the claimed action occur, does the answer hold*) requires reading prose and is **#14525's** |

**Nothing here is bundled for tidiness.** P1, P2 and P3 have disjoint seams — the outcome computation and
summary, the judge loop's round accounting, and the turn's pre-judgement assembly — and can be three PRs in
the order §14 gives. Items 3 and 4a share no seam with any of them.

---

## 11. Competent alternatives rejected

| | alternative | why it loses |
|---|---|---|
| **A1** | **Sharpen the nudges — a fourth tier, better wording** | Measured at zero: three live runs, three different block states, three different nudges, **zero `recall` calls** (#14250). The finding is not that the wording was poor but that *the model did not accept the premise* — it had the answer, so a sentence about the graph being empty had nothing to act on. Fails compliance independence outright. #13065 additionally rules that a prompt patch here **hides** the defect |
| **A2** | **Force the tool — `tool_choice` on a thin or empty block** | **The strongest rejected alternative**, and the honest fallback if P3 disappoints. It is the one mechanism that makes the model act regardless of its opinion. It loses on three counts: it spends a model call (C2) on a query the model composes from *the same input whose retrieval just failed*, and the dominant miss class is **vocabulary** (#11133) — the same model, same input, composes a near-identical query; there is no `tool_choice` anywhere in either adapter today, so it is a new mechanism, not a dial; and it removes the model's legitimate ability to answer directly, which #14247 shows is sometimes exactly right. **Named in §15 as the escalation if P3's falsifier fails** |
| **A3** | **Respond to a shutout by raising the byte budget or deepening the aperture** | Measured: the budget raise buys one row; aperture 20 → 100 moves admitted 0.39 → 0.43 with all six new arrivals `cut` (#14524 §2). And it is the exact shape C6 forbids |
| **A4** | **Re-derive the query and retry retrieval** | Costs a model call, and the uncontaminated arm D measured retrieved flat and admitted **−0.09** (#11288). The derivation also carries a 30 s bound it has missed live |
| **A5** | **`NeedsClarification` alone — the 2026-09-07 prescription** | **Consumed as an ancestor, not rejected as an idea.** Its diagnosis is right and this design descends from it. Where it loses is the mechanism: it makes the empty outcome something **the model elects to produce** and the loop then labels. P1 inverts that — the outcome is something **the loop observes** from facts it already holds. That inversion is what makes it compliance-independent, and it is why it can be verified retroactively over 42 records instead of requiring live runs to elicit the behaviour |
| **A6** | **Say it in the answer prose** (#14535 option 3) | Right about the surface, wrong about the mechanism: the model can only say what it is told, so this is a nudge wearing a product label. `what-a-successful-run-withheld.md` §1 already established the renderer is structurally incapable of disclosing a cut and the model was *never told* — telling it is the compliance-dependent step |
| **A7** | **One detector, one concept, one response** | It would have to run after all three stages, i.e. after the model budget is already spent. §2's table is the argument: the remedy has to sit where it is still affordable |
| **A8** | **Extend the nudge tier in `RenderToolResult` for the repeated-recall case, and stop there** | It states a true fact the model lacks, which is the good half — but it does not stop the token burn, and on the measured runs the burn is the damage. Kept as P2's *second* response, explicitly not load-bearing (§7.3) |
| **A9** | **Verify the claimed action inside the loop** — cross the answer prose against `ToolCalls` | Requires reading prose; a false positive refuses a correct answer. #14525 owns it, and P1 supplies its inputs |
| **A10** | **Relax the self-produced exclusion on a self-produced shutout** | Symmetrical with the floor relaxation and it loses on a different account: it re-opens self-poisoning (#11141), which is a measured, structural hazard rather than a mis-set confidence gate |

---

## 12. Where the judgement lives

**A different layer from the nudge tier, and the reason is structural rather than stylistic.**

The nudge tier lives in two **pure render functions** whose only output is text for the model and whose
entire effect is mediated by the model's compliance. A pure renderer cannot issue a retrieval, cannot
decline a round, and cannot compare this round against the last — it does not hold the objects. So the
mechanisms here **cannot** be an extension of it, whatever one thinks of the nudges.

| component | gains | keeps | does **not** gain |
|---|---|---|---|
| **`Turn`** (`turn.go`) | the exclusion reading and the relaxation ladder (P3); the yield ledger and the recall-closed bound (P2); the outcome computation (P1) | its existing responsibility for orchestration and I/O | any opinion about what the answer says |
| **`Assemble` / `admit`** | nothing | purity (C4). It is re-run with a different floor; it is not told why | knowledge of attempts, relaxations or rounds |
| **`renderBlock`** | one provenance line when the attempt was relaxed, supplied to it as data | its nudges, unchanged | turn state |
| **`RenderToolResult`** | one rendering for an empty-yield round, selected by a flag on the exchange | its cut-reason tiering, unchanged | the ledger itself |
| **`summary.go`** | the outcome line, absorbing the existing ad-hoc empty-answer marker | everything else | any new computation — it renders what the outcome already decided |
| **the adapters** | nothing in this unit | — | — |

**The nudges are not removed and not rewritten here.** Their *role* is demoted: from *the loop's response
to insufficiency* to *provenance on the block*. That demotion is stated rather than enacted, so that
`reframe-before-you-escalate.md` is not silently contradicted and #14242 keeps its owner.

---

## 13. What would show it worked — falsifiers as properties

**P1's falsifier has been run.** The verdict is a pure function of fields the record already carries, so it
was computed over the **42 archived run records** at `6e132a3` **before any code was written** — which is
exactly what §14's phase ordering exists to make possible.

> **F-1 (pre-ship, no new instrument required) — RUN 2026-09-22.** Computing the verdict over the existing
> 42 records labels **#14247 as `ungrounded`**, **#13718 and #13719 as `curtailed`**, and **every run that
> stopped as `answered` having produced nothing as `empty`**. If the vocabulary does not separate those
> runs from the runs that delivered, **the vocabulary is wrong and P1 does not ship as specified.**
>
> **Result: FAIL as §6.2 was first written; PASS after the two repairs §6.2 now carries.** Both readings of
> `curtailed` were tested against both precedences — four combinations — and **exactly one passes:
> `curtailed` restricted to tool-dispatch bounds, ordered above `empty`.** The other three fail: the
> original precedence cannot reach `curtailed` for #13718/#13719 at all, and the broad `curtailed` swallows
> #14247 so that `ungrounded` never fires anywhere in the corpus. Distribution under the passing
> combination: **`curtailed` 4, `empty` 2, `ungrounded` 1, `delivered` 35.** **No named run lands on
> `delivered` under any of the four** — the separation this falsifier actually asks about held throughout;
> what failed was the specification's internal consistency.
>
> **Two incidental results worth keeping.** Every field all four facts read is present on all 42 records,
> and `Fills`, `PayloadCap` and `RenderedSize` appear on **zero** — they postdate the corpus entirely — so
> F-1 needed **no schema-drift handling at all**, and neither will the implementation. One record carries a
> tool round with no `tool` name, which perturbs `acted`'s grouping and no verdict.
>
> This mirrors #14525's own falsifier — *if a tier-1 pass over the 42 records flags zero runs, the task is
> wrong* — and it is why P1 ships first: it is the only phase whose central claim can be settled against
> data this repo already holds. **It has now been settled, and it moved the specification.**

**F-2.** For every record, the verdict is reproducible from that record's own fields alone. Falsified by
any verdict that cannot be recomputed from the record — which would mean the loop knows something it did
not write down.

**F-3.** No run record contains three recall rounds whose yield was empty. Falsified by any such record.

**F-4.** No rendered round result presents rows as results when every row in it was already visible to the
model. Falsified by any record whose round rendering and yield disagree.

**F-5.** No record shows an empty block while carrying candidates that a named relaxation in §8.2's ladder
would have admitted, and no relaxation attempt. Falsified by any such record.

**F-6.** No relaxed attempt admits more content bytes than `AssemblyByteBudget`. This is §9 commitment 1
stated as a checkable property of every record.

**F-7.** Every relaxed run's record carries both attempts, so the unrelaxed counterfactual stays computable.
Falsified by any record carrying only the final attempt.

**F-8 (cost, P2).** Over a corpus of runs that repeat a recall, total input tokens strictly decrease and no
run's admitted row set shrinks. This is measurable from records alone, with **no model-in-the-loop
instrument**.

**F-9 (P3, mechanical — read the caveat before quoting it).** An `ungrounded` run whose candidates were cut
**for the floor alone** must become `delivered` under P3, because the relaxation admits those rows by
construction. **That measures the mechanism firing, not the answer improving.** `grounded` claims nothing
about correctness (§6.2), and a verdict moving `ungrounded` → `delivered` is a statement about what reached
the model. Anyone quoting *"P3 took `ungrounded` from 1 to 0"* as a quality result has committed #13534
§3's anti-pattern using this document's own instrument.

**The baseline all of these count forward from**, measured at `6e132a3` over all 42 records:

| verdict | records | n |
|---|---|---|
| `curtailed` | #13031, #13040, #13718, #13719 | 4 |
| `empty` | #13034, #14249 | 2 |
| `ungrounded` | #14247 | 1 |
| `delivered` | the remaining records | 35 |

**Reproduced independently at implementation time.** QA re-ran F-1 against the production computation
rather than the reference script and obtained this table exactly, with all four named records landing where
this section requires. The precedence mutant moves it to **`curtailed` 0 / `empty` 6** and reddens four
tests, so the ordering §6.2 repaired is pinned by the suite rather than merely asserted here.

### What this design cannot verify, stated plainly

**P3's claim that the answer gets better is not verifiable in this repo today.** `cmd/eval` sweeps
retrieval only, with zero model calls, and is structurally barred from the model adapters by its own
dependency-closure test — so a model-in-the-loop instrument is a new binary, not a flag (#14525). What P3
*can* prove from records alone is narrower and still worth having: **the block stopped being empty, the
rows admitted were real graph rows in rank order, and the counterfactual is preserved.** Whether those rows
improved the answer **depends on #14525 tier 2** and is not claimed here.

**Against #13534's test:** P1 answers *would he be able to tell* outright. P2 answers *does the answer get
better* in the measurable sense of not burning a run to nothing. P3 answers the second half and makes an
unverified claim on the first, which this section declines to assert.

---

## 14. Rollout — three phases, three PRs, this order

| # | phase | why here | gates on |
|---|---|---|---|
| **1** | **P1 — the outcome** | Nothing else is observable without it, and it is the only phase whose central claim (F-1) can be settled before implementation. It also partly supplies #14525 tier 1 from inside the loop, at the moment of the run rather than in a later sweep | nothing |
| **2** | **P2 — the no-gain round** | Strictly a cost reduction: it removes rounds and tokens and adds none. Its benefit (F-8) is measurable from records alone. It changes nothing the model sees except one truthful rendering | P1, so the saved runs terminate with a recorded verdict rather than a silent `answered` |
| **3** | **P3 — the relaxed re-admission** | The highest-value and highest-risk phase: it is the only one that changes what the model is given. It goes last so that P1's instrument is in place to tell whether it helped or hurt | P1 (the verdict), and nothing from P2 |

**Independently shippable alongside, in any order, sharing no seam:** #14130 (the reasoning channel) and
4a (recording the discarded tool calls). §10 explains why 4b is not among them.

**No phase ships with a zero-valued dial.** Five mechanisms merged between 2026-09-11 and 2026-09-17 and
every one ships inert or unobserved (#14524 §8). **This design introduces no new constant at all** — P3's
relaxations re-use the floor and the window the run already has, and the byte budget is unchanged — so
there is nothing to set to zero and nothing waiting on a measurement to be turned on.

---

## 15. Decisions that belong to Toni

Stated with options and a recommendation, so that none of them stalls the unit.

**D-1. Does an `ungrounded` run still return its answer?** #14247 produced a correct, useful Godot shader
with zero memory contribution. Options: **(a)** return it, marked `ungrounded` — *recommended*; **(b)**
return it with the answer itself caveated — rejected, compliance-dependent; **(c)** refuse it. This is a
product question about what the harness is: a memory-grounded assistant that declines outside its memory,
or a general assistant with memory. #13065's correction warns specifically against treating a defensible
behaviour as a defect, which is why (a) is recommended — but the call is his.

**D-2. Is the relevance floor a confidence statement or an admissibility statement?** P3's floor relaxation
assumes the first: a rule that refuses everything is mis-set for this input. The alternative reading is that
an empty block is the *correct* output when nothing clears the floor, and the honest response is to say so
and stop. Recommendation: the first, on the measured 0.0146 miss in #14250 — but if Toni holds the second,
P3 reduces to the window relaxation plus the record's honesty, and P1 and P2 are unaffected.

**D-3. Does a `curtailed` run return its partial answer or fail?** Today an empty answer is returned with
200. Options: return the partial with the verdict (*recommended* — the verdict is the signal); or fail the
run. This becomes live the moment P2 closes recall early.

**If P3's falsifiers pass mechanically and the answers still do not improve once #14525 exists, A2
(`tool_choice` forcing) is the designed escalation** — it is the only remaining mechanism that does not
depend on the model's agreement, and it should be designed then rather than guessed at now.

---

## 16. Risks

| | risk | mitigation |
|---|---|---|
| **R1** | The relaxed block floods the model with noise and the answer gets worse | The comparison is against an **empty** block, not a good one — P3 fires only on a total exclusion. Budget unchanged, rank order preserved, both attempts recorded so the counterfactual is computable (F-7) |
| **R2** | `ungrounded` reads as an accusation and a later reader treats a correct answer as a defect | §6.2 defines it mechanically and says what it does not claim; #13065 is cited in the field's own documentation, not only here |
| **R3** | The verdict vocabulary ossifies a wrong distinction — #13065's exact hazard | Every predicate is recorded beside the verdict, so a later vocabulary is recomputable over old records. F-2 makes that a property, not an intention |
| **R4** | This unit ships inert like the previous five | No new constant and no new dial (§14). F-1 proves P1 fires before it is written |
| **R5** | The relaxation masks a genuine *nothing is here*, converting an honest empty into a confident wrong | The relaxed rows are real graph rows, never invention; the verdict and the relaxation label are on the record; D-2 is the decision that owns this trade-off explicitly |
| **R6** | The window relaxation doubles graph load on affected runs — up to 7 recalls at 100 rows each, twice | It fires only when retrieval returned nothing, so the first pass was cheap by construction. Bounded to once. The floor relaxation costs **no** I/O at all. A2 records this as an assumption to re-read if run latency regresses |
| **R7** | The yield ledger refuses a round that would have helped | The hash arm fails toward *new* (§7.2), and the bound is two consecutive empty-yield rounds, not one |
| **R8** | P2 and P3 both touch `turn.go` and will conflict textually with anything else editing it | Disjoint sites, same file — the pattern `what-a-successful-run-withheld.md` already sequenced. §14's order is the sequencing |

---

## 17. Open questions

1. **Does #14525's tier-1 instrument read P1's verdict or recompute it independently?** Independence argues
   for recompute; cost argues for read. F-2 makes either workable. Needs deciding when #14525 is designed,
   not here.
2. **Should the empty-yield rendering name the rows it is suppressing, or only their count?** Naming them
   costs bytes in a prompt this design otherwise only shrinks. Leaning to count plus ids, no content.
3. **`recall`'s one-sentence tool description omits that a semantic query cannot match a date** — a property
   the harness states verbatim in its own derivation prompt and never tells the answering model. Correcting
   it is a contract correction rather than a nudge, and near-free (two duplicated declarations, C7). It is
   nonetheless **compliance-dependent**, so it is named here rather than folded into a phase; the operator
   may ship it alone. Its falsifier: the rate of recall queries containing a date literal, measurable from
   existing records before and after.
4. **Whether the attempt chain belongs in the HTTP response body at all**, given the finding that a new key
   in a 70 KB body is not a signal. It costs ~20 metadata rows per attempt. Leaning to yes, on the grounds
   that the response is an archive, not a signal, and the signal is the verdict.

---

## 18. Implementation guidance — architectural milestones

**Phase 1 — P1, the outcome.**

1. ~~Run F-1 against the 42 archived run records **first**~~ — **done, 2026-09-22 (§13).** It failed as the
   first draft of §6.2 was written, and §6.2 now carries the two repairs under which it passes. **Re-run it
   if §6.2 changes again**; it is cheap, needs no model, and is the only check that can precede the code.
2. Add the outcome — **three predicates**, the derived verdict, and `acted` as a recorded fact — as a
   computation over the record, sited so that it is a pure function of the record and nothing else (F-2).
3. Land it on the record, on the summary (absorbing the existing ad-hoc empty-answer marker), and as one
   WARN on a non-`delivered` verdict.
4. Leave the existing rank-1 `cutForWantOfRoom` WARN untouched (§6.4).

**Phase 2 — P2, the no-gain round.**

1. Introduce the yield ledger in the judge loop: the rows visible to the model, seeded from the block's
   admitted set, extended by each round's returned rows.
2. Record each round's yield on the round's record entry.
3. Close recall for the turn after the second consecutive empty-yield round, and mark the turn curtailed by
   that bound rather than by the call cap.
4. Give an empty-yield round its own truthful rendering, selected by a flag on the exchange, not by new
   logic inside the pure renderer.

**Phase 3 — P3, the relaxed re-admission.**

1. Introduce the exclusion reading between assembly and judgement: zero admitted, and which rule accounts
   for it.
2. Implement the ladder of §8.2 exactly, including the two rules that are **not** relaxed and are recorded
   as such.
3. Re-run admission (floor case) or retrieval-then-admission (window case), once each at most.
4. Extend the record with the attempt chain, leaving `Record.Candidates` meaning what it means today (§8.4).
5. Supply the relaxation provenance to the block renderer as data; the renderer gains no state.

**Throughout:** no new constant, no new dial, no change to `AssemblyByteBudget`, `MaxModelCalls`,
`RelevanceFloor`, `CandidateLimit` or `MaxOutputTokens`. If a phase appears to need one, that is a signal
the phase has drifted into the demoted tuning work of #13534 §5.

---

# Operator decisions, 2026-09-22 — the three open questions, resolved

The design left three questions to Toni with recommendations attached. Under #1176 Rule 9 a recommended option is taken rather than queued, and recorded here so the choice is answerable later. **All three take the design's own recommendation.** Each is cheap to reverse; if Toni prefers otherwise the reversal is named.

## 1. Does an `ungrounded` run still return its answer?

**Chosen: yes, returned and marked.**

**Why:** #14247's shader was *correct* — produced from parametric knowledge with an empty block. Withholding it would have destroyed a good answer to make a point about provenance. The verdict's job is to say *where the answer came from*, never to judge whether it is right; the design says so in terms, and `ungrounded` deliberately claims nothing about correctness.

**Alternatives:** *withhold and fail the run* — loses correct answers, and makes the harness's confidence in its own retrieval a precondition for the model being allowed to be useful. *Return without marking* — the status quo, and the exact shape that misleads an operator fastest.

**Reversal cost: cheap.** The verdict is already on the record; suppressing the body is a later decision at the response boundary.

## 2. Is the relevance floor a confidence statement or an admissibility statement?

**Chosen: a confidence statement — relaxable when it excludes the entire population.**

**Why:** this is settled by Toni's own direction of the same day (#13534 §16.3), and the design reached it independently:

> *"We are in exploration mode, there is no given truth — we can give out expectations, but they can fail and the behavior could still be right."*

**The floor is an expectation.** A threshold that removes twenty rows of twenty has produced evidence about itself, not about the graph — which is the principle the whole design rests on. #14250 sharpens it: the top candidate missed by **0.0146**. Treating 0.6154 as categorically unusable while 0.63 is usable asserts a precision the number does not have.

**Alternatives:** *admissibility — an empty block is the correct output* — defensible, and it is what ships today; it makes the floor a claim about truth rather than about confidence, which is the frame §16.3 retires. If Toni holds it, **P3 reduces to the window relaxation and P1/P2 are unaffected** — the design is built so this choice does not cascade.

**Reversal cost: cheap**, and deliberately so: the relaxation is one arm of a deterministic ladder, fires only on a total exclusion, and is pure with zero I/O.

## 3. Does a `curtailed` run return its partial answer or fail?

**Chosen: return it, with the verdict.**

**Why:** #13718/#13719 returned `""` after 183,286 input tokens — a failure that also destroyed whatever the run had. Returning the partial keeps the evidence; the verdict is what stops it reading as a clean success. Failing the run would re-create the defect #11312 was raised to fix: an outcome that reports one opaque code when everything needed to explain it was in hand.

**Alternatives:** *fail with an error* — loses partial work and forces a full re-run at full cost. *Return silently* — the status quo, and indistinguishable from success.

**Reversal cost: cheap** — a branch at the response boundary, not a change to the detector.

---

**Note on sequencing:** P1's falsifier **F-1 runs before any production line is written** — the verdict is a pure function of fields the records already carry, so it is computed over the 42 archived records first. If F-1 does not separate the named runs, P1 does not ship as specified and these three decisions are moot until the vocabulary is fixed.
