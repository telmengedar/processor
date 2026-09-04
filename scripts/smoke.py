#!/usr/bin/env python3
"""Run two live turns of the real product against the real graph and the real model, and print
what each turn sent and got back.

Two turns, not one: the first turn is clean by construction, and the defect this exists to catch
lives in what turn 1 writes to the graph and turn 2 then retrieves from it (DiVoid #11141, #11142).

    python scripts/smoke.py
    python scripts/smoke.py --subject 10422 --input "the question you actually want to ask"

The default input and subject match no row in internal/eval/corpus.json, deliberately: a run writes a
record that outranks every real candidate for its own input, so a default matching a corpus row would
poison the next sweep of that row.

Costs two model calls at minimum and six at most, and writes two run records to the graph which it
names on exit; it deletes nothing. Interrupting a run in flight waits for the write-back rather than
killing it — a second interrupt kills, and a kill landing between the record's node and its body
leaves a bodyless node behind.

Exit 0 both turns admitted context; 1 a turn admitted zero candidates; 2 the run could not be made,
or turn 1 never stored its record and turn 2 therefore had nothing new to retrieve.
"""

import argparse
import http.client
import json
import os
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

DEFAULT_INPUT = (
    "Three agents wrote the code, the design and the review. "
    "Who owns the text of the pull request body?"
)
DEFAULT_SUBJECT = 10192
DEFAULT_MODEL_URL = "http://127.0.0.1:8099"
DEFAULT_MODEL_ID = "qwen/qwen3-coder-480b-a35b-instruct-maas"
RUN_NAME_PREFIX = "processor-run"

HEALTH_TIMEOUT_S = 30
RUN_TIMEOUT_S = 11 * 60 + 30
IDLE_GRACE_S = 15
DRAIN_GRACE_S = 11 * 60

RULE = "=" * 88
THIN = "-" * 88


class SmokeFailure(Exception):
    pass


def child_env():
    env = os.environ.copy()

    url = env.get("PROCESSOR_DIVOID_URL") or env.get("DIVOID_URL")
    key = env.get("PROCESSOR_DIVOID_KEY") or env.get("DIVOID_RAZIEL_KEY")
    if not url or not key:
        raise SmokeFailure(
            "no graph credential. The binary reads PROCESSOR_DIVOID_URL and PROCESSOR_DIVOID_KEY; "
            "this machine supplies the same credential as DIVOID_URL and DIVOID_RAZIEL_KEY and this "
            "script maps one onto the other. Neither pair is set, so set DIVOID_URL and "
            "DIVOID_RAZIEL_KEY (or the PROCESSOR_ names directly) and run again."
        )

    env["PROCESSOR_DIVOID_URL"] = url
    env["PROCESSOR_DIVOID_KEY"] = key
    env.setdefault("PROCESSOR_MODEL_URL", DEFAULT_MODEL_URL)
    env.setdefault("PROCESSOR_MODEL_ID", DEFAULT_MODEL_ID)
    return env


def build(binary):
    result = subprocess.run(
        ["go", "build", "-o", str(binary), "./cmd/processor"], cwd=str(REPO)
    )
    if result.returncode != 0:
        raise SmokeFailure("go build ./cmd/processor failed; its output is above")


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def start_server(binary, env, log):
    flags = subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0
    return subprocess.Popen(
        [str(binary)], env=env, cwd=str(REPO), stdout=log, stderr=subprocess.STDOUT,
        creationflags=flags,
    )


def wait_for_health(proc, port, log_path):
    deadline = time.monotonic() + HEALTH_TIMEOUT_S
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            raise SmokeFailure(
                f"the server exited with code {proc.returncode} before answering /health; "
                f"its log follows\n{read_log(log_path)}"
            )
        try:
            with urllib.request.urlopen(f"http://127.0.0.1:{port}/health", timeout=2) as response:
                if json.loads(response.read()).get("status") == "ok":
                    return
        except (urllib.error.URLError, OSError, json.JSONDecodeError):
            time.sleep(0.2)
    raise SmokeFailure(
        f"the server never answered /health on port {port} within {HEALTH_TIMEOUT_S}s; "
        f"its log follows\n{read_log(log_path)}"
    )


def post_run(port, text, subject):
    body = json.dumps({"input": text, "subject": subject}).encode("utf-8")
    request = urllib.request.Request(
        f"http://127.0.0.1:{port}/runs", data=body,
        headers={"Content-Type": "application/json"}, method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=RUN_TIMEOUT_S) as response:
            payload = response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", "replace").strip()
        raise SmokeFailure(f"POST /runs returned {err.code}: {detail}") from err
    except (urllib.error.URLError, OSError, http.client.HTTPException) as err:
        raise SmokeFailure(f"POST /runs did not complete: {type(err).__name__}: {err}") from err

    try:
        return json.loads(payload)
    except json.JSONDecodeError as err:
        raise SmokeFailure(
            f"POST /runs returned 200 and {len(payload)} bytes that are not JSON ({err}); "
            f"the body starts {payload[:200]!r}"
        ) from err


