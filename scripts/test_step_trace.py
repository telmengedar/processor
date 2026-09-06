#!/usr/bin/env python3
"""Unit tests for the pure functions in step_trace.py -- classify_round, render_candidate_table,
render_trace, announce_written -- run offline, over plain dicts, no build/server/model/graph.

    python scripts/test_step_trace.py
    python -m unittest scripts.test_step_trace -v

Written after review rejected the first version of this tool for two critical defects (DiVoid
review, 2026-09-05):

C1: a toolCalls[i] entry whose `error` is a ToolError message (the model's tool call was itself
malformed -- bad JSON, an empty query) was rendered as a normal dispatched round with `query=None`
and 0 results, discarding the record's own error string. A reader would conclude "the model asked
the graph and the graph had nothing" when the truth is "the model's tool call never reached the
graph". MalformedToolRequestUncappedTests and MalformedToolRequestCappedTests below are built directly on that shape
-- the reviewer noted a test like this "would have caught C1 outright".

C2/C3: the trace claimed "the anchor is exempt from the assembly budget" and tested a candidate's
admissibility against the raw assemblyByteBudget constant. `internal/loop/assemble.go` charges the
anchor's bytes against the budget before any candidate is considered (`remaining := budget -
len(anchor.Content)`, floored at zero) -- it is exempt from being CUT, not from being CHARGED.
ShutoutOversizedAnchorTests and UnadmittableUsesRemainingBudgetTests pin the corrected arithmetic.

The sampling classes were added the round the harness gained PROCESSOR_MODEL_TEMPERATURE and
PROCESSOR_MODEL_TOP_P, which turned the script's printed paragraph about sampling from true into
false with no test failing anywhere -- because no test read the printed prose. The rule that follows
from it, and that SamplingLineTests exists to keep: prose this script PRINTS is tested by reading the
printed output, including the shape where the record predates the field and no value may be invented.

The sharper half of that rule, learned the round after, when a suite obeying it still shipped a
renderer that rounded the number it claimed to quote: A TEST THAT ONLY EXERCISES THE LINE PROVES
NOTHING. When you pin a formatting or predicate decision, pick the fixture that DISCRIMINATES the
shipped implementation from its most likely wrong neighbour. Every sampling value here was once 0,
0.7, 0.91 or 0.5 -- all fixed points of six-significant-figure rounding, so nothing could separate
`f"{value:g}"` from an honest formatter; every sampling object was once None or truthy, so nothing
could separate `is not None` from truthiness. Both suites were green and both were one fixture choice
away. Where a class pins such a decision, its docstring names the neighbour the fixture rules out.

The third thing this suite now exists to catch, learned when Disposition.Sources landed: a sentence
can be TRUE when written, reviewed as true, and made false later by a change that never opens this
directory. STEP 2's rank note declined to say whether a reserved-tail row was a neighbourhood hit,
on the stated ground that no candidate records which recall returned it -- and two commits later
every candidate did. No review could have caught it from either side.

Be exact about what this suite can and cannot do about that, because the comfortable version of the
lesson is itself a false claim. These tests run over hand-written dicts, not over the binary: adding
a field to internal/loop cannot turn any of them red, and no test below would have failed on the
commit that falsified the prose. What SourceColumnTests, SourceQueryIndexTests, AttributionNoteTests
and SupplementaryRoundSourcesTests do buy is the other half -- the printed claim is now pinned
sentence by sentence against fixtures shaped to the Go type, so it cannot be quietly reworded, and a
fixture that stops matching the type is a visible, greppable lie rather than an absence. Keeping the
two in step across languages stays a review obligation, not something this file can automate.
"""

import contextlib
import io
import unittest

import step_trace


# Fixture sentinel for `sampling`, which needs three states where a default argument gives two: the
# usual object (this constant), some other object (passed explicitly), and NO KEY AT ALL (pass None).
# A record written before the run record carried the field is a real shape this script must survive,
# and `sampling=None` is how a test asks for it.
ABSENT = object()

# internal/loop/turn.go's five run constants as the record carries them. A module constant, not an
# inline literal, so a test that needs one value different (ScopeReserveLineTests moves the candidate
# limit) can copy this and change that one key instead of restating a dict the fixture also owns.
LIMITS = {
    "candidateLimit": 20,
    "assemblyByteBudget": 60_000,
    "supplementaryByteBudget": 20_000,
    "maxModelCalls": 3,
    "maxOutputTokens": 4096,
}


def record(
    input_text="does not matter",
    subject=999,
    queries=None,
    anchor=None,
    candidates=None,
    tool_calls=None,
    model_calls=1,
    cap_reached=False,
    usage=None,
    stop_reason=None,
    limits=None,
    written=None,
    answer="the answer",
    block="x" * 10,
    sampling=ABSENT,
    workspace=None,
):
    # `is not None` throughout, deliberately -- not `x or default`: an explicitly empty dict (e.g.
    # limits={} for MissingLimitsDoesNotFabricateNumbersTests) must stay empty, not silently fall
    # back to the default just because {} is falsy. This fixture had exactly that bug once.
    rec = {
        "input": input_text,
        "subject": subject,
        "query": input_text,
        # turn.go writes Queries as []string{input} -- exactly one entry, always. The fixture carries
        # it because the src column's query index is printed only when a record has more than one,
        # and a fixture that omitted the key could not tell "one query" from "field absent".
        "queries": queries if queries is not None else [input_text],
        "anchor": anchor if anchor is not None else {"id": 1, "type": "documentation", "name": "anchor node", "size": 10, "contentHash": "abc123"},
        "candidates": candidates if candidates is not None else [],
        "block": block,
        "answer": answer,
        "model": "test-model",
        "toolCalls": tool_calls if tool_calls is not None else [],
        "modelCalls": model_calls,
        "capReached": cap_reached,
        "usage": usage if usage is not None else [{"inTokens": 100, "outTokens": 10}] * model_calls,
        "stopReason": stop_reason if stop_reason is not None else {"reason": "answered", "raw": "stop"},
        "limits": limits if limits is not None else dict(LIMITS),
        "written": written if written is not None else {"state": "stored", "nodeId": 12345},
    }
    # The default is the shape the binary writes with no sampling variable set at all: the
    # temperature variable defaults to 0 and is sent, the top_p one has no default and its key is
    # omitted. Passing sampling=None omits the object itself -- an older record, not an empty one.
    if sampling is ABSENT:
        sampling = {"temperature": 0}
    if sampling is not None:
        rec["sampling"] = sampling
    # Omitted rather than empty when no write was attempted: `workspace` is omitempty on the record,
    # so a run that wrote nothing has no key at all, and a fixture that always carried one could not
    # tell that case from a run whose directory was opened.
    if workspace is not None:
        rec["workspace"] = workspace
    return rec


