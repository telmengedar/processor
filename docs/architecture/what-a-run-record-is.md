# Architectural Document: What a Run Record Is

> **Repo path `docs/architecture/what-a-run-record-is.md`, DiVoid node #14054.** The two carry the same
> document byte for byte; an edit to one is not finished until the other matches it.
> **Source task:** DiVoid **#13602**, named by **#13601 §19 row 1** and deliberately not designed there.
> The *"this task ends the over-fetch bridge"* expectation this document falsifies (§13.3) is **#13602's
> own framing**, built on **#13601 §13.2**'s enumeration of three routes — not on §19 row 1, which carries
> no bridge claim.
> **Baseline:** `main` at **`e8a10b1`**, clean tree. Every `file:line` in this document was resolved
> against that tree before submission.
> **Live measurements:** taken 2026-09-16 against the production graph, in memory, reported as counts,
> offsets, similarities and key names. No node body is quoted and no file was written.
> **Standards:** Design Contracts **#1136** (§1 KISS/DRY/YAGNI load-bearing; §5 walked in §18 below),
> Code Contracts **#114 §0** — cited, not restated. Product briefing **#13534**.
> **Adjacent designs, read in full and composed with rather than superseded:** **#13238**
> (`what-goes-in-the-block.md`), **#13480** (`what-a-run-records-name-carries.md`), **#13601**
> (`the-aperture-spends-slots-admission-refuses.md`), **#13274** (the skew measurement).

---

## TL;DR

