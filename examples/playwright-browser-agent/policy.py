"""Offline finite policy for one bounded Jev browser decision."""
from __future__ import annotations
import json, subprocess, time
from dataclasses import dataclass

@dataclass(frozen=True)
class Candidate:
    ident: str; operation: str; ref: str = ""; value: str = ""; role: str = ""; name: str = ""
@dataclass(frozen=True)
class Decision:
    operation: str; ref: str = ""; value: str = ""; model: str = ""; usage: object = None; latency_ms: int = 0

class Policy:
    def __init__(self, goal: str, fill_values: tuple[str, ...], budget: int = 6):
        self.goal, self.fill_values, self.budget, self.steps = goal[:512], tuple(v[:512] for v in fill_values[:16]), budget, 0
        self.history: list[str] = []; self._candidates = {}
    def request(self, elements: dict, url: str, title: str) -> dict:
        if self.steps >= self.budget: raise ValueError("action budget exhausted")
        candidates: dict[str, Candidate] = {}
        operations = {"wait": "Wait briefly", "scroll": "Scroll down", "DONE": "Task is complete", "BLOCKED": "Task cannot continue"}
        for ident, operation in (("op_wait", "wait"), ("op_scroll", "scroll"), ("op_done", "DONE"), ("op_blocked", "BLOCKED")): candidates[ident] = Candidate(ident, operation)
        heads = {"operation": {x.ident: x.operation for x in candidates.values()}}
        for ident, element in list(elements.items())[:32]:
            for operation in ("click", "fill", "select", "check", "uncheck"):
                if operation == "click" and element.role not in {"button", "link"}: continue
                if operation == "fill" and element.role not in {"searchbox", "textbox"}: continue
                if operation == "select" and element.role != "combobox": continue
                if operation in {"check", "uncheck"} and element.role != "checkbox": continue
                cid = f"target_{len(candidates)}"; candidates[cid] = Candidate(cid, operation, ident, role=element.role, name=element.name)
                if operation == "select":
                    for option in tuple(element.options)[:16]:
                        oid = f"{cid}_{len(candidates)}"; candidates[oid] = Candidate(oid, operation, ident, value=str(option)[:512], role=element.role, name=element.name)
                heads.setdefault(operation, {})[cid] = f"{element.role} {element.name} ref {ident}; assumed operation: {operation}"
        for cid, candidate in candidates.items():
            heads["operation"][cid] = candidate.operation
            if candidate.operation in {"click", "fill", "select", "check", "uncheck"}:
                heads.setdefault(candidate.operation, {})[cid] = f"{candidate.role} {candidate.name} ref {candidate.ref} value {candidate.value}; assumed operation: {candidate.operation}"
        if any(c.operation == "fill" for c in candidates.values()): heads["fill_value"] = {f"value_{i}": value for i, value in enumerate(self.fill_values)}
        state = {"goal": self.goal, "url": url[:512], "title": title[:256], "elements": {k: {"role": v.role, "name": v.name, "options": [str(x)[:512] for x in tuple(v.options)[:16]]} for k,v in list(elements.items())[:32]}, "recent_actions": self.history[-8:]}
        questions = {"operation": {"type":"choice", "instructions":"Choose the next operation.", "criteria":heads["operation"]}}
        for operation, criteria in heads.items():
            if operation != "operation": questions[operation] = {"type":"choice", "instructions":f"Choose a target assuming the operation is {operation}.", "criteria":criteria}
        self._candidates = {k:v.__dict__ for k,v in candidates.items()}
        return {"model":"jev-latest", "state":state, "questions":questions}
    def consume(self, response: dict, request: dict) -> Decision:
        answers = response.get("answers", response.get("_jeq", {}).get("answers"))
        if not isinstance(answers, dict) or not isinstance(answers.get("operation"), dict): raise ValueError("malformed decision envelope")
        operation = answers["operation"].get("choice");         candidates = self._candidates
        op = next((c for c in candidates.values() if c["operation"] == operation), None)
        if not op: raise ValueError("unknown operation")
        if operation in {"wait", "scroll", "DONE", "BLOCKED"}:
            self.steps += 1; self.history.append(operation)
            return Decision(operation, model=str(response.get("model", "")), usage=response.get("usage"), latency_ms=int(response.get("_decision_latency_ms", 0)))
        head = answers.get(operation, {}); selected = head.get("choice") if isinstance(head, dict) else None
        target = candidates.get(selected or "")
        if not target or target["operation"] != operation or (operation == "select" and not target.get("value")): raise ValueError("stale or incompatible target")
        value = ""
        if operation == "fill":
            value_id = answers.get("fill_value", {}).get("choice") if isinstance(answers.get("fill_value"), dict) else None
            if value_id not in request["questions"]["fill_value"]["criteria"]: raise ValueError("unknown fill value")
            value = request["questions"]["fill_value"]["criteria"][value_id]
        self.steps += 1; self.history.append(operation)
        return Decision(operation, target["ref"], value or target.get("value", ""), str(response.get("model", "")), response.get("usage"), int(response.get("_decision_latency_ms", 0)))

def ask(runner, request: dict) -> dict:
    payload = json.dumps(request).encode()
    started = time.monotonic(); raw = runner(payload); response = json.loads(raw); response["_decision_latency_ms"] = round((time.monotonic() - started) * 1000); return response

def subprocess_runner(binary="jeq"):
    def run(payload):
        result = subprocess.run([binary, "ask", "--request", "-"], input=payload, capture_output=True, timeout=15, check=True)
        return result.stdout
    return run
