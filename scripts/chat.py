#!/usr/bin/env python3
"""An interactive chat with a running Processor: one line in, one reply out.

    python scripts/chat.py --subject 901
    python scripts/chat.py --subject 901 --url http://127.0.0.1:8080

Each line is posted to /runs together with the exchanges this client holds, and the reply is printed
and kept as the next exchange. The service keeps no conversation state, so the conversation lives
here and ends with the process. The client holds a window larger than the service's own limit on
purpose: how many exchanges a turn actually uses is the service's decision, reported by
`limits.historyExchangeLimit` on a verbose run.

A turn the service refused is kept as an input without a reply, because the user said it and the
bot said nothing; so is a 200 whose body is not JSON or carries no string `reply`. Ctrl-D (or
Ctrl-Z then Enter on Windows) and Ctrl-C end the chat. Standard library only.

Exit 0 the chat ended; 2 the service could not be reached or the arguments were wrong.
"""

import argparse
import collections
import http.client
import json
import sys
import urllib.error
import urllib.request

DEFAULT_URL = "http://127.0.0.1:8080"
HISTORY_WINDOW = 16
PROMPT = "> "


class TurnRefused(Exception):
    pass


class ServiceUnreachable(Exception):
    pass


def post_run(url, subject, text, history):
    body = json.dumps({"input": text, "subject": subject, "history": history}).encode("utf-8")
    request = urllib.request.Request(
        f"{url}/runs", data=body, headers={"Content-Type": "application/json"}, method="POST"
    )
    try:
        with urllib.request.urlopen(request) as response:
            payload = response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as err:
        with err:
            detail = err.read().decode("utf-8", "replace").strip()
        raise TurnRefused(f"HTTP {err.code}: {detail}") from err
    except (urllib.error.URLError, OSError, http.client.HTTPException) as err:
        raise ServiceUnreachable(f"POST {url}/runs did not complete: {type(err).__name__}: {err}") from err
    return reply_of(payload)


def reply_of(payload):
    try:
        reply = json.loads(payload)["reply"]
    except (json.JSONDecodeError, KeyError, TypeError):
        reply = None
    if not isinstance(reply, str):
        raise TurnRefused(
            f"POST /runs returned 200 and {len(payload)} bytes without a string reply; "
            f"the body starts {payload[:200]!r}"
        )
    return reply


def converse(url, subject, lines, out, err):
    held = collections.deque(maxlen=HISTORY_WINDOW)
    for line in lines:
        text = line.rstrip("\r\n")
        if not text.strip():
            continue
        try:
            reply = post_run(url, subject, text, list(held))
        except TurnRefused as refused:
            print(refused, file=err)
            held.append({"input": text})
            continue
        print(reply, file=out)
        held.append({"input": text, "reply": reply})


def typed_lines():
    while True:
        try:
            yield input(PROMPT)
        except EOFError:
            return


def parse_args(argv):
    parser = argparse.ArgumentParser(description="Chat with a running Processor.")
    parser.add_argument("--subject", type=int, required=True, help="the subject node id every turn runs against")
    parser.add_argument("--url", default=DEFAULT_URL, help=f"the service's base URL (default {DEFAULT_URL})")
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    sys.stdin.reconfigure(encoding="utf-8")
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")
    try:
        converse(args.url.rstrip("/"), args.subject, typed_lines(), sys.stdout, sys.stderr)
    except KeyboardInterrupt:
        print(file=sys.stderr)
    except ServiceUnreachable as unreachable:
        print(unreachable, file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
