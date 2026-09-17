# Architectural Document: Reframe Before You Escalate

> Repo path: `docs/architecture/reframe-before-you-escalate.md`.
> Not published to the graph. Parity is published at merge; this file is the canonical copy until then.
> Project: **#10422**.
>
> Written on `feat/reframe-before-you-escalate`, after PR #98's first round was rejected by QA with four
> criticals (CF-1..CF-4) and three warnings (W-1, W-2, W-3). QA reviewed that round as an uncommitted
> working tree on top of `af3e3d6`; the same content was committed afterwards as `87a0ad2`, which is the
> only commit on this branch carrying the rejected wording. This document is the fix round's design
> record — the `## Decisions taken autonomously` list that CF-4 said belonged in the repo, not only in a
> PR body. It also serves the retroactive role the original round skipped: the nudge shipped in PR #97
> and split into two tiers in PR #98 without a design document at either step, which is itself the
> defect CF-4 named (#114 §4: *"there is no third place"* — code carries the what, a design doc or the
> code map carries the why, nothing else does).
>
> Precedent consumed: `docs/architecture/retrieval-admission-and-the-empty-outcome.md` §4 (the
> `NeedsClarification` prescription this document departs from — see §5), `docs/architecture/
> what-goes-in-the-block.md` (what a block may contain).

---

## TL;DR

A run whose recall keeps coming up empty was getting one instruction — *"go research, go
experiment"* — at every point it happened, regardless of *why* it came up empty and regardless of
whether the model had acted yet. That collapsed three different situations into one piece of advice,
and for one of those situations the advice was backwards.

**Two axes, not one tier.**

1. **Before the model has acted** (`renderBlock`, the initial context block): the cheapest hypothesis
   for a thin or empty block is that the *framing* is off, not that the knowledge is absent. The nudge
   here says *try a different angle* — it never tells the model to go and act, because it hasn't had the
   chance to yet.
2. **After the model has issued a `recall` and it came back with nothing usable** (`RenderToolResult`):
   the reframe has already had its shot. What differs now is *why* nothing came back, and that decides
   the advice:
   - **Cut for byte budget** — the graph found real matches; they just didn't fit. The correct advice is
     to **narrow the query**, not to escalate. Telling a model that just found twenty oversized hits to
     go "research" is not aspirational, it is wrong — the knowledge is sitting right there.
   - **Cut below the floor, cut as self-produced, or nothing found at all** — the graph plausibly does
     not have this. The advice here is to **say so plainly and name what is missing**, which is what the
     shipped system text (`cmd/processor/system_text.go:16`) already tells the model to do — not a new
     "go research" instruction with no tool behind it.

The gate on `r.Tool == ToolRecall` (§4) makes the second axis structural rather than incidental: only a
`recall` round gets directive advice appended to its result.

---

## 1. What PR #98 got wrong, in the model's own terms

PR #97 (`feat/an-empty-block-says-go-and-learn`) added one nudge, fired only from `renderBlock`, worded
as *"you should explore more."* PR #98 split the reachability into two functions — `renderBlock` before
the model acts, `RenderToolResult` after a `recall` round completes — and reworded each accordingly.

QA's review (round 2) found the split was mechanically sound but behaviourally wrong at the tier-2 site,
for two independent reasons:

- **CF-2.** `RenderToolResult`'s all-cut branch fired for three different `CutReason` values
  (`cutReasonByteBudget`, `cutReasonBelowFloor`, `cutReasonSelfProduced`) and gave the same advice to all
  three. QA's probe: twenty rows at similarity 0.97, each 20,001 bytes against a 20,000-byte
  supplementary budget — all cut for byte budget, all real matches, and the model was told *"you have
  looked and found little, go research."* That advice is not merely unhelpful there, it is the opposite
  of correct: the graph found exactly what was asked for, twenty times over.
- **CF-3.** Even where escalation *was* the right call (floor cuts, self-produced cuts, nothing found),
  the wording — *"research it first, and fall back to experimentation only if research does not get you
  there"* — instructed a research step the loop has no lever for beyond `recall` itself, which the model
  had already exhausted to reach that branch. Worse, it landed as a `tool` message *after* the system
  message, making it the more recent instruction, and it contradicted `cmd/processor/system_text.go:16`'s
  already-shipped *"if you still do not have enough information, say so plainly and say what is
  missing."* A model holding `recall` as its only research tool, told to "research it first," calls
  `recall` again — into the thing that just failed, against a cap of six calls per turn
  (`MaxModelCalls`).

