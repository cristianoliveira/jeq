---
id: TASK-0018
title: Capture paid live TypeSafe API baseline
status: doing
depends_on: []
priority: high
tags: []
---

# Capture paid live TypeSafe API baseline

## Problem
Mock servers prove gev behavior but not the real TypeSafe authentication, endpoint, response schema, model resolution, or usage accounting.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Use `TYPESAFE_API_KEY` from the environment without printing it or placing its value in logs/files.
- [ ] Real `GET /v1/models` returns 200 and a parseable non-empty model list.
- [ ] One paid real `POST /v1/systemone` uses synthetic state, pinned `jev-1.13.0`, and one Noul + Choice + Score in the same request.
- [ ] Assert response shape only: resolved model present; matching named answers/types; Noul in [0,1]; Choice/Score probabilities in [0,1] and sum≈1; confidence in [0,1] where present; score legend present; non-negative token usage.
- [ ] Record HTTP status, latency, resolved model, token usage, approximate input cost, and sanitized response evidence in `docs/qa/live-api-baseline.md`.
- [ ] Never assert exact model answer values and never send proprietary/personal state.

## Notes
This is deliberately paid evidence. Raw response stays in `.tmp/`; committed evidence is synthetic and sanitized.

