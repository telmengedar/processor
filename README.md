# Processor

Processor is a harness whose substrate is memory, not history (see `VISION.md`). Three binaries: one
serves the turn over HTTP, one sweeps a retrieval corpus and scores what retrieval delivered, and one
derives substance for a named, bounded set of graph nodes — offline, and never inside a turn.

## Status

<!-- Maintenance: this section is anchored to VISION.md's milestone list, not to the tree. A merge
     changes nothing here; a finished milestone moves one item from "what is coming" up into "what you
     get today". Keep counts, package names, file lists and graph node ids out of it — they belong to
     the sections below that own them, where they are already checked against a real run.
     What this section is NOT immune to, and what has now falsified it twice: the SHAPE of a turn. A step
     added, an order changed, or a behaviour that becomes conditional falsifies these sentences while
     every count in them stays right, so the anchoring above does not catch it. On a merge that changes
     what a turn does, re-read this against the turn rather than against the numbers. -->

**What you get by cloning this today.** Build it, point it at a DiVoid graph and at any
OpenAI-compatible model endpoint — a local runtime works with no key and no per-token spend — and one
`POST /runs` gives you a full turn: it asks the model what to search for, pulls context out of the graph
mechanically, asks the model again to answer — letting it request supplementary lookups of its own —
writes the result back to the graph as a node, and hands you the whole record of what it retrieved, what it kept, what it cut and why. That
path has been run end to end against a real model, on both the plain-answer route and the tool-using
one. There is also a second binary that scores how good the retrieval was — it builds and runs, but the
hand-labelled answer key it scores against does not exist yet, **so today it can tell you nothing
meaningful**. Writing that answer key is the work currently in front of the project.

**What to expect if you come back later.**

- **Better context assembly**, which is the next real capability and the one the project is actually
  about. What is here now is deliberately the simplest thing that is honestly mechanical: a fan-out of
  recalls fused by reciprocal rank — across the whole graph, or across a span of time when the request
  asked about one — three of the twenty candidate slots reserved for the subject's own two-hop
  neighbourhood, a byte budget, and no memory of previous turns.
- **Gates a model cannot talk its way past** — work described in the graph as obligations to meet, and
  checked mechanically rather than asserted in prose.
- **A background pass that keeps the memory from silting up** — grouping, re-homing and marking what has
  been superseded, so recall still works past the first month.
- Further out: a way to inspect a run visually, and more than one agent sharing the same memory.

**The boundary, plainly.** This is an experiment about whether retrieved memory can stand in for a
conversation transcript. It is not a product, and its central question — whether the assembled context
is any good — is exactly the one not answered yet. The list above is `VISION.md`'s roadmap in plain
words; that document carries the argument.

## What's here

- `cmd/processor` — the process entry point: loads the boot configuration, binds the listener, wires
  OS signals, constructs the graph adapter, the model adapter the configured protocol selects, and
  the loop, owns the exit code.
- `cmd/eval` — the retrieval sweep: loads a corpus file (`-corpus`) and, optionally, a sidecar of
  pinned per-row queries (`-derivations`), and per row does what a real run does from the anchor
  onwards — fetch the anchor, merge the row's input with its pinned queries through the same
  `loop.MergeQueries` the turn uses, call the loop's own `Retrieve`, `Assemble` — then stops before the
  model. It reads its query set from the sidecar rather than deriving one, so a full sweep costs no model
  call at all, on either arm. Without the sidecar every row is swept on its
  own input alone; the result names the arm it ran and the sha256 of the sidecar it read, so two
  readings taken minutes apart can be told apart. Writes the measurement as JSON to stdout and the
  human summary to stderr, and exits non-zero when retrieval could not be verified against the control
  stratum. A control node the byte budget merely cut exits zero under a named budget alarm — that is the
  assembler being measured, not the instrument failing. Needs the graph half of the boot configuration
  only.
- `scripts/smoke.py` — the live smoke run, and the opposite instrument to the sweep: `python
  scripts/smoke.py` builds `cmd/processor`, starts it on a free port and posts the **same input twice**,
  printing per turn how many candidates were admitted out of how many, the block that was actually sent
  to the model verbatim, the model's answer verbatim, and whether the supplementary lookup was used —
  then the diff between the two turns on admitted count, rank-1 candidate and block size. **Two turns
  and not one**, because a turn writes its record into the same graph the next turn recalls from: the
  first turn is clean by construction, so only the second can show what the first left behind. It takes
  the graph credential from `PROCESSOR_DIVOID_URL`/`PROCESSOR_DIVOID_KEY` when those are set and
  otherwise from the ambient `DIVOID_URL`/`DIVOID_RAZIEL_KEY`, naming both pairs when neither is there.
  It costs **four model calls at minimum and fourteen at most** — two per turn, the query derivation and
  one judgement, and up to the loop's own judgement cap of six per turn when the model asks for
  supplementary recall — writes two run records and names
  them on exit; it deletes nothing. Its default input and subject match no corpus row on purpose: a run
  writes a record that outranks every real candidate for its own input, so a default matching a row
  would poison the next sweep of that row. Exits non-zero when a turn admits zero candidates, and also
  when turn 1 never stored its record, which voids the two-turn premise. Python 3, standard library
  only.
- `internal/boot` — the boot configuration: the module's one environment read site, split into the
  listen address, a graph half and a model half, so a caller that needs only graph configuration is
  never asked for model configuration.
- `internal/server` — the HTTP route table and the serve/drain lifecycle
  (`docs/architecture/m0-service-skeleton.md`).
- `internal/loop` — the turn: mechanical context assembly (`Assemble`, a pure function — no I/O, no
  clock, no randomness) and its sequencing (`Turn.Run`): fetch the subject, derive the query set and the
  retrieval window from the input, retrieve, assemble, judge — dispatching the two tools as the model
  asks for them, up to a call cap — then write the record back. The tools are
  supplementary recall and one file write; both go through the same round, so a run record carries them
  in one `toolCalls` list, each entry naming which tool it was. See
  `docs/architecture/m1-skeleton-loop.md` §9.
