#!/usr/bin/env python3
"""Run ONE task through the built binary and print an ordered, numbered step trace of what the
turn actually did -- not a summary, a reconstruction, in order, of every step the record lets us
see, each with its own input and its own output.

    python scripts/step_trace.py --input "Generate a new barebones webpage and a repo for it." --subject 10422
    python scripts/step_trace.py --input "..." --subject 10850 --model-url http://gangolf:12434/engines/v1 --model-id ai/qwen3-coder

(Named step_trace.py, not trace.py: this file's directory is prepended to sys.path so it can import
compare.py, and a sibling module literally named trace.py would shadow the standard library's own
trace module process-wide for anything imported afterward -- W3 on this file's own review.)

Why this exists, verbatim from the operator (DiVoid task, 2026-09-05): this project has spent a
long day measuring components -- retrieval rates, admission budgets, compression ratios -- and has
never watched the loop attempt a task end to end. THE DELIVERABLE IS VISIBILITY, NOT IMPROVEMENT.
If a run does something useless, this script's job is to show that clearly, not to soften it.

What "one turn" actually is (internal/loop/turn.go, DiVoid #10850 / #10846), verified against this
tree rather than assumed: fetch the anchor by id -> retrieve candidates (Retrieve, DiVoid #11259 --
a two-list reciprocal-rank fusion of an unscoped recall plus a recall scoped to the anchor's two-hop
neighbourhood, NOT a single similarity-sorted query) -> assemble a byte-budgeted block -> call the
model, looping while it asks for supplementary recall, bounded by MaxModelCalls=3 -> write a run
record. The model's only tool is that supplementary "recall"; there is no file, shell, network or
repo tool, and no notion of a task spanning more than one HTTP call. A task like "generate a webpage
and a repo" has no mechanism to succeed here by construction -- that is expected, and is not what
this script exists to show. What it exists to show is HOW it fails: refuses, answers about the task,
asks for recall, claims completion, or produces something confidently wrong.

One POST /runs is one turn and returns one JSON record (internal/loop/types.go's Record, wrapped in
{ ...Record fields, "written": {state, nodeId} } by internal/server/routes.go). Every "step" below is
reconstructed from that one record after the fact -- there is no intermediate progress feed -- so the
ordering is inferred from the record's own structure (usage is one entry per model call, in call
order; toolCalls is one entry per call that asked for recall, in call order, including a call that
never reached the graph at all -- see the tool-round classification note below) rather than observed
live. Anywhere that inference could be wrong, the trace says so rather than guessing quietly.

TOOL-ROUND CLASSIFICATION, corrected after review (C1): a toolCalls[i] entry with a non-empty
`error` is not one thing. `internal/loop/turn.go`'s dispatchRecall returns *before ever calling
`Graph.Recall`* when the model's own tool call was malformed (wire.go's RecallError -- unparseable
arguments or an empty query, itself ordinary local-model misbehaviour, not rare) -- so that round
never touched the graph. This script distinguishes three shapes by the exact `error` string:
`""` is a real dispatch with real results (a tool-call step follows); the literal "call cap reached"
is a round the call cap stopped before dispatch could happen; the literal "supplementary recall
failed" is a dispatch that reached the graph and the graph call itself errored (turn.go's own
scrubbing rule keeps the real reason out of this surface, DiVoid #10850). Any OTHER non-empty error
string -- including one this script does not otherwise recognise -- means the request was malformed
and, per dispatchRecall's own control flow, GUARANTEED never dispatched: no tool-call step is
printed for it, the record's error string is shown verbatim, and no query is available (turn.go's
RecallExchange construction on this path never sets Query at all). Printing a fabricated tool-call
step here was the exact defect a reviewer caught: it read as "the model asked the graph and the
graph had nothing" when the truth was "the model's tool call never reached the graph".

BUDGET ARITHMETIC, corrected after review (C2/C3): the anchor is not exempt from the assembly
budget. `internal/loop/assemble.go`: `remaining := budget - len(anchor.Content)`, floored at zero --
the anchor's bytes are charged against the budget, in full, before any candidate is even considered.
What the anchor IS exempt from is being CUT: its full content always reaches the model regardless of
size (design correction pinned in this tree at `m1-skeleton-loop.md:765`: "the anchor is exempt from
being cut, not from being charged"). This script computes and prints the actual per-run candidate
budget (`assemblyByteBudget - anchor.size`, floored at zero) and tests admissibility against THAT
number, not the raw constant -- a candidate that fits the constant but not the anchor-adjusted
remainder is unadmittable for this run and previously got no marker at all (C3). The supplementary
round is unaffected by this: `dispatchRecall` passes `SupplementaryByteBudget` to `admit` as-is, with
no anchor subtraction, because the anchor is not re-sent on that path.

WHAT THE RECORD CANNOT TELL A READER -- found while building this, not fixed (constraint: no Go
file changes):
  - No per-step wall-clock or timestamp. turn.go computes one elapsed duration for the whole run and
    logs it to stderr; the record itself carries no timing at all, so this trace cannot show how long
    retrieval took versus how long the model took.
  - No raw content for the anchor or for any candidate, admitted or cut. AnchorSummary and
    Disposition carry only {id, type, name, size, contentHash} -- a hash, not the bytes. The ONLY
    place actual node content survives in the record is the assembled Block string (for whatever was
    admitted) and the final Answer. A cut candidate's content is gone from the record forever, which
    means a reader can never audit what was cut, only that it was and why.
  - No system prompt text. The record has no System field, so the exact bytes sent to the model on
    calls after the first (which also replay prior tool rounds as synthetic assistant/tool messages,
    internal/openaicompat/wire.go's buildMessages) are not reconstructable from the record alone --
    only the token counts (Usage) are. This script does not reach past the record for it.
  - On a malformed tool round that also happens to be the final call, the record cannot say whether
    the model-call cap would ALSO have stopped it: turn.go's construction on that shared branch lets
    the malformed-request reason win outright rather than recording both, so `capReached` can be true
    on a run whose last round shows a plain malformed-tool-call error instead of the cap's own
    literal. This script flags that ambiguity inline when it can detect the shape (see below) rather
    than guessing which one actually fired.
  - Temperature is not configurable. internal/openaicompat's chatRequest wire type
    (internal/openaicompat/wire.go) has no temperature field and boot/config.go exposes no such
    setting, so --temperature below is accepted and reported but is NOT sent to the endpoint. The
    operator's own measurement elsewhere found a 57% output-size swing across identical runs at
    default sampling; this trace is run under whatever the endpoint's own default is, and is not
    reproducible byte-for-byte on that account. This is a finding, not a bug this script papers over.

Server lifecycle (build, free port, health wait, post, drain, stop) is reused from
scripts/compare.py by import rather than copied a third time -- DiVoid #11326 already found eight
functions drifted between compare.py and smoke.py and flagged a third copy as the thing not to do
silently. Extracting scripts/_processor_harness.py so compare.py and smoke.py stop drifting from
EACH OTHER is out of scope here (a judgement call the task explicitly leaves open) and is not done
by this script.

Every run writes one run record to the graph and this script never deletes it (DiVoid #11141): a
repeated identical task text reads back the first run's own record rather than measuring anything
new, so this script checks for a prior run of the exact input via the same semantic-recall probe
compare.py uses (find_prior_run) and says so loudly rather than silently reusing or refusing it --
visibility, not gatekeeping, is this tool's job. Whatever node id a run writes is printed immediately
after the run completes, before the trace is rendered, so a crash in rendering can never hide what
was written (the script's whole contract is "name it, never delete it").

Exit 0 the run produced a record (whatever it says); 1 the run could not be made at all.
"""

