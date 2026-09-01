# Processor — a harness whose substrate is memory, not history

*Working title. Status: vision draft, 2026-09-01.*

---

## 1. The thing we are reacting against

Every agent harness in circulation — Claude Code, opencode, openclaw, Cursor's agent, all of them —
is architecturally the same object: **an append-only transcript with tools bolted onto it.**

The loop is:

```
transcript += user message
transcript += model output
transcript += tool result
... until context limit, then summarize and continue
```

Memory, where it exists at all, is a *tool the model may choose to call* — usually a directory of
markdown files with an index. It sits outside the loop. Nothing forces it to be consulted, nothing
forces it to be written, and its quality is whatever the model felt like doing that day.

This produces a specific and recognisable set of failures:

- **Contradiction accumulation.** A human thinks out loud. Turn 3 says "let's use Postgres", turn 40
  says "actually SQLite". Both sit in the prompt with equal standing. The model has no structural
  signal about which is *true now* — only position, which is a weak and unreliable proxy.
- **Attention dilution.** By turn 60, the relevant 2 KB is suspended in 200 KB of irrelevant
  scrollback: abandoned approaches, a stack trace from a bug that is fixed, a listing of a directory
  nobody has touched since. The model must *infer* relevance out of a haystack, every single turn.
- **Relevance by accident.** When the right fact is in context, it is usually because it was mentioned
  recently — not because anything determined it was relevant. Correct behaviour is a lucky draw from a
  large, noisy sample.
- **Memory as an act of will.** "Please remember this." "Did you check your memory?" The user ends up
  as the harness's scheduler, manually driving a subsystem that should be autonomic.
- **Unverifiable process.** "Run the tests" is an *instruction*, and the report that comes back is a
  *claim*. Agents skip steps and describe them as done. The harness cannot tell the difference.

The industry's answer to all of this is *more context*: bigger windows, better summarisation, prompt
caching to make the waste affordable. That is a scaling answer to a structural problem. The strategy
is "feed it enough and something right will happen" — and it works often enough to ship, which is
exactly why nobody has questioned the substrate.

## 2. The thesis

> **An intelligence without memory is not useful, and *how* the memory behaves determines what the
> intelligence can be. So build the harness around the memory instead of bolting the memory onto the
> harness.**

Three inversions:

**(1) Context is retrieved, not accumulated.**
Every input — a user message, a tool result, a webhook, a step completion — triggers a *mechanical*
retrieval against the memory graph. The context for the next model call is **assembled from what
memory says is relevant to this step**, not inherited from whatever happened to be said earlier. No
model call is spent deciding what to look up; the first lookup is deterministic code.

**(2) Memory is the substrate, not a tool.**
Reading memory is not a decision the model makes — it is the mechanism by which the model is invoked
at all. Writing memory is a step in the loop, not an act of virtue. The graph is not *consulted by*
the process; the graph *is* the process's state.

**(3) The harness is configuration-free.**
Briefings, agent roles, workflows, tool definitions, gates, policies — all of it lives in the graph as
nodes. The binary boots on a graph URL, a graph key, and the credentials for whatever else it must
authenticate to. Everything that is not a secret is data it reads from the substrate it already
depends on. No config file, no `.agents/` directory, no prompt folder drifting out of sync with reality.

> **Corrected 2026-09-01, at the first milestone that could test it.** This read *"exactly two things to
> boot: a DiVoid URL and an API key"* — a claim M1's design showed cannot hold. A model provider's key
> is a **secret**, and secrets never go in the graph, so the honest floor is three and it grows by one
> per credential. The *spirit* is intact and is the part that matters: no configuration files, no prompt
> folders, nothing to drift. But "exactly two" was a quotable sentence that would have been quoted, and
> the count was wrong. Recorded rather than silently rewritten, because the difference between a claim
> that survives contact with an implementation and one that does not is the whole point of writing them
> down.
>
> **Sharpened the same day, once the model call was ruled provider-agnostic.** The corrected list is not
> merely "more than two" — it is **complete and principled**, splitting boot inputs by a real property:
> those that *cannot* live in the substrate (the graph URL, because you cannot read the graph to find
> the graph; and secrets, which never go in it) versus those that merely *do not yet* (an endpoint
> address, a model name — not secrets, just not graph-driven at this milestone). Against a **local**
> model the third clause is empty, so the floor is genuinely two again in that case.
>
> One thing not to oversell: across the same milestone the **wired** members went *up* (4 → 6) while the
> **irreducible** ones went *down*. Two quantities moving in opposite directions, and this section is a
> claim about the second.

