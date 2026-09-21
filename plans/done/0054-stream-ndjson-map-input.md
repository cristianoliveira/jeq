---
id: TASK-0054
title: Stream NDJSON map input
status: done
depends_on: []
priority: high
tags: [cli, map, streaming, scalability, determinism]
---

# Stream NDJSON map input

## Problem
`jeq map` buffers the complete input before evaluating its first record. The 64 MiB whole-input and 10,000-record limits force callers to partition large datasets even though each map judgment is independent.

## Desired outcome
For NDJSON input, `jeq map` lazily reads one bounded record at a time and emits its enriched result before reading the complete stream. The command remains sequential in this task so streaming behavior and existing failure semantics can be established independently from concurrency.

```sh
cat records.ndjson | jeq map --input ndjson --as risk ...
```

## Acceptance criteria

### Streaming behavior
- [ ] NDJSON map no longer reads or stores the complete stdin document before its first evaluation or output.
- [ ] The 64 MiB whole-input and 10,000-record limits no longer apply to NDJSON map. The existing 8 MiB per-record limit remains enforced before evaluation.
- [ ] Empty lines remain ignored; malformed JSON, oversized records, and non-object records return the existing stable input error class.
- [ ] Records are evaluated and written sequentially in input order, preserving current stdout and request-count behavior.
- [ ] Memory remains bounded by one record plus fixed reader/output overhead, independent of total stream length.
- [ ] JSON input keeps its current single-document behavior and safety bound; this task does not redefine JSON as an array.

### Failure and cancellation
- [ ] On failure, stdout contains the same contiguous successful prefix as current sequential map.
- [ ] A provider, context, stdin-read, or stdout-write failure stops further reading and evaluation promptly.
- [ ] Signal cancellation keeps the existing interrupted classification.
- [ ] No goroutine or response-body leak is introduced.

### Evidence
- [ ] Deterministic tests prove first output occurs before the input stream reaches EOF.
- [ ] Tests cover empty lines, malformed and oversized records, read failures, output failures, and cancellation without sleep-based timing.
- [ ] A local fake evaluator processes more than the former 10,000-record limit without a live call.
- [ ] Existing native/composed request modes, retries, rendering, tracing, and NDJSON errors remain covered.
- [ ] Focused tests and the configured watcher final gate pass.

## Constraints
- Keep request/enrichment semantics in the domain pipeline and stream reading/orchestration in the CLI boundary.
- Preserve append-only `_jeq.<name>` evidence and Unix stdin/stdout behavior.
- Do not add concurrency, batching flags, hierarchical reduction, persistence, or paid/live verification in this task.

## Implementation sequence
1. Add failing tests for lazy first output, removal of total/count limits, and sequential failure behavior.
2. Introduce a bounded NDJSON record reader with explicit oversized-line detection and read-error propagation.
3. Route NDJSON map through the reader while leaving JSON on the existing bounded single-document path.
4. Run focused tests and the watcher final gate; close before starting concurrent execution.

## Notes
- Current limits are 8 MiB per record, 10,000 records, and 64 MiB total buffered input.
- This task makes arbitrary finite NDJSON streams possible over time, but intentionally preserves one in-flight evaluation.
