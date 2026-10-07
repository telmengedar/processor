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

What "one turn" actually is (internal/loop/turn.go), verified against this tree rather than assumed:
fetch the anchor by id -> retrieve candidates (Retrieve -- one unscoped recall plus one recall scoped
to the anchor's two-hop neighbourhood, combined by reciprocal-rank fusion IN PRINCIPLE, but turn.go
always calls Retrieve with exactly one query (`[]string{input}`), and RRF over a single list is
order-preserving -- see the RECALL RANKING note below for what that actually means for the order you
see) -> assemble a byte-budgeted block -> call the model, looping while it asks for a tool, bounded
by MaxModelCalls=6 -> write a run record. The model has THREE tools: the supplementary "recall";
"write_file", which writes one file into a working directory the run is given (internal/workspace);
and "read_node", which fetches ONE part of memory by the id the model names and puts it in front of
the model in full. There is still no shell, no network and no repo tool, and no notion of a task
spanning more than one HTTP call. So a task like "generate a webpage and a repo" can now get the
webpage written and cannot get the repo -- and whether the model reaches for a tool at all,
unprompted, is the thing this trace exists to show. What it also shows is HOW it fails when it does:
refuses, answers about the task, asks for recall, claims completion without writing anything, or
produces something confidently wrong.

The file tool is offered but not urged, and only where a working directory exists:
internal/systemtext composes each call's system text from the tools that call offers, naming each in
one neutral sentence, and still tells the model to write its answer as prose for a person. A run in which the model describes the work instead of doing it is
therefore a result, not a misconfiguration -- and stopReason "answered" on a run that wrote nothing
stays exactly as wrong as it was before, deliberately: nothing here was changed to make it right.

One POST /runs is one turn and returns one JSON record (internal/loop/types.go's Record, wrapped by
internal/server/routes.go, when asked for with ?verbose=true as compare.post_run does, in
{ ...Record fields, "written": {state, nodeId}, "reply": string }). Every "step" below is
reconstructed from that one record after the fact -- there is no intermediate progress feed -- so the
ordering is inferred from the record's own structure (usage is one entry per model call, in call
order; toolCalls is one entry per call that asked for a tool, in call order, including a call the
loop refused -- see the tool-round classification note below) rather than observed live. Anywhere
that inference could be wrong, the trace says so rather than guessing quietly.

TOOL-ROUND CLASSIFICATION, and the rule it is built to keep: A NARRATION THIS SCRIPT CANNOT SUPPORT
IS VISIBLY ABSENT, NEVER INVENTED. A toolCalls[i] entry is read on two axes and each has an explicit
arm for "this script does not know", because both axes have grown under it before.

  THE TOOL. `tool` is one of "recall", "writeFile" or "readNode", and each has a renderer of its
  own. A round whose `tool` key is ABSENT -- absent, not empty -- is read as a recall, correct
  because every round in a record older than the field was one. A round whose `tool` is PRESENT and
  is none of the three is rendered as UNRECOGNISED: this script prints the record's fields verbatim
  and says it cannot narrate the round. THE EMPTY STRING IS PRESENT, not absent, and takes the
  unrecognised arm with every other unknown name. That arm is the point of the design. It was added
  after a served "readNode" -- a single Graph.Node fetched by id -- fell through into the recall
  renderer and printed `recall(query=None, limit=20, scope=nil -- whole graph, deliberately
  unscoped)`, a whole-graph search the run never issued, with a limit and a scope the round did not
  have. The next tool the loop grows reaches the unrecognised arm rather than the nearest renderer
  that happens to be last. The first version of THIS paragraph was already true of a tool named
  wrongly and false of a tool named emptily, because the code behind it read the two the same way;
  the seam is closed and both halves are pinned by a test.

  THE CAUSE. A non-empty `error` is not one thing. `""` is a served round and a tool-call step
  follows it. A sentence the LOOP authored -- the reserved-call refusal, a closed recall, and the
  four refusals of a read -- is printed as that sentence and nothing more. Any OTHER non-empty
  string is UNRECOGNISED, on the same terms as an unknown tool.

  WHAT IS NO LONGER CLAIMED, and its falsifier. This script used to say of every unrecognised cause
  that "the tool call itself was malformed and NEVER reached the graph". That was true when the only
  causes were the adapter's own ToolError; it is false now, and demonstrably. Counted against
  dispatchRead's control flow rather than summarised: of the four refusals of a read, ONE
  (`errReadOfSubject`) is raised before `Graph.Node` is called and THREE (`errNoSuchNode`,
  `errAlreadyReadInFull`, the too-large refusal) after it has returned -- so the retired sentence
  was false of three of the four, not of some of them. The reserved-call refusal describes a
  well-formed request the loop declined, which is a third thing again. No arm of this script now
  says whether a refused round reached the graph, because the record does not carry it: the cause
  strings are the whole of what survives, and they do not sort into before and after as a set. The
  claim is falsified the moment a record shows a refused round whose fetch demonstrably happened --
  which is exactly why it is gone rather than reworded. The first version of this paragraph said
  "two before and two after", which was itself a confident, specific and false sentence about
  another package's mechanism, printed by --help, inside the change that exists to stop printing
  those. It was caught by review reading turn.go line by line rather than reading this paragraph.

  ARCHIVE-ONLY CAUSES. "call cap reached", "supplementary recall failed" and "file write failed"
  were once written into records and are not any more. Their branches are kept because this script
  renders archived records and the archive still carries all three, and every line they print says
  ARCHIVE SHAPE so nobody reads them as live.

