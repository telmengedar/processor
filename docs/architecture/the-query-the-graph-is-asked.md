# Architectural Document: The Query the Graph Is Asked

> **Repo path:** `docs/architecture/the-query-the-graph-is-asked.md` (canonical copy).
> **DiVoid node: #13585** — it carries this file verbatim, uploaded from the path above. **The identity is
> against the LF form**, which is what this working tree holds (0 CRLF, 1,132 LF) and what is uploaded; a
> `core.autocrlf` checkout on this machine produces a different digest for the same document, so the claim
> is *node #13585 == this file, LF*, not "byte-identical" unqualified. A parity publish goes to **#13585 and
> no other node**.
>
> **Revision 2, 2026-09-11**, answering QA **#13590** (⚠️ approved with warnings; seven, all document-side).
> Corrections are made **in place and marked where they were wrong**, rather than silently rewritten —
> §2.2/§3/§12/§17.5 (fusion is inert today), §4.2 F-1 (a control that could not fail), §7.1/§16 (a deferral
> with no trigger), §7.4 (a chosen constant called derived; two indistinguishable deadlines), §7.5 (an
> arithmetic that contradicted invariant 1), §11 (an enumeration that grew), §14 Q1 (argued from its weakest
> ground), §17.2/§17.3 (two limits that stopped being theoretical). **Some defects here were found by this revision's own sweep rather
> than by the review; the two load-bearing ones are §11's carrier count and §17.3's mitigation clause.**
> *(No total is given deliberately: a count written as a closing flourish is the thing a correction round
> most reliably gets wrong, and this header had it wrong at "two" before the sweep was run over itself.)*
>
> **Ordering authority:** **#13534** (the Processor product briefing) — this is **third** in its §8 order,
> behind the container (`feat/the-product-runs-in-a-container`, PR #64) and the failure report
> (**#13564** / `design/what-a-failed-run-reports`, PR #65). **The goal state this unit moves toward, in
> one sentence — #13534 §7 requires that such a sentence be named; the sentence itself is this document's,
> not a quotation:** Toni gives the container a task and the graph is asked a question the product designed,
> so the next failure is a failure of a designed query rather than of an undesigned one.
>
> **Prior design this completes rather than replaces:** **#11235** / `docs/architecture/m3-derived-recall.md`
> — *retrieval that thinks before it looks*. Its §11 steps 1–3 shipped; **step 4 is this document's Unit 1**,
> and §4 below re-opens exactly one of its rulings (the port) and confirms the rest against evidence that
> did not exist when it was written. **Nothing here supersedes it and no banner is owed** (#1136 §5).
> **Related and consumed:** **#11296** (the sidecar-coverage guard — its numbers are stale, §2.5) ·
> **#11348** (how the blind half of the sidecar was generated) · **#11361** (the lexical-overlap
> measurement, reproduced by two parties) · **#13564** (what a failed run reports — the live precedent for
> §7.5) · **#13238** §7.4.3 (the fill's separate call counter — the precedent §4.4 adopts) · **#13091** (the
> six-run live baseline every cost figure here is taken against) · **#13345** /
> `what-the-adapters-may-share.md` (the line §7.1 places the prompt on) · **Project** #10422 · **repo map
> root** #10454.
>
> **Consumed in revision 2, and each one changed a claim rather than decorating it:** **#11398** (RRF is
> inert in a turn — §1, §2.2, §3, §12, §17.5, F-9) · **#11365** §6 and §8 (the measured rejection invariant 1
> complies with, and the measured cost of fusion becoming live) · **#11288** (half of step 3's gain is the
> scope reserve — Q1) · **#13590** (the QA round this revision answers) · **#13591** / **#13592** (the first
> live task run against this design's own subject — §12, §17.2, §17.3) · **#13565**'s `CORRECTION
> 2026-09-11` and **#13578** (the third URL carrier — §11).
>
> **Standards applied:** Design Contracts **#1136** (§1 KISS/DRY/YAGNI load-bearing; the §5 checklist is
> walked as §15 of this document) · DRY threshold **#1267** · Go Code Contracts **#11034**, in particular
> **P-51** — every measurement claim below carries its measurement or its hedge **in the same sentence** ·
> falsifier discipline **#12958 §18.6** — every universal states what would falsify it and **names the
> detector** · **#1225** — an exhaustive or absence claim is itself a measurement, swept by stem and by the
> datum's name.
>
> **Baseline:** `main` at **`0df5c14`**, tree clean, branch `main`. Every line number, byte count and token
> figure attributed to *"the tree"* was read out of that tree during this design. Read-only git; nothing
> here was committed by its author, and no `gh` was run.

---

## TL;DR

**What.** The turn stops asking the graph the task text and starts asking questions **derived** from it. One
model call before retrieval turns the input into at most five additional queries; the raw input stays at
index 0; `loop.Retrieve` is untouched in code. **The mechanism is the one `scripts/generate_derivations.py`
already runs** — moved out of a script that fills the eval's sidecar into the product the sidecar is
supposed to be a record of.

**How.** `loop.ModelPort` gains one method — prompt in, text out. `internal/loop` owns the prompt, the parse
and the merge rule; `internal/eval.QueriesFor` is re-pointed at that same merge rule, so the two arms become
one arm by construction rather than by assertion. The derivation call gets **its own counter and a ceiling
of one — never `MaxModelCalls`** — and **its own 30 s bound**. Any outcome that is not a usable query set
degrades to today's exact behaviour, and the record carries **the cause, bounded — not a boolean**.