def render(rec, temperature=None):
    return step_trace.render_trace(rec, "http://model.example/v1", "test-model", temperature, None)


def capture(fn, *args, **kwargs):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        fn(*args, **kwargs)
    return buf.getvalue()


def candidate(rank, node_id, sources=ABSENT, name=None, size=100, included=True):
    """One Disposition. `sources=ABSENT` omits the key entirely -- the shape of a record written
    before Disposition.Sources existed, which must render as silence and never as "no recall
    returned this row"."""
    row = {
        "rank": rank,
        "id": node_id,
        "type": "documentation",
        "name": name if name is not None else f"row {rank}",
        "similarity": 0.5,
        "size": size,
        "contentHash": "hash",
        "included": included,
    }
    if sources is not ABSENT:
        row["sources"] = sources
    return row


def unscoped(rank, query=0):
    """One entry of Disposition.sources for an unscoped recall: retrieve.go's sourcesOf stamps the
    index of the query whose list carried the row, and the 1-based rank it came back at."""
    return {"query": query, "rank": rank}


def scoped(rank):
    """One entry for the two-hop-scoped recall. `query` is 0 by construction -- sourcesOf hardcodes
    it, because retrieve.go issues the scoped recall with queries[0] however many queries there are.
    A fixture that varied it would be describing a record the loop cannot write."""
    return {"query": 0, "scoped": True, "rank": rank}


def row_for(out, node_id):
    """The one rendered table line for a node id, so a per-row assertion cannot be satisfied by some
    other row's text elsewhere in the trace."""
    matches = [line for line in out.splitlines() if f"#{node_id}" in line]
    assert len(matches) == 1, f"expected exactly one line for #{node_id}, got {len(matches)}"
    return matches[0]


class ClassifyRoundTests(unittest.TestCase):
    """classify_round is the whole fix for C1: one function, one dispatch on the exact `error`
    string, and everything not equal to the two known literals -- including empty -- has a
    well-defined answer rather than falling through to a default that assumes a dispatch happened."""

    def test_empty_error_is_dispatched(self):
        self.assertEqual(step_trace.classify_round({"error": "", "results": []}), step_trace.ROUND_DISPATCHED)

    def test_missing_error_key_is_dispatched(self):
        self.assertEqual(step_trace.classify_round({"results": []}), step_trace.ROUND_DISPATCHED)

    def test_call_cap_reached_is_capped(self):
        self.assertEqual(
            step_trace.classify_round({"query": "q", "error": "call cap reached"}), step_trace.ROUND_CAPPED
        )

    def test_supplementary_recall_failed_is_dispatch_failed(self):
        self.assertEqual(
            step_trace.classify_round({"query": "q", "error": "supplementary recall failed"}),
            step_trace.ROUND_DISPATCH_FAILED,
        )

    def test_recall_error_message_is_malformed(self):
        """The exact defect: a ToolError string (wire.go's own wording) is neither known literal
        and must resolve to ROUND_MALFORMED, not fall through to "must be a real dispatch"."""
        self.assertEqual(
            step_trace.classify_round({"error": "tool arguments could not be parsed: unexpected EOF"}),
            step_trace.ROUND_MALFORMED,
        )

    def test_empty_query_recall_error_is_malformed(self):
        self.assertEqual(
            step_trace.classify_round({"error": "tool arguments had an empty query"}),
            step_trace.ROUND_MALFORMED,
        )

    def test_unrecognised_future_error_string_is_malformed(self):
        """W1: if turn.go's literals are ever renamed, an error string this script has never seen
        must still resolve to the safe interpretation (no dispatch happened) rather than being
        silently treated as a successful round -- the exact failure mode the reviewer's W1 named."""
        self.assertEqual(
            step_trace.classify_round({"error": "some future error nobody has written yet"}),
            step_trace.ROUND_MALFORMED,
        )


class MalformedToolRequestUncappedTests(unittest.TestCase):
    """C1, non-final round: the model's tool call was malformed on round 1 of 2. No graph call was
    ever made -- dispatchRecall returns before calling Graph.Recall on this path (turn.go:230-233)."""

    def test_no_tool_call_step_is_fabricated(self):
        rec = record(
            model_calls=2,
            tool_calls=[{"error": "tool arguments could not be parsed: unexpected end of JSON input"}],
            stop_reason={"reason": "answered", "raw": "stop"},
        )
        out = render(rec)
        self.assertNotIn("STEP 5  tool call", out)
        self.assertNotIn("query=None", out)
        self.assertIn("wants recall, but the tool call itself was malformed and NEVER reached the graph", out)
        self.assertIn("tool arguments could not be parsed: unexpected end of JSON input", out)
        self.assertIn("query not recorded", out)

    def test_no_ambiguous_cap_note_when_not_the_final_call(self):
        rec = record(
            model_calls=2,
            tool_calls=[{"error": "tool arguments had an empty query"}],
            cap_reached=False,
        )
        out = render(rec)
        self.assertNotIn("malformed-request reason wins", out)


