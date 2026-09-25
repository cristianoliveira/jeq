import hashlib
import json
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import harness


class MapShapeHarnessTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixtures = harness.load_fixtures()

    def workload(self, name):
        return next(item for item in self.fixtures["workloads"] if item["id"] == name)

    def test_fixtures_cover_five_requested_workload_shapes(self):
        self.assertEqual(
            [item["id"] for item in self.fixtures["workloads"]],
            [
                "short-unrelated",
                "large-unrelated",
                "multi-question",
                "exact-duplicates",
                "large-shared-reference",
            ],
        )
        self.assertEqual(len(self.fixtures["workloads"]), 5)

    def test_per_record_replay_manifest_contains_only_matching_request_hashes(self):
        manifest_path = Path(__file__).with_name("per-record-baseline-replay.json")
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        self.assertEqual(manifest["fixture_sha256"], hashlib.sha256(harness.FIXTURES.read_bytes()).hexdigest())
        self.assertEqual(manifest["model"], harness.MODEL_VERSION)
        for workload, recorded in zip(self.fixtures["workloads"], manifest["workloads"], strict=True):
            plans = harness.plan_per_record(workload, harness.MODEL_VERSION)
            expected = [
                {"record_id": record["id"], "sha256": hashlib.sha256(plan.body).hexdigest()}
                for record, plan in zip(workload["records"], plans, strict=True)
            ]
            self.assertEqual(recorded["id"], workload["id"])
            self.assertAlmostEqual(
                recorded["answers_per_1000_input_tokens"],
                recorded["attached_answers"] * 1000 / recorded["input_tokens"],
                places=6,
            )
            self.assertEqual(recorded["request_hashes_by_record"], expected)
            self.assertEqual(recorded["requests"], len(plans) * self.fixtures["repetitions"])

    def test_exact_dedup_only_collapses_byte_identical_canonical_requests(self):
        duplicates = self.workload("exact-duplicates")
        self.assertEqual(len(harness.plan_per_record(duplicates, harness.MODEL_VERSION)), 4)
        deduplicated = harness.plan_exact_dedup(duplicates, harness.MODEL_VERSION)
        self.assertEqual(len(deduplicated), 2)
        self.assertEqual(len(deduplicated[0].routes), 2)

        large = self.workload("large-unrelated")
        self.assertEqual(len(harness.plan_exact_dedup(large, harness.MODEL_VERSION)), 4)

    def test_multi_question_plan_keeps_each_question_on_each_record(self):
        workload = self.workload("multi-question")
        plans = harness.plan_per_record(workload, harness.MODEL_VERSION)
        self.assertEqual(len(plans), 4)
        self.assertTrue(all(len(plan.request["questions"]) == 3 for plan in plans))
        self.assertTrue(all(len(plan.routes) == 3 for plan in plans))
        self.assertNotIn(b"expected", b"".join(plan.body for plan in plans))

    def test_shared_context_requires_explicit_opt_in(self):
        workload = self.workload("large-shared-reference")
        with self.assertRaisesRegex(harness.PlanError, "explicit --allow-shared-context"):
            harness.plan_shared_context(workload, harness.MODEL_VERSION, allow_shared_context=False)

        plans = harness.plan_shared_context(
            workload, harness.MODEL_VERSION, allow_shared_context=True
        )
        self.assertEqual(len(plans), 1)
        self.assertEqual(len(plans[0].routes), 4)

    def test_namespaced_answers_route_to_original_records_and_input_order(self):
        workload = self.workload("multi-question")
        plan = harness.plan_shared_context(
            workload, harness.MODEL_VERSION, allow_shared_context=True
        )
        route_ids = [item["answer_id"] for item in plan[0].routes]
        self.assertEqual(len(route_ids), len(set(route_ids)))
        for item in plan[0].routes:
            question = plan[0].request["questions"][item["answer_id"]]
            self.assertIn(item["record_id"], question["instructions"])
            self.assertIn("Ignore other record entries", question["instructions"])
        fake_response = {
            item["answer_id"]: f"fake:{item['record_id']}:{item['question_id']}"
            for item in reversed(plan[0].routes)
        }

        mapped = harness.map_answers_in_input_order(workload, plan, [fake_response])
        self.assertEqual(
            [item["record_id"] for item in mapped],
            [item["id"] for item in workload["records"]],
        )
        for record, output in zip(workload["records"], mapped, strict=True):
            for question_id in workload["questions"]:
                self.assertEqual(
                    output["answers"][question_id],
                    f"fake:{record['id']}:{question_id}",
                )

    def test_unmapped_answer_id_is_rejected_not_assigned_to_another_record(self):
        workload = self.workload("multi-question")
        plan = harness.plan_shared_context(
            workload, harness.MODEL_VERSION, allow_shared_context=True
        )
        with self.assertRaisesRegex(harness.PlanError, "unmapped answer IDs"):
            harness.map_answers_in_input_order(
                workload, plan, [{"record_other__urgent": 0.9}]
            )

    def test_serialized_requests_obey_byte_ceilings_with_more_than_half_context_headroom(self):
        matrix = harness.build_matrix(self.fixtures, allow_shared_context=True)
        self.assertLessEqual(matrix["max_serialized_request_bytes"], 24 * 1024)
        self.assertLessEqual(matrix["max_state_plus_longest_question_bytes"], 12 * 1024)
        self.assertTrue(matrix["byte_limits_are_safety_guards_not_billing_estimates"])

    def test_oversized_request_fails_closed_without_truncation(self):
        state = {"payload": "x" * (harness.MAX_TOTAL_REQUEST_BYTES + 1)}
        request = harness.make_request(
            harness.MODEL_VERSION,
            state,
            {"q": {"type": "noul", "instructions": "Is it safe?"}},
        )
        original = request["state"]["payload"]
        with self.assertRaisesRegex(harness.PlanError, "serialized request"):
            harness.guard_request(request)
        self.assertEqual(request["state"]["payload"], original)

    def test_state_plus_longest_question_ceiling_is_checked_separately(self):
        request = harness.make_request(
            harness.MODEL_VERSION,
            {"payload": "x" * (harness.MAX_STATE_AND_LONGEST_QUESTION_BYTES - 500)},
            {"q": {"type": "noul", "instructions": "y" * 1000}},
        )
        self.assertLessEqual(len(harness.canonical_json(request)), harness.MAX_TOTAL_REQUEST_BYTES)
        with self.assertRaisesRegex(harness.PlanError, "state plus longest question"):
            harness.guard_request(request)

    def test_usage_and_quality_metrics_use_fake_inputs_without_claiming_provider_results(self):
        observations = [
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
        metrics = harness.calculate_usage_metrics(observations)
        self.assertEqual(metrics["attempted_requests"], 2)
        self.assertEqual(metrics["successful_responses"], 2)
        self.assertEqual(metrics["answers"], 2)
        self.assertEqual(metrics["useful_answers"], 2)
        self.assertEqual(metrics["threshold_correct_answers"], 1)
        self.assertEqual(metrics["threshold_accuracy"], 0.5)
        self.assertEqual(metrics["input_tokens"], 200)
        self.assertEqual(metrics["output_tokens"], 5)
        self.assertEqual(metrics["latency_ms_distribution"]["median"], 15)
        self.assertEqual(metrics["latency_ms_distribution"]["p95"], 20)
        self.assertEqual(metrics["useful_answers_per_1000_input_tokens"], 10.0)
        zero_token_metrics = harness.calculate_usage_metrics(
            [{
                "attempts": 1,
                "response_valid": True,
                "input_tokens": 0,
                "output_tokens": 0,
                "elapsed_ms": 5,
                "answers": [{"probability": 0.8, "threshold": 0.5, "expected": True}],
            }]
        )
        self.assertIsNone(zero_token_metrics["useful_answers_per_1000_input_tokens"])
        retry_metrics = harness.calculate_usage_metrics(
            [{
                "attempts": 2,
                "response_valid": True,
                "input_tokens": 200,
                "output_tokens": 3,
                "elapsed_ms": 5,
                "answers": [{"probability": 0.8, "threshold": 0.5, "expected": True}],
            }]
        )
        self.assertIsNone(retry_metrics["useful_answers_per_1000_input_tokens"])

        baseline = [
            {"record_id": "r1", "question_id": "q", "probability": 0.4, "threshold": 0.5, "expected": True},
            {"record_id": "r1", "question_id": "q", "probability": 0.6, "threshold": 0.5, "expected": True},
        ]
        candidate = [
            {"record_id": "r1", "question_id": "q", "probability": 0.7, "threshold": 0.5, "expected": True},
            {"record_id": "r1", "question_id": "q", "probability": 0.9, "threshold": 0.5, "expected": True},
        ]
        quality = harness.summarize_probability_runs(candidate, baseline)
        self.assertEqual(quality["candidate_mean_probability_by_answer"]["r1/q"], 0.8)
        self.assertEqual(quality["candidate_threshold_accuracy"], 1.0)
        self.assertEqual(quality["baseline_threshold_accuracy"], 0.5)
        self.assertAlmostEqual(quality["candidate_brier_score"], 0.05)
        self.assertAlmostEqual(quality["baseline_brier_score"], 0.26)
        self.assertAlmostEqual(quality["baseline_mean_within_run_probability_sd"], 0.1)
        self.assertAlmostEqual(quality["mean_probability_movement_vs_baseline"], 0.3)
        self.assertEqual(quality["threshold_decision_disagreement_vs_baseline"], 0.0)

    def test_run_matrix_and_budget_pin_model_and_bound_requests_tokens_and_retries(self):
        matrix = harness.build_matrix(self.fixtures, allow_shared_context=True)
        self.assertEqual(matrix["model"], "jev-1.13.0")
        duplicate_case = next(item for item in matrix["cases"] if item["workload"] == "exact-duplicates")
        self.assertEqual(
            duplicate_case["requests_per_repetition"]["exact-dedup"]["requests_per_repetition"],
            2,
        )
        self.assertIn(
            "maximum",
            duplicate_case["requests_per_repetition"]["shared-context"]["serialized_request_bytes"],
        )
        self.assertEqual(matrix["planned_requests_with_exact_dedup"], 129)
        self.assertEqual(matrix["maximum_requests_without_dedup_savings"], 135)
        self.assertEqual(matrix["max_retries"], 0)
        self.assertEqual(matrix["planning_token_ceiling_including_one_request_reserve"], 814_000)
        self.assertEqual(matrix["planning_spend_at_token_ceiling_usd"], 0.034188)
        self.assertEqual(
            matrix["max_context_tokens_at_request_cap_if_each_call_uses_full_context"],
            8_640_000,
        )
        self.assertEqual(matrix["max_context_list_price_envelope_usd"], 0.36288)
        self.assertTrue(matrix["reported_usage_does_not_guarantee_provider_billing"])
        self.assertTrue(matrix["halts_after_missing_usage_invalid_response_wrong_version_or_retry"])

        request = harness.make_request(
            harness.MODEL_VERSION, {"message": "synthetic"},
            {"q": {"type": "noul", "instructions": "Is this synthetic?"}},
        )
        budget = harness.PaidRunBudget()
        with self.assertRaisesRegex(harness.PlanError, "must pin jev-1.13.0"):
            budget.begin("jev-latest", request)
        with self.assertRaisesRegex(harness.PlanError, "must set max_retries=0"):
            budget.begin(harness.MODEL_VERSION, request, max_retries=1)
        budget.begin(harness.MODEL_VERSION, request)
        budget.finish(12_000, resolved_model_version=harness.MODEL_VERSION)
        self.assertEqual(budget.input_tokens, 12_000)
        budget.begin(harness.MODEL_VERSION, request)
        budget.finish(None)
        self.assertEqual(budget.input_tokens, 76_000)
        self.assertTrue(budget.halted)
        with self.assertRaisesRegex(harness.PlanError, "halted"):
            budget.begin(harness.MODEL_VERSION, request)

        wrong_model = harness.PaidRunBudget()
        wrong_model.begin(harness.MODEL_VERSION, request)
        wrong_model.finish(1_000, resolved_model_version="jev-latest")
        self.assertTrue(wrong_model.halted)
        self.assertEqual(wrong_model.input_tokens, harness.MAX_SINGLE_REQUEST_TOKENS)

        malformed = harness.PaidRunBudget()
        malformed.begin(harness.MODEL_VERSION, request)
        malformed.finish(
            1_000,
            resolved_model_version=harness.MODEL_VERSION,
            response_valid=False,
        )
        self.assertTrue(malformed.halted)
        self.assertEqual(malformed.input_tokens, harness.MAX_SINGLE_REQUEST_TOKENS)

        retried = harness.PaidRunBudget()
        retried.begin(harness.MODEL_VERSION, request)
        retried.finish(
            1_000,
            resolved_model_version=harness.MODEL_VERSION,
            actual_attempts=2,
        )
        self.assertTrue(retried.halted)
        self.assertEqual(retried.requests, 2)
        self.assertEqual(retried.input_tokens, 2 * harness.MAX_SINGLE_REQUEST_TOKENS)

    def test_offline_check_is_explicitly_not_quality_or_cost_evidence(self):
        result = harness.offline_check(self.fixtures)
        self.assertTrue(result["offline_only"])
        self.assertTrue(result["synthetic_metrics_are_not_provider_quality_or_cost_evidence"])
        self.assertEqual(result["matrix"]["planned_requests_with_exact_dedup"], 129)


if __name__ == "__main__":
    unittest.main()
