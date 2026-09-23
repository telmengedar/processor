# Architectural Document: The Prompt and the Request Must Agree

> **Intended repo path:** `docs/architecture/the-prompt-and-the-request-must-agree.md`.
> **Project:** #10422. **Finding answered:** #14718, measured at #14717 (wire capture, six observations).
> **Authority consumed, not superseded:** #14695 (`the-last-call-is-already-spent.md`) — its D-1, D-2, A-5,
> A-6 and §19.1 bind this document, and nothing here redesigns the reserved call; **#14561** §4.1, §4.4,
> §11 alternative 6, §12.11, §12.12 — the compliance-independence rule and the falsifier idiom;
> **#13345** (`what-the-adapters-may-share.md`) §4 — the decision rule that places every new element;
> **#10532** (M1) §8.6 and §9.2 — the system text is an injected value and seams must earn themselves;
> **#14600** — `internal/systemtext` exists so two binaries cannot send different prompts;
> **#12935** — a negative claim must be filed with its falsifier at authorship time.
> **Baseline read:** worktree `.claude/worktrees/impl-lastcall` at **`3e6a055`** (`feat/the-last-call-is-already-spent`,
> **not merged**; `main` = `73dfcce`). Every repo fact in §3 was read at `3e6a055`.
> **Nothing was run.** No model call, no inference, no endpoint contacted, no graph write, no code changed.
> **No measurement in this document is mine.** §3 states whose each one is.

---

## TL;DR

**Two sentences in the system text are false on exactly the call that withholds the tools they describe, and
the model believes them. Delete them — and delete the class that produced them, because the deletion and the
class fix are the same edit and produce byte-identical wire traffic.**