- `internal/workspace` — the working directory a run writes into: one fresh directory per run
  beneath a root the operator names (`PROCESSOR_WORKSPACE_DIR`). Two layers keep a write inside it.
  First, **shape rules on the path the model asked for**, each refused with the rule that refused it:
  no empty path, no null byte, no absolute or root-anchored path, no colon, no `..` that leaves the
  directory. A path that *contains* `..` but normalises back inside is accepted — the rule is on where
  the write lands, not on how it is spelled. The normalising step is load-bearing rather than cosmetic,
  but not for safety: without it the traversal rule cannot see a `..` spelled with intermediate
  components (`site/../../../x`) or with the other platform's separator, and such a path falls through
  to the second layer — still refused, but with a generic reason in place of the one the model can act
  on. Second, **the write itself goes through `os.Root`** (`openat`-style
  confinement), so anything that resolves outside the run directory is refused by construction rather
  than by a check that has to enumerate the ways out: symbolic links, **NTFS junctions** — which Go
  reports as `ModeIrregular`, not `ModeSymlink`, and which need no administrator rights to create —
  and, on Windows, reserved device names such as `NUL`, which would otherwise swallow the bytes and
  report success. Because the resolution and the write are one confined operation, there is no
  inspect-then-write window for the filesystem to change under: the code contains no separate
  inspection step to race. A refusal is returned to the model as the tool's result so it can correct
  itself, and never fails the run; a filesystem failure carries its own cause onto that surface too,
  bounded, and is named in full in the operator's log. The two are still told apart structurally — a
  confinement refusal is not a `syscall.Errno` and every real filesystem failure is — so the split
  does not depend on matching an error message; what differs is which sentence each carries, not
  whether one of them carries a sentence at all. The guard is exercised on **both** host platforms:
  `link_windows_test.go` plants a junction, `symlink_linux_test.go` plants a symlink, and each
  asserts the escape is refused *and* that nothing appeared outside.
- `internal/divoid` — the graph adapter: reads the subject node, the semantic recall query and the
  edges incident to a node, and writes the run record back as one node linked to its subject.
