# Architectural Document: The ceiling that governed admission

> Repo path: `docs/architecture/the-ceiling-that-governed-admission.md` on branch
> `design/the-ceiling-that-governed-admission` in `github.com/telmengedar/processor` (canonical copy — the
> DiVoid node **#13522** carries the same document verbatim, byte for byte; §7.5 is why the pointer is inside
> the file rather than prepended to the node).

*Standards applied: Design Contracts **#1136** (§1 KISS/DRY/YAGNI, §2 existing systems first, §3 configurability, §4 less is better, §5 Pre-Design Checklist — audited in §15 below; **§5/#868 is the provision that carries this design**, see §10). Code Contracts **#114 §0** in its Processor form **#11034** (P-2, P-3, P-17, P-20, P-27, P-29, P-40, P-41, P-42, P-43, P-44, P-51, P-52). DRY threshold **#1267**. Sweep discipline **#1225** (2026-09-10 stem rider, and the datum-keyed sibling round 2 added — §3.1).*

*Source question: the open question **#13483** deliberately left unanswered, re-read against **#13507**. Related: **#11326**, **#13429**, **#13428**.*

*Measured at `main` = `860c0e7`, read-only. Generators preserved and named in §3.1.*

> **Revision history.** Every correction below is recorded, dated, and struck where superseded **at its own
> site**. This table names the round and points there; it deliberately does **not** restate what those sites
> already carry, because a claim carried in two places is the defect this document exists to remove (§7.2,
> §10, §15). Only what lives nowhere else is written here: the review, its verdict, and what the round did to
> the design as a whole.
>
> | rev | review | verdict | what it did to the design | recorded at |
> |---|---|---|---|---|
> | 1 | — | — | first draft | — |
> | 2 | **#13526** | REJECTED, 3 critical | **the central decision was upheld and is unchanged.** The anchor charge turned out to have landed *inside* the record span, which deleted the legacy derivation, the shared accessor and Go's fallback; the reader inventory was re-keyed on the datum; §10's justification was replaced | §3.4 · §7.3 · §12 F-3 · §3.1 pass H · §3.3(c) · §7.4 · §10 · §13 · §15 |
> | 3 | **#13530** | APPROVED WITH WARNINGS, 0 critical | **no contract changed and no decision moved** — seven precision defects | §3.2 · §3.4 · §7.3 · §7.5 · §12 F-1/F-7 · §14 · §15 |
> | 4 | — | — | this table. The two prose banners it replaces restated corrections their own sites already carried — the document's history contradicting the document's thesis, in the one section a sceptical reader checks the thesis against | §15 |
>
> Every row's pointers were verified to resolve before the banners were removed; the struck text and dated
> notes at those sites are the record, and this table is only an index to them.

---

## TL;DR

**What.** The loop computes the byte ceiling candidates are actually admitted against — `budget − anchor.size`, floored at zero — uses it to decide every admission, and then **throws it away**. It publishes its *inputs* and withholds its *result*, so five renderers reconstruct it. And the arithmetic that produces it **changed once, five days before this design was written, inside the span of the stored records**: so each of the **three renderers that actually read the stored corpus** is right on one slice and wrong on the other, and **no static choice of denominator is right on both** (§3.4). The two Go renderers never meet a stored record at all (A6), so for them the same inversion runs across *code eras* rather than corpus slices. This is not a duplication problem and DRY does not justify fixing it (§10 states the math and it argues *against* extraction). It is a missing-output problem, and a recorded per-run value is the only thing that can be right across a corpus whose arithmetic moved.

**How.** The producer publishes the number it governed with. `Assemble` already computes it; the run record carries it, and `eval.RowResult`'s `budgetBytes` is **replaced** by it rather than added beside it. Every renderer reads it and none derives it. `summaryRemainingAfterAnchor` deletes outright — with no fallback, because no Go path ever decodes a stored record (A6). `step_trace.py`'s subtraction deletes; `compare.py` and `smoke.py` never acquire one. Where the field is absent the answer is **not knowable**, never a number: the pre-field ceiling cannot be recovered from a record, and revision 1's attempt to recover it was measurably wrong.

**The cost, where non-zero.** Four. (1) One integer added to the run record — the **durable** surface, where the change is purely additive — and one field's meaning **changed** on the eval result, which is an ephemeral stdout stream regenerated per sweep (verified: `parseFlags` declares no output path and the package writes no file). (2) **All 18 stored records predate the field**, so every script renders "not knowable" for every historical record until new ones accumulate. That is a real loss of operator utility and it is the price of not printing a number that is wrong on at least 2 of the 18 and unfalsifiable on the rest — §16 D-2. (3) Three of the five renderers are Python and **this repo has no Python gate**: README names `go test`, `gofmt`, `go vet` and the container only, so `scripts/`' 166-test suite is green and run by nobody. §16 D-4. (4) Nothing here *forces* a future renderer to read the ceiling rather than the raw constant; §12 F-1 says so and names its falsifier.