class MalformedToolRequestCappedTests(unittest.TestCase):
    """C1's sharper case: the FINAL call both wanted recall and was malformed, and capReached is
    also true on the record. turn.go's construction lets the malformed reason win outright over the
    cap reason on the same round -- the trace must say that ambiguity exists, not silently print a
    'cap reached' story invented from the top-level capReached flag."""

    def test_still_no_fabricated_dispatch_and_flags_the_ambiguity(self):
        rec = record(
            model_calls=1,
            tool_calls=[{"error": "tool arguments had an empty query"}],
            cap_reached=True,
        )
        out = render(rec)
        self.assertNotIn("STEP 5  tool call", out)
        self.assertIn("wants recall, but the tool call itself was malformed and NEVER reached the graph", out)
        self.assertIn("tool arguments had an empty query", out)
        self.assertIn("malformed-request reason wins over the cap reason", out)


class CleanCapReachedTests(unittest.TestCase):
    """The one shape that legitimately IS a clean cap: error is exactly 'call cap reached' and the
    query survives (ToolError was empty on this round, per turn.go's exchange construction)."""

    def test_cap_reached_with_real_query_and_no_tool_call_step(self):
        rec = record(
            model_calls=1,
            tool_calls=[{"query": "Pooshit organization ID", "error": "call cap reached"}],
            cap_reached=True,
        )
        out = render(rec)
        self.assertNotIn("STEP 5  tool call", out)
        self.assertIn("wants recall (query='Pooshit organization ID')", out)
        self.assertIn("NOT dispatched, counted only", out)
        self.assertNotIn("malformed", out)


class DispatchedRoundStillRendersTests(unittest.TestCase):
    """Regression guard: a genuinely dispatched round (error == "") must still get its tool-call
    step with the real candidate table -- the C1 fix must not swallow the happy path."""

    def test_real_dispatch_gets_a_tool_call_step(self):
        rec = record(
            model_calls=2,
            tool_calls=[{
                "query": "second provider",
                "results": [{"rank": 1, "id": 42, "type": "documentation", "name": "hit", "similarity": 0.8, "size": 100, "included": True}],
            }],
        )
        out = render(rec)
        self.assertIn("tool call", out)
        self.assertIn("recall(query='second provider'", out)
        self.assertIn("1 candidate(s) returned, 1 admitted", out)


class ShutoutOversizedAnchorTests(unittest.TestCase):
    """C2: an anchor bigger than the whole budget floors the remaining candidate budget to zero --
    it must NOT be described as 'anchor exempt', and every candidate should read as unadmittable."""

    def test_remaining_budget_floors_to_zero_and_shutout_fires(self):
        # block is deliberately >= the anchor's own size: renderBlock always contains the anchor in
        # full, so a block shorter than its anchor is a shape the binary cannot emit (reviewer note).
        # No candidate here is admitted, so the block need not account for any candidate bytes.
        rec = record(
            anchor={"id": 1, "type": "documentation", "name": "huge", "size": 70_660, "contentHash": "deadbeef"},
            candidates=[
                {"rank": 1, "id": 2, "type": "task", "name": "a", "similarity": 0.9, "size": 500, "included": False, "cutReason": "byte budget exceeded"},
                {"rank": 2, "id": 3, "type": "task", "name": "b", "similarity": 0.8, "size": 300, "included": False, "cutReason": "byte budget exceeded"},
            ],
            block="x" * 70_700,
        )
        out = render(rec)
        self.assertNotIn("anchor exempt", out)
        self.assertIn("CHARGED", out)
        self.assertIn("leaving 0 B for every candidate", out)
        self.assertIn("SHUTOUT", out)
        # Both cut candidates are unadmittable against a 0-byte remaining budget.
        self.assertEqual(out.count("UNADMITTABLE"), 2)


class UnadmittableUsesRemainingBudgetTests(unittest.TestCase):
    """C3, the sharp regression: a candidate that fits the raw 60,000-byte constant but not the
    anchor-adjusted remainder must still be flagged UNADMITTABLE. Anchor is 59,500 B, leaving only
    500 B; a 5,000-byte cut candidate is well under the raw budget but can never be admitted here."""

    def test_candidate_under_full_budget_but_over_remaining_is_flagged(self):
        # Same block-size note as ShutoutOversizedAnchorTests above: block must be >= the anchor's
        # own size, since renderBlock always contains the anchor in full.
        rec = record(
            anchor={"id": 1, "type": "documentation", "name": "big", "size": 59_500, "contentHash": "cafe"},
            candidates=[
                {"rank": 1, "id": 2, "type": "task", "name": "a", "similarity": 0.9, "size": 5_000, "included": False, "cutReason": "byte budget exceeded"},
            ],
            block="x" * 59_600,
        )
        out = render(rec)
        self.assertIn("leaving 500 B for every candidate", out)
        self.assertIn("UNADMITTABLE: 5000 bytes alone exceeds the 500-byte budget", out)


class MissingLimitsDoesNotFabricateNumbersTests(unittest.TestCase):
    """Reviewer note: with `limits` absent, the old anchor/assemble lines used `or 0` and printed a
    concrete, fabricated number ("consumes 4,000 B of the 0 B budget") for a field the record simply
    did not carry. Unreachable from the binary today (Limits is a value struct, always present), but
    the record is an external boundary this script should not assume never changes shape."""

    def test_anchor_note_admits_it_cannot_compute_the_charge(self):
        rec = record(
            anchor={"id": 1, "type": "documentation", "name": "a", "size": 4_000, "contentHash": "x"},
            limits={},
            block="x" * 4_100,
        )
        out = render(rec)
        self.assertIn("the anchor's charge against it cannot be computed", out)
        self.assertNotIn("of the 0 B budget", out)
        self.assertNotIn("leaving 0 B", out)