- `internal/openaicompat` — a model adapter: one OpenAI-compatible chat-completions client. Named for
  the protocol, not a vendor — it will mostly point at things that are not OpenAI. Provider-agnostic by
  ruling (design **#10521**): a local runtime works with no credential and no per-token spend.
- `internal/ollama` — a model adapter: one client for ollama's own `/api/chat`, selected by
  `PROCESSOR_MODEL_PROTOCOL=ollama`. The native surface differs from the OpenAI-compatible one in three
  ways that are the whole of the adapter's job: the response carries a single `message` rather than a
  `choices` array; a tool call's arguments arrive as a **JSON object**, not as a string holding encoded
  JSON; and sampling lives under `options`, where the output cap is spelled `num_predict`. Which adapter
  and which endpoint served a run is recorded on the run record, so a trace can name its own transport.
- `internal/eval` — the corpus and the scorer: the row type with its validating loader, the pure scoring
  of a row's required nodes against what assembly did with them, and the reporter. **Two** rates, not
  one — *retrieved* (did retrieval surface the node at all) and *admitted* (did it survive the
  byte budget) — because they fail differently and imply opposite fixes
  (`docs/architecture/m2-retrieval-eval.md`).
- `GET /health` — returns `200` with `Content-Type: application/json` and body `{"status":"ok"}`.
- `POST /runs` — assembles context for one input against one subject node, judges it against the
  configured model (dispatching the recall and file-write tools as needed), writes the run record to the
  graph, and returns the record (see "`POST /runs`" below).
- `Dockerfile` — builds `cmd/processor` into a `gcr.io/distroless/static-debian12:nonroot` image; no
  secret is ever baked in (see "Container" below).

No third-party dependencies. Standard library only.

## Prerequisites

Go 1.27 or later, on `PATH`.

## Build

```sh
go build ./...
```

A clean build prints nothing and exits `0`.

## Run

```sh
go build -o processor.exe ./cmd/processor
./processor.exe
```

This logs a line like `level=INFO msg=listening addr=127.0.0.1:8080` to stderr. Open
`http://127.0.0.1:8080/health` in a browser, or:

```sh
curl -i http://127.0.0.1:8080/health
```

which returns:

```
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok"}
```

**Shutdown on `Ctrl+C`:** in an interactive terminal, pressing `Ctrl+C` sends `os.Interrupt`, the same
signal exercised by the automated test described under "What is and isn't verified here" below, which
measured the process logging `shutdown started` then `shutdown complete` and exiting `0`.

**A shutdown drains an in-flight run rather than cancelling it.** The two numbers behind that are
constants, not configuration (`docs/architecture/run-record-fate.md` §8.4):

| Constant | Value | What it bounds |
|---|---|---|
| Run bound | **10 minutes** | Everything from the handler's entry up to and including the answer. Exceeding it is `504 run_deadline_exceeded` |
| Drain grace | **11 min 15 s** | How long `Shutdown` is willing to wait for connections to go idle: the run bound, plus **60 s** for a write-back of at most four graph calls at 15 s each, plus **15 s** of stated margin — one whole per-call graph timeout, so a fifth graph call added to the write-back without re-deriving this number **exhausts the margin rather than overrunning the grace**: the worst case becomes 11 min 15 s against a grace of 11 min 15 s, which is zero headroom and the state the margin exists to prevent. A test asserts the literal against both parts separately, and it **measures** the four by running the graph adapter's write-back against a counting transport rather than restating the count, so a fifth call turns it red |

An **idle** shutdown still returns immediately — the grace is a ceiling, not a wait — so the longer number
costs nothing when there is no run to protect.

**The drain guarantee is conditional on the supervisor.** `docker stop` sends `SIGTERM` and then `SIGKILL`
after **10 seconds** by default, which is long before the grace. A container expected to finish its run
needs the kill timeout raised to at least the grace:

```sh
docker stop -t 675 <container>
```

Without that, a run in flight at shutdown is still killed — the process offers the ceiling, the supervisor
decides how much of it is honoured.

### Configuration

Twelve environment variables, read once — still the module's one environment read site
(`internal/boot/config.go`). `cmd/processor` calls every loader at startup and stops at the first one that
refuses, so a startup error names the variable that caused it:

| Variable | Default | Behaviour |
|---|---|---|
| `PROCESSOR_HTTP_ADDR` | `127.0.0.1:8080` | Listen address, used verbatim when set and non-empty. Set but empty is a startup error (an explicitly-emptied variable is treated as an operator mistake, not a request for the default). |
| `PROCESSOR_DIVOID_URL` | *(none — required)* | The DiVoid API **origin only** — the graph client (`internal/divoid`) appends `/api/nodes` itself. Absent or present-but-empty is a startup error: there is no defensible default for a base URL. A value whose path already ends in `/api` (or already contains `/api/nodes`) is also a startup error naming the supplied value and suggesting the corrected origin — that shape is what `~/.claude/secrets/.divoid-online`'s `Url=` line holds for direct REST calls, and pasting it here would double the path to `.../api/api/nodes`. Caught at boot because the value is wrong regardless of what the graph does with the doubled path, not because that downstream behaviour has been measured. This variable is not validated to exclude userinfo — use `PROCESSOR_DIVOID_KEY` for the credential, never a `user:pass@` prefix here. If one is present anyway, it is redacted before reaching a returned error; it is never rejected at boot. |
| `PROCESSOR_DIVOID_KEY` | *(none — required)* | The DiVoid API bearer key, used verbatim. Absent or present-but-empty is a startup error. Never logged, never echoed in an error, never written to the graph. |
| `PROCESSOR_MODEL_PROTOCOL` | `openai-compat` | Which model adapter to build. `openai-compat` is the OpenAI-compatible chat-completions client; `ollama` is the client for ollama's own `/api/chat`. **Absent means `openai-compat`** — the adapter the service had before there was a second one, so an existing deployment is unaffected by the variable existing. Present-but-empty is a startup error, and so is any other value: the error quotes what was supplied and names both accepted values, because a protocol this binary has no adapter for has no safe default to fall back to. |
| `PROCESSOR_MODEL_URL` | *(none — required)* | The model endpoint's **base URL**, used verbatim; the adapter appends its own route (`/chat/completions` under `openai-compat`, `/api/chat` under `ollama`), so set the origin the route hangs off, not the route. Absent or present-but-empty is a startup error: a local runtime and a hosted gateway serve different addresses, so there is no defensible default. This variable is not validated to exclude userinfo — use `PROCESSOR_MODEL_KEY` for the credential, never a `user:pass@` prefix here. If one is present anyway, it is redacted out of the run record, its substance, and the log; it is never rejected at boot. |
| `PROCESSOR_MODEL_ID` | *(none — required)* | The model id sent with every request, and the value recorded in the run record's `model` field. Absent or present-but-empty is a startup error. |
| `PROCESSOR_MODEL_KEY` | *(none — optional)* | The model endpoint's bearer key, sent the same way under both protocols. **Absent means no `Authorization` header is sent at all** — the point of the ruling, not an edge of it: a local runtime commonly needs none. **Present-but-empty is still a startup error**, exactly like every required member — an empty value is a mistake, never a way to spell "no auth", and treating it as absent would be a silent auth downgrade. Never logged, never echoed in an error, never written to the graph. |
| `PROCESSOR_MODEL_TEMPERATURE` | `0.2` | The sampling temperature sent with **every** model call. Absent means `0.2` — a little sampling range, not greedy decoding — so the sampler is a small, deliberate source of run-to-run variation. **Present-but-empty is a startup error**, and so is a non-numeric value; both name the variable. **The default is a trade, not a free win, and it costs exact run-to-run reproducibility**: the same request can now return a different completion on a retry, where at `temperature: 0` it would return the same one (approximately — GPU batching and reduction order admit some variance even at `0`; DiVoid #14154). What the trade buys is structural rather than measured: at `0`, a completion that comes back malformed for a given prompt comes back malformed *every* time that prompt is asked, because the request is otherwise a pure function of its inputs — a human re-asking gets the identical failure, and the retry fallback the service leans on elsewhere cannot cover it. At `0.2` a bad draw is no longer permanent for that prompt. **Nothing measured separates `0` from `0.2`: the bands actually sampled were `1.0`, `0.6` and `0.0`, and `0.0` and `0.6` both came back clean — `0.2` sits between them but was never itself a sampled point, so this is a decision about the *shape* a failure takes, not a demonstrated drop in how often one happens; it remains possible that `0.2` produces the same errors and they simply have not been seen yet (DiVoid #14154).** Set the variable to opt into more of the endpoint's own sampling, or back down to `0` for exact-as-possible reproducibility. |
| `PROCESSOR_WORKSPACE_DIR` | *(none — optional, every write is refused)* | The directory beneath which each run's own working directory is created. The service creates the root if it does not exist, and one fresh directory per run, lazily — a run that never asks to write a file leaves nothing behind. **Absent means the file tool is still offered and every call to it is refused**, with `no working directory is configured` recorded on the round and shown to the model: the tool list does not change shape with the environment, so a trace reads the same either way and names the operator condition rather than hiding it. Present-but-empty is a startup error naming the variable. Nothing constrains what a run writes inside its own directory, so point this at a directory whose contents are disposable. |
| `PROCESSOR_MODEL_FLOOR_TOKENS_PER_SECOND` | `10` | **What this deployment asserts its model endpoint generates at, in output tokens per second.** It is a *declaration*, not a measurement: nothing probes the endpoint, and boot says so in the line that reports it. Every model call site declares its own output budget, and boot checks each one against the bound that will actually cut that call — `budget ÷ this floor + prompt ÷ the prompt floor < bound × 0.8`. A site the declared floors cannot afford is reported as a warning naming **the rate this host would have to deliver**, which is the number the service could never state before. Boot does not refuse on it: a pessimistic declaration is a wrong guess, not a reason to take the service down, and the fill site's own budget is sized per node rather than from any bound (that is a separate open defect, not decided here). Absent means `10` — a round number below every generation rate this product has been observed at, chosen so a host that fails it is genuinely pathological rather than merely slow. Present-but-empty is a startup error, so is a non-numeric value, and so is zero or less: a rate of zero asserts an endpoint that never finishes. |
| `PROCESSOR_MODEL_FLOOR_PROMPT_BYTES_PER_SECOND` | `3000` | **What this deployment asserts its model endpoint reads a prompt at, in bytes per second**, on the same terms as the generation floor above: a declaration, reported at boot, never probed. Each call site's prompt allowance is its own declared prompt ceiling divided by this rate — the judgement site is priced at the assembly byte budget plus one supplementary round, which is the largest prompt one call can carry. Absent means `3000`. Present-but-empty, non-numeric, zero or negative are all startup errors. |
| `PROCESSOR_MODEL_TOP_P` | *(none — optional, nothing is sent)* | The nucleus-sampling mass sent with every model call. **Absent means the parameter is omitted from the request entirely**, not sent as `0`: `temperature: 0` is the conventional spelling of greedy decoding, but `top_p: 0` is *endpoint-dependent* — the OpenAI-compatible protocol does not specify what a runtime must do with it, and runtimes differ — so there is no value that spells "unset" and none is invented. Asserting `1.0` instead would claim knowledge of an endpoint this service deliberately does not have. Present-but-empty is a startup error, and so is a non-numeric value; both name the variable. |

```sh
PROCESSOR_HTTP_ADDR=:9090 \
PROCESSOR_DIVOID_URL=https://divoid.example \
PROCESSOR_DIVOID_KEY=xxxxxxxx \
PROCESSOR_MODEL_URL=http://127.0.0.1:11434/v1 \
PROCESSOR_MODEL_ID=llama-3.1-8b-instruct \
./processor.exe
```

`PROCESSOR_MODEL_KEY` is omitted above deliberately — that is the ruling's own target case: a local
runtime with no credential, no spend, and no terms-of-service exposure. Set it when the endpoint requires
one.

### `POST /runs`

Request:

```json
{"input": "free text", "subject": 12345}
```

`input` must be non-empty; `subject` is the id of the node the run is about. The turn fetches that
subject first — so an unreachable graph and a subject that resolves to nothing both fail before any model
call and cost **none**. It then asks the model once, under its own 30-second bound, for up to five
further queries derived from `input` **and for a calendar day range** — one extra output line, `DATES: YYYY-MM-DD..YYYY-MM-DD` or
`DATES: none`, resolved against the instant the prompt states — which becomes the run's retrieval window.
That call is not charged to the run's six-call judgement budget, and any outcome which is not a usable
query set leaves the run asking `input` alone and records why on the record's `derivationError`. A
`DATES` line that is absent, unrecognised or malformed yields **no window** rather than a guessed one,
and retrieval is then unbounded, exactly as it was before the line existed.
`input` is always the first query. Each query is ranked against the whole graph — or, when the run
derived a window, against the nodes whose last update falls inside it — and `input` is ranked once more
inside the subject's own two-hop neighbourhood, which costs one extra read of the subject's edges. **That
neighbourhood recall is never bounded by the window**, deliberately: it answers what is structurally
adjacent to the subject, which has no time dimension, and bounding it would silently empty the reserve
below. The ranked lists are fused by reciprocal rank; the last three of the twenty candidate slots are
held for the neighbourhood list, so a node ranked past the cap can still arrive while the nodes already
at the top keep the ranks they had.

The turn: fetch the subject, derive the query set and window described above, recall candidates against
them, assemble a byte-budgeted context block (anchor first,
then admitted candidates sorted by node id ascending, never by score), judge it against the configured
model, dispatch a tool each time the model asks for one (up to a **judgement**-call cap of 6, so at most 5
tool dispatches per run — the capping call's request is counted but never dispatched; the derivation call
above is not charged to this cap, so a turn makes up to **seven** model calls in all), then write the
record back to the graph as one `session-log` node linked to the subject. The node's content is
`text/markdown; charset=utf-8`: the run's own account first (the same prose `RenderSummary` renders, in
`internal/loop/summary.go`), then a blank line, a line reading exactly `---`, then the complete record as
one fenced ` ```json ` block. The record — described next — is recovered by taking the content's last
fenced `json` block and parsing it.

Two tools are offered on every call: `recall`, which searches the same graph, and `write_file`, which
writes one file into the run's working directory. Neither is urged — the system text names each in one
sentence and still asks for prose — so whether a run reaches for the file tool is a property of the model
and the task, not of the prompt.

Response (`200`) is the run record: the input, the instant the prompt stated (`now`, absent when it
stated none), the query and the full query set the derivation produced (`queries`, with `input` always
first) together with `derivationError` when that derivation failed, the update-time window retrieval was
held to (`window`, absent when the run expressed no time constraint), the anchor summary, **every** candidate
retrieval returned — not only the ones kept — each with its rank, similarity, size, content hash, and
whether it was included or cut and why, the assembled block itself, the model's answer, the model id, which adapter and endpoint served the
run's model calls (`provider`: `adapter` and `endpoint`, as the adapter itself reported them rather than as
the environment was set — with two protocols live a trace that cannot name its own transport cannot be
compared against another), every
tool round in call order (`tool`, naming which tool it was — `recall` or `writeFile`; `source`, naming how
the adapter obtained the call — `native` where the endpoint reported it in its own tool-call field,
`content` where the endpoint reported none and the adapter recovered the call from the response text;
absent on a round no adapter produced; for a recall round the
query and, for every row it returned — not only the admitted ones — the same
rank/id/type/name/similarity/size/content-hash/included/cut-reason columns the candidates carry; for a write
round the `path` the model asked for and the `bytes` it offered; and on either an `error` if the round was
malformed, refused, capped or failed), the run's `workspace` directory when one was opened — absent when the
run attempted no write, and never carrying what was written, which is only on disk — how many model calls were made and whether the per-run call cap
was reached (`capReached`), token usage as one entry per model call, in call order, named for the direction
of travel (`inTokens`/`outTokens` — a `null` entry means that call's endpoint reported no usage object,
absent, never zero-filled), the stop reason (both the loop's own neutral value and the endpoint's raw
string), why the turn stopped where the run's own remaining time could not afford another judgement call
(`timeShortfall`, absent on every run that ended any other way), the constants that governed the run
(`limits`: candidate limit, assembly byte budget, supplementary byte budget, max model calls, and one
output budget per model call site — `derivationBudget` and `judgementBudget`), and the sampling the run was made with
(`sampling`: `temperature` and `topP`, each key absent when nothing was sent for it).

**Read that as a description, not as the field list.** `Record` in `internal/loop/types.go` is the field
list; three merges have each added a key that did not reach this paragraph until someone
swept for it. To check it, read the struct's tags rather than trusting the sentence.

**`source` exists because a correct call is not always reported as one.** Against `qwen3-coder-fixed:30b`
on ollama 0.33.3 the endpoint sometimes returns `done_reason: stop` with no tool call and a complete,
correct `<function=NAME><parameter=KEY>value</parameter></function>` block left sitting in the response
text; the adapter reads that shape when, and only when, the tool-call field is empty. **A round the
fallback carried is not the same event as one the endpoint parsed**, so the record distinguishes them
rather than letting a degraded path look like a working one. A call-shaped block the adapter finds but
cannot complete is recorded as a failed tool round with its `error`, never answered as prose.

**`sampling` reports what was requested, as sent — not what the endpoint applied.** An endpoint that clamps
a value, or ignores it, yields a record that is true about the request and false about the generation. The
record can be honest about only the near side of the wire, and this is that side.

The response carries **one key more than the record**: `written`, the write receipt, which says where the
record was filed. It is not a member of the record and never reaches the stored copy — a stored record is
at the node it would be naming. **The record inside the stored node's last fenced `json` block is the
response body minus that one key, and nothing else differs** (`docs/architecture/run-record-fate.md`
§8.1) — the account and the fence wrapped around the record are additions to the stored body, not a
difference within the record itself.

| `written.state` | `written.nodeId` | Meaning | What the caller does |
|---|---|---|---|
| `stored` | present | The record is at that node, bodied and linked to the subject | nothing — the ordinary outcome |
| `unlinked` | present | The record is at that node and complete; the edge to the subject is missing | nothing; the record is safe, the edge is repairable, and the operator's log names the node |
| `notStored` | absent | No node holds this record — **the response is the only copy** | keep the body if it matters |

A write-back failure does not fail the request: the record already carries everything of value, and the
receipt names what happened. The receipt carries no reason string — every `notStored` cause produces the
same caller decision, and the diagnosis goes to stderr.

Errors use a small closed envelope, `{"error":{"code":"...","message":"..."}}`. `code` is the stable
machine-readable classifier and keeps its meaning below. `message` is prose: for the two 502 codes it
is the class sentence followed by the cause the failure was handed — the endpoint's own sentence, or
the operating system's — bounded to 512 runes. The operator's log carries the same cause unbounded.

| Code | Status | Meaning |
|---|---|---|
| `invalid_request` | 400 | Body unparseable, or `input`/`subject` missing or empty |
| `subject_not_found` | 404 | The subject id resolves to nothing |
| `graph_unavailable` | 502 | The graph could not be read, or a failure the service does not classify; `message` names the cause |
| `model_unavailable` | 502 | The model call did not complete (transport failure, non-2xx status, or an undecodable response). `message` names the model, the endpoint, the assembled request's size in bytes, the elapsed time against the client bound that was in force, and then the cause |
| `run_deadline_exceeded` | 504 | The run did not produce an answer within the service's own ceiling. Retrying unchanged hits the same ceiling — something must change (a faster endpoint, a smaller subject, a different input) |

```sh
curl -s -X POST http://127.0.0.1:8080/runs \
  -H 'Content-Type: application/json' \
  -d '{"input":"what changed in the assembler","subject":10521}' | jq .
```

## Container

```sh
docker build -t processor .
```

The build stage is `golang:1.27` (matching the version pinned everywhere else in this file); the
final image is `gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager, CA
certificates present for the outbound HTTPS calls to the graph and the model endpoint, running as
uid/gid 65532 rather than root. `go.mod` declares no external dependencies, so the build downloads
nothing and needs no credential.

**The image sets one default that bare-metal does not need:** `PROCESSOR_HTTP_ADDR=0.0.0.0:8080`.
The service's own default (`127.0.0.1:8080`, see the configuration table above) is correct for a
process sharing the host's network namespace and silently unreachable from outside a container — a
loopback bind never sees traffic arriving on a published port. This is not a secret and is still
overridable with `-e PROCESSOR_HTTP_ADDR=...` if the port itself needs to change.

```sh
# Linux / macOS
docker volume create processor-workspace

docker run -d --name processor \
  -p 8080:8080 \
  -e PROCESSOR_DIVOID_URL=https://divoid.example \
  -e PROCESSOR_DIVOID_KEY=xxxxxxxx \
  -e PROCESSOR_MODEL_URL=http://127.0.0.1:11434/v1 \
  -e PROCESSOR_MODEL_ID=llama-3.1-8b-instruct \
  -v processor-workspace:/data/workspace \
  -e PROCESSOR_WORKSPACE_DIR=/data/workspace \
  processor

# PowerShell
docker volume create processor-workspace

docker run -d --name processor `
  -p 8080:8080 `
  -e PROCESSOR_DIVOID_URL=https://divoid.example `
  -e PROCESSOR_DIVOID_KEY=xxxxxxxx `
  -e PROCESSOR_MODEL_URL=http://127.0.0.1:11434/v1 `
  -e PROCESSOR_MODEL_ID=llama-3.1-8b-instruct `
  -v processor-workspace:/data/workspace `
  -e PROCESSOR_WORKSPACE_DIR=/data/workspace `
  processor

# Git Bash
docker volume create processor-workspace

MSYS_NO_PATHCONV=1 docker run -d --name processor \
  -p 8080:8080 \
  -e PROCESSOR_DIVOID_URL=https://divoid.example \
  -e PROCESSOR_DIVOID_KEY=xxxxxxxx \
  -e PROCESSOR_MODEL_URL=http://127.0.0.1:11434/v1 \
  -e PROCESSOR_MODEL_ID=llama-3.1-8b-instruct \
  -v processor-workspace:/data/workspace \
  -e PROCESSOR_WORKSPACE_DIR=/data/workspace \
  processor
```

**On Windows, use the variant that matches your shell.** MSYS2's path conversion (the layer
underneath Git Bash) rewrites any argument that looks like an absolute Unix path — including the
`-e PROCESSOR_WORKSPACE_DIR=/data/workspace` value above — into a Windows path rooted at Git's own
install directory, silently: the container starts, the request still returns `200`, and the file
lands at a path like `/home/nonroot/C:/Program Files/Git/data/workspace/...` inside the container
rather than on the named volume. `MSYS_NO_PATHCONV=1` disables that rewrite for the whole command —
the same fix used for the same reason under "The Linux gate" below — or run the PowerShell variant
instead, which is not subject to MSYS path conversion at all.