import argparse
import os
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(Path(__file__).resolve().parent))
import compare as harness  # noqa: E402  -- reused, not copied; see module docstring above.

DEFAULT_MODEL_URL = "http://gangolf:12434/engines/v1"
DEFAULT_MODEL_ID = "ai/qwen3-coder"
DEFAULT_SUBJECT = 10422  # DiVoid #10422, "Processor -- memory-substrate agent harness": the project node.

# Mirrors internal/loop/turn.go's unexported string constants -- duplicated here because the loop
# package exports no vocabulary for its tool-round error strings (deliberately: DiVoid #10850's
# scrubbing rule keeps them out of any surface an untrusted reader reaches). These two are matched
# by exact string; anything else -- including a renamed one of these two, should turn.go's literals
# ever change -- falls through to the "malformed / never dispatched" branch (see classify_round
# below), which is the safe direction to fail in: it under-claims a dispatch rather than fabricating
# one. That is what actually fixes W1's complaint, not a hard throw on an unrecognised string.
ERR_CALL_CAP_REACHED = "call cap reached"
ERR_SUPPLEMENTARY_RECALL_FAILED = "supplementary recall failed"
CUT_SELF_PRODUCED = "self-produced"
CUT_BYTE_BUDGET = "byte budget exceeded"

# classify_round's return values.
ROUND_DISPATCHED = "dispatched"          # error == "": real Graph.Recall call, real results.
ROUND_CAPPED = "capped"                  # error == ERR_CALL_CAP_REACHED: recorded, never dispatched.
ROUND_DISPATCH_FAILED = "dispatch-failed"  # error == ERR_SUPPLEMENTARY_RECALL_FAILED: dispatched, Graph.Recall itself errored.
ROUND_MALFORMED = "malformed"            # any other non-empty error: RecallError path, never dispatched.

