---
id: TASK-0014
title: Black-box binary suite
status: todo
depends_on: [TASK-0011, TASK-0012, TASK-0013]
priority: normal
tags: []
---

# Black-box binary suite

## Problem
The shell contract (exit codes, stdout/stderr separation, trailing newline) must hold for the compiled binary agents invoke.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Build the real binary once; execute it as a subprocess against fixtures and local fake servers—never call cli.Run directly.
- [ ] Matrix covers home/version/help/models/validate/ask, native/composed/file/stdin, default/explicit JSON, and all stable exit classes 0/1/2/130.
- [ ] Every case asserts parseable single-document stdout, exactly one trailing newline, stderr separation, no color/TTY variance, and no secret/stack/raw dependency leak.
- [ ] SIGINT a deliberately blocked request and assert prompt exit 130; `--timeout 50ms` produces GEV_TIMEOUT/exit 1 deterministically.
- [ ] Unknown command/flag names the way out; low-confidence response exits 0; ambiguous connection drop sends exactly one request.
- [ ] Suite is bounded, parallel-safe, uses no production network, and runs under `make check`.

## Notes
Paid production behavior is separate in TASK-0019; this suite owns deterministic shell-contract coverage.

