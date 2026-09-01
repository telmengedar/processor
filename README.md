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

Go 1.27 or later. On this machine Go is installed at `C:\Program Files\Go\bin\go.exe` and is **not**
on the default shell `PATH`, so every command below is given in both forms.

## Build

```sh
# full path
"/c/Program Files/Go/bin/go.exe" build ./...

# or, after adding Go to PATH for the session
export PATH="/c/Program Files/Go/bin:$PATH"
go build ./...
```

A clean build prints nothing and exits `0`.

## Run

```sh
export PATH="/c/Program Files/Go/bin:$PATH"
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

One environment variable, read once, in `main`:

| Variable | Default | Behaviour |
|---|---|---|
| `PROCESSOR_HTTP_ADDR` | `127.0.0.1:8080` | Listen address, used verbatim when set and non-empty. Set but empty is a startup error (an explicitly-emptied variable is treated as an operator mistake, not a request for the default). |

```sh
PROCESSOR_HTTP_ADDR=:9090 ./processor.exe
```

## Test

```sh
export PATH="/c/Program Files/Go/bin:$PATH"
go test -count=1 -v ./...
```

`-count=1` disables Go's test cache; without it a re-run can print `(cached)` and execute nothing.
A passing run prints exactly two `ok` lines (one per package) and every `--- PASS:` line for each test.

## Format and vet

```sh
export PATH="/c/Program Files/Go/bin:$PATH"
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
