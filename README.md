# Processor

Processor is a harness whose substrate is memory, not history (see `VISION.md`). **M0** built a minimal
Go service skeleton that starts, serves one HTTP endpoint, and shuts down cleanly
(`docs/architecture/m0-service-skeleton.md`). **M1 unit A** adds the first thing the project exists to
do: mechanical context assembly, with no model call yet
(`docs/architecture/m1-skeleton-loop.md`). Unit B — the model call and the write-back — is a separate,
later PR.

## What's here

- `cmd/processor` — the process entry point: reads the boot configuration, binds the listener, wires
  OS signals, constructs the graph adapter and the loop, owns the exit code.
- `internal/server` — the HTTP route table and the serve/drain lifecycle.
- `internal/loop` — the turn: mechanical context assembly (`Assemble`) and its sequencing (`Turn.Run`).
  A pure function with no I/O, no clock, no randomness — see `docs/architecture/m1-skeleton-loop.md` §9.
- `internal/divoid` — the graph adapter: reads the subject node and runs the semantic recall query
  against the DiVoid API.
- `GET /health` — returns `200` with `Content-Type: application/json` and body `{"status":"ok"}`.
- `POST /runs` — assembles context for one input against one subject node and returns the run record.
  No model call and no write-back in unit A (see "`POST /runs`" below).

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

**Shutdown on `Ctrl+C`:** in an interactive terminal, pressing `Ctrl+C` drives the same `os.Interrupt`
path as the automated console-interrupt probe described below, which measured the process logging
`shutdown started` then `shutdown complete` and exiting `0` (see "What is and isn't verified here").

### Configuration

Three environment variables, read once, in `main` — still the module's one environment read site
(`cmd/processor/config.go`):

| Variable | Default | Behaviour |
|---|---|---|
| `PROCESSOR_HTTP_ADDR` | `127.0.0.1:8080` | Listen address, used verbatim when set and non-empty. Set but empty is a startup error (an explicitly-emptied variable is treated as an operator mistake, not a request for the default). |
| `PROCESSOR_DIVOID_URL` | *(none — required)* | The DiVoid API base URL, used verbatim. Absent or present-but-empty is a startup error: there is no defensible default for a base URL. |
| `PROCESSOR_DIVOID_KEY` | *(none — required)* | The DiVoid API bearer key, used verbatim. Absent or present-but-empty is a startup error. Never logged, never echoed in an error, never written to the graph. |

```sh
PROCESSOR_HTTP_ADDR=:9090 \
PROCESSOR_DIVOID_URL=https://divoid.example/api \
PROCESSOR_DIVOID_KEY=xxxxxxxx \
./processor.exe
```

### `POST /runs`

Request:

```json
{"input": "free text", "subject": 12345}
```

`input` must be non-empty; `subject` is the id of the node the run is about. The retrieval query sent to
the graph is `input`, verbatim — no rewriting, no expansion, no model.

Response (`200`) is the run record: the input, the query, the anchor (subject node) summary, **every**
candidate the recall query returned — not only the ones kept — each with its rank, similarity, size,
content hash, and whether it was included or cut and why, and the assembled block itself (anchor first,
then admitted candidates sorted by node id ascending, never by score). Unit A's record has no `answer`,
`toolCalls`, `modelCalls`, `usage`, `stopReason` or `written` field — those arrive with unit B.

Errors use a small closed envelope, `{"error":{"code":"...","message":"..."}}`:

| Code | Status | Meaning |
|---|---|---|
| `invalid_request` | 400 | Body unparseable, or `input`/`subject` missing or empty |
| `subject_not_found` | 404 | The subject id resolves to nothing |
| `graph_unavailable` | 502 | The graph could not be read |

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
`internal/server`) and every `--- PASS:` line for each test. The default suite is fully offline and
hermetic: no network call, no credential, no live graph, no spend — every graph-facing test runs
against a local `httptest.Server`.

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
- **Signal-triggered shutdown (`Ctrl+C`):** covered by no automated test in M0 — not because it can't
  be, but because of where it would have to run. A console-interrupt test was built and measured to
  work on Windows: it drives a real `CTRL_BREAK_EVENT` into the process, ran 5/5 green, and kills the
  one mutant nothing else in the suite kills — the deletion of the `signal.NotifyContext` wiring. It is
  declined here because running it pops a console window on whoever's interactive desktop the tests run
  on, not because it is infeasible (design `docs/architecture/m0-service-skeleton.md` §9.5, §10.7). The
  containerised replacement — run where that side effect doesn't land on a person's desktop — is DiVoid
  **#10439**. The build/run/`curl` check above (under "Start, serve, shut down cleanly") is human
  corroboration of the HTTP path on this dev platform, not "the only reliable check" — and it says
  nothing about this signal path. The signal path carries no human corroboration here, and needs none:
  the out-of-tree C16 probe measured the interrupt-driven shutdown directly, which is what grounds the
  outcome described above.
- **Mechanical context assembly (`POST /runs`, unit A):** `Assemble` is byte-pinned by an offline golden
  test (fixed candidate rows in, one exact block out), and separately: admission stops rather than
  back-fills, every candidate is hashed and sized including the cut ones, and the render order is by
  node id — a score reshuffle does not move a byte. `internal/divoid`'s two read operations are pinned
  at the wire level against a local test server, including the C30 empty-result discrimination (a
  missing subject is a `200` with an empty result, never a `404`). **Not verified here:** an end-to-end
  run against the live DiVoid graph — the design's own measured facts (`docs/architecture/m1-skeleton-loop.md`
  §3) are the closest thing to that, and a first live run is the natural next check once this ships. Unit
  A makes no model call, so C37 (the model API's `200` response shape) does not arise in this unit at all
  — it is unit B's open question, recorded here only so its absence isn't mistaken for an oversight.
