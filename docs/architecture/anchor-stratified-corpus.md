# The anchor-stratified corpus — pre-registration

**Status: pre-registered, never run.** This document and the corpus file it describes were written by an
author who did not execute a sweep over the corpus, did not observe any row pass or fail, and does not know
any outcome. Whoever measures it will be a different author with a different context. That separation is the
instrument's only structural defence, and §7 states exactly what it protects.

The corpus is `internal/eval/corpus-anchor.json`. It is a **second** corpus, not an extension of
`internal/eval/corpus.json`. §1 says why.

---

## 1. Why this is a second corpus and not a bigger first one

Three independent reasons, any one of which is sufficient.

**The two corpora measure different things, and one rate over both would blend two populations.** The
existing corpus varies the input and takes whatever anchor each row happened to be authored with. This one
holds the input fixed and varies the anchor. A combined retrieved-or-admitted rate would average a
population where the anchor is a nuisance variable with one where it is the treatment, and no reader of that
number could tell which half moved. Provenance was shipped precisely so that a rate can be attributed; a
merge would spend it.

**The existing corpus's anchor classification exists only after the fact.** The 11/17 place/work split on
those rows was derived by re-partitioning runs whose outcomes were already known. Writing that split into
the same column as a classification made before any run would make a contaminated label and a
pre-registered one indistinguishable by inspection — and the contamination is the exact defect this corpus
exists to avoid.

**The existing corpus file must not move.** Its sha256 identifies the baseline that the A/B rests on, and
re-hashing it to add a column would de-identify that baseline for a change that buys the baseline nothing.
Its rows also carry a burn: a set of them have been seen to miss, and rows seen to miss may not be
regenerated. A separate file keeps the burn boundary at a file boundary, where it is visible.

The existing corpus keeps its role unchanged: the harm guard, with its control stratum, at its current hash.

## 2. The stratification — matched pairs on the anchor

Every input appears **twice**: once anchored on a **place-titled** node and once on a **work-titled** node,
with the same required nodes both times. The two rows form a **pair**.

**Proportion: exactly 50/50, by construction rather than by tally.** Sixteen pairs, thirty-two rows, sixteen
place-titled anchors and sixteen work-titled anchors. The split cannot drift as rows are added, because a row
can only be added as half of a pair.

**Why pairing rather than two independent groups.** The prediction the corpus exists to test is a claim about
*difference between the strata*. In an unpaired corpus, input difficulty varies across the strata and is not
observable, which is what made the existing corpus's accidental 11/17 split uninformative about the anchor.
Pairing removes that variable exactly: within a pair the input, the required nodes and the answer key are
identical, so any difference in what retrieval delivers is attributable to the anchor or to noise, and to
nothing else about the row.

**What pairing costs, stated so nobody has to discover it.** The two rows of a pair are not independent
observations. An aggregate rate over all thirty-two rows counts each input twice and its confidence interval
is not the interval of thirty-two independent draws. **Report this corpus per stratum or per pair, never as
one pooled rate**, and do not compare a pooled rate from this corpus against a rate from the existing one.

**Two projects, by construction.** Eight pairs are anchored in this repository's own project and eight in an
unrelated backend project, so cross-project crowding is present in the population rather than in whichever
rows happened to attract it.

**Every row is anchored on a different node.** Thirty-two rows, thirty-two anchors. The complaint against the
existing corpus is not only that its split was accidental but that six of its rows share one anchor, so a
single node's neighbourhood carries a quarter of the evidence. Distinctness is a property of this file rather
than a rule the loader imposes — a future corpus could legitimately repeat an anchor — so it is pinned by the
test that loads the shipped file, not by `Load`.

**No control stratum.** A control row is constructed to be retrieved on every sweep; it has no anchor whose
title class means anything, so admitting one would put an unstratified row inside the stratified population —
the blend §1 rejects, one level down. The control stratum stays in the existing corpus, where it works.

## 3. The classification rule, applied to the title alone

The class is decided **from the anchor's title, without opening the node**, before the row exists.

- **work-titled** — the title names a bounded episode: a task, a defect, a pull request, a review verdict, a
  run, a fix round, a session log, a ruling on one unit of work.