class ModelCallsZeroWithToolCallsTests(unittest.TestCase):
    """Reviewer note: a modelCalls=0 record carrying a toolCalls entry would silently iterate zero
    times and drop that round from the trace with no mention at all -- the one silent drop in the
    file. Unreachable from the loop's own control flow (every round is dispatched from inside the
    model-call loop), but flagged explicitly rather than swallowed if it ever occurs."""

    def test_inconsistency_is_reported_not_swallowed(self):
        rec = record(model_calls=0, tool_calls=[{"query": "q", "results": []}], usage=[])
        out = render(rec)
        self.assertIn("modelCalls is 0 but the record carries 1 toolCalls entry", out)


class RenderTraceUsesRecordFieldsTests(unittest.TestCase):
    """W4: the header must come from the record's own input/subject, not from whatever argv passed
    to the run that produced it -- there is no argv in this test at all, only the record."""

    def test_task_text_and_subject_come_from_the_record(self):
        rec = record(input_text="THIS EXACT TEXT CAME FROM THE RECORD", subject=555)
        out = render(rec)
        self.assertIn("THIS EXACT TEXT CAME FROM THE RECORD", out)
        self.assertIn("subject #555", out)


class AnnounceWrittenTests(unittest.TestCase):
    """W5: the written node id (or its absence) must be printable from the record alone, so the
    caller can print it before doing anything that might fail."""

    def test_prints_node_id_when_stored(self):
        rec = record(written={"state": "stored", "nodeId": 999})
        out = capture(step_trace.announce_written, rec)
        self.assertIn("#999", out)
        self.assertIn("stored", out)

    def test_prints_not_stored_when_absent(self):
        rec = record(written={"state": "notStored"})
        out = capture(step_trace.announce_written, rec)
        self.assertIn("no run record stored", out)
        self.assertIn("notStored", out)


class ScopeReserveLineTests(unittest.TestCase):
    """STEP 2's rank note, tested by reading the printed output for the same reason the sampling
    classes are: it is prose this script PRINTS about a mechanism in another language, and it went
    false once already by claiming the reserved tail slots are FILLED FROM the scoped recall.

    retrieve.go's `fuse` runs three passes, not two -- fill to `limit - reserve` from the fused list,
    take up to `reserve` unseen rows from the scoped list, then fill whatever the scope left over
    from the fused list AGAIN. These tests pin the corrected mechanism.

    It then went false a SECOND way, and this is the round that fixed it. The line used to close by
    refusing to say whether a tail row was a neighbourhood hit, on the stated ground that no
    candidate records which recall returned it. Two commits after that sentence shipped,
    Disposition.Sources landed and recorded exactly that -- so the refusal became the false claim,
    and nothing could have caught it: the review that passed this file predated the field, and the
    change that added the field never opened scripts/. SourceColumnTests and AttributionNoteTests
    below pin what replaced it; the surviving test here pins that the decline itself is gone."""

    def test_reserved_slots_are_not_claimed_to_come_from_the_scoped_recall(self):
        out = render(record())
        self.assertIn("RESERVED FOR a second recall scoped to the anchor's two-hop neighbourhood", out)
        self.assertIn("which is not the same as filled from it", out)
        self.assertIn("from the unscoped list AGAIN", out)
        self.assertNotIn("reserved for and backfilled from", out)

    def test_the_decline_the_record_no_longer_justifies_is_gone(self):
        """Every phrase asserted absent here was printed on every trace this script produced, and
        every one of them is now contradicted by Disposition.Sources."""
        out = render(record())
        self.assertNotIn("no candidate records which recall returned it", out)
        self.assertNotIn("may be a neighbourhood hit or a similarity continuation", out)
        self.assertNotIn("this trace will not guess which", out)

    def test_the_fused_order_is_the_unscoped_order_minus_the_anchor(self):
        """The other thing that changed under this file: fuse seeds its seen-set with the anchor id,
        so the head rows are the unscoped recall's order with the subject removed. "verbatim" was
        accurate before the anchor was excluded from its own candidate set and overclaims after."""
        out = render(record())
        self.assertIn("minus the anchor, which fuse excludes from its own candidate set", out)
        self.assertNotIn("plain-similarity order, verbatim", out)

    def test_split_point_is_computed_from_the_record_not_hardcoded(self):
        """The reserve is 3 whatever the limit is, so the boundary moves with limits.candidateLimit.
        A trace that printed 1-17 / 18-20 against a record whose limit was 10 would be describing a
        different run than the table underneath it."""
        out = render(record(limits=dict(LIMITS, candidateLimit=10)))
        self.assertIn("rows 1-7 below", out)
        self.assertIn("The last 3 slots (8-10)", out)

    def test_absent_candidate_limit_computes_nothing_and_claims_nothing(self):
        """With no limits.candidateLimit there is no boundary to name. The fallback must not fall
        back to the old claim about where the tail rows came from either."""
        out = render(record(limits={}))
        self.assertIn("the fused/scoped split point cannot be computed", out)
        self.assertNotIn("RESERVED FOR", out)
        self.assertNotIn("neighbourhood", out)


