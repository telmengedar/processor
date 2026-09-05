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

The sampling classes were added the round the harness gained PROCESSOR_MODEL_TEMPERATURE and
PROCESSOR_MODEL_TOP_P, which turned the script's printed paragraph about sampling from true into
false with no test failing anywhere -- because no test read the printed prose. The rule that follows
from it, and that SamplingLineTests exists to keep: prose this script PRINTS is tested by reading the
printed output, including the shape where the record predates the field and no value may be invented.
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
    sampling=ABSENT,
):
    # `is not None` throughout, deliberately -- not `x or default`: an explicitly empty dict (e.g.
    # limits={} for MissingLimitsDoesNotFabricateNumbersTests) must stay empty, not silently fall
    # back to the default just because {} is falsy. This fixture had exactly that bug once.
    rec = {
        "input": input_text,
        "subject": subject,
        "query": input_text,
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
        "limits": limits if limits is not None else {
            "candidateLimit": 20,
            "assemblyByteBudget": 60_000,
            "supplementaryByteBudget": 20_000,
            "maxModelCalls": 3,
            "maxOutputTokens": 4096,
        },
        "written": written if written is not None else {"state": "stored", "nodeId": 12345},
    }
    # The default is the shape the binary writes with no sampling variable set at all: the
    # temperature variable defaults to 0 and is sent, the top_p one has no default and its key is
    # omitted. Passing sampling=None omits the object itself -- an older record, not an empty one.
    if sampling is ABSENT:
        sampling = {"temperature": 0}
    if sampling is not None:
        rec["sampling"] = sampling
    return rec


def render(rec, temperature=None):
    return step_trace.render_trace(rec, "http://model.example/v1", "test-model", temperature, None)


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

    def test_older_record_with_the_flag_reports_both(self):
        out = render(record(sampling=None), temperature=0.7)
        self.assertIn("no sampling object at all", out)
        self.assertIn("--temperature 0.7 was passed to this script", out)
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


if __name__ == "__main__":
    unittest.main()
