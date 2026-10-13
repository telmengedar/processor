#!/usr/bin/env python3
"""smoke.py prints what a turn's supplementary rounds admitted, and fails when a turn reads a run
record back.

    python -m unittest discover -s scripts -v

A round that re-returns rows the working memory already holds admits them without charging them, so
the printed round line says how many it already held. The verdict is pinned over finished turns and
the leak counter against the Go log message it mirrors. No network.
"""

import contextlib
import io
import re
import unittest
from pathlib import Path

import smoke


def printed(record):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        smoke.print_turn(1, record)
    return buf.getvalue()


def record(results):
    return {
        "anchor": {"id": 1, "type": "documentation", "name": "anchor", "size": 10},
        "candidates": [],
        "toolCalls": [{"query": "again", "results": results}],
        "limits": {},
        "block": "x" * 10,
        "answer": "answer",
        "stopReason": {"reason": "answered", "raw": "stop"},
        "usage": [],
        "model": "m",
        "modelCalls": 2,
        "capReached": False,
    }


class RoundAdmittedLineTests(unittest.TestCase):
    def test_a_round_that_re_returned_held_rows_says_how_many_it_already_held(self):
        out = printed(record([{"id": 11, "included": True, "held": True}, {"id": 52, "included": True}]))
        self.assertIn("-> 2 of 2 admitted (1 already held)", out)

    def test_a_round_that_held_nothing_prints_no_held_note(self):
        out = printed(record([{"id": 52, "included": True}]))
        self.assertIn("-> 1 of 1 admitted", out)
        self.assertNotIn("already held", out)


RUN_NODE = 9001


def row(row_id, name="some node", included=True):
    return {"id": row_id, "name": name, "included": included}


def turn(rows=None, rounds=None, fills=None):
    return {
        "candidates": [row(1)] if rows is None else rows,
        "toolCalls": [{"query": "q", "results": r} for r in rounds or []],
        "fills": fills or [],
    }


STORED = {"state": "stored", "nodeId": RUN_NODE}
RUN_NAME = "processor-run 2026-10-13T10:00:00Z — the input"


class ExclusionBreachTests(unittest.TestCase):
    def test_a_clean_pair_of_turns_has_no_breach(self):
        self.assertEqual([], smoke.exclusion_breaches([turn(), turn()], RUN_NODE))

    def test_a_supplementary_round_carrying_the_record_is_found(self):
        records = [turn(), turn(rounds=[[row(RUN_NODE)]])]
        self.assertEqual([(2, 1, RUN_NODE, "some node")], smoke.exclusion_breaches(records, RUN_NODE))

    def test_a_run_record_name_under_another_id_is_found(self):
        records = [turn(), turn(rows=[row(1), row(77, RUN_NAME)])]
        self.assertEqual([(2, 0, 77, RUN_NAME)], smoke.exclusion_breaches(records, RUN_NODE))

    def test_a_breach_in_turn_one_is_found(self):
        records = [turn(rows=[row(1), row(78, RUN_NAME)]), turn()]
        self.assertEqual([(1, 0, 78, RUN_NAME)], smoke.exclusion_breaches(records, RUN_NODE))

    def test_the_record_id_is_matched_in_the_initial_candidates(self):
        records = [turn(), turn(rows=[row(RUN_NODE)])]
        self.assertEqual([(2, 0, RUN_NODE, "some node")], smoke.exclusion_breaches(records, RUN_NODE))


class LeakCounterTests(unittest.TestCase):
    def test_it_counts_only_the_lines_carrying_the_message(self):
        log = (
            f'level=WARN msg="{smoke.LEAK_MESSAGE}" subject=1 rows=2\n'
            'level=INFO msg="something else"\n'
            f'level=WARN msg="{smoke.LEAK_MESSAGE}" subject=1 rows=1\n'
        )
        self.assertEqual(2, smoke.count_leaked_rows(log))

    def test_a_log_without_the_message_counts_zero(self):
        self.assertEqual(0, smoke.count_leaked_rows("level=INFO msg=ok\n"))

    def test_the_message_is_the_one_the_loop_logs(self):
        source = (Path(smoke.REPO) / "internal" / "loop" / "retrieve.go").read_text(encoding="utf-8")
        found = re.search(r'const logLeakedRows = "([^"]+)"', source)
        self.assertIsNotNone(found)
        self.assertEqual(found.group(1), smoke.LEAK_MESSAGE)


class VerdictTests(unittest.TestCase):
    def test_a_clean_run_passes(self):
        code, lines = smoke.verdict([turn(), turn()], [STORED, STORED], 0)
        self.assertEqual(0, code)
        self.assertTrue(any(line.startswith("PASS") for line in lines))

    def test_a_missing_node_id_is_exit_two(self):
        code, _ = smoke.verdict([turn(), turn()], [{"state": "stored"}, STORED], 0)
        self.assertEqual(2, code)

    def test_a_not_stored_receipt_is_exit_two(self):
        code, _ = smoke.verdict([turn(), turn()], [{"state": "notStored"}, STORED], 0)
        self.assertEqual(2, code)

    def test_a_breach_is_exit_one_and_says_self_read(self):
        code, lines = smoke.verdict([turn(), turn(rounds=[[row(RUN_NODE)]])], [STORED, STORED], 0)
        self.assertEqual(1, code)
        self.assertTrue(any(line.startswith("FAIL: self-read") for line in lines))

    def test_a_failure_outranks_a_void_premise(self):
        records = [turn(), turn(rounds=[[row(77, RUN_NAME)]])]
        code, _ = smoke.verdict(records, [{"state": "notStored"}, STORED], 0)
        self.assertEqual(1, code)

    def test_zero_admitted_is_exit_one(self):
        code, _ = smoke.verdict([turn(rows=[row(1, included=False)]), turn()], [STORED, STORED], 0)
        self.assertEqual(1, code)

    def test_a_leak_is_exit_one(self):
        code, lines = smoke.verdict([turn(), turn()], [STORED, STORED], 2)
        self.assertEqual(1, code)
        self.assertTrue(any("2 time(s)" in line for line in lines))

    def test_the_leak_count_is_printed_when_there_is_none(self):
        _, lines = smoke.verdict([turn(), turn()], [STORED, STORED], 0)
        self.assertTrue(any(line.startswith("LEAK REPORT: 0") for line in lines))


class SubstancesFilledTests(unittest.TestCase):
    def test_only_the_fills_that_landed_are_counted(self):
        record = turn(fills=[{"id": 1, "filled": True}, {"id": 2, "filled": False, "deferred": True}])
        self.assertEqual(1, smoke.filled_in_turn(record))


if __name__ == "__main__":
    unittest.main()
