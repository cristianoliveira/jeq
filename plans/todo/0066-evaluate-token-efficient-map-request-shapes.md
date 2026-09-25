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
- The earlier focused offline tests passed. At that point the configured Funzzy watcher was unavailable because `.watch.sock` was absent; this remains historical evidence only. Keep this task `doing` and do not start paid runs until fresh watcher verification and separate authorization.

## Executor phase progress (2026-09-25)
- Added a separate executor with a no-network dry-run, a pinned synthetic corpus, explicit authorization checks, sequential single-attempt HTTPS transport, observed-usage planning guards, and mode-restricted ignored JSONL results.
- Added deterministic fake-transport tests for success, malformed/missing usage, wrong model, retries, byte guards, token/request ceilings, credential echo rejection, and private file permissions. Tests and watcher do not use the paid execution path.
- Generated `examples/map-shapes-evaluation/preflight-authorization.json` from the offline dry-run. It records the 129 planned requests, 135 attempt cap, 814,000-token / $0.034188 planning figures, and 8.64M-token / $0.36288 current-price full-context envelope. The estimates are not guaranteed billing.
- During executor implementation and preflight, no provider calls or credentials were used. The later approved paid matrix is recorded below; keep TASK-0066 `doing` pending independent QA and do not start TASK-0067.

## Paid run phase (2026-09-25)
- Cristian explicitly approved one bounded 129-request matrix over the committed synthetic corpus. It completed 129/129 sequential attempts with zero retries, exact model `jev-1.13.0`, 62,223 reported input tokens, 4,956 output tokens, and `plan_complete`; no failure stop or extra request occurred.
- Current-list-price input usage is estimated at $0.002613366, not an invoice. The authorized $0.36288 full-context envelope remains non-guaranteed. Raw responses and the detailed decision table remain mode-restricted under ignored `examples/map-shapes-evaluation/private/`.
- Shared context saved 24.5–62.7% input tokens by workload, but every candidate strategy exceeded the strict per-answer probability-stability check in at least one workload. Threshold accuracy against synthetic labels was 80/84 per-record, 79/84 exact-dedup, and 81/84 shared-context. Keep per-record for all workloads; cheaper calls alone do not pass.
- The comparator is the synthetic per-record strategy measured directly by this executor, not a historical production usage summary. TASK-0065 recorded no paid/live baseline; production `jeq map --usage-summary` parity remains unverified.
- Kelly independently QAed the private evidence and decision table: pass, with no numeric or gate discrepancies. The watcher is disconnected (`.watch.sock` missing), so there is no fresh verification generation. Commit the documentation changes and restore watcher evidence before closing TASK-0066. No further provider calls are authorized.

## Non-goals
- Shipping a new default request shape.
- Claiming that fewer requests always cost less.
- Batching native requests with incompatible models or question schemas.

## Constraints
- Optimize useful answers per input token first; latency and requests are secondary metrics.
- Provider usage is the cost evidence. Serialized bytes are only a conservative safety bound, not a billing estimate.
- Large shared state must not expose one record's answer to unrelated or private record content without explicit user choice.