**And it switches a mechanism on, which is the part easy to miss.** Reciprocal-rank fusion over a single
list is order-preserving, so **a turn today performs no fusion at all** (#11398, re-derived in §2.2). This
is the first time the product ranks the way its instrument has been scoring — not more input to a running
mechanism. Risk **F-9**.

**Cost.** One model call, assembled prompt **2,623 B** — **3.7 %** of the 71,800 B judgement call it
precedes. **Five extra unscoped recalls: 4 graph calls before judgement become 9**, and their wall-clock
cost is **unmeasured** (F-3). One new record string field, empty on the success path. Full figures and their
provenance: §2.4.

**What it buys, and it is small — stated here rather than only 400 lines down.** On the pinned sidecar,
**9/23 → 11/23 retrieved: +2 rows, and one of the two is contaminated — so one row of uncontaminated
evidence out of twenty-three.** **That is a poor trade if the goal is the rate, and it is not the goal**;
what is bought is **the instrument measuring the product**. Two further honest limits: that +2 moves *two*
variables at once (more queries **and** fusion switching on), and on the first live task run against this
design's own subject the right document was **rank 1 and cut on size** — retrieval was not the failing stage
(#13592). §12.

**Does it need a model call at all? Yes, measured, not assumed:** **0 of 25** pinned keyword lines are a
token subset of their own input, and that survives an independent re-run under two stemmers (§4.2; output
quoted at #13590 §1).

**Strongest rejected alternative — pseudo-relevance feedback**, model-free and *not* refuted by that
measurement. Rejected because it derives the second query from the first query's results — the anti-pattern
#13534 §3 exists to stop — and because its nearest measured relative scored **10/13 against fusion's
12/13**. **Reversal low and additive.** §12 R-2.

**This block carries more than one decision, deliberately.** The call-budget ruling (§4.4), the 30 s bound
(§7.4) and the record field's position (§7.5) are each separately steerable; §14 lists eight, each with its
alternative and reversal cost.

---

## 1. Problem Statement

**The product cannot produce the arm its own instrument measures.**

`internal/eval` is a sidecar of pinned query sets per corpus row, `cmd/eval` sweeps a corpus under whichever
arm it is given, and `internal/eval/derivations.go:39-44` names the arm `raw-input` when no sidecar is
supplied and the sidecar's path otherwise. **A derived-arm sweep therefore reports a number about a system
that does not exist**, because `internal/loop/turn.go:117` reads `queries := []string{input}` and nothing in
the product derives anything.

Toni, verbatim, and this document is written to these words (#13534 §3):

> *"You optimize the memory result, but still use the actual task input as a query — we discussed before
> that we need something which actually builds the semantic query from the input (like a pre-prompt).
> **Optimizing using the potentially bad effect of a potentially bad query is not really a targeted
> process.**"*

The complaint is not that the retrieved rate is low. It is that **every retrieval and admission constant
this project has tuned was tuned against an input nobody designed** — and the instrument that measured them
can already express the designed input while the product cannot emit it. That is #11142's rule inverted: the
instrument is ahead of the product, and the gap is invisible because both call the same `loop.Retrieve`.

**The gap is one level deeper than "which queries", and #11398 is where that was measured.** Its statement:
*"The instrument is measuring a ranking the product does not perform, on every row where the sidecar pins
more than one query — which after PR #32 is all of them."* The two arms share `Retrieve` and diverge on its
argument, and the argument decides whether a **ranking algorithm** runs at all — §2.2 re-derives why. So the
divergence this document closes is *which questions* **and** *which ranking*, and a reader who has only the
first half will mis-size the change.

**Success criterion.** A run's record shows the graph was asked more than the task text, names what it was
asked, and — when it was asked only the task text — names why. And **a sweep of the pinned sidecar becomes a
statement about the product**, because the product runs the same merge rule and (after Unit 2) the same
prompt.

> **Unit 1 meets the first sentence and half of the second, and §16 says so at the point it matters.** The
> merge rule becomes shared **by construction** (§7.3, guarded by G-4); the *prompt* stays two texts until
> Unit 2, held equal by a trigger rather than by the compiler (§7.1). **A reader should not take "the sweep
> measures the product" as fully delivered by Unit 1.**

---

## 2. The measured state at `0df5c14`

### 2.1 One line is the whole of it — and the sweep was run by stem, not by phrase

`internal/loop/turn.go:117`:

> `queries := []string{input}`

**The absence claim is a measurement (#1225), so it was swept twice.** By the datum's name — every
`.Recall(` call site in the tree outside tests — and by stem — every occurrence of `quer` in
`internal/` and `cmd/` outside tests. The complete result:

| site | what it issues |
|---|---|
| `internal/loop/retrieve.go:19` | one unscoped recall per member of `queries` |
| `internal/loop/retrieve.go:31` | one recall of `queries[0]` inside the anchor's two-hop scope |
| `internal/loop/turn.go:353` | the supplementary recall, carrying the **model's own** `RecallQuery` |

and exactly two places construct a query *set*: `internal/loop/turn.go:117` (the product — the input, once)
and `internal/eval/derivations.go:92-101` (`QueriesFor` — the input, then whatever the sidecar pinned,
without repeats). **There is no trimming, keywording, templating or rewriting anywhere between the HTTP
handler and the embedding.** #11235 §1 measured the same thing at `1dc361f`; it is still true two weeks and
eleven merges later, and it is the only thing about retrieval that has not moved.

### 2.2 Everything downstream of the query set is already built

This is the fact that makes the unit small, and it is source-provable:

- **`Retrieve` already takes `[]string`** (`retrieve.go:12`) and fans out one recall per query.
- **A reciprocal-rank fusion step is already written** — `fuseByReciprocalRank` at `retrieve.go:120-141`
  with `fusionRankConstant = 10` (`retrieve.go:9`), wrapped by `fuse` at `retrieve.go:77-118`, which holds
  the reserved scope allocation and the anchor exclusion.
- **Per-candidate attribution already exists**: `Source{Query int, Scoped bool, Rank int}`
  (`types.go:31-36`), populated in `sourcesOf` and carried into every `Disposition`, so **which query
  returned which node is already recorded on every run and every sweep**.
- **`Record.Queries []string` already exists** (`types.go:176`) and `RenderSummary` already renders it as
  `q0…qN` with a correct plural (`summary.go:77-82`).
- **`cmd/eval/sweep.go:54` calls the same `loop.Retrieve`** the turn calls — #11235 §11 step 1's extraction,
  shipped.

**But "already built" is not "already running", and the difference is the size of this change.** #11398
measured it and this design re-derived it from `retrieve.go:120-141` rather than accepting the sentence:
with one list, every entry's score is `1/(fusionRankConstant + rank + 1)`, **strictly decreasing in rank**,
so the list is already in descending score order when `SortStableFunc` sees it and **the sort is the
identity**. *The premise that makes that discriminate: it holds exactly while no single recall list repeats
a node id — a repeat would accumulate score and promote it. `Recall` returns distinct nodes, so the identity
holds today.*

**Therefore, in a turn today, there is no fusion.** Positions 1–17 are the unscoped recall's plain
similarity order; **at most** three further slots go to the scope reserve (`fuse`, `retrieve.go:77-118`,
`limit − reserve = 17`) — *at most*, because `fuse` backfills from the fused list when the scoped list holds
fewer than three unseen rows. The
machinery is not *running on* a one-element slice — it is **inert**.

**So the accurate statement of this change is not that it feeds more input to a running mechanism. It
activates, in the product, a ranking behaviour the product has never performed.** Three consequences carried
where they belong rather than left here: §3's out-of-scope item 1 is true of the code and needed restating
about the behaviour; §12's `+2 rows` measures **two** changes at once, not one; and §13 gains **F-9**, a
risk that only exists once fusion is live.

### 2.3 The judgement budget is already tight, measured

`MaxModelCalls = 6` (`turn.go:17`), and the six-run live baseline #13091 measured against it:

| trajectory | runs | model calls | terminal | `capReached` |
|---|---|---|---|---|
| A | 4 | 6 | `wantsWrite` | **true** |
| B | 1 | **5** | `answered` | false |
| C | 1 | **6** | `answered` | false |

**Two of six runs reached their own terminal, and one of those two used the last call the cap allows.**
Taking a call from that budget would have converted trajectory C from *finished* to *capped*. §4.4 is
decided by this table.

### 2.4 What a derivation-shaped call costs, measured on this exact call

`scripts/generate_derivations.py` is a working derivation step that lives outside the product. Measured
from its source and from `internal/eval/`:

| quantity | figure |
|---|---|
| system prompt | **1,790 B** |
| two format-only few-shot exchanges | **710 B** |
| corpus input | 70–203 B, **median 123 B** |
| **assembled prompt, median input** | **2,623 B** |
| output, five queries | 239–395 B, **median 297 B** |
| generation latency, twelve rows, `ai/qwen3-coder` | **~0.4 s/row**, 5/5 valid distinct queries on the first attempt for every row (#11348, verbatim) — *the runtime attribution (Docker Model Runner) is not #11348's; it is source-provable from `scripts/generate_derivations.py:62` and `:79`* |

Against the judgement call it precedes: **71,800 B / 19,687 prompt tokens**, and **both figures are of one
request** — #13091 §5(b)'s byte-identical seq2/seq3 call 1, sha256 `bf6b122498c52765…`, whose
`prompt_eval_count` is 19,687 in both trajectories. The derivation prompt is **3.7 %** of it by bytes
(2,623 / 71,800 = 3.65 %). *(Bytes, not tokens: nothing in this product tokenizes, so a token ratio would be
a fabricated one — the same P-51 constraint #13564 A2 states. The token count is quoted only to size the
call, never divided into.)*

> **Corrected in review (#13590 §12).** An earlier revision paired the 71,800 B with **19,187** tokens.
> 19,187 is #13091 §3's `prompt_eval_cached_count` on **trajectory A's** call 1 — a *different* request,
> whose own prompt was 19,188 tokens. The byte ratio was never affected; the pairing was.

### 2.5 #11296 is stale in its numbers and open in its substance — both halves measured

#11296 records *"the corpus has 25 rows and the sidecar has 13"*. **Measured at `0df5c14`:
`internal/eval/corpus.json` holds 25 rows (23 labelled, 2 control), `internal/eval/derivations.json` pins
25, and `Derivations.Unpinned(corpus)` returns the empty list.** Sources: 13 `hand-authored`, 12
`blind-generated`; the arm line counts only labelled rows (`labelledRowsSourced` filters `StratumLabelled`)
and so prints **11 hand-authored, 12 blind-generated**, the two controls being hand-authored and excluded.
#11348 is the record of how the twelve were closed.

**What remains open is the guard direction, not the coverage.** `validateDerivations` still checks only
sidecar → corpus; the reverse is a `Warn` in `cmd/eval/main.go:85-92` plus the coverage figure on the arm
line — #11296's options (2) and (3), shipped. **This design does not touch that guard and does not need
to**, because it changes no sidecar row and adds no corpus row. It is named here because a reader arriving
from #11296 would otherwise reason from a 13/25 that has not been true since PR #32.

### 2.6 What is *not* in the tree at this baseline

`git ls-files` matches **no** `Dockerfile`, no compose file and no `.dockerignore` at `0df5c14`. The
container lives on `feat/the-product-runs-in-a-container` and is not on `main`. Nothing in this design
depends on it; the sequencing in #13534 §8 does.

---

## 3. Scope and Non-Scope

**In scope.** How the turn obtains its query set; what a derivation failure does and reports; where the
prompt, the parse and the merge rule live; what the record carries; the call budget and the bound the
derivation call runs under; and the parity contract between the product's query set and the sidecar's.

**Out of scope, explicitly:**

1. **`loop.Retrieve`, fusion, the scope reserve, `admit`, the byte budget, the block layout.** #11235 §4.2
   measured the combiner and #11235 §11 steps 1–3 shipped it. **Not one line inside that function
   changes.** But say the behavioural half too, because the code half alone misleads: since fusion is inert
   at one query (§2.2, #11398), changing the input is what makes the fusion step **start doing work in a
   turn**. *No code in scope; a behaviour that is.* §13 F-9 carries the risk that follows.
2. **The supplementary recall** (`turn.go:348-361`). Its query is the model's own tool argument, already a
   derived query by a different route, and #11263 ruled that call site stays direct.
3. **Retrieval or admission tuning of any kind.** #13534 §5 demotes it until a designed input exists; this
   unit is what supplies one. Re-tuning against the new arm is the *next* question and not this one.
4. **Document-side retrieval failure** — #11235 §2.4's mode C, a 357-byte answer inside a 9,519-byte node
   ranked 34 on its own text. No query reaches it; it is a DiVoid-side chunking question (#11235 §8 F1).
5. **Making `cmd/eval` call a model.** The sweep stays the zero-variance layer (#11235 §4.5). Unit 2 puts
   the model call in a *separate* command that writes a sidecar; the sweep still reads a frozen file.
6. **Regenerating the pinned sidecar.** `internal/eval/derivations.json` is the comparability baseline for
   every arm-versus-arm figure this project holds (#11348), and Unit 2 writes a new file rather than
   overwriting it.
7. **A second, cheaper model endpoint for derivation.** #11235 §12 q1 is a spend decision that is Toni's;
   §14 Q2 records it as decided-for-now with its reversal cost.
8. **Retry of a failed derivation.** §4.5.

---

## 4. The ruling: does a derivation need a model call at all?

This is the question the brief refuses to pre-answer, and it is settled first because everything downstream
turns on it. **The row is written before the list** (#13564 §17): the property is *every mechanism that can
turn one input string into a set of query strings*, and the partition is by *can the mechanism emit a token
the input does not contain*.

### 4.1 The partition, with its complement checked

| class | membership rule | members |
|---|---|---|
| **1 — input-vocabulary-only** | emits only tokens present in the input, in the anchor already fetched at `turn.go:109`, or in a compile-time constant | stopword strip / keyword-dense line; re-casing and reordering; a fixed template wrapping the input; concatenating the anchor's name or type; any composition of these |
| **2 — graph-vocabulary** | emits tokens sourced from the graph's own reply to a first query | pseudo-relevance feedback: issue the raw query, harvest top-k candidate names or substances, re-query with them |
| **3 — model** | emits tokens from a model conditioned on the input | one completion call before retrieval |

**The partition is checked rather than asserted.** A mechanism that emits a token found in neither the
input, nor the anchor, nor a constant, nor the graph's reply, has no fourth source available inside this
process. **The re-derivation rule, so a reader can regenerate this rather than trust it:** list the packages
that construct an outbound HTTP request (`grep -l 'NewRequestWithContext\|http.Client' --include=*.go`,
production files only). At `0df5c14` that returns `internal/divoid` and the two model adapters — **two
destinations, the graph and the model endpoint** — plus `cmd/condense`, which wires an adapter rather than
adding a destination. **Do not trust this list to be complete; re-run it.** Given those two destinations,
classes 1–3 cover the whole, and `cmd/eval` proves a retrieval pipeline needs neither beyond the graph.

### 4.2 Class 1 is refuted, and the measurement is first-hand

The arm the eval scores is `{input} ∪ pinned`. If class 1 could produce it, the model call would be
unjustified — this is the cheap version the brief warns against reaching past.

**Measurement.** Tokens are lower-cased `[a-z0-9]+` runs compared as sets — the same family #11361 used, and
its caveat is inherited verbatim: *the decimals are tokenizer-dependent; the directions are not.* Over all
25 sidecar rows against their own corpus inputs:

| quantity | min | median | max |
|---|---|---|---|
| novel-token fraction of the **keyword line alone** (line 5) | 0.20 | **0.60** | 1.00 |
| novel-token fraction of the row's **five pooled queries** | 0.61 | **0.74** | 0.87 |

**And the categorical result, which is the one that does not move with the tokenizer: 0 of 25 rows have a
keyword line that is a token subset of its own input.** The property that makes it hold is that the line
contains a word the input never spells — `accountability` (r02), `mutation` and `green` (r03), `consistency`
(r18), `exit`, `code` and `pipeline` (r08).

> **Hedge, stated in the same sentence as the claim (P-51).** *Stem variance was checked by hand on the six
> lowest-novelty rows only* — r13's `lookup` is a variant of the input's `lookups` and r20's `reporting` of
> `report`; each of those rows retains at least one non-variant novel token (`index`, `period`), so the
> zero-of-25 survives on those six. **The full population was not re-run under a stemmer**, and F-1 names
> that as the detector.

This is consistent in direction with #11361's population figures: query-side reuse **0.263** (hand) /
**0.347** (model), i.e. roughly two thirds to three quarters of the pinned queries' tokens are not the
input's. A second reviewer reimplemented that specification from scratch — her own tokenizer, her own
stopword list — and reproduced it in **all four metric directions across all four tokenizer variants.**

> **What was reproduced, stated precisely, because the distinction is #11361's own.** Its reproduction table
> answers **"yes"** for directions and **"no, and not expected to"** for *"absolute decimals, band cells,
> exact rank positions"*. **So 0.263 and 0.347 are one implementation's decimals, not reproduced values** —
> what survives a change of tokenizer is the direction, which is the same caveat this section inherits for
> its own figures. *An earlier revision's phrasing put the decimals inside the reproduction claim.* Two
> measurements, different normalisations, same direction — and that is the whole of what is claimed.

> **Falsifier F-1, differential, with its detector named (#12958 §18.6 clauses 2–3 — clause 2 requires the falsifier be named in the same passage, clause 3 that it be differential).** *Claim arm:* re-run the
> set comparison over all 25 rows under a stemming tokenizer; the claim is falsified if **any** row's
> keyword line becomes a token subset of its input.
>
> *Liveness arm — **two** controls, because one of them does not discriminate the thing under test:*
>
> - **Control A — a keyword line built by literally deleting stopwords from its input.** Must report a
>   subset. This proves only that the **subset predicate** works.
> - **Control B — control A plus one inflectional variant** (`lookups` for `lookup`, say). Must report a
>   subset **under the stemmed run and must NOT under the plain one.** This is the one that proves the
>   **stemmer was engaged**, and it is the arm F-1 exists for.
>
> **Why B is required and A is not sufficient — stated because it is the defect this falsifier had.**
> Control A reports a subset under *every* normalisation, plain included. So a stemmed re-run that silently
> ran the plain tokenizer — precisely the blindness F-1 guards — **passes control A**. A detector that
> cannot be made to fail by the failure it names is a decoration.

**Both arms have been executed, and their observed output is quoted at #13590 §1** — so this is a
measurement rather than a prediction — which is the standard a falsifier cell is held to, since a predicted
red that nobody has watched go red is prose:

| normalisation | keyword-line subsets, all 25 | keyword median | pooled median | control A | control B |
|---|---|---|---|---|---|
| plain (this document's family) | **0 / 25** | 0.600 | 0.741 | subset ✓ | **not subset** |
| Porter (1980) | **0 / 25** | 0.500 | 0.710 | subset ✓ | **subset ✓** |
| aggressive suffix-strip | **0 / 25** | 0.571 | 0.727 | subset ✓ | not subset |

**The claim arm did not falsify: the zero-of-25 survives full-population stemming**, and the hedge above
turns out to have been conservative rather than load-bearing. **Control B discriminates as required** — a
subset under Porter and not under plain. The hedge is left standing above rather than deleted, because it
is what invited the test that discharged it.

### 4.3 Class 2 is not refuted by that measurement, and loses on other grounds

Pseudo-relevance feedback costs zero model calls and one or two extra graph reads, and it *can* emit
vocabulary the input lacks — so §4.2 does not touch it. It is the strongest model-free alternative and it is
rejected explicitly:

1. **It derives the second query from the results of the first.** The whole unit exists because Toni
   objected to optimising on the output of an undesigned query; a mechanism whose expansion terms come from
   whatever that query happened to return is that objection with an extra hop. Query drift is this family's
   documented failure mode, and here the drift source *is* the thing being distrusted.
2. **Its nearest measured relative lost.** #11235 §4.2's combiner table measured union-then-sort-by-similarity
   over the same pool at **10/13** against reciprocal-rank fusion's **12/13** on identical lists. The reason,
   quoted verbatim from **#11235's TL;DR** (not §4.2, which states the same thing in different words):
   *"because the nodes that miss, miss precisely because their similarity is low"*. Feedback harvested from the top of a ranking the
   required node is absent from inherits that property. **This is an argument by adjacency and is labelled
   as one** — no feedback arm has been swept.
3. **It produces a third arm.** Neither the pinned sidecar nor any figure this project holds describes it,
   so shipping it would leave the instrument/product gap open in a new direction.

**Reversal cost: low, additive.** `Retrieve` takes `[]string`; a feedback step appends to that slice and
needs no port, no record change and no eval change. §14 Q4.

### 4.4 The call must not come out of `MaxModelCalls`

**Precedent, quoted rather than paraphrased** — #13238 §7.4.3, ruling on the fill's model calls:

> *"**Fills must not consume `MaxModelCalls`.** That cap governs the model's reasoning budget — six calls in
> which to answer. A fill is infrastructure, not judgement. Spending a judgement call on a condensation
> would make a turn's reasoning budget depend on how condensed the graph happens to be, which is exactly the
> coupling this project keeps eliminating elsewhere. **Separate counter, separate ceiling, both in the
> record.**"*

**The precedent applies, and here it is stronger than it was there, because the measurement exists.** §2.3:
four of six runs already hit the cap and trajectory C finished on call 6. A derivation charged to that
budget would have turned a completed run into a capped one on the only complete six-run sample this project
has. Making a run's *reasoning* budget depend on whether its *query* was derived is the same coupling under
a different name.

**Ceiling: exactly one derivation call per turn**, and it is not a tunable constant (#1136 §3) — one,
because the degraded path is free and a second attempt is a retry, which §4.5 rules out. The counter and the
ceiling reach the record through `Record.ModelCalls`, which stays the count of *judgement* calls, and
through the record's derivation field, which says whether a derivation call happened at all.

### 4.5 The degraded mode, and why there is no retry

**Any outcome that is not a usable query set degrades to `queries = {input}` — today's shipped behaviour,
byte for byte.** #11235 §5 ruled this and it stands unchanged. The turn does not fail because derivation
failed: retrieval on the raw input alone is whatever the raw-input arm currently reads, and erroring the run
instead would trade a degraded answer for no answer.

**No retry.** `scripts/generate_derivations.py` makes up to four *attempts* — `MAX_FILL_ATTEMPTS = 4` at
`scripts/generate_derivations.py:81`, i.e. three retries — because it must *collect* five
distinct queries for a file that will be pinned forever; a turn does not — it takes whatever parses and
proceeds, and a partial set of two queries is strictly better than the raw input alone. A retry would double
the latency of the step whose cheapness is its justification, in the branch where the endpoint has already
shown it is slow or broken. #13564 §3 out-of-scopes retry for the judgement call and reaches the same conclusion — *on a different ground*, #11312's: an operator who could read “model is loading” would wait. Same answer, not the same argument.

### 4.6 A boolean is a constant, and #13564 already ruled what that costs

#11235 §4.4 asked the record to carry *"whether they came from the model or from the raw-input fallback"* —
a flag. **#13564's ruling makes that the same defect one level down:** *"Replacing a sentence with `file
write failed` is not a bounding step; it is a deletion."* A `fallback: true` says a fallback happened and
not why, and the causes have different operator responses — the endpoint is unreachable (fix the host), the
model returned nothing parseable (fix the prompt or the model), the bound was hit (the host is slow, or the
bound is wrong). Three responses, one bit.

**So the record carries the cause, bounded, and the log carries it whole** — #13564 §4.4's shape, applied to
a second call site rather than re-argued. §7.5 states the field.

**And the *fact* of the fallback needs no field at all**, because it is already derivable: `len(Queries) == 1`
holds exactly when no derived query survived the merge, including the case where the model returned only
duplicates of the input (which §4.5 classes as a fallback anyway). One new field, not two — #1136 §4's
can-it-be-deleted, run and acted on.

---

## 5. Assumptions and Constraints

| # | assumption / constraint | confidence |
|---|---|---|
| A1 | Nothing in the product tokenizes, so every size figure here is bytes. | **Certain** — no tokenizer in the tree. |
| A2 | The configured model endpoint serves a completion with no tools; both adapters already do exactly this for `Condense`. | **Certain** — `internal/ollama/condense.go:57`, `internal/openaicompat/condense.go:49`, identical signatures. |
| A3 | A derived query's value is retrieval-side only: it never reaches the model as text and never appears in the block. | **Certain** — `Retrieve` returns `[]Candidate`; `renderBlock` renders anchor and candidates only. |
| A4 | The graph's ranking of a derived query is a pure function of the query and the graph state, unchanged by this design. | **Certain** — `divoid.Client.Recall` sets `query` and nothing else derived from the caller. |
| A5 | Two turns on the same input may now retrieve different candidate sets, because the endpoint is not reproducible across sessions. | **High** — #13091 §5(b) measured byte-identical requests producing different completions minutes apart at temperature 0, and explicitly rules cache state out as the explanation. §11 treats it as a trade, not a surprise. |
| A6 | 0.4 s/row (#11348) was measured on a *different* runtime (Docker Model Runner, `ai/qwen3-coder`) from the one the container failed against; 2.4 s (#13534 §8) was measured on the product's own adapter path against the degraded host. **Neither is a measurement of this code, which does not exist.** | **Stated as a hedge, not a claim.** F-3. |
| A7 | `internal/eval` may import `internal/loop` — it already does (`result.go:7`) — so the merge rule can live in `loop` without a cycle. | **Certain** — read from the tree. |

---

## 6. Architectural Overview

```
  input, subject
      │
      ├─ Graph.Node(subject) ──────────────────────────────────────────► anchor   (unchanged)
      │
      ├─ loop.DeriveQueries(ctx, model, input)          ── ONE model call, 30 s bound
      │      │                                             own counter, own ceiling of 1
      │      ├─ loop.DerivationPrompt(input)  ─────────► prompt   (loop owns the words)
      │      ├─ ModelPort.Derive(ctx, prompt, maxTok) ─► text     (adapter owns the wire)
      │      └─ loop.ParseDerivation(text)    ─────────► []string (loop owns the parse)
      │              any failure / empty / all-blank  →  nil + a cause
      │
      ├─ loop.MergeQueries(input, derived) ────────────► queries  (input first, no repeats)
      │        ▲
      │        └───────────── the SAME function eval.Derivations.QueriesFor calls ───────┐
      ▼                                                                                  │
  loop.Retrieve(ctx, graph, anchor, queries, CandidateLimit, RecallScopeReserve)         │
      UNCHANGED IN CODE. Takes []string, holds no ModelPort. One impl, two callers. ─────┘
      CHANGED IN BEHAVIOUR: at len(queries) > 1 its reciprocal-rank fusion step
      stops being the identity and starts ranking. §2.2, #11398. F-9.
      │
      ▼
  Assemble → judge (MaxModelCalls = 6, untouched) → WriteRun          UNCHANGED
```

**The mechanical boundary of #11235 §3 is preserved and is still enforced by a type.** The model's entire
contribution to retrieval is a string of text that `loop` parses into `[]string`. It cannot name a node id,
a score, a rank, an inclusion or an exclusion. **The boundary moves one step *inward* under this design and
is stronger for it:** #11235 put the `[]string` conversion in each adapter; here the adapter returns raw
text and `loop` — the mechanical, pure, test-owned half — performs the conversion, so there is one parse
instead of two and it lives on the side of the line that the model does not author.

---

## 7. Components and Responsibilities

### 7.1 `internal/loop` — the prompt, the parse, the merge

**#13345's line decides this and it is applied rather than restated:** *does it render the loop's own data,
or declare something to an endpoint?*

| element | owns | does **not** own |
|---|---|---|
| `DerivationPrompt(input string) string` | the instruction text and the two format-only exemplars; it names **no** protocol token, no JSON schema, no tool envelope | the request body, sampling, routes, headers |
| `ParseDerivation(text string) []string` | splitting to lines, stripping list decoration and reasoning artifacts, dropping blanks, dropping exact case-folded echoes of the input, capping at `MaxDerivedQueries` | anything about how the text arrived |
| `MergeQueries(input string, derived []string) []string` | the input first, then each derived query that is not already present, compared exactly as `QueriesFor` compares today | how `derived` was obtained |
| `DeriveQueries(ctx, model ModelPort, input string) ([]string, error)` | composing the three above around one bounded port call | the fallback decision, which is `Run`'s |

**The prompt is the one already in the tree**, transcribed from `scripts/generate_derivations.py`'s
`SYSTEM_PROMPT` and `FEW_SHOT` — four standalone questions naming a mechanism, concept or technical term
likely to appear in the *documentation*, plus one dense keyword line; explicitly forbidden from restating
the input or answering it. **It is not re-authored here**, because it is the prompt that produced r12 — the
only uncontaminated positive evidence for derivation this project holds (#11235 R2a) — and re-writing it
would discard that evidence for a prompt with none.

**DRY, with the math (#1267).** Between Unit 1 and Unit 2 the prompt exists twice. The block is **1,790 B**
of prompt text — **17 source lines** as `SYSTEM_PROMPT` is written in Python, ~30 as a Go raw-string
constant — at **2 sites**. Whichever line count is used, `17 × 2 = 34` or `30 × 2 = 60` is above #1267's
~15–20 threshold, so the conclusion does not depend on the estimate. Under #1267's own bands this is
>5 lines at 2 (not >2) sites — the *"judgment call; state the trade-off explicitly"* band, which is what
this paragraph is. **It is a real violation and it is not rationalised.**

**Unit 2 discharges it**, by replacing the Python generator with a command that calls this same function.

> **The discharge has a trigger and an owner, because a condition with neither never fires (#13590 §6).**
> An earlier revision said only *"if Unit 2 is not going to be built, embed the prompt as a single file."*
> **Nobody ever decides that Unit 2 is not going to be built; it simply does not happen** — and Unit 2 is
> instrument work, which #13534 §5 demotes, so the branch where the fallback is needed is the *likely* one,
> not the exotic one.
>
> **Trigger:** *Unit 2 has not merged within **10 merges to `main`** after Unit 1 merges.*
> **Fallback:** the prompt becomes a single file in the repo, embedded into Go and read by
> `scripts/generate_derivations.py` from that same path, so there is one text with two readers instead of
> two texts.
> **Owner: DiVoid task #13596**, filed with this revision and linked to this design and to #13590. It
> carries the trigger, the fallback and the deferrals it releases. **Unit 1's PR does not exist yet, so the
> operator adds that link when it opens** — the architect runs no `gh` and files no PR (#7506 §1).
> **Why a merge count and not a date:** the repo's own clock is merges, `MaxModelCalls`-style constants
> move on merges, and a date passes while nobody is working.

**And F-5 must be readable in that branch, which it currently is not** — see §13 F-5, corrected.

### 7.2 The adapters — the wire, and nothing else

`loop.ModelPort` gains:

> `Derive(ctx context.Context, prompt string, maxOutputTokens int) (string, error)`

Each of `internal/ollama` and `internal/openaicompat` implements it by delegating to its existing `Condense`
request builder and returning `.Text`. **DRY math: 3 lines × 2 sites = 6**, far below #1267's threshold, so
it is inlined and no shared helper is introduced — the same call #13564 §11 R-7 declined to extract for the
same reason.

**No third request builder is added.** A derivation call is exactly what `Condense` already sends: one user
message, no tools, repetition penalties pinned to zero, the reasoning stream off. The method's *name* is
wrong for the second use and its *shape* is right; renaming `Condense` to something neutral across two
adapters, `internal/condense`, `cmd/condense` and their tests is a tidy that belongs to whoever next touches
that surface, and is filed rather than folded in (§14 Q6).

**Sampling is the configured sampling** — `PROCESSOR_MODEL_TEMPERATURE`, default `0`. **This differs from
how the pinned sidecar was generated**, which used `temperature 0.5` (`scripts/generate_derivations.py:397`),
and the divergence is named here rather than discovered later: a sidecar regenerated by Unit 2 will differ
from the pinned one for that reason as well as for the model's own variance.

### 7.3 `internal/eval` — re-pointed, not rewritten

`Derivations.QueriesFor(row)` keeps its signature and its meaning and calls `loop.MergeQueries(row.Input,
d.Queries[row.ID])`. **DRY math: the rule is 9 lines × 2 sites = 18, at the threshold — but the extraction is
not justified by line count.** It is justified by parity: if the product's merge rule and the sweep's merge
rule are two implementations, the arms differ in a way no single-package test can see. That is #11142's rule
and #11235 §4.3's argument for extracting `Retrieve`, one level down, and it comes with the same guard shape
(G-4, a cross-package mutation arm).

**No sidecar row, no corpus row and no eval flag changes.** `cmd/eval` still constructs no model client.

### 7.4 `internal/loop/turn.go` — the caller

`Run` gains one step between `Graph.Node` and `Retrieve`:

- derive, under a **30 s** bound applied with `context.WithTimeout` **in the loop, not in the adapter** —
  the adapter's own five-minute `DefaultTimeout` governs the judgement call and must not govern this one;
- merge; record the queries and, on failure, the cause;
- call `Retrieve` with the merged list, exactly as today.

**Why 30 s. It is a chosen round number, and the argument for it is the asymmetry, not a derivation.**
The cost of a bound that is too tight is a fallback to today's behaviour, which is free. The cost of a bound
that is too loose is five of the run's ten minutes spent on a step whose failure costs nothing — instance 3
of #11312 reproduced in a place that cannot possibly justify it. **A step whose failure is free should be
bounded well below the step whose failure is not**, and any value in the low tens of seconds satisfies that;
30 is the round one.

**Three ratios, offered as sanity checks on that choice and not as its derivation.** 30 s is **12.5×** the
worst small-prompt latency measured through this product's adapter path on the degraded RAM-serving host
(2.4 s, #13534 §8), **75×** the median measured generation latency for this call (0.4 s/row, #11348), and
**5 %** of `runBound = 10 * time.Minute` (`internal/server/server.go:13`). All three land on 30.0 exactly,
which is **arithmetic rather than convergence** — the multipliers are what were computed (30/2.4, 30/0.4,
30/600), and reading three exact quotients as three agreeing measurements is the shape P-51 exists to stop
(#13590 §4). *An earlier revision's heading read "derived rather than picked"; it had this backwards.*

**The direction of the choice is conservative, and that is the right direction.** 2.4 s was measured on the
worst host in hand and on a *small* prompt, and 2,623 B is a small prompt — so the reference is apt and
taking 12.5× it biases toward **not firing spuriously**. A spurious fire silently converts the derived arm
into the raw arm (F-4), which is the expensive failure; a late fire costs one wasted call.

#### A third bound is now live — and `context.DeadlineExceeded` does not say which one fired

**#13564 §2.2's finding applies immediately:** *"Two bounds are live, and the failing one is never named."*
So the cause recorded on the fallback path **names this bound when it is what fired**, in the same sentence
as the elapsed time — #13564 §7.1's preamble applied to a second call site.

**But naming it is not free, and asserting it without saying how is a build-blocking gap.** The turn's
context already carries a deadline: `routes.go:77` is `context.WithTimeout(r.Context(), runBound)`. A
derivation bound applied as a child timeout produces `context.DeadlineExceeded` **when either deadline
fires** — Go propagates the parent's expiry into the child as the same sentinel value. So an implementer who
reads `ctx.Err()` and writes *"the 30 s derivation bound fired"* is writing a claim the value does not
carry, and under load — the exact branch F-4 exists for — it will be **wrong in the direction that hides a
sick host**: a run whose ten minutes ran out during derivation would be recorded as a fast step that took
too long.

**So the bound must identify itself rather than be inferred:** apply it as a timeout carrying its **own
named cause**, and read that cause rather than `ctx.Err()`. Go 1.27 is in `go.mod`, so the cause-carrying
form of `context.WithTimeout` is available; the derivation's own cause is then returned when its bound
fires, and the parent's when the parent's does. *The discriminating premise, stated so the next reader can
check it:* the two bounds are 30 s and 600 s on the same context chain, so they are distinguishable by
**identity** but never by the `error` value, and only sometimes by the elapsed — a run bound firing 29.8 s
into a derivation is indistinguishable by elapsed alone.

### 7.5 The record — one field, and where it sits is load-bearing

`Record` gains **one** string field, `derivationError`, `omitempty`, **declared immediately after `Queries`
and before `Anchor`**.

**The position is the design, not tidiness.** #13481 — an open `task` node carrying its own measurement,
and carrying its own guard: *the figures rest on `MaxLength = 8000` and on name-first composition, and must
be re-run if DiVoid changes either* — measured DiVoid's embedding as `name + "\n\n" + content`
truncated at 8,000 characters, of which `candidates` occupies **93.2 %** on live record #13472 — which is
why #13564 §11 R-2 rejected **a structured failure object** on the response and the record — one of its three
grounds being exactly this cap. *(R-2 rejects that object, not new record fields as a class; the wider
statement is #13564 F-5's own "this design adds no field to `Record`". An earlier revision attributed the
wider claim to R-2.)* Appended today they land past the cap and are
invisible to the search that would retrieve them. **This field does not, and the arithmetic is re-derived
here from invariant 1 rather than from a figure:**

The JSON preceding the field is `input`, `subject`, `query` and `queries`. Writing **L** for the input's
length and **D** for the derived queries' joined length, and applying **invariant 1 — `queries[0]` is the
raw input on every path** — those four cost:

| field | length | why |
|---|---|---|
| `input` | **L** | — |
| `subject` | ~10 | an int64 |
| `query` | **L** | a copy of `input` (§9) |
| `queries` | **L + D** | invariant 1 puts the input **inside** `queries`, then the derived set |
| field names, quotes, commas | ~100 | — |

**So the offset is `3L + D + ~100` characters — not `2L + D + 100`.** The input appears **three** times, and
the third appearance is the one invariant 1 guarantees.

- **At the median it makes no difference:** `3(123) + 297 + 100 = 766` characters, comfortably inside
  #13472's measured 7,880-character content budget.
- **At the threshold it makes a large one.** Solving `3L + 297 + 100 = 7880` gives **L ≈ 2,494**.

**The condition, corrected: an input longer than roughly 2,500 characters pushes the field past the cap**,
and `maxRequestBodyBytes = 1 MiB` (`routes.go:53`) permits one.

> **Corrected in review (#13590 §5).** An earlier revision gave this threshold as ~3,500. That figure comes
> from holding `queries` constant at 297 B while `input` grows — which **contradicts this document's own
> invariant 1**, since `queries` contains the input. It overstated the headroom by ~40 % in the section the
> document itself calls load-bearing. The formula was right; the substitution was not. *This is why the
> table above derives each term from an invariant instead of quoting a measured total.*

**That is a stated limit, not a guarantee**, and it degrades to exactly what #13564 lives with today: past
the cap the field is still on the record and still in the HTTP response, and only search reachability is
lost (F-6).

Content of the field: the cause, **bounded to the same 512 runes #13564 §7.3 applies** if that design has
merged, and to a bound declared here if it has not. **This design introduces no second bounding mechanism**
— if #13564 is on `main`, this field uses its constant and its truncator; if it is not, Unit 1 declares one
constant and #13564's merge collapses them. Named as a soft dependency: **#13564 makes this field better and
is not a gate on it**, because the fallback cause is informative even as a bare Go transport error, which is
strictly more than the boolean #11235 asked for.

### 7.6 The summary and the log

- **`RenderSummary` needs no change to render multiple queries** — `summary.go:77-82` already prints
  `N queries` and one `q0…qN` line each. The only addition is one line carrying `derivationError` when it is
  non-empty, reusing `summaryTrunc` and the existing `summaryQueryRunes = 88`. **No third truncator and no
  new constant** (#13564 Q6 may widen that constant to 200; this design takes no position and inherits
  whichever value is in the tree).
- **The operator log carries the cause whole**, unbounded, at `Warn` on the fallback path — #13564 §4.4's
  asymmetry, adopted: stderr is the operator's own stream and the bound exists for the durable, shared,
  re-read destinations. On the success path, one `Info` naming how many queries were derived.

---

## 8. Interactions and Data Flow

**Success path, one turn.**

1. `Graph.Node(subject)` → anchor. *(unchanged)*
2. `DerivationPrompt(input)` → ~2,623 B for a median input.
3. `ModelPort.Derive` under a 30 s bound → completion text, ~297 B for five queries.
4. `ParseDerivation` → up to 5 strings; `MergeQueries` → up to 6, input first.
5. `Retrieve` issues **6 unscoped recalls + 1 `Neighbours` + 1 scoped recall** (`retrieve.go:18-31`),
   **fuses the six by reciprocal rank — which at six lists is a real reordering, where at one list it was
   the identity (§2.2)** — reserves up to 3 slots for the scoped list, caps at `CandidateLimit = 20`.
6. `Assemble`, `judge`, `WriteRun`. *(unchanged)*

**Graph-call arithmetic, source-provable.** Before: `1 Node + 1 unscoped + 1 Neighbours + 1 scoped = 4`
calls before judgement. After: `1 + 6 + 1 + 1 = 9`. **+5, and their wall-clock cost is unmeasured** — #13091
reports non-model time only as an aggregate 4.4–6.8 s per whole run including write-back, and no per-read
figure exists anywhere in this project. F-3 names the measurement; §12 names the mitigation that is *not*
being designed now.

**Failure path.** Any of: transport failure, non-2xx, decode failure, the 30 s bound firing, text that
parses to nothing, or text whose every line is blank or an echo of the input. All six produce the identical
outcome — `queries = {input}`, the run proceeds, `derivationError` carries the cause bounded,
the log carries it whole. **The bound-fired case names *which* bound and the elapsed time** — and per §7.4 it names it from the
timeout's own cause, never from `ctx.Err()`, which is the same `context.DeadlineExceeded` whether the 30 s
derivation bound or the 600 s `runBound` expired. The other five carry whatever the adapter handed up.

**What a reader of the record can now answer that they could not before:** which questions the graph was
asked; which of them returned each candidate and at what rank (`Disposition.Sources`, already shipped);
and, when only the input was asked, why.

---

## 9. Data Model (Conceptual)

**One new field. No new entity, no new type, no reordering of anything that exists.**

| element | change |
|---|---|
| `Record.Queries []string` | **unchanged in type and name**; changes only in *length*, from always 1 to 1–6. Every existing reader already handles N (`summary.go:77`, `eval.RowResult.Queries`). |
| `Record.Query string` | **unchanged** — the raw input, as #11235 §4.4 ruled and §12 q4 left open. Not deprecated here; §14 Q5. |
| `Record.derivationError string` | **new**, `omitempty`, declared between `Queries` and `Anchor` (§7.5). |
| `Disposition.Sources []Source` | **unchanged.** It already answers *which query carried this node*, for up to six queries, and has since PR #22's fusion step. This is the field that makes the change measurable and it required no work. |
| `internal/eval` types | **unchanged.** |

**No failure record, no derivation record, no provenance object.** The four-week-named-decision gate (#868,
#1136 §5) is met by one field with two named readers — a human reading a 200-OK record and `RenderSummary`
— and by nothing more.

---

## 10. Contracts and Interfaces (Abstract)

| contract | before | after |
|---|---|---|
| **Turn → `Retrieve`** | a one-element slice holding the raw input | a 1–6 element slice, input at index 0 | 
| **`loop` → `ModelPort`** | `Judge(ctx, JudgeInput) (JudgeResult, error)` | the same, plus `Derive(ctx, prompt, maxOutputTokens) (string, error)` — text in, text out, no protocol token in either direction |
| **`ModelPort.Derive` → `loop`** | — | completion text, or an error whose sentence is carried into the record and the log unaltered except for bounding |
| **`loop` → the graph** | 1 unscoped recall + 1 scoped | `len(queries)` unscoped + 1 scoped; `queries[0]` is still what the scoped recall carries |
| **`eval.QueriesFor` → `Retrieve`** | its own merge of input + pinned | `loop.MergeQueries`, the product's own rule |
| **Record → reader** | `queries` always `[input]` | `queries` is what was asked; `derivationError` says why it was only the input |

**Invariants.**

1. **`queries[0]` is the raw input on every path** — success, fallback, and sweep. The scoped recall depends
   on it (`retrieve.go:31`, guarded today by
   `TestTheScopedRecallCarriesTheRawInputRatherThanADerivedQuery`).

   **Provenance, because this invariant reads as arbitrary without it and it is not.** The alternative —
   letting a *derived* query carry the scoped recall — was **measured and rejected**: #11365 §6 ran the
   scoped recall on each of the six queries in turn at depth 400 and found the required node **inside the
   3 reserved slots on 0 of 14 miss rows, under any query**. The symmetric variant (fuse the six scoped
   lists and fill the reserve from that) improved scoped ranks and, through the full pipeline, **changed
   nothing: 11/23 retrieved, 9/23 admitted, zero verdict changes.** So this design does not do the thing
   that was measured and rejected, and G-2 keeps it that way. *Recording this matters because compliance
   without provenance is indistinguishable from luck — a future reader would have no way to tell a
   deliberate avoidance from an accident of ordering.*
2. **`loop.Retrieve` holds no `ModelPort`** and its result is a pure function of `(graph state, []string,
   anchor, limits)`. The #11235 §3 ruling is unchanged and its falsifier is unchanged: it breaks the moment
   the model's output can name a node.
3. **A derivation outcome that is not a usable query set produces the same candidate set as a raw-only run**
   — not merely a non-error, which is #11235 G-3's premise and is why the guard asserts set equality.
4. **The derivation call is never counted in `Record.ModelCalls`** and never consumes `MaxModelCalls`.
5. **The product's query set and the sidecar's are produced by one function.** After §7.3 this is literal.

---

## 11. Cross-Cutting Concerns

**Security.** No new outbound host, no new credential, no new configuration variable. The derivation prompt
carries **the task input and two fixed exemplars** to the same endpoint that already receives the input plus
a ~59 kB block, so it exposes strictly less than the call that follows it.

**The carrier question, re-derived from the property rather than from a table — because the table moved.**

> **Corrected in this revision. The claim below replaces a false one, and the way it was false is the
> point.** An earlier revision read: *"The two carriers #13564 §4.3(b) names — `PROCESSOR_MODEL_URL` and
> `PROCESSOR_DIVOID_URL` — reach a derivation cause by exactly the routes they already reach a judgement
> cause, so **#13565's redaction covers this field the day it lands**."* **Nothing in that sentence was
> false when written**, and no word of it has changed since. **The set it quantified over grew.**
> #13565's `CORRECTION 2026-09-11` establishes a **third** carrier — the adapters' own `*url.Error` — the one
> `(*http.Client).Do` returns, wrapped at `openaicompat/client.go:93` and `ollama/client.go:106`; *#13565
> cites the enclosing guards `:82`/`:92` and `:95`/`:105`, and its build-request sites `:83`/`:96` return a
> plain error rather than a `*url.Error`, which does not change its finding* — filed as **#13578**, whose eight
> enumerated sites *include both `condense.go` files*. **Checking my claim against §4.3(b)'s table was the
> defect**: a table is a record of what the last review found, not the property that defines the set.

**The property:** *every route by which a config-derived URL reaches a destination this design opens.* Run
that way rather than by reading a list, the answer for `derivationError` is **three carriers**, and the
third is the one this design uses most directly — §7.2 has both adapters implement `Derive` by delegating
to their existing `Condense` request builder, so a derivation **transport** failure, the most likely
derivation failure of all, produces exactly that third-carrier `*url.Error` and this design carries it into
the record and the substance.

**What follows, and what does not:**

- **This design still adds no new carrier.** The route is one #13578 has already enumerated; `Derive`
  reaches it by delegation, not by opening a new one. That half of the old claim survives re-derivation.
- **But the gate is `#13565` *and* `#13578`, not #13565 alone** — and #13578 is **open**, listed by
  #13534 §9 as not started and blocked. So the honest statement is: *the redaction that covers this field
  is not on `main`, and the one node that would have told a reader so is not the one the old sentence
  cited.*
- **This is a statement of a dependency, not a design of one.** #13565, #13578 and #13564 are separate
  nodes and separate PRs; nothing here specifies their fix, and Unit 1 is not gated on them — the field is
  informative today and the hazard is *permitted-shape, not observed* (#13565: no deployment on record sets
  userinfo, and `PROCESSOR_MODEL_KEY` is the documented alternative).
- **The falsifier for this paragraph** is #13565's own, unchanged in form: build the client over a base URL
  carrying a sentinel in its userinfo, force a **derivation** transport failure, and read `derivationError`.
  The sentinel appears today. **If it does not, this paragraph is wrong and the third carrier does not
  reach this field.**

**Observability.** Success: one `Info` with the derived count. Failure: one `Warn` with the whole cause. The
record carries the queries themselves, and `Disposition.Sources` already carries per-query attribution — so
a bad derivation is *readable* rather than inferred, which is #11235 §4.4's obligation and the reason the
change is measurable at all.

**Determinism.** Lost for a live turn, deliberately, and it was already lost: #13091 §5(b) measured
byte-identical judgement requests producing different completions minutes apart at temperature 0. **Retained
completely for the instrument** — `cmd/eval` constructs no model client, reads a frozen sidecar, and its
candidate lists stay byte-identical across arms.

**Error handling.** One rule, no exemptions: every derivation outcome that is not a usable query set
degrades to `{input}` and reports its cause. No new error kind, no new HTTP code, no new sentinel —
`ErrModelUnavailable` remains the judgement step's failure and is never returned from the derivation step
(#11235 §5, unchanged).

**Concurrency.** `Retrieve` issues its unscoped recalls sequentially (`retrieve.go:18-24`). At six queries
that is six serialised round-trips where there was one. **Parallelising them is a local change inside one
already-extracted function and is deliberately not designed here** — YAGNI until F-3 produces a number.
Filed, §14 Q3.

---

## 12. Quality Attributes and Trade-offs

**The attribute being bought is not retrieval quality.** It is *the instrument measuring the product*. The
retrieval effect is a secondary benefit with a known and modest size, and saying so is worth more than a
rate that hides it:

| arm, 25/25 sidecar, one graph state, zero model calls (#11235 §9.2) | labelled retrieved | labelled admitted |
|---|---|---|
| raw-input (input alone, plus the scope reserve) | 9/23 = 0.39 | 7/23 = 0.30 |
| pinned at 25/25, `derivationHash 2ec61afc` | **11/23 = 0.48** | **9/23 = 0.39** |

**+2 rows on each rate, and the two rows are named: r03 and r12.** r03's winning query was hand-written by
an author who had read the diagnosis first (#11235 §2.2 flags its own contamination); **r12's was generated
blind, by a model that never saw its row's `required`, `subject`, `hash` or `why`** — blindness enforced
structurally by `project_corpus_blind()`, not by care. **So the honest statement of what a live derivation
is expected to buy is: one row of uncontaminated evidence out of twenty-three**, and the ordering argument
does not rest on it.

**And that +2 is not attributable to derivation alone, which makes it weaker still.** Per §2.2, the 9/23
raw-input row was taken with **one** query — no fusion — and the 11/23 pinned row with **six**, under which
fusion is operative. **The comparison therefore moves two variables at once: more queries, and a ranking
algorithm switching on.** That is legitimate as an arm-versus-arm comparison and it is exactly the arm this
design makes the product match — but *"derivation buys +2"* is not what was measured, and the design does
not claim it. What was measured is *"the pinned arm beats the raw arm by 2."* Separating the two would take
a third arm — six queries with fusion disabled — that nothing has swept.

**A live run has since bounded this further, and it bounds it downward.** On 2026-09-10 a task was run
through the container against the live graph whose subject *was this design* (record **#13591**, subject
#10422, one query, 20 candidates, 6 admitted, 14 cut). The document that answers it — **#13585, this
document, at 67,312 B** — came back **rank 1** and was **cut on the byte budget**, as was #11235 at
**96,555 B**. Both exceed the entire 56,592 B candidate budget on their own, so **no ranking change admits
either**:
retrieval was not the failing stage, and six derived queries would have found the same node and lost it the
same way.

**That is not a fact about derivation and it is not an argument against this unit** — the sweep corpus is a
retrieval instrument, where **twelve of fourteen misses never reach the candidate set at all** (#11365 §2),
which is why the +2 is a retrieval figure at all. **But it is the honest ceiling on what a reader should
expect Unit 1 to do for a live task**: where the answer is an oversize node, this changes nothing, and §17
rows 2 and 3 are no longer theoretical (see both, updated).

**Rejected alternatives, each with the reason it lost:**

| # | alternative | why rejected |
|---|---|---|
| **R-1** | **A mechanical rewrite of the input** — stopword strip, keyword-dense line, fixed template, anchor concatenation | §4.2. It cannot emit tokens the input lacks, and **0 of 25** pinned keyword lines are a token subset of their input, median novel-token fraction **0.60** for the keyword line and **0.74** pooled. It would also produce a fourth arm no figure describes. **This is the cheap version the brief warned about reaching past, and it is refuted by measurement rather than by preference.** |
| **R-2** | **Pseudo-relevance feedback** — model-free, graph-vocabulary | §4.3. Structurally the anti-pattern the unit exists to fix; nearest measured relative scored 10/13 against 12/13. **Reversal cost low and additive.** |
| **R-3** | **A separate `QueriesPort`**, as #11235 §4.3 specified | §14 Q2. One implementation, no named second endpoint, and #13238's compile-cycle reason for a separate fill port does not apply here — `internal/loop` declares `ModelPort` itself. An interface with one implementation and no concrete plan for a second is indirection (#1136 §4). **Reversal cost low**, and it is what a spend decision would trigger. |
| **R-4** | **Derivation sees the anchor** (name, type, or body) | §14 Q1, and the grounds are ordered there as they are here — **product-side first**. (a) It would make the derived query set a function of the subject node's **live, unversioned** graph content (#11235 §9.1), adding a *second* independent variance source to the thing being measured. (b) The anchor already reaches retrieval by a better-suited route: `retrieve.go:31` recalls `queries[0]` inside the anchor's two-hop scope with `RecallScopeReserve = 3` slots held for it — **the anchor constrains *where* the graph is searched, the query constrains *what* is asked**, and folding one into the other conflates two separately measurable mechanisms. (c) The anchor's vocabulary already reaches the model: `renderBlock(anchor, admitted)` writes its id, type, name and full content into the block, so query width spent on it buys nothing. (d) *Then* comparability: the sidecar was generated from `{id, input}` alone. **Reversal cost low-to-medium and conditional** — see Q1. |
| **R-5** | **A boolean fallback flag**, as #11235 §4.4 specified | §4.6. Three causes with three different operator responses rendered as one bit — #13564's ruling, one call site over. |
| **R-6** | **Re-author the derivation prompt** for the product | It would discard the only uncontaminated evidence the project holds (r12, produced by this exact prompt) in exchange for a prompt with none. #11361 additionally rules that the prompt's own vocabulary-bridging rationale is **an unmeasured design assumption**, not a finding — which is a reason to leave it alone and measure, not a reason to rewrite it on a different unmeasured assumption. |
| **R-7** | **A third request builder in each adapter** | `Condense`'s request is already exactly a derivation request. **DRY math, measured off the existing builders rather than estimated:** the request/options/response types plus the call body run `internal/ollama/condense.go:34-111` (**78 lines**) and `internal/openaicompat/condense.go:33-106` (**74 lines**) — **~152 lines duplicated across two adapters**, far above #1267's threshold, against a `3 × 2 = 6`-line delegation. |

**Trade-off accepted, named concretely.** Every turn now pays one model call and five extra graph reads for
a retrieval improvement measured at two rows in twenty-three on pinned queries, one of which is
uncontaminated. **That is a poor trade if the goal is the rate, and it is not the goal.** The goal is that
the rate becomes a statement about the product, and there is no cheaper way to buy that: a mechanical arm is
refuted, a feedback arm is a third unmeasured arm, and leaving it alone leaves every constant in
`internal/loop` tuned against an input nobody designed. **If Toni's own runs show the latency is
unacceptable, the two levers are `MaxDerivedQueries` and parallel recalls, both named and both cheap.**

---

## 13. Risks and Mitigations

| # | risk | mitigation | falsifier |
|---|---|---|---|
| **F-1** | **§4.2's class-1 refutation is a tokenizer artifact** | The categorical result (0 of 25 subsets) rests on words the input never spells, not on decimals | §4.2's differential pair, **both arms executed with their output quoted at #13590 §1**: stemmed re-run over all 25 (0/25 survives under Porter and under an aggressive suffix-strip), plus **two** controls — A (stopword strip) proving the subset predicate, and **B (stopword strip + one inflectional variant) proving the stemmer is engaged**, subset under the stemmed run and not under the plain one. **A alone passes even when the stemmer never ran**, which is why B is the arm that matters. |
| **F-2** | **A live model's derivations are worse than the pinned ones** — #11235 R1, still the largest open risk in the line | `Record.Queries` records what was actually asked, verbatim, so a bad derivation is readable rather than inferred | **Unit 2 discharges it**: generate a second sidecar with the product's own `Derive` against the product's own configured model, sweep it and the pinned one in **one session**, and compare arm to arm — never against a rate recorded on another day, because the graph is live and unversioned (#11235 §9.1). Falsified if the live-derived arm does not beat the raw-input arm taken in the same session. **The branch where this detector is silent: Unit 2 not built** — see the note under F-5, which owns the trigger for both. **It is *not* exposed to the admission confound §12 records from record #13591**, because it is a *sweep* measurement and on the sweep corpus twelve of fourteen misses never reach the candidate set at all (#11365 §2). |
| **F-3** | **The latency claim is built from two figures neither of which measured this code** (A6) | Named as a hedge in every sentence that uses them | **Split, because one clock cannot answer both halves.** *(a) Does the derivation step cost what is claimed?* — Unit 1 logs the step's own wall clock on **both** paths (§16 step 5), so the first run that carries it answers this; the cost section is wrong if it exceeds ~5 s on a healthy host. *(b) Do the five extra recalls cost more than the model call?* — **not answerable by that clock**, and Unit 1 does not add the second one. It needs `Retrieve` timed separately, which is Q3's instrumentation and is not in this unit. **Stating (b) as though (a) answered it was the defect; the honest position is that (b) is unmeasured and named.** Either way the levers are `MaxDerivedQueries` and parallel recalls. |
| **F-4** | **The 30 s bound silently converts the derived arm into the raw arm under load** | The cause is recorded and **names the bound by identity** (§7.4) rather than by `ctx.Err()`, which cannot distinguish it from `runBound` | Any run whose `derivationError` names the derivation bound. It is a measurement, not a defect, and it says the host is slow — the distinction #13534 §8 insists on. **The premise that makes this discriminate:** the derivation timeout carries its own named cause, so a run whose *ten-minute* bound expired mid-derivation reports `runBound`, not this one. **Without that, the detector fires on the wrong bound and reads a dying run as a slow step** — and it fires under load, which is the only condition it exists for. |
| **F-5** | **The prompt drifts between Go and Python** between Unit 1 and Unit 2 | §7.1 states the DRY math, names Unit 2 as the discharge, and gives the fallback a **trigger (10 merges) and an owner (a filed task)** | A regenerated sidecar whose shape differs from the product's output for a reason nobody can name. **This detector can only fire if Unit 2 is built** — and Unit 2 is instrument work that #13534 §5 demotes, so it is silent in exactly the branch that threatens the claim. **That is why the fallback carries a trigger instead of a condition:** the trigger fires on merge count whether or not anyone decides anything, and it is the only thing standing in this branch. |
| **F-6** | **`derivationError` lands past the embedding cap for a long input** | §7.5 derives the offset from invariant 1 and states its condition rather than asserting a guarantee | An input over **~2,500** characters (`3L + D + 100 > 7880`). The field is still on the record and still in the HTTP response; only search reachability is lost — the state #13564 lives with for every field after `candidates`. |
| **F-7** | **A future adapter forgets `Derive`** | Go's type system: `var _ loop.ModelPort = (*Client)(nil)` fails to compile. #10466's *"Adding a model provider"* archetype gains a step, as #13345 §11 added one for `RenderToolResult` | A third adapter that compiles without it — impossible by construction, which is the point. |
| **F-8** | **This unit is built and Toni still cannot run a task**, because the container is on a branch | Nothing here depends on the container; #13534 §8 sequences them | Stated, not mitigated. §2.6 measures it. |
| **F-9** | **Fusion becoming operative demotes a node that only one query finds.** New in this revision, and it exists *because* §2.2 establishes fusion is inert today. **#11365 §8 measured it on the sweep's six-query arm:** r13, r16 and r23 are *"single-list, demoted **by the fusion** — one query finds them, five do not; RRF scores **agreement**, and five queries derived from one input agree about that input's concrete surface."* So widening the query set can **lose** a row the raw input alone would have surfaced — and today's product, having no fusion, cannot lose one this way | **None is designed, and that is the ruling, not an oversight.** The arm being adopted is the arm the instrument already scores, demotions included; suppressing them would produce a fourth arm nothing has measured (§4.3 ground 3, same reasoning). The mitigation is that it is **visible**: `Disposition.Sources` already records which query returned each node at what rank, so a demotion is readable per candidate rather than inferred | **`Record.Queries` plus `Disposition.Sources` on any run where the expected node is absent**: if it appears in exactly one query's list at a good rank and still misses the cut, that is this mechanism and not a bad derivation. **Premise:** `Sources` is populated per query in `sourcesOf` (`retrieve.go:39-53`) and carried into every `Disposition`, so the two causes — *never retrieved* and *retrieved by one query and out-voted* — are distinguishable on the record without a new field. **Net direction is measured and is positive but not uniformly so:** the pinned arm beats the raw arm by +2 overall (§12), while #11365 §9 records the per-query split as *"on r17, r19, r22 the raw input beats every derived query; on r13, r14, r16, r20, r23 a derivation wins"* — 3 against 5 at n=11, on a set its own author calls burned. |

---

## 14. Open Questions — taken as decisions

| # | question | decision | alternative | reversal cost |
|---|---|---|---|---|
| **Q1** | Does the derivation see the anchor? | **No — and the grounds are ordered so the first one is the one that carries it.** (a) **It would add a second, independent source of variance to the thing being measured.** An anchor-aware derivation makes the query set a function of the subject node's **current graph content**, which is live and unversioned (#11235 §9.1) — on top of the endpoint non-determinism A5 already concedes (#13091 §5(b)). *Two* uncontrolled inputs to one measurement is a materially worse instrument than one, and this holds with the eval deleted from the picture. (b) **The anchor already reaches retrieval by a better-suited route.** `retrieve.go:31` recalls `queries[0]` inside the anchor's two-hop neighbourhood with `RecallScopeReserve = 3` (`turn.go:13`) slots held for it, and #11288 measured roughly **half** of M3 step 3's retrieval gain as coming from that reserve rather than from derivation. **The anchor constrains *where* the graph is searched; the query constrains *what* is asked** — folding the subject's name into the query text conflates two mechanisms that are currently separable and separately measurable. (c) **The block already carries the anchor's vocabulary.** `renderBlock(anchor, admitted)` (`assemble.go:108-114`) writes its id, type, name and full content, so query width spent there buys nothing. (d) **Only then, comparability:** the sidecar was generated from `{id, input}` alone. | R-4. | **Low, and conditional on Unit 2 — not medium.** Reversal needs a sidecar generated from `{id, input, anchor}`, and **Unit 2 is precisely the machinery that generates sidecars**. If Unit 2 ships, the cost is *one batch of 25 calls and a sweep*. If it does not, the cost is that batch **plus** building the generator — which is Unit 2. **So this deferral is not self-locking, and it is not independent of §7.1's trigger**: the same task that owns the prompt fallback owns this. |
| **Q2** | Separate port or a method on `ModelPort`? | **A method on `ModelPort`.** Reverses #11235 §4.3's `QueriesPort` on two grounds: the compile-cycle reason that forced a separate fill port (#13238 §7.4.4) does not apply, and no second endpoint is named. | R-3. | **Low.** Split the method onto its own port and build a second client in `main`. No record change, no eval change — `Retrieve` never sees a port. |
| **Q3** | Parallelise the six unscoped recalls? | **No, not now.** Six serialised round-trips against an unmeasured per-read cost is a number, not a design problem, and F-3 produces it. | Fan out with `errgroup` in `Retrieve`. | **Low.** One function, already extracted, already pure in its fusion half. |
| **Q4** | Add a feedback pass on top of derivation? | **No.** §4.3. | R-2. | **Low, additive.** |
| **Q5** | Deprecate `Record.Query`? | **No**, unchanged from #11235 §12 q4. It is the raw input, it is what `queries[0]` is by invariant, and removing it is a deletion with a reader inventory owed. **Now genuinely redundant** and a deletion candidate for a later pass with a real inventory in hand. | Delete it in this unit. | **Low**, but it is a second feature and belongs in its own PR. |
| **Q6** | Rename `Condense` to something neutral now that two callers use it? | **No.** It touches two adapters, `internal/condense`, `cmd/condense` and their tests for a naming improvement, in a PR whose feature is elsewhere. | Fold the rename in. | **Trivial**, and it stays trivial. |
| **Q7** | `MaxDerivedQueries = 5`? | **Yes, and 5 is inherited rather than fitted — say so.** It is `QUERIES_PER_ROW = 5` at `scripts/generate_derivations.py:80`, a constant **chosen** in the generator; all 25 sidecar rows carry that shape because the generator was told to. What follows is weaker than "measured" and is the actual reason: **5 is the only value for which an arm-versus-arm figure exists at all**, because the pinned sidecar is the only derived arm this project has swept. Matching it is what keeps §12's comparison a comparison. | 3, or 4. | **Trivial, and free to settle by measurement:** truncate the sidecar's query lists to *k* and sweep. Zero model calls, one session, and #11235 §12 q2 already ruled this a measurement rather than a decision. |
| **Q8** | Does a failed derivation change the HTTP status? | **No.** The run succeeds; the response is 200 and carries `derivationError` on the record. A degraded query set is not a failed run. | Return a warning header or a non-2xx. | **Trivial**, and it would break the closed set of five codes `routes.go:37-43` maintains. |

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

- [x] **No new type whose value-space mirrors an existing one.** No new type at all — one string field, one
      interface method, three pure functions.
- [x] **No new abstraction with one implementation.** Q2 *removes* the abstraction #11235 specified: no new
      port, no new package, no wrapper in `main`.
- [x] **Nothing justified by "we might need X later."** Q1–Q8 each name the concrete alternative and the
      cost of taking it. The prompt is not made configurable; the derived count is a `const` inherited from
      the generator (Q7); **the bound is a `const` chosen on an asymmetry argument, with three ratios as
      sanity checks rather than as its derivation** (§7.4). *Both of those sentences said "derived" in an
      earlier revision, and in both cases the number was picked — #13590 §4.*
- [x] **No deprecation period, feature flag, shim, or transition window.** Records are forward-only
      (#13238 §9.6.5); nothing is dual-written; `Record.Query` is kept because it is still correct, not for
      compatibility.
- [x] **DRY math quoted for every inline-versus-extract call.** Adapter delegation `3 × 2 = 6` — inlined
      (§7.2). The merge rule `9 × 2 = 18` — **extracted**, and on a parity argument rather than the line
      count (§7.3). The prompt — 1,790 B, `17 × 2 = 34` counted in Python or `30 × 2 = 60` estimated in Go,
      **above threshold either way, named as a violation, discharged by Unit 2, and with the fallback given
      a trigger (10 merges) and an owner (a filed task) rather than a condition nobody fires** (§7.1). A
      third request builder — `78 + 74 = ~152` lines **measured off the existing builders**, not estimated
      — rejected (R-7).

**Existing systems first**

- [x] **Existing surfaces audited.** `loop.Retrieve` (multi-query, shipped), the fusion step, `Record.Queries`,
      `Disposition.Sources`, `RenderSummary`'s plural query block, both adapters' `Condense` request
      builders, and `eval.Derivations.QueriesFor` — **all already exist and all are reused.** The only thing
      that did not exist is the step that produces the strings.
- [x] **No new layer proposed**, so none is owed a justification. The one interface method added replaces a
      whole port the predecessor design specified.
- [x] **New persisted data point justified.** One field, two named readers (a human reading the record, and
      `RenderSummary`), enabling one concrete decision inside four weeks: *is the graph being asked what we
      think it is being asked, and if not, is the endpoint, the model, or the bound at fault?*
- [x] **Consumer chain recursed.** `derivationError` → `Record` (HTTP response, read by the operator) **and**
      → `RenderSummary` → the node substance → a human reading a search result. Both terminate in a human,
      not in another projection. `Record.Queries` → `RenderSummary` **and** → `eval.RowResult.Queries` →
      the sweep's machine report.

**Configurability**

- [x] **No new knob and no new environment variable.** The nine in the README's table are unchanged.
- [x] **`MaxDerivedQueries`, the 30 s bound and the derivation ceiling of 1 are `const`s** — no named
      operator will tune them and none differs across environments (#1136 §3).
- [x] **No telemetry-then-tune compound.** Q7's tuning path is a sweep over an existing file, not a knob
      plus an audit column.

**Less is better**

- [x] **can-it-be-deleted / merged / inlined run on every element.** A fallback boolean was deleted, because
      `len(Queries) == 1` already carries the fact (§4.6). A separate port was deleted (Q2). A retry was
      deleted (§4.5). A `derivation` sub-object was deleted down to one field. A third adapter method was
      deleted (R-7). A summary constant was deleted in favour of the existing `summaryQueryRunes` (§7.6).
      **What survives survives because §8 shows the run is unreadable without it.**
- [x] **Trade-offs named explicitly** — §12 states that the retrieval gain is two rows in twenty-three and
      that the rate is not the goal, rather than quoting 0.39 → 0.48 and stopping; that the +2 moves **two**
      variables (queries *and* fusion switching on); and that a live run bounded it further, since an
      oversize answer is lost at admission whatever the ranking. **And the honest gain is now in the
      `## TL;DR`, not only 400 lines down** — a reader who stops at the top block meets the small number
      beside the large ones, which is where the impression of value is formed (#13590 §7).
- [x] **Radical-clean over compromise** where the surface has no consumer: no failure record, no provenance
      object, no derivation sub-object — one field.

**Data deliverables** — not applicable; no SQL, no migration, no backfill.

**Document discipline**

- [x] **#1136, #11034, #12958 and #1267 cited as load-bearing** (header).
- [x] **Reader and scope inventories explicit** — §2.1's swept call-site table, §9's field table, §10's
      contract table, §15's consumer-chain recursion.
- [x] **Out of scope listed explicitly**, eight numbered items (§3), not merely absent.
- [x] **Every exhaustive claim quantifies over a set that was checked, and the check is stated** — §2.1 was
      swept by stem (`quer`) *and* by the datum's name (`.Recall(`), per #1225; §4.1's partition states its
      membership rule and checks its complement; §4.2's 0-of-25 names the tokenizer and its hedge, **and
      the hedge has since been discharged by execution** — the stemmed re-run happened and 0-of-25 held
      (F-1, output at #13590 §1).
- [x] **And one exhaustive claim was checked the wrong way, which is worth keeping rather than tidying.**
      An earlier revision of this bullet read *"§11's carrier claim is checked against #13564 §4.3(b)'s
      table rather than restated"* — presented as the careful option, and it was the defect. **A table is a
      record of what the last review found; the set it names can grow without a word of it changing**, and
      this one had: two carriers became three (#13565's 2026-09-11 correction). §11 now derives from the
      property — *every route by which a config-derived URL reaches a destination this design opens* — and
      says what that returns. **The general form: "is this still true?" does not fire on an enumeration
      that merely became incomplete; "is this an enumeration, and has the set grown?" does.**
- [x] **Exhaustive claims still standing without full discharge, named rather than implied:** §4.1's
      three-class partition rests on the tree containing exactly two outbound clients — re-derivable, not
      re-derived every round; and §13 F-3(b) is stated as **unmeasured** rather than answered.
- [x] **No multi-paragraph rationale for things that obviously stay.** `Retrieve`, fusion, `admit`, the
      block layout and `MaxModelCalls` get one line each in §3.
- [x] **No predecessor superseded.** This document **completes** #11235 §11 step 4 and reverses exactly one
      of its rulings (Q2), saying so at the point of reversal. #11235 stays live and no banner is owed.
- [x] **Every negative claim carries a differential falsifier in the same passage** (#12958 §18.6 clauses 2–3 — clause 2 requires the falsifier be named in the same passage, clause 3 that it be differential)
      — §4.2's F-1 pair, §4.3's explicit labelling of its second ground as an argument by adjacency with no
      arm swept, and F-3's admission that no figure here measured this code.

---

## 16. Implementation Guidance for the Next Agent

**Two units, two PRs, in this order. No git and no `gh` — return the working tree.**

### Unit 1 — the product derives its query *(this is the feature)*

| step | what | acceptance |
|---|---|---|
| 1 | **Extract the merge rule.** `loop.MergeQueries(input string, derived []string) []string` — input first, then each derived query not already present, compared exactly as `derivations.go:92-101` compares today. Re-point `eval.Derivations.QueriesFor` at it. | **Zero-delta:** a sweep of `internal/eval/derivations.json` returns **the same rates as a sweep taken immediately before the change, in the same session.** Not against a rate from another day — the graph is live and unversioned (#11235 §9.1). |
| 2 | **`loop.DerivationPrompt` and `loop.ParseDerivation`**, both pure, no `ctx`, no port — `Assemble`'s discipline. The prompt is transcribed from `scripts/generate_derivations.py`'s `SYSTEM_PROMPT` + `FEW_SHOT`; the parse is its `parse_lines` + `dedupe_against`, capped at `MaxDerivedQueries = 5`. **Do not re-author either.** | Table tests over the twelve `blind-generated` sidecar rows' raw shapes; `ParseDerivation` of a text whose every line is blank returns an empty slice, not nil-vs-empty ambiguity. |
| 3 | **`loop.ModelPort.Derive(ctx, prompt string, maxOutputTokens int) (string, error)`**; both adapters delegate to their existing `Condense` and return `.Text`. Every test fake implementing `ModelPort` needs the method — that is the bulk of the diff and it is mechanical. | `var _ loop.ModelPort = (*Client)(nil)` still compiles in both adapter packages. |
| 4 | **`loop.DeriveQueries(ctx, model, input)`** — prompt, one bounded port call, parse. The 30 s bound is applied **here with `context.WithTimeout`**, never in the adapter. | The bound fires against a fake that blocks, and the returned cause names the bound and the elapsed time. |
| 5 | **`Run` calls it**, merges, and records. `Record.derivationError` declared **between `Queries` and `Anchor`**. One `Info` on success and one `Warn` with the whole cause on fallback, **both carrying the derivation step's own elapsed** — F-3(a) is answerable only if the success path is timed too, and it is one value the step already holds. One summary line. | §8's six failure modes each produce **the same candidate set as a raw-only run** — assert set equality, never non-nil (#11235 G-3's premise). |
| 6 | **The 30 s bound identifies itself.** Apply it as a timeout carrying its **own named cause** and attribute the fallback from that cause, never from `ctx.Err()` — §7.4 shows why the two live deadlines are indistinguishable by error value. | A fixture whose *parent* context expires during a blocked derivation records the **run** bound, and one whose parent is healthy records the **derivation** bound. **Two fixtures, because one passes under the defect.** |

**Guards, each stating the premise that makes it discriminate (#11034 P-42):**

- **G-1 — the derived queries reach the graph.** Assert the graph was asked a **distinctive** derived string.
  *Premise:* a derivation that is computed and never issued produces the raw arm's ranking under the derived
  arm's name — the exact hazard `derivations.go:148`'s error message already names on the eval side.
- **G-2 — the raw input is still `queries[0]`.** *Premise:* `retrieve.go:31` carries `queries[0]` into the
  scoped recall; if a derived query took index 0, the reserve would rank a question the caller did not ask
  inside the neighbourhood.
- **G-3 — every failure branch degrades identically.** Six fixtures, not one: error, non-2xx, bound fired,
  empty text, all-blank lines, and text whose only line echoes the input. *Premise:* a model that refuses
  returns *something* rather than erroring, so an implementation handling only the error branch passes any
  single-fixture test (#11235 G-4).
- **G-4 — the sweep and the turn share one merge rule.** A **cross-package mutation arm**: mutate
  `MergeQueries` in `internal/loop` and assert **both** `internal/loop` and `internal/eval` redden. *Premise:*
  if only one reddens there are two merge rules and the arms are not the same arm — #11142, and the same
  guard shape #11235 G-5 used for `Retrieve`.
- **G-5 — the cause reaches the record and the log.** Assert a distinctive cause string survives into
  `Record.derivationError` **and** that the field is empty on the success path. *Premise (P-17):* an empty
  string is what a dropped field decodes to, so the success-path assertion is what discriminates.
- **G-6 — the derivation call is not counted as a judgement call.** Assert `Record.ModelCalls` is unchanged
  for a turn that derived successfully, against the same turn with derivation failing. *Premise:* §2.3 —
  four of six measured runs already hit the cap, so a silent charge to that budget changes which runs finish.

**Do not:** touch `Retrieve`, `fuse`, `admit`, `Assemble`, the block layout, `MaxModelCalls`, the
supplementary recall, any corpus row, any sidecar row, or `cmd/eval`'s flags. Do not add a config variable.
Do not construct a model client anywhere in `cmd/eval`.

### Unit 2 — one prompt, one shape check *(the instrument; its own PR, sequenced after)*

Replace `scripts/generate_derivations.py` with a Go command that calls `loop.DerivationPrompt`,
`ModelPort.Derive` and `loop.ParseDerivation` and writes a sidecar. **It writes a new file; it does not
overwrite `internal/eval/derivations.json`**, which is the comparability baseline for every arm-versus-arm
figure this project holds (#11348). Preserve the script's `--only` / `--force` refusal semantics and its
structural blindness — the generator must read `{id, input}` and discard every other corpus field before any
model-facing value is constructed.

**What Unit 2 buys, and it is the reason it exists:** it discharges **F-2 / #11235 R1** — the largest open
risk in this line — by turning *"does a real model derive as well as the pinned set?"* from a live-turn
experiment into a 25-call batch followed by two sweeps in one session. It also ends §7.1's prompt
duplication, and it collapses **Q1's** reversal cost to that same batch.

#### Unit 2 carries three deferrals, and the ordering rule works against it — so it needs a trigger, not a hope

**The decisions in this document that Unit 2 discharges, and that nothing else does:** the prompt
duplication (§7.1 / F-5), the live-derivation risk (F-2), and Q1's reversal cost. **The re-derivation rule,
since a list is not a specification:** grep this document for `Unit 2` and read each hit — a site is a
deferral if the decision above it is not settled by Unit 1 alone.

**And that sweep finds a fourth site, which is §1's success criterion itself.** §1 asks that *"a sweep of the
pinned sidecar becomes a statement about the product, because the product runs the same merge rule and
(after Unit 2) the same prompt."* **Unit 1 delivers the merge rule; the prompt half waits on Unit 2.** So
**Unit 1 alone does not fully meet this document's own success criterion** — it meets the half that is
enforced by construction (§7.3's shared `MergeQueries`, guarded by G-4) and leaves the half that rests on
two texts staying equal. That is the same deferral as F-5 wearing different clothes, and it is worth saying
in the success criterion's own words rather than only in a DRY paragraph. **And Unit 2 is instrument work, which
#13534 §5 demotes** — its first bullet is *"Retrieval and admission tuning, until §4.2 gives it a designed
input"*, and instrument work sits behind whatever a real task exposes. So
the unit carrying three deferrals is the one most exposed to the ordering rule this document is written
under, and **F-5 and F-2 are both silent in exactly that branch**: neither can fire unless Unit 2 exists.

**What this document specifies, and what it does not.** *Specified:* a **trigger** — 10 merges to `main`
after Unit 1 — and a **named fallback** that costs one file move and no design (§7.1). *Not specified and
deliberately so:* whether Unit 2 is built. That is #13534's ordering call and Toni's.

**The trigger's owner is DiVoid task #13596**, filed with this revision and linked to this design and to
#13590, carrying the trigger, the fallback and the deferrals it releases. **The operator links it to Unit
1's PR when that PR opens** — this architect runs no `gh` and files no PR (#7506 §1).

**And if the answer is that Unit 2 is not being built, say so as a decision and take the fallback**, rather
than letting the branch arrive by silence. That is the whole of what the trigger buys: it converts *"Unit 2
did not happen"* from a non-event into an event.

---

## 17. What this design does not fix

| # | not fixed | why, and what would |
|---|---|---|
| 1 | **Document-side retrieval failure** (#11235 §2.4). A 357-byte answer inside a 9,519-byte node ranks **34** when queried with its own answering text. | No query reaches it. Sub-node chunking, which is a **DiVoid-side** change (#11235 §8 F1). |
| 2 | **Oversize required nodes.** 163,590 B against a 60,000 B budget. **No longer theoretical, and the instance is this document.** Record **#13591** (2026-09-10, live container, subject #10422) ran a task *about this design*: **#13585 — this document as published at that moment, 67,312 B — came back rank 1 and was cut on the byte budget**, as was **#11235** at **96,555 B**. Six rows admitted, 53.0 kB of the 56,592 B candidate budget (60,000 − a 3,408 B anchor). **Both cut nodes exceed that entire budget on their own.** Written up at **#13592**, whose §3 checked the aperture finding against the code rather than inferring it from the record. | Retrieval finds them; nothing admits them. **And no ranking change reaches them**, which is why this row bounds what Unit 1 can buy (§12) — a derived query set would have found the same node and lost it the same way. Compaction at admission is the measured lever (#11365 §3) and it is a different unit. |
| 3 | **Self-poisoning.** #13091 §6 measured **10 of 20** candidate slots going to this system's own prior run records — so the effective candidate limit for that input was 10, not 20. Record **#13591** reproduces it on a **novel** input — the harder case — at **9 of 20 slots**, ranks 4 through 15, all prior `processor-run` records of 66–72 kB each: **45 % of the aperture on a first-time input** (#13592 §3). The gap between that and #13091 §6's 10-of-20 on a *repeated* input is much smaller than "repeated task text" predicts. Six more queries widen the aperture and **widen this too**. | **Corrected in this revision — the previous mitigation was false against the code.** It read *"`Retrieve` already drops self-produced rows at fusion, so they no longer consume the limit."* **`internal/loop/retrieve.go` never reads `SelfProduced`** — `fuse`'s only exclusion is the anchor. The single production read is `assemble.go:52-53`, inside `admit`, which runs **after** `fuse` has already capped the list at `CandidateLimit`. **So self-produced rows do consume the candidate limit; what they are spared is the byte budget** (the `switch` short-circuits before `cumulative += size`). The old clause also contradicted #13091 §6 *in the same row* — "effective limit is 10, not 20" is only possible because they consume it. **The risk is therefore larger than this row previously allowed, not smaller**, and ~~nothing here mitigates it: they remain what the graph ranks highest for a repeated task text (#11179, #11141). Untouched.~~ **MITIGATED ELSEWHERE 2026-09-11, and the correction above stays exactly as written because it was true of the tree it measured** (`docs/architecture/the-aperture-spends-slots-admission-refuses.md`, #13601). `fuse` now skips a self-produced row on all three fill passes and `Retrieve` asks the graph for more rows than it returns, so the **initial** aperture no longer spends slots on rows admission refuses: the effective candidate limit for such an input is 20 again, not 10 or 11. **What this row still names, and it is not small:** the *supplementary* aperture does not pass through `fuse` and spends its slots exactly as before; the six-query fan-out this design adds multiplies the fetch rather than the aperture, so the mitigation scales with it; and the fetch-side headroom is a bridge with a stated expiry, watched by a WARN rather than assumed. **Nothing in that change is a judgement about what a run record is worth** — the predicate is inherited from `admit`, which already refused these rows. |
| 4 | **The reverse coverage guard** (#11296). `validateDerivations` still checks sidecar → corpus only. | A warn and a coverage figure mitigate it; the guard direction is still open and this design changes no sidecar row. |
| 5 | **Whether the retrieval constants are right for the new arm.** Every one of them was tuned on the old input **and under a ranking the turn will no longer perform** — per §2.2 and #11398, a turn today runs no fusion at all, so `CandidateLimit`, `RecallScopeReserve` and the byte budget were all set against plain similarity order plus a scope reserve. **After this change the first 17 slots are reciprocal-rank fused for the first time**, which is a second reason the constants are untested, independent of the input changing. | That is the *next* question and #13534 §5 demotes it until Toni's own runs say what to aim at. **F-9 is the specific hazard**; #11365 §8 already measured three sweep rows demoted by fusion. |
