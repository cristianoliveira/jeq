---
id: TASK-0066
title: Evaluate token-efficient map request shapes
status: doing
depends_on: [TASK-0065]
priority: high
tags: [cost, benchmark, map, batching, deduplication, typesafe]
---

# Evaluate token-efficient map request shapes

## Problem
map sends one request per record, but TypeSafe charges per input token. We do not know when per-record calls, deduplication, or shared-context batches produce the most useful judgments per token without harming answer quality.

## Desired outcome
A reproducible benchmark identifies which request shape delivers the most useful map answers per input token for each supported workload. The result states where the current per-record shape is already optimal, where exact-request deduplication helps, and whether shared-context batching saves tokens without changing decisions.

## Approach
Compare three strategies against the current map contract:

1. One request per selected record with every configured question.
2. Canonical-request deduplication for byte-identical evaluations.
3. One structured state containing several records, with namespaced per-record questions that point explicitly to one record.

Use representative fixtures rather than one favorable example: short unrelated records, large unrelated records, several questions per record, duplicate records, and records sharing a large reference document. Measure provider-reported input tokens, useful answers, latency, request count, probability movement, and threshold decisions.

## Acceptance criteria
- [ ] Add a reproducible benchmark harness whose inputs and calculations are committed, while live credentials, response caches, and paid outputs remain ignored.
- [ ] Establish the existing per-record map behavior as the baseline using TASK-0065 usage summaries.
- [ ] Compare all three strategies on short unrelated records, large unrelated records, multi-question records, exact duplicates, and large shared-context workloads.
- [ ] Report total input tokens, answers per 1,000 input tokens, requests, latency, and request-size distribution for every case.
- [ ] Prove offline that batched answer IDs map back to the correct record and original question without changing input order.
- [ ] Evaluate quality using repeated provider runs: compare mean probabilities and configured threshold decisions against both baseline variation and expected labels; batching does not pass solely because it is cheaper.
- [ ] Keep each experimental request below 64k total tokens and 32k tokens for state plus the longest question, with documented safety margin and no retry-after-oversize strategy.
- [ ] Explicitly test the TypeSafe warning that unrelated shared state can reduce accuracy.
- [ ] Produce a decision table that selects a strategy by workload, including a valid `keep per-record requests` outcome when batching has no meaningful token advantage.
- [ ] Any paid benchmark runs only after offline fixtures and watcher are green and the user authorizes the spend; record model version, run count, and observed token total.
- [ ] Fresh watcher passes at clean committed HEAD.

## Non-goals
- Shipping a new default request shape.
- Claiming that fewer requests always cost less.
- Batching native requests with incompatible models or question schemas.

## Constraints
- Optimize useful answers per input token first; latency and requests are secondary metrics.
- Provider usage is the cost evidence. Serialized bytes are only a conservative safety bound, not a billing estimate.
- Large shared state must not expose one record's answer to unrelated or private record content without explicit user choice.

