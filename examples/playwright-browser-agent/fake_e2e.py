#!/usr/bin/env python3
"""Offline end-to-end test: real jeq and playwright-cli, fake loopback TypeSafe API."""
from __future__ import annotations

import functools
import http.server
import json
import os
import pathlib
import threading

from demo import QuietHandler, ROOT, verify_fixture
from policy import subprocess_runner
from runner import run_agent


class FakeTypeSafe(http.server.BaseHTTPRequestHandler):
    calls = 0

    def log_message(self, _format, *_args):
        pass

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        request = json.loads(self.rfile.read(length))
        step = type(self).calls
        type(self).calls += 1
        sequence = ["fill", "select", "check", "click_find", "click_view", "DONE"]
        desired = sequence[step] if step < len(sequence) else "DONE"

        answers = {}
        operation_id = _operation_id(request["questions"]["operation"]["criteria"], desired)
        for question_id, question in request["questions"].items():
            criteria = question["criteria"]
            selected = next(iter(criteria))
            if question_id == "operation":
                selected = operation_id
            elif desired == "fill" and question_id == "fill_target":
                selected = _criterion_id(criteria, "Destination")
            elif desired == "fill" and question_id == "fill_value":
                selected = _criterion_id(criteria, "Lisbon")
            elif desired == "select" and question_id == "select_target":
                selected = _criterion_id(criteria, "option Design")
            elif desired == "check" and question_id == "check_target":
                selected = _criterion_id(criteria, "Free cancellation")
            elif desired == "click_find" and question_id == "click_target":
                selected = _criterion_id(criteria, "Find stays")
            elif desired == "click_view" and question_id == "click_target":
                selected = _criterion_id(criteria, "View Casa Flora")
            answers[question_id] = {
                "type": "choice",
                "choice": selected,
                "probabilities": {candidate: float(candidate == selected) for candidate in criteria},
                "confidence": 1,
            }

        body = json.dumps({
            "model": "fake-jev",
            "answers": answers,
            "usage": {"input_tokens": 10, "output_tokens": 2},
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def _criterion_id(criteria, text):
    return next(candidate for candidate, description in criteria.items() if text in description)


def _operation_id(criteria, desired):
    needles = {
        "fill": "Fill one observed",
        "select": "Select one observed",
        "check": "Check one observed",
        "click_find": "Click one observed",
        "click_view": "Click one observed",
        "DONE": "Every requirement",
    }
    return _criterion_id(criteria, needles[desired])


def main() -> int:
    fixture_handler = functools.partial(QuietHandler, directory=str(ROOT))
    fixture = http.server.ThreadingHTTPServer(("127.0.0.1", 0), fixture_handler)
    provider = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FakeTypeSafe)
    threads = [
        threading.Thread(target=fixture.serve_forever, daemon=True),
        threading.Thread(target=provider.serve_forever, daemon=True),
    ]
    for thread in threads:
        thread.start()

    env = os.environ.copy()
    env.pop("JEQ_CONFIG", None)
    env.update({
        "JEQ_PROVIDER": "typesafe",
        "TYPESAFE_BASE_URL": f"http://127.0.0.1:{provider.server_port}",
        "TYPESAFE_API_KEY": "offline-test",
    })
    screenshot = pathlib.Path("output/playwright/playwright-browser-agent/fake-final.png")
    try:
        FakeTypeSafe.calls = 0
        result = run_agent(
            url=f"http://127.0.0.1:{fixture.server_port}/fixture.html",
            goal="Find Design stays in Lisbon with free cancellation, then open Casa Flora.",
            fill_values=("Lisbon",),
            decision_runner=subprocess_runner(os.environ.get("JEQ_BIN", "jeq"), env),
            verify=verify_fixture,
            screenshot=screenshot,
            max_steps=8,
        )
        if FakeTypeSafe.calls != 6 or not all(verify_fixture(result.outcome).values()):
            raise AssertionError(f"unexpected result: calls={FakeTypeSafe.calls} outcome={result.outcome}")
        print(f"PASS fake_requests={FakeTypeSafe.calls} screenshot={screenshot}")
        return 0
    finally:
        for server in (fixture, provider):
            server.shutdown()
            server.server_close()
        for thread in threads:
            thread.join(timeout=2)


if __name__ == "__main__":
    raise SystemExit(main())
