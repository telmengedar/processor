#!/usr/bin/env python3
"""Table-driven unit tests for the pure functions in compare.py.

    python scripts/test_compare.py
    python -m unittest discover -s scripts -v

No network, no model, no graph, no subprocess: route, answer_node_report, task_stage,
node_ratio, arrived and describe_cut are pure functions over plain dicts, and every defect
raised across this script's review rounds (F1, F2, W1-W3, N1-N3, and DiVoid #11333's four
findings) was a disagreement between a branch's condition and the prose it prints, or between
two axes (arrival route vs. retrieval stage) that had been collapsed into one. Each test below
asserts the indictment text a disposition carries, not only the stage constant it compares
equal to: pinning the constant alone would have let every one of F1/F2/W1/W2 back in, since in
each of those the constant was right and the prose attached to it was wrong.

DiVoid #11333 reframed the anchor: it is not a retrieval stage but a separate arrival route,
excluded from the population every Stage describes rather than ranked within it. The tests below
that exercise this are grouped under RouteTests, AnchorAxisTests, TaskStageAnchorTests,
NodeRatioTests and ArrivedTests.
"""

import copy
import json
import pickle
import tempfile
import unittest
from pathlib import Path

import compare


def task(answer_nodes, subject=1):
    return {"id": "t", "task": "does not matter for these tests", "subject": subject, "answerNodes": answer_nodes}


def record(anchor_id=999, candidates=None, tool_calls=None, limits=None):
    return {
        "anchor": {"id": anchor_id, "type": "documentation", "name": "anchor node", "size": 10},
        "candidates": candidates or [],
        "toolCalls": tool_calls or [],
        "limits": limits or {},
    }


class RouteTests(unittest.TestCase):
    """route() is the whole axis split (DiVoid #11333): one comparison, node == anchor.id."""

    def test_anchor_route(self):
        self.assertEqual(compare.route(100, record(anchor_id=100)), compare.ROUTE_ANCHOR)

    def test_retrieval_route(self):
        self.assertEqual(compare.route(200, record(anchor_id=100)), compare.ROUTE_RETRIEVAL)

    def test_no_anchor_on_record_is_retrieval_route(self):
        rec = record()
        rec["anchor"] = {}
        self.assertEqual(compare.route(200, rec), compare.ROUTE_RETRIEVAL)


