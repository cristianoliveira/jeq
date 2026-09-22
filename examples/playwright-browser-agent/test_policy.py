import unittest
from types import SimpleNamespace
from policy import Policy, ask

class PolicyTests(unittest.TestCase):
 def setUp(self): self.elements={"e1":SimpleNamespace(role="button",name="Find",options=()),"e2":SimpleNamespace(role="searchbox",name="Destination",options=())}; self.p=Policy("find a stay",("Lisbon","Oslo"))
 def test_request_has_independent_heads_and_bounds(self):
  r=self.p.request(self.elements,"http://x","title"); self.assertIn("operation",r["questions"]); self.assertIn("click",r["questions"]); self.assertIn("fill_value",r["questions"]); self.assertLessEqual(len(r["state"]["goal"]),512)
 def test_matching_target_and_finite_value(self):
  r=self.p.request(self.elements,"u","t"); target=next(k for k,v in r["_policy_candidates"].items() if v["operation"]=="fill"); d=self.p.consume({"model":"m","answers":{"operation":{"choice":"fill"},"fill":{"choice":target},"fill_value":{"choice":"value_0"}}},r); self.assertEqual((d.operation,d.ref,d.value),("fill","e2","Lisbon"))
 def test_rejects_stale_and_unused_invalid_head(self):
  r=self.p.request(self.elements,"u","t")
  with self.assertRaises(ValueError): self.p.consume({"answers":{"operation":{"choice":"click"},"click":{"choice":"target_999"},"fill_value":{"choice":"bad"}}},r)
 def test_injected_runner(self): self.assertEqual(ask(lambda payload:b'{"answers":{"operation":{"choice":"DONE"}}}',self.p.request(self.elements,"u","t")).get("answers")["operation"]["choice"],"DONE")
if __name__ == "__main__": unittest.main()
