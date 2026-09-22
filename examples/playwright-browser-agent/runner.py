"""Bounded observe, decide, execute loop for the Playwright CLI example."""
from __future__ import annotations

import json
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

from agent import Action, BrowserBoundary
from policy import Policy, ask


@dataclass(frozen=True)
class RunResult:
    outcome: dict[str, str]
    decisions: int
    input_tokens: int
    output_tokens: int
    elapsed_ms: int
    models: tuple[str, ...]
    screenshot: str
    evidence: object = None


def run_agent(
    *,
    url: str,
    goal: str,
    fill_values: tuple[str, ...],
    decision_runner: Callable[[bytes], bytes],
    verify: Callable[[dict[str, str]], dict[str, bool]],
    screenshot: Path,
    boundary: BrowserBoundary | None = None,
    max_steps: int = 8,
    timeout_seconds: float = 90,
    emit: Callable[[str], None] = print,
    on_observe: Callable[[int, dict, dict[str, str]], None] | None = None,
    finalize: Callable[[BrowserBoundary, dict[str, str]], object] | None = None,
) -> RunResult:
    browser = boundary or BrowserBoundary(budget=max_steps)
    policy = Policy(goal, fill_values, budget=max_steps)
    started = time.monotonic()
    traces: list[dict] = []

    try:
        browser.open(url)
        for step in range(1, max_steps + 1):
            if time.monotonic() - started > timeout_seconds:
                raise RuntimeError("browser-agent timeout exhausted")

            elements = browser.observe()
            observed = browser.inspect()
            if on_observe is not None:
                on_observe(step, elements, observed)
            plan = policy.plan(elements, observed["url"], observed["title"], observed["body"])
            decision = policy.consume(ask(decision_runner, plan.request), plan)
            usage = decision.usage or {}
            trace = {
                "step": step,
                "operation": decision.operation,
                "target": _target_label(elements, decision.ref),
                "model": decision.model,
                "latency_ms": decision.latency_ms,
                "input_tokens": int(usage.get("input_tokens", 0)),
                "output_tokens": int(usage.get("output_tokens", 0)),
            }
            traces.append(trace)
            emit(json.dumps(trace, separators=(",", ":")))

            if decision.operation == "DONE":
                checks = verify(observed)
                if not checks or not all(checks.values()):
                    raise RuntimeError(f"independent verification failed: {checks}")
                evidence = finalize(browser, observed) if finalize is not None else None
                screenshot.parent.mkdir(parents=True, exist_ok=True)
                browser.screenshot(str(screenshot))
                elapsed_ms = round((time.monotonic() - started) * 1000)
                result = RunResult(
                    outcome=observed,
                    decisions=len(traces),
                    input_tokens=sum(trace["input_tokens"] for trace in traces),
                    output_tokens=sum(trace["output_tokens"] for trace in traces),
                    elapsed_ms=elapsed_ms,
                    models=tuple(dict.fromkeys(trace["model"] for trace in traces)),
                    screenshot=str(screenshot),
                    evidence=evidence,
                )
                emit(json.dumps({
                    "status": "PASS",
                    "decisions": result.decisions,
                    "input_tokens": result.input_tokens,
                    "output_tokens": result.output_tokens,
                    "elapsed_ms": result.elapsed_ms,
                    "models": result.models,
                    "screenshot": result.screenshot,
                }, separators=(",", ":")))
                return result

            if decision.operation == "BLOCKED":
                raise RuntimeError("Jev selected BLOCKED")

            browser.execute(Action(decision.operation, decision.ref, decision.value))

        raise RuntimeError("browser-agent action budget exhausted")
    finally:
        browser.close()


def _target_label(elements: dict, ref: str) -> str:
    element = elements.get(ref)
    if element is None:
        return ""
    return f"{element.role}:{element.name}"[:160]