def stop_server(proc, log_path, run_in_flight):
    if proc.poll() is not None:
        print(f"server: already gone, exit code {proc.returncode}")
        return
    grace = DRAIN_GRACE_S if run_in_flight else IDLE_GRACE_S
    if run_in_flight:
        print(
            f"server: a run may still be in flight — waiting up to {grace // 60} minutes for its "
            f"write-back rather than killing it. Interrupt again to kill; a kill landing between "
            f"the run record's node and its body leaves a bodyless node in the graph."
        )
    if os.name == "nt":
        proc.send_signal(signal.CTRL_BREAK_EVENT)
    else:
        proc.terminate()
    try:
        proc.wait(timeout=grace)
    except (subprocess.TimeoutExpired, KeyboardInterrupt):
        proc.kill()
        proc.wait(timeout=5)
        print("server: killed without draining — a run record may be at a node with no body")
        return
    graceful = "shutdown complete" in read_log(log_path)
    print(f"server: stopped, exit code {proc.returncode}" + (", drained" if graceful else ""))


def read_log(log_path):
    try:
        return Path(log_path).read_text(encoding="utf-8", errors="replace")
    except OSError:
        return "(no server log)"


def admitted(record):
    return sum(1 for c in candidates(record) if c.get("included"))


def candidates(record):
    return record.get("candidates") or []


def block_bytes(record):
    return len(record.get("block", "").encode("utf-8"))


def one_line(value, width):
    text = " ".join(str(value).split())
    return text if len(text) <= width else text[: width - 1] + "…"


def print_turn(number, record):
    print()
    print(RULE)
    print(f"TURN {number}")
    print(RULE)

    rows = candidates(record)
    kept = admitted(record)
    print(f"included: {kept} of {len(rows)}")

    cuts = {}
    for row in rows:
        if not row.get("included"):
            reason = row.get("cutReason") or "(no reason given)"
            cuts[reason] = cuts.get(reason, 0) + 1
    for reason, count in sorted(cuts.items(), key=lambda kv: -kv[1]):
        print(f"cut: {count} x {reason}")

    budget = (record.get("limits") or {}).get("assemblyByteBudget")
    for row in rows:
        if budget and row.get("size", 0) > budget:
            print(
                f"UNADMITTABLE: #{row.get('id')} is {row.get('size')} bytes against a "
                f"{budget}-byte assembly budget, so no run can ever admit it. It costs its own "
                f"slot; the candidates behind it are still considered."
            )

    anchor = record.get("anchor") or {}
    print(
        f"anchor: #{anchor.get('id')} {anchor.get('type')} "
        f"{one_line(anchor.get('name'), 50)} {anchor.get('size')} bytes"
    )
    print(f"query: {one_line(record.get('query'), 80)}")
    print()

    for row in rows:
        mark = "IN " if row.get("included") else "cut"
        print(
            f"  {mark} rank {row.get('rank'):>2}  #{row.get('id'):<7} "
            f"{one_line(row.get('type'), 14):<14} sim {row.get('similarity', 0):.4f}  "
            f"size {row.get('size', 0):>6}  {one_line(row.get('name'), 44)}"
        )

    print()
    print(f"BLOCK SENT TO THE MODEL, {block_bytes(record)} bytes, verbatim:")
    print(THIN)
    print(record.get("block", ""))
    print(THIN)

    stop = record.get("stopReason") or {}
    usage = record.get("usage") or []
    tokens = ", ".join(
        f"{u['inTokens']} in / {u['outTokens']} out" if u else "no usage reported" for u in usage
    )
    print()
    print(
        f"MODEL: {record.get('model')}  modelCalls {record.get('modelCalls')}  "
        f"capReached {record.get('capReached')}  "
        f"stopReason {stop.get('reason')} (raw {stop.get('raw')!r})  {tokens}"
    )

    tool_calls = record.get("toolCalls") or []
    if not tool_calls:
        print("SUPPLEMENTARY LOOKUP: not used — the model answered from the block above alone")
    else:
        print(f"SUPPLEMENTARY LOOKUP: used, {len(tool_calls)} round(s)")
        for index, call in enumerate(tool_calls, 1):
            results = call.get("results") or []
            if call.get("error"):
                print(f"  round {index}: ERROR {call['error']}")
            else:
                kept_here = sum(1 for r in results if r.get("included"))
                print(
                    f"  round {index}: query {one_line(call.get('query'), 60)!r} "
                    f"-> {kept_here} of {len(results)} admitted"
                )

    print()
    print("MODEL ANSWER, verbatim:")
    print(THIN)
    print(record.get("answer", ""))
    print(THIN)


