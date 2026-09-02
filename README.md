# Processor

Processor is a harness whose substrate is memory, not history (see `VISION.md`). **M0** built a minimal
Go service skeleton that starts, serves one HTTP endpoint, and shuts down cleanly
(`docs/architecture/m0-service-skeleton.md`). **M1** closes the skeleton loop —
input → mechanical assembly → one model call → write-back — in two units
(`docs/architecture/m1-skeleton-loop.md`): **unit A** built mechanical context assembly with no model
call; **unit B** adds the judgement step and the write-back. The model call is **provider-agnostic**: one
adapter speaks the OpenAI-compatible chat-completions protocol, so a local runtime works with no
credential and no per-token spend (design #10521's ruling).

## What's here

- `cmd/processor` — the process entry point: reads the boot configuration, binds the listener, wires
  OS signals, constructs the graph adapter, the model adapter and the loop, owns the exit code.
- `internal/server` — the HTTP route table and the serve/drain lifecycle.
- `internal/loop` — the turn: mechanical context assembly (`Assemble`, a pure function — no I/O, no
  clock, no randomness) and its sequencing (`Turn.Run`): fetch, assemble, judge — dispatching the one
  supplementary-recall tool as the model asks for it, up to a call cap — then write the record back. See
  `docs/architecture/m1-skeleton-loop.md` §9.
- `internal/divoid` — the graph adapter: reads the subject node and the semantic recall query, and
  writes the run record back as one node linked to its subject.
- `internal/openaicompat` — the model adapter: one OpenAI-compatible chat-completions client. Named for
  the protocol, not a vendor — it will mostly point at things that are not OpenAI.
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

### Configuration

Six environment variables, read once, in `main` — still the module's one environment read site
(`cmd/processor/config.go`):

| Variable | Default | Behaviour |
|---|---|---|
| `PROCESSOR_HTTP_ADDR` | `127.0.0.1:8080` | Listen address, used verbatim when set and non-empty. Set but empty is a startup error (an explicitly-emptied variable is treated as an operator mistake, not a request for the default). |
| `PROCESSOR_DIVOID_URL` | *(none — required)* | The DiVoid API base URL, used verbatim. Absent or present-but-empty is a startup error: there is no defensible default for a base URL. |
| `PROCESSOR_DIVOID_KEY` | *(none — required)* | The DiVoid API bearer key, used verbatim. Absent or present-but-empty is a startup error. Never logged, never echoed in an error, never written to the graph. |
| `PROCESSOR_MODEL_URL` | *(none — required)* | The model endpoint's base URL (an OpenAI-compatible chat-completions server), used verbatim. Absent or present-but-empty is a startup error: a local runtime and a hosted gateway serve different addresses, so there is no defensible default. |
| `PROCESSOR_MODEL_ID` | *(none — required)* | The model id sent with every request, and the value recorded in the run record's `model` field. Absent or present-but-empty is a startup error. |
| `PROCESSOR_MODEL_KEY` | *(none — optional)* | The model endpoint's bearer key. **Absent means no `Authorization` header is sent at all** — the point of the ruling, not an edge of it: a local runtime commonly needs none. **Present-but-empty is still a startup error**, exactly like every required member — an empty value is a mistake, never a way to spell "no auth", and treating it as absent would be a silent auth downgrade. Never logged, never echoed in an error, never written to the graph. |

```sh
PROCESSOR_HTTP_ADDR=:9090 \
PROCESSOR_DIVOID_URL=https://divoid.example/api \
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

`input` must be non-empty; `subject` is the id of the node the run is about. The retrieval query sent to
the graph is `input`, verbatim — no rewriting, no expansion, no model.

The turn: fetch the subject and recall candidates, assemble a byte-budgeted context block (anchor first,
then admitted candidates sorted by node id ascending, never by score), judge it against the configured
model, dispatch the one supplementary-recall tool as the model asks for it (up to a call cap of 3 model
calls, so at most 2 tool dispatches per run — the capping call's recall is counted but never dispatched),
then write the record back to the graph as one `session-log` node linked to the subject.

Response (`200`) is the run record: the input, the query, the anchor summary, **every** candidate the
recall query returned — not only the ones kept — each with its rank, similarity, size, content hash, and
whether it was included or cut and why, the assembled block itself, the model's answer, the model id, every
supplementary-recall round (query, and for every row the round returned — not only the admitted ones — the
same rank/id/type/name/similarity/size/content-hash/included/cut-reason columns the candidates carry, or an
error if the round was malformed or failed), how many model calls were made and whether the per-run call cap
was reached (`capReached`), token usage as one entry per model call, in call order, named for the direction
of travel (`inTokens`/`outTokens` — a `null` entry means that call's endpoint reported no usage object,
absent, never zero-filled), the stop reason (both the loop's own neutral value and the endpoint's raw
string), the write-back outcome (the written node's id, or the reason it was not written — a write-back
failure does not fail the request; the record already carries everything of value), and the five constants
that governed the run (`limits`: candidate limit, assembly byte budget, supplementary byte budget, max model
calls, max output tokens).

Errors use a small closed envelope, `{"error":{"code":"...","message":"..."}}`:

| Code | Status | Meaning |
|---|---|---|
| `invalid_request` | 400 | Body unparseable, or `input`/`subject` missing or empty |
| `subject_not_found` | 404 | The subject id resolves to nothing |
| `graph_unavailable` | 502 | The graph could not be read |
| `model_unavailable` | 502 | The model call did not complete (transport failure, non-2xx status, or an undecodable response) |

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
A passing run prints one `ok` line per package (`cmd/processor`, `internal/divoid`, `internal/loop`,
`internal/openaicompat`, `internal/server`) and every `--- PASS:` line for each test. The default suite is
fully offline and hermetic: no network call, no credential, no live graph, no live model, no spend — every
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

  Expects one `ok` line per package — **four** as of unit A, which adds `internal/divoid` and
  `internal/loop` — no `?`, exit `0`. One 1.32 GB `golang:1.27` image pull, once.
- **What is still not covered:** `run()`'s serve-error exit branch and a second interrupt arriving
  during the shutdown drain — both are structurally unreachable from outside the process without adding
  a slow route or a delay knob to shipping code, which is declined (design
  `docs/architecture/process-boundary-test-harness.md` §2.4) — and the drain-failure axis (`server.go`'s
  `shutdownGrace` / `ReadHeaderTimeout` / shutdown-error propagation), which is a different instrument
  tracked separately as DiVoid **#10489**.
- **Mechanical context assembly (`POST /runs`, unit A):** `Assemble` is byte-pinned by an offline golden
  test (fixed candidate rows in, one exact block out), and separately: admission stops rather than
  back-fills, every candidate is hashed and sized including the cut ones, and the render order is by
  node id — a score reshuffle does not move a byte. `internal/divoid`'s two read operations are pinned
  at the wire level against a local test server, including the C30 empty-result discrimination (a
  missing subject is a `200` with an empty result, never a `404`). **Not verified here:** an end-to-end
  run against the live DiVoid graph — the design's own measured facts (`docs/architecture/m1-skeleton-loop.md`
  §3) are the closest thing to that, and a first live run is the natural next check once this ships.
- **The judgement step and write-back (`POST /runs`, unit B):** `internal/openaicompat` is pinned at the
  wire level against a local test server — the exact request shape (model, messages, `max_tokens`, the one
  tool declaration), the `Authorization` header sent only when a key is configured, decoding of a
  text-only answer, a tool call, a truncated response, a refused response, and an unrecognised finish
  reason with the raw value preserved, and that a missing usage object decodes as absent rather than
  zero. `internal/loop`'s tool cycle, call cap, and every design §6.5 failure-path row (including that a
  recall failure produces no model call at all, and that a write-back failure does not fail the request)
  are pinned at the port level against canned graph and model doubles. `internal/divoid`'s write side (the
  three-POST sequence, the content-type header on the body POST, the bare-id body on the link POST, and
  that the adapter alone supplies the written node's type, name and edge) is pinned at the wire level.
  **Not verified here:** the OpenAI-compatible protocol's actual **200** response shape from a real
  implementation — no local runtime (Ollama, LM Studio, llama.cpp, vLLM, koboldcpp) was installed or
  listening on this machine when this was built (checked, not assumed: `ollama`/`lms`/`llama-server`/
  `vllm`/`koboldcpp` all absent from `PATH`, no `~/.ollama`, and `GET /v1/models` on `127.0.0.1:11434`,
  `:1234`, `:8000` and `:8080` all had nothing listening). The live check is therefore **not performed**,
  not merely unautomated — installing a runtime and running one real turn against it is the natural next
  step, and is deliberately a manual command rather than a gate (design §10.6): a live model is
  nondeterministic and cannot be what a per-change assertion runs against.
