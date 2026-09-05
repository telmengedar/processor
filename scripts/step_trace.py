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
one unscoped recall plus one recall scoped to the anchor's two-hop neighbourhood, combined by
reciprocal-rank fusion IN PRINCIPLE, but turn.go always calls Retrieve with exactly one query
(`[]string{input}`), and RRF over a single list is order-preserving -- see the RECALL RANKING note
below for what that actually means for the order you see) -> assemble a byte-budgeted block -> call
the model, looping while it asks for supplementary recall, bounded by MaxModelCalls=3 -> write a run
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

RECALL RANKING, corrected after review (W-N2 -- a prose defect, not a logic one, but it printed on
every trace and asserted the opposite of what retrieve.go does): `retrieve.go`'s `fuse` does NOT fuse
the scoped list in. `fuseByReciprocalRank(lists)` runs over the UNSCOPED lists only; the scoped list
gets a reserved quota (`RecallScopeReserve`, mirrored below as RECALL_SCOPE_RESERVE = 3) taken after
the first `limit - reserve` fused entries, backfilled from the fused list only if the scope doesn't
fill its reserve. Since a turn always passes exactly one query, RRF over that single list changes
nothing -- it is order-preserving. Net, for every trace this script prints: positions 1 through
`limit - RECALL_SCOPE_RESERVE` are the one unscoped recall's own plain-similarity order, verbatim.
The last `RECALL_SCOPE_RESERVE` positions are RESERVED FOR the scoped recall, which is not the same
as filled from it. `fuse` runs THREE passes, not two: fill to `limit - reserve` from the fused list;
take up to `reserve` UNSEEN rows from the scoped list; then fill any slot the scope left over FROM
THE FUSED LIST AGAIN, continuing its plain-similarity order past where the first pass stopped. So a
run whose scoped recall returned fewer than `RECALL_SCOPE_RESERVE` unseen rows ends with tail rows
that are not neighbourhood hits at all, and which pass placed any given tail row is NOT recoverable
from the record: `Disposition` carries rank, id, type, name, similarity, size, content hash and the
admit decision, the `Candidate` it is built from records only whether the row is self-produced --
never WHICH RECALL RETURNED IT -- and the scoped list is not in the record to compare against. The
trace therefore reports the reservation and declines to label the rows -- inventing a per-row
distinction the record cannot support would be the same defect one level up. Fusion of multiple
ranked lists is real and does real work in `cmd/eval`, which passes more than one query -- it is
simply inert inside a turn, which never does. The two-hop
half of the mechanism (`RecallScope` = subject + its linked neighbours) is accurate as stated.

