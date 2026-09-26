---
id: TASK-0077
title: Migrate domain tests to Testify
status: doing
depends_on: [TASK-0076]
priority: normal
tags: [testing, readability, domain]
---

# Migrate domain tests to Testify

## Problem
Domain test failures often report several conditions in one manual `t.Fatal` message. The Testify pilot cannot be used in domain tests yet because depguard permits only standard library, domain, and fixture imports there.

## Desired outcome
Readable, independent expected/actual checks in all 12 domain test files without weakening domain isolation or changing behavior.

## Acceptance criteria
- [ ] Permit only Testify `assert` and `require` in `internal/domain/**/*_test.go` via the test-only depguard rule. Keep production domain's standard-library-only rule unchanged and document the distinction.
- [ ] Inventory every remaining manual assertion in the domain tests. Convert checks where Testify expresses the same condition; list any retained `t.Fatal`/`t.Error` with a reason, including callback or setup failures that cannot safely use Testify.
- [ ] Preserve every test/subtest, input, expected error code, error identity/wrapping (`errors.Is`/`errors.As`), response bytes, and evaluator call count. Guard typed `*jeq.Error` pointers with `require.NotNil` or an explicit typed nil check before reading fields; never rely on `require.Error` for this guard.
- [ ] Use `require` only where further assertions are unsafe; use `assert` for independent observations. Do not call `require` in worker goroutines.
- [ ] Compare domain package coverage and test/subtest inventory before and after; run focused tests, lint/architecture checks, normal watcher gate, and CI. Keep one focused PR for this domain refactor, with no `mock` or `suite` imports.

## Constraints
Depends on the merged Testify pilot (TASK-0076). Keep domain production code and public behavior unchanged. Avoid mechanical rewrites that add noise or weaken failure messages; any justified exception must be recorded in the PR.
