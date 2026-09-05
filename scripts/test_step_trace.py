#!/usr/bin/env python3
"""Unit tests for the pure functions in step_trace.py -- classify_round, render_candidate_table,
render_trace, announce_written -- run offline, over plain dicts, no build/server/model/graph.

    python scripts/test_step_trace.py
    python -m unittest scripts.test_step_trace -v

Written after review rejected the first version of this tool for two critical defects (DiVoid
review, 2026-09-05):

C1: a toolCalls[i] entry whose `error` is a RecallError message (the model's tool call was itself
malformed -- bad JSON, an empty query) was rendered as a normal dispatched round with `query=None`
and 0 results, discarding the record's own error string. A reader would conclude "the model asked
the graph and the graph had nothing" when the truth is "the model's tool call never reached the
graph". RecallErrorUncappedTests and RecallErrorCappedTests below are built directly on that shape
-- the reviewer noted a test like this "would have caught C1 outright".

C2/C3: the trace claimed "the anchor is exempt from the assembly budget" and tested a candidate's
admissibility against the raw assemblyByteBudget constant. `internal/loop/assemble.go` charges the
anchor's bytes against the budget before any candidate is considered (`remaining := budget -
len(anchor.Content)`, floored at zero) -- it is exempt from being CUT, not from being CHARGED.
ShutoutOversizedAnchorTests and UnadmittableUsesRemainingBudgetTests pin the corrected arithmetic.
"""

import contextlib
import io
import unittest

import step_trace


def record(
    input_text="does not matter",
    subject=999,
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
):
    return {
        "input": input_text,
        "subject": subject,
        "query": input_text,
        "anchor": anchor or {"id": 1, "type": "documentation", "name": "anchor node", "size": 10, "contentHash": "abc123"},
        "candidates": candidates or [],
        "block": block,
        "answer": answer,
        "model": "test-model",
        "toolCalls": tool_calls or [],
        "modelCalls": model_calls,
        "capReached": cap_reached,
        "usage": usage if usage is not None else [{"inTokens": 100, "outTokens": 10}] * model_calls,
        "stopReason": stop_reason or {"reason": "answered", "raw": "stop"},
        "limits": limits or {
            "candidateLimit": 20,
            "assemblyByteBudget": 60_000,
            "supplementaryByteBudget": 20_000,
            "maxModelCalls": 3,
            "maxOutputTokens": 4096,
        },
        "written": written or {"state": "stored", "nodeId": 12345},
    }


def render(rec):
    return step_trace.render_trace(rec, "http://model.example/v1", "test-model", None, None)


def capture(fn, *args, **kwargs):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        fn(*args, **kwargs)
    return buf.getvalue()


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
        """The exact defect: a RecallError string (wire.go's own wording) is neither known literal
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


class RecallErrorUncappedTests(unittest.TestCase):
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


class RecallErrorCappedTests(unittest.TestCase):
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
    query survives (RecallError was empty on this round, per turn.go's exchange construction)."""

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
        rec = record(
            anchor={"id": 1, "type": "documentation", "name": "huge", "size": 70_660, "contentHash": "deadbeef"},
            candidates=[
                {"rank": 1, "id": 2, "type": "task", "name": "a", "similarity": 0.9, "size": 500, "included": False, "cutReason": "byte budget exceeded"},
                {"rank": 2, "id": 3, "type": "task", "name": "b", "similarity": 0.8, "size": 300, "included": False, "cutReason": "byte budget exceeded"},
            ],
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
        rec = record(
            anchor={"id": 1, "type": "documentation", "name": "big", "size": 59_500, "contentHash": "cafe"},
            candidates=[
                {"rank": 1, "id": 2, "type": "task", "name": "a", "similarity": 0.9, "size": 5_000, "included": False, "cutReason": "byte budget exceeded"},
            ],
        )
        out = render(rec)
        self.assertIn("leaving 500 B for every candidate", out)
        self.assertIn("UNADMITTABLE: 5000 bytes alone exceeds the 500-byte budget", out)


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


if __name__ == "__main__":
    unittest.main()
