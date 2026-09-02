# Architectural Document: The Run Record's Own Fate

> Repo path: `docs/architecture/run-record-fate.md` (canonical copy — the DiVoid node carries the same
> document verbatim).
> Findings settled: **#10899** (the stored `written` is `{}`), **#10863** (the orphan on a partial
> write-back), **#10890** (the drain grace is five seconds; a run is not).
> Project: **#10422** · Vision: **#10424**, including **refinement round 8** · Predecessor design:
> **#10532** (M1, the skeleton loop) — **consumed, partially corrected, not superseded.** §16 lists the
> four paragraphs of it this document overrides and the amendment that must land with this one.
> Standards applied: Design Contracts **#1136**, Code Contracts **#114 §0 and §4** (§4 via the Go annex
> **#10861** — **cited, not restated**), vocabulary rule **#1220 §2**, falsifiable-universals rule
> **#1220 §5**, edge conventions **#7216**, container rule **#10440**.
> Baseline: `main` at **`5e84c00`**. Every fact in §3 was read out of that tree or out of the two live
> records; the two that are **not** measurements are labelled as such in the table rather than passed
> off as ones.

---

## TL;DR

*A record can account for everything up to and including its own composition. It cannot account for its
own filing. Three surfaces, not one, and the design says which fate lives where.*

**The finding behind the three findings.** M1's philosophy is §6.5's table: **every way a run can fail
ends in something that names the failure.** All three open findings are places where a run reaches a
state that nothing names — the stored record's `written` is a third, undefined value (`{}`); a
half-succeeded write leaves a node the design never described; and a shutdown mid-run erases the run
entirely. They are one question: *what must a run record be able to say about its own fate, and where
does it say it?*

**The answer, in one line each:**

| Surface | Reader | Carries |
|---|---|---|
| **The stored node body** | milestone 2's corpus | the record — everything about the *turn*. **Nothing about its own filing** |
| **The HTTP response body** | the caller, a human with `curl` | the same record, byte-for-byte, **plus exactly one key**: the write receipt |
| **stderr** | the operator | the fates no record can carry: the orphan left behind, the wall clock, the upstream diagnosis, and the run that ended before a record existed |

**"The response body *is* the record" does not survive**, and is not repaired by a fourth request. It is
replaced by a claim that is both stronger and checkable: **the stored body is the response body minus
exactly one key.** A test asserts that; byte-identity never could.

**The write receipt is a closed set of three**, each with a producer and a reader **today**: `stored`,
`unlinked`, `notStored`. It lives only in the response, because the stored copy can express nothing by
carrying it — a stored record is at the node it would be naming.

**The partial write is ruled by one sentence:** *the record is the artifact, the node is its container;
keep any container that holds the record, discard any container that does not.* A failed link leaves a
complete record and is kept (`unlinked`). A failed content write leaves a name-only shell that is worth
nothing to anybody, and is deleted best-effort (`notStored`).

**The shutdown case is answered by drain, not by cancel — and the brief's own position is argued
against.** §8.4a of #10532 rejected a longer grace because *"every shutdown would hang for minutes"*.
That is **false about the mechanism**: `Shutdown` returns the moment connections are idle, so the grace
is a **ceiling, not a wait** (C47), and it is paid only when there is a run to protect. Draining
delivers the whole run — answer, record, spend recovered; cancelling delivers an assembly-only fragment
and costs a shutdown-aware context, a detached write path and a new terminal vocabulary. **KISS
decides.** What draining needs is a bound, and that bound is what #10890 asked for anyway.

**One fate stays unrecordable and is named rather than papered over:** the process dying before the turn
completes. Its only trace is a `run started` log line with no `run finished` — **and neither line
exists in the shipped tree** (C49), although #10532 §10.3 specifies both. That is this document's fourth
finding and its cheapest fix.

**Two new constants, one new error code, one field moved, one field dropped, four log lines.** No new
package, no new port, no new dependency, no retry, no queue.

---

## 1. Problem Statement

**The question, stated once:** *what must a run record be able to say about its own fate — and where does
it say it?*

M1 shipped a philosophy before it shipped a mechanism for it. #10532 §6.5 is a table in which every
failure path terminates in an artifact that names the failure: a `502` with a code, or a `200` whose
record carries the failure in a field. The three open findings are the three places that philosophy is
violated, and each is violated in the same way — **the run reaches a state the artifact has no way to
express**:

| Finding | The state | What the artifact says |
|---|---|---|
| **#10899** | The record was written to node N | `written: {}` — a third value the field's contract does not define, indistinguishable from *never attempted* |
| **#10863** | The node was created and the body set, but the edge was not | Nothing. The stored record is byte-identical to a fully successful one |
| **#10890** | `SIGTERM` arrived mid-run; the process exited before write-back | Nothing at all. No response, no record, and — measured — not even a log line (C49) |

Toni's framing is that these are *"I don't know where I went"*, *"I went somewhere unreachable"* and
*"I never got to go"*. That is right, and it locates the design gap precisely: **M1 designed what a
record says about the turn and never designed what it says about itself.**

### Why this outranks the cleanup queue

Milestone 2 is the retrieval eval harness, and it **scores a corpus of stored run records**. #10532 §8.2
already made this argument once, to justify the `limits` field: *"every record written before that change
is uninterpretable without knowing which values were in force."* The same reasoning was never applied to
the record's own fate. The corpus M2 will read is being written **now**, by every live run, and every
defect in it is permanent. This is on the M2 path.

### The shape borrowed from round 8

Vision #10424's eighth refinement rules that *a process step is not a choice*, and its sharpest
distinction is between **a step that ran** and **a step whose authority was resolved** — mode 3, *"ran,
and had no authority"*, which emits a green signal while failing. It also rules that **completion must
be evidenced by an artifact, never by a claim**.

The analogue here is exact and it is the design's organising principle:

> **A record that exists is not the same as a record that can account for itself.** A stored record whose
> `written` is `{}` is a green signal — it is a well-formed artifact in the corpus, and it asserts
> nothing it can be held to. Round 8's remedy applies unchanged: **make the property structural rather
> than remembered.** A field that cannot be true in one of its two destinations must not be *in* the
> type that goes to both.

### Success criteria

| # | Criterion | How it is judged |
|---|---|---|
| F1 | No run reaches a terminal state that no surface names | §6.1's table has a row for every state, and §6.1's last row is the one exception, named as an exception |
| F2 | The two artifacts' relationship is stated as something checkable | The stored body is the response body minus exactly one key. Asserted in a test, not claimed in prose |
| F3 | The write receipt is a closed set with no unproduced member | Each of the three values has a producer in this design and a reader today. The future list is prose (§8.2) |
| F4 | A shutdown mid-run no longer erases the run | The drain covers a bounded run; the residual is named and its detection is a log pair |
| F5 | Milestone 2's corpus is not degraded by any of the three | The stored record carries no undefined value, no field it cannot populate, and no missing rows from runs that vanished |

---

## 2. Scope & Non-Scope

### 2.1 In scope

- The **write receipt**: its vocabulary, its home, and what each value means.
- The **relationship between the two artifacts**, stated as a checkable claim.
- The **write-back sequence's partial-failure policy** — which container is kept, which is discarded.
- An **overall run bound**, and the drain grace derived from it.
- One new **error-envelope code** for the bound.
- The **per-run log pair** #10532 §10.3 specifies and the tree does not have, plus the wall clock it must
  carry.
- The amendment to #10532 that keeps it from reading as current where this document overrides it (§16).

### 2.2 Out of scope — declined explicitly

