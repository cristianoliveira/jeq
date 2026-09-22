import json
import unittest
from types import SimpleNamespace
from unittest.mock import patch

from agent import parse_snapshot
from policy import AskResult, Policy, ask, subprocess_runner


def element(role, name, options=(), checked=False):
    return SimpleNamespace(role=role, name=name, options=options, checked=checked)


def choice(candidate_id):
    return {"type": "choice", "choice": candidate_id}


def response(answers):
    return AskResult({"model": "jev-test", "answers": answers, "usage": {"input_tokens": 3, "output_tokens": 1}}, 17)


class PolicyTests(unittest.TestCase):
    def setUp(self):
        self.elements = {
            "e1": element("button", "Find"),
            "e2": element("searchbox", "Destination"),
            "e3": element("combobox", "Category", ("All", "Design")),
            "e4": element("checkbox", "Free cancellation"),
            "e5": element("checkbox", "Breakfast", checked=True),
        }
        self.policy = Policy("find a design stay", ("Lisbon", "Oslo"))

    def test_request_has_real_operations_independent_heads_and_no_private_map(self):
        plan = self.policy.plan(self.elements, "http://127.0.0.1:1/", "Stays")

        offered = set(plan.operations.values())
        self.assertTrue({"click", "fill", "select", "check", "uncheck", "wait", "DONE"} <= offered)
        self.assertIn("select_target", plan.request["questions"])
        self.assertIn("Assume the chosen operation is select", plan.request["questions"]["select_target"]["instructions"])
        self.assertNotIn("_policy_candidates", plan.request)
        self.assertNotIn("fill_values", plan.request)

    def test_select_target_carries_observed_option_value(self):
        plan = self.policy.plan(self.elements, "u", "t")
        operation_id = next(key for key, value in plan.operations.items() if value == "select")
        target_id = next(key for key, value in plan.targets["select"].items() if value.value == "Design")

        decision = self.policy.consume(
            response({"operation": choice(operation_id), "select_target": choice(target_id)}),
            plan,
        )

        self.assertEqual((decision.operation, decision.ref, decision.value), ("select", "e3", "Design"))

    def test_matching_fill_heads_ignore_invalid_unused_speculation(self):
        plan = self.policy.plan(self.elements, "u", "t")
        operation_id = next(key for key, value in plan.operations.items() if value == "fill")
        target_id = next(iter(plan.targets["fill"]))
        answers = {
            "operation": choice(operation_id),
            "fill_target": choice(target_id),
            "fill_value": choice("value_0"),
            "click_target": {"type": "choice", "choice": "invented"},
        }

        decision = self.policy.consume(response(answers), plan)

        self.assertEqual((decision.ref, decision.value), ("e2", "Lisbon"))
        self.assertEqual((decision.model, decision.usage, decision.latency_ms), ("jev-test", {"input_tokens": 3, "output_tokens": 1}, 17))

    def test_checkbox_state_offers_only_compatible_operation(self):
        plan = self.policy.plan(self.elements, "u", "t")

        self.assertEqual({candidate.ref for candidate in plan.targets["check"].values()}, {"e4"})
        self.assertEqual({candidate.ref for candidate in plan.targets["uncheck"].values()}, {"e5"})
        parsed = parse_snapshot('- checkbox "Free cancellation" [checked] [ref=e9]')
        self.assertTrue(parsed["e9"].checked)

    def test_rejects_malformed_unknown_stale_and_unknown_fill_value(self):
        plan = self.policy.plan(self.elements, "u", "t")
        click_operation = next(key for key, value in plan.operations.items() if value == "click")
        fill_operation = next(key for key, value in plan.operations.items() if value == "fill")
        fill_target = next(iter(plan.targets["fill"]))
        cases = [
            AskResult({}, 0),
            response({"operation": choice("unknown")}),
            response({"operation": choice(click_operation), "click_target": choice("stale")}),
            response({"operation": choice(fill_operation), "fill_target": choice(fill_target), "fill_value": choice("unknown")}),
            response({"operation": {"choice": click_operation}}),
        ]

        for result in cases:
            with self.subTest(result=result):
                with self.assertRaises(ValueError):
                    self.policy.consume(result, plan)

    def test_state_history_elements_options_and_values_are_bounded(self):
        values = tuple(f"v{i}" for i in range(20))
        policy = Policy("g" * 700, values)
        policy.history = [str(index) for index in range(20)]
        many = {
            f"e{index}": element("combobox", "n" * 400, tuple(f"o{option}" for option in range(40)))
            for index in range(80)
        }

        plan = policy.plan(many, "u" * 700, "t" * 400, "body " * 2_000)
        state = plan.request["state"]

        self.assertEqual(len(state["elements"]), 64)
        self.assertEqual(len(state["elements"][0]["options"]), 32)
        self.assertEqual(len(plan.targets["select"]), 64)
        self.assertEqual(len(state["recent_actions"]), 8)
        self.assertEqual(len(state["goal"]), 512)
        self.assertEqual(len(state["visible_text"]), 4_000)
        self.assertEqual(len(policy.fill_values), 16)
        with self.assertRaises(ValueError):
            Policy("g", ("x" * 513,))

    def test_controls_consume_budget(self):
        policy = Policy("g", (), budget=1)
        plan = policy.plan({}, "u", "t")
        wait_id = next(key for key, value in plan.operations.items() if value == "wait")

        decision = policy.consume(response({"operation": choice(wait_id)}), plan)

        self.assertEqual(decision.operation, "wait")
        with self.assertRaisesRegex(ValueError, "budget"):
            policy.plan({}, "u", "t")

    def test_scroll_controls_map_to_executable_bounded_actions(self):
        plan = self.policy.plan({}, "u", "t")
        down_id = next(key for key, value in plan.operations.items() if value == "scroll_down")

        decision = self.policy.consume(response({"operation": choice(down_id)}), plan)

        self.assertEqual((decision.operation, decision.value), ("scroll", "400"))

    def test_ask_measures_latency_and_rejects_bad_json(self):
        with patch("policy.time.perf_counter", side_effect=[10.0, 10.025]):
            result = ask(lambda payload: json.dumps({"sent": json.loads(payload)}).encode(), {"state": "x"})
        self.assertEqual(result.latency_ms, 25)
        self.assertEqual(result.response["sent"], {"state": "x"})
        with self.assertRaisesRegex(ValueError, "malformed JSON"):
            ask(lambda _payload: b"not json", {})

    @patch("policy.subprocess.run")
    def test_subprocess_adapter_uses_native_stdin_request(self, run):
        run.return_value.stdout = b"{}"

        subprocess_runner("jeq-test")(b"{}")

        args, kwargs = run.call_args
        self.assertEqual(args[0], ["jeq-test", "ask", "--request", "-"])
        self.assertEqual(kwargs["input"], b"{}")
        self.assertTrue(kwargs["check"])


if __name__ == "__main__":
    unittest.main()
