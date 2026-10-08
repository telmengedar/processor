#!/usr/bin/env python3
"""smoke.py prints what a turn's supplementary rounds admitted.

    python -m unittest discover -s scripts -v

A round that re-returns rows the working memory already holds admits them without charging them, so
the printed round line says how many it already held. No network.
"""

import contextlib
import io
import unittest

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


if __name__ == "__main__":
    unittest.main()
