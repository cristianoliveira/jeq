#!/usr/bin/env python3
"""Safe, finite boundary around the installed playwright-cli executable."""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
import time
from urllib.parse import urljoin, urlparse
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

REF = re.compile(r"\[ref=(e\d+)\]")
ROLE = re.compile(r"-\s+(button|link|searchbox|textbox|checkbox|combobox)\s+(?:\"([^\"]*)\")?.*\[ref=(e\d+)\]")
OPTION = re.compile(r"option \"([^\"]*)\"")
HREF = re.compile(r"- /url: (.+)$")

@dataclass(frozen=True)
class Element:
    ref: str
    role: str
    name: str
    options: tuple[str, ...] = ()
    checked: bool = False
    value: str = ""
    href: str = ""

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
        children = _descendants(lines, i)
        options = tuple(OPTION.findall("\n".join(children))) if role == "combobox" else ()
        selected = next((OPTION.search(option).group(1) for option in children if "[selected]" in option and OPTION.search(option)), "")
        href = next((HREF.search(child).group(1).strip() for child in children if HREF.search(child)), "")
        trailing = line[match.end() :].strip()
        value = trailing[1:].strip() if trailing.startswith(":") else selected
        elements[ref] = Element(ref, role, name or "", options, "[checked]" in line, value, href)
    return elements


def _descendants(lines: list[str], index: int) -> list[str]:
    parent_indent = len(lines[index]) - len(lines[index].lstrip())
    descendants: list[str] = []
    for line in lines[index + 1 :]:
        if not line.strip():
            continue
        indent = len(line) - len(line.lstrip())
        if indent <= parent_indent:
            break
        descendants.append(line)
    return descendants


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
    def __init__(
        self,
        executable: str = "playwright-cli",
        budget: int = 4,
        headed: bool = False,
        allowed_hosts: tuple[str, ...] = (),
        allowed_paths: tuple[str, ...] = (),
        read_only: bool = False,
        link_name_pattern: str = "",
    ):
        self.executable, self.budget, self.headed = executable, budget, headed
        self.allowed_hosts = frozenset(allowed_hosts)
        self.allowed_paths = allowed_paths
        self.read_only = read_only
        self.link_name_pattern = re.compile(link_name_pattern) if link_name_pattern else None
        self.session = "jeq-" + uuid.uuid4().hex[:8]
        self.used = 0
        self.current_url = ""
        self.elements: dict[str, Element] = {}

    def run(self, *args: str) -> str:
        result = subprocess.run([self.executable, f"-s={self.session}", *args], text=True, capture_output=True, timeout=30)
        if result.returncode:
            raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "playwright-cli failed")
        return result.stdout

    def open(self, url: str) -> None:
        validated = self._validate_url(url)
        args = ("open", validated, "--headed") if self.headed else ("open", validated)
        self.run(*args)
        self.current_url = validated

    def observe(self) -> dict[str, Element]:
        elements = parse_snapshot(self.run("snapshot"))
        if self.read_only:
            elements = {
                ref: element
                for ref, element in elements.items()
                if element.role == "link"
                and self._link_allowed(element.href)
                and (self.link_name_pattern is None or self.link_name_pattern.fullmatch(element.name.strip()))
            }
        self.elements = elements
        return dict(self.elements)

    def execute(self, action: Action) -> str:
        if self.used >= self.budget:
            raise RuntimeError("action budget exhausted")
        if self.read_only and action.operation not in {"click", "scroll", "wait", "stop"}:
            raise ValueError("remote read-only mode permits navigation controls only")
        element = self.elements.get(action.ref)
        if self.read_only and action.operation == "click" and (element is None or not self._link_allowed(element.href)):
            raise ValueError("link is absent from the remote navigation allowlist")
        command = allow(action, self.elements)
        if command == ["stop"]:
            return "stopped"
        self.used += 1
        if command[0] == "wait":
            time.sleep(min(max(float(command[1]), 0.0), 5.0))
            return "waited"
        result = self.run(*command)
        if self.read_only and action.operation == "click":
            current_url = str(self.evaluate("location.href"))
            self._validate_url(current_url)
            self.current_url = current_url
        return result

    def _validate_url(self, raw_url: str) -> str:
        parsed = urlparse(urljoin(self.current_url or raw_url, raw_url))
        if parsed.username or parsed.password:
            raise ValueError("browser URL must not contain credentials")
        loopback = parsed.hostname in {"127.0.0.1", "localhost", "::1"}
        if loopback and parsed.scheme == "http" and parsed.port:
            return parsed.geturl()
        if parsed.scheme != "https" or parsed.hostname not in self.allowed_hosts or parsed.port not in {None, 443}:
            raise ValueError("browser URL is outside the HTTPS host allowlist")
        if self.allowed_paths and not any(parsed.path == path for path in self.allowed_paths):
            raise ValueError("browser URL path is outside the read-only allowlist")
        return parsed.geturl()

    def _link_allowed(self, href: str) -> bool:
        if not href:
            return False
        try:
            self._validate_url(href)
            return True
        except ValueError:
            return False

    def screenshot(self, filename: str) -> str:
        return self.run("screenshot", f"--filename={filename}")

    def evaluate(self, expression: str):
        output = self.run("eval", expression)
        lines = output.splitlines()
        try:
            value_line = lines[lines.index("### Result") + 1]
        except (ValueError, IndexError) as exc:
            raise RuntimeError(f"unexpected eval output: {output!r}") from exc
        import json
        return json.loads(value_line)

    def inspect(self) -> dict[str, str]:
        # Independent inspection does not trust a model action or snapshot text.
        outcome = {
            "title": str(self.evaluate("document.title")),
            "url": str(self.evaluate("location.href")),
            "body": str(self.evaluate("document.body.innerText")),
        }
        self._validate_url(outcome["url"])
        self.current_url = outcome["url"]
        return outcome

    def verify(self, outcome: dict[str, str]) -> bool:
        return bool(outcome.get("url") and outcome.get("title") and outcome.get("body"))

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
