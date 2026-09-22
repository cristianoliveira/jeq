import json
import pathlib
import tempfile
import unittest
from types import SimpleNamespace

from runner import run_agent


class FakeBrowser:
    def __init__(self):
        self.elements = {"e1": SimpleNamespace(role="button", name="Continue", options=(), checked=False, value="")}
        self.actions = []
        self.closed = 0
        self.screenshots = []

    def open(self, url):
        self.url = url

    def observe(self):
        return self.elements

    def inspect(self):
        return {"url": self.url, "title": "Complete", "body": "verified"}

    def execute(self, action):
        self.actions.append(action)

    def screenshot(self, path):
        self.screenshots.append(path)

    def close(self):
        self.closed += 1


def answer(request, desired):
    answers = {}
    for question_id, question in request["questions"].items():
        criteria = question["criteria"]
        selected = next(iter(criteria))
        if question_id == "operation":
            selected = next(key for key, description in criteria.items() if desired in description)
        answers[question_id] = {"type": "choice", "choice": selected}
    return json.dumps({"model": "fake", "answers": answers, "usage": {"input_tokens": 1, "output_tokens": 1}}).encode()


class RunnerTests(unittest.TestCase):
    def test_one_decision_and_at_most_one_action_per_cycle(self):
        browser = FakeBrowser()
        calls = []
        traces = []

        def decide(payload):
            request = json.loads(payload)
            calls.append(request)
            desired = "Click one observed" if len(calls) == 1 else "Every requirement"
            return answer(request, desired)

        with tempfile.TemporaryDirectory() as directory:
            result = run_agent(
                url="http://127.0.0.1:1/fixture.html",
                goal="continue",
                fill_values=(),
                decision_runner=decide,
                verify=lambda outcome: {"body": outcome["body"] == "verified"},
                screenshot=pathlib.Path(directory) / "final.png",
                boundary=browser,
                emit=traces.append,
            )

        self.assertEqual(len(calls), 2)
        self.assertEqual(calls[0]["state"]["visible_text"], "verified")
        self.assertNotIn("verified", "".join(traces))
        self.assertEqual(len(browser.actions), 1)
        self.assertEqual(result.decisions, 2)
        self.assertEqual(browser.closed, 1)
        self.assertEqual(len(browser.screenshots), 1)

    def test_done_fails_when_independent_verifier_fails_and_still_closes(self):
        browser = FakeBrowser()

        with self.assertRaisesRegex(RuntimeError, "independent verification"):
            run_agent(
                url="http://127.0.0.1:1/fixture.html",
                goal="continue",
                fill_values=(),
                decision_runner=lambda payload: answer(json.loads(payload), "Every requirement"),
                verify=lambda _outcome: {"required": False},
                screenshot=pathlib.Path("unused.png"),
                boundary=browser,
                emit=lambda _line: None,
            )

        self.assertEqual(browser.closed, 1)
        self.assertEqual(browser.screenshots, [])


if __name__ == "__main__":
    unittest.main()