BUDGET ARITHMETIC, corrected after review (C2/C3): the anchor is not exempt from the assembly
budget. `internal/loop/assemble.go`: `remaining := budget - len(anchor.Content)`, floored at zero --
the anchor's bytes are charged against the budget, in full, before any candidate is even considered.
What the anchor IS exempt from is being CUT: its full content always reaches the model regardless of
size (design correction pinned in this tree at `m1-skeleton-loop.md:765`: "the anchor is exempt from
being cut, not from being charged"). This script computes and prints the actual per-run candidate
budget (`assemblyByteBudget - anchor.size`, floored at zero) and tests admissibility against THAT
number, not the raw constant -- a candidate that fits the constant but not the anchor-adjusted
remainder is unadmittable for this run and previously got no marker at all (C3). The supplementary
round is unaffected by this, but not for the reason an earlier version of this file claimed: the
anchor IS re-sent on every model call -- `wire.go`'s `buildMessages` rebuilds the same
`buildUserContent(in.Block, in.Input)` (the whole block, anchor included) on every call, so its bytes
go over the wire again each time. The real reason `SupplementaryByteBudget` is used unadjusted is
simpler: `dispatchRecall` has no anchor in scope at all -- it calls `admit(candidates,
SupplementaryByteBudget)` directly, with nothing available to subtract even if the mechanism wanted
to. The repeated anchor bytes are a token-cost fact (see WHAT THE RECORD CANNOT TELL A READER's
system-prompt bullet), not a budget-arithmetic one.

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
  - Whether the endpoint HONOURED the sampling the record reports. internal/boot reads
    PROCESSOR_MODEL_TEMPERATURE (optional, defaulting to 0) and PROCESSOR_MODEL_TOP_P (optional, no
    default -- unset means the parameter is omitted from the request entirely, never sent as 0);
    internal/openaicompat's client puts exactly those two on the wire and hands the same pair back,
    which is what the record's `sampling` object carries. So the record -- and the SAMPLING line this
    script prints off it -- is an account of what was REQUESTED, AS SENT, and of nothing else. An
    endpoint that clamps a value, or ignores it, yields a record true about the request and false
    about the generation, and nothing on this side of the wire can see which happened. Temperature 0,
    the default, stops the sampler being a source of run-to-run variation IF the endpoint honours it;
    that is the whole of the claim, and it is not a claim that the run repeats. --temperature below
    sets PROCESSOR_MODEL_TEMPERATURE for the run it traces, so the flag does reach the endpoint --
    but the SAMPLING line is still read back off the record, never off the flag.

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

# internal/loop/turn.go's RecallScopeReserve -- not carried in the record's `limits` object (Limits
# only has the five members turn.go stamps into it), so it is mirrored here the same way the two
# error-string literals above are, purely for narrating STEP 2's actual rank order (W-N2).
RECALL_SCOPE_RESERVE = 3

# internal/boot's own variable name. The binary has no flag surface at all -- boot/config.go reads
# the process environment and nothing else -- so setting this on the child's environment is the only
# way --temperature can reach the endpoint, and it is the same mechanism this script already uses for
# the model url, id and key.
ENV_MODEL_TEMPERATURE = "PROCESSOR_MODEL_TEMPERATURE"

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
    it is the raw `supplementaryByteBudget`, unadjusted, because `dispatchRecall` has no anchor in
    scope to subtract -- NOT because the anchor is omitted from that call's prompt (it isn't: the
    whole block, anchor included, is resent on every model call; see the module docstring's BUDGET
    ARITHMETIC section for the corrected reasoning).
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


def anchor_charge_phrase(anchor_size, assembly_budget, remaining_budget):
    """Text for how much of the assembly budget the anchor consumes, or an honest admission that it
    cannot be said -- a missing `limits.assemblyByteBudget` must never be silently treated as 0 (a
    reviewer caught this: `or 0` elsewhere in this file would otherwise print a fabricated concrete
    number, e.g. "consumes 4,000 B of the 0 B budget", for a field the record simply did not carry).
    Unreachable from the binary today (Limits is a value struct, always present), but the record is
    an external boundary and this script should not assume today's shape survives unchanged.
    """
    if assembly_budget is None:
        return "the record carries no limits.assemblyByteBudget, so the anchor's charge against it cannot be computed"
    return (
        f"this anchor consumes {fmt_bytes(anchor_size)} of the {fmt_bytes(assembly_budget)} budget, "
        f"leaving {fmt_bytes(remaining_budget)} for every candidate below -- see STEP 3"
    )


def fmt_sampling(value):
    """The number the record carries, spelled with no conversion and no rounding.

    `repr` and not a format spec, deliberately: `f"{value:g}"` rounds to six significant figures, so
    a temperature of 0.123456789 -- really sent, really recorded -- would print as 0.123457, and
    1234567.0 as 1.23457e+06. Numbers the endpoint was never asked for, printed by the one line in
    this script whose whole claim is what was REQUESTED, AS SENT. `repr` of a float is its shortest
    round-tripping decimal, so what prints here parses back to exactly what the record holds; 0.1+0.2
    printing as 0.30000000000000004 is the honest outcome and stays.

    Not `repr(float(value))` either: the binary marshals a temperature of 0 as the JSON number `0`,
    which decodes to a Python int, and converting it would print `0.0` -- a spelling the record does
    not contain. Quoting the decoded value is the whole job; every conversion on the way is a chance
    to quote something else.
    """
    return repr(value)


