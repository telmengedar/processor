# Architectural Document: What a Run Record's Name Carries

**Source task:** DiVoid **#13261** — *"A node name is ranked text and no design treats it as one."*
**Repo path:** `docs/architecture/what-a-run-records-name-carries.md`
**DiVoid parity node:** **#13480** — this file and that node carry the same bytes (P-40 / P-48). Either is the artifact; neither is a copy of the other.
**Written against:** Processor `main` `860c0e7`, clean tree. Measurements run 2026-09-10 against the live graph.
**Reviewed:** #13505 (round 2) re-ran every load-bearing measurement with a 6.2× tighter instrument and reproduced all of them; the revisions that review required are marked **[R2]** where they land. **#13511** (round 3) approved both documents with five warnings — all sentence- or figure-sized, none reopening a decision or a measurement; the edits they required are marked **[R3]**.
**Standards:** Design Contracts **#1136** (§1 KISS/DRY/YAGNI load-bearing; §5 Pre-Design Checklist walked in §14 below), Code Contracts **#114**, Go rule set **#11034** (**P-51** — every *because* clause carries its measurement or its hedge in the same sentence), and the standing commissioning rule of **#12958 §18.6**.

---

## TL;DR

**What.** The run record's name stops restating the input and starts stating the run's own outcome. The shape goes from

> `processor-run <RFC3339> — <input, truncated to 80 runes>`

to

> `processor-run <RFC3339> — <outcome, truncated to 80 runes>`

where *outcome* is the answer's opening when the run produced one, and a terminal-state clause naming the stop reason and call count when it did not.

**How.** One function in `internal/divoid/write.go` (`runName`) plus one small unexported clause builder. Nothing else moves. `RunNamePrefix` is unchanged, so `IsRunRecord` and every consumer that keys on it are untouched — all six of them key on the prefix and nothing parses past it (§7 inventory).

**Why, in one measured sentence.** DiVoid embeds `name + "\n\n" + content` as **one text truncated to 8,000 characters** (`EmbeddingInputComposer.MaxLength`, read from source at `C:\dev\claude\DiVoid\Backend\Services\Embeddings\EmbeddingInputComposer.cs`, and confirmed on the deployed build by the bracket in §4.2). For the live specimen **#13472** that window covers **11.1%** of the record and **`answer` sits at character 69,261 of a 7,880-character content budget** — so the run's own conclusion is not merely diluted, it is **not embedded at all**, while the input is embedded **twice** (once in the name, once at content offset 1). The name is the only field of a run record whose presence in that window is guaranteed regardless of body size, and today it is spent on the one thing the content already carries.

**The cost, measured, and it is not zero.** Dropping the input from the name costs a verbatim repeat **−0.0374** similarity (0.5900 → 0.5526) and moves the record from **rank 1 to rank 5** of 11,044 nodes. It does **not** eliminate crowding, and no name change can: the input remains at content offset 1, inside the window. What it buys is the other half — text that exists *only* in a name reached **rank 2 of 11,044** on an alien query (§4.3), so the outcome moves from *outside the window* to *the highest-weight position in it*.