**Neither secret goes in the image, in a layer, or in a default — both arrive at `docker run` time,
exactly as above, and only that way.** `PROCESSOR_MODEL_KEY` is omitted deliberately, same as in the
bare-metal example: no credential, no spend. If your `PROCESSOR_DIVOID_URL` comes from
`~/.claude/secrets/.divoid-online`'s `Url=` line, strip its trailing `/api` first — that line is
shaped for direct REST calls and the boot check above rejects it verbatim for the reason given
there.

**Set `PROCESSOR_WORKSPACE_DIR` without a matching `-v`, and the image's own `VOLUME` declaration
still gives you a volume** — an anonymous one, named by hash rather than `processor-workspace`,
which survives `docker rm` unless the container was run with `--rm -v` (or removed with
`docker rm -v`) — so an omitted `-v` is *harder* to notice and recover from than it would be without
the `VOLUME` instruction, not safer. Always pair the environment variable with an explicit `-v`
naming a volume you can find again.

**The workspace is a named volume, not a bind mount, because a bind mount's host path is resolved
by whichever machine actually runs the Docker daemon, not by the machine issuing `docker run`** —
if your `docker` CLI talks to a remote or SSH-based context (`docker context ls` shows more than
`default`, or `DOCKER_HOST` is set), `-v "$(pwd)/workspace:/data/workspace"` names a path on *your*
filesystem and Docker looks for it on the *daemon's* — measured on this project's own Docker
context: it does not merely get the ownership wrong, the path does not exist at all. A named volume
has no such ambiguity: it lives entirely inside Docker's own storage on whichever host the daemon
runs, and `docker cp` (or a throwaway container) is how you look inside it regardless of where your
shell is. **If your daemon and your shell are the same machine**, a bind mount to a directory you
own is also fine and slightly more convenient to browse — just chown it to `65532:65532` first.