RECALL RANKING, corrected twice -- once after review found it asserting the opposite of what
retrieve.go does (a prose defect, not a logic one, but it printed on every trace), and once after
`Disposition.Sources` landed and turned that correction's own closing claim false. `retrieve.go`'s
`fuse` does NOT fuse the scoped list in. `fuseByReciprocalRank(lists)` runs over the UNSCOPED lists
only; the scoped list gets a reserved quota (`RecallScopeReserve`, mirrored below as
RECALL_SCOPE_RESERVE = 3) taken after the first `limit - reserve` fused entries, backfilled from the
fused list only if the scope doesn't fill its reserve. Since a turn always passes exactly one query,
RRF over that single list changes nothing -- it is order-preserving. Net, for every trace this
script prints: positions 1 through `limit - RECALL_SCOPE_RESERVE` are the one unscoped recall's own
plain-similarity order, MINUS THE ANCHOR -- `fuse` seeds its seen-set with the anchor id, so the
subject never appears in its own candidate set, and an unscoped recall that returned it leaves a gap
the rest of the order closes up rather than a row you can see. The last `RECALL_SCOPE_RESERVE`
positions are RESERVED FOR the scoped recall, which is not the same as filled from it. `fuse` runs
THREE passes, not two: fill to `limit - reserve` from the fused list; take up to `reserve` UNSEEN
rows from the scoped list; then fill any slot the scope left over FROM THE FUSED LIST AGAIN,
continuing its plain-similarity order past where the first pass stopped. So a run whose scoped
recall returned fewer than `RECALL_SCOPE_RESERVE` unseen rows ends with tail rows that are not
neighbourhood hits at all -- and the record now says which rows those are.

WHICH RECALL RETURNED A ROW IS RECORDED. `Disposition.Sources` carries, per row, every recall that
returned it, as `{query, scoped, rank}` (`retrieve.go`'s `sourcesOf` and `attribute`), so this trace
prints it in a `src` column and stops declining the question. An earlier version of this file said
the opposite -- that no candidate records which recall returned it and the trace therefore would not
label the rows. That was true when it was written and false two commits later, which makes the
decline itself the false claim now. What this trace still does NOT print is which of `fuse`'s three
passes PLACED a row. `Sources` is per-recall, not per-pass, and the two do not coincide in general:
a row returned by both recalls carries a scoped source wherever it sits, including the slots above
the reserve, which the unscoped order fills first -- so a scoped source is never read here as "the
reserve put it there". Any pass attribution would rest on an invariant of `fuse`'s control flow
rather than on the record, which is the same class of defect one level up. The one place this trace
does combine rank with source is exactly the question the reserved tail was written to raise: a row
inside the reserved range carrying no scoped source is a row the neighbourhood recall did not
return, and that is two facts the record states, not an inference about passes. A row may carry MORE
THAN ONE source -- the same node returned by more than one recall -- and every source it carries is
printed. The query index is deliberately not printed for a single-query run: `sourcesOf` stamps the
scoped source with `Query: 0` by construction, and a turn issues exactly one query, so the index is
the same constant on every row and would read as information it is not; where a record carries more
than one query, the unscoped sources ARE printed with their index, because there it separates rows.
A supplementary round carries no sources at all and none is claimed for it -- `turn.go`'s
`dispatchRecall` hands one unscoped `Graph.Recall` straight to `admit`, and only `Retrieve`
attributes. Fusion of multiple ranked lists is real and does real work in `cmd/eval`, which passes
more than one query -- it is simply inert inside a turn, which never does. The two-hop
half of the mechanism (`RecallScope` = subject + its linked neighbours) is accurate as stated.

BUDGET ARITHMETIC, corrected after review (C2/C3): the anchor is not exempt from the assembly
budget. `internal/loop/assemble.go`: `remaining := budget - len(anchor.Content)`, floored at zero --
the anchor's bytes are charged against the budget, in full, before any candidate is even considered.
What the anchor IS exempt from is being CUT: its full content always reaches the model regardless of
size -- the M1 design puts it as "the anchor is exempt from being cut, not from being charged". This script computes and prints the actual per-run candidate
budget (`assemblyByteBudget - anchor.size`, floored at zero) and tests admissibility against THAT
number, not the raw constant -- a candidate that fits the constant but not the anchor-adjusted
remainder is unadmittable for this run and previously got no marker at all (C3). A supplementary
round is bounded differently: its room is the smaller of `SupplementaryByteBudget` and what
`JudgementPromptCeiling` leaves after everything the turn's working memory already holds (anchor
included). The record does not carry that room, so this script cannot print it; it prints the
constant as the round's most and says so. A row the memory already held is admitted without being
charged (`held` on its disposition), so a round's bytes are the rows it ADDED, not every row it
returned. The anchor is in every call's block, which is a token-cost fact (see WHAT THE RECORD
CANNOT TELL A READER's system-prompt bullet), not a budget-arithmetic one.

WHAT THE RECORD CANNOT TELL A READER -- found while building this, not fixed (constraint: no Go
file changes):
  - No content for a file the run wrote. A write round records the tool, the path, the byte
    count and whether it was accepted or refused and why -- never the bytes. What the model wrote is
    on disk, under the run's `workspace` directory, which this trace names; the record cannot be
    read as a substitute for looking there.
  - No per-step wall-clock or timestamp. turn.go computes one elapsed duration for the whole run and
    logs it to stderr; the record itself carries no timing at all, so this trace cannot show how long
    retrieval took versus how long the model took.
  - No raw content for the anchor or for any candidate, admitted or cut. AnchorSummary carries
    {id, type, name, size, contentHash}, and Disposition adds rank, similarity, the admit decision
    and its recall sources -- identity, provenance and size, and a hash where the bytes would be. The ONLY
    place actual node content survives in the record is the assembled Block string (for whatever was
    admitted) and the final Answer. A cut candidate's content is gone from the record forever, which
    means a reader can never audit what was cut, only that it was and why.
  - No system prompt text and no per-call block. The record has no System field, and the system text
    is composed per call from the tools that call offers; Record.Block is the FIRST call's block only,
    while each later call carries that block plus the rows earlier rounds found (the working memory).
    No call replays tool rounds as messages: every call is one system message and one user message.
    The exact bytes of a later call are not reconstructable from the record alone -- only the token
    counts (Usage) are. This script does not reach past the record for it.
  - Whether a REFUSED round reached the graph before it was refused. The record carries the loop's
    refusal sentence and nothing else, and the sentences do not divide along that line: of the four
    read refusals, THREE are raised after the fetch and ONE before it. This script prints the
    sentence and declines the question. An earlier version answered it, always the same way, and
    was wrong about most of the sentences the loop can write.
  - What the turn's reserved answering call did, beyond the state the record names. `capReached`
    means the turn's last call was reserved for answering because the budget was spent -- not that
    a round was cut off -- so this script prints the flag and `reservedCall` as the record spells
    them and draws no conclusion from either. It no longer prints a note about the cap racing a
    malformed request: that note described turn.go choosing between two reasons for one round, and
    the cause it was written against was deleted from turn.go.
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
scripts/compare.py by import rather than copied a third time -- eight of those functions had
already drifted between compare.py and smoke.py when this file was written, and a third copy was
the thing not to do silently. Extracting scripts/_processor_harness.py so compare.py and smoke.py stop drifting from
EACH OTHER is out of scope here (a judgement call the task explicitly leaves open) and is not done
by this script.

