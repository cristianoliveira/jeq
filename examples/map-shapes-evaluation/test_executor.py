import base64
import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import executor
import harness


class FakeTransport:
    def __init__(self, responses=None, *, input_tokens=128):
        self.responses = list(responses or [])
        self.input_tokens = input_tokens
        self.bodies = []

    def post(self, body):
        self.bodies.append(body)
        if self.responses:
            return self.responses.pop(0)
        request = json.loads(body)
        response = {
            "model": harness.MODEL_VERSION,
            "answers": {
                question_id: {"type": "noul", "noul": 0.75}
                for question_id in request["questions"]
            },
            "usage": {"input_tokens": self.input_tokens, "output_tokens": 2},
        }
        return executor.TransportResponse(200, json.dumps(response).encode(), 3, 1)


class MapShapeExecutorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.prepared = executor.prepare_run(allow_shared_context=True)

    def test_preflight_is_pinned_synthetic_and_has_required_cost_envelopes(self):
        summary = executor.preflight_summary(self.prepared)
        self.assertTrue(summary["dry_run"])
        self.assertTrue(summary["synthetic_only"])
        self.assertFalse(summary["paid_execution_authorized"])
        self.assertEqual(summary["model"], "jev-1.13.0")
        self.assertEqual(summary["planned_requests"], 129)
        self.assertEqual(summary["hard_attempt_cap"], 135)
        self.assertEqual(summary["planning_token_ceiling"], 814_000)
        self.assertEqual(summary["planning_spend_estimate_usd"], 0.034188)
        self.assertEqual(summary["input_price_usd_per_million_tokens"], 0.042)
        self.assertEqual(summary["full_context_list_price_envelope_usd"], 0.36288)
        self.assertTrue(summary["provider_billing_is_not_guaranteed"])
        self.assertTrue(summary["all_planned_requests_pass_byte_guards"])
        self.assertTrue(summary["shared_context_opted_in"])
        self.assertTrue(all(b"expected" not in call.planned.body for call in self.prepared.calls))

    def test_committed_preflight_snapshot_matches_dry_run(self):
        snapshot = json.loads((executor.ROOT / "preflight-authorization.json").read_text())
        self.assertEqual(snapshot, executor.preflight_summary(self.prepared))

    def test_preflight_requires_explicit_shared_context_opt_in(self):
        with self.assertRaisesRegex(harness.PlanError, "allow-shared-context"):
            executor.prepare_run(allow_shared_context=False)

    def test_paid_execution_requires_the_pinned_corpus_at_head(self):
        result = Mock(returncode=0, stdout=self.prepared.fixture_bytes)
        with patch.object(executor.subprocess, "run", return_value=result) as run:
            executor.verify_fixture_is_committed()
        self.assertEqual(run.call_args.args[0][0:2], ["git", "show"])

        result.returncode = 1
        with patch.object(executor.subprocess, "run", return_value=result):
            with self.assertRaisesRegex(executor.ExecutionError, "must be committed"):
                executor.verify_fixture_is_committed()

    def test_pinned_corpus_rejects_modified_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            modified = Path(directory) / "fixtures.json"
            modified.write_bytes(executor.FIXTURES.read_bytes() + b"\n")
            with patch.object(executor, "FIXTURES", modified):
                with self.assertRaisesRegex(harness.PlanError, "pinned synthetic corpus"):
                    executor.prepare_run(allow_shared_context=True)

    def test_dry_run_prints_summary_without_credentials_network_or_results(self):
        with patch.object(
            executor,
            "HTTPSingleAttemptTransport",
            side_effect=AssertionError("network"),
        ):
            with patch.object(
                executor,
                "PrivateJSONLWriter",
                side_effect=AssertionError("results"),
            ):
                output = io.StringIO()
                with contextlib.redirect_stdout(output):
                    status = executor.main(["--dry-run", "--allow-shared-context"])
        summary = json.loads(output.getvalue())
        self.assertEqual(status, 0)
        self.assertEqual(summary["planned_requests"], 129)
        self.assertEqual(summary["paid_calls_made"], 0)
        self.assertFalse(summary["paid_execution_authorized"])

    def test_executor_sends_calls_sequentially_and_writes_results_only_to_sink(self):
        calls = self.prepared.calls[:3]
        transport = FakeTransport()
        results = []
        outcome = executor.execute_calls(self.prepared, calls, transport, results.append)
        self.assertEqual(len(transport.bodies), 3)
        self.assertEqual(outcome.completed_requests, 3)
        self.assertEqual(outcome.attempted_requests, 3)
        self.assertEqual(outcome.input_tokens, 3 * 128)
        self.assertEqual(outcome.stop_reason, "plan_prefix_complete")
        self.assertEqual(len(results), 3)
        self.assertTrue(all(item["status"] == "ok" for item in results))
        self.assertTrue(all(item["model"] == harness.MODEL_VERSION for item in results))

    def test_all_request_byte_guards_run_before_first_transport_call(self):
        transport = FakeTransport()
        with patch.object(harness, "MAX_TOTAL_REQUEST_BYTES", 100):
            with self.assertRaisesRegex(harness.PlanError, "serialized request"):
                executor.execute_calls(
                    self.prepared,
                    self.prepared.calls[:1],
                    transport,
                    lambda _: None,
                )
        self.assertEqual(transport.bodies, [])

    def test_missing_usage_halts_before_any_following_request(self):
        missing_usage = executor.TransportResponse(
            200,
            json.dumps({
                "model": harness.MODEL_VERSION,
                "answers": {"risk": {"type": "noul", "noul": 0.9}},
            }).encode(),
            2,
            1,
        )
        transport = FakeTransport([missing_usage])
        results = []
        outcome = executor.execute_calls(
            self.prepared, self.prepared.calls[:3], transport, results.append
        )
        self.assertEqual(len(transport.bodies), 1)
        self.assertEqual(outcome.attempted_requests, 1)
        self.assertEqual(outcome.input_tokens, harness.MAX_SINGLE_REQUEST_TOKENS)
        self.assertEqual(outcome.stop_reason, "malformed_response")
        self.assertEqual(results[0]["status"], "halted")
        raw_response = base64.b64decode(results[0]["raw_response_base64"])
        self.assertEqual(json.loads(raw_response)["model"], harness.MODEL_VERSION)

    def test_wrong_model_and_malformed_response_halt_before_next_call(self):
        wrong_model = executor.TransportResponse(
            200,
            json.dumps({
                "model": "jev-latest",
                "answers": {"risk": {"type": "noul", "noul": 0.9}},
                "usage": {"input_tokens": 20, "output_tokens": 1},
            }).encode(),
            2,
            1,
        )
        malformed = executor.TransportResponse(200, b"not json", 2, 1)
        wrong_answer_ids = executor.TransportResponse(
            200,
            json.dumps({
                "model": harness.MODEL_VERSION,
                "answers": {"unexpected": {"type": "noul", "noul": 0.9}},
                "usage": {"input_tokens": 20, "output_tokens": 1},
            }).encode(),
            2,
            1,
        )
        for response, expected_reason in (
            (wrong_model, "model_mismatch"),
            (malformed, "malformed_response"),
            (wrong_answer_ids, "malformed_response"),
        ):
            with self.subTest(reason=expected_reason):
                transport = FakeTransport([response])
                result = executor.execute_calls(
                    self.prepared, self.prepared.calls[:2], transport, lambda _: None
                )
                self.assertEqual(len(transport.bodies), 1)
                self.assertEqual(result.stop_reason, expected_reason)

    def test_unexpected_retry_is_charged_per_attempt_and_stops(self):
        retried = executor.TransportResponse(
            200,
            json.dumps({
                "model": harness.MODEL_VERSION,
                "answers": {"risk": {"type": "noul", "noul": 0.9}},
                "usage": {"input_tokens": 20, "output_tokens": 1},
            }).encode(),
            2,
            2,
        )
        transport = FakeTransport([retried])
        result = executor.execute_calls(
            self.prepared, self.prepared.calls[:2], transport, lambda _: None
        )
        self.assertEqual(len(transport.bodies), 1)
        self.assertEqual(result.attempted_requests, 2)
        self.assertEqual(result.input_tokens, 2 * harness.MAX_SINGLE_REQUEST_TOKENS)
        self.assertEqual(result.stop_reason, "unexpected_retry")

    def test_budget_enforces_the_hard_135_attempt_cap(self):
        budget = harness.PaidRunBudget()
        request = self.prepared.calls[0].planned.request
        for _ in range(harness.MAX_REQUESTS):
            budget.begin(harness.MODEL_VERSION, request)
            budget.finish(0, resolved_model_version=harness.MODEL_VERSION)
        with self.assertRaisesRegex(harness.PlanError, "135 request cap"):
            budget.begin(harness.MODEL_VERSION, request)
        self.assertEqual(budget.requests, 135)
        self.assertEqual(budget.stop_reason, "hard_request_cap")

    def test_planning_threshold_allows_only_one_final_reserved_request(self):
        budget = harness.PaidRunBudget()
        request = self.prepared.calls[0].planned.request
        for _ in range(11):
            budget.begin(harness.MODEL_VERSION, request)
            budget.finish(64_000, resolved_model_version=harness.MODEL_VERSION)
        self.assertEqual(budget.input_tokens, 704_000)
        budget.begin(harness.MODEL_VERSION, request)
        budget.finish(46_000, resolved_model_version=harness.MODEL_VERSION)
        self.assertEqual(budget.input_tokens, 750_000)
        budget.begin(harness.MODEL_VERSION, request)
        budget.finish(64_000, resolved_model_version=harness.MODEL_VERSION)
        self.assertEqual(budget.input_tokens, 814_000)
        self.assertTrue(budget.halted)
        self.assertEqual(budget.stop_reason, "final_request_reserve_consumed")
        with self.assertRaisesRegex(harness.PlanError, "halted"):
            budget.begin(harness.MODEL_VERSION, request)

    def test_executor_stops_at_token_ceiling_before_sending_an_unreserved_request(self):
        transport = FakeTransport(input_tokens=64_000)
        outcome = executor.execute_calls(
            self.prepared, self.prepared.calls, transport, lambda _: None
        )
        self.assertEqual(len(transport.bodies), 12)
        self.assertEqual(outcome.attempted_requests, 12)
        self.assertEqual(outcome.input_tokens, 768_000)
        self.assertEqual(outcome.stop_reason, "planning_token_ceiling")

    def test_execute_cli_requires_all_authorization_flags_before_io(self):
        with patch.object(
            executor,
            "HTTPSingleAttemptTransport",
            side_effect=AssertionError("network"),
        ):
            with patch.object(
                executor,
                "PrivateJSONLWriter",
                side_effect=AssertionError("results"),
            ):
                errors = io.StringIO()
                with contextlib.redirect_stderr(errors):
                    status = executor.main(["--execute", "--allow-shared-context"])
        self.assertEqual(status, 2)
        self.assertIn("current full-context envelope", errors.getvalue())

    def test_paid_result_directory_is_ignored_by_git(self):
        self.assertTrue(executor._path_is_git_ignored(executor.PRIVATE_DIR))

    def test_response_echoing_credential_is_not_saved(self):
        response = executor.TransportResponse(
            200,
            json.dumps({
                "model": harness.MODEL_VERSION,
                "answers": {"risk": {"type": "noul", "noul": 0.9}},
                "usage": {"input_tokens": 20, "output_tokens": 1},
                "diagnostic": "secret-test-key",
            }).encode(),
            2,
            1,
        )
        results = []
        outcome = executor.execute_calls(
            self.prepared,
            self.prepared.calls[:2],
            FakeTransport([response]),
            results.append,
            credential="secret-test-key",
        )
        self.assertEqual(outcome.stop_reason, "credential_echo_rejected")
        self.assertNotIn("secret-test-key", json.dumps(results))

    def test_transport_rejects_header_injection_credentials(self):
        with self.assertRaisesRegex(executor.ExecutionError, "invalid header characters"):
            executor.HTTPSingleAttemptTransport("key\r\nAuthorization: forged")

    def test_private_result_writer_creates_restricted_jsonl(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "private"
            with patch.object(executor, "_path_is_git_ignored", return_value=True):
                writer = executor.PrivateJSONLWriter(target)
            writer.write({"status": "ok", "request_id": "synthetic-only"})
            result_path = writer.path
            writer.close()
            self.assertTrue(result_path.is_file())
            self.assertEqual(result_path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(target.stat().st_mode & 0o077, 0)
            self.assertEqual(json.loads(result_path.read_text()), {
                "request_id": "synthetic-only",
                "status": "ok",
            })

    def test_raw_response_private_record_is_encoded_without_stdout(self):
        call = self.prepared.calls[0]
        transport = FakeTransport()
        results = []
        result = executor.execute_calls(
            self.prepared, [call], transport, results.append
        )
        self.assertEqual(result.completed_requests, 1)
        raw_response = base64.b64decode(results[0]["raw_response_base64"])
        self.assertEqual(json.loads(raw_response)["model"], harness.MODEL_VERSION)
        # The preflight summary never leaks raw request or answer data.
        summary = executor.preflight_summary(self.prepared)
        self.assertNotIn("answers", summary)
        self.assertNotIn("request_body_base64", summary)


if __name__ == "__main__":
    unittest.main()
