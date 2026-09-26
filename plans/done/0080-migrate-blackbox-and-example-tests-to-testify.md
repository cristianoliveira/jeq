---
id: TASK-0080
title: Migrate blackbox and example tests to Testify
status: done
depends_on: [TASK-0076]
priority: normal
tags: [testing, readability, blackbox, examples]
---

# Migrate blackbox and example tests to Testify

## Problem
Blackbox and example tests check subprocess exits, streams, local fake APIs, and workflow receipts in compound failures that do not always identify which contract broke.

## Desired outcome
Clear assertions across all 11 blackbox and 7 example test files without changing executable workflows, test fixtures, or network boundaries.

## Acceptance criteria
- [x] Inventory manual assertion sites and convert suitable checks to Testify; document exceptions where original checks are clearer or an assertion runs outside the test goroutine.
- [x] Preserve every CLI exit, stdout/stderr contract, privacy/secret check, fake API request count and order, partial output, timeout/cancellation, offline guarantee, and script fixture.
- [x] Use `require` only before unsafe downstream work such as parsed output dereference; use `assert` for independent outcomes. Do not use `require` inside server handlers, subprocess callbacks, or workers.
- [x] Compare focused blackbox/example coverage and test/subtest inventory before/after; run subprocess-focused and race-sensitive tests, normal watcher gate, and CI.
- [x] Publish reviewable PRs by coherent binary or example workflow group, with scope verified against the integrated base before any merge.

## Completion evidence

- PR #23 migrated four workflow example test files and PR #24 migrated `examples/examples_test.go`; both passed independent Kelly review and GitHub CI, and are merged to `main`.
- Before/after `go test -json ./examples` inventories match exactly: 74 passing test/subtest events.
- Focused example tests and race tests, full `go test ./...`, and `nix develop -c golangci-lint run` passed. Full `go test ./...` passed on merged main `5f12bfd`.
- Coverage runs for `./examples` and `./internal/blackbox` report `[no statements]` both before and after because these external test harness packages have no production statements to instrument; execution inventory is the applicable regression comparison.
- Audited all 69 `*_test.go` files: zero direct `testing.T` `Error`, `Errorf`, `Fatal`, `Fatalf`, `Fail`, or `FailNow` calls remain. No remaining manual assertion exceptions were found; no Testify assertions run in fake server handlers or callbacks.
- Funzzy watcher was unavailable (`.watch.sock` missing); direct tests/lint and green GitHub CI provide the checks.

## Constraints
Depends on merged TASK-0076. No paid provider tests, production behavior changes, `mock`, or `suite`. Prefer meaningful failure diagnostics over shorter code.
