---
id: TASK-0057
title: Add a local map concurrency stress example
status: done
depends_on: [TASK-0056]
priority: high
tags: [examples, map, concurrency, stress, observability]
---

# Add a local map concurrency stress example

## Problem
The bounded concurrent map scheduler is covered by tests, but users cannot run a visible end-to-end workload that proves requests overlap, output stays ordered, and concurrency remains capped without spending provider budget.

## Desired outcome
A self-contained example starts a local fake System One endpoint, sends a configurable NDJSON workload through the real `jeq map` command, and reports record count, request count, peak in-flight requests, elapsed time, and order verification. It exits non-zero when concurrency exceeds four, requests never overlap, output is missing or reordered, or a request leaves the local endpoint.

## Acceptance criteria
- [ ] The documented command runs against a loopback-only fake endpoint and needs no provider credential or paid call.
- [ ] The example invokes the installed `jeq` binary and the real `map --input ndjson` command rather than duplicating scheduler code.
- [ ] Default workload is large enough to exercise repeated worker waves and can be adjusted with an explicit record-count argument.
- [ ] The fake endpoint measures current and peak in-flight requests safely.
- [ ] The run proves peak concurrency is greater than one and no greater than the fixed internal bound of four.
- [ ] The run parses every output line and proves record count and input order are preserved.
- [ ] Human output states workload size, request count, peak concurrency, elapsed duration, and PASS/FAIL.
- [ ] Automated tests execute the example with the repository-built binary and assert behavior, not README text.
- [ ] No wall-clock speedup threshold is used as correctness evidence; elapsed time is informational only.
- [ ] Watcher and independent QA pass without live calls.

## Constraints
- Keep the example local, deterministic, and dependency-light.
- Do not add a public concurrency option or import internal scheduler packages.
- Do not expose a fake endpoint beyond loopback.
- Do not weaken production timeouts or retry policy for the example.

## Suggested interface

```sh
go install ./cmd/jeq
go run ./examples/map-concurrency --records 100
```

## Notes
- This is an executable demonstration and regression example, not a throughput benchmark for the external provider.
- Timing varies by machine; measured overlap and ordered output are the correctness signals.

