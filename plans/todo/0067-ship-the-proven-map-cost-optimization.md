---
id: TASK-0067
title: Ship the proven map cost optimization
status: todo
depends_on: [TASK-0066]
priority: low
tags: [cost, map, batching, deduplication, cli, typesafe]
---

# Ship the proven map cost optimization

## Current decision (2026-09-25)
TASK-0066 found no strategy that passed its pre-set probability-stability gate on the synthetic workloads. Keep per-record map requests; do not implement batching or deduplication from the observed token savings alone. This task is retained for future evidence, not ready for implementation on the current results. Reassess only with a new quality case and explicit authorization for any further paid validation. Cristian accepted the no-optimization decision when closing TASK-0066.

## Problem
After measuring competing request shapes, jeq must apply the cheapest validated strategy without hiding usage, exceeding TypeSafe context limits, or weakening deterministic stream behavior.

## Desired outcome
jeq applies the strategy proven by TASK-0066 to eligible map streams and exposes explicit controls for every semantic tradeoff. Users get more useful answers per input token while preserving ordered output, bounded concurrency, honest usage evidence, and predictable failures.

## Approach
Implement only benchmark-backed optimizations. Exact-request reuse and shared-context batching are separate capabilities because they have different correctness and privacy consequences. Keep the current per-record path as the fallback for ineligible or unproven workloads.

## Acceptance criteria
- [ ] Record the TASK-0066 decision table in user documentation and implement only strategies with measured token savings and an accepted quality result.
- [ ] Preserve the current per-record behavior when optimization is disabled, unsupported, or not expected to reduce input tokens.
- [ ] Any cache or deduplication behavior is opt-in and keyed by the complete canonical evaluation request, including model, state, questions, and provider-relevant extension fields.
- [ ] Any shared-context batch behavior is opt-in unless TASK-0066 proves a safe general default; help text states that questions see the shared batch state.
- [ ] Batch construction respects the documented 64k total and 32k state-plus-longest-question budgets with a conservative local ceiling; an oversized batch is split before network I/O.
- [ ] Record/question namespacing is collision-safe, and every returned answer is attached to exactly one original record and question.
- [ ] Output remains in input order, concurrency stays bounded, cancellation drains workers, and the first ordered failure preserves only the valid output prefix.
- [ ] Batch usage is reported once and never copied into each record in a way that causes downstream token totals to multiply; the evidence format documents batch attribution.
- [ ] Retries do not silently expand a failed batch or create duplicate successful evidence.
- [ ] Security tests cover cross-record confusion, private shared context, hostile record content, stale answer IDs, unknown answer IDs, and mixed native-request schemas.
- [ ] A before/after acceptance benchmark demonstrates the expected improvement in useful answers per 1,000 input tokens on every enabled workload and no regression on fallback workloads.
- [ ] Documentation gives one cost-efficient example for each supported strategy and tells users to select the smallest relevant state with `--state-pointer`.
- [ ] Fresh watcher and independent QA pass at clean committed HEAD.

## Non-goals
- Automatic semantic caching across separate command invocations.
- Combining incompatible models or question schemas.
- Trading answer quality for a lower token count without an explicit user choice.
- Treating HTTP request count as the billing metric.

## Constraints
- TypeSafe input-token usage is the source of truth for cost efficiency.
- stdout remains composable JSON or NDJSON; diagnostics and aggregate usage stay on stderr.
- No paid validation without explicit user authorization.

