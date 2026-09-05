#!/usr/bin/env python3
"""Run each task twice -- once through the whole product, once as a plain model call with no
assembled context -- and print both answers verbatim, side by side.

    python scripts/compare.py
    python scripts/compare.py --only t1-second-provider,t3-second-ask-worse
    python scripts/compare.py --model-url http://127.0.0.1:12434/engines/v1 --model-id qwen3-coder

Requires Python 3.10+: TemporaryDirectory(ignore_cleanup_errors=...) below needs 3.10.

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
deletes. Two checks run before any model call to keep a repeat from reading its own prior record back:
the loader refuses a task set that carries the same task text twice in one file, and a graph query
(find_prior_run) refuses a task whose text an earlier invocation's run record already carries verbatim
(DiVoid #11141). The graph check is a semantic recall against the same endpoint the binary itself
queries, ranked to a fixed number of rows -- it can miss an older record that no longer ranks near its
own text, so it is a real check and not a guarantee.

Neither check buys a clean invocation, and the first full run of the shipped set measured why: task 3
retrieved the records tasks 1 and 2 had just written, minutes earlier, on unrelated text. #11141
documents the mechanism for a repeated input; it also fires across different inputs, so a task late in
a set is read against a graph the earlier ones moved. The per-task RETRIEVAL-SLOT DISPLACEMENT line
reports it whenever it happens. Nothing here prevents that, and the fix is #11092 section 4.2's
decorating port, which is Go and has no caller yet.

Costs one model call per arm at minimum -- more where the substrate arm asks for supplementary recall
-- and one graph write per task. Exit 0 every task ran both arms; 1 a task's substrate arm admitted
nothing at all -- neither in the initial context block nor in any supplementary recall round -- so for
that task the two arms were the same call; 2 the comparison could not be made.
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
import textwrap
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
DEFAULT_TASKS = Path(__file__).resolve().parent / "compare_tasks.json"

RUN_NAME_PREFIX = "processor-run"
RUN_NODE_TYPE = "session-log"

CUT_SELF_PRODUCED = "self-produced"
CUT_BYTE_BUDGET = "byte budget exceeded"

HEALTH_TIMEOUT_S = 30
PROBE_TIMEOUT_S = 60
RUN_TIMEOUT_S = 11 * 60 + 30
TRANSCRIPT_TIMEOUT_S = 5 * 60
IDLE_GRACE_S = 15
DRAIN_GRACE_S = 11 * 60
GRAPH_QUERY_TIMEOUT_S = 15
GRAPH_QUERY_COUNT = 50

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
        if not isinstance(row, dict):
            raise CompareFailure(f"{where} is a {type(row).__name__}, not a JSON object")
        for key in ("id", "task", "subject"):
            if key not in row:
                raise CompareFailure(f"{where} has no {key!r}")
        if not str(row["task"]).strip():
            raise CompareFailure(f"{where} has an empty task")
        if not isinstance(row["subject"], int) or row["subject"] <= 0:
            raise CompareFailure(f"{where} has subject {row['subject']!r}; node ids are positive integers")
        answer_nodes = row.get("answerNodes")
        if answer_nodes is not None and (
            not isinstance(answer_nodes, list)
            or not all(isinstance(node, int) and not isinstance(node, bool) for node in answer_nodes)
        ):
            raise CompareFailure(
                f"{where} has answerNodes {answer_nodes!r}; it must be a JSON list of integers. Run "
                f"records carry node ids as numbers, so a quoted id such as \"11141\" never matches "
                f"one and this tool would silently report every one of that task's answer nodes as "
                f"'not retrieved'"
            )
        if answer_nodes:
            # dict.fromkeys dedupes while keeping first-occurrence order -- the same fix --only
            # already applies below. Left undeduped, a repeated id prints twice in answer_node_report
            # and inflates both halves of the found/len(hits) ratio print_table shows (W3).
            row["answerNodes"] = list(dict.fromkeys(answer_nodes))

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
        selected = rows
    else:
        # dict.fromkeys dedupes while keeping first-occurrence order: "--only t1,t1" must run t1
        # once, not twice, or both the in-file duplicate-task check above and the graph check
        # below are defeated by a repeated id rather than a repeated file row (DiVoid #11141).
        wanted = list(dict.fromkeys(name.strip() for name in only.split(",") if name.strip()))
        by_id = {row["id"]: row for row in rows}
        missing = [name for name in wanted if name not in by_id]
        if missing:
            raise CompareFailure(
                f"--only names {', '.join(missing)}, which {path} does not define. It defines "
                f"{', '.join(by_id)}"
            )
        selected = [by_id[name] for name in wanted]

    warn_self_referential(selected)
    return selected


def warn_self_referential(tasks):
    """DiVoid #11333 S4: a task whose own subject appears in its answerNodes has bought part of
    its answer for free. That node is this run's anchor -- it reaches the model unconditionally,
    before any candidate and with no budget check -- so route() (below) excludes it from the
    task's retrieval-eligible population; for that node the task tests nothing about retrieval.

    A warning, not a refusal: the run this produces is still an honest arm comparison (arm
    TRANSCRIPT is unaffected either way, and the anchor is real product behaviour, not an
    instrument artefact), and refusing outright would break continuity with the runs DiVoid
    #11319 already records. Revising the task set so no task's subject is among its own answer
    nodes is separate, future work -- not this script's job.
    """
    offenders = [t["id"] for t in tasks if t["subject"] in (t.get("answerNodes") or [])]
    if offenders:
        singular = len(offenders) == 1
        noun = "task" if singular else "tasks"
        verb = "names" if singular else "name"
        possessive = "its" if singular else "their"
        print(
            f"WARN: {noun} {', '.join(offenders)} {verb} {possessive} own subject as an answer node. "
            f"That node is this run's anchor: it reaches the model unconditionally, so it is "
            f"excluded from that task's retrieval-eligible population (route anchor, not a "
            f"retrieval stage -- see route() in this file, DiVoid #11333). This is a warning, "
            f"not a refusal; the comparison still runs."
        )


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


def graph_credentials():
    url = os.environ.get("PROCESSOR_DIVOID_URL") or os.environ.get("DIVOID_URL")
    key = os.environ.get("PROCESSOR_DIVOID_KEY") or os.environ.get("DIVOID_RAZIEL_KEY")
    if not url or not key:
        raise CompareFailure(
            "no graph credential, and arm SUBSTRATE is the graph. The binary reads "
            "PROCESSOR_DIVOID_URL and PROCESSOR_DIVOID_KEY; this machine supplies the same credential "
            "as DIVOID_URL and DIVOID_RAZIEL_KEY and this script maps one onto the other. Neither "
            "pair is set, so set DIVOID_URL and DIVOID_RAZIEL_KEY (or the PROCESSOR_ names directly)"
        )
    return url.rstrip("/"), key


def find_prior_run(divoid_url, divoid_key, task_text):
    """Return (nodeId, name) of an existing run record whose input is task_text verbatim, or None.

    This queries GET /api/nodes?query=... -- the same semantic-recall endpoint
    internal/divoid/client.go's Recall uses for retrieval -- rather than any kind of exact index
    lookup, because the graph exposes no such lookup to this script. A record embedding task_text
    verbatim normally ranks at or near the top of a recall for that same text, but this is still a
    ranked search: a match outside GRAPH_QUERY_COUNT rows is missed. That is a real check, not a
    guarantee.
    """
    query = urllib.parse.urlencode({
        "query": task_text,
        "count": str(GRAPH_QUERY_COUNT),
        "fields": "id,type,name,content",
    })
    request = urllib.request.Request(
        f"{divoid_url}/api/nodes?{query}",
        headers={"Authorization": f"Bearer {divoid_key}", "Accept": "application/json"},
        method="GET",
    )
    try:
        with urllib.request.urlopen(request, timeout=GRAPH_QUERY_TIMEOUT_S) as response:
            payload = response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", "replace").strip()
        raise CompareFailure(
            f"GET {divoid_url}/api/nodes returned {err.code} while checking for a prior run of this "
            f"task: {detail}"
        ) from err
    except (urllib.error.URLError, OSError, http.client.HTTPException) as err:
        raise CompareFailure(
            f"GET {divoid_url}/api/nodes did not complete while checking for a prior run of this "
            f"task, and arm SUBSTRATE needs the same graph regardless: {type(err).__name__}: {err}"
        ) from err

    try:
        wire = json.loads(payload)
    except json.JSONDecodeError as err:
        raise CompareFailure(
            f"{divoid_url}/api/nodes returned {len(payload)} bytes that are not JSON ({err}) while "
            f"checking for a prior run of this task"
        ) from err

    for row in wire.get("result") or []:
        if row.get("type") != RUN_NODE_TYPE or not str(row.get("name", "")).startswith(RUN_NAME_PREFIX):
            continue
        try:
            record = json.loads(row.get("content") or "")
        except json.JSONDecodeError:
            continue
        if record.get("input") == task_text:
            return row.get("id"), row.get("name")
    return None


def refuse_repeats_against_graph(tasks, divoid_url, divoid_key):
    for task in tasks:
        prior = find_prior_run(divoid_url, divoid_key, task["task"])
        if prior is not None:
            node_id, name = prior
            raise CompareFailure(
                f"task {task['id']} was already run: graph node #{node_id} ({name!r}) carries a run "
                f"record whose input is this task's text verbatim. Refused rather than run again: the "
                f"substrate arm's retrieval would read that record back (DiVoid #11141), so a second "
                f"run of the same text is not a second measurement of anything. This is a semantic "
                f"recall check, not an index lookup (see find_prior_run's docstring), and the loader's "
                f"in-file duplicate check does not reach this case -- it only compares rows in the "
                f"task set file against each other, not against the graph"
            )


def child_env(model_url, model_id, model_key, divoid_url, divoid_key):
    env = os.environ.copy()
    env["PROCESSOR_DIVOID_URL"] = divoid_url
    env["PROCESSOR_DIVOID_KEY"] = divoid_key
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
    """Count of candidates the *initial* recall round admitted. Paired everywhere it is printed with
    len(candidates(record)), which is also initial-round-only -- this is deliberately not the total
    admitted across the run; see total_admitted for that."""
    return sum(1 for c in candidates(record) if c.get("included"))


def total_admitted(record):
    """Count of distinct nodes admitted anywhere in the run -- the initial block or any supplementary
    recall round. Use this, not admitted(), for any question of the shape "did the model receive
    anything" (e.g. starvation): a node the initial round cut and a later supplementary round
    recovered was still received (N2, DiVoid #11141 follow-up)."""
    return len(admitted_ids(record))


def admitted_ids(record):
    ids = {c.get("id") for c in candidates(record) if c.get("included")}
    ids.update(node_id for node_id, row in _supplementary_by_id(record).items() if row.get("included"))
    ids.discard(None)
    return ids


def block_bytes(record):
    return len(record.get("block", "").encode("utf-8"))


def answer_bytes(text):
    return len((text or "").encode("utf-8"))


def one_line(value, width):
    text = " ".join(str(value).split())
    return text if len(text) <= width else text[: width - 1] + "…"


class Stage(str):
    """A task/answer-node stage name that carries its own indictment text.

    Subclassing str keeps every existing use working unchanged: dict keys, f"{stage}",
    f"{stage!r}" (str.__repr__ renders the string content, not the subclass name), equality
    against a plain disposition value, and `stage in (...)` membership checks. What changes is
    that the narration is now an attribute of the stage value itself (`stage.indictment`)
    instead of a second table (formerly a module-level INDICTMENT dict) keyed by the same
    string a few dozen lines away from the branch that picks it. Every defect raised across
    this script's review rounds was exactly that: a branch condition and its narration edited
    in different places until they disagreed. Collapsing the two into one object removes the
    second place -- there is nothing left to edit apart.
    """

    def __new__(cls, label, indictment):
        self = super().__new__(cls, label)
        self.indictment = indictment
        return self

    def __getnewargs__(self):
        # str's default pickle/copy protocol reconstructs an instance via __new__(cls, str(self))
        # alone -- one positional argument -- and Stage.__new__ requires two. Without this, both
        # copy.copy/deepcopy and pickle.loads raise TypeError on any Stage value. Not reachable
        # through the CLI today (nothing here copies or pickles a Stage), but it is the latent
        # edge: the moment something serializes a run's stage tally, this is what fires.
        return (str(self), self.indictment)


NOT_RETRIEVED = Stage(
    "not retrieved",
    "retrieval never returned the answer at all -- neither the initial recall query nor any "
    "supplementary recall the model asked for. This indicts the recall query, not the budget",
)

RETRIEVED_CUT = Stage(
    "retrieved, cut",
    "the initial recall query found the answer and assembly cut it there before the block was "
    "sent -- the byte budget or the self-produced check; the per-node detail below names which "
    "-- and no supplementary round on this task recovered it afterward",
)

RETRIEVED_CUT_SUPPLEMENTARY = Stage(
    "retrieved, cut (supplementary)",
    "the initial recall query never returned the answer at all; a supplementary round did, and "
    "assembly cut it there too -- the byte budget or the self-produced check; the per-node "
    "detail below names which. This is not the same cut as RETRIEVED_CUT: the budget in play "
    "here is the supplementary round's own SupplementaryByteBudget, a different number from the "
    "initial round's assemblyByteBudget that describe_cut reports against, and the candidates "
    "this was ranked among are the supplementary round's own results, not the initial round's",
)

ADMITTED_PRIMARY = Stage(
    "admitted, primary",
    "the answer reached the model in the initial context block. Anything wrong past here is the "
    "prompt, the model, or the node's own content, and only reading both answers separates those "
    "three",
)

ADMITTED_SUPPLEMENTARY = Stage(
    "admitted, supplementary",
    "the answer is not in the initial candidate list at all -- assembly was never given the "
    "chance to admit or cut it, because it never got that far. That can mean the initial recall "
    "query did not return it, or that it did and the fixed candidate limit truncated it before "
    "assembly ever saw it; this record cannot tell those two apart. It reached the model only "
    "because the model asked for supplementary recall and that second, unscoped query found it. "
    "This indicts the initial recall query or the candidate limit, not the byte budget: "
    "assembly was never in a position to cut it",
)

ADMITTED_SUPPLEMENTARY_RECOVERED = Stage(
    "admitted, supplementary (recovered after cut)",
    "the initial recall query DID return the answer -- it is in the initial candidate list, at "
    "the rank printed on the answer-node line below -- and assembly cut it there before the "
    "block was sent (the byte budget or the self-produced check; the per-node detail below "
    "names which -- never the candidate limit, which is already reflected in that list by the "
    "time assembly sees it, so a node present in it was not truncated). The model then asked "
    "for supplementary recall and a second, unscoped query re-admitted it. This indicts "
    "assembly's admission at the initial round, not the recall query: the recall query worked",
)

UNLABELLED = Stage(
    "no answer nodes named", "the task set names no answering node, so nothing mechanical can be attributed"
)

ANCHOR_ONLY = Stage(
    "anchor only",
    "every answer node this task names is its own subject, so the run sent them all "
    "unconditionally and this task measures nothing about retrieval. This is constant by "
    "construction -- it reports a property of the task set, not of this run -- which is why it "
    "is a distinct task-level value rather than a seventh retrieval stage (DiVoid #11333 S1)",
)

# Route, not stage (DiVoid #11333): the instrument has one axis -- how far did this node get? --
# and a node that never entered the race does not belong on it. Every named answer node is on
# exactly one of these two routes; route() below is the one comparison that decides which, and
# nothing about it can fail. A node on ROUTE_ANCHOR is not ranked against the six Stage values
# above -- it is removed from the population they describe. found() already applied that rule;
# task_stage's precedence and print_table's nodes ratio did not, and both were wrong the same way.
ROUTE_ANCHOR = "anchor"
ROUTE_RETRIEVAL = "retrieval"

ANCHOR_ROUTE_NOTE = (
    "the anchor route: internal/loop/assemble.go's renderBlock writes the run's anchor and its "
    "full content into the block unconditionally, before any candidate and with no budget check, "
    "so an answer node on this route reached the model regardless of what recall returned or "
    "what assembly admitted or cut. This is not a retrieval finding -- it says nothing about the "
    "recall query, the candidate limit, or the byte budget -- and it is removed from the "
    "population the stage above describes rather than ranked within it: a task naming its own "
    "subject as an answer node is not a fair test of retrieval for that node, whatever "
    "candidates(record) or the supplementary rounds separately say about the same id"
)


def _supplementary_by_id(record):
    """Union every disposition any supplementary-recall round returned, keyed by node id.

    A node can appear in more than one round; an included disposition wins over a cut one for the
    same id, since 'was it ever admitted' is what the stage cares about.
    """
    by_id = {}
    for call in record.get("toolCalls") or []:
        for row in call.get("results") or []:
            node_id = row.get("id")
            existing = by_id.get(node_id)
            if existing is None or (not existing.get("included") and row.get("included")):
                by_id[node_id] = row
    return by_id


def route(node, record):
    """`ROUTE_ANCHOR` if `node` is this run's own anchor id, else `ROUTE_RETRIEVAL`.

    One comparison, `node == record.anchor.id`; nothing about it can fail. This is the entire
    axis split DiVoid #11333 turns on: arrival route (did this node enter the race at all?) and
    retrieval stage (how far did it get, given that it did?) are two different questions, and a
    node on the anchor route is not a candidate answer to the second one.
    """
    anchor_id = (record.get("anchor") or {}).get("id")
    return ROUTE_ANCHOR if anchor_id is not None and node == anchor_id else ROUTE_RETRIEVAL


def answer_node_report(task, record):
    """One (node, route, stage, rank, detail) row per answer node the task set names.

    `route` comes from route() and is checked first, short-circuiting everything else: an answer
    node on ROUTE_ANCHOR reached the model regardless of what candidates(record) or a
    supplementary round separately say about that same id, so `stage` is None for it -- it is not
    a retrieval outcome of any kind, admitted or otherwise. Checking candidates/supplementary
    first for such a node would report it as NOT_RETRIEVED, RETRIEVED_CUT, or even admitted-by-luck
    depending on whether scoped recall also happened to return it (F1) -- every one of those
    readings is a retrieval verdict about a node that never went through retrieval to reach the
    model. This is deliberately re-checked even when the node is ALSO an admitted candidate, a cut
    candidate, or a supplementary hit: route wins regardless, because arrival and stage are
    different axes and a node's presence on one says nothing about its place on the other
    (DiVoid #11333).

    For a ROUTE_RETRIEVAL row, `rank` is the rank at the round the stage is reported from (the
    initial round for ADMITTED_PRIMARY and a cut reported as RETRIEVED_CUT, the supplementary
    round for a supplementary stage or RETRIEVED_CUT_SUPPLEMENTARY). `detail` carries the fact the
    top-line stage can't: which round's candidates a cut happened in and why, or -- for
    ADMITTED_SUPPLEMENTARY_RECOVERED -- the initial round's own rank and cutReason, the fact that
    makes the initial query's success and the subsequent cut both real.

    The three-way split on "supplementary is included" exists because that is not one outcome: a
    node absent from the initial candidate list entirely (`primary is None`) means the initial
    recall query never returned it in a form assembly ever saw -- that indicts the query or the
    candidate limit, never the byte budget. A node present in the initial candidate list but not
    included (`primary is not None`) means the initial query found it and assembly cut it -- that
    indicts assembly's admission (byte budget or self-produced), never the candidate limit, since
    survival into candidates(record) already proves the candidate limit did not cut it (N1, DiVoid
    #11141 follow-up; the limit-vs-budget attribution corrected here is F2).

    The same split applies to the two ways a node is retrieved-but-never-admitted: found and cut at
    the initial round (RETRIEVED_CUT) is not the same finding as never found initially but found and
    cut at a supplementary round (RETRIEVED_CUT_SUPPLEMENTARY) -- the latter went through a
    different budget (SupplementaryByteBudget, not assemblyByteBudget) and a different candidate
    list. Collapsing either pair reads one outcome as the other (W1, alongside the original N1).
    """
    wanted = task.get("answerNodes") or []
    if not wanted:
        return []
    primary_by_id = {c.get("id"): c for c in candidates(record)}
    supplementary_by_id = _supplementary_by_id(record)

    rows = []
    for node in wanted:
        if route(node, record) == ROUTE_ANCHOR:
            rows.append((node, ROUTE_ANCHOR, None, None, None))
            continue

        primary = primary_by_id.get(node)
        supplementary = supplementary_by_id.get(node)
        primary_included = bool(primary is not None and primary.get("included"))
        supplementary_included = bool(supplementary is not None and supplementary.get("included"))

        if primary_included:
            rows.append((node, ROUTE_RETRIEVAL, ADMITTED_PRIMARY, primary.get("rank"), None))
        elif supplementary_included and primary is not None:
            reason = primary.get("cutReason") or "no reason given"
            detail = f"initial recall ranked it #{primary.get('rank')} but assembly cut it ({reason})"
            rows.append(
                (node, ROUTE_RETRIEVAL, ADMITTED_SUPPLEMENTARY_RECOVERED, supplementary.get("rank"), detail)
            )
        elif supplementary_included:
            rows.append((node, ROUTE_RETRIEVAL, ADMITTED_SUPPLEMENTARY, supplementary.get("rank"), None))
        elif primary is not None:
            reason = primary.get("cutReason") or "no reason given"
            rows.append(
                (node, ROUTE_RETRIEVAL, RETRIEVED_CUT, primary.get("rank"), f"cut at the initial round: {reason}")
            )
        elif supplementary is not None:
            reason = supplementary.get("cutReason") or "no reason given"
            rows.append(
                (
                    node,
                    ROUTE_RETRIEVAL,
                    RETRIEVED_CUT_SUPPLEMENTARY,
                    supplementary.get("rank"),
                    f"cut at a supplementary round: {reason}",
                )
            )
        else:
            rows.append((node, ROUTE_RETRIEVAL, NOT_RETRIEVED, None, None))
    return rows


ADMITTED_STAGES = (ADMITTED_PRIMARY, ADMITTED_SUPPLEMENTARY, ADMITTED_SUPPLEMENTARY_RECOVERED)


def arrived(row):
    """True if this answer-node row reached the model at all -- by the anchor route, or by any of
    the three admitted retrieval stages. Gates the two-line split in DiVoid #11333 S2: `any(...)`
    over a task's rows for the arrival line ("at least one named node reached the model, worth
    reading the answers for"), `all(...)` for the completion line ("nothing mechanical is left
    unreported"). A row on RETRIEVED_CUT, RETRIEVED_CUT_SUPPLEMENTARY, or NOT_RETRIEVED did not
    arrive -- those are exactly the stages that mean the node never reached the model."""
    _, node_route, stage, _, _ = row
    return node_route == ROUTE_ANCHOR or stage in ADMITTED_STAGES


def node_ratio(rows):
    """(found, eligible) over the retrieval-eligible rows only, or None if that population is
    empty. `eligible` counts every row on ROUTE_RETRIEVAL, admitted or not; `found` counts those
    with an admitted stage. Anchor rows leave both halves -- DiVoid #11333 S3: an anchor was never
    in a position for retrieval to succeed or fail on it, so it is not a numerator any more than a
    denominator. An empty retrieval-eligible population (a task naming only its own anchor(s), or
    naming no answer nodes at all) has no ratio to report; callers must print that as a dash, never
    as 0/0 or 0/1."""
    eligible = [r for r in rows if r[1] == ROUTE_RETRIEVAL]
    if not eligible:
        return None
    found = sum(1 for r in eligible if r[2] in ADMITTED_STAGES)
    return found, len(eligible)


def anchor_arrivals(rows):
    """Count of named answer nodes that are the run's own anchor -- DiVoid #11333 S3's narrow
    column, printed beside the nodes ratio rather than folded into either half of it."""
    return sum(1 for r in rows if r[1] == ROUTE_ANCHOR)


# Best-first precedence for task_stage: the earliest entry found among a task's *retrieval-eligible*
# answer-node dispositions wins -- an anchor row carries no stage at all (route() removed it from
# this population entirely, DiVoid #11333) and so cannot appear here or win anything.
# ADMITTED_SUPPLEMENTARY_RECOVERED outranks the genuine-miss ADMITTED_SUPPLEMENTARY: a node the
# initial query found and the budget cut, later recovered, is a working query plus a tight budget;
# a node the initial query never found at all is a query miss papered over by a second, unscoped
# round -- the first is the milder finding. The same reasoning orders RETRIEVED_CUT (initial query
# found it) ahead of RETRIEVED_CUT_SUPPLEMENTARY (initial query never found it; only a
# supplementary round did, and that too was cut).
STAGE_PRECEDENCE = (
    ADMITTED_PRIMARY,
    ADMITTED_SUPPLEMENTARY_RECOVERED,
    ADMITTED_SUPPLEMENTARY,
    RETRIEVED_CUT,
    RETRIEVED_CUT_SUPPLEMENTARY,
    NOT_RETRIEVED,
)


def task_stage(task, record):
    """The furthest point any of the task's retrieval-eligible answer nodes reached; see
    STAGE_PRECEDENCE for order. Anchor rows are excluded from the population this precedence runs
    over (route(), not stage) -- a task whose answer nodes are ALL anchors therefore has no
    retrieval-eligible row at all and reports ANCHOR_ONLY instead, a task-set property rather than
    a run outcome (DiVoid #11333 S1)."""
    rows = answer_node_report(task, record)
    if not rows:
        return UNLABELLED
    eligible = [row for row in rows if row[1] == ROUTE_RETRIEVAL]
    if not eligible:
        return ANCHOR_ONLY
    dispositions = {stage for _, _, stage, _, _ in eligible}
    for stage in STAGE_PRECEDENCE:
        if stage in dispositions:
            return stage
    return NOT_RETRIEVED  # unreachable: STAGE_PRECEDENCE ends in NOT_RETRIEVED, the report's only other value


def describe_cut(row, budget):
    """One line describing why a non-admitted candidate was cut, from its own cutReason -- never an
    inferred mechanism. internal/loop/assemble.go defines exactly two: self-produced and byte budget
    exceeded, and the latter covers both an individually oversized row and a cumulative overflow."""
    reason = row.get("cutReason") or "no reason given"
    size = row.get("size", 0)

    if reason == CUT_SELF_PRODUCED:
        return (
            f"CUT #{row.get('id')}: self-produced -- a run record this system wrote earlier; "
            f"assembly refuses it before the byte budget is even consulted"
        )
    if reason == CUT_BYTE_BUDGET:
        if budget is None:
            return (
                f"CUT #{row.get('id')}: {size} bytes, cut for byte budget exceeded -- but this "
                f"record carries no assemblyByteBudget, so whether it was oversized alone or only "
                f"in combination with rows admitted ahead of it cannot be said from here"
            )
        if size > budget:
            return (
                f"CUT #{row.get('id')}: {size} bytes alone exceeds the {budget}-byte assembly "
                f"budget, so no run can admit it regardless of rank"
            )
        return (
            f"CUT #{row.get('id')}: {size} bytes fits the budget alone but not after the "
            f"candidates admitted ahead of it -- a cumulative-budget cut. Admission skips a row "
            f"that does not fit and keeps checking the ones behind it, so this does not stop "
            f"anything ranked later from being admitted"
        )
    return f"CUT #{row.get('id')}: {reason}"


def print_displacement(rows, label, limit):
    """Report retrieval-slot displacement for one recall round's result rows.

    dispatchRecall calls Recall with the same fixed CandidateLimit for a supplementary round as for
    the initial one, and assembly cuts a self-produced row there exactly the same way -- so a
    supplementary round can suffer the same tail displacement as the initial round, silently, if
    this is only ever checked on the initial round's candidates (N3, DiVoid #11141 follow-up).
    Called once per round; prints nothing when that round admits no self-produced rows.
    """
    mine = [c for c in rows if c.get("cutReason") == CUT_SELF_PRODUCED]
    if not mine:
        return
    ids = ", ".join(f"#{c['id']}" for c in mine)
    limit_note = f"the top {limit}" if limit else "the fixed number of"
    noun = "is a run record" if len(mine) == 1 else "are run records"
    cuts = "cuts it" if len(mine) == 1 else "cuts them"
    slot = "a slot" if len(mine) == 1 else "slots"
    print(
        f"           RETRIEVAL-SLOT DISPLACEMENT ({label}): {ids} {noun} this system wrote "
        f"earlier that recall ranked into {limit_note} candidates it returns. Assembly always "
        f"{cuts} as self-produced -- that check runs before the byte budget, so the product "
        f"never reads its own history back -- but occupying {slot} in a fixed-size recall list "
        f"means a real candidate that would otherwise have been retrieved fell off the tail "
        f"instead. That is the defect, not a self-read."
    )


def print_task(task, record, transcript):
    print()
    print(RULE)
    print(f"TASK {task['id']}   subject #{task['subject']}")
    print(RULE)
    print(task["task"])

    why = task.get("whyMemoryIsRequired")
    if why:
        print()
        print("WHY MEMORY IS REQUIRED (this task's hypothesis, from the task set):")
        print(textwrap.fill(str(why), width=88))

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

    if record.get("capReached"):
        print()
        print(
            "CAP REACHED: the model still wanted supplementary recall when the model-call cap "
            "stopped it. The substrate answer below is not the model's last word -- it is what the "
            "model had produced when it was cut off still asking for more context."
        )

    budget = (record.get("limits") or {}).get("assemblyByteBudget")
    for row in candidates(record):
        if not row.get("included"):
            print(f"           {describe_cut(row, budget)}")

    limit = (record.get("limits") or {}).get("candidateLimit")
    print_displacement(candidates(record), "initial recall", limit)
    for index, call in enumerate(record.get("toolCalls") or [], 1):
        print_displacement(call.get("results") or [], f"supplementary round {index}", limit)

    rows = answer_node_report(task, record)
    stage = task_stage(task, record)
    print()
    print(f"STAGE: {stage} -- {stage.indictment}")
    for node, node_route, node_stage, rank, detail in rows:
        if node_route == ROUTE_ANCHOR:
            print(f"       answer node #{node}: anchor route -- matches anchor #{anchor.get('id')} above")
            continue
        at = f" at rank {rank}" if rank is not None else ""
        line = f"       answer node #{node}: {node_stage}{at}"
        if detail:
            line += f" -- {detail}"
        print(line)
    if any(node_route == ROUTE_ANCHOR for _, node_route, _, _, _ in rows):
        print(f"       {ANCHOR_ROUTE_NOTE}")

    # DiVoid #11333 S2: the old single "attribution stops here" sentence asserted two different
    # things gated on neither -- that SOMETHING arrived (worth reading the answers for) and that
    # NOTHING mechanical is left unreported (no answer node still sitting at not-retrieved). Those
    # are an any() and an all() over the same rows, and a task can satisfy one without the other
    # (the t4 shape: one node is the anchor, the other was never retrieved -- arrival fires,
    # completion does not).
    if rows and any(arrived(row) for row in rows):
        print(
            "       At least one named answer node reached the model (see above for which, and "
            "by which route), so the answer pair below is worth reading."
        )
    if rows and all(arrived(row) for row in rows):
        if all(node_route == ROUTE_ANCHOR for _, node_route, _, _, _ in rows):
            # ANCHOR_ONLY shape (no shipped task today, but latent): every named node arrived by
            # the anchor route, none by retrieval, so there is no admitted node for the usual
            # completion text to point at -- printing it unhedged would claim retrieval was
            # tested here when it never was.
            print(
                "       The mechanical attribution stops here for every named node, but none of "
                "them was admitted by retrieval -- each is this task's own anchor, so it reached "
                "the model unconditionally rather than being admitted. Whether the substrate "
                "answer draws on it, and whether it beats the transcript answer anyway, is still "
                "the reader's call on the two texts below, but retrieval itself was never tested "
                "by this task."
            )
        else:
            print(
                "       The mechanical attribution stops here. Whether the substrate answer actually "
                "draws on the node, and whether it beats the transcript answer anyway, is the reader's "
                "call on the two texts below -- an admitted node that the answer ignores indicts the "
                "prompt, and one the answer uses and still loses on indicts the node's content."
            )

    substrate_stop = record.get("stopReason") or {}
    print()
    print(
        f"SUBSTRATE ANSWER, verbatim (stopReason={substrate_stop.get('reason')!r}, "
        f"raw={substrate_stop.get('raw')!r}):"
    )
    print(THIN)
    print(record.get("answer", ""))
    print(THIN)

    print()
    print(
        f"TRANSCRIPT ANSWER, verbatim (same model, same text, no context block, no tool; "
        f"finishReason={transcript.get('finishReason')!r}):"
    )
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
        f"  {'task':<27} {'admit':>7} {'block B':>8} {'calls':>5} {'nodes':>6} {'anchor':>6} "
        f"{'subst B':>8} {'trans B':>8}  stage"
    )
    for task, record, transcript in rows:
        hits = answer_node_report(task, record)
        ratio = node_ratio(hits)
        nodes_cell = f"{ratio[0]}/{ratio[1]}" if ratio is not None else "-"
        anchor_count = anchor_arrivals(hits)
        anchor_cell = str(anchor_count) if anchor_count else "-"
        print(
            f"  {one_line(task['id'], 27):<27} "
            f"{admitted(record):>3}/{len(candidates(record)):<3} "
            f"{block_bytes(record):>8} {record.get('modelCalls'):>5} "
            f"{nodes_cell:>6} {anchor_cell:>6} "
            f"{answer_bytes(record.get('answer')):>8} {answer_bytes(transcript['answer']):>8}  "
            f"{task_stage(task, record)}"
        )

    tally = {}
    for task, record, _ in rows:
        stage = task_stage(task, record)
        tally[stage] = tally.get(stage, 0) + 1
    print()
    for stage, count in sorted(tally.items(), key=lambda kv: -kv[1]):
        print(f"  {count} task(s) reached stage {stage!r}: {stage.indictment}")
    print()
    print(
        "  admit = candidates admitted of candidates returned; nodes = admitted / retrieval-eligible "
        "answer nodes named by the task set -- an answer node that is the task's own anchor leaves "
        "BOTH halves of this ratio, since it was never in a position for retrieval to succeed or "
        "fail on it (route, not stage: DiVoid #11333); '-' means no retrieval-eligible answer node "
        "was named at all (never printed as 0/0 or 0/1); anchor = how many of the task's named "
        "answer nodes are its own anchor ('-' means none); subst B / trans B = answer sizes in "
        "bytes. Answer size is a length, not a quality, and the stage says where the answer got to, "
        f"never whether it is right (except stages {ANCHOR_ONLY!r} and {UNLABELLED!r}, which "
        f"report a property of the task set rather than a run outcome). Read the two answers "
        f"above."
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
        "       Nothing was deleted. Each record embeds its task text verbatim, so a later query -- "
        "a repeat of this text, or an unrelated task whose vocabulary overlaps -- can rank it among "
        "the top rows recall returns. Assembly always cuts it (cutReason 'self-produced', checked "
        "before the byte budget), but recall returns a fixed number of rows, so a self-produced row "
        "occupying one of them displaces a real row that would otherwise have been retrieved -- "
        "retrieval-slot displacement, not an admission effect (DiVoid #11141, #11133). Leaving them "
        "in place changes what the next invocation of this tool retrieves and can move a corpus "
        "row's retrieval in the baseline sweep. Delete them by hand."
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
        divoid_url, divoid_key = graph_credentials()
        refuse_repeats_against_graph(tasks, divoid_url, divoid_key)
        model_url, model_id, model_key = model_config(args)
        env = child_env(model_url, model_id, model_key, divoid_url, divoid_key)
        probe_model(model_url, model_id, model_key)
    except CompareFailure as err:
        print(f"FAIL: {err}")
        return 2
    except Exception as err:
        traceback.print_exc(file=sys.stderr)
        print(f"FAIL: the comparison could not be made: {type(err).__name__}: {err}")
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

            # Union of initial-round and supplementary-round admissions (total_admitted), not
            # admitted() alone: admitted() only counts the initial round, so a task where the
            # initial round admitted nothing but a supplementary round recovered a node would be
            # misreported here as starved -- the model did receive that node, through a tool round
            # (N2, DiVoid #11141 follow-up).
            starved = [(t, r) for t, r, _ in rows if not total_admitted(r)]
            if starved:
                print()
                for task, record in starved:
                    print(
                        f"FAIL: task {task['id']} admitted nothing: 0 of "
                        f"{len(candidates(record))} initial candidates, and no supplementary round "
                        f"admitted anything either. Arm SUBSTRATE was sent the anchor and nothing "
                        f"else, so for this task the two arms differ by one node rather than by a "
                        f"memory substrate, and the comparison above is not one."
                    )
                return 1

            print()
            print(
                f"DONE: every task ran both arms. No verdict is printed and none is available from "
                f"this instrument -- read the answer pairs, and read each one against the stage above "
                f"it, which says how far the answer to that task got before the model saw anything "
                f"-- except stages {ANCHOR_ONLY!r} and {UNLABELLED!r}, which report a property of "
                f"the task set rather than of this run."
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
        except Exception as err:
            print()
            traceback.print_exc(file=sys.stderr)
            print(
                f"FAIL: the comparison could not be made: {type(err).__name__}: {err}. This is not "
                f"one of the failures this script names on purpose -- see the traceback on stderr."
            )
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
