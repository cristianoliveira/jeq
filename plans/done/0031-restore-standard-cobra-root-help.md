---
id: TASK-0031
title: Restore standard Cobra root help
status: done
depends_on: []
priority: high
tags: []
---

# Restore standard Cobra root help

## Problem
Bare gev currently prints a custom status dashboard. The root command should follow standard Cobra behavior and show help/usage instead of credential/model state.

## Acceptance criteria
- [x] Bare `gev` prints the same standard Cobra help as `gev --help` on stdout, exits 0, and writes no stderr.
- [x] Remove the custom root dashboard, credential readiness probe, default-model resolution, command list, and `Next:` output.
- [x] Bare root execution performs no environment, config, filesystem, credential, stdin, client, or network work.
- [x] Keep `gev examples` as explicit workflow discovery and keep all TASK-0030 output boundaries unchanged.
- [x] Update current docs/tests, run full checks and independent QA, then commit.

## Non-goals
Do not add a status command or move dashboard fields elsewhere.