def sampling_lines(record, temperature_requested):
    """The SAMPLING block: what the record says was put on the wire, and the exact limit of that
    claim.

    Three record shapes, kept apart deliberately. A record with no `sampling` object at all is older
    than the field and says nothing about what was sent; a `sampling` object with neither member set
    says nothing was sent for either; a member with a value is a value that was sent. None of the
    three may be rendered as any of the others -- a missing field must not print as 0, which is a
    real number this endpoint would really have been asked for.
    """
    sampling = record.get("sampling")
    if sampling is None:
        lines = [
            "SAMPLING: this record carries no sampling object at all -- it was written before the "
            "run record had the field. What that run sent for temperature or top_p cannot be read "
            "off it, and this trace does not invent it."
        ]
        temperature = None
    else:
        temperature = sampling.get("temperature")
        top_p = sampling.get("topP")
        if temperature is None and top_p is None:
            lines = [
                "SAMPLING: nothing sent -- the record's sampling object carries neither temperature "
                "nor top_p, so both were left to whatever the endpoint's own default is. What that "
                "default is, the record cannot say: it reports the near side of the wire only."
            ]
        else:
            temp_text = (
                f"temperature {fmt_sampling(temperature)}" if temperature is not None
                else "temperature not sent"
            )
            top_p_text = (
                f"top_p {fmt_sampling(top_p)}" if top_p is not None
                else "top_p not sent (omitted from the request entirely, never sent as 0)"
            )
            lines = [
                f"SAMPLING: {temp_text}, {top_p_text}. This is what was REQUESTED, AS SENT -- not "
                f"what the endpoint applied. An endpoint that clamps a value, or ignores it, yields "
                f"a record true about the request and false about the generation, and nothing on "
                f"this side of the wire can see which happened."
            ]
        if temperature == 0:
            lines.append(
                "          temperature 0 is greedy decoding: IF the endpoint honours it, the sampler "
                "stops being a source of run-to-run variation. That is the whole of the claim -- it "
                "is not a claim that this run repeats."
            )

    if temperature_requested is not None:
        lines.append(
            f"          --temperature {temperature_requested} was passed to this script, which set "
            f"{ENV_MODEL_TEMPERATURE} for this run; the SAMPLING line above is the record's own "
            f"account of what the client sent, read back rather than restated from the flag."
        )
        if sampling is not None and temperature != temperature_requested:
            lines.append(
                f"          MISMATCH: the record reports {temperature!r}, not the "
                f"{temperature_requested!r} this script asked for. The request did not reach the "
                f"client the way this script assumes it does -- trust the record, not the flag."
            )
    return lines


