#!/usr/bin/env python3
"""Safe, finite boundary around the installed playwright-cli executable."""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
import time
from urllib.parse import urlparse
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

REF = re.compile(r"\[ref=(e\d+)\]")
ROLE = re.compile(r"-\s+(button|link|searchbox|textbox|checkbox|combobox)\s+(?:\"([^\"]*)\")?.*\[ref=(e\d+)\]")
OPTION = re.compile(r"option \"([^\"]*)\"")

@dataclass(frozen=True)
class Element:
    ref: str
    role: str
    name: str
    options: tuple[str, ...] = ()

@dataclass(frozen=True)
class Action:
    operation: str
    ref: str = ""
    value: str = ""


def parse_snapshot(snapshot: str) -> dict[str, Element]:
    elements: dict[str, Element] = {}
    lines = snapshot.splitlines()
    for i, line in enumerate(lines):
        match = ROLE.search(line)
        if not match:
            continue
        role, name, ref = match.groups()
        options = tuple(OPTION.findall("\n".join(lines[i : i + 8]))) if role == "combobox" else ()
        elements[ref] = Element(ref, role, name or "", options)
    return elements


def allow(action: Action, elements: dict[str, Element]) -> list[str]:
    if action.operation == "stop":
        return ["stop"]
    if action.operation == "wait":
        return ["wait", str(min(max(int(action.value or "1"), 0), 5))]
    if action.operation == "scroll":
        return ["mousewheel", "0", str(min(max(int(action.value or "400"), -1000), 1000))]
    element = elements.get(action.ref)
    if not element:
        raise ValueError("ref is absent from the latest snapshot")
    if action.operation == "click" and element.role in {"button", "link"}:
        return ["click", element.ref]
    if action.operation == "fill" and element.role in {"searchbox", "textbox"}:
        if len(action.value) > 512 or "\x00" in action.value:
            raise ValueError("fill value exceeds 512 characters or contains NUL")
        return ["fill", element.ref, action.value]
    if action.operation in {"check", "uncheck"} and element.role == "checkbox":
        return [action.operation, element.ref]
    if action.operation == "select" and element.role == "combobox" and action.value in element.options:
        return ["select", element.ref, action.value]
    raise ValueError("operation is not allowlisted for the observed element")

class BrowserBoundary:
    def __init__(self, executable: str = "playwright-cli", budget: int = 4):
        self.executable, self.budget = executable, budget
        self.session = "jeq-" + uuid.uuid4().hex[:8]
        self.used = 0
        self.elements: dict[str, Element] = {}

    def run(self, *args: str) -> str:
        result = subprocess.run([self.executable, f"-s={self.session}", *args], text=True, capture_output=True, timeout=10)
        if result.returncode:
            raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "playwright-cli failed")
        return result.stdout

    def open(self, url: str) -> None:
        parsed = urlparse(url)
        if parsed.scheme != "http" or parsed.username or parsed.password or parsed.hostname not in {"127.0.0.1", "localhost", "::1"} or not parsed.port:
            raise ValueError("browser fixture URL must be loopback HTTP without credentials")
        self.run("open", url)

    def observe(self) -> dict[str, Element]:
        self.elements = parse_snapshot(self.run("snapshot"))
        return dict(self.elements)

    def execute(self, action: Action) -> str:
        if self.used >= self.budget:
            raise RuntimeError("action budget exhausted")
        command = allow(action, self.elements)
        if command == ["stop"]:
            return "stopped"
        self.used += 1
        if command[0] == "wait":
            time.sleep(min(int(command[1]), 5) * 0.1)
            return "waited"
        return self.run(*command)

    def inspect(self) -> dict[str, str]:
        # Independent inspection does not trust a model action or snapshot text.
        def value(output: str) -> str:
            lines = output.splitlines()
            try: value_line = lines[lines.index("### Result") + 1]
            except (ValueError, IndexError) as exc: raise RuntimeError(f"unexpected eval output: {output!r}") from exc
            import json
            return str(json.loads(value_line))
        return {"title": value(self.run("eval", "document.title")), "url": value(self.run("eval", "location.href")), "body": value(self.run("eval", "document.body.innerText"))}

    def close(self) -> None:
        try:
            self.run("close")
        except (OSError, RuntimeError, subprocess.TimeoutExpired):
            pass

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("url")
    parser.add_argument("--fill", default="")
    args = parser.parse_args()
    boundary = BrowserBoundary()
    try:
        boundary.open(args.url)
        boundary.observe()
        search = next((e for e in boundary.elements.values() if e.role == "searchbox"), None)
        if search and args.fill:
            boundary.execute(Action("fill", search.ref, args.fill))
            boundary.observe()
        print(boundary.inspect())
        return 0
    except (ValueError, RuntimeError, subprocess.TimeoutExpired) as exc:
        print(f"browser boundary failed: {exc}", file=sys.stderr)
        return 1
    finally:
        boundary.close()

if __name__ == "__main__":
    raise SystemExit(main())
