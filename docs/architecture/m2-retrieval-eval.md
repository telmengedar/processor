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
> **Revision 7 (2026-09-03)** folds in **#11052**, **#11065** and **#11066**, and rules on **#11069**.
> The task-outcome A/B that #11069 proposes is **not** in this document: it is DiVoid node **#11092**,
> which **consumes** this instrument rather than replacing it. **#11092 has no repo file, deliberately** —
> #1176's 2026-09-03 ruling puts a design that genuinely precedes any code in the graph rather than in a
> repository PR, and #11092 §10 opens by stating that nothing in it is buildable today. Nothing here is
> superseded — §16 argues the split, and §14's last row records why no banner is owed.

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

**What exit 1 means, because the first sweep is about to be read (revision 6, from #11045).** The control
stratum is read at **both** boundaries, not one. A control node that was **never retrieved** is an instrument
failure and exits **1** — the retriever, the graph or the harness is broken and nothing in the sweep is
readable. A control node that was **retrieved and then cut** exits **0** with a named budget alarm, because
that is R13 doing exactly what §11.3 says the first sweep will measure. Scoring controls on `admitted`, as
first written, would have made the pollution itself indistinguishable from a broken harness — and that is the
most likely way this command exits 1 on its first real run. **Exit 0 means the numbers can be believed. It
does not mean they are good.**

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
| S1 | A sweep costs no model calls | **`go list -deps ./cmd/eval` does not contain `internal/openaicompat`**, while `./cmd/processor` does. The only `ModelPort` implementation in the module is not linked into the eval binary, so a model call is not merely unwritten but **unreachable** (§13 G-26a) |
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
- Two rates, four per-node verdicts, six diagnostics, one machine-readable result, one human summary.
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
| **Task-outcome measurement, and the A/B that carries it** *(revision 7)* | It needs model calls and a human judgement of result quality, and it measures a **different object**: end-to-end task outcome at a held-constant memory state, against this document's per-question retrieval at whatever state the sweep meets. Designed in DiVoid **#11092** — a node, not a repo file, per #1176's 2026-09-03 ruling — which **consumes** this instrument — the sweep supplies the only zero-variance signals in that experiment. **This row is a pointer, not a deferral:** the work is designed, and what gates it is #11066 plus a settled task set, not a trigger on this milestone |

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

**Revision 6 sharpens what "~1.0" means here.** A constructed row reads ~1.0 at the **retrieval** boundary by
construction — that is the whole of what makes it a control. It reads ~1.0 at the **admission** boundary only
while nothing oversized outranks it, which is a property of the graph on the day and not of the control. §9.2
guard 3 therefore splits at the two boundaries §5 already drew, and only the retrieval half is an invariant.

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

**A labelled miss is a finding, not a verdict** *(revision 7, from #11065)*. The definition above says
when a node is *required*. It does not say what a **miss** means — and three causes produce output that is
byte-identical in every field the harness prints today:

| # | Cause | What it is |
|---|---|---|
| 1 | the retriever got worse | a real regression |
| 2 | **the graph gained a better node** and retrieval correctly preferred it | an improvement wearing a regression's clothes |
| 3 | the label was wrong, or has become wrong | a corpus defect |

**So a labelled red obliges a look, not a fix — and *"update the corpus"* is a legitimate outcome of that
look.** Toni's words, which are where this came from:

> *"our memory is a living system. as soon as more fitting or more precise nodes exist it will probably
> include different nodes, at some point it might not include the expected ones. So its more of like an
> indicator telling us - we should check that and not outright a fail. The check then would tell us whether
> the signal is actually a red flag or whether we need to change the corpus."*

The reason is §4.5's own property read from the other side. **The corpus names required node *ids*, which
is what makes it survive every retriever change — and it is exactly what makes it not survive the graph
gaining a better answer.** The property that gives the format its durability against one kind of change is
the property that exposes it to the other. That is the trade the format makes; it is stated here rather
than left for a reader to discover from a red.

**The exit code does not change, and revision 6's spine is why.** That revision ruled that the instrument
reports what the pipeline does and does not adjudicate. The same rule governs the labelled rate: a labelled
red is evidence and the reader adjudicates. Labelled misses never drove the exit code (§4.2), and this is
the reason — **you cannot automate *go look*.** What revision 7 adds is the evidence that makes the look
possible: §8.2 retains the candidate set, §8.3 names what outranked the required node, and §6.5 says which
kind of label rot is decidable and which is not.

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

### 5.4 A required node larger than the budget

**This document is one.** #10926 was **95,661 B** when #11046 measured it, #10904 is **79,046 B**, and the
budget is 60,000 B. **A required node larger than the budget can never be `admitted`, whatever the retriever
does.** No figure is quoted for this file's current size on purpose: it grows with every revision, and a
number that must be re-checked on each edit is the same unverifiable-citation shape §13 removed line numbers
for. Its best attainable verdict is `cut`, and it
contributes a permanent zero to the admitted rate for a reason that is not about retrieval at all. Since every
question whose answer genuinely lives in one of this project's design documents is such a row, that is a large
and interesting class. The first corpus keeps **one** of them (`r09`) deliberately, so the fact is countable
rather than invisible.

**It is not rejected at load, and not checked at load at all** — for a reason stronger than taste. The loader
is offline by construction (§15 step 3: *"no graph access in this step at all"*) and every §6.2 rule is
decidable from the file alone. A size check is a graph read, and it would make corpus **validity** depend on
live graph state: a corpus that loaded yesterday would fail today because someone edited a node. That is
exactly the drift §6.5 already settled, and it settled it **at sweep time as a flag on a scored row**, never at
load time as a rejection. The same answer binds here. Rejecting the row outright would additionally forbid the
corpus from stating a true and important fact about the shipped pipeline.

**It does not get its own stratum either.** A third member costs a third rate and would pull the row out of the
labelled rate — discarding the true half of what it measures, which is that the retriever *did* surface a 95 KB
design document for a question about it. §6.1's stratum has two members and keeps them.

**What ships is the fact, not a verdict about it.** The node's own byte size is already on the `Disposition`
the scorer walks — admission is computed from it — so it is carried onto the required-node result and printed
beside every `cut` (§8.2, §8.3), against a budget the same line already prints. The distinction that decides a
milestone is then immediate:

| Reading | What it means | Who can act on it |
|---|---|---|
| `cut`, and the node **fits** the budget | something above it consumed the budget first | M3's tuning. This is a target |
| `cut`, and the node **exceeds** the budget alone | it cannot reach the model at any ranking whatsoever | not tuning. Chunking, which is a different milestone |

Carrying the size rather than a derived `oversize` boolean is deliberate and is §5.1's habit applied once more:
the budget is already on the same line, so a boolean would encode a comparison the reader can make, while the
raw number answers *by how much* — which is the input to whether chunking is worth a milestone.

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
| ~~every `required` entry has a non-empty `why` and `hash`~~ every `required` entry has a non-empty `why`, and a `hash` that is **64 lowercase hex characters** — the form `assemble.go`'s content hash takes | §4.3 and §6.5 respectively; an empty one is a row that was never really labelled. A **malformed** hash is worse than an empty one: it loads, and then reads as `stale: true` on every sweep, which says *the graph moved under a good label* where the truth is *the label was never right*. Silent where it could be caught, misleading where it is seen |
| `required[].node` is unique **within the row** *(revision 7)* | the uniqueness rule was stated for row ids and never for required-node ids inside a row. A repeat counts **one** label two or three times in both rates — numerator and denominator both move, so the row's contribution is silently mis-weighted with no diagnostic anywhere |
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
~~**Trigger to grow it:** a tuning change moves the number by less than ~0.12 and the decision depends on
whether that was real.~~

**Revision 7 suspends the growth instruction, and only the instruction.** The arithmetic above is unchanged
and stays true — 45 judgements at p≈0.8 still carry SE ≈ 0.06, and this instrument still cannot resolve
0.05. What changed is what the number is *for*. Under the task-outcome A/B design (DiVoid **#11092**) the
headline claim is a task outcome, not this rate, so **the corpus is sized to the task set rather than to a
standard error** — and the eleven labelled rows now in `internal/eval/corpus.json` may already be the right
number. **Suspended, not deleted:** the trigger comes back the moment anyone quotes this rate as a headline
rather than as an explanation, and the arithmetic is what makes that quotation wrong. §17 question 1's
*"an agent extends to thirty"* is suspended with it.

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

**And `stale` covers one of the two ways a label rots, not both** *(revision 7, from #11065)*. The table
above reads as *the* drift guard. It is not:

| Rot | Mechanism | Decidable by this harness? |
|---|---|---|
| the required node's **content** changed under the label | content hash inequality | **yes** — this is `stale` |
| a **better node appeared beside it**, and the label's node is no longer the best answer | none available | **no** |

The second is not decidable by any check this harness can run, **because *better* is precisely the
judgement §4.3 assigns to the labeller** — the same judgement, arriving later. A flag for it would have to
encode the definition of *required*, which §4.3 deliberately left to a human with a stated reason per node.

> **So it does not get a flag. It gets evidence and a procedure.** A flag here is #11034 P-3's shape
> exactly — a guard whose every available response is worse than the violation: it would fire on every
> graph addition, or never fire at all. The evidence is §8.2's retained candidate set and §8.3's
> outranked-by line; the procedure is §4.3's *a labelled red obliges a look*.

**Editing the corpus is therefore expected, and three things already keep it honest** — none of them a new
mechanism, which is why this is a cross-reference rather than a design:

1. **An edit meets the same bar as its creation.** §6.2's validation runs on **every load**, so an edited
   row is checked exactly like a new one. §4.3's test — state what is specifically wrong with an answer
   lacking this node — is a field on the row, not a ceremony at authoring time.
2. **Edits are dated and reasoned in git, not in the file.** The corpus is a JSON file in the repo (§6.4),
   so `git log -p -- internal/eval/corpus.json` already gives every edit with its date, its author and a
   diff of the `why` fields. An in-file changelog would be a second and worse copy of git history —
   #1136 §2 form 3, pure restatement. **The discipline is: a corpus edit is its own commit, and the reason
   for it is the commit message.** This is a benefit of §6.4's repo-over-graph ruling that §6.4 did not
   name.
3. **Comparability across an edit is already guarded**, by the first element of the result tuple above: a
   score is not comparable across a corpus change, because `corpusHash` moves. The change *is* part of
   what you are comparing, and the diff is in git.

**The residual, stated rather than solved** — this section's own habit. No mechanism stops a bad-faith
edit, and building one is P-3's bad trade. §12 E10 carries the risk and its falsifier.

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

**Exit code — and it is narrower than it looks.** `2` is a usage error, `1` means **this sweep's numbers cannot
be believed**, `0` means they can. *Believable* is not *good*: a sweep that faithfully measures a badly
performing pipeline exits **0**, because the number is true and is the thing the milestone exists to produce.
§9.2 guard 3 states the two conditions that make a number unbelievable; §5.4 and §11.3 state the ones that
look like it and are not.

---

## 8. Contracts

### 8.1 The corpus row

| Field | Semantics | Invariant |
|---|---|---|
| `id` | stable key across sweeps | unique within the corpus |
| `input` | the text a run would receive, verbatim | non-empty |
| `subject` | the anchor node id | `> 0`, and **absent from `required`** |
| `stratum` | `labelled` or `control` | closed set; the two are never summed |
| `required[].node` | a node id that must be retrieved | `> 0`; **unique within the row** *(revision 7)* |
| `required[].hash` | its content hash at labelling time | non-empty; sha256 hex, the same function `assemble.go` uses |
| `required[].why` | what is wrong with an answer lacking it | non-empty |

### 8.2 The result

**Header:** `corpusHash`, `sweptAt`, `limits`, `rowCount`. **No graph URL** — #10466 §5 treats an
operator-supplied endpoint as a secret because it may carry credentials, and the same reasoning binds here.

**`limits` carries two members, not `loop.Limits`' five.** The values are read from `internal/loop`'s
exported constants and never copied, but only the two the sweep actually applies are reported:
`candidateLimit` and `assemblyByteBudget`. `supplementaryByteBudget`, `maxModelCalls` and `maxOutputTokens`
govern the model path, which a model-free sweep never exercises — and **reporting a constant the run never
applied is §7.3's dishonesty one level up**: a reader diffing two results would be invited to attribute a
change to a number that was never in force. So the eval declares its own two-member type rather than
embedding the loop's five. That is a **narrowing of what is reported, not a copy of the values**, and G-14b
is the guard that says so.

**Per row**, in corpus order so results diff cleanly:

| Field | Why it is reported |
|---|---|
| `row`, `stratum`, `subject` | identification |
| `candidateCount`, `admittedCount` (`k′`) | the variable `k` §5.1 makes explicit |
| `admittedBytes`, `budgetBytes` | **§3.2's discriminator** — 96% versus 54% utilisation are opposite problems |
| `anchorWasCandidate`, `anchorAdmittedAsCandidate` | **§3.3's shipped defect**, made countable |
| `selfProducedCandidates` | **§3.4 / R13**, made countable |
| `shutout` | `admittedCount == 0` while `candidateCount > 0` — §3.4's catastrophic shape, which a recall number alone reports as an ordinary miss |
| `topSimilarity` | **§3.2's discriminator at the other end.** It separates *the required node was lost inside a near-tied top-20* (measured spread 0.0316) from *recall returned nothing useful at all* — a distinction a rank-less `notRetrieved` verdict cannot make. Read by whoever is deciding whether the retriever or the input is at fault |
| `required[]` → `{node, verdict, rank, size, stale}` | the attribution that makes the number actionable. **`size`** is the required node's own byte count, present whenever the node was a candidate at all — it is what separates a `cut` M3 tuning can rescue from a `cut` no ranking can (§5.4), and it costs no read, because admission already computed it |
| `candidates[]` *(revision 7)* | **the row's full candidate set, in rank order** — the `[]loop.Disposition` the scorer already receives and today discards. It is what makes a labelled miss triageable: §4.3's three causes are byte-identical in every other field, and telling them apart is a human reading of **what outranked the required node**. A `Disposition` carries no content body (rank, id, type, name, similarity, size, content hash, included, cut reason), so a 34-row corpus adds roughly 680 scalar records to a local file — and **no extra graph read**, so §10's GET count is unchanged |

**A *diagnostic* here is a value the sweep computes in order to name one known failure mode** — not the
evidence it was computed from, and not the row's identity *(the boundary is stated after QA #11102 flagged
that it was nowhere stated, so the count below could be neither confirmed nor refuted)*. Under it, `row`,
`stratum` and `subject` are **identification**; `required[]` is **the attribution**; `candidateCount` and
`admittedCount` are the **variable `k`** §5.1 makes explicit; and **`candidates[]` is the evidence** — the
input the diagnostics are derived from, retained rather than computed. **`candidates[]` is therefore not a
seventh diagnostic, and the count is six.** That is also why it carries its own delete test in the next
paragraph rather than joining the list below: **deleting a diagnostic costs an explanation; deleting
`candidates[]` costs the evidence every explanation is built from** — a different kind of loss, and the
reason §11.2's capture-or-lose argument binds it and nothing else in this section.

Every one of the six diagnostics survives the delete test: remove `anchorWasCandidate` and a shipped defect
stays invisible; remove `selfProducedCandidates` and the first sweep's low score is inexplicable; remove
`shutout` and an admission catastrophe reads as a retrieval failure; remove the byte pair and the two cut
regimes are indistinguishable; remove `topSimilarity` and a `notRetrieved` verdict cannot say whether the
retriever was close or nowhere near; remove `stale` and the corpus rots silently.

**`candidates[]` is captured rather than printed, and the argument is §11.2's own** *(revision 7)*. A
sweep is a point-in-time reading of a live graph, so **evidence not written into the result at sweep time
is unrecoverable** — re-running next week queries a different graph and produces a different candidate set.
This is #10532 §7's lesson (*"a record of ids alone rots as the nodes change"*) applied to the instrument
instead of the loop. **Delete test:** remove it and every labelled miss is untriageable *forever*, because
the graph has moved by the time anyone looks. It is the only moment the evidence exists.

**And it adds no exposure.** The withheld graph URL above is a credential; node names are not — and the M1
run record already stores full candidate **bodies** in the graph itself, which is §3.4's whole problem.

**Classifying a self-produced candidate** uses *both* the run node type and the run name prefix — type alone
over-counts, because other agents write `session-log` nodes into this graph. Both constants live in
`internal/divoid/write.go` and are **exported** (`RunNodeType`, `RunNamePrefix`) so the eval reads the same
values the writer uses rather than a copy that can drift. Both are already on a `Disposition`
(`Type`, `Name`), so the classification costs no extra read.

### 8.3 The human summary

Roughly twenty lines on stderr, and ~~three~~ **four** rules govern it:

1. **Every rate carries its numerator and denominator.** `retrieved 34/41 (0.83)`, never `0.83`. §6.3's
   resolution limit is what makes a bare rate dangerous.
2. **Every miss is named** — row, node, verdict, rank — never only counted. This is the rule that answers
   Toni's test: `recall = 0.62` changes nobody's mind; *"row 07 required #10424, which ranked 8 and was cut
   by a 30 KB node at rank 8 while the budget was half empty"* changes it immediately.
3. **The two control alarms are different sentences, and only one of them is an exit code** (§9.2). A control
   that was never retrieved is an instrument failure; a control that was retrieved and cut is a budget
   measurement. Printing them with the same words puts a reader back where §9.2 started.
4. **Every miss also names what outranked the required node** *(revision 7)* — uniformly the three
   highest-ranked candidates by id and similarity, **one rule with no branching on verdict**. For a
   `notRetrieved` it says what came back instead; for a `cut` at rank 9 it says what beat it. This is the
   line that lets a reader run §4.3's *look* without going back to the graph, which by then has moved. A
   miss with fewer than three candidates above it prints what exists rather than padding.

```
corpus internal/eval/corpus.json — 34 rows (30 labelled, 4 control), hash 9f2c…
limits candidateLimit=20 assemblyByteBudget=60000

labelled   retrieved 34/41 (0.83)   admitted 22/41 (0.54)
control    retrieved   4/4 (1.00)   admitted   4/4 (1.00)

misses (labelled):
  r03  #10861  notRetrieved                  20 candidates, top similarity 0.71
               outranked by                  #11052 (0.79)  #11046 (0.77)  #10995 (0.75)
  r07  #10424  cut at rank 8                 k'=7, 32504/60000 bytes admitted, node 30104 B
               outranked by                  #10897 (0.81)  #10898 (0.78)  #10861 (0.76)
  r22  #10904  cut at rank 3                 k'=1, 56302/60000 bytes admitted, node 79046 B
               outranked by                  #10897 (0.83)  #10898 (0.80)
  ...
diagnostics:
  anchor also a candidate      11/30 rows (admitted in 9)
  self-produced candidates     24 across 30 rows
  shutouts                      3 rows
  stale labels 2   unresolved (excluded from both rates) 1
```

And the shape §9.2's split exists for — the same sweep on a day a run record sits at rank 1. It exits **0**:

```
labelled   retrieved 34/41 (0.83)   admitted  0/41 (0.00)
control    retrieved   4/4 (1.00)   admitted   0/4 (0.00)

budget alarm: the control stratum was retrieved in full and cut by the budget. Retrieval is intact and this
sweep's retrieved rate is trustworthy; the admitted rate is a reading of the assembler, not of the retriever.
```

**Two things about the outranked-by line, both deliberate.** `r22` above prints **two** candidates, not
three, because only two outranked it — padding to a fixed three with a zero-valued entry would report a
candidate that does not exist, and the specimen shows the honest shape so an implementer does not have to
infer it.

**And a reading aid that costs nothing, stated in prose rather than computed.** DiVoid node ids are an
ascending primary key, so a candidate id **higher than the required node's** is a node that did not exist
when the label was written. In `r03` above all three outranking candidates are newer than #10861 — §4.3's
cause 2 on sight. It is a **hint for a reader, not a check**: it rests on an id convention this harness does
not own, and promoting it to a computed field would be inventing exactly the flag §6.5 rules out.

---

## 9. What the instrument must not do

### 9.1 It must not mutate the substrate it measures

The sweep calls `Node` and `Recall` and nothing else. It never calls `WriteRun`, so it never adds to the
pool §3.4 shows is already self-polluting.

**Enforced by a test, with a grep as the weaker backstop.** A `ReadOnlyGraphPort` with one implementation is
the "abstraction with one implementation" anti-pattern (#1136 §6), so the sweep consumes `loop.GraphPort`
whole — and that has a consequence revision 1 of this section missed. **Every test double must therefore
implement `WriteRun`**, so a grep for `WriteRun` across `internal/eval` and `cmd/eval` returns a hit in test
sources **by construction, on a fully compliant implementation**. Revision 1's falsifier could never come
back clean, and *a falsifier that fires on compliant code is worse than none* — the next reader runs it,
finds the hit, concludes the property is violated, and learns to disregard the column.

The falsifier that exists is a strict strengthening:

1. **The fake's `WriteRun` calls `t.Fatal`.** A grep could only ever prove a *string* absent; the fatal
   proves the *call never happens*, across every sweep test at once. Inserting one `WriteRun` into `sweepRow` was
   observed red across the sweep tests.
2. **The grep survives as a cheap backstop, scoped to non-test sources**: `WriteRun`, `http.MethodPost` and
   `http.MethodDelete` must return nothing in production code.

### 9.2 It must not report a plausible score while measuring nothing

This is the brief's hardest question. Four guards, deliberately independent:

| # | Guard | Kind |
|---|---|---|
| 1 | The scorer is pure, so every verdict is exhaustively unit-testable against hand-built dispositions including cases that never occur naturally | tests (§13 G-5..G-8) |
| 2 | **A deliberately unsatisfiable row must be reported as a miss.** A fixture row requiring a node the query cannot surface. If the harness reports it as a hit, the harness is lying | test (§13 G-15) |
| 3 | **The control stratum runs on every sweep**, read at **two** boundaries (split below). A control node that was never retrieved means the graph moved or the harness broke. A control node retrieved and then cut means the assembler discarded it — a measurement, not a fault | runtime, every sweep |
| 4 | Admission is the loop's own `Assemble`, not a reimplementation | test **plus** grep falsifier (§9.3) |

Guard 3 is the one that runs unattended, and it is the real job of the constructed rows §4.2 demoted. This
is #10466's non-negotiable gate applied to the instrument: **a harness you have not seen report a miss is
decoration.**

#### Guard 3 splits at the two boundaries — revision 6, from #11045

~~As first written, guard 3 read the control stratum at the admission boundary alone: every control row's
required node had to be `admitted`, and anything less exited 1 saying "either the graph moved or the harness
broke".~~ **That verdict cannot mean what it says while §3.4's pollution is unfixed, and every fact that
proves it is already in this document.** Admission is stop-not-skip (§5.1); #10897 is a **70,660 B** run
record against a 60,000 B budget and #10898 is **56,302 B**; either at rank 1 admits nothing or nearly nothing
and cuts every row beneath it, controls included. **So the most likely cause of a control failure today is the
known, unfixed self-recall pollution — which is neither the harness nor the graph**, and is the one thing the
stratum exists to rule out. It is also the most likely way `cmd/eval` exits 1 on its first real run, and a
first measurement that exits 1 for a misattributed reason is worse than no measurement, because it is read as
evidence about the instrument.

**This was an inconsistency inside the document, not a new decision.** §11.3 already rules on this exact event
for labelled rows — *"the first sweep will measure a defect, not a retriever, and that is correct"* — while
guard 3 applied the opposite rule to control rows running through the identical pipeline, and nothing
reconciled them. §5's own move is the reconciliation: **one boundary was split into two because the two
failures want opposite fixes, and the guard that verifies the metric never inherited the split.**

| The control's required node | Boundary | Reads as | Exit |
|---|---|---|---|
| `notRetrieved` | retrieval | **an invariant is broken.** A query that is a paraphrase of its own required node failed to surface it in twenty candidates: the retriever, the graph or the harness is not doing what C22 measured. Retrieval is the ceiling under every number in the sweep, so none of them is readable | **1** |
| `unresolved`, or the row errored | — | the control's own referent is gone, or the graph call failed. There was **no self-check this sweep**, and an absent guard is not a passing one | **1** |
| no control rows at all | — | unchanged from guard 3 as first written | **1** |
| `cut` | admission | **a budget alarm, and a measurement.** Retrieval is intact — the control is what proves it — and the assembler discarded the node. The retrieved rate is fully trustworthy; the admitted rate is a true reading of a pipeline with a shipped defect in it | **0**, named loudly (§8.3 rule 3) |

**The stratum gains a second job by not aborting.** On a labelled row a shutout is ambiguous — it could be a
bad label. On a control row retrieval is guaranteed by construction, so a cut control is **unambiguously the
assembler**, and it is the sharpest single piece of evidence a sweep can produce about budget collapse. The
first semantics converted that evidence into an exit code and threw it away.

**And the exit code keeps a meaning it can carry.** Nothing else in the sweep can detect a broken retriever: a
labelled row that misses is indistinguishable from a labelled row that was mislabelled, which is exactly why
§9.2 needed a stratum whose retrieval is true by construction. That property is untouched by the split — it
was never an admission property in the first place.

**Four alternatives, rejected:**

| Alternative | Rejected because |
|---|---|
| **Fix R13 first**, so the precondition guard 3 assumed actually holds | it inverts this milestone's own ordering claim. #10822 R13 says *"do not build it before the measurement"*, and §11.3 makes R13's exclusion **M3's first tuning decision taken against a measurement instead of an argument**. Making the defect a prerequisite for the instrument that measures it discards the property the milestone exists to demonstrate — and the fix would then be taken against an argument, since no sweep would have run |
| **Keep `admitted`, but exempt a cut attributable to a self-produced candidate** | it special-cases the one polluter already found and stays wrong for the next one, and it is §11.3's rejected filtering move one level up: the instrument declining to see what it was built to see |
| **Give control rows their own budget** | the control would stop measuring the shipped pipeline, which breaks §9.3's no-drift property, and it is a configuration knob whose named operator does not exist (#1136 §3) |
| **Drop the control stratum, relying on G-15 and the mutation round** | those pin the harness against fixtures. Guard 3 is the only check that runs against the **live graph** on every sweep; deleting it removes the only channel that can say *the graph moved* |

**Score controls on `retrieved` and say nothing further** — #11045's first option — is absent from that table
because it is this ruling minus one report line, and it reaches the same exit codes. It is rejected only for
what it discards: a cut control is the budget evidence named two paragraphs above, and §8.3 rule 2 already
requires that every miss be named rather than counted.

### 9.3 It must not drift from the loop

Two properties, and they need different instruments — which is the #1220 §5 lesson applied rather than
quoted:

- **Behavioural:** the dispositions the sweep produces for a given `(input, subject)` are identical to those
  a real run's record carries. Pinned by a test (§13 G-14).
- **Structural:** admission and the limit constants are not reimplemented. **No single test pins this** — a
  correct reimplementation passes the behavioural test. Pinned by **two mutations** (§13 G-14b): mutate
  `loop.AssemblyByteBudget` and both the admitted counts and the reported limits must move; mutate
  `Assemble`'s admission rule and `admittedCount` must move. Revision 1 asked for a grep instead, and §13
  G-14b records why that grep can never come back clean.

**One rule collision worth settling here rather than rediscovering it.** #10466 requires **literals on the
expected side** of an assertion — *"if the expected value comes from a production constant the handler also
consumes, mutating that constant moves both sides together and the assertion can never fail"*. G-14b requires
the eval to reference `loop`'s constants rather than re-type their values. Applied to the same assertion those
two rules contradict each other, and the resolution is that **each applies where its property lives**:

| Package | Property it owns | Therefore |
|---|---|---|
| `internal/loop` | *the limit is 20 / 60,000* | pins it with **literals** — `internal/loop/turn_test.go` already does |
| `internal/eval` | *I read the loop's constant rather than a copy of it* | references **the constant**; a literal here would invert the property into the exact drift G-14b forbids |

There is no residual gap: the values are pinned one package over, at the layer that owns them. Discrimination
in the eval's own assertion is re-established by mutation instead — swapping the two reported members is
detectable because the constants hold different values. (Surfaced and confirmed by QA #10945 ruling E.)

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
**not** extend to the rest of the graph, which is 10,079 nodes and growing. **Stated, not solved.**
~~Solving it would need a graph snapshot, which is a substantial mechanism for a problem that has not yet
bitten.~~ *(Revision 7: that was a cost argument, and it does not survive a reader who wants a fixed state
on purpose. §11.4 now carries a rejection that does not depend on whether the problem has bitten.)*
**Trigger:** a tuning comparison whose result reverses when re-run days later.

**Revision 7 also sharpens what this residual *is*.** As written above, a new node is framed as a confound
to be held constant. That framing is wrong and Toni's is better: **a better node appearing is the memory
substrate working.** The thing to hold constant is not the graph — it is the question of whether the label
still names the right answer, which is §6.5's second rot and §4.3's *go look*. So this residual is not a
defect of the corpus. **It is the corpus pointing at the harder question** — *does more memory help?* —
which is answerable only across time, on a moving graph, against labels naming what an answer should draw
on. That is the artifact the corpus already is. The companion instrument that holds the memory state fixed
(the task-outcome A/B design, DiVoid **#11092**) answers the other question and is **structurally blind to
this one** — which is why this cross-time machinery is kept rather than retired as the A/B takes over the
headline.

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
| **A graph snapshot, to hold the memory state constant** *(revision 7 — replaces §11.2's original rejection)* | **A snapshot you can query is a different retriever.** Holding the memory state constant means holding node bodies **and the ranking over them** constant. §11.1 records C22's measurement that DiVoid's ranking is bit-stable for a fixed `(graph state, query)`; a reimplementation carries no such guarantee. Copy the nodes into any other store and the embedding and the similarity ordering are that store's, not DiVoid's — **so a snapshot does not hold the memory state constant, it replaces it**, and anything measured against it is measuring a retriever this system does not ship. This argument does not depend on the problem having bitten, which is why it replaces the cost argument rather than joining it |

---

## 12. Risks and mitigations

| # | Risk | Mitigation | Falsifier |
|---|---|---|---|
| E1 | **The labeller's definition of *required* silently drifts** across rows, so the metric measures the labeller | the definition is written (§4.3) and each row carries a `why` that can be relitigated | read ten `why` lines cold; if two use *required* in incompatible senses, the definition needs sharpening before the corpus grows |
| E2 | **The corpus is too small to resolve the moves M3 makes** | §6.3 states the resolution in the document and prints denominators | a tuning change moves the number by less than ~0.12 |
| E3 | **Graph drift makes two sweeps incomparable** | stale/unresolved counts for the corpus's own referents; §11.2 states the residual honestly | a comparison reverses on re-run days later |
| E4 | **Self-produced content dominates and the number says nothing about retrieval** | reported by name per sweep (§8.2), not hidden; §11.3 makes it M3's first decision | `selfProducedCandidates` exceeds ~25% of candidate slots, or shutouts exceed ~10% of rows |
| E5 | **The harness reports plausible numbers while broken** | four independent guards, one running every sweep (§9.2) | the control stratum's **retrieved** rate reads below 1.00. *(Revision 6: as first written this row said "the control stratum reads below 1.0", which the admitted rate can do for a reason that is neither the harness nor the graph — §9.2)* |
| E6 | **The eval drifts from the loop** as M3 changes assembly | behavioural test (G-14) plus two mutations on the loop's own constants and admission rule (§9.3, G-14b) | a `loop` constant changes and no eval test notices |
| E7 | **The anchor duplication (§3.3) distorts every number** | rejected at load where it would be a free hit (§6.2); reported per row where it is budget cost (§8.2) | `anchorWasCandidate` is true on a majority of rows and the admitted byte totals are correspondingly inflated |
| E8 | **The admitted rate carries permanent zeros nothing can move**, from required nodes larger than the budget (§5.4), and a reader attributes them to the retriever | the node's own `size` is printed beside every `cut`, against the budget on the same line | a `cut` line appears without the required node's size, or the admitted rate is quoted in a comparison without the count of oversized required nodes behind it |
| E9 | **A reader treats exit 0 as "the numbers are good"** now that a budget collapse no longer exits 1 (§9.2) | §7.4 states what the code means; the budget alarm is printed in words, and in the same sweep the labelled admitted rate is visibly near zero | someone cites a sweep's exit 0 as evidence about the pipeline rather than about the sweep |
| E10 | **A disappointing score is resolved by editing the answer key** *(revision 7)*. §4.3 makes *"update the corpus"* a legitimate outcome of a labelled miss — which is right, and is also the path of least resistance: every individual edit looks reasonable and the erosion is invisible | §6.5's three parts, all of which already exist: §6.2 re-validates an edited row exactly like a new one, git carries the date and the reason, and `corpusHash` breaks the comparison chain at the point of the edit. **No new mechanism** — building one is #11034 P-3's bad trade, and E1 already covers the definition drifting rather than the rows moving | **a corpus edit lands in the same commit as anything else**, or a comparison is quoted across two differing `corpusHash` values without the corpus diff beside it |

---

## 13. Coverage — the guard, not the mechanism

Per **#1220 §5 addendum** (origin: my own #10904 §9, corrected after QA #10918 W-6), every row names a
**test**, not a described mechanism, and the falsifier for the table itself is stated:

> **Any row whose named guard would still pass against an implementation lacking the claimed property.**

Two rows below fail that question by construction and are therefore split into a behavioural test **plus**
a structural grep falsifier, rather than being left as a name that reads like evidence.

**Citation format: a name and a file, never a line number.** A test name is stable under edits and is
**mechanically resolvable** — check 1 below diffs every name in this document against the tree in one
command, and a file path is verifiable the same way. A line number is neither: nothing mechanical resolves
it, and it rots on every edit made *above* the cited line, in a file this document does not track. Revision 4
cited a guard one line off from where its function actually sits, and all four checks passed it, because they
verify names and paths and cannot verify lines. (The instance is in §16; it is not repeated here, so that a
residual-citation sweep over this section stays clean.)

The rule that follows is the general one, and it is why the fix is the format rather than the digit:

> **Do not mix a checked claim with an unchecked one in the same citation.** A row that is half-verified
> reads as fully verified — the verified half lends its credibility to the half nobody can check. Either
> every part of a citation is resolvable by the checks the document runs, or the unresolvable part comes out.

Line numbers therefore appear nowhere in this table. Where one would have pointed — `internal/eval/corpus.go`'s
`maxRequiredPerRow`, for instance — the row names the **identifier** instead, which greps.

**Revision 6 adds two rows for guards the tree does not yet carry**, G-28 and G-29, and ~~check 1's `comm -23`
will list both until they land~~ *(revision 7: **false, and it was checked this time.** Both rows cite
`TestControlIntactIsFalseWhenAControlRowDidNotAdmitEveryRequiredNode` — the test they retarget — which exists
today, so check 1 resolves them and never listed either. A claim about what a check would output, written into
the section about falsifiable claims, by an author who did not run it.)* ~~**That residue is this revision's
to-do list, not a defect**; every other name in the table resolves today.~~

**Revision 7 adds six rows, and states the residue as a measurement rather than as a count of rows, because
the two are different numbers:**

| Rows | What check 1 does with them | Status, measured on this branch 2026-09-03 |
|---|---|---|
| G-28, G-29 | **not in the residue** — both cite an existing test by name | **owed to the tree.** No implementation exists; **#11048** is the task |
| G-30, G-30b, G-31, G-31b | **the entire residue: five names** | **written, and in flight on another branch.** The four rows' five tests and the two loader guards exist on `origin/fix/corpus-loader-guards` (tip `dd46f65`), and `git merge-base --is-ancestor origin/fix/corpus-loader-guards origin/main` reports **not merged** — so they are absent from `main` and from this branch. They land with that branch; **nobody should re-implement them** |
| G-32, G-33 | **not in the residue** — neither cites a test name yet | **owed to the tree.** No implementation exists; **#11066** is the task |

**Check 1's residue on this document at revision 7 is exactly those five names, and the check was run
(2026-09-03).** ~~Six rows are owed to the tree and only four of them are visible to check 1 at all — which
is the finding rather than the bookkeeping.~~

**The population, stated — because this section used one phrase for three different ones and got the count
wrong twice** *(corrected after QA #11099 CF-1; §16 carried the same sentence with the numerator inverted)*.
Count **rows in the table below whose guard does not exist on this branch**: there are **eight** — G-28,
G-29, G-30, G-30b, G-31, G-31b, G-32, G-33. That is deliberately **not** the status table's *"owed to the
tree"*, which is the narrower **four** that no branch carries at all; the four on
`fix/corpus-loader-guards` are absent from here and owed to nobody. Both counts are true and they are
different populations, which is exactly how the wrong number got written.

| Of the **eight** rows whose guard is absent from this branch | Count | Which, and why |
|---|---|---|
| **visible** to check 1 — the row cites a name that does not resolve | **4** | G-30, G-30b, G-31, G-31b — five names between them, because G-30b cites two |
| **invisible** to check 1 | **4** | **Two mechanisms, and they are not equally bad.** *Cites nothing, so nothing can fail:* **G-29, G-32, G-33**. *Cites a **different** existing test, so check 1 resolves it and passes:* **G-28** alone, which names `TestControlIntactIsFalseWhenAControlRowDidNotAdmitEveryRequiredNode` — the test it retargets. ~~G-32 and G-33 cite no test name at all; G-28 and G-29 cite the existing test they retarget~~ *(corrected after QA #11102 W-5: G-29 cites no test either — its cell names G-28, not a test. The counts are unaffected)* |

**Half of them are invisible, and that is the finding rather than the bookkeeping.** P-41 checks that every
cited name resolves; **it cannot check that a row owing a guard has cited a name for it** — a row citing
nothing has nothing to fail on, and a row citing a *different, existing* test passes.

**The second mechanism is the worse one, and it was measured rather than argued** (QA #11102, on G-28): the
cited name is in the cited set, resolves in the present set, and occurs in the `comm -23` residue **zero**
times. So check 1 does not merely stay quiet about a row whose guard does not exist — **it passes it**. A
silent absence is a gap; a silent pass is a wrong answer, and exactly one row in this table has that shape.

That is the audit gap
revision 6 named two paragraphs down, arriving from the other direction: revision 6 found a *property* with
no row, and revision 7 finds *rows* whose guard no name reaches. The status table above is what stands in
for it until the guards land.

**That residue is a to-do list, not a defect**; every other name in the table resolves today. It is stated
here because an unexplained residue is what trains a reader to stop running check 1 — this section's own
failure mode, one level up. **And the statuses must not be collapsed:** a reader who takes all six owed rows
as *unwritten* would file #11048's and #11066's work correctly and would then also re-write four rows whose
guards already exist, instead of merging the branch that clears them.

**On the ordering of the ids below.** G-28 and G-29 sit before G-27, because revision 6 inserted them where
the property they guard is discussed rather than at the end. Left as it is on #11034's own rule that **an id
is a handle, not a position**; renumbering to restore visual order would break every citation that already
names one, which is the cost the handle rule exists to avoid.

**And a gap this revision found by looking for the row that should have caught #11045: there was none.** The
control stratum's exit-code behaviour was pinned by six tests across three packages and appeared in **zero**
coverage rows, so the *name the guard, not the mechanism* discipline never reached the one property a caller
actually consumes. **P-41 checks that every cited name resolves; nothing checks that every guarded property is
cited**, and those are not the same audit. G-28 and G-29 close it here; the general lesson is in §16.

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
| G-14b | **Structural half of §9.3** — a correct reimplementation would pass G-14, so this is not a test. **Revision 1's grep cannot come back clean:** it forbade "any numeric literal equal to a `loop` limit", while §4.3 of this same document mandates a required-node cap of **3**, which equals `loop.MaxModelCalls`. `internal/eval/corpus.go` — `const maxRequiredPerRow = 3` — violates it on a fully compliant implementation | **Two mutations, not a grep.** (a) Mutate `loop.AssemblyByteBudget`: the sweep's admitted counts **and** its reported limits must follow — a re-typed value does not. (b) Mutate `Assemble`'s admission from stop-not-skip to skip: `admittedCount` must follow — a reimplementation does not. The grep survives only in its checkable form: **every limit the eval reports resolves through the exported constant**, no re-typed limit *value* |
| G-15 | A corpus row demanding a node the query cannot surface is reported as a miss | `TestSweepReportsAMissForARequiredNodeTheQueryCannotSurface` |
| G-16 | `shutout` is reported when candidates were retrieved and none admitted | `TestSweepReportsAShutoutWhenAnOversizedRankOneCandidateAdmitsNothing` |
| G-17 | Whether the anchor also appeared as a candidate, and whether it was admitted, is recorded per row | `TestSweepRecordsThatTheAnchorAlsoAppearedAmongTheCandidates` |
| G-18 | Admitted bytes and the budget are both recorded, so utilisation is derivable | `TestSweepRecordsTheAdmittedByteTotalBesideTheBudget` |
| G-19 | A candidate written by the loop's own write path is counted as self-produced; a foreign `session-log` is not | `TestSweepCountsOnlyRunRecordsAsSelfProducedAndNotOtherSessionLogs` |
| G-20 | The result carries the corpus hash and the loop's limits, so a result names what produced it | `TestResultHeaderCarriesTheCorpusHashAndTheLoopLimits` |
| G-21 | Result rows are emitted in corpus order, so two results diff | `TestResultRowsAreEmittedInCorpusOrder` |
| G-22a | **`Render`'s parameter contract** — the result goes to its machine writer, the summary to its human writer. **Premise:** this sits one call-layer *below* the stream binding and structurally cannot observe it — `go list -deps ./internal/eval` contains no `cmd/eval` | `TestRenderWritesTheResultToItsMachineWriterAndTheSummaryToItsHumanWriter` (`internal/eval/report_test.go`) |
| G-22b | **The binding is not swapped.** `run()` passes stdout as machine and stderr as human, so `> result.json` captures the JSON and not the summary | `TestRunWritesTheMachineResultToItsFirstStreamAndTheHumanSummaryToItsSecond` (`cmd/eval/main_test.go`). **Falsifier:** swap the two arguments at the `Render` call. Observed surviving green before this test existed |
| G-22c | **The log never contaminates the machine stream.** A logger bound to the machine writer interleaves text into the JSON on the happy path | `TestRunKeepsItsLogOutOfTheMachineStreamWhenAStepBeforeTheSweepFails` (`cmd/eval/main_test.go`). **Falsifier:** `slog.NewTextHandler(machine, nil)` |
| G-22d | **A render failure exits non-zero**, rather than exiting 0 having emitted no measurement — §9.2's own failure mode at the output boundary | `TestRunExitsNonZeroWhenTheMeasurementCannotBeWritten` (`cmd/eval/main_test.go`). **Falsifier:** weaken the error check to `err != nil && false`. Observed surviving green before this test existed |
| G-23 | The graph boot half loads with **no** model variable present | `TestGraphBootConfigLoadsWhenNoModelVariableIsSet` |
| G-24 | The model boot half still errors when a required model variable is absent | `TestModelBootConfigErrorsWhenTheModelUrlIsAbsent` |
| G-25a | A secret never appears in the **graph** half's boot error. **Premise that makes it discriminate:** after the split no single loader reads both a secret and the member that fails, so the pre-split scenario is vacuous; this is re-pinned on the co-located pair — `PROCESSOR_DIVOID_KEY` present, `PROCESSOR_DIVOID_URL` empty | `TestBootConfigErrorsNameTheVariableAndNeverItsValue`, graph scenario |
| G-25b | A secret never appears in the **model** half's boot error — `PROCESSOR_MODEL_KEY` present, `PROCESSOR_MODEL_ID` empty. **Premise:** the split created a second secret in a second loader, so this half had no guard before it | `TestBootConfigErrorsNameTheVariableAndNeverItsValue`, model scenario |
| G-26a | **§1 S1 — no model call is reachable.** Stronger than any grep, because it is a fact about the binary rather than about spellings | **Linker falsifier:** `go list -deps ./cmd/eval` does not contain `internal/openaicompat`; `go list -deps ./cmd/processor` does. Confirmed by QA #10945 ruling B |
| G-26b | **§9.1 — the instrument never writes.** A grep cannot pin this: consuming `loop.GraphPort` forces every double to implement `WriteRun`, so a literal grep hits test sources on compliant code | `fakeGraph.WriteRun` calls `t.Fatal`, observed red by inserting one `WriteRun` into `sweepRow`. The grep remains as a backstop **scoped to non-test sources** |
| G-28 | **A control node that was retrieved and cut does not fail the sweep** — it is a budget alarm, exit 0 (§9.2). **Premise that makes it discriminate:** the fixture's control row is `cut`, never `notRetrieved`, since a fixture conflating the two is passed by an implementation that reads either as a failure | **Guard required by revision 6; not yet in the tree.** It replaces `TestControlIntactIsFalseWhenAControlRowDidNotAdmitEveryRequiredNode`, which pins the semantics this revision overturns and is retargeted rather than deleted. **Falsifier:** restore the `Verdict != Admitted` condition; the guard must redden |
| G-29 | **A control node that was never retrieved fails the sweep** — exit 1 (§9.2). **Premise:** paired with G-28 over otherwise identical fixtures, so neither passes an implementation that collapses the two verdicts | **Guard required by revision 6; not yet in the tree.** **Falsifier:** widen the condition to accept `notRetrieved`; the guard must redden. It runs as a pair with G-28 — either one alone is satisfied by a constant |
| G-27 | **`main()` binds the real streams in the right order.** `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))` is one line no in-process test can reach — `main()` is not callable and the real file descriptors are not substitutable from inside the process | **Grep falsifier, admissible here:** exactly one hit for `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))` in non-test source, and **zero** for the swapped spelling. See the admissibility note below |
| G-30 | A `required[].hash` that is not 64 lowercase hex is rejected at load. **Premise that makes it discriminate:** the five rejected arms are *4 characters*, *outside the alphabet*, *uppercase*, *63 characters* and *a 128-character lowercase sha512 digest* — an implementation checking only non-emptiness passes none of them, one checking only length passes neither the alphabet nor the case arm, and one checking `len < 64` rather than `len != 64` passes the sha512 arm alone. That last arm was added after QA #11057 CF-1 found the `<` mutation surviving the whole suite green: the boundary needs both sides, and a sha512 digest is the most likely wrong-hash-function paste | `TestLoadRejectsARequiredHashThatIsNotLowercaseSha256Hex` (`internal/eval/corpus_test.go`) — **on `origin/fix/corpus-loader-guards`, not on this branch** |
| G-30b | **The dual: a legitimate hash still loads.** Uppercase is excluded on purpose — `assemble.go`'s content hash is lowercase-only and `score.go` compares with plain equality, so an uppercase hash is guaranteed permanently stale — and the accepted set is therefore argued from what the **format** permits rather than from what the corpus writes today: an all-digit hash, an all-letter hash, and **the content hash `loop.Assemble` itself produces**, fed back in through the loader. The last of those pins the eval guard's premise to `loop.contentHash` rather than to eval's own encoder, which QA #11057 W1 identified as agreeing only by coincidence; mutating `loop.contentHash` to uppercase, or to a 128-character digest, reddens it | `TestLoadAcceptsAnAllDigitAndAnAllLetterSha256Hash`, `TestLoadAcceptsAsARequiredHashTheContentHashAssembleProduces` (`internal/eval/corpus_test.go`) — **same branch caveat as G-30** |
| G-31 | The same node id twice inside one row's `required` is rejected at load | `TestLoadRejectsARowListingTheSameRequiredNodeTwice` (`internal/eval/corpus_test.go`) — **same branch caveat as G-30** |
| G-31b | **The dual: three distinct required nodes still load**, so the guard cannot be satisfied by refusing every second entry. **Premise, verified by QA #11057:** `TestLoadRejectsARowListingTheSameRequiredNodeTwice` does *not* redden under `if len(required) > 0` — the mutant still rejects the duplicate row with the same message — so the rejection test cannot distinguish *rejects a repeated node* from *rejects any second entry*, and this dual is the sole discriminator. **Falsifier:** weaken the check to `if len(required) > 0`; observed reddening this row and nothing else | `TestLoadAcceptsThreeDistinctRequiredNodesOnOneRow` (`internal/eval/corpus_test.go`) — **same branch caveat as G-30** |
| G-32 | The row result carries **every** candidate the sweep produced, in **rank** order (§8.2). **Premise that makes it discriminate:** the fixture's candidates must differ from each other in id *and* in similarity, and must include at least one cut candidate, or an implementation retaining only the admitted prefix passes, and one re-sorting by similarity rather than preserving the rank the graph returned passes too | **Guard required by revision 7; not yet in the tree** — **#11066** is the task. **Falsifier:** retain `dispositions[:admittedCount]`; the guard must redden |
| G-33 | A miss line names the three highest-ranked candidates, and a miss with fewer than three above it prints what exists rather than padding (§8.3 rule 4). **Premise:** the fixture carries one miss with three or more outranking candidates and one with exactly two, so an implementation that always prints three — padding with a zero-valued entry — fails the second, and one that prints by insertion order rather than by rank fails the first | **Guard required by revision 7; not yet in the tree** — **#11066** is the task. **Falsifier:** print `candidates[:3]` unconditionally; the guard must redden on the two-candidate row |

Applying the table's own falsifier to the rows most at risk: G-4's empty-corpus half fails against code that
returns `0/0` and exits 0 — the test asserts an error, so it discriminates. G-9 fails against code that
never reads hashes — the fixture's hash differs, so it discriminates. G-12 fails against a truncating
reporter **only because** the fixture carries twelve misses; that premise is stated in the row rather than
left for the reader to re-derive. G-14 and G-26 do **not** discriminate for the structural property they
appear to claim, which is why each is split.

**Revision 3 ran the table's falsifier over the falsifiers themselves and found two of a second, distinct
kind** — revision 1's G-26 grep and revision 1's G-14b grep. Both are replaced above. The shape deserves its
own statement, because #1220 §5 names the opposite defect:

> A row that **cannot discriminate** is a wish. A row that **fires on a compliant implementation** is worse
> than absent, because a reader who runs it, sees the hit and finds the code correct learns to disregard the
> whole column. Both are unfalsifiable; only the second is actively misleading.

Both instances were greps, and both failed the same way: **a string search can express a property about
spellings, never about calls or values.** Where the property is *this call never happens*, the guard is a
fatal in a double; where it is *this value is not re-typed*, the guard is a mutation on the constant. Neither
is a pattern, and reaching for one is the tell. The second instance is the sharper warning of the two — it
was **self-inflicted across sections**: §4.3 mandates a required-node cap of three and G-14b forbade the
literal `3`, so the document required and prohibited the same constant without either section noticing.

### Why a grep is admissible for G-27 and was not for G-14b or G-26b

The distinction is not "greps are bad". It is **what kind of property a string search can express**:

| Property | Expressible by a pattern? |
|---|---|
| *this call never happens* (G-26b) | **No** — a call site can be spelled many ways, and the interface forces the identifier into every double |
| *this value is not re-typed* (G-14b) | **No** — the digit is legal elsewhere, and §4.3 mandates one instance of it |
| *these identifiers appear in this one call expression* (G-27) | **Yes** — the property genuinely is about the spelling of a single line |

G-27 also passes both later checks where the revoked two failed: exactly one hit on compliant code, zero on
the swapped variant, and **the swap was observed surviving the full suite green**, so it is falsifiable and
has been seen to fail. It is admissible and proportionate to one unreachable line.

**Proportionate — with an expiry.** The repo already owns the better instrument: `cmd/processor` is observed
from outside by a process-boundary harness (`process_linux_test.go`). **The grep is owed that harness the
moment `main()` grows a second line.** Recorded as a trigger, because a proportionality argument that is not
written down expires silently and leaves a pattern standing in place of a test.

### The falsifier is four checks, not one — and two of them are commands

Revision 3 ran the falsifier over the whole column, replaced two rows, canonised the rule above — and left
**G-22 naming a test that did not exist**, on the very row the round's critical fail was about. The sweep
asked *would this guard fire wrongly?* and never asked *is this guard there at all?*

The falsifier is therefore four checks, not one, and **the two added here are mechanical**:

| # | Check | How |
|---|---|---|
| 1 | **Does the named test exist, and in the file cited?** | `sed '/^## 16\./,$d'` this document, extract every backticked `Test*`, diff against `grep -rh '^func Test' --include='*_test.go'`; then resolve each name to its file and compare with the path in the row. **Both halves are one command each, and the first is the check that would have caught CF-A.** The `sed` is load-bearing, not tidiness: §16 quotes dead names as *history*, and a check run over the whole file fires on the revision log — which would make check 1 itself a falsifier that fires on a compliant document, the defect §13 revokes rows for. **A name before §16 is a claim; a name inside §16 is a record** |
| 2 | **Can the guard's package observe the property?** | `go list -deps <guard package>` must contain the package holding the property. `internal/eval`'s closure contains no `cmd/eval` |
| 3 | Would it pass against an implementation lacking the property? | revision 1's original |
| 4 | Does it fire on a compliant implementation? | revision 3's addition |

**Check 2 is the one that reframes the G-22 split.** The measurement said the old guard appeared in none of
the red sets; the reason is stronger than that and is categorical. No mutation of `cmd/eval/main.go` *could*
redden an `internal/eval` test, because `cmd/eval` is not in that package's dependency closure. The old guard
sat in a package **structurally incapable of observing the property it was named for**. Widening it was never
available — **the split was forced, not preferred**, and a row can be a wish for reasons of topology and not
only of wording.

**And the meta-lesson, which is the one that generalises.** A verification pass finds the defect shape it is
looking for. Revision 3 was looking for guards that fire wrongly and swept past a guard that was not there,
on a row it read three times. **A checklist of checks beats a sharper eye**, which is why the four above are
written down as a procedure rather than left as judgement — and why two of them are commands rather than
questions.

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
| Out-of-scope listed explicitly | ✓ §2, ~~eight rows, each with a trigger~~ **nine rows, each with a trigger or a stated reason** *(revision 7 added the ninth — the task-outcome A/B — and broke both halves of the old claim at once: the count, and the qualifier, because that row carries **gates** (#11066 plus a settled task set) rather than a trigger. It points at designed work, not deferred work)* |
| No multi-paragraph rationale for things that obviously stay | ✓ |
| Predecessor designs banner-marked where superseded | ✓ **Nothing is superseded.** #10532 and #10904 are consumed and extended; §16 lists the ~~two~~ places this document *sharpens* them without overriding any claim *(revision 7: the table has three rows, not two)*. **And this document is not superseded either.** The task-outcome A/B design (DiVoid **#11092**) is a **successor, not a supersession**: it changes which number is the headline and leaves every mechanism here canonical and in force. #1136 §5's banner rule binds an **end-to-end** supersession, so it does not fire — what is owed instead is a forward pointer, which the header carries. §16 argues the split |

---

## 15. Implementation order

Five steps. Each is independently reviewable and each ends with something observable.

| # | Step | Ends when |
|---|---|---|
| 1 | **Move the boot config** to `internal/boot` and split it into the three loaders of §7.3, over the existing helpers. `cmd/processor` calls all three, in the order address → graph → model. | **Every assertion of the pre-split config tests survives**, with the same environments and the same expected strings; plus G-23, G-24, G-25a, G-25b. Retargeting and renaming a test to its new call surface does not violate this; deleting or weakening an assertion does |
| 2 | **Export `RunNodeType` and `RunNamePrefix`** from `internal/divoid`. | The write path uses the exported names; no literal survives |
| 3 | **`internal/eval` — corpus and scorer.** The row type, the loader with §6.2's validation, and the pure scorer. **No graph access in this step at all.** | G-1..G-8 |
| 4 | **`internal/eval` — reporter**, over hand-built results. | G-11, G-12, G-13, G-16..G-21, **G-22a**. G-22b/c/d cannot land here — they are about `cmd/eval`'s stream binding, which this package's dependency closure cannot reach |
| 5 | **`cmd/eval` — the sweep.** Boot, load, per-row `Node` → `Recall` → `Assemble` → score, stale/unresolved resolution, render. | G-9, G-10, G-14, G-15, **G-22b, G-22c, G-22d**; G-26a's linker check excludes `internal/openaicompat`; G-26b's fatal fake and G-14b's two mutations each observed **red**; G-27's grep returns one hit and zero on the swap |

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

### Revision 3 — 2026-09-02, after steps 3–5 were implemented and reviewed (#10945)

The implementation was found faithful to the design (48 of 51 independent mutations red on the named guard).
Four corrections, and **three of the four are the same defect in different clothes**: a guard the document
described in prose that the implementation could not satisfy as written.

| Correction | Where it was wrong | Now |
|---|---|---|
| **§9.1's grep was unsatisfiable** | consuming `loop.GraphPort` forces every double to implement `WriteRun`, so the grep hits test sources on compliant code | §9.1 and **G-26b**: the fake's `WriteRun` calls `t.Fatal` — proving the *call* never happens, not that a *string* is absent. Grep demoted to a non-test-scoped backstop |
| **§13 G-14b's grep was self-colliding** | it forbade any numeric literal equal to a `loop` limit, while §4.3 of this document mandates a required-node cap of **3** = `loop.MaxModelCalls` | **two mutations** on `AssemblyByteBudget` and on the admission rule. The grep survives only as *no re-typed limit value* |
| **§1 S1's grep was weaker than a free fact** | it searched for `ModelPort` / `openaicompat` / POST, and carried a dangling `§13 F-1` reference to a row that never existed | **G-26a**: `go list -deps ./cmd/eval` excludes `internal/openaicompat`. The model adapter is not linked, so the call is unreachable rather than merely unwritten |
| **§8.2's field table was missing `topSimilarity`** | it appeared only in §8.3's specimen — a seam between two sections of one document | a row with its reader named, and the delete-test sentence extended from five diagnostics to six |

**The lesson revision 3 adds**, and it is a different one from revision 2's: revision 2 found a row that
*described a mechanism*; revision 3 found rows that *named a check which fires on correct code*. The first
teaches a reader nothing; the second teaches them to ignore the column. A grep was the instrument in all
three, and the tell is uniform — **a string search expresses properties about spellings, and every one of
these properties was about a call or a value.** Where the document reaches for a pattern to pin behaviour,
it is reaching for the wrong instrument.

Two further things this round settled and folded in rather than leaving in a PR body: **§8.2** now states why
the result reports two limits instead of `loop.Limits`' five, and **§9.3** now settles the collision between
#10466's *literals on the expected side* and G-14b's *reference the constant* by assigning each to the
package that owns the property.

### Revision 4 — 2026-09-02, after CF-1 was closed and the ledger re-audited (#10952)

**The critical fail this round was in this document, not in the code**, and it was on a row revision 3 read
three times while sweeping the column it sits in. Four corrections, all in §13 and §15.

| Correction | Where it was wrong | Now |
|---|---|---|
| **§13 G-22 named a test that does not exist** | the test was renamed when CF-1 was closed; a repo-wide grep for `TestSweepWritesTheResultToStdoutAndTheSummaryToStderr` returned exactly one hit — the design line itself. Neither the renamed guard nor any of the four new `cmd/eval` guards appeared in the document | split into **G-22a** (`internal/eval`, `Render`'s parameter contract) and **G-22b/c/d** (`cmd/eval`: the binding is not swapped, the log does not contaminate the machine stream, a render failure exits non-zero), every name verified against the tree with its file and line |
| **§15 step 4 assigned G-22 to `internal/eval`** | that package **structurally cannot hold it** — `go list -deps ./internal/eval` contains no `cmd/eval` | G-22a stays in step 4; b/c/d move to step 5, with the topological reason stated in both rows |
| **The `main()` binding was not in the ledger at all** | it was covered by a grep living only in the implementer's report, so §13 — the ledger §9's thesis rests on — did not carry it | **G-27**, with the admissibility argument (this property *is* about the spelling of one line) and an expiry trigger: it is owed the process-boundary harness the moment `main()` grows a second line |
| **§13's falsifier was one check where it needed four** | it asked *would this guard fire wrongly?* and never *is this guard there at all?* | four checks, **two of them mechanical commands** rather than judgement — existence, observability, discrimination, false-firing |

**Why this one is worse than revisions 2 and 3, stated plainly.** Those found rows whose wording was weak.
This found a row that pointed at nothing — the failure mode "name the guard, not the mechanism" exists to
prevent, occurring inside the table that rule built. **An unverified test name is a mechanism description
wearing a test's clothes**: it reads as evidence, it is checkable in principle, and nobody checked it. The
name is the entire value of the row, and a name nobody resolved is worth less than the prose it replaced,
because prose does not claim to be falsifiable.

**And the transferable half:** a verification pass finds the defect shape it is hunting. Revision 3 hunted
guards that fire wrongly and swept past a guard that was absent. The remedy is not more care — it is that
checks 1 and 2 are now **commands you run**, not questions you remember to ask.

### Revision 5 — 2026-09-02, after the ledger was approved with warnings (#10961)

**No critical fail.** One warning, and the fix taken is the **format** rather than the instance.

| Correction | Where it was wrong | Now |
|---|---|---|
| **§13 cited `report_test.go:245` for a function at `:246`** | the same class as revision 4's CF-A at a fraction of the severity, and with the property that makes the class dangerous: **check 1 verifies names and paths, never lines**, so nothing mechanical would ever catch it and it rots on every edit above the cited line | **line numbers dropped from every row.** §13 states why: a name and a path are mechanically resolvable, a line is not, and mixing a checked claim with an unchecked one in one citation makes the unchecked half read as verified |
| **Check 1 verified names but not paths** | this round verified all five cited paths by hand and found them correct — but the *procedure* did not include it, so the next round would not have | check 1 now resolves each name to its file and compares it against the row |
| **§13 and §9.1 cited mutation-matrix indices (M41, M44/M45, M46) as evidence** | the matrix is a point-in-time measurement of one round and exists in neither the graph nor the repo, so a reader cannot resolve the citation | replaced with the **mutation each row owns, described**: *insert one `WriteRun` into `sweepRow`*, *swap the two arguments at the `Render` call*, *weaken the error check to `err != nil && false`*. Reproducible by anyone, forever |

**All five cited file paths were verified correct** — only the line number was wrong. So the format change is
preventive rather than a second repair.

**The lesson, and it is the same one revision 4 reached from the other side.** Revision 4 made two checks
mechanical because judgement had missed a defect. Revision 5 removes a claim **because no check can reach
it** — the complement of the same rule. A document that runs checks over itself must not carry assertions
outside their range: an unverifiable claim sitting beside verified ones does not merely fail to help, it
**borrows credibility from its neighbours**. Drop it, or bring it in range — here, by naming the identifier
instead of the line.

The mutation-index correction is the same shape a third time: **a citation is only worth what a reader can
resolve.** `M44` resolves against a document that was never durable; *"swap the two arguments"* resolves
against the code.

### Revision 6 — 2026-09-03, after the first corpus was authored and before any sweep was run (#11045, #11046)

**No implementation round produced this one.** Both findings came from **authoring against the schema rather
than reading it** — a third way of testing a design, and the only one that had not been tried: §13's guards
test the code, the mutation round tests the guards, and writing eleven real rows tested the *document*.

| Correction | Where it was wrong | Now |
|---|---|---|
| **§9.2 guard 3 read the control stratum at one boundary** | it required `admitted`, so under stop-not-skip a single 70,660 B run record at rank 1 cuts every control and the sweep exits 1 reporting *"the graph moved or the harness broke"* — when the cause is the self-recall pollution §3.4 measured. **The stratum reported the one thing it was built to rule out** | split at the two boundaries §5 already drew: `notRetrieved` and `unresolved` exit **1**, `cut` exits **0** as a named budget alarm. §4.2, §7.4, §8.3, §12 E5 and §13 G-28/G-29 follow |
| **§12 E5's falsifier was *"the control stratum reads below 1.0"*** | it does not say **which rate**, and the admitted rate can read below 1.0 for a reason that is neither the harness nor the graph | *"the control stratum's **retrieved** rate reads below 1.00"* |
| **Nothing said a required node can be larger than the budget** | #10926 is 95,661 B against 60,000 B, so `r09` contributes a permanent zero to the admitted rate for a reason unrelated to retrieval, and neither §6.2 nor the loader mentions it | **§5.4**: not rejected at load, not a third stratum — the node's own `size` ships beside every `cut` (§8.2, §8.3), with §12 E8 carrying the residual risk |
| **The property that drives the exit code had no coverage row** | six tests pinned it across three packages and §13 cited none of them, so the *name the guard* discipline never reached the one behaviour a caller consumes | **G-28 / G-29**, both marked as owed to the tree, with the check-1 residue explained in §13's preamble |

**The lesson, and it is a new shape.** Revisions 2–5 all found rows whose **wording** was weak — a mechanism
described as a guard, a check that fires on compliant code, a name that resolved to nothing, a line number
nothing could verify. Revision 6 found something else: **two sections that were each correct and that
contradicted each other.** §11.3 rules that a defect faithfully measured is a correct result; §9.2 guard 3
ruled that the same event on a control row is an instrument failure. Both were written in the same pass, both
survived four revisions and a mutation round, and **no check in this document or in #11034 can find that** —
every one of them resolves a claim against the tree, and here the tree matched the claim. What found it was a
reader with a use for the number.

> **A document can be locally true everywhere and globally inconsistent.** Checks that resolve each claim
> against the code cannot see it, because each claim resolves. The instrument for it is a reader who needs the
> answer — which is the argument for authoring the corpus *before* the first sweep rather than after, and the
> second time that ordering has paid (the first was §3.4, which fired R13 before a sweep existed).

**What this revision deliberately does not change.** R13 stays unfixed and unscheduled: #10822's *"do not
build it before the measurement"* and §11.3's ordering claim both survive, and the split is what lets them —
the first sweep can now measure the pollution instead of aborting on it. The corpus is unchanged in shape,
`r09` stays, and the two rates keep their definitions.

**One consequence of amending this document at all, which is worth a line because it recurs.** `r09`'s
required node **is this document**, and the hash on that row is this file's. Every revision therefore rots
that label, and the sweep reports `stale` for a reason that has nothing to do with graph drift — noise on the
exact channel §6.5 built to detect it. **The rule: a corpus row requiring a repo-backed design document is
re-hashed in the same change that revises the document.** This document now has three representations that
must move together — the repo file, the DiVoid node (#11034 P-40), and any corpus hash pointing at it.

### Revision 7 — 2026-09-03, folding three pending amendments and ruling on a fourth (#11052, #11065, #11066, #11069)

**Four pending edits had accumulated against a blocked file.** Three were amendments to this document's own
claims and are folded in below. The fourth — #11069, carrying Toni's reframe of what the milestone should
measure — is **not** folded in, and the ruling against folding it is the substance of this revision.

**One thing about the change that carries this, because a reader diffing the repo will meet it and the
branch name does not say so** *(recorded after QA #11099 §4)*. **The commit lands revisions 6 and 7
together**, not revision 7 alone: the repo file's §16 ended at *Revision 5*, because revision 6 lived only
on node #10926 and the repo file was the half that was behind. So revision 6 appears and is amended inside
one commit. That is not a scope defect — it is the P-40/P-50 parity gap #11057 ruled on, closing — and
P-49's binding applies to the pair, not to revision 7 alone.

| Correction | Where it was wrong or silent | Now |
|---|---|---|
| **§6.2 stated only that a `hash` is non-empty** (#11052) | a malformed hash *loads*, and then reads as `stale: true` on every sweep — saying *the graph moved under a good label* where the truth is *the label was never right*. Silent where it could be caught, misleading where it is seen | the rule is 64 lowercase hex, old text struck. **§8.1 needed no edit**: its invariant column already said *"sha256 hex, the same function `assemble.go` uses"*, so §8.1 was the **overstating** section and #11046 offered the choice of weakening it — the code was made true instead |
| **Nothing required `required[].node` to be unique inside a row** (#11052) | uniqueness was stated for row ids and never for required-node ids. A repeat counts **one** label two or three times in both rates — numerator and denominator both move, so the row is silently mis-weighted with no diagnostic anywhere | §6.2 gains the rule; §8.1's invariant gains *unique within the row* |
| **Nothing said what a labelled miss *means*** (#11065) | §4.3 defined *required* and stopped. Three causes — a real regression, a better node appearing, a wrong label — are byte-identical in every field the harness prints, so a red reads as a verdict when it is a finding | §4.3 carries the three causes and Toni's words; §6.5 names which of the two ways a label rots is decidable; §12 E10 carries the gaming risk the permission creates |
| **The evidence needed to tell those causes apart was being discarded** (#11066) | `BuildRow` receives the full `[]loop.Disposition` and keeps only scalars, and `missLine` prints no candidate identity for any verdict. A sweep is a point-in-time reading of a live graph, so the evidence is unrecoverable the moment the sweep ends | §8.2 retains `candidates[]`; §8.3 gains the outranked-by line, the two-candidate specimen, and the ascending-id reading aid as prose |
| **§11.2 rejected a graph snapshot on cost** — *"a problem that has not yet bitten"* (#11069) | a cost argument does not survive a reader who wants a fixed memory state **on purpose**, which is exactly what a task-outcome A/B wants | struck, and replaced in §11.4 by an argument that does not depend on whether the problem has bitten: **a snapshot you can query is a different retriever** |
| **§6.3's growth trigger sized the corpus against a standard error** (#11069) | the arithmetic is fine; the *instruction* rested on this rate being the headline, which it no longer is | the instruction is **struck and suspended**, the arithmetic is kept and is now the reason the rate cannot be quoted as a headline. §17 question 1 and question 3 follow |

### The ruling on #11069: a successor document, and not for the reasons offered

**Ruled: the task-outcome A/B becomes its own design — DiVoid **#11092** — and this document is not
superseded.** Two reasons were put to me and I rest on neither.

- **"This file is too big to hold" is not a reason.** Splitting a document on length, without a boundary that
  means something, produces two documents that must be read together and a relationship the reader now has to
  reconstruct. Size is evidence that a document may be carrying more than one thing; it is not the finding.
- **"It measures a different object" is true and is not decisive.** It would equally justify a §18.

**~~What decides it is P-43, and it decides it cleanly.~~ P-43 excludes #11069's fold-in *as it was asked
for*, and that is the whole of what it excludes** *(re-ordered after QA #11099 W4 — the strike is on the
ordering claim, not on the reasoning under it, which stands and which QA agreed with)*. #11069's own
fold-in table asks for §1 and §2 —
*what the milestone is for*. A design document is a **dated record**: corrected in place with the old text
struck, never retro-edited. But §1 and §2 are the premise the shipped implementation was built against, and
four of §15's five steps have shipped against them. Striking a Problem Statement leaves a document with two,
one of them struck, and **every downstream section then serves an ambiguous premise** — §4.3's argument, §5's
split and §6.3's sizing would each have to be read against *which §1?*. That is precisely the failure revision
6 named — *a document can be locally true everywhere and globally inconsistent* — manufactured deliberately.

**What P-43 does *not* do is choose between a successor artifact and a new §18 here — and the original
ordering of these reasons got that wrong.** A §18 that *adds* the A/B beside §1 rather than striking it
needs no strike at all, so it passes P-43 untouched. **The reason that discriminates between a section and
a separate artifact is the measured one, and it is promoted here from a footnote:**

- **The lifecycles differ, and by P-49 they cannot share a hash.** This design is closed: five steps, four
  shipped and the fifth in flight. The A/B is unbuilt and blocked on two gates. `r09` requires **this
  document** at its content hash — so by the P-49 mechanism **this very revision is an instance of**, every
  future revision of an A/B section would rot that row: the revision-6 problem, on a design that has not
  started changing yet, multiplied by however many revisions it takes to settle. **A closed record and an
  open design cannot share a hash**, and that is the sentence a §18 has no answer to.

One further reason, which stands on its own and is argued rather than measured:

- **It is a project-level claim in a milestone document.** #11069 says the A/B supersedes the rates *as the
  headline*. That is a claim about how this **project** measures itself, not about what M2 delivers. #10924's
  charter is #10424 §9's one sentence and nothing in it is about task outcome.

**Where I rule against #11069's own fold-in table**, which is the part of this ruling that is not a
formality: its targets are not all of one kind, and treating them as one is what made the choice look binary.

| #11069 target | Ruling |
|---|---|
| §1, §2 — the milestone's purpose | **Successor.** These are the retro-edits P-43 forbids. §2 gains a pointer row, which is additive |
| §4.3 — `required[]` as the A/B's precondition | **Successor.** It is a forward citation *from* the new document; this one owes nothing. #11092 §3 quotes §4.3 verbatim rather than restating it |
| §5 — the split retained on its original argument | **Neither.** #11069 says the justification needs no rewriting, so folding in a note that nothing changed is pure restatement (#1136 §2 form 3). **Not written** |
| §6.3 — struck | **Folded in, narrowed.** The arithmetic does not become false; the *prescription* does. Only the prescription is struck |
| §6.5, §11.2 | **Split.** §11.2's re-framing is a correction to a claim this document makes and is folded in. *"`stale` proves two arms saw one graph"* is the successor's use of this mechanism and is stated there |
| §9.1 — no-mutation extended to the loop while measured | **Successor.** §9.1 is about *the instrument*. Extending it to a component this milestone does not build is scope creep; #11092 §4.2 carries it, and #11071 is the task |
| §10 — the A/B's price | **Successor** |
| §11.4 — the snapshot rejection re-argued | **Folded in, and it is the sharpest of the six.** A rejected alternative whose stated reason has stopped holding is exactly what P-51 obliges the noticing hop to correct |

**And it is a node, not a repo file** *(added after QA #11099 CF-2; the artifact decision is the
operator's under #10192, and this records it)*. The successor was written as
`docs/architecture/task-outcome-ab.md` and that file was withdrawn before this change was committed.
**#1176**'s 2026-09-03 ruling: a design that genuinely precedes any code is a DiVoid node, not a repository
PR — and #11092 §10 opens by stating that nothing in it is buildable today, behind two gates, one of which
needs Toni. The failure mode that ruling names is exactly the one available here: *a design merges, the
implementation does not follow, and the document ages into a description of something that was never
built — while reading as current, because merged looks like shipped.* **Nothing was lost by dropping the
path**: this document cited the successor by path at **six** sites — the header, §2's out-of-scope row,
§6.3, §11.2, §14 and §16 — and every one of them already carried #11092 beside the path, so the six
became node-only citations with no information removed. *(QA #11099 CF-2 put the number at five and
named §11.4 among them; §11.4 carried no path citation, and §6.3 and §14 did. The remedy is unchanged
— recorded because a count in a dated record is a claim.)*
**And the withdrawal is scoped to that one new file.** Correcting *this* document in place is P-43 working
as intended, and `r09` is bound to it by P-49 — which is the same asymmetry the ruling itself draws
between a design that precedes code and one whose implementation is four steps in.

### What the A/B cannot measure — and one limit is not enough

Toni named one himself: *"its always a smoke test."* That is the statistical limit and it is the one a larger
sample fixes. ~~**#11092 §5 names five, of which three are structural**~~ **#11092 §5 names five, and only
§5.1 — the smoke-test limit — is statistical; every other one is structural** — no sample size, no rubric
and no care removes them *(corrected after QA #11102 W-7: "three" was wrong here, at two sites inside
#11092, and in #11069. It is the **remediable** count, which is a different cut of the same five)*. The two that neither Toni nor #11069 named, recorded here because they bear on *this* document:

- **The A/B is blind to tasks whose answer no node holds.** Its admission precondition — *"we know that our
  system has all the information to solve it"* — **selects the task set for cases where retrieval can
  succeed.** The population it cannot see is the population a memory substrate exists to shrink.
- **Alone, it is the single-scalar design §1 already rejected.** A task-outcome verdict is one bit over
  retrieval, assembly, prompt, model and tool loop together; it cannot say which stage moved. §1's test —
  *would this instrument change anyone's mind?* — has the same answer it had for a bare recall number.
  **That is the strongest argument that the two layer rather than compete**, and it is this document's own
  test doing the work.

### The blocking defect, and why the A/B design is credible in spite of it

**#11071 is real and the A/B is not runnable until it is answered.** `internal/loop/turn.go:131` calls
`WriteRun` unconditionally *(read first-hand on this branch, 2026-09-03)*, so an A/B of N tasks × 2 arms
writes 2N run records into the state it exists to hold constant — and §3.4 measured those records outranking
real content and exceeding the whole budget. **#11092 §4.2 answers it** with a shape that requires **no change
to `internal/loop`**: the runner supplies a decorating `GraphPort` whose `WriteRun` files nothing and returns
the existing `notStored` receipt. No flag, no second code path through the turn, no addition to a shipped
closed set. #11071 deliberately left the shape to the implementer; choosing between competing shapes is design,
so it is ruled in #11092 rather than deferred — the constraint #11071 states is untouched.

### What #11048 and #11049 build against

Both were told not to start against `main`'s copy of this document. **They now build against this file at
revision 7**, on `design/m2-revision-7`, and neither is changed in substance by this revision:

- **#11048** (the control verdict split) implements §9.2, §7.4, §8.3 rule 3, §12 E5 and §13 **G-28 / G-29** —
  all of which are revision 6's and are untouched here. §13's preamble now states G-28/G-29's status
  explicitly, which is the only delta it sees.
- **#11049** (the required node's `size`) implements §5.4, §8.2 and §8.3 — also revision 6's and untouched.
  The one thing to note is that §8.3's specimen has grown an *outranked by* line under each miss (rule 4,
  #11066's work); #11049 adds the size to the **miss line itself**, not to that new line, and the two are
  independent.
- **#11066** (the retained candidate set) now has its design: §8.2, §8.3 rule 4 and §13 **G-32 / G-33**.

### What could not be verified here, stated because P-51 requires it in the sentence and not in a preamble

- **The container gate was reachable and was run** for the corpus change this revision carries — so, unlike
  the round P-51 was written against, nothing here rests on reasoning from source about a container I could
  not enter. Where a figure is inherited rather than re-measured — the 50,000–110,000 B run-record range, the
  70,660 B of #10897, C22's bit-stability measurement, §10.2's token counts — #11092 labels it as inherited in
  the sentence that uses it.
- **Two things in the brief that produced this revision were wrong against the tree, and are recorded because
  a corrected brief is not the same as a corrected document.** First, #11052 states its two coverage rows are
  *"next free ids G-28 and G-29"* — but revision 6 had already taken both, so they are folded in here as
  **G-30 / G-30b / G-31 / G-31b**; anyone reading #11052 will find the old numbers. Second, #11052 describes
  its loader guards as *shipped*, which is true of `origin/fix/corpus-loader-guards` and **not** of `main` or
  of this branch — verified by `git merge-base --is-ancestor`, and recorded in §13's preamble so the four rows
  are not re-implemented by someone reading them as owed.
- **And running check 1 rather than describing it found a third thing, in §13's own preamble.** Revision 6
  wrote that *"check 1's `comm -23` will list both until they land"* of G-28 and G-29. It never did and never
  could: both rows cite, by name, the existing test they retarget, so check 1 resolves them. **A claim about
  what a check would print, in the section whose subject is claims that cannot be checked, written by an
  author who did not run it** — struck in place. What the run actually returns is five names, all from
  `fix/corpus-loader-guards`, and §13 now states that as a measurement with the date it was taken.
- **The gap that made it possible is worth more than the instance.** P-41 checks that every *cited* name
  resolves. It cannot check that a row *owing* a guard has cited a name for it — a row citing nothing has
  nothing to fail on, and a row citing a *different, existing* test is resolved and **passed**. **Of the
  eight rows in §13 whose guard does not exist on this branch, four are invisible to check 1** by one of
  those two mechanisms. **§13's table carries the row-by-row assignment and this sentence deliberately no
  longer repeats it** *(after QA #11102 W-5: the assignment stood here in duplicate and was wrong in both
  copies — G-29 cites no test name either, so the second mechanism is G-28 alone. **The duplication is the
  finding**, not the misattribution: this was one claim at two sites, inside the remedy for a defect that
  was one claim at two sites. One site now owns it)*. Revision 6 found a property with no row; this is rows
  whose guard no name reaches, and it is the same audit gap from the other side.
  *(Corrected after QA #11099 CF-1. As first written this said "four of the six rows this document owes the
  tree are **invisible**" while §13 said four of six were **visible** — the same measurement stated twice
  with the numerator inverted, under a denominator neither section defined and a phrase, "owed to the
  tree", that §13's own status table uses for a different and narrower set. **Three populations were
  reachable from the text and the document named none.** The population is now stated at both sites, and it
  is eight, not six.)*

---

## 17. Open questions

1. **Who authors the corpus?** Toni, or an agent whose rows Toni reviews. It changes nothing structural, but
   §4.3's definition means the labeller's judgement *is* the metric, so it should be a deliberate choice
   rather than whoever is free. My recommendation: Toni authors the first ten to fix the definition by
   example, and ~~an agent extends to thirty against those ten~~ *(revision 7: the extension to thirty is
   **suspended** — §6.3 explains why sizing is now against the task set, not against a standard error.
   Eleven rows exist and may be enough. The authorship question itself is unchanged and still open)*.
2. **Should a sweep result ever be filed to DiVoid?** A measurement is knowledge and the Hivemind contract
   says knowledge goes to the graph — but a result node per sweep re-creates §3.4's pollution with a
   near-duplicate flood. **Recommendation, taken as the default:** the harness writes files; a human or agent
   files the *interesting* ones as `documentation` carrying the numbers and the reason, never as a dump.
   Automating it would be exactly the knob #1136 §3 refuses.
3. ~~**Is 30 rows the right start?** §6.3 states what it buys and what it does not. If M3's first tuning move
   is expected to be small, the corpus needs to be larger before M3 begins, and that is a scheduling
   decision rather than a design one.~~ *(Revision 7: **answered, and the answer is that the question was
   the wrong shape.** It presupposed the labelled rate is the headline. Under #11092 it is not, so the
   corpus is sized to the task set — §6.3. What replaces it is question 4.)*
4. **Is the A/B's task set the eleven corpus rows, or a different set?** *(Revision 7.)* They were authored
   to test **retrieval**, not to be **tasks a system solves** — the two overlap heavily and are not the same
   thing. #11092 §11 carries the question and its recommendation; it is repeated here because the answer
   determines whether this corpus grows, shrinks or stays at eleven, and that is a decision about **this**
   file.
