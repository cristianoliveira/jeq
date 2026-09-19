#!/usr/bin/env python3
"""Validate AGENTS.md source landmarks against public source declarations."""

from __future__ import annotations

import argparse
import ast
import json
import re
from dataclasses import asdict, dataclass
from pathlib import Path, PurePosixPath

from validate_agents_links import FENCE_RE, find_guides

HEADING_RE = re.compile(r"^(#{1,6})\s+(Landmarks|Boundary flows)\s*$", re.IGNORECASE)
ANY_HEADING_RE = re.compile(r"^(#{1,6})\s+")
BULLET_RE = re.compile(r"^\s*[-*+]\s+")
LANDMARK_PATTERN = (
    r"`([^`:\n]+(?:/[^`:\n]+)*):"
    r"([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`"
)
LANDMARK_RE = re.compile(LANDMARK_PATTERN)
HANDOFF_RE = re.compile(
    rf"^\s*[-*+]\s+Information flow:\s+{LANDMARK_PATTERN}\s*->\s*"
    rf"{LANDMARK_PATTERN}\s+via\s+{LANDMARK_PATTERN}\s*;\s*value:\s*`[^`]+`\.\s*$",
    re.IGNORECASE,
)
MAX_SOURCE_BYTES = 2_000_000
SCRIPT_EXTENSIONS = {".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"}


@dataclass(frozen=True)
class LandmarkIssue:
    guide: str
    line: int
    landmark: str
    code: str
    message: str


def issue(
    guide: Path,
    root: Path,
    line: int,
    landmark: str,
    code: str,
    message: str,
) -> LandmarkIssue:
    return LandmarkIssue(
        guide=guide.relative_to(root).as_posix(),
        line=line,
        landmark=landmark,
        code=code,
        message=message,
    )


NavigationEntry = tuple[int, str | None, str, str]


def updated_fence(fence: str | None, line: str) -> tuple[str | None, bool]:
    fence_match = FENCE_RE.match(line)
    if fence_match is None:
        return fence, False
    marker = fence_match.group(1)[0]
    if fence is None:
        return marker, True
    return (None if fence == marker else fence), True


def navigation_bullet(section: str, line_number: int, line: str) -> list[NavigationEntry]:
    if section != "boundary flows":
        match = LANDMARK_RE.search(line)
        landmark = match.group(0) if match else None
        return [(line_number, landmark, line.strip(), "invalid-landmark")]
    if re.fullmatch(HANDOFF_RE.pattern, line, HANDOFF_RE.flags) is None:
        return [(line_number, None, line.strip(), "invalid-handoff")]
    return [
        (line_number, match.group(0), line.strip(), "")
        for match in re.finditer(LANDMARK_RE.pattern, line)
    ]


def landmarks(guide: Path) -> list[NavigationEntry]:
    results: list[NavigationEntry] = []
    heading_level: int | None = None
    section = ""
    fence: str | None = None
    lines = guide.read_text(encoding="utf-8").splitlines()
    for line_number, line in enumerate(lines, start=1):
        fence, is_fence_line = updated_fence(fence, line)
        if is_fence_line or fence is not None:
            continue

        heading_match = HEADING_RE.match(line.strip())
        if heading_match:
            heading_level = len(heading_match.group(1))
            section = heading_match.group(2).lower()
            continue
        any_heading = ANY_HEADING_RE.match(line.strip())
        if any_heading:
            if heading_level is not None and len(any_heading.group(1)) <= heading_level:
                heading_level = None
                section = ""
            continue
        if heading_level is not None and BULLET_RE.match(line):
            results.extend(navigation_bullet(section, line_number, line))
    return results


def go_declarations(source: str) -> dict[str, bool]:
    declarations: dict[str, bool] = {}
    for match in re.finditer(r"(?m)^\s*func\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\[[^]]+\])?\s*\(", source):
        name = match.group(1)
        declarations[name] = name == "main" or name[0].isupper()
    for match in re.finditer(
        r"(?m)^\s*func\s*\(\s*[A-Za-z_][A-Za-z0-9_]*\s+\*?([A-Za-z_][A-Za-z0-9_]*)[^)]*\)\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(",
        source,
    ):
        receiver, name = match.groups()
        declarations[f"{receiver}.{name}"] = receiver[0].isupper() and name[0].isupper()
    for match in re.finditer(r"(?m)^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\b", source):
        name = match.group(1)
        declarations[name] = name[0].isupper()
    return declarations


def script_declarations(source: str) -> tuple[set[str], set[str]]:
    declaration = r"(?:class|function|interface|type|enum|const|let|var)"
    identifier = r"([A-Za-z_$][A-Za-z0-9_$]*)"
    exported = {
        match.group(1)
        for match in re.finditer(
            rf"(?m)^\s*export\s+(?:default\s+)?(?:abstract\s+)?{declaration}\s+{identifier}",
            source,
        )
    }
    declared = {
        match.group(1)
        for match in re.finditer(
            rf"(?m)^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?{declaration}\s+{identifier}",
            source,
        )
    }
    return exported, declared


