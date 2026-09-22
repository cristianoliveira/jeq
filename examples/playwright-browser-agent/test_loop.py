import unittest
from types import SimpleNamespace
from loop import run_loop
from policy import Policy

class Boundary:
 def __init__(self): self.calls=0; self.executed=[]
 def observe(self): return {"e1":SimpleNamespace(role="button",name="Find",options=(),checked=False)}
 def inspect(self): return {"url":"http://fixture","title":"Fixture"}
 def execute(self,a): self.executed.append(a.operation)
 def verify(self,state): return state["title"] == "Fixture"

class LoopTests(unittest.TestCase):
 def test_fake_jeq_e2e_and_request_count(self):
  boundary=Boundary(); policy=Policy("find",()); requests=[]
  def ask(request):
   requests.append(request)
   choice="op_click" if len(requests)==1 else "op_done"
   answer={"type":"choice","choice":choice}
   answers={"operation":answer}
   if choice == "op_click": answers["click_target"]={"type":"choice","choice":next(iter(request["questions"]["click_target"]["criteria"]))}
   return {"response":{"model":"fake","usage":{} ,"answers":answers},"latency_ms":3}
  trace=run_loop(boundary,policy,ask)
  self.assertEqual(len(requests),2); self.assertEqual(boundary.executed,["click"]); self.assertEqual([x.operation for x in trace],["click","DONE"])

if __name__ == "__main__": unittest.main()