RULE = "=" * 92
THIN = "-" * 92


class TraceFailure(Exception):
    pass


def fmt_bytes(n):
    return f"{n:,} B"


def one_line(value, width):
    text = " ".join(str(value).split())
    return text if len(text) <= width else text[: width - 1] + "…"


def classify_round(tool_call):
    """One of the ROUND_* constants for a single toolCalls[i] entry (C1).

    The record cannot distinguish "dispatched and got zero results" from "never dispatched" by
    result-count alone -- only the `error` string does, and only three of its shapes are known. See
    the module docstring's TOOL-ROUND CLASSIFICATION section for why each one means what it means.
    """
    error = tool_call.get("error") or ""
    if error == "":
        return ROUND_DISPATCHED
    if error == ERR_CALL_CAP_REACHED:
        return ROUND_CAPPED
    if error == ERR_SUPPLEMENTARY_RECALL_FAILED:
        return ROUND_DISPATCH_FAILED
    return ROUND_MALFORMED


def render_candidate_table(dispositions, budget):
    """`budget` is the cumulative admission ceiling this specific round of candidates was actually
    measured against -- for the initial round that is `assemblyByteBudget - anchor.size` (floored at
    zero), NOT the raw constant (C2/C3: the anchor is charged, not exempt); for a supplementary round
    it is the raw `supplementaryByteBudget`, unadjusted, since the anchor is not re-sent on that path.
    """
    lines = []
    for d in dispositions:
        mark = "IN " if d.get("included") else "cut"
        reason = f"  ({d.get('cutReason')})" if not d.get("included") and d.get("cutReason") else ""
        lines.append(
            f"      {mark} rank {d.get('rank'):>2}  #{d.get('id'):<7} "
            f"{one_line(d.get('type'), 12):<12} sim {d.get('similarity', 0):.4f}  "
            f"size {d.get('size', 0):>7}  {one_line(d.get('name'), 46):<46}{reason}"
        )
        if not d.get("included") and d.get("cutReason") == CUT_SELF_PRODUCED:
            lines.append(
                f"           self-produced: a run record this system wrote earlier -- cut before "
                f"the byte budget is even consulted (internal/loop/assemble.go)"
            )
        if not d.get("included") and d.get("cutReason") == CUT_BYTE_BUDGET and budget is not None:
            if d.get("size", 0) > budget:
                lines.append(
                    f"           UNADMITTABLE: {d.get('size')} bytes alone exceeds the "
                    f"{budget}-byte budget this round was measured against; no run can ever admit "
                    f"this row regardless of rank"
                )
    return lines


def admitted_bytes(dispositions):
    return sum(d.get("size", 0) for d in dispositions if d.get("included"))


