# Architectural Document: What a Failed Run Reports

> **Repo path:** `docs/architecture/what-a-failed-run-reports.md` (canonical copy).
> **DiVoid node: #13564** — it carries this file verbatim, uploaded from the path above so the two
> sides are byte-identical. A parity publish goes to **#13564 and no other node**.
> **Source task:** **#11312** (*the error we print is not the error that happened* — three instances,
> raised to severity 4 on 2026-09-10). **Ordering authority:** **#13534** (the product briefing) §8,
> which places #11312 **second** in the MVP measurement loop, behind the container. **Sibling
> instance:** **#13542** (the same discard one tool over, in the container's packaging).
> **Project:** #10422 · **repo map root:** #10454 · **run-record fate:** #13238 /
> `run-record-fate.md` · **embedding-window measurement:** #13481 · **summary-template open
> decisions:** #13460 · **adapter-sharing line:** #13345 / `what-the-adapters-may-share.md` ·
> **prior art on this exact secrecy question, a different repo:** #9939 (`Pooshit.Http`,
> query-redaction and body removal).
> **Standards applied:** Design Contracts **#1136** (§1 KISS/DRY/YAGNI load-bearing; the §5 checklist
> is walked as §14 of this document), DRY threshold **#1267**, Go Code Contracts **#11034** (**P-51**
> — a transcribed rationale is an asserted claim; **P-18** — the wire suite drives a failure-path
> fixture), comment contract **#10861**, falsifier discipline **#12958 §18.6** (an untested account
> names its own **differential** falsifier in the same passage that states it).
> **Hard dependency: #13565 must merge before §15's gated steps.** It carries the report-time URL
> redaction for **both** config-derived carriers. Until it lands, Step 3 would carry the graph URL
> into the record, the substance and the model, and **Step 4 would carry it into the HTTP response**
> by a different route — `routes.go` has no `case` for `ErrGraphUnavailable`, so graph errors reach
> the `default:` arm that Step 4 rewrites. **§15 Step 0 is the one place the gated set is enumerated**
> and this header deliberately does not restate it; §17 explains why.
> **Review history:** round 1 QA **#13574** — REJECTED on one critical fail (§4.3's sweep visited one
> credential carrier and stopped) plus six warnings. This is revision 2; §16 records what changed.
> **Baseline:** `main` at **`0df5c14`**, tree clean. Every line number and figure in §2 was read out
> of that tree. Read-only git; nothing here was committed by its author.

---

## TL;DR

**What.** A failed run stops reporting a constant. The cause it was handed — the upstream's own
sentence, or the operating system's — is carried into the HTTP response, into the stored record, and
to the model, **bounded to 512 runes**; and it is carried into the operator's log **whole**. Four
facts that were in hand at every recorded failure and are reported nowhere today — **elapsed against
the bound that was hit**, the **assembled request's size in bytes**, the **model id** and the
**endpoint** — become a short fixed preamble on every model-call failure, composed by the adapter,
which is the only layer that knows **three** of them (the model id also lives on `Turn.ModelID`; the
other three do not survive a failed call at all).

**How.** No new error kind, no taxonomy, no structured failure object, no failure record. The change
is *one wrap and four un-discards*: each model adapter wraps its judgement call once at its outermost
point, attaching the four facts; and the four sites that today overwrite a cause with a constant
(`turn.go:326`, `turn.go:339`, `turn.go:356`, `routes.go:88`+`:90`) carry the bounded cause instead.
Three constants stay exactly as they are, because at those sites the loop is genuinely the author of
the failure.

