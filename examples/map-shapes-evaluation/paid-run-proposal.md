# Proposed paid run matrix

**Status: one matrix completed on 2026-09-25 after explicit authorization.** The raw JSONL remains under ignored `private/`; sanitized aggregate results are in [`paid-run-results.md`](paid-run-results.md), with private evidence in `private/paid-run-analysis.md`. Do not rerun without renewed explicit approval. Refresh live TypeSafe limits, price, and billing behavior before any future run.

## Matrix

Pin model `jev-1.13.0`; reject any response with a different resolved model. Use three repeated runs for each workload/strategy pair. Make calls sequentially in a deterministic, interleaved strategy order. Disable retries (`--max-retries 0`). A missing-usage, malformed, or wrong-version response halts the run before another request. No oversize retry or request truncation.

| Workload | Per-record requests / repeat | Exact-dedup requests / repeat | Shared-context requests / repeat |
| --- | ---: | ---: | ---: |
| short unrelated | 4 | 4 | 1 |
| large unrelated | 4 | 4 | 1 |
| multi-question | 4 | 4 | 1 |
| exact duplicates | 4 | 2 | 1 |
| large shared reference | 4 | 4 | 1 |
| **Total** | **20** | **18** | **5** |

Three repetitions require 129 planned requests. The hard request cap is 135, the total if exact dedup saves nothing. A request-count cap is not a token or quality result. The live comparator is the per-record strategy. `internal/cli/map_baseline_replay_test.go` verifies offline that production `map --usage-summary` constructs the same canonical request bodies and reproduces sanitized per-workload usage totals with fake responses. This proves local request and summary parity, not another live baseline or original per-call token distribution. Run dedup and shared requests as explicit native requests and keep a separate local answer-ID map. Never put expected labels in provider input.

For every valid answer record the response model version, provider input/output usage, attempt count, latency, probability, expected label, and threshold. Compare per-record baseline variation against each candidate: per-answer mean probability, baseline within-run standard deviation, threshold-decision disagreement, threshold accuracy against fixture labels, and Noul Brier score. Report total requests, request-size distribution, decoded answers attached to records, total input/output tokens, elapsed latency, and TASK-0065 useful answers per 1,000 input tokens. A useful answer is a successfully decoded answer attached to its requested record; label correctness is reported separately. Require complete token usage for the efficiency ratio; if any attempted request, retry, or failed response has unknown usage, mark the ratio unknown rather than treating missing tokens as zero.

The experiment is exploratory. Three repeats can identify large movements, not certify production accuracy. Shared-context passes only if answer-ID routing is correct, threshold decisions and probability movement stay within baseline variation for every workload, and expected-label quality does not regress materially. Cheaper calls alone do not pass. `large-unrelated` directly tests the documented risk that irrelevant state lowers accuracy. If no strategy gives a useful token advantage without a quality regression, keep per-record requests.

### Decision rules by workload

These were the pre-run hypotheses; compare them with the sanitized measured decision table in [`paid-run-results.md`](paid-run-results.md):

| Workload | Candidate selection rule |
| --- | --- |
| Short unrelated | Keep per-record; use exact dedup only when canonical requests are identical. Consider shared context only if repeated runs show a useful token gain without quality movement. |
| Large unrelated | Keep per-record by default. The provider warns that unrelated state can reduce accuracy; shared context must clear the same quality gates. |
| Multi-question | Keep per-record unless shared context improves measured token efficiency and preserves each question's quality. Per-record already asks all configured questions together. |
| Exact duplicates | Prefer exact dedup only for byte-identical canonical requests and only if answer mapping and repeated quality checks agree with per-record. Otherwise keep per-record. |
| Large shared reference | Consider shared context only with explicit privacy opt-in and no measured quality regression; otherwise keep per-record despite repeated reference tokens. |

## Size and planning budgets

The offline planner currently measures 129 requests, with a maximum serialized request of 8,696 bytes and maximum serialized state plus longest question of 7,855 bytes. It checks stricter ceilings of 24 KiB and 12 KiB respectively. Those byte limits are safety checks only, not a token or price estimate.

The separately invoked executor's `--dry-run` prints the preflight authorization summary and makes no network request. Its `--execute` path requires `--allow-shared-context`, `--authorize-envelope-usd 0.36288`, `--confirm-input-price-usd-per-million 0.042`, `--confirm-synthetic-corpus-sha256` with the pinned hash, `--acknowledge-billing-uncertainty`, and `TYPESAFE_API_KEY`; tests inject a fake transport and the watcher only runs offline tests. The executor uses `PaidRunBudget` as an observed-usage planning guard: exact model `jev-1.13.0`, at most 135 sequential attempts, zero retries, and per-request validation. Reserve one full 64,000-token context before every call. At the 750,000 reported-token stop target, allow at most one final request if its full context still fits, then halt. This gives an **814,000 reported-usage planning ceiling**, not a guaranteed charge ceiling. If usage is missing, the response is malformed, the returned model is wrong, or an unexpected retry occurs, debit one full context per attempt in the planning ledger and halt before another request. This cannot prove how TypeSafe bills the call already made.

At the currently published $0.042 per million input tokens, 814,000 reported tokens correspond to a **$0.034188 planning estimate** (about 3.42 cents), not a maximum invoice. The conservative worst-case context envelope is 135 capped attempts × 64,000 input tokens = 8,640,000 tokens, or **$0.36288 at today's list price**, assuming every attempt is billed at full context. Use $0.36288 as the authorization envelope under the published per-attempt context and price. It is not a guaranteed invoice: TypeSafe's billing for failed or malformed requests is not established here, and prices, taxes, hidden billing treatment, or other charges can change. The request/token guards do not infer billed usage from serialized bytes. If the provider's current billing behavior or price cannot be confirmed, stop and ask Cristian to approve a revised envelope.

## Authorization gate

Before any paid request:

1. Run the offline tests and matrix on the final committed fixtures.
2. Confirm current Jev limits, version, price, and failure billing behavior from the linked TypeSafe docs.
3. Confirm the only data is the committed synthetic corpus and explicitly opt in to shared context.
4. Show Cristian the 135-request cap, 814,000-token observed-usage planning ceiling, $0.034188 planning estimate, $0.36288 full-context list-price envelope, and matrix above. Wait for explicit authorization.
5. Save raw responses only as mode-0600 JSONL under the ignored `examples/map-shapes-evaluation/private/` directory. Read the API key only from `TYPESAFE_API_KEY`; never persist it. Do not commit raw results or credentials. Commit only sanitized aggregates without request, state, or response content.
6. Stop before spending if the returned model differs, usage is missing, the response is malformed, an unexpected retry occurs, a request exceeds either byte guard, the 135-attempt cap is reached, or the 814,000-token planning ceiling stops the run.
