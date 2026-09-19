# Live TypeSafe API baseline (TASK-0018)

Status: paid production evidence, sanitized. No code or plans changed; nothing committed.
Run (UTC): **2026-09-19T08:26:32+00:00** · endpoint: `https://api.typesafe.ai` · commit context: af7de85 (main)
Credential handling: `TYPESAFE_API_KEY` read from the environment **inside** the harness process (`.tmp/live-baseline/run.py`); never printed, never in argv, fixtures, reports, or raw captures (leak-scan for `Bearer`/key material over raw files: clean; raws are git-ignored).

## Calls

| Call | Status | Attempts | Latency | Notes |
| --- | --- | --- | --- | --- |
| `GET /v1/models` | 200 | 1 | 593 ms | Parseable, non-empty: lists aliases `jev-latest`, `jev-preview` (matches docs: aliases only; versioned IDs accepted but not listed) |
| `POST /v1/systemone` | 200 | 1 | 724 ms | One paid request batching Noul + Choice + Score; synthetic state only; pinned `model: "jev-1.13.0"` |

## Accounting

- Resolved model: **`jev-1.13.0`** — equals the pinned versioned ID (pin honored, no alias drift).
- Token usage: **input 462, output 73** (both non-negative integers; output free per pricing).
- Approximate input cost: **≈ $0.0000194** (462 × $42/Btok). Price source: https://docs.typesafe.ai/models ("$42 / Btok", "Charged per input token. Output tokens are free."), retrieved 2026-09-19.
- Latency is a single sample per call — recorded evidence, not a benchmark.

## Assertions — 22/22 PASS

Schema/range checks only; exact judgment values withheld by design (never asserted, not recorded here). Stated epsilon for "floats that sum to 1": **|sum − 1| ≤ 1e-6**.

| # | Assertion | Result |
| --- | --- | --- |
| 1 | GET status 200 | PASS |
| 2 | Models list parseable and non-empty | PASS |
| 3 | POST status 200, single attempt | PASS |
| 4 | Resolved model present | PASS |
| 5 | Resolved model == pinned `jev-1.13.0` | PASS |
| 6 | Answers keyed exactly by the three question ids | PASS |
| 7 | Noul answer `type` matches | PASS |
| 8 | Noul probability ∈ [0,1] | PASS |
| 9 | Choice answer `type` matches | PASS |
| 10 | Chosen option ∈ defined criteria options | PASS |
| 11 | Probability map covers exactly the defined options | PASS |
| 12 | Choice probabilities ∈ [0,1] | PASS |
| 13 | Choice probabilities sum ≈ 1 (observed \|sum−1\| = 0.00e+00) | PASS |
| 14 | Choice confidence ∈ [0,1] | PASS |
| 15 | Score answer `type` matches | PASS |
| 16 | Legend present, string keys 0..n−1 matching criteria levels | PASS |
| 17 | Score probability keys == legend keys | PASS |
| 18 | Score probabilities ∈ [0,1] | PASS |
| 19 | Score probabilities sum ≈ 1 (observed \|sum−1\| = 0.00e+00) | PASS |
| 20 | Score value within [0, n−1] level range | PASS |
| 21 | Score confidence ∈ [0,1] | PASS |
| 22 | Usage integers, non-negative | PASS |

Raw responses (git-ignored): `.tmp/live-baseline/raw_models.json`, `.tmp/live-baseline/raw_systemone.json`. Harness: `.tmp/live-baseline/run.py`.

## Contract confirmations for D2 work

- Response shape matches ADR 0001's lossless-output expectations: `model`, `answers` keyed by caller ids, per-answer `type`, `confidence` on Choice/Score, `legend` on Score, `usage` — nothing observed that jeq's contract types (TASK-0003) must deviate from.
- Pinning behavior confirmed: sending a versioned ID returns that same versioned ID in `model` — the fake-server fixtures should mirror this (fixture `response_200_full.json` uses `jev-1.13.0`).
- `GET /v1/models` returns aliases only — TASK-0012's models view should not expect versioned IDs in the list.

## Findings

**None contradicting the contract.** Two non-blocking observations:

1. Rate-limit/overload paths (429/529) were **not** exercised live — no such status occurred in one attempt each. Bounded-retry behavior (TASK-0007) remains verified against the fake server only; a live 429 probe would need deliberate limit-pushing and is not worth the spend for this baseline.
2. Docs rate limits (250k tok/s, 1200 req/min) are flagged "adjusting dynamically" upstream — irrelevant to jeq's correctness contract, but retry bounds should not assume those numbers stay stable.