Every run writes one run record to the graph and this script never deletes it: a
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
DEFAULT_SUBJECT = 10422  # the project node, "Processor -- memory-substrate agent harness".

# ARCHIVE LITERALS. Every one of these was a cause turn.go stamped on a round; none of them is any
# more. `call cap reached` was deleted outright, and the two `... failed` sentences became log lines
# while the record started carrying the underlying error itself. They are kept because this script
# renders archived records and the archive still carries all three -- measured, 7 rounds across the
# 42 dumped records -- and they are named here as archive-only so nobody reads their branches as
# live. A record written by the current binary reaches none of them.
ERR_CALL_CAP_REACHED = "call cap reached"
ERR_SUPPLEMENTARY_RECALL_FAILED = "supplementary recall failed"
ERR_FILE_WRITE_FAILED = "file write failed"

# Live, and the only one of the write causes that still is: internal/loop/turn.go stamps it when the
# service was started with no working directory at all.
ERR_NO_WORKING_DIRECTORY = "no working directory is configured"

# internal/loop/turn.go's own refusal sentences, mirrored by exact text except the last, which is a
# format string and is matched on the stable middle of it. These are sentences the LOOP authored and
# showed the model, not failures of the model's request, and they can land on a round of any tool --
# which is why they are matched before the tool is looked at. A sentence outside this set is one this
# script cannot name, and the round is rendered as unrecognised rather than as any of these.
REFUSAL_RESERVED_CALL = "the turn's last call was reserved for answering, so it offered no tool and dispatched none"
REFUSAL_RECALL_CLOSED = "recall is closed for this turn: two consecutive rounds returned nothing you had not already been shown"
REFUSAL_NO_SUCH_NODE = "no part of memory has that id"
REFUSAL_READ_OF_SUBJECT = "that id is this request's own subject, and the context block already carries it in full"
REFUSAL_ALREADY_READ_IN_FULL = "that part has already been shown to you in full in this turn"
REFUSAL_NODE_TOO_LARGE_MIDDLE = " bytes and one read carries at most "

LOOP_REFUSALS = (
    REFUSAL_RESERVED_CALL,
    REFUSAL_RECALL_CLOSED,
    REFUSAL_NO_SUCH_NODE,
    REFUSAL_READ_OF_SUBJECT,
    REFUSAL_ALREADY_READ_IN_FULL,
)

# internal/loop/types.go's ToolRecall / ToolWriteFile / ToolReadNode -- the record's own tool
# vocabulary, which is deliberately NOT the wire spelling the adapter sends ("write_file",
# "read_node"). A record older than the field carries no `tool` at all; such a round is read as a
# recall, which is what every round was before the field existed. A `tool` this script does not know
# is NOT read as a recall: it is rendered as unrecognised, because a fourth tool will find this file
# the way the third one did.
TOOL_RECALL = "recall"
TOOL_WRITE_FILE = "writeFile"
TOOL_READ_NODE = "readNode"

KNOWN_TOOLS = (TOOL_RECALL, TOOL_WRITE_FILE, TOOL_READ_NODE)

# internal/workspace's rejection prefix: a refusal by the path rules, whose reason is safe to show
# and is shown verbatim.
WRITE_REJECTED_PREFIX = "write rejected: "
CUT_SELF_PRODUCED = "self-produced"
CUT_BYTE_BUDGET = "byte budget exceeded"

# internal/loop/turn.go's RecallScopeReserve -- not carried in the record's `limits` object (Limits
# only has the five members turn.go stamps into it), so it is mirrored here the same way the two
# error-string literals above are: to narrate STEP 2's actual rank order and to locate the
# reserved tail, which is where a row's recorded sources become worth reading against its rank.
RECALL_SCOPE_RESERVE = 3

# internal/boot's own variable name. The binary has no flag surface at all -- boot/config.go reads
# the process environment and nothing else -- so setting this on the child's environment is the only
# way --temperature can reach the endpoint, and it is the same mechanism this script already uses for
# the model url, id and key.
ENV_MODEL_TEMPERATURE = "PROCESSOR_MODEL_TEMPERATURE"

# internal/boot's variable for the root the run's working directory is created beneath. Optional to
# the binary -- absent, every write is refused with ERR_NO_WORKING_DIRECTORY, which is a legible
# trace and a useless run -- so this script always sets it, to a directory it creates and NEVER
# deletes. The file the run wrote outliving the process is the entire point of tracing a task.
ENV_WORKSPACE_DIR = "PROCESSOR_WORKSPACE_DIR"

# classify_round's return values.
ROUND_DISPATCHED = "dispatched"          # error == "": real Graph.Recall call, real results.
ROUND_REFUSED = "refused"                # error is one of LOOP_REFUSALS: the loop declined the round in its own words.
ROUND_CAPPED = "capped"                  # error == ERR_CALL_CAP_REACHED: archive only.
ROUND_DISPATCH_FAILED = "dispatch-failed"  # error == ERR_SUPPLEMENTARY_RECALL_FAILED: archive only.
ROUND_UNRECOGNISED = "unrecognised"      # any other non-empty error: a cause this script cannot name.