def render_trace(record, model_url, model_id, temperature_requested, prior_note):
    """Pure function: every value comes from `record` (W4 -- the record is the source of truth,
    not the argv that produced the request that produced it) plus display-only context the record
    itself has no way to carry (the endpoint address, the --temperature this script was asked for --
    reported only so a disagreement with the record is visible, never as a substitute for it -- and
    the duplicate-run warning computed against the graph before the run started).
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
    out.extend(sampling_lines(record, temperature_requested))
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
        f"(remaining := budget - len(anchor.Content), floored at zero). "
        f"{anchor_charge_phrase(anchor_size, assembly_budget, remaining_budget)}. Its raw text is "
        f"not itself in the record -- only this summary and, indirectly, whatever of it survives "
        f"inside the block below."
    )
    out.append("")

    # ---- STEP: recall (initial) ------------------------------------------------------------
    candidates = record.get("candidates") or []
    candidate_limit = limits.get("candidateLimit")
    admitted_count = sum(1 for c in candidates if c.get("included"))
    out.append(f"{head('recall')} input: query={record.get('query')!r} (the task input, verbatim -- no rewrite, no model in the path)")
    if candidate_limit is not None:
        fused_slots = max(candidate_limit - RECALL_SCOPE_RESERVE, 0)
        rank_note = (
            f"a turn always issues exactly one query, so the reciprocal-rank fusion step "
            f"(internal/loop/retrieve.go's fuse/fuseByReciprocalRank) is a no-op over it -- rows 1-"
            f"{fused_slots} below are that one UNSCOPED recall's own plain-similarity order, "
            f"verbatim. The last {RECALL_SCOPE_RESERVE} slots ({fused_slots + 1}-{candidate_limit}) "
            f"are RESERVED FOR a second recall scoped to the anchor's two-hop neighbourhood (subject "
            f"+ its linked nodes) -- which is not the same as filled from it: fuse takes unseen "
            f"scoped rows up to the reserve and then fills whatever the scope left over from the "
            f"unscoped list AGAIN, continuing its plain-similarity order. Which pass placed any one "
            f"of those {RECALL_SCOPE_RESERVE} rows is not in the record -- no candidate records "
            f"which recall returned it, and the scoped list is not recorded -- so each of them may be a "
            f"neighbourhood hit or a similarity continuation, and this trace will not guess which. "
            f"Fusion across multiple queries only does real work in cmd/eval, which passes more than "
            f"one -- it never touches the order here"
        )
    else:
        rank_note = (
            f"the record carries no limits.candidateLimit, so the fused/scoped split point cannot be "
            f"computed; see the module docstring's RECALL RANKING section for the mechanism"
        )
    out.append(
        f"{'':<24} output: {len(candidates)} candidate(s) returned (limit {candidate_limit}); {rank_note}"
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
        f"{len(candidates)} candidate(s); "
        f"{anchor_charge_phrase(anchor_size, assembly_budget, remaining_budget)}"
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

    if model_calls == 0 and tool_calls:
        out.append(
            f"NOTE: modelCalls is 0 but the record carries {len(tool_calls)} toolCalls entr"
            f"{'y' if len(tool_calls) == 1 else 'ies'} -- a shape the loop should not be able to "
            f"produce (every recall round is dispatched from inside the model-call loop). Printed "
            f"here explicitly rather than silently iterating zero times, which would otherwise drop "
            f"these rounds from the trace with no trace of the drop itself."
        )
        out.append("")

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


def apply_temperature(env, temperature):
    """Put --temperature where the binary will actually read it, mutating and returning `env`.

    Only when asked: compare.py's child_env starts from os.environ.copy(), so an ambient
    PROCESSOR_MODEL_TEMPERATURE already reaches the child unaided, and writing the key
    unconditionally would overwrite that inherited value with this script's own idea of a default.
    Absent the flag, the binary's own default (boot/config.go: 0 when the variable is unset) wins.
    """
    if temperature is None:
        return env
    # repr() of a float is its shortest round-tripping decimal, so the number Go parses back is
    # bit-for-bit the one argparse produced -- which is what lets sampling_lines compare the record's
    # value against the request by equality without inventing a tolerance. The float() is deliberate
    # HERE and deliberately absent in fmt_sampling: this writes a value Go must parse as a float
    # literal, that one quotes a value the record already holds. Do not unify them.
    env[ENV_MODEL_TEMPERATURE] = repr(float(temperature))
    return env


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
    env = apply_temperature(
        harness.child_env(model_url, model_id, model_key, divoid_url, divoid_key), temperature
    )

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
        help=f"sets {ENV_MODEL_TEMPERATURE} for this run; omit to leave the environment alone and "
             f"take the binary's own default (0). The trace reports the sampling the record says was "
             f"sent, not this flag -- and says what that can and cannot claim",
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
