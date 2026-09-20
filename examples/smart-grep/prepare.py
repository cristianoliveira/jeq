#!/usr/bin/env python3
"""Locally shortlist log excerpts. This command never makes network requests."""

import argparse
import json
from pathlib import Path
import re
import sys

MAX_INPUT_BYTES = 1024 * 1024
MAX_CANDIDATES = 30
MAX_PAYLOAD_BYTES = 12 * 1024


def prepare(path, pattern, context):
    if not path.is_file():
        raise ValueError("input must be a readable regular file")
    with path.open("rb") as source:
        raw = source.read(MAX_INPUT_BYTES + 1)
    if len(raw) > MAX_INPUT_BYTES:
        raise ValueError("input exceeds 1 MiB; narrow the log locally first")
    lines = raw.decode("utf-8").splitlines()
    matches = re.compile(pattern)
    groups = {}
    for index, line in enumerate(lines):
        if not matches.search(line):
            continue
        # Preserve meaningful differences. No timestamp/ID/number normalization.
        excerpt = "\n".join(lines[index:index + context + 1])
        if excerpt not in groups:
            if len(groups) == MAX_CANDIDATES:
                raise ValueError("more than 30 distinct excerpts; narrow the pattern")
            groups[excerpt] = {
                "id": f"line-{index + 1}",
                "lines": [],
                "description": excerpt,
            }
        groups[excerpt]["lines"].append(index + 1)
    if not groups:
        return None
    candidates = list(groups.values())
    candidates.append({
        "id": "none",
        "description": "None of the log excerpts provides evidence matching the search request.",
    })
    payload = json.dumps(candidates, ensure_ascii=False) + "\n"
    if len(payload.encode("utf-8")) > MAX_PAYLOAD_BYTES:
        raise ValueError("candidate payload exceeds 12 KiB; narrow the log or context")
    return payload


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("log", type=Path)
    parser.add_argument("pattern", help="local Python regex, chosen by you, not the model")
    parser.add_argument("--context", type=int, choices=range(4), default=1,
                        help="following lines per match, 0..3 (default: 1)")
    args = parser.parse_args()
    try:
        payload = prepare(args.log, args.pattern, args.context)
    except (OSError, ValueError, re.error) as error:
        print(f"smart-grep: {error}", file=sys.stderr)
        return 2
    if payload is None:
        print("smart-grep: no local matches; nothing to send", file=sys.stderr)
        return 1
    sys.stdout.write(payload)
    return 0


if __name__ == "__main__":
    sys.exit(main())
