# Architectural Document: A Successful Call That Carried Nothing

> Repo path: `docs/architecture/a-successful-call-that-carried-nothing.md`.
> **Not published to the graph.** Parity is published at merge; this file is the canonical copy until then.
> Project: **#10422**. Overlapping open task: **#14130** (*the `think` key is never sent and `thinking` is
> never read*) — §3.4 **corrects one of its claims** and §11 states how the two designs divide.
> Precedent consumed, not superseded: `docs/architecture/m1-skeleton-loop.md` §10.5 (*retries, and their
> absence*), `docs/architecture/the-query-the-graph-is-asked.md` §4.5 + §18.2 (*the degraded mode, and why
> there is no retry*), `docs/architecture/retrieval-admission-and-the-empty-outcome.md` §1 (*the loop
> cannot fail*), `docs/architecture/what-a-successful-run-withheld.md` (*a successful run that hid what it
> dropped*), `docs/architecture/what-the-adapters-may-share.md` §4 (*the decision rule*).
>
> **Baseline: `9f49229`, worktree `design-empty`, working tree clean.** Every repo fact below was read at
> that ref. A bare `file.go:N` is under `internal/loop/`; every citation outside that package carries its
> full path.
>
> **Every figure quoted from outside this tree is bracketed with the state it was taken at.** No
> measurement in this document is mine; §3.1 states whose they are and what they do and do not cover.
> PR #90 (`impl-floor`) and PR #89 (`impl-backfill`) were not read; no claim here holds at those refs.

---

## TL;DR

**The question.** A model call returns `200`, `done_reason: stop`, and an empty `content`. What should the
loop do?

**The answer, in one line: name it, and read the other channel — do not retry.**

Three changes, none of them a retry:

1. **`TerminalReason` gains `Empty`.** The loop — not the adapter — classifies an answer of zero length as
   `Empty` rather than `Answered`. `Answered`'s own doc comment says *"a completed prose answer"*
   (`types.go:90-91`); an empty string is not one, so the vocabulary is currently telling a lie about
   itself, and `summary.go:31`'s `"<-- terminal reason says answered"` flag exists to apologise for it.
2. **Each adapter reads its own reasoning channel into a new `JudgeResult.Reasoning`.** When `content` is
   empty and the reasoning channel is not, the loop promotes that text to the answer and records
   **`AnswerSource: reasoning`** — the exact move `ToolSource` already makes for a tool call the adapter
   recovered from content because the endpoint reported none (`types.go:111-119`). **No parsing. No
   heuristic. A channel, named.**
3. **The derivation arm gets a different policy, deliberately** — the reasoning channel is **not** promoted
   into `ParseDerivation`, because `parseQueryLines` takes the first five distinct lines and the first five
   lines of a reasoning transcript are reasoning sentences, which would then be searched for in the graph.
   Instead the derivation arm gets **two causes where it has one**, and a recorded shape observation that
   catches §4.5's degenerate third case, which no emptiness test can see.

**What a user gets.** Today: `200`, `answer: ""`, `stopReason.reason: "answered"`, and an operator-only
prose flag nothing acts on. After: on the ~1-in-6 draw that lands in the reasoning channel, **the answer
comes back** — verbose, marked `answerSource: "reasoning"`, and honest. On the genuinely empty draw, the
body says `stopReason.reason: "empty"` and a caller can branch on it. §13 states this in full.

**Cost.** One enum member, one `JudgeResult` field, one `Record` field, one field read per adapter. **Zero
new constants. Zero new model calls. Zero provider vocabulary added to `internal/loop`** — the falsifier is
in §10.1 and its baseline is zero at `9f49229`.

**Rejected.** A retry on either arm (§7.2) · an adapter-level error (§7.1) · parsing the answer out of the
reasoning prose (§7.3) · a validity filter on derived query lines (§7.5) · always carrying reasoning onto
the record (§7.4).

---

## 1. Problem Statement

A judgement call completes. The transport succeeded, the endpoint returned `200`, and it reported its own
terminal state as `stop` — not `length`, not a filter, not an error. And the message body carries no text.

The loop has no member of its vocabulary for this. `mapDoneReason` maps `"stop"` to `loop.Answered`
(`internal/ollama/wire.go:264-273`; `mapFinishReason` likewise at `internal/openaicompat/wire.go:230-241`),
`turn.go:314` assigns `result.Answer` whatever it is, `judge()` breaks out of its loop because `Answered`
is not a tool reason, and `Run` files the record and returns. `internal/server/routes.go:97` encodes the
whole record and the handler writes `200`.

So **a caller receives a successful, well-formed response whose answer field is the empty string, whose
terminal reason claims the model answered, and which took the full cost of a model call to produce.**

The only thing in the tree that notices is `renderSummaryAnswer` (`summary.go:205-213`), which prints
`answer EMPTY (0 B)  <-- terminal reason says answered`. That string is an operator-facing apology for a
vocabulary defect. **Nothing acts on it.**

### 1.1 This is a known shape in this repo, under a different cause

`docs/architecture/retrieval-admission-and-the-empty-outcome.md` §1 states the same structural defect from
the retrieval side, in 2026-09-07:

> *"It cannot fail. There is no path by which 'I have nothing useful' becomes an outcome. The loop's own
> closed set of terminal reasons (`Answered`, `WantsRecall`, `Truncated`, `Refused`, `WantsWrite`,
> `Unrecognised`) contains **no member for 'I need clarification'**. Even if the model produced that
> behaviour, the harness would record it as `Answered`."*

That document diagnosed a **missing outcome** and a vocabulary whose `Answered` member swallows things that
are not answers. **This document is the same diagnosis reached from a different cause**, and it is
strictly the cheaper half: where that one needs the model to learn a new behaviour, this one needs the loop
to stop mislabelling a behaviour the model already has.

`docs/architecture/what-a-successful-run-withheld.md` is the same family again — *a run that returns `200`
and hides what it lost*. Its remedy was one WARN and one changed sentence to the model. **This document's
remedy is larger by exactly one enum member, and for the same reason: the thing being hidden here is the
answer itself.**

### 1.2 Success criteria

| # | Criterion | How it is checked |
|---|---|---|
| S-1 | A run whose model produced no text is distinguishable **from the response body alone**, without reading logs | `stopReason.reason == "empty"` |
| S-2 | A run whose model produced its text on the reasoning channel **returns that text**, and says so | `answer` non-empty, `answerSource == "reasoning"` |
| S-3 | Neither outcome costs a second model call | `modelCalls` unchanged for the same trajectory |
| S-4 | `internal/loop` gains **no** provider vocabulary | §10.1's grep stays at zero |
| S-5 | The two adapters behave identically at the port for the same endpoint behaviour | §10.2's paired test |
| S-6 | The derivation arm's two distinct failures are distinguishable in `Record.DerivationError` | §10.3 |

