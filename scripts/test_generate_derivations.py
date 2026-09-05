#!/usr/bin/env python3
"""Unit tests for the pure functions in generate_derivations.py.

    python scripts/test_generate_derivations.py
    python -m unittest discover -s scripts -v

No network, no model, no filesystem beyond a tempfile fixture: project_corpus_blind,
parse_lines, dedupe_against, shape_errors, pinned_ids, corpus_order_key, select_targets and
merge_sidecar are all pure (or pure but for a single read of a path handed to them). call_model
is exercised once, with urllib.request.urlopen monkeypatched, to prove what actually leaves the
process rather than what the code merely intends to send.

BlindnessTests is deliberately first. The blind projection is the entire justification for this
unit over hand-authoring (DiVoid #11235 S7 R2a, #11292) and, before these tests, was guaranteed
only by a docstring: a future edit adding `subject` to BlindRow "for better logging" would leave
every other test green and produce a contaminated sidecar no downstream measurement could detect.
These tests assert the dataclass's field set directly and prove, by capturing the actual HTTP
request body, that a row's `required`/`subject`/`why` text cannot reach the model even via the
retry path (`exclude`, in call_model, carries only model-generated text forward).

TargetSelectionTests covers the defect a real QA pass demonstrated without spending a model call:
`--only r01 --endpoint <dead>` printed `rows-to-generate=['r01']` and only the dead endpoint
stopped it from overwriting the pinned baseline. select_targets is the extracted decision the CLI
now defers to, so these tests exercise the refusal directly rather than through argv/subprocess.

ShapeTests covers the second half of that same review round: the prompt asks for four questions
then one dense keyword line, and nothing checked it. r15's fifth generated line was a prose
question that satisfied the only mechanical rule then in force (no question mark is not the same
check as ends with one) while missing the intent entirely.
"""

import dataclasses
import json
import tempfile
import unittest
import urllib.error
from pathlib import Path
from unittest import mock

import generate_derivations as gd


def write_json(obj) -> Path:
    handle = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8")
    json.dump(obj, handle)
    handle.close()
    return Path(handle.name)


class BlindnessTests(unittest.TestCase):
    """The property the whole unit rests on: the model never sees anything but id/input."""

    def test_blind_row_carries_exactly_id_and_input(self):
        field_names = {f.name for f in dataclasses.fields(gd.BlindRow)}
        self.assertEqual(field_names, {"id", "input"},
                          "BlindRow grew a field beyond id/input -- whatever it is now leaks "
                          "into call_model's row-shaped input unless every caller is re-audited")

    def test_project_corpus_blind_discards_everything_but_id_and_input(self):
        corpus_path = write_json([
            {
                "id": "r99",
                "input": "What is the retry budget?",
                "subject": 424242,
                "stratum": "labelled",
                "required": [{"node": 999999, "hash": "deadbeef", "why": "SECRET_ANSWER_KEY_TEXT"}],
            }
        ])
        try:
            rows = gd.project_corpus_blind(corpus_path)
        finally:
            corpus_path.unlink()

        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(row.id, "r99")
        self.assertEqual(row.input, "What is the retry budget?")
        self.assertFalse(hasattr(row, "required"))
        self.assertFalse(hasattr(row, "subject"))
        self.assertFalse(hasattr(row, "why"))

    def test_call_model_payload_never_carries_the_answer_key_fields(self):
        """Even if a caller had a contaminated row available, call_model's signature only
        accepts row_input (a string) and exclude (model-generated strings from earlier
        attempts) -- so the request body it builds cannot contain a row's required/subject/why
        text unless that text is echoed back by the model itself, which these values are not."""
        secret_subject = "424242"
        secret_node_id = "999999"
        secret_why = "SECRET_ANSWER_KEY_TEXT_THE_MODEL_MUST_NEVER_SEE"
        blind_input = "What is the retry budget?"

        captured = {}

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *exc):
                return False

            def read(self):
                return json.dumps({"choices": [{"message": {"content": "line\n" * 5}}]}).encode("utf-8")

        def fake_urlopen(request, timeout):
            captured["body"] = request.data.decode("utf-8")
            return FakeResponse()

        with mock.patch("urllib.request.urlopen", side_effect=fake_urlopen):
            gd.call_model(gd.DEFAULT_ENDPOINT, gd.DEFAULT_MODEL, blind_input, [], 5, 0.5)

        body = captured["body"]
        self.assertIn(blind_input, body, "sanity: the row's own input must reach the model")
        for secret in (secret_subject, secret_node_id, secret_why):
            self.assertNotIn(secret, body,
                              f"{secret!r} reached the request body -- the answer key leaked")

    def test_few_shot_exemplars_are_format_only_and_untouched_rows_stay_untouched(self):
        """F6: the exemplars are drawn from r01/r06 verbatim, which is only safe as long as
        those rows are never regenerated. select_targets (TargetSelectionTests) is what actually
        closes this; this test just pins that the exemplar table still only names those two."""
        exemplar_inputs = {example["input"] for example in gd.FEW_SHOT}
        self.assertEqual(len(gd.FEW_SHOT), 2)
        self.assertTrue(all(isinstance(q, str) for ex in gd.FEW_SHOT for q in ex["queries"]))
        # The exemplars carry no required/subject/why keys at all -- only input/queries.
        for example in gd.FEW_SHOT:
            self.assertEqual(set(example.keys()), {"input", "queries"})
        self.assertEqual(len(exemplar_inputs), 2)


