import unittest
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

    def test_parses_allowlisted_elements_and_options(self):
        self.assertEqual(self.elements["e16"].role, "searchbox")
        self.assertEqual(self.elements["e20"].options, ("All stays", "Design"))
        self.assertEqual(allow(Action("fill", "e16", "Lisbon"), self.elements), ["fill", "e16", "Lisbon"])
        self.assertEqual(allow(Action("select", "e20", "Design"), self.elements), ["select", "e20", "Design"])

    def test_rejects_stale_unknown_and_unsafe_actions(self):
        with self.assertRaises(ValueError): allow(Action("click", "e999"), self.elements)
        with self.assertRaises(ValueError): allow(Action("eval", "e17", "document.cookie"), self.elements)
        with self.assertRaises(ValueError): allow(Action("select", "e20", "Unknown"), self.elements)
        with self.assertRaises(ValueError): allow(Action("fill", "e16", "x" * 513), self.elements)

if __name__ == "__main__": unittest.main()
