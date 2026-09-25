#!/usr/bin/env python3
"""Offline-only planner and metric self-check for TASK-0066.

This module has no provider client and never opens a network connection.
"""

from __future__ import annotations

import argparse
import json
import math
import re
import statistics
import sys
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parent
FIXTURES = ROOT / "fixtures.json"
MODEL_VERSION = "jev-1.13.0"
MAX_REQUESTS = 135
PLANNING_REPORTED_INPUT_TOKEN_STOP = 750_000
MAX_SINGLE_REQUEST_TOKENS = 64_000
PLANNING_TOKEN_CEILING = PLANNING_REPORTED_INPUT_TOKEN_STOP + MAX_SINGLE_REQUEST_TOKENS
MAX_CONTEXT_TOKENS_AT_REQUEST_CAP = MAX_REQUESTS * MAX_SINGLE_REQUEST_TOKENS
MAX_TOTAL_REQUEST_BYTES = 24 * 1024
MAX_STATE_AND_LONGEST_QUESTION_BYTES = 12 * 1024
MAX_RETRIES = 0
PRICE_USD_PER_MILLION_INPUT_TOKENS = 0.042
PLANNING_SPEND_AT_TOKEN_CEILING_USD = (
    PLANNING_TOKEN_CEILING * PRICE_USD_PER_MILLION_INPUT_TOKENS / 1_000_000
)
MAX_CONTEXT_LIST_PRICE_ENVELOPE_USD = (
    MAX_CONTEXT_TOKENS_AT_REQUEST_CAP * PRICE_USD_PER_MILLION_INPUT_TOKENS / 1_000_000
)
STRATEGIES = ("per-record", "exact-dedup", "shared-context")
RECORD_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$")
QUESTION_ID = re.compile(r"^[A-Za-z][A-Za-z0-9_-]{0,63}$")


class PlanError(ValueError):
    """The offline plan violates its declared safety or mapping rules."""


@dataclass(frozen=True)
class PlannedRequest:
    workload_id: str
    strategy: str
    request: dict[str, Any]
    routes: tuple[dict[str, str], ...]
    body: bytes
    state_and_longest_question_bytes: int


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def expand_fixture_value(value: Any) -> Any:
    if isinstance(value, dict):
        if set(value) == {"repeat_text", "repeat_count"}:
            text = value["repeat_text"]
            count = value["repeat_count"]
            if not isinstance(text, str) or not isinstance(count, int) or count < 0:
                raise PlanError("invalid deterministic repeat fixture")
            return text * count
        return {key: expand_fixture_value(child) for key, child in value.items()}
    if isinstance(value, list):
        return [expand_fixture_value(child) for child in value]
    return value