def script_member_status(source: str, owner_is_exported: bool, member: str) -> bool | None:
    member_match = re.search(
        rf"(?m)^\s*(?:(public|protected|private)\s+)?(?:static\s+)?(?:async\s+)?{re.escape(member)}\s*(?:<[^>]+>)?\s*\(",
        source,
    )
    if member_match is None:
        return None
    return owner_is_exported and member_match.group(1) != "private"


def script_declaration(source: str, symbol: str) -> bool | None:
    parts = symbol.split(".")
    exported, declared = script_declarations(source)
    if len(parts) == 1:
        if symbol in exported:
            return True
        return False if symbol in declared else None

    owner, member = parts[0], parts[-1]
    if owner not in declared:
        return None
    return script_member_status(source, owner in exported, member)


def python_declarations(source: str) -> dict[str, bool]:
    declarations: dict[str, bool] = {}
    try:
        module = ast.parse(source)
    except SyntaxError:
        return declarations
    for node in module.body:
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            declarations[node.name] = not node.name.startswith("_")
            continue
        if not isinstance(node, ast.ClassDef):
            continue
        declarations[node.name] = not node.name.startswith("_")
        for child in node.body:
            if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                declarations[f"{node.name}.{child.name}"] = (
                    not node.name.startswith("_") and not child.name.startswith("_")
                )
    return declarations


def declaration_status(source_path: Path, symbol: str, source: str) -> bool | None:
    if source_path.suffix == ".go":
        return go_declarations(source).get(symbol)
    if source_path.suffix in SCRIPT_EXTENSIONS:
        return script_declaration(source, symbol)
    if source_path.suffix == ".py":
        return python_declarations(source).get(symbol)
    return None


def validate_landmark(
    guide: Path,
    root: Path,
    line: int,
    raw_landmark: str,
) -> LandmarkIssue | None:
    match = re.fullmatch(LANDMARK_RE.pattern, raw_landmark)
    if match is None:
        return issue(guide, root, line, raw_landmark, "invalid-landmark", "use `path:PublicSymbol`")
    raw_path, symbol = match.groups()
    path = PurePosixPath(raw_path)
    if path.is_absolute() or not path.parts or path.parts[0] == "." or ".." in path.parts or "\\" in raw_path:
        return issue(guide, root, line, raw_landmark, "unsafe-source", "source path must remain under repository root")

    candidate = root.joinpath(*path.parts)
    try:
        resolved = candidate.resolve()
        resolved.relative_to(root.resolve())
    except (OSError, ValueError):
        return issue(guide, root, line, raw_landmark, "unsafe-source", "source path resolves outside repository root")
    if not resolved.is_file():
        return issue(guide, root, line, raw_landmark, "missing-source", "source file does not exist")
    if resolved.stat().st_size > MAX_SOURCE_BYTES:
        return issue(guide, root, line, raw_landmark, "source-too-large", "source file exceeds 2 MB")
    try:
        source = resolved.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as error:
        return issue(guide, root, line, raw_landmark, "unreadable-source", str(error))

    status = declaration_status(resolved, symbol, source)
    if status is True:
        return None
    if status is False:
        return issue(guide, root, line, raw_landmark, "non-public-symbol", "symbol exists but is not public")
    return issue(guide, root, line, raw_landmark, "missing-symbol", "public symbol declaration does not exist")


def validate(root: Path) -> list[LandmarkIssue]:
    root = root.resolve()
    issues: list[LandmarkIssue] = []
    for guide in find_guides(root):
        try:
            entries = landmarks(guide)
        except (OSError, UnicodeDecodeError) as error:
            issues.append(issue(guide, root, 0, "", "unreadable-guide", str(error)))
            continue
        for line, raw_landmark, raw_line, invalid_code in entries:
            if raw_landmark is None:
                expected = (
                    "use `path:PublicSymbol`"
                    if invalid_code == "invalid-landmark"
                    else "use producer -> consumer via orchestrator; value syntax"
                )
                issues.append(issue(guide, root, line, raw_line, invalid_code, expected))
                continue
            invalid = validate_landmark(guide, root, line, raw_landmark)
            if invalid is not None:
                issues.append(invalid)
    return issues


def print_table(issues: list[LandmarkIssue]) -> None:
    if not issues:
        print("valid\tall AGENTS.md landmarks resolve to public source declarations")
        return
    print("guide\tline\tcode\tlandmark\tmessage")
    for invalid in issues:
        print(f"{invalid.guide}\t{invalid.line}\t{invalid.code}\t{invalid.landmark}\t{invalid.message}")


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
        print(json.dumps([asdict(invalid) for invalid in issues], indent=2))
    else:
        print_table(issues)
    return 1 if issues else 0


if __name__ == "__main__":
    raise SystemExit(main())