Both are fixed by the same change: branch tier 2 on `CutReason` rather than on "cut at all."

## 2. The fix

`RenderToolResult` (`internal/loop/assemble.go`), in the `len(r.Results) == 0` branch, now reads:

```go
if r.Tool != ToolRecall {
    return base
}
if anyCutForByteBudget(r.Dispositions) {
    return base + "\n" + nudgeNarrowQuery
}
return base + "\n" + nudgeEscalate
```

`anyCutForByteBudget` is a private helper: `true` if **any** disposition in the set was cut for byte
budget. That is deliberately an "any", not an "all": a byte-budget cut is affirmative evidence — the
model asked for something that exists and is too big — and that evidence does not stop being true
because *other* rows in the same batch were also excluded for a different reason (below floor,
self-produced). Given a mixed batch, telling the model to narrow the query is still the more actionable
half of the truth; there is no scenario in this design where "some of what you asked for was too big"
should be suppressed in favour of "nothing is here."

`nudgeNarrowQuery` names **only the lever the tool actually has**. `recall` takes exactly one argument,
`query` (`internal/openaicompat/wire.go`, `internal/ollama/wire.go` — *"Takes one argument: query"*, and the
JSON schema carries that one property); there is no slice, limit, offset or size parameter. Advice to "ask
for a smaller slice" would be the same defect as CF-3 relocated to the other branch — naming a lever the
loop does not expose — so the sentence stops at *narrow the query*.

