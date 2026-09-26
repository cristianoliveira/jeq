---
id: TASK-0080
title: Migrate blackbox and example tests to Testify
status: doing
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
- [ ] Inventory manual assertion sites and convert suitable checks to Testify; document exceptions where original checks are clearer or an assertion runs outside the test goroutine.
- [ ] Preserve every CLI exit, stdout/stderr contract, privacy/secret check, fake API request count and order, partial output, timeout/cancellation, offline guarantee, and script fixture.
- [ ] Use `require` only before unsafe downstream work such as parsed output dereference; use `assert` for independent outcomes. Do not use `require` inside server handlers, subprocess callbacks, or workers.
- [ ] Compare focused blackbox/example coverage and test/subtest inventory before/after; run subprocess-focused and race-sensitive tests, normal watcher gate, and CI.
- [ ] Publish reviewable PRs by coherent binary or example workflow group, with scope verified against the integrated base before any merge.

## Constraints
Depends on merged TASK-0076. No paid provider tests, production behavior changes, `mock`, or `suite`. Prefer meaningful failure diagnostics over shorter code.
