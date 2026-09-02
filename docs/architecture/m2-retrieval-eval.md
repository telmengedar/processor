# Architectural Document: M2 — the retrieval eval harness

> Repo path: `docs/architecture/m2-retrieval-eval.md` (canonical copy — the DiVoid node carries the same
> document verbatim).
> Task: **#10924** · Vision: **#10424 §9** and refinement rounds 4, 7, 8 · Project: **#10422**
> Precedent consumed: M1 task **#10521**, M1 design **#10532** (§6.1–§6.4a, §8.2, §9.4), record-fate
> design **#10904**, live verification **#10883**, deferred budget findings **#10822** · Map root **#10454**
> Records read: **#10897** (answer path), **#10898** (tool path)
> Standards applied: Design Contracts **#1136** (§1 KISS/DRY/YAGNI load-bearing; §5 checklist answered in
> §14), Code Contracts **#114 §0** and **§4 via the Go annex #10861** — cited, not restated, and binding
> on everything this design authorises anyone to write · coverage-row rule **#1220 §5 addendum** ·
> zero-dependency property **#10466** · container rule **#10440**
> Baseline: `main` at **`9992209`** (PR #8 merged). Every number in §3 was read out of that tree, out of
> the two live records, or out of a read-only query I ran against `divoid.mamgo.io` on 2026-09-02.
> Measurements are labelled as such; the two derivations are labelled as derivations.

---

## TL;DR

*Scoring retrieval needs **zero model calls**. A full sweep of the corpus costs about seventy graph GETs
and no money. That is the finding that reshapes the milestone, and it is true because M1's own §6.1
already says steps 1–6 contain no model — the harness simply stops there.*

**What is built.** One new command, `cmd/eval`, and one new package, `internal/eval`. Per corpus row it
does exactly what a real run's first six steps do — fetch the anchor, run one recall, call the loop's own
`Assemble` — and then stops before the model. It compares the resulting `[]Disposition` against a
hand-labelled required-node set and prints two rates plus the misses by name.

**`recall@k` is the wrong shape by one word, and the fix is the design's spine.** There are **two**
boundaries, not one `k`: retrieval (`candidateLimit=20`) and admission (a *byte* budget with a
stop-not-skip rule that makes the effective `k` data-dependent). They fail differently and imply opposite
fixes, so the harness reports **two rates** — *retrieved* and *admitted* — and the gap between them is the
budget's cost, attributable per row. **Measured, in the only two runs that exist:** #10897 cut 13 of 20
candidates at **96%** budget utilisation; #10898 cut 13 of 20 at **54%**, because one 30,902 B node at
rank 8 tripped the stop while four candidates below it would have fitted trivially. One number cannot tell
those apart, and they want opposite tunings.

**Ground truth — the ruling, arguing with the brief's position.** Back-derivation (2) is **rejected
outright, including as an expansion mechanism**, and for a harder reason than self-reference: the corpus
would be conditioned on runs judged correct, so the metric *rises* as the retriever gets worse at hard
cases. It moves in the wrong direction. It is also not cheap — the record stores prose, not citations, so
back-deriving needs a model call or a protocol change. **Constructed rows (3) are demoted from a primary
source to a control stratum**, capped and reported separately: a question authored from a node inherits
that node's vocabulary, so it measures an embedding matching a paraphrase of itself and will read ~1.0
under every tuning M3 tries. **Hand-labelling (1) is the primary and only source of the headline number**,
with an explicit written definition of *required*, a one-line justification per required node, and a hard
cap of three required nodes per row. The brief's three-way split collapses on inspection: an
*adversarially* constructed row is a hand-labelled row with a different starting point, so there is one
source (human judgement) with two authoring directions, one control stratum, and one rejected shape — not
a ratio.

**The corpus does not live in the graph, and that is measured rather than argued.** Querying the live
graph on 2026-09-02 with #10897's exact input returns **#10897 itself at rank 1, similarity 0.7143** —
above #10424 at 0.6704, which was rank 1 when the run actually executed. The record's body is **70,660 B**,
larger than the whole 60,000 B assembly budget, so under stop-not-skip it is cut at rank 1 **and takes
every candidate behind it with it**: the block would be the anchor and nothing else. A corpus node would be
strictly worse — it *is* the query text, plus the answer key. **The corpus is a JSON file in the repo.**

**#10822's R13 trigger has fired.** It asked for a session-log run record above rank 5; one is at rank 1
and would produce a shutout. M2 does **not** fix it — fixing the thing you are about to measure, before
measuring it, is backwards. The harness reports self-produced candidates as a named diagnostic, and R13's
exclusion becomes **M3's first tuning decision taken against a number instead of an argument.** That is
the ordering argument of the whole milestone doing its job on day one.

**Cost.** One full sweep of a 30-row corpus: **0 model calls**, ~70 graph GETs, ~2 minutes wall clock, no
currency. The composition path (mechanical + the model's own supplementary query) is **out of scope** and
priced here so the exclusion is deliberate: ~90 model calls and ~1.35 M input tokens for the same corpus.

**Honest resolution limit, stated up front.** 30 labelled rows carry ~45 required-node judgements, so the
standard error near p≈0.8 is about **0.06**. The instrument resolves a move of ~0.12 and **cannot** resolve
0.05. M3 must not read a three-point improvement as a win.

**Two new packages, one moved file, one flag, four exported identifiers. No new dependency, no metrics
framework, no scoring interface, no config surface, no mode the phase does not implement.**

---

## 1. Problem Statement

Vision **#10424 §9**, verbatim, and it is the whole ordering argument:

> *"Retrieval eval harness. The (input → required nodes) corpus and a recall@k score. **Nothing downstream
> is tunable without it, so it comes second, not last.**"*

M3 is the core experiment — three tiers, supersession suppression, a token budget. Every one of those is a
**tuning** decision. M2 exists so those are measured rather than argued. Toni's test for this document,
verbatim from the brief:

> *"So the question the design must keep answering is: **would this instrument change anyone's mind?**"*

That test rules out a design that reports a single scalar. A scalar is a scoreboard. What changes a mind is
an *attribution*: which input, which node, missed at which stage, and what it cost. Every choice below is
made against that test, and §11 records where it lost to something else.

### Success criteria

| # | Criterion | How it is judged |
|---|---|---|
| S1 | A sweep costs no model calls | Grep the eval packages for `ModelPort`, `openaicompat`, or any HTTP POST — must return nothing (§13 F-1) |
| S2 | A miss at retrieval and a miss at admission are never reported as the same thing | Two rates, and a per-required-node verdict from a closed set of four (§5, §8.2) |
| S3 | The harness cannot report a plausible score while measuring nothing | Four independent guards, one of which runs on every sweep (§9.2) |
| S4 | The instrument does not mutate the substrate it measures | The sweep calls `Node` and `Recall` and nothing else; grep falsifier (§9.1) |
| S5 | A number is never printed without the attribution that makes it actionable | Every rate carries its numerator and denominator; every miss is named (§8.3) |

---

## 2. Scope and Non-Scope

### In scope

- A corpus format for `(input, subject) → required nodes`, living in the repository.
- A command that sweeps the corpus against the live graph using the loop's own retrieval and admission,
  and stops before the model.
- Two rates, four per-node verdicts, five diagnostics, one machine-readable result, one human summary.
- The split of `cmd/processor/config.go` into a graph half and a model half, so the eval binary does not
  require a credential it never dereferences.

### Explicitly out of scope

| Not built | Why, and what would change that |
|---|---|
| **Scoring the composition (mechanical + supplementary recall)** | It needs model calls (§10 prices it at ~90 per sweep), it is nondeterministic because the model writes the query, and it measures the model's query-writing rather than the retriever. **Trigger:** when labelled admitted-recall exceeds ~0.9 and answers are still wrong, the binding constraint has moved and this becomes the next instrument. `toolCalls[]` already accrues the data (§9.4 of #10532), so nothing is lost by waiting. Per **#1220 §2 addendum**, this is a named future in prose — **not** a flag, a mode, or a member anyone declares |
| **Scoring answer quality** | A different question with a different instrument (an adversarial reader, per #10424 round 8's own "the gate buys the first; only an adversarial reader buys the second") |
| **Precision, or any measure of noise** | #10424 rules that *"precision without recall is more dangerous than noise"*, and M1 §6.2 deliberately ships no filter. A precision number now would drive M3 toward filtering, which is the wrong direction to push it in first |
| **Fixing R13, R4 or R14** (#10822) | M2 is the instrument. §3.4 records that R13's trigger has fired; §6.4 explains why measuring it beats pre-emptively fixing it |
| **Fixing the anchor duplication found in §3.3** (filed as **#10927**) | Same reason. The harness reports its incidence and cost per sweep, which is what makes the fix a decision rather than a preference |
| **Any change to `internal/loop`** | The eval consumes `Assemble` and the limit constants as they ship. If the eval needed the loop to change, the eval would no longer be measuring what runs |
| **Tuning anything** | #10924, verbatim: *"M2 builds the instrument and reports a number; acting on the number is M3."* |
| **Concurrency in the sweep** | §10 measures a sweep at ~2 minutes. A worker pool buys 90 seconds and costs ordering discipline and error aggregation. **Trigger:** a sweep past ~20 minutes |

---

## 3. What was measured while designing this

Everything in this section is a first-hand reading of `9992209`, of records #10897 / #10898, or of a
read-only query I ran on 2026-09-02. The two derivations are labelled.

### 3.1 The pipeline M2 scores, as it actually ships

`Turn.Run` (`internal/loop/turn.go`) does, in order: `Graph.Node(subject)` → `Graph.Recall(input, 20)` →
`Assemble(anchor, candidates, 60_000)` → the model. Three properties fall out and all three matter:

| Property | Where | Consequence for M2 |
|---|---|---|
| The recall query **is the raw input**, verbatim (`Record.Query == Record.Input`) | `turn.go`, `Recall(ctx, input, CandidateLimit)` | Scoring the retriever needs no query construction and no model |
| **The anchor is fetched by id, not retrieved**, and is exempt from the budget | `assemble.go`, `renderBlock` | The subject can never be a recall miss. A corpus row listing its own subject as required is a free hit (§6.2 rejects it at load) |
| Admission is **a stop, not a skip** | `assemble.go`, `admit` | The admitted set is always a rank *prefix*, so the byte budget behaves as a **variable `k`** (§5.1) |

`Assemble` is pure — no I/O, no clock, no randomness — which is what makes it directly reusable by an
instrument. `Disposition` already carries `Rank`, `Similarity`, `Size`, `ContentHash`, `Included` and
`CutReason` for **every** candidate, admitted or cut. That is #10532 §9.4 obligation 1 discharged, and
without it none of this design would be possible.

### 3.2 The two cut regimes, measured — the reason one number is not enough

| | #10897 (answer path) | #10898 (tool path) |
|---|---|---|
| Candidates retrieved | 20 | 20 |
| Admitted | 7 | 7 |
| Admitted bytes / budget | **57,756 / 60,000 = 96.3%** | **32,504 / 60,000 = 54.2%** |
| Cut at | rank 8 (10,957 B would overflow) | rank 8 (**30,902 B**) |
| Would have fitted below the stop | nothing meaningful | ranks 10, 12, 15, 16 at 1,496 / 2,455 / 2,705 / 3,238 B |
| Wasted budget | 2,244 B | **27,496 B (45.8%)** |
| Similarity spread over all 20 | 0.6388 – 0.6704 (**0.0316**) | 0.7114 – 0.7987 (0.0873) |

Both runs cut 13 of 20. **A single "13 cut" number reports them identically and they are opposite
problems.** #10897 says the budget is genuinely too small. #10898 says one oversized node is costing you
twelve candidates and the budget is half empty. Those imply different M3 changes — raise the budget, versus
cap per-candidate size or make admission a skip. This is the strongest available argument that the metric
must carry an attribution, and it is measured rather than reasoned.

**Second finding in the same table.** #10897's similarity spread across the entire top-20 is **0.0316**.
The ranking barely discriminates on a broad input. Any metric keyed to a small `k` would be dominated by
ordering noise among near-tied scores. That is a further argument for reporting *retrieved anywhere in the
candidate set* as the primary retrieval rate, with the `k`-curve available but secondary (§5.2).

### 3.3 A shipped defect visible in the first live record: the anchor is in the block twice

In #10897 the anchor is **#10422**, and **candidate rank 2 is also #10422**, `included: true`, 2,872 B.
`Assemble` does not remove the anchor from the candidate set. So the block renders the anchor's body twice
and 2,872 B of a 60,000 B budget bought a duplicate.

It was filed nowhere before this design; it is now **#10927**. It corrupts M2's numbers in two directions
at once: it consumes budget that would otherwise admit a real candidate, and it makes any row whose
required set touches the subject a guaranteed hit. §6.2's load-time rejection closes the second. The first is unit A's
assembly and **M2 measures it rather than fixing it** — one boolean and one integer per row (§8.2), which
turns an unmeasured defect into a per-sweep number. It is the same discipline #10822 already applies to R4,
R13 and R14.

### 3.4 The measurement that settles where the corpus lives — and fires R13

**Query run 2026-09-02 against `divoid.mamgo.io`, read-only, with #10897's exact input text:**

| Rank | Node | Similarity | Note |
|---|---|---|---|
| **1** | **#10897** | **0.7143** | the run record of the run that input produced |
| 2 | #10424 | 0.6704 | the vision — **this was rank 1 when the run actually executed** |
| 3 | #10422 | 0.6683 | the project |

`#10897`'s stored body is **70,660 B** — larger than the entire 60,000 B assembly budget, and larger than
the biggest node M1 measured in this graph (42,978 B, C23). Under `admit`'s stop-not-skip rule a rank-1
candidate of 70,660 B is cut immediately and **`stopped` is set before any candidate is admitted**, so
every one of the remaining nineteen is cut too. Replaying that input today yields a block containing the
anchor and nothing else. I call this shape a **shutout** and §8.2 reports it by name.

Two things follow.

**First, #10822's R13 trigger has fired.** R13 asked for *"run ten, read the eleventh's candidate set. If
`session-log` nodes appear above rank 5, or one is admitted and cuts the rest, the exclusion goes in with a
measurement behind it."* One is at **rank 1**, and it does not merely cut the rest — it cuts them without
being admitted. Two runs sufficed, not ten. **Honest caveat:** I probed with the exact input of a past run,
which is the worst case by construction, not a random sample. It is also **precisely the case M2 creates**,
because an eval corpus replays the same inputs deliberately and repeatedly.

**Second, the graph is disqualified as the corpus's home**, and by a stronger argument than R13's. A run
record merely *contains* the input. A corpus row *is* the input, verbatim, plus the ids of the nodes that
must be retrieved for it. Stored as a node it would outrank the run record that already outranks the real
answer — and it would leak the answer key into the context of any real run with a similar input. §6.4
settles it.

### 3.5 What the supplementary path bought, once

#10898's model-composed query returned 20 rows of which **16 were already in the mechanical top-20** — an
80% overlap, four new nodes. It also re-retrieved #10861, which had already been admitted at mechanical
rank 1. **n=1, on a run deliberately phrased to force a tool call**, so this is an observation, not a
measurement of the tool path's value. It is recorded because it is a data point for the §2 trigger that
would bring composition scoring into scope, and because it is evidence the tool path is not obviously worth
90 model calls a sweep today.

### 3.6 Substrate facts

| Fact | Value | Source |
|---|---|---|
| Nodes in the graph | **10,079** | listing `total`, 2026-09-02 |
| Median node size | 5,758 B | C23 (#10532) |
| Recall latency | ~1.5 s per GET, 2.2 s cold | C20 (#10532) |
| Ranking determinism | bit-stable to nine decimals for fixed (graph state, query) | C22 (#10532) |
| `type=` composes with `query=` | yes | C25 (#10532) |
| Graph credential | ambient `DIVOID_URL` / `DIVOID_RAZIEL_KEY`, mapped onto `PROCESSOR_`-prefixed names at launch | #10883 correction |

C22 is load-bearing here in a way it was not at M1: **because retrieval is deterministic given a fixed
graph, two sweeps taken minutes apart differ only where the graph moved.** The instrument's own
repeatability is therefore a property of the substrate, not something the harness has to engineer.

---

## 4. Ground truth — the ruling

The brief's position, verbatim:

> *"a mix weighted toward 3 and 1, with 2 explicitly rejected as a primary source for the reason above —
> though it may be legitimate as a cheap expansion mechanism once a labelled core exists to validate it
> against. I hold this loosely; the reasoning is what matters, not the ratio."*

I agree with rejecting 2 and disagree on the rest. Three changes.

### 4.1 Back-derivation (2) is rejected outright, including as an expansion mechanism

The brief's reason — it cannot discover a node the retriever has never surfaced — is true and is the
weaker half. The disqualifying property is this:

> **Back-derivation samples from runs judged correct. A run is judged correct partly *because* retrieval
> succeeded. The corpus is therefore conditioned on retrieval success, so it systematically over-samples
> the inputs the retriever is already good at — and as the retriever gets *worse* at hard cases, those
> cases stop producing correct runs and stop entering the corpus. The metric goes up.**

A metric that moves in the wrong direction under the failure it exists to detect is not a weak instrument;
it is an anti-instrument. That disqualifies it as a primary source **and** as an expansion mechanism,
because expansion inherits the conditioning. Validating expansion against a held-out labelled set would
correct it, and is more machinery than this milestone should own.

**Second, independent reason: it is not cheap.** `Record.Answer` is prose. Nothing in the record says which
nodes the answer used. Back-deriving would need either a model call to attribute the answer to nodes — a
model judging its own process, which is #10424's *"unverifiable process"* failure exactly — or a citation
protocol added to the system text, which changes M1's shipped surface. The only argument for option 2 was
that it is cheap, and it is not.

### 4.2 Constructed rows (3) are demoted from a source to a control

A question authored *from* node X inherits X's vocabulary. Semantic search over embeddings will then find X
almost regardless of how the retriever is tuned, because the query is a paraphrase of the document.
**Constructed rows measure an embedding's ability to match a paraphrase of itself.** They will read near
1.0 today and near 1.0 after every M3 change. Under Toni's own test — *would this instrument change anyone's
mind?* — a number that cannot move is worth nothing.

Worse, they structurally exclude the failure the vision names as the important one. #10424 §5.2's
worst case is *a confident agent with a clean, plausible context missing the one node that mattered* — which
is the case where the required node's vocabulary does **not** match the input's. C27 is the extreme instance:
`query=why?` returns *"Philosophy"*, *"falloutdude356's obscure probes"*, *"Test"*. A row authored from its
own required node cannot exhibit that case.

And the counter-move dissolves the category. A constructed row *could* be authored adversarially — pick X,
write a question X answers that shares no vocabulary with X. That is a good row, but the authoring is then
exactly as expensive as hand-labelling and the "guaranteed ground truth" property is doing no work, because
you already knew X. So:

> **The cheap version of (3) is a control. The expensive version of (3) is (1).** There are not three
> sources with a ratio. There is one source — human judgement — with two authoring directions, plus a
> control stratum, plus one rejected shape.

Constructed rows are **retained** because the control job is genuinely valuable and is the answer to the
brief's hardest question (§9.2). They are **capped, labelled `control`, and never summed into the labelled
rate** — blending them would inflate the headline and, worse, add trials that all score 1.0, shrinking the
apparent standard error while measuring nothing (§11.4).

### 4.3 Hand-labelling (1) is primary, and its stated weakness is its advantage

The brief names the cost: *"the labeller's model of 'required' becomes the metric's definition."* That is
true, and it is true of **every** source. Under (3) the labeller's definition is *"the node I wrote the
question from"* — which is a worse definition precisely because it is invisible. **An explicit definition
is strictly better than a smuggled one.** So the definition is written down and it is load-bearing:

> **A node is `required` for an input when an answer that does not draw on it is wrong or materially
> incomplete — judged by a reader who knows the correct answer independently of the system.**

That is still a judgement. What makes it checkable rather than a feeling is one operational rule:

> **The labeller must be able to state, per required node, what specifically is wrong with an answer that
> lacks it. That sentence is a field on the row.**

It costs one line, and it does two things. It forces the distinction between *required* and *merely
relevant* — the failure that makes hand-built corpora useless. And it lets a future reader relitigate a row
instead of trusting it, which matters because every row is one person's judgement.

**A hard cap of three required nodes per row**, enforced at load (§6.2). A row listing eight nodes is
measuring "did we get most of the neighbourhood", which is diffuse; a row listing one to three, each with a
stated reason, measures something sharp. A row that cannot be narrowed under three is a row containing two
inputs, and it splits.

### 4.4 One source considered and not built

The graph's ~10,079 nodes already carry explicit human cross-references (*"see #10861"*, *"Related: #10424
§9"*, *"Origin: DiVoid #1168"*). That is a human judgement, already made, that one node is needed to
understand another — and it would be a cheap way to **propose** required nodes to a labeller who cannot list
a node they have forgotten exists.

**Not built.** It yields `(node → related nodes)`, not `(input → required nodes)`; turning it into a row
still requires authoring an input, which returns you to (1). It is a corpus-*authoring* aid, not a harness
component, and building it now is the YAGNI shape #1136 §6 names. **Trigger:** if labelled rows repeatedly
turn out to have missed an obviously-required node that the required node's own inbound references would
have surfaced, it earns a place — as a one-off authoring script, not as part of the harness.

### 4.5 Is the whole framing wrong?

The brief invites that answer. It is not: a corpus of `(input → required nodes)` scored by recall is the
right instrument, for a reason the brief itself supplies — the corpus is stated in terms of **node ids, not
ranks or mechanisms**, so it survives every retriever change M3 makes and is exactly what makes an M3
number comparable to an M1 number. A corpus that named expected ranks would die at the first hybrid-retrieval
change.

**One word of the framing is wrong: `recall@k`.** It presupposes a single `k` is the thing to choose. It is
not — `candidateLimit` is already fixed at 20, and the byte budget makes the effective `k` data-dependent.
§5 replaces the single `k` with two boundaries. That is a sharpening, not a rejection, and I decline to
manufacture a larger disagreement than the evidence supports.

---

## 5. What is scored: two boundaries, not one `k`

### 5.1 The two boundaries

```
input ──► Recall(input, candidateLimit=20) ──► 20 ranked candidates
                                                     │
                                      admit(60,000 B, stop-not-skip)
                                                     │
                                             ┌───────┴────────┐
                                       admitted (a rank      cut
                                        prefix, k' rows)
```

| Boundary | Constant | Failure | What it means |
|---|---|---|---|
| **Retrieval** | `candidateLimit = 20` | the required node is not in the candidate set at all | the retriever never saw it. §5.2 of the vision's worst case |
| **Admission** | `assemblyByteBudget = 60,000` | the required node is in the set with `included: false` | the retriever found it and the assembler threw it away |

Because admission is a stop and not a skip, **the admitted set is always a rank prefix**, so
*admitted-recall* is exactly *recall@k′* where `k′` is that run's admitted count — a **variable `k`
determined by the sizes of the bodies, not by a constant anyone chose.** `k′` is therefore reported per row
(§8.2). Saying "recall@k" without saying which `k` is silently reporting a number whose denominator moved.

### 5.2 The rates

| Reported | Definition |
|---|---|
| **retrieved** | required nodes appearing anywhere in `candidates[]` ÷ required nodes on valid rows |
| **admitted** | required nodes appearing in `candidates[]` with `included: true` ÷ the same denominator |

*retrieved* is the ceiling — what the retriever can possibly deliver. *admitted* is the floor — what the
model actually saw. **The gap is the assembler's cost**, and because each cut carries a rank it is
attributable per row.

The `k`-curve (recall@1/5/10/20) is computable for free from the same `Rank` field and is emitted into the
result file, but it is **not** in the human summary. §3.2's measured 0.0316 similarity spread is why: within
a near-tied top-20 the curve is largely ordering noise, and putting it beside the headline invites reading
noise as signal.

### 5.3 The four verdicts

A closed set, one per required node, each with a producer in this design and a reader today:

| Verdict | Condition | Reads as |
|---|---|---|
| `admitted` | present in `candidates[]`, `Included == true` | the model saw it |
| `cut` | present, `Included == false` | the retriever found it, the budget discarded it — carries the rank |
| `notRetrieved` | absent from all 20 rows | the retriever never surfaced it |
| `unresolved` | the node id no longer resolves in the graph | **not a score.** The row is excluded from both rates and counted separately (§6.5) |

---

## 6. The corpus

### 6.1 Shape

A corpus row is `(input, subject) → required nodes`. **The subject is part of the row**, because `POST /runs`
takes one and an input without a subject is not a runnable input (§3.1). Inventing a default subject would
be inventing part of the situation being scored.

```
id        stable key across sweeps, so results diff even when rows are inserted
input     the text, verbatim — what would be POSTed
subject   the anchor node id
stratum   "labelled" | "control"
required  1..3 entries, each: { node, hash, why }
```

| Field | Earns its place because |
|---|---|
| `id` | the array index shifts when a row is inserted, which would break every result diff |
| `hash` | the required node's content hash at labelling time — §6.5, and directly the #10532 §7 lesson: *"a record of ids alone rots as the nodes change, and without the hash the record looks precise while quietly lying"* |
| `why` | **its value is at authoring time, not read time** — it is what forces the labeller to distinguish required from relevant (§4.3). Its read-time consumer is a human relitigating a disputed miss. Named here explicitly because #1136 §2's transitive-dead-code rule would otherwise correctly kill a field the code never branches on |
| `stratum` | the two rates must never be summed (§4.2). Two members, both implemented at M2 |

**Dropped after the delete test:** `labelledBy` (one author), `labelledOn` (the hash states staleness far
more precisely than a date does). Both are the "audit columns for traceability" shape #1136 §6 names.

### 6.2 Validation at load — every rule is a guard, not a convention

| Rule | Why it is not merely tidiness |
|---|---|
| `required` must not contain the row's `subject` | the anchor is fetched by id and can never be a recall miss (§3.1), so such a row is a **free hit that silently inflates every sweep** |
| `1 <= len(required) <= 3` | §4.3 — above three the row is measuring the neighbourhood |
| every `required` entry has a non-empty `why` and `hash` | §4.3 and §6.5 respectively; an empty one is a row that was never really labelled |
| `id` unique, `input` non-empty, `subject > 0`, `stratum` in the closed set | a malformed row must fail loudly, never silently score |
| a malformed file, or an empty corpus, is an **error** | a sweep reporting `0/0` and exiting 0 is the exact instrument failure this milestone exists to avoid |

### 6.3 Size, and the resolution it buys

**Start at 30 labelled rows plus ~4 control rows.** 30 rows at ~1.5 required nodes each is ~45
required-node judgements. At p≈0.8 the standard error is `sqrt(0.8 × 0.2 / 45) ≈ 0.06`.

> **The instrument resolves a move of roughly 0.12. It cannot resolve 0.05.**

That is stated in the document, printed as a numerator and denominator by the harness (§8.3), and **not
computed by the harness** — a statistics surface is machinery nobody asked for. The denominator is what
prevents the misreading; the arithmetic is the reader's.

30 is chosen because it is what one person will author carefully, not because a formula produced it.
**Trigger to grow it:** a tuning change moves the number by less than ~0.12 and the decision depends on
whether that was real.

### 6.4 Where it lives — a file in the repo, and why the graph is disqualified

**`internal/eval/corpus.json`.** One file, JSON, decoded with `encoding/json`.

§3.4 is the argument and it is measured: a node whose body *is* the query text plus the answer key would
outrank the run record that already outranks the correct answer, would consume or shut out the budget, and
would leak the required-node list into any real run with a similar input.

| Alternative | Rejected because |
|---|---|
| Store in the graph, exclude its type from the recall query (`type=` composes, C25) | makes correctness of the **experiment** depend on a filter inside the **system under test** — the wrong dependency direction. M1 §6.2 ships no filter deliberately, and R13's own fix is still gated behind a measurement. It is a mitigation for an unforced error |
| Store in the graph with restricted `access` | rests correctness on access semantics this design has not measured, and is the same wrong-direction dependency |
| A Go source file (`var corpus = []Row{…}`) | a malformed row would be a build error, which is genuinely nicer — but it makes every corpus edit a code change and requires the labeller to write Go. JSON plus §6.2's loud validation buys the same safety at the point it matters |
| One file per row | N files for no gain; a row is five lines and the corpus is diffed as a unit |

**On the global "DiVoid is the primary store" rule and #10424's "the harness is configuration-free":** the
corpus is neither knowledge nor configuration. It is a **test input and a measuring instrument**, and the
rule's own logic — content that would corrupt the graph stays out — points the same way. This *design
document* goes to DiVoid; the corpus data does not. An instrument that lives inside the system it measures
is precisely the failure the brief's own §4 warns about.

### 6.5 Surviving change — two different questions, two different answers

**A change to `limits` does not touch the corpus.** `(input → required nodes)` says nothing about
`candidateLimit` or the byte budget. What is limits-dependent is the **result**, not the corpus. A result is
therefore `(corpusHash, limits, sweptAt, rows)`, and comparing two results is legitimate when the first is
equal; when `limits` differ, that difference **is** the comparison, which is what M3 does. This is #10532
§8.2's argument for the `limits` field, applied one level up, and it is the same reasoning that made
`limits` a record field in the first place.

**A change to the *retriever* does not touch the corpus either**, and that is the property that justifies
the format: the corpus names required ids, not ranks or mechanisms, so hybrid retrieval, supersession
suppression and tier assembly all score against the same rows.

**Graph drift is the real durability problem**, and the mechanism is the one M1 already chose:

| Live state of a required node | Handling |
|---|---|
| hash matches the corpus hash | the row is valid; score it |
| hash differs | the node moved under the label. **Score it, flag `stale`, report the count.** Not an error — the label may still be right — but a reader must know |
| the id no longer resolves | verdict `unresolved`; **the row is excluded from both rates** and counted separately. Excluding it is the only honest choice: neither a hit nor a miss is true |

**The economy that keeps this cheap:** `admit` already computes `ContentHash` for **every** candidate,
admitted or cut. So a required node that *was* retrieved yields its live hash for free from its
`Disposition`. A separate `Node()` read is needed **only for required nodes that were not retrieved** —
which are exactly the rows whose verdict is a miss and which would be embarrassing if the miss turned out to
be a deletion. §10 prices it.

---

## 7. Components and flow

### 7.1 Components

| Component | Owns | Does not own |
|---|---|---|
| `internal/eval` — corpus | the row type, loading, and §6.2's validation | reading the graph; scoring |
| `internal/eval` — scorer | **a pure function** `(required, []loop.Disposition) → []verdict`. No I/O, no clock | fetching anything; deciding what is required |
| `internal/eval` — reporter | the human summary and the result document | any scoring logic |
| `cmd/eval` | the sweep: boot the graph client, load the corpus, per row call `Node` → `Recall` → `loop.Assemble` → score, then render | the model; writing to the graph; admission logic |
| `internal/boot` (moved) | the boot configuration, split into a graph half and a model half | anything above it |

The scorer being pure is the load-bearing structural choice: it makes the cases that never occur naturally
— required node at rank 1, at rank 20, absent, duplicated — exhaustively testable against hand-built
dispositions (§13).

### 7.2 Flow, per row

```
  corpus row (input, subject, required[])
        │
        ├─► Graph.Node(subject)              ── 1 GET ── not found → row error, reported, not scored
        │
        ├─► Graph.Recall(input, loop.CandidateLimit)   ── 1 GET
        │
        ├─► loop.Assemble(anchor, candidates, loop.AssemblyByteBudget)
        │        └─► block (discarded) + []Disposition   ◄── THE MEASUREMENT
        │
        ├─► score(required, dispositions) ─► verdict per required node
        │
        └─► for each required node NOT in dispositions:
                 Graph.Node(id)  ── 1 GET ── resolves? → notRetrieved : unresolved
```

**The block is computed and discarded.** That is deliberate: calling `loop.Assemble` rather than
reimplementing admission is what makes the harness measure the shipped assembler (§9.3). Rendering ~40 KB
of transient string per row is the price and it is negligible.

**`admit` is unexported and stays that way.** `Assemble` returns the dispositions as its second value, which
is all the harness needs. **No change to `internal/loop`.**

### 7.3 The boot-config split, and why it is not gold-plating

#10466's extension rule is explicit and checkable: *"Do not add a second `os.LookupEnv` anywhere in the
module — one read site is the invariant, and it is checkable with one grep."* `cmd/eval` needs
`PROCESSOR_DIVOID_URL` and `PROCESSOR_DIVOID_KEY`, so it cannot do its own lookups.

`cmd/processor/config.go` moves to `internal/boot` and splits over the same `lookupFunc` and the same
`requireEnv` / `optionalEnv` helpers. **The count of loaders is three, not two**, and the rule that fixes the
count matters more than the count:

> **A loader returns exactly what one caller dereferences. The number of loaders is the number of
> independently-dereferenced groups — not a fixed two. A new member joins the group whose caller reads it, or
> starts a group of its own.**

| Loader | Returns | Called by |
|---|---|---|
| `LoadHTTPAddr` | the listen address — a bare string with a default, not a half | `cmd/processor` |
| `LoadGraph` | `GraphConfig{URL, Key}` | `cmd/processor`, `cmd/eval` |
| `LoadModel` | `ModelConfig{URL, ID, Key}` | `cmd/processor` |

**Ruling on the sixth member (gap 1 of #10928).** Revision 1 of this section said *"a graph loader returning
the two graph members, and a model loader returning the three model members"*. Two plus three is five and the
contract has six — `PROCESSOR_HTTP_ADDR` is neither. **The third one-member loader is right, and this section
was under-counted rather than mis-shaped.** The implementer's reason — folding it into the graph half hands
`cmd/eval` an address it will never bind — points the right way but is one notch too strong: the address is
the *optional-with-a-default* class (#10466), so the eval would not have **failed** without it, merely carried
a field it ignores. A rule stated more strongly than it holds gets applied where it does not, so the
load-bearing reason is recorded as **naming and ownership**: a `GraphConfig` that also carries the server's
listen address is not a graph config, which #6836 forbids, and the next reader of `cmd/eval` would have to
work out why a populated field goes unused. The two config **types** stay exactly as designed.

**Call order in `cmd/processor` is a contract, not a style choice: address → graph → model.** The
process-boundary harness starts a child with only `PROCESSOR_HTTP_ADDR=` set and asserts the boot error names
**that** variable (`TestConfigurationError`, `cmd/processor/process_linux_test.go`). Load the graph half first
and the child dies naming `PROCESSOR_DIVOID_URL`, and the harness then fails for a reason unrelated to what it
tests. Verified red by mutation in the container.

**The process-boundary harness spells its own environment constants rather than importing the moved ones**,
and that is the correct direction for an observer of a process contract: it should state the contract it
asserts, so renaming an internal identifier breaks it loudly instead of letting it follow silently. A later
refactor that "helpfully" re-imports them deletes the guard without touching a test.

| Alternative | Rejected because |
|---|---|
| `cmd/eval` calls the existing whole-config loader | it would then **require `PROCESSOR_MODEL_URL` and `PROCESSOR_MODEL_ID` to run a sweep that never dereferences either.** That is exactly the dishonesty #10424 round 7 corrected about boot inputs, and it blocks the scenario that makes M2 valuable — sweeping retrieval when no model is available, which was M1's actual state for most of a day |
| `cmd/eval` reads the two variables itself | breaks the one-read-site invariant #10466 names as checkable by grep, to avoid a file move |
| Pass the URL and key as flags | a key on a command line is visible in process listings — worse than the error-message leak #10466 §5 already forbids |
| `PROCESSOR_HTTP_ADDR` folded into `GraphConfig` | the type stops being what its name says (#6836), and `cmd/eval` carries a populated field it never reads |
| `PROCESSOR_HTTP_ADDR` folded into `ModelConfig` | strictly worse — a model-free sweep is the exact case this split exists to enable |
| A one-field `ServerConfig` type instead of a bare string | a struct wrapping a single string is an indirection, not a boundary (#1136 §4 can-it-be-merged) |

The split also happens to be the seam #10424 needs when configuration eventually comes from the graph. That
is a **consequence, not a justification** — the justification is the invariant above.

### 7.4 Command surface

One flag: `-corpus <path>`, required, an error naming the flag if absent (matching the boot loader's
required-member class).

**The machine-readable result goes to stdout; the human summary goes to stderr.** So `eval -corpus c.json >
result.json` captures one and shows the other, and there is no `-out` flag to design, validate or test. It
also matches the existing convention — the service's logger writes to stderr.

**Execution venue: a container (#10440).** The sweep binds nothing, opens no window and makes no model call,
so it is an ordinary foreground command by that ruling's own scope correction; it goes in the container
because that is where the `PROCESSOR_`-prefixed environment is already assembled, and because containers
here are **linux/arm64** (`GOARCH=arm64`, per #10440's measured toolchain trap).

---

## 8. Contracts

### 8.1 The corpus row

| Field | Semantics | Invariant |
|---|---|---|
| `id` | stable key across sweeps | unique within the corpus |
| `input` | the text a run would receive, verbatim | non-empty |
| `subject` | the anchor node id | `> 0`, and **absent from `required`** |
| `stratum` | `labelled` or `control` | closed set; the two are never summed |
| `required[].node` | a node id that must be retrieved | `> 0` |
| `required[].hash` | its content hash at labelling time | non-empty; sha256 hex, the same function `assemble.go` uses |
| `required[].why` | what is wrong with an answer lacking it | non-empty |

### 8.2 The result

**Header:** `corpusHash`, `sweptAt`, `limits` (read from `internal/loop`'s exported constants, never
copied), `rowCount`. **No graph URL** — #10466 §5 treats an operator-supplied endpoint as a secret because
it may carry credentials, and the same reasoning binds here.

**Per row**, in corpus order so results diff cleanly:

| Field | Why it is reported |
|---|---|
| `row`, `stratum`, `subject` | identification |
| `candidateCount`, `admittedCount` (`k′`) | the variable `k` §5.1 makes explicit |
| `admittedBytes`, `budgetBytes` | **§3.2's discriminator** — 96% versus 54% utilisation are opposite problems |
| `anchorWasCandidate`, `anchorAdmittedAsCandidate` | **§3.3's shipped defect**, made countable |
| `selfProducedCandidates` | **§3.4 / R13**, made countable |
| `shutout` | `admittedCount == 0` while `candidateCount > 0` — §3.4's catastrophic shape, which a recall number alone reports as an ordinary miss |
| `required[]` → `{node, verdict, rank, stale}` | the attribution that makes the number actionable |

Every one of the five diagnostics survives the delete test: remove `anchorWasCandidate` and a shipped defect
stays invisible; remove `selfProducedCandidates` and the first sweep's low score is inexplicable; remove
`shutout` and an admission catastrophe reads as a retrieval failure; remove the byte pair and the two cut
regimes are indistinguishable; remove `stale` and the corpus rots silently.

**Classifying a self-produced candidate** uses *both* the run node type and the run name prefix — type alone
over-counts, because other agents write `session-log` nodes into this graph. Both constants live in
`internal/divoid/write.go` and are **exported** (`RunNodeType`, `RunNamePrefix`) so the eval reads the same
values the writer uses rather than a copy that can drift. Both are already on a `Disposition`
(`Type`, `Name`), so the classification costs no extra read.

### 8.3 The human summary

Roughly twenty lines on stderr, and two rules govern it:

1. **Every rate carries its numerator and denominator.** `retrieved 34/41 (0.83)`, never `0.83`. §6.3's
   resolution limit is what makes a bare rate dangerous.
2. **Every miss is named** — row, node, verdict, rank — never only counted. This is the rule that answers
   Toni's test: `recall = 0.62` changes nobody's mind; *"row 07 required #10424, which ranked 8 and was cut
   by a 30 KB node at rank 8 while the budget was half empty"* changes it immediately.

```
corpus internal/eval/corpus.json — 34 rows (30 labelled, 4 control), hash 9f2c…
limits candidateLimit=20 assemblyByteBudget=60000

labelled   retrieved 34/41 (0.83)   admitted 22/41 (0.54)
control    retrieved   4/4 (1.00)   admitted   4/4 (1.00)

misses (labelled):
  r03  #10861  notRetrieved                  20 candidates, top similarity 0.71
  r07  #10424  cut at rank 8                 k'=7, 32504/60000 bytes admitted
  ...
diagnostics:
  anchor also a candidate      11/30 rows (admitted in 9)
  self-produced candidates     24 across 30 rows
  shutouts                      3 rows
  stale labels 2   unresolved (excluded from both rates) 1
```

---

## 9. What the instrument must not do

### 9.1 It must not mutate the substrate it measures

The sweep calls `Node` and `Recall` and nothing else. It never calls `WriteRun`, so it never adds to the
pool §3.4 shows is already self-polluting.

**Enforced by a grep falsifier, not by an interface.** A `ReadOnlyGraphPort` with one implementation is the
"abstraction with one implementation" anti-pattern (#1136 §6). The falsifier: grep `internal/eval` and
`cmd/eval` for `WriteRun`, `http.MethodPost`, `http.MethodDelete` — must return nothing.

### 9.2 It must not report a plausible score while measuring nothing

This is the brief's hardest question. Four guards, deliberately independent:

| # | Guard | Kind |
|---|---|---|
| 1 | The scorer is pure, so every verdict is exhaustively unit-testable against hand-built dispositions including cases that never occur naturally | tests (§13 G-5..G-8) |
| 2 | **A deliberately unsatisfiable row must be reported as a miss.** A fixture row requiring a node the query cannot surface. If the harness reports it as a hit, the harness is lying | test (§13 G-15) |
| 3 | **The control stratum runs on every sweep.** Constructed rows should read ~1.0; if they do not, either the graph moved or the harness broke, and the labelled number is not trustworthy that day | runtime, every sweep |
| 4 | Admission is the loop's own `Assemble`, not a reimplementation | test **plus** grep falsifier (§9.3) |

Guard 3 is the one that runs unattended, and it is the real job of the constructed rows §4.2 demoted. This
is #10466's non-negotiable gate applied to the instrument: **a harness you have not seen report a miss is
decoration.**

### 9.3 It must not drift from the loop

Two properties, and they need different instruments — which is the #1220 §5 lesson applied rather than
quoted:

- **Behavioural:** the dispositions the sweep produces for a given `(input, subject)` are identical to those
  a real run's record carries. Pinned by a test (§13 G-14).
- **Structural:** admission and the limit constants are not reimplemented. **A test cannot pin this** — a
  correct reimplementation passes the behavioural test. Pinned by grep falsifiers: no byte-budget
  accumulation loop and no numeric literal equal to any `loop` limit constant, anywhere in `internal/eval`
  or `cmd/eval`.

### 9.4 It must not be a `go test`

The tempting shape — make the sweep a test and let CI run it — is wrong, and worth refusing by name:

> **A measurement is not an assertion.** The whole point of M2 is that the number moves. A guard whose
> expected value changes with the substrate is not a guard; it is a flake with a green tick.

`internal/eval`'s **unit** tests are ordinary tests over the pure scorer and the loader, and they run in
CI with no network. The **sweep** is a command.

### 9.5 Comment discipline

**#10861 binds everything this design authorises.** Concretely for the implementer: no fencing banners, no
body comments, no trailing comments; one tight line of godoc on exported identifiers and one package doc
line per package; test intent carried by the test name. And the row that is not confined to comments —
**no DiVoid node ids or QA finding ids anywhere in source**, including string literals and `t.Fatalf`
messages. A design-section reference (`§6.2`) in a failure message is permitted and is the right way to make
a validation error traceable back to this document.

**A move is not comment-neutral.** Moving a file re-adds every one of its lines into diff scope, so a file
predating the #10861 ruling delivers its violations as **new** ones — *"it was already like that"* stops being
true the moment the lines are in your diff. Drop them in the move rather than carrying them across. Found in
practice when the boot config's test file moved (#10928): three fencing banners and two multi-line doc
comments.

---

## 10. Cost

### 10.1 The mechanical sweep — the headline

Per row: 1 `Node(subject)` + 1 `Recall` + 1 `Node` **per required node that was not retrieved**.

| | Value |
|---|---|
| **Model calls** | **0** |
| Graph GETs, 34-row corpus, ~20% miss rate | ~68 + ~10 ≈ **~78** |
| Wall clock at C20's measured ~1.5 s/GET, sequential | **~2 minutes** |
| Currency | **none** — reads against the user's own instance |

**This is the finding the brief asked to be stated loudly, and it changes the milestone's economics.** The
constraint Toni raised — *"an eval sweep multiplies calls, so sweep size is a design input"* — does not bind
the primary instrument at all. Sweep size is bounded by how many rows a person will label carefully (§6.3),
not by a budget.

### 10.2 The composition path, priced so the exclusion is deliberate

If §2's trigger ever fires and composition scoring is built, the price for the same 34-row corpus, using
M1's measured usage (15,327 in / 130 out on a one-call run; 11,446 + 16,508 in on a two-call run) and
`MaxModelCalls = 3`:

| | Value |
|---|---|
| Model calls at a mean of 1.5 per row | **~51**, up to **102** at the cap |
| Input tokens | **~0.8 M**, up to ~1.5 M |

Against Toni's constraint — *"as long as you don't run insane model loops we should be fine"* — that is
affordable occasionally and not on every change. Which is the whole argument for keeping it out of the loop
that runs on every tuning step.

---

## 11. Quality attributes and trade-offs

### 11.1 The instrument is deterministic because the substrate is

C22 measured DiVoid's ranking as bit-stable to nine decimals for a fixed `(graph state, query)`. `Assemble`
is pure. **So the sweep is deterministic except where the graph moved** — and the content hashes say
exactly where. Repeatability is inherited, not engineered, which is why nothing here caches, seeds or pins.

### 11.2 The honest limit: a sweep is a point-in-time reading of a live graph

Two sweeps a week apart are not directly comparable, because the graph gained nodes. The mitigation is what
§6.5 already buys — the stale/unresolved counts say whether the corpus's own referents moved — and it does
**not** extend to the rest of the graph, which is 10,079 nodes and growing. **Stated, not solved.** Solving
it would need a graph snapshot, which is a substantial mechanism for a problem that has not yet bitten.
**Trigger:** a tuning comparison whose result reverses when re-run days later.

### 11.3 The first sweep will measure a defect, not a retriever — and that is correct

§3.4 predicts it: run records now outrank real content for inputs that resemble past runs, and one of them
is larger than the entire budget. The first sweep's labelled number may be poor for that reason alone.

**Filtering them out would be wrong**, and #10532 §6.2 says why in its own words: *"a corpus that
systematically under-represents exactly the failures the eval exists to find."* The eval measures the
retriever **as shipped**. So the score stands, and the `selfProducedCandidates` and `shutout` diagnostics
name the cause beside it.

> **R13's exclusion then becomes M3's first tuning decision taken against a measurement instead of an
> argument. That is the ordering claim of this milestone — "it comes second, not last" — discharging itself
> on day one.**

### 11.4 Rejected alternatives, and why

| Alternative | Rejected because |
|---|---|
| **One blended recall number** | §3.2 measures two cut regimes that a single number reports identically and that want opposite fixes |
| **Blending labelled and control strata** | inflates the headline and adds trials that all read 1.0, shrinking the apparent standard error while measuring nothing |
| **Driving the sweep through `POST /runs`** | one model call per row for a retrieval metric, and it makes the measurement depend on the model being up. #10532 §6.1's own *"steps 1–6 contain no model"* draws the boundary for us |
| **A metrics framework / plugin scoring interface / config surface** | one metric, one scorer. #1136 §6's "abstraction layer for future flexibility". Zero third-party dependencies is a defended property (#10466) and this is exactly where someone would spend it |
| **Exporting `admit`** | `Assemble`'s second return value is sufficient; exporting would widen a shipped package's API for the instrument's convenience |
| **A `ReadOnlyGraphPort` interface** | one implementation; the grep falsifier is stronger and free |
| **Making the sweep a `go test`** | §9.4 |
| **Concurrent fan-out** | saves ~90 seconds; costs a worker pool, ordering discipline and error aggregation |
| **A precision or noise metric** | §2, and #10424's *"precision without recall is more dangerous than noise"* |

---

## 12. Risks and mitigations

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| E1 | **The labeller's definition of *required* silently drifts** across rows, so the metric measures the labeller | the definition is written (§4.3) and each row carries a `why` that can be relitigated | read ten `why` lines cold; if two use *required* in incompatible senses, the definition needs sharpening before the corpus grows |
| E2 | **The corpus is too small to resolve the moves M3 makes** | §6.3 states the resolution in the document and prints denominators | a tuning change moves the number by less than ~0.12 |
| E3 | **Graph drift makes two sweeps incomparable** | stale/unresolved counts for the corpus's own referents; §11.2 states the residual honestly | a comparison reverses on re-run days later |
| E4 | **Self-produced content dominates and the number says nothing about retrieval** | reported by name per sweep (§8.2), not hidden; §11.3 makes it M3's first decision | `selfProducedCandidates` exceeds ~25% of candidate slots, or shutouts exceed ~10% of rows |
| E5 | **The harness reports plausible numbers while broken** | four independent guards, one running every sweep (§9.2) | the control stratum reads below 1.0 |
| E6 | **The eval drifts from the loop** as M3 changes assembly | behavioural test plus two grep falsifiers (§9.3) | a `loop` constant changes and no eval test notices |
| E7 | **The anchor duplication (§3.3) distorts every number** | rejected at load where it would be a free hit (§6.2); reported per row where it is budget cost (§8.2) | `anchorWasCandidate` is true on a majority of rows and the admitted byte totals are correspondingly inflated |

---

## 13. Coverage — the guard, not the mechanism

Per **#1220 §5 addendum** (origin: my own #10904 §9, corrected after QA #10918 W-6), every row names a
**test**, not a described mechanism, and the falsifier for the table itself is stated:

> **Any row whose named guard would still pass against an implementation lacking the claimed property.**

Two rows below fail that question by construction and are therefore split into a behavioural test **plus**
a structural grep falsifier, rather than being left as a name that reads like evidence.

| # | Property | Guard |
|---|---|---|
| G-1 | A row listing its own subject as required is rejected at load | `TestLoadRejectsARowWhoseRequiredSetContainsItsOwnSubject` |
| G-2 | A row with more than three required nodes is rejected at load | `TestLoadRejectsARowRequiringMoreThanThreeNodes` |
| G-3 | A required entry with an empty `why` or `hash` is rejected at load | `TestLoadRejectsARequiredEntryMissingItsReasonOrItsHash` |
| G-4 | A malformed file, and an empty corpus, are errors — never zero rows scored as success | `TestLoadOnAMalformedFileReturnsAnErrorAndNoRows`, `TestLoadOnAnEmptyCorpusReturnsAnErrorNotAnEmptySweep` |
| G-5 | A required node present and admitted scores `admitted` | `TestScoreClassifiesARequiredNodeAdmittedIntoTheBlock` |
| G-6 | A required node present and cut scores `cut` **and keeps its rank** | `TestScoreClassifiesARequiredNodeCutByTheBudgetAndKeepsItsRank` |
| G-7 | A required node absent from all candidate rows scores `notRetrieved` | `TestScoreClassifiesARequiredNodeAbsentFromEveryCandidateRow` |
| G-8 | `cut` and `notRetrieved` are distinguished, not collapsed | `TestScoreDistinguishesCutFromNotRetrievedOnOtherwiseIdenticalRows` |
| G-9 | A required node whose live hash differs from the corpus hash is flagged `stale` **and still scored** | `TestSweepFlagsAStaleRequiredNodeAndStillScoresItsRow` |
| G-10 | A required node that no longer resolves is `unresolved` and its row leaves **both** rates | `TestSweepExcludesFromBothRatesARowWhoseRequiredNodeNoLongerResolves` |
| G-11 | Every rate is printed with its numerator and denominator | `TestReportPrintsTheNumeratorAndDenominatorBesideEveryRate` |
| G-12 | Every miss is named, never only counted. **Premise that makes it discriminate:** the fixture carries more misses than any plausible truncation limit, so an implementation that prints "the first five" fails it | `TestReportNamesAllTwelveMissesInAFixtureWithTwelveMisses` |
| G-13 | The labelled and control strata are reported as separate rates and never summed | `TestReportKeepsTheLabelledAndControlRatesSeparate` |
| G-14 | **Behavioural half of §9.3:** the sweep's dispositions equal what a real run's record carries for the same inputs | `TestSweepDispositionsEqualTheRecordDispositionsForTheSameAnchorAndCandidates` |
| G-14b | **Structural half of §9.3** — a correct reimplementation would pass G-14, so this is not a test | **Grep falsifier:** no byte-budget accumulation loop, and no numeric literal equal to any `loop` limit constant, in `internal/eval` or `cmd/eval` |
| G-15 | A corpus row demanding a node the query cannot surface is reported as a miss | `TestSweepReportsAMissForARequiredNodeTheQueryCannotSurface` |
| G-16 | `shutout` is reported when candidates were retrieved and none admitted | `TestSweepReportsAShutoutWhenAnOversizedRankOneCandidateAdmitsNothing` |
| G-17 | Whether the anchor also appeared as a candidate, and whether it was admitted, is recorded per row | `TestSweepRecordsThatTheAnchorAlsoAppearedAmongTheCandidates` |
| G-18 | Admitted bytes and the budget are both recorded, so utilisation is derivable | `TestSweepRecordsTheAdmittedByteTotalBesideTheBudget` |
| G-19 | A candidate written by the loop's own write path is counted as self-produced; a foreign `session-log` is not | `TestSweepCountsOnlyRunRecordsAsSelfProducedAndNotOtherSessionLogs` |
| G-20 | The result carries the corpus hash and the loop's limits, so a result names what produced it | `TestResultHeaderCarriesTheCorpusHashAndTheLoopLimits` |
| G-21 | Result rows are emitted in corpus order, so two results diff | `TestResultRowsAreEmittedInCorpusOrder` |
| G-22 | The machine result goes to stdout and the human summary to stderr | `TestSweepWritesTheResultToStdoutAndTheSummaryToStderr` |
| G-23 | The graph boot half loads with **no** model variable present | `TestGraphBootConfigLoadsWhenNoModelVariableIsSet` |
| G-24 | The model boot half still errors when a required model variable is absent | `TestModelBootConfigErrorsWhenTheModelUrlIsAbsent` |
| G-25a | A secret never appears in the **graph** half's boot error. **Premise that makes it discriminate:** after the split no single loader reads both a secret and the member that fails, so the pre-split scenario is vacuous; this is re-pinned on the co-located pair — `PROCESSOR_DIVOID_KEY` present, `PROCESSOR_DIVOID_URL` empty | `TestBootConfigErrorsNameTheVariableAndNeverItsValue`, graph scenario |
| G-25b | A secret never appears in the **model** half's boot error — `PROCESSOR_MODEL_KEY` present, `PROCESSOR_MODEL_ID` empty. **Premise:** the split created a second secret in a second loader, so this half had no guard before it | `TestBootConfigErrorsNameTheVariableAndNeverItsValue`, model scenario |
| G-26 | **§9.1 — the instrument never writes.** No test can pin an absence of calls that are absent | **Grep falsifier:** `WriteRun`, `http.MethodPost`, `http.MethodDelete` return nothing in `internal/eval` or `cmd/eval` |

Applying the table's own falsifier to the rows most at risk: G-4's empty-corpus half fails against code that
returns `0/0` and exits 0 — the test asserts an error, so it discriminates. G-9 fails against code that
never reads hashes — the fixture's hash differs, so it discriminates. G-12 fails against a truncating
reporter **only because** the fixture carries twelve misses; that premise is stated in the row rather than
left for the reader to re-derive. G-14 and G-26 do **not** discriminate for the structural property they
appear to claim, which is why each is split.

---

## 14. Pre-Design Checklist (#1136 §5), answered in order

**KISS / DRY / YAGNI**

| Item | Answer |
|---|---|
| No new type mirroring an existing type | ✓ The corpus row and the result row are distinct shapes with distinct lifecycles (one is authored, one is produced). Nothing mirrors `loop.Disposition` — the harness **consumes** it (§7.2) |
| No abstraction with one implementation | ✓ Two were considered and rejected by name: `ReadOnlyGraphPort` and a scoring interface (§11.4). Grep falsifiers replace both |
| Nothing justified by "we might need X later" | ✓ Composition scoring, corpus growth, concurrency and reference-derived labelling each carry a **named trigger**, not a hope (§2, §4.4, §6.3) |
| No deprecation period, feature flag, compatibility shim or transition window | ✓ None. `cmd/processor/config.go` moves; no shim is left behind |
| `block_size × site_count` quoted for any inline-vs-extract call | ✓ One such call: the run node type and name prefix would be **1 line × 2 sites = 2**, below the ~15–20 threshold — so DRY does not force extraction. They are **exported anyway** (§8.2) on a **drift** argument, not a duplication one: if the write path renames the prefix, a duplicated literal makes the eval silently report zero self-produced candidates |

**Existing systems first**

| Item | Answer |
|---|---|
| Existing surface audited | ✓ `loop.Assemble`, `loop`'s limit constants, `divoid.Client.Node`/`Recall`, and the boot loader are all **reused**. The retrieval primitive is DiVoid's, per #10424 §6's split |
| The concrete reason a new layer cannot live on an existing surface | ✓ `internal/eval` has a genuinely different lifecycle (run-once, exit with a report) from a long-lived server, and `internal/loop` cannot host it without the loop depending on its own instrument. `cmd/eval` is a separate binary for the same reason — a subcommand would put a mode branch into a `main` whose entire contract today is boot-listen-serve |
| Concrete 4-week decision for any new persisted data | ✓ Every corpus and result field's reader is named in §8.1/§8.2; the first decision they enable is R13's exclusion (§11.3). Two fields were **dropped** by this test: `labelledBy`, `labelledOn` (§6.1) |
| Consumer chain recursed for every field | ✓ `why` is the only field the code never branches on; §6.1 names its authoring-time value and its read-time human consumer explicitly rather than letting it pass on "the loader projects it" |

**Configurability**

| Item | Answer |
|---|---|
| Every new knob has a named operator or environment difference | ✓ One flag, `-corpus`, which is an **operand**, not a tuning knob. No thresholds, no floors, no `k`, no budget override |
| No telemetry-then-tune compound | ✓ None. The limits are read from `loop`'s constants and are **not** overridable by the eval — deliberately, since the eval must measure what ships |
| Magic numbers stay `const` | ✓ The one new constant is the required-node cap of three, in code, with §4.3 as its reason |

**Less is better**

| Item | Answer |
|---|---|
| Delete / merge / inline test run on every element | ✓ Five diagnostics each survive an explicit delete test (§8.2); two corpus fields did not and were dropped (§6.1); `-out` was deleted in favour of stdout/stderr (§7.4); a `ReadOnlyGraphPort` was deleted in favour of a grep (§9.1) |
| Trade-offs named where the complex option won | ✓ The boot-config split is the one place a simpler option existed (`cmd/eval` calls the whole loader); §7.3 names the two alternatives and the cost of each |
| Radical-clean chosen where nothing is consumed | ✓ `admit` stays unexported; `internal/loop` is untouched |
| Reader inventories explicit | ✓ §8.1 and §8.2 are field-by-field with their readers |

**Document discipline**

| Item | Answer |
|---|---|
| Cites Code Contracts and Design Contracts as load-bearing | ✓ Header; #114 §4 via #10861 is cited, not restated (§9.5) |
| Out-of-scope listed explicitly | ✓ §2, eight rows, each with a trigger |
| No multi-paragraph rationale for things that obviously stay | ✓ |
| Predecessor designs banner-marked where superseded | ✓ **Nothing is superseded.** #10532 and #10904 are consumed and extended; §16 lists the two places this document *sharpens* them without overriding any claim |

---

## 15. Implementation order

Five steps. Each is independently reviewable and each ends with something observable.

| # | Step | Ends when |
|---|---|---|
| 1 | **Move the boot config** to `internal/boot` and split it into the three loaders of §7.3, over the existing helpers. `cmd/processor` calls all three, in the order address → graph → model. | **Every assertion of the pre-split config tests survives**, with the same environments and the same expected strings; plus G-23, G-24, G-25a, G-25b. Retargeting and renaming a test to its new call surface does not violate this; deleting or weakening an assertion does |
| 2 | **Export `RunNodeType` and `RunNamePrefix`** from `internal/divoid`. | The write path uses the exported names; no literal survives |
| 3 | **`internal/eval` — corpus and scorer.** The row type, the loader with §6.2's validation, and the pure scorer. **No graph access in this step at all.** | G-1..G-8 |
| 4 | **`internal/eval` — reporter**, over hand-built results. | G-11, G-12, G-13, G-16..G-22 |
| 5 | **`cmd/eval` — the sweep.** Boot, load, per-row `Node` → `Recall` → `Assemble` → score, stale/unresolved resolution, render. | G-9, G-10, G-14, G-15; both grep falsifiers (G-14b, G-26) return nothing |

**On step 1's completion condition, corrected (gap 2 of #10928).** Revision 1 required *"the existing config
tests pass unchanged in their new home"* — which the same row's own **split** makes unsatisfiable, because the
split deletes the function every one of those tests calls. Both halves could not hold. Proceeding and flagging
it was the right call; stopping would have made the step unimplementable rather than surfacing a behaviour
change. The general shape is worth more than the fix:

> **A completion condition must be satisfiable by the change it gates.** *"The existing tests pass unchanged"*
> is a condition on a **move** — meaningful only while the change is behaviour-preserving at the call surface.
> A **split** destroys that surface by construction. Where a step changes the call surface, the condition names
> the property that survives it — every assertion, the same environments, the same expected strings — never the
> artifact that cannot.

This is #1220 §5's coverage rule one level up: a condition that reads like evidence versus one that names
something falsifiable. The clause **"with the same environments and the same expected strings"** is what makes
the replacement checkable — a reviewer diffs the assertion bodies, and a rename is visibly not a weakening.

**Then, and not as part of the implementation PR: author the corpus.** Thirty labelled rows and about four
control rows, by a human, against §4.3's definition. That is the milestone's real work and it cannot be
delegated to the code — which is itself a fact worth stating, because a green build here proves the
instrument works and proves nothing about whether it measures anything.

**PR shape (#1176 one-feature-per-PR):** steps 1–2 are pre-existing fix-ups that unblock the feature and are
their own unit; steps 3–5 are the harness. Two PRs, in that order, the second referencing the first.

---

## 16. Where this document sharpens its predecessors

Nothing here supersedes a claim, so nothing gets a `SUPERSEDED` banner. Three sharpenings, recorded so a
reader does not have to diff:

| Predecessor | Sharpening |
|---|---|
| **#10532 §9.4** — *"milestone 2 scores `input → the node ids that must be in scope`"* | It scores it at **two boundaries**, not one, and the *"recall@k is uncomputable without k"* argument for the `limits` field is the right argument for the wrong reason: the byte budget makes `k` **data-dependent**, so `limits` is necessary but not sufficient — `admittedCount` per run is what makes the denominator legible. `limits` already carries what is needed; no record change follows |
| **#10904** — *"the stored node body ... reader: milestone 2's corpus"* | Precise correction: M2's **corpus** is authored in the repo; the stored node bodies are M2's **subject of study** and its regression material, not its corpus. The record-fate design's field decisions are all still right; the label on the reader was one word off |
| **#10822 R13** — *"do not build it before the measurement"* | The measurement has been taken (§3.4) and the trigger has fired, harder than R13 anticipated: the record is larger than the budget, so it produces a **shutout** rather than merely crowding. The instruction stands — M2 still does not build the fix — and §11.3 explains why measuring it first is the point |

### Revision 2 — 2026-09-02, after steps 1–2 were implemented (#10928)

Three corrections to **this** document: two raised by the implementer, the third falling out of the second.
Recorded rather than quietly patched, because a design that is silently corrected teaches nobody.

| Correction | Where it was wrong | Now |
|---|---|---|
| **§7.3 split five of six boot members** | *"a graph loader … and a model loader"*; `PROCESSOR_HTTP_ADDR` is neither and the section did not say where it goes | §7.3 states the **rule** that fixes the count — a loader returns what one caller dereferences — names three loaders, and records the call-order contract the process-boundary harness depends on |
| **§15 step 1's completion condition was not jointly satisfiable** | it asked for a **split** and for *"the existing config tests pass unchanged"*; the split deletes the function those tests call | replaced with the property that survives a split, plus the general shape: a completion condition must be satisfiable by the change it gates |
| **§13 G-25 claimed the secrets guard *"survives the move"*** | it did not. After the split no single loader reads both a secret and the member that fails, so the pre-split scenario went **vacuous** and was re-pinned on co-located pairs | split into **G-25a / G-25b**, one per half, each carrying the premise that makes it discriminate. Coverage **widened** — the split created a second secret in a second loader, and the guard now covers both halves where it covered one |

The third row is the one worth keeping. It is **#1220 §5's own lesson landing on the document that cites it**:
a row describing a *mechanism* (*"survives the move"*) rather than naming a falsifiable property concealed the
fact that the property had changed underneath it. The row was not false when written and was false by the time
it was implemented, and nothing in its wording could surface that. A named guard plus a stated premise would
have.

---

---

## 17. Open questions

1. **Who authors the corpus?** Toni, or an agent whose rows Toni reviews. It changes nothing structural, but
   §4.3's definition means the labeller's judgement *is* the metric, so it should be a deliberate choice
   rather than whoever is free. My recommendation: Toni authors the first ten to fix the definition by
   example, and an agent extends to thirty against those ten.
2. **Should a sweep result ever be filed to DiVoid?** A measurement is knowledge and the Hivemind contract
   says knowledge goes to the graph — but a result node per sweep re-creates §3.4's pollution with a
   near-duplicate flood. **Recommendation, taken as the default:** the harness writes files; a human or agent
   files the *interesting* ones as `documentation` carrying the numbers and the reason, never as a dump.
   Automating it would be exactly the knob #1136 §3 refuses.
3. **Is 30 rows the right start?** §6.3 states what it buys and what it does not. If M3's first tuning move
   is expected to be small, the corpus needs to be larger before M3 begins, and that is a scheduling
   decision rather than a design one.