# classify_write_round's return values.
WRITE_ACCEPTED = "write-accepted"          # error == "": the file was written.
WRITE_REFUSED = "write-refused"            # error starts with WRITE_REJECTED_PREFIX: the path rules refused it.
WRITE_UNCONFIGURED = "write-unconfigured"  # error == ERR_NO_WORKING_DIRECTORY: the service has no directory to write into.
WRITE_CAPPED = "write-capped"              # error == ERR_CALL_CAP_REACHED: archive only.
WRITE_FAILED = "write-failed"              # error == ERR_FILE_WRITE_FAILED: archive only.
WRITE_UNRECOGNISED = "write-unrecognised"  # any other non-empty error: a cause this script cannot name.

RULE = "=" * 92
THIN = "-" * 92


class TraceFailure(Exception):
    pass


def fmt_bytes(n):
    return f"{n:,} B"


def one_line(value, width):
    text = " ".join(str(value).split())
    return text if len(text) <= width else text[: width - 1] + "…"


def loop_refusal(tool_call):
    """The loop's own sentence for a round it declined, or "" when the error is not one of them.

    Checked ahead of every per-tool classifier because these sentences are cross-tool: turn.go's
    undispatchedExchange stamps the reserved-call refusal onto whichever tool the model asked for,
    so the same string arrives on a recall, a write and a read round alike.
    """
    error = tool_call.get("error") or ""
    if error in LOOP_REFUSALS:
        return error
    if REFUSAL_NODE_TOO_LARGE_MIDDLE in error:
        return error
    return ""


def classify_round(tool_call):
    """One of the ROUND_* constants for a single recall round.

    The record cannot distinguish "dispatched and got zero results" from "never dispatched" by
    result-count alone, and the `error` string does it only for the shapes named above. Anything
    else is ROUND_UNRECOGNISED, which is a statement about this script, not about the round.
    """
    error = tool_call.get("error") or ""
    if error == "":
        return ROUND_DISPATCHED
    if loop_refusal(tool_call):
        return ROUND_REFUSED
    if error == ERR_CALL_CAP_REACHED:
        return ROUND_CAPPED
    if error == ERR_SUPPLEMENTARY_RECALL_FAILED:
        return ROUND_DISPATCH_FAILED
    return ROUND_UNRECOGNISED


def round_tool(tool_call):
    """Which tool a toolCalls[i] entry is for, as the record spells it.

    A record written before the `tool` field existed carries none, and every round in such a record
    is a recall -- which is why an ABSENT field defaults to recall rather than printing a shrug on
    every historical record this script is pointed at. A field that is PRESENT is returned as it
    stands, including a tool this script has never heard of; see known_tool below for the difference
    that makes, which is the whole of this file's defence against the next tool the loop grows.

    ABSENT AND EMPTY ARE READ APART, and the test is `is None` rather than falsiness. `""` is a
    PRESENT value naming no tool, so it is returned as it stands and reaches the unrecognised arm; it
    is not a record too old to carry the field. The difference is not academic: `ToolCallRecord.Tool`
    carries no `omitempty`, so every round serialises the key and `""` is a legal value of it, and
    with `or` here that one input rendered as a whole-graph recall -- this file's own defect, at the
    one seam the restructure had left. The route in is a fourth tool whose dispatcher forgets to set
    Tool, which is the same one-merge-behind failure this file has now suffered seven times.
    """
    tool = tool_call.get("tool")
    return TOOL_RECALL if tool is None else tool


def known_tool(tool_call):
    """Whether this round's tool is one this script can narrate."""
    return round_tool(tool_call) in KNOWN_TOOLS


def round_wanted(tool_call):
    """What the round asked for, in the record's own terms, for any tool including an unknown one.

    Every clause here is a field the record carries, and a field the record does NOT carry is said
    to be missing rather than spelled `None`: turn.go builds the exchange for a malformed tool call
    without ever setting the query, the path or the id, and `query=None` on such a round reads as a
    search issued with an empty query. That is the smallest version of this file's whole defect.
    """
    tool = round_tool(tool_call)
    if tool == TOOL_RECALL:
        query = tool_call.get("query")
        if not query:
            return "wants recall, and the record carries no query for the round"
        return f"wants recall (query={query!r})"
    if tool == TOOL_READ_NODE:
        node_id = tool_call.get("nodeId")
        if node_id is None:
            return "wants to read a part, and the record carries no id for the round"
        return f"wants to read part #{node_id} in full"
    if tool == TOOL_WRITE_FILE:
        path = tool_call.get("path")
        if not path:
            return "wants to write, and the record carries no path for the round"
        return f"wants to write {path!r} ({fmt_bytes(tool_call.get('bytes', 0))})"
    return f"wants the tool the record names {tool!r}"


def classify_write_round(tool_call):
    """One of the WRITE_* constants for a single write round.

    Six shapes, separated by the exact `error` string, because they mean six different things to a
    reader asking "did a file appear?": accepted (yes), refused by the path rules (no, and the
    reason is the model's own to fix), the filesystem failed (no, and the reason is scrubbed off
    this surface), no working directory is configured (no, and no run can write until the operator
    sets one), capped (no, the call cap stopped it before dispatch), malformed (no, the tool call
    never parsed and the working directory was never touched).
    """
    error = tool_call.get("error") or ""
    if error == "":
        return WRITE_ACCEPTED
    if error == ERR_CALL_CAP_REACHED:
        return WRITE_CAPPED
    if error == ERR_FILE_WRITE_FAILED:
        return WRITE_FAILED
    if error == ERR_NO_WORKING_DIRECTORY:
        return WRITE_UNCONFIGURED
    if error.startswith(WRITE_REJECTED_PREFIX):
        return WRITE_REFUSED
    return WRITE_UNRECOGNISED