def render_trace(record, model_url, model_id, temperature_requested, prior_note):
    """Pure function: every value comes from `record` (W4 -- the record is the source of truth,
    not the argv that produced the request that produced it) plus display-only context the record
    itself has no way to carry (the endpoint address, the un-sent --temperature, the duplicate-run
    warning computed against the graph before the run started).
    """
    task_text = record.get("input", "")
    subject = record.get("subject")

    step = [0]

    def head(label):
        step[0] += 1
        return f"STEP {step[0]:<2} {label:<15}"

    out = []
    out.append(RULE)
    out.append(f"TRACE  subject #{subject}  model {model_id} @ {model_url}")
    out.append(RULE)
    out.append("TASK (verbatim, this is exactly what the record's own Input field carries):")
    out.append(THIN)
    out.append(task_text)
    out.append(THIN)
    if temperature_requested is not None:
        out.append(
            f"NOTE: --temperature {temperature_requested} was requested but the binary's model client "
            f"(internal/openaicompat) has no temperature field on its wire request and no config "
            f"surface for one -- it was NOT sent. Sampling is whatever the endpoint defaults to, and "
            f"this trace is not reproducible byte-for-byte on that account."
        )
    if prior_note:
        out.append(prior_note)
    out.append("")

    # ---- STEP: anchor ----------------------------------------------------------------------
    anchor = record.get("anchor") or {}
    anchor_size = anchor.get("size", 0)
    limits = record.get("limits") or {}
    assembly_budget = limits.get("assemblyByteBudget")
    remaining_budget = None if assembly_budget is None else max(assembly_budget - anchor_size, 0)

    out.append(f"{head('anchor')} input: subject id #{subject}")
    out.append(
        f"{'':<24} output: #{anchor.get('id')} {anchor.get('type')} "
        f"{anchor.get('name')!r}  {fmt_bytes(anchor_size)}  "
        f"hash {one_line(anchor.get('contentHash'), 16)}"
    )
    out.append(
        f"{'':<24} note: the anchor is never CUT -- its full content always reaches the model "
        f"(internal/loop/assemble.go's renderBlock writes it first, unconditionally, regardless of "
        f"size) -- but it IS CHARGED against the assembly budget before any candidate is considered "
        f"(remaining := budget - len(anchor.Content), floored at zero). This anchor consumes "
        f"{fmt_bytes(anchor_size)} of the {fmt_bytes(assembly_budget or 0)} budget, leaving "
        f"{fmt_bytes(remaining_budget or 0)} for every candidate below -- see STEP 3. Its raw text "
        f"is not itself in the record -- only this summary and, indirectly, whatever of it survives "
        f"inside the block below."
    )
    out.append("")

    # ---- STEP: recall (initial) ------------------------------------------------------------
    candidates = record.get("candidates") or []
    candidate_limit = limits.get("candidateLimit")
    admitted_count = sum(1 for c in candidates if c.get("included"))
    out.append(f"{head('recall')} input: query={record.get('query')!r} (the task input, verbatim -- no rewrite, no model in the path)")
    out.append(
        f"{'':<24} output: {len(candidates)} candidate(s) returned (limit {candidate_limit}), "
        f"mechanism: reciprocal-rank fusion of an unscoped recall with a recall scoped to the anchor's "
        f"two-hop neighbourhood (internal/loop/retrieve.go's Retrieve/fuse) -- 'rank' below is fused "
        f"rank, not a plain similarity sort"
    )
    out.extend(render_candidate_table(candidates, remaining_budget))
    out.append("")

    # ---- STEP: assemble ---------------------------------------------------------------------
    block = record.get("block", "")
    block_size = len(block.encode("utf-8"))
    cand_bytes = admitted_bytes(candidates)
    framing = block_size - anchor_size - cand_bytes
    cut_tally = {}
    for c in candidates:
        if not c.get("included"):
            reason = c.get("cutReason") or "(no reason given)"
            cut_tally[reason] = cut_tally.get(reason, 0) + 1
    out.append(
        f"{head('assemble')} input: anchor ({fmt_bytes(anchor_size)}, charged in full) + "
        f"{len(candidates)} candidate(s); assembly budget {fmt_bytes(assembly_budget or 0)} total, "
        f"{fmt_bytes(remaining_budget or 0)} remaining for candidates after the anchor charge"
    )
    out.append(
        f"{'':<24} output: {admitted_count} of {len(candidates)} admitted, block {fmt_bytes(block_size)} "
        f"(anchor {fmt_bytes(anchor_size)} + candidates {fmt_bytes(cand_bytes)} + framing {fmt_bytes(framing)})"
    )
    if cut_tally:
        tally_text = ", ".join(f"{count}x {reason}" for reason, count in sorted(cut_tally.items(), key=lambda kv: -kv[1]))
        out.append(f"{'':<24} cut reasons: {tally_text}")
    if candidates and admitted_count == 0:
        out.append(
            f"{'':<24} SHUTOUT: assembly admitted nothing from a non-empty candidate set -- the block "
            f"below carries the anchor alone. internal/loop/turn.go logs this as a WARN to stderr "
            f"(not visible in the record itself; the record shows it only via this arithmetic)."
        )
    out.append("")
    out.append(f"{'':<24} BLOCK SENT TO THE MODEL, {fmt_bytes(block_size)}, verbatim:")
    out.append(THIN)
    out.append(block)
    out.append(THIN)
    out.append("")

    # ---- model-call / tool-call loop ---------------------------------------------------------
    tool_calls = record.get("toolCalls") or []
    usage = record.get("usage") or []
    model_calls = record.get("modelCalls") or 0
    cap_reached = bool(record.get("capReached"))
    stop_reason = record.get("stopReason") or {}
    supplementary_budget = limits.get("supplementaryByteBudget")

    for i in range(model_calls):
        call_no = i + 1
        u = usage[i] if i < len(usage) else None
        prompt_desc = f"{u['inTokens']} tok in" if u else "usage not reported"
        wanted_recall = i < len(tool_calls)
        tc = tool_calls[i] if wanted_recall else None
        category = classify_round(tc) if wanted_recall else None
        out_tok = f"{u['outTokens']} tok" if u else "? tok"

        prior_rounds = i  # how many completed recall exchanges are already replayed into this call's prompt
        out.append(
            f"{head('model call ' + str(call_no))} input: system + block ({fmt_bytes(block_size)}) + "
            f"task input + {prior_rounds} prior recall round(s) replayed as tool messages "
            f"[{prompt_desc}]"
        )

        if not wanted_recall:
            out.append(
                f"{'':<24} output: {stop_reason.get('reason')!r} (endpoint raw={stop_reason.get('raw')!r}). "
                f"out={out_tok}"
            )
        elif category == ROUND_CAPPED:
            out.append(
                f"{'':<24} output: wants recall (query={tc.get('query')!r}), but the model-call cap "
                f"(MaxModelCalls={limits.get('maxModelCalls')}) was reached -- NOT dispatched, "
                f"counted only. out={out_tok}"
            )
        elif category == ROUND_DISPATCH_FAILED:
            out.append(
                f"{'':<24} output: wants recall (query={tc.get('query')!r}), dispatched, but the "
                f"supplementary recall call itself FAILED (scrubbed reason on this surface by "
                f"design -- internal/loop/turn.go's error-scrubbing rule, DiVoid #10850). "
                f"out={out_tok}"
            )
        elif category == ROUND_MALFORMED:
            ambiguous_cap = cap_reached and i == model_calls - 1
            out.append(
                f"{'':<24} output: wants recall, but the tool call itself was malformed and NEVER "
                f"reached the graph -- error: {tc.get('error')!r}. query not recorded (turn.go's "
                f"RecallExchange on this path never carries one). out={out_tok}"
            )
            if ambiguous_cap:
                out.append(
                    f"{'':<24} note: this is also the final model call and capReached=true, but "
                    f"turn.go's malformed-request reason wins over the cap reason when both apply to "
                    f"the same round -- whether the cap would separately have stopped this round "
                    f"cannot be told from the record."
                )
        else:  # ROUND_DISPATCHED
            out.append(
                f"{'':<24} output: wants recall (query={tc.get('query')!r}). out={out_tok}"
            )
        out.append("")

        if category == ROUND_DISPATCHED:
            results = tc.get("results") or []
            kept = sum(1 for r in results if r.get("included"))
            kept_bytes = admitted_bytes(results)
            out.append(
                f"{head('tool call')} input: recall(query={tc.get('query')!r}, "
                f"limit={candidate_limit}, scope=nil -- whole graph, deliberately unscoped)"
            )
            out.append(
                f"{'':<24} output: {len(results)} candidate(s) returned, {kept} admitted under "
                f"the supplementary budget ({fmt_bytes(supplementary_budget or 0)}), "
                f"{fmt_bytes(kept_bytes)} kept"
            )
            out.extend(render_candidate_table(results, supplementary_budget))
            out.append("")

    # ---- RESULT -------------------------------------------------------------------------------
    answer = record.get("answer", "")
    answer_bytes = len(answer.encode("utf-8"))
    written = record.get("written") or {}
    out.append(RULE)
    out.append(
        f"RESULT  answer {fmt_bytes(answer_bytes)}, {model_calls} model call(s), "
        f"{'cap reached' if cap_reached else 'cap not reached'}, "
        f"stopReason={stop_reason.get('reason')!r}, receipt={written.get('state')!r}"
        + (f", node #{written.get('nodeId')}" if written.get("nodeId") else "")
    )
    out.append(RULE)
    out.append("ANSWER, full text, verbatim:")
    out.append(THIN)
    out.append(answer)
    out.append(THIN)

    return "\n".join(out)


