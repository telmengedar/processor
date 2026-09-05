# Architectural Document: Processor M1 — The Skeleton Loop

> Repo path: `docs/architecture/m1-skeleton-loop.md` (canonical copy — the DiVoid node carries the same
> document verbatim).
> Task: DiVoid **#10521** · Project: **#10422** · Vision: **#10424**, expanded at `VISION.md`.
> Predecessor designs: **#10437** (M0 service skeleton) and **#10488** (process-boundary test harness),
> whose configuration boundary, listener-injection shape and container venue this document consumes
> unchanged.
> Standards applied: Design Contracts **#1136**, Code Contracts **#114 §0 and §4** (§4 via the Go annex
> **#10861** — see the correction directly below), DRY threshold **#1267**,
> anti-seed-complexity **#1184**, vocabulary rule **#1220 §2 addendum 2026-08-17**, falsifiable-universals
> rule **#1220 §5 addendum 2026-08-28**, container rule **#10440**, edge conventions **#7216**.
> ~~**Code Contracts #114's Go annex is still not a ruling** (#10437 §13.1, #10488 §13.2, task **#10495** —
> this is the third consecutive design to record it). Nothing here leans on it: only the
> language-independent sections are used — §0's principles, §11's logging discipline, §13's
> parallel-by-default and no-shared-fixture-state, and §13.1.1's guard-axis rule. No decision below waits
> on it.~~
> **CORRECTED 2026-09-02 — the annex now exists, and the scoping struck above was wrong where it counted.**
> **#114 §4 (Comments) binds Go code on this repo**, per the ruling **#10861**, which supersedes that
> scoping insofar as it excluded §4. The applied set is **§0, §4, §11, §13, §13.1.1**. §4 and #10861 are
> **cited, not restated** — read them at the nodes. Question 3 of **#10495** (what replaces §16's pre-PR
> checklist for a Go module) **remains open**. §13.5 carries the correction in full.
> **Every fact in §3 was measured on this machine on 2026-09-01, with the commands and outputs quoted.**
> Two hypotheses this document was going to assert were measured before being written: one held (C36) and
> one was **wrong in its strong form and is recorded in its weakened, measured form** (C38).
> **Revision 2, 2026-09-01 — Toni's provider-agnosticism ruling (recorded in full on task #10521).** The
> model call is no longer Anthropic-specific: M1's first and only adapter speaks an **OpenAI-compatible
> chat-completions** endpoint, and its API key is **optional**. Unit A contains no model call, is
> implemented and in review, and **nothing in this revision touches it**. Revision 2 re-measured rather
> than inherited (C40–C44), and three of this document's own arguments moved as a result — each recorded
> in place rather than quietly repaired: **the model port's justification is withdrawn and replaced**
> (§9.2), **the dependency's quantitative cost nearly evaporated while its structural cost did not**
> (§10.9), and **the no-retry rule lost its spend argument in the local case** (§10.5). The credential
> blocker (§13.1) is withdrawn outright.
> **Revision 3, 2026-09-02 — the budget system made whole, from QA review #10821.** Revision 2 bounded the
> assembly path and left the supplementary-recall path unbounded; measured, one recall round renders
> **3.3× the entire assembly budget** into the same prompt (#10821 CF-4, on C23's own 20-row set). The
> defect was not a missing number on one path — it was that **the budget was stated as a fraction of an
> unnamed window** (§8.4), so no path had an absolute ceiling to be checked against. Revision 3 states the
> ceiling in bytes (§8.4), applies §6.3's admission discipline to the recall path (§6.4a), gives the two
> paths **the same record columns** because they are the same event (§8.2), and re-derives §8.4's own
> "1.5% of the window" figure, which was arithmetic against the provider revision 2 removed. Three
> contract questions the review raised are settled in place: the usage type stops being one provider's
> object (§8.3), usage is recorded **per model call** rather than overwritten (§8.2), and the record
> carries the constants that governed the run so it stays interpretable after they change (§8.2).
> **Unit A is untouched again** — where the sweep landed on shipped assembly code, §11 R4, R13 and R14
> record the finding, its number and its falsifier rather than changing it.
>
> **CORRECTION SET 2026-09-04 — the loop was poisoning its own next turn, and §6.3's falsifier fired.**
> Design **#11158**, from task **#11141**: two turns on the same input admitted **zero of twenty**
> candidates, because a run record ranks early on a repeat of its own input, is structurally larger than
> the budget that produced it, and admission stopped at the first candidate that did not fit. Unit A's
> shipped assembly changes for the first time, and six sites in this document are **corrected in place**
> — strike, do not delete, per #10466's own convention and the worked precedent at
> `docs/architecture/run-record-fate.md` §16:
>
> | # | Section | What is falsified | Correction |
> |---|---|---|---|
> | **B1** | **§6.1 step 5** | *"The first candidate that would exceed it is cut, and so is every candidate after it"* | Admission skips; the walk visits every candidate |
> | **B2** | **§6.3**, *"Why the budget stops rather than skips"* | The rule, and the invariant *"'included' means 'outranked everything excluded'"*. **§6.3's own stated falsifier has fired, twice, and is what carries the change** | Struck and replaced with the measured firing and the two rules that replace it |
> | **B3** | **§6.4a**, the *Admission* row | *"stop at the first that does not fit, no back-fill"* on the supplementary path | Both paths call one `admit`; both skip |
> | **B4** | **§8.2** `candidates[]`, **§8.3** recall row, **§10.3** | Neither the second cut reason nor the shutout alarm existed | Extended, not struck — these are additions |
> | **B5** | **§11 R13** | Its remedy — *"excluding the run-record type is one query parameter"* | Ruled **adopted by a different mechanism than it proposed**: cut at admission, never filtered from the query |
> | **B6** | **§14** unit A step 3 and *Do not add* | Both direct a future implementer at the rule this correction removes | Corrected in place |
>
> **What this does not change:** `Assemble` stays pure (§9.1), ~~the anchor stays exempt (§11 R4)~~
> **STRUCK 2026-09-05 (#11335, E8): the anchor is charged against the budget from this date; only its
> rendering is unconditional — see the correction set directly below**, the
> record still carries every candidate (§9.4 obligation 1), and the recall query is still unscoped
> (§6.2). **The record is deliberately still larger than the budget** — #11158 §7.3 rejects capping it,
> because a record small enough to fit is a record that gets *admitted*.
>
> **CORRECTION SET 2026-09-05 — `AssemblyByteBudget` did not bound the artifact it names, and now nearly
> does.** Task **#11335**, ruled by **#11365 §4**: `Assemble` charges the anchor's body against the budget
> before admitting any candidate — `remaining = AssemblyByteBudget − anchorSize`, floored at zero. **Only the
> accounting changed. `renderBlock` is untouched: the anchor still renders whole and is never cut**, which
> #11335 is ruled on — a turn whose subject was dropped for size answers about nothing. **This document
> asserted the opposite invariant — that the anchor is exempt from the budget and rides unbounded on top of
> the ceiling — across every section listed below, and each is corrected in place.** The sweep was run by
> row rather than by phrase and joined across line breaks, per **#11034 P-52**; the *"plus the anchor"*
> family at E7 is five statements of one claim in five sections, which a phrase sweep aimed at §11 R4 would
> not have reached. Labels continue the
> **A** (2026-09-02) and **B** (2026-09-04) sets; **C** is this document's measured-fact series and **D** is
> §6.6's depended-on subset, so this set is **E**.
>
> **Why now, and it is not tidiness.** **#11364** added the constrained-context vector: the measured working
> context on the project's own RTX 3090 is **32,768 tokens** — roughly 131,000 bytes of raw text before the
> system prompt, the input and the reserved output. Measured blocks ran **60,601–130,383 B** against a
> constant reading 60,000, with **r05 at 130,383 B** and **r14 at 108,317 B**. Under that vector an
> unbounded anchor stops being untidiness and becomes a product failure: a project whose claim is *"a
> well-chosen 60 KB"* cannot emit 130 KB. The ruling is **#11365 §4** and is **cited, not restated** — read
> it at the node.
>
> **And it is a bound with two named residuals, not a ceiling — stated here so a reader who stops at the
> header is not left with the rounded version.** The budget governs **content bytes**; `Assemble` returns a
> **rendered string**, and `renderBlock`'s banners and `id:` / `type:` / `name:` headers are framing no budget
> sees. So `len(block) = anchorSize + Σ admittedBytes + framing(1 + admittedCount)`, of which only
> `anchorSize + Σ admittedBytes ≤ max(anchorSize, AssemblyByteBudget)` is bounded — **framing measured at
> 178–2,444 B across the sweep, mean 1,384 B, unbudgeted.** **#11365's falsifier F3 —
> *"no block exceeds `blockBudget`"* — therefore passes on content bytes and fails on the rendered
> artifact:** on content, **22 of 23 rows respect 60,000** and r05 does not, at 70,660 B, because its anchor
> alone is; on `len(block)`, **5 of 23 respect it and 18 do not**, the worst at **70,838 B**. Both numbers are
> stated because F3's own words are about the block, and the content-only yardstick — this document's
> pre-existing convention — is not the one F3 was written in. Closing the anchor remainder needs
> **compaction** (#11365 §3), the next unit, which carries an unmeasured risk this one does not; **the framing
> residual lands on that unit too, and §11 R4 says why it is not a rounding error.** §11 R4 carries the whole
> of it with the measurements.
>
> | # | Section | What is falsified | Correction |
> |---|---|---|---|
> | **E1** | **§11 R4** | The row itself: *"The anchor is exempt from the budget, so a pathologically large subject node is an unbounded input"* — and its remedy, *"truncate with an explicit marker"* | Struck and replaced with the bound, its named residual, and the mechanism that actually shipped. **This row is the correction's home; every other site points here** |
> | **E2** | **§6.4a**, the *Exemption* row | *"The anchor (§11 R4)"* as the assembly path's standing exemption | The assembly path's exemption is now from *cutting*, not from *accounting* |
> | **E3** | **§6.4a**, *"Why there is no exemption"* | Its premise — *"the anchor's exemption from the assembly budget is this design's one unbounded input"* | The argument survives and its premise moves: the unbounded input is gone, the never-cut input is not |
> | **E4** | **§6.4a**, the per-run ceiling block | *"and the anchor on top + \|anchor\|, unbounded"* | The anchor is inside `AssemblyByteBudget`; what remains outside is the residual, stated as such |
> | **E5** | **§8.4**, the ceiling block, the *~2,000 B* allowance and the window table | *"+ the anchor \|anchor\| — unbounded"*, the column *"anchor excluded"*, *"leaving ≈ 3,200 tokens — about 12,500 bytes — for the anchor"*, and — found in review — *"~2,000 B for the system text and framing"*, measured at **3,656 B, about 1.8× short** | The ceiling now includes the anchor; the 32,768 row's leftover becomes slack rather than an unfunded allocation for it; the allowance and that row's 90% figure are corrected, with no verdict changing |
> | **E6** | **§8.4**, the *Assembly byte budget* row | What the constant bounds, and its measured admission profile *"admits ranks 1–5, uses 44,931 B, cuts 15 of 20"*, which held only for a zero-length anchor | Restated as a block budget, with the measured cost; **the name stays `AssemblyByteBudget` and §8.4 records why**, so it is not re-opened |
> | **E7** | **TL;DR**, **§10.3**, **§11 R5**, **§11 R7**, **§11 R14** | Five sites of one claim — *"100,000 bytes per run, **plus the anchor**"*, *"the block is up to 60,000 bytes"* (false before, nearly true now), the **50,000–110,000 B** record size, *"~15,000 input tokens **plus the anchor** per call"*, *"183% … **before the anchor**"* | Corrected in place. Enumerated as one row because they are one claim in five places (#11034 P-52) |
> | **E8** | **the 2026-09-04 set's own** *"What this does not change"* | *"the anchor stays exempt (§11 R4)"* | Struck above, dated, not deleted (#11034 P-43) |
> | **E9** | **§6.5**, the supplementary-shutout row | Its analogy — *"§11 R4's anchor exemption"* — cited an exemption that has since narrowed | The analogy holds against the narrower exemption and says which one it means |
> | **E10** | **§14** unit A step 3 and *Do not add* | Step 3 pins an admission rule that no longer describes the run; *Do not add*'s *"a size exemption is still forbidden on either budget"* is now ambiguous about the one exemption that stands | Corrected in place; step 3 gains the two pins this change ships with |
> | **E11** | **§10.3**, the shutout WARN | Nothing — **extended, not struck.** The alarm now fires on a cause it could not previously reach | Recorded, because the detector needed no change to reach it |
> | **E12** | **§6.1 step 5**, **§6.3**'s admission walk, and **§6.3**'s block layout | The first two describe the walk as spending `AssemblyByteBudget` and never say the anchor is charged first; the layout presents its banners as free | The mechanism's own sites — the walk starts from the budget minus the anchor, floored at zero, and the layout gains the framing term that no budget sees |
>
> **What this does not change.** `Assemble` stays pure (§9.1) and its signature is unchanged. The render
> order is still by id (§6.3), the skip-don't-stop walk is still the admission rule (§6.3, #11158 B1–B3),
> and the record still carries every candidate with its disposition (§9.4 obligation 1). **Retrieval is not
> touched at all** — this change begins after the candidate list exists, and the 2026-09-04 set's
> *"the recall query is still unscoped (§6.2)"* is **not** restated here, because it has since stopped being
> true of the tree: the shipped read path reserves slots for anchor neighbours and runs derived queries,
> under the later design **#11235** (`docs/architecture/m3-derived-recall.md`). §6.2 stands as dated record
> of what M1 shipped and is superseded there, not here. **`AssemblyByteBudget` keeps its name and its value
> of 60,000** (§8.4). **And the
> residual is not closed here** — §11 R4 states it as a bound with a named remainder rather than as a
> ceiling with an exception.
>
> **One further edit, listed because it is not a falsified site and a reader diffing this revision will meet
> it:** §15's *"four of this document's own claims have been measured against and did not survive intact"*
> becomes **five**, with the fifth stated directly below that table. It is a different class from the four —
> a **conclusion reversed**, not a reason lapsing under a surviving conclusion — which is why it sits below
> the table rather than in it, and why the pattern §15 names does not cover it.

---

## TL;DR

*Three decisions, not one: the input surface, the tool, and the dependency.*

**What.** `POST /runs` takes a text input and the id of the node the run is about, assembles a context
block **mechanically** from the graph, makes **one** model call, and writes a run record back. The
~~response body *is* the record~~ — every candidate, its score, and whether it was kept or cut.
**CORRECTED 2026-09-02 (#10899, A3): the response body is the record ***plus exactly one key*** — the
write receipt, which the stored copy structurally cannot carry. See `docs/architecture/run-record-fate.md`
§8.1 for what is guaranteed instead.**

**How.** Two graph reads: the subject by id, and one semantic query whose text is the input **verbatim**.
A byte budget applied in score order. A block rendered **sorted by node id, never by score**. One tool —
supplementary recall — whose results append at the message tail, **under a byte budget of their own**,
and never enter the block. **Every graph body that reaches the model is under a stated ceiling: 100,000
bytes per run, ~~plus the anchor~~** (§8.4). **CORRECTED 2026-09-05 (#11335, E7): the anchor is now
*inside* that ceiling rather than on top of it — with one residual, stated at §11 R4.** Write-back is a harness step: the harness picks the type, name and edge; the model contributes prose only.

**Cost.** Five environment variables, each arriving with its consumer, at the one existing read site.
Measured ~2 s and ~206 KB of graph traffic per run. **No new dependency.** Ships as **two PRs**: A =
assembly, fully offline-testable, no model call; B = the model call and the write-back.

**Provider.** One adapter, speaking the **OpenAI-compatible chat-completions** protocol — the interface
every local runtime serves. The endpoint address and the model name are required boot members; the API
key is **optional**, because a local endpoint commonly has none. **The loop never sees a provider's
vocabulary**: it speaks its own terminal reasons and its own recall requests, and the adapter translates
(§8.3). That, not the interface itself, is what "provider-agnostic" means here, and §9.2 states how to
check it by inspection.

**Not blocked.** Unit B's live check needs a local runtime installed — no secret, no spend, nothing to
procure. Measured (C40): none is installed on this machine today, so it is an implementer's step rather
than Toni's, and §14 makes it a condition of hand-off rather than a post-merge hope.

**Rejected:** an OpenAI Go SDK — measured at 5 required modules into a `go.mod` that has none (C42), far
cheaper than the 11 the Anthropic SDK wanted (C39), and **it still fails #10488's gate under
`--network none`** (C43). The size argument nearly evaporated; the structural one did not, and it is the
whole of the decision now. Also rejected: zero tools (§2.4).

---

## 1. Problem Statement

The milestone statement, verbatim from vision #10424 §9:

> "**Skeleton loop.** Input → mechanical assembly → one model call → write-back. One agent, one tool, no
> workflows. Prove the assembled context is *legible* and the loop closes."

And #10521's restatement of the same goal:

> Input → **mechanical** context assembly → **one** model call → write-back.

The write path's governing constraint is Toni's, verbatim (#10424 §5.3):

> "when an agent decides to write something back it can not lead to a decision — throw it in there."

M0 built a process that starts, serves and stops. It reads one environment variable, answers one static
route, and touches nothing outside itself. **M1 is the first milestone in which the process does the thing
the project exists to do**, and the first in which it talks to anything.

Three things are therefore true at once, and the design has to serve all three:

1. **It must close the loop.** Input in, record out, graph changed.
2. **It must be legible.** A human must be able to read what the model was given, and — critically — what
   it was *not* given. A context assembler whose cut decisions are invisible is the transcript problem
   wearing a new hat.
3. **It must not hand milestone 2 an impossible job.** The retrieval eval harness scores
   `input → node ids that must be in scope`. If M1 records only what it kept, recall is uncomputable
   forever after, because the misses were never written down.

### Success criteria

| # | Criterion | How it is judged |
|---|---|---|
| S1 | The loop closes | A run posted to `POST /runs` returns a record and the graph holds a new node linked to the run's subject. Judged live, in a container, against a local OpenAI-compatible runtime — no credential, no spend (§10.6, §14 step 14) |
| S2 | The assembled context is legible | The response body names every candidate the query returned, with its score, rank, size, content hash, and **included or cut with the reason**. A human reads it without opening the graph |
| S3 | No LLM chooses what is retrieved | The retrieval query is the input text, verbatim, and the subject id comes from the request. Falsifier: any model output feeding the assembler's query. The supplementary-recall tool is *additive* and post-assembly (§2.4) |
| S4 | Assembly is deterministic and byte-pinnable | Given a fixed candidate set, the assembled block is one exact string, asserted byte-for-byte offline (§9) |
| S5 | Milestone 2 is not foreclosed | The record carries the **whole** candidate set with scores and hashes, not the surviving subset (§9.4) |
| S6 | The configuration boundary survives | Still exactly one environment-read site, in `main` (#10437 §8.1, #10466). Falsifier: a second lookup anywhere in the module |

---

## 2. Scope & Non-Scope

### 2.1 In scope

- One input surface: `POST /runs`, synchronous, one input to one turn.
- Mechanical context assembly: two graph reads, a byte budget, a deterministic render order.
- One model call per turn, with one tool and a hard cap on calls per run.
- One model adapter, speaking the OpenAI-compatible chat-completions protocol, behind a port whose
  vocabulary is the loop's own (§8.3, §9.2).
- Write-back of a run record, structured entirely by the harness.
- Five new boot-configuration members, each with a live consumer.
- The **error envelope**, which M0 §8.5 deferred to "the first endpoint that can fail". This is it.
- The offline test suite, and the documented live verification.

### 2.2 Out of scope — declined explicitly, not by omission

| Excluded | Where it belongs |
|---|---|
| Working set, episodic buffer, semantic recall as **named tiers** | #10424 milestone 3 |
| Hybrid retrieval (lexical, trigram, graph expansion, structural pins) | milestone 3; DiVoid-side growth, #10424 §6 |
| Supersession demotion, decision trails, validity intervals, type-scoped half-lives | milestone 3 |
| The retrieval eval corpus and recall@k | milestone 2. M1's obligation to it is §9.4, and nothing more |
| An `intent` field on the model's result | milestone 3 — it is a query for the *next* step's assembly, and M1 has no next step. Declaring it now is precisely #1220 §2's violation |
| Workflows, gates, obligations, affordances, step resolution | milestone 4 |
| Consolidation / the memory core as a component | milestone 5. M1 builds the *port* it will stand behind (§5.3) |
| Run explorer UI | milestone 6 |
| Roles, hand-off, multi-agent | milestone 7 |
| Prompt caching, cache breakpoints, and reading back any cached-token count | #10424 round 3's sequencing call, verbatim: *"find out about the effect first … then optimise"*. The **one** caching decision banked now is stability ordering (§6.3), because it is free today and expensive to retrofit. **Stated without a provider's member names**, because the mechanism differs per provider and naming one here would be declaring a vocabulary the ruling forbids |
| A second provider, and any member that would select between providers | M1 ships **one** adapter, so a selection has nothing to select between. "In the end we need more providers" is a direction, not a licence to declare their vocabulary now (#1220 §2). §12 names the seam the second one arrives through |
| Streaming responses | No consumer. The caller is a human with `curl` |
| Retry, backoff, rate-limit handling on either external call | §10.5. One attempt, error surfaced |
| Idempotency keys | §10.7 |
| `/health` reporting on the new dependencies | §10.8 |
| Reading the system text from a graph node | §8.6 — the deliberate, stated departure from #10424's inversion 3, with its seam named |

**The named future list, in prose, as #1220 §2 requires — these are not members, and no implementer
declares them in this milestone:** working set, episodic buffer, tier labels, supersession edges and
demotion, decision trails, freshness half-lives, `intent`, workflow steps, obligations, affordances,
gates, consolidation, roles, hand-off, cache breakpoints, epochs and re-bases, relevance hysteresis,
provider selection, per-provider request options, streaming.
**Each arrives with the milestone that first reads it.** The seams, not the members, are this milestone's
deliverable, and §12 enumerates them.

### 2.3 Ruling on the unit split — **two units, two PRs**

M1 is one milestone but not one PR. The split is not arbitrary; it falls out of two things that are both
true independently.

| Unit | Contents | Independently valuable because | Boot members it brings |
|---|---|---|---|
| **A — assembly** | `internal/divoid` read side, `internal/loop` assembly, `POST /runs` returning the record with the assembled block and **no model call, no write-back** | It *is* success criterion S2. It answers "is the assembled context legible" — the milestone's own bar — with no model of any kind in the picture | DiVoid base URL, DiVoid API key |
| **B — the turn closes** | `internal/openaicompat`, the model port, the tool cycle, `internal/divoid` write side, run-record write-back | It closes the loop (S1) on top of a shipped, readable assembler | Model endpoint URL, model id, model API key (optional) |

**The ruling weakened one of this split's two justifications, and it survives on the other.** As written,
the split rested on (i) each unit carrying its own configuration members with their consumers, and
(ii) unit B being blocked on a credential that did not exist. **(ii) is now false**: there is nothing to
procure, and unit B's live check needs only a local runtime (C40). Re-run honestly, the split still holds:

- **(i) is untouched, and it was always the stronger reason.** Unit A dereferences the graph URL and key;
  unit B dereferences the model endpoint, model id and optional key. Nothing is declared before it is
  read — #1220 §2 satisfied per unit, not merely per milestone.
- **(iii), which was implicit and is now doing real work:** unit A *is* S2. It is independently valuable
  because it answers the milestone's own legibility question with no model involved at all, and that
  experiment (§6.2's falsifier — ten real runs, read the cut column) is worth running before the model
  call exists to confound it.
- **What (ii) has shrunk to:** unit B's live check requires a runtime an implementer installs, not a
  secret a person supplies. That is a smaller claim and it no longer justifies a split on its own. It is
  recorded here because a reader who inherited the old sentence would over-weight it.

Unit A is already implemented and in review, so the split is settled in fact as well as in argument.
A later PR that depends on the earlier one references it in its body. The orchestrator owns that (#10192).

### 2.4 The "one tool" ruling, and the ambiguity behind it

**The milestone statement says both "one model call" and "one tool". If the tool is ever used, there are
two model calls.** That is a genuine ambiguity in #10424 §9, not a reading error, and it is resolved here
with a position rather than left for the implementer to guess.

**Position: "one model call" describes the *shape* — one judgement step per input, not a chain of
orchestrated steps — and the tool cycle happens inside that step.** M1 ships one tool.

**What the tool is:** *supplementary recall* — a semantic query against the graph. It is the **same
operation the assembler already performs**, exposed to the model for what assembly missed. It adds no
external surface and no adapter method.

**Why it earns its place, stated as math rather than as enthusiasm.** Run the delete test honestly first:
delete the tool, and S1 and S2 still hold — the loop still closes, the context is still legible. So the
tool is **not** load-bearing for the milestone's stated bar, and a design that claimed otherwise would be
rationalising.

What it *is* load-bearing for is the thesis. #10424 §5.2 states the project's own most dangerous failure
mode — *"a confident agent with a clean, plausible context missing the one node that mattered"* — and
prescribes that *"mechanical retrieval is the FLOOR, not the ceiling"* and that *"recall is measured"*. A
milestone that closes the loop and provides no way to observe whether the floor was ever too low has built
the demo and skipped the experiment. The tool converts that into a counted event: **per run, did the model
need more than assembly gave it, and what did it ask for?** — which is the raw material milestone 2's
corpus is built from.

- **Cost:** one tool definition, one dispatch branch, one loop-cap constant, one rendering rule (§6.4).
  No new external call, no new port method.
- **Value:** the only day-one instrument for the project's own stated failure mode.
- **Rejected alternative — zero tools.** Satisfies "one model call" literally and is one constant and one
  branch cheaper. Rejected because it makes the milestone unmeasurable in exactly the dimension the vision
  says matters most. **Toni's one-line lever if this is the wrong call: drop the tool, and unit B loses
  the dispatch branch, the cap, and §6.4. Nothing else in the design moves.**

---

## 3. Measured Facts

Everything below was executed on this machine on 2026-09-01. Probes ran in
`C:\dev\claude\_scratch\sarah-m1-qz4x\` and have been removed. No personal data was read, written or
quoted; the graph reads returned this project's own notes and only ids, names, scores and byte counts are
reproduced. **The `C` numbering continues #10437's series, which ended at C18**, so a label identifies one
measurement across all three Processor designs.

### 3.1 The graph read path

| # | Fact | Consequence |
|---|---|---|
| C20 | Semantic search is a query parameter on the ordinary listing route: `GET /api/nodes?query=<text>&count=N&fields=id,type,name,status,similarity`. Measured 200 in **1.48 s / 1.93 s / 1.48 s** over three consecutive calls (2.20 s on the first, cold) | The retrieval primitive M1 needs is **one HTTP GET**. Nothing has to be reimplemented over REST (#10424 §6's split holds today) |
| C21 | Adding `content` to `fields=` returns bodies inline. 8 rows → **30,286 B** response, of which **28,491 B** are bodies | Candidates and their text arrive in one round trip. No per-node fetch loop |
| C22 | **The ranking is bit-stable.** Four identical calls returned the same six ids in the same order with `similarity` identical to nine decimal places (`10424:0.694141600 10521:0.665112850 6913:0.660652000 …`) | Retrieval is **deterministic given (graph state, query text)**. The nondeterminism in this milestone is not in retrieval — see §9 |
| C23 | `count=20` with content: **206,201 B in 1.58 s / 1.48 s**; 199,608 B of bodies. Body sizes min **2,872**, median **5,758**, max **42,978** | One round trip is affordable. **Node sizes vary 15×, so a `count` cap does not bound bytes** — a byte budget is not optional |
| C24 | On that same 20-row set, cumulative body bytes cross 40,000 **after rank 3**; a 60,000-byte budget admits ranks **1–5** (44,931 B used) and cuts **15 of 20** | Any budget in the tens of thousands **binds in production**, not only in tests. The cut path is exercised on ordinary runs |
| C25 | `query=` composes with `linkedto=` (22 rows scoped to #10423) and with `type=` | Scoping is available mechanically. §6.2 records why M1 does not use it |
| C26 | `minSimilarity=0.66` narrowed `total` from **9,929 to 4** | A score floor is available server-side. §6.2 records why M1 does not use it |
| C27 | **The deixis collapse, measured.** `query=why?` returned, in rank order: `Philosophy`, `falloutdude356's obscure probes`, `Test`, `Backend`, `gentlemandapper's non-sequitur` — nothing related to any work in flight | #10424 §5.1's claim is true and is now measured on our own graph. **M1's assembly is honest about this rather than papering over it** (§6.2, §11 R3) |
| C28 | An unscoped query pulls cross-project noise into the top 8: against *"how does the processor read configuration"*, rank 7 was `#331 Dead config: Rematching:CostCeilingUsd…` at similarity **0.634**, from an unrelated project | Precision is not free at M1. The record makes it visible; §6.2 explains why it is not filtered away |
| C29 | The subject node fetches **with content in one call**: `GET /api/nodes?id=N&fields=id,type,name,status,contentType,content` → 200, 3,913 B, **0.31 s** | The anchor costs one cheap round trip |
| C30 | **A missing id via the listing form returns `200 {"result":[],"total":0}` — not 404.** The single-node route `GET /api/nodes/{id}` *does* return 404 with `{"code":"data_entitynotfound","text":"'Node' with id '99999999' not found"}` | **A "subject not found" check must test the empty result, never the status code.** An implementer who checks for 404 on the listing form will silently accept a run with no subject |
| C31 | A bad bearer token returns `401 {"code":"authorization_invalidtoken","text":"API key not recognised"}` | Auth failure is loud and distinguishable from a not-found |

### 3.2 The graph write path

| # | Fact | Consequence |
|---|---|---|
| C32 | Writing one node is **three POSTs**: create (`POST /api/nodes` → `{"id":10525,…}`, **0.46 s**), body (`POST /api/nodes/{id}/content` with the content type as a header, **0.27 s**), link (`POST /api/nodes/{id}/links` with the bare target id as the body, **0.13 s**). `DELETE /api/nodes/{id}` removed it and a refetch returned 404 | The write is **one port operation over three HTTP calls** (§8.3). The three-call shape is an adapter detail the loop never sees |
| C33 | The node created in C32 was assigned canvas coordinates automatically once linked | Nothing in M1 sets position. Recorded so nobody adds a member for it |

### 3.3 The model call — **superseded in scope by the ruling; retained as the record**

C34–C37 were measured against `api.anthropic.com`, **which M1 no longer calls.** They are still true and
they are kept here rather than deleted, because a decision propped up by a cost nobody can re-read is a
decision nobody can re-examine. **None of them is load-bearing any more**, and each row's consequence is
restated to say what it now is:

| # | Fact (unchanged) | What it means after the ruling |
|---|---|---|
| C34 | **No Anthropic credential exists on this machine.** `ANTHROPIC_API_KEY` unset, `ANTHROPIC_AUTH_TOKEN` unset, `ANTHROPIC_BASE_URL` unset, `ant` not installed, `~/.claude/secrets/` contains no Anthropic key | **No longer a blocker and no longer relevant.** M1 does not need this credential. §13.1 is withdrawn and §9.2's seam argument, which leaned on this fact, is re-derived rather than repaired |
| C35 | `api.anthropic.com` is reachable from here. `POST /v1/messages` without a key returned **401 in 0.31 s** with `{"type":"error","error":{…},"request_id":"req_…"}` | Retained only as evidence that egress from this machine works. The route, the header and the `request_id` are one provider's and M1 does not depend on them |
| C36 | The Anthropic request contract, confirmed at the header level by C35 | Off the critical path. C41 is its replacement for the surface M1 actually sends |
| C37 | **NOT measured:** the Anthropic **200** response shape | Off the critical path. Its successor — the OpenAI-compatible 200 shape — is **also** documented rather than measured, but unlike C37 it is measurable *before* unit B is handed off (§9.5, §14 step 14) |

### 3.3b The model call, as the ruling shapes it — measured 2026-09-01

| # | Fact | Consequence |
|---|---|---|
| **C40** | **No OpenAI-compatible endpoint runs on this machine, and no local runtime is installed.** `ollama`, `llama-server`, `lms`, `vllm`, `koboldcpp` all absent from `PATH`; no `~/.ollama` and no install directory; `GET /v1/models` on `127.0.0.1:11434`, `:1234`, `:8000`, `:8080` all returned no response, and none of those ports is listening | **The live check is not blocked, it is unstarted.** What it needs is a runtime an implementer installs — no secret, no spend, nobody to ask. That is a materially different obligation from a credential, and §14 makes it a hand-off condition rather than a wish |
| **C41** | The auth contract, measured against the reference implementation: `POST https://api.openai.com/v1/chat/completions` with a body and no credential returned **401 in 0.30 s** with `{"error":{"message":"You didn't provide an API key … using Bearer auth (i.e. Authorization: Bearer YOUR_KEY) …","type":"invalid_request_error","param":null,"code":null}}` | The route is `POST {base}/chat/completions` and the credential, **when there is one**, travels as `Authorization: Bearer`. Also the error envelope an adapter may see: an object under `error` with `message`/`type`/`param`/`code` — note it carries **no request id**, unlike C35 (§10.3) |
| **C42** | `go get github.com/openai/openai-go` resolves to **v1.12.0** and, after `go mod tidy` against a file that imports it, leaves **5 required modules** (1 direct, 4 indirect: `tidwall/gjson`, `match`, `pretty`, `sjson`), **12 `go.sum` lines**, and **18 modules in `go list -m all`** | **Less than half the Anthropic SDK's footprint** (C39: 11 / 38 / 65). The "you would start having a dependency tree" argument is much weaker for this candidate and §10.9 no longer leans on it |
| **C43** | **The gate today**, re-run on the shipped unit A: `docker run --rm --network none -v <repo>:/src:ro -w /src golang:1.27 sh -c 'go vet ./... && go test -count=1 ./...'` → **four `ok` lines, exit 0, 9.96 s**. **The same gate on the tree that requires `openai-go`:** fails — `dial tcp: lookup proxy.golang.org … network is unreachable`. **With the network on:** passes in **12.6 s** | **The decisive fact, and it is structural rather than quantitative.** The cold penalty is **+27 %**, not the +90 % C38 measured for the larger SDK — but `--rm` means no persistent module cache, so *any* `require` line, of any size, converts a gate that needs **no network at all** into one that does. The first `require` is the whole cost; the tree behind it is rounding |
| **C44** | `ollama/ollama:latest`, linux/amd64: **4 layers, 3,226 MiB compressed**, before any model weights are pulled | What putting a local runtime *inside* the gate would cost, if the nondeterminism argument had not already ruled it out (§10.6) |

### 3.4 The dependency question — measured on the gate that already exists

The first hypothesis this document had to weaken. **Revision 2 weakened it a second time; see the note
at the end of this section and §10.9.**

**Was going to say:** *"a third-party dependency breaks #10488's container gate, because the working tree
is mounted read-only."* **Measured: false in that form.** The module cache lives in the container's own
writable layer at `/go/pkg/mod`, not in the mount, so `go vet` + `go test` run fine under `:ro` with a
`require` block present. What the read-only mount actually blocks is `go get`, which rewrites `go.mod`
(`go: updating go.mod: open /src/go.mod: read-only file system`) — an author-time operation, not a gate.

The true cost is different, and smaller, and still decisive:

| # | Fact | Consequence |
|---|---|---|
| C38 | **The gate today, with `--network none`:** `docker run --rm --network none -v <repo>:/src:ro -w /src golang:1.27 sh -c 'go vet ./... && go test -count=1 ./...'` → two `ok` lines, exit 0, **9.9 s** wall. **The same gate on a tree that requires the Anthropic Go SDK:** fails — `dial tcp: lookup proxy.golang.org … network is unreachable`. **With the network on:** passes in **18.9 s** cold | A dependency converts a gate that needs **no network at all** into one that does, and roughly **doubles its cold wall clock**. #10488's venue uses `--rm`, so there is no persistent module cache to amortise this |
| C39 | `go get github.com/anthropics/anthropic-sdk-go` resolves to **v1.69.0** and writes **11 required modules** (1 direct, 10 indirect), **38 `go.sum` lines**, and **65 modules in `go list -m all`**, into a `go.mod` that today has **no `require` block whatsoever** | The step is not "add a dependency". It is "start having a dependency tree" |

**The remedies exist and each is itself a change to a contract that shipped this week:** vendoring puts
the dependency's source in the repo; mounting a module cache changes #10488 §8.1's venue command and
makes the gate carry state. Neither is free, and neither was priced by anyone.

**Revision 2 note.** C38 and C39 were measured against the Anthropic SDK, which is no longer the
candidate. **They were re-measured against the one that is** — C42 and C43 — and the result moved the
argument's centre of gravity: the module-count case shrank by more than half and no longer carries
anything, while the `--network none` case came back **identical**, because it does not depend on the
dependency's size at all. §10.9 is re-derived on C42/C43 and no longer quotes C39's numbers as its
reason.

**What would re-open this:** a feature in M1's scope that raw HTTP genuinely cannot carry — streaming SSE
parsing, cache-breakpoint placement, or a response shape too irregular to decode by hand. M1 sends one
non-streaming POST and decodes one message object. See §10.9 for the full comparison, including the
counter-authority.

---

## 4. Architectural Overview

```
  POST /runs  {input, subject}
        │
        │  cmd/processor constructs the graph client, the model client,
        │  the system text, and passes them DOWN as values (M0 §8.1 rule)
        ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │ internal/loop — one turn, no I/O of its own                          │
  │                                                                      │
  │  [1] ASSEMBLE  (pure, no LLM, no clock, no randomness)               │
  │        ├── graph.Node(subject)      ──────► the ANCHOR   (C29)       │
  │        └── graph.Recall(input, 20)  ──────► CANDIDATES   (C20/C23)   │
  │              ├─ hash + size every candidate                          │
  │              ├─ admit in SCORE order until the byte budget is spent  │
  │              └─ render admitted set in ID order  ◄── stability, §6.3 │
  │                                                                      │
  │  [2] CALL      one judgement step, ≤ maxModelCalls                   │
  │        model.Judge(system, block, input, recall-available)           │
  │        while reason == WantsRecall and calls remain:                 │
  │            graph.Recall(requested query) ─► result at the TAIL       │
  │                                          never inside the block      │
  │        (reason is the LOOP's closed set, never a wire string)        │
  │                                                                      │
  │  [3] RECORD    the harness structures it; the model contributes prose│
  │        graph.WriteRun(record)  ──► one node + one edge to the subject│
  └──────────────────────────────────────────────────────────────────────┘
        │
        ▼
   200 + the run record  (input · query · anchor · EVERY candidate with
                          score, rank, hash, size, included|cut · the
                          assembled block · the answer · usage · node id)

  internal/divoid          internal/openaicompat
  ├─ Node(id)              └─ Judge(...) → neutral judgement
  ├─ Recall(query, limit)     POST {base}/chat/completions   (C41)
  └─ WriteRun(record)         Authorization: Bearer — only if a key is set
     (three POSTs, C32)       net/http, no SDK  (C42/C43)
```

**Two external processes, two ports, one direction.** The loop depends on two narrow contracts it
declares itself and never constructs. `cmd/processor` constructs the real implementations. Nothing below
`main` reads the environment. That is M0's rule (#10437 §8.1) and #10466's archetype C applied unchanged —
this milestone adds the first real dependencies the module has ever had, and it adds them through the
door M0 built for exactly this.

**The overview's load-bearing line is `[1]`.** Everything the project is arguing lives in the fact that
step 1 has no model in it and no branch a model can influence.

---

## 5. Components & Responsibilities

### 5.1 `internal/loop` — the turn, and the assembler inside it

**Owns:** the run record's shape; the assembly function; the turn's sequencing and its call cap; the
translation of external failures into run outcomes.

**Does not own:** HTTP transport of any kind, JSON wire formats of either external service, the route
table, exit codes, configuration lookup, or the graph's node vocabulary.

Two responsibilities, one package, deliberately:

- **Assembly** is a pure function: `(anchor, candidates, budget) → (block, per-candidate disposition)`.
  No I/O, no clock, no randomness, no globals.
- **The turn** is the sequencer: fetch, assemble, call, dispatch tools, record.

**Why one package and not two.** Run the merge test in both directions. *Split assembly into its own
package:* it would be a package for one pure function called from one place — indirection, not
abstraction (#1136 §4). *Merge the loop into `internal/server`:* the loop's tests would then need HTTP
recorders to assert a pure function's output string, and the package would own two unrelated things.
**One package, two files.**

### 5.2 `internal/divoid` — the graph adapter

**Owns:** URL construction, bearer authentication, request/response encoding for three operations, and
the three-call write sequence (C32). Translates transport and status failures into typed outcomes,
including the C30 trap: **subject-not-found is an empty result set, not a status code.**

**Does not own:** which query to run, what to keep, what a run record means, or when to write.

**Named for what it is** (#6836). It is a client for one specific external system, not a generic "graph".

### 5.3 `internal/openaicompat` — the model adapter

**Owns:** the OpenAI-compatible chat-completions request body; the `Authorization: Bearer` header **when
and only when a key was configured** (C41); decoding the response's message object; and — the part that
matters most — **translating that provider's vocabulary into the loop's** (§8.3): a finish reason string
into the loop's closed terminal-reason set, a requested function call into a supplementary-recall request
with a query, and a missing usage object into *absent* rather than zero. Standard library only.

**Does not own:** the system text, the tool's semantics, the loop, retries, or the cap.

**Named for the protocol, not the vendor** (#6836 applied deliberately, and differently from §5.2). Its
sibling `internal/divoid` is named for one specific external system, because that is what it talks to.
This package talks to **a wire protocol that many unrelated systems serve** — local runtimes, gateways,
and OpenAI itself — so naming it `openai` would name the one server it is *least* likely to be pointed at
in this project, and naming it `model` would name its role rather than what it is, and would have to be
renamed or turned into a parent the day a second protocol arrives. `openaicompat` is the honest name and
it leaves the sibling slot free.

**What it may not do, stated as a rule because it is the ruling's whole content:** it may not leak a
provider's field names, finish-reason strings, or error shapes past its own boundary. §9.2's second
falsifier is how a reviewer checks that by inspection.

### 5.4 `cmd/processor` — unchanged in shape, larger in content

Gains: five configuration members read at the existing single site; construction of the two adapters and
the system text; passing them into the handler builder. Loses nothing. **`main` is also where the choice
of model adapter lives** — today by construction rather than by selection, because there is one.

### 5.5 `internal/server` — one new route

Gains `POST /runs` and, per #10466 archetype C, **the first parameter on the handler-building function**.
The rule #10466 states — *"the first dependency arrives as a parameter, never as a package-level
variable"* — is discharged here for the first time.

Owns: request decoding, status-code selection, the error envelope (§8.5). Owns no policy.

### 5.6 What is deliberately NOT a component

| Not built | Why |
|---|---|
| A memory core | Milestone 5. M1 builds the **port** it stands behind: the loop says *what happened*, the adapter decides type, name and edge (§5.3 of #10424 made operational at M1 scale) |
| A retrieval-policy or ranking component | There is one query and one budget. A component for that is a name, not a boundary |
| A `RunStore` / repository abstraction | The graph port already has the one write operation |
| **A provider registry, factory, or protocol enum** | The obvious over-build the ruling invites, and the one to refuse. *"In the end we need more providers"* is satisfied by the port (§8.3) plus `main` constructing one — a registry with one entry is a lookup table nobody reads. **The second provider brings the selection with it**, and will know what it needs to select on; today's guess would not (#1220 §2) |
| A queue, worker pool or scheduler | The run is synchronous and the caller waits |
| Middleware of any kind | M0 §10.4's exclusion stands; nothing here needs it |
| An `internal/config` package | M0 §5.3 inlined it into `main` and nothing changed that |

---

## 6. Interactions & Data Flow

### 6.1 The run, end to end

1. **Decode.** `input` (non-empty text) and `subject` (a node id). Either missing or malformed → `400`.
2. **Anchor.** One graph read by id **with content** (C29). Empty result → `404` (C30's trap: test the
   result, not the status).
3. **Recall.** One semantic query. **The query text is the input text, verbatim.** No rewriting, no
   expansion, no summarisation, no model. Limit: the candidate constant (§8.4).
4. **Hash and size** every candidate returned — *every* one, including the ones about to be cut.
5. **Admit** in descending score order every candidate whose body fits the budget still unspent.
   **CORRECTED 2026-09-05 (#11335, E12): the budget the walk starts from is `AssemblyByteBudget` minus the
   anchor's body, floored at zero — the anchor is charged before the first candidate is considered, and is
   still rendered whole in step 6 whatever its size (§6.3, §11 R4).**
   ~~The first candidate that would exceed it is cut, **and so is every candidate after it** — see §6.3
   on why the rule is a stop, not a skip.~~ **CORRECTED 2026-09-04 (#11158, B1): a candidate that does
   not fit is cut and the walk continues, so it costs its own slot and nothing else. A candidate this
   system produced is cut before the byte test and charges nothing (§6.3).**
6. **Render** the block (§6.3).
7. **Call** the model once: system text, the block, the input, the tool definition.
8. **Dispatch** tool uses (§6.4) until the model stops asking or the call cap is reached.
9. **Record.** One node, one edge, written by the adapter (§8.3).
10. **Respond** `200` with the record.

**Steps 1–6 contain no model and no branch a model can influence.** That sentence is the milestone.

### 6.2 Why the query is the raw input, and why nothing filters it

Three mechanical narrowings are available and measured, and **M1 uses none of them**:

| Available | Measured | Why not at M1 |
|---|---|---|
| `linkedto=<subject>` | C25 — composes; 22 rows scoped to #10423 | It is a one-hop filter. The best node for an input is routinely two hops away, and a one-hop scope is a *policy* milestone 3 replaces with graph expansion from pins. Building it now means unbuilding it then |
| `minSimilarity=<floor>` | C26 — 9,929 → 4 | A floor with no scored corpus behind it is a guess. Milestone 2 is where floors become measurable; choosing one now would be a number nobody can defend |
| Rewriting the query with a model | — | **Forbidden by the thesis.** A model call that chooses what to look up is the thing this project is a reaction against |

**So M1 retrieves against the whole graph and records what comes back, including the noise.** C28 shows
what that costs: a node from an unrelated project at rank 7. C27 shows the worse case: a deictic input
retrieves nothing related to anything.

**This is a deliberate choice to make the problem visible rather than to hide it behind an unmeasured
filter.** Milestone 2 cannot score a retriever whose failures were filtered out before they were written
down. The alternative — ship a scope filter now — would produce cleaner-looking runs and a corpus that
systematically under-represents exactly the failures the eval exists to find.

**Falsifier for the claim that this is the right call:** if the first ten real runs show that noise
crowds out the correct node so consistently that the model cannot answer at all, the floor or the scope
becomes necessary before milestone 2, and this section is wrong. That is a cheap experiment and unit A
exists to run it.

### 6.3 Rendering — stability order, not relevance order

**The rule: ranking decides *entry*; a deterministic total order decides *position*. The assembled block
is sorted by node id ascending and never by score.**

This is #10424 round 2's rule, and round 3 states why it is the one caching decision to bank on day one:
it is *"nearly free to get right … and expensive to retrofit"*. It costs one comparator. Relevance-sorting
would reshuffle the same nodes on every step, so nothing before the change point is ever a cache read.

**But the cache is not why M1 does it.** M1 has one step per run and no cache to preserve. The reason
that binds *today* is §9: a block sorted by id is **byte-identical across reruns for a fixed candidate
set**, and is therefore assertable byte-for-byte in an offline test. A block sorted by score is identical
too — right up until two candidates tie or a score shifts in the ninth decimal, at which point the golden
test is flaky for a reason that has nothing to do with the code. Id order is a **total** order over
distinct nodes; score order is not.

Round 6's continuity-of-attention argument is the third reason and it is recorded here only as prose:
*"a context which reshuffles completely every step produces a thought process that reshuffles completely
every step."*

~~**Why the budget stops rather than skips.** When a candidate does not fit, admission stops; smaller
lower-ranked candidates are **not** back-filled. Back-filling optimises byte utilisation at the cost of a
non-monotone admission rule — "included" would no longer mean "outranked everything excluded", and the
record's cut column would stop being readable as a ranking. Cost of the simple rule, measured on C23's
distribution: with a 42,978 B node at a good rank, most of the budget is spent on it and the rest is cut.
That is legible; the alternative is not. **Falsifier:** if the record repeatedly shows a large node
starving a run that a back-fill would have saved, the rule is wrong — and the record is what shows it.~~

**STRUCK 2026-09-04 (#11158, B2). The falsifier above fired — twice, on two records eight weeks apart in
the same graph — and it is the authority for the change, not any fresh argument against the rule:**

| Date | Record | Rank-1 size | Budget consumed | Would a back-fill have saved it? |
|---|---|---|---|---|
| 2026-09-02 | #10897 replayed (`docs/architecture/m2-retrieval-eval.md` §5.2) | 70,660 B | 0 of 60,000 | Yes — every remaining candidate was cut without one byte admitted |
| 2026-09-04 | #11138 live (#11141) | 65,409 B | 0 of 60,000 | Yes — ranks 2 and 3 were 5,710 B and 2,470 B |

**CORRECTED 2026-09-05 (#11335, E12) — what the walk below spends is not the whole budget.** The anchor's
body is subtracted from `AssemblyByteBudget` first, floored at zero, and the three rules that follow run
against that remainder. Nothing else about the walk changes: it is still one pass, still latch-free, still
`<=` at the boundary, and the anchor is still rendered whole and never cut (§11 R4). The two facts are
independent — **the anchor is charged but not cuttable** — and keeping them apart is what makes the
invariant a bound with a residual rather than a ceiling:
`content = anchorSize + admittedBytes ≤ max(anchorSize, AssemblyByteBudget)` — **content bytes, not the
rendered artifact.** The render adds a framing term that no budget sees; it is stated with the block layout
at the end of this section.

**The admission rule, as it now stands.** Walk the candidates once, in the order recall returned. For
each, record its disposition exactly as before — rank, id, type, name, similarity, size, hash — and then,
**in this order**:

1. if the candidate is **self-produced** — a run record this system wrote — cut it with its own reason,
   **do not** charge its size, continue;
2. else if the cumulative admitted size plus this body still fits the budget, admit it, charge it,
   continue;
3. else cut it with the byte-budget reason, **do not** charge its size, continue.

There is no latch: the walk always visits every candidate. The boundary stays inclusive (`<=`).

**Order 1-before-2 is load-bearing.** A run record that happened to fit must still be reported as
self-produced, or the record's own account of why it cut something becomes wrong in the field that says
why (`docs/architecture/run-record-fate.md`'s thesis: a record accounts for its composition).

**Conceded, explicitly:** `included` now means *"outranked everything excluded **that was also small
enough**"*, not *"outranked everything excluded"*, and admission is non-monotone in size. The half of the
struck paragraph that does **not** survive is its second clause: the block is rendered sorted by node id
and never by score (the rule directly above), and `candidates[]` preserves rank order independently of
admission, so no reader ever saw a rank prefix in either artifact. **The cut column becomes more
accurate, not less** — before this change a shutout record carried nineteen candidates claiming
*"byte budget exceeded"* that were never measured against a budget.

**Why self-produced content is cut at admission rather than filtered from the query.** §6.2's three
narrowings stay unbuilt and §11 R13's *"one query parameter"* remedy is **rejected as a mechanism** while
its goal is adopted: filtering removes the row before it is written down, which is the one thing
milestone 2 forbids (§9.4 obligation 1), and node type alone cannot carry the distinction because run
records are typed `session-log` and so are human-written session logs — two of them were ranks 2 and 3 in
the failing run. Cut at admission, the row is still retrieved, still ranked, and still recorded with its
similarity and a cut reason of its own, so the evidence about how self-produced content competes with
real content accumulates in every record written from here on. **Self-recall is ruled a capability worth
having; the run record is the wrong vehicle for it** (#11158 §4).

**Block layout, fixed:**

```
  1. the ANCHOR      — the subject node: id, type, name, full body
  2. the CANDIDATES  — admitted set, ascending by id, each: id, type, name, body
```

The anchor first because it is the run's stable subject; candidates second because they are the volatile
part. That is the same stability gradient milestone 3 generalises, expressed with the two things M1 has.

**What the layout costs, and it is outside every budget — recorded 2026-09-05 (#11335, E12).** Those two
lines are section banners, not free: each section carries a banner line and `id:` / `type:` / `name:`
headers, and those bytes reach the model without passing any budget. The artifact `Assemble` returns is
therefore two terms, not one:

```
  len(block) = anchorSize + Σ admittedBytes + framing(1 + admittedCount)
               └──────── ≤ max(anchorSize, blockBudget) ────────┘
               framing 178–2,444 B on this corpus (mean 1,384 B), unbudgeted
```

**Measured across the 23-row sweep:** about 178 B for the anchor section, and 120–190 B per admitted
candidate, worst on a thirteen-candidate row. At `AssemblyByteBudget = 60,000` that is under 4% of the
block, and the honest reading is *a second, small residual*. **It stops being small when the budget is
swept downward** — which is exactly what #11365 F5 proposes, to answer #11364's floor question. §11 R4
carries that arithmetic and why it lands on the next unit rather than this one.

### 6.4 The tool cycle, and where a tool result goes

The tool takes a query string and returns ranked hits — the same operation as step 3, with the model's
words instead of the input's.

**A tool result is appended to the message tail as a tool-result message. It never enters the assembled
block.** How that message is spelled on the wire is the adapter's business (§6.6); *that it goes at the
tail* is the loop's. Two reasons, and the second is the one that matters:

1. The block is rendered once per run, before the first call. Re-rendering it mid-turn would invalidate
   everything a cache could ever hold — the failure mode #10424 round 5 describes.
2. **It is the honest place.** The block is what *memory* selected; a supplementary hit is what *the
   model* asked for. Merging them would destroy the one distinction the run record exists to show, and
   milestone 2 would be unable to tell mechanical recall from model-driven recall in its own corpus.

**Bounds and outcomes:**

- A constant cap on model calls per run (§8.4). Reaching it is not an error: the turn ends, and the record
  says the cap was reached — **explicitly, as its own field** (§8.2), not by a derivation that needs a
  constant the record does not carry.
- A malformed tool input returns an error-flagged tool result and the turn continues; it is counted, not
  dropped.
- Every supplementary query, and every node it returned, is recorded — that is the measurement the tool
  exists for (§2.4). **Returned, not admitted:** §6.4a admits a subset into the prompt, and the record
  carries every row either way with its admit-or-cut decision, exactly as `candidates[]` does.
- **A byte budget on the result itself — §6.4a.**

### 6.4a The supplementary result is under a budget — the same one discipline, on the second path

**New in revision 3, from #10821 CF-4.** Revision 2 said where a tool result goes and never said how big
it may be. Measured on C23's own 20-row set, the answer was **199,608 bytes of bodies in a single round**
— 3.3× the entire assembly budget, 6.6× across the two rounds the call cap allows — injected into the
same prompt the budget exists to keep small.

**The defect is not that one path was missing a number. It is that no path had an absolute ceiling.**
§8.4 justified the assembly budget as *"roughly 1.5% of the model's window"*, and under the ruling there
is no such window: every endpoint has a different one and none of them is known at design time. **A
budget stated as a fraction of an unmeasured quantity bounds nothing and cannot be checked** — which is
exactly how a path shipped at 3.3× over it with nobody able to say so. §8.4 now states the ceiling in
bytes; this section spends part of it.

**The rule, and it is §6.3's rule with nothing added:**

| Aspect | The assembled block (§6.3) | A supplementary result (this section) |
|---|---|---|
| Budget | `AssemblyByteBudget`, per run | `SupplementaryByteBudget`, **per round** |
| Admission | Rank order, ~~stop at the first that does not fit, **no back-fill**~~ **CORRECTED 2026-09-04 (#11158, B3): skip what does not fit and continue; cut self-produced rows first** | Identical — both paths call the one `admit` (§6.3) |
| Position | Sorted by node id ascending | **Rank order as returned** |
| Exemption | ~~The anchor (§11 R4)~~ **CORRECTED 2026-09-05 (#11335, E2): the anchor is exempt from being *cut*, not from being *charged*. It is subtracted from the budget before the first candidate is considered, and only then rendered whole (§11 R4)** | **None** |
| Recorded | Every row, admitted or cut, with the reason | Identical (§8.2) |

**Why the position differs, and why that is not an inconsistency.** §6.3 sorts by id for one reason that
binds today — a block byte-identical across reruns is assertable byte-for-byte — and one that binds
later: a stability order is what a cache can hold. **Neither applies here.** A tool result is rendered
once, for one call, and is never re-rendered or reused, so there is nothing to keep stable across runs;
and rank order *is* a total order over distinct ids given C22's bit-stable ranking, so it is just as
assertable. What rank order additionally buys is that the model reads the best answer to its own question
first. Sorting a tool result by id would spend that for nothing.

**Why there is no exemption.** ~~The anchor's exemption from the assembly budget is this design's one
unbounded input (§11 R4), and it is the shape of the defect this section exists to fix.~~
**CORRECTED 2026-09-05 (#11335, E3): the premise moved and the argument did not.** The anchor is no longer
an unbounded input — it is charged against `AssemblyByteBudget` before any candidate is considered (§11 R4)
— so *"this design's one unbounded input"* is false from that date. What the anchor still has is the right
to be rendered whole however large it is, and **that** is the shape this section refuses to copy: a single
row admitted regardless of size, ahead of everything it displaces. The distinction is the whole of the
correction — the anchor earns it by being the run's subject, and no recall hit is. Repeating it here
— *"admit the best hit whatever its size"* — would reintroduce it on the very path that demonstrated why
it matters. **If nothing fits, the round admits nothing:** the model is told the round produced nothing
usable, and the record carries all twenty rows cut with the reason. That is an unhelpful round; it is not
a dishonest one, and the record is what makes the difference readable. **Falsifier:** if records
repeatedly show rounds admitting zero because the single best hit is larger than the budget, the budget
is too small — and the number to change is in §8.4, with the evidence in hand.

**The error branch is under the same bound.** A round that failed renders its error instead of its rows,
and that text reaches both the model's prompt and the written record. It is **bounded to a short sentence
and carries no address**. §8.5's rule — no key, no URL, no upstream body — was written for the
caller-facing envelope; it applies here for the same reason and one more, because these two surfaces are
read by a model and written to a shared graph rather than returned to the operator who owns the
deployment. (#10821's W-6 reaches the same site from the implementation side. This is the rule that fix
should be made against, not a second instruction.)

**Per round or per run — both, because the call cap makes them one instrument.** They are genuinely
different bounds with different failure modes, and the reason a single number settles it here is
specific: **the conversation is rebuilt from scratch on every call, so tool results accumulate.** The
third call's prompt carries both earlier rounds, not just the last one, so a per-round bound on its own
would bound a quantity nobody reads. But the number of rounds is itself capped by a constant, so a
per-round budget composes into a stated per-run ceiling:

```
  per round              SupplementaryByteBudget          =   20,000 B
  per run                × (MaxModelCalls − 1)            =   40,000 B
  with the block         + AssemblyByteBudget             =  100,000 B   ← the ceiling (§8.4)
  and the anchor on top  + |anchor|, unbounded                           ← §11 R4
```

**CORRECTED 2026-09-05 (#11335, E4).** The fourth line is struck: **the anchor is no longer on top.** It is
charged against `AssemblyByteBudget`, so the first three lines already contain it and 100,000 B is the whole
of the graph-derived prompt. The residual the fourth line used to carry is smaller and is stated exactly
once, at §11 R4: a run whose anchor alone exceeds 60,000 B renders it anyway, so the ceiling for that run is
`|anchor| + 40,000 B` rather than 100,000 B. `SupplementaryByteBudget`'s derivation below is unaffected —
it solves the 32,768-token row against the ceiling, and the ceiling's arithmetic did not move.

**The mechanism is per-round; the claim is per-run.** §8.4 states the ceiling as a literal so that a test
can assert it against the arithmetic of the three constants, and §14 requires exactly that — they cannot
drift apart from the number this design defends without something going red.

**Where the bound is applied, and it is not a free choice.** The loop admits; the adapter renders. §6.4
already says the *spelling* of a tool message is the adapter's business and that stays true — but the
*selection* cannot be, because the record must say which rows the model actually saw (§8.2, §9.4
obligation 3). An adapter that quietly truncated would leave the record claiming twenty hits for a call
that showed five, and the record would then be wrong in the one direction milestone 2 has no way to
detect. So the loop decides admission before the round is handed over, and the adapter renders what it
is given.

### 6.5 Failure paths, decided rather than defaulted

| Failure | Outcome | Why |
|---|---|---|
| Anchor read fails (transport, `401`, `5xx`) | `502`, nothing written | The run has no subject |
| Anchor not found (empty result, C30) | `404`, nothing written | Caller error, not a service failure |
| Recall fails | `502`, **no model call**, nothing written | **Assembly failure is run failure.** A model call on an empty context is precisely the confident-answer-from-nothing this project exists to prevent. Falling back to "no context" would be the single worst defect available in this milestone |
| Model call fails — transport, non-2xx, or a body that will not decode | `502`, nothing written | §10.5 — one attempt. The upstream status is logged; the upstream body is not echoed to the caller (§8.5) |
| **The endpoint does not honour part of the depended subset** — rejects the tool field, rejects the token cap, or answers in a shape that will not decode | `502`, nothing written, upstream status logged | §6.6. It is indistinguishable from any other failed call *to the caller*, and deliberately so: the operator pointed the service at an endpoint that cannot serve it, and the fix is the endpoint, not a retry |
| **The endpoint silently ignores the tool** rather than rejecting it | `200`, recorded, with **zero tool calls** | Not detectable and not treated as an error. It is indistinguishable from a model that did not need supplementary recall — which is honest, because at M1 those two really are the same observation. Named so nobody later reads a zero as proof the floor was high enough |
| **The endpoint reports no usage** | `200`, recorded with that call's usage entry **absent** | Absent, never zero. A zero in milestone 2's corpus is a measurement; an absence is the truth. **Per call, not per run** (§8.2, revision 3): on a multi-call run the calls that did report are still recorded, and a run is never reported as having measured nothing merely because its last call did not |
| Model declines to answer — the endpoint's terminal reason maps to the loop's `Refused` | `200`. It is an **outcome**, recorded with the raw reason the endpoint gave alongside the loop's neutral one | A refusal is information, not an error. The loop branches on its own reason; the record carries both (§8.3) |
| Model output was truncated — terminal reason maps to `Truncated` | `200`, recorded | The answer is prose for a human, who can see it was cut off. §8.4's token cap is what binds, and this row is how a wrong cap becomes visible |
| Call cap reached | `200`, recorded, **with `capReached` set** | §6.4, §8.2 |
| **A supplementary round admits nothing** — every hit is larger than the round's budget (§6.4a) | `200`, recorded, the round carrying all its rows **cut** with the reason | Not an error and not silence: the model is told the round produced nothing usable, and the record says it was the budget rather than the graph. The alternative — admitting one oversized hit anyway — is §11 R4's anchor exemption repeated on the path that demonstrated why it is a defect. **CORRECTED 2026-09-05 (#11335, E9): the exemption this sentence points at has narrowed and the analogy survives on the narrower half.** The anchor is charged against the budget from that date; what it keeps is the right to be rendered whole, and *that* is what admitting an oversized hit would copy — a row admitted regardless of size ahead of everything it displaces. The anchor earns it by being the run's subject; a recall hit does not |
| **Write-back fails** | **`200`**, with the failure named in the record | The expensive artifact already exists and is in the body. A `5xx` invites the caller to retry, which re-spends the model call. The graph write is the second copy, not the first. **AMENDED 2026-09-02 (#10863, A3): this row specifies the fully-failed write only and is silent on the half-succeeded one. `docs/architecture/run-record-fate.md` §6.1 carries the complete terminal-state table and §6.2 the partial-write ruling** |
| Two runs concurrently | Both proceed | The loop holds no shared mutable state. Falsifier: any package-level variable in `internal/loop` |

---

### 6.6 What "OpenAI-compatible" is depended on — the subset, stated

**"OpenAI-compatible" is a de facto standard with no specification and no conformance suite.** Every
implementation serves a different subset, and several serve supersets with the same field names meaning
different things. A design that says "we use an OpenAI-compatible endpoint" and stops has named a family,
not a contract. So M1 names the contract, and it is deliberately **small** — the smaller it is, the more
implementations satisfy it, which is the entire reason the ruling chose this protocol.

**Depended on. An endpoint that serves all of this can run M1:**

| # | Depended on | Why M1 needs it |
|---|---|---|
| D1 | `POST {base}/chat/completions` accepting JSON and answering JSON (C41) | The one call |
| D2 | `Authorization: Bearer <key>` honoured **when sent**, and the endpoint not *requiring* it (C41) | The optional-key rule (§8.1). An endpoint that requires a key still works — you set one |
| D3 | A `messages` array with roles `system`, `user`, `assistant`, and `tool` | The system text, the block, the input, and the tool-result tail (§6.4) |
| D4 | A response carrying **one** assistant message with text content | The answer. M1 reads the first choice and no other |
| D5 | A terminal reason on that choice, from *any* vocabulary | Mapped to the loop's closed set (§8.3). M1 depends on the field existing, **not** on its values |
| D6 | Function-style tool declarations, and tool calls returned with a name and an arguments payload | The one tool (§2.4) |

**Explicitly NOT depended on, so that a thinner endpoint still works:**

- **A specific set of terminal-reason strings.** M1 maps the ones it recognises and treats anything else
  as terminal-and-unrecognised, recording the raw value. An endpoint inventing its own vocabulary
  degrades one field in the record; it does not break a run.
- **A usage object.** Absent is recorded as absent (§6.5).
- **Structured refusals, content filtering, logprobs, seeds, system-fingerprints, `n > 1`, streaming,
  response-format or JSON-mode, or any caching mechanism.** None is read.
- **Arguments arriving as parsed JSON.** They arrive as a string in the reference implementation and M1
  parses them itself, which also means a malformed payload lands on §6.4's existing error-flagged path
  rather than on a new one.
- **A request id on the response or on an error** — C41 measured its absence on the reference
  implementation's own 401, so §10.3 logs one only when the endpoint volunteers it.

**The two known divergences, named with what M1 does:**

| Divergence | M1's position |
|---|---|
| The output-token cap is spelled `max_tokens` by every local runtime and by the older reference API, and `max_completion_tokens` by the newer one, which rejects the older spelling on some models | **M1 sends `max_tokens`.** It is the spelling the ruling's target — local runtimes — universally accepts, and the newer reference API is not M1's target. An endpoint that rejects it fails loudly on the first call (§6.5), which is the right direction for this error. **Falsifier:** if the first endpoints anyone actually points this at reject `max_tokens`, the constant becomes two spellings tried in order, or a member — and that is a decision to take with the failing endpoint in hand, not now |
| Text content may be `null` when a tool call is present | Decoded as absent rather than as required. A run whose only output is a tool call is normal, not malformed |

**Why this is not solved by trying harder.** The tempting alternative is capability probing: call
`GET {base}/models`, or send a trial request, and adapt. That is a second external call per boot with no
consumer, a matrix of behaviours nobody has measured, and — worst — it makes the request M1 sends
**depend on the endpoint**, which destroys §9.1's byte-exact request pinning. **M1 sends one fixed
request shape and lets a non-conforming endpoint fail visibly.** The failure is loud, immediate, names
the field, and is the operator's to fix.

---

## 7. Data Model (Conceptual)

Three entities. None is persisted by Processor; the graph is the store.

| Entity | Owned by | Lives where |
|---|---|---|
| **Node** — id, type, name, status, content type, body, and on a recall hit a similarity score | DiVoid | Read-only to M1 |
| **Run record** — the whole of one turn (§8.2) | `internal/loop` | Returned in the HTTP response **and** written as one graph node. **CORRECTED 2026-09-02 (#10899, A3): the two artifacts are the same record in every key that describes the run; the response carries one key more (the write receipt) and the stored copy carries no key for it at all** |
| **Candidate disposition** — one row per node a query returned: rank, score, size, content hash, admitted or cut | `internal/loop` | Inside the run record, on **both** paths — the assembled block's candidates and each supplementary round's hits carry the same columns, because they are the same event: graph rows admitted into a prompt under a byte budget (§6.4a, §8.2) |

**The content hash is the only field whose value accrues later, and it is included deliberately.**
#10424 §5.7 names the defect it prevents: a record of node ids rots, because the nodes change afterwards,
and *"without it the record looks precise and quietly lies."* Cost: one standard-library hash per
candidate — bodies are already in hand from C21, so there is no extra call. The alternative, adding it
when milestone 2 needs it, has a real and non-speculative cost: **every record written before that point
is permanently untrustworthy**, and those are exactly the runs milestone 2 will want to score. This is
not a "we might need it later" member (#1136 §1) — it has a writer and a reader in this milestone (the
record displays it), and the deferred value is a bonus, not the justification.

**Not modelled:** node embeddings, edges beyond the one M1 writes, node positions (C33), users, sessions.

---

## 8. Contracts & Interfaces (Abstract)

### 8.1 Boot configuration — the members M0 deliberately did not declare

M0 §8.1 named three future members in prose and declared none. **M1 dereferences them, so M1 declares
them** — each in the unit that reads it (§2.3).

| Member | Variable | Absent | Present, empty | Present | Unit |
|---|---|---|---|---|---|
| HTTP listen address | `PROCESSOR_HTTP_ADDR` | default `127.0.0.1:8080` | **error** | verbatim | (exists) |
| DiVoid base URL | `PROCESSOR_DIVOID_URL` | **error** | **error** | ~~verbatim~~ **CORRECTED 2026-09-05 (#11328): verbatim, except a path ending in `/api` (or already containing `/api/nodes`) is also an error — the client appends `/api/nodes` itself, and that shape is exactly what the operator's credentials file holds for direct REST calls** | A |
| DiVoid API key | `PROCESSOR_DIVOID_KEY` | **error** | **error** | verbatim | A |
| Model endpoint base URL | `PROCESSOR_MODEL_URL` | **error** | **error** | verbatim | B |
| Model id | `PROCESSOR_MODEL_ID` | **error** | **error** | verbatim | B |
| Model API key | `PROCESSOR_MODEL_KEY` | **no `Authorization` header is sent** | **error** | sent as `Authorization: Bearer` | B |

**One rule, read down the two middle columns.** *Present-but-empty is an error for **every** member,
without exception.* What differs between members is only what **absent** means — a default, an error, or
a documented behaviour. That is not a third rule; it is the existing rule with the one axis that was
always going to vary. The shipped loader already implements the required half of it (`requireEnv`), and
the optional member is its sibling, not its exception.

**Why present-but-empty stays an error even for the optional member, which is the whole of the question
the ruling raises.** Absent and empty look interchangeable and are not:

- **Absent is a statement.** "This endpoint needs no authentication" — the local case, which is the point
  of the ruling, not an edge of it.
- **Empty is a mistake.** `PROCESSOR_MODEL_KEY=` in a compose file, or `PROCESSOR_MODEL_KEY=$SOME_VAR`
  where `SOME_VAR` never got set. Nobody writes an empty string to mean "no auth"; they write it by
  accident.
- **And the two mistakes are not symmetric.** Treating empty as absent means a deployment that *intended*
  to authenticate against a remote endpoint silently sends no credential. Best case it gets a `401` and
  someone reads a log. Worst case the endpoint accepts unauthenticated requests and the run succeeds
  against something nobody meant to reach. **A silent auth downgrade is exactly the failure a startup
  error costs nothing to prevent**, and the error names the variable, never the value.

**Why the endpoint URL and the model id are required, with no default.** Both are properties of *which
endpoint you pointed this at*, and both vary by construction: a local runtime and a hosted gateway serve
different addresses and different model names, and two local runtimes serve different model names from
each other. A default address would be worse than friction — **it would silently point the service
somewhere**, and "somewhere" is either a paid frontier endpoint (which is the spend the ruling declined)
or whatever else happens to be listening on a loopback port. A default model id has the same shape: it
either names a paid model or fails opaquely on an endpoint that has never heard of it. **There is no
defensible default, so absent is an error** — the same conclusion the two DiVoid members reached, by the
same argument.

**No provider-selection member is declared.** There is one adapter; a selector would have one arm.
#1220 §2: the member arrives with the milestone that first reads it (§12).

**Not declared, and each with its reason:**

| Tempting member | Why it is a constant instead |
|---|---|
| Output-token cap, candidate limit, byte budget, call cap | §8.4 |
| The model call's client timeout | A constant, like unit A's `divoid.DefaultTimeout` — but **a different one**, and §8.4a is why |
| System text source | §8.6 |
| Anything selecting a provider or a protocol | One adapter exists |

**Secrets discipline.** Two of the four members are secrets. The loader's error names the **variable**,
never the value — the existing loader already behaves exactly this way (measured in #10488 M16:
`error="PROCESSOR_HTTP_ADDR is set but empty"`). No key is logged, echoed in the error envelope, or
written to the graph, at any level.

**S6's falsifier, restated because this is the milestone that could break it:** a search of the module
for environment reads must return **exactly one site, in `main`**. Three new members do not change that;
a second lookup does.

### 8.2 The run record

| Field | Semantics |
|---|---|
| `input` | The caller's text, verbatim |
| `subject` | The node id the run is about |
| `query` | The text sent to the retriever. **At M1 this equals `input`, and it is a separate field precisely so that the day it stops being equal is visible in the record** |
| `anchor` | Subject node: id, type, name, size, content hash |
| `candidates[]` | **Every** row the query returned, in rank order: rank, id, type, name, similarity, size, content hash, `included` or `cut`, and the cut reason. **Two cut reasons since 2026-09-04 (#11158, B4)**: the byte budget, and *self-produced* for a run record this system wrote. A self-produced row is recorded exactly like any other — retrieved, ranked, sized and hashed — which is what keeps the evidence about how it competes against real content |
| `block` | The assembled context, verbatim |
| `answer` | The model's final text |
| `toolCalls[]` | Per supplementary recall round: the query the model asked, and **every** row it returned — with the same columns `candidates[]` carries: rank, id, type, name, similarity, size, content hash, `included` or `cut`, and the cut reason. **Widened in revision 3** (#10821 CF-4, W-7): the two paths are the same event under §6.4a, a cut is unreadable without the size that caused it, and §9.4 obligation 3 was simply false for any run that used the tool |
| `modelCalls` | How many. **And `capReached`, as its own field** — revision 3, from #10821 W-7. This row promised the fact and left it derivable from `stopReason` plus a comparison against `MaxModelCalls`, a constant the record does not carry; and the derivation holds only under one of the two sanctioned fixes for the at-cap recall query, so it is contingent on an implementation choice made later. **A fact this record promises is not delivered by a derivation the reader cannot perform and the design cannot guarantee** |
| `model` | **The model id that was sent.** New in revision 2, and not optional: under provider-agnosticism the model is a boot member rather than a constant, so a record that omits it cannot be interpreted after the fact and milestone 2's corpus would be scoring answers without knowing what produced them. Same argument as the content hash in §7, and the same reader |
| `usage` | **One entry per model call, in call order** — revision 3, from #10821 W-1. Each entry is the endpoint's two token counts as it reported them, **or absent**, never zero-filled (§6.5, §6.6, §8.3); the array's length always equals `modelCalls`. **The loop aggregates nothing.** A run total is the reader's sum of the present entries, and a run where some calls reported and others did not is legible as exactly that. **Rejected — the last call's counts:** under-reports a three-call run by up to two thirds, and reports *absent* for a run that measured something. **Rejected — one summed object:** a sum over a partially-reporting run is a number presented as a total that is not one, which is §6.5's own defect one level up, and it discards which call was expensive — the escalation signal milestone 2 is looking for |
| `stopReason` | **Two values, deliberately.** The loop's neutral terminal reason, which is what the loop branched on, **and** the raw string the endpoint returned, which is what milestone 2 will want when a mapping turns out to be wrong. The loop never branches on the second (§8.3) |
| ~~`written`~~ | ~~The node id the record was written to, or the reason it was not~~ **STRUCK 2026-09-02 (#10899, A2). The stored record carries neither: the body is serialised before the node exists, so the field renders as `{}` — a third value this row's own contract does not define, in every stored record. The write receipt is *not* a member of the record; it is a response-only key. `docs/architecture/run-record-fate.md` §7 recurses the consumer chain and §8.2 states the receipt's closed vocabulary** |
| `limits` | **The constants that governed this run**: the candidate limit, the assembly byte budget, the supplementary byte budget, the model-call cap and the output-token cap. New in revision 3. Same argument as `model` above and the same reader: §8.4 names milestone 2 as the event at which the first three become measurable, so **the corpus will span a change to them**, and every record written before that change is uninterpretable without knowing which values were in force. It is also what makes `candidates[]` readable at all — **recall@k is uncomputable without k** |

**Unit A's record has no `answer`, `model`, `toolCalls`, `modelCalls`, `usage`, `stopReason`, `written`
or `limits`.** Those fields arrive in unit B, together with their writers. That is #1220 §2 applied
within the milestone, and unit A's shipped `Record` already reflects it. **`limits` arrives in unit B
even though two of its members governed unit A**, because unit A is shipped and revision 3 does not touch
it — and because the corpus milestone 2 reads begins when the loop closes, not at PR 1.

### 8.3 The two ports

Declared by `internal/loop`; implemented by the adapters; constructed in `main`.

| Port | Operation | In | Out | Invariants |
|---|---|---|---|---|
| **graph** | subject fetch | node id | node with body, or not-found | Not-found is a distinct outcome, not an error (C30) |
| | recall | query text, limit | ranked rows with bodies and scores, **each marked self-produced or not** | Rank order as returned. The adapter does not re-sort and **drops nothing** — it marks. **Extended 2026-09-04 (#11158, B4):** whether a row is a record this system wrote is a fact about the graph's node vocabulary, which §5.1 places outside `internal/loop`, so the adapter classifies and assembly decides |
| | write run | a run record | the new node id | **The adapter chooses type, name and edge. The caller supplies no structure.** Three POSTs (C32) |
| **model** | judge | the system text, the assembled block, the input, the conversation so far, and whether supplementary recall is offered | the final prose; zero or more **supplementary-recall requests**, each a query string with an id to answer against; **one terminal reason from the loop's closed set**; the raw reason verbatim; **the two token counts if reported, or their absence** (revision 3, below) | One attempt. No retry. No interpretation of the prose. **Nothing in this column is a provider's word** |

**The model port's vocabulary is the whole of provider-agnosticism, and it is the part that is easy to
get wrong.** An interface whose method is `Complete` and whose return type has fields called
`stop_reason`, `tool_calls` and `finish_reason` is not a neutral port — it is one vendor's wire format
with an interface drawn around it, and the second provider will contort to fit it. So the contract is
stated as a rule an implementer and a reviewer can both check:

**The loop's terminal reasons are a closed set the loop defines**: *answered*, *wants recall*,
*truncated*, *refused*, *unrecognised*. The adapter maps into it; the loop branches only on it. **The
loop never sees the strings** an endpoint used. The record carries both (§8.2) because the record is data
for milestone 2, not control flow — and when a mapping turns out to be wrong, the raw values are the
evidence that shows it.

**The one place this contract still spoke a wire format — revision 3, from #10821 W-4.** The terminal
reasons are the loop's own closed set and the supplementary-recall requests are the loop's own
vocabulary; those were the hard parts and they hold. The token counts were not. They were carried as
**three fields copied name-for-name from one provider family's usage object** — a prompt count, a
completion count and a total — which is a wire format with a port drawn around it, in the one type where
the mistake is cheap to make and easy to miss.

**Two things are wrong with it and only one is obvious.** The obvious one: a second adapter must invent
the total, because the family reporting two counts and no total is at least as common as the one
reporting three — and a field the adapter must fabricate is precisely §6.5's *"a zero is a measurement,
an absence is the truth"* defect, moved from the object down to the field, where that rule does not
reach. The less obvious one: **the total is not a measurement at all.** It is the sum of the other two in
every endpoint that reports it, so it carries nothing a reader cannot compute, and its only effect is to
create the fabrication.

**The ruling:**

- **The total is dropped.** A reader sums. Nothing is lost and the fabrication has nowhere left to happen.
- **The two counts are named for the direction of travel** — tokens *in*, tokens *out* — and the record's
  keys follow. *Prompt* and *completion* are not neutral words in disguise: M1 does not send a prompt, it
  sends a system message, a block, an input and a tool tail, so "prompt tokens" names a field of a format
  the loop does not have. The test stated above applies to itself here — a reader who has never seen any
  provider's API can say what *in* and *out* mean.
- **Absence stays at the object level, and the conservative reading is stated rather than left open.** An
  endpoint reporting one count and not the other has not been observed on any runtime; until one is, such
  an object is recorded **absent in whole** rather than half zero-filled. **Falsifier:** the first runtime
  that reports one count and not the other proves this discards a real measurement, and the two counts
  become independently optional. That is one type's shape, decided the day there is a measurement.

**Two things this does not mean.** It is not an anti-corruption layer, and it is not a plugin system:
there is one adapter, one mapping, no registry, no selection. It is the ordinary port the loop already
has for the graph, with its column filled in from the loop's side rather than the wire's.

**The write port is the milestone's expression of "agents emit observations, not nodes."** The loop hands
over *what happened*; the adapter is the only thing that decides where it lands. That is the seam
milestone 5's memory core replaces — the adapter, not its callers.

**Written node contract:**

| Aspect | Contract |
|---|---|
| Type | `session-log` — #10424 §5.7's own name for the narrative tier |
| Name | Deterministic from the run: a fixed prefix, the timestamp, and a bounded prefix of the input, so a graph listing is legible without opening anything |
| Body | The run record |
| Edges | **Exactly one: a plain undirected edge to the subject node.** Per #7216, direction and a verb are added only where a real verb applies; "about" is close to the filler the rule names, nothing in the existing vocabulary fits, and coining one for a milestone's single edge is the over-decoration that node warns against. When a second edge kind appears, #7216 binds |
| Status | None. `session-log` carries no lifecycle |

### 8.4 Constants, not configuration (#1136 §3)

| Constant | Value | Why it is not a knob |
|---|---|---|
| Candidate limit | **20** | Measured C23: one round trip, 206 KB, ~1.5 s — and the whole Processor neighbourhood is ~22 nodes (C25), so 20 sees the region. No operator tunes it; milestone 2 is where it becomes measurable |
| Assembly byte budget | **60,000 UTF-8 bytes** | ≈15,000 tokens — bytes, not tokens, because a tokenizer is a dependency (§10.9) and the ratio is stable enough for a floor. **Revision 3 corrects the justification, not the value.** Revision 1 called it *"roughly 1.5% of the model's window, by intent"*. That figure is arithmetic against a **one-million-token** window — the provider revision 2 removed — and under the ruling there is no single window to take a fraction of. The premise it was expressing still stands: a small precise context beats a large one, so the budget is small by choice, not by capacity. But **a budget stated as a fraction of an unmeasured quantity bounds nothing and cannot be checked**, which is how the recall path shipped at 3.3× over it with nobody able to say so (#10821 CF-4, §6.4a). The number is 60,000 bytes; what fraction of a window that is, is stated below against named window classes instead of assumed. ~~Measured on C23's real 20-row set: it admits ranks 1–5, uses 44,931 B, and **cuts 15 of 20**, so the cut path runs on ordinary production runs and not only in tests~~ **CORRECTED 2026-09-05 (#11335, E6). What this constant bounds changed, and so did that measurement.** It now bounds **the whole initial block — the anchor plus the admitted candidates** — rather than the admitted candidates alone: `Assemble` subtracts the anchor's size from it before considering the first candidate, floored at zero (§11 R4). The C23 profile above holds only for a zero-length anchor and is superseded by the 23-row sweep in §11 R4, where mean documents per block fall **8.09 → 6.91** and one row in twenty-three admits nothing. The value is **unchanged at 60,000** and the cut path is exercised harder, not less. **The name is unchanged too, and deliberately.** `AssemblyByteBudget` now names a block budget, which reads a shade narrow, and renaming it was declined: it would move roughly fifteen references in this document, the JSON wire key `assemblyByteBudget` that already-written records carry in `limits` (§8.2), and any external script reading that literal key — cost with no product (#11034 P-3). **Recorded so it is not re-opened;** the godoc line on the constant states what it bounds, which is where a reader of the code meets it |
| Model call cap per run | **3** | One judgement call plus two supplementary rounds. Enough to observe the escalation path; small enough that a loop cannot run away |
| Output-token cap (`max_tokens`) | **4,096** | **Re-derived in revision 2; it was 16,000.** The old value came from the Anthropic reference's non-streaming default, and that justification is gone with the provider. Re-derived against the ruling's actual target: many local models cap output well below 16,000 and either clamp silently or reject the request, and the answer here is *prose for a human*, which does not need 16,000 tokens. 4,096 is inside every plausible local runtime's capability and generous for the job. **Falsifier:** the §6.5 *truncated* row is exactly how a wrong value announces itself — if real runs report truncation, the number is too small and the record says so |
| **Supplementary byte budget** | **20,000 UTF-8 bytes, per recall round** | New in revision 3 (§6.4a). **Derived, not picked:** it is the largest per-round figure that keeps a whole run's graph-derived prompt inside a 32,768-token window with the output cap reserved — the derivation is below. Against C23's real distribution it admits **3 median bodies** (5,758 B each), 6 of the smallest (2,872 B), and **none** of the largest (42,978 B) — which the record reports as a cut rather than hiding |

**The ceiling, in bytes, against named windows — revision 3.**

The constants above are not independent. Two bound bytes and a third multiplies one of them, so together
they state how large a run's prompt can get — and that number is what an endpoint has to be able to hold:

```
  the block        AssemblyByteBudget                             =   60,000 B
  supplementary    SupplementaryByteBudget × (MaxModelCalls − 1)  =   40,000 B
                                                                    ──────────
  graph-derived prompt, per run — THE CEILING                     =  100,000 B   ≈ 25,000 tokens
  + the system text and framing                                   ~    2,000 B   ≈    500 tokens
  + the anchor     |anchor| — unbounded (§11 R4)
  + reserved for the answer, maxOutputTokens                                     ≈  4,096 tokens
```

**CORRECTED 2026-09-05 (#11335, E5). The anchor line is struck: it is inside the first line, not a fifth
term.** `AssemblyByteBudget` bounds the anchor plus the admitted candidates, so **100,000 B is the whole
graph-derived prompt in content bytes** and the ceiling is a ceiling over what the budgets can see. What
was an unbounded term becomes **two bounded-or-measured residuals**, and they are the only ways a run
exceeds 100,000 B:

```
  ordinary run   content = |anchor| + admitted  ≤  60,000 B    supplementary ≤ 40,000 B  →  ≤ 100,000 B
  residual 1     |anchor| > 60,000 B: the anchor renders whole and nothing else is admitted (§11 R4)
                 content = |anchor|                            →  |anchor| + 40,000 B
  residual 2     render framing, on EVERY run: 178–2,444 B measured, outside every budget (§11 R4, §6.3)
```

**Measured, on the largest anchor this graph holds (70,660 B, r05):** its rendered block is **70,838 B**
(70,660 B of content plus 178 B of framing, no candidates admitted), so the residual run's prompt is
**110,838 B ≈ 27,710 tokens**, plus 303 for the system text and 4,096 reserved — **98.0% of a
32,768-token window, and it still fits.** Before this change the same run's block alone measured
**130,383 B** and the whole prompt would not have fitted that window at all. **That is the change stated at
its most useful: the worst measured run moves from over the target window to inside it, with both residuals
named.**

**ALSO CORRECTED 2026-09-05 (#11335, E5): the *"~2,000 B for the system text and framing"* line above
under-provisions the one term that was accounted for at all.** Measured: the system text is **1,212 B** and
the block's render framing runs **178–2,444 B** (§11 R4, §6.3), so the pair reaches **3,656 B — about 1.8×
the allowance.** Read that line as **~3,700 B ≈ 925 tokens**. The 32,768 row of the table below moves from
≈ 29,600 tokens to ≈ **30,000 tokens (92%)**; **no verdict in the table changes**, and the 8,192 row was
already unreachable by a wide margin. The allowance is stated here rather than edited into the fence,
because a fence carries no strikethrough and the original figure is dated record (#11034 P-43).

At the same four-bytes-per-token ratio this table already uses for the budget itself:

| Endpoint window | The block alone | A worst-case run, ~~anchor excluded~~ **anchor included (E5)** | Verdict |
|---|---|---|---|
| **8,192 tokens** | **183%** | does not fit | **M1 cannot run here at all** — and the tool is not why. §11 R14 |
| **32,768 tokens** | 46% | ~~≈ 29,600 tokens, **90%**~~ **≈ 30,000 tokens, 92% — CORRECTED 2026-09-05 (#11335, E5): the framing allowance was 1.8× short; the verdict is unchanged** | Fits, leaving ~~≈ 3,200 tokens — about **12,500 bytes** — for the anchor~~ **CORRECTED 2026-09-05 (#11335, E5): ≈ 2,750 tokens, about 11,000 bytes, and it is slack rather than an allocation. The anchor is funded from the 46% column now; the leftover shrank because the framing allowance in the same row grew** |
| **131,072 tokens** | 11% | 23% | Comfortable |
| **1,000,000 tokens** | 1.5% | 3% | The figure revision 1 quoted, and the window it was quoting |

**Two things this table is for.** First, `SupplementaryByteBudget` is the free variable, and 20,000 is
what solving the 32,768 row gives — a derivation, not a taste. Second, the row that matters is the one
with the smallest window: **the ruling's own target is small local runtimes, and 8,192 is a real window
size among them.** M1 does not fit it — because of the *assembly* budget, which is unit A's shipped
constant and outside this revision. §11 R14 records that with its falsifier rather than repairing it
here, and §13.6 asks the one question that would settle it.

**`100,000` is a literal this design defends, not an incidental product.** §14 requires it asserted as a
literal against the arithmetic of the three constants, so that moving any one of them without
re-deriving the window table above turns a test red. The alternative — a test computing the same
expression production computes — is the assertion #10466 names as one that can never fail, and #10821
CF-2 found seven of those in this milestone already.

**Model id is no longer here.** It was `claude-opus-5`, justified as having "no environment difference".
The ruling makes that false by construction — every endpoint serves different model names — so it is a
required boot member (§8.1). Recorded rather than silently moved, because the *reason* it moved is the
ruling and a later reader should be able to see that.

**None of these is a "telemetry-then-tune" knob**, so #1136 §3's filed-task requirement does not bind:
they ship as constants. Milestone 2 is the named event at which the first three become measurable, and
when it arrives it will have scores in hand rather than a shape guessed today.

### 8.4a The model call's timeout — a constant, and *not* unit A's constant

Unit A's review added `divoid.DefaultTimeout = 15s` to bound a hung graph read (finding W-7). **The model
adapter needs its own, and it must not reuse that one.** This is not a style point: under the ruling the
latency profile of the model call is qualitatively different from anything M0 or unit A dealt with. A
hosted endpoint answers in seconds; **a local model on CPU can take minutes** for a few hundred tokens,
and that is the deployment the ruling exists to enable. A 15-second bound would turn the ruling's own
target into a service that never completes a run.

- **The model adapter's timeout is its own constant, generous by intent**, sized for a slow local
  generation rather than a fast hosted one. It exists to stop a hung socket, not to enforce a latency
  budget — there is no latency budget at M1, because the caller is a human with `curl`.
- **Two more timeouts already exist and were checked rather than assumed.** M0's server sets
  `ReadHeaderTimeout` only and **no `WriteTimeout`**, so a long-running handler is not cut off —
  the shipped shape happens to be exactly right for this and nothing needs to change.
- **One interaction is named and deliberately left alone:** `shutdownGrace` is 5 s, so a run in flight
  against a slow local model is abandoned on `SIGTERM`. ~~**M1 does not raise it.**~~ The write-back is the
  turn's last step, so an abandoned run writes nothing and costs one rerun by the human who started it;
  ~~raising the grace to cover a worst-case local generation would make every shutdown hang for minutes to
  protect a single re-runnable request.~~ **Falsifier:** the day a run is expensive or a non-human producer
  drives it, the trade reverses — and neither is true at M1.
  **CORRECTED 2026-09-02 (#10890, A1) — the struck mechanism claim is false, and the conclusion reverses
  with it.** `Shutdown` closes the listeners and then returns **as soon as connections are idle**; the
  context it is given is a **ceiling, not a fixed wait**, so an idle shutdown is instantaneous at any
  grace and the cost is paid only when there is a run to protect. This paragraph's own falsifier was met
  by a route it did not predict — not by the run becoming expensive, but by the mechanism claim being
  wrong. `docs/architecture/run-record-fate.md` §6.3 argues the three shutdown options and §8.4 derives
  the grace from a stated run bound.

### 8.5 The error envelope — M0's deferred decision, taken

M0 §8.5 deferred this to *"the first endpoint that can fail"*. Here it is, minimal:

`{"error":{"code":"<stable token>","message":"<human sentence>"}}`, with `Content-Type: application/json`.

| Code | Status | Meaning |
|---|---|---|
| `invalid_request` | 400 | Body unparseable, or `input`/`subject` missing or empty |
| `subject_not_found` | 404 | The subject id resolves to nothing (C30) |
| `graph_unavailable` | 502 | The graph could not be read |
| `model_unavailable` | 502 | The model call did not complete |

**Four codes, closed, and each has a caller decision behind it**: 400 means fix the request, 404 means
fix the id, 502 means retry later. A refusal is **not** in this table — it is a `200` outcome. A
write-back failure is **not** in this table — §6.5.

`message` is for a human and carries no key, no URL with credentials, and no upstream body. **The same
rule governs every other surface an error string reaches** — revision 3: the tool-result message the
model reads and the run record written to the graph (§6.4a). It was stated here for the caller and
applies there for the same reason and one more, because those two surfaces are read by a model and
written to a shared substrate rather than returned to the operator who owns the deployment, so an
internal address in them travels further than one in a 502 body.

### 8.6 The system text — the one departure from inversion 3, stated

#10424's third inversion says briefings live in the graph. **M1's system text is a constant in code**,
constructed in `main` and passed into the loop as a value.

**Why.** Reading it from the graph needs a node id, which arrives either as a fifth configuration member
or as a name convention. The first is a member with no consumer beyond itself; the second is magic. The
milestone that reads briefings from the graph will have a *reason* to identify them — a role, a workflow
step — and will bring the identification with it.

**The seam is real and costs nothing:** the text is a value at the construction site, exactly like the
listener M0 injects. Substituting "read it from a node" is a change in `main`, not in the loop.

**What the text must establish** (its wording is a prompt-engineering deliverable, not an architectural
one — §14): that the assembled block is what memory selected for this step and is not a transcript; that
the block is not necessarily complete and the recall tool exists for what is missing; that the answer is
prose for a human. **It must not** ask the model to decide what to remember, where to write, or how to
structure anything — that would reintroduce the failure §5.3 of the vision was written from.

---

## 9. What Is Deterministic, What Is Pinnable, and Where the Seam Is

*This section answers #10521's own most-wanted question: "the model is nondeterministic" must not become
"so we do not test it."*

### 9.1 The turn, classified stage by stage

| Stage | Deterministic? | Pinned how, and where |
|---|---|---|
| **1. Retrieval** | **Yes, given (graph state, query text)** — measured bit-stable to nine decimals over four calls (C22) | Not against the live graph, which is a shared substrate that drifts. Pinned at the **wire level** in the adapter (URL construction, decoding, the C30 empty-result trap) and **replayed** from fixed rows into the loop |
| **2. Assembly** | **Fully.** A pure function with no I/O, no clock, no randomness — and, because the render order is by id (§6.3), a **total** order | **Byte-exact golden test.** Fixed candidate rows in, one exact string out. This is the milestone's central assertion |
| **3a. The model *request*** | **Yes.** ~~A deterministic function of (system text, block, input, tool definition, model id)~~ **CORRECTED 2026-09-05 (#11401): a deterministic function of (system text, block, input, tool definition, model id, the configured sampling)** — the request gained `temperature` and `top_p`, read once at boot from `PROCESSOR_MODEL_TEMPERATURE` and `PROCESSOR_MODEL_TOP_P`. **The verdict is untouched and the enumeration is what changed:** the two added terms are boot constants, fixed for the process's lifetime, so the request is still a pure function of its inputs — and it stays deterministic **because M1 sends one fixed request shape rather than adapting to the endpoint** (§6.6) | Byte-exact, at the wire level, against a local test server |
| **3b. The model *response*** | **No. This is the only nondeterministic thing in M1** | Not pinned, by construction. Substituted at the port |
| **3c. Translating that response into the loop's vocabulary** | **Yes, given the response** — the reason mapping, the recall request, usage-or-absent (§8.3) | Wire level, from fixtures. **New in revision 2**: the ruling adds a deterministic stage on the far side of the nondeterministic one, and it is fully pinnable |
| **4. Tool dispatch** | **Yes, given the response** — which query is issued, where the result is placed, when the cap fires | Port-level, from canned responses |
| **5. Write-back** | **Yes, given the record** — type, name, body, edge target | Port-level (what was handed over) plus wire-level (the three-POST sequence, C32) |

**Read the boundary between 3a and 3b as the whole answer.** Everything before it is a pure function;
everything after it is data. Nondeterminism does not permeate the milestone — it enters at exactly one
line, and that line is where the seam goes. **Revision 2 did not move that line**; it added 3c behind it,
which is deterministic, which is why provider-agnosticism costs the test strategy nothing.

**Why sampling is two parameters under two different unset rules — added 2026-09-05 (#11401), the home for
3a's correction.** `temperature` has a default and is always sent; `top_p` has no default and is **omitted
from the request** when the operator configures nothing. The asymmetry is not an oversight. `temperature: 0`
is well-defined argmax decoding on every OpenAI-compatible runtime, so a value exists that is both
meaningful and maximally reproducible, and sending it is benign. `top_p: 0` has no such standing: its
behaviour is *endpoint-dependent* — some runtimes clamp it to top-1, others reject it as a validation error
— and a configuration value whose meaning varies by runtime is precisely what a change made for
reproducibility must not introduce. Asserting `1.0` instead would be worse than silence: it claims knowledge
of an endpoint §6.6 deliberately refuses to model, and it newly sends a parameter some endpoints validate.
So the pointer's `nil` is the wire's *absence*, not a zero, and the default configuration carries exactly
one determinism lever rather than two overlapping ones — at `temperature: 0` the distribution is already
collapsed and `top_p` is a no-op at best. **What the record can therefore claim, and what it cannot:** it
reports the sampling **as sent**, never what the endpoint applied. A clamping or ignoring runtime yields a
record that is true about the request and false about the generation; the near side of the wire is the only
side this design can pin, and §9.1 row 3a is the statement of why.

### 9.2 The two seams, and the measurement that justifies each

The local standard is #10437 / PR #3: the `httpServer` seam was upheld because deleting it stopped the
guard reddening. A seam that cannot show that is test-induced design damage.

**Seam 1 — the model port. The justification this document originally gave is WITHDRAWN, and the seam
survives on a different one.** This is the largest change revision 2 makes, and it is stated as a
withdrawal rather than an edit because the original argument was load-bearing and a reader who inherited
it would be relying on something false.

*What the document used to say:* delete the port; let the loop construct its own HTTP client against
`api.anthropic.com`. Then every loop test — assembly ordering, budget cutting, the tool cycle, the call
cap, the record's shape, every §6.5 branch — could only run by making a live, paid, nondeterministic
call, and C34 said no credential existed, so the suite would be red for everyone. *"The seam does not
make those tests better; it makes them possible."*

***That is now false, and the ruling is what made it false.*** The endpoint address is a configuration
value — it has to be (§8.1) — so a port-less loop can be pointed at a local test HTTP server serving
canned chat-completions JSON. **The tests are possible without the port.** They are worse, but "worse" is
not what §9.2's own standard asks for, and a seam kept on a lapsed argument is precisely the
test-induced design damage this section exists to catch.

*So the experiment is re-run, honestly, under the new premise.* **Delete the model port. What actually
happens:**

1. **The tests still run** — offline, no credential, no spend, against a local test server. The port does
   not rescue them.
2. **But every loop test now expresses its input as OpenAI-compatible JSON.** "The cap fires after three
   calls" becomes a test server returning three wire-shaped tool-call payloads in sequence. The loop's
   tests would assert loop behaviour *through one vendor's format*. That is a real cost and not a
   decisive one.
3. **And the loop itself now holds the request construction and the response decoding.** A second
   provider is then either a branch inside `internal/loop` or a rewrite of it — and **a loop that speaks
   one provider's JSON is the exact thing ruling item 1 forbids**, more severely than a port shaped
   around one vendor would be.

**Point 3 is the justification, and it is a better one than the argument it replaces.** The seam is no
longer a test affordance that happens to help production; it is a **production requirement** — the
mechanism by which the ruling's *"more providers in the end"* is a construction change in `main` rather
than surgery on the loop — that happens to help tests. A test affordance is the kind of justification
this section is suspicious of. A stated product requirement is not.

*Falsifiers, both new, because the old one is now satisfiable and has stopped being a falsifier:*

- **Substitution:** if a second provider can be added without touching `internal/loop` while the loop
  constructs its own HTTP, the port is not justified and should go.
- **Neutrality — the one that will actually catch this if it goes wrong:** if the port's contract turns
  out to be a transliteration of the OpenAI-compatible wire shape — the same field names, the same
  finish-reason strings crossing the boundary — then it is **not** a neutral port, it has not earned its
  name, and it is one vendor's format with an interface in front of it. Checkable by inspection: grep
  `internal/loop` for any provider's vocabulary. **It should return nothing.**

**Seam 2 — the graph port. Re-checked under the ruling, unchanged.** The ruling touches the model call
and nothing else, and the experiment comes out exactly as before.
*Justifying experiment:* delete it. Every run of the loop's tests then performs C32's three POSTs against
the production graph, creating durable nodes on every `go test`. And the assembly golden test would
depend on live graph state, which C22 shows is stable *at an instant* but which drifts by design —
a golden test on a live shared substrate is a time bomb with a green light on it.
*Falsifier:* if the write can be made a no-op without a port, it is not justified. It cannot: writing is
the run's contract.

**Not a seam, and not dressed as one.** The system text (§8.6) is a value passed in. That is M0's
configuration rule, not a test affordance, and it introduces no interface.

**No other seams.** Assembly is a function, not an interface. The record is a value. The handler takes the
loop, not an abstraction of it. **Falsifier for this whole section:** any interface in the module with one
implementation, one test double, and no experiment of the above shape behind it.

### 9.3 Each level asserts only what is its own

M0 §9.1's rule, applied so that the two levels do not re-assert each other:

| Level | Instrument | Owns |
|---|---|---|
| Adapter, wire | A local test HTTP server | URL and query construction; header presence; response decoding; the C30 empty-result discrimination; the C31 auth-failure shape; the three-POST write sequence; the exact model request body |
| Loop, port | Canned values through the ports | Assembly bytes; admission and cut; render order; tool dispatch and the cap; record shape; every §6.5 branch |
| Handler | An in-process recorder | Status codes and the error envelope |

**The default `go test ./...` is fully offline and hermetic**: no network, no credential, no live graph,
no spend, **and no model runtime**. Revision 2 re-derived the reason. It used to be C34 — no credential
existed, so it had to be. That reason is gone, and two better ones replace it: **C43**, because
#10488's gate runs `--network none` and a suite that needs a socket cannot pass it; and the plain fact
that **a live model is nondeterministic**, so it can never be what a per-change suite asserts against.
Neither reason can lapse the way the credential one did.

### 9.4 What M1 owes milestone 2, and what would break it

Milestone 2 scores `input → the node ids that must be in scope`. It can only do that if M1 wrote down the
misses. Three obligations:

1. **The record carries the *whole* candidate set** with scores, ranks, hashes and cut flags — not the
   surviving subset. **If M1 records only what it kept, recall@k is uncomputable for every run ever
   written, retroactively.** This is the single most important thing in the milestone that is easy to get
   wrong and impossible to fix later.
2. **The query text is recorded verbatim** and as its own field (§8.2), so that the day it stops equalling
   the input is visible rather than inferred.
3. **What the model saw is reconstructible from the record — on both paths.** The block is stored
   verbatim, and the per-candidate hashes say whether a node has changed since. **Revision 3 extends this
   to the tool rounds, where it was simply false:** the model saw full bodies for every admitted
   supplementary hit, and revision 2's record carried those hits as an id and a score with no size, no
   hash and no admission flag — so a run that used the tool could not be reconstructed, and could not
   even be *detected* as unreconstructible. `toolCalls[]` now carries the same columns as `candidates[]`
   (§8.2).
   **The honest limit, and it is weaker here than for the block.** The rendered tool text is *not* stored
   verbatim the way the block is. A reader reconstructs it from the admitted ids, their hashes and
   §6.4a's rendering rule — which detects a changed body but cannot recover the old one. The block earns
   its verbatim copy because a human reads it to judge S2 (§11 R12); the tool text has no such reader
   today, and storing it would roughly double a record already 10–19× the size of a median node in this
   graph (§11 R5). **Falsifier:** if milestone 2 finds it cannot score tool rounds without the bodies,
   the rendered text joins the record and the record doubles — a decision to take with the corpus in
   hand, not now.

**The honest limit, stated rather than implied.** "Replayable" at M1 means *replayable from the record*,
not *reproducible against the live graph*. The same input a day later may return a different candidate
set, because the graph moved. The content hashes are what make the difference **visible** — they say
whether a node changed or the ranking did. Falsifier: re-run one input a week apart; if the hashes cannot
distinguish those two cases, the record is not doing its job.

### 9.5 Stated as not covered

| Gap | Falsifying experiment |
|---|---|
| The OpenAI-compatible **200** response shape is documented, not measured. C41 measured the route, the auth header and the error envelope; **the success shape was not measured, because C40 says no runtime is installed here** | A live run against a local runtime either decodes or does not. **Unlike C37, this is closeable before merge** — it needs an install, not a credential — and §14 step 14 makes it a hand-off condition |
| Which subset real endpoints honour (§6.6) is taken from documentation and from the reference implementation's behaviour, not from a survey of runtimes | Point it at two different local runtimes. Each either serves D1–D6 or names what it rejects |
| Assembly's behaviour against a real 20-row set is inferred from C23/C24's byte distribution, not run | Post one real input through unit A and read the cut column |
| Latency per run against a *local* model is completely unmeasured, and §8.4a's timeout is sized on judgement rather than data | One live run reports it. This is the single largest unknown the ruling introduces |
| Flake behaviour of the wire-level tests over many runs | Run the suite 20 times. Not run here, and not claimed as run |
| **The four-bytes-per-token ratio** every figure in §8.4's ceiling table rests on. It is an assumption inherited from the assembly budget, and it varies by tokenizer and by language | One live run reports the input count for a prompt of known byte length — that *is* the ratio, measured. §14 step 14 already collects it |
| **The window sizes in §8.4's table**, and that an endpoint **rejects** rather than silently truncating a prompt over its window | §6.5 assumes it fails loudly and nothing has confirmed which. Point it at a runtime whose window is smaller than the run needs; §14 step 14 records the advertised window |

---

## 10. Cross-Cutting Concerns

### 10.1 Layout

```
cmd/processor/        main.go · config.go            (+5 members, same single read site)
internal/server/      routes.go · server.go           (+1 route, +1 handler parameter)
internal/loop/        assemble.go · run.go            NEW  (unit A shipped)
internal/divoid/                                      NEW  (unit A shipped read side)
internal/openaicompat/                                NEW  (unit B)
```

Three new packages. Each was passed through delete / merge / inline in §5.1 and §5.6.

### 10.2 Security

**One secret always, two sometimes.** The graph key is always present; the model key is present only when
the endpoint requires one, and in the ruling's own target case — a local runtime — there is **no second
secret at all**. Each travels as a value from the one read site to the one adapter that uses it. Neither
is ever logged, ever in the error envelope (§8.5), ever in the run record, or **ever written to the
graph** — the graph is a shared substrate and a key in it is a leak. The run record's body is prose and
node references; it carries nothing from the environment.

**The model endpoint URL is treated as a secret too, and this is new.** It is not one by nature — but
it is operator-supplied, and gateway URLs in the wild carry credentials in userinfo or in a query
parameter. Since nothing needs it in the record or the log, it goes in neither: **the record carries the
model *id*, never the endpoint** (§8.2), and the log carries the same. That costs nothing and removes a
whole class of leak from a member that did not exist before revision 2.

Neither adapter follows redirects to a different host, and neither accepts a base URL from anywhere but
boot configuration. **The model adapter sends `Authorization` only when a key was configured** — an
absent key means an absent header, not an empty one (§8.1).

### 10.3 Observability

M0's partition holds: **stderr carries operational events; the graph carries content.**

Per run, two structured records on stderr: run started (subject id, input length) and run finished (node
id written, candidate count, cut count, model calls, **the model id**, token usage summed across the calls that reported it — or its absence, never a zero (§8.2) — outcome). **The
assembled block is never logged** — ~~it is up to 60,000 bytes~~ **CORRECTED 2026-09-05 (#11335, E7): this
was false when written and is now nearly true. Blocks measured 60,601–130,383 B under the old accounting;
the bound on its content is `max(anchorSize, 60,000)` from this date, plus 178–2,444 B of render framing
that no budget sees (§11 R4, §6.3). The rule it justifies never depended on the number** — and it has a home (§8.2). On a failed
external call, log the upstream status and, **when the endpoint volunteers a request id, that** — but do
not require one: C41 measured the reference implementation returning an error with **no** request id at
all, and local runtimes generally have no such concept. The old text treated Anthropic's `request_id` as
a field that would be there; under the ruling it is a field that usually is not.

**A third record, on failure only — new 2026-09-04 (#11158, B4).** A run that admitted **nothing** from
a **non-empty** candidate set logs a **warning** naming the condition, with the subject and the candidate
count. The block that reached the model was the anchor alone, which is a degraded answer rather than a
failed one: **WARN, not ERROR** — the turn succeeded at the HTTP boundary, a caller got an answer, and
the record is still the evidence. An **empty** candidate set is deliberately **not** this condition; that
is a retrieval failure, not an admission one.

**EXTENDED 2026-09-05 (#11335, E11) — the alarm now covers a cause it could not previously reach, and it
needed no change to do so.** The condition is stated as *a non-empty candidate set admitting nothing*,
which is agnostic to **why** nothing was admitted. Before the anchor was charged there was exactly one way
to satisfy it — every candidate individually too large — and **anchor exhaustion could not reach it at all,
because the anchor never consumed a byte of the budget.** #11365 §4 names that gap in the before-state:
*"an anchor larger than the whole block budget produces a block that is anchor and nothing else — #11158's
shutout arriving by the route #11335 predicted, and the WARN does not fire on it."* It fires on it now.
**Worth recording precisely because nothing was added:** a detector written against the observable
condition rather than against the cause it was built for picked up a second cause for free, and the fact
that the paragraph above needs no edit is the evidence for that. Pinned by
`TestTurnRunWarnsWhenTheAnchorAloneConsumesTheWholeBudget`, which drives two individually tiny candidates
against an anchor one byte over the budget and asserts both the shutout record and that both candidates are
still cut. The empty-candidate-set carve-out is unaffected and is separately pinned.

No key value, **no endpoint URL** (§10.2), no input text beyond its length, at any level.

**The logger is a required dependency, and the one guard around it is deliberate — recorded 2026-09-02
from #10868 W-2.** The turn's constructor takes a logger and does not default one, because a defaulted
logger silently swallows the channel this section has just partitioned — a worse outcome than a build
that will not start. That mirrors `internal/server`, whose serve function takes a required logger and
dereferences it with no guard. **The mirror is imperfect in exactly the way that matters.** Serve logs on
its first line, so a missing logger fails at boot, immediately, in every environment. The turn
dereferences its logger **only on failure branches** — a supplementary recall that could not be reached
(§6.5), a write-back that did not land (§8.2) — so the same omission survives every green run and panics
in production months later, on the branch least likely to have been exercised. And the turn is an
**exported type with exported fields**: a keyed literal builds a structurally valid turn with no logger
and never touches the constructor. That literal is the exposure. **The decision: the constructor stays
required — upheld twice (#10829 ruling 2, #10862 W-4) and not re-opened here — and the turn's internal
logging accessor discards when the field is nil.** Discarding rather than substituting a real logger: the
constructed path keeps its channel, the bypassing path degrades to silence instead of a crash, and
neither path invents an output the operator did not configure. Pinned by
`TestTurnBuiltByAnExternalKeyedLiteralSurvivesTheNilLoggerBranch`, which drives a keyed-literal turn
through the recall-failure branch. **This paragraph exists because the reasoning was deleted from the
code rather than moved here** — #114 §4's remedy for a load-bearing comment is relocation to this
document, and #10868 W-2 measured the one place the comment sweep took the first half and dropped the
second.

### 10.4 Error handling

M0 §10.4's shape carries: errors travel up as values and are decided at the boundary that owns the
outcome. The loop translates external failures into run outcomes; the handler translates run outcomes into
status codes and envelopes. No panics, no recovery middleware, no error type hierarchy — §6.5's table is
small enough to be read whole.

### 10.5 Retries, and their absence

**One attempt per external call.** Adding retry now would mean choosing a backoff, a budget and a jitter —
three constants with no measurement behind any of them.

**Revision 2 lost half of this argument and it is recorded rather than quietly kept.** The original text
added *"and a retried model call spends money twice"*. Against a local endpoint that is simply false: a
retry costs some seconds of a machine that is already yours. The spend argument now applies only when the
endpoint is a paid one, which is a deployment choice rather than a property of the design — so it cannot
carry the rule.

**What carries it is the unmeasured-constants argument, unchanged and sufficient**, plus one the ruling
adds: retrying against a slow local model means a caller waiting a multiple of an already-long generation,
with no way to tell a hung endpoint from a working one. **What would change it:** a measured `429` or
transient-`5xx` rate from real runs. Until then the caller retries, which is the correct layer at M1
because the caller is a human.

### 10.6 The container, and what runs where

#10440's rule binds unchanged: live execution that reaches the interactive desktop, or binds beyond
loopback, goes in a container.

- **The default suite is unaffected** — it is hermetic (§9.3), makes no network call, and stays exactly
  as fast and as portable as it is today.
- **#10488's Linux gate is unchanged**, and C43 re-measured why it must stay that way: on the shipped
  unit A it passes today with `--network none`, four `ok` lines, in **9.96 s**.
- **The live end-to-end check is one documented manual command** in the container, with the model members
  supplied — **no new automated gate**, no availability probe (#10488 §10.2's argument transfers
  verbatim). **No longer blocked**: it needs a local runtime installed (C40), not a credential procured.

**Does a local runtime belong in the container gate? No — a separate opt-in tier, and the reason is not
cost.** The question is worth answering explicitly because the ruling makes a local model *available* to
the gate for the first time, and availability reads like an invitation.

1. **The decisive reason: a gate asserts, and a live model cannot be asserted against.** It is
   nondeterministic by construction — that is §9.1's entire classification. The most a gated model call
   could check is *that a turn completed*, which is a smoke test wearing a gate's clothes: it fails for
   reasons unrelated to the change under review, and a gate that reddens for unrelated reasons stops
   being read. #10488 §10.2 already rejected an availability probe on exactly this ground.
2. **It would destroy a measured property.** `--network none` is live today at 9.96 s (C43). A model
   server means an image — **3,226 MiB compressed for `ollama/ollama` before any weights** (C44) — plus
   weights that must be fetched, in a venue that runs `--rm` and therefore caches nothing between runs.
3. **And it would buy almost nothing**, because the offline suite already covers everything
   deterministic: the wire level pins the exact request body, the port level pins every loop branch
   (§9.3). The live tier adds exactly **one** thing — confirmation that a real implementation's success
   response decodes. That is a **one-off confirmation, not a per-change assertion**, and per-change is
   the only thing a gate is for.

**So: no automated model gate at any tier in M1.** One documented manual command, run once by unit B's
implementer before hand-off (§14 step 14). What the ruling changes is not *where* that check runs but
*whether anyone can run it* — and now anyone can.

### 10.7 Idempotency, concurrency, state

**No idempotency.** A repeated `POST /runs` runs again, calls the model again, and writes a second record.
That is correct at M1: the caller is a human and each run is a distinct observation. *What would change
it:* a retrying automated producer, which does not exist.

**Concurrency:** two runs may be in flight. The loop holds no shared mutable state; every run is a value
(§6.5).

### 10.8 `/health` does not grow

#10466 says *"when a real dependency exists, the endpoint reports on it and the body grows a member with
a reader."* Two real dependencies now exist and the endpoint still does not grow, because **the reader
does not**: there is no orchestrator, no probe, no dashboard. A dependency check with no consumer is a
persisted claim nobody acts on (#1136 §2 form 1) that also makes the endpoint slow and flaky. It grows
with its first reader.

### 10.9 The dependency decision, in full

**Decision: unchanged — no third-party dependency. `net/http`, `encoding/json`, `crypto/sha256`,
`net/url`. But the reasoning is re-derived against the new candidate, not inherited**, because the
candidate changed and one of the two original grounds nearly evaporated when re-measured.

| | Official OpenAI Go SDK | Raw HTTP |
|---|---|---|
| Surface M1 uses | One non-streaming POST; one message object; §6.6's subset | The same |
| `go.mod` | **5 required modules**, 12 `go.sum` lines, 18 in the build list (C42) — into a file with no `require` block at all | Unchanged: still none |
| #10488's gate with `--network none` | **Fails** (C43) | Passes, 9.96 s (C43) |
| Cold gate wall clock, network on | 12.6 s (C43) | 9.96 s |
| Getting the request shape wrong | Prevented by types | **Fails loudly** — a `4xx` naming the field (C41's shape) |
| Which subset of "OpenAI-compatible" is sent | The SDK's, and it is shaped for the reference implementation's full surface | **Ours, and §6.6 states it** |
| Migration surface | The SDK's, on its schedule | Ours |

**What re-measuring changed.** C39 said the Anthropic SDK cost 11 modules and roughly doubled the gate's
cold wall clock. The OpenAI SDK costs **5** and adds **+27 %** (C42, C43). **The quantitative argument is
now weak enough that it should not be quoted as a reason** — and this document did quote it, which is why
the re-measurement was worth doing rather than assuming the old numbers transferred.

**What survived is structural and did not move by a single second.** The gate runs `--rm`, so there is no
persistent module cache; a tree with *any* `require` line must reach `proxy.golang.org` before `go vet`
can run. **The cost is not proportional to the dependency's size — the first `require` line is the whole
of it**, and the tree behind it is rounding. A five-module dependency fails `--network none` exactly as
completely as a sixty-five-module one (C43, measured both ways).

**And the ruling adds a second ground that did not exist before, which is now the stronger of the two.**
§6.6 is a decision about *which subset of an unspecified de facto standard M1 depends on*. That decision
is the design's, and it is the thing that determines how many endpoints can serve this service. An SDK
makes that decision instead — and makes it in favour of the one vendor it is built for, whose full
surface is precisely what a local runtime does not implement. **Under a provider-agnosticism ruling, the
argument for owning the request bytes is strictly stronger than it was under a single-provider design**,
exactly as the ruling anticipated. #10424 §7's own position says the same thing from the other side:
*"nearly all of it is HTTP + SSE + JSON, and we want control of that layer anyway."*

**The counter-authority, recorded rather than omitted.** Anthropic's bundled API reference states that
an official SDK is the default whenever one exists for the language. That guidance is about *its own*
API and the departure was already argued; under the ruling it is doubly off-target, since the endpoint
M1 calls is usually not a vendor API at all.

**The direction of the error matters and it is favourable.** A wrong request shape fails immediately,
visibly, and with the offending field named (C41). This is not a boundary where under-inclusion is silent.

**What re-opens it:** streaming, cache breakpoints, or a response shape too irregular to decode by hand.
When that day comes the decision is re-argued with the feature in hand — and note that the numbers to
weigh it against must be **re-measured for whatever the candidate is then**, which is the lesson this
section just learned about itself.

---

## 11. Risks & Mitigations

| # | Risk | Mitigation |
|---|---|---|
| R1 | **Retrieval noise makes the context useless.** Measured: an unrelated project's node at rank 7 (C28) | The record shows it, per run, with scores. §6.2's falsifier is a ten-run experiment on unit A. Milestone 2 is the durable answer |
| R2 | **The budget starves a run.** One 42,978 B node (C23) can consume most of 60,000 | Legible in the cut column. ~~§6.3 states the back-fill alternative and why it loses.~~ **CORRECTED 2026-09-04 (#11158): §6.3 now states the back-fill as the rule.** *"If the record shows it repeatedly, the rule changes with evidence"* is the sentence this row should be read for — the record showed it twice and the rule changed (§6.3, §11 R13) |
| R3 | **Deictic input retrieves noise.** Measured, spectacularly (C27) | Stated, not hidden. M1 does not claim to solve deixis; #10424 §5.1 assigns it to the tiers and to `intent`, both milestone 3. The anchor gives every run *something* stable regardless |
| R4 | ~~**The anchor is exempt from the budget**, so a pathologically large subject node is an unbounded input — and after §6.4a bounded the recall path, **it is the only one left**~~ **STRUCK 2026-09-05 (#11335, E1). The anchor is charged; what is left is two named residuals — one bounded by the anchor's own size, one measured — rather than an unbounded input. See the correction in the next column, which is this correction set's home.** | **Re-derived in revision 3, and it changed direction.** The measured ceiling in this graph is unchanged — 42,978 B (C23) — but revision 2 judged it against the *budget*, where an oversized anchor merely crowds the candidates, and concluded *"no instance exists"*. §8.4 now judges it against the *window*, where the anchor is the one term that can push a run past what the endpoint can hold: on a 32,768-token window a worst-case run leaves about **12,500 bytes** for it, and this graph already contains a node **3.4× that**. **The instance exists and is measured.** The fix is unchanged and still cheap — truncate with an explicit marker and record the truncation — but it is a change to unit A's shipped assembly, so revision 3 records it with the number rather than making it. **Falsifier, now answerable rather than hypothetical:** run any node over ≈ 12,500 B as the subject against a 32K-window endpoint. **CORRECTED 2026-09-05 (#11335, ruled by #11365 §4). The falsifier was answered by measurement and the accounting changed; the remedy this row proposed is not the one that shipped.** `Assemble` now spends `remaining = AssemblyByteBudget − anchorSize`, floored at zero, and admits candidates against that remainder rather than against the whole budget. **`renderBlock` is untouched — the anchor renders whole and is never cut**, which #11335 is ruled on: a turn whose subject was dropped for size answers about nothing. **So the remedy stated above — *"truncate with an explicit marker and record the truncation"* — was rejected as the mechanism while its goal was adopted**, the same shape R13 took under #11158 B5. Truncation of the *anchor* is a live proposal, but it belongs to compaction (#11365 §3), not here. **The invariant, and it names two objects rather than one — corrected 2026-09-05 after review.** The budget governs **content bytes**; `Assemble` returns a **rendered string**, and `renderBlock`'s `===== ANCHOR =====` / `===== CANDIDATE =====` banners, `id:` / `type:` / `name:` headers and section newlines are framing that no budget sees. So the statement is two residuals, not one: `len(block) = anchorSize + Σ admittedBytes + framing(1 + admittedCount)`, in which **only `anchorSize + Σ admittedBytes ≤ max(anchorSize, AssemblyByteBudget)` is bounded by this change**, and **framing is a second, unbudgeted residual measured at 178–2,444 B across the sweep, mean 1,384 B** — about 178 B for the anchor section plus 120–190 B per admitted candidate, worst on a thirteen-candidate row. §6.3 carries the same statement as a diagram, beside the layout that produces it. **So #11365's own falsifier F3 — *"no block exceeds `blockBudget`"* — passes on content bytes and fails on the rendered artifact, and the yardstick is the whole of the difference.** On content bytes, **22 of 23 rows respect 60,000** and one does not: r05, at 70,660 B, because its anchor alone is 70,660 B. **On `len(block)` — what `Assemble` actually returns — 5 of 23 respect it and 18 do not, the worst at 70,838 B**, which also breaks the `max(anchorSize, …)` form, because framing sits outside it. The content-only yardstick is this document's pre-existing convention rather than one adopted to flatter the result, but **F3's own words are about the block**, so both figures are stated and neither stands alone. **Why 1,384 B is not a quibble, and why it lands on the next unit: framing does not shrink with the dial.** #11365 F5 — the product of this very change — makes `blockBudget` a dial to sweep *downward* to answer #11364's floor question. At 60,000 framing is under 4% of the block. **At `blockBudget = 4,000` with twenty candidates it is roughly 3,100 B — about 78% of the dial's value — and the model receives ~7,100 B while the constant reads 4,000.** A floor measurement taken that way is wrong by nearly 2×, in the direction that makes the harness look like it survives smaller windows than it does. What r05 no longer carries is a single extraneous candidate byte: `70,660 + 59,723 = 130,383 B` of content before, `70,660 + 0 = 70,660 B` after (70,838 B rendered). **Measured on the live graph either side of the change** (`corpusHash ffa291d5`, `candidateLimit=20 assemblyByteBudget=60000 recallScopeReserve=3`): retrieved **11/23 → 11/23**; admitted **9/23 → 9/23, and a different nine**; mean documents per block **8.09 → 6.91**; shutouts **0/23 → 1/23** (r05). **Answer-neutral, and it is a trade rather than a free win:** r02 is rescued — a tighter budget starves the 29,325 B candidate at rank 4 — and r05 is lost, its anchor leaving zero candidate room. **What survives as a live risk: both residuals, and they are different in kind.** The **anchor** residual is that a subject node larger than 60,000 B still produces a block larger than 60,000 B, and one exists in this graph today; **closing it needs anchor compaction — #11365 §3, the next unit — and that unit carries an unmeasured risk this one does not:** compaction is measured to establish byte headroom and is *not* measured to preserve meaning, which is #11365 F1's whole subject and the reason it is a separate decision rather than an extension of this one. The **framing** residual is that no budget sees the render, on any run; **it is small at 60,000 and is not small at the bottom of F5's sweep**, per the arithmetic above. Both land on the unit after this one, and neither is repaired here. **Falsifier for what is claimed now, and it takes two readings because the claim names two objects:** sweep the corpus and read **(a)** `anchorSize + Σ admittedBytes` per row against `max(anchorSize, 60,000)` — one row over that falsifies the bound, and more than one row over 60,000 B falsifies the anchor residual's claim to be a single, anchor-sized remainder; and **(b)** `len(block)` per row against the same figure. **The gap between the two readings is framing**, and a gap outside the 178–2,444 B band recorded here falsifies the framing residual as stated |
| R5 | **Run records flood the graph.** One node per request | Deliberate and visible at M1, where a human drives every run. #10424 §5.7's retention tiering is the durable answer and belongs with the milestone that has volume to tier. **Revision 3 adds the size, which nobody had stated:** a record carries the block verbatim, so one run writes a node of roughly ~~**50,000–110,000 B** — **10 to 19× the median node in this graph** (C23: 5,758 B).~~ **CORRECTED 2026-09-05 (#11335, E7): both numbers were derived from a block of `AssemblyByteBudget + anchorSize`, and the anchor is now inside the budget. The block's content is `max(anchorSize, AssemblyByteBudget)` plus render framing (§11 R4), so an ordinary run's record shrinks by the anchor's size — a measured mean of 12,174 B (#11365 §4) — and only a run whose anchor exceeds 60,000 B still reaches the old upper end. The multiple against C23's 5,758 B median narrows with it. The row's point is untouched: one run still writes a node an order of magnitude above the median.** Ten runs put more bytes into this neighbourhood than the ≈22 nodes it currently holds (C25). That is a fact about the graph; **R13 is the fact about the runs that follows from it** |
| R6 | **The success response shape is unmeasured** (§9.5; C41 measured only the route, the auth header and the error shape) | Fixture-driven decoding, isolated in one adapter, blast radius one file. **And the mitigation got stronger under the ruling:** confirming it needs a local runtime rather than a credential, so §14 makes it a condition of hand-off instead of a post-merge hope |
| R7 | **Cost per run is unmeasured** — and after the ruling, *cost* means two different things | Against a local endpoint the marginal cost is machine time, not money, which is the ruling's point. Against a paid endpoint it is ~15,000 input tokens ~~plus the anchor~~ **CORRECTED 2026-09-05 (#11335, E7): the anchor included, except on a run whose anchor alone exceeds the budget (§11 R4)** per call. The record carries usage, so ten runs turn either estimate into a number. #10424's framing still applies: *"more expensive per task but better/faster results is a valid outcome and a business decision"* |
| **R11** | **An endpoint does not honour §6.6's subset** — the family is a de facto standard with no conformance suite | §6.6 states the subset, keeps it deliberately small, and §6.5 makes the failure loud and immediate rather than silent. **Falsifier:** point it at two different runtimes; if D1–D6 do not both hold, the subset is too large and must shrink |
| **R12** | **A local model is too weak to demonstrate the milestone's own claim.** S2 asks whether the assembled context is legible *to a model*; a small quantised local model may answer badly for reasons that have nothing to do with the assembly | **Named because the experiment could otherwise be misread as a design failure.** Unit A already separates the two: it shows the assembled block to a *human*, with no model involved, which is where S2 is actually judged. A weak model degrades S1's demonstration, not S2's evidence. **Falsifier:** run the same input against a local model and a strong hosted one; if only the block's legibility is in question, both should be able to use it |
| R8 | **The model treats the block as a transcript** and answers from position rather than relevance | §8.6's system text establishes what the block is. Falsifiable on the first live runs by reading the answers |
| R9 | **Graph latency dominates.** ~2 s per run before the model is reached (C20, C23, C29) | Accepted at M1. The two reads are independent and could run concurrently — one obvious optimisation, deliberately not taken until a measurement says it matters |
| R10 | **The seams get load-bearing** and the design drifts toward interface-per-service | §9.2's falsifier is checkable by inspection: any interface without an experiment behind it goes |
| **R13** | **Run records are unfiltered recall candidates, and they are copies of earlier prompts.** M1 writes a `session-log` node per run (§8.3) into the same graph its recall query reads, and nothing scopes that query (§6.2). So run *n*'s candidate set can contain run *n−1*'s record — a node 10–19× median size (R5) whose body is a **verbatim copy of a block the run already has**, and which under §6.3's stop-don't-skip admission can consume the entire assembly budget and cut all nineteen other candidates | **Named in revision 3; not fixed here, and the distinction matters.** §6.2's argument against filtering was made about *human-authored* content and it still holds; **self-produced content is a case it does not cover**, and this design did not notice that writing into the pool it reads from changes that premise. The remedy is measured and cheap — C25 confirms `type=` composes with `query=`, so excluding the run-record type is one query parameter — but it is a change to unit A's shipped read path, and **no run record has ever been written, so the evidence for it does not exist yet.** **Falsifier, and it produces that evidence in ten runs:** run ten, then read the eleventh's candidate set. If `session-log` nodes appear above rank 5, or if one is admitted and cuts the rest, the exclusion goes in with a measurement behind it — the same standard R1 and R2 are held to. **FIRED AND RULED 2026-09-04 (#11158, B5). It took two runs, not ten:** the record ranked first, could never be admitted, and cut all nineteen behind it (#11141). The goal is adopted and **the proposed mechanism is not** — neither branch this row named shipped. A `type=` exclusion filters the row before it is written down (§9.4 obligation 1 forbids it) and node type alone cannot carry the distinction, because human-written session logs carry the same type and were ranks 2 and 3 in the failing run. **The row is cut at admission instead** (§6.3), so it stays retrieved, ranked and recorded. **What survives of this row as a live risk:** run records still occupy retrieval slots out of the candidate limit. That is countable per run from `candidates[]`, and it is what would justify a query-level change later — with the measurement §6.2 has always demanded |
| **R14** | **M1 does not fit an 8,192-token window, and that is a real size among the ruling's own target runtimes.** §8.4's ceiling table: the assembly budget alone is ≈ 15,000 tokens, **183%** of such a window, ~~before the anchor and~~ **CORRECTED 2026-09-05 (#11335, E7): the anchor is inside that 183% now, so the figure is the whole block rather than a floor under it — the row is unchanged in verdict and slightly sharper in statement** before any tool use | **Named rather than repaired, because the constant that causes it is unit A's and is shipped.** The tool path is not the cause — §6.4a bounds it, and a run with zero tool calls overflows an 8K window just as badly. The fix, when there is evidence for it, is `AssemblyByteBudget`. The honest position today is that **M1's floor is a 32,768-token endpoint**, and §13.6 asks whether that is acceptable. **Falsifier:** point it at an 8K runtime. The first call fails or truncates, §6.5's rows carry it, and the number to change is in §8.4 |

---

## 12. Migration / Rollout Strategy

Nothing to migrate. Rollout is §2.3's two PRs, in order.

**The seams M1 must not foreclose, and how each is left open:**

| Later milestone needs | M1 leaves it open by |
|---|---|
| Three-tier assembly | Assembly being a pure function of its inputs. Tiers change *what goes in*, not *where it runs* |
| Supersession, freshness, hybrid retrieval | The graph port's recall operation being one call with one query. More retrieval means a richer operation behind the same port |
| Stable byte runs and caching | The block already being stability-ordered, never score-ordered (§6.3) |
| The retrieval eval corpus | The record already carrying the whole candidate set, with scores and hashes (§9.4) |
| The memory core | The write port already being *"record what happened"*, with the adapter owning structure (§8.3) |
| **A second provider** | The model port's contract being the loop's vocabulary rather than a wire format (§8.3), and `main` constructing the adapter. A second provider is a sibling package and a construction change. **Nothing about it is declared now** — not a selection member, not a protocol enum, not a registry (#1220 §2) |
| Graph-driven configuration | One environment read site, values travelling downward (S6), and the system text already being an injected value (§8.6). **The ruling improved this seam:** the model endpoint and model id are *not secrets*, so unlike a key they are exactly the kind of thing that can move into the substrate later — see §13.2 |
| `intent`, workflow steps, gates | Nothing declared. The prose list in §2.2 is the whole of their presence |

**No predecessor design is superseded.** #10437 and #10488 remain live and correct; this document consumes
them.

---

## 13. Open Questions

### 13.1 ~~A model API key — for Toni.~~ **WITHDRAWN. Answered by the 2026-09-01 ruling, by removing the need.**

This asked Toni for an Anthropic key, and recorded C34 — no credential on this machine — as blocking
unit B's live verification. **The ruling answers it not by supplying a credential but by removing the
requirement for one.** Against a local OpenAI-compatible runtime there is nothing to procure, nothing to
spend, and no terms-of-service exposure.

**What replaced it is much smaller and is not Toni's:** a local runtime has to be installed, because C40
measured that none is. That is an implementer's step, so it is not an open question — it is §14 step 14,
and it is a condition of hand-off rather than a wish.

**Two things elsewhere in this document leaned on the withdrawn blocker and were re-derived rather than
patched:** §9.2's model-port justification (withdrawn and replaced) and §2.3's second reason for the unit
split (weakened, with the split surviving on its other two).

### 13.2 The vision's **two** boot inputs — the correction is right, and the ruling makes it sharper

#10424's third inversion: *"The binary needs exactly two things to boot: a DiVoid URL and an API key.
Everything else is data it reads from the substrate."*

**The correction has been made** — `VISION.md` and #10424 now read *"a graph URL, a graph key, and the
credentials for whatever else it must authenticate to."* **The ruling makes that sentence better than
this section knew, and the vision should carry the sharper version.**

The original argument was: a model key is a secret, secrets cannot live in the graph, so the floor is
three. **Under the ruling the floor in the local case is two again — genuinely, not by rounding.** With
an optional key and a local endpoint there is **no second credential at all**, so the amended sentence's
third clause is *empty* and it reads exactly as the original did.

**And the reason is more interesting than the arithmetic, which is why it is worth writing down.** The
ruling splits what used to be one lump into two kinds of boot input:

| | Can it live in the graph? |
|---|---|
| Graph URL | **No, ever.** You cannot read the graph to find the graph. This one is irreducible by logic |
| Graph key | **No.** A secret in a shared substrate is a leak |
| **Model endpoint URL, model id** | **Yes — they are not secrets.** They are ordinary data that M1 puts in the environment only because graph-driven configuration is not built yet |
| Model key, and any future provider credential | **No** — and in the local case there is none |

So the honest general statement is: **the irreducible environment inputs are the graph URL, the graph
key, and credentials for whatever else must be authenticated to — and that list is complete.** Everything
else is data. In the local case the third clause is empty and the floor is two. The vision's original
sentence was not wrong about the floor; it was under-specified about the third clause, and the amendment
supplies it.

**One caveat, so the good news is not oversold.** The *count of boot members* went **up** in revision 2,
from four to six — the model endpoint and model id are new. Those are different quantities: the vision is
about what is *irreducible*, and this design is about what is *wired today*. Both numbers are true and
they move in opposite directions, which is exactly why the sentence is worth stating precisely.

### 13.3 "One model call" versus "one tool" — resolved with a position, reversible in one line

§2.4 carries the argument and the lever. Flagged here because it is a reading of the milestone statement,
not a deduction from it, and Toni may simply have meant zero tools.

### 13.4 Should `POST /runs` be the surface, or should the input arrive from the graph? — for Toni, low stakes

M1 picks HTTP because M0 serves HTTP and because a human must be able to drive a run and read the result.
A defensible alternative exists: poll the graph for a node of a given type and treat *that* as the input,
which would make the harness graph-driven on both ends. It costs a poll loop and a marker convention, and
it makes the run harder to observe. One endpoint to change if the answer is different.

### 13.5 Code Contracts #114's Go annex — **ruled 2026-09-02 in part**; one question of it stays open

~~#10437 §13.1 raised it, #10488 §13.2 restated it, task **#10495** is open. This design leans on nothing
from it (see the header). Three consecutive designs recording the same gap is itself the finding.~~

**CORRECTED 2026-09-02.** The annex exists — **#10861**, ruled by Toni on PR #6 — and it **supersedes the
scoping struck above insofar as that scoping excluded §4**. The paragraph is retained struck rather than
rewritten because PRs #1–#3 shipped under it and a reader needs to see what was believed and when.

**What binds on this repo:** #114 **§0** (principles), **§4** (comments), **§11** (logging discipline),
**§13** (parallel-by-default, no shared fixture state), **§13.1.1** (guard-axis rule). §4 is the
correction: the struck list named four sections and omitted the one that was contested.

**§4 and #10861 are cited, not restated.** Read §4 at #114 and the ruling at #10861 — the ruling carries
the channel-by-channel table, the Go replacement for §4's `[Description]` escape hatch, and the fact that
no Go tool in this repo's gates enforces any of it. A design that transcribes a contract loses the
contract's exceptions; #10861 was itself revised on the day it was written for exactly that reason, so
this document does not repeat the mistake by paraphrasing either one.

**Why this stays an open question:** **question 3 of #10495** is unresolved — what replaces §16's pre-PR
checklist for a Go module. §16 is written against XML docs, `var`, `Pooshit.Json`, `[TestFixture]`,
`Assert.That` and `WebApplicationFactory`, none of which have Go referents, so QA still has nothing
Go-shaped to check against. Questions 1 and 2 are settled by #10861; #10495 stays open on question 3
alone.

**What the wrong scoping cost, recorded rather than glossed.** Four QA reviews (#10493, #10821, #10829,
#10835) passed over the comment channel on the authority of the struck paragraph, and none was wrong
under the authority it had. QA **#10862**, run under the ruling, measured the result on this branch: 890
comment lines in 185 groups against 3,603 added Go lines — **24.7% of the diff** — of which 2 of 137
doc-comment groups meet §4's bar. "Our design mandates the text" is not a defence; where it is true, the
design is the defect and gets fixed here too.

### 13.6 The window class M1 targets — for Toni, and it is a number rather than a preference

§8.4's ceiling table says M1's floor is a **32,768-token** endpoint, and that an **8,192-token** one
cannot run it at all (§11 R14). That is derived, not measured: **no runtime has been pointed at this yet**
(C40), so the window sizes come from what local runtimes commonly serve rather than from what the one you
install actually reports, and the four-bytes-per-token ratio underneath them is an assumption (§9.5).

**The question is one line: is a 32K floor acceptable, or does M1 need to run on 8K?** If 8K, the number
that has to move is `AssemblyByteBudget` — unit A's, shipped — which is exactly why this is a question
here rather than a decision taken here. **Low stakes and cheaply answered:** §14 step 14's live run
reports the real numbers, and it now records the endpoint's advertised context window alongside the wall
clock for this reason.

---

## 14. Implementation Guidance for the Next Agent

No code appears in this document by design. The order below is architectural, not procedural.

### Unit A — assembly (PR 1)

1. **Configuration.** Add the two DiVoid members at the **existing single read site** in `main`. Required
   semantics per §8.1. Test all three cases per member — absent, present-verbatim, present-empty — with
   **literals on the expected side** (#10466: a constant shared with production moves both sides together
   and the assertion can never fail).
2. **`internal/divoid`, read side.** The two read operations. Pin at the wire level against a local test
   server: query construction, header presence, decoding, and **explicitly the C30 empty-result
   discrimination** — a test that a missing subject is detected, and a mutation check that a
   status-code-based check would pass wrongly.
3. **`internal/loop`, assembly.** The pure function and the record. **The golden test is the deliverable
   here**: fixed candidate rows in, one byte-exact block out. Separately pin: ~~admission stops rather
   than skips (§6.3)~~ **CORRECTED 2026-09-04 (#11158, B6) — admission skips rather than stops, and a
   self-produced row is cut before the byte test and charges nothing (§6.3)**; every candidate is hashed
   and sized including the cut ones (§9.4 obligation 1); the render order is by id and a score reshuffle
   does not move a byte.
   **EXTENDED 2026-09-05 (#11335, E10) — two pins this step did not ask for and the shipped code now owes.**
   First, **the anchor is charged against the budget before the first candidate is considered**: pin a run
   whose anchor consumes most of the budget and assert that a candidate which would have fitted the whole
   budget is cut, with the byte-budget reason and its size still recorded. Second, **the subtraction floors
   at zero**: pin an anchor larger than the whole budget and assert that a one-byte candidate is cut rather
   than admitted against a negative remainder, and that the block still carries the anchor whole — the
   anchor is never cut (§11 R4). The shutout WARN's new reach is §10.3's pin, not this step's.
4. **`POST /runs`.** The handler, with the loop as **a parameter to the handler-building function**
   (#10466 archetype C). Status codes and the envelope per §8.5.
5. **Before handing off, mutate and watch it redden.** Change the render order from id to score and
   confirm the golden test fails. Remove the cut candidates from the record and confirm a test fails.
   Make the not-found check status-based and confirm a test fails. **A suite nobody has seen fail has not
   been shown to be an instrument** (#10466).
6. **README:** the new endpoint, the two variables, and an honest "what is and isn't verified" entry
   naming the absent live run.

### Unit B — the turn closes (PR 2)

7. **Configuration.** Three members at the same site: `PROCESSOR_MODEL_URL` and `PROCESSOR_MODEL_ID`
   required, `PROCESSOR_MODEL_KEY` **optional**. The optional one is a sibling of the shipped
   `requireEnv`, not an exception to it. **Test all three cases for it explicitly** — absent, empty,
   present — and assert the one that matters: **absent sends no `Authorization` header, empty is a
   startup error naming the variable** (§8.1). A test that only covers absent-and-present will not catch
   a silent auth downgrade.
8. **System text.** Its *wording* is a prompt-engineering deliverable against §8.6's requirements —
   route it to `kim-prompt-engineer` rather than improvising it. Its *placement* is architectural and is
   settled: a value constructed in `main`. **Note for whoever writes it:** the target now includes small
   local models, so it should not assume a frontier model's instruction-following.
9. **`internal/openaicompat`.** Request body per §6.6's D1–D6; `Authorization` only when a key is
   configured; its own timeout constant per §8.4a, **not `divoid.DefaultTimeout`**. Pin the **exact
   request body** byte-for-byte at the wire level, and decode a fixture of each response shape: text
   only, a tool call, a truncated answer, and a terminal reason the mapping does not recognise.
   **Pin the translation, not just the decoding** — assert that an unrecognised reason maps to the loop's
   unrecognised value *and* that the raw string survives into the record, and that a missing usage object
   yields absent rather than zero. **Revision 3 (§8.3):** the counts are *in* and *out* and there is no
   total — assert that an endpoint reporting three fields yields two, and that an endpoint reporting a
   usage object carrying only one of the two counts yields **absent**, not a half-zero-filled object.
    **Then check the neutrality falsifier before going further** (§9.2): grep `internal/loop` for any
    provider vocabulary — wire field names, finish-reason strings. It must return nothing. Two minutes,
    and it catches the one way this design fails quietly.
10. **The tool cycle.** Definition, dispatch, cap. Pin from canned port responses: a clean single call, a
    single tool round trip, the cap firing, and a malformed tool input producing an error-flagged result
    that does not abort the turn.
    **And the budget (§6.4a), which revision 3 adds and which is where this unit was rejected once.** The
    loop admits hits in rank order against `SupplementaryByteBudget`, ~~stops at the first that does not
    fit, back-fills nothing,~~ **CORRECTED 2026-09-04 (#11158, B6): skips what does not fit and
    back-fills what does, cutting self-produced rows first — both paths call the one `admit` (§6.3)** and
    exempts nothing. Pin a round whose hits straddle the budget and assert
    **both** the admitted set and that *every* row still reaches the record with its admit-or-cut
    decision. Pin the round that admits **zero** because its best hit is oversized (§6.5's new row). Pin
    that the error branch is bounded and carries no address (§8.5). **And assert the ceiling as a
    literal:** `100,000` against `AssemblyByteBudget + SupplementaryByteBudget × (MaxModelCalls − 1)` —
    the literal on the expected side per step 1, so that moving any constant without re-deriving §8.4's
    window table turns this red.
11. **`internal/divoid`, write side.** The three-POST sequence (C32). Pin at the wire level: the order,
    the content-type header on the body POST, the bare-id body on the link POST, and that the **adapter**
    supplies the type, name and edge. Pin at the port level that the loop supplies none of them.
11a. **The record's own shape, which is what milestone 2 actually consumes** — new in revision 3, and the
    part of this unit #10821 found unpinned end to end. Pin it **at the wire level, through a decode
    struct with literal JSON tags**, with fixture values distinctive enough that a wrong field cannot
    pass: `answer`, `model`, `toolCalls`, `modelCalls`, `capReached`, `usage`, `stopReason`, ~~`written`~~
    and `limits`. **STRUCK 2026-09-02 (#10899, A4): `written` is no longer a member of the record, so
    pinning it here would direct an implementer to assert a field that must not exist. The write receipt
    is a response-only key; `docs/architecture/run-record-fate.md` §8.1 states what to pin instead — that
    the stored body is the response body minus exactly that one key. Line 829's list is left standing:
    it is a true statement about what unit A shipped, and history is not corrected.** Three of these are rules rather than fields and deserve their own assertions:
    **`usage` has exactly one entry per model call, in order, empty where a call reported nothing**;
    **`toolCalls[].results` carries every row the round returned, not the admitted subset**, with the
    same columns `candidates[]` has; and **`limits` reports the constants actually in force** — the one
    place in this suite where asserting against the production constant is correct, because that field
    exists to *be* their value. Everywhere else, literals (step 1).

12. **The §6.5 table, every row.** Each is a test. The `graph_unavailable`-before-the-model row is the one
    that matters most: assert that a recall failure produces **no model call at all**.
13. **Mutate and watch it redden**, same discipline as step 5. In particular: make a recall failure fall
    through to an empty-context model call, and confirm a test fails. **Three more that revision 2 adds,
    because each is an invariant a plausible refactor would quietly break:** make an *empty*
    `PROCESSOR_MODEL_KEY` mean "no auth" instead of an error and confirm a test fails (§8.1's silent
    auth downgrade); zero-fill a missing usage object and confirm a test fails (§6.5); and drop the raw
    terminal reason from the record, keeping only the mapped one, and confirm a test fails (§8.2).
14. **The live run, in the container — and this is now a condition of hand-off, not a post-merge hope.**
    Install a local OpenAI-compatible runtime (C40 says none is installed here), point
    `PROCESSOR_MODEL_URL` and `PROCESSOR_MODEL_ID` at it, leave `PROCESSOR_MODEL_KEY` **unset** — that
    path is the ruling's own target and must be the one exercised. Record what came back: the answer,
    the usage (or its absence), the raw terminal reason, the wall clock — which §9.5 names as the largest
    unmeasured quantity in the milestone — and, **new in revision 3, the endpoint's advertised context
    window and the input-token count for a prompt of known byte length.** Those two close §9.5's other
    open rows: they turn §8.4's window table and its four-bytes-per-token ratio from derivation into
    measurement, and they answer §13.6. **This closes R6 and feeds §8.4a's timeout a real
    number instead of a judgement.** It is a manual command; it does **not** become a gate (§10.6).

### Do not add

Retries · backoff · a circuit breaker · streaming · caching or cache breakpoints · a config knob for any
constant in §8.4 · ~~**a back-fill or a size exemption on either budget (§6.3, §6.4a)**~~ **CORRECTED
2026-09-04 (#11158, B6): the back-fill is now the rule (§6.3); a size *exemption* is still forbidden on
either budget** · **CLARIFIED 2026-09-05 (#11335, E10): still forbidden, and the anchor is not a
counter-example. The anchor is exempt from being *cut*, never from being *charged* (§11 R4) — a new
exemption of either kind is what this line forbids** · ~~**a type filter on the recall query (§11 R13 — it needs a measurement first)**~~
**CORRECTED 2026-09-04 (#11158, B6): still forbidden, and no longer pending a measurement — the
measurement was taken and the ruling is that the query stays unscoped (§11 R13)**  · middleware · a `/health` dependency check · an `intent` field · anything from §2.2's
prose list · a second environment read site · a dependency (§10.9) · **a second provider, a
provider-selection member, or capability probing of the endpoint (§6.6)** · **a provider's field names
anywhere in `internal/loop` (§9.2)**.

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

| Item | Answer |
|---|---|
| No new type mirroring an existing one | ✓ The run record is the only new composite. Nothing in the module resembles it |
| No abstraction with one implementation and no concrete second | ✓ Two ports, each with a concrete second implementation (the test double) **and** an experiment justifying it (§9.2). §9.2's falsifier is stated so a later reader can re-check rather than re-argue |
| No element justified by "we might need X later" | ✓ The content hash is the closest call and is argued on present cost and a **non-speculative** later cost, in §7. Everything in §2.2's prose list is excluded precisely on this ground |
| No deprecation period, feature flag, compatibility shim, transition window | ✓ None. Nothing exists to be compatible with |
| `block_size × site_count` quoted for any inline-at-N-sites decision | **Not applicable — no such decision exists.** No multi-line block recurs at more than one site. The three-POST write sequence occurs once, in one adapter |

**Existing systems first**

| Item | Answer |
|---|---|
| Audited whether an existing surface covers this | ✓ The module is M0's two packages; neither can host a loop, a graph client or a model client. The *retrieval primitive* was audited against DiVoid and **is** reused rather than reimplemented (C20) — #10424 §6's split, discharged |
| Concrete reason a new layer can't live on an existing surface | ✓ §5.1 runs the merge test in both directions and records both outcomes; §5.6 lists the components declined |
| Concrete 4-week decision for any new persisted data point | ✓ The run record's reader is **this milestone's own success criterion S2**, and milestone 2 is the second named reader (§9.4). Every field in §8.2 is displayed |
| Consumer chain recursed for anything justified by "an existing reader projects it" | ✓ Applied to `/health` (§10.8): the reader does not exist, so the field does not |

**Configurability**

| Item | Answer |
|---|---|
| Every knob has a named operator or an environment difference | ✓ Six members (§8.1). Five differ by environment by construction — a dev graph vs. a prod graph, each environment's own keys, and (new in revision 2) each endpoint's own address and model name, which vary between two local runtimes let alone between environments. §8.1's second table lists the four tempting members that stayed constants |
| Every "telemetry-then-tune" knob has a filed task naming the reader and the event | ✓ **None exists** — every tunable value ships as a constant (§8.4), so the rule does not bind |
| Magic numbers that need not vary stay named constants | ✓ §8.4, with the §3 test applied to each |

**Less is better**

| Item | Answer |
|---|---|
| Every element passed delete / merge / inline | ✓ Recorded, not asserted: the tool survives a **failed** delete test and is kept on a stated, weighed argument (§2.4); assembly was *merged* into `internal/loop` and the loop was *not* merged into `internal/server` (§5.1); `/health` growth was *deleted* (§10.8); retries, idempotency and streaming were *deleted* (§10.5, §10.7, §2.2) |
| Trade-offs named where a more complex design wins | ✓ Five: the two ports over direct construction (§9.2, **re-derived in revision 2 on a new justification after the old one lapsed**); the tool over no tool (§2.4); two PRs over one (§2.3); the model port's neutral vocabulary over passing the wire shape through (§8.3); raw HTTP over the SDK (§10.9) — that last one being the *simpler* option winning on a measurement |
| Radical-clean shape chosen where the existing surface has no consumer | ✓ `/health` (§10.8) and the §2.2 prose list. The compromise shape #1136 §4 rules out — declare the members, leave them unread — is the shape M0 already refused and this design does not reintroduce |
| Reader inventories cover AST *and* string-literal references | Not applicable — nothing is renamed or removed |
| Carrier-swap tables enumerate every affected DTO | Not applicable — no DTO changes |

**Data deliverables** — not applicable. No SQL, no migration, no backfill. The graph writes are three API
calls (C32), not schema changes.

**Document discipline**

| Item | Answer |
|---|---|
| Cites #114 and #1136 as load-bearing | ✓ Header, and applied throughout |
| Reader / scope inventories explicit | ✓ §2.1, §2.2 (with destinations), §5.6, §14's do-not-add list |
| Out-of-scope listed explicitly | ✓ §2.2, as a table |
| No multi-paragraph rationale for things that obviously stay | ✓ §7 is three rows and one paragraph, and the paragraph exists only because the hash is a genuinely close call |
| Predecessor design marked superseded | Not applicable — §12 records that #10437 and #10488 stay live and are consumed unchanged |

**Falsifiable-universals check (#1220 §5 addendum).** Every universal in this document names what would
break it:

| Claim | Falsifier |
|---|---|
| *"No LLM chooses what is retrieved"* (S3) | Any model output feeding the assembler's query. The supplementary tool is additive and post-assembly; if a tool result ever re-entered the assembled block, the claim would be false (§6.4) |
| *"The environment is read in exactly one place"* (S6) | More than one lookup site in the module |
| *"Assembly is deterministic and byte-pinnable"* (S4) | Any clock, randomness, map-iteration order, or score-dependent positioning in the render path |
| *"The default suite is fully offline"* (§9.3) | Any test that opens a socket to a host other than a local test server, or that requires a credential |
| *"Every seam has a measurement"* (§9.2) | Any interface with one implementation, one test double, and no deletion experiment behind it |
| *"The record makes recall computable"* (§9.4) | A record that omits any candidate the query returned |
| *"Two runs cannot interfere"* (§6.5) | Any package-level mutable variable in `internal/loop` |
| **"The loop never sees a provider's vocabulary"** (§8.3, the ruling's requirement 1) | Any provider field name, wire key or finish-reason string appearing in `internal/loop`. Checkable by grep, and §14 step 9a makes it a build step rather than a hope |
| **"Present-but-empty is an error for every configuration member"** (§8.1) | Any member for which an empty value is accepted — including the optional one, where accepting it would be a silent auth downgrade |

**And two claims are deliberately *bounded* rather than universal**, per the same addendum's preference:
the block is byte-stable *for a fixed candidate set* (not across graph drift — §9.4), and retrieval is
deterministic *given graph state* (measured C22, not asserted in general).

**~~Four~~ Five of this document's own claims have now been measured against and did not survive intact**
*(the fifth is 2026-09-05's, and it is a different class — stated directly below the table)*. **Each is
recorded where it stood rather than repaired in place**, because a decision propped up by a cost nobody
can re-read is a decision nobody can re-examine (#10488 §3.5's lesson, applied repeatedly to this
document's own strongest arguments):

| Claim as first written | What measurement did to it | Where |
|---|---|---|
| A dependency *breaks* the container gate | **Weakened.** It does not break it; it makes the gate network-dependent and roughly doubled its cold time on the then-candidate | §3.4 (revision 1) |
| …and the dependency tree is the reason | **Weakened again, on re-measurement.** The new candidate costs 5 modules and +27 %, not 11 and +90 %. The module-count argument no longer carries anything; **`--network none` carries all of it, and does not depend on size** | §10.9, C42/C43 |
| The model port is justified because those tests are otherwise *impossible* | **Withdrawn.** With a configurable endpoint they are possible, just worse. The seam survives on a **different and better** justification — provider substitution, a product requirement rather than a test affordance | §9.2 |
| No retry, partly because a retry *spends money twice* | **Weakened.** False against a local endpoint. The rule survives on the unmeasured-constants argument alone, which was always the stronger half | §10.5 |

**A fifth, added 2026-09-05 (#11335), and it is a different class — which is why it sits below the table
rather than in it.** *"The anchor is exempt from the budget"* (§11 R4) was not a reason that lapsed under a
surviving conclusion; it was **the conclusion itself, reversed**. Revision 3 re-derived it, judged it
against the window rather than the budget, found the instance and still declined to change it. Measurement
then said the constant did not bound the artifact it named — blocks of 60,601–130,383 B against a constant
reading 60,000 (#11365 §4) — and the design changed. The table's own remedy applies unchanged (recorded
where it stood, struck rather than repaired), but **the pattern named below does not cover it**, and a
reader who generalises *"the conclusion always holds"* from four cases would have been wrong on the fifth.

**The pattern is worth naming for whoever revises this next.** In all four cases the *conclusion* held and
the *reason* did not — and in two of them the replacement reason is stronger than the original. That is
the argument for re-deriving rather than inheriting: had these been carried forward unexamined, the
design would have been right for reasons that had quietly stopped being true, and the next person to
question it would have had nothing solid to push against.
