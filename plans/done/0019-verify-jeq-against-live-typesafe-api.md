---
id: TASK-0019
title: Verify jeq against live TypeSafe API
status: done
depends_on: [TASK-0009, TASK-0011, TASK-0012, TASK-0013, TASK-0018]
priority: high
tags: []
---

# Verify jeq against live TypeSafe API

## Problem
Before v1 ships, the compiled JSON-only CLI must prove its full command path against the paid production TypeSafe service, not only fixtures.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Build the release candidate and run real `jeq models`, default JSON `jeq ask`, explicit `--output json`, and `jeq validate` (offline control).
- [ ] Use synthetic state and a pinned model; batch Noul + Choice + Score into one paid ask (a second only if needed to prove explicit-output equivalence).
- [ ] Assert exit codes, structured parsing, response shape/ranges, resolved model, usage, and default/explicit JSON semantic equality; never exact judgments.
- [ ] Run one unhappy live path with an intentionally invalid key isolated to the process; assert JEQ_AUTH_REJECTED/exit 1 and secret redaction.
- [ ] Record sanitized commands/evidence, latency, token usage, approximate cost, and release commit in `docs/qa/live-jeq-verification.md`.

## Notes
Live verification is manual/release-gated, never part of normal CI and never silently skipped as a pass.