The same client/daemon split applies to `-p 8080:8080` above: it publishes the port on the
**daemon's** network interfaces, not the client's, so `curl http://127.0.0.1:8080/health` (the
bare-metal example earlier in this file) only reaches the containerized service when your shell and
the daemon are the same machine — against a remote or SSH `docker context`, curl the daemon host's
own address instead, on the port you gave `-p`.

The container still runs as uid/gid 65532, and a named volume Docker creates fresh (nothing by that
name existed before) is seeded from the image's `/data/workspace` directory, ownership included —
so a first-ever `docker volume create processor-workspace` above followed by `docker run` needs no
further step; **verified**: a brand-new volume mounted over that path came up owned by 65532 and
accepted a write from a `--user 65532:65532` container with no extra command. That seeding is
one-time, though — it does **not** reach a volume that already exists, so if you (like Toni) already
ran an earlier version of this image and have a `processor-workspace` volume that came up
root-owned, fix that one existing volume once:

```sh
docker run --rm -v processor-workspace:/data/workspace busybox \
  chown -R 65532:65532 /data/workspace
```

**Verified the other direction too**: re-mounting an already-existing, already-root-owned volume
into the corrected image left it root-owned and still denied the write — chowning the image's own
copy of the directory has no effect on a volume that already exists, only on one Docker is creating
for the first time. The one-line fix above works either way (freshly created or already broken) and
does not depend on that seeding behaviour at all, so it is the one command worth remembering; the
seeding is a bonus for a clean start, not the mechanism to rely on.