**Cost, where non-zero.** A tool-round error text grows from a 17–29-rune constant to at most 512
runes; it is re-sent to the model on every subsequent call in that turn, so the worst case adds
≤2,048 UTF-8 bytes to a prompt measured at 15,304 input tokens on record #13472 (the token cost of
those bytes is **not** measured here and no rune-to-token ratio is asserted). The run-record
summary's error line widens from 88 to 200 runes, at most **+112 bytes** on a substance measured at
2.0–3.8 % of the record (#13245). Nothing else grows: no new node, no new field, no new type, no new
package.

**Strongest rejected alternative — a structured failure object plus a stored failure record.**
Rejected on three grounds, each measured rather than asserted: (1) no consumer can be named for the
next four weeks (#1136 §5 / #868) — the three recorded instances were each read by one human within
minutes and then written up by hand; (2) **at `0df5c14`**, on the record, the new fields would be
declared after `candidates` and therefore land **past** DiVoid's 8,000-character embedding cap
(#13481 measures `candidates` alone at 93.2 % of that window), so a stored failure would be
unfindable by the search that is supposed to retrieve it — a §2 form-1 data dump with extra steps.
**That figure is a state of another artifact and it is dated on purpose:** #13481's own
suggested-work item 2 proposes reordering `Record` so the outcome fields precede `candidates`, which
would make a new field embeddable and would weaken this ground. Q2's reversal-cost cell already names
that surgery; (3) #11312 rules the point
directly and this document does not reopen it: *"Not a new error type per cause.
`ErrModelUnavailable` is a correct classification for routing the status code — the defect is that
the detail dies with it, not that the class is too coarse."*

---

## 1. Problem Statement

A run that fails must tell the operator **what failed**, in the HTTP response and in whatever record
survives, without becoming a credential leak.

The measurement loop the whole product is now aimed at (#13534) is: Toni starts the container, gives
it a real task, it probably fails, and **that failure is the first real measurement**. #13534 §8
records that the loop does not close:

> **A failure that reports one opaque code is not a measurement.** It is a prompt to start an
> investigation with tools the product does not provide.

Three instances are on record in #11312, with **three different upstream causes** and **three
different correct operator responses**, rendered **identically**:

| # | what happened | what the operator saw | correct response |
|---|---|---|---|
| 1 | 2026-09-05, openaicompat: the endpoint rejected the request deterministically (*"output does not match the expected peg-native format"*) | `502 model_unavailable`, *"the model call did not complete"* | change the request |
| 2 | 2026-09-10, ollama: the endpoint was cold-loading a 30 B model | identical | retry |
| 3 | 2026-09-10, ollama: the model was serving from **system RAM, not VRAM**, so a ~15 k-token prompt exceeded a five-minute bound at **302 s** | identical | fix the host, or shrink the prompt |

Instance 3 took four separate hand measurements to diagnose — `/api/ps` on the model host, a second
full run against a different model, a container-network probe, and a small-prompt call through the
identical path — **by someone with shell access to the model host.** Toni will not have that.

A fourth instance, #13542, is the same defect one tool over: the container ran as uid 65532 against a
root-owned volume, the record said `file write failed`, and only the container's stderr said
`permission denied`.

**Success criteria.** For each of the four recorded instances, a reader with **only** the HTTP
response and the container's stderr can name the remedy without querying the model host.

---

## 2. The measured shape at `0df5c14`

### 2.1 Five sites, and the one thing they do not agree about

| # | site | what the log gets | what the caller / record gets |
|---|---|---|---|
| A | `turn.go:334-336` — a write the workspace **rejected** (`ErrWriteRejected`) | *nothing* | `err.Error()` — **the real message** |
| B | `turn.go:323-327` — `OpenRun` failed | the real error | `errFileWriteFailed`, a constant |
| C | `turn.go:338-340` — `Write` failed, unrecognised | the real error | `errFileWriteFailed`, a constant |
| D | `turn.go:355-356` — supplementary recall failed | the real error | `errSupplementaryRecallFailed`, a constant |
| E | `turn.go:243` → `turn.go:143-146` → `routes.go:88` — the model call failed | **nothing at all** | `model_unavailable` + a constant |

Row A is the only row that preserves a cause, and it is the row whose message the loop **already
knows**: `internal/workspace/workspace.go` authors all eight `ErrWriteRejected` reasons itself, from a
closed set (`path must not be empty`, `path must not leave the working directory`, …).

> **The asymmetry is exactly inverted.** A *recognised* error is the one a reader can already guess
> from the closed set; an *unrecognised* one is the only case where the text is the whole of the
> information. The code preserves the guessable and discards the informative.

**And that is not a framing — it is what one function does by construction.**
`internal/workspace/workspace.go:153-159`:

`classify` branches on `errors.As(err, &errno)`. On a `syscall.Errno` it returns the error plainly
wrapped; on anything else it returns `rejected(reasonOutside)`, one of the closed set. So **row A *is*
the non-errno branch and row C *is* the errno branch** — the split between "preserved" and "discarded"
is *literally the test for whether the operating system authored the text*. Row C is selected by
carrying an errno and is then the only row whose text is thrown away. Nothing else in this document
needed to be argued once that was read; the rest follows.

Row E is worse than rows B–D, and worse than the framing this design was briefed with: `judge()`
wraps with `fmt.Errorf("%w: %v", ErrModelUnavailable, jerr)` at `turn.go:243` and **no site logs it**;
`Run` returns at `turn.go:143-146`, before `logFinished`, so the run emits `run started` and then
silence. At the model site the cause is not merely discarded from the record — **it is discarded
everywhere.**

### 2.2 Two bounds are live, and the failing one is never named

- `internal/ollama/client.go:18` and `internal/openaicompat/client.go:18`:
  `DefaultTimeout = 5 * time.Minute` — **per model call**.
- `internal/server/server.go:13`: `runBound = 10 * time.Minute` — **per run**, and checked first at
  `routes.go:83`, so a run-deadline kill is correctly reported as `504 run_deadline_exceeded`.

Instance 3 died at 302 s — the **adapter's** ceiling, not the run's. It was reported as
`model_unavailable`, which is the right class and says nothing about whose clock ran out. Go's own
transport error for that case reads
`Post "http://gangolf:11434/api/chat": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`
— it names *a* timeout and never the number, and with two ceilings live the number is the whole
question.

### 2.3 What the record can and cannot carry

- **A failed model call writes no record at all.** `Run` returns `Record{}, WriteReceipt{}, err` at
  `turn.go:143-146`; `WriteRun` (`turn.go:159`) is never reached. There is no stored artifact for
  instances 1–3.
- **A failed tool round does write a record.** The run returns `200`; `toolCalls[i].error` carries the
  text; `RenderSummary` renders it (`summary.go:148-149`) truncated to `summaryQueryRunes = 88`
  (`summary.go:13`); the record is filed and its substance set (`divoid/write.go:56-72`).
- **A stored record never re-enters a later block.** `IsRunRecord` marks it `SelfProduced`
  (`divoid/client.go:185`) and `admit()` cuts every self-produced row (`assemble.go:52-53`) in
  **both** the primary assembly and supplementary recall, because both go through the same `admit`.
  The premise that a stored record is *"rendered into later blocks"* is **false at `main`** and this
  design does not rely on it. What is true is that a record is **recalled** (consuming one of
  `CandidateLimit = 20` slots) and **embedded**.
- **The embedding window is already spent.** #13481 measures DiVoid embedding
  `name + "\n\n" + content` capped at 8,000 characters, of which `candidates` occupies **93.2 %**;
  `answer`, `model`, `stopReason`, `toolCalls`, `limits` are **all past the cap** on live record
  #13472. Any field appended to `Record` today is, for search purposes, invisible.
- **The tool error has a third consumer nobody names.** `RenderToolResult` (`assemble.go:84-87`)
  returns `"error: " + r.Error` — the exchange error **is the text the model reads** before deciding
  what to do next. Under #13542's instance the model was told `file write failed`, which is
  indistinguishable from *"your path was wrong, try another"*, and it has up to `MaxModelCalls = 6`
  rounds in which to keep trying.

---

## 3. Scope and Non-Scope

**In scope.** What a failed run reports at rows B, C, D and E of §2.1; the four facts the brief names;
where each of them goes; the bound applied to a carried cause; and the secrecy ruling that governs all
of it.

**Redaction is no longer in scope, and that is a round-2 change.** Revision 1 folded the
`Provider.Endpoint` redaction into §7.2 and called it *"the one pre-existing credential exposure this
design would otherwise promote"*. **There were two, and the count was the defect** — QA #13574 §3
found the second, and §4.3(b) now records both. Both are carried by **#13565**, which was split out
and widened to cover them, and which this design now depends on rather than contains. **This design
performs no redaction and asserts no redaction invariant of its own** (§4.4, §10 item 4, §15).

**Out of scope, explicitly.**

1. **Retry.** #11312 rules it out and instance 2 does not reopen it: *"an operator who could read
   'model is loading' would wait."* Reporting the cause is what makes the human's retry decision
   possible; automating it would turn a fast clear failure into a slow one.
2. **A new error code or a per-cause error type.** #11312 §*What it does not need*, quoted in the
   TL;DR.
3. **The confident answer over a failed write.** #13542's *"they ask for a file, get a confident
   answer, and find no file"* is a prompt-and-loop question — should a refused tool round constrain
   the final answer? — with a different owner. The render half is already filed as **#13460 D5**.
4. **The summary template's open decisions** (#13460 D3, D5, and the third question). This design
   touches exactly one constant in `summary.go` (§7.4) and takes no position on the rest.
5. **The `candidates`-eats-the-embedding-window problem** (#13481). This design is careful not to
   *depend* on the window, and adds nothing to it.
6. **The workspace-ownership fix itself** (#13542's five options). This design makes its failure
   legible; it does not choose the packaging remedy.
7. **Query derivation** (#13534 §4.2), which is sequenced third, after this.

---

## 4. The ruling on the secrecy inconsistency

This is the load-bearing question and everything downstream turns on it, so it is settled first.

### 4.1 The contract, quoted

`README.md:199` and `:203`, for `PROCESSOR_DIVOID_KEY` and `PROCESSOR_MODEL_KEY` respectively:

> *"Never logged, never echoed in an error, never written to the graph."*

That is a contract about **two specific values**. It is not a contract about arbitrary text, and the
current code reads it as if it were.

### 4.2 The inconsistency, restated precisely

Rows B, C and D log the real error and then substitute a constant. If arbitrary error text is unsafe,
the log is already the leak. If it is safe, the constant protects nothing. Row E resolves the tension
in the worst available direction: it withholds the text from **both**.

### 4.3 The ruling

> **The secrecy argument does not survive contact with the code, and it is not the argument that
> should govern. Carry the cause, in both places, bounded — and replace the secrecy justification
> with a budget-and-durability one.**

Three findings support it, each checkable at `0df5c14`:

**(a) Neither key can enter an error string by any route this codebase controls.** Both are sent
**only** as `Authorization` request headers — `ollama/client.go:101`, `openaicompat/client.go:88`,
and the graph client's own send path. Neither is ever placed in a URL, a query string, or a request
body. Go's transport failures surface as `*url.Error`, which carries `Op`, `URL` and the wrapped
cause and **never** the request headers; a non-2xx surfaces as status plus the response body read
through `readUpstreamMessage`, capped at 4,096 bytes. **The only remaining route is a foreign server
echoing our own `Authorization` header back inside its error body** — and if that ever happens, the
key is already on stderr today at rows B–D and inside `readUpstreamMessage`'s return value regardless
of what the record does. The constant in the record has never protected against it.

> **Falsifier for (a), differential, named here because the claim is a claim about an absence
> (#12958 §18.6 clause 2, #12935).** Two arms against a local test upstream, with
> `PROCESSOR_MODEL_KEY` set to a distinctive sentinel. **Arm 1 — the detector's liveness:** an
> upstream that echoes the request's `Authorization` header into its 500 body; the sentinel **must**
> appear in the wrapped error. If it does not, the detector is blind and (a) is untested, not
> confirmed. **Arm 2 — the claim:** the sentinel must appear in **none** of the **five** failure modes
> §15 Step 2 enumerates — encode, build, transport refusal, non-2xx, decode. Revision 1's Arm 2 named
> three and left encode and decode to *"almost certainly safe"*, which is the reasoning this falsifier
> exists to replace; the two arms and Step 2's five modes now quantify over the same set, so a mode
> cannot be covered by one and missed by the other. The pair is differential: it can return *"text
> does reach the error, and it is not specific to the echo route"*, which falsifies (a) rather than
> confirming it.

**(b) The record already carries config-derived strings that can hold a credential, and nothing
guards them. There are two carriers, not one.** Revision 1 named one and stopped; QA #13574 §3 found
the second, and the miss is instructive — it is **P-52**'s shape exactly, *a finding that lists sites
is a finding about a claim, and the remedy is scoped to the claim, never to the list*. The row here is
**every config-derived string that can reach a carried cause**, and revision 1 enumerated by the
phrase it happened to be looking at.

| carrier | how it reaches a cause | where this design would newly send it |
|---|---|---|
| `PROCESSOR_MODEL_URL` → `Provider.Endpoint` | composed as base URL + route and reported on the **success** path; the failure preamble would report it too | already in `record.provider.endpoint`, the substance (`summary.go:57-61`) and the log (`turn.go:178`) **today**; the preamble adds the response |
| `PROCESSOR_DIVOID_URL` → `*url.Error` | `divoid/client.go:236` builds `reqURL` from `c.baseURL`; `:247` and `:285` wrap the `*url.Error` from `Do`, whose `Error()` prints the **whole request URL, userinfo included**. `loadGraph` (`boot/config.go:101-116`) stores the value verbatim and `rejectAPIBase` inspects only `parsed.Path` — it **never reads `parsed.User`** | **stderr only today.** Under this design, row D carries it into the **record, the substance and the model**, and `routes.go:90`'s `default:` arm carries it into the **HTTP response** — four new destinations, on the most common graph failure |

Neither is a live leak: `loadModel` never parses its URL and `PROCESSOR_MODEL_KEY` is the documented
way to authenticate, so userinfo-in-URL is an operator error with a working alternative already in the
README. Both are **permitted-input hazards**, which is enough to fix and not enough to call an
incident.

**§11's accepted trade-off does not cover them.** That paragraph prices *upstream-returned* text
against local endpoints. This is **our own configuration**, entering through Go's own `*url.Error`,
deterministically.

**Both are #13565's, not this document's.** Revision 1 folded the endpoint half into a §7.2 that no
longer exists. #13565 was split out — on the precedent of #13481, split from #13480 on this same
surface, because redaction acts on the **success** path and changes every successful record's
serialization — and widened to both carriers. **This design therefore performs no redaction and
states no redaction invariant of its own.** What it states instead is a **precondition** (§4.4).

> **Falsifier for (b), differential — and it belongs to #13565, which is why it is restated there
> rather than owned here.** Per carrier: build the client over a base URL carrying a sentinel in its
> userinfo and force a failure. **Liveness:** at `0df5c14` the sentinel appears — in
> `Provider.Endpoint` for the model carrier, in the wrapped `*url.Error` text for the graph carrier.
> If it does not appear, that carrier's finding is wrong and its half of #13565 is unnecessary.
> **Sensitivity:** after #13565 it appears in neither the field, the error text, nor the rendered
> substance, **while host, port and path remain legible** — an implementation that blanks the whole
> URL fails this arm, because two failing endpoints must stay distinguishable (#9939 R3).

**(c) The real distinction between the log and the record is not secrecy.** stderr is the operator's
own stream, on the operator's own host, unbounded and ephemeral. The record is **durable**,
**shared**, **embedded**, and — through `RenderToolResult` — **read back into a later model call**.
Those properties justify a **bound**, and a bound is a length, not a constant. Replacing a sentence
with `file write failed` is not a bounding step; it is a deletion.

### 4.4 What follows

- The **log** carries the cause **whole**, at every site including the model site, which today logs
  nothing.
- The **response**, the **record** and the **model** carry the cause **bounded** (§7.3).
- **The precondition, stated as a precondition and not as an invariant of this mechanism:** carrying
  a cause is safe with respect to URL userinfo **exactly when #13565 has landed**, because this
  design's own mechanism does nothing about it. That is why **§15 Step 0** gates every step that
  promotes a config-derived string into a destination it does not reach today, as a hard dependency
  rather than a caveat. A reader checking whether this document delivers what it claims should find
  that it claims a dependency here, not a guarantee.
- **And the gate must be as wide as the claim.** A precondition relocates an unmet claim rather than
  discharging it precisely when its gate is narrower than its claim's quantifier — which is what
  revision 2 shipped, with §10 item 4 quantified over *every* destination and the gate stopping short
  of the one that reaches the HTTP response. So, as the companion to this document's own rule that
  *a document may not assert an invariant its mechanism does not deliver*: **a document may not state
  a precondition its gate does not enforce.** §15 Step 0 is where that is checked.
- **Free text, not structured fields.** §4.3 removes the only argument that ever pointed at
  structure. What remains — the KISS answer the brief predicts — is *carry the cause, bounded, and
  name the bound*, and the three recorded instances are already separated by it: instance 1 by
  `…does not match the expected peg-native format`, instance 2 by the endpoint's own loading message,
  instance 3 by `Client.Timeout exceeded` **plus** the elapsed-and-bound preamble that is the only
  thing telling the reader whose five minutes ran out.

---

## 5. Assumptions and Constraints

| # | Assumption / constraint | Confidence |
|---|---|---|
| A1 | The container's stderr is reachable by the operator (`docker logs`). | High — #13534 §8 reads it. |
| A2 | Nothing in the product tokenizes. A token count for a request is therefore not producible without a fabricated ratio (P-51). Bytes are. | Certain — no tokenizer in the tree. |
| A3 | `readUpstreamMessage`'s 4,096-byte cap bounds how much foreign text can enter an error at all. | Certain — read in both adapters. |
| A4 | DiVoid's embedding window is 8,000 characters over `name + "\n\n" + content`. | Measured in #13481, whose *Guard* section says: **re-run the two-probe bracket if DiVoid changes** — nothing announces such a change. |
| A5 | The operator may set **either** `PROCESSOR_MODEL_URL` **or** `PROCESSOR_DIVOID_URL` with userinfo. Neither is observed and neither is parsed for it (`loadModel` never parses; `rejectAPIBase` reads only `parsed.Path`). | Permitted-input reasoning, not an observation, and stated as such. Revision 1 named only the first — the count was the round-1 critical fail (§4.3(b)). Both are #13565's to close; this design depends on that, and asserts nothing about it itself. |
| C1 | Read-only git for this unit. The document is written, not committed. | Instruction. |
| C2 | One feature, one PR. The endpoint redaction (§7.2) ships **inside** this unit because this unit is what makes `endpoint` load-bearing; nothing else is folded in. | #1136, PR-scope discipline. |

---

## 6. Architectural Overview

```
                       the cause is authored here
                                  |
   +------------------------------+------------------------------+
   |                              |                              |
[adapter]                    [workspace]                      [graph]
 upstream text,               OS errno text                   read failure
 transport text
   |  wraps ONCE at its outermost point, attaching
   |  model / endpoint / request bytes / elapsed vs bound
   v
[loop.judge] --- err ---> [loop.Run] --- err ---> [server]
   |                          |                      |
   | log WHOLE                | log WHOLE            | response: code + message
   |                          |   ("run failed")     |   + bounded cause
   |                          |                      v
   |                          |                 { "error": { "code": "...",
   |                          |                   "message": "<class>: <cause>" } }
   |                          |
[loop.dispatch*] -- bounded cause --> ToolExchange.Error
                                          |            |
                                          v            v
                                    record.toolCalls   RenderToolResult
                                    (durable, embedded) -> the model, next call
                                          |
                                          v
                                    RenderSummary -> substance (bounded again, tighter)
```

**One rule governs every arrow:** *the log gets the cause whole; every destination that is durable,
shared, or prompt-bearing gets it bounded.* No arrow substitutes a constant for a cause it was
handed.

---

## 7. Components and Responsibilities

### 7.1 The model adapters — the only layer that knows three of the four facts

**Owns:** composing, on failure, a message that names **model id**, **endpoint**, **assembled request
size in bytes**, and **elapsed against the bound that was hit**; and appending the cause it was
handed.

**Does not own:** classification (that stays `ErrModelUnavailable` in the loop), status codes,
truncation, logging, or **redaction** — the endpoint arrives already redacted from #13565, which is
why Step 2 is gated on it.

**Three of the four facts are adapter-only, and three is what forces the relocation.** Endpoint,
request bytes and elapsed-against-the-adapter's-own-client-bound exist nowhere else at the moment of
failure: `Provider` travels on `JudgeResult`, which is zero-valued when `Judge` returns an error
(`turn.go:250` assigns `judged.provider` **after** the error return at `:243`), and `routes.go:19`
hands the server nothing but the `*loop.Turn` — no config, no adapter, no base URL. **The model id is
the exception** and revision 1's *"and nowhere else"* was wrong about it: `Turn.ModelID`
(`turn.go:87`) holds it and survives a failed call, which is how `record.Model` is fed at
`turn.go:149`. It is composed in the adapter anyway because the adapter already has it and splitting
one message across two layers to save nothing would be the worse shape. This is the finding that
relocates the fix. #11312's
*"narrow fix — one line at `routes.go:83-91`"* is right about **logging** and structurally incapable
of carrying the four facts, which do not exist at that layer. Both changes are wanted; they are not
the same change, and this document says which is which.

**Shape: one wrap, at the outermost point of the judgement call.** All four facts are established
before any I/O — the endpoint is composed, the body is marshalled, the clock is read — so a single
wrapping point attaches them regardless of which internal step failed (encode, build, transport,
status, decode). The bound named in the message is the one the adapter is **actually** operating
under — its HTTP client's own timeout — not the package default, so an injected client cannot make
the message lie.

**Why this is not extracted into a shared helper.** #13345's line is *"does it render the loop's own
data, or declare something to an endpoint?"* — and the failure preamble does neither: it reports the
**adapter's own transport facts**, which only the adapter has. DRY math per #1267: the wrap is
~4 lines × 2 sites = **8**, below the ~15–20 threshold. It stays duplicated, as a decision,
consistent with the three functions #13345 already rules stay.

### 7.2 The redaction — *removed from this design in revision 2; it is #13565's*

Revision 1 specified the `Provider.Endpoint` redaction here. It is gone, for two reasons that arrived
together: the sweep that justified folding it in was **incomplete** (§4.3(b): two carriers, not one),
and the redaction was **split out as #13565** on the #13481 precedent, because it acts on the success
path and changes every successful record's serialization. Widened to both carriers, it is a real unit
with its own falsifier and its own README note.

**What remains here is one sentence:** the endpoint this design's preamble reports must already be
redacted when it reports it, which is a **dependency**, discharged by #13565 merging first (§15).

The section is kept as a stub rather than deleted so that a reader of the review record can see the
decision was reversed and why. `parsed.Redacted()` at `boot/config.go:146` remains the technique
#13565 should reuse — and it is worth being precise, because revision 1 was loose about it: that call
is a **precedent for the technique inside one error message for a different variable**, and it is the
only occurrence in the tree. It is not an existing redaction policy that #13565 merely extends.

### 7.3 The bounding step

**Owns:** one named length, applied once per crossing, converting a cause into the text a bounded
destination receives.

**Does not own:** the log, which never passes through it.

**The bound: 512 runes.** The math, from measured causes:

| measured cause | runes |
|---|---|
| `The model produced output that does not match the expected peg-native format` (#11312, instance 1) | 76 |
| the transport text of instance 3, `Post "…/api/chat": context deadline exceeded (Client.Timeout exceeded while awaiting headers)` | **112** |
| the #13542 write failure, `workspace: create run directory: mkdir …: permission denied` with its real run-directory name | 88 |

**The merge test, run — revision 1 claimed it on every element and skipped this pair.** Largest
measured cause is **112 runes**, so **a single bound of 200 would carry every measured cause whole in
every destination.** 512 is therefore *not* justified by any measurement, and revision 1's *"4.6× the
measured worst case"* was a ratio with no consumer attached — the exact shape §11 rejects everywhere
else in this document. The number stays; its reason changes.

**512 is headroom, and the consumer is named.** `readUpstreamMessage` caps a foreign error body at
**4,096 bytes** (`ollama/client.go:126`, and its twin in `openaicompat`), so upstream text genuinely
can run far past 200 runes — and in instance 1's class the upstream's sentence *is* the whole
information. Two destinations can afford the headroom and both act on it: the **HTTP response**, which
is what the operator reads, and **the model**, which `RenderToolResult` makes a live consumer. The
substance cannot afford it, which is the whole of why there are two numbers (§13 Q7). So: **200 is the
bound every measured cause fits inside; 512 is the bound an unmeasured upstream body is allowed to
reach in the two places that can carry it.**

The upper cost is bounded by UTF-8 at **2,048 bytes**, which is the figure to use because it is
exact; the token cost of those bytes is not measured here and no rune-to-token ratio is asserted
(P-51). Against record #13472's measured 15,304 input tokens the addition is small; *how* small is not
a number this project can produce today.

**Why bound it at all, when the record has room for 70 kB?** Because a tool-round error is re-sent to
the model on **every** subsequent call in the turn, up to `MaxModelCalls = 6`. `AssemblyByteBudget`
does not cover it — that budget bounds the block only, and a tool result is a message outside it. An
unbounded 4,096-byte upstream body would therefore inflate exactly the variable that produced
instance 3: **an unbounded error message about a prompt-size timeout makes the next prompt bigger.**
That is the concrete failure of the simpler "carry it verbatim" form, and it is why *bounded* rather
than *verbatim* is this design's answer.

### 7.4 The run-record summary's error line

**Owns:** one constant. Today `summary.go:149` renders `call.Error` through `summaryQueryRunes = 88`.

The one measured file-tool cause is **exactly 88 runes** (§7.3's table) — it renders whole with
**zero** margin. Two things make that worse than revision 1 said, both in the argument's favour:

1. **The length is not stable even for a fixed root.** `os.MkdirTemp(root, "run-")`
   (`workspace.go:60`) appends a random number; #13542's two observed directories both happen to carry
   ten digits and nine is equally reachable. The line already sits *at* the bound with a
   nondeterministic tail.
2. **The concrete X does not have to be hypothetical.** Revision 1 argued from *"a root one character
   longer"*. It does not need to: **#13542's own remedy list proposes "a bind mount to a host
   directory the operator already owns"**, which on any real host path is longer than the measured
   root and truncates `permission denied` off the end the moment it lands.

**The bound, derived rather than picked.** The cause has a fixed skeleton, and only the root and the
random suffix vary:

```
  33   "workspace: create run directory: "
+  6   "mkdir "
+ R    the workspace root
+  5   "/run-"
+ D    the random suffix's digits
+ 19   ": permission denied"
= 63 + R + D
```

Checked against the measurement: `R = 15` (the documented `/data/…` root) and `D = 10` gives
**63 + 15 + 10 = 88**, which is the observed line exactly. Solving the other way at `D = 10`, **a bound
of 200 admits any workspace root up to 127 characters** — against the 15 measured and the ~30–40 a
host bind-mount path implies. That is the derivation; revision 1's *"2.3× the measured worst case"*
was a ratio computed after choosing a round number and presented as if it had produced it (P-51).

**Cost:** worst case **+112 bytes** on a substance measured at 2.0–3.8 % of the record (#13245) — on
record #13472 that band is 1,414–2,687 characters, so the addition is at most 7.9 % of the low end.
Not 512, because a 512-rune line would be **36 % of that low end** (§13 Q7). **This is the single most
cheaply reversed element in the document** and is flagged as such in §13.

### 7.5 The loop

**Owns:** classification (`ErrModelUnavailable`, `ErrGraphUnavailable`, `ErrSubjectNotFound`,
`ErrWriteRejected` — unchanged), logging the cause whole at rows B–E, and applying §7.3's bound to
**every carried cause, with no exemptions.**

**"No exemptions" is a correction, and it resolves a disagreement revision 1 shipped.** Revision 1's
§7.5 said *"each tool-error assignment"* while its §15 Step 3 said *"rows B, C, D and at the HTTP
boundary"* — two different sets, because there are three further assignments neither list named: the
`result.ToolError` pass-throughs at `turn.go:285`, `:314` and `:350`. Those are **already** carried
causes, not discards, so they need no un-discarding; and they are demonstrably short — three fixed
strings plus one `fmt.Sprintf` over a `json.Unmarshal` error, which does not echo the payload
(`ollama/wire.go`, `openaicompat/wire.go`). **The bound is applied there too**, where it is a no-op on
every value they can currently hold. Stating it costs one clause and buys a rule with no edge: *every
cause the loop hands onward is bounded*, so no future assignment can be exempt by having been
overlooked.

**Gains one log line it does not have:** a `run failed` record, symmetric with `logFinished`, carrying
subject, elapsed and the cause. Today a failed run emits `run started` and nothing — #11312's third
instance names this precisely.

**Keeps three constants, and this is a decision, not an oversight:**

| constant | why it stays |
|---|---|
| `errCallCapReached` | The loop **is** the author. `MaxModelCalls` was reached; there is no other party's sentence to carry. |
| `errNoWorkingDirectory` | The loop is the author, and `README.md:205` documents this exact string as the operator-facing statement. |
| the eight `ErrWriteRejected` reasons | Already carried (row A), already the loop's own closed set, already correct. |

> **The discriminating line, stated so it does not have to be re-derived:** *a constant is the true
> and complete statement when the loop authored the failure; a constant is a **discard** when
> somebody else did.* Rows B, C, D and E are discards. The three above are not.

**`errSupplementaryRecallFailed` (row D) is included, and it is unwitnessed.** No instance of it is on
record. It is included because it is the *same discard, in the same function family, in the same
file*, and shipping a stated rule with one site exempted leaves the next reader to invent a reason for
the exemption. This is consistency of a rule, not a speculative feature; it is named here as
unwitnessed so that nobody later cites it as evidence of a fourth instance.

### 7.6 The HTTP layer

**Owns:** the status and code mapping — unchanged: `routes.go:82-92` keeps all five codes, both 502
branches, and the deadline check that runs first — and writing the bounded cause into the envelope's
`message`.

**Does not own:** the four facts, the bound's value, or logging (§7.5 already logged it whole; a
second log line at the boundary would double every failure in the operator's stream).

**The envelope does not grow a key.** `code` stays the stable machine-readable classifier and keeps
its documented meaning; `message` has always been human prose and becomes prose that carries the
cause: `"<the existing class sentence>: <bounded cause>"`. Keeping the class sentence preserves what a
reader of the README already knows, and anything keying on `code` is unaffected.

**The `default:` arm at `routes.go:89-90` is the sharpest single instance in the whole defect.** Today
anything unrecognised is reported as *"the graph could not be read"* — a **specific and possibly
false** claim about a component that may not be involved. Under this design that arm reports its own
cause, so a future sentinel that reaches it stops being misattributed to the graph.

---

## 8. Interactions and Data Flow — the four recorded instances, end to end

| instance | what the operator will read in the 502 | what stderr adds | remedy readable? |
|---|---|---|---|
| 1 — deterministic rejection | `the model call did not complete: openaicompat: model=ai/llama3.2 endpoint=… request=76490 B elapsed=3.7s (client bound 5m0s): unexpected status 500: The model produced output that does not match the expected peg-native format` | the same, unbounded | **yes** — change the request |
| 2 — cold load | the same preamble, and the endpoint's own loading sentence | the same | **yes** — retry |
| 3 — CPU-bound timeout | `…request=61234 B elapsed=302.5s (client bound 5m0s): request failed: … context deadline exceeded (Client.Timeout exceeded while awaiting headers)` | the same | **yes** — `elapsed ≈ bound` says *our* ceiling cut it, and `request=` names the variable that separates it from the 2.4 s small-prompt call |
| 4 — #13542's write | `200`, and `toolCalls[0].error` carrying the workspace's own `permission denied` sentence | the same, unbounded | **yes** — chown the volume |

Instance 3 is the one that justifies the preamble rather than the cause alone: `context deadline
exceeded` is true of both live ceilings, and only `elapsed` beside the named `bound` distinguishes
them. It is also honest about its limit — see §12 F-2.

**A fifth flow, and it is not the operator's:** instance 4's cause now also reaches the **model**
through `RenderToolResult`. `permission denied` on a directory the model never named tells it that
retrying the same write is pointless, where `file write failed` invites five more attempts against
the call cap. No prompt change is needed to obtain this; it falls out of the same field.

---

## 9. Data Model (Conceptual)

**Unchanged. No new entity, no new field, no reordering.**

- `ToolCallRecord.Error` already exists and already serializes; only its **content** changes.
- `Provider.Endpoint` already exists; only its **redaction** changes.
- `Record` gains nothing, which is deliberate: #13481 measures that anything appended today lands past
  the 8,000-character embedding cap, so a new field would be a persisted value no search can reach.
- **No failure record is created.** §13 Q1 states the alternative and its reversal cost.

---

## 10. Contracts and Interfaces (Abstract)

| contract | before | after |
|---|---|---|
| **Adapter → loop, on failure** | an error whose text names the adapter and the cause | the same, prefixed with model id, redacted endpoint, request bytes, elapsed, and the bound in force |
| **Adapter → loop, on success** | `Provider{Adapter, Endpoint}` | the same, with `Endpoint` redacted |
| **Loop → operator log** | rows B–D: whole cause. Row E: nothing | every row: whole cause, plus a `run failed` line symmetric with `run finished` |
| **Loop → `ToolExchange.Error`** | rows B–D: a constant | the cause, bounded to 512 runes |
| **Loop → HTTP** | five sentinels | unchanged — five sentinels, five codes, five statuses |
| **HTTP → caller** | `{code, message}` with a per-code constant message | `{code, message}` where `message` is the class sentence plus the bounded cause. **The key set does not change.** |
| **Record → model** | `"error: " + Error` | unchanged mechanism, informative content |
| **Record → substance** | error truncated at 88 runes | error truncated at 200 runes |

**Invariants, and one precondition.**

1. No destination receives a constant in place of a cause it was handed.
2. The log is the only unbounded destination.
3. Neither key value can appear in any of them — §4.3(a), with its falsifier. **This one this design's
   mechanism does deliver**, because it is a property of how the keys travel, not of anything this
   design adds.
4. **Precondition, not invariant:** no URL userinfo appears in any of them **provided #13565 has
   merged**. Revision 1 asserted this as an invariant of this mechanism and it was not one — the
   mechanism performs no redaction, and §4.3(b)'s second carrier reaches the record, the substance,
   the model and the response through row D and the `default:` arm. **A document may not assert an
   invariant its mechanism does not deliver**, so this is stated as the dependency it is, and §15
   gates on it.
5. `code` remains a closed set of five and keeps its documented meanings.

---

## 11. Quality Attributes and Trade-offs

**Diagnosability** is the attribute being bought, and §8 is its acceptance table.

**Rejected alternatives, each with the reason it lost:**

| # | alternative | why rejected |
|---|---|---|
| R-1 | **Store a failure record** for a failed model call | Two grounds, and revision 1 offered a third that does not discriminate. **(i)** No consumer nameable in four weeks (#1136 §5, #868). **(ii)** The three instances were each read within minutes by the person in front of the container and then written up by hand into #11312 — which is a *better* artifact than a machine record, because it carries the diagnosis rather than the symptom. **Struck:** *"it would consume one of `CandidateLimit = 20` recall slots"* — true of **every** record the system already writes, successes included; if it were a good argument it would argue for storing no records at all. What survives of it is narrower: a failure record inherits the same slot cost as a successful one **without the substance a human searches for**. See §13 Q1 — the decision is phase-bound. |
| R-2 | **A structured failure object** (kind, elapsed, bound, size, endpoint, upstream) on the response and/or record | #11312 rules the class question closed; the consumers are two humans reading a 502 and a log; and on the record the fields land past the embedding cap (#13481). This is the strongest alternative and it loses on all three. |
| R-3 | **Carry the cause verbatim, unbounded** | Fails on the prompt-bearing destination: up to five re-sends of a 4,096-byte body inflate the exact variable that produced instance 3 (§7.3). |
| R-4 | **Fix only the log** (#11312's own narrow fix) | Correct and insufficient. The operator this is for reads the **response**; #13534 §8's whole finding is that the loop does not close when diagnosis needs a second window. It is also structurally unable to carry the four facts, which do not exist at `routes.go`. |
| R-5 | **A `detail` key in the error envelope** | A new wire key for text that `message` — prose by definition — already accommodates. Adding it later is additive and cheap if a machine consumer ever appears (§13 Q3). |
| R-6 | **Redact the whole endpoint** rather than its userinfo | Destroys the field's purpose: two failing endpoints must stay distinguishable. Same requirement as #9939 R3. |
| R-7 | **A shared adapter helper for the failure preamble** | `4 lines × 2 sites = 8`, below #1267's ~15–20 threshold, and it reports transport facts only the adapter holds — the wrong side of #13345's line. |

**Trade-off accepted, named concretely.** A cause carried into the record is a cause that is durable
and shared. If a future upstream returns something genuinely sensitive that is *not* one of the two
keys — a customer identifier inside a gateway's error body, say — it will be stored. Probability: not
observed in three instances across two protocols and two adapters, against local endpoints. Cost if
it materialises: one record body to redact, and a set of names to add to a policy that does not exist
yet. Cost of the alternative that prevents it: the defect this document exists to fix. The trade is
made knowingly, and the shape of the eventual remedy is already designed one repo over — #9939's
name-set policy — if it is ever needed.

---

## 12. Risks and Mitigations

| # | risk | mitigation |
|---|---|---|
| F-1 | The 512-rune bound truncates a cause whose informative half is at the end | Upstream error text puts the message first in all three measured instances, and the log carries it whole regardless. The residual risk is confined to a foreign body between 512 runes and `readUpstreamMessage`'s 4,096-byte cap — the one carrier this project has never measured, which is why 512 rather than 200 is the bound in the two destinations that can afford it (§7.3). |
| F-2 | `elapsed ≈ bound` is *evidence* of a ceiling kill, not proof — a coincidental failure at 299 s reads the same | Named here rather than engineered around. The message reports two measured numbers and draws no conclusion; the reader draws it. Fabricating a *"timed out"* verdict from an inequality would be the P-51 failure this project has already paid for once. |
| F-3 | A future adapter (a third protocol) forgets the preamble and silently regresses | #10466's *"Adding a model provider"* archetype gains a step, exactly as #13345 §11 added one for `RenderToolResult`. Cheap, and it is the mechanism that already worked. |
| F-4 | A1 is wrong for some deployment: the operator cannot read stderr | Then the response is the only channel — which is precisely why this design puts the cause there and does not stop at #11312's log-only fix. |
| F-5 | DiVoid changes its embedding window and §2.3's reasoning about `Record` shifts | This design adds no field to `Record`, so the measurement is a *reason not to act*, not a dependency. #13481's two-probe bracket remains the guard if the question is ever reopened. |
| F-6 | The redaction changes `Provider.Endpoint` for **existing** consumers on the **success** path | **This risk moved to #13565 with the work**, and it is exactly why the split was ruled: every successful run's record and substance change content. Grep-checkable and small: `summary.go:57-61`, `turn.go:178`, and `scripts/`. No stored record is rewritten — record fate is forward-only (#13238 §9.6.5), so old records keep the endpoint they were written with. |
| F-7 | **#13565 does not land, and this design ships anyway** | Then Step 3 carries the graph URL into the record, the substance and the model, **Step 4 carries it into the HTTP response**, and §10 item 4's precondition is unmet. §15 makes the gate hard rather than advisory for this reason. **This row asserted in revision 2 that "the two steps it gates" made a partial merge visible — a completeness claim that was false by one step**, which is the round-2 critical fail in miniature. It no longer states a count: §15 Step 0 enumerates the gated set, states the ungated remainder beside it, and checks that the two partition all seven. |

---

## 13. Open Questions — taken as decisions

Each is **decided**, with the alternative named and the cost of reversing it.

| # | question | decision | alternative | reversal cost |
|---|---|---|---|---|
| Q1 | Does a failed model call produce a stored record? | **No — for this phase.** The decision is **phase-bound, not permanent**: it holds while Toni runs single tasks by hand and writes each failure into #11312 himself, which is where the diagnosis actually lands. **When he runs at volume and stops hand-writing them, the consumer appears and this flips.** A decision resting on a four-week consumer gate should name what ends the four weeks. | R-1, a failure record. | **Low, additive.** `WriteRun` is already called with a fully-built `Record`; a failure record would be a second call site with a partially-populated one. Nothing here forecloses it. |
| Q2 | Structured fields or free text? | **Free text, bounded.** | R-2. | **Medium.** Adding fields later means field-order surgery on `Record` (#13481) for them to be reachable at all — which is why the decision is made now, and made against. |
| Q3 | Response `message`, or a new `detail` key? | **`message`.** | R-5. | **Low, additive** — a new key breaks no existing consumer, and `code` is what anything machine-readable keys on. |
| Q4 | Request size in tokens or bytes? | **Bytes.** The product does not tokenize (A2); a token figure would be a fabricated ratio (P-51). | Report an estimated token count. | **Low**, and it becomes free the day anything in the product tokenizes. |
| Q5 | Does the redaction belong in this unit? | **No — reversed in revision 2.** Revision 1 folded it in; QA #13574 found the sweep behind that call had missed a second carrier, and the redaction was split out as **#13565**, widened to both. The #13481 precedent decides it: redaction acts on the **success** path and changes every successful record's serialization, which is its own unit. | Fold it in, as revision 1 did. | **Now a dependency, not a caveat.** Reversing means re-absorbing #13565 into this PR and re-arguing the one-feature-one-PR split against #13481. Revision 1's conditional (*"then the preamble must not carry the endpoint"*) is retired: under the split it never binds, because #13565 is on `main` before Step 2 runs. |
| Q6 | Widen the summary's error line from 88 to 200 runes? | **Yes**, and **200 is now derived, not picked** (§7.4): the cause is `63 + R + D` runes, the measurement checks at `R=15, D=10 → 88`, and 200 admits any workspace root up to **127** characters. The concrete X is #13542's own bind-mount remedy, not a hypothetical. | Leave it; the record carries 512 one fetch away. | **Trivial — one constant.** Still the element most cheaply reversed, and revision 1 called it the one a reviewer should challenge first. A reviewer did, and it survived with a better derivation. |
| Q7 | Is the bound one number or per-destination? | **Two: 512 for response/record/model, 200 for the substance** — and the merge test revision 1 skipped is now run (§7.3). A single bound of **200 carries every measured cause whole**; 512 is headroom for an unmeasured upstream body, which `readUpstreamMessage`'s 4,096-byte cap makes real, in the two destinations that can afford it. The substance cannot: 512 runes is **36 %** of the low end of #13245's measured 1,414–2,687-character band. | One number everywhere. | **Trivial** — two constants. |
| Q8 | Include row D (supplementary recall), which is unwitnessed? | **Yes**, as rule-consistency, flagged unwitnessed (§7.5). | Exempt it until an instance appears. | **Trivial.** |

---

## 14. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

- [x] **No new type whose value-space mirrors an existing one.** No new type at all.
- [x] **No new abstraction with one implementation.** The only new named elements are two length
      constants and the step that applies one of them.
- [x] **Nothing justified by "we might need X later."** Q1–Q8 each name the concrete X or decide
      against. §7.4's justification is #13542's own proposed bind-mount remedy, a filed alternative
      rather than a projection, and its bound is derived from the cause's `63 + R + D` skeleton.
- [x] **No deprecation period, feature flag, shim, or transition window.** Records are forward-only by
      prior design (#13238 §9.6.5); nothing is rewritten and nothing is dual-written.
- [x] **DRY math quoted for every inline-versus-extract call.** §7.1: `4 × 2 = 8`, below #1267's
      ~15–20. §7.2's redaction left this design entirely in revision 2 and is #13565's. §7.3's
      bounding step is one named element with four call sites, not four inlined blocks. §7.4 reuses
      `summary.go`'s existing rune-excerpt machinery rather than adding a third truncator beside the
      two the tree already has.

**Existing systems first**

- [x] **Existing surfaces audited.** `ToolCallRecord.Error`, `Provider.Endpoint`, the `{code,message}`
      envelope, `RenderToolResult`, `RenderSummary` and the adapters' `readUpstreamMessage` cap all
      already exist and all are reused. Nothing new is introduced to hold this data.
- [x] **No new layer proposed**, so no justification is owed.
- [x] **No new persisted data point.** Q1 and Q2 both decide against, on the four-week-named-decision
      gate.
- [x] **Consumer chain recursed.** The tool error's chain is `ToolExchange.Error` → record body
      (embedded, but past the cap) **and** → `RenderToolResult` → **the model** **and** →
      `RenderSummary` → substance. The model is a live, named consumer that acts on the value, which
      is what keeps this from being transitive-dead-code. The endpoint's chain is `Provider.Endpoint`
      → record → substance → a human reading a search result.

**Configurability**

- [x] **No new knob.** 512 and 200 are `const`s, per #1136 §3 — no operator will tune them and they do
      not differ across environments.
- [x] **No telemetry-then-tune compound.**

**Less is better**

- [x] **can-it-be-deleted / merged / inlined** run on every element — **including the pair revision 1
      claimed this for and skipped.** The merge test on the two bounds is now run in §7.3 and it
      changed the argument: 200 alone carries every measured cause, so 512 is headroom and had to name
      its consumer instead of a ratio. The four facts survive because §8 shows instance 3 is
      undiagnosable without `elapsed` + `bound` + `request`. A structured failure object was deleted
      (R-2). A shared preamble helper was deleted (R-7). A `detail` key was deleted (R-5). A failure
      record was deleted (R-1). **The redaction was deleted from this document altogether** (§7.2).
- [x] **Trade-offs named explicitly** — §11's closing paragraph, and F-2's refusal to infer a verdict
      from an inequality.
- [x] **Radical-clean over compromise** where the surface has no consumer: no failure record, rather
      than a slimmer one.

**Data deliverables** — not applicable; no SQL, no migration, no backfill.

**Document discipline**

- [x] **#1136 and #11034 cited as load-bearing** (header).
- [x] **Reader and scope inventories explicit** — §2.1's five-site table, §10's contract table, §14's
      consumer-chain recursion.
- [x] **Out-of-scope listed explicitly**, not merely absent — §3, seven numbered items, plus the
      redaction that left scope in revision 2.
- [x] **Every exhaustive claim quantifies over a set that was checked** — **and this item is run over
      the text the current revision adds, not only over the phrases the last review named.** That
      application clause is the whole of the item; without it the item is a retrospective that passes
      forever while the class keeps shipping. **The evidence that it is needed is that it was not
      applied:** revision 1 shipped three instances (*"the one pre-existing credential exposure"* —
      there were two; §10 item 4, which the mechanism did not deliver; §15's *"each names its own
      witness"*, which two steps do not), revision 2 corrected all three **and shipped two more in its
      own new text** — Step 0's partition of the seven steps and F-7's *"the two steps it gates"* —
      both quantifications over an unchecked set, both in the section this item most directly governs.
      Running the item over revision 3's new text is what produced §15 Step 0's explicit
      gated-plus-ungated partition and F-7's removal of its count (**P-52**, and §17).
- [x] **No multi-paragraph rationale for things that obviously stay.** The three surviving constants
      get one table row each.
- [x] **No predecessor superseded.** This document consumes #11312, #13542 and #13534 §8; it
      supersedes nothing and no banner is owed. It **qualifies** one clause of #11312 — its narrow fix
      is necessary and not sufficient, §7.1 — and says so where the qualification is made.
- [x] **Every negative claim carries a differential falsifier in the same passage** (#12958 §18.6
      clause 2) — §4.3(a), §4.3(b), and F-2's explicit statement of what the elapsed/bound pair does
      **not** prove.

---

## 15. Implementation Guidance for the Next Agent

Ordered. Each step is independently reviewable, and **each of the five code steps names its own
witness**; Steps 6 and 7 are documentation and take none — revision 1 quantified over all seven and
two of them could not meet it. **No step writes a node id into source** (#10861); one tight godoc line
on exported identifiers, none on unexported, no body or trailing comments.

> ### Step 0 — the gate. **#13565 merges first, and it is a dependency, not a caveat.**
>
> **This is the only authority for the gated set.** Every other mention defers to it rather than
> restating it — the bounce rule below names members only after deferring here, and no step heading
> carries a membership marker, because a per-step label is a distributed enumeration that goes stale
> one step at a time. Revision 2 restated the set in four places with no authority among them (§17).
>
> **The membership rule, stated over the row rather than over a list:** *a step is gated exactly when
> it promotes a config-derived string into a destination it does not reach today.* Applied, that is
> **Steps 2, 3 and 4**, and the three fail differently:
>
> | step | what it promotes | into | #13565 half it needs |
> |---|---|---|---|
> | **2** | `PROCESSOR_MODEL_URL` via the endpoint in the failure preamble | the HTTP response | **model** |
> | **3** | `PROCESSOR_DIVOID_URL` via row D's `*url.Error` (`turn.go:355-356`) | the record, the substance, **the model** | **graph** |
> | **4** | `PROCESSOR_DIVOID_URL` via the `default:` arm's message (`routes.go:89-90`) | the HTTP response | **graph** |
>
> **Steps 3 and 4 are independent and neither implies the other.** `routes.go:82-92` has **no `case`
> for `ErrGraphUnavailable`**, so the graph errors wrapped at `turn.go:111` and `:121` land in the
> `default:` arm — a different function, in a different file, from row D. Shipping Step 3 without
> Step 4, or Step 4 without Step 3, each leaks by its own route.
>
> **The partition, stated so a step cannot be absent from both sets:** gated **2, 3, 4**; ungated
> **1, 5, 6, 7**; three plus four is seven, so every step is in exactly one. Step 1 is ungated
> deliberately: it newly logs the graph URL to **stderr**, which §4.3(c) rules out of scope as the
> operator's own unbounded stream on their own host, and row D already writes the same string there
> today.
>
> **A partial merge does not clear the gate.** #13565 covers two carriers; Step 2 needs the model
> half and Steps 3 and 4 need the graph half, so confirm **both halves are on `main`** rather than
> confirming that #13565 merged.

**Step 1 — the log gains what it never had.** `judge()`'s model-call failure and `Run`'s early
returns log the cause whole, and a `run failed` line lands symmetric with `logFinished` (subject,
elapsed, cause). *Witness:* a failing model double produces a stderr line naming the cause;
**sensitivity** — an implementation that logs only the sentinel class, not the wrapped detail, must
redden.

**Step 2 — the adapters attach the four facts.** One wrap at the
outermost point of each adapter's judgement call. The endpoint arrives already redacted; this step
performs no redaction of its own and must not be implemented as though it did. *Witness:* one fixture
per failure mode — encode, build, transport, non-2xx, decode — confirming the preamble is present on
all five, and §4.3(a)'s Arm 2 runs over **the same five**, so a mode cannot be covered for the
preamble and missed for the sentinel. That is #11034 **P-18**'s failure-path obligation, and it names
the mutant most likely to be missed: a wrap placed inside the happy path covers exactly one of the
five and looks correct.

**Step 3 — the bounding step, and the four un-discards.** One named
length, applied at rows B, C and D, at the HTTP boundary, and at the three `result.ToolError`
pass-throughs (`turn.go:285`, `:314`, `:350`) where it is a no-op — no exemptions, per §7.5. The three
constants of §7.5 are left alone. *Witness:* the discriminating
fixture is the **pair** — a rejected write (row A) must still render its own reason, and an
unrecognised write (row C) must now render the OS cause. A change that makes both carry the same
thing has erased the distinction rather than fixed it.

**Step 4 — the HTTP envelope.** `message` becomes class-sentence-plus-bounded-cause at both 502
branches and at `default:`. `code` and status are untouched. *Witness:* the `default:` arm no longer
claims the graph could not be read for a cause that has nothing to do with the graph — drive an
unrecognised sentinel through it and confirm the message names that sentinel.

**Step 5 — the summary's error width.** One constant, 88 → 200, with its own name rather than a
shared one. *Witness:* a cause of 89 runes renders whole; at 88 it did not.

**Step 6 — `README.md`.** The error table at **`:300-304`** (the envelope sentence is at `:296`)
documents the fixed messages and stops being true at Step 4. The `PROCESSOR_MODEL_KEY` and
`PROCESSOR_DIVOID_KEY` rows keep their *"never logged, never echoed in an error, never written to the
graph"* sentence, which §4.3(a) has now made checkable rather than aspirational. **The URL rows'
redaction note belongs to #13565's README change, not this one.** Documentation step; no test
witness.

**Step 7 — the archetype step.** #10466's *"Adding a model provider"* gains a step requiring a new
adapter to attach the failure preamble (F-3), in the same shape #13345 §11 used. Documentation step;
no test witness.

**What to bounce on rather than implement.** Revision 1 put a conditional here — *if the redaction is
ruled a separate feature, lift it out, and then the preamble must not carry the endpoint*. **That
conditional is retired: the redaction was ruled separate, it is #13565, and under the split the caveat
never binds**, because #13565 is on `main` before Step 2 runs. What replaces it is Step 0's gate.
**Bounce if the gate is not met** — if either half of #13565 is not on `main`, do not implement **any
step in Step 0's gated set** with its promoting surface omitted as a workaround: not Step 2 without
the endpoint, not Step 3 without row D, **not Step 4 without the `default:` arm's cause**. Say so and
stop. A half-built version of this design is one whose §10 item 4 is silently false, and Step 4 is
the operator-facing payoff — the 502 that finally says what happened — so it is the step most
tempting to ship alone and the one that reaches the destination round 1's critical fail was about.

---

## 16. Revision record

**Revision 2, 2026-09-10**, against QA **#13574** (REJECTED: one critical fail, six warnings).

| finding | what changed |
|---|---|
| **CF** — §4.3(b) swept one credential carrier and stopped; §10 invariant 4 asserted more than the mechanism delivered | Both remedies taken. §4.3(b) now enumerates **two** carriers with the route each takes; §7.2's redaction is **removed from this design** and is #13565's, widened to both; §3 drops *"the one pre-existing credential exposure"*; §10 invariant 4 is restated as a **precondition**; §15 gains **Step 0**, a hard gate on both halves. |
| **W-1** — §7.1's *"and nowhere else"* is false for the model id | Narrowed to the three facts that are adapter-only, with `Turn.ModelID` named. The relocation still follows from three. |
| **W-2** — §7.5 and §15 Step 3 named different sets of bound sites | Reconciled to *every carried cause, no exemptions*, naming the three `result.ToolError` pass-throughs where the bound is a no-op. |
| **W-3** — the TL;DR's embedding-cap claim was undated | Dated to `0df5c14`, with #13481's own item 2 named as what would weaken it. |
| **W-4** — the falsifier's claim arm covered three of five failure modes | Arm 2 now runs over the same five §15 Step 2 enumerates. |
| **W-5** — R-1's third ground did not discriminate | Struck and restated; Q1 is now explicitly **phase-bound**, with the trigger that flips it named. |
| **W-6** — §15's opening quantified a witness over all seven steps; two are documentation | Narrowed to the five code steps; Steps 6 and 7 say they take none. |
| **Q6 / Q7 rulings** | 200 is **derived** from the cause's `63 + R + D` skeleton (it admits a root up to 127 characters) rather than back-fitted as a ratio; 512 is justified by its **consumer** — an unmeasured upstream body under `readUpstreamMessage`'s 4,096-byte cap reaching the response and the model — after running the merge test revision 1 skipped. |
| **Citations** | `summary.go:57-63` corrected to `:57-61`; README error table `:296-303` corrected to `:300-304`; instance 3's transport text 111 corrected to **112** runes. |
| **QA's own finding, adopted** | §2.1 now cites `workspace.go:153-159`'s `classify`: its `errors.As(err, &errno)` test **is** the row A / row C split, so *"preserves the guessable and discards the informative"* is a mechanism, not a framing. |

**Unchanged, and QA verified each:** the composition point (the adapters), the bound's existence, free
text over structured fields, the four facts, the falsifier construction, Q1–Q4, Q8, and every other
line number in the document.

**Revision 3, 2026-09-10**, against QA **#13579** (REJECTED: one critical fail, no warnings).

| finding | what changed |
|---|---|
| **CF** — the gate closed Steps 2 and 3; **Step 4** reaches the HTTP response by its own route and was in neither the gated nor the ungated set | §15 **Step 0** rewritten around the membership *row* — *a step is gated exactly when it promotes a config-derived string into a destination it does not reach today* — giving **2, 3, 4**, with a per-step table naming which half of #13565 each needs and why Steps 3 and 4 are independent (`routes.go` has **no `case` for `ErrGraphUnavailable`**, so `turn.go:111`/`:121` land in the `default:` arm Step 4 rewrites). Step 0 is now the sole **authority**; the header, §4.4 and F-7 refer to it instead of restating, and the per-step **GATED** markers on Steps 2 and 3 were stripped rather than completed with a third — a per-step label is a distributed enumeration, and revision 3 shipped it two-of-three, which is the round-2 shape at label level (QA #13580 §4). The bounce rule covers the whole gated set. The header's mis-attribution of the response to Step 3 is corrected to Step 4. |
| **Completeness claim** — F-7 asserted *"the two steps it gates … so a partial merge cannot look complete"* | Corrected, and the row now states no count at all; Step 0 carries the gated set, the ungated remainder, and the check that the two partition all seven. |
| **Precondition coverage** — the form was ruled correct, property 3 (gate at least as wide as the claim's quantifier) failed | Fixed by widening the gate. §4.4 gains QA's companion rule as a standing test: **a document may not state a precondition its gate does not enforce.** §10's list heading now reads *"Invariants, and one precondition"* and references are *"§10 item 4"*. |
| **§14's class item** — reached the class in wording, was not applied to the revision's own new text | Gains the application clause, with revision 2's two new instances cited as the evidence that the clause is what makes it executable. |

**Not taken, and named so the omission is a decision rather than an oversight:** QA's §6 observes
that `os.MkdirTemp` draws its suffix from a `uint32`, so `D = 10` is the **maximum** and §7.4's 127 is
a **floor** rather than an estimate. It is correct and it strengthens a claim this document already
makes. It is left out because round 3's scope was the critical fail, the precondition's coverage and
§14's clause, and QA filed it as an insight rather than a finding. It is one clause whenever the
document is next opened.

---

## 17. The finding this arc produced three times

**This is the arc's deepest finding and it is not about failed runs.** The same mechanism produced
three separate defects across three rounds, and naming it once is cheaper than catching it a fourth
time:

| round | the remedy was scoped to… | …instead of the row | cost |
|---|---|---|---|
| 0 | the **file** the task text pointed at (`routes.go`) | *every layer that discards a cause it was handed* | the model site logs nothing at all, which the brief did not know |
| 1 | the **carrier** the sweep was looking at (`PROCESSOR_MODEL_URL`) | *every config-derived string that can reach a carried cause* | `PROCESSOR_DIVOID_URL` reaches four new destinations; critical fail |
| 2 | the **steps** the author was looking at (2 and 3) | *every step that promotes a config-derived string into a destination it does not reach today* | Step 4 in neither set; critical fail |

**The shape, stated so it is recognisable rather than merely regretted:** each time, a correct finding
about a *list* was mistaken for the finding about the *class*, and the remedy inherited the list's
boundary. **P-52** names this and its sharpening — *enumerate by row, never by phrase* — and the
three instances above are what it looks like when the row is not written down first.

**The operational form, which is what this section exists to leave behind:**

1. **Write the row before the list.** Say the property in one sentence — *every X that does Y* — then
   derive the members. A list assembled first will define the property afterwards, and it will define
   it as "whatever I found".
2. **State the membership rule beside the members**, so a reader can re-run it. §15 Step 0 does this;
   revision 2's four restatements did not, which is why none of them could be checked.
3. **Enumerate the complement too, and check the partition.** Every one of these three defects was an
   element in *neither* set. Two sets that must cover a known whole make an omission arithmetic
   rather than a matter of noticing.
4. **State the set once — and read "the set" as covering markers, not just prose.** Revision 2
   restated the gated set four times; correcting one would have left three, and there was no
   authority among them. §15 Step 0 is now the sole authority. **A per-item marker is a distributed
   enumeration**: revision 3 carried `GATED` labels on two of the three gated steps and did not look
   like a restatement until one noticed which step lacked one. They were removed rather than
   completed, because completing a label set leaves the next author to maintain it.

**Where this binds beyond this document:** it is why §7.5's bound has *"no exemptions"* rather than a
list of sites that are short today, and why §4.3(b) is a carrier table rather than a sentence about
the endpoint. Both were written to the row after the class was named; the gate was not, and that is
the one place revision 2 reverted to the old habit.