def check_prior_run(divoid_url, divoid_key, task_text):
    try:
        prior = harness.find_prior_run(divoid_url, divoid_key, task_text)
    except harness.CompareFailure as err:
        return f"NOTE: could not check for a prior run of this exact text ({err}); proceeding anyway."
    if prior is None:
        return None
    node_id, name = prior
    return (
        f"NOTE: this exact task text was already run -- graph node #{node_id} ({name!r}) carries a "
        f"run record whose input matches verbatim. This run's retrieval will read that record back "
        f"(DiVoid #11141: a repeated input is not a second measurement of anything). Running anyway, "
        f"per instructions -- this note exists so the reader knows before reading the trace."
    )


def announce_written(record):
    """Print what this run wrote to the graph immediately -- before rendering the trace, and
    unconditionally -- so that a crash in render_trace (a bug, a record shape this script does not
    yet handle) can never suppress the one thing this script promises never to lose track of (W5:
    "name what it wrote and never delete it" has to hold even when everything after it throws)."""
    written = record.get("written") or {}
    if written.get("nodeId"):
        print(
            f"GRAPH: wrote run record #{written['nodeId']} ({written.get('state')!r}) -- named here "
            f"in case rendering below fails; this script never deletes it."
        )
    else:
        print(f"GRAPH: no run record stored (state={written.get('state')!r})")


