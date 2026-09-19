"""Small subprocess adapter for the compiled gev CLI.

This module owns transport and process plumbing only. Workflow modules keep
judgment policy and action selection in named Python functions.
"""

from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any


JsonObject = dict[str, Any]


@dataclass
class GevFailure(Exception):
    """A gev process failure whose streams and status must remain observable."""

    status: int
    stdout: str
    stderr: str


class InputFailure(Exception):
    """A local workflow input or response-shape failure."""

    def __init__(self, reason: str) -> None:
        super().__init__(reason)
        self.reason = reason


def _command(base_args: list[str]) -> list[str]:
    command = [os.environ.get("GEV_BIN", "gev"), *base_args]
    base_url = os.environ.get("GEV_BASE_URL", "")
    if base_url:
        command.extend(["--base-url", base_url])
    return command


def _run(command: list[str], document: Any) -> JsonObject:
    process = subprocess.run(
        command,
        input=json.dumps(document, separators=(",", ":")),
        text=True,
        capture_output=True,
        check=False,
    )
    if process.stderr:
        sys.stderr.write(process.stderr)
    if process.returncode != 0:
        status = process.returncode
        if status < 0 and -status == signal.SIGINT:
            status = 130
        raise GevFailure(status, process.stdout, process.stderr)
    try:
        parsed = json.loads(process.stdout)
    except json.JSONDecodeError as error:
        raise InputFailure(f"gev returned invalid JSON: {error.msg}") from error
    if not isinstance(parsed, dict):
        raise InputFailure("gev returned a JSON document that is not an object")
    return parsed


def ask_composed(questions_path: Path, state: Any, model: str | None = None) -> JsonObject:
    """Make one composed-mode ask request with JSON state on stdin."""

    chosen_model = model or os.environ.get("GEV_MODEL", "jev-latest")
    command = _command(
        [
            "ask",
            "--questions",
            str(questions_path),
            "--state-json",
            "-",
            "--model",
            chosen_model,
        ]
    )
    return _run(command, state)


def ask_native(request: JsonObject) -> JsonObject:
    """Make one native-mode request; the request itself is never interpreted."""

    return _run(_command(["ask", "--request", "-"]), request)


def read_json_stdin() -> Any:
    """Read one JSON value without echoing the input in an error."""

    try:
        return json.load(sys.stdin)
    except json.JSONDecodeError as error:
        raise InputFailure(f"stdin is not valid JSON: {error.msg}") from error


def answer(response: JsonObject, name: str) -> JsonObject:
    """Return one typed answer or fail without exposing request state."""

    answers = response.get("answers")
    if not isinstance(answers, dict) or not isinstance(answers.get(name), dict):
        raise InputFailure(f"response is missing answer {name}")
    return answers[name]


def usage(response: JsonObject) -> JsonObject:
    """Keep the server usage document intact in workflow receipts."""

    value = response.get("usage", {})
    return value if isinstance(value, dict) else {}


def add_usage(first: JsonObject, second: JsonObject) -> JsonObject:
    """Aggregate numeric usage while preserving non-numeric usage fields."""

    result: JsonObject = {}
    for key in set(first) | set(second):
        left, right = first.get(key), second.get(key)
        if isinstance(left, (int, float)) and isinstance(right, (int, float)):
            result[key] = left + right
        elif right is not None:
            result[key] = right
        else:
            result[key] = left
    return result


def emit_json(document: JsonObject) -> None:
    """Write exactly one compact JSON document."""

    sys.stdout.write(json.dumps(document, separators=(",", ":")) + "\n")


def emit_local_failure(error: InputFailure) -> int:
    emit_json({"code": "PYTHON_INPUT_INVALID", "message": error.reason})
    return 2


def run_workflow(workflow: Any) -> int:
    """Run a workflow while preserving gev failures verbatim."""

    try:
        receipt, status = workflow()
    except GevFailure as error:
        sys.stdout.write(error.stdout)
        return error.status
    except InputFailure as error:
        return emit_local_failure(error)
    emit_json(receipt)
    return status
