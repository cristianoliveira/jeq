---
id: TASK-0065
title: Measure useful judgments per input token
status: doing
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
- [ ] Define the efficiency metric as `successful decoded answers / input tokens`, reported as answers per 1,000 input tokens; report the raw numerator and denominator beside it.
- [ ] Add an explicit CLI option that emits one final machine-readable usage summary on stderr for `ask`, `map`, `rate`, `rank`, and `reduce`; normal stdout and exit behavior remain unchanged.
- [ ] The summary includes attempted and successful provider requests, processed records, decoded answers, input tokens, output tokens, elapsed time, and resolved model versions when available.
- [ ] Multi-question requests count each successfully decoded answer once; records and requests are not mislabeled as answers.
- [ ] Stream summaries aggregate only usage actually returned by the provider and remain correct when results complete out of order.
- [ ] Cancellation, malformed responses, retries without usage, and prefix-preserving failures never invent or double-count tokens.
- [ ] Tests cover one-state/many-question, multi-record, partial failure, cancellation, retry, and zero-token fixtures with deterministic injected responses.
- [ ] Documentation explains that the summary measures observed successful-response usage, not necessarily every charge a provider could apply to failed requests.
- [ ] Fresh watcher passes at clean committed HEAD.

## Non-goals
- Predicting tokens before a request.
- Hard-coded USD pricing.
- Changing request packing or caching in this task.

## Constraints
- stdout remains composable JSON or NDJSON.
- No additional provider call is made to calculate usage.
- Trace and summary writes remain concurrency-safe.