## 3. The loop

```
input (message | tool result | event | step completion)
   │
   ├─► [1] mechanical context assembly            ← no LLM
   │        working set (pinned ids)
   │      + episodic buffer (last k turns, small, verbatim)
   │      + semantic recall (vector) over the graph
   │      + graph expansion (n-hop from hits and pins)
   │      + lexical recall (identifiers, error strings, file paths)
   │      − superseded / invalidated nodes
   │      → ranked, deduped, budgeted context block
   │
   ├─► [2] step resolution                        ← no LLM
   │        which workflow node are we on? what does it require?
   │        does the next transition need judgement, or is it a tool run?
   │
   ├─► [3a] deterministic step → run it, capture the FACT (exit code, diff, test report)
   │   [3b] judgement step     → one model call: assembled context + that step's briefing
   │
   ├─► [4] gate                                   ← no LLM
   │        the transition's obligations are checked against facts, not claims
   │        fail → route back with the fact attached, not with an opinion
   │
   └─► [5] write-back
            what was decided, observed, produced → nodes + edges
            run record: exact context, candidate set with scores, what was cut
```

Steps 1, 2, 4 and most of 5 are code. The model is called where judgement is genuinely required —
which is far less often than a transcript-shaped harness implies, because a transcript-shaped harness
has no way to express "this part is not a thinking problem."

## 4. The five properties that fall out

1. **Context is right by construction, not by luck.** Something is present because retrieval put it
   there. If retrieval leaves it out, that is a *measurable* recall failure with a name and a fix —
   not a mystery.
2. **Context adapts per step.** Planning, implementing and reviewing the same task pull different
   neighbourhoods out of the graph. Today they all share one undifferentiated blob.
3. **Facts gate transitions.** "Tests pass" is an exit code the harness observed, not a sentence the
   model wrote. An agent cannot lie past a gate, because the gate does not read prose.
4. **The model is called only where intelligence is needed.** Routing, retrieval, verification,
   bookkeeping and state transitions are code. That is the cost story, and more importantly the
   reliability story.
5. **Every run is replayable.** Context assembly is deterministic given (graph state, input, policy),
   so a run can be reconstructed exactly. "Why did it do that" becomes answerable: *these nodes were
   in scope, at these scores, and these were cut for budget.*

## 5. The hard parts (where this actually gets interesting)

These are not caveats bolted onto a pitch. They are the engineering, and getting them wrong is how
the idea dies as a demo that impresses for ten minutes.

### 5.1 Deixis: half of human input carries no semantic content

"no, the other one." "that's wrong." "why?" "yes, do it." Embed those and you retrieve noise. Pure
retrieval on the literal input collapses in conversation — which is the medium.

**Answer: three-tier memory, not history-versus-retrieval.**

| Tier | Contents | Size | Mechanism |
|---|---|---|---|
| **Working set** | node ids explicitly in focus for this run | ~5–15 nodes | pinned; mutated deliberately by steps |
| **Episodic buffer** | last *k* exchanges, verbatim | small, fixed | ring buffer, never grows |
| **Semantic recall** | everything else | budgeted | retrieval over the graph |

History is not abolished — it is **demoted** from *the substrate* to *a small fixed-size buffer*, and
its job narrows to resolving reference ("the other one" → an id in the working set). That is the job
it is genuinely good at. The claim is not "history is useless"; it is "history is a terrible primary
index and a fine short-term scratchpad."

