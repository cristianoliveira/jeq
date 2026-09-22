import json
import pathlib
import tempfile
import unittest

from agent import Action, Element
from hn_demo import find_most_discussed, verify_discussion
from runner import run_agent


class FakeHackerNews:
    def __init__(self):
        self.front = {
            "e1": Element("e1", "link", "120 comments", href="item?id=small"),
            "e2": Element("e2", "link", "525 comments", href="item?id=winner"),
            "e3": Element("e3", "link", "999 comments", href="https://example.com/item?id=hostile"),
        }
        self.elements = self.front
        self.url = "https://news.ycombinator.com/front?day=2026-09-21"
        self.closed = 0
        self.actions = []

    def open(self, url):
        self.url = url

    def observe(self):
        return self.elements

    def inspect(self):
        if self.url.endswith("item?id=winner"):
            return {"url": self.url, "title": "Winning story | Hacker News", "body": "story\nuser 1 hour ago\nMain comment\nreply" * 20}
        return {"url": self.url, "title": "front | Hacker News", "body": "Story A 120 comments\nStory B 525 comments"}

    def execute(self, action: Action):
        self.actions.append(action)
        self.url = "https://news.ycombinator.com/item?id=winner"
        self.elements = {}

    def screenshot(self, _path):
        pass

    def evaluate(self, _expression):
        return json.dumps([{"user": "alice", "text": "Main comment"}])

    def close(self):
        self.closed += 1


def choice_response(request, step):
    answers = {}
    for question_id, question in request["questions"].items():
        criteria = question["criteria"]
        selected = next(iter(criteria))
        if question_id == "operation":
            needle = "Click one observed" if step == 1 else "Every requirement"
            selected = next(key for key, text in criteria.items() if needle in text)
        elif question_id == "click_target" and step == 1:
            selected = next(key for key, text in criteria.items() if "525 comments" in text)
        answers[question_id] = {"type": "choice", "choice": selected}
    return json.dumps({"model": "fake-jev", "answers": answers, "usage": {"input_tokens": 1, "output_tokens": 1}}).encode()


class HackerNewsTests(unittest.TestCase):
    def test_finds_maximum_same_host_discussion_and_ignores_external_count(self):
        expected = find_most_discussed(FakeHackerNews().front, "https://news.ycombinator.com/front?day=2026-09-21")

        self.assertEqual(expected, {
            "comments": 525,
            "url": "https://news.ycombinator.com/item?id=winner",
            "item_id": "winner",
        })

    def test_fake_jev_navigates_to_verified_maximum_and_extracts_comments(self):
        browser = FakeHackerNews()
        expected = {}
        calls = 0

        def observe(step, elements, outcome):
            if step == 1:
                expected.update(find_most_discussed(elements, outcome["url"]))

        def decide(payload):
            nonlocal calls
            calls += 1
            return choice_response(json.loads(payload), calls)

        with tempfile.TemporaryDirectory() as directory:
            result = run_agent(
                url=browser.url,
                goal="Open the discussion with the largest displayed comment count, then choose DONE.",
                fill_values=(),
                decision_runner=decide,
                verify=lambda outcome: verify_discussion(outcome, expected),
                screenshot=pathlib.Path(directory) / "hn.png",
                boundary=browser,
                max_steps=3,
                emit=lambda _line: None,
                on_observe=observe,
                finalize=lambda boundary, _outcome: json.loads(boundary.evaluate("fixed expression")),
            )

        self.assertEqual(calls, 2)
        self.assertEqual(len(browser.actions), 1)
        self.assertEqual(browser.actions[0].ref, "e2")
        self.assertEqual(result.evidence[0]["text"], "Main comment")
        self.assertEqual(browser.closed, 1)

    def test_verifier_rejects_wrong_item_and_external_host(self):
        expected = {"item_id": "winner"}
        body = "comment\nreply" * 30

        self.assertFalse(verify_discussion({"url": "https://news.ycombinator.com/item?id=wrong", "title": "x | Hacker News", "body": body}, expected)["item"])
        self.assertFalse(verify_discussion({"url": "https://example.com/item?id=winner", "title": "x | Hacker News", "body": body}, expected)["host"])


if __name__ == "__main__":
    unittest.main()