def load_fixtures() -> dict[str, Any]:
    try:
        fixtures = json.loads(FIXTURES.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise PlanError(f"cannot load committed fixtures: {error}") from error
    if fixtures.get("schema") != "jeq.map-shapes-fixtures.v1":
        raise PlanError("unsupported fixture schema")
    if fixtures.get("model") != MODEL_VERSION:
        raise PlanError(f"fixtures must pin {MODEL_VERSION}")
    if fixtures.get("data_class") != "synthetic":
        raise PlanError("fixtures must declare data_class=synthetic")
    return expand_fixture_value(fixtures)


def provider_questions(questions: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {
        question_id: {key: value for key, value in question.items() if key != "threshold"}
        for question_id, question in questions.items()
    }


def check_question_ids(questions: dict[str, Any]) -> None:
    for question_id in questions:
        if not QUESTION_ID.fullmatch(question_id):
            raise PlanError(f"question ID is not safe for namespacing: {question_id!r}")


def make_request(model: str, state: Any, questions: dict[str, Any]) -> dict[str, Any]:
    return {"model": model, "state": state, "questions": questions}


def guard_request(request: dict[str, Any]) -> tuple[bytes, int]:
    """Apply a 2x byte/token planning margin, not a billing estimate."""
    state_bytes = canonical_json(request["state"])
    question_sizes = [canonical_json(question) for question in request["questions"].values()]
    longest_question = max(question_sizes, key=len, default=b"")
    state_and_question_bytes = len(state_bytes) + len(longest_question)
    body = canonical_json(request)
    if len(body) > MAX_TOTAL_REQUEST_BYTES:
        raise PlanError(
            f"serialized request is {len(body)} bytes; ceiling is {MAX_TOTAL_REQUEST_BYTES}"
        )
    if state_and_question_bytes > MAX_STATE_AND_LONGEST_QUESTION_BYTES:
        raise PlanError(
            "serialized state plus longest question is "
            f"{state_and_question_bytes} bytes; ceiling is {MAX_STATE_AND_LONGEST_QUESTION_BYTES}"
        )
    return body, state_and_question_bytes


def record_state(workload: dict[str, Any], record: dict[str, Any]) -> Any:
    state = record["state"]
    if "shared_reference" not in workload:
        return state
    return {"shared_reference": workload["shared_reference"], "record": state}


def route(record_id: str, question_id: str, answer_id: str) -> dict[str, str]:
    return {"answer_id": answer_id, "record_id": record_id, "question_id": question_id}


def plan_per_record(workload: dict[str, Any], model: str) -> list[PlannedRequest]:
    questions = provider_questions(workload["questions"])
    planned = []
    for record in workload["records"]:
        request = make_request(model, record_state(workload, record), questions)
        body, size = guard_request(request)
        routes = tuple(route(record["id"], question_id, question_id) for question_id in questions)
        planned.append(PlannedRequest(workload["id"], "per-record", request, routes, body, size))
    return planned


def plan_exact_dedup(workload: dict[str, Any], model: str) -> list[PlannedRequest]:
    questions = provider_questions(workload["questions"])
    groups: dict[bytes, dict[str, Any]] = {}
    for record in workload["records"]:
        request = make_request(model, record_state(workload, record), questions)
        key = canonical_json(request)
        group = groups.setdefault(key, {"request": request, "record_ids": []})
        group["record_ids"].append(record["id"])

    planned = []
    for group in groups.values():
        request = group["request"]
        body, size = guard_request(request)
        routes = tuple(
            route(record_id, question_id, question_id)
            for record_id in group["record_ids"]
            for question_id in questions
        )
        planned.append(PlannedRequest(workload["id"], "exact-dedup", request, routes, body, size))
    return planned


def plan_shared_context(
    workload: dict[str, Any], model: str, *, allow_shared_context: bool
) -> list[PlannedRequest]:
    if not allow_shared_context:
        raise PlanError("shared-context state requires explicit --allow-shared-context opt-in")

    records = workload["records"]
    for record in records:
        if not RECORD_ID.fullmatch(record["id"]):
            raise PlanError(f"record ID is not safe for namespacing: {record['id']!r}")
    check_question_ids(workload["questions"])

    state: dict[str, Any] = {
        "records": [{"record_id": record["id"], "content": record["state"]} for record in records]
    }
    if "shared_reference" in workload:
        state["shared_reference"] = workload["shared_reference"]

    questions: dict[str, Any] = {}
    routes = []
    for record in records:
        for question_id, question in workload["questions"].items():
            answer_id = f"record_{record['id']}__{question_id}"
            namespaced_question = {
                key: value for key, value in question.items() if key != "threshold"
            }
            original = namespaced_question["instructions"]
            namespaced_question["instructions"] = (
                f"Judge only record_id {record['id']} in state.records. "
                f"Use its content and, when relevant, the shared_reference. "
                f"Ignore other record entries. Question: {original}"
            )
            if answer_id in questions:
                raise PlanError(f"namespaced answer ID collision: {answer_id}")
            questions[answer_id] = namespaced_question
            routes.append(route(record["id"], question_id, answer_id))

    request = make_request(model, state, questions)
    body, size = guard_request(request)
    return [PlannedRequest(workload["id"], "shared-context", request, tuple(routes), body, size)]


def map_answers_in_input_order(
    workload: dict[str, Any], planned: list[PlannedRequest], answers_by_request: list[dict[str, Any]]
) -> list[dict[str, Any]]:
    """Route answer IDs back to their exact record/question, then restore input order."""
    if len(planned) != len(answers_by_request):
        raise PlanError("one response map is required for every planned request")
    mapped: dict[tuple[str, str], Any] = {}
    for request_plan, answers in zip(planned, answers_by_request, strict=True):
        expected_ids = {item["answer_id"] for item in request_plan.routes}
        unknown = set(answers) - expected_ids
        if unknown:
            raise PlanError(f"response contains unmapped answer IDs: {sorted(unknown)}")
        for item in request_plan.routes:
            if item["answer_id"] not in answers:
                continue
            key = (item["record_id"], item["question_id"])
            if key in mapped and mapped[key] != answers[item["answer_id"]]:
                raise PlanError(f"conflicting duplicate response mapped to {key!r}")
            mapped[key] = answers[item["answer_id"]]

    result = []
    for record in workload["records"]:
        record_answers = {
            question_id: mapped[(record["id"], question_id)]
            for question_id in workload["questions"]
            if (record["id"], question_id) in mapped
        }
        result.append({"record_id": record["id"], "answers": record_answers})
    return result


def build_matrix(fixtures: dict[str, Any], *, allow_shared_context: bool) -> dict[str, Any]:
    cases = []
    actual_total = 0
    maximum_total = 0
    total_body_bytes = 0
    max_body_bytes = 0
    max_state_question_bytes = 0
    for workload in fixtures["workloads"]:
        per_strategy = {}
        for strategy in STRATEGIES:
            if strategy == "per-record":
                requests = plan_per_record(workload, fixtures["model"])
            elif strategy == "exact-dedup":
                requests = plan_exact_dedup(workload, fixtures["model"])
            else:
                requests = plan_shared_context(
                    workload, fixtures["model"], allow_shared_context=allow_shared_context
                )
            sizes = sorted(len(request.body) for request in requests)
            per_strategy[strategy] = {
                "requests_per_repetition": len(requests),
                "serialized_request_bytes": {
                    "minimum": sizes[0],
                    "median": statistics.median(sizes),
                    "p95": sizes[math.ceil(0.95 * len(sizes)) - 1],
                    "maximum": sizes[-1],
                },
            }
            actual_total += len(requests) * fixtures["repetitions"]
            total_body_bytes += sum(sizes) * fixtures["repetitions"]
            max_body_bytes = max(max_body_bytes, max(sizes, default=0))
            max_state_question_bytes = max(
                max_state_question_bytes,
                max((item.state_and_longest_question_bytes for item in requests), default=0),
            )
        maximum_total += len(workload["records"]) * 2 * fixtures["repetitions"]
        maximum_total += fixtures["repetitions"]
        cases.append({"workload": workload["id"], "requests_per_repetition": per_strategy})

    if actual_total > MAX_REQUESTS or maximum_total > MAX_REQUESTS:
        raise PlanError(f"planned calls exceed the {MAX_REQUESTS} request cap")
    if fixtures["model"] != MODEL_VERSION:
        raise PlanError(f"run matrix must pin {MODEL_VERSION}")
    return {
        "schema": "jeq.map-shape-run-matrix.v1",
        "offline_only": True,
        "model": fixtures["model"],
        "repetitions_per_workload_strategy": fixtures["repetitions"],
        "strategies": list(STRATEGIES),
        "cases": cases,
        "planned_requests_with_exact_dedup": actual_total,
        "maximum_requests_without_dedup_savings": maximum_total,
        "max_requests": MAX_REQUESTS,
        "max_retries": MAX_RETRIES,
        "planning_reported_input_token_stop": PLANNING_REPORTED_INPUT_TOKEN_STOP,
        "planning_token_ceiling_including_one_request_reserve": PLANNING_TOKEN_CEILING,
        "max_input_tokens_per_request": MAX_SINGLE_REQUEST_TOKENS,
        "max_context_tokens_at_request_cap_if_each_call_uses_full_context": (
            MAX_CONTEXT_TOKENS_AT_REQUEST_CAP
        ),
        "price_usd_per_million_input_tokens": PRICE_USD_PER_MILLION_INPUT_TOKENS,
        "planning_spend_at_token_ceiling_usd": round(PLANNING_SPEND_AT_TOKEN_CEILING_USD, 6),
        "max_context_list_price_envelope_usd": round(MAX_CONTEXT_LIST_PRICE_ENVELOPE_USD, 6),
        "reported_usage_does_not_guarantee_provider_billing": True,
        "halts_after_missing_usage_invalid_response_wrong_version_or_retry": True,
        "max_serialized_request_bytes": max_body_bytes,
        "max_state_plus_longest_question_bytes": max_state_question_bytes,
        "serialized_request_bytes_across_matrix": total_body_bytes,
        "byte_limits_are_safety_guards_not_billing_estimates": True,
    }


def calculate_usage_metrics(observations: list[dict[str, Any]]) -> dict[str, Any]:
    """Calculate metrics from injected observations; this function performs no I/O."""
    attempts = sum(item["attempts"] for item in observations)
    successful = [item for item in observations if item["response_valid"]]
    answers = [answer for item in successful for answer in item["answers"]]
    input_tokens = sum(item["input_tokens"] for item in successful if item["input_tokens"] is not None)
    output_tokens = sum(item["output_tokens"] for item in successful if item["output_tokens"] is not None)
    known_input = sum(item["input_tokens"] is not None for item in successful)
    latencies = sorted(item["elapsed_ms"] for item in observations)
    latency_distribution = None
    if latencies:
        latency_distribution = {
            "minimum": latencies[0],
            "median": statistics.median(latencies),
            "p95": latencies[math.ceil(0.95 * len(latencies)) - 1],
            "maximum": latencies[-1],
        }
    threshold_correct = sum(
        (answer["probability"] >= answer["threshold"]) == answer["expected"]
        for answer in answers
    )
    useful = len(answers)
    usage_complete = (
        len(successful) == len(observations)
        and all(item["input_tokens"] is not None and item["attempts"] == 1 for item in successful)
    )
    return {
        "attempted_requests": attempts,
        "successful_responses": len(successful),
        "answers": len(answers),
        "useful_answers": useful,
        "threshold_correct_answers": threshold_correct,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
        "responses_with_reported_input_tokens": known_input,
        "latency_ms_distribution": latency_distribution,
        "useful_answers_per_1000_input_tokens": (
            useful * 1000.0 / input_tokens if usage_complete and input_tokens > 0 else None
        ),
        "threshold_accuracy": threshold_correct / len(answers) if answers else None,
    }


def summarize_probability_runs(
    candidate_answers: list[dict[str, Any]], baseline_answers: list[dict[str, Any]]
) -> dict[str, Any]:
    def grouped(rows: list[dict[str, Any]]) -> dict[tuple[str, str], list[float]]:
        values: dict[tuple[str, str], list[float]] = defaultdict(list)
        for row in rows:
            probability = row["probability"]
            if not 0.0 <= probability <= 1.0:
                raise PlanError("probability must be in [0, 1]")
            values[(row["record_id"], row["question_id"])].append(probability)
        return values

    candidate, baseline = grouped(candidate_answers), grouped(baseline_answers)
    candidate_threshold_correct = [
        (row["probability"] >= row["threshold"]) == row["expected"]
        for row in candidate_answers
    ]
    baseline_threshold_correct = [
        (row["probability"] >= row["threshold"]) == row["expected"]
        for row in baseline_answers
    ]
    candidate_brier = [
        (row["probability"] - float(row["expected"])) ** 2 for row in candidate_answers
    ]
    baseline_brier = [
        (row["probability"] - float(row["expected"])) ** 2 for row in baseline_answers
    ]
    means = {
        f"{record_id}/{question_id}": statistics.fmean(values)
        for (record_id, question_id), values in sorted(candidate.items())
    }
    baseline_sds = [statistics.pstdev(values) for values in baseline.values() if values]
    movements = []
    threshold_changes = []
    baseline_means = {key: statistics.fmean(values) for key, values in baseline.items() if values}
    candidate_means = {key: statistics.fmean(values) for key, values in candidate.items() if values}
    for key in sorted(candidate_means.keys() & baseline_means.keys()):
        movements.append(candidate_means[key] - baseline_means[key])
        threshold = next(
            row["threshold"]
            for row in candidate_answers
            if (row["record_id"], row["question_id"]) == key
        )
        threshold_changes.append(
            (candidate_means[key] >= threshold) != (baseline_means[key] >= threshold)
        )
    return {
        "candidate_threshold_accuracy": (
            statistics.fmean(candidate_threshold_correct) if candidate_threshold_correct else None
        ),
        "baseline_threshold_accuracy": (
            statistics.fmean(baseline_threshold_correct) if baseline_threshold_correct else None
        ),
        "candidate_brier_score": statistics.fmean(candidate_brier) if candidate_brier else None,
        "baseline_brier_score": statistics.fmean(baseline_brier) if baseline_brier else None,
        "candidate_mean_probability_by_answer": means,
        "baseline_mean_within_run_probability_sd": (
            statistics.fmean(baseline_sds) if baseline_sds else None
        ),
        "mean_probability_movement_vs_baseline": statistics.fmean(movements) if movements else None,
        "mean_absolute_probability_movement_vs_baseline": (
            statistics.fmean(abs(value) for value in movements) if movements else None
        ),
        "threshold_decision_disagreement_vs_baseline": (
            sum(threshold_changes) / len(threshold_changes) if threshold_changes else None
        ),
    }


class PaidRunBudget:
    """Offline-tested planning guard; usage reports do not prove final billing."""

    def __init__(self) -> None:
        self.requests = 0
        self.input_tokens = 0
        self.in_flight = False
        self.halted = False
        self.final_request_reserved = False
        self.stop_reason: str | None = None

    def begin(
        self, model: str, request: dict[str, Any], *, max_retries: int = MAX_RETRIES
    ) -> None:
        if self.halted:
            raise PlanError("paid run halted; do not start another request")
        if model != MODEL_VERSION:
            raise PlanError(f"paid run must pin {MODEL_VERSION}")
        if max_retries != MAX_RETRIES:
            raise PlanError(f"paid run must set max_retries={MAX_RETRIES}")
        if request.get("model") != model:
            raise PlanError("request and run budget model differ")
        guard_request(request)
        if self.in_flight:
            raise PlanError("paid requests must be sequential")
        if self.requests >= MAX_REQUESTS:
            self.halted = True
            self.stop_reason = "hard_request_cap"
            raise PlanError(f"paid run reached the {MAX_REQUESTS} request cap")
        if self.input_tokens >= PLANNING_REPORTED_INPUT_TOKEN_STOP:
            if self.final_request_reserved:
                self.halted = True
                self.stop_reason = "final_request_reserve_consumed"
                raise PlanError("paid run already used its final request reserve")
            if self.input_tokens + MAX_SINGLE_REQUEST_TOKENS > PLANNING_TOKEN_CEILING:
                self.halted = True
                self.stop_reason = "planning_token_ceiling"
                raise PlanError("paid run cannot reserve the final request context")
            self.final_request_reserved = True
        elif self.input_tokens + MAX_SINGLE_REQUEST_TOKENS > PLANNING_TOKEN_CEILING:
            self.halted = True
            self.stop_reason = "planning_token_ceiling"
            raise PlanError("paid run cannot reserve the maximum context for another request")
        self.requests += 1
        self.in_flight = True

    def finish(
        self,
        reported_input_tokens: int | None,
        *,
        resolved_model_version: str | None = None,
        response_valid: bool = True,
        actual_attempts: int = 1,
    ) -> None:
        if not self.in_flight:
            raise PlanError("no request is awaiting completion")
        self.in_flight = False
        attempts_valid = (
            isinstance(actual_attempts, int)
            and not isinstance(actual_attempts, bool)
            and actual_attempts == 1
        )
        if not attempts_valid:
            safe_attempts = (
                actual_attempts
                if isinstance(actual_attempts, int) and actual_attempts > 0
                else 1
            )
            self.requests += max(0, safe_attempts - 1)
            self.input_tokens += safe_attempts * MAX_SINGLE_REQUEST_TOKENS
            self.halted = True
            self.stop_reason = "unexpected_retry"
            return
        usage_valid = (
            isinstance(reported_input_tokens, int)
            and not isinstance(reported_input_tokens, bool)
            and 0 <= reported_input_tokens <= MAX_SINGLE_REQUEST_TOKENS
        )
        if (
            not response_valid
            or resolved_model_version != MODEL_VERSION
            or not usage_valid
        ):
            self.input_tokens += MAX_SINGLE_REQUEST_TOKENS
            self.halted = True
            if not response_valid:
                self.stop_reason = "malformed_response"
            elif resolved_model_version != MODEL_VERSION:
                self.stop_reason = "model_mismatch"
            else:
                self.stop_reason = "missing_or_invalid_usage"
            return
        self.input_tokens += reported_input_tokens
        if self.input_tokens > PLANNING_TOKEN_CEILING:
            self.halted = True
            self.stop_reason = "planning_token_ceiling"
            raise PlanError("reported usage exceeded the planning token ceiling")
        if self.final_request_reserved:
            self.halted = True
            self.stop_reason = "final_request_reserve_consumed"


def offline_check(fixtures: dict[str, Any]) -> dict[str, Any]:
    matrix = build_matrix(fixtures, allow_shared_context=True)
    if matrix["planned_requests_with_exact_dedup"] > matrix["maximum_requests_without_dedup_savings"]:
        raise PlanError("deduplication increased the request count")

    workload = next(item for item in fixtures["workloads"] if item["id"] == "exact-duplicates")
    baseline = plan_per_record(workload, fixtures["model"])
    deduplicated = plan_exact_dedup(workload, fixtures["model"])
    if len(baseline) != 4 or len(deduplicated) != 2:
        raise PlanError("exact canonical-request dedup fixture did not produce 4-to-2 requests")

    shared = plan_shared_context(workload, fixtures["model"], allow_shared_context=True)
    response = {
        item["answer_id"]: f"fake:{item['record_id']}:{item['question_id']}"
        for item in reversed(shared[0].routes)
    }
    ordered = map_answers_in_input_order(workload, shared, [response])
    if [item["record_id"] for item in ordered] != [item["id"] for item in workload["records"]]:
        raise PlanError("shared answers did not retain input order")
    for record, output in zip(workload["records"], ordered, strict=True):
        expected = f"fake:{record['id']}:category"
        if output["answers"].get("category") != expected:
            raise PlanError("shared answer ID routed to the wrong record")

    metrics = calculate_usage_metrics(
        [
            {
                "attempts": 1,
                "response_valid": True,
                "input_tokens": 200,
                "output_tokens": 3,
                "elapsed_ms": 10,
                "answers": [{"probability": 0.8, "threshold": 0.5, "expected": True}],
            },
            {
                "attempts": 1,
                "response_valid": True,
                "input_tokens": 0,
                "output_tokens": 2,
                "elapsed_ms": 20,
                "answers": [{"probability": 0.3, "threshold": 0.5, "expected": True}],
            },
        ]
    )
    if metrics["attempted_requests"] != 2 or metrics["useful_answers"] != 2:
        raise PlanError("synthetic usage metric self-check failed")
    if metrics["threshold_correct_answers"] != 1 or metrics["threshold_accuracy"] != 0.5:
        raise PlanError("synthetic threshold-quality metric self-check failed")
    if metrics["useful_answers_per_1000_input_tokens"] != 10.0:
        raise PlanError("synthetic efficiency metric self-check failed")

    budget = PaidRunBudget()
    budget.begin(
        MODEL_VERSION,
        make_request(MODEL_VERSION, {"synthetic": True}, {"q": {"type": "noul", "instructions": "Synthetic?"}}),
    )
    budget.finish(None)
    if budget.input_tokens != MAX_SINGLE_REQUEST_TOKENS:
        raise PlanError("missing usage was not charged against the offline safety budget")

    return {
        "schema": "jeq.map-shape-offline-check.v1",
        "offline_only": True,
        "synthetic_metrics_are_not_provider_quality_or_cost_evidence": True,
        "checks": [
            "five workload fixtures",
            "per-record, exact canonical dedup, and opted-in shared-context plans",
            "exact duplicate request deduplication",
            "namespaced answer routing and original input order",
            "shared-context opt-in guard",
            "per-request byte ceilings",
            "unknown-usage budget reservation",
            "fake usage and quality metric calculations",
        ],
        "matrix": matrix,
        "synthetic_metrics": metrics,
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("check", "plan"), nargs="?", default="check")
    parser.add_argument(
        "--allow-shared-context",
        action="store_true",
        help="opt in to constructing synthetic shared-state plans; no data is sent",
    )
    args = parser.parse_args(argv)
    try:
        fixtures = load_fixtures()
        if args.command == "plan":
            result = build_matrix(fixtures, allow_shared_context=args.allow_shared_context)
        else:
            if not args.allow_shared_context:
                raise PlanError("offline check includes shared state; pass --allow-shared-context")
            result = offline_check(fixtures)
        sys.stdout.write(json.dumps(result, indent=2, sort_keys=True, allow_nan=False) + "\n")
    except (PlanError, OSError, ValueError) as error:
        sys.stderr.write(f"map-shapes offline harness: {error}\n")
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
