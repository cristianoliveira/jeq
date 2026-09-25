#!/usr/bin/env python3
"""Explicitly gated TASK-0066 executor; importing it never performs I/O."""

from __future__ import annotations

import argparse
import base64
import hashlib
import http.client
import json
import math
import os
import ssl
import stat
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Protocol

import harness

ROOT = Path(__file__).resolve().parent
REPO_ROOT = ROOT.parents[1]
FIXTURES = ROOT / "fixtures.json"
PRIVATE_DIR = ROOT / "private"
EXPECTED_FIXTURE_SHA256 = "119868552bbccfd9b0287153b2d1ce33fc5d50cc374900834db4b3440c91f841"
API_HOST = "api.typesafe.ai"
API_PATH = "/v1/systemone"
HTTP_TIMEOUT_SECONDS = 20
MAX_RESPONSE_BYTES = 8 << 20


class ExecutionError(RuntimeError):
    """A safe-to-display executor failure without request or credential data."""


class TransportFailure(RuntimeError):
    def __init__(self, code: str = "transport_error", attempts: int = 1) -> None:
        super().__init__(code)
        self.code = code
        self.attempts = attempts


@dataclass(frozen=True)
class RunCall:
    repetition: int
    workload: dict[str, Any]
    strategy: str
    planned: harness.PlannedRequest


@dataclass(frozen=True)
class PreparedRun:
    fixture_bytes: bytes
    fixture_sha256: str
    allow_shared_context: bool
    calls: tuple[RunCall, ...]


@dataclass(frozen=True)
class TransportResponse:
    status: int
    body: bytes
    elapsed_ms: int
    attempts: int


@dataclass(frozen=True)
class DecodedResponse:
    model: str
    input_tokens: int
    output_tokens: int
    answers: dict[str, dict[str, Any]]


@dataclass(frozen=True)
class ExecutionOutcome:
    completed_requests: int
    attempted_requests: int
    input_tokens: int
    stop_reason: str


class Transport(Protocol):
    def post(self, body: bytes) -> TransportResponse: ...


def _decode_fixture_bytes(raw: bytes) -> dict[str, Any]:
    digest = hashlib.sha256(raw).hexdigest()
    if digest != EXPECTED_FIXTURE_SHA256:
        raise harness.PlanError("refusing to use anything except the pinned synthetic corpus")
    try:
        fixtures = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise harness.PlanError("pinned synthetic corpus is not valid JSON") from error
    if fixtures.get("schema") != "jeq.map-shapes-fixtures.v1":
        raise harness.PlanError("unsupported fixture schema")
    if fixtures.get("model") != harness.MODEL_VERSION:
        raise harness.PlanError(f"synthetic corpus must pin {harness.MODEL_VERSION}")
    if fixtures.get("data_class") != "synthetic":
        raise harness.PlanError("paid executor accepts synthetic fixtures only")
    return harness.expand_fixture_value(fixtures)


def _plans_for(workload: dict[str, Any], *, allow_shared_context: bool):
    if not allow_shared_context:
        raise harness.PlanError("executor requires explicit --allow-shared-context opt-in")
    return {
        "per-record": harness.plan_per_record(workload, harness.MODEL_VERSION),
        "exact-dedup": harness.plan_exact_dedup(workload, harness.MODEL_VERSION),
        "shared-context": harness.plan_shared_context(
            workload, harness.MODEL_VERSION, allow_shared_context=True
        ),
    }


def _build_calls(fixtures: dict[str, Any], *, allow_shared_context: bool) -> tuple[RunCall, ...]:
    calls = []
    for repetition in range(1, fixtures["repetitions"] + 1):
        for workload in fixtures["workloads"]:
            for strategy, plans in _plans_for(
                workload, allow_shared_context=allow_shared_context
            ).items():
                calls.extend(
                    RunCall(repetition, workload, strategy, plan) for plan in plans
                )
    if len(calls) != 129 or len(calls) > harness.MAX_REQUESTS:
        raise harness.PlanError("pinned fixture call count differs from the approved matrix")
    return tuple(calls)