| Excluded | Why |
|---|---|
| **Anything in the loop's assembly, judgement or tool cycle** | M1 shipped and is verified live (#10883). This is about what the record says, not about how the turn runs |
| **A fourth request that patches the stored record after the node id is known** | §8.1. It restores a claim literally at the cost of adding a failure mode to a sequence whose failure modes are the subject of this document |
| **A self-reference marker in the stored copy** (`{"self": true}`) | §8.1. It costs a request to tell a reader something they established by reading the node |
| Retry or backoff on any leg of the write-back | #10532 §10.5 is unchanged and its surviving argument — three constants with no measurement behind them — binds here identically |
| A transaction, a two-phase commit, or an outbox | The graph API offers no transaction (C46). An outbox is a store, and Processor has none by design |
| A durable queue, a background writer, a retry worker | Same. The run is synchronous and the caller waits (#10532 §10.7) |
| Cancelling a run on shutdown and writing a cancellation record | §6.3 argues it and rejects it. Recorded as a rejected alternative, not omitted |
| Making `shutdownGrace` or the run bound configurable | #1136 §3. Both are constants; §8.4 states the derivation and the falsifier |
| A duration field on the run record | §8.6. The wall clock's reader is the operator choosing the bound, and the operator's channel is stderr |
| Reporting the write receipt's *cause* to the caller | §8.2. Every `notStored` cause produces the same caller decision; the diagnosis is the operator's and goes to stderr (#10532 §8.5) |
| A cleanup sweep for orphans already in the graph | There is at most a handful and they predate the rule. A human deletes them; #13.2 asks whether Toni wants one filed |
| Anything about milestone 2's scoring itself | This document's obligation to M2 is F5 and nothing more |

**The named future list, in prose, as #1220 §2 requires — these are not members and no implementer
declares them:** a `cancelled` receipt value, a `deferred` or `queued` value, a `supersededByRetry`
value, a fallback store, a per-leg failure reason on the receipt, a retry count, a run duration field, a
node-id echo on the discarded-shell path. **Each arrives with the milestone that first reads it.**

### 2.3 This is one PR

The three findings are one field, one sequence and one number, and they are not independently shippable
in any useful sense: the receipt's vocabulary is meaningless until the partial-write policy produces its
values, and the log pair that carries the unrecordable fate is the same instrument that measures the
bound. **One unit, one PR.** The orchestrator owns the decomposition call (#10192); this is the
architect's recommendation and the reason for it.

---

## 3. Measured Facts

The `C` numbering continues #10532's series, which ended at C44, so a label identifies one measurement
across all four Processor designs. **Two rows below are not measurements** and say so in place.

| # | Fact | Consequence |
|---|---|---|
| **C45** | **The stored record's `written` is `{}` and the response's is `{"nodeId": N}`** — read off the two live records **#10897** and **#10898** (#10883), the first two turns ever run | The defect is structural, not a fluke. Both artifacts of both runs show it |
| **C46** | **The cause, read out of the tree at `5e84c00`:** `internal/divoid/write.go` serialises the record as its **first** action, then issues create → content → link. The record therefore cannot contain the id the create is about to return. The receipt type's two members both carry `omitempty` and are both zero at that instant, so the object renders as `{}` rather than being absent | The `{}` is not a bug in the write; it is the honest output of asking an object to describe an event that has not happened yet. **The order — content before link — is load-bearing and is kept** (§6.2) |
| **C47** | **Not measured — read from the standard library's documented contract and from the shipped `serve` function.** `Shutdown` closes the listeners, then waits for connections to become idle, and **returns as soon as they are**; the context it is given bounds how long it is willing to wait. It is a **ceiling, not a fixed wait** | **#10532 §8.4a's argument against a longer grace is false.** It says raising the grace *"would make every shutdown hang for minutes"*. An idle shutdown returns immediately whatever the grace is. The cost is paid **only when a run is in flight**, which is exactly the case the grace exists to protect |
| **C48** | **No overall bound on a run exists at `5e84c00`.** The components each bound themselves: `divoid.DefaultTimeout` = 15 s per graph call, `openaicompat.DefaultTimeout` = **5 min** per model call, `MaxModelCalls` = 3. `internal/server` sets `ReadHeaderTimeout` = 5 s and `shutdownGrace` = 5 s, and no `ReadTimeout`, `WriteTimeout`, `IdleTimeout` or per-request deadline. `POST /runs` runs on `r.Context()`, which nothing gives a deadline | The composed worst case is **≈ 16 minutes**; the grace is **5 seconds**. A bound must be **stated**, not derived from the parts — a bound equal to the composed sum would be a number no operator would accept (§8.4) |
| **C49** | **The two per-run log records #10532 §10.3 specifies do not exist.** The whole module logs on nine lines; `internal/loop` logs on exactly two, both failure branches (write-back failed, supplementary recall failed). There is **no `run started` and no `run finished`** | **This is a fourth finding, and it is what makes #10890's worst case total.** A run killed mid-flight leaves no response, no record **and no log line** — nothing anywhere says it began. The design's own §10.3 already prescribes the fix |
| **C50** | **No surface records how long a run took** — not the record, not the log, not the response | The run bound (§8.4) is chosen without data, and cannot be re-derived from the corpus. §8.6 puts the wall clock on the log line that has to exist anyway |
| **C51** | **Not measured — from the container runtime's documented default.** `docker stop` sends `SIGTERM` and then `SIGKILL` after **10 seconds** unless `-t` says otherwise | **The supervisor caps the grace, not the process.** Raising `shutdownGrace` does not make a container take longer to stop under the default; it makes a longer drain *available* to an operator who asks for it. That bounds the cost of §8.4's choice and it is also the honest limit on the drain guarantee (§11 R3) |

---

## 4. Architectural Overview

Nothing new is built. Three existing surfaces are given non-overlapping scopes, and the fate of a run is
distributed across them by **what each reader can already know**.

```
                       POST /runs
                            │
                            ▼
   ┌─────────────────────────────────────────────────────────────────┐
   │ the turn — unchanged (assembly · judgement · tool cycle)         │
   │                                                                  │
   │  bounded, for the first time, by ONE deadline covering           │
   │  everything up to and including the answer          §8.4         │
   └─────────────────────────────────────────────────────────────────┘
                            │
                the answer exists ⇒ THE RECORD EXISTS
                            │
              ┌─────────────┴──────────────┐
              ▼                            ▼
    ┌──────────────────┐        ┌────────────────────────────────┐
    │ write-back       │        │ the response                   │
    │ (detached from   │        │                                │
    │  the request)    │        │  the record, verbatim          │
    │                  │        │  + ONE key: the receipt        │
    │ create → content │        └────────────────────────────────┘
    │        → link    │                     │
    │                  │                     ▼
    │ keeps a container│         stored body = response body
    │ that holds the   │                    minus exactly one key
    │ record; discards │
    │ one that does not│
    └──────────────────┘
              │
              ▼
    ┌──────────────────────────────────────────────────────────────┐
    │ stderr — the operator's channel                              │
    │   run started · run finished (wall clock, receipt, node id)  │
    │   the orphan that could not be discarded                     │
    │   the upstream diagnosis the caller-facing surfaces omit     │
    └──────────────────────────────────────────────────────────────┘
```

**The overview's load-bearing line is the one in the middle.** *The answer exists ⇒ the record exists.*
Everything before it is a run that may fail and leave no record — which is already #10532 §6.5's
philosophy, unchanged. Everything after it is a record that **will** be delivered somewhere, and the
design's job is to say where and to say so honestly when one of the two destinations did not take it.

---

## 5. Components & Responsibilities

No package is added, and no responsibility moves between packages.

| Component | Gains | Still does not own |
|---|---|---|
| `internal/loop` | The receipt **type** and its closed vocabulary (declared beside the record it accompanies); the per-run log pair; the detachment of the write-back step from the request's lifetime | The three HTTP calls, the discard decision's mechanics, the status code |
| `internal/divoid` | The partial-failure policy on the write sequence: which container is kept and which is discarded, and the outcome it reports for each (§8.3) | What the outcome *means* to a caller, and where it is rendered |
| `internal/server` | The run deadline applied at the handler; one new code in the error envelope; the receipt appended to the response body as one key beside the record's own | Any policy about the record or the write |
| `cmd/processor` | Nothing. **No new configuration member** — both new numbers are constants (§8.4) | — |
| `internal/openaicompat` | Nothing | — |

**Why the receipt's vocabulary is declared in `internal/loop` and not in `internal/divoid`.** The
adapter *produces* the fate; the loop *owns the record's shape and the vocabulary of everything the
record is accompanied by*, exactly as it owns the terminal-reason set the model adapter maps into
(#10532 §8.3). Putting the vocabulary in the adapter would make one graph implementation's words the
contract, which is the mistake §8.3 spent a revision removing from the model port. **Falsifier:** if a
second graph implementation would have to invent a fourth value to describe an ordinary outcome, the
vocabulary is the adapter's and belongs there.

**What is deliberately NOT a component:**

| Not built | Why |
|---|---|
| A write-back retrier, an outbox, a background queue | §2.2. Each is a store or a schedule, and the run is synchronous |
| A graph-hygiene sweeper for unlinked records | The graph answers "which run records have no edges" in one query. A component for that is a name, not a boundary |
| A `Fate` or `Outcome` service | The fate is three values on one field |
| A shutdown coordinator | The drain is `Shutdown`'s existing behaviour with a number that covers a run (§6.3) |

---

## 6. Interactions & Data Flow

### 6.1 The complete fate vocabulary — every terminal state a run can reach

This is the table Toni asked for. It is the whole set, including the states nothing currently records.
**Rows T1–T4 and T6 are #10532 §6.5 unchanged and are reproduced only so the set is complete**; the rest
is what this document adds.

| # | Terminal state | The caller gets | The graph gets | stderr gets | Status |
|---|---|---|---|---|---|
| T1 | Request malformed — `input` or `subject` missing or empty | `400 invalid_request` | nothing | nothing | unchanged |
| T2 | Subject id resolves to nothing | `404 subject_not_found` | nothing | nothing | unchanged |
| T3 | A graph read failed — anchor or the assembly recall | `502 graph_unavailable` | nothing | the upstream diagnosis | unchanged |
| T4 | The model call failed | `502 model_unavailable` | nothing | the upstream diagnosis | unchanged |
| **T5** | **The run exceeded its deadline before an answer existed** | **`504 run_deadline_exceeded`** | nothing | the diagnosis and the elapsed time | **new** (§8.4, §8.5) |
| T6 | Answered; all three write calls landed | `200`, record + receipt `stored` with the node id | the record, bodied and linked | the finished line | unchanged in behaviour, **named** for the first time |
| **T7** | **Answered; the node was created and bodied, and the edge was not** | `200`, record + receipt **`unlinked`** with the node id | the record, bodied, **not walkable from the subject** | the finished line **and** a repairable-orphan line carrying the node id | **new** (#10863) |
| **T8** | **Answered; no node holds the record** — the create failed, or the body write failed and the shell was discarded | `200`, record + receipt **`notStored`** | nothing | the finished line and the diagnosis | **existed; had no name and no defined value** (#10899) |
| **T9** | **Answered; the caller went away before the response could be delivered** | nothing — there is nobody to give it to | the record, filed anyway (§6.4) | the finished line | **new** (§6.4) |
| **T10** | **The process terminated before the turn completed** | nothing | nothing | **`run started` with no `run finished`** | **named, not eliminated** (§6.3, §8.6) |

**T10 is the design's one unrecordable fate and it is stated as such rather than engineered around.** An
artifact cannot describe its own non-existence. What *can* exist is the asymmetry: a started line with no
finished line is a signature the operator can grep for, and it is the reason §8.6 makes the log pair a
deliverable rather than a nicety. **Today T10 leaves nothing at all** (C49), which is why #10890's
description — *"the run is not slow, or failed, or recorded as failed; it is silently gone"* — is exactly
right, and why the log pair is the cheapest half of the fix.

### 6.2 The write-back sequence, and the one rule that governs its partial states

The sequence is unchanged: **create → set content → link** (C32, C46). It is not atomic, it cannot be
made atomic against an API that offers no transaction, and this design does not pretend otherwise. What
it does is decide what each surviving partial state means.

**The rule, and it generates every row below:**

> **The record is the artifact. The node is its container. Keep any container that holds the record;
> discard any container that does not.**

| The call that failed | What exists | Ruling | Receipt |
|---|---|---|---|
| **create** | nothing | nothing to do | `notStored` |
| **set content** | a node with a name and **no body** — a shell | **Discard it, best effort.** A bodyless run node is worth nothing to any reader: it cannot be scored, cannot be read, and its name is a copy of the caller's input, which makes it a high-scoring recall hit that answers nothing (#10532 §11 R13 is the same hazard, and this is its emptiest form). `DELETE` is measured to work (C32) | `notStored` |
| **link** | a node with the **complete record** in its body | **Keep it.** The record is correct and complete; it is findable by semantic search, which is how DiVoid's own conventions expect an unwalkable node to be reached. Deleting it would destroy the expensive artifact to repair cheap metadata | `unlinked` |
| **the discard itself** | the shell survives | Nothing further is attempted. The receipt is still `notStored` — **the record's fate is unchanged by whether the litter was collected** — and the surviving node id goes to stderr for a human to remove | `notStored` |

**Why the order matters and is now load-bearing.** Content before link means the only partial state that
*keeps* a node is the one whose node holds a complete record. Reversing the order would trade that for a
linked, bodyless node — discoverable from the subject and worthless when opened, which is the worse half
of both options. **The order was already right; this document is what makes it a decision rather than an
accident.** Falsifier: if the graph ever gains a create-with-body call, the shell state disappears and
this row goes with it.

**Why not compensate on the link failure too.** Deleting a complete record because an edge is missing
optimises walkability at the cost of the corpus. Milestone 2 finds records by type and by query, not by
walking from subjects (#10532 §9.4), so an unlinked record is a **full-value corpus row with a
navigation defect**. The delete would convert a small defect into a total loss.

**Why the compensating delete is not itself a hazard.** It runs on exactly one branch, against exactly
one node id this run just created, and its failure leaves the world in the state it would have been in
had it not been attempted. There is no case in which trying is worse than not trying. **Falsifier:** if
a delete ever removes a node this run did not create, the branch is wrong, and the check that prevents it
is that the id came from this run's own create response and from nowhere else.

**On "the orphan is undetectable from its own content" (#10899).** True, and **not a defect** — see
§13.1. Detecting an unlinked node from its own body would require the body to carry a fact the body
learns after it is sealed. Detecting it from the *graph* is one query. Detection belongs to the
substrate, and this is the substrate's job.

### 6.3 Shutdown mid-run — drain, and the argument against the brief's position

**#10890 leaves three sub-decisions open and Toni's stated reading is *"cancel and write a record naming
the cancellation"*, offered as a position to argue with. This section argues with it and lands
elsewhere.**

**Sub-decision 3 first, because it determines the other two. What is correct behaviour on a shutdown
mid-run?**

Three options with real differences:

| Option | What the run yields | What it costs |
|---|---|---|
| **A — cancel silently** | nothing | nothing. **This is today, and it is the one nobody chose** |
| **B — cancel and record the cancellation** | a record with a full candidate set and block, **no answer, no usage, no terminal reason** | a shutdown-aware context plumbed to the handler (the standard library's `Shutdown` does **not** cancel in-flight request contexts, so the run would not notice today); a write path that runs on a context deliberately detached from the one just cancelled; a new receipt or terminal value; a test that shuts down mid-run |
| **C — refuse new runs and drain the in-flight one** | **everything** — the answer to the caller, the record to the graph, the model spend recovered | one constant changed, one constant added, and one documented deployment flag |

**C wins, and the reason B looked competitive is a factual error in #10532 §8.4a.** That section rejected
a longer grace on the ground that *"raising the grace to cover a worst-case local generation would make
every shutdown hang for minutes to protect a single re-runnable request."* **`Shutdown` returns as soon
as connections are idle (C47).** An idle shutdown is instantaneous whatever the grace is. The cost is
paid only when there is a run in flight — which is to say, only in exchange for the thing being bought.
The argument confused a timeout with a sleep, and with it removed, C is strictly cheaper *and* strictly
better than B.

**What B would have delivered, weighed honestly.** A cancelled run's record is not empty: it carries the
whole assembly half, which milestone 2 can score. That is a genuine argument and it is why B is worth
arguing rather than dismissing. It loses on three counts. First, **the window is small** — the record
exists only after the model answers, and the fraction of a run spent after that point is seconds out of
minutes, so B's yield is mostly assembly-only fragments. Second, **a fragment costs a full node**: a run
record is 10–19× the median node in this graph (#10532 §11 R5) and is an unfiltered recall candidate
(R13), so B pays the corpus's heaviest per-row price for its thinnest row. Third, **B writes to the graph
while the process is tearing down** — the operation most likely to fail is scheduled at the moment
failure is most likely.

**And the residual is identical under both.** Neither B nor C survives a supervisor that sends `SIGKILL`
(C51). B narrows the window; it does not close it. Since neither option eliminates T10, the choice is
between the option that saves the whole run in the ordinary case and the option that saves a fragment in
a slightly wider one. **C, and T10 is named (§6.1) rather than claimed away.**

**Sub-decision 1 — what bounds a run overall?** #10890 offers a derived bound (`MaxModelCalls` × a
per-call timeout) and a flat ceiling. **A flat ceiling, and the derived shape is rejected on arithmetic:**
composed from the shipped constants it is **≈ 16 minutes** (C48), which is not a bound anyone would
accept and which moves silently whenever any component timeout changes. An overall deadline is not a sum
of the parts — **it is a statement about what the caller and the operator will tolerate**, and it must
therefore be stated. §8.4 states it, with its derivation and its falsifier.

**Sub-decision 2 — does `shutdownGrace` stay at 5 s?** **No, and it stops being an independent number.**
It was chosen when the longest request was a health check, and nothing re-derived it when the first
long-running endpoint arrived — which is precisely what map node #10461 predicted (*"the first endpoint
with a real body brings its own deadline requirement"*). It becomes **derived from the run bound**, so the
drain guarantee is arithmetic rather than hope: a run in flight has at most the run bound left, plus the
write-back, so a grace covering both always completes the drain (§8.4).

### 6.4 The write-back outlives the request, and the invariant that buys

**The write-back step runs on a context derived from the process's lifetime, not from the request's.**
Its bound is the graph client's own per-call timeout, which already covers each of the three (or four)
calls; no new deadline constant is introduced.

**What it buys is one invariant, stated as a universal with its falsifier:**

> **Once the model has answered, the record exists and is filed, regardless of what happens to the
> caller.**

Today a caller who disconnects after the answer — a `curl` interrupted with Ctrl-C — cancels the request
context, the write-back's graph call fails on that cancellation, and **both copies are lost**: the
response has nobody to go to and the graph copy was never attempted. That is #10890's shape at a smaller
scale and on a path nobody has looked at.

**Why this is not the same machinery §6.3 just rejected.** B's detached write exists to file a fragment;
this one exists to file a **complete record** whose model spend is already paid and whose only possible
surviving copy is the graph's. Same mechanism, different value, and the asymmetry is the whole
justification. **Falsifier for including it:** if a run whose caller vanished is judged not worth a node,
delete the detachment and the invariant weakens to *"…regardless of what happens to the caller, provided
the caller waits"*, which is not an invariant.

**Falsifier for the invariant itself:** any path on which the model answered and neither artifact exists,
other than T10.

---

## 7. Data Model (Conceptual)

Two entities, one of which is new and is **not** part of the record.

| Entity | Owned by | Scope | Lives where |
|---|---|---|---|
| **Run record** — the whole of one turn, unchanged except that one member leaves it | `internal/loop` | the **turn** | the stored node body **and** the response body |
| **Write receipt** — where the record was filed, in the vocabulary of §8.2 | `internal/loop` | the **filing** | the response body **only** |

**The receipt is not a field of the record and this is the document's central modelling decision.** Run
the consumer-chain recursion #1136 §2 requires, on the stored copy:

- Does anything read `written` in a stored record? The corpus reader is milestone 2. In a stored record
  the field can only ever say *"this node"* — a fact its reader established by opening the node — or
  carry the `{}` that is the defect. **The chain dead-ends.**
- Does anything read it in the response? Yes: the caller, who is holding the other copy and needs to know
  whether a second one exists. **The chain terminates in a named reader.**

A member whose value is information in one destination and noise in the other is not one member. **The
radical-clean shape (#1136 §4) is to take it out of the type**, and the compromise shape — leave it in
and suppress it when empty — is the one that keeps the trap: the next field added to the record diverges
silently in exactly the same way, and nothing in the type says which fields are corpus-scoped. Round 8's
rule again: **structural, not remembered.**

**Not modelled:** a per-leg failure reason, a retry count, a duration, an orphan register.

---

## 8. Contracts & Interfaces (Abstract)

### 8.1 The two artifacts — what replaces "the response body *is* the record"

**The claim does not survive and is not repaired.** #10899 lists a fourth request and a self-reference
marker as ways to make it literally true. Both are rejected here:

- **A fourth request** buys byte-identity by adding a failure mode to the very sequence whose failure
  modes are §6.2's subject — and it can itself fail, at which point the stored copy carries a *stale*
  fate, which is worse than an absent one. It also fails on its own terms: it cannot make the two copies
  identical at the moment either is read, only eventually.
- **A self-reference marker** spends a request, or a special value, to tell a reader something they
  established by opening the node.

**What is guaranteed instead**, and it is checkable in a way byte-identity never was:

> **The stored node's body and the HTTP response body carry the same record, byte-for-byte, in every key
> that describes the run. The response body carries exactly one key more: the write receipt. The stored
> body is the response body minus that one key, and nothing else differs.**

Two properties follow, and both are assertions rather than prose:

| Property | How it is checked |
|---|---|
| The stored body is the response body minus one key | One test: run a turn against doubles, take both byte sequences, remove the receipt key from the response, compare. **A new field added to either side without a decision reddens it** |
| The stored body carries no undefined value for a fate it cannot know | The receipt key is **absent**, not empty. Absence is honestly *"not applicable here"*; `{}` was not |

**The human reading `curl` output sees no change**: the receipt sits beside the record's own keys at the
top level of the response, so the body still reads as one flat object exactly as it does today. What
changed is which type owns the key.

### 8.2 The write receipt

| Aspect | Contract |
|---|---|
| **Home** | The HTTP response body only. Never the stored node. Never the record type |
| **Shape** | A state from the closed set below, and the node id when there is one |
| **Vocabulary** | Closed, three values, each with a producer in §6.2 and a reader today |

| Value | Means | Node id | The caller's decision |
|---|---|---|---|
| `stored` | The record is at node N, bodied and linked to the subject | present | none — the ordinary outcome |
| `unlinked` | The record is at node N and complete; the edge to the subject is missing | present | the record is safe; the edge is repairable, and the operator's log names it |
| `notStored` | No node holds this record. **The response is the only copy** | absent | keep the body if it matters |

**No reason string, and this is deliberate.** #10532 §8.5 rules that caller-facing text carries no
upstream body and no address; and every `notStored` cause — a failed create, a failed body write —
produces the **same** caller decision. A reason on the receipt would be a field whose only reader is a
human who would then have to look in the log anyway. **The diagnosis is the operator's and goes to
stderr** (§8.6). The current implementation's generic `"write-back failed"` string is therefore not
migrated; it is **dropped**, because `notStored` already says it and says it in a closed vocabulary.

**Why the state is named rather than derived from the node id's presence.** #10532 §8.2 settled this
exact question for `capReached`: *"a fact this record promises is not delivered by a derivation the
reader cannot perform."* A reader deriving `unlinked` from *"there is an id but something else is off"*
cannot, because nothing else is there to be off. The state is the fact; it is named.

**Not declared** (#1220 §2, in prose): `cancelled`, `deferred`, `queued`, `supersededByRetry`,
`writtenToFallback`. None has a producer in this design.

### 8.3 The graph port's write operation

The port's shape is unchanged — *the loop hands over what happened; the adapter alone decides type, name
and edge* (#10532 §8.3). What is added is that its outcome is no longer "an id or an error":

| Aspect | Contract |
|---|---|
| **In** | a run record. Unchanged |
| **Out** | one of §8.2's three states, and the node id where one exists |
| **Invariant** | The adapter **never returns `stored` unless all three calls landed.** `unlinked` is returned only when the body was written and the edge was not |
| **Invariant** | The adapter **discards only a node it created in this call**, and only on the body-write failure branch |
| **Invariant** | A failed discard changes the operator's log and **not** the state reported (§6.2) |
| **Invariant** | The operation's own errors — URLs, upstream bodies — reach the **log only**, never the receipt (#10532 §8.5, §10.2) |

**One consequence for the loop:** its two current outcomes (`nodeID` / `error`) collapse into one
three-valued outcome, so the branch that today chooses between two record fields disappears. That is a
simplification, not an addition.

### 8.4 Constants, not configuration (#1136 §3)

Two numbers, both constants, neither a knob. **Neither is a "telemetry-then-tune" compound** — there is
no audit column and no config member, so #1136 §3's filed-task requirement does not bind.

| Constant | Value | Derivation, and why it is not a knob |
|---|---|---|
| **The run bound** | **10 minutes** | It bounds everything from the handler's entry up to and including the answer. **Stated, not derived from the parts** (§6.3): the composed worst case is ≈ 16 min (C48), which no caller would wait for, and a bound assembled from three component timeouts moves silently whenever one of them does. 10 min is sized against the deployment the provider ruling exists to enable — a slow local model, where one call can take minutes (#10532 §8.4a) and the cap allows three — and against a caller who is a human with `curl` and can interrupt. **It differs by no environment**: a fast hosted endpoint never approaches it and a slow local one is exactly what it is sized for. **Falsifier, and it is cheap: §8.6's wall clock.** Ten real runs against a local runtime either sit far below it or push it, and the number moves with data in hand rather than judgement |
| **The drain grace** | **11 minutes** = **run bound + 3 × the graph client's per-call timeout (45 s) + 15 s of stated headroom**. ~~asserted against `run bound + 4 × the graph client's per-call timeout`~~ | It stops being an independently chosen number (§6.3). A run in flight at `SIGTERM` has at most the run bound left, plus its write-back. **CORRECTED 2026-09-02 (QA #10910 W-2): the write-back's maximum is three graph calls, not four**, and the two halves of this number are now labelled because they are not the same kind of number. ~~a write-back of at most four graph calls at 15 s each (three POSTs and the possible discard) — 60 s~~ over-counted: the discard is reachable **only** on the branch where the body write failed, and on that branch the link call is never issued, so no path issues four (§8.4a). The derivation is therefore **45 s**; the remaining **15 s is margin, chosen so the grace does not sit exactly on its own bound**, and it is called margin rather than dressed as arithmetic. The literal is asserted against both parts so that moving either input without re-deriving this turns a test red — the same discipline #10532 §8.4 applies to its 100,000-byte ceiling |

**What raising the grace does *not* cost (C47, C51).** An idle shutdown still returns immediately. A
container under `docker stop`'s default is still killed 10 s after `SIGTERM`. **The process offers a
ceiling; the supervisor decides how much of it is honoured** — so the longer grace is opt-in at the
deployment, and the deployment note is §14's, not a code change.

**The per-call timeouts are untouched.** `divoid.DefaultTimeout` and `openaicompat.DefaultTimeout` bound
a hung socket and remain exactly as #10532 §8.4a set them. The run bound is a different instrument: it
bounds a **slow** run, not a hung call, and the two do not substitute for each other.

### 8.4a The write-back's maximum call count — enumerated, because §8.4 got it wrong once

**Added 2026-09-02 from QA #10910 W-2.** §8.4's first draft derived the grace from *four* graph calls.
The number is **three**, and the enumeration is recorded here rather than left implicit, because a
derivation whose count nobody can re-check is the thing that invited the error.

**How it was established:** every path through the write-back operation in the merged tree was walked and
its outbound graph calls counted. Each leg is one request — the adapter's send helper issues exactly one
round trip per call and there is no retry anywhere on the path (§10, unchanged).

| Path | Calls issued | Count | Receipt |
|---|---|---|---|
| The record will not serialise | none | **0** | `notStored` |
| create fails | create | **1** | `notStored` |
| body write fails, discard succeeds | create, body, **discard** | **3** | `notStored` |
| body write fails, discard also fails | create, body, **discard** (attempted) | **3** | `notStored` |
| link fails | create, body, link | **3** | `unlinked` |
| all three land | create, body, link | **3** | `stored` |

**The maximum is 3, and four is not reachable on any path.** The over-count added the discard on top of a
full three-call success — but the discard exists **only** on the body-write-failure branch, which returns
before the link is ever issued. The two are mutually exclusive by construction (§6.2), which is the same
early return that makes the keep-or-discard rule work at all.

**Why this is corrected rather than left as harmless slack.** It errs safe — the grace is larger than the
bound needs, never smaller — so nothing is broken today. That is exactly the reason to fix it: a number
carrying a **stated derivation** invites a later reader to re-derive it, and a reader re-deriving from the
same wrong count would "correct" the grace *downward* onto a bound it no longer clears. §8.4 now separates
the 45 s that is derived from the 15 s that is margin, so the two cannot be confused for each other.

### 8.5 The error envelope gains one code

#10532 §8.5 closed the envelope at four codes, each with a caller decision behind it. The run bound adds
a fifth because it adds a decision:

| Code | Status | Meaning | The caller's decision |
|---|---|---|---|
| `run_deadline_exceeded` | **504** | The run did not produce an answer within the service's ceiling | **Retrying unchanged will hit the same ceiling.** Something must change — a faster endpoint, a smaller subject, a different input |

**Why not fold it into `model_unavailable`.** The model was not unavailable; the service hung up. §6.5's
whole ethos is that a failure is named rather than approximated, and reporting our own bound as an
upstream failure sends the caller to fix the wrong thing.

**One ordering obligation, because it is the way this gets implemented wrongly:** a deadline expiry
reaches the handler *disguised* — it surfaces as whichever external call was in flight when the context
died, and the loop wraps that as a graph or model failure. **The handler must test the run context's own
expiry before it classifies the error**, or every deadline will be reported as a `502` naming an upstream
that was fine.

### 8.6 The per-run log pair, and the wall clock

#10532 §10.3 already specifies this and the tree does not have it (C49). It is restated here as a
deliverable because **it is the only surface T10 can appear on**:

| Record | When | Carries |
|---|---|---|
| **run started** | before the anchor read | the subject id and the input's **length**. Never the input text (#10532 §10.3) |
| **run finished** | after the write-back, on every path that reached the answer | the receipt state, the node id when there is one, the candidate and cut counts, the model call count, the model id, the usage summed over the calls that reported it or its absence, and — **new here** — the **wall clock** |

**Two more lines, both on failure branches:**

| Record | When | Carries |
|---|---|---|
| **repairable orphan** | on `unlinked` | the node id, so a human can add the edge |
| **uncollected shell** | when the compensating discard itself failed | the node id, so a human can remove it |

**Why the wall clock goes here and not on the record.** Its reader is the operator choosing §8.4's two
numbers, and stderr is the operator's channel — M0's partition, unchanged: *stderr carries operational
events; the graph carries content*. Milestone 2 scores retrieval, not latency. A duration on the record
would be a persisted member whose named 4-week decision (#1136 §2) is already served by a log line that
has to exist anyway. **Falsifier:** if milestone 2 turns out to need per-run latency in the corpus, the
field joins the record with that requirement in hand.

**The detection property this creates, stated plainly so nobody has to rediscover it:** a `run started`
with no matching `run finished` is T10, and it is the only trace T10 leaves.

---

## 9. What Is Deterministic and Pinnable

Everything this document adds is on the deterministic side of #10532 §9.1's boundary. Nondeterminism
still enters at exactly one line — the model's reply — and none of the following is downstream of it in
any way that matters.

**CORRECTED 2026-09-02 from QA #10918 W-6 — the table below now names guards, and the run-bound row was
false.** The first draft of this table described, for each addition, the *mechanism* by which it would be
covered. That is a claim, and one of the eight was wrong in a way nobody could see from the table: the
run-bound row described coverage by **the caller supplying its own clock**, which cannot observe whether
the handler applies a bound at all — Go propagates a parent context's expiry into the child, so a handler
that applied no ceiling would satisfy every test the row described. The same defect had already been
raised against the implementation and was its critical fail; §9 asserted the coverage that was missing,
which is why it survived a full review cycle. **The rule that follows, and it is this document's own
subject applied to itself: a coverage row names the guard, not the mechanism.** A named guard is an
artifact a reader can open; a described mechanism is a claim they have to re-derive. Round 8, on this
document's own table.

| Addition | Deterministic? | The guard that pins it |
|---|---|---|
| The receipt's three states | **Yes, given the write outcome** | Port level: `TestTurnRunReportsEachWriteStateVerbatimAndInterpretsNone`. Adapter side, one per state: `TestWriteRunReportsStoredWithTheNodeIDWhenAllThreeCallsLand`, `TestWriteRunReportsNotStoredAndAttemptsNothingFurtherWhenTheCreateFails`, `TestWriteRunKeepsTheCompleteRecordAndReportsUnlinkedWhenTheLinkFails` |
| The discard branch | **Yes, given which leg failed** | `TestWriteRunDiscardsTheBodylessShellAndReportsNotStoredWhenTheContentWriteFails`, and the two that make it safe rather than merely present: `TestWriteRunDiscardsOnlyTheNodeItsOwnCreateReturned` and `TestWriteRunStillReportsNotStoredWhenTheDiscardItselfFails` |
| Keep-on-link-failure | **Yes** | `TestWriteRunKeepsTheCompleteRecordAndReportsUnlinkedWhenTheLinkFails`, with `TestWriteRunLogsTheRepairableOrphanWithItsNodeIDOnlyWhenTheLinkFailed` pinning that the operator is told |
| The two artifacts' relationship (§8.1) | **Yes** | `TestTheStoredBodyIsTheResponseBodyMinusTheWriteReceiptAndNothingElse` — the assertion that replaces the withdrawn identity claim — with `TestTheStoredBodyCarriesNoWriteReceiptWhileTheResponseDoes`, `TestWriteRunStoredBodyCarriesNoWriteReceiptKey` and `TestTurnRunRecordSerialisesWithNoKeyAboutItsOwnFiling` |
| **The run bound exists at all** — a run the caller never bounded is still ceiled, and the ceiling stops at the answer | **Yes** | **`TestRunsCeilsARunTheCallerNeverBoundedAtTenMinutes`** — the request carries no deadline, so a deadline observed inside the turn can only have come from the handler; it asserts one is there and that its remaining time is in range. **`TestRunsCeilingStopsAtTheAnswerAndDoesNotReachTheWriteBack`** — the write-back's context carries no deadline, which is §8.4's *"up to and including the answer"* and §6.4's detachment, in one assertion |
| The run bound's expiry, **classified** | **Yes** | `TestRunsReturns504WhenTheRunContextExpiresBeforeAnAnswerExists`, `TestRunsDoesNotReport502ForAnExpiredRunEvenThoughAGraphCallCarriedTheExpiry` and `TestRunsReports502NotTheDeadlineCodeWhenTheCallerMerelyDisconnects`. **These pin the classification and nothing more.** They supply their own deadline, so they cannot show that a ceiling exists — the row above is what does, and the two rows are separate because conflating them is exactly the error this section is correcting. The fifth code itself (§8.5) is held closed by `TestTheErrorEnvelopeCanEmitExactlyFiveCodes` and `TestEachErrorCodeConstantCarriesItsWireValue` |
| **The write-back outliving the request** (§6.4's invariant) | **Yes** | `TestTurnRunFilesTheRecordAfterTheRequestContextIsCancelled`, with `TestTurnRunDoesNotFailTheRunWhenNoNodeHoldsTheRecord`. **Added 2026-09-02: the first draft's table had no row for this at all**, although §14 step 5 named it a deliverable — an omission of the same kind as the row above it |
| The drain | **Yes** | `TestSIGTERMDrainsAnInFlightRunInsteadOfDroppingIt` — signal during an in-flight run, assert the response still arrives and the ordered shutdown records still appear |
| The grace's arithmetic | **Yes** | `TestTheDrainGraceIsElevenMinutesDerivedFromTheRunBoundTheWriteBackAndAStatedMargin` and `TestTheDrainGraceKeepsAPositiveMarginOverTheBoundItMustCover`, literal on the expected side per #10466 — the two parts §8.4 separates are separated in the assertion too, the second asserting the margin is positive and that the grace clears the bound rather than sitting on it. **CORRECTED 2026-09-02 from QA #10923 W-9: the second guard was cited under its pre-rename name, ~~`TestTheDrainGraceDoesNotSitExactlyOnTheBoundItMustCover`~~, which had ceased to resolve** |
| The log pair | **Yes** | `TestACompletedRunEmitsTheStartedAndFinishedPairInOrder` at process level; `TestTurnRunLogsRunStartedBeforeTheAnchorRead`, `TestTurnRunStartedRecordCarriesTheInputLengthAndNeverTheInputText`, `TestTurnRunFinishedRecordCarriesTheReceiptCountsAndWallClock` and — the one that makes T10's signature real — `TestTurnRunLogsNoRunFinishedWhenTheRunNeverReachedAnAnswer` |

**How to keep this table honest, since it has now been wrong once.** A row is only worth its space if the
named guard would **fail** on the design's own negation of that row. Where the discrimination is not
obvious from the guard's name, the row says what makes it discriminate — the caller-supplied-no-deadline
premise in the run-bound row is the example, and it is the fact the wrong row omitted. **Falsifier for
the whole section:** any row whose named guard passes on an implementation that does not have the
property the row claims.

**The trade this table makes, and the failure mode it bought — named 2026-09-02 after QA #10923 W-9.**
Named guards are checkable where the prose they replaced was not, and they acquire a way to rot that
prose did not have: **a rename in a file this document does not own silently falsifies a row, and the
falsification is invisible from here.** The check is mechanical — extract the backticked `Test…` names
from this file, **discard any inside a struck `~~…~~` span, which are historical rather than cited**, and
confirm each of the rest resolves to a `func Test…` in some `_test.go` — and it is **a
point-in-time result, not a property**, so it is re-run whenever this section is edited or trusted,
never reported once and relied on afterwards. That distinction is this document's own subject, and W-9
is the second time it has been the answer.

**A note on numbering, so this is not folded into the wrong set.** §16's `A1`–`A4` are amendments to
**#10532**. This correction is to *this* document and is deliberately **not** `A5` — the A-set counts
corrections to the predecessor, and a set that also counted self-corrections would stop answering the
question it exists to answer.

**The default suite stays fully offline and hermetic.** Nothing here opens a socket to anything but a
local test server, needs a credential, or needs a model runtime — #10532 §9.3's property is preserved and
is one of this document's constraints, not an outcome.

---

## 10. Cross-Cutting Concerns

**Security.** Unchanged and re-checked. The receipt carries a node id and a state — no key, no URL, no
upstream body. The two new log lines carry node ids. The `run started` line carries the input's **length
only**, per #10532 §10.3, and this document does not relax it.

**Is the shell a leak?** #10863 calls the orphan's name *"the leak surface"* because it carries 80 runes
of user input. **On the evidence, it is litter rather than a leak**: the successful path publishes the
same 80 runes in the same node name, so the shell exposes nothing new. What it *is* — and what makes
discarding it right — is a bodyless, high-scoring recall candidate whose name is a copy of an input and
whose body answers nothing (§6.2).

**Observability.** §8.6. The partition is M0's and does not move: the log gains the fates the record
cannot carry and nothing else.

**Error handling.** #10532 §10.4's shape carries: errors travel up as values and are decided at the
boundary that owns the outcome. The one new obligation is §8.5's ordering.

**Idempotency and concurrency.** Unchanged. A repeated `POST /runs` runs again. The detached write-back
introduces no shared state — each run's write is its own value on its own context. **Falsifier for the
concurrency claim, unchanged from #10532:** any package-level mutable variable in `internal/loop`.

**Retries.** Still none, on any leg, including the write-back's three calls and the discard. #10532
§10.5's surviving argument — three constants with no measurement behind any of them — binds here
identically, and the discard's best-effort single attempt is the same rule, not an exception to it.

---

## 11. Risks & Mitigations

| # | Risk | Mitigation |
|---|---|---|
| **R1** | **The run bound is a judgement, not a measurement** (C50). Too small and it cuts legitimate slow local runs — the deployment the provider ruling exists to enable | The asymmetry is stated and the number is sized to the tolerant side (§8.4). §8.6's wall clock is the instrument, and ten runs move it with data. **Falsifier:** any `504` on a run that would have answered |
| **R2** | **A longer grace looks like a slower service** to whoever reads the constant without reading C47 | The number is derived and the derivation is in §8.4, not in a comment (#10861). The measured fact that an idle shutdown is unaffected is C47, in this document, where a reader of the constant is sent |
| **R3** | **The drain guarantee is conditional on the supervisor** (C51). Under `docker stop`'s 10 s default the process is killed long before the grace, and T10 returns | Stated rather than assumed, and it is a deployment flag rather than a code change (§14). **This is the honest limit of the fix, and no option available here removes it** — §6.3 shows the rejected alternative does not either |
| **R4** | **The unlinked record is invisible from its own content** and a walk from the subject silently misses it | By design (§6.2, §13.1). Detection is a graph query; the operator is told at the moment it happens (§8.6); and milestone 2 does not walk |
| **R5** | **The discard removes a node someone wanted** | It runs only on the branch where the node has no body, and only against an id this run created. Both are stated as port invariants (§8.3) and both are pinned (§9) |
| **R6** | **The receipt moving out of the record breaks a consumer** | There is one consumer — a human with `curl` — and the response's byte shape is unchanged at the top level (§8.1). Milestone 2 reads stored bodies, which never carried a usable value in this field |
| **R7** | **T10 is still reachable** and this document does not close it | Named as the one unrecordable fate (§6.1) with the only trace it can leave made to exist (§8.6). A design that claimed to close it would be claiming an artifact can describe its own non-existence |
| **R8** | **The detached write-back files records for runs nobody is waiting for** (T9), adding rows to a corpus already flagged for volume (#10532 §11 R5) | Deliberate: the run happened and its cost was paid, so the record is evidence rather than noise. **Falsifier:** if abandoned runs turn out to be a material share of the corpus, the detachment goes and the invariant weakens (§6.4) |

---

## 12. Migration / Rollout Strategy

Nothing to migrate; no schema, no stored shape that must be read back. **Records already written keep
their `{}`** — two of them exist (#10897, #10898), both from the live-turn task, and both are
interpretable because #10883 documents exactly what they show. They are not rewritten: the corpus's
honesty is served better by two rows whose defect is documented than by two rows quietly edited.

**The seams this must not foreclose:**

| Later need | Left open by |
|---|---|
| A `cancelled` fate, if a non-human producer ever drives runs | The receipt being a named closed set with one owner. A fourth value is a value, not a redesign |
| An outbox or a retrying writer | The write port's outcome already being a *state* rather than an id-or-error, so a durable writer reports into the same vocabulary |
| Per-run latency in the corpus | The wall clock already being computed at the finish line; promoting it to a record field is a field, not a mechanism (§8.6) |
| A graph-hygiene sweep | The two orphan states being named and logged with their ids |

---

## 13. Open Questions

### 13.1 One part of #10899 is not a defect, and it is recorded as such

Toni asked for this outcome explicitly if it arose. It did, in one place:

> *"a record stored by a partially failed write-back is byte-identical in this field to a fully
> successful one. The orphan is undetectable from its own content."*

**True, and not a defect.** A stored body cannot carry a fact it learns after it is sealed, and making it
do so requires the fourth request §2.2 rejects. Detecting an unlinked node is a property of the **graph**
— one query for run-record nodes with no edges — and DiVoid's own conventions already treat an orphan as
findable by search even when it is unwalkable. **The design's obligation is to tell the operator at the
moment it happens (§8.6) and to keep the record whole (§6.2), not to make a body self-aware.**

The rest of #10899 stands: `{}` is a third state the contract does not define, it is emitted in every
stored record, and the claim it violates is one the design makes twice.

### 13.2 Should the two existing `{}` records be left alone? — for Toni, low stakes

§12 says yes, on the ground that a documented defect is more honest than a quiet edit. If Toni prefers a
clean corpus, it is two `PATCH`es and a note. **Recommendation: leave them.** They are the evidence for
this design.

### 13.3 Is 10 minutes the right ceiling? — for Toni, and it is a number rather than a preference

§8.4 derives it from tolerance rather than from arithmetic, because the arithmetic gives 16 minutes and
no measurement exists (C50). The first ten runs with §8.6's wall clock answer it. **Nothing else in this
document moves if the number changes** — the grace is derived from it and follows automatically.

### 13.4 Should an orphan sweep be filed? — for Toni

§2.2 excludes it. There is at most a handful of orphans and no evidence any exists. If Toni wants the
graph checked, it is a one-query task rather than a component.

---

## 14. Implementation Guidance for the Next Agent

No code appears in this document by design. The order below is architectural.

**Before anything: the comment contract.** #114 §4 binds Go on this repo via **#10861** — **read them at
the nodes.** Every derivation in this document (§8.4's arithmetic, §6.2's keep-or-discard rule, §6.3's
argument, §8.5's ordering obligation) belongs **here**, not above the code that implements it. #10532
§10.3's own closing paragraph is the precedent: when a rationale is load-bearing, it is relocated to the
design, not preserved as a comment.

1. **The receipt type and its vocabulary** (§8.2), declared beside the record. Three values. **The record
   type loses its write member and the generic reason string is dropped, not migrated.**
2. **The response body** (§8.1). The receipt sits beside the record's own keys at the top level, so the
   body's shape is unchanged for a human reader. **The deliverable assertion of this step is the
   relationship test**: take both byte sequences, remove the receipt key from the response, compare. Pin
   it before anything else depends on it.
3. **The write port's outcome** (§8.3): one three-valued outcome replacing id-or-error. Pin at the port
   level that the loop reports what the adapter returned and interprets nothing.
4. **The write sequence's partial policy** (§6.2), at the wire level, one test per leg: create fails →
   `notStored`, no discard attempted; body write fails → `notStored` **and a discard for that id and no
   other**; link fails → `unlinked` **and no discard**; discard fails → still `notStored`, and the
   uncollected-shell line on the log. **The order — content before link — is now load-bearing; pin that
   it is not reordered.**
5. **The detached write-back** (§6.4). Pin the invariant, not the mechanism: a run whose request context
   is cancelled after the answer still files its record.
6. **The run bound** (§8.4), applied at the handler over the request's context. Then **§8.5's ordering**:
   the handler tests the run context's expiry **before** classifying the error. Pin `504` with the new
   code, and pin the mutation — make the handler classify by error type first and confirm the test
   reddens with a `502`.
7. **The grace** (§8.4), as a literal asserted against `run bound + 3 × the graph client's per-call
   timeout + 15 s of headroom`, **literal on the expected side** (#10466). **The count is three, not
   four** (§8.4a), and the headroom is named separately in the assertion so a later reader cannot
   re-derive it away as a miscount. Note that `internal/server` must not reach into the graph adapter
   for that input — the derivation lives in §8.4, the constant is a literal, and the assertion is what
   ties them.
8. **The drain**, at the process level: signal during an in-flight run and assert the response still
   arrives and the ordered shutdown records still appear. This is the test #10890 names as absent.
9. **The log pair and the two orphan lines** (§8.6). Pin that `run started` carries a **length** and never
   the input text, and that `run finished` carries the wall clock.
10. **Mutate and watch it redden**, same discipline as #10532 §14 steps 5 and 13. In particular: put the
    receipt back inside the record and confirm the relationship test fails; make the discard run on the
    link-failure branch and confirm a test fails; report a deadline expiry as `502` and confirm a test
    fails; drop the `run started` line and confirm the process-level test fails.
11. **README**: the new code in the envelope, the two constants and — the operational half, which is the
    easiest thing here to omit — that **the drain guarantee requires the supervisor's kill timeout to be
    at least the grace** (C51), so a container that is expected to finish its run needs `docker stop -t`
    set accordingly.
12. **§16's amendment to `docs/architecture/m1-skeleton-loop.md`**, in the same change, with the DiVoid
    node kept byte-identical to the file.

### Do not add

A fourth write request · a self-reference marker in the stored record · a retry or backoff on any write
leg · a reason string on the receipt · a per-leg failure code · a duration field on the record · an
outbox, a queue or a background writer · a cancellation record or a `cancelled` receipt value · a
configuration member for either constant · a `WriteTimeout` or `IdleTimeout` on the server (nothing here
needs one and a `WriteTimeout` would cut exactly the long runs this design protects) · a compensating
delete on the link-failure branch · a sweep for existing orphans · anything from §2.2's prose list.

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

| Item | Answer |
|---|---|
| No new type mirroring an existing one | ✓ One new type, the receipt, and it exists precisely **because** folding it into the existing one is the defect (§7) |
| No abstraction with one implementation and no concrete second | ✓ No new interface. The write port's outcome changes shape; the port does not multiply |
| No element justified by "we might need X later" | ✓ Each of the three receipt values has a producer in §6.2 and a reader in §8.2. The fourth-value candidates are prose (§8.2), which is #1220 §2's own remedy and the specific trap the brief named |
| No deprecation period, feature flag, compatibility shim, transition window | ✓ None. The two existing records are left as evidence (§12), which is a decision about data, not a shim in code |
| `block_size × site_count` quoted for any inline-at-N-sites decision | **Not applicable.** No multi-line block recurs at more than one site. The discard is one branch in one adapter; the deadline is one wrap at one handler |

**Existing systems first**

| Item | Answer |
|---|---|
| Audited whether an existing surface covers this | ✓ Three existing surfaces (stored body, response, stderr) are given scopes; **nothing new is built**. The drain is `Shutdown`'s existing behaviour with a re-derived number, and the log pair is #10532 §10.3's own unimplemented specification rather than a new channel |
| Concrete reason a new layer can't live on the existing surface | ✓ §7 recurses the consumer chain and finds the stored copy's chain dead-ends. That is the reason the receipt leaves the record, and it is the only structural change |
| Concrete 4-week decision for any new persisted data point | ✓ **No new persisted data point exists** — the stored record loses a member and gains none. The wall clock is a log attribute, not persisted, and §8.6 names its reader and its falsifier |
| Consumer chain recursed for anything justified by "an existing reader projects it" | ✓ Applied to the stored `written` (§7) and to a would-be duration field (§8.6). Both dead-end; both drop |

**Configurability**

| Item | Answer |
|---|---|
| Every knob has a named operator or an environment difference | ✓ **No knob is added.** Both numbers are constants; §8.4 states why neither differs by environment |
| Every "telemetry-then-tune" knob has a filed task naming the reader and the event | ✓ None exists. §8.4's falsifier is a measurement that moves a **constant**, with no config surface and no audit column — the compound #1136 §3 warns about is absent in both halves |
| Magic numbers that need not vary stay named constants | ✓ Both, with the grace additionally asserted against its own arithmetic |

**Less is better**

| Item | Answer |
|---|---|
| Every element passed delete / merge / inline | ✓ Recorded, not asserted: the compensating delete **failed** the delete test on the shell branch and **passed** it on the link branch, so it exists on one and not the other (§6.2); the receipt's reason string was **deleted** (§8.2); a duration field was **deleted** (§8.6); a per-leg code was **deleted**; the derived-from-parts run bound was **deleted** in favour of a stated one (§6.3); the detached write-back **survived** its delete test on a stated invariant (§6.4) |
| Trade-offs named where a more complex design wins | ✓ Four: the receipt leaving the record over `omitempty` (§7 — the compromise keeps the trap); drain over cancel-and-record (§6.3 — argued against the brief's own position, on C47); keep-on-unlinked over compensate (§6.2); the detached write-back over none (§6.4, with the asymmetry against §6.3 stated rather than glossed) |
| Radical-clean shape chosen where the existing surface has no consumer | ✓ The stored `written` has no consumer, and the shape chosen is **removal from the type**, not suppression in serialisation (§7) |
| Reader inventories cover AST *and* string-literal references | Applicable and stated in §14: the receipt's field name and the dropped reason string are both string-literal surfaces (JSON keys, log attributes) as well as identifiers, and step 2's relationship test is what catches a missed one |
| Carrier-swap tables enumerate every affected DTO | ✓ One carrier changes: the write member moves from the record to the response. There is one other artifact (the stored body) and §8.1 states what it does — **nothing, by absence** |

**Data deliverables** — not applicable. No SQL, no migration, no backfill.

**Document discipline**

| Item | Answer |
|---|---|
| Cites #114 and #1136 as load-bearing | ✓ Header, and §14 opens with the comment contract because this design constrains what the implementer may write |
| Reader / scope inventories explicit | ✓ §2.1, §2.2 (with reasons), §5's not-a-component table, §14's do-not-add list |
| Out-of-scope listed explicitly | ✓ §2.2, as a table, plus #1220 §2's prose future list in §2.2 and §8.2 |
| No multi-paragraph rationale for things that obviously stay | ✓ The per-call timeouts get two sentences (§8.4); the unchanged §6.5 rows get five table rows and no prose |
| Predecessor design marked superseded | **Applicable in part and handled in §16.** #10532 is **not** superseded end-to-end and must not be banner-marked as if it were; four of its paragraphs are overridden (A1–A4, §16; A4 added 2026-09-02 from QA #10910 W-6) and each gets a correction in place, in the same change |

**Falsifiable-universals check (#1220 §5).** Every universal here names what would break it:

| Claim | Falsifier |
|---|---|
| *"The stored body is the response body minus exactly one key"* (§8.1) | Any other difference between the two byte sequences. Asserted, not claimed |
| *"Once the model has answered, the record exists and is filed"* (§6.4) | Any path on which the model answered and neither artifact exists, other than T10 |
| *"Every terminal state is named by some surface"* (§6.1) | A run outcome that fits no row of §6.1's table |
| *"The adapter never reports `stored` unless all three calls landed"* (§8.3) | A `stored` receipt on a run whose link call failed |
| *"The discard only ever removes a node this run created"* (§6.2, §8.3) | A `DELETE` for any id not returned by this run's own create |
| *"The receipt's vocabulary is closed at three"* (§8.2) | A fourth value with a producer. The prose list is not a declaration |
| *"The grace always covers the drain"* (§8.4) | A run that outlives the grace **while the supervisor is honouring it** — which is the arithmetic being wrong, not C51's cap being hit |
| *"The default suite stays offline and hermetic"* (§9) | Any test here that opens a socket to a host other than a local test server |

**And one claim is deliberately bounded rather than universal:** the drain completes *provided the
supervisor honours the grace* (C51, §11 R3). Stating it as universal would be the failure mode #10532's
own §8.4 was corrected for — a bound expressed against an unstated quantity.

---

## 16. The Amendment to #10532 — four paragraphs, corrected in place

**#10532 is not superseded and must not be banner-marked.** It is M1's design, M1 shipped, and it is
consumed here unchanged everywhere except four places where a reader would now be relying on something
false. Those four get a correction **in place**, in the same change as this document, following
#10532's own established convention — **strike, do not delete; record what was believed and when** — and
the DiVoid node is kept byte-identical to the repo file.

| # | Where | What is wrong | The correction |
|---|---|---|---|
| **A1** | **§8.4a**, the paragraph beginning *"One interaction is named and deliberately left alone"* | Its mechanism claim is false: *"raising the grace to cover a worst-case local generation would make every shutdown hang for minutes"*. `Shutdown` returns as soon as connections are idle (C47), so an idle shutdown is unaffected at any grace. **Its conclusion also reverses**: M1 does not raise the grace; this design does | Strike the mechanism sentence, keep the paragraph, and record that its own stated falsifier — *"the day a run is expensive… the trade reverses"* — was met by a different route than it predicted: not by the run becoming expensive, but by the mechanism claim being wrong. Point at this document |
| **A2** | **§8.2**, the `written` row | It says the field carries *"the node id the record was written to, or the reason it was not"*, in a table describing the record. In the stored record it carries neither and structurally cannot (C45, C46) | Strike the row and replace it with a pointer: the write receipt is **not** a member of the record, and §8.1 of this document states what the two artifacts guarantee instead |
| **A3** | **§6.5**, the *"Write-back fails"* row, and the **TL;DR** and §7 sentences asserting *"the response body **is** the record"* | The row specifies the fully-failed case only and is silent on the partial one (#10863); the identity claim is false in one key (#10899) | Amend the row to point at §6.1's complete table, and amend the two identity sentences to the minus-one-key claim, marked as a correction with its date and the finding that produced it |
| **A4** | **§14, step 11a's pin checklist** (the record-shape pin list) | It still names `written` among the fields a wire-level decode struct must assert. That member no longer exists, so the line **directs a future implementer to pin something that must not exist**. **Added 2026-09-02 from QA #10910 W-6** | Strike `written` from the list and point at §8.1's replacement assertion. **§8.2's sentence naming the fields unit A's record lacks is deliberately left standing** — it is a true statement about what unit A shipped, and this document corrects claims that are false now, not history that was true then |

**Why this shape and not a revision 4.** #10532's revisions were made while M1 was in flight, and each
one moved a decision the milestone had not yet shipped. M1 is closed and verified live (#10883). A
revision 4 would rewrite a closed milestone's history to contain a decision taken after it; four
correction pointers say honestly that the milestone shipped, was run, and taught us what it taught. **The
same reasoning #10532 applies to its own struck paragraphs applies to it.**

**A4 is what this shape is for, and it arrived a day later.** It was not a paragraph this document
overlooked — it became false only once the receipt left the record, which is a change *this* document
makes. A banner-marked predecessor would have hidden it; four dated pointers make it one more row.
**The count is expected to keep moving**, and whatever states it — this heading, the header line, and
§15's document-discipline row — moves with it. **It counts corrections to #10532 and nothing else:**
this document's own corrections (§8.4a, §9) are dated in place and are deliberately not numbered into
the A-set, because a set that counted both would stop answering the question it exists to answer.
