# Architectural Document: A Measured Run Files Nothing

> Repo path: `docs/architecture/a-measured-run-files-nothing.md`.
> Constraint ruled in DiVoid **#11071**, shape ruled in its `SHAPE RULED` section, **widened** by
> **#14561 §12.10** after QA found the first implementation closed one seam and left another open.
> Consumer: **#14571 §10**, the runner sequence around the six report-form probes.
> Standards applied: Design Contracts **#1136**, Code Contracts **#114 §0 and §4** (§4 via the Go annex
> **#10861** — cited, not restated), falsifier rule **V-13** (#14561).
> Baseline: `main` at **`52a1df4`**. Every fact in §2 was read out of that tree.

---

## TL;DR

*A run that is being measured must change nothing about the state it is measured against. That is a
property of the whole run, not of one write — and the first draft of it was a property of one write.*

**The constraint, as widened:**

> **No graph mutation of any kind may originate from a measured run** — record, substance, link, or
> anything a future port adds — from the opening of a measurement window to its close.

**The shape:** every mutating port the run holds is a **decorator** over the real one. Reads pass through
untouched. Writes are recorded and dropped, and the caller is told the success it would have been told.
Nothing in `internal/loop` changes, and there is no flag: the named caller is the `measure` binary, which
has no other mode.

**Two layers, and they are not redundant:**

| layer | role |
|---|---|
| a decorator per seam | the **mechanism** — each port gets a *modelled* not-written outcome, so the run behaves as the product behaves |
| zero non-GET requests at the transport | the **falsifier** — it needs nobody to enumerate the ports, and it is what catches the third mutating port written by someone who never read this document |

---

## 1. Problem Statement

`Turn.Run` calls `WriteRun` unconditionally. Every full-loop run files one `session-log` record of
50,000–110,000 B into the graph it reads from. An A/B of N tasks × 2 arms therefore writes 2N records into
the memory state the experiment exists to hold constant, and arm B runs against a graph containing arm A's.
Those records outrank real content for inputs resembling past runs and are larger than the whole assembly
budget, so the most likely outcome of a naive comparison is a reported configuration difference that is
purely an ordering artifact.

That is the defect the constraint was written against. It is **not** the whole of the defect, and §3
records how the rest was discovered rather than designed.

## 2. What the tree actually holds — measured at `52a1df4`

| fact | where | consequence |
|---|---|---|
| `loop.GraphPort` has **four** methods: `Node`, `Recall`, `Neighbours`, `WriteRun` | `internal/loop/turn.go` | the ruling says three; `Neighbours` was added after it was written. The argument is unaffected — this is still a second implementation of an existing seam, not a new abstraction |
| `WriteState` is a closed set of three, and `NotStored` is documented as *"no node holds the record"* | `internal/loop/types.go` | exactly true of a suppressed write, so nothing is added to a shipped closed set |
| `condense.GraphPort` has three methods, one of which writes: `SetSubstance` | `internal/condense/condense.go` | **the second mutating seam**, and it is not behind `loop.GraphPort` |
| the fill is built from a `*divoid.Client` in `package main`, not from the loop's graph port | `cmd/processor/main.go` | a decorator on the loop's seam cannot reach it |
| `systemText` and the model/fill wiring live in `package main` | `cmd/processor` | not importable by a second binary; see §5 |
| the form dial `SubstanceRatioThreshold` ships at **0** | `internal/loop/assemble.go` | no ratio is below it, so every candidate renders as content today |

## 3. The intra-run channel, verified rather than assumed

A first draft argued that the fill's write could not affect the block *within* a run, because the block is
assembled before the fill runs and judgement uses the already-assembled block. **That argument is wrong,
and the code says where.**

`Turn.Run` orders the run: `Assemble` → `t.fill` → `t.judge`. Inside `judge`, a `wantsRecall` round calls
`dispatchRecall`, which issues `Graph.Recall(…)` **unscoped over the whole graph** and passes the result to
`admit`. So:

1. the fill writes a substance to node *N*;
2. a later supplementary round recalls node *N* — the graph adapter's candidate projection includes
   `substance`, so the row comes back carrying the substance the fill just wrote;
3. `admit` records `SubstanceAvailable: c.Substance != ""` and `SubstanceSize: len(c.Substance)` on that
   round's disposition, and `renderedPayload` reads `c.Substance` when choosing the row's form.

**The channel is real, and it fires at two different strengths:**

- **Today, at the shipped dial:** `renderedPayload` selects `FormSubstance` only when the substance/content
  ratio is *below* the threshold, and the threshold is 0, so the rendered bytes do not change. But
  `SubstanceAvailable` and `SubstanceSize` **do** change, and they are recorded in the measured record
  itself, inside `ToolCallRecord.Results`. A measured run's own record would differ according to whether an
  earlier fill in the same run had written.
- **At any non-zero dial:** the form flips to `FormSubstance`, `RenderedSize` changes, and admission and the
  block bytes change with it.

**Consequence for the design.** The rejected alternative — *let the fill's write stand, because it cannot
reach this run's block* — was **unsound outright**, not merely fragile. It rests on a claim about statement
order that the supplementary-recall round falsifies, and it would have had to be re-verified every time the
loop's call order moved.

## 4. Decision — a decorator per seam, and the fill keeps running

### 4.1 The shape

Two decorators, each a second implementation of an interface that already exists:

- **`measure.Graph`** over `loop.GraphPort`. `Node`, `Recall`, `Neighbours` delegate. `WriteRun` records
  `{ordinal, subject, size}`, files nothing, returns `{State: NotStored}`.
