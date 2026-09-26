---
id: TASK-0078
title: Migrate CLI tests to Testify
status: todo
depends_on: [TASK-0076]
priority: normal
tags: [testing, readability, cli]
---

# Migrate CLI tests to Testify

## Problem
CLI tests report multiple exit, stream, renderer, read, and request conditions in compound manual failures, making the actual mismatch hard to find.

## Desired outcome
Clear failure diagnostics across all 25 CLI test files while preserving command behavior and every independent assertion.

## Acceptance criteria
- [ ] Inventory remaining `t.Fatal`/`t.Error` assertion sites and convert the meaningful checks to Testify `assert`/`require`. Record justified exceptions, especially test setup and asynchronous callbacks.
- [ ] Preserve existing test/subtest names, exit codes, stdout/stderr content, zero-network-before-validation guarantees, auth/privacy checks, read/request counts, retry bounds, and JSON ordering where observable.
- [ ] Guard typed `*jeq.Error` pointers before dereference. Use `require` only for fatal prerequisites and `assert` for independent checks. Do not call `require` inside workers, HTTP callbacks, or hooks invoked from other goroutines.
- [ ] Compare focused CLI coverage and test/subtest inventory before and after. Run focused tests, race tests for scheduler/stream concurrency, normal watcher gate, and CI.
- [ ] Publish reviewable CLI batch PRs if one diff is too large; each PR changes one coherent group, targets an integrated base, and is verified independently. Do not merge stacked children into stale parent branches.

## Constraints
Start from the merged pilot (TASK-0076) and avoid provider-test corrective PR conflicts. No new `mock`/`suite` use or production behavior changes. Maintain test readability: a simple failure check may remain with documented reason if replacement is less clear.
