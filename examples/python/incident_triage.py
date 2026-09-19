"""Two-stage incident triage with category-scoped runbook selection."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any

from gev_cli import (
    InputFailure,
    add_usage,
    answer,
    ask_composed,
    ask_native,
    read_json_stdin,
    run_workflow,
    usage,
)


ROOT = Path(__file__).parent
STAGE_ONE_QUESTIONS = ROOT / "specs" / "incident-stage-one.json"
RUNBOOKS = ROOT / "specs" / "incident-runbooks.json"
MIN_CONFIDENCE = 0.70
PAGE_AMBIGUITY = (0.30, 0.70)
MAX_SEVERITY_LEVEL = 3


def load_runbooks() -> dict[str, list[dict[str, str]]]:
    with RUNBOOKS.open(encoding="utf-8") as stream:
        document = json.load(stream)
    categories = document.get("categories")
    if not isinstance(categories, dict):
        raise InputFailure("runbook catalog has no categories")
    return categories


def bounded(answer_value: dict[str, Any], field: str, name: str) -> float:
    value = answer_value.get(field)
    if not isinstance(value, (int, float)) or isinstance(value, bool) or not 0 <= value <= 1:
        raise InputFailure(f"{name} {field} must be numeric in [0,1]")
    return float(value)


def score_level(answer_value: dict[str, Any], name: str) -> float:
    """Validate a four-level Score position without flattening its evidence."""

    value = answer_value.get("score")
    if not isinstance(value, (int, float)) or isinstance(value, bool) or not 0 <= value <= MAX_SEVERITY_LEVEL:
        raise InputFailure(f"{name} score must be numeric in [0,{MAX_SEVERITY_LEVEL}]")
    return float(value)


def stage_one(response: dict[str, Any]) -> tuple[dict[str, Any], str | None]:
    """Validate stage-one evidence and return the chosen category if confident."""

    category = answer(response, "category")
    severity = answer(response, "severity")
    page = answer(response, "page")
    category_confidence = bounded(category, "confidence", "category")
    score_level(severity, "severity")
    severity_confidence = bounded(severity, "confidence", "severity")
    page_probability = bounded(page, "noul", "page")
    if category_confidence < MIN_CONFIDENCE or severity_confidence < MIN_CONFIDENCE:
        raise InputFailure("category or severity confidence requires human review")
    if PAGE_AMBIGUITY[0] < page_probability < PAGE_AMBIGUITY[1]:
        raise InputFailure("page probability is ambiguous")
    selected = category.get("choice")
    if not isinstance(selected, str) or not selected:
        raise InputFailure("category choice is missing")
    return {"category": category, "severity": severity, "page": page}, selected


def review_receipt(
    response: dict[str, Any],
    reason: str,
    stage_count: int = 1,
    extra: dict[str, Any] | None = None,
    second: dict[str, Any] | None = None,
) -> tuple[dict[str, Any], int]:
    answers = response.get("answers", {})
    if extra:
        answers = {**answers, **extra}
    total_usage = usage(response)
    model = response.get("model")
    if second is not None:
        total_usage = add_usage(total_usage, usage(second))
        model = second.get("model", model)
    return (
        {
            "workflow": "incident-triage-python",
            "decision": {"action": "human_review", "reason": reason},
            "answers": answers,
            "model": model,
            "usage": total_usage,
            "stage_count": stage_count,
        },
        11,
    )


def native_selection_request(incident: dict[str, Any], candidates: list[dict[str, str]]) -> dict[str, Any]:
    criteria = {candidate["id"]: candidate["summary"] for candidate in candidates}
    return {
        "state": incident,
        "model": os.environ.get("GEV_MODEL", "jev-latest"),
        "questions": {
            "runbook": {
                "type": "choice",
                "instructions": "Select exactly one applicable runbook ID.",
                "criteria": criteria,
            }
        },
    }


def select_runbook(response: dict[str, Any], candidates: list[dict[str, str]]) -> tuple[str, float]:
    selected = answer(response, "runbook")
    runbook_id = selected.get("choice")
    confidence = bounded(selected, "confidence", "runbook")
    allowed = {candidate.get("id") for candidate in candidates}
    if runbook_id not in allowed:
        raise InputFailure("selected runbook is outside the category allowlist")
    if confidence < MIN_CONFIDENCE:
        raise InputFailure("runbook confidence requires human review")
    return runbook_id, confidence


def happy_receipt(
    first: dict[str, Any], second: dict[str, Any], stage_one_answers: dict[str, Any], runbook_id: str
) -> tuple[dict[str, Any], int]:
    return (
        {
            "workflow": "incident-triage-python",
            "decision": {"action": "runbook_selected", "runbook_id": runbook_id, "reason": "two confident stages"},
            "answers": {"stage_one": stage_one_answers, "stage_two": second.get("answers", {})},
            "model": second.get("model"),
            "usage": add_usage(usage(first), usage(second)),
            "stage_count": 2,
        },
        0,
    )


def workflow() -> tuple[dict[str, Any], int]:
    incident = read_json_stdin()
    if not isinstance(incident, dict):
        raise InputFailure("incident must be a JSON object")
    first = ask_composed(STAGE_ONE_QUESTIONS, incident)
    try:
        stage_answers, category = stage_one(first)
    except InputFailure as error:
        return review_receipt(first, error.reason)
    candidates = load_runbooks().get(category)
    if not candidates:
        return review_receipt(first, "category has no versioned runbook allowlist", extra={"category": category})
    second = ask_native(native_selection_request(incident, candidates))
    try:
        runbook_id, _ = select_runbook(second, candidates)
    except InputFailure as error:
        return review_receipt(
            first,
            error.reason,
            stage_count=2,
            extra={"stage_two": second.get("answers", {})},
            second=second,
        )
    return happy_receipt(first, second, stage_answers, runbook_id)


if __name__ == "__main__":
    raise SystemExit(run_workflow(workflow))
