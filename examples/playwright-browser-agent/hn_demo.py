#!/usr/bin/env python3
"""Navigate Hacker News with Jev under a read-only same-host policy."""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import re
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urljoin, urlparse

from agent import BrowserBoundary
from policy import subprocess_runner
from runner import run_agent

HOST = "news.ycombinator.com"
COMMENT_COUNT = re.compile(r"^(\d+)\s+comments?$")
TOP_COMMENTS_EXPRESSION = r'''JSON.stringify(Array.from(document.querySelectorAll("tr.comtr")).filter(function(row){var indent=row.querySelector("td.ind img");return indent && Number(indent.getAttribute("width"))===0;}).slice(0,5).map(function(row){return {user:(row.querySelector(".hnuser")||{}).textContent||"",text:((row.querySelector(".commtext")||{}).innerText||"").slice(0,600)};}))'''


def previous_utc_day() -> str:
    return (datetime.now(timezone.utc).date() - timedelta(days=1)).isoformat()


def find_most_discussed(elements: dict, base_url: str) -> dict:
    discussions = []
    for element in elements.values():
        match = COMMENT_COUNT.match(element.name.strip())
        if element.role != "link" or not match:
            continue
        url = urljoin(base_url, element.href)
        parsed = urlparse(url)
        item_id = parse_qs(parsed.query).get("id", [""])[0]
        if parsed.hostname == HOST and parsed.path == "/item" and item_id:
            discussions.append({"comments": int(match.group(1)), "url": url, "item_id": item_id})
    if not discussions:
        raise RuntimeError("yesterday page exposed no discussion links with comment counts")
    return max(discussions, key=lambda item: item["comments"])


def verify_discussion(outcome: dict[str, str], expected: dict) -> dict[str, bool]:
    parsed = urlparse(outcome.get("url", ""))
    item_id = parse_qs(parsed.query).get("id", [""])[0]
    body = outcome.get("body", "")
    return {
        "https": parsed.scheme == "https",
        "host": parsed.hostname == HOST,
        "item": parsed.path == "/item" and item_id == expected.get("item_id"),
        "title": outcome.get("title", "").endswith(" | Hacker News"),
        "discussion": "reply" in body.lower() and len(body) > 200,
    }


def extract_top_comments(browser: BrowserBoundary, outcome: dict[str, str]) -> dict:
    comments = json.loads(str(browser.evaluate(TOP_COMMENTS_EXPRESSION)))
    return {
        "story": outcome["title"].removesuffix(" | Hacker News"),
        "url": outcome["url"],
        "top_level_comments": comments,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--day", default=previous_utc_day())
    parser.add_argument("--jeq-bin", default=os.environ.get("JEQ_BIN", "jeq"))
    parser.add_argument("--screenshot", type=pathlib.Path, default=pathlib.Path("output/playwright/hacker-news/jev-discussion.png"))
    parser.add_argument("--headed", action=argparse.BooleanOptionalAction, default=True)
    args = parser.parse_args()

    start_url = f"https://{HOST}/front?day={args.day}"
    expected: dict = {}

    def observe(step, elements, outcome):
        if step == 1:
            expected.update(find_most_discussed(elements, outcome["url"]))

    def verify(outcome):
        return verify_discussion(outcome, expected)

    goal = (
        f"On Hacker News's {args.day} front page, open the discussion link with the largest "
        "displayed numeric comment count. Only same-host read-only links are offered. Once the "
        "expected discussion page is visibly open, choose DONE. Do not follow comment permalinks."
    )
    boundary = BrowserBoundary(
        budget=3,
        headed=args.headed,
        allowed_hosts=(HOST,),
        allowed_paths=("/front", "/item"),
        read_only=True,
        link_name_pattern=r"\d+\s+comments?",
    )
    result = run_agent(
        url=start_url,
        goal=goal,
        fill_values=(),
        decision_runner=subprocess_runner(args.jeq_bin),
        verify=verify,
        screenshot=args.screenshot,
        boundary=boundary,
        max_steps=3,
        timeout_seconds=90,
        on_observe=observe,
        finalize=extract_top_comments,
    )
    print(json.dumps({
        "selected": expected,
        "verified": verify_discussion(result.outcome, expected),
        "evidence": result.evidence,
    }, ensure_ascii=False, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
