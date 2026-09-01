# Architectural Document: Processor — Process-Boundary Test Harness

> Repo path: `docs/architecture/process-boundary-test-harness.md` (canonical copy — the DiVoid node
> carries the same document verbatim).
> Task: DiVoid **#10439** · Project: **#10422** · Standing rule: **#10440** · Predecessor design:
> **#10437** (M0), whose §9.5 U1/U2 and §10.7 this document closes.
> Standards applied: Design Contracts **#1136**, Code Contracts **#114 §0**, DRY threshold **#1267**,
> anti-seed-complexity **#1184**, vocabulary rule **#1220 §2**, run-certification incident **#10351**.
> Every fact in §3 was measured on this machine on 2026-09-01, in the container this design names, with
> the commands quoted. One hypothesis this document was going to assert was **measured false** before it
> was written and is recorded as such in §3 M12 — that is the section's purpose.

---

## TL;DR

**What.** The M0 signal path, `main`'s exit codes and its configuration-error branch are pinned by
nothing (#10437 §9.5, U1–U4; `main.go` is 0/22 statements). This design closes all of that except the
drain-failure axis, which is a different instrument and a different PR (§2.3).

**How.** **One new file and no new production code.** A Linux-only test file,
`cmd/processor/process_linux_test.go`, runs the real built binary as a **child process** and observes it
from outside: exit code, and the ordered records on its stderr. Three test functions, four scenarios.
Go's `_linux_test.go` suffix excludes the file from the Windows default run **at toolchain level** — it
lands in `IgnoredGoFiles`, not behind a runtime skip (measured, §3 M8).

**The venue decision is the load-bearing one: the container runs `go test`, rather than a test reaching
out to a container.** The whole suite executes inside a stock `golang:1.27` image over a read-only
bind-mount of the working tree — measured, 9 seconds cold, exactly two `ok` lines (§3 M2). Because the
container is the *runner*, no Go code ever asks whether Docker is available, so the
`docker info --format` exit-0-with-empty-stdout trap (#10439, #10440) **has no site to occur at**. A dead
or unreachable engine fails the operator's own command with exit 1 and a connect error on stderr
(measured, §3 M7).

**Cost.** One test file (~150 lines, a forecast, not a measurement). Three README lines and one edit to
its "What is and isn't verified here" section. One 1.32 GB image pull, once. Zero production-code
changes, zero new environment variables, zero build tags, zero flags, zero skip guards.

**Strongest rejected alternative.** A host-side Go test that shells out to `docker run`. Rejected because
it must decide, at runtime, what to do when Docker is absent — and **both available answers break a
stated constraint**: a skip is "a green line that means nothing" (#10437 §10.7), and a hard failure makes
the default `go test ./...` require Docker (#10440). There is no honest third answer. Notably it is *not*
rejected on the discriminator, which survives the CLI boundary fine (§3 M12). Full comparison in §10.9.

---

## 1. Problem Statement

Toni's ruling, 2026-09-01, verbatim (#10440):

> "okay - next time (there was a window coming up here) - smoke tests, live executions whatever - we have
> a docker, use that, no interruptions and you probably can handle that better than a live windows system
> where a user is interacting all the time."

M0 shipped with the signal path pinned by nothing. The instrument that would pin it on Windows was built
and measured — 5/5 green, and it kills the one mutant nothing else kills — and was **declined**, because
generating a console control event surfaces a window on the desktop somebody is working at (#10437 C16,
C17). At the time the containerised replacement could not be designed either: no Docker engine was
reachable, and a shape derived on a machine that cannot execute it is the same defect in a new location.

**That premise has changed, and it was verified end to end rather than taken from Docker's own report.**
A container was pulled, started, executed and exited (#10440 update; independently re-measured here,
§3 M1–M2). This design is what the changed premise licenses.

### What must become true

| # | Goal | Source |
|---|---|---|
| G1 | An interrupt makes the **process** run the graceful path and exit 0 | #10437 §9.5 **U1** |
| G2 | `SIGTERM` reaches the same wiring — the leg Windows can never deliver | #10437 §9.5 **U2** |
| G3 | The configuration-error branch of `run()` exits non-zero and names the offending variable | #10439 W2, survivor "config error ignored" |
| G4 | The bind-error branch of `run()` exits non-zero | #10439 W2, survivor `main.go:31` exit `1` → `0` |
| G5 | The default `go test ./...` on a developer machine still requires no Docker and opens no window | #10440, #10437 §10.7 |
| G6 | A missing or dead daemon fails or skips **visibly**, never quietly green | #10439, the #10351 shape |

`main.go:31` is the **listen** branch's `return 1`, and `main.go:34` is the `signal.NotifyContext` line —
read from `origin/main`, not inferred from the mutation report's line numbers.

---

## 2. Scope & Non-Scope

### 2.1 In scope

- One Linux-only test file in the existing `cmd/processor` package, covering G1–G4.
- The documented command that runs the suite in a container, covering G5–G6.
- The README's "What is and isn't verified here" section, which currently describes the gap this work
  closes and would otherwise become false the moment this ships.

### 2.2 Out of scope — declined explicitly, not by omission

| Excluded | Why, and where it belongs |
|---|---|
| The drain-failure axis — `ReadHeaderTimeout`, `shutdownGrace` 1ns/30s, shutdown-error propagation | A different instrument. §2.3 is the ruling |
| U3 — a second interrupt during the drain | Needs **both** instruments at once (§2.4). Stays open, stays withdrawn-not-reversed |
| `main.go:39` — the serve-error exit branch | Structurally unreachable from outside the process (§2.4) |
| A Dockerfile, an image build, a registry, CI | #10437 §2 excludes deployment and §10.7 is explicit that using a stock image as an execution venue introduces none of these. Unchanged |
| A `scripts/` directory, a Makefile, a task runner | §5.3 |
| Any change to `cmd/processor` or `internal/server` production code | §5.3. The harness observes the binary that ships; it does not adapt it |

### 2.3 Ruling on the unit split — **two units, two PRs**

The question is decided on the merits. Sequencing no longer forces anything: PR #3 merged, so the
`httpServer` seam is on `main` today (`Serve` delegates to an unexported `serve` behind a two-method
interface, with a fake in the tests) and the drain work is unblocked.

**Unit A — this design.** The signal path, the exit codes and the configuration-error branch are **one
instrument**, and the operator's reading is right. All four scenarios need the same three things: the
real binary, run as a real process, observed from outside. Nothing in the module can see any of them any
other way — `run()` is not called by any in-process test and cannot be, because it terminates the
process.

**Unit B — the drain-failure axis.** In-process tests in `internal/server`, driving `serve` with the
merged fake and/or a deliberately blocking handler, asserting a return value.

**The decisive argument is structural, not stylistic: Unit A's instrument cannot reach Unit B's axis at
all.** Making the drain exceed its five-second grace requires an in-flight request that outlives it. The
shipping binary serves exactly one route, `/health`, which returns immediately and cannot be made slow
from outside the process. The only way to give Unit A that reach would be to add a slow route, or a
delay knob, to production code — **which is precisely what #1220 §2 forbids and what the removal of the
fake's `shutdownErr` field before PR #3 merged has just re-established**. A test affordance declared in
shipping code at zero present value is the same defect whether it is a struct field or a route.

Three supporting reasons, each independently sufficient to keep them apart:

| | Unit A | Unit B |
|---|---|---|
| **Package** | `cmd/processor` | `internal/server` |
| **Observation** | Exit code + stderr across a process boundary | A returned error value |
| **Platform reach** | Linux-only by construction — it delivers signals | Platform-independent; belongs in the default run on every machine |
| **Gate it runs behind** | The container command | `go test ./...` everywhere |
| **Review question** | "Does this observe the process, and does it die on the mutant?" | "Does the fake faithfully stand in for `*http.Server`?" |

They share no helper and no file. Bundling them would put a platform-independent test behind a container
gate for no reason, and would ask one reviewer two unrelated questions in one diff — the shape #10192
names as why the QA loop oscillates.

### 2.4 What stays open after both units, and why

**This is stated so it is not mistaken for covered.**

| Gap | Why neither unit reaches it | Falsifier, for whoever picks it up |
|---|---|---|
| **U3** — a second interrupt during the drain | It is a *process*-level observation (Unit A's instrument) that requires a *slow drain* (Unit B's affordance). Neither unit has both | Deliver two interrupts while a deliberately slow request is draining. Immediate termination confirms the withdrawn claim; completing the drain and exiting 0 falsifies it |
| **`main.go:39`** — `run()`'s serve-error exit | Reaching it requires `Serve` to return non-`nil`, which requires a drain timeout, which requires a slow handler the binary does not have | Run the binary with a route that outlives the grace deadline and assert exit code 1 plus a `msg=serve` record at Error. That route does not exist and must not be added for this purpose |

Both remain unverified after this work. A gap with a falsifier is an open question; a gap with a story
about why it cannot be closed is what #10437 §9.5 used to be.

---

## 3. Measured Facts

Everything below was executed on this machine on 2026-09-01. Probes ran in
`C:\dev\claude\_scratch\sarah-sigharness-q4v9\` against a copy of the module and have been removed.

### 3.1 The venue

| # | Fact | Consequence |
|---|---|---|
| M1 | `docker --context desktop-linux info` → `server=29.6.2 os=linux arch=aarch64 ncpu=12`, exit 0. `docker context ls` shows `default` = `ssh://Max@bazzite`, `desktop-linux` = the local named pipe, and prints the warning *"DOCKER_HOST environment variable overrides the active context"* | The venue exists and is `linux/arm64`, confirming the ARM finding on #10439 from the engine's own report |
| M2 | `docker run --rm -v <repo>:/src:ro -w /src golang:1.27 go test -count=1 ./...` → `go version go1.27.0 linux/arm64`, then **exactly two `ok` lines**, exit 0. **9 seconds wall clock, cold**, including container start and a full compile | The entire existing M0 suite runs unmodified on `linux/arm64`. #10437 §9.6's "exactly two `ok` lines, no `?`" criterion holds in the container as well as on Windows |
| M3 | `--context desktop-linux` **does** override `DOCKER_HOST`: with `DOCKER_HOST=ssh://Max@bazzite` still set, `docker ps` exits **1** but `docker --context desktop-linux ps` exits **0** | #10439's advice is correct as written. Either remedy works |
| M4 | `DOCKER_HOST=` (set to the **empty string**, not unset) also resolves to the local engine — `DOCKER_HOST= docker ps` exits 0 | A one-token, context-name-free neutraliser. It needs no machine-local context name in the repo, which is why §8.1 documents this form |
| M5 | In Git Bash, `-v "$PWD":/src` requires `MSYS_NO_PATHCONV=1`; without it msys rewrites the container-side path and Docker rejects `'C:/Program Files/Git/src' is invalid` | A shell-specific prefix, not a repo concern. Recorded so nobody debugs it twice |
| M6 | In PowerShell, `$env:DOCKER_HOST=''` then `docker run --rm -v "${PWD}:/src:ro" -w /src golang:1.27 go test -count=1 ./...` → two `ok` lines, exit 0 | The primary shell on this machine works with no path mangling |
| M7 | Against an unreachable endpoint (`DOCKER_HOST=tcp://127.0.0.1:1`), `docker run --rm alpine true` prints `error during connect: … connection refused` and exits **1** | **G6 is satisfied by the runner, with nothing written.** The failure is loud, immediate, and legible |

### 3.2 The toolchain boundary

| # | Fact | Consequence |
|---|---|---|
| M8 | With a `*_linux_test.go` file present in `cmd/processor`, on Windows `go list -f '…'` reports `GoFiles=[config.go main.go] TestGoFiles=[config_test.go] IgnoredGoFiles=[probe_linux_test.go]` | Excluded **by the toolchain**, not by a runtime skip. G5's "must not require Docker" is satisfied by the file name alone |
| M9 | With that file present, on Windows: `go test -count=1 ./...` → two `ok` lines, exit 0; `go vet ./...` → silent, exit 0; `gofmt -l .` → silent, exit 0 | The default run is unchanged in every observable way. The M0 acceptance criterion survives |
| M10 | `gofmt -l .` **does** inspect the Linux-only file on Windows (it formats regardless of build constraints), but `go vet` **does not** — the file is in `IgnoredGoFiles` | Formatting stays gated on Windows; **vetting does not**, and §8.1's command closes that gap deliberately |
| M11 | Inside the container, `go vet ./...` over the tree including the Linux-only file exits 0 and prints nothing | The container pass is a real vet gate, not a formality |
| M20 | `exec.LookPath("go")` from inside the test binary resolves to `/usr/local/go/bin/go` in the container; `go build -o <t.TempDir()>/processor .` succeeds with the module mounted **read-only**, because the output goes outside the mount | The test can build the binary it tests, with no orchestration outside `go test` and no writable source tree |

### 3.3 The discriminator — measured, because the Windows tell does not transfer

On Windows the tell was `exit status 0xc000013a` — a *positive, specific* code. On Linux it is different
in kind, and simpler:

| # | Scenario | `Wait()` error | `ExitCode()` | `Signaled()` | `Exited()` |
|---|---|---|---|---|---|
| M12a | Shipping binary, `SIGINT` | `<nil>` | **0** | false | true |
| M12b | Shipping binary, `SIGTERM` | `<nil>` | **0** | false | true |
| M12c | `signal.NotifyContext`-deleted mutant, `SIGINT` | `signal: interrupt` | **−1** | true | false |
| M12d | Same mutant, `SIGTERM` | `signal: terminated` | **−1** | true | false |

**A signal-terminated process reports `ExitCode() == −1`, never 0**, because Go returns −1 whenever
`Exited()` is false. So on Linux, `ExitCode() == 0` is *by itself* an unambiguous discriminator between
"handled the signal" and "was killed by it" — no `Signaled()` inspection is needed, and the assertion is
simpler than the Windows one it replaces. The stderr-record assertion (§8.3) is kept anyway, because
#10437 §9.5 U1's falsifier demands both and because exit 0 alone cannot distinguish a graceful shutdown
from a process that exited cleanly for some other reason.

In M12a/b the full stderr was, in order:
`msg=listening addr=127.0.0.1:<port>` → `msg="shutdown started"` → `msg="shutdown complete"`.
In M12c/d only the `listening` record was present. **Both assertions kill the mutant independently.**

### 3.4 The process contract

| # | Fact | Consequence |
|---|---|---|
| M13 | `PROCESSOR_HTTP_ADDR=127.0.0.1:0` → the binary logs its **actual** bound port, e.g. `msg=listening addr=127.0.0.1:36753`, and serves there | The harness needs no fixed port. It consumes #10437 §6.1 step 6, an existing designed property — no new affordance |
| M14 | A child launched with an **explicit minimal environment** — only `PROCESSOR_HTTP_ADDR` — starts, serves and shuts down normally | The process under test can be made hermetic. The developer's ambient `PROCESSOR_HTTP_ADDR` cannot influence a test |
| M15 | A child launched with an **entirely empty** environment takes the documented default `127.0.0.1:8080` and shuts down cleanly on `SIGINT` | The binary needs nothing else from the environment; the default path is confirmed at process level as a side effect |
| M16 | An explicit `PROCESSOR_HTTP_ADDR=` (set, empty) passed through the child's environment → exit **1**, stderr `level=ERROR msg="boot configuration" error="PROCESSOR_HTTP_ADDR is set but empty"` | G3's assertion is exactly expressible, and set-but-empty survives an explicit environment slice |
| M17 | Starting a second instance on an address already held → exit **1**, stderr `level=ERROR msg=listen addr=127.0.0.1:33747 error="listen tcp …: bind: address already in use"` | G4 is expressible without a fixed port: the first instance's port-0 address is the second's input |
| M18 | The **config-error-ignored** mutant does **not** exit. It binds `[::]:44511` — all interfaces — and serves until killed (`timeout 3` → 124) | **The configuration-error test must use a bounded wait, never a blocking one.** A bare wait would hang until the package's test timeout and report the wrong thing. Recorded because it is the one place this harness could quietly become useless |
| M19 | Four scenarios plus a build each: whole package run **1.4 s** in-container, ~0.35 s per scenario, no observed flake | Cheap enough that no shared-state optimisation is warranted (§10.4) |

### 3.5 One hypothesis measured false before it was written down

| # | Claim this document was going to make | What measurement returned |
|---|---|---|
| M21 | *"A container's PID 1 has no default signal disposition, so an unhandled `SIGINT` to it is discarded and the mutant would hang rather than die — which is why the host-side `docker run` alternative loses its discriminator."* | **False.** Running the `signal.NotifyContext`-deleted mutant as PID 1 and issuing `docker kill --signal=SIGINT` gave `Running=false ExitCode=130` — the default action applied. The kernel's PID-1 protection covers signals sent from *within* the process's own PID namespace; `docker kill` sends from the host namespace, an ancestor, so the default action is applied normally |

The rejection of that alternative therefore **does not rest on the discriminator**, which works. It rests
on §10.9's availability argument, which is structural. Recorded rather than quietly corrected, because a
rejection propped up by a false cost is a decision nobody can re-examine.

---

## 4. Architectural Overview

```
  DEFAULT DEVELOPER RUN                     THE LINUX GATE
  (Windows or any host)                     (one documented command)

  go test -count=1 ./...                    docker run --rm -v <tree>:/src:ro -w /src \
        |                                     golang:1.27 sh -c 'go vet ./... && go test -count=1 ./...'
        |                                             |
        v                                             v
  +---------------------------+             +--------------------------------------+
  | toolchain reads the tree  |             |  container: linux/arm64, stock image  |
  |                           |             |                                      |
  | *_linux_test.go lands in  |             |   go test  (the SAME suite, plus the |
  | IgnoredGoFiles  [M8]      |             |             *_linux_test.go file)    |
  |                           |             |          |                           |
  | -> 2 ok lines, no Docker, |             |          v                           |
  |    no window       [M9]   |             |   builds the binary  [M20]           |
  +---------------------------+             |          |                           |
                                            |          v                           |
                                            |   +--------------------------+       |
                                            |   | child process:           |       |
                                            |   |   the real ./processor   |       |
                                            |   |   hermetic env    [M14]  |       |
                                            |   |   PROCESSOR_HTTP_ADDR    |       |
                                            |   |     = 127.0.0.1:0 [M13]  |       |
                                            |   +--------------------------+       |
                                            |      |  stderr        ^  signal      |
                                            |      v                |              |
                                            |   observed: exit code [M12]          |
                                            |             ordered records          |
                                            +--------------------------------------+
```

Three properties of this picture are the design:

1. **The container runs the test; the test does not run the container.** Everything in §10.2 follows.
2. **The binary under test is a child of the test process, never PID 1.** Signal disposition, exit-status
   reporting and stderr capture are all ordinary Go, with no container-runtime semantics in the path.
3. **The two runs differ by exactly one file.** The Linux gate is the same suite plus one file, so a
   reader comparing the two outputs is comparing like with like.

---

## 5. Components & Responsibilities

### 5.1 `cmd/processor/process_linux_test.go` — the observer

| | |
|---|---|
| **Owns** | Building the binary under test · launching it with a hermetic environment · establishing that it served · delivering one signal · observing exit status and stderr · asserting the four scenario contracts (§8.4) |
| **Does not own** | Anything about the container. It contains no reference to Docker, no image name, no availability check, and no knowledge that a container exists |
| **Package-level state** | None (§10.4) |

**Why this file, in this package, under this name.**

- **`cmd/processor`, not a new package.** A new package whose only test file is platform-tagged emits
  `? <pkg> [no test files]` on the other platform, which breaks #10437 §9.6's glanceable criterion. In
  the existing package the Windows run still prints exactly two `ok` lines and no `?` (M9).
- **`_linux_test.go`, not a build tag and not a runtime skip.** The suffix is a toolchain-level GOOS
  constraint (M8). A runtime skip would be a green line that means nothing — #10351's shape wearing
  different clothes, and forbidden by #10437 §10.7.
- **`process_`, not `signal_`.** The file's subject is the process boundary: signals are one of its four
  scenarios, and exit codes are the other three.

### 5.2 The Linux gate — a documented command, not an artifact

The venue is **one command in the README** (§8.1). It is not a script, not a Makefile target, not a
wrapper. It has no state, no configuration and no failure mode of its own beyond the engine's.

### 5.3 What is deliberately NOT built

| Not built | Why not |
|---|---|
| A Docker-availability probe, in Go or in shell | §10.2. This is the single most important omission in the document |
| A `scripts/` or `tools/` directory, a Makefile, a task runner | One command that fits on one line does not need a runner. The can-it-be-deleted check (#1136 §4) deletes it |
| A `Dockerfile` | The venue is a stock image. #10437 §10.7 is explicit that this introduces no image build, no registry and no deployment story |
| An environment variable or flag to opt the Linux tests in or out | The file name already decides it, at toolchain level. A knob would be a second, weaker mechanism for a decision already made — and one no operator has asked to tune (#1136 §3) |
| A slow or configurable route, a delay knob, or any test affordance in shipping code | #1220 §2, freshly re-established by the removal of the fake's `shutdownErr` field before PR #3 merged. It is also what makes §2.3's split structural |
| A helper that skips when `/health` is unreachable | The readiness step is an assertion, not a precondition. If the binary never serves, that is a failure, not a reason to stop testing |
| A digest-pinned image reference | §8.1. The tag is derived from `go.mod`; a digest adds a maintenance obligation with no named beneficiary |
| Any change to `main.go`, `config.go`, `server.go` or `routes.go` | The harness observes what ships. If a scenario cannot be expressed against the shipping binary, that is §2.4's answer, not a production change |

---

## 6. Interactions & Data Flow

All four scenarios share the same skeleton; they differ in what they do in the middle.

### 6.1 The graceful-shutdown scenario (G1, G2) — two subtests, one body

1. Build the binary from the package directory into a per-test temporary directory (M20).
2. Launch it with an **explicit environment containing exactly one variable**,
   `PROCESSOR_HTTP_ADDR=127.0.0.1:0`, capturing stderr (M13, M14).
3. **Await readiness**, and it is two steps for two different reasons: read the actual bound address out
   of the `listening` record, then issue a real `GET /health` against it until it answers. The first
   step is how the address becomes known at all; the second establishes that the process genuinely
   **served** before anything asserts that it stopped — Code Contracts §13.1.1, the same discipline
   #10437 §9.4 applies to its own negative test.
4. Deliver one signal to the child — `os.Interrupt` in one subtest, `SIGTERM` in the other.
5. **Await exit under a deadline** and assert: exit code exactly 0 (M12), and the three lifecycle records
   present in order — `listening`, `shutdown started`, `shutdown complete`.

### 6.2 The configuration-error scenario (G3)

1. Build.
2. Launch with an explicit environment of exactly `PROCESSOR_HTTP_ADDR=` — set, empty (M16).
3. There is no readiness step: the process is expected never to serve.
4. **Await exit under a deadline** and assert exit code 1 and an `ERROR`-level `boot configuration`
   record naming `PROCESSOR_HTTP_ADDR`.

**The deadline is not a formality here.** M18 measured that the mutant this scenario exists to kill does
not exit at all — it binds every interface and serves. A blocking wait would hang to the package timeout
and report the wrong thing.

### 6.3 The bind-error scenario (G4)

1. Build; launch a first instance on port 0 and await readiness, exactly as §6.1 — this is what makes a
   known-occupied address available without ever naming a fixed port.
2. Launch a second instance whose explicit environment sets `PROCESSOR_HTTP_ADDR` to the first
   instance's **actual** address.
3. **Await exit under a deadline** and assert exit code 1 and an `ERROR`-level `listen` record carrying
   that address (M17).
4. Terminate the first instance and reap it. Teardown asserts nothing.

---

## 7. Data Model (Conceptual)

None. This design persists nothing, models no entity and introduces no type that outlives a test
function. The only data crossing a boundary are a process's exit status and its stderr byte stream, both
described in §8.3.

---

## 8. Contracts & Interfaces (Abstract)

### 8.1 The venue contract

| Aspect | Contract |
|---|---|
| **Image** | `golang:1.27` — the stock image, no build, no registry |
| **Tag derivation** | The tag tracks `go.mod`'s `go` directive. **When that directive moves, the tag moves in the same commit.** One version, stated in two places, kept in step by a rule rather than by memory |
| **Mount** | The working tree at `/src`, **read-only**, working directory `/src` |
| **Command** | `go vet ./...` then `go test -count=1 ./...` |
| **Certifying form** | `go test -count=1 -v ./...`, per #10437 §9.6. `-count=1` is not optional in any run whose output is quoted as evidence |
| **Expected output** | Exactly two `ok` lines, no `?` line, no `(cached)` — the same criterion as the Windows run (M2, M9) |
| **Availability handling** | **None, deliberately.** §10.2 |

The commands, in the two shells that exist on this machine, both measured:

```
# PowerShell (M6)
$env:DOCKER_HOST=''
docker run --rm -v "${PWD}:/src:ro" -w /src golang:1.27 sh -c 'go vet ./... && go test -count=1 ./...'

# Git Bash (M4, M5)
MSYS_NO_PATHCONV=1 DOCKER_HOST= docker run --rm -v "$PWD":/src:ro -w /src golang:1.27 \
  sh -c 'go vet ./... && go test -count=1 ./...'
```

**Three details in those lines each pay for themselves, and none is a knob.**

- **`DOCKER_HOST=`** (empty, M4) rather than `--context desktop-linux` (M3). Both work. The empty
  assignment names no machine-local context, so the command in the repo stays true on a machine with a
  different context layout, while still neutralising the `ssh://Max@bazzite` override that is present in
  this environment and points at a host that is usually off. The context form belongs in a developer's
  shell, not in the repo.
- **`:ro`** — one token. The container runs as root over the developer's source tree; read-only makes
  "the gate cannot modify the working tree" true rather than hoped, and it is measured compatible with
  `go build` and `go test` because both write outside the mount (M20).
- **`go vet` before `go test`** — M10 is the reason and it is precise. `gofmt -l .` on Windows *does*
  inspect the Linux-only file, so formatting needs no Linux pass; `go vet` on Windows *cannot* see it.
  Without this the file would be the one file in the module that is never vetted.

### 8.2 The child-process contract

| Aspect | Contract |
|---|---|
| **Binary** | Built from the package under test into a per-test temporary directory. Never a checked-in artifact, never a shared path |
| **Environment** | **Explicit and minimal — the ambient environment is never inherited by the child.** At most one variable, `PROCESSOR_HTTP_ADDR` (M14, M15) |
| **Address** | `127.0.0.1:0` wherever the process is expected to serve. **No test names a port** (#10437 §9.1) |
| **Address discovery** | Read from the child's `listening` record, which by #10437 §6.1 step 6 carries the **actual** bound address. The harness consumes an existing property; it adds none |
| **Readiness** | A real `GET /health` returning 200. Polled, never slept-for |
| **Termination** | Every launched process is either asserted to have exited or is killed and reaped before the test returns. No test leaves a process behind |
| **PID** | Always a child of the test process, never a container's PID 1 (§4, M21) |

### 8.3 The observation contract

Two channels, and each answers a different question.

| Channel | What it answers | How it is asserted |
|---|---|---|
| **Exit status** | *Did the process reach its own exit path, or was it killed?* | `ExitCode()` compared to an exact expected value. **0 is reachable only by a process that exited on its own** — a signalled process reports −1 (M12) |
| **stderr records** | *Did it take the path we claim, in the order we claim?* | Presence and **relative order** of named records |

**Assert a record's identity, not its rendering.** A record is identified by its level and its message —
`msg=listening`, `msg="shutdown started"`, `msg="shutdown complete"`, `level=ERROR msg="boot
configuration"`, `level=ERROR msg=listen` — plus, where the contract names a value (the offending
variable, the conflicting address), that value's presence. Timestamps and the handler's exact spacing are
not part of any contract and are not asserted; pinning them would make an unrelated logging change break
these tests.

**Every wait on the child is bounded** and its failure message says what did not happen — not "timeout",
but which record never appeared or which exit never came. M18 is why: the mutant under test may simply
keep running.

### 8.4 Scenario contracts

| # | Scenario | Child environment | Expected exit | Expected records |
|---|---|---|---|---|
| 1 | Interrupt after serving | `PROCESSOR_HTTP_ADDR=127.0.0.1:0` | **0** | `listening` → `shutdown started` → `shutdown complete`, in order |
| 2 | `SIGTERM` after serving | same | **0** | same |
| 3 | Configuration error | `PROCESSOR_HTTP_ADDR=` (set, empty) | **1** | `ERROR` `boot configuration`, naming `PROCESSOR_HTTP_ADDR` |
| 4 | Bind error | `PROCESSOR_HTTP_ADDR=` the first instance's actual address | **1** | `ERROR` `listen`, carrying that address |

---

## 9. What This Pins, and What Would Falsify It

### 9.1 Mutant matrix — the instrument must be able to fail

A test that cannot fail is decoration. Each row below was measured (M12, M16, M17, M18) or is entailed by
a measured row.

| Mutation | Source | Killed by | How it dies |
|---|---|---|---|
| `signal.NotifyContext` deleted (`main.go:34`) | #10439 W2 | Scenarios 1 **and** 2 | Exit code −1 instead of 0 (M12c/d); and the two shutdown records absent. **Two independent kills** |
| Configuration error ignored (`main.go:23`) | #10439 W2 | Scenario 3 | The process never exits; the bounded wait fails naming the exit that never came (M18) |
| `main.go:25` `return 1` → `return 0` | same branch | Scenario 3 | Exit code 0 instead of 1 |
| `main.go:31` `return 1` → `return 0` | #10439 W2 | Scenario 4 | Exit code 0 instead of 1 |
| `SIGTERM` dropped from the notify list | — | Scenario 2 only | Exit code −1. **This is the leg Windows can never pin** (#10437 §9.5 U2) |
| The `listening` record's address changed to the *configured* value rather than the actual one | — | Scenarios 1, 2, 4 | With port 0 the configured value is `…:0`, which is not connectable; readiness fails |

**Verification step for the implementer, and it is not optional:** before handing off, delete
`signal.NotifyContext` locally, run the Linux gate, confirm scenarios 1 and 2 **fail**, and restore. A
suite that has never been seen to fail has not been shown to be an instrument. This is the same check
#10437 §14 step 3 required of the lifecycle's paired negative test.

### 9.2 Claims this design makes that are not measured

| Claim | Falsifier |
|---|---|
| An image whose Go predates `go.mod`'s directive fails loudly rather than silently downloading a toolchain | Run the §8.1 command against `golang:1.22` and observe whether it errors or fetches a toolchain. `GOTOOLCHAIN` defaults to `auto`, so the silent-download outcome is plausible and **this document does not assert either way** |
| The file size forecast (~150 lines) | Count it after implementation. This is a forecast and is marked as one; #10446's lesson is that a forecast restated in the indicative is a defect |
| The scenarios are non-flaky over many runs | Measured once at 5 scenarios in 1.4 s with no flake (M19). Falsifier: run the gate 20 times; any intermittent failure falsifies it. Not run here, and not claimed as run |

---

## 10. Cross-Cutting Concerns

### 10.1 The platform predicate and the harm predicate coincide

#10440's rule is scoped to a measured harm: a run that **reaches the interactive desktop** — a window, a
console control event, stolen focus — or that **binds beyond loopback**.

On Windows, delivering a signal requires generating a console control event, which requires a console:
harm and mechanism coincide, which is why M0's probe was declined. **On Linux they come apart.** Sending
`SIGINT` to your own child process opens nothing, steals nothing and binds nothing; every scenario here
binds `127.0.0.1:0`. So a developer on a Linux workstation running `go test ./...` natively runs these
tests with no harm at all — and needs no guard, no environment variable and no opt-in to be safe.

**That is why the file name is the whole mechanism.** The GOOS constraint excludes the file exactly where
the harm exists and includes it exactly where it does not. Any additional gate would be a second,
weaker expression of a decision the toolchain already makes correctly (#1136 §4; #1220 §2).

One consequence stated plainly: scenario 3's mutant binds every interface when it misbehaves (M18). In a
container that is contained. On a Linux desktop it would be a momentary all-interfaces bind by a mutated
build that only exists during a deliberate mutation check — noted, not mitigated, because mitigating it
would mean adding the guard this section just declined.

### 10.2 No availability probe exists, and that is the design

**#10439's sharpest constraint is G6, and this design satisfies it by removing the site where it could
be violated rather than by handling it.**

The trap is specific: `docker info --format '{{.ServerVersion}}'` exits **0 with empty stdout** when the
engine is down. Any Go test that probes for Docker and decides skip-or-fail is one careless probe away
from being permanently, silently green — the #10351 shape.

Because the container is the *runner*, **no Go code in this repository ever asks whether Docker is
available.** The operator runs one command. If the engine is dead, unreachable, or pointed at a host that
is off, that command prints a connect error and exits 1 (M7). There is no probe to get wrong, no skip to
be silently taken, and no branch to review.

**This is also why the alternative in §10.9 loses.** It cannot avoid the question; it can only answer it
badly in one of two ways.

### 10.3 The Windows run is unchanged, and that is checkable

M9 measured all three gates with the new file present: `go test -count=1 ./...` → two `ok` lines;
`go vet ./...` → silent; `gofmt -l .` → silent. **G5 is satisfied by construction and verified by
measurement, not by argument.**

The residual, stated because M10 makes it real: a Windows developer editing the Linux-only file gets
**no compile feedback** — the toolchain does not parse it, so a broken file is invisible until the gate
runs. Mitigation: the gate is one command and §14 requires it before handoff. This is a genuine cost of
platform-constrained files and it is accepted, not hidden.

### 10.4 Parallelism, state, and why there is no shared build

Every test marks itself parallel, consistent with #10437 §9.1. Nothing here shares mutable state: no
fixed port (§8.2), no package-level variable, no ambient environment read by the child (M14), and one
temporary directory per test.

**A shared build via `TestMain` is rejected.** It would centralise the three build calls behind a
package-level variable holding the binary path — package-level mutable state, which #10437 §9.1 forbids —
to save an amount that was measured and is negligible: Go's build cache makes repeat builds effectively
free, and the whole package ran in **1.4 s** with four builds in it (M19). The optimisation costs a rule
and buys nothing measurable.

### 10.5 DRY — the helper decisions, with the math

Per #1267 the threshold is `block_size × site_count` above ~15–20.

| Block | Size | Sites | Product | Decision |
|---|---|---|---|---|
| Build the binary into a temp dir and fail loudly on build error | ~9 | 3 test functions | **27** | Extract. Name it for what it returns: the path to the built binary |
| Launch with an explicit environment, read the address from the `listening` record, poll `/health` until it answers | ~21 | 2 (graceful, bind-error's first instance) | **42** | Extract. It returns the running process, its captured stderr, and its actual address |
| Wait for exit under a deadline and surface what did not happen | ~10 | 3 asserting sites | **30** | Extract |

All three are above the threshold and all three name cleanly in one to three words, which is #1267's
other test. Three helpers, no more: the assertions themselves differ per scenario and are written at each
site, where a reader can see them next to the scenario they belong to.

### 10.6 Security

Nothing here handles a credential. The container runs a stock image with no network service published
(no `-p`), a read-only source mount, and `--rm`. The processes it starts bind loopback inside the
container's own network namespace and are unreachable from the host.

### 10.7 Observability of the harness itself

A failure must say what did not happen. Each bounded wait's failure message names the specific
expectation — the record that never appeared, the exit that never came — and includes the child's stderr
captured so far. A bare "timed out" would make the mutant in M18 indistinguishable from a slow machine.

### 10.8 README obligations

The README's "What is and isn't verified here" section currently describes this gap as open and explains
why. **Shipping this work without rewriting that section leaves a false statement in the repo's most-read
file** — the same one-hop propagation failure #10439 records against its own earlier revision. It must
say, after this work:

- Signal-triggered shutdown **is** covered, by an automated test, for both `os.Interrupt` and `SIGTERM`,
  which runs in a Linux container.
- The exit codes for the configuration-error and bind-error branches are covered by the same file.
- The default `go test ./...` does **not** run these — the file is excluded by GOOS at toolchain level —
  and therefore the default run remains a **partial** gate.
- The Linux gate command, in the form of §8.1, with the two shell prefixes.
- What is still not covered: `run()`'s serve-error branch and the second-interrupt behaviour (§2.4), and
  the drain-failure axis (Unit B).

### 10.9 The rejected alternative, in full

**Shape:** a Go test on the Windows host that cross-builds the binary, then shells out to `docker run` to
execute it in a stock image, delivering the signal with `docker kill --signal=` and reading the outcome
from the container's exit code and logs.

| Cost | Measured? | Detail |
|---|---|---|
| **It must decide skip-or-fail when Docker is absent** | Structural | There is no file-name suffix for "machines with a Docker daemon". A skip is a green line that means nothing (#10437 §10.7); a hard failure makes the default `go test ./...` require Docker, breaking G5. **Both answers break a stated constraint** — this is the decisive cost |
| **And that decision needs the trapped command** | M7, #10440 | Any such guard asks whether the engine is up. The obvious command for that exits 0 with empty stdout on a dead engine |
| **The GOARCH trap returns** | #10439, M1 | A host-side build must target the engine's architecture. Here that is `arm64` while the toolchain reports `amd64`. The test would have to either hardcode a machine-local architecture in the repo or derive it from `docker info` — the trapped command again. **The in-container build has no such trap: `go env GOARCH` there is `arm64` (M2)** |
| **Container lifecycle becomes test-owned** | Observed | A named container leaks if the test dies between start and removal — this happened once during measurement and needed a manual `docker rm -f`. The chosen shape starts no container that test code owns |
| **The discriminator** | **M21 — not a cost** | It survives: an unhandled signal to the container's PID 1 exits **130**, a handled one exits 0. Recorded because a rejection propped up by a false cost is one nobody can re-examine |

**Also rejected, more briefly:**

- **A shell or PowerShell script wrapping the docker command.** It would need to work in two shells, and
  it adds a file whose only content is a line the README already carries. The can-it-be-deleted check
  deletes it (#1136 §4).
- **A fixed port with a readiness poll** (the Windows probe's shape). Simpler by one step, and forbidden:
  a fixed port is shared mutable state, shared with every other test *and* with the developer's own
  running instance (#10437 §9.1).
- **Pre-reserving a port by binding 0, closing, and reusing the number.** Reintroduces exactly the
  bind race the port-0 design was chosen to avoid.
- **Treating the `listening` record as sufficient readiness, skipping the `GET /health`.** The listener is
  bound before the record is written, so a connection would be accepted by the backlog before the server
  is serving. More importantly, Code Contracts §13.1.1: the negative direction ("it stopped") is asserted
  only after the scenario is shown to arise ("it served").

---

## 11. Risks & Mitigations

| # | Risk | Mitigation |
|---|---|---|
| R1 | **The gate is never run**, because it is not the default command, and the Linux-only file silently rots | §14 makes running it a handoff criterion, and §10.8 makes the README name it. Residual risk accepted: it is the cost of G5, which is a firm constraint |
| R2 | **A broken Linux-only file is invisible on Windows** (M10) | The gate is one command and compiles it. §10.3 states this plainly rather than hiding it |
| R3 | **The image tag drifts from `go.mod`** | §8.1's derivation rule: they move in the same commit. Falsifier: compare the tag in the README to the `go` directive |
| R4 | **A test hangs instead of failing**, because the mutant it targets keeps running (M18) | §8.3: every wait is bounded and its message names what did not happen |
| R5 | **The stderr assertions break on an unrelated logging change** | §8.3 asserts a record's identity and order, never its rendering. Timestamps and spacing are not contracts |
| R6 | **A leaked child process holds a port** | §8.2: every launched process is asserted-exited or killed and reaped before the test returns. Port 0 means a leak cannot collide with a later test regardless |
| R7 | **The 1.32 GB pull surprises someone on a metered connection** | Named in the README next to the command, with the one-time cost stated |
| R8 | **Someone adds a Docker-availability probe later**, in good faith, to make the gate "friendlier" | §10.2 states the omission as a decision with its reason, so a later reader meets an argument rather than an absence |

---

## 12. Migration / Rollout Strategy

Nothing to migrate. Two notes for the operator:

- **This is one PR: Unit A.** Unit B — the drain-failure axis — is a separate PR against
  `internal/server`, using the `httpServer` seam that merged with PR #3. Per §2.3 the two are ordered
  only by convenience; neither depends on the other.
- When a Dockerfile eventually arrives (out of scope, #10437 §2), its test stage subsumes this gate, and
  Code Contracts §15's BuildKit lesson — a test stage nothing depends on is silently skipped, so the
  final stage must copy **from** the test stage — applies to it directly.

---

## 13. Open Questions

### 13.1 Does the Linux gate belong in a future CI, and does that change the image choice? — for Toni

Not blocking. This design names a venue for a developer's machine. If the gate later runs in CI on
`linux/amd64`, nothing in the design changes — the file is GOOS-constrained, not GOARCH-constrained, and
the in-container build follows the container. Recorded so the decision is made deliberately when CI
exists rather than inherited by accident.

### 13.2 Code Contracts #114's Go annex remains raised, not settled

#10437 §13.1 raised it and it is **still not a ruling**. This design leans on nothing from it beyond the
language-independent sections — §0's principles, §11's logging discipline, §13's parallel-by-default and
no-shared-fixture-state, and §13.1.1's assert-the-positive-before-the-negative. It is cited here as
scoping, exactly as #10437 cited it, and no decision in this document waits on it.

### 13.3 Is `golang:1.27` the tag Toni wants, or should the venue pin a digest? — low stakes

The design chooses the floating minor tag because it tracks `go.mod`'s directive and needs no
maintenance. A digest would make the venue byte-reproducible at the cost of an update obligation with no
named beneficiary today (#1136 §3). One line to change if the answer is different.

---

## 14. Implementation Guidance for the Next Agent

Ordered so each step is verifiable before the next begins. **This is one unit of work and one PR**;
the steps are build order, not PR boundaries.

**The two columns are deliberately different things.** *Step* names what is built; *Done when* names what
is checked. A criterion is never widened to match a title.

| # | Step | Done when |
|---|---|---|
| 1 | **Establish the venue.** Run the §8.1 command against the tree as it stands, before writing anything | Two `ok` lines, no `?`, exit 0, and `go vet` silent. If this does not pass on the unmodified tree, stop — the venue is wrong, not the code |
| 2 | **The graceful-shutdown scenarios** (§6.1), as one test with a subtest per signal, plus the three helpers of §10.5 | Both subtests pass in the container; both are absent from `go list`'s `TestGoFiles` on Windows |
| 3 | **The configuration-error scenario** (§6.2) | Passes, and its wait is bounded — M18 is the reason and the implementer should read it before writing the wait |
| 4 | **The bind-error scenario** (§6.3) | Passes, and no fixed port appears anywhere in the file |
| 5 | **Prove the instrument can fail** (§9.1) | With `signal.NotifyContext` deleted locally, the gate **fails** on scenarios 1 and 2 — observed, then restored. *This is the step that distinguishes a test from decoration; do not skip it and do not report it as done without having watched it go red* |
| 6 | **README** (§10.8) | The "What is and isn't verified here" section is rewritten, the gate command is present in both shell forms, and every command in the README has been run |
| 7 | **Certify** (#10437 §9.6) | Windows: `go test -count=1 -v ./...`, `go vet ./...`, `gofmt -l .` — two `ok` lines, both silent. Container: the §8.1 certifying form — three `ok` lines' worth of tests? **No: still exactly two `ok` lines**, because the new file joins an existing package. Report executed test counts, not "tests pass" |

**Self-checks before handing off**, each one command:

1. Fixed ports in the new file: **none**.
2. `os.Environ()` inherited by any child under test: **none** (M14).
3. Unbounded waits on a child process: **none** (M18).
4. Package-level variables in the new file: **none** (§10.4).
5. References to Docker, an image name, or an availability check inside any `.go` file: **none** (§10.2).
6. Changes to `main.go`, `config.go`, `server.go`, `routes.go`: **none** (§5.3).

**What NOT to add**, because each was considered and declined: a Dockerfile · a `scripts/` directory or
Makefile · a Docker-availability probe or skip · an environment variable or flag to opt these tests in or
out · a `TestMain` with a shared built binary · a fixed port · a slow route or delay knob in production
code · a digest-pinned image · an assertion on log timestamps or exact formatting · a third-party test or
assertion library.

**If any of it seems necessary while implementing, that is a bounce, not a decision** (#114 §0, #1333):
name the principle this design's shape violates and what the alternative is.

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

| Item | Answer |
|---|---|
| No new type mirroring an existing one | ✓ No type is introduced at all |
| No abstraction with one implementation and no concrete second | ✓ No interface, no seam, no indirection. The design's central move is *removing* a decision (§10.2), not adding a layer |
| No element justified by "we might need X later" | ✓ §5.3 lists nine declined elements with reasons. The one temptation — a test affordance in production code — is refused in §2.3 on #1220 §2 grounds |
| No deprecation period, feature flag, compatibility shim, or transition window | ✓ None |
| `block_size × site_count` quoted for every extract/inline decision | ✓ §10.5: 27, 42, 30 — all three above threshold, all three extracted |

**Existing systems first**

| Item | Answer |
|---|---|
| Audited whether an existing surface already covers this | ✓ The existing `cmd/processor` package hosts the file rather than a new one (§5.1); the existing `listening` record supplies the address rather than a new mechanism (M13); the existing `go test` invocation is the runner rather than a new tool |
| If a new layer is proposed, the concrete reason it cannot live on the existing surface is named | ✓ No new layer. The only new file is a test file in an existing package |
| New persisted data | ✓ None |
| Consumer chain recursed | ✓ Every assertion has a named consumer: the mutant it kills (§9.1) |

**Configurability**

| Item | Answer |
|---|---|
| Every new knob has a named operator or environment difference | ✓ **There are no knobs.** No environment variable, no flag, no build tag, no skip guard (§5.3, §10.1) |
| Telemetry-then-tune compounds | ✓ None |
| Magic numbers stay `const` where they need not vary | ✓ The only numbers are per-test deadlines, which are local to the test that uses them |

**Less is better**

| Item | Answer |
|---|---|
| Every element passed can-it-be-deleted / merged / inlined | ✓ The script, the Makefile, the availability probe, the `TestMain` and the opt-in flag were all deleted by this check (§5.3, §10.4) |
| Trade-offs named explicitly where a complex design wins | ✓ §10.9 is the full comparison, including the cost that measurement **removed** from the rejection (M21) |
| Radical-clean chosen over compromise where nothing is consumed | ✓ No availability probe rather than a "careful" one (§10.2) |

**Document discipline**

| Item | Answer |
|---|---|
| Cites Code Contracts #114 and Design Contracts #1136 as load-bearing | ✓ Header and §13.2 |
| Scope inventories explicit, not implicit | ✓ §2.1, §2.2, §5.3, §14's what-not-to-add |
| Out-of-scope items listed rather than absent | ✓ §2.2, and §2.4 lists what stays open **after** this work |
| No multi-paragraph rationale for things that obviously stay | ✓ |
| Predecessor designs marked when superseded | ✓ Nothing is superseded. #10437 is **extended**: its §9.5 U1 and U2 close, U3 and U4 do not, and §10.8 states the README correction that keeps the two in step |
| Every claim measured or marked unmeasured with a falsifier | ✓ §3 is the measurements; §9.2 is the three claims that are not, each with the experiment that would settle it; M21 is a claim measurement **falsified** before it shipped |
