---
id: TASK-0055
title: Make trace emission concurrency-safe
status: done
depends_on: [TASK-0054]
priority: high
tags: [trace, concurrency, observability, determinism, testing]
---

# Make trace emission concurrency-safe

## Problem
The trace observer is shared by a command and its HTTP client. Concurrent map evaluations would mutate event counters and suppression state and write JSON lines through the same `io.Writer` without synchronization. Adding the map scheduler first would introduce data races, corrupt trace lines, and make sequence order unreliable.

## Desired outcome
A single `trace.Config` safely accepts concurrent operation and transport events. Each emitted line is complete JSON, sequence numbers match write order, existing privacy and event bounds remain intact, and disabled tracing stays cheap. This task changes tracing only; concurrent map scheduling remains blocked.

## Acceptance criteria

### Trace synchronization
- [ ] Reservation, event/suppression counters, sequence assignment, JSON encoding, and the corresponding writer call are coordinated so concurrent emissions cannot race or interleave.
- [ ] Trace lines appear in strictly increasing sequence order with no duplicates or gaps among emitted events.
- [ ] At most one `events.suppressed` event is emitted when the existing bound is crossed.
- [ ] Existing allowlisted fields, event names, bounds, privacy guarantees, and stdout/stderr separation remain unchanged.
- [ ] `Enabled`, command metadata, trace ID, and output writer are treated as immutable after command setup or are synchronized explicitly.
- [ ] Writer failures retain current non-fatal behavior; synchronization does not deadlock through nested suppression emission.

### Evidence
- [ ] Deterministic concurrent tests invoke operation, attempt, and retry events from multiple goroutines and parse every output line as one JSON object.
- [ ] Tests prove sequence order, suppression uniqueness, bounded event behavior, and unchanged disabled-trace output.
- [ ] Existing trace contract and privacy tests remain green.
- [ ] Focused `go test -race` passes for `internal/trace` and the HTTP observer paths without live calls.
- [ ] The configured watcher final gate and independent QA pass.

## Constraints
- Do not add map concurrency, change trace schema, expose payloads/URLs/credentials, or make tracing affect command success.
- Prefer one clear synchronization boundary over a mix of atomics and partially protected state.
- Keep the trace implementation independent of CLI scheduling policy.

## Implementation sequence
1. Add failing concurrent trace tests that demonstrate state and writer races.
2. Refactor emission so reserve, sequence, suppression, and line writing are one safe operation without recursive locking.
3. Run focused race detection, watcher verification, and independent QA.

## Notes
- The unsafe scheduler experiment was reverted without being committed.
- TASK-0056 will consume this guarantee before adding concurrent map workers.
