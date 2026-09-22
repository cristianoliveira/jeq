"""Bounded observe, decide, execute loop; browser and ask runners are injected."""
from __future__ import annotations
from dataclasses import dataclass
import time
from policy import AskResult, Policy

@dataclass(frozen=True)
class Trace:
    step: int; operation: str; target: str; model: str; usage: object; latency_ms: int

def run_loop(boundary, policy: Policy, ask_runner, goal: str = "", timeout: float = 30.0) -> list[Trace]:
    traces = []; started = time.monotonic()
    try:
      for step in range(policy.budget):
        if time.monotonic() - started > timeout: raise RuntimeError("loop timeout")
        elements = boundary.observe()
        plan = policy.plan(elements, boundary.inspect()["url"], boundary.inspect()["title"])
        result = ask_runner(plan.request)
        decision = policy.consume(AskResult(result["response"], result.get("latency_ms", 0)), plan)
        traces.append(Trace(step + 1, decision.operation, decision.ref, decision.model, decision.usage, decision.latency_ms))
        if decision.operation == "DONE":
            verified = boundary.inspect()
            if not boundary.verify(verified): raise RuntimeError("independent verification failed")
            return traces
        if decision.operation == "BLOCKED": raise RuntimeError("policy blocked task")
        boundary.execute(type("Action", (), {"operation": decision.operation, "ref": decision.ref, "value": decision.value})())
      raise RuntimeError("action budget exhausted")
    finally:
      close = getattr(boundary, "close", None)
      if close: close()