def verify_fixture_is_committed() -> None:
    relative = FIXTURES.resolve().relative_to(REPO_ROOT.resolve()).as_posix()
    result = subprocess.run(
        ["git", "show", f"HEAD:{relative}"],
        cwd=REPO_ROOT,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    committed_digest = hashlib.sha256(result.stdout).hexdigest()
    if result.returncode != 0 or committed_digest != EXPECTED_FIXTURE_SHA256:
        raise ExecutionError("the pinned synthetic fixture corpus must be committed at HEAD")


def prepare_run(*, allow_shared_context: bool) -> PreparedRun:
    """Load only the hash-pinned committed corpus and build its fixed run matrix."""
    if not allow_shared_context:
        raise harness.PlanError("executor requires explicit --allow-shared-context opt-in")
    try:
        raw = FIXTURES.read_bytes()
    except OSError as error:
        raise harness.PlanError("cannot read pinned synthetic corpus") from error
    fixtures = _decode_fixture_bytes(raw)
    digest = hashlib.sha256(raw).hexdigest()
    return PreparedRun(
        raw,
        digest,
        allow_shared_context,
        _build_calls(fixtures, allow_shared_context=allow_shared_context),
    )


def _validated_calls(
    prepared: PreparedRun,
    calls: tuple[RunCall, ...] | list[RunCall],
) -> tuple[RunCall, ...]:
    digest = hashlib.sha256(prepared.fixture_bytes).hexdigest()
    if digest != EXPECTED_FIXTURE_SHA256 or digest != prepared.fixture_sha256:
        raise harness.PlanError("run plan does not use the pinned synthetic corpus")
    fixtures = _decode_fixture_bytes(prepared.fixture_bytes)
    expected = _build_calls(fixtures, allow_shared_context=prepared.allow_shared_context)
    if prepared.calls != expected:
        raise harness.PlanError("run plan differs from the pinned synthetic corpus")
    chosen = tuple(calls)
    if chosen != expected[: len(chosen)]:
        raise harness.PlanError("executor accepts only the deterministic plan prefix")
    if not chosen:
        raise harness.PlanError("run plan has no requests")
    if len(chosen) > harness.MAX_REQUESTS:
        raise harness.PlanError("run plan exceeds the hard request cap")
    for call in chosen:
        request = call.planned.request
        if request.get("model") != harness.MODEL_VERSION:
            raise harness.PlanError(f"executor must pin {harness.MODEL_VERSION}")
        body, state_question_bytes = harness.guard_request(request)
        if body != call.planned.body:
            raise harness.PlanError("request bytes differ from the validated plan")
        if _contains_key(request, "expected"):
            raise harness.PlanError("expected labels must never enter provider requests")
        if len(body) > harness.MAX_TOTAL_REQUEST_BYTES:
            raise harness.PlanError("request exceeds the total-byte guard")
        if state_question_bytes > harness.MAX_STATE_AND_LONGEST_QUESTION_BYTES:
            raise harness.PlanError("request exceeds the state/question-byte guard")
    return chosen


def _contains_key(value: Any, target: str) -> bool:
    if isinstance(value, dict):
        return target in value or any(
            _contains_key(child, target) for child in value.values()
        )
    if isinstance(value, list):
        return any(_contains_key(child, target) for child in value)
    return False


def preflight_summary(prepared: PreparedRun) -> dict[str, Any]:
    calls = _validated_calls(prepared, prepared.calls)
    matrix = harness.build_matrix(
        _decode_fixture_bytes(prepared.fixture_bytes), allow_shared_context=True
    )
    return {
        "schema": "jeq.map-shape-paid-run-preflight.v1",
        "dry_run": True,
        "network_accessed": False,
        "paid_calls_made": 0,
        "paid_execution_authorized": False,
        "authorization_required": True,
        "synthetic_only": True,
        "synthetic_corpus_sha256": prepared.fixture_sha256,
        "committed_corpus_required_for_execution": True,
        "model": harness.MODEL_VERSION,
        "planned_requests": len(calls),
        "hard_attempt_cap": harness.MAX_REQUESTS,
        "sequential": True,
        "max_retries": 0,
        "max_request_bytes": harness.MAX_TOTAL_REQUEST_BYTES,
        "max_state_plus_longest_question_bytes": harness.MAX_STATE_AND_LONGEST_QUESTION_BYTES,
        "largest_planned_request_bytes": matrix["max_serialized_request_bytes"],
        "largest_planned_state_plus_question_bytes": matrix[
            "max_state_plus_longest_question_bytes"
        ],
        "all_planned_requests_pass_byte_guards": True,
        "shared_context_opted_in": prepared.allow_shared_context,
        "planning_token_stop_target": harness.PLANNING_REPORTED_INPUT_TOKEN_STOP,
        "planning_token_ceiling": harness.PLANNING_TOKEN_CEILING,
        "planning_spend_estimate_usd": harness.PLANNING_SPEND_AT_TOKEN_CEILING_USD,
        "input_price_usd_per_million_tokens": harness.PRICE_USD_PER_MILLION_INPUT_TOKENS,
        "full_context_tokens_at_attempt_cap": harness.MAX_CONTEXT_TOKENS_AT_REQUEST_CAP,
        "full_context_list_price_envelope_usd": harness.MAX_CONTEXT_LIST_PRICE_ENVELOPE_USD,
        "provider_billing_is_not_guaranteed": True,
        "next_step": (
            "review this summary, refresh provider limits and price, "
            "then obtain explicit authorization"
        ),
    }


def _reject_duplicate_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON object key")
        result[key] = value
    return result


def decode_response(body: bytes, expected_answer_ids: set[str]) -> DecodedResponse:
    if len(body) > MAX_RESPONSE_BYTES:
        raise ValueError("response exceeds size limit")
    document = json.loads(body.decode("utf-8"), object_pairs_hook=_reject_duplicate_keys)
    if not isinstance(document, dict):
        raise ValueError("response must be a JSON object")
    model = document.get("model")
    answers = document.get("answers")
    usage = document.get("usage")
    if not isinstance(model, str) or not isinstance(answers, dict) or not isinstance(usage, dict):
        raise ValueError("response is missing model, answers, or usage")
    input_tokens = usage.get("input_tokens")
    output_tokens = usage.get("output_tokens")
    if (
        not isinstance(input_tokens, int)
        or isinstance(input_tokens, bool)
        or not 0 <= input_tokens <= harness.MAX_SINGLE_REQUEST_TOKENS
        or not isinstance(output_tokens, int)
        or isinstance(output_tokens, bool)
        or output_tokens < 0
    ):
        raise ValueError("response usage is invalid")
    if set(answers) != expected_answer_ids:
        raise ValueError("response answer IDs do not match the request")
    validated_answers = {}
    for answer_id, answer in answers.items():
        if not isinstance(answer, dict) or answer.get("type") != "noul":
            raise ValueError("response contains an invalid answer")
        probability = answer.get("noul")
        if (
            not isinstance(probability, (int, float))
            or isinstance(probability, bool)
            or not math.isfinite(probability)
            or not 0 <= probability <= 1
        ):
            raise ValueError("response probability is invalid")
        validated_answers[answer_id] = answer
    return DecodedResponse(model, input_tokens, output_tokens, validated_answers)


def _answer_evidence(
    call: RunCall,
    answers: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    evidence = []
    records = {record["id"]: record for record in call.workload["records"]}
    for route in call.planned.routes:
        question_id = route["question_id"]
        answer = answers[route["answer_id"]]
        evidence.append(
            {
                "record_id": route["record_id"],
                "question_id": question_id,
                "probability": answer["noul"],
                "threshold": call.workload["questions"][question_id]["threshold"],
                "expected": records[route["record_id"]]["expected"][question_id],
            }
        )
    return evidence


def execute_calls(
    prepared: PreparedRun,
    calls: tuple[RunCall, ...] | list[RunCall],
    transport: Transport,
    results_sink: Callable[[dict[str, Any]], None],
    *,
    credential: str | None = None,
) -> ExecutionOutcome:
    """Execute an explicitly selected plan prefix through an injected single-call transport."""
    validated = _validated_calls(prepared, calls)
    if results_sink is None:
        raise ExecutionError("private results sink is required before execution")
    budget = harness.PaidRunBudget()
    completed = 0
    for call in validated:
        try:
            budget.begin(
                harness.MODEL_VERSION,
                call.planned.request,
                max_retries=harness.MAX_RETRIES,
            )
        except harness.PlanError:
            if budget.halted:
                break
            raise
        try:
            response = transport.post(call.planned.body)
        except TransportFailure as error:
            budget.finish(None, response_valid=False, actual_attempts=error.attempts)
            budget.stop_reason = error.code
            results_sink(_failure_event(call, error.code, error.attempts))
            break
        except Exception:
            budget.finish(None, response_valid=False)
            budget.stop_reason = "transport_error"
            results_sink(_failure_event(call, "transport_error", 1))
            break

        if not isinstance(response, TransportResponse):
            budget.finish(None, response_valid=False)
            budget.stop_reason = "invalid_transport_response"
            results_sink(_failure_event(call, "invalid_transport_response", 1))
            break
        if (
            not isinstance(response.attempts, int)
            or isinstance(response.attempts, bool)
            or response.attempts != 1
        ):
            budget.finish(None, response_valid=False, actual_attempts=response.attempts)
            results_sink(_failure_event(call, "unexpected_retry", response.attempts))
            break
        if (
            not isinstance(response.status, int)
            or isinstance(response.status, bool)
            or not isinstance(response.elapsed_ms, int)
            or isinstance(response.elapsed_ms, bool)
            or response.elapsed_ms < 0
            or not isinstance(response.body, bytes)
        ):
            budget.finish(None, response_valid=False)
            budget.stop_reason = "invalid_transport_response"
            results_sink(_failure_event(call, "invalid_transport_response", 1))
            break
        if len(response.body) > MAX_RESPONSE_BYTES:
            budget.finish(None, response_valid=False)
            budget.stop_reason = "response_too_large"
            results_sink(_failure_event(call, "response_too_large", response.attempts))
            break
        if credential and credential.encode() in response.body:
            budget.finish(None, response_valid=False)
            budget.stop_reason = "credential_echo_rejected"
            results_sink(_failure_event(call, "credential_echo_rejected", response.attempts))
            break
        if response.status != 200:
            budget.finish(None, response_valid=False)
            budget.stop_reason = "provider_status"
            results_sink(
                _failure_event(call, "provider_status", response.attempts, response.body)
            )
            break
        try:
            decoded = decode_response(
                response.body,
                {route["answer_id"] for route in call.planned.routes},
            )
        except (UnicodeDecodeError, json.JSONDecodeError, ValueError, TypeError):
            budget.finish(None, response_valid=False)
            results_sink(
                _failure_event(call, "malformed_response", response.attempts, response.body)
            )
            break
        if decoded.model != harness.MODEL_VERSION:
            budget.finish(
                decoded.input_tokens,
                resolved_model_version=decoded.model,
                response_valid=True,
            )
            results_sink(
                _failure_event(call, "model_mismatch", response.attempts, response.body)
            )
            break
        budget.finish(
            decoded.input_tokens,
            resolved_model_version=decoded.model,
            response_valid=True,
        )
        results_sink(
            {
                "status": "ok",
                "repetition": call.repetition,
                "workload": call.workload["id"],
                "strategy": call.strategy,
                "request_sha256": hashlib.sha256(call.planned.body).hexdigest(),
                "request_body_base64": base64.b64encode(call.planned.body).decode("ascii"),
                "raw_response_base64": base64.b64encode(response.body).decode("ascii"),
                "model": decoded.model,
                "input_tokens": decoded.input_tokens,
                "output_tokens": decoded.output_tokens,
                "attempts": response.attempts,
                "elapsed_ms": response.elapsed_ms,
                "answers": _answer_evidence(call, decoded.answers),
            }
        )
        completed += 1
        if budget.halted:
            break

    if budget.stop_reason:
        reason = budget.stop_reason
    elif completed == len(validated):
        reason = "plan_prefix_complete" if len(validated) < len(prepared.calls) else "plan_complete"
    else:
        reason = "stopped"
    return ExecutionOutcome(completed, budget.requests, budget.input_tokens, reason)


def _failure_event(
    call: RunCall,
    reason: str,
    attempts: int,
    raw_response: bytes | None = None,
) -> dict[str, Any]:
    event = {
        "status": "halted",
        "reason": reason,
        "repetition": call.repetition,
        "workload": call.workload["id"],
        "strategy": call.strategy,
        "attempts": attempts,
    }
    if raw_response is not None:
        event["raw_response_base64"] = base64.b64encode(raw_response).decode("ascii")
    return event


class HTTPSingleAttemptTransport:
    """Pinned HTTPS endpoint; one HTTP exchange only, with no redirect or retry."""

    def __init__(self, api_key: str) -> None:
        if not api_key:
            raise ExecutionError("TYPESAFE_API_KEY is required for paid execution")
        if "\r" in api_key or "\n" in api_key:
            raise ExecutionError("TYPESAFE_API_KEY contains invalid header characters")
        self._api_key = api_key

    def post(self, body: bytes) -> TransportResponse:
        started = time.monotonic()
        connection = http.client.HTTPSConnection(
            API_HOST,
            timeout=HTTP_TIMEOUT_SECONDS,
            context=ssl.create_default_context(),
        )
        try:
            connection.request(
                "POST",
                API_PATH,
                body=body,
                headers={
                    "Authorization": f"Bearer {self._api_key}",
                    "Content-Type": "application/json",
                    "Accept": "application/json",
                    "Connection": "close",
                },
            )
            response = connection.getresponse()
            raw = response.read(MAX_RESPONSE_BYTES + 1)
            if len(raw) > MAX_RESPONSE_BYTES:
                raise TransportFailure("response_too_large")
            elapsed = int((time.monotonic() - started) * 1000)
            return TransportResponse(response.status, raw, elapsed, 1)
        except TransportFailure:
            raise
        except (OSError, http.client.HTTPException, ssl.SSLError):
            raise TransportFailure() from None
        finally:
            connection.close()


def _path_is_git_ignored(path: Path) -> bool:
    candidate = path / "paid-run-preflight.jsonl" if path.name == "private" else path
    try:
        relative = candidate.resolve().relative_to(REPO_ROOT.resolve())
    except ValueError:
        return False
    result = subprocess.run(
        ["git", "check-ignore", "--no-index", "--quiet", "--", relative.as_posix()],
        cwd=REPO_ROOT,
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    return result.returncode == 0


class PrivateJSONLWriter:
    """Create mode-0600 result files only inside an ignored private directory."""

    def __init__(self, directory: Path = PRIVATE_DIR) -> None:
        if directory.name != "private":
            raise ExecutionError("paid results must use the private directory")
        if not _path_is_git_ignored(directory):
            raise ExecutionError("private result directory is not git-ignored")
        try:
            directory.mkdir(mode=0o700, parents=False, exist_ok=True)
            info = directory.lstat()
        except OSError:
            raise ExecutionError("cannot create private result directory") from None
        if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode):
            raise ExecutionError("private result path must be a real directory")
        if info.st_mode & 0o077:
            raise ExecutionError("private result directory permissions must be 0700")
        directory_fd = None
        file_fd = None
        try:
            directory_fd = os.open(
                directory,
                os.O_RDONLY | getattr(os, "O_DIRECTORY", 0) | getattr(os, "O_NOFOLLOW", 0),
            )
            filename = f"paid-run-{uuid.uuid4().hex}.jsonl"
            file_fd = os.open(
                filename,
                os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0),
                0o600,
                dir_fd=directory_fd,
            )
            os.fchmod(file_fd, 0o600)
        except OSError:
            for descriptor in (file_fd, directory_fd):
                if descriptor is not None:
                    os.close(descriptor)
            raise ExecutionError("cannot create private result file") from None
        self._directory_fd = directory_fd
        self._file_fd = file_fd
        self.path = directory / filename

    def write(self, event: dict[str, Any]) -> None:
        try:
            line = (
                json.dumps(event, ensure_ascii=False, sort_keys=True, allow_nan=False)
                .encode("utf-8")
                + b"\n"
            )
            view = memoryview(line)
            while view:
                written = os.write(self._file_fd, view)
                view = view[written:]
            os.fsync(self._file_fd)
        except (OSError, TypeError, ValueError):
            raise ExecutionError("cannot write private result record") from None

    def close(self) -> None:
        for descriptor in (self._file_fd, self._directory_fd):
            try:
                os.close(descriptor)
            except OSError:
                pass


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument(
        "--dry-run", action="store_true", help="print a preflight summary; no network or files"
    )
    mode.add_argument(
        "--execute",
        action="store_true",
        help="send paid requests only with every authorization flag",
    )
    parser.add_argument(
        "--allow-shared-context",
        action="store_true",
        help="explicitly authorize synthetic records to share one request context",
    )
    parser.add_argument("--authorize-envelope-usd", default="")
    parser.add_argument("--confirm-input-price-usd-per-million", default="")
    parser.add_argument("--confirm-synthetic-corpus-sha256", default="")
    parser.add_argument("--acknowledge-billing-uncertainty", action="store_true")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    try:
        prepared = prepare_run(allow_shared_context=args.allow_shared_context)
        summary = preflight_summary(prepared)
        if args.dry_run:
            sys.stdout.write(json.dumps(summary, indent=2, sort_keys=True) + "\n")
            return 0
        required_envelope = f"{harness.MAX_CONTEXT_LIST_PRICE_ENVELOPE_USD:.5f}"
        if args.authorize_envelope_usd != required_envelope:
            raise ExecutionError(
                "paid execution requires authorization for the current full-context envelope"
            )
        required_price = f"{harness.PRICE_USD_PER_MILLION_INPUT_TOKENS:.3f}"
        if args.confirm_input_price_usd_per_million != required_price:
            raise ExecutionError(
                "paid execution requires confirmation of the current input-token price"
            )
        if args.confirm_synthetic_corpus_sha256 != prepared.fixture_sha256:
            raise ExecutionError(
                "paid execution requires confirmation of the pinned synthetic corpus hash"
            )
        verify_fixture_is_committed()
        if not args.acknowledge_billing_uncertainty:
            raise ExecutionError(
                "paid execution requires acknowledging provider billing uncertainty"
            )
        api_key = os.environ.get("TYPESAFE_API_KEY", "")
        if not api_key:
            raise ExecutionError("TYPESAFE_API_KEY is required for paid execution")
        authorized_summary = {
            **summary,
            "dry_run": False,
            "paid_execution_authorized": True,
            "authorization_required": False,
        }
        sys.stdout.write(json.dumps(authorized_summary, indent=2, sort_keys=True) + "\n")
        writer = PrivateJSONLWriter()
        try:
            outcome = execute_calls(
                prepared,
                prepared.calls,
                HTTPSingleAttemptTransport(api_key),
                writer.write,
                credential=api_key,
            )
        finally:
            writer.close()
        results_path = writer.path.relative_to(REPO_ROOT).as_posix()
        sys.stdout.write(json.dumps({
            "paid_execution_authorized": True,
            "private_results_file": results_path,
            "requests_completed": outcome.completed_requests,
            "attempts_recorded": outcome.attempted_requests,
            "reported_input_tokens": outcome.input_tokens,
            "stop_reason": outcome.stop_reason,
        }, sort_keys=True) + "\n")
        if outcome.stop_reason in {
            "plan_complete",
            "plan_prefix_complete",
            "planning_token_ceiling",
            "final_request_reserve_consumed",
            "hard_request_cap",
        }:
            return 0
        return 2
    except (harness.PlanError, ExecutionError, OSError, ValueError) as error:
        sys.stderr.write(f"map-shapes executor: {error}\n")
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