class SourceColumnTests(unittest.TestCase):
    """The src column: which recall returned each row, and at what rank, off Disposition.sources.

    Fixtures chosen to discriminate, not merely to exercise (the rule this suite's docstring states).
    The four rows are the four cases that a wrong-but-plausible renderer would collapse: a head row
    with one unscoped source, a head row returned by BOTH recalls, a reserved-slot row the scoped
    recall did return, and a reserved-slot row it did not. A renderer that printed only sources[0]
    passes on three of them and fails on the second; one that ignored `scoped` fails on the third;
    one that fired the reserved-slot callout on rank alone fails on the first; one that fired it on
    an unscoped source alone fails on the first as well; one that fired it on any reserved row fails
    on the third. The two ranks inside one row's pair differ (unscoped r4, scoped r2) so that a
    renderer reusing one source's rank for the other cannot pass either."""

    def rows(self):
        return [
            candidate(1, 101, [unscoped(1)], name="head, unscoped only"),
            candidate(4, 104, [unscoped(4), scoped(2)], name="head, returned by both recalls"),
            candidate(18, 118, [scoped(1)], name="reserved slot, neighbourhood hit"),
            candidate(19, 119, [unscoped(9)], name="reserved slot, scope did not return it"),
        ]

    def render_rows(self, **kwargs):
        return render(record(candidates=self.rows(), block="x" * 900, **kwargs))

    def test_every_source_prints_with_its_recall_and_its_own_rank(self):
        out = self.render_rows()
        self.assertIn("src unscoped r1", row_for(out, 101))
        self.assertIn("src scoped r1", row_for(out, 118))
        self.assertIn("src unscoped r9", row_for(out, 119))

    def test_a_row_returned_by_both_recalls_names_both(self):
        out = self.render_rows()
        self.assertIn("src unscoped r4, scoped r2", row_for(out, 104))

    def test_only_a_reserved_row_the_scoped_recall_missed_is_called_out(self):
        out = self.render_rows()
        self.assertEqual(out.count("RESERVED SLOT, NOT A NEIGHBOURHOOD HIT"), 1)
        self.assertIn("rank 19 is inside the range held for the scoped recall", out)

    def test_a_scoped_source_above_the_reserve_is_not_read_as_the_reserve_placing_it(self):
        """Sources is per-recall, not per-pass: rank 4 carries a scoped source because the scoped
        recall returned that node, and the unscoped order reached it first regardless."""
        out = self.render_rows()
        self.assertNotIn("rank 4 is inside", out)
        self.assertNotIn("rank 1 is inside", out)
        self.assertNotIn("rank 18 is inside", out)

    def test_the_callout_boundary_moves_with_the_records_own_candidate_limit(self):
        """Same two rows, two limits. At limit 20 the reserve starts at 18 and neither row is in it;
        at limit 10 it starts at 8 and the second row is. A hardcoded boundary cannot do both."""
        rows = [candidate(7, 107, [unscoped(7)]), candidate(8, 108, [unscoped(8)])]
        wide = render(record(candidates=rows, block="x" * 900))
        self.assertNotIn("RESERVED SLOT", wide)
        narrow = render(record(candidates=rows, block="x" * 900, limits=dict(LIMITS, candidateLimit=10)))
        self.assertEqual(narrow.count("RESERVED SLOT"), 1)
        self.assertIn("rank 8 is inside the range held for the scoped recall", narrow)

    def test_a_row_with_no_sources_is_silence_not_an_unscoped_row(self):
        """The sharp one: a reserved-slot row whose sources key is absent must NOT be called out as
        a row the scoped recall missed. Treating absence as "unscoped only" is the exact way this
        file would invent an attribution again, and rank 19 here is where it would surface."""
        out = render(record(candidates=[candidate(19, 119)], block="x" * 900))
        self.assertIn("src not recorded", row_for(out, 119))
        self.assertNotIn("RESERVED SLOT", out)


class SourceQueryIndexTests(unittest.TestCase):
    """The query index is printed only where it separates rows. sourcesOf stamps the scoped source
    with Query 0 always, and a turn issues exactly one query, so on every record the loop can write
    the index is the same constant on every source -- printed, it would read as a distinction."""

    def test_a_single_query_record_does_not_print_the_constant(self):
        out = render(record(candidates=[candidate(1, 101, [unscoped(1), scoped(2)])], block="x" * 900))
        self.assertIn("src unscoped r1, scoped r2", out)
        self.assertNotIn("unscoped q", out)

    def test_several_queries_index_the_unscoped_sources_but_not_the_scoped_one(self):
        """A shape turn.go cannot produce and the record could still carry. Here the index does
        separate the two rows, so it is printed -- for the unscoped sources only, since the scoped
        source's 0 is retrieve.go's constant rather than a query this row came back under."""
        out = render(record(
            queries=["first", "second"],
            candidates=[
                candidate(1, 101, [unscoped(1, query=0)]),
                candidate(2, 102, [unscoped(3, query=1), scoped(4)]),
            ],
            block="x" * 900,
        ))
        self.assertIn("src unscoped q0 r1", row_for(out, 101))
        self.assertIn("src unscoped q1 r3, scoped r4", row_for(out, 102))
        # Spelled against this row's own scoped rank: "scoped q0" alone is a substring of the
        # perfectly correct "unscoped q0 r1" one row up, so it cannot be asserted absent globally.
        self.assertNotIn("scoped q0 r4", row_for(out, 102))


class AttributionNoteTests(unittest.TestCase):
    """STEP 2's sentence introducing the src column, which is prose this script PRINTS about a field
    in another language -- so it is tested by reading the printed output, like the sampling lines.

    Three record states, and the fixtures separate all three: every row attributed, no row
    attributed (a record older than the field), and some rows attributed (a shape the loop cannot
    write, but the record is an external boundary). A note computed with any() prints the first
    sentence for the mixed record; one computed with all() prints the second; only counting both
    ends passes all three."""

    def test_every_row_attributed_says_so_and_still_declines_the_pass_question(self):
        out = render(record(candidates=[candidate(1, 101, [unscoped(1)])], block="x" * 900))
        self.assertIn("Which recall returned each row IS recorded", out)
        self.assertIn("does not say is which of fuse's three passes PLACED a row", out)

    def test_a_record_older_than_the_field_reports_silence_not_absence_of_recall(self):
        out = render(record(candidates=[candidate(1, 101), candidate(2, 102)], block="x" * 900))
        self.assertIn("No row below carries Disposition.sources", out)
        self.assertIn("predates the field", out)
        self.assertIn("not about the rows", out)
        self.assertNotIn("Which recall returned each row IS recorded", out)

    def test_a_partially_attributed_record_is_reported_as_neither_of_the_other_two(self):
        out = render(record(
            candidates=[candidate(1, 101, [unscoped(1)]), candidate(2, 102)],
            block="x" * 900,
        ))
        self.assertIn("1 of 2 rows below carry Disposition.sources", out)
        self.assertNotIn("Which recall returned each row IS recorded", out)
        self.assertNotIn("No row below carries Disposition.sources", out)

    def test_an_empty_candidate_set_introduces_no_column(self):
        """There are no rows to read, so a sentence about how to read them is noise -- and a note
        that fired here would describe an empty table as unattributed."""
        out = render(record())
        self.assertNotIn("Disposition.sources", out)
        self.assertNotIn("src column", out)


