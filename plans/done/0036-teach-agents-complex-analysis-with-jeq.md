---
id: TASK-0036
title: Teach agents complex analysis with JEQ
status: done
depends_on: []
priority: high
tags: []
---

# Teach agents complex analysis with JEQ

## Problem
Agents can invoke JEQ, but they lack durable guidance for deciding when judgment is appropriate, decomposing complex analysis, controlling privacy and request budgets, and interpreting uncertainty without turning model evidence into hidden policy.

## Desired outcome
A Pi agent can turn a complex semantic-analysis request into a bounded, auditable JEQ workflow while keeping deterministic work, policy, privacy, and actions outside the model.

## Acceptance criteria
- [x] Install a global `jeq-complex-analysis` skill with precise positive and negative triggers and explicit routing to `typesafe-ai`, `code-review`, and `arch-review` where appropriate.
- [x] Teach analysis-contract definition, state minimization, typed question design, ask/map/reduce/gate selection, offline validation, model pinning, and evidence interpretation.
- [x] Distinguish logical evaluations from retry-bounded HTTP attempts; require disclosure and authorization for sensitive, large, or unclear-cost live runs.
- [x] Prevent prompt content or model answers from becoming executable behavior, unrestricted paths, or hidden policy.
- [x] Every copyable multi-stage workflow materializes output, checks status/completeness, and prevents partial failed output from reaching the next stage.
- [x] Provide reproducible fixtures and evaluations for map/reduce review, ranking/privacy/config precedence, retry budgets, prompt injection, uncertainty, and a deterministic no-JEQ case.
- [x] Skill validation and independent technical/safety review pass. No live TypeSafe call is needed to validate the skill.

## Non-goals
Do not replace JEQ reference docs, teach SDK integration, establish universal thresholds, or run paid content evaluations without an agreed budget.

