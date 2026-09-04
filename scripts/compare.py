#!/usr/bin/env python3
"""Run each task twice -- once through the whole product, once as a plain model call with no
assembled context -- and print both answers verbatim, side by side.

    python scripts/compare.py
    python scripts/compare.py --only t1-second-provider,t3-second-ask-worse
    python scripts/compare.py --model-url http://127.0.0.1:12434/engines/v1 --model-id qwen3-coder

Arm SUBSTRATE is POST /runs against the built binary: retrieve, assemble, judge, write back.
Arm TRANSCRIPT is the same model and the same task text posted straight to /chat/completions as a
single user message, with no system message, no context block and no recall tool.

This is not the A/B of DiVoid #11092. That protocol varies one configuration difference and both of
its arms are this system; it answers whether a change helped and says in its own section 5.2 that it
cannot answer whether the harness is worth having. This asks the blunter question first, so it prints
no verdict: there is no rubric, no blinding and no repeat, and a script that scored these outputs
would be inventing a measurement. It prints the evidence; a human reads it.

The evidence includes a stage, because losing is the expected outcome and a bare loss names nothing to
fix. Each task in the set names the nodes carrying its answer, and the record says of each whether
recall returned it and whether assembly admitted it -- so a loss lands on retrieval, on the byte
budget, or past the block entirely. The stage stops at the block: whether an admitted node was any use
is the reader's call on the two answers, and it is the case no instrument in this project can see.

Every task's substrate arm writes one run record to the graph, which this names on exit and never
deletes. A repeated task is refused before any model call rather than warned about, because the second
run of one input reads the record the first one filed (DiVoid #11141).

Refusing repeats does not buy a clean invocation, and the first full run of the shipped set measured
why: task 3 retrieved the records tasks 1 and 2 had just written, minutes earlier, on unrelated text.
#11141 documents the mechanism for a repeated input; it also fires across different inputs, so a task
late in a set is read against a graph the earlier ones moved. The per-task PRIOR OUTPUT line reports
it whenever it happens. Nothing here prevents it, and the fix is #11092 section 4.2's decorating port,
which is Go and has no caller yet.

Costs one model call per arm at minimum -- more where the substrate arm asks for supplementary recall
-- and one graph write per task. Exit 0 every task ran both arms; 1 a task's substrate arm admitted no
candidates, so for that task the two arms were the same call; 2 the comparison could not be made.
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
DEFAULT_TASKS = Path(__file__).resolve().parent / "compare_tasks.json"

RUN_NAME_PREFIX = "processor-run"

HEALTH_TIMEOUT_S = 30
PROBE_TIMEOUT_S = 60
RUN_TIMEOUT_S = 11 * 60 + 30
TRANSCRIPT_TIMEOUT_S = 5 * 60
IDLE_GRACE_S = 15
DRAIN_GRACE_S = 11 * 60

RULE = "=" * 88
THIN = "-" * 88


class CompareFailure(Exception):
    pass


def load_tasks(path, only):
    try:
        rows = json.loads(Path(path).read_text(encoding="utf-8"))
    except OSError as err:
        raise CompareFailure(f"the task set at {path} could not be read: {err}") from err
    except json.JSONDecodeError as err:
        raise CompareFailure(f"the task set at {path} is not valid JSON: {err}") from err

    if not isinstance(rows, list) or not rows:
        raise CompareFailure(f"the task set at {path} must be a non-empty JSON list")

    for index, row in enumerate(rows):
        where = f"{path} entry {index}"
        for key in ("id", "task", "subject"):
            if key not in row:
                raise CompareFailure(f"{where} has no {key!r}")
        if not str(row["task"]).strip():
            raise CompareFailure(f"{where} has an empty task")
        if not isinstance(row["subject"], int) or row["subject"] <= 0:
            raise CompareFailure(f"{where} has subject {row['subject']!r}; node ids are positive integers")

    seen_ids = duplicates(row["id"] for row in rows)
    if seen_ids:
        raise CompareFailure(f"{path} reuses the task id(s) {', '.join(sorted(seen_ids))}")

    seen_tasks = duplicates(" ".join(str(row["task"]).split()) for row in rows)
    if seen_tasks:
        raise CompareFailure(
            f"{path} carries the same task text more than once. Refused rather than run: the substrate "
            f"arm files a run record embedding its input verbatim, so the second run of one text reads "
            f"what the first one wrote and is not a second measurement of anything (DiVoid #11141). "
            f"The repeated text begins {sorted(seen_tasks)[0][:80]!r}"
        )

    if only is None:
        return rows

    wanted = [name.strip() for name in only.split(",") if name.strip()]
    by_id = {row["id"]: row for row in rows}
    missing = [name for name in wanted if name not in by_id]
    if missing:
        raise CompareFailure(
            f"--only names {', '.join(missing)}, which {path} does not define. It defines "
            f"{', '.join(by_id)}"
        )
    return [by_id[name] for name in wanted]


def duplicates(values):
    seen, repeated = set(), set()
    for value in values:
        if value in seen:
            repeated.add(value)
        seen.add(value)
    return repeated


def model_config(args):
    url = args.model_url or os.environ.get("PROCESSOR_MODEL_URL")
    model_id = args.model_id or os.environ.get("PROCESSOR_MODEL_ID")
    key = args.model_key or os.environ.get("PROCESSOR_MODEL_KEY") or ""

    if not url:
        raise CompareFailure(
            "no model endpoint. Both arms call the same one -- arm TRANSCRIPT directly and arm "
            "SUBSTRATE through the binary -- so there is nothing to compare without it. Pass "
            "--model-url or set PROCESSOR_MODEL_URL. A local Docker Model Runner serves it at "
            "http://127.0.0.1:12434/engines/v1"
        )
    if not model_id:
        raise CompareFailure(
            "no model id. Both arms must request the same model or the comparison is between two "
            "models rather than two harnesses. Pass --model-id or set PROCESSOR_MODEL_ID"
        )
    return url.rstrip("/"), model_id, key


def child_env(model_url, model_id, model_key):
    env = os.environ.copy()

    url = env.get("PROCESSOR_DIVOID_URL") or env.get("DIVOID_URL")
    key = env.get("PROCESSOR_DIVOID_KEY") or env.get("DIVOID_RAZIEL_KEY")
    if not url or not key:
        raise CompareFailure(
            "no graph credential, and arm SUBSTRATE is the graph. The binary reads "
            "PROCESSOR_DIVOID_URL and PROCESSOR_DIVOID_KEY; this machine supplies the same credential "
            "as DIVOID_URL and DIVOID_RAZIEL_KEY and this script maps one onto the other. Neither "
            "pair is set, so set DIVOID_URL and DIVOID_RAZIEL_KEY (or the PROCESSOR_ names directly)"
        )

    env["PROCESSOR_DIVOID_URL"] = url
    env["PROCESSOR_DIVOID_KEY"] = key
    env["PROCESSOR_MODEL_URL"] = model_url
    env["PROCESSOR_MODEL_ID"] = model_id
    if model_key:
        env["PROCESSOR_MODEL_KEY"] = model_key
    return env


def chat(model_url, model_id, model_key, text, max_tokens, timeout):
    body = json.dumps({
        "model": model_id,
        "messages": [{"role": "user", "content": text}],
        "max_tokens": max_tokens,
    }).encode("utf-8")

    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if model_key:
        headers["Authorization"] = "Bearer " + model_key

    request = urllib.request.Request(
        model_url + "/chat/completions", data=body, headers=headers, method="POST"
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload = response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", "replace").strip()
        raise CompareFailure(f"POST {model_url}/chat/completions returned {err.code}: {detail}") from err
    except (urllib.error.URLError, OSError, http.client.HTTPException) as err:
        raise CompareFailure(
            f"POST {model_url}/chat/completions did not complete: {type(err).__name__}: {err}"
        ) from err

    try:
        wire = json.loads(payload)
    except json.JSONDecodeError as err:
        raise CompareFailure(
            f"{model_url}/chat/completions returned {len(payload)} bytes that are not JSON ({err}); "
            f"the body starts {payload[:200]!r}"
        ) from err

    choices = wire.get("choices") or []
    if not choices:
        raise CompareFailure(
            f"{model_url}/chat/completions returned a response with no choices: {payload[:200]!r}"
        )
    message = choices[0].get("message") or {}
    return {
        "answer": message.get("content") or "",
        "finishReason": choices[0].get("finishReason") or choices[0].get("finish_reason"),
        "usage": wire.get("usage") or {},
    }


def probe_model(model_url, model_id, model_key):
    try:
        chat(model_url, model_id, model_key, "ping", 1, PROBE_TIMEOUT_S)
    except CompareFailure as err:
        raise CompareFailure(
            f"the model endpoint is unusable and both arms need it -- arm TRANSCRIPT calls it "
            f"directly, arm SUBSTRATE calls it through the binary. The endpoint came from "
            f"--model-url or PROCESSOR_MODEL_URL as {model_url!r} and the model id from --model-id "
            f"or PROCESSOR_MODEL_ID as {model_id!r}. The probe was a one-token completion and it "
            f"failed: {err}"
        ) from err


def build(binary):
    result = subprocess.run(["go", "build", "-o", str(binary), "./cmd/processor"], cwd=str(REPO))
    if result.returncode != 0:
        raise CompareFailure("go build ./cmd/processor failed; its output is above")


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
            raise CompareFailure(
                f"the server exited with code {proc.returncode} before answering /health, so arm "
                f"SUBSTRATE has no endpoint; its log follows\n{read_log(log_path)}"
            )
        try:
            with urllib.request.urlopen(f"http://127.0.0.1:{port}/health", timeout=2) as response:
                if json.loads(response.read()).get("status") == "ok":
                    return
        except (urllib.error.URLError, OSError, json.JSONDecodeError):
            time.sleep(0.2)
    raise CompareFailure(
        f"the server never answered /health on port {port} within {HEALTH_TIMEOUT_S}s, so arm "
        f"SUBSTRATE has no endpoint; its log follows\n{read_log(log_path)}"
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
        raise CompareFailure(f"POST /runs returned {err.code}: {detail}") from err
    except (urllib.error.URLError, OSError, http.client.HTTPException) as err:
        raise CompareFailure(f"POST /runs did not complete: {type(err).__name__}: {err}") from err

    try:
        return json.loads(payload)
    except json.JSONDecodeError as err:
        raise CompareFailure(
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
            f"server: a run may still be in flight -- waiting up to {grace // 60} minutes for its "
            f"write-back rather than killing it. Interrupt again to kill; a kill landing between the "
            f"run record's node and its body leaves a bodyless node in the graph."
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
        print("server: killed without draining -- a run record may be at a node with no body")
        return
    graceful = "shutdown complete" in read_log(log_path)
    print(f"server: stopped, exit code {proc.returncode}" + (", drained" if graceful else ""))


def read_log(log_path):
    try:
        return Path(log_path).read_text(encoding="utf-8", errors="replace")
    except OSError:
        return "(no server log)"


def candidates(record):
    return record.get("candidates") or []


def admitted(record):
    return sum(1 for c in candidates(record) if c.get("included"))


def block_bytes(record):
    return len(record.get("block", "").encode("utf-8"))


def answer_bytes(text):
    return len((text or "").encode("utf-8"))


def one_line(value, width):
    text = " ".join(str(value).split())
    return text if len(text) <= width else text[: width - 1] + "…"


NOT_RETRIEVED = "not retrieved"
RETRIEVED_CUT = "retrieved, cut"
ADMITTED = "admitted"
UNLABELLED = "no answer nodes named"

INDICTMENT = {
    NOT_RETRIEVED: "retrieval never returned the answer at all -- the recall query, not the budget",
    RETRIEVED_CUT: "recall found the answer and assembly dropped it -- the byte budget and admission",
    ADMITTED: (
        "the answer reached the model. Anything wrong past here is the prompt, the model, or the "
        "node's own content, and only reading both answers separates those three"
    ),
    UNLABELLED: "the task set names no answering node, so nothing mechanical can be attributed",
}


def answer_node_report(task, record):
    wanted = task.get("answerNodes") or []
    if not wanted:
        return []
    by_id = {c.get("id"): c for c in candidates(record)}
    rows = []
    for node in wanted:
        row = by_id.get(node)
        if row is None:
            rows.append((node, NOT_RETRIEVED, None))
        elif row.get("included"):
            rows.append((node, ADMITTED, row.get("rank")))
        else:
            reason = row.get("cutReason") or "no reason given"
            rows.append((node, f"{RETRIEVED_CUT} ({reason})", row.get("rank")))
    return rows


def task_stage(task, record):
    rows = answer_node_report(task, record)
    if not rows:
        return UNLABELLED
    dispositions = [disposition for _, disposition, _ in rows]
    if any(d == ADMITTED for d in dispositions):
        return ADMITTED
    if any(d.startswith(RETRIEVED_CUT) for d in dispositions):
        return RETRIEVED_CUT
    return NOT_RETRIEVED


def print_task(task, record, transcript):
    print()
    print(RULE)
    print(f"TASK {task['id']}   subject #{task['subject']}")
    print(RULE)
    print(task["task"])

    print()
    print(
        f"SUBSTRATE retrieval: {admitted(record)} of {len(candidates(record))} candidates admitted, "
        f"block {block_bytes(record)} bytes, {record.get('modelCalls')} model call(s)"
    )
    anchor = record.get("anchor") or {}
    print(
        f"           anchor: #{anchor.get('id')} {anchor.get('type')} {anchor.get('size')} bytes  "
        f"{one_line(anchor.get('name'), 40)}"
    )

    budget = (record.get("limits") or {}).get("assemblyByteBudget")
    for row in candidates(record):
        if budget and row.get("size", 0) > budget:
            print(
                f"           UNADMITTABLE: #{row.get('id')} is {row.get('size')} bytes against a "
                f"{budget}-byte assembly budget, so no run can admit it. Admission stops at the "
                f"first candidate that does not fit, so it also cut everything ranked behind it."
            )

    mine = [c for c in candidates(record) if str(c.get("name", "")).startswith(RUN_NAME_PREFIX)]
    if mine:
        ids = ", ".join(f"#{c['id']}" for c in mine)
        was = "is a run record" if len(mine) == 1 else "are run records"
        print(
            f"           PRIOR OUTPUT IN CANDIDATES: {ids} {was} an earlier invocation of this tool "
            f"or of smoke.py left in the graph. Arm SUBSTRATE read its own history here, so this "
            f"task's retrieval is not the retrieval a clean graph would have produced."
        )

    stage = task_stage(task, record)
    print()
    print(f"STAGE: {stage} -- {INDICTMENT[stage]}")
    for node, disposition, rank in answer_node_report(task, record):
        at = f" at rank {rank}" if rank is not None else ""
        print(f"       answer node #{node}: {disposition}{at}")
    if stage == ADMITTED:
        print(
            "       The mechanical attribution stops here. Whether the substrate answer actually "
            "draws on the node, and whether it beats the transcript answer anyway, is the reader's "
            "call on the two texts below -- an admitted node that the answer ignores indicts the "
            "prompt, and one the answer uses and still loses on indicts the node's content."
        )

    print()
    print("SUBSTRATE ANSWER, verbatim:")
    print(THIN)
    print(record.get("answer", ""))
    print(THIN)

    print()
    print("TRANSCRIPT ANSWER, verbatim (same model, same text, no context block, no tool):")
    print(THIN)
    print(transcript["answer"])
    print(THIN)

    tool_calls = record.get("toolCalls") or []
    if tool_calls:
        print()
        print(f"SUBSTRATE supplementary lookup: used, {len(tool_calls)} round(s)")
        for index, call in enumerate(tool_calls, 1):
            results = call.get("results") or []
            if call.get("error"):
                print(f"  round {index}: ERROR {call['error']}")
            else:
                kept = sum(1 for r in results if r.get("included"))
                print(
                    f"  round {index}: query {one_line(call.get('query'), 55)!r} "
                    f"-> {kept} of {len(results)} admitted"
                )


def print_table(rows):
    print()
    print(RULE)
    print("ACROSS TASKS -- facts only; nothing here ranks the two arms")
    print(RULE)
    print(
        f"  {'task':<27} {'admit':>7} {'block B':>8} {'calls':>5} {'nodes':>6} "
        f"{'subst B':>8} {'trans B':>8}  stage"
    )
    for task, record, transcript in rows:
        hits = answer_node_report(task, record)
        found = sum(1 for _, disposition, _ in hits if disposition == ADMITTED)
        print(
            f"  {one_line(task['id'], 27):<27} "
            f"{admitted(record):>3}/{len(candidates(record)):<3} "
            f"{block_bytes(record):>8} {record.get('modelCalls'):>5} "
            f"{f'{found}/{len(hits)}' if hits else '-':>6} "
            f"{answer_bytes(record.get('answer')):>8} {answer_bytes(transcript['answer']):>8}  "
            f"{task_stage(task, record)}"
        )

    tally = {}
    for task, record, _ in rows:
        stage = task_stage(task, record)
        tally[stage] = tally.get(stage, 0) + 1
    print()
    for stage, count in sorted(tally.items(), key=lambda kv: -kv[1]):
        print(f"  {count} task(s) reached stage {stage!r}: {INDICTMENT[stage]}")
    print()
    print(
        "  admit = candidates admitted of candidates returned; nodes = answer nodes the task set "
        "names that arm SUBSTRATE admitted; subst B / trans B = answer sizes in bytes. Answer size is "
        "a length, not a quality, and the stage says where the answer got to, never whether it is "
        "right. Read the two answers above."
    )


def print_records_written(receipts):
    print()
    stored = [r for r in receipts if r.get("nodeId")]
    if not stored:
        print("GRAPH: no run record was stored -- the output above is the only copy.")
        return
    ids = ", ".join(f"#{r['nodeId']} ({r['state']})" for r in stored)
    print(f"GRAPH: this invocation wrote {len(stored)} run record(s): {ids}")
    print(
        "       Nothing was deleted. Each record embeds its task text verbatim and is larger than the "
        "assembly budget, so it ranks early on any repeat of that text and admission stops there "
        "(DiVoid #11141, #11133). Leaving them in place changes what the next invocation of this tool "
        "retrieves and can move a corpus row's retrieval in the baseline sweep. Delete them by hand."
    )


def parse_args():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--tasks", default=str(DEFAULT_TASKS), help="the task set JSON file")
    parser.add_argument("--only", help="comma-separated task ids to run, in that order")
    parser.add_argument("--model-url", help="overrides PROCESSOR_MODEL_URL for both arms")
    parser.add_argument("--model-id", help="overrides PROCESSOR_MODEL_ID for both arms")
    parser.add_argument("--model-key", help="overrides PROCESSOR_MODEL_KEY for both arms")
    return parser.parse_args()


def main():
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    args = parse_args()

    try:
        tasks = load_tasks(args.tasks, args.only)
        model_url, model_id, model_key = model_config(args)
        env = child_env(model_url, model_id, model_key)
        probe_model(model_url, model_id, model_key)
    except CompareFailure as err:
        print(f"FAIL: {err}")
        return 2

    with tempfile.TemporaryDirectory(prefix="processor-compare-", ignore_cleanup_errors=True) as tmp:
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
                f"model {model_id} at {model_url}, graph {env['PROCESSOR_DIVOID_URL']}, "
                f"server on 127.0.0.1:{port}"
            )
            print(f"tasks: {len(tasks)} from {args.tasks}")

            log = open(log_path, "wb")
            proc = start_server(binary, env, log)
            wait_for_health(proc, port, log_path)

            rows = []
            for task in tasks:
                in_flight = True
                response = post_run(port, task["task"], task["subject"])
                in_flight = False
                receipts.append(response.get("written") or {})

                max_tokens = (response.get("limits") or {}).get("maxOutputTokens")
                if not max_tokens:
                    raise CompareFailure(
                        f"task {task['id']}'s run record carries no limits.maxOutputTokens, so arm "
                        f"TRANSCRIPT has no output bound to match. The two arms would be bounded "
                        f"differently and the comparison would not be one."
                    )
                transcript = chat(
                    model_url, model_id, model_key, task["task"], max_tokens, TRANSCRIPT_TIMEOUT_S
                )

                print_task(task, response, transcript)
                rows.append((task, response, transcript))

            print_table(rows)

            starved = [(t, r) for t, r, _ in rows if not admitted(r)]
            if starved:
                print()
                for task, record in starved:
                    print(
                        f"FAIL: task {task['id']} admitted 0 of {len(candidates(record))} candidates. "
                        f"Arm SUBSTRATE was sent the anchor and nothing else, so for this task the "
                        f"two arms differ by one node rather than by a memory substrate, and the "
                        f"comparison above is not one."
                    )
                return 1

            print()
            print(
                "DONE: every task ran both arms. No verdict is printed and none is available from "
                "this instrument -- read the answer pairs, and read each one against the stage above "
                "it, which says how far the answer to that task got before the model saw anything."
            )
            return 0

        except CompareFailure as err:
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