class ParseLinesTests(unittest.TestCase):
    def test_strips_think_blocks(self):
        raw = "<think>reasoning the user never asked for</think>\nWhat is X?\nkeyword line"
        self.assertEqual(gd.parse_lines(raw), ["What is X?", "keyword line"])

    def test_strips_bullets_numbering_and_quotes(self):
        raw = '1. "What is X?"\n- What is Y?\n* keyword phrase\n2) another one'
        self.assertEqual(
            gd.parse_lines(raw),
            ["What is X?", "What is Y?", "keyword phrase", "another one"],
        )

    def test_drops_blank_lines(self):
        raw = "What is X?\n\n   \nkeyword line"
        self.assertEqual(gd.parse_lines(raw), ["What is X?", "keyword line"])

    def test_case_insensitive_think_block(self):
        raw = "<THINK>internal</THINK>\nWhat is X?"
        self.assertEqual(gd.parse_lines(raw), ["What is X?"])


class DedupeAgainstTests(unittest.TestCase):
    def test_drops_exact_duplicate_of_raw_input_case_insensitive(self):
        result = gd.dedupe_against("What is X?", ["WHAT IS X?", "What is Y?"], [])
        self.assertEqual(result, ["What is Y?"])

    def test_drops_repeats_of_already_collected(self):
        result = gd.dedupe_against("input", ["a", "b", "a"], ["B"])
        self.assertEqual(result, ["a"])

    def test_drops_blanks(self):
        result = gd.dedupe_against("input", ["", "   ", "real query"], [])
        self.assertEqual(result, ["real query"])

    def test_keeps_order_and_first_occurrence(self):
        result = gd.dedupe_against("input", ["a", "a", "b"], [])
        self.assertEqual(result, ["a", "b"])


class ShapeTests(unittest.TestCase):
    """F5: the prompt asks for 4 questions + 1 keyword line; nothing checked the shape."""

    def test_well_shaped_set_has_no_errors(self):
        queries = ["Q1?", "Q2?", "Q3?", "Q4?", "keyword line no question mark"]
        self.assertEqual(gd.shape_errors(queries), [])

    def test_wrong_count_is_one_error(self):
        errors = gd.shape_errors(["Q1?", "Q2?"])
        self.assertEqual(len(errors), 1)
        self.assertIn("got 2", errors[0])

    def test_non_question_in_first_four_is_flagged(self):
        queries = ["Q1?", "not a question.", "Q3?", "Q4?", "keyword line"]
        errors = gd.shape_errors(queries)
        self.assertEqual(len(errors), 1)
        self.assertIn("line 2", errors[0])

    def test_last_line_as_a_question_is_flagged(self):
        """This is r15's actual pinned fifth line, verbatim: no trailing '?' at all, so a
        mark-only check would have missed it entirely -- it is unmistakably an interrogative
        sentence ("How does...") rather than a dense keyword line, which is what
        looks_like_a_question's word-starter fallback exists to catch."""
        queries = [
            "Q1?", "Q2?", "Q3?", "Q4?",
            "How does the ranking system prevent item cycling in top positions",
        ]
        errors = gd.shape_errors(queries)
        self.assertEqual(len(errors), 1)
        self.assertIn("line 5", errors[0])
        self.assertIn("keyword", errors[0])

    def test_last_line_with_trailing_question_mark_is_also_flagged(self):
        queries = ["Q1?", "Q2?", "Q3?", "Q4?", "also phrased as a question?"]
        errors = gd.shape_errors(queries)
        self.assertEqual(len(errors), 1)
        self.assertIn("line 5", errors[0])

    def test_multiple_problems_all_reported(self):
        queries = ["not a question", "Q2?", "Q3?", "Q4?", "also a question?"]
        errors = gd.shape_errors(queries)
        self.assertEqual(len(errors), 2)


class PinnedIdsTests(unittest.TestCase):
    def test_extracts_row_ids(self):
        entries = [{"row": "r01", "queries": ["a"]}, {"row": "c02", "queries": ["b"]}]
        self.assertEqual(gd.pinned_ids(entries), {"r01", "c02"})

    def test_empty_list_is_empty_set(self):
        self.assertEqual(gd.pinned_ids([]), set())


