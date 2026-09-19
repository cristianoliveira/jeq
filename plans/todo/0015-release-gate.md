---
id: TASK-0015
title: Release gate
status: todo
depends_on: [TASK-0009, TASK-0014, TASK-0019]
priority: normal
tags: []
---

# Release gate

## Problem
v1 must ship with supply-chain, portability, and round-trip evidence.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] `nix develop -c make check` is green from a clean checkout and prints only `true` on success.
- [ ] `govulncheck ./...` findings are fixed or explicitly triaged; dependencies are pinned/license-compatible and the graph contains no rejected TOON encoder.
- [ ] Cross-build smoke succeeds for linux/darwin on amd64/arm64; release binary reports injected version/commit.
- [ ] JSON semantic corpus, black-box binary suite, and paid live TASK-0019 evidence all pass at the exact release commit.
- [ ] `docs/ARCHITECTURE.md` and `docs/DEVELOPMENT.md` describe package arrows, composition root, test commands, live-test opt-in/cost, and release procedure.
- [ ] Working tree is clean; no `.tmp`, credentials, raw live payloads, or ignored artifacts are committed; tag only after all board dependencies are done.

## Notes
The release gate does not accept skipped live verification as a pass.

