# Architectural Document: What a Successful Run Withheld

> Repo path: `docs/architecture/what-a-successful-run-withheld.md` · DiVoid node **#13674**
> (the node carries this document verbatim — P-40 parity verified by sha256, §19).
> Task: **#13594** · Project: **#10422** · Product briefing: **#13534** (§13 is the test this design
> answers to).
> Evidence: five-run measurement **#13592** · run record **#13591**.
> Boundaries held, not crossed: **#13564** (a call that *fails*) · **#13601** (rows never *fetched*) ·
> **#11308** + `docs/architecture/substance-backed-admission.md` (the oversize node itself).
> Corrections consumed: **#13671** (the disclosure volume is **9**, not §16.2's ~12).
> **Baseline: `main` at `872156e`, working tree clean.** Every repo fact in this document was read out
> of that tree **except where a section dates itself otherwise** — §14a and §14c state their own refs.
> A citation here is a dated record, corrected in place with the old text struck, never silently
> renumbered to a later tree (P-43). #13601 Unit 1 is on **PR #70, open** — no claim here holds at that ref unless it says so.
> **Citation convention:** a bare `file.go:N` is under **`internal/loop/`**; every citation outside
> that package is written with its full path. **Node sizes carry `(record)` or `(live <date>)`** — §19.

---

## TL;DR

**What.** A successful run stops hiding that it dropped its best evidence. **Two units** — one
operator-facing, one model-facing — on paths that already exist.

**How.** (1) `turn.go`'s run-summary block gains **one WARN**, beside the existing shutout WARN, when
the **rank-1** candidate was cut for the byte budget — naming the node, its size, the budget that
remained. (2) `RenderToolResult` stops telling the model *"no additional results found."* when recall
**found** rows and admission cut every one.

**Cost.** Zero bytes of model budget, zero record fields, zero response fields, zero new constants,
zero new types.

**Rejected — warn on *any* byte-budget cut.** Falsified: run **#13598** cut **9** rows on size and
answered **correctly**. Cut count discriminates **0 of 3** runs; rank-1 admission, **3 of 3**.

**Not designed here.** The oversize node itself is open task **#11308**, design
`substance-backed-admission.md` Unit B — §5.1 argues that is the right owner, not a workaround.

---

## 1. Problem Statement

**The goal, in the operator's terms:** an operator who receives a fluent answer can tell whether the
loop read the documents that would have answered the question.

On 2026-09-11 the first real task through the container asked for the risks of a change this project
has a design document for. The graph ranked that document **first**, at the highest similarity in the
set — and admission cut it:

| rank | id | similarity | size **as recorded** | outcome |
|---|---|---|---|---|
| 1 | **#13585** | 0.7405 | 67,312 B | `cut: byte budget exceeded` |
| 3 | **#11235** | 0.7232 | 96,555 B | `cut: byte budget exceeded` |

