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
- [x] Add a reproducible benchmark harness whose inputs and calculations are committed, while live credentials, response caches, and paid outputs remain ignored.
- [ ] Establish the existing per-record map behavior as the baseline using TASK-0065 usage summaries.
- [ ] Compare all three strategies on short unrelated records, large unrelated records, multi-question records, exact duplicates, and large shared-context workloads.
- [ ] Report total input tokens, answers per 1,000 input tokens, requests, latency, and request-size distribution for every case.
- [x] Prove offline that batched answer IDs map back to the correct record and original question without changing input order.
- [ ] Evaluate quality using repeated provider runs: compare mean probabilities and configured threshold decisions against both baseline variation and expected labels; batching does not pass solely because it is cheaper.
- [x] Keep each experimental request below 64k total tokens and 32k tokens for state plus the longest question, with documented safety margin and no retry-after-oversize strategy.
- [ ] Explicitly test the TypeSafe warning that unrelated shared state can reduce accuracy.
- [ ] Produce a decision table that selects a strategy by workload, including a valid `keep per-record requests` outcome when batching has no meaningful token advantage.
- [ ] Any paid benchmark runs only after offline fixtures and watcher are green and the user authorizes the spend; record model version, run count, and observed token total.
- [ ] Fresh watcher passes at clean committed HEAD.

## Offline phase progress (2026-09-24)
- Commit `2e8ac48` adds deterministic synthetic fixtures, the offline planner and metric checks, byte ceilings, and an ignored-results policy.
- The matrix plans 129 requests across 5 workloads, 3 strategies, and 3 repetitions (135-call hard cap). It pins `jev-1.13.0`, disables retries, and uses an 814,000-token observed-usage planning ceiling. This is not a guaranteed billing cap. At the checked current price, that is a $0.034188 planning estimate; the 135 × 64,000-token full-context list-price envelope is $0.36288, subject to provider billing and price uncertainty. Details and decision rules are in [`examples/map-shapes-evaluation/paid-run-proposal.md`](../../examples/map-shapes-evaluation/paid-run-proposal.md).
- No TypeSafe calls, credentials, caches, or paid outputs were used. Fake metric outputs are arithmetic tests, not quality or cost evidence. Shared-context semantic quality, provider token counts, and latency remain unmeasured pending explicit authorization.
- Focused offline tests pass. The configured Funzzy watcher was unavailable because `.watch.sock` was absent, so the fresh watcher criterion remains open. Keep this task `doing` and do not start paid runs until watcher verification and separate authorization.

## Non-goals
- Shipping a new default request shape.
- Claiming that fewer requests always cost less.
- Batching native requests with incompatible models or question schemas.

## Constraints
- Optimize useful answers per input token first; latency and requests are secondary metrics.
- Provider usage is the cost evidence. Serialized bytes are only a conservative safety bound, not a billing estimate.
- Large shared state must not expose one record's answer to unrelated or private record content without explicit user choice.