---

## 2. Scope & Non-Scope

### 2.1 In scope

- The judgement arm's handling of a successful call carrying no text (§5, §6).
- The derivation arm's handling of the same (§8), **as a separate decision reaching a different answer**.
- The derivation arm's handling of a *non-empty but degenerate* response (§9).
- What the record and the response body carry about both (§6.4).
- Both adapters, equally (§5.3).

### 2.2 Out of scope — declined explicitly

| Declined | Where it belongs |
|---|---|
| **Whether `think` is sent on the judgement request, and with what value** | **#14130.** §11 states how this design changes that decision's stakes without making it |
| **Whether reasoning tokens are accounted in `Record.Usage`** | **#14130** decision 2. This design carries reasoning *text* on one branch; it does not touch usage accounting |
| **The `RENDERER qwen3.8` / `PARSER qwen3.5` modelfile mismatch** | Host configuration. Raised and refuted as the cause (§3.3); may still be worth fixing on its own merits |
| **Retry, backoff, jitter on any external call** | Already ruled, twice, and this design does not reopen it — §7.2 |
| **A temperature setting for either arm** | The sweep in flight (§3.1) informs #14130 and the operator's configuration; it changes no decision here |
| **Making the model *ask for clarification*** | `retrieval-admission-and-the-empty-outcome.md`. A different outcome with a different cause |
| **Prompt changes to either the system text or `DerivationPrompt`** | A prompt change is a tuning decision with a role that owns it. §7.6 states why it is not the remedy here |

---

## 3. Measured Facts, and Whose They Are

### 3.1 The measurements are the operator's, and here is exactly what they cover

**Taken by the operator, not by me. State: `qwen3.8:27b` on `gangolf:11434`, `/api/chat`, `stream=false`,
the product's own `DerivationPrompt` reconstructed by hand. Graph state not stated and not load-bearing —
the derivation prompt does not read the graph.**

