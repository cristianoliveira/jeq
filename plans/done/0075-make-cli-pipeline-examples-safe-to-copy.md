---
id: TASK-0075
title: Make CLI pipeline examples safe to copy
status: done
depends_on: []
priority: high
tags: [cli, ux, examples, shell, testing]
---

# Make CLI pipeline examples safe to copy

## Problem
The rank help example reads `handlers.ndjson` without `--input ndjson`, so it fails with an array-shape error. Built-in gate-to-jq recipes omit `pipefail`: a rejected gate followed by successful jq returns pipeline status 0. Text-only assertions do not catch these workflow failures.

## Desired outcome
Published shell blocks can be copied into their declared shell and behave as described, including failures. Users learn gate semantics directly from help rather than an external guide or a failed production script.

## Acceptance criteria
- [x] Fix the rank example's NDJSON flag and supply or explicitly create the synthetic candidate/state inputs used by copyable examples.
- [x] Audit built-in recipes and the primary README/getting-started/composition examples for framing, required input creation, declared tools, and error propagation.
- [x] Bash pipeline recipes enable `set -o pipefail` within the copied block when a later stage could hide an earlier failure. Identify Bash as a prerequisite where its semantics are required.
- [x] Explain that pipefail returns the rightmost failing stage; show `PIPESTATUS` only when a workflow must inspect individual stages. Do not promise pipefail preserves every stage's exact error.
- [x] Gate help includes exits 0/10/11, required thresholds, `0 <= reject-max < pass-min <= 1`, inclusive comparisons, and reject-over-uncertain aggregate status.
- [x] Explain that gate emits all processed decisions, including reject/uncertain; filtering and actions remain the caller's job.
- [x] Copyable policy examples deliberately handle nonzero policy outcomes. Do not add blanket `set -e` that prevents intended reject/uncertain handling.
- [x] Execute representative published shell recipes with synthetic data and a local stub for provider calls. Assert actual output and status for pass, reject, uncertain, malformed input, and upstream failure followed by a successful downstream command.
- [x] Keep recipe discovery offline and non-executing. Tests must not call a paid provider or infer actions from model text.
- [x] Focused tests and the configured watcher gate pass.

## Constraints
Use one recipe data source. Shell files and logs, if any, are explicitly created by the caller or test harness, never remembered by jeq. Keep result schemas and existing policy semantics unchanged.

## Context
Inspect `internal/cli/examples.go`, `rank.go`, `gate.go`, `examples/*_test.go`, and nearby CLI example tests. Follow-up to TASK-0029/TASK-0068. TASK-0071 owns navigation; this task owns executable correctness, so no start dependency is required.

## Completion
- Code commit: `2772e16 fix(examples): handle pipeline policy exits (TASK-0075)`.
- QA: Kelly approved exact HEAD `2772e163b437a50ec30831b4688b8cc19ea0f1e2`; no blocking gaps.
- Verification: `go test ./internal/cli ./examples -count=1`, Bash syntax checks for published snippets, and configured watcher generation 75 passed.
- The shell tests use a local jeq stub and offline gate; no paid provider calls.
