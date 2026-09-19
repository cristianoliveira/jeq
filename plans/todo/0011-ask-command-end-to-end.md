---
id: TASK-0011
title: ask command end-to-end
status: todo
depends_on: [TASK-0005, TASK-0006, TASK-0007, TASK-0008, TASK-0010, TASK-0017, TASK-0020]
priority: high
tags: []
---

# ask command end-to-end

## Problem
The primary capability must work: flags to compose to client to render to exit code.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] `gev ask` wires source selection → bounded reads → Compose/Validate → credential lookup → TypeSafe client → selected renderer, in that order.
- [ ] Native `--request` and composed `--questions` plus exactly one state source work with files and explicit `-`; all conflicts fail before filesystem, env, or network access.
- [ ] Flags: `--model`, `--base-url`, `--timeout` (10s default), `--max-retries` (0..5, default 2), and `--output json` for D2; env precedence follows ADR 0001.
- [ ] Missing/rejected auth, 422, retry exhaustion, timeout, malformed response, source failure, invalid input, and interruption map to stable documents and exits 1/2/130.
- [ ] Success emits exactly one lossless document/newline on stdout; stderr contains retry diagnostics only; low confidence still exits 0.
- [ ] Black-box-like command tests use `httptest.Server`, injected stdin/files/env/clock/sleeper, and assert request count/body, stdout/stderr, and exit code for native/composed happy and unhappy paths.

## Notes
Real paid CLI verification is TASK-0019. Normal tests never call production.

