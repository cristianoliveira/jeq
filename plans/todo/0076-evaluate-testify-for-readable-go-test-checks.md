---
id: TASK-0076
title: Evaluate Testify for readable Go test checks
status: doing
depends_on: []
priority: normal
tags: [testing, readability, dependencies]
---

# Evaluate Testify for readable Go test checks

## Problem
Go tests use repeated manual conditionals and failure messages that can hide the failed condition. Adding every Testify package without a use case could replace clear checks with a larger dependency and more indirection.

## Desired outcome
Use Testify only where it makes test intent and failure output clearer. Establish a small, reviewable convention for `assert` and `require` before considering a wider migration.

## Acceptance criteria
- [ ] Compare two or three representative existing tests with `testing` checks and Testify equivalents. Include both a happy path and an error path; show how their failure messages differ. Keep test names, inputs, request counts, and observed behavior unchanged.
- [ ] Introduce a pinned Testify test dependency only if the pilot improves readability. Use `require` for prerequisites that must stop the subtest; use `assert` for independent observations. Do not replace clear `t.Fatal`/`t.Errorf` checks mechanically.
- [ ] Do not add `mock` or `suite` imports unless a separate concrete test needs them and their extra structure is easier to read than the current fake or plain `t.Run`. Installing the module does not require using every package.
- [ ] Check the added module graph, licenses, and known-vulnerability findings; confirm production packages do not import Testify. Record any Nix/offline CI or dependency-policy changes needed.
- [ ] Run focused package tests and compare coverage before and after. Run the normal watcher gate; inspect the final diff to confirm no assertions were weakened and no formatter-only churn was introduced.
- [ ] If the pilot does not improve readability or dependency cost is unjustified, leave the existing checks in place and record a no-adoption decision with evidence.

## Constraints
Keep this separate from the open descriptive-subtest PR stack. One focused PR for the pilot; do not convert the whole test suite by default. Preserve deterministic offline tests and keep test-only imports out of production code.

## Context
Start with the repeated result checks in `internal/infra/source/optional_test.go` and one CLI or domain test with several independent assertions. PRs #10 and #11 may move those tests; use the latest merged revision before choosing the pilot. The existing `go.mod` has no Testify dependency. Follow `docs/DEVELOPMENT.md` for offline verification and `plans/README.md` for board lifecycle.
