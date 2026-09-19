---
id: TASK-0007
title: Bounded retry policy
status: done
depends_on: [TASK-0006]
priority: normal
tags: []
---

# Bounded retry policy

## Problem
Documented 429/529 responses need bounded, Retry-After-aware retries that never replay ambiguous transport failures.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Bounds fixed (OQ-3 resolved): default 2 retries (3 attempts max); `--max-retries` accepts 0–5.
- [ ] Backoff: 250ms base, doubling per attempt (250ms, 500ms, …); any single wait capped at 10s.
- [ ] `Retry-After` honored when ≤ 10s; if absent, malformed, or > 10s, fall back to the backoff schedule.
- [ ] Retries only on 429 and 529. Never on 401/422/5xx, never after an ambiguous post-send transport failure (exactly one request on connection drop).
- [ ] Sleep is injected; tests assert recorded waits, never real sleeping.
- [ ] Retry attempts log one line to stderr (diagnostics channel), never stdout.

## Notes
Resolves Kelly OQ-3. Timeout override for binary-level tests uses the existing `--timeout` flag (OQ-4; no new knob).

