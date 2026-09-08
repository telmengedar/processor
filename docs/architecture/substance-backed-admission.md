# Architectural Document: Substance-Backed Admission

> Repo path: `docs/architecture/substance-backed-admission.md` (canonical copy — the DiVoid node carries
> the same document verbatim).
> Project: **#10422** · Vision: **#10424** + the constrained-context vector **#11364**.
> The capability: **`Node.Substance`**, requested by **#11367**, designed and shipped as **#11371**
> (including §17's invalidation, verified live 2026-09-05).
> Consumed findings: admission triage **#11365** · step traces **#11414** · anchor-grounded-recall ruling
> **#12822** (its evidential standard binds this document) · nodes larger than the budget **#11308** ·
> self-poisoning **#11141** · sidecar drift **#11327**.
> Baseline: `main` at **`270f04e`**, read-only. Every repo fact in §3 and §6 was read out of that tree.

---

## TL;DR

*Substance is not a smaller node. It is a **second, lossy representation** of a node, and the whole design
question is where a lossy representation may stand in for a faithful one. The answer defensible on this
project's own evidence is: **only where the faithful one was going to be absent.***

**The recommendation, in four lines.**

| | |
|---|---|
| **Generation** | **Offline**, in a separate binary (`cmd/condense`) over a **named, bounded** node set. Never inside the turn; never asynchronously from the turn. |
| **Use** | **Two-pass admission.** Pass 1 is byte-identical to today's rule. Pass 2 spends only the leftover budget, offering `substance` to candidates pass 1 already cut. |
| **First unit** | **Unit A — generation and the fidelity audit.** Zero harness change. It answers the question that decides whether Unit B may exist at all. |
| **Falsifier** | The **budget dial** on `corpus.json`: if the substance arm's admitted-vs-budget curve is not shifted downward against baseline, the mechanism buys nothing and does not ship. And, harder: **any** required node whose substance fails its pre-registered `why` blocks the ship. |

**Why two-pass and not "always render substance when present".** Under always-substance a lossy
condensation *replaces a faithful node the model already had* — every fidelity defect is a regression. Under
two-pass, every byte of substance rendered occupies budget that would otherwise have been **empty**, so a
lossy condensation is compared against **absence**, never against the full node. The fidelity risk is not
merely mitigated by this choice; it is **bounded by construction**. That is a stronger argument than "simple
versus adaptive", and it is the argument this design rests on.

**Why generation is offline.** #11414 records that retrieval "runs before the model and is deterministic
given the graph", and #11365's entire method rests on two sweeps being "byte-identical candidate list for
candidate list". **In-turn generation destroys that property.** The same row swept twice would produce
different blocks, and every A/B this project has run or will run rests on the baseline that destroys. That
is not a latency cost to be weighed against a benefit; it is the loss of the project's only instrument. The
twenty-model-calls-against-a-cap-of-three arithmetic is the second reason, not the first.

**The honest cost.** At the current 60,000-byte budget this buys **very little**. #11365 §1 measured leftover
budget of **26–2,230 bytes on 23 of 23 rows** — smaller than the median candidate, and for most rows smaller
than a plausible substance. Pass 2 is spending scraps. **The mechanism's value appears as the budget falls**,
which is precisely the regime #11364 declares to be the product and #11414 shows the harness currently
failing in the worst possible way ("removes the answer entirely while the block still looks full"). A reader
evaluating this design at a single point of 60,000 bytes will conclude it is worthless, and at that point
they will be approximately right. **The measurement is therefore a curve, not a point.**

---

## 1. Problem Statement

DiVoid nodes now carry a client-written `Substance`: a condensed form of `Content`, stored on the same node,
purely consumer-side. Processor's memory harness must decide two things:

1. **Where is substance generated?** Nothing generates it today; the field is absent on effectively every
   node in the graph.
2. **When does the harness render substance instead of content?**

The goal is #11364's: **make the harness work at a context budget an ordinary 24 GB GPU can actually
field.** Success is not "the block got smaller". Success is **the required fact reaches the model at a budget
where today it does not**, without the harness silently substituting a summary that no longer carries the
fact.

**The failure this design exists to avoid** is #11365 §9's load-bearing unmeasured claim: *"Not that a
compacted node still answers. Measured: byte headroom. Unmeasured: meaning."* A condensed form that drops the
answer converts a retrieval success into an answer failure, and the existing sweep **cannot see it** —
`eval.scoreOne` matches a required node's **id** against `loop.Disposition.ID` and reads `Included`. It never
inspects the bytes that were rendered. A design that ships condensation on the strength of a rising
`admitted` rate would be reading an instrument that is blind to its own worst outcome.

---

## 2. Scope & Non-Scope

**In scope**

- Reading `substance` on the recall and node routes.
- The admission rule that decides when a candidate is rendered condensed.
- The run-record fields that make that decision auditable after the fact.
- An offline generation pass over a bounded node set, and where its output and provenance are written.
- The instrument change that lets the decision be measured, and the falsifier that decides it.

**Out of scope, explicitly**

| Excluded | Why |
|---|---|
| **The condensation prompt itself** | `kim-prompt-engineer` owns it. §8.4 states what this design *requires* of it; it specifies no wording. |
| **Deduplication / near-duplicate collapse** | #11414 is explicit: *"fourteen near-identical documents condensed individually are still fourteen near-identical documents. Deduplication is a separate mechanism."* This design does not address #11414's failure and must not be presented as doing so. |
| **Anchor condensation** | The anchor is the run's subject and is rendered whole today. Condensing it is a substance-versus-**full-content** substitution, which §7.2's blast-radius argument does not protect. Named as Unit D with its own falsifier; not in this design's shipping unit. |
| **Substance in ranking or embedding** | Measured: substance does not participate in ranking. #11367's go-ahead puts search integration out of scope on the DiVoid side as well. |
| **Graph-wide backfill** | Unit A targets a named set. Coverage policy is §12 and is materially affected by A10's write-triggered invalidation. |
| **Raising `AssemblyByteBudget`** | Withdrawn by #11365 §5 under #11364. This design lowers the budget at which the harness works; it never raises it. |
| **`corpus-anchor.json`** | Nothing here varies the anchor. The corpus stays pre-registered and unspent. |

---

## 3. Assumptions & Constraints

### 3.1 What is known about the field, and how

| # | Fact | Source |
|---|---|---|
| A1 | `substance` is a requestable field on the **exact** route `Recall` already calls; it returns **alongside content in the same round trip**. `nodeFields` takes it too. | Measured. `internal/divoid/client.go:34` |
| A2 | An unknown field name returns **400**. The projection is validated, so adding a field is a deliberate, verifiable change. | Measured |
| A3 | The key is **omitted entirely when unset**. "Never generated" is distinguishable from "generated and empty". | Measured |
| A4 | Substance **does not participate in ranking**. Retrieval ranks on content; substance changes only what is rendered. | Measured, by planting a distinctive marker present only in substance and querying nearly verbatim on it |
| A5 | Opaque verbatim string. No length or shape validation. | Measured |
| A6 | Write path is **`PATCH /api/nodes/{id}`** with a JSON-Patch body, `[{"op":"replace","path":"/substance","value":"…"}]`; clear is the same op with `value: null`. There is **no** `/substance` sub-route — #11371 §12.4 rejected one. A plain-object PATCH body is malformed and is not a backend defect. | #11371; confirmed |
| A7 | Client rule (#11371 §17.3): **treat null, absent, and empty-or-whitespace-only alike as "no substance"**. The server writes only NULL, never `""`; a client may write `""` and it is stored verbatim. | #11371 |
| A8 | Bulk read requires `?fields=substance`; the single-node route returns it unconditionally. Reading substance alone on #11367 costs **374 B** against 8,292 B of content. | #11371 |
| A9 | **Invalidation holds.** A content write clears substance. Verified live on a scratch node: create-with-substance → present; content changed → **cleared**; metadata (name) changed → **preserved**. | Verified 2026-09-05 |
| A10 | **Invalidation is triggered by a content *write*, not a content *change*.** Re-posting byte-identical content clears a perfectly good substance. | Verified 2026-09-05 |

**A3 versus A7.** A3 says omission distinguishes "never generated" from "generated and empty"; A7 instructs
clients to collapse the three states. **This design adopts A7 in the renderer** — the only thing the harness
could do with an empty substance is emit a candidate header with no body, spending bytes to say nothing. The
distinction A3 preserves is real and useful to the *generator* (§6.6); it is useless to the *renderer*.

### 3.2 The invalidation contract — verified, and two properties of it that are not what was described

The dependency this design was originally written against is now satisfied. It is worth being precise about
*what* is satisfied, because two properties of the verified behaviour are load-bearing and neither was in the
original description.

**A9 removes the staleness risk in the dangerous direction.** The harness can never be served a substance
derived from content that has since changed, because the change destroyed the substance. #11367's Q3 (a hash
or timestamp pair) is answered by the server rather than by the field: **absence is the signal**, exactly as
the operator's model states. The design's earlier ledger-as-staleness-detector is therefore **withdrawn** —
it would be a sidecar consulted about a fact the server already guarantees, which is #11327's shape for no
benefit. What remains of the ledger is provenance only (§6.6).

**A10 makes the economics worse than "how often does content change" suggests, and this is the number to
reason about.** Invalidation is conservative — it fires on writes that changed nothing — so it can never
serve a stale condensation. But cache hit rate is governed by **write frequency, not edit frequency**, and on
this graph those diverge sharply: P-40 parity republishes a design document after every revision, and agents
re-post node bodies routinely. Each such write silently discards a substance and buys a regeneration.

Three consequences, all of which this design absorbs rather than works around:

1. **It strengthens the case for offline generation.** On a graph where substance is discarded by ordinary
   write traffic, in-turn generation is not a warm cache with occasional misses — it is a system that pays
   model calls inside turn latency at a rate set by *other agents' publishing habits*. That is an unbounded
   and externally-controlled cost sitting inside a three-call cap.
2. **It is a direct threat to the measurement instrument, and §6.4's `substanceHash` is the defence.** Two
   sweeps minutes apart can differ because an unrelated session republished a candidate node between them.
   #11365's method depends on byte-identical reproduction; without per-candidate evidence of which
   condensation was present, a sweep difference caused by external write traffic is indistinguishable from
   one caused by the change under test. **Every measurement arm must therefore run the pass immediately
   before the sweep, and every sweep must record, per candidate, whether substance was present and its
   hash.** A measurement that does not do both is not comparable to another.
3. **Coverage is a decaying quantity, not an accumulating one.** §12 treats it as such.

**The residual staleness window this design must name, because nothing above closes it.** The pass reads
content, condenses it, and writes substance — three steps with wall-clock time between them. If a content
write lands *inside* that window, the clear fires before the substance write, and the pass then writes a
substance derived from the superseded body onto a node whose content is new. **The backend guarantee does not
cover this; it is a lost-update race, and it is the only route by which a stale substance can exist.**

- *Likelihood:* low. The window is one model call wide, and the target set is documentation and design nodes
  rather than actively-edited ones.
- *Mitigation, and it is cheap:* re-read the node's content hash immediately before the substance write and
  skip the write if it differs from the hash the condensation was derived from. This is a compare-and-write
  without server support, so it narrows the window to one round trip rather than closing it.
- *Residual:* a window of one HTTP round trip. **Accepted, and recorded here so that a future stale substance
  is diagnosed rather than treated as a contract violation.** If it ever matters, the fix is server-side —
  a conditional substance write, or the `substanceDerivedFrom` hash of Q1.
- *Bounded by §7.2 regardless:* even a substance produced through this race is rendered only where the node
  would have been cut, so it never displaces a correct full node.

### 3.3 Constraints inherited from measurement

| Constraint | Consequence for this design |
|---|---|
| #11365: **admission is the binding gate; the block is budgeted, not the candidates** | The mechanism must act at admission. It must not touch ranking, and it must not require the budget to rise. |
| #11365 §1: leftover budget **26–2,230 B on 23 of 23 rows** | Pass 2 has scraps to spend at 60,000. The design is honest that its value is at lower budgets. |
| #11365 §1: cut candidates median **6,425 B**, p90 **24,560 B**, max **195,448 B**; 4 of 460 exceed the whole budget | Substance is the **only** mechanism that reaches #11308's class at all. |
| #11365 §1: the fourteen missing required nodes run **2,459–12,242 B**, median **4,481 B** | The answers are small. A substance at ~10% of content is ~450 B — inside the upper half of the leftover range, outside the lower half. |
| #11414: at a **4,000-byte** budget the answer is removed entirely while the block still looks full | The budget dial is where this design is measured, and 4,000 is the interesting end of it. |
| #11364: the measured working context on the project's own RTX 3090 is 32,768 tokens ≈ 131,000 bytes | The dial's useful range is well below today's constant, not above it. |
| #12822: a design is held to its **own** pre-registered falsifier, honoured even when the headline numbers are positive | §11 is written to be unmoved by net-gain arithmetic. |
| #11360 / #11365 §7: rows r13, r16–r23 are **burned** as a diagnosis set | No claim in §11 is stated about a named row. Claims are stated on curve shape. |

---

## 4. Architectural Overview

```
                        ┌───────────────────────── OFFLINE ──────────────────────────┐
                        │                                                            │
  target set  ────────► │  cmd/condense                                              │
  (§6.5)                │    1. resolve targets → node ids                           │
                        │    2. read content + substance (one projected GET)         │
                        │    3. skip where substance already present (A7 normalised) │
                        │    4. condense            (ModelPort, pinned sampling)     │
                        │    5. re-read content hash; abort this node if it moved    │
                        │    6. write   (PATCH /api/nodes/{id}, JSON-Patch)          │
                        │    7. append provenance row; emit audit bundle             │
                        └────────────────────┬───────────────────────────────────────┘
                                             │ writes substance onto nodes
                                             ▼
                          ╔══════════════════════════════════╗
                          ║   DiVoid graph  (node.substance) ║
                          ║   cleared by any content write   ║
                          ╚══════════════════┬═══════════════╝
                                             │ read in the same round trip as content (A1)
                        ┌────────────────────┴───────────────────── IN-TURN ─────────┐
                        │                                                            │
   input ──► Retrieve ──┤  candidates now carry BOTH Content and Substance           │
             (unchanged)│                                                            │
                        │  Assemble                                                  │
                        │    remaining = budget − len(anchor.Content)   (unchanged)  │
                        │                                                            │
                        │    PASS 1  greedy first-fit by rank, FULL CONTENT only     │
                        │            ══ byte-identical to today ══                   │
                        │                                                            │
                        │    PASS 2  over the candidates PASS 1 cut, in rank order:  │
                        │            admit substance where it exists and fits        │
                        │            the leftover. Never revisits PASS 1's set.      │
                        │                                                            │
                        │  renderBlock — condensed candidates marked in the header   │
                        └────────────────────────────────────────────────────────────┘
```

**The seam.** The turn reads; the pass writes. They share only the graph and the node id. There is no queue,
no worker inside the service, no shared mutable state, and no in-turn model call added. The pass can be
deleted, re-run, or replaced without touching the loop, and the loop degrades to today's exact behaviour
wherever the pass has not run.

---

## 5. Components & Responsibilities

| Component | Owns | Does **not** own |
|---|---|---|
| **`internal/divoid.Client` (read)** | Requesting `substance` in both projections and carrying it onto `loop.Candidate` and `loop.Anchor`, A7-normalised at the boundary. | Deciding whether it is usable. |
| **`internal/divoid.Client` (write)** | One new method setting a node's substance via A6's JSON-Patch. Never writes `content`. Never creates nodes. | Choosing what to condense, or what the text should be. |
| **`loop.admit`** | The two-pass decision and the per-candidate form verdict. | Rendering. Ranking. Generation. |
| **`loop.renderBlock`** | Emitting the chosen representation and **marking a condensed one as condensed**. | Choosing it. |
| **`loop.Disposition`** | Recording, per candidate, which representation was rendered, at what size, and the hashes of both. | Any interpretation of that record. |
| **`cmd/condense`** | Target resolution, model call at pinned sampling, the compare-and-write, provenance, audit bundle. | Anything inside a turn. Any change to loop behaviour. |
| **`cmd/eval`** | Accepting a block budget as a parameter and reporting it, so the dial exists; recording substance presence per candidate. | Choosing the budget. Scoring fidelity. |
| **The audit bundle** | Pairing each required node's pre-registered `why` with its generated substance, for a reviewer's verdict. | Producing the verdict. It is evidence, not a judgement. |

**What has no owner, and must not silently acquire one:** nothing decides *at runtime* that a substance is
good enough. There is no runtime fidelity check, no length heuristic, no "does this look complete" gate. A
substance is either present — and trusted within §7.2's bound — or absent. A runtime quality heuristic would
be a new unmeasured mechanism wearing a safety argument, which is the shape #12822 rejected.

---

## 6. Interactions & Data Flow

### 6.1 The read path (in-turn, synchronous, unchanged in shape)

`Recall` issues one HTTP GET per query plus one scoped GET. Adding `substance` to `candidateFields` adds
**zero round trips** (A1) and some response bytes: at most the substances of twenty candidates, bounded by
the field's own purpose and absent on every node the pass has not reached.

`Node` (the anchor fetch) also requests it. The anchor's substance is **read and recorded but not rendered**
in this design (§2, Unit D). Reading it costs one field on one node and makes Unit D measurable later from
records this design already writes.

**A7 normalisation happens at the adapter boundary**, once, so that `loop` never sees the three spellings of
absence. The adapter is where "what the wire said" becomes "what the domain means"; putting the check in
`admit` would scatter it.

### 6.2 Admission — the two passes, precisely

Let `remaining = budget − len(anchor.Content)`, floored at zero, exactly as today.

**Pass 1.** Walk candidates in rank order. For each:

- `SelfProduced` → cut, reason `self-produced`. *(unchanged)*
- `cumulative + len(Content) ≤ remaining` → admit, **form = content**, `cumulative += len(Content)`.
- otherwise → cut, reason `byte budget exceeded`. *(unchanged)*

**Pass 1 is byte-identical to today's `admit`.** This is the load-bearing property of the whole design and it
must be pinned by a test asserting identical output on a candidate set carrying no substance anywhere.

**Pass 2.** Walk the candidates pass 1 cut, **in rank order**. For each:

- **Skip if `SelfProduced`.** A cut for self-poisoning is not a cut for size; #11141 is why. Re-admitting a
  run record in condensed form would reintroduce that exact defect at a discount.
- **Skip if substance is absent** under A7.
- **Skip if `len(Substance) ≥ len(Content)`.** A substance no smaller than its content is a generation defect,
  not a compaction; rendering it would spend the same bytes and lose fidelity for nothing.
- `cumulative + len(Substance) ≤ remaining` → admit, **form = substance**, `cumulative += len(Substance)`,
  clear the cut reason, set `Included = true`.
- otherwise → leave cut, reason unchanged.

**Why not the simpler per-candidate ladder** (try content, else substance, else cut, in one pass)? Because it
displaces. A rank-5 node whose content does not fit would be admitted as substance, consuming bytes that
today go to a rank-9 node that fits whole — so a node admitted today becomes cut. The ladder trades one
answer for another and requires a measurement to justify each trade, which is #11365 §5's rejected
prefer-smaller in a new costume. **Two-pass gives `admitted ⊇ today's admitted` by construction**, which is
what turns §11's regression test from a judgement call into a clean falsifier.

**Ordering within pass 2 is rank order and nothing else.** Prefer-smallest-substance-first would admit more
nodes per row and is a plausible improvement; it is also a new selection rule, and this design introduces no
selection rule it has not measured. It is Q3 in §13, to be settled behind a sweep flag before it is anybody's
default.

### 6.3 Rendering

Each candidate block already carries `id / type / name`. A candidate rendered as substance carries **one
additional header line naming the form**.

**Why mark it.** An unmarked condensation invites the model to treat a lossy artifact as the complete node,
and this project has a named class for printed prose asserting something the artifact does not support
(**#10943 mode 10**, recorded three times in one file). The marker costs roughly 25 bytes per condensed
candidate against a mechanism reclaiming thousands.

**What the marker must not do:** editorialise. It names the form; it does not say "may be incomplete". How
the system text instructs the model to treat a condensed block is a prompt question, belongs to Kim, and is
Q4 — the risk that marking causes the model to *discount* a condensed block is real and unmeasured.

### 6.4 The record

Per candidate the record must answer, after the fact: what did the model actually see, and what did it cost?

| Field | Meaning | Change |
|---|---|---|
| `size` | **`len(Content)`** — the node's own size | **unchanged meaning.** Re-basing it onto rendered bytes would silently change what every historical record means. |
| `contentHash` | **`sha256(Content)`** | **unchanged, and this is not optional.** `eval.scoreOne` compares `d.ContentHash` against the corpus's labelled `req.Hash`. Hashing the *rendered* string instead would mark **every** substance-rendered required node `stale: true` — a false rot signal across the whole corpus, indistinguishable by inspection from a real one. |
| `form` | `"substance"` when condensed; omitted otherwise | new |
| `renderedSize` | bytes actually rendered; omitted when equal to `size` | new |
| `substanceHash` | `sha256(Substance)` whenever substance was **present**, whether or not it was rendered | new — and A10 promotes it from nice-to-have to necessary. Without it, a sweep difference caused by an unrelated session republishing a candidate node is indistinguishable from one caused by the change under test. |

`Limits` gains nothing: the two-pass rule is not parameterised. **This design introduces no new tunable
constant**, deliberately. #11365's plateau finding is that the gain comes from the mechanism rather than a
value, and every constant introduced here is a constant somebody later fits.

### 6.5 Generation — target resolution

The pass takes an explicit target set. Three resolvers, in the order they become useful:

1. **Corpus-derived — all Unit A needs.** Every `required[].node` in `corpus.json`: **25 distinct nodes, each
   carrying a pre-registered `why`.** That is the fidelity population. Optionally the union of candidate ids a
   baseline sweep returned — that is the admission population.
2. **Cut-history-derived (later).** The operator's fourth shape — *generate only where it would change the
   outcome* — is unavailable in-turn: by the time you know a candidate is about to be cut, you are inside the
   turn paying model latency for it. **Offline it is available, and it is the right general targeting rule**:
   run records already carry `candidates[].cutReason == "byte budget exceeded"`, so the graph itself holds a
   list of nodes measurably losing to the budget. Caveat, stated rather than discovered: run records are
   deleted under some circumstances (#11141, `run-record-fate.md`), so this population is lossy.
3. **Explicit id list**, for operator-driven passes and re-derivation.

**No resolver walks the whole graph.** Graph-wide coverage is a spend decision with a per-node model call
attached and, under A10, a recurring one. It is not this design's to make.

### 6.6 Generation — provenance, idempotence, and the compare-and-write

**The trigger is the operator's, unchanged:** absent substance and present content. A9 makes absence
authoritative, so the pass needs no staleness logic of its own — **the earlier ledger-as-staleness-detector
is withdrawn.**

What the pass keeps is **provenance**, which the field cannot carry: for each node touched,
`(id, sha256(content) at condensation time, len(substance), ratio, model id, sampling, timestamp)`. It lives
as a DiVoid node linked to this design and the project, not as a repo file — a repo file would drift from a
graph the pass mutates. **The harness never reads it.** It exists so that a later reader can ask which model
and which prompt produced a given substance, and so that the ratio distribution §8.4 needs is a measurement
rather than an impression.

**The compare-and-write (§3.2's race).** Between reading content and writing substance the pass makes a model
call. Immediately before the write it re-reads the node's content hash and **skips the node if it moved**.
This narrows the lost-update window to one round trip. It does not close it; §3.2 records the residual.

**Idempotence.** Re-running the pass over the same targets with the same content must be a no-op: nodes with
substance present are skipped. A `--force` flag re-derives, and is what a prompt revision uses.

**Failure isolation.** A failure on one node — a graph read, the model call, the substance write, or a
condensation that exhausted its output budget — skips that node and continues. The pass never aborts a run
because one node failed, and it reports the skips. A pass that half-completed leaves a graph in a valid
state by construction, because every write is independent and substance is optional everywhere.

*(Corrected 2026-09-08. This sentence read "model error, write rejection, content moved" — three causes, one
of which is not a failure at all. **Content moved is a rule the pass applies**, not a failure of the pass:
the condensation completed and the write was correctly declined because its premise had changed. And it
omitted **truncation**, which is a failure. The corrected list is the four in §6.7's table. The sentence was
written before the partition below existed and nothing pointed at it, which is the gap §6.7 closes.)*

### 6.7 The failure boundary — a principle, not a list

**The pass reports a count of operational failures, and a binary's exit code turns on it.** So the rule
deciding *did the pass fail, or did it correctly decline* is load-bearing, and until recently it existed
nowhere in `docs/architecture/` — it survived only in two test names and their failure strings. This section
is that rule, stated once, where the pass is designed.

> **A skip is *rule-side* when the skip *is* a decision the pass made on a complete basis.**
> **Every other skip is *operational* — by definition, not by enumeration.**

**One side is defined and the other is its complement, and that is the whole point of the shape.** A
principle with two independent positive clauses can leave a hole between them, and this one did — see the
correction below. A defined side plus its complement cannot: every skip is either in the defined set or it
is not.

**The two-part test, for whoever adds the fifteenth reason.** Rule-side requires **yes to both**:

1. **Did the pass have a complete basis to decide on?** If it never obtained one, the skip reports a
   shortfall rather than a judgement. → operational.
2. **Is the skip itself the decision the pass made?** If the pass decided *to write* and the skip is what
   happened instead of that decision, the skip is not the decision. → operational.

**Stated as a principle deliberately, because a list does not survive its own growth.** A fifteenth skip
reason will be added by someone who never read this document; a principle tells them which side it belongs
on, an enumeration only tells them which side today's reasons are on.

**The current partition is a consequence of the principle, not the definition of it.** Fourteen reasons,
ten rule-side and four operational:

| | reasons | why the principle puts them here |
|---|---|---|
| **Rule-side (10)** | node absent · content absent · substance present · self-produced · non-prose content type · condensation empty · condensation not shorter than content · condensation below the floor · condensation opens with a preamble · **content moved during condensation** | In every one the pass held a complete basis and the skip **is** the decision it took on it. The four quality reasons judge a *completed* condensation; `content moved` re-checks the premise before writing and **chooses not to write** |
| **Operational (4)** | graph read failed · model call failed · **substance write failed** · **condensation truncated** | Each fails one of the two tests. `read`/`model` never yield a basis. **`substance write failed` fails test 2**: the decision was *write*, and the skip is what happened instead of it. `condensation truncated` fails test 1 |

**The two write-step reasons are the pair that makes the principle earn its keep.** `content moved` and
`substance write failed` are raised by **consecutive checks inside `condenseOne`**
(`internal/condense/condense.go`), both **after a completed condensation**, and they land on opposite
sides. Nothing about *when* they occur separates them; what
separates them is **chose not to write** versus **was prevented from writing** — test 2, exactly.

**Truncation is the case that tests the other clause.** It *looks* rule-side — output arrived and was
rejected — but the pass never obtained a complete candidate to judge: the model exhausted the output budget
mid-sentence. **Nothing was decided about the node; the machinery ran out.** The shipped test says exactly
this in its name (`TestATruncatedCondensationIsAnOperationalFailureBecauseThePassExhaustedItsOwnOutputBudget`),
and it is why a reader cannot derive the boundary from *"was there output?"*.

**The default direction for an unclassified reason is operational, and under this shape it is *derived*
rather than chosen.** Rule-side is the defined set; membership in it is a positive claim that has to be
established. A reason nobody has classified has not been shown to satisfy either test, so **it is in the
complement by construction** — not by a judgement call about caution. That is a stronger footing than the
previous statement had, and it is what a fail-safe default should rest on. The alternative has already cost
this project once: a pass that condensed nothing and reported `operational failures 0` is precisely the
*instrument reports clean while measuring nothing* shape, and a whitelist that silently absorbs a new reason
reproduces it by construction.

> **Correction, 2026-09-08 — the falsifier fired, on a reason already inside the table.** The first
> statement of this principle read: *"A skip is rule-side when the pass reached a decision about the node
> and the decision was 'no substance belongs here'. It is operational when the pass could not reach a
> decision at all."* Its falsifier asked for *a skip reason whose side cannot be decided by that question*.
>
> **`substance write failed` is one, and it was in the table the whole time.** It *reached* a decision, and
> the decision was that substance **does** belong — so clause 1 excludes it, and clause 2 excludes it too,
> because the pass plainly did reach a decision. It fell between the clauses. Found by QA (**#13305**) while
> re-reviewing the code half, in the sharpest available form: **`content moved` and `substance write failed`
> both sit at the write step after a completed condensation and land on opposite sides, and the original
> question does not separate them.** The missing axis was *chose not to write* versus *was prevented from
> writing*.
>
> **Practical effect was nil** — all fourteen were and are classified correctly in the code, and the
> fail-safe default already made operational the effective complement. **This was a completeness defect in
> the statement, not a misclassification in the pass.** The repair is the shape above: define one side,
> take the other as its complement, so a hole of this kind is not expressible.
>
> **Recorded rather than quietly fixed, because it is evidence about the method.** A principle that named
> what would break it, and was then broken by something already inside its own table, is better evidence
> that the falsifier was doing work than a principle nobody ever tested. The previous wording is quoted
> above rather than deleted, so a reader meeting the old form elsewhere can recognise it.

**Falsifier for this section, restated for the new shape.** A complement cannot leave a gap, so the
falsifiable half is now the **defined** side: *a skip that satisfies both tests — the pass held a complete
basis and the skip is the decision it took — yet ought to be reported as a failure of the pass.* If one
appears, the defined side is drawn in the wrong place and this section is wrong. Note this is a strictly
narrower target than the original falsifier, which is what closing the hole bought.
---

## 7. Data Model (Conceptual)

### 7.1 Entities

| Entity | Owner | Relationship |
|---|---|---|
| **Node** | DiVoid | Carries `content` (authoritative) and optionally `substance` (derived, lossy, client-written, destroyed by any content write). One node, two representations. |
| **Candidate** | the loop | One recall row. Now holds **both** representations plus the graph's rank signal. |
| **Rendered form** | assembly | Which representation reached the model, per candidate, per turn. A property of the **turn**, never of the node. |
| **Provenance row** | `cmd/condense` | What produced one substance, and from which content. |
| **Fidelity verdict** | the reviewer | Whether one substance still supports its node's pre-registered `why`. A property of the **substance**, produced once, outside the harness. |

**The invariant governing all of it:** `content` is authoritative, `substance` is derived. Nothing in the
harness may treat substance as a source of truth about a node, and nothing may write substance and content in
the same operation.

### 7.2 The rule that bounds fidelity risk — stated as an invariant, because it *is* the design

> **A candidate is rendered as substance only in a turn where, without substance, it would have been rendered
> not at all.**

Two-pass admission is the mechanism that makes this true; pass-1 identity is the property that makes it
provable rather than asserted. Its consequences:

- **A lossy substance never displaces a faithful node.** The comparison is always substance-versus-absent.
- **A raced substance (§3.2) never displaces a correct node.** Same comparison — with the honest caveat that
  substance-versus-absent is not automatically a win when the substance asserts a superseded fact. #11308's
  option-2 warning and #11141's scar both say a confidently-stated wrong fact can be worse than silence.
  **I do not claim stale beats absent. I claim only that it is not a regression against a correct full node,
  because a correct full node was never going to be there.**
- **No row that passes today can fail tomorrow.** `admitted ⊇ today's admitted`, at every budget, by
  construction. A regression is therefore an implementation bug rather than a trade-off — which is what makes
  §11's F-C a clean test rather than a judgement call.

**What the invariant costs, stated plainly.** It forecloses the case where a *faithful* condensation would
have been strictly better than a full node — where dropping 90% of a node's prose frees room for three more
answers at no loss. That case is real, it is the larger prize, and this design cannot capture it. Capturing
it requires exactly the fidelity evidence Unit A produces, and it is Unit E.

---

## 8. Contracts & Interfaces (Abstract)

### 8.1 Graph read → loop

| | |
|---|---|
| **Input** | A query, a limit, an optional scope. |
| **Output** | Candidates in the graph's own rank order, each carrying id, type, name, similarity, **content**, **substance**, and the self-produced flag. |
| **Invariants** | Rank order is never re-sorted (existing). Substance is A7-normalised: null, absent, and whitespace-only all arrive as the empty value. Substance is never substituted for content by the adapter. |
| **Failure** | Unchanged. A 400 from an unknown field name (A2) is a startup-visible programming error, not a runtime condition. |

### 8.2 Loop → graph write (new)

| | |
|---|---|
| **Input** | A node id and a substance string. |
| **Semantics** | Replace the node's substance. Never touches content. Never creates or deletes nodes. |
| **Invariants** | The caller has already verified the content hash it derived from is still live (§6.6). The empty string is never written — an empty condensation is a failure and is reported, not stored. |
| **Failure** | Per-node, isolated, reported, non-fatal to the pass. |

### 8.3 Assembly → record

| | |
|---|---|
| **Input** | Anchor, ranked candidates, block budget. |
| **Output** | The rendered block, plus one disposition per candidate carrying the fields of §6.4. |
| **Invariants** | (1) Pass 1's admitted set is exactly what today's rule admits. (2) Every admitted candidate's rendered bytes appear in the block exactly once. (3) `contentHash` is always the hash of content. (4) A candidate rendered as substance is marked as such in the block. (5) `sum(renderedSize) + len(anchor.Content) ≤ budget`. |
| **Purity** | Unchanged: no I/O, no clock, no randomness. |

### 8.4 What this design needs from the condensation prompt — for Kim, not from Sarah

The prompt is the operator's and its wording is out of scope. These are the **properties the architecture
depends on**; if the prompt cannot deliver them, this design's economics or its safety argument change and it
must come back.

| # | Requirement | Why the architecture needs it |
|---|---|---|
| K1 | **A measured ratio distribution**, not a target length. The pass reports `len(substance)/len(content)` per node. | The entire value of pass 2 is that a substance fits in a leftover of 26–2,230 bytes. If the median ratio is 0.5, the mechanism is inert at every budget and F-A fires. This is the single number that decides the design's worth. |
| K2 | **Determinism.** Pinned sampling (temperature 0 or a pinned seed), recorded in the provenance row. | Re-running the pass must not change the block, or every A/B loses its baseline. #11414 already recorded three traces made non-reproducible by an uncontrolled sampler. |
| K3 | **No fabrication and no hedge-hardening.** A condensation that resolves "appears to" into "is" manufactures a fact. | #10943's family is precisely prose asserting something the underlying artifact does not support. A condenser that smooths uncertainty is a machine for producing that class. |
| K4 | **Self-containment.** The substance is rendered without its content, beside other nodes. No "as above", no pronouns referring to elided sections, no references to the node's own structure. | The block is a flat concatenation. A substance that assumes its content is adjacent is unreadable where it is actually used. |
| K5 | **No repetition of id, type, or name.** | The block header already carries them. Repeating them spends the bytes the mechanism exists to save. |
| K6 | **A stated policy for non-prose content**, e.g. `application/json` bodies. Unit A's simplest answer is to exclude them from the target set. | Run records are `application/json` and are already cut as self-produced, so the case is currently moot — but `contentType` varies across the graph and a prose prompt applied to JSON produces something worse than either. |
| K7 | **Failure is a refusal, not a short output.** An empty or refusing condensation must be reportable and must not be written. | §8.2's invariant. A written empty substance is invisible under A7 and wastes a model call twice. |

**The one thing the design cannot ask the prompt for:** a guarantee that the answer survives. That is not a
prompt property; it is a per-node measurement, and §9.2 is how it is taken.

---

## 9. Cross-Cutting Concerns

### 9.1 Determinism and reproducibility

The single most important non-functional property this design protects. **The turn performs no generation**,
so the block remains a pure function of (graph state, anchor, queries, budget) — the property #11414 relies on
and #11365's method requires. The generation pass is the only non-deterministic component, it runs offline,
and its output is pinned into the graph before any measurement reads it.

**A10's consequence for measurement discipline, restated as a rule:** an arm of any A/B must run the pass
immediately before the sweep, and the sweep must record `substanceHash` per candidate. Two sweeps whose
per-candidate substance hashes differ are not comparable, whatever their headline rates say.

### 9.2 Fidelity — how the risk gets caught before it ships, and where it cannot be

This is requirement 3, and it deserves a direct answer rather than a process.

**It gets caught on 25 nodes, using an oracle that already exists and predates substance by weeks.** Every
required node in `corpus.json` carries a hand-authored `why` of the form *"An answer lacking this would
present X, when the truth is Y."* Those sentences are a **pre-registered statement of the fact the node must
carry**, written before substance existed and therefore unfittable to any substance. Examples, verbatim from
the shipped corpus:

- r01/#10861 — *"…would present the long-test-name convention as house style, when the truth is a dated ruling
  that the comment contract binds Go on this repo…"*
- r03/#10877 — *"…would conclude the assertion is dead, when the truth is that a nil-normalising guard one
  layer downstream supplies the same value…"*

**The audit:** for each of the 25, place the `why` beside the generated substance and record a binary
verdict — does the substance still support that statement, unaltered and unhardened? The verdict is rendered
by a reviewer who did not write the prompt. It is recorded before Unit B is briefed.

**Where it cannot be caught, said plainly.** The oracle covers **25 nodes out of a graph of ten thousand**.
For every other node, a substance that quietly drops a fact nobody labelled is **invisible to every instrument
this project has**, and it will remain invisible. The sweep will not see it, because the sweep scores ids.
The answer will look plausible, because that is what a condensation does.

**So the design's response is not coverage; it is blast radius.** §7.2 is the whole answer: an unaudited
substance is only ever rendered where nothing would have been rendered. That converts "a lossy condensation
might lose an answer we had" into "a lossy condensation might fail to supply an answer we did not have" —
which is today's behaviour, at worst. **This is the honest position, and it should not be dressed up as a
fidelity guarantee.** The unaudited majority is trusted because it cannot do harm relative to the status quo,
not because it has been checked.

**What would make it checkable:** an end-to-end answer comparison — the same tasks, the same budget,
substance on and off, with the answers graded. The project has no answer grader; the sweep scores ids. That
instrument is Unit E's prerequisite and this design does not claim it.

### 9.3 Error handling

| Condition | Behaviour |
|---|---|
| Substance absent | Pass 2 skips. Block is today's block. **This is the default state of the graph and must be the boring path.** |
| Substance present but larger than content | Skipped in pass 2, recorded. A generation defect surfaced as data, not as a crash. |
| Substance present but does not fit the leftover | Candidate stays cut, original reason preserved. |
| Pass: model error / write rejection / content moved | Per-node skip, reported, pass continues. |
| Graph returns 400 on the field projection | Startup-visible programming error (A2). |

### 9.4 Observability

- **Per turn:** the record's `form` / `renderedSize` / `substanceHash` per candidate; a run-level count of
  candidates rescued by pass 2 and bytes reclaimed.
- **Per pass:** per node, the ratio; per run, the ratio distribution, skip reasons, and spend.
- **The one warning worth adding:** pass 2 admitted nothing *and* at least one cut candidate had substance
  that did not fit. That is the signal that the leftover is too small for the mechanism to operate — the
  condition under which F-A fires — and it should be visible without a sweep.

### 9.5 Security, concurrency, consistency

- **Security:** no new surface. The pass writes to the graph with the same credential the harness already
  holds. It must never write `content` and never create or delete nodes — a bound worth asserting in code,
  because the same client object can do both.
- **Concurrency:** the pass is single-flight per node. Multiple concurrent passes over overlapping targets
  are safe but wasteful; the last write wins and both are derived from the same content unless §3.2's race
  fires.
- **Consistency:** eventual and one-directional. Content is authoritative; substance follows or is absent.
  The harness never waits for substance and never requests its generation.

---

## 10. Quality Attributes & Trade-offs

| Attribute | How this design serves it | What it costs |
|---|---|---|
| **Resource efficiency (#11364's second ground)** | More documents per block at the same byte budget; the block's byte cost per admitted document falls. | Model calls moved offline, not eliminated — and A10 makes them recur with write traffic. |
| **The floor (#11364 item 3)** | The mechanism's whole purpose: admit at budgets where whole-node admission admits nothing. This is what the dial measures. | Nothing at 60,000. The design is worth little at today's constant. |
| **Reproducibility** | Preserved exactly: no in-turn generation. | The pass must be sequenced before every measurement arm. |
| **Fidelity** | Bounded by §7.2's invariant; audited on 25 nodes. | Unaudited everywhere else, permanently (§9.2). |
| **Simplicity** | No new tunable constant. One new field on the candidate, three on the disposition, one new binary. | Two passes are more code than one, and pass 2's skip conditions are four rules that must each be tested. |
| **Reach** | The only mechanism that touches #11308's class — a 195,448 B node becomes admissible for the first time. | Only when its substance fits the leftover, which at 60,000 is uncommon. |

### Alternatives considered and rejected

| Alternative | Rejected because |
|---|---|
| **Generate in-turn on encounter** (the operator's stated model) | Destroys the determinism every measurement in this project rests on — the primary reason. Secondarily: up to 20 model calls before assembly, inside turn latency, against a three-call cap. Under A10 the miss rate is set by other agents' publishing habits, so the cost is externally controlled and unbounded. **The policy is right — absent substance is the trigger. The place is wrong.** |
| **Generate asynchronously — use content this turn, queue the condensation** | Buys latency back but keeps a non-deterministic writer coupled to the turn that read it, and introduces a queue, a worker lifecycle, and a failure mode inside a service that currently has none. All the offline pass's benefits, none of its isolation. Reconsider only if targeting proves impossible. |
| **Always render substance when present** | Turns every fidelity defect into a regression against a node the model previously had in full, and spends fidelity when budget is ample. It also makes the audit's 25-node coverage load-bearing over the whole graph, which §9.2 shows it cannot be. Simpler, and strictly worse where it differs. |
| **Per-candidate ladder in one pass** (content, else substance, else cut) | Displaces: a rank-5 substance consumes bytes a rank-9 full node uses today, so a node admitted today becomes cut. Trades answers for answers and forfeits the clean falsifier. |
| **Parameterised truncation, `min(size, C)`** — #11365 §3's simulated form | Establishes headroom, not meaning; #11308's option-2 warning is that a truncated rule "may read as complete". Substance is the extraction this simulation stood in for; shipping the simulation was never the plan. |
| **Condense the anchor in this unit** | Substance-versus-full-content, so §7.2 does not protect it. It is the highest-value remaining target (#11365 §4: anchors to 70,660 B, 54.2% of a block) and it deserves its own falsifier, not a free ride on this one. Unit D. |
| **A staleness ledger the harness consults** | Withdrawn after A9. It would be a sidecar consulted about a fact the server now guarantees — #11327's shape for no benefit. |
| **Raising `AssemblyByteBudget`** | Concedes #11364's question. Withdrawn by #11365 §5 on its own author's evidence. |

---

## 11. The Measurement That Decides It, and What Would Falsify the Recommendation

### 11.1 The instrument, and why an existing one genuinely works here

**`corpus.json` can carry the admission half, and it is the right instrument.** Substance changes rendered
bytes; admission is measured on bytes; the sweep is deterministic and issues **zero model calls**. Nothing
about the mechanism touches ranking (A4), so retrieval is unchanged by construction and the sweep's retrieved
rate becomes a control rather than an outcome.

This is a rarer situation than it sounds, and it is worth naming why it holds: #12822 retired `corpus.json`
for *positive evidence about retrieval* because the corpus cannot vary the anchor. **This design does not
touch retrieval.** It changes what admitted bytes are made of, and the corpus's role as a harm guard —
explicitly preserved by #12822 — is exactly the role required.

**One change to the instrument is needed:** the block budget is compiled in at three places
(`internal/eval/result.go:71–72`, `:89`, and `cmd/eval/sweep.go` via `loop.AssemblyByteBudget`). It must
become a parameter, reported in `Result.Limits` — where the field already exists. That is a bounded change
and it is the thing #11365 F5 said would become available once the block was bounded.

**`corpus-anchor.json` is not spent.** Nothing here varies the anchor.

### 11.2 The measurement

**The budget dial.** Two arms — `baseline` (no substance in the graph) and `substance` (Unit A's pass run
immediately before) — swept at `assemblyByteBudget ∈ {60000, 30000, 15000, 8000, 4000}`. Metric: the
**labelled `admitted` rate as a function of budget**, plus the control stratum's budget-cut alarm.

**Pre-registered predictions, written before any run:**

| | Prediction |
|---|---|
| **P1** | At 60,000, `admitted` is **≥** baseline, and the difference is small — 0 to +2 rows. |
| **P2** | The substance arm's curve is **shifted to lower budgets**: the budget at which labelled `admitted` first falls below the baseline's own 60,000-byte value is **strictly lower** in the substance arm. This is the claim. |
| **P3** | `retrieved` is **identical**, row for row, at every budget, in both arms. |
| **P4** | No row admitted in baseline is cut in the substance arm, at any budget. |

### 11.3 Falsifiers

**F-A — the one that rejects the recommendation.** *The substance arm's admitted-vs-budget curve is not
shifted: at every budget the two arms admit the same rows.* Then the mechanism buys nothing at admission and
**it does not ship into the loop.** The most likely cause is K1 — the ratio is not good enough for a substance
to fit a leftover — and the correct response is to take that back to the prompt, not to widen the mechanism.

**F-B — the one that matters more, and it is not about rates.** *Any required node whose substance fails its
pre-registered `why`* (§9.2) — the fact absent, altered, or hardened from a hedge — **blocks the ship.**
Threshold stated before the run: **zero tolerated failures on the 25**, because a required node is by
definition one whose absence changes the answer. A failure returns the prompt to Kim and the pass re-runs
with `--force`. **Three consecutive prompt revisions failing rejects the mechanism**, rather than licensing a
fourth.

**F-C — the implementation-correctness falsifier.** *Any movement in `retrieved`, in either direction, at any
budget* rejects the implementation. Substance does not participate in ranking (A4); movement means the change
reached retrieval. Same shape as #12822's Unit A pre-registration, and for the same reason: a change that
moves a rate it has no mechanism to move is an unmeasured change wearing a correctness argument.

**F-D — standing, on evidence not yet collectible.** If a later end-to-end answer comparison shows tasks whose
required node was admitted *as substance* answering **worse** than the same tasks with that node **absent**,
then §7.2's blast-radius argument is wrong and the ladder must be inverted. This project cannot run that
comparison today (§9.2). **The falsifier is registered anyway**, so that when the grader exists the claim is
already exposed rather than retrofitted.

### 11.4 What this measurement does not establish, and the burn

- **It says nothing about answer quality.** It measures bytes reaching the model, not answers leaving it.
  F-B covers 25 nodes of fidelity; F-D is registered against the rest and cannot yet fire.
- **The 60,000 point is not evidence.** r02 and r09 are the corpus's only known retrieved-then-cut rows and
  are **burned** (#11360, #11365 §7). Any gain at 60,000 lands there, and a claim about it would be
  selection. **Only the curve's shape across the dial is evidence**, and P2 is deliberately stated as a shape
  rather than as a set of rows.
- **Nothing about the mechanism was chosen after seeing which rows miss.** Two-pass admission follows from
  #11365 §3's ruling and §7.2's invariant, not from any row's outcome. That is what makes it legitimate to
  measure on a partly-burned corpus at all — and it is a claim a reader should check rather than accept.
- **One graph state.** #11235 §9.1's ±1-row cross-day drift governs replication, and under A10 the drift now
  has a second source (§9.1).

---

## 12. Migration / Rollout Strategy

**There is no migration.** The graph's default state is "no substance", the harness's behaviour at that state
is byte-identical to today, and coverage grows node by node as the pass runs. There is no cutover, no flag,
and no state in which the system is half-migrated and behaving oddly.

**But coverage decays.** A10 means every content write destroys a substance, and on this graph writes are
frequent and often no-ops. So coverage is a **maintained** quantity:

- **Unit A** covers 25 nodes and re-runs on demand. Adequate for measurement.
- **Beyond that**, coverage requires a cadence — a scheduled pass over a target population — and its cost is
  proportional to *write* traffic on that population, not to edit traffic. **That cost should be measured
  before any graph-wide policy is proposed**, and the provenance rows (§6.6) are what makes it measurable:
  regeneration count per node per week is a number the pass can report.
- **The harness never degrades as coverage decays.** A node that loses its substance is rendered exactly as
  it is today. That is the property that makes decay tolerable and a cadence a tuning decision rather than a
  correctness one.

---

## 13. Open Questions

| # | Question | Who decides |
|---|---|---|
| **Q1** | Should DiVoid expose `substanceDerivedFrom` (the content hash the substance was condensed from)? It would close §3.2's write race at the consumer, at zero extra round trips (A1), and is smaller than the clear that already shipped. | Operator / DiVoid. Not blocking. |
| **Q2** | Is a substance ever *worse than absence* — a superseded or hardened claim rendered where silence would have been better? #11141 and #11308 both suggest yes in the limit. F-D is registered against it; no instrument can fire it today. | Needs the answer grader. |
| **Q3** | Should pass 2 order by rank or by smallest-substance-first? Smallest-first admits more nodes per row; it is also a new selection rule. Free to measure behind a sweep flag. | Measurement, before it is anybody's default. |
| **Q4** | Does marking a block as condensed cause the model to **discount** it? §6.3 argues marking is required on honesty grounds; the cost is unmeasured. | Kim, plus an answer-level comparison. |
| **Q5** | What is the median ratio `len(substance)/len(content)` the operator's prompt actually achieves? **This single number decides whether the design is worth building** (K1, F-A), and Unit A produces it before anything ships. | Unit A. |
| **Q6** | What is the regeneration cost per week under A10 for a realistic target population? Decides whether coverage beyond Unit A's 25 nodes is affordable at all. | Unit A's provenance rows, extended. |

---

## 14. Implementation Guidance for the Next Agent

> **Superseded in part by §15.5 (amendment 2026-09-05): the unit *ordering* below is corrected there.**
> The unit *definitions* stand.

**Three units. One ships first, and it changes no harness behaviour at all.**

### Unit A — generation and the fidelity audit *(the first unit)*

**Scope:** `cmd/condense` with the corpus-derived resolver; the substance write method on the DiVoid client;
pinned sampling; the compare-and-write of §6.6; provenance rows; the audit bundle pairing each of the 25
required nodes' `why` with its generated substance.

**Explicitly not in it:** any change to `loop`, to `cmd/eval`, or to the block.

**What it is worth alone — the reason it is first:**

1. **It produces Q5's number**, and Q5 decides whether Unit B is worth building. If the median ratio is 0.5,
   F-A is already answered and Unit B is never briefed. **Learning that costs one offline pass instead of a
   loop change, an instrument change, and a ten-point sweep.**
2. **It is the only way F-B can fire before the mechanism ships.** #11365 §9 named "a compacted node still
   answers" as its ruling's load-bearing unmeasured claim, and #11365 F1 predicted the exact silent failure —
   `admitted` rising while answers do not. Unit A is where that gets looked for rather than assumed away.
3. **It delivers value with no consumer.** Substance on 25 well-chosen nodes is readable by every agent and
   every human touching the graph, independent of Processor entirely. That is #11367's original ask, and it
   is satisfied by Unit A alone.
4. **It cannot regress anything.** It writes an optional field nothing reads yet.

**Order within the unit:** the write method and its bound (never writes content) → target resolution → the
pass with pinned sampling and per-node isolation → compare-and-write → provenance → audit bundle. The audit
bundle is a deliverable, not an afterthought; the unit is not done without it.

### Unit B — substance-backed admission *(only if Unit A clears F-B and Q5)*

`substance` in both projections; A7 normalisation at the adapter; `Candidate.Substance`; the two-pass
`admit`; the marked rendering; the three new disposition fields with `contentHash` **unchanged**; the block
budget as a `cmd/eval` parameter reported in `Result.Limits`; per-candidate substance presence recorded in
the sweep. Pin pass-1 identity with a test before the second pass exists.

### Unit C — the budget dial run

The two arms, five budgets, the four pre-registered predictions, the four falsifiers. Pass run immediately
before each arm (§9.1). Result filed to DiVoid, linked to this document, with the falsifier outcomes stated
before the discussion of them.

### Later, each needing its own design and its own falsifier

- **Unit D — the anchor.** Substance-versus-full-content; §7.2 does not cover it; #11365 §4 says it is where
  the bytes are.
- **Unit E — faithful condensation preferred over full content**, i.e. relaxing §7.2's invariant. Requires an
  answer grader, which is its real prerequisite.
- **Cut-history targeting** (§6.5 resolver 2) and a coverage cadence (§12), both gated on Q6.

---

## 15. Amendment 2026-09-05 — unit ordering corrected, and this design's relation to #11365 §3

Raised by the coordinator against the hub-pruning ruling (**#12969**) and tonight's census (**#12966**,
**#12967**, **#12968**), after §1–§14 were filed. **§14's ordering is wrong and is corrected here.** Nothing
else in the document is withdrawn.

### 15.1 This design is *not* #11365 §3, and must not be reported as satisfying it

Three mechanisms sit on one axis — *what a shortened form is allowed to displace*:

| | Displaces | Reclaims bytes from | Safety |
|---|---|---|---|
| **#11365 §3 proper** (`min(size, C)`) | full content the model would have had | the **admitted head** (ranks 1–6, 78% of the budget) | none by construction |
| **Unit B, this document** | **nothing** — only what pass 1 already cut | the **leftover**, 26–2,230 B | bounded by §7.2 |
| **Unit D, the anchor** | full anchor content | the **anchor**, to 70,660 B and 54.2% of a block | bounded, if conditional (§15.3) |

§3's measured 2-of-2 conversion comes from **shrinking the head**: compacting a rank-1 node from 32,105 B to
6,000 B frees 26,105 B for ranks 7–20. Pass 2 never touches the head. **Unit B is deliberately weaker than
§3. Shipping B does not satisfy §3's Build ruling, and closing §3 against B would quietly retire a measured
lever.** §3 proper is this document's **Unit E** (§14), deferred for the reason §3 states about itself — its
F1 is written on answers, not on admitted counts — and it cannot be made safe by a conditional, because its
entire gain is bytes taken from candidates that currently fit.

**Generation is built once.** #11367 was written as the answer to §3's unmeasured half; substance *is* the
extraction §3 named and declined to specify. All three units read the same field from the same pass; only the
resolver's target set differs (candidates / anchors / admitted head). That is a flag, not a component.

**One caveat, and §3 supplies its own licence.** Substance is not parameterised: it implements
`min(size, whatever the condenser produced)`, not `min(size, C)` for arbitrary `C`. **§3's plateau — flat
across a 3.3× range of C — is what makes a non-parameterised extract an acceptable stand-in**, and should be
cited as the reason rather than treated as a coincidence.

### 15.2 The defect this surfaced in §11.2 — Unit B's falsifier cannot fire without Unit D

§11.2 sweeps `assemblyByteBudget ∈ {60000, 30000, 15000, 8000, 4000}`. `remaining = budget −
len(anchor.Content)`, floored at zero, and the mean anchor is **12,174 B** (#11365 §4).

**At 8,000 and 4,000 — the two points carrying the entire claim — `remaining` clamps to zero on a large
fraction of rows.** Both arms admit nothing, pass 2 has no leftover to spend, the curve goes flat for a
reason unrelated to substance, and **F-A fires spuriously: the design is rejected on an artifact of the
anchor rule.** The exact row count is computable from a baseline sweep and **must be computed before the dial
is run rather than assumed** — but the direction does not depend on it. Even at half the rows, half the
corpus carries zero information at the end of the dial that matters.

**Unit D is therefore a prerequisite for Unit B's own falsifier, not a preference.**

### 15.3 Unit D, restated as shippable — the conditional anchor

§2 excluded anchor condensation because §7.2 does not protect it. That stands. **It gets its own invariant
instead, and its own falsifier, exactly as §2 required:**

> **The anchor is rendered as substance only in turns where, rendered whole, the block would carry the anchor
> and nothing else.**

The comparison is then *full subject + zero context* versus *condensed subject + N candidates*, taken only in
a state that is already the known #11158 shutout. **It cannot fire on a row that currently works.**

**Falsifier, needing no answer grader:** *no row whose block currently admits ≥1 candidate may change in any
way.* Only degenerate rows move; any other movement rejects the implementation.

**Two implementation invariants.** (1) The budget must be computed from **the representation actually
rendered** — today both derive from `anchor.Content` and they must not diverge. (2) The trigger is "would
admit zero candidates", not a byte constant. No new tunable, consistent with §6.4.

**Two honest flags.**

- **Do not justify Unit D on r05.** #12967 records that anchor as a 70,660-byte **run record** — #11141's
  self-poisoning shape in the subject position, and plausibly a corpus defect rather than a mechanism gap.
  Justify D on the anchor size distribution across the dial, where it is unarguable, and let r05 illustrate
  rather than warrant. Building a mechanism to rescue a bad row is how a fitted constant is born.
- **It makes K6 live.** §8.4 K6 asked for a non-prose content policy while noting the case was moot. A
  run-record anchor is `application/json`; it is no longer moot, and Kim needs that answer for Unit D.

### 15.4 #12968 — greedy rank-order spend

Absorbed already: pass 2 walks in rank order, and prefer-smallest-first is Q3, to be settled behind a sweep
flag rather than shipped as a default. **For Unit E the question is not the same one:** `min(size, C)` changes
*which candidates are in the head at all*, so ordering and compaction interact there in a way they do not in
a leftover fill. One more reason E is not a variant of B.

### 15.5 The corrected order

**§14's ordering is superseded by this list.**

1. **Unit A** — generation + fidelity audit. In flight, unchanged, except: **extend the resolver to cover the
   corpus's anchor (`subject`) nodes as well as its required nodes.** That is the only thing D needs from it.
2. **Unit D — the conditional anchor.** Next. Prerequisite for B's falsifier; addresses #12967; largest
   measured lever in the system.
3. **Unit B — the leftover fill**, as specified in §1–§14. After D, when the dial can discriminate.
4. **Unit E — #11365 §3 proper.** Gated on an answer-level instrument, per §3's own F1. **The §3 Build ruling
   stays open until this ships.** Nothing before it satisfies that ruling.
