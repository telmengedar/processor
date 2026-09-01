# Architectural Document: Processor M0 — Go Service Skeleton

> Repo path: `docs/architecture/m0-service-skeleton.md` (canonical copy — the DiVoid node carries the
> same document verbatim).
> Task: DiVoid **#10435** · Project: **#10422** · Vision precedent: **#10424**, expanded at `VISION.md`
> (613 lines / 39,097 B against the node's 163 / 25,869 B). **Neither is a mirror of the other:** the node
> condenses for graph retrieval and keeps its refinement rounds 2–6 as separate sections; `VISION.md`
> integrates the same material into a continuous document. No canonical vision record has been
> designated — that is an open call for Toni, and no M0 decision waits on it.
> Standards applied: Design Contracts **#1136**, Code Contracts **#114 §0**, DRY threshold **#1267**,
> anti-seed-complexity **#1184**, principles-trump-design **#1333**, vocabulary rule **#1220 §2 addendum
> 2026-08-17**, run-certification incident **#10351**.
> Revised 2026-09-01 after QA review **#10438**: §9.5's claim that a Windows console-interrupt test was
> infeasible was measured **false** and has been replaced, not annotated; the signal test is descoped to
> **#10439** under the standing rule in §10.7. Sections swept for the same claim: TL;DR, §1 S1, §3, §6.3,
> §9.4, §9.5, §10.6, §11 R7/R10, §12, §13.4, §14, §15.
> Revised again 2026-09-01 after QA re-review **#10445**, on two predicates rather than two line lists.
> (a) *Module path* — the document said bare `processor` while `go.mod` ships
> `github.com/telmengedar/processor`, and said it **prescriptively**; §10.8 is now a decision record and
> §3 A1, §13.2 and §14 step 1 are reconciled. (b) *Corroboration* — a human `Ctrl+C` run was named as
> corroborating the interrupt-shutdown outcome and **never happened**; the outcome is real but its
> instrument was the out-of-tree C16 probe, and §1 S1, §10.6, §11 R7 and §14 step 5 are re-attributed.
> Sites found by sweep beyond those enumerated in #10445: §10.6's README prescription. Also applied:
> W2 (R10 reordered) and W3 (U3 marked withdrawn in place).
> Revised a third time 2026-09-01 after QA re-review **#10446**, on three items. (a) *CF-6* — the previous
> revision stated the module-path reconciliation cost as "three import lines" and prefixed it "at exactly
> the predicted cost", hardening a hedged forecast into an accuracy claim that measurement contradicts;
> §3 A1 and §10.8 now carry the measured figure, **one import line**, marked as measured. (b) *W5* — the
> header called #10424 "mirrored at `VISION.md`"; the two records are related but neither reproduces the
> other, and the line now says what is true. (c) *W8* — §10.7's rule triggered on "spawns a process",
> which its own §14 steps 5 and 6 require; it is rescoped to the harm C17 measured. The same rescoping
> is being applied to #10440, which carries the same wording.

---

## TL;DR

**What.** A Go module in this repo with one binary that starts an HTTP server, serves `GET /health`,
and shuts down cleanly — plus a test suite that proves all three.

**How.** Two packages, standard library only, zero dependencies. `cmd/processor` reads the environment,
binds the listener, wires signals, owns exit codes. `internal/server` owns the route table and the
serve/drain lifecycle. **The caller binds the listener and passes it in**, so tests bind port 0 and prove
start–serve–shutdown in-process, with no subprocess and no signals. That covers the serve/drain lifecycle
on every platform; it does **not** cover the signal wiring in `main`, which M0 leaves unpinned on purpose
and #10439 closes in a container (§9.5, §10.7).

**Configuration boundary.** One boot input with a live consumer: `PROCESSOR_HTTP_ADDR`, default
`127.0.0.1:8080`. The environment is read in exactly one place, and that place is `main`. The DiVoid URL
and API key are **not declared** — they are added by the milestone that first dereferences them.

**Cost.** Nothing beyond the listed files. No third-party dependency.

**Rejected.** A boot-config type carrying the DiVoid URL and key today — declaring vocabulary this phase
will not implement (#1220 §2 addendum).

---

## 1. Problem Statement

Toni, 2026-09-01, verbatim:

> "So lets start building the basic api - golang is installed - a basic server, tests of course - setup a
> git repo (as always i will create the repo on github). Lets see how our usual .net flow works in golang
> with our agents."

Two goals are stacked in that sentence and only one of them is code.

1. **A running, tested Go API service.** It starts, serves, shuts down cleanly, and the test suite is real
   rather than decorative.
2. **A first exercise of the agent chain on Go.** The Sarah → John → Jenny → operator flow has only ever
   run against .NET. This milestone is the instrument that measures whether it transfers.

The second goal constrains the first more than it looks: **a large M0 would measure the chain badly.** If
the deliverable is big, a chain failure and a design failure become indistinguishable. The smallest
deliverable that still exercises every station of the chain is the correct one.

### Success criteria

| # | Criterion | How it is judged |
|---|---|---|
| S1 | The service starts, serves HTTP, and shuts down cleanly | A test that does all three against a real socket (§9.4). **It proves less than the sentence claims:** it cancels the context directly, so it cannot see the signal wiring in `main`. That leg is unpinned in M0, owned by #10439, and enumerated with its falsifier in §9.5 U1. What corroborates the outcome is the **out-of-tree C16 probe**, which measured it once against the built binary — corroboration, not coverage, and not a substitute for the test. **No human `Ctrl+C` run was performed**; an earlier revision said one was |
| S2 | A human can confirm it works | `GET /health` in a browser or curl returns `200` and a legible body |
| S3 | The configuration boundary exists and does not foreclose graph-driven config | Exactly one environment read site in the module; the boundary is a value passed downward, never looked up |
| S4 | The test run is verifiable, not merely claimed | §9.6 certification table; the reported claim names executed test counts and the command that produced them |
| S5 | It builds and runs on this machine | Commands in the README are the ones verified in §3 |

---

## 2. Scope & Non-Scope

### In scope

- Go module, repository layout, repo hygiene files.
- HTTP server bootstrap, serve, and clean shutdown.
- The **configuration boundary** — where it sits, what it holds today, how it is substituted under test.
- One endpoint, sufficient to prove the service end to end.
- The test suite, the command that runs it, and the output that certifies the run happened.

### Out of scope — declined explicitly, not by omission

| Excluded | Where it belongs |
|---|---|
| The memory loop, context assembly, retrieval, workflow engine, memory core | #10424 §Milestones — later milestones with their own tasks |
| Any DiVoid API client | The milestone that first needs to read the graph |
| Graph-driven configuration (briefings, roles, workflows, tool defs as nodes) | #10424 §Thesis inversion 3 — M0 establishes the seam only |
| Web UI | #10424 milestone 6 |
| Deployment, Dockerfile, CI | Out of scope on #10435; see §12 |
| An error-response envelope | The first endpoint that can fail brings it (§8.5) |
| Middleware of any kind — request logging, CORS, auth, request IDs, panic recovery | §10.4 |
| Persistence, database, migrations | No milestone yet |

Three untracked files already exist in the tree — `VISION.md`, `docs/pitch.html`,
`docs/Processor-Vision-Pitch.pdf`. **The implementer does not touch them.** Whether they are committed is
the operator's packaging decision.

---

## 3. Assumptions & Constraints

### Verified on this machine (2026-09-01) — not assumed

Everything in this section was measured, not inferred. The probes ran in `C:\dev\claude\_scratch\` and
have been removed.

**A negative claim is a claim about a mechanism, and it takes a probe like any other.** The one assertion
in the first revision of this document that was not measured — §9.5's claim that a Windows console-
interrupt test was infeasible — is the one that turned out to be false. That is not a coincidence. A
statement that something *cannot* be done is acted on by avoidance: nobody exercises the thing, so nothing
ever contradicts it, and it has no natural expiry (#114 addendum 2026-08-27). It therefore needs a probe
*more* than a positive claim does, not less. C16–C18 are the probes that should have been run before that
sentence was written.

| # | Fact | Consequence for the design |
|---|---|---|
| C1 | `go version` → `go1.27.0 windows/amd64`, at `C:\Program Files\Go\bin\go.exe` | `go.mod` declares `go 1.27` |
| C2 | `go` is **not** on the Bash tool's PATH — `go: command not found` | README gives the full path and the PATH-prepend form |
| C3 | `go build ./...` on success prints **nothing** and exits 0 | The Go analogue of #10351: silence is the success mode, so silence is never evidence |
| C4 | A second identical `go test ./...` prints `ok <pkg> (cached)` — **nothing executed** | `-count=1` is mandatory in every certifying run |
| C5 | A package with no test files prints `? <pkg> [no test files]` and the run stays green | A `?` line against a package holding behaviour is a coverage gap, and must be reported |
| C6 | A `-run` filter matching nothing prints `ok <pkg> [no tests to run]`, exit 0 | Green with zero executions; the annotation is the only tell |
| C7 | `gofmt -l` and `go vet ./...` print **nothing** when clean | Same silence-is-success shape as C3 |
| C8 | `git config core.autocrlf` → `true`, and a CRLF `.go` file **compiles and tests fine but `gofmt -l` flags it** | Without `.gitattributes`, a fresh checkout makes `gofmt -l .` report the entire tree — a full-tree false positive, which is the shape that trains people to ignore a gate. See §10.6 |
| C9 | Loopback sockets work under the tool sandbox — an in-process test server and a real HTTP request against it both pass | The lifecycle test may use a real socket; no sandbox carve-out needed |
| C10 | Module downloads work under the sandbox (a dependency fetch succeeded) | Zero-dependency is a KISS choice, not a sandbox workaround — stated honestly |
| C11 | A method-scoped route pattern yields **405 with `Allow: GET, HEAD`** on a wrong method, and 404 for an unknown path | Both are free from the standard router and are pinned by test rather than assumed |
| C12 | A short JSON body with **no explicit `Content-Type`** is sniffed as `text/plain; charset=utf-8` | The header must be set explicitly; its absence is a real defect and is asserted (§9.3) |
| C13 | After a deliberate shutdown, serving returns the "server closed" sentinel; on an unusable listener it returns a different, non-sentinel error | The translation rule in §8.3 is safe, and the negative test in §9.4 is valid |
| C14 | After the lifecycle returns, a request to the same address is **refused** | "The port is free when the function returns" is assertable, not racy |
| C15 | Binding an address already in use fails loudly with a bind error | The dev-machine port collision (§11 R1) surfaces before serving, not during |
| C16 | A console control event addressed to a child's own process group **does** drive a real interrupt into a Go process on Windows: 5/5 green at ~0.9s each, and it kills the `signal.NotifyContext` mutant, which nothing else in the suite kills (measured by QA, #10438; probe at `_scratch\qa-m0-x7k2\sigprobe\`) | §9.5's former impossibility claim is false and is removed. The instrument is viable — it is declined for the reason in C17, which is a different reason |
| C17 | Generating that event surfaces a console window on the interactive desktop, mid-session, on the machine a person is working at | §10.7's standing rule. The instrument is descoped from M0 to #10439 as a containerised test |
| C18 | **No Docker daemon is reachable from the agent shell.** The `default` context is `ssh://…@bazzite`, and that host does not resolve here; the `desktop-linux` context's named pipe answers but returns `500` on every API version tried, with Docker Desktop's WSL2 backend distro stopped. `docker version` and `docker ps` exit 1 — while `docker info --format '{{.ServerVersion}}'` exits **0** with an empty value | The containerised smoke test cannot be written or run today, so it is not specified here (§9.5, #10439). And any future availability probe must use a command that exits non-zero when the engine is dead (§10.7) |

### Assumptions — flagged

| # | Assumption | Confidence | If wrong |
|---|---|---|---|
| A1 | **No longer an assumption — resolved 2026-09-01.** The GitHub repository was created during this milestone, so the canonical module path *is* knowable and is `github.com/telmengedar/processor` | n/a — a fact, not an assumption | The branch this column anticipated is the one that was taken, and it was cheap: **one `go.mod` line and one import line**. Measured 2026-09-01 (#10446): `cmd/processor/main.go` is the only file in the module that names the module path. The forecast said ~3 imports and was written while a separate `internal/config` package was still in the layout; §5.3 inlined it into `main`, so the outcome came in below the estimate rather than at it. §10.8 records the decision, §13.2 records it as answered |
| A2 | The service is developed and run locally only; there is no container, no cluster, no reverse proxy | High — deployment is out of scope | The listen-address default (§10.2) is the only affected decision, and it is already configurable |
| A3 | Code Contracts #114 binds this work only through its language-independent sections | **Low — this is the open question in §13.1** | John needs the ruling before he can self-audit against §16 |

### Constraints

- **Design-only run.** No production code, no git, no PR from the architect (#7506 v2.2, #10192).
- **Windows.** `SIGTERM` is defined in Go on Windows but is not delivered by the OS; `Ctrl+C` arrives as
  an interrupt. The `SIGTERM` leg of the wiring can therefore only ever be pinned on Linux (§9.5 U2).
- **The dev machine is an interactive desktop, not a runner.** A test may not spawn a console, steal
  focus, or otherwise interrupt the person using it. This is a standing constraint on every milestone, not
  a note about one test — §10.7.
- **No container runtime is reachable from the agent shell today** (C18). Anything whose correct venue is
  a container is therefore descoped rather than sketched.
- **The implementer runs no git.** Files are returned in the working tree; the operator packages.

---

## 4. Architectural Overview

M0 is three responsibilities and one boundary. The boundary is the interesting part; the rest is a small
amount of correct plumbing.

```
                    process environment
                            |
                            |  read EXACTLY ONCE, here and nowhere else
                            v
  +---------------------------------------------------------------+
  |  cmd/processor  (package main)                                 |
  |                                                                |
  |   boot config loader ---> boot configuration (1 member today)  |
  |   binds the listener                                           |
  |   derives a cancellable context from OS signals                |
  |   owns the exit code                                           |
  +--------------------+------------------------------------------+
                       |
        context + bound listener + handler + logger
                       |
                       v
  +---------------------------------------------------------------+
  |  internal/server                                               |
  |                                                                |
  |   route table  ----------->  GET /health -> 200 JSON           |
  |                              anything else -> 404              |
  |                              wrong method  -> 405              |
  |                                                                |
  |   lifecycle    ----------->  serve until ctx done              |
  |                              then drain, bounded, then return  |
  +---------------------------------------------------------------+
```

Under test, the top box is absent entirely. The test constructs the boot configuration directly, binds its
own listener on port 0, and drives the lifecycle with its own context. **That is the substitution
mechanism** — no interface, no mock, no fake, no build tag.

### Why the caller owns the listener

This is the load-bearing shape of the whole design and it earns two things at once.

1. **No startup race.** The alternative — the lifecycle binds internally and publishes its address through
   a channel or a getter — forces every test to synchronise on "has it bound yet". That synchronisation is
   the classic source of a flaky server test. When the caller binds first, the address is known before the
   lifecycle is ever invoked, and there is nothing to wait for.
2. **A better failure boundary.** A bind failure is a startup failure, not a serving failure. Binding in
   `main` means a port collision is reported by `main`, with `main`'s exit code, before anything is
   serving (C15). Binding inside the lifecycle would fold that into the serve error path and blur the two.

It costs one parameter.

---

## 5. Components & Responsibilities

### 5.1 `cmd/processor` — process entry point

| | |
|---|---|
| **Owns** | Environment access · boot configuration construction · logger construction · listener binding · signal-to-context wiring · exit codes |
| **Does not own** | Routing · handler behaviour · HTTP semantics · the serve/drain algorithm |
| **Package-level state** | None |

The environment is read here and nowhere else in the module. That single sentence is the configuration
boundary; everything else in §8.1 is its mechanics.

### 5.2 `internal/server` — the HTTP surface and its lifecycle

| | |
|---|---|
| **Owns** | The route table · handler behaviour · serving until cancellation · bounded draining · closing the listener |
| **Does not own** | Binding · environment access · signal handling · exit codes · what the process does after shutdown |
| **Package-level state** | None |

**Why one package and not two.** Routes and lifecycle are two files and two responsibilities, but they are
one thing: the HTTP server. Splitting them buys an import edge and a package name and prevents nothing.
The can-it-be-merged check (#1136 §4) says merge, and there is no concrete counter-reason — only a
speculative one about later milestones, which is #1184's failure shape exactly.

**Why `internal/`.** It mechanically prevents import from outside the module, which is a true statement
about this code (Processor is an application, not a library) that would otherwise be a convention nobody
enforces. Cost: one directory segment.

### 5.3 What is deliberately NOT a component

| Not built | Why not |
|---|---|
| A configuration package | The boot configuration is `main`'s concern and has one member. A package for it is an indirection (#1136 §4) |
| A router abstraction | The standard router does method-scoped patterns, 404 and 405 already (C11) |
| A service/handler interface pair | One implementation, no second in view. #1136 §4: that is an indirection, not an abstraction |
| A dependency-injection container | Wiring is one function call chain in `main` |
| A panic-recovery middleware | The standard HTTP server already recovers a handler panic, logs it, and closes that one connection; the process survives. A recovery middleware here is a guard for a scenario the runtime already handles (#1136 §6, defensive code for impossible scenarios) |
| A health *checker* (as opposed to a health *endpoint*) | There is nothing to check. When the first dependency exists, the endpoint gains a real check and a real failure mode |

---

## 6. Interactions & Data Flow

### 6.1 Startup

1. `main` builds the boot configuration by calling the loader with the real environment lookup.
   On error: log it, exit non-zero. Nothing else has been constructed yet.
2. `main` constructs the logger — structured records to **stderr**.
3. `main` binds a TCP listener at the configured address. On error (C15, the port-in-use case): log the
   address and the error, exit non-zero.
4. `main` derives a context that is cancelled on interrupt and on `SIGTERM`.
5. `main` calls the lifecycle with that context, the listener, the route table, and the logger.
6. The lifecycle logs **the listener's actual bound address** — not the configured one. When the
   configured address ends in port `0`, or when a container remaps, the actual address is the only useful
   one, and it is what the test reads.

### 6.2 Serving

A request is accepted, matched against the route table, and answered. No middleware, no per-request log
line, no state touched.

### 6.3 Shutdown

1. The context is cancelled — by `Ctrl+C`, by `SIGTERM` where the OS delivers it, or by the test.
2. The lifecycle logs that shutdown has begun.
3. It stops accepting, closes the listener, and drains in-flight requests under a **grace deadline**
   (§8.3).
4. It logs completion and returns: `nil` on a clean drain, an error if the drain exceeded the deadline.
5. `main` exits 0 on `nil`, non-zero otherwise.

**A second interrupt is not handled, and what happens instead is not specified here.** No extra handling is
written: the design registers the signal once and does nothing further. The registration is released only
when the entry point returns, so it is still live while the drain is running, and the resulting behaviour
on a second interrupt is a runtime property nobody on this project has observed. An earlier revision
asserted that the second `Ctrl+C` terminates the process immediately. **That assertion is withdrawn — and
deliberately not replaced by its opposite, which is equally unmeasured.** It is listed as unverified with
its falsifier in §9.5 U3.

---

## 7. Data Model (Conceptual)

M0 persists nothing and models no domain entity. The only data structures are the boot configuration
(§8.1) and the health response body (§8.2). Both are described where they are contracted; there is no
third place and no schema.

---

## 8. Contracts & Interfaces (Abstract)

### 8.1 Boot configuration — the seam

**This is the section the brief's item 3 asks for.** The vision (#10424) states the harness boots knowing
only a graph URL and an API key, reading everything else from the graph. That is two claims: what boot
configuration *is*, and what it *is not*. M0 must make the first true and must not make the second
unreachable.

**The boundary is a rule, not a type:** *configuration enters the process at exactly one site, in `main`,
and travels downward as values.* Everything else follows from it.

| Aspect | Contract |
|---|---|
| **Input** | A lookup function from variable name to (value, present). `main` supplies the real environment lookup; a test supplies its own |
| **Output** | A boot configuration value, or an error naming the offending variable |
| **Members today** | Exactly one: the HTTP listen address |
| **Variable** | `PROCESSOR_HTTP_ADDR` |
| **Default** | `127.0.0.1:8080` when the variable is absent |
| **Present and non-empty** | Used verbatim. No normalisation, no rewriting |
| **Present and empty** | An error. An explicitly-emptied variable is an operator mistake, not a request for the default — silently defaulting would hide it |
| **Validation** | None beyond the above. The loader does **not** check that the address is syntactically valid or bindable — binding produces a better, more specific error at the point of failure (C15). Duplicating that check would give two error messages for one fault, and the worse one first |
| **Side effects** | None. No I/O, no logging, no globals. It is a pure function of its lookup |

**Substitution under test.** The loader takes its lookup as a parameter, so the test supplies its own and
touches no real environment variable. Below `main`, nothing takes configuration at all — the lifecycle
takes a listener, not an address.

**Why a lookup parameter rather than reading the environment inside the loader and using the standard
environment-setting test helper.** That helper is documented as incompatible with parallel tests: a test
that uses it cannot mark itself parallel. Code Contracts #114's testing discipline makes
parallel-by-default the house rule, and the ruling of 2026-08-08 (tests never share mutable state) names
the reason — the damage from shared state shows up as an intermittent failure months later, not a red
test. **The process environment is shared mutable global state.** A one-parameter seam that keeps the
whole suite parallel and confines the environment to a single site is a better trade than a helper that
removes one parameter and forfeits both. Cost: one parameter on one function. This is close enough to be
worth stating; the rejected alternative is named here rather than hidden.

#### What is deliberately NOT declared — and the list that replaces it

The DiVoid base URL and the DiVoid API key are **not members of the boot configuration in M0.** Nothing in
M0 dereferences them, and #1220's §2 addendum of 2026-08-17 is explicit about what happens when a design
declares vocabulary its phase will not implement: the members get built as declarations, several are never
wired, and an unimplemented *member* is worse than an unimplemented *feature* because it is present,
documented, callable and silently inert.

**The named future list, in prose, as the addendum requires — these are not members, and no implementer
declares them in this milestone:**

- the DiVoid base URL,
- the DiVoid API key (a secret — see §10.2),
- and, later still, whatever the loop needs that genuinely cannot come from the graph.

**Each is added by the milestone that first dereferences it, together with its consumer.** The seam, not
the member, is this milestone's deliverable.

#### Falsifier for the boundary claim

The claim *"the environment is read in exactly one place"* is not rhetoric; it is checkable, and it fails
the moment a handler or a lower package reaches for an environment variable directly. **A search of the
module for environment reads must return exactly one site, in `main`.** More than one, and the boundary is
gone regardless of what this document says.

### 8.2 Route table

| Aspect | Contract |
|---|---|
| **Construction** | A function returning the HTTP handler. It takes no parameters today because there is nothing to inject. **The rule is that the first dependency arrives as a parameter, never as a package-level variable** |
| **`GET /health`** | `200`, `Content-Type: application/json`, body `{"status":"ok"}` exactly |
| **`HEAD /health`** | Served automatically by the method-scoped pattern (C11). Not a separate route; do not add one and do not remove it |
| **Any other method on `/health`** | `405`, with `Allow: GET, HEAD` supplied by the router (C11) |
| **Any unmatched path** | `404` from the router's default (C11) |
| **Invariants** | Handlers hold no state, read no environment, and perform no I/O |

**`Content-Type` must be set explicitly.** C12 measured that a body this short is otherwise sniffed as
`text/plain; charset=utf-8`. The omission is invisible in a browser and wrong on the wire, which is why it
is asserted (§9.3) rather than trusted.

**Why `{"status":"ok"}` and nothing more.** A build stamp, a version, an uptime or a timestamp all have
the same problem: no consumer. The body needs to satisfy S2 — a human sees something legible and
unambiguous — and be exactly comparable by a test. It does both. When a real dependency exists, the
endpoint reports on it and the body grows a member with a reader.

### 8.3 Server lifecycle

| Aspect | Contract |
|---|---|
| **Inputs** | A context · an already-bound listener · a handler · a logger |
| **Behaviour** | Log the listener's actual bound address · serve until the context is done · then stop accepting, close the listener, and drain in-flight requests under the grace deadline · log start and completion of shutdown |
| **Returns `nil`** | The context was cancelled and the drain completed within the deadline |
| **Returns an error** | Serving failed for any reason other than a deliberate shutdown, **or** the drain exceeded the grace deadline |
| **Invariant 1** | The "server closed" sentinel is never returned as an error. It is the *expected* outcome of a deliberate shutdown (C13), and returning it makes a clean `Ctrl+C` exit non-zero. This is the single most common defect in this exact piece of Go |
| **Invariant 2** | The function returns only after the listener is closed and the drain has finished or timed out. **A caller that receives `nil` knows the port is free** (C14) |
| **Invariant 3** | It never reads the environment, never reads configuration, and never decides an exit code |

**Constants, not configuration** (#1136 §3 — a magic number made configurable is still a magic number,
with a layer on top):

| Constant | Value | Why not configurable |
|---|---|---|
| Shutdown grace deadline | 5 seconds | No operator will tune it and it does not differ by environment. Long enough for an in-flight request, short enough that `Ctrl+C` feels immediate |
| Read-header timeout | 5 seconds | Same. A connection that never finishes its headers otherwise has no deadline at all, and would hold the drain open for the full grace period on every shutdown — which would make invariant 2 slow rather than false |

**Deliberately not set:** overall read and write timeouts. They would cap legitimate long requests, and M0
has none to cap. The first endpoint with a real body brings its own deadline requirement.

### 8.4 Process entry point

Owns exit codes: `0` when the lifecycle returns `nil`; non-zero when configuration loading, binding, or
the lifecycle fails. Every non-zero exit is preceded by a log record naming what failed.

### 8.5 The error envelope, deliberately absent

M0 has no endpoint that can fail for an application reason. Designing a machine-readable error body now
would be specifying a vocabulary this phase does not implement — the same failure as §8.1's DiVoid
members. **The first endpoint that can fail brings the error shape with it**, and it will be designed
against a real failure rather than an imagined one.

---

## 9. Testing — and how a reader knows a run happened

### 9.1 Principles this suite obeys

- **Every test is parallel.** Nothing in the module has package-level mutable state, no test touches the
  real environment, and no test binds a fixed port.
- **A fixed port in a test is shared mutable global state** — shared with every other test *and* with the
  developer's own running instance. It is the same class of defect as the shared fixture field ruled out
  on 2026-08-08, with the same symptom: an intermittent failure, months later, triaged as flaky.
  **Tests bind port 0.**
- **Each level asserts only what is its own.** The socket-level test does not re-assert the response body;
  the handler tests own that. Duplicated assertions across levels are how a suite gets expensive to change
  without getting better at catching anything.

### 9.2 `cmd/processor` — boot configuration

| Test pins | Expected |
|---|---|
| Variable absent | The documented default address |
| Variable present, non-empty | That value, verbatim |
| Variable present, empty | An error naming the variable |

Each supplies its own lookup. None touches a real environment variable, so all three are parallel.

### 9.3 `internal/server` — route table, in-process

Driven through the constructed handler with an in-memory request and response recorder. No socket, no
port, microseconds.

| Test pins | Expected |
|---|---|
| `GET /health` | `200` · `Content-Type: application/json` · body exactly `{"status":"ok"}` |
| `GET` on an unregistered path | `404` |
| `POST /health` | `405` |

The `Content-Type` assertion is not decoration — it is the only thing standing between the contract and
C12's silent sniffing.

### 9.4 `internal/server` — lifecycle, over a real socket

This is S1's proof, and it is one test that makes the whole claim at once:

1. Bind a listener on `127.0.0.1:0`; read its actual address.
2. Run the lifecycle in a goroutine with a cancellable context.
3. Issue a real HTTP request to that address and assert it succeeds — **the service is serving.**
4. Cancel the context.
5. Assert the lifecycle returns `nil` within a bounded wait — **it shut down cleanly.**
6. Assert a request to the same address now fails — **the listener really is closed** (C14).

**Paired negative test — the mutant this suite must kill.** An implementation that returns `nil` for
*every* serve error, rather than only for the deliberate-shutdown sentinel, passes step 5 and looks
correct. So a second test runs the lifecycle against a listener that is already closed and asserts the
return is **non-`nil`** (C13 confirms the error is distinguishable). Without it, invariant 1 of §8.3 is
unpinned, and the design's own load-bearing property is the one thing not tested — which is precisely the
finding recorded on #10351.

Per Code Contracts §13.1.1: the negative direction is asserted only after the scenario is shown to arise.
Step 3 establishes that the server genuinely served before step 6 asserts that it no longer does.

**This test stays, and its reach is bounded.** It cancels the context directly, so `signal.NotifyContext`,
`run()`'s exit codes and the process boundary are all outside what it can observe. It is the strongest
instrument M0 has for the drain and the shutdown sentinel, and it is not an instrument for the signal path
at all. Reading it as proof of S1 in full is the specific mistake §9.5 exists to prevent.

### 9.5 What is NOT tested, stated rather than implied

**Signal delivery is not tested in M0. The reason is a cost, not an impossibility.**

The first revision of this section asserted that driving a real console interrupt into a child process on
Windows "requires console-control-event machinery that is more fragile than the code it would be testing",
and that the only reliable check was a manual interactive run. **That was measured and it is false**
(C16, review #10438). The instrument is **one 48-line test file** — a 36-line function body, measured in
the preserved probe — which starts the binary in a new process group, polls `/health` until it answers,
then generates a console control event addressed to the child's group. It ran 5/5 green at about 0.9s
each with no observed flake; against a build with
`signal.NotifyContext` deleted it fails with `exit status 0xc000013a` — `STATUS_CONTROL_C_EXIT`, meaning
the OS terminated the process instead of the process exiting through its own shutdown path. It kills the
one mutant nothing else in the suite kills. The break event is not a compromise for the interrupt event:
a console control event cannot address a single process group with `CTRL_C`, and the Go runtime delivers
both to the process as the same `os.Interrupt`, so the code path under test is identical. The probe is
preserved at `C:\dev\claude\_scratch\qa-m0-x7k2\sigprobe\`.

**It is not shipped, and fragility is not why.** It spawns a process in a new console group on a machine
that is somebody's interactive desktop, and generating the event surfaces a window while that person is
working (C17). Per the standing rule in §10.7, live execution belongs in a container, not on this desktop.
The right instrument is a containerised Linux smoke test — which is what the first revision also said, and
that half of the judgement survives. Only the impossibility claim was wrong, and it was the half stated in
the indicative.

**And that instrument cannot be specified today.** No Docker daemon is reachable from this machine's agent
shell (C18). Writing the shape of a test that nobody can execute would put back exactly the class of claim
this section is being rewritten to remove. It is therefore **descoped from M0 and owned by #10439**, which
designs and implements it once a daemon is reachable and its environment can be measured.

**Consequence, stated plainly: in M0 the signal path is pinned by nothing.** Everything the signal
*causes* is still tested through context cancellation (§9.4) and that leg is real; what no test in this
module can see is `signal.NotifyContext` itself, `run()`'s exit codes, and the process boundary.

#### What is unverified in M0, and what would falsify each

| # | Claim currently unverified | What would falsify it |
|---|---|---|
| U1 | An interrupt makes the **process** run the graceful path and exit 0 | Run the built binary under a supervising parent, deliver an interrupt, and assert exit code exactly 0 *and* the ordered stderr records `listening` → `shutdown started` → `shutdown complete`. A non-zero code, or a zero code without the two shutdown records, falsifies it. The out-of-tree probe *asserted* the exit code and *observed* the records; #10439 turns both into assertions in a test that actually runs |
| U2 | `SIGTERM` reaches the same wiring | **Unfalsifiable on Windows** — the OS never delivers it (§3), so no experiment here can bear on it. On Linux: deliver `SIGTERM` to the running binary and assert the same exit-0-plus-records outcome. A process killed by the signal (no shutdown records) falsifies it. This is the leg the container gains that Windows can never have |
| U3 | **(Withdrawn, not held — a different status from U1/U2, which this design believes but has not pinned.)** A second interrupt during the drain terminates the process immediately | Deliver two interrupts while a deliberately slow request is draining. Immediate termination confirms it; completing the drain and exiting 0 falsifies it. §6.3 previously asserted this outcome; the assertion is withdrawn rather than reversed, because the opposite is equally unmeasured |
| U4 | The drain-failure axis — that the grace deadline and the shutdown-error propagation are load-bearing | Mutants 1–4 of #10438 survive the current suite: the shutdown error turned into `nil`, the read-header timeout deleted, and the grace deadline moved to 1ns and to 30s all leave it green. A test with a deliberately blocking handler would kill the first and third. **Out of M0's scope** (W3 in #10438, M1 follow-up); recorded here so it is not mistaken for covered |

A gap with a falsifier is an open question. A gap with a story about why it cannot be closed is what §9.5
used to be.

### 9.6 Certifying that a run happened

The .NET incident (#10351) was a build and test command returning **exit 0 with no output at all**, read
by an agent as clean. Go has the same hazard in four distinct shapes, all measured on this machine in §3.

| Output | What it actually means | Verdict |
|---|---|---|
| `go build ./...` prints nothing, exit 0 | It compiled. **Nothing was tested** (C3) | Silence is the success mode — never evidence about tests |
| `ok <pkg> 0.28s` | That package's tests ran and passed | **Trust** |
| `ok <pkg> (cached)` | A previous identical run is being replayed. **Nothing executed** (C4) | **Distrust.** Re-run with `-count=1` |
| `? <pkg> [no test files]` | The package has no tests. Green regardless of correctness (C5) | **Distrust as coverage.** Report the gap |
| `ok <pkg> [no tests to run]` | A filter matched nothing; zero tests executed (C6) | **Distrust** |
| `--- FAIL: TestX` + `FAIL <pkg>` + exit 1 | A test failed | **Trust** |
| `gofmt -l` / `go vet ./...` print nothing | Clean (C7) | Same silence-is-success shape as the build |

**The commands.**

| Purpose | Command |
|---|---|
| The gate | `go test -count=1 ./...` |
| The certifying run | `go test -count=1 -v ./...` |
| Formatting | `gofmt -l .` — must print nothing |
| Vet | `go vet ./...` — must print nothing |

**`-count=1` is not optional in any run whose output is quoted as evidence.** Its presence in the quoted
command is what lets a reader see that caching was disabled; without it, `(cached)` is the only tell and
it is one word wide.

**The shape of a defensible claim.** Not "tests pass", but: *N tests executed across M packages, command
quoted with `-count=1`, with the verbatim `ok` lines and their durations.* Under `-v`, the count of
`--- PASS:` lines is the number of tests that actually executed.

**And one acceptance criterion that is checkable in a glance:** with the layout in §10.1,
`go test -count=1 ./...` prints **exactly two `ok` lines and no `?` line**, because both packages carry
tests. A `?` line appearing means a package lost its tests — visible without reading anything.

---

## 10. Cross-Cutting Concerns

### 10.1 Layout

```
processor/
  go.mod
  .gitattributes
  .gitignore
  README.md
  VISION.md                     (exists, untracked, not touched)
  docs/
    architecture/
      m0-service-skeleton.md    (this document)
    pitch.html                  (exists, untracked, not touched)
    Processor-Vision-Pitch.pdf  (exists, untracked, not touched)
  cmd/
    processor/
      main.go                   entry point, wiring, exit codes
      config.go                 boot configuration + loader
      config_test.go
  internal/
    server/
      routes.go                 route table + handlers
      routes_test.go
      server.go                 serve / drain lifecycle
      server_test.go
```

**No `pkg/` directory.** It is a convention with no compiler meaning and nothing here is for external
consumption.

**One type per file is not applied.** Go's unit of organisation is the package, not the file; the compiler
treats every file in a package as one scope, and splitting cohesive declarations across files is a foreign
convention with a real navigation cost. Code Contracts §1 is a .NET rule — see the open question in §13.1.

### 10.2 Security

- **The API key is a future member and this design does not introduce it** (§8.1). When it arrives it is
  an environment variable, never a committed file, never logged, and never returned by any endpoint — the
  same discipline as Code Contracts §14.
- `.gitignore` covers built binaries and any local environment file, so a secret cannot be committed by
  reflex once one exists.
- The default listen address is `127.0.0.1:8080` rather than all interfaces: a dev-only service is not
  reachable off-box by accident, and Windows does not raise a firewall prompt on startup. A container
  deployment overrides it — which is exactly why the address is the one value that is configurable.
- No authentication. There is no protected resource; `/health` is public by intent.

### 10.3 Observability

Structured records to **stderr**, at three points and no more:

| Event | Level | Carries |
|---|---|---|
| Listening | Info | The listener's **actual** bound address |
| Shutdown started | Info | — |
| Shutdown complete, or drain deadline exceeded | Info / Error | The failure, when there is one |

**No per-request logging.** Code Contracts §11's rule ("log writes, never reads") would silence the only
endpoint M0 has, and a request log for a health check is pure noise. This is the same conclusion by a
shorter route: there is nothing worth logging per request yet.

**Why the actual bound address rather than the configured one.** With port `0`, or a container remap, the
configured value is not where anyone can reach the service. The log line's job is to be actionable.

**Why stderr.** It keeps the diagnostic stream separate from anything the process might later write to
stdout, and it is where an operator looks.

### 10.4 Error handling

Errors are values, returned upward and decided at the boundary that can act:

- **The loader** returns an error naming the offending variable. `main` logs and exits non-zero.
- **Binding** fails in `main`, which logs the address and the error and exits non-zero (C15 — the
  port-collision case, which will happen on a dev machine).
- **The lifecycle** returns `nil` for the deliberate-shutdown sentinel and the real error otherwise
  (§8.3 invariant 1).
- **Handlers** cannot fail in M0. When one can, §8.5's error shape arrives with it.

No middleware, and specifically no panic-recovery middleware — the standard HTTP server already contains
a handler panic to its own connection and keeps the process alive (§5.3).

### 10.5 Dependencies

**Standard library only. Zero third-party modules.**

| Reflex | Why not, here |
|---|---|
| A router library | The standard router does method patterns, 404 and 405 (C11). For one route it buys nothing |
| An assertion library | Explicit comparisons are clear at this size, and the usual library's soft-versus-hard assertion split is itself a source of tests that keep running after a failure |
| A configuration library | One variable |
| A logging library | Structured logging is in the standard library |

Two consequences worth naming: the build needs no network (measured — C10 shows fetching *works*, so this
is a KISS choice and not a sandbox workaround), and a service with one endpoint does not commit the
project to a dependency it carries forever. **This is not a ban.** The first milestone with a concrete
need names its dependency then, with the need in hand.

### 10.6 Repo hygiene — and one measured hazard

- **`.gitattributes` is not optional here.** C8 measured it: `core.autocrlf` is `true` on this machine,
  and a CRLF Go file compiles and tests perfectly while `gofmt -l` flags it. Without a normalisation rule,
  a fresh checkout makes `gofmt -l .` report **every file in the tree** — a full-tree false positive,
  which is the failure shape that teaches people to ignore the gate. It must pin `*.go` to LF, and `*.md`
  too, because the same conversion is what makes a design document's byte count differ between the
  committed blob and a checkout (#1220 §3 addendum 2026-08-31).
- **`.gitignore`** covers the built binary (including the `.exe` form) and any local environment file.
- **`README.md`** carries what §3 measured: that Go is not on the Bash PATH (C2) and both working
  invocations, the build/run/test commands, and the endpoint to open in a browser. Its
  "what is and isn't verified" section must state the signal gap **as it now stands**, in three parts:
  the signal path is covered by no automated test in M0; a console-interrupt test was *measured to work*
  on Windows (C16) and is declined because it opens a window on an interactive desktop (C17, §10.7), not
  because it is infeasible; and the containerised replacement is #10439. The manual run that stays is the
  **build / run / `curl` check** — described as a human corroboration on the dev platform, **not** as "the
  only reliable check", which is the false claim this design propagated into the README and is the
  correction owed there. **No human `Ctrl+C` run may be claimed anywhere**, in this document or the README:
  nobody performed one. The interrupt-shutdown outcome was measured once by the out-of-tree C16 probe and
  by nothing else, and attributing it to a keypress is a claim about an instrument that was never operated
  — the same defect as an unrun test reported as green, one artifact over.
- **`go.mod`** declares `go 1.27` (C1).

### 10.7 Live execution runs in a container, not on this desktop — standing rule

**The rule.** A run that **reaches the interactive desktop** — opens or attaches its own console or GUI
window, generates console control events, delivers a signal, or otherwise steals focus from the person at
the machine — or that **binds beyond loopback** goes inside a Linux container. It does not run on this
machine's desktop, and it is never part of the default test command here.

**What the rule does not cover, stated because the boundary has been misread.** An ordinary foreground
command in the agent's or the implementer's own shell is not a live execution in this sense, however many
processes it spawns. `go build`, `go test`, `go vet`, `gofmt`, running the README's commands (§14 step 6),
and starting the service in the foreground to answer a loopback `GET /health` (§14 step 5) are all
permitted, and §14 requires them. **Spawning a process is not the harm.** C17 measured a *window*
appearing on somebody's desktop, and the rule is scoped to that. On Windows the two coincide for exactly
one thing: an interrupt can only be delivered by generating a console control event, and that needs a
console — which is why the signal leg specifically is descoped to #10439 while start-and-serve
specifically is not. This scoping matches §3's constraint bullet ("A test may not spawn a console, steal
focus, or otherwise interrupt the person using it") and §11 R10, both of which were already stated by
harm; §10.7's rule sentence was the one place that was not.

Two reasons, in order of weight.

1. **The machine is a person's working environment, not a runner.** Measured 2026-09-01 (C17): a
   console-control-event test surfaced a window on the desktop while Toni was working at it. A test that
   steals focus is a defect independently of whether its assertions are correct, and no amount of green
   makes it acceptable.
2. **The deployment target is Linux** — the project's stated direction (#10424 §Stack), not a measurement;
   A2 records that no container exists today. The container is therefore not a Windows workaround; it is the
   platform of record. A signal test there also pins `SIGTERM`, which Windows cannot deliver at all
   (§3) — so the container is strictly the *better* instrument, not the fallback one.

**Consequences.**

- The Windows `go test -count=1 ./...` is deliberately a **partial** gate. It must never be quoted as
  though it covered process-level behaviour; §9.5 is the list of what it leaves open.
- A platform-specific test file is excluded **by the toolchain**, never skipped at runtime. A runtime skip
  is a green line that means nothing, which is #10351's shape wearing different clothes.
- When a container gate does exist, its unavailability must fail loudly. Measured 2026-09-01 (C18): with
  the engine dead, `docker version` and `docker ps` exit non-zero — but `docker info` with a `--format`
  argument exits **0**, printing an empty value on stdout and the error on stderr. An availability probe
  built on that command is green when the daemon is dead. Probe with a command that exits non-zero.

**This is not the deployment story.** Using a stock image as an execution venue introduces no Dockerfile,
no image build and no registry; §2's exclusion of deployment stands unchanged.

**Later milestones inherit this.** The tool-runner subprocess boundary and the Python skills layer behind a
process boundary (#10424 §Stack) are the next two places the rule binds. Neither is in M0, and the rule is
recorded here so that the next design does not have to rediscover it from a second interrupted desktop.

### 10.8 Module path

The module is **`github.com/telmengedar/processor`**, and `go.mod` declares exactly that.

**The principle, which is the part that generalises:** a module path names a repository, so it may state
only a repository that exists. It is never guessed and never aspirational — `go.mod` is the one file
authoritative about this module's identity, and a path naming a repository nobody has created is a false
statement in it.

**Why an earlier revision of this section said bare `processor`, and why that is not a reversal.** When
this design was written no remote existed (#10435), and the same principle yielded the bare path: nothing
in M0 needs a resolvable path — there is no external importer and `internal/` packages are unimportable by
construction — so the honest path was the one that claimed nothing. Toni then created the repository, the
premise changed, and the unchanged principle yielded the host-prefixed path. The rule held; its input
moved. Cost of the move, **measured**: one `go.mod` line and one import line — only `cmd/processor`
imports the module path; `internal/server` imports nothing internal and no test file does (#10446). The
forecast was ~3, written when the layout still had a separate `internal/config` package that §5.3 later
inlined. The figure is recorded as measured rather than as predicted, because a forecast restated in the
indicative is the defect this section exists to correct.

**This section is a decision record, not an argument.** That distinction is the actual defect being
corrected. The earlier revision was *prescriptive* — it argued that a host-prefixed path **would be** a
false statement — so it handed a later reader a documented reason to "fix" a shipped `go.mod` that is
correct, and §14's `go build ./...` gate passes under either path and would not have caught it. A design
document describing a decision it no longer owns must read as a record, or it silently becomes an
instruction to undo the code.

---

## 11. Risks & Mitigations

| # | Risk / failure mode | Mitigation |
|---|---|---|
| R1 | **Configured port already in use.** Certain on a dev machine — a second run, or a stale process | Binding happens in `main` before serving (C15). The error names the address and the process exits non-zero. Tests never use a fixed port, so they cannot collide with a running instance |
| R2 | **A clean shutdown is reported as a failure.** The shutdown sentinel returned as an error makes `Ctrl+C` exit non-zero | §8.3 invariant 1, pinned by §9.4 |
| R3 | **Every serve error swallowed as `nil`** — the over-correction of R2, and it looks correct | The paired negative test in §9.4. This is the mutant the suite exists to kill |
| R4 | **A hung request silently truncated at shutdown** | The drain is bounded and exceeding the deadline is an *error*, not a shrug. The read-header timeout prevents the commonest cause |
| R5 | **A green test run that executed nothing** — the #10351 shape | §9.6's table and the mandatory `-count=1`. The two-`ok`-lines-no-`?` criterion makes the common case checkable at a glance |
| R6 | **`gofmt` reports the entire tree** after a fresh checkout | §10.6's `.gitattributes` |
| R7 | **The signal path is exercised by nothing automated** (§9.5) | Named as an open gap with a falsifier (§9.5 U1–U2) and owned by **#10439**. Meanwhile the outcome — graceful path, exit 0, the three ordered records — was measured **once, out of tree**, by the C16 probe. That is corroboration and **not** coverage: nothing in the shipping module pins it, which is why U1 stays in §9.5's unverified table. Explicitly **not** mitigated by a claim that it cannot be tested: that claim was measured false (C16), and a false impossibility is worse than an admitted gap because nothing ever contradicts it. And explicitly **not** mitigated by a human `Ctrl+C` run — an earlier revision named one here, in this mitigation column, and it never happened |
| R8 | **The configuration seam erodes** — a later handler reads the environment directly and the boundary quietly stops existing | §8.1's falsifier: the module must contain exactly one environment-read site. It is a one-command check and belongs in every future review of this repo |
| R9 | **#114's .NET rules transplanted literally into Go**, producing worse Go than no contract at all | §13.1 is the open question, raised rather than settled. The design states which rules it applied and why the rest are out of reach |
| R10 | **A test reaches the developer's desktop** — opens a console, steals focus, or binds a port a person is using | §10.7's standing rule: live execution runs in a container, and platform-specific files are excluded by the toolchain rather than skipped at runtime. This risk is realised, not hypothetical — C17 is the incident that produced the rule |

---

## 12. Migration / Rollout Strategy

There is nothing to migrate — this is the first code in the repository. Two notes for the operator:

- Deployment, Dockerfile and CI are out of scope on #10435. Worth recording for whoever picks them up:
  Code Contracts §15's BuildKit lesson — that a test stage nothing depends on is silently skipped, so the
  final stage must copy **from the test stage** — is language-independent and will apply verbatim to a Go
  Dockerfile.
- The three pre-existing untracked files are not the implementer's concern (§2).
- **The signal-path gap ships open, and #10439 closes it.** Its instrument is a Linux container (§10.7),
  which makes it blocked on a reachable Docker daemon (C18) — a machine-state dependency, not a design
  one. §13.4 is the question that unblocks it. Nothing in M0 waits on either.
- When a Dockerfile does arrive, its test stage subsumes that container run, and the BuildKit lesson above
  applies to it directly.

---

## 13. Open Questions

### 13.1 Does Code Contracts #114 need a Go annex? — for the operator to take to Toni

**Raised, not settled**, per the brief. I read #114 in full. **My assessment is that it needs an annex
rather than a translation, because roughly half of its sections are .NET- and Pooshit-mechanism-specific,
and three of them would produce actively worse Go if applied literally.** The evidence:

**Transfers unchanged — these are rules about discipline, not about C#:**

| Section | Note |
|---|---|
| §0 Principles + the bounce rule | Language-independent. Applied throughout this document |
| §4 Comments | Transfers. Go's doc-comment convention (a comment starting with the identifier's name) is the `<summary>` analogue |
| §11 Logging | Levels map 1:1 onto the standard structured logger. "Log writes, never reads" transfers |
| §13 discipline — parallel by default, no shared mutable fixture state | Transfers, and matters **more** in Go: parallelism is opt-in per test, so a shared-state defect hides until someone opts in |
| §14 Configuration & secrets | Transfers |
| §15's BuildKit `--from=test` lesson | Language-independent |
| The 2026-08-08 `[Description]` **meta**-pattern (narration relocates to whichever channel is still allowed) | Transfers, and the ruling explicitly predicted a fourth channel. In Go the channels to watch are the package doc comment, over-long doc comments on exported identifiers, and subtest names used as narration |

**No clean Go equivalent — needs replacing, not translating:**

| Section | Why |
|---|---|
| §1 one type per file | Go's unit of organisation is the package; the compiler treats all files in a package as one scope. A package-cohesion rule would be the honest replacement |
| §2 Naming | **Directly conflicting.** Go encodes exportedness in capitalisation, so "PascalCase for methods and properties" would make everything public. The `I` prefix on interfaces is a marker of a non-Go codebase; the convention is `-er` names |
| §3 never `var` | **Directly conflicting.** Short variable declaration is the idiomatic default. Separately, §3's brace and blank-line rules are moot: `gofmt` settles formatting mechanically, which is a better answer than the rule |
| §5 Types & DTOs | Ocelot/attribute-specific throughout. §5.4 (no parallel mirror enums) survives as a *principle*, but Go has no enums — the analogue is typed constants and the failure looks different |
| §6.2 "never create services without a corresponding interface" | **The sharpest conflict, and the most likely to be transplanted by reflex.** Accepted Go practice is the inverse: accept interfaces, return concrete types, and let the *consumer* declare the interface it needs. An interface-per-implementation pair is exactly the one-implementation indirection #1136 §4 rules out |
| §7 Controllers | ASP.NET-specific. The intents transfer (document the statuses, no error handling in the handler, return the domain object); the mechanisms do not, because **Go has no exceptions** and therefore no exception-to-status middleware |
| §8 Dependency Injection | No container in idiomatic Go. Singleton/Transient has no meaning. The correct Go rule — wire explicitly in `main`, no globals — is worth stating positively rather than translating |
| §9 HTTP / §10 JSON | Pooshit library rules. §10 hides a **genuine trap**: Pooshit.Json defaults to camelCase, whereas Go's encoder defaults to the *Go field name*. The camelCase wire contract survives, but in Go it must be enforced by an explicit tag on every field instead of by a default — same contract, opposite mechanism |
| §12 Error Handling | **The largest gap.** The whole section rests on exceptions bubbling to middleware. Go has no exceptions; errors are values handled at each call site. Every rule needs replacing — wrapping, sentinel errors and identity checks, and how a handler maps an error value to a status |
| §13.4 + the 2026-08-08 `InternalsVisibleTo` ruling | **The most interesting case.** The mechanism *evaporates*: a Go test file in the same package sees unexported identifiers natively, so there is nothing to widen and no `Program`-visibility tangle. But the ruling's underlying hazard — a test that reaches inward and verifies a function instead of the program — **intensifies**, precisely because reaching inward is now frictionless. The ruling loses its enforcement surface exactly where its concern gets sharper |

**Why this needs answering soon rather than eventually.** John will be briefed with #114 and will hit this
in his first file. Absent a ruling he will improvise, and the most likely improvisation is the literal one
— an `IServerService` interface, explicit types everywhere, one type per file — which produces Go that a
Go reader will not recognise. **This design applied only the language-independent sections above**, and
says so in §10.1 and §10.3 where it visibly departs from §1 and §11. That is scoping, not a ruling.

### 13.2 Module path — ANSWERED 2026-09-01

**Answered, and no longer open.** Toni created the repository during this milestone. The module path is
`github.com/telmengedar/processor`, `go.mod` ships it, and §10.8 records the decision and the principle
behind it.

Kept in this section rather than deleted so the record shows the question was asked and settled, not
quietly dropped — but it is **not** an item awaiting anyone.

### 13.3 Default listen address — for Toni, low stakes

The default is `127.0.0.1:8080` (§10.2). If Toni wants it reachable from another machine on the LAN during
development, the default becomes `:8080` — one word, and the variable already overrides it either way.

### 13.4 Which Docker endpoint is this project's test venue? — for Toni

Neither of the two configured endpoints is usable from the agent shell today (C18): the `default` context
points at `bazzite` over SSH and the host does not resolve here, while `desktop-linux` answers its pipe but
returns `500` with Docker Desktop's WSL2 backend stopped. Both remedies are Toni's to apply on his own
desktop, and neither is urgent.

The question is not "please fix Docker" — it is **which endpoint is the intended venue**, because #10439's
smoke test and any later CI story both target it, and they are different designs: a remote host that is
"not always on" needs the test to fail loudly and legibly when it is off, whereas a local engine does not.
**No M0 decision waits on this answer.**

---

## 14. Implementation Guidance for the Next Agent

Ordered so that each step is verifiable before the next begins. **This is one unit of work and one PR** —
the milestones below are build order, not PR boundaries; none of them is independently shippable.

**The two columns are deliberately not the same thing.** *Milestone* names what the step **builds**;
*Done when* names what is **checked**. Where a built leg is not checked, the criterion says so and names
who owns it — step 5 is the case in point: it builds the signal and exit-code wiring, and it checks only
start-and-serve. A criterion is never widened to match a title (§15).

| # | Milestone | Done when |
|---|---|---|
| 1 | **Module and hygiene.** `go.mod` (`go 1.27`, module `github.com/telmengedar/processor` — §10.8), `.gitattributes` (§10.6 — before any Go file is written, so nothing is created in the wrong form), `.gitignore` | `go build ./...` succeeds; `gofmt -l .` prints nothing; **and the module line is read and confirmed to be exactly `github.com/telmengedar/processor`.** The build is not a gate on the path — it passes under any self-consistent one — so the path is checked directly or not at all |
| 2 | **Route table** (§8.2) and its tests (§9.3) | Three tests pass; the `Content-Type` assertion is present and passes |
| 3 | **Lifecycle** (§8.3) and its tests (§9.4), including the paired negative test | Both tests pass; deliberately breaking invariant 1 makes one of them fail — check this, it is the point of the pair |
| 4 | **Boot configuration** (§8.1) and its tests (§9.2) | Three tests pass without touching a real environment variable |
| 5 | **Entry point** (§8.4, §6.1) — wiring, signals, exit codes | The service starts and `GET /health` answers in a browser. **The signal leg is deliberately not a criterion on this step.** No automated signal test ships in M0 (§9.5, §10.7); the implementer does not write one, and he does not run one either — §10.7 keeps live console execution off this desktop. The interrupt outcome was measured once out of tree (C16) and #10439 turns that into a test. An earlier revision put a human `Ctrl+C` run in this column; nobody performed it, and **a criterion may only record a check the person doing the step actually carries out** |
| 6 | **README** (§10.6) | Every command in it has been run and its output matches |
| 7 | **Certify** (§9.6) | `go test -count=1 -v ./...` — exactly two `ok` lines, no `?` line, no `(cached)`; `go vet ./...` and `gofmt -l .` silent |

**Self-checks before handing off** — each is one command and each maps to a claim this document makes:

1. Environment reads across the module: **exactly one site**, in `main` (§8.1 falsifier).
2. Package-level mutable variables: **none**.
3. Fixed ports in tests: **none** (§9.1).
4. `-count=1` present in the quoted test command (§9.6).
5. `? <pkg> [no test files]` lines: **none**.

**What NOT to add**, because the reflex is strong and every one of these was considered and declined:
a router library · an assertion library · a configuration package · a DI container · a service interface ·
panic-recovery or logging middleware · request IDs · a Dockerfile · CI · an error envelope · a version or
build stamp in the health body · DiVoid URL or API key members · overall read/write timeouts · **an
automated signal or console-control-event test** (§10.7 — it works, and it opens a window on somebody's
desktop; #10439 owns the containerised version).

**If any of it seems necessary while implementing, that is a bounce, not a decision** (#114 §0, #1333):
say which principle the design's shape violates and what the alternative is.

---

## 15. Pre-Design Checklist (#1136 §5)

**KISS / DRY / YAGNI**

| Item | Answer |
|---|---|
| No new type mirroring an existing one | ✓ Nothing exists yet to mirror |
| No abstraction with one implementation and no concrete second | ✓ No interface is introduced. §5.3 lists the ones declined and why |
| No element justified by "we might need X later" | ✓ The DiVoid members are the case; they are a prose list, not members (§8.1) |
| No deprecation period, feature flag, compatibility shim, transition window | ✓ None. Nothing exists to be compatible with |
| `block_size × site_count` quoted for any inline-at-N-sites decision | **Not applicable — no such decision exists.** M0 has one route, one lifecycle, one loader; no multi-line block recurs at any site, let alone more than two. There is no duplication to weigh, so there is no threshold to quote |

**Existing systems first**

| Item | Answer |
|---|---|
| Audited whether an existing service/table/DTO covers this | ✓ The repository is empty apart from three untracked documents. Project #10422 has no other code artefact |
| Concrete reason a new layer can't live on an existing surface | ✓ There is no existing surface. Each of the two packages is justified individually in §5, and the merge of routes-with-lifecycle is stated rather than assumed |
| Concrete 4-week decision for any new persisted data point | ✓ Nothing is persisted |
| Consumer chain recursed for anything justified by "an existing reader projects it" | ✓ Applied to the DiVoid boot members: no reader exists in M0, so they are not declared (§8.1) |

**Configurability**

| Item | Answer |
|---|---|
| Every knob has a named operator or an environment difference | ✓ One knob, `PROCESSOR_HTTP_ADDR`. It differs by environment by construction — the test suite binds port 0, a human binds a browsable port, a container binds all interfaces |
| Every "telemetry-then-tune" knob has a filed task naming the reader and the event | ✓ None exists |
| Magic numbers that need not vary stay named constants | ✓ Grace deadline and read-header timeout are constants, with the §3 test applied to each (§8.3) |

**Less is better**

| Item | Answer |
|---|---|
| Every element passed delete / merge / inline | ✓ Applied and recorded: the lifecycle survives *delete* because it owns the bounded drain and the shutdown-sentinel translation, which nothing else in the module could own, and §9.4 is the only cross-platform instrument that pins them; routes and lifecycle were *merged* into one package (§5.2); the configuration package was *inlined* into `main` (§5.3) |
| Trade-offs named where a more complex design wins | ✓ Two: the lookup-function parameter over the environment-setting test helper (§8.1), and the caller-owns-the-listener shape over internal binding (§4) |
| Radical-clean shape chosen where the existing surface has no consumer | ✓ The DiVoid boot members have no consumer and are omitted entirely rather than stubbed — the compromise shape (declare them, leave them unread) is the one #1136 §4 rules out |
| Reader inventories cover AST *and* string-literal references | Not applicable — nothing is being renamed or removed |
| Carrier-swap tables enumerate every affected DTO | Not applicable — no DTO changes |

**Data deliverables** — not applicable. No SQL, no migration, no backfill.

**Document discipline**

| Item | Answer |
|---|---|
| Cites #114 and #1136 as load-bearing | ✓ Header, and applied throughout |
| Reader / scope inventories explicit | ✓ §2, §5.3, §14's do-not-add list |
| Out-of-scope listed explicitly | ✓ §2, as a table with destinations |
| No multi-paragraph rationale for things that obviously stay | ✓ §7 is one sentence because there is no data model |
| Predecessor design marked superseded | Not applicable — this is the first design in the repository |

**Falsifiable-universals check (#1220 §5 addendum).** Three universal claims are made, and each names what
would break it: *"the environment is read in exactly one place"* → more than one environment-read site
(§8.1); *"the suite is fully parallel"* → any fixed port, any environment mutation in a test, any
package-level mutable variable (§9.1); *"the `ok` line certifies a run"* → the `(cached)`,
`[no test files]` and `[no tests to run]` annotations (§9.6). The signal path is named as **not** covered
(§9.5) rather than left to be assumed.

**A fourth claim was made in the first revision and was not falsifiable as stated** — that a Windows
console-interrupt test required machinery "more fragile than the code it would be testing". It named no
experiment, and it was measured false at the first attempt (C16). It has been removed rather than
annotated, per #114's addendum of 2026-08-27: a note under a false paragraph leaves the false paragraph
readable as current. Every remaining gap in §9.5 now carries the experiment that would settle it, which is
the standard the removed sentence failed.

**Revision 3 corrected two further statements — and neither was an unfalsifiable universal.** Both were
ordinary assertions contradicted by a file sitting beside this one, which is the *cheaper* failure to
catch and was caught later than it should have been. §10.8 described the module path as bare `processor`
while `go.mod` shipped `github.com/telmengedar/processor`, and described it **prescriptively**, so the
document argued for breaking the code it describes. Three sites attributed the interrupt-shutdown outcome
to a human `Ctrl+C` run nobody performed; the outcome is real, but its instrument was the out-of-tree C16
probe.

**The sharper of the two is the second, at §14 step 5, where the unperformed check sat in a "Done when"
column.** An acceptance criterion recording an act nobody carried out is worse than a missing one: a
missing criterion is visibly missing, while a false one is discharged by being read. That is §9.5's defect
class — a claim nothing ever contradicts because nobody exercises it — one layer down, in the column whose
entire job is to be exercised. Two rules for future designs fall out: *a criterion may name only a check
its owner performs*, and *every "corroborated by" must name the instrument and identify who operated it.*

**Revision 4 corrected a third shape: a hedge hardened into an accuracy claim.** §3 A1 and §10.8 carried a
forecast — *~3 import lines* — which revision 3 restated as "three import lines", at "exactly the predicted
cost". The tilde and the word *predicted* were the whole of that sentence's honesty; dropping them turned an
estimate into a measurement, and the measurement is **one**. Nothing downstream broke, which is precisely
why it survived a review: **a wrong number in a retrospective costs nothing today and is believed forever.**
Rule: *a figure is labelled by its provenance — forecast or measurement — and a forecast is never restated
in the indicative.*

Two corrections came with it, both the same family — a sentence claiming more than its evidence. The header
called #10424 "mirrored at `VISION.md`" when the two records overlap without either reproducing the other.
And §10.7's rule sentence triggered on "spawns a process" when the harm it measured (C17) was a window
opening on somebody's desktop — so read literally the rule forbade the two runs §14 steps 5 and 6 require,
and the next agent could not tell which manual runs were permitted. Rule: *a standing rule states the harm
it measured, not the nearest broader category that contains it* — with the corollary applied at §14, *a
criterion is never widened to match a milestone's title.*