**What.** A run record node stops being a JSON dump and becomes **an account of its run, with the full
record underneath it.** The node's content is `RenderSummary`'s prose — which the system already
generates and currently hides in a field nothing reads — followed by a separator and the complete
record in a fenced ` ```json ` block. Content type becomes `text/markdown`.

**How.** `internal/divoid/write.go:62` writes that composition instead of `json.Marshal(record)`. The
account is already computed one line below it (`:68`). A one-off, deterministic, re-runnable backfill
re-composes the **39** records already in the graph from their own stored bytes — no model, no cost.

**Why, measured.** DiVoid embeds `name + content` truncated to ~8,000 characters, and a run's own
`answer` is outside that window on **39 of 39** records in the graph. So three natural-language questions
about a run's outcome return the record **nowhere in the top 60**. With the account leading the content
the same three return it at ranks **7, 48, 8**. The record is not un-admitted because admission refuses
it; it is worthless because nothing in it is reachable.

**Cost, non-zero.** Crowding gets **worse**, measured: the account-led node scores **+0.027** similarity
on a repeat of its own input. So the aperture exclusion stays and this design **extends** #13601's
over-fetch bridge rather than ending it.

**Strongest rejected alternative: drop the record and keep only the account** (best outcome ranks — 3,
12, 6). Rejected: it discards the original at the capture point, which #1220's capture-the-original
ruling forbids, and `scripts/compare.py:280` reads it.

---

## 1. Problem Statement

Every completed run files its whole `Record` as a `session-log` node's content, links it to the subject,
and sets its substance (`internal/divoid/write.go:47`–`:77`). Those nodes are now a measurable share of
the graph's own answers, and #13601 §19 row 1 filed the question they raise rather than answering it:

> **What should a run record be, now that it is a first-class competitor in its own graph?**

Toni's ruling is the frame, and #13602 is built around it:

> *"Memory is always worth something — self produced is not less worth than memory of other agents,
> should that be the case then its not memory but just noise and shouldn't even be in DiVoid. So its
> about what is the shape of the memory and does it actually contain substantial information."*

#13602 states the fork that quote opens:

- **If a run record is memory worth reading**, the defect is that `admit` refuses it unconditionally
  (`internal/loop/assemble.go:53`), and the design is about **degraded admission**.
- **If it is not**, it should not be produced in that shape, and the design is about what the durable
  record is *for*.

**Neither branch is implied by the symptom, and this document takes the first one — with a correction to
what the first one means.** §5 states the fork taken and why; §11 and §12 answer #13602's two other
"done when" clauses explicitly.

### 1.1 Success criteria

1. A run record answers a natural-language question about **its own run**, and is not merely retrievable
   as *"a run"*. Measured, not asserted (§4.4).
2. A human or agent opening the node reads an account, not a serialized struct.
3. Nothing the record carries today is lost.
4. The decision is stated in the quote's terms — shape, and substantial information — not in terms of
   slot cost.

---

## 2. Scope & Non-Scope

**In scope.**

- What the run record node's **content** is: its form, its ordering, its content type.
- What happens to the **39 records already in the graph**.
- Whether this design makes run records **admissible**, and what that does to `fuse`'s exclusion and to
  #13601's over-fetch bridge.
- The two existing invariants this change re-rules or breaks (§10.3, §10.4).

**Explicitly out of scope, each with where it lives.**

| Out of scope | Where it lives |
|---|---|
| Removing `block` from the record | **#13238 §9.5** — decided, three of four reasons standing, **not implemented**. This design composes with it and is measured both with and without it (§4.5) |
| What the record's **name** carries | **#13480** — decided, **not implemented**. Orthogonal: the name is embedded whatever the content is |
| The aperture's over-fetch multiplier and the self-produced filter at `fuse` | **#13601** — merged and implemented. This design does not touch it (§13) |
| A relevance floor on recall (`minSimilarity`) | **not filed before this document**; filed by it (§20) |
| A negation predicate on the graph's listing route | **#13604** — DiVoid-side, another repo |
| The supplementary aperture | **#13603** |
| Anything about *whether* a record is written, or where a partial write leaves a node | **`run-record-fate.md`** — a different document about a similarly-named subject. **Resolve the two by their file names before citing either** |

**Not in scope and worth saying so: this design does not change the loop's answer.** It changes what the
graph returns to a human or an agent searching it, and what a reader sees on opening a record. §15.3
states that plainly against the product briefing's own test, rather than letting the reader infer a
retrieval win this document does not deliver.

---

## 3. Assumptions & Constraints

| # | Assumption | Basis |
|---|---|---|
| A1 | DiVoid embeds `name + "\n\n" + content` as one text truncated to ~8,000 characters, name first and never truncated, content head-kept and tail-cut | **#6115** (read this session) and **#13480 §4.2** / **#13505 §1.2**, which bracket the deployed cap between 7,900 and 8,000 |
| A2 | `substance` is **not** embedded | #13241, carried as A17 by #13238. Re-confirmed indirectly this session: 17 of 39 records carry no substance at all and rank identically to the 22 that do |
| A3 | `TextContentTypePredicate` admits `text/*` **and** an allowlist of `application/*` including `json`. So the content-type change in this design does **not** change whether the node is embedded | #6115. **Stated because it is easy to assume the opposite**, and a design that silently relied on de-indexing by content type would be both wrong and dishonest |
| A4 | An embedding is regenerated on every content upload | #6115. This is what makes the backfill in §11 a plain re-upload rather than a re-index operation |
| A5 | Retrieval is reproducible run to run | #13592, via #13601 A2. Relied on in §4 only for the claim that the probe comparisons are attributable to the variant rather than to noise |
| A6 | `RenderSummary` is a pure function of `Record` and one timestamp, with no model call | `internal/loop/summary.go:41`. **This is what makes the backfill deterministic and re-runnable**, and it is why #1220's non-deterministic-backfill clause does not bind here (§11.3) |
| A7 | Nothing in the **Go** tree reads a stored run record back | Measured: the only consumers of `RunNodeType` / `RunNamePrefix` outside `write.go` are `IsRunRecord`'s callers, and every one of them **excludes**. `cmd/eval/sweep.go:54` recomputes retrieval live rather than reading stored records |
| A8 | **One consumer outside the Go tree does parse the stored body**, and it fails silently if it cannot | `scripts/compare.py:280` — `json.loads(row.get("content"))` inside a `try` whose `except JSONDecodeError` is `continue`. **A7 is true and would have been a false universal without A8**; §10.4 carries the consequence |

**Constraint.** The model is an HTTP endpoint on `gangolf:11434`. Nothing in this design calls it —
`RenderSummary` is a template (A6), and that is load-bearing for §11.

---

## 4. The measurements this design rests on

All taken 2026-09-16 against the live graph, in memory. **Re-derived rather than inherited from #13602**,
because the corpus grows by one record per run. Where a figure differs from #13602's, both are given.

### 4.1 The corpus, re-derived

| fact | #13602 (2026-09-11) | **measured 2026-09-16** |
|---|---|---|
| run records in the graph | 37 | **39**, created 2026-09-02 … **2026-09-12** |
| direct neighbours of subject #10422 that are run records | 19 of 57 | **21 of 81** |
| stored content size | 37,163 – 77,554 B, median 66,804 | **37,163 – 107,487 B, median 71,102**, sum 2,837,179 |
| records carrying a substance | — | **22 of 39**, median 1,953 B (17 predate the write path) |

**Two corrections to figures #13602 states as trends, both in the direction that lowers urgency:**

1. **The neighbour *share* is not rising monotonically.** The count is (19 → 21); the share fell from
   **33.3 % to 25.9 %**, because the subject gained more documents and tasks than records over the same
   five days. *"Rising monotonically and shrinking never"* is true of the count and false of the share,
   and the share is what an aperture argument needs.
2. **No run has executed since 2026-09-12.** #13601 §13.2 sized its bridge's remaining life at **2.2
   days** on a 20-runs-per-day peak. Four days have since passed with **two** new records. The bridge is
   not expiring on that schedule, and §13.3 restates its life on the measured rate rather than the peak.

### 4.2 Where the bytes are, and it is not where the ranking is

Per-key share of the whole 2,837,179-byte corpus, by the encoded size of each top-level value:

| key | bytes | share |
|---|---|---|
| `block` | 2,195,742 | **77.4 %** |
| `candidates` | 302,912 | 10.7 % |
| `toolCalls` | 200,028 | 7.1 % |
| `answer` | 29,956 | **1.1 %** |
| everything else (17 keys) | 108,541 | 3.8 % |

**The run's own conclusion is 1.1 % of what the run stores about itself.** And per A1 the ranked surface
is the first ~7,880 characters of content, where `answer` never appears at all. **Resolved across all 39
records**, each against its own content budget (8,000 − its name − 2, giving 7,880 – 7,910):

| | measured, 39 / 39 |
|---|---|
| records whose `answer` begins **inside** the budget | **0 of 39** |
| records whose `block` begins **inside** the budget | **17 of 39**, at offsets 5,457 – 7,151 |
| records whose `block` begins outside it | 22 of 39, at offsets 8,250 – 10,476 |

The `answer` row reproduces **#13480 §5.2** independently and widens it from one specimen to the corpus.

> **The `block` row does not reproduce a companion claim, and the disagreement is stated rather than
> smoothed over.** #13480 §5.4 and #13238 §9.5 clause 1 both rest on `block` contributing *"exactly zero"*
> to the embedding. That is measured on **#13472**, where `block` begins at 8,360, and it holds for the
> **22** records shaped like it; on the other **17** a tail of `block` is inside the window. **This does
> not reopen #13238 §9.5 reason 4** — clause 2 fails independently of clause 1, on #13480 §5.4's own
> arithmetic — but *"exactly zero"* is a property of one record's shape rather than of run records, and
> §20 carries it back. This document's decisive specimen #13718 has `block` at **10,476**, outside on
> both counts.

### 4.3 What each shape costs against the two byte budgets

`AssemblyByteBudget = 60_000`, `SupplementaryByteBudget = 20_000` (`internal/loop/turn.go:15`, `:18`).

| shape | min | median | max | over 60 kB | over 20 kB |
|---|---|---|---|---|---|
| as stored | 37,163 | 71,102 | 107,487 | **32 / 39** | **39 / 39** |
| minus `block` (#13238's decided end state) | 7,999 | 10,449 | 48,871 | 0 / 39 | 4 / 39 |
| minus `block`, `candidates` and `toolCalls` — **the account's data** | 893 | 1,781 | 6,113 | 0 / 39 | 0 / 39 |

**And the account fits the ranked window for every record in the corpus.** Serialized as a leading object,
the identity fields occupy **890 – 6,096 B, median 1,778** against a content budget of ~7,880 characters:
**39 / 39 fit**, the largest at 77 % of budget.

### 4.4 The decisive experiment — seven shapes of one record, name held constant

**Method, published as run, and every arm is published as a recipe so every one is rebuildable.** Nodes
were created carrying **#13718's exact name** (so the name's contribution to the embedding is identical
across arms) and differing content; DiVoid re-embedded each on content upload (A4); four queries were
issued against `GET /api/nodes?query=…&count=60`; **every probe node was then deleted and the corpus
verified back at exactly 39 run records.**

**Probe accounting, in full.** This re-run allocated **7** ids (14063–14069, one per arm — including A,
so that every arm sits in the same population) and the cliff check in finding 1 a further **1** (14070).
An earlier round allocated **7** more, spanning three experiments rather than one: **14047–14048** on a
different specimen (#13034, a reordering test whose results this document does not carry), **14049, 14050,
14052, 14053** for arms B, C, D and E on #13718, and **14051** for §4.5's prose twin of #13599 — in that
round arm A was read from the live node rather than probed. **Fifteen node writes to the shared graph
across this design, every one deleted, the corpus verified at 39 after each round.**

*An earlier revision of this section said "all five probe nodes were then deleted" — it was counting the
arms in its own table rather than the writes it had made. The cleanup was complete and the accounting was
not, and the accounting is what a reader quotes.*

**#13718 is the specimen because its outcome is distinctive**: `stopReason: wantsRecall`, 6 model calls,
**empty answer** — a run that spent its call cap still asking for a tool. Its stored content is
**107,460 B**.

#### The arms, each as a recipe

Let `rec` be #13718's stored content parsed as a `Record`, `sub` its stored substance, and

```
ACCOUNT = input, subject, now, query, queries, derivationError, window, anchor,
          answer, stopReason, model, provider, modelCalls, capReached, workspace,
          usage, limits, sampling
```

— every top-level key of `Record` **except** `candidates`, `toolCalls` and `block`, in that order. On
#13718 four of the eighteen (`now`, `derivationError`, `window`, `workspace`) are absent from the stored
record and are therefore absent from the arm.

| arm | recipe | size | sha256 (first 12) |
|---|---|---|---|
| **A** | #13718's stored content, byte for byte | **107,460 B** | `caed978ba109` |
| **B** | `ACCOUNT` re-serialized as one JSON object in that key order, with `", "` / `": "` separators | **1,725 B** | `23a7ec3d1845` |
| **B-0** | the same, with compact `","` / `":"` separators — **what Go's `encoding/json` actually emits** | 1,645 B | `3bf582c56f25` |
| **B-plus** | `ACCOUNT` **plus `candidates` and `toolCalls`**, same serializer as B | 48,848 B | `026cc8e6ab08` |
| **C** | `sub` — `RenderSummary`'s own output, taken from the node's substance | 3,110 B | `6be3e3e431f3` |
| **D** | `sub`, then a `---` line, then A inside a ` ```json ` fence | 110,590 B | `0884e6ad4976` |
| **E** | as D, with `block` replaced by `blockBytes` in the fenced record | 51,999 B | `d1b38d876e9e` |

> **Arm B is published as a recipe because an earlier revision published only its size**, and a reviewer's
> good-faith reconstruction at 1,741 B then disagreed with it on one query (#14062 CF-2). Arm B is the only
> thing standing between this design and the cheap alternative, so it owed the same reconstructibility the
> other four already had. **The re-run below settles the disagreement**; the recipe is what stops anyone
> needing to reconstruct it again.

#### The results, re-run 2026-09-16 with all seven arms present

| query | A | B | B-0 | B-plus | C | D | E |
|---|---|---|---|---|---|---|---|
| **Q1** *"a run that used up all six model calls and still wanted another recall, producing no answer at all"* | — | 0.6491 | — | — | **0.6869** | 0.6701 | 0.6678 |
| **Q2** *"which runs ended by hitting the model call cap instead of answering"* | — | — | — | — | **0.6586** | 0.6399 | 0.6391 |
| **Q3** *"a run where twenty candidates were retrieved and only seven were admitted, thirteen cut"* | — | — | — | 0.6573 | **0.6782** | 0.6763 | 0.6735 |
| **Q4** a **verbatim repeat of the run's own input** | 0.7451 | 0.7671 | **0.7810** | 0.7592 | 0.7626 | 0.7717 | 0.7710 |

**`—` means the arm was not among the 60 rows the query returned.** Ranks in that pass: Q1 — C 3, D 7,
E 9, B 29; Q2 — C 12, D 48, E 49; Q3 — C 6, D 8, E 10, B-plus 58; Q4 — B-0 1, D 2, E 3, B 4, C 5,
B-plus 6, A 8.

> **Ranks are stated with the probe set they were taken in and similarities are not.** Arm B sat at rank
> 28 with five arms present, 29 with seven and 30 with eight, while its similarity stayed **0.6491** at
> four decimal places throughout. A rank is a property of the population; a similarity is a property of
> the pair. **Quote the similarity.**

**Every cell of the earlier five-arm revision reproduced to four decimal places** — A, B, C, D and E on
all four queries — on a fresh set of probe nodes. The reviewer independently reproduced A, C and D to the
same precision, with arm A bit-identical to the live record.

#### Three findings, and the second is the one that constrains the design

1. **Form decides findability, not order and not size.** Arm B is the account's own fields hoisted to the
   head of the window, **62.3x smaller than A** (107,460 / 1,725) — and it reaches **one** of three outcome
   queries. Arm C reaches all three, inside rank 12. **JSON key–value pairs do not embed against a
   natural-language question about what happened**; `"stopReason":{"reason":"wantsRecall"},"modelCalls":6`
   is not near *"hit the model call cap"*, and `"answer":""` is an **absence**, which has no text to embed
   at all. A design that merely reordered the struct would have been measured and wrong.

   **And the faithful version of that alternative is worse, not better.** Arm B used Python's default
   `", "` / `": "` separators; **Go emits compact JSON**, which is arm **B-0** — and B-0 reaches **zero** of
   the three outcome queries while posting **the highest crowding figure of all seven arms** (0.7810,
   rank 1, on a repeat of its own input). Measured as the code would actually produce it, R-2 is strictly
   worse than the arm originally used to reject it.

   **What was tested against the reviewer's contrary reconstruction, and what it showed.** Three
   constructions neighbouring B were measured: compact separators (**B-0**, above); `ACCOUNT` plus the four
   zero-valued keys Go omits (+145 B — Q1 moved +0.0008 to 0.6499, Q3 still absent); and `ACCOUNT` plus the
   assembly arrays (**B-plus**, 48,848 B — Q3 at **0.6573, rank 58**, inside the set and still 0.0209 below
   arm C). **None reaches Q3 above rank 58**, and the +145 B perturbation shows Q3 is not sitting on a cliff
   that a small body change tips. So the reconstruction that scored 0.6841 at rank 4 carried material no
   reading of *"the account's fields, still as JSON"* produces — **the divergence is in the reconstruction,
   not in the arm.** Which bytes differed I cannot know, and the recipe above is what makes the question
   unnecessary rather than answerable.

   **The reviewer's numerals hypothesis, measured rather than inherited.** #14062 CF-2 asks whether Q3's
   numerals (`20`, `7`, `13`) place it inside the falsifying class §17 names for this document's own
   universal. **Arm B contains neither 7 nor 13** — the admitted and cut counts exist only inside
   `candidates`, which B excludes; B carries only `limits.candidateLimit: 20`. **B-plus carries all three
   and reaches Q3 at rank 58.** So numerals in JSON do measurably help on Q3 and come nowhere near the
   prose arm. §17 carries the consequence.
2. **Every new shape crowds *more* on a repeat of its own input, not less.** A to C is **+0.0175**; A to D
   is **+0.0266**; A to B-0 is **+0.0359**, the largest of the set. Dilution was doing protective work.
   §12 and §13 are built on this.
3. **`block`'s presence is worth ≤ 0.003 similarity on all four queries** (D vs E: 0.0023, 0.0008, 0.0028,
   0.0007). On **#13718** that is because `block` begins at character 10,476, outside the window — but §4.2
   measures that this holds for only 22 of 39 records, so the finding is stated of this specimen and not of
   run records generally.

### 4.5 A second specimen, because one is one sample

Arms A and C were repeated on **#13599** — the record that ranks **first** on the ill-matched input the
product briefing §11 measured (*"write 17 × 23 into a file"*). **Same recipes as §4.4**, same name, content
swapped for the account: A is #13599's stored content, **38,638 B**, read from the live node; C is its
stored substance, **1,654 B** (1,626 characters), written to one probe node — **#14051**, inside the
14047–14053 block accounted for in §4.4.

| query | A (38,638 B JSON) | C (1,654 B prose) | delta |
|---|---|---|---|
| the ill-matched input | 0.6641 | 0.7929 | **+0.1288** |
| a verbatim repeat of #13599's own input | 0.6933 | 0.8206 | **+0.1272** |
| the generic *"a run"* query of #13274 | 0.7143 | 0.7301 | +0.0158 |

**Finding 2 holds across both specimens, at magnitudes from +0.016 to +0.129.** Bounded claim: *on the two
specimens measured, replacing the JSON body with the prose account raised the record's similarity to a
repeat of its own input in every case.* **What would falsify it:** a record whose account happens to
paraphrase its input less faithfully than the JSON's verbatim `input` field does — none was found, and
only two were tested.

### 4.6 The aperture today, re-derived

On **14** records' own inputs, the unscoped top 20 held **9 – 19** run-record rows. At `fetch = 100`
(#13601's over-fetch), run records took **19 – 34** of 100 across 8 sampled inputs, and **20 real rows
were available in every one** — so the bridge is holding at 39 records, with its own falsifier (*"more than
80 of the top 100"*) untouched.

---

## 5. The fork, taken

**Fork A: a run record is memory worth reading.** And the correction that goes with it:

> **The defect was never that admission refuses the record. The defect is that the node is not shaped as
> memory, so there is nothing in it for admission to be right or wrong about.**

Read against Toni's two criteria, field by field — which is the only honest way to read them, because the
answer is not uniform over the record:

| | shape | substantial information? |
|---|---|---|
| **the run's own account** — what was asked, what was retrieved, what was admitted and cut and why, how it ended, what it concluded | today: JSON fields at offsets 65,000–76,000, **outside the ranked window**, rendered to a reader as a struct | **Yes — and unreachable.** Three questions about it return the record nowhere in the top 60 (§4.4) |
| **`block`** — 77.4 % of the corpus | verbatim bodies of nodes the record itself addresses by id | **No.** Toni: *"duplicating content in a graph in general is complete nonsense — that's what edges are for."* Already ruled: #13238 §9.5 |
| **`candidates` + `toolCalls`** — 17.8 % | ids, similarities, hashes and **twenty other nodes' names** | **Not as memory.** It is instrument output, and `candidates` alone fills **93.2 % of the embedded content budget** today (#13480 §5.3 — 93.2 % is against the 7,880-character *budget*; against the 8,000-character *window* it is 91.8 %, and §5.3's own [R2] correction exists because those two were once quoted against each other. `toolCalls` sits outside the window entirely, per #13480 §5.2) — so the record's semantic identity is a summary of the twenty nodes it looked at |

**So the quote's own test separates the artifact into two.** A run record is *one node holding two
artifacts*: an **account**, which is memory, and a **measurement**, which is an instrument's output. A
node can only be one thing to the index, and today it is the second — which is exactly Toni's *"not
memory but just noise."*

**Why this is fork A and not fork B.** Fork B says the record *should not be produced in that shape*, and
this design changes the shape — so the branches touch. The distinction that decides it is what the
*durable artifact* is for. Fork B's reading is that the record is a forensic blob and the graph should
hold an operator-facing account **instead**. This design says the record is **memory whose account was
buried**, and keeps both: the account becomes the node, and the measurement stays under it, whole.
**Nothing is discarded, and the thing that becomes retrievable is the memory that was always there.**

**And the fork's own premise is corrected, not inherited.** #13602 offers "degraded admission — the
substance instead of the body" as fork A's mechanism, noting the measured asymmetry that run records
already carry a ~2.2–2.4 KB cheaper form and nothing reads it. **That asymmetry is real and this design
acts on it — one layer up from where the fork put it.** Degraded admission would hand the *loop* the
account while the *graph* went on ranking the node on a catalogue of twenty other nodes: the record
would still be retrieved as "a run", still be unfindable as itself, and still render to a human as a
struct. **The account should not be a degraded form of the record. It should be the record's face.**

---

## 6. Architectural Overview

One node, one content, two layers, in the order the index reads them.

```
  session-log node "processor-run <RFC3339> — <…>"
  ├── name         ──────────────────────────────►  always embedded, never truncated   [#13480 owns this]
  ├── content      text/markdown
  │   ├── (1) THE ACCOUNT      RenderSummary(record, at)      ~0.9 – 6.1 kB
  │   │        what was asked · the subject · the model and limits
  │   │        ASSEMBLY: queries, admitted, cut and why
  │   │        TOOLS:    the round sequence
  │   │        OUTCOME:  terminal reason, calls, cap, the answer
  │   │                                      ─────────────────►  inside the ~7,880-char window, at its head
  │   ├── ---
  │   └── (2) THE RECORD       ```json { … } ```                the complete Record, unchanged
  │                                            ─────────────►  head of it is inside the window; tail is cut
  └── substance    RenderSummary(record, at)                    not embedded (A2); read by #13238's form rule
```

**What changed is one composition and one content type.** No new type, no new package, no new port, no new
configuration. The account is already computed at `internal/divoid/write.go:68`; this design computes it
once and uses it twice.

---

## 7. Components & Responsibilities

| Component | Owns | Does **not** own |
|---|---|---|
| **`RenderSummary`** (`internal/loop/summary.go:41`) | rendering a `Record` as the run's account: a deterministic, model-free projection of the record's own fields | the record's contents; the block (it reads `len(Block)` only, and a guard at `summary_test.go:56`–`:61` holds it to that) |
| **`WriteRun`** (`internal/divoid/write.go:47`) | composing account + separator + fenced record; declaring the content type; filing, substancing and linking the node | what the account says; what the record contains |
| **the backfill operation** (§11) | re-composing existing records' content from their own stored bytes, by selector, re-runnably | producing any content the write path would not have produced |
| **`admit` / `fuse`** (`assemble.go:53`, `retrieve.go:90`) | refusing self-produced rows, **unchanged by this design** | anything about the record's shape |

**Single responsibility, stated where it is easy to blur:** `RenderSummary` is the *only* place that
decides what the account says. This design does not add a second renderer, a second template, or a
"content" variant of the summary. If the account is wrong, there is exactly one function to fix.

---

## 8. Interactions & Data Flow

The write-back, at the end of a turn, with the one step that changes marked:

1. `WriteRun` receives the completed `Record`.
2. It reads the clock once (`at`) — unchanged.
3. It creates the node with its name — unchanged, and **#13480 owns what that name says**.
4. **It renders the account once: `RenderSummary(record, at)`.** Today this happens at step 6.
5. **It composes the content: account, a `---` separator, then the record inside a ` ```json ` fence, and
   posts it as `text/markdown; charset=utf-8`.** Today this posts `json.Marshal(record)` as
   `application/json` (`write.go:62`, constant at `:20`).
6. It sets the substance to **the same rendered account** — unchanged in value, now reusing step 4's
   result instead of computing it again.
7. It links the node to the subject — unchanged.
8. It returns the same three-state write receipt — unchanged.

**The failure ordering is untouched**, and that matters: a failed content write still discards the shell
and reports `notStored`; a failed substance write is still logged and does not fail the run; a failed link
still yields `unlinked`. This design adds no new failure mode to a path whose failure modes are settled by
`run-record-fate.md`.

**The read side has three consumers and they diverge for the first time here:**

| Reader | Reads | After this change |
|---|---|---|
| DiVoid's embedder | `name` + the first ~7,880 characters of content | gets the account and the head of the record, instead of twenty other nodes' names |
| a human or agent opening the node | the content, rendered | reads prose; the record is below it, folded into a code block by any markdown renderer |
| `scripts/compare.py:280` | the content, parsed as JSON | **breaks silently unless changed in the same PR** — §10.4 |

---

## 9. Data Model (Conceptual)

**No entity changes.** `Record` keeps every field it has, with the same names and the same meanings. This
design changes the **encoding of one node's content**, not the model.

Stated explicitly because it is the cheap wrong version of this design: **`Record` does not gain an
`account` field, and `RenderSummary`'s output is not persisted as a member of the record.** The account is
a projection, computed at write time from fields the record already carries; storing it inside the record
would make the record contain its own summary, which is the duplication this whole document is against.

Two entities, one node:

| | The account | The record |
|---|---|---|
| **is** | a projection of the record | the original |
| **lives** | the head of the content, and the substance | the tail of the content, fenced |
| **for** | the index, a human, and #13238's form rule | forensics, and any future harness that wants dispositions |
| **derived from** | the record, by a pure function (A6) | nothing — it is the capture |

**Direction of derivation is the load-bearing property**, and it is #1220's capture-the-original ruling
applied at a capture point: the account is computable from the record forever; the record is not
recoverable from the account. So the record is what is stored and the account is what is derived — even
though the account is the part anyone reads.

---

## 10. Contracts & Interfaces (Abstract)

### 10.1 The content contract

> **A run record node's content is: the account, then a line containing exactly `---`, then the complete
> record as a single fenced ` ```json ` block, which is the content's last fenced block.**

Three properties an implementer must hold, each falsifiable:

| # | Property | How it is checked |
|---|---|---|
| C1 | The content's **last fenced `json` block**, parsed, is a complete `Record` | a test that round-trips a composed content back to a `Record` and compares every top-level member |
| C2 | The content **begins** with the account — no preamble, no title line, nothing before it | a test asserting the composed content has `RenderSummary`'s output as a prefix |
| C3 | The account in the content and the account in the substance are **the same string** | a test comparing the two, which is free because both come from one call |

**C1 is the contract every machine reader keys on**, and it is stated as *last* fenced block rather than
*only* fenced block so that an account which ever contains a fence cannot break it. Today's account
contains none; the rule costs nothing and removes a class of future breakage that would be silent.

### 10.2 The content type

`text/markdown; charset=utf-8`, replacing `application/json` (`write.go:20`).

**This does not change whether the node is embedded** (A3) and the design must not be read as if it does.
It changes what the type *claims*, and the claim currently becomes false the moment the account is
prepended. A node whose declared type does not parse is worse than one with an inconvenient type.

### 10.3 The stored-vs-response invariant, re-ruled

`cmd/processor/artifacts_test.go:163`,
`TestTheStoredBodyIsTheResponseBodyMinusTheWriteReceiptAndNothingElse`, enforces #10904 §8.1:

> *the stored node's body and the HTTP response body carry the same record, byte-for-byte, in every key
> that describes the run; the response carries exactly one key more, the write receipt.*

**#13238 §9.6 already amends this** (response keeps `block`, stored body carries `blockBytes`). This design
amends it a second time, and **the property is preserved rather than weakened**:

> **The stored content's last fenced `json` block, parsed, is the response body minus exactly the write
> receipt.**

The guard is renamed and extended, not deleted: it extracts the fence first and then runs the same
member-by-member comparison it runs today, so a **third** divergence still reddens it. **What the
invariant was protecting — that the record served and the record stored never differ silently — is
untouched.** What changed is that the stored body is now a document containing the record rather than
being the record.

**Interaction with #13238, stated because the two amendments must not collide:** they are independent and
composable in either order. #13238 changes *which keys* the stored record has; this design changes *where
in the content* the record sits. An implementer landing the second of the two amends one test twice.

### 10.4 The one external reader, and it fails silently

`scripts/compare.py:280` parses a recalled node's content as JSON to find a prior run of the same task
text, and its `except json.JSONDecodeError: continue` means a content it cannot parse is **skipped without
a message**. `refuse_repeats_against_graph` then stops refusing repeats — a guard whose whole purpose is to
stop a second run of the same text from being counted as a second measurement.

> **This must change in the same PR as the write path.** `find_prior_run` extracts the last fenced `json`
> block before parsing, and falls back to parsing the whole body so that records not yet backfilled are
> still found.

**And the fallback is not optional while §11's backfill is a separate unit**: between the two PRs the graph
holds both shapes, which is the intended operating mode rather than a migration window.

**Property to sweep for, rather than the two sites I found** (#1220's 2026-09-10 addendum): *every reader
that turns a run record node's content into structured data must go through the fence*. I located two —
`compare.py:280` and the invariant test at `artifacts_test.go:163`. **Do not trust that list to be
complete**; re-derive it with

```
grep -rnE "json\.(loads|Unmarshal|NewDecoder|Decoder|Delim)|JSONDecodeError" scripts/ cmd/ internal/
```

and by reading every call site that reaches a `session-log` body. **Run at `e8a10b1` this returns 103 rows
and reaches both named sites.** *An earlier revision published this without the decoder alternatives; that
form returns 94 rows and **zero** from `cmd/processor/artifacts_test.go`, which parses with
`json.NewDecoder` / `dec.Decode` / `json.Delim` (`:133`–`:154`) and matches none of the three original
patterns — so the published command could not see one of the two sites this very section names.*

---

## 11. The 39 records already in the graph

#13602's second "done when": *whatever is chosen states what happens to the records already in the graph,
which are not rewritten by a change to the write path.*

**They are backfilled.** Leaving them is not neutral: they are the corpus — 21 of subject #10422's 81
direct neighbours (§4.1) — and every crowding and skew measurement this project takes is taken against
them. A write-path change alone would leave the measured defect in place for every record that exists and
fix it only for records that do not yet.

### 11.1 The operation

A **scoped, re-runnable** operation, per #1220's backfill addendum:

| Property | Answer |
|---|---|
| **Selector** | a set of node ids, or *all* `session-log` rows whose name carries the run prefix. **Scope is a parameter**; the first invocation is narrow |
| **Per record** | fetch content → parse as `Record` → render the account with the timestamp **the node's own name carries** → re-post the composed content as `text/markdown` → re-post the same account as substance |
| **Collision rule** | a record whose content already satisfies C1 and C2 is **skipped**, unless forced. Re-running over ground already covered is therefore free and safe |
| **Cost** | **zero model calls** (A6). Two HTTP writes per record; 39 records today |
| **Re-embedding** | automatic on content upload (A4). Nothing re-indexes anything by hand |
| **First cohort to judge** | the **17 records that carry no substance** — the ones written before the summary path shipped. They are the arm where the account has never existed, so rendering it is the whole change and a reader can check it against the record below it |
| **Criterion for widening** | a human who did not write it reads three backfilled accounts against their records and finds every count and figure correct. That is #13238 Unit 3a step 4's bar, and the same bar applies here because it is the same renderer |

### 11.2 What "not processed yet" looks like, and it is a first-class state

Between the two PRs — and after, if the selector stays narrow — **the graph holds both shapes at once, and
that is the intended operating mode rather than a window to be endured.** A reader distinguishes them by
C1: a record whose content parses whole as JSON has not been backfilled. §10.4's fallback exists precisely
so no consumer has to know which is which.

### 11.3 Why #1220's non-determinism clause does not bind

That addendum is written for a backfill that re-runs an **LLM extraction** — where two runs over the same
input can disagree and the second answer is not automatically better. **`RenderSummary` is a template**
(A6): the same record and the same timestamp produce the same account, byte for byte, forever. So the
clause's collision hazard does not arise, and the operation needs no *"re-processed only if the source
changed"* rule. **Stated rather than assumed, because the addendum's other clauses — scope as a parameter,
a judgeable first cohort, a stated widening criterion — do bind and are answered above.**

### 11.4 What backfilling cannot recover

**The name.** #13480's decision — that the name carries the outcome rather than the input — is about text
that is written at node-create time. A content backfill does not touch it, and a **name** backfill is
#13480's to specify, not this design's. Until it runs, backfilled records carry an account about their
outcome under a name about their input. That is strictly better than today and it is not the end state.

---

## 12. Why a node that outranks real content on its own input is right to keep embedding

#13602's third "done when", and the sharpest of the three. The record stays large — arm D is the whole
record plus the account — and §4.4 measures arm D as the **worst** crowder of the five on a repeat of its
own input (0.7717 against the stored shape's 0.7451). So the question is owed a real answer.

**The answer has two parts, and the first is a correction to the question's premise.**

### 12.1 It does not outrank real content on its own input. It outranks it on a *repeat* of its own input — and on that query it is not a competitor

A node that says *"this exact question was asked before; here is what was retrieved, what was cut, and
what was concluded"* is, on a repeat, **the single most on-point thing the graph holds.** Ranking it first
is not a defect of the ranking; it is the ranking working. That is Toni's ruling read literally: *memory is
always worth something, and self-produced is not worth less.*

**The falsifying input class, and it is the paragraph you are reading** (#1220 §5): the repeat is being asked
*because the first run's answer was wrong*, or because the graph has changed since. Then a top-ranked
record teaches the model the stale answer with the authority of memory. **Two things bound it, and only
the second is delivered by this design:**

- The record is still **excluded from the loop's own aperture** (§13), so the model never reads it at all.
- When the exclusion is eventually relaxed, the account states the terminal reason, the call count, the
  admitted/cut split and the answer's size **in the ranked window** — so a failed run reads as a failed
  run. **Today none of that is embedded or rendered**, which is precisely why the hazard is invisible now.

### 12.2 The measured defect is a different one, and it is not about the record's shape

The product briefing §11 measured the case that actually hurts: on an input the graph answers badly
(*"write 17 × 23 into a file"*), run records took **17 of 20** slots, and this session's re-derivation gives
**19 of 20** at 39 records, in a similarity band of **0.6164 – 0.6641**.

**There is no displaced real content in that result.** The one real row in the top 20 is still there. What
the band shows is that *nothing in the graph matches*, and the ranking returns its best twenty anyway.

> **That is an absent relevance floor, not a badly shaped node.** DiVoid's listing route already supports
> a `minSimilarity` floor (#6115) and this product does not use it. A shape change cannot fix it, a
> provenance rule only hides it, and the same defect would return with any other class of node that
> happened to sit at 0.62.

**So: embedding stays** — because the record is genuinely the best answer to a question about itself, and
because the case that looks like poisoning is a floor problem filed in §20 rather than a reason to hide the
node from the index.

### 12.3 The one thing that would change this answer

If a run record were ever admitted **and** the loop had no relevance floor, §4.5's +0.13 would be paid on
every ill-matched input. **That combination is the hazard**, and this design does not create it: it
changes the shape and leaves the exclusion standing. §13 says so in the terms #13602 asks for.

---

## 13. The coupling #13602 names, answered directly

### 13.1 Does this design trip `TestFuseExcludesExactlyWhatAdmitWouldCutAsSelfProduced`? No

G-4 (`internal/loop/retrieve_test.go:624`) asserts that the set `fuse` excludes and the set `admit` refuses
as self-produced are **the same set**. It exists so that the day admission stops refusing these rows,
the aperture cannot go on excluding rows the block would then accept.

**This design changes neither site.** `assemble.go:53` keeps its `SelfProduced` arm; `retrieve.go:90`
keeps its exclusion; `client.go:190` keeps setting the flag from `IsRunRecord`. The two sets stay
identical by construction and **G-4 stays green without being touched.**

**What would trip it:** removing the `SelfProduced` arm from `admit` without removing the exclusion from
`fuse`, or the reverse. Any future change that grants run records admissibility must do both in one
commit — which is exactly what G-4 was built to force, and this design leaves that force intact.

### 13.2 Does this design make run records admissible? No — and that is a decision, not an omission

**Fork A says a run record is memory. It does not follow that it is worth an aperture slot**, and §4's
measurements say it is not yet:

- A compact account is a **stronger** competitor than the JSON blob it replaces (+0.016 to +0.129, §4.5).
- On the arm the product briefing measured as worst, 19 of 20 rows are already run records (§12.2).
- So relaxing the exclusion now converts a provenance defect into a relevance defect, on exactly the
  inputs where the few real rows that exist matter most. **The briefing's own finding is that the crowding
  is anti-correlated with the memory being useful**, and nothing in this design changes that correlation.

**What it does change is that the question becomes answerable on form rather than on provenance.** After
this design and #13238 Unit 3, a run record has a substance materially smaller than its content, so
#13238 §8.1's form rule admits it in substance form **with no special case and no rule of the shape
"nodes we wrote are treated differently"** — which is what #13238 §8.3 wanted. The remaining gate is the
relevance floor in §20, and it is a gate on the *aperture*, not on the record.

### 13.3 Does this end #13601's over-fetch bridge? No — it extends it, and the premise that it would end it is falsified

**#13601 §13.2** enumerates three routes that would end the bridge — a graph-side negation predicate, a
change to what a run record is, or a two-phase fetch — and adds *"None of them is this task"*, meaning none
was #13601's own scope. **#13602** then takes that enumeration and states the expectation directly: *"this
task is one of the three things that ends it."* **That is the sentence the measurement falsifies, and it is
#13602's, not #13601 §19 row 1's** — row 1 carries the record-size and admit-vs-produce fork and no bridge
claim at all. The finding belongs back at both nodes rather than only here.

The premise is that a better-shaped record becomes admissible, so nothing needs excluding, so no over-fetch
is needed. **The premise's first half holds and its second does not:** the better-shaped record is a
*stronger* competitor (§4.4 finding 2), so the exclusion becomes more load-bearing, not less — and the
over-fetch that makes the exclusion affordable stays with it.

**What this design does give the bridge is time, measured rather than assumed.** #13601 §13.2 sized the
remaining life at **2.2 days** on a 20-runs-per-day peak, against its own falsifier — *an input for which
more than 80 of the top 100 rows are run records*, which needs **81 records** in the graph. Four days have
passed and the corpus grew by **two** (37 → 39, §4.1), an observed rate of **0.5/day**; the **42 further
records** that falsifier now needs are **~84 days** away at that rate. **Both figures are honest and they
answer different questions**: 2.2 days is what a burst can do, 84 days is what has happened.
*(#13601 §13.2 also states a separate figure — an over-fetch headroom of **80 rows**, being fetch 100 minus
limit 20. An earlier revision of this paragraph fused the two into a "44-record headroom", which is neither
document's quantity: 44 was the record shortfall measured when 37 existed, not a headroom.)*

A bridge sized against bursts is sized correctly; a bridge *reported* as having 2.2 days left when it has
not moved in four is a figure that will be quoted into a brief. #13601's WARN detector is what makes
either number safe to be wrong about.

---

## 14. Cross-Cutting Concerns

| Concern | Position |
|---|---|
| **Security / secrets** | Unchanged. The record's credential-carrier redaction (`internal/redacturl`) runs before `WriteRun` and this design does not move it. **The account is a projection of the already-redacted record**, so it cannot introduce a carrier the record does not have — but an implementer must not add a field to the account that reads from anywhere other than the `Record` it is given |
| **Error handling** | Unchanged (§8). No new failure mode; the composition is in-process string building and cannot fail |
| **Observability** | Unchanged. The existing four log sites in `WriteRun` keep their meanings |
| **Idempotency** | The backfill is idempotent by C1/C2 (§11.1): a record already in the new shape is skipped |
| **Consistency** | The account and the record on one node are written in one content POST, so they cannot diverge. The account and the **substance** are two writes and can diverge if the second fails — which is already true today, is already logged as `run record stored without its summary`, and is now *detectable*, because a reader can compare the substance against the content's own head (C3) |
| **Concurrency** | None introduced. The backfill is a sequential operation over a node list |
| **Caching** | None |

---

## 15. Quality Attributes & Trade-offs

### 15.1 Rejected alternatives, each with why

| # | Alternative | Why not |
|---|---|---|
| **R-1** | **Keep only the account; drop the record from the graph** (arm C — the best outcome ranks by a clear margin: 3, 12, 6) | It discards the original at the capture point, which #1220's *capture the original, derive the compressed form* ruling forbids in terms. The account is computable from the record forever; the record is not recoverable from the account. And it is not consumer-free: `compare.py:280` reads the stored record (A8), and #10904 §9.4 obligation 1 makes the dispositions the reason recall@k is computable retroactively at all. **Cost of rejecting it, stated per query rather than as a range: C − D on the three outcome queries is 0.0168, 0.0187 and 0.0019** — three ranks on each, and the third is an order of magnitude smaller than the other two. *An earlier revision stated this as "0.017–0.019", which is the two larger deltas rounded and silently drops the third. The error ran **against** this design's own position — it overstates the price of the alternative the design rejects — which is why nothing internal caught it, and correcting it moves Q1's balance toward R-1.* |
| **R-2** | **Reorder `Record`'s struct fields so the account's data leads the JSON** (arms B and B-0) — free, lossless, one struct | **Measured insufficient, and the faithful form is worse.** Arm B reaches **one** of three outcome queries; arm **B-0**, which uses the compact separators Go's `encoding/json` actually emits, reaches **none** — while posting the highest crowding figure of all seven arms (0.7810). Form decides findability, not order (§4.4 finding 1). This is the alternative a reasoning-only design would have chosen, and the probe is why it was not. **Both arms are published as recipes in §4.4** so this rejection is re-runnable |
| **R-3** | **Put the record in the node's `substance`**, which is not embedded, and the account in the content | Inverts `substance`'s graph-wide meaning — DiVoid defines it as the *condensed* form of the content, and every other agent and #13238's own form rule reads it that way. A local dodge that breaks a shared contract |
| **R-4** | **Declare a content type outside DiVoid's embeddable allowlist** so the node ranks on its name alone | Two objections and either is fatal: it is a false declaration about bytes that really are JSON, and it makes a Processor decision by mis-stating a fact to a shared substrate. If run-record content should not be embedded, that is a DiVoid-side rule to propose openly (§20) |
| **R-5** | **Cap how many self-produced rows may hold aperture slots** | It is still a provenance rule, and it is aimed at the wrong quantity. §12.2's measurement says the binding defect is relevance, not authorship — and a cap would leave the same 19 bad rows in place on an ill-matched input, just fewer of them |
| **R-6** | **Do nothing to existing records; change only the write path** | §11 — the 39 records *are* the corpus every measurement is taken against |

### 15.2 The trade-off this design makes, named at its width

**It buys** outcome-findability (three queries from *nowhere in top 60* to ranks 7, 48, 8) and
renderability (a reader opens prose, not a struct), **and pays** +0.0266 similarity on a repeat of a
record's own input against today's shape.

**That payment is currently free**, because the loop excludes these rows before admission — so the cost
falls only on a human or agent searching the graph, who is the same party the benefit falls on. **It stops
being free on the day the exclusion is relaxed**, which is the ordering constraint in §13.2 and the reason
§20's relevance floor is filed rather than deferred silently.

### 15.3 Against the product briefing's own test

#13534 §10 replaced *"is Toni closer to running a task"* with **"does the answer he gets get better — and
would he be able to tell?"**

**Honestly: this design does not change the answer the loop produces.** The exclusion stands, so no
admitted row moves. What it changes is the *second* half and a reader the first half does not name: Toni
and peer agents search DiVoid, and today a run record is retrievable only as *"a run"*, indistinguishable
from 38 others, and renders as a JSON dump. After this, a run is findable by what it did and readable by
a person. **DiVoid is the product's memory substrate, so that is product value — but it is not a retrieval
win for the loop, and a summary of this document that implies one is taking more than the measurement
gives.**

---

## 16. Risks, and the falsifiers that catch them

**Falsifier-column discipline (#1220 §5 / §9):** a cell here names a mutation or a check **and where its
observed output is quoted**. Where no one has run it, the cell says so rather than predicting a colour.

| # | Risk | Guard, named | Observed? |
|---|---|---|---|
| **F-1** | A machine reader gets a body it cannot parse and **fails silently** | `compare.py`'s `find_prior_run` extracts the fence; a test feeds it a composed body and asserts the prior run is found. **Premise that makes it discriminate:** today's `except JSONDecodeError: continue` means the *absence* of the fix is a silent skip, so the test must assert the record **is found**, never that no error was raised | **No runnable falsifier established** — the extractor does not exist yet |
| **F-2** | The stored record and the served record diverge by more than the write receipt | `artifacts_test.go:163`, renamed and extended per §10.3: extract the fence, then compare members. **Discriminates because** it compares two live bodies from one turn, so a composition that dropped a key reddens it | The test exists and is green today on the current shape (`go test ./cmd/processor`) |
| **F-3** | The account and the substance drift apart | C3's test compares the two strings from one `WriteRun`. **Discriminates because** a second `RenderSummary` call site would let the two differ, and this test is the only thing that would notice | **No runnable falsifier established** |
| **F-4** | The backfill produces an account that misstates its own record | §11.1's first cohort, read by someone who did not write it, every count checked. **This is a human gate, not a test**, and it is named as one |  #13242 ran the equivalent gate on the model path and failed it; the template path has not been gated at scale |
| **F-5** | Backfilling raises crowding on the existing corpus before any benefit is visible | **This is not a risk; it is a measured certainty** (§4.5: +0.13 on the specimen that ranks first on the ill-matched arm). It is safe only because the exclusion stands. **Falsifier for the safety claim:** any change relaxing `admit`'s or `fuse`'s self-produced arm — G-4 reddens if only one moves, and §20's floor is the gate if both do |
| **F-6** | A future account grows past the ranked window, pushing the outcome out again | The account is **890 – 6,096 B** today against ~7,880 (§4.3) — the largest is at 77 % of budget. **A check worth running before adding any field to `RenderSummary`:** re-measure the account's max across the corpus. **What would break the headroom claim:** a run with a very large `answer`, since the account embeds up to 180 runes of it plus the size — or a raised `CandidateLimit`, since the account lists admitted rows one per line |

---

## 17. The universal in this document, and what would falsify it

The document's central claim is a universal and is stated here with its falsifier rather than left in the
argument (#1220 §5):

> **Claim.** *A serialized struct cannot be retrieved by a natural-language question about what it
> records; the same facts rendered as prose can.*

**Bounded to what was measured:** three questions, one specimen, one embedding model
(`gemini-embedding-001`, #6115), seven arms. Arm B — the account's fields as JSON at the head of the
window — reached one query; arm B-0, the same in the compact form Go emits, reached none; arm C reached
all three inside rank 12.

**The input class that would break it:** a question phrased in the struct's own vocabulary — *"a record
whose stopReason reason is wantsRecall with modelCalls 6"*. That query would likely favour a JSON arm, and
**it was not tested.** It is also not the query anyone asks, which is why the bounded claim is the useful
one — but a design that said *"JSON is never retrievable"* would be false, and this one does not.

**One of the three questions sits nearer that class than the other two, and it was measured rather than
argued.** Q3 names three counts — twenty, seven, thirteen — so a JSON arm carrying them as numerals has
the class's own vocabulary in its window. **Arm B does not carry seven or thirteen** (they live only in
`candidates`, which B excludes), so Q3 is outside the class *for the arm the universal is stated against*.
**Arm B-plus does carry all three, and reaches Q3 at 0.6573, rank 58** — measurably helped, and 0.0209
below the prose arm. So the numerals matter and they do not overturn the claim; what they do is narrow it:
**on a question phrased in the struct's vocabulary the gap shrinks, and this document has one data point
for how far.** Raised by #14062 CF-2 as a hypothesis and settled here by measuring it.

**Which direction the error runs, and it is the quiet one.** An over-broad claim here produces a design
that renders prose where JSON would have done — visible, cheap, self-correcting. An under-broad one
leaves the account unreachable and *looks fine*, because a record that returns nothing returns no error.
**The failure this design exists to fix is the silent direction**, which is why it was measured rather than
argued.

---

## 18. #1136 §5 Pre-Design Checklist — walked

**KISS / DRY / YAGNI**

- **No new type mirroring an existing one.** No type is introduced at all. `Record` is unchanged (§9).
- **No new abstraction with one implementation.** None introduced: no port, no interface, no package.
- **No element justified by "we might need X later".** The fence rule (C1) is justified by two readers
  that exist today (§10.4), not by a future one. The *last*-fence wording costs nothing and is argued in
  §10.1.
- **No deprecation period, flag, or shim.** The `compare.py` fallback in §10.4 is **not** a transition
  window: the mixed corpus is the intended operating mode while the backfill's selector is narrow
  (§11.2), and it is removed when the selector has covered everything.
- **DRY math.** No multi-line block is inlined at multiple sites. The one duplication this design creates
  is **one string stored twice on one node** (account in content, account in substance, ~2–3 kB). It
  passes the can-it-be-deleted check with a named reason: #13238 §8.1's merged form rule reads the
  substance, and deleting it would make that rule render 52 kB of content instead of 3 kB. Both copies
  come from **one** `RenderSummary` call, so they cannot drift (C3).

**Existing systems first**

- **Audited.** `RenderSummary` already exists and is already called on this path (`write.go:68`). This
  design adds no renderer; it moves an existing one's output to where it is read.
- **No new layer proposed**, so no "why it can't live on the existing surface" is owed.
- **No new persisted data point.** Not one new field. The account is a projection, and §9 says explicitly
  that it must not become a member of `Record`.
- **Consumer chain recursed.** A7/A8: the Go tree has no stored-record reader; `compare.py:280` does, and
  its consumer is `refuse_repeats_against_graph`, which has a named human consumer (the comparison
  harness's operator). The chain terminates in a live consumer, so the record is not transitive dead code.

**Configurability**

- **No new knob.** The backfill's selector is an operation parameter, not configuration.
- **No telemetry-then-tune compound.**
- **No magic number introduced.** The document quotes `AssemblyByteBudget` and the ~8,000-character
  embedding cap; it changes neither.

**Less is better**

- **Can-it-be-deleted, run over every element:** the separator and fence — **no**, C1's readers need it;
  the content type — **no**, the declaration would otherwise be false; the backfill — **no**, §11; the
  substance write — **no**, #13238's rule reads it; the `compare.py` change — **no**, §10.4, it fails
  silently without it. **Nothing survived that could be deleted.**
- **Can-it-be-merged:** the two `RenderSummary` calls are merged into one (§8 step 4).
- **Trade-off named explicitly:** §15.2, at its measured width.
- **Radical-clean where unconsumed:** applied and **rejected on a named consumer.** The radical-clean
  shape here is R-1 (drop the record); §15.1 rejects it because A8 names a live consumer and #1220's
  capture ruling binds. This is the checklist item that most wanted the opposite answer, and the consumer
  is why it did not get it.
- **Reader inventory covers AST *and* string-literal references:** §10.4's sweep rule is stated as a
  property with a re-derivation command, and explicitly **not** as a complete list.

**Document discipline**

- Cites #114 §0 and #1136 as load-bearing (header).
- Reader and scope inventories explicit (§2, §8, §10.4).
- Out-of-scope items listed, each with where it lives (§2).
- **No predecessor superseded.** #13238, #13480, #13601 and #13274 are all **composed with**, and §2's
  table says which decision lives where. The one correction this document makes to a live document is to
  #13601 §13.2's bridge lifetime and §19 row 1's expectation (§13.3) — filed back to it rather than only
  recorded here.

---

## 19. Open Questions

| # | Question | For |
|---|---|---|
| **Q1** | **Is a run record's forensic half worth keeping in the shared graph at all?** This design keeps it on the capture-the-original ruling and on one live consumer. The opposite reading — that DiVoid is a memory substrate and an instrument's output does not belong in it regardless of its size — is a legitimate reading of the same quote, and it is R-1. **The measurement favours R-1 on every outcome query.** If the answer is "the graph holds memory; forensics live with the operator", say so and this design collapses to its simpler arm | **Toni.** This is a question about what DiVoid is for, not about Processor |
| **Q2** | **Should DiVoid stop embedding `application/json` content?** The allowlist (#6115) is what makes a serialized struct rank as prose across the whole graph, not only here. §4.4 arm B is evidence that JSON in the window buys nothing and dilutes what is beside it. Filed as a cross-project task (§20), but the call is not Processor's | **Toni**, as DiVoid's owner |
| **Q3** | **Does an aperture relevance floor belong to #13601's surface or its own?** §12.2 argues the ill-matched crowding is a floor problem. #13601 is merged and implemented; #13593 is closed. I filed it as its own task — **#14055** — rather than reopening either | the operator |
| **Q4** | **Should the backfill's first cohort be the 17 substance-less records or the 3 most recent?** §11.1 says the 17, because the account has never existed for them. The counter-argument is that the most recent are the ones anyone will read | the operator — a small call, and either is defensible |

---

## 20. What this does not fix — filed, not solved

| # | Not fixed | Disposition |
|---|---|---|
| 1 | **The aperture has no relevance floor.** On an input the graph answers badly, the top 20 is 19 run records at 0.6164–0.6641 and the ranking returns its best twenty regardless. DiVoid's listing route already supports `minSimilarity` (#6115) and the product does not use it. **This is the gate on ever relaxing the self-produced exclusion** (§13.2) | **#14055**, open, linked to this design and to #13601 |
| 2 | **DiVoid embeds `application/json`.** Q2 | **#14056**, open, DiVoid-side, linked to this design and to #6115 |
| 3 | **`block` still leaves the record** — #13238 §9.5, decided, unimplemented. Independent of this design and composable in either order (§10.3). Measured worth ≤0.003 similarity to the embedding (§4.4 finding 3) — **an independent confirmation on a second specimen, not a narrowing**: #13238 §9.5 reason 4 was already struck FALSIFIED on 2026-09-10 from #13480, in both clauses, so there was nothing left to narrow. Reasons 1–3 are untouched | #13238 |
| 4 | **The name still carries the input** — #13480, decided, unimplemented. Orthogonal; §11.4 | #13480 |
| 4a | **`block` contributes "exactly zero" to the embedding on 22 of 39 records and not on the other 17** (§4.2). #13480 §5.4 and #13238 §9.5 clause 1 both state it unqualified, measured on #13472. **This does not reopen reason 4** — clause 2 fails on its own — but the claim is a property of one record's shape rather than of run records | to be carried back to **#13480 §5.4** and **#13238 §9.5** as dated notes |
| 5 | **#13602's expectation that this task ends the over-fetch bridge is falsified** (§13.3) — the expectation is #13602's own sentence, resting on #13601 §13.2's three routes, not on #13601 §19 row 1. And #13601 §13.2's *2.2 days* has not been borne out by four days of observation | to be carried back to **#13602** and **#13601 §13.2** as dated notes |

---

## 21. Implementation Guidance for the Next Agent

**No code in this document. Each unit is its own branch and its own PR** (#8385's one-feature-one-PR rule).

### Unit 1 — the record node carries its account

**One PR. The `compare.py` change is inside it, not after it** — without it the comparison harness stops
refusing repeats silently (§10.4).

1. In `WriteRun` (`internal/divoid/write.go:47`), render the account **once**, before the content post, and
   use the same value for the content and for the substance at `:68`.
2. Compose the content: account, then a line containing exactly `---`, then the record inside a
   ` ```json ` fence. Post it with the content type from step 3.
3. Change `runContentType` (`:20`) to `text/markdown; charset=utf-8`. **This does not change whether the
   node is embedded** (A3); it stops the declaration being false.
4. Update `scripts/compare.py`'s `find_prior_run` to extract the content's **last fenced `json` block**
   before parsing, falling back to parsing the whole body so un-backfilled records are still found.
5. Amend `TestTheStoredBodyIsTheResponseBodyMinusTheWriteReceiptAndNothingElse`
   (`cmd/processor/artifacts_test.go:163`) per §10.3 — extract the fence, then run the member comparison
   unchanged. **Rename it to say what it now asserts.**
6. Add the three contract guards of §10.1 (C1 round-trip, C2 prefix, C3 account/substance identity).
7. **Sweep for the property, not for my two sites** (§10.4): *every reader that turns a run record node's
   content into structured data must go through the fence.* Re-derive the list; do not trust §10.4's.

**Do not change** `assemble.go:53`, `retrieve.go:90`, `client.go:190`, or `IsRunRecord`. G-4 must stay
green **without being edited** — if it needs editing, something in this unit went outside its scope.

### Unit 2 — the 39 existing records carry theirs

**Its own PR, after Unit 1 is on `main`**, so the backlog stops growing while it is written.

1. Build the operation of §11.1: selector, per-record recompose, skip-unless-forced collision rule.
2. Run it over the **first cohort** — the 17 records carrying no substance.
3. Have someone who did not write it check three accounts against their records, every count and figure
   (F-4). **That is the widening criterion**, not a schedule.
4. Widen to the rest.
5. Re-run §4.6's aperture measurement afterwards and report it. **Expect crowding to rise** (§4.5 / F-5);
   a result showing it did not is a reason to check the instrument before believing it.

### The order, and the one thing that must not happen

**Nothing in either unit relaxes the self-produced exclusion**, and nothing should until §20 item 1 is
answered. The measured combination to avoid is *admissible run records* plus *no relevance floor*: that is
the one state in which this design's +0.13 (§4.5) is paid on exactly the inputs the product briefing
measured as worst.