def run_one(model_url, model_id, model_key, divoid_url, divoid_key, task_text, subject, temperature):
    env = harness.child_env(model_url, model_id, model_key, divoid_url, divoid_key)

    prior_note = check_prior_run(divoid_url, divoid_key, task_text)

    with tempfile.TemporaryDirectory(prefix="processor-trace-", ignore_cleanup_errors=True) as tmp:
        binary = Path(tmp) / ("processor.exe" if os.name == "nt" else "processor")
        log_path = Path(tmp) / "server.log"
        log = None
        proc = None
        in_flight = False
        try:
            harness.build(binary)
            port = harness.free_port()
            env["PROCESSOR_HTTP_ADDR"] = f"127.0.0.1:{port}"
            print(f"model {model_id} at {model_url}, graph {divoid_url}, server on 127.0.0.1:{port}")

            log = open(log_path, "wb")
            proc = harness.start_server(binary, env, log)
            harness.wait_for_health(proc, port, log_path)

            in_flight = True
            record = harness.post_run(port, task_text, subject)
            in_flight = False

            announce_written(record)
            return render_trace(record, model_url, model_id, temperature, prior_note)
        finally:
            if proc is not None:
                harness.stop_server(proc, log_path, in_flight)
            if log is not None:
                log.close()


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--input", required=True, help="the task text, sent verbatim as the run's input")
    parser.add_argument("--subject", type=int, default=DEFAULT_SUBJECT, help=f"subject node id (default {DEFAULT_SUBJECT})")
    parser.add_argument("--model-url", default=os.environ.get("PROCESSOR_MODEL_URL", DEFAULT_MODEL_URL))
    parser.add_argument("--model-id", default=os.environ.get("PROCESSOR_MODEL_ID", DEFAULT_MODEL_ID))
    parser.add_argument("--model-key", default=os.environ.get("PROCESSOR_MODEL_KEY", ""))
    parser.add_argument(
        "--temperature", type=float, default=None,
        help="requested but NOT sent -- the binary has no temperature knob; see the script docstring",
    )
    return parser.parse_args()


def main():
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    args = parse_args()

    try:
        divoid_url, divoid_key = harness.graph_credentials()
    except harness.CompareFailure as err:
        print(f"FAIL: {err}")
        return 1

    try:
        trace = run_one(
            args.model_url, args.model_id, args.model_key,
            divoid_url, divoid_key, args.input, args.subject, args.temperature,
        )
    except harness.CompareFailure as err:
        print(f"FAIL: {err}")
        return 1
    except KeyboardInterrupt:
        print("FAIL: interrupted")
        return 1

    print(trace)
    return 0


if __name__ == "__main__":
    sys.exit(main())
