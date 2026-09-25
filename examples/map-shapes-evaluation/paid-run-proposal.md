# Proposed paid run matrix

**Status: proposal only. No provider requests have been sent.** Run only after Cristian authorizes the spend and the synthetic data scope. Recheck the live TypeSafe limits and price immediately before execution.

## Matrix

Pin model `jev-1.13.0`; reject any response with a different resolved model. Use three repeated runs for each workload/strategy pair. Make calls sequentially in a deterministic, interleaved strategy order. Disable retries (`--max-retries 0`); a rejected or missing-usage request stops or consumes the conservative budget instead of retrying. No oversize retry or request truncation.

| Workload | Per-record requests / repeat | Exact-dedup requests / repeat | Shared-context requests / repeat |
| --- | ---: | ---: | ---: |
| short unrelated | 4 | 4 | 1 |
| large unrelated | 4 | 4 | 1 |
| multi-question | 4 | 4 | 1 |
| exact duplicates | 4 | 2 | 1 |
| large shared reference | 4 | 4 | 1 |
| **Total** | **20** | **18** | **5** |

Three repetitions require 129 planned requests. The hard request cap is 135, the total if exact dedup saves nothing. A request-count cap is not a token or quality result. Use the existing `map --usage-summary` behavior for the per-record baseline and TASK-0065 summaries for request/token accounting. Run dedup and shared requests as explicit native requests and keep a separate local answer-ID map. Never put expected labels in provider input.

For every valid answer record the response model version, provider input/output usage, attempt count, latency, probability, expected label, and threshold. Compare per-record baseline variation against each candidate: per-answer mean probability, baseline within-run standard deviation, threshold-decision disagreement, threshold accuracy against fixture labels, and Noul Brier score. Report total requests, request-size distribution, useful answers, total input/output tokens, elapsed latency, and useful answers per 1,000 input tokens. Require complete token usage for the efficiency ratio; if any attempted request, retry, or failed response has unknown usage, mark the ratio unknown rather than treating missing tokens as zero.

The experiment is exploratory. Three repeats can identify large movements, not certify production accuracy. Shared-context passes only if answer-ID routing is correct, threshold decisions and probability movement stay within baseline variation for every workload, and expected-label quality does not regress materially. Cheaper calls alone do not pass. `large-unrelated` directly tests the documented risk that irrelevant state lowers accuracy. If no strategy gives a useful token advantage without a quality regression, keep per-record requests.

### Decision rules by workload

These are hypotheses for the paid results, not measured findings:

| Workload | Candidate selection rule |
| --- | --- |
| Short unrelated | Keep per-record; use exact dedup only when canonical requests are identical. Consider shared context only if repeated runs show a useful token gain without quality movement. |
| Large unrelated | Keep per-record by default. The provider warns that unrelated state can reduce accuracy; shared context must clear the same quality gates. |
| Multi-question | Keep per-record unless shared context improves measured token efficiency and preserves each question's quality. Per-record already asks all configured questions together. |
| Exact duplicates | Prefer exact dedup only for byte-identical canonical requests and only if answer mapping and repeated quality checks agree with per-record. Otherwise keep per-record. |
| Large shared reference | Consider shared context only with explicit privacy opt-in and no measured quality regression; otherwise keep per-record despite repeated reference tokens. |

## Size and budget caps

The offline planner currently measures 129 requests, with a maximum serialized request of 8,696 bytes and maximum serialized state plus longest question of 7,855 bytes. It checks stricter ceilings of 24 KiB and 12 KiB respectively. Those byte limits are safety checks only, not a token or price estimate.

The paid executor must use `PaidRunBudget`'s policy in the harness: exact model `jev-1.13.0`, at most 135 requests, sequential calls, zero retries, per-request validation, and no next call unless a full 64,000-token context still fits in the 814,000-token budget. The 814,000 allowance is a 750,000 reported-usage stop target plus one 64,000-token final-request reserve. If usage is missing or the response is malformed, conservatively charge the full 64,000-token context against the allowance. Stop before another request when its maximum context would exceed the budget.

The current models page lists input at $0.042 per million tokens and output as free. At the proposed 814,000 input-token cap, the maximum input charge is **$0.034188 (about 3.42 cents)**, excluding taxes or any future price change. The executor must enforce the token cap, not infer billed usage from serialized bytes. Failed provider requests with unreported usage consume a full context reserve. If the provider's billing rules change or this cap cannot be enforced, do not run; ask Cristian to approve a revised cap.

## Authorization gate

Before any paid request:

1. Run the offline tests and matrix on the final committed fixtures.
2. Confirm current Jev limits, version, price, and failure billing behavior from the linked TypeSafe docs.
3. Confirm the only data is the committed synthetic corpus and explicitly opt in to shared context.
4. Show Cristian the 135-request cap, 814,000-token cap, $0.034188 maximum, and matrix above. Wait for explicit authorization.
5. Save raw responses, credentials, and caches only under ignored `private/`, `.cache/`, or `results/` directories. Do not commit them.
6. Stop before spending if the returned model differs, the budget guard fails, a request exceeds the byte guard, or the measured usage reaches the cap.
