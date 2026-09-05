# Processor

Processor is a harness whose substrate is memory, not history (see `VISION.md`). Two binaries: one
serves the turn over HTTP, the other sweeps a retrieval corpus and scores what retrieval delivered.

## Status

<!-- Maintenance: this section is anchored to VISION.md's milestone list, not to the tree. A merge
     changes nothing here; a finished milestone moves one item from "what is coming" up into "what you
     get today". Keep counts, package names, file lists and graph node ids out of it — they belong to
     the sections below that own them, where they are already checked against a real run. -->

**What you get by cloning this today.** Build it, point it at a DiVoid graph and at any
OpenAI-compatible model endpoint — a local runtime works with no key and no per-token spend — and one
`POST /runs` gives you a full turn: it pulls context out of the graph mechanically, asks the model once
(letting the model request one supplementary lookup of its own), writes the result back to the graph as
a node, and hands you the whole record of what it retrieved, what it kept, what it cut and why. That
path has been run end to end against a real model, on both the plain-answer route and the tool-using
one. There is also a second binary that scores how good the retrieval was — it builds and runs, but the
hand-labelled answer key it scores against does not exist yet, **so today it can tell you nothing
meaningful**. Writing that answer key is the work currently in front of the project.

**What to expect if you come back later.**

- **Better context assembly**, which is the next real capability and the one the project is actually
  about. What is here now is deliberately the simplest thing that is honestly mechanical: a fan-out of
  whole-graph recalls fused by reciprocal rank, three of the twenty candidate slots reserved for the
  subject's own two-hop neighbourhood, a byte budget, and no memory of previous turns.
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
  OS signals, constructs the graph adapter, the model adapter and the loop, owns the exit code.
- `cmd/eval` — the retrieval sweep: loads a corpus file (`-corpus`) and, optionally, a sidecar of
  pinned per-row queries (`-derivations`), and per row does exactly what a real run's first six steps
  do — fetch the anchor, call the loop's own `Retrieve`, `Assemble` — then stops before the model, so a
  full sweep costs no model call at all, on either arm. Without the sidecar every row is swept on its
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
  It costs **two model calls at minimum and six at most** — one per turn, and up to the loop's own cap
  of three per turn when the model asks for supplementary recall — writes two run records and names
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
  clock, no randomness) and its sequencing (`Turn.Run`): fetch, assemble, judge — dispatching the one
  supplementary-recall tool as the model asks for it, up to a call cap — then write the record back. See
  `docs/architecture/m1-skeleton-loop.md` §9.
- `internal/divoid` — the graph adapter: reads the subject node, the semantic recall query and the
  edges incident to a node, and writes the run record back as one node linked to its subject.
