# jeq request-shape results

One explicitly authorized matrix completed on 2026-09-25: 3 repetitions, 5 synthetic workloads, 3 strategies, 129/129 successful requests, one attempt each, no retries, model `jev-1.13.0`. Reported usage was 62,223 input and 4,956 output tokens. At the checked $0.042/M input list price, the estimate is $0.002613366, not an invoice or guaranteed charge. No additional provider calls were made.

The per-record baseline below is the per-record strategy from this same live matrix, not historical production usage. TASK-0065 did not capture a paid baseline. Offline parity is now tested separately: `jeq map --usage-summary` request-body hashes match the planner's per-record hashes for every synthetic record, and deterministic fake responses reproduce the per-workload usage aggregates. This verifies production CLI request construction and summary accounting; it is not a second live baseline or per-request billing replay. The fake replay distributes aggregate token totals across its responses and does not preserve the original per-request usage distribution.

## Aggregate by strategy

“Attached answers” follows TASK-0065: successfully decoded answers attached to their record. “Correct labels” is a separate synthetic-fixture threshold-quality measure.

| Strategy | Requests | Input / output tokens | Attached answers | Answers per 1,000 input tokens | Correct labels | Threshold accuracy | Brier | Input list-price estimate |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Per-record baseline | 60 | 24,912 / 1,584 | 84 | 3.371869 | 80/84 | 95.24% | 0.064974 | $0.001046304 |
| Exact dedup | 54 | 23,184 / 1,464 | 84 | 3.623188 | 79/84 | 94.05% | 0.064885 | $0.000973728 |
| Shared context | 15 | 14,127 / 1,908 | 84 | 5.946061 | 81/84 | 96.43% | 0.048861 | $0.000593334 |

## Usage and label quality by workload

| Workload | Strategy | Requests | Input / output tokens | Input savings | Attached answers | Answers / 1,000 input tokens | Correct labels | Accuracy | Brier |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Short unrelated | Per-record | 12 | 3,462 / 240 | baseline | 12 | 3.466205 | 11/12 | 91.7% | 0.078017 |
|  | Exact dedup | 12 | 3,462 / 240 | 0% | 12 | 3.466205 | 10/12 | 83.3% | 0.081617 |
|  | Shared context | 3 | 1,770 / 276 | 48.9% | 12 | 6.779661 | 12/12 | 100% | 0.011617 |
| Large unrelated | Per-record | 12 | 6,528 / 240 | baseline | 12 | 1.838235 | 12/12 | 100% | 0.009233 |
|  | Exact dedup | 12 | 6,528 / 240 | 0% | 12 | 1.838235 | 12/12 | 100% | 0.008483 |
|  | Shared context | 3 | 4,836 / 276 | 25.9% | 12 | 2.481390 | 12/12 | 100% | 0.004917 |
| Multi-question | Per-record | 12 | 3,876 / 624 | baseline | 36 | 9.287926 | 33/36 | 91.7% | 0.115133 |
|  | Exact dedup | 12 | 3,876 / 624 | 0% | 36 | 9.287926 | 33/36 | 91.7% | 0.113594 |
|  | Shared context | 3 | 2,928 / 804 | 24.5% | 36 | 12.295082 | 33/36 | 91.7% | 0.105408 |
| Exact duplicates | Per-record | 12 | 3,456 / 240 | baseline | 12 | 3.472222 | 12/12 | 100% | 0.001675 |
|  | Exact dedup | 6 | 1,728 / 120 | 50.0% | 12 | 6.944444 | 12/12 | 100% | 0.001750 |
|  | Shared context | 3 | 1,764 / 276 | 49.0% | 12 | 6.802721 | 12/12 | 100% | 0.001375 |
| Large shared reference | Per-record | 12 | 7,590 / 240 | baseline | 12 | 1.581028 | 12/12 | 100% | 0.020492 |
|  | Exact dedup | 12 | 7,590 / 240 | 0% | 12 | 1.581028 | 12/12 | 100% | 0.021558 |
|  | Shared context | 3 | 2,829 / 276 | 62.7% | 12 | 4.241782 | 12/12 | 100% | 0.007892 |