- **`measure.CondenseGraph`** over `condense.GraphPort`. `NodeWithSubstance` and `Content` delegate.
  `SetSubstance` records `{ordinal, node, size}`, writes nothing, returns `nil`.

`measure.Runner` holds both and runs `loop.NewTurn(graph, …)` — the shipped `Turn`, unmodified.

### 4.2 Why the fill keeps running rather than being switched off

1. **Only this shape generalises.** The defect is not *the fill writes*; it is *a port built outside the
   decorator writes*. Refusing to boot with the fill configured has to be re-litigated for every future
   port, and each time the cheap answer is *turn it off* — eroding the measured configuration until it is
   no longer the product.
2. **A validity cost is not tradeable against a scheduling cost.** The discarded condense call costs the
   measurement nothing in block terms; it costs wall time on a contended endpoint. Switching the fill off
   trades the first for the second.
3. **Only this shape lets `Fills` appear in a measured record at all.** `MaxFills` bounds the cost at two
   condense calls per run, and the substances withheld are reported in the result.

### 4.3 Why `SetSubstance` returns success rather than an error

A write refused at the transport surfaces to the loop as an **error**, and the loop's error handling then
changes the measured behaviour: `condense.Run` would record `skipWriteFailed`, and the run's `Fills` would
report a failure that did not happen. Returning `nil` gives the fill the *modelled* not-written outcome —
everything it does except the byte reaching the graph — and the withheld bytes are reported separately in
`Result.Substances`.

### 4.4 The two layers

The per-seam decorators are the **mechanism**. The transport assertion — *a measured run issues zero
non-GET requests* — is the **falsifier**, and it is deliberately ignorant of how many seams exist. It is
what catches a third mutating port added later by someone who never reads this document, which is exactly
how `SetSubstance` was missed the first time.

## 5. What had to move, and why it is not scope creep

A measured run needs a caller. The caller cannot be the service, because a flag on `POST /runs` is the
permanently-off knob the ruling forbids. So it is a second binary — and Go's `main` is not importable, so
anything the shipped binary wires up privately has to move first:

| moved | to | why the measured arm needs it |
|---|---|---|
| `systemText` | `internal/systemtext.Text` | it is `Turn.System`; different system text is a second variable in the comparison |
| `newModel`, `newFillPort`, the fill adapters | `internal/ports` as `ports.Model`, `ports.Fill`, `ports.FillGraph` | the fill is part of the configuration being measured, and it must be **reachable** to be decorated. §3 is why *turning it off* is not the alternative it looks like |

Both are behaviour-preserving relocations; the system-text const body is byte-identical to its predecessor.

**The general form, worth keeping:** *a no-drift guarantee for a second caller is not free when the first
caller is a `main` package.* A ruling that promises "touches no shipped file" for a change introducing a
second entry point is promising something the language does not permit unless every shared piece already
lives under `internal/`.

## 6. Alternatives rejected

| alternative | why rejected |
|---|---|
| move `WriteRun` out of `Turn.Run` into its caller | defensible, but it changes `internal/loop`'s contract and splits the run's single finish log line, to buy a property the decorator gets for free. It becomes right only if a second production caller ever needs a different record fate |
| a suppression flag on the turn | a permanently-off knob with no named operator, and a second code path through the turn |
| defer the records and file them after every arm has run | adds exactly the content that pollutes the graph. A discard satisfies the constraint; a deferred write only moves it |
| refuse to boot when a condense model is configured | does not generalise past the fill, and every future port gets the same *turn it off* answer |
| let the fill's write stand | **unsound**, not merely fragile — see §3 |

## 7. Falsifiers

- **F-1.** A measured run issues zero non-GET requests to the graph, **with the fill on** — a condense model
  configured and a candidate over `FillSizeFloor` cut for want of room. *Falsified by* any non-GET request.
- **F-2.** No record reaches the graph before every turn of a comparison has completed. The fake graph
  records write **ordering**, not a count; the assertion is positional. *Falsified by* a write event before
  the last completion marker — which is what a defer-then-file implementation produces.
- **F-3.** A measured run is still a real run: its reads reach the graph unchanged, and the record it
  produces is byte-for-byte the record the undecorated `loop.NewTurn` produces on the same fixtures.
  *Falsified by* any divergence outside the run's own wall-clock field.
- **F-4 (V-13).** Every zero-valued assertion in this unit is shown capable of being non-zero: remove the
  marker it counts, the guard goes red, put it back. *Falsified by* a zero-assertion whose marker can be
  deleted with the suite staying green — which is how the fill's write stayed invisible.
- **F-5.** The size a suppressed write reports equals the byte length the graph would really have received,
  cross-checked against what `divoid.Client.WriteRun` posts. A record that cannot be encoded reports
  `UnsizedRecord`, never a zero indistinguishable from a real one.

## 8. What this does not settle

- **Concurrency.** `measure.Graph` and `measure.CondenseGraph` are safe for concurrent use; `measure.Runner`
  runs one turn at a time. Running two measured turns concurrently through one runner would interleave their
  reported windows. The sequence that consumes this is ordered, so this is documented rather than solved.
- **The residual fidelity gap.** Under suppression a later supplementary recall sees no substance where
  production would have seen one (§3). That is the *modelled* not-written outcome and is the intended cost
  of the constraint — but it is a difference from production, and it is named here rather than hidden.
- **Whether the measurement window is enforced anywhere but by convention.** Nothing in this unit prevents
  an unrelated task from running against the store while a window is open.