`base` is the pre-existing report sentence (*"results were found, but none were included."* / *"no
additional results found."*), unchanged from PR #97 and untouched by this round's `admit`, relevance
floor, or cut-reason constraints — CF-2/CF-3 only changed what gets **appended** after it, and only for
`recall`.

## 3. Binary, not threshold

Tier 1's nudge is threshold-shaped (`thinKnowledgeThreshold = 5`, unchanged, unowned by this round —
its derivation lives in PR #97's body). Tier 2's is **not**. `anyCutForByteBudget` is a boolean over a
closed, three-valued enum (`cutReasonByteBudget`, `cutReasonBelowFloor`, `cutReasonSelfProduced`); there
is no "how many" question to answer, no sweep to calibrate, and no constant to own. If a future revision
wants graded tier-2 advice (e.g. "narrow it a lot" vs "narrow it a little"), that graduation would need
its own measurement in its own document — nothing here should be read as having decided that question
either way. This round asks only: *is at least one row real, but too large?*

## 4. The tool gate (W-3)

Before this round, `len(r.Results) == 0` was reached by anything that was not `ToolWriteFile` and carried
no error — correct today only because `recall` is the only other tool that exists. Now that this branch
emits **directive** advice (narrow the query / say so plainly), that correctness-by-elimination stopped
being safe: a third tool added later would silently inherit an escalation instruction meant for `recall`.
`RenderToolResult` now gates explicitly: `if r.Tool != ToolRecall { return base }`. A non-`recall` tool
with an empty result gets the bare report sentence, nothing appended. Pinned by
`TestRenderToolResultAddsNoNudgeWhenTheToolIsNotRecall` (`internal/loop/toolresult_test.go`).

## 5. Departure from `retrieval-admission-and-the-empty-outcome.md` §4

That document's §4 (*"Where this design departs from the operator's five points, and why"*) reasoned
about exactly this state — a run that cannot find enough to answer — and concluded:

> "The system text does contain a nudge ... And the model can execute it ... So the behaviour is
> reachable. What is missing is not the nudge. It is that clarification has no terminal reason, so it
> cannot be recorded, counted, or tested."

Its prescription was a `NeedsClarification` terminal reason: a first-class outcome the loop could record,
count, and test, distinct from `StopReason`'s existing values. **That never shipped.**
`grep -rn NeedsClarification --include=*.go` returns nothing in this tree.

This round does not implement it. `nudgeEscalate`'s wording (*"say so plainly and name what is
missing"*) is chosen specifically to **point at the behaviour that document already asked for** — echo
the system text's own instruction rather than compete with it — but it remains a prompt-level nudge
toward a model *choosing* to say "I don't know," not a structural outcome the loop can observe, count, or
gate a test on. If the model says so plainly, that answer is still recorded as an ordinary successful
`StopReason` with no marker distinguishing it from an answer that actually used the graph. That gap is
unchanged by this PR and is not silently closed by wording alone — flagged here so
`retrieval-admission-and-the-empty-outcome.md` is not read as up to date on this point without a pointer
to why it is not.

## 6. The partition is per-function, and the prompt does not inherit its exclusivity (W-1)

The claim carried into PR #98 was that `renderBlock` and `RenderToolResult` partition "before the model
acted" from "after" — true of *reachability* (§7 below), and QA's W-1 found it can read as a claim about
the *prompt*, which is false. `buildMessages` (`internal/openaicompat/wire.go:117`) emits the block
**once**, in the user message, and then one `assistant`/`tool` message pair **per entry in
`in.PriorTools`**. The block is fixed for the whole turn (`Turn.Run` calls `Assemble` once, before the
judge loop starts); each `recall` round gets its own tool message. A turn where the initial block was
thin and the model then issues three empty recalls sends **one** request containing the tier-1 nudge
once (inside the block) and the tier-2 nudge three times (once per tool message) — all co-present.

**This is intended, not a residual bug.** Suppressing the tier-1 nudge once a tool round starts would
require threading turn-level state (has a tool been called yet?) through `Assemble` or `renderBlock`,
which the original task explicitly ruled out (*"`Assemble` and `RenderToolResult` stay pure. No clock,
no I/O, no new state threaded through the turn."*) — and there is no evidence the co-presence is harmful.
By the model's third empty recall, both statements are still true simultaneously: the *initial* context
really was thin (that is a fact about the retrieval pass, not a fact that expires), and *this* recall
really did come back with too little or the wrong shape (a fact about this attempt). A model reading the
full transcript sees an accurate history, not a contradiction.

**What changed to reflect this decision honestly:** the per-function tests (`TestRenderBlockNudges...` in
`internal/loop/assemble_test.go`, `TestRenderToolResult...` in `internal/loop/toolresult_test.go`) certify
exclusivity **within their own function's return value only** — a block never contains the tier-2 nudge,
a tool result never contains a tier-1 nudge — and none of them claim anything about the full prompt. A
new test, `TestJudgeCarriesTheBlockNudgeAndEveryToolResultNudgeInOneRequestWhenRecallKeepsComingUpEmpty`
(`internal/openaicompat/usercontent_test.go`), asserts the co-presence directly: a thin block plus three
empty `recall` exchanges produces one 8-message request carrying the block's nudge once and the tool
nudge three times. It asserts those two counts against **literal** nudge sentences, not against values
re-derived from `Assemble` and `RenderToolResult` — a derived expectation moves with the code and would
pin nothing. That test is the record of this decision — it fails, not passes, if a later change tries to
make the tiers mutually exclusive across a turn without an explicit design change here.

## 7. Where the state comes from (both rounds)

Unchanged from PR #98, restated because CF-2/CF-3/W-1 all depend on it: `renderBlock` is called from
`Assemble`, itself called once in `Turn.Run` before the judge loop starts — reaching it means the model
has not had a turn yet. `RenderToolResult`'s empty branches are reached only from a `ToolExchange` built
by `Turn.dispatchRecall`, itself only invoked once the model has requested `recall` and that call
returned — reaching *this* function means the model acted, and `r.Dispositions` (populated by
`admit` in the same package) already carries `CutReason` per row. Tier 2's branch on `CutReason`
(§2) draws on state that was already in hand at the call site, exactly as free as the reachability split
itself — CF-2's point was that the earlier round argued from that freedom and then declined to use it one
level down.

## 8. Mutation coverage, briefly

Full detail is in the round-2 PR body; the shape worth recording here is that tier 1, tier 2's
byte-budget branch, and tier 2's escalate branch are each pinned by at least one literal-equality test
(catches a wording mutation) and, for tier 2's branch selection, by at least one `HasSuffix` +
`Contains`-exclusion test that is **not** preceded by an exact-equality assertion on the same value
(catches a branch-swap mutation; CF-1 found the round-1 versions of these were dead — reached only when
the preceding equality check had already forced them false). `TestRenderToolResultNarrowsTheQueryWhen
TheCutSetMixesByteBudgetWithAnotherReason` and `TestRenderToolResultEscalatesWhenTheCutSetIsAllBelowFloor`
are the two replacement guards; both were confirmed live by swapping `nudgeNarrowQuery` and
`nudgeEscalate` in the production branch and observing both redden (reverted, `sha256` checked).

Both of those guards also carry the **tier-1-exclusion** assertion that certifies §6's *"a tool result
never contains a tier-1 nudge"*. They sit **before** their test's `HasSuffix` check and there is no
exact-equality assertion on the same value anywhere in either test, so neither can be forced constant the
way CF-1 found the round-1 versions were. Confirmed live by appending `nudgeNone` to both tier-2 returns
and observing the guard line itself — not a later assertion — report the failure in each test.