## Probability stability

Mean movement is the mean absolute difference between each answer's candidate and baseline three-run mean. Baseline SD is the mean per-answer population standard deviation across the three per-record repetitions. “Within SD” requires every answer to remain within its own baseline SD. Threshold disagreement compares the threshold decisions of candidate and baseline means.

| Workload | Strategy | Mean movement / mean baseline SD | Threshold disagreement | Within baseline SD for every answer? |
| --- | --- | ---: | ---: | --- |
| Short unrelated | Exact dedup | 0.00667 / 0.00838 | 25.0% | No |
|  | Shared context | 0.12833 / 0.00838 | 0% | No |
| Large unrelated | Exact dedup | 0.00500 / 0.00440 | 0% | No |
|  | Shared context | 0.02833 / 0.00440 | 0% | No |
| Multi-question | Exact dedup | 0.00444 / 0.00368 | 0% | No |
|  | Shared context | 0.04139 / 0.00368 | 0% | No |
| Exact duplicates | Exact dedup | 0.00250 / 0.00118 | 0% | No |
|  | Shared context | 0.01333 / 0.00118 | 0% | No |
| Large shared reference | Exact dedup | 0.00333 / 0.00665 | 0% | No |
|  | Shared context | 0.04000 / 0.00665 | 0% | No |

## Request size and latency

Body sizes are UTF-8 serialized request bytes. Latency is per HTTP exchange; p50/p95 are across each workload/strategy's three repetitions. Medians are rounded to whole units.

| Workload | Strategy | Body bytes min / median / max | Latency p50 / p95 ms |
| --- | --- | ---: | ---: |
| Short unrelated | Per-record | 183 / 185 / 185 | 634 / 668 |
|  | Exact dedup | 183 / 185 / 185 | 613 / 711 |
|  | Shared context | 1,408 / 1,408 / 1,408 | 609 / 671 |
| Large unrelated | Per-record | 2,002 / 2,007 / 2,010 | 615 / 642 |
|  | Exact dedup | 2,002 / 2,007 / 2,010 | 612 / 657 |
|  | Shared context | 8,696 / 8,696 / 8,696 | 626 / 638 |
| Multi-question | Per-record | 376 / 381 / 387 | 607 / 654 |
|  | Exact dedup | 376 / 381 / 387 | 618 / 653 |
|  | Shared context | 3,490 / 3,490 / 3,490 | 649 / 652 |
| Exact duplicates | Per-record | 182 / 187 / 192 | 619 / 654 |
|  | Exact dedup | 182 / 187 / 192 | 611 / 679 |
|  | Shared context | 1,466 / 1,466 / 1,466 | 619 / 649 |
| Large shared reference | Per-record | 2,275 / 2,279 / 2,285 | 632 / 652 |
|  | Exact dedup | 2,275 / 2,279 / 2,285 | 624 / 694 |
|  | Shared context | 3,647 / 3,647 / 3,647 | 622 / 634 |

## Decision

Keep per-record requests for all workloads. Exact dedup saves 50% of input tokens only for byte-identical records, but this sample did not clear the probability-stability gate and exact-dedup label accuracy fell on short unrelated inputs. Shared context saves 24.5–62.7% per workload and did not regress threshold accuracy in this sample, but it exceeded per-answer baseline variation in all five workloads. Cheaper calls alone do not pass.

Three repetitions over synthetic fixtures are exploratory, not a production accuracy guarantee. Labels are fixture expectations, not independent ground truth. The live per-record values came from the one approved matrix; no additional provider calls were made. Input-price values use the checked $0.042/M rate and are estimates, not invoice amounts. Raw requests, states, responses, credentials, and personal data are not included in this report. The hash-only replay manifest contains no request content.
