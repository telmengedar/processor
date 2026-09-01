# Processor

Processor is a harness whose substrate is memory, not history (see `VISION.md`). This is **M0**: a
minimal Go service skeleton that starts, serves one HTTP endpoint, and shuts down cleanly — the
foundation later milestones build on. Design: `docs/architecture/m0-service-skeleton.md`.

## What's here

- `cmd/processor` — the process entry point: reads the boot configuration, binds the listener, wires
  OS signals, owns the exit code.
- `internal/server` — the HTTP route table and the serve/drain lifecycle.
- `GET /health` — returns `200` with `Content-Type: application/json` and body `{"status":"ok"}`.

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

One environment variable, read once, in `main`:

| Variable | Default | Behaviour |
|---|---|---|
| `PROCESSOR_HTTP_ADDR` | `127.0.0.1:8080` | Listen address, used verbatim when set and non-empty. Set but empty is a startup error (an explicitly-emptied variable is treated as an operator mistake, not a request for the default). |

```sh
PROCESSOR_HTTP_ADDR=:9090 ./processor.exe
```

## Test

```sh
go test -count=1 -v ./...
```

`-count=1` disables Go's test cache; without it a re-run can print `(cached)` and execute nothing.
A passing run prints exactly two `ok` lines (one per package) and every `--- PASS:` line for each test.

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

  Expects exactly two `ok` lines, no `?`, exit `0`. One 1.32 GB `golang:1.27` image pull, once.
- **What is still not covered:** `run()`'s serve-error exit branch and a second interrupt arriving
  during the shutdown drain — both are structurally unreachable from outside the process without adding
  a slow route or a delay knob to shipping code, which is declined (design
  `docs/architecture/process-boundary-test-harness.md` §2.4) — and the drain-failure axis (`server.go`'s
  `shutdownGrace` / `ReadHeaderTimeout` / shutdown-error propagation), which is a different instrument
  tracked separately as DiVoid **#10489**.