- **place-titled** — the title names something durable that can be returned to: a directory, a file, a
  package, a project, a repository map, a standing convention, a concept, a measured property of the
  substrate.

**The deciding test is one question: could this title sensibly be marked done?** If yes it is work-titled; if
no it is place-titled. The test is what makes the two classes decidable rather than a matter of taste, and it
resolves the cases that a keyword rule cannot:

- A **concept**-titled node is place-titled. The account being tested defines the class as *a durable place
  **or subject***, and a concept is a durable subject. This is stated here rather than settled row by row.
- A **design document or ruling** titled for a unit of work is work-titled; one titled for a standing rule is
  place-titled. "Could it be marked done" separates them.
- A title that opens with a file or service name but goes on to assert a defect is **work-titled** — it names
  the defect, and the defect can be fixed. Leading with a place name does not make a title place-titled.
- A file-node title carrying a parenthetical work reference is still **place-titled**; a file cannot be done.

**Titles the test cannot decide are excluded from the corpus, not given a third class.** The classification
is the experimental variable and it stays binary. The consequence is that the two-way split is exhaustive
**over the rows this corpus contains**, and is not a claim that every node in the graph is one or the other.
That restriction is the honest form of the split, and it is recorded here so the restriction is visible
instead of silent.

## 4. The fields this corpus adds

Three fields on a row, beyond what the existing corpus carries.

| Field | What it is |
|---|---|
| `anchor` | the class, `place` or `work` — **the experiment**, recorded before anything else exists |
| `anchorTitle` | the anchor's title **as it stood when the class was decided**, pinned verbatim |
| `pair` | the pair this row is one half of |

`anchorTitle` is not decoration. The classification is a human judgement, and a judgement can only be
audited against the thing it was made from. Node titles are human-authored prose and are freely edited, so a
later reader who fetches the live title may be re-deriving the judgement from different words than the
author saw. Pinning the title makes a disagreement adjudicable: either the reader disputes the judgement, or
the title moved, and the two are distinguishable.

## 5. What the loader enforces, and what it cannot

An unvalidated hand-typed field drifts silently. A closed set is necessary and is not sufficient, because it
catches only one of the two ways this field goes wrong.

**Enforced offline, with no graph and no credential:**

1. `anchor` is `place` or `work` — closed set. Catches a value outside the vocabulary.
2. **All-or-none**: either every row in the file carries a class or none does. A half-classified corpus would
   report a stratified rate over a population that is only partly stratified.
3. `anchorTitle` is present and non-empty on every classified row.
4. `pair` is all-or-none across the file, on the same reasoning as 2.
5. **The pair invariant**: a pair holds **exactly two** rows; their classes are **one `place` and one
   `work`**; their inputs are **identical**; their required node sets are **identical**. This is the check a
   closed set cannot do — it catches a class that is *wrong but spelled correctly*, which is the ordinary way
   a hand-typed field fails.

**Not enforced, and named so nobody assumes otherwise:**

- **A pair whose two classes are consistently swapped passes every check.** The invariant sees one of each;
  it cannot see which is which. The mitigation is `anchorTitle`, which lets a second reader re-derive both
  classes from the pinned words without trusting either field.
- **`anchorTitle` cannot be checked against the live graph offline.** Confirming that a pinned title still
  matches the node needs a credential and a fetch, which is a different guard from a file linter — the same
  split that already separates *malformed* from *stale* for the required-node hashes. It is not built here.
- **A required node's hash moves whenever a peer edits that node**, from another session, without seeing this
  repository. Rows requiring frequently-edited nodes will go stale by design; that is the format's trade, not
  a defect in these rows.

## 6. The authoring protocol

1. The anchor's title was read and classified first, under §3, from the title alone.
2. The input was written as an **imperative, project-situated instruction** — work on this codebase, of the
   shape the product is actually given — and never as a self-contained question.
3. The required nodes were chosen as what a correct answer must surface, and the reason records what an
   answer lacking them would get wrong. Each `why` names a specific wrong conclusion, not a topic.
4. Each required node's content was fetched and hashed **at authoring time**, against the node's content as
   it then stood, and never copied from a figure recorded elsewhere.