class SupplementaryRoundSourcesTests(unittest.TestCase):
    """A supplementary round carries no sources at all: dispatchRecall hands one unscoped
    Graph.Recall straight to admit, and only Retrieve attributes. The absence is the round's
    construction, so these rows get no src field and the trace says why -- rendering them as
    "not recorded" would report the loop's design as a gap in the record."""

    def dispatched(self, results):
        return record(model_calls=2, candidates=[], block="x" * 900,
                      tool_calls=[{"query": "second provider", "results": results}])

    def test_supplementary_rows_carry_no_src_field_and_the_absence_is_explained(self):
        out = render(self.dispatched([candidate(1, 42, name="hit")]))
        self.assertNotIn("src ", out)
        self.assertIn("no recall sources and none is shown for them", out)
        self.assertIn("That is the round's construction, not a gap in this record.", out)

    def test_an_empty_supplementary_round_explains_nothing_because_there_is_nothing_to_explain(self):
        out = render(self.dispatched([]))
        self.assertIn("0 candidate(s) returned", out)
        self.assertNotIn("no recall sources", out)


class SamplingLineTests(unittest.TestCase):
    """The SAMPLING line is prose this script PRINTS, so it is tested by reading the printed output
    -- the whole reason this round exists is that a printed paragraph about sampling went false while
    nothing asserted a word of it. Each case pins one record shape and the claim it may make."""

    def test_temperature_zero_prints_its_scope_and_claims_no_repeatability(self):
        out = render(record())
        self.assertIn("SAMPLING: temperature 0,", out)
        self.assertIn("top_p not sent", out)
        self.assertIn("REQUESTED, AS SENT -- not what the endpoint applied", out)
        self.assertIn("stops being a source of run-to-run variation", out)
        self.assertIn("not a claim that this run repeats", out)

    def test_both_values_render_and_greedy_note_is_withheld(self):
        out = render(record(sampling={"temperature": 0.7, "topP": 0.91}))
        self.assertIn("SAMPLING: temperature 0.7, top_p 0.91.", out)
        self.assertNotIn("greedy decoding", out)

    def test_top_p_alone_says_temperature_was_not_sent(self):
        out = render(record(sampling={"topP": 0.5}))
        self.assertIn("SAMPLING: temperature not sent, top_p 0.5.", out)

    def test_empty_sampling_object_says_nothing_was_sent_and_invents_no_value(self):
        """`sampling: {}` is a run made with neither parameter on the wire. Rendering that as
        "temperature 0" would report a number the endpoint was never asked for."""
        out = render(record(sampling={}))
        self.assertIn("SAMPLING: nothing sent", out)
        self.assertNotIn("temperature 0", out)

    def test_record_without_the_field_neither_crashes_nor_fabricates(self):
        """A record written before the run record carried `sampling` at all. It must render (the
        trace still completes to its RESULT line) and must say the value is unavailable rather than
        printing the default the current binary would have used."""
        out = render(record(sampling=None))
        self.assertIn("no sampling object at all", out)
        self.assertIn("does not invent it", out)
        self.assertNotIn("temperature 0", out)
        self.assertIn("RESULT", out)


class SamplingNumberFormatTests(unittest.TestCase):
    """The SAMPLING line quotes numbers, so the quoting itself needs a test that reads the printed
    output -- with values where a plausible formatter and an exact one disagree.

    `f"{value:g}"` rounds to six significant figures. Every value in the other classes (0, 0.7, 0.91,
    0.5) is unchanged by that rounding, so it renders identically under both and no assertion
    anywhere in this file could tell them apart. These two values can: 0.123456789 rounds to
    0.123457, and 1234567.0 becomes 1.23457e+06. Numbers that were never sent, printed by the line
    whose claim is what was sent -- and the MISMATCH guard cannot catch it, because it compares
    floats and the floats are equal."""

    def test_a_long_decimal_is_quoted_whole_not_rounded_to_six_figures(self):
        out = render(record(sampling={"temperature": 0.123456789}))
        self.assertIn("temperature 0.123456789,", out)
        self.assertNotIn("0.123457,", out)

    def test_a_large_value_is_not_reformatted_into_scientific_notation(self):
        """1234567, not 1234567.0: Go marshals float64(1234567) as the JSON number `1234567`, which
        decodes to a Python int, so the float spelling is a record shape the binary cannot write. The
        int discriminates just as sharply -- `:g` reformats it to 1.23457e+06 all the same, and
        repr(float(value)) would print 1234567.0 -- while also being a record that can exist."""
        out = render(record(sampling={"temperature": 1234567}))
        self.assertIn("temperature 1234567,", out)
        self.assertNotIn("1.23457e+06", out)
        self.assertNotIn("1234567.0", out)

    def test_an_integer_zero_keeps_the_spelling_the_record_carries(self):
        """The binary marshals a temperature of 0 as the JSON number `0`, which decodes to a Python
        int. Converting to float before rendering would print 0.0 -- a spelling the record does not
        contain, in a line that exists to quote the record."""
        out = render(record(sampling={"temperature": 0}))
        self.assertIn("SAMPLING: temperature 0,", out)
        self.assertNotIn("temperature 0.0", out)

    def test_a_non_numeric_value_is_not_presented_as_a_number(self):
        """The discriminator between repr and str, and the only one there is: in Python 3
        `str(x) == repr(x)` for every numeric value, so no number in this class can tell an f-string
        cleanup (`f"{value}"`) apart from the shipped `repr(value)` -- and that cleanup is the single
        most likely accidental regression here. A non-numeric value separates them: repr shows a
        string AS a string, temperature '0.7' with quotes, where str prints temperature 0.7 and
        claims a number was sent that was not. Unreachable from today's *float64 field; pinned
        because it is the only assertion in this file that can see the difference."""
        out = render(record(sampling={"temperature": "0.7"}))
        self.assertIn("temperature '0.7',", out)

    def test_a_float_that_cannot_be_written_exactly_is_printed_honestly(self):
        """0.1 + 0.2 is 0.30000000000000004 and that is what was sent. The ugly spelling is the true
        one; a renderer that tidied it to 0.3 would be reporting a request nobody made."""
        out = render(record(sampling={"temperature": 0.1 + 0.2}))
        self.assertIn("temperature 0.30000000000000004,", out)