**The strongest rejected alternative.** *Leave the copies and add a guard* — the shape this repo shipped for the drain grace (#13428) and the shape the source question invited. Rejected on a **signature** fact, not a fixture one: `BuildRow(row, queries, dispositions)` does not receive the anchor, so the correct value at the newly-found fifth site is **not computable from the inputs that site is given**. No guard and no fixture reaches that. A data change is required there regardless of what happens to the other four; once it is, recording *the number that governed* dominates recording *another input to re-derive it from*. §11 A.

---

## 1. Problem Statement

`internal/loop/assemble.go`'s `Assemble` charges the anchor's bytes against the assembly budget in full before any candidate is considered, and admits candidates cumulatively against what is left:

> `remaining := budget - len(anchor.Content)`, floored at zero, then `admit(candidates, remaining)`.

`remaining` decides every admission in the initial round. It is a local variable — never recorded, never returned, never named in any artifact the run produces.

Every artifact that later *explains* those admissions — to an operator at a terminal, or to a model reading a stored record — needs that number and is not given it. Each is handed `limits.assemblyByteBudget` (a constant that governed nothing on its own) and, where present, `anchor.size`, and asked to reconstruct the ceiling. **And the reconstruction rule is not stable over the corpus**: §3.4 measures the commit where it changed.

**Success criterion.** An artifact that states or implies a byte ceiling states the ceiling that governed the run it describes, or states that it does not have one. No artifact reconstructs it from parts.

**Non-criterion.** Reducing the number of places the subtraction is written. §10 shows that count is already below every DRY threshold this project holds.

---

## 2. Scope & Non-Scope

### In scope

| | |
|---|---|
| `internal/loop` | publish the initial round's admission ceiling on the run record; delete `summaryRemainingAfterAnchor` |
| `internal/eval` | **replace** `RowResult.BudgetBytes`'s meaning with the governing ceiling; `report.go`'s cut-miss denominator follows; **invert** the two assertions named in §7.4 |
| `cmd/eval` | thread the ceiling `Assemble` applied into `BuildRow` — a **signature** change, since `BuildRow` cannot compute it today |
| `scripts/step_trace.py` | read the published ceiling; **delete** its subtraction |
| `scripts/compare.py` | read the published ceiling in `describe_cut`'s caller |
| `scripts/smoke.py` | read the published ceiling; the `UNADMITTABLE` predicate tests against it |
| `docs/architecture/m2-retrieval-eval.md:848` | a dated in-place revision note on the **field-spec row** — see §7.5 |
| the discriminating fixture | one record shape, five renderers, one guard each |

**Revision 2 deleted two rows that revision 1 had here**: a shared Python accessor and a legacy derivation. §7.3 says why both are gone.

### Explicitly out of scope

**`#11326`'s harness extraction.** Revision 1 put one function into a shared Python module and had to reason about not pre-empting #11326's shape. Revision 2 introduces **no Python module at all** (§7.3), so the entanglement is gone rather than managed.

**`supplementaryByteBudget`.** `dispatchRecall` calls `admit(candidates, SupplementaryByteBudget)` **directly** — no anchor in scope, no derivation, no defect. The supplementary round's ceiling *is* a constant already in `limits`. Publishing a second one would restate it — #1136 §2 Form 3.

**`anchor.size` and `limits.assemblyByteBudget` on the run record.** Both stay. `step_trace.py`'s anchor-charge sentence legitimately needs all three numbers, and the limits lines state the constants *as constants*, which is honest and is not this defect.

**m2 §3.2's prose and its 96.3 % / 54.2 % figures.** Both of its records predate the anchor charge (§3.4), so those figures are correct as recorded and **P-43 protects them**. Recomputing #10897 against `budget − anchor` yields **101.1 %**, an impossible utilisation — which is the proof that they must not be recomputed. Only the field-spec row at :848 is in scope.

**PR #60 (`compare.py`).** In review, not merged at `860c0e7`. §16 D-1.

**CI.** #13429, filed and blocked on a decision that is not a finding's to make. §16 D-4 makes only the documentation half.

**`docs/architecture/*.md` prose restating the derivation.** Dated records under P-43. Not sites. §3.6.

---

## 3. The measurement

### 3.1 The instruments — all eight passes, and the one revision 1 was missing

Generators preserved, both deterministic and creating no state:

- `C:\dev\claude\_scratch\budgetderiv-k4m9\sweep_budget_sites.sh` — passes A–G, takes the repo root as its one argument.
- `C:\dev\claude\_scratch\budgetderiv-k4m9\check_charge.py` — takes run-record JSON paths and re-derives §3.4's split.

| pass | keyed on | what it can and cannot see |
|---|---|---|
| **A** | `assembl[a-z]*[_ ]?[Bb]yte[_ ]?[Bb]udget` over `*.go` `*.py` `*.json` | every *name* of the constant. Blind to a site that renders the figure without naming it |
| **B** | the same stem, markup-tolerant, over `*.md` | prose restating the derivation. **Keyed on the derivation, so blind to the datum** — this is the round-2 CRITICAL 2 |
| **C** | a budget-ish name minus an anchor-ish name | every site that **states** the derivation. 7 hits |
| **D** | a size compared against a budget-ish or remainder-ish name | every site using a ceiling as an **admissibility predicate**. 5 hits |
| **E** | `cumulativ` | the admission loop itself |
| **F** | `remain(ing\|der)` in code | the *result* under any name |
| **G** | `git ls-files -- '*.py'` | the denominator of D and F |
| **H** *(new, revision 2)* | **`git grep -in -E 'budget[_ ]?bytes' -- '*.go' '*.py' '*.json' '*.md'`** | **the datum §7.2 changes, by its own name** |

**Pass H returns both of revision 1's misses in one command**: `internal/eval/report.go:149`, which revision 1 found only by reading pass A's residue, and `docs/architecture/m2-retrieval-eval.md:848`, which revision 1 never found at all.

**The instrument lesson, in the form that is checkable.** #1225's stem rider fixes the *shape* of a pattern. This is about the *noun the pattern is keyed on*: **a sweep can only find a claim that has a token**, and `%d/%d bytes admitted` names neither the constant nor the subtraction — the assertion *"this denominator is the ceiling"* is carried by the slash. No pattern over the arithmetic reaches it at any stem. **When a design changes a datum, sweep the datum's name, not the derivation that produces it.** Revision 1 changed a noun and swept for a verb, then ticked §15's reader-inventory box against an instrument that could not satisfy it.

*Every count here is from `git grep`, which is tracked-file scoped and therefore immune to the four nested worktrees under `.claude/worktrees/`. A reviewer reproducing the sweep with `grep -rn` will get roughly quadrupled counts.*

### 3.2 The sites

One origin and five restatements. **#13483's table names three; #13507 corrects it to four; it is five, and the fifth is in Go.** Of the six sites, three are Go (`Assemble`, `summary.go`, `eval/report.go`); of the five *restatements*, two are Go.

| # | site (by symbol) | lang | shape | right pre-charge? | right post-charge? |
|---|---|---|---|---|---|
| 0 | `loop.Assemble` | Go | computes it and applies it | the origin — correct by construction at every revision | |
| 1 | `loop.summaryRemainingAfterAnchor` | Go | re-derives; renders as a denominator | **no** | **yes** (PR #59, `ec75787`) |
| 2 | `eval.missLine`'s cut branch, via `eval.BuildRow`'s `BudgetBytes` | Go | renders a raw-constant denominator; **cannot** re-derive | **yes** | **no** |
| 3 | `step_trace.render_trace` → `render_candidate_table` | Python | re-derives; renders **and** predicates | **no** | **yes** |
| 4 | `compare.describe_cut`'s caller | Python | tests against the raw constant | **yes** | **no** (PR #60 in review) |
| 5 | `smoke.print_turn`'s `UNADMITTABLE` block | Python | tests against the raw constant | **yes** (silently) | **no** — under-reports |

**Every one of the five is wrong somewhere, and the two columns are each other's inverse.** Revision 1 said *"four of five were or are wrong"* and certified site 3 as correct; §3.4 shows site 3 is wrong on the pre-charge records, and sites 4 and 5 are accidentally right on them.

**Two readings of the columns, and only one of them is about the corpus.** Sites **3, 4 and 5** are the scripts, and an operator can point them at #10897 or #12981 today — for those three the columns are literally corpus slices, and **{3} wrong pre-charge / {4,5} wrong post-charge is disjoint and exhaustive over them.** Sites **1 and 2** are Go, and **A6 says no Go path ever decodes a stored record**: each renders only what its own process just produced, so their cells state *which era's code was right*, not which slice they were pointed at. Site 1 is doubly so — `internal/loop/summary.go` was created at `004afa8` (2026-09-08), **three days after the charge**, and `summaryRemainingAfterAnchor` itself at `d4bc7f5` (2026-09-10, merged as PR #59 = `ec75787`), so it never existed in the pre-charge era in either reading. Revision 2 wrote the corpus claim over all five, which A6 forbids for two of them.

**The conclusion is untouched by the narrowing.** Over the three corpus readers the partition is still disjoint and still exhaustive: no static choice of denominator is right about every stored record, so only a value recorded per run can be. And site 2's defect never needed the corpus argument at all — §3.3(b)'s signature fact is independent of it.

### 3.3 Three facts that decide the design, all measured here

**(a) The failed boundary is not the language boundary.** The source question frames this as *"Go computes it; the restatements are Python; there is no import."* Site 2 falsifies it: **Go**, same module, one import from `loop` (`internal/eval/result.go:6`), and wrong. The boundary that failed is between **the code that decides admission and the code that explains it**, and it runs through `internal/`.

*(Revision 1 wrote: ~~three of the five restatements are Go~~. **Two** of the five restatements are Go; three of the **six sites** are. Corrected in the document and in the DiVoid node's substance. The conclusion rests on site 2 alone and is untouched.)*

**(b) The correct value is not computable from the inputs site 2 is given.** `RowResult` carries no anchor size — and `BuildRow(row Row, queries []string, dispositions []loop.Disposition)` **does not receive the anchor either**, though `cmd/eval`'s `rowDispositions` fetches the full-content node and hands it straight to `Assemble`. This is a **signature** defect, not a fixture defect: no fixture supplies an operand a signature does not accept, and no guard over `report.go`'s text can reach it. **This single fact justifies the design at a defect rate of one in five**, and it is what §11 A's rejection rests on.

**(c) The site is not unguarded — it is guarded to the wrong value.** Two live assertions pin the defect:

- `internal/eval/result_test.go:123-124` — `if got.BudgetBytes != loop.AssemblyByteBudget { … }`
- `internal/eval/report_test.go:157` — `mustContain(t, human, fmt.Sprintf("k'=7, 32504/%d bytes admitted", loop.AssemblyByteBudget))`

Both must be **inverted**, and §14 Unit 2 says so. Revision 1 called the site ~~not merely unguarded but unguardable with the fixtures present~~; it is guarded, to the raw constant, by assertions the design had not read.

*(Revision 1 cited **P-21** for the 16-byte anchor at `internal/eval/result_test.go:13`: ~~This is #11034 P-21 exactly~~. **The citation is struck.** P-21 forbids a fixture that leaves two implementations *indistinguishable*; 60,000 and 59,984 are distinguishable by exact equality. A 16-byte anchor is **illegible, not indiscriminating**. A bigger anchor is still wanted, so a reader can see which denominator a failure names — but that is a readability argument, not a contract one. P-51: a rationale you transcribe is a claim you assert.)*

### 3.4 The split — the arithmetic changed inside the span of the records

**One command, one hit:**

```sh
git log -S 'remaining := budget - len(anchor.Content)' -- internal/loop/assemble.go
# e307a24  fix(loop): charge the anchor against the budget, so the constant bounds what it names
#          2026-09-05T15:23:10+02:00  =  13:23:10 UTC
```

At `e307a24^`, `Assemble` passed `budget` **raw** to `admit`; the anchor was not charged and the governing ceiling *was* the constant. I read the parent revision to confirm this rather than inferring it from the diff stat.

**The 18 stored records split 2 / 16.** Enumerated by `divoid_list` on `name LIKE 'processor-run %'` — total 18, confirming the count both prior tasks report. **#10897** (2026-09-02T11:35:08Z) and **#10898** (2026-09-02T11:37:42Z) predate the commit; the earliest of the other sixteen is **#11384** at 2026-09-05T15:20:35Z, two hours after it.

**#10897 disproves any backwards derivation arithmetically** — re-derived by me with `check_charge.py`:

| record | budget | `anchor.size` | `budget − anchor` | bytes actually admitted | verdict |
|---|---|---|---|---|---|
| **#10897** | 60,000 | 2,872 | 57,128 | **57,756** | **admits 628 B more than the derivation says existed** — 101.1 % of a ceiling it never faced |
| #10898 | 60,000 | 7,095 | 52,905 | 32,504 | consistent with the charge — **so the invariant does not detect it** |
| #11384 | 60,000 | 3,408 | 56,592 | 56,336 | consistent, and tight (99.5 %) — evidence the charge was in force |

**#10898 is the important row.** A detector of the form `admittedBytes > budget − anchor.size` catches #10897 and is silent on #10898, so **there is no reliable way to tell a pre-charge record from a post-charge one by reading it.** I checked the key set **of the two pre-charge records themselves**, since they are the class the claim is about and their shape differs from a current record's: #10897 and #10898 both carry `input, subject, query, anchor, candidates, block, answer, model, toolCalls, modelCalls, capReached, usage, stopReason, written, limits` — they have **`written`** (empty, `{}`, which is the #10899 defect) and lack `queries`, `workspace` and `sampling`. **No version, no commit, no arithmetic marker in either**, and `limits` is identical on both. *(Revision 2 quoted #12981's key set here — a post-charge record, one hop outside the class the sentence is about. P-44. The conclusion is unchanged; the evidence now comes from the right records.)*

**The pre-field ceiling is therefore not knowable from the record**, and that is what collapses §7.3.

### 3.5 The live instance, re-measured rather than inherited

Re-derived from run record **#12981** directly: budget 60,000, `anchor.size` 18,208, governing ceiling **41,792**, candidate **#6375** at rank 4, **43,273 B**, `included: false`, `cutReason: "byte budget exceeded"`, cumulative admitted ahead **1,111**.

`43,273 > 41,792` — unadmittable at rank 1 with nothing ahead of it. `1,111 + 43,273 = 44,384 ≤ 60,000` — so `compare.describe_cut`, tested against the raw constant, takes its **cumulative** branch and reports *"fits the budget alone but not after the candidates admitted ahead of it"*, which is false. `smoke.py` prints nothing. `step_trace.py` flags it correctly. Confirms both prior tasks on this record.

*#13507's further claim — `anchor.size` present on 18/18 — is that task's measurement and not mine. **After §3.4 it is no longer load-bearing anywhere in this design**, because presence was never the discriminator: both operands are present on #10897 and the derivation is still wrong.*

### 3.6 Deliberate near-misses — recorded as deliberate, per #1225

Each matched a pass and is **correctly** keyed to the raw constant, or is a different quantity.

| site | why it is right |
|---|---|
| `loop.self_poisoning_test`'s setup precondition | asserts a *fixture* exceeds the constant — a setup guard, not a claim about a run |
| `eval.writeSummary`'s `limits … assemblyByteBudget=` line | states a constant as a constant |
| `compare.py`'s prose distinguishing supplementary from assembly budget | states the distinction correctly |
| `step_trace.py`'s supplementary call passing `supplementary_budget` raw | correct — `dispatchRecall` has no anchor to subtract, and the docstring says so |
| `loop.turn_test`'s `AssemblyByteBudget + SupplementaryByteBudget*(MaxModelCalls-1)` | a different derivation (the whole-run ceiling) |
| every `strings.Repeat(…, loop.AssemblyByteBudget+1)` fixture | constructs an oversized row; asserts nothing about a ceiling |
| `internal/condense/report.go:167`'s `%d B from %d B · ratio %.3f` | **the only other byte-ratio renderer in the tree** — condensation input/output, not an admission ceiling |
| `internal/eval/report.go:106`, `:221`, `:223` | `%d/%d` over **row counts**, not bytes |
| `docs/architecture/*.md` prose | dated records under P-43 — but **not** m2:848, which is a field spec and is in scope (§7.5) |

`internal/eval/result_test.go`'s exactly-`AssemblyByteBudget`-sized candidate remains flagged rather than resolved: whether that test's *name* claims more than its fixture can carry is a **P-20** question for Unit 2 (§16 D-6).

---

## 4. Assumptions & Constraints

| | |
|---|---|
| **A1** | The admission arithmetic is correct **as of `e307a24`** and is not in question. Nothing here changes what is admitted. |
| **A2** | There is no import across Go→Python and none is being introduced. The record is the crossing and already exists. |
| **A3** | This repo has no CI (#13429). Every gate is a command a person runs. |
| **A4** | Stored run records are immutable graph nodes. The 18 that exist can only be read, and **all 18 predate the published field**. |
| **A5** | The run record is read by a model as well as by operators (#13245, #13480). |
| **A6** | **No Go path decodes a stored run record.** Verified exhaustively: every non-test `Unmarshal`/`Decode` in the module is a graph-client, corpus, derivations-sidecar, model-adapter or HTTP-request decode; none produces a `loop.Record`. `RenderSummary`'s only production caller is `internal/divoid/write.go:68`, on the in-process record the same run just built. **So absence of the ceiling is a Python-side concept only**, and Go needs no fallback branch. |
| **C1** | `internal/loop` has no dependency on `internal/eval` and must not gain one. |
| **C2** | The module has zero third-party dependencies by policy. |
| **C3** | `go.mod` declares `go 1.27`. Verified as a `go.mod` fact, not by compiling a change. |

---

## 5. Architectural Overview

Today — the producer keeps its result and publishes only its inputs, and the reconstruction rule is not stable over the corpus:

```
   loop.Assemble            (rule changed at e307a24, INSIDE the record span)
   +--------------------------------------+
   | remaining := budget - anchor.size    |  <- governs every admission
   | admit(candidates, remaining)         |
   +---------------+----------------------+
                   |  remaining is DISCARDED
                   v
        run record                    eval RowResult
        assemblyByteBudget            budgetBytes = the raw constant
        anchor.size                   (no anchor - and BuildRow is not GIVEN one)
                   |
     +-------------+---------------+--------------+--------------+
     v             v               v              v              v
 summary.go   eval/report.go   step_trace.py  compare.py     smoke.py
  pre: wrong    pre: right       pre: wrong     pre: right     pre: right
  post: right   post: WRONG      post: right    post: WRONG    post: WRONG
```

*Read the last two rows together: every renderer is wrong on one slice of the corpus, and the two
groups are each other's inverse. That is what no reconstruction rule can fix and a receipt can.*

After — the producer publishes the number it governed with, and nobody reconstructs it:

```
   loop.Assemble
   +--------------------------------------+
   | ceiling := max(budget - anchor, 0)   |  <- ONE statement of the derivation, at the origin
   | admit(candidates, ceiling)           |
   +---------------+----------------------+
                   |  ceiling is RETURNED and RECORDED
      +------------+------------+
      v                         v
 run record                eval RowResult
 + candidateByteCeiling    budgetBytes <- REPLACED (cmd/eval threads it in)
                   |
     +-------------+---------------+--------------+--------------+
     v             v               v              v              v
 summary.go   eval/report.go   step_trace.py  compare.py     smoke.py
    reads         reads            reads          reads         reads
   (no absence case - A6)     \-- absent => "not knowable", never a number
```

**The idea in one sentence.** The number that decides an admission and the number that explains it are the same number, so the code that decides hands it over — and once the arithmetic has moved under a corpus, handing it over is the only thing that can be right about every record in it.

---

## 6. Components & Responsibilities

| component | owns | does **not** own |
|---|---|---|
| `loop.Assemble` | the **single** statement of the derivation, applying it, and surfacing it | rendering it; what any consumer says about it |
| `internal/loop` — the record | publishing the ceiling `Assemble` applied | the raw constants (`limits` owns those); the supplementary ceiling (a constant, already carried) |
| `loop.RenderSummary` | stating the ceiling it was handed | deriving it. `summaryRemainingAfterAnchor` **deletes**, with no fallback (A6) |
| `cmd/eval` | **threading** the applied ceiling from `Assemble` into `BuildRow` — the seam a real anchor passes through | rendering |
| `internal/eval` — `RowResult` | carrying the governing ceiling under `budgetBytes` | carrying the raw constant under that name. `Result.Limits` keeps the constant, once, where it is honest |
| `internal/eval` — the report | stating a denominator that is true | subtracting anything |
| `step_trace.py` / `compare.py` / `smoke.py` | what each says about a row given a ceiling, **and how each renders "not knowable"** | obtaining the ceiling by arithmetic |
| the shared discriminating fixture | one record shape where raw-constant and governing-ceiling implementations **must** disagree | asserting anything itself |

*Revision 1 had a "shared Python accessor" row here. It is gone — §7.3.*

---

## 7. Contracts

### 7.1 The published ceiling — run record

**Semantics.** The cumulative byte ceiling the **initial** round's candidates were admitted against: the value `Assemble` passed to `admit`. Not a constant, not a limit, not a budget — the applied ceiling of one run.

| property | contract |
|---|---|
| **name** | one member on the run record (`candidateByteCeiling` working name; §16 D-3). **Not a member of `Limits`** — that type's doc comment says *"the five constants that governed one run"*, and a derived figure inside it falsifies that sentence. A **P-44** one-hop check either way |
| **value** | integer, ≥ 0, floored at zero exactly as `Assemble` floors it |
| **invariant** | `ceiling ≤ limits.assemblyByteBudget`, and `ceiling = max(assemblyByteBudget − anchor.size, 0)` — **as a property of records this design's code writes, forward only.** It does **not** hold backwards: §3.4 measures a record that violates it by 628 B. Revision 1 stated this invariant without that boundary and used it to license a backwards derivation |
| **absence** | possible only on the 18 pre-field records, and only in Python (A6). A consumer reading absence renders **not knowable** and makes no admissibility claim — never 0, never a derived number. `MissingLimitsDoesNotFabricateNumbersTests` (`scripts/test_step_trace.py:363`) already ships this discipline for the sibling field |
| **position** | late in the record's field order, adjacent to `limits`. #13481 measures the record's embedded window as dominated by the candidate list, and `limits` is second-to-last (verified: the key order ends `…, stopReason, limits, sampling`), so an integer beside it does not move what the record is findable by |
| **risk** | **additive on the durable surface.** No stored record's meaning changes; no reader of an existing key is affected |

### 7.2 The changed field — `eval.RowResult.budgetBytes`

A **replacement, not an addition**, and the sharper half of the change.

| property | contract |
|---|---|
| **before** | `loop.AssemblyByteBudget`, set unconditionally in `BuildRow`, and the denominator of the cut-miss line |
| **after** | the ceiling `Assemble` applied for that row, **threaded in by `cmd/eval`** — a signature change, since `BuildRow` cannot compute it from what it receives (§3.3b) |
| **why replace and not add — the affirmative argument** | **The raw denominator inverts m2 §3.2's prescription on a live record.** m2 specifies `admittedBytes / budgetBytes` as *the* discriminator between two opposite tunings. On **#12981** (post-charge; anchor 18,208, admitted 41,423): against the raw 60,000 it reads **69.0 %** — m2's *"budget half empty, one oversized node is costing you candidates; cap per-candidate size"* regime — and against the governing 41,792 it reads **99.1 %** — m2's *"the budget is genuinely too small; raise it"* regime. **Opposite prescriptions from the same run.** §7.2 does not endanger m2's discriminator; it is the only denominator under which the discriminator means what m2 says it means. Measured, and it survives §3.4 untouched |
| **why replace and not add — the secondary argument** | `budgetBytes` has exactly one reader, and its only correct value is the ceiling. Adding a second field would leave the wrong number in scope beside the right one at the site that has been getting it wrong. Weaker than the argument above, and kept only because removing the wrong number is the one move here that *forces* correctness rather than enabling it (§12 F-1) |
| **the constant is not lost** | `Result.Limits.AssemblyByteBudget` still carries it and the report header still states it, once, as a constant |
| **risk** | **low, and lower than revision 1 claimed.** `cmd/eval` writes the measurement as JSON to **stdout** (README:50); `parseFlags` declares only `-corpus` and `-derivations`, and the package writes no file. The eval result is a stream regenerated from the live graph per sweep; nothing in this repo stores one. §16 D-5 |
| **fixtures** | `report_test.go` builds `RowResult` literals and **cannot** carry an anchor; the legibility argument for a bigger anchor belongs to `cmd/eval` and `internal/eval/result_test.go`, where `Assemble` actually runs. §7.4 |

### 7.3 Absence — and why there is no legacy derivation, no accessor, and no module

Revision 1 specified a three-way accessor in a shared Python module: published field, else `max(budget − anchor_size, 0)`, else not-knowable. **§3.4 deletes the middle branch**, and with it everything built to hold it.

- **The middle branch is wrong for records it would serve.** #10897 has both operands present and the derivation over-states the ceiling it faced by 628 B. **Presence was never the discriminator**, which is what revision 1's D-2 keyed on.
- **There is no discriminator.** No version, no commit, no marker in the record; the `admittedBytes` invariant catches #10897 and is silent on #10898 (§3.4). Keying a renderer to `e307a24`'s date would couple every script to a commit — a worse coupling than the one being removed, and rejected in §11 G.
- **So the contract is two-way: read the field, or say it is not knowable.** Nothing subtracts anything, anywhere.
- **And Go has no absence case at all** (A6): `RenderSummary` only ever sees the in-process record the run just built, so the ceiling is a struct member that is always set. `summaryRemainingAfterAnchor` deletes with no fallback and no branch.
- **What remains in Python is a key lookup and a sentinel.** `block_size × site_count ≈ 2 × 3 = 6`, and 2 lines is not over 5 — **P-2 gates the extraction out on the block clause**, matching §10's row for the same computation. *(Not on the site clause: **3 is more than 2**, so that clause is satisfied and does not gate. Revision 2 said "both clauses" and contradicted its own §10.)* One failing clause is enough. **No shared module**, and #1136 §4's can-it-be-inlined check is what deletes it.
- **Two of the three scripts already have an absence idiom; the third has the failure mode §7.1 forbids.** `step_trace.py:468` says *"the record carries no limits.assemblyByteBudget, so the anchor's charge against it cannot be computed"*, and `compare.py`'s `budget is None` branch is explicit. **`smoke.py:220` is neither** — `if budget and row.get("size", 0) > budget:` is a silent truthiness short-circuit that prints nothing on an absent budget, which is exactly what §7.1's absence row exists to forbid. It does not move the P-2 gate (the block clause carries it), and §7.4's absence row already requires new behaviour there — but `smoke.py` is the script with no test file, which is how it acquired the original defect, so it is the one to write first.

**Net effect on the derivation, in executable code.** Measured today: three sites *state* it (§10). After Units 1 and 3: **one** — `Assemble`, the origin, which is not a restatement. **Prose is a separate count and Unit 3 takes it too**: `step_trace.py:113` (the module docstring) and `:349` both state the rule in words, and §2's P-43 exclusion covers `docs/architecture/*.md`, not a script's own header. Left standing, the script would ship carrying the derivation this design proved unreliable, inside the one script that reads pre-charge records.

### 7.4 The guards

One shared record shape, whose property is stated so it can be checked: **an implementation testing against the raw constant and one testing against the governing ceiling must produce different output on it.** The #6375 geometry is real (§3.5): a non-trivial anchor and a cut candidate strictly between the ceiling and the raw constant, at rank 1 with nothing ahead of it. The fixture is shared; the assertions are not. `UnadmittableUsesRemainingBudgetTests` is the existing instance and the model to copy — it names in its docstring the wrong neighbour its fixture rules out.

**One row per component** (P-42: a row names a guard and states the premise that makes it discriminate).

| component | guard | premise that makes it discriminate |
|---|---|---|
| `loop.Assemble` | **new** — the ceiling it surfaces equals the ceiling it applied | a mutation surfacing `budget` instead of the remainder must redden |
| `loop.RenderSummary` | exists — `TestRenderSummaryHeadsTheAdmittedListWithItsCountAndBytesAgainstTheSpaceRemainingAfterTheAnchor`, `TestRenderSummaryFloorsTheRemainingAfterAnchorAtZeroWhenTheAnchorAloneExceedsTheAssemblyBudget` | a 3,408 B anchor makes the headline denominator differ by 3,408. Re-point from the derivation to the field |
| **`cmd/eval`** | **new — this is the seam, and revision 1 had no row for it** | a real full-content anchor runs through `rowDispositions` → `Assemble` → `BuildRow`; the guard fails if the ceiling `BuildRow` receives is not the one `Assemble` applied. **This is the only place a real anchor exists**, so it is the only place a fixture anchor can discriminate |
| `internal/eval` report | **invert** `report_test.go:157` | it asserts the rendered denominator **is** the raw constant. Its fixtures are `RowResult` literals with no anchor, so its premise is *the report prints the `BudgetBytes` it is handed* — the anchor argument does not belong to this component |
| `internal/eval` `BuildRow` | **invert** `result_test.go:123-124` | it asserts `BudgetBytes == loop.AssemblyByteBudget`. After the change it must assert the threaded ceiling, and a bigger anchor makes the two visibly different rather than 16 B apart |
| `step_trace.py` | exists — `UnadmittableUsesRemainingBudgetTests`, `ShutoutOversizedAnchorTests` | already built on this geometry. Re-point from the derivation to the field |
| `compare.py` | **new** | the row must fall in the cumulative branch under the raw constant and the oversized branch under the ceiling |
| `smoke.py` | **new** — `smoke.py` has no test file today | the row must be silent under the raw constant and flagged under the ceiling |
| all three scripts | **new** — the absence case | a record with no ceiling renders "not knowable" and no number |

### 7.5 What is owed to m2 — a dated note on the field spec, not a correction to §3.2

`docs/architecture/m2-retrieval-eval.md:848` is the row of m2's per-row field table that **specifies what `budgetBytes` carries and why**:

> `| admittedBytes, budgetBytes | §3.2's discriminator — 96% versus 54% utilisation are opposite problems |`

It carries no derivation, so §2's P-43 exclusion of design prose does not reach it — it is the spec of the shipped type, and this design changes what that type carries.

**The contract:** a **dated in-place revision note**, in the same form the `required[] → size` row two lines below already uses (*"**Revision 8:** `size` is specified here and is **not** in the shipped type…"*). m2's current revision is 8, so this is **revision 9**. It must state that `budgetBytes` now carries the applied ceiling, and that sweep figures taken before the change are not comparable with figures taken after.

**It must not touch §3.2's prose or its table at `:185`.** Both of m2's records predate `e307a24` (§3.4), so 57,756 / 60,000 = 96.3 % and 32,504 / 60,000 = 54.2 % are correct as recorded and P-43 protects them. #10897 against `budget − anchor` yields 101.1 %.

**P-40 applies to this edit, and the parity form is `cmp`-literal.** m2's masthead states the DiVoid node carries the document verbatim, so the m2 file and its node are one unit of change with one author — whoever makes this edit republishes both and `cmp`s them.

**The form, stated because this document must model what it demands.** A repo-backed design node and its file are `cmp`-equal — **one digest, quoted once, true of both** — and the way to get there is m2's: the repo-path pointer lives **inside the document**, so it is carried by the file and the node alike. It is not prepended to the node at publish time, because a prefix only the node carries makes every parity claim a claim about a *stripped* body, and "identical after removing the part that differs" is a weaker sentence that reads like a stronger one. Revisions 1 and 2 of this document published that way and their parity line was true of one side only; revision 3 moves the pointer into the file (see the masthead) so a single `sha256` covers both. **Any design node under this project should follow the same rule**, and the check is one command, not a convention: `cmp` the file against the downloaded node body.

---

## 8. Data Model (Conceptual)

| entity | owner | change |
|---|---|---|
| **Run record** | `internal/loop` | gains one integer: the applied admission ceiling of the initial round. Purely additive |
| **Limits** | `internal/loop` | **unchanged.** It carries constants; the ceiling is not one, and its doc comment stays true |
| **Anchor summary**, **Disposition** | `internal/loop` | unchanged |
| **eval `RowResult`** | `internal/eval` | `budgetBytes` changes meaning. No field added, none removed |
| **eval `Result.Limits`** | `internal/eval` | unchanged — still the sweep's constants |
| **`BuildRow`'s signature** | `internal/eval` | gains the ceiling. The change §3.3(b) shows is unavoidable |

**The datum is a receipt, not a denormalisation.** A denormalised copy can disagree with what it copies; this cannot, because it *is* the value applied, captured where it is applied. §3.4 is the proof rather than the promise: the arithmetic has already moved once, and a receipt would have carried both regimes correctly while every reconstruction rule was wrong on one of them.

---

## 9. Cross-Cutting Concerns

**Backward compatibility.** §7.1's absence row and §7.3. Absent, never wrong; rendered as an admission, never as zero.

**Forward compatibility.** A consumer written later that reads `assemblyByteBudget` and subtracts nothing is still possible. §12 F-1.

**Observability / error handling.** Nothing logged changes; no new failure path. The ceiling is computed from two values already in hand at a site that cannot fail.

**Consistency.** The record is written once at the end of a run; the ceiling and the dispositions cannot disagree.

**Model-facing surface (A5).** One integer near the end of the record; §7.1's position row says why the cost is ~zero and how to check it.

**Comment discipline (#10861 / P-38).** No node ids, no QA coordinates in source. The absence contract belongs in each function's docstring, the module's existing idiom.

---

## 10. The DRY math — and it argues against extraction

Per **#1267** and **#11034 P-2**, as numbers rather than a paraphrase.

**Block size.** One expression in each language: `max(budget − anchor_size, 0)`. `summaryRemainingAfterAnchor`'s five lines are its longhand.

**Site count at `860c0e7`, counting the whole block.** Sites that *state* the derivation: **three** — `loop.Assemble`, `loop.summaryRemainingAfterAnchor`, `step_trace.py`. `compare.py` and `smoke.py` do not contain it and `eval/report.go` cannot; counting omissions as duplication would inflate the figure.

> **`block_size × site_count = 1 × 3 = 3`**, against #1267 / P-2's threshold of **~15–20**. An order of magnitude below.

**P-2's site clause fails in every partition too** — it extracts a block *over 5 lines* at *more than 2 sites*:

| partition | block | sites | verdict |
|---|---|---|---|
| within Go | 1 | 2 | 1 is not over 5; 2 is not more than 2. **Gates out twice** |
| within Python | 1 | 1 | nothing to extract |
| across the boundary | 1 | 3 | no import exists; extraction is not an available mechanism at any threshold |
| *the absence handling (revision 2)* | ~2 | 3 | 2 is not over 5. **This is what deletes the shared accessor** (§7.3) |

**Stated plainly: DRY justifies none of this work, and no part of this design is a DRY remedy.** If the question were *"is one line written down three times too many times"*, the answer is **no — leave it**, and the document would end here.

**What justifies it is #1136 §5 / #868** — *a new persisted data point names the concrete decision it enables* — resting on the structural fact in §3.3(b): **`BuildRow` does not receive the anchor, so the correct value is not computable from the inputs that site is given.** No fixture and no guard reaches a signature. That justifies the design at a defect rate of **one in five**, and §11's trade-off states the same thing from the other end: *if site 2 did not exist, alternative A would win on YAGNI.*

§3.4 then adds the second, independent justification: **the reconstruction rule is not stable over the corpus**, so no static denominator is right about every stored record and only a recorded per-run value can be.

*Revision 1 offered instead: ~~a defect rate, not a line count: five restatements, of which four were or are wrong (80%)~~. **Struck.** A rate is not load-bearing — *N of M copies were wrong* would justify arbitrary work if the rate were the criterion — and **P-3 was the wrong hook**, since P-3 governs guards and the primary move here is a persisted datum. The number was decoration on an argument that did not need it, and foregrounding it made the design look weaker than it is.*

---

## 11. Alternatives considered and rejected

| # | alternative | why rejected |
|---|---|---|
| **A** | **Leave the copies; add guards only.** The #13428 drain-grace shape, and the shape the source question invited. | **The strongest alternative, and it cannot reach site 2 — for a signature reason.** `BuildRow` is not given the anchor, so the correct value is not computable from its inputs; no guard over `report.go`'s text and no fixture in `internal/eval` can supply an operand a signature does not accept. A data change is required there regardless. And §3.4 adds a second defeat: guarding a reconstruction rule guards a rule that was **wrong for 2 of the 18 records** before anyone wrote it. A's behavioural-guard half is **adopted wholesale** in §7.4; what is rejected is the half that keeps five parties reconstructing a number the producer already had. |
| **B** | **Extract a shared module per language; no wire change.** | Rejected outright in revision 2 — revision 1 adopted a fragment of it as a legacy path, and §3.4 deleted that. It leaves site 2 unfixed for A's reason, §10's math says the extraction is not owed, and after §7.3 there is nothing left to extract. |
| **C** | **Publish the ceiling from the binary** — a `/health` field or a `--print-limits` flag the scripts probe. | Moot: the run record already *is* the published surface, all three scripts read it, and the eval result is the same for `cmd/eval`. A second channel for a number the existing one is built to carry is #1136 §2 Form 2. |
| **D** | **A Go test reading `scripts/*.py` as text** — the `script_grace_mirror_test.go` shape — asserting no script compares a size against a raw budget. | Right instinct, wrong defect. That guard keys on a **declaration** with a stated line shape. Here there is no declaration; the property is *which variable is on the right of a comparison*, which is dataflow, and a regex approximating it fires on compliant code that names its local differently — **#1220 §9 / P-29**, the failure QA measured on the grace guard's own first round. |
| **E** | **Remove `anchor.size` / `assemblyByteBudget` from the record** so the wrong claim has no inputs. | Breaks correct statements to prevent incorrect ones. The narrow form *is* adopted, at the one site where the raw value has a single reader whose only correct value is the ceiling — §7.2. |
| **F** | **Publish a supplementary-round ceiling too**, for symmetry. | `dispatchRecall` passes `SupplementaryByteBudget` raw; no derivation, no defect. #1136 §2 Form 3, and §4's can-it-be-deleted: nothing breaks if absent. |
| **G** | **Key the scripts' legacy rendering to `e307a24`'s date** so pre-charge records still get a number. | Couples every renderer to a commit date, in scripts that read a record carrying no timestamp. A worse coupling than the one being removed, for a number nobody can verify. §16 D-2. |

**Trade-off, stated plainly.** A is cheaper and does less. The difference is one site A cannot reach for a signature reason, plus §3.4's finding that A would be guarding a rule that is wrong on part of the corpus. If site 2 did not exist and the arithmetic had never moved, A would win on YAGNI.

---

## 12. Falsifiers

**F-1 — nothing here forces a *future* renderer to read the ceiling.** `limits.assemblyByteBudget` stays on the record for good reasons (§2), so a sixth renderer can still take it and subtract nothing. The claim is that publication makes the correct path the *easiest* path and §7.2 removes the wrong number at the one site where that is possible — not that correctness is forced. **Falsified if** a renderer written after this change reproduces the defect — **and the detector is §3.1's pass H**, which finds any new reader of `budgetBytes` by name, so this is checkable by one command at any later date rather than by someone happening to notice. That would show publication is insufficient and alternative D must be solved rather than rejected.

**F-2 — the guard set is behavioural and per-component, so it grows with components and misses one nobody tested.** `smoke.py` has no test file today, which is how it acquired the defect. **Falsified if** a component ships without its §7.4 row and is later found wrong. P-41's residue check reads what is *cited* and cannot ask what is *missing* — #11034 says so about itself.

**~~F-3 — the two legacy derivations are coupled by nothing, deliberately. §7.3 argues the domain is frozen and shrinking so a guard is decoration. Falsified if the admission arithmetic changes in a way that makes the legacy Python path wrong for records it still serves — i.e. if a record written before the field can also be a record the new arithmetic applied to. That cannot happen for a change shipped after this one, which is why the argument holds; it fails if a change is ever backdated, which nothing in this repo can do.~~**

> **F-3 was FALSIFIED at `860c0e7`, and by the condition it declared unreachable.** It looked only *forward*, and reasoned that the pre-field domain was homogeneous. **The arithmetic had already changed once, on 2026-09-05, inside that domain** (§3.4), so a record written before the field can indeed be a record a *different* arithmetic applied to — not by backdating, but because "pre-field" and "pre-charge" are different lines and nobody looked for the second one. This is **#1225's 2026-09-10 absence rider** in its exact form: a negative claim about the tree (*"that cannot happen"*), written from reasoning rather than from a one-line command, presented as background. And **P-51**: the *because* clause carried neither a measurement nor a hedge. The remedy is not a better F-3 — the legacy derivation it defended is deleted, so there is nothing left to falsify. **Retained struck because a falsifier that was falsified is the most instructive row in this document.**

**F-4 — the ceiling is recorded, so a bug that records the wrong ceiling is invisible to every consumer at once.** Today's copies fail independently, which is how three instruments found three defects. A single source fails together. Mitigated by §7.4's `Assemble` and `cmd/eval` rows. **Falsified if** either guard can be satisfied by an implementation that surfaces a value it did not apply; P-27's sensitivity witness is the check.

**F-5 — no CI (A3), so every guard reaches whoever runs the suite.** Strictly stronger than today, where four defects were invisible to every party at every stage; still a human gate. **Falsified if** a defect of this class ships past a round in which the suite was run.

**F-6 — the Python guards land in a gate the README does not name.** §16 D-4 addresses it with documentation and no infrastructure. **Falsified if** the Python suite goes red and stays red across a merge — which would show that naming a gate in prose is not gating.

**F-7 *(new)* — "not knowable" is the honest answer and it may be the useless one.** Every one of the 18 stored records renders no ceiling after this change, so the scripts lose a figure for the entire present corpus. The claim is that a correct absence beats a number that is wrong on 2 of 18 and unfalsifiable on the other 16. **Falsified if** an operator needs the historical figure often enough that the blank drives someone back to deriving it — in which case the right answer is not to restore the derivation but to record the split as data (§16 D-2 names that path and why it is not taken now). **The detector is §3.1's pass C**: returning to deriving means a derivation reappears *as code*, and pass C is keyed on exactly that shape. **This is the whole difference from F-3** — F-3 depended on nobody having looked; these two depend on a command anyone can run.

---

## 13. Risks & Mitigations

| risk | mitigation |
|---|---|
| The meaning change on `eval`'s `budgetBytes` is silent to a reader of that JSON | The artifact is a stdout stream regenerated per sweep and nothing in this repo stores one (§7.2's risk row). §16 D-5 states the residual honestly and it is smaller than revision 1 claimed |
| `BuildRow` needs a new parameter, which touches every construction site | It is the change §3.3(b) shows is unavoidable. Two live assertions (§7.4) redden immediately, which is the desired behaviour, not a hazard |
| A single source fails everywhere at once | F-4, and its two guard rows |
| The 18 stored records all render "not knowable" | F-7 and §16 D-2. Accepted deliberately: the alternative prints a number measured to be wrong on #10897 |
| PR #60 lands first and adds a sixth statement of the derivation | §16 D-1. Two lines, deleted by a unit already touching that function |
| The m2 edit is a two-sided artifact (file + node) | §7.5's P-40 paragraph names one author for both halves |

*Revision 1 carried a risk row about the Python accessor's placement pre-empting #11326's extraction shape. **Deleted** — §7.3 introduces no module, so the risk does not exist.*

---

## 14. Implementation Guidance for the Next Agent

**No code appears here.** Ordered by dependency; **one independently-meaningful unit per PR** (P-46).

**Unit 1 — the producer publishes.** `internal/loop`: have `Assemble` surface the ceiling it applied; record it on the run record per §7.1; **delete `summaryRemainingAfterAnchor` outright** — no fallback branch, per A6 — and have the summary read the field. Guards: §7.4's `loop.Assemble` row, plus the two existing summary cases re-pointed from the derivation to the field. **P-44:** grep one hop out from every deleted symbol.

**Unit 2 — the eval site.** `cmd/eval` threads the applied ceiling into `BuildRow` (**a signature change**); `internal/eval` changes `budgetBytes`'s meaning per §7.2. **Two live assertions must be inverted, not merely updated** — `internal/eval/result_test.go:123-124` and `internal/eval/report_test.go:157` currently assert the raw constant, and until they are inverted this unit cannot be green. Add the **`cmd/eval`** guard row: that is the only seam where a real anchor exists, and the legibility argument for a bigger anchor lives there and in `result_test.go`, **not** in `report_test.go`, whose fixtures are `RowResult` literals with no anchor. §16 D-6's P-20 question is this unit's to answer. **One more hop:** the five `RowResult` literals in `report_test.go` (`:50, :77, :131, :417, :438`) carry `BudgetBytes: loop.AssemblyByteBudget` as *fixture data*, not assertions — so "invert" does not apply to them, but after §7.2 they silently mean *"a row whose anchor was zero bytes"*, which none of them intends. Re-base them on a real ceiling. Independent of every Python change.

**Unit 3 — the three scripts.** `step_trace.py` reads the field and **deletes** its subtraction **at all three of `:634`, `:113` and `:349`** — the last two are prose, in the module docstring and in `render_candidate_table`'s, and a script's own header is not covered by §2's P-43 exclusion; `compare.py` and `smoke.py` read the field. **No shared module** (§7.3). Each renders absence as "not knowable" in its own idiom, and each gets its §7.4 row plus the absence row. `smoke.py` needs a test file created; whether `print_turn` is tested through captured stdout or gains a returns-lines seam is an implementation call. Depends on Unit 1.

**Unit 4 — m2's field spec (§7.5).** A dated **revision 9** note on `m2-retrieval-eval.md:848` only, in the `required[] → size` row's form, republished to m2's DiVoid node by the same author (P-40). §3.2's prose and table are not touched. Documentation only.

**Unit 5 — the gate (§16 D-4).** README's gate section names the Python suite; **#11034 §1's host run gains the command and its P-6 count is corrected from 8 to 12 in the same edit** — `go list ./...` returns 12 and README already says twelve, so editing §1 without fixing P-6 ratifies a stale gate spec (P-44). Documentation only; depends on nothing.

---

## 15. Pre-Design Checklist audit (#1136 §5)

**KISS / DRY / YAGNI**

- [x] **No new type mirroring an existing one.** One integer added, one field's meaning changed, one signature widened. **No new module in either language** — revision 2 deleted the one revision 1 proposed.
- [x] **No new abstraction with one implementation.** None is introduced.
- [x] **No element justified by "we might need X later".** Alternative F rejected on exactly that ground.
- [x] **No deprecation period / feature flag / compatibility shim.** Revision 1's legacy path was the nearest thing to one and it is deleted — on correctness grounds (§3.4), which is the stronger reason.
- [x] **`block_size × site_count` quoted for every do-not-extract decision.** §10, with the partition table including the absence-handling row that deletes the accessor — **and it argues against extraction, which §10 states plainly.**

**Existing systems first**

- [x] **Audited whether an existing surface covers this.** It does, and that is the design: the run record and the eval result are the existing channels and every consumer already reads them. Alternative C rejected for proposing a second.
- [x] **New layer's concrete reason named.** No new layer.
- [x] **New persisted datum's concrete decision named (#868).** Five renderers' correctness across a corpus whose arithmetic moved — and one of the five cannot compute the value at all. **This is the provision that carries the design** (§10).
- [x] **Consumer chain recursed.** `budgetBytes` → `missLine` → the human report → an operator applying m2 §3.2's rubric (§7.2 shows the rubric inverts). `summaryRemainingAfterAnchor` → the summary → `SetSubstance` → a stored record → a model (#13245).

**Configurability**

- [x] No new knob; nothing here is tunable. `AssemblyByteBudget` untouched.

**Less is better**

- [x] **Can-it-be-deleted / merged / inlined run on every element.** It deleted the shared accessor, the legacy derivation, Go's fallback branch, and one risk row. **The claim that the document therefore shrank is false and is not made.** Every byte of the difference, rev1 → this revision, in rows that **sum to the file delta by construction** — the preamble is a row, and §15 is a row:

| row | rev1 | now | Δ | | row | rev1 | now | Δ |
|---|---:|---:|---:|---|---|---:|---:|---:|
| preamble | 856 | 2848 | +1992 | | §8 | 1282 | 1090 | -192 |
| TL;DR | 2809 | 3329 | +520 | | §9 | 1089 | 887 | -202 |
| §1 | 1340 | 1296 | -44 | | §10 | 2651 | 2715 | +64 |
| §2 | 3344 | 2637 | -707 | | §11 | 3793 | 3208 | -585 |
| §3 | 8479 | 12982 | +4503 | | §12 | 2944 | 4551 | +1607 |
| §4 | 1125 | 1347 | +222 | | §13 | 1193 | 1228 | +35 |
| §5 | 2868 | 2544 | -324 | | §14 | 1993 | 2968 | +975 |
| §6 | 1686 | 1396 | -290 | | §15 | 3889 | 6855 | +2966 |
| §7 | 6523 | 12493 | +5970 | | §16 | 4157 | 3929 | -228 |
| | | | | | **file** | **52021** | **68303** | **+16282** |

**8 rows shrank, 10 grew; the file is +31.3 % on revision 1.** Generator: `_scratch\budgetderiv-k4m9\sectionsize2.py`, which prints the rows and asserts they reconcile to the file delta. The growth is concentrated in three places and each is something that did not exist in revision 1: **§3** carries the anchor-charge measurement, **§7** gained §7.5 entirely, and **§12** retains F-3's struck original under P-43. §7.3 — the section the finding collapsed — lost a whole component and shrank by only ~70 B, which is a smaller saving than "deleted a module" sounds like, because the deletion had to be argued.
  **§15 is now inside its own table, which is the honest way to close the fixed point.** Revision 2 excluded it and disclosed the exclusion — but it also omitted the preamble without saying so, so the printed items summed to +6,309 against a stated total of +7,105, and two prose section-counts disagreed with the list two clauses above them. **A measured claim that fails its own addition, inside the section that audits this document's discipline, is the class this revision exists to remove wearing a new costume.** The fix is not a better disclaimer: it is to count every row, let §15 describe itself, and let the arithmetic be checkable. The residual is bounded and stated — this paragraph is measured on the file as published, so the last edit to it is inside the figure it quotes, which is why the numbers are written to fixed width and re-derived rather than re-typed.
  **And the revision history no longer restates any of these figures — or any other correction.** It was two prose banners until revision 4; each restated, in summary, corrections that its own site already carried dated and struck. A document arguing that a claim carried in two places is a defect cannot carry its own corrections in two places, and the history section is the one a sceptical reader checks the thesis against. It is now an index that points.
- [x] **Trade-offs named.** §11's closing paragraph names the single site that decides it and concedes the cheaper alternative would win without it.
- [x] **Radical-clean over compromise.** §7.2 replaces rather than adds; §7.3 deletes rather than keeps a fallback.
- [x] **Reader inventory covers AST *and* string-literal references — and is now keyed on the datum, not only the derivation.** §3.1 lists **all eight** passes including H. *Revision 1 ticked this box against seven passes of which none was keyed on `budgetBytes`, and listed only four of them.*

**Data deliverables** — none. No SQL, no migration, no backfill. The 18 records are immutable and are read, never rewritten.

**Document discipline**

- [x] Cites #11034 and #1136 as load-bearing — masthead.
- [x] Scope inventories explicit in both directions; out-of-scope listed rather than absent.
- [x] **Supersedes nothing.** #13428 is a sibling design about a different constant and stays current; §11 D names precisely which of its mechanisms does not transfer.
- [x] **P-43:** F-3 and the three struck claims in §3.3 and §10 are retained struck with dated notes; m2 §3.2 is protected from retro-editing by §7.5.
- [x] **P-40, and this document now models the form §7.5 prescribes.** The repo pointer is inside the file (masthead), so the branch file and node **#13522** are `cmp`-equal under a single digest rather than equal-after-stripping. Revisions 1 and 2 published the pointer as a node-only prefix and their parity line was therefore true of one side only.
- [x] **P-41:** every guard name cited resolves in the tree or is marked **new**. Re-run against this revision.
- [x] **P-51:** every *because* clause carries its measurement or its hedge in the same sentence. **F-3 is the recorded instance of failing this**, and #13507's 18/18 figure — the one hedged claim revision 1 leaned on — is no longer load-bearing anywhere (§3.5).

---

## 16. Open questions — taken as decisions

**D-1 — PR #60 (`compare.py`) adds a sixth statement of the derivation that this design deletes.**
**Decision: let it land unchanged.** It closes a live false classification today; Unit 3 replaces its two lines later. Alternatives: *hold it* — leaves a measured false statement in an operator instrument for a multi-unit arc; *widen it* — one-feature-one-PR, and it would put a wire change in a script fix. **Reversal cost: two lines.**

**D-2 — every one of the 18 stored records will render "not knowable".**
**Decision: accept the blank.** The pre-field ceiling is not recoverable from a record (§3.4) and the derivation revision 1 proposed is measurably wrong on #10897 by 628 B. Alternatives by name: *derive anyway* — rejected, measured wrong, and #10898 shows the error is not even detectable; *key the rendering to `e307a24`'s date* (alternative G) — rejected, couples every renderer to a commit; *backfill a ceiling onto the 18 nodes* — rejected, they are immutable records of what happened and a computed field written later is not a receipt. **Reversal cost: low and it improves with time** — the blank shrinks as new records accumulate, and if F-7 fires the right answer is to record the split as data rather than to restore the arithmetic.

**D-3 — the published field's name.**
**Decision: `candidateByteCeiling` as the working name**, with the implementer free to take a better one satisfying three properties: it says *ceiling* or *applied*, not *budget* or *limit*; it does not read as a constant; it distinguishes the initial round from the supplementary rounds. Rejected: `remainingAfterAnchor` — names the *derivation*, the framing being left behind; `assemblyRemaining` — "remaining" reads as live rather than recorded. **Reversal cost: a rename before the first record carries it; a legacy alias after.** Settle in Unit 1's review.

**D-4 — the Python guards land in a gate the README does not name (F-6).**
**Decision: name the existing suite as a gate — README's gate section and #11034 §1's host run — and correct P-6's package count in the same edit.** Verified: `python -m unittest discover -s .` from `scripts/` gives **166 tests, OK**; README's gate section has zero hits for `unittest`, `pytest`, `python -m`; `go list ./...` returns **12**, README says twelve, **P-6 says 8 and names eight packages**. Editing §1 without fixing P-6 ratifies a stale spec (P-44). Alternatives: *do nothing* — knowingly reproduces #13428 alternative D in its green-and-unrun form; *stand up CI first* — that is #13429, blocked on Toni's call. **Reversal cost: a README paragraph.** Honest limit: naming a gate in prose is still a human gate.

**D-5 — external readers of the eval result JSON.**
**Decision: proceed, and record what I did and did not verify.** I verified in-repo that `cmd/eval` writes JSON to stdout, declares no output path, and writes no file — so the artifact is ephemeral and the meaning change is far cheaper than revision 1 claimed. I did not and cannot enumerate consumers outside this repository. **Reversal cost: low** — anyone re-running a sweep gets the new meaning; the only exposure is a figure someone copied into a document by hand. *Revision 1 attached "reversal cost: high if wrong — cannot be un-shipped from an artifact someone has stored" to this field; that describes the **run record** instead, where §7.1 is purely additive and safe. The labels were on the wrong halves and are swapped here and in §7.1/§7.2.*

**D-6 — `internal/eval`'s exactly-`AssemblyByteBudget`-sized fixture.**
**Decision: Unit 2 answers it, as a P-20 question.** Whether that test's *name* claims a property its fixture cannot carry is a question about that test, and the round changing the fixtures around it has the context. Rejected: *decide it here* — I read the fixture and not the assertions it feeds.