class CorpusOrderKeyTests(unittest.TestCase):
    def test_sorts_by_corpus_position(self):
        corpus_ids = ["r01", "r02", "r03"]
        entries = [{"row": "r03"}, {"row": "r01"}, {"row": "r02"}]
        entries.sort(key=gd.corpus_order_key(corpus_ids))
        self.assertEqual([e["row"] for e in entries], ["r01", "r02", "r03"])

    def test_unknown_row_sorts_last(self):
        corpus_ids = ["r01", "r02"]
        entries = [{"row": "unknown"}, {"row": "r01"}]
        entries.sort(key=gd.corpus_order_key(corpus_ids))
        self.assertEqual([e["row"] for e in entries], ["r01", "unknown"])


class TargetSelectionTests(unittest.TestCase):
    """F2: --only must never silently overwrite a pinned row."""

    def setUp(self):
        self.blind_rows = [gd.BlindRow(id=f"r{i:02d}", input=f"input {i}") for i in range(1, 4)]
        self.corpus_ids = [row.id for row in self.blind_rows]

    def test_default_run_targets_only_unpinned_rows(self):
        targets, error = gd.select_targets(self.blind_rows, self.corpus_ids, {"r01"}, None, False)
        self.assertIsNone(error)
        self.assertEqual([r.id for r in targets], ["r02", "r03"])

    def test_only_on_unpinned_row_succeeds(self):
        targets, error = gd.select_targets(self.blind_rows, self.corpus_ids, {"r01"}, "r02", False)
        self.assertIsNone(error)
        self.assertEqual([r.id for r in targets], ["r02"])

    def test_only_on_pinned_row_without_force_is_refused(self):
        """This is the exact case demonstrated live: --only r01 with r01 already pinned must
        error out before any model call, not silently agree to overwrite the baseline."""
        targets, error = gd.select_targets(self.blind_rows, self.corpus_ids, {"r01"}, "r01", False)
        self.assertEqual(targets, [])
        self.assertIsNotNone(error)
        self.assertIn("r01", error)
        self.assertIn("--force", error)

    def test_only_on_pinned_row_with_force_succeeds(self):
        targets, error = gd.select_targets(self.blind_rows, self.corpus_ids, {"r01"}, "r01", True)
        self.assertIsNone(error)
        self.assertEqual([r.id for r in targets], ["r01"])

    def test_only_naming_unknown_row_is_refused_regardless_of_force(self):
        targets, error = gd.select_targets(self.blind_rows, self.corpus_ids, set(), "r99", True)
        self.assertEqual(targets, [])
        self.assertIn("r99", error)

    def test_mixed_only_with_one_pinned_row_is_refused_even_if_others_are_not(self):
        targets, error = gd.select_targets(self.blind_rows, self.corpus_ids, {"r02"}, "r01,r02", False)
        self.assertEqual(targets, [])
        self.assertIn("r02", error)


class MergeSidecarTests(unittest.TestCase):
    def test_generated_rows_are_added_in_corpus_order(self):
        sidecar = [{"row": "r01", "queries": ["old"]}]
        generated = {"r03": ["new3"], "r02": ["new2"]}
        merged = gd.merge_sidecar(sidecar, generated, ["r01", "r02", "r03"])
        self.assertEqual([e["row"] for e in merged], ["r01", "r02", "r03"])
        self.assertEqual(merged[1]["queries"], ["new2"])
        self.assertEqual(merged[2]["queries"], ["new3"])

    def test_generated_rows_are_stamped_blind_generated(self):
        """The source field is what lets a sweep tell a hand-authored row from one this script
        produced without ever re-typing a count anywhere -- see SourceBlindGenerated in
        internal/eval/derivations.go."""
        sidecar = []
        merged = gd.merge_sidecar(sidecar, {"r02": ["new2"]}, ["r02"])
        self.assertEqual(merged, [{"row": "r02", "queries": ["new2"], "source": "blind-generated"}])

    def test_generated_row_replaces_pinned_row_of_the_same_id(self):
        """This is the only path via which a pinned row's queries in the file actually change --
        select_targets is what must have already agreed to it (F2); merge_sidecar just performs
        the replacement once targets are legitimate."""
        sidecar = [{"row": "r01", "queries": ["old"], "source": "hand-authored"}, {"row": "r02", "queries": ["untouched"]}]
        generated = {"r01": ["replaced"]}
        merged = gd.merge_sidecar(sidecar, generated, ["r01", "r02"])
        self.assertEqual(merged, [
            {"row": "r01", "queries": ["replaced"], "source": "blind-generated"},
            {"row": "r02", "queries": ["untouched"]},
        ])

    def test_untouched_pinned_rows_are_byte_identical(self):
        sidecar = [{"row": "r01", "queries": ["a", "b"]}, {"row": "c01", "queries": ["c"]}]
        merged = gd.merge_sidecar(sidecar, {"r02": ["d"]}, ["r01", "r02", "c01"])
        untouched = [e for e in merged if e["row"] in ("r01", "c01")]
        self.assertEqual(untouched, sidecar)


if __name__ == "__main__":
    unittest.main()
