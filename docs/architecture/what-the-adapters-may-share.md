# Architectural Document: What the Adapters May Share

> Repo path: `docs/architecture/what-the-adapters-may-share.md` (canonical copy).
> **DiVoid node: #13345** — it carries this document verbatim under P-40 parity. A parity publish of
> this file goes to **#13345 and no other node**; anything else is the wrong destination.
> Finding settled: **#13337** (*the adapters "cannot drift" claim is narrower than it reads*).
> Concept node this came out of: **#13334** (the model-adapter family). Repo map root: **#10454**.
> Guide amended by §11: **#10466** (*Concept — how to extend Processor*), its *"Adding a model
> provider, or a second protocol"* archetype.
> Project: **#10422**. Predecessor design: **#10532** (M1, the skeleton loop) — **consumed, not
> superseded**; this document adds a rule it did not state and corrects nothing in it.
> Standards applied: Design Contracts **#1136** (§1 KISS/DRY/YAGNI, §5 checklist run in §15), DRY
> threshold **#1267**, Code Contracts **#114 §0**, architect template **#1220** (§5 falsifiable
> universals, §9 name-the-guard).
> Baseline: `main` at **`28d2998`**. **Every fact in §3 was hashed or grepped out of that tree via
> `git show` / `git grep 28d2998`, tracked files only** — never a recursive read of the working
> directory, because `.claude/worktrees/` holds further full checkouts at other commits and a
> recursive sweep reports their union (#9766, #8385). No count of them is given here on purpose: the
> number changes between sessions, and it changed while this document was being written.
> **Three measurement refs appear below and every measurement names its own.** `28d2998` is the base
> the design was written against; **`1b3f8f6`** is the implementation branch tip, and the dated
> corrections in §3.5, §9 and §10 were measured there; **`860c0e7`** is `main` at the 2026-09-10
> revision, and §18's sweep was measured there. Revision records: §17, §18.

---

## TL;DR

**Move one function. Leave sixteen. Add the missing archetype step.**

`renderToolResult` — 20 lines, byte-identical in both adapters, and the branch that renders recall
results is pinned by **no assertion in either package** — moves to `loop.RenderToolResult`, beside
`RenderUserContent`. It renders the loop's own `ToolExchange` in the loop's own `===== SECTION =====`
format and names no protocol token.

The other three functions #13337 names **stay duplicated, as a decision.** `recallTool`,
`writeFileTool` and `wireToolName` declare a capability *to an endpoint*: an envelope, a JSON
parameter schema, a wire spelling, and a description tuned for a model. Extracting them puts
JSON-Schema inside `internal/loop`, deleting the neutrality invariant #10466 step 5 protects and which
is measured clean at `28d2998`.

**The line, stated so the next reader can place a new function without asking:** *does it render the
loop's own data, or declare something to an endpoint?*

**Rejected: a cross-adapter equality test.** It has no referee — equality reports that two things
differ, never which one is wrong — and it would assert 17 of `internal/ollama`'s 50 declarations equal
while the other 33 must not be, with nothing in the tree saying which is which.

**Cost:** one exported function, one new test file in `internal/loop`, ~20 lines net removed. No new
type, no new package, no port change, no signature change outside the two adapters.

---

## 1. Problem Statement

PR #49 extracted the duplicated user-message composer into `loop.RenderUserContent`. Its PR body
claimed the two adapters can no longer drift. **That is true of exactly one function**, and #13337
names four more that are byte-identical in `internal/ollama/wire.go` and
`internal/openaicompat/wire.go`: `recallTool`, `writeFileTool`, `wireToolName`, `renderToolResult`.

#13337 offers three unweighed options — extract, pin against each other, or accept and state — and
asks for one, defended. The operator's framing names the constraint that makes it non-obvious, and it
is quoted here because the document is written to it:

> *"the adapter's job is to speak a wire protocol, and the loop's vocabulary must never leak into it —
> so 'extract to a shared package' has to answer *which* package, and whether a `loop`-level symbol
> describing a **tool schema** is still the loop's vocabulary or has become the wire's.
> `RenderUserContent` composes a *prompt*, which is arguably the loop's business; a JSON
> tool-parameter schema is arguably not. That asymmetry is the actual design question and I want it
> addressed head-on rather than assumed either way."*

And, on the second option:

> *"whether option 2's cross-adapter comparison test is a **guard** or an **instrument that will be
> deleted the first time the adapters legitimately need to differ** … Say where that line sits and how
> a future reader knows which side a new function is on."*

### 1.1 The preventive half

#10466's *"Adding a model provider"* archetype has no step requiring a new adapter to call
`loop.RenderUserContent` rather than compose its own user content. A third provider added by following
the guide correctly recreates the original duplication. **Fixing either half alone leaves the other
live**, so §11 specifies the archetype's amendment in full, in the node's own correction form.

### 1.2 Success criteria

1. One decision, defended, with the two alternatives rejected on grounds a reader can check.
2. A rule that places a *future* function on one side of the line without further design work.
3. Whatever duplication remains is stated as a decision with its arithmetic, not left as drift.
4. A falsifier a reviewer runs as commands, not as judgement.
5. The archetype step's exact wording, in #10466's dated-correction form.

---

## 2. Scope & Non-Scope

### 2.1 In scope

- `internal/ollama` and `internal/openaicompat`: which declarations are shared, which stay local.
- One new exported symbol in `internal/loop` and the two-part guard that pins it.
- The wording of a new step and one correction in #10466.

### 2.2 Out of scope — declined explicitly

- **The implementation.** This document is not a patch.
- **`internal/loop`'s production behaviour.** Nothing about assembly, retrieval, the turn, the ports
  or the record changes. `RenderToolResult` is a relocation of code that already runs, not new
  behaviour: the string it returns for any given `ToolExchange` is byte-for-byte what both adapters
  return today.
- **The tool parameter schemas' own test coverage.** §3.4 records that the schemas' *content* is
  pinned in neither adapter — only that the value parses as a JSON object. That is a coverage gap, not
  a duplication finding, and folding it in here would make one PR carry two features (#1176 PR-scope).
  It is filed as Open Question §13.2 for the operator to raise as its own task.
- **`internal/divoid`, `internal/condense`, `internal/eval`, `cmd/*`.** None is touched.
- **A registry or factory for providers.** #10466 refuses both by name and this document does not
  reopen it.

---

## 3. Measured Facts

Every row below was produced at `28d2998` over **tracked files only**, via `git show 28d2998:<path>`
and `git grep <pattern> 28d2998 -- <path>`. Method for the identity hashes: each declaration's body
was sliced out of the two files by line range and hashed with `sha256sum`; **doc comments above a
declaration are excluded from the slice**, because several of them legitimately differ (each names its
own protocol) while the declaration beneath is identical.

### 3.1 The four functions #13337 names — re-measured, and they hold

| function | `internal/ollama/wire.go` | `internal/openaicompat/wire.go` | sha256 (first 16) |
|---|---|---|---|
| `recallTool` | `61-74` | `58-71` | `024970e0f3e03ca0` |
| `writeFileTool` | `76-92` | `73-89` | `a954ccb99faee5bc` |
| `wireToolName` | `137-142` | `149-154` | `0616cc12d7515243` |
| `renderToolResult` | `153-172` | `165-184` | `3035769d6248e51f` |

Control, to show the instrument discriminates: `toolArguments` sits between them in both files and is
**not** identical — `ded25986f23163b2` against `a81561d7304abbd1`, because ollama sends the tool
arguments as a JSON object and openaicompat sends them as an encoded string.

### 3.2 The duplication is larger than four functions — 17 declarations, 120 lines

The same slice-and-hash pass over all three production files of each package:

| file | identical declarations | lines (per adapter) |
|---|---|---|
| `wire.go` | `wireTool` (4), `wireFunction` (5), `recallToolArguments` (3), `writeFileToolArguments` (4), `recallTool` (14), `writeFileTool` (17), `wireToolName` (6), `renderToolResult` (20) | **73** |
| `client.go` | `DefaultTimeout` (1), the `recallToolName`/`writeFileToolName` const block (4), `var _ loop.ModelPort = (*Client)(nil)` (1), `Client` (7), `NewClient` (12), `defaultHTTPClient` (3), `client()` (6) | **34** |
| `condense.go` | `SentSampling` (7), `CondenseResult` (6) | **13** |
| | **17 declarations** | **120** |

Against a total of **50** top-level declarations in `internal/ollama`'s three production files and
**41** in `internal/openaicompat`'s. So **17 of 50 are byte-identical and 33 are not** — a number §6.2
uses to reject option 2.

**Not all 17 are the same kind of duplication, and that is the whole design.** The seven in
`client.go` are two implementations of a documented archetype — #10466's *injected collaborator* shape
prescribes exactly `NewClient`, `defaultHTTPClient()`, `client()` and a bounded fallback, and two
correct instances of one prescribed shape look alike **by construction**. Extracting them would
abolish the archetype, not the duplication.

### 3.3 What is actually unguarded — the sharpest fact, and #13337 does not have it

#13337 and #13334 both state that a change *"must be made twice and nothing fails if it is made
once."* Measured branch by branch, **that is true of some of it and false of the rest**, and the
difference is what selects the remedy.

| duplicated element | one-sided edit reddens… | measured by |
|---|---|---|
| tool **descriptions** (both) | **that adapter's own test** — each package restates the literal in its `client_test.go` (`ollama:140,142`, `openaicompat:118,120`). The **sibling stays green**, so the divergence is silent even though the edit is not | `git grep "Search memory for something" 28d2998` → 4 hits: 2 production, 2 test |
| wire tool names `recall` / `write_file` | **that adapter's own test** — both names are pinned as literals in both `client_test.go` files | `git grep -c 'write_file' 28d2998` → **15** hits in `internal/ollama` and **7** in `internal/openaicompat`, of which exactly **one per package is production** (the const in `client.go`); every other hit is a test-side literal |
| `renderToolResult` → `"wrote %d bytes to %s"` | **ollama only**, byte-exact at `client_test.go:370`. openaicompat asserts `contains("11")` and `contains("index.html")` — substring, not layout | `write_test.go` |
| `renderToolResult` → `"error: " + r.Error` | **nothing, in either adapter.** The `error: ` prefix is asserted by no test in the module | `git grep '"error: ' 28d2998 -- internal/` → **2 hits, both production** |
| `renderToolResult` → `"no additional results found."` | **nothing, in either adapter** | `git grep "no additional results found" 28d2998` → **2 hits, both production** |
| `renderToolResult` → the `===== RESULT =====` block | **nothing in ollama**; openaicompat asserts only `contains("found body")` and `contains("second found body")` | `git grep "===== RESULT =====" 28d2998` → **2 hits, both production** |

**Read the table as one sentence:** the three functions that declare something to an endpoint are
guarded locally in both adapters; the one function that renders the loop's own data is the one nothing
pins. The finding's remedy and the coverage gap point at the same function, from opposite directions.

### 3.4 The precedent's guard is two tests, not one — and this matters for the design

`RenderUserContent` is pinned by a **pair** whose halves do different jobs:

| test | pins | what mutation reddens it |
|---|---|---|
| `internal/loop/usercontent_test.go` — `TestRenderUserContentOpensWithTheRequestAndKeepsTheTailCopy` (and its sibling) | **the layout**, against `"===== INPUT ====="` written as a literal on the expected side (`:18-20`) | changing the shared function's output |
| `internal/ollama/usercontent_test.go:41` and `internal/openaicompat/usercontent_test.go:41` — `… ByteEqualToRenderUserContentOfTheSameBlockAndInput` | **that the adapter's emitted bytes equal the shared function's output** for the same input, by comparing the captured request body against `loop.RenderUserContent(block, input)` | any adapter-side composition whose bytes **differ** — and **not** a byte-identical one, which is the correction below |

The adapter-side test **cannot** pin the layout — mutating the shared function moves both sides of its
comparison together — and is not meant to. This division is exactly #10466's *"use literals on the
expected side"* rule, applied across a package boundary, and §9 reproduces it for `RenderToolResult`
rather than inventing a new shape.

> **Correction, 2026-09-10 (`860c0e7`) — the second row said the adapter-side test pins the *route*,
> and it does not.** The cell read: *"**that the adapter routed through the shared symbol**"*, with
> *"the adapter composing its own string"* as the mutation that reddens it. **A byte-equality
> assertion observes the output, never the route** — a byte-identical inline re-implementation passes
> it, and Go offers no seam to intercept a direct cross-package call. §9's own correction block
> measured this on the successor guard and says in terms that **this precedent has the identical
> hole**; what it did not do is come back and correct the precedent's own row, which is the row the
> later correction was reasoning from. **Nothing about the division of labour changes** — the
> adapter-side test still cannot pin the layout, the pairing is still the design, and §9 still
> reproduces it correctly. Only the claim about what the adapter-side half *proves* was too strong.
> **This row is not a measurement and carries no ref**; the measurements in §3 stand at `28d2998`
> untouched. See §18.

### 3.5 Neutrality, measured at `28d2998`

| check, over `internal/loop/*.go` — **test files included, deliberately** (§10 F-3) | `28d2998` | `1b3f8f6` |
|---|---|---|
| `git grep -E '"type": *"object"' <ref> -- 'internal/loop/*.go'` | **zero hits** | **zero hits** |
| `git grep 'write_file' <ref> -- 'internal/loop/*.go'` | **zero hits** | **zero hits** |

> **Correction, 2026-09-09 (`1b3f8f6`).** This table's header read *"over `internal/loop/*.go`
> production files only"* and its first check read
> `git grep -E 'json\.RawMessage|"type": *"object"|"properties"|"required"' …`. **Two defects, one of
> them live.** *(a)* The header said **production** and the pathspec did not implement it — that scope
> lived only in the `| grep -v _test` pipe the measurement was taken with and never reached the
> document. *(b)* The pattern's first term is **not schema vocabulary at all**: `json.RawMessage` is
> its sole contributor, at `internal/loop/attribution_test.go:162`, where it is the raw-body
> key-absence assertion **#11034 P-19** mandates. Measured at both refs: `json.RawMessage` alone →
> **1**; the three schema spellings alone → **0**. Retained rather than deleted because **the fix that
> suggests itself is the wrong one**: excluding test files yields the right number by removing the
> check's reach over exactly the files a migrated schema would bring with it. **Dropping the
> non-schema term is the fix; narrowing the pathspec is not.**

Both discriminate, and the second was chosen deliberately: the loop's own record spelling for the
recall tool is `ToolRecall = "recall"`, which collides with the wire spelling, so a grep for `recall`
fires on compliant code. The write tool's two spellings differ (`ToolWriteFile = "writeFile"` in the
loop, `write_file` on the wire), so `write_file` is the token that separates them. **A falsifier that
fires on compliant code is worse than none** (#1220 §9), and this is where that trap sits in this
particular tree.

**One clarification the raw grep does not give you:** `internal/loop/types.go` carries **58** `json:`
struct tags. Those are the **run record's** serialisation to the graph, not any provider's format.
#10466's step 5 falsifier is scoped to *the provider's* vocabulary — field names, finish-reason
strings, error shapes — and a sweep on the bare word `json` would report a false positive on every
one of them.

**Why `"type": "object"` alone carries the check.** Every JSON-Schema object opens with it; both tool
schemas contain it at `28d2998` and at `1b3f8f6`, and #10466's archetype requires it. The two terms
dropped alongside `json.RawMessage` — `"properties"` and `"required"` — detect nothing that this one
misses, and both carry a live false-positive path: they are quoted spellings, so a future
`json:"required"` struct tag on `Record` would match, and `types.go` already carries 58 such tags.
**Strictly less false-positive surface for no loss of detection** is the whole trade.

**An import-graph arm was considered and is measurably unusable — recorded so nobody re-proposes it.**
*"`internal/loop`'s production code links no JSON codec"* would be a fact about the dependency graph
rather than a spelling, which #1220 §9 rates the stronger instrument. It does not survive contact:
`go list -deps ./internal/loop` **contains `encoding/json`**, pulled in transitively by `log/slog`,
which `turn.go` imports for the run logger. No production file in `internal/loop` imports it directly,
so the claim is true and **the command that would check it is not** — it fires on compliant code, for
a reason with nothing to do with tool schemas. That is the defect this whole section is about, and it
was found only by running the command instead of reasoning about it.

**F-3 and F-4 are not #11034 P-36, and must not be read as restating it.** P-36 is the standing
provider-neutrality check and is **production-scoped** by its own terms. F-3 and F-4 ask a narrower,
change-specific question — *was §5.2's decision silently reversed?* — for which test files are **in
scope on purpose**, because a schema that migrated into the loop would bring its test with it. Two
checks, two scopes, deliberately.

### 3.6 The adapters already differ in ways nobody would want removed

Named because §6.2's rejection of option 2 rests on them being *legitimate*, not accidental:

- **ollama** carries an entire native fallback parser — `recoveredCall`, `recoverCall`,
  `recoverParameters`, `trimOneNewline`, `translateRecoveredCall`, `recoverRecall`, `recoverWrite`,
  and the five `<function=` marker constants. openaicompat has none of it.
- **openaicompat** maps `content_filter` → `loop.Refused`; ollama's `mapDoneReason` has no such
  branch, because the native protocol reports no such reason.
- **openaicompat** can fail with *"response has no choices"*; ollama cannot.
- Tool results are addressed by `tool_call_id` on one side and by `tool_name` on the other.
- **A prompt-shaping constant already varies per provider**: `condenseThinking = false` exists in
  `internal/ollama/condense.go` and in no other package, because only the native protocol has a
  `think` key to switch off.

That last one is not decoration. It is the tree's own precedent that **per-provider variation of what
is sent to a model is a legitimate, already-instanced decision** — which §5.2 leans on when it keeps
the tool descriptions local.

---

## 4. The Decision Rule

> **Does it render the loop's own data, or declare something to an endpoint?**
>
> **Renders the loop's own data** → it belongs in `internal/loop`, as an exported function, pinned by
> the two-test pair of §3.4.
> **Declares something to an endpoint** → it belongs in the adapter, duplicated across adapters
> without apology, pinned by that adapter's own wire tests.

### 4.1 Why this rule and not "prose versus JSON"

"Prose versus JSON" is the tempting formulation and it breaks on the first case. A tool
**description** is prose, and it belongs to the adapter. A JSON parameter **schema** is JSON, and if
prose-versus-JSON were the rule, one would extract the description and leave the schema — splitting a
single declaration down the middle and producing the worst available outcome: a `wireFunction` literal
whose `Name` and `Parameters` are local and whose `Description` is imported from another package, for
no gain a reader could name.

The rule above places every existing member without a special case:

| element | renders the loop's data? | declares to an endpoint? | side |
|---|---|---|---|
| `RenderUserContent` | yes — `Block` + `Input` | no | **loop** (already there) |
| `renderBlock` | yes — `Anchor` + admitted `Candidate`s | no | **loop** (already there, unexported) |
| `renderToolResult` | yes — a `loop.ToolExchange` | no | **loop** (this document moves it) |
| `recallTool` / `writeFileTool` | no | yes — envelope, name, schema, description | **adapter** |
| `wireToolName` | no | yes — the endpoint's spelling of a tool | **adapter** |
| `chatRequest`, `translate`, `mapDoneReason`, … | no | yes | **adapter** |

### 4.2 The asymmetry, addressed head-on

The operator's question was whether a `loop`-level symbol describing a tool schema is still the loop's
vocabulary. **It is not, and the reason is not aesthetic.**

`RenderUserContent` takes two of the loop's own strings and returns one string. Its output has exactly
one consumer characteristic: **a model reads it.** Nothing about any protocol constrains its shape,
and a third protocol changes nothing about it.

A tool declaration is four things bound into one literal, and **every one of them is per-protocol**:

1. **The envelope.** `{"type":"function","function":{…}}` is not a universal. Ollama adopted OpenAI's
   shape; it is a fact about two protocols, not a requirement. The evidence that it is coincidence
   rather than convergence is in the same two files: `chatRequest`, `wireMessage`, `wireToolCall` and
   `wireFunctionCall` all sit beside `wireTool` and all four **differ**.
2. **The parameter schema.** JSON-Schema is the wire's type language. It is precisely the vocabulary
   #10466 step 5's falsifier exists to keep out of `internal/loop`, and §3.5 measures that the
   falsifier is clean today.
3. **The wire name.** `write_file` is the endpoint's spelling; the loop's is `writeFile`. A shared
   symbol here would have to pick one, and picking the wire's puts a protocol token in the loop.
4. **The description.** It is a prompt, and prompts are tuned per model — a fact this repo already
   acts on (§3.6, `condenseThinking`) and staffs a role for.

So the answer is: **a tool schema has become the wire's.** And the tool result has not — it is the
loop's record of what already happened, formatted in the loop's own `===== SECTION =====` family,
whose two other members already live in `internal/loop`.

### 4.3 The objection to this rule, and the answer

*If a description can legitimately vary per provider because prompts are tuned per model, why can a
tool **result** not vary per provider for the same reason?*

Because they are different kinds of text. A description is an **instruction** — it tells a model what
a capability does, and phrasing it differently for a different model is prompt engineering. A tool
result is **data the run already recorded**, rendered. Two providers given differently-rendered
recall results are no longer being asked the same question, and the eval corpus that compares
providers (`cmd/eval`, #10466's *adding a command* archetype) would be measuring the rendering rather
than the model. **A per-provider instruction is a tuning decision; a per-provider rendering of the
same data is a measurement defect.**

That is the falsifier for the rule itself: **if a case ever arises where one provider genuinely needs
a different rendering of the same `ToolExchange`, this rule is wrong and should be revisited rather
than worked around.** None exists at `28d2998`; the two renderings are byte-identical.

---

## 5. The Recommendation

### 5.1 Move `renderToolResult` to `loop.RenderToolResult`

- **Where:** `internal/loop/assemble.go`, immediately after `RenderUserContent`. That file already
  holds `Assemble`, `renderBlock` and `RenderUserContent` — everything composed for the model to read.
  A new `render.go` was considered and rejected: a new file for one function, when the coherent home
  already exists (#1136 §4, *can it be merged with something existing*).
- **Signature, in prose:** it takes one completed tool round in the loop's own vocabulary and returns
  the text a model should see for it. No context, no error return, no options.
- **Behaviour:** byte-for-byte what both adapters return today. This is a relocation, not a redesign.
  The four branches — a recorded error, a write receipt, an empty recall, and the `===== RESULT =====`
  block per candidate — keep their exact current text.
- **Imports:** none new for `internal/loop`; `fmt` and `strings` are already imported by
  `assemble.go`.
- **Both adapters** delete their copy and call the shared one from `buildMessages`. Nothing else in
  either package changes; no exported surface, no port, no signature outside these two files.

**DRY arithmetic (#1267):** `block_size × site_count = 20 × 2 = 40`, against the ~15–20 threshold.
Above it, so the extraction is what the contract's own math requires, not a preference. The
named-helper test passes in two words: **render tool result**.

### 5.2 Leave the rest, as a decision

`recallTool`, `writeFileTool`, `wireToolName`, the two tool-name constants, `wireTool`, `wireFunction`,
`recallToolArguments`, `writeFileToolArguments`, and the seven archetype-shaped declarations in
`client.go` and the two in `condense.go` **stay duplicated.** §7 states the scope and grounds the
override.

### 5.3 Amend #10466's archetype

§11 gives the exact wording: one new step **4a**, one dated correction on step 5, and a *"Find me by"*
extension. This half is not optional — #13337's whole point is that either half alone leaves the other
live.

---

## 6. Alternatives Rejected

### 6.1 Extract all four (#13337's option 1, taken whole)

**Rejected**, and the reason is measurable rather than stylistic. Extracting `recallTool` and
`writeFileTool` requires moving `wireTool`, `wireFunction` and two JSON-Schema literals into
`internal/loop` — **four declarations, 34 lines, all of them the wire's type language.** That

- deletes the invariant #10466 step 5 protects, which §3.5 measures clean at `28d2998`, and does so
  *silently*: the step-5 falsifier would begin returning hits, and a reader who finds the guide
  telling them to run a check that now fails by design will conclude the check is broken;
- puts the module in the position that the **third** protocol either adopts an envelope it does not
  use, or `internal/loop` grows a second envelope shape — at which point the loop is carrying two
  vendors' formats with an interface in front of them, which is the exact outcome #10466 step 4 names
  as the thing the whole arrangement exists to prevent;
- buys nothing the guard does not already give: §3.3 measures that a one-sided edit to a description
  or a wire name **already reddens that adapter's own test**.

This is the math-grounded override #1136 §6 demands rather than a paraphrase: the DRY arithmetic does
point at extraction (`17 × 2 = 34` for `writeFileTool` alone), and it is overridden by a named,
checkable cost — the deletion of a standing measured invariant plus a structural commitment against a
protocol nobody has written yet.

### 6.2 Pin the two adapters against each other (#13337's option 2)

**Rejected. It is an instrument, not a guard**, and it fails on three independent grounds.

**(a) It has no referee.** This is the structural objection and it generalises past this case. The
`RenderUserContent` pin works because there is a **third thing** — the shared symbol — that both sides
are compared *against*: `adapter_A == shared` and `adapter_B == shared` tells you *which* side is
wrong. `adapter_A == adapter_B` tells you only that they differ, and leaves the next reader to decide
which one to change. A test whose failure message cannot say what is wrong is a detector, and a
detector without a definition behind it gets resolved in whichever direction is cheaper that day.

**(b) It encodes a line nobody can locate from the code.** §3.2 measures 17 of `internal/ollama`'s 50
declarations byte-identical and 33 not. A byte-equality pin over four of them is a claim that *these
four must stay equal, those thirty-three must not, and the remaining thirteen identical ones are
neither pinned nor forbidden* — a three-way partition with no principle behind it. The first
legitimate divergence collides with it: OpenAI's `strict` schema flag, a per-model description tweak,
or a protocol whose tool name is not `write_file`. Then the test is red on **correct** code, and
#1220's §9 addendum names the consequence exactly: *a reader who runs it, sees the hit, checks the
code, and finds the code correct learns to disregard the whole column.* It gets amended-to-fit once
and deleted the second time.

**(c) Its claimed unique advantage does not survive inspection.** #13337 credits it as *"the only
option that also covers future duplication without moving code."* Both readings fail:

- **Named-function equality** covers exactly the functions it names. It covers no future duplication
  at all — the next duplicated function is simply not in it.
- **A general cross-package duplicate detector** would cover future duplication, and it fires on
  **all 17** identical declarations including the seven in `client.go` that #10466's injected-collaborator
  archetype *prescribes*. It would flag two correct instances of a documented shape as a defect on
  every run. That is not a test, it is a lint, and it fires on compliant code by construction.

**Where the line sits, and how a future reader knows which side a new function is on:** §4's one
question. It is answerable by reading the function's inputs — if they are `loop.` types and its output
is text a model reads, it is the loop's; if it names a protocol token, an envelope, a schema or a wire
spelling, it is the adapter's. §11's step 4a puts that question in the guide, so the reader meets it
while writing the adapter rather than after.

### 6.3 Accept all the duplication and state it (#13337's option 3)

**Rejected for `renderToolResult`, adopted for everything else.** §7 is option 3, applied to the
sixteen declarations where it is right.

It is wrong for `renderToolResult` on the evidence of §3.3: that function's recall branch, its
empty-results sentence and its `error:` prefix are pinned by **nothing in either package**, so
"accept and state" here means accepting a 20-line block that is duplicated *and* unguarded *and*
unambiguously the loop's. And #10466's own precedent cuts against it — its 2026-09-03 correction on
the injected-collaborator archetype: *"partial application of a settled shape is worse than none,
since a reader meeting the unguarded one has active evidence that the constructor discipline was
thought sufficient."* Prompt composition in `internal/loop`, pinned from each side, **is** a settled
shape here. `renderToolResult` is that shape, unapplied.

### 6.4 A new shared package for the tool declarations

**Rejected on sight.** A package holding two string constants and an envelope type, imported by
exactly two packages, is #1136 §2 form 3 — a pure restatement — and fails §4's *can it be deleted*
check outright. It also relocates rather than resolves the question of §4.2: a third protocol with a
different envelope makes the shared package wrong in the same way `internal/loop` would have been,
with an extra import path to justify.

---

## 7. What Stays Duplicated — Stated as a Decision

After §5.1, **16 declarations totalling 100 lines per adapter remain byte-identical**, in three groups.
Each is a decision with its own reason.

| group | declarations | lines | why it stays |
|---|---|---|---|
| **The tool declaration** | `recallTool` (14), `writeFileTool` (17), `wireToolName` (6), the `recallToolName`/`writeFileToolName` const block (4), `wireTool` (4), `wireFunction` (5), `recallToolArguments` (3), `writeFileToolArguments` (4) | **57** | §4.2 — all four constituents are per-protocol. Extracting requires JSON-Schema in `internal/loop`. Guarded locally: §3.3 measures that a one-sided edit to a description or a wire name reddens that adapter's own `client_test.go` |
| **The archetype-shaped client** | `DefaultTimeout` (1), `var _ loop.ModelPort` (1), `Client` (7), `NewClient` (12), `defaultHTTPClient` (3), `client()` (6) | **30** | Two correct instances of #10466's *injected collaborator* archetype. They are identical **because the archetype prescribes them**, and a third adapter will make it three. Extracting a prescribed shape abolishes the prescription |
| **The condensation result vocabulary** | `SentSampling` (7), `CondenseResult` (6) | **13** | These are each adapter's side of `condense.ModelPort`'s result vocabulary. `internal/condense` is where a shared form would have to live, and that is a different port, a different design and a different PR |

**DRY arithmetic for the override (#1267, #1136 §6).** The largest surviving block is `writeFileTool`
at `17 × 2 = 34`, above the ~15–20 threshold, so the contract's math points at extraction and is
overridden. The override is grounded in §6.1's named costs — the deletion of a measured invariant and
a structural commitment against an unwritten protocol — not in a paraphrase. The `client.go` group is
not an override at all: `12 × 2 = 24` for `NewClient`, and the arithmetic does not apply to two
instances of a shape a guide prescribes, any more than it applies to two types both having a
constructor.

**What this means in practice, said plainly so nobody re-discovers it as a finding:** a change to a
tool description, a parameter schema or a wire tool name **must still be made twice.** It will redden
the edited adapter's own test, so it will not be silent; it will not redden the sibling, so **the test
suite does not detect the divergence.** That is accepted.

**But *"the suite does not detect it"* is not *"nothing detects it"*, and the gap between those two
sentences is the entire residual risk.** §10's **F-5** catches a one-sided description change at
review time, and **G-5** reddens on the edited side. What is missing is not a detector but an
*automatic* one: F-5 is a command a reviewer runs, and nothing runs it unprompted. **State a gap of
this kind as manual-versus-automatic, never as present-versus-absent** — the second reads as an open
hole and sends the next reader to build a guard that already exists. The third adapter makes it three
sites, and §11's step 4a is what stops the *loop's* vocabulary joining them.

---

## 8. Contracts & Interfaces (Abstract)

### 8.1 `loop.RenderToolResult`

| | |
|---|---|
| **Input** | one completed tool round in the loop's vocabulary — the tool's loop-side name, an optional recorded error, a path and byte count for a write, and the recalled candidates for a recall |
| **Output** | one string: the text a model should be shown for that round |
| **Semantics** | a recorded error wins over everything else and is rendered with its prefix; otherwise a write round renders its receipt; otherwise an empty recall renders its sentence; otherwise each candidate renders as a delimited section carrying id, type, name and body, sections separated by a single newline |
| **Invariants** | total — no input produces an error or a panic. Deterministic — same input, same bytes, no clock, no map iteration, no randomness. Pure — reads nothing outside its argument and writes nothing |
| **Not its business** | which protocol will carry the string, how it is escaped on the wire, which message field it lands in, and whether the endpoint pairs it by id or by name. All four are the caller's |

### 8.2 What each adapter owes it

Each adapter calls it once per prior tool round while building its request, and places the returned
string in whatever field its protocol uses for a tool result. **An adapter that composes that text
itself is in breach.** §9's G-2 and G-3 detect that breach **only when the composed bytes differ**;
a byte-identical re-implementation is a breach no assertion in this language can see, and §10's F-1
is what catches that one — structurally, and at review time rather than automatically. See §18.

### 8.3 Unchanged

`loop.ModelPort`, `loop.GraphPort`, `loop.FilePort`, `loop.JudgeInput`, `loop.JudgeResult`,
`loop.ToolExchange`, both adapters' exported surfaces, both binaries' protocol switches, and the
error envelope. **No port, no signature, no wire byte changes.**

---

## 9. Coverage — the Guards, Named

Reproduces §3.4's pair. Each row names a **test**, not a mechanism (#1220 §9).

| # | Property | Guard | Why it discriminates |
|---|---|---|---|
| G-1 | The rendered layout is what the design says | `internal/loop/toolresult_test.go`, five tests, one per branch: `TestRenderToolResultRendersARecordedErrorBehindItsPrefix`, `TestRenderToolResultRendersAWriteRoundAsAReceiptNamingTheByteCountAndThePath`, `TestRenderToolResultRendersARecallThatFoundNothingAsOneSentenceRatherThanAnEmptyString`, `TestRenderToolResultRendersOneRecalledCandidateAsADelimitedSectionCarryingItsIdentityAndBody`, `TestRenderToolResultPutsExactlyOneNewlineBetweenTwoRecalledCandidatesSections` — each with the expected text as a **literal** on the expected side | a literal expectation cannot move with the production function, so any change to the layout reddens it. This is the half neither adapter can hold |
| G-2 | `internal/ollama`'s **emitted** tool result equals the shared function's output for the same exchange | `internal/ollama/usercontent_test.go` — `TestJudgeSendsANativeToolResultByteEqualToRenderToolResultOfTheSameExchange` | it reddens on any adapter-side composition whose bytes **differ**, and on any change to the shared function that the adapter did not follow. It does **not** observe the *route*, and it cannot pin the layout — both sides move together. See the limit below, which is measured |
| G-3 | `internal/openaicompat`'s **emitted** tool result equals the shared function's output for the same exchange | `internal/openaicompat/usercontent_test.go` — `TestJudgeSendsAToolResultByteEqualToRenderToolResultOfTheSameExchange` | as G-2 |
| G-4 | The wire placement is still each adapter's own | the existing `TestJudgeReplaysPriorToolRoundsAsAssistantToolCallsAndNamedToolResults` (ollama) and `TestJudgeReconstructsPriorRecallRoundsAsAssistantAndToolMessages` / `TestJudgeReplaysAPriorWriteRoundAsTheWriteToolAndItsReceipt` (openaicompat), unchanged | they assert `tool_name` versus `tool_call_id` pairing, which is the wire's business and stays local. Named here so nobody deletes them as redundant with G-2/G-3 |
| G-5 | The tool declarations are still guarded locally | the existing `TestJudgeNativeRequestCarriesModelSystemBlockInputAndBothTools` (ollama) and `TestJudgeRequestBodyCarriesModelSystemBlockInputAndBothTools` (openaicompat), unchanged | each restates both descriptions and both wire names as literals, so a one-sided edit reddens that adapter. This is the guard §7 relies on when it leaves them duplicated |

> **Correction, 2026-09-09 (`1b3f8f6`) — this block said the guards were unmeasured, and prescribed a
> mutation that cannot work.** It read: *"No runnable falsifier is established for G-1, G-2 or G-3…
> for **G-2** and **G-3**, replace the adapter's call with an inline `fmt.Sprintf` producing the same
> text."* The first half is merely superseded — the tests exist at `1b3f8f6` and QA exercised them.
> **The second half was wrong when written**, and it is retained because the way it is wrong is the
> lesson: *producing the same text* **cannot redden a byte-equality assertion**, by construction. A
> reader following it literally observes green and concludes the guard is dead. **A prescribed
> mutation that cannot produce its predicted result is worse than none** — it manufactures a false
> negative in the hands of whoever trusts it.
>
> **The corrected mutations.** For **G-1**, change one character of the empty-recall sentence in
> `loop.RenderToolResult`. For **G-2** and **G-3**, replace the adapter's call with an inline
> re-composition producing **different** text — which must redden that adapter's row and **must not**
> redden G-1, the pair of observations being what proves the two rows measure different things.
>
> **The limit these guards have, stated so it stops being rediscovered.** A byte-equality pin observes
> the **output**, never the **route**, and Go offers no seam to intercept a direct cross-package call.
> QA measured it: re-adding the deleted function to `internal/ollama/wire.go` byte-identical and
> routing to it is gofmt-clean and leaves the guard set green (her M8, across the seven guards her
> harness runs). **The `RenderUserContent` pin this is modelled on has the identical hole**, so PR
> #49's parity claim is weaker than its body read — a second, smaller instance of the defect #13337 is
> about.
>
> **The clone is contained, and by two things that were measured — so this is a stated limit, not an
> open hole.** *(1)* It survives only while it stays **behaviourally indistinguishable**: QA edited
> `loop.RenderToolResult` by one character and **G-2 went red**, because the clone did not follow,
> while G-3 stayed green because openaicompat did. A clone is therefore a **maintenance** hazard, not
> a **divergence** hazard. *(2)* **§10's F-1 catches it structurally** — 1 hit on her M8 tree, 0 on the
> delivered tree. The honest residual is that F-1 is a command a reviewer runs and nothing runs it
> unprompted: **manual, not absent.** Closing that would mean detecting a call rather than an output,
> which no assertion in this language can do; it is not filed as a task because there is nothing to
> build.
>
> G-4 and G-5 predate this change and are cited, not proposed. **Every test name in this table was
> resolved against `1b3f8f6`** by `git grep '^func Test' 1b3f8f6 -- 'internal/loop/toolresult_test.go'
> 'internal/ollama/*_test.go' 'internal/openaicompat/*_test.go'` (#11034 P-41).

**Topology check (#1220 §9, revision-3 rule).** Every row's guard sits in a package whose dependency
closure reaches the code it tests: G-1's test is in `internal/loop` and calls the function directly;
G-2 to G-5 are in the adapter packages, which import `internal/loop`. No row is a wish for reasons of
topology.

---

## 10. The Falsifier — what a reviewer runs

Mechanical. Every command is scoped to tracked files at a named ref, because a recursive sweep from
the repo root reads every full checkout under `.claude/worktrees/` and reports the union — and the
failure of that mistake is always a false **clean**, since a stale copy can only ever *add* hits.
#8385's *Trap 2026-09-09* is the record, and it names the three passes it silently defeats, the
reference-resolution pass **P-41** among them; #9766 is the worktree convention that puts the
checkouts inside the repo in the first place.

Let `<ref>` be the branch tip under review.

| # | Command | Expected | Baseline at `28d2998` | What a wrong result means |
|---|---|---|---|---|
| **F-1** | `git grep -n "no additional results found" <ref> -- internal/ollama internal/openaicompat` | **zero hits** | 2 hits (one per adapter) | the copies were not deleted — **or a byte-identical clone was re-added**, which §9's measured limit shows no assertion can see and this catches structurally (1 hit on QA's M8 tree, 0 on the delivered tree). **Test files are in scope on purpose:** after the move neither adapter has any reason to name the layout, and an adapter test that does is itself a finding, because pinning the layout is G-1's job at the loop level |
| **F-2** | run it **once per package**: `git grep -c "RenderToolResult" <ref> -- internal/ollama`, then again for `internal/openaicompat` | **≥1 hit in that package's `wire.go` and ≥1 in its test file**, per package | 0 and 0 | an adapter is not naming the shared symbol. **The aggregated form was the defect:** the expectation is per-package and one invocation over both emits a joint list a reader must partition by hand, which is the same shape as F-5's. Honestly a spelling check, which is what a grep is for; the behavioural version is G-2/G-3 |
| **F-3** | `git grep -nE '"type": *"object"' <ref> -- 'internal/loop/*.go'` | **zero hits** | zero | the parameter schema migrated into `internal/loop` — §5.2's decision was reversed without saying so. **Tests are in scope on purpose**, and this is **not** #11034 P-36, which is production-scoped and asks a different question. §3.5's correction records the two terms dropped from this pattern, why the pathspec is *not* the fix, and why the import-graph form was rejected on measurement |
| **F-4** | `git grep -n "write_file" <ref> -- 'internal/loop/*.go'` | **zero hits** | zero | a wire tool spelling entered the loop. Chosen over `recall`, which collides with the loop's own `ToolRecall` value and would fire on compliant code. **Tests are in scope on purpose:** the loop's own spelling is `writeFile`, so a loop test naming `write_file` is itself the finding. This row carried F-3's stated-scope defect too — the header said production and the pathspec never did — and it was latent rather than live only because no loop test happens to name the wire spelling |
| **F-5** | `git grep -c "Search memory for something" <ref> -- internal/ollama internal/openaicompat` | **exactly four files, one hit each**: both `wire.go` and both `client_test.go` | those four files | the descriptions were extracted after all, or one adapter's literal test pin was deleted. **`-c` rather than `-n` on purpose:** the previous form emitted four lines and asserted a two-production/two-test split *in prose* — a partition drawn over a count, which a change moving one literal into a test helper falsifies silently while the total stays 4. The per-file breakdown must be the command's **output**, not the row's claim |
| **F-6** | `git diff --stat 28d2998..<ref>` | touches only `internal/loop/assemble.go`, a new `internal/loop/toolresult_test.go`, the two `wire.go` files, the two adapter test files, and `docs/architecture/what-the-adapters-may-share.md` | — | scope crept |
| **F-7** | `divoid_get_content(id=10466)` contains the step **4a** text of §11.1 verbatim, the step-5 correction of §11.2, and the *Find me by* additions of §11.3 | present | absent | the preventive half did not ship, and #13337 is only half closed |
| **F-8** | `go build ./... && go test ./...` in the container | green | green | — |

**F-1 is the load-bearing one and its shape was chosen deliberately.** A *count* of that literal across
the module is **not** discriminating: it is 2 before the change (two production copies) and 2 after
(one production copy in `loop`, one literal in `loop`'s own test). Only the *location* separates the
two states, which is why F-1 is scoped to the two adapter packages and asserts zero rather than
counting the module.

> **Correction, 2026-09-09 (`1b3f8f6`) — four of these eight rows stated a scope the command did not
> implement, and the one that was live cried wolf on compliant code.** F-3 returned a hit at
> `28d2998` on `internal/loop/attribution_test.go:162` and the property it tests was true throughout
> (§3.5). F-4 carried the identical pathspec defect, latent. F-2 stated a per-package expectation and
> emitted a joint list; F-5 asserted a production/test partition over a bare count. All four are
> corrected above, and F-6, F-7 and F-8 were audited and are clean — F-6's expectation is a file set
> and its output is a file set, F-7 is a manual node read, F-8's container qualifier is stated and
> load-bearing.
>
> **The generalisation, now a briefing rule (#1220):** *the command you run and the command you
> publish must be the same string.* F-3's scope existed in the `| grep -v _test` pipe it was measured
> with and never reached the document; F-5's existed in a prose column; §9's routing claim existed as
> an assertion over an output. **The tell is a falsifier whose expectation is a sentence and whose
> output is a number** — the sentence is where an unimplemented scope hides, because nothing compares
> the two. Seven of eight rows reproducing exactly is fully consistent with looseness that nothing has
> yet triggered, which is why the audit was run on the seven that passed and not only on the one that
> failed.

---

## 11. The Archetype Amendment to #10466

Three edits to the *"Adding a model provider, or a second protocol"* section. **This document specifies
the wording; it does not apply it** — the node is edited as part of the same PR by whoever holds the
graph write.

### 11.1 A new step, inserted immediately after step 4 and numbered **4a**

Numbered `4a` rather than renumbering, because #10466's existing correction blocks cite steps of this
archetype by number (*"Step 1 read…"*, *"this step previously read…"*) and the same node's own layer-5
hazard is resolved references breaking when something between the reference and its target moves.
`4a` inserts without falsifying a citation.

Exact text:

> 4a. **Call the loop's renderers; never compose the model's text yourself.** Everything the model
>    *reads* that is built out of the loop's own data is rendered by an exported function in
>    `internal/loop` — today `RenderUserContent` (the user message) and `RenderToolResult` (one
>    completed tool round). Your adapter calls them and places the result in whatever field its
>    protocol uses. **This is the dual of step 4:** step 4 keeps the wire's vocabulary out of the loop,
>    and this keeps the loop's vocabulary out of the wire. It is the step whose absence produced
>    #13337 — four functions byte-identical in the two existing adapters, because the second was
>    written by reading the first and the guide never said not to.
>
>    **Pin it from your side, and know exactly what that pin can and cannot do.** Assert that the
>    message your adapter sends is byte-equal to the loop function's output for the same input — the
>    shape `usercontent_test.go` already uses in both adapters. **That test proves your adapter's
>    *output* equals the shared function's. It does not prove you called it:** a byte-identical inline
>    re-implementation passes it, and no assertion in Go can tell the two apart, because there is no
>    seam to intercept a direct cross-package call. It *does* catch any composition whose bytes differ,
>    and any change to the shared function your copy failed to follow. It cannot pin the *layout*
>    either, because mutating the shared function moves both sides of the comparison together — the
>    layout is pinned once, with literals on the expected side, in `internal/loop`'s own test. Both
>    halves are needed and neither substitutes for the other. The clone case is caught structurally
>    instead, by the design's F-1.
>
>    **What you do *not* share is the tool declaration** — the envelope, the JSON parameter schema, the
>    endpoint's spelling of the tool's name, and the description. All four are per-protocol and
>    per-model, they stay in your adapter, and they are duplicated across adapters on purpose. The
>    ollama and openaicompat declarations are byte-identical today only because ollama adopted
>    OpenAI's tool envelope; the request types beside them already differ, and a third protocol need
>    not adopt either.
>
>    **The one question that places any new function:** *does it render the loop's own data, or declare
>    something to an endpoint?* Renders the loop's data → `internal/loop`, exported, pinned by the pair
>    above. Declares to an endpoint → your adapter, and let it be duplicated. Design
>    `docs/architecture/what-the-adapters-may-share.md` (**#13337**) argues the line and records what
>    stays duplicated as a decision.

### 11.2 A dated correction on step 5

Placed directly beneath step 5, in the node's established form — old text quoted in full, retained
rather than deleted, with the reason for retaining it:

> **Correction, 2026-09-09 (`28d2998`).** Step 5 read: *"Check the falsifier before you finish: grep
> `internal/loop`'s production code for the new provider's vocabulary — field names, finish-reason
> strings, error shapes. It must return nothing. This is a two-second check and it is the only thing
> standing between a neutral port and one vendor's format with an interface in front of it."* **The
> check itself is unchanged, still cheap, and still run.** *"The only thing"* is **false**. The
> falsifier is **one-way**: it detects the wire's vocabulary leaking *into* the loop, and is silent on
> the loop's vocabulary being re-implemented *inside every adapter*. Measured at `28d2998`, the
> falsifier returns nothing and the two adapters nonetheless carry **17 byte-identical declarations,
> 120 lines**, four of which the guide as written did not forbid (#13337). Step 4a above is the other
> direction. Retained rather than rewritten, because a reader who runs step 5, gets a clean result and
> concludes the seam is sound will reproduce exactly this duplication — which is how it arrived.

### 11.3 *Find me by* additions

Appended as a new `**Find me by (additions):**` block at the end of the amended archetype — the form
#10466 already uses at the close of its PR #43 section and its `2b07bed` section, which extends the
node's top-level `## Find me by` list rather than replacing it:

> **Find me by (additions):** what may two adapters share, may I extract a duplicated function out of
> an adapter, where does a tool description live, is a JSON tool schema the loop's vocabulary or the
> wire's, does the loop own the prompt, why are the two adapters byte-identical in places, should I
> write a test comparing two adapters against each other, loop vocabulary versus wire vocabulary,
> `RenderToolResult`, why is the neutrality falsifier not enough, one-way falsifier, #13337

---

## 12. Cross-Cutting Concerns, Quality Attributes, Risks

**Security, observability, concurrency, consistency:** unaffected. The moved function is pure, total
and deterministic; it touches no context, no clock, no I/O and no shared state. No log line, no error
code, no record field and no configuration input changes.

**Performance:** a cross-package call replaces an intra-package one. Not measurable and not a
consideration.

**Maintainability — what actually improves.** One layout instead of two, and it acquires a guard where
it currently has none (§3.3). The loop's `===== SECTION =====` family is complete in one package
instead of split two ways.

| Risk | Severity | Mitigation |
|---|---|---|
| The relocation changes a byte of the rendered text | would silently alter what every model sees | G-1's literals are written from the current text, and F-8's suite includes `client_test.go:370`'s existing byte-exact write-receipt assertion, which reddens on any drift in that branch |
| A reader takes §7 as licence to duplicate anything | the wire half grows without limit | §4's question is the limit and §11.1 puts it in the guide, which is where the next adapter's author will meet it |
| The design half ships and the archetype half does not | #13337 is half-closed and the next provider re-creates the duplication | F-7 is a review gate, not a nice-to-have |
| `assemble.go` accumulates renderers until it is no longer coherent | a later reader cannot find things | not a present problem — the file is **119 lines with 6 functions** at `28d2998` and gains one of about 20. Splitting it later is a move, not a design decision, and pre-empting it is YAGNI |

---

## 13. Open Questions

### 13.1 Does the operator want the two condensation types unified? — low stakes

`SentSampling` and `CondenseResult` are byte-identical in both adapters (§3.2) and are each adapter's
side of `condense.ModelPort`'s result vocabulary. Whether that vocabulary should be declared once in
`internal/condense` — as `loop` declares `JudgeResult` — is a real question and a different port, a
different design and a different PR. **§7 leaves them alone deliberately**; this is a flag, not a
recommendation.

### 13.2 The tool parameter schemas are pinned by nobody — file it?

§3.4's measurement, offered because it was found while measuring something else: neither adapter
asserts the *content* of a tool's parameter schema. Both assert only that it parses and that its
`type` is `"object"`; the property names `query`, `path` and `content` are asserted nowhere. A mutation
renaming `query` to `q` in one adapter's schema reddens nothing, and the endpoint would then reject or
mis-shape every recall call from that provider. **That is a coverage gap, not a duplication finding**,
and folding it into this PR would put two features in one (#1176). Recommended as its own task.

### 13.3 Nothing else

The design forces no other decision. Per #1184, no seeded questions are offered.

---

## 14. Implementation Guidance for the Next Agent

One unit, one PR. Ordered so that each step's guard exists before the step it guards.

1. **Add `RenderToolResult` to `internal/loop/assemble.go`**, beside `RenderUserContent`, as an exact
   relocation of `internal/ollama/wire.go:153-172` at `28d2998`, with its parameter type unqualified
   (`ToolExchange`, not `loop.ToolExchange`) now that it is inside the package that declares it. Do
   not improve it, do not reorder its branches, do not change a byte of its output.
2. **Write `internal/loop/toolresult_test.go`** (G-1) with literals on the expected side, covering all
   four branches and the two-candidate separator. **Observe it red** by mutating one character of the
   empty-recall sentence, and quote the output.
3. **Delete both adapters' copies** and call `loop.RenderToolResult` from each `buildMessages`.
4. **Add the byte-equality test to each adapter** (G-2, G-3), in the shape of that package's existing
   `usercontent_test.go:41`. **Observe each red** by inlining a `fmt.Sprintf` at the call site that
   produces **different** text. **Not an equivalent one:** *producing the same text cannot redden a
   byte-equality assertion*, by construction, so an equivalent inline copy is observed green and reads
   as a dead guard. That is §9's corrected mutation; this step prescribed the version §9 retired,
   and §18 records what it said. Confirm in the same run that G-1 stays green — the pair of
   observations is the evidence that the two guards measure different things.
5. **Run the falsifier table in §10 in full**, including F-6's diff scope, and quote each result.
6. **Run the suite in the container** (#10466: five GOOS-constrained tests are invisible on a Windows
   host), and the `=== RUN` gap tripwire while you are there.
7. **Apply §11's three edits to #10466** verbatim.

### Do not

- Touch `internal/loop`'s production behaviour beyond adding the one function.
- Extract, share, or "tidy" anything in §7's table.
- Add a cross-adapter comparison test (§6.2).
- Change any port, signature, exported adapter surface, or wire byte.
- Run a recursive grep from the repo root (§10's preamble).

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

- [x] No new type whose value-space mirrors an existing one — **no new type at all.**
- [x] No new abstraction with one implementation — no interface, no package, no registry.
- [x] No element justified by *"we might need X later"*. The one speculative-sounding claim — that a
      third protocol may not share the tool envelope — is used only to **decline** work, never to
      justify building any.
- [x] No deprecation period, flag, or shim.
- [x] Every extract/inline decision quotes `block_size × site_count`: **extracted** at `20 × 2 = 40`
      (§5.1); **overridden** at `17 × 2 = 34` for `writeFileTool`, with the named cost in §6.1 and
      §7 rather than a paraphrase.

**Existing systems first**

- [x] Audited whether an existing surface covers the concern: **it does.** `internal/loop` already
      exports a renderer of exactly this kind, pinned by exactly this pair of tests. The recommendation
      is a second member of a standing shape, not a new mechanism.
- [x] No new layer proposed, so no justification is owed. The one that was considered — a shared
      tool-declaration package — is named and rejected in §6.4.
- [x] No new persisted data.
- [x] Consumer chain recursed: `RenderToolResult` has two named production callers on day one, one per
      adapter.

**Configurability**

- [x] No new config knob, no environment variable, no `os.LookupEnv`. The module's single-read-site
      invariant is untouched.
- [x] No magic number promoted to configuration; the rendered text stays literal in one place.

**Less is better**

- [x] *Can it be deleted?* — G-1 without G-2/G-3 leaves the adapters free to compose their own text;
      G-2/G-3 without G-1 leaves the layout unpinned. Neither deletes. §6.4's package deletes cleanly
      and is therefore rejected.
- [x] *Can it be merged?* — yes, and it is: into `assemble.go`, not a new `render.go` (§5.1).
- [x] *Can it be inlined?* — it is inlined today, twice, which is the finding.
- [x] Trade-offs named explicitly: §6 rejects three alternatives on stated grounds; §7 states what
      stays duplicated with its arithmetic.
- [x] Radical-clean chosen where the surface has no consumer: `renderToolResult`'s two copies are
      removed outright rather than one kept as a fallback.
- [x] Reader inventory covers **both** AST references and string literals — §10's F-1 and F-5 are
      literal sweeps precisely because the AST view cannot see a duplicated string constant.

**Document discipline**

- [x] Cites #114 §0 and #1136 as load-bearing (header).
- [x] Scope and non-scope explicit (§2), including the two things declined with reasons.
- [x] No multi-paragraph rationale for things that obviously stay.
- [x] Supersedes no predecessor; #10532 is consumed and left live, correctly.
- [x] Every claim about the tree in §3 carries the command that produced it, at a named ref.
- [x] Every node id cited in this document was resolved against its node before being written
      (#1220 sweep layer 6): **#13337** (task, the finding), **#13334** (documentation, the
      model-adapter concept node, root #10454), **#10466** (documentation, the extension guide),
      **#10422** (project), **#10454** (repo map root), **#10532**, **#1136**, **#1267**, **#1176**,
      **#1184**, **#1220**, **#114**, **#8385**, **#9766**.

---

## 16. Provenance

Written 2026-09-09 against `main` at `28d2998`, from the finding **#13337** produced by the
concept-layer pass of the repo-map build (**#13334**). Every measurement in §3 was taken by this
document's author out of that ref via `git show` and `git grep`, over tracked files only; none was
carried over from the finding on report. Two of #13337's own statements were sharpened by that
re-measurement and both are recorded above rather than corrected silently:

1. **The duplication is 17 declarations and 120 lines, not four functions** (§3.2). The four #13337
   names are real and hash-identical; they are a subset.
2. **"Nothing fails if it is made once" is true of the tool result and false of the descriptions and
   the wire names** (§3.3). Each adapter restates those literals in its own test, so a one-sided edit
   reddens locally. What is silent in every case is the **divergence from the sibling** — which is the
   claim the finding is actually making, and it holds.

---

## 17. Revision 2026-09-09 — three corrections from QA, at `1b3f8f6`

The implementation shipped and QA reviewed it **APPROVED WITH WARNINGS**, with all three warnings
against this document and none against the code. Corrected in place per #11034 P-43 — a design
document is a dated record, so the superseded text is quoted rather than deleted wherever it carried a
lesson.

| # | Where | What was wrong |
|---|---|---|
| W-1 | §10 **F-3**, §3.5 | The command returned a hit on compliant code. Its stated scope (*production only*) was never implemented, and its first pattern term was not schema vocabulary. **Fixed by dropping the term, not by narrowing the pathspec** — QA's population measurement is what distinguished the two remedies |
| W-2 | §9 blockquote | The prescribed G-2/G-3 mutation — *"producing the same text"* — cannot redden a byte-equality assertion, so a reader following it observes green and concludes the guard is dead |
| W-3 | §9 G-2 column | *"an adapter that composes its own string fails it"* is measurably false for **byte-identical** composition. QA's M8 proved it; the over-claim was confined to this document, since the test names claim byte-equality rather than routing |

**Three further rows were corrected that nobody reported** — F-4, F-2 and F-5 — found by auditing the
seven falsifiers that *passed* rather than only the one that failed. F-4 carried W-1's defect
identically and was latent; F-2 and F-5 stated expectations at a finer grain than their commands
emitted. See §10's correction block for the rule this produced.

**What QA independently reproduced and this revision does not restate:** §3.2's 17 declarations / 120
lines at `28d2998` falling to 16 / 100 at `1b3f8f6`; the relocation byte-exact under her own
instrument; §3.1's `renderToolResult` hash; provider neutrality clean on all fourteen wire terms.

**Filed out of this revision, not fixed in it:** **#13356** — `internal/openaicompat` asserts the
write receipt by substring where `internal/ollama` asserts it byte-exact, which is why §9's clone
mutation reddens G-2 and not G-3.

> **Correction, 2026-09-10 (`f86e1ce`) — the scope note above was true when written and is not true
> now.** #13356 was fixed forty-eight hours after it was filed, by **PR #56**, merged as `f86e1ce`:
> `internal/openaicompat/write_test.go`, +2/−2, replacing `contains("11")` and `contains("index.html")`
> in `TestJudgeReplaysAPriorWriteRoundAsTheWriteToolAndItsReceipt` with a single literal assertion of
> the whole receipt, in the shape `internal/ollama/client_test.go:370` already used. **#13356 is
> closed**, and the cross-adapter obligation it named — *mutating `loop.RenderToolResult` must redden
> at least one test in each adapter package* — now holds symmetrically rather than strongly on one
> side and weakly on the other.
>
> **The superseded sentence is retained rather than rewritten** (#11228 Lesson 3). This section is the
> revision record — the section a later reader consults to learn what a revision did and deliberately
> did not do — and a scope note that is silently repaired stops being a record of what was believed
> and when. What made it worth correcting at all is the opposite hazard: *"filed, not fixed"* is the
> shape that gets cited as evidence a gap is still open, two days after a merged PR closed it.
>
> **§3.3's row stating the same fact is deliberately left standing.** It is a measurement pinned at
> `28d2998`, the base this design was written against, and it was **true there**. A sweep correcting
> every occurrence of the claim would falsify a correctly-dated measurement — the opposite failure and
> the more expensive one, because it destroys the record of what was true when. The rule the two
> halves make together: **a §3 row is read against its own ref and goes stale by design; a §17 scope
> note is read as current and must be corrected when it stops being so.**
>
> **`f86e1ce` is cited here as provenance for a closure, not as a measurement ref.** Every measurement
> in §3, §9 and §10 stays at `28d2998` and `1b3f8f6`, the refs the header names for them.

> **Second correction, 2026-09-10 (`860c0e7`) — the retained sentence's closing clause was wrong
> when it was written, and independently of the fact corrected above.** It ends: *"…which is why §9's
> clone mutation reddens G-2 and not G-3."* §9's clone mutation is the **ollama-side** experiment —
> re-add `renderToolResult` to `internal/ollama/wire.go` byte-identical, route to it, then edit
> `loop.RenderToolResult` by one character. **G-2 reddens because the clone did not follow the edit;
> G-3 stays green because `internal/openaicompat` did follow it**, which is correct behaviour and not
> a weak assertion. The substring-versus-literal asymmetry #13356 names lives in `write_test.go`, a
> **G-4**-class test, and bears on neither row. #13356's own body frames it against a different
> obligation entirely: *mutating `loop.RenderToolResult` must redden at least one test in each adapter
> package.* **The clause is retained rather than struck** for the same reason the sentence above it is:
> a revision record that quietly loses a bad inference stops being evidence of how the inference was
> made. Two facts were joined by *"which is why"* because both concerned the same pair of adapters,
> and adjacency was mistaken for causation.

---

## 18. Revision 2026-09-10 — the routing claim, swept rather than patched, at `860c0e7`

Opened for §17's one-section correction (#13442) and found two live defects; filed as **#13518** before
this revision widened, so what stayed out of the narrow task is on record ahead of the fix rather than
behind it. **The sweep was run for the claim, not for the two sentences #13518 names**, and it found a
third asserting site that neither #13518 nor the 2026-09-09 revision had.

### 18.1 The claim, and every site of it

The retired claim is: **a byte-equality assertion detects that the adapter *called* the shared
function.** It does not. It observes the adapter's **output**; a byte-identical inline
re-implementation passes it, and Go offers no seam to intercept a direct cross-package call. QA
measured this on 2026-09-09 (her M8) and §9 was corrected for it. **Three further sites asserted the
same claim and were not.**

| site | what it said | what it says now |
|---|---|---|
| **§3.4**, the second table row | *"**that the adapter routed through the shared symbol**"*, reddened by *"the adapter composing its own string"* | the emitted bytes equal the shared function's output; reddened by any composition whose bytes **differ**, and not by a byte-identical one. Dated note beneath the table |
| **§8.2** | *"An adapter that composes that text itself is in breach, and §9's **routing test** is what detects it."* | the breach is detected **only when the composed bytes differ**; the byte-identical breach is caught by §10's F-1, structurally and at review time |
| **§14 step 4** | *"**Add the routing test** to each adapter — **Observe each red** by inlining an **equivalent** `fmt.Sprintf` at the call site."* | *Add the byte-equality test* — observe each red by inlining one that produces **different** text, with the reason an equivalent one cannot |

**§14 step 4 is why this revision did not wait.** The other two are assertions a reader may believe;
step 4 is a **procedure that hands the reader a false negative**. Following it, they inline an
equivalent call, observe green, and conclude the guard is dead — which is the precise failure §9's W-2
correction exists to prevent, re-issued as the method. An instruction that cannot produce its
predicted result is worse than no instruction, and it was live on `main`.

**§3.4 is the site that explains the other two.** It is the *precedent's* row — the pair of tests
`RenderUserContent` is pinned by — and §9 says it *reproduces* that pairing rather than inventing a
new shape. §9's own correction block states that **this precedent has the identical hole**, then does
not come back and correct the precedent's row. So the corrected successor was left modelled on an
uncorrected source, and every restatement downstream inherited the source's wording. **A correction
that names a defect in the thing it was modelled on has not finished until it corrects that thing
too.**

### 18.2 The sweep, with its zeros

#11228 Lesson 1: enumerate the paraphrases a claim can take, grep each, and **report every count
including the zeros** — a zero reported is evidence, a zero unreported is an assumption. Run over the
document at `860c0e7` plus §17's #13442 note.

| pattern | hits | asserting the claim | denying, prescribing correctly, or quoting a superseded form |
|---|---|---|---|
| stem `rout*`, case-insensitive **and markup-tolerant** | 8 | **3** — `:187` (§3.4), `:516` (§8.2), `:769` (§14 step 4) | 5 — `:533`, `:554`, `:556` (§9, all denying), `:621` (§10, naming it retired), `:876` (§17 W-3, quoting) |
| `equivalent` | 1 | **1** — `:770`, the mutation prescription | 0 |
| `producing the same` / `same text` | 5 | 0 | 5 — `:540`, `:543` (§9, the superseded prescription under its own correction), `:875` (§17 W-2) |
| `composes its own` / `composing its own` / `compose their own` / `compose its own` | 4 | **1** — `:187`, already counted above | 3 — `:80` (a statement about #10466's *missing step*, not about detection), `:821` (§15, see below), `:876` (quoting) |
| `proves you` / `does not prove` | 2 | 0 | 2 — `:656-657`, §11.1's step 4a, which already states the limit correctly |
| `identical text` | **0** | — | — |
| `proves the adapter` | **0** | — | — |
| `calls the shared` | **0** | — | — |

**The line numbers above are the pre-revision ones** — they resolve against the state the sweep was
run on, not against this file, because correcting three of them shifted everything below. A sweep's
citations name the tree it measured, exactly as a §3 row names its ref.

**Surviving sites after this revision: zero.** Stated as a number because a retraction reported
without a count is untested (#11228, the check to run before calling one done). Re-run after the
edits, the `rout*` stem still returns its hits and **none of them asserts** — each is a denial, a
correction, or a quotation of a superseded form inside a dated record.

**The markup lesson, worth one line because it nearly cost the sweep.** `grep -i 'the route'` returns
**0** on this document while `**route**` and `*route*` both occur, because the emphasis markers sit
inside the phrase. **Sweep by stem, not by phrase, in any document whose prose is marked up** — the
first form reports a clean zero that means only that the writer used bold.

### 18.3 Checked and deliberately left

- **`:821`, §15's *can it be deleted?* row** — *"G-1 without G-2/G-3 leaves the adapters free to
  compose their own text"*. This is a **different claim** and it is true: without G-2/G-3 an adapter
  may compose text whose bytes differ and nothing objects. It makes no claim about detecting a
  byte-identical copy. Left as written.
- **`:80`, §1.1** — *"no step requiring a new adapter to call `loop.RenderUserContent` rather than
  compose its own user content"*. A statement about what the **guide** lacked, not about what a test
  proves. True, and it is the finding §11 closes. Left as written.
- **§12's byte-drift risk row** cites only `internal/ollama/client_test.go:370` as its mitigation.
  Since `f86e1ce` the openaicompat side carries an equivalent literal pin, so the cell **under-sells**
  its own mitigation. A cell that under-claims misleads nobody; left as written.

### 18.4 What this revision did not touch, and why

**§3's measurements stand at `28d2998`.** The `28d2998` pin in §3.3's write-receipt row — the one
§17's first correction declined to sweep — is untouched here for the same reason: it is a
correctly-dated measurement and it was true at its ref. **The distinction this revision runs on:** a
row that *measures the tree at a ref* goes stale by design and is read against its ref; a sentence
that *claims what a test proves* was either true or false the day it was written, carries no ref, and
is corrected wherever it appears. §3.4's row is the second kind sitting inside a section full of the
first, which is most of why three sweeps walked past it.

**Every edit inside §3 is additive or confined to the non-measurement cells of one row.** No command,
no hash, no count and no ref in §3 changed.

### 18.5 Parity

The repo file and node **#13345** were republished together and compared byte-for-byte in both
directions, before and after (P-40). An edit to one is not finished until the other matches it.
