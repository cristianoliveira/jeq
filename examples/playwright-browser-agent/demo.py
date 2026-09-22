#!/usr/bin/env python3
"""Run the bounded Jev and Playwright CLI demonstration on a loopback fixture."""
from __future__ import annotations

import argparse
import functools
import http.server
import json
import pathlib
import sys
import threading

from agent import BrowserBoundary
from policy import subprocess_runner
from runner import run_agent

ROOT = pathlib.Path(__file__).parent
DEFAULT_SCREENSHOT = pathlib.Path("output/playwright/playwright-browser-agent/final.png")
DEFAULT_GOAL = (
    "Find Design stays in Lisbon with free cancellation, then open Casa Flora."
)


class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, _format, *_args):
        pass


def verify_fixture(outcome: dict[str, str]) -> dict[str, bool]:
    body = outcome.get("body", "")
    return {
        "detail_url": outcome.get("url", "").endswith("/fixture.html#casa-flora"),
        "title": outcome.get("title") == "Casa Flora · Forma",
        "stay": "Selected stay: Casa Flora" in body,
        "destination": "Destination: Lisbon" in body,
        "category": "Category: Design" in body,
        "cancellation": "Free cancellation: enabled" in body,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goal", default=DEFAULT_GOAL)
    parser.add_argument("--fill-value", action="append", default=[])
    parser.add_argument("--jeq-bin", default="jeq")
    parser.add_argument("--screenshot", type=pathlib.Path, default=DEFAULT_SCREENSHOT)
    parser.add_argument("--max-steps", type=int, default=8)
    parser.add_argument("--headed", action="store_true", help="show the Playwright browser window")
    args = parser.parse_args()

    handler = functools.partial(QuietHandler, directory=str(ROOT))
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    url = f"http://127.0.0.1:{server.server_port}/fixture.html"

    try:
        result = run_agent(
            url=url,
            goal=args.goal,
            fill_values=tuple(args.fill_value or ["Lisbon"]),
            decision_runner=subprocess_runner(args.jeq_bin),
            verify=verify_fixture,
            screenshot=args.screenshot,
            boundary=BrowserBoundary(budget=args.max_steps, headed=args.headed),
            max_steps=args.max_steps,
        )
        checks = verify_fixture(result.outcome)
        print(json.dumps({"verified": checks}, separators=(",", ":")))
        return 0
    except Exception as exc:
        print(json.dumps({"status": "FAIL", "error": str(exc)}), file=sys.stderr)
        return 1
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


if __name__ == "__main__":
    raise SystemExit(main())