**The write-side counterpart matters as much as the read side.** Memory does not record history as it
was uttered — it *puts it into perspective*. Do not store "he said do the other one"; store *"Toni
chose approach B (#X) over approach A (#Y)"*, with the edge to say so. Resolve the reference at
write time, while the referent is still unambiguous, and the episodic buffer can stay tiny — because
nothing downstream ever has to re-derive what "the other one" meant.

> **Flesh out the substrate; do not just record.**

### 5.2 The new failure mode looks better and is worse

Today's failure is a confused agent with a messy context. Ours is a **confident agent with a clean,
tidy, plausible context that is missing the one node that mattered.** Messy contexts sometimes rescue
you by accident — the stray line of scrollback that happened to contain the answer. A precise context
has no such slack.

> **Precision without recall is more dangerous than noise.**

So mechanical retrieval is the **floor**, never the ceiling:

- **Hybrid retrieval by default** — vector + graph expansion + lexical + structural pins. Vector-only
  retrieval misses exact identifiers, error strings and file paths, which is most of what code work
  actually runs on.
- **The model keeps an escalation path** — it can issue supplementary recalls when the assembled
  context feels thin. Gangolf's `remember` / `apropos` actions already prove a model drives this
  usefully when given the affordance.
- **Recall is measured, not vibed.** A retrieval eval set — (input → node ids that must be in scope) —
  exists from day one, and every tuning change is scored against it. This is the line between a
  research toy and a system.

There is also a structural answer to the specific "stray scrollback line" case. If a line of
scrollback would have saved you, it was *about* the matter — it scored low, or it contradicted the
current framing, but in a well-built graph it still sits in that neighbourhood. So the assembler does
not only chase high-similarity hits; it also **traverses one hop into the demoted region** — nodes
marked superseded, obsolete, or explicitly decided against — and surfaces them compactly (see §5.4).
The accident becomes a deliberate traversal. This is only as good as the graph, which is why §5.3 is
the load-bearing subsystem.

### 5.3 The write path is the actual hard problem — and we already have the scars

This is where Gangolf (DiVoid #905 → #1317) is worth more than any paper. Empirically, across eight
rounds of prompt tuning against a live graph:

- **Auto-linking everything to hubs** (project, bot, every id encountered in the loop) made every
  memory reachable only *through* the hub. The graph became a star and stopped being a graph (#935).
- **Handing linking to the model** produced hub saturation — 45-way fans on a single node.
- **Guarding against saturation** converted hub-and-spoke into **floating orphans**: under pressure
  the model picked "link to nothing" as the cheapest legal option (#1317).
- **Proactive memorisation fires rarely** without constant prompt pressure (#930). Left to its own
  judgement mid-task, a model does not write memory.

The conclusion is structural, and it is the most valuable thing we own:

> **Write-time discipline cannot be fixed at write time.** The writing agent has neither the bandwidth
> nor the global view to also be a librarian.

**Answer: two-phase memory, borrowed wholesale from how brains do it.**

- **Write-time (episodic):** cheap, permissive, near-mandatory. Capture generously. Bad structure is
  acceptable; *not capturing* is not.
- **Offline (consolidation, "dreaming"):** an asynchronous pass, decoupled from any task, that reads
  what actually clustered and ratifies it — grouping saturated fans into thematic children, re-homing
  orphans, merging duplicates, marking supersession, pruning dead ends.

Consolidation is a **day-one subsystem**, not a follow-up. It is what makes the memory still good in
six months, and it is the part nobody else is building.

**A second rule falls out of the same evidence: agents emit observations, not nodes.** Every Gangolf
failure above happened because the *writing model* was made responsible for graph structure — node
type, placement, `linkTo` — as a side quest during a task it was already spending its budget on. So
the write API an agent sees should be almost ceremony-free: *say what happened, in your own words,
with whatever ids you have in hand.* A dedicated **memory core** is the only component that writes
structure: it decides type, placement, edges, dedup against existing nodes, and whether this is a new
node or an update to one. Structuring is a librarian's job with a global view, and it should be done
by the component that has one — synchronously where cheap, in consolidation where not.

Put bluntly: **if writing memory can produce a bad decision, it is the wrong API.** Throwing it in
must always be safe; making it well-formed is somebody else's problem, on purpose.

### 5.4 Contradiction needs first-class supersession

A transcript resolves contradiction by position: the later statement is visibly later. **A retrieval
result has no inherent time ordering** — the March decision and the August reversal come back with
equal standing and no tiebreaker.

So supersession must be structural, not inferred:

- directed `supersedes` / `superseded-by` edges (the vocabulary already exists in our conventions),
- validity intervals on decision-shaped nodes,
- and a retrieval rule that **demotes** superseded nodes rather than deleting them from view.

Without this, "no contradictions unless they are in memory" degrades into "all contradictions,
permanently, with no way to rank them." Correctness requirement, not a refinement.

**Demoted is not hidden — and this is the interesting half.** A superseded node must lose the
competition for context budget against the live one, but it must stay *reachable* from it. Knowing
what not to follow, and why, is worth as much as knowing where to go: an agent that re-proposes the
approach we rejected in March has cost more than one that merely lacks an idea. So the assembler
carries a **decision trail** — when a live node has a `supersedes` edge, it attaches a one-line
annotation (*"was: connection pooling in the client — rejected 2026-03, see #X"*) instead of the full
superseded body. Tokens: near zero. Effect: the whole rejected-axis region becomes visible without
being expensive.

> The graph's most valuable content is often **negative knowledge** — what we tried, what failed, and
> why we stopped. No transcript harness retains it past its window.

**On freshness.** Nodes carry created/updated timestamps, so recency is available — but recency is a
weak signal on its own, and using it as a global rank multiplier is wrong: an architecture decision
from a year ago routinely outranks a note from yesterday. Three rules:

1. **Edges beat decay.** An explicit `supersedes` is authoritative. Decay is only the fallback for
   when nobody recorded the supersession.
2. **Recency is a tiebreaker, not a ranker** — it applies *between competing claims about the same
   subject*, not across the whole candidate set.
3. **Half-lives are type-scoped.** A `bug` observation goes stale in days; a `documentation`
   architecture decision in years. One decay curve for all node types will be wrong for most of them.

### 5.5 Workflows must be gates, not scripts

Storing a workflow as graph nodes is right. Storing it as a *rigid state machine* is a trap: real work
branches in ways you did not encode, and a harness that cannot express "something unexpected happened"
degrades into an agent fighting its own rails.

**Answer: a workflow node declares obligations and affordances, not a path.**

- An **obligation** is a precondition on a *transition*: this edge is not traversable unless a green
  test run exists for the current tree state; unless a design document exists for a non-trivial
  change; unless the artifact being changed has an owner.
- An **affordance** is what is reachable from here: which tools, which sub-workflows, which roles.

The model chooses the path; the harness refuses illegal transitions.

> **The harness is a referee, not a script.**

That keeps the LLM as the intelligence while making the *process* mechanically trustworthy — which is
exactly the split the current generation gets backwards.

**A workflow is authored step-at-a-time, not as one document.** Each step node says what comes next
and under what condition: *do X; if the result is A continue with Y, otherwise Z*. Nothing needs a
global description of the whole process, and no step needs to be read before it is reached.

**Steps are hybrid by necessity, and the split is precise.** A step can name a tool — but parameters
are highly variable ("run the test suite for the projects you changed" cannot be a fixed command
line), so invocation is not purely mechanical. The clean division:

| | Who | Why |
|---|---|---|
| **Intent** | authored in the graph | "verify the change with tests" is stable and reviewable |
| **Invocation** | resolved by the model | which projects, which command, which flags — genuinely variable |
| **Result** | observed by the harness | this is where lying would happen, so it is the one part the model never authors |

The entire integrity guarantee lives in the third row. The model may *see* the result; the gate reads
the harness's own copy of it, never the model's restatement. Intelligence chooses what to run; it
does not get a vote on what came back.

### 5.6 Caching: the assembler must be append-mostly, not relevance-sorted

An append-only transcript is extremely cache-friendly: the prefix is stable, so most of it is served
at ~0.1× price. A freshly assembled per-step context looks like the opposite. The intuition that we
could "cache the nodes, since the same ones recur across a task" is the right instinct pointed at the
wrong mechanism — and the mechanism decides the design.

**How the cache actually works** (Anthropic API, verified 2026-09-01):

- It is a **prefix match**, not a chunk store. There is no way to cache node A and node B separately
  and get a hit when they are recombined in a different order. The key is the exact bytes of the
  rendered prompt up to a breakpoint; **one byte differs at position N and everything from N on is
  cold.**
- Render order is `tools` → `system` → `messages`. Up to **4** `cache_control` breakpoints per request.
- Minimum cacheable prefix is model-dependent (512 tokens on Opus 5, 1024 on Opus 4.8 / Sonnet 5,
  4096 on Opus 4.6 / Haiku 4.5). Below it, nothing caches and no error is raised.
- Reads cost ~0.1×; writes cost 1.25× at the 5-minute TTL and 2× at the 1-hour TTL. **At 5 minutes,
  break-even is two requests.** Steps within a task are seconds apart, so a shared prefix pays back
  almost immediately.
- The cache is **model-scoped**, so routing a cheap mechanical step to a small model forfeits the
  prefix for that step. Worth knowing before we route by cost.

**What that implies for the assembler — the actionable part:**

> **Order the assembled context by *stability*, not by *relevance*.**

Relevance-sorting is a silent cache invalidator of exactly the kind the docs warn about: the same
five nodes reshuffled by score every step produce different bytes, so every step is a cold write and
we pay the 1.25× premium forever while never reading. Instead, lay the context out in stability tiers
and place breakpoints at the boundaries:

| Tier | Contents | Changes | Breakpoint |
|---|---|---|---|
| 1 | identity, role briefing, tool definitions (deterministically sorted) | per role | ✔ |
| 2 | task briefing, working set, nodes that recurred over the last *n* steps | per task | ✔ |
| 3 | this step's fresh recall hits | per step | ✔ (carry-over) |
| 4 | the input itself | every request | — |

And the corollary that makes tier 2 work: **the assembler is append-mostly.** Once a node enters the
task-stable tier it keeps its byte position until it is evicted; it is not re-sorted when its score
moves. Ranking decides *entry and eviction*, never *ordering within a tier*.

**Stability is not achievable continuously — the goal is that invalidation is rare and batched.**
This is the part the "keep the prefix stable" framing hides. A prompt whose front edge moves *at all*
goes cold behind the move, so a naive rolling window that drops one message per turn is the worst
possible design: it pays a cold write every single turn and never reads. That is why nobody ships one
— transcript harnesses keep the prefix strictly append-only for as long as they can and then take
**one large invalidation event** (compaction, or context editing that clears old tool results) instead
of continuous small ones. Amortised, that is one cold write every *n* turns rather than every turn,
and the turn immediately after a compaction is the most expensive turn in the conversation.

**Why it cannot simply be chunked** (the obvious fix, and why nobody shipped it): the cache does not
store the *text* of a chunk, it stores the key/value vectors for its tokens — and those were computed
by attending over everything before them, at the positions they occupied. Evict an early chunk and
shift the rest, and the cached vectors for the surviving chunks describe attention over a prefix that
no longer exists. They are not stale, they are wrong. Game-engine streaming works because world
chunks are *independent*; attention is *cumulative*, so the property that makes paging work is absent.
Prefix-only is the shape of the math, not an oversight. (Research on relaxing it — position-remapped
or blended chunk caches — exists and generally trades accuracy or partial recompute; treat the
constraint as "what is offered today," not as permanent.) What is offered *is* a coarse chunking:
up to 4 breakpoints, with **graded** invalidation — a change after breakpoint 2 still reads everything
up to breakpoint 2. Nested chunks rather than independent ones, and the oldest can never be dropped
without killing everything behind it.

Hence the real strategy in a transcript harness: **do not slide — collapse.** Append-only for as long
as possible; when the front edge must move, compaction *replaces* the history with a summary rather
than shifting it, so the cold write afterwards covers a much smaller prefix. Long cheap stretches,
one modest expensive step, climb again.

**And this is where the assembled-context design wins outright, not merely ties.** The sawtooth is a
*growth* problem, and our context does not grow — it is assembled to a budget every step, bounded by
construction. We never approach the window limit, so **we never need compaction at all: there is no
cliff, ever.** Our only invalidation is a tier-2 re-base that we schedule ourselves and that is small
by design. A transcript harness must eventually detonate its cache; our worst case is a deliberate,
bounded re-base. Worth stating as an asset rather than burying it in a caveat.

One cheap piece of insurance follows: **our nodes are already natural chunk boundaries** — discrete,
stable-ordered, content-hashed. If position-independent chunk caching ever ships, an assembler that
renders clean deterministic node boundaries can exploit it immediately, where a transcript has nothing
to chunk *on*. Preserving that property costs nothing today.

The same discipline transfers directly, and it is what makes tier 2 realistic:

> Changes to the task-stable tier are batched into an **epoch**. Between re-bases the tier is
> byte-frozen and all movement happens at the tail. Evictions and new pins accumulate and are applied
> at a deliberate **re-base** point — never incrementally.

This also dissolves the "hole in the middle" worry — that 100 nodes recur but one dropped out of the
middle, punching a hole in the shared prefix. That problem only exists if the tier is *re-derived and
re-diffed every step*. An append-mostly tier is not re-derived, so nothing punches holes between
re-bases. If it ever does bite, the cheap countermeasure is **admit eagerly, evict lazily** — a node
that falls out of the relevance set is not removed on the spot; it decays out at the next epoch
boundary, so short gaps never fragment the prefix. Deliberately padding the context with irrelevant
nodes to preserve a byte run is the expensive last resort, and it trades away exactly the context
precision the project exists to get — measure before ever reaching for it.

**And lazy eviction is not only a cache trick — it is better cognition.** A node whose relevance dips
for one step should *degrade*, not vanish; it leaves only after failing to matter for several steps in
a row. That hysteresis buys the cache a stable byte run, but the reason to want it anyway is that a
context which reshuffles completely on every step produces a **thought process that reshuffles
completely on every step.** Continuity of attention is a feature in its own right; the cache
friendliness is the side effect. By design this architecture is cache-hostile — but a stable working
state is something we can offer it without giving up adaptivity, and that is the shape of the
compromise: not dodging the cache issue, just meeting it somewhere.

Two smaller notes with the same source: operator instructions arriving mid-run should be sent as
`role: "system"` messages appended to the message list (supported on Opus 5), not by editing the
top-level system prompt — editing the front of the prefix invalidates the whole conversation behind
it. And the honest measurement is `cache_read_input_tokens`; if it is zero across steps, something in
tier 1 or 2 is moving and the whole scheme is off.

**The cost conclusion stands, with the asterisk.** Even done well this probably does not save money in
proportion to the tokens saved — the transcript's stable prefix is very hard to beat on pure
economics. The bet is that a tighter, adaptive context produces **fewer correction loops**, and that
loops are where the real money goes. If it turns out to cost more per task and deliver better or
faster results, that is a *valid outcome and a business decision* — "do I want to pay more for result
X, or is the ordinary workflow enough?" — not a failed experiment. What matters is that we measure
both sides and can state the trade honestly.

Retrieval also adds round-trips. A per-step latency budget is a design constraint, not an afterthought.

### 5.7 Observability is the sleeper feature

Because the context is synthesised rather than inherited, we *must* persist something to debug
anything. But the naive version — dump the full assembled context at every step — explodes into
unreadable data within one task, and the cheap version — store the node ids per step — rots, because
nodes change after the fact and the list no longer tells you what the agent actually saw.

**Three retention tiers, each with a different job:**

| Tier | What | Retention | Job |
|---|---|---|---|
| **Narrative log** | "asked to do X, did Y and Z, because of these few nodes" | forever, tiny | tells the story; a human catching up |
| **Assembly record** | per step: node id + **content hash** + score + rank + included/cut | forever, small | forensics — and the hash is what makes it honest |
| **Full replay** | the verbatim assembled block | bounded window / opt-in | deep debugging of a specific bad step |

The content hash is the piece that fixes the rotting-id problem: it tells you later whether the node
you are reading now is the node the agent read then. Without it the record looks precise and quietly
lies. With it, the small forever-tier is enough for almost all forensics, and the expensive verbatim
tier can stay bounded.

The narrative log is a genuinely separate artifact, and it is not a debugging tool — it is the "what
happened and roughly why" story, which is exactly what a `session-log` node already is in our graph.

That obligation turns into the best feature in the product. Current harnesses can show you a
transcript. This one shows you a **causal audit trail** — *the agent did X because these seven nodes
were in scope at these scores, and this one was cut at rank 12 by the token budget.* For anyone
running agents on work that matters, that is the thing they cannot get anywhere else.

### 5.8 Open question: who mutates the working set?

The working set is the one tier with a genuine ownership fork, and it is load-bearing for
determinism, for caching, and for whether "memory as an act of will" stays dead.

**A — step-mutated (mechanical).** The harness admits and evicts from structural events: the workflow
step, the task node, artifacts touched, entities in the input that resolve to nodes, recall hits that
were actually used. Deterministic, so replay and the audit trail mean something; free in tokens and
latency; and mutation happens at known points, which maps exactly onto the epoch model above. Its
weakness is that **it cannot represent intent** — the model knows it is chasing the auth bug and that
#X is why, while a rule sees only file paths. It will therefore mis-pin precisely in the cases where
judgement mattered, with no channel to correct it, and the patch for each miss is another heuristic:
rule sprawl into a relevance engine nobody understands.

**B — model-mutated (deliberate).** The model gets `pin` / `unpin` and curates its own focus.
Expressive, handles "keep this in mind through the refactor" natively, and Gangolf shows a model does
drive this kind of action usefully. But it reintroduces the failure this project exists to remove:
discretionary bookkeeping inside a task that is already spending its budget. Gangolf's `memorize`
fires rarely without constant prompt pressure, and `pin` is the same shape of action. The other
direction is worse — models under-unpin, focus becomes a junk drawer, and tier 2 grows back into a
transcript with extra steps. It also puts nondeterminism in the exact layer we wanted deterministic.

**Resolution: A, with no exception — and the model steers by `intent`.**

An earlier draft of this section proposed a middle path where the model emits `focus(id)` / `release(id)`
as hints. That is discarded. It still required the model to see ids, track them, and choose among
them — bookkeeping wearing a different hat, and a violation of the premise that memory is the
foundation rather than a decision.

The mechanism instead: **the model's step result carries an `intent` field, stated semantically and
never in node ids.** *"It seems X plays a role here; I should look into Y."* That sentence becomes a
retrieval query for the next step's assembly, alongside the queries the memory layer derives
mechanically from the same result. The model says what it wants to think about; **the memory layer
decides what it gets to see.**

Why this is strictly better than a pin channel:

- **The model does only what it is good at.** It reasons in meaning, not in addresses. Handing it ids
  to curate is asking a language model to be a filing system; handing it a sentence is asking it to
  articulate a thought.
- **It costs nothing.** A field in a response the model is already producing — no extra call, no
  round-trip, no tool invocation, no mechanical lookup loop burning tokens to decide what to look up.
- **It is hard to skip.** A required field in a structured response gets filled; a discretionary
  `pin()` action does not. This is the precise mechanism that failed in Gangolf, and the schema
  sidesteps it.
- **It matches the substrate.** DiVoid retrieves by semantic similarity, and a stated intent is
  usually a *better* query than the raw input: more specific, on-topic, and free of deixis. Note the
  side effect — when the user says only "why?", the model's *previous* intent is still a high-quality
  query. Intent partly solves §5.1 from the other end.
- **It is the human analogy, correctly.** Nobody recalls by address. You form an intention to
  remember and the recall happens beneath you, unencumbered by the how.

**Three flavours of intent, which want different handling** (conflating them is how search queries end
up written into the graph as if they were knowledge):

| Flavour | Example | Treatment |
|---|---|---|
| **Prospective** | "I should look into Y next" | one-shot query for the next step; short half-life |
| **Sustained** | "keep the auth refactor constraints in mind throughout" | a *standing* query, re-run each step until released or the workflow step changes |
| **Retrospective** | "X turned out to be the cause" | not a query at all — an observation for the write path (§5.3) |

The sustained flavour is how this reproduces the useful half of pinning without pins: **a standing
intent is a derived pin.** Which closes the section — the working set is **derived, not stored**: a
pure function of (workflow step, task node, standing intents, recent-usage decay), with hysteresis so
it does not thrash. Nobody mutates it, so nobody can corrupt it.

**Two risks this introduces, both with structural answers.**

1. **Narrowing spiral.** If step *n*'s context is driven mainly by step *n*'s intent, and the intent is
   formed from the context just received, the loop reinforces its own hypothesis and rabbit-holes. So
   intent is **one query among several with a bounded share of the budget** — the structural queries
   and a query derived from the *original task statement* always run too, and the latter does not
   drift. The anchor stays attached.
2. **Steering away from correction.** A model steering its own recall can steer clean past the thing
   that would have corrected it. Answer: **intent-driven recall always includes the demoted-region hop**
   from §5.4. Aiming at Y is exactly the moment you most need to know Y was rejected in March.

**And it sharpens the eval rather than softening it.** Because intent is text, every step logs
(intent → nodes it retrieved → whether they were used), and the channel can be muted on a subset of
runs for a clean A/B. That is a better experiment than measuring pin usage, because it is one lever
with a measurable effect. As a freebie, the intent field is also the best available raw material for
the narrative log in §5.7 — *"asked to do X; thought Y might matter; looked at Z; concluded W"* is the
story tier writing itself.

## 6. What the graph has to provide

DiVoid is already most of the way there. What Processor leans on today:

- **Semantic search with scores** over typed nodes — exists.
- **Typed nodes and directed, labelled edges** (`supersedes`, `depends-on`, `implements`, …) — exists.
- **Neighbourhood walks** (`linkedto`) — exists.
- **Content blobs of arbitrary type** — exists.
- **Agent-to-agent messaging** — exists.

What it likely needs to grow:

- **A composite context endpoint** — one call doing vector + n-hop expansion + type/status filters +
  supersession suppression, returning a scored candidate set. Retrieval belongs *in the graph* where
  the embeddings live; the harness should not reimplement it over REST.
- **Lexical / trigram search** alongside vector, for identifiers and error strings.
- **Validity and supersession semantics** as query-level concepts, not conventions expressed in prose.
- **Cheap bulk reads** — assembly fetches many small nodes per step; per-node round-trips will not hold.

**The architectural split:** the *retrieval primitive* belongs to DiVoid; the *assembly policy*
(budgets, ordering, layering, dedup, tier merging) belongs to Processor. Policy is where the experiment
lives, and it must stay cheap to change.

## 7. Stack

**Core loop and API: Go.** A long-lived stateful server doing heavy concurrent I/O fan-out (model
streams, graph calls, tool subprocesses), shipped as one static binary with no runtime. Good fit, and
an unglamorous one — the right kind of choice for the part that must not be exciting. The thinner LLM
SDK ecosystem barely matters: nearly all of it is HTTP + SSE + JSON, and we want control of that layer
anyway.

Considered and set aside: **TypeScript** (best ecosystem, worst story for a long-lived disciplined
process), **Rust** (the safety we need is process-level, not memory-level; iteration cost too high for
an experiment), **.NET** (genuinely strong, and home turf — the reason to skip it is that the core is a
thin I/O loop, which is where C#'s strengths buy the least).

**Skills / tools layer: Python**, behind a *process boundary* — JSON over stdio or a local HTTP
endpoint, never FFI. Skills are untrusted, generated and replaceable; they must be able to crash
without touching the core.

**Interface: API-first, thin web UI.** The UI's distinguishing job is not chat — it is **showing the
assembled context and the graph around a run.** If the UI is just another chat window, we have missed
the point of our own architecture.

**DiVoid stays as it is.** Processor is a client. No fork, no embedded copy.

## 8. What we are not claiming

- **Not that history is useless.** It is demoted to a bounded buffer with a narrow job.
- **Not that this is RAG.** RAG retrieves documents to answer a question. This retrieves *the agent's
  own operating state* — briefings, workflow position, prior decisions, obligations — per step of a
  loop, not per question.
- **Not that it is cheaper on day one.** It should be smaller and more controllable, which usually
  becomes cheaper; caching muddies the arithmetic and we should say so out loud.
- **Not that determinism replaces intelligence.** It constrains where intelligence gets spent.

## 9. Milestones

1. **Skeleton loop.** Input → mechanical assembly → a single *judgement step* → write-back. One agent,
   one tool, no workflows. Prove the assembled context is *legible* and the loop closes.

   *Corrected 2026-09-01:* this read "one model call", which is inconsistent with "one tool" — if the
   tool is ever used there are two calls. The unit is one **judgement step**, with the tool cycle
   inside it. The tool is supplementary recall, and it exists because §5.2's worst failure mode
   (a confident agent with a clean context missing the one node that mattered) is otherwise invisible:
   a milestone that closes the loop with no way to see whether the mechanical floor was too low has
   built the demo and skipped the experiment.
2. **Retrieval eval harness.** The (input → required nodes) corpus and a recall@k score. Nothing
   downstream is tunable without it, so it comes second, not last.
3. **Three-tier assembly.** Working set + episodic buffer + hybrid recall, with supersession
   suppression and a token budget. This is the core experiment.
4. **Workflows as graph.** Obligations and affordances; the first mechanical gate — a test run the
   model cannot talk its way past.
5. **Consolidation pass.** Offline grouping, re-homing, supersession marking. The thing that keeps the
   memory alive past month one.
6. **Run explorer UI.** Assembled context, candidate scores, cut list, graph neighbourhood per step.
7. **Multi-agent.** Roles as graph nodes, hand-off as edges, messaging as it already works.

## 10. Prior art we own

The Gangolf memory loop (DiVoid task #905, design #906, closure #930, structure #935, consolidation
findings #1317) is a year-long, production-observed run of the small version of this idea: a bounded
action loop over a DiVoid memory with `remember` / `apropos` / `memorize` / `respond`. Everything in
§5.3 is paid-for knowledge rather than speculation. Processor generalises it from "a bot with memory"
to "a harness whose substrate is memory."

---

*Contradictions between this document and reality are to be resolved in favour of reality, and
recorded as supersession.*
