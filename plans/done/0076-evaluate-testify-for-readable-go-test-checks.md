---
id: TASK-0076
title: Evaluate Testify for readable Go test checks
status: done
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
- [x] Compare representative existing tests with `testing` checks and Testify equivalents. Include both a happy path and an error path; show how their failure messages differ. Keep test names, inputs, request counts, and observed behavior unchanged.
- [x] Introduce a pinned Testify test dependency only if the pilot improves readability. Use `require` for prerequisites that must stop the subtest; use `assert` for independent observations. Do not replace clear `t.Fatal`/`t.Errorf` checks mechanically.
- [x] Do not add `mock` or `suite` imports unless a separate concrete test needs them and their extra structure is easier to read than the current fake or plain `t.Run`. Installing the module does not require using every package.
- [x] Check the added module graph, licenses, and known-vulnerability findings; confirm production packages do not import Testify. Record any Nix/offline CI or dependency-policy changes needed.
- [x] Run focused package tests and compare coverage before and after. Run the normal watcher gate; inspect the final diff to confirm no assertions were weakened and no formatter-only churn was introduced.
- [x] The pilot improved readability, so the no-adoption path is not applicable; record the adoption decision and evidence.

## Constraints
Keep this separate from the open descriptive-subtest PR stack. One focused PR for the pilot; do not convert the whole test suite by default. Preserve deterministic offline tests and keep test-only imports out of production code.

## Context
Start with the repeated result checks in `internal/infra/source/optional_test.go` and one CLI or domain test with several independent assertions. PRs #10 and #11 may move those tests; use the latest merged revision before choosing the pilot. The existing `go.mod` has no Testify dependency. Follow `docs/DEVELOPMENT.md` for offline verification and `plans/README.md` for board lifecycle.

## Completion
- Pilot PR #13 merged to `main` as `5152325` after green CI and Kelly's review. It uses `assert`/`require` in CLI and source tests only. Domain tests remain unchanged because their depguard rule requires an explicit test-only exception in TASK-0077.
- Source/CLI/pipeline coverage stayed 83.6%/82.3%/71.0%. A typed `*jeq.Error` uses `require.NotNil` before field access; no `mock`/`suite` imports were added.
- Testify v1.12.1 and the Nix vendor hash are pinned. Dependency licenses and `govulncheck` were reviewed; Nix build, focused tests, watcher, and GitHub CI passed. Remote `main` was verified to contain both the pilot and the separate provider correction PR #12.
- The user subsequently requested a full-suite conversion; that separate scope is tracked by TASK-0077 through TASK-0080.