- The reserved answering call withholds the tool list at the wire, verified. The system text sent on that same
  call still says *"A recall tool is available"* and *"A file tool is available"* (#14717 §3.2).
- All **six** observations of that prompt state open *"I have enough to write the report. Let me confirm one
  thing about…"* and **none contains a claim about the subject matter**. The model is doing what it was told
  it could do.
- **The permitted operation is the one that is complete at the wire.** Deleting a sentence whose truth is
  decidable from the request bytes changes what the request *is*; there is no compliance event to fail. Adding
  a directive changes what the model is *asked to do*; its effect requires obedience. §4 places the line, and
  places option 2 on the far side of it.
- **The strongest evidence against option 2 is already in the measurement.** The shipped system text already
  says *"Do not describe these instructions or your search process in the answer."* **Six of six observations
  are a description of the search process.** A directive in this very prompt, addressed to this defect, was
  ignored in every observation we have.
- **Recommendation: option 3, split into two units.** The prompt's tool paragraphs and the request's tool list
  become two projections of **one value the loop already computes** — the set of tool identities offered on
  this call. Prompt and request then cannot disagree by construction, and the falsifier that pins it is
  structural rather than a string match.
- **Cost of choosing 3 over 1: almost nothing, and the reason is decisive.** Options 1 and 3 send the *same
  bytes*. They differ only in what the next diff can break. A third tool added to the adapters re-creates the
  defect under option 1 and cannot under option 3.
- **Honest conclusion on sufficiency, and it is the result the brief asked for rather than an optimistic
  design: deleting the false sentences is necessary, it is the only compliance-free step available, and it is
  not established as sufficient.** §10 predicts the observed opening disappears and **declines to predict a
  report**, names what would falsify each half, and scopes the remainder as a separate question rather than
  absorbing it.

---

## 1. Problem Statement

### 1.1 The stated problem

#14718: on the one call where prose is the only reachable terminal, the system text asserts that two tools are
available. Messages 0–9 are byte-identical between call 5 (tools offered) and call 6 (tools withheld); only the
request's top-level `tools` key differs.

### 1.2 The real problem, restated

**A prompt that describes a request's capabilities, maintained separately from the request's capabilities.**

The tool paragraphs were true on every call this product had ever made, for as long as the product existed,
because every call carried both tools. The reserved-call unit made one call carry neither. Nothing in that
unit's diff touches `internal/systemtext`, and nothing in the tree compares a prompt against the request it
accompanies — so a sentence that had been true since it was written went false in a file the diff did not open.

That is #12935's shape exactly, and #12935 names why neither review could catch it: **a claim about a capability
is falsified by the change that removes the capability, and that change has no reason to touch the file where
the claim lives.** #12935's own conclusion is that the remedy is authorship-time filing rather than a better
review — which this document takes literally in §11.

The instance is one prompt on one call. **The class is every future call that varies what it sends**, and the
product has just acquired its first such call. The second will arrive the moment any other call varies — a
streaming call, a call that offers recall but not the file tool, a second answering site.

### 1.3 Success criteria

1. **No judgement request states that a capability is available which that same request does not carry.** The
   property is decidable from the request bytes alone, without the response.
2. **Nothing in the mechanism depends on the model taking advice** (#14561 §11 alt 6; #14695 C-2). §4 is the
   argument that this design satisfies it rather than the assertion that it does.
3. **Every non-reserved call is byte-identical to today.** The change is inert on the five research calls, and
   that inertness is asserted, not assumed.
4. **A diff that reintroduces the disagreement fails a test**, including one written by someone who has not
   read this document.
5. **The failure mode is today's behaviour, never worse** (#14695 D-4's discipline, inherited).

---

## 2. Scope & Non-Scope

### 2.1 In scope

- Which system text is sent on a call whose tool list is withheld, and how that selection is made.
- Where the seam goes between `internal/loop`, `internal/systemtext` and the two adapters, given the standing
  positions in #13345 §4 and #10532 §8.6.
- The falsifier that fires if prompt and request disagree again, and where it lives.
- Whether the reserved call's prompt should differ in any other way (§7.4: no, and why for each candidate).
- Ordering, so each unit ships alone with no intermediate state worse than today.

### 2.2 Out of scope — declined explicitly

| declined | why |
|---|---|
| **Redesigning the reserved call** | #14695 owns it and it is merging. This document changes what that call *says*, never when it happens, what it withholds, or what its budget is. |
| **Whether the reserved call can produce a usable answer at all** | §10 gives the honest answer and §16 scopes the remainder as **Q-S1**, a separate question with a named owner. Absorbing it here would bundle a truth-correction with an unmeasured prompt-engineering unit. |
| **Any new sentence, in this unit** | #14695 §19.1 (*do not add a sentence to the prompt*) and #14630 §10 (one variable per comparison). §4 establishes that a new sentence is a different kind of change with a different owner; §8 A-3 rejects it as the primary. |
| **Rewording the four sentences that stay** | The wording is a prompt-engineering deliverable (#14600, #10532 §14). This design moves paragraphs; it authors none. |
| **The block on the reserved call** | #14695 §18 Q1 assumes the full block stays. Unchanged here. |
| **Storing `UnofferedToolCalls` / `ReasoningBytes` in the record** | #14717 §3.4 reports the gap. It is a record-completeness matter that predates this finding and is not made worse by it. Filed, not worked (§17). |
| **Making the system text per-adapter** | #13345 §4.3: a per-provider *rendering of the same data* is a measurement defect. Barred, with a falsifier in §11 (V-4). |

---

## 3. Measured Facts, and Whose They Are

### 3.1 Not mine — the operator's, at #14717, 2026-09-23

| fact | value |
|---|---|
| withholding at the wire | request 6 keys `[max_tokens, messages, model, reasoning_effort, temperature]`; request 5 the same **plus `tools`** |
| prompt invariance | messages 0–9 **byte-identical** between calls 5 and 6; message 0 carries both tool paragraphs |
| observations of the reserved call | **6** — one live proxied call plus five replays of the byte-identical captured request |
| content | 244 B live; 162–244 B on replays; **0 B on the unproxied primary run** |
| shape | **6 of 6** open *"I have enough to write the report. Let me confirm one thing / one more thing about…"* and then name what they still want to look up |
| subject-matter claims | **0 of 6** |
| finish reason | `stop` on all six — **none was truncated**; each stopped of its own accord after the preamble |
| research cost of the reservation | **zero** — five rounds dispatched on the branch, five on the baseline |

### 3.2 Not mine — #14250's, carried through #14695 A-5

The nudge family — telling the model what to do about its own run — was measured at **zero effect** on this
model class: three live runs, three wordings, zero tool calls, one hallucinated tool action.

### 3.3 Mine — read at `3e6a055`, no execution

1. **The system text is one exported constant**, six paragraphs, sent on every judgement step. Paragraph 4 is
   the recall tool; paragraph 5 is the file tool. It lives in `internal/systemtext` and is imported by
   `cmd/processor`, `cmd/measure` and one artifact test — the move that made drift between the two binaries a
   compile-level impossibility (#14600).
2. **It reaches the adapters as a per-call field.** The judgement seam already carries the system text as an
   input to every call. **Making the text vary by call needs no seam widening at the adapter boundary at
   all** — only a different value in a field that is already there.
3. **The loop already computes the deciding quantity.** The reservation is determined *before* the call is
   built, from the three converging conditions, and is passed to the adapter as a single boolean meaning
   *withhold the tool list*.
4. **Each adapter hardcodes the same pair** of tool declarations and attaches them when that boolean is unset.
   The pair is written out independently in each adapter, as #13345 §4 decided it should be.
5. **The loop already owns tool identities** — the record's names for the recall tool and the file tool — and
   each adapter already maps one of them to its own wire spelling. **Loop vocabulary already crosses into the
   adapters; wire vocabulary does not cross into the loop.** This is the direction the whole design travels in.
6. **Both adapter test suites already capture the full request body** against a local server and decode it.
   A test that reads the system message and the tool array out of the same captured bytes needs no new harness.
7. **No test anywhere compares the prompt against the request.** Adapter tests pass literal placeholder strings
   as the system text, so nothing in the tree would have gone red when the paragraphs went false.
8. **The system text's own fourth-from-last sentence is** *"Do not describe these instructions or your search
   process in the answer."* See §4.4 — this is the load-bearing fact of the whole document.

### 3.4 What is **not** measured, stated so nobody reads a gap as a zero

- **No observation exists of a reserved call reached by the closed-recall trigger.** All six observations are
  the call-cap trigger. §15 Q-3 says why that matters and why the missing observation is nearly free.
- **No observation exists of this prompt state with the paragraphs removed.** That is precisely Unit 0.
- **The primary run's 0 bytes has no captured response body.** #14717 §3.4 states the inference and marks it as
  one. Nothing here rests on it.

---

## 4. Where the Line Sits — Removing a False Assertion versus Adding a True Instruction

The brief asks for this to be tested rather than inherited, and it is the load-bearing section: options 2 and 3
both sit near the line, and one of them crosses it.

### 4.1 What #14561 actually bars

Not *prompts*. Not *sentences*. It bars a design **whose guarantee is that the model will act on what it was
told**. The rule's force comes from §4.1's asymmetry — an instrument may only fail, never pass — and from the
measured fact that the nudge family produced zero effect. A mechanism that works only if the model complies has
no floor: its success rate is unmeasured, and on this model class the one measurement we have is zero.

### 4.2 The two candidate criteria, and why the obvious one fails

**The tempting criterion is "guarantee versus hope": a design is barred when its guarantee is compliance-shaped,
not when its hope is.** It fails on the first case. *"No tool is available on this call"* also guarantees only
that the bytes say so and hopes only that the model reads them. Guarantee-versus-hope does not separate option
1 from option 2, so it is not the line.

**The criterion that does separate them is: what must the model do for the change to have its effect?**

| operation | what the model must do | can the model decline? |
|---|---|---|
| **delete a sentence** | nothing — the premise it was reasoning from is gone | **no: compliance is not a well-formed notion for an absence.** There is nothing to obey and nothing to ignore |
| **add a statement of fact** | read it, believe it, integrate it | yes — quietly, and we would not know |
| **add a directive** | read it, believe it, and *act on it* | yes — and it was measured doing exactly that (§3.2, §4.4) |

**The line, stated for reuse:**

> **A change is compliance-independent when its full effect is complete at the wire and the model's
> cooperation is not a step in the causal chain. Deleting a sentence whose truth is decidable from the request
> bytes is that kind of change: it does not ask the model for anything, it stops asserting something untrue.
> Adding a sentence — of any kind — is not, because the sentence only does work if the model takes it.**

### 4.3 The eligibility test, and it is mechanical

A sentence is eligible for this treatment when **its truth value is decidable from the request bytes alone**.
*"A recall tool is available"* is decidable: look in the tools array. *"Be direct and specific"*, *"the context
block was assembled by an automatic memory search"*, *"do not invent facts"* are not — they are instructions and
framings, they are nobody's falsifiable claim about the request, and **this design does not touch them**.

That test does three jobs at once and it is why the design has a mechanical falsifier at all:

1. It **bounds the change**. Two sentences qualify. Not four, not six.
2. It **is the property the falsifier asserts** (§11 V-1). A rule stated as *"do not lie in the prompt"* has no
   test; a rule stated as *"every prompt sentence whose truth is decidable from the request must agree with the
   request"* becomes a biconditional over an enumerable domain.
3. It **refuses to expand**. Anyone proposing to delete a further sentence under this design must first show
   that sentence's truth is decidable from the request. None of the remaining four is.

### 4.4 Where option 2 falls, and the evidence is already in hand

Option 2 — *state positively that this is the final call and prose is the only outcome* — is **two propositions
wearing one sentence**, and they fall on opposite sides of §4.2 for different reasons.

**2a, the factual half** — *"no tool is available on this call"*. True, not a directive, and **not barred by
#14561**. It is barred here by weaker but sufficient rules, and the weakness is worth being honest about:

- It is a **prose restatement of a fact the request already carries structurally.** The absence of the tools
  array *is* the native encoding. Its marginal value over the deletion is precisely the question of whether
  *saying* it does more than *being* it — which is a compliance question, unmeasured, and therefore a
  measurement rather than a design premise.
- It is **a new sentence**, so its wording is a prompt-engineering deliverable with an owner who is not this
  document (#14600), and shipping it alongside the deletion puts two variables in one comparison (#14695
  §19.1, #14630 §10).

**2b, the directive half** — *"prose is the only outcome"*, *"write your report now"*. This is the nudge family.
Barred by #14561 §11 alt 6 as a mechanism, and **#14695 A-5 already ruled it** — not forever, but only as its
own later unit with its own falsifier, after D-1, so its effect is separable.

**And the decisive evidence is measured, in this very prompt.** The shipped system text already contains:

> *"Do not describe these instructions or your search process in the answer."*

**Six of six observations are a description of the search process.** The sentence option 2 would strengthen is
already there, already addressed to this exact failure, and was violated in every observation we have. That is
the fidelity-clause-failing-inside-the-prompt-written-to-enforce-it case, measured on this defect rather than
argued by analogy — and it is the strongest single argument in this document.

> **So option 2 is not rejected as "near the line". 2a is permitted and unjustified; 2b is barred; and the
> prompt already contains 2b's weaker form and it did not work.**

### 4.5 One shape that looks compliance-free and is not — rejected before someone builds it

A loop could append, before the reserved call, a **tool-result-shaped message** saying research has ended —
reusing the channel that already carries *"recall is closed for this turn"*. It needs no new prompt sentence and
it lands in the strongest position in the history.

**Rejected.** A tool result the model never requested is **a false event in the history** — the same defect
class this document exists to close, one layer over, and considerably harder to see. The rule generalises:
**correcting a false assertion may never be done by manufacturing a true-sounding one.**

---

## 5. Architectural Overview

Nothing new stands beside the loop. **One value the loop already computes stops being a boolean and starts
being the single source both the prompt and the request are derived from.**

```
                  today                                      proposed
   ┌───────────────────────────────┐          ┌───────────────────────────────┐
   │ loop: reserved?  (yes / no)   │          │ loop: reserved?  (yes / no)   │
   └──────────┬────────────────────┘          └──────────┬────────────────────┘
              │ boolean                                   │ OFFERED SET
              │                                           │ {recall, writeFile} | {}
              v                                           ├──────────────┬──────────────┐
   ┌───────────────────────────────┐                      v              v
   │ JudgeInput                    │          ┌──────────────────┐  ┌──────────────────┐
   │   System: THE constant  ──────┼──── x    │ system text      │  │ JudgeInput       │
   │   WithholdTools: bool         │          │ composer         │  │   OfferedTools   │
   └──────────┬────────────────────┘          │ (per-identity    │  └────────┬─────────┘
              v                               │  paragraphs)     │           v
   ┌───────────────────────────────┐          └────────┬─────────┘  ┌──────────────────┐
   │ adapter: if !withhold →       │                   │            │ adapter: declare │
   │   hardcoded pair              │                   └──── System │   each offered id │
   └───────────────────────────────┘                        field   └──────────────────┘
              │                                                              │
              v                                                              v
      prompt and tools maintained separately              both are projections of ONE value
      (they disagreed, and nothing noticed)               (they cannot disagree; a test says so)
```

**`x` marks the defect:** today two arrows leave the loop and nothing downstream reconciles them. The proposal
removes the second independent source, not by moving text around, but by making the tool paragraphs and the
tool declarations **two renderings of the same set**.

**What crosses which boundary, and nothing else does:** the loop hands **down** a set of its own identities.
The adapter renders each identity into its own wire declaration, as it already does for the tool *name*. The
composer renders each identity into its own paragraph. **No wire vocabulary moves into the loop, and no wording
moves into the adapters.**

---

## 6. Components & Responsibilities

| component | owns | does **not** own |
|---|---|---|
| **the judgement cycle** (`internal/loop`) | which tool identities this call offers — it already decides this, one step earlier; the identity vocabulary itself, which is the **referee** for §11 V-2 | any wording; any wire declaration; the order of paragraphs in the prompt |
| **the system text composer** (`internal/systemtext`) | the invariant text, one paragraph per tool identity, and how a given offered set renders into one string | when tools are withheld, and why; anything about a protocol |
| **each protocol adapter** | declaring each offered identity to its endpoint, in its own envelope, schema, spelling and description — **duplicated across adapters, as #13345 §4 decided** | the reservation policy; the wording; which identities exist |
| **`main`, in both binaries** | wiring the composer in at exactly the site that wires the constant in today, preserving #10532 §8.6's substitution right | composing anything itself |

**The placement rule that puts each of these where it is** is #13345 §4 — *does it render the loop's own data, or
declare something to an endpoint?* Applied to the new elements:

| element | renders the loop's data? | declares to an endpoint? | side |
|---|---|---|---|
| the offered set | yes — the loop's own decision, in the loop's own identities | no | **loop** |
| a tool's prompt paragraph | yes — it describes a capability the loop is offering, to a reader | no. **One consumer characteristic: a model reads it.** No protocol constrains its shape | **systemtext**, protocol-neutral, one instance for both adapters |
| a tool's wire declaration | no | yes — envelope, schema, wire name, description | **adapter**, duplicated |

The paragraph and the declaration describe the same capability and land on opposite sides. **That is not a
contradiction, it is the rule working:** #13345 §4.2 already establishes that a tool's *description to an
endpoint* is four per-protocol things bound into one literal, while a prompt has exactly one consumer
characteristic. The paragraph is a prompt. It stays neutral, single-instance, and shared — and §11 V-4 is the
guard that keeps it that way.

---

## 7. The Decisions

### 7.1 D-1 — The prompt's tool paragraphs and the request's tool list are two projections of one value

**Statement.** The loop determines, before each call, **which tool identities it is offering** — a set, not a
flag. That set is the sole input to both the system text sent on that call and the tool list attached to it.
Neither is maintained independently of the other.

**Why a set and not the existing boolean.** A boolean cannot be projected into per-tool paragraphs without
re-encoding the pairing somewhere else — and *somewhere else* is exactly the second independent source the
defect came from. A set makes the pairing the only representation there is.

**Why this is compliance-independent.** The guarantee is a property of the bytes leaving the process: *no
request states that a capability is available which that request does not carry*. It is checkable without the
response, and the model cannot decline it. §4.

**What it costs.** On every non-reserved call: **nothing, byte for byte** — asserted by V-3, not assumed. On the
reserved call: two paragraphs, about 380 bytes of prompt, removed.

**Why it covers the triggers the reserved-call unit has not shipped yet.** The set is derived from the
reservation, not from the trigger. The call-cap trigger, the closed-recall trigger and #14695's not-yet-shipped
time-guard trigger all converge on the same reservation, so all three are covered on the day they exist, with
no further interaction to manage.

### 7.2 D-2 — The offered set is ordered, and the composition is deterministic

**Statement.** The offered set has a fixed, declared order, and both projections consume it in that order.

**Why this is a decision and not an implementation detail.** #10532 §9.1 classifies the model *request* as
deterministic — a pure function of its inputs, pinned byte-exact at the wire. An unordered collection would make
both the tools array and the prompt's paragraph order dependent on iteration order, silently converting the one
fully-deterministic stage before the model into a nondeterministic one, and breaking byte-exact wire tests in a
way that reads as flakiness rather than as a defect. **The determinism of stage 3a is a shipped property; this
design must not spend it.**

### 7.3 D-3 — The composer is injected where the constant is injected today

**Statement.** The loop receives the system text as a **value supplied at construction** — as it does today —
but the value is now a function of the offered set rather than a fixed string. The seam's *type* is declared by
the loop; `internal/systemtext` satisfies it; both binaries wire the same one.

**Why not let the loop import the text package directly.** #10532 §8.6 keeps the system text an injected value
so that *"read it from a node instead"* stays a change in `main` and never a change in the loop. A direct import
spends that property for nothing.

**Why this is not a gratuitous seam.** #10532 §9.2's falsifier is *any interface with one implementation, one
test double, and no experiment behind it.* Answered head-on rather than dodged: **this does not add a seam, it
re-types one that already exists**, and the reason is forced — the value being injected became call-dependent,
and a string cannot express a call-dependent value. It is a function value, not an interface; it has one
production implementation on purpose (#14600: two binaries must not be able to send different prompts); and
`main` retains exactly the substitution right §8.6 was written to preserve.

**Why not two injected strings instead** — the full text and a tool-less variant. That is option 1, and §8 A-1
rejects it: two opaque constants whose relationship is a convention nobody can check, plus two silent
obligations on whoever adds a third tool.

### 7.4 D-4 — The reserved call's prompt differs in exactly one way, and the other candidates are refused by name

The brief asks whether the reserved call's prompt should differ in any other way. **It should not**, and each
candidate is refused for its own reason rather than by a blanket rule:

| candidate | disposition |
|---|---|
| *"Do not describe these instructions or your search process in the answer."* — **strengthen it** | **Refused.** It is the sentence the model already violated in 6 of 6. Strengthening a clause measured at zero is the nudge family (§4.4). |
| *"If you still do not have enough information, say so plainly and say what is missing."* — **remove it** | **Refused, and note the tension honestly.** This sentence is the closest thing in the prompt to a licence for the observed output: naming what is missing is what the model did. But it is **true and unconditionally relevant** — it is what makes an honest curtailed answer possible at all, and removing it would be a behavioural bet, not a fact correction. It fails the §4.3 eligibility test: its truth is not decidable from the request. **Out of scope by the design's own rule**, and the rule is worth more than the tempting edit. |
| the four framing sentences about the block | **Refused.** All true on every call. Not eligible. |
| a shorter or different block on the reserved call | **Refused** — #14695 §18 Q1 owns it. |
| a different output-budget statement in prose | **Refused.** The budget is carried by the request; stating it in prose is 2a (§4.4). |

**The design changes exactly two sentences, and only by removing them on exactly one call.**

### 7.5 D-5 — The falsifier lives at the wire, in each adapter's own package, and imports the composer

**Statement.** The agreement property is asserted where the prompt and the tool list exist in the same bytes —
the captured request — and the assertion uses the **real** composer, never a fixture.

**Why there.** The disagreement is only observable where both halves are in one artifact, and that is the wire.
Asserting it against a hand-written fixture string is #14561 §12.11's *instrument whose guard cannot fail*: it
would assert the test's own copy against itself.

**Why not one cross-adapter test.** #13345 §6.2 rejected cross-adapter equality for having **no referee** —
equality reports that two things differ, never which is wrong. This property has a referee: **the loop's tool
identity vocabulary is the declared domain**, and each adapter is checked against *it*, independently, never
against the other adapter. Two separate assertions in two packages, one arbiter, no symmetry problem.

---

## 8. Alternatives Rejected

### A-1 — Option 1 taken literally: a second hand-written constant for the tool-less call — **rejected**

It is the minimal true statement and it would work today. It loses on three counts:

1. **The relationship between the two constants is a convention with no checker.** Nothing says the second is
   the first minus the tool paragraphs; a reword of one leaves the other stale, silently.
2. **It leaves two silent obligations on whoever adds a third tool** — add a paragraph, and remember to keep it
   out of the other constant. #12935's finding is that obligations nobody is reminded of are the failure mode,
   not the fix.
3. **It buys nothing.** §8's decisive fact below.

> **A-1 and D-1 send the same bytes.** The wire delta is identical: two paragraphs vanish from message 0 on the
> reserved call, and nothing else moves. **The choice between them is entirely about what the next diff can
> break**, and it costs one small registry and one seam re-type. Choosing the option that makes the class
> unrepresentable is therefore close to free — which is why this document chooses it decisively rather than
> presenting it as the ambitious option.

### A-2 — Option 2, the positive statement — **rejected; 2a unjustified, 2b barred**

Fully argued in §4.4. The half that is permitted restates a fact the request already carries and its marginal
value *is* the compliance question; the half that is barred already exists in the prompt in weaker form and was
violated 6 of 6.

### A-3 — Delete the paragraphs *and* add the true statement in one unit — **rejected on comparability**

Even granting 2a, shipping both puts two variables in one wire delta, so a measured improvement could not be
attributed. #14695 §19.1 and #14630 §10. **Not barred forever**: §16 Q-S2 is exactly this, as its own unit, with
the deletion-only arm as its baseline.

### A-4 — Have the adapter strip the tool paragraphs when it withholds the tools — **rejected**

It puts prompt surgery in the adapter, makes the text per-adapter in practice (#13345 §4.3: a measurement
defect), and requires the adapter to recognise sentences — the worst available coupling.

### A-5 — Have the loop strip the paragraphs from the injected constant — **rejected**

The loop would have to parse a prompt to find them. It makes the loop's correctness depend on the exact wording
of a prompt-engineering deliverable, which is the coupling #10532 §8.6 exists to prevent.

### A-6 — A synthesised tool result announcing that research has ended — **rejected**

§4.5. Manufacturing a true-sounding event is the same defect class one layer over.

### A-7 — Change nothing; the mechanism already works at the wire — **rejected, and it deserves the hearing**

The tool withholding *is* implemented exactly as designed and the reservation costs no research. But the
product's own prompt contains two false statements about the request carrying it, and 6 of 6 observations show
the model acting on them. **A design whose central claim is that the last call's output is worth keeping cannot
leave standing the sentence that makes that output a research preamble.** It also loses on #12935's ground: the
class is now live, and doing nothing means the next varying call reproduces it.

---

## 9. Contracts & Interfaces (Abstract)

| contract | input | output | invariants |
|---|---|---|---|
| **the offered set** | the loop's reservation decision for this call | a deterministically ordered set of the loop's own tool identities; **empty on a reserved call, the full vocabulary otherwise** | drawn only from the loop's declared vocabulary; never constructed anywhere but the one site that decides the reservation |
| **the system text composer** | an offered set | one string | **for the full vocabulary it returns today's shipped constant byte for byte** (V-3); for each identity, its paragraph appears **iff** the identity is in the set; the remaining text is invariant across every set; **protocol-neutral** — the same instance serves both adapters |
| **the judgement seam, per call** | the offered set, plus everything it carries today | — | the adapter attaches a declaration for **exactly** the offered identities and no others; an empty set omits the tools key entirely, as it does today |
| **each adapter's declaration map** | one loop identity | that adapter's own wire declaration | total over the loop's vocabulary (V-2); lives in the adapter, duplicated between adapters without apology |

**What no contract here says:** what the paragraphs say. The wording is unchanged, and is not this document's to
change.

---

## 10. Is the Deletion Sufficient? — The Honest Answer

The brief asks for this result even if it is unwelcome. **It is: I do not believe the deletion alone can be
relied on to make the reserved call produce a usable report, and I would not have the design claim otherwise.**

### 10.1 What the deletion clearly removes

The observed output has two clauses: *"I have enough to write the report."* then *"Let me confirm one thing
about X."* **The first clause is the model's own conclusion that it can answer.** The second is a research move,
and the stated licence for it is the paragraph being deleted. Removing the licence removes the stated basis for
the second clause. That much I predict.

Supporting detail, and it matters: **all six stopped of their own accord** — `finish_reason: stop` at 82–104
completion tokens against a 192-token budget. **None was truncated.** The natural continuation of *"let me
confirm one thing"* is a tool call; with no tools declared there was nothing to emit, so the turn ended. Remove
the preamble and nothing forces that stop.

### 10.2 What the deletion does **not** remove, and this is the honest half

1. **The history still demonstrates five successful tool calls.** Messages 2–11 are five assistant tool-call
   messages and five tool results, every one of them productive. **In-context demonstration is a stronger signal
   than a paragraph**, and the deletion does not touch it. On the call-cap trigger specifically, the last thing
   the model sees before the reserved call is a **successful tool result** — the maximal available evidence that
   more research is possible.
2. **Nothing true in the prompt says research has ended.** On the cap trigger, the request's silence about tools
   is the only carrier of that fact, and silence is a weak carrier against five demonstrations.
3. **One remaining sentence licenses a shortfall statement** — *"if you still do not have enough information,
   say so plainly and say what is missing"* — and the observed outputs are arguably a malformed execution of it.
   The plausible best case of the deletion is therefore not a report but a **clean shortfall statement**: *"I do
   not have enough information; what is missing is X."* That clears `produced=true` and **does not clear
   #14717 §4.2's bar**, which is whether an operator learns anything about the subject matter.

### 10.3 The claim, and what falsifies each half

| claim | prediction | falsified by |
|---|---|---|
| **the deletion removes the observed opening** | the *"let me confirm one thing"* opening disappears | **≥1 of 6** replays still opening with a research preamble — which would show the paragraphs were not the operative licence and would make reading 10.2(1) the live one |
| **the deletion is not sufficient for a report** *(my pessimism)* | fewer than 5 of 6 replays contain substantive subject-matter prose | **≥5 of 6** carrying real claims about the subject — in which case §16 Q-S1 closes unopened and this section was wrong, which is a good outcome and should be recorded as one |
| **the design is not worse than today** | ≥1 of 6 produces bytes; none is a tool-shaped payload | **all six returning 0 bytes** — a stop condition, see §17 Unit 0 |

### 10.4 The instrument is already built

#14717 captured the byte-identical request and replayed it five times. **The measurement is the same replay with
message 0's two paragraphs removed and nothing else changed** — six single-call observations, no graph write, no
product change, comparable to the existing six by construction because every other byte is held. That is the
cheapest decisive evidence available on this question and it exists because someone already built the harness.

---

## 11. Falsifiers — Filed at Authorship Time, With Their Consequences

**#12935's rule applied literally: each is filed now, with the diff that would falsify it named, because that
diff need not touch the file where the claim lives.**

| # | property | falsified by | named consequence |
|---|---|---|---|
| **V-1** | **Prompt/request agreement, at the wire.** On any judgement request, the system message contains a tool's paragraph **iff** the request's tool list declares that tool. Asserted in each adapter's own package, in **both** directions — once on an offering call and once on a withholding call — over the captured request body, using the real composer | any request where the biconditional fails. **The diff that would do it:** adding a third tool to an adapter, or making the composer's paragraph selection independent of the offered set | **The class is back.** This is the assertion that makes the class unrepresentable rather than the instance fixed |
| **V-2** | **Domain coverage.** Every identity in the loop's tool vocabulary has a paragraph in the composer and a declaration in **each** adapter, and neither carries an identity the loop does not declare | a vocabulary entry with no paragraph, or a declaration with no entry. **The diff:** a tool added to one projection only | V-1 can pass vacuously for a tool nobody declares. **V-2 is what stops V-1 being satisfiable by omission** |
| **V-3** | **Inertness on every non-reserved call.** For the full offered set, the composed text equals today's shipped constant **byte for byte** | any difference, including whitespace | The five research calls are provably untouched, which is what keeps the change a **one-variable** delta for any later comparison (#14630 §10) |
| **V-4** | **Neutrality of the composer.** No adapter imports the system text composer in production code, and no composed text varies by protocol | any adapter production file importing it, or any per-protocol branch in the composer | A per-provider rendering of the same prompt is a **measurement defect**, not a tuning decision (#13345 §4.3) |
| **V-5** | **Positive control.** Removing the omission logic must turn V-1 red; removing one paragraph from the composer must turn V-2 red | a guard whose subject can be deleted with the suite still green | #14561 §12.11 and V-13. **An instrument whose guard cannot fail is not an instrument** — and the implementer must demonstrate both reds before the unit is called done |

**What no property here checks:** whether the resulting prose is any good, or whether the deletion helped. That
is §10's measurement and #14561 §17.6's adjudicator's question — **a finding, never a gate.**

**The negative claim this document makes, filed with its expiry per #12935:** *"no sentence other than the two
tool paragraphs states something about the request whose truth is decidable from the request bytes"* — true at
`3e6a055`, and **falsified by any future sentence added to the system text that describes the request**. V-1 does
not catch that on its own, because V-1 iterates the tool vocabulary; a new self-describing sentence about
something other than a tool is outside its domain. **Named here so it is not discovered later as a stale claim.**

---

## 12. Cross-Cutting Concerns, Quality Attributes, Trade-offs

| concern | disposition |
|---|---|
| **Determinism** | Preserved and explicitly defended — D-2. The request stays a pure function of its inputs; an unordered set would have quietly ended that. |
| **Observability** | No new log line. The reservation log already names the condition and the budget; a second line naming the offered set would restate what V-1 pins at the wire. **Deliberately not added.** |
| **Error handling** | Unchanged. Nothing in this design can fail at runtime: composing a string from a set has no failure mode, and a reserved call that fails is still #14695 D-4's problem, already solved. |
| **Comparability** | **Zero cost.** Reserved calls exist only on an unmerged branch, so the population of archived records affected is **empty**. No figure dies (contrast #14695 §13). |
| **Performance** | Two paragraphs fewer on one call in a curtailed run. Unmeasurable and irrelevant. |
| **Maintainability — the actual return** | The product gains a place where *"what this call offers"* is written once. The next varying call inherits the property instead of re-creating the defect. |
| **The trade-off, stated rather than hidden** | D-1 introduces a capability nobody needs today: the offered set makes **partial** offerings expressible, and no current caller wants one. That is YAGNI pressure, and the answer is that the set is not the feature — **the single source is the feature**, and a boolean cannot be a single source for two per-tool projections. The expressiveness is a by-product, and it is confined by there being exactly one construction site. |
| **The second trade-off** | The composer seam is a function value where a string stood. It is more surface than option 1. §7.3 answers #10532 §9.2 head-on rather than hoping nobody asks. |

---

## 13. Risks & Mitigations

| # | risk | mitigation |
|---|---|---|
| **R-1** | **The deletion does not help, and the reserved call still yields no report.** Live, and §10 says it is more likely than not | Unit 0 measures it **before** anything is claimed. The deletion is justified as a truth-correction regardless of its effect, so R-1 does not block the unit — it opens Q-S1 |
| **R-2** | **The composer is treated as a general prompt-templating facility** and grows conditionals | V-4, plus the §4.3 eligibility test, which admits exactly the sentences whose truth the request decides |
| **R-3** | **V-1 becomes a string-matching nuisance and is deleted** the first time the wording changes | Under D-1 it cannot: it asserts against the composer's own paragraphs, so a reword moves both sides together. **This is the single strongest practical argument for option 3 over option 1**, where the equivalent test would have to match a literal sentence |
| **R-4** | **`internal/systemtext` stops being the "one constant, no test file" package** #14600 describes | Real. It gains a function and a test file, and #14600 must be reconciled in the same arc (§17). The alternative — keeping the package inert — is what forced the disagreement to live somewhere unchecked |
| **R-5** | **Test churn at the construction sites** (three production call sites plus several test call sites take a composer where they took a string) | Mechanical and one-time. Kept out of the measurable unit by the split in §17 |
| **R-6** | **Someone "fixes" the remaining silence** by adding a sentence in the same PR | §7.4 and A-3 name it; the PR body should quote them. The door to 2a is deliberately left one line wide and is not walked through here |

---

## 14. Migration and Rollout

**This design stacks on `3e6a055` (`feat/the-last-call-is-already-spent`, PR open, not merged). Rebase onto it;
do not bundle with it.** Before `3e6a055` there is no call that withholds tools, so there is no disagreement to
fix and the unit has no meaning.

**No migration.** No stored artifact changes shape, no record field is added, no archived population is
reinterpreted. The change is confined to bytes leaving the process on calls that do not yet exist in any record.

---

## 15. Open Questions

1. **Q-1 — Should the empty offered set eventually have its own paragraph?** D-1's registry makes *"no tool is
   available on this call"* a one-line addition later. **Deliberately not taken** (§4.4, A-3). It is a measured
   prompt-engineering unit, not a refactor, and its baseline is the deletion-only arm. Left open, with the door
   named so nobody has to rediscover it.
2. **Q-2 — Does the file tool's paragraph deserve separate treatment?** Both paragraphs are deleted together
   because both are false together. But a run whose intended deliverable is a file now reaches a last call on
   which it can never write one. That is #14695's decision, not this one; **removing the paragraph makes the
   loss legible to the model rather than creating it.** Flagged in case someone reads the deletion as the cause.
3. **Q-3 — How does the closed-recall reserved call behave?** Unmeasured (§3.4), and it is a **natural
   experiment already present in the product**: on that trigger the history *already* carries a true statement
   that recall is closed, in the strongest position available, while the system text still says a recall tool is
   available. **It is the cheapest evidence anyone will get on whether telling the model something true about its
   own run changes what it writes** — which is precisely what option 2 would bet on, and it needs no new
   sentence to observe. Recommend it be captured the next time a run closes recall; **not** a gate on this unit.
4. **Q-4 — Should the record carry what the prompt asserted?** The record carries the block and not the system
   text, so no record-level falsifier for this property is possible today. Adding one is a prompt-provenance
   design, and it is where #14717 §3.4's unstored `UnofferedToolCalls` / `ReasoningBytes` gap belongs too.
   **Filed, not worked.**

---

## 16. Scoped Out, Deliberately

**Q-S1 — Does the reserved call need more than a truthful prompt to produce a usable answer?**

§10 says the answer is probably yes and declines to solve it here. The brief asks for this to be scoped
separately rather than absorbed, and it should be, for a reason stronger than tidiness: **the remedies live in
different families with different owners** — a measured prompt unit (Q-1 / A-3), a loop-level change such as
retaining the prose the model wrote alongside earlier tool calls (#14700), or a conclusion that the reserved
call is not recoverable at this budget and this prompt state. Choosing among them needs Unit 0's result first,
and bundling any of them with a truth-correction would make the correction unmeasurable.

**Owner:** whoever owns #14695. **Input:** Unit 0's six observations. **Trigger:** Unit 0 showing the deletion
removes the preamble without producing subject-matter prose.

**Q-S2 — Does stating the terminal status in prose add anything over the request already carrying it?** §4.4's
2a, as its own unit, its own arm, its own falsifier, after this one. Not barred — unjustified until measured.

---

## 17. Implementation Guidance — Ordered Units

**No code in this document. Each unit is one PR, in this order, per the one-feature-one-PR rule. Rebase onto
`3e6a055` first.**

**Unit 0 — the measurement, no product change.**
Replay #14717's captured reserved-call request **six times with message 0's two tool paragraphs removed and
every other byte held**, on the same endpoint and model, writes suppressed, no graph read or write. Record the
opening clause and whether any subject-matter claim appears, against §10.3's table.
**Unit 0 does not gate Units 1–2 — it gates the *claim*.** Removing a false assertion is justified whether or not
it helps; what needs evidence is any statement about its effect. **One stop condition:** if all six return 0
bytes where the un-deleted prompt returned 162–244, the deletion has made the call worse and the design stops
for re-briefing.

**Unit 1 — the offered set replaces the boolean. Wire-inert by construction.**
1. The loop declares its tool identity vocabulary as an ordered domain (D-2) and constructs the offered set at
   the one site that decides the reservation.
2. The judgement seam carries the set where it carried the flag.
3. Each adapter declares exactly the offered identities, from its own declaration map, in the set's order.
4. **V-2** in each adapter package; **V-3**'s equivalent for the tools array: an offering call's request is
   byte-identical to today's.
5. **Nothing about the prompt changes in this unit.** Its wire delta is empty, and that is the property to
   assert and to state in the PR body.

**Unit 2 — the system text becomes a function of the offered set. One wire delta.**
1. The composer gains one paragraph per identity and the invariant remainder; **V-3** pins the full-set output
   against the shipped constant byte for byte.
2. The seam that injects the system text takes a composer value; both binaries wire the same one (D-3).
3. **V-1** in each adapter package, both directions, against the real composer.
4. **V-5**: demonstrate both reds before calling it done.
5. Reconcile **#14600** — `internal/systemtext` is no longer a package with one constant and no test file.

**Filed, not worked** — `task` nodes linked to #10422 and to this document:

- **Q-3's observation** — capture a closed-recall reserved call's request and response. The cheapest available
  evidence on whether a true statement about the run changes what the model writes.
- **Q-4** — prompt provenance in the record, carrying #14717 §3.4's unstored warning counters with it.
- **Q-S1** and **Q-S2** — §16, owned by #14695.

### 17.1 What an implementer must not do

- **Do not add a sentence to the prompt.** §4.4, A-3, #14695 §19.1. Two paragraphs are removed; none is written.
- **Do not reword the four sentences that remain.** The wording is a prompt-engineering deliverable.
- **Do not delete any sentence whose truth is not decidable from the request bytes** (§4.3). The eligibility
  test is the scope boundary, not a guideline.
- **Do not let the composer branch on protocol, adapter, model or trigger.** V-4.
- **Do not assert V-1 against a fixture copy of the text.** A guard that checks its own copy checks nothing.
- **Do not make the offered set unordered.** D-2 — it would end stage 3a's determinism silently.
- **Do not synthesise a tool result to announce that research has ended.** §4.5.
- **Do not bundle Units 1 and 2.** Unit 1's wire delta must be empty and provably so; Unit 2's must be exactly
  the two paragraphs.
- **Do not claim an effect before Unit 0 has run.** The correction stands on truth; any statement about what it
  achieves stands on the six observations.

---

## 18. Provenance

Written 2026-09-23 against `3e6a055` in `.claude/worktrees/impl-lastcall`, read-only. **Nothing was run: no
model call, no inference, no endpoint contacted, no graph mutation, no code changed.** Every repo fact in §3.3
was read out of that tree. Every measurement in §3.1 and §3.2 is #14717's and #14250's respectively, attributed
where used. Three judgements are mine and are marked as such: §4's placement of the line, §10's pessimism about
sufficiency, and §8's ruling that A-1 and D-1 send the same bytes so the class fix is nearly free.