**The stop timeout must not contradict the drain grace above, and this file already restates that
number more than once with nothing checking the copies agree (tracked as DiVoid #13432 — the
Go-side mirror test that pins `shutdownGrace` covers `scripts/*.py`, not README's own prose).
Rather than add a fourth copy here, this section reuses the one already given as a runnable example
under "Run" two sections above, where the same 675-second figure appears in `docker stop -t 675
<container>` —**

```sh
docker stop -t 675 <container>
```

**substituting your container's name.** There is no Dockerfile instruction that sets a default stop
timeout — Docker has none — so every appearance of the number in this file is prose, not
configuration — tracked together under #13432 rather than deduplicated here — including no
`docker-compose.yml`: a compose file's own `stop_grace_period` would be a fifth, independently-editable
copy of the same literal in a different syntax, which is the drift #13427 already named as costly.
This repo ships no compose file for exactly that reason. A `docker stop` (or `docker compose stop`)
issued without an explicit timeout defaults to 10 seconds and kills a run mid-write-back — the drain
grace exists to prevent exactly that outcome, and a supervisor that does not honour it makes the
guarantee moot regardless of what the process offers.

## Test

```sh
go test -count=1 -v ./...
```

`-count=1` disables Go's test cache; without it a re-run can print `(cached)` and execute nothing.
A passing run prints one `ok` line per package — **thirteen at `bdd5fea`**: `cmd/condense`, `cmd/eval`,
`cmd/processor`, `internal/boot`, `internal/condense`, `internal/divoid`, `internal/eval`,
`internal/loop`, `internal/ollama`, `internal/openaicompat`, `internal/redacturl`, `internal/server`,
`internal/workspace`. **`go list ./...` owns that number, not this sentence** — run it rather than
counting the names here, which have been one short since `internal/redacturl` arrived. And
every `--- PASS:` line for each test. A `?` line is the one to watch for: it means a package shipped with
no test at all. The default suite is fully offline and hermetic: no network call, no credential, no live
graph, no live model, no spend — every
graph-facing and model-facing test runs against a local `httptest.Server`, deliberately (design §9.3): a
live model is nondeterministic, so it can never be what a per-change suite asserts against. This is also
why there is no automated model gate at any tier — see "What is and isn't verified here" below.

## Format and vet

```sh
gofmt -l .
go vet ./...
```

Both print nothing when clean. If `gofmt -l .` lists files after a fresh checkout on Windows, check
that `.gitattributes` was honoured — it pins `*.go` and `*.md` to LF line endings regardless of the
local `core.autocrlf` setting, because a CRLF Go file compiles and tests fine while `gofmt -l` still
flags it.

## What is and isn't verified here

- **Start, serve, shut down cleanly:** proven by an automated test over a real socket
  (`internal/server/server_test.go`), and confirmed manually on this machine (build, run, `curl`
  `/health`, `200` returned).
- **Signal-triggered shutdown (`Ctrl+C` / `SIGTERM`) is covered**, by an automated test, for both
  `os.Interrupt` and `SIGTERM`, which runs in a Linux container
  (`cmd/processor/process_linux_test.go`). It builds the real binary and launches two independent child
  processes in parallel, one signal per process (`os.Interrupt` and `SIGTERM` respectively), and asserts
  exit code `0` plus the ordered `listening` / `shutdown started` / `shutdown complete` records on
  stderr for each.
- **The exit codes for the configuration-error and bind-error branches are covered by the same file**:
  a set-but-empty `PROCESSOR_HTTP_ADDR` exits `1` naming the variable, and a second instance pointed at
  an already-bound address exits `1` naming that address.
- **On a non-Linux host, the default `go test ./...` does not run these** — the file is named
  `process_linux_test.go`, so Go's toolchain excludes it at the `GOOS` level, not behind a runtime skip.
  On a Linux host the default run *does* include it. Either way, run the container command below for the
  full, host-independent certification.
- **The Linux gate:**

  ```sh
  # PowerShell
  $env:DOCKER_HOST=''
  docker run --rm -v "${PWD}:/src:ro" -w /src golang:1.27 sh -c 'go vet ./... && go test -count=1 ./...'

  # Git Bash
  MSYS_NO_PATHCONV=1 DOCKER_HOST= docker run --rm -v "$PWD":/src:ro -w /src golang:1.27 \
    sh -c 'go vet ./... && go test -count=1 ./...'
  ```

  Expects the same one-`ok`-line-per-package output as the host run under "Test" above — the package set
  is identical, the container simply also runs the Linux-only file — with no `?` and exit `0`. One
  1.32 GB `golang:1.27` image pull, once.
- **The drain is covered by an automated process-level test** (`cmd/processor/process_linux_test.go`):
  the real binary is launched against local graph and model test servers, the model endpoint is made
  slow, `SIGTERM` is sent while a run is genuinely in flight (the model server signals when it has been
  reached), and the test asserts that the caller still receives the full record, that the graph still
  receives the run node, and that the process exits `0` with the ordered shutdown records. **What it does
  not establish:** that the process survives `SIGKILL` (nothing can), that a container supervisor honours
  the grace (that is a deployment flag — see "Run" above), or the grace's *value* — the run it drives is
  seconds long, and the 11 min 15 s literal is pinned by the arithmetic assertion in
  `internal/server/server_test.go` instead.
- **The run's own ceiling is covered at the handler**: a request that carries no deadline of its own
  still reaches the loop on a context bounded at ten minutes, and that ceiling stops at the answer —
  the write-back runs on a context with no deadline at all. Both are observed from a graph double
  reading `ctx.Deadline()`, so neither the bound's existence nor its value depends on waiting for it.
  Separately, the `504` path and its ordering obligation (the handler tests the run context's expiry
  *before* it classifies the error, so a deadline is never reported as an upstream `502`) are pinned
  with a caller-supplied deadline.
- **The per-run log pair is covered at both levels**: that `run started` precedes `run finished`, carries
  the input's **length** and never its text, and that `run finished` carries the receipt state and the
  wall clock. A `run started` with no matching `run finished` is the only trace a process killed mid-run
  can leave.
- **What is still not covered:** `run()`'s serve-error exit branch and a second interrupt arriving
  during the shutdown drain — both are structurally unreachable from outside the process without adding
  a slow route or a delay knob to shipping code, which is declined (design
  `docs/architecture/process-boundary-test-harness.md` §2.4) — and the remaining drain-failure axis
  (`ReadHeaderTimeout` and shutdown-error propagation at the process level), which is a different
  instrument tracked separately as DiVoid **#10489**.
- **Mechanical context assembly (`POST /runs`, unit A):** `Assemble` is byte-pinned by an offline golden
  test (fixed candidate rows in, one exact block out), and separately: admission skips rather than
  stops, so a candidate that does not fit cuts only itself; a candidate this system wrote is cut
  before the byte test and charges nothing; every candidate is hashed and sized including the cut
  ones; the render order is by node id — a score reshuffle does not move a byte; and a run that
  admits nothing from a non-empty candidate set raises a warning on the operator log. `internal/divoid`'s three read
  operations are pinned at the wire level against a local test server, including the C30 empty-result
  discrimination (a missing subject is a `200` with an empty result, never a `404`). **Since verified live** against
  `divoid.mamgo.io`: every decode assumption held, including that the `fields` projection populates
  `content` inline rather than needing a second fetch, and that `Recall` must not re-sort what the graph
  already returned in rank order (**#10883**). The retrieval window is pinned at the same wire level: a
  non-zero window sends `updatedFrom`/`updatedTo` as RFC 3339, and a **zero** window sends an encoded
  query string byte-identical to one from before the parameter existed, so an un-windowed run cannot
  silently acquire a filter. The wire spelling is established **both** by the suite and by a
  filtered-versus-unfiltered control run against `divoid.mamgo.io`, recorded in **PR #77's body**. That
  control exists because **#13721** measured the hazard it guards against: of nine parameter names probed
  on 2026-09-12, **five were silently ignored**, each returning rows and a `total` identical to the
  unfiltered control, so each looked exactly like a filter that worked. **#13721 is the source of the
  hazard, not of PR #77's table** — they are separate probes of a moving graph taken three days apart,
  and their control totals differ accordingly. **What has not been
  done** is running *this project's own client* against the live graph under that control: the probe was
  raw REST, so the suite is the only thing coupling the spelling to `internal/divoid`.
- **The judgement step and write-back (`POST /runs`, unit B):** `internal/openaicompat` is pinned at the
  wire level against a local test server — the exact request shape (model, messages, `max_tokens`, the one
  tool declaration), the `Authorization` header sent only when a key is configured, decoding of a
  text-only answer, a tool call, a truncated response, a refused response, and an unrecognised finish
  reason with the raw value preserved, and that a missing usage object decodes as absent rather than
  zero. `internal/loop`'s tool cycle, call cap, and every design §6.5 failure-path row (including that a
  recall failure produces no **judgement** call — since `3489a2d` the derivation call precedes retrieval and has
  already completed, so the property pinned is that `Judge` is never reached — and that a write-back failure
  does not fail the request)
  are pinned at the port level against canned graph and model doubles. `internal/divoid`'s write side (the
  three-POST sequence, the content-type header on the body POST, the bare-id body on the link POST, and
  that the adapter alone supplies the written node's type, name and edge) is pinned at the wire level,
  including every partial-failure branch: a failed create stores nothing and discards nothing, a failed
  body write discards the bodyless shell it just created and no other node, a failed link keeps the
  complete record and issues no `DELETE`, and a discard that itself fails still reports `notStored` while
  naming the surviving node on stderr. The two-artifact relationship (`cmd/processor/artifacts_test.go`)
  is asserted end-to-end: one turn through the real handler and the real graph adapter, both byte
  sequences taken, the record extracted from the stored body's last fenced `json` block, and that
  record compared key-for-key against the response minus `written`.
  **Since verified live, which the suite structurally could not do** — every success fixture was written
  from the same reading of the protocol that produced the decoder, so a misreading would be reproduced on
  both sides: two real turns against a real OpenAI-compatible endpoint, in a container, covering the
  answer path (**#10897**) and the tool path (**#10898**), where the model composed its own recall query,
  received the rows and answered out of them. Every decode assumption in `internal/openaicompat/wire.go`
  held, the endpoint's extra fields were ignored (the safe direction), and the write path landed all three
  POSTs with UTF-8 intact. It found one defect — the stored record's receipt was empty where the
  response's was not (**#10899**), since fixed by the record-fate design above. It stays a **manual**
  check rather than a gate (design §10.6): a live model is nondeterministic and cannot be what a
  per-change assertion runs against.
- **The native ollama adapter (`internal/ollama`):** pinned at the wire level against a local test
  server, from fixtures **captured off the live endpoint rather than written from a reading of the
  protocol** — the route, `stream: false` sent explicitly (omitted, the endpoint streams and the
  single-object decode fails), sampling under `options` with nothing at the request top level, the output
  cap spelled `num_predict`, the `Authorization` header sent only when a key is configured, a tool call
  whose arguments arrive as a **JSON object** rather than a string holding encoded JSON, a call read even
  though `done_reason` says `stop`, usage read off `prompt_eval_count`/`eval_count`, and the error body
  that is a bare string rather than an object. The prior-round replay is pinned to the assistant call plus
  a `tool_name`-tagged result, the native protocol having no call id to pair by.
- **The content fallback is verified by unit test and has not fired in a live harness run.** The failing
  shape is real and was reproduced here against `qwen3-coder-fixed:30b` — `done_reason: stop`, no tool
  call, and a complete `<function=…><parameter=…>` block left in the response text — and the fixtures are
  those exact bytes. But it reproduces at **short** prompts: a bare user message with one tool. The
  harness's own request, which carries the full system text and both tool schemas, parsed natively on
  every call of the live trace below. **So the fallback is a guard against a measured endpoint behaviour
  that the harness's own request shape did not provoke**, and the run record's `source` field is what will
  say if that changes — a round it carries reads `content`, never `native`.
- **The retrieval eval harness (`cmd/eval`, M2):** the corpus loader's validation is pinned rule by rule
  (a required set naming its own subject, one over the cap of three, an empty one, a missing reason or
  hash, a duplicate row id, a stratum outside the closed set), the scorer's four verdicts are pinned
  — including that *cut* and *not retrieved* are distinguished on otherwise identical rows, and that a
  moved content hash raises a stale flag rather than a verdict — and the reporter is pinned over
  hand-built results. The sweep is pinned at the port level against a graph double: that it recalls with
  the raw input verbatim at the loop's own candidate limit and unscoped, that it ranks that same input a
  second time inside the subject's own scope, that it issues one whole-graph recall per pinned
  derivation, that it holds exactly three of its candidate slots for the neighbourhood — the same
  assertion the turn carries, so reverting either caller to its own retrieval reddens both packages —
  that its dispositions equal the ones a real run produces for the same anchor and candidates, that it reads the graph and never writes to it, that a
  row whose required node no longer resolves leaves both denominators instead of scoring as a miss, and
  that a control node the retriever never surfaced exits non-zero while a control node the byte budget
  cut exits zero under a budget alarm — the pair over otherwise identical fixtures, so neither is
  satisfied by an implementation that reads the two verdicts alike. **Not verified here — and not
  verifiable by any test:** that the numbers mean anything. No corpus exists yet, and a green build
  proves the instrument works while proving nothing about what it measures (design **#10926** §15).
