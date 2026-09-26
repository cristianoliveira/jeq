---
id: TASK-0079
title: Migrate adapter and support tests to Testify
status: doing
depends_on: [TASK-0076]
priority: normal
tags: [testing, readability, infrastructure]
---

# Migrate adapter and support tests to Testify

## Problem
Infrastructure and support tests mix manual assertions with file, HTTP, trace, fixture, architecture, and process setup, so a broad replacement could hide the failed condition or call fatal assertions from the wrong goroutine.

## Desired outcome
Readable, behavior-preserving checks across the 7 infrastructure, 3 trace, 2 fixture, 1 architecture, and 1 command test files.

## Acceptance criteria
- [ ] Inventory manual assertion sites and convert suitable checks to `assert`/`require`; record justified exceptions for setup and callback contexts.
- [ ] Preserve HTTP status/body/headers, retry counts and delays, response extras, bounded reads, trace privacy, import-architecture rules, and CLI process exits.
- [ ] Keep all Testify assertions on the test goroutine. HTTP handlers must record observations for later assertions; preload fixture bytes rather than introducing `require` or fatal fixture helpers inside handlers.
- [ ] Guard typed `*jeq.Error` nil before field access; preserve `errors.Is`/`errors.As`, request count, and negative assertions.
- [ ] Compare per-package coverage and test/subtest inventory; run focused tests, race tests where relevant, normal watcher gate, and CI. Publish focused PRs by coherent adapter/support area with verified base/head diffs.

## Constraints
Depends on merged TASK-0076. No production imports of Testify, behavior changes, blanket helper abstractions, `mock`, or `suite`. Existing module license/security review from the pilot must remain valid.
