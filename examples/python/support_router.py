"""Readable one-stage support routing workflow."""

from __future__ import annotations

import sys
from pathlib import Path
from typing import Any

from gev_cli import InputFailure, answer, ask_composed, run_workflow, usage


ROOT = Path(__file__).parent
QUESTIONS = ROOT / "specs" / "support-routing.json"
ROUTE_ACTIONS = {
    "billing": "billing_queue",
    "technical": "technical_queue",
    "sales": "sales_queue",
}
MIN_ROUTE_CONFIDENCE = 0.70


def route_judgment(ticket: str) -> dict[str, Any]:
    """Ask independent routing questions in one paid call."""

    return ask_composed(QUESTIONS, ticket)


def number(answer_value: dict[str, Any], field: str, name: str) -> float:
    value = answer_value.get(field)
    if not isinstance(value, (int, float)) or isinstance(value, bool):
        raise InputFailure(f"{name} answer has no numeric {field}")
    return float(value)


def choose_action(route: dict[str, Any]) -> tuple[str, str]:
    """Apply confidence policy before consulting the closed action map."""

    choice = route.get("choice")
    confidence = number(route, "confidence", "route")
    if confidence < MIN_ROUTE_CONFIDENCE:
        return "human_review", "route confidence is below the review threshold"
    if choice not in ROUTE_ACTIONS:
        return "human_review", "route is outside the allowlist"
    return ROUTE_ACTIONS[choice], "confident allowlisted route"


def build_receipt(response: dict[str, Any]) -> dict[str, Any]:
    """Keep typed answers and accounting evidence, never the ticket text."""

    route = answer(response, "route")
    urgent = answer(response, "urgent")
    escalate = answer(response, "escalate")
    action, reason = choose_action(route)
    return {
        "workflow": "support-router-python",
        "decision": {"action": action, "reason": reason},
        "answers": {"route": route, "urgent": urgent, "escalate": escalate},
        "model": response.get("model"),
        "usage": usage(response),
        "stage_count": 1,
    }


def workflow() -> tuple[dict[str, Any], int]:
    ticket = sys.stdin.read()
    if not ticket.strip():
        raise InputFailure("stdin ticket is empty")
    return build_receipt(route_judgment(ticket)), 0


if __name__ == "__main__":
    raise SystemExit(run_workflow(workflow))
