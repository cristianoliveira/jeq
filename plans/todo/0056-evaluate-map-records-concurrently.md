---
id: TASK-0056
title: Evaluate map records concurrently
status: doing
depends_on: [TASK-0054, TASK-0055]
priority: high
tags: [cli, map, concurrency, backpressure, determinism]
---

# Evaluate map records concurrently

## Problem
Once trace emission is concurrency-safe, sequential network evaluation still limits map throughput. jeq should own a small fixed worker bound, ordered output, and backpressure so callers gain parallelism without configuring concurrency or writing orchestration.

## Desired outcome
The unchanged map command uses a small fixed internal worker bound for NDJSON evaluation. Users provide semantic intent only; there is no concurrency flag or configuration. jeq preserves deterministic output order, bounds read-ahead and memory, and coordinates failure and cancellation safely.

```sh
cat records.ndjson | jeq map --input ndjson --as risk ...
```

## Acceptance criteria

### Internal scheduling
- [ ] A small fixed number of independent record evaluations can be in flight concurrently.
- [ ] No `--concurrency`, environment variable, config field, or machine-derived worker count is added.
- [ ] Output records are written in original input order even when evaluations complete out of order.
- [ ] Reader progress, queued records, completed results, and memory are bounded by a small constant related to the worker limit.
- [ ] Slow evaluation or stdout applies backpressure rather than accumulating the remaining stream.
- [ ] Each record is evaluated at most once, excluding the existing explicit 429/529 retry policy; scheduling creates no duplicates or speculative retries.

### Ordered failure and cancellation
- [ ] The coordinator commits results strictly by input index. A later failure cannot discard already successful lower-index records.
- [ ] On failure, stdout contains only the contiguous successful prefix that sequential map would have emitted; no later record is written.
- [ ] Once a failure is known, no new record is dispatched. Already in-flight work is bounded and canceled or drained without leaks.
- [ ] Context/signal cancellation and stdout failure stop reading and outstanding requests promptly with existing error classifications.
- [ ] Retry diagnostics, trace events, rendering, and final summary counts remain correct under concurrent completion.
- [ ] Partial-output and request-count behavior is deterministic enough for caller-owned checkpoint/retry policy.

### Evidence
- [ ] Channel-controlled tests prove multiple simultaneous evaluations without sleep-based timing assertions.
- [ ] Tests force out-of-order success and later-index-first failure, proving byte-for-byte ordered output and contiguous-prefix behavior.
- [ ] Tests prove bounded read-ahead/backpressure, cancellation, output failure, no duplicate evaluation, and no dispatch after known failure.
- [ ] Retry and trace integration tests prove attempt counts, diagnostics, events, and summaries remain valid.
- [ ] Focused race detection passes for CLI, trace, and HTTP-adapter paths exercised by concurrent map.
- [ ] The configured watcher final gate and independent QA pass without live or paid calls.

## Constraints
- TASK-0054 streaming and TASK-0055 trace synchronization must be accepted first.
- Preserve domain request/enrichment semantics and append-only `_jeq.<name>` output.
- Keep the worker bound internal and fixed. Performance tuning is not user policy.
- Do not add hierarchical reduce, semantic batching, grouped reduction, checkpoint persistence, or unordered output.

## Implementation sequence
1. Add deterministic failing scheduler tests for simultaneous work, order, backpressure, and later-index-first failure.
2. Add a bounded result coordinator around TASK-0054's lazy reader, relying on TASK-0055's safe trace observer.
3. Add cancellation/output/retry integration coverage and run focused race detection.
4. Run the watcher final gate and independent QA.

## Notes
- The first unsafe scheduler experiment was reverted without being committed.
- Concurrency improves throughput only; it does not increase provider context limits or reduce request count or cost.

