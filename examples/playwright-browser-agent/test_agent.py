import unittest
from unittest.mock import patch
import sys
from agent import Action, Element, allow, parse_snapshot

SNAPSHOT = '''- searchbox "Destination" [ref=e16]
- button "Find stays" [ref=e17] [cursor=pointer]
- combobox "Stay category" [ref=e20] [cursor=pointer]:
  - option "All stays" [selected]
  - option "Design"
- checkbox "Free cancellation" [ref=e22] [cursor=pointer]'''

class BoundaryTests(unittest.TestCase):
    def setUp(self):
        self.elements = parse_snapshot(SNAPSHOT)

    def test_parses_allowlisted_elements_options_and_current_state(self):
        self.assertEqual(self.elements["e16"].role, "searchbox")
        self.assertEqual(self.elements["e20"].options, ("All stays", "Design"))
        self.assertEqual(self.elements["e20"].value, "All stays")
        current = parse_snapshot('- searchbox "Destination" [active] [ref=e1]: Lisbon\n- checkbox "Free" [checked] [ref=e2]')
        self.assertEqual(current["e1"].value, "Lisbon")
        self.assertTrue(current["e2"].checked)
        self.assertEqual(allow(Action("fill", "e16", "Lisbon"), self.elements), ["fill", "e16", "Lisbon"])
        self.assertEqual(allow(Action("select", "e20", "Design"), self.elements), ["select", "e20", "Design"])
        self.assertEqual(allow(Action("wait", value="2"), self.elements), ["wait", "2"])
        self.assertEqual(allow(Action("scroll", value="500"), self.elements), ["mousewheel", "0", "500"])

    def test_rejects_stale_unknown_and_unsafe_actions(self):
        with self.assertRaises(ValueError): allow(Action("click", "e999"), self.elements)
        from agent import BrowserBoundary
        boundary = BrowserBoundary()
        for url in ("http://127.0.0.1:1@evil.example/", "http://evil.example/", "https://127.0.0.1/"):
            with self.assertRaises(ValueError): boundary.open(url)

    def test_main_closes_after_failure(self):
        import agent
        class FakeBoundary:
            closed = 0
            def open(self, _url): pass
            def observe(self): self.elements = {}
            def inspect(self): raise RuntimeError("synthetic failure")
            def close(self): FakeBoundary.closed += 1
        with patch.object(agent, "BrowserBoundary", FakeBoundary), patch.object(sys, "argv", ["agent.py", "http://127.0.0.1:1234/"]):
            self.assertEqual(agent.main(), 1)
        self.assertEqual(FakeBoundary.closed, 1)

    def test_subprocess_failure_is_evidence_and_close_is_safe(self):
        from agent import BrowserBoundary
        boundary = BrowserBoundary(executable="/usr/bin/false")
        with self.assertRaises(RuntimeError): boundary.run("snapshot")
        boundary.close()

    def test_budget_and_wait_are_bounded(self):
        from agent import BrowserBoundary
        boundary = BrowserBoundary(budget=1)
        boundary.elements = self.elements
        boundary.run = lambda *args: "ok"
        with patch("agent.time.sleep") as sleep:
            self.assertEqual(boundary.execute(Action("wait", value="9")), "waited")
            sleep.assert_called_once_with(5.0)
        with self.assertRaises(RuntimeError): boundary.execute(Action("click", "e17"))
        with self.assertRaises(ValueError): allow(Action("eval", "e17", "document.cookie"), self.elements)
        with self.assertRaises(ValueError): allow(Action("select", "e20", "Unknown"), self.elements)
        with self.assertRaises(ValueError): allow(Action("fill", "e16", "x" * 513), self.elements)

if __name__ == "__main__": unittest.main()
