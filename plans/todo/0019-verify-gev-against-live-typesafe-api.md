---
id: TASK-0019
title: Verify gev against live TypeSafe API
status: todo
depends_on: [TASK-0009, TASK-0011, TASK-0012, TASK-0013, TASK-0018]
priority: high
tags: []
---

# Verify gev against live TypeSafe API

## Problem
Before v1 ships, the compiled CLI must prove its JSON and TOON paths against the paid production TypeSafe service, not only fixtures.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Build the release candidate and run real `gev models`, `gev ask --output json`, default-TOON `gev ask`, and `gev validate` (offline control).
- [ ] Use synthetic state and a pinned model; batch Noul + Choice + Score into one paid ask per renderer.
- [ ] Assert exit codes, structured parsing, response shape/ranges, resolved model, and usage; never exact judgments.
- [ ] Compare JSON and decoded TOON semantically for the same response shape.
- [ ] Run one unhappy live path with an intentionally invalid key isolated to the process; assert GEV_AUTH_REJECTED/exit 1 and secret redaction.
- [ ] Record sanitized commands/evidence, latency, token usage, approximate cost, and release commit in `docs/qa/live-gev-verification.md`.

## Notes
Live verification is manual/release-gated, never part of normal CI and never silently skipped as a pass.

