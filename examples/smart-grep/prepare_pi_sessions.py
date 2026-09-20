#!/usr/bin/env python3
"""Prepare privacy-safe Smartgrep candidates from Pi session JSONL files."""
import argparse
import json
import os
import re
import sys
import time
from collections import Counter, defaultdict

DEFAULT_ROOT = os.path.expanduser("~/.pi/agent/sessions")
TAXONOMY = {
    "edit.validation": ("edit", r"validation failed", "Edit call failed schema validation."),
    "edit.stale": ("edit", r"oldtext must match|could not find edits|oldtext.*not found", "Edit targeted stale source text."),
    "edit.nonunique": ("edit", r"must be unique|multiple matches|not unique|overlap", "Edit target was ambiguous or overlapping."),
    "read.offset": ("read", r"offset .* beyond end", "Read requested an offset beyond the file."),
    "read.missing": ("read", r"enoent|no such file|not found", "Read targeted a missing path."),
    "read.validation": ("read", r"validation failed", "Read call failed schema validation."),
    "bash.timeout": ("bash", r"timed out|timeout", "Shell command exceeded its timeout."),
    "bash.credentials": ("bash", r"credentials in environment", "Credential guard stopped the shell command."),
    "bash.test": ("bash", r"tap version|--- fail:|test failed|tests? failed", "A verification command reported test failure."),
    "bash.missing": ("bash", r"command not found|no such file|cannot find", "Shell command referenced a missing executable or path."),
    "bash.permission": ("bash", r"permission denied|operation not permitted", "Shell command lacked permission."),
    "watcher.unavailable": ("watcher", r"unavailable.*enoent|connect enoent|socket", "Watcher service or socket was unavailable."),
    "watcher.invalid": ("watcher", r"invalid_options|cannot carry|requires .*wait", "Watcher call used invalid options."),
    "watcher.stale": ("watcher", r"\bstale\b", "Watcher result was stale."),
    "watcher.ambiguous": ("watcher", r"ambiguous.*matches|target.*ambiguous", "Watcher target was ambiguous."),
    "watcher.no_target": ("watcher", r"no .*target matching|not matching", "Watcher target matched no target."),
    "provider.abort": ("provider", r"abort(?:ed|ing)?", "Provider operation was aborted."),
    "provider.quota": ("provider", r"usage limit|quota|limit exhausted", "Provider usage or quota limit was exhausted."),
    "provider.socket": ("provider", r"websocket.*closed|socket connection.*closed|connection ended", "Provider socket closed unexpectedly."),
    "provider.auth": ("provider", r"invalid.*token|expired.*token", "Provider authentication failed."),
    "provider.overload": ("provider", r"overload|rate-limited|try again later", "Provider was overloaded or rate limited."),
    "provider.internal": ("provider", r"internal server error", "Provider returned an internal error."),
    "provider.other": ("provider", r".", "Provider or model request failed."),
}

def tool_text(message):
    content = message.get("content")
    if not isinstance(content, list):
        return ""
    return "\n".join(part.get("text", "") for part in content if isinstance(part, dict) and isinstance(part.get("text"), str)).lower()

def classify(tool, text):
    if tool.startswith("watcher_"):
        tool = "watcher"
    if tool not in {"edit", "read", "bash", "watcher", "provider"}:
        return "tool.other", "Tool call failed for another reason."
    for category, (owner, pattern, description) in TAXONOMY.items():
        if owner == tool and re.search(pattern, text):
            return category, description
    if tool == "watcher":
        return "watcher.other", "Watcher call failed for another reason."
    return f"{tool}.other", f"{tool.title()} call failed for another reason."

def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=DEFAULT_ROOT)
    parser.add_argument("--days", type=float, default=7)
    parser.add_argument("--since", type=float)
    parser.add_argument("--until", type=float)
    parser.add_argument("--max-files", type=int, default=1000)
    parser.add_argument("--max-bytes", type=int, default=512 * 1024 * 1024)
    parser.add_argument("--max-candidates", type=int, default=29)
    parser.add_argument("--max-payload-bytes", type=int, default=12 * 1024)
    args = parser.parse_args(argv)
    if args.days <= 0 or args.max_files <= 0 or args.max_bytes <= 0 or args.max_candidates <= 0 or args.max_payload_bytes <= 0:
        raise ValueError("days and caps must be positive")
    if args.since is not None and args.until is not None and args.since > args.until:
        raise ValueError("since must not be later than until")
    now = time.time()
    since = args.since if args.since is not None else now - args.days * 86400
    until = args.until if args.until is not None else now
    files = []
    for base, _, names in os.walk(args.root):
        for name in names:
            if not name.endswith(".jsonl"):
                continue
            path = os.path.join(base, name)
            try:
                if since <= os.stat(path).st_mtime <= until:
                    files.append(path)
            except OSError:
                continue
    files.sort()
    if len(files) > args.max_files:
        raise ValueError("session file cap exceeded")
    events = Counter()
    sessions = defaultdict(set)
    descriptions = {}
    total_bytes = 0
    for path in files:
        total_bytes += os.path.getsize(path)
        if total_bytes > args.max_bytes:
            raise ValueError("session byte cap exceeded")
        with open(path, encoding="utf-8", errors="strict") as source:
            for line in source:
                try:
                    row = json.loads(line)
                except json.JSONDecodeError as error:
                    raise ValueError("malformed session JSONL") from error
                if not isinstance(row, dict) or row.get("type") != "message":
                    continue
                message = row.get("message")
                if not isinstance(message, dict):
                    continue
                if message.get("role") == "assistant" and isinstance(message.get("errorMessage"), str):
                    category, description = classify("provider", message["errorMessage"].lower())
                elif message.get("role") == "toolResult" and message.get("isError") is True:
                    category, description = classify(str(message.get("toolName", "tool")).lower(), tool_text(message))
                else:
                    continue
                events[category] += 1
                sessions[category].add(path)
                descriptions[category] = description
    groups = []
    for category, count in events.items():
        affected = len(sessions[category])
        groups.append((category, count, affected, count - affected))
    groups.sort(key=lambda item: (-item[2], -item[3], item[0]))
    candidates = []
    for category, count, affected, repeats in groups[:args.max_candidates]:
        candidates.append({"id": category, "description": f"{descriptions[category]} Observed {count} events across {affected} session files, with {repeats} repeat events."})
    candidates.append({"id": "none", "description": "None of these classified common failures is a preventable friction worth prioritizing."})
    payload = json.dumps(candidates, separators=(",", ":")) + "\n"
    if len(payload.encode()) > args.max_payload_bytes:
        raise ValueError("candidate payload cap exceeded")
    sys.stdout.write(payload)

if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as error:
        print(f"prepare_pi_sessions.py: {error}", file=sys.stderr)
        raise SystemExit(2)