class SamplingFlagTests(unittest.TestCase):
    """--temperature is now sent, so the trace may report it -- but only beside the record's own
    account, never instead of it. A disagreement between the two is the interesting case: it means
    the request did not arrive, which is exactly the failure this round was called in to end."""

    def test_flag_is_reported_beside_a_record_that_agrees_with_it(self):
        out = render(record(sampling={"temperature": 0.7}), temperature=0.7)
        self.assertIn("--temperature 0.7 was passed to this script", out)
        self.assertIn("PROCESSOR_MODEL_TEMPERATURE", out)
        self.assertIn("read back rather than restated from the flag", out)
        self.assertNotIn("MISMATCH", out)

    def test_record_disagreeing_with_the_flag_is_flagged(self):
        out = render(record(sampling={"temperature": 0}), temperature=0.7)
        self.assertIn("MISMATCH", out)
        self.assertIn("trust the record, not the flag", out)

    def test_older_record_reports_the_flag_but_diagnoses_no_mismatch(self):
        """A record that predates the field cannot disagree with the flag -- it can only fail to
        say. MISMATCH diagnoses "the request did not reach the client", which here would be a wrong
        diagnosis of a record that is merely silent, so the flag is reported and nothing is called."""
        out = render(record(sampling=None), temperature=0.7)
        self.assertIn("no sampling object at all", out)
        self.assertIn("--temperature 0.7 was passed to this script", out)
        self.assertNotIn("MISMATCH", out)

    def test_sampling_object_missing_the_temperature_key_is_a_real_mismatch(self):
        """The narrower case that must still fire: the record DOES report what was sent, and what it
        reports is that no temperature went on the wire -- so a flag that asked for one did not
        arrive. Guarding the whole comparison on the object's presence must not lose this."""
        out = render(record(sampling={"topP": 0.9}), temperature=0.7)
        self.assertIn("MISMATCH", out)

    def test_an_empty_sampling_object_with_a_flag_is_still_a_mismatch(self):
        """The fixture that discriminates `is not None` from truthiness, and the only one that can:
        `{}` is a sampling object that affirmatively reports nothing was sent, so a flag that asked
        for a temperature did not arrive and MISMATCH must fire. Every other record here is either
        None or truthy, so `if sampling and ...` renders identically for all of them and no other
        assertion in this file could tell the shipped guard from that mutation."""
        out = render(record(sampling={}), temperature=0.7)
        self.assertIn("SAMPLING: nothing sent", out)
        self.assertIn("MISMATCH", out)

    def test_no_flag_prints_no_flag_line(self):
        self.assertNotIn("--temperature", render(record()))


class ApplyTemperatureTests(unittest.TestCase):
    """The other half of the round: the flag has to actually reach the binary, which reads the
    environment and nothing else. These run over a plain dict -- no child process, no server."""

    def test_no_flag_leaves_the_environment_untouched(self):
        self.assertEqual(step_trace.apply_temperature({"PROCESSOR_MODEL_ID": "m"}, None), {"PROCESSOR_MODEL_ID": "m"})

    def test_no_flag_keeps_an_inherited_value(self):
        """child_env starts from os.environ.copy(), so an exported value already reaches the child.
        The flag's absence must not overwrite it with this script's own idea of a default."""
        env = step_trace.apply_temperature({"PROCESSOR_MODEL_TEMPERATURE": "0.9"}, None)
        self.assertEqual(env["PROCESSOR_MODEL_TEMPERATURE"], "0.9")

    def test_flag_sets_the_variable_the_binary_actually_reads(self):
        """The literal name is asserted here, not step_trace's constant: the constant matching itself
        proves nothing, and this string is the entire contract with internal/boot."""
        env = step_trace.apply_temperature({}, 0.7)
        self.assertEqual(env["PROCESSOR_MODEL_TEMPERATURE"], "0.7")

    def test_flag_overrides_an_inherited_value(self):
        env = step_trace.apply_temperature({"PROCESSOR_MODEL_TEMPERATURE": "0.9"}, 0)
        self.assertEqual(env["PROCESSOR_MODEL_TEMPERATURE"], "0.0")


class RoundToolTests(unittest.TestCase):
    """`tool` is what separates a write round from a recall round; a record older than the field
    carries none, and every round in such a record was a recall."""

    def test_a_missing_tool_key_reads_as_recall(self):
        self.assertEqual(step_trace.round_tool({"query": "q"}), step_trace.TOOL_RECALL)

    def test_the_record_spelling_of_the_write_tool_is_not_the_wire_spelling(self):
        self.assertEqual(step_trace.round_tool({"tool": "writeFile"}), step_trace.TOOL_WRITE_FILE)
        self.assertNotEqual(step_trace.TOOL_WRITE_FILE, "write_file")


