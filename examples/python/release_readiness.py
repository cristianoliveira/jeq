"""Readable release-readiness policy with a zero-call local blocker path."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from gev_cli import InputFailure, answer, ask_composed, read_json_stdin, run_workflow, usage


ROOT = Path(__file__).parent
QUESTIONS = ROOT / "specs" / "release-readiness.json"
RISK_PASS_MAX = 0.30
RISK_CANARY_MAX = 0.60
RISK_BLOCK_MIN = 0.80
MAX_SCORE_LEVEL = 3
MIN_CONFIDENCE = 0.70


def local_blockers(evidence: dict[str, Any]) -> list[str]:
    """Reject known deterministic failures before spending on semantic judgment."""

    blockers: list[str] = []
    if evidence.get("tests_passed") is False or evidence.get("tests_failed"):
        blockers.append("tests_failed")
    tests = evidence.get("tests")
    if isinstance(tests, dict) and (tests.get("passed") is False or tests.get("failed")):
        blockers.append("tests_failed")
    vulnerabilities = evidence.get("known_vulnerabilities", [])
    if isinstance(vulnerabilities, (list, dict, str)) and vulnerabilities:
        blockers.append("known_vulnerabilities")
    if isinstance(vulnerabilities, (int, float)) and vulnerabilities > 0:
        blockers.append("known_vulnerabilities")
    return list(dict.fromkeys(blockers))


def blocker_receipt(blockers: list[str]) -> dict[str, Any]:
    return {
        "workflow": "release-readiness-python",
        "decision": "block",
        "reason": "deterministic local blocker: " + ", ".join(blockers),
        "blockers": blockers,
        "answers": {},
        "model": None,
        "usage": {},
        "stage_count": 0,
    }


def bounded(answer_value: dict[str, Any], field: str, name: str) -> float:
    value = answer_value.get(field)
    if not isinstance(value, (int, float)) or isinstance(value, bool) or not 0 <= value <= 1:
        raise InputFailure(f"{name} {field} must be numeric in [0,1]")
    return float(value)


def normalize_score(answer_value: dict[str, Any], name: str) -> tuple[float, float]:
    """Convert a four-level Score position (0..3) to the policy scale (0..1)."""

    raw = answer_value.get("score")
    if not isinstance(raw, (int, float)) or isinstance(raw, bool) or not 0 <= raw <= MAX_SCORE_LEVEL:
        raise InputFailure(f"{name} score must be numeric in [0,{MAX_SCORE_LEVEL}]")
    normalized = round(float(raw) / MAX_SCORE_LEVEL, 6)
    return float(raw), normalized


def judgments(response: dict[str, Any]) -> tuple[dict[str, Any], float, float, str, float, float]:
    risk = answer(response, "risk")
    manual = answer(response, "manual_review")
    rollout = answer(response, "rollout")
    raw_risk_score, risk_score = normalize_score(risk, "risk")
    risk_confidence = bounded(risk, "confidence", "risk")
    manual_probability = bounded(manual, "noul", "manual_review")
    rollout_choice = rollout.get("choice")
    rollout_confidence = bounded(rollout, "confidence", "rollout")
    if rollout_choice not in {"full", "canary", "hold"}:
        raise InputFailure("rollout choice is outside the policy allowlist")
    if risk_confidence < MIN_CONFIDENCE or rollout_confidence < MIN_CONFIDENCE:
        raise InputFailure("risk or rollout confidence is below the policy floor")
    return (
        {"risk": risk, "manual_review": manual, "rollout": rollout},
        risk_score,
        manual_probability,
        rollout_choice,
        rollout_confidence,
        raw_risk_score,
    )


def policy(response: dict[str, Any]) -> tuple[str, str, dict[str, Any], dict[str, float]]:
    evidence, risk, manual, rollout, rollout_confidence, raw_risk = judgments(response)
    trace = {"risk_raw_score": raw_risk, "risk_normalized": risk}
    if risk >= RISK_BLOCK_MIN or manual >= RISK_BLOCK_MIN or rollout == "hold":
        return "block", "risk, manual review, or rollout policy blocks release", evidence, trace
    if risk <= RISK_PASS_MAX and manual <= RISK_PASS_MAX and rollout == "full":
        return "pass", "low risk supports full rollout", evidence, trace
    if risk <= RISK_CANARY_MAX and manual <= RISK_CANARY_MAX and rollout == "canary":
        return "canary", f"bounded risk supports canary rollout at confidence {rollout_confidence:.2f}", evidence, trace
    return "review", "evidence does not satisfy pass or canary policy", evidence, trace


def receipt(response: dict[str, Any]) -> tuple[dict[str, Any], int]:
    try:
        decision, reason, evidence, trace = policy(response)
        status = 0 if decision in {"pass", "canary"} else 10
    except InputFailure as error:
        decision, reason, status = "uncertain", error.reason, 11
        evidence = response.get("answers", {})
        trace = {}
    return (
        {
            "workflow": "release-readiness-python",
            "decision": decision,
            "reason": reason,
            "answers": evidence,
            "model": response.get("model"),
            "usage": usage(response),
            "stage_count": 1,
            "policy_trace": trace,
            "thresholds": {
                "risk_pass_max": RISK_PASS_MAX,
                "risk_canary_max": RISK_CANARY_MAX,
                "risk_block_min": RISK_BLOCK_MIN,
                "minimum_confidence": MIN_CONFIDENCE,
                "score_max_level": MAX_SCORE_LEVEL,
                "normalization": "raw_score / score_max_level",
            },
        },
        status,
    )


def workflow() -> tuple[dict[str, Any], int]:
    evidence = read_json_stdin()
    if not isinstance(evidence, dict):
        raise InputFailure("release evidence must be a JSON object")
    blockers = local_blockers(evidence)
    if blockers:
        return blocker_receipt(blockers), 10
    return receipt(ask_composed(QUESTIONS, evidence))


if __name__ == "__main__":
    raise SystemExit(run_workflow(workflow))
