---
id: TASK-0055
title: Evaluate map records concurrently
status: todo
depends_on: [TASK-0054]
priority: high
tags: [cli, map, concurrency, backpressure, determinism, observability]
---

# Evaluate map records concurrently

## Problem
After NDJSON map becomes lazy, sequential network evaluation still limits throughput. jeq should own a small fixed worker bound, ordered output, and backpressure so callers gain parallelism without configuring concurrency or writing orchestration.

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
- [ ] Reader progress, queued records, completed results, and memory are bounded by a small constant related to the internal worker limit.
- [ ] Slow evaluation or stdout applies backpressure rather than accumulating the remaining stream.
- [ ] Each record is evaluated at most once, excluding the existing explicit 429/529 retry policy; scheduling creates no duplicates or speculative retries.

### Failure and cancellation
- [ ] On evaluation failure, stdout contains only the contiguous successful prefix that sequential map would have emitted; no later record is written.
- [ ] Once failure is known, no new record is dispatched. Already in-flight work is bounded and canceled or drained without goroutine leaks.
- [ ] Context/signal cancellation and stdout failure stop reading and outstanding requests promptly with existing error classifications.
- [ ] The shared provider client, retry diagnostics, trace observer, renderer, and summary counters are race-safe.
- [ ] Partial-output and request-count behavior is explicit and deterministic enough for callers to retry from their own checkpoint.

### Evidence
- [ ] Channel-controlled tests prove multiple simultaneous evaluations without sleep-based timing assertions.
- [ ] Tests force out-of-order completion and prove byte-for-byte ordered output.
- [ ] Tests prove bounded read-ahead/backpressure, earliest-record failure behavior, cancellation, output failure, and no dispatch after known failure.
- [ ] Retry and trace tests prove concurrency does not corrupt attempt counts, diagnostics, event records, or final summaries.
- [ ] Focused race detection passes for CLI, trace, and HTTP-adapter code exercised by concurrent map.
- [ ] The configured watcher final gate and independent QA pass without live or paid calls.

## Constraints
- TASK-0054 streaming behavior must be complete first; do not combine both risk surfaces into one implementation commit.
- Preserve current domain request/enrichment semantics and append-only `_jeq.<name>` output.
- Keep the worker bound internal and fixed for the first implementation. Performance tuning is not user policy.
- Do not add hierarchical reduce, automatic semantic batching, grouped reduction, checkpoint persistence, or unordered output.

## Implementation sequence
1. Add deterministic failing tests for simultaneous in-flight work, ordering, backpressure, and failure semantics.
2. Add the bounded scheduler/coordinator around TASK-0054's lazy record stream.
3. Make shared trace/retry/rendering paths race-safe where evidence requires changes.
4. Run focused race detection, the watcher final gate, and independent QA.

## Notes
- Concurrency improves throughput only. It does not increase provider request/context limits or reduce the number and cost of evaluations.
- A fixed internal bound makes initial behavior predictable across machines. An override should be introduced only from measured operational evidence.

