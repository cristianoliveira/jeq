#!/usr/bin/env python3
"""Validate guide-relative AGENTS.md links within a repository root."""

from __future__ import annotations

import argparse
import json
import os
import re
from dataclasses import asdict, dataclass
from pathlib import Path, PurePosixPath
from urllib.parse import urlsplit

LINK_RE = re.compile(r"\[[^]\n]+\]\(\s*([^\s)]+)(?:\s+[^)]*)?\)")
FENCE_RE = re.compile(r"^\s*(`{3,}|~{3,})")


@dataclass(frozen=True)
class LinkIssue:
    guide: str
    line: int
    target: str
    code: str
    message: str


def find_guides(root: Path) -> list[Path]:
    guides: list[Path] = []
    for directory, child_directories, files in os.walk(root, followlinks=False):
        child_directories[:] = sorted(name for name in child_directories if name != ".git")
        if "AGENTS.md" in files:
            guides.append(Path(directory) / "AGENTS.md")
    return sorted(guides)


def markdown_links(guide: Path) -> list[tuple[int, str]]:
    links: list[tuple[int, str]] = []
    fence: str | None = None
    for line_number, line in enumerate(guide.read_text(encoding="utf-8").splitlines(), start=1):
        fence_match = FENCE_RE.match(line)
        if fence_match:
            marker = fence_match.group(1)
            marker_character = marker[0]
            if fence is None:
                fence = marker_character
            elif fence == marker_character:
                fence = None
            continue
        if fence is not None:
            continue
        links.extend((line_number, match.group(1).strip("<>")) for match in LINK_RE.finditer(line))
    return links


def issue(guide: Path, root: Path, line: int, target: str, code: str, message: str) -> LinkIssue:
    return LinkIssue(
        guide=guide.relative_to(root).as_posix(),
        line=line,
        target=target,
        code=code,
        message=message,
    )


def validate_target(guide: Path, root: Path, line: int, target: str) -> LinkIssue | None:
    parsed_target = urlsplit(target)
    if parsed_target.scheme or parsed_target.netloc or target.startswith("//"):
        return issue(guide, root, line, target, "external-target", "external links are not allowed")

    if parsed_target.query or parsed_target.fragment:
        return issue(guide, root, line, target, "non-file-target", "queries and fragments are not allowed")

    path = PurePosixPath(parsed_target.path)
    if path.is_absolute() or not path.parts:
        return issue(
            guide,
            root,
            line,
            target,
            "non-relative-target",
            "link must be relative to the containing AGENTS.md folder",
        )

    if "\\" in target or path.name != "AGENTS.md":
        return issue(
            guide,
            root,
            line,
            target,
            "wrong-target-name",
            "link must target an AGENTS.md file",
        )

    candidate = guide.parent.joinpath(*path.parts)
    try:
        candidate.resolve().relative_to(root.resolve())
    except ValueError:
        return issue(
            guide,
            root,
            line,
            target,
            "outside-repository-target",
            "link resolves outside the repository root",
        )

    if not candidate.exists():
        return issue(guide, root, line, target, "missing-target", "target does not exist")
    if not candidate.is_file():
        return issue(guide, root, line, target, "non-file-target", "target is not a file")
    return None


def validate(root: Path) -> list[LinkIssue]:
    root = root.resolve()
    issues: list[LinkIssue] = []
    for guide in find_guides(root):
        try:
            links = markdown_links(guide)
        except (OSError, UnicodeDecodeError) as error:
            issues.append(issue(guide, root, 0, "", "unreadable-guide", str(error)))
            continue
        for line, target in links:
            invalid_link = validate_target(guide, root, line, target)
            if invalid_link is not None:
                issues.append(invalid_link)
    return issues


def print_table(issues: list[LinkIssue]) -> None:
    if not issues:
        print("valid\tall AGENTS.md links resolve from their containing folders")
        return
    print("guide\tline\tcode\ttarget\tmessage")
    for invalid_link in issues:
        print(
            f"{invalid_link.guide}\t{invalid_link.line}\t{invalid_link.code}\t"
            f"{invalid_link.target}\t{invalid_link.message}"
        )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=Path, help="repository root to validate")
    parser.add_argument("--format", choices=("table", "json"), default="table")
    args = parser.parse_args()

    root = args.root.resolve()
    if not root.is_dir():
        parser.error(f"repository root is not a directory: {root}")

    issues = validate(root)
    if args.format == "json":
        print(json.dumps([asdict(invalid_link) for invalid_link in issues], indent=2))
    else:
        print_table(issues)
    return 1 if issues else 0


if __name__ == "__main__":
    raise SystemExit(main())
