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
takes labelled retrieval from 8/11 to 10/11 and admitted from 7/11 to ~~9/11~~ 8/11** *(retrieved simulated
2026-09-04 and confirmed by a real sweep 2026-09-05; the admitted figure was **predicted as 9/11 and is
struck** — the binary returns 8/11, and §6 item 3 says why. All of these are on the eleven-labelled-row
corpus that PR #27 has since replaced with a 23-labelled-row one: **§9.2 owns the current rates — §9.1 is the
reading it supersedes — and this line is not the place to read them.**)*

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
costs: labelled retrieved 0.73, three rows `notRetrieved` *(#11133's figure, a **dated record** on the
eleven-row corpus — the same arm reads 9/11 = 0.82 today and the third row is no longer a miss; §9.1 item 2
carries the re-measurement, and §2.1 the same note)*. #11134 diagnosed the cause as a vocabulary gap
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

*(Dated record, kept as measured. The baseline has since moved on its own: the same raw-input arm over the
same eleven rows read **9/11 labelled** on 2026-09-05, because the graph is live and unversioned. §9.1 item 2
carries the re-measurement; nothing in this section is retro-edited to match it.)*

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
  a full-graph search that looks like a working one.
- **There are two ways to fail open, not one, and the second is the right key.** Besides the wrong-key case
  above, `linkedto=` **with an empty value** also returns the full-graph ranking *(measured 2026-09-04, table
  in #11262; written down here at #11275 item 1's request)*. So a scope that computes to empty does not
  narrow nothing — it silently searches everything, which is the more dangerous of the two because the key is
  correct and only the value is wrong. Guard G-6 covers **both**: one `linkedto` per scope id, and **no
  `linkedto` key at all** for an empty scope.

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
      │                                        queries = {input} ∪ derived (input first)
      │
      ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │  loop.Retrieve(ctx, graph, anchor, queries, limit, reserve)          │
  │      MODEL-FREE BY CONSTRUCTION (takes []string, never a ModelPort). │
  │      One implementation, called by BOTH turn.go and cmd/eval.        │
  │      The caller passes the ANCHOR; the scope is built in here, so    │
  │      there is exactly one copy of scope construction in the tree.    │
  │                                                                      │
  │    for each q in queries:  Graph.Recall(q, CandidateLimit, nil)      │
  │    then:  scope = RecallScope(anchor.ID)                             │
  │             └─ Graph.Neighbours(anchor.ID); {anchor.ID} ∪ those,     │
  │                sorted and deduplicated                               │
  │    once:  Graph.Recall(queries[0], CandidateLimit, scope)            │
  │             └─ queries[0] is the raw input                           │
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
| `internal/loop/retrieve.go` **(new)** | `Retrieve(ctx, graph GraphPort, anchor Anchor, queries []string, limit, reserve int) ([]Candidate, error)`. Fan-out, the scoped recall, and fusion. **Takes no `ModelPort`** — the mechanical boundary of §3 expressed as a signature. The fusion itself is a separate **pure** function with no `ctx` and no port, on `Assemble`'s discipline. **`Retrieve` builds the anchor scope itself**, via `RecallScope(ctx, graph, anchor.ID)`: the caller passes the **anchor** and never a scope, so exactly one copy of scope construction exists in the tree — the caller-builds reading would put a second one in `cmd/eval/sweep.go`, which is **R5 realised and invisible to any single-package test**. `queries[0]` is the raw input and is what the scoped recall carries; the unscoped fan-out covers all of `queries`, `queries[0]` included |
| `internal/loop/turn.go` | `Run` calls `Queries.Derive`, then `Retrieve`. `Assemble`, `judge`, `dispatchRecall`, `logFinished` unchanged. New constants `RecallScopeReserve = 3` and `MaxDerivedQueries` beside the existing five |
| `internal/loop/types.go` | `QueriesPort` with one method, `Derive(ctx, input string) ([]string, error)`. `Record` gains the derived queries and the per-query provenance of each candidate — §4.4 |
| `internal/loop/turn.go` (`GraphPort`) | `Recall` gains a `scope []int64` parameter, `nil` meaning global — one method rather than a near-duplicate `RecallScoped`. New `Neighbours(ctx, id int64) ([]int64, error)` |
| `internal/divoid/client.go` | `Recall` emits `linkedto` when scope is non-empty; `Neighbours` reads `GET /api/nodes/links?ids=<id>` and returns the other endpoint of every incident edge, **sorted and deduplicated**. Sorting makes the scope order-stable. **Dedup is load-bearing, not tidiness:** a node linked to itself is returned by the API as its own neighbour, and repeated edges between one pair are possible, so without the collapse the same id is sent to `linkedto` twice. Guarded by `TestNeighboursReadsTheFarEndpointOfEveryEdgeInEitherDirectionCountingARepeatedPairOnce` and `TestRecallScopeCarriesTheSubjectOnceWhenASelfEdgeMakesItItsOwnNeighbour`. **It also follows the `continue` cursor** — the row says *every* incident edge, which one page satisfies only while no node exceeds the page size — and **warns on the operator log when it stops before the graph's last page**, so a truncated neighbourhood is readable rather than silent |
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
figure #11101 item 9 already flags as the one that will be forgotten *(and which PR #27 has since moved to
`ffa291d5` for unrelated reasons; the ruling below stands, its premise does not — §8's dated note)*. Putting derivations in the corpus would
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
retrieval quality on the raw input alone is whatever §9.1's raw-input arm currently reads — it was never
better than a bit over half the labelled rows; erroring the whole run instead would trade a degraded answer
for no answer, which is strictly worse. **`ErrModelUnavailable` is not returned from the derivation
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
| 3 | **The admitted rate's structural blindness to admission changes** | #11133's trap is intact and is **not** contradicted by this design moving the admitted rate. The rate is blind to *admission* changes on this corpus because only r09 is `cut` and it exceeds the budget either way; it is **not** blind to *retrieval* changes, because a required node newly entering the top 20 and fitting the budget flips its verdict. r03 and r08 do exactly that *(measured 2026-09-04)*. **And it cuts both ways, which this row originally did not say:** a retrieval change moves the admitted rate in **both directions** — a required node newly entering the top 20 flips its verdict upward, and a required node newly **outranked by larger admitted material** flips downward *without leaving the retrieved set*. **r02 does the second** — admitted on the raw-input arm, `cut at rank 6` on the pinned arm, still retrieved, because the widened candidate set consumes the byte budget ahead of it *(re-measured 2026-09-05 on `corpusHash ffa291d5`: r02 is not a miss on the raw-input arm and is `cut at rank 6, k'=6, 59,615/60,000 B admitted` on the pinned one — the same cut #11288 recorded on the eleven-row corpus, reproduced digit for digit; and its raw-arm counterpart is **re-measured here on the raw-input arm: 7 admitted / 56,031 B against a 60,000 B budget** — #11288's figure reproduced digit for digit too, which is the stronger reading, because it shows r02's raw-arm candidate set survived the corpus change unchanged)*. This is the shape §4.2 measured and rejected for the 50/50 interleave — *"it buys one row's survival at the exact margin with a general degradation"* — arriving on the **admitted** rate rather than the retrieved one, where §9's framing did not look for it |
| 4 | **The vocabulary gap in general** | Derivation narrows it and does not close it. Mode A is fixed for the two rows measured; the population of symptom-phrased questions is not eleven rows wide, and #11092 §5.3's blindness — tasks no node answers — is untouched |
| 5 | **Self-recall (#11179)** | Not fixed, and **made more exposed**, which is §7 R4. This design increases how much of the graph is reachable per turn, and run records are part of that graph |
| 6 | **Determinism of a live turn** | Lost, deliberately (§3). Retained for the sweep |

---

## 7. Risks

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| R1 | **A real model's derivations are worse than the hand-written ones this design was measured on.** The largest risk in the document | §9's A/B compares a live-derived arm against the pinned arm on one graph state; §4.4 records the derived queries verbatim so a bad derivation is readable rather than inferred | a live-derivation sweep **fails to beat a raw-input arm taken in the same session** ~~scores at or below 10/13~~ *(converted 2026-09-05: `/13` identified the corpus `corpusHash 6c1ba696` that PR #27 replaced. This is a forward-looking acceptance threshold rather than a dated record, so it is restated as arm-vs-arm on whatever the corpus then is — the same conversion §9 acceptance item 1 and §11 step 1 already carry, and for the same reason: the graph is live and unversioned, so a fixed denominator is a comparison against a system that no longer exists)* |
| R2 | **The design was selected on the same 13 rows it *was* scored on.** Six combiners were compared and the best kept — that is selection on the test set, and the honest reading of 12/13 is *an upper bound*. *(Tense corrected 2026-09-05: it is now scored on 25. The twelve rows PR #27 added are the only ones it was **not** selected on — ~~and the sidecar pins derivations for none of them, §12 q5, which is why they cannot yet answer the question they exist to answer.~~ **Struck later the same day: PR #32 pinned all twelve (§12 q5 closed), so they now *can* answer it. §9.2 is what they answer — and the answer is one row.**)* | Stated here rather than mitigated. The corpus was authored before this design existed and none of its rows were changed; #11092 §11 Q1's task-set question is the place this gets resolved | **Fired — and §9.1 already explains it. Do not read this row as an unfired falsifier.** The advantage did shrink on rows added after this document: **+0.09 on eleven rows** (0.82 → 0.91) became **+0.04 on twenty-three** (0.39 → 0.43). §9.1 reads that as **coverage dilution, not generalisation failure** — twelve of the twenty-three labelled rows are identical between the arms *by construction*, because the sidecar gives them no derivations, so they are incapable of showing an advantage either way. **That explanation is on the record and is itself testable**: §12 q5 option (a) or (b) would settle it by giving the twelve rows derivations. ~~Until one of them happens this firing is **uninformative rather than answered**.~~ ~~**Note also that R2a below was re-confirmed on 2026-09-05 and this row was not.**~~ *(Both struck 2026-09-05, later the same day: (b) happened — PR #32 closed the sidecar to 25/25 with blind, model-generated queries — and **§9.2 is the reading.** **Ruling: the dilution explanation is confirmed as the cause of the shrinkage and refuted as a complete account of it.** Confirmed, because closing coverage restores the full-corpus margin to **+0.09 on twenty-three** (0.39 → 0.48), which is the eleven-row figure to two decimals; the +0.04 was an artifact of rows incapable of differing. Refuted as complete, because dilution implied the twelve rows were merely **silent** — given queries, **eleven of the twelve still miss**, and the restored margin is wins and denominator growing at the same rate rather than evidence of generalisation: derivation converts **1 of 2** available misses on the eleven selected-on rows and **1 of 12** on the out-of-sample twelve. **This row is answered, not closed** — the charge it names has moved from unmeasurable to measurable-and-unsettled, and §9.2 carries the restated falsifier.)* |
| R2a | **R2 bites hardest on precisely the row carrying the derived half of the hypothesis, and that is not a coincidence.** The derived arm's advantage over the scope-only arm is **one corpus row, r03** *(re-confirmed 2026-09-05. `Retrieve` builds the scope and issues the scoped recall unconditionally, so a sweep with no `-derivations` **is** the scope-only arm — and r03 is the single row separating it from the pinned arm on retrieved, 9/11 → 10/11)*. **r03's first pinned query is §2.2's own measured best derived question, verbatim** — and §2.2 already flags its author as contaminated for r03, r07 and r08. So the one row that distinguishes derivation from scope alone is a row whose winning query was written by someone who had read the diagnosis first. **Step 4's case rests on it** | Not mitigable inside this corpus, and naming it is the mitigation. R1's live-derivation reading (§9 item 4) is the only thing that answers it: it asks whether a model, uncontaminated, produces a query as good as the hand-written one. Until that reading exists, treat the derived arm's margin over the scope-only arm as **one hand-written query's worth of evidence** | a live-derived sweep leaves r03 `notRetrieved`, collapsing the derived arm onto the scope-only arm. *(**Weakened in the design's favour 2026-09-05, later the same day** — and this is the best news in §9.2. The derived arm's advantage over the scope-only arm is **no longer one row**. At 25/25 sidecar coverage it is **two — r03 and r12** — and **r12's queries had no author in the sense this row means**: they were generated by a model that never saw the row's `required`, `subject`, `hash` or `why` fields, blindness enforced structurally rather than by care (PR #32; `project_corpus_blind()` discards those fields before any model-facing value is constructed). **So r03 could collapse entirely and the arms would still separate.** This falsifier is not retired — it is simply no longer sufficient on its own to collapse the arms, and the row's core charge is correspondingly narrowed: the derived half of the hypothesis no longer rests **only** on a contaminated query. One uncontaminated row is not a result, but it is the first evidence in this document that is not r03's.)* |
| R3 | **Cost per turn rises from 1 graph read to 1 + N + 2** (N derived queries, one scoped recall, one links read), plus one model call | All reads are `GET`s against one host; the model call is small — a derivation prompt carries the input, not the block. **The sweep's model cost stays zero** (§4.5) | a turn's wall time or graph error rate rises materially in the smoke rig |
| R4 | **More of the graph reachable means more self-produced content reachable** (#11179) | `admit` already cuts self-produced rows (#11158) and the sweep counts them. **The smoke rig must report the self-produced count among admitted rows**, which #11179 asks for anyway | a two-turn smoke run admits a `processor-run` record and nothing reports it |
| R5 | **The sweep and the turn drift apart** — the instrument stops measuring the product (#11142) | §4.3's extraction: one `Retrieve`, two callers, and G-5's cross-package mutation arm | a fusion constant is mutated and only one of `internal/loop` / `cmd/eval` reddens |
| R6 | **A silently-ignored scope parameter yields a full-graph search that looks scoped** — measured: `rootnode`, `root_node_id`, **and the right key with an empty value** are all accepted and ignored (§2.3) | G-6 | **the request** a scoped recall issues carries no `linkedto` key, or carries one the caller did not ask for, and G-6 stays green. *Deliberately not "a scoped recall returns a candidate outside the scope set and nothing fails": that is a real **production** symptom, and it is also the verbatim assertion §10 G-6 struck on 2026-09-05 as structurally incapable of discriminating. The filtering is the graph's, not this code's, so inside a fixture a fake that honours `linkedto` passes whether the client sent `linkedto`, `rootnode`, or nothing at all. The symptom is what you would see in production; **the request is the only thing observable from inside this process**, so it is what the falsifier has to name.* |
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

> **Overtaken 2026-09-05, and worth recording because the reasoning was sound and the premise still expired.**
> The half of this check about *this design* holds: it changes no corpus row, and the sidecar remains a
> separate file with its own `derivationHash`. But **`6c1ba696` no longer identifies anything a sweep prints.**
> PR #27 took `internal/eval/corpus.json` from thirteen rows to twenty-five, moving `corpusHash` to
> **`ffa291d5`** for reasons unrelated to this design. The care §4.5 took to keep the corpus hash still was
> correct and was spent on a hash that a different change moved anyway. **The lesson is not that the ruling was
> wrong** — a design cannot hold a shared file still — **but that `corpusHash` was never a stable identifier
> for a baseline, only a detector of change to one file**, and #11133's baseline is pinned by a graph state
> that carries no hash at all (§9.1's third identity). ~~Sidecar coverage did not follow the corpus: it still
> pins 13 of the 25 rows, which §9.1 measures the consequence of.~~
>
> **Struck 2026-09-05, later the same day.** PR #32 closed the sidecar to **25 of 25**, and **§9.2 measures
> the consequence of closing it**. The sentence was true when written and is struck rather than deleted
> because §9.1's numbers are still read under it — but the gap it describes no longer exists, and the note
> above now has *two* expired premises rather than one, which is itself the point it was written to make.

---

## 9. Verification

**Which rate moves, and why it can.** The **retrieved** rate. It was at 0.73 labelled when #11134 was filed
against it *(see §9.2 for what it reads now, and §9.1 for the reading it supersedes — the corpus, the sidecar
and the graph have all moved since)*, and a
required node entering the top 20 flips its verdict — which is exactly what
r03 and r08 do. The **admitted** rate also moves here, and #11133's trap is *not* violated by saying so: that
trap is about admission changes on a corpus with one `cut` row, and this is a retrieval change (§6 item 3).

**Predicted, from the simulation** *(measured 2026-09-04, against the live graph with the shipped `admit`
rule and pinned hand-written derivations, on the **eleven-labelled-row** corpus of that date —
`corpusHash 6c1ba696`)*:

| | labelled retrieved | labelled admitted | control |
|---|---|---|---|
| baseline (#11133) | 8/11 = 0.73 | 7/11 = 0.64 | 2/2 |
| this arm, simulated | **10/11 = 0.91** | ~~9/11 = 0.82~~ → **8/11 = 0.73** | 2/2 |

**The admitted cell was a wrong prediction and is corrected in place, not retro-edited** *(P-43; struck
2026-09-05, first measured in #11288)*. The **retrieved** prediction reproduced digit for digit against a real
sweep. The **admitted** one did not, and the whole gap is **r02** — §6 item 3 states the mechanism and why it
was not looked for.

**These are a prediction from a simulation, not a sweep result.** The simulation reproduces `admit`'s rule in
Python against live content; it is not the binary. **A number this document predicts and a number a sweep
returns are not the same kind of thing** — which is #11133's own 0.64/0.64 lesson, and the reason the
acceptance criterion below is a re-run and not a match against this table. The corrected cell is itself the
worked example: the prediction was arithmetic over a simulated `admit`, and the binary disagreed.

### 9.1 Re-measured against the current corpus *(2026-09-05)*

**The table above has a denominator no sweep prints any more.** PR #27 took `internal/eval/corpus.json` from
thirteen rows to **25 — 23 labelled, 2 control** — so `corpusHash` is now **`ffa291d5`** and the eleven-row
figures above are a dated record rather than a live expectation.

Two sweeps, same tree, same graph state, minutes apart, **both free of model calls** (§4.5; the `eval` binary
constructs no model client at all — it reads only the graph half of the boot configuration):

| arm | labelled retrieved | labelled admitted | control |
|---|---|---|---|
| raw-input (no `-derivations`) | 9/23 = 0.39 | 7/23 = 0.30 | 2/2 = 1.00 |
| pinned — `internal/eval/derivations.json` at **13/25 coverage**, `derivationHash 23b50552` | **10/23 = 0.43** | **8/23 = 0.35** | 2/2 = 1.00 |

> **The paragraph below describes the sidecar as it stood when the table above was taken — 13 of 25.** PR #32
> has since closed it to 25/25, so the "by construction" claim is true **of that sidecar** and not of the one
> a sweep loads today. It is left standing because the table above cannot be read without it. **§9.2 is the
> measurement of the closed state.** *(2026-09-05, later the same day.)*

**Read those full-corpus rates as a measurement of this design and you will be wrong, and the reason is a
coverage gap rather than a result.** The sidecar pins **13 of the 25 rows** — r01–r11 plus both controls —
because it was authored against the eleven-row corpus and PR #27 did not extend it. `LoadDerivations`
validates that every sidecar row resolves to a corpus row, but **does not require that every corpus row
carries a derivation**, so **r12–r23 sweep on their raw input alone on *both* arms**. Twelve of the
twenty-three labelled rows are identical between the arms by construction, and all twelve are misses. The
full-corpus rates are this design applied to eleven rows and diluted by twelve it was never given queries for.

Restricted to the eleven labelled rows where the two arms **can** differ — they receive different query sets
on all eleven, and differ in *verdict* on three of them (r02, r03, r08):

| arm | labelled retrieved | labelled admitted |
|---|---|---|
| raw-input | 9/11 = 0.82 | 7/11 = 0.64 |
| pinned | **10/11 = 0.91** | **8/11 = 0.73** |

Row by row, the whole of the difference: **r03** `notRetrieved` → admitted, **r08** `cut` → admitted, **r02**
admitted → `cut at rank 6`. Net **+1 retrieved, +1 admitted**.

**Two things in that subset table deserve to be read carefully.**

1. **The pinned arm reproduces #11288's step-3 reading exactly** — 10/11 = 0.91 retrieved, 8/11 = 0.73
   admitted. So `0.73` was never stale in the sense of being wrong; what went stale is the **denominator**.
   Both halves of the prediction have now been checked against the binary twice, a day apart.
2. **The baseline moved, and this document's own acceptance step is what catches it.** #11133's raw-input arm
   scored **8/11 = 0.73** retrieved; the same arm on the same eleven rows today scores **9/11 = 0.82**,
   because r08's required node **#10839** is now retrieved (`cut at rank 18`) where it was `notRetrieved`
   before. Nothing in this design caused that — **the graph is live, unversioned, and moved under the
   baseline between 2026-09-04 and 2026-09-05.**

**What would invalidate every number in this section.** Each hangs on three identities, and a rate quoted
without all three names nothing: `corpusHash ffa291d5` (the corpus file), `derivationHash 23b50552` (the
sidecar, at 13-of-25 coverage), and **the graph's own content**, which carries no hash and is the one that
moved. Item 2 above is the proof that the third is not hypothetical — which is why the acceptance step below
compares two arms **taken in one session** and never a fresh arm against a rate recorded on another day.

**The second identity has since expired too** *(2026-09-05, later the same day)*. PR #32 replaced the sidecar
with a 25-of-25 one, so **`23b50552` now identifies a file that is not in the tree**. Every number in §9.1
remains a valid reading **of that file**; none of them is a reading of the sidecar a sweep loads today.
**That is two of the three identities expiring inside two days** — and it is why §9.2 states its ruling in
terms of arm-vs-arm differences per population rather than in rates, since a difference between two arms taken
in one session survives all three moving and a rate survives none of them.

### 9.2 Coverage closed — the addendum §9.1 asked for *(2026-09-05, later the same day)*

**PR #32 closed the sidecar to 25 of 25**, and it did so by §12 q5's option **(b)**: r12–r23 were generated by
a local model that never saw its rows' `required`, `subject`, `hash` or `why` fields. The blindness is
**structural rather than a matter of care** — `project_corpus_blind()` discards those fields before any
model-facing value is constructed — which is the only form of blindness that survives an author in a hurry.
**r01–r11 and both controls are byte-identical** to the sidecar §9.1 measured (120 insertions, 0 deletions),
so the two readings differ in exactly the twelve rows that were added, and nothing else. Record: **#11348**.

**Measured twice, by two parties** — once after generation, once by QA re-running both arms from a fresh
build — **identical both times**, on **one graph state** and with **zero model calls**. *(An earlier draft
called this "measured twice independently". It was not: same binary, same corpus, same sidecar, same graph
state. What the second run rules out is a transcription error or a one-off fluke in the first — nothing about
the instrument, which both runs share entirely.)* `corpusHash ffa291d5`, limits
`candidateLimit=20 assemblyByteBudget=60000 recallScopeReserve=3`:

| arm | labelled retrieved | labelled admitted | control |
|---|---|---|---|
| raw-input (no `-derivations`) | 9/23 = 0.39 | 7/23 = 0.30 | 2/2 = 1.00 |
| pinned at **13/25** — the state §9.1 records, `derivationHash 23b50552` | 10/23 = 0.43 | 8/23 = 0.35 | 2/2 = 1.00 |
| **pinned at 25/25** — `derivationHash` **`38349b3c`** | **11/23 = 0.48** | **9/23 = 0.39** | 2/2 = 1.00 |

> **The 25/25 sidecar's hash was written after the line endings were fixed, not before.** The generator
> originally wrote CRLF, so the hash the first sweeps printed was a hash of **the working tree** rather than
> of what a fresh checkout produces — `38349b3c` is the LF hash, and it is what the loader reports on a
> normalised file. It was measured through `LoadDerivations` on the corrected file rather than transcribed
> from a report, because under **P-51** a hash written into this document is a hash this document asserts.
> No rate above was ever affected: the sweep loads the file it loads, whatever a hash says about it — only
> the identity a later reader would use to decide whether their re-run is comparable.

**Per-row, raw-input → 25/25: exactly four verdict changes and no others.**

| row | raw-input | 25/25 | among the twelve added? |
|---|---|---|---|
| r02 | admitted | **`cut` at rank 6**, 59,615 / 60,000 B | no |
| r03 | `notRetrieved` | **admitted** | no |
| r08 | `cut` | **admitted** | no |
| r12 | `notRetrieved` | **admitted** | **yes — and the only one** |

r02, r03 and r08 lie in r01–r11, which PR #32 left byte-identical, so they are the same three moves §9.1
already recorded. **Of the twelve rows the PR adds, exactly one moved. Eleven of twelve changed nothing.**

#### The ruling on §7 R2's falsifier: confirmed as a cause, refuted as a complete account

**Confirmed.** §9.1 read the shrinkage from **+0.09 on eleven rows** to **+0.04 on twenty-three** as coverage
dilution rather than generalisation failure, and — correctly — flagged that reading as *testable rather than
established*. The test has now run and it agrees. Closing coverage moves the full-corpus margin to **+0.09 on
twenty-three** (0.39 → 0.48), the eleven-row figure to two decimals. The +0.04 was an artifact of twelve rows
that were incapable of differing. **R2's "uninformative rather than answered" clause is discharged.**

**Refuted as a complete account, and this is the half that matters.** Dilution carried an implicit promise:
that the twelve rows were *silent*, not *negative* — that once given queries they would behave like the
eleven. They do not. The margin is restored only because the wins and the denominator grew at the same rate,
which is arithmetic coincidence and not mechanism. Decompose by population and the halves are nothing alike:

| population | raw-input | pinned 25/25 | margin | raw misses converted |
|---|---|---|---|---|
| **r01–r11** — hand-authored; the rows the design was selected on | 9/11 = 0.82 | 10/11 = 0.91 | +0.09 | **1 of 2** |
| **r12–r23** — blind-generated; the only rows it was **not** selected on | **0/12 = 0.00** | 1/12 = 0.08 | +0.08 | **1 of 12** |

*(The 0/12 is not a separate reading — it is forced by §9.1's own two tables, whose raw arm scores 9/23 in
total and 9/11 on the eleven. It is recorded here because a figure that falls out of arithmetic is still a
figure the next reader should not have to re-derive.)*

**The margins are near-identical and the conversion rates differ by a factor of six.** So a full-corpus
*margin* is no more a measurement of this design than a full-corpus *rate* was — §9.1's own lesson arriving
one level up, and the reason this section rules on differences **per population** rather than on any rate.

**And the six-fold gap cannot be attributed — not to authorship, which is the only axis anyone wants to read
it on.** Begin with the part that is an observation rather than an inference: **the raw-input arm uses no
derivations at all**, so its **0.82 on the eleven against 0.00 on the twelve** is a statement about the
*rows*. No query written by anybody enters it. The twelve are simply harder for this retriever, measured
directly, before derivation is in the picture at all — which is a stronger footing for the confound than
calling it an inference, and an earlier draft undersold it.

On top of that, *hand-written versus model-written* is confounded with **three** further axes, and all of them
move together because they are the same event seen from different sides:

- **Difficulty.** 0.82 versus 0.00 on the raw arm — a gap far larger than the effect being measured.
- **Vintage.** r12–r23 all arrived together in **PR #27**, authored later than r01–r11 and against a different
  graph state. The nodes they require are newer as a block (**#11049–#11278** against **#10440–#10982**), so
  *harder* and *written later* are one fact counted twice. **This one is not obviously harmless**: a newer
  node has had less time to accumulate the links and neighbours a graph search reaches it through, so vintage
  plausibly *causes* part of the difficulty rather than merely accompanying it.
- **Input shape.** The twelve inputs are **60% longer** than the eleven — 15.0 against 9.4 content tokens —
  and are scenario-shaped rather than single-question. This is the axis the lexical-overlap measurement below
  runs aground on, and it was invisible until that measurement was actually taken.

**One axis a sceptical reader reaches for first does *not* explain any of it: both populations are uniformly
`stratum: labelled`.** All 23 labelled rows carry that same value in `internal/eval/corpus.json` (read from
the tree 2026-09-05); the only two `control` rows are c01/c02, which belong to neither population. Stratum is
not the difference, and saying so costs one sentence and forecloses one wrong reading.

**Nothing here licenses the reading that blind derivations are worse than contaminated ones, and nothing here
rules it out. What has changed is that R2's charge is measurable at all**, and the first measurement of it is
one row.

**One row is real here and unreplicable anywhere else, and those are different statements.** The
within-session protocol is what makes r12 a genuine difference: both arms were taken minutes apart on one
graph state, twice, by two parties, and **§9.1 item 2's ±1 row is a *cross-day* drift figure, which does not
govern a within-session paired comparison at all.** An earlier draft justified the threshold with that
statistic anyway, which pointed a sound argument at the wrong target. **The sharp version concedes nothing
about this reading and everything about what can be built on it:** §9.1 item 2 records the **baseline itself**
moving by one row overnight because the graph is live and unversioned — and **every replication of this
finding will be cross-day**, where ±1 row is exactly the noise floor. So the design's entire demonstrated
out-of-sample effect is **the size of the smallest thing a second party on a second day could tell apart from
nothing**. A one-row effect on a live unversioned graph is **unreplicable by construction**. Read r12 as an
existence proof, never as a rate.

**Restated falsifier for R2, replacing the one that has fired:** *derivation fails to beat a raw-input arm on
rows outside the selection set by **more than one row**, on a corpus large enough that one row is not the
whole margin.* This is not answerable at twelve out-of-sample rows and does not become answerable by
re-measuring them; it needs the next corpus growth (#11101), and that is the honest cost of the finding.

#### Why eleven of the twelve still miss — one named cause, one retracted, and seven rows unaccounted for

**Ranking depth is the one cause this section can name.** On **r13, r14, r16 and r17** the required node **is in the candidate pool**,
scoring **0.73–0.80**, and is edged out of the top 20 by near-identical neighbours. **r16 is the clean case,
and the recorded figures support a narrower claim than an earlier draft made of them: the required node sits
at 0.8041 and is displaced by the row's own subject, recorded as 0.80.**

> **Corrected 2026-09-05, later the same day.** The draft read this as the required node losing its slot to
> something scoring **below** it, and made that inversion carry the paragraph. It cannot: **`0.80` covers
> `[0.795, 0.805)`, which straddles `0.8041`**, so a two-decimal figure compared against a four-decimal one
> does not establish an ordering either way. **Nor is the four-decimal figure recoverable.** Only a
> same-session re-run of both arms would produce it, and the graph has moved since — which is §9.1 item 2's
> own lesson arriving on this document's own evidence. **If the inversion matters, it is one same-session
> sweep away**: re-run and record the displacing candidate's score at the precision the required node's is
> recorded at. Until then this row claims a tie, not an inversion.

**What the recorded figures do establish is the point the paragraph needs, and arguably needs more sharply:**
the required node and the node that takes its slot are **within the resolution of the recorded score** — the
cap is imposing an order between two candidates whose scores do not separate them. That is not losing on
relevance; it is losing to the fusion's rank arithmetic and the cap, which is **G-1's premise working exactly
as designed and showing its cost for the first time**. **No rewriting of the question reaches these rows.**
They are a `candidateLimit` and ranking-resolution problem, adjacent to §2.4's mode C and to F1.

**They are four of the twelve — a third, not "the majority", which is what an earlier draft of this sentence
claimed.** With the second cause below withdrawn, the arithmetic is worth stating plainly: of the twelve rows
PR #32 added, **one moved (r12), four miss with a named cause (r13, r14, r16, r17), and seven miss with no
cause this document has identified.** That is not a gap worth papering over with a second explanation — it is
the honest state of the reading, and it is where a future triage pass should start.

**A second cause was asserted here, was never measured, and does not survive being measured.**

> **Retracted 2026-09-05, later the same day — and the retraction is the finding.** An earlier draft of this
> paragraph stated that QA had **measured** the twelve new rows' derived queries as carrying *higher lexical
> overlap with their own inputs* than the pinned eleven do. **No metric was ever attached to that sentence**
> — and three paragraphs further down, that same draft proposed the very same measurement as one that had
> **not** been taken, complete with a falsifiable prediction. Both could not be true, and the internal
> contradiction was the visible symptom of the real fault. Under **P-51** a measurement this document transcribes is a measurement
> this document asserts, so the sentence is its own and it is struck. What replaces it is the arithmetic.

**Measured** *(2026-09-05, later the same day; arithmetic only — no graph, no model, no sweep, so nothing
below depends on a graph state)*. Tokens are lower-cased `[a-z0-9]+` runs compared as sets; `I` is a corpus
row's `input` and `Q` a single derived query; each metric is averaged over a row's five queries and then over
the population. Four metrics: **input recall** (shared tokens as a fraction of the input's), **Jaccard**
(shared over union), **union coverage** (the five queries' pooled tokens as a fraction of the input's), and
**query-side reuse** (shared tokens as a fraction of *the query's*). Each was run under four tokenizer
variants — stopwording on/off crossed with naive plural stripping on/off.

| metric | normalised by | hand r01–r11 | model r12–r23 | direction |
|---|---|---|---|---|
| input recall | the input | 0.163 | 0.145 | model **lower** |
| Jaccard | the union | 0.118 | 0.115 | model **lower** |
| union coverage | the input | 0.440 | 0.330 | model **lower** |
| query-side reuse | **the query** | 0.263 | **0.347** | model **higher** |

*(Stopworded, unstemmed variant shown. All four variants agree on all four directions with a single
exception: Jaccard reverses in the stopworded-and-stemmed variant, and by 0.004 (0.134 against 0.138). The
stopword list is an ordinary English function-word set; **the decimals depend on it and the directions do
not**, which is the whole reason the unstopworded variants were run at all. **Provenance, because it is not the same for both halves:** the eleven hand rows were
read from `internal/eval/derivations.json` in the tree; the twelve model rows were read from the generation
record **#11348**, because the 25/25 sidecar lives on PR #32's branch and this correction was written against
`main`. #11348 is the file's own source and not the file — if the two ever disagree, this table is wrong and
so is the record.)*

**Three of the four say the model rows recycle their inputs *less*; one says more; and neither direction
survives its confound.** The three pointing one way all divide by the size of the **input**. The one pointing
the other divides by the size of the **query**. And — as the confound list above now records — **the twelve
later inputs are 60% longer than the eleven**, which moves every one of these metrics on its own: across all
23 rows and on this same stopworded tokenization, input length correlates **−0.23** with input recall and
**+0.52** with query-side reuse. Restrict to
the length band where the two populations actually overlap and the gaps collapse or invert depending on where
the band is cut — at `|I|` in [9,16] query-side reuse reads **0.328 against 0.328**; at [10,15] it reads
**0.369 against 0.328**, the model now *lower*. **At eleven rows against twelve, with inputs that differ
systematically in length, these two populations are not separable on lexical overlap by this family of
measurements.** The original sentence was not merely unmeasured — it named a direction this instrument cannot
resolve, and so does its reverse.

**What *is* robust sits at row level, and it is one row rather than three.** Across all eight
metric-by-variant combinations, **r13's derived set is the first or second most input-recycling of the
twelve** — that holds however the tokens are counted, and r13 missed. **r14 does not hold**: it ranks
anywhere from 1st to 11th of twelve depending on the variant, so *"r13 and r14 recycle the input's nouns"*
was half right, and the surviving half is r13's. **And r15 is not a lexical observation at all** — it ranks
4th to 12th of the twelve across the eight combinations, never among the three most-recycling on any of them,
and on unstopworded query-side reuse it is the lowest of the twelve. What r15 does is inherit its input's
**framing**:
all five of its queries ask about ranking, which is precisely the false premise its corpus row exists to catch
(`why`: *"An answer lacking this would retune the ranking, when the truth is that an item larger than the
whole budget can never be admitted at any rank"*). **Filing r15 under lexical overlap concealed a second and
genuinely different failure mode** — a derivation can introduce entirely fresh vocabulary and still inherit
the question's wrong frame — and that mode is worth more than the claim it was bundled into, because no
overlap constraint in a prompt would catch it.

**So *"on at least three of twelve rows the model did not honour its own brief"* is withdrawn**, and what
stands in its place is narrower and better attested: **one row recycles measurably (r13), one row inherits its
input's frame (r15), and the population as a whole differs from the hand-written eleven in no way this
measurement can detect.**

**Which leaves the one win without the mechanism this section gave it.** r12's derived set does introduce
terms its input lacks (*filter contract*, *boundary enforcement*, *scope enforcement*) — that reading of the
queries is correct and is not in dispute. The claim that failed is the **comparative** one, and it was the
load-bearing half: **the hypothesis** was *a derived query earns its keep exactly when it introduces
vocabulary the input lacks, and a derivation that merely rewords is indistinguishable from the raw-input arm
by construction.* It would have explained the hand-written eleven doing well for the same reason under another
name — a contaminated author bridges the vocabulary gap *because* they have read the target — which would
have made **R2a's contamination and derivation quality one axis rather than two concerns.**

**Its prediction has fired against it.** The prediction on record was *r12's overlap is the lowest of the
twelve, r13's and r14's among the highest.* Measured: **r12 ranks between 3rd and 10th of twelve across the
eight variants and is never the lowest** — on stopworded, unstemmed input recall it is the **third most**
recycling row of the twelve. Only r13's half survives. **The half that failed is the half carrying the
mechanism**: the one row that moved is not distinguished from the eleven that did not by the property the
hypothesis names.

**Withdrawn as a reading of this data.** With r12 unremarkable, the hypothesis has no positive instance left.
It retains one negative instance — r13 recycles most and missed — and with eleven of twelve missing, any
property of any missing row is consistent with anything. **n=1 has become n=0.**

**What it always was, and this is the correction worth more than the retraction.** Vocabulary bridging is
**the generation prompt's own stated rationale**, on the record in **#11348** before any sweep ran: *"A
derived query earns its keep only if it bridges that vocabulary gap — restating the input in different words
does not, since the raw-input arm (already the control) covers that."* Offering it here as a mechanism *the
result suggested* was **reading the prompt's prior back out of the data that prompt generated** — a closed
loop wearing the clothes of a finding. It stands where it stood before: a design assumption behind the
prompt, unmeasured, and now known not to be visible in the single outcome that was supposed to reveal it.
**Any future attempt to revive it must name its metric before it measures**, because this section is the
demonstration that the metric family does not agree with itself, and a metric chosen after the results are in
is a result chosen after the results are in — the same fault as regenerating a row because it was seen to
miss, applied to the instrument instead of the sample.

**Its regeneration half is untouched by all of this, and the trap still stands.** Regenerating r13, r14 and
r15 *because they were seen to miss* is selection on the test set, and it would destroy the only
uncontaminated rows the corpus has — the precise failure (a) was rejected for. **The admissible form is to
change the prompt, regenerate all twelve blind, and re-measure.** Blindness is a property of the procedure and
not of the intention; it survives a prompt change only if no row is exempted. **What has changed is the
warrant, and whatever files that work must say so:** an overlap constraint in the prompt is now a change made
on a **prior**, not on a finding — and on the evidence above, the constraint most worth adding may not be
about overlap at all but about r15's frame inheritance, which no token-level constraint reaches.

#### What this discharges elsewhere in the document

- **§7 R2a is materially weakened, in the design's favour** — see that row. The derived arm's advantage over
  the scope-only arm is now two rows, and one of them has no author.
- **§9 acceptance item 4 is partly discharged, cheaply and half by accident.** Item 4 asks whether a real
  model's derivations score like the hand-written ones. Twelve sidecar rows now *are* that — an
  uncontaminated model's output, pinned and swept — on a disjoint row set. It is **not** item 4, which wants
  one model's derivations across the whole corpus on a live turn, and the confounding above is exactly why
  this cannot substitute for it. But **R1, the largest risk in this document, has its first real reading for
  the price of one local generation run, and the reading is not a firing**: the model-derived rows beat their
  raw arm, by one row.
- **§6's cost side has its first measured instance, with the byte figure §9.1 lacked.** **r02 went admitted →
  `cut` at rank 6 with 59,615 of 60,000 bytes consumed.** Derivation did not fail on r02 — it succeeded on the
  rows *above* r02's required node and spent the budget on them. **The binding constraint on admission is the
  assembly byte budget, not the candidate cap.** That is why raising `candidateLimit` to rescue
  r13/r14/r16/r17 would move the **retrieved** rate close to definitionally — that rate *is* "entered the top
  N" — while leaving **admitted** to the budget. **Any unit proposing a deeper cap must state which of the two
  rates it expects to move and why**, or it is buying a number rather than measuring one.

### Acceptance

1. **A sweep on the raw-input arm** (no `-derivations`), taken **in the same session** as every reading it
   will be compared against. ~~returns **0.73 / 0.64 / control 1.00** with `corpusHash 6c1ba696`. If it does
   not, the graph moved and nothing below is comparable — stop and re-baseline.~~ *(Struck 2026-09-05: both
   halves are dead. `6c1ba696` identified the thirteen-row corpus PR #27 replaced with `ffa291d5`, and 0.73
   no longer reproduces even on the eleven surviving rows, which now read 0.82 — §9.1 item 2.)* **The step's
   purpose survives its numbers, and is now the whole of it:** the raw-input arm is a **control taken
   alongside**, not a constant to match. Compare the two arms against each other; never against a rate
   recorded on another day, because the graph is live and unversioned and a cross-day comparison is two
   different systems. A raw-input arm that disagrees with §9.1 is evidence the graph moved — which is
   information, not a stop.
2. **A sweep on the pinned arm**, same session, minutes apart, returns the retrieved rate. Both readings carry
   `corpusHash`, `derivationHash` and the arm's name. **Both are free of model calls**, so this is the whole
   A/B for the retrieval question and it costs nothing.
3. **`admittedBytes` and `admittedCount` are carried on both readings**, per #11133's explicit instruction —
   a rate match across a change that moves context is the trap that node exists to prevent. **They are carried
   for every row whatever its verdict**, because they are plain fields on the row struct and are marshalled
   unconditionally (`internal/eval/result.go:25-26`, read from the tree 2026-09-05). Only the **human**
   report's byte diagnostic is miss-only — `writeMisses` skips a row whose verdict is `Admitted`
   (`internal/eval/report.go:123-127`), so the figure for a non-miss row is read off the machine stream, not
   the printed report. Do not mistake the human report's silence for the number being unavailable: an earlier
   draft of §6 item 3 did exactly that and declined to re-measure a figure the JSON was already carrying.
4. **A live-derivation reading** on the same graph state, to answer R1: do a real model's derivations score
   like the pinned ones? This one costs model calls, one per row, and is the only part that does.
   *(**Partly discharged 2026-09-05, later the same day** — §9.2's closing subsection. Twelve of the sidecar's
   rows are now an uncontaminated model's output, pinned and swept. That is **not** this item, which wants one
   model across the whole corpus on a live turn, and §9.2 states the confounding that stops the two being
   substituted for one another. The item stays open; what has changed is that it is no longer the *only*
   evidence bearing on R1.)*
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
| **G-6** | A scoped recall is actually scoped | Assert **the request carries `linkedto` once per scope id**, and carries **no `linkedto` key at all** for an empty scope. Premise: the graph ignores an unknown scope key **and an empty scope value**, returning the full-graph ranking either way *(measured 2026-09-04: `rootnode=` and `root_node_id=` return the full-graph ranking, and so does `linkedto=` with an empty value — table in #11262)* — so there are **two** ways to fail open on the right key, and the only thing distinguishable from inside this process is **the request**. ~~Assert every returned candidate is inside the scope set, against a fixture whose unscoped result contains an out-of-scope node ranked first.~~ **Struck 2026-09-05:** the filtering is the graph's, not this code's, so against an `httptest` server that assertion tests the fixture's obedience rather than the client's key — a fake that honours `linkedto` passes whether the client sends `linkedto`, `rootnode`, or nothing. It was structurally incapable of failing for the reason this row names. Shipped as `TestRecallSendsOneLinkedToParameterPerScopeIDBecauseAnUnknownScopeKeyIsSilentlyIgnored` and `TestRecallSendsNoScopeKeyAtAllForAnEmptyScopeSoTheRankingStaysWholeGraph`, measured red on `linkedto`→`rootnode`, on `Add`→`Set` (only the last id survives), on dropping the loop, and on emitting an empty `linkedto` |
| **G-7** | The scope is two hops, not one | The fixture's required node must be a neighbour **of a neighbour** of the anchor and **not** a direct neighbour. Premise: measured, one-hop scoping scores 10/13 and two-hop 13/13 on the scoped arm; a fixture where the required node is directly linked cannot tell them apart |
| **G-8** | `Neighbours` is order-stable **and repeat-free** | Assert the returned slice is sorted against a fixture whose edge rows arrive out of order, **and that a pair joined by more than one edge appears once**. Premise: the scope feeds a `linkedto` list, and an unstable scope makes the sweep non-reproducible for a reason no rate would ever surface; a repeated id sends the same node to `linkedto` twice, which a sorted-only assertion cannot see. **Shipped as three fixtures, one per half plus the self-edge:** `TestNeighboursReadsTheFarEndpointOfEveryEdgeInEitherDirectionCountingARepeatedPairOnce` (the repeat-free half), `TestNeighboursReturnsTheScopeAscendingSoOneEdgeSetAlwaysYieldsOneScope` (the order-stable half — this is what discharges "sorted against an out-of-order fixture"), and `TestRecallScopeCarriesTheSubjectOnceWhenASelfEdgeMakesItItsOwnNeighbour`. **The self-edge earns its own fixture**: the API returns a node linked to itself as its own neighbour, so the subject reaches the scope twice by a route no repeated-pair fixture exercises |
| **G-9** | The derived queries and the fallback flag reach the record | Assert a **distinctive** derived string survives into `Record`, and that the flag is `true` on the fallback path and `false` otherwise. Premise (P-17): a `false`/`""` expectation is what a dropped field decodes to |

**Not guarded, deliberately:** derivation *quality*. No test can assert that a model's questions are good ones;
that is what §9 item 4 measures and what R1 carries.

---

## 11. Implementation order

| # | Step | Why this order |
|---|---|---|
| 1 | **Extract `loop.Retrieve` and have `cmd/eval/sweep.go` call it**, with today's single-query behaviour and no new capability. Sweep re-run must return **the same rates as a raw-input sweep taken immediately before the extraction** — ~~**0.73 / 0.64 / 1.00** unchanged~~ *(struck 2026-09-05: that pair identified a corpus and a graph state that no longer exist; §9.1)* | A pure refactor with a **zero-delta acceptance criterion** — the only step whose correctness is checkable against an existing number. **The number must be taken, not quoted:** the delta is zero against a reading from the same session, and against nothing else. Doing this step after the behaviour change would leave nothing to check it against |
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
5. **Who writes derivations for r12–r23, and does anyone?** *(Raised 2026-09-05, from §9.1.)* The sidecar
   pins 13 of the corpus's 25 rows; PR #27 added twelve labelled rows and no derivations for them, and
   `LoadDerivations` permits the shortfall silently — it validates that every *sidecar* row exists in the
   corpus, never that every *corpus* row is pinned. **Every full-corpus rate this sweep prints is therefore
   a blend of a derived arm over eleven rows and a raw arm over twelve**, and it will keep drifting downward
   as the corpus grows. Three options, and the choice is Toni's because two of them cost something real:
   **(a)** hand-author the twelve — cheap, and it **reproduces R2a's contamination twelve more times**, since
   the author would again be writing queries with the diagnosis in hand; **(b)** generate them with the live
   model once and pin the output — uncontaminated and it makes the sidecar a *record of a model run* rather
   than a hand-tuned ideal, which is arguably what it should have been from the start; **(c)** make the
   shortfall loud — have `LoadDerivations` report coverage, and have the sweep print `13/25 rows derived`
   beside `derivationHash` so no reading is ever mistaken for a full-corpus one. **(c) is not exclusive with
   either and should happen regardless** — it is the only one of the three that costs nothing and it is what
   turns a silent dilution into a visible one. **My recommendation is (c) now, (b) next**; (a) buys a
   headline number at the cost of the evidence it is supposed to be.

   **Closed 2026-09-05, later the same day.** The recommendation was followed in order: **(c) shipped as PR
   #30** and **(b) as PR #32**. The sidecar is 25/25 and every option in this list is now either done or ruled
   out. **What (b) cost:** one generation run — twelve rows at roughly 0.4 s each, 5/5 valid distinct queries
   on the first attempt for every row, 120 insertions and 0 deletions, plus a re-runnable generator kept in
   the tree. *(The preferred 32B model would not load — a CUDA KV-cache allocation failed outright — and the
   run fell back to a 30B coder. Worth recording, because the result below is that model's and not the
   preferred one's.)* **What it bought:** **one row.** r12 moved `notRetrieved` → admitted; eleven of twelve
   changed nothing (§9.2). **So the headline answer to "was it worth it" is *barely* — and the headline is the
   wrong reading.** The row is not what (b) purchased. It purchased **twelve rows capable of falsifying
   something**, and under (a) the corpus would now hold twenty-three contaminated rows and none such. That is
   the entire return, and it appears in no rate.

   **What this implies for the framing if the corpus grows again.** **(b) is the default and the question is
   settled** — the generator exists, the run is minutes, and its blindness is structural rather than a matter
   of the author's care, which is the only form that survives a deadline. **(a) is not merely disfavoured but
   positively ruled out by what (b) measured:** hand-authoring **would be expected to** convert more of the
   twelve — an author who has read the required node can write a query that reaches it — and every conversion
   so bought would have been uninterpretable. *(**Expected**, not established, and the difference is the one
   §9.2 was corrected for: nobody hand-authored the twelve, so that comparison has never been run. The
   mechanism is plausible and r03 is one instance of it; an earlier draft wrote "very nearly by definition",
   which is a plausible mechanism asserted as near-certainty.)* **(c) is confirmed load-bearing and needs
   extending rather than repeating.** The coverage counter did its job; the next silent dilution will not be a
   coverage gap but the thing §9.1 walked into one level up — a full-corpus **margin** read as a measurement
   of the design when the corpus mixes selected-on and out-of-sample rows. The counter that catches it prints
   the **composition** of a rate beside the rate: **`25/25 rows derived` should become `25/25 derived, 11 in
   the selection set`**, and the sweep should be able to report the two populations separately. That is the
   natural successor to (c), it costs nothing, and it is the only one of these that prevents a misreading
   rather than producing a number.
