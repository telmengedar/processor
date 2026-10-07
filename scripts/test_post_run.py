#!/usr/bin/env python3
"""Both live-run scripts must ask POST /runs for the full record.

    python scripts/test_post_run.py
    python -m unittest discover -s scripts -v

The default POST /runs body is the chat reply and the write receipt; compare.py and smoke.py read the
whole record out of it, so each must ask for ?verbose=true. No network: urlopen is replaced.
"""

import json
import unittest
from unittest import mock

import compare
import smoke


class FakeResponse:
    def __init__(self, payload):
        self.payload = payload

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def read(self):
        return json.dumps(self.payload).encode("utf-8")


class PostRunAsksForTheFullRecordTests(unittest.TestCase):
    def requested(self, module):
        record = {"answer": "an answer", "block": "a block"}
        seen = []

        def urlopen(request, timeout):
            seen.append(request)
            return FakeResponse(record)

        with mock.patch.object(module.urllib.request, "urlopen", urlopen):
            returned = module.post_run(8080, "what changed", 42)
        self.assertEqual(returned, record)
        self.assertEqual(len(seen), 1)
        return seen[0]

    def test_compare_requests_the_verbose_form(self):
        request = self.requested(compare)
        self.assertEqual(request.full_url, "http://127.0.0.1:8080/runs?verbose=true")
        self.assertEqual(json.loads(request.data), {"input": "what changed", "subject": 42})

    def test_smoke_requests_the_verbose_form(self):
        request = self.requested(smoke)
        self.assertEqual(request.full_url, "http://127.0.0.1:8080/runs?verbose=true")
        self.assertEqual(json.loads(request.data), {"input": "what changed", "subject": 42})


if __name__ == "__main__":
    unittest.main()