| Arm | Condition | Result | n |
|---|---|---|---|
| Derivation prompt, **random seed**, `think` unset | the condition the operator calls *"the shipped request shape"* | **empty `content` on 2 of 12**, `done_reason: stop` | **12** |
| Derivation prompt, **fixed seed 1–5**, `think` unset | — | 0 empty, 5/5 six lines | 5 |
| Derivation prompt, **fixed seed 1–5**, `think: true` | **byte-identical to `think` unset** | 0 empty, 5/5 six lines | 5 |
| Derivation prompt, **fixed seed 1–5**, `think: false` | — | 0 empty, **3 of 5** six lines | 5 |
| Judgement-shaped prompt, `think` unset | *(from #14130, 4 seeds per arm)* | content 389–439 B, thinking 849–1,396 B, **11.5–24.7 s** | 4 |
| Judgement-shaped prompt, `think: false` | *(from #14130, 4 seeds per arm)* | content 299–369 B, thinking 0, **2.6–3.5 s** | 4 |

**The mechanism, and it is the single most load-bearing fact in this document:** in **both** empty cases the
`thinking` field **ends on the answer's own final line**. The model wrote a complete, well-formed answer
*inside* the reasoning block and never transitioned out of it.

**A rate is bracketed, not pinned.** *2 of 12* is a point estimate on a small sample from a sampled process;
a temperature sweep (1.0 / 0.6 / 0.0) was in flight when this was written and **may move it**. Every
argument below is built to survive any rate strictly between 0 and 1 — the design question is *what the loop
does when it happens*, and a rate changes only how often the answer is exercised. **No decision here is
contingent on the rate**, and §10 names no falsifier that depends on one.

**And the rate's temperature is unstated — added 2026-09-16, after #14154.** The product's shipped default
is **`defaultModelTemperature = 0.0`** (`internal/boot/config.go:31`, read at `9f49229`), substituted
whenever `PROCESSOR_MODEL_TEMPERATURE` is unset. The measurement above was taken **by hand against
`/api/chat`, not through the product**, so whatever temperature it ran at is whatever that hand-built
request carried, or whatever the endpoint's own default supplied — **not necessarily 0.0**. The rate
therefore may not describe the shipped configuration at all. This changes no decision (see the paragraph
above), but it does change the failure's *shape*, which §7.2.1 works out and §13 states to the user.

### 3.2 What is *not* measured, stated so nobody reads a gap as a zero

- **No measurement of a non-`qwen3.8` model.** Every figure is one model on one host.
- **No measurement through the shipped `Judge` path.** The operator reconstructed the *derivation* prompt
  and sent it with the *judgement* request shape. §3.4 is about exactly this, and it matters.
- **No openai-compat measurement at all.** Everything is `/api/chat`. §5.3 designs for that gap rather than
  assuming across it.
- **No measurement of what a caller downstream does with an empty answer.** There is no automated caller;
  `m1-skeleton-loop.md` §10.7 records that the caller is a human.

### 3.3 The parser/renderer theory is refuted, and is recorded so nobody re-derives it

The model's modelfile pairs `RENDERER qwen3.8` with `PARSER qwen3.5`, where every other model on the host
pairs them by the same name. **It is not the cause:** a mismatched parser would fail deterministically, and
this succeeds 10 times in 12 with the split done correctly. Recorded, and out of scope (§2.2).

### 3.4 A correction: the derivation arm does **not** send `think` unset — it sends `think: false`

**This is a fact about the tree, measured by me at `9f49229`, and it contradicts both the brief's framing
and #14130's own text.**

`ModelPort.Derive` is not implemented against `wire.go`'s `chatRequest` on either adapter. It delegates to
`Condense`:

```
internal/ollama/condense.go:116      func (c *Client) Derive(...) → c.Condense(...)
internal/openaicompat/condense.go:111 func (c *Client) Derive(...) → c.Condense(...)
```

and the ollama condensation request **carries the key**:

```
internal/ollama/condense.go:16   condenseThinking = false
internal/ollama/condense.go:40   Think bool `json:"think"`
internal/ollama/condense.go:64   Think: condenseThinking,
```

So #14130's sentence — *"The judgement path and the derivation path — every `Chat` and every `Derive` —
send no such key"* — is **true of the judgement path and false of the ollama derivation path.** And the
brief's *"`think` unset (the shipped request shape)"* describes the **judgement** request shape, applied to
the derivation prompt.

**Three consequences, and they change the design:**

1. **The 2-in-12 rate is a judgement-arm figure.** It was taken with `think` unset, which is what
   `internal/ollama/wire.go`'s `chatRequest` sends and what `internal/ollama/condense.go` does not. The
   prompt was a derivation prompt; the *request shape* was the judgement one. So the measurement's best
   reading is: **the judgement arm, on a thinking model, loses its whole answer about one call in six.**
   That is the arm with no fallback at all.
2. **The ollama derivation arm is already structurally protected** against this shape of emptiness. It
   turns the reasoning stream off, so an answer cannot be stranded inside it. Its residual risk is the
   *other* degradation the seeded table measures: `think: false` produced the demanded six lines on only
   **3 of 5** seeds. **A format problem, not an emptiness problem** — which is §9's subject, not §5's.
3. **The openai-compat derivation arm is protected by nothing.** `internal/openaicompat/condense.go:34-42`
   has no thinking control, because the OpenAI chat-completions protocol has no such request key. Whatever
   a reasoning model does there, it does unchecked.

**This does not weaken the case for the design; it sharpens where the design has to bite.** It moves the
urgency from the arm that has a documented fallback and a loud warning onto the arm that has neither, and
it is the reason §5 and §8 reach different answers.

**Falsifier for this correction:** any request body captured from `ModelPort.Derive` on the ollama adapter
that does not contain `"think":false`. At `9f49229` the encoding path is
`Derive → Condense → json.Marshal(condenseRequest{Think: false})` with no branch between them.

### 3.5 Neutrality baseline, measured at `9f49229`

| check over `internal/loop/*.go`, test files included | `9f49229` |
|---|---|
| `git grep -cE '"thinking"\|reasoning_content' HEAD -- 'internal/loop/*.go'` | **0** |
| `git grep -nE '\bthink\b' HEAD -- 'internal/loop/*.go'` | **2** — both `derivationThinkBlock`, `derive.go:75` and `derive.go:104` |

The two `think` hits are the **existing, documented accommodation**: a regexp that strips an inline
`<think>…</think>` block out of *content*. It is a different mechanism from the `thinking` *field*, and this
design does not touch it. **It stays at two** (§10.1).

---

## 4. Assumptions & Constraints

| # | Assumption or constraint | Confidence |
|---|---|---|
| C-1 | Both adapters must behave the same for the same endpoint behaviour; `what-the-adapters-may-share.md` §4 governs what may be shared | **stated, binding** |
| C-2 | `internal/loop` must gain no provider knowledge; the falsifier is §10.1 and the baseline is §3.5 | **stated, binding** |
| C-3 | The run bound is **10 minutes** (`internal/server/server.go:13`) and judgement with reasoning on measured 11.5–24.7 s per call against a cap of 6 | measured, §3.1 |
| C-4 | `MaxModelCalls = 6` counts **judgement** calls; `the-query-the-graph-is-asked.md` §4.4 pins that the derivation call is *not* charged to it | **read at ref**, `turn.go:17`, `turn.go:300` |
| C-5 | The caller is a human, and retries are the human's layer | `m1-skeleton-loop.md` §10.5, §10.7 |
| C-6 | A record is the expensive artifact and a `5xx` invites a retry that re-spends the model call | `m1-skeleton-loop.md` §6.6 table |
| C-7 | The response body already embeds the whole `Record` (`internal/server/routes.go:45-48`), so a new record member reaches the caller with **no handler change** | read at ref |
| C-8 | The shipped model temperature default is **0.0** — `defaultModelTemperature`, `internal/boot/config.go:31` — so the product runs greedy unless `PROCESSOR_MODEL_TEMPERATURE` is set | **measured at `9f49229`**; #14154 |
| C-9 | **This design depends on determinism nowhere.** No mechanism, invariant or falsifier in it requires a live model to be reproducible — F-8 | **derived**, §10 |
| A-1 | The openai-compat reasoning field is spelled `reasoning_content` on the endpoints this project would meet | **unverified — §12 Q-1** |
| A-2 | A promoted reasoning transcript is more useful to a human caller than an empty string | **unverified judgement — §12 Q-2, and it is Toni's, not a measurement's** |

---

## 5. The Decision — Judgement Arm

### 5.1 Decision 1: emptiness is a **terminal reason**, decided in the loop

> **A successful call whose answer has zero length ends with `TerminalReason.Empty`, and the loop assigns
> it — after the adapter has translated, from the adapter's output, with no reference to any provider
> field.**

**Why a terminal reason and not an error, and not a bare flag.** `TerminalReason` is documented as *"the
loop's own closed set of ways a judgement step can end"* (`types.go:86-87`). This is a way a judgement step
can end. `Answered` is documented as *"a completed prose answer with no pending tool request"*
(`types.go:90-91`) — an empty string satisfies the second half and fails the first, which is precisely why
`summary.go:31` had to invent a flag reading *"terminal reason says answered"* to contradict the field next
to it. **Adding the member makes the vocabulary true and makes that flag dead code** — a pleasant falsifier
in its own right (§10.4).

**Why the loop decides it and not the adapter — and this is the whole answer to C-1.** The classification
input is `JudgeResult.Answer`, which is **the loop's own data**, not a provider's field. By
`what-the-adapters-may-share.md` §4's rule — *"renders the loop's own data → `internal/loop`; declares
something to an endpoint → the adapter"* — the classification belongs in the loop. And that placement gives
C-1 **by construction rather than by discipline**: there is one site, so there is nothing for two adapters
to drift apart on. A per-adapter `if answer == ""` would be the seventeenth duplicated declaration that
document already counts, with no reason.

**Where.** In `judge()` (`turn.go:295-338`), immediately after `translate` returns and before the tool
branch. It must not alter control flow: `Empty` is not a tool reason, `wantsTool` returns false for it, and
the loop breaks exactly where it breaks today.

**The endpoint's own word is not destroyed.** `StopReason` is a pair (`types.go:145-149`): `Reason` becomes
`Empty`, `Raw` stays `"stop"` verbatim. Nothing is hidden; the two halves disagree and the record shows
both, which is the point of the pair.

**Reach.** The rule is *answer of zero length*, not *`Answered` with zero length*. A `Truncated` response
that carried nothing is also `Empty` — the model produced no text either way, and the caller's decision is
the same. `Refused` is the exception and is left alone: a refusal is a stated outcome with its own meaning
and `m1-skeleton-laid` treats it as a `200` outcome already; relabelling it `Empty` would delete
information. **Precedence: `Refused` wins over `Empty`; `Empty` wins over everything else.**

**Rejected:** see §7.1.

### 5.2 Decision 3: the reasoning channel is **read**, not parsed

> **Each adapter reads its own reasoning channel into a new `JudgeResult.Reasoning`. When `Answer` is empty
> and `Reasoning` is not, the loop promotes `Reasoning` to `Answer` and records
> `AnswerSource: reasoning`.**

**This is not a parse, and the distinction is the whole design.** The brief frames the option as *"parsing
an answer out of reasoning prose, and the reasoning is not contractually structured."* That framing is
correct about a parse and **the parse is not what is proposed.** What is proposed is: *the call produced
text on one channel instead of the other; take the channel, say which channel it was, change not one byte
of it.* No segmentation, no marker hunting, no "take the last paragraph", no contract asserted over the
reasoning's shape.

**The house pattern for this already exists and is already on the record.** `ToolSource`
(`types.go:111-119`):

> `ToolSourceNative` — *"a call the endpoint itself reported in its own tool-call field"*
> `ToolSourceContent` — *"a call the adapter recovered from the response text because the endpoint reported
> none"*

That is **this exact move**, one field over: *the thing arrived on the wrong channel, the adapter took it
from there, and the record names the channel so nobody mistakes it for the clean case.* `AnswerSource` is
`ToolSource` for the answer, with the same two-member shape and the same honesty property. It is not a new
idea in this codebase; it is an existing idea applied to the field next to the one that already has it.

**Why it is free where a retry is a whole call.** The measurement is unambiguous: *the thinking field ends
on the answer's own final line.* The answer the retry would go and buy **is already in the response you
paid for**, one JSON key away. A retry re-rolls a die whose winning face is visible in your hand.

**Why the whole channel and not a slice of it.** Because a slice is a claim about the reasoning's structure
and there is no such contract. The whole channel is a claim about *where the bytes were*, which is a fact.
The cost is verbosity — the caller gets a reasoning transcript ending in the answer, not the answer alone —
and it is **stated, marked, and paid only on the branch that would otherwise have returned nothing.**

**Precedence with §5.1.** Promotion runs first. If promotion happens, the terminal reason is whatever the
endpoint's own report mapped to (`Answered`, `Truncated`, …) and `AnswerSource` is `reasoning`. `Empty` is
reserved for *no text on any channel* — which keeps the two axes orthogonal:

| | `AnswerSource: native` | `AnswerSource: reasoning` |
|---|---|---|
| `Answered` | today's good case | the recovered case, **new** |
| `Truncated` | today's cut-off case | cut off inside the reasoning block |
| `Empty` | no text anywhere, **new** | *unreachable by construction* |

**Rejected:** a single fused reason (`AnsweredFromReasoning`) — it is a cross-product, and it would need a
member per existing reason. Orthogonal axes cost one field and stay correct when a reason is added later.
Also rejected: §7.3, parsing.

### 5.3 The adapters, and how C-1 is met without shared code

`what-the-adapters-may-share.md` §4's rule places every piece of this without a special case:

| element | renders the loop's data? | declares/reads to an endpoint? | side |
|---|---|---|---|
| reading `message.thinking` | no | **yes — a provider field name** | **ollama adapter** |
| reading `choices[].message.reasoning_content` | no | **yes — a provider field name** | **openaicompat adapter** |
| *"empty answer ⇒ `Empty`"* | **yes — `JudgeResult.Answer`** | no | **loop, one site** |
| *"empty answer + reasoning ⇒ promote, mark source"* | **yes — two `JudgeResult` fields** | no | **loop, one site** |

So the duplication is **one struct field and one assignment per adapter**, which is what that document
already rules should be duplicated without apology, and the *behaviour* — every branch a reviewer would care
about — lives at one site and is identical for both adapters because there is only one of it.

**ollama.** `wireResponseMessage` (`internal/ollama/wire.go:101-104`) gains `Thinking string
\`json:"thinking"\``. `translate` assigns it to `result.Reasoning`. Note the same struct is reused by
`condenseResponse` (`internal/ollama/condense.go:52-56`), which is harmless and mildly useful: the
condensation request pins `think: false`, so the field is empty there by construction and becomes a free
assertion (§10.5).

**openaicompat.** The field spelling is **A-1, unverified** (§12 Q-1). The design is built so the unverified
assumption is safe in both directions: **if the field is absent or differently spelled, `Reasoning` stays
empty, promotion does not fire, and the outcome is `Empty` — which is strictly today's outcome plus a
correct name.** No regression is reachable from getting A-1 wrong. The implementer confirms the spelling
against the configured endpoint before wiring it; §14 makes that a step.

---

## 6. Components, Contracts and Flow

### 6.1 Components and what changes

| Component | Owns | Changes | Does **not** own |
|---|---|---|---|
| `internal/ollama` | ollama's wire vocabulary | reads `message.thinking` | what emptiness *means* |
| `internal/openaicompat` | openai-compat's wire vocabulary | reads the reasoning field | what emptiness *means* |
| `internal/loop` — `judge()` | the turn's control flow and the closed reason set | classifies `Empty`; promotes reasoning; sets `AnswerSource` | any provider field name |
| `internal/loop` — `derive.go` | the query set and the window | two distinct causes (§8); records shape (§9) | promoting reasoning (§8.2) |
| `internal/loop` — `summary.go` | the operator's rendering | renders the source; **loses** the apology flag | deciding anything |
| `internal/server` | status codes and the envelope | **nothing** — C-7 | — |

### 6.2 Contracts (abstract)

**`ModelPort.Judge`** — its doc comment at `turn.go:81` (*"One attempt; no retry"*) **stands unchanged.**
This design adds no attempt.

**`JudgeResult` gains one member, `Reasoning`.** Semantics: *the text the endpoint reported on a channel it
distinguishes from the answer, verbatim and unmodified; empty when the endpoint reported none or has no such
channel.* Invariants: the adapter never merges it into `Answer`; the adapter never inspects it; an adapter
with no such channel leaves it empty and is fully correct.

**`Record` gains one member, `AnswerSource`.** A closed two-member set mirroring `ToolSource`. Semantics:
*which channel the answer in this record arrived on.* Absent (`omitempty`) on the native case, so every
existing record and every existing consumer is unaffected; present exactly when promotion fired.

**`TerminalReason` gains one member, `Empty`.** Semantics: *the call completed and produced no text on any
channel the adapter reads.* Invariant: `Empty` implies `Answer == ""`; the converse holds except under
`Refused` (§5.1).

### 6.3 Flow — a judgement step, after

```
  adapter                          loop.judge()                       record
  ───────                          ────────────                       ──────
  decode response
    ├─ content        → Answer ────┐
    ├─ reasoning chan → Reasoning ─┤
    ├─ tool calls     → Reason ────┤
    └─ done_reason    → RawReason ─┘
                                   │
                      ┌────────────▼─────────────┐
                      │ Answer != ""             │──yes──► unchanged, AnswerSource absent
                      └────────────┬─────────────┘
                                   │no
                      ┌────────────▼─────────────┐
                      │ Reasoning != ""          │──yes──► Answer := Reasoning
                      └────────────┬─────────────┘          AnswerSource := reasoning
                                   │no                      Reason unchanged (Answered/Truncated)
                                   ▼
                      Reason := Empty   (unless Refused)
                      RawReason keeps "stop" verbatim
```

Everything downstream of this box — `wantsTool`, the cap check, `dispatch`, `WriteRun`, the handler — is
untouched. **The change is one decision box on one edge.**

### 6.4 What the record and the response carry

| Case | `answer` | `stopReason.reason` | `stopReason.raw` | `answerSource` |
|---|---|---|---|---|
| normal | the answer | `answered` | `stop` | *absent* |
| **recovered** | the reasoning text | `answered` | `stop` | **`reasoning`** |
| **genuinely empty** | `""` | **`empty`** | `stop` | *absent* |
| truncated, no text | `""` | **`empty`** | `length` | *absent* |
| refused | `""` | `refused` | `content_filter` | *absent* |

Because the response embeds the record (C-7), **all of this reaches the caller with no change to
`internal/server`.**

---

## 7. Alternatives Rejected

### 7.1 Make emptiness an error at the adapter boundary — **rejected**

*The shape:* the adapter returns `fmt.Errorf("%w: empty content", …)`, `judge()` wraps it in
`ErrModelUnavailable`, the handler writes `502`.

**Rejected on three grounds, any one sufficient.**

1. **It is false.** `ErrModelUnavailable` is documented as *"any failure completing the model call itself"*
   (`turn.go:47-48`). The call completed. The endpoint was available, answered in time, reported `stop`,
   and reported its token counts. Calling that *unavailable* puts a lie in the one field an operator reads
   to decide whether to retry — and `what-a-failed-run-reports.md` exists precisely because that decision
   depends on the cause being true.
2. **It destroys the run.** `turn.go:170-173` discards the whole record on a judge error. A run that
   reached `Empty` may have made five prior calls, run two recall tools, and written a file to the
   workspace. All of it is thrown away, and the block, the dispositions and the usage with it.
   `m1-skeleton-loop.md` §6.6's write-back row already ruled this trade in the other direction: *"the
   expensive artifact already exists and is in the body. A `5xx` invites the caller to retry, which
   re-spends the model call."* Same argument, same answer.
3. **It would have to live in two adapters** and would therefore be exactly the drift C-1 forbids.

### 7.2 Retry — **rejected, on both arms**

*The shape:* on `Empty`, call again; up to N times; charged to `MaxModelCalls` or to a new budget.

This project has ruled against retry twice, on independent grounds, and **this case is weaker for retry
than either of the cases already rejected.**

| Standing ground | Source | Does it hold here? |
|---|---|---|
| Three unmeasured constants — backoff, budget, jitter | `m1-skeleton-loop.md` §10.5 | **Yes, and worse.** A fourth appears: *how many re-rolls of a die whose rate is bracketed, not pinned* (§3.1) |
| A caller waiting a multiple of an already-long generation, unable to tell a hung endpoint from a working one | `m1-skeleton-loop.md` §10.5 | **Yes, and measurably.** 11.5–24.7 s per judgement call with reasoning on (§3.1) against a 10-minute run bound (C-3) and a cap of 6 |
| An operator who could read the cause would wait | #13564 §3, via `the-query-the-graph-is-asked.md` §4.5 | **Yes** — and this design's entire first half is *making the cause readable* |
| A retry doubles the latency of a step in the branch where the endpoint has already shown it is slow | `the-query-the-graph-is-asked.md` §4.5 | **Yes** |
| An instrument that retries where the turn does not stops measuring the turn | `the-query-the-graph-is-asked.md` §18.2 | **Yes, and it is the sharpest.** A judgement retry makes `Record.ModelCalls` stop meaning *what the trajectory cost*, and every eval arm built on it silently measures the harness's repair instead of the model |

**And one ground specific to this case, which is the decisive one:**

> **The answer is already in the response.** The measurement says the model produced a complete, well-formed
> answer and put it on the reasoning channel. A retry pays a full model call — 11.5–24.7 s — to go and ask
> for a thing it is currently holding and declining to read. **Retry here is not merely expensive; it is
> the expensive way to avoid reading a JSON key.**

**The accounting problem, for completeness, because the brief asks which budget it charges.** There is no
good answer, which is itself an argument. Charged to `MaxModelCalls`, a retry consumes a step of the
model's reasoning budget to fix the harness's reading problem — the same coupling
`the-query-the-graph-is-asked.md` §4.4 rejected for the derivation call, where it showed that charging it
would have turned the one complete six-run sample's finishing trajectory into a capped one. Charged to a
new budget, `Record.ModelCalls` no longer counts the calls the run made, and the record becomes wrong
about what the run did — which the brief names as the failure mode to avoid.

#### 7.2.1 A sixth ground, independent of the five — and the five do not rest on it

**Added 2026-09-16 from #14154.** The product runs **greedy** unless someone sets an environment variable:
`defaultModelTemperature = 0.0` (`internal/boot/config.go:31`), plumbed into both adapters' judgement and
condense paths. At temperature 0.0, same prompt and same model, **a retry returns the same completion.**

So at the shipped default, retry is not merely unwise — **it is inoperable.** Self-healing by re-asking
requires something to differ: a non-zero temperature, an explicit seed, or a modified request. None is
present.

**This is recorded as a sixth ground, not as a premise of the other five, and that distinction is the
point.** Re-read the table above: not one of the five references sampling. Unmeasured constants, caller
latency, *an operator who could read the cause would wait*, *an instrument that retries stops measuring the
turn*, and *the answer is already in the response you paid for* are each true at any temperature — and the
decisive fifth is a claim about **where the bytes sat**, which sampling does not touch. **If the default
moves to 0.2 tomorrow, every one of the five survives unchanged and this ruling does not reopen.**

**Why it is written down anyway, and the reason is not the one it looks like.** The risk is not that a
stated support silently vanishes — it was never stated, so it cannot. It is the reverse: a reader finding a
five-ground argument here would reasonably conclude that retry was a *live option* argued down on judgement,
when at the shipped default it was never operable. **Anyone who later wants to reopen retry needs to know
that the first thing they must change is the temperature default**, and that is knowable only if it is here.

**What it does change — the failure's shape, and this is the part that matters.** At T≈0 the variation
source is largely gone, so the defect stops distributing over *draws* and starts distributing over
*prompts*: not *"one call in six fails"* but **"some prompts fail every time they are asked."** Two
consequences:

1. **The human-retries layer fails for an afflicted prompt.** `m1-skeleton-loop.md` §10.5 and §10.7 rest
   the no-retry position partly on *the caller is a human and the human retries* (C-5). A human re-asking an
   afflicted prompt at T=0 **gets the same empty answer again.** That standing fallback does not cover this
   case, and nothing else does.
2. **That makes §5.2 the sole remedy, not a convenience.** Reasoning recovery is not merely the cheap
   alternative to a retry here — at the shipped default it is **the only thing in the design that returns an
   answer at all** for such a prompt. It should be read as load-bearing accordingly.

**Hedged deliberately:** greedy decoding on a GPU is *approximately*, not bit-exactly, reproducible —
batching and reduction-order effects admit some variation. *"Every time"* is directional, not a guarantee,
and the direction is what the argument uses.

**What would change this.** Not a change to the temperature default — see above; the five grounds are
indifferent to it. What would reopen this section is **a measured case where the reasoning channel is also
empty at a non-trivial rate**, i.e. the endpoint genuinely produced nothing, repeatedly. Nothing in §3.1 is
that case: the two empty draws both had full reasoning. If that case is ever measured, this is the section
to reopen, and it should be reopened with a rate, not an anecdote (§3.1's own method note).

### 7.3 Parse the answer out of the reasoning prose — **rejected**

*The shape:* locate the answer inside `thinking` — last paragraph, text after a marker, the final N lines.

**Rejected.** The reasoning channel carries no contract. Every available locator is a heuristic, and each
one fails in the direction that is invisible: *"the last paragraph"* returns a self-critique when the model
ends by second-guessing itself; *"text after `Final answer:`"* returns nothing when the model does not
write that phrase, silently converting a recoverable case into an empty one; *"the last N lines"* fails on
every answer of a different length. A locator that is wrong returns **confident, well-formed, wrong text
with no marker on it** — which is the failure `what-a-successful-run-withheld.md` §1 was written about.

§5.2's channel read has none of this exposure because it asserts nothing about the reasoning's shape.

### 7.4 Carry the reasoning onto every record — **rejected, and it is #14130's to reopen**

*The shape:* `Record.Reasoning` populated on every call, promoted or not.

**Rejected here on cost and ownership.** The measurement has reasoning at 849–1,396 B on a judgement call
whose answer is 389–439 B — so records roughly triple for a thinking model, on every call, with **no
consumer named**. This design carries the reasoning only where it *became* the answer, where the consumer
is the person reading the answer. The general question — *should a record account for what the reasoning
cost* — is #14130 decision 2 and it belongs there, next to the usage accounting it actually bears on.

### 7.5 A validity filter on derived query lines — **rejected**, see §9.2

### 7.6 Fix it in the prompt — **rejected as the remedy, not as an action**

*The shape:* add *"do not write your answer inside your reasoning"* to the system text.

**Rejected as the remedy** for two reasons. It is unfalsifiable at design time — nobody can state what rate
it would move the 2-in-12 to without measuring it, and a prompt change that is not measured is not a
mitigation. And it is **per-model by nature** (`what-the-adapters-may-share.md` §4.2 point 4 rules that
prompt text is tuned per model and this repo already staffs a role for that), so it cannot be the
structural answer for a loop that must work against any configured model.

**Not rejected as an action.** It may well help, it is cheap, and it belongs to the prompt-engineering role
with a measurement attached. It is simply not what makes the loop correct, and the loop must be correct
when it does not help.

---

## 8. The Derivation Arm — Decision 4: a **different** policy, and why

> **The derivation arm does not get §5.2's promotion. It gets §5.1's honesty instead, in the form of two
> causes where it currently has one.**

### 8.1 The two arms are not alike, and the asymmetry is structural

| | judgement | derivation |
|---|---|---|
| Output contract | free prose | **exactly 6 lines**, one a `DATES:` directive, four ending in `?`, one keyword-style (`derive.go:34-38`) |
| On empty today | silent `200`, `answer: ""` | **loud** — `errDerivationUnusable` → WARN *"the query set fell back to the raw input alone"* (`turn.go:199`) |
| Fallback today | **none** | `queries = {input}`, ruled and stable (`the-query-the-graph-is-asked.md` §4.5) |
| Thinking on ollama | **unset** → on | **`think: false`** → off (§3.4) |
| Cost of a wrong recovery | a wrong answer to a human | **five wrong queries searched in the graph**, plus a lost retrieval window |

**Giving both arms the same policy would be wrong in the direction that does not show.** The judgement arm
needs recovery because it has no fallback; the derivation arm has a good fallback and a strict format that
makes recovery actively dangerous.

### 8.2 Why promotion is refused here specifically

`ParseDerivation` strips `<think>…</think>` from *content* (`derive.go:104`), then `parseQueryLines`
(`derive.go:183-203`) takes **the first `MaxDerivedQueries` distinct non-empty lines** and returns them as
queries. Hand it a reasoning transcript and it returns **the first five lines of the model's reasoning**, as
search queries, against the graph. That is not a degraded outcome — it is §9's degenerate case,
manufactured deliberately, five at a time. And it would be silent: five non-empty distinct lines is a
*successful* derivation by every test in the file.

**So: the reasoning channel is read by the adapter and, on the derivation path, ignored by the loop.** The
adapter change from §5.3 is one change serving one consumer; `DeriveQueries` is not that consumer.

### 8.3 What the derivation arm gets instead

**Split `errDerivationUnusable` into two causes.** Today one sentence — *"the model returned no usable
query"* (`derive.go:21`) — covers two genuinely different events, which is #14130 decision 3's request:

| event | cause, in `Record.DerivationError` | what an operator does |
|---|---|---|
| the call returned **no text at all** | *the model returned no text* | look at the model/endpoint — thinking, budget, health |
| the call returned text, **no line of which parsed** | *the model returned text, no line of which parsed as a query* | look at the **prompt** and the model's format compliance |

The degraded path itself is **unchanged, byte for byte**: `queries = {input}`, warn, continue
(`turn.go:198-201`). `the-query-the-graph-is-asked.md` §4.5 ruled that and it stands. **This adds a name, not
a behaviour** — which is the same move §5.1 makes on the judgement arm, and the reason the two arms differ
in *remedy* while agreeing in *principle*.

**Why this is worth doing when the ollama arm is already protected (§3.4).** Because the openai-compat arm
is not, because the protection is a constant that a future change could flip without noticing, and because
the second cause — *text that did not parse* — is the one the seeded `think: false` measurement actually
exercises (3 of 5 six-line compliance), and it is live on **both** adapters today.

---

## 9. Decision 5 — The Non-Empty Degenerate Response

**The case.** One sample returned `content` of *"We should not produce output"*. It is not empty, so
`errDerivationUnusable` never fires, so no warning is emitted; `parseQueryLines` keeps it as a derived
query, and **the graph is semantically searched for that sentence.** A policy keyed on emptiness cannot see
it. Neither can a retry.

**And it costs a second thing nobody has named.** That response has no `DATES:` line, so
`windowFromDirectiveLines` (`derive.go:118-144`) returns a zero `UpdateWindow` — **the retrieval window is
lost too**, and `Record.Window` is `omitzero`, so its absence is indistinguishable from *"the request
expressed no time constraint"*. On a time-bounded request that is a silent widening of retrieval.

### 9.1 The decision: **observe the shape; do not gate on it**

> **The record carries how many derived queries were obtained and whether the `DATES:` directive was
> present and parsed. The loop warns when the set is short. Nothing is rejected.**

Both facts are **structural, not heuristic**: the prompt demands six lines of which the first is a directive
(`derive.go:34-38`); *"one query was obtained where five were demanded"* and *"no directive line was
present"* are counts, not judgements about meaning. The degenerate sample fails both, loudly and without
any test that could misfire on a legitimate query.

The precedent is exact. `the-query-the-graph-is-asked.md` §18.2 ruled for the derivation *generator*:

> *"What replaces the fill loop, and this is what makes the trade acceptable: a short row is **visible, not
> silent.** `shapeProblems` emits a WARN whenever a row's query count is not `loop.MaxDerivedQueries`, so
> raggedness is read as a measurement of the product instead of being repaired into a measurement of the
> instrument."*

**The instrument already does this. The product does not.** That asymmetry is the defect, and closing it is
this decision. It also restores the derivation arm's parity with the judgement arm's existing shutout
warnings (`turn.go:235-247`), which warn on exactly this class of quiet under-delivery.

### 9.2 Rejected: filter or reject on line validity

*The shape:* drop lines that do not end in `?`, or that read as meta-commentary; fall back if too few
survive.

**Rejected on two grounds.**

1. **It repairs the measurement.** §18.2's rule is explicit — raggedness must be *read as a measurement of
   the product*, not repaired. A filter makes `Record.Queries` describe what the harness kept rather than
   what the model produced, and every eval arm downstream then measures the filter.
2. **It buys nothing even in the case that motivates it.** The raw input is already query #1
   (`MergeQueries`, `derive.go:210-218`) and stays. Rejecting *"We should not produce output"* does not
   improve retrieval; it removes one of twenty candidate slots' worth of noise (`CandidateLimit = 20`,
   `RecallScopeReserve = 3`) while risking the removal of a legitimate query that happens to lack a
   question mark — which the prompt's own sixth line, *"a dense, keyword-style query (no question
   mark)"*, **requires to exist**. The filter that catches the degenerate case also catches the mandated
   keyword line. That is not a tuning problem; it is the rule being wrong.

---

## 10. Falsifiers and Coverage

Each names the command and the state it must hold in. **A falsifier that fires on compliant code is worse
than none** (`what-the-adapters-may-share.md` §3.5) — each below was chosen against that.

| # | Falsifier | Baseline at `9f49229` | Must hold after |
|---|---|---|---|
| **F-1** | `git grep -cE '"thinking"\|reasoning_content' HEAD -- 'internal/loop/*.go'` — C-2, a provider field name reaching the loop | **0** | **0** |
| **F-2** | `git grep -nE '\bthink\b' HEAD -- 'internal/loop/*.go'` — the documented accommodation does not grow | **2**, both `derivationThinkBlock` | **2** |
| **F-3** | Paired port test: **one** table of endpoint behaviours, run against **both** adapters, asserting identical `JudgeResult` + post-classification outcome. Rows: content only · content + reasoning · **reasoning only** · neither · tool call + reasoning | does not exist | passes — **this is C-1's guard** |
| **F-4** | `summaryAnsweredYetEmpty` (`summary.go:31`) is **deleted**. Its condition — `Answer == "" && Reason == Answered` — is unreachable once §5.1 ships. If it is still referenced, §5.1 did not land | live, `summary.go:31,209` | **absent** |
| **F-5** | Any request body captured from `ModelPort.Derive` on ollama lacking `"think":false` falsifies §3.4 | holds by construction | holds |
| **F-6** | `Record.ModelCalls` for a fixed trajectory is unchanged by this design — S-3, and the guard against a retry arriving later by the side door | — | equal |
| **F-7** | A record with `answerSource` absent decodes identically to a record written before this change — the `omitempty` compatibility claim | — | passes |
| **F-8** | **No determinism dependency** (C-9). Every falsifier above is a grep (F-1, F-2), the absence of a symbol (F-4), an encoding fact true by construction (F-5), or a table run against a **stubbed** port (F-3, F-6, F-7). None calls a live model, so **changing `defaultModelTemperature` reddens nothing in this design.** Falsified by any test added under this design that needs a real endpoint to return the same bytes twice | — | holds — and `m1-skeleton-loop.md` §10.6 already forbids an automated model gate at any tier: *"a gate asserts, and a live model cannot be asserted against"* |

**The `AnswerSource` naming trap, named so the implementer does not step in it.** A grep for `reasoning` in
`internal/loop` will fire on the loop's **own** `AnswerSourceReasoning` constant, which is compliant code.
F-1 therefore greps the **quoted wire spellings** (`"thinking"`, `reasoning_content`) and not the bare word
— the same discrimination `what-the-adapters-may-share.md` §3.5 had to make between `recall` (collides) and
`write_file` (separates).

---

## 11. How This Divides With #14130

The two overlap and must not both decide the same thing.

| Question | Owner |
|---|---|
| Is `think` sent on the judgement request, and with what value? | **#14130** |
| Is `think` sent per call site, or once? | **#14130** |
| Are reasoning tokens accounted in `Record.Usage`? | **#14130** |
| Is the reasoning **text** read off the response at all? | **this design** — §5.2. #14130 names the discard; this decides what reading it is for |
| What does the loop do when the answer is empty? | **this design** |
| Does the derivation arm distinguish *no text* from *unparseable text*? | **this design** §8.3, answering #14130's decision 3 |

**This design changes #14130's stakes in one direction and corrects it in another.**

- **It de-risks turning thinking on.** #14130's live worry is that a thinking model returns nothing about
  one call in six. With §5.2 shipped, *that call returns the answer* — verbose and marked, but returned. So
  #14130's decision 1 becomes a **latency-and-quality** trade (11.5–24.7 s vs 2.6–3.5 s) rather than a
  latency-versus-blank-answers trade. **That is a materially easier decision, and it should be made after
  this ships, not before.**
- **It corrects #14130's derivation claim.** §3.4: `Derive` routes through `Condense`, which sends
  `think: false` on ollama. #14130's *"every `Chat` and every `Derive` send no such key"* is true only of
  `Chat`. The correction lowers the derivation arm's severity on ollama and leaves it untouched on
  openai-compat — and it should be filed against #14130 rather than carried only here.

**Recommended order:** this design first, #14130 second. The reverse order makes #14130 decide decision 1
under a risk this design removes.

---

## 12. Open Questions

**Needing a measurement, not Toni** — the implementer resolves these and they block nothing at design time:

- **Q-1 (A-1).** What does the configured openai-compat endpoint actually call its reasoning field, and does
  it expose one at all? §5.3 is built so a wrong guess degrades to today's behaviour plus a correct name,
  so this gates the *value* of the openai-compat arm, not its *correctness*.
- **Q-2.** Does the temperature sweep move the 2-in-12? It changes no decision here (§3.1) but it belongs on
  #14130 and it belongs in the record of what the rate is.

**Genuinely needing Toni** — judgement calls I should not make alone:

- **Q-A — the one that actually matters. Is a reasoning transcript an acceptable thing to hand a caller as
  the answer?** §5.2 promotes the whole channel: the caller gets the model's thinking, ending in the
  answer, marked `answerSource: "reasoning"`. The alternative is to return `Empty` with no text and let the
  human re-ask. **My recommendation is to promote** — a verbose answer that contains the answer beats an
  empty one, and the marking makes it impossible to mistake. **But it is a product judgement about what a
  user should be shown, it is A-2, and it is unmeasured.** If the answer is *"no, I would rather get
  nothing than get a wall of reasoning"*, §5.2 becomes *record the reasoning, do not promote it* — a
  one-line change to this design and no change to §5.1, §8 or §9.
- **Q-B.** Should `Empty` be visible at the HTTP layer as something other than `200`? This design says
  **no** — C-6's argument (the record is the expensive artifact; a `5xx` invites a re-spend), and the
  caller can already branch on `stopReason.reason`. **But if a non-human caller is ever expected, that
  answer changes**, and Toni knows whether one is coming. `m1-skeleton-loop.md` §10.7 names *"a retrying
  automated producer, which does not exist"* as the thing that would change several rulings; this is
  another of them.
- **Q-C.** §7.6 leaves a prompt-side mitigation unowned. Should the prompt-engineering role be given
  *"stop the model answering inside its reasoning"* as a measured task alongside this? It is independent of
  everything here and might cut the rate cheaply — but only Toni decides whether it is worth a role's time.

---

## 13. What a User Gets — plainly

**Today**, on the draw where the model writes its answer inside its reasoning:

> The caller `POST /runs`, waits 11–25 seconds, and receives `200 OK` with a complete, well-formed JSON
> body: the block that was assembled, the candidates that were admitted, the token counts, the model id —
> and `"answer": ""` beside `"stopReason": {"reason": "answered", "raw": "stop"}`. **The response says the
> model answered and contains no answer.** The run cost a full model call. Nothing in the body indicates
> that anything went wrong, and the only thing in the system that noticed printed a line in an operator
> summary that no code reads.

**After**, same draw:

> The caller receives `200 OK` and **the answer is in the body** — the model's reasoning, ending in the
> complete answer it wrote there, with `"answerSource": "reasoning"` next to it saying which channel it
> came from. No second model call was made. No extra second was spent. The bytes were in the first
> response the whole time.

**After**, on the draw where the model genuinely produced nothing on any channel:

> The caller receives `200 OK`, `"answer": ""`, and `"stopReason": {"reason": "empty", "raw": "stop"}`.
> **The response now says what happened.** A human reads *empty* and re-asks. A script can branch on it.
> The record — block, candidates, tool calls, usage, workspace — survives intact instead of being thrown
> away behind a `502`.

**And on the derivation arm:**

> A run whose derivation returned nothing, and a run whose derivation returned prose that parsed as nothing,
> stop sharing one sentence. The first points at the model; the second points at the prompt. And a run whose
> derivation came back malformed — one line where six were demanded, no `DATES:` directive — **says so in
> the record and in a warning**, instead of quietly searching the graph for *"We should not produce
> output"* and quietly discarding the time window.

**What a user does not get:** a retry. Not on either arm. The loop still makes exactly the calls it makes
today, and when the model genuinely produces nothing twice in a row, the person asking is still the one who
decides to ask again — which is the layer this project has twice ruled that decision belongs to.

**And one thing the user could not have got any other way** (added 2026-09-16, #14154). The product runs
greedy by default — `defaultModelTemperature = 0.0` (`internal/boot/config.go:31`). So for a prompt that
strands its answer in the reasoning block, **asking again returns the same empty answer**, and asking a
third time returns it again. The failure does not distribute over attempts; it sticks to the question.
Today such a user has no route to an answer at all — not a retry in the loop, and not a retry by hand.
**After, they get their answer on the first ask**, because it was in the first response the whole time.
§7.2.1 carries the reasoning and its hedge.

---

## 14. Implementation Guidance

**Ordered. Each step is independently reviewable. Steps 1–4 are one PR; step 5 is a second PR; step 6 is a
third.** They are separable features with separable value, per the one-feature-one-PR rule.

**PR 1 — the judgement arm names its outcome.**

1. `types.go`: add `Empty` to `TerminalReason` with a doc comment saying *the call completed and produced
   no text on any channel*; add `AnswerSource` as a closed two-member set beside `ToolSource`, modelled on
   it; add `Reasoning` to `JudgeResult` and `AnswerSource` to `Record` (`omitempty`).
2. Adapters, one field each: `internal/ollama/wire.go:101-104` gains `Thinking` and `translate` assigns it.
   **First confirm Q-1** against the configured openai-compat endpoint; wire the confirmed spelling, or
   leave the field unread and record why, which is a correct outcome (§5.3).
3. `turn.go`'s `judge()`: the one decision box of §6.3, between `translate` and the tool branch. Promotion
   first, classification second, `Refused` exempt. **No control-flow change** — F-6.
4. `summary.go`: **delete** `summaryAnsweredYetEmpty` and its branch (F-4); render `answerSource` on the
   answer line when present. Tests: F-3's paired table, F-7's compatibility case.

**PR 2 — the derivation arm names its two causes.**

5. `derive.go`: split `errDerivationUnusable` per §8.3. `DeriveQueries` distinguishes *text was empty* from
   *text parsed to nothing*. `turn.go:198-201`'s fallback and warning are **unchanged in behaviour**; only
   the cause string differs.

**PR 3 — the derivation arm records its shape.**

6. `derive.go` / `turn.go`: carry derived-query count and directive-presence onto the record; WARN on a
   short set, mirroring `turn.go:235-247`'s existing family and `shapeProblems`' rule from
   `the-query-the-graph-is-asked.md` §18.2. **No rejection, no filter** (§9.2).

**Do not:**

- Add a retry, a backoff, a jitter, or an attempt counter anywhere. §7.2.
- Put a provider field name in `internal/loop`. F-1.
- Parse, slice, or trim the reasoning text. §7.3 — carry it whole or not at all.
- Feed the reasoning channel to `ParseDerivation`. §8.2.
- Filter or reject a derived query line on its content. §9.2.
- Change `internal/server`. C-7 — the record is already the body.
- Ship steps 1–4 and 5 and 6 as one PR.
