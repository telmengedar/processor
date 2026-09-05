#!/usr/bin/env python3
"""Table-driven unit tests for the pure functions in compare.py.

    python scripts/test_compare.py
    python -m unittest scripts.test_compare -v

No network, no model, no graph, no subprocess: answer_node_report, task_stage and describe_cut
are pure functions over plain dicts, and every defect raised across this script's review rounds
(F1, F2, W1, W2, N1-N3) was a disagreement between a branch's condition and the prose it prints --
exactly what hand-feeding a synthetic record and asserting on both the stage AND its indictment
text below would have caught in round one. Each test below asserts the indictment text a
disposition carries, not only the stage constant it compares equal to: pinning the constant alone
would have let every one of F1/F2/W1/W2 back in, since in each of those the constant was right and
the prose attached to it was wrong.
"""

import json
import tempfile
import unittest
from pathlib import Path

import compare


def task(answer_nodes):
    return {"id": "t", "task": "does not matter for these tests", "subject": 1, "answerNodes": answer_nodes}


def record(anchor_id=999, candidates=None, tool_calls=None, limits=None):
    return {
        "anchor": {"id": anchor_id, "type": "documentation", "name": "anchor node", "size": 10},
        "candidates": candidates or [],
        "toolCalls": tool_calls or [],
        "limits": limits or {},
    }


class AnswerNodeReportTests(unittest.TestCase):
    def test_anchor_as_answer_node(self):
        """F1: a node that IS the run's anchor must be its own stage, checked before candidates
        or supplementary results are consulted at all -- even though this record also happens to
        carry the same id as a cut candidate, which would (wrongly) read as RETRIEVED_CUT if the
        anchor check ran second."""
        rec = record(
            anchor_id=100,
            candidates=[{"id": 100, "rank": 3, "included": False, "cutReason": "byte budget exceeded"}],
        )
        rows = compare.answer_node_report(task([100]), rec)
        self.assertEqual(len(rows), 1)
        node, stage, rank, detail = rows[0]
        self.assertEqual(node, 100)
        self.assertEqual(stage, compare.ANCHOR)
        self.assertIsNone(rank)
        self.assertIsNone(detail)
        self.assertIn("run's own anchor", stage.indictment)
        self.assertIn("not a retrieval finding", stage.indictment)
        self.assertIn("must not be scored", stage.indictment)

    def test_primary_admitted(self):
        rec = record(candidates=[{"id": 200, "rank": 1, "included": True}])
        rows = compare.answer_node_report(task([200]), rec)
        node, stage, rank, detail = rows[0]
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
        node, stage, rank, detail = rows[0]
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
        node, stage, rank, detail = rows[0]
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
        node, stage, rank, detail = rows[0]
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
        node, stage, rank, detail = rows[0]
        self.assertEqual(stage, compare.RETRIEVED_CUT)
        self.assertEqual(rank, 2)
        self.assertEqual(detail, "cut at the initial round: self-produced")
        self.assertIn("no supplementary round on this task recovered it", stage.indictment)

    def test_not_retrieved(self):
        rec = record(candidates=[], tool_calls=[])
        rows = compare.answer_node_report(task([700]), rec)
        node, stage, rank, detail = rows[0]
        self.assertEqual(stage, compare.NOT_RETRIEVED)
        self.assertIsNone(rank)
        self.assertIsNone(detail)

    def test_no_answer_nodes_named(self):
        self.assertEqual(compare.answer_node_report(task([]), record()), [])
        self.assertEqual(compare.answer_node_report({"id": "t", "task": "x", "subject": 1}, record()), [])


class TaskStageTests(unittest.TestCase):
    def test_unlabelled_when_no_answer_nodes(self):
        self.assertEqual(compare.task_stage(task([]), record()), compare.UNLABELLED)

    def test_precedence_anchor_wins_over_everything(self):
        """A task naming several answer nodes at different stages, including its own anchor,
        must report ANCHOR at the task level -- it is checked first in STAGE_PRECEDENCE regardless
        of the order the nodes are listed in the task set."""
        rec = record(
            anchor_id=50,
            candidates=[
                {"id": 10, "rank": 1, "included": True},                                        # primary
                {"id": 20, "rank": 2, "included": False, "cutReason": "byte budget exceeded"},   # cut
            ],
        )
        # 50 (anchor) listed last: precedence must not depend on list order.
        self.assertEqual(compare.task_stage(task([20, 10, 50]), rec), compare.ANCHOR)

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

    def test_self_produced(self):
        row = {"id": 903, "size": 10, "cutReason": compare.CUT_SELF_PRODUCED}
        message = compare.describe_cut(row, 4000)
        self.assertIn("self-produced", message)
        self.assertIn("before the byte budget is even consulted", message)


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


if __name__ == "__main__":
    unittest.main()
