# Architectural Document: retrieval that thinks before it looks — derived queries, mechanical fusion, and the limit neither reaches

> **This document is DiVoid node #11235, and this file is its repo copy.** #1176's 2026-09-03 ruling: a design
> that genuinely precedes any code is a node, not a repository PR. It earned this file when the first
> implementation step shipped, and that PR carried it — the condition the paragraph below anticipated, now met.
> Both sides are byte-identical and stay that way (P-40).
>
> Source task **#11134** · Baseline **#11133** · Instrument doctrine **#11142** · A/B protocol **#11069** /
> **#11092** · Project **#10422** · Map root **#10454** · Self-recall exposure **#11179** · Corpus/hash unit
> **#11101**
>
> Standards: Design Contracts **#1136** · Go Code Contracts **#11034**, in particular **P-51** — every
> rationale below carries a named measurement or an explicit hedge **in the same sentence**, and **P-42** —
> every guard names the premise that makes it discriminate.
>
> **Baseline of the tree read here:** `main` at `1dc361f` (PR #22 merged). All measurements labelled
> *(measured 2026-09-04)* were taken first-hand against the live graph at `https://divoid.mamgo.io` during
> this design; everything inherited from #11133, #11134 or #11158 is labelled as inherited and was **not**
> re-measured, except where this document says it re-measured it.

---

## TL;DR

**Toni's shape is right and is adopted.** The model derives several questions from the request; the derived
questions — not the request — are what the graph is asked; results are fused mechanically. **Measured: it
takes labelled retrieval from 8/11 to 10/11 and admitted from 7/11 to 9/11** *(measured 2026-09-04, simulated
against the live graph with the shipped `admit` rule; overall including control, 10/13 → 12/13 retrieved and
9/13 → 11/13 admitted)*.

**But it does not fix the row the source task was written from, and the reason is not a query problem.**
#10883 — r07's required node — is a **9,519-byte task node about an M1 hand-off**, and the passage that
answers r07 is **357 bytes of it, 3.8%** *(measured 2026-09-04)*. Queried with **its own answering passage
verbatim**, that node comes back at **rank 34** *(measured 2026-09-04)*. The control node #10877 comes back at
rank 1 for its own name and for its own opening text. **No query reaches a node whose best possible query
ranks it 34th** — so r07 is a *document-side* failure, not a query-side one, and it is out of reach of every
mechanism in #11134's list and of Toni's shape alike.

**Three failure modes, measured, where #11134 named one.** r03 and r08 are query-side and derivation fixes
them decisively (required node moves from rank 42→2 and 35→6 on individual derived queries). r07 is
document-side and nothing here fixes it. Cross-project crowding is real but is a *third* thing, and it is
cheap to compensate for.

**Ruling on mechanical-vs-LLM retrieval: the founding claim survives, and the boundary is drawn in a type
signature, not in prose.** The model returns `[]string` and nothing else. Everything downstream of that
string slice — which queries are issued, what comes back, how lists are fused, what is admitted — is
mechanical, pure, and shared byte-for-byte between the product and the sweep. **The model widens the
aperture; it never decides what is in it.** The orchestrator's reading is correct, and §3 states the falsifier
that would break it rather than asserting it.

**The sweep stays free**, via pinned derivations in a **sidecar file with its own hash** — deliberately not in
the corpus, because a corpus edit moves `corpusHash` and collides with #11101's unit and #11133's baseline
identity.

**One measurement kills the obvious combiner.** Fusing the lists by **similarity** scores 10/13 — no better
than the baseline — because the nodes that miss, miss *precisely because their similarity is low*. Fusion
must be **rank-based**. *(measured 2026-09-04: union-then-sort-by-similarity 10/13 versus reciprocal-rank
fusion 12/13 on the same lists.)*

---

## 1. Problem Statement

Toni, verbatim, and this document is written to these words:

> *"perhaps we have to think more like an actual thinking process. Currently its more like - question is
> looked up directly in memory (correct me if i'm wrong). What a thinking process does is more like - i look
> at the question and speculate a bit what could be relevant. Keywords, questions which are derived from the
> request - so not the request is used to look up memory, but the request itself is processed logically and
> the results are inputs to the memory. This process then doesn't care for a task or to do anything, its a
> super small step only used to derive questions which arise naturally from the request. […] Each of these are
> then queried against divoid and only an overlap or high relevance results are pushed to the context. Also
> type of nodes could play a role - open tasks, docs, playbooks and such could have different relevance in
> application even though there match-score is the same, just because we are in context task solving."*

With his own caveat: *"these are not high quality derivations now, i just want to give you an example to make
clear what i think"* — so this document takes the shape and not the specific questions.

**"Correct me if I'm wrong" — he is not wrong, and it is verified rather than assumed.** `internal/loop/turn.go`
line 95 is `candidates, err := t.Graph.Recall(ctx, input, CandidateLimit)`. The argument is the caller's raw
input string, unmodified; `record.Query` is set to the same `input`. There is no trimming, expansion,
keywording or rewriting anywhere between the HTTP handler and the embedding *(measured 2026-09-04, read from
the tree at `1dc361f`)*.

**The problem.** One semantic query over the raw request is the whole of retrieval. #11133 measured what that
costs: labelled retrieved 0.73, three rows `notRetrieved`. #11134 diagnosed the cause as a vocabulary gap
between the question's phrasing and the node's. **That diagnosis is half right and the half it misses changes
the design**, which is §2.

**Success criterion.** The **retrieved** rate moves, on the corpus, in a re-run sweep — and the reader can say
which of the three failure modes each surviving miss belongs to.

---

## 2. What is actually broken — three failure modes, not one

#11133 states of the three misses: *"That is not three unlucky rows. It is a **class**."* **Measured against
the live graph, they are three rows in two classes, and one of them is not a retrieval problem at all.**

### 2.1 The baseline was reproduced before anything was proposed

Issuing every corpus input as a plain `GET /api/nodes?query=…&count=20` returns **10 of 13 required nodes**,
and r07's top similarity is **0.68031085** *(measured 2026-09-04)* — digit-for-digit the figure #11134 records,
and consistent with #11133's 8/11 labelled + 2/2 control. **The instrument used for every number below agrees
with the harness's own reading**, which is the precondition for believing any of them.

### 2.2 Mode A — query-side. The question is a bad query. **Derivation fixes it.**

For r03 and r08 the required node is reachable; the raw phrasing simply ranks it far down. Depth of the
required node per query, at `count=100` *(measured 2026-09-04)*:

| Row | raw input | best derived question | derived keywords |
|---|---|---|---|
| r03 → #10877 | **42** | **2** (*"Why would a mutation leave the suite green?"*) | **6** |
| r08 → #10839 | **35** | **7** (*"What does an exit code mean that the per-package output does not show?"*) | **6** |
| r07 → #10883 | **75** | **55** (*"How are environment variables supplied to this binary at launch?"*) | **>100** |

This is Toni's mechanism working exactly as he describes it, and the improvement is not marginal — rank 42 to
rank 2 is the difference between invisible and first.

**Caveat, and it is the same one #11134 attaches to its own rephrasing test.** The derived questions above were
written by me from the input text, and I had read #11134's diagnosis first, so **I am contaminated for r03,
r07 and r08.** What this measurement establishes is that *a good derivation exists and is reachable from the
question's own words*; it does **not** establish that a model will produce one. That is what §9's A/B is for,
and it is the single largest open risk in this design (R1).

### 2.3 Mode B — pool-side. Cross-project crowding. **Scope compensates, at zero model cost.**

Sixteen of r07's twenty candidate slots go to other projects' secret-hygiene material, several in German
*(inherited from #11134; the top-8 was re-measured 2026-09-04 and matches)*. The graph holds **10,364** nodes
matching r07's query at all.

`linkedto` composes with `query` on `GET /api/nodes`, so a semantic query can be ranked **inside** a
link-scoped set in one request *(measured 2026-09-04)*. Scoping to the anchor plus the anchor's neighbours —
two hops, a 297-node pool for anchor #10422 — puts all three missed nodes in the top 20: ranks **5, 10, 3**
for r03, r07, r08 *(measured 2026-09-04)*.

**Two things about this are load-bearing and both were measured rather than reasoned:**

- **One hop is not enough.** Scoping to `linkedto=<anchor>` alone scores **10/13** — no better than baseline.
  The required nodes hang off the project's `Tasks` (#10434) and `Docs` (#10423) *group* nodes, one hop
  further out. The extra `GET /api/nodes/links` call earns its place *(measured 2026-09-04)*.
- **`rootNodeId` is not a usable scope here.** `?rootNodeId=10422` returns `{"result":[],"total":0}` — the
  Processor nodes carry no `rootNodeId` grouping *(measured 2026-09-04)*. Worse, the spellings `rootnode` and
  `root_node_id` are **silently ignored** and return the unscoped result, so a typo'd scope parameter produces
  a full-graph search that looks like a working one. Guard G-6 exists for this.

### 2.4 Mode C — document-side. The answer is 3.8% of its node. **Nothing here fixes it.**

r07's required node **#10883** is a `task` named *"[processor] M1 hand-off: one live turn against a real
OpenAI"*, **9,519 bytes**, about protocol fidelity, model runtimes and PATH probes. The passage that answers
r07 is a **357-byte** correction inside it — **3.8% of the body** *(measured 2026-09-04)*.

**The decisive measurement.** Query the graph with that passage's own text, verbatim:

| Query | Required node's rank |
|---|---|
| #10883's **own answering passage**, verbatim | **34** |
| #10883's own **name** | **1** |
| control #10877's own name | 1 |
| control #10877's own first 400 characters | 1 |

*(all measured 2026-09-04, `count=100`)*

**A whole-node embedding of 9,519 bytes does not represent a 357-byte passage inside it.** The best query that
could ever be written for r07 — the answer's own words — ranks the node 34th. So:

> **No reformulation, no derivation, no HyDE, no keyword expansion and no amount of fusion can retrieve
> #10883 for r07's question. The ceiling is a property of the document, not of the query.**

This is why #11134's *"one failure, not three"* has to be corrected: r03 and r08 are one failure; r07 is a
different one, and it is the one the task was written from. **The product's most-cited miss is the one this
design does not fix**, and saying so is worth more than a rate that hides it.

**What would fix it:** sub-node retrieval — chunking a node's body and embedding the chunks — which is a
change to how the *graph* represents content, not to how the loop queries it. That is a DiVoid-side unit, not
a Processor one, and §8 files it rather than designing it here.

### 2.5 Mode D — not a failure. r09.

`cut at rank 7`, required node **163,590 bytes** against a 60,000-byte budget *(measured 2026-09-04)*. The
deliberate oversize specimen (#11046 gap 1). Retrieval finds it; nothing can admit it. Unchanged by this
design and correctly so.

---

## 3. The ruling on mechanical-vs-LLM retrieval

**The question, restated.** #10422 claims *"every input triggers a **mechanical (no-LLM) retrieval** against
the graph, and the model call is built from that."* A derivation step is a model call before retrieval.
Violation, or refinement?

**Ruling: refinement, and the boundary is enforceable rather than rhetorical.** The orchestrator's reading —
*"the model proposes what to ask; it never decides what comes back"* — is correct, and this document does not
adopt it on its own authority. It makes it **checkable**, which is what P-51 asks for and what a convenient
reconciliation would not survive:

> **The derivation step's entire output type is `[]string`.** It cannot return a node id, a score, a
> ranking, a weight, an inclusion, an exclusion, or a byte of content. Everything from that string slice
> onward — which queries are issued, what the graph returns, how the lists are fused, what survives the byte
> budget — is executed by code the model does not touch, and is a **pure function of `(graph state, []string,
> anchor, limits)`**.

**The falsifier, stated as #1220 §5 requires.** The claim breaks the moment the model's output can name a
node — a `preferredNodes` field, a re-rank instruction, an exclusion list, a "boost these ids" hint, or a
free-form blob a later step parses ids out of. **If any of those is ever added, this ruling is void and the
founding claim is genuinely violated.** Guard G-4 exists to make that non-silent.

**Why this is the property worth protecting rather than "no model call anywhere".** The inversion in #10422 is
not an aesthetic preference for model-free code. It is the guarantee that **a model cannot talk its way into
its own context** — the same guarantee the workflow-obligations design states as *"an agent cannot talk its
way past a gate"*. A model that proposes a question and receives whatever the graph independently returns has
no route to smuggle content in: it can ask a bad question and get bad results, which is a quality problem, not
a soundness one. **A model that could name what comes back would have that route.** The `[]string` boundary is
exactly the line between the two.

**Honest cost of the ruling.** Two things get worse and both are real:

1. **Retrieval acquires a failure mode it did not have.** §5 rules the degraded mode.
2. **A live turn is no longer deterministic given the graph state**, because the derivation is a model call.
   The *sweep* stays deterministic (§4.3), so the instrument keeps the property; the product loses it. That is
   a genuine loss and it is the reason §9 keeps the sweep as the primary measurement rather than the A/B.

---

## 4. The design

### 4.1 Shape

```
  input, subject
      │
      ├─ Graph.Node(subject) ─────────────────────────────► anchor          (unchanged)
      │
      ├─ Queries.Derive(input)  ── model call ── []string ──┐  §5 rules failure
      │      degraded: [] or error  →  fall back to {input} │
      │                                                     ▼
      │                                        queries = {input} ∪ derived
      │
      ├─ Graph.Neighbours(subject) ──────────────► scope = {subject} ∪ neighbours(subject)
      │
      ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │  loop.Retrieve  —  MODEL-FREE BY CONSTRUCTION (takes []string, not   │
  │  a ModelPort). One implementation, called by BOTH turn.go and        │
  │  cmd/eval/sweep.go.                                                  │
  │                                                                      │
  │    for each q in queries:  Graph.Recall(q, CandidateLimit, nil)      │
  │    once:                   Graph.Recall(input, CandidateLimit, scope)│
  │    fuse: reciprocal-rank over the unscoped lists,                    │
  │          then RecallScopeReserve slots from the scoped list,         │
  │          then backfill from the fused list, capped at CandidateLimit │
  └──────────────────────────────────────────────────────────────────────┘
      │
      ▼
  Assemble(anchor, candidates, AssemblyByteBudget)    — UNCHANGED, still pure
      │
      ▼
  judge → write back                                   — UNCHANGED
```

### 4.2 The measured combiner, and the two alternatives it beat

**Fusion is reciprocal-rank over the unscoped lists**, `score(n) = Σ 1/(k + rank_i(n))` with `k = 10`, then a
**reserved allocation** of `RecallScopeReserve = 3` slots for the anchor-scoped list, then backfill from the
fused order.

| Combiner | retrieved | Why it is not the design |
|---|---|---|
| **Reciprocal-rank + 3 reserved scoped slots** | **12/13** | **chosen** |
| Union of all lists, **sorted by similarity**, top 20 | **10/13** | **The obvious combiner and it buys nothing.** The missed nodes miss *because* their similarity is low; sorting by it re-creates the baseline. This is the single most important negative result in the document |
| Rank-interleave, 50/50 between fused and scoped | 13/13 | Reaches r07 — at rank **20 of 20** — and pays for it: r01 drops 1→8, r02 4→7, r04 3→7. It buys one row's survival at the exact margin with a general degradation, and §2.4 shows that row's survival is luck against a node ranked 34 on its own text. **A design that claimed 1.00 on this basis would be claiming a coincidence** |
| Reserved allocation at 5, 7 or 10 slots | 12/13 at 5 and 7; 12/13 at 10 (r07 enters at 19, r02 falls out) | No reserve size reaches 13/13 without the 50/50 split. **12/13 is the measured ceiling of this family**, and reserve=3 is the smallest value achieving it |
| Derived queries **without** any scoped list | 12/13 | Equal on the headline; ranks are comparable. **Kept the scoped list anyway**, and the reason is stated below rather than hidden |
| Anchor-scoped list **without** derivation | 13/13 | The 50/50 interleave case above. Same objection, and additionally it does nothing for the class Toni's shape addresses |

*(all measured 2026-09-04, same corpus, same instant, one graph state)*

**Why the scoped list stays when it does not move the headline.** It costs no model call and one extra
request; it is the only arm that reaches r07 at all; and it is the mechanism that generalises to the
cross-project crowding #11134 measured, which is a property of a **shared** graph that will get worse as other
projects grow, not better. Deleting it would be justified by today's 12/13 and would be re-derived within a
quarter. This is the one place in this document where an element survives the can-it-be-deleted test on a
forward argument rather than a present measurement, and it is flagged as such (#1136 §4).

### 4.3 Where the code goes, and the drift hazard that must be closed first

**Finding, and it is a precondition rather than a refinement:** `cmd/eval/sweep.go` does **not** call
`Turn.Run`. Its `rowDispositions` re-implements the retrieval half — `graph.Node`, `graph.Recall`,
`loop.Assemble` — as its own copy *(measured 2026-09-04, read from the tree at `1dc361f`)*. Today the copy is
three lines and the drift risk is low. **The moment the pipeline grows a fan-out and a fusion step, two
implementations of it exist, and the instrument stops measuring the product** — which is #11142's rule
arriving in the one place that rule was written about.

**So the first implementation step is an extraction, before any new behaviour.**

| Package | Change |
|---|---|
| `internal/loop/retrieve.go` **(new)** | `Retrieve(ctx, graph GraphPort, anchor Anchor, queries []string, limit, reserve int) ([]Candidate, error)`. Fan-out, the scoped recall, and fusion. **Takes no `ModelPort`** — the mechanical boundary of §3 expressed as a signature. The fusion itself is a separate **pure** function with no `ctx` and no port, on `Assemble`'s discipline |
| `internal/loop/turn.go` | `Run` calls `Queries.Derive`, then `Retrieve`. `Assemble`, `judge`, `dispatchRecall`, `logFinished` unchanged. New constants `RecallScopeReserve = 3` and `MaxDerivedQueries` beside the existing five |
| `internal/loop/types.go` | `QueriesPort` with one method, `Derive(ctx, input string) ([]string, error)`. `Record` gains the derived queries and the per-query provenance of each candidate — §4.4 |
| `internal/loop/turn.go` (`GraphPort`) | `Recall` gains a `scope []int64` parameter, `nil` meaning global — one method rather than a near-duplicate `RecallScoped`. New `Neighbours(ctx, id int64) ([]int64, error)` |
| `internal/divoid/client.go` | `Recall` emits `linkedto` when scope is non-empty; `Neighbours` reads `GET /api/nodes/links?ids=<id>` and returns the other endpoint of every incident edge, **sorted**, so the scope is order-stable |
| `cmd/eval/sweep.go` | `rowDispositions` calls `loop.Retrieve` — the same function the turn calls. Queries come from the pinned sidecar (§4.5) or, absent one, are `{row.Input}` alone |
| `cmd/eval/main.go`, `internal/eval` | `-derivations <path>` flag; `derivationHash` and the arm's name reported beside `corpusHash` |

**Not changed, deliberately:** `Assemble` and `admit` — the byte budget, the skip rule from #11158, the
self-produced cut, the anchor-first block layout, `Disposition`, both rates, the four verdicts, the exit code,
and `Turn.judge`'s supplementary-recall loop.

### 4.4 What the record must carry, or the change is unmeasurable

`Record.Query` is a single string today and becomes a lie the moment several queries are issued. **Both of the
following are obligations, on #11066's argument that evidence not written into the result is unrecoverable
because the next read queries a different graph:**

1. **The derived queries, verbatim**, plus whether they came from the model or from the raw-input fallback.
   Without the flag, a run that silently degraded to today's behaviour is indistinguishable from one that did
   not — which is the failure #11133's own 0.64/0.64 trap is about.
2. **Per candidate: which queries returned it, and at what rank in each.** This is what makes *overlap* — the
   part of Toni's proposal that is doing the work — visible in the record instead of inferred. It is also the
   only way a future reader can tell a node that surfaced on five derived queries from one that scraped in on
   the reserved scope slot.

`Record.Query` is **retained and keeps its meaning as the raw input**; the derived set is a new field beside
it. Nothing shipped is redefined.

### 4.5 The sweep stays free — and what that costs, named

Corpus inputs are constants, so a derivation for a corpus row is a pure function of a constant and can be
computed once and pinned. **Ruling: pinned derivations live in a sidecar file, not in the corpus.**

**Why not in the corpus, and this is the collision the orchestrator asked about.** `eval.Load` hashes the
corpus file's raw bytes as `Corpus.Hash` *(measured 2026-09-04, `internal/eval/corpus.go`)*. Adding
derivations to it moves `corpusHash`, and **#11133 identifies the baseline by `corpusHash 6c1ba696`** — the
figure #11101 item 9 already flags as the one that will be forgotten. Putting derivations in the corpus would
move it a second time, for a second reason, in a unit that has already been deferred four times. A sidecar
with its own `derivationHash` keeps the two identities independent and keeps this design off #11101's
critical path entirely.

**The cost, stated rather than glossed:** the sweep then measures retrieval against a **frozen** derivation,
not the one a live turn would produce. The two diverge the moment the deriving model or its prompt changes,
and **the sweep cannot detect that divergence** — it has no model to compare against.

**That is acceptable, and the reason is what the sweep is for.** #11092 §3 already rules the sweep the
*zero-variance* layer of a three-layer instrument set. A frozen derivation is not a defect in that role; it is
what makes the arm reproducible. The divergence is detected on the other layer: the two-turn smoke rig runs a
live derivation and prints it, so a drifting deriver shows up as derived questions that no longer resemble the
pinned ones. **`derivationHash` is what makes that comparison possible**, and a sweep whose sidecar hash is
not recorded beside its result is a sweep whose arm nobody can identify later.

### 4.6 Node type as a weight — deferred, with the measurement that would settle it

Toni: *"type of nodes could play a role… different relevance in application even though there match-score is
the same."* **The observation is correct: retrieval ignores `type` entirely today** — the field is projected
and carried into `Candidate` and `Disposition` and never read by any decision *(measured 2026-09-04)*.

**Deferred, and P-3 requires the reason to be the blast radius rather than taste.** I have **no measurement
that type discriminates on this corpus**, and one datum argues it would not have helped here: r07's crowding
was other projects' `documentation`, and r07's required node is a `task` — but r03's required node is
`documentation` and r08's is a `task`, so no single type ordering separates the misses from the crowd. A
weight with no measured direction is a knob with a named operator of "nobody", which is #1136 §3.

**What would settle it, and it is cheap:** the sweep already records every candidate's `type`. Once #11066's
retained candidate set is on the row result, a one-off count of *type of required node* versus *type of the
candidates that outranked it*, across the corpus, is arithmetic over data already captured — no new
mechanism, no sweep change. **If a direction appears there, the fusion step of §4.2 is exactly where a
per-type multiplier goes**, and it goes in at that point with a measurement behind it. Filed in §8.

---

## 5. The degraded mode — ruled, not assumed

Retrieval can now fail in a way it could not before. **Ruling: any derivation outcome that is not a usable
query set degrades to `queries = {input}`, which is today's shipped behaviour exactly.**

| Outcome of `Derive` | Action |
|---|---|
| Error, timeout, transport failure, non-2xx | fall back to `{input}`; **log at warn**; record the fallback flag |
| Returns empty, or only blank/whitespace strings | identical treatment — **this is a distinct branch from the error branch and a distinct test**, because a model that refuses usually returns *something* rather than erroring |
| Returns more than `MaxDerivedQueries` | truncate to the first `MaxDerivedQueries`; the raw input is always included regardless |
| Returns usable queries | `{input} ∪ derived` |
| Port is nil | `{input}`. The zero value degrades rather than panicking — P-33's injected-collaborator archetype, step 6: **bound what the fallback returns** |

**The turn does not fail because derivation failed.** A derivation failure costs retrieval quality, and
retrieval quality was 0.73 before this design existed; erroring the whole run instead would trade a degraded
answer for no answer, which is strictly worse. **`ErrModelUnavailable` is not returned from the derivation
step** — it stays what it is, the judgement step's failure.

**The degraded path must be visible, not silent.** A run that fell back is a run whose retrieval is the old
retrieval, and a reader comparing it against a sweep taken on the new arm would be comparing two different
systems. Hence the warn log and the record flag, and hence guard G-3, whose premise is that the fallback
produce **the same candidate set as a raw-only run** — not merely that it produce something.

---

## 6. What this does not fix

| # | Not fixed | Why, and what would |
|---|---|---|
| 1 | **r07, the row the task was written from** | §2.4. Its answering passage is 3.8% of a 9,519-byte node and the node ranks **34** on that passage's own text. Fixed only by sub-node retrieval — chunk-level embedding — which is a **DiVoid-side** change, not a Processor one (§8 F1) |
| 2 | **r09** | The required node is 163,590 bytes against a 60,000-byte budget. Retrieval already finds it; nothing here or anywhere admits it whole |
| 3 | **The admitted rate's structural blindness to admission changes** | #11133's trap is intact and is **not** contradicted by this design moving the admitted rate. The rate is blind to *admission* changes on this corpus because only r09 is `cut` and it exceeds the budget either way; it is **not** blind to *retrieval* changes, because a required node newly entering the top 20 and fitting the budget flips its verdict. r03 and r08 do exactly that *(measured 2026-09-04)* |
| 4 | **The vocabulary gap in general** | Derivation narrows it and does not close it. Mode A is fixed for the two rows measured; the population of symptom-phrased questions is not eleven rows wide, and #11092 §5.3's blindness — tasks no node answers — is untouched |
| 5 | **Self-recall (#11179)** | Not fixed, and **made more exposed**, which is §7 R4. This design increases how much of the graph is reachable per turn, and run records are part of that graph |
| 6 | **Determinism of a live turn** | Lost, deliberately (§3). Retained for the sweep |

---

## 7. Risks

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| R1 | **A real model's derivations are worse than the hand-written ones this design was measured on.** The largest risk in the document | §9's A/B compares a live-derived arm against the pinned arm on one graph state; §4.4 records the derived queries verbatim so a bad derivation is readable rather than inferred | a sweep on live derivations scores at or below 10/13 |
| R2 | **The design was selected on the same 13 rows it is scored on.** Six combiners were compared and the best kept — that is selection on the test set, and the honest reading of 12/13 is *an upper bound* | Stated here rather than mitigated. The corpus was authored before this design existed and none of its rows were changed; #11092 §11 Q1's task-set question is the place this gets resolved | the arm's advantage shrinks on any row added to the corpus after this document |
| R3 | **Cost per turn rises from 1 graph read to 1 + N + 2** (N derived queries, one scoped recall, one links read), plus one model call | All reads are `GET`s against one host; the model call is small — a derivation prompt carries the input, not the block. **The sweep's model cost stays zero** (§4.5) | a turn's wall time or graph error rate rises materially in the smoke rig |
| R4 | **More of the graph reachable means more self-produced content reachable** (#11179) | `admit` already cuts self-produced rows (#11158) and the sweep counts them. **The smoke rig must report the self-produced count among admitted rows**, which #11179 asks for anyway | a two-turn smoke run admits a `processor-run` record and nothing reports it |
| R5 | **The sweep and the turn drift apart** — the instrument stops measuring the product (#11142) | §4.3's extraction: one `Retrieve`, two callers, and G-5's cross-package mutation arm | a fusion constant is mutated and only one of `internal/loop` / `cmd/eval` reddens |
| R6 | **A silently-ignored scope parameter yields a full-graph search that looks scoped** — measured: `rootnode` and `root_node_id` are accepted and ignored (§2.3) | G-6 | a scoped recall returns a candidate outside the scope set and nothing fails |
| R7 | **The derivation step becomes a place to smuggle node ids**, voiding §3's ruling | G-4, and the ruling states its own falsifier | `Derive`'s return type is ever anything but `[]string`, or any downstream code parses ids out of a derived query |

---

## 8. Filed rather than designed here

| # | Item | Where it belongs |
|---|---|---|
| F1 | **Sub-node (chunk-level) retrieval.** §2.4's measurement is the evidence: a 357-byte answer inside a 9,519-byte node is unreachable by any query, and this is a property of the substrate's embedding granularity | **DiVoid**, not Processor. File as a DiVoid task citing the rank-34 measurement |
| F2 | **Type weighting.** §4.6. The settling measurement is arithmetic over #11066's retained candidate set | Processor, after #11066 lands |
| F3 | **The `Recall`-signature change touches #11092 §4.2's decorating `GraphPort`.** That decorator implements the port as it stands today; widening `Recall` widens the decorator | Note on #11092 — no design change, but the A/B runner's brief must know |

**Collision check against #11101, run rather than assumed:** this design changes no corpus row, so
`corpusHash` does not move and `6c1ba696` keeps identifying #11133's baseline. The sidecar is a new file with
its own hash. **This unit does not enter #11101's queue**, which is the point of §4.5's ruling.

---

## 9. Verification

**Which rate moves, and why it can.** The **retrieved** rate. It is at 0.73 labelled, it is the number
#11134 was filed against, and a required node entering the top 20 flips its verdict — which is exactly what
r03 and r08 do. The **admitted** rate also moves here, and #11133's trap is *not* violated by saying so: that
trap is about admission changes on a corpus with one `cut` row, and this is a retrieval change (§6 item 3).

**Predicted, from the simulation** *(measured 2026-09-04, against the live graph with the shipped `admit`
rule and pinned hand-written derivations)*:

| | labelled retrieved | labelled admitted | control |
|---|---|---|---|
| baseline (#11133) | 8/11 = 0.73 | 7/11 = 0.64 | 2/2 |
| this arm, simulated | **10/11 = 0.91** | **9/11 = 0.82** | 2/2 |

**These are a prediction from a simulation, not a sweep result.** The simulation reproduces `admit`'s rule in
Python against live content; it is not the binary. **A number this document predicts and a number a sweep
returns are not the same kind of thing** — which is #11133's own 0.64/0.64 lesson, and the reason the
acceptance criterion below is a re-run and not a match against this table.

### Acceptance

1. **A sweep on `main`'s arm** (no `-derivations`) returns **0.73 / 0.64 / control 1.00** with `corpusHash
   6c1ba696`. If it does not, the graph moved and nothing below is comparable — stop and re-baseline.
2. **A sweep on the pinned arm**, same session, minutes apart, returns the retrieved rate. Both readings carry
   `corpusHash`, `derivationHash` and the arm's name. **Both are free of model calls**, so this is the whole
   A/B for the retrieval question and it costs nothing.
3. **`admittedBytes` and `admittedCount` are carried on both readings**, per #11133's explicit instruction —
   a rate match across a change that moves context is the trap that node exists to prevent.
4. **A live-derivation reading** on the same graph state, to answer R1: do a real model's derivations score
   like the pinned ones? This one costs model calls, one per row, and is the only part that does.
5. **Two-turn smoke run** (`scripts/smoke.py`), which is the only surface for what no rate sees (#11142): the
   derived queries printed verbatim, the block, the answer, the admitted count, **and the self-produced count
   among admitted rows** (R4). A single-turn run re-creates the blindness the rig exists to remove.

**Do the existing rates suffice? For the retrieval question, yes — and only because of item 3.** The rates
answer *did the required node arrive*. They do not answer *did the derivation help or did the scope reserve
carry it*, and the per-candidate query provenance of §4.4 is what answers that. Without it, a 12/13 is a
number with no attribution, which is #11092 §5.4's single-scalar objection one level down.

**The A/B of #11069/#11092 is not required for this change and should not be waited on.** Its blocking
precondition — write-back suppression — is unresolved, and the sweep answers the retrieval question at zero
cost. The A/B answers *did the answers get better*, which is the next question and not this one.

---

## 10. Guards (P-42 — each states the premise that makes it discriminate)

| # | Guard | The premise, without which it does not discriminate |
|---|---|---|
| **G-1** | Fusion is rank-based, not similarity-based | The fixture's correct node must have the **lowest similarity of any candidate** while appearing in **several** lists. On any fixture where rank order and similarity order agree, a similarity sort passes. *Measured basis: similarity-union scores 10/13 against rank fusion's 12/13 on identical lists* |
| **G-2** | The scope reserve is a reserve, not a concatenation | The fixture's scoped list must contain a required node that the fused order places **beyond the cap**. An implementation that appends the scoped list after the full fused list passes every fixture where the fused list is shorter than the cap |
| **G-3** | A derivation failure produces the raw-input candidate set | Assert the **candidate set equals a raw-only run's**, not that the call returned without error. A test asserting non-nil passes an implementation that returns an empty set on failure — which is a shutout, not a fallback |
| **G-4** | An empty or all-blank derivation degrades exactly as an error does | Two fixtures, not one: `([], nil)` and `(nil, err)`. They are different branches, and a model that refuses returns the first. An implementation handling only the error branch passes any single-fixture test |
| **G-5** | The sweep and the turn share one retrieval implementation | A **cross-package mutation arm**: mutate the fusion constant in `internal/loop` and assert **both** `internal/loop` and `cmd/eval` redden. If only one reddens there are two pipelines, which is R5 already realised and invisible to any single-package test |
| **G-6** | A scoped recall is actually scoped | Assert **every returned candidate is inside the scope set**, against a fixture whose unscoped result contains an out-of-scope node ranked first. Premise: the API **silently ignores** unknown scope parameters and returns the unscoped result *(measured 2026-09-04: `rootnode=` and `root_node_id=` return the full-graph ranking)* — so a wrong parameter name produces a plausible, wrong, passing result |
| **G-7** | The scope is two hops, not one | The fixture's required node must be a neighbour **of a neighbour** of the anchor and **not** a direct neighbour. Premise: measured, one-hop scoping scores 10/13 and two-hop 13/13 on the scoped arm; a fixture where the required node is directly linked cannot tell them apart |
| **G-8** | `Neighbours` is order-stable | Assert the returned slice is sorted against a fixture whose edge rows arrive out of order. Premise: the scope feeds a `linkedto` list, and an unstable scope makes the sweep non-reproducible for a reason no rate would ever surface |
| **G-9** | The derived queries and the fallback flag reach the record | Assert a **distinctive** derived string survives into `Record`, and that the flag is `true` on the fallback path and `false` otherwise. Premise (P-17): a `false`/`""` expectation is what a dropped field decodes to |

**Not guarded, deliberately:** derivation *quality*. No test can assert that a model's questions are good ones;
that is what §9 item 4 measures and what R1 carries.

---

## 11. Implementation order

| # | Step | Why this order |
|---|---|---|
| 1 | **Extract `loop.Retrieve` and have `cmd/eval/sweep.go` call it**, with today's single-query behaviour and no new capability. Sweep re-run must return **0.73 / 0.64 / 1.00** unchanged | A pure refactor with a **zero-delta acceptance criterion** — the only step whose correctness is checkable against an existing number. Doing it after the behaviour change would leave nothing to check it against |
| 2 | **`GraphPort.Recall` scope parameter + `Neighbours`**, `internal/divoid` implementing both. Still no derivation | G-6, G-7, G-8 land here. Independently valuable and independently reviewable |
| 3 | **Fusion + the scope reserve**, wired into `Retrieve`. Sweep gains `-derivations`; a pinned sidecar is authored | G-1, G-2 land here. **This is where the retrieved rate is expected to move, with zero model calls on either arm** — the whole A/B of §9 items 1–3 is runnable at the end of this step |
| 4 | **`QueriesPort`, the derivation prompt, the degraded mode, the record fields** | G-3, G-4, G-9. The live half. Deliberately last, because steps 1–3 are measurable without a model and this one is not |

**Step 3 is the milestone.** If its sweep does not move the retrieved rate, step 4 should not be built —
because at that point the pinned derivations *are* the hypothesis, and a model that must produce them is a
cost with nothing measured behind it (P-3).

**Each step is one PR** (P-46). Step 1 carries no behaviour change and should say so in its body.

---

## 12. Open questions

1. **Which model derives?** The judgement model is a 480B coder proxy. Derivation is a small, cheap,
   high-frequency step and may want a different (smaller, local) endpoint. `QueriesPort` is a separate port
   partly so this stays open at zero present cost. **Needs Toni** — it is a spend decision.
2. **How many derived queries?** Four plus keywords was the shape measured. `MaxDerivedQueries` is a constant
   and the sweep can test values at zero model cost once the sidecar exists — so this is a measurement to
   take, not a decision to make now.
3. **Does the corpus need rows for mode C?** Every corpus row's required node is a whole node. §2.4 says the
   substrate cannot retrieve a passage inside a large node about something else; **no row measures that
   directly**, and r07 measures it only by accident. Adding one deliberate row would make F1's case
   measurable — and it moves `corpusHash`, so it belongs in #11101's unit, not this one.
4. **Should `Record.Query` be deprecated in favour of the derived set?** Kept as the raw input here, on
   YAGNI. If nothing ever reads it once the derived set exists, it is a deletion candidate — but that is a
   later reading of a real inventory, not a guess now.