5. **Neither anchor of a pair may state the answer.** An anchor is rendered into the block whole, so what a
   row gets for free is the anchor's prose, not its identity. Both anchors of every pair were fetched and
   swept for the required node's identifier and for the distinctive wording of its thesis; five candidate
   anchors were rejected on that sweep and replaced. Details in §9.
6. No row regenerates a row previously seen to miss. None of the required nodes of the burned rows appears
   here, and no input restates one.
7. No sweep was run, and no row was observed to pass or fail.

## 7. What is pre-registered

Everything in the corpus file, and specifically: the thirty-two rows, their inputs, their anchors, **the
class of every anchor**, the required node of every row, and the content hash each required node carried when
it was labelled — all fixed before any measurement exists.

The reason the classification in particular is pre-registered is that the account this corpus tests was
generated from data by someone who classified anchors **after seeing the outcomes**, and said so. A corpus
whose author could watch rows pass or fail while authoring would inherit that defect one level up, selecting
rows on how they behave without meaning to. Separating authoring from measurement is the only structural fix
for it, which is why it is a constraint on the author rather than a recommendation.

Any later change to an `anchor` value is a change to the experiment, not a correction to a row.

## 8. Uncontrolled covariates

Pairing controls input difficulty and the answer key exactly. It does not control everything, and the
remainder is named here rather than discovered later.

- **Anchor size.** The anchor's content is rendered into the block and is charged against the budget, so the
  two rows of a pair start from different remaining budgets. Anchors were kept within an ordinary range and
  outliers were rejected during authoring, but the covariate is real and is not balanced.
- **Anchor topical distance.** A place anchor for a directory covers a wider subject than a task anchor for
  one defect. Both are plausible things to hand an agent for the input, which is the bar that was applied;
  they are not equally specific.
- **Whether a work-titled anchor has a restatement family** — a design document or review whose title
  paraphrases the anchor's — is *not* recorded as a field, deliberately. It is a fact about the graph,
  mechanically re-derivable at any time by a query, and therefore not contaminated by having seen results.
  Pinning an author's guess at it would be strictly worse than deriving it when it is needed.

## 9. What the specification asked for that the graph would not supply

Recorded because it is a fact about the substrate, and the next instrument built on this graph will meet it.

**Place anchors are abundant and work anchors that fit are scarce.** For any topic, a directory, file or map
node exists and is neutral. The work-titled node that belongs to the same topic is very often *the task that
asks for the work*, or the review that already worked it out — and those state the answer. Every work anchor
here therefore had to be an **adjacent** unit of work rather than the obvious one: a prior round, a
neighbouring defect, the session log of the step before. That is realistic, and it means the work stratum is
systematically one hop further from its input than the place stratum is. It is the one asymmetry pairing
could not remove.

**Five candidate anchors stated the answer and were replaced.** Every one of them was the *obvious* choice
for its row, which is why they are worth naming as a class:

1. A file rollup carrying a live bullet that asserts the required node's finding verbatim, current as of the
   previous merge. **A directory or file rollup on this repository is a running summary of everything true
   about that file**, which makes it the most likely place for an answer to leak into a *place* anchor — the
   one class that looked safe.
2. A review verdict that raised the question the required node rules on, and sketched the remedy in the
   paragraph naming the constraints any fix must satisfy.
3. A review verdict that had reached the required node's rule independently, rounds before anyone wrote the
   rule down.
4. A sibling task filed out of the same investigation as the required node, carrying its headline sentence.
5. A closed defect task whose body was updated with the real cause after it closed.

The last two are the general form: **a work node keeps being edited after its work finishes**, so a task that
did not state the answer when it was filed may state it today. The sweep in §6 step 5 has to be run against
each anchor's *current* content, and its result does not stay true.

Three of the five are review verdicts or tasks — the *work* class — which is where the scarcity in the
previous paragraph comes from. The fourth and fifth are the reason the screening cannot be done from titles.

**The screening cost is not in the input.** Choosing an anchor pair took longer than writing the input and
the reason together, and almost all of it was verifying that neither anchor gives the row away. An instrument
of this shape does not scale by writing more questions.