class ClassifyWriteRoundTests(unittest.TestCase):
    """Six shapes, separated by the exact `error` string, because they answer "did a file appear?"
    differently and a reader cannot tell them apart from the result count."""

    def test_empty_error_is_accepted(self):
        self.assertEqual(step_trace.classify_write_round({"error": ""}), step_trace.WRITE_ACCEPTED)

    def test_the_cap_literal_is_capped(self):
        self.assertEqual(
            step_trace.classify_write_round({"error": "call cap reached"}), step_trace.WRITE_CAPPED
        )

    def test_the_scrubbed_failure_literal_is_a_failed_write(self):
        self.assertEqual(
            step_trace.classify_write_round({"error": "file write failed"}), step_trace.WRITE_FAILED
        )

    def test_the_unconfigured_literal_is_its_own_shape(self):
        self.assertEqual(
            step_trace.classify_write_round({"error": "no working directory is configured"}),
            step_trace.WRITE_UNCONFIGURED,
        )

    def test_a_path_rule_refusal_is_refused(self):
        self.assertEqual(
            step_trace.classify_write_round(
                {"error": "write rejected: path must not leave the working directory"}
            ),
            step_trace.WRITE_REFUSED,
        )

    def test_anything_else_never_reached_the_working_directory(self):
        self.assertEqual(
            step_trace.classify_write_round({"error": "tool arguments could not be parsed"}),
            step_trace.WRITE_MALFORMED,
        )


class WriteRoundRenderingTests(unittest.TestCase):
    """The defect this replaces: without the `tool` branch, a write round with an empty `error`
    classifies as ROUND_DISPATCHED and prints "wants recall (query=None)" plus a recall step that
    never happened -- C1 one tool along."""

    def accepted_record(self):
        return record(
            model_calls=2,
            tool_calls=[{"tool": "writeFile", "path": "index.html", "bytes": 452, "results": []}],
            workspace="/runs/run-1",
        )

    def test_an_accepted_write_is_not_narrated_as_a_recall(self):
        out = render(self.accepted_record())
        self.assertNotIn("wants recall", out)
        self.assertNotIn("recall(query=", out)

    def test_an_accepted_write_names_the_path_the_bytes_and_that_a_file_exists(self):
        out = render(self.accepted_record())
        self.assertIn("write_file(path='index.html'", out)
        self.assertIn("452", out)
        self.assertIn("A FILE NOW EXISTS ON DISK", out)

    def test_a_refusal_is_shown_verbatim_and_says_nothing_was_written(self):
        rec = record(
            model_calls=2,
            tool_calls=[{
                "tool": "writeFile",
                "path": "../escape.html",
                "bytes": 3,
                "error": "write rejected: path must not leave the working directory",
                "results": [],
            }],
            workspace="/runs/run-1",
        )
        out = render(rec)
        self.assertIn("REFUSED by the path rules", out)
        self.assertIn("path must not leave the working directory", out)
        self.assertIn("NO FILE WAS WRITTEN.", out)

    def test_an_unconfigured_service_is_named_as_an_operator_condition(self):
        rec = record(
            model_calls=2,
            tool_calls=[{
                "tool": "writeFile",
                "path": "index.html",
                "bytes": 3,
                "error": "no working directory is configured",
                "results": [],
            }],
        )
        out = render(rec)
        self.assertIn("PROCESSOR_WORKSPACE_DIR", out)
        self.assertIn("WORKSPACE  none", out)

    def test_a_capped_write_round_gets_no_tool_call_step(self):
        rec = record(
            model_calls=3,
            cap_reached=True,
            tool_calls=[
                {"tool": "writeFile", "path": "a.html", "bytes": 3, "results": []},
                {"tool": "writeFile", "path": "b.html", "bytes": 3, "results": []},
                {"tool": "writeFile", "path": "c.html", "bytes": 3, "error": "call cap reached", "results": []},
            ],
            workspace="/runs/run-1",
        )
        out = render(rec)
        self.assertIn("NOT dispatched, counted only", out)
        self.assertEqual(out.count("write_file(path="), 2)

    def test_a_malformed_write_round_gets_no_tool_call_step(self):
        rec = record(
            model_calls=2,
            tool_calls=[{
                "tool": "writeFile",
                "error": "tool arguments could not be parsed: unexpected EOF",
                "results": [],
            }],
        )
        out = render(rec)
        self.assertIn("NEVER reached the working directory", out)
        self.assertNotIn("write_file(path=", out)


class WorkspaceLineTests(unittest.TestCase):
    """Where the file is, named once, because the record carries no file content and a reader who
    wants to check the claim has to go and look."""

    def test_a_run_with_no_write_round_prints_no_workspace_section(self):
        self.assertNotIn("WORKSPACE", render(record()))

    def test_the_directory_and_the_accepted_paths_are_both_named(self):
        rec = record(
            model_calls=2,
            tool_calls=[{"tool": "writeFile", "path": "index.html", "bytes": 452, "results": []}],
            workspace="/runs/run-1",
        )
        out = render(rec)
        self.assertIn("WORKSPACE  /runs/run-1", out)
        self.assertIn("1 write attempt(s), 1 accepted", out)
        self.assertIn("Files on disk: 'index.html'", out)


class MixedRoundTests(unittest.TestCase):
    """A recall round and a write round in one run must each be narrated as themselves; a renderer
    that read the tool of the first round and applied it to both would pass every test above."""

    def test_each_round_is_narrated_as_its_own_tool(self):
        rec = record(
            model_calls=3,
            tool_calls=[
                {"tool": "recall", "query": "the missing thing", "bytes": 0, "results": []},
                {"tool": "writeFile", "path": "index.html", "bytes": 452, "results": []},
            ],
            workspace="/runs/run-1",
        )
        out = render(rec)
        self.assertIn("recall(query='the missing thing'", out)
        self.assertIn("write_file(path='index.html'", out)


if __name__ == "__main__":
    unittest.main()