**Strongest rejected alternative: the timestamp alone** (#13261's own second question). Rejected on measurement, not taste: it removes **only 35%** of the repeat-match lift (content-only 0.5526 against both-copies 0.5900, over a floor of 0.4836) while removing **100%** of the name's value as a list label and as the record's guaranteed-embedded identity. It pays the full identification price for a partial retrieval benefit, and leaves the name — the densest ranked text the node has — carrying nothing at all.

---

## 1. Problem Statement

A run record's name is `processor-run <timestamp> — <input truncated to 80 runes>`. Two consequences, both measured before this design:

- **#13261** — a query equal to the task input returns **13 of 30** run records, holding **eleven consecutive top ranks**. A repeat of an input is a near-exact lexical match against every prior run on that input.
- **#13274** — a query describing exactly one record's most distinctive outcome returns it **not at all**. A run record is findable as *"a run"* and not as *"the run where X happened."*

#13261 deliberately left four questions unweighed, and they are this document's subject: whether a name should carry the input at all; whether the timestamp alone would suffice; what a name is *for* once it is also ranked text; and whether the problem generalises beyond run records.

**The goal this design is written to** — stated as a goal, not a mechanism:

> **A run record should be retrievable by what it concluded, and should not outrank the nodes that own the text it was asked about.**

Two halves. The **positive** half (findable as itself) is #13274's failure; the **negative** half (not crowding) is #13261's.

*Which half is more serious is a judgement, not a measurement, and it is stated as one (P-51):* #13274's ruling is that for a system whose substrate is memory, *the right row cannot be recognised* is worse than *the wrong rows appear*. **Nothing here measures that ordering**, and the design does not need it — both halves are addressed, and §10 reports the negative half's residue as a number rather than arguing it away. The ordering is carried only because it decides which falsifier (F-2) is the acceptance test.

### 1.1 Success criteria

| # | Criterion | How it is checked |
|---|---|---|
| S-1 | The run's conclusion is inside the embedded window | Structural: the name is always preserved by the composer, so this holds by construction |
| S-2 | A query describing a record's distinctive outcome returns that record | F-2 in §12 — #13274's exact failing query shape, re-run against a record written after the change |
| S-3 | A repeat of an input returns fewer prior records at the head than #13261's 13-of-30 baseline | F-3 in §12 — comparable only once several records exist under the new name |
| S-4 | No consumer of the run name breaks | §7 reader inventory; all key on `RunNamePrefix` |

---

## 2. Scope & Non-Scope

**In scope.** What text the run record's name carries, and the rule that derives it from the record. One function in the graph adapter.

**Explicitly out of scope** (each named, per #1136 §5 document discipline):

| Out of scope | Why, and where it belongs |
|---|---|
| The composition of the 8,000-character embedding window — i.e. the record's JSON **field order** | §5.3 measures it and it is the larger lever, but it changes the record's serialization, which is **#13238**'s live surface. Bundling would violate one-feature-one-PR. **Filed as a follow-up task** (§13). |
| Removing `block` from the stored record | **#13238, now merged to `main` at `860c0e7`.** §5.4.2 falsifies **one of its four reasons** (reason 4, the retrieval claim) and the correction is filed at that document rather than only here (§14). **It does not re-decide the removal**, which stands on reasons 1, 2 and 3. |
| Whether `substance` should participate in ranking | #13241 measured that it does not. Changing that is a DiVoid-side decision, not a Processor one. |
| The self-produced exclusion at fusion and admission | #13238, superseding the closed #48. Live at `main` (`internal/loop/assemble.go:52`). §6.3 states what it means for urgency. |
| Renaming the 17 existing records | A graph-write script, not a format change. **Decision D-3** (§9), filed as its own task. |
| Any change to DiVoid's embedding pipeline | Foreign repo. §12 F-4 registers the guard instead. |

---

## 3. Assumptions & Constraints

| # | Assumption | Confidence | What would falsify it |
|---|---|---|---|
| A-1 | DiVoid embeds `name + "\n\n" + content`, capped at 8,000 characters, name always preserved | **Measured twice** — read from `EmbeddingInputComposer.cs` source, and confirmed against the deployed build by the §4.2 bracket | A change to `MaxLength` or to the composition order. F-4 registers the re-run. |
| A-2 | `substance` is not embedded | Measured, #13241 | DiVoid re-embedding on substance write. #13241 already registers this guard. |
| A-3 | An answer's opening clause states the run's conclusion rather than a preamble | **Assumed, not measured.** One specimen (#13472) opens *"The text of the pull request body is owned by the orchestrator"* — a conclusion. n=1. | F-1 in §12, **and it must be read against the clause, not against the name**: of 20 records, more than a third share the first 30 runes **of the outcome clause — the name from rune 37 onward**. The name's own first 37 runes are prefix and timestamp and can say nothing about A-3. |
| A-4 | Node names are not length-bounded by DiVoid | Checked — `Node.Name` carries no length attribute in `Backend/Models/Nodes/Node.cs` | A schema constraint appearing later; the design stays inside ~118 characters regardless. |
| A-5 | `PATCH /name` regenerates the embedding | Documented in #6115 and #440 as one of four mutually-exclusive UPDATE branches; **not independently re-measured here** | Only load-bearing for the deferred corpus rename (D-3), not for this design. |

**Constraint.** Whatever the name becomes must be derivable from `loop.Record` at write time, with no additional model call and no additional graph read. `runName` already receives the whole record, so this constrains the *content* of the rule, not its plumbing.

---

## 4. The measurement this design turns on

#13261 assumed the name is embedded and heavily weighted. That was **inferred, not measured** — and the brief for this work asked for it to be settled before designing, because the whole shape of the answer depends on it. It is now settled, and the answer is more specific than the assumption.

### 4.1 The composition, from source

`EmbeddingInputComposer.Compose(name, content, contentType)` returns

> `name` + `"\n\n"` + `content[.. 8000 − len(name) − 2]`

**One text, one vector.** Not two vectors, not a weighted blend, not a name field the ranker knows about. `MaxLength = 8000` characters, commented in source as *"heuristic for ~2k-token Gemini embedding budget… treat as a tunable constant, not a contract."* The name is **always preserved**; the content is what gets cut. Model: `gemini-embedding-001`.

**So the name is not privileged by weight — it is privileged by position and by guarantee.** It is the only text a node has that cannot be truncated out of its own embedding.

### 4.2 The deployed build agrees with the source

Source is a claim about a build; the graph is a deployment. Two probe nodes, identical filler vocabulary, one marker phrase placed at different offsets:

| probe | marker at character | similarity on a query for the marker | rank |
|---|---|---|---|
| TIN | 7,363 | **0.7084** | 1 of the whole graph |
| TOUT | 8,610 | **0.5430** | 4 |

TOUT's 0.5430 is not a weak signal, it is **no signal**: the non-probe row immediately above it scored **0.5431**, and the field ran 0.5192–0.5431. The marker at 8,610 contributed nothing distinguishable from noise. **The cap is between 7,363 and 8,610 on the deployed build**, consistent with `MaxLength = 8000`. Margin between the arms: **0.165**.

**[R2] The bracket has since been tightened 6.2×, by a stronger instrument, and it agrees.** #13505 §1.2 built **identical-prefix pairs** — two nodes byte-identical through 7,983 characters, differing only in a 1,000-character distinctive passage placed after that point. That design's prediction under truncation is not *"weaker signal"* but *"no difference at all"*, which is a far harder test than a single marker's similarity dropping. The passage was found at offset 0 and at 6,900; moved to 8,000 and nothing else changed, both probes **vanished from the top 500 of an 11,000-node graph, in both query directions independently**. **The cap lies between 7,900 and 8,000**, containing 7,983 = 8000 − 15 − 2. Two arms this design did not run were added there and both matter: content is **head-kept and tail-cut**, which is the fact that actually puts `answer` outside the window rather than inside it; and a 110-rune marker in a **126-character name on a 20,000-character body** — 2.5× the cap — took **rank 1 of the whole graph at 0.6159**, which is §4.3's A10 arm corroborated at a body size that overruns the cap by 150%.

*Caveat on re-running this bracket, and it is not cosmetic (P-51).* **The 0.165 margin is a property of this marker, not of the method.** #13505 §1.4 reports an independent probe design whose markers were ~110 characters of moderate distinctiveness buried in ~9,000 characters of alien filler: it returned a **null-to-negative differential at every offset, including offsets well inside the cap**. A short marker in a long low-signal body is below this instrument's detection floor. **F-4's re-run must use a lexically distinctive marker phrase, or a null result will be misread as a cap change.**

### 4.3 Where the input has to live for a repeat to match — a 2×2, with a floor

Four probes, record-shaped bodies (a `candidates`-style disposition list as filler, the shape that actually fills a run record's window), one query equal to the input, run 2026-09-10 against a graph of 11,044 nodes. **Both populations named per #12958 §18.6 clause 5: every similarity below is drawn from the same four-probe set and the same single query; the "floor" is the fourth arm of that same set, not a band from elsewhere.**

| arm | input in name | input in content | similarity | rank | lift over floor |
|---|---|---|---|---|---|
| **A11** — today's shape | yes | yes | **0.5900** | 1 | +0.1064 |
| **A10** — name only | yes | no | **0.5734** | 2 | +0.0898 |
| **A01** — timestamp-only name | no | yes | **0.5526** | 5 | +0.0690 |
| **A00** — floor control | no | no | **0.4836** | absent from top 300 | — |

Cross-instrument check (#13241's method note — run every instrument the conclusion will be quoted against): `divoid_search` returned 0.5900182 / 0.5733563 / 0.55256736 for A11 / A10 / A01. **The MCP tool and the HTTP route `Recall` builds agree to seven decimals.**

**[R2] How the floor arm was obtained, because neither instrument as named above reaches it (W-5).** A00 scores below both routes' result horizon: the HTTP route hard-caps at 500 rows and the 500th scored 0.5216, and a scoped `divoid_search` returned 50 of 147. **The reproducible method is to re-parent the floor arm under a fresh single-member group and scope the search to that group**, which forces the row to be returned regardless of rank. Done that way it reads **0.48361197** (#13505 §1.4, independently). A reader who tries to check the headline 35% figure without this step will not find the row at all — that is the instrument's horizon, not a missing measurement.

**The two marginal figures that decide this design:**

- Adding the input to a name that already has it in content: **+0.0374** (A01 → A11).
- Adding the input to content when the name already has it: **+0.0166** (A10 → A11).

The name copy is worth **2.25×** the content copy **at the margin**, on **1.4% of the probe's own embedded text** — the probe name is 118 characters of the 8,237 characters the composer actually embedded for that probe. *(Denominator named explicitly per W-7: this is the probe's embedded length, which is not the 8,000-character cap and not §5's 7,880-character budget. Three different denominators are in play across this document and each is now named where it is used.)* And the two copies are strongly redundant — 0.0898 + 0.0690 = 0.1588 against a combined 0.1064 — which is Toni's *"duplicating content in a graph is complete nonsense"* showing up inside a single node's own two text surfaces.

*Hedge, in the same breath (P-51):* one query, one graph snapshot, one embedding model, one name length against one content length. The direction is measured; the ratio is a single point.

---

## 5. Architectural Overview — what is actually in a run record's embedding

Everything above is generic. This is the specimen. **#13472**, written 2026-09-10T13:21:01Z, the first run record in the graph carrying a substance.

```
node #13472           body = 70,719 chars           name = 118 chars

  EMBEDDED WINDOW  (name 118 + separator 2 + content 7,880 = 8,000 chars)
  +--------------------------------------------------------------------+
  | name          0 .. 118   "processor-run <ts> — <input, 80 runes>"   |  1.5% of the 8,000 window
  | "input"       1          the SAME input again, verbatim, untruncated|
  | "subject"     111                                                    |
  | "query"       127                                                    |
  | "queries"     237                                                    |
  | "anchor"      351                                                    |
  | "candidates"  535 ....................................... 7,880 cut |  91.8% of the 8,000 window
  |               ids, OTHER NODES' names, similarities, hashes          |  (= 93.2% of the 7,880 budget)
  +--------------------------------------------------------------------+
        ~~~ nothing below this line is embedded at all ~~~
  | "block"       8,360      60,669 B of other nodes' bodies             |
  | "answer"      69,261     what this run concluded                     |
  | "model"       70,316                                                 |
  | "stopReason"  70,511     the terminal reason                         |
  | "limits"      70,559     "usage" 70,466   "capReached" 70,447        |
```

Four readings fall straight out, and three of them correct a standing account.

### 5.1 The window covers 11.1% of the record

7,880 of 70,719 characters. Everything else in the record has **no** effect on retrieval — it is stored, served, and rendered, but it is not ranked.

### 5.2 The run's own outcome is not in the embedding

`answer` at 69,261. `stopReason` at 70,511. `model`, `usage`, `capReached`, `toolCalls` — all outside. **This is the mechanism behind #13274's null result**, and it is a sharper account than the one #13274 gave. #13274 attributed the failure to the embedding being *"73% other nodes' bodies"*, so that the identity fields *"contribute almost nothing."* Corrected: **they contribute exactly nothing, because they are past the cap.** The 73% figure is true of the record's *stored bytes* and false of its *embedded text* — where `block`'s contribution is not 73% but **zero**.

### 5.3 93% of the *content budget* is other nodes' names

**[R2] One denominator per figure, named (W-7).** `candidates` runs from 535 to the cut at 7,880 — **7,345 characters**. Against the **7,880-character content budget** that is **93.2%**; against the **8,000-character window** (budget plus the 118-character name plus the 2-character separator) it is **91.8%**. Both are true, they are not the same number, and the earlier heading quoted the first against the second. The budget is the right denominator for *"what share of the record's own content got embedded"*, which is the claim this section makes. **What fills that budget is the candidate disposition list** — ids, similarities, content hashes, and the **names** of twenty other nodes. This is literally *"pushing contents of other nodes into it"*, and it survives the removal of `block` because the disposition list is not `block`. It is the larger lever, and it is out of scope here (§2) for a reason stated in §13.

### 5.4 Removing `block` does not bring the outcome into the window

**[R2] Re-measured exactly, by excising the `block` span from the stored bytes rather than re-serializing them.** The `block` value spans 8,360 → 69,261, so a record without it is **9,818** characters and `answer` begins at **8,360** — against a content budget of **7,880**. It is outside **by 480 characters**. With a timestamp-only 34-character name the budget rises to 7,964: still outside, by 396.

> *Method note, because it is worth 10 characters and the difference is instructive.* Re-serializing the record through a non-Go JSON encoder gives 9,808 / 8,350 instead. The gap is **exactly two `&` characters in the candidate names**, which Go's `encoding/json` HTML-escapes to the six-character sequence `\u0026` while other encoders emit the single character. **Excising the span from the stored bytes involves no encoder at all**, so 8,360 is the figure that matches what DiVoid actually received. The conclusion is identical either way; the discipline is that an offset claim about stored bytes should be measured on the stored bytes.

### 5.4.1 The margin is 1.2 candidate entries wide — and that belongs in the sentence

**[R2] This is the correction that most needed a number (P-51).** §5.4 previously hedged that *"a record with fewer candidates would clear the budget earlier"* without saying how many fewer. Measured, on this record's own entries:

| candidates | `answer` offset, `block` removed | inside 7,880 (today's name)? | inside 7,964 (timestamp-only name)? |
|---|---|---|---|
| **20** (today's configured limit) | **8,360** | no, by 480 | no, by 396 |
| **19** | **7,955** | no, by 75 | **YES, by 9** |
| **18** | **7,556** | **YES, by 324** | **YES, by 408** |

~~Twenty entries occupy 7,825 characters, a mean of **389.6 each**.~~ **[R3] Corrected 2026-09-10 (W-4): the twenty entries occupy 7,790 characters, a mean of 389.5 each**, range 306–515 — measured on #13472's stored bytes by walking the array and summing the twenty entry spans. **The struck figure conflated the entries with the field that holds them.** 7,825 is the whole `candidates` field *including* the comma that separates it from the preceding field, its `"candidates":[` key and both brackets (offsets 534–8,358; the key-to-bracket span alone is 7,824). The old sentence's two halves could not both be true of the same objects — 20 × 389.6 = 7,792 ≠ 7,825 — and it stood in the paragraph that fixed a denominator conflation, which is #13274's own closing rule landing on the correction that quotes it: **name the denominator in the sentence.**

**Nothing downstream moves.** 480 / 389.5 = **1.23 candidate entries**, which is what this subsection's heading and table already said, and the candidate count is a **configured limit** (`20 cands` in the record's own limits line), not a constant of the design. The figure travelled to three sites and all three are corrected — §14's *Document discipline* table carries the sweep.

**State the conclusion at the width it actually has.** *For records of the shape the loop writes today — twenty candidates, a 60,669-byte block, a 118-character name — removing `block` leaves the outcome outside the window.* At **eighteen** candidates it would not. At nineteen with a timestamp-only name the answer lands inside **by nine characters**, which is a coin toss, not a finding. **This is a narrow margin measured at one record, not a property of run records in general**, and anyone quoting it onward must carry the candidate count with it.

*What that does and does not do to the decision:* nothing. D-1 does not rest on `answer` being outside — §10's F-4 note already records that if the cap grew to cover a whole record, **D-1 still stands**, because a name restating the input is a second copy of a field the content carries either way. §5.4 bears on **#13238's reason 4**, not on this design's decision.

### 5.4.2 What survives in #13238, stated as the table rather than as a summary

**[R2] The previous sentence here undercounted, and that undercount is what let the correction go unfiled (W-3).** It said removal *"remains right for the reasons #13238 gives"* and named two — duplication, and the recursion hazard — where duplication is one of four reasons, two more plainly survive, and **the recursion hazard is not one of §9.5's four reasons at all**: it comes from #13274. Restated against #13238 §9.5 as written:

| | #13238 §9.5 reason | Standing after this measurement |
|---|---|---|
| 1 | **Redundant storage** — ~60 KB of verbatim duplicate of content the graph holds behind an edge | **Survives untouched.** Rests on no retrieval claim. |
| 2 | **It makes a record unrenderable** — a prose compressor handed 73.6% other nodes' bodies digests those nodes (#13242) | **Survives untouched.** A claim about the condense path. |
| 3 | **It makes a record unadmittable** — 77–88 KB against a 60,000-byte budget | **Survives untouched.** Byte arithmetic on the assembly budget. |
| **4** | **`block` displaces the record's identity in its own embedding, so removal is a retrieval improvement** | **FALSIFIED, in both clauses.** `block` starts at 8,360 against a 7,880-character budget, so it contributes **exactly zero** to the embedded text and can displace nothing; and with it removed `answer` still lands at 8,360, outside. |
| — | ~~It causes the crowding~~ | Already struck by #13261. Stays struck. |

**Three of four reasons stand, so removing `block` still proceeds** — and Toni's own principle, *duplicating content in a graph is nonsense, that's what edges are for*, never needed a retrieval argument in the first place. **This narrows #13238's argument; it does not reverse its decision.** Saying it in that order is the point: *"three of four reasons still stand"* is the true statement, and *"the case for removing `block` has weakened"* is the one a careless summary would produce.

**#13274's consequence — that removing `block` *"is what makes a run record retrievable as itself"* — does not hold for a record of this shape.** It is not the fix for skew. **The name is the only carrier of the outcome that survives the cap without reordering fields**, which is what makes this design a small change rather than a serialization change.

**These corrections have been carried to their origins, not left here.** #13238 §9.5 and #13274 are both amended under dated notes (§14, *Document discipline*). A correction that lives only in the document that discovered it is not a correction — it is a disagreement, and #11034 **P-43** and **P-52** exist because the two are easy to confuse.

---

## 6. Components & Responsibilities

Only one component changes. It is named here with what it does **not** own, per single-responsibility framing.

| Component | Owns | Does not own |
|---|---|---|
| `internal/divoid` — the run-name rule (`runName` and a new unexported clause builder) | Deriving the record's node name from the record: the prefix, the timestamp, and the outcome clause | The record's *content* (that is `loop.Record`'s JSON), the *substance* (that is `loop.RenderSummary`), whether the record is written at all, and what the embedding does with the name |
| `internal/loop` — `RenderSummary` | The human-facing projection, including the input at 200 runes on line 2 | Nothing changes. It is named here because it is **why the input is not lost** — see §6.2 |
| `internal/divoid` — `IsRunRecord` | Recognising a row as self-produced, by **prefix** | Nothing changes, and this is load-bearing: the prefix is the contract, the tail is not |

### 6.1 The name's job, stated once

> **A node's name is the only text it has that is guaranteed to be inside its own embedding, and the only text that is rendered in every listing that never renders its content or its substance.** It must therefore carry what is true of *this* node and false of its neighbours.

**[R2] The two premises do not carry equal weight, and the *therefore* was resting on both (P-51).** The first premise is **measured** — §4.1 from source, §4.2's bracket on the deployed build, and #13505's tighter re-bracket. The second — *rendered in every listing that never renders content or substance* — is an **observation of the DiVoid clients in front of me** (the graph UI's node lists, `divoid_list` and `divoid_search` result rows, the link-neighbour listings), **not a measurement and not a guarantee any contract offers**; a future client could render substance in a list and it would not be wrong to. So the conclusion rests on the first premise, which is enough on its own: text guaranteed to be in the embedding should not be spent on a second copy of text already in the embedding. The listing observation is why the change is *also* a legibility win, and it is not load-bearing.

That is the answer to #13261's third question — *what is a name for, once it is also ranked text?* — and it is not specific to run records. §11 states it as a universal with its falsifier.

### 6.2 The input is not removed. It is de-duplicated.

This matters and should not be glossed:

- The input stays in the record's **content**, at offset 1, **inside** the embedded window, untruncated.
- The input stays in the record's **substance**, line 2, at 200 runes — which is what a reader sees on opening the node.
- What goes away is the **second** copy, in the name.

So a repeat of an input still matches (measured: A01 at 0.5526, rank 5 of 11,044). A human opening the record still reads the input immediately. Nothing becomes unreachable; one of two copies of one field stops competing for the node's scarcest and highest-weight text.

### 6.3 What today's crowding actually costs, and why that bounds the urgency

The self-produced exclusion is **live** at `main` — `internal/loop/assemble.go:52` cuts every self-produced candidate with reason `self-produced`, and #13274 records the cut at fusion as well. **So today's crowding harms a human or an agent searching the graph; it does not harm the loop's own recall**, because the loop discards those rows before admission.

That changes under **#13238**, which relaxes the exclusion. **The crowding becomes a loop defect on the day the exclusion is relaxed, and not before.** That is the argument for doing this now and the argument for not bundling it with anything (§13).

---

## 7. Contracts & Interfaces (Abstract)

### 7.1 The name grammar

| Segment | Content | Bound | Stability |
|---|---|---|---|
| prefix | the literal `processor-run` | fixed | **Contract.** `IsRunRecord` and five other consumers key on it. Never changes. |
| separator | a single space | fixed | — |
| timestamp | the write instant, RFC3339, UTC | 20 chars | Unchanged from today |
| separator | ` — ` (space, em dash, space) | fixed | Unchanged from today |
| **outcome clause** | **new** — see §7.2 | **80 runes**, ellipsis when truncated | The changed segment |

Total stays at approximately **118 characters**, the same order as today, so the content budget inside the 8,000-character window does not move materially.

### 7.2 The outcome clause — the rule, with branch 2's literal grammar

Two branches, tested in order.

**Branch 1 — the record carries a non-empty answer.** The clause is the answer's leading text, with all runs of whitespace collapsed to single spaces, then truncated to the rune bound.

> **[R2] "Non-empty" means non-empty *after* collapsing and trimming (W-4).** An answer of nothing but whitespace is non-empty by length, would collapse to a single space, and would produce `processor-run <ts> — ` with a dangling separator and no clause. I-3 does not catch it, because the prefix keeps the name non-blank. **Order the test after the collapse**, so such a record falls through to branch 2 — which describes it correctly in any case, since a run that produced only whitespace produced no answer.

**Branch 2 — the record carries no answer.** The clause names what happened instead. **[R2] Its literal grammar, because §15 step 4 requires an implementer to pin a literal expected name and cannot do so against prose (CF-3):**

| case | clause, literally |
|---|---|
| general | `<reason>, <modelCalls>/<maxModelCalls> model calls, cap <reached\|not reached>` |
| `answered` yet empty | `answered but the answer was empty, <modelCalls>/<maxModelCalls> model calls, cap <reached\|not reached>` |

with these fillings, all of which are already fixed elsewhere in the repo and are **reused, not redefined**:

| placeholder | source | vocabulary |
|---|---|---|
| `<reason>` | the record's terminal reason | the loop's closed set, verbatim and unstyled: `answered`, `wantsRecall`, `truncated`, `refused`, `wantsWrite`, `unrecognised` |
| `<modelCalls>` / `<maxModelCalls>` | the record's call count and its configured cap | decimal, no padding, joined by `/`, exactly as the summary's `OUTCOME` line renders them |
| `cap <reached\|not reached>` | the record's cap flag | the two literals `cap reached` and `cap not reached` |

**Separators are part of the contract:** `, ` between the three parts, `/` between the counts, and no trailing punctuation. Worked examples, so there is nothing left to invent:

> `processor-run 2026-09-10T13:21:01Z — truncated, 6/6 model calls, cap reached`
> `processor-run 2026-09-10T13:21:01Z — answered but the answer was empty, 3/6 model calls, cap not reached`

**Why this grammar and not a freshly-invented one (DRY, #1136 §1).** `RenderSummary` already renders exactly these three facts on its `OUTCOME` line — `answered (raw "stop"), 1/6 model calls, cap not reached` — and the repo already owns the terminal-reason vocabulary and the `reached` / `not reached` literals. The clause is that line **minus the ` (raw %q)` segment**. **[R3] That segment is `StopReason.Raw`, the endpoint's verbatim terminal-reason token** (`"stop"`, or a vendor string) — provider noise a name should not carry. **It is not an endpoint** (W-5): the earlier wording here said *"the raw endpoint string"*, eliding `internal/loop/types.go:132`'s *"the endpoint's verbatim string"* into the wrong noun, in the one sentence that tells an implementer what to strip. The worked example above quotes it correctly as `answered (raw "stop")`. Two renderings of one fact should not disagree about how to word it.

**Why the `answered`-yet-empty case gets its own wording, and why it is worth the extra branch.** That contradiction is the record's single most distinctive property, and it is *verbatim* the query #13274 measured as returning nothing — *"a run where the terminal reason said answered but the answer came back empty after three model calls."* The clause above contains **answered**, **empty**, and the call count, which is what makes S-2 / F-2 a real acceptance test rather than a hopeful one. The summary flags the same contradiction with `<-- terminal reason says answered`, which is right for a rendered pane and wrong for ranked text — arrows do not retrieve.

*One cosmetic note for the implementer, not an invariant.* Branch 1's clause will carry whatever markup the answer opens with — `**`, node ids — into every list label. Harmless for the embedding, which is what this design is about. Stripping it is not proposed: it would be a second truncation rule to keep correct, and I-5 exists to avoid exactly that.

### 7.3 Invariants the implementer must hold

| # | Invariant | Why |
|---|---|---|
| I-1 | The name never contains a newline or a tab | Names are single-line labels in every listing; the composer joins name and content with `\n\n` and a newline inside the name blurs that boundary |
| I-2 | The name always begins with exactly `RunNamePrefix` | §7.1. Six consumers. |
| I-3 | The name is never empty and never whitespace-only | DiVoid's composer treats a whitespace-only name as absent (#6115); the prefix makes this structural |
| I-4 | The rune bound is a `const` in the adapter, not a config knob | #1136 §3 — no operator and no environment differs on it |
| I-5 | Truncation reuses the existing rune-truncation helper **`truncateRunes`** (`internal/divoid/write.go:111`), unchanged — including its `…` ellipsis on overflow | #1136 §1 DRY. **[R2] The helper is named** so this cannot be re-implemented by accident. Word-boundary truncation was considered and dropped as gold-plating: the embedding is indifferent and the reader loses nothing. |

---

## 8. Interactions & Data Flow

Unchanged in shape. The write-back sequence is: build the name → create the node → post the content → set the substance → link to the subject. **Only the first step's inputs change**, from `record.Input` to `record.Answer` / `record.StopReason` / `record.ModelCalls` / `record.CapReached` — all already present on the `Record` value `runName` receives.

No new call, no new read, no new failure mode, no ordering change. The name is computed after the run completes, as it already is.

---

## 9. Decisions, with alternatives named and reversal cost

Per the brief: no stalling — each is a decision, not a question.

| # | Decision | Alternative rejected | Reversal cost |
|---|---|---|---|
| **D-1** | The name carries the run's **outcome**; the input is dropped from the name | **Keep the input and append the outcome** — both in ~160 characters. Rejected on the measured half: it keeps the **+0.0374** crowding lift (§4.3, A01 → A11) while halving the outcome's share of the name. **[R2] The legibility half is a judgement and is stated as one (P-51)** — a ~160-character label carrying two unrelated clauses is *asserted* to be harder to scan than an ~118-character label carrying one; nothing here measures reading. It is not load-bearing: the measured half rejects the alternative on its own. **[R2] Structurally this is #1136 §5's radical-clean-vs-compromise box, and the rejected option is the compromise shape by construction** — it keeps the old carrier *and* adds the new one, so it pays both costs and commits to neither. | One format string |
| **D-2** | The outcome clause is the **answer's opening**, falling back to a terminal-state clause | **Metadata-only clause** (stop reason + admitted/cut counts, no prose). Rejected because metadata is drawn from small closed sets — six terminal reasons, a call count bounded by the configured cap of 6, a boolean — so the clause's *whole* variability across runs is those three fields, while the answer varies with what the run concluded. **[R2] The consequence — that near-identical clauses would crowd each other — is an inference from §4.3, not a separate measurement of it (P-51):** §4.3 measured that near-identical *name* text lifts a match by +0.0374 at the margin, and the inference is that text which is near-identical *by construction* lifts every record on the same query. It is not measured directly, and the cheap way to measure it is F-1, which counts exactly this collision on the branch that actually ships it. | One helper; F-1 in §12 is the trigger |
| **D-3** | **Forward-only.** The 17 existing records (18 including #13472) keep their names; the corpus rename is filed as its own task | **Bundle a rename backfill.** Rejected on PR scope: the writer change is a format string, the backfill is a script performing graph writes — two units, two PRs. §6.3 bounds the cost of waiting: the exclusion is live, so the stale names are a search nuisance and not a loop defect until #13238 relaxes it. Precedent: #13238 §9.6.5 already ruled forward-only for `block`. | **Zero.** Names are recomputable from stored content at any time — every existing record carries `answer` and `stopReason`, and `PATCH /name` re-embeds. Running it later costs exactly what running it now costs. |
| **D-4** | **No model call at write time.** If the answer-opening rule degrades, the **condense pass** reshapes names later | **Generate a title with a model call during write-back.** Rejected: it puts a model call and a new failure mode on the write path, makes the name non-deterministic and unreproducible from the record, and duplicates a capability the condense pass (#13335) already owns off the turn's critical path. | The condense pass is the escalation path and already exists |
| **D-5** | The name carries **no subject id** | **Include `#<subject>`.** Rejected under DRY: the subject is already the record's link, its content field, and its substance line 2 — a fourth copy. **[R2] The *"weak ranked text"* claim is an inference, not a measurement (P-51):** nothing here measured how a bare `#10422` ranks against eight characters of prose under `gemini-embedding-001`. The DRY argument stands without it — a fourth copy of a field the record already carries three times is refused whatever it ranks like — and that is the reason to prefer. | Trivial |

---

## 10. Quality Attributes & Trade-offs

| Attribute | Effect |
|---|---|
| **Retrieval — positive half** | The outcome moves from **outside the embedded window entirely** (character 69,261 of a 7,880-character budget) to the position the composer guarantees. §4.3 A10 is the evidence for what that position is worth: text existing *only* in a 118-character name reached **rank 2 of 11,044** on a query alien to the whole graph. |
| **Retrieval — negative half** | Crowding is **reduced, not eliminated**, by a measured 0.0374; rank 1 → rank 5 in §4.3's conditions. **No name change can eliminate it**, because the input is at content offset 1, inside the window. Eliminating it is the field-order work of §13. Stating this plainly is the point: the alternative is a design that claims a fix it does not deliver. |
| **Human legibility** | A list of run records becomes a list of *what happened* instead of a list of *what was asked*. Two runs on the same input become distinguishable in a listing for the first time. |
| **Determinism** | Preserved. The name is a pure function of the record and the write instant, exactly as today, and reproducible from stored content. |
| **Complexity** | One function, one helper, one renamed constant. The rune bound, the truncation helper, the prefix and the timestamp format are all reused unchanged. |
| **Risk taken deliberately** | The name's quality now tracks the answer's quality. A run whose answer opens with boilerplate gets a boilerplate name. Accepted, for the reason in §11.2 — and it is **self-announcing**, which is a good property in a failure mode. |

---

## 11. The universal, and what would falsify it

#13261's fourth question: *does this generalise?*

### 11.1 The claim

> **For a content-bearing node, the name is the highest-weight-per-character ranked text the node has. Spending it on text the node's own content already carries buys a second copy of a match the node would win anyway; spending it on text that other nodes own better makes the node compete with them.**
>
> **[R2] Its gate, inside this block rather than two subsections away (#12958 §18.6 clause 2, which requires the falsifier in the same passage that states the account).** The non-differential half — *names are embedded and rank* — is known from #440 and is not in dispute. The differential half, which carries all the explanatory content, is that **the name's marginal contribution is disproportionate to its character share**. **The gate:** measure name-only lift against content-only lift for the *same text*, with a floor arm; **if the name's marginal contribution is at or below the content copy's, this claim is a restatement of #440 and explains nothing new.** It was run — §4.3 is that gate — and it fired in favour: **+0.0374 against +0.0166 at the margin**, on 1.4% of the probe's embedded characters. *Hedge, in this same block, because this blockquote is the part that will be quoted onward:* one query, one graph snapshot, one embedding model, one name length against one content length. §11.3 records the gate's provenance; the gate itself now travels with the claim.

**Bounded, explicitly.** It applies to **content-bearing** nodes only. For a group, project, or person node the name is the *only* embeddable surface — DiVoid's name-only branch exists precisely for them (#440) — and there the name must restate the subject, because it is all there is.

### 11.2 Why the answer in a name is not the same defect

An answer is the run's **own product**, not another node's prose. A query that matches it is a query the record legitimately answers. This is Toni's memory ruling applied directly: *self-produced is not worth less than memory of other agents; the question is whether the shape carries substantial information.* A record whose answer is pure quotation is a record that should not have been produced — that is #13238's exclusion question, not a naming question.

**[R2] But the claim that follows from that — *"the record outranking a source node on its own conclusion is retrieval working, not crowding"* — is a negative claim about a defect, and it shipped without a falsifier. That is this design's own rule (#12935) landing on this design, and the objection is concrete rather than theoretical.**

An answer is synthesised **from the admitted candidates**, so it is not independent of them. On the live specimen the answer opens *"The text of the pull request body is owned by the orchestrator"* — a near-restatement of its **top admitted candidate**, #10192 *"Artifact Ownership — one artifact, one maintainer"*, admitted at 0.781. So **the new name will frequently paraphrase the source node the run drew on**, and the record will then compete with that node on outcome-shaped queries. That is a **new crowding surface manufactured by this change**, in exchange for the one it removes.

It also sits awkwardly against the evidence §11.4 invokes: #12958 measured that a graph whose conventions produce **title-paraphrasing documentation** retrieves that documentation rather than answers. This change makes more title-paraphrasing nodes.

**The argument above is still the right one — a record that concluded something should be findable by that conclusion, even next to the node that taught it — but it is an argument, and F-6 in §12 is now the measurement that can take it away.** F-1…F-5 covered boilerplate names, the positive half, input crowding, the DiVoid guard and prefix parsing; **none of them covers *records now crowd their own sources*,** which is the risk this section creates. **[R3] F-6's threshold is set where the blessing in that sentence stops (W-3):** *ranking next to* the node that taught it is the outcome this section blesses, so a falsifier firing there could not tell the blessed case from the defect. F-6 fires when the source is **displaced out of recognition** on its own subject — #13274's **skew**, not a one-rank swap.

### 11.3 The gate's provenance (#12958 §18.6 clauses 2–3)

**[R2] The gate itself has moved into §11.1's blockquote**, where clause 2 requires it — *in the same passage that states the account, not in a later section* — because that blockquote is what gets quoted onward and it was travelling without its hedge. This subsection now records only where the gate's numbers came from, which is the part a reader checking the work needs.

**§4.3 is the gate.** Four arms, one query, one graph snapshot of 11,044 nodes, run 2026-09-10; both populations named in §4.3 itself per clause 5. Name-only **+0.0898** over floor against content-only **+0.0690**; at the margin **+0.0374** against **+0.0166**. Every figure re-derived independently to seven decimals in #13505 §1.4, from a byte-identical regeneration of the probe bodies — the generator was seeded and preserved, which is what made a re-run possible after the probe nodes were deleted. **Keep seeded generators and preserve them; a deleted probe is unauditable otherwise.**

### 11.4 Graph-wide, this is partly already measured — and #13261 slightly overstates the gap

#13261 says *"nothing has measured how widespread that is."* That is **very nearly** true and worth correcting precisely, because the near-miss is load-bearing. **#12957 / #12958** measured the same phenomenon from the query side and states the surviving result as generic. **[R2] The citation is split, because the figures live in two sections and quoting them as one produced a blockquote that exists verbatim in neither (W-9):**

> **§18.2** — 4 of 6 work-titled anchors and **6 of 6** place-titled retrieved title-paraphrasing documentation ahead of answers; median excess **+0.0152** and **+0.0245** respectively.
>
> **§18.8**, summarising — *a title-derived query, in a graph whose conventions produce title-paraphrasing documentation, retrieves that documentation rather than answers*, at **2.5× the normalised excess** for place titles.

That measured *"a query built from a name retrieves nodes whose names paraphrase it."* #13261 asks the converse: *"a node whose name restates its subject competes with the nodes about that subject."* Same phenomenon, opposite end, and the first is strong evidence for the second — but it is **not the same measurement**, and this design does not treat it as one.

**So it is filed, with its gate, and no instrument is commissioned against it** (#12958 §18.6 clauses 1 and 4). The pre-test is in §13.

---

## 12. Risks, and the falsifiers that catch them

| # | Falsifier | What it kills |
|---|---|---|
| **F-1** | **[R2] Corrected — it must be measured on the clause, not on the name. [R3] And over the right population (W-2).** Of 20 records **that took branch 1** — that is, records carrying a non-empty answer after collapsing (§7.2) — take each name **from rune 37 onward** — the outcome clause, after `processor-run`(13) + space(1) + RFC3339(20) + ` — `(3). If **more than a third of those clauses share their first 30 runes with another**, A-3 is false. **Branch-2 records are excluded from numerator and denominator alike**, because two of them with the same terminal reason and call count share their first 30 runes *by construction* (§7.2's grammar), and A-3 claims nothing about a run that produced no answer. | A-3 is false — answers open with preamble, not conclusion. Take D-2's alternative: the terminal-state clause becomes the primary branch. Cheap to check: list the branch-1 names, slice at rune 37, compare prefixes. **It watches; it does not gate** — see below. |
| **F-2** | A query describing one record's distinctive outcome does **not** return it in the top 20, for a record written after the change. | The positive half failed. This is #13274's exact failing query shape and is the design's main acceptance test. |
| **F-3** | A verbatim repeat of an input returns **as many or more** prior records at the head as #13261's 13-of-30 baseline. | The negative half failed. *Hedge:* comparable only once several records exist under the new name, and against the same query family. |
| **F-4** | **Guard.** DiVoid changes `EmbeddingInputComposer.MaxLength`, or the composition order, or begins embedding `substance`. | §4 and §5 are invalidated silently — nothing in the graph would announce it. Re-run the §4.2 bracket (two probes, one query). **Note what survives:** if the cap grew to cover a whole record, §5.2/§5.4 weaken but **D-1 does not** — a name restating the input is still a second copy of a field the content carries either way. |
| **F-5** | A run record's name is found being parsed past the prefix by any consumer. | I-2's contract is wider than §7's inventory found. Checked at write time (§7 inventory: six consumers, all prefix-only). |
| **F-6** | **[R2] New — the risk this design creates (W-1). [R3] Threshold set at unrecognisability, not at one rank (W-3).** Take a record whose answer restates one of its admitted candidates (the live specimen #13472 is one: its answer opens on #10192's subject, admitted at 0.781). Query that candidate's **subject**, not the record's. **It fires when the source node is no longer recognisable on its own subject** — it drops out of the top 20, or out of admission, with one or more records of this shape above it. **A one-rank swap does not fire it:** §11.2 blesses a record ranking *next to* the node that taught it, so a threshold set there could not tell the blessed outcome from the defect. | §11.2's argument that this is *"retrieval working, not crowding"*. **It can still come back negative, which is what makes it a gate** — if the source stays recognisable, §11.2's concern is answered by measurement rather than by argument; **[R3] and its positive branch now says something too** — a source displaced out of recognition is #13274's skew reproduced by this change against the *sources* rather than against the records, and D-1 would have to be reconsidered. Cheap: one query per specimen, once records exist under the new name. |

### 12.1 [R2] Why F-1 was inoperative, and what kind of broken it was

The original F-1 read *"more than a third of names share their first 30 runes."* **A run record's name has no outcome in its first 37 runes** — §7.1's own grammar puts `processor-run`(13) + space(1) + RFC3339(20) + ` — `(3) there, verified against the live specimen (118 = 37 + 80 + 1). So the first 30 runes are `processor-run 2026-09-10T13:21` for **every** record, and the test read the timestamp.

**The precise defect is worth stating, because it is the more dangerous of the two possibilities.** The test was not one that *always* fires; it was one that **almost never can**. Two names share their first 30 runes exactly when the two runs were written **in the same minute** — the 30th rune falls inside the timestamp's minute field. Runs are far more than a minute apart in practice: the graph's newest record before #13472 was #13155 at **2026-09-07**, three days earlier. So the check would return *"0% shared"* on essentially any real sample and be read as **A-3 confirmed**.

> **A falsifier that cannot fire silently certifies the assumption it was written to retire.** That is worse than one that always fires, which at least announces itself the first time it is run. A-3 is this design's only unmeasured assumption and stands at n=1; it would have been carrying a certificate issued by a test that never examined it. This is exactly the failure #12958 §18.6 exists to prevent, and §11.3 invokes that standard by name.

**Gates or watches: it watches.** F-1 cannot gate this change, because the twenty records it needs do not exist until the change ships. The run rate is the problem: eighteen records exist in total, the newest is #13472 (2026-09-10) and the one before it is #13155 (2026-09-07). **Twenty records under the new name is not a near-term horizon**, so a falsifier that had to fire before shipping would block indefinitely. So the shipping decision rests on D-2's fallback branch being **correct in itself** rather than on A-3 holding: branch 2 (§7.2) is fully specified, is measured against #13274's exact query, and is what a boilerplate-opening answer would want anyway. **If F-1 later fires, the change is a reordering of two branches that both already exist and are both already tested** — which is why its reversal cost in §9 is one helper.

**Failure modes considered and accepted:** two runs in the same second collide on timestamp — already true today, unchanged. Empty-answer records share a near-identical clause and will mutually crowd — accepted, the population is small and pathological, *"show me runs that ended empty"* is a legitimate query for which mutual retrieval is the wanted behaviour, and each still has a unique timestamp.

**[R3] That same near-identity is why F-1's population is scoped to branch 1 (W-2).** The crowding half was already accepted above; the half not stated was that those clauses also sit in F-1's *numerator*. Seven answerless records in twenty would fire F-1 on boilerplate — on the one thing A-3 says nothing about. It is the same class of defect as the offset error this subsection exists to record: **a statistic measuring something other than the assumption it names.** It was filed as a warning rather than a fail because it misfires conservatively — toward D-2's terminal-state branch, which is safe — and because F-1 only watches; it is closed here because the fix is one clause.

---

## 13. Follow-up work, filed rather than bundled

Two tasks, both out of scope here for reasons stated in §2, each independently valuable:

1. **The 8,000-character window is 93% other nodes' names** (§5.3). The record's field order decides what gets embedded, and `candidates` — a list of *other* nodes' names, ids and hashes — occupies 7,345 of the 7,880-character content budget. This is the larger lever and the one that would let a record be found by its outcome through its *content* as well as its name. It changes the record's serialization, which is #13238's surface.
2. **The graph-wide generalisation pre-test** (§11.4), with its differential gate written into the task: take nodes whose names restate their subject and nodes whose names state something unique; for each, query its own name and compare the rank it achieves against the rank of the node that *owns* that subject. **If both classes displace subject-owners at the same rate, the effect is generic name weight and the account explains nothing new** — which is the outcome that must be able to come back, or it is not a gate.

---

## 14. #1136 §5 Pre-Design Checklist — walked

**KISS / DRY / YAGNI**
- No new type. No new abstraction. One function's body changes and one unexported helper appears.
- No element justified by "we might need X later": D-4 explicitly refuses the model-generated title and names the existing pass that would own it.
- No deprecation period, flag, shim, or transition window. D-3 is forward-only, which is a decision with a stated reversal cost, not a transition window.
- **[R2] Extract-vs-inline, run properly on the one element this design adds (W-10, #114 §0's KISS trigger — *"can the new helper be three lines at the call site?"*).** The honest answer is **no, and here is the arithmetic**: the clause builder holds a two-branch decision, a whitespace collapse, an empty-after-collapse re-test, a three-part terminal-state format with its own special case, and a rune truncation. Inlined, `runName` becomes a function that both *decides what the name says* and *formats it* — two responsibilities in one body, and the branch that #13274's acceptance query targets would be buried inside a format expression where no test can reach it in isolation. **The extract is justified by testability, not by reuse**, and that distinction is stated because "it has one caller" is otherwise a correct argument for inlining it. It stays one unexported function in the same file — not a type, not an interface, not a package.
- **[R2] Consumer-chain recursion box (W-10).** Walked: the new name is derived from `record.Answer`, and nothing downstream derives an answer from a name. `RenderSummary` reads the record, not the node; `IsRunRecord` reads the prefix. **There is no path by which a name feeds a later record's name** — the one path that could exist, a record's name entering a later record's `block` or `candidates` and thence its answer, is cut by the self-produced exclusion (§6.3) and would in any case be the exclusion's problem, not the naming rule's. Recorded because #13274's latent-recursion point makes this the right question to ask of any change to a record's own text.
- **[R2] No-multi-paragraph-keep-rationale box, against §6.2 (W-10).** §6.2 argues at length that the input is de-duplicated rather than removed. Checked against the box: this is not a rationale kept in place of a decision — the decision is D-1, stated in one row with its cost and its reversal. §6.2 exists because *"the input is dropped"* is the sentence a reader will carry away and it is **false as stated** (the input survives in content at offset 1 and in substance line 2), so the paragraph corrects a misreading rather than defending a choice. It stays.
- **DRY applied to the artifact itself:** §6.2 is the DRY argument — the name was a second copy of a content field.

**Existing systems first**
- Audited: `RenderSummary` already owns the human-facing projection and already carries the input at 200 runes; the name does not need to. `IsRunRecord` already owns self-produced detection by prefix; the name's tail is free to change.
- No new layer proposed.
- No new persisted data point proposed. The name already exists; its derivation changes.

**Configurability**
- No new knob. I-4 keeps the rune bound a `const`; no operator and no environment differs on it.
- The existing `runNameInputRunes = 80` is **renamed, not re-tuned** — reusing the bound rather than inventing a second magic number.

**Less is better**
- Can-it-be-deleted: word-boundary truncation was proposed and deleted (I-5). The subject id was proposed and deleted (D-5). A model call was proposed and deleted (D-4).
- Trade-offs named explicitly: §10, and the crowding half is stated as **reduced, not eliminated**, with the number.
- Reader inventory covers **both** AST references and string literals: `IsRunRecord`, `client.go:185`, `cmd/condense/adapters.go:34`, `internal/eval/result.go:162` (AST), plus the string literals `"processor-run"` at `internal/loop/summary.go:24`, `scripts/compare.py:68`, `scripts/smoke.py:47` and the test fixtures at `client_test.go:235/237`, `result_test.go:135/137`, `self_poisoning_test.go:51`, `summary_test.go:29/30/214`. **All key on the prefix; none parses past it.**

**Data deliverables** — none. No schema, no migration, no backfill in this unit (D-3 defers the only candidate).

**Document discipline**
- Cites #114, #1136, #11034 as load-bearing (header).
- Out-of-scope items listed explicitly (§2), not merely absent.
- No predecessor design is superseded end-to-end by this one.
- **[R2] Three corrections are carried to their origins, not merely stated here (P-43 corrected in place at origin, P-52 sweep the sites).** The first revision said this document *"corrects two claims in place"* — but *in place* meant *in this document*, which is the one place a reader of the corrected documents will never look. A correction that lives only where it was discovered is a disagreement. The sites, and what each now carries:

| origin | what this design falsified | correction filed |
|---|---|---|
| **#13238 §9.5, reason 4** (`docs/architecture/what-goes-in-the-block.md`, on `main` at `860c0e7`) | Both clauses: `block` cannot displace identity in the embedding (it starts at 8,360 against a 7,880 budget and contributes zero), and removing it does not bring `answer` inside (8,360, still outside) | **Dated correction appended at the site**, striking reason 4 and stating in the same passage that **reasons 1, 2 and 3 are untouched and removal of `block` still proceeds.** #13238 named this test itself — *"the test is to write one such record and re-run the distinctive-outcome query"* — and it has now effectively fired, by arithmetic on a real record. Repo file and node #13238 both updated, parity verified. |
| **#13274** — *"its embedding is 73% other nodes' bodies"* | The 73% is true of the record's **stored bytes** and false of its **embedded text**, where `block`'s share is **zero** | **Dated correction appended**, and the thesis **strengthened**: the window is **93.2%** other nodes' material (the `candidates` list), not 73%. |
| **#13274** — *"removing `block` is what makes a run record retrievable as itself"* | Does not hold for a record of this shape (§5.4.1) | **Dated correction appended**, scoped to the narrow margin it is — 20 candidates, 1.2 entries wide. |
| **[R3] This document §5.4.1, #13238 §9.5 and #13274's Correction 2** — *"20 entries occupy 7,825 characters, a mean of 389.6"* | The entries occupy **7,790**, mean **389.5**; 7,825 is the whole `candidates` field including its separating comma, key and brackets. The halves could not both be true: 20 × 389.6 = 7,792 ≠ 7,825 | **Struck in place with the corrected figure beneath, at all three sites** (#11228 Lesson 3 — a figure that vanishes leaves a reader who quoted it no way to learn it was wrong). Sweep run over `7,825`, `389.6`, `391`: **three sites, all three corrected.** The derived conclusion (1.23 entries) is unchanged and nothing else depends on the figure. #13505's round-1 quotation of the same span as *"a mean of 391 characters each"* is a review node and is corrected by its author at #13511 §8. |

  **What is deliberately *not* corrected:** #13274's crowding-vs-skew distinction, which is its most valuable contribution and is untouched; its latent-recursion point; and #13238 §9.5 reasons 1–3. **A correction that takes more than it measured is its own defect**, and the two documents' theses both survive.

---

## 15. Implementation Guidance — ordered

No code below; these are architectural units in dependency order. One PR.

1. **Rename the bound.** `runNameInputRunes` → **`runNameOutcomeRunes`** (**[R2]** the constant is named, per CF-3), value **80** unchanged, staying in the same `const` block in `internal/divoid/write.go`. Nothing else touches it.
2. **Add the clause builder** — one unexported function in `internal/divoid/write.go` taking the record and returning the outcome clause per §7.2's two branches, holding I-1 (whitespace collapsed, single line), applying the empty-after-collapse test before choosing a branch (§7.2, W-4), emitting branch 2 in the literal grammar §7.2 tabulates, and reusing **`truncateRunes`** unchanged (I-5).
3. **Change `runName`** to compose prefix + timestamp + clause. The signature does not change; it already receives the whole record.
4. **Update the two name assertions** in `internal/divoid/write_test.go` — the exact-name test at line ~243 and the truncation-bound test below it. Per the repo's pinning discipline, pin the **literal expected name**, not two substrings.
5. **Add tests for the second branch**, each pinning the **literal expected name** against §7.2's grammar: a record with an empty answer and a terminal reason of `answered` (expect the `answered but the answer was empty, …` clause); a record that hit the call cap (`cap reached`); a record with a **whitespace-only** answer, which must fall through to branch 2 and must **not** produce a dangling separator (**[R2]** W-4); a record with a multi-line answer (I-1); a record whose answer exceeds the rune bound (ellipsis, from `truncateRunes`).
6. **Verify the prefix contract** — a test asserting `IsRunRecord` still recognises a name produced by the new rule. This is I-2 made executable and is the cheapest guard against F-5.
7. **Do not touch** `RunNamePrefix`, `summaryRunPrefix`, `RenderSummary`, the record's JSON shape, or the exclusion.

**Deliberately not in this PR:** the corpus rename (D-3), the field-order work and the generalisation pre-test (§13).