def format_sources(sources, name_query):
    """One row's `Disposition.sources` spelled as "which recall returned this, and at what rank".

    Every source the row carries is printed, in the order the record lists it (`sourcesOf` appends
    the unscoped recalls first, then the scoped one): a node returned by BOTH recalls is one row with
    two sources, and dropping either would misreport it as exclusive to the survivor.

    `name_query` gates the query index, which is printed only when the record carries more than one
    query. A turn issues exactly one, and `sourcesOf` stamps the scoped source with `Query: 0`
    regardless of how many there are -- so on every trace a turn can produce, the index is a constant
    on every row and would read as a distinction it is not. It is per-recall data either way: none of
    this says which of `fuse`'s passes placed the row, and nothing here should be phrased as if it
    did (module docstring, RECALL RANKING).
    """
    parts = []
    for source in sources:
        rank = source.get("rank")
        if source.get("scoped"):
            parts.append(f"scoped r{rank}")
        elif name_query:
            parts.append(f"unscoped q{source.get('query')} r{rank}")
        else:
            parts.append(f"unscoped r{rank}")
    return ", ".join(parts)


def render_candidate_table(dispositions, budget, attributed=False, name_query=False, reserve_from=None):
    """`budget` is the cumulative admission ceiling this specific round of candidates was actually
    measured against -- for the initial round that is `assemblyByteBudget - anchor.size` (floored at
    zero), NOT the raw constant (C2/C3: the anchor is charged, not exempt); for a supplementary round
    it is the raw `supplementaryByteBudget`, the most one round can take; the round's real room is
    smaller once the working memory holds rows, and the record does not carry it (see the module
    docstring's BUDGET ARITHMETIC section).

    `attributed` says whether recall sources are recorded for THIS round at all, which is a property
    of the round and not of the rows: `Retrieve` attributes every candidate it returns, and
    `dispatchRecall` attributes none of them (it hands one unscoped `Graph.Recall` straight to
    `admit`). So a supplementary round prints no `src` column -- absence there is by construction and
    means nothing about the row -- while a missing `sources` key on an ATTRIBUTED round is a record
    older than the field and prints as "not recorded", never as "no recall returned it".

    `reserve_from` is the first rank inside the slots reserved for the scoped recall, or None when
    the record carries no candidateLimit to compute it from. It is used for one statement only: a row
    sitting in a reserved slot that the scoped recall did not return. That is the question the
    reservation raises, and both halves of it are recorded -- the rank and the sources. Which of
    `fuse`'s three passes put the row there is NOT recorded and is not claimed here.
    """
    lines = []
    for d in dispositions:
        mark = "held" if d.get("held") else ("IN " if d.get("included") else "cut")
        reason = f"  ({d.get('cutReason')})" if not d.get("included") and d.get("cutReason") else ""
        sources = d.get("sources") or []
        if not attributed:
            src = ""
        elif sources:
            src = f"  src {format_sources(sources, name_query)}"
        else:
            src = "  src not recorded"
        lines.append(
            f"      {mark} rank {d.get('rank'):>2}  #{d.get('id'):<7} "
            f"{one_line(d.get('type'), 12):<12} sim {d.get('similarity', 0):.4f}  "
            f"size {d.get('size', 0):>7}  {one_line(d.get('name'), 46):<46}{src}{reason}"
        )
        if (
            attributed
            and sources
            and reserve_from is not None
            and isinstance(d.get("rank"), int)
            and d["rank"] >= reserve_from
            and not any(s.get("scoped") for s in sources)
        ):
            lines.append(
                f"           RESERVED SLOT, NOT A NEIGHBOURHOOD HIT: rank {d['rank']} is inside the "
                f"range held for the scoped recall, and the scoped recall did not return this row -- "
                f"only the unscoped one did"
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


def attribution_note(dispositions):
    """How STEP 2 introduces the `src` column, decided by what this particular record carries.

    Three shapes, kept apart on purpose, for the same reason `sampling_lines` keeps its three apart:
    a record with sources on every row, a record with them on none (written before the field
    existed), and a record with them on some are three different states of knowledge, and rendering
    any of them as another is the failure this file keeps being corrected for. In particular, a
    missing `sources` key is the record saying nothing about that row -- never the record saying no
    recall returned it.

    Empty string for an empty candidate set: there is no column to introduce, and a sentence about
    how to read rows that do not exist is noise.
    """
    if not dispositions:
        return ""

    attributed = sum(1 for d in dispositions if d.get("sources"))

    if attributed == len(dispositions):
        return (
            "Which recall returned each row IS recorded: the src column below is that row's "
            "Disposition.sources -- every recall that returned it, and at what rank -- so a row in "
            "the reserved range that the scoped recall never returned is called out as such. What "
            "sources does not say is which of fuse's three passes PLACED a row, and this trace does "
            "not claim it: a row returned by both recalls carries a scoped source wherever it sits, "
            "the slots above the reserve included."
        )
    if attributed == 0:
        return (
            "No row below carries Disposition.sources, so which recall returned each row cannot be "
            "read off this record -- it predates the field. That silence is a fact about the "
            "record's age and not about the rows: nothing here says any of them is or is not a "
            "neighbourhood hit."
        )
    return (
        f"{attributed} of {len(dispositions)} rows below carry Disposition.sources and the rest do "
        f"not -- a shape the loop does not produce, since Retrieve attributes every row it returns. "
        f"The src column reports each row exactly as the record has it, and 'not recorded' is the "
        f"record's silence about that row, never a claim that no recall returned it."
    )


def round_added(dispositions):
    """(bytes the round added to memory, rows it already held) over a round's included dispositions,
    the same two figures the Go summary prints: a held row is charged nothing."""
    added = held = 0
    for d in dispositions:
        if not d.get("included"):
            continue
        if d.get("held"):
            held += 1
            continue
        added += d.get("renderedSize", d.get("size", 0))
    return added, held


def added_phrase(dispositions):
    added, held = round_added(dispositions)
    return fmt_bytes(added) + " new" + (f", {held} already held" if held else "")


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


def provider_lines(record):
    """The PROVIDER block: which adapter and which endpoint the record says served this run.

    Read off the record, never off argv, for the same reason sampling_lines is: the endpoint address
    passed to this script is what it ASKED for, and the record is what the adapter reports it DID. A
    record older than the field carries no `provider` at all, and that is said rather than guessed --
    an absent field means the binary predates the attribution, not that the run used some default.
    """
    provider = record.get("provider") or {}
    adapter = provider.get("adapter")
    endpoint = provider.get("endpoint")
    if not adapter and not endpoint:
        return [
            "PROVIDER  not recorded -- this record predates run attribution, so which adapter and "
            "endpoint produced it cannot be read off the record and must not be inferred."
        ]
    return [f"PROVIDER  adapter {adapter!r}  endpoint {endpoint!r}  (as the adapter reported them)"]


def source_lines(tool_call):
    """How the adapter came by this round's tool call, when the record says.

    `native` is the endpoint reporting the call in its own tool-call field. `content` is the endpoint
    reporting NONE and the adapter recovering the call from the response text -- a degraded path that
    must not read as a working one, which is the entire reason the field exists. Absent means the
    record predates the field; that is stated, not filled in.
    """
    source = (tool_call or {}).get("source")
    if source == "native":
        return []
    if source == "content":
        return [
            f"{'':<24} note: this call was NOT reported by the endpoint. It was recovered from the "
            f"response TEXT by the adapter's fallback. The run acted, but the endpoint's own tool "
            f"parsing did not fire -- read this as a degraded path, not a working one."
        ]
    return [
        f"{'':<24} note: this round records no call source; the record predates the field, so "
        f"whether the endpoint reported the call cannot be told from it."
    ]


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
    out.extend(provider_lines(record))
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
    # More than one query is a shape a turn cannot produce (turn.go passes []string{input}), but the
    # record is an external boundary; where it does carry several, the query index on an unscoped
    # source separates rows and is printed. On a one-query record it is a constant and is not.
    name_query = len(record.get("queries") or []) > 1
    out.append(f"{head('recall')} input: query={record.get('query')!r} (the task input, verbatim -- no rewrite, no model in the path)")
    sources_note = attribution_note(candidates)
    if candidate_limit is not None:
        fused_slots = max(candidate_limit - RECALL_SCOPE_RESERVE, 0)
        reserve_from = fused_slots + 1
        rank_note = (
            f"a turn always issues exactly one query, so the reciprocal-rank fusion step "
            f"(internal/loop/retrieve.go's fuse/fuseByReciprocalRank) is a no-op over it -- rows 1-"
            f"{fused_slots} below are that one UNSCOPED recall's own plain-similarity order, minus "
            f"the anchor, which fuse excludes from its own candidate set. The last "
            f"{RECALL_SCOPE_RESERVE} slots ({reserve_from}-{candidate_limit}) "
            f"are RESERVED FOR a second recall scoped to the anchor's two-hop neighbourhood (subject "
            f"+ its linked nodes) -- which is not the same as filled from it: fuse takes unseen "
            f"scoped rows up to the reserve and then fills whatever the scope left over from the "
            f"unscoped list AGAIN, continuing its plain-similarity order."
            # Joined, not interpolated: an empty sources_note (no candidates to attribute) must not
            # leave a double space in a line whose every other word is asserted by a test.
            + (f" {sources_note}" if sources_note else "")
            + f" Fusion across multiple queries only does real work in cmd/eval, which passes more "
            f"than one -- it never touches the order here"
        )
    else:
        reserve_from = None
        rank_note = (
            f"the record carries no limits.candidateLimit, so the fused/scoped split point cannot be "
            f"computed; see the module docstring's RECALL RANKING section for the mechanism."
            + (f" {sources_note}" if sources_note else "")
        )
    out.append(
        f"{'':<24} output: {len(candidates)} candidate(s) returned (limit {candidate_limit}); {rank_note}"
    )
    out.extend(
        render_candidate_table(
            candidates, remaining_budget, attributed=True, name_query=name_query, reserve_from=reserve_from
        )
    )
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
        wanted_tool = i < len(tool_calls)
        tc = tool_calls[i] if wanted_tool else None
        out_tok = f"{u['outTokens']} tok" if u else "? tok"

        if i == 0:
            block_desc = f"block ({fmt_bytes(block_size)})"
        else:
            block_desc = f"block (the {fmt_bytes(block_size)} block plus the rows earlier rounds found)"
        out.append(
            f"{head('model call ' + str(call_no))} input: system + {block_desc} + "
            f"task input [{prompt_desc}]"
        )

        if not wanted_tool:
            out.append(
                f"{'':<24} output: {stop_reason.get('reason')!r} (endpoint raw={stop_reason.get('raw')!r}). "
                f"out={out_tok}"
            )
            out.append("")
            continue

        out.extend(source_lines(tc))
        out.extend(round_lines(head, tc, out_tok, limits, candidate_limit, supplementary_budget))

    # ---- RESULT -------------------------------------------------------------------------------
    out.extend(workspace_lines(record))
    answer = record.get("answer", "")
    answer_bytes = len(answer.encode("utf-8"))
    written = record.get("written") or {}
    out.append(RULE)
    out.append(
        f"RESULT  answer {fmt_bytes(answer_bytes)}, {model_calls} model call(s), "
        f"capReached={cap_reached}, {reserved_call_phrase(record)}, "
        f"stopReason={stop_reason.get('reason')!r}, receipt={written.get('state')!r}"
        + (f", node #{written.get('nodeId')}" if written.get("nodeId") else "")
    )
    out.append(RULE)
    out.append("ANSWER, full text, verbatim:")
    out.append(THIN)
    out.append(answer)
    out.append(THIN)

    return "\n".join(out)


def round_lines(head, tool_call, out_tok, limits, candidate_limit, supplementary_budget):
    """Every line for one tool round, dispatched on what the record says the round was.

    The dispatch is exhaustive by construction. The loop's own refusal sentences are checked first
    because they are cross-tool; then the three tools this script can narrate, each named
    explicitly; then one arm for a round that is none of those. NOTHING falls through into a
    renderer written for a different tool, which is the failure this shape exists to make
    impossible rather than to fix once: the fourth tool the loop grows will reach the last arm and
    print what the record carries, not a story borrowed from the third.
    """
    refusal = loop_refusal(tool_call)
    if refusal:
        return refused_round_lines(tool_call, out_tok, refusal)

    tool = round_tool(tool_call)
    if tool == TOOL_WRITE_FILE:
        return write_round_lines(head, tool_call, out_tok, limits)
    if tool == TOOL_READ_NODE:
        return read_round_lines(head, tool_call, out_tok, supplementary_budget)
    if tool == TOOL_RECALL:
        return recall_round_lines(head, tool_call, out_tok, limits, candidate_limit, supplementary_budget)
    return unrecognised_round_lines(tool_call, out_tok)


def refused_round_lines(tool_call, out_tok, refusal):
    """A round the loop declined in a sentence of its own, printed as that sentence.

    No tool-call step follows and none is invented: a refused round carries no rows. Whether the
    fetch happened before the refusal is NOT stated, because the record does not carry it -- turn.go
    raises one of the four read refusals before the graph and three after, and only the sentence
    survives. An earlier version of this file claimed such a round "NEVER reached the graph"; that
    claim was false of the reserved-call refusal, of a closed recall, and of three of the four read
    refusals.
    """
    return [
        f"{'':<24} output: {round_wanted(tool_call)}, and the LOOP refused the round in its own "
        f"words -- {refusal!r}. That sentence is what the model was shown for the call it spent, "
        f"and it is the whole of what the record says about the refusal. out={out_tok}",
        "",
    ]


def unrecognised_round_lines(tool_call, out_tok, reason=None):
    """A round this script cannot narrate: a tool it does not know, a cause it does not know, or both.

    Every line here is a field of the record printed as it stands. Nothing is said about what the
    round did, because this script does not know -- and filling that silence from the nearest tool
    it DOES know is exactly the defect this arm exists to prevent. A trace that says "I cannot read
    this round" is worth more to an operator than a confident sentence about a call never made.

    `reason` is supplied by a tool's own renderer for the third case, which is neither of the two
    computed here: a tool this script DOES know, on a round missing the field that renderer needs to
    say anything true. A read with no id and a write with no path are the two, and both used to be
    narrated with the missing field spelled `None` -- `read_node(id=None)`, and worse,
    `ACCEPTED -- 0 B written to None ... A FILE NOW EXISTS ON DISK.`
    """
    results = tool_call.get("results") or []
    fields = {key: value for key, value in tool_call.items() if key != "results"}
    if reason is None:
        reason = (
            f"the tool is not one of {', '.join(repr(t) for t in KNOWN_TOOLS)}"
            if not known_tool(tool_call)
            else "the cause is not one this file knows"
        )
    return [
        f"{'':<24} output: {round_wanted(tool_call)}. THIS SCRIPT CANNOT NARRATE THIS ROUND -- "
        f"{reason}. out={out_tok}",
        f"{'':<24} the record's own fields for it, verbatim: {fields!r}",
        f"{'':<24} it carries {len(results)} result row(s). Nothing is claimed here about what the "
        f"round did or whether it reached the graph: the record does not say and this script will "
        f"not guess. Read internal/loop/turn.go and teach this file the tool and the cause.",
        "",
    ]


def read_round_lines(head, tool_call, out_tok, supplementary_budget):
    """The model-call output line and the tool-call step for a served addressed read.

    An addressed read is a Graph.Node fetch BY ID, and rendering it through the recall branch was
    the defect this function closes: a served read printed a whole-graph recall line complete with a
    limit and a scope, none of which the round had. turn.go's four refusals of a read are caught by
    loop_refusal before the dispatch reaches here, so any error still present is one this script
    cannot name and goes to the unrecognised arm rather than to a story about a fetch.
    """
    if tool_call.get("error"):
        return unrecognised_round_lines(tool_call, out_tok)

    node_id = tool_call.get("nodeId")
    if node_id is None:
        return unrecognised_round_lines(
            tool_call,
            out_tok,
            reason="the tool is the addressed read but the record carries no id for the round, and "
            "a read with no id cannot be narrated as a fetch of anything",
        )

    results = tool_call.get("results") or []
    kept = sum(1 for r in results if r.get("included"))
    lines = [
        f"{'':<24} output: {round_wanted(tool_call)}. out={out_tok}",
        "",
        f"{head('tool call')} input: read_node(id={node_id}) -- one part fetched by "
        f"the id the model named, not a search",
    ]
    lines.append(
        f"{'':<24} output: {len(results)} row(s) returned, {kept} admitted ({added_phrase(results)}), "
        f"within at most {fmt_bytes(supplementary_budget or 0)}"
    )
    if results:
        lines.append(
            f"{'':<24} note: an addressed read is ONE Graph.Node handed straight to admit (turn.go's "
            f"dispatchRead) -- no query, no ranking, no fusion, and a relevance floor of zero, so "
            f"the row is here because the model named its id, not because it scored. It carries no "
            f"recall sources and none is shown for it."
        )
    lines.extend(render_candidate_table(results, supplementary_budget))
    lines.append("")
    return lines


def recall_round_lines(head, tool_call, out_tok, limits, candidate_limit, supplementary_budget):
    """The model-call output line and, where one happened, the tool-call step for a recall round."""
    category = classify_round(tool_call)
    if category == ROUND_REFUSED:
        return refused_round_lines(tool_call, out_tok, loop_refusal(tool_call))
    if category == ROUND_UNRECOGNISED:
        return unrecognised_round_lines(tool_call, out_tok)

    lines = []
    if category == ROUND_CAPPED:
        lines.append(
            f"{'':<24} output: {round_wanted(tool_call)}, but the model-call cap "
            f"(MaxModelCalls={limits.get('maxModelCalls')}) was reached -- NOT dispatched, counted "
            f"only. ARCHIVE SHAPE: the cause this reads is one turn.go no longer writes. out={out_tok}"
        )
    elif category == ROUND_DISPATCH_FAILED:
        lines.append(
            f"{'':<24} output: {round_wanted(tool_call)}, dispatched, but the supplementary recall "
            f"call itself FAILED with its reason scrubbed off this surface. ARCHIVE SHAPE: the cause "
            f"this reads is one turn.go no longer writes. out={out_tok}"
        )
    else:  # ROUND_DISPATCHED
        lines.append(f"{'':<24} output: {round_wanted(tool_call)}. out={out_tok}")
    lines.append("")

    if category != ROUND_DISPATCHED:
        return lines

    results = tool_call.get("results") or []
    kept = sum(1 for r in results if r.get("included"))
    lines.append(
        f"{head('tool call')} input: recall(query={tool_call.get('query')!r}, "
        f"limit={candidate_limit}, scope=nil -- whole graph, deliberately unscoped)"
    )
    lines.append(
        f"{'':<24} output: {len(results)} candidate(s) returned, {kept} admitted "
        f"({added_phrase(results)}), within at most {fmt_bytes(supplementary_budget or 0)}"
    )
    if results:
        lines.append(
            f"{'':<24} note: a supplementary round is ONE unscoped Graph.Recall handed "
            f"straight to admit (turn.go's dispatchRecall) -- no second query, no scope, no "
            f"reserve and no fusion -- so these rows carry no recall sources and none is "
            f"shown for them. That is the round's construction, not a gap in this record."
        )
    lines.extend(render_candidate_table(results, supplementary_budget))
    lines.append("")
    return lines


def reserved_call_phrase(record):
    """The turn's reserved answering call, as the record spells its state.

    The record carries `reservedCall` only since the loop started reserving one, and the three
    states it can take are the only thing that separates a turn that answered from its reserved call
    from one that never made it. Absent means the record predates the field, which is stated rather
    than read as any of the three.
    """
    reserved = record.get("reservedCall")
    if not isinstance(reserved, dict):
        return "reservedCall absent -- the record predates the field"
    state = reserved.get("state")
    error = reserved.get("error")
    return f"reservedCall={state!r}" + (f" ({error!r})" if error else "")


def write_round_lines(head, tool_call, out_tok, limits):
    """The model-call output line and, where one happened, the tool-call step for a write round.

    A write round is NOT rendered through the recall branch, and that is the whole point of the
    `tool` field: without it a write round with an empty `error` classifies as ROUND_DISPATCHED and
    prints as "wants recall (query=None)" followed by a fabricated recall step -- the same class of
    defect the fabricated recall step was, one tool along.
    """
    path = tool_call.get("path")
    size = tool_call.get("bytes", 0)
    category = classify_write_round(tool_call)
    lines = []

    if category == WRITE_UNRECOGNISED:
        return unrecognised_round_lines(tool_call, out_tok)

    if category != WRITE_CAPPED and not path:
        return unrecognised_round_lines(
            tool_call,
            out_tok,
            reason="the tool is the file write but the record carries no path for the round, and a "
            "write with no path cannot be narrated as a file",
        )

    if category == WRITE_CAPPED:
        lines.append(
            f"{'':<24} output: {round_wanted(tool_call)}, but the model-call cap "
            f"(MaxModelCalls={limits.get('maxModelCalls')}) was reached -- NOT dispatched, counted "
            f"only. Nothing was written. ARCHIVE SHAPE: the cause this reads is one turn.go no "
            f"longer writes. out={out_tok}"
        )
        lines.append("")
        return lines

    lines.append(f"{'':<24} output: {round_wanted(tool_call)}. out={out_tok}")
    lines.append("")
    lines.append(f"{head('tool call')} input: write_file(path={path!r}, {fmt_bytes(size)})")

    if category == WRITE_ACCEPTED:
        lines.append(
            f"{'':<24} output: ACCEPTED -- {fmt_bytes(size)} written to {path!r} inside the run's "
            f"working directory. A FILE NOW EXISTS ON DISK."
        )
    elif category == WRITE_REFUSED:
        lines.append(
            f"{'':<24} output: REFUSED by the path rules -- {tool_call.get('error')!r}. Nothing was "
            f"written. The model is shown this sentence and may try again."
        )
    elif category == WRITE_UNCONFIGURED:
        lines.append(
            f"{'':<24} output: REFUSED -- {tool_call.get('error')!r}. The service was started "
            f"without PROCESSOR_WORKSPACE_DIR, so no run can write anything; this is an operator "
            f"condition, not the model's."
        )
    else:  # WRITE_FAILED
        lines.append(
            f"{'':<24} output: the write reached the working directory and FAILED, with its reason "
            f"scrubbed off this surface and left in the server's stderr log. Nothing was written. "
            f"ARCHIVE SHAPE: the cause this reads is one turn.go no longer writes."
        )

    lines.append("")
    return lines


def workspace_lines(record):
    """What the run did to disk, named once, where a reader looking for the file will find it."""
    workspace = record.get("workspace")
    writes = [tc for tc in (record.get("toolCalls") or []) if round_tool(tc) == TOOL_WRITE_FILE]
    if not writes and not workspace:
        return []

    accepted = [tc for tc in writes if classify_write_round(tc) == WRITE_ACCEPTED]
    lines = [RULE]
    if workspace:
        lines.append(f"WORKSPACE  {workspace}")
    else:
        lines.append("WORKSPACE  none -- no working directory was ever opened for this run")
    lines.append(
        f"           {len(writes)} write attempt(s), {len(accepted)} accepted. "
        + (
            "Files on disk: " + ", ".join(repr(tc.get("path")) for tc in accepted)
            if accepted
            else "NO FILE WAS WRITTEN."
        )
    )
    return lines


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
        f"-- a repeated input is not a second measurement of anything. Running anyway, "
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


def run_one(model_url, model_id, model_key, divoid_url, divoid_key, task_text, subject, temperature, workspace):
    env = apply_temperature(
        harness.child_env(model_url, model_id, model_key, divoid_url, divoid_key), temperature
    )
    env[ENV_WORKSPACE_DIR] = workspace
    print(f"workspace {workspace} (created here, never deleted -- look in it for what the run wrote)")

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
        "--workspace", default=None,
        help="root the run's working directory is created beneath; defaults to a fresh directory "
             "this script creates and never deletes. Whatever the run writes lands under it and "
             "survives the process, which is what makes 'did a file appear?' answerable",
    )
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
        workspace = args.workspace or tempfile.mkdtemp(prefix="processor-workspace-")
        trace = run_one(
            args.model_url, args.model_id, args.model_key,
            divoid_url, divoid_key, args.input, args.subject, args.temperature, workspace,
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