class AnswerNodeReportTests(unittest.TestCase):
    def test_primary_admitted(self):
        rec = record(candidates=[{"id": 200, "rank": 1, "included": True}])
        rows = compare.answer_node_report(task([200]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node_route, compare.ROUTE_RETRIEVAL)
        self.assertEqual(stage, compare.ADMITTED_PRIMARY)
        self.assertEqual(rank, 1)
        self.assertIsNone(detail)
        self.assertIn("reached the model in the initial context block", stage.indictment)

    def test_primary_cut_then_supplementary_recovered(self):
        """F2/W2: the node WAS in the initial candidate list (primary is not None), so the initial
        recall query worked and the candidate limit provably did not cut it -- only assembly's
        admission (byte budget or self-produced) could have. The indictment must name the budget,
        not the candidate limit, and the per-row detail must carry the real cutReason rather than
        a hardcoded guess (this record's cutReason is byte budget exceeded; a self-produced cut
        would show up here identically, which is why the top-line text hedges between the two)."""
        rec = record(
            candidates=[{"id": 300, "rank": 5, "included": False, "cutReason": "byte budget exceeded"}],
            tool_calls=[{"query": "q", "results": [{"id": 300, "rank": 1, "included": True}]}],
        )
        rows = compare.answer_node_report(task([300]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node_route, compare.ROUTE_RETRIEVAL)
        self.assertEqual(stage, compare.ADMITTED_SUPPLEMENTARY_RECOVERED)
        self.assertEqual(rank, 1)
        self.assertEqual(detail, "initial recall ranked it #5 but assembly cut it (byte budget exceeded)")
        self.assertIn("recall query worked", stage.indictment)
        # F2: the candidate limit is explicitly ruled out here (it already cannot have cut a node
        # that made it into candidates(record)) rather than being offered as a co-equal cause the
        # way the pre-fix text did ("the byte budget or candidate limit at the initial round").
        self.assertIn("never the candidate limit", stage.indictment)

    def test_genuine_supplementary_miss(self):
        """F2: the node is absent from the initial candidate list entirely (primary is None), so
        assembly never had a chance to admit or cut it -- the initial recall query missing it and
        the candidate limit truncating it before assembly saw it are indistinguishable from this
        record, and the indictment must say so rather than asserting the query 'missed it
        entirely' as the only explanation."""
        rec = record(
            candidates=[],
            tool_calls=[{"query": "q", "results": [{"id": 400, "rank": 2, "included": True}]}],
        )
        rows = compare.answer_node_report(task([400]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node_route, compare.ROUTE_RETRIEVAL)
        self.assertEqual(stage, compare.ADMITTED_SUPPLEMENTARY)
        self.assertEqual(rank, 2)
        self.assertIsNone(detail)
        self.assertIn("candidate limit", stage.indictment)
        self.assertIn("cannot tell those two apart", stage.indictment)
        self.assertNotIn("missed the answer entirely", stage.indictment)

    def test_supplementary_cut(self):
        """W1: found only by a supplementary round and cut there is a different finding from found
        and cut at the initial round -- a different budget (SupplementaryByteBudget) and a
        different candidate list -- and must not collapse into the same RETRIEVED_CUT stage."""
        rec = record(
            candidates=[],
            tool_calls=[{"query": "q", "results": [{"id": 500, "rank": 3, "included": False, "cutReason": "byte budget exceeded"}]}],
        )
        rows = compare.answer_node_report(task([500]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node_route, compare.ROUTE_RETRIEVAL)
        self.assertEqual(stage, compare.RETRIEVED_CUT_SUPPLEMENTARY)
        self.assertEqual(rank, 3)
        self.assertEqual(detail, "cut at a supplementary round: byte budget exceeded")
        self.assertIn("SupplementaryByteBudget", stage.indictment)
        self.assertNotEqual(stage, compare.RETRIEVED_CUT)

    def test_self_produced_cut(self):
        """Initial round found it, cut it as self-produced, and no supplementary round recovered
        it -- RETRIEVED_CUT, with the real cutReason (not a guessed one) in the detail."""
        rec = record(candidates=[{"id": 600, "rank": 2, "included": False, "cutReason": "self-produced"}])
        rows = compare.answer_node_report(task([600]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node_route, compare.ROUTE_RETRIEVAL)
        self.assertEqual(stage, compare.RETRIEVED_CUT)
        self.assertEqual(rank, 2)
        self.assertEqual(detail, "cut at the initial round: self-produced")
        self.assertIn("no supplementary round on this task recovered it", stage.indictment)

    def test_not_retrieved(self):
        rec = record(candidates=[], tool_calls=[])
        rows = compare.answer_node_report(task([700]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node_route, compare.ROUTE_RETRIEVAL)
        self.assertEqual(stage, compare.NOT_RETRIEVED)
        self.assertIsNone(rank)
        self.assertIsNone(detail)

    def test_no_answer_nodes_named(self):
        self.assertEqual(compare.answer_node_report(task([]), record()), [])
        self.assertEqual(compare.answer_node_report({"id": "t", "task": "x", "subject": 1}, record()), [])


class AnchorAxisTests(unittest.TestCase):
    """DiVoid #11333: route wins ahead of everything else, including when the SAME node id also
    happens to be an admitted candidate, a cut candidate, or a supplementary hit -- these are F1's
    original four cases, re-pointed at the route/stage split. All four must return route `anchor`
    with stage undefined (None): the node never went through retrieval to reach the model,
    whatever candidates(record) or a supplementary round separately say about the same id."""

    def test_anchor_also_admitted_candidate(self):
        rec = record(
            anchor_id=100,
            candidates=[{"id": 100, "rank": 1, "included": True}],
        )
        rows = compare.answer_node_report(task([100]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node, 100)
        self.assertEqual(node_route, compare.ROUTE_ANCHOR)
        self.assertIsNone(stage)
        self.assertIsNone(rank)
        self.assertIsNone(detail)

    def test_anchor_also_cut_candidate(self):
        """F1's original case: this record also carries the anchor's id as a cut candidate, which
        would (wrongly) read as RETRIEVED_CUT if route were checked after candidates."""
        rec = record(
            anchor_id=100,
            candidates=[{"id": 100, "rank": 3, "included": False, "cutReason": "byte budget exceeded"}],
        )
        rows = compare.answer_node_report(task([100]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node, 100)
        self.assertEqual(node_route, compare.ROUTE_ANCHOR)
        self.assertIsNone(stage)
        self.assertIsNone(rank)
        self.assertIsNone(detail)

    def test_anchor_also_supplementary_hit(self):
        rec = record(
            anchor_id=100,
            tool_calls=[{"query": "q", "results": [{"id": 100, "rank": 1, "included": True}]}],
        )
        rows = compare.answer_node_report(task([100]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node, 100)
        self.assertEqual(node_route, compare.ROUTE_ANCHOR)
        self.assertIsNone(stage)
        self.assertIsNone(rank)
        self.assertIsNone(detail)

    def test_anchor_absent_from_both(self):
        """The plain case: the anchor is named as an answer node and appears nowhere else in the
        record at all."""
        rec = record(anchor_id=100, candidates=[], tool_calls=[])
        rows = compare.answer_node_report(task([100]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertEqual(node, 100)
        self.assertEqual(node_route, compare.ROUTE_ANCHOR)
        self.assertIsNone(stage)
        self.assertIsNone(rank)
        self.assertIsNone(detail)


class TaskStageTests(unittest.TestCase):
    def test_unlabelled_when_no_answer_nodes(self):
        self.assertEqual(compare.task_stage(task([]), record()), compare.UNLABELLED)

    def test_precedence_recovered_beats_genuine_miss(self):
        rec = record(
            candidates=[
                {"id": 10, "rank": 1, "included": False, "cutReason": "byte budget exceeded"},
            ],
            tool_calls=[{"query": "q", "results": [
                {"id": 10, "rank": 1, "included": True},   # recovers node 10
                {"id": 20, "rank": 4, "included": True},   # node 20 never in initial candidates
            ]}],
        )
        self.assertEqual(compare.task_stage(task([20, 10]), rec), compare.ADMITTED_SUPPLEMENTARY_RECOVERED)

    def test_precedence_initial_cut_beats_supplementary_cut(self):
        rec = record(
            candidates=[
                {"id": 10, "rank": 1, "included": False, "cutReason": "byte budget exceeded"},
            ],
            tool_calls=[{"query": "q", "results": [
                {"id": 20, "rank": 1, "included": False, "cutReason": "byte budget exceeded"},
            ]}],
        )
        self.assertEqual(compare.task_stage(task([20, 10]), rec), compare.RETRIEVED_CUT)

    def test_precedence_primary_beats_supplementary_and_cuts(self):
        rec = record(
            candidates=[
                {"id": 10, "rank": 1, "included": True},
                {"id": 30, "rank": 2, "included": False, "cutReason": "byte budget exceeded"},
            ],
            tool_calls=[{"query": "q", "results": [{"id": 20, "rank": 1, "included": True}]}],
        )
        self.assertEqual(compare.task_stage(task([20, 30, 10]), rec), compare.ADMITTED_PRIMARY)


class TaskStageAnchorTests(unittest.TestCase):
    """DiVoid #11333 S1: ANCHOR is removed from STAGE_PRECEDENCE entirely, not reordered within
    it -- an anchor row carries no stage and cannot win a precedence contest it does not enter.
    These three are known mutation survivors from QA's round-four sweep (#11330 W6, extended):
    each must redden if the anchor axis leaks back into the Stage domain."""

    def test_anchor_restored_to_stage_precedence_reddens(self):
        """Survivor: if ANCHOR were put back into STAGE_PRECEDENCE ahead of the other stages (the
        pre-ruling behaviour), this task -- one node genuinely retrieved-and-cut, one node that is
        the anchor -- would report ANCHOR at the task level instead of RETRIEVED_CUT. This is the
        t1 shape: DiVoid #11319 certifies t1 as `retrieved, cut`, driven by the one node that
        actually went through retrieval; the anchor must not out-rank it."""
        rec = record(
            anchor_id=50,
            candidates=[
                {"id": 20, "rank": 2, "included": False, "cutReason": "byte budget exceeded"},
            ],
        )
        self.assertEqual(compare.task_stage(task([20, 50]), rec), compare.RETRIEVED_CUT)

    def test_anchor_counted_in_found_reddens(self):
        """Survivor: anchor counted as 'found' in the nodes ratio. Only node 20 is genuinely
        admitted; node 50 is the anchor and must leave both halves of the ratio (DiVoid #11333
        S3), so this must read 1/1, never 2/2."""
        rec = record(
            anchor_id=50,
            candidates=[{"id": 20, "rank": 1, "included": True}],
        )
        rows = compare.answer_node_report(task([20, 50]), rec)
        self.assertEqual(compare.node_ratio(rows), (1, 1))

    def test_anchor_admitted_to_attribution_gate_reddens(self):
        """Survivor: anchor treated as an admitted retrieval stage for arrived()/gating purposes
        beyond the route check itself. A node that is the anchor counts as arrived (route alone
        is sufficient, per DiVoid #11333 S2) but must never be reachable through the Stage-typed
        ADMITTED_STAGES tuple -- its stage is None, not one of the three admitted Stage values."""
        rec = record(anchor_id=50)
        rows = compare.answer_node_report(task([50]), rec)
        node, node_route, stage, rank, detail = rows[0]
        self.assertIsNone(stage)
        self.assertNotIn(stage, compare.ADMITTED_STAGES)
        self.assertTrue(compare.arrived(rows[0]))


class AnchorOnlyTaskTests(unittest.TestCase):
    """DiVoid #11333 S1: a task whose answer nodes are ALL the anchor has no retrieval-eligible
    row at all. ANCHOR_ONLY reports that as a property of the task set, not a run outcome --
    distinct from every genuine Stage and from UNLABELLED (which is for naming no nodes at all)."""

    def test_single_anchor_only_task(self):
        rec = record(anchor_id=50)
        self.assertEqual(compare.task_stage(task([50]), rec), compare.ANCHOR_ONLY)

    def test_multiple_answer_nodes_all_anchor(self):
        """Only one node can literally BE the anchor, but a task set could still name the same
        anchor id more than once (pre-dedupe) or via distinct entries that all resolve to the
        anchor route -- the eligible population is empty either way."""
        rec = record(anchor_id=50)
        self.assertEqual(compare.task_stage(task([50, 50]), rec), compare.ANCHOR_ONLY)

    def test_anchor_only_is_not_unlabelled(self):
        self.assertNotEqual(compare.ANCHOR_ONLY, compare.UNLABELLED)

    def test_anchor_only_node_ratio_is_none(self):
        rec = record(anchor_id=50)
        rows = compare.answer_node_report(task([50]), rec)
        self.assertIsNone(compare.node_ratio(rows))


class NodeRatioTests(unittest.TestCase):
    """DiVoid #11333 S3: one column, one population -- the retrieval-eligible one. Anchor rows
    leave both halves; a zero denominator is None (never printed as 0/0 or 0/1)."""

    def test_zero_answer_nodes_is_none(self):
        self.assertIsNone(compare.node_ratio([]))

    def test_t1_shape_zero_of_two(self):
        """t1 (DiVoid #11319, #11308): three named answer nodes, one of which is the anchor and
        two genuinely retrieval-eligible, neither admitted. Must read 0/2, not 0/3 and not '-'."""
        rec = record(
            anchor_id=50,
            candidates=[{"id": 10, "rank": 1, "included": False, "cutReason": "byte budget exceeded"}],
        )
        rows = compare.answer_node_report(task([10, 20, 50]), rec)
        self.assertEqual(compare.node_ratio(rows), (0, 2))

    def test_anchor_never_in_numerator(self):
        """Rejected by the ruling: counting the anchor in the numerator (t1 -> 3/3). An anchor
        admitted-looking row must not inflate `found` even though it plainly 'reached the
        model' -- reaching the model by the anchor route is not what this ratio measures."""
        rec = record(anchor_id=50, candidates=[{"id": 10, "rank": 1, "included": True}])
        rows = compare.answer_node_report(task([10, 50]), rec)
        self.assertEqual(compare.node_ratio(rows), (1, 1))


class ArrivedTests(unittest.TestCase):
    """DiVoid #11333 S2: arrived() gates the two-line split -- any() for the arrival line, all()
    for the completion line. A row is arrived if its route is anchor, or its stage is one of the
    three admitted stages; RETRIEVED_CUT / RETRIEVED_CUT_SUPPLEMENTARY / NOT_RETRIEVED did not
    arrive."""

    def test_anchor_row_arrived(self):
        rec = record(anchor_id=50)
        rows = compare.answer_node_report(task([50]), rec)
        self.assertTrue(compare.arrived(rows[0]))

    def test_admitted_row_arrived(self):
        rec = record(candidates=[{"id": 10, "rank": 1, "included": True}])
        rows = compare.answer_node_report(task([10]), rec)
        self.assertTrue(compare.arrived(rows[0]))

    def test_cut_row_did_not_arrive(self):
        rec = record(candidates=[{"id": 10, "rank": 1, "included": False, "cutReason": "byte budget exceeded"}])
        rows = compare.answer_node_report(task([10]), rec)
        self.assertFalse(compare.arrived(rows[0]))

    def test_not_retrieved_row_did_not_arrive(self):
        rec = record(candidates=[], tool_calls=[])
        rows = compare.answer_node_report(task([10]), rec)
        self.assertFalse(compare.arrived(rows[0]))

    def test_t4_shape_arrival_fires_completion_does_not(self):
        """t4: one named node is the anchor (arrived by route), the other was never retrieved
        (did not arrive). any() must be True (there is an answer worth reading), all() must be
        False (the mechanical attribution is not complete -- #7506 was never retrieved)."""
        rec = record(anchor_id=10192, candidates=[], tool_calls=[])
        rows = compare.answer_node_report(task([10192, 7506], subject=10192), rec)
        self.assertTrue(any(compare.arrived(r) for r in rows))
        self.assertFalse(all(compare.arrived(r) for r in rows))

    def test_t2_shape_both_fire(self):
        """t2: one named node is the anchor (arrived by route), the other was genuinely admitted
        (arrived by stage). Both any() and all() must be True."""
        rec = record(anchor_id=10937, candidates=[{"id": 10466, "rank": 1, "included": True}])
        rows = compare.answer_node_report(task([10937, 10466], subject=10937), rec)
        self.assertTrue(any(compare.arrived(r) for r in rows))
        self.assertTrue(all(compare.arrived(r) for r in rows))

    def test_neither_fires_when_nothing_arrived(self):
        rec = record(candidates=[], tool_calls=[])
        rows = compare.answer_node_report(task([10, 20]), rec)
        self.assertFalse(any(compare.arrived(r) for r in rows))
        self.assertFalse(all(compare.arrived(r) for r in rows))


class DescribeCutTests(unittest.TestCase):
    def test_missing_assembly_byte_budget(self):
        """N.B. this record carries no limits.assemblyByteBudget at all (budget=None) -- describe_cut
        must say it cannot tell an individually-oversized row from a cumulative-overflow one,
        rather than guessing either."""
        row = {"id": 900, "size": 4096, "cutReason": compare.CUT_BYTE_BUDGET}
        message = compare.describe_cut(row, None)
        self.assertIn("carries no assemblyByteBudget", message)
        self.assertIn("cannot be said from here", message)

    def test_individually_oversized(self):
        row = {"id": 901, "size": 5000, "cutReason": compare.CUT_BYTE_BUDGET}
        message = compare.describe_cut(row, 4000)
        self.assertIn("exceeds the 4000-byte assembly budget", message)
        self.assertIn("regardless of rank", message)

    def test_cumulative_cut(self):
        row = {"id": 902, "size": 100, "cutReason": compare.CUT_BYTE_BUDGET}
        message = compare.describe_cut(row, 4000)
        self.assertIn("fits the budget alone but not after", message)

    def test_boundary_size_equals_budget_is_not_oversized(self):
        """Survivor from QA's round-four mutation sweep: mutating the strict '>' to '>=' in
        describe_cut survived every existing test. size == budget fits the budget alone -- a
        strict inequality is required to call a row individually oversized -- so this must land
        in the cumulative-cut branch, not the oversized one."""
        row = {"id": 904, "size": 4000, "cutReason": compare.CUT_BYTE_BUDGET}
        message = compare.describe_cut(row, 4000)
        self.assertIn("fits the budget alone but not after", message)
        self.assertNotIn("exceeds the", message)

    def test_self_produced(self):
        row = {"id": 903, "size": 10, "cutReason": compare.CUT_SELF_PRODUCED}
        message = compare.describe_cut(row, 4000)
        self.assertIn("self-produced", message)
        self.assertIn("before the byte budget is even consulted", message)


class SupplementaryByIdTests(unittest.TestCase):
    def test_included_wins_across_rounds(self):
        """Survivor from QA's round-four mutation sweep: mutating _supplementary_by_id to
        last-round-wins instead of included-wins survived every existing test. Its docstring
        promises an included disposition beats a cut one for the same id across rounds; this is
        reachable whenever recall runs twice and a node's disposition differs between rounds --
        here, round 1 includes node 42 and round 2 (later) sees it cut. Last-round-wins would
        report it as not included; included-wins must keep it included."""
        rec = record(tool_calls=[
            {"query": "q1", "results": [{"id": 42, "rank": 1, "included": True}]},
            {"query": "q2", "results": [{"id": 42, "rank": 5, "included": False, "cutReason": "byte budget exceeded"}]},
        ])
        by_id = compare._supplementary_by_id(rec)
        self.assertTrue(by_id[42]["included"])

    def test_cut_then_included_also_ends_up_included(self):
        """The reverse order of rounds: round 1 cuts it, round 2 includes it. Both orders must
        agree, since 'was it ever admitted' -- not 'what did the last round say' -- is the rule."""
        rec = record(tool_calls=[
            {"query": "q1", "results": [{"id": 42, "rank": 5, "included": False, "cutReason": "byte budget exceeded"}]},
            {"query": "q2", "results": [{"id": 42, "rank": 1, "included": True}]},
        ])
        by_id = compare._supplementary_by_id(rec)
        self.assertTrue(by_id[42]["included"])


class StagePicklingTests(unittest.TestCase):
    """One-liner from the ruling's outstanding list: Stage subclasses str and overrides __new__
    to require (label, indictment); without __getnewargs__, str's default copy/pickle protocol
    reconstructs it via __new__(cls, label) alone and TypeErrors on the missing positional
    indictment argument. Not reachable through the CLI today -- nothing here copies or pickles a
    Stage -- but it is the latent edge."""

    def test_copy_preserves_indictment(self):
        copied = copy.copy(compare.ADMITTED_PRIMARY)
        self.assertEqual(copied, compare.ADMITTED_PRIMARY)
        self.assertEqual(copied.indictment, compare.ADMITTED_PRIMARY.indictment)

    def test_pickle_roundtrip_preserves_indictment(self):
        restored = pickle.loads(pickle.dumps(compare.ADMITTED_PRIMARY))
        self.assertEqual(restored, compare.ADMITTED_PRIMARY)
        self.assertEqual(restored.indictment, compare.ADMITTED_PRIMARY.indictment)


class LoadTasksAnswerNodeDedupeTests(unittest.TestCase):
    """W3: answerNodes is now deduped at load time the same way --only already deduped its list,
    so a repeated id in the task set file cannot print twice or inflate found/len(hits)."""

    def test_duplicate_answer_nodes_are_deduped_in_order(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "tasks.json"
            path.write_text(
                json.dumps([{"id": "t1", "task": "hello", "subject": 1, "answerNodes": [5, 3, 5, 7, 3]}]),
                encoding="utf-8",
            )
            rows = compare.load_tasks(str(path), only=None)
            self.assertEqual(rows[0]["answerNodes"], [5, 3, 7])


class LoadTasksSelfReferentialWarningTests(unittest.TestCase):
    """DiVoid #11333 S4: load_tasks warns, once, on any task whose subject is among its own
    answerNodes -- a warning, not a refusal, since the run is still an honest arm comparison."""

    def _load(self, rows, only=None):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "tasks.json"
            path.write_text(json.dumps(rows), encoding="utf-8")
            import contextlib
            import io
            buf = io.StringIO()
            with contextlib.redirect_stdout(buf):
                loaded = compare.load_tasks(str(path), only=only)
            return loaded, buf.getvalue()

    def test_warns_when_subject_is_its_own_answer_node(self):
        rows = [{"id": "t1", "task": "hello", "subject": 10884, "answerNodes": [10466, 10884, 10846]}]
        loaded, out = self._load(rows)
        self.assertEqual(len(loaded), 1)
        self.assertIn("WARN", out)
        self.assertIn("t1", out)
        self.assertIn("own subject", out)

    def test_no_warning_when_subject_is_not_named(self):
        rows = [{"id": "t3", "task": "hello", "subject": 10846, "answerNodes": [11141]}]
        _, out = self._load(rows)
        self.assertEqual(out, "")

    def test_no_warning_when_task_names_no_answer_nodes(self):
        rows = [{"id": "t0", "task": "hello", "subject": 1}]
        _, out = self._load(rows)
        self.assertEqual(out, "")

    def test_warning_reflects_only_filter(self):
        """The warning is about the tasks actually selected to run, not every row in the file --
        a task excluded by --only should not appear in the warning."""
        rows = [
            {"id": "t1", "task": "self-ref", "subject": 10884, "answerNodes": [10466, 10884]},
            {"id": "t3", "task": "clean", "subject": 10846, "answerNodes": [11141]},
        ]
        _, out = self._load(rows, only="t3")
        self.assertEqual(out, "")


if __name__ == "__main__":
    unittest.main()