Budget 60,000 B, anchor 3,408 B, **56,592 B remaining**, six rows admitted at 53,047 B. HTTP **200** in
21.6 s. The answer was confident, well-formed, correctly cited — and named three risks belonging to
other parts of the system (#13592 §1).

**Every node size in this document is tagged `(record)` or `(live 2026-09-11)`, and the two can
differ.** The sizes above are **`(record)`** — #13591's own frozen dispositions, which is the
instrument #13592 rules preferable precisely because *"a stored record is frozen at write time …
whereas the same query re-issued today returns something else."* #13585 has since grown: it is
**101,201 B `(live 2026-09-11)`**, its `lastUpdate` of `2026-09-10T23:08:57Z` falling **17 m 37 s
after** the `2026-09-10T22:51:20Z` the record stamps on the run. #11235 reads **96,555 B under both**.

**The tag is on every figure of this shape and not only the one under argument.** §5.1 refuses to move
a substance range between sets, and this rule is the same refusal applied to the figure nobody
challenged — which is where the first version of this document slipped (QA #13680 W-1).

**The load-bearing fact, verified in code rather than inferred from the answer text.** `renderBlock`
(`assemble.go:108`) writes one `ANCHOR` section and then one `CANDIDATE` section **per admitted row**
(`assemble.go:116`). Its parameters are `(anchor Anchor, admitted []Candidate)`, and **`Candidate`
(`types.go:14`) carries neither `Included` nor `CutReason`** — those live only on `Disposition`
(`types.go:62-63`), which goes to the record. **The renderer is structurally incapable of disclosing a
cut, because nothing it receives records one.**

So the model was not withholding a caveat. **It was never told**, and from inside the block the six
admitted rows are indistinguishable from *the six best things the graph holds*. A model reading a set
it believes complete, and reasoning well over it, produces exactly what this run produced. **The
confident tone is correct behaviour, not a flaw in the answer** — which is why any remedy aimed at the
wording of the answer is aimed at the wrong stage.

**Success criterion.** A run whose highest-ranked candidate was dropped for the byte budget says so, in
a place an operator meets without being told to look; and the model is never told something false
about what recall returned.

**The test this design is answerable to, quoted rather than paraphrased.** #13534 §10's standing
correction to §2:

> *"**if this ships, does the answer he gets get better — and would he be able to tell?** The second
> half is not decoration; this run scores badly on it and passes the first half."*

and §13's closing line, which asks of the three designed changes now on `main`:

> *"**which of three designed changes most improves the answer he gets, and whether he could tell.**"*

**This design answers the second clause and makes no claim on the first.** It does not make the answer
better — §5.2 shows it cannot, because the model has no tool that can reach a 67,312 B `(record)`
node, still less the 101,201 B `(live 2026-09-11)` it has since become. It makes the failure
**tellable**, which is the half #13591 scored badly on while passing the other.

---

## 2. What is already true at `872156e` — six surfaces, measured

**#13594's part B reads *"nothing tells anyone what was withheld. Neither the model nor the
operator."* That is half wrong, and the half that is wrong changes the size of this design.** Read out
of the tree at `872156e`:

| # | surface | discloses a cut? | where |
|---|---|---|---|
| 1 | HTTP response body, `candidates[]` | **yes — every row, with `cutReason`, at the wire level** | `internal/server/routes.go:45` embeds `loop.Record`; `types.go:63` tags the field `cutReason`. Nearest live assertion: `internal/server/routes_test.go:558`, on the tool-round copy of the same type |
| 2 | stderr `run finished` | **aggregate only** — `cut=N`, no reason split, no rank | `turn.go:168` |
| 3 | stderr WARN | **only in the shutout case** — every candidate cut | `turn.go:189` |
| 4 | run record node substance | **yes — fully, grouped by reason, with ids and bytes** | `summary.go:244`, written via `internal/divoid/write.go:68` |
| 5 | the model's block | **no — structurally incapable** | `assemble.go:108`, `:116` |
| 6 | the supplementary tool result | **no, and it states a falsehood** | `assemble.go:92` |

**The operator half is not a data gap. It is a salience gap.** Three of the six surfaces already carry
every disposition. What no surface carries is a *signal*: all three treat a cut rank-1 node exactly as
they treat a cut rank-20 node.

**Surface 1 is not the place to fix that, and the reason is measured.** The response embeds the whole
record including `Block` — 57,589 B on #13591 — so the body an operator receives runs to roughly
70–100 KB (#13592 records stored records at ~70 KB and 100,350 B). **A new key in a 70 KB JSON body is
not a signal; it is more data**, and it is exactly as invisible as `candidates[0].cutReason` unless the
reader already knows to ask for it. That is why §6 puts the signal in the log and not on the response.

**Surface 6 is a finding this task did not carry, and it is the sharpest one.**
`RenderToolResult` (`assemble.go:84`) returns the literal `"no additional results found."`
(`assemble.go:92`) whenever `len(r.Results) == 0`. `Results` holds only the **admitted** rows
(`turn.go:359-360`): `dispatchRecall` runs `admit(candidates, SupplementaryByteBudget)` at a 20,000 B
budget and puts the admitted set in `Results` while the full set goes to `Dispositions`. **So a
supplementary recall that fetched twenty rows and admitted none tells the model that nothing was
found.** That is not silence. It is an affirmative false statement on the model-facing path, in a
success case, and it is in exactly the class #13594 names.

---

## 3. Scope and Non-Scope

**In scope.** What a run that returned **200** tells the operator, and what it tells the model, about
candidates it fetched and did not use. Two sites: `turn.go`'s run-summary block, and
`RenderToolResult`'s empty branch.

**Out of scope, explicitly — with the interaction stated and then stopped.**

1. **#13564 — what a failed run reports.** Scoped to a call that *fails*. **Interaction:** it rewrites
   the three sites in `turn.go` that overwrite an upstream cause with a constant —
   `dispatchWrite`'s two `errFileWriteFailed` assignments and `dispatchRecall`'s
   `errSupplementaryRecallFailed` return — plus two sites in `routes.go`. This design edits `turn.go`'s
   `logFinished` (`:166-193`) and nothing in `routes.go`. Disjoint sites, same file, so they conflict
   textually and not semantically — §18 sequences them.
   **Cited by name rather than by line, and its lines re-resolved anyway:** #13564 names those sites at
   `0df5c14` as `turn.go:326`, `:339`, `:356`. Re-resolved at `872156e` **all three still land on the
   assignment they name** (`:326` and `:339` on `exchange.Error = errFileWriteFailed`, `:356` on the
   `errSupplementaryRecallFailed` return) — so #13564's citations have **not** drifted and an
   implementer may trust them at this ref. The names are used here regardless, because a name does not
   expire and a line number does. **Nothing in #13564's mechanism is re-specified here.**
2. **#13601 — the aperture spends slots admission refuses.** Its under-delivery WARN is about rows
   **never fetched**; this is about rows **fetched and cut**. That distinction is already a table in
   its §16.2 and is not restated. **Interaction:** #13601 Unit 1 adds a WARN to the same run-summary
   block, and its landing changes the population this design fires over (§15). **Nothing in #13601 is
   re-specified here.**
3. **The oversize node itself — #13594's part A.** Open task **#11308** owns it
   (*"Our own briefing documents are 66–138 KB and can never be admitted"*, filed 2026-09-04, status
   `open`). Its merged design is `docs/architecture/substance-backed-admission.md`, **Unit B**
   (two-pass admission: *"Pass 2 spends only the leftover budget, offering `substance` to candidates
   pass 1 already cut"*). §5.1 argues that is the correct owner. **This design specifies no admission
   change of any kind.**
4. **#13602, #13603, #13604, #13605, #13606, #13671.** Untouched.
5. **The answer's wording.** §1 establishes that a remedy aimed there is aimed at the wrong stage.
6. **Any change to `Record`, `Disposition`, `Candidate`, or `runResponse`.** §9 states why the data
   model is deliberately untouched.

---

## 4. Assumptions and Constraints

| | |
|---|---|
| **A1** | The operator reads the container's stderr. Established precedent: #13564's own success criterion is *"a reader with **only** the HTTP response and the container's stderr can name the remedy"*. |
| **A2** | A node's `name` is not a secret in this system. `RenderSummary` already writes candidate names into the run record's substance in DiVoid (`summary.go`, via `internal/divoid/write.go:68`). The WARN in §6 therefore introduces no new exposure class. #13564 §4's secrecy ruling governs *carried upstream causes*; it does not reach a node name this system already publishes. |
| **A3** | Retrieval is reproducible: 19 of 20 candidates identical across two runs of an identical input (#13592). This is the premise that makes any before/after attributable to the change rather than to noise. |
| **C1** | **Running the product mutates the corpus its own measurements are taken against** (#13592). Every run writes a record that joins the population later runs retrieve. §15's instrument is `cmd/eval`, which reads the graph and never writes to it — pinned by `TestSweepReadsTheGraphAndNeverWritesToIt` (`cmd/eval/sweep_test.go:369`), whose fake fatals on any `WriteRun` (`:77-78`). |
| **C2** | A figure read from the live graph carries its timestamp the way a code figure carries its ref. Every graph-derived number in this document is dated **2026-09-11**. |
| **C3** | The model's only tools are `recall` (query string) and `writeFile`. It **cannot fetch a node by id**, and `recall` re-runs `admit` at 20,000 B (`turn.go:359`). This bounds what disclosure to the model can buy — see §5.2. |

---

## 5. The two halves, answered

### 5.1 Q1 — is disclosure the right remedy, or is the budget the wrong instrument?

**The budget is not miscalibrated. The admission *granularity* is wrong, and that is a different
problem with a different owner.**

`admit` (`assemble.go:30`) has exactly two outcomes per row: the whole content, or a `CutReason`.
There is no degraded admission. So the unit of admission is a whole node, and some nodes are larger
than any block:

- **#13585 was 67,312 B `(record)` against a 60,000 B *total* budget**, and is **101,201 B
  `(live 2026-09-11)`**. It does not merely fail to fit beside the other rows; it exceeds the entire
  budget, anchor included. Admitting it needed ≥ 70,720 B on the day, and needs ≥ 104,609 B today.
  **The argument strengthens under the live reading and depends on neither** — which is why both are
  given rather than one chosen.
- **#11235 is 96,555 B `(record)` and 96,555 B `(live 2026-09-11)`** — unchanged, verified under both.
  Admitting it needs ≥ 100,000 B — for **one row**.
- **Measured on the eval corpus's own required set** (`internal/eval/corpus.json`, 25 unique required
  nodes, all **`(live 2026-09-11)`** — no record covers this set): min **1,503 B**, median
  **4,452 B**, mean **14,429 B**, max **200,749 B**. **One of 25 (#10926, a Processor design document) exceeds the
  56,592 B that remained on #13591** — and it can never be admitted at any budget this product would
  plausibly carry.

**A budget large enough to swallow one of these is not a budget.** So raising it is not a competent
alternative, and this design does not propose it.

**The granularity remedy already exists, is already designed, and is already merged.** `#11308` is
open; `substance-backed-admission.md` Unit B is its mechanism, and its own §15.1 names this class
directly: *"the only mechanism that touches #11308's class — a 195,448 B node becomes admissible for
the first time."* Its Unit A binary ships at `cmd/condense` and has **not been run** over the affected
nodes: **#13585, #11235 and #10926 all carry `substance: null` on 2026-09-11** (checked, not inherited
from #13594).

**So — is part B a workaround for part A? No, and the arithmetic says so.** On #13591 the leftover
after admission was **56,592 − 53,047 = 3,545 B**, which is precisely what Unit B's pass 2 spends.
*Whether a substance for #13585 would have fitted in it is unmeasured, and this document does not
assert that it would* — the 2.2–2.4 KB figure #13594 quotes is a property of **run-record**
substances, and no substance has ever been generated for a design document of this size. Moving that
range to a different set is the error #13592 records itself making. What can be said without moving a
figure is the shape: **the mechanism that would rescue #13585 spends exactly the budget that was left
over, and it is owned elsewhere.**

**The honest ordering, therefore:** A is blocked on an *operation* (`cmd/condense` over the oversize
design nodes) plus Unit B's implementation, both under #11308. **B is independent of all of it**, fires
on runs where A never will, and is the half that closes #13534 §13's second clause.

### 5.2 Q2 — what does telling the model cost?

**Named, not assumed, and split into a forced half and an elective half.**

**The forced half costs nothing and is not a trade-off.** `RenderToolResult` currently asserts
*"no additional results found."* when results were found and cut (§2 surface 6). **Correcting a false
statement is not a disclosure decision.** It spends no budget — the tool-result text already exists and
is already sent — and it needs no measurement to justify, because *"do not tell the model something
untrue"* has no opposing arm.

**The elective half is a manifest of cut rows inside the assembled block, and this design does not
ship it.** Three costs, of which two are measured and one is not:

| cost | status |
|---|---|
| **Budget.** A manifest line per cut row (`id / type / name / size / reason`) runs ~110 B. At **9** size-cuts on the ill-matched arm after #13601 Unit 1 (#13671, correcting §16.2's ~12) that is **~1 KB**. | **Estimated**, not measured. The per-line figure is an estimate and is labelled as one. |
| **Displacement.** A manifest charged to the budget must displace admitted content when the residue is smaller than it. | **Unmeasured on the arm that matters.** The 780 B residue #13601 measured is a property of **#13598's well-matched arm**; the 9 size-cuts are a property of **#13599's ill-matched arm**. Those are different sets and the figures do not combine. |
| **Behaviour.** A model told *"a more relevant document exists and you cannot see it"* may answer worse — hedging into uselessness, or spending a `recall` round it cannot win. | **Unmeasured in both directions.** |

**And the agency argument fails on this system's own tool surface, which is the decisive point.** A
model told that #13585 was dropped **cannot act on it**: `recall` takes a query string, not an id, and
re-runs `admit` at a 20,000 B supplementary budget (`turn.go:359`), so a 67,312 B `(record)` node —
101,201 B `(live 2026-09-11)` — is unreachable
by any tool the model has. **Disclosure to the model buys an honest answer, not a recoverable one.**
That is worth something — it is the difference between a confidently wrong answer and a hedged partial
one — but it is a smaller prize than it first appears, and it is the prize whose behavioural cost is
the one nobody has measured.

**Decision: ship the forced half, do not ship the elective half, and name the instrument that would
decide it** (§15.3). This is the disposition #13534 §3's anti-pattern demands — *optimising a measured
quantity whose input is unmeasured* — applied to the model's own behaviour.

### 5.3 Q3 — sub-node chunking

**#11235 §8 F1 places it on the DiVoid side, and this design agrees.** Chunking requires the graph to
return a bounded slice of a node's content; nothing in this repo can synthesise that, because `Recall`
returns whole `content` (`internal/divoid/client.go:35` projects `id,type,name,similarity,content,substance`)
and a client-side truncation is a lossy cut this repo would be inventing without a fidelity story.

**What this repo does in the meantime:** nothing new. The in-repo degraded-admission answer is
`substance`, under #11308 / Unit B, whose whole argument is that a lossy form may stand in **only where
the faithful one was going to be absent**. That argument is already made and already merged.

**What this repo would ask DiVoid for**, stated so it can be filed rather than designed here: a recall
projection that returns a **bounded prefix** of `content` together with the node's true byte length, so
admission can charge the slice and the record can still report the whole. That is a DiVoid-side task
and §17 Q3 carries it. **It is named, not specified** — the shape belongs to whoever owns the graph API.

---

## 6. Architectural Overview

Nothing is added to the turn. Two existing render/report sites learn to say one more true thing.

```
  Retrieve ──► Assemble ──┬──► block ──────────────────────────────► model
   (unchanged)            │     (renderBlock: admitted rows only —
                          │      UNCHANGED, see §5.2)
                          │
                          └──► dispositions ──┬──► Record.Candidates ──► response  (already carries cutReason)
                                              │                      └─► node substance (already groups by reason)
                                              │
                                              └─► logFinished ──┬─► "run finished"  cut=N       (exists)
                                                                ├─► WARN shutout               (exists, turn.go:189)
                                                                └─► WARN rank-1 dropped        ◄── UNIT 1 (new)

  dispatchRecall ──► admit(20,000 B) ──┬──► Results (admitted) ──► RenderToolResult ──► model
                                       └──► Dispositions (all) ──►        ▲
                                                                          └── UNIT 2: the empty branch reads
                                                                              Dispositions, so it can tell
                                                                              "found nothing" from
                                                                              "found rows, none included"
```

> **Corrected 2026-09-11 during Unit 2's implementation (QA #13697 CF-1).** The last line of the diagram
> read *"found rows, none fit"*. The branch is reason-neutral by construction — `admit` cuts on **two**
> reasons, and the self-produced one charges no bytes at all — so *"none fit"* names only one of them
> and is false on the other. Corrected here and at §14's G-6 row; the rendered sentence is
> `"results were found, but none were included."`

**The whole design is that the two sites which already hold the fact start saying it.** No new
component, no new port, no new type, no new constant, no new persisted field.

---

## 7. Components and Responsibilities

| component | gains | still does not own |
|---|---|---|
| **`logFinished` (`turn.go:166`)** | one WARN: the rank-1 candidate was dropped for the byte budget. | Deciding admission; reading the block; any reason other than the byte budget. Self-produced cuts belong to #13601's WARN and are explicitly not this one's (§11). |
| **`RenderToolResult` (`assemble.go:84`)** | the ability to distinguish *recall returned nothing* from *recall returned rows and admission cut all of them*. | Naming which rows, or their ids — the model cannot fetch by id (C3), so ids would be budget spent on something unusable. |
| **`admit` (`assemble.go:30`)** | **nothing.** | — |
| **`renderBlock` (`assemble.go:108`)** | **nothing.** | — |
| **`Record` / `Disposition` / `runResponse`** | **nothing.** | — |

---

## 8. Interactions and Data Flow

**Unit 1, on the run that produced this task (#13591).** Assemble returns twenty dispositions; rank 1
carries `cutReason: "byte budget exceeded"`. `logFinished` already computes `cut` (`turn.go:168`) and
already decides whether to raise the shutout WARN (`turn.go:189`). It additionally scans the
dispositions in rank order for the first row whose cut reason is the byte budget; that row is rank 1,
so it raises the WARN naming `#13585`, `67,312 B` — the size **that run** charged, which is what the
WARN reports and why it is a `(record)` figure by construction — and the `56,592 B` that remained.
**The `run
finished` Info line is unchanged.**

**Unit 1, on a run whose top hit was admitted (#13598, #13600).** The first byte-budget cut is at some
rank > 1. **No WARN.** This is the case that makes the signal worth having: those runs cut **9** and
**8** rows respectively and answered correctly.

**Unit 1, on a shutout.** Both WARNs fire — the existing one says *nothing was admitted*, the new one
says *and the best thing was X at Y bytes against Z remaining*. **They are complementary, not
duplicates**, and §14 pins that they coexist. On the anchor-alone path
(`TestTurnRunWarnsWhenTheAnchorAloneConsumesTheWholeBudget`, `turn_test.go:1259`) the remaining budget
the WARN names is `0`, which is what makes that case self-explanatory rather than misleading.

**Unit 2.** `dispatchRecall` (`turn.go:359-360`) already puts the full disposition set on the exchange
alongside the admitted set. `RenderToolResult`'s empty branch reads the count of dispositions it
already receives and says which of the two situations obtains. **When recall genuinely returned
nothing, the existing sentence is unchanged** — pinned by a test that already exists
(`TestRenderToolResultRendersARecallThatFoundNothingAsOneSentenceRatherThanAnEmptyString`,
`toolresult_test.go:29`), which constructs a `ToolExchange` with no dispositions and must stay green.

---

## 9. Data Model (Conceptual)

**Unchanged, and that is the design's main claim rather than an omission.**

`Disposition` already carries `Rank`, `Included`, `CutReason`, `Size` and `Similarity`
(`types.go:54-70`). `ToolExchange` already carries `Dispositions` beside `Results` (`types.go:199-208`).
**Every fact both units need is already modelled, already populated, and already on three surfaces.**

Three consequences follow, and each is a cost avoided rather than a feature:

- **The record does not grow.** #13481 measures `candidates` alone at 93.2 % of DiVoid's 8,000-character
  embedding window; a new field declared after it would land outside. This design adds none.
- **The corpus does not shift.** A run record whose shape changes changes what later runs retrieve
  (C1). This one does not.
- **No new derived field is added to the response.** §2 establishes it would not be a signal.

---

## 10. Contracts and Interfaces (Abstract)

| | contract |
|---|---|
| **I-1** | The run-summary block raises **at most one** rank-1-dropped record per run, and only when the **first** disposition in rank order carrying the byte-budget reason is at **rank 1**. |
| **I-2** | That record names, at minimum: the subject, the dropped node's id, its size in bytes, and the budget that remained after the anchor. The node's name is included and may be truncated; the *identifiers* may not. |
| **I-3** | A cut whose reason is **self-produced** never raises I-1. The two reasons are distinct strings (`assemble.go:12-13`) and the byte-budget one is the only trigger. |
| **I-4** | I-1 is **independent** of the shutout record (`turn.go:189`). Both may fire for one run; neither suppresses the other. |
| **I-5** | The `run finished` Info line, the HTTP response, the record, and the block are **byte-identical** to what they are at `872156e` for every input. |
| **I-6** | A tool result whose admitted set is empty states which of two things happened: recall returned no rows, or recall returned rows and none were admitted. The first case keeps its current sentence **verbatim**. |
| **I-7** | The tool result names **no node ids and no node names** for unadmitted rows (C3: the model cannot act on them). |

---

## 11. Cross-Cutting Concerns

**Error handling.** Neither unit can fail. Both are pure projections of data already in hand: no I/O,
no allocation that can fail, no new failure mode, no new error kind. Neither changes any status code.

**Security / secrecy.** A-2 covers the node name. No credential, no endpoint, no upstream text is
touched — that surface belongs to #13564 (**design merged, not implemented**) and to #13565 / #13578
(**both closed, implementations on `main`**), and this design does not reach it.

**Observability.** The new record is a **WARN**, matching the severity of the shutout record it sits
beside. That is deliberate: a signal is only a signal at a level the reader filters *for*, and `Info`
already carries `cut=N`, which §12 shows is the non-discriminating figure.

**Idempotency / concurrency / caching.** Not engaged. `logFinished` runs once per turn on the turn's
own goroutine; `RenderToolResult` is a pure function of one exchange.

**Consistency model.** Unchanged. Nothing is written.

---

## 12. Quality Attributes and Trade-offs

### 12.1 The rejected alternative, falsified on measurement

**Rejected: raise the WARN whenever any row is cut on the byte budget.** It is the obvious rule, it is
simpler to state, and it is wrong:

| run | byte-budget cuts | rank-1 outcome | answer | *any-cut* rule | *rank-1* rule |
|---|---|---|---|---|---|
| **#13591** | 5 | **cut on size** | **wrong** | fires ✔ | fires ✔ |
| **#13598** | **9** | admitted | right | fires ✘ | silent ✔ |
| **#13600** | **8** | admitted | right | fires ✘ | silent ✔ |

**Cut count discriminates 0 of 3. Rank-1 admission discriminates 3 of 3.** A WARN that fires on the
runs that went well is a WARN that gets filtered out, and then the real one is gone with it.

**n = 3, and it is three runs, not three inputs** — #13598 and #13600 are the same input repeated, so
the independent samples are two. Stated rather than rounded up.

**Also rejected: report the highest-ranked byte-budget cut at any rank** (e.g. *"rank 4 was dropped"*).
It is strictly more informative and strictly less usable: every run in the table above would emit it,
including the two that answered correctly, so it reduces to the any-cut rule with extra characters.

### 12.2 The trade-offs taken

| attribute | how it is served | what is given up |
|---|---|---|
| **Simplicity** | Two edits, no new type, no new constant, no new field, no new tunable. The whole operator half is a projection of data three surfaces already carry. | The signal lives in stderr, not in the response the operator is holding. §2 argues the response cannot carry a signal at 70–100 KB; that argument is measured but it is an argument, not a proof. |
| **Precision** | Rank 1 is not a threshold — it is the definition of *the best thing the graph found*, so no constant is introduced and none can drift (#1136 §3). | The rule is narrow by construction: see §13 F-1. |
| **Reversibility** | Both units are deletable in one commit each. Nothing persists, nothing migrates, no consumer is created. | — |
| **Honesty to the model** | Unit 2 removes a false statement at zero budget cost. | The model still learns nothing about partial cuts. §5.2 argues silence is not a lie and the manifest is unmeasured; §15.3 names what would settle it. |

---

## 13. Risks, and what would falsify each universal

**F-1 — the universal this design states, and the input class that breaks it.**

> *Stated:* the rank-1 rule fires exactly on the runs where the loop failed to read what would have
> answered the question.

**The class that falsifies it: rank 1 is admitted and ranks 2–3 are cut on size and held the answer.**
On #13591 that class was *also* live — **#11235 at rank 3, 96,555 B**, cut. Had #13585 been 9 KB and
#11235 unchanged, the WARN would have stayed **silent on a run that still lost a required document.**
**This class exists in the wild, in the very run this task was filed from.** The error direction is
**under-inclusion, and therefore silent** — the worse of the two directions (#1220 §5 addendum).

**Accepted, with the reason stated rather than hidden:** the alternative that covers it is the any-cut
rule, which §12.1 falsifies on measurement. A rule that covers F-1 and still discriminates has not been
found and is not invented here. **The bounded claim this design actually makes** is the one it can
defend: *a run whose single best candidate was dropped for size says so.*

**F-2 — the WARN fires on a run that was fine.** An arithmetic task whose best match is a large,
irrelevant design document. The statement is **true** and the defect is absent. Direction:
**over-inclusion, visible to whoever meets it**, self-correcting on read. Accepted.

**F-3 — the population shifts under the rule when #13601 Unit 1 lands.** With self-produced rows
excluded at fusion, rank 1 on an ill-matched run becomes a real node that may be oversize, so the
WARN's fire rate on that arm changes. **Whether rank 1 is among #13671's 9 size-cuts is not measured,
and this document does not predict it** — §15.1 makes it the first thing to read.

**F-4 — a second WARN on the same block dilutes both.** After #13601 Unit 1 and this unit, the
run-summary block can raise three records for one run. **Mitigation is that each names a distinct
condition**, and §14 G-3 pins that the reasons do not collapse. **Not mitigated:** the aggregate
readability of three WARNs, which is a judgement no test settles.

**F-5 — #13564 and this unit collide in `turn.go`.** Textual, not semantic: disjoint line ranges
(`:326`/`:339`/`:356` versus `:166-193`). §18 sequences rather than bundles.

---

## 14. Coverage — the guard, the premise that makes it discriminate, and its falsifier

**Re-derivation rule, because this list must not be trusted as complete:** the guards below are those
the two units' contracts (§10 I-1 … I-7) require. To regenerate, read §10 and ask of each contract
*which named test would redden if an implementation lacked it*. **Do not treat the table as the
inventory.**

| # | contract | guard (name to create) | the premise that makes it discriminate | falsifier |
|---|---|---|---|---|
| **G-1** | I-1, I-2 | `TestTurnRunWarnsWhenTheTopRankedCandidateWasDroppedForTheByteBudget` | The fixture's **rank-1** candidate is oversize while **lower-ranked** rows are admitted. An implementation that only counts cuts, or only fires on a shutout, stays silent here — which is the #13591 shape exactly. | **No runnable falsifier established.** The mutation (delete the WARN site) is not run by the author; the implementer quotes its output. |
| **G-2** | I-1 | `TestTurnRunDoesNotWarnWhenTheTopRankedCandidateWasAdmittedEvenThoughOthersWereCut` | The fixture admits rank 1 **and cuts several lower rows on size** — the #13598 shape, 9 cuts and a correct answer. **A guard without the "even though others were cut" clause passes against the any-cut rule §12.1 rejects.** | **No runnable falsifier established.** |
| **G-3** | I-3 | `TestTheTopCutWarnIsSilentWhenTheTopCandidateWasCutAsSelfProduced` | `admit` reaches its self-produced arm (`assemble.go:53`) **before** the budget arm (`:55`), so a rank-1 row cut as self-produced carries that reason and **was never charged for bytes**. **The fixture must therefore carry a `Size` strictly greater than the remaining budget:** with `Anchor.Size` 100 and `AssemblyByteBudget` 60,000, remaining is **59,900**, so `Size: 70_000`. Below that threshold the byte-budget condition is unreachable, the guard passes on arithmetic alone, and it separates nothing — see §14b. Built by hand against `logFinished`, because the case is unreachable through `Turn.Run` and that is the invariant working, not a defect; the name no longer claims `Turn.Run` because the assertion cannot carry it (P-20). | **Established, and its output is quoted.** QA #13947 probed `logFinished` with a self-produced rank-1 disposition at `Size: 70000` against `remaining = 59900`: `MISATTRIBUTES: self-produced row at Size=70000 (> remaining 59900) raised the BYTE-BUDGET warn`. A reason-reading implementation stays green on that fixture; one keyed on `!Included && Size > remaining` reddens. |
| **G-4** | I-4 | `TestTurnRunRaisesBothTheShutoutAndTheDroppedTopCandidateRecordsWhenNothingWasAdmitted` | Every candidate oversize: the shutout condition and the rank-1 condition are **both** true. An implementation that treats the new record as an `else` branch of the shutout reddens. | **No runnable falsifier established.** |
| **G-5** | I-5 | `TestTurnRunLeavesTheRecordAndTheBlockUnchangedWhenTheTopCandidateWasDropped` | Compares the record and block against the pre-change golden for the same fixture. **This is the guard that makes "zero cost" checkable** rather than asserted. | **No runnable falsifier established.** |
| **G-6** | I-6 | `TestRenderToolResultSaysResultsWereFoundAndNoneWereIncludedWhenAdmissionCutThemAll` | The exchange carries **empty `Results` and non-empty `Dispositions`** — the `turn.go:359-360` shape. A renderer reading only `Results` cannot tell this from the empty case and reddens. | **Live, and its output is quoted:** at `872156e`, `git grep -n "no additional results found" -- 'internal/**'` returns `internal/loop/assemble.go:92` and `internal/loop/toolresult_test.go:34`. G-6 requires a third site to exist and the `assemble.go:92` branch to be conditional. |
| **G-7** | I-6 | `TestRenderToolResultRendersARecallThatFoundNothingAsOneSentenceRatherThanAnEmptyString` — **exists**, `toolresult_test.go:29` | It constructs a `ToolExchange` with **no dispositions at all**, so it distinguishes *fix the empty branch* from *replace the empty branch*. **Must stay green unmodified.** | **Live:** it is in the tree today and passes; a fix that rewrites the genuinely-empty message reddens it. |
| **G-9** | I-3, §14a | `TestTheAssembledBlockIsAFunctionOfTheAdmittedRowsAlone` | Calls `Assemble` twice with the same anchor and budget: once with the full candidate list, once with **only the rows the first call admitted**. The two blocks must be **byte-identical**. A disclosure differs between the two calls **only where an arm realizes the state that disclosure is keyed on** — so G-9's reach is its arms, not the input domain (§14c). Widen the arms to widen the reach; this row is not closure and must not be read as it. | **Established.** §14a's probe appends a withheld-count manifest inside `Assemble`; of every instrument this design relied on, G-9 is the only one that reddens against it. |
| **G-10** | I-3, §14c | `TestTheBlockTheModelIsSentIsTheBlockTheRecordCarries` | Runs a real `Turn.Run` whose candidate set has at least one row cut, captures the `JudgeInput` the model port received, and asserts its `Block` is byte-identical to `Record.Block`. **Measured at `1547aed`, where the guard has two arms** — *partial admission* (rank 1 cut, one lower row admitted) and *shutout* (every row cut). **Catches:** M8, and M11 — a post-record append keyed on a shutout, which reds the shutout arm as the sole failure. **Does not catch:** M9 (pre-record) and M13 (post-record, keyed on rank 1 admitted with a lower row withheld) — §14c R1. No span is claimed for this guard; the row states what was run against it and nothing else. | **Established.** §14c's **M8** — the append keyed on any row having been withheld — left the whole suite green at `274dcc8`, before this guard existed, which is why it exists; at `e343870` it reddens this guard **and nothing else**, and the guard is green unmutated. |
| **G-8** | I-7 | `TestRenderToolResultNamesNoUnadmittedNodeInTheAllCutSentence` | C3: the model cannot fetch by id, so an id in that sentence is budget spent on an unusable fact. A renderer that lists the cut rows reddens. | **No runnable falsifier established.** |

> **Corrected 2026-09-11 during Unit 2's implementation (QA #13697 CF-1).** G-6's name above read
> `TestRenderToolResultSaysResultsWereFoundAndNoneFitWhenAdmissionCutThemAll`. **A name-to-create is a
> claim, and that one was false.** The sentence the guard asserts is reason-neutral —
> `"results were found, but none were included."` — while *"none fit"* is budget-specific language for a
> branch that also fires when every row was cut as **self-produced**. `admit` reaches the self-produced
> arm (`assemble.go:52-53`) **before** the byte-budget arm (`:58-59`), and the supplementary path is the
> only one that can reach it: `dispatchRecall` calls `Graph.Recall` directly at `turn.go:357` and hands
> the rows to `admit` at `:363`, bypassing `Retrieve`, whose `fuse` drops self-produced rows outright.
> So an all-self-produced cut set admits nothing while nothing was ever charged for bytes, and *"none
> fit"* would be **an affirmative falsehood on a reachable success path** — the exact class this unit
> exists to remove. Renamed to match the wording `RenderToolResult` actually produces; §18's acceptance
> for Unit 2 is satisfied by the renamed guard.
>
> **Two guards not listed above were added, under this section's own re-derivation rule** (*"do not
> treat the table as the inventory"*): an all-self-produced fixture, which is the only guard that makes
> the reason-neutrality discriminate, and a genuinely-empty fixture carrying a **non-nil, zero-length**
> `Dispositions` slice — the shape `admit:31`'s `make(...)` actually delivers, which `r.Dispositions !=
> nil` would pass while breaking production silently. The second of those is what satisfies G-6's
> falsifier: it supplies the third `"no additional results found"` site the column requires.

### 14a. RETRACTED 2026-09-15 — both instruments this section named are unsound, and the replacement is G-9

**This section previously made §5.2's elective half mechanically checkable two ways. Neither works, and
the measurement is below.** §18's Unit 1 acceptance criterion built on the first of them is retired in
the same amendment.

**What it said.** A pre-submit grep — `git grep -n "CutReason" -- 'internal/loop/*.go'
':!internal/loop/*_test.go'` — whose output this design had to leave unchanged at four lines; and,
named as strictly stronger, a type-level fact: *"`renderBlock`'s signature is `(anchor Anchor, admitted
[]Candidate)` and `Candidate` declares no `CutReason` and no `Included`. **Disclosure to the model is
impossible without changing that signature or that type.**"*

**That sentence is false.** Measured in a detached worktree at `fb48f66`: `Assemble` (`assemble.go:18`)
holds **both** the dispositions and the rendered block, and returns `renderBlock(anchor, admitted)`. A
`withheldManifest` helper appended to that return value discloses to the model how many rows were
withheld, and:

| instrument | result |
|---|---|
| `renderBlock`'s signature | **unchanged** (`assemble.go:127`) |
| `Candidate` (`types.go:27`) | **unchanged** — no diff in `types.go` at all |
| §18's nine bounce conditions | **0 changed lines**, each |
| the whole suite | **13/13 `ok`** |
| §18's grep | **the same four sites, no fifth** |

The manifest counts `!d.Included` and never names a reason, so no spelling of `CutReason` appears; it
lives in `Assemble`, which is on neither the signature nor the bounce list. **Every instrument this
design owned passed a change that puts withheld-row information directly into the model's block.**

**So the grep is retired, and the type-level claim is withdrawn rather than relied on.** The grep was
also wrong in the other direction, which is how this was found: it pins *spellings across files*, so it
forbids a **read** on the operator-log path — which cannot reach the model and which the property
permits. It bounced a compliant implementation and cost a real mechanism change (#13944), and it is the
*fires-on-compliant-code* shape this repo has now recorded four times. A third defect, minor but worth
naming: *"the same four lines"* is ambiguous between the four **sites** and their **line numbers**, and
under the literal reading any unrelated edit to `assemble.go` fails it.

**The replacement is behavioural: G-9** asserts that the block `Assemble` returns is a function of the
admitted rows alone — assemble twice, once with everything and once with only what was admitted, and
require byte-identical output. It reddens against the manifest above, and it is **strictly stronger than
the retired grep**, which that manifest leaves untouched.

> **RETRACTED 2026-09-15 — this section originally continued:** *"it does so without caring what field
> carries the disclosure, which function emits it, or how anything is spelled … A longer list of names
> is the same instrument that just failed … **G-9 protects the property.**"* **Both halves are false.**
> §14c carries the measurements that falsify them and the ruling that replaces them.

### 14b. Why G-3's fixture value is the whole of G-3

`admit` charges no bytes against a row it cuts as self-produced, so the byte-budget condition
`!Included && Size > remaining` is **false by default** on any self-produced fixture whose size is
small. The shipped Unit 1 fixture used `Size: 500` against `remaining = 59900` — 119× below the
threshold — so the guard passed without consulting the reason at all. QA measured all three
consequences: deleting `CutReason: cutReasonSelfProduced` from the fixture left it **green**; breaking
`appendUnseen` reddened **seven** other tests and left it **green**; and the same guard at `Size: 70000`
**reddens** against the shipped implementation.

**A guard whose stated job is to reject an approach, which that approach passes, is decoration** — and
the cost here was not the guard: it was that the round's one source of information about the collision
stayed green, so the design bounce (§18) was never triggered and the resolution was made under a gate
that should not have existed. **A fixture below the threshold at which a guard can discriminate is a
defect in the guard, not a detail of it.**

---

### 14c. RULING 2026-09-15 — a behavioural gate over `Assemble` cannot establish this property, and three rounds of widening is the evidence

§14a retired a false universal and installed another one section later. This is the ruling on whether a
third attempt would fare better. **It would not, and the reason is categorical rather than a matter of
fixture quality.**

#### What was measured

| id | disclosure | where | suite |
|---|---|---|---|
| **M1** (§14a) | unconditional withheld-count manifest | `Assemble` | **red** — G-9 both arms |
| **M2** (#13957) | keyed on a shutout | `Assemble` | **red** — G-9 shutout arm only |
| **M3** (#13980) | keyed on the rank-1 row cut for the byte budget | `Assemble` | **green** |
| **M4** (#13980) | keyed on a shutout whose reasons are all byte-budget | `Assemble` | **green** |
| **M5** (#13980) | keyed on a withheld count of four or more | `Assemble` | **green** |
| **M8** (this ruling) | keyed on **any row having been withheld**, after `record.Block` is set | **`Turn.Run`** | **green** |
| **M8u** (#13985) | the same, **unconditional** — appended on every run | **`Turn.Run`** | **red** — a pre-existing assertion, below |

M3 is this unit's own subject condition — the rank-1 row cut for the byte budget is exactly what the
operator WARN reports, and whether the same fact reaches the **model** is §5.2's elective half.

**M8 is mine, and it escapes `Assemble` entirely.** Appending to `block` between the record's
construction and `t.judge(...)` sends the model a block the record does not carry. Measured in a detached
worktree at `274dcc8`, re-measured at `098c4ca` and again at `e343870`: **the suite stays green**, and a
probe asserting `JudgeInput.Block == Record.Block` was green unmutated and red under it, printing the
delta verbatim.

> **CORRECTED 2026-09-15 (#13985).** This paragraph first described M8 as **unconditional** and said it
> *"needs no predicate at all"*. **That is false of the mutation that was run.** M8 appends only when some
> row was withheld (`for _, d := range dispositions { if !d.Included { … } }`) — which is a predicate, and
> the natural one, since a withheld-row disclosure has nothing to say when nothing was withheld. The
> *"13 `ok`, 0 `--- FAIL`"* figure is correct for that mutation and reproduces; it does **not** reproduce
> for the unconditional variant the sentence described. **The number was right and the label was wrong**,
> which is this arc's recurring shape and is why the correction is recorded rather than patched away.

**The unconditional variant is caught, and by an assertion nobody would find.** `internal/loop/turn_test.go`
has carried `if model.calls[0].Block != record.Block` since **`11ad386`**, inside
`TestTurnRunRecordsTheModelsAnswerAndStopsAtOneCallWhenAnswered` — a name that carries none of this
property (P-20). It is a real discriminator, and it reddens against an unconditional append.

**It does not weaken the case for G-10; measurement sharpens it.** That test's fixture is `baseGraph()`,
which supplies **no candidates at all**, so no row is ever withheld in it and M8's predicate never fires
there — verified: under M8 the whole suite reddens **only** G-10, and that test passes. So the
pre-existing assertion catches the one shape a withheld-row disclosure would never take (*append even
when nothing was withheld*) and misses the shape it would (*append because something was*). **It is
fixture-limited in exactly the direction that matters**, on top of being unfindable by name. G-10's
fixture carries an oversize row (`AssemblyByteBudget+1`), so it exercises the predicate and catches both
variants.

**Keep the old assertion; do not rename it and do not delete it as redundant.** Renaming would mis-describe
the dozen other things its test checks, and deleting a green guard because another one covers it today is
how a pair stops discriminating quietly. It is recorded here so the next reader neither re-discovers it as
news nor mistakes it for this property's guard.

**And G-10 catches M8 and none of M1–M5 — by design, not as a gap.** `Turn.Run` calls `Assemble` once and
uses the one returned block for both the record and the judge, so a disclosure injected *inside* `Assemble`
poisons both copies identically and the two stay equal. **G-10 tests for divergence between them, not for
disclosure as such**, so it structurally cannot see an assembly-scope injection — which is exactly what the
forbidden-list entry is for. The two instruments partition the problem and neither covers the other's half;
confirmed by constructing M3 and watching G-10 stay green.

#### Why widening cannot close it

G-9 evaluates the property at the points its arms realize. The property is a **universal over an
unbounded input domain** — for every anchor, candidate set and budget, the block is a function of the
admitted rows alone. For any finite set of arms there exists a predicate false at all of them, so a
disclosure keyed on that predicate survives. **That is a fact about sampling a universal, not about the
arms chosen**; M3, M4 and M5 are three instances of it rather than three oversights. Answering them
tells the next round nothing about M6.

**So the closure claim is withdrawn rather than re-attempted.** G-9 keeps a real and stated job: it is
strictly stronger than the retired grep, it costs one test, and it reddens any disclosure that varies
with the withheld rows **at the states its arms realize**. That is a usable result. A universal the next
attack falsifies is worse than no claim, because §18 declined a structural protection on the strength of
this one.

#### Where a disclosure can be introduced at all, which is derivable rather than enumerated

A model-facing disclosure needs both the rendered block and the withheld rows in one scope. At
`274dcc8` exactly two scopes hold both, and the list is closed by construction rather than by search:

| scope | holds both? | why |
|---|---|---|
| `renderBlock` | no | receives `(anchor, admitted)`; `Candidate` declares neither `Included` nor `CutReason` |
| `admit` | no | holds the dispositions and the admitted rows, never the block |
| **`Assemble`** | **yes** | computes both and returns both |
| **`Turn.Run`** | **yes** | `block, dispositions := Assemble(...)`, and `block` then flows to `t.judge` |
| `cmd/eval`'s sweep | no | discards the block (`_, dispositions := loop.Assemble(...)`) and calls no model |

#### The ruling

1. **`Assemble` goes on §18's forbidden list.** This reverses §14a, and the reasoning that rejected it —
   *"a longer list of names is the same instrument that just failed"* — was wrong on the comparison. The
   grep matched **content**, so it fired on a compliant operator-path read. A bounce condition is
   **diff-scoped**: it asks whether this change touched `Assemble`, which cannot be true of a compliant
   change and cannot be false of any of M1–M5. It is the opposite failure direction, not the same
   instrument. And the entry is principled rather than enumerative — `Assemble` is on the list because
   it is one of the two scopes above, not because someone thought of it.
2. **`Turn.Run` cannot go on the list**, because every unit edits it. **What G-10 is measured to catch:**
   M8, at `e343870` — and nothing else among the mutations run against it. **What it is measured not to
   catch:** M9 and M11, both below. No claim is made here about which positions or spans are guarded;
   two such claims have been made in this section and both were falsified.
3. **The residuals, enumerated and ranked by reachability. They are not characterised** — three
   characterisations of this set have now been falsified (*"requires breaking a prohibition"*, *"is
   recorded"*, *"the silent half is guarded"*), so the enumeration carries measurements only.

   **R1 — an append inside `Turn.Run`.** Breaks no prohibition: `Turn.Run` is deliberately off §18's
   list. **Membership re-measured at `1547aed`**, each row a whole-suite run at exit 0 with no reds:

   | id | position | key | measured at `1547aed` |
   |---|---|---|---|
   | **M9** | before the record literal (`turn.go:149`) | shutout | **survives** — both G-10 arms green, both G-9 arms green, `assemble.go` unchanged so §18's bounce is not triggered |
   | **M13** | after the record literal, before `t.judge` | rank 1 admitted, a lower row withheld | **survives** — the shape §15 calls *well-matched*; G-10's partial arm has rank 1 **cut**, so it does not realize this |
   | ~~**M11**~~ | after the record literal | shutout | **no longer a member** — at `1547aed` it reds G-10's shutout arm as the sole failure. It was a member at `0d53a8c`, before that arm existed. |

   **M9 is the member that decides the ruling below, and it replaced M11 when the arms landed.** Its
   standing is **structural rather than fixtural**: a pre-record append lands in `Record.Block` *and* in
   the block sent to the model, so the two values G-10 compares stay identical and the relation holds.
   **No number of arms can change that** — arms fix M11 and M13; they cannot reach M9, because G-10
   asserts an equality that this position preserves.

   **R2 — an append inside `Assemble`, keyed on a state G-9's arms do not realize.** Measured survivors:
   M3, M4, M5. Additionally requires changing `Assemble` against §18's explicit prohibition, which R1
   does not.

4. **No further guard is specified here, and the reason is a correction.** An earlier revision declined a
   recomputing guard (`Assemble` recomputed and compared to `Record.Block`) on the ground that it would
   be the fixture-sampled class while G-10 was not. **That distinction does not exist** (QA #13993). A recomputing
   guard is the same instrument class as G-10 — a relation between two observed values in one run — and
   **there is no non-fixture-sampled formulation of that relation that is a test**, because a test checks
   its own fixture. Adding guards extends fixture sets; it does not change the class. G-10's missing
   shutout arm was a defect in its arms, not in its specification; it landed at `1547aed`, and M11 moved
   out of R1 as a result.

#### The one instrument that is not fixture-sampled, priced and declined

Disclosure becomes impossible if the block's type is unforgeable outside its constructor: move
`renderBlock` into its own package returning an opaque `Block` whose only field is unexported, and type
`JudgeInput.Block` as that type. Then no scope outside that package can append to a block, because none
can construct one. It is **the only instrument in this document that is not fixture-sampled**. The price
is one package, one type, `JudgeInput.Block`'s type, **two adapter read sites** (`internal/ollama/wire.go:113`, `internal/openaicompat/wire.go:120`), and every test that
constructs or reads a `JudgeInput`.

**Declined on proportion alone.** This unit's deliverable is one operator WARN (§18 Unit 1); the remedy
restructures the model-facing type system. RULING 2026-09-03 and #1136 §2 both bind on that ground, and
neither needs a claim about what the gates cover.

> **No detectability argument is offered for this decline.** This section previously offered two — *"it
> requires breaking a prohibition"* and *"the leak is recorded"* — and R1 falsifies both: M9 and M11 break
> no prohibition, and M11 is absent from `Record.Block`. **The decline rests on proportion to this unit
> and on nothing else, and it is re-decidable on that ground alone:** if the property is wanted closed,
> this is the instrument and its price is above. That is a scoping decision for the operator, not a
> finding this document can settle.

#### The check this section owes itself

Four claims corrected in this document were **generalisations of true ones**, and in each case a
correctly-scoped original already existed in a row of a table.

> **Before a summary sentence about a guard ships, resolve it against the row it summarises** — the way a
> `file:line` citation is resolved against the file. A summary that widens its row is the defect; the row
> is the record.

**The row may be anywhere, and proximity is not part of the check.** At least one original did not sit
near its summary: the claim that *"§15's both-arms benchmark
reads that block"* was contradicted by the scope table **two sections earlier**, which records that
`cmd/eval`'s sweep *"discards the block (`_, dispositions := loop.Assemble(...)`) and calls no model"*.
The sweep also calls `Assemble` directly and never enters `Turn.Run`, so it could not observe a
`Turn.Run` disclosure at any distance. A proximity-scoped check would have passed that sentence.

#### And the check above cannot find a stale row, so here is the second one

Every defect corrected in this section up to `17933d7` was **a summary widening a row**. The next one was
**the inverse**: G-10 gained a shutout arm at `1547aed`, and this section's rows went on describing the
one-fixture guard. **Resolving summaries against rows cannot find that** — the rows agreed with each
other and disagreed with the tree. The first check is sound and structurally blind to it.

> **A guard row is a measurement against a tree, not a statement about a guard. It carries the ref it was
> measured at, and it is re-resolved against the tree — not against other rows.**

**The trigger is mechanical and needs no judgement:** for each guard row, if
`git diff <the row's ref>..HEAD -- <the guard's test file>` is non-empty, that row is **due
re-measurement** and may not be relied on until it has been.

**The trigger applies to rows that carry a measurement, and §14's table is not yet uniform — stated rather
than asserted, because this paragraph was itself written claiming otherwise and the check above caught
it.** Of the guard rows, **three claim a measurement**: G-3, G-9 and G-10. **Only G-10 carries a commit
ref**; G-3 cites a QA node and G-9 cites *"§14a's probe"*, neither of which fixes a tree. So the trigger is
runnable for G-10 today, and **G-3 and G-9 are due a ref**. The remaining rows read *"No runnable
falsifier established"*, which needs no trigger: there is no measurement in them to go stale. **A row that
names a measurement must carry its ref; a row that names none must not pretend to.**

**Two halves, and a row must say which it has.** The cheap half is mechanical — the named test resolves,
and it still has the shape the row assumes (`TestTheBlockTheModelIsSentIsTheBlockTheRecordCarries` has
**two** arms at `1547aed`, not one). The expensive half is re-running the mutation the row names. This
round ran the expensive half for M9, M11 and M13; the rows say so, at that ref.

**The direction of this defect was conservative** — the rows *understated* what the guards catch, so
nothing shipped weaker than described. That is worth recording and is not a reason to grade it lower: it
was a live false claim in the table §18 reads its acceptance from, and it made a caught mutation the
ruling's pivot.

## 15. How the remedy is measured — both arms

**The premise that makes any before/after attributable:** retrieval is reproducible — 19 of 20
candidates identical across two runs of an identical input (#13592, A-3).

**The instrument is `cmd/eval`, not the product.** Running the product mutates the corpus its own
measurements are taken against (C1); `cmd/eval` reads the graph and never writes to it, pinned by
`TestSweepReadsTheGraphAndNeverWritesToIt` (`cmd/eval/sweep_test.go:369`). Its `RowResult` already
carries `Candidates []loop.Disposition` (`internal/eval/result.go:38`), **so the baseline needs no code
change to measure: the rank-1-cut-on-size predicate is computable from a sweep's existing output.**

### 15.1 Both arms, stated — and what a well-matched benchmark reports

| arm | input shape | measured | what the WARN does |
|---|---|---|---|
| **Well-matched** | the graph answers it well (#13598, #13600 — the Go comment rules) | top hit **#10861, 9,037 B, admitted**; **9** and **8** rows cut on size; answers correct | **silent** — and this is the arm a benchmark is built from |
| **The arm that filed the task** | our own design documents (#13591) | top hit **#13585, 67,312 B `(record)`, cut**; answer wrong | **fires** |
| **Ill-matched, post-#13601** | the graph holds nothing relevant (#13599) | **11 admitted, 9 cut on size**, total unusable 17 → 9 (#13671, correcting §16.2's ~12) | **not measured** — F-3 |

**The class this design addresses is reported as near-absent by a corpus benchmark, and here is the
number.** Across the 25 unique required nodes of `internal/eval/corpus.json`, exactly **one**
(#10926, 200,749 B) exceeds the 56,592 B that remained on #13591 — **1 of 25, 4 %** — and the two nodes
that actually caused the failure (#13585, #11235) **are not in the corpus at all.** A sweep therefore
under-reports the class by construction, and any claim of *"this defect is rare"* built from the corpus
inherits that. (Measured from node sizes on 2026-09-11, **not** from a sweep: whether #10926 is
retrieved for its row is not measured here.)

### 15.2 Acceptance

**Unit 1 and Unit 2 are accepted on their named guards (§14) and on nothing else.** No run of the
product is required to accept either, and none should be performed to accept them — C1.

**Reported, not gated:** after #13601 Unit 1 is on `main`, one `cmd/eval` sweep, and from its
`Candidates` arrays the count of rows whose rank-1 disposition carries the byte-budget reason. That
number is F-3's answer and it sizes how often the WARN speaks. **It is not a threshold and no threshold
exists anywhere in this design.**

### 15.3 What would decide the elective half (§5.2), so it is deferred and not abandoned

Two paired runs on the **ill-matched** arm — identical input, with and without the block manifest —
judged by a human on one question: *does the answer claim a completeness it does not have?* There is no
automated scorer for that and this design does not invent one. **Both preconditions:** #13601 Unit 1 on
`main` (the 9-cut volume is a property of post-#13601 behaviour), and an accepted way to run the
product twice without the first run's record joining the second's candidate set (C1). **Neither holds
today**, which is why the manifest is named and not built.

---

## 16. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

- **No new type mirroring an existing one.** No type is added.
- **No new abstraction with one implementation.** None is added.
- **No element justified by "we might need X later".** The manifest is the one candidate and it is
  **not designed** — §5.2 names its unmeasured costs and §15.3 names what would settle it.
- **No deprecation period, flag, shim or transition window.** None; both units are atomic.
- **DRY math.** No block is inlined at multiple sites. The rank-1 scan is **one** site
  (`turn.go:166-193`); the tool-result branch is **one** site (`assemble.go:92`).
  `block_size × site_count = n × 1` for both, so no extraction question arises.

**Existing systems first**

- **Audited.** §2's six-surface table is the audit, and it is what shrank this design: three surfaces
  already carry the data, so the change is a signal and not a store.
- **No new layer proposed**, so no reason to justify one.
- **No new persisted data point**, so no 4-week-decision gate to clear (#868).
- **Consumer chain recursed.** The WARN's consumer is the operator diagnosing a fluent answer — named,
  present, and the person who filed #13594. The tool-result sentence's consumer is the model, which
  reads it on every subsequent call of the turn (`turn.go` `PriorTools`). **Neither is a
  reader-with-no-downstream.**

**Configurability**

- **No new knob, and none is wanted.** Rank 1 is not a tunable: it is the definition of the graph's
  best answer. §3's `const`-stays-`const` rule is satisfied by adding no constant at all.
- **No telemetry-then-tune compound.** §15.2's reported number is read once to answer F-3, by a named
  reader, and tunes nothing.

**Less is better**

- **Can it be deleted?** The WARN: without it, a fluent wrong answer carries no signal on any surface
  the operator meets unasked — §1's failure. The tool-result branch: without it, the model is told
  something false. Neither deletes.
- **Can it be merged?** The WARN sits inside the existing `logFinished` block beside the existing
  shutout WARN — it **is** the merged form; a separate log site was rejected. The tool-result change is
  inside the existing branch.
- **Can it be inlined?** Both already are. No helper is proposed.
- **Trade-off named where the simple design has a downside:** §13 F-1 (the narrow rule's silent blind
  spot) and §12.2 (the signal lives in stderr).
- **Radical-clean over compromise:** the compromise shape here would be a derived summary object on the
  response — §2 rejects it on a measured property of the response (70–100 KB), not on taste.

**Document discipline**

- Cites **#114** (code contracts) and **#1136** (this checklist) as load-bearing; **#1267** applies
  vacuously (site_count = 1 at both sites).
- **Scope inventories explicit**, §3, including the three designs whose boundaries are named and then
  left alone.
- **No superseded predecessor.** This design supersedes nothing; it is the first design against #13594.
- **No multi-paragraph rationale for things that obviously stay.**

---

## 17. Open Questions

| | question | owner |
|---|---|---|
| **Q1** | **Is rank 1 too narrow?** §13 F-1 names the class it misses, and that class was live on #13591 (#11235 at rank 3). A wider rule that still discriminates has not been found, and any width beyond rank 1 introduces a threshold constant #1136 §3 would refuse. **Recommendation: ship rank 1, read §15.2's number, revisit only if it says the rule is silent on runs that went badly.** | Toni |
| **Q2** | **Should `cmd/condense` be run over the oversize design nodes?** #13585, #11235 and #10926 carry `substance: null` today. That is an **operation**, not a code change, and it is the gate on #11308's remedy rather than on this one. It is named here because nothing else names it. | Toni / #11308 |
| **Q3** | **Does DiVoid want a bounded-prefix recall projection?** §5.3 names the shape — a slice of `content` plus the node's true byte length — as the thing this repo cannot synthesise. **Not filed by this design** (the brief bars touching the filed DiVoid-side tasks); it is offered so the orchestrator can decide whether it is one. | orchestrator / DiVoid |
| **Q4** | **Does the third WARN on one run-summary block make the set unreadable?** F-4. No test settles it; one reading of a real log after #13601 Unit 1 lands would. | Toni |

---

## 18. Implementation Guidance for the Next Agent

**Two units, two PRs. The reason is sequencing, not ceremony.**

`turn.go` is the contended file: **#13564, #13585 Unit 1 and #13601 Unit 1 all edit it** (#13534 §13),
and they cannot run concurrently in one checkout. **Unit 2 does not touch `turn.go` at all**, so it is
free of that queue and can proceed immediately. Bundling them would put a `turn.go`-free change into a
`turn.go`-blocked PR for no gain.

### Unit 2 — the tool result stops asserting a falsehood *(unblocked; do this first)*

**The property that must become true in `internal/loop`:** *no rendered tool result states that recall
found nothing when recall returned rows that admission cut.* Sweep the package for that property;
**the site below is the one found, and this list is not exhaustive by construction.**

- Known occurrence: `assemble.go:92`, the `len(r.Results) == 0` branch of `RenderToolResult`.

**Acceptance:** G-6 and G-8 exist and pass; **G-7 (`toolresult_test.go:29`) passes unmodified** — if it
needed editing, the genuinely-empty message was changed and I-6 is violated.
**Not acceptance:** any product run.

### Unit 1 — the operator can tell *(gated: start after #13601 Unit 1 is on `main`)*

**Why gated:** #13601 Unit 1 adds its under-delivery WARN to the same run-summary block
(`turn.go:166-193`). Landing second means rebasing one WARN over another in a block neither design
owns alone. **Confirm the gate with `gh pr view 70 --json state,mergedAt` immediately before starting —
PR state is never assumed.**

**The property that must become true:** *a run that returned 200 and dropped its highest-ranked
candidate for the byte budget raises a WARN naming that candidate, its size, and the budget that
remained — and a run that admitted its highest-ranked candidate does not, however many lower rows were
cut.* Sweep `turn.go`'s run-summary block for that property; **the site below is the one found.**

- Known occurrence: `turn.go:166-193`, `logFinished`, beside the shutout WARN at `:189`.

**Acceptance:** G-1 … G-5, **G-9** and **G-10** exist and pass, with **G-3 carrying the fixture §14 specifies**
(`Size: 70_000`, strictly above the remaining budget) — a G-3 below that threshold is not acceptance,
it is decoration. **Not acceptance:** any product run, and §15.2's sweep figure — that is **reported**,
and gates nothing.

**RETIRED 2026-09-15 — the `CutReason` grep is no longer an acceptance criterion.** It forbade a read
on the operator-log path that the property permits, and it passed a model-facing disclosure that G-9
catches; §14a carries the measurement. **Consequence, and it is the point of retiring it:** the WARN
**reads `Disposition.CutReason`** to decide that the top row was cut for the byte budget. It is a claim
about a *reason*, so it reads the reason. Deriving it arithmetically from `!Included && Size >
remaining` is equivalent on every input reachable today — QA verified that end to end — but only
because `fuse`/`appendUnseen` (`retrieve.go:90`) drops self-produced rows before `Assemble` sees them,
which is a cross-file invariant that **no test couples to this inference** (#13943). The equivalence is
contingent; the field is not. Read the field.

### What neither unit may do

Change `admit`, **`Assemble`**, `renderBlock`, `Record`, `Disposition`, `Candidate`, `runResponse`, or
any budget constant. If an implementation finds it needs one of those, **the design is wrong and should
bounce** rather than be widened.

**`Assemble` was added 2026-09-15 (§14c), reversing §14a's decision to leave it off.** It is one of only
two scopes holding both the rendered block and the withheld rows, and every measured disclosure to date
was introduced inside it. Reading a field is not changing a type and calling `Assemble` is not changing
it — the condition is zero changed lines in its body.

---

## 19. Parity and provenance

**P-40.** The DiVoid node and this file are the same document, verified by **download-and-compare of
the sha256**, not by byte count — two files of equal length are not the same file. This repo's
`.gitattributes` carries `*.md text eol=lf`, so working tree, blob and node are the same LF bytes on
any machine. **No digest is quoted here deliberately:** it drifts on the next republish.

**Verify with the form that matches how this document is being delivered.** It is returned in the
**working tree and is untracked**, so a `HEAD:`-scoped command fails — `git cat-file blob
HEAD:docs/architecture/what-a-successful-run-withheld.md` returns *"exists on disk, but not in
'HEAD'"*, which is **correct** and not a defect (QA #13680 W-2). Use:

```
sha256sum docs/architecture/what-a-successful-run-withheld.md
```

against the node's downloaded bytes. **Once the operator has committed it**, the ref-scoped form
becomes the better one, because it names the form as well as the value (`*.md text eol=lf` makes the
two equal here, but that is a property of this repo and not of the number):

```
git cat-file blob <ref>:docs/architecture/what-a-successful-run-withheld.md | sha256sum
```

**Baseline.** `main` at **`872156e`**, working tree clean at the time of reading (`git rev-parse HEAD`,
`git status --porcelain` empty). **#13601 Unit 1 is on PR #70 and is not on `main`** — every claim in
§2, §8, §14 and §18 about `turn.go`, `assemble.go`, `retrieve.go` and `types.go` holds at `872156e` and
is not asserted at `987c421`.

**Graph figures — and the two populations are not interchangeable.** Every node size in this document
carries a tag, because **the live graph is not a reproducible instrument (#13592) and the run records
are**:

| tag | source | re-derive it by |
|---|---|---|
| **`(record)`** | a stored run record's own frozen dispositions — #13591, #13598, #13599, #13600, read via #13592 | reading that record; it returns the same number forever |
| **`(live 2026-09-11)`** | a read of the live graph on that date — the corpus required-set sizes in §5.1, and every `substance: null` check | re-reading the node, **which may now differ** |

**#13585 is the worked example of why the tag is not decoration:** 67,312 B `(record)` and 101,201 B
`(live 2026-09-11)`, because the node was edited 17 m 37 s after the run. **An earlier revision of
this section attributed §5.1's sizes wholesale to a live read, which made the `(record)` figure
unre-derivable by anyone following this section** — found by QA #13680 W-1, and it is the same class
§5.1 refuses two paragraphs earlier when it declines to move a substance range between sets. **The
figure under argument was held; the adjacent one that arrived unchallenged was not.**
