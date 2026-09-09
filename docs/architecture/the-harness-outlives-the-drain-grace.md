# Architectural Document: The harness outlives the drain grace

*Standards applied: Design Contracts **#1136** (§1 KISS/DRY/YAGNI, §2 existing systems first, §3 configurability, §4 less is better, §5 Pre-Design Checklist — audited in §12 below). Code Contracts **#114 §0**, in its Processor form **#11034** (P-1, P-2, P-3, P-15, P-16, P-20, P-27, P-29, P-41, P-42, P-51). Comment contract **#10861**.*

*Source task: DiVoid **#13427**. Measured at `main` = `70c71ec`.*

*Revision 2 — 2026-09-09. Corrected against QA review **#13430**, which falsified three load-bearing claims of revision 1 (§8's over-determination reason, §9 F3's stated price, §7.1's supporting citation) and named two implementation defects whose remedies move this document's contracts (§7.1's line shape, §7.2's guard name). Superseded text is retained under dated correction notes rather than rewritten away, so a reader can see what was believed and when — which grew the document by more than half, from 32,370 to 50,840 bytes. **Measured on revision 2, 2026-09-09:** its seven dated correction notes total **7,583 bytes**, which is **41 %** of the +18,470-byte growth; the remaining 59 % is new contract prose — §7.1's recognition/validation split, §7.2's Assertion E, §8's assertion-strength paragraph, F8, §11 step 5's rebuilt matrix, §13's decisions 3 and 4 — most of it load-bearing rather than apparatus. Revision 2 said *"almost entirely quoted-and-measured"*, which is a *because* clause carrying no measurement, in the masthead of a revision written to fix three #11034 P-51 violations (QA #13431 W-3). QA's observation that a document's falsification rate tracks its length still stands, and §10's R3–R5 were cut in this round on exactly that ground.*

*Revision 3 — 2026-09-09. Corrected against QA review **#13431** — approved with warnings, no critical fails, 55 mutation arms and 0 mismatches, with both round-1 critical fails and all four round-1 warnings verified closed. Three edits, one of them a contract change: §7.1's recognition half now **bounds the optional type annotation** to contain no `=` (W-1), and the two prose sites saying a recognised line's *value* fails now say the line fails, because a line can fail §7.1 on its annotation with a perfectly correct value. The masthead's growth claim is replaced by its measurement (W-3), and §7.2's quoted superseded guard name is struck rather than italicised, which empties P-41's residue (W-4). **No assertion, no value, no rejected alternative and no implementation decision moves**, and the recogniser does not change — QA's matrix stands as run, bar the one arm whose message wording this revision fixes (§11 step 5). QA's W-2 — a third restatement of the grace at `README.md:173`, `:183`, `:381` — is **not** pulled into this PR; it is recorded as §10 R6 and filed as #13432.*

---

## TL;DR

**What.** `scripts/compare.py` and `scripts/smoke.py` wait 660 s for a server whose own drain ceiling is 675 s, then `SIGKILL` it and report the kill as the server's failure. Raise the wait past the ceiling, and make a future move of the ceiling redden a test instead of passing silently.

**How.** Each script gains a **named mirror** of the server's constant — `SHUTDOWN_GRACE_S = 675` — and derives its wait from it: `DRAIN_GRACE_S = SHUTDOWN_GRACE_S + 30` (705 s). A new guard in `internal/server`'s own test package reads `scripts/*.py` **as text** and fails if any mirror disagrees with `shutdownGrace`, or if any `DRAIN_GRACE_S` stops being that mirror plus a positive margin. The guard lives in `package server` because that is the only place `shutdownGrace` is visible, and — more to the point — it is the package a grace change is edited in, so the person who moves the constant is the person whose gate goes red.

**The cost, where non-zero.** Three: (1) the mirror is still a restated number — the guard makes it *loud*, it does not make it *derived*, because there is no import across Go→Python; (2) a Go test now depends on the repo layout `../../scripts` and on a stated line-shape in two Python files; (3) this repo has **no CI**, so the red only appears to whoever runs `go test ./...` — which under #11034 §1 is the implementer and QA on every round, but is a human gate, not a machine one.

