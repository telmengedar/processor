#!/usr/bin/env python3
"""chat.py sends the conversation it holds and remembers what the user saw.

    python -m unittest discover -s scripts -v

Each test runs the chat against a fake service on a local port that records the request bodies it
receives. No model, no network beyond loopback.
"""

import http.server
import io
import json
import sys
import threading
import unittest
from unittest import mock

import chat

FALLBACK = "I could not put an answer together for that."


class FakeService:
    def __init__(self, responses):
        self.requests = []
        self._responses = list(responses)
        service = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers["Content-Length"])
                service.requests.append(json.loads(self.rfile.read(length)))
                status, payload = service._responses.pop(0)
                body = payload if isinstance(payload, bytes) else json.dumps(payload).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        self._server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self._server.server_address[1]}"
        threading.Thread(target=self._server.serve_forever, daemon=True).start()

    def close(self):
        self._server.shutdown()
        self._server.server_close()


def ok(reply):
    return 200, {"reply": reply, "written": True}


class ChatTests(unittest.TestCase):
    def chat_with(self, responses, lines):
        service = FakeService(responses)
        self.addCleanup(service.close)
        out, err = io.StringIO(), io.StringIO()
        chat.converse(service.url, 901, lines, out, err)
        return service.requests, out.getvalue(), err.getvalue()

    def test_each_exchange_received_is_sent_as_history_on_the_next_turn(self):
        requests, _, _ = self.chat_with(
            [ok("one"), ok("two"), ok("three")], ["a\n", "b\n", "c\n"]
        )
        self.assertEqual(
            [r["history"] for r in requests],
            [
                [],
                [{"input": "a", "reply": "one"}],
                [{"input": "a", "reply": "one"}, {"input": "b", "reply": "two"}],
            ],
        )
        self.assertEqual([r["input"] for r in requests], ["a", "b", "c"])
        self.assertEqual({r["subject"] for r in requests}, {901})

    def test_a_failed_turn_is_remembered_without_a_reply(self):
        refusal = {"error": {"code": "run_deadline_exceeded", "message": "too slow"}}
        requests, out, err = self.chat_with([(504, refusal), ok("fine")], ["lost\n", "next\n"])
        self.assertEqual(requests[1]["history"], [{"input": "lost"}])
        self.assertNotIn("reply", requests[1]["history"][0])
        self.assertIn("504", err)
        self.assertIn("too slow", err)
        self.assertEqual(out, "fine\n")

    def test_the_reply_the_user_saw_is_the_reply_remembered(self):
        requests, out, _ = self.chat_with([ok(FALLBACK), ok("later")], ["ask\n", "again\n"])
        self.assertEqual(out, f"{FALLBACK}\nlater\n")
        self.assertEqual(requests[1]["history"], [{"input": "ask", "reply": FALLBACK}])

    def test_a_blank_line_is_not_sent_and_not_remembered(self):
        requests, _, _ = self.chat_with([ok("one"), ok("two")], ["a\n", "  \n", "\n", "b\n"])
        self.assertEqual(len(requests), 2)
        self.assertEqual(requests[1]["history"], [{"input": "a", "reply": "one"}])

    def test_the_client_holds_only_its_window_of_the_newest_exchanges(self):
        turns = chat.HISTORY_WINDOW + 2
        requests, _, _ = self.chat_with(
            [ok(f"r{i}") for i in range(turns)], [f"q{i}\n" for i in range(turns)]
        )
        sent = requests[-1]["history"]
        self.assertEqual(len(sent), chat.HISTORY_WINDOW)
        self.assertEqual(sent[0], {"input": "q1", "reply": "r1"})
        self.assertEqual(sent[-1], {"input": f"q{turns - 2}", "reply": f"r{turns - 2}"})

    def test_a_200_that_is_not_json_is_a_refused_turn(self):
        requests, out, err = self.chat_with(
            [(200, b"<html>wrong service</html>"), ok("fine")], ["a\n", "b\n"]
        )
        self.assertEqual(requests[1]["history"], [{"input": "a"}])
        self.assertIn("wrong service", err)
        self.assertEqual(out, "fine\n")

    def test_a_200_without_a_string_reply_is_a_refused_turn(self):
        requests, out, _ = self.chat_with(
            [(200, {"written": True}), (200, {"reply": None}), ok("fine")], ["a\n", "b\n", "c\n"]
        )
        self.assertEqual(requests[2]["history"], [{"input": "a"}, {"input": "b"}])
        self.assertEqual(out, "fine\n")


class MainTests(unittest.TestCase):
    def run_main(self, stdin_bytes, argv, interrupt=False):
        def stream(data=b""):
            return io.TextIOWrapper(io.BytesIO(data), encoding="cp1252")

        stdin, stdout, stderr = stream(stdin_bytes), stream(), stream()
        with mock.patch.object(sys, "stdin", stdin), mock.patch.object(sys, "stdout", stdout), \
                mock.patch.object(sys, "stderr", stderr):
            if interrupt:
                with mock.patch.object(chat, "typed_lines", side_effect=KeyboardInterrupt):
                    try:
                        code = chat.main(argv)
                    except KeyboardInterrupt:
                        self.fail("Ctrl-C escaped main")
            else:
                code = chat.main(argv)
            stdout.flush()
            stderr.flush()
            return code, stdout.buffer.getvalue().decode("utf-8"), stderr.buffer.getvalue().decode("utf-8")

    def test_an_unreachable_service_prints_one_line_to_stderr_and_exits_2(self):
        service = FakeService([])
        url = service.url
        service.close()
        code, out, err = self.run_main(b"hello\n", ["--subject", "901", "--url", url])
        self.assertEqual(code, 2)
        self.assertEqual(len(err.splitlines()), 1)
        self.assertIn("did not complete", err)
        self.assertEqual(out, chat.PROMPT)

    def test_typed_input_is_read_as_utf_8(self):
        service = FakeService([ok("grüß dich")])
        self.addCleanup(service.close)
        code, out, _ = self.run_main(
            "héllo ü\n".encode("utf-8"), ["--subject", "901", "--url", service.url]
        )
        self.assertEqual(code, 0)
        self.assertEqual(service.requests[0]["input"], "héllo ü")
        self.assertIn("grüß dich", out)

    def test_ctrl_c_ends_the_chat_with_exit_0(self):
        code, _, err = self.run_main(b"", ["--subject", "901"], interrupt=True)
        self.assertEqual(code, 0)
        self.assertNotIn("Traceback", err)


if __name__ == "__main__":
    unittest.main()