- `internal/openaicompat` — the model adapter: one OpenAI-compatible chat-completions client. Named for
  the protocol, not a vendor — it will mostly point at things that are not OpenAI. Provider-agnostic by
  ruling (design **#10521**): a local runtime works with no credential and no per-token spend.
- `internal/eval` — the corpus and the scorer: the row type with its validating loader, the pure scoring
  of a row's required nodes against what assembly did with them, and the reporter. **Two** rates, not
  one — *retrieved* (did retrieval surface the node at all) and *admitted* (did it survive the
  byte budget) — because they fail differently and imply opposite fixes
  (`docs/architecture/m2-retrieval-eval.md`).
- `GET /health` — returns `200` with `Content-Type: application/json` and body `{"status":"ok"}`.
- `POST /runs` — assembles context for one input against one subject node, judges it against the
  configured model (dispatching the recall tool as needed), writes the run record to the graph, and
  returns the record (see "`POST /runs`" below).

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
| Drain grace | **11 minutes** | How long `Shutdown` is willing to wait for connections to go idle: the run bound, plus **45 s** for a write-back of at most three graph calls at 15 s each, plus **15 s** of stated margin so the grace does not sit exactly on its own bound. A test asserts the literal against both parts separately — the 45 s is derived, the 15 s is not |

An **idle** shutdown still returns immediately — the grace is a ceiling, not a wait — so the longer number
costs nothing when there is no run to protect.

**The drain guarantee is conditional on the supervisor.** `docker stop` sends `SIGTERM` and then `SIGKILL`
after **10 seconds** by default, which is long before the grace. A container expected to finish its run
needs the kill timeout raised to at least the grace:

```sh
docker stop -t 660 <container>
```

Without that, a run in flight at shutdown is still killed — the process offers the ceiling, the supervisor
decides how much of it is honoured.

### Configuration

Six environment variables, read once — still the module's one environment read site
(`internal/boot/config.go`). `cmd/processor` calls all three loaders, in the order the table lists, so
the first offending variable is the one named:

| Variable | Default | Behaviour |
|---|---|---|
| `PROCESSOR_HTTP_ADDR` | `127.0.0.1:8080` | Listen address, used verbatim when set and non-empty. Set but empty is a startup error (an explicitly-emptied variable is treated as an operator mistake, not a request for the default). |
| `PROCESSOR_DIVOID_URL` | *(none — required)* | The DiVoid API **origin only** — `internal/boot` appends `/api/nodes` itself. Absent or present-but-empty is a startup error: there is no defensible default for a base URL. A value whose path already ends in `/api` (or already contains `/api/nodes`) is also a startup error naming the supplied value and suggesting the corrected origin — that shape is what `~/.claude/secrets/.divoid-online`'s `Url=` line holds for direct REST calls, and pasting it here would otherwise double the path and fail silently at request time instead of at boot. |
| `PROCESSOR_DIVOID_KEY` | *(none — required)* | The DiVoid API bearer key, used verbatim. Absent or present-but-empty is a startup error. Never logged, never echoed in an error, never written to the graph. |
| `PROCESSOR_MODEL_URL` | *(none — required)* | The model endpoint's base URL (an OpenAI-compatible chat-completions server), used verbatim. Absent or present-but-empty is a startup error: a local runtime and a hosted gateway serve different addresses, so there is no defensible default. |
| `PROCESSOR_MODEL_ID` | *(none — required)* | The model id sent with every request, and the value recorded in the run record's `model` field. Absent or present-but-empty is a startup error. |
| `PROCESSOR_MODEL_KEY` | *(none — optional)* | The model endpoint's bearer key. **Absent means no `Authorization` header is sent at all** — the point of the ruling, not an edge of it: a local runtime commonly needs none. **Present-but-empty is still a startup error**, exactly like every required member — an empty value is a mistake, never a way to spell "no auth", and treating it as absent would be a silent auth downgrade. Never logged, never echoed in an error, never written to the graph. |

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

`input` must be non-empty; `subject` is the id of the node the run is about. The queries sent to the
graph are `input`, verbatim — no rewriting, no expansion, no model. A turn issues it twice: once ranked
against the whole graph, and once ranked inside the subject's own two-hop neighbourhood, which costs one
extra read of the subject's edges. The whole-graph lists are fused by reciprocal rank; the last three of
the twenty candidate slots are held for the neighbourhood list, so a node the whole graph ranks past the
cap can still arrive while the nodes already at the top keep the ranks they had.

The turn: fetch the subject and recall candidates, assemble a byte-budgeted context block (anchor first,
then admitted candidates sorted by node id ascending, never by score), judge it against the configured
model, dispatch the one supplementary-recall tool as the model asks for it (up to a call cap of 3 model
calls, so at most 2 tool dispatches per run — the capping call's recall is counted but never dispatched),
then write the record back to the graph as one `session-log` node linked to the subject.

Response (`200`) is the run record: the input, the query, the anchor summary, **every** candidate
retrieval returned — not only the ones kept — each with its rank, similarity, size, content hash, and
whether it was included or cut and why, the assembled block itself, the model's answer, the model id, every
supplementary-recall round (query, and for every row the round returned — not only the admitted ones — the
same rank/id/type/name/similarity/size/content-hash/included/cut-reason columns the candidates carry, or an
error if the round was malformed or failed), how many model calls were made and whether the per-run call cap
was reached (`capReached`), token usage as one entry per model call, in call order, named for the direction
of travel (`inTokens`/`outTokens` — a `null` entry means that call's endpoint reported no usage object,
absent, never zero-filled), the stop reason (both the loop's own neutral value and the endpoint's raw
string), and the five constants that governed the run (`limits`: candidate limit, assembly byte budget,
supplementary byte budget, max model calls, max output tokens).

The response carries **one key more than the record**: `written`, the write receipt, which says where the
record was filed. It is not a member of the record and never reaches the stored copy — a stored record is
at the node it would be naming. **The stored node's body is the response body minus that one key, and
nothing else differs** (`docs/architecture/run-record-fate.md` §8.1).

| `written.state` | `written.nodeId` | Meaning | What the caller does |
|---|---|---|---|
| `stored` | present | The record is at that node, bodied and linked to the subject | nothing — the ordinary outcome |
| `unlinked` | present | The record is at that node and complete; the edge to the subject is missing | nothing; the record is safe, the edge is repairable, and the operator's log names the node |
| `notStored` | absent | No node holds this record — **the response is the only copy** | keep the body if it matters |

A write-back failure does not fail the request: the record already carries everything of value, and the
receipt names what happened. The receipt carries no reason string — every `notStored` cause produces the
same caller decision, and the diagnosis goes to stderr.

Errors use a small closed envelope, `{"error":{"code":"...","message":"..."}}`:

| Code | Status | Meaning |
|---|---|---|
| `invalid_request` | 400 | Body unparseable, or `input`/`subject` missing or empty |
| `subject_not_found` | 404 | The subject id resolves to nothing |
| `graph_unavailable` | 502 | The graph could not be read |
| `model_unavailable` | 502 | The model call did not complete (transport failure, non-2xx status, or an undecodable response) |
| `run_deadline_exceeded` | 504 | The run did not produce an answer within the service's own ceiling. Retrying unchanged hits the same ceiling — something must change (a faster endpoint, a smaller subject, a different input) |

```sh
curl -s -X POST http://127.0.0.1:8080/runs \
  -H 'Content-Type: application/json' \
  -d '{"input":"what changed in the assembler","subject":10521}' | jq .
```

## Test

```sh
go test -count=1 -v ./...
```

`-count=1` disables Go's test cache; without it a re-run can print `(cached)` and execute nothing.
A passing run prints one `ok` line per package — **eight**: `cmd/eval`, `cmd/processor`, `internal/boot`,
`internal/divoid`, `internal/eval`, `internal/loop`, `internal/openaicompat`, `internal/server` — and
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
  seconds long, and the 11-minute literal is pinned by the arithmetic assertion in
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
  already returned in rank order (**#10883**).
- **The judgement step and write-back (`POST /runs`, unit B):** `internal/openaicompat` is pinned at the
  wire level against a local test server — the exact request shape (model, messages, `max_tokens`, the one
  tool declaration), the `Authorization` header sent only when a key is configured, decoding of a
  text-only answer, a tool call, a truncated response, a refused response, and an unrecognised finish
  reason with the raw value preserved, and that a missing usage object decodes as absent rather than
  zero. `internal/loop`'s tool cycle, call cap, and every design §6.5 failure-path row (including that a
  recall failure produces no model call at all, and that a write-back failure does not fail the request)
  are pinned at the port level against canned graph and model doubles. `internal/divoid`'s write side (the
  three-POST sequence, the content-type header on the body POST, the bare-id body on the link POST, and
  that the adapter alone supplies the written node's type, name and edge) is pinned at the wire level,
  including every partial-failure branch: a failed create stores nothing and discards nothing, a failed
  body write discards the bodyless shell it just created and no other node, a failed link keeps the
  complete record and issues no `DELETE`, and a discard that itself fails still reports `notStored` while
  naming the surviving node on stderr. The two-artifact relationship (`cmd/processor/artifacts_test.go`)
  is asserted end-to-end: one turn through the real handler and the real graph adapter, both byte
  sequences taken, and the stored body compared key-for-key against the response minus `written`.
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
