# Architectural Document: A Date Is a Filter, Not a Query

> DiVoid node: **#13891** (`documentation`, root `#10422`). Repo path: `docs/architecture/a-date-is-a-filter-not-a-query.md`.
> The node carries this document in full, not an abstract of it.
> Task: **#13721**. Sibling that already shipped: PR #76 (`6c8a1df`). Baseline for every citation below: **`6c8a1df`**.

**Toni, 2026-09-12, verbatim:**

> *"Dates should not be part of a semantic query since it rarely or only randomly appears as verbatim text. Dates
> should be filtered by the createdat and updatedat fields which also speeds up the query by a lot because a certain
> date already reduces the list of nodes to check by a huge margin."*

---

## TL;DR

**What.** A run whose input expresses a time constraint retrieves inside that constraint, instead of putting the
date in the semantic query where it does nothing.

**How.** The derivation call that already turns the input into queries also names a **calendar day range** on one
extra output line — `DATES: 2026-09-12..2026-09-12`, or `DATES: none`. The loop resolves those days in the zone of
the **one instant the prompt already states** (PR #76's `NOW`) and sends them as `updatedFrom` / `updatedTo` on
every topical recall, the block's and the model's alike. No time in the input, no window.

**Cost.** One value type, one parameter on `GraphPort.Recall`, one line each in the derivation prompt, the `NOW`
span and the record. **It reverses PR #76 on one point**: the prompt states the host's civil time, not UTC.
Measured, not reasoned: that reddens **six guards across three packages**, two of them adapter test files — §4.3
carries the run and the disposition of each.

**Rejected.** Giving the *model* the window through the `recall` tool — **15 delta sites, 9 in the adapters**, a new
model-authored-date failure class, and it leaves the **assembled block** unbounded, so "today" still costs one of
the six model calls. A window in the derivation costs zero.

*Five rulings in all (§4), and #13720's "the recalls returned zero rows" premise is corrected in §0.*

---

## 0. Provenance, and one correction to the premise

Every figure below carries where it came from. Two instruments, and they do not establish the same things.

| Claim | Instrument | When |
|---|---|---|
| Wire parameter **spellings** `createdFrom` / `createdTo` / `updatedFrom` / `updatedTo`, and five inert names | raw REST, recorded in **#13721** | 2026-09-12 |
| Filter **semantics** and all **magnitudes** quoted here | `mcp__divoid__divoid_search` / `divoid_list`, re-measured | 2026-09-15 |
| #13718's recall behaviour | the run record's own JSON, re-derived | 2026-09-15 |
| Repo facts, line citations, greps | the working tree at `6c8a1df` | 2026-09-15 |

**What my instrument structurally cannot reach:** the MCP server sits between me and the HTTP surface, so nothing I
measured establishes the **wire spelling** of the parameters. That comes from #13721 alone. Milestone 1 re-verifies
it against an unfiltered control before anything else is built — see §16.

### The premise correction, and it strengthens the ruling rather than weakening it

**#13720 states that all six supplementary recalls "returned zero rows". That is false.** Re-derived from the run
record **#13718**'s own JSON:

| round | query | rows returned | rows admitted |
|---|---|---|---|
| 1 | `processor retrieval path changes today` | **20** | 5 |
| 2 | `processor retrieval path changes 2026-09-05` | **20** | 5 |
| 3 | `processor retrieval path changes 2026-09-06` | **20** | 5 |
| 4 | `processor retrieval path changes 2026-09-07` | **20** | 5 |
| 5 | `processor retrieval path changes 2026-09-08` | **20** | 5 |
| 6 | `processor retrieval path changes 2026-09-09` | — | never dispatched, cap reached |

The failure was not emptiness. It was **invariance**:

- The **admitted set was identical in all five rounds** — `{10422, 10423, 10434, 10451, 10455}`.
- `#10422` is the run's **own anchor**, already in the block. `#10423` is the *Docs* group node, `#10434` the
  *Tasks* group node. The remaining two are repo documentation with no relation to any day: `#10451` (a closed task
  about machine-local paths in the README) and `#10455` (the repo map's `cmd/processor/` entry).
- Pairwise Jaccard across the five returned lists: **0.600 – 0.905**; 14 of 20 ids common to all five.
- **98,310 bytes** of admitted material across the five rounds, carrying five copies of the same five nodes.
- Input tokens per call: **17,451 → 22,683 → 27,925 → 33,167 → 38,409 → 43,651**. The prompt grew 2.5× carrying
  restatements of itself.
- Outcome: `capReached: true`, empty answer, no file, HTTP 200.

**Five different date strings; one result set.** That is a cleaner demonstration of Toni's ruling than "zero rows"
was: the date in the query text did not narrow retrieval, and it did not fail loudly either — it was simply
**inert**, and the model had no way to see that.

Confirmed independently against today's graph, same query, `count=1`:

| query | `total` | top similarity |
|---|---|---|
| `processor retrieval path changes` | **11,414** | 0.681499 |
| `processor retrieval path changes 2026-09-15` | **11,414** | 0.681543 |

Identical candidate population. The date moved the fourth decimal of a similarity score and nothing else.

---

## 1. Problem Statement

A task whose input expresses a period of time — *"today"*, *"this week"*, *"since Monday"* — cannot be answered.
The loop has no way to express a time constraint to the graph, so the model's only available move is to write the
date into the semantic query, where it matches only nodes that happen to contain that date as verbatim text.

**Success criterion:** a run whose input expresses a time constraint issues retrieval bounded by that constraint,
and the run record states which bound was applied. A run whose input expresses none behaves exactly as it does
today.

### Why PR #76 does not close this on its own, and why this does not close without it

PR #76 put the current instant into the prompt (`===== NOW =====`) and into `Record.Now`. Under Toni's ruling the
model would have failed **with the correct date too**, because the failure was never a wrong date — it was that a
date in a semantic query matches nothing. Conversely a filter with no stated date is a parameter nobody can fill.
The two are one mechanism delivered in two PRs.

---

## 2. The measured state at `6c8a1df`

### 2.1 The loop cannot express a window, and the whole of that is four lines

`internal/divoid/client.go:163-170` — `Recall` builds its query string from exactly four parameters:

- `:165` `query`, `:166` `count`, `:167` `fields`, `:169` one `linkedto` per scope id.

Published and run, whole-repo, production and test alike:

```
git grep -c -iE 'updatedFrom|updatedTo|createdFrom|createdTo' 6c8a1df -- '*.go'
```

Exit status **1**, no output: **no date parameter name appears anywhere in the Go tree.** Not a partial
implementation, not a disabled one — absent.

### 2.2 The graph already supports it, re-measured on today's corpus

Filter cardinality is query-independent — the same `total` comes back with a semantic query and, via
`divoid_list`, with none — so these are properties of the corpus, not of the probe.

| filter | `total` | share of corpus |
|---|---|---|
| *(control, no filter)* | **11,414** | 100 % |
| `createdFrom` = 2026-09-15 | **11** | 0.10 % |
| `createdTo` = 2026-09-15 | **11,403** | 99.90 % |
| `createdFrom` + `createdTo`, the single day 2026-09-15 | **11** | 0.10 % |
| `createdFrom` + `createdTo`, the single day 2026-09-12 | **51** | 0.45 % |
| `updatedFrom` + `updatedTo`, the single day 2026-09-12 | **174** | 1.52 % |
| `updatedFrom` = 2026-08-16 (trailing 30 days) | **4,582** | 40.1 % |

**11 + 11,403 = 11,414 exactly**, so `createdFrom` and `createdTo` partition the corpus with no overlap and no gap.
A single-day window prunes **99.55 %** (created) or **98.48 %** (updated) of the corpus before any similarity work.
Toni's speed argument holds on today's corpus as it did on 2026-09-12's, at a different magnitude — the corpus grew
from 11,260 to 11,414 in three days, which is why these are re-measured rather than inherited.

**Offsets are honoured, and this is load-bearing for §4.3:**

| `createdFrom` | `total` |
|---|---|
| `2026-09-15T00:00:00Z` | **11** |
| `2026-09-15T00:00:00+02:00` | **13** |

The second window opens two hours earlier in absolute time and admits two more nodes. The graph parses the offset
and filters on the instant it denotes — it does not truncate to a date.

### 2.3 A bounded recall answers the question the run could not

The same query the run used, bounded to the run's own day:

```
query = "processor retrieval path changes", createdFrom = 2026-09-12, createdTo = 2026-09-13
total = 51
rank 1  #13719  session-log  processor-run 2026-09-12T13:20:39Z …
rank 2  #13718  session-log  processor-run 2026-09-12T13:13:43Z …
rank 3  #13726  task         divoid-mcp: the server is absent from john-backend-dev's tool surface
rank 4  #13723  task         divoid-mcp: server absent in a john-backend-dev subagent surface
rank 5  #13706  session-log  Morpheus Nightly Pass 2026-09-12 …
rank 6  #13729  task         P-12 should carry its derivation, not its number
```

Every one of the six is material from that day. The unbounded form returned the *Docs* group node five times.

### 2.4 The derivation step, and the rule that looked like an obstacle

`internal/loop/derive.go:76-85` renders one prompt from instructions (`:23-38`), two format-only exemplars
(`:45-66`) and the input. `:88-108` parses the completion one query per line. The call is bounded separately
(`DerivationBound`, `:17`) and **does not come out of `MaxModelCalls`** — established in the derivation design
(`docs/architecture/the-query-the-graph-is-asked.md` §4.4). It is therefore a free place to put work.

`derive.go:32` reads, verbatim:

> `- Do not invent specifics (names, numbers, node ids) that are not implied by the request itself.`

PR #76 recorded this as being in tension with supplying the date. **It is not, and the tension dissolves on
reading:** the rule forbids *inventing*, and an instant stated in the prompt is supplied, not invented. What the
rule should forbid — and does not yet say — is a date appearing **in a query line**, which is precisely #13718's
failure. §7.1 sharpens the rule in Toni's direction rather than weakening it.

### 2.5 What is not in the tree at this baseline

No date parameter (§2.1). No window on any port. No time vocabulary in either adapter's tool schema. No test named
for any temporal property of retrieval. The suite is green at `6c8a1df` (`go build ./... && go test ./...`, all 13
packages `ok`).

---

## 3. Scope and Non-Scope

### In scope

1. A time window derived from the input, once per run.
2. That window carried to the graph on topical recalls — the block's and the model's supplementary ones.
3. The window stated in the record and in the prompt.
4. The frame the prompt's instant is stated in, because the window's correctness depends on it (§4.3).

### Explicitly out of scope

| Item | Where it lives instead |
|---|---|
| `RenderSummary`'s header instant being a second clock read | **#13884**. §4.3 rules the *frame*; it does not fix the *second read*. See §17. |
| Node #8 documenting `query`, `similarity`, `substance`, the four date filters | **Nowhere, as its own item.** #13604 is a task about a missing *negation predicate*; the documentation gap is a two-sentence "worth checking before designing" note inside it, and it covers only the first three. The date filters are recorded nowhere. See §17. |
| The `recall` tool's schema, and both adapters' tool translation | Untouched by design — §4.1 |
| Supplementary recall re-serving the anchor and repeating rows across rounds | **#13892** — found here, filed here, not fixed here. §17. |
| The call cap not backing off on unproductive recalls | #13720's own item 2, not filed against this design |
| Derivation specificity (derived queries broader than the input) | #13720's item 3 |
| Any configurable timezone, retention, or default-window setting | §4.2, §4.3 — refused, not deferred |

---

## 4. The rulings

Five decisions. The brief named three as open; the shape below **deletes one of them** and answers a fourth the
brief did not name.

### 4.1 D1 — the loop determines the window, through the derivation call it already makes

**Ruling.** The derivation call emits the window alongside the queries. The judgement model gets no window knob; the
`recall` tool's schema is unchanged and **no adapter production file changes**. (D3 does touch one *test* file in
each adapter, for a reason unrelated to the tool — §4.3.)

**The decisive argument is the call budget, and it is measured.** `MaxModelCalls` is 6 (`turn.go:17`), and #13718
spent all six. A window produced by the derivation call bounds **the assembled block itself** — the material the
model sees before its first judgement call — and the derivation call is outside the cap. A window supplied through
the tool can only bound material the model asks for *after* seeing an unbounded block, which costs at least one of
the six calls for every time-bounded task, forever.

**And the cost is not close — enumerated rather than asserted.** A tool-supplied window touches, in the adapters:

```
git grep -c -E '^(type recallToolArguments|func (recallTool|toolArguments|translateRecall|recoverRecall))' 6c8a1df -- internal/ollama/wire.go internal/openaicompat/wire.go
git grep -c 'recoveredCall' 6c8a1df -- internal/openaicompat
```

The first is anchored on **declarations**, so its per-file output *is* the count rather than something asserted
about the count. Its output, which is a pair of `grep -c` counts and not a pair of line citations:

```
6c8a1df:internal/ollama/wire.go:5
6c8a1df:internal/openaicompat/wire.go:4
```

The anchor is load-bearing rather than decorative: **drop it and the same pattern counts use sites too.** That
claim is a command, published because a number asserted about an unpublished command is the defect this very
section exists to fix —

```
git grep -c -E 'recallToolArguments|func recallTool|func toolArguments|func translateRecall|func recoverRecall' 6c8a1df -- internal/ollama/wire.go internal/openaicompat/wire.go
```

```
6c8a1df:internal/ollama/wire.go:7
6c8a1df:internal/openaicompat/wire.go:6
```

7 and 6, not 5 and 4, because `recallToolArguments` is matched again at each of its use sites. The declaration
counts are the site counts; these are not.

`internal/ollama/wire.go` **5** — the argument struct (`:52`), the JSON-schema literal (`:61`), the replay
marshaller (`:144`), the native-call parser (`:231`), and the text-recovered-call parser (`:197`, whose parameters
are a flat string map). `internal/openaicompat/wire.go` **4** — the same minus the text-recovered-call parser,
because the second command exits **1** with no output: **`recoveredCall` does not exist in that package at all.**
The adapters are therefore **9 sites, not a symmetric 10**. Outside them: `JudgeResult`, `ToolExchange`, `ToolCallRecord`,
`cappedExchange`, `toolCallRecords` and the system text — **6 more, for 15**. The port and client work is *not*
counted, because this design needs it either way and a shared cost is not a delta. It also opens a failure class that does not otherwise exist:
**a model-authored date string**, which can be `"yesterday"`, `"2026-09-32"`, or an instant in an unstated zone,
and which must be validated and reported on every round.

Against that, the derivation route reuses a step whose single responsibility already **is** "turn the request into
the parameters retrieval runs with". A time window is a retrieval parameter. Putting it anywhere else creates a
second place windows come from (#1136 §2, Form 2 — parallel layer).

**What the model loses:** it cannot widen or narrow the window mid-run. Accepted. The measured failure is the
opposite one — a model with no bound at all burning its whole budget — and a window only exists when the input
asked for one, so a run bounded to "today" was asked about today.

**Rejected alternative, named because it is the tempting one:** detect time expressions in the loop with a keyword
list or regexp, with no model involvement. Natural-language time expressions are unbounded (*"since Monday"*,
*"the last three days"*, *"im letzten Monat"*), and a hand-maintained list of them is exactly the shape Toni ruled
against in #10913 — *a hand-maintained allow-list where the model's own shape was the gate*. The model already
reads the input. It answers.

### 4.2 D2 — there is no default window, and the shape makes the question disappear

**Ruling.** When the input expresses no time constraint, no window is applied and retrieval is byte-identical to
`6c8a1df`.

**This is not a default that was chosen; it is a value the derivation emits.** The output line carries either a day
range or the literal `none`. "What does the loop fall back to when nothing was said" has no answer because there is
no gap for the loop to fill — absence of a constraint is stated, not inferred. That is the decision the brief
listed as open, deleted rather than made.

For completeness, in case a default is ever proposed: a trailing-30-day default would silently exclude **59.9 %**
of the corpus (§2.2), including every design document this project has ever written. The exclusion would be
invisible in the answer. Under-inclusion is the silent direction of error (#1220 §5 addendum), and a default window
fails in exactly that direction.

### 4.3 D3 — the window's day is the day the prompt states, and the prompt states the host's civil day

**Ruling.** `Turn.Run` reads its clock **once**, keeps the reading's zone, states it in the prompt in RFC 3339 with
offset, and builds the window in **that reading's own location**. One instant, one frame, one run.

**The structural form matters more than the choice.** The window is constructed in `now.Location()` — the location
of the very instant the derivation model read to resolve the word "today". There is no second frame to disagree
with, so the two steps cannot drift apart by construction rather than by convention. A design that stated the
instant in UTC and filtered in local time would be wrong on purpose; one that did both in UTC would be
*consistently* wrong for the human who wrote the task.

**Why not UTC, which is what ships today.** `turn.go:148` reads `now := t.now().UTC()` and `assemble.go:78` formats
`now.UTC()`. On this host (+02:00) the UTC calendar date differs from the civil date during local `00:00–02:00` —
**8.3 % of the day**; on `-04:00` the mismatch inverts to local `20:00–24:00`, **16.7 %**. Inside that band a
question about "today" is answered with a **different calendar day entirely**. That is not a rounding error at a
boundary, it is a whole wrong day, and the answer looks completely normal.

**The substrate supports it — measured, not assumed.** `createdFrom=2026-09-15T00:00:00+02:00` returns 13 where
`…Z` returns 11 (§2.2). The graph filters on the instant an offset denotes.

**The cost, measured rather than reasoned, because it reverses a decision three days old.** The mutation §16
Milestone 3 prescribes — `turn.go:148` and `assemble.go:78`, those two lines and nothing else — applied at
`6c8a1df` on a green baseline, then `go build ./... && go test ./...` over the whole tree, unfiltered:

```
internal/loop/turn.go:148      now := t.now().UTC()                           ->  now := t.now()
internal/loop/assemble.go:78   b.WriteString(now.UTC().Format(time.RFC3339))  ->  b.WriteString(now.Format(time.RFC3339))
```

`go build` exit 0. `go test ./...` exit 1: **six guards red across three packages**, and a seventh whose name reads
as though it were affected is **green**.

| # | Guard | Package | What it pins | Why the mutation reaches it | Disposition |
|---|---|---|---|---|---|
| 1 | `TestRenderUserContentStatesTheInstantAsRFC3339InUTCWhateverZoneItWasGivenIn` | `internal/loop` | **the frame itself** — the rendered instant is forced to UTC whatever zone it arrived in | its property *is* what D3 reverses | **invert** → G8 |
| 2 | `TestRenderUserContentOpensWithTheRequestAndKeepsTheTailCopy` | `internal/loop` | the user-content layout: request, `NOW` span, block, request | its `want` embeds `userContentTestInstantUTC` (`usercontent_test.go:24`; const at `:16`) | **re-point the constant**, property unchanged → G14 |
| 3 | `TestRenderUserContentPlacesExactlyTwoVerbatimRequestCopiesTheFirstAtTheHead` | `internal/loop` | exactly two verbatim request copies, head and tail | its `blockLayout` embeds the same constant (`usercontent_test.go:79`) | **re-point the constant**, property unchanged → G15 |
| 4 | `TestTheRunRecordCarriesTheSameInstantTheAssembledPromptStates` | `internal/loop` | one clock read per turn, and `record.Now` == the prompt's instant | asserts the spelling `recordInstantUTC` (`promptclock_test.go:106` and `:114`; const at `:79`) | **split the constant** — `:79` is also read by row 7 at `:129`, so re-pointing it reddens the guard this table forbids editing. Row 4 reads a new sibling; `recordInstantUTC` is left byte-untouched → G16 |
| 5 | `TestJudgeSendsANativeUserMessageByteEqualToRenderUserContentOfTheSameBlockAndInput` | `internal/ollama` | the wire user message is byte-equal to `RenderUserContent` of the same inputs | **the byte-equality survives** — its `want` is computed from `RenderUserContent`, so it moves with the change. What fails is the *separate* `strings.Contains(…, instantUTC)` at `usercontent_test.go:51` — **the test's only absolute anchor**, not an incidental extra (§4.3.2) | **re-point one constant** (`:20`) → G17 |
| 6 | `TestJudgeSendsAUserMessageByteEqualToRenderUserContentOfTheSameBlockAndInput` | `internal/openaicompat` | the same property, same shape, same line numbers | the same | **re-point one constant** (`:20`) → G17 |
| — | `TestTheRunRecordsInstantIsOnTheWireUnderTheKeyNow` | `internal/loop` | the JSON key `now`, and RFC 3339 marshalling of whatever the field holds | **it does not reach it.** `promptclock_test.go:124` marshals `Record{Now: recordInstantLocal.UTC()}` — the test supplies the UTC value itself and never touches `turn.clock` | **green under the mutation. Do not edit it.** |

**The last row is the one worth reading twice.** An earlier revision of this document listed it among the guards to
invert, on the strength of its *name* — which does contain the property. It is a correct, passing test of a
*different* property, and an implementer told to re-point it would have been editing green code on this document's
authority. No citation check catches that: the name resolves, the line resolves, and the claim about what happens
when the code changes had simply never been run. **A resolved citation is not an observed outcome** — the guard
column and the result column are two different claims, and only one of them a resolver can audit.

**And the correction cuts both ways.** More guards move than the earlier count said — six, not three, reaching two
packages it never mentioned, which is why §12.4's adapter sentence is now scoped to production files. But five of
the six leave their property alone and change only what they expect, and rows 5 and 6 keep their byte-equality
assertion intact. The bundle is wider and shallower than it was stated to be.

### 4.3.1 The remedy was run, and one of the five expectation edits is not one edit

D3's mutation plus the edits this table prescribes for rows 1–6, applied at `6c8a1df`: `go build ./...` exit 0,
`go test ./...` **exit 0, 13/13 packages ok**, with `TestTheRunRecordsInstantIsOnTheWireUnderTheKeyNow` and the
constant it reads **byte-untouched**. That is Milestone 3's acceptance predicate and it is satisfiable — but only
after one decision the word *re-point* hides.

Every constant these rows name has a reader set, and changing it is one edit only when that set is unanimous:

| constant | readers at `6c8a1df` | guards reading it | unanimous? |
|---|---|---|---|
| `userContentTestInstantUTC` (`usercontent_test.go:16`) | `:24`, `:38`, `:40`, `:79` | rows 1, 2, 3 | **yes** — all three want the offset spelling once row 1 is inverted. One edit. |
| `instantUTC` (`ollama/usercontent_test.go:20`) | `:51`, `:53` | row 5 only, and it is function-local | **yes.** One edit. |
| `instantUTC` (`openaicompat/usercontent_test.go:20`) | `:51`, `:53` | row 6 only, function-local | **yes.** One edit. |
| `recordInstantUTC` (`promptclock_test.go:79`) | `:106`, `:108`, `:114`, `:116`, **`:129`** | row 4 **and row 7** | **no.** `:129` is row 7, which supplies its own UTC value and must keep expecting the `Z` spelling. |

**So row 4 is a constant *split*, not a re-point.** `recordInstantUTC` keeps its value and its definition and
becomes row 7's private constant — after the split its only reader is row 7's own assertion (`:129` at `6c8a1df`; the split adds a line, so it moves). Row 4 reads a new sibling holding the
same instant in the run's own zone. Nothing inside row 7's block is edited, which is what lets its prohibition
stand rather than collide with row 4's remedy.

Re-pointing the shared constant instead — the edit an earlier revision of this table prescribed — reddens row 7:

```
promptclock_test.go:130: the record wire carries no "now":"2026-09-12T10:30:00+02:00";
body={"input":"","subject":0,"now":"2026-09-12T08:30:00Z","query":"", ... }
```

Production is right there and the expectation is wrong. The trap is that an implementer meeting that failure has
been told three times not to touch the guard that just went red, leaving two visible moves — violate the
instruction, or abandon row 4's remedy — when the correct third move was never named.

**The rule this generalises to, because nothing about it is specific to time.** *A shared constant is a caller.* A
remedy can be correct for its own scope and broken by something else that reads what it touches, and no amount of
care **within** the scope finds it — the two edits alter each other's reachable input space, so each needs
re-proving against the other's new shape. Before prescribing an edit to any named symbol, run its reader inventory
(`git grep -n` for the symbol, scoped to its package) and ask of each reader whether it wants the new value. Where
the answer is not unanimous the remedy is a **split**, and the design owes that decision explicitly.

**And note which of this document's own safeguards missed it.** §16 M3's *"if your run disagrees with §4.3, your
run wins"* does not fire, because §4.3 is **correct** about the mutation — the disagreement appears only after the
remedy, a state the table never describes. §14 bounds G14–G17 to one unmeasured property: whether the re-pointed
versions **discriminate**. The defect sat in a second one nobody had named — **what else the prescribed edit
reaches**. A correctly-stated bound is not protection against the thing it bounds out.

### 4.3.2 Rows 5 and 6 keep an assertion that looks redundant and is not

Their byte-equality at `usercontent_test.go:48` computes its `want` from `RenderUserContent` itself. That is the
**correct** shape for a fidelity guard — its subject is whether the adapter transmits the loop's rendering verbatim,
so it must track the collaborator, and its survival under D3 is correct by construction rather than luck. Measured
by QA over three adapter-side manglings (append a space, swap block and input, substitute the adapter's own
rendering): **3 of 3 killed, control green.**

But a fourth mutation — `RenderUserContent` stops emitting the `NOW` span at all, so the instant never reaches the
wire — **passes `:48`**, because both arms of the comparison lose the span together. Only `:51` catches it.

**So `:51` is the pair's only absolute anchor**, and the only assertion that can detect the property the test's own
name claims. It is listed in rows 5 and 6 as a cost because that is where D3 touches it; it must not be read as the
annoying leftover of an otherwise self-sufficient check. Deleting it as redundant with `:48` would convert a
discriminating pair into a circular one.

**The general form, and the obvious rule is the wrong one here.** #8385's relation-guard rule — *a relation guard
is only as strong as the span its arms cover* — prescribes widening the arms' spread. That **cannot** repair this
pair: against a change on the producer both arms share, the arms move identically at every fixture, so no spread
separates them. #8385's own later sharpening (2026-09-15) states the rule that does apply, and it was written
about this exact pair in these two adapters:

> **Where the expected side comes from a producer both arms share, the paired absolute anchor is the guard. Do not
> price it as redundancy, and do not delete it on the strength of the relation-guard rule.**

`:51` is that anchor. Citing the relation-guard rule here would point a reader at the remedy that destroys it.

**No information is lost on the wire:** RFC 3339
with an offset denotes the same instant as its `Z` spelling, so any consumer comparing `Record.Now` as a time is
unaffected. What changes is which calendar day a *reader* — human or model — sees.

**Unchanged by this ruling, deliberately:** `internal/divoid/write.go:108` (`runName`) and
`internal/loop/summary.go:53` (`renderSummaryHeader`) format their own separate `at` argument in UTC. A durable node
name in UTC is a good convention and this design does not disturb it. That those two take a *second clock reading*
is **#13884**, and D3 does not settle it — see §17.

### 4.4 D4 — the window filters `updated`, and the limit is stated rather than hidden

**Ruling.** One field: `updatedFrom` / `updatedTo`. Not `created`, not a choice between them.

**Why `updated`.** The question class the task names is *what changed*. A node created in 2026-05 and rewritten
today changed today; a `created` filter misses it silently. And for a window whose end is **now** — which
*"today"*, *"this week"*, *"since Monday"* all are — `updated` strictly dominates: a node's `lastUpdate` is set at
creation (verified on #13718: created `13:13:43.445`, lastUpdate `13:13:43.910`), so every node created inside a
now-ending window also has its last update inside it. Measured today: `created` from 2026-09-15 = **11**,
`updatedFrom` 2026-09-15 = **22**, consistent with containment.

**Why not make it a choice.** A second knob is a second thing the derivation can get wrong, a second branch to
test, and #1136 §3 — configurability is not free. One field, named in the type.

**The falsifiable limit, stated because the claim above is bounded and not universal.** The graph stores only the
**latest** update, not a history. Therefore:

- For a **historical** window (*"what changed on 2026-09-05"*), an `updated` filter returns only nodes whose last
  touch was in that window. Anything edited since has moved out and is unreachable — **silently**.
- For that same historical window, `created` and `updated` do **not** contain one another in either direction. The
  containment argument above holds only while the window's end is now.
- No parameterisation fixes this. It is a property of the substrate. The 174-vs-51 figures in §2.2 are two
  cardinalities of a historical window, and are **not** evidence of containment.

**Direction of error:** `updated` over-includes (a 2024 node edited today appears in a "today" query — and it did
change today, so the inclusion is defensible, and similarity ranking demotes it). `created` under-includes, which
is the silent direction. The design takes the visible failure.

### 4.5 D5 — the window bounds topical recall; the anchor-scoped leg stays unbounded

**Ruling.** `Retrieve` applies the window to the per-query recalls (`retrieve.go:24`) and **not** to the scoped
recall (`retrieve.go:36`). `dispatchRecall` (`turn.go:404`) applies it.

**Why the exception is not special-casing.** The two legs answer different questions. The query legs answer *"what
in the graph is about this?"* — a question a time constraint narrows. The scoped leg answers *"what is structurally
adjacent to the subject?"* — two hops from the anchor, a question with no time dimension at all. Bounding it would
answer a question nobody asked, and would **silently empty the reserve** that `anchor-grounded-recall` built
deliberately: a one-day window over an anchor whose neighbourhood is months old returns nothing, and
`RecallScopeReserve` (3 of 20 slots, `turn.go:13`) goes unfilled.

**The cost, bounded and identifiable:** a windowed run can return up to **3** rows from outside its window. They are
not indistinguishable — every disposition carries `Sources`, and a scoped row's source has `scoped: true`
(`types.go:33-38`). A reader of the record can tell exactly which rows crossed the boundary and why.

---

## 5. Assumptions and Constraints

| # | Assumption | Confidence | If false |
|---|---|---|---|
| A1 | The wire spellings are `updatedFrom` / `updatedTo` | **From #13721 only; not re-verified by my instrument** | Milestone 1 catches it before anything is built. This is the one assumption the design cannot absorb. |
| A2 | An unsupported parameter is **accepted and ignored**, not rejected | From #13721: five of nine probed names returned `total` identical to the unfiltered control | Every verification in §16 pairs the filtered call with an unfiltered control. Without the control, a wrong name looks exactly like a right one. |
| A3 | The graph parses RFC 3339 with offsets and filters on the instant | **Measured**, §2.2 | D3 collapses to UTC-only and the 8.3 % band becomes a stated limit instead of a fix |
| A4 | `lastUpdate` is set at creation | **Verified** on #13718 | D4's containment argument for now-ending windows fails; the ruling survives on the *what changed* argument alone |
| A5 | The derivation call remains outside `MaxModelCalls` | `the-query-the-graph-is-asked.md` §4.4, code at `turn.go:194` | D1's decisive argument weakens to a site-count argument, which still wins |
| A6 | The model can read a stated instant and name calendar days | Untested here | Derivation yields no valid `DATES` line → no window → today's behaviour. The failure is a no-op, not a wrong answer. |
| A7 | Atomic deploy, single private repo, no external consumer of `Record` | Project convention | — |

**Constraints.** Go 1.27. `internal/loop` declares its ports and depends on no adapter. `Assemble` is pure — no
clock, no I/O — and stays so. The model endpoint is an HTTP service on `gangolf:11434`; nothing in this design
changes how it is reached.

---

## 6. Architectural Overview

```
Turn.Run
  │
  ├─ clock read ONCE ─────────────────► now   (keeps its zone — D3)
  │                                      │
  ├─ derive(input, now) ─────────────────┤          ← one model call, outside MaxModelCalls
  │    derivation prompt = instructions + exemplars + NOW(now) + input
  │    completion  ─► ParseDerivation(text, input, now.Location())
  │                       │
  │                       ├─► queries []string        (as today)
  │                       └─► window  UpdateWindow    (zero when "DATES: none")
  │
  ├─ Retrieve(queries, window)
  │    ├─ per-query recall  ──► Recall(q, fetch, nil,   window)   ← bounded   (D5)
  │    └─ scoped recall     ──► Recall(q0, fetch, scope, zero)    ← unbounded (D5)
  │
  ├─ Assemble ──► block                       (unchanged, still pure)
  │
  ├─ RenderUserContent(block, input, now, window)
  │      ===== NOW =====
  │      2026-09-15T02:30:00+02:00
  │      retrieval is limited to nodes updated 2026-09-15 … 2026-09-15
  │
  ├─ judge loop
  │    └─ dispatchRecall ──► Recall(modelQuery, CandidateLimit, nil, window)  ← bounded (D5)
  │
  └─ Record{ Now: now, Window: window, … }
```

**The spine:** one clock reading, whose zone is the frame, whose day-words the model resolves, whose window the
loop builds in that same zone, and which every bounded recall in the run shares. Nothing in the diagram is new
except `UpdateWindow` and the line that produces it.

---

## 7. Components and Responsibilities

### 7.1 `internal/loop/derive.go` — reads the request, names the days

**Owns:** turning one request into the parameters retrieval runs with. That already includes the queries; it now
includes the day range. **Does not own:** zone arithmetic, wire spellings, or what the window is applied to.

**Prompt.** Gains (a) the stated instant, in the same `===== NOW =====` span shape the judgement prompt uses, so
the model meets one vocabulary rather than two; (b) an instruction to emit **one additional first line** naming a
day range; (c) one sharpened rule; and (d) — load-bearing, and easy to miss — **reconciliation with the three
directives the new line contradicts.**

A `DATES:` line placed first is a sixth line, is not a question ending in `?`, and is literally a preamble. Each of
those is forbidden by text already in this same prompt, rendered from `MaxDerivedQueries = 5` (`derive.go:14`) at
`derive.go:78`:

| site | rendered text | conflict |
|---|---|---|
| `derive.go:33` | *"Output exactly 5 lines:"* | the output is now 6 |
| `derive.go:34` | *"The first 4 lines are distinct, standalone questions (each ending in ?) …"* | the first line is now the `DATES:` line |
| `derive.go:36` | *"Output ONLY those 5 lines. No numbering, no bullets, no quotes, no preamble, no commentary …"* | the `DATES:` line is a preamble |

All three must be restated to describe the new shape — one `DATES:` line followed by five query lines, of which the
first four are questions — rather than left for a later instruction to override. **The failure is silent by
construction:** §7.2 turns every unrecognised shape into a zero window, so a model obeying the unreconciled count
directive emits no `DATES:` line, gets no window, and behaves exactly as it does today. Milestone 4's property
never becomes true and nothing reports it. This is the one change in the design whose omission produces no error,
no red test and no log line.

**Output contract — one extra line, first, prefixed:**

| Form | Meaning |
|---|---|
| `DATES: YYYY-MM-DD..YYYY-MM-DD` | the request constrains retrieval to these calendar days, inclusive of both |
| `DATES: none` | the request expresses no time constraint |

A single day is expressed as both endpoints equal. The prefix is what makes the line distinguishable from a query,
so parsing degrades safely (§7.2).

**The sharpened rule**, replacing nothing and sitting beside `:32`'s existing "do not invent specifics":

> A query line must never contain a date, a month, a year, or a day-word. A semantic search matches a date only
> where that date appears as verbatim text, which is nowhere. Time constraints go on the `DATES:` line and nowhere
> else.

This is Toni's ruling written into the step that generates queries — so the derivation cannot itself reproduce
#13718's failure mode, which is a live risk the moment the derivation is told what day it is.

**Exemplars.** Both `derivationExemplars` entries gain the line: one carrying a real range, one carrying `none`, so
the model sees both branches in the format-only examples. The exemplar struct gains one field. This is not optional
— without it the model has no shape to copy and A6 fails on every run.

### 7.2 `internal/loop` — the parse, and where the frame enters

`ParseDerivation` gains one argument, a `*time.Location`, and one return value, the window. It stays **pure**: the
location is passed in, never read from process globals, so the zone is explicit at the call site and fixed in
tests.

| Input condition | queries | window |
|---|---|---|
| First non-empty line matches the `DATES:` prefix with a valid range | as today, from the remaining lines | the range, resolved in the given location |
| First non-empty line matches the prefix with `none` | as today | zero |
| First non-empty line matches the prefix with anything else | as today | **zero** |
| No line matches the prefix | as today, all lines | **zero** |
| No usable query at all | — | error, as today; **no window** |

Every malformed shape degrades to *no window*, which is today's behaviour. A wrong window is never constructed from
an unparseable line — the design has no path from bad model output to narrowed retrieval.

**Resolution to instants**, in the supplied location:

- `From` = midnight at the start of the first day.
- `To` = midnight at the start of the day **after** the last day.

The half-open form is what makes an inclusive single-day request expressible at all, and it matches the graph's own
partition behaviour (§2.2: `createdFrom` + `createdTo` sum exactly to the unfiltered total, so the boundary is
counted once).

**A parsed range whose `To` precedes its `From` is a zero window**, not an inverted one. There is no case in which
the loop sends the graph a range it knows to be empty.

### 7.3 `internal/loop/turn.go` — one clock, one frame, one window

- `Run` reads the clock once and **keeps the zone** (`now := t.now()`).
- `derive` returns the window alongside the queries and the derivation error.
- The window reaches `Retrieve`, `RenderUserContent`, `judge` → `dispatchRecall`, and `Record`.
- `GraphPort.Recall` gains the window as its final parameter.

**Does not own:** deciding what the window is, or which field it filters.

### 7.4 `internal/divoid/client.go` — the only place a wire spelling appears

`Recall` sets `updatedFrom` and `updatedTo` when the window is non-zero, and **sets neither when it is zero** — not
an empty string, not a sentinel date. A zero window must produce a request byte-identical to today's, or every
un-windowed run silently acquires a filter.

**Does not own:** anything else. This is the single site in the tree that knows the parameters are called
`updatedFrom` and `updatedTo`.

### 7.5 `internal/loop/assemble.go` — one more line in an existing span

`RenderUserContent` gains the window and, when it is non-zero, renders one line inside the existing
`===== NOW =====` span, immediately under the instant.

**Why it earns its place** — this is a judgement, not a measurement. A model given narrow results and no
explanation re-queries, and re-querying against an unbounded surface is exactly what cost #13718 its budget. One
line tells it why the aperture is narrow, in the span that already exists for this run's temporal frame. No new
span, no new vocabulary member for the model to learn.

**Why not a new `===== WINDOW =====` span:** the instant and the window are the same kind of fact about the run.
Splitting them adds a vocabulary member that buys nothing (#1220, 2026-08-17 addendum — a future member is a list,
never a member).

`Assemble` itself is untouched and remains pure.

### 7.6 The record and the summary

`Record` gains one field, `Window`, omitted entirely when zero. Without it a run's dispositions are
uninterpretable — *"why only 51 candidates?"* has no answer, and the next #13720-shaped diagnosis would have to be
re-derived from the queries every time, as §0's was. Same argument that put `Queries` and `Now` in the record.
**Not "impossible":** §0 performs exactly that diagnosis against a record carrying no `Window`, because none exists
at `6c8a1df`. The field buys cheapness and durability, not possibility, and §11 states it in that form.

`RenderSummary` prints the window on the `ASSEMBLY` line when present. One line.

### 7.7 What is deliberately not touched

| Component | Why |
|---|---|
| `internal/ollama/wire.go`, `internal/openaicompat/wire.go` | The `recall` **tool** schema is unchanged (D1). A grep for `Recall` hits `translateRecall` / `recoverRecall` in both files — those are tool-call translation, not `GraphPort.Recall`, and they do not change. |
| `cmd/processor/system_text.go` | The model is told nothing new. It gains no capability to describe. |
| `internal/eval` | Sweeps run pinned query sidecars with no derivation call. `cmd/eval/sweep.go:54` passes a zero window; eval determinism is preserved by construction. |
| `Assemble`, `admit`, `fuse`, `RecallScope` | No time semantics. |

---

## 8. Interactions and Data Flow

**Flow A — the input names a time (the #13718 case, as it would run).**

1. Clock read once: `2026-09-12T15:13:43+02:00`.
2. Derivation prompt carries that instant and the input *"…what changed in this project's retrieval path today…"*.
3. Completion line 1: `DATES: 2026-09-12..2026-09-12`. Lines 2–6: five queries, **none containing a date**.
4. Window resolved in `+02:00`: `[2026-09-12T00:00:00+02:00, 2026-09-13T00:00:00+02:00)`.
5. Six per-query recalls, each bounded. Scoped recall unbounded.
6. Block assembled; `NOW` span states the instant and the window.
7. Any supplementary recall the model asks for is bounded to the same window.
8. Record carries `now` and `window`.

**Flow B — the input names no time.** Step 3 yields `DATES: none`; the window is zero; steps 4–8 issue requests
byte-identical to `6c8a1df` and the `NOW` span gains no line. **Flow B is the common case and must be provably
unchanged** — see §14.

**Flow C — the derivation fails or is unusable.** As today: queries fall back to the raw input alone,
`DerivationError` is recorded, and the window is zero. A run cannot acquire a window from a call that failed.

---

## 9. Data Model (Conceptual)

**`UpdateWindow`** — a closed instant range over the graph's *last update* field.

| Field | Meaning |
|---|---|
| `From` | inclusive lower bound; zero means unbounded below |
| `To` | exclusive upper bound; zero means unbounded above |

Invariants:

- Both zero ⇒ **unbounded**; retrieval is indistinguishable from today's.
- The type answers *"am I unbounded?"* itself. That predicate is needed at four sites (the client, the two render
  paths, the record's wire form), and holding it on the type is why the type exists rather than two loose
  parameters.
- Instants, not calendar dates. The instants are what was **sent**, which is the operative fact for diagnosis; and
  since they carry their offset, the calendar days are readable straight back off them
  (`2026-09-12T00:00:00+02:00 … 2026-09-13T00:00:00+02:00` says "the 12th, local" with nothing lost). The richer
  form is captured and the compressed one derived, per #1220's 2026-08-28 capture-the-original rule.

**Ownership.** `internal/loop` owns the type. `internal/divoid` translates it to wire parameters and owns nothing
about its meaning.

**Why a type rather than two parameters.** The pair travels together through `GraphPort.Recall`, `Recall`,
`Retrieve`, `RenderUserContent` and `Record` — five carriers. As loose fields that is `2 × 5 = 10` parameters and
four separate two-field zero-checks; as a type it is 5 and one predicate. Per RULING 2026-09-03 one new type per
short requirement is *suspicious*, so it is stated here rather than assumed: this is the whole of the new
vocabulary, it carries no behaviour beyond the zero predicate, and if a reviewer disagrees the substitution to two
parameters is mechanical and changes no logic.

---

## 10. Contracts and Interfaces (Abstract)

| Contract | Input | Output | Invariant |
|---|---|---|---|
| **Derivation completion** | one request, one stated instant | first line `DATES: <range>` or `DATES: none`, then the query lines as today | no query line contains a date, month, year or day-word |
| **`ParseDerivation`** | completion text, the input, a location | queries, window | pure; any unrecognised or malformed `DATES` value yields a **zero** window, never a guessed one; `To` ≤ `From` yields zero |
| **`GraphPort.Recall`** | query, limit, scope, window | candidates in the graph's own rank order | a zero window produces a request **byte-identical** to the pre-change one; a non-zero window sets both `updatedFrom` and `updatedTo` |
| **`Retrieve`** | anchor, queries, limit, reserve, window | fused candidates | the window is applied to every query leg and to **no** scoped leg |
| **`dispatchRecall`** | the model's query, the run's window | one tool exchange | the model cannot widen, narrow or clear the window |
| **`RenderUserContent`** | block, input, instant, window | user message | a zero window renders **byte-identically** to the no-window layout; a non-zero one adds exactly one line inside the existing `NOW` span |
| **`Record`** | — | JSON | the `window` key is **absent** when unbounded; present with both instants when bounded |

**Nothing above is a new port, a new service, or a new implementation of an existing interface.**

---

## 11. Cross-Cutting Concerns

**Failure.** Every failure path in this design terminates at *no window*, which is today's behaviour: derivation
call fails, derivation returns no usable query, `DATES` line absent, `DATES` value malformed, range inverted. There
is no path from a degraded input to a narrowed retrieval. That asymmetry is deliberate — a run that returns too
much is legible to its reader; a run that silently returned too little is not.

**Observability.** The window is in the record (§7.6) and in the summary line. A run that returned few candidates
can be told apart from a run that was bounded to few, by reading one field. Without it, #13720's diagnosis would
have to be re-derived from the queries every time.

**Security / PII.** None. A window is two instants.

**Concurrency, idempotency, retries.** Unchanged. The window is computed once per run and read-only thereafter. The
derivation call is already not retried (`the-query-the-graph-is-asked.md` §4.5) and this does not change.

**Consistency.** The window is a filter on mutable graph state; two runs with the same window can return different
rows. That is already true of every recall this system makes and is why dispositions carry content hashes
(`assemble.go:134-141`).

**Configuration.** None added. No knob, no environment variable, no default to tune. The zone is the host's, and
the host's zone is not a setting this project introduces (#1136 §3).

---

## 12. Quality Attributes and Trade-offs

### 12.1 The sizing ratio, stated out loud

RULING 2026-09-03 binds here. The requirement is two sentences. The design is:

| | |
|---|---|
| new types | **1** (`UpdateWindow`, two fields and a zero predicate) |
| new interfaces | 0 |
| new services, layers, packages | 0 |
| new config knobs | 0 |
| new vocabulary members the model must learn | 0 (the `recall` tool is unchanged) |
| new prompt spans | 0 (one line inside an existing span) |
| changed port signatures | 1 |
| new record fields | 1 |
| decisions **deleted** | 1 (D2 — see §4.2) |

One new type against a two-sentence requirement is on the line Toni's ruling draws, which is why §9 states the math
rather than asserting proportionality. Nothing else here is new; the rest is a parameter threaded through machinery
that already exists.

### 12.2 The necessity question, asked once, of this design

**Is this needed?** Yes, and the evidence is measured rather than argued. On 2026-09-12 a run spent its entire
six-call budget and grew its prompt from 17,451 to 43,651 input tokens re-serving **the identical five nodes** —
its own anchor and two structural group nodes among them — across five recalls that differed only in the date
string they carried (§0). It returned an empty answer and wrote no file. The date was not wrong; it was inert. With
a window, the same question returns that day's two run records at ranks 1 and 2 (§2.3). The system cannot answer
"today" at all today, and it does not degrade gracefully when asked — it degrades expensively and silently.

### 12.3 Performance

A one-day window prunes 98.5–99.6 % of the corpus before similarity work (§2.2). Toni's second claim — that the
filter *speeds up* the query — is confirmed directionally by cardinality; **wall-clock latency was not measured**
and this design does not claim it.

### 12.4 Maintainability

The wire spellings appear at exactly one site (§7.4). The frame appears at exactly one site — the clock read —
and propagates as data. **No adapter production file changes**, so the two-adapter duplication that makes
tool-schema changes expensive is not paid — the 9 sites §4.1 enumerates for the rejected route are all avoided.
What D3 does reach in each adapter is one *test* file, `usercontent_test.go:20`, whose hardcoded UTC spelling is a
one-constant edit (§4.3 rows 5 and 6).

### 12.5 The trade-offs, named

| Trade-off | Cost | Why taken |
|---|---|---|
| Model cannot adjust the window mid-run (D1) | a task whose time scope the derivation misjudges cannot be rescued within the run | the measured failure is *no bound at all*; and D1 buys a bounded **block**, which is worth a model call every time |
| `updated` only (D4) | historical windows lose anything touched since | substrate has no update history; the alternative errs silently |
| Scoped leg unbounded (D5) | up to 3 of 20 rows may fall outside the window | keeps a mechanism a prior design built; the rows are identifiable by `Sources[].scoped` |
| Local frame (D3) | **measured:** reddens 6 guards in 3 packages — 1 property inverted, 4 expectations re-pointed, 1 constant **split**, and a 7th that looks affected is not (§4.3, §4.3.1) | 8.3–16.7 % of the day is otherwise answered with the wrong calendar day, silently |
| Window in the record | one more field on a large struct | without it every windowed run's disposition list is uninterpretable |

---

## 13. Risks and Falsifiable Limits

**This design states no universal.** Each claim below names what would falsify it (#1220 §5 addendum).

| # | Claim | What would falsify it | Present? |
|---|---|---|---|
| L1 | A one-day window makes a "today" question answerable | a corpus day with no relevant node in it — the window then returns a small, confidently-wrong set instead of a large vague one | **Yes, at low volume.** Quiet days exist. Mitigated only by the window being visible in the prompt and record. |
| L2 | `updated` covers "what changed" | a historical window; the field holds only the *latest* update, so anything touched since has moved out — **silently** | **Yes**, §4.4. Unfixable at this substrate. |
| L3 | A zero window is indistinguishable from today | any request that acquires a parameter when the window is zero | guarded, §14 G1 |
| L4 | The model resolves day-words correctly from a stated instant | any run where it does not | degrades to no window (A6), never to a wrong one — **except** if it emits a *valid but wrong* range, which no guard can catch. Visible in the record. |
| L5 | The parameters are spelled `updatedFrom` / `updatedTo` | an unfiltered control returning the same `total` as the filtered call | **Not established by my instrument** (§0). Milestone 1. |
| L6 | The scoped leg's exemption is harmless | a run where the 3 out-of-window rows dominate the answer to a time-bounded question | possible; identifiable in the record by `scoped: true` |

**The hazard that made all of this invisible, restated because it governs §16:** an unsupported parameter is
**accepted and ignored**. Five of nine names probed in #13721 returned rows and a `total` identical to the
unfiltered control, and **all five looked fine**. No claim that a filter works may be made without an unfiltered
control beside it.

---

## 14. Coverage — the guards the implementation must carry

**Read this before the table.** None of these tests exists at `6c8a1df`, so **no falsifier here has been executed**.
The third column states the premise that makes the guard discriminate — a judgement, not a measurement — and the
implementer is expected to confirm each by mutation before claiming the row. Rows are named guards rather than
described mechanisms (#1220 §9).

| # | Guard | Pins | Why it discriminates |
|---|---|---|---|
| G1 | `TestAZeroWindowSendsTheSameQueryStringAsBeforeTheWindowExisted` | a zero window sets no parameter | asserts the **encoded query string**, not the absence of a value. A implementation setting `updatedFrom=""` or a zero-time sentinel passes an "is it empty" check and fails this one. |
| G2 | `TestANonZeroWindowSendsBothUpdatedFromAndUpdatedTo` | both parameters, RFC 3339 with offset | asserts the exact spellings against a recorded request. The spellings live at one site (§7.4), so this is the only guard that can catch L5 in-tree. |
| G3 | `TestRetrieveBoundsEveryQueryLegAndLeavesTheScopedLegUnbounded` | D5 | the fake records the window per call **and** the scope; a uniform implementation (all bounded, or none) fails on one of the two assertions. A guard asserting only "some call was bounded" would pass either mistake. |
| G4 | `TestTheSupplementaryRecallCarriesTheRunsWindow` | `dispatchRecall` is bounded | the premise: the model supplies only a query string, so a window observed on that call can only have come from the turn. This is the guard that closes #13718; without it the model can re-open the unbounded path. |
| G5 | `TestParseDerivationYieldsNoWindowForEveryMalformedDatesValue` | §7.2's degradation table | table-driven over malformed, absent, inverted and non-date values; asserts **zero**, not merely "not the parsed range". A parser guessing a partial range fails. |
| G6 | `TestParseDerivationResolvesADayRangeInTheLocationItIsGiven` | D3's frame | the same `DATES` line parsed against two fixed zones must produce two different instants. A parser reading `time.Local` or forcing UTC produces one, and fails. |
| G7 | `TestTheWindowIsBuiltInTheZoneOfTheInstantTheRunStates` | one frame per run | the turn's clock returns a fixed non-UTC zone; the window's location must equal the stated instant's. Fails against any implementation that normalises either to UTC — including today's. |
| G8 | `TestTheAssembledPromptStatesTheInstantInTheZoneItWasRead` | D3's prompt half | replaces `TestRenderUserContentStatesTheInstantAsRFC3339InUTCWhateverZoneItWasGivenIn`, asserting the offset spelling is preserved. |
| G9 | `TestAZeroWindowRendersTheUserContentByteIdenticallyToTheNoWindowLayout` | Flow B | byte equality against the current layout constant. A guard asserting "does not contain the word window" would pass an implementation that added a blank line. |
| G10 | `TestTheRecordOmitsTheWindowKeyWhenRetrievalWasUnbounded` | wire shape | asserts the key's **absence** and the absence of `0001-01-01`, matching the existing `Now` guard's shape. |
| G11 | `TestTheRecordCarriesTheWindowThatBoundedRetrieval` | diagnosis | asserts both instants **and their offset**, so a record normalised to UTC — which would lose which civil day was meant — fails. |
| G12 | `TestNoDerivedQueryContainsADate` | §7.1's sharpened rule, at the parse boundary | a shape check over the parsed queries. **Limit:** it pins the parser's contract, not the model's output; the model is not in the test's reach. Stated rather than claimed. |
| G13 | `TestEvalSweepRunsWithNoWindow` | eval determinism | asserts the window observed by the sweep's fake is zero. A future change that threads a derivation into eval fails here rather than silently changing corpus scores. |
| G14 | `TestRenderUserContentOpensWithTheRequestAndKeepsTheTailCopy` *(existing, re-pointed)* | §4.3 row 2 | the layout property is frame-independent and stays as written; only `userContentTestInstantUTC` at `usercontent_test.go:16` becomes the offset spelling. Discriminates as it always did — a renderer that drops or reorders the `NOW` span still fails it. |
| G15 | `TestRenderUserContentPlacesExactlyTwoVerbatimRequestCopiesTheFirstAtTheHead` *(existing, re-pointed)* | §4.3 row 3 | same constant, same reasoning. Its real property — two verbatim request copies — is untouched by the frame, and an implementation that emitted one copy fails it either way. |
| G16 | `TestTheRunRecordCarriesTheSameInstantTheAssembledPromptStates` *(existing, re-pointed onto a **new** constant)* | §4.3 row 4 | pins **one clock read per turn** (`reads != 1` is a `t.Fatalf`) as well as the spelling. That half is frame-independent and is the half G7 leans on. Its expectation must move to a **sibling** constant: `recordInstantUTC` is shared with the one guard that must not change (§4.3.1), so editing it in place reddens that guard. |
| G17 | `TestJudgeSends…UserMessageByteEqualToRenderUserContentOfTheSameBlockAndInput`, both adapters *(existing, re-pointed)* | §4.3 rows 5 and 6 | the byte-equality assertion needs no change — its `want` is computed from `RenderUserContent`. Only the standalone `strings.Contains(…, instantUTC)` at `usercontent_test.go:51` moves, in each adapter, one constant at `:20`. **`:51` must survive that edit, never be dropped as redundant** — it is the pair's only absolute anchor and the only assertion that catches the loop losing the `NOW` span entirely (§4.3.2). |

**One named guard needs no replacement and must not be edited.**
`TestTheRunRecordsInstantIsOnTheWireUnderTheKeyNow` measured **green** under D3's mutation (§4.3, last row). It
pins the wire key and the marshalling of whatever the field holds, not production's frame. It is recorded here so
that an implementer sweeping §4.3 for "guards about the instant" meets the adjudication rather than the name.

**The falsifier for this table itself:** any row whose named guard would still pass against an implementation
lacking the claimed property. G1, G3, G5, G7, G9 and G11 are each written so that the *nearest wrong
implementation* — empty-string parameter, uniform bounding, partial parse, UTC normalisation, extra whitespace,
UTC record — fails them, and that is what the third column records.

**G14–G17 are the one part of this table that is not a prediction.** They are existing tests, and their behaviour
under D3's change was **observed** rather than reasoned: six red, one green, from one `go test ./...` at `6c8a1df`
under the two-line mutation (§4.3). That is also their limit — the run establishes that they *move*, and says
nothing about whether the re-pointed versions discriminate, because the re-pointed versions do not exist yet.
Their third column is a judgement like every other row's.

**Not covered, and named rather than omitted:** no guard can catch a model emitting a **valid but wrong** day range
(L4). There is no oracle for it in-tree. It is visible in the record and nowhere else.

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

- **No new type mirroring an existing one.** `UpdateWindow` has no counterpart in the tree; §2.1's grep returns
  nothing. Its cost is stated as math in §9, not asserted.
- **No new abstraction with one implementation.** No interface added. `GraphPort` gains a parameter, not a method —
  a sibling `RecallWindowed` was considered and rejected as #1136 §2 Form 2.
- **No element justified by "we might need X later."** The window is applied where a failure was measured (§0) and
  nowhere else. No timezone setting, no retention default, no per-field choice.
- **No deprecation period, feature flag, compatibility shim or transition window.** Atomic deploy, single repo.
  Both the port signature and the three PR #76 guards change in place.
- **Block-level DRY.** No block is inlined at multiple sites by this design. The wire spellings appear once
  (§7.4); the frame is established once (the clock read) and propagates as data. `block_size × site_count` is not
  reached because no duplication is proposed.

**Existing systems first**

- **Audited.** The derivation step already owns "turn the request into retrieval parameters"
  (`the-query-the-graph-is-asked.md` §7.1). The window is a retrieval parameter, so it goes there rather than into
  a new step. The `NOW` span already exists for this run's temporal frame, so the window line goes inside it rather
  than into a new span.
- **No new layer proposed**, so the "why can't it live on the existing surface" question does not arise. The only
  new persisted data point is `Record.Window`, and the concrete decision it enables is named in §7.6: without it a
  windowed run's disposition list cannot be interpreted, which is the exact diagnosis this task rests on.
- **Consumer chain recursed** for `Record.Window`: written by `Turn.Run` → read by `RenderSummary` (the summary is
  the node's `substance`, which is what every future recall of that record retrieves) → read by a human diagnosing
  a run, which is how #13718 was diagnosed here. Named consumer at the end, not a dead end.

**Configurability**

- **Zero new knobs.** No operator to name, no environment difference, nothing sensitive.
- No telemetry-then-tune compound.
- No magic numbers introduced. `MaxDerivedQueries`, `CandidateLimit`, `RecallScopeReserve` are untouched.

**Less is better**

- **Can it be deleted?** Run on every element. `Record.Window` — deletable, kept, reason named (§7.6). The prompt's
  window line — deletable, kept, reason named and **labelled a judgement rather than a measurement** (§7.5). The
  `UpdateWindow` type — deletable in favour of two parameters, kept, math given (§9). Everything else is load-
  bearing: without it the feature does not exist.
- **Can it be merged?** The window line merged into the existing `NOW` span rather than taking its own. The parse
  merged into `ParseDerivation` rather than a second pass.
- **Can it be inlined?** The zero predicate is one method on the type serving four call sites; inlining it is four
  two-field comparisons.
- **Radical-clean where unconsumed:** not applicable — nothing is being removed.
- **Trade-offs named explicitly:** §12.5, five rows.

**Data deliverables** — none. No SQL, no migration, no backfill.

**Document discipline**

- Cites Code Contracts **#114 §0** and Design Contracts **#1136** as load-bearing. #114 §0 is cited, not restated.
- Scope and non-scope both explicit, §3, including the four adjacent items that are **filed elsewhere** rather than
  dropped.
- No multi-paragraph rationale for anything that obviously stays.
- **Supersedes nothing.** No predecessor document is left live as if current. `the-query-the-graph-is-asked.md`
  remains correct — this extends its step, it does not replace it. One consequence for it is named in §16
  milestone 5.

---

## 16. Implementation Guidance for the Next Agent

Five milestones. Each has one property that must become true. **The lists of sites under each are what I found, not
a specification — sweep for the property, do not stop at the list** (#1220, 2026-09-10 addendum).

### Milestone 1 — establish the wire spelling, before writing anything

**Property:** it is measured, against an unfiltered control, that `updatedFrom` and `updatedTo` filter on the live
graph.

This is L5, the one assumption the design cannot absorb, and A2 is why the control is mandatory rather than
diligent. Issue three calls with the same query and `count=1`: no filter, `updatedFrom` alone, `updatedFrom` +
`updatedTo` for one day. Compare the `total` of each against the control. **A filtered call whose `total` equals
the control's has not filtered** — it has been silently ignored, which is what five of nine probed names did in
#13721.

If the spellings are wrong, stop and report. Do not guess neighbours (`updatedAfter`, `lastUpdateFrom`) — those are
among the five that were measured inert.

*Evaluate in memory, write no file, pipe rather than redirect, and report only counts and parameter names.*

### Milestone 2 — the window exists and reaches the graph

**Property:** `GraphPort.Recall` can express an update window, exactly one site in the tree knows its wire spelling,
and a zero window produces a byte-identical request.

Add `UpdateWindow` and the parameter; implement it in `internal/divoid`. Guards **G1, G2**.

Sites found — the property is *every* declaration, implementation and call of `GraphPort.Recall`. Three commands,
each scoped so that its output **is** the partition rather than something asserted about the output:

```
git grep -n -E 'Recall\(ctx context\.Context|graph\.Recall\(|Graph\.Recall\(' 6c8a1df -- '*.go' ':!*_test.go'
git grep -c -E '\) Recall\('  6c8a1df -- '*_test.go'
git grep -c -E '\.Recall\('   6c8a1df -- '*_test.go'
```

Run at `6c8a1df` they emit, respectively: **6 production lines** — `internal/divoid/client.go:163`
(implementation), `internal/loop/turn.go:61` (declaration), `internal/loop/retrieve.go:24` and `:36`,
`internal/loop/turn.go:404`, plus `internal/loop/turn.go:399` (`dispatchRecall`'s own signature, which must carry
the window through to `:404`); **9 test-double method signatures across 6 files**; and **13 test call sites across
2 files** (`internal/divoid/client_test.go` 10, `internal/loop/retrieve_test.go` 3).

A looser `git grep -n 'Recall('` also returns `translateRecall` / `recoverRecall` in both adapters. **Those are
`recall`-tool translation and do not change** — the first command's pattern excludes them deliberately, and they
are named here so a reader running the loose form does not read the difference as a miss.

### Milestone 3 — one clock, one frame

**Property:** the run reads its clock once, and the instant's zone is the frame for both what the prompt states and
how the window is built.

`turn.go:148` stops normalising to UTC; `assemble.go:78` stops re-normalising. Guards **G7, G8**, plus **G14–G17**
for the existing tests that move.

**Run the mutation before editing any test.** §4.3's table is the output of `go test ./...` under exactly these two
edits: **six guards go red, and one guard whose name suggests it should is green.** Row 1 has its property
inverted; rows 2–6 change only what they expect, leaving their properties alone;
`TestTheRunRecordsInstantIsOnTheWireUnderTheKeyNow` is **not** to be touched. Do not edit a test your run does not
redden — re-derive the list from your own run rather than from this table, and **if your run disagrees with §4.3,
your run wins and this document is wrong.**

**Then run the reader inventory before making any of those edits, because the mutation table is not the
specification — §4.3.1 is.** For each constant an expectation edit would touch, `git grep -n` the symbol across its
package and check every reader: three of the four are unanimous and take one edit, and `recordInstantUTC`
(`promptclock_test.go:79`) is **not** — it is shared between row 4 and row 7, so it is **split**, never re-pointed.
Re-pointing it reddens row 7, and the run that says so is in §4.3.1.

**This milestone's acceptance predicate is a green suite, not a red one.** Apply the D3 mutation and the row 1–6
remedies and run `go test ./...`: it must reach **exit 0 across all 13 packages with `promptclock_test.go`'s row-7
assertion and the constant it reads unedited**. Measured achievable at `6c8a1df` (§4.3.1). If your run cannot reach
green with row 7 intact, stop and report it rather than editing row 7 — that outcome would mean this design is
wrong about the split, not that the prohibition should yield.

Sweep for the property, not the rows: **every site that forces a frame on the instant this run states.**
Note the two sites that must *not* change — `internal/divoid/write.go:108` and `internal/loop/summary.go:53`
format their own separate `at` argument, and their UTC form is correct (#13884, §17).

### Milestone 4 — the derivation names the days

**Property:** the derivation output carries a day range or an explicit `none`, the parse is pure and degrades to
zero on every malformed shape, and no derived query contains a date.

`derive.go`: the prompt gains the instant, the `DATES:` instruction and the sharpened no-dates-in-queries rule;
both exemplars gain the line (one range, one `none`); `ParseDerivation` gains the location argument and the window
return. `turn.go`'s `derive` threads it. Guards **G5, G6, G12**.

**A second property, and it is the one with no guard behind it:** no directive in `derivationInstructions`
contradicts the shape the model is now asked for. `derive.go:33`, `:34` and `:36` are the three I found (§7.1) —
read the whole of `derivationInstructions` (`:23-38`) rather than stopping at them, because a directive that merely
*implies* the old shape counts too. **Verify by rendering the prompt and reading it**, not by diffing the source:
the conflicts are between sentences, and only the rendered text puts them side by side. Nothing in the suite can
catch a miss here — §7.2 makes it a silent no-op.

### Milestone 5 — the window is applied, stated and recorded

**Property:** every topical recall in the run is bounded by the window and no scoped recall is; the prompt says so;
the record says so.

`Retrieve` and `dispatchRecall` apply it; `RenderUserContent` renders the line; `Record` and `RenderSummary` carry
it; `cmd/eval/sweep.go:54` passes zero. Guards **G3, G4, G9, G10, G11, G13**.

**One documentation consequence:** `docs/architecture/the-query-the-graph-is-asked.md` describes the derivation
step's output as the query set. The property that must become true throughout that file is — *the derivation step
produces the query set **and** the retrieval window*. Sweep the whole file for statements that enumerate the
derivation's outputs, define its prompt shape, or assert what its completion contains; §7.1, §10 and §16's Unit 1
are the ones I found, and that list is **not** exhaustive. Counting language (*"exactly N lines"*, *"the last
line"*, *"one query per line"*) is the grep handle.

### Pre-submit, mechanically, on every file the branch touches

Extract every `file:line` citation and resolve it against the branch commit; extract every `#N` node id and resolve
it against its node; check any section coordinate against the target's actual headings. All three are commands, not
judgements. This document's own citations were resolved against `6c8a1df` before it was submitted.

---

## 17. What this design does not fix

**Supplementary recall re-serves material the block already carries.** Found in §0 and not fixed here: `admit`
(`assemble.go:31`) applies no anchor exclusion and no cross-round deduplication, so #13718's tool rounds re-served
the run's **own anchor** (`#10422`) five times, along with two structural group nodes. `fuse` excludes the anchor
for the block (`retrieve.go:86`); `dispatchRecall` does not. A window narrows the population but does not stop the
repetition — the same five nodes inside a window are still the same five nodes. **Filed as #13892**, with the
measurement and the asymmetry table, because its fix touches the admission path rather than the retrieval
parameters and because whether the exclusion belongs client-side or on the wire is #13604's question.

**#13884 — the second clock read.** §4.3 rules that the instant a run *states* carries the host's zone. It does not
touch `RenderSummary`'s header (`summary.go:53`) or `runName` (`write.go:108`), which format a **different**,
separately-read instant. My ruling determines their *frame* should they ever be unified with `Record.Now` — one
instant per run — but the work of unifying them is #13884's, not this design's, and nothing here makes it more or
less urgent.

**Historical windows lose what was touched since.** L2. A property of the substrate; no parameterisation fixes it.

**A valid-but-wrong day range.** L4. No in-tree oracle. Visible in the record.

**Latency.** §12.3. Cardinality was measured; wall-clock was not.

**Node #8's missing documentation of the search surface — and it is less filed than it looks.** Resolving the
citation showed that **#13604 is a task about `GET /api/nodes` having no *negation* predicate**. The documentation
gap appears inside it as a two-sentence aside ("*two facts worth checking before designing*") naming `query=`,
`similarity` and `substance`, and it predates the date-filter finding entirely — **the four date parameters are
recorded in no task at all.** So a design that depends on `updatedFrom` / `updatedTo` is depending on a surface
nothing in the graph documents and nothing tracks documenting. Milestone 1 is the compensating control, but it is a
control, not a fix. Raised for the operator rather than filed here, because §3 put this item out of scope on the
premise that it was already filed.
