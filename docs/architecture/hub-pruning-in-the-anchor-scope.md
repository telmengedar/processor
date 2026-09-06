# Hub pruning in the anchor scope (§14 Unit 3) — the ruling

**Architect's ruling, 2026-09-06.** Written on the scope-channel census (**#12966**) and the two defects it
surfaced (**#12968**, **#12967**), against `docs/architecture/anchor-grounded-recall.md` §14 Unit 3 and §18.7.
Verified against the shipped retrieval and admission path in `internal/loop` and `cmd/eval` at Processor
`main` `1e42355`. **Nothing was edited, swept, or run.** No corpus row was read or spent; no graph query was
issued for this ruling beyond reading the four nodes named above.

**Repo parity.** This file is under P-40 parity with its DiVoid node. `anchor-grounded-recall.md` §14 and
§18.7 carry in-place markers pointing here, because §18.6(b) ruled that a hedge does not travel with a
specification once the specification is read on its own — and §18.7 currently reads as a commission. The
operator publishes both sides.

---

## The short form, for anyone who reads only this much

1. **Hub pruning is not commissioned. Unit 3 is retired, with a stated reopening condition (§11).**
   Not deferred behind #12968 — retired. Its entire delivery surface is at most three candidate slots at
   fused ranks 18–20, and three separate measurements this project already owns say that band is not on a
   path to an answer.
2. **#12968 is not the unit that unblocks it.** Its non-differential half is measured and true; its
   differential half — that the 94.7% cut rate is a fact about *the reserve* rather than about *rank 18–20* —
   is untested, and the project's prior measurement (#11365 §1: P(admit) = 0.04 at rank 20) points the other
   way. A better-placed reserve and a better-scoped reserve are the same bet on the same dead band.
3. **What is measured, cheaply, before anything at all is commissioned: D1, the promotion counterfactual**
   (§6). It is pure arithmetic over the 460 dispositions the census already holds — no sweep, no corpus row,
   no graph query, no model. It is differential: it can return *"the effect is real, and it is not specific
   to the reserve."* It can also falsify this ruling, and §6 states exactly how.
4. **The answer to "should anything be built at all right now" is: not on this channel.** The one unit with a
   measured mechanism, an already-issued Build ruling, and no implementation is **#11365 §3, compaction at
   admission** — still unbuilt at `main` `1e42355`, while its paired §4 shipped. **#12967 is the cost §11365
   §4 named in advance and accepted on exactly this row**, not a new defect. §9.

| Component | Ruling |
|---|---|
| **§14 Unit 3 — hub pruning in the anchor scope** | **Retired, not deferred.** §5. Reopening condition in §11. |
| **§18.7's "this is where the ruling turns positive"** | **Withdrawn as a commission.** The three converging lines identify the anchor's *only* channel; they do not establish that the channel reaches an outcome. §5.4. |
| **#12966 §6's pruning arithmetic (1,037 → 14, 1,027 → 16)** | **Accepted and not in dispute.** It is also non-differential: pool shrinkage cannot fail. §5.2. |
| **#12968 as the unblocking unit** | **Not commissioned.** Differential half untested; obvious remedy is inside the family #11365 §6 already rejected on measurement. §7. |
| **D1 — the promotion counterfactual** | **Commissioned. Runs first, gates everything else.** §6. |
| **D2 / D3 — hub-only displacement and re-ranking** | **Conditionally specified, not commissioned.** They run only if D1 falsifies this ruling. §6.3, §6.4. |
| **The design itself** | **Held in escrow, §10.** Written so that a reopening does not require rediscovering it. |
| **#12967** | **Interacts, but not through the mechanism.** It is a measurement-capacity interaction and it contaminates every `admitted` denominator quoted from this corpus. §9. |

---

## 1. Problem Statement

§14 Unit 3 proposes pruning hub nodes from the anchor's recall scope. Its motivation was one measured
observation (t2's 362-node two-hop scope reaching every project through `person Toni`), and #12966 §6 has now
supplied the population measurement it was waiting for: over 23 corpus rows the two-hop pool runs **min 15,
median 168, mean 395, max 1,237** — the maximum being **11.7% of the 10,577-node graph** — and in every
thousand-node case the inflation traces to a single edge to a project-scale group node, removable at a 60–74×
reduction.

The question put to this ruling is not *how do I prune a hub*. It is:

> **Given that the census's positive half measures the inflation and its negative half measures the channel's
> yield at zero, does hub pruning earn its build — and does it earn it before or after the placement defect
> #12968 describes?**

The answer must be able to be *no*, and it must not be reached by assuming the positive half licenses the
build. §18.6(d) is explicit: an account whose only gate asks whether the predicted effect appears has no gate.
Pool shrinkage is precisely such an effect — it is arithmetic and cannot fail.

**Success criterion for this ruling:** a decision on Unit 3 that names, in the same passage, a differential
falsifier runnable before anything is commissioned, and that says plainly what should be built now.

---

## 2. Scope & Non-Scope

**In scope**

- Ruling on §14 Unit 3 and on §18.7's characterisation of it.
- The delivery surface of the anchor scope in the shipped code, established from the code rather than from a
  node.
- Whether a scope-based prediction is testable on records that already exist (asked explicitly; answered in
  §8).
- The interaction with #12967.
- A pre-registered, differential, zero-cost gate.
- The escrow design, so a reopening costs a re-read rather than a re-derivation.

**Out of scope**

- Commissioning #11365 §3 (compaction). It carries its own Build ruling and its own pre-registered
  falsifiers F1 and F2; naming it as the live unit is not re-commissioning it, and this document does not
  redesign it.
- Any change to `corpus.json`. It is intact, it is the harm guard, and this ruling neither reads nor spends a
  row.
- The retired #12941 anchor-stratified rows. Nothing here plans on them.
- The reserve-rebalancing family — sizes 5/7/10 and the 50/50 interleave. Rejected on measurement at #11365
  §6 and not re-proposed. Where §7 discusses placement it discusses it in order to *reject* it, and it names
  the measurement that already did so.
- Whether a run record should be a corpus subject. #12967 deliberately did not fold that in; neither does
  this.

---

## 3. Assumptions & Constraints

| # | Assumption / constraint | Confidence |
|---|---|---|
| A1 | `CandidateLimit` = 20, `RecallScopeReserve` = 3, `AssemblyByteBudget` = 60,000, at `main` `1e42355`. | Read from the source. Certain. |
| A2 | The scoped recall's results reach the output **only** through the reserve step; they contribute nothing to the reciprocal-rank fusion. | Read from the source. Certain. §4.1. |
| A3 | Admission is greedy in rank order against `budget − len(anchor.Content)`, whole-node, skip-not-stop. | Read from the source. Certain. |
| A4 | #12966's figures are as reported; its sweep result was recovered rather than produced, and its provenance argument is the corpus-hash and rate agreement it states. | Taken as given. The census flags this itself. |
| A5 | §14's standing rule that no step introduces a tunable constant still binds. | Assumed. It directly constrains the escrow design (§10.4) and is the reason the recommended pruning rule is structural rather than numeric. |
| C1 | No corpus row may be read or spent by any gate this document commissions. | Hard. |
| C2 | M-1 (#12961) binds: the falsifier is named in the same passage as the mechanism, is differential, and runs before anything is commissioned. | Hard. §6. |
| C3 | M-2 (#12961): if an instrument is needed, its authoring pass is priced separately. | Hard. Answered in §6.5 — no instrument is needed for the deciding gate. |
| C4 | #11365 §6's rejection of the reserve-rebalancing family stands and is not re-litigated. | Hard. |

---

## 4. Architectural Overview — the delivery surface, established from the code

This section is the load-bearing one, because the ruling turns on how narrow the surface is and that is a
fact about `internal/loop/retrieve.go` and `internal/loop/assemble.go` rather than about any node.

### 4.1 The anchor's scope reaches the output through exactly one aperture, and it is three slots wide

```
  queries ──▶ one unscoped recall per query ──▶ reciprocal-rank fusion ─┐
                                                                        │  ranks 1..17
  anchor.ID ─▶ Neighbours ─▶ scope = {anchor} ∪ N(anchor)               │
                   │                                                    ▼
                   └─▶ one scoped recall, on queries[0] only ──▶ RESERVE ranks 18..20   (≤ 3, unseen only)
                                                                        │
                                                    fused backfill ─────┘  any remaining slots

  ranks 1..20 ──▶ greedy whole-node admission, in rank order,
                  against  remaining = 60,000 − len(anchor.Content)
```

Three properties of that picture decide this ruling, and each is read from the source rather than inferred:

1. **The scoped list is not fused.** Reciprocal-rank fusion is computed over the unscoped lists alone. The
   scoped list is consumed only by the reserve step. It therefore contributes **no ranking signal** — it
   cannot promote, demote or reorder anything.
2. **The reserve step admits only candidates the fused list did not already contain.** A scoped result that
   the unscoped recall also returned occupies no reserve slot and gains no position from having been scoped.
   Its rank was decided by fusion.
3. **The first fused pass stops at `limit − reserve` = 17.** So reserve occupants are always at fused ranks
   **18, 19, 20**. The census observes exactly this: 13 at rank 18, 21 at 19, 23 at 20, totalling 57.

**Consequence, and it is the whole of the ruling's foundation:** every effect any change to the anchor's
scope can have on a shipped turn is confined to the identity of at most three candidates at fused ranks
18–20. Hub pruning changes the pool those three are drawn from. It changes nothing else, anywhere.

### 4.2 What arrives in those three slots, and what becomes of it

From #12966 §3, §4, §7 and §8 — one arm, one graph state, 460 labelled dispositions:

| Quantity | Measured |
|---|---|
| Reserve slots available (23 rows × 3) | 69 |
| Slots filled by scoped-only arrivals | **57 (83%)** |
| Rows with zero scoped-only arrivals | **0** |
| Scoped-only arrivals **admitted** | **3 (5.3%)** |
| Scoped-only arrivals cut, all for byte budget | **54 (94.7%)** |
| Required node arriving *only* via the scoped arm | 1 of 23 — and cut |
| Required node **admitted** via the scoped arm | **0**, in all twelve arms ever swept |
| Scoped arrivals that displaced a required node | **0** of 23 |
| Arrivals that were off-project by title | 4 of 57 (7%) |

The channel is full, near-silent, and — this matters for the harm side — **not noisy**. 53 of 57 arrivals are
same-project material.

### 4.3 The band the aperture opens onto is dead for everyone, not only for the reserve

#11365 §1 measured the admission curve over the same corpus:

> **P(admit) by rank: 1.00 at rank 1, 0.74 at 6, 0.39 at 9–10, 0.09 at 12, 0.04 at 20.** The budget is 78%
> spent by rank 6 and 95% by rank 10. **On 23 of 23 rows the leftover budget is smaller than the median
> candidate.**

**Naming both populations, per M-1 clause 5, because this is the exact defect that clause is made of.**
The 0.04 figure is drawn from #11365 §1: all 460 candidates of the **pinned-derivation arm** (six queries per
row), `corpusHash ffa291d5`, `derivationHash 38349b3c`, at a graph state reading 11/23 retrieved and 9/23
admitted. The 0.053 figure is drawn from #12966: the 57 scoped-only candidates of the **raw-input shipped
arm**, same corpus hash, `sweptAt` 2026-09-05T17:39:46Z, at a graph state reading 9/23 retrieved and 6/23
admitted. **Two arms, two graph states.** These two numbers may not be subtracted, compared as an excess, or
reported as a finding. What they are is the **reason to run D1**, which makes the comparison
same-population, same-file, same-instant. §6.

---

## 5. The ruling on Unit 3, and the four things it rests on

**Hub pruning in the anchor scope is retired.** Four independent measurements, none of them commissioned by
this ruling, converge on the same statement: *the scoped arm's candidate quality is not on a path to an
admitted outcome.*

### 5.1 F4 is a standing pre-registered claim and it has never been falsified

#11365 §7 registered, as a free standing falsifier on every future sweep:

> **F4 — *No required node is admitted at fused rank > 16 under the current rule.*** True today with margin —
> the nine admitted rows sit at ranks 1, 1, 1, 1, 1, 4, 4, 9, 11. One counterexample retires the claim.

The reserve occupies ranks 18–20, strictly inside F4's forbidden band. #12966 §8's twelve-arm sweep — *zero
required nodes admitted via the scoped arm, in every arm without exception* — is **F4 holding**, not a new
result. This is the strongest form of evidence available here: a claim registered before the data, on a
quantity nobody was trying to protect, surviving twelve independent opportunities to fail.

**So Unit 3 delivers into a band that a standing pre-registration of this project says cannot deliver an
answer.** That is not an argument about how good hub pruning would be at its job.

### 5.2 The experiment "make the scoped arm better" has already been run, differently, and returned zero

#11365 §6 measured the symmetric fix — fuse all six scoped lists and fill the reserve from the fused result,
instead of taking one scoped list from `queries[0]`:

> It improves scoped ranks — r07 13→9, r23 not-in-20→18, r21 not-in-20→33 — and **through the full pipeline
> it changes nothing: 11/23 retrieved, 9/23 admitted, zero verdict changes.**

That is the same class of change as hub pruning: it improves what the scoped arm supplies without touching
where the scoped arm delivers. It improved the intermediate quantity and moved nothing end to end.

Hub pruning's measured motivation (#12966 §6) is likewise an **intermediate** quantity — pool cardinality.
The 1,037 → 14 reduction is arithmetic on a neighbour set. Per §18.6(d) it is the non-differential half: it
cannot fail, and a gate it would pass is not a gate. The differential half — *that the arrivals on inflated
rows are worse than the arrivals on uninflated rows* — has never been tested, and §5.3 is what the existing
data says about it.

### 5.3 What the census's own numbers say when the pool sizes are joined to the outcomes

Joining #12966 §6's per-row pool table to §3's outcomes gives a result the census does not itself state:

| Row | Two-hop pool | What its scoped arrivals did |
|---|---|---|
| r08 (#10482) | **93** | the corpus's only required node to arrive via the scoped arm |
| r22 (#10936) | **105** | two of the three admitted scoped arrivals |
| r09 (#10924) | **168** | the third admitted scoped arrival |
| r13, r16, r17, r19, r20, r23 | **1,017 – 1,237** | nothing admitted, no required node |

**Every scoped arrival that did anything at all came from a row at or below the median pool size. The six
thousand-node rows — the exact rows hub pruning exists to fix — produced no admitted arrival and no required
node.**

This reads at first as evidence *for* pruning: the inflated rows are the useless ones. It is not, and saying
why is the point. There are four events here (three admissions, one required-node arrival), and two stories
fit them equally: *inflation destroys yield*, or *yield comes from rows that never needed pruning and pruning
would not have reached them.* At n=4 neither is distinguishable from the other. This is §18.4.2's error
exactly — a number with several candidate causes and no arm separating them — and it is the reason §6's gate
is specified as a counterfactual over all 57 arrivals rather than as a correlation over 4 events.

### 5.4 The harm side is not bounded at zero, and the census closed the argument that justified pruning

Two of the census's findings cut against the build directly:

- **The displacement channel is measured at zero.** §4(b): re-running admission with every scoped-only
  candidate above a required node removed **flips nothing** — near-impossible by construction, since scoped
  arrivals only ever sit at 18–20 and the three cut required nodes sit at ranks 3, 3 and 18. So hub pruning
  cannot recover a slot from a hub-borne intruder, because no hub-borne intruder has ever cost a slot.
- **The "it stops other projects leaking in" justification is largely not what is happening.** §7: only
  **4 of 57** arrivals are off-project. 53 are same-project material. **Pruning therefore removes, in
  expectation, mostly legitimate same-project reachability from a channel whose measured contribution to
  answers is zero.** The upside is bounded above by three dead slots; the downside is not bounded at all.

§18.7 wrote that three independent lines converge on the id-to-scope channel. They do — and what they
establish is that it is **the anchor's only channel**, which is a fact about the anchor, not a fact about the
channel's yield. A channel can be the only one and still be three slots wide and 96% cut. **That inference is
where §18.7 turned positive one step too early, and it is withdrawn as a commission.**

### 5.5 The one argument for pruning that is not about retrieval, and why it does not carry the build

Pruning would also shrink the scoped recall's request: a union over up to 37 `linkedto` parameters against a
1,237-node pool becomes a union against 14. That is a real cost reduction on the turn's hot path.

**It is unmeasured.** No latency figure for the scoped recall exists in any record this project owns. An
unmeasured cost is not evidence, and commissioning a build on it would be the same shape of error M-1
exists to prevent. If the operator wants the cost argument to carry weight, it needs a measurement, and that
measurement is one timed recall per corpus subject — cheaper than D3 and independent of everything here.

---

## 6. The falsifier — pre-registered, differential, and it runs before anything is commissioned

**This section discharges M-1 for the recommendation this document makes.** The recommendation is a negative
claim, so #12935 applies as well: a negative claim files its falsifier the moment it is written.

**The claim being gated, stated so it can fail:**

> The scoped arm's candidate quality is not on any path to an admitted outcome, because its delivery ranks
> are 18–20, and rank 18–20 is a dead band for every candidate regardless of provenance. Therefore no change
> to *what* the scoped arm supplies — hub pruning included — can move `admitted`, and no change to *where* it
> delivers can either, because the band is dead for byte-budget reasons that promotion does not remove.

### 6.1 D1 — the promotion counterfactual. Commissioned. Runs first.

**Input:** the census's own sweep result — 460 labelled dispositions, one arm, one graph state, each carrying
rank, size, `included`, `cutReason` and `sources`. **Cost: one pass over one file.** No sweep, no corpus row,
no graph query, no model call, nothing written.

**Procedure.** Per row, re-run the same greedy whole-node admission against the same `remaining`, with the
scoped-only candidates moved from ranks 18–20 to the front of the rank order (relative order among them
preserved, every other candidate shifted down by the same amount). Sizes are the recorded sizes; no
re-ranking, no re-recall, no new content. Report, before and after:

| Reported quantity | Why it is in the gate |
|---|---|
| (a) scoped-only candidates admitted | the volume the reserve could buy if placement were free |
| (b) **required nodes admitted** | the only outcome the corpus can adjudicate objectively |
| (c) **required nodes lost** — admitted today, evicted after | the harm #11365 §6 measured live on the interleave |
| (d) mean documents per block | whether the change is volume or answers |

**Registered outcomes, written before it runs:**

| Result | Reading |
|---|---|
| (b) flat **and** (a) rises sharply | **This ruling stands, and the differential half has fired.** The 94.7% cut rate is real and is *not* specific to the reserve's provenance or its placement: putting better or earlier candidates into a saturated byte prefix buys volume, not answers. Both Unit 3 and #12968 are optimising a quantity no outcome reads. This is the same shape as #11365 §5's `CandidateLimit` result (*"moves `retrieved` only, never `admitted`"*) and §6's fused-scoped result (*"changes nothing end-to-end"*), and it would make three. |
| (b) rises with (c) at zero | **This ruling is falsified.** Placement is the binding constraint, #12968 is real and specific, and Unit 3 becomes a live unit behind it — in that order, never before it. |
| (b) rises but (c) rises too | #11365 §6's interleave finding reproduces arithmetically. The family stays rejected; neither unit is commissioned; the finding is filed. |
| (b) **falls** | Stronger than this ruling claims: the reserve's late placement is currently *protective*, and any promotion proposal is actively harmful. |

**Why this is differential and not a restatement.** It can return *"the effect is real, and it is not specific
to the stated cause"* — outcome row 1 is exactly that sentence, and it is the outcome I expect. It can also
return the opposite and kill this document. A gate that could only confirm the 94.7% figure would confirm
arithmetic; this one separates *rank* from *provenance*, which is the disputed clause in both #12968 and
Unit 3.

**One honesty note on D1's own limits.** Greedy admission is order-dependent and promotion changes the order
for every candidate, not only the promoted ones. D1 therefore measures the *counterfactual*, not a live
system; it cannot see any re-ranking a real placement change would induce. That is the correct scope: the
question at issue is whether the byte prefix or the provenance is binding, and both are visible in the
recorded sizes. If D1 falsifies this ruling, the live version is the next purchase — and only then.

### 6.2 D1 must be run over one arm and reported over one arm

M-1 clause 5, applied to my own gate. Every quantity in D1 is drawn from the same file, the same arm, the
same corpus hash and the same graph instant. The 0.04-versus-0.053 comparison in §4.3 is **not** part of D1
and is not a finding; it is the motivation for making the comparison same-population. If D1 is run and
reported, §4.3's cross-arm pair should be dropped from the record rather than carried alongside it.

### 6.3 D2 — the hub-only displacement gate. Specified, **not** commissioned.

Runs only if D1 falsifies this ruling. **Necessity gate for Unit 3, independent of benefit.**

For each of the 57 arrivals, determine whether it is reachable from its anchor in the *pruned* two-hop pool.
Pruning changes the shipped output only for arrivals that are **hub-only** — reachable today solely through
the pruned edge. If hub-only arrivals are near zero, pruning does not change who occupies the three slots and
Unit 3 is a no-op on the output, whatever D1 said.

**Threshold, registered now so it cannot be chosen afterwards: Unit 3 clears necessity only if at least 19 of
57 arrivals (one third) are hub-only.** Below that, the 60–74× pool reduction is real and inconsequential —
the inflation is there, and it is not what fills the reserve.

**Cost:** a graph walk over the 21 distinct corpus subjects' scope sets. No corpus row, no sweep, no model.
The census already did the harder half of this when it computed the pruned pools.

### 6.4 D3 — the re-ranking half. Specified, **not** commissioned.

Runs only if D2 clears. D2 says whether pruning *vacates* slots; it cannot say what *fills* them, because
that requires ranking inside the pruned pool — a live scoped recall per row. **23 recalls against the graph,**
comparable in cost to #12957's twelve queries; still no corpus row, no sweep harness and no model. Its
outcome measure is the required-node retrieval rate, and its harm guard is that no currently-retrieved
required node may become not-retrieved.

### 6.5 M-2 — what an instrument's authoring pass would buy, if one were bought

**No instrument is needed for the deciding gate.** D1 runs on a file that exists.

The nearest instrument-shaped purchase is D2's **hub classification of the corpus's scope sets**, and it is
worth pricing because §18.5 established that the authoring pass, not the artefact, is where the yield lands.
Its authoring pass alone would buy three things, all of which survive Unit 3 never being built:

1. **An operational definition of "hub", which the census shows is at least two families, not one.** The
   thousand-node inflation traces to **structural group containers** (#324 `Tasks`, #325 `Docs` of project
   #69 — nodes that carry no type at all under the #493 conventions). #10422's 384 → 234 traces to **identity
   nodes** (#10 `person Toni`, #1 `organization Pooshit`, #11 `agent Selene`) — ordinary typed nodes that are
   hubs by degree. Any pruning rule must choose which family it targets, and the two admit different rules.
2. **A per-row scope-member degree table**, reusable by any future scope work and by the cost measurement in
   §5.5.
3. **The answer to whether a constant-free structural rule covers the six thousand-node rows** — which is the
   question §14's no-tunable-constants rule forces, and which §10.4 currently answers only by argument.

**Do not buy it now.** It is gated behind D1 and D2, in that order.

---

## 7. On #12968 — why it is not the unit that unblocks this

#12968's measurement is right and its structural reading is right as far as it goes: the reserve's slots are
ranks 18–20 by construction, admission spends the budget in rank order, and the two arrangements are in
direct contradiction. Its own closing sentence — *"the disposition-level split is the measurement to carry
forward"* — is the correct instruction and this ruling adopts it.

**Where it stops short is the differential clause**, and it is the same clause that killed the last two
accounts:

| Half | Status |
|---|---|
| "Scoped arrivals are cut at 94.7%" | **Measured. Cannot fail. Non-differential.** |
| "…*because they are the reserve*, placed where the budget is gone" | **Untested.** It requires that rank 18–20 is worse for scoped arrivals than for anything else there. |

#11365 §1 measured P(admit) = 0.04 at rank 20 over that corpus (populations named in §4.3 — do not subtract).
If the same-population version comes back at parity, then **the reserve is admitted at the rate its rank
predicts**, "the reserve is structurally unable to pay for itself" is a restatement of "admission is a byte
prefix roughly ten to twelve candidates deep", and the reserve is not the thing to fix.

Three further facts constrain #12968's option set before any design work begins:

1. **Its option 1 — place scoped arrivals where admission can pay — is largely foreclosed by measurement.**
   #11365 §6 ran the crude form (a 50/50 interleave) and it *demoted required nodes on rows that were
   working*. Promotion evicts, and what it evicts is ranks 1–11, which is where every admitted required node
   in this project's history has ever sat.
2. **No admission change currently on the table brings ranks 18–20 into reach.** #11365 §3's compaction, at
   its most aggressive measured bound, takes mean documents per block from 8.1 to 12.2 out of 20. That
   extends the horizon; it does not extend it to 18.
3. **Its option 2 is free and this ruling adopts it.** *Stop counting the reserve as an admission mechanism.*
   The reserve buys retrieval breadth — one row of `retrieved` on this corpus, and it is the only
   non-similarity ranking a turn performs (#11398). That is a real if small property and it should be
   reported as retrieval breadth, never as an admission channel, in every rate this project quotes.

**So: hub pruning is not worth building before the placement problem is solved, and it does not become worth
building after it either, because the placement problem as framed is probably not a problem about placement.**
Both units are bets on the same band, and D1 settles whether the band is dead for reasons either of them can
reach.

---

## 8. Is a scope-based prediction testable on records that already exist? Partly — and the testable part is
not the deciding part

Asked directly, so answered directly.

| Quantity | Derivable without a sweep? |
|---|---|
| Two-hop pool cardinality per row | **Yes.** The census computed it (§6) from graph queries alone. |
| Which scope member causes the inflation | **Yes.** The census did it (§6's pruning table). |
| Whether a given arrival is hub-only — i.e. whether pruning would *vacate* its slot | **Yes.** D2. Graph walk, no corpus row, no sweep. |
| Which node would *fill* a vacated slot | **No.** Requires ranking inside the pruned pool — 23 live scoped recalls. D3. Still no corpus row and no sweep harness, but not derivable from records. |
| Whether the filling node would be admitted | **No, and it does not need to be asked** — its rank is 18–20 by construction, and §5.1 and §6.1 already govern that band. |

**So the necessity half of a scope-based prediction is testable on existing records; the benefit half costs
23 graph queries; and neither reaches the question that decides the unit**, which is whether ranks 18–20 can
ever pay. That question is answered by D1, on a file, at no cost. **That inversion — the cheapest test is the
deciding one and the graph-based tests are the subordinate ones — is why D1 runs first and why D2 and D3 are
specified but not commissioned.**

---

## 9. #12967 — it interacts, and not through the mechanism

**Mechanism: no interaction.** r05's anchor #10897 has a three-member neighbour set and a 74-node two-hop
pool. It is not a hub row, and pruning cannot help a row whose candidate budget is zero: `remaining` clamps
to zero when a 70,660-byte anchor is charged against a 60,000-byte budget, so nothing is admitted at any
rank, from any arm, however well scoped.

**Measurement capacity: a real interaction, and it should be fixed before any further rate is quoted.**
r05 admits nothing, ever, structurally. Every `admitted` rate this project reports over this corpus therefore
has a denominator of 23 and an effective denominator of 22. The row reads as a retrieval miss in every rate,
which is what #12967 says: *"nothing in the record says the block had zero room."* Any future unit measured
on this corpus — Unit 3, #12968's successor, compaction — inherits a permanently dead row and a silently
wrong denominator. **The record distinction #12967 asks for is the part with no argument against it, and it
is a precondition for measuring anything else here.**

**And #12967 is not a new defect.** #11365 §4 shipped `candidateBudget = blockBudget − len(anchor.Content)`,
named r05 as its cost in advance — *"loses r05 (anchor 70,660 B leaves zero candidate room)"* — and stated
the remedy in the same passage: *"r05 is the argument for doing §3 and §4 together rather than either alone.
The anchor needs the same compaction the candidates need."* **§4 shipped. §3 did not.** #12967 is that
predicted, accepted, single-row cost, still standing because the half that was supposed to absorb it was
never built. That is worth recording in #12967, because it changes the fix's shape: the third option
#12967 rejects (a bounded prefix, which *"silently misrepresents the subject"*) is the *truncation* form of
compaction, and #11365 §3's own F1 says the implementable form is extraction or summarisation, not a prefix.

---

## 10. The design, held in escrow

Written so that a reopening (§11) does not require re-deriving it. **Nothing here is commissioned.**

### 10.1 Components and responsibilities

| Component | Owns | Does not own |
|---|---|---|
| **Scope construction** (today: the step that builds `{anchor} ∪ N(anchor)`) | Deciding *which* nodes the scoped recall ranks inside. This is the correct and only seam for pruning — it already owns exactly this question. | Ranking, fusion, reserve placement, admission. |
| **A scope-member classifier** (new) | Deciding, per neighbour, whether it is a hub. Pure given its inputs. | Fetching those inputs. |
| **A degree or metadata supplier** (new capability on the graph seam) | Supplying the classifier's inputs for a *set* of ids in as few reads as possible. | The pruning decision itself. |
| **Fusion, reserve, admission** | Unchanged. | — |

### 10.2 The one non-obvious mechanical finding, and it shapes the port

A naive degree probe costs one link read per scope member — up to 37 extra round trips on the turn's hot
path, which would be an unacceptable price for sharpening three slots. It is avoidable: **the links route
already accepts a set of ids and returns every incident edge for the whole set**, while the existing
neighbour-reading capability passes one id only. So member degrees are obtainable in *one* batched read plus
pagination.

The cost is then proportional to the total incident edges of the scope set — which is dominated by the very
hub being looked for. Discovering a 1,000-member hub costs roughly 1,000 edge rows at 500 per page. **Bounded,
deterministic, but not free**, and it must be stated rather than discovered during implementation. This is
the principal reason the recommended rule below is structural rather than degree-based.

### 10.3 Contract for the classifier, in prose

- **Input:** the anchor's neighbour set, together with whatever per-member attribute the chosen rule reads.
- **Output:** the subset to rank inside, in a deterministic order.
- **Invariants, all of which must hold or the unit is a regression:**
  1. **The anchor is never pruned.** Removing it removes its own one-hop ring from the union, which is the
     core signal, not the noise.
  2. **The result is never empty.** If every member classifies as a hub, the scope degrades to the anchor
     alone — never to nothing, which would silently convert the scoped recall into a whole-graph recall and
     produce the *opposite* of the intended effect.
  3. **Determinism:** one graph state yields one scope.
  4. **Totality:** every anchor is handled, including isolated nodes and nodes whose neighbours are all hubs.
  5. **Observability:** the run record states which members were pruned and on what ground. Without this the
     change is unmeasurable after the fact — the same obligation §14 Unit 1 step 4 imposed, and for the same
     reason.

### 10.4 The rule itself — structural, not numeric, and the trade-off is explicit

§14 forbids introducing a tunable constant. A degree threshold is a tunable constant; a relative rule
("k× the median member degree") is a tunable constant wearing a disguise. The only constant-free rule
available is **structural**: prune members that are group containers — the nodes that carry no type under the
#493 conventions and exist to hold a project's Tasks or Docs.

**What that buys and what it misses, stated rather than glossed:**

- **Buys** all six thousand-node rows. Every one reaches its pool through #324 or #325 alone, and both are
  group containers. This is the 1,037 → 14 and 1,027 → 16 case.
- **Misses** the identity-node case entirely. #10422's 384 → 234 comes from pruning `person`, `organization`
  and `agent` nodes, which are ordinary typed nodes and structurally indistinguishable from any other typed
  neighbour. Catching them requires either a degree threshold (a constant, forbidden) or a type denylist
  (a constant in list form, and worse — it hard-codes one graph's ontology into the loop).

**Recommendation, if the unit ever reopens: take the structural rule and accept the miss.** It removes the
whole thousand-node class at no constant and at one batched read; the medium case is a 1.6× reduction whose
value has never been measured and which §5 gives no reason to expect matters.

### 10.5 Cross-cutting

- **Failure behaviour:** if the degree or metadata read fails, the correct fallback is the *unpruned* scope —
  today's behaviour. A pruning failure must never silently widen or empty the scope.
- **Determinism and the sweep seam:** the turn and the sweep must inherit pruning from one construction. A
  second copy in the sweep is the drift hazard M3 §4.3 exists to close, and it is invisible to any
  single-package test. §14 Unit 1 step 5 applies verbatim.
- **Harm guard, mandatory and not ceremonial:** no required node currently retrieved may become
  not-retrieved. §7 of the census establishes that pruning removes mostly same-project material, so this
  guard is protecting against a live risk rather than a theoretical one. It runs before any effect
  measurement, exactly as F2 ran before F1.

### 10.6 Quality attributes and the rejected alternatives

| Alternative seam | Verdict |
|---|---|
| Prune at scope construction (recommended) | Correct seam; owns the question; one batched read. |
| Prune by degree threshold | Rejected: introduces a tunable constant, forbidden by §14, and costs the full hub-sized edge read to evaluate. |
| Denylist of known hub ids | Rejected: hard-codes one graph's ontology into the loop; a constant in list form. |
| Prune after ranking (drop hub-borne results from the scoped list) | Rejected: does not reduce the pool, so the good candidates were already outranked before the filter sees them. Fixes the symptom's symptom. |
| Push the constraint into the graph API | Out of scope; requires a graph-side feature and a second team. |

---

## 11. Reopening condition — stated, so "retired" is falsifiable rather than final

Unit 3 is not dead in principle. It is dead because its delivery band is dead. **It reopens automatically
when, on any shipped configuration, the measured admission rate of candidates at fused ranks 18–20 exceeds
0.25** — one in four, against today's measured 0.04 at rank 20 and 0.053 for scoped-only arrivals (populations
named in §4.3).

That threshold is registered now, before any admission change is designed, so that it cannot be chosen to fit
whatever compaction turns out to deliver. If a future change moves the horizon that far, the reserve becomes
a live channel, scope quality acquires a path to an outcome, and §10's escrow design is the starting point.

---

## 12. Risks and mitigations

| # | Risk | Mitigation |
|---|---|---|
| R1 | **This ruling is wrong and the reserve really is provenance-disadvantaged.** | D1, run first, can return exactly that and kill this document. §6.1's registered outcome table names the result that falsifies it. |
| R2 | **A reader takes "retired" as "hub inflation is fine".** It is not — an 11.7%-of-graph pool is a real property. | §5.2 accepts the inflation measurement in full. What is denied is that it reaches an outcome. §11 states what would change that. |
| R3 | **The escrow design is read as a commission**, the §18.6(b) failure repeating one document later. | §10's heading says held in escrow; the short form and the ruling table say retired; the reopening condition is a separate numbered section with a threshold. If this still reads as a commission, that is a defect in this document and should be reported as one. |
| R4 | **D1 is run and its result is reported cross-arm**, reproducing the §18.3(c) defect. | §6.2 forbids it explicitly and names the pair to drop. |
| R5 | **D2's threshold is renegotiated after it runs.** | Registered in §6.3 before it runs: 19 of 57. |
| R6 | **#12967's dead row silently contaminates whatever is measured next.** | §9. The record distinction is a precondition, not a follow-up. |

---

## 13. Open Questions

| # | Question | Blocking? |
|---|---|---|
| **O1** | Does D1's promotion counterfactual reproduce #11365 §6's live interleave result? If the arithmetic and the live measurement disagree, one of them is wrong and it matters which. | No — but it is the first thing to look at in D1's output. |
| **O2** | **Supplementary recall passes no scope at all** and admits against a *fresh* 20,000-byte budget, so its early ranks are genuinely admissible — unlike the reserve's. It is the only place in the shipped loop where scope quality could plausibly reach an admitted outcome. **It is also unmeasured by every instrument the project owns**, because the sweep calls no model and therefore never reaches it. Recorded as an observation generated from the code path, per §18.4.3's preferred provenance. **Not commissioned, and it must not be** until an instrument exists that could score it. | No. |
| **O3** | An offline pass deriving a node's condensed `substance` appears to be in flight outside `main` (a `condense` worktree at this checkout, not on `main` at `1e42355`). If it lands, #11365 §3's compaction acquires a non-truncating compact form, which is precisely the half F1 says is unmeasured. Stated as an observation with its provenance; **not verified against any branch and not relied on anywhere in this ruling.** | No. |
| **O4** | Should the reserve be reported as retrieval breadth rather than as an admission channel in the project's standing rates (#12968's option 2, adopted in §7)? This is a reporting change, not a product change. | No. |

---

## 14. Implementation Guidance for the Next Agent

**Nothing is to be built on the anchor-scope channel.** In order:

1. **Run D1** (§6.1). One pass over the census's sweep result. No sweep, no corpus row, no graph query, no
   model, nothing written. File the four reported quantities and the registered reading. **This gates
   everything else in this document.**
2. **If D1 leaves this ruling standing** — which §6.1 names as the expected outcome — then:
   a. File the result as the third member of the family with #11365 §5 and §6: *changes to what the candidate
      supply contains move `retrieved` and not `admitted`.*
   b. Adopt #12968's option 2 (§7, item 3): stop reporting the reserve as an admission mechanism.
   c. **Stop.** Do not commission D2, D3, Unit 3, or a placement change.
3. **If D1 falsifies this ruling**, run D2 (§6.3) against its registered threshold, then D3 (§6.4) only if D2
   clears. Unit 3 is commissioned only after both, and only behind the placement fix, never before it.
4. **Independently of all of the above**, and not gated by it: fix #12967's *record* distinction so a caller
   can tell "nothing was relevant" from "there was no room for anything". §9 — it is a precondition for
   trusting any `admitted` denominator quoted from this corpus, including D1's.
5. **The live unit, and it is not this one.** #11365 §3, **compaction at admission**, carries a Build ruling,
   a measured mechanism flat across a 3.3× range, a +2-of-2 conversion on the only two rescuable rows, and
   pre-registered falsifiers F1 and F2. It is unbuilt at `main` `1e42355` while its paired §4 shipped, and
   §9 shows that gap is what left #12967 standing. **It is not commissioned by this document and does not
   need to be — it was already commissioned and never built.** It is named here so the answer to "what should
   be built right now" is not left empty by a ruling that says no to two things.

---

## 15. Refs

Census **#12966** · reserve placement defect **#12968** · anchor-larger-than-budget defect **#12967** ·
the ruling this supersedes in part **#12958** (§14 Unit 3, §18.7) · method contracts M-1 / M-2 **#12961** ·
admission triage, F4, the rejected reserve family **#11365** · fusion inert in a turn **#11398** ·
constrained context **#11364** · anchor budgeting **#11335** · nodes larger than the budget **#11308** ·
self-poisoning **#11141** · skip-not-stop **#11158** · corpus trap **#11360** · structural conventions
**#493** · discriminating tests **#12174** · falsifier-with-negative-claim **#12935**.