**The strongest rejected alternative.** Make the binary *publish* its grace — an exported `ShutdownGrace`, surfaced through a `--print` flag or a `/health` field, read by the harness at runtime. It removes the restatement entirely, which is the structurally correct answer. Rejected because a runtime probe needs a failure path, and the only available failure path is a literal fallback — which re-creates the restatement with a quieter failure mode — while adding a permanent operator-visible surface whose only consumer is a test rig (#1136 §2 Form 3). Five new surfaces against a 15-second arithmetic error does not pass §4's can-it-be-deleted check.

**Not fixed here, deliberately:** the `compare.py`/`smoke.py` protocol duplication stays at **#11326**; `RUN_TIMEOUT_S` stays as it is. §2 says why.

---

## 1. Problem Statement

`internal/server` honours a drain grace of 675 s. Both operator harnesses that stop the built binary wait 660 s and then kill it. There is therefore a 15-second window in which a server that is behaving exactly as designed — finishing a write-back inside the margin PR #53 added for precisely that purpose — is `SIGKILL`ed by the instrument that exists to demonstrate the drain, and reported as *"killed without draining — a run record may be at a node with no body"*, which reads to the operator as a defect in the server.

Two properties must hold when this is done:

1. **The harness never terminates a server still inside the drain grace the server itself is honouring.**
2. **A future change to the server's grace cannot leave the harness behind silently.**

The second is the load-bearing one. PR #53 removed this exact class from `internal/server` — it deleted `writeBackCalls = 3` and measured the count by running the real adapter over a counting transport. Two copies of the same restated constant survived in `scripts/`. Correcting `11 * 60` to `11 * 60 + 15` reproduces the defect and defers it to the next time the grace moves.

**Success criteria.** (a) `DRAIN_GRACE_S > shutdownGrace` with stated headroom. (b) Editing `shutdownGrace` without editing the scripts produces a failing test, with a message naming both values and the file. (c) No new operator-facing surface, no new binary, no new dependency, no configuration knob.

---

## 2. Scope & Non-Scope

### In scope

| | |
|---|---|
| `scripts/compare.py` | add `SHUTDOWN_GRACE_S`; re-derive `DRAIN_GRACE_S`; correct the operator line's stated wait |
| `scripts/smoke.py` | the same three changes |
| `internal/server` (test only) | one new guard test file |

`scripts/step_trace.py` needs **no edit**: it imports `compare` and calls its `stop_server`, so it consumes `compare.DRAIN_GRACE_S` and inherits the fix (`step_trace.py:189`). There are **two definition sites and three consumers**.

This is **one PR**. The three changes are one unit: the bump alone is the rejected naive fix, the guard alone fixes nothing.

### Explicitly out of scope

**The harness protocol duplication (#11326).** `compare.py` and `smoke.py` are diverging copies of thirteen lifecycle functions; extraction to `scripts/_processor_harness.py` is open at #11326 and now has **three** call sites, one by import, with an untested seam on both sides (#11340). Bundling it here tangles a correctness fix with a refactor of a second and third file, against the one-feature-one-PR rule.

**#11034 P-2 does not force the extraction as part of *this* change.** P-2 extracts a block **over 5 lines** recurring at **more than 2 sites**; two sites is not more than two. The duplication this design adds is `block_size × site_count = 2 × 2 = 4` lines, an order of magnitude below the ~15–20 threshold. The design chooses the guard *because* the duplication is small, not in spite of it.

The guard is written so the extraction does not have to touch it: it checks *every* `scripts/*.py` that defines either constant, so a single definition in `_processor_harness.py` satisfies it unchanged. #11326's listing of this constant should be updated to say the margin is now **negative**, not zero — that correction belongs on the node, not in this PR.

**`RUN_TIMEOUT_S`.** It reads `11 * 60 + 30` = 690 s in both files. It is **not** a copy of the grace and must not be coupled to it: it is the client-side wait for the `POST /runs` response, and the ceiling it must clear is the **handler's** deadline — `runBound`, 10 min, `routes.go:77` — not the drain. 690 s over a 600 s handler ceiling is 90 s of slack and is correct. It is left untouched, and this paragraph exists so the next reader does not "helpfully" re-derive it from `SHUTDOWN_GRACE_S`. It is worth a line on #11326 or #11340; it is not worth a code change.

**`IDLE_GRACE_S = 15`.** Unrelated — the no-run-in-flight path. Untouched.

**The pre-existing body comment at `internal/server/server.go:59-65`.** Out of this diff. #10861 forbids **adding** one; it does not make deleting an existing one part of this change.

**CI.** This repo has no workflow files. Adding one is a real improvement and a separate decision; see §10.

---

## 3. Assumptions & Constraints

| | |
|---|---|
| **Go cannot be imported by Python and Python cannot be imported by Go.** | This is the whole problem. Nothing bridges the boundary structurally; every option below is a choice of *which restatement, checked where*. |
| **`go test ./...` must not acquire a Python interpreter dependency.** | The gate runs on a Windows host and in a `golang:1.27` container (#11034 §1). The container image has no Python. Any guard that *executes* the scripts is therefore excluded. The guard reads them as text. |
| **`shutdownGrace` is unexported.** | A guard that reads it must be in `package server`. This is forced, not chosen. |
| **The module has zero third-party dependencies** and defending that is a stated property (#10466). | The guard uses the standard library only. |
| **`shutdownGrace` is a whole number of seconds** (675). | Assumed, and the guard asserts it — see §9 falsifier F5. |
| **The scripts are hand-run operator instruments**, never imported by Go and never run in `go test` (#11340). | So the guard cannot observe their behaviour, only their source. |
| **No CI.** | Assumed from the absence of workflow files at `70c71ec`. Verified by inspection; if a pipeline exists elsewhere, §10 R3 softens. |

---

## 4. The measurement at `70c71ec`

| name | site | value |
|---|---|---|
| `runBound` | `internal/server/server.go:13` | 600 s |
| write-back ceiling | measured by `writeBackGraphCallCeiling()` | 4 calls |
| `divoid.DefaultTimeout` | `internal/divoid/client.go:24` | 15 s |
| `graceMargin` | `server_test.go` | = `divoid.DefaultTimeout` = 15 s |
| **`shutdownGrace`** | `internal/server/server.go:14` | **675 s** = 600 + 4×15 + 15 |
| `DRAIN_GRACE_S` | `compare.py:79`, `smoke.py:52` | **660 s** |
| `RUN_TIMEOUT_S` | `compare.py:76`, `smoke.py:50` | 690 s |
| `IDLE_GRACE_S` | `compare.py:78`, `smoke.py:51` | 15 s |

`stop_server` (`smoke.py:148-171`, same shape at `compare.py:437-459`) waits `proc.wait(timeout=grace)` and, on `TimeoutExpired`, calls `proc.kill()`.

**Two clocks, and they are not the same clock.** The server's 675 s starts when `serve` observes `ctx.Done()` and enters `context.WithTimeout(..., shutdownGrace)`. The harness's wait starts one line after `send_signal`/`terminate`, and ends when the **process exits** — which is later than the shutdown context's expiry, because `srv.Shutdown` must return, `main` must log `shutdown complete`, and the process must unwind. The harness window therefore has to be a strict superset of the server's, which is why equality (the state before PR #53) was already wrong and only looked safe.

---

## 5. Architectural Overview

```
  internal/server/server.go
    shutdownGrace = 675s ──────────────┐  owns the value
                                       │
  internal/server/<guard>_test.go      │  reads the value directly (same package)
    ├── reads ../../scripts/*.py as text
    ├── asserts   SHUTDOWN_GRACE_S == shutdownGrace          (exact equality)
    ├── asserts   DRAIN_GRACE_S is derived from that mirror  (textual reference)
    └── asserts   at least one file defined each             (no vacuous pass)
                                       │
  scripts/compare.py   scripts/smoke.py│
    SHUTDOWN_GRACE_S = 675  ◄──────────┘  mirror, guarded
    DRAIN_GRACE_S = SHUTDOWN_GRACE_S + 30 = 705s
        │
        └── scripts/step_trace.py  (imports compare; no constant of its own)
```

Three ideas carry the design:

1. **Split the mirrored number from the harness's own margin.** Today one constant carries both jobs, so a guard could only assert an inequality and could not say *which* half was wrong. Split, the guard asserts an **equality** on the half that belongs to `internal/server`, and the harness keeps full freedom over the half that belongs to it. The failure message becomes exact: *"smoke.py declares SHUTDOWN_GRACE_S = 675, but shutdownGrace is 720."*

2. **Put the detector where the change is made.** Drift is caused by editing Go. A Python-side check would be run by whoever runs the rigs — which is not the person who moved the constant, and may be weeks later. A Go-side check is run by the gate that every change to `internal/server` already passes through. **This is the single most important structural choice in the document**, and it is why the guard is in Go despite the thing it inspects being Python.

   > **Correction 2026-09-09 (QA #13430 W-2).** This objection discriminates against alternative **D** — a Python-side *test* — and does **not** reach alternative **C**, a Python-side *derivation*. C has nothing to detect and no place to put a detector: it recomputes the value at import and cannot be stale. §8's C row is corrected accordingly and C is rejected on its other grounds.

3. **Read the text; do not run it.** The guard must work in a container with no Python. Reading source text is weaker than measuring behaviour — and that weakness is stated, priced and falsified in §9 rather than hidden.

**This is the same remedy shape PR #53 chose, degraded by exactly one step because the boundary allows nothing better.** PR #53 replaced a restated constant with a **measurement** (drive the real adapter, count the calls). Here the sequence is not drivable from Go, so the fallback is #10466's second-best: a value stated in one place and **checked** against the owner, rather than copied and hoped over. The existing worked precedent for a same-package test inspecting another directory's source is `internal/condense/isolation_test.go`, which parses `../loop` and carries its own no-vacuous-pass clause.

---

## 6. Components & Responsibilities

| Component | Owns | Does not own |
|---|---|---|
| `internal/server` (production) | the drain grace and its derivation from `runBound` + measured write-back ceiling + margin | anything about the harness |
| `internal/server` (test) — the new guard | detecting that a script's mirror has fallen out of step with the grace, and that a script still derives its wait from its mirror | the harness's margin, the harness's behaviour, whether the scripts run at all |
| `scripts/compare.py`, `scripts/smoke.py` | the mirror of the grace; their own margin over it; the operator-facing wording of the wait | the grace itself |
| `scripts/step_trace.py` | nothing here — consumer only | — |

Single-responsibility framing for the split: **`SHUTDOWN_GRACE_S` is a fact about `internal/server`, checked by `internal/server`. `DRAIN_GRACE_S` is a decision by the harness, owned by the harness.** One line, one owner each.

---

## 7. Contracts

### 7.1 The Python line-shape contract

The guard reads text, so the shape is a contract. It has two halves, and keeping them apart is what makes the guard fail closed.

**Recognition — by identifier, and at least as wide as Python permits.** A line is a *declaration* of one of these constants if, at column 0, it opens with the identifier, followed by optional whitespace, an optional type annotation **containing no `=`**, and `=`. All of `NAME = 675`, `NAME=675`, `NAME : int = 675` are recognised; `NAME_SOMETHING = 675` is not. **The annotation's bound is the one place this contract stops short of what Python permits, and it is stated rather than left implicit:** an annotation carrying its own `=` — `NAME: Annotated[int, Meta(u='s')] = 675` — is outside the contract, because the line's first `=` is then not the assignment, and no reader working at text level can find the value without evaluating Python. Such a line is still **recognised** and still reddens, under Assertion E; the bound costs no blindness, only one legal spelling that nothing here writes (measured 2026-09-09: across all seven `scripts/*.py` there are zero column-0 annotated assignments, and no `typing` or `Annotated` import anywhere under `scripts/`). **Recognition never narrows to the spelling this tree happens to use.** A shape the guard fails to recognise is a shape whose staleness it can never report; with two definition sites, one file drifting out of recognition leaves the other satisfying the no-vacuous-pass clause while that file's mirror sits stale and unchecked forever. This is the defect QA measured as CF-1 (#13430), and #11034 P-31 is the rule it broke: *an instrument's route enumeration is argued from what the language permits, never from what the tree writes today.*

**Validation — by value, and narrow.** Once a line is recognised, the assigned text up to any trailing comment must satisfy:

| | Requirement |
|---|---|
| **Placement** | Module level, name at column 0, one assignment per line, in the existing timeout-constant block. |
| **`SHUTDOWN_GRACE_S`** | A **bare decimal integer of seconds** — digits only. No arithmetic, no expression, no sign, no digit separator. It exists to be compared by a reader that cannot evaluate Python. |
| **`DRAIN_GRACE_S`** | Exactly `SHUTDOWN_GRACE_S`, then `+`, then a **bare decimal integer of at least 1**. Nothing else: not a bare literal, not `-`, not `*`, not the operands reversed, not a parenthesised or wrapped expression. |
| **Trailing comment** | Permitted on either line; the guard reads the value up to the comment. |

**A recognised line that fails validation is a failure naming this section — never a skip, and never a claim about what the line means.** The distinction is not pedantry: revision 1's guard, meeting a wrapped `DRAIN_GRACE_S = (`, reported *"does not derive from SHUTDOWN_GRACE_S"* about a line that derives perfectly well. A check that fires on compliant code with a false reason is #1220 §9's worst class, because it teaches the reader to route around the instrument.

`SHUTDOWN_GRACE_S` carries a one-line trailing comment naming `internal/server/server.go` and the identifier it mirrors. **This is deliberate and is not a #10861 violation**: #10861 rules on Go, and these scripts already carry explanatory comments of exactly this kind — `step_trace.py:195-201`, a block opening *"Mirrors internal/loop/turn.go's unexported string constants — duplicated here because…"*, which is the same construction for the same reason. No DiVoid node id, no PR number, no finding coordinate — the comment names a file and an identifier only.

> **Correction 2026-09-09 (QA #13430 CF-1 and W-3).** Two things were wrong here.
>
> *The superseded contract* had no recognition half at all, and its `DRAIN_GRACE_S` row required only "an expression whose text **names `SHUTDOWN_GRACE_S`**". It constrained **placement and value and never whitespace or annotation**, so `SHUTDOWN_GRACE_S=660` and `SHUTDOWN_GRACE_S: int = 660` were compliant by this section's own words while invisible to a guard matching the literal prefix `"SHUTDOWN_GRACE_S = "`; QA measured both passing with a stale mirror. Narrowing §7.1 to forbid those spellings would not have fixed it — a contract the instrument cannot observe is not enforcement — so the recognition half is widened and the narrowness moved to the value, where the guard can act on it. See §13 decision 3.
>
> *The superseded citation read:* "these scripts already carry explanatory comments of exactly this kind (`step_trace.py:189`)". `step_trace.py:189` is `import compare as harness  # noqa: E402 …`, an import-suppression note, not a mirrored-constant comment. The claim was true and its evidence sat six lines further down; §2 cites `:189` correctly for a different claim, and this section had copied the coordinate across. Both coordinates re-read in the tree before correcting.

> **Correction 2026-09-09 (QA #13431 W-1).** *The recognition half previously permitted* "an optional type annotation" *with no bound.* Under that wording `SHUTDOWN_GRACE_S: Annotated[int, Meta(u='s')] = 675` was §7.1-compliant by this section's own words — module level, column 0, one assignment per line, an optional annotation, and a bare-integer value of 675 — while the guard, reading up to the line's first `=`, captured the annotation's residue and reported a **value** failure that had not occurred. It is the only input in a 55-arm matrix whose message was false of its input. The remedy is the bound above and not a wider recogniser: recognition already reaches this shape and already reddens on it, so what was missing was the contract saying the shape is out, not the instrument being able to see it. Two prose sites in this document and both failure-message sites in the guard said "whose value"; all four are corrected — #11034 P-52, since the finding named the two code sites and the claim lived at four.

### 7.2 The guard's contract

| | |
|---|---|
| **Input** | Every `*.py` in the repository's `scripts/` directory, read **non-recursively**, as UTF-8 text. Never executed, never imported. |
| **Recognition** | A line is a declaration of either constant per §7.1's recognition half — **by identifier**, not by a fixed literal line prefix. |
| **Assertion A — equality** | For each file declaring `SHUTDOWN_GRACE_S`: its integer value, taken as seconds, equals `shutdownGrace`. Exact equality, not `>=` — a grace that moves **down** must redden too, so the harness's margin is not silently inflated into an unstated one. |
| **Assertion B — derivation and sign** | For each file declaring `DRAIN_GRACE_S`: the assigned text is `SHUTDOWN_GRACE_S`, then `+`, then a bare decimal integer of at least 1. This stops both a "simplification" back to a bare literal — the mirror then sits correct and unread — and a subtraction or a zeroing, which puts the wait back inside the grace and is the defect #13427 opened. |
| **Assertion C — no vacuous pass** | If the sweep found no `SHUTDOWN_GRACE_S` at all, or no `DRAIN_GRACE_S` at all, the guard **fails**. A guard that finds nothing and reports success is the instrument-failure class this repo has recorded three times (#10943). |
| **Assertion D — whole seconds** | If `shutdownGrace` is not a whole number of seconds, the guard fails, naming the value. The mirror's contract is integer seconds; a sub-second grace is a decision, not a rounding. |
| **Assertion E — line shape** | A **recognised** line that does not satisfy §7.1 fails the guard, naming §7.1 and quoting the line. It is never skipped, and the message never asserts what the line means — including that the fault lies in the line's *value*, since a line can fail §7.1 on its annotation while its value is correct. |
| **Failure message** | Names the file, the declared value, and `shutdownGrace`, in that order, so the fix is readable without opening anything. |
| **Not asserted** | That a harness script declares these constants **at all** — Assertion C requires only that *some* file declares each, so a rig computing its wait without naming either is invisible to the guard (F4). Nor the **sufficiency** of the margin: that it is 30 s and not 5 s is a judgement (§7.3), not a checked property (F1, F2). |

> **Correction 2026-09-09 (QA #13430 W-1).** *Assertion B previously read:* "For each file declaring `DRAIN_GRACE_S`: the assigned expression's text names `SHUTDOWN_GRACE_S`. This is what stops a 'simplification' back to a bare literal from passing while the mirror sits correct and unread." *And the* **Not asserted** *row previously read:* "That `DRAIN_GRACE_S` **exceeds** the grace. The guard checks the *reference*, not the sign or the magnitude of the addend. See F3."
>
> QA measured what that bought: `DRAIN_GRACE_S = SHUTDOWN_GRACE_S - 660` (a 15-second wait) and `SHUTDOWN_GRACE_S * 0` (no wait at all) both passed. The price §9 F3 quoted for closing it was wrong — see the correction there — and with the sign checked, the wait now provably exceeds the grace for every file that declares it, which these two rows explicitly disclaimed. Assertion E is new, and carries the verdict for a recognised line the validation cannot read.

**Guard test name** (this name is a contract — #11034 P-41 checks that a design's cited guard resolves to a real `func Test…`; P-20 forbids a name claiming more than the assertion carries):

> `TestEveryDeclaredHarnessGraceEqualsShutdownGraceAndEveryDeclaredWaitIsThatMirrorPlusAPositiveMargin`

Both universals in the name are scoped to **declared**, because that is the scope Assertions A and B range over. Applying P-20's falsifier to the whole sentence: a stale mirror reddens (A); a bare-literal, negated or zeroed wait reddens (B); a harness script declaring neither constant falls outside both quantifiers — honestly, and its invisibility is stated as the limitation F4 rather than denied by the name. The name does not claim Assertion C or D, which are instrument-integrity properties rather than claims about the tree; their own messages say what they are.

> **Correction 2026-09-09 (QA #13430 CF-2).** *Superseded name:* ~~`TestEveryHarnessScriptMirrorsTheDrainGraceAndDerivesItsOwnWaitFromThatMirror`~~, glossed here as *"It claims the mirror and the derivation. It does not claim the wait outlives the grace, because it does not check that."* The gloss audited one overclaim and missed the quantifier one word into the sentence: **every** harness script does not mirror the grace. `scripts/step_trace.py` is a harness script, it calls `stop_server` at `step_trace.py:1028`, it declares neither constant, and the guard was green — so the name was false at `70c71ec`, in this repository, against this tree. P-20's falsifier applies to the whole sentence, not to the clause the author happened to be thinking about. The superseded name appears in this document only inside this note, as a quotation; the name §11 step 4 instructs John to create is the one above.

**Placement.** A new file in `package server`, sibling to `server_test.go`. Not folded into `server_test.go`: that file is the drain's own derivation (`runBound` + measured ceiling + margin), and this is a different claim about a different consumer. `internal/condense/isolation_test.go` is the precedent for the file shape.

### 7.3 Values after the change

| | before | after |
|---|---|---|
| `shutdownGrace` | 675 s | 675 s — **unchanged** |
| `SHUTDOWN_GRACE_S` | — | **675** |
| `DRAIN_GRACE_S` | 660 s | **`SHUTDOWN_GRACE_S + 30` = 705 s** |
| harness headroom | **−15 s** | **+30 s** |

**Why 30, stated rather than derived** (#11034 P-51 — a *because* clause carries a measurement or a hedge in the same sentence). The margin covers signal delivery to the child plus the process's own exit after `srv.Shutdown` returns; **neither has been measured on this repo**, so 30 s is a slack figure, not a derivation. It is the figure this repo already chose for the same kind of harness-clock slack in `RUN_TIMEOUT_S` (`11 * 60 + 30`), which makes it consistent rather than correct. The cost of it being too large is 30 s of extra waiting on a hang that was going to be a kill anyway; the cost of it being too small is the defect this document exists to remove. The asymmetry is why it is not tuned downward.

### 7.4 The operator line

`stop_server` prints *"waiting up to `{grace // 60}` minutes"*. At 705 s that renders **"11 minutes"** while waiting 11 m 45 s — the same class of false operator-facing claim #13427 names in the current text. Change the rendering to state minutes **and** seconds, so the printed number is the number waited. One expression, two sites, no new constant.

---

## 8. Alternatives considered and rejected

| # | Alternative | Why rejected |
|---|---|---|
| **A** | **Bump the literal only** (`11 * 60` → `11 * 60 + 15`, or a bare 705). | Satisfies property 1, fails property 2 outright. This is the shape that cost PR #53 its follow-up. It is the baseline the whole design is measured against, not a candidate. |
| **B** | **Publish the grace from the binary** — export `ShutdownGrace`, surface it via a `--print-lifecycle` flag or a `/health` field, have the harness read it at startup. | The structurally strongest option: no restatement anywhere. Rejected on three concrete costs. **(i)** A runtime probe needs a failure path, and the only one available is a literal fallback — which restores the restatement with a *quieter* failure mode than today's. Hard-failing instead makes all three rigs unstartable on any regression in a new binary surface. **(ii)** A `/health` field makes an internal timing constant a permanent wire contract, and archetype A (#10466) requires it be pinned by literal in the route tests forever after. **(iii)** A flag whose only consumer is a test rig is #1136 §2 Form 3, pure restatement, and #1136 §3's bar for a new surface. Five new surfaces (export + flag-or-field + its test + the harness probe + its fallback) against a 15-second arithmetic error fails §4's can-it-be-deleted check. |
| **C** | **Parse `server.go` from Python** at import time and evaluate the Go duration expression. | Zero restatement, no Go surface change — genuinely attractive, and **it would have prevented this defect outright**: a one-line sum over the `<int> * time.<Unit>` terms derives 660 from `11 * time.Minute` at `28d2998` and 675 from `11*time.Minute + 15*time.Second` at `70c71ec`, so PR #53's reformatting carries straight through (run against both revisions, 2026-09-09). Rejected on three grounds that survive that. **(i)** It couples to Go *formatting*, and this constant is one edit from a shape those terms are absent from: §4 shows `shutdownGrace` is already conceptually `runBound + ceiling + margin`, so `shutdownGrace = base + graceMargin` is the natural next refactor and matches nothing. **(ii)** That failure has nowhere good to go — raising at import makes all three rigs unstartable on a Go refactor, and falling back to a literal restores the restatement with a *quieter* failure mode. This is alternative B's objection (i) reached by a different road; it is reasoning, not a measurement. **(iii)** A Go-duration evaluator in Python is more code than the guard and carries its own failure modes. |
| **D** | **Guard on the Python side** — a case in `scripts/test_compare.py` asserting `DRAIN_GRACE_S > shutdownGrace` read out of `server.go`. | Right assertion, wrong place, and **never green**. `python -m unittest discover -s scripts` is run by nobody editing `internal/server`, and the README's gate section names `go test`, `gofmt`, `go vet` and the container only — so there is no Python gate for it to be part of. Its accurate description is **red and unrun**: red at `28d2998` (660 > 660 is false) and red at `70c71ec` (660 > 675 is false), with nothing running it either time. The distinction matters — a red-and-unrun test is found by the first person to run the Python suite, whereas the state we actually had was found only by a manual repo-map reconcile. |
| **E** | **Have the guard execute the scripts** and read the computed `DRAIN_GRACE_S` — the true measurement, matching PR #53's remedy exactly. | Requires a Python interpreter inside `go test`, including the `golang:1.27` container gate (#11034 P-9). The module has zero dependencies by policy and this would add an unversioned host one. If Python ever becomes a stated build prerequisite, this is the upgrade path and the guard's contract (§7.2) is the seam it swaps in behind. |
| **F** | **Do the #11326 extraction now** and fix the constant once in `_processor_harness.py`. | Removes one of the two copies but not the cross-language restatement — the extracted module would still hold a literal 675 with nothing coupling it to Go, so the guard is needed either way. Meanwhile it drags a three-call-site refactor with an untested import seam into a correctness fix, against one-feature-one-PR. The guard is written to survive the extraction untouched, which is the cheap way to keep both options open. |

**Trade-off, stated plainly.** B is the better architecture and the worse change. The design takes the worse architecture because the defect is 15 seconds of arithmetic in two constants, and the difference between B and the chosen shape is not correctness but *how loudly* the next drift announces itself — B makes it impossible, the guard makes it a failing test. Against that, B costs a permanent public surface and a fallback that quietly re-creates the problem. If the grace ever becomes something operators tune, B's cost is already paid and it should be revisited; today nothing tunes it.

> **Correction 2026-09-09 (QA #13430 W-2).** Rows C and D previously rested on a shared claim that both alternatives *"would each have been green through PR #53"* — D's row said so in those words; C's closed on *"the failure lands on the operator running the rig… detection in the wrong place, which is §5 idea 2 inverted."* Both halves are false. QA reconstructed PR #53's aftermath (`28d2998` → `70c71ec`, a merge touching only `README.md`, `internal/server/server.go`, `internal/server/server_test.go` — no Python) and ran both alternatives against both revisions:
>
> | revision | `shutdownGrace` (Go text) | alt-C derives | `DRAIN_GRACE_S` | margin | alt-D (asserts `DRAIN > grace`) |
> |---|---|---|---|---|---|
> | `28d2998` pre-#53 | `11 * time.Minute` | 660 | 660 | 0 s | **RED** |
> | `70c71ec` post-#53 | `11*time.Minute + 15*time.Second` | 675 | 660 | −15 s | **RED** |
>
> D was **red at both revisions**, including the zero-margin state that preceded PR #53 — not green, and never green. C has no assertion to be green or red at all: it **derives**, so scoring it as "green through PR #53" is a category error, and §5 idea 2's wrong-place objection does not reach it. I re-ran the derivation over both revisions independently before accepting the finding. **The decision is unchanged** — C's grounds (i)–(iii) carry it on their own — but a false reason was offered under a right decision, which is #11034 P-51's named failure mode precisely because such a reason survives review indefinitely.

**The comparison was on placement, and placement was not the only axis.** These alternatives were weighed by *where the failure lands* and never by *what the assertion says*. On the second axis alternative D's assertion — `DRAIN_GRACE_S > shutdownGrace` — was strictly **stronger** than what revision 1 shipped, which checked the reference and not the sign: D reddens on a negated or zeroed margin, revision 1's guard stayed green on both. The right place had been chosen and the weaker assertion put in it, and nothing in the document noticed, because no row compared the candidates on that axis. Assertion B's strengthening in §7.2 is what makes the chosen placement dominate on **both** axes rather than only on the one that was examined; without it, the honest statement would have been that this design traded assertion strength for placement without saying so.

---

## 9. Falsifiers

The design asserts three universals. Each is falsifiable, and here is what falsifies it.

**Universal 1 — *the harness never kills a server still inside its drain grace.***

- **F1.** The server's post-drain exit takes longer than 30 s — a blocked log flush, a goroutine that does not unwind after `Shutdown` returns. `proc.wait` expires and kills. The kill is then *outside* the grace and correct, but `stop_server`'s message still blames the drain.
- **F2.** Signal delivery to the child takes longer than 30 s under extreme load. Same outcome.
- **F3. Closed 2026-09-09.** Someone writes `DRAIN_GRACE_S = SHUTDOWN_GRACE_S - 30`, or `- 660`, or `* 0`. Assertion B now requires the operator to be `+` and the addend to be a bare integer of at least 1, so all three redden naming §7.1, and the wait provably exceeds the grace for every file that declares it.

  > **Correction 2026-09-09 (QA #13430 W-1).** *Superseded text:* "Someone writes `DRAIN_GRACE_S = SHUTDOWN_GRACE_S - 30`. The guard checks that the mirror is *referenced*, not the sign of the addend, and stays green. **This is a stated hole**, accepted because the drift class being defended against is a stale copy, not a deliberate inversion; closing it would mean the guard parsing Python arithmetic, which is alternative C's cost in the guard."
  >
  > The price was wrong, and it was the entire reason the hole was left open — a *because* clause carrying neither a measurement nor a hedge, which is #11034 P-51 exactly. §7.1 had **already** constrained `SHUTDOWN_GRACE_S` to a bare decimal integer; extending the identical narrowness one line down costs the same `TrimPrefix`/`Atoi` pair the guard was already making — not a Go-duration evaluator, and nothing of alternative C's cost. Nor was the accepted risk as small as "a deliberate inversion" implies: QA measured `DRAIN_GRACE_S = SHUTDOWN_GRACE_S - 15` reinstating #13427's exact defect under a fully green suite. Disclosure is a mitigation of the process, not of the code.
- **F4.** A rig computes its wait without naming either constant. Assertion C catches the *disappearance* of both names from `scripts/` entirely; it does not catch a third rig that never used them.

**Universal 2 — *a change to the grace cannot leave the harness behind silently.***

- **F5.** `shutdownGrace` becomes a non-integer number of seconds. Assertion D reddens — the guard working, but on a legitimate change; the fix is a deliberate decision about how the mirror states sub-second values, not a bump.
- **F6.** **Nobody runs `go test ./...`.** There is no CI in this repo (§3). The guard converts silent drift into a red test *for whoever runs the gate* — under #11034 §1 that is the implementer and QA on every round, which is a human gate, not a machine one. **This is the residual, and it is the honest limit of the whole design.** It is nonetheless strictly stronger than the present state, where the drift is invisible to every party at every stage.
- **F7.** The `scripts/` directory moves or is renamed. Assertion C fails (nothing found), which is the correct outcome — a loud stop rather than a silent pass.

**Universal 3 — *a line in `scripts/*.py` with the shape of a declaration is one.***

- **F8.** A column-0 `SHUTDOWN_GRACE_S = 999` inside a triple-quoted string is read as a declaration and reddens the guard. This is not hypothetical terrain: `scripts/step_trace.py` opens with a module docstring running from line 2 to line 179. It is inherent to §5 idea 3 — the guard reads text and does not run it, and text alone cannot tell a string from a statement. **Left open deliberately, on the direction of the failure.** F8 fails *closed*: it stops the gate with a message naming a file and a line, and is cleared by rewording one line of prose. The failure §7.1's correction note records went the other way — a stale mirror passing in silence — and that asymmetry is the whole reason one is disclosed and the other was fixed. Closing F8 needs a triple-quote state machine over both quote styles, with escape and raw-string handling: a partial Python lexer, a different order of thing from the `TrimPrefix`/`Atoi` pair. I have not built one and am quoting no figure for it.

---

## 10. Risks and mitigations

| # | Risk | Mitigation |
|---|---|---|
| **R1** | The guard passes vacuously — regex misses, file not found, directory moved. This is the recorded instrument-failure class on this repo (#10943). | Assertion C, modelled on `isolation_test.go`'s own vacuous-pass clause. John must observe it fire (§11 step 5). |
| **R2** | The guard is brittle to Python spelling — either reddening on a compliant edit (#1220 §9's *fires on compliant code*) or, worse, failing to recognise a legal respelling and going silently blind on that file. | §7.1's recognition/validation split. Recognition is argued from **what Python permits** for an assignment to that identifier; validation from the stated value shape, with a verdict (Assertion E) for anything recognised that it cannot read. #11034 P-29 requires a **dual**, and P-31 says where its members come from: from the language, never from what these two files write today. §11 step 5 hands John an enumeration written to that rule. |
| **R3** | No CI, so the red depends on a human running the gate. | §9 F6, where it is stated as the design's residual rather than mitigated. Out of scope here; the guard is the thing that would make a pipeline worth having, so the order is right. |
| **R4** | The 30 s margin is a slack figure, not a measurement. | §7.3. |
| **R5** | The #11326 extraction later moves the constants and someone deletes the guard as "no longer needed". | §2 — the guard's contract is the whole `scripts/` directory rather than two named files, so #11326's flat extraction needs no edit to it, and the reason is recorded where the extraction is discussed. The sweep is non-recursive (§7.2), so a move into a subdirectory *would* need one. |
| **R6** | The grace is restated a third time in `README.md` — `:183`'s `docker stop -t 675`, and `:173`/`:381` in prose — which this guard does not reach and this PR does not touch. The next move of `shutdownGrace` therefore leaves README instructing the operator to `SIGKILL` a still-draining container: #13427's defect, one artifact over. `README.md:173` says a test pins that literal, and one does — it pins the **Go constant** in `internal/server`, not README's restatement of it, so nothing reddens when the prose or the `-t` argument goes stale. | **Not mitigated here, and deliberately.** #13427 scoped itself to `scripts/` and §2 holds that line; the README mirror is prose and a shell argument, not a Python assignment, so whether the guard should reach it is a design question rather than this PR's bug fix. Filed as **#13432**. Recorded because of *how* it survived: the task, this design, the implementation and QA round 1 all swept **by phrase, inside `scripts/`** — #11034 P-52's third sharpening exactly, *"whether what your change makes false is asserted anywhere in the language you were not editing in."* |

> **Correction 2026-09-09 (QA #13430 CF-1).** *R2's superseded mitigation read:* "The line-shape contract (§7.1) is narrow and stated; the constants sit in a block that has been stable across PRs #21, #28, #36, #43. #11034 P-29 requires a **dual**: John enumerates what *legal* Python in that position looks like — trailing comment, extra blank lines, reordered constants, a third script defining neither — and confirms each stays green." That enumeration **is** the defect. Its four members are the four spellings these two files already contain, so it was derived from what the tree writes and looked complete because every construction it could see, it read — P-31 verbatim. All four passed; the shapes outside it (`NAME=660`, `NAME: int = 660`) were never asked, and are where the guard was blind. A dual is only as strong as the rule its members were generated by.
>
> **Correction 2026-09-09.** R3, R4 and R5 previously restated §9 F6, §7.3 and §2 at paragraph length; they are cut to pointers. QA's closing observation on #13430 is the reason to make the cut rather than defend the length: this document's falsification rate is a function of that length, and a claim restated in two places goes stale in two places independently of each other. R1 and R2 stay in full because each says something said nowhere else — and R2's dual requirement is what CF-1 turned on.

---

## 11. Implementation guidance

One branch, one PR. In order:

1. **`scripts/smoke.py`** — insert `SHUTDOWN_GRACE_S` in the timeout-constant block (currently lines 49–52) with its one-line trailing comment naming `internal/server/server.go` and `shutdownGrace`; re-express `DRAIN_GRACE_S` as the mirror plus 30. Per §7.1: column 0, one assignment per line, bare integer on the mirror.
2. **`scripts/compare.py`** — the identical change at lines 75–79.
3. **Both files** — `stop_server`'s printed wait states minutes and seconds, so the printed number is the number waited (§7.4). The two files' messages differ today in punctuation only (`—` vs `--`); **keep them as they are.** Converging their wording is #11326's business and would put a third file's worth of diff noise in this PR.
4. **The guard** — a new file in `package server`, holding `TestEveryDeclaredHarnessGraceEqualsShutdownGraceAndEveryDeclaredWaitIsThatMirrorPlusAPositiveMargin`, implementing §7.2's five assertions. **Recognise a declaration by identifier, never by a fixed literal line prefix** — §7.1's recognition half is the contract and states why. Standard library only; text read, never executed; `../../scripts` resolved relative to the package directory as `isolation_test.go` resolves `../loop`. **No comments** — #10861 as ruled for Go, and the test name is the sole carrier of intent (#11034 P-20). No node ids, PR numbers or finding coordinates anywhere in the file (P-38).
5. **The mutation round** — #10466's gate: *a guard you have not seen fail is decoration*. The recognition change moves the parse for **both** constants, so the matrix is re-run **whole**, not only on the arms that changed. Each arm observed:
   - **Sensitivity:** `shutdownGrace` to 12 min → red, message naming both files; to 10 min → red (equality, not `>=`; this arm is what proves it); to `11m15s + 500ms` → red on Assertion D, before any mirror comparison runs.
   - **Liveness:** one script's `DRAIN_GRACE_S` back to a bare literal → red on Assertion B, that file only.
   - **Sign — new, closes F3:** `SHUTDOWN_GRACE_S - 30`, `SHUTDOWN_GRACE_S - 660`, `SHUTDOWN_GRACE_S * 0` → **all three red**. All three were green in round 1.
   - **Recognition — new:** `SHUTDOWN_GRACE_S=660` and `SHUTDOWN_GRACE_S: int = 660` in one file, the other left correct → **red on Assertion A**, that file named, because the mirror is now seen and is stale. Both were green in round 1 with that file silently unswept, which is what makes these the two arms that matter most.
   - **Control (P-23):** point the sweep at a directory that does not exist, at one containing no `.py`, and at a constant name that does not exist → Assertion C fatals rather than passing. Include a no-op arm proving the applied-diff assertion fires.
   - **Line shape (Assertion E):** `DRAIN_GRACE_S = (` opening a wrapped parenthesised expression → red **naming §7.1's line shape**, never claiming the value "does not derive". Round 1 printed the latter about a line that derives.
   - **Line shape — the annotated arm, re-scored:** `SHUTDOWN_GRACE_S: Annotated[int, Meta(u='s')] = 675` → red naming §7.1, with a message **true of the input**. The recogniser does **not** change: it already stops the optional annotation at the line's first `=`, which is exactly §7.1's bound as now stated. The only edit is dropping the words "whose value" from both failure messages, leaving *"… declares a `SHUTDOWN_GRACE_S` line that does not satisfy design §7.1: %q"* and its `DRAIN_GRACE_S` twin. Round 2 reddened on this input and said the line's *value* was at fault when the value was `675` (QA #13431 W-1).
   - **Dual (P-29/P-31) — enumerated from what Python permits, not from what these two files write:** trailing comment present and absent; blank line between the constants; constants reordered; a third `scripts/*.py` defining neither; extra spaces around `=`; the identifier appearing inside a single-line string. All green. **One member of round 1's dual list is retired:** `DRAIN_GRACE_S = 30 + SHUTDOWN_GRACE_S` was green and must now be **red naming §7.1** — §13 decision 4 says why the operand order is fixed.
   - **Known false positive (F8):** a column-0 `SHUTDOWN_GRACE_S = 999` inside a module docstring reddens. Expected, disclosed in §9, and not to be fixed.
6. **Gates** — `go test -count=1 -v ./...`, `gofmt -l .`, `go vet ./...`, all foreground and unpiped (P-7), plus the container gate (P-9/P-10). Report the test-count delta **by name**, not by arithmetic (P-11). Report the comment ledger per P-37, remembering that a new untracked `_test.go` is invisible to `git diff`.
7. **Do not run the scripts.** `smoke.py` costs two model calls and writes two undeleted graph records; `compare.py` costs one call per arm per task. The change is verified by the guard and by reading, not by a live run. If a live run is wanted, that is the operator's call and its cost is stated on #11340.

**The design document is a deliverable**: this file ships on the branch, and the PR body names its DiVoid node (#11034 P-48).

---

## 12. Pre-Design Checklist audit (#1136 §5)

**KISS / DRY / YAGNI**
- [x] No new type mirroring an existing one. `SHUTDOWN_GRACE_S` is a *deliberate* mirror across a language boundary with no import — the design's subject, priced in §8 and falsified in §9, not a §5.4 parallel type.
- [x] No new abstraction with one implementation. No abstraction at all: two constants, one test.
- [x] No element justified by "we might need X later". Alternative B is named as revisitable **if** the grace becomes operator-tuned — as a rejection, not a hook. Nothing is built for it.
- [x] No deprecation period, feature flag, compatibility shim or transition window.
- [x] DRY math quoted: `block_size × site_count = 2 × 2 = 4` lines, against a ~15–20 threshold, and #11034 P-2's "two sites is not more than two". The extraction stays at #11326 with the reason stated.

**Existing systems first**
- [x] Existing surface audited: `internal/server`'s own test package already owns the grace derivation and already reaches outside itself for a measured input (`divoid.DefaultTimeout`, the counting transport). The guard is one more test in it, not a new layer.
- [x] No new layer proposed. No new file in production code; one new test file, with `internal/condense/isolation_test.go` as the precedent for a same-package test inspecting another directory's source.
- [x] No new persisted data.
- [x] Consumer chain recursed: `SHUTDOWN_GRACE_S` has exactly one consumer, `DRAIN_GRACE_S`, which has one consumer, `stop_server`, which has three call sites (`smoke.py`, `compare.py`, `step_trace.py` by import). Named, not speculative — Assertion B exists precisely to keep the first link from becoming dead.

**Configurability**
- [x] No new config knob. Both values stay constants in code (#1136 §3).
- [x] No telemetry-then-tune compound.
- [x] Magic numbers stay magic and named: 675 mirrored under the name it mirrors, 30 inline with its cost asymmetry stated.

**Less is better**
- [x] Can-it-be-deleted: removing `SHUTDOWN_GRACE_S` loses the guard's anchor; removing the guard loses property 2; removing the +30 restores the zero-margin defect. Nothing else was added.
- [x] Can-it-be-merged: the mirror and the margin were deliberately **un**-merged — §6 gives the reason (one owner each; equality assertable on one half, freedom kept on the other), which is the one place this design adds rather than removes.
- [x] Trade-offs explicit: §8's B row and its closing paragraph.
- [x] Radical-clean where unconsumed: not applicable — every surface here has a named consumer.

**Document discipline**
- [x] Cites #114 (via #11034) and #1136 as load-bearing.
- [x] Scope inventory explicit; out-of-scope items listed with reasons (§2), not merely absent.
- [x] No multi-paragraph rationale for things that obviously stay. **Round 2:** §10's R3, R4 and R5 were paragraph-length restatements of §9 F6, §7.3 and §2, and are cut to pointers.
- [x] Supersedes no prior design; no banner needed.
- [x] §5-addendum on falsifiable universals: §9 states what falsifies each of the three, including F6 (the residual) and F8 (the docstring false positive). **Calibration from round 2:** one reviewer falsified three of this document's claims in a single pass (#13430 W-1, W-2, W-3). The addendum's discipline had been applied to the *universals* and not to the *reasons* — but a because-clause is exactly as falsifiable as a universal, which is what P-51 says, and §8 and §9 F3 now carry the measurements their original reasons lacked.
- [x] P-41: the one guard name this document cites is the one §11 step 4 instructs John to create, spelled identically. Round 2 changed it; §7.2 and §11 step 4 moved together, and the round-1 name survives only inside §7.2's correction note, quoted and `~~struck~~` — strikethrough being the exclusion P-41's command relies on. Run against this document the command's residue is **empty**, so the contract's own instrument reads the one name this checkbox claims. Round 2 left the quotation italicised and the residue read two (QA #13431 W-4).

**Not applicable:** data deliverables (no SQL, no migration, no schema), reader/carrier-swap inventories (no field rename).

---

## 13. Open questions

None blocking. Four decisions taken rather than asked, all cheaply reversible:

1. **The margin is 30 s, not 15 or 60.** Taken for consistency with `RUN_TIMEOUT_S`'s existing slack and the cost asymmetry in §7.3. Reversal cost: one integer in two files; the guard is indifferent to it.
2. **`RUN_TIMEOUT_S` is left alone.** Taken because its correct ceiling is the handler deadline, not the drain (§2). Reversal cost: none incurred — nothing here depends on it.
3. **Recognition widens and validation narrows — rather than narrowing §7.1 to match round 1's guard.** Taken because a contract the instrument cannot observe is not enforcement: forbidding `SHUTDOWN_GRACE_S=660` in prose while the guard still cannot see it leaves the silent blindness exactly where QA found it. Alternatives by name: *narrow §7.1 to the single spelling and leave the literal-prefix match* — rejected, the violation stays unobservable and the file stays unswept; *widen the guard but leave §7.1 permissive about the value* — rejected, a recognised line the guard cannot read then has no stated verdict, which is how round 1 came to report "does not derive" about a line that derives. Reversal cost: one row of §7.1 and one matcher in the guard.
4. **`DRAIN_GRACE_S` has one canonical form, `SHUTDOWN_GRACE_S + <integer ≥ 1>`, and a reversed operand order is a §7.1 failure.** Taken because §7.1 already imposed a single canonical form on the mirror — a bare decimal integer, which forbids the perfectly legal `11 * 60` this repo itself used before this change — and one rule applied twice is cheaper to state, implement and read than two. Alternatives by name: *accept either operand order* — rejected, an extra branch and an extra dual arm bought for a spelling nobody has written; *accept any expression naming the mirror* — that was round 1, and it is the hole QA measured. **This retires a dual member:** `DRAIN_GRACE_S = 30 + SHUTDOWN_GRACE_S` passed round 1's guard and must now redden naming §7.1. Reversal cost: one branch in the guard and one row in §7.1.

One thing the orchestrator may want to do outside this PR: **#11326's listing of this constant is understated** — it still describes a zero margin where the margin is now negative — and `RUN_TIMEOUT_S`'s derivation is unrecorded anywhere. Both are node edits, not code.