def print_diff(first, second):
    print()
    print(RULE)
    print("TURN 1 -> TURN 2")
    print(RULE)

    rows_first, rows_second = candidates(first), candidates(second)
    comparison = [
        ("admitted", admitted(first), admitted(second)),
        ("candidates returned", len(rows_first), len(rows_second)),
        (
            "rank-1 candidate",
            f"#{rows_first[0]['id']}" if rows_first else "(none)",
            f"#{rows_second[0]['id']}" if rows_second else "(none)",
        ),
        ("block bytes", block_bytes(first), block_bytes(second)),
    ]
    for label, before, after in comparison:
        changed = "   CHANGED" if before != after else ""
        print(f"  {label:<22} {str(before):>10}  ->  {str(after):<10}{changed}")


def print_records_written(receipts):
    print()
    stored = [r for r in receipts if r.get("nodeId")]
    if not stored:
        print("GRAPH: no run record was stored — the output above is the only copy.")
        return
    ids = ", ".join(f"#{r['nodeId']} ({r['state']})" for r in stored)
    print(f"GRAPH: this run wrote {len(stored)} run record(s) to the graph: {ids}")
    print(
        "       Nothing was deleted. Run records rank first on a repeat of the same input and are "
        "larger than the assembly budget, so leaving them in place changes what the next run "
        "retrieves and contaminates the baseline sweep (DiVoid #11133, #11141). Delete them by hand "
        "if that matters."
    )


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--input", default=DEFAULT_INPUT, help="the run input, sent twice unchanged")
    parser.add_argument("--subject", type=int, default=DEFAULT_SUBJECT, help="the subject node id")
    return parser.parse_args()


def main():
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    args = parse_args()

    try:
        env = child_env()
    except SmokeFailure as err:
        print(f"FAIL: {err}")
        return 2

    with tempfile.TemporaryDirectory(prefix="processor-smoke-", ignore_cleanup_errors=True) as tmp:
        binary = Path(tmp) / ("processor.exe" if os.name == "nt" else "processor")
        log_path = Path(tmp) / "server.log"
        log = None
        proc = None
        receipts = []
        in_flight = False
        try:
            build(binary)
            port = free_port()
            env["PROCESSOR_HTTP_ADDR"] = f"127.0.0.1:{port}"
            print(
                f"model {env['PROCESSOR_MODEL_ID']} at {env['PROCESSOR_MODEL_URL']}, "
                f"graph {env['PROCESSOR_DIVOID_URL']}, server on 127.0.0.1:{port}"
            )
            print(f"input: {args.input}")
            print(f"subject: #{args.subject}")

            log = open(log_path, "wb")
            proc = start_server(binary, env, log)
            wait_for_health(proc, port, log_path)

            records = []
            for turn in (1, 2):
                in_flight = True
                response = post_run(port, args.input, args.subject)
                in_flight = False
                receipts.append(response.get("written") or {})
                records.append(response)
                print_turn(turn, response)

            print_diff(records[0], records[1])

            prior = [
                c for c in candidates(records[0])
                if str(c.get("name", "")).startswith(RUN_NAME_PREFIX)
            ]
            if prior:
                print()
                print(
                    f"TURN 1 WAS NOT CLEAN: {', '.join('#' + str(c['id']) for c in prior)} — run "
                    f"record(s) this tool wrote on an earlier invocation — were already among turn "
                    f"1's candidates. Turn 1 is a clean baseline only on a graph nothing has "
                    f"answered this input on before, so the comparison above understates what one "
                    f"run does to the next. Delete them, or pass a --input nothing has run yet."
                )

            premise_void = receipts[0].get("state") == "notStored"
            if premise_void:
                print()
                print(
                    "PREMISE NOT ESTABLISHED: turn 1's record was never stored, so turn 2 recalled "
                    "from the same memory state turn 1 did. The two turns above are one turn run "
                    "twice, and the comparison shows nothing about what a run leaves behind."
                )

            zero = [(n, r) for n, r in enumerate(records, 1) if not admitted(r)]
            if zero:
                print()
                for number, record in zero:
                    print(
                        f"FAIL: turn {number} admitted 0 of {len(candidates(record))} candidates — "
                        f"the model was sent the anchor and nothing else, and answered with no "
                        f"retrieved context at all."
                    )
                return 1

            if premise_void:
                print()
                print(
                    "FAIL: the two-turn premise did not hold, so this run tested one turn twice. "
                    "That is the instrument failing, not the product."
                )
                return 2

            print()
            print("PASS: both turns admitted context. Read the two answers above; no rate can judge them.")
            return 0

        except SmokeFailure as err:
            print()
            print(f"FAIL: {err}")
            return 2
        except KeyboardInterrupt:
            print()
            print("FAIL: interrupted")
            return 2
        finally:
            if proc is not None:
                stop_server(proc, log_path, in_flight)
            if log is not None:
                log.close()
            if receipts:
                print_records_written(receipts)


if __name__ == "__main__":
    sys.exit(main())
