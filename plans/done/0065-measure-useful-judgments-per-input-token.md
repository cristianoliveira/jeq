---
id: TASK-0065
title: Measure useful judgments per input token
status: done
depends_on: []
priority: high
tags: [cost, usage, observability, cli, typesafe]
---

# Measure useful judgments per input token

## Problem
jeq exposes provider usage inside individual responses but does not summarize stream-wide token efficiency, so we cannot compare request shapes or detect cost regressions with evidence.

## Desired outcome
A user can measure the token efficiency of every paid jeq command without changing its data output. For streams, one final machine-readable summary reports successful provider requests, input records, returned answers, input tokens, and useful answers per 1,000 input tokens.

A useful answer is one provider answer that passed contract decoding and is attached to the state or record it was requested for. Request count and latency remain separate metrics because TypeSafe bills input tokens rather than a flat amount per call.

## Approach
Aggregate the `usage` already returned by TypeSafe. Keep stdout unchanged and emit the optional summary on stderr through the existing trace/diagnostic boundary. Treat provider-reported input tokens as authoritative; do not build a speculative tokenizer or hard-code a dollar price.

## Acceptance criteria
- [x] Define the efficiency metric as `successful decoded answers / input tokens`, reported as answers per 1,000 input tokens; report the raw numerator and denominator beside it.
- [x] Add an explicit CLI option that emits one final machine-readable usage summary on stderr for `ask`, `map`, `rate`, `rank`, and `reduce`; normal stdout and exit behavior remain unchanged.
- [x] The summary includes attempted and successful provider requests, processed records, decoded answers, input tokens, output tokens, elapsed time, and resolved model versions when available.
- [x] Multi-question requests count each successfully decoded answer once; records and requests are not mislabeled as answers.
- [x] Stream summaries aggregate only usage actually returned by the provider and remain correct when results complete out of order.
- [x] Cancellation, malformed responses, retries without usage, and prefix-preserving failures never invent or double-count tokens.
- [x] Tests cover one-state/many-question, multi-record, partial failure, cancellation, retry, and zero-token fixtures with deterministic injected responses.
- [x] Documentation explains that the summary measures observed successful-response usage, not necessarily every charge a provider could apply to failed requests.
- [x] Fresh watcher passes at clean committed HEAD.

## Non-goals
- Predicting tokens before a request.
- Hard-coded USD pricing.
- Changing request packing or caching in this task.

## Constraints
- stdout remains composable JSON or NDJSON.
- No additional provider call is made to calculate usage.
- Trace and summary writes remain concurrency-safe.

## Completion
- Code commits: `63c856b` and `e2f5a1c`, both reference TASK-0065.
- QA: Kelly approved exact HEAD `e2f5a1cf5617d6d657bab59bacdbc72d109f740c`; no blockers.
- Focused CLI/domain/adapter tests and targeted race tests passed. Watcher generation 142 passed the full configured gate with current freshness.
- No paid or live provider calls. This task unblocked TASK-0066.

